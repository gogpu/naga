// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestInlineReadsAValueArgumentWhereTheCallerEvaluatedIt: after inlining
// f(acc, bump(&acc)), f's a.x reads the load of acc that was emitted before
// bump's store into acc, as WGSL evaluates the arguments left to right, so
// the shader writes 10 and not 11.
func TestInlineReadsAValueArgumentWhereTheCallerEvaluatedIt(t *testing.T) {
	mod := lowerWGSL(t, `
struct Out { v: i32 }
@group(0) @binding(0) var<storage, read_write> out: Out;
fn bump(p: ptr<function, vec2<i32>>) -> i32 { *p = *p + vec2<i32>(1, 1); return 0; }
fn f(a: vec2<i32>, b: i32) -> i32 { return a.x + b; }
@compute @workgroup_size(1) fn main() {
	var acc = vec2<i32>(10, 20);
	out.v = f(acc, bump(&acc));
}`)
	if err := ir.InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	fn := &mod.EntryPoints[0].Function
	emittedAt := func(h ir.ExpressionHandle) int {
		for i, s := range fn.Body {
			if em, ok := s.Kind.(ir.StmtEmit); ok && em.Range.Start <= h && h < em.Range.End {
				return i
			}
		}
		return -1
	}

	store, read := -1, -1
	for i, s := range fn.Body {
		st, ok := s.Kind.(ir.StmtStore)
		if !ok {
			continue
		}
		if lv, ok := fn.Expressions[st.Pointer].Kind.(ir.ExprLocalVariable); ok && fn.LocalVars[lv.Variable].Name == "acc" {
			store = i
			break
		}
	}
	for h, e := range fn.Expressions {
		ai, ok := e.Kind.(ir.ExprAccessIndex)
		if !ok || emittedAt(ir.ExpressionHandle(h)) < 0 {
			continue
		}
		if _, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLoad); ok {
			read = emittedAt(ai.Base)
		}
	}
	if store < 0 || read < 0 || read > store {
		t.Errorf("a.x reads a load of acc emitted at statement %d, bump stores acc at %d, body=%+v", read, store, fn.Body)
	}
}
