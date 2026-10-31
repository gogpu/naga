// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package module

import (
	"strings"
	"testing"
)

func TestVerifyCFG(t *testing.T) {
	ret := func() *Instruction { return NewRetVoidInstr() }
	br := NewBrInstr
	brc := func(a, b int) *Instruction { return NewBrCondInstr(a, b, 0) }
	phi := func(from ...int) *Instruction {
		var in []PhiIncoming
		for _, f := range from {
			in = append(in, PhiIncoming{ValueID: 1, BBIndex: f})
		}
		return NewPhiInstr(nil, in)
	}
	add := &Instruction{Kind: InstrBinOp, HasValue: true}
	fn := func(blocks ...[]*Instruction) *Function {
		f := &Function{Name: "f"}
		for _, ins := range blocks {
			f.BasicBlocks = append(f.BasicBlocks, &BasicBlock{Instructions: ins})
		}
		return f
	}
	tests := []struct {
		name    string
		fn      *Function
		wantErr string // "" = valid
	}{
		{"diamond with a phi", fn(
			[]*Instruction{brc(1, 2)},
			[]*Instruction{br(3)},
			[]*Instruction{br(3)},
			[]*Instruction{phi(2, 1), add, ret()},
		), ""},
		{"unreachable block is a predecessor too", fn(
			[]*Instruction{br(2)},
			[]*Instruction{br(2)},
			[]*Instruction{phi(0, 1), ret()},
		), ""},
		{"no terminator", fn([]*Instruction{add}), "has no terminator"},
		{"instruction after the terminator", fn([]*Instruction{ret(), add}), "follows its terminator"},
		{"empty block", fn([]*Instruction{br(1)}, nil), "is empty"},
		{"branch out of the function", fn([]*Instruction{br(5)}), "branches to block 5"},
		{"phi from a block that is no predecessor", fn(
			[]*Instruction{brc(1, 2)},
			[]*Instruction{ret()},
			[]*Instruction{br(3)},
			[]*Instruction{phi(1, 2), ret()},
		), "phi incomings from blocks [1 2], predecessors are [2]"},
		{"phi missing a predecessor", fn(
			[]*Instruction{brc(1, 2)},
			[]*Instruction{br(3)},
			[]*Instruction{br(3)},
			[]*Instruction{phi(1), ret()},
		), "predecessors are [1 2]"},
		{"phi after another instruction", fn(
			[]*Instruction{br(1)},
			[]*Instruction{add, phi(0), ret()},
		), "not at the start of its block"},
		{"declaration", &Function{Name: "d", IsDeclaration: true}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn.VerifyCFG()
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("valid CFG rejected: %v", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("err = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}
