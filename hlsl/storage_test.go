// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test Helpers
// =============================================================================

// newTestWriter creates a minimal Writer suitable for unit testing storage functions.
// It sets up the module, names, typeNames, and a current function with the given
// expressions and their type resolutions.
func newTestWriter(module *ir.Module, names map[nameKey]string, typeNames map[ir.TypeHandle]string) *Writer {
	w := &Writer{
		module:                    module,
		options:                   &Options{BindingMap: make(map[ResourceBinding]BindTarget)},
		names:                     names,
		typeNames:                 typeNames,
		namer:                     newNamer(),
		epResultTypes:             make(map[ir.TypeHandle]epResultInfo),
		entryPointIO:              make(map[int]*entryPointInterface),
		entryPointNames:           make(map[string]string),
		registerBindings:          make(map[string]string),
		needsStorageLoadHelpers:   make(map[string]bool),
		structConstructors:        make(map[ir.TypeHandle]struct{}),
		structConstructorsWritten: make(map[ir.TypeHandle]struct{}),
		arrayConstructorsWritten:  make(map[ir.TypeHandle]struct{}),
		wrappedZeroValues:         make(map[ir.TypeHandle]struct{}),
		wrappedBinaryOps:          make(map[wrappedBinaryOpKey]struct{}),
		wrappedStructMatrixAccess: make(map[wrappedStructMatrixAccessKey]struct{}),
	}
	if names == nil {
		w.names = make(map[nameKey]string)
	}
	if typeNames == nil {
		w.typeNames = make(map[ir.TypeHandle]string)
	}
	return w
}

// setCurrentFunction sets the writer's current function context for tests.
func setCurrentFunction(w *Writer, fn *ir.Function) {
	w.currentFunction = fn
	w.currentFuncHandle = 0
	w.currentEPIndex = -1
	w.localNames = make(map[uint32]string)
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpressions = make(map[ir.ExpressionHandle]struct{})
}

// output returns the writer's accumulated output and resets the buffer.
func output(w *Writer) string {
	s := w.out.String()
	w.out.Reset()
	return s
}

// =============================================================================
// TestAlignedOffset
// =============================================================================

func TestAlignedOffset(t *testing.T) {
	tests := []struct {
		name      string
		offset    uint32
		alignment uint32
		want      uint32
	}{
		{"already_aligned_4", 0, 4, 0},
		{"align_1_to_4", 1, 4, 4},
		{"align_3_to_4", 3, 4, 4},
		{"align_4_to_4", 4, 4, 4},
		{"align_5_to_4", 5, 4, 8},
		{"align_to_16", 12, 16, 16},
		{"align_15_to_16", 15, 16, 16},
		{"align_16_to_16", 16, 16, 16},
		{"zero_alignment", 5, 0, 5},
		{"align_1_to_1", 7, 1, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignedOffset(tt.offset, tt.alignment)
			if got != tt.want {
				t.Errorf("alignedOffset(%d, %d) = %d, want %d", tt.offset, tt.alignment, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestCalculateBufferOffset
// =============================================================================

func TestCalculateBufferOffset(t *testing.T) {
	tests := []struct {
		name         string
		base, member uint32
		want         uint32
	}{
		{"zero_zero", 0, 0, 0},
		{"zero_offset", 0, 16, 16},
		{"base_plus_member", 64, 8, 72},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBufferOffset(tt.base, tt.member)
			if got != tt.want {
				t.Errorf("calculateBufferOffset(%d, %d) = %d, want %d", tt.base, tt.member, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestGetScalarTypeSize
// =============================================================================

func TestGetScalarTypeSize(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   uint32
	}{
		{"bool_1byte", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, 1},
		{"u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, 4},
		{"f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 4},
		{"f64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, 8},
		{"i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getScalarTypeSize(tt.scalar)
			if got != tt.want {
				t.Errorf("getScalarTypeSize(%v) = %d, want %d", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestGetTypeAlignment
// =============================================================================

func TestGetTypeAlignment(t *testing.T) {
	// Build a module with various types for alignment testing
	module := &ir.Module{
		Types: []ir.Type{
			// [0] u32
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// [1] f32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// [2] vec2<f32>
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [3] vec4<f32>
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [4] mat4x4<f32>
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [5] struct { u32, f32 }
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: 0, Offset: 0},
					{Name: "b", Type: 1, Offset: 4},
				},
				Span: 8,
			}},
		},
	}

	tests := []struct {
		name   string
		handle ir.TypeHandle
		want   uint32
	}{
		{"scalar_u32", 0, 4},
		{"scalar_f32", 1, 4},
		{"vec2_f32", 2, 8},
		{"vec4_f32", 3, 16},
		{"mat4x4_f32", 4, 16},
		{"struct", 5, 4},
		{"out_of_range", 99, 4}, // default
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTypeAlignment(module, tt.handle)
			if got != tt.want {
				t.Errorf("getTypeAlignment(handle=%d) = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestGetTypeSize
// =============================================================================

func TestGetTypeSize(t *testing.T) {
	four := uint32(4)
	module := &ir.Module{
		Types: []ir.Type{
			// [0] u32
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			// [1] vec4<f32>
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [2] mat2x3<f32>
			{Inner: ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			// [3] array<u32, 4> stride=4
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &four}, Stride: 4}},
			// [4] struct { span: 24 }
			{Inner: ir.StructType{Span: 24}},
		},
	}

	tests := []struct {
		name   string
		handle ir.TypeHandle
		want   uint32
	}{
		{"u32", 0, 4},
		{"vec4_f32", 1, 16},
		{"mat2x3_f32", 2, 24}, // 2*3*4
		{"array_u32_4", 3, 16},
		{"struct_span24", 4, 24},
		{"out_of_range", 99, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTypeSize(module, tt.handle)
			if got != tt.want {
				t.Errorf("getTypeSize(handle=%d) = %d, want %d", tt.handle, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestAlignmentFromVectorSize
// =============================================================================

func TestAlignmentFromVectorSize(t *testing.T) {
	tests := []struct {
		size ir.VectorSize
		want uint32
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
	}
	for _, tt := range tests {
		got := alignmentFromVectorSize(tt.size)
		if got != tt.want {
			t.Errorf("alignmentFromVectorSize(%d) = %d, want %d", tt.size, got, tt.want)
		}
	}
}

// =============================================================================
// TestScalarHLSLCast
// =============================================================================

func TestScalarHLSLCast(t *testing.T) {
	tests := []struct {
		kind ir.ScalarKind
		want string
	}{
		{ir.ScalarFloat, "asfloat"},
		{ir.ScalarSint, "asint"},
		{ir.ScalarUint, "asuint"},
		{ir.ScalarBool, "asuint"}, // default
	}
	for _, tt := range tests {
		got := scalarHLSLCast(tt.kind)
		if got != tt.want {
			t.Errorf("scalarHLSLCast(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// =============================================================================
// TestAtomicFunctionToHLSL
// =============================================================================

func TestAtomicFunctionToHLSL(t *testing.T) {
	tests := []struct {
		name    string
		fun     ir.AtomicFunction
		want    string
		wantErr bool
	}{
		{"add", ir.AtomicAdd{}, hlslInterlockedAdd, false},
		{"subtract", ir.AtomicSubtract{}, hlslInterlockedAdd, false}, // negated
		{"and", ir.AtomicAnd{}, hlslInterlockedAnd, false},
		{"xor", ir.AtomicExclusiveOr{}, hlslInterlockedXor, false},
		{"or", ir.AtomicInclusiveOr{}, hlslInterlockedOr, false},
		{"min", ir.AtomicMin{}, hlslInterlockedMin, false},
		{"max", ir.AtomicMax{}, hlslInterlockedMax, false},
		{"exchange", ir.AtomicExchange{}, hlslInterlockedExchange, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := atomicFunctionToHLSL(tt.fun)
			if (err != nil) != tt.wantErr {
				t.Errorf("atomicFunctionToHLSL(%T) error = %v, wantErr %v", tt.fun, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("atomicFunctionToHLSL(%T) = %q, want %q", tt.fun, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestIsAtomicSubtract
// =============================================================================

func TestIsAtomicSubtract(t *testing.T) {
	if !isAtomicSubtract(ir.AtomicSubtract{}) {
		t.Error("isAtomicSubtract(AtomicSubtract{}) should be true")
	}
	if isAtomicSubtract(ir.AtomicAdd{}) {
		t.Error("isAtomicSubtract(AtomicAdd{}) should be false")
	}
}

// =============================================================================
// TestGetRegisterTypeForAddressSpace
// =============================================================================

func TestGetRegisterTypeForAddressSpace(t *testing.T) {
	tests := []struct {
		name     string
		space    ir.AddressSpace
		readOnly bool
		want     RegisterType
	}{
		{"uniform", ir.SpaceUniform, false, RegisterTypeB},
		{"storage_rw", ir.SpaceStorage, false, RegisterTypeU},
		{"storage_ro", ir.SpaceStorage, true, RegisterTypeT},
		{"handle", ir.SpaceHandle, false, RegisterTypeT},
		{"default", ir.SpaceFunction, false, RegisterTypeT},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRegisterTypeForAddressSpace(tt.space, tt.readOnly)
			if got != tt.want {
				t.Errorf("getRegisterTypeForAddressSpace(%d, %v) = %d, want %d", tt.space, tt.readOnly, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestGetRegisterBinding
// =============================================================================

func TestGetRegisterBinding(t *testing.T) {
	tests := []struct {
		name    string
		regType RegisterType
		binding *BindTarget
		want    string
	}{
		{"nil_binding", RegisterTypeT, nil, ""},
		{"t0_space0", RegisterTypeT, &BindTarget{Register: 0, Space: 0}, ": register(t0, space0)"},
		{"u3_space1", RegisterTypeU, &BindTarget{Register: 3, Space: 1}, ": register(u3, space1)"},
		{"b0_space2", RegisterTypeB, &BindTarget{Register: 0, Space: 2}, ": register(b0, space2)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getRegisterBinding(tt.regType, tt.binding)
			if got != tt.want {
				t.Errorf("getRegisterBinding(%s, %v) = %q, want %q", tt.regType, tt.binding, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestWriteByteAddressBuffer
// =============================================================================

func TestWriteByteAddressBuffer(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name     string
		bufName  string
		binding  *BindTarget
		readOnly bool
		want     string
	}{
		{
			"rw_with_binding",
			"buf", &BindTarget{Register: 0, Space: 0}, false,
			"RWByteAddressBuffer buf : register(u0, space0);\n",
		},
		{
			"ro_with_binding",
			"buf", &BindTarget{Register: 1, Space: 2}, true,
			"ByteAddressBuffer buf : register(t1, space2);\n",
		},
		{
			"rw_no_binding",
			"myBuf", nil, false,
			"RWByteAddressBuffer myBuf;\n",
		},
		{
			"ro_no_binding",
			"data", nil, true,
			"ByteAddressBuffer data;\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.indent = 0
			w.writeByteAddressBuffer(tt.bufName, tt.binding, tt.readOnly)
			got := output(w)
			if got != tt.want {
				t.Errorf("writeByteAddressBuffer:\n  got:  %q\n  want: %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestWriteStructuredBuffer
// =============================================================================

func TestWriteStructuredBuffer(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name     string
		bufName  string
		elemType string
		binding  *BindTarget
		readOnly bool
		want     string
	}{
		{
			"rw_float4",
			"buf", "float4", &BindTarget{Register: 0, Space: 0}, false,
			"RWStructuredBuffer<float4> buf : register(u0, space0);\n",
		},
		{
			"ro_uint",
			"data", "uint", &BindTarget{Register: 2, Space: 1}, true,
			"StructuredBuffer<uint> data : register(t2, space1);\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.indent = 0
			w.writeStructuredBuffer(tt.bufName, tt.elemType, tt.binding, tt.readOnly)
			got := output(w)
			if got != tt.want {
				t.Errorf("writeStructuredBuffer:\n  got:  %q\n  want: %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestWriteConstantBuffer
// =============================================================================

func TestWriteConstantBuffer(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	t.Run("with_binding", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		w.writeConstantBuffer("Params", []cbufferMember{
			{name: "mvp", typeName: "float4x4"},
			{name: "color", typeName: "float4"},
		}, &BindTarget{Register: 0, Space: 0})
		got := output(w)
		want := "cbuffer Params : register(b0, space0) {\n    float4x4 mvp;\n    float4 color;\n};\n"
		if got != want {
			t.Errorf("writeConstantBuffer:\n  got:  %q\n  want: %q", got, want)
		}
	})

	t.Run("no_binding", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		w.writeConstantBuffer("Data", []cbufferMember{
			{name: "x", typeName: "float"},
		}, nil)
		got := output(w)
		want := "cbuffer Data {\n    float x;\n};\n"
		if got != want {
			t.Errorf("writeConstantBuffer:\n  got:  %q\n  want: %q", got, want)
		}
	})
}

// =============================================================================
// TestWriteBufferLoad
// =============================================================================

func TestWriteBufferLoad(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name       string
		bufExpr    string
		offset     string
		components int
		want       string
	}{
		{"load1", "buf", "0", 1, "buf.Load(0)"},
		{"load2", "buf", "16", 2, "buf.Load2(16)"},
		{"load3", "buf", "offset", 3, "buf.Load3(offset)"},
		{"load4", "data", "idx*4", 4, "data.Load4(idx*4)"},
		{"load_default_ge5", "buf", "0", 5, "buf.Load4(0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.writeBufferLoad(tt.bufExpr, tt.offset, tt.components)
			got := output(w)
			if got != tt.want {
				t.Errorf("writeBufferLoad(%d):\n  got:  %q\n  want: %q", tt.components, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestWriteBufferLoadT
// =============================================================================

func TestWriteBufferLoadT(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	w.writeBufferLoadT("buf", "float4x4", "32")
	got := output(w)
	want := "buf.Load<float4x4>(32)"
	if got != want {
		t.Errorf("writeBufferLoadT:\n  got:  %q\n  want: %q", got, want)
	}
}

// =============================================================================
// TestWriteBufferStore
// =============================================================================

func TestWriteBufferStore(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		name       string
		bufExpr    string
		offset     string
		value      string
		components int
		wantSuffix string // check suffix since writeLine adds indent+newline
	}{
		{"store1", "buf", "0", "val", 1, "buf.Store(0, val);\n"},
		{"store2", "buf", "8", "val", 2, "buf.Store2(8, val);\n"},
		{"store3", "buf", "off", "v", 3, "buf.Store3(off, v);\n"},
		{"store4", "buf", "0", "v", 4, "buf.Store4(0, v);\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.indent = 0
			w.writeBufferStore(tt.bufExpr, tt.offset, tt.value, tt.components)
			got := output(w)
			if got != tt.wantSuffix {
				t.Errorf("writeBufferStore(%d):\n  got:  %q\n  want: %q", tt.components, got, tt.wantSuffix)
			}
		})
	}
}

// =============================================================================
// TestWriteAtomicOp
// =============================================================================

func TestWriteAtomicOp(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	t.Run("with_result", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		result := "orig"
		err := w.writeAtomicOp(ir.AtomicAdd{}, "dest", "1", &result)
		if err != nil {
			t.Fatalf("writeAtomicOp: %v", err)
		}
		got := output(w)
		want := "InterlockedAdd(dest, 1, orig);\n"
		if got != want {
			t.Errorf("writeAtomicOp with result:\n  got:  %q\n  want: %q", got, want)
		}
	})

	t.Run("without_result", func(t *testing.T) {
		w.out.Reset()
		w.indent = 0
		err := w.writeAtomicOp(ir.AtomicAnd{}, "mem", "mask", nil)
		if err != nil {
			t.Fatalf("writeAtomicOp: %v", err)
		}
		got := output(w)
		want := "InterlockedAnd(mem, mask);\n"
		if got != want {
			t.Errorf("writeAtomicOp without result:\n  got:  %q\n  want: %q", got, want)
		}
	})
}

// =============================================================================
// TestWriteAtomicCompareExchange
// =============================================================================

func TestWriteAtomicCompareExchange(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	w.indent = 0
	w.writeAtomicCompareExchange("dest", "cmp", "val", "orig")
	got := output(w)
	want := "InterlockedCompareExchange(dest, cmp, val, orig);\n"
	if got != want {
		t.Errorf("writeAtomicCompareExchange:\n  got:  %q\n  want: %q", got, want)
	}
}

// =============================================================================
// TestWriteAtomicCompareStore
// =============================================================================

func TestWriteAtomicCompareStore(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	w.indent = 0
	w.writeAtomicCompareStore("dest", "cmp", "val")
	got := output(w)
	want := "InterlockedCompareStore(dest, cmp, val);\n"
	if got != want {
		t.Errorf("writeAtomicCompareStore:\n  got:  %q\n  want: %q", got, want)
	}
}

// =============================================================================
// TestWriteAtomicExchange
// =============================================================================

func TestWriteAtomicExchange(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	w.indent = 0
	w.writeAtomicExchange("dest", "val", "orig")
	got := output(w)
	want := "InterlockedExchange(dest, val, orig);\n"
	if got != want {
		t.Errorf("writeAtomicExchange:\n  got:  %q\n  want: %q", got, want)
	}
}

// =============================================================================
// TestIsStoragePointer
// =============================================================================

func TestIsStoragePointer(t *testing.T) {
	// Module setup:
	// Types:
	//   [0] u32
	//   [1] ptr<storage, u32>
	//   [2] ptr<uniform, u32>
	// GlobalVariables:
	//   [0] var<storage> storBuf: u32
	//   [1] var<uniform> uniBuf: u32
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},    // 0: u32
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceStorage}}, // 1: ptr<storage, u32>
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceUniform}}, // 2: ptr<uniform, u32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "storBuf", Space: ir.SpaceStorage, Type: 0},
			{Name: "uniBuf", Space: ir.SpaceUniform, Type: 0},
		},
	}

	storHandle := ir.TypeHandle(1)
	uniHandle := ir.TypeHandle(2)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			// [0] GlobalVariable(0) -> storBuf (storage)
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			// [1] GlobalVariable(1) -> uniBuf (uniform)
			{Kind: ir.ExprGlobalVariable{Variable: 1}},
			// [2] AccessIndex on expr[0] (storage chain)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &storHandle}, // [0] -> ptr<storage>
			{Handle: &uniHandle},  // [1] -> ptr<uniform>
			{Handle: &storHandle}, // [2] -> ptr<storage> (access into storage)
		},
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	t.Run("storage_global", func(t *testing.T) {
		if !w.isStoragePointer(0) {
			t.Error("expr[0] (storage global) should be a storage pointer")
		}
	})

	t.Run("uniform_global", func(t *testing.T) {
		if w.isStoragePointer(1) {
			t.Error("expr[1] (uniform global) should NOT be a storage pointer")
		}
	})

	t.Run("access_into_storage", func(t *testing.T) {
		if !w.isStoragePointer(2) {
			t.Error("expr[2] (access into storage) should be a storage pointer")
		}
	})

	t.Run("nil_function", func(t *testing.T) {
		w2 := newTestWriter(module, nil, nil)
		// currentFunction is nil
		if w2.isStoragePointer(0) {
			t.Error("nil function should return false")
		}
	})
}

// =============================================================================
// TestFillAccessChain
// =============================================================================

func TestFillAccessChain(t *testing.T) {
	// Module:
	// Types:
	//   [0] u32
	//   [1] f32
	//   [2] struct S { a: u32 (offset 0), b: f32 (offset 4) }
	//   [3] array<u32, 4> stride=4
	//   [4] ptr<storage, struct S>
	//   [5] vec4<f32>
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                                                         // 0: u32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                                        // 1: f32
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}, {Name: "b", Type: 1, Offset: 4}}, Span: 8}}, // 2: struct
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: ptrU32(4)}, Stride: 4}},                                            // 3: array<u32,4>
			{Inner: ir.PointerType{Base: 2, Space: ir.SpaceStorage}},                                                                      // 4: ptr<storage, S>
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},                                  // 5: vec4<f32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 4},
		},
	}

	ptrHandle := ir.TypeHandle(4)
	structHandle := ir.TypeHandle(2)

	t.Run("struct_member_access", func(t *testing.T) {
		// expr[0] = GlobalVariable(0)  -> buf
		// expr[1] = AccessIndex(base=0, index=1)  -> buf.b (offset=4)
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},
				{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &ptrHandle},
				{Handle: &structHandle}, // resolves to struct (deref'd by computeSubAccess)
			},
		}

		w := newTestWriter(module, nil, nil)
		setCurrentFunction(w, fn)

		varHandle, err := w.fillAccessChain(1)
		if err != nil {
			t.Fatalf("fillAccessChain: %v", err)
		}
		if varHandle != 0 {
			t.Errorf("expected global var handle 0, got %d", varHandle)
		}
		if len(w.tempAccessChain) != 1 {
			t.Fatalf("expected 1 chain element, got %d", len(w.tempAccessChain))
		}
		if w.tempAccessChain[0].kind != subAccessOffset {
			t.Errorf("expected subAccessOffset, got %d", w.tempAccessChain[0].kind)
		}
		if w.tempAccessChain[0].offset != 4 {
			t.Errorf("expected offset 4, got %d", w.tempAccessChain[0].offset)
		}
	})

	t.Run("direct_global", func(t *testing.T) {
		fn := &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &ptrHandle},
			},
		}

		w := newTestWriter(module, nil, nil)
		setCurrentFunction(w, fn)

		varHandle, err := w.fillAccessChain(0)
		if err != nil {
			t.Fatalf("fillAccessChain: %v", err)
		}
		if varHandle != 0 {
			t.Errorf("expected global var handle 0, got %d", varHandle)
		}
		if len(w.tempAccessChain) != 0 {
			t.Errorf("expected 0 chain elements, got %d", len(w.tempAccessChain))
		}
	})
}

// =============================================================================
// TestWriteStorageAddress
// =============================================================================

func TestWriteStorageAddress(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	t.Run("empty_chain", func(t *testing.T) {
		w.out.Reset()
		err := w.writeStorageAddress(nil)
		if err != nil {
			t.Fatalf("writeStorageAddress: %v", err)
		}
		got := output(w)
		if got != "0" {
			t.Errorf("empty chain: got %q, want \"0\"", got)
		}
	})

	t.Run("single_offset", func(t *testing.T) {
		w.out.Reset()
		chain := []subAccess{{kind: subAccessOffset, offset: 16}}
		err := w.writeStorageAddress(chain)
		if err != nil {
			t.Fatalf("writeStorageAddress: %v", err)
		}
		got := output(w)
		if got != "16" {
			t.Errorf("single offset: got %q, want \"16\"", got)
		}
	})

	t.Run("two_offsets", func(t *testing.T) {
		w.out.Reset()
		chain := []subAccess{
			{kind: subAccessOffset, offset: 8},
			{kind: subAccessOffset, offset: 4},
		}
		err := w.writeStorageAddress(chain)
		if err != nil {
			t.Fatalf("writeStorageAddress: %v", err)
		}
		got := output(w)
		if got != "8+4" {
			t.Errorf("two offsets: got %q, want \"8+4\"", got)
		}
	})
}

// =============================================================================
// TestWriteStorageLoad
// =============================================================================

func TestWriteStorageLoad(t *testing.T) {
	// Module with a storage buffer global named "buf"
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                        // 0: u32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 1: f32
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2: vec2<f32>
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 3: vec4<f32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 0},
		},
	}

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buf",
	}
	w := newTestWriter(module, names, nil)

	tests := []struct {
		name     string
		resultTy ir.TypeInner
		chain    []subAccess
		want     string
	}{
		{
			"scalar_u32",
			ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			[]subAccess{{kind: subAccessOffset, offset: 0}},
			"asuint(buf.Load(0))",
		},
		{
			"scalar_f32",
			ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			[]subAccess{{kind: subAccessOffset, offset: 8}},
			"asfloat(buf.Load(8))",
		},
		{
			"scalar_i32",
			ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			[]subAccess{{kind: subAccessOffset, offset: 4}},
			"asint(buf.Load(4))",
		},
		{
			"vec2_f32",
			ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			[]subAccess{{kind: subAccessOffset, offset: 0}},
			"asfloat(buf.Load2(0))",
		},
		{
			"vec4_f32",
			ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			[]subAccess{{kind: subAccessOffset, offset: 16}},
			"asfloat(buf.Load4(16))",
		},
		{
			"vec4_u32",
			ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			nil,
			"asuint(buf.Load4(0))",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.out.Reset()
			w.tempAccessChain = tt.chain
			err := w.writeStorageLoad(0, tt.resultTy, nil)
			if err != nil {
				t.Fatalf("writeStorageLoad: %v", err)
			}
			got := output(w)
			if got != tt.want {
				t.Errorf("writeStorageLoad(%s):\n  got:  %q\n  want: %q", tt.name, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestWriteStorageStore
// =============================================================================

func TestWriteStorageStore(t *testing.T) {
	// We need a module with types and a function with expressions for storeValue
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                        // 0: u32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 1: f32
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 2: vec2<f32>
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 0},
		},
	}

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buf",
	}
	typeNames := map[ir.TypeHandle]string{
		0: "uint",
		1: "float",
		2: "float2",
	}
	w := newTestWriter(module, names, typeNames)

	// Set up a function with a literal expression for testing storeValueExpression
	litExprHandle := ir.TypeHandle(0)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(42)}}, // expr[0] = 42u
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &litExprHandle}, // type = u32
		},
	}
	setCurrentFunction(w, fn)

	t.Run("scalar_u32_store", func(t *testing.T) {
		w.out.Reset()
		w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 0}}
		sv := storeValue{kind: storeValueTempIndex, depth: 1, index: 0}
		err := w.writeStorageStore(0, sv, 0, nil)
		if err != nil {
			t.Fatalf("writeStorageStore: %v", err)
		}
		got := output(w)
		// Scalar u32 width=4: buf.Store(0, asuint(_value1[0]));
		want := "buf.Store(0, asuint(_value1[0]));\n"
		if got != want {
			t.Errorf("scalar u32 store:\n  got:  %q\n  want: %q", got, want)
		}
	})

	t.Run("vec2_f32_store", func(t *testing.T) {
		w.out.Reset()
		// For storeValueTempIndex, the type is resolved via getTypeInner(sv.base)
		// sv.base should be the vec2<f32> type handle
		w.tempAccessChain = []subAccess{{kind: subAccessOffset, offset: 8}}
		sv := storeValue{kind: storeValueTempIndex, depth: 1, index: 0, base: 2} // base=2 -> vec2<f32>
		err := w.writeStorageStore(0, sv, 0, nil)
		if err != nil {
			t.Fatalf("writeStorageStore: %v", err)
		}
		got := output(w)
		// Vec2 f32 width=4: buf.Store2(8, asuint(_value1[0]));
		want := "buf.Store2(8, asuint(_value1[0]));\n"
		if got != want {
			t.Errorf("vec2 f32 store:\n  got:  %q\n  want: %q", got, want)
		}
	})
}

// =============================================================================
// TestScalarToHLSLStr
// =============================================================================

func TestScalarToHLSLStr(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"int32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{"int64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "int64_t"},
		{"uint32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{"uint64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "uint64_t"},
		{"float16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "half"},
		{"float32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{"float64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "double"},
		{"bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarToHLSLStr(tt.scalar)
			if got != tt.want {
				t.Errorf("scalarToHLSLStr(%v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TestFormatBinding
// =============================================================================

func TestFormatBinding(t *testing.T) {
	t.Run("without_array", func(t *testing.T) {
		bt := BindTarget{Register: 3, Space: 1}
		got := formatBinding(RegisterTypeU, bt)
		want := ": register(u3, space1)"
		if got != want {
			t.Errorf("formatBinding: got %q, want %q", got, want)
		}
	})

	t.Run("with_array", func(t *testing.T) {
		arrSize := uint32(8)
		bt := BindTarget{Register: 0, Space: 0, BindingArraySize: &arrSize}
		got := formatBinding(RegisterTypeT, bt)
		want := ": register(t0, space0) /* array[8] */"
		if got != want {
			t.Errorf("formatBinding with array: got %q, want %q", got, want)
		}
	})
}

// =============================================================================
// TestGetSpaceBinding
// =============================================================================

func TestGetSpaceBinding(t *testing.T) {
	got := getSpaceBinding(RegisterTypeU, 2, 3)
	want := ": register(u2, space3)"
	if got != want {
		t.Errorf("getSpaceBinding: got %q, want %q", got, want)
	}
}

// =============================================================================
// TestIndentStr
// =============================================================================

func TestIndentStr(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)

	tests := []struct {
		level int
		want  string
	}{
		{0, ""},
		{1, "    "},
		{2, "        "},
		{3, "            "},
	}
	for _, tt := range tests {
		got := w.indentStr(tt.level)
		if got != tt.want {
			t.Errorf("indentStr(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// =============================================================================
// TestNagaDivMod — verify naga_div/naga_mod helper generation
// =============================================================================

func TestNagaDivMod(t *testing.T) {
	module := &ir.Module{}

	t.Run("div_helper", func(t *testing.T) {
		w := newTestWriter(module, nil, nil)
		w.needsDivHelper = true
		w.writeSpecialHelperFunctions()
		got := output(w)
		if !strings.Contains(got, "naga_div") {
			t.Error("expected naga_div helper in output")
		}
		if !strings.Contains(got, "b != 0 ? a / b : 0") {
			t.Error("expected safe division logic")
		}
	})

	t.Run("mod_helper", func(t *testing.T) {
		w := newTestWriter(module, nil, nil)
		w.needsModHelper = true
		w.writeSpecialHelperFunctions()
		got := output(w)
		if !strings.Contains(got, "naga_mod") {
			t.Error("expected naga_mod helper in output")
		}
		if !strings.Contains(got, "a - b * (a / b)") {
			t.Error("expected truncated mod logic")
		}
	})

	t.Run("no_helpers_when_not_needed", func(t *testing.T) {
		w := newTestWriter(module, nil, nil)
		w.writeSpecialHelperFunctions()
		got := output(w)
		if got != "" {
			t.Errorf("expected empty output when no helpers needed, got %q", got)
		}
	})
}

// =============================================================================
// TestWriteStorageLoadMatrix
// =============================================================================

func TestWriteStorageLoadMatrix(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 0},
		},
	}

	names := map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "buf",
	}
	w := newTestWriter(module, names, nil)

	// Test mat2x2<f32> load
	w.out.Reset()
	w.tempAccessChain = nil
	matTy := ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	err := w.writeStorageLoad(0, matTy, nil)
	if err != nil {
		t.Fatalf("writeStorageLoad matrix: %v", err)
	}
	got := output(w)
	// mat2x2 -> float2x2(col0, col1) where each column is a vec2 loaded via Load2
	// Row alignment for vec2 = 2, stride = 2*4 = 8
	if !strings.Contains(got, "float2x2(") {
		t.Errorf("expected float2x2 constructor, got %q", got)
	}
	if !strings.Contains(got, "buf.Load2(") {
		t.Errorf("expected buf.Load2 for vec2 columns, got %q", got)
	}
}

// =============================================================================
// TestArrayConstructorNaming
// =============================================================================

func TestArrayConstructorNaming(t *testing.T) {
	// Array constructors in storage loads should use hlslTypeId naming
	// (e.g., "Constructarray2_float_") not typeNames (e.g., "Constructtype_1_").
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                             // 0: float
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: ptrU32(2)}, Stride: 4}}, // 1: float[2]
		},
	}
	names := map[nameKey]string{}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "float", 1: "type_1_"})

	typeId := w.hlslTypeId(1)
	if typeId != "array2_float_" {
		t.Errorf("hlslTypeId for Array{float, 2} = %q, want %q", typeId, "array2_float_")
	}
}

// =============================================================================
// TestConstructorDependencyOrder
// =============================================================================

func TestConstructorDependencyOrder(t *testing.T) {
	// When a struct contains an array member and is loaded from storage,
	// the array constructor should be declared BEFORE the struct constructor.
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                                           // 0: float
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: ptrU32(2)}, Stride: 4}},                               // 1: float[2]
			{Name: "MyStruct", Inner: ir.StructType{Members: []ir.StructMember{{Name: "arr", Type: 1, Offset: 0}}, Span: 8}}, // 2: struct
		},
	}
	names := map[nameKey]string{}
	w := newTestWriter(module, names, map[ir.TypeHandle]string{0: "float", 1: "type_1_", 2: "MyStruct"})

	// Register constructors for the struct (should write array first, then struct)
	inner := w.module.Types[2].Inner
	w.registerConstructorsForType(2, inner)
	output := w.out.String()

	arrayPos := strings.Index(output, "Constructarray2_float_")
	structPos := strings.Index(output, "ConstructMyStruct")

	if arrayPos == -1 {
		t.Fatal("array constructor not written")
	}
	if structPos == -1 {
		t.Fatal("struct constructor not written")
	}
	if arrayPos > structPos {
		t.Error("array constructor should be declared before struct constructor (dependency order)")
	}
}

// =============================================================================
// TestWriteDynamicBufferOffsets
// =============================================================================

func TestWriteDynamicBufferOffsets(t *testing.T) {
	t.Run("no_targets_produces_nothing", func(t *testing.T) {
		w := newTestWriter(&ir.Module{}, nil, nil)
		w.options.DynamicStorageBufferOffsetsTargets = nil
		w.writeDynamicBufferOffsets()
		got := output(w)
		if got != "" {
			t.Errorf("expected empty output, got %q", got)
		}
	})

	t.Run("single_group", func(t *testing.T) {
		w := newTestWriter(&ir.Module{}, nil, nil)
		w.options.DynamicStorageBufferOffsetsTargets = map[uint32]OffsetsBindTarget{
			0: {Register: 1, Size: 2, Space: 0},
		}
		w.writeDynamicBufferOffsets()
		got := output(w)
		if !strings.Contains(got, "struct __dynamic_buffer_offsetsTy0 {") {
			t.Error("missing struct declaration")
		}
		if !strings.Contains(got, "uint _0;") || !strings.Contains(got, "uint _1;") {
			t.Error("missing uint members")
		}
		if !strings.Contains(got, "ConstantBuffer<__dynamic_buffer_offsetsTy0> __dynamic_buffer_offsets0: register(b1, space0);") {
			t.Errorf("missing ConstantBuffer declaration in output: %s", got)
		}
	})

	t.Run("multiple_groups_sorted", func(t *testing.T) {
		w := newTestWriter(&ir.Module{}, nil, nil)
		w.options.DynamicStorageBufferOffsetsTargets = map[uint32]OffsetsBindTarget{
			1: {Register: 2, Size: 1, Space: 0},
			0: {Register: 1, Size: 2, Space: 0},
		}
		w.writeDynamicBufferOffsets()
		got := output(w)
		// Group 0 should appear before group 1
		pos0 := strings.Index(got, "__dynamic_buffer_offsetsTy0")
		pos1 := strings.Index(got, "__dynamic_buffer_offsetsTy1")
		if pos0 == -1 || pos1 == -1 {
			t.Fatalf("missing struct declarations in output: %s", got)
		}
		if pos0 > pos1 {
			t.Error("group 0 should appear before group 1 (sorted order)")
		}
	})
}

// TestWriteStorageAddressWithBufferOffset verifies that subAccessBufferOffset
// entries in the access chain emit __dynamic_buffer_offsets{group}._{offset}.
func TestWriteStorageAddressWithBufferOffset(t *testing.T) {
	t.Run("buffer_offset_only", func(t *testing.T) {
		w := newTestWriter(&ir.Module{}, nil, nil)
		chain := []subAccess{
			{kind: subAccessOffset, offset: 0},
			{kind: subAccessBufferOffset, bufferGroup: 0, bufferOffset: 1},
		}
		err := w.writeStorageAddress(chain)
		if err != nil {
			t.Fatal(err)
		}
		got := output(w)
		want := "0+__dynamic_buffer_offsets0._1"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("multiple_groups", func(t *testing.T) {
		w := newTestWriter(&ir.Module{}, nil, nil)
		chain := []subAccess{
			{kind: subAccessOffset, offset: 4},
			{kind: subAccessBufferOffset, bufferGroup: 2, bufferOffset: 0},
		}
		err := w.writeStorageAddress(chain)
		if err != nil {
			t.Fatal(err)
		}
		got := output(w)
		if !strings.Contains(got, "__dynamic_buffer_offsets2._0") {
			t.Errorf("expected buffer offset reference in output, got %q", got)
		}
	})
}

// TestFillAccessChainWithDynamicOffset verifies that fillAccessChain pushes
// a subAccessBufferOffset when the global variable has a dynamic_storage_buffer_offsets_index.
func TestFillAccessChainWithDynamicOffset(t *testing.T) {
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	arrayType := ir.Type{Name: "array<u32, 1>", Inner: ir.ArrayType{
		Base:   0,
		Size:   ir.ArraySize{Constant: ptrU32(1)},
		Stride: 16,
	}}
	ptrType := ir.Type{Name: "ptr<storage, array<u32, 1>>", Inner: ir.PointerType{
		Base:  1,
		Space: ir.SpaceStorage,
	}}

	module := &ir.Module{
		Types: []ir.Type{u32Type, arrayType, ptrType},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "buf",
				Space:   ir.SpaceStorage,
				Type:    1,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 3},
			},
		},
	}

	dynIdx := uint32(0)
	w := newTestWriter(module, nil, nil)
	w.options.BindingMap = map[ResourceBinding]BindTarget{
		{Group: 0, Binding: 3}: {
			Register:                         2,
			Space:                            0,
			DynamicStorageBufferOffsetsIndex: &dynIdx,
		},
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprAccess{Base: 0, Index: 0}}, // would need index expr, simplified
		},
	}
	setCurrentFunction(w, fn)

	// Test fillAccessChain on the GlobalVariable expression directly
	gvHandle, err := w.fillAccessChain(0)
	if err != nil {
		t.Fatal(err)
	}
	if gvHandle != 0 {
		t.Errorf("expected global variable handle 0, got %d", gvHandle)
	}

	// Check that a buffer offset was pushed
	found := false
	for _, sa := range w.tempAccessChain {
		if sa.kind == subAccessBufferOffset && sa.bufferGroup == 0 && sa.bufferOffset == 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected subAccessBufferOffset in access chain for dynamic storage buffer binding")
	}
}

// TestNeedsRestrictIndexingPerBinding verifies that per-binding restrict_indexing
// is respected for Uniform address space.
func TestNeedsRestrictIndexingPerBinding(t *testing.T) {
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	arrayType := ir.Type{Name: "array<u32, 1>", Inner: ir.ArrayType{
		Base:   0,
		Size:   ir.ArraySize{Constant: ptrU32(1)},
		Stride: 16,
	}}

	module := &ir.Module{
		Types: []ir.Type{u32Type, arrayType},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "uniform_buf",
				Space:   ir.SpaceUniform,
				Type:    1,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 2},
			},
		},
	}

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		},
	}

	t.Run("restrict_indexing_false", func(t *testing.T) {
		w := newTestWriter(module, nil, nil)
		w.options.RestrictIndexing = true
		w.options.BindingMap = map[ResourceBinding]BindTarget{
			{Group: 0, Binding: 2}: {Register: 0, Space: 0, RestrictIndexing: false},
		}
		setCurrentFunction(w, fn)
		if w.needsRestrictIndexing(0) {
			t.Error("should return false when per-binding restrict_indexing is false")
		}
	})

	t.Run("restrict_indexing_true", func(t *testing.T) {
		w := newTestWriter(module, nil, nil)
		w.options.RestrictIndexing = true
		w.options.BindingMap = map[ResourceBinding]BindTarget{
			{Group: 0, Binding: 2}: {Register: 0, Space: 0, RestrictIndexing: true},
		}
		setCurrentFunction(w, fn)
		if !w.needsRestrictIndexing(0) {
			t.Error("should return true when per-binding restrict_indexing is true")
		}
	})
}

// =============================================================================
// TestComputeSubAccess_ValuePointerType — stride from scalar width
// =============================================================================

func TestComputeSubAccess_ValuePointerType(t *testing.T) {
	// ValuePointerType should use scalar width as stride, matching Rust's
	// TypeInner::ValuePointer { scalar, .. } => Parent::Array { stride: scalar.width }
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ValuePointerType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Space: ir.SpaceStorage}}, // 0: ptr to f32 (width=4)
			{Inner: ir.ValuePointerType{
				Size:   ptrVecSize(ir.Vec4),
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
				Space:  ir.SpaceStorage,
			}}, // 1: ptr to vec4<f32> (scalar width=4)
		},
	}

	vpScalarHandle := ir.TypeHandle(0)
	vpVecHandle := ir.TypeHandle(1)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // 0: base expr (scalar ptr)
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // 1: base expr (vector ptr)
			{Kind: ir.Literal{Value: ir.LiteralU32(3)}}, // 2: runtime index
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vpScalarHandle},
			{Handle: &vpVecHandle},
			{Handle: &vpScalarHandle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	t.Run("scalar_const_index", func(t *testing.T) {
		sa, err := w.computeSubAccess(0, false, 0, 2)
		if err != nil {
			t.Fatalf("computeSubAccess: %v", err)
		}
		if sa.kind != subAccessOffset {
			t.Errorf("expected subAccessOffset, got %d", sa.kind)
		}
		// stride=4, constIndex=2 => offset=8
		if sa.offset != 8 {
			t.Errorf("expected offset 8, got %d", sa.offset)
		}
	})

	t.Run("scalar_runtime_index", func(t *testing.T) {
		sa, err := w.computeSubAccess(0, true, 2, 0)
		if err != nil {
			t.Fatalf("computeSubAccess: %v", err)
		}
		if sa.kind != subAccessIndex {
			t.Errorf("expected subAccessIndex, got %d", sa.kind)
		}
		// stride = scalar width = 4
		if sa.stride != 4 {
			t.Errorf("expected stride 4, got %d", sa.stride)
		}
	})

	t.Run("vector_ptr_uses_scalar_width", func(t *testing.T) {
		sa, err := w.computeSubAccess(1, false, 0, 3)
		if err != nil {
			t.Fatalf("computeSubAccess: %v", err)
		}
		if sa.kind != subAccessOffset {
			t.Errorf("expected subAccessOffset, got %d", sa.kind)
		}
		// stride=4 (scalar width, not vector size), constIndex=3 => offset=12
		if sa.offset != 12 {
			t.Errorf("expected offset 12, got %d", sa.offset)
		}
	})
}

func ptrVecSize(s ir.VectorSize) *ir.VectorSize {
	return &s
}

// =============================================================================
// Helpers
// =============================================================================

func ptrU32(v uint32) *uint32 {
	return &v
}
