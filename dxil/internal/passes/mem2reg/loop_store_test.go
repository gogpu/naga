// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package mem2reg

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestRunKeepsStoreBeforeLoopOfLoopStoredVariable: a variable stored inside
// a loop is not promoted (no loop-header phis), so every store to it must
// survive. The structured pass dropped the store before the loop (`var j = i`
// lowers to one) and only disqualified the variable when it reached the
// loop, so the loop read an uninitialized alloca.
func TestRunKeepsStoreBeforeLoopOfLoopStoredVariable(t *testing.T) {
	const src = `
@group(0) @binding(0) var<storage, read_write> result: array<u32>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    for (var j = i; j < 4u; j++) {
        result[j] = j + 1u;
    }
}`
	m, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fn := &m.EntryPoints[0].Function
	before := topLevelLocalStores(fn)
	if before == 0 {
		t.Fatal("expected the lowerer to emit a store before the loop")
	}
	if err := Run(m, fn); err != nil {
		t.Fatal(err)
	}
	if after := topLevelLocalStores(fn); after != before {
		t.Errorf("stores to locals before the loop: %d, want %d (kept)", after, before)
	}
}

// TestRunKeepsLoopCarriedVariable: a variable whose stores and loads all sit
// in one block inside a loop, read before it is stored in that block, carries
// its value from one iteration to the next (P0-DXIL-046). The single-block
// pass promoted it as if the block ran once: the load became the value before
// the loop and the store was dropped, so `x += i + 1u; result[i] = x;` wrote
// 1, 2, 3, 4 instead of 1, 3, 6, 10 (and the `loop` form froze its counter,
// hanging the GPU). LLVM's mem2reg gives such a load the loop-header phi; we
// have no loop-header phis yet, so the variable keeps its alloca, as the
// structured pass already does for variables stored in a loop.
func TestRunKeepsLoopCarriedVariable(t *testing.T) {
	const prelude = `
@group(0) @binding(0) var<storage, read> count: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> result: array<u32>;
@compute @workgroup_size(1)
fn main() {
`
	for name, body := range map[string]string{
		"for":   `var x: u32; for (var i = 0u; i < count[0]; i++) { x += i + 1u; result[i] = x; }`,
		"init":  `var x: u32 = 0u; for (var i = 0u; i < count[0]; i++) { x += i + 1u; result[i] = x; }`,
		"loop":  `var x: u32; var i: u32 = 0u; loop { if i >= count[0] { break; } x += i + 1u; result[i] = x; i++; }`,
		"while": `var x: u32; var i: u32 = 0u; while i < count[0] { x += i + 1u; result[i] = x; i++; }`,
	} {
		t.Run(name, func(t *testing.T) {
			m, _, err := wgsl.Parse(prelude+body+"\n}\n", wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			fn := &m.EntryPoints[0].Function
			// x is the accumulator; i is the counter, which the `loop`
			// form reads and increments in the same block (promoted, it
			// stayed 0 and the loop never ended).
			before := map[string]int{"x": localStores(fn, fn.Body, "x"), "i": localStores(fn, fn.Body, "i")}
			if before["x"] == 0 {
				t.Fatal("expected a store to x in the loop")
			}
			if err := Run(m, fn); err != nil {
				t.Fatal(err)
			}
			for v, n := range before {
				if after := localStores(fn, fn.Body, v); after != n {
					t.Errorf("stores to %s: %d, want %d: the loop-carried variable was promoted", v, after, n)
				}
			}
		})
	}
}

// TestRunPromotesVariableDeclaredInLoop guards the fix above against
// over-correction: a variable declared inside the loop body is stored (its
// initializer, or zero) before every read in that block, so each iteration
// starts afresh and single-block promotion stays right.
func TestRunPromotesVariableDeclaredInLoop(t *testing.T) {
	for name, decl := range map[string]string{
		"initializer":    `var y = 5u;`,
		"no initializer": `var y: u32;`,
	} {
		t.Run(name, func(t *testing.T) {
			m, _, err := wgsl.Parse(`
@group(0) @binding(0) var<storage, read> count: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> result: array<u32>;
@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < count[0]; i++) { `+decl+` y += i; result[i] = y; }
}
`, wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			fn := &m.EntryPoints[0].Function
			if localStores(fn, fn.Body, "y") == 0 {
				t.Fatal("expected stores to y in the loop")
			}
			if err := Run(m, fn); err != nil {
				t.Fatal(err)
			}
			if n := localStores(fn, fn.Body, "y"); n != 0 {
				t.Errorf("stores to y after mem2reg: %d, want 0 (promoted)", n)
			}
		})
	}
}

// localStores counts the stores, at any depth of block, to the local named
// name.
func localStores(fn *ir.Function, block ir.Block, name string) int {
	n := 0
	for _, s := range block {
		switch k := s.Kind.(type) {
		case ir.StmtStore:
			if lv, ok := fn.Expressions[k.Pointer].Kind.(ir.ExprLocalVariable); ok && fn.LocalVars[lv.Variable].Name == name {
				n++
			}
		case ir.StmtBlock:
			n += localStores(fn, k.Block, name)
		case ir.StmtIf:
			n += localStores(fn, k.Accept, name) + localStores(fn, k.Reject, name)
		case ir.StmtLoop:
			n += localStores(fn, k.Body, name) + localStores(fn, k.Continuing, name)
		case ir.StmtSwitch:
			for ci := range k.Cases {
				n += localStores(fn, k.Cases[ci].Body, name)
			}
		}
	}
	return n
}

func topLevelLocalStores(fn *ir.Function) int {
	n := 0
	for _, s := range fn.Body {
		if st, ok := s.Kind.(ir.StmtStore); ok {
			if _, isLocal := fn.Expressions[st.Pointer].Kind.(ir.ExprLocalVariable); isLocal {
				n++
			}
		}
	}
	return n
}
