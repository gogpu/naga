package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestMSL_EntryPointPrologueOrder pins the order of an entry point's prologue.
// Rust naga writes the private globals first (as local variables, since Metal
// has no private mutable globals), then reconstructs the stage-input aliases,
// then the function's own local variables (back/msl/writer.rs: the
// private-global block, the argument reconstruction, put_locals). The order is
// only visible when an entry point has both stage inputs and a private global;
// this test is the pin for the change that moved the alias emission after the
// private-global block.
func TestMSL_EntryPointPrologueOrder(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	var locationIn ir.Binding = ir.LocationBinding{Location: 0}
	var locationOut ir.Binding = ir.LocationBinding{Location: 0}
	retExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "counter", Type: tF32, Space: ir.SpacePrivate},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: ir.Function{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "color", Type: tF32, Binding: &locationIn},
				},
				Result: &ir.FunctionResult{Type: tF32, Binding: &locationOut},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprLoad{Pointer: 0}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		},
	}

	result := compileModule(t, module)

	private := strings.Index(result, "float counter")
	alias := strings.Index(result, "const auto color")
	if private < 0 || alias < 0 {
		t.Fatalf("prologue lines not found (private=%d alias=%d):\n%s", private, alias, result)
	}
	if private > alias {
		t.Fatalf("private global emitted after the input alias, want private globals first (private=%d alias=%d):\n%s",
			private, alias, result)
	}
}
