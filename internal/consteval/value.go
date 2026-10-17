package consteval

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Value is the operand and result of the numeric kernel: a scalar, or a
// vector of 2 to 4 components that all have the same scalar type.
type Value struct {
	comps []ir.LiteralValue
}

// Scalar returns a scalar value.
func Scalar(lit ir.LiteralValue) Value {
	return Value{comps: []ir.LiteralValue{lit}}
}

// Vector returns a vector value. It panics unless there are 2 to 4 components
// of one scalar type: a malformed vector is a bug in the caller, not a
// shader error.
func Vector(comps ...ir.LiteralValue) Value {
	if len(comps) < 2 || len(comps) > 4 {
		panic(fmt.Sprintf("consteval: vector of %d components", len(comps)))
	}
	s := ScalarOf(comps[0])
	for _, c := range comps[1:] {
		if ScalarOf(c) != s {
			panic(fmt.Sprintf("consteval: vector mixes %T and %T", comps[0], c))
		}
	}
	return Value{comps: comps}
}

// IsScalar reports whether v is a scalar.
func (v Value) IsScalar() bool { return len(v.comps) == 1 }

// Len returns the number of components: 1 for a scalar.
func (v Value) Len() int { return len(v.comps) }

// Size returns the vector size; it is only meaningful for a vector.
func (v Value) Size() ir.VectorSize { return ir.VectorSize(len(v.comps)) } //nolint:gosec // 2 to 4, checked by Vector

// Component returns component i.
func (v Value) Component(i int) ir.LiteralValue { return v.comps[i] }

// Components returns the components; the caller must not modify the slice.
func (v Value) Components() []ir.LiteralValue { return v.comps }

// Scalar returns the scalar type of the components.
func (v Value) Scalar() ir.ScalarType { return ScalarOf(v.comps[0]) }

// withComponents returns a value of v's shape holding comps.
func (v Value) withComponents(comps []ir.LiteralValue) Value {
	return Value{comps: comps}
}

// ScalarOf returns the scalar type of a literal. Abstract types have width 8,
// the width of their representation (int64 and float64).
func ScalarOf(lit ir.LiteralValue) ir.ScalarType {
	switch lit.(type) {
	case ir.LiteralBool:
		return ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	case ir.LiteralI32:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	case ir.LiteralU32:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	case ir.LiteralI64:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 8}
	case ir.LiteralU64:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 8}
	case ir.LiteralF16:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	case ir.LiteralF32:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	case ir.LiteralF64:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	case ir.LiteralAbstractInt:
		return ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}
	case ir.LiteralAbstractFloat:
		return ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}
	}
	panic(fmt.Sprintf("consteval: unknown literal %T", lit))
}
