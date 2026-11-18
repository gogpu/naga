package emit

// The five ballot-mask built-ins (card P2-IR-042, ADR-014 D5). DXIL has no
// ballot-mask value, so the emitter synthesizes the vec4 from
// dx.op.waveGetLaneIndex and dx.op.waveGetLaneCount: the mask spans every
// invocation of the wave, and dx.op.ballot(true) holds only the active lanes.
// internal/subgroupmask is the semantic oracle.

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/internal/subgroupmask"
	"github.com/gogpu/naga/ir"
)

// TestEmitSubgroupMaskWordsMatchTheOracle: the Go transcription of
// emitSubgroupMaskWord/emitMaskBelow equals internal/subgroupmask's
// spec-equation evaluator for every power-of-two size up to 128 and every
// lane, including the word boundaries and the partial last word — the check
// the emitted DXIL itself cannot be run through locally.
func TestEmitSubgroupMaskWordsMatchTheOracle(t *testing.T) {
	for size := uint32(1); size <= 128; size <<= 1 {
		for lane := uint32(0); lane < size; lane++ {
			for kind := subgroupmask.Kind(0); kind <= subgroupmask.Lt; kind++ {
				var got [4]uint32
				for c := uint32(0); c < 4; c++ {
					got[c] = emitSubgroupMaskWord(uint32(kind), int(c), int(lane), int(size))
				}
				if want := subgroupmask.Mask(kind, lane, size); got != want {
					t.Fatalf("size %d lane %d kind %d: %08x, want %08x", size, lane, kind, got, want)
				}
			}
		}
	}
}

// emitMaskBelow mirrors the emitted emitMaskBelow: (1 << n) - 1 for
// 1 <= n <= 31, 0xFFFFFFFF for n >= 32, 0 otherwise.
func emitMaskBelow(n int32) uint32 {
	if n >= 32 {
		return 0xFFFFFFFF
	}
	if n <= 0 {
		return 0
	}
	return (1 << uint32(n)) - 1
}

// emitSubgroupMaskWord mirrors the emitted emitSubgroupMaskWord line for
// line; a change to the emitted instruction sequence must change this mirror.
func emitSubgroupMaskWord(kind uint32, c, lane, size int) uint32 {
	base := int32(32 * c)
	lo := int32(size) - base
	if lo < 0 {
		lo = 0
	}
	if lo > 32 {
		lo = 32
	}
	full := emitMaskBelow(lo)
	if kind == 0 { // Eq
		if int32(lane)>>5 == int32(c) {
			return 1 << (uint32(lane) & 31)
		}
		return 0
	}
	if kind <= 2 { // Ge, Gt
		start := int32(lane)
		if kind == 2 {
			start++
		}
		delta := start - base
		keep := full &^ emitMaskBelow(delta)
		if start <= base {
			return full
		}
		return keep
	}
	// Le, Lt
	delta := int32(lane) - base
	if kind == 3 {
		delta++
	}
	if delta < 0 {
		delta = 0
	}
	if delta > lo {
		delta = lo
	}
	return emitMaskBelow(delta)
}

// TestEmitSubgroupMaskLoadBindsFourComponents: the mask argument's expression
// carries four component IDs after the synthesis, so a store of the vec4
// reads all of them.
func TestEmitSubgroupMaskLoadBindsFourComponents(t *testing.T) {
	binding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinSubgroupEqMask})
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "vec4u32", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{64, 1, 1},
			Function: ir.Function{
				Name:      "main",
				Arguments: []ir.FunctionArgument{{Name: "mask", Type: 1, Binding: &binding}},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},
				},
				Body: []ir.Statement{{Kind: ir.StmtReturn{}}},
			},
		}},
	}
	dxMod := module.NewModule(module.ComputeShader)
	emitter := &Emitter{mod: dxMod, ir: mod}
	emitter.intConsts = make(map[int64]int)
	emitter.constMap = make(map[int]*module.Constant)
	emitter.exprValues = make(map[ir.ExpressionHandle]int)
	emitter.exprComponents = make(map[ir.ExpressionHandle][]int)
	emitter.dxOpFuncs = make(map[dxOpKey]*module.Function)
	fnTy := dxMod.GetFunctionType(dxMod.GetVoidType(), nil)
	dxFn := dxMod.AddFunction("main", fnTy, false)
	emitter.currentBB = dxFn.AddBasicBlock("entry")

	fn := &mod.EntryPoints[0].Function
	if err := emitter.emitSubgroupMaskLoad(fn, 0, 0); err != nil {
		t.Fatalf("emitSubgroupMaskLoad: %v", err)
	}
	handle := emitter.findArgExprHandle(fn, 0)
	comps := emitter.exprComponents[handle]
	if len(comps) != 4 {
		t.Fatalf("mask components = %d, want 4", len(comps))
	}
	if emitter.exprValues[handle] != comps[0] {
		t.Errorf("exprValues does not hold the first component")
	}
}
