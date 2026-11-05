// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestRefuseInt16 refuses 16-bit integer types and literals (enable
// wgpu_int16). Rust naga does not target GLSL with them (int16.toml has no
// GLSL target); its writer would name int16_t and uint16_t without the
// GL_EXT_shader_explicit_arithmetic_types_int16 extension that declares
// them, which glslang rejects, and GLSL ES has no such type. naga reports
// the error instead of writing GLSL that does not compile.
func TestRefuseInt16(t *testing.T) {
	for name, src := range map[string]string{
		"i16 storage": `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> a: array<i16, 2>;
@compute @workgroup_size(1) fn main() { a[0] = a[1] + i16(5); }`,
		"u16 vector": `enable wgpu_int16;
@group(0) @binding(0) var<storage, read_write> o: u32;
@compute @workgroup_size(1) fn main() { let v = vec2<u16>(u16(o), u16(2)); o = u32(v.x); }`,
	} {
		module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
		if err != nil {
			t.Fatalf("%s: WGSL lower error: %v", name, err)
		}
		code, _, err := Compile(module, Options{LangVersion: Version450})
		if !errors.Is(err, errNoInt16) {
			t.Errorf("%s: err = %v, want errNoInt16; output:\n%s", name, err, code)
		}
	}
}

// TestRefuseInt16Literal: a 16-bit literal alone is refused too.
func TestRefuseInt16Literal(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version450})
	for _, lit := range []ir.Literal{{Value: ir.LiteralI16(-1)}, {Value: ir.LiteralU16(1)}} {
		if _, err := w.writeLiteral(lit); !errors.Is(err, errNoInt16) {
			t.Errorf("writeLiteral(%v): err = %v, want errNoInt16", lit.Value, err)
		}
		if _, err := w.formatLiteral(lit); !errors.Is(err, errNoInt16) {
			t.Errorf("formatLiteral(%v): err = %v, want errNoInt16", lit.Value, err)
		}
	}
}
