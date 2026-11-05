// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// Inlining a helper that already had its own helpers inlined adds no store,
// and so no alloca, for the locals the inliner made in it: return slots,
// "returned" flags and argument spills are written before they are read in
// every run of the inlined body, and mem2reg promotes them (P2-DXIL-088). A
// zero store at every inline point kept them in memory: gg's coarse shader,
// whose minf/maxf/clamp_u32 return inside ifs and are called from helpers in
// loops, went from 74 allocas to 139. The shader below has 11 allocas when
// the inliner stores none of its own locals (the count before the inline
// point stores of 6f84482) and had 16 with them.
func TestInlinedHelperChainAllocas(t *testing.T) {
	fn := emitMain(t, `
@group(0) @binding(0) var<storage, read_write> data: array<u32, 8>;
fn minu(a: u32, b: u32) -> u32 { if a < b { return a; } return b; }
fn clampu(v: u32, lo: u32, hi: u32) -> u32 { if v < lo { return lo; } if v > hi { return hi; } return v; }
fn row(i: u32) -> u32 {
    var r = minu(i, 3u) + clampu(i * 2u, 1u, 5u);
    if i > 1u { r += minu(clampu(i, 0u, 2u), 9u); }
    return r;
}
@compute @workgroup_size(1) fn main() {
    var s = 0u;
    for (var i = 0u; i < data[0]; i++) { s = s * 7u + row(i); }
    data[1] = s;
}
`)
	const want = 11
	allocas := 0
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind == module.InstrAlloca {
				allocas++
			}
		}
	}
	if allocas != want {
		t.Errorf("main has %d allocas, want %d: the inliner's own locals are re-initialized at the inline point", allocas, want)
	}
}
