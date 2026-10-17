package consteval

import (
	"errors"
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
)

func afv(xs ...float64) Value {
	out := make([]ir.LiteralValue, len(xs))
	for i, x := range xs {
		out[i] = afloat(x)
	}
	return Vector(out...)
}

// Expected values are Rust naga v30 results. f32 arguments check that each
// step is rounded to f32, as Rust computes them (reflect gives 0.75600004,
// not the exactly rounded 0.756).
func TestMathGeometric(t *testing.T) {
	negZero := float32(math.Copysign(0, -1))
	runMath(t, []mathCase{
		{"dot abstract", ir.MathDot, []Value{afv(1, 2, 3), afv(4, 5, 6)}, af(32)},
		{"dot i32", ir.MathDot, []Value{Vector(ir.LiteralI32(2), ir.LiteralI32(3)), Vector(ir.LiteralI32(4), ir.LiteralI32(-5))}, i32s(-7)},
		{"dot u32 wraps", ir.MathDot, []Value{Vector(ir.LiteralU32(4000000000), ir.LiteralU32(1)), Vector(ir.LiteralU32(2), ir.LiteralU32(1))}, u32s(3705032705)},
		{"dot f32 stepwise", ir.MathDot, []Value{f32v(0.1, 0.2), f32v(0.3, 0.7)}, f32s(0.17)},
		{"cross abstract", ir.MathCross, []Value{afv(1, 0, 0), afv(0, 1, 0)}, afv(0, 0, 1)},
		{"cross f32", ir.MathCross, []Value{f32v(1, 2, 3), f32v(4, 5, 6)}, f32v(-3, 6, -3)},
		{"length abstract", ir.MathLength, []Value{afv(3, 4)}, af(5)},
		{"length scalar", ir.MathLength, []Value{af(-2.5)}, af(2.5)},
		{"length f32 stepwise", ir.MathLength, []Value{f32v(0.1, 0.2)}, f32s(0.22360681)},
		{"distance", ir.MathDistance, []Value{afv(1, 1), afv(4, 5)}, af(5)},
		{"distance scalar", ir.MathDistance, []Value{af(1), af(4.5)}, af(3.5)},
		{"normalize abstract", ir.MathNormalize, []Value{afv(3, 4)}, afv(0.6, 0.8)},
		{"normalize f32", ir.MathNormalize, []Value{f32v(0.1, 0.2)}, f32v(0.4472136, 0.8944272)},
		{"faceForward d >= 0 negates", ir.MathFaceForward, []Value{f32v(0, 1), f32v(0, 1), f32v(0, 1)}, f32v(negZero, -1)},
		{"faceForward d < 0 keeps", ir.MathFaceForward, []Value{f32v(0, 1), f32v(0, -1), f32v(0, 1)}, f32v(0, 1)},
		{"reflect", ir.MathReflect, []Value{f32v(1, -1), f32v(0, 1)}, f32v(1, 1)},
		{"reflect f32 stepwise", ir.MathReflect, []Value{f32v(0.3, -0.7), f32v(0.6, 0.8)}, f32v(0.75600004, -0.09200001)},
		{"refract", ir.MathRefract, []Value{f32v(0, -1), f32v(0, 1), f32s(0.5)}, f32v(0, -1)},
		{"refract total internal reflection", ir.MathRefract, []Value{f32v(0.7071, -0.7071), f32v(0, 1), f32s(2)}, f32v(0, 0)},
		{"refract f32 stepwise", ir.MathRefract, []Value{f32v(0.6, -0.8), f32v(0, 1), f32s(0.75)}, f32v(0.45000002, -0.89302856)},
	})
}

func TestMathGeometricErrors(t *testing.T) {
	tests := []struct {
		name  string
		fun   ir.MathFunction
		args  []Value
		class error
		msg   string
	}{
		{"normalize zero", ir.MathNormalize, []Value{afv(0, 0)}, ErrOutOfDomain, "normalize built-in function argument is out of valid range"},
		{"length overflow", ir.MathLength, []Value{f32v(3e38, 3e38)}, ErrOverflow, "length operation overflowed"},
		{"dot overflow", ir.MathDot, []Value{f32v(3e38, 1), f32v(3e38, 1)}, ErrOverflow, "in mathfunction::dot operation overflowed"},
		{"dot abstract int overflow", ir.MathDot, []Value{Vector(aint(math.MaxInt64), aint(1)), Vector(aint(2), aint(1))}, ErrOverflow, "in dot built-in operation overflowed"},
		// Rust v30 does not check distance (quirk 3); a non-finite result
		// is a shader-creation error per the spec.
		{"distance overflow (spec over Rust)", ir.MathDistance, []Value{f32v(3e38, 0), f32v(-3e38, 0)}, ErrOverflow, ""},
		{"dot of scalars", ir.MathDot, []Value{af(1), af(2)}, ErrInvalidOperand, ""},
		{"dot length mismatch", ir.MathDot, []Value{afv(1, 2), afv(1, 2, 3)}, ErrInvalidOperand, ""},
		{"cross vec2", ir.MathCross, []Value{afv(1, 2), afv(3, 4)}, ErrInvalidOperand, ""},
		{"cross ints", ir.MathCross, []Value{Vector(aint(1), aint(0), aint(0)), Vector(aint(0), aint(1), aint(0))}, ErrInvalidOperand, ""},
		{"refract eta vector", ir.MathRefract, []Value{f32v(0, -1), f32v(0, 1), f32v(1, 1)}, ErrInvalidOperand, ""},
		{"length of ints", ir.MathLength, []Value{Vector(ir.LiteralI32(1), ir.LiteralI32(2))}, ErrInvalidOperand, ""},
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
