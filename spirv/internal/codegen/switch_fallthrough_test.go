package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestEmitSwitchNonEmptyFallThrough pins the SPIR-V block structure of a
// switch whose fall-through cases have bodies, including a non-last default:
//
//	case 1 (falls through):  v = 10
//	default (falls through): v = 20
//	case 2:                  v = 30
//
// Each case block must run its own store and then branch to the next case's
// label (Rust back/spv/block.rs:3718-3728: a fall-through case finishes with
// a branch to the next case's id), and the last case to the merge block. WGSL never builds this shape;
// the SPIR-V frontend does.
func TestEmitSwitchNonEmptyFallThrough(t *testing.T) {
	i32 := ir.TypeHandle(0)
	store := func(value ir.ExpressionHandle) ir.Block {
		return ir.Block{{Kind: ir.StmtStore{Pointer: 1, Value: value}}}
	}
	module := &ir.Module{
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
	spv, err := NewBackend(DefaultOptions()).Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	instrs := decodeSPIRVInstructions(spv)

	// Per block: number of stores and the OpBranch target.
	type blockInfo struct {
		stores int
		branch uint32
	}
	blocks := map[uint32]*blockInfo{}
	var cur *blockInfo
	var mergeLabel, defaultLabel uint32
	caseLabel := map[uint32]uint32{} // literal -> label
	for _, in := range instrs {
		switch in.opcode {
		case spvenums.OpLabel:
			cur = &blockInfo{}
			blocks[in.words[1]] = cur
		case spvenums.OpStore:
			if cur != nil {
				cur.stores++
			}
		case spvenums.OpBranch:
			if cur != nil {
				cur.branch = in.words[1]
			}
		case spvenums.OpSelectionMerge:
			mergeLabel = in.words[1]
		case spvenums.OpSwitch:
			defaultLabel = in.words[2]
			for k := 3; k+1 < len(in.words); k += 2 {
				caseLabel[in.words[k]] = in.words[k+1]
			}
		}
	}
	if mergeLabel == 0 || defaultLabel == 0 || len(caseLabel) != 2 {
		t.Fatalf("switch not found: merge=%d default=%d cases=%v", mergeLabel, defaultLabel, caseLabel)
	}
	want := []struct {
		name  string
		label uint32
		next  uint32
	}{
		{"case 1", caseLabel[1], defaultLabel},
		{"default", defaultLabel, caseLabel[2]},
		{"case 2", caseLabel[2], mergeLabel},
	}
	for _, w := range want {
		b := blocks[w.label]
		if b == nil {
			t.Fatalf("%s: no block %%%d", w.name, w.label)
		}
		if b.stores != 1 || b.branch != w.next {
			t.Errorf("%s: stores=%d branch=%%%d, want stores=1 branch=%%%d", w.name, b.stores, b.branch, w.next)
		}
	}
}
