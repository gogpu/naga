package spirv

import (
	"testing"

	"github.com/gogpu/naga/ir"
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
	opts.BoundsCheckPolicies.ImageLoad = BoundsCheckUnchecked

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)
	assertNoCapability(t, caps, CapabilityImageQuery)
}

// TestImageBoundsCheck_Restrict verifies Restrict adds ImageQuery and clamping.
func TestImageBoundsCheck_Restrict(t *testing.T) {
	module := makeImageLoadModule()
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.ImageLoad = BoundsCheckRestrict

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, CapabilityImageQuery)

	if !divmodHasOpcode(spvBytes, OpImageQuerySizeLod) {
		t.Error("Restrict should emit OpImageQuerySizeLod for coordinate clamping")
	}
	if !divmodHasOpcode(spvBytes, OpImageQueryLevels) {
		t.Error("Restrict should emit OpImageQueryLevels for level clamping")
	}
	if !divmodHasOpcode(spvBytes, OpImageFetch) {
		t.Error("Restrict should emit OpImageFetch")
	}
}

// TestImageBoundsCheck_RZSW verifies ReadZeroSkipWrite adds ImageQuery and branching.
func TestImageBoundsCheck_RZSW(t *testing.T) {
	module := makeImageLoadModule()
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.ImageLoad = BoundsCheckReadZeroSkipWrite

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)
	assertCapability(t, caps, CapabilityImageQuery)

	if !divmodHasOpcode(spvBytes, OpImageQuerySizeLod) {
		t.Error("RZSW should emit OpImageQuerySizeLod for bounds checking")
	}
	if !divmodHasOpcode(spvBytes, OpSelectionMerge) {
		t.Error("RZSW should emit OpSelectionMerge for conditional load")
	}
	if !divmodHasOpcode(spvBytes, OpPhi) {
		t.Error("RZSW should emit OpPhi for merging zero/loaded values")
	}
}
