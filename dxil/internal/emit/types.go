package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// scalarToDXIL maps a naga ScalarType to a DXIL type.
// DXIL types are always scalar — vectors are decomposed by the emitter.
func scalarToDXIL(mod *module.Module, s ir.ScalarType) *module.Type {
	switch s.Kind {
	case ir.ScalarBool:
		return mod.GetIntType(1)
	case ir.ScalarSint:
		return mod.GetIntType(uint(s.Width) * 8)
	case ir.ScalarUint:
		return mod.GetIntType(uint(s.Width) * 8)
	case ir.ScalarFloat:
		return mod.GetFloatType(uint(s.Width) * 8)
	default:
		// Abstract types should have been resolved before reaching the backend.
		return mod.GetIntType(32)
	}
}

// typeToDXIL maps a naga IR TypeInner to a DXIL type.
// Vectors and matrices are scalarized — this returns the element type.
// Callers must handle multi-component types by iterating over components.
func typeToDXIL(mod *module.Module, irMod *ir.Module, inner ir.TypeInner) (*module.Type, error) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarToDXIL(mod, t), nil

	case ir.VectorType:
		// DXIL has no native vectors. Return the scalar element type.
		// The caller is responsible for creating N separate values.
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.MatrixType:
		// DXIL has no native matrices. Return the scalar element type.
		// Matrices are fully scalarized: columns * rows separate values.
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.ArrayType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXIL(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("array element type: %w", err)
		}
		if t.Size.Constant != nil {
			return mod.GetArrayType(elemTy, uint(*t.Size.Constant)), nil
		}
		// Runtime-sized array — use large count.
		return mod.GetArrayType(elemTy, 0), nil

	case ir.StructType:
		elems := make([]*module.Type, len(t.Members))
		for i, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			var err error
			elems[i], err = typeToDXIL(mod, irMod, memberTy.Inner)
			if err != nil {
				return nil, fmt.Errorf("struct member %d (%s): %w", i, m.Name, err)
			}
		}
		return mod.GetStructType("", elems), nil

	case ir.AtomicType:
		// Atomic types map to their underlying scalar in DXIL (i32, i64, etc.).
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.PointerType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXIL(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("pointer element type: %w", err)
		}
		return mod.GetPointerType(elemTy), nil

	default:
		return mod.GetIntType(32), nil
	}
}

// typeToDXILFull maps a naga IR TypeInner to a DXIL type WITHOUT scalarizing
// vectors. Used for struct allocas and GEP source element types where the
// full composite type is needed.
func typeToDXILFull(mod *module.Module, irMod *ir.Module, inner ir.TypeInner) (*module.Type, error) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return scalarToDXIL(mod, t), nil

	case ir.VectorType:
		// Keep as scalar — DXIL has no vectors, but struct members that are vectors
		// are represented as multiple scalar fields or as the scalar type.
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.MatrixType:
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.ArrayType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXILFull(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("array element type: %w", err)
		}
		if t.Size.Constant != nil {
			return mod.GetArrayType(elemTy, uint(*t.Size.Constant)), nil
		}
		return mod.GetArrayType(elemTy, 0), nil

	case ir.StructType:
		// Flatten struct members: vectors expand to N scalar fields,
		// nested structs recurse. This ensures every element in the DXIL
		// struct is a scalar type, matching DXIL's scalarized model.
		var elems []*module.Type
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			memberElems, err := flattenStructMember(mod, irMod, memberTy.Inner)
			if err != nil {
				return nil, fmt.Errorf("struct member %s: %w", m.Name, err)
			}
			elems = append(elems, memberElems...)
		}
		return mod.GetStructType("", elems), nil

	case ir.AtomicType:
		// Atomic types map to their underlying scalar in DXIL (i32, i64, etc.).
		return scalarToDXIL(mod, t.Scalar), nil

	case ir.PointerType:
		base := irMod.Types[t.Base]
		elemTy, err := typeToDXILFull(mod, irMod, base.Inner)
		if err != nil {
			return nil, fmt.Errorf("pointer element type: %w", err)
		}
		return mod.GetPointerType(elemTy), nil

	default:
		return mod.GetIntType(32), nil
	}
}

// flatMemberOffset computes the flat field index of a struct member at the
// given IR member index. This sums the total scalar component counts of all
// preceding members (vectors count as N, nested structs recursively flatten).
func flatMemberOffset(irMod *ir.Module, st ir.StructType, memberIndex int) int {
	offset := 0
	for i := 0; i < memberIndex && i < len(st.Members); i++ {
		memberTy := irMod.Types[st.Members[i].Type]
		offset += totalScalarCount(irMod, memberTy.Inner)
	}
	return offset
}

// totalScalarCount returns the total number of scalar values in a type,
// recursively flattening vectors, matrices, arrays, and nested structs.
func totalScalarCount(irMod *ir.Module, inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	case ir.ArrayType:
		// Arrays are kept as array types in the DXIL struct (not flattened),
		// so they count as 1 element for flat offset computation.
		return 1
	case ir.StructType:
		total := 0
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			total += totalScalarCount(irMod, memberTy.Inner)
		}
		return total
	default:
		return 1
	}
}

// flattenStructMember expands a struct member type into scalar DXIL types.
// Vectors become N scalars, arrays expand element-wise, nested structs recurse,
// scalars return as-is.
func flattenStructMember(mod *module.Module, irMod *ir.Module, inner ir.TypeInner) ([]*module.Type, error) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return []*module.Type{scalarToDXIL(mod, t)}, nil
	case ir.VectorType:
		s := scalarToDXIL(mod, t.Scalar)
		elems := make([]*module.Type, t.Size)
		for i := range elems {
			elems[i] = s
		}
		return elems, nil
	case ir.MatrixType:
		s := scalarToDXIL(mod, t.Scalar)
		count := int(t.Columns) * int(t.Rows)
		elems := make([]*module.Type, count)
		for i := range elems {
			elems[i] = s
		}
		return elems, nil
	case ir.ArrayType:
		// Arrays are kept as array types within the struct — NOT flattened.
		// Only vectors/matrices are scalarized.
		arrayTy, err := typeToDXIL(mod, irMod, inner)
		if err != nil {
			return nil, fmt.Errorf("array member: %w", err)
		}
		return []*module.Type{arrayTy}, nil
	case ir.StructType:
		var elems []*module.Type
		for _, m := range t.Members {
			memberTy := irMod.Types[m.Type]
			sub, err := flattenStructMember(mod, irMod, memberTy.Inner)
			if err != nil {
				return nil, err
			}
			elems = append(elems, sub...)
		}
		return elems, nil
	case ir.AtomicType:
		return []*module.Type{scalarToDXIL(mod, t.Scalar)}, nil
	default:
		return []*module.Type{mod.GetIntType(32)}, nil
	}
}

// cbvComponentCount returns the total number of scalar components for a type
// in the context of CBV (constant buffer) loads. Unlike componentCount, this
// recursively expands structs and arrays to count all scalar values.
func cbvComponentCount(irMod *ir.Module, inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	case ir.ArrayType:
		elemCount := 0
		if t.Size.Constant != nil {
			elemCount = int(*t.Size.Constant)
		}
		if elemCount == 0 {
			return 1
		}
		return elemCount * cbvComponentCount(irMod, irMod.Types[t.Base].Inner)
	case ir.StructType:
		total := 0
		for _, m := range t.Members {
			total += cbvComponentCount(irMod, irMod.Types[m.Type].Inner)
		}
		return total
	default:
		return 1
	}
}

// componentCount returns the number of scalar components for a type.
// isScalarizableType returns true if the type can be scalarized for DXIL
// function parameters (scalars, vectors, matrices). Returns false for
// arrays, structs, pointers, and other complex types that need special
// calling convention handling not yet implemented.
func isScalarizableType(inner ir.TypeInner) bool {
	switch inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		return true
	default:
		return false
	}
}

func componentCount(inner ir.TypeInner) int {
	switch t := inner.(type) {
	case ir.ScalarType:
		return 1
	case ir.VectorType:
		return int(t.Size)
	case ir.MatrixType:
		return int(t.Columns) * int(t.Rows)
	default:
		return 1
	}
}

// scalarOfType returns the scalar type underlying any type.
// For scalars, returns the scalar directly. For vectors/matrices,
// returns the element scalar.
func scalarOfType(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.AtomicType:
		return t.Scalar, true
	default:
		return ir.ScalarType{}, false
	}
}

// isFloatType returns true if the type is a floating-point type.
func isFloatType(inner ir.TypeInner) bool {
	s, ok := scalarOfType(inner)
	if !ok {
		return false
	}
	return s.Kind == ir.ScalarFloat
}

// isSignedInt returns true if the type is a signed integer type.
func isSignedInt(inner ir.TypeInner) bool {
	s, ok := scalarOfType(inner)
	if !ok {
		return false
	}
	return s.Kind == ir.ScalarSint
}
