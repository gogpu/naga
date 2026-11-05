// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package irutil_test

import (
	"strings"
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

// TestInlineReinitializesAggregateCalleeLocalsPerCall: a struct, array or
// matrix local of an inlined function that the function writes starts from
// its initializer, or zero, at every call too (P0-DXIL-091). The inliner left
// such a local its initializer, which the caller sets once, at its entry, so
// a call inside a loop started from what the previous iteration left. Each one
// now gets a store at the inline point, inside the loop, and keeps no
// initializer. A local the function never writes holds its initializer at
// every call anyway and keeps it: a store would only cost the backend an
// alloca.
func TestInlineReinitializesAggregateCalleeLocalsPerCall(t *testing.T) {
	mod := lowerWGSL(t, `
struct S { a: u32, b: u32 }
fn arr_acc(x: u32) -> u32 { var arr = array<u32, 2>(5u, 6u); arr[1] = arr[1] + x; return arr[1]; }
fn st_acc(x: u32) -> u32 { var st = S(5u, 6u); st.b = st.b + x; return st.b; }
fn nest_acc(x: u32) -> u32 {
    var nest = array<array<u32, 2>, 2>(array<u32, 2>(1u, 2u), array<u32, 2>(5u, 6u));
    nest[1][1] = nest[1][1] + x;
    return nest[1][1];
}
fn mat_acc(x: f32) -> f32 { var m = mat2x2<f32>(1.0, 2.0, 3.0, 4.0); m[1] = m[1] + vec2<f32>(x); return m[1].y; }
fn zero_acc(x: u32) -> u32 { var zarr: array<u32, 2>; zarr[1] = zarr[1] + x; return zarr[1]; }
fn untouched(i: u32) -> u32 { var table = array<u32, 2>(7u, 8u); return table[i % 2u]; }
fn f(n: u32) -> u32 {
    var s = 0u;
    for (var i = 0u; i < n; i++) {
        s += arr_acc(i) + st_acc(i) + nest_acc(i) + zero_acc(i) + untouched(i) + u32(mat_acc(f32(i)));
    }
    return s;
}`)
	if err := irutil.InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	f := functionByName(t, mod, "f")
	var loopBody ir.Block
	walkStatements(f.Body, func(s ir.StatementKind) {
		if l, ok := s.(ir.StmtLoop); ok && loopBody == nil {
			loopBody = l.Body
		}
	})
	storedInLoop := map[string]bool{}
	walkStatements(loopBody, func(s ir.StatementKind) {
		st, ok := s.(ir.StmtStore)
		if !ok {
			return
		}
		if lv, ok := f.Expressions[st.Pointer].Kind.(ir.ExprLocalVariable); ok {
			storedInLoop[f.LocalVars[lv.Variable].Name] = true
		}
	})
	for i, lv := range f.LocalVars {
		switch lv.Name {
		case "arr", "st", "nest", "m", "zarr":
			if lv.Init != nil {
				t.Errorf("inlined local %d (%s) keeps an initializer; it must be stored at the inline point", i, lv.Name)
			}
			if !storedInLoop[lv.Name] {
				t.Errorf("inlined local %d (%s) is not stored whole inside the loop: a call starts from the previous call's value", i, lv.Name)
			}
		case "table":
			if lv.Init == nil || storedInLoop[lv.Name] {
				t.Errorf("inlined local %d (table) is never written by its function; it keeps its initializer and gets no store", i)
			}
		}
	}
}

// TestInlineDoesNotReinitializeItsOwnLocals: the locals the inliner makes
// itself (a return slot, a "returned" flag, an argument spill) are written
// before they are read in every run of the inlined body: the return slot on
// every return path, the flag cleared at its start, the spill at the call.
// When a function that already holds them (its callees were inlined into it)
// is inlined in turn, they need no initializer store at the inline point; one
// was added for each, which only kept them out of registers (P2-DXIL-088: gg
// coarse went from 74 to 139 allocas). Rows of a helper inlined into a helper
// inlined in a loop, as gg's minf/maxf/clamp_u32 are.
func TestInlineDoesNotReinitializeItsOwnLocals(t *testing.T) {
	src := `
fn minu(a: u32, b: u32) -> u32 { if a < b { return a; } return b; }
fn clampu(v: u32, lo: u32, hi: u32) -> u32 { if v < lo { return lo; } if v > hi { return hi; } return v; }
fn row(i: u32) -> u32 {
    var r = minu(i, 3u) + clampu(i * 2u, 1u, 5u);
    if i > 1u { r += minu(clampu(i, 0u, 2u), 9u); }
    return r;
}
fn f(n: u32) -> u32 {
    var s = 0u;
    for (var i = 0u; i < n; i++) { s = s * 7u + row(i); }
    return s;
}`
	mod := lowerWGSL(t, src)
	orig := mod.Clone()
	if err := irutil.InlineUserFunctions(mod, func(*ir.Function) bool { return true }); err != nil {
		t.Fatal(err)
	}
	f := functionByName(t, mod, "f")
	for n := uint32(0); n < 6; n++ {
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
	// The names are the inliner's, and only the test reads them.
	walkStatements(f.Body, func(s ir.StatementKind) {
		st, ok := s.(ir.StmtStore)
		if !ok {
			return
		}
		lv, ok := f.Expressions[st.Pointer].Kind.(ir.ExprLocalVariable)
		if !ok {
			return
		}
		name := f.LocalVars[lv.Variable].Name
		if _, zero := f.Expressions[st.Value].Kind.(ir.ExprZeroValue); zero && strings.HasPrefix(name, "_inline_") {
			t.Errorf("the inliner's own local %d (%s) gets a zero store at an inline point", lv.Variable, name)
		}
	})
}
