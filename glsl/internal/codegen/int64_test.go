// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestRefuseInt64 refuses 64-bit integer types and literals, as Rust
// naga's GLSL writer does (glsl_scalar: UnsupportedScalar; literals: "GLSL
// has no 64-bit integer type"). We wrote int64_t and 5000000000L without
// the extension that declares them, which glslang rejects ("syntax error,
// unexpected IDENTIFIER"), while Compile reported success
// (BUG-F16-SILENT-BACKENDS-001).
func TestRefuseInt64(t *testing.T) {
	for name, src := range map[string]string{
		"i64 storage": `
@group(0) @binding(0) var<storage, read_write> a: array<i64, 2>;
@compute @workgroup_size(1) fn main() { a[0] = a[1] + 5000000000li; }`,
		"u64 storage": `
@group(0) @binding(0) var<storage, read_write> b: array<u64, 2>;
@compute @workgroup_size(1) fn main() { b[0] = b[1] * 3lu; }`,
		"u64 constant": `
const k = 18446744073709551615lu;
@group(0) @binding(0) var<storage, read_write> o: u32;
@compute @workgroup_size(1) fn main() { o = u32(k >> 32u); }`,
	} {
		module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
		if err != nil {
			t.Fatalf("%s: WGSL lower error: %v", name, err)
		}
		code, _, err := Compile(module, Options{LangVersion: Version450})
		if !errors.Is(err, errNoInt64) {
			t.Errorf("%s: err = %v, want errNoInt64; output:\n%s", name, err, code)
		}
	}
}

// TestRefuseInt64Literal: a 64-bit literal alone, without a 64-bit type
// the writer names, is refused too.
func TestRefuseInt64Literal(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version450})
	for _, lit := range []ir.Literal{{Value: ir.LiteralI64(-1)}, {Value: ir.LiteralU64(1)}} {
		if _, err := w.writeLiteral(lit); !errors.Is(err, errNoInt64) {
			t.Errorf("writeLiteral(%v): err = %v, want errNoInt64", lit.Value, err)
		}
		if _, err := w.formatLiteral(lit); !errors.Is(err, errNoInt64) {
			t.Errorf("formatLiteral(%v): err = %v, want errNoInt64", lit.Value, err)
		}
	}
}
