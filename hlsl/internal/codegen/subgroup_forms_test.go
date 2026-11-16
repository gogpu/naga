package codegen

// The subgroup forms ADR-014 added to the IR: elect and all-equal. WGSL has
// subgroupElect but no all-equal, so the all-equal cases build the IR module
// directly.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestHLSL_SubgroupElect: subgroupElect lowers to WaveIsFirstLane (the WGSL
// subgroups proposal's Appendix B mapping), SM 6.0.
func TestHLSL_SubgroupElect(t *testing.T) {
	code := compileWGSLToHLSL(t, `enable subgroups;
@group(0) @binding(0) var<storage, read_write> o: array<u32, 1>;
@compute @workgroup_size(64) fn main() {
    if subgroupElect() { o[0] = 1u; }
}`, nil)
	mustContain(t, code, []string{"WaveIsFirstLane()"})
}

// TestHLSL_SubgroupAllEqual: subgroupAllEqual becomes WaveActiveAllEqual with
// the argument as it is — HLSL's overloads take the numeric scalars and their
// vectors (DXC DxilOperations.cpp, opcode 115).
func TestHLSL_SubgroupAllEqual(t *testing.T) {
	code, _, err := Compile(hlslAllEqualModule(false), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	mustContain(t, code, []string{"WaveActiveAllEqual("})
}

// TestHLSL_SubgroupAllEqualBool: a bool operand is passed as it is and relies
// on HLSL's bool-to-int conversion, because the intrinsic's overloads are the
// numeric types.
func TestHLSL_SubgroupAllEqualBool(t *testing.T) {
	code, _, err := Compile(hlslAllEqualModule(true), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	mustContain(t, code, []string{"WaveActiveAllEqual("})
	if strings.Contains(code, "WaveActiveAllEqual((uint)") {
		t.Errorf("the bool operand is cast, want the conversion left to HLSL:\n%s", code)
	}
}

// hlslAllEqualModule builds a compute entry point whose only statement is one
// subgroup all-equal over true (boolOperand) or 1u, with a bool result.
func hlslAllEqualModule(boolOperand bool) *ir.Module {
	types := []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}
	argument := ir.Literal{Value: ir.LiteralU32(1)}
	if boolOperand {
		types[0] = ir.Type{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}
		argument = ir.Literal{Value: ir.LiteralBool(true)}
	}
	types = append(types, ir.Type{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}})
	return &ir.Module{
		Types: types,
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: argument}, // [0] argument
					{Kind: ir.ExprSubgroupOperationResult{Type: 1}}, // [1] bool result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupCollectiveOperation{
						Op:           ir.SubgroupOperationAllEqual,
						CollectiveOp: ir.CollectiveReduce,
						Argument:     0,
						Result:       1,
					}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}
