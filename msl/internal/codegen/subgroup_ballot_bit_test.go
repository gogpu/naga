package codegen

// The ballot bit operations (card P2-IR-039, ADR-014 D3): arithmetic over the
// four mask words with metal::popcount / ctz / clz, the subgroup size and lane
// index threaded in as kernel inputs and by-value callee parameters.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestMSL_SubgroupBallotBitForms pins the seven forms: the count and find
// forms mask the words to the subgroup size ([[threads_per_simdgroup]]), the
// scans and inverse ballot read the lane index ([[thread_index_in_simdgroup]]),
// and extract reads neither. Metal has no function that reads a ballot value's
// bits, so the forms are arithmetic, never a simd vote over the live threads.
func TestMSL_SubgroupBallotBitForms(t *testing.T) {
	for _, tc := range []struct {
		name     string
		op       ir.SubgroupBallotBitOperation
		want     []string
		wantSize bool
		wantLane bool
	}{
		{"extract", ir.SubgroupBallotBitExtract, []string{">> (", "& 1u) != 0u"}, false, false},
		{"inverse ballot", ir.SubgroupInverseBallot, []string{"& 1u) != 0u"}, false, true},
		{"count", ir.SubgroupBallotBitCount, []string{"metal::popcount("}, true, false},
		{"inclusive count", ir.SubgroupBallotBitInclusiveCount, []string{"metal::popcount(", "+ (("}, false, true},
		{"exclusive count", ir.SubgroupBallotBitExclusiveCount, []string{"metal::popcount("}, false, true},
		{"find lsb", ir.SubgroupBallotFindLSB, []string{"metal::ctz(", "0xFFFFFFFFu"}, true, false},
		{"find msb", ir.SubgroupBallotFindMSB, []string{"metal::clz(", "31u -", "0xFFFFFFFFu"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src, _, err := Compile(mslBallotBitModule(tc.op), DefaultOptions())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(src, want) {
					t.Errorf("output lacks %q:\n%s", want, src)
				}
			}
			if got := strings.Contains(src, "[[threads_per_simdgroup]]"); got != tc.wantSize {
				t.Errorf("threads_per_simdgroup present = %v, want %v:\n%s", got, tc.wantSize, src)
			}
			if got := strings.Contains(src, "[[thread_index_in_simdgroup]]"); got != tc.wantLane {
				t.Errorf("thread_index_in_simdgroup present = %v, want %v:\n%s", got, tc.wantLane, src)
			}
			if strings.Contains(src, "simd_sum(") || strings.Contains(src, "simd_prefix_exclusive_sum(") {
				t.Errorf("the form votes over the live threads, not the mask value:\n%s", src)
			}
		})
	}
}

// TestMSL_SubgroupBallotBitCallee: a ballot bit operation in a helper function
// receives the subgroup size as a by-value pass-through parameter, and the
// entry point synthesizes the kernel input and passes it down (the
// _buffer_sizes pattern).
func TestMSL_SubgroupBallotBitCallee(t *testing.T) {
	src, _, err := Compile(mslBallotBitCalleeModule(), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, want := range []string{
		"metal::uint __subgroup_size", // the callee's pass-through parameter
		"[[threads_per_simdgroup]]",   // the entry point's kernel input
		"metal::popcount(",            // the count arithmetic
	} {
		if !strings.Contains(src, want) {
			t.Errorf("output lacks %q:\n%s", want, src)
		}
	}
}

// TestMSL_SubgroupBallotBitReusesBuiltin: a declared gl_SubgroupSize argument
// is reused for the count's size mask — the writer adds no second
// [[threads_per_simdgroup]] parameter (the workgroup-init reuse pattern).
func TestMSL_SubgroupBallotBitReusesBuiltin(t *testing.T) {
	mod := mslBallotBitModule(ir.SubgroupBallotBitCount)
	var binding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupSize}
	mod.EntryPoints[0].Function.Arguments = []ir.FunctionArgument{
		{Name: "gl_SubgroupSize", Type: 0, Binding: &binding},
	}
	src, _, err := Compile(mod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if n := strings.Count(src, "[[threads_per_simdgroup]]"); n != 1 {
		t.Errorf("threads_per_simdgroup appears %d times, want 1 (the declared argument reused):\n%s", n, src)
	}
	if strings.Contains(src, "__subgroup_size") {
		t.Errorf("a synthetic size parameter was added despite the declared builtin:\n%s", src)
	}
}

// mslBallotBitModule builds a compute entry point with a ballot followed by
// one ballot bit operation over its mask: expr 0 the mask, expr 1 a u32
// index, expr 2 the result (bool for extract/inverse ballot, u32 otherwise).
func mslBallotBitModule(op ir.SubgroupBallotBitOperation) *ir.Module {
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

// mslBallotBitCalleeModule builds a void helper that counts a ballot's bits,
// called from the entry point: the size must reach the helper by value.
func mslBallotBitCalleeModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "helper",
			Expressions: []ir.Expression{
				{Kind: ir.ExprSubgroupBallotResult{}},           // [0] mask
				{Kind: ir.ExprSubgroupOperationResult{Type: 0}}, // [1] count result
			},
			Body: []ir.Statement{
				{Kind: ir.StmtSubgroupBallot{Result: 0}},
				{Kind: ir.StmtSubgroupBallotBit{Op: ir.SubgroupBallotBitCount, Mask: 0, Result: 1}},
				{Kind: ir.StmtReturn{}},
			},
		}},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{64, 1, 1},
			Function: ir.Function{
				Name: "main",
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}
