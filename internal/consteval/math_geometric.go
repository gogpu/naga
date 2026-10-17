package consteval

import (
	"math"

	"github.com/gogpu/naga/ir"
)

// arith performs float arithmetic in one WGSL float type, rounding after
// every operation as a shader (and Rust naga) computes in that type. Each
// operation is done in float64 and rounded once; for + - * / and sqrt that
// is the IEEE result of the narrower type.
type arith struct {
	s ir.ScalarType
}

func (a arith) round(x float64) float64 {
	switch {
	case a.s.Kind == ir.ScalarAbstractFloat || a.s.Width == 8:
		return x
	case a.s.Width == 4:
		return float64(float32(x))
	default:
		r, ok := roundF16(x)
		if !ok {
			return math.Inf(int(math.Copysign(1, x)))
		}
		return float64(r)
	}
}

func (a arith) add(x, y float64) float64 { return a.round(x + y) }
func (a arith) sub(x, y float64) float64 { return a.round(x - y) }
func (a arith) mul(x, y float64) float64 { return a.round(x * y) }
func (a arith) div(x, y float64) float64 { return a.round(x / y) }
func (a arith) sqrt(x float64) float64   { return a.round(math.Sqrt(x)) }

// dot sums the products from the first component on, rounding each step.
func (a arith) dot(x, y []float64) float64 {
	acc := 0.0
	for i := range x {
		acc = a.add(acc, a.mul(x[i], y[i]))
	}
	return acc
}

// length is sqrt of the sum of squares, rounded per step; a scalar's
// length is its absolute value.
func (a arith) length(x []float64) float64 {
	if len(x) == 1 {
		return math.Abs(x[0])
	}
	return a.sqrt(a.dot(x, x))
}

var geometricFuncs = map[ir.MathFunction]func([]Value) (Value, error){
	ir.MathDot:         mathDot,
	ir.MathCross:       mathCross,
	ir.MathLength:      mathLength,
	ir.MathDistance:    mathDistance,
	ir.MathNormalize:   mathNormalize,
	ir.MathFaceForward: mathFaceForward,
	ir.MathReflect:     mathReflect,
	ir.MathRefract:     mathRefract,
}

// floats returns the components of float values that all have the scalar
// type and length of the first; vectorOnly rejects scalars.
func floats(vectorOnly bool, vs ...Value) ([][]float64, arith, error) {
	first := vs[0]
	s := first.Scalar()
	if !isFloat(s) || (vectorOnly && first.IsScalar()) {
		return nil, arith{}, errInvalidMathArg()
	}
	out := make([][]float64, len(vs))
	for i, v := range vs {
		if v.Scalar() != s || v.Len() != first.Len() {
			return nil, arith{}, errInvalidMathArg()
		}
		out[i] = make([]float64, v.Len())
		for j, c := range v.comps {
			out[i][j] = floatOf(c)
		}
	}
	return out, arith{s}, nil
}

// floatResult builds a value of type s from xs; a non-finite component is
// an overflow of the named operation.
func floatResult(like Value, s ir.ScalarType, op string, xs ...float64) (Value, error) {
	out := make([]ir.LiteralValue, len(xs))
	for i, x := range xs {
		lit, ok := makeFloat(s, x)
		if !ok {
			return Value{}, newError(ErrOverflow, op+" operation overflowed")
		}
		out[i] = lit
	}
	if len(out) == 1 {
		return Scalar(out[0]), nil
	}
	return like.withComponents(out), nil
}

func mathDot(args []Value) (Value, error) {
	x, y := args[0], args[1]
	if x.IsScalar() || x.Len() != y.Len() || x.Scalar() != y.Scalar() {
		return Value{}, errInvalidMathArg()
	}
	s := x.Scalar()
	switch {
	case isFloat(s):
		f, a, err := floats(true, x, y)
		if err != nil {
			return Value{}, err
		}
		return floatResult(x, s, "in mathfunction::dot", a.dot(f[0], f[1]))
	case s.Kind == ir.ScalarAbstractInt:
		var acc int64
		for i := range x.comps {
			p, ok := mulInt64(int64(x.comps[i].(ir.LiteralAbstractInt)), int64(y.comps[i].(ir.LiteralAbstractInt)))
			if ok {
				acc, ok = addInt64(acc, p)
			}
			if !ok {
				return Value{}, newError(ErrOverflow, "in dot built-in operation overflowed")
			}
		}
		return Scalar(ir.LiteralAbstractInt(acc)), nil
	case isInteger(s):
		var acc uint64 // concrete integers wrap
		for i := range x.comps {
			acc += intBits(x.comps[i]) * intBits(y.comps[i])
		}
		return Scalar(makeInt(s, acc)), nil
	}
	return Value{}, errInvalidMathArg()
}

func mulInt64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	p := a * b
	if p/b != a || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		return 0, false
	}
	return p, true
}

func addInt64(a, b int64) (int64, bool) {
	s := a + b
	if (b > 0 && s < a) || (b < 0 && s > a) {
		return 0, false
	}
	return s, true
}

func mathCross(args []Value) (Value, error) {
	f, a, err := floats(true, args[0], args[1])
	if err != nil || args[0].Len() != 3 {
		return Value{}, errInvalidMathArg()
	}
	x, y := f[0], f[1]
	return floatResult(args[0], args[0].Scalar(), "cross",
		a.sub(a.mul(x[1], y[2]), a.mul(x[2], y[1])),
		a.sub(a.mul(x[2], y[0]), a.mul(x[0], y[2])),
		a.sub(a.mul(x[0], y[1]), a.mul(x[1], y[0])))
}

func mathLength(args []Value) (Value, error) {
	f, a, err := floats(false, args[0])
	if err != nil {
		return Value{}, err
	}
	return floatResult(args[0], args[0].Scalar(), "length", a.length(f[0]))
}

func mathDistance(args []Value) (Value, error) {
	f, a, err := floats(false, args[0], args[1])
	if err != nil {
		return Value{}, err
	}
	d := make([]float64, len(f[0]))
	for i := range d {
		d[i] = a.sub(f[0][i], f[1][i])
	}
	return floatResult(args[0], args[0].Scalar(), "distance", a.length(d))
}

func mathNormalize(args []Value) (Value, error) {
	f, a, err := floats(false, args[0])
	if err != nil {
		return Value{}, err
	}
	l := a.length(f[0])
	if math.IsInf(l, 0) || math.IsNaN(l) {
		return Value{}, newError(ErrOverflow, "normalize operation overflowed")
	}
	if l == 0 {
		return Value{}, newError(ErrOutOfDomain, "normalize built-in function argument is out of valid range")
	}
	out := make([]float64, len(f[0]))
	for i, x := range f[0] {
		out[i] = a.div(x, l)
	}
	return floatResult(args[0], args[0].Scalar(), "normalize", out...)
}

// mathFaceForward(e1, e2, e3) is e1 if dot(e2, e3) < 0, else -e1.
func mathFaceForward(args []Value) (Value, error) {
	f, a, err := floats(true, args[0], args[1], args[2])
	if err != nil {
		return Value{}, err
	}
	d := a.dot(f[1], f[2])
	if math.IsInf(d, 0) || math.IsNaN(d) {
		return Value{}, newError(ErrOverflow, "in mathfunction::faceforward operation overflowed")
	}
	out := make([]float64, len(f[0]))
	for i, x := range f[0] {
		if d < 0 {
			out[i] = x
		} else {
			out[i] = -x
		}
	}
	return floatResult(args[0], args[0].Scalar(), "in mathfunction::faceforward", out...)
}

// mathReflect(e1, e2) is e1 - 2 * dot(e2, e1) * e2, evaluated as Rust does:
// a - ((2 * d) * b) per component.
func mathReflect(args []Value) (Value, error) {
	f, a, err := floats(true, args[0], args[1])
	if err != nil {
		return Value{}, err
	}
	d := a.dot(f[1], f[0])
	twoD := a.mul(2, d)
	out := make([]float64, len(f[0]))
	for i := range out {
		out[i] = a.sub(f[0][i], a.mul(twoD, f[1][i]))
	}
	return floatResult(args[0], args[0].Scalar(), "in mathfunction::reflect", out...)
}

// mathRefract(e1, e2, eta): k = 1 - eta*eta*(1 - d*d) with d = dot(e2, e1);
// the zero vector if k < 0, else eta*e1 - (eta*d + sqrt(k))*e2.
func mathRefract(args []Value) (Value, error) {
	eta := args[2]
	if !eta.IsScalar() || eta.Scalar() != args[0].Scalar() {
		return Value{}, errInvalidMathArg()
	}
	f, a, err := floats(true, args[0], args[1])
	if err != nil {
		return Value{}, err
	}
	e := floatOf(eta.comps[0])
	d := a.dot(f[1], f[0])
	k := a.sub(1, a.mul(a.mul(e, e), a.sub(1, a.mul(d, d))))
	out := make([]float64, len(f[0]))
	if k >= 0 {
		scale := a.add(a.mul(e, d), a.sqrt(k))
		for i := range out {
			out[i] = a.sub(a.mul(e, f[0][i]), a.mul(scale, f[1][i]))
		}
	}
	return floatResult(args[0], args[0].Scalar(), "in mathfunction::refract", out...)
}
