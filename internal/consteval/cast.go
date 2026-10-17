package consteval

import (
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Cast converts each component of v to the scalar type to, as the WGSL
// conversion built-ins (value constructors such as i32(e)) and automatic
// conversions of abstract values do:
//
//   - integer to integer of the same width reinterprets the bits;
//   - float to integer truncates toward zero and saturates to the value of
//     the target closest to the result that the source float type can
//     represent exactly (WGSL "scalar floating point to integral conversion");
//   - integer or float to float rounds to nearest, ties to even;
//   - anything to bool is "not zero", bool to a number is 0 or 1.
//
// Converting an abstract value is range-checked: a value the target cannot
// represent is a NotRepresentable error, with Rust naga's message. A
// conversion whose float result would be infinite is the same error for every
// source type (the specification forbids a non-finite const-expression; Rust
// naga v30 instead yields an f16 infinity from integer and f32 sources).
// Concrete values never convert to abstract types (InvalidArgs).
func Cast(v Value, to ir.ScalarType) (Value, error) {
	out := make([]ir.LiteralValue, v.Len())
	for i, c := range v.comps {
		r, err := castScalar(c, to)
		if err != nil {
			return Value{}, err
		}
		out[i] = r
	}
	return v.withComponents(out), nil
}

func castScalar(lit ir.LiteralValue, to ir.ScalarType) (ir.LiteralValue, error) {
	from := ScalarOf(lit)
	if from == to {
		return lit, nil
	}
	switch to.Kind {
	case ir.ScalarBool:
		return ir.LiteralBool(!isZero(lit)), nil
	case ir.ScalarSint, ir.ScalarUint:
		return castToInt(lit, from, to)
	case ir.ScalarFloat:
		return castToFloat(lit, to)
	case ir.ScalarAbstractFloat:
		if v, ok := lit.(ir.LiteralAbstractInt); ok {
			return ir.LiteralAbstractFloat(float64(v)), nil
		}
	}
	return nil, newError(ErrInvalidArgs, "cannot convert "+typeName(from)+" to "+typeName(to))
}

func isZero(lit ir.LiteralValue) bool {
	switch v := lit.(type) {
	case ir.LiteralBool:
		return !bool(v)
	case ir.LiteralF16, ir.LiteralF32, ir.LiteralF64, ir.LiteralAbstractFloat:
		return floatOf(lit) == 0
	}
	return intBits(lit) == 0
}

func castToInt(lit ir.LiteralValue, from, to ir.ScalarType) (ir.LiteralValue, error) {
	switch v := lit.(type) {
	case ir.LiteralBool:
		if v {
			return makeInt(to, 1), nil
		}
		return makeInt(to, 0), nil
	case ir.LiteralAbstractInt:
		if !intFits(int64(v), to) {
			return nil, notRepresentable(to, strconv.FormatInt(int64(v), 10))
		}
		return makeInt(to, u64FromI64(int64(v))), nil
	case ir.LiteralI32, ir.LiteralU32, ir.LiteralI64, ir.LiteralU64:
		// Same-width reinterpretation, or extension from 32 to 64 bits by
		// the source's signedness; narrowing keeps the low bits.
		return makeInt(to, intBits(lit)), nil
	}
	if from.Kind == ir.ScalarFloat || from.Kind == ir.ScalarAbstractFloat {
		return makeIntFromFloat(floatOf(lit), from, to), nil
	}
	return nil, newError(ErrInvalidArgs, "cannot convert "+typeName(from)+" to "+typeName(to))
}

// makeIntFromFloat implements the specification's float to integer
// conversion: truncate, then clamp to the target range narrowed to values the
// source float type represents exactly.
func makeIntFromFloat(x float64, from, to ir.ScalarType) ir.LiteralValue {
	x = math.Trunc(x)
	lo, hi := intRange(to)
	lo, hi = towardZeroIn(lo, from), towardZeroIn(hi, from)
	switch {
	case x <= lo:
		x = lo
	case x >= hi:
		x = hi
	}
	if to.Kind == ir.ScalarSint {
		return makeInt(to, u64FromI64(int64(x)))
	}
	return makeInt(to, uint64(x))
}

// intRange returns the bounds of an integer type as float64 values; the
// upper bounds of 64-bit types round up (2^63, 2^64) and are narrowed by
// towardZeroIn.
func intRange(t ir.ScalarType) (lo, hi float64) {
	switch {
	case t.Kind == ir.ScalarSint && t.Width == 4:
		return math.MinInt32, math.MaxInt32
	case t.Kind == ir.ScalarUint && t.Width == 4:
		return 0, math.MaxUint32
	case t.Kind == ir.ScalarSint:
		return math.MinInt64, math.Ldexp(1, 63)
	default:
		return 0, math.Ldexp(1, 64)
	}
}

// towardZeroIn returns the value of the float type t nearest to bound that is
// not farther from zero than bound. bound is a power of two or an integer
// below 2^53, so it is exact in float64; only its rounding into a narrower
// type, or the 64-bit upper bounds 2^63 and 2^64, need narrowing.
func towardZeroIn(bound float64, t ir.ScalarType) float64 {
	r := roundTo(bound, t)
	isPow2Limit := bound == math.Ldexp(1, 63) || bound == math.Ldexp(1, 64)
	if math.Abs(r) > math.Abs(bound) || (isPow2Limit && r == bound) {
		r = nextToward0(r, t)
	}
	return r
}

// roundTo rounds x to the float type t (f16 values cannot occur here: the
// f16 range is inside every integer range, so the bounds are never reached).
func roundTo(x float64, t ir.ScalarType) float64 {
	if t.Kind == ir.ScalarFloat && t.Width == 4 {
		return float64(float32(x))
	}
	return x
}

func nextToward0(x float64, t ir.ScalarType) float64 {
	if t.Kind == ir.ScalarFloat && t.Width == 4 {
		return float64(math.Nextafter32(float32(x), 0))
	}
	return math.Nextafter(x, 0)
}

func intFits(v int64, t ir.ScalarType) bool {
	switch {
	case t.Kind == ir.ScalarSint && t.Width == 4:
		return v >= math.MinInt32 && v <= math.MaxInt32
	case t.Kind == ir.ScalarUint && t.Width == 4:
		return v >= 0 && v <= math.MaxUint32
	case t.Kind == ir.ScalarSint:
		return true
	default:
		return v >= 0
	}
}

// makeInt returns the integer literal of type t holding the low bits of u.
func makeInt(t ir.ScalarType, u uint64) ir.LiteralValue {
	switch {
	case t.Kind == ir.ScalarSint && t.Width == 4:
		return ir.LiteralI32(i32Low(u))
	case t.Kind == ir.ScalarUint && t.Width == 4:
		return ir.LiteralU32(u32Low(u))
	case t.Kind == ir.ScalarSint:
		return ir.LiteralI64(i64FromU64(u))
	default:
		return ir.LiteralU64(u)
	}
}

// intBits returns the bits of an integer literal, extended to 64 bits by its
// signedness.
func intBits(lit ir.LiteralValue) uint64 {
	switch v := lit.(type) {
	case ir.LiteralI32:
		return u64FromI64(int64(v))
	case ir.LiteralU32:
		return uint64(v)
	case ir.LiteralI64:
		return u64FromI64(int64(v))
	case ir.LiteralU64:
		return uint64(v)
	case ir.LiteralAbstractInt:
		return u64FromI64(int64(v))
	}
	return 0
}

func castToFloat(lit ir.LiteralValue, to ir.ScalarType) (ir.LiteralValue, error) {
	var x float64
	exact := ""
	switch v := lit.(type) {
	case ir.LiteralBool:
		if v {
			x = 1
		}
	case ir.LiteralAbstractInt:
		x, exact = float64(v), strconv.FormatInt(int64(v), 10)
	case ir.LiteralI32, ir.LiteralI64:
		x = float64(i64FromU64(intBits(lit)))
	case ir.LiteralU32, ir.LiteralU64:
		x = float64(intBits(lit))
	case ir.LiteralAbstractFloat:
		x, exact = float64(v), rustFloat(float64(v))
	default:
		x = floatOf(lit)
	}
	switch to.Width {
	case 2:
		if r, ok := roundF16(x); ok {
			return ir.LiteralF16(r), nil
		}
	case 4:
		if r := float32(x); !math.IsInf(float64(r), 0) {
			return ir.LiteralF32(r), nil
		}
	case 8:
		return ir.LiteralF64(x), nil
	}
	if exact == "" {
		exact = rustFloat(x)
	}
	return nil, notRepresentable(to, exact)
}

func floatOf(lit ir.LiteralValue) float64 {
	switch v := lit.(type) {
	case ir.LiteralF16:
		return float64(v)
	case ir.LiteralF32:
		return float64(v)
	case ir.LiteralF64:
		return float64(v)
	case ir.LiteralAbstractFloat:
		return float64(v)
	}
	return 0
}

func notRepresentable(to ir.ScalarType, value string) *Error {
	return newError(ErrNotRepresentable, "the concrete type `"+typeName(to)+
		"` cannot represent the abstract value `"+value+"` accurately")
}

// typeName returns the WGSL name of a scalar type.
func typeName(t ir.ScalarType) string {
	switch t.Kind {
	case ir.ScalarBool:
		return "bool"
	case ir.ScalarSint:
		return "i" + strconv.Itoa(int(t.Width)*8)
	case ir.ScalarUint:
		return "u" + strconv.Itoa(int(t.Width)*8)
	case ir.ScalarFloat:
		return "f" + strconv.Itoa(int(t.Width)*8)
	case ir.ScalarAbstractInt:
		return "AbstractInt"
	case ir.ScalarAbstractFloat:
		return "AbstractFloat"
	}
	return "unknown"
}

// rustFloat formats x like Rust's {:?} for f64, which naga's messages use:
// the shortest representation that round-trips, in exponent form when
// |x| < 1e-4 or |x| >= 1e16, otherwise in decimal form with at least one
// fractional digit ("65520.0", "1e39", "3.4028236e38").
func rustFloat(x float64) string {
	if a := math.Abs(x); a != 0 && (a < 1e-4 || a >= 1e16) {
		s := strconv.FormatFloat(x, 'e', -1, 64) // "3.4028236e+38"
		mant, exp, _ := strings.Cut(s, "e")
		exp = strings.TrimPrefix(exp, "+")
		neg := strings.HasPrefix(exp, "-")
		exp = strings.TrimLeft(strings.TrimPrefix(exp, "-"), "0")
		if neg {
			exp = "-" + exp
		}
		return mant + "e" + exp
	}
	s := strconv.FormatFloat(x, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}
