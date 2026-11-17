package codegen

// The subgroup forms ADR-014 added to the IR: elect and all-equal, written as
// the GLSL builtins of the basic and vote families.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestGLSL_SubgroupElect: subgroupElect is a basic-family builtin; the
// backend's feature preamble already requests GL_KHR_shader_subgroup_basic.
func TestGLSL_SubgroupElect(t *testing.T) {
	source, _, err := Compile(glslElectModule(), Options{LangVersion: Version450})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(source, "subgroupElect();") {
		t.Errorf("output lacks subgroupElect():\n%s", source)
	}
}

// TestGLSL_SubgroupAllEqual: subgroupAllEqual is a vote-family builtin, and its
// result is the bool the IR expression has even though the argument is a uint.
func TestGLSL_SubgroupAllEqual(t *testing.T) {
	source, _, err := Compile(glslAllEqualModule(), Options{LangVersion: Version450})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(source, "subgroupAllEqual(") {
		t.Errorf("output lacks subgroupAllEqual(:\n%s", source)
	}
	// The result is declared bool, not the argument's uint.
	if !strings.Contains(source, "bool _e") {
		t.Errorf("the all-equal result is not a bool:\n%s", source)
	}
}

// TestGLSL_SubgroupBallotBit: the seven ballot bit forms are written as the
// ballot family's builtins over the ballot mask, with a bool result for
// extract and inverse ballot and uint for the counts and finds (card
// P2-IR-039, ADR-014 D3).
func TestGLSL_SubgroupBallotBit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		op         ir.SubgroupBallotBitOperation
		call       string
		boolResult bool
	}{
		{"extract", ir.SubgroupBallotBitExtract, "subgroupBallotBitExtract(_e0, 1u)", true},
		{"count", ir.SubgroupBallotBitCount, "subgroupBallotBitCount(_e0)", false},
		{"inclusive count", ir.SubgroupBallotBitInclusiveCount, "subgroupBallotInclusiveBitCount(_e0)", false},
		{"exclusive count", ir.SubgroupBallotBitExclusiveCount, "subgroupBallotExclusiveBitCount(_e0)", false},
		{"find lsb", ir.SubgroupBallotFindLSB, "subgroupBallotFindLSB(_e0)", false},
		{"find msb", ir.SubgroupBallotFindMSB, "subgroupBallotFindMSB(_e0)", false},
		{"inverse ballot", ir.SubgroupInverseBallot, "subgroupInverseBallot(_e0)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, _, err := Compile(glslBallotBitModule(tc.op), Options{LangVersion: Version450})
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if !strings.Contains(source, tc.call) {
				t.Errorf("output lacks %s:\n%s", tc.call, source)
			}
			decl := "uint _e"
			if tc.boolResult {
				decl = "bool _e"
			}
			if !strings.Contains(source, decl) {
				t.Errorf("the result is not declared %q:\n%s", decl, source)
			}
			if !strings.Contains(source, "#extension GL_KHR_shader_subgroup_ballot : require") {
				t.Errorf("output lacks the ballot extension:\n%s", source)
			}
		})
	}
}

// glslBallotBitModule builds a compute entry point with a ballot followed by
// one ballot bit operation over its mask: expr 0 the mask, expr 1 a u32
// index, expr 2 the result (bool for extract/inverse ballot, u32 otherwise).
func glslBallotBitModule(op ir.SubgroupBallotBitOperation) *ir.Module {
	resultType := ir.TypeHandle(0) // u32
	if op == ir.SubgroupBallotBitExtract || op == ir.SubgroupInverseBallot {
		resultType = 1 // bool
	}
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprSubgroupBallotResult{}},                    // [0] mask
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},              // [1] index
					{Kind: ir.ExprSubgroupOperationResult{Type: resultType}}, // [2] result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupBallot{Result: 0}},
					{Kind: ir.StmtSubgroupBallotBit{Op: op, Mask: 0, Index: 1, Result: 2}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}

// glslElectModule builds a compute entry point whose only statement is
// subgroupElect, with a bool result expression.
func glslElectModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name:        "main",
				Expressions: []ir.Expression{{Kind: ir.ExprSubgroupOperationResult{Type: 0}}},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupElect{Result: 0}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}

// glslAllEqualModule builds a compute entry point whose only statement is one
// subgroup all-equal over a uint literal, with a bool result.
func glslAllEqualModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},     // [0] argument
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
