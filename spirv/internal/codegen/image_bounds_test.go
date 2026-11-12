package codegen

import (
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// makeImageLoadModule creates a minimal IR module with a fragment shader that
// does textureLoad on a 2D texture. Used to test bounds check policy effects.
func makeImageLoadModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                         // [0] f32
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                         // [1] vec4<f32>
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},                          // [2] vec2<i32>
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                                          // [3] i32
			{Inner: ir.ImageType{Dim: ir.Dim2D, Arrayed: false, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}}, // [4] texture_2d<f32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Type: 4, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},                                 // [0] &tex
						{Kind: ir.Literal{Value: ir.LiteralI32(5)}},                                // [1] 5i
						{Kind: ir.Literal{Value: ir.LiteralI32(10)}},                               // [2] 10i
						{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{1, 2}}},   // [3] vec2(5,10)
						{Kind: ir.Literal{Value: ir.LiteralI32(0)}},                                // [4] level 0
						{Kind: ir.ExprImageLoad{Image: 0, Coordinate: 3, Level: ptrExprHandle(4)}}, // [5] textureLoad
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
					},
				},
			},
		},
	}
}

// TestImageBoundsCheck_Unchecked verifies no ImageQuery with Unchecked policy.
func TestImageBoundsCheck_Unchecked(t *testing.T) {
	module := makeImageLoadModule()
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.ImageLoad = backend.BoundsCheckUnchecked

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)
	assertNoCapability(t, caps, spvenums.CapabilityImageQuery)
}

// TestImageBoundsCheck_Restrict verifies Restrict adds ImageQuery and clamping.
func TestImageBoundsCheck_Restrict(t *testing.T) {
	module := makeImageLoadModule()
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.ImageLoad = backend.BoundsCheckRestrict

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, spvenums.CapabilityImageQuery)

	if !divmodHasOpcode(spvBytes, spvenums.OpImageQuerySizeLod) {
		t.Error("Restrict should emit OpImageQuerySizeLod for coordinate clamping")
	}
	if !divmodHasOpcode(spvBytes, spvenums.OpImageQueryLevels) {
		t.Error("Restrict should emit OpImageQueryLevels for level clamping")
	}
	if !divmodHasOpcode(spvBytes, spvenums.OpImageFetch) {
		t.Error("Restrict should emit OpImageFetch")
	}
}

// TestImageBoundsCheck_RZSW verifies ReadZeroSkipWrite adds ImageQuery and branching.
func TestImageBoundsCheck_RZSW(t *testing.T) {
	module := makeImageLoadModule()
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.ImageLoad = backend.BoundsCheckReadZeroSkipWrite

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, spvenums.CapabilityImageQuery)

	if !divmodHasOpcode(spvBytes, spvenums.OpImageQuerySizeLod) {
		t.Error("RZSW should emit OpImageQuerySizeLod for bounds checking")
	}
	if !divmodHasOpcode(spvBytes, spvenums.OpSelectionMerge) {
		t.Error("RZSW should emit OpSelectionMerge for conditional load")
	}
	if !divmodHasOpcode(spvBytes, spvenums.OpPhi) {
		t.Error("RZSW should emit OpPhi for merging zero/loaded values")
	}
}

// TestImageBoundsCheckCoordinateSigns: a checked textureLoad is valid
// SPIR-V for i32 and u32 coordinates, on sampled, arrayed, multisampled, 1D
// and storage textures. Two bugs that the Restrict default exposed:
//   - Restrict subtracts a vector of ones from the image size; its
//     constituents must have the coordinates' type. Rust naga builds it from
//     an i32 one (get_scope_constant), which spirv-val rejects in a uvec2
//     (OpConstantComposite), so u32 coordinates were invalid there too.
//   - ReadZeroSkipWrite put the OpSelectionMerge only on the level check, so
//     an image without a level (multisampled, storage) branched without one.
//     Rust's Selection::if_true puts it on the first check, whichever it is.
func TestImageBoundsCheckCoordinateSigns(t *testing.T) {
	for _, policy := range []backend.BoundsCheckPolicy{backend.BoundsCheckRestrict, backend.BoundsCheckReadZeroSkipWrite} {
		for _, scalar := range []string{"i32", "u32"} {
			source := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var arr: texture_2d_array<f32>;
@group(0) @binding(2) var ms: texture_multisampled_2d<f32>;
@group(0) @binding(3) var line: texture_1d<f32>;
@group(0) @binding(4) var st: texture_storage_2d<r32float, read>;
@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let c = vec2<` + scalar + `>(p.xy);
    return textureLoad(tex, c, ` + scalar + `(1)) + textureLoad(arr, c, ` + scalar + `(2), ` + scalar + `(0))
        + textureLoad(ms, c, ` + scalar + `(3)) + textureLoad(line, ` + scalar + `(p.x), ` + scalar + `(0))
        + textureLoad(st, c);
}`
			opts := Options{Version: Version1_3, BoundsCheckPolicies: backend.BoundsCheckPolicies{ImageLoad: policy}}
			t.Run(policy.String()+"/"+scalar, func(t *testing.T) {
				validateWithVulkanSDK(t, compileWGSLForCapabilityTestWithOpts(t, source, opts))
			})
		}
	}
}

// TestImageBoundsCheckExternalTexture: textureLoad on a texture_external has
// no level, and OpImageQuerySize needs a multisampled or storage image, so a
// checked load queries the size of level 0 with OpImageQuerySizeLod (the
// level the unchecked OpImageFetch reads). Rust naga has no SPIR-V
// texture_external (back/spv/image.rs: unimplemented).
func TestImageBoundsCheckExternalTexture(t *testing.T) {
	const source = `
@group(0) @binding(0) var ext: texture_external;
@fragment fn main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    return textureLoad(ext, vec2<u32>(p.xy)) + textureLoad(ext, vec2<i32>(p.xy));
}`
	for _, policy := range []backend.BoundsCheckPolicy{backend.BoundsCheckRestrict, backend.BoundsCheckReadZeroSkipWrite, backend.BoundsCheckUnchecked} {
		t.Run(policy.String(), func(t *testing.T) {
			opts := Options{Version: Version1_3, BoundsCheckPolicies: backend.BoundsCheckPolicies{ImageLoad: policy}}
			validateWithVulkanSDK(t, compileWGSLForCapabilityTestWithOpts(t, source, opts))
		})
	}
}
