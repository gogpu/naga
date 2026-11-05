// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"
	"strings"
	"testing"
)

// TestCompileRefusesOverlappingRegisterRanges (P1-DXIL-094): two resources
// of one class whose register ranges overlap in a space are refused, as DXC
// refuses them ("resource plain_storage at register 1 overlaps with resource
// storage_array at register 0, space 0", DxilCondenseResources.cpp:128-170
// AllocateRegisters, checked with dxc on the equivalent HLSL). We wrote the
// blob and IDxcValidator rejected it (binding-buffer-arrays/main without a
// binding map).
func TestCompileRefusesOverlappingRegisterRanges(t *testing.T) {
	src := `
enable wgpu_binding_array;
struct Data { values: array<u32> }
@group(0) @binding(0) var<storage, read> arr: binding_array<Data, 10>;
@group(0) @binding(1) var<storage, read> plain: Data;
@group(0) @binding(2) var<storage, read_write> out: Data;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
    out.values[0] = arr[i].values[0] + plain.values[0];
}`
	m := lowerSource(t, src)
	_, _, err := Compile(m, DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "overlaps with resource arr at register 0, space 0") {
		t.Fatalf("Compile: err = %v, want the overlap of plain and arr refused", err)
	}

	// With a binding map that separates them it compiles and validates.
	opts := DefaultOptions()
	opts.BindingMap = BindingMap{
		{Group: 0, Binding: 0}: {Register: 0, BindingArraySize: ptrU32(10)},
		{Group: 0, Binding: 1}: {Register: 10},
		{Group: 0, Binding: 2}: {Register: 0},
	}
	blob, _, err := Compile(m, opts)
	if err != nil {
		t.Fatalf("Compile with a binding map: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Errorf("Validate: %v", err)
	}

	// Different classes do not overlap: an SRV and a UAV at register 0.
	compileValidateEach(t, `
@group(0) @binding(0) var<storage, read> a: array<u32>;
@group(1) @binding(0) var<storage, read_write> b: array<u32>;
@compute @workgroup_size(1)
fn main() { b[0] = a[0]; }`)
}

// TestCompileRefusesSignatureRowsPast32 (P1-DXIL-094): a signature has 32
// rows; an element packed at row 32 or later is out of range, which the
// validator reports as DXC's does for the same HLSL ("signature element LOC
// at location (32,0) size (1,1) is out of range": DxilSignatureAllocator
// with 32 rows, kConflictFit, DxilValidation.cpp:4790-4805). A vertex input
// has one row per element, so 33 inputs do not fit; locations themselves
// may go past 31.
func TestCompileRefusesSignatureRowsPast32(t *testing.T) {
	inputs := func(n int, stride int) string {
		var b strings.Builder
		b.WriteString("struct In {\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "    @location(%d) v%d: f32,\n", i*stride, i)
		}
		b.WriteString("}\n@vertex fn main(i: In) -> @builtin(position) vec4<f32> {\n    return vec4(i.v0")
		for i := 1; i < n; i++ {
			fmt.Fprintf(&b, " + i.v%d", i)
		}
		b.WriteString(", 0.0, 0.0, 1.0);\n}\n")
		return b.String()
	}
	m := lowerSource(t, inputs(33, 1))
	_, _, err := Compile(m, DefaultOptions())
	if err == nil || !strings.Contains(err.Error(), "signature element LOC32 at location (32,0) size (1,1) is out of range") {
		t.Fatalf("33 vertex inputs: err = %v, want row 32 refused", err)
	}
	// 32 inputs fit; two inputs at locations 0 and 40 take rows 0 and 1.
	compileValidateEach(t, inputs(32, 1))
	compileValidateEach(t, inputs(2, 40))
}

func ptrU32(v uint32) *uint32 { return &v }
