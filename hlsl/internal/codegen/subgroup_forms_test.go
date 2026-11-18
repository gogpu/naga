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

// TestHLSL_SubgroupClusteredRefused: the HLSL wave intrinsics have no
// clustered form before the SM 6.5 multi-prefix operations, and the writer
// targets SM 6.0, so a clustered reduction is refused with a named limitation
// rather than written as the full-wave one (card P2-IR-040, ADR-014 D4).
func TestHLSL_SubgroupClusteredRefused(t *testing.T) {
	mod := hlslAllEqualModule(false)
	mod.EntryPoints[0].Function.Body[0].Kind = ir.StmtSubgroupCollectiveOperation{
		Op:           ir.SubgroupOperationAdd,
		CollectiveOp: ir.CollectiveOperationClusteredReduce,
		Argument:     0,
		Result:       1,
		ClusterSize:  4,
	}
	_, _, err := Compile(mod, DefaultOptions())
	if err == nil {
		t.Fatal("a clustered reduction compiled, want a refusal")
	}
	if !strings.Contains(err.Error(), "clustered") {
		t.Errorf("error = %v, want the clustered-reduction refusal", err)
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

// TestHLSL_SubgroupBallotBit: the ballot bit forms are arithmetic over the
// four mask words — countbits, firstbitlow / firstbithigh and shifts — with
// the count and find forms masked to the subgroup size (card P2-IR-039,
// ADR-014 D3). The wave count intrinsics read the live wave's predicate, not
// a mask value, so they must not appear: they would silently change the
// result for any mask other than the current ballot.
func TestHLSL_SubgroupBallotBit(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   ir.SubgroupBallotBitOperation
		want []string
	}{
		{"extract", ir.SubgroupBallotBitExtract, []string{
			"const uint4 _e2_b", "const uint _e2_i",
			"const bool _e2 = ((_e2_b[_e2_i / 32u] >> (_e2_i % 32u)) & 1u) != 0u;",
		}},
		{"inverse ballot", ir.SubgroupInverseBallot, []string{
			"WaveGetLaneIndex()",
			"const bool _e2 = ((_e2_b[_e2_li / 32u] >> (_e2_li % 32u)) & 1u) != 0u;",
		}},
		{"count", ir.SubgroupBallotBitCount, []string{
			"int(WaveGetLaneCount())",
			"countbits(_e2_w0) + countbits(_e2_w1) + countbits(_e2_w2) + countbits(_e2_w3)",
		}},
		{"inclusive count", ir.SubgroupBallotBitInclusiveCount, []string{
			"WaveGetLaneIndex()", "countbits(_e2_b.x",
			"+ ((_e2_b[_e2_li / 32u] >> (_e2_li % 32u)) & 1u)",
		}},
		{"exclusive count", ir.SubgroupBallotBitExclusiveCount, []string{
			"WaveGetLaneIndex()", "countbits(_e2_b.x",
		}},
		{"find lsb", ir.SubgroupBallotFindLSB, []string{
			"firstbitlow(_e2_w0)", "96u + firstbitlow(_e2_w3)", "0xFFFFFFFFu",
		}},
		{"find msb", ir.SubgroupBallotFindMSB, []string{
			"96u + firstbithigh(_e2_w3)", "firstbithigh(_e2_w0)", "0xFFFFFFFFu",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, err := Compile(hlslBallotBitModule(tc.op), DefaultOptions())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			mustContain(t, code, tc.want)
			if strings.Contains(code, "WaveActiveCountBits") || strings.Contains(code, "WavePrefixCountBits") {
				t.Errorf("the form counts the live wave's predicate, not the mask value:\n%s", code)
			}
		})
	}
}

// hlslBallotBitModule builds a compute entry point with a ballot followed by
// one ballot bit operation over its mask: expr 0 the mask, expr 1 a u32
// index, expr 2 the result (bool for extract/inverse ballot, u32 otherwise).
func hlslBallotBitModule(op ir.SubgroupBallotBitOperation) *ir.Module {
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
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{64, 1, 1},
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
