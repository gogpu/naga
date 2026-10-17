package consteval

import "errors"

// Sentinel errors classify evaluation failures; every error the package
// returns is an *Error that wraps one of them, so callers test the class with
// errors.Is(err, consteval.ErrOverflow). They mirror the variants of Rust
// naga's ConstantEvaluatorError that a WGSL const-expression can produce.
var (
	// ErrNotImplemented: the operation is legal but cannot be evaluated
	// here. A function body keeps it as a run-time expression.
	ErrNotImplemented = errors.New("not implemented")
	// ErrInvalidBinaryArgs: operand types a binary operator does not accept
	// here. A function body keeps the expression and leaves it to
	// validation (Rust InvalidBinaryOpArgs).
	ErrInvalidBinaryArgs = errors.New("invalid binary operator arguments")
	// ErrInvalidOperand: an operand a math, conversion, unary or relational
	// operation does not accept (Rust InvalidMathArg, InvalidCastArg,
	// InvalidUnaryOpArg, InvalidRelationalArg). Unlike invalid binary
	// arguments it is an error in every context.
	ErrInvalidOperand = errors.New("invalid operand")
	// ErrNotConst: an operand is not a const-expression.
	ErrNotConst = errors.New("not a const-expression")
	// ErrOverrideInConst: an override-expression where a const-expression
	// is required.
	ErrOverrideInConst = errors.New("override-expression in a const-expression")
	// ErrOverflow: the result is not finite or does not fit its type.
	ErrOverflow = errors.New("overflow")
	// ErrOutOfDomain: an argument is outside the function's domain.
	ErrOutOfDomain = errors.New("argument out of domain")
	// ErrDivisionByZero: integer division by zero, or signed MIN / -1.
	ErrDivisionByZero = errors.New("division by zero")
	// ErrRemainderByZero: integer remainder by zero, or signed MIN % -1.
	ErrRemainderByZero = errors.New("remainder by zero")
	// ErrInvalidClamp: clamp with low > high.
	ErrInvalidClamp = errors.New("invalid clamp")
	// ErrBitRange: extractBits/insertBits offset + count exceeds the width.
	ErrBitRange = errors.New("bit range out of bounds")
	// ErrShiftOverflow: a shift amount or shifted value out of range.
	ErrShiftOverflow = errors.New("shift overflow")
	// ErrNotRepresentable: a value cannot be converted to the target type.
	ErrNotRepresentable = errors.New("value not representable")
)

// Error is an evaluation error: a class (one of the Err sentinels) and the
// full message, worded like Rust naga's where Rust reports the condition.
type Error struct {
	Class error
	Msg   string
}

func (e *Error) Error() string { return e.Msg }

// Unwrap returns the class, for errors.Is.
func (e *Error) Unwrap() error { return e.Class }

func newError(class error, msg string) *Error {
	return &Error{Class: class, Msg: msg}
}

// FallsBack reports whether, in a function body, an expression whose
// evaluation failed with err is kept as a run-time expression instead of
// being a shader-creation error. It is Rust naga's rule in
// try_eval_and_append: only NotImplemented and InvalidBinaryOpArgs fall back;
// overflow, domain, division and range errors are errors wherever the
// operands are constant.
func FallsBack(err error) bool {
	return errors.Is(err, ErrNotImplemented) || errors.Is(err, ErrInvalidBinaryArgs)
}
