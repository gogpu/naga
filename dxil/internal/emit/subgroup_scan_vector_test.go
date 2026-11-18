package emit

// P1-DXIL-115 (no silent scan substitution) and P1-DXIL-116 (vector subgroup
// operands). The DXIL wave ops have no prefix form for min/max/and/or/xor, so
// those scans are refused like the MSL/HLSL/GLSL writers (P0-MSL-111); Add/Mul
// scans stay. A vector operand is expanded per component, the form DXC itself
// emits for a vector wave intrinsic (it scalarizes into one dx.op.waveActiveOp
// per component) and the MSL writer uses (P2-GLSLIN-028).

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// TestEmitSubgroupScanRefused: every min/max/and/or/xor scan (inclusive and
// exclusive) is refused with a named limitation, never written as the reduction
// or as an unverified wavePrefixOp min/max. On the defective base the And/Or/Xor
// scans emit dx.op.waveActiveBit and the Min/Max scans emit dx.op.wavePrefixOp,
// both without an error.
func TestEmitSubgroupScanRefused(t *testing.T) {
	ops := []ir.SubgroupOperation{
		ir.SubgroupOperationMin, ir.SubgroupOperationMax,
		ir.SubgroupOperationAnd, ir.SubgroupOperationOr, ir.SubgroupOperationXor,
	}
	scans := []ir.CollectiveOperation{ir.CollectiveInclusiveScan, ir.CollectiveExclusiveScan}
	for _, op := range ops {
		for _, cop := range scans {
			irMod := dxilCollectiveModule(op, cop, false)
			if _, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0}); err == nil {
				t.Errorf("subgroup scan op=%d cop=%d emitted, want a refusal", op, cop)
			}
		}
	}
}

// TestEmitSubgroupClusteredRefused: DXIL has no clustered wave operation
// (SM 6.5's WaveMultiPrefix is an HLSL-level shuffle emulation), so every
// clustered reduction is refused with a named limitation instead of being
// written as the full-wave one (card P2-IR-040, ADR-014 D4).
func TestEmitSubgroupClusteredRefused(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{
		ir.SubgroupOperationAdd, ir.SubgroupOperationMul, ir.SubgroupOperationMin,
		ir.SubgroupOperationMax, ir.SubgroupOperationAnd, ir.SubgroupOperationOr,
		ir.SubgroupOperationXor,
	} {
		irMod := dxilCollectiveModule(op, ir.CollectiveOperationClusteredReduce, false)
		if _, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0}); err == nil {
			t.Errorf("clustered reduction op=%d emitted, want a refusal", op)
		}
	}
}

// TestEmitSubgroupArithmeticScanWritten guards the Add/Mul scans: they have a
// DXIL prefix form (dx.op.wavePrefixOp, DXC emits it for WavePrefixSum/Product)
// and must keep being written, inclusive and exclusive.
func TestEmitSubgroupArithmeticScanWritten(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{ir.SubgroupOperationAdd, ir.SubgroupOperationMul} {
		for _, cop := range []ir.CollectiveOperation{ir.CollectiveInclusiveScan, ir.CollectiveExclusiveScan} {
			irMod := dxilCollectiveModule(op, cop, false)
			mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
			if err != nil {
				t.Fatalf("Add/Mul scan op=%d cop=%d refused: %v", op, cop, err)
			}
			if !hasDxOpCall(mod, "dx.op.wavePrefixOp.i32") {
				t.Errorf("Add/Mul scan op=%d cop=%d: no dx.op.wavePrefixOp.i32 call", op, cop)
			}
		}
	}
}

// TestEmitSubgroupBitReduceWritten guards the And/Or/Xor reductions: the
// reduction is the correct dx.op.waveActiveBit form and must stay written.
func TestEmitSubgroupBitReduceWritten(t *testing.T) {
	for _, op := range []ir.SubgroupOperation{ir.SubgroupOperationAnd, ir.SubgroupOperationOr, ir.SubgroupOperationXor} {
		irMod := dxilCollectiveModule(op, ir.CollectiveReduce, false)
		mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
		if err != nil {
			t.Fatalf("bit reduction op=%d refused: %v", op, err)
		}
		if !hasDxOpCall(mod, "dx.op.waveActiveBit.i32") {
			t.Errorf("bit reduction op=%d: no dx.op.waveActiveBit.i32 call", op)
		}
	}
}

// TestEmitSubgroupVectorReducePerComponent: a vec4 reduction expands into one
// scalar dx.op.waveActiveOp per component (4 calls), the form DXC emits for
// WaveActiveSum(float4). On the defective base only the first component is
// computed (1 call) and the rest of the result resolves to it.
func TestEmitSubgroupVectorReducePerComponent(t *testing.T) {
	irMod := dxilCollectiveModule(ir.SubgroupOperationAdd, ir.CollectiveReduce, true)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("vector Add reduction failed: %v", err)
	}
	if n := countDxOpCalls(mod, "dx.op.waveActiveOp.i32"); n != 4 {
		t.Errorf("vector Add reduction: %d dx.op.waveActiveOp.i32 calls, want 4 (one per component)", n)
	}
}

// TestEmitSubgroupVectorBitReducePerComponent: a vec4 bit reduction expands into
// one scalar dx.op.waveActiveBit per component.
func TestEmitSubgroupVectorBitReducePerComponent(t *testing.T) {
	irMod := dxilCollectiveModule(ir.SubgroupOperationAnd, ir.CollectiveReduce, true)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("vector And reduction failed: %v", err)
	}
	if n := countDxOpCalls(mod, "dx.op.waveActiveBit.i32"); n != 4 {
		t.Errorf("vector And reduction: %d dx.op.waveActiveBit.i32 calls, want 4 (one per component)", n)
	}
}

// TestEmitSubgroupVectorScanPerComponent: a vec4 Add inclusive scan expands into
// one scalar dx.op.wavePrefixOp per component (the inclusive combine is a
// per-component binary add on top).
func TestEmitSubgroupVectorScanPerComponent(t *testing.T) {
	irMod := dxilCollectiveModule(ir.SubgroupOperationAdd, ir.CollectiveInclusiveScan, true)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("vector Add inclusive scan failed: %v", err)
	}
	if n := countDxOpCalls(mod, "dx.op.wavePrefixOp.i32"); n != 4 {
		t.Errorf("vector Add inclusive scan: %d dx.op.wavePrefixOp.i32 calls, want 4 (one per component)", n)
	}
}

// dxilCollectiveModule builds a compute entry point with one collective
// operation over a u32 argument: a scalar literal, or a vec4<u32> splat when
// vector is true. The result type matches the argument type (the per-component
// arithmetic/bit form); All/Any/AllEqual are not built here.
func dxilCollectiveModule(op ir.SubgroupOperation, cop ir.CollectiveOperation, vector bool) *ir.Module {
	u32 := ir.Type{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	if vector {
		v4 := ir.Type{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}
		return dxilSubgroupModule(
			[]ir.Type{u32, v4},
			[]ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
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
		[]ir.Type{u32},
		[]ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			{Kind: ir.ExprSubgroupOperationResult{Type: 0}},
		},
		[]ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtSubgroupCollectiveOperation{Op: op, CollectiveOp: cop, Argument: 0, Result: 1}},
			{Kind: ir.StmtReturn{}},
		},
	)
}

// countDxOpCalls counts the calls to a dx.op function by name in main.
func countDxOpCalls(mod *module.Module, name string) int {
	mainFn := findMainFunc(mod)
	if mainFn == nil {
		return 0
	}
	n := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == name {
				n++
			}
		}
	}
	return n
}
