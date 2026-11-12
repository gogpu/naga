package spirv

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/internal/unsupported"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/codegen"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// --- Configuration types (real types, not aliases) ---

// Version represents a SPIR-V version.
type Version struct {
	Major uint8
	Minor uint8
}

// Common SPIR-V versions.
var (
	Version1_0 = Version{1, 0}
	Version1_1 = Version{1, 1}
	Version1_2 = Version{1, 2}
	Version1_3 = Version{1, 3}
	Version1_4 = Version{1, 4}
	Version1_5 = Version{1, 5}
	Version1_6 = Version{1, 6}
)

// Capability is a SPIR-V capability, by its number in the SPIR-V
// specification (spirv::Capability in Rust). The constants name the ones the
// backend writes or callers commonly allow; any other value of the
// specification can be converted.
type Capability uint32

// CapabilitySet is a set of capabilities. Its nil value is an empty set, which
// is not the same as a nil Options.AllowedCapabilities: that one allows every
// capability.
type CapabilitySet map[Capability]struct{}

// NewCapabilitySet returns a set holding every capability in caps.
func NewCapabilitySet(caps ...Capability) CapabilitySet {
	set := make(CapabilitySet, len(caps))
	for _, c := range caps {
		set[c] = struct{}{}
	}
	return set
}

// Has reports whether the set holds c.
func (s CapabilitySet) Has(c Capability) bool {
	_, ok := s[c]
	return ok
}

// Common capabilities.
const (
	CapabilityMatrix                             Capability = 0
	CapabilityShader                             Capability = 1
	CapabilityFloat16                            Capability = 9
	CapabilityFloat64                            Capability = 10
	CapabilityInt64                              Capability = 11
	CapabilityInt16                              Capability = 22
	CapabilityImageGatherExtended                Capability = 25
	CapabilityInt8                               Capability = 39
	CapabilityLinkage                            Capability = 5
	CapabilityInt64Atomics                       Capability = 12
	CapabilityClipDistance                       Capability = 32
	CapabilityImageCubeArray                     Capability = 34
	CapabilitySampleRateShading                  Capability = 35
	CapabilitySampled1D                          Capability = 43
	CapabilityImage1D                            Capability = 44
	CapabilitySampledCubeArray                   Capability = 45
	CapabilityStorageImageExtendedFormats        Capability = 49
	CapabilityImageQuery                         Capability = 50
	CapabilityDerivativeControl                  Capability = 51
	CapabilityStorageImageWriteWithoutFormat     Capability = 56
	CapabilityDrawParameters                     Capability = 4427
	CapabilityStorageBuffer16BitAccess           Capability = 4433
	CapabilityUniformAndStorageBuffer16BitAccess Capability = 4434
	CapabilityStorageInputOutput16               Capability = 4436
	CapabilityMultiView                          Capability = 4439
	CapabilityRayQueryKHR                        Capability = 4472
	CapabilityFragmentBarycentricKHR             Capability = 5284
	CapabilityDemoteToHelperInvocation           Capability = 5379
	CapabilityShaderNonUniform                   Capability = 5301
	CapabilityAtomicFloat32AddEXT                Capability = 6033
	CapabilityDotProductInputAll                 Capability = 6016
	CapabilityDotProductInput4x8Bit              Capability = 6017
	CapabilityDotProductInput4x8BitPacked        Capability = 6018
	CapabilityDotProduct                         Capability = 6019
	CapabilityGroupNonUniform                    Capability = 61
	CapabilityGroupNonUniformVote                Capability = 62
	CapabilityGroupNonUniformArithmetic          Capability = 63
	CapabilityGroupNonUniformBallot              Capability = 64
	CapabilityGroupNonUniformShuffle             Capability = 65
	CapabilityGroupNonUniformShuffleRel          Capability = 66
	CapabilityGroupNonUniformQuad                Capability = 68
	CapabilityGeometry                           Capability = 2
	CapabilitySubgroupBallotKHR                  Capability = 4423
	CapabilityInt64ImageEXT                      Capability = 5016
)

// Options configures SPIR-V generation.
type Options struct {
	// Version is the SPIR-V version of the module. The zero value is 1.0,
	// the only version every Vulkan device accepts without knowing the
	// device, as Rust naga's default; wgpu-hal derives it from the device
	// (Vulkan 1.0 → 1.0, 1.1 → 1.3, 1.2 → 1.5, 1.3 → 1.6, 1.4 with
	// VK_KHR_spirv_1_4). The backend never raises it: a feature that needs a
	// newer version (subgroup operations need 1.3) is an error, as Rust
	// naga's SpirvVersionTooLow. Below 1.4, values copied between the
	// layout-free Workgroup types and the decorated ones are copied member by
	// member instead of with OpCopyLogical.
	Version Version

	// ExtraCapabilities are declared in addition to those the module needs
	// (no Rust naga counterpart).
	ExtraCapabilities []Capability

	// Debug includes debug information.
	Debug bool

	// UseStorageInputOutput16 passes f16 entry point inputs and outputs as
	// f16 with the StorageInputOutput16 capability. The zero value converts
	// them to and from f32, which every device accepts; the capability needs
	// SHADER_F16 and device support, so it is an opt-in. Rust naga's
	// use_storage_input_output_16 defaults to true; wgpu-hal passes whether
	// the device supports it.
	UseStorageInputOutput16 bool

	// ForcePointSize adds a BuiltIn PointSize output variable with value 1.0.
	ForcePointSize bool

	// AdjustCoordinateSpace negates the y of every Position output after the
	// entry point stores it (Rust naga's ADJUST_COORDINATE_SPACE). Vulkan
	// flips y with a negative viewport height, so the zero value leaves
	// positions as they are; set it for a device that ignores a negative
	// height (wgpu-hal's IGNORED_NEGATIVE_VIEWPORT_HEIGHT workaround). Rust's
	// Options::default() has it on.
	AdjustCoordinateSpace bool

	// SkipLoopBounding leaves loops as they are. By default every loop gets
	// a decrementing 64-bit counter that breaks out of it, so the driver's
	// shader compiler cannot prove the loop endless. An endless loop is
	// undefined behavior there: the compiler may delete it, and may remove
	// bounds checks elsewhere in the function on the strength of it. Rust naga's
	// force_loop_bounding defaults to true, and wgpu-hal passes true.
	SkipLoopBounding bool

	// SkipIntDivChecks writes integer division and remainder as the raw
	// OpSDiv, OpUDiv and OpUMod. By default every `/` and `%` on an integer
	// goes through a helper function that replaces a zero divisor (and, for
	// signed types, the INT_MIN / -1 overflow) with 1, which is what WGSL
	// requires: x / 0 is x, x % 0 is 0, INT_MIN / -1 is INT_MIN. Raw
	// instructions give implementation-defined results in those cases, so set
	// this only for shaders whose divisors are never zero or -1. Rust naga's
	// emit_int_div_checks defaults to true, and wgpu-hal passes the
	// shader runtime checks' int_div_checks, on by default.
	//
	// Signed `%` keeps its helper even with this set: OpSRem is poison for a
	// negative operand in Vulkan without VK_KHR_maintenance8, so the helper
	// writes it as a - b * (a / b), as Rust naga does regardless of
	// emit_int_div_checks. OpSRem is never written.
	SkipIntDivChecks bool

	// BoundsCheckPolicies selects how out-of-bounds indices and image loads
	// are handled. The zero value is backend.BoundsCheckRestrict everywhere,
	// which WebGPU requires; Rust naga's default is Unchecked, and wgpu-hal
	// passes the policies of the device. Every field honors every policy:
	// Restrict with OpExtInst UMin (index) and clamped coordinates (image
	// loads), ReadZeroSkipWrite with a comparison and a selection, Unchecked
	// with no code. The exceptions: an unbounded binding array under
	// Restrict or ReadZeroSkipWrite needs its size in BindingMap
	// (BindingArraySize), and ReadZeroSkipWrite cannot guard a runtime-sized
	// array or a binding array of acceleration structures; Compile fails
	// then.
	BoundsCheckPolicies backend.BoundsCheckPolicies

	// EntryPoint selects the one entry point the module gets, as Rust
	// naga's spv::PipelineOptions: the module then leaves out the globals
	// that entry point does not use and the functions it cannot call. nil
	// writes every entry point. One the module does not have is an error
	// wrapping backend.ErrEntryPointNotFound.
	EntryPoint *backend.EntryPointSelector

	// AllowedCapabilities is the set of capabilities the module may use, as
	// Rust naga's Options::capabilities. It is consulted only where an
	// operation has a fallback: the packed dot product (dot4I8Packed,
	// dot4U8Packed) uses its native opcodes only when both DotProduct and
	// DotProductInput4x8BitPacked are in the set, and falls back to emulation
	// otherwise. A capability the module needs for its types, builtins or
	// memory is declared whether or not the set lists it (f16, i64,
	// DrawParameters, RayQueryKHR, …), as Rust naga's capabilities_used table
	// does; the set is not a rejection list. nil allows every capability; an
	// empty, non-nil set allows none of the fallback operations.
	AllowedCapabilities CapabilitySet

	// SkipRayQueryInitTracking leaves out the state variable every ray query
	// gets to prevent misuse: by default rayQueryInitialize checks its ray
	// (tmin <= tmax and the like) and skips the query when it is invalid, and
	// rayQueryProceed does nothing on a query that was not initialized. Rust
	// naga's ray_query_initialization_tracking defaults to true, and
	// wgpu-hal passes true.
	SkipRayQueryInitTracking bool

	// BindingMap maps WGSL @group/@binding to the SPIR-V DescriptorSet and
	// Binding decorations, and can give a binding array its size. Mirrors
	// Rust naga's spv::Options::binding_map. A nil map keeps every resource
	// at its WGSL @group/@binding. A non-nil map, empty too, must list every
	// resource the module declares: one without an entry is an error
	// wrapping backend.ErrMissingBinding, unless FakeMissingBindings is set.
	BindingMap map[ir.ResourceBinding]BindingInfo

	// FakeMissingBindings keeps a resource that a non-nil BindingMap leaves
	// out at its WGSL @group/@binding instead of failing. Rust naga's
	// fake_missing_bindings defaults to true; wgpu-hal passes false, the
	// zero value here.
	FakeMissingBindings bool

	// ZeroInitializeWorkgroupMemory is how compute entry points zero their
	// workgroup variables, as WebGPU requires. The zero value is
	// backend.ZeroInitWorkgroupPolyfill, which works on every device; every
	// mode is honored.
	ZeroInitializeWorkgroupMemory backend.ZeroInitializeWorkgroupMemoryMode

	// ClampFragDepth, when set, clamps the depth a fragment entry point
	// writes to the range in two f32 push constants, min then max, at byte
	// offset ClampFragDepth.Offset, for which the module declares a push
	// constant block: WebGPU clamps frag_depth to the viewport's depth
	// range, and Vulkan doesn't. The caller pushes the viewport's minDepth
	// and maxDepth there. Rust naga's CLAMP_FRAG_DEPTH clamps to 0..1;
	// Dawn's ClampFragDepth transform reads the range as this does. An
	// entry point that writes frag_depth and has push constants of its own
	// finds the range in its own block instead, members appended after its
	// data, as an entry point has at most one push constant block, so Offset
	// has to be past that data. Every stage that uses such a block then has
	// the range in its block too.
	ClampFragDepth *backend.ClampFragDepth

	// DemoteToHelperInvocation writes WGSL discard as
	// OpDemoteToHelperInvocation, which makes the invocation a helper
	// invocation: derivatives in the quad stay defined after a discard, as WGSL
	// requires. The zero value writes OpKill, which ends the invocation and is
	// what Rust naga always emits (back/spv/block.rs); Tint selects the demote
	// with extensions.use_demote_to_helper_invocation. The instruction is core
	// in SPIR-V 1.6; below it the module declares the
	// SPV_EXT_demote_to_helper_invocation extension. Vulkan needs the
	// shaderDemoteToHelperInvocation feature (core in Vulkan 1.3), so set this
	// only from a device that has it.
	DemoteToHelperInvocation bool
}

// BindingInfo is where a resource goes in SPIR-V. Mirrors Rust naga's
// spv::BindingInfo.
type BindingInfo struct {
	// DescriptorSet is the SPIR-V DescriptorSet decoration.
	DescriptorSet uint32
	// Binding is the SPIR-V Binding decoration.
	Binding uint32
	// BindingArraySize, when set on a binding array, declares it with this
	// many elements instead of its WGSL size. It is also the length bounds
	// checks use for an unbounded binding array, which otherwise has none.
	BindingArraySize *uint32
}

// DefaultOptions returns the zero Options, the safe defaults: SPIR-V 1.0,
// Restrict bounds checks, bounded loops, f16 inputs and outputs through f32.
func DefaultOptions() Options {
	return Options{}
}

// TranslationInfo describes what Compile wrote.
type TranslationInfo struct {
	// EntryPointNames maps the name of each entry point the module holds to
	// its OpEntryPoint name, the same in SPIR-V.
	EntryPointNames map[string]string
}

// Compile translates module to a SPIR-V module, returned as its
// little-endian 32-bit words (Rust naga's back::spv::write_vec, as bytes).
func Compile(module *ir.Module, options Options) ([]byte, TranslationInfo, error) {
	// With an entry point selected the writer keeps what
	// entrypoint.NewFilter keeps (Rust naga's write_logical_layout).
	ep, err := entrypoint.Index(module, options.EntryPoint, false)
	if err != nil {
		return nil, TranslationInfo{}, fmt.Errorf("spirv: %w", err)
	}
	out := unsupported.FilteredOutput(module, ep)
	if err := unsupported.OutputCapabilities("spirv", module, out, SupportedCapabilities()); err != nil {
		return nil, TranslationInfo{}, err
	}
	binary, err := codegen.NewBackend(toCodegenOptions(options)).Compile(module)
	if err != nil {
		return nil, TranslationInfo{}, err
	}
	info := TranslationInfo{EntryPointNames: make(map[string]string)}
	for _, ep := range module.EntryPoints {
		// The backend does not write mesh and task entry points yet.
		if ep.Stage == ir.StageMesh || ep.Stage == ir.StageTask {
			continue
		}
		if sel := options.EntryPoint; sel == nil || (sel.Name == ep.Name && sel.Stage == ep.Stage) {
			info.EntryPointNames[ep.Name] = ep.Name
		}
	}
	return binary, info, nil
}

// --- Internal conversion ---

// toCodegenOptions converts public Options to internal codegen Options.
func toCodegenOptions(o Options) codegen.Options {
	return codegen.Options{
		Version: codegen.Version{
			Major: o.Version.Major,
			Minor: o.Version.Minor,
		},
		ExtraCapabilities:       toCodegenCapabilities(o.ExtraCapabilities),
		Debug:                   o.Debug,
		UseStorageInputOutput16: o.UseStorageInputOutput16,
		ForcePointSize:          o.ForcePointSize,
		AdjustCoordinateSpace:   o.AdjustCoordinateSpace,
		ForceLoopBounding:       !o.SkipLoopBounding,
		SkipIntDivChecks:        o.SkipIntDivChecks,
		BoundsCheckPolicies:     o.BoundsCheckPolicies,
		EntryPoint:              o.EntryPoint,
		AllowedCapabilities:     toCodegenCapabilitySet(o.AllowedCapabilities),
		RayQueryInitTracking:    !o.SkipRayQueryInitTracking,
		BindingMap:              toCodegenBindingMap(o.BindingMap),
		FakeMissingBindings:     o.FakeMissingBindings,

		ZeroInitializeWorkgroupMemory: o.ZeroInitializeWorkgroupMemory,
		ClampFragDepth:                o.ClampFragDepth,
		DemoteToHelperInvocation:      o.DemoteToHelperInvocation,
	}
}

func toCodegenCapabilities(caps []Capability) []spvenums.Capability {
	if caps == nil {
		return nil
	}
	out := make([]spvenums.Capability, len(caps))
	for i, c := range caps {
		out[i] = spvenums.Capability(c)
	}
	return out
}

// toCodegenCapabilitySet keeps nil nil: nil allows every capability.
func toCodegenCapabilitySet(set map[Capability]struct{}) map[spvenums.Capability]struct{} {
	if set == nil {
		return nil
	}
	out := make(map[spvenums.Capability]struct{}, len(set))
	for c := range set {
		out[spvenums.Capability(c)] = struct{}{}
	}
	return out
}

func toCodegenBindingMap(m map[ir.ResourceBinding]BindingInfo) map[ir.ResourceBinding]codegen.BindingInfo {
	if m == nil {
		return nil
	}
	out := make(map[ir.ResourceBinding]codegen.BindingInfo, len(m))
	for k, v := range m {
		info := codegen.BindingInfo{DescriptorSet: v.DescriptorSet, Binding: v.Binding}
		if v.BindingArraySize != nil {
			size := *v.BindingArraySize
			info.BindingArraySize = &size
		}
		out[k] = info
	}
	return out
}
