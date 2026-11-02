// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package irutil_test

import (
	"testing"

	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
)

// TestInlineReinitializesCalleeLocalsPerCall: WGSL creates a function's
// variables afresh at every call, from their initializer or zero. The inliner
// copied a callee's locals into the caller with their initializer and never
// stored it at the inline point, so a call inside a loop saw the value its
// previous iteration left: `var t: u32; t += i; t += 1u; return t;` called for
// i = 0..3 gave 1, 3, 6, 10 instead of 1, 2, 3, 4 once the DXIL mem2reg kept
// such a local in memory (P0-DXIL-046 follow-up). Every inlined local is now
// stored at the inline point (as the WGSL lowerer does for a variable declared
// inside a loop) and keeps no initializer.
func TestInlineReinitializesCalleeLocalsPerCall(t *testing.T) {
	mod := lowerWGSL(t, `
fn zero_init(i: u32) -> u32 { var t: u32; t += i; t += 1u; return t; }
fn const_init(i: u32) -> u32 { var t = 5u; t += i; return t; }
fn loop_acc(n: u32) -> u32 { var sum = 0u; for (var k = 0u; k < n; k++) { sum += k + 1u; } return sum; }
fn branch(i: u32) -> u32 { var best: u32; if i > 1u { best = i; } return best; }
fn f(n: u32) -> u32 {
    var s = 0u;
    for (var i = 0u; i < n; i++) {
        s = s * 7u + zero_init(i) + const_init(i) * 3u + loop_acc(2u) * 5u + branch(i) * 11u;
    }
    return s;
}`)
	orig := mod.Clone()
	if err := irutil.InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	f := functionByName(t, mod, "f")
	for n := uint32(0); n < 5; n++ {
		want, err := newIREval(orig).call(functionByName(t, orig, "f"), []uint32{n})
		if err != nil {
			t.Fatal(err)
		}
		got, err := newIREval(mod).call(f, []uint32{n})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("f(%d) = %d after inlining, %d before", n, got, want)
		}
	}
	for i, lv := range f.LocalVars {
		if lv.Init != nil && lv.Name != "s" && lv.Name != "i" {
			t.Errorf("inlined local %d (%s) keeps an initializer; it must be stored at the inline point", i, lv.Name)
		}
	}
}

// A callee local whose run-time initializer the lowerer stores at its
// declaration is stored before it is read, so the inliner adds no zero store
// in front of it: that store would be dead and would keep the local out of
// registers.
func TestInlineAddsNoZeroStoreBeforeARunTimeInitializer(t *testing.T) {
	mod := lowerWGSL(t, `
fn rt(x: u32) -> u32 { var y = x * 2u; y += 1u; return y; }
fn f(n: u32) -> u32 {
    var s = 0u;
    for (var i = 0u; i < n; i++) { s = s + rt(i); }
    return s;
}`)
	if err := irutil.InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	f := functionByName(t, mod, "f")
	stores := 0
	walkStatements(f.Body, func(s ir.StatementKind) {
		st, ok := s.(ir.StmtStore)
		if !ok {
			return
		}
		if lv, ok := f.Expressions[st.Pointer].Kind.(ir.ExprLocalVariable); ok && f.LocalVars[lv.Variable].Name == "y" {
			stores++
		}
	})
	if stores != 2 {
		t.Errorf("stores to y = %d, want 2 (the declaration and the +=, no zero store)", stores)
	}
}
