package codegen

// P1-GLSL-016: the GLSL subgroup functions have no prefix form for
// min/max/and/or/xor, so those scans are refused, never silently written as
// subgroupAdd (the previous default) or another operation. The Add/Mul scans
// have a GLSL builtin and stay written. The P0-MSL-111 silent-substitution
// class, the GLSL sibling of P1-DXIL-115.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestGLSL_SubgroupScanRefused: every min/max/and/or/xor scan (inclusive and
// exclusive) is refused. On the defective base funcName defaulted to
// "subgroupAdd", so the scan compiled to subgroupAdd(arg) — a silently wrong
// result reachable from SPIR-V input (the GLSL frontend refuses these scans).
func TestGLSL_SubgroupScanRefused(t *testing.T) {
	ops := []ir.SubgroupOperation{
		ir.SubgroupOperationMin, ir.SubgroupOperationMax,
		ir.SubgroupOperationAnd, ir.SubgroupOperationOr, ir.SubgroupOperationXor,
	}
	scans := []ir.CollectiveOperation{ir.CollectiveInclusiveScan, ir.CollectiveExclusiveScan}
	for _, op := range ops {
		for _, cop := range scans {
			if _, _, err := Compile(glslScanModule(op, cop), Options{LangVersion: Version450}); err == nil {
				t.Errorf("subgroup scan op=%d cop=%d compiled, want a refusal", op, cop)
			}
		}
	}
}

// TestGLSL_SubgroupArithmeticScanWritten guards the Add/Mul scans: they have a
// GLSL builtin (subgroupExclusive/InclusiveAdd/Mul) and must keep being written.
func TestGLSL_SubgroupArithmeticScanWritten(t *testing.T) {
	cases := []struct {
		op   ir.SubgroupOperation
		cop  ir.CollectiveOperation
		want string
	}{
		{ir.SubgroupOperationAdd, ir.CollectiveExclusiveScan, "subgroupExclusiveAdd("},
		{ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan, "subgroupInclusiveAdd("},
		{ir.SubgroupOperationMul, ir.CollectiveExclusiveScan, "subgroupExclusiveMul("},
		{ir.SubgroupOperationMul, ir.CollectiveInclusiveScan, "subgroupInclusiveMul("},
	}
	for _, tc := range cases {
		source, _, err := Compile(glslScanModule(tc.op, tc.cop), Options{LangVersion: Version450})
		if err != nil {
			t.Fatalf("Add/Mul scan op=%d cop=%d refused: %v", tc.op, tc.cop, err)
		}
		if !strings.Contains(source, tc.want) {
			t.Errorf("Add/Mul scan op=%d cop=%d: output lacks %s:\n%s", tc.op, tc.cop, tc.want, source)
		}
	}
}

// glslScanModule builds a compute entry point with one collective scan over a
// uint literal; the result type matches the argument (the per-component form).
func glslScanModule(op ir.SubgroupOperation, cop ir.CollectiveOperation) *ir.Module {
	return &ir.Module{
		Types: []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},     // [0] argument
					{Kind: ir.ExprSubgroupOperationResult{Type: 0}}, // [1] uint result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupCollectiveOperation{
						Op:           op,
						CollectiveOp: cop,
						Argument:     0,
						Result:       1,
					}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}
