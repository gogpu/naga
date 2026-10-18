package overrides

import (
	"math"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/ir"
)

// f32Overflow is the smallest magnitude that rounds to infinity as float32:
// halfway between the largest finite float32 and the next power of two.
var f32Overflow = math.Float64frombits(0x47effffff0000000)

// literalOf converts a pipeline-constant value to a literal of the override's
// scalar type, as WebIDL converts a double to the WGSL type (Rust naga's
// map_value_to_literal):
//   - bool: true unless the value is 0 or NaN;
//   - i32, u32: finite, truncated toward zero, in range ([EnforceRange]);
//   - f16, f32: finite, rounded to nearest, finite after rounding;
//   - f64: finite.
func literalOf(value float64, s ir.ScalarType) (ir.LiteralValue, error) {
	if s.Kind == ir.ScalarBool {
		return ir.LiteralBool(value != 0 && !math.IsNaN(value)), nil
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, ErrValueNotFinite
	}
	if s.Kind == ir.ScalarFloat {
		return floatLiteralOf(value, s.Width)
	}
	if s.Width != 4 {
		// The validator allows overrides of bool, i32, u32, f16, f32 and f64
		// only.
		return nil, ErrValueOutOfRange
	}
	v := math.Trunc(value)
	switch s.Kind {
	case ir.ScalarSint:
		if v < math.MinInt32 || v > math.MaxInt32 {
			return nil, ErrValueOutOfRange
		}
		return ir.LiteralI32(int32(v)), nil
	case ir.ScalarUint:
		if v < 0 || v > math.MaxUint32 {
			return nil, ErrValueOutOfRange
		}
		return ir.LiteralU32(uint32(v)), nil
	}
	return nil, ErrValueOutOfRange
}

// floatLiteralOf converts a finite value to a float literal of width bytes,
// rounded to nearest; a value that rounds beyond the finite range does not
// fit.
func floatLiteralOf(value float64, width uint8) (ir.LiteralValue, error) {
	switch width {
	case 2:
		v, ok := consteval.RoundF16(value)
		if !ok {
			return nil, ErrValueOutOfRange
		}
		return ir.LiteralF16(v), nil
	case 4:
		if math.Abs(value) >= f32Overflow {
			return nil, ErrValueOutOfRange
		}
		return ir.LiteralF32(float32(value)), nil
	case 8:
		return ir.LiteralF64(value), nil
	}
	return nil, ErrValueOutOfRange
}
