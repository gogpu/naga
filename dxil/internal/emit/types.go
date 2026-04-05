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

// componentCount returns the number of scalar components for a type.
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
