// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package module

import (
	"fmt"
	"slices"
)

// VerifyCFG checks the control-flow invariants LLVM's verifier enforces that
// the emitter can break (lib/IR/Verifier.cpp, visitBasicBlock and visitPHINode):
//
//   - every block ends with exactly one terminator (ret or br), and nothing
//     follows it;
//   - every branch target is a block of the function;
//   - phis come first in their block, and each has exactly one incoming per
//     predecessor of that block, from exactly those predecessors.
//
// DXC's validator and the drivers' compilers do not all check these: a phi
// with an incoming from a block that is not a predecessor can pass
// IDxcValidator and read an undefined value on a GPU. Running the check on
// every compile turns such an emitter bug into a compile error.
func (f *Function) VerifyCFG() error {
	if f.IsDeclaration {
		return nil
	}
	if len(f.BasicBlocks) == 0 {
		return fmt.Errorf("function %q has no blocks", f.Name)
	}
	preds, err := f.verifyTerminators()
	if err != nil {
		return err
	}
	for bi, bb := range f.BasicBlocks {
		if err := f.verifyPhis(bi, bb, preds[bi]); err != nil {
			return err
		}
	}
	return nil
}

// verifyTerminators checks that every block ends with one terminator and
// branches inside the function, and returns each block's predecessors.
func (f *Function) verifyTerminators() ([][]int, error) {
	n := len(f.BasicBlocks)
	preds := make([][]int, n)
	for bi, bb := range f.BasicBlocks {
		if len(bb.Instructions) == 0 {
			return nil, fmt.Errorf("function %q: block %d (%s) is empty", f.Name, bi, bb.Name)
		}
		for ii, instr := range bb.Instructions {
			last := ii == len(bb.Instructions)-1
			isTerm := instr.Kind == InstrRet || instr.Kind == InstrBr
			if isTerm != last {
				if isTerm {
					return nil, fmt.Errorf("function %q: block %d (%s): instruction %d follows its terminator", f.Name, bi, bb.Name, ii+1)
				}
				return nil, fmt.Errorf("function %q: block %d (%s) has no terminator", f.Name, bi, bb.Name)
			}
		}
		for _, t := range branchTargets(bb.Instructions[len(bb.Instructions)-1]) {
			if t < 0 || t >= n {
				return nil, fmt.Errorf("function %q: block %d (%s) branches to block %d of %d", f.Name, bi, bb.Name, t, n)
			}
			// Each distinct predecessor counts once, as for LLVM's phis
			// (we never emit a conditional branch with both edges to one
			// block).
			if !slices.Contains(preds[t], bi) {
				preds[t] = append(preds[t], bi)
			}
		}
	}
	return preds, nil
}

// branchTargets returns the blocks a terminator branches to: none for ret,
// one for br, two for a conditional br (operands true, false, condition).
func branchTargets(term *Instruction) []int {
	switch {
	case term.Kind != InstrBr:
		return nil
	case len(term.Operands) == 3:
		return term.Operands[:2]
	default:
		return term.Operands[:1]
	}
}

// verifyPhis checks that block bi's phis come first and have one incoming
// from each predecessor in preds and from nothing else.
func (f *Function) verifyPhis(bi int, bb *BasicBlock, preds []int) error {
	want := slices.Sorted(slices.Values(preds))
	for ii, instr := range bb.Instructions {
		if instr.Kind != InstrPhi {
			if slices.ContainsFunc(bb.Instructions[ii:], func(in *Instruction) bool { return in.Kind == InstrPhi }) {
				return fmt.Errorf("function %q: block %d (%s): a phi follows instruction %d, not at the start of its block", f.Name, bi, bb.Name, ii)
			}
			return nil
		}
		from := make([]int, 0, len(instr.PhiIncomings))
		for _, in := range instr.PhiIncomings {
			from = append(from, in.BBIndex)
		}
		slices.Sort(from)
		if !slices.Equal(from, want) {
			return fmt.Errorf("function %q: block %d (%s): phi incomings from blocks %v, predecessors are %v", f.Name, bi, bb.Name, from, want)
		}
	}
	return nil
}
