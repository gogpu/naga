package codegen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestMSL_SubgroupScansRefused: Metal's simd functions have min, max, and, or
// and xor as reductions only, while the IR — and the SPIR-V frontend that
// produces it — carries their prefix scans. Writing the reduction for a scan
// compiles and returns the wrong value (card P0-MSL-111), so the writer must
// refuse the scan; the reductions below stay.
func TestMSL_SubgroupScansRefused(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{
		ir.SubgroupOperationMin, ir.SubgroupOperationMax, ir.SubgroupOperationAnd,
		ir.SubgroupOperationOr, ir.SubgroupOperationXor,
	} {
		for _, cop := range []ir.CollectiveOperation{
			ir.CollectiveInclusiveScan, ir.CollectiveExclusiveScan,
		} {
			_, _, err := Compile(subgroupCollectiveModule(op, cop), DefaultOptions())
			if err == nil {
				t.Fatalf("operation %d with collective op %d compiled, want a refusal", op, cop)
			}
			if !strings.Contains(err.Error(), "subgroup scan of operation") {
				t.Errorf("operation %d with collective op %d: error = %v, want the subgroup-scan refusal",
					op, cop, err)
			}
		}
	}
}

// TestMSL_SubgroupCollectiveForms pins the forms the writer does write: the
// min/max/and/or/xor reductions, and the Add and Mul scans (Metal's simd
// prefix functions exist for sum and product only).
func TestMSL_SubgroupCollectiveForms(t *testing.T) {
	cases := []struct {
		op   ir.SubgroupOperation
		cop  ir.CollectiveOperation
		want string
	}{
		{ir.SubgroupOperationMin, ir.CollectiveReduce, "simd_min("},
		{ir.SubgroupOperationXor, ir.CollectiveReduce, "simd_xor("},
		{ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan, "simd_prefix_inclusive_sum("},
		{ir.SubgroupOperationMul, ir.CollectiveExclusiveScan, "simd_prefix_exclusive_product("},
	}
	for _, tt := range cases {
		src, _, err := Compile(subgroupCollectiveModule(tt.op, tt.cop), DefaultOptions())
		if err != nil {
			t.Fatalf("operation %d with collective op %d: %v", tt.op, tt.cop, err)
		}
		if !strings.Contains(src, tt.want) {
			t.Errorf("operation %d with collective op %d: output lacks %q:\n%s", tt.op, tt.cop, tt.want, src)
		}
	}
}

// TestMSL_VectorSubgroupOperand: Metal's simd collective functions take and
// return a scalar, while the SPIR-V subgroup operations on a vector are the
// per-component operations. The writer evaluates the operand once and expands
// the call into one scalar simd call per component (P2-GLSLIN-028), instead of
// writing the simd_sum(float4) the Metal compiler rejects.
func TestMSL_VectorSubgroupOperand(t *testing.T) {
	cases := []struct {
		op   ir.SubgroupOperation
		cop  ir.CollectiveOperation
		want string
	}{
		{ir.SubgroupOperationAdd, ir.CollectiveReduce, "metal::simd_sum(unnamed_1[%d])"},
		{ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan, "metal::simd_prefix_inclusive_sum(unnamed_1[%d])"},
		{ir.SubgroupOperationAnd, ir.CollectiveReduce, "metal::simd_and(unnamed_1[%d])"},
	}
	for _, tt := range cases {
		t.Run("", func(t *testing.T) {
			src, _, err := Compile(vectorSubgroupModule(tt.op, tt.cop), DefaultOptions())
			if err != nil {
				t.Fatalf("operation %d with collective op %d: %v", tt.op, tt.cop, err)
			}
			// The operand is evaluated once, into its own temporary.
			if !strings.Contains(src, "metal::float4 unnamed_1 = metal::float4(") {
				t.Errorf("operation %d with collective op %d: the operand is not evaluated into one temporary:\n%s",
					tt.op, tt.cop, src)
			}
			// Every component is its own scalar call, and no vector call remains.
			for component := 0; component < 4; component++ {
				want := fmt.Sprintf(tt.want, component)
				if !strings.Contains(src, want) {
					t.Errorf("operation %d with collective op %d: output lacks %q:\n%s", tt.op, tt.cop, want, src)
				}
			}
			if strings.Contains(src, "simd_sum(unnamed_1)\n") || strings.Contains(src, "simd_sum(metal::float4(") {
				t.Errorf("operation %d with collective op %d: a vector operand reached a simd function whole:\n%s",
					tt.op, tt.cop, src)
			}
		})
	}
}

// TestMSL_SubgroupElect pins subgroupElect's Metal form: simd_is_first(),
// true for the first active thread of the simdgroup (the WGSL subgroups
// proposal's Appendix B maps subgroupElect to it; card P2-IR-038, ADR-014).
func TestMSL_SubgroupElect(t *testing.T) {
	src, _, err := Compile(subgroupElectModule(), DefaultOptions())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(src, "metal::simd_is_first()") {
		t.Errorf("output lacks metal::simd_is_first():\n%s", src)
	}
}

// TestMSL_SubgroupAllEqual pins the all-equal expansion. Metal has no simd
// all-equal function (Appendix B has no entry), so the writer evaluates the
// operand once, broadcasts the first active thread's value with
// simd_broadcast_first and reduces the comparisons with simd_all; a vector
// operand compares per component. A bool operand is widened to int, because
// simd_broadcast_first is documented for the numeric scalar types.
func TestMSL_SubgroupAllEqual(t *testing.T) {
	t.Run("uint", func(t *testing.T) {
		src, _, err := Compile(subgroupAllEqualModule(subgroupAllEqualScalarArgument), DefaultOptions())
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		for _, want := range []string{
			"= 1u;",
			"metal::simd_all(",
			" == metal::simd_broadcast_first(",
		} {
			if !strings.Contains(src, want) {
				t.Errorf("output lacks %q:\n%s", want, src)
			}
		}
	})

	t.Run("bool widened", func(t *testing.T) {
		src, _, err := Compile(subgroupAllEqualBoolModule(), DefaultOptions())
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		for _, want := range []string{
			"metal::simd_all((int)",
			"metal::simd_broadcast_first((int)",
		} {
			if !strings.Contains(src, want) {
				t.Errorf("output lacks %q:\n%s", want, src)
			}
		}
	})

	t.Run("vector per component", func(t *testing.T) {
		src, _, err := Compile(subgroupAllEqualModule(subgroupAllEqualVectorArgument), DefaultOptions())
		if err != nil {
			t.Fatalf("Compile: %v", err)
		}
		for _, want := range []string{
			"[0] == ",
			"[1] == ",
			" && ",
		} {
			if !strings.Contains(src, want) {
				t.Errorf("output lacks %q:\n%s", want, src)
			}
		}
		if strings.Contains(src, "simd_all(") == false || strings.Count(src, "metal::simd_all(") != 2 {
			t.Errorf("want one simd_all per component (2):\n%s", src)
		}
	})
}

// TestMSL_AllEqualScanRefused: all-equal has no scan form; a module that
// carries one is refused rather than written as its reduction.
func TestMSL_AllEqualScanRefused(t *testing.T) {
	mod := subgroupAllEqualModule(subgroupAllEqualScalarArgument)
	op := mod.EntryPoints[0].Function.Body[0].Kind.(ir.StmtSubgroupCollectiveOperation)
	op.CollectiveOp = ir.CollectiveInclusiveScan
	mod.EntryPoints[0].Function.Body[0].Kind = op
	if _, _, err := Compile(mod, DefaultOptions()); err == nil {
		t.Fatal("an all-equal inclusive scan compiled, want a refusal")
	}
}

// subgroupElectModule builds a compute entry point whose only statement is
// subgroupElect, with a bool result expression.
func subgroupElectModule() *ir.Module {
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

// The operand shapes subgroupAllEqualModule can carry, named by the
// expression handle they pass as the statement's argument.
const (
	subgroupAllEqualScalarArgument = 0 // expression 0: uint literal
	subgroupAllEqualVectorArgument = 2 // expression 2: uvec2 splat of expression 0
)

// subgroupAllEqualModule builds a compute entry point whose only statement is
// one subgroup all-equal over a uint literal (or its uvec2 splat), with a bool
// result expression.
func subgroupAllEqualModule(argument int) *ir.Module {
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},     // [0] scalar argument
					{Kind: ir.ExprSubgroupOperationResult{Type: 1}}, // [1] bool result
					{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 0}},   // [2] vector argument
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupCollectiveOperation{
						Op:           ir.SubgroupOperationAllEqual,
						CollectiveOp: ir.CollectiveReduce,
						Argument:     ir.ExpressionHandle(argument),
						Result:       1,
					}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}

// subgroupAllEqualBoolModule builds the all-equal over a bool literal, the
// operand the writer widens to int.
func subgroupAllEqualBoolModule() *ir.Module {
	return &ir.Module{
		Types: []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // [0] argument
					{Kind: ir.ExprSubgroupOperationResult{Type: 0}}, // [1] bool result
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

// vectorSubgroupModule builds a compute entry point whose only statement is one
// subgroup collective operation over a vector literal.
func vectorSubgroupModule(op ir.SubgroupOperation, cop ir.CollectiveOperation) *ir.Module {
	scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vec4 := ir.TypeHandle(1)
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: scalar},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: scalar}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(2)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(3)}},
					{Kind: ir.Literal{Value: ir.LiteralF32(4)}},
					{Kind: ir.ExprCompose{Type: vec4, Components: []ir.ExpressionHandle{0, 1, 2, 3}}}, // [4] argument
					{Kind: ir.ExprSubgroupOperationResult{Type: vec4}},                                // [5] result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupCollectiveOperation{Op: op, CollectiveOp: cop, Argument: 4, Result: 5}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}

// subgroupCollectiveModule builds a compute entry point whose only statement is
// one subgroup collective operation over a literal, so the writer's form for it
// is pinned without any other machinery.
func subgroupCollectiveModule(op ir.SubgroupOperation, cop ir.CollectiveOperation) *ir.Module {
	u32 := ir.TypeHandle(0)
	return &ir.Module{
		Types: []ir.Type{{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralU32(1)}},       // [0] argument
					{Kind: ir.ExprSubgroupOperationResult{Type: u32}}, // [1] result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSubgroupCollectiveOperation{Op: op, CollectiveOp: cop, Argument: 0, Result: 1}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
}
