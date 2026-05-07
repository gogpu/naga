// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// typeInnerToGLSL Tests — all type variants
// =============================================================================

func TestTypeInnerToGLSL_Sampler(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.SamplerType{}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	got := w.typeInnerToGLSL(ir.SamplerType{})
	if got != "sampler" {
		t.Errorf("SamplerType = %q, want %q", got, "sampler")
	}
}

func TestTypeInnerToGLSL_Pointer(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.typeNames[0] = "float"

	got := w.typeInnerToGLSL(ir.PointerType{Base: 0, Space: ir.SpaceFunction})
	if got != "float" {
		t.Errorf("PointerType = %q, want %q (pointee type)", got, "float")
	}
}

func TestTypeInnerToGLSL_AccelerationStructure(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version460})

	got := w.typeInnerToGLSL(ir.AccelerationStructureType{})
	if got != "accelerationStructureEXT" {
		t.Errorf("AccelerationStructureType = %q, want %q", got, "accelerationStructureEXT")
	}
}

func TestTypeInnerToGLSL_RayQuery(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version460})

	got := w.typeInnerToGLSL(ir.RayQueryType{})
	if got != "rayQueryEXT" {
		t.Errorf("RayQueryType = %q, want %q", got, "rayQueryEXT")
	}
}

// Note: typeInnerToGLSL unknown type path is tested indirectly.
// Direct testing requires implementing ir.TypeInner marker interface.

func TestTypeInnerToGLSL_StructLookup(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "MyData", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: 0, Offset: 0},
					{Name: "y", Type: 0, Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.typeNames[1] = "MyData"

	got := w.typeInnerToGLSL(ir.StructType{
		Members: []ir.StructMember{
			{Name: "x", Type: 0, Offset: 0},
			{Name: "y", Type: 0, Offset: 4},
		},
		Span: 8,
	})
	if got != "MyData" {
		t.Errorf("struct lookup = %q, want %q", got, "MyData")
	}
}

func TestTypeInnerToGLSL_StructUnknown(t *testing.T) {
	// Struct that doesn't match any registered type
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.typeInnerToGLSL(ir.StructType{
		Members: []ir.StructMember{
			{Name: "unregistered", Type: 0, Offset: 0},
		},
	})
	if got != "struct_unknown" {
		t.Errorf("unregistered struct = %q, want %q", got, "struct_unknown")
	}
}

func TestTypeInnerToGLSL_Atomic(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version430})

	tests := []struct {
		name   string
		atomic ir.AtomicType
		want   string
	}{
		{"uint", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint"},
		{"int", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int"},
		{"default", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.typeInnerToGLSL(tt.atomic)
			if got != tt.want {
				t.Errorf("AtomicType = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// imageToGLSL Tests — storage images with format
// =============================================================================

func TestImageToGLSL_StorageUint(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version430})

	got := w.imageToGLSL(ir.ImageType{
		Dim:           ir.Dim2D,
		Class:         ir.ImageClassStorage,
		StorageFormat: ir.StorageFormatRgba8Uint,
	})
	if got != "uimage2D" {
		t.Errorf("storage uint image = %q, want %q", got, "uimage2D")
	}
}

func TestImageToGLSL_StorageSint(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version430})

	got := w.imageToGLSL(ir.ImageType{
		Dim:           ir.Dim2D,
		Class:         ir.ImageClassStorage,
		StorageFormat: ir.StorageFormatRgba8Sint,
	})
	if got != "iimage2D" {
		t.Errorf("storage sint image = %q, want %q", got, "iimage2D")
	}
}

func TestImageToGLSL_StorageFloat(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version430})

	got := w.imageToGLSL(ir.ImageType{
		Dim:           ir.Dim2D,
		Class:         ir.ImageClassStorage,
		StorageFormat: ir.StorageFormatRgba32Float,
	})
	if got != "image2D" {
		t.Errorf("storage float image = %q, want %q", got, "image2D")
	}
}

// =============================================================================
// imageToGLSL Tests — Dim1D
// =============================================================================

func TestImageToGLSL_1D(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.Dim1D,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarFloat,
	})
	if got != "sampler1D" {
		t.Errorf("1D sampled = %q, want %q", got, "sampler1D")
	}
}

func TestImageToGLSL_1DArray(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.Dim1D,
		Arrayed:     true,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarFloat,
	})
	if got != "sampler1DArray" {
		t.Errorf("1D array sampled = %q, want %q", got, "sampler1DArray")
	}
}

// =============================================================================
// imageToGLSL Tests — 2D multisampled array
// =============================================================================

func TestImageToGLSL_2DMSArray(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version430})

	got := w.imageToGLSL(ir.ImageType{
		Dim:          ir.Dim2D,
		Multisampled: true,
		Arrayed:      true,
		Class:        ir.ImageClassSampled,
		SampledKind:  ir.ScalarFloat,
	})
	if got != "sampler2DMSArray" {
		t.Errorf("2DMS array = %q, want %q", got, "sampler2DMSArray")
	}
}

// =============================================================================
// imageToGLSL Tests — cube arrays
// =============================================================================

func TestImageToGLSL_CubeArray(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version400})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.DimCube,
		Arrayed:     true,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarFloat,
	})
	if got != "samplerCubeArray" {
		t.Errorf("cube array = %q, want %q", got, "samplerCubeArray")
	}
}

func TestImageToGLSL_CubeArrayShadow(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version400})

	got := w.imageToGLSL(ir.ImageType{
		Dim:     ir.DimCube,
		Arrayed: true,
		Class:   ir.ImageClassDepth,
	})
	if got != "samplerCubeArrayShadow" {
		t.Errorf("cube array shadow = %q, want %q", got, "samplerCubeArrayShadow")
	}
}

// =============================================================================
// imageToGLSL Tests — 2D array depth
// =============================================================================

func TestImageToGLSL_2DArrayShadow(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:     ir.Dim2D,
		Arrayed: true,
		Class:   ir.ImageClassDepth,
	})
	if got != "sampler2DArrayShadow" {
		t.Errorf("2D array shadow = %q, want %q", got, "sampler2DArrayShadow")
	}
}

// =============================================================================
// imageToGLSL Tests — sampled with uint/sint kind
// =============================================================================

func TestImageToGLSL_SampledUint(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.Dim2D,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarUint,
	})
	if got != "usampler2D" {
		t.Errorf("sampled uint = %q, want %q", got, "usampler2D")
	}
}

func TestImageToGLSL_SampledSint(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.Dim2D,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarSint,
	})
	if got != "isampler2D" {
		t.Errorf("sampled sint = %q, want %q", got, "isampler2D")
	}
}

// =============================================================================
// imageToGLSL Tests — default dimension
// =============================================================================

func TestImageToGLSL_DefaultDim(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.ImageDimension(15), // unknown dimension
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarFloat,
	})
	if got != "sampler2D" {
		t.Errorf("unknown dim = %q, want fallback %q", got, "sampler2D")
	}
}

// =============================================================================
// imageToGLSL Tests — 3D
// =============================================================================

func TestImageToGLSL_3D(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.Dim3D,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarFloat,
	})
	if got != "sampler3D" {
		t.Errorf("3D sampled = %q, want %q", got, "sampler3D")
	}
}

// =============================================================================
// imageToGLSL Tests — 2D sampled array (non-depth)
// =============================================================================

func TestImageToGLSL_2DArray(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:         ir.Dim2D,
		Arrayed:     true,
		Class:       ir.ImageClassSampled,
		SampledKind: ir.ScalarFloat,
	})
	if got != "sampler2DArray" {
		t.Errorf("2D array sampled = %q, want %q", got, "sampler2DArray")
	}
}

// =============================================================================
// imageToGLSL Tests — default class
// =============================================================================

func TestImageToGLSL_DefaultClass(t *testing.T) {
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	got := w.imageToGLSL(ir.ImageType{
		Dim:   ir.Dim2D,
		Class: ir.ImageClass(15), // unknown class
	})
	if got != "sampler2D" {
		t.Errorf("unknown class = %q, want fallback %q", got, "sampler2D")
	}
}

// =============================================================================
// scalarToGLSL Tests — abstract types
// =============================================================================

func TestScalarToGLSL_AbstractTypes(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"abstract_int", ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 4}, "int"},
		{"abstract_float", ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 4}, "float"},
		{"unknown_kind", ir.ScalarType{Kind: ir.ScalarKind(99), Width: 4}, "int"}, // default fallback
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
// vectorToGLSL Tests — float16
// =============================================================================

func TestVectorToGLSL_Float16(t *testing.T) {
	// f16 vectors are not standard GLSL types but we should handle them
	vec := ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}
	got := vectorToGLSL(vec)
	// Should fall through to default vec case
	if got != "vec3" {
		t.Errorf("f16 vec3 = %q, want %q", got, "vec3")
	}
}

func TestVectorToGLSL_DefaultKind(t *testing.T) {
	// Unknown scalar kind
	vec := ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarKind(99), Width: 4}}
	got := vectorToGLSL(vec)
	if got != "vec2" {
		t.Errorf("unknown kind vec2 = %q, want %q (default)", got, "vec2")
	}
}

// =============================================================================
// matrixToGLSL Tests — default kind
// =============================================================================

func TestMatrixToGLSL_DefaultKind(t *testing.T) {
	mat := ir.MatrixType{Columns: 3, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarKind(99), Width: 4}}
	got := matrixToGLSL(mat)
	if got != "mat3x2" {
		t.Errorf("unknown kind mat = %q, want %q (default)", got, "mat3x2")
	}
}

// =============================================================================
// arrayToGLSL Tests — dynamic array
// =============================================================================

func TestArrayToGLSL_DynamicSize(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: nil}, Stride: 4}}, // dynamic
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.typeNames[0] = "float"

	got := w.typeToGLSL(module.Types[1])
	if got != "float[]" {
		t.Errorf("dynamic array = %q, want %q", got, "float[]")
	}
}

// =============================================================================
// structsEqual Tests — type mismatch
// =============================================================================

func TestStructsEqual_TypeMismatch(t *testing.T) {
	a := ir.StructType{
		Members: []ir.StructMember{
			{Name: "x", Type: 0, Offset: 0},
		},
	}
	b := ir.StructType{
		Members: []ir.StructMember{
			{Name: "x", Type: 1, Offset: 0}, // different type
		},
	}

	if structsEqual(a, b) {
		t.Error("structs with different member types should not be equal")
	}
}
