// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

var (
	f32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	i32  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	u32  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	vec2 = ir.VectorType{Size: ir.Vec2, Scalar: f32}
)

// TestImmediateItems: OpenGL has no immediate data, so the writer declares
// the immediate variable as a plain uniform and TranslationInfo lists its
// leaf scalars, vectors and matrices, each with the GL name the runtime
// looks up and the bytes of the immediate data it is fed from (Rust naga's
// ReflectionInfo.immediates_items).
//
// The offsets are the variable's WGSL layout, the one setImmediateData
// writes. Rust naga instead walks the members one after another, rounding
// each up to its alignment and ignoring StructMember.offset, so a member
// that follows an @size or @align member can get the wrong offset: for
// "sizealign" Rust reports b, c, d at 4, 8, 16 and for "nested struct size"
// e2.x, e2.y, z at 8, 12, 16, though its own IR places sizealign's b, c, d
// at 16, 32, 40 (checked on gfx-rs/wgpu 14328f49). The cases without those
// attributes match Rust's output for the same shader.
func TestImmediateItems(t *testing.T) {
	tests := []struct {
		name string
		src  string
		ep   string
		want []glsl.ImmediateItem
	}{
		{
			name: "nested",
			src: `
struct Inner { value: f32, v2: vec2<f32> }
struct Imm { inner: Inner, m: mat3x3<f32>, m2: mat2x2<f32>, s: u32, v3: vec3<i32>, t: f32 }
var<immediate> im: Imm;
var<workgroup> w: f32;
@compute @workgroup_size(1) fn main() { w = im.inner.value + im.t; }`,
			ep: "main",
			want: []glsl.ImmediateItem{
				{AccessPath: "_immediates_binding_cs.inner.value", Type: f32, Offset: 0, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.inner.v2_", Type: vec2, Offset: 8, SizeBytes: 8},
				{AccessPath: "_immediates_binding_cs.m", Type: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32}, Offset: 16, SizeBytes: 48},
				{AccessPath: "_immediates_binding_cs.m2_", Type: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32}, Offset: 64, SizeBytes: 16},
				{AccessPath: "_immediates_binding_cs.s", Type: u32, Offset: 80, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.v3_", Type: ir.VectorType{Size: ir.Vec3, Scalar: i32}, Offset: 96, SizeBytes: 12},
				{AccessPath: "_immediates_binding_cs.t", Type: f32, Offset: 108, SizeBytes: 4},
			},
		},
		{
			name: "sizealign",
			src: `
struct S { @size(16) a: f32, b: f32, @align(16) c: vec2<f32>, d: f32 }
var<immediate> im: S;
var<workgroup> w: f32;
@compute @workgroup_size(1) fn main() { w = im.d; }`,
			ep: "main",
			want: []glsl.ImmediateItem{
				{AccessPath: "_immediates_binding_cs.a", Type: f32, Offset: 0, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.b", Type: f32, Offset: 16, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.c", Type: vec2, Offset: 32, SizeBytes: 8},
				{AccessPath: "_immediates_binding_cs.d", Type: f32, Offset: 40, SizeBytes: 4},
			},
		},
		{
			name: "nested struct size",
			src: `
struct E { x: f32, @size(12) y: f32 }
struct P { e1: E, e2: E, @align(16) z: f32 }
var<immediate> im: P;
var<workgroup> w: f32;
@compute @workgroup_size(1) fn main() { w = im.z; }`,
			ep: "main",
			want: []glsl.ImmediateItem{
				{AccessPath: "_immediates_binding_cs.e1_.x", Type: f32, Offset: 0, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.e1_.y", Type: f32, Offset: 4, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.e2_.x", Type: f32, Offset: 16, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.e2_.y", Type: f32, Offset: 20, SizeBytes: 4},
				{AccessPath: "_immediates_binding_cs.z", Type: f32, Offset: 32, SizeBytes: 4},
			},
		},
		{
			name: "bare vector",
			src: `
var<immediate> im: vec4<f32>;
var<workgroup> w: f32;
@compute @workgroup_size(1) fn main() { w = im.x; }`,
			ep: "main",
			want: []glsl.ImmediateItem{
				{AccessPath: "_immediates_binding_cs", Type: ir.VectorType{Size: ir.Vec4, Scalar: f32}, Offset: 0, SizeBytes: 16},
			},
		},
		{
			name: "unused",
			src: `
struct S { a: f32 }
var<immediate> im: S;
var<workgroup> w: f32;
@compute @workgroup_size(1) fn main() { w = 1.0; }`,
			ep: "main",
		},
		// Rust's push-constants.wgsl: one immediate variable per stage.
		{
			name: "vertex",
			src:  pushConstantsWGSL,
			ep:   "vert_main",
			want: []glsl.ImmediateItem{
				{AccessPath: "_immediates_binding_vs.position_clip", Type: f32, Offset: 0, SizeBytes: 4},
				{AccessPath: "_immediates_binding_vs.matrix", Type: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32}, Offset: 16, SizeBytes: 48},
			},
		},
		{
			name: "fragment",
			src:  pushConstantsWGSL,
			ep:   "main",
			want: []glsl.ImmediateItem{
				{AccessPath: "_immediates_binding_fs.multiplier", Type: f32, Offset: 0, SizeBytes: 4},
				{AccessPath: "_immediates_binding_fs.tint", Type: ir.VectorType{Size: ir.Vec4, Scalar: f32}, Offset: 16, SizeBytes: 16},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, info, err := glsl.Compile(lowerWGSL(t, tt.src), glsl.Options{LangVersion: glsl.VersionES320, EntryPoint: tt.ep})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(info.ImmediateItems, tt.want) {
				t.Errorf("ImmediateItems:\n got %+v\nwant %+v", info.ImmediateItems, tt.want)
			}
			// Every path names a member the writer declared.
			for _, item := range info.ImmediateItems {
				for _, seg := range strings.Split(item.AccessPath, ".")[1:] {
					if !strings.Contains(out, " "+seg+";") {
						t.Errorf("%s: member %q is not declared in\n%s", item.AccessPath, seg, out)
					}
				}
			}
		})
	}
}

const pushConstantsWGSL = `
struct ImmediateDataVert {
    position_clip: f32,
    matrix: mat3x3<f32>,
}
var<immediate> im_vert: ImmediateDataVert;

struct ImmediateDataFrag {
    multiplier: f32,
    tint: vec4f,
}
var<immediate> im_frag: ImmediateDataFrag;

struct FragmentIn {
    @location(0) color: vec4<f32>
}

@vertex
fn vert_main(
  @location(0) pos : vec2<f32>,
  @builtin(instance_index) ii: u32,
  @builtin(vertex_index) vi: u32,
) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(ii) * f32(vi) * pos, 0.0, im_vert.position_clip);
}

@fragment
fn main(in: FragmentIn) -> @location(0) vec4<f32> {
    return in.color * im_frag.tint;
}
`

// TestMultipleImmediatesRefused: an entry point may statically use at most
// one immediate variable (WGSL immediate-data proposal), and the GLSL
// writer has one uniform name per stage for it, so it refuses a second one
// as Rust naga does (Error::MultipleImmediateData).
func TestMultipleImmediatesRefused(t *testing.T) {
	const src = `
var<immediate> a: f32;
var<immediate> b: f32;
var<workgroup> w: f32;
@compute @workgroup_size(1) fn main() { w = a + b; }`
	_, _, err := glsl.Compile(lowerWGSL(t, src), glsl.Options{LangVersion: glsl.VersionES320, EntryPoint: "main"})
	if err == nil || !strings.Contains(err.Error(), "multiple immediate") {
		t.Errorf("err = %v, want the multiple immediate data error", err)
	}
}

// TestImmediateArrayRefused: WGSL excludes arrays, and structs with array
// members, from immediate data, and Rust naga's validator rejects them
// (ImmediateError::InvalidArray), so the writer refuses them instead of
// leaving their elements out of ImmediateItems.
func TestImmediateArrayRefused(t *testing.T) {
	for name, src := range map[string]string{
		"array":  "var<immediate> im: array<vec4<f32>, 2>;\nvar<workgroup> w: f32;\n@compute @workgroup_size(1) fn main() { w = im[1].x; }",
		"member": "struct S { a: f32, b: array<vec4<f32>, 2> }\nvar<immediate> im: S;\nvar<workgroup> w: f32;\n@compute @workgroup_size(1) fn main() { w = im.a; }",
	} {
		_, _, err := glsl.Compile(lowerWGSL(t, src), glsl.Options{LangVersion: glsl.VersionES320, EntryPoint: "main"})
		if err == nil || !strings.Contains(err.Error(), "not allowed in immediate data") {
			t.Errorf("%s: err = %v, want the immediate data type error", name, err)
		}
	}
}
