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
	tokens, err := wgsl.NewLexer(src).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatal(err)
	}
	m, err := wgsl.LowerWithSource(ast, src)
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
