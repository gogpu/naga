// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// atomicLoadStoreSource stores to and loads from atomics in both address
// spaces, through a scalar, a struct member and an array element.
const atomicLoadStoreSource = `
struct S {
    scalar: atomic<u32>,
    arr: array<atomic<i32>, 2>,
}
@group(0) @binding(0) var<storage, read_write> s_scalar: atomic<u32>;
@group(0) @binding(1) var<storage, read_write> s_struct: S;
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
    atomicStore(&s_scalar, a + c + d + u32(b));
}
`

// TestAtomicLoadStorePolyfill pins atomicStore and atomicLoad as atomic
// operations. GLSL has no atomic load or store, so a plain assignment or read
// races with the other invocations' atomics; Rust naga writes atomicExchange
// and atomicOr(p, 0) (gfx-rs/wgpu f0201c9c, #9387), and so do we.
func TestAtomicLoadStorePolyfill(t *testing.T) {
	output := wgslToGLSL(t, atomicLoadStoreSource, Options{LangVersion: Version430})
	for _, want := range []string{
		"atomicExchange(_group_0_binding_0_cs, 1u);",
		"atomicExchange(_group_0_binding_1_cs.arr[1], 2);",
		"atomicExchange(w_scalar, 3u);",
		"atomicExchange(w_struct.scalar, 4u);",
		"uint a = atomicOr(_group_0_binding_0_cs, 0u);",
		"int b = atomicOr(_group_0_binding_1_cs.arr[1], 0);",
		"uint c = atomicOr(w_scalar, 0u);",
		"uint d = atomicOr(w_struct.scalar, 0u);",
	} {
		glslMustContain(t, output, want)
	}
	for _, plain := range []string{
		"_group_0_binding_0_cs = 1u;",
		"w_scalar = 3u;",
		"w_struct.scalar = 4u;",
		"uint c = w_scalar;",
	} {
		if strings.Contains(output, plain) {
			t.Errorf("output has the non-atomic %q.\nGot:\n%s", plain, output)
		}
	}
}

// TestAtomicSubNegativeValue pins the space after the negation that emulates
// atomicSub: without it a negative operand becomes the predecrement `--1`,
// which is not valid GLSL (gfx-rs/wgpu cdc9d0c4, #9924; Rust's test
// 9883-atomic_sub-negative).
func TestAtomicSubNegativeValue(t *testing.T) {
	const source = `
var<workgroup> a: atomic<i32>;

@compute @workgroup_size(1)
fn main() {
    let x = atomicSub(&a, -1i);
}
`
	output := wgslToGLSL(t, source, Options{LangVersion: Version430})
	glslMustContain(t, output, "int _e2 = atomicAdd(a, - -1);")
	if strings.Contains(output, "--1") {
		t.Errorf("output has the predecrement --1.\nGot:\n%s", output)
	}
}

// TestImageAtomicSubNegativeValue pins the same space for an image atomic
// subtract. WGSL has no textureAtomicSub, so the test turns the IR of a
// textureAtomicAdd into one; another frontend can produce it directly.
func TestImageAtomicSubNegativeValue(t *testing.T) {
	const source = `
@group(0) @binding(0) var img: texture_storage_2d<r32sint, read_write>;

@compute @workgroup_size(1)
fn main() {
    textureAtomicAdd(img, vec2<i32>(0, 0), -1i);
}
`
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("WGSL lower error: %v", err)
	}
	body := module.EntryPoints[0].Function.Body
	found := false
	for i := range body {
		if s, ok := body[i].Kind.(ir.StmtImageAtomic); ok {
			s.Fun = ir.AtomicSubtract{}
			body[i].Kind = s
			found = true
		}
	}
	if !found {
		t.Fatal("no image atomic statement in the entry point")
	}
	output, _, err := Compile(module, Options{LangVersion: Version430})
	if err != nil {
		t.Fatalf("GLSL compile error: %v", err)
	}
	glslMustContain(t, output, ", - -1);")
	if strings.Contains(output, "--1") {
		t.Errorf("output has the predecrement --1.\nGot:\n%s", output)
	}
}
