// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"
)

// TestAtomicLoadStorePolyfill pins atomicStore and atomicLoad as atomic
// operations. A plain Store or Load on a RWByteAddressBuffer or a groupshared
// variable races with the other invocations' Interlocked* calls; Rust naga
// writes InterlockedExchange into a dummy and InterlockedOr(p, 0)
// (gfx-rs/wgpu f0201c9c, #9387), and so do we. HLSL is wgpu's default DX12
// path (FXC), so the plain forms were a silent data race there.
func TestAtomicLoadStorePolyfill(t *testing.T) {
	const source = `
struct S {
    scalar: atomic<u32>,
    arr: array<atomic<i32>, 2>,
}
@group(0) @binding(0) var<storage, read_write> s_scalar: atomic<u32>;
@group(0) @binding(1) var<storage, read_write> s_struct: S;
@group(0) @binding(2) var<storage, read> dummy: array<u32>;
var<workgroup> w_scalar: atomic<u32>;
var<workgroup> w_struct: S;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&s_scalar, 1u);
    atomicStore(&s_struct.arr[1], 2i);
    atomicStore(&w_scalar, 3u);
    atomicStore(&w_struct.scalar, 4u);
    let a = atomicLoad(&s_scalar);
    let b = atomicLoad(&s_struct.arr[1]);
    let c = atomicLoad(&w_scalar);
    let d = atomicLoad(&w_struct.scalar);
    _ = atomicLoad(&w_scalar);
    atomicStore(&s_scalar, a + c + d + u32(b) + dummy[0]);
}
`
	code := compileWGSLToHLSL(t, source, nil)
	// The dummy names go through the namer, so a global called "dummy"
	// moves the first one to dummy_1, as in Rust's globals.hlsl.
	mustContain(t, code, []string{
		"{ uint dummy_1 = 0; s_scalar.InterlockedExchange(0, 1u, dummy_1); }",
		"{ int dummy_2 = 0; s_struct.InterlockedExchange(4+4, int(2), dummy_2); }",
		"{ uint dummy_3 = 0; InterlockedExchange(w_scalar, 3u, dummy_3); }",
		"{ uint dummy_4 = 0; InterlockedExchange(w_struct.scalar, 4u, dummy_4); }",
		"uint a; s_scalar.InterlockedOr(0, 0, a);",
		"int b; s_struct.InterlockedOr(4+4, 0, b);",
		"uint c; InterlockedOr(w_scalar, 0, c);",
		"uint d; InterlockedOr(w_struct.scalar, 0, d);",
	})
	for _, plain := range []string{
		"s_scalar.Store(0, asuint(1u));",
		"w_scalar = 3u;",
		"w_struct.scalar = 4u;",
		"uint c = w_scalar;",
		"s_scalar.Load(0)",
	} {
		if strings.Contains(code, plain) {
			t.Errorf("output has the non-atomic %q.\nGot:\n%s", plain, code)
		}
	}
	// An atomicLoad whose result is unused is still written, as an atomic
	// read, not dropped or inlined into nothing.
	if n := strings.Count(code, "InterlockedOr(w_scalar, 0, "); n != 2 {
		t.Errorf("InterlockedOr(w_scalar, 0, …) written %d times, want 2.\nGot:\n%s", n, code)
	}
}

// TestAtomicSubNegative64 pins the space after the negation that emulates
// atomicSub when the operand itself starts with a minus: a 64-bit literal is
// written -1L, and "--1L" is a predecrement DXC rejects ("expression is not
// assignable"). Rust naga writes "-" unconditionally and has this bug; a
// 32-bit literal is safe there because it is written int(-1).
func TestAtomicSubNegative64(t *testing.T) {
	const source = `
@group(0) @binding(0) var<storage, read_write> a: atomic<i64>;
var<workgroup> b: atomic<i64>;

@compute @workgroup_size(1)
fn main() {
    let x = atomicSub(&a, -1li);
    let y = atomicSub(&b, -2li);
}
`
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.ShaderModel = ShaderModel6_6
	code := compileWGSLToHLSL(t, source, opts)
	mustContain(t, code, []string{
		"int64_t _e2; a.InterlockedAdd64(0, - -1L, _e2);",
		"int64_t _e5; InterlockedAdd(b, - -2L, _e5);",
	})
	if strings.Contains(code, "--") {
		t.Errorf("output has a predecrement.\nGot:\n%s", code)
	}
}

// TestAtomicSubKeepsRustForm pins Rust's "-" without a space where the
// operand does not start with a minus, so the outputs stay equal to Rust's.
func TestAtomicSubKeepsRustForm(t *testing.T) {
	const source = `
var<workgroup> u: atomic<u32>;
var<workgroup> i: atomic<i32>;

@compute @workgroup_size(1)
fn main() {
    let x = atomicSub(&u, 1u);
    let y = atomicSub(&i, -1i);
}
`
	code := compileWGSLToHLSL(t, source, nil)
	mustContain(t, code, []string{
		"uint _e2; InterlockedAdd(u, -1u, _e2);",
		"int _e5; InterlockedAdd(i, -int(-1), _e5);",
	})
}

// TestAtomicLoadStorePolyfill64 pins the 64 form of the RWByteAddressBuffer
// methods for 64-bit storage atomics. Rust naga (f0201c9c) writes the 32-bit
// InterlockedExchange and InterlockedOr there; DXC accepts them with
// "conversion from larger type 'uint64_t' to smaller type 'unsigned int &'",
// cutting the value to 32 bits. Workgroup atomics use the overloaded
// intrinsics, as in Rust.
func TestAtomicLoadStorePolyfill64(t *testing.T) {
	const source = `
@group(0) @binding(0) var<storage, read_write> s_u: atomic<u64>;
@group(0) @binding(1) var<storage, read_write> s_i: array<atomic<i64>, 2>;
var<workgroup> w_u: atomic<u64>;

@compute @workgroup_size(1)
fn main() {
    atomicStore(&s_u, 1lu);
    atomicStore(&s_i[1], 2li);
    atomicStore(&w_u, 3lu);
    let a = atomicLoad(&s_u);
    let b = atomicLoad(&s_i[1]);
    let c = atomicLoad(&w_u);
    atomicStore(&s_u, a + c + u64(b));
}
`
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.ShaderModel = ShaderModel6_6
	code := compileWGSLToHLSL(t, source, opts)
	mustContain(t, code, []string{
		"{ uint64_t dummy = 0; s_u.InterlockedExchange64(0, 1uL, dummy); }",
		"{ int64_t dummy_1 = 0; s_i.InterlockedExchange64(8, 2L, dummy_1); }",
		"{ uint64_t dummy_2 = 0; InterlockedExchange(w_u, 3uL, dummy_2); }",
		"uint64_t a; s_u.InterlockedOr64(0, 0, a);",
		"int64_t b; s_i.InterlockedOr64(8, 0, b);",
		"uint64_t c; InterlockedOr(w_u, 0, c);",
	})
}
