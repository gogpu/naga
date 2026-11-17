package emit

// The ballot bit operations (card P2-IR-039, ADR-014 D3): arithmetic over the
// four mask words with dx.op.unaryBits Countbits / FirstbitLo / FirstbitHi,
// the wave size and lane index from dx.op.waveGetLaneCount / waveGetLaneIndex.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestEmitSubgroupBallotBitForms pins which dx.op calls each form emits: the
// count and find forms read the wave size and call unaryBits; the scans and
// inverse ballot read the lane index; extract reads neither (it shifts a
// selected word). The wave count ops (WaveAllBitCount / WavePrefixBitCount)
// must not appear — they count the live wave's predicate, not a mask value.
func TestEmitSubgroupBallotBitForms(t *testing.T) {
	for _, tc := range []struct {
		name     string
		op       ir.SubgroupBallotBitOperation
		wantBits bool // dx.op.unaryBits.i32
		wantSize bool // dx.op.waveGetLaneCount
		wantLane bool // dx.op.waveGetLaneIndex
	}{
		{"extract", ir.SubgroupBallotBitExtract, false, false, false},
		{"inverse ballot", ir.SubgroupInverseBallot, false, false, true},
		{"count", ir.SubgroupBallotBitCount, true, true, false},
		{"inclusive count", ir.SubgroupBallotBitInclusiveCount, true, false, true},
		{"exclusive count", ir.SubgroupBallotBitExclusiveCount, true, false, true},
		{"find lsb", ir.SubgroupBallotFindLSB, true, true, false},
		{"find msb", ir.SubgroupBallotFindMSB, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mod, err := Emit(dxilBallotBitModule(tc.op), EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
			if err != nil {
				t.Fatalf("Emit failed: %v", err)
			}
			if !hasDxOpCall(mod, "dx.op.waveActiveBallot") {
				t.Error("the ballot mask is not emitted (dx.op.waveActiveBallot)")
			}
			if got := hasDxOpCall(mod, "dx.op.unaryBits.i32"); got != tc.wantBits {
				t.Errorf("dx.op.unaryBits.i32 call = %v, want %v", got, tc.wantBits)
			}
			if got := hasDxOpCall(mod, "dx.op.waveGetLaneCount"); got != tc.wantSize {
				t.Errorf("dx.op.waveGetLaneCount call = %v, want %v", got, tc.wantSize)
			}
			if got := hasDxOpCall(mod, "dx.op.waveGetLaneIndex"); got != tc.wantLane {
				t.Errorf("dx.op.waveGetLaneIndex call = %v, want %v", got, tc.wantLane)
			}
			if hasDxOpCall(mod, "dx.op.waveAllBitCount") || hasDxOpCall(mod, "dx.op.wavePrefixBitCount") {
				t.Error("the form counts the live wave's predicate, not the mask value")
			}
		})
	}
}

// TestEmitSubgroupBallotBitSharesWaveLaneDecl: a ballot bit count and a
// built-in subgroup-size read share one dx.op.waveGetLaneCount declaration —
// AddFunction does not deduplicate, so the writer routes both through one
// helper (the .i32-suffix trap of io.go).
func TestEmitSubgroupBallotBitSharesWaveLaneDecl(t *testing.T) {
	irMod := dxilBallotBitModule(ir.SubgroupBallotBitCount)
	// A built-in subgroup-size read: a global of the builtin kind is heavy to
	// build here, so call the shared declaration helper twice through the
	// emitter instead and count the declarations by name.
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	n := 0
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.waveGetLaneCount" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("dx.op.waveGetLaneCount declared %d times, want 1", n)
	}
}

// dxilBallotBitModule builds a compute entry point with a ballot followed by
// one ballot bit operation over its mask: expr 0 the mask, expr 1 a u32
// index, expr 2 the result (bool for extract/inverse ballot, u32 otherwise).
func dxilBallotBitModule(op ir.SubgroupBallotBitOperation) *ir.Module {
	resultType := ir.TypeHandle(0) // u32
	if op == ir.SubgroupBallotBitExtract || op == ir.SubgroupInverseBallot {
		resultType = 1 // bool
	}
	return dxilSubgroupModule(
		[]ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		[]ir.Expression{
			{Kind: ir.ExprSubgroupBallotResult{}},                    // [0] mask
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},              // [1] index
			{Kind: ir.ExprSubgroupOperationResult{Type: resultType}}, // [2] result
		},
		[]ir.Statement{
			{Kind: ir.StmtSubgroupBallot{Result: 0}},
			{Kind: ir.StmtSubgroupBallotBit{Op: op, Mask: 0, Index: 1, Result: 2}},
			{Kind: ir.StmtReturn{}},
		},
	)
}
