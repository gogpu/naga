package consteval

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

func f32v(xs ...float32) Value {
	out := make([]ir.LiteralValue, len(xs))
	for i, x := range xs {
		out[i] = ir.LiteralF32(x)
	}
	if len(out) == 1 {
		return Scalar(out[0])
	}
	return Vector(out...)
}

func af(x float64) Value   { return Scalar(afloat(x)) }
func f32s(x float32) Value { return Scalar(ir.LiteralF32(x)) }
func i32s(x int32) Value   { return Scalar(ir.LiteralI32(x)) }

type mathCase struct {
	name string
	fun  ir.MathFunction
	args []Value
	want Value
}

func runMath(t *testing.T, tests []mathCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Math(tt.fun, tt.args...)
			if err != nil {
				t.Fatalf("Math: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Math = %v, want %v", got.Components(), tt.want.Components())
			}
		})
	}
}

// Expected f32 values are Rust naga v30 results for `const X: f32 = f(...)`;
// abstract arguments are evaluated in f64 and the result converted to f32.
func TestMathFloat(t *testing.T) {
	cast32 := func(v Value) Value {
		r, err := Cast(v, tF32)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	abstractThenF32 := func(fun ir.MathFunction, args ...float64) Value {
		vs := make([]Value, len(args))
		for i, a := range args {
			vs[i] = af(a)
		}
		r, err := Math(fun, vs...)
		if err != nil {
			t.Fatalf("Math(%v): %v", fun, err)
		}
		return cast32(r)
	}
	tests := []struct {
		name string
		got  Value
		want float32
	}{
		{"cos", abstractThenF32(ir.MathCos, 1), 0.5403023},
		{"cosh", abstractThenF32(ir.MathCosh, 1), 1.5430807},
		{"sin", abstractThenF32(ir.MathSin, 1), 0.84147096},
		{"tan", abstractThenF32(ir.MathTan, 1), 1.5574077},
		{"tanh", abstractThenF32(ir.MathTanh, 0.5), 0.46211717},
		{"acos", abstractThenF32(ir.MathAcos, 0.5), 1.0471976},
		{"atan", abstractThenF32(ir.MathAtan, 1), 0.7853982},
		{"atan2", abstractThenF32(ir.MathAtan2, 1, -1), 2.3561945},
		{"asinh", abstractThenF32(ir.MathAsinh, 1), 0.8813736},
		{"acosh", abstractThenF32(ir.MathAcosh, 2), 1.316958},
		{"atanh", abstractThenF32(ir.MathAtanh, 0.5), 0.54930615},
		{"radians", abstractThenF32(ir.MathRadians, 180), 3.1415927},
		{"degrees", abstractThenF32(ir.MathDegrees, 3.14159265358979), 180},
		{"ceil", abstractThenF32(ir.MathCeil, -1.5), -1},
		{"floor", abstractThenF32(ir.MathFloor, -1.5), -2},
		{"round ties to even down", abstractThenF32(ir.MathRound, 2.5), 2},
		{"round ties to even up", abstractThenF32(ir.MathRound, 3.5), 4},
		{"round negative", abstractThenF32(ir.MathRound, -2.5), -2},
		{"fract negative", abstractThenF32(ir.MathFract, -1.25), 0.75},
		{"trunc", abstractThenF32(ir.MathTrunc, -1.7), -1},
		{"exp", abstractThenF32(ir.MathExp, 1), 2.7182817},
		{"exp2", abstractThenF32(ir.MathExp2, 10), 1024},
		{"log", abstractThenF32(ir.MathLog, 2), 0.6931472},
		{"log2", abstractThenF32(ir.MathLog2, 8), 3},
		{"pow", abstractThenF32(ir.MathPow, 2, 10), 1024},
		{"sqrt", abstractThenF32(ir.MathSqrt, 2), 1.4142135},
		{"inverseSqrt", abstractThenF32(ir.MathInverseSqrt, 4), 0.5},
		{"sign", abstractThenF32(ir.MathSign, -3), -1},
		{"fma", abstractThenF32(ir.MathFma, 2, 3, 1), 7},
		{"step below edge", abstractThenF32(ir.MathStep, 1, 0.5), 0},
		{"step at edge", abstractThenF32(ir.MathStep, 1, 1), 1},
		{"saturate", abstractThenF32(ir.MathSaturate, 1.5), 1},
		{"abs", abstractThenF32(ir.MathAbs, -2.5), 2.5},
		{"max", abstractThenF32(ir.MathMax, 1.5, 2.5), 2.5},
		{"clamp", abstractThenF32(ir.MathClamp, 5, 0, 3), 3},
	}
	for _, tt := range tests {
		if got := tt.got.Component(0); got != ir.LiteralF32(tt.want) {
			t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestMathConcreteFloat(t *testing.T) {
	runMath(t, []mathCase{
		{"cos f32", ir.MathCos, []Value{f32s(1)}, f32s(0.5403023)},
		{"exp f32", ir.MathExp, []Value{f32s(1)}, f32s(2.7182817)},
		{"exp f32 near max", ir.MathExp, []Value{f32s(88)}, f32s(1.6516363e38)},
		{"sqrt f32", ir.MathSqrt, []Value{f32s(2)}, f32s(1.4142135)},
		{"tanh f32", ir.MathTanh, []Value{f32s(0.5)}, f32s(0.46211717)},
		{"pow f32", ir.MathPow, []Value{f32s(2), f32s(0.5)}, f32s(1.4142135)},
		{"asinh f32", ir.MathAsinh, []Value{f32s(1)}, f32s(0.8813736)},
		{"sqrt -0", ir.MathSqrt, []Value{f32s(float32(math.Copysign(0, -1)))}, f32s(float32(math.Copysign(0, -1)))},
		{"sign -0 is +0", ir.MathSign, []Value{f32s(float32(math.Copysign(0, -1)))}, f32s(0)},
		{"sqrt f16", ir.MathSqrt, []Value{Scalar(ir.LiteralF16(2))}, Scalar(ir.LiteralF16(1.4140625))},
		{"exp f16", ir.MathExp, []Value{Scalar(ir.LiteralF16(1))}, Scalar(ir.LiteralF16(2.71875))},
		{"round f16", ir.MathRound, []Value{Scalar(ir.LiteralF16(2.5))}, Scalar(ir.LiteralF16(2))},
		{"vector component-wise", ir.MathFloor, []Value{f32v(1.5, -1.5, 2)}, f32v(1, -2, 2)},
		{"vector two args", ir.MathMax, []Value{f32v(1, 5), f32v(3, 2)}, f32v(3, 5)},
	})
}

func TestMathInteger(t *testing.T) {
	runMath(t, []mathCase{
		{"abs i32 MIN wraps", ir.MathAbs, []Value{i32s(math.MinInt32)}, i32s(math.MinInt32)},
		{"abs u32 identity", ir.MathAbs, []Value{Scalar(ir.LiteralU32(7))}, Scalar(ir.LiteralU32(7))},
		{"abs aint", ir.MathAbs, []Value{Scalar(aint(-5))}, Scalar(aint(5))},
		{"min i32", ir.MathMin, []Value{i32s(3), i32s(-2)}, i32s(-2)},
		{"max u32", ir.MathMax, []Value{Scalar(ir.LiteralU32(3)), Scalar(ir.LiteralU32(4000000000))}, Scalar(ir.LiteralU32(4000000000))},
		{"clamp aint", ir.MathClamp, []Value{Scalar(aint(5)), Scalar(aint(0)), Scalar(aint(3))}, Scalar(aint(3))},
		{"sign i32", ir.MathSign, []Value{i32s(-3)}, i32s(-1)},
		{"sign aint zero", ir.MathSign, []Value{Scalar(aint(0))}, Scalar(aint(0))},
		{"min i64", ir.MathMin, []Value{Scalar(ir.LiteralI64(-1)), Scalar(ir.LiteralI64(1))}, Scalar(ir.LiteralI64(-1))},
	})
}

func TestMathErrors(t *testing.T) {
	tests := []struct {
		name  string
		fun   ir.MathFunction
		args  []Value
		class error
		msg   string
	}{
		{"acos domain", ir.MathAcos, []Value{af(1.5)}, ErrOutOfDomain, "acos built-in function argument is out of valid range"},
		{"asin domain", ir.MathAsin, []Value{af(-1.5)}, ErrOutOfDomain, "asin built-in function argument is out of valid range"},
		{"acosh domain", ir.MathAcosh, []Value{af(0.5)}, ErrOutOfDomain, "acosh built-in function argument is out of valid range"},
		{"atanh domain", ir.MathAtanh, []Value{af(1)}, ErrOutOfDomain, "atanh built-in function argument is out of valid range"},
		{"log zero", ir.MathLog, []Value{af(0)}, ErrOutOfDomain, "log built-in function argument is out of valid range"},
		{"log negative", ir.MathLog, []Value{af(-1)}, ErrOutOfDomain, "log built-in function argument is out of valid range"},
		{"log2 zero", ir.MathLog2, []Value{f32s(0)}, ErrOutOfDomain, "log2 built-in function argument is out of valid range"},
		{"pow negative base", ir.MathPow, []Value{af(-2), af(2)}, ErrOutOfDomain, "pow built-in function argument is out of valid range"},
		{"pow 0^0", ir.MathPow, []Value{af(0), af(0)}, ErrOutOfDomain, "pow built-in function argument is out of valid range"},
		{"sqrt negative", ir.MathSqrt, []Value{af(-1)}, ErrOutOfDomain, "sqrt built-in function argument is out of valid range"},
		// Rust v30 has no check and yields an infinite literal (quirk 2);
		// the spec domain of inverseSqrt excludes 0 and negatives (Tint).
		{"inverseSqrt zero (spec over Rust)", ir.MathInverseSqrt, []Value{f32s(0)}, ErrOutOfDomain, "inverseSqrt built-in function argument is out of valid range"},
		{"inverseSqrt negative (spec over Rust)", ir.MathInverseSqrt, []Value{af(-4)}, ErrOutOfDomain, "inverseSqrt built-in function argument is out of valid range"},
		{"exp f32 overflow", ir.MathExp, []Value{f32s(89)}, ErrOverflow, "exp operation overflowed"},
		{"exp f16 overflow", ir.MathExp, []Value{Scalar(ir.LiteralF16(20))}, ErrOverflow, "exp operation overflowed"},
		{"cosh overflow", ir.MathCosh, []Value{f32s(100)}, ErrOverflow, "cosh operation overflowed"},
		{"pow overflow", ir.MathPow, []Value{f32s(10), f32s(100)}, ErrOverflow, "pow operation overflowed"},
		{"abstract exp overflow", ir.MathExp, []Value{af(1000)}, ErrOverflow, "exp operation overflowed"},
		{"clamp low > high", ir.MathClamp, []Value{i32s(5), i32s(3), i32s(0)}, ErrInvalidClamp, "value of `low` is greater than `high` for clamp built-in function"},
		{"clamp low > high float", ir.MathClamp, []Value{f32s(5), f32s(3), f32s(0)}, ErrInvalidClamp, ""},
		{"arity", ir.MathAbs, []Value{f32s(1), f32s(2)}, ErrInvalidOperand, "Abs built-in function expects 1 arguments but 2 were supplied"},
		{"float function on ints", ir.MathCos, []Value{i32s(1)}, ErrInvalidOperand, "Cannot apply math function to type"},
		{"mixed argument types", ir.MathMax, []Value{f32s(1), i32s(1)}, ErrInvalidOperand, "Cannot apply math function to type"},
		{"vector and scalar", ir.MathMax, []Value{f32v(1, 2), f32s(1)}, ErrInvalidOperand, "Cannot apply math function to type"},
		{"sign of unsigned", ir.MathSign, []Value{Scalar(ir.LiteralU32(1))}, ErrInvalidOperand, ""},
		{"saturate of int", ir.MathSaturate, []Value{i32s(1)}, ErrInvalidOperand, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Math(tt.fun, tt.args...)
			if err == nil {
				t.Fatalf("Math = %v, want an error", got.Components())
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

// mathNames is built from the order of the ir.MathFunction constants; a
// reordering of ir must fail here, not mislabel messages.
func TestRustMathNames(t *testing.T) {
	for fun, want := range map[ir.MathFunction]string{
		ir.MathAbs: "Abs", ir.MathAtan2: "Atan2", ir.MathDot4U8Packed: "Dot4U8Packed",
		ir.MathSmoothStep: "SmoothStep", ir.MathQuantizeF16: "QuantizeToF16",
		ir.MathFirstLeadingBit: "FirstLeadingBit", ir.MathUnpack4xU8: "Unpack4xU8",
	} {
		if got := rustMathName(fun); got != want {
			t.Errorf("rustMathName(%d) = %q, want %q", fun, got, want)
		}
	}
	if got := rustMathName(ir.MathUnpack4xU8 + 1); got[:13] != "MathFunction(" {
		t.Errorf("past the last function: %q", got)
	}
}
