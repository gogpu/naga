package consteval

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

func bools(b ...bool) Value {
	out := make([]ir.LiteralValue, len(b))
	for i, x := range b {
		out[i] = ir.LiteralBool(x)
	}
	if len(out) == 1 {
		return Scalar(out[0])
	}
	return Vector(out...)
}

func ai(x int64) Value { return Scalar(aint(x)) }

// Expected values are Rust naga v30 results unless a comment says the spec
// decides.
func TestBinary(t *testing.T) {
	tests := []struct {
		name string
		op   ir.BinaryOperator
		a, b Value
		want Value
	}{
		{"i32 add wraps", ir.BinaryAdd, i32s(math.MaxInt32), i32s(1), i32s(math.MinInt32)},
		{"u32 sub wraps", ir.BinarySubtract, u32s(0), u32s(1), u32s(math.MaxUint32)},
		{"i32 remainder sign of dividend", ir.BinaryModulo, i32s(-7), i32s(3), i32s(-1)},
		{"f32 remainder", ir.BinaryModulo, f32s(-7.5), f32s(2), f32s(-1.5)},
		{"f32 mul", ir.BinaryMultiply, f32s(0.1), f32s(3), f32s(0.3)},
		{"abstract add", ir.BinaryAdd, ai(math.MaxInt64 - 1), ai(1), ai(math.MaxInt64)},
		{"abstract float", ir.BinaryDivide, af(1), af(4), af(0.25)},
		// Rust v30 has no 64-bit concrete arithmetic; the spec wraps it.
		{"i64 add wraps (spec)", ir.BinaryAdd, Scalar(ir.LiteralI64(math.MaxInt64)), Scalar(ir.LiteralI64(1)), Scalar(ir.LiteralI64(math.MinInt64))},
		{"i32 shl", ir.BinaryShiftLeft, i32s(1), u32s(30), i32s(1 << 30)},
		{"i32 shl to MIN", ir.BinaryShiftLeft, i32s(-1), u32s(31), i32s(math.MinInt32)},
		{"u32 shl", ir.BinaryShiftLeft, u32s(1), u32s(31), u32s(1 << 31)},
		{"i32 shr arithmetic", ir.BinaryShiftRight, i32s(-8), u32s(1), i32s(-4)},
		{"abstract shl", ir.BinaryShiftLeft, ai(1), u32s(62), ai(1 << 62)},
		{"abstract shl to MIN", ir.BinaryShiftLeft, ai(-1), u32s(63), ai(math.MinInt64)},
		// Spec: abstract >> fills with the sign bit for any count; Rust v30
		// gives 0 for a count >= 64 even for negative values (quirk 12).
		{"abstract shr past width negative (spec)", ir.BinaryShiftRight, ai(-8), u32s(70), ai(-1)},
		{"abstract shr past width positive", ir.BinaryShiftRight, ai(8), u32s(70), ai(0)},
		{"and", ir.BinaryAnd, u32s(0b1100), u32s(0b1010), u32s(0b1000)},
		{"xor", ir.BinaryExclusiveOr, i32s(5), i32s(3), i32s(6)},
		{"logical and", ir.BinaryLogicalAnd, bools(true), bools(false), bools(false)},
		{"bool and", ir.BinaryAnd, bools(true), bools(true), bools(true)},
		{"compare floats", ir.BinaryLess, af(1), af(2), bools(true)},
		{"equal bools", ir.BinaryEqual, bools(true), bools(true), bools(true)},
		{"vector + scalar", ir.BinaryAdd, Vector(ir.LiteralI32(1), ir.LiteralI32(2)), i32s(3), Vector(ir.LiteralI32(4), ir.LiteralI32(5))},
		{"scalar - vector keeps order", ir.BinarySubtract, af(2), afv(1, 5), afv(1, -3)},
		{"vector compare", ir.BinaryLess, Vector(ir.LiteralI32(1), ir.LiteralI32(2)), Vector(ir.LiteralI32(2), ir.LiteralI32(2)), bools(true, false)},
		{"vector shift", ir.BinaryShiftLeft, Vector(ir.LiteralU32(1), ir.LiteralU32(2)), Vector(ir.LiteralU32(1), ir.LiteralU32(2)), Vector(ir.LiteralU32(2), ir.LiteralU32(8))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Binary(tt.op, tt.a, tt.b)
			if err != nil {
				t.Fatalf("Binary: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Binary = %v, want %v", got.Components(), tt.want.Components())
			}
		})
	}
}

func TestBinaryErrors(t *testing.T) {
	tests := []struct {
		name  string
		op    ir.BinaryOperator
		a, b  Value
		class error
		msg   string
	}{
		{"i32 div by zero", ir.BinaryDivide, i32s(7), i32s(0), ErrDivisionByZero, "Division by zero"},
		{"u32 rem by zero", ir.BinaryModulo, u32s(7), u32s(0), ErrRemainderByZero, "Remainder by zero"},
		{"abstract rem by zero", ir.BinaryModulo, ai(7), ai(0), ErrRemainderByZero, "Remainder by zero"},
		{"i32 MIN / -1", ir.BinaryDivide, i32s(math.MinInt32), i32s(-1), ErrOverflow, "division operation overflowed"},
		{"i32 MIN % -1", ir.BinaryModulo, i32s(math.MinInt32), i32s(-1), ErrOverflow, "remainder operation overflowed"},
		{"abstract add overflow", ir.BinaryAdd, ai(math.MaxInt64), ai(1), ErrOverflow, "addition operation overflowed"},
		{"abstract mul overflow", ir.BinaryMultiply, ai(math.MaxInt64), ai(2), ErrOverflow, "multiplication operation overflowed"},
		{"abstract sub overflow", ir.BinarySubtract, ai(-math.MaxInt64), ai(2), ErrOverflow, "subtraction operation overflowed"},
		{"abstract MIN / -1", ir.BinaryDivide, ai(math.MinInt64), ai(-1), ErrOverflow, "division operation overflowed"},
		{"f32 overflow", ir.BinaryMultiply, f32s(1e38), f32s(10), ErrOverflow, "Float literal is infinite"},
		{"f32 0/0", ir.BinaryDivide, f32s(0), f32s(0), ErrOverflow, "Float literal is NaN"},
		{"abstract float div by zero", ir.BinaryDivide, af(1), af(0), ErrOverflow, "Divide operation overflowed"},
		{"abstract float mul overflow", ir.BinaryMultiply, af(1e308), af(10), ErrOverflow, "Multiply operation overflowed"},
		{"i32 shl overflow", ir.BinaryShiftLeft, i32s(1), u32s(31), ErrOverflow, "<< operation overflowed"},
		{"i32 shl by width", ir.BinaryShiftLeft, i32s(0), u32s(32), ErrOverflow, "<< operation overflowed"},
		{"u32 shl overflow", ir.BinaryShiftLeft, u32s(2), u32s(31), ErrOverflow, "<< operation overflowed"},
		{"u32 shl by width", ir.BinaryShiftLeft, u32s(1), u32s(32), ErrShiftOverflow, "RHS of shift operation is greater than or equal to 32"},
		{"u32 shr by width", ir.BinaryShiftRight, u32s(1), u32s(32), ErrShiftOverflow, "RHS of shift operation is greater than or equal to 32"},
		{"abstract shl overflow", ir.BinaryShiftLeft, ai(1), u32s(63), ErrOverflow, "<< operation overflowed"},
		{"abstract shl past width", ir.BinaryShiftLeft, ai(0), u32s(70), ErrOverflow, "<< operation overflowed"},
		{"mixed types", ir.BinaryAdd, ai(1), af(2.5), ErrInvalidBinaryArgs, "Cannot apply the binary op to the arguments"},
		{"bool xor", ir.BinaryExclusiveOr, bools(true), bools(false), ErrInvalidBinaryArgs, ""},
		{"bool ordering", ir.BinaryLess, bools(true), bools(false), ErrInvalidBinaryArgs, ""},
		{"vector shift by scalar", ir.BinaryShiftLeft, Vector(ir.LiteralU32(1), ir.LiteralU32(2)), u32s(1), ErrInvalidBinaryArgs, ""},
		{"vector logical and", ir.BinaryLogicalAnd, bools(true, false), bools(true, true), ErrInvalidBinaryArgs, ""},
		{"vector size mismatch", ir.BinaryAdd, afv(1, 2), afv(1, 2, 3), ErrInvalidBinaryArgs, ""},
		{"shift by signed", ir.BinaryShiftLeft, i32s(1), i32s(1), ErrInvalidBinaryArgs, ""},
		{"float bitwise", ir.BinaryAnd, f32s(1), f32s(1), ErrInvalidBinaryArgs, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Binary(tt.op, tt.a, tt.b)
			if err == nil {
				t.Fatalf("Binary = %v, want an error", got.Components())
			}
			if !errors.Is(err, tt.class) {
				t.Errorf("error %q is not %v", err, tt.class)
			}
			if tt.msg != "" && err.Error() != tt.msg {
				t.Errorf("error = %q\nwant    %q", err.Error(), tt.msg)
			}
		})
	}
}

func TestUnary(t *testing.T) {
	tests := []struct {
		name string
		op   ir.UnaryOperator
		v    Value
		want Value
	}{
		{"negate i32 MIN wraps", ir.UnaryNegate, i32s(math.MinInt32), i32s(math.MinInt32)},
		{"negate f32 zero", ir.UnaryNegate, f32s(0), f32s(float32(math.Copysign(0, -1)))},
		{"negate abstract", ir.UnaryNegate, ai(5), ai(-5)},
		{"not bool", ir.UnaryLogicalNot, bools(true, false), bools(false, true)},
		{"bitwise not i32", ir.UnaryBitwiseNot, i32s(5), i32s(-6)},
		{"bitwise not u32", ir.UnaryBitwiseNot, u32s(0), u32s(math.MaxUint32)},
		{"negate vector", ir.UnaryNegate, f32v(1, -2), f32v(-1, 2)},
	}
	for _, tt := range tests {
		got, err := Unary(tt.op, tt.v)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: Unary = %v, want %v", tt.name, got.Components(), tt.want.Components())
		}
	}
	for _, bad := range []struct {
		op ir.UnaryOperator
		v  Value
	}{
		{ir.UnaryNegate, u32s(1)},
		{ir.UnaryNegate, bools(true)},
		{ir.UnaryLogicalNot, i32s(1)},
		{ir.UnaryBitwiseNot, f32s(1)},
	} {
		if _, err := Unary(bad.op, bad.v); !errors.Is(err, ErrInvalidOperand) {
			t.Errorf("Unary(%v, %v): %v, want ErrInvalidOperand", bad.op, bad.v.Components(), err)
		}
	}
}
