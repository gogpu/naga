// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"math"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// glslBuiltIn Tests
// =============================================================================

func TestGlslBuiltIn(t *testing.T) {
	tests := []struct {
		name     string
		builtin  ir.BuiltinValue
		isOutput bool
		want     string
	}{
		{"position_output", ir.BuiltinPosition, true, "gl_Position"},
		{"position_input", ir.BuiltinPosition, false, "gl_FragCoord"},
		{"vertex_index", ir.BuiltinVertexIndex, false, "uint(gl_VertexID)"},
		{"instance_index", ir.BuiltinInstanceIndex, false, "(uint(gl_InstanceID) + naga_vs_first_instance)"},
		{"front_facing", ir.BuiltinFrontFacing, false, "gl_FrontFacing"},
		{"frag_depth", ir.BuiltinFragDepth, true, "gl_FragDepth"},
		{"sample_index", ir.BuiltinSampleIndex, false, "gl_SampleID"},
		{"sample_mask", ir.BuiltinSampleMask, false, "gl_SampleMaskIn[0]"},
		{"local_invocation_id", ir.BuiltinLocalInvocationID, false, "gl_LocalInvocationID"},
		{"local_invocation_index", ir.BuiltinLocalInvocationIndex, false, "gl_LocalInvocationIndex"},
		{"global_invocation_id", ir.BuiltinGlobalInvocationID, false, "gl_GlobalInvocationID"},
		{"workgroup_id", ir.BuiltinWorkGroupID, false, "gl_WorkGroupID"},
		{"num_workgroups", ir.BuiltinNumWorkGroups, false, "gl_NumWorkGroups"},
		{"view_index", ir.BuiltinViewIndex, false, "gl_ViewID_OVR"},
		{"barycentric", ir.BuiltinBarycentric, false, "gl_BaryCoordEXT"},
		{"point_size", ir.BuiltinPointSize, true, "gl_PointSize"},
		{"primitive_index", ir.BuiltinPrimitiveIndex, false, "gl_PrimitiveID"},
		{"num_subgroups", ir.BuiltinNumSubgroups, false, "gl_NumSubgroups"},
		{"subgroup_id", ir.BuiltinSubgroupID, false, "gl_SubgroupID"},
		{"subgroup_size", ir.BuiltinSubgroupSize, false, "gl_SubgroupSize"},
		{"subgroup_invocation_id", ir.BuiltinSubgroupInvocationID, false, "gl_SubgroupInvocationID"},
		{"clip_distance", ir.BuiltinClipDistance, false, "gl_ClipDistance"},
		{"unknown_builtin", ir.BuiltinValue(255), false, "gl_UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := glslBuiltIn(tt.builtin, tt.isOutput)
			if got != tt.want {
				t.Errorf("glslBuiltIn(%v, %v) = %q, want %q",
					tt.builtin, tt.isOutput, got, tt.want)
			}
		})
	}
}

// =============================================================================
// glslStorageFormat Tests
// =============================================================================

func TestGlslStorageFormat(t *testing.T) {
	tests := []struct {
		format ir.StorageFormat
		want   string
	}{
		{ir.StorageFormatRgba8Unorm, "rgba8"},
		{ir.StorageFormatRgba8Snorm, "rgba8_snorm"},
		{ir.StorageFormatRgba8Uint, "rgba8ui"},
		{ir.StorageFormatRgba8Sint, "rgba8i"},
		{ir.StorageFormatRgba16Uint, "rgba16ui"},
		{ir.StorageFormatRgba16Sint, "rgba16i"},
		{ir.StorageFormatRgba16Float, "rgba16f"},
		{ir.StorageFormatRgba32Uint, "rgba32ui"},
		{ir.StorageFormatRgba32Sint, "rgba32i"},
		{ir.StorageFormatRgba32Float, "rgba32f"},
		{ir.StorageFormatR32Uint, "r32ui"},
		{ir.StorageFormatR32Sint, "r32i"},
		{ir.StorageFormatR32Float, "r32f"},
		{ir.StorageFormat(255), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := glslStorageFormat(tt.format)
			if got != tt.want {
				t.Errorf("glslStorageFormat(%v) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

// =============================================================================
// glslStorageAccess Tests
// =============================================================================

func TestGlslStorageAccess(t *testing.T) {
	tests := []struct {
		access ir.StorageAccess
		want   string
	}{
		{ir.StorageAccessRead, "readonly "},
		{ir.StorageAccessWrite, "writeonly "},
		{ir.StorageAccessReadWrite, ""},
		{ir.StorageAccess(255), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := glslStorageAccess(tt.access)
			if got != tt.want {
				t.Errorf("glslStorageAccess(%v) = %q, want %q", tt.access, got, tt.want)
			}
		})
	}
}

// =============================================================================
// scalarZeroInit Tests
// =============================================================================

func TestScalarZeroInit(t *testing.T) {
	tests := []struct {
		kind ir.ScalarKind
		want string
	}{
		{ir.ScalarBool, "false"},
		{ir.ScalarSint, "0"},
		{ir.ScalarUint, "0u"},
		{ir.ScalarFloat, "0.0"},
		{ir.ScalarKind(99), "0"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := scalarZeroInit(tt.kind)
			if got != tt.want {
				t.Errorf("scalarZeroInit(%v) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}

// =============================================================================
// scalarVecPrefix Tests (comprehensive)
// =============================================================================

func TestScalarVecPrefix_AllKinds(t *testing.T) {
	tests := []struct {
		kind ir.ScalarKind
		want string
	}{
		{ir.ScalarBool, "b"},
		{ir.ScalarSint, "i"},
		{ir.ScalarUint, "u"},
		{ir.ScalarFloat, ""},
		{ir.ScalarAbstractInt, ""}, // abstract types fall through to default ""
		{ir.ScalarAbstractFloat, ""},
	}

	for _, tt := range tests {
		got := scalarVecPrefix(tt.kind)
		if got != tt.want {
			t.Errorf("scalarVecPrefix(%v) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// =============================================================================
// formatFloat Edge Cases
// =============================================================================

func TestFormatFloat_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    float32
		check    func(string) bool
		describe string
	}{
		{
			"positive_infinity",
			float32(math.Inf(1)),
			func(s string) bool { return strings.Contains(s, "Inf") || strings.Contains(s, "inf") || s == "+Inf" },
			"should contain Inf",
		},
		{
			"negative_infinity",
			float32(math.Inf(-1)),
			func(s string) bool { return strings.Contains(s, "Inf") || strings.Contains(s, "inf") },
			"should contain Inf",
		},
		{
			"zero",
			0.0,
			func(s string) bool { return s == "0.0" },
			"should be 0.0",
		},
		{
			"negative_zero",
			float32(math.Copysign(0, -1)),
			func(s string) bool { return strings.Contains(s, "0") },
			"should contain 0",
		},
		{
			"integer_value",
			42.0,
			func(s string) bool { return strings.Contains(s, ".") },
			"should have decimal point",
		},
		{
			"no_plus_in_exponent",
			1.5e10,
			func(s string) bool { return !strings.Contains(s, "e+") },
			"should not have e+ in exponent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat(tt.input)
			if !tt.check(got) {
				t.Errorf("formatFloat(%v) = %q, %s", tt.input, got, tt.describe)
			}
		})
	}
}

func TestFormatFloat64_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		check func(string) bool
		desc  string
	}{
		{
			"zero",
			0.0,
			func(s string) bool { return s == "0.0LF" },
			"should be 0.0LF",
		},
		{
			"one",
			1.0,
			func(s string) bool { return strings.HasSuffix(s, "LF") && strings.Contains(s, "1") },
			"should end with LF",
		},
		{
			"small_fraction",
			0.001,
			func(s string) bool { return strings.HasSuffix(s, "LF") },
			"should end with LF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat64(tt.input)
			if !tt.check(got) {
				t.Errorf("formatFloat64(%v) = %q, %s", tt.input, got, tt.desc)
			}
		})
	}
}

// =============================================================================
// isDynamicallySized Tests
// =============================================================================

func TestIsDynamicallySized(t *testing.T) {
	fixedSize := uint32(10)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                 // [0] scalar — not dynamic
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &fixedSize}, Stride: 4}},    // [1] fixed array
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}, Stride: 4}},           // [2] dynamic array
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "a", Type: 2, Offset: 0}}}},    // [3] struct with dynamic last member
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "b", Type: 0, Offset: 0}}}},    // [4] struct with scalar last member
			{Inner: ir.StructType{Members: []ir.StructMember{}}},                                   // [5] empty struct
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [6] vector
		},
	}

	tests := []struct {
		name       string
		typeHandle ir.TypeHandle
		want       bool
	}{
		{"scalar", 0, false},
		{"fixed_array", 1, false},
		{"dynamic_array", 2, true},
		{"struct_with_dynamic_member", 3, true},
		{"struct_with_scalar_member", 4, false},
		{"empty_struct", 5, false},
		{"vector", 6, false},
		{"out_of_range", 99, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter(module, &Options{LangVersion: Version330})
			got := w.isDynamicallySized(tt.typeHandle)
			if got != tt.want {
				t.Errorf("isDynamicallySized(%d) = %v, want %v", tt.typeHandle, got, tt.want)
			}
		})
	}
}

// =============================================================================
// getTypeName Tests
// =============================================================================

func TestGetTypeName(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	t.Run("registered_name", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.typeNames[0] = "float"

		got := w.getTypeName(0)
		if got != "float" {
			t.Errorf("getTypeName(0) = %q, want %q", got, "float")
		}
	})

	t.Run("out_of_range", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		got := w.getTypeName(99)
		if !strings.HasPrefix(got, "type_") {
			t.Errorf("getTypeName(99) = %q, expected fallback type_99", got)
		}
	})
}

// =============================================================================
// Version Helper Tests (pack/unpack/fma support)
// =============================================================================

func TestSupportsPack2x16snorm(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 30}, false},          // Desktop 330
		{Version{Major: 4, Minor: 20}, true},           // Desktop 420
		{Version{Major: 3, Minor: 0, ES: true}, true},  // ES 300
		{Version{Major: 2, Minor: 0, ES: true}, false}, // ES 200
	}

	for _, tt := range tests {
		got := tt.version.supportsPack2x16snorm()
		if got != tt.want {
			t.Errorf("Version(%v).supportsPack2x16snorm() = %v, want %v",
				tt.version, got, tt.want)
		}
	}
}

func TestSupportsPack2x16unorm(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 30}, false},          // Desktop 330
		{Version{Major: 4, Minor: 0}, true},            // Desktop 400
		{Version{Major: 3, Minor: 0, ES: true}, true},  // ES 300
		{Version{Major: 2, Minor: 0, ES: true}, false}, // ES 200
	}

	for _, tt := range tests {
		got := tt.version.supportsPack2x16unorm()
		if got != tt.want {
			t.Errorf("Version(%v).supportsPack2x16unorm() = %v, want %v",
				tt.version, got, tt.want)
		}
	}
}

// =============================================================================
// isWebGL Tests
// =============================================================================

func TestIsWebGL(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 0, ES: true}, true},   // ES 300 = WebGL 2.0
		{Version{Major: 3, Minor: 10, ES: true}, false}, // ES 310, not WebGL
		{Version{Major: 3, Minor: 0, ES: false}, false}, // Desktop, not WebGL
		{Version{Major: 4, Minor: 0, ES: true}, false},  // ES 400, not WebGL
	}

	for _, tt := range tests {
		got := tt.version.isWebGL()
		if got != tt.want {
			t.Errorf("Version(%v).isWebGL() = %v, want %v", tt.version, got, tt.want)
		}
	}
}

// =============================================================================
// formatLiteral Tests (comprehensive)
// =============================================================================

func TestFormatLiteral_AllTypes(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	tests := []struct {
		name     string
		literal  ir.Literal
		contains string
	}{
		{"bool_true", ir.Literal{Value: ir.LiteralBool(true)}, "true"},
		{"bool_false", ir.Literal{Value: ir.LiteralBool(false)}, "false"},
		{"i32_positive", ir.Literal{Value: ir.LiteralI32(42)}, "42"},
		{"i32_negative", ir.Literal{Value: ir.LiteralI32(-1)}, "-1"},
		{"i32_zero", ir.Literal{Value: ir.LiteralI32(0)}, "0"},
		{"u32", ir.Literal{Value: ir.LiteralU32(100)}, "100u"},
		{"f32_decimal", ir.Literal{Value: ir.LiteralF32(3.14)}, "3.14"},
		{"f32_zero", ir.Literal{Value: ir.LiteralF32(0.0)}, "0.0"},
		{"f64", ir.Literal{Value: ir.LiteralF64(2.718)}, "LF"},
		{"i64", ir.Literal{Value: ir.LiteralI64(999)}, "999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.formatLiteral(tt.literal)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("formatLiteral(%v) = %q, should contain %q", tt.literal.Value, got, tt.contains)
			}
		})
	}
}

// =============================================================================
// zeroInitValue Tests (extended)
// =============================================================================

func TestZeroInitValue_Extended(t *testing.T) {
	mat2x2 := ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	size2 := uint32(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, // [0] f64
			{Inner: mat2x2}, // [1] mat2x2
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &size2}, Stride: 8}},     // [2] array<f64, 2>
			{Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},        // [3] atomic<u32>
			{Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},        // [4] atomic<i32>
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "a", Type: 0, Offset: 0}}}}, // [5] struct
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.typeNames[0] = "double"
	w.typeNames[1] = "mat2x2"
	w.typeNames[2] = "double[2]"
	w.typeNames[3] = "uint"
	w.typeNames[4] = "int"
	w.typeNames[5] = "MyStruct"

	tests := []struct {
		typeHandle ir.TypeHandle
		want       string
	}{
		{0, "0.0"},           // f64
		{1, "mat2x2(0.0)"},   // matrix
		{3, "0u"},            // atomic<u32>
		{4, "0"},             // atomic<i32>
		{5, "MyStruct(0.0)"}, // struct
	}

	for _, tt := range tests {
		got := w.zeroInitValue(tt.typeHandle)
		if got != tt.want {
			t.Errorf("zeroInitValue(%d) = %q, want %q", tt.typeHandle, got, tt.want)
		}
	}
}

// =============================================================================
// writeConstExpr Tests — ZeroValue
// =============================================================================

func TestWriteConstExpr_ZeroValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                 // [0]
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [1]
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 0}}, // [0] zero float
			{Kind: ir.ExprZeroValue{Type: 1}}, // [1] zero vec4
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.writeConstExpr(ir.ExpressionHandle(0))
	if got != "0.0" {
		t.Errorf("writeConstExpr(ZeroValue f32) = %q, want %q", got, "0.0")
	}

	got2 := w.writeConstExpr(ir.ExpressionHandle(1))
	if got2 != "vec4(0.0)" {
		t.Errorf("writeConstExpr(ZeroValue vec4) = %q, want %q", got2, "vec4(0.0)")
	}
}

// =============================================================================
// writeScalarValue Tests
// =============================================================================

func TestWriteScalarValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},  // [0]
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},  // [1]
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // [2]
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // [3]
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	tests := []struct {
		name       string
		value      ir.ScalarValue
		typeHandle ir.TypeHandle
		contains   string
	}{
		{"bool_true", ir.ScalarValue{Kind: ir.ScalarBool, Bits: 1}, 0, "true"},
		{"bool_false", ir.ScalarValue{Kind: ir.ScalarBool, Bits: 0}, 0, "false"},
		{"int", ir.ScalarValue{Kind: ir.ScalarSint, Bits: 42}, 1, "42"},
		{"uint", ir.ScalarValue{Kind: ir.ScalarUint, Bits: 100}, 2, "100u"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.writeScalarValue(tt.value, tt.typeHandle)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("writeScalarValue() = %q, should contain %q", got, tt.contains)
			}
		})
	}
}

// =============================================================================
// Namer edge case Tests
// =============================================================================

func TestNamer_EmptyString(t *testing.T) {
	n := newNamer()
	name := n.call("")
	if name == "" {
		t.Error("namer should not return empty string for empty input")
	}
}

func TestNamer_SpecialChars(t *testing.T) {
	n := newNamer()
	// Names with special chars get sanitized
	name := n.call("type::inner<f32>")
	if strings.ContainsAny(name, "<>:") {
		t.Errorf("namer should sanitize special chars, got %q", name)
	}
}

// =============================================================================
// WriterFlags Tests
// =============================================================================

func TestWriterFlags(t *testing.T) {
	// Ensure flag values are distinct
	flags := []WriterFlags{
		WriterFlagNone,
		WriterFlagExplicitTypes,
		WriterFlagDebugInfo,
		WriterFlagMinify,
		WriterFlagAdjustCoordinateSpace,
		WriterFlagForcePointSize,
		WriterFlagTextureShadowLod,
	}

	for i, f1 := range flags {
		for j, f2 := range flags {
			if i != j && f1 == f2 && f1 != WriterFlagNone {
				t.Errorf("flags %d and %d have same value: %d", i, j, f1)
			}
		}
	}

	// Test combining flags
	combined := WriterFlagAdjustCoordinateSpace | WriterFlagForcePointSize
	if combined&WriterFlagAdjustCoordinateSpace == 0 {
		t.Error("combined flags should include AdjustCoordinateSpace")
	}
	if combined&WriterFlagForcePointSize == 0 {
		t.Error("combined flags should include ForcePointSize")
	}
	if combined&WriterFlagDebugInfo != 0 {
		t.Error("combined flags should not include DebugInfo")
	}
}

// =============================================================================
// BoundsCheckPolicy Tests
// =============================================================================

func TestBoundsCheckPolicy_Values(t *testing.T) {
	// Ensure policies have distinct values
	if BoundsCheckUnchecked == BoundsCheckRestrict {
		t.Error("Unchecked and Restrict should be different")
	}
	if BoundsCheckRestrict == BoundsCheckReadZeroSkipWrite {
		t.Error("Restrict and ReadZeroSkipWrite should be different")
	}
}

// =============================================================================
// storageLayoutPrefix Tests
// =============================================================================

func TestStorageLayoutPrefix(t *testing.T) {
	module := &ir.Module{}

	t.Run("std430_no_binding", func(t *testing.T) {
		opts := Options{LangVersion: Version330}
		w := newWriter(module, &opts)
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
		}
		got := w.storageLayoutPrefix(gv)
		if got != "layout(std430) " {
			t.Errorf("got %q, want %q", got, "layout(std430) ")
		}
	})

	t.Run("std430_with_binding_map", func(t *testing.T) {
		opts := Options{
			LangVersion: Version{Major: 4, Minor: 50},
			BindingMap: map[BindingMapKey]uint8{
				{Group: 0, Binding: 5}: 12,
			},
		}
		w := newWriter(module, &opts)
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 5},
		}
		got := w.storageLayoutPrefix(gv)
		if got != "layout(std430, binding = 12) " {
			t.Errorf("got %q, want %q", got, "layout(std430, binding = 12) ")
		}
	})
}

// =============================================================================
// getSelectedEntryPoint Tests
// =============================================================================

func TestGetSelectedEntryPoint(t *testing.T) {
	t.Run("no_entry_points", func(t *testing.T) {
		module := &ir.Module{}
		w := newWriter(module, &Options{LangVersion: Version330})
		ep := w.getSelectedEntryPoint()
		if ep != nil {
			t.Error("expected nil for module with no entry points")
		}
	})

	t.Run("first_entry_point_default", func(t *testing.T) {
		module := &ir.Module{
			EntryPoints: []ir.EntryPoint{
				{Name: "vert_main", Stage: ir.StageVertex, Function: ir.Function{}},
				{Name: "frag_main", Stage: ir.StageFragment, Function: ir.Function{}},
			},
		}
		w := newWriter(module, &Options{LangVersion: Version330})
		ep := w.getSelectedEntryPoint()
		if ep == nil || ep.Name != "vert_main" {
			t.Errorf("expected first entry point, got %v", ep)
		}
	})

	t.Run("named_entry_point", func(t *testing.T) {
		module := &ir.Module{
			EntryPoints: []ir.EntryPoint{
				{Name: "vert_main", Stage: ir.StageVertex, Function: ir.Function{}},
				{Name: "frag_main", Stage: ir.StageFragment, Function: ir.Function{}},
			},
		}
		w := newWriter(module, &Options{LangVersion: Version330, EntryPoint: "frag_main"})
		ep := w.getSelectedEntryPoint()
		if ep == nil || ep.Name != "frag_main" {
			t.Errorf("expected frag_main, got %v", ep)
		}
	})
}

// =============================================================================
// wrapUintCast Tests
// =============================================================================

func TestWrapUintCast(t *testing.T) {
	scalarU32 := ir.TypeHandle(0)
	scalarI32 := ir.TypeHandle(1)
	vecU32 := ir.TypeHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                 // [0]
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                 // [1]
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}, // [2]
		},
	}

	t.Run("unsigned_scalar", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &scalarU32},
			},
		}
		got := w.wrapUintCast("findLSB(x)", 0)
		if got != "uint(findLSB(x))" {
			t.Errorf("got %q, want %q", got, "uint(findLSB(x))")
		}
	})

	t.Run("signed_scalar", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &scalarI32},
			},
		}
		got := w.wrapUintCast("findLSB(x)", 0)
		if got != "findLSB(x)" {
			t.Errorf("got %q, want %q — should not wrap signed", got, "findLSB(x)")
		}
	})

	t.Run("unsigned_vector", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &vecU32},
			},
		}
		got := w.wrapUintCast("findMSB(v)", 0)
		if got != "uvec3(findMSB(v))" {
			t.Errorf("got %q, want %q", got, "uvec3(findMSB(v))")
		}
	})
}

// =============================================================================
// isUnsignedExpr Tests
// =============================================================================

func TestIsUnsignedExpr(t *testing.T) {
	scalarU32 := ir.TypeHandle(0)
	scalarI32 := ir.TypeHandle(1)
	vecU32 := ir.TypeHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                 // [0]
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                 // [1]
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}, // [2]
		},
	}

	t.Run("scalar_uint_via_handle", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{{Handle: &scalarU32}},
		}
		size, ok := w.isUnsignedExpr(0)
		if !ok || size != 0 {
			t.Errorf("expected (0, true) for scalar uint, got (%d, %v)", size, ok)
		}
	})

	t.Run("scalar_sint_via_handle", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{{Handle: &scalarI32}},
		}
		_, ok := w.isUnsignedExpr(0)
		if ok {
			t.Error("expected false for signed int")
		}
	})

	t.Run("vector_uint_via_handle", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{{Handle: &vecU32}},
		}
		size, ok := w.isUnsignedExpr(0)
		if !ok || size != 2 {
			t.Errorf("expected (2, true) for uvec2, got (%d, %v)", size, ok)
		}
	})

	t.Run("via_value_scalar", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{
				{Value: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			},
		}
		size, ok := w.isUnsignedExpr(0)
		if !ok || size != 0 {
			t.Errorf("expected (0, true) for value-based uint, got (%d, %v)", size, ok)
		}
	})

	t.Run("via_value_vector", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			ExpressionTypes: []ir.TypeResolution{
				{Value: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			},
		}
		size, ok := w.isUnsignedExpr(0)
		if !ok || size != 4 {
			t.Errorf("expected (4, true) for value-based uvec4, got (%d, %v)", size, ok)
		}
	})

	t.Run("nil_function", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = nil
		_, ok := w.isUnsignedExpr(0)
		if ok {
			t.Error("expected false with nil function")
		}
	})
}
