package lower

import (
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// The notes cite the rule behind the two errors below; a lowerer note is a
// second SourceError, which wgsl.Parse turns into notes next to the first.
const (
	noteConstDivisorZero = "WGSL spec §arithmetic-expr: division or remainder by a const-expression that evaluates to zero is a shader-creation error"
	noteConstShiftAmount = "WGSL spec §arithmetic-expr: a const-expression shift amount at or above the bit width of the shifted value is a shader-creation error"
)

// checkConstantRHS reports the shader-creation errors WGSL §arithmetic-expr
// makes for a run-time binary operation whose right operand is a
// const-expression: an integer divisor of zero ("Division by zero", Rust's
// text) and a shift amount at or above the bit width of the left operand
// ("Shift amount exceeds the bit width of ...", Rust's text with the type).
//
// Only true const-expressions are checked. A let is a run-time expression
// even when bound to a constant (the evaluator marks it), and an
// override-expression is a pipeline-creation error at most, not a
// shader-creation one. Rust naga checks both in its validator, where a let
// bound to a constant looks like the constant and valid shaders are rejected
// with it (valid/expression.rs validate_constant_divisor and
// validate_constant_shift_amounts); the frontend knows the difference, so the
// check lives here (P1-WGSL-101). A fully const operation never reaches this:
// the evaluator folds it and reports its own error.
func (l *Lowerer) checkConstantRHS(op ir.BinaryOperator, left, right ir.ExpressionHandle, span parser.Span) {
	switch op {
	case ir.BinaryDivide, ir.BinaryModulo, ir.BinaryShiftLeft, ir.BinaryShiftRight:
	default:
		return
	}
	ev := l.eval()
	if !ev.IsConst(right) || ev.IsConst(left) {
		return
	}
	leftType := l.resolveExprValueType(left)
	scalar, ok := concreteIntegerLeaf(leftType)
	if !ok {
		return
	}
	value, ok := ev.Value(right)
	if !ok {
		return
	}
	if op == ir.BinaryShiftLeft || op == ir.BinaryShiftRight {
		bits := uint64(scalar.Width) * 8
		for _, comp := range value.Components() {
			if amount, ok := shiftAmountValue(comp); ok && amount >= bits {
				l.addError(fmt.Sprintf("Shift amount exceeds the bit width of %s", shiftLHSDebug(leftType)), span)
				l.addError(noteConstShiftAmount, parser.Span{})
				return
			}
		}
		return
	}
	for _, comp := range value.Components() {
		if integerIsZero(comp) {
			l.addError("Division by zero", span)
			l.addError(noteConstDivisorZero, parser.Span{})
			return
		}
	}
}

// concreteIntegerLeaf is the scalar of a concrete integer scalar or vector
// type. Abstract kinds are left to the constant evaluator, which folds them
// (an abstract operand makes the operation a const-expression), and other
// types cannot divide or shift at all.
func concreteIntegerLeaf(inner ir.TypeInner) (ir.ScalarType, bool) {
	var scalar ir.ScalarType
	switch t := inner.(type) {
	case ir.ScalarType:
		scalar = t
	case ir.VectorType:
		scalar = t.Scalar
	default:
		return ir.ScalarType{}, false
	}
	if scalar.Kind != ir.ScalarSint && scalar.Kind != ir.ScalarUint {
		return ir.ScalarType{}, false
	}
	return scalar, true
}

// shiftLHSDebug is the left operand's type as Rust's ShiftAmountTooLarge
// message embeds it: the Debug of TypeInner, which for the types a shift
// accepts (a concrete integer scalar or vector, see concreteIntegerLeaf) is
// Scalar(Scalar { kind: ..., width: ... }) or Vector { size: ..., scalar: ... }.
// The validator has the full formatter (valid/rust_debug.go); the layering
// test keeps valid on the IR and the typifier, so the frontend carries the
// two shapes its own message can produce.
func shiftLHSDebug(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return "Scalar(" + shiftScalarDebug(t) + ")"
	case ir.VectorType:
		return fmt.Sprintf("Vector { size: %s, scalar: %s }", shiftVectorSizeDebug(t.Size), shiftScalarDebug(t.Scalar))
	}
	return fmt.Sprintf("%T", inner)
}

// shiftScalarDebug is s as Rust's Debug prints a Scalar.
func shiftScalarDebug(s ir.ScalarType) string {
	kind := fmt.Sprintf("ScalarKind(%d)", s.Kind)
	switch s.Kind {
	case ir.ScalarSint:
		kind = "Sint"
	case ir.ScalarUint:
		kind = "Uint"
	}
	return fmt.Sprintf("Scalar { kind: %s, width: %d }", kind, s.Width)
}

// shiftVectorSizeDebug is n as Rust's Debug prints a VectorSize.
func shiftVectorSizeDebug(n ir.VectorSize) string {
	switch n {
	case ir.Vec2:
		return "Bi"
	case ir.Vec3:
		return "Tri"
	case ir.Vec4:
		return "Quad"
	}
	return fmt.Sprintf("VectorSize(%d)", n)
}

// integerIsZero reports an integer literal equal to zero.
func integerIsZero(v ir.LiteralValue) bool {
	switch n := v.(type) {
	case ir.LiteralI32:
		return n == 0
	case ir.LiteralU32:
		return n == 0
	case ir.LiteralI16:
		return n == 0
	case ir.LiteralU16:
		return n == 0
	case ir.LiteralI64:
		return n == 0
	case ir.LiteralU64:
		return n == 0
	}
	return false
}

// shiftAmountValue is a literal shift amount as an unsigned value; the right
// operand of a shift is u32, so a negative value cannot appear.
func shiftAmountValue(v ir.LiteralValue) (uint64, bool) {
	switch n := v.(type) {
	case ir.LiteralU32:
		return uint64(n), true
	case ir.LiteralI32:
		if n >= 0 {
			return uint64(n), true
		}
	case ir.LiteralU16:
		return uint64(n), true
	case ir.LiteralI16:
		if n >= 0 {
			return uint64(n), true
		}
	case ir.LiteralU64:
		return uint64(n), true
	case ir.LiteralI64:
		if n >= 0 {
			return uint64(n), true
		}
	}
	return 0, false
}
