package emit

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// TestEmitSwitchNonEmptyFallThrough pins the DXIL block structure of a switch
// whose fall-through cases have bodies, including a non-last default:
//
//	case 1 (falls through):  v = 10
//	default (falls through): v = 20
//	case 2:                  v = 30
//
// Each case block must keep its store and end with a branch to the next case's
// block; the last case branches to the merge block. WGSL never builds this
// shape; the SPIR-V frontend does. (Rust naga has no DXIL backend; the SPIR-V
// writer's equivalent is back/spv/block.rs:3718-3728.)
func TestEmitSwitchNonEmptyFallThrough(t *testing.T) {
	i32 := ir.TypeHandle(0)
	store := func(value ir.ExpressionHandle) ir.Block {
		return ir.Block{{Kind: ir.StmtStore{Pointer: 1, Value: value}}}
	}
	irMod := &ir.Module{
		Types: []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		EntryPoints: []ir.EntryPoint{{
			Name:      "main",
			Stage:     ir.StageCompute,
			Workgroup: [3]uint32{1, 1, 1},
			Function: ir.Function{
				Name:      "main",
				LocalVars: []ir.LocalVariable{{Name: "v", Type: i32}},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},  // 0: selector
					{Kind: ir.ExprLocalVariable{Variable: 0}},    // 1
					{Kind: ir.Literal{Value: ir.LiteralI32(10)}}, // 2
					{Kind: ir.Literal{Value: ir.LiteralI32(20)}}, // 3
					{Kind: ir.Literal{Value: ir.LiteralI32(30)}}, // 4
				},
				Body: ir.Block{
					{Kind: ir.StmtSwitch{
						Selector: 0,
						Cases: []ir.SwitchCase{
							{Value: ir.SwitchValueI32(1), Body: store(2), FallThrough: true},
							{Value: ir.SwitchValueDefault{}, Body: store(3), FallThrough: true},
							{Value: ir.SwitchValueI32(2), Body: store(4)},
						},
					}},
					{Kind: ir.StmtReturn{}},
				},
			},
		}},
	}
	fn := mainFunc(t, emitOK(t, irMod))

	index := map[string]int{}
	for i, bb := range fn.BasicBlocks {
		if _, dup := index[bb.Name]; !dup {
			index[bb.Name] = i
		}
	}
	want := []struct{ block, next string }{
		{"switch.case_0", "switch.default"},
		{"switch.default", "switch.case_2"},
		{"switch.case_2", "switch.merge"},
	}
	for _, w := range want {
		bi, ok := index[w.block]
		if !ok {
			t.Fatalf("block %q not found", w.block)
		}
		bb := fn.BasicBlocks[bi]
		stores := 0
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrStore {
				stores++
			}
		}
		if len(bb.Instructions) == 0 {
			t.Fatalf("%s: empty block", w.block)
		}
		last := bb.Instructions[len(bb.Instructions)-1]
		if stores != 1 || last.Kind != module.InstrBr || len(last.Operands) != 1 || last.Operands[0] != index[w.next] {
			t.Errorf("%s: stores=%d last=%v %v, want 1 store and br to %s (bb %d)",
				w.block, stores, last.Kind, last.Operands, w.next, index[w.next])
		}
	}
}
