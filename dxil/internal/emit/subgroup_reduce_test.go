package emit

// The reduction, vote and float-scan branches of emitSubgroupCollectiveScalar,
// subgroupOpToWaveOp and combineForInclusive that the scan-refusal and vector
// tests (subgroup_scan_vector_test.go) do not reach. These use Emit only, so
// they cover the emitter on every CI platform — the IDxcValidator snapshot test
// (snapshot/dxil_subgroup_validate_test.go) skips where dxil.dll cannot load.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

var (
	subgroupU32  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	subgroupF32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	subgroupBool = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
)

// TestEmitSubgroupArithmeticReduce: Add/Mul/Min/Max reductions write
// dx.op.waveActiveOp with the matching wave op (sum/mul/min/max) and sign.
func TestEmitSubgroupArithmeticReduce(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{
		ir.SubgroupOperationAdd, ir.SubgroupOperationMul, ir.SubgroupOperationMin, ir.SubgroupOperationMax,
	} {
		mod, err := Emit(dxilTypedCollective(op, ir.CollectiveReduce, subgroupU32, ir.Literal{Value: ir.LiteralU32(1)}, false),
			EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err != nil {
			t.Fatalf("reduction op=%d failed: %v", op, err)
		}
		if !hasDxOpCall(mod, "dx.op.waveActiveOp.i32") {
			t.Errorf("reduction op=%d: no dx.op.waveActiveOp.i32 call", op)
		}
	}
}

// TestEmitSubgroupFloatReduce: the float overload of the arithmetic reductions.
func TestEmitSubgroupFloatReduce(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{
		ir.SubgroupOperationAdd, ir.SubgroupOperationMul, ir.SubgroupOperationMin, ir.SubgroupOperationMax,
	} {
		mod, err := Emit(dxilTypedCollective(op, ir.CollectiveReduce, subgroupF32, ir.Literal{Value: ir.LiteralF32(1)}, false),
			EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err != nil {
			t.Fatalf("float reduction op=%d failed: %v", op, err)
		}
		if !hasDxOpCall(mod, "dx.op.waveActiveOp.f32") {
			t.Errorf("float reduction op=%d: no dx.op.waveActiveOp.f32 call", op)
		}
	}
}

// TestEmitSubgroupVoteReduce: All/Any write dx.op.waveAllTrue / waveAnyTrue,
// whose signature is i1(i32 opcode, i1 cond) — no value overload suffix.
func TestEmitSubgroupVoteReduce(t *testing.T) {
	cases := []struct {
		op   ir.SubgroupOperation
		name string
	}{
		{ir.SubgroupOperationAll, "dx.op.waveAllTrue"},
		{ir.SubgroupOperationAny, "dx.op.waveAnyTrue"},
	}
	for _, tc := range cases {
		mod, err := Emit(dxilTypedCollective(tc.op, ir.CollectiveReduce, subgroupBool, ir.Literal{Value: ir.LiteralBool(true)}, false),
			EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err != nil {
			t.Fatalf("vote %s failed: %v", tc.name, err)
		}
		if !hasDxOpCall(mod, tc.name) {
			t.Errorf("vote %s: no call", tc.name)
		}
	}
}

// TestEmitSubgroupVectorVoteReduce: a bool-vector All/Any expands per component.
func TestEmitSubgroupVectorVoteReduce(t *testing.T) {
	cases := []struct {
		op   ir.SubgroupOperation
		name string
	}{
		{ir.SubgroupOperationAll, "dx.op.waveAllTrue"},
		{ir.SubgroupOperationAny, "dx.op.waveAnyTrue"},
	}
	for _, tc := range cases {
		mod, err := Emit(dxilTypedCollective(tc.op, ir.CollectiveReduce, subgroupBool, ir.Literal{Value: ir.LiteralBool(true)}, true),
			EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err != nil {
			t.Fatalf("vector vote %s failed: %v", tc.name, err)
		}
		if n := countDxOpCalls(mod, tc.name); n != 4 {
			t.Errorf("vector vote %s: %d calls, want 4 (one per component)", tc.name, n)
		}
	}
}

// TestEmitSubgroupVectorArithmeticReduce: a vec4 Add/Mul/Min/Max reduction
// expands into one dx.op.waveActiveOp per component.
func TestEmitSubgroupVectorArithmeticReduce(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{
		ir.SubgroupOperationAdd, ir.SubgroupOperationMul, ir.SubgroupOperationMin, ir.SubgroupOperationMax,
	} {
		mod, err := Emit(dxilTypedCollective(op, ir.CollectiveReduce, subgroupU32, ir.Literal{Value: ir.LiteralU32(1)}, true),
			EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err != nil {
			t.Fatalf("vector reduction op=%d failed: %v", op, err)
		}
		if n := countDxOpCalls(mod, "dx.op.waveActiveOp.i32"); n != 4 {
			t.Errorf("vector reduction op=%d: %d dx.op.waveActiveOp.i32 calls, want 4", op, n)
		}
	}
}

// TestEmitSubgroupFloatScan: the Add/Mul scans over a float operand write
// dx.op.wavePrefixOp.f32; the inclusive form combines with a float add/mul.
func TestEmitSubgroupFloatScan(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{ir.SubgroupOperationAdd, ir.SubgroupOperationMul} {
		for _, cop := range []ir.CollectiveOperation{ir.CollectiveInclusiveScan, ir.CollectiveExclusiveScan} {
			mod, err := Emit(dxilTypedCollective(op, cop, subgroupF32, ir.Literal{Value: ir.LiteralF32(1)}, false),
				EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
			if err != nil {
				t.Fatalf("float scan op=%d cop=%d failed: %v", op, cop, err)
			}
			if !hasDxOpCall(mod, "dx.op.wavePrefixOp.f32") {
				t.Errorf("float scan op=%d cop=%d: no dx.op.wavePrefixOp.f32 call", op, cop)
			}
		}
	}
}

// dxilTypedCollective builds a compute entry point with one collective operation
// over an argument of the given scalar type (a literal, or a vec4 splat of it).
// The result type matches the argument type.
func dxilTypedCollective(op ir.SubgroupOperation, cop ir.CollectiveOperation, scalar ir.ScalarType, lit ir.ExpressionKind, vector bool) *ir.Module {
	argT := ir.Type{Name: "", Inner: scalar}
	if vector {
		vecT := ir.Type{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: scalar}}
		return dxilSubgroupModule(
			[]ir.Type{argT, vecT},
			[]ir.Expression{
				{Kind: lit},
				{Kind: ir.ExprSplat{Size: ir.Vec4, Value: 0}},
				{Kind: ir.ExprSubgroupOperationResult{Type: 1}},
			},
			[]ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtSubgroupCollectiveOperation{Op: op, CollectiveOp: cop, Argument: 1, Result: 2}},
				{Kind: ir.StmtReturn{}},
			},
		)
	}
	return dxilSubgroupModule(
		[]ir.Type{argT},
		[]ir.Expression{
			{Kind: lit},
			{Kind: ir.ExprSubgroupOperationResult{Type: 0}},
		},
		[]ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtSubgroupCollectiveOperation{Op: op, CollectiveOp: cop, Argument: 0, Result: 1}},
			{Kind: ir.StmtReturn{}},
		},
	)
}
