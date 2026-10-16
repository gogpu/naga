// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// indexOperand returns the Index operand of the first Access expression in
// the entry point "main", the expression a bounds check has to classify.
func indexOperand(t *testing.T, module *ir.Module) (*ir.Function, ir.ExpressionHandle) {
	t.Helper()
	fn := &module.EntryPoints[0].Function
	for _, e := range fn.Expressions {
		if a, ok := e.Kind.(ir.ExprAccess); ok {
			return fn, a.Index
		}
	}
	t.Fatal("no Access expression")
	return nil, 0
}

// TestConstIndexValue pins which index expressions are known at translation
// time, as Rust naga's GuardedIndex::from_expression / get_const_val_from
// decide it: a u32 or non-negative i32 literal, a scalar zero value, or a
// constant whose initializer is one of those.
func TestConstIndexValue(t *testing.T) {
	u32h, i32h, f32h, vech := ir.TypeHandle(0), ir.TypeHandle(1), ir.TypeHandle(2), ir.TypeHandle(3)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(7)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(-2)}},
			{Kind: ir.ExprZeroValue{Type: u32h}},
		},
		Constants: []ir.Constant{
			{Name: "seven", Type: u32h, Init: 0},
			{Name: "minus_two", Type: i32h, Init: 1},
			{Name: "zero", Type: u32h, Init: 2},
		},
	}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralU32(2)}},
		{Kind: ir.Literal{Value: ir.LiteralI32(3)}},
		{Kind: ir.Literal{Value: ir.LiteralI32(-1)}},
		{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
		{Kind: ir.ExprZeroValue{Type: u32h}},
		{Kind: ir.ExprZeroValue{Type: i32h}},
		{Kind: ir.ExprZeroValue{Type: f32h}},
		{Kind: ir.ExprZeroValue{Type: vech}},
		{Kind: ir.ExprConstant{Constant: 0}},
		{Kind: ir.ExprConstant{Constant: 1}},
		{Kind: ir.ExprConstant{Constant: 2}},
		{Kind: ir.ExprFunctionArgument{Index: 0}},
	}}
	tests := []struct {
		name   string
		handle ir.ExpressionHandle
		want   uint32
		known  bool
	}{
		{"u32_literal", 0, 2, true},
		{"i32_literal", 1, 3, true},
		{"negative_i32_literal", 2, 0, false},
		{"float_literal", 3, 0, false},
		{"u32_zero_value", 4, 0, true},
		{"i32_zero_value", 5, 0, true},
		{"float_zero_value", 6, 0, false},
		{"vector_zero_value", 7, 0, false},
		{"constant_u32", 8, 7, true},
		{"constant_negative_i32", 9, 0, false},
		{"constant_zero_value", 10, 0, true},
		{"function_argument", 11, 0, false},
		{"out_of_range_handle", 99, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := ir.ConstIndexValue(module, fn, tt.handle)
			if known != tt.known || got != tt.want {
				t.Errorf("ConstIndexValue = %d, %v; want %d, %v", got, known, tt.want, tt.known)
			}
		})
	}
}

// TestConstIndexValueFromWGSL checks the lowered forms of an Access index: a
// module constant stays an Access with a Constant index (literals become
// AccessIndex in the lowerer, as in Rust naga), a runtime value is unknown.
func TestConstIndexValueFromWGSL(t *testing.T) {
	tests := []struct {
		name  string
		index string
		decl  string
		want  uint32
		known bool
	}{
		{name: "u32_constant", decl: "const K = 1u;", index: "K", want: 1, known: true},
		{name: "i32_constant", decl: "const K: i32 = 2;", index: "K", want: 2, known: true},
		{name: "runtime_value", index: "n", known: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := tt.decl + `
@group(0) @binding(0) var<storage, read_write> out: array<u32, 4>;
@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) n: u32) {
    var a = array<u32, 4>(1u, 2u, 3u, 4u);
    out[0] = a[` + tt.index + `];
}`
			module := lowerWGSL(t, source)
			fn, index := indexOperand(t, module)
			got, known := ir.ConstIndexValue(module, fn, index)
			if known != tt.known || got != tt.want {
				t.Errorf("ConstIndexValue(%s) = %d, %v; want %d, %v", tt.index, got, known, tt.want, tt.known)
			}
		})
	}
}

// TestIndexableLength pins the length used to bounds-check an index into a
// type, as Rust naga's TypeInner::indexable_length: vectors by size,
// matrices by columns, arrays by their constant size; runtime-sized arrays
// and unbounded binding arrays are dynamic; pointers are seen through.
func TestIndexableLength(t *testing.T) {
	u32h := ir.TypeHandle(0)
	vec3h := ir.TypeHandle(1)
	four := uint32(4)
	module := &ir.Module{Types: []ir.Type{
		{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		{Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		{Inner: ir.ArrayType{Base: u32h, Size: ir.ArraySize{Constant: &four}}},
		{Inner: ir.ArrayType{Base: u32h}},
	}}
	tests := []struct {
		name    string
		inner   ir.TypeInner
		want    ir.IndexableLength
		wantErr bool
	}{
		{"vector", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, ir.IndexableLength{Known: 4}, false},
		{"matrix_columns", ir.MatrixType{Columns: ir.Vec3, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, ir.IndexableLength{Known: 3}, false},
		{"sized_array", ir.ArrayType{Base: u32h, Size: ir.ArraySize{Constant: &four}}, ir.IndexableLength{Known: 4}, false},
		{"runtime_array", ir.ArrayType{Base: u32h}, ir.IndexableLength{Dynamic: true}, false},
		{"sized_binding_array", ir.BindingArrayType{Base: u32h, Size: &four}, ir.IndexableLength{Known: 4}, false},
		{"unbounded_binding_array", ir.BindingArrayType{Base: u32h}, ir.IndexableLength{Dynamic: true}, false},
		{"pointer_to_vector", ir.PointerType{Base: vec3h, Space: ir.SpaceFunction}, ir.IndexableLength{Known: 3}, false},
		{"pointer_to_runtime_array", ir.PointerType{Base: 3, Space: ir.SpaceStorage}, ir.IndexableLength{Dynamic: true}, false},
		{"value_pointer_to_vector", ir.ValuePointerType{Size: ptrTo(ir.Vec2), Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Space: ir.SpaceFunction}, ir.IndexableLength{Known: 2}, false},
		{"scalar_is_not_indexable", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.IndexableLength{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ir.IndexableLengthOf(module, tt.inner)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("IndexableLengthOf = %+v, %v; want %+v, error %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func ptrTo[T any](v T) *T { return &v }
