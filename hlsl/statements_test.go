// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// TestAtomicFunSuffix
// =============================================================================

func TestAtomicFunSuffix(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.AtomicFunction
		want string
	}{
		{"add", ir.AtomicAdd{}, "Add"},
		{"subtract", ir.AtomicSubtract{}, "Add"},
		{"and", ir.AtomicAnd{}, "And"},
		{"or", ir.AtomicInclusiveOr{}, "Or"},
		{"xor", ir.AtomicExclusiveOr{}, "Xor"},
		{"min", ir.AtomicMin{}, "Min"},
		{"max", ir.AtomicMax{}, "Max"},
		{"exchange", ir.AtomicExchange{}, "Exchange"},
		{"compare_exchange", ir.AtomicExchange{Compare: ptrExprHandle(5)}, "CompareExchange"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := atomicFunSuffix(tt.fun)
			if got != tt.want {
				t.Errorf("atomicFunSuffix(%T) = %q, want %q", tt.fun, got, tt.want)
			}
		})
	}
}

func ptrExprHandle(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

// =============================================================================
// TestGetPointerAddressSpace
// =============================================================================

func TestGetPointerAddressSpace(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},      // 0: u32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceStorage}},   // 1: ptr<storage, u32>
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceWorkGroup}}, // 2: ptr<workgroup, u32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 0},
			{Name: "wg", Space: ir.SpaceWorkGroup, Type: 0},
		},
	}

	storPtrHandle := ir.TypeHandle(1)
	wgPtrHandle := ir.TypeHandle(2)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}}, // 0: storage global
			{Kind: ir.ExprGlobalVariable{Variable: 1}}, // 1: workgroup global
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &storPtrHandle},
			{Handle: &wgPtrHandle},
		},
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	if got := w.getPointerAddressSpace(0); got != ir.SpaceStorage {
		t.Errorf("expr 0 (storage ptr): got space %d, want %d (SpaceStorage)", got, ir.SpaceStorage)
	}
	if got := w.getPointerAddressSpace(1); got != ir.SpaceWorkGroup {
		t.Errorf("expr 1 (workgroup ptr): got space %d, want %d (SpaceWorkGroup)", got, ir.SpaceWorkGroup)
	}
}

// =============================================================================
// TestWriteAtomicStatementWorkgroup
// =============================================================================

func TestWriteAtomicStatementWorkgroup(t *testing.T) {
	// Test that workgroup atomic operations produce InterlockedXxx(pointer, ...) pattern
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},      // 0: u32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceWorkGroup}}, // 1: ptr<workgroup, u32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "wg_var", Space: ir.SpaceWorkGroup, Type: 0},
		},
	}

	wgPtrHandle := ir.TypeHandle(1)
	u32Handle := ir.TypeHandle(0)

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "wg_var",
	}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})

	resultHandle := ir.ExpressionHandle(2)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},  // 0: wg_var ptr
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}}, // 1: 1u
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // 2: result placeholder
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &wgPtrHandle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}
	setCurrentFunction(w, fn)

	t.Run("add_with_result", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		stmt := ir.StmtAtomic{
			Pointer: 0,
			Fun:     ir.AtomicAdd{},
			Value:   1,
			Result:  &resultHandle,
		}
		err := w.writeAtomicStatement(stmt)
		if err != nil {
			t.Fatalf("writeAtomicStatement: %v", err)
		}
		got := w.out.String()
		// Should produce: "uint _e2; InterlockedAdd(wg_var, 1u, _e2);\n"
		if !strings.Contains(got, "uint _e2;") {
			t.Errorf("missing result declaration: %q", got)
		}
		if !strings.Contains(got, "InterlockedAdd(wg_var") {
			t.Errorf("missing InterlockedAdd: %q", got)
		}
		if !strings.Contains(got, ", _e2)") {
			t.Errorf("missing result argument: %q", got)
		}
	})

	t.Run("subtract_negation", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		fn.NamedExpressions = make(map[ir.ExpressionHandle]string) // reset
		stmt := ir.StmtAtomic{
			Pointer: 0,
			Fun:     ir.AtomicSubtract{},
			Value:   1,
			Result:  &resultHandle,
		}
		err := w.writeAtomicStatement(stmt)
		if err != nil {
			t.Fatalf("writeAtomicStatement: %v", err)
		}
		got := w.out.String()
		// Subtract should use InterlockedAdd with negated value: "-1u"
		if !strings.Contains(got, "InterlockedAdd(") {
			t.Errorf("subtract should use InterlockedAdd: %q", got)
		}
		if !strings.Contains(got, "-1u") {
			t.Errorf("subtract should negate value: %q", got)
		}
	})
}

// =============================================================================
// TestWriteAtomicStatementStorage
// =============================================================================

func TestWriteAtomicStatementStorage(t *testing.T) {
	// Test that storage atomic operations produce buffer.InterlockedXxx(offset, ...) pattern
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},    // 0: u32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceStorage}}, // 1: ptr<storage, u32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "storage_buf", Space: ir.SpaceStorage, Type: 0},
		},
	}

	storPtrHandle := ir.TypeHandle(1)
	u32Handle := ir.TypeHandle(0)

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "storage_buf",
	}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})

	resultHandle := ir.ExpressionHandle(2)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},  // 0: storage_buf ptr
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}}, // 1: 1u
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // 2: result placeholder
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &storPtrHandle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}
	setCurrentFunction(w, fn)

	t.Run("storage_add", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		stmt := ir.StmtAtomic{
			Pointer: 0,
			Fun:     ir.AtomicAdd{},
			Value:   1,
			Result:  &resultHandle,
		}
		err := w.writeAtomicStatement(stmt)
		if err != nil {
			t.Fatalf("writeAtomicStatement: %v", err)
		}
		got := w.out.String()
		// Storage pattern: "uint _e2; storage_buf.InterlockedAdd(0, 1u, _e2);\n"
		if !strings.Contains(got, "storage_buf.InterlockedAdd(") {
			t.Errorf("storage atomic should use buffer.InterlockedAdd: %q", got)
		}
		if !strings.Contains(got, "_e2") {
			t.Errorf("missing result variable: %q", got)
		}
	})
}

// =============================================================================
// TestIsPointerExpressionAccessIndex
// =============================================================================

func TestIsPointerExpressionAccessIndex(t *testing.T) {
	// AccessIndex on a pointer base should be detected as pointer-valued.
	// This ensures let-bindings like `let p = &arr[0]` are not baked in HLSL.
	ptrHandle := ir.TypeHandle(1)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},     // 0: i32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}}, // 1: ptr<function, i32>
		},
	}
	names := map[nameKey]string{}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "int"})

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // 0: arg (pointer)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: arg[0]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &ptrHandle}, // 0: pointer type
			{},                   // 1: no pointer type in ExpressionTypes
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}
	setCurrentFunction(w, fn)

	// Expression 1 (AccessIndex on pointer base) should be pointer-valued
	if !w.isPointerExpression(1) {
		t.Error("AccessIndex on pointer base should be detected as pointer expression")
	}
}

// =============================================================================
// TestBakeRefCountForLoad
// =============================================================================

func TestBakeRefCountForLoad(t *testing.T) {
	// Load expressions have bake_ref_count=1, so they should always be baked
	// when referenced at least once (matching Rust naga behavior).
	load := ir.ExprLoad{Pointer: 0}
	got := bakeRefCount(load)
	if got != 1 {
		t.Errorf("bakeRefCount(ExprLoad) = %d, want 1", got)
	}
}

// =============================================================================
// TestBakeRefCountForBinary
// =============================================================================

func TestBakeRefCountForBinary(t *testing.T) {
	// Binary expressions have bake_ref_count=2, so they are only baked
	// when referenced 2+ times.
	binary := ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}
	got := bakeRefCount(binary)
	if got != 2 {
		t.Errorf("bakeRefCount(ExprBinary) = %d, want 2", got)
	}
}

// =============================================================================
// TestMatCx2StoreDetection
// =============================================================================

func TestMatCx2StoreDetection(t *testing.T) {
	// Setup: Baz struct with mat3x2 member (matCx2)
	scalarTy := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	matTy := ir.Type{Name: "mat3x2<f32>", Inner: ir.MatrixType{Columns: 3, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}
	bazTy := ir.Type{Name: "Baz", Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "m", Type: 1, Offset: 0},
		},
		Span: 24,
	}}

	module := &ir.Module{
		Types: []ir.Type{scalarTy, matTy, bazTy},
	}

	t.Run("findMatCx2InAccessChain_direct_member", func(t *testing.T) {
		// Expression chain: LocalVar(t) -> AccessIndex(member=0) for t.m
		fn := &ir.Function{
			LocalVars: []ir.LocalVariable{
				{Name: "t", Type: 2}, // Baz type
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0] = t
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] = t.m (matCx2 member)
			},
		}

		w := newTestWriter(module, nil, nil)
		setCurrentFunction(w, fn)

		matExpr, vecIdx, scalarIdx := w.findMatCx2InAccessChain(1)
		if matExpr == nil {
			t.Fatal("expected to find matCx2 in access chain")
		}
		if *matExpr != 1 {
			t.Errorf("expected matExpr=1, got %d", *matExpr)
		}
		if vecIdx != nil {
			t.Error("expected nil vectorIdx for direct member access")
		}
		if scalarIdx != nil {
			t.Error("expected nil scalarIdx for direct member access")
		}
	})

	t.Run("findMatCx2InAccessChain_column_access", func(t *testing.T) {
		// Expression chain: LocalVar(t) -> AccessIndex(member=0) -> AccessIndex(col=0) for t.m[0]
		fn := &ir.Function{
			LocalVars: []ir.LocalVariable{
				{Name: "t", Type: 2},
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0] = t
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] = t.m
				{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}}, // [2] = t.m[0] (column 0)
			},
		}

		w := newTestWriter(module, nil, nil)
		setCurrentFunction(w, fn)

		matExpr, vecIdx, scalarIdx := w.findMatCx2InAccessChain(2)
		if matExpr == nil {
			t.Fatal("expected to find matCx2 in access chain")
		}
		if *matExpr != 1 {
			t.Errorf("expected matExpr=1, got %d", *matExpr)
		}
		if vecIdx == nil {
			t.Fatal("expected vectorIdx for column access")
		}
		if !vecIdx.isStatic || vecIdx.static_ != 0 {
			t.Errorf("expected static vectorIdx=0, got isStatic=%v, static=%d", vecIdx.isStatic, vecIdx.static_)
		}
		if scalarIdx != nil {
			t.Error("expected nil scalarIdx")
		}
	})

	t.Run("findMatCx2InAccessChain_non_matCx2", func(t *testing.T) {
		// A matrix with rows != 2 should NOT be detected
		mat4x3Ty := ir.Type{Name: "mat4x3<f32>", Inner: ir.MatrixType{Columns: 4, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}
		otherStTy := ir.Type{Name: "Other", Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "m", Type: 3, Offset: 0},
			},
			Span: 64,
		}}

		mod := &ir.Module{
			Types: []ir.Type{scalarTy, matTy, bazTy, mat4x3Ty, otherStTy},
		}

		fn := &ir.Function{
			LocalVars: []ir.LocalVariable{
				{Name: "o", Type: 4}, // Other type
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			},
		}

		w := newTestWriter(mod, nil, nil)
		setCurrentFunction(w, fn)

		matExpr, _, _ := w.findMatCx2InAccessChain(1)
		if matExpr != nil {
			t.Error("should not detect mat4x3 as matCx2")
		}
	})
}

// TestResolveExprTypeHandle tests the expression type resolution function.
func TestResolveExprTypeHandle(t *testing.T) {
	scalarTy := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	bazTy := ir.Type{Name: "Baz", Inner: ir.StructType{
		Members: []ir.StructMember{
			{Name: "x", Type: 0, Offset: 0},
		},
		Span: 4,
	}}

	module := &ir.Module{
		Types: []ir.Type{scalarTy, bazTy},
	}

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "b", Type: 1}, // Baz type
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // [0] = b
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] = b.x
		},
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	t.Run("local_variable", func(t *testing.T) {
		ty := w.resolveExprTypeHandle(fn, 0)
		if ty == nil || *ty != 1 {
			t.Errorf("expected type handle 1 (Baz), got %v", ty)
		}
	})

	t.Run("struct_member_access", func(t *testing.T) {
		ty := w.resolveExprTypeHandle(fn, 1)
		if ty == nil || *ty != 0 {
			t.Errorf("expected type handle 0 (f32), got %v", ty)
		}
	})
}

// TestResolveExpressionTypeFallback verifies the fallback type resolver for
// Splat expressions when ExpressionTypes is not populated.
func TestResolveExpressionTypeFallback(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
		},
	}
	f32Handle := ir.TypeHandle(0)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}}, // 0: literal f32
			{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 0}}, // 1: splat to vec3
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle}, // 0: has type
			{},                   // 1: missing type (nil)
		},
	}

	w := &Writer{module: module, currentFunction: fn}

	// Expression 1 (Splat) has empty ExpressionTypes - getExpressionType returns nil
	if got := w.getExpressionType(1); got != nil {
		t.Fatal("expected nil from getExpressionType for expression 1")
	}

	// Fallback should resolve to VectorType{Size: 3, Scalar: f32}
	resolved := w.resolveExpressionTypeFallback(1)
	if resolved == nil {
		t.Fatal("resolveExpressionTypeFallback returned nil")
	}
	vec, ok := resolved.Inner.(ir.VectorType)
	if !ok {
		t.Fatalf("expected VectorType, got %T", resolved.Inner)
	}
	if vec.Size != ir.Vec3 {
		t.Errorf("expected size Vec3, got %d", vec.Size)
	}
	if vec.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("expected float scalar, got %v", vec.Scalar.Kind)
	}
}

// =============================================================================
// TestWriteImageAtomicStatement — InterlockedXxx on storage textures
// =============================================================================

func TestWriteImageAtomicStatement(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                       // 0: u32
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage}},                           // 1: texture_storage_2d
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}}, // 2: vec2<i32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "img", Type: ir.TypeHandle(1), Space: ir.SpaceHandle},
		},
	}

	u32Handle := ir.TypeHandle(0)
	imgHandle := ir.TypeHandle(1)
	vec2iHandle := ir.TypeHandle(2)

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "img",
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                                                    // 0: img
			{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{ir.ExpressionHandle(3), 3}}}, // 1: coord
			{Kind: ir.Literal{Value: ir.LiteralU32(42)}},                                                  // 2: value
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},                                                   // 3: coord component
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &imgHandle},
			{Handle: &vec2iHandle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	tests := []struct {
		name        string
		fun         ir.AtomicFunction
		wantContain string
	}{
		{"add", ir.AtomicAdd{}, "InterlockedAdd("},
		{"exchange", ir.AtomicExchange{}, "InterlockedExchange("},
		{"min", ir.AtomicMin{}, "InterlockedMin("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})
			setCurrentFunction(w, fn)
			w.out.Reset()
			w.indent = 0

			stmt := ir.StmtImageAtomic{
				Image:      0,
				Coordinate: 1,
				Fun:        tt.fun,
				Value:      2,
			}
			err := w.writeImageAtomicStatement(stmt)
			if err != nil {
				t.Fatalf("writeImageAtomicStatement: %v", err)
			}
			got := w.out.String()

			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("expected to contain %q, got %q", tt.wantContain, got)
			}
			// Must reference the image with bracket indexing
			if !strings.Contains(got, "img[") {
				t.Errorf("expected image bracket access img[, got %q", got)
			}
			// Must end with semicolon + newline
			if !strings.HasSuffix(got, ");\n") {
				t.Errorf("expected to end with );\\n, got %q", got)
			}
		})
	}
}

// =============================================================================
// TestWriteWorkGroupUniformLoadStatement — barrier+load+barrier pattern
// =============================================================================

func TestWriteWorkGroupUniformLoadStatement(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},      // 0: u32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceWorkGroup}}, // 1: ptr<workgroup, u32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "wg_data", Space: ir.SpaceWorkGroup, Type: 0},
		},
	}

	wgPtrHandle := ir.TypeHandle(1)
	u32Handle := ir.TypeHandle(0)

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "wg_data",
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},  // 0: wg_data ptr
			{Kind: ir.ExprWorkGroupUniformLoadResult{}}, // 1: result
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &wgPtrHandle},
			{Handle: &u32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "uint"})
	setCurrentFunction(w, fn)
	w.out.Reset()
	w.indent = 0

	stmt := ir.StmtWorkGroupUniformLoad{
		Pointer: 0,
		Result:  1,
	}
	err := w.writeWorkGroupUniformLoadStatement(stmt)
	if err != nil {
		t.Fatalf("writeWorkGroupUniformLoadStatement: %v", err)
	}
	got := w.out.String()

	// Must contain two barriers
	barrierCount := strings.Count(got, "GroupMemoryBarrierWithGroupSync()")
	if barrierCount != 2 {
		t.Errorf("expected 2 barriers, got %d in:\n%s", barrierCount, got)
	}

	// Must contain the assignment with result name
	if !strings.Contains(got, "_e1") {
		t.Errorf("expected result name _e1, got:\n%s", got)
	}

	// Must contain the load from pointer
	if !strings.Contains(got, "wg_data") {
		t.Errorf("expected pointer reference wg_data, got:\n%s", got)
	}

	// Verify the named expression was cached
	if w.namedExpressions[1] != "_e1" {
		t.Errorf("expected namedExpressions[1] = _e1, got %q", w.namedExpressions[1])
	}
}
