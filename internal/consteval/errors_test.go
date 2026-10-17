package consteval

import (
	"errors"
	"fmt"
	"testing"
)

var allClasses = []error{
	ErrNotImplemented, ErrInvalidBinaryArgs, ErrInvalidOperand, ErrNotConst, ErrOverrideInConst, ErrOverflow,
	ErrOutOfDomain, ErrDivisionByZero, ErrRemainderByZero, ErrInvalidClamp, ErrBitRange,
	ErrShiftOverflow, ErrNotRepresentable,
}

// Rust naga's try_eval_and_append falls back to a run-time expression in a
// function body only for NotImplemented and InvalidBinaryOpArgs; every other
// evaluation error is a shader-creation error in every context.
func TestFallsBack(t *testing.T) {
	for _, class := range allClasses {
		want := errors.Is(class, ErrNotImplemented) || errors.Is(class, ErrInvalidBinaryArgs)
		err := error(newError(class, "msg"))
		if got := FallsBack(err); got != want {
			t.Errorf("FallsBack(%v) = %v, want %v", class, got, want)
		}
		// Wrapping by a caller keeps the classification.
		if got := FallsBack(fmt.Errorf("constant 'X': %w", err)); got != want {
			t.Errorf("FallsBack(wrapped %v) = %v, want %v", class, got, want)
		}
	}
	if FallsBack(errors.New("other")) {
		t.Error("FallsBack(unrelated error) = true")
	}
}

func TestErrorClass(t *testing.T) {
	err := error(newError(ErrOutOfDomain, "sqrt built-in function argument is out of valid range"))
	if !errors.Is(err, ErrOutOfDomain) || errors.Is(err, ErrOverflow) {
		t.Errorf("errors.Is: OutOfDomain=%v Overflow=%v", errors.Is(err, ErrOutOfDomain), errors.Is(err, ErrOverflow))
	}
	if err.Error() != "sqrt built-in function argument is out of valid range" {
		t.Errorf("Error() = %q", err.Error())
	}
	var e *Error
	if !errors.As(err, &e) || !errors.Is(e.Class, ErrOutOfDomain) {
		t.Errorf("errors.As: %+v", e)
	}
}
