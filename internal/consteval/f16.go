package consteval

import "math"

// maxF16 is the largest finite binary16 value.
const maxF16 = 65504

// roundF16 rounds x to the nearest IEEE 754 binary16 value (ties to even),
// including subnormals, and returns it as a float32, which represents every
// binary16 value exactly. ok is false if x is NaN or infinite or rounds
// beyond the finite range: a const-expression must not produce such a value.
//
// x is rounded once, from float64; rounding through float32 first would
// round twice and can differ in the last place.
func roundF16(x float64) (float32, bool) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, false
	}
	a := math.Abs(x)
	if a == 0 {
		return float32(x), true // keeps the sign of zero
	}
	// The spacing of binary16 values around a: 2^(e-10) for a normal value
	// with exponent e, and the subnormal spacing 2^-24 below 2^-14.
	_, exp := math.Frexp(a) // a = frac·2^exp, frac in [0.5, 1)
	e := exp - 1
	if e < -14 {
		e = -14
	}
	quantum := math.Ldexp(1, e-10)
	r := math.RoundToEven(a/quantum) * quantum // a/quantum is exact
	if r > maxF16 {
		return 0, false
	}
	return float32(math.Copysign(r, x)), true
}
