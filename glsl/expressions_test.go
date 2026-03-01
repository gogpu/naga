// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Helpers for GLSL tests
// =============================================================================

func compileGLSL(t *testing.T, module *ir.Module) string {
	t.Helper()
	opts := DefaultOptions()
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("GLSL Compile failed: %v", err)
	}
	return result
}

func mustContainGLSL(t *testing.T, source, expected string) {
	t.Helper()
	if !strings.Contains(source, expected) {
		t.Errorf("Expected output to contain %q.\nOutput:\n%s", expected, source)
	}
}

// =============================================================================
// Test: GLSL scalar types
// =============================================================================

func TestGLSL_ScalarTypes(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},
		{"int32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{"int64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "int64_t"},
		{"uint32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{"uint64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "uint64_t"},
		{"float16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "float16_t"},
		{"float32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{"float64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "double"},
		{"int8_as_int", ir.ScalarType{Kind: ir.ScalarSint, Width: 1}, "int"},
		{"int16_as_int", ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, "int"},
		{"uint8_as_uint", ir.ScalarType{Kind: ir.ScalarUint, Width: 1}, "uint"},
		{"uint16_as_uint", ir.ScalarType{Kind: ir.ScalarUint, Width: 2}, "uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarToGLSL(tt.scalar)
			if got != tt.want {
				t.Errorf("scalarToGLSL(%+v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: GLSL vector types
// =============================================================================

func TestGLSL_VectorTypes(t *testing.T) {
	tests := []struct {
		name   string
		vector ir.VectorType
		want   string
	}{
		{"vec2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec2"},
		{"vec3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec3"},
		{"vec4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec4"},
		{"ivec2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "ivec2"},
		{"ivec3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "ivec3"},
		{"ivec4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "ivec4"},
		{"uvec2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uvec2"},
		{"uvec3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uvec3"},
		{"uvec4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uvec4"},
		{"bvec2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bvec2"},
		{"bvec3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bvec3"},
		{"bvec4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bvec4"},
		{"dvec2", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dvec2"},
		{"dvec3", ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dvec3"},
		{"dvec4", ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dvec4"},
		{"invalid_size_clamps", ir.VectorType{Size: 1, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec4"},
		{"default_kind", ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarKind(99), Width: 4}}, "vec2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vectorToGLSL(tt.vector)
			if got != tt.want {
				t.Errorf("vectorToGLSL(%+v) = %q, want %q", tt.vector, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: GLSL matrix types
// =============================================================================

func TestGLSL_MatrixTypes(t *testing.T) {
	tests := []struct {
		name   string
		matrix ir.MatrixType
		want   string
	}{
		{"mat2", ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2"},
		{"mat3", ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat3"},
		{"mat4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat4"},
		{"mat2x3", ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2x3"},
		{"mat3x4", ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat3x4"},
		{"mat4x2", ir.MatrixType{Columns: 4, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat4x2"},
		{"dmat2", ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat2"},
		{"dmat3", ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat3"},
		{"dmat4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat4"},
		{"dmat2x3", ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat2x3"},
		{"invalid_cols_clamp", ir.MatrixType{Columns: 1, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat4x2"},
		{"invalid_rows_clamp", ir.MatrixType{Columns: 2, Rows: 5, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2x4"},
		{"non_float_default", ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "mat3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matrixToGLSL(tt.matrix)
			if got != tt.want {
				t.Errorf("matrixToGLSL(%+v) = %q, want %q", tt.matrix, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: GLSL image types
// =============================================================================

func TestGLSL_ImageTypes(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	tests := []struct {
		name string
		img  ir.ImageType
		want string
	}{
		{"sampler1D", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled}, "sampler1D"},
		{"sampler2D", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}, "sampler2D"},
		{"sampler3D", ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled}, "sampler3D"},
		{"samplerCube", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled}, "samplerCube"},
		{"sampler2DShadow", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth}, "sampler2DShadow"},
		{"samplerCubeShadow", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassDepth}, "samplerCubeShadow"},
		{"sampler1DArray", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled, Arrayed: true}, "sampler1DArray"},
		{"sampler2DArray", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Arrayed: true}, "sampler2DArray"},
		{"sampler2DMS", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Multisampled: true}, "sampler2DMS"},
		{"sampler2DMSArray", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Multisampled: true, Arrayed: true}, "sampler2DMSArray"},
		{"samplerCubeArray", ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, Arrayed: true}, "samplerCubeArray"},
		{"image2D", ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage}, "image2D"},
		{"depth1D", ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassDepth}, "sampler1D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.imageToGLSL(tt.img)
			if got != tt.want {
				t.Errorf("imageToGLSL(%+v) = %q, want %q", tt.img, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: GLSL atomic types
// =============================================================================

func TestGLSL_AtomicTypes(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	tests := []struct {
		name   string
		atomic ir.AtomicType
		want   string
	}{
		{"sint", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int"},
		{"uint", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint"},
		{"default", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.atomicToGLSL(tt.atomic)
			if got != tt.want {
				t.Errorf("atomicToGLSL(%+v) = %q, want %q", tt.atomic, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: structsEqual helper
// =============================================================================

func TestGLSL_StructsEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b ir.StructType
		want bool
	}{
		{
			"equal",
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}},
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}},
			true,
		},
		{
			"diff_name",
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}},
			ir.StructType{Members: []ir.StructMember{{Name: "y", Type: 0}}},
			false,
		},
		{
			"diff_type",
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}},
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 1}}},
			false,
		},
		{
			"diff_len",
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}},
			ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}, {Name: "y", Type: 0}}},
			false,
		},
		{
			"both_empty",
			ir.StructType{Members: nil},
			ir.StructType{Members: nil},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := structsEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("structsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: scalarKindToGLSL
// =============================================================================

func TestGLSL_ScalarKindToGLSL(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	tests := []struct {
		kind ir.ScalarKind
		want string
	}{
		{ir.ScalarBool, "bool"},
		{ir.ScalarSint, "int"},
		{ir.ScalarUint, "uint"},
		{ir.ScalarFloat, "float"},
		{ir.ScalarKind(99), "int"}, // default
	}

	for _, tt := range tests {
		got := w.scalarKindToGLSL(tt.kind)
		if got != tt.want {
			t.Errorf("scalarKindToGLSL(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// =============================================================================
// Test: getInverseScalarKind
// =============================================================================

func TestGLSL_GetInverseScalarKind(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	tests := []struct {
		kind ir.ScalarKind
		want string
	}{
		{ir.ScalarSint, "float"},
		{ir.ScalarUint, "float"},
		{ir.ScalarFloat, "int"},
		{ir.ScalarBool, "int"}, // default
	}

	for _, tt := range tests {
		got := w.getInverseScalarKind(tt.kind)
		if got != tt.want {
			t.Errorf("getInverseScalarKind(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// =============================================================================
// Test: GLSL derivative expressions
// =============================================================================

func TestGLSL_DerivativeExpressions(t *testing.T) {
	tests := []struct {
		name    string
		axis    ir.DerivativeAxis
		control ir.DerivativeControl
		want    string
	}{
		{"dFdxFine", ir.DerivativeX, ir.DerivativeFine, "dFdxFine("},
		{"dFdxCoarse", ir.DerivativeX, ir.DerivativeCoarse, "dFdxCoarse("},
		{"dFdyFine", ir.DerivativeY, ir.DerivativeFine, "dFdyFine("},
		{"dFdyCoarse", ir.DerivativeY, ir.DerivativeCoarse, "dFdyCoarse("},
		{"fwidthFine", ir.DerivativeWidth, ir.DerivativeFine, "fwidthFine("},
		{"fwidthCoarse", ir.DerivativeWidth, ir.DerivativeCoarse, "fwidthCoarse("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.ExprDerivative{Axis: tt.axis, Control: tt.control, Expr: 0}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL statement generation
// =============================================================================

func TestGLSL_Statements(t *testing.T) {
	t.Run("break", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{},
			Functions: []ir.Function{{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{Body: []ir.Statement{{Kind: ir.StmtBreak{}}}}},
				},
			}},
		}
		result := compileGLSL(t, module)
		mustContainGLSL(t, result, "break;")
	})

	t.Run("continue", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{},
			Functions: []ir.Function{{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{Body: []ir.Statement{{Kind: ir.StmtContinue{}}}}},
				},
			}},
		}
		result := compileGLSL(t, module)
		mustContainGLSL(t, result, "continue;")
	})

	t.Run("discard", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{},
			Functions: []ir.Function{{
				Name: "test_fn",
				Body: []ir.Statement{{Kind: ir.StmtKill{}}},
			}},
		}
		result := compileGLSL(t, module)
		mustContainGLSL(t, result, "discard;")
	})

	t.Run("return_void", func(t *testing.T) {
		module := &ir.Module{
			Types: []ir.Type{},
			Functions: []ir.Function{{
				Name: "test_fn",
				Body: []ir.Statement{{Kind: ir.StmtReturn{}}},
			}},
		}
		result := compileGLSL(t, module)
		mustContainGLSL(t, result, "return;")
	})
}

// =============================================================================
// Test: GLSL loop with break-if
// =============================================================================

func TestGLSL_LoopWithBreakIf(t *testing.T) {
	tBool := ir.TypeHandle(0)
	breakIfExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtLoop{
					Body:    []ir.Statement{},
					BreakIf: &breakIfExpr,
				}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "for (;;) {")
	mustContainGLSL(t, result, "if (")
	mustContainGLSL(t, result, "{ break; }")
}

// =============================================================================
// Test: GLSL switch with different value types
// =============================================================================

func TestGLSL_SwitchStatements(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tI32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtSwitch{
					Selector: 0,
					Cases: []ir.SwitchCase{
						{Value: ir.SwitchValueI32(1), Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
						{Value: ir.SwitchValueU32(2), Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
						{Value: ir.SwitchValueDefault{}, Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
					},
				}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "switch (")
	mustContainGLSL(t, result, "case 1:")
	mustContainGLSL(t, result, "case 2u:")
	mustContainGLSL(t, result, "default:")
}

// =============================================================================
// Test: GLSL barrier statements
// =============================================================================

func TestGLSL_BarrierStatements(t *testing.T) {
	tests := []struct {
		name  string
		flags ir.BarrierFlags
		want  string
	}{
		{"workgroup", ir.BarrierWorkGroup, "barrier()"},
		{"storage", ir.BarrierStorage, "memoryBarrierBuffer()"},
		{"texture", ir.BarrierTexture, "memoryBarrierImage()"},
		{"pure_exec", 0, "barrier()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.LangVersion = Version{Major: 4, Minor: 30}

			module := &ir.Module{
				Types: []ir.Type{},
				Functions: []ir.Function{{
					Name: "test_fn",
					Body: []ir.Statement{
						{Kind: ir.StmtBarrier{Flags: tt.flags}},
					},
				}},
			}
			result, _, err := Compile(module, opts)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL barrier version requirement
// =============================================================================

func TestGLSL_BarrierVersionRequirement(t *testing.T) {
	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 3, Minor: 0}

	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{{
			Name: "test_fn",
			Body: []ir.Statement{
				{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}},
			},
		}},
	}
	_, _, err := Compile(module, opts)
	if err == nil {
		t.Error("Expected error for barrier with GLSL 3.0, got nil")
	}
}

// =============================================================================
// Test: GLSL ray query statement
// =============================================================================

func TestGLSL_RayQueryUnsupported(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.ExprZeroValue{Type: tI32}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tI32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtRayQuery{
					Query: 0,
					Fun:   ir.RayQueryTerminate{},
				}},
			},
		}},
	}
	_, _, err := Compile(module, DefaultOptions())
	if err == nil {
		t.Error("Expected error for ray query in GLSL, got nil")
	}
}

// =============================================================================
// Test: joinStrings helper
// =============================================================================

func TestGLSL_JoinStrings(t *testing.T) {
	tests := []struct {
		name string
		strs []string
		sep  string
		want string
	}{
		{"empty", nil, ", ", ""},
		{"single", []string{"a"}, ", ", "a"},
		{"multiple", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"no_sep", []string{"x", "y"}, "", "xy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinStrings(tt.strs, tt.sep)
			if got != tt.want {
				t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.strs, tt.sep, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: GLSL literal expressions
// =============================================================================

func TestGLSL_Literals(t *testing.T) {
	tests := []struct {
		name    string
		literal ir.LiteralValue
		want    string
	}{
		{"bool_true", ir.LiteralBool(true), "true"},
		{"bool_false", ir.LiteralBool(false), "false"},
		{"i32", ir.LiteralI32(42), "42"},
		{"i32_neg", ir.LiteralI32(-7), "-7"},
		{"u32", ir.LiteralU32(100), "100u"},
		{"f32", ir.LiteralF32(3.5), "3.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(0)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: tt.literal}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL binary operators
// =============================================================================

func TestGLSL_BinaryOperators(t *testing.T) {
	tests := []struct {
		name string
		op   ir.BinaryOperator
		want string
	}{
		{"add", ir.BinaryAdd, "+"},
		{"sub", ir.BinarySubtract, "-"},
		{"mul", ir.BinaryMultiply, "*"},
		{"div", ir.BinaryDivide, "/"},
		{"mod", ir.BinaryModulo, "_naga_mod"},
		{"eq", ir.BinaryEqual, "=="},
		{"ne", ir.BinaryNotEqual, "!="},
		{"lt", ir.BinaryLess, "<"},
		{"le", ir.BinaryLessEqual, "<="},
		{"gt", ir.BinaryGreater, ">"},
		{"ge", ir.BinaryGreaterEqual, ">="},
		{"and", ir.BinaryAnd, "&"},
		{"xor", ir.BinaryExclusiveOr, "^"},
		{"or", ir.BinaryInclusiveOr, "|"},
		{"logical_and", ir.BinaryLogicalAnd, "&&"},
		{"logical_or", ir.BinaryLogicalOr, "||"},
		{"shift_left", ir.BinaryShiftLeft, "<<"},
		{"shift_right", ir.BinaryShiftRight, ">>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(2)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
							{Kind: ir.ExprBinary{Op: tt.op, Left: 0, Right: 1}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL unary operators
// =============================================================================

func TestGLSL_UnaryOperators(t *testing.T) {
	tests := []struct {
		name string
		op   ir.UnaryOperator
		want string
	}{
		{"negate", ir.UnaryNegate, "-("},
		{"not", ir.UnaryLogicalNot, "!("},
		{"bitwise_not", ir.UnaryBitwiseNot, "~("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
							{Kind: ir.ExprUnary{Op: tt.op, Expr: 0}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tF32},
							{Handle: &tF32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
							{Kind: ir.StmtReturn{Value: &retExpr}},
						},
					},
				},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL version directive in output
// =============================================================================

func TestGLSL_VersionDirective(t *testing.T) {
	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 3, Minor: 0, ES: true}

	module := &ir.Module{
		Types:     []ir.Type{},
		Functions: []ir.Function{},
	}
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	mustContainGLSL(t, result, "#version 300 es")
}

func TestGLSL_VersionDirective450(t *testing.T) {
	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 4, Minor: 50}

	module := &ir.Module{
		Types:     []ir.Type{},
		Functions: []ir.Function{},
	}
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	mustContainGLSL(t, result, "#version 450")
}

// =============================================================================
// Test: GLSL if statement
// =============================================================================

func TestGLSL_IfStatement(t *testing.T) {
	tBool := ir.TypeHandle(0)
	tF32 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    []ir.Statement{{Kind: ir.StmtReturn{Value: &retExpr}}},
					Reject:    nil,
				}},
				{Kind: ir.StmtReturn{}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "if (")
}

func TestGLSL_IfElseStatement(t *testing.T) {
	tBool := ir.TypeHandle(0)
	tF32 := ir.TypeHandle(1)
	expr1 := ir.ExpressionHandle(1)
	expr2 := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    []ir.Statement{{Kind: ir.StmtReturn{Value: &expr1}}},
					Reject:    []ir.Statement{{Kind: ir.StmtReturn{Value: &expr2}}},
				}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "if (")
	mustContainGLSL(t, result, "} else {")
}

// =============================================================================
// Test: GLSL store statement
// =============================================================================

func TestGLSL_StoreStatement(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "=")
}

// =============================================================================
// Test: GLSL math expressions
// =============================================================================

func TestGLSL_MathExpressions(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.MathFunction
		want string
	}{
		{"cos", ir.MathCos, "cos("},
		{"sin", ir.MathSin, "sin("},
		{"tan", ir.MathTan, "tan("},
		{"acos", ir.MathAcos, "acos("},
		{"asin", ir.MathAsin, "asin("},
		{"atan", ir.MathAtan, "atan("},
		{"cosh", ir.MathCosh, "cosh("},
		{"sinh", ir.MathSinh, "sinh("},
		{"tanh", ir.MathTanh, "tanh("},
		{"exp", ir.MathExp, "exp("},
		{"exp2", ir.MathExp2, "exp2("},
		{"log", ir.MathLog, "log("},
		{"log2", ir.MathLog2, "log2("},
		{"sqrt", ir.MathSqrt, "sqrt("},
		{"inversesqrt", ir.MathInverseSqrt, "inversesqrt("},
		{"abs", ir.MathAbs, "abs("},
		{"sign", ir.MathSign, "sign("},
		{"floor", ir.MathFloor, "floor("},
		{"ceil", ir.MathCeil, "ceil("},
		{"trunc", ir.MathTrunc, "trunc("},
		{"round", ir.MathRound, "round("},
		{"fract", ir.MathFract, "fract("},
		{"length", ir.MathLength, "length("},
		{"normalize", ir.MathNormalize, "normalize("},
		{"saturate", ir.MathSaturate, "clamp("},
		{"transpose", ir.MathTranspose, "transpose("},
		{"determinant", ir.MathDeterminant, "determinant("},
		{"inverse", ir.MathInverse, "inverse("},
		{"countOneBits", ir.MathCountOneBits, "bitCount("},
		{"reverseBits", ir.MathReverseBits, "bitfieldReverse("},
		{"firstLeadingBit", ir.MathFirstLeadingBit, "findMSB("},
		{"firstTrailingBit", ir.MathFirstTrailingBit, "findLSB("},
		{"countLeadingZeros", ir.MathCountLeadingZeros, "findMSB("},
		{"countTrailingZeros", ir.MathCountTrailingZeros, "findLSB("},
		{"radians", ir.MathRadians, "radians("},
		{"degrees", ir.MathDegrees, "degrees("},
		{"pack4x8snorm", ir.MathPack4x8snorm, "packSnorm4x8("},
		{"pack4x8unorm", ir.MathPack4x8unorm, "packUnorm4x8("},
		{"pack2x16snorm", ir.MathPack2x16snorm, "packSnorm2x16("},
		{"pack2x16unorm", ir.MathPack2x16unorm, "packUnorm2x16("},
		{"pack2x16float", ir.MathPack2x16float, "packHalf2x16("},
		{"unpack4x8snorm", ir.MathUnpack4x8snorm, "unpackSnorm4x8("},
		{"unpack4x8unorm", ir.MathUnpack4x8unorm, "unpackUnorm4x8("},
		{"unpack2x16snorm", ir.MathUnpack2x16snorm, "unpackSnorm2x16("},
		{"unpack2x16unorm", ir.MathUnpack2x16unorm, "unpackUnorm2x16("},
		{"unpack2x16float", ir.MathUnpack2x16float, "unpackHalf2x16("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprMath{Fun: tt.fun, Arg: 0}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tF32},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				}},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

func TestGLSL_MathTwoArgs(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.MathFunction
		want string
	}{
		{"min", ir.MathMin, "min("},
		{"max", ir.MathMax, "max("},
		{"pow", ir.MathPow, "pow("},
		{"step", ir.MathStep, "step("},
		{"distance", ir.MathDistance, "distance("},
		{"dot", ir.MathDot, "dot("},
		{"cross", ir.MathCross, "cross("},
		{"reflect", ir.MathReflect, "reflect("},
		{"atan2", ir.MathAtan2, "atan("},
		{"outer", ir.MathOuter, "outerProduct("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			arg1 := ir.ExpressionHandle(1)
			retExpr := ir.ExpressionHandle(2)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
						{Kind: ir.ExprMath{Fun: tt.fun, Arg: 0, Arg1: &arg1}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tF32},
						{Handle: &tF32},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				}},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

func TestGLSL_MathThreeArgs(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.MathFunction
		want string
	}{
		{"clamp", ir.MathClamp, "clamp("},
		{"mix", ir.MathMix, "mix("},
		{"smoothstep", ir.MathSmoothStep, "smoothstep("},
		{"fma", ir.MathFma, "fma("},
		{"faceforward", ir.MathFaceForward, "faceforward("},
		{"refract", ir.MathRefract, "refract("},
		{"extractBits", ir.MathExtractBits, "bitfieldExtract("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			arg1 := ir.ExpressionHandle(1)
			arg2 := ir.ExpressionHandle(2)
			retExpr := ir.ExpressionHandle(3)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				},
				Functions: []ir.Function{{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
						{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},
						{Kind: ir.ExprMath{Fun: tt.fun, Arg: 0, Arg1: &arg1, Arg2: &arg2}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tF32},
						{Handle: &tF32},
						{Handle: &tF32},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				}},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL select expression
// =============================================================================

func TestGLSL_SelectExpression(t *testing.T) {
	tBool := ir.TypeHandle(0)
	tF32 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
				{Kind: ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
				{Handle: &tF32},
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "?")
}

// =============================================================================
// Test: GLSL relational expressions
// =============================================================================

func TestGLSL_RelationalExpressions(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.RelationalFunction
		want string
	}{
		{"isnan", ir.RelationalIsNan, "isnan("},
		{"isinf", ir.RelationalIsInf, "isinf("},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tF32 := ir.TypeHandle(0)
			tBool := ir.TypeHandle(1)
			retExpr := ir.ExpressionHandle(1)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
				},
				Functions: []ir.Function{{
					Name: "test_fn",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
						{Kind: ir.ExprRelational{Fun: tt.fun, Argument: 0}},
					},
					ExpressionTypes: []ir.TypeResolution{
						{Handle: &tF32},
						{Handle: &tBool},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
						{Kind: ir.StmtReturn{Value: &retExpr}},
					},
				}},
			}
			result := compileGLSL(t, module)
			mustContainGLSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: GLSL type cast (As) expression
// =============================================================================

func TestGLSL_AsExpression(t *testing.T) {
	t.Run("convert", func(t *testing.T) {
		tF32 := ir.TypeHandle(0)
		tI32 := ir.TypeHandle(1)
		width := uint8(4)
		retExpr := ir.ExpressionHandle(1)

		module := &ir.Module{
			Types: []ir.Type{
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			},
			Functions: []ir.Function{{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(3.14)}},
					{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarSint, Convert: &width}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		}
		result := compileGLSL(t, module)
		mustContainGLSL(t, result, "int(")
	})

	t.Run("bitcast", func(t *testing.T) {
		tF32 := ir.TypeHandle(0)
		tI32 := ir.TypeHandle(1)
		retExpr := ir.ExpressionHandle(1)

		module := &ir.Module{
			Types: []ir.Type{
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
				{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			},
			Functions: []ir.Function{{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(3.14)}},
					{Kind: ir.ExprAs{Expr: 0, Kind: ir.ScalarSint, Convert: nil}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			}},
		}
		result := compileGLSL(t, module)
		mustContainGLSL(t, result, "BitsTo")
	})
}

// =============================================================================
// Test: GLSL zero value expression
// =============================================================================

func TestGLSL_ZeroValueExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.ExprZeroValue{Type: tF32}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "(0)")
}

// =============================================================================
// Test: GLSL return with value
// =============================================================================

func TestGLSL_ReturnWithValue(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name:   "test_fn",
			Result: &ir.FunctionResult{Type: tF32},
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "return")
	mustContainGLSL(t, result, "42.0")
}

// =============================================================================
// Test: GLSL call statement
// =============================================================================

func TestGLSL_CallStatement(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "helper_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0, Arguments: nil}},
				},
			},
		},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "helper_fn(")
}

// =============================================================================
// Test: GLSL ImageStore statement
// =============================================================================

func TestGLSL_ImageStoreStatement(t *testing.T) {
	_ = ir.TypeHandle(0) // tF32
	tVec2 := ir.TypeHandle(1)
	tVec4 := ir.TypeHandle(2)
	tImg := ir.TypeHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}}, // image placeholder
				{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}}, // coordinate placeholder
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // value placeholder
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tImg},
				{Handle: &tVec2},
				{Handle: &tVec4},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
				{Kind: ir.StmtImageStore{Image: 0, Coordinate: 1, Value: 2}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "imageStore(")
}

// =============================================================================
// Test: GLSL atomic statement
// =============================================================================

func TestGLSL_AtomicStatement(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 4, Minor: 30}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
				{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tI32},
				{Handle: &tI32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtAtomic{
					Pointer: 0,
					Value:   1,
					Fun:     ir.AtomicAdd{},
				}},
			},
		}},
	}
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	mustContainGLSL(t, result, "atomicAdd(")
}

func TestGLSL_AtomicVersionRequirement(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 3, Minor: 0}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
				{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tI32},
				{Handle: &tI32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtAtomic{Pointer: 0, Value: 1, Fun: ir.AtomicAdd{}}},
			},
		}},
	}
	_, _, err := Compile(module, opts)
	if err == nil {
		t.Error("Expected error for atomic with old GLSL version")
	}
}

// =============================================================================
// Test: GLSL WorkGroupUniformLoad statement
// =============================================================================

func TestGLSL_WorkGroupUniformLoadStatement(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 4, Minor: 30}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
				{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tI32},
				{Handle: &tI32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtWorkGroupUniformLoad{Pointer: 0, Result: 1}},
			},
		}},
	}
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	mustContainGLSL(t, result, "barrier()")
}

// =============================================================================
// Test: GLSL compute entry point (exercises writeComputeLayout)
// =============================================================================

func TestGLSL_ComputeEntryPoint(t *testing.T) {
	tVec3U32 := ir.TypeHandle(0)

	var globalIdBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID}

	opts := DefaultOptions()
	opts.LangVersion = Version{Major: 4, Minor: 30}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec3u", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "cs_main",
			Arguments: []ir.FunctionArgument{
				{Name: "global_id", Type: tVec3U32, Binding: &globalIdBinding},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtReturn{}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "cs_main", Stage: ir.StageCompute, Function: 0, Workgroup: [3]uint32{64, 1, 1}},
		},
	}
	result, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	mustContainGLSL(t, result, "layout(local_size_x = 64")
}

// =============================================================================
// Test: GLSL local variable and load
// =============================================================================

func TestGLSL_LocalVariableAndLoad(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			LocalVars: []ir.LocalVariable{
				{Name: "myLocal", Type: tF32},
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprLocalVariable{Variable: 0}},
				{Kind: ir.ExprLoad{Pointer: 0}},
				{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
				{Kind: ir.StmtStore{Pointer: 0, Value: 2}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "myLocal")
}

// =============================================================================
// Test: GLSL splat expression
// =============================================================================

func TestGLSL_SplatExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				{Kind: ir.ExprSplat{Value: 0, Size: 4}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tVec4},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "vec4(")
}

// =============================================================================
// Test: GLSL vertex entry point with IO
// =============================================================================

func TestGLSL_VertexEntryPoint(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	tVec2 := ir.TypeHandle(1)

	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var loc0Binding ir.Binding = ir.LocationBinding{Location: 0}

	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec2f", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "vs_main",
			Arguments: []ir.FunctionArgument{
				{Name: "position", Type: tVec4, Binding: &loc0Binding},
			},
			Result: &ir.FunctionResult{
				Type:    tVec4,
				Binding: &posBinding,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tVec4},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "layout(location = 0) in")
	mustContainGLSL(t, result, "gl_Position")
	_ = tVec2
}

// =============================================================================
// Test: GLSL fragment entry point with outputs
// =============================================================================

func TestGLSL_FragmentEntryPoint(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	tVec2 := ir.TypeHandle(1)

	var loc0Input ir.Binding = ir.LocationBinding{Location: 0}
	var loc0Output ir.Binding = ir.LocationBinding{Location: 0}

	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec2f", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "fs_main",
			Arguments: []ir.FunctionArgument{
				{Name: "uv", Type: tVec2, Binding: &loc0Input},
			},
			Result: &ir.FunctionResult{
				Type:    tVec4,
				Binding: &loc0Output,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tVec2},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "layout(location = 0) in")
	mustContainGLSL(t, result, "layout(location = 0) out")
}

// =============================================================================
// Test: GLSL constant expression
// =============================================================================

func TestGLSL_ConstantExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "PI", Type: tF32, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40490fdb}},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.ExprConstant{Constant: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: (*ir.ExpressionHandle)(&tF32)}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "PI")
}

// =============================================================================
// Test: GLSL global variable
// =============================================================================

func TestGLSL_GlobalVariable(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "gValue", Type: tF32, Space: ir.SpacePrivate},
		},
		Functions: []ir.Function{{
			Name: "test_fn",
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 0}},
				{Kind: ir.ExprLoad{Pointer: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
				{Handle: &tF32},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
				{Kind: ir.StmtReturn{Value: (*ir.ExpressionHandle)(&tF32)}},
			},
		}},
	}
	result := compileGLSL(t, module)
	mustContainGLSL(t, result, "gValue")
}
