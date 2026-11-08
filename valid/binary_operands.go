package valid

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// This file is the operand check of binary operators: Rust naga's
// validate_expression for Expression::Binary (valid/expression.rs).
//
// Rust also rejects a constant integer divisor of zero and a constant shift
// amount of at least the bit width there (validate_constant_divisor,
// validate_constant_shift_amounts). The WGSL spec makes those errors only
// when the operand is a const-expression, and in the IR a `let` bound to a
// constant looks the same: Rust rejects the valid `let z = 0u; x / z`, and
// after ProcessOverrides a let initialized from an override. Those checks
// live in the WGSL frontend instead, which knows a let from a const
// (wgsl/internal/lower, checkConstantRHS; P1-WGSL-101).

// binaryOperatorNames are Rust naga's BinaryOperator variant names.
var binaryOperatorNames = [...]string{
	ir.BinaryAdd:          "Add",
	ir.BinarySubtract:     "Subtract",
	ir.BinaryMultiply:     "Multiply",
	ir.BinaryDivide:       "Divide",
	ir.BinaryModulo:       "Modulo",
	ir.BinaryEqual:        "Equal",
	ir.BinaryNotEqual:     "NotEqual",
	ir.BinaryLess:         "Less",
	ir.BinaryLessEqual:    "LessEqual",
	ir.BinaryGreater:      "Greater",
	ir.BinaryGreaterEqual: "GreaterEqual",
	ir.BinaryAnd:          "And",
	ir.BinaryExclusiveOr:  "ExclusiveOr",
	ir.BinaryInclusiveOr:  "InclusiveOr",
	ir.BinaryLogicalAnd:   "LogicalAnd",
	ir.BinaryLogicalOr:    "LogicalOr",
	ir.BinaryShiftLeft:    "ShiftLeft",
	ir.BinaryShiftRight:   "ShiftRight",
}

// binaryOperatorName is op as Rust's Debug prints it in an error message.
func binaryOperatorName(op ir.BinaryOperator) string {
	if int(op) < len(binaryOperatorNames) {
		return binaryOperatorNames[op]
	}
	return fmt.Sprintf("BinaryOperator(%d)", op)
}

// checkBinaryOperands checks the operand types of a binary operator
// (ExpressionError::InvalidBinaryOperandTypes).
func (v *validator) checkBinaryOperands(handle ir.ExpressionHandle, b ir.ExprBinary) {
	left, okL := v.expressionInner(b.Left)
	right, okR := v.expressionInner(b.Right)
	if !okL || !okR {
		return
	}
	if !binaryOperandsValid(b.Op, left, right) {
		v.addErrorInExpression(handle, fmt.Sprintf("Operation %s can't work with [%d] (of type %s) and [%d] (of type %s)",
			binaryOperatorName(b.Op), b.Left, typeInnerDebug(left), b.Right, typeInnerDebug(right)))
	}
}

// scalarOf is the scalar of a scalar, vector or matrix type (Rust's
// TypeInner::scalar).
func scalarOf(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	}
	return ir.ScalarType{}, false
}

// scalarOrVectorKind is the scalar kind of a scalar or vector type.
func scalarOrVectorKind(inner ir.TypeInner) (ir.ScalarKind, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind, true
	case ir.VectorType:
		return t.Scalar.Kind, true
	}
	return 0, false
}

// isConcreteNumber reports a concrete integer or float kind.
func isConcreteNumber(k ir.ScalarKind) bool {
	return k == ir.ScalarSint || k == ir.ScalarUint || k == ir.ScalarFloat
}

// isSizedInner is Rust's TypeInner::is_sized (valid/mod.rs).
func isSizedInner(inner ir.TypeInner) bool {
	switch t := inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType, ir.AtomicType, ir.PointerType, ir.StructType:
		return true
	case ir.ArrayType:
		return t.Size.Constant != nil
	}
	return false
}

// binaryOperandsValid is Rust's `good` match of Expression::Binary.
func binaryOperandsValid(op ir.BinaryOperator, left, right ir.TypeInner) bool {
	scalarOrVectorOf := func(kinds ...ir.ScalarKind) bool {
		k, ok := scalarOrVectorKind(left)
		if !ok {
			return false
		}
		for _, want := range kinds {
			if k == want {
				return sameTypeInner(left, right)
			}
		}
		return false
	}
	switch op {
	case ir.BinaryAdd, ir.BinarySubtract:
		if _, ok := left.(ir.MatrixType); ok {
			return sameTypeInner(left, right)
		}
		return scalarOrVectorOf(ir.ScalarUint, ir.ScalarSint, ir.ScalarFloat)
	case ir.BinaryDivide, ir.BinaryModulo,
		ir.BinaryLess, ir.BinaryLessEqual, ir.BinaryGreater, ir.BinaryGreaterEqual:
		return scalarOrVectorOf(ir.ScalarUint, ir.ScalarSint, ir.ScalarFloat)
	case ir.BinaryMultiply:
		return multiplyOperandsValid(left, right)
	case ir.BinaryEqual, ir.BinaryNotEqual:
		return isSizedInner(left) && sameTypeInner(left, right)
	case ir.BinaryLogicalAnd, ir.BinaryLogicalOr:
		return scalarOrVectorOf(ir.ScalarBool)
	case ir.BinaryAnd, ir.BinaryInclusiveOr:
		return scalarOrVectorOf(ir.ScalarBool, ir.ScalarSint, ir.ScalarUint)
	case ir.BinaryExclusiveOr:
		return scalarOrVectorOf(ir.ScalarSint, ir.ScalarUint)
	case ir.BinaryShiftLeft, ir.BinaryShiftRight:
		return shiftOperandsValid(left, right)
	}
	return false
}

// multiplyOperandsValid is Rust's BinaryOperator::Multiply case: a concrete
// numeric left kind, matching shapes (scalar and vector of one scalar,
// float scalar and matrix, vectors of one size, matrix columns against vector
// size, vector size against matrix rows, matrix columns against matrix rows),
// and equal scalar widths.
func multiplyOperandsValid(left, right ir.TypeInner) bool {
	ls, ok := scalarOf(left)
	if !ok || !isConcreteNumber(ls.Kind) {
		return false
	}
	rs, _ := scalarOf(right)
	match := false
	switch l := left.(type) {
	case ir.ScalarType:
		switch r := right.(type) {
		case ir.ScalarType:
			match = l == r
		case ir.VectorType:
			match = l == r.Scalar
		case ir.MatrixType:
			match = l.Kind == ir.ScalarFloat
		}
	case ir.VectorType:
		switch r := right.(type) {
		case ir.ScalarType:
			match = l.Scalar == r
		case ir.VectorType:
			match = l.Scalar == r.Scalar && l.Size == r.Size
		case ir.MatrixType:
			match = l.Scalar.Kind == ir.ScalarFloat && l.Size == r.Rows
		}
	case ir.MatrixType:
		switch r := right.(type) {
		case ir.ScalarType:
			match = r.Kind == ir.ScalarFloat
		case ir.VectorType:
			match = r.Scalar.Kind == ir.ScalarFloat && l.Columns == r.Size
		case ir.MatrixType:
			match = l.Columns == r.Rows
		}
	}
	return match && ls.Width == rs.Width
}

// shiftOperandsValid is Rust's shift case: an integer scalar or vector
// shifted by a u32 scalar or a u32 vector of the same size.
func shiftOperandsValid(left, right ir.TypeInner) bool {
	var baseSize ir.VectorSize
	var baseKind ir.ScalarKind
	switch l := left.(type) {
	case ir.ScalarType:
		baseKind = l.Kind
	case ir.VectorType:
		baseSize, baseKind = l.Size, l.Scalar.Kind
	default:
		return false
	}
	var shiftSize ir.VectorSize
	switch r := right.(type) {
	case ir.ScalarType:
		if r.Kind != ir.ScalarUint {
			return false
		}
	case ir.VectorType:
		if r.Scalar.Kind != ir.ScalarUint {
			return false
		}
		shiftSize = r.Size
	default:
		return false
	}
	return (baseKind == ir.ScalarSint || baseKind == ir.ScalarUint) && baseSize == shiftSize
}
