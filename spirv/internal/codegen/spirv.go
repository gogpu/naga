// Package codegen implements SPIR-V code generation from naga IR.
package codegen

import (
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// Version represents a SPIR-V version.
type Version struct {
	Major uint8
	Minor uint8
}

// Common SPIR-V versions
var (
	Version1_0 = Version{1, 0}
	Version1_1 = Version{1, 1}
	Version1_2 = Version{1, 2}
	Version1_3 = Version{1, 3}
	Version1_4 = Version{1, 4}
	Version1_5 = Version{1, 5}
	Version1_6 = Version{1, 6}
)

// Options configures SPIR-V generation.
type Options struct {
	// Version is the SPIR-V version to target
	Version Version

	// ExtraCapabilities are declared in addition to those the module needs.
	ExtraCapabilities []spvenums.Capability

	// Debug includes debug information
	Debug bool

	// UseStorageInputOutput16 enables StorageInputOutput16 capability for f16
	// types used in entry point inputs/outputs. When true, f16 I/O uses native
	// 16-bit types; when false, a polyfill converts to/from f32.
	UseStorageInputOutput16 bool

	// ForcePointSize adds a spvenums.BuiltIn PointSize output variable with value 1.0
	// to every vertex shader entry point. Required by some Vulkan drivers.
	ForcePointSize bool

	// AdjustCoordinateSpace flips the Y coordinate of Position built-in outputs
	// in vertex shaders to match Vulkan's coordinate system (Y pointing down).
	AdjustCoordinateSpace bool

	// ForceLoopBounding inserts a decrementing counter at every loop header
	// that breaks when the counter reaches zero. This prevents infinite loops
	// from hanging the GPU. Matches Rust naga's force_loop_bounding option
	// (default true). The counter uses a vec2<u32> to simulate 64-bit range.
	ForceLoopBounding bool

	// SkipIntDivChecks writes integer division and remainder as raw spvenums.OpSDiv,
	// OpUDiv and spvenums.OpUMod instead of calling the guard helpers (Rust naga's
	// emit_int_div_checks = false). Negative polarity so that a zero Options
	// keeps the guards. Signed remainder keeps its helper either way.
	SkipIntDivChecks bool

	// BoundsCheckPolicies selects how out-of-bounds indices and image loads
	// are handled; see spirv.Options.BoundsCheckPolicies.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// EntryPoint selects the one entry point to write; nil writes all.
	EntryPoint *backend.EntryPointSelector

	// AllowedCapabilities gates the operations that have a fallback. When nil,
	// all capabilities are available (the default). When non-nil, an
	// operation whose capabilities are not all in the set uses its polyfill;
	// capabilities the backend adds unconditionally are not filtered, as Rust
	// naga's capabilities_used table. Matches Rust naga's Options::capabilities.
	AllowedCapabilities map[spvenums.Capability]struct{}

	// RayQueryInitTracking enables initialization tracking for ray queries.
	// When true (default), ray query helper functions include validation checks
	// that track whether the query was properly initialized before proceed/get.
	// When false, validation checks are skipped and helper functions branch
	// unconditionally. Matches Rust naga's ray_query_initialization_tracking.
	RayQueryInitTracking bool

	// BindingMap maps WGSL @group/@binding to the SPIR-V DescriptorSet and
	// Binding, and optionally gives a binding array its size. Rust naga's
	// Options::binding_map.
	BindingMap map[ir.ResourceBinding]BindingInfo

	// FakeMissingBindings keeps a resource that a non-nil BindingMap leaves
	// out at its WGSL @group/@binding; without it that is an error. A nil
	// BindingMap keeps every resource there (Rust naga's
	// fake_missing_bindings).
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory is how compute entry points zero their
	// workgroup variables.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// ClampFragDepth clamps the depth a fragment entry point writes to the
	// range two f32 push constants hold, min then max, at byte offset
	// ClampFragDepth.Offset (see the spirv package).
	ClampFragDepth *backend.ClampFragDepth

	// DemoteToHelperInvocation writes WGSL discard as
	// OpDemoteToHelperInvocation instead of spvenums.OpKill; see
	// spirv.Options.DemoteToHelperInvocation.
	DemoteToHelperInvocation bool
}

// BindingInfo is where a resource binding goes in SPIR-V; see
// spirv.BindingInfo. Mirrors Rust naga's back::spv::BindingInfo.
type BindingInfo struct {
	DescriptorSet uint32
	Binding       uint32
	// BindingArraySize, when set, declares a binding array with this many
	// elements instead of its WGSL size.
	BindingArraySize *uint32
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Version:                 Version1_1,
		Debug:                   false,
		UseStorageInputOutput16: true,
		ForceLoopBounding:       true,
		RayQueryInitTracking:    true,
	}
}

// Writer generates SPIR-V from IR.
type Writer struct {
	options Options

	// Internal state
	nextID      uint32
	typeIDs     map[uint32]uint32
	constantIDs map[uint32]uint32
}

// NewWriter creates a new SPIR-V writer.
func NewWriter(options Options) *Writer {
	return &Writer{
		options:     options,
		nextID:      1,
		typeIDs:     make(map[uint32]uint32),
		constantIDs: make(map[uint32]uint32),
	}
}

// SPIR-V magic number and constants
const (
	MagicNumber = 0x07230203
	GeneratorID = 0x00000000 // Unregistered generator
)

// StorageFormatToImageFormat converts an IR storage format to a SPIR-V image format.
func StorageFormatToImageFormat(format ir.StorageFormat) spvenums.ImageFormat {
	switch format {
	// 8-bit formats
	case ir.StorageFormatR8Unorm:
		return spvenums.ImageFormatR8
	case ir.StorageFormatR8Snorm:
		return spvenums.ImageFormatR8Snorm
	case ir.StorageFormatR8Uint:
		return spvenums.ImageFormatR8ui
	case ir.StorageFormatR8Sint:
		return spvenums.ImageFormatR8i

	// 16-bit formats
	case ir.StorageFormatR16Uint:
		return spvenums.ImageFormatR16ui
	case ir.StorageFormatR16Sint:
		return spvenums.ImageFormatR16i
	case ir.StorageFormatR16Float:
		return spvenums.ImageFormatR16f
	case ir.StorageFormatRg8Unorm:
		return spvenums.ImageFormatRg8
	case ir.StorageFormatRg8Snorm:
		return spvenums.ImageFormatRg8Snorm
	case ir.StorageFormatRg8Uint:
		return spvenums.ImageFormatRg8ui
	case ir.StorageFormatRg8Sint:
		return spvenums.ImageFormatRg8i

	// 32-bit formats
	case ir.StorageFormatR32Uint:
		return spvenums.ImageFormatR32ui
	case ir.StorageFormatR32Sint:
		return spvenums.ImageFormatR32i
	case ir.StorageFormatR32Float:
		return spvenums.ImageFormatR32f
	case ir.StorageFormatRg16Uint:
		return spvenums.ImageFormatRg16ui
	case ir.StorageFormatRg16Sint:
		return spvenums.ImageFormatRg16i
	case ir.StorageFormatRg16Float:
		return spvenums.ImageFormatRg16f
	case ir.StorageFormatRgba8Unorm:
		return spvenums.ImageFormatRgba8
	case ir.StorageFormatRgba8Snorm:
		return spvenums.ImageFormatRgba8Snorm
	case ir.StorageFormatRgba8Uint:
		return spvenums.ImageFormatRgba8ui
	case ir.StorageFormatRgba8Sint:
		return spvenums.ImageFormatRgba8i
	case ir.StorageFormatBgra8Unorm:
		return spvenums.ImageFormatRgba8 // BGRA not directly supported, use RGBA

	// Packed 32-bit formats
	case ir.StorageFormatRgb10a2Uint:
		return spvenums.ImageFormatRgb10a2ui
	case ir.StorageFormatRgb10a2Unorm:
		return spvenums.ImageFormatRgb10A2
	case ir.StorageFormatRg11b10Ufloat:
		return spvenums.ImageFormatR11fG11fB10f

	// 64-bit formats
	case ir.StorageFormatRg32Uint:
		return spvenums.ImageFormatRg32ui
	case ir.StorageFormatRg32Sint:
		return spvenums.ImageFormatRg32i
	case ir.StorageFormatRg32Float:
		return spvenums.ImageFormatRg32f
	case ir.StorageFormatRgba16Uint:
		return spvenums.ImageFormatRgba16ui
	case ir.StorageFormatRgba16Sint:
		return spvenums.ImageFormatRgba16i
	case ir.StorageFormatRgba16Float:
		return spvenums.ImageFormatRgba16f

	// 128-bit formats
	case ir.StorageFormatRgba32Uint:
		return spvenums.ImageFormatRgba32ui
	case ir.StorageFormatRgba32Sint:
		return spvenums.ImageFormatRgba32i
	case ir.StorageFormatRgba32Float:
		return spvenums.ImageFormatRgba32f

	// Normalized 16-bit per channel formats
	case ir.StorageFormatR16Unorm:
		return spvenums.ImageFormatR16
	case ir.StorageFormatR16Snorm:
		return spvenums.ImageFormatR16Snorm
	case ir.StorageFormatRg16Unorm:
		return spvenums.ImageFormatRg16
	case ir.StorageFormatRg16Snorm:
		return spvenums.ImageFormatRg16Snorm
	case ir.StorageFormatRgba16Unorm:
		return spvenums.ImageFormatRgba16
	case ir.StorageFormatRgba16Snorm:
		return spvenums.ImageFormatRgba16Snorm

	// 64-bit integer formats (SPV_EXT_shader_image_int64)
	case ir.StorageFormatR64Uint:
		return spvenums.ImageFormatR64ui
	case ir.StorageFormatR64Sint:
		return spvenums.ImageFormatR64i

	default:
		return spvenums.ImageFormatUnknown
	}
}
