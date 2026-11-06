// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package emit

import "github.com/gogpu/naga/dxil/internal/module"

// removeUnreachableBlocks drops the blocks of fn that no path from the entry
// block reaches, renumbering the branch targets and phi predecessors of the
// blocks that stay. It runs before finalize, so the dropped instructions take
// no value numbers.
//
// The emitter opens a block for the code after a statement before it knows
// whether control reaches it: an if/else whose arms both return, a loop body
// after an unconditional break. That block then holds only the branch the
// function end adds, and nothing branches to it. DXC never emits one (its
// SimplifyCFG removes unreachable blocks), nor does Mesa (it emits blocks
// from NIR's structured CFG after dead control flow is removed), and D3D12
// GPU-based validation mishandles such a block as a predecessor of the merge.
//
// A reachable instruction that uses a value defined in an unreachable block
// would break dominance whether or not the block is dropped; fn is then left
// as it is, so this pass never turns one invalid module into another.
func removeUnreachableBlocks(fn *module.Function) {
	n := len(fn.BasicBlocks)
	if n < 2 {
		return
	}
	reachable := reachableBlocks(fn)
	newIndex := make([]int, n)
	kept := 0
	for i := range fn.BasicBlocks {
		if reachable[i] {
			newIndex[i] = kept
			kept++
		} else {
			newIndex[i] = -1
		}
	}
	if kept == n || usesValueOfUnreachableBlock(fn, reachable) {
		return
	}

	blocks := make([]*module.BasicBlock, 0, kept)
	for i, bb := range fn.BasicBlocks {
		if !reachable[i] {
			continue
		}
		for _, instr := range bb.Instructions {
			renumberBlockRefs(instr, newIndex)
		}
		blocks = append(blocks, bb)
	}
	fn.BasicBlocks = blocks
}

// reachableBlocks marks the blocks of fn that a path from the entry block
// (index 0) reaches through branch edges.
func reachableBlocks(fn *module.Function) []bool {
	reachable := make([]bool, len(fn.BasicBlocks))
	reachable[0] = true
	work := []int{0}
	for len(work) > 0 {
		bb := fn.BasicBlocks[work[len(work)-1]]
		work = work[:len(work)-1]
		for _, instr := range bb.Instructions {
			for _, succ := range branchTargets(instr) {
				if succ >= 0 && succ < len(reachable) && !reachable[succ] {
					reachable[succ] = true
					work = append(work, succ)
				}
			}
		}
	}
	return reachable
}

// branchTargets returns the block indices a branch instruction can jump to:
// [bb] for an unconditional branch, [trueBB, falseBB] for a conditional one
// (whose operands are [trueBB, falseBB, cond]).
func branchTargets(instr *module.Instruction) []int {
	if instr.Kind != module.InstrBr {
		return nil
	}
	switch len(instr.Operands) {
	case 1:
		return instr.Operands[:1]
	case 3:
		return instr.Operands[:2]
	}
	return nil
}

// usesValueOfUnreachableBlock reports whether an instruction of a reachable
// block uses a value an unreachable block defines. A phi incoming whose
// predecessor is unreachable does not count: it is dropped with the block.
func usesValueOfUnreachableBlock(fn *module.Function, reachable []bool) bool {
	dead := map[int]bool{}
	for i, bb := range fn.BasicBlocks {
		if reachable[i] {
			continue
		}
		for _, instr := range bb.Instructions {
			if instr.HasValue {
				dead[instr.ValueID] = true
			}
		}
	}
	if len(dead) == 0 {
		return false
	}
	for i, bb := range fn.BasicBlocks {
		if !reachable[i] {
			continue
		}
		for _, instr := range bb.Instructions {
			if usesValueOf(instr, dead, reachable) {
				return true
			}
		}
	}
	return false
}

// usesValueOf reports whether instr uses one of the values in dead, other
// than through a phi incoming from an unreachable predecessor.
func usesValueOf(instr *module.Instruction, dead map[int]bool, reachable []bool) bool {
	if instr.Kind == module.InstrRet && instr.ReturnValue >= 0 && dead[instr.ReturnValue] {
		return true
	}
	for _, idx := range valueOperandIndices(instr) {
		if idx < len(instr.Operands) && dead[instr.Operands[idx]] {
			return true
		}
	}
	for _, in := range instr.PhiIncomings {
		if in.BBIndex >= 0 && in.BBIndex < len(reachable) && reachable[in.BBIndex] && dead[in.ValueID] {
			return true
		}
	}
	return false
}

// renumberBlockRefs rewrites the block indices of a branch or phi to the
// numbering after the unreachable blocks are dropped. Phi incomings from a
// dropped block go away with it.
func renumberBlockRefs(instr *module.Instruction, newIndex []int) {
	switch instr.Kind {
	case module.InstrBr:
		switch len(instr.Operands) {
		case 1:
			instr.Operands[0] = newIndex[instr.Operands[0]]
		case 3:
			instr.Operands[0] = newIndex[instr.Operands[0]]
			instr.Operands[1] = newIndex[instr.Operands[1]]
		}
	case module.InstrPhi:
		incomings := instr.PhiIncomings[:0]
		for _, in := range instr.PhiIncomings {
			if in.BBIndex < 0 || in.BBIndex >= len(newIndex) || newIndex[in.BBIndex] < 0 {
				continue
			}
			in.BBIndex = newIndex[in.BBIndex]
			incomings = append(incomings, in)
		}
		instr.PhiIncomings = incomings
	}
}
