package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestGLSL_ImageQueryLodVersionGate pins the version rule of the LOD query in
// the output backend. textureQueryLod is core in desktop GLSL 4.00 and absent
// from every GLSL ES version, so a 3.30 target is refused: glslangValidator
// happens to accept the function at 3.30 as well, a leniency the specification
// does not share, and writing it anyway gives GLSL the driver rejects (the
// same rule FeatureDrawIndex follows below 4.60).
func TestGLSL_ImageQueryLodVersionGate(t *testing.T) {
	module := lodQueryModule()

	opts := DefaultOptions()
	opts.LangVersion = Version330
	_, _, err := Compile(module, opts)
	if err == nil {
		t.Fatal("Compile at GLSL 330 succeeded, want a missing-feature refusal")
	}
	if !strings.Contains(err.Error(), "TEXTURE_QUERY_LOD") {
		t.Errorf("error = %v, want it to name TEXTURE_QUERY_LOD", err)
	}

	opts.LangVersion = Version400
	if _, _, err := Compile(module, opts); err != nil {
		t.Fatalf("Compile at GLSL 400 failed: %v", err)
	}
}

// lodQueryModule builds the smallest module that requires the LOD query: a
// sampled 2-D image and a sampler global, a fragment entry point taking the
// coordinate, and the query itself as the returned value.
func lodQueryModule() *ir.Module {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	tImg := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(3)

	outBinding := ir.Binding(ir.LocationBinding{Location: 0})
	locBinding := func(loc uint32) *ir.Binding {
		b := ir.Binding(ir.LocationBinding{Location: loc})
		return &b
	}

	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Name: "", Inner: f32},
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: f32}},
			{Name: "", Inner: ir.SamplerType{}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: tImg},
			{Name: "smp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 3},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "fs_main",
			Stage: ir.StageFragment,
			Function: ir.Function{
				Name:      "fs_main",
				Arguments: []ir.FunctionArgument{{Name: "uv", Type: 2, Binding: locBinding(0)}},
				Result:    &ir.FunctionResult{Type: 2, Binding: &outBinding},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},  // [0] uv
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // [1] tex
					{Kind: ir.ExprGlobalVariable{Variable: 1}}, // [2] smp
					{Kind: ir.ExprImageQuery{ // [3] the LOD query
						Image: 1,
						Query: ir.ImageQueryLod{Sampler: 2, Coordinate: 0},
					}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		}},
	}
}
