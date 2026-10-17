package consteval

import (
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Math evaluates a math built-in function on constant arguments.
//
// Component-wise functions take scalars or vectors; every argument must have
// the same shape and scalar type (the caller converts abstract arguments and
// splats scalars first, as the WGSL overload rules require).
//
// Float results are computed in float64 and rounded once to the argument's
// type. Rust naga computes f32 and f16 in f32 libm; a transcendental result
// can therefore differ from Rust's in the last place, never by more.
func Math(fun ir.MathFunction, args ...Value) (Value, error) {
	if want := mathArity(fun); len(args) != want {
		return Value{}, newError(ErrInvalidOperand, rustMathName(fun)+" built-in function expects "+
			strconv.Itoa(want)+" arguments but "+strconv.Itoa(len(args))+" were supplied")
	}
	if f, ok := floatFuncs[fun]; ok {
		return componentWise(args, f.apply)
	}
	if f, ok := bitFuncs[fun]; ok {
		return f(args)
	}
	if f, ok := geometricFuncs[fun]; ok {
		return f(args)
	}
	switch fun {
	case ir.MathAbs:
		return componentWise(args, absLane)
	case ir.MathMin:
		return componentWise(args, func(xs []ir.LiteralValue) (ir.LiteralValue, error) {
			return pick(xs[0], xs[1], func(c int) bool { return c <= 0 })
		})
	case ir.MathMax:
		return componentWise(args, func(xs []ir.LiteralValue) (ir.LiteralValue, error) {
			return pick(xs[0], xs[1], func(c int) bool { return c >= 0 })
		})
	case ir.MathClamp:
		return componentWise(args, clampLane)
	case ir.MathSign:
		return componentWise(args, signLane)
	}
	return Value{}, newError(ErrNotImplemented, rustMathName(fun)+" built-in function")
}

// componentWise applies lane to each component of the arguments, which must
// all have the shape and scalar type of the first.
func componentWise(args []Value, lane func([]ir.LiteralValue) (ir.LiteralValue, error)) (Value, error) {
	first := args[0]
	for _, a := range args[1:] {
		if a.Len() != first.Len() || a.Scalar() != first.Scalar() {
			return Value{}, errInvalidMathArg()
		}
	}
	out := make([]ir.LiteralValue, first.Len())
	xs := make([]ir.LiteralValue, len(args))
	for i := range out {
		for j, a := range args {
			xs[j] = a.comps[i]
		}
		r, err := lane(xs)
		if err != nil {
			return Value{}, err
		}
		out[i] = r
	}
	return first.withComponents(out), nil
}

func errInvalidMathArg() *Error {
	return newError(ErrInvalidOperand, "Cannot apply math function to type")
}

// floatFunc is a float-only component-wise function.
type floatFunc struct {
	name   string                  // WGSL name, for messages
	domain func(xs []float64) bool // nil: every finite argument
	eval   func(xs []float64) float64
}

func (f floatFunc) apply(lits []ir.LiteralValue) (ir.LiteralValue, error) {
	s := ScalarOf(lits[0])
	if !isFloat(s) {
		return nil, errInvalidMathArg()
	}
	xs := make([]float64, len(lits))
	for i, l := range lits {
		xs[i] = floatOf(l)
	}
	if f.domain != nil && !f.domain(xs) {
		return nil, newError(ErrOutOfDomain, f.name+" built-in function argument is out of valid range")
	}
	r, ok := makeFloat(s, f.eval(xs))
	if !ok {
		return nil, newError(ErrOverflow, f.name+" operation overflowed")
	}
	return r, nil
}

func isFloat(s ir.ScalarType) bool {
	return s.Kind == ir.ScalarFloat || s.Kind == ir.ScalarAbstractFloat
}

// makeFloat rounds x to the float type s; ok is false if the result is not
// finite.
func makeFloat(s ir.ScalarType, x float64) (ir.LiteralValue, bool) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil, false
	}
	switch {
	case s.Kind == ir.ScalarAbstractFloat:
		return ir.LiteralAbstractFloat(x), true
	case s.Width == 2:
		r, ok := roundF16(x)
		return ir.LiteralF16(r), ok
	case s.Width == 4:
		r := float32(x)
		return ir.LiteralF32(r), !math.IsInf(float64(r), 0)
	default:
		return ir.LiteralF64(x), true
	}
}

func unary(f func(float64) float64) func([]float64) float64 {
	return func(xs []float64) float64 { return f(xs[0]) }
}

var floatFuncs = map[ir.MathFunction]floatFunc{
	ir.MathSaturate: {"saturate", nil, unary(func(x float64) float64 { return math.Min(math.Max(x, 0), 1) })},
	ir.MathCos:      {"cos", nil, unary(math.Cos)},
	ir.MathCosh:     {"cosh", nil, unary(math.Cosh)},
	ir.MathSin:      {"sin", nil, unary(math.Sin)},
	ir.MathSinh:     {"sinh", nil, unary(math.Sinh)},
	ir.MathTan:      {"tan", nil, unary(math.Tan)},
	ir.MathTanh:     {"tanh", nil, unary(math.Tanh)},
	ir.MathAcos:     {"acos", func(xs []float64) bool { return math.Abs(xs[0]) <= 1 }, unary(math.Acos)},
	ir.MathAsin:     {"asin", func(xs []float64) bool { return math.Abs(xs[0]) <= 1 }, unary(math.Asin)},
	ir.MathAtan:     {"atan", nil, unary(math.Atan)},
	ir.MathAtan2:    {"atan2", nil, func(xs []float64) float64 { return math.Atan2(xs[0], xs[1]) }},
	ir.MathAsinh:    {"asinh", nil, unary(math.Asinh)},
	ir.MathAcosh:    {"acosh", func(xs []float64) bool { return xs[0] >= 1 }, unary(math.Acosh)},
	ir.MathAtanh:    {"atanh", func(xs []float64) bool { return math.Abs(xs[0]) < 1 }, unary(math.Atanh)},
	ir.MathRadians:  {"radians", nil, unary(func(x float64) float64 { return x * (math.Pi / 180) })},
	ir.MathDegrees:  {"degrees", nil, unary(func(x float64) float64 { return x * (180 / math.Pi) })},
	ir.MathCeil:     {"ceil", nil, unary(math.Ceil)},
	ir.MathFloor:    {"floor", nil, unary(math.Floor)},
	ir.MathRound:    {"round", nil, unary(math.RoundToEven)},
	ir.MathFract:    {"fract", nil, unary(func(x float64) float64 { return x - math.Floor(x) })},
	ir.MathTrunc:    {"trunc", nil, unary(math.Trunc)},
	ir.MathExp:      {"exp", nil, unary(math.Exp)},
	ir.MathExp2:     {"exp2", nil, unary(math.Exp2)},
	ir.MathLog:      {"log", func(xs []float64) bool { return xs[0] > 0 }, unary(math.Log)},
	ir.MathLog2:     {"log2", func(xs []float64) bool { return xs[0] > 0 }, unary(math.Log2)},
	ir.MathPow:      {"pow", powDomain, func(xs []float64) float64 { return math.Pow(xs[0], xs[1]) }},
	ir.MathFma:      {"fma", nil, func(xs []float64) float64 { return math.FMA(xs[0], xs[1], xs[2]) }},
	ir.MathStep:     {"step", nil, func(xs []float64) float64 { return step(xs[0], xs[1]) }},
	ir.MathSqrt:     {"sqrt", func(xs []float64) bool { return xs[0] >= 0 }, unary(math.Sqrt)},
	// Rust v30 has no domain check here and yields an infinite or NaN
	// literal; the specification's domain is x > 0 (Tint checks it).
	ir.MathInverseSqrt: {"inverseSqrt", func(xs []float64) bool { return xs[0] > 0 },
		unary(func(x float64) float64 { return 1 / math.Sqrt(x) })},
}

// powDomain is Rust naga's: no negative base, and none of the indeterminate
// forms 1^inf, inf^0 and 0^0.
func powDomain(xs []float64) bool {
	b, e := xs[0], xs[1]
	return b >= 0 && !(b == 1 && math.IsInf(e, 0)) && !(math.IsInf(b, 0) && e == 0) && !(b == 0 && e == 0)
}

func step(edge, x float64) float64 {
	if edge <= x {
		return 1
	}
	return 0
}

// isSigned reports a signed integer type, concrete or abstract.
func isSigned(s ir.ScalarType) bool {
	return s.Kind == ir.ScalarSint || s.Kind == ir.ScalarAbstractInt
}

func isInteger(s ir.ScalarType) bool {
	return isSigned(s) || s.Kind == ir.ScalarUint
}

// compare returns -1, 0 or 1 comparing two numbers of one scalar type.
func compare(a, b ir.LiteralValue) int {
	s := ScalarOf(a)
	switch {
	case isFloat(s):
		x, y := floatOf(a), floatOf(b)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	case isSigned(s):
		x, y := i64FromU64(intBits(a)), i64FromU64(intBits(b))
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	default:
		x, y := intBits(a), intBits(b)
		switch {
		case x < y:
			return -1
		case x > y:
			return 1
		}
		return 0
	}
}

func isNumber(s ir.ScalarType) bool { return isFloat(s) || isInteger(s) }

// pick returns a if keepA(compare(a, b)), else b: min and max.
func pick(a, b ir.LiteralValue, keepA func(int) bool) (ir.LiteralValue, error) {
	if !isNumber(ScalarOf(a)) {
		return nil, errInvalidMathArg()
	}
	if keepA(compare(a, b)) {
		return a, nil
	}
	return b, nil
}

func clampLane(xs []ir.LiteralValue) (ir.LiteralValue, error) {
	e, lo, hi := xs[0], xs[1], xs[2]
	if !isNumber(ScalarOf(e)) {
		return nil, errInvalidMathArg()
	}
	if compare(lo, hi) > 0 {
		return nil, newError(ErrInvalidClamp, "value of `low` is greater than `high` for clamp built-in function")
	}
	switch {
	case compare(e, lo) < 0:
		return lo, nil
	case compare(e, hi) > 0:
		return hi, nil
	}
	return e, nil
}

// absLane: floats drop the sign; signed integers wrap (abs(MIN) == MIN, as
// WGSL and Rust's wrapping_abs define it); unsigned integers are unchanged.
func absLane(xs []ir.LiteralValue) (ir.LiteralValue, error) {
	x := xs[0]
	s := ScalarOf(x)
	switch {
	case isFloat(s):
		r, _ := makeFloat(s, math.Abs(floatOf(x)))
		return r, nil
	case isSigned(s):
		v := i64FromU64(intBits(x))
		if v < 0 {
			v = -v // wraps for the minimum, as defined
		}
		return makeIntOrAbstract(s, u64FromI64(v)), nil
	case s.Kind == ir.ScalarUint:
		return x, nil
	}
	return nil, errInvalidMathArg()
}

// signLane is -1, 0 or 1 for signed types; +0 for both zeros. Unsigned
// integers are rejected (WGSL defines sign only for signed types).
func signLane(xs []ir.LiteralValue) (ir.LiteralValue, error) {
	x := xs[0]
	s := ScalarOf(x)
	switch {
	case isFloat(s):
		v := floatOf(x)
		r := 0.0
		switch {
		case v > 0:
			r = 1
		case v < 0:
			r = -1
		}
		lit, _ := makeFloat(s, r)
		return lit, nil
	case isSigned(s):
		v := i64FromU64(intBits(x))
		r := int64(0)
		switch {
		case v > 0:
			r = 1
		case v < 0:
			r = -1
		}
		return makeIntOrAbstract(s, u64FromI64(r)), nil
	}
	return nil, errInvalidMathArg()
}

// makeIntOrAbstract is makeInt that also builds AbstractInt literals.
func makeIntOrAbstract(s ir.ScalarType, u uint64) ir.LiteralValue {
	if s.Kind == ir.ScalarAbstractInt {
		return ir.LiteralAbstractInt(i64FromU64(u))
	}
	return makeInt(s, u)
}

// mathArity is Rust naga's MathFunction::argument_count.
func mathArity(fun ir.MathFunction) int {
	switch fun {
	case ir.MathMin, ir.MathMax, ir.MathAtan2, ir.MathLdexp, ir.MathPow, ir.MathDot,
		ir.MathDot4I8Packed, ir.MathDot4U8Packed, ir.MathOuter, ir.MathCross,
		ir.MathDistance, ir.MathReflect, ir.MathStep:
		return 2
	case ir.MathClamp, ir.MathFaceForward, ir.MathRefract, ir.MathFma, ir.MathMix,
		ir.MathSmoothStep, ir.MathExtractBits:
		return 3
	case ir.MathInsertBits:
		return 4
	}
	return 1
}

// rustMathName is the Rust MathFunction variant name, which Rust's messages
// print ("Abs built-in function expects 1 arguments ...").
func rustMathName(fun ir.MathFunction) string {
	if n, ok := mathNames[fun]; ok {
		return n
	}
	return "MathFunction(" + strconv.Itoa(int(fun)) + ")"
}

var mathNames = func() map[ir.MathFunction]string {
	names := strings.Fields(`Abs Min Max Clamp Saturate Cos Cosh Sin Sinh Tan Tanh Acos Asin
		Atan Atan2 Asinh Acosh Atanh Radians Degrees Ceil Floor Round Fract Trunc Modf Frexp
		Ldexp Exp Exp2 Log Log2 Pow Dot Dot4I8Packed Dot4U8Packed Outer Cross Distance Length
		Normalize FaceForward Reflect Refract Sign Fma Mix Step SmoothStep Sqrt InverseSqrt
		Inverse Transpose Determinant QuantizeToF16 CountTrailingZeros CountLeadingZeros
		CountOneBits ReverseBits ExtractBits InsertBits FirstTrailingBit FirstLeadingBit
		Pack4x8snorm Pack4x8unorm Pack2x16snorm Pack2x16unorm Pack2x16float Pack4xI8 Pack4xU8
		Pack4xI8Clamp Pack4xU8Clamp Unpack4x8snorm Unpack4x8unorm Unpack2x16snorm
		Unpack2x16unorm Unpack2x16float Unpack4xI8 Unpack4xU8`)
	m := make(map[ir.MathFunction]string, len(names))
	for i, n := range names {
		m[ir.MathAbs+ir.MathFunction(i)] = n
	}
	return m
}()
