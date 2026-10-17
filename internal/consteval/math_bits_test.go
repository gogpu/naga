package consteval

import (
	"errors"
	"math"
	"testing"

	"github.com/gogpu/naga/ir"
)

func u32s(x uint32) Value { return Scalar(ir.LiteralU32(x)) }

// Expected values are Rust naga v30 results.
func TestMathBits(t *testing.T) {
	runMath(t, []mathCase{
		{"ctz 0", ir.MathCountTrailingZeros, []Value{u32s(0)}, u32s(32)},
		{"ctz 8", ir.MathCountTrailingZeros, []Value{u32s(8)}, u32s(3)},
		{"clz -1", ir.MathCountLeadingZeros, []Value{i32s(-1)}, i32s(0)},
		{"clz 1", ir.MathCountLeadingZeros, []Value{u32s(1)}, u32s(31)},
		{"popcount 255", ir.MathCountOneBits, []Value{u32s(255)}, u32s(8)},
		{"popcount -1", ir.MathCountOneBits, []Value{i32s(-1)}, i32s(32)},
		{"reverse u32", ir.MathReverseBits, []Value{u32s(1)}, u32s(1 << 31)},
		{"reverse i32", ir.MathReverseBits, []Value{i32s(1)}, i32s(math.MinInt32)},
		{"ftb 0u", ir.MathFirstTrailingBit, []Value{u32s(0)}, u32s(math.MaxUint32)},
		{"ftb 12", ir.MathFirstTrailingBit, []Value{u32s(12)}, u32s(2)},
		{"ftb 0i", ir.MathFirstTrailingBit, []Value{i32s(0)}, i32s(-1)},
		{"flb 0u", ir.MathFirstLeadingBit, []Value{u32s(0)}, u32s(math.MaxUint32)},
		{"flb max", ir.MathFirstLeadingBit, []Value{u32s(math.MaxUint32)}, u32s(31)},
		{"flb -1", ir.MathFirstLeadingBit, []Value{i32s(-1)}, i32s(-1)},
		{"flb MIN", ir.MathFirstLeadingBit, []Value{i32s(math.MinInt32)}, i32s(30)},
		{"flb 5", ir.MathFirstLeadingBit, []Value{i32s(5)}, i32s(2)},
		{"extract u32", ir.MathExtractBits, []Value{u32s(0xF0), u32s(4), u32s(4)}, u32s(15)},
		{"extract i32 sign-extends", ir.MathExtractBits, []Value{i32s(-16), u32s(4), u32s(4)}, i32s(-1)},
		{"extract i32 positive", ir.MathExtractBits, []Value{i32s(0x70), u32s(4), u32s(4)}, i32s(7)},
		{"extract count 0", ir.MathExtractBits, []Value{u32s(0xF0), u32s(4), u32s(0)}, u32s(0)},
		{"extract full width", ir.MathExtractBits, []Value{i32s(-5), u32s(0), u32s(32)}, i32s(-5)},
		{"extract vector", ir.MathExtractBits, []Value{Vector(ir.LiteralU32(0xF0), ir.LiteralU32(0x0F)), u32s(0), u32s(4)},
			Vector(ir.LiteralU32(0), ir.LiteralU32(15))},
		{"insert u32", ir.MathInsertBits, []Value{u32s(0), u32s(0xF), u32s(4), u32s(4)}, u32s(240)},
		{"insert i32", ir.MathInsertBits, []Value{i32s(-1), i32s(0), u32s(0), u32s(8)}, i32s(-256)},
		{"insert count 0", ir.MathInsertBits, []Value{u32s(5), u32s(7), u32s(3), u32s(0)}, u32s(5)},
		{"insert full width", ir.MathInsertBits, []Value{u32s(5), u32s(9), u32s(0), u32s(32)}, u32s(9)},
		{"dot4I8Packed", ir.MathDot4I8Packed, []Value{u32s(0x01FF0203), u32s(0x0102FF04)}, i32s(9)},
		{"dot4U8Packed", ir.MathDot4U8Packed, []Value{u32s(0x01FF0203), u32s(0x0102FF04)}, u32s(1033)},
		{"vector popcount", ir.MathCountOneBits, []Value{Vector(ir.LiteralU32(3), ir.LiteralU32(7))},
			Vector(ir.LiteralU32(2), ir.LiteralU32(3))},
	})
}

func TestMathBitsErrors(t *testing.T) {
	tests := []struct {
		name  string
		fun   ir.MathFunction
		args  []Value
		class error
		msg   string
	}{
		{"extract range", ir.MathExtractBits, []Value{u32s(0xF0), u32s(30), u32s(4)}, ErrBitRange,
			"`offset` + `count` must be at most 32, the bit width of the operand of the extractBits built-in function, but 30 + 4 is greater"},
		{"insert range", ir.MathInsertBits, []Value{u32s(0), u32s(1), u32s(32), u32s(1)}, ErrBitRange,
			"`offset` + `count` must be at most 32, the bit width of the operand of the insertBits built-in function, but 32 + 1 is greater"},
		{"extract range with count 0", ir.MathExtractBits, []Value{u32s(0), u32s(33), u32s(0)}, ErrBitRange, ""},
		{"popcount of float", ir.MathCountOneBits, []Value{f32s(1)}, ErrInvalidOperand, ""},
		{"extract offset not u32", ir.MathExtractBits, []Value{u32s(0), i32s(1), u32s(1)}, ErrInvalidOperand, ""},
		{"insert mixed types", ir.MathInsertBits, []Value{u32s(0), i32s(1), u32s(0), u32s(1)}, ErrInvalidOperand, ""},
		{"dot4 of i32", ir.MathDot4U8Packed, []Value{i32s(1), u32s(1)}, ErrInvalidOperand, ""},
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
