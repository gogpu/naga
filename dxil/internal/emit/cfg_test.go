// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package emit

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// cfgFunc builds a function whose blocks hold the given instructions.
func cfgFunc(blocks ...[]*module.Instruction) *module.Function {
	fn := &module.Function{Name: "f"}
	for _, instrs := range blocks {
		bb := fn.AddBasicBlock("")
		bb.Instructions = instrs
	}
	return fn
}

// blockOf returns the index of bb in fn, or -1.
func blockOf(fn *module.Function, bb *module.BasicBlock) int {
	for i, b := range fn.BasicBlocks {
		if b == bb {
			return i
		}
	}
	return -1
}

func valueInstr(id int) *module.Instruction {
	return &module.Instruction{Kind: module.InstrBinOp, HasValue: true, ValueID: id, Operands: []int{100, 101, 0}}
}

func TestRemoveUnreachableBlocks(t *testing.T) {
	t.Run("merge after two returning arms", func(t *testing.T) {
		// 0: br c, 1, 2 | 1: br 4 | 2: br 4 | 3: br 4 (no predecessor) | 4: ret
		fn := cfgFunc(
			[]*module.Instruction{module.NewBrCondInstr(1, 2, 50)},
			[]*module.Instruction{module.NewBrInstr(4)},
			[]*module.Instruction{module.NewBrInstr(4)},
			[]*module.Instruction{module.NewBrInstr(4)},
			[]*module.Instruction{module.NewRetVoidInstr()},
		)
		exit := fn.BasicBlocks[4]
		removeUnreachableBlocks(fn)
		if len(fn.BasicBlocks) != 4 {
			t.Fatalf("blocks = %d, want 4", len(fn.BasicBlocks))
		}
		if got := blockOf(fn, exit); got != 3 {
			t.Fatalf("exit block at %d, want 3", got)
		}
		if got := fn.BasicBlocks[0].Instructions[0].Operands; !reflect.DeepEqual(got, []int{1, 2, 50}) {
			t.Errorf("entry branch = %v, want [1 2 50] (the condition is a value, not a block)", got)
		}
		for i := 1; i <= 2; i++ {
			if got := fn.BasicBlocks[i].Instructions[0].Operands; !reflect.DeepEqual(got, []int{3}) {
				t.Errorf("arm %d branch = %v, want [3]", i, got)
			}
		}
	})

	t.Run("a block reached only from an unreachable one", func(t *testing.T) {
		// 0: br 3 | 1: br 2 (dead) | 2: br 3 (dead, its only predecessor is dead) | 3: ret
		fn := cfgFunc(
			[]*module.Instruction{module.NewBrInstr(3)},
			[]*module.Instruction{module.NewBrInstr(2)},
			[]*module.Instruction{module.NewBrInstr(3)},
			[]*module.Instruction{module.NewRetVoidInstr()},
		)
		removeUnreachableBlocks(fn)
		if len(fn.BasicBlocks) != 2 {
			t.Fatalf("blocks = %d, want 2", len(fn.BasicBlocks))
		}
		if got := fn.BasicBlocks[0].Instructions[0].Operands; !reflect.DeepEqual(got, []int{1}) {
			t.Errorf("entry branch = %v, want [1]", got)
		}
	})

	t.Run("phi drops the incoming of an unreachable predecessor", func(t *testing.T) {
		// 0: br c, 1, 3 | 1: br 3 | 2: v20; br 3 (dead) | 3: phi [10 from 0, 11 from 1, 20 from 2]; ret
		phi := module.NewPhiInstr(nil, []module.PhiIncoming{
			{ValueID: 10, BBIndex: 0}, {ValueID: 11, BBIndex: 1}, {ValueID: 20, BBIndex: 2},
		})
		fn := cfgFunc(
			[]*module.Instruction{module.NewBrCondInstr(1, 3, 50)},
			[]*module.Instruction{module.NewBrInstr(3)},
			[]*module.Instruction{valueInstr(20), module.NewBrInstr(3)},
			[]*module.Instruction{phi, module.NewRetVoidInstr()},
		)
		removeUnreachableBlocks(fn)
		if len(fn.BasicBlocks) != 3 {
			t.Fatalf("blocks = %d, want 3", len(fn.BasicBlocks))
		}
		want := []module.PhiIncoming{{ValueID: 10, BBIndex: 0}, {ValueID: 11, BBIndex: 1}}
		if !reflect.DeepEqual(phi.PhiIncomings, want) {
			t.Errorf("phi incomings = %v, want %v", phi.PhiIncomings, want)
		}
		if got := fn.BasicBlocks[0].Instructions[0].Operands; !reflect.DeepEqual(got, []int{1, 2, 50}) {
			t.Errorf("entry branch = %v, want [1 2 50]", got)
		}
	})

	t.Run("a reachable use of an unreachable value leaves the function as it is", func(t *testing.T) {
		// 0: br 2 | 1: v20; br 2 (dead) | 2: v21 = v20 op v20; ret
		use := &module.Instruction{Kind: module.InstrBinOp, HasValue: true, ValueID: 21, Operands: []int{20, 20, 0}}
		fn := cfgFunc(
			[]*module.Instruction{module.NewBrInstr(2)},
			[]*module.Instruction{valueInstr(20), module.NewBrInstr(2)},
			[]*module.Instruction{use, module.NewRetVoidInstr()},
		)
		removeUnreachableBlocks(fn)
		if len(fn.BasicBlocks) != 3 {
			t.Fatalf("blocks = %d, want 3 (unchanged)", len(fn.BasicBlocks))
		}
		if got := fn.BasicBlocks[0].Instructions[0].Operands; !reflect.DeepEqual(got, []int{2}) {
			t.Errorf("entry branch = %v, want [2] (unchanged)", got)
		}
	})

	t.Run("every block reachable", func(t *testing.T) {
		fn := cfgFunc(
			[]*module.Instruction{module.NewBrCondInstr(1, 2, 50)},
			[]*module.Instruction{module.NewBrInstr(2)},
			[]*module.Instruction{module.NewRetVoidInstr()},
		)
		before := append([]*module.BasicBlock(nil), fn.BasicBlocks...)
		removeUnreachableBlocks(fn)
		if !reflect.DeepEqual(fn.BasicBlocks, before) {
			t.Errorf("blocks changed")
		}
		if got := fn.BasicBlocks[0].Instructions[0].Operands; !reflect.DeepEqual(got, []int{1, 2, 50}) {
			t.Errorf("entry branch = %v, want [1 2 50]", got)
		}
	})
}
