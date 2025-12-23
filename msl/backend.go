package msl

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Version represents an MSL language version.
type Version struct {
	Major uint8
	Minor uint8
}

// Common MSL versions.
var (
	Version1_2 = Version{Major: 1, Minor: 2}
	Version2_0 = Version{Major: 2, Minor: 0}
	Version2_1 = Version{Major: 2, Minor: 1}
	Version2_3 = Version{Major: 2, Minor: 3}
	Version3_0 = Version{Major: 3, Minor: 0}
)

// String returns the version as "major.minor".
func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// BoundsCheckPolicy controls how out-of-bounds accesses are handled.
type BoundsCheckPolicy uint8

const (
	// BoundsCheckUnchecked performs no bounds checking.
	// Out-of-bounds accesses have undefined behavior.
	BoundsCheckUnchecked BoundsCheckPolicy = iota

	// BoundsCheckReadZeroSkipWrite returns zero for out-of-bounds reads
	// and skips out-of-bounds writes.
	BoundsCheckReadZeroSkipWrite

	// BoundsCheckRestrict clamps indices to valid range.
	BoundsCheckRestrict
)

// BoundsCheckPolicies configures bounds checking for different access types.
type BoundsCheckPolicies struct {
	// Index applies to array, vector, and matrix indexing.
	Index BoundsCheckPolicy

	// Buffer applies to buffer (storage/uniform) accesses.
	Buffer BoundsCheckPolicy

	// Image applies to texture read/write operations.
	Image BoundsCheckPolicy

	// BindingArray applies to binding array (texture array) indexing.
	BindingArray BoundsCheckPolicy
}

// DefaultBoundsCheckPolicies returns conservative bounds check policies.
func DefaultBoundsCheckPolicies() BoundsCheckPolicies {
	return BoundsCheckPolicies{
		Index:        BoundsCheckReadZeroSkipWrite,
		Buffer:       BoundsCheckReadZeroSkipWrite,
		Image:        BoundsCheckReadZeroSkipWrite,
		BindingArray: BoundsCheckReadZeroSkipWrite,
	}
}

// BindTarget specifies the Metal binding slots for a resource.
type BindTarget struct {
	// Buffer is the buffer binding slot. Nil if not bound as buffer.
	Buffer *uint8

	// Texture is the texture binding slot. Nil if not bound as texture.
	Texture *uint8

	// Sampler is the sampler binding slot. Nil if not bound as sampler.
	Sampler *uint8

	// Mutable indicates if this is a read-write resource.
	Mutable bool
}

// EntryPointResources maps WGSL resource bindings to Metal binding slots.
type EntryPointResources struct {
	// Resources maps (group, binding) pairs to Metal bind targets.
	Resources map[ir.ResourceBinding]BindTarget

	// PushConstantBuffer is the buffer slot for push constants.
	// Nil if push constants are not used.
	PushConstantBuffer *uint8

	// SizesBuffer is the buffer slot for runtime array sizes.
	// Required when using runtime-sized arrays.
	SizesBuffer *uint8
}

// Options configures MSL code generation.
type Options struct {
	// LangVersion is the target MSL version.
	// Defaults to Version2_1 if zero.
	LangVersion Version

	// PerEntryPointMap maps entry point names to their resource bindings.
	// If nil, bindings are auto-generated.
	PerEntryPointMap map[string]EntryPointResources

	// BoundsCheckPolicies controls bounds checking behavior.
	BoundsCheckPolicies BoundsCheckPolicies

	// ZeroInitializeWorkgroupMemory enables zero-initialization of
	// workgroup (threadgroup) memory at the start of compute shaders.
	// This adds overhead but ensures defined behavior.
	ZeroInitializeWorkgroupMemory bool

	// ForceLoopBounding adds loop iteration limits to prevent infinite loops.
	// Recommended for untrusted shaders.
	ForceLoopBounding bool

	// FakeMissingBindings generates placeholder bindings for resources
	// that are referenced but not in the PerEntryPointMap.
	FakeMissingBindings bool
}

// DefaultOptions returns sensible default options for MSL generation.
func DefaultOptions() Options {
	return Options{
		LangVersion:                   Version2_1,
		BoundsCheckPolicies:           DefaultBoundsCheckPolicies(),
		ZeroInitializeWorkgroupMemory: true,
		ForceLoopBounding:             true,
	}
}

// PipelineOptions configures options specific to a single pipeline/entry point.
type PipelineOptions struct {
	// EntryPoint specifies which entry point to compile.
	// If nil, all entry points are compiled.
	EntryPoint *EntryPointSelector

	// AllowAndForcePointSize forces point size output for vertex shaders.
	// Required for point primitive topology.
	AllowAndForcePointSize bool
}

// EntryPointSelector identifies a specific entry point.
type EntryPointSelector struct {
	Stage ir.ShaderStage
	Name  string
}

// TranslationInfo contains information about the compiled MSL output.
type TranslationInfo struct {
	// EntryPointNames maps original entry point names to generated MSL names.
	EntryPointNames map[string]string

	// RequiresSizesBuffer indicates if a sizes buffer is needed for
	// runtime-sized arrays.
	RequiresSizesBuffer bool
}

// Compile generates MSL source code from an IR module.
// Returns the MSL source as a string and translation info, or an error.
func Compile(module *ir.Module, options Options) (string, TranslationInfo, error) {
	return CompileWithPipeline(module, options, PipelineOptions{})
}

// CompileWithPipeline generates MSL source code with pipeline-specific options.
func CompileWithPipeline(module *ir.Module, options Options, pipeline PipelineOptions) (string, TranslationInfo, error) {
	// Apply defaults for zero values
	if options.LangVersion.Major == 0 {
		options.LangVersion = Version2_1
	}

	// Create writer
	w := newWriter(module, &options, &pipeline)

	// Generate MSL code
	if err := w.writeModule(); err != nil {
		return "", TranslationInfo{}, fmt.Errorf("msl: %w", err)
	}

	info := TranslationInfo{
		EntryPointNames:     w.entryPointNames,
		RequiresSizesBuffer: w.needsSizesBuffer,
	}

	return w.String(), info, nil
}
