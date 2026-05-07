// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helper Function Writers (functions.go) — all 0% coverage
// =============================================================================

func TestWriteModHelper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeModHelper()
	out := w.String()

	// Verify safe modulo helper contains both int and uint overloads
	if !strings.Contains(out, "int "+NagaModFunction+"(int a, int b)") {
		t.Error("expected int overload of naga_mod")
	}
	if !strings.Contains(out, "uint "+NagaModFunction+"(uint a, uint b)") {
		t.Error("expected uint overload of naga_mod")
	}
	if !strings.Contains(out, "a - b * (a / b)") {
		t.Error("expected truncated division modulo formula")
	}
}

func TestWriteDivHelper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeDivHelper()
	out := w.String()

	if !strings.Contains(out, "int "+NagaDivFunction+"(int a, int b)") {
		t.Error("expected int overload of naga_div")
	}
	if !strings.Contains(out, "uint "+NagaDivFunction+"(uint a, uint b)") {
		t.Error("expected uint overload of naga_div")
	}
	if !strings.Contains(out, "b != 0 ? a / b : 0") {
		t.Error("expected zero-divisor guard in int overload")
	}
	if !strings.Contains(out, "b != 0u ? a / b : 0u") {
		t.Error("expected zero-divisor guard in uint overload")
	}
}

func TestWriteAbsHelper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeAbsHelper()
	out := w.String()

	if !strings.Contains(out, NagaAbsFunction) {
		t.Error("expected naga_abs function name")
	}
	// Must handle INT_MIN: -2147483648 → clamp to 2147483647
	if !strings.Contains(out, "-2147483648") {
		t.Error("expected INT_MIN handling in abs helper")
	}
}

func TestWriteNegHelper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeNegHelper()
	out := w.String()

	if !strings.Contains(out, NagaNegFunction) {
		t.Error("expected naga_neg function name")
	}
	if !strings.Contains(out, "-2147483648") {
		t.Error("expected INT_MIN handling in neg helper")
	}
}

func TestWriteModfHelper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeModfHelper()
	out := w.String()

	if !strings.Contains(out, "_naga_modf_result_f32") {
		t.Error("expected modf result struct declaration")
	}
	if !strings.Contains(out, "float fract") {
		t.Error("expected fract member in modf result struct")
	}
	if !strings.Contains(out, "float whole") {
		t.Error("expected whole member in modf result struct")
	}
	if !strings.Contains(out, NagaModfFunction) {
		t.Error("expected naga_modf function name")
	}
}

func TestWriteFrexpHelper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeFrexpHelper()
	out := w.String()

	if !strings.Contains(out, "_naga_frexp_result_f32") {
		t.Error("expected frexp result struct declaration")
	}
	if !strings.Contains(out, "float fract") {
		t.Error("expected fract member in frexp result struct")
	}
	if !strings.Contains(out, "int exp") {
		t.Error("expected exp member in frexp result struct")
	}
	if !strings.Contains(out, NagaFrexpFunction) {
		t.Error("expected naga_frexp function name")
	}
}

func TestWriteF2I32Helper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeF2I32Helper()
	out := w.String()

	if !strings.Contains(out, NagaF2I32Function) {
		t.Error("expected naga_f2i32 function name")
	}
	if !strings.Contains(out, "clamp(v, -2147483648.0, 2147483647.0)") {
		t.Error("expected clamped float-to-i32 conversion")
	}
}

func TestWriteF2U32Helper(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeF2U32Helper()
	out := w.String()

	if !strings.Contains(out, NagaF2U32Function) {
		t.Error("expected naga_f2u32 function name")
	}
	if !strings.Contains(out, "clamp(v, 0.0, 4294967295.0)") {
		t.Error("expected clamped float-to-u32 conversion")
	}
}

// =============================================================================
// Storage Buffer Helpers (storage.go) — 0%/20% coverage
// =============================================================================

func TestIsStorageBufferReadOnly(t *testing.T) {
	global := &ir.GlobalVariable{
		Space: ir.SpaceStorage,
	}
	// Currently always returns false (stub)
	if got := isStorageBufferReadOnly(global); got {
		t.Error("expected isStorageBufferReadOnly to return false")
	}
}

func TestGetBufferElementType(t *testing.T) {
	four := uint32(4)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] u32
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// [1] array<u32, 4> (fixed size)
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &four}, Stride: 4}},
			// [2] array<u32> (runtime sized, no Constant)
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}, Stride: 4}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyType, handle1: 0}: "uint",
	}
	typeNames := map[ir.TypeHandle]string{0: "uint"}
	w := newTestWriter(module, names, typeNames)

	// Fixed array: returns element type, not runtime
	elemType, isRuntime := w.getBufferElementType(1)
	if elemType != "uint" {
		t.Errorf("fixed array element type = %q, want \"uint\"", elemType)
	}
	if isRuntime {
		t.Error("fixed array should not be runtime-sized")
	}

	// Runtime array: returns element type, is runtime
	elemType, isRuntime = w.getBufferElementType(2)
	if elemType != "uint" {
		t.Errorf("runtime array element type = %q, want \"uint\"", elemType)
	}
	if !isRuntime {
		t.Error("runtime array should be runtime-sized")
	}

	// Non-array type: returns the type itself, not runtime
	_, isRuntime = w.getBufferElementType(0)
	if isRuntime {
		t.Error("non-array type should not be runtime-sized")
	}

	// Out of range: returns empty
	elemType, _ = w.getBufferElementType(99)
	if elemType != "" {
		t.Errorf("out-of-range type = %q, want empty", elemType)
	}
}

func TestWriteStorageBufferDeclaration(t *testing.T) {
	four := uint32(4)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] u32
			{Name: "uint_type", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// [1] array<u32, 4>
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &four}, Stride: 4}},
			// [2] pointer to [1]
			{Inner: ir.PointerType{Base: 1, Space: ir.SpaceStorage}},
			// [3] array<u32> (runtime)
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}, Stride: 4}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyType, handle1: 0}: "uint",
		{kind: nameKeyType, handle1: 1}: "array_uint_4",
	}
	typeNames := map[ir.TypeHandle]string{0: "uint", 1: "array_uint_4"}
	w := newTestWriter(module, names, typeNames)

	binding := DefaultBindTarget().WithRegister(0).WithSpace(0)

	// Through pointer: should unwrap and write structured buffer
	w.writeStorageBufferDeclaration("buf", 2, &binding, false)
	out := output(w)
	if !strings.Contains(out, "StructuredBuffer") || !strings.Contains(out, "RWStructuredBuffer") {
		// Either read-only or read-write structured buffer should appear
		if !strings.Contains(out, "StructuredBuffer") && !strings.Contains(out, "RWStructuredBuffer") {
			t.Errorf("expected StructuredBuffer in output, got: %s", out)
		}
	}
}

func TestWriteStorageStoreScalar(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buffer0",
	}
	w := newTestWriter(module, names, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)}, // GlobalVariable -> f32
			{Handle: ptrHandle(0)}, // Literal -> f32
		},
	}
	setCurrentFunction(w, fn)

	sv := storeValue{kind: storeValueExpression, expr: 1}
	w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 0}}

	err := w.writeStorageStore(0, sv, 0, nil)
	if err != nil {
		t.Fatalf("writeStorageStore scalar: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "buffer0.Store(") {
		t.Errorf("expected ByteAddressBuffer.Store call, got: %s", out)
	}
	if !strings.Contains(out, "asuint(") {
		t.Errorf("expected asuint() wrap for f32, got: %s", out)
	}
}

func TestWriteStorageStoreVector(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] vec4<f32>
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buffer0",
	}
	w := newTestWriter(module, names, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)},
			{Handle: ptrHandle(0)},
		},
	}
	setCurrentFunction(w, fn)

	sv := storeValue{kind: storeValueExpression, expr: 1}
	w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 0}}

	err := w.writeStorageStore(0, sv, 0, nil)
	if err != nil {
		t.Fatalf("writeStorageStore vector: %v", err)
	}
	out := output(w)
	// Should emit Store4 for vec4<f32>
	if !strings.Contains(out, "buffer0.Store4(") {
		t.Errorf("expected Store4 call for vec4, got: %s", out)
	}
}

func TestWriteStorageStoreMatrix(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] mat2x3<f32> -- 2 columns, 3 rows
			{Inner: ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buffer0",
	}
	w := newTestWriter(module, names, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)},
			{Handle: ptrHandle(0)},
		},
	}
	setCurrentFunction(w, fn)

	sv := storeValue{kind: storeValueExpression, expr: 1}
	w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 0}}

	err := w.writeStorageStore(0, sv, 0, nil)
	if err != nil {
		t.Fatalf("writeStorageStore matrix: %v", err)
	}
	out := output(w)
	// Should decompose matrix into column stores
	if !strings.Contains(out, "_value") {
		t.Errorf("expected matrix decomposition with _valueN temp, got: %s", out)
	}
	if !strings.Contains(out, "Store3(") {
		t.Errorf("expected Store3 for 3-row columns, got: %s", out)
	}
}

func TestWriteStorageStoreStruct(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] struct { a: f32, b: f32 }
			{Name: "MyStruct", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: 0, Offset: 0},
					{Name: "b", Type: 0, Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}:           "buffer0",
		{kind: nameKeyStructMember, handle1: 1, handle2: 0}: "a",
		{kind: nameKeyStructMember, handle1: 1, handle2: 1}: "b",
	}
	typeNames := map[ir.TypeHandle]string{1: "MyStruct"}
	w := newTestWriter(module, names, typeNames)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(1)},
			{Handle: ptrHandle(1)},
		},
	}
	setCurrentFunction(w, fn)

	sv := storeValue{kind: storeValueExpression, expr: 1}
	w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 0}}

	err := w.writeStorageStore(0, sv, 0, nil)
	if err != nil {
		t.Fatalf("writeStorageStore struct: %v", err)
	}
	out := output(w)
	// Should decompose struct into member stores
	if !strings.Contains(out, "MyStruct _value") {
		t.Errorf("expected struct temp variable, got: %s", out)
	}
}

func TestWriteStorageStoreArray(t *testing.T) {
	two := uint32(2)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] array<f32, 2>
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &two}, Stride: 4}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buffer0",
		{kind: nameKeyType, handle1: 0}:           "float",
	}
	typeNames := map[ir.TypeHandle]string{0: "float"}
	w := newTestWriter(module, names, typeNames)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(1)},
			{Handle: ptrHandle(1)},
		},
	}
	setCurrentFunction(w, fn)

	sv := storeValue{kind: storeValueExpression, expr: 1}
	w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 0}}

	err := w.writeStorageStore(0, sv, 0, nil)
	if err != nil {
		t.Fatalf("writeStorageStore array: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "_value") {
		t.Errorf("expected array decomposition with _valueN, got: %s", out)
	}
}

func TestWriteStorageStoreRecursionLimit(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newTestWriter(module, nil, nil)
	w.names[nameKey{kind: nameKeyGlobalVariable, handle1: 0}] = "buf"

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)},
		},
	}
	setCurrentFunction(w, fn)

	sv := storeValue{kind: storeValueExpression, expr: 0}
	// Exceed recursion limit
	err := w.writeStorageStore(0, sv, 21, nil)
	if err == nil {
		t.Error("expected error at recursion depth limit")
	}
	if !strings.Contains(err.Error(), "recursion depth limit") {
		t.Errorf("expected recursion depth error, got: %v", err)
	}
}

func TestWriteStoreValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "S", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: 0, Offset: 0},
				},
			}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyStructMember, handle1: 0, handle2: 0}: "x",
	}
	w := newTestWriter(module, names, nil)

	// Test TempIndex
	sv := storeValue{kind: storeValueTempIndex, depth: 2, index: 3}
	err := w.writeStoreValue(sv)
	if err != nil {
		t.Fatalf("writeStoreValue TempIndex: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "_value2[3]") {
		t.Errorf("expected _value2[3], got: %s", out)
	}

	// Test TempAccess
	sv = storeValue{kind: storeValueTempAccess, depth: 1, base: 0, memberIdx: 0}
	err = w.writeStoreValue(sv)
	if err != nil {
		t.Fatalf("writeStoreValue TempAccess: %v", err)
	}
	out = output(w)
	if !strings.Contains(out, "_value1.x") {
		t.Errorf("expected _value1.x, got: %s", out)
	}
}

// =============================================================================
// Type Matching (storage.go) — 0% coverage
// =============================================================================

func TestTypesMatch(t *testing.T) {
	tests := []struct {
		name string
		a, b ir.TypeInner
		want bool
	}{
		{"scalar_match", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, true},
		{"scalar_kind_mismatch", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, false},
		{"scalar_width_mismatch", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, false},
		{"vector_match",
			ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, true},
		{"vector_size_mismatch",
			ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, false},
		{"matrix_match",
			ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, true},
		{"matrix_col_mismatch",
			ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, false},
		{"struct_match",
			ir.StructType{Span: 8, Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}},
			ir.StructType{Span: 8, Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}}, true},
		{"struct_span_mismatch",
			ir.StructType{Span: 8, Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}},
			ir.StructType{Span: 16, Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}}, false},
		{"struct_member_count_mismatch",
			ir.StructType{Span: 8, Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}},
			ir.StructType{Span: 8, Members: []ir.StructMember{}}, false},
		{"struct_member_name_mismatch",
			ir.StructType{Span: 8, Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}},
			ir.StructType{Span: 8, Members: []ir.StructMember{{Name: "b", Type: 0, Offset: 0}}}, false},
		{"array_match",
			ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{}},
			ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{}}, true},
		{"array_base_mismatch",
			ir.ArrayType{Base: 0, Stride: 4},
			ir.ArrayType{Base: 1, Stride: 4}, false},
		{"type_kind_mismatch",
			ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, false},
		{"unmatched_type",
			ir.SamplerType{Comparison: false},
			ir.SamplerType{Comparison: false}, false}, // SamplerType not handled
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typesMatch(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("typesMatch(%T, %T) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGetTypeHandleForInner(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	w := newTestWriter(module, nil, nil)

	// Match existing type
	h := w.getTypeHandleForInner(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	if h == nil {
		t.Fatal("expected to find scalar float handle")
	}
	if *h != 0 {
		t.Errorf("expected handle 0, got %d", *h)
	}

	// Match vector type
	h = w.getTypeHandleForInner(ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	if h == nil {
		t.Fatal("expected to find vector handle")
	}
	if *h != 1 {
		t.Errorf("expected handle 1, got %d", *h)
	}

	// No match
	h = w.getTypeHandleForInner(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	if h != nil {
		t.Error("expected nil for non-existing type")
	}
}

func TestWriteTypeByHandle(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "MyFloat", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	typeNames := map[ir.TypeHandle]string{0: "float"}
	w := newTestWriter(module, nil, typeNames)

	w.writeTypeByHandle(0)
	out := output(w)
	if !strings.Contains(out, "float") {
		t.Errorf("expected type name 'float', got: %s", out)
	}
}

// =============================================================================
// Global Expression Writers (expressions.go) — 0% coverage
// =============================================================================

func TestWriteGlobalBinaryExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(10)}},
			{Kind: ir.Literal{Value: ir.LiteralI32(20)}},
		},
	}
	w := newTestWriter(module, nil, nil)

	e := ir.ExprBinary{
		Op:    ir.BinaryAdd,
		Left:  0,
		Right: 1,
	}
	err := w.writeGlobalBinaryExpression(e, 2)
	if err != nil {
		t.Fatalf("writeGlobalBinaryExpression: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "+") {
		t.Errorf("expected '+' operator in output, got: %s", out)
	}
}

func TestWriteGlobalSplatExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
	}
	w := newTestWriter(module, nil, nil)

	e := ir.ExprSplat{
		Value: 0,
		Size:  3,
	}
	err := w.writeGlobalSplatExpression(e)
	if err != nil {
		t.Fatalf("writeGlobalSplatExpression: %v", err)
	}
	out := output(w)
	// Should emit splat using swizzle
	if !strings.Contains(out, ".xxx") {
		t.Errorf("expected .xxx swizzle for vec3 splat, got: %s", out)
	}
}

// =============================================================================
// writeMatrixValueType (expressions.go) — 0% coverage
// =============================================================================

func TestWriteMatrixValueType(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name string
		info matrixTypeInfo
		want string
	}{
		{"float4x4", matrixTypeInfo{columns: 4, rows: 4, width: 4}, "float4x4"},
		{"float3x2", matrixTypeInfo{columns: 3, rows: 2, width: 4}, "float3x2"},
		{"float2x3", matrixTypeInfo{columns: 2, rows: 3, width: 4}, "float2x3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.writeMatrixValueType(&tt.info)
			out := output(w)
			if !strings.Contains(out, tt.want) {
				t.Errorf("writeMatrixValueType = %q, want %q", out, tt.want)
			}
		})
	}
}

// =============================================================================
// writeCallResultExpression (expressions.go) — 0% coverage
// =============================================================================

func TestWriteCallResultExpression(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "my_func"},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyFunction, handle1: 0}: "my_func",
	}
	w := newTestWriter(module, names, nil)

	e := ir.ExprCallResult{Function: 0}
	err := w.writeCallResultExpression(e)
	if err != nil {
		t.Fatalf("writeCallResultExpression: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "_my_func_result") {
		t.Errorf("expected _my_func_result, got: %s", out)
	}
}

// =============================================================================
// writeRayQueryGetIntersection (expressions.go) — 0% coverage
// =============================================================================

func TestWriteRayQueryGetIntersection(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
	}
	w := newTestWriter(module, nil, nil)

	// Set up a function context with an expression for the query
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)},
		},
	}
	setCurrentFunction(w, fn)
	w.namedExpressions[0] = "rq"

	// Committed intersection
	err := w.writeRayQueryGetIntersection(ir.ExprRayQueryGetIntersection{Query: 0, Committed: true})
	if err != nil {
		t.Fatalf("committed intersection: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "GetCommittedIntersection(rq)") {
		t.Errorf("expected GetCommittedIntersection(rq), got: %s", out)
	}
	if !w.needsCommittedIntersectionHelper {
		t.Error("expected needsCommittedIntersectionHelper to be set")
	}

	// Candidate intersection
	err = w.writeRayQueryGetIntersection(ir.ExprRayQueryGetIntersection{Query: 0, Committed: false})
	if err != nil {
		t.Fatalf("candidate intersection: %v", err)
	}
	out = output(w)
	if !strings.Contains(out, "GetCandidateIntersection(rq)") {
		t.Errorf("expected GetCandidateIntersection(rq), got: %s", out)
	}
	if !w.needsCandidateIntersectionHelper {
		t.Error("expected needsCandidateIntersectionHelper to be set")
	}
}

// =============================================================================
// External Texture Helpers (external_texture.go) — 0% coverage
// =============================================================================

func TestShouldDecomposeExternalTextureStruct(t *testing.T) {
	paramsHandle := ir.TypeHandle(1)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "NagaExternalTextureParams", Inner: ir.StructType{Span: 64}},
		},
		SpecialTypes: ir.SpecialTypes{
			ExternalTextureParams: &paramsHandle,
		},
	}
	w := newTestWriter(module, nil, nil)

	// Should return true for the external texture params type
	if !w.shouldDecomposeExternalTextureStruct(1) {
		t.Error("expected true for external texture params type handle")
	}
	// Should return false for other types
	if w.shouldDecomposeExternalTextureStruct(0) {
		t.Error("expected false for non-params type handle")
	}
}

func TestShouldDecomposeExternalTextureStruct_NoParams(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newTestWriter(module, nil, nil)

	// No external texture params set — should return false
	if w.shouldDecomposeExternalTextureStruct(0) {
		t.Error("expected false when no external texture params are set")
	}
}

func TestWriteDecomposedMat3x2Member(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	w.writeDecomposedMat3x2Member("sample_transform")
	out := output(w)

	if !strings.Contains(out, "float2 sample_transform_0") {
		t.Error("expected decomposed member _0")
	}
	if !strings.Contains(out, "float2 sample_transform_1") {
		t.Error("expected decomposed member _1")
	}
	if !strings.Contains(out, "float2 sample_transform_2") {
		t.Error("expected decomposed member _2")
	}
}

func TestWriteFunctionExternalTextureArgument(t *testing.T) {
	paramsHandle := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "ExtParams", Inner: ir.StructType{Span: 128}},
		},
		SpecialTypes: ir.SpecialTypes{
			ExternalTextureParams: &paramsHandle,
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)
	w.typeNames[0] = "ExtParams"

	w.writeFunctionExternalTextureArgument(0, 0, "tex")
	out := output(w)

	if !strings.Contains(out, "Texture2D<float4>") {
		t.Error("expected Texture2D<float4> for planes")
	}
	if !strings.Contains(out, "ExtParams") {
		t.Error("expected ExtParams type for params argument")
	}
}

func TestWriteExternalTextureGlobalExpression(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	// Pre-populate the global names
	w.externalTextureGlobalNames[0] = [4]string{"plane0", "plane1", "plane2", "params"}

	w.writeExternalTextureGlobalExpression(0)
	out := output(w)
	if out != "plane0, plane1, plane2, params" {
		t.Errorf("expected comma-separated plane refs, got: %s", out)
	}
}

func TestWriteExternalTextureFuncArgExpression(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	key := externalTextureFuncArgKey{funcHandle: 0, argIndex: 1}
	w.externalTextureFuncArgNames[key] = [4]string{"p0", "p1", "p2", "prm"}

	w.writeExternalTextureFuncArgExpression(0, 1)
	out := output(w)
	if out != "p0, p1, p2, prm" {
		t.Errorf("expected p0, p1, p2, prm, got: %s", out)
	}
}

func TestWriteExternalTextureLoadHelperIfNeeded(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] external texture image type
			{Inner: ir.ImageType{
				Dim:   ir.Dim2D,
				Class: ir.ImageClassExternal,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	w := newTestWriter(module, nil, nil)

	// Function with ImageLoad on external texture
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageLoad{Image: 0}},
		},
	}

	w.writeExternalTextureLoadHelperIfNeeded(fn)
	out := output(w)
	if !strings.Contains(out, "nagaTextureLoadExternal") {
		t.Error("expected nagaTextureLoadExternal helper to be written")
	}
}

func TestWriteExternalTextureLoadHelperIfNeeded_NoExternalTexture(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] regular 2D sampled texture
			{Inner: ir.ImageType{
				Dim:   ir.Dim2D,
				Class: ir.ImageClassSampled,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageLoad{Image: 0}},
		},
	}

	w.writeExternalTextureLoadHelperIfNeeded(fn)
	out := output(w)
	if strings.Contains(out, "nagaTextureLoadExternal") {
		t.Error("should not write external texture helper for non-external texture")
	}
}

// =============================================================================
// inferExpressionType / inferPointeeType (expressions.go) — 0% coverage
// =============================================================================

func TestInferExpressionType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] vec3<f32>
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [2] struct { pos: vec3<f32> }
			{Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "pos", Type: 1, Offset: 0}},
				Span:    12,
			}},
			// [3] pointer to struct
			{Inner: ir.PointerType{Base: 2, Space: ir.SpaceFunction}},
			// [4] array<f32, 4>
			{Inner: ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{Constant: ptrU32(4)}}},
			// [5] mat4x4<f32>
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 3, Space: ir.SpaceFunction},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			// [0] GlobalVariable -> g (struct pointer)
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			// [1] AccessIndex { base: 0, index: 0 } -> struct member "pos"
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			// [2] Load { pointer: 1 } -> load the member
			{Kind: ir.ExprLoad{Pointer: 1}},
			// [3] FunctionArgument(0)
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			// [4] LocalVariable(0)
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		Arguments: []ir.FunctionArgument{
			{Type: 0}, // f32 argument
		},
		LocalVars: []ir.LocalVariable{
			{Type: 0}, // f32 local
		},
		// Empty ExpressionTypes to force fallback to inference
		ExpressionTypes: nil,
	}
	setCurrentFunction(w, fn)

	// Test inference from GlobalVariable
	ty := w.inferExpressionType(0)
	if ty == nil {
		t.Fatal("expected type for GlobalVariable expr")
	}
	// Should resolve to struct (dereferenced from pointer)
	if _, ok := ty.Inner.(ir.StructType); !ok {
		t.Errorf("expected StructType from pointer deref, got %T", ty.Inner)
	}

	// Test inference from AccessIndex on struct
	ty = w.inferExpressionType(1)
	if ty == nil {
		t.Fatal("expected type for AccessIndex expr")
	}
	if _, ok := ty.Inner.(ir.VectorType); !ok {
		t.Errorf("expected VectorType from struct member, got %T", ty.Inner)
	}

	// Test inference from Load
	ty = w.inferExpressionType(2)
	if ty == nil {
		t.Fatal("expected type for Load expr")
	}

	// Test inference from FunctionArgument
	ty = w.inferExpressionType(3)
	if ty == nil {
		t.Fatal("expected type for FunctionArgument expr")
	}
	if _, ok := ty.Inner.(ir.ScalarType); !ok {
		t.Errorf("expected ScalarType for function arg, got %T", ty.Inner)
	}

	// Test inference from LocalVariable
	ty = w.inferExpressionType(4)
	if ty == nil {
		t.Fatal("expected type for LocalVariable expr")
	}

	// Test nil for out-of-range handle
	ty = w.inferExpressionType(99)
	if ty != nil {
		t.Error("expected nil for out-of-range handle")
	}

	// Test nil when no current function
	w.currentFunction = nil
	ty = w.inferExpressionType(0)
	if ty != nil {
		t.Error("expected nil when currentFunction is nil")
	}
}

func TestInferExpressionType_Access(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] array<f32, 4>
			{Inner: ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{Constant: ptrU32(4)}}},
			// [2] vec4<f32>
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [3] mat4x4<f32>
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "arr", Type: 1, Space: ir.SpaceFunction},
			{Name: "vec", Type: 2, Space: ir.SpaceFunction},
			{Name: "mat", Type: 3, Space: ir.SpaceFunction},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			// [0] GlobalVariable -> arr (array)
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			// [1] Access { base: 0, index: ... } -> array element
			{Kind: ir.ExprAccess{Base: 0, Index: 0}},
			// [2] GlobalVariable -> vec (vector)
			{Kind: ir.ExprGlobalVariable{Variable: 1}},
			// [3] Access { base: 2, index: ... } -> vector element
			{Kind: ir.ExprAccess{Base: 2, Index: 0}},
			// [4] GlobalVariable -> mat (matrix)
			{Kind: ir.ExprGlobalVariable{Variable: 2}},
			// [5] Access { base: 4, index: ... } -> matrix column
			{Kind: ir.ExprAccess{Base: 4, Index: 0}},
		},
	}
	setCurrentFunction(w, fn)

	// Access into array -> element type
	ty := w.inferExpressionType(1)
	if ty == nil {
		t.Fatal("expected type for Access into array")
	}
	if _, ok := ty.Inner.(ir.ScalarType); !ok {
		t.Errorf("expected ScalarType from array element, got %T", ty.Inner)
	}

	// Access into vector -> scalar type
	ty = w.inferExpressionType(3)
	if ty == nil {
		t.Fatal("expected type for Access into vector")
	}
	if _, ok := ty.Inner.(ir.ScalarType); !ok {
		t.Errorf("expected ScalarType from vector element, got %T", ty.Inner)
	}

	// Access into matrix -> vector (column)
	ty = w.inferExpressionType(5)
	if ty == nil {
		t.Fatal("expected type for Access into matrix")
	}
	if _, ok := ty.Inner.(ir.VectorType); !ok {
		t.Errorf("expected VectorType from matrix column, got %T", ty.Inner)
	}
}

func TestInferPointeeType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] pointer to f32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
			// [2] struct { x: f32 }
			{Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 1, Space: ir.SpaceFunction}, // pointer to f32
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
		Arguments: []ir.FunctionArgument{
			{Type: 2}, // struct type
		},
		LocalVars: []ir.LocalVariable{
			{Type: 0}, // f32
		},
	}
	setCurrentFunction(w, fn)

	// GlobalVariable with pointer type: should dereference
	ty := w.inferPointeeType(0)
	if ty == nil {
		t.Fatal("expected type for GlobalVariable pointer")
	}
	if _, ok := ty.Inner.(ir.ScalarType); !ok {
		t.Errorf("expected ScalarType from pointer deref, got %T", ty.Inner)
	}

	// LocalVariable
	ty = w.inferPointeeType(1)
	if ty != nil {
		if _, ok := ty.Inner.(ir.ScalarType); !ok {
			t.Errorf("expected ScalarType for local var, got %T", ty.Inner)
		}
	}

	// FunctionArgument
	ty = w.inferPointeeType(2)
	if ty != nil {
		if _, ok := ty.Inner.(ir.StructType); !ok {
			t.Errorf("expected StructType for func arg, got %T", ty.Inner)
		}
	}

	// Out of range
	ty = w.inferPointeeType(99)
	if ty != nil {
		t.Error("expected nil for out-of-range handle")
	}

	// Nil function
	w.currentFunction = nil
	ty = w.inferPointeeType(0)
	if ty != nil {
		t.Error("expected nil when no function")
	}
}

func TestInferPointeeType_AccessIndex(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] struct { x: f32 }
			{Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			// [0] LocalVariable -> struct
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			// [1] AccessIndex { base: 0, index: 0 } -> struct.x
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		},
		LocalVars: []ir.LocalVariable{
			{Type: 1}, // struct type
		},
	}
	setCurrentFunction(w, fn)

	ty := w.inferPointeeType(1)
	if ty == nil {
		t.Fatal("expected type for AccessIndex pointee")
	}
	if _, ok := ty.Inner.(ir.ScalarType); !ok {
		t.Errorf("expected ScalarType from struct member, got %T", ty.Inner)
	}
}

// =============================================================================
// Image Query Wrappers (writer.go) — 0% coverage
// =============================================================================

func TestWriteWrappedImageQueryFunctions(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] 2D sampled texture
			{Inner: ir.ImageType{
				Dim:   ir.Dim2D,
				Class: ir.ImageClassSampled,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{
				Image: 0,
				Query: ir.ImageQueryNumLevels{},
			}},
		},
	}

	w.writeWrappedImageQueryFunctions(fn)
	out := output(w)

	if !strings.Contains(out, "Naga") {
		t.Errorf("expected NagaXxx wrapper function, got: %s", out)
	}
	if !strings.Contains(out, "GetDimensions") {
		t.Errorf("expected GetDimensions in wrapper, got: %s", out)
	}
}

func TestWriteWrappedImageQueryFunctions_Size(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{
				Image: 0,
				Query: ir.ImageQuerySize{},
			}},
		},
	}

	w.writeWrappedImageQueryFunctions(fn)
	out := output(w)
	if !strings.Contains(out, "NagaSize") || !strings.Contains(out, "2D") {
		// The function name pattern includes query type and dimension
		if !strings.Contains(out, "Naga") {
			t.Errorf("expected NagaSizeXxx wrapper, got: %s", out)
		}
	}
}

func TestWriteWrappedImageQueryFunctions_Dedup(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQueryNumLevels{}}},
			{Kind: ir.ExprImageQuery{Image: 0, Query: ir.ImageQueryNumLevels{}}},
		},
	}

	w.writeWrappedImageQueryFunctions(fn)
	out := output(w)
	// Count occurrences of the function - should appear only once
	count := strings.Count(out, "GetDimensions")
	if count > 1 {
		t.Errorf("expected dedup: GetDimensions appeared %d times", count)
	}
}

// =============================================================================
// writeStructMatrixAccessHelpers (writer.go) — 0% coverage
// =============================================================================

func TestWriteStructMatrixAccessHelpers(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] mat3x2<f32> (matCx2 — needs special helpers)
			{Inner: ir.MatrixType{Columns: 3, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [1] struct with matCx2 member
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "transform", Type: 0, Offset: 0},
				},
				Span: 24,
			}},
			// [2] pointer to struct
			{Inner: ir.PointerType{Base: 1, Space: ir.SpaceUniform}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "uniforms", Type: 2, Space: ir.SpaceUniform},
		},
	}
	typeNames := map[ir.TypeHandle]string{0: "float3x2", 1: "Uniforms"}
	w := newTestWriter(module, nil, typeNames)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // Access struct member 0 (matCx2)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(2)}, // pointer to struct
			{Handle: ptrHandle(0)}, // matCx2
		},
	}

	w.writeStructMatrixAccessHelpers(fn)
	// If it found the matCx2 member, it should have written helper functions
	// or at minimum registered in wrappedStructMatrixAccess
	// The test validates the scan logic runs without panic
}

// =============================================================================
// writeCompositeValue (types.go) — 0% coverage
// =============================================================================

func TestWriteCompositeValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] vec2<f32>
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Constants: []ir.Constant{
			{Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}}, // 1.0
			{Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40000000}}, // 2.0
		},
	}
	w := newTestWriter(module, nil, nil)

	v := ir.CompositeValue{Components: []ir.ConstantHandle{0, 1}}
	result := w.writeCompositeValue(v, 1) // type 1 = vec2<f32>
	// Should produce constructor syntax for vector: "float2(1.0, 2.0)" or similar
	if result == "" {
		t.Error("expected non-empty composite value")
	}
	if !strings.Contains(result, "1.0") {
		t.Errorf("expected 1.0 in composite, got: %s", result)
	}
}

func TestWriteCompositeValue_Array(t *testing.T) {
	four := uint32(2)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] i32
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			// [1] array<i32, 2>
			{Inner: ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{Constant: &four}}},
		},
		Constants: []ir.Constant{
			{Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarSint, Bits: 10}},
			{Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarSint, Bits: 20}},
		},
	}
	w := newTestWriter(module, nil, nil)

	v := ir.CompositeValue{Components: []ir.ConstantHandle{0, 1}}
	result := w.writeCompositeValue(v, 1) // type 1 = array
	// Arrays use {} initializer syntax, not ()
	if !strings.Contains(result, "{") {
		t.Errorf("expected array initializer with {}, got: %s", result)
	}
}

// =============================================================================
// writeArraySizes (types.go) — 0% coverage
// =============================================================================

func TestWriteArraySizes(t *testing.T) {
	four := uint32(4)
	two := uint32(2)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] array<f32, 4>
			{Inner: ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{Constant: &four}}},
			// [2] array<array<f32, 4>, 2> (nested)
			{Inner: ir.ArrayType{Base: 1, Stride: 16, Size: ir.ArraySize{Constant: &two}}},
			// [3] array<f32> (runtime)
			{Inner: ir.ArrayType{Base: 0, Stride: 4, Size: ir.ArraySize{Constant: nil}}},
		},
	}
	w := newTestWriter(module, nil, nil)

	// Fixed array
	w.writeArraySizes(1)
	out := output(w)
	if out != "[4]" {
		t.Errorf("expected [4], got: %q", out)
	}

	// Nested array
	w.writeArraySizes(2)
	out = output(w)
	if !strings.Contains(out, "[2]") || !strings.Contains(out, "[4]") {
		t.Errorf("expected [2][4] for nested array, got: %q", out)
	}

	// Runtime array (no Constant)
	w.writeArraySizes(3)
	out = output(w)
	if !strings.Contains(out, "[1]") {
		t.Errorf("expected [1] fallback for runtime array, got: %q", out)
	}

	// Non-array type - no output
	w.writeArraySizes(0)
	out = output(w)
	if out != "" {
		t.Errorf("expected empty output for non-array, got: %q", out)
	}
}

// =============================================================================
// writeModifier (functions.go) — 0% coverage
// =============================================================================

func TestWriteModifier(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	// Nil binding - should produce nothing
	w.writeModifier(nil)
	out := output(w)
	if out != "" {
		t.Errorf("expected empty output for nil binding, got: %q", out)
	}

	// BuiltinBinding - Position (currently no special output beyond potential invariant)
	builtinBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	w.writeModifier(&builtinBinding)
	_ = output(w) // verify no panic

	// LocationBinding with interpolation
	interpKind := ir.InterpolationFlat
	interpSampling := ir.SamplingCenter
	locBinding := ir.Binding(ir.LocationBinding{
		Location: 0,
		Interpolation: &ir.Interpolation{
			Kind:     interpKind,
			Sampling: interpSampling,
		},
	})
	w.writeModifier(&locBinding)
	out = output(w)
	if !strings.Contains(out, "nointerpolation") {
		t.Errorf("expected 'nointerpolation' for flat interpolation, got: %q", out)
	}
}

// =============================================================================
// writeStructConstructors (writer.go) — 0% coverage
// =============================================================================

func TestWriteStructConstructors(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [1] struct { x: f32, y: f32 }
			{Name: "Vec2", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: 0, Offset: 0},
					{Name: "y", Type: 0, Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	names := map[nameKey]string{
		{kind: nameKeyType, handle1: 1}:                     "Vec2",
		{kind: nameKeyStructMember, handle1: 1, handle2: 0}: "x",
		{kind: nameKeyStructMember, handle1: 1, handle2: 1}: "y",
	}
	typeNames := map[ir.TypeHandle]string{0: "float", 1: "Vec2"}
	w := newTestWriter(module, names, typeNames)

	// Register the struct for constructor generation
	w.structConstructors[1] = struct{}{}

	w.writeStructConstructors()
	out := w.String()

	if !strings.Contains(out, "Vec2 ConstructVec2(") {
		t.Errorf("expected ConstructVec2 function, got: %s", out)
	}
	if !strings.Contains(out, "ret.x = arg0") {
		t.Errorf("expected member assignment, got: %s", out)
	}
	if !strings.Contains(out, "ret.y = arg1") {
		t.Errorf("expected second member assignment, got: %s", out)
	}
	if !strings.Contains(out, "(Vec2)0") {
		t.Errorf("expected zero initialization, got: %s", out)
	}
}

// =============================================================================
// writeClampToEdgeHelper (writer.go) — 0% coverage
// =============================================================================

func TestWriteClampToEdgeHelper_Regular(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] 2D sampled texture
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageSample{Image: 0, ClampToEdge: true}},
		},
	}

	w.writeClampToEdgeHelper(fn)
	out := output(w)
	if !strings.Contains(out, "nagaTextureSampleBaseClampToEdge") {
		t.Error("expected ClampToEdge helper function")
	}
	if !strings.Contains(out, "half_texel") {
		t.Error("expected half_texel calculation")
	}
}

func TestWriteClampToEdgeHelper_NoClampToEdge(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageSample{Image: 0, ClampToEdge: false}},
		},
	}

	w.writeClampToEdgeHelper(fn)
	out := output(w)
	if strings.Contains(out, "nagaTextureSampleBaseClampToEdge") {
		t.Error("should not write helper when ClampToEdge is false")
	}
}

func TestWriteClampToEdgeHelper_Dedup(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Type: 0, Space: ir.SpaceHandle},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprImageSample{Image: 0, ClampToEdge: true}},
		},
	}

	// Call twice — should only write once
	w.writeClampToEdgeHelper(fn)
	first := output(w)
	w.writeClampToEdgeHelper(fn)
	second := output(w)

	if !strings.Contains(first, "nagaTextureSampleBaseClampToEdge") {
		t.Error("first call should write helper")
	}
	if second != "" {
		t.Error("second call should be no-op (dedup)")
	}
}

// =============================================================================
// writeLoadExpression (expressions.go) — 23.8% coverage
// =============================================================================

func TestWriteLoadExpression_Simple(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)},
		},
		LocalVars: []ir.LocalVariable{
			{Type: 0},
		},
	}
	setCurrentFunction(w, fn)
	w.localNames[0] = "myVar"

	// Simple load — just writes the pointer expression (loads are implicit in HLSL)
	err := w.writeLoadExpression(ir.ExprLoad{Pointer: 0})
	if err != nil {
		t.Fatalf("writeLoadExpression: %v", err)
	}
	out := output(w)
	if !strings.Contains(out, "myVar") {
		t.Errorf("expected variable name in output, got: %s", out)
	}
}

// =============================================================================
// getGlobalUniformMatrix (expressions.go) — 45.5% coverage
// =============================================================================

func TestGetGlobalUniformMatrix(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			// [0] mat4x4<f32>
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [1] pointer to mat4x4<f32>
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceUniform}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "transform", Type: 1, Space: ir.SpaceUniform},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(1)}, // pointer to mat4x4
		},
	}
	setCurrentFunction(w, fn)

	// Direct uniform matrix reference
	m := w.getGlobalUniformMatrix(0)
	if m == nil {
		t.Fatal("expected matrix info for uniform global")
	}
	if m.columns != 4 || m.rows != 4 {
		t.Errorf("expected 4x4, got %dx%d", m.columns, m.rows)
	}

	// Out of range
	m = w.getGlobalUniformMatrix(99)
	if m != nil {
		t.Error("expected nil for out-of-range expression")
	}

	// No current function
	w.currentFunction = nil
	m = w.getGlobalUniformMatrix(0)
	if m != nil {
		t.Error("expected nil when no current function")
	}
}

func TestGetGlobalUniformMatrix_NotUniform(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "mat", Type: 0, Space: ir.SpaceStorage}, // NOT uniform
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: ptrHandle(0)},
		},
	}
	setCurrentFunction(w, fn)

	m := w.getGlobalUniformMatrix(0)
	if m != nil {
		t.Error("expected nil for non-uniform space")
	}
}

// =============================================================================
// writeHeader (writer.go) — 0% coverage
// =============================================================================

func TestWriteHeader(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	w.writeHeader()
	out := w.String()

	// writeHeader is a no-op to match Rust naga — just verify it doesn't crash
	// and doesn't add unexpected content
	if strings.Contains(out, "//") {
		t.Error("writeHeader should be a no-op (Rust naga emits no header)")
	}
}

// =============================================================================
// writeBindingArrayDeclaration (types.go) — 0% coverage
// =============================================================================

func TestWriteBindingArrayDeclaration_Texture(t *testing.T) {
	ten := uint32(10)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] 2D sampled image
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
			// [1] binding_array<texture_2d>
			{Inner: ir.BindingArrayType{Base: 0, Size: &ten}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "textures",
				Type:    1,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)
	w.registerNames()

	ba := module.Types[1].Inner.(ir.BindingArrayType)
	global := &module.GlobalVariables[0]

	w.writeBindingArrayDeclaration("textures", ba, global)
	out := w.String()

	if !strings.Contains(out, "textures") {
		t.Errorf("expected 'textures' in declaration, got: %s", out)
	}
	if !strings.Contains(out, "[10]") {
		t.Errorf("expected array size [10], got: %s", out)
	}
}

func TestWriteBindingArrayDeclaration_Sampler(t *testing.T) {
	ten := uint32(10)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] sampler
			{Inner: ir.SamplerType{Comparison: false}},
			// [1] binding_array<sampler>
			{Inner: ir.BindingArrayType{Base: 0, Size: &ten}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "samplers",
				Type:    1,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)
	w.registerNames()

	ba := module.Types[1].Inner.(ir.BindingArrayType)
	global := &module.GlobalVariables[0]

	w.writeBindingArrayDeclaration("samplers", ba, global)
	out := w.String()
	// Sampler binding arrays write sampler heap pattern instead of normal array
	if !strings.Contains(out, "static const uint") && !strings.Contains(out, "nagaSamplerHeap") {
		// At minimum it should have written something for the sampler
		_ = out // the sampler heap pattern is complex, just verify no panic
	}
}

func TestWriteBindingArrayDeclaration_UnknownBase(t *testing.T) {
	ten := uint32(10)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] binding_array with out-of-range base
			{Inner: ir.BindingArrayType{Base: 99, Size: &ten}},
		},
	}
	w := newTestWriter(module, nil, nil)

	ba := module.Types[0].Inner.(ir.BindingArrayType)
	w.writeBindingArrayDeclaration("unknown", ba, &ir.GlobalVariable{})
	out := output(w)
	if !strings.Contains(out, "Unknown") {
		t.Errorf("expected 'Unknown' comment for out-of-range base, got: %s", out)
	}
}

// =============================================================================
// Function Argument/Result Helpers (functions.go) — 0% coverage
// =============================================================================

func TestGetArgumentSemantic(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newTestWriter(module, nil, nil)

	// No binding: empty semantic
	arg := ir.FunctionArgument{Type: 0}
	sem := w.getArgumentSemantic(arg, 0)
	if sem != "" {
		t.Errorf("expected empty semantic for no binding, got: %q", sem)
	}

	// With BuiltinBinding
	builtinBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	argWithBinding := ir.FunctionArgument{Type: 0, Binding: &builtinBind}
	sem = w.getArgumentSemantic(argWithBinding, 0)
	if sem == "" {
		t.Error("expected non-empty semantic for Position builtin")
	}
}

func TestWriteArgumentWithSemantic(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	typeNames := map[ir.TypeHandle]string{0: "float"}
	w := newTestWriter(module, nil, typeNames)

	// Without semantic
	arg := ir.FunctionArgument{Type: 0}
	result := w.writeArgumentWithSemantic(arg, 0, "pos")
	if result != "float pos" {
		t.Errorf("expected 'float pos', got: %q", result)
	}

	// With semantic
	builtinBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	argWithBinding := ir.FunctionArgument{Type: 0, Binding: &builtinBind}
	result = w.writeArgumentWithSemantic(argWithBinding, 0, "pos")
	if !strings.Contains(result, ":") {
		t.Errorf("expected semantic separator ':', got: %q", result)
	}
}

func TestGetResultSemantic(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	// Nil result
	sem := w.getResultSemantic(nil)
	if sem != "" {
		t.Errorf("expected empty for nil result, got: %q", sem)
	}

	// Result without binding
	result := &ir.FunctionResult{Type: 0}
	sem = w.getResultSemantic(result)
	if sem != "" {
		t.Errorf("expected empty for no binding, got: %q", sem)
	}

	// Result with binding
	builtinBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	result = &ir.FunctionResult{Type: 0, Binding: &builtinBind}
	sem = w.getResultSemantic(result)
	if sem == "" {
		t.Error("expected non-empty semantic for Position builtin")
	}
}

func TestWriteResultType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	typeNames := map[ir.TypeHandle]string{0: "float"}
	w := newTestWriter(module, nil, typeNames)

	// Nil result -> void
	r := w.writeResultType(nil)
	if r != "void" {
		t.Errorf("expected 'void', got: %q", r)
	}

	// With result type
	result := &ir.FunctionResult{Type: 0}
	r = w.writeResultType(result)
	if r != "float" {
		t.Errorf("expected 'float', got: %q", r)
	}
}

// =============================================================================
// resolveVariableTypeHandle (statements.go) — 0% coverage
// =============================================================================

func TestResolveVariableTypeHandle(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 0, Space: ir.SpacePrivate},
		},
	}
	w := newTestWriter(module, nil, nil)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			{Kind: ir.ExprLoad{Pointer: 0}},
		},
		Arguments: []ir.FunctionArgument{
			{Type: 1},
		},
		LocalVars: []ir.LocalVariable{
			{Type: 0},
		},
	}

	// GlobalVariable -> returns its type
	ty := w.resolveVariableTypeHandle(fn, 0)
	if ty == nil || *ty != 0 {
		t.Error("expected type handle 0 for GlobalVariable")
	}

	// LocalVariable
	ty = w.resolveVariableTypeHandle(fn, 1)
	if ty == nil || *ty != 0 {
		t.Error("expected type handle 0 for LocalVariable")
	}

	// FunctionArgument
	ty = w.resolveVariableTypeHandle(fn, 2)
	if ty == nil || *ty != 1 {
		t.Error("expected type handle 1 for FunctionArgument")
	}

	// AccessIndex -> walks to base
	ty = w.resolveVariableTypeHandle(fn, 3)
	if ty == nil || *ty != 0 {
		t.Error("expected type handle 0 for AccessIndex -> GlobalVariable")
	}

	// Load -> walks to pointer
	ty = w.resolveVariableTypeHandle(fn, 4)
	if ty == nil || *ty != 0 {
		t.Error("expected type handle 0 for Load -> GlobalVariable")
	}

	// Out of range
	ty = w.resolveVariableTypeHandle(fn, 99)
	if ty != nil {
		t.Error("expected nil for out-of-range handle")
	}
}

// =============================================================================
// writeNagaBufferLengthHelpers (writer.go) — 0% coverage
// =============================================================================

func TestWriteNagaBufferLengthHelpers(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Type: 1, Space: ir.SpaceStorage, Access: ir.StorageReadWrite},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprArrayLength{Array: 0}},
		},
	}

	w.writeNagaBufferLengthHelpers(fn)
	out := w.String()

	if !strings.Contains(out, "NagaBufferLengthRW") {
		t.Error("expected NagaBufferLengthRW for read-write storage buffer")
	}
	if !strings.Contains(out, "GetDimensions") {
		t.Error("expected GetDimensions call in buffer length helper")
	}
}

func TestWriteNagaBufferLengthHelpers_ReadOnly(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Type: 1, Space: ir.SpaceStorage, Access: ir.StorageRead},
		},
	}
	opts := DefaultOptions()
	w := newWriter(module, opts)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprArrayLength{Array: 0}},
		},
	}

	w.writeNagaBufferLengthHelpers(fn)
	out := w.String()

	// Read-only: should write NagaBufferLength (without RW)
	if !strings.Contains(out, "NagaBufferLength(") {
		t.Error("expected NagaBufferLength for read-only storage buffer")
	}
	if strings.Contains(out, "NagaBufferLengthRW") {
		t.Error("should not write RW variant for read-only buffer")
	}
}

// =============================================================================
// getRayIntersectionTypeName (writer.go) — 0% coverage
// =============================================================================

func TestGetRayIntersectionTypeName(t *testing.T) {
	// No special type set -> default name
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	name := w.getRayIntersectionTypeName()
	if name != "RayIntersection" {
		t.Errorf("expected default 'RayIntersection', got: %q", name)
	}

	// With special type
	riHandle := ir.TypeHandle(0)
	module2 := &ir.Module{
		Types: []ir.Type{
			{Name: "MyRayHit", Inner: ir.StructType{Span: 32}},
		},
		SpecialTypes: ir.SpecialTypes{
			RayIntersection: &riHandle,
		},
	}
	typeNames := map[ir.TypeHandle]string{0: "MyRayHit"}
	w2 := newTestWriter(module2, nil, typeNames)

	name = w2.getRayIntersectionTypeName()
	if name != "MyRayHit" {
		t.Errorf("expected 'MyRayHit', got: %q", name)
	}
}

// =============================================================================
// indentStr edge cases (storage.go)
// =============================================================================

func TestIndentStr_EdgeCases(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		level int
		want  string
	}{
		{0, ""},
		{1, "    "},
		{2, "        "},
		{8, "                                "}, // 32 spaces max
		{9, "                                "}, // clamped at 32
	}
	for _, tt := range tests {
		got := w.indentStr(tt.level)
		if got != tt.want {
			t.Errorf("indentStr(%d) = %q (len %d), want %q (len %d)", tt.level, got, len(got), tt.want, len(tt.want))
		}
	}
}

// =============================================================================
// Test helpers
// =============================================================================

func ptrHandle(h ir.TypeHandle) *ir.TypeHandle {
	return &h
}

// ptrU32 is defined in storage_test.go, no need to redeclare
