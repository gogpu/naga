package consteval

import (
	"errors"
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
)

// Rust naga v30 does not evaluate these built-ins (NotImplemented), but the
// WGSL specification marks them @const. Expected values follow the spec's
// formulas, computed by hand.
func TestMathSpecOnly(t *testing.T) {
	vec4f := func(a, b, c, d float32) Value { return f32v(a, b, c, d) }
	runMath(t, []mathCase{
		{"mix", ir.MathMix, []Value{af(2), af(4), af(0.25)}, af(2.5)},
		{"mix f32 vector", ir.MathMix, []Value{f32v(0, 10), f32v(1, 20), f32v(0.5, 0.5)}, f32v(0.5, 15)},
		// t = clamp((x - e0) / (e1 - e0), 0, 1); t * t * (3 - 2t).
		{"smoothstep mid", ir.MathSmoothStep, []Value{af(0), af(1), af(0.5)}, af(0.5)},
		{"smoothstep quarter", ir.MathSmoothStep, []Value{af(0), af(4), af(1)}, af(0.15625)},
		{"smoothstep below", ir.MathSmoothStep, []Value{af(1), af(2), af(0)}, af(0)},
		{"smoothstep reversed edges", ir.MathSmoothStep, []Value{af(1), af(0), af(0.25)}, af(0.84375)},
		{"ldexp", ir.MathLdexp, []Value{af(1.5), Scalar(aint(3))}, af(12)},
		{"ldexp f32 negative exponent", ir.MathLdexp, []Value{f32s(1), i32s(-149)}, f32s(math.SmallestNonzeroFloat32)},
		{"quantizeToF16", ir.MathQuantizeF16, []Value{f32s(0.1)}, f32s(0.0999755859375)},
		{"quantizeToF16 vector", ir.MathQuantizeF16, []Value{f32v(1, 2049)}, f32v(1, 2048)},
		// floor(0.5 + 127 * clamp(e, -1, 1)) per byte, low byte first.
		{"pack4x8snorm", ir.MathPack4x8snorm, []Value{vec4f(1, -1, 0.5, 2)}, u32s(0x7F40817F)},
		{"pack4x8unorm", ir.MathPack4x8unorm, []Value{vec4f(1, 0, 0.5, -1)}, u32s(0x008000FF)},
		{"pack2x16snorm", ir.MathPack2x16snorm, []Value{f32v(1, -1)}, u32s(0x80017FFF)},
		{"pack2x16unorm", ir.MathPack2x16unorm, []Value{f32v(0.5, 1)}, u32s(0xFFFF8000)},
		{"pack2x16float", ir.MathPack2x16float, []Value{f32v(1, -2)}, u32s(0xC0003C00)},
		{"pack4xI8", ir.MathPack4xI8, []Value{Vector(ir.LiteralI32(1), ir.LiteralI32(-1), ir.LiteralI32(300), ir.LiteralI32(-129))}, u32s(0x7F2CFF01)},
		{"pack4xU8", ir.MathPack4xU8, []Value{Vector(ir.LiteralU32(1), ir.LiteralU32(255), ir.LiteralU32(256), ir.LiteralU32(0x1FF))}, u32s(0xFF00FF01)},
		{"pack4xI8Clamp", ir.MathPack4xI8Clamp, []Value{Vector(ir.LiteralI32(1), ir.LiteralI32(-1), ir.LiteralI32(300), ir.LiteralI32(-129))}, u32s(0x807FFF01)},
		{"pack4xU8Clamp", ir.MathPack4xU8Clamp, []Value{Vector(ir.LiteralU32(1), ir.LiteralU32(255), ir.LiteralU32(256), ir.LiteralU32(0x1FF))}, u32s(0xFFFFFF01)},
		// max(v / 127, -1) per signed byte.
		{"unpack4x8snorm", ir.MathUnpack4x8snorm, []Value{u32s(0x7F40817F)}, vec4f(1, -1, float32(64.0/127), 1)},
		{"unpack4x8snorm -128", ir.MathUnpack4x8snorm, []Value{u32s(0x00000080)}, vec4f(-1, 0, 0, 0)},
		{"unpack4x8unorm", ir.MathUnpack4x8unorm, []Value{u32s(0x008000FF)}, vec4f(1, 0, float32(128.0/255), 0)},
		{"unpack2x16snorm", ir.MathUnpack2x16snorm, []Value{u32s(0x80017FFF)}, f32v(1, -1)},
		{"unpack2x16unorm", ir.MathUnpack2x16unorm, []Value{u32s(0xFFFF8000)}, f32v(float32(32768.0/65535), 1)},
		{"unpack2x16float", ir.MathUnpack2x16float, []Value{u32s(0xC0003C00)}, f32v(1, -2)},
		{"unpack4xI8", ir.MathUnpack4xI8, []Value{u32s(0x7F2CFF01)}, Vector(ir.LiteralI32(1), ir.LiteralI32(-1), ir.LiteralI32(44), ir.LiteralI32(127))},
		{"unpack4xU8", ir.MathUnpack4xU8, []Value{u32s(0xFF00FF01)}, Vector(ir.LiteralU32(1), ir.LiteralU32(255), ir.LiteralU32(0), ir.LiteralU32(255))},
	})
}

func TestMathSpecOnlyErrors(t *testing.T) {
	tests := []struct {
		name  string
		fun   ir.MathFunction
		args  []Value
		class error
	}{
		{"smoothstep equal edges", ir.MathSmoothStep, []Value{af(1), af(1), af(0.5)}, ErrOutOfDomain},
		// Spec: e2 > bias + 1 is a shader-creation error (f32 bias 127).
		{"ldexp exponent above bias+1", ir.MathLdexp, []Value{f32s(1), i32s(129)}, ErrOutOfDomain},
		{"ldexp overflow", ir.MathLdexp, []Value{f32s(2), i32s(128)}, ErrOverflow},
		{"quantizeToF16 out of range", ir.MathQuantizeF16, []Value{f32s(65520)}, ErrOverflow},
		{"quantizeToF16 of abstract", ir.MathQuantizeF16, []Value{af(1)}, ErrInvalidOperand},
		{"pack2x16float out of range", ir.MathPack2x16float, []Value{f32v(1, 70000)}, ErrOverflow},
		{"unpack2x16float of infinity", ir.MathUnpack2x16float, []Value{u32s(0x7C00)}, ErrOverflow},
		{"pack4x8snorm of vec2", ir.MathPack4x8snorm, []Value{f32v(1, 2)}, ErrInvalidOperand},
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
		})
	}
}

// Modf, Frexp (struct results) and the matrix functions are evaluated by the
// arena evaluator, not the kernel.
func TestMathKernelLeavesStructAndMatrixResults(t *testing.T) {
	for _, fun := range []ir.MathFunction{ir.MathModf, ir.MathFrexp, ir.MathOuter, ir.MathInverse, ir.MathTranspose, ir.MathDeterminant} {
		args := make([]Value, mathArity(fun))
		for i := range args {
			args[i] = af(1)
		}
		if _, err := Math(fun, args...); !errors.Is(err, ErrNotImplemented) {
			t.Errorf("Math(%s): %v, want ErrNotImplemented", rustMathName(fun), err)
		}
	}
}

func TestF16BitsRoundTrip(t *testing.T) {
	for b := 0; b < 0x10000; b++ {
		bits := uint16(b)
		if bits&0x7C00 == 0x7C00 {
			continue // infinities and NaNs
		}
		x := f16FromBits(bits)
		if x != decodeF16(bits) {
			t.Fatalf("f16FromBits(%#04x) = %g, want %g", bits, x, decodeF16(bits))
		}
		if got := f16Bits(x); got != bits {
			t.Fatalf("f16Bits(%g) = %#04x, want %#04x", x, got, bits)
		}
	}
}

// The list Rust v30 returns NotImplemented for (constant_evaluator.rs
// 2063-2091): a function body keeps these as run-time expressions.
func TestRustEvaluates(t *testing.T) {
	notInRust := map[ir.MathFunction]bool{
		ir.MathModf: true, ir.MathFrexp: true, ir.MathLdexp: true, ir.MathOuter: true,
		ir.MathMix: true, ir.MathSmoothStep: true, ir.MathInverse: true, ir.MathTranspose: true,
		ir.MathDeterminant: true, ir.MathQuantizeF16: true,
	}
	for fun := ir.MathPack4x8snorm; fun <= ir.MathUnpack4xU8; fun++ {
		notInRust[fun] = true
	}
	for fun := ir.MathAbs; fun <= ir.MathUnpack4xU8; fun++ {
		if got := RustEvaluates(fun); got == notInRust[fun] {
			t.Errorf("RustEvaluates(%s) = %v", rustMathName(fun), got)
		}
	}
}
