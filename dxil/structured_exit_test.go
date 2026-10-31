// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// brTargets returns the names of the blocks bb's terminator branches to.
func brTargets(fn *module.Function, bb *module.BasicBlock) []string {
	term := bb.Instructions[len(bb.Instructions)-1]
	if term.Kind != module.InstrBr {
		return nil
	}
	ops := term.Operands[:1]
	if len(term.Operands) == 3 {
		ops = term.Operands[:2]
	}
	var names []string
	for _, t := range ops {
		names = append(names, fn.BasicBlocks[t].Name)
	}
	return names
}

// TestBreakLeavesOnlyTheSwitch (P0-DXIL-057): in the IR, as in WGSL and Rust
// naga, a break in a switch case leaves the switch. The emitter branched to
// the merge of the enclosing loop, so the loop ended at the first such break
// (dx12-dxil read back 6 where every other backend reads 9), and a break in a
// switch outside any loop did not compile.
func TestBreakLeavesOnlyTheSwitch(t *testing.T) {
	tests := []struct{ name, body string }{
		{"in a loop", `for (var i = 0u; i < 4u; i++) { switch i { case 2u: { break; } default: { dst[0] = dst[0] + 1u; } } }`},
		{"outside a loop", `switch dst[1] { case 2u: { if dst[2] == 0u { break; } dst[0] = 5u; } default: {} }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "@group(0) @binding(0) var<storage, read_write> dst: array<u32>;\n@compute @workgroup_size(1) fn main() {\n" + tt.body + "\n}"
			_, fn := emitMainModule(t, src)
			found := 0
			for _, bb := range fn.BasicBlocks {
				for _, target := range brTargets(fn, bb) {
					if target == "loop.merge" && strings.HasPrefix(bb.Name, "switch.") {
						t.Errorf("block %s inside the switch branches to the loop's merge", bb.Name)
					}
				}
				if bb.Name == "if.then" || bb.Name == "switch.case_0" {
					if ts := brTargets(fn, bb); len(ts) == 1 && ts[0] == "switch.merge" {
						found++
					}
				}
			}
			if found == 0 {
				t.Errorf("no break branches to the switch's merge")
			}
			compileWGSLToDXIL(t, src)
		})
	}
}

// TestEntryReturnBranchesToTheExit (P0-DXIL-052): a return inside control
// flow of an entry point ends the invocation. The emitter emitted nothing for
// it, so the invocation ran on: the bounds guard every compute kernel starts
// with did not stop out-of-range invocations from writing. Like DXC, the entry
// point has one exit block that every return branches to.
func TestEntryReturnBranchesToTheExit(t *testing.T) {
	const src = `@group(0) @binding(0) var<storage, read_write> dst: array<u32>;
@compute @workgroup_size(8) fn main(@builtin(local_invocation_index) i: u32) {
    if i >= dst[0] { return; }
    for (var k = 0u; k < 4u; k++) { if dst[k] == 9u { return; } }
    dst[i] = i;
}`
	_, fn := emitMainModule(t, src)
	rets, toExit := 0, 0
	for _, bb := range fn.BasicBlocks {
		if bb.Instructions[len(bb.Instructions)-1].Kind == module.InstrRet {
			rets++
			if bb.Name != "entry.exit" {
				t.Errorf("ret in block %s, want only in entry.exit", bb.Name)
			}
		}
		for _, target := range brTargets(fn, bb) {
			if target == "entry.exit" {
				toExit++
			}
		}
	}
	if rets != 1 {
		t.Errorf("ret instructions = %d, want 1", rets)
	}
	// The two returns and the end of the body.
	if toExit != 3 {
		t.Errorf("branches to entry.exit = %d, want 3", toExit)
	}
	compileWGSLToDXIL(t, src)
}
