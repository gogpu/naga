package overrides

import (
	"errors"
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
)

var (
	tBool = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	tI32  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	tU32  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	tF32  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	tF64  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
)

// TestLiteralOf is Rust naga's test_map_value_to_literal
// (back/pipeline_constants.rs), plus truncation toward zero.
func TestLiteralOf(t *testing.T) {
	ok := []struct {
		value float64
		s     ir.ScalarType
		want  ir.LiteralValue
	}{
		{0, tBool, ir.LiteralBool(false)},
		{math.Copysign(0, -1), tBool, ir.LiteralBool(false)},
		{math.NaN(), tBool, ir.LiteralBool(false)},
		{1, tBool, ir.LiteralBool(true)},
		{math.Inf(1), tBool, ir.LiteralBool(true)},
		{math.Inf(-1), tBool, ir.LiteralBool(true)},
		{math.MinInt32, tI32, ir.LiteralI32(math.MinInt32)},
		{math.MaxInt32, tI32, ir.LiteralI32(math.MaxInt32)},
		{-2.9, tI32, ir.LiteralI32(-2)},
		{0, tU32, ir.LiteralU32(0)},
		{math.MaxUint32, tU32, ir.LiteralU32(math.MaxUint32)},
		{-0.5, tU32, ir.LiteralU32(0)}, // truncates to 0, in range
		{-math.MaxFloat32, tF32, ir.LiteralF32(-math.MaxFloat32)},
		{math.MaxFloat32, tF32, ir.LiteralF32(math.MaxFloat32)},
		{-math.Float64frombits(0x47efffffefffffff), tF32, ir.LiteralF32(-math.MaxFloat32)},
		{math.Float64frombits(0x47efffffefffffff), tF32, ir.LiteralF32(math.MaxFloat32)},
		{-math.MaxFloat64, tF64, ir.LiteralF64(-math.MaxFloat64)},
		{math.MaxFloat64, tF64, ir.LiteralF64(math.MaxFloat64)},
	}
	for _, tc := range ok {
		got, err := literalOf(tc.value, tc.s)
		if err != nil || got != tc.want {
			t.Errorf("literalOf(%v, %v) = %v, %v; want %v", tc.value, tc.s, got, err, tc.want)
		}
	}
	bad := []struct {
		value float64
		s     ir.ScalarType
		want  error
	}{
		{math.MinInt32 - 1, tI32, ErrValueOutOfRange},
		{math.MaxInt32 + 1, tI32, ErrValueOutOfRange},
		{-1, tU32, ErrValueOutOfRange},
		{math.MaxUint32 + 1, tU32, ErrValueOutOfRange},
		{-math.Float64frombits(0x47effffff0000000), tF32, ErrValueOutOfRange},
		{math.Float64frombits(0x47effffff0000000), tF32, ErrValueOutOfRange},
	}
	for _, s := range []ir.ScalarType{tI32, tU32, tF32, tF64} {
		for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			bad = append(bad, struct {
				value float64
				s     ir.ScalarType
				want  error
			}{v, s, ErrValueNotFinite})
		}
	}
	for _, tc := range bad {
		if _, err := literalOf(tc.value, tc.s); !errors.Is(err, tc.want) {
			t.Errorf("literalOf(%v, %v): err = %v, want %v", tc.value, tc.s, err, tc.want)
		}
	}
}
