package valid_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// lodQueryModule builds a module with an image and a sampler global, and one
// ImageQueryLod (GLSL's textureQueryLod) in the entry point named main at
// stage. The module is hand-built because no text frontend produces a lod query
// in a non-fragment stage: WGSL has no such query and the GLSL frontend refuses
// it outside a fragment shader, which leaves the validator as the only place
// the stage rule can be pinned.
func lodQueryModule(stage ir.ShaderStage) *ir.Module {
	lod := ir.ExpressionHandle(4)
	return &ir.Module{
		Types: []ir.Type{
			{Name: "tex", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Name: "samp", Inner: ir.SamplerType{}},
			{Name: "lod", Inner: ir.VectorType{Size: ir.Vec2,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Type: 0, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
			{Name: "samp", Space: ir.SpaceHandle, Type: 1, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     stage,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Name:      "main",
				LocalVars: []ir.LocalVariable{{Name: "lod", Type: 2}},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprGlobalVariable{Variable: 1}},
					{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
					{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 2}},
					{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQueryLod{Sampler: 1, Coordinate: 3}}},
					{Kind: ir.ExprLocalVariable{Variable: 0}},
				},
				Body: []ir.Statement{
					// A literal and a global are pre-emit expressions; the Emit
					// statement introduces the splat and the query only.
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
					{Kind: ir.StmtStore{Pointer: 5, Value: lod}},
				},
			},
		}},
	}
}

// TestImageQueryLodFragmentOnly: SPIR-V defines OpImageQueryLod for the
// Fragment execution model only ("This instruction is only valid in the
// Fragment Execution Model", SPIR-V 1.6 §3.32.13), so a module whose lod query
// is reachable from a non-fragment entry point is invalid even though no
// capability is missing. Rust naga's validator has no such check because its
// IR has no lod query; the stage rule is ours, and it mirrors the GLSL
// frontend's refusal of the call outside a fragment shader (glslang declares no
// overload there).
func TestImageQueryLodFragmentOnly(t *testing.T) {
	expectValid(t, lodQueryModule(ir.StageFragment), valid.Options{Capabilities: valid.CapAll})
	for _, tt := range []struct {
		stage ir.ShaderStage
		want  string
	}{
		{ir.StageVertex, "Entry point main at Vertex is invalid: Uses operations forbidden at this stage"},
		{ir.StageCompute, "Entry point main at Compute is invalid: Uses operations forbidden at this stage"},
	} {
		expectRejected(t, lodQueryModule(tt.stage), valid.Options{Capabilities: valid.CapAll}, tt.want)
	}
}

// TestImageQueryLodReachedThroughCall: a helper function that contains the lod
// query is fragment-only, so a non-fragment entry point that calls it is
// invalid; a fragment one is not.
func TestImageQueryLodReachedThroughCall(t *testing.T) {
	module := func(stage ir.ShaderStage) *ir.Module {
		lod := ir.ExpressionHandle(4)
		helper := ir.Function{
			Name:   "lodOf",
			Result: &ir.FunctionResult{Type: 2},
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},
				{Kind: ir.ExprGlobalVariable{Variable: 1}},
				{Kind: ir.Literal{Value: ir.LiteralF32(0)}},
				{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 2}},
				{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQueryLod{Sampler: 1, Coordinate: 3}}},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
				{Kind: ir.StmtReturn{Value: &lod}},
			},
		}
		return &ir.Module{
			Types: []ir.Type{
				{Name: "tex", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
				{Name: "samp", Inner: ir.SamplerType{}},
				{Name: "lod", Inner: ir.VectorType{Size: ir.Vec2,
					Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			},
			GlobalVariables: []ir.GlobalVariable{
				{Name: "tex", Space: ir.SpaceHandle, Type: 0, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}},
				{Name: "samp", Space: ir.SpaceHandle, Type: 1, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}},
			},
			Functions: []ir.Function{helper},
			EntryPoints: []ir.EntryPoint{{
				Name:      "main",
				Stage:     stage,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Name:      "main",
					LocalVars: []ir.LocalVariable{{Name: "lod", Type: 2}},
					Expressions: []ir.Expression{
						{Kind: ir.ExprCallResult{Function: 0}},
						{Kind: ir.ExprLocalVariable{Variable: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtCall{Function: 0, Result: exprHandle(0)}},
						// The local-variable expression is pre-emit; only the
						// call result and the store remain.
						{Kind: ir.StmtStore{Pointer: 1, Value: 0}},
					},
				},
			}},
		}
	}
	expectValid(t, module(ir.StageFragment), valid.Options{Capabilities: valid.CapAll})
	expectRejected(t, module(ir.StageCompute), valid.Options{Capabilities: valid.CapAll},
		"Entry point main at Compute is invalid: Uses operations forbidden at this stage")
}

// TestImageQueryLodHandleChecks: the lod query carries its sampler and its
// coordinate, and a handle that does not exist is reported with the same
// wording the other image-query operands use.
func TestImageQueryLodHandleChecks(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{{
			Name: "fn",
			Expressions: []ir.Expression{
				{Kind: ir.ExprImageQuery{
					Image: ir.ExpressionHandle(999),
					Query: ir.ImageQueryLod{
						Sampler:    ir.ExpressionHandle(998),
						Coordinate: ir.ExpressionHandle(997),
					},
				}},
			},
		}},
	}
	_, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll})
	if err == nil {
		t.Fatal("validated; the missing handles must be reported")
	}
	texts := errorTexts(err)
	for _, want := range []string{
		"image expression 999 does not exist",
		"sampler expression 998 does not exist",
		"coordinate expression 997 does not exist",
	} {
		found := false
		for _, text := range texts {
			if strings.Contains(text, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("errors %q; want one containing %q", texts, want)
		}
	}
}

// exprHandle returns a pointer to h, for the optional statement operands.
func exprHandle(h ir.ExpressionHandle) *ir.ExpressionHandle { return &h }
