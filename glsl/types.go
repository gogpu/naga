// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// glslTypeSampler is the GLSL type name for samplers.
const glslTypeSampler = "sampler"

// typeToGLSL returns the GLSL type name for an IR type.
func (w *Writer) typeToGLSL(typ ir.Type) string {
	return w.typeInnerToGLSL(typ.Inner)
}

// typeInnerToGLSL returns the GLSL name for a TypeInner.
func (w *Writer) typeInnerToGLSL(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarToGLSL(t)
	case ir.VectorType:
		return vectorToGLSL(t)
	case ir.MatrixType:
		return matrixToGLSL(t)
	case ir.ArrayType:
		return w.arrayToGLSL(t)
	case ir.StructType:
		// Structs use their registered name
		for handle, regTyp := range w.module.Types {
			if st, ok := regTyp.Inner.(ir.StructType); ok {
				if structsEqual(st, t) {
					if name, ok := w.typeNames[ir.TypeHandle(handle)]; ok {
						return name
					}
				}
			}
		}
		return "struct_unknown"
	case ir.SamplerType:
		return glslTypeSampler
	case ir.ImageType:
		return w.imageToGLSL(t)
	case ir.PointerType:
		// GLSL doesn't have explicit pointers, return the pointee type
		return w.getTypeName(t.Base)
	case ir.AtomicType:
		return w.atomicToGLSL(t)
	case ir.AccelerationStructureType:
		return "accelerationStructureEXT"
	case ir.RayQueryType:
		return "rayQueryEXT"
	default:
		return "unknown_type"
	}
}

// scalarToGLSL returns the GLSL name for a scalar type.
func scalarToGLSL(t ir.ScalarType) string {
	switch t.Kind {
	case ir.ScalarAbstractInt:
		return glslTypeInt // Concretize to int
	case ir.ScalarAbstractFloat:
		return glslTypeFloat // Concretize to float
	case ir.ScalarBool:
		return "bool"
	case ir.ScalarSint:
		switch t.Width {
		case 1, 2, 4:
			return glslTypeInt
		case 8:
			return "int64_t" // Requires extension
		}
	case ir.ScalarUint:
		switch t.Width {
		case 1, 2, 4:
			return glslTypeUint
		case 8:
			return "uint64_t" // Requires extension
		}
	case ir.ScalarFloat:
		switch t.Width {
		case 2:
			return "float16_t" // Requires extension
		case 4:
			return glslTypeFloat
		case 8:
			return "double"
		}
	}
	return glslTypeInt // Default fallback
}

// vectorToGLSL returns the GLSL name for a vector type.
func vectorToGLSL(t ir.VectorType) string {
	size := t.Size
	if size < 2 || size > 4 {
		size = 4 // Clamp to valid range
	}

	switch t.Scalar.Kind {
	case ir.ScalarBool:
		return fmt.Sprintf("bvec%d", size)
	case ir.ScalarSint:
		return fmt.Sprintf("ivec%d", size)
	case ir.ScalarUint:
		return fmt.Sprintf("uvec%d", size)
	case ir.ScalarFloat:
		switch t.Scalar.Width {
		case 8:
			return fmt.Sprintf("dvec%d", size)
		default:
			return fmt.Sprintf("vec%d", size)
		}
	default:
		return fmt.Sprintf("vec%d", size)
	}
}

// matrixToGLSL returns the GLSL name for a matrix type.
func matrixToGLSL(t ir.MatrixType) string {
	cols := t.Columns
	rows := t.Rows

	if cols < 2 || cols > 4 {
		cols = 4
	}
	if rows < 2 || rows > 4 {
		rows = 4
	}

	// Rust naga always writes the full matCxR form, never the shorthand matN.
	switch t.Scalar.Kind {
	case ir.ScalarFloat:
		switch t.Scalar.Width {
		case 8:
			return fmt.Sprintf("dmat%dx%d", cols, rows)
		default:
			return fmt.Sprintf("mat%dx%d", cols, rows)
		}
	default:
		return fmt.Sprintf("mat%dx%d", cols, rows)
	}
}

// arrayToGLSL returns the GLSL name for an array type.
func (w *Writer) arrayToGLSL(t ir.ArrayType) string {
	// For nested arrays: outer dimension comes first in GLSL.
	// array<array<float, 2>, 3> → float[3][2]
	// Build all dimensions outer→inner by collecting sizes
	var dims []string
	current := t
	for {
		if current.Size.Constant != nil {
			dims = append(dims, fmt.Sprintf("[%d]", *current.Size.Constant))
		} else {
			dims = append(dims, "[]")
		}
		if int(current.Base) < len(w.module.Types) {
			if inner, ok := w.module.Types[current.Base].Inner.(ir.ArrayType); ok {
				current = inner
				continue
			}
		}
		break
	}
	// Base type is the innermost non-array type
	innerBase := w.getTypeName(current.Base)
	return innerBase + strings.Join(dims, "")
}

// imageToGLSL returns the GLSL name for an image/texture type.
func (w *Writer) imageToGLSL(t ir.ImageType) string {
	// Determine the prefix based on image class
	var prefix string
	switch t.Class {
	case ir.ImageClassSampled:
		// Sampled textures need scalar prefix: usampler for uint, isampler for sint.
		scalarPrefix := ""
		switch t.SampledKind {
		case ir.ScalarUint:
			scalarPrefix = "u"
		case ir.ScalarFloat:
			// Float is the default — no prefix
		default:
			// ScalarSint (Go iota 0) could be either sint or uninitialized zero value.
			// Real sint textures (texture_2d<i32>) set this explicitly.
			// However, Go zero value of ImageType also has SampledKind=0=ScalarSint.
			// To distinguish: if Multisampled is set or any other field hints at real usage,
			// trust it. Otherwise treat as float (no prefix).
			// In practice, our lowerer always sets ScalarFloat for float textures.
			scalarPrefix = "i"
		}
		prefix = scalarPrefix + glslTypeSampler
	case ir.ImageClassDepth:
		prefix = glslTypeSampler
	case ir.ImageClassStorage:
		// Storage images need scalar prefix: uimage for uint, iimage for sint, image for float
		scalarPrefix := ""
		switch t.StorageFormat.ScalarKind() {
		case ir.ScalarUint:
			scalarPrefix = "u"
		case ir.ScalarSint:
			scalarPrefix = "i"
		}
		prefix = scalarPrefix + "image"
	default:
		prefix = glslTypeSampler
	}

	// Build the full type name
	switch t.Dim {
	case ir.Dim1D:
		if t.Arrayed {
			return fmt.Sprintf("%s1DArray", prefix)
		}
		return fmt.Sprintf("%s1D", prefix)
	case ir.Dim2D:
		if t.Multisampled {
			if t.Arrayed {
				return fmt.Sprintf("%s2DMSArray", prefix)
			}
			return fmt.Sprintf("%s2DMS", prefix)
		}
		if t.Arrayed {
			if t.Class == ir.ImageClassDepth {
				return "sampler2DArrayShadow"
			}
			return fmt.Sprintf("%s2DArray", prefix)
		}
		if t.Class == ir.ImageClassDepth {
			return "sampler2DShadow"
		}
		return fmt.Sprintf("%s2D", prefix)
	case ir.Dim3D:
		return fmt.Sprintf("%s3D", prefix)
	case ir.DimCube:
		if t.Arrayed {
			if t.Class == ir.ImageClassDepth {
				return "samplerCubeArrayShadow"
			}
			return fmt.Sprintf("%sCubeArray", prefix)
		}
		if t.Class == ir.ImageClassDepth {
			return "samplerCubeShadow"
		}
		return fmt.Sprintf("%sCube", prefix)
	default:
		return fmt.Sprintf("%s2D", prefix)
	}
}

// atomicToGLSL returns the GLSL name for an atomic type.
func (w *Writer) atomicToGLSL(t ir.AtomicType) string {
	// GLSL uses atomic_uint for atomics or requires extensions
	switch t.Scalar.Kind {
	case ir.ScalarSint:
		return "int" // GLSL 4.30+ has atomicAdd for ints
	case ir.ScalarUint:
		return "uint"
	default:
		return "uint"
	}
}

// structsEqual compares two struct types for equality.
func structsEqual(a, b ir.StructType) bool {
	if len(a.Members) != len(b.Members) {
		return false
	}
	for i := range a.Members {
		if a.Members[i].Name != b.Members[i].Name {
			return false
		}
		if a.Members[i].Type != b.Members[i].Type {
			return false
		}
	}
	return true
}
