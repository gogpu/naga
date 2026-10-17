package consteval

import (
	"math"
	"math/bits"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// Binary evaluates a binary operator on scalars or vectors.
//
// Both operands must have the same scalar type, except the right operand of
// a shift, which is u32. Vector operands are combined component-wise; a
// vector and a scalar combine only with + - * / %, the scalar staying on its
// side. Comparisons yield bool components.
//
// Concrete integer + - * wrap; abstract integer arithmetic is
// overflow-checked; float results must be finite. Invalid operand types are
// ErrInvalidBinaryArgs, which a function body keeps as a run-time expression.
func Binary(op ir.BinaryOperator, a, b Value) (Value, error) {
	switch {
	case a.Len() == b.Len():
		if !a.IsScalar() && (op == ir.BinaryLogicalAnd || op == ir.BinaryLogicalOr) {
			return Value{}, errInvalidBinaryArgs()
		}
		return lanes(a.Len(), func(i int) (ir.LiteralValue, error) { return scalarBinary(op, a.comps[i], b.comps[i]) })
	case a.IsScalar() != b.IsScalar():
		vec := a
		if a.IsScalar() {
			vec = b
		}
		if !isArithmetic(op) || vec.Scalar().Kind == ir.ScalarBool {
			return Value{}, errInvalidBinaryArgs()
		}
		return lanes(vec.Len(), func(i int) (ir.LiteralValue, error) {
			return scalarBinary(op, a.comps[min(i, a.Len()-1)], b.comps[min(i, b.Len()-1)])
		})
	}
	return Value{}, errInvalidBinaryArgs()
}

func lanes(n int, f func(int) (ir.LiteralValue, error)) (Value, error) {
	out := make([]ir.LiteralValue, n)
	for i := range out {
		r, err := f(i)
		if err != nil {
			return Value{}, err
		}
		out[i] = r
	}
	return Value{comps: out}, nil
}

func isArithmetic(op ir.BinaryOperator) bool {
	return op <= ir.BinaryModulo
}

func isComparison(op ir.BinaryOperator) bool {
	return op >= ir.BinaryEqual && op <= ir.BinaryGreaterEqual
}

func errInvalidBinaryArgs() *Error {
	return newError(ErrInvalidBinaryArgs, "Cannot apply the binary op to the arguments")
}

// rustBinaryName is the Rust BinaryOperator variant name used in its float
// overflow messages ("Divide operation overflowed").
var rustBinaryName = [...]string{"Add", "Subtract", "Multiply", "Divide", "Modulo"}

func scalarBinary(op ir.BinaryOperator, x, y ir.LiteralValue) (ir.LiteralValue, error) {
	if op == ir.BinaryShiftLeft || op == ir.BinaryShiftRight {
		return shift(op, x, y)
	}
	s := ScalarOf(x)
	if ScalarOf(y) != s {
		return nil, errInvalidBinaryArgs()
	}
	if isComparison(op) {
		return compareOp(op, x, y)
	}
	switch {
	case s.Kind == ir.ScalarBool:
		return boolBinary(op, bool(x.(ir.LiteralBool)), bool(y.(ir.LiteralBool)))
	case isFloat(s):
		return floatBinary(op, s, floatOf(x), floatOf(y))
	case s.Kind == ir.ScalarAbstractInt:
		return abstractIntBinary(op, int64(x.(ir.LiteralAbstractInt)), int64(y.(ir.LiteralAbstractInt)))
	case isInteger(s):
		return concreteIntBinary(op, s, intBits(x), intBits(y))
	}
	return nil, errInvalidBinaryArgs()
}

func compareOp(op ir.BinaryOperator, x, y ir.LiteralValue) (ir.LiteralValue, error) {
	if ScalarOf(x).Kind == ir.ScalarBool {
		eq := x == y
		switch op {
		case ir.BinaryEqual:
			return ir.LiteralBool(eq), nil
		case ir.BinaryNotEqual:
			return ir.LiteralBool(!eq), nil
		}
		return nil, errInvalidBinaryArgs()
	}
	c := compare(x, y)
	var r bool
	switch op {
	case ir.BinaryEqual:
		r = c == 0
	case ir.BinaryNotEqual:
		r = c != 0
	case ir.BinaryLess:
		r = c < 0
	case ir.BinaryLessEqual:
		r = c <= 0
	case ir.BinaryGreater:
		r = c > 0
	default:
		r = c >= 0
	}
	return ir.LiteralBool(r), nil
}

func boolBinary(op ir.BinaryOperator, x, y bool) (ir.LiteralValue, error) {
	switch op {
	case ir.BinaryLogicalAnd, ir.BinaryAnd:
		return ir.LiteralBool(x && y), nil
	case ir.BinaryLogicalOr, ir.BinaryInclusiveOr:
		return ir.LiteralBool(x || y), nil
	}
	return nil, errInvalidBinaryArgs()
}

// floatBinary computes one operation, rounded once to s. Rust reports a
// non-finite f32 result through its literal check and others as an
// operation overflow; the messages follow it.
func floatBinary(op ir.BinaryOperator, s ir.ScalarType, x, y float64) (ir.LiteralValue, error) {
	var r float64
	switch op {
	case ir.BinaryAdd:
		r = x + y
	case ir.BinarySubtract:
		r = x - y
	case ir.BinaryMultiply:
		r = x * y
	case ir.BinaryDivide:
		r = x / y
	case ir.BinaryModulo:
		r = math.Mod(x, y)
	default:
		return nil, errInvalidBinaryArgs()
	}
	lit, ok := makeFloat(s, r)
	if ok {
		return lit, nil
	}
	if s.Kind == ir.ScalarFloat && s.Width == 4 {
		if math.IsNaN(r) {
			return nil, newError(ErrOverflow, "Float literal is NaN")
		}
		return nil, newError(ErrOverflow, "Float literal is infinite")
	}
	return nil, newError(ErrOverflow, rustBinaryName[op]+" operation overflowed")
}

func overflowed(what string) *Error {
	return newError(ErrOverflow, what+" operation overflowed")
}

func abstractIntBinary(op ir.BinaryOperator, x, y int64) (ir.LiteralValue, error) {
	var r int64
	switch op {
	case ir.BinaryAdd:
		var ok bool
		if r, ok = addInt64(x, y); !ok {
			return nil, overflowed("addition")
		}
	case ir.BinarySubtract:
		if y == math.MinInt64 {
			if x >= 0 {
				return nil, overflowed("subtraction")
			}
			r = x - y
		} else {
			var ok bool
			if r, ok = addInt64(x, -y); !ok {
				return nil, overflowed("subtraction")
			}
		}
	case ir.BinaryMultiply:
		var ok bool
		if r, ok = mulInt64(x, y); !ok {
			return nil, overflowed("multiplication")
		}
	case ir.BinaryDivide, ir.BinaryModulo:
		q, err := signedDivRem(op, x, y, math.MinInt64)
		if err != nil {
			return nil, err
		}
		r = q
	case ir.BinaryAnd:
		r = x & y
	case ir.BinaryExclusiveOr:
		r = x ^ y
	case ir.BinaryInclusiveOr:
		r = x | y
	default:
		return nil, errInvalidBinaryArgs()
	}
	return ir.LiteralAbstractInt(r), nil
}

// signedDivRem is x / y or x % y truncating toward zero; zero divisors and
// MIN / -1 (MIN being the type's minimum) are errors.
func signedDivRem(op ir.BinaryOperator, x, y, minimum int64) (int64, error) {
	div := op == ir.BinaryDivide
	if y == 0 {
		return 0, zeroDivisor(op)
	}
	if x == minimum && y == -1 {
		if div {
			return 0, overflowed("division")
		}
		return 0, overflowed("remainder")
	}
	if div {
		return x / y, nil
	}
	return x % y, nil
}

// zeroDivisor is the error for an integer / or % by zero.
func zeroDivisor(op ir.BinaryOperator) *Error {
	if op == ir.BinaryDivide {
		return newError(ErrDivisionByZero, "Division by zero")
	}
	return newError(ErrRemainderByZero, "Remainder by zero")
}

func concreteIntBinary(op ir.BinaryOperator, s ir.ScalarType, x, y uint64) (ir.LiteralValue, error) {
	var r uint64
	switch op {
	case ir.BinaryAdd:
		r = x + y
	case ir.BinarySubtract:
		r = x - y
	case ir.BinaryMultiply:
		r = x * y
	case ir.BinaryDivide, ir.BinaryModulo:
		q, err := concreteDivRem(op, s, x, y)
		if err != nil {
			return nil, err
		}
		r = q
	case ir.BinaryAnd:
		r = x & y
	case ir.BinaryExclusiveOr:
		r = x ^ y
	case ir.BinaryInclusiveOr:
		r = x | y
	default:
		return nil, errInvalidBinaryArgs()
	}
	return makeInt(s, r), nil
}

// concreteDivRem divides the bit patterns of two values of the concrete
// integer type s: signed types truncate toward zero and reject MIN / -1; a
// zero divisor is an error for both.
func concreteDivRem(op ir.BinaryOperator, s ir.ScalarType, x, y uint64) (uint64, error) {
	if s.Kind == ir.ScalarSint {
		minimum := int64(math.MinInt64)
		if s.Width == 4 {
			minimum = math.MinInt32
		}
		w := uint(s.Width) * 8
		q, err := signedDivRem(op, signExtend(x, w), signExtend(y, w), minimum)
		return u64FromI64(q), err
	}
	if y == 0 {
		return 0, zeroDivisor(op)
	}
	if op == ir.BinaryDivide {
		return x / y, nil
	}
	return x % y, nil
}

// shift evaluates << and >>. The count is u32. Shifting left must not lose
// significant bits (for signed types, bits that differ from the result's
// sign); a count of at least the operand width is an error, except that an
// abstract >> fills with the sign bit for any count, as the spec defines.
func shift(op ir.BinaryOperator, x, y ir.LiteralValue) (ir.LiteralValue, error) {
	count, ok := y.(ir.LiteralU32)
	s := ScalarOf(x)
	if !ok || !isInteger(s) {
		return nil, errInvalidBinaryArgs()
	}
	b := uint(count)
	w := uint(s.Width) * 8
	if op == ir.BinaryShiftRight {
		if s.Kind == ir.ScalarAbstractInt {
			v := int64(x.(ir.LiteralAbstractInt))
			return ir.LiteralAbstractInt(v >> min(b, 63)), nil
		}
		if b >= w {
			return nil, shiftTooFar(w)
		}
		if s.Kind == ir.ScalarSint {
			return makeInt(s, u64FromI64(signExtend(intBits(x), w)>>b)), nil
		}
		return makeInt(s, intBits(x)>>b), nil
	}
	if isSigned(s) {
		v := signExtend(intBits(x), w)
		m := v
		if m < 0 {
			m = ^m
		}
		// Leading zeros of m within the operand width.
		lz := uint(bits.LeadingZeros64(u64FromI64(m))) - (64 - w)
		if lz <= b {
			return nil, overflowed("<<")
		}
		return makeIntOrAbstract(s, u64FromI64(v)<<b), nil
	}
	if b >= w {
		return nil, shiftTooFar(w)
	}
	u := intBits(x)
	if r := u << b; r>>b != u || (w < 64 && r>>w != 0) {
		return nil, overflowed("<<")
	}
	return makeInt(s, u<<b), nil
}

// signExtend interprets the low w bits of u as a signed value.
func signExtend(u uint64, w uint) int64 {
	return i64FromU64(u<<(64-w)) >> (64 - w)
}

func shiftTooFar(w uint) *Error {
	return newError(ErrShiftOverflow, "RHS of shift operation is greater than or equal to "+strconv.FormatUint(uint64(w), 10))
}

// Unary evaluates a unary operator component-wise. Negation wraps for signed
// integers (-MIN == MIN, as defined) and flips the sign of floats, including
// zero; unsigned and bool operands are invalid.
func Unary(op ir.UnaryOperator, v Value) (Value, error) {
	return lanes(v.Len(), func(i int) (ir.LiteralValue, error) {
		x := v.comps[i]
		s := ScalarOf(x)
		switch {
		case op == ir.UnaryNegate && isFloat(s):
			r, _ := makeFloat(s, -floatOf(x))
			return r, nil
		case op == ir.UnaryNegate && isSigned(s):
			return makeIntOrAbstract(s, -intBits(x)), nil
		case op == ir.UnaryLogicalNot && s.Kind == ir.ScalarBool:
			return !x.(ir.LiteralBool), nil
		case op == ir.UnaryBitwiseNot && isInteger(s):
			return makeIntOrAbstract(s, ^intBits(x)), nil
		}
		return nil, newError(ErrInvalidOperand, "Cannot apply the unary op to the argument")
	})
}
