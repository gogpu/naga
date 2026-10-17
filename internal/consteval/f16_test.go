package consteval

import (
	"math"
	"testing"
)

// Expected values are IEEE 754 binary16 roundings (round to nearest, ties to
// even), computed by hand from the format: 1 sign, 5 exponent (bias 15) and
// 10 fraction bits; subnormals are k·2^-24.
func TestRoundF16(t *testing.T) {
	sub := math.Ldexp(1, -24) // smallest subnormal
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"zero", 0, 0},
		{"one", 1, 1},
		{"max finite", 65504, 65504},
		{"just below rounding to inf", 65519.99, 65504},
		{"0.1", 0.1, 0.0999755859375},
		{"tie to even down", 2049, 2048},
		{"tie to even up", 2051, 2052},
		{"above tie", 2049.01, 2050},
		{"min normal", math.Ldexp(1, -14), math.Ldexp(1, -14)},
		{"largest subnormal", math.Ldexp(1023, -24), math.Ldexp(1023, -24)},
		{"min subnormal", sub, sub},
		{"half min subnormal ties to zero", sub / 2, 0},
		{"just above half min subnormal", sub/2 + math.Ldexp(1, -60), sub},
		{"1.5 subnormal ties to even 2", 1.5 * sub, 2 * sub},
		{"2.5 subnormal ties to even 2", 2.5 * sub, 2 * sub},
		{"subnormal rounds into normal", math.Ldexp(1, -14) - sub/4, math.Ldexp(1, -14)},
		{"negative", -0.1, -0.0999755859375},
		{"mantissa carry", 1 + math.Ldexp(1, -11) + math.Ldexp(1, -20), 1 + math.Ldexp(1, -10)},
	}
	for _, tt := range tests {
		got, ok := roundF16(tt.in)
		if !ok || float64(got) != tt.want {
			t.Errorf("%s: roundF16(%g) = %g, %v; want %g", tt.name, tt.in, got, ok, tt.want)
		}
	}
}

// decodeF16 decodes binary16 bits independently of roundF16.
func decodeF16(bits uint16) float64 {
	sign := 1.0
	if bits&0x8000 != 0 {
		sign = -1
	}
	exp := int(bits>>10) & 0x1F
	frac := float64(bits & 0x3FF)
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

// Exhaustive over all positive finite binary16 values: each is a fixed point,
// a midpoint between neighbors ties to the even one, and values just off the
// midpoint go to the nearer neighbor.
func TestRoundF16Exhaustive(t *testing.T) {
	for bits := uint16(0); bits < 0x7C00; bits++ {
		v := decodeF16(bits)
		if got, ok := roundF16(v); !ok || float64(got) != v {
			t.Fatalf("roundF16(%g [%#04x]) = %g, %v; want itself", v, bits, got, ok)
		}
		if bits == 0x7BFF {
			break // no finite neighbor above the maximum
		}
		next := decodeF16(bits + 1)
		mid := (v + next) / 2
		even := v
		if bits&1 == 1 {
			even = next
		}
		if got, _ := roundF16(mid); float64(got) != even {
			t.Fatalf("roundF16(midpoint %g) = %g, want even neighbor %g", mid, got, even)
		}
		eps := (next - v) / 1024
		if got, _ := roundF16(mid - eps); float64(got) != v {
			t.Fatalf("roundF16(%g) = %g, want %g", mid-eps, got, v)
		}
		if got, _ := roundF16(mid + eps); float64(got) != next {
			t.Fatalf("roundF16(%g) = %g, want %g", mid+eps, got, next)
		}
	}
}

func TestRoundF16Overflow(t *testing.T) {
	for _, in := range []float64{65520, 1e6, -65520, math.Inf(1), math.NaN()} {
		if got, ok := roundF16(in); ok {
			t.Errorf("roundF16(%g) = %g, ok; want overflow", in, got)
		}
	}
}

func TestRoundF16NegativeZero(t *testing.T) {
	got, ok := roundF16(-math.Ldexp(1, -30))
	if !ok || got != 0 || !math.Signbit(float64(got)) {
		t.Errorf("roundF16(-tiny) = %g (signbit %v), want -0", got, math.Signbit(float64(got)))
	}
}
