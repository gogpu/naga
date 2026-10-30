// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"reflect"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

// TestImmediateItemsPublic: glsl.Compile carries the writer's immediate
// items into TranslationInfo field by field. The items themselves are
// tested in internal/codegen (TestImmediateItems).
func TestImmediateItemsPublic(t *testing.T) {
	const src = `
struct S { @size(16) a: f32, m: mat3x3<f32> }
var<immediate> im: S;
@vertex fn main() -> @builtin(position) vec4<f32> { return vec4<f32>(im.a); }`
	_, info, err := glsl.Compile(lowerWGSL(t, src), glsl.Options{LangVersion: glsl.VersionES320, EntryPoint: "main"})
	if err != nil {
		t.Fatal(err)
	}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	want := []glsl.ImmediateItem{
		{AccessPath: "_immediates_binding_vs.a", Type: f32, Offset: 0, SizeBytes: 4},
		{AccessPath: "_immediates_binding_vs.m", Type: ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec3, Scalar: f32}, Offset: 16, SizeBytes: 48},
	}
	if !reflect.DeepEqual(info.ImmediateItems, want) {
		t.Errorf("ImmediateItems:\n got %+v\nwant %+v", info.ImmediateItems, want)
	}
}
