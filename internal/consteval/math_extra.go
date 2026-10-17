package consteval

import (
	"math"

	"github.com/gogpu/naga/ir"
)

// Built-ins the WGSL specification marks @const but Rust naga v30 does not
// evaluate (NotImplemented there). They follow the specification's
// definitions; float steps are rounded to the operand type like the other
// functions.
var specOnlyFuncs = map[ir.MathFunction]func([]Value) (Value, error){
	ir.MathMix:             mathMix,
	ir.MathSmoothStep:      mathSmoothStep,
	ir.MathLdexp:           mathLdexp,
	ir.MathQuantizeF16:     mathQuantizeToF16,
	ir.MathPack4x8snorm:    packNorm(4, 127, true),
	ir.MathPack4x8unorm:    packNorm(4, 255, false),
	ir.MathPack2x16snorm:   packNorm(2, 32767, true),
	ir.MathPack2x16unorm:   packNorm(2, 65535, false),
	ir.MathPack2x16float:   pack2x16float,
	ir.MathPack4xI8:        pack4x8int(true, false),
	ir.MathPack4xU8:        pack4x8int(false, false),
	ir.MathPack4xI8Clamp:   pack4x8int(true, true),
	ir.MathPack4xU8Clamp:   pack4x8int(false, true),
	ir.MathUnpack4x8snorm:  unpackNorm(4, 127, true),
	ir.MathUnpack4x8unorm:  unpackNorm(4, 255, false),
	ir.MathUnpack2x16snorm: unpackNorm(2, 32767, true),
	ir.MathUnpack2x16unorm: unpackNorm(2, 65535, false),
	ir.MathUnpack2x16float: unpack2x16float,
	ir.MathUnpack4xI8:      unpack4x8int(true),
	ir.MathUnpack4xU8:      unpack4x8int(false),
}

// RustEvaluates reports whether Rust naga v30 evaluates the math function.
// A function body keeps the others as run-time expressions, as Rust does, so
// that its IR matches Rust's; a const-expression evaluates them anyway.
func RustEvaluates(fun ir.MathFunction) bool {
	_, specOnly := specOnlyFuncs[fun]
	return !specOnly && !kernelNotImplemented(fun)
}

func kernelNotImplemented(fun ir.MathFunction) bool {
	switch fun {
	case ir.MathModf, ir.MathFrexp, ir.MathOuter, ir.MathInverse, ir.MathTranspose, ir.MathDeterminant:
		return true
	}
	return false
}

func outOfDomain(name string) *Error {
	return newError(ErrOutOfDomain, name+" built-in function argument is out of valid range")
}

// mathMix is e1 * (1 - e3) + e2 * e3, component-wise.
func mathMix(args []Value) (Value, error) {
	f, a, err := floats(false, args...)
	if err != nil {
		return Value{}, err
	}
	out := make([]float64, len(f[0]))
	for i := range out {
		out[i] = a.add(a.mul(f[0][i], a.sub(1, f[2][i])), a.mul(f[1][i], f[2][i]))
	}
	return floatResult(args[0], args[0].Scalar(), "mix", out...)
}

// mathSmoothStep is t * t * (3 - 2t) with t = clamp((x - e0) / (e1 - e0), 0,
// 1). Equal edges divide by zero: a domain error, as Tint reports it.
func mathSmoothStep(args []Value) (Value, error) {
	f, a, err := floats(false, args...)
	if err != nil {
		return Value{}, err
	}
	out := make([]float64, len(f[0]))
	for i := range out {
		e0, e1, x := f[0][i], f[1][i], f[2][i]
		if e0 == e1 {
			return Value{}, outOfDomain("smoothstep")
		}
		t := math.Min(math.Max(a.div(a.sub(x, e0), a.sub(e1, e0)), 0), 1)
		out[i] = a.mul(a.mul(t, t), a.sub(3, a.mul(2, t)))
	}
	return floatResult(args[0], args[0].Scalar(), "smoothstep", out...)
}

// mathLdexp is e1 * 2^e2. The specification makes e2 > bias + 1 a
// shader-creation error (128 for f32, 16 for f16, 1024 for abstract and f64).
func mathLdexp(args []Value) (Value, error) {
	e1, e2 := args[0], args[1]
	s := e1.Scalar()
	if !isFloat(s) || !isSigned(e2.Scalar()) || e2.Len() != e1.Len() {
		return Value{}, errInvalidMathArg()
	}
	limit := int64(1024)
	switch {
	case s.Kind == ir.ScalarFloat && s.Width == 4:
		limit = 128
	case s.Kind == ir.ScalarFloat && s.Width == 2:
		limit = 16
	}
	out := make([]float64, e1.Len())
	for i := range out {
		exp := i64FromU64(intBits(e2.comps[i]))
		if exp > limit {
			return Value{}, outOfDomain("ldexp")
		}
		out[i] = math.Ldexp(floatOf(e1.comps[i]), int(max(exp, -2000)))
	}
	return floatResult(e1, s, "ldexp", out...)
}

// mathQuantizeToF16 rounds f32 components to binary16 and back; a value
// outside the binary16 range is a shader-creation error.
func mathQuantizeToF16(args []Value) (Value, error) {
	v := args[0]
	if v.Scalar() != (ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}) {
		return Value{}, errInvalidMathArg()
	}
	out := make([]ir.LiteralValue, v.Len())
	for i, c := range v.comps {
		r, ok := roundF16(floatOf(c))
		if !ok {
			return Value{}, newError(ErrOverflow, "quantizeToF16 operation overflowed")
		}
		out[i] = ir.LiteralF32(r)
	}
	return v.withComponents(out), nil
}

var scalarF32 = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

// f32Vector returns the components of a vecN<f32> argument.
func f32Vector(v Value, n int) ([]float64, bool) {
	if v.Len() != n || v.IsScalar() || v.Scalar() != scalarF32 {
		return nil, false
	}
	out := make([]float64, n)
	for i, c := range v.comps {
		out[i] = floatOf(c)
	}
	return out, true
}

// packNorm packs n components as floor(0.5 + scale * clamp(e, lo, 1)) into
// 32/n-bit fields, the first component in the lowest bits.
func packNorm(n int, scale float64, signed bool) func([]Value) (Value, error) {
	return func(args []Value) (Value, error) {
		xs, ok := f32Vector(args[0], n)
		if !ok {
			return Value{}, errInvalidMathArg()
		}
		lo := 0.0
		if signed {
			lo = -1
		}
		bitsPer := 32 / n
		mask := uint32(1)<<bitsPer - 1
		var r uint32
		for i, x := range xs {
			q := math.Floor(0.5 + scale*math.Min(1, math.Max(lo, x)))
			r |= (u32Low(u64FromI64(int64(q))) & mask) << (bitsPer * i)
		}
		return Scalar(ir.LiteralU32(r)), nil
	}
}

// unpackNorm splits a u32 into n fields; signed fields are max(v / scale,
// -1), unsigned v / scale, computed in f32.
func unpackNorm(n int, scale float64, signed bool) func([]Value) (Value, error) {
	return func(args []Value) (Value, error) {
		u, ok := u32Scalar(args[0])
		if !ok {
			return Value{}, errInvalidMathArg()
		}
		bitsPer := 32 / n
		mask := uint32(1)<<bitsPer - 1
		out := make([]ir.LiteralValue, n)
		for i := range out {
			field := (u >> (bitsPer * i)) & mask
			v := float64(field)
			if signed && field>>(bitsPer-1) == 1 {
				v -= float64(uint32(1) << bitsPer)
			}
			out[i] = ir.LiteralF32(float32(math.Max(v/scale, -1)))
		}
		return Vector(out...), nil
	}
}

// pack4x8int packs the low 8 bits of four i32 or u32 components, clamping
// them to the 8-bit range first for the Clamp variants.
func pack4x8int(signed, clamp bool) func([]Value) (Value, error) {
	want := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	if signed {
		want.Kind = ir.ScalarSint
	}
	return func(args []Value) (Value, error) {
		v := args[0]
		if v.Len() != 4 || v.Scalar() != want {
			return Value{}, errInvalidMathArg()
		}
		var r uint32
		for i, c := range v.comps {
			b := intBits(c)
			if clamp {
				if signed {
					b = u64FromI64(min(max(i64FromU64(b), -128), 127))
				} else {
					b = min(b, 255)
				}
			}
			r |= (u32Low(b) & 0xFF) << (8 * i)
		}
		return Scalar(ir.LiteralU32(r)), nil
	}
}

// unpack4x8int splits a u32 into four bytes, sign-extended to i32 or
// zero-extended to u32.
func unpack4x8int(signed bool) func([]Value) (Value, error) {
	return func(args []Value) (Value, error) {
		u, ok := u32Scalar(args[0])
		if !ok {
			return Value{}, errInvalidMathArg()
		}
		out := make([]ir.LiteralValue, 4)
		for i := range out {
			b := (u >> (8 * i)) & 0xFF
			if signed {
				out[i] = ir.LiteralI32(int8(b)) //nolint:gosec // the byte's two's-complement value is the definition
			} else {
				out[i] = ir.LiteralU32(b)
			}
		}
		return Vector(out...), nil
	}
}

// pack2x16float converts two f32 components to binary16 bit patterns, the
// first in the low half; a value outside the binary16 range is an error.
func pack2x16float(args []Value) (Value, error) {
	xs, ok := f32Vector(args[0], 2)
	if !ok {
		return Value{}, errInvalidMathArg()
	}
	var r uint32
	for i, x := range xs {
		h, ok := roundF16(x)
		if !ok {
			return Value{}, newError(ErrOverflow, "pack2x16float operation overflowed")
		}
		r |= uint32(f16Bits(float64(h))) << (16 * i)
	}
	return Scalar(ir.LiteralU32(r)), nil
}

// unpack2x16float decodes two binary16 bit patterns; an infinity or NaN
// pattern makes the const-expression non-finite, an error.
func unpack2x16float(args []Value) (Value, error) {
	u, ok := u32Scalar(args[0])
	if !ok {
		return Value{}, errInvalidMathArg()
	}
	out := make([]float64, 2)
	for i := range out {
		out[i] = f16FromBits(uint16(u >> (16 * i))) //nolint:gosec // the 16-bit half is the definition
	}
	return floatResult(Vector(ir.LiteralF32(0), ir.LiteralF32(0)), scalarF32, "unpack2x16float", out...)
}

// f16Bits encodes a value that binary16 represents exactly.
func f16Bits(x float64) uint16 {
	var sign uint16
	if math.Signbit(x) {
		sign = 0x8000
		x = -x
	}
	if x == 0 {
		return sign
	}
	_, exp := math.Frexp(x) // x = frac * 2^exp, frac in [0.5, 1)
	e := exp - 1
	if e < -14 { // subnormal: x = m * 2^-24
		return sign | uint16(math.Ldexp(x, 24)) // m < 1024
	}
	m := math.Ldexp(x, 10-e) - 1024            // 10 fraction bits
	return sign | uint16(e+15)<<10 | uint16(m) //nolint:gosec // e in [-14, 15], m < 1024
}

// f16FromBits decodes a binary16 bit pattern.
func f16FromBits(b uint16) float64 {
	sign := 1.0
	if b&0x8000 != 0 {
		sign = -1
	}
	exp := int(b>>10) & 0x1F
	frac := float64(b & 0x3FF)
	switch exp {
	case 0:
		return sign * math.Ldexp(frac, -24)
	case 0x1F:
		if frac == 0 {
			return sign * math.Inf(1)
		}
		return math.NaN()
	}
	return sign * math.Ldexp(1024+frac, exp-25)
}
