package consteval

import (
	"math/bits"
	"strconv"

	"github.com/gogpu/naga/ir"
)

// bitFuncs are the integer bit built-ins.
var bitFuncs = map[ir.MathFunction]func([]Value) (Value, error){
	ir.MathCountTrailingZeros: func(a []Value) (Value, error) {
		return bitsLane(a, func(u uint32) uint32 { return bitCount(bits.TrailingZeros32(u)) })
	},
	ir.MathCountLeadingZeros: func(a []Value) (Value, error) {
		return bitsLane(a, func(u uint32) uint32 { return bitCount(bits.LeadingZeros32(u)) })
	},
	ir.MathCountOneBits: func(a []Value) (Value, error) {
		return bitsLane(a, func(u uint32) uint32 { return bitCount(bits.OnesCount32(u)) })
	},
	ir.MathReverseBits:      func(a []Value) (Value, error) { return bitsLane(a, bits.Reverse32) },
	ir.MathFirstTrailingBit: func(a []Value) (Value, error) { return bitsLane(a, firstTrailingBit) },
	ir.MathFirstLeadingBit:  func(a []Value) (Value, error) { return componentWise(a, firstLeadingBitLane) },
	ir.MathExtractBits:      extractBits,
	ir.MathInsertBits:       insertBits,
	ir.MathDot4I8Packed:     func(a []Value) (Value, error) { return dot4Packed(true, a) },
	ir.MathDot4U8Packed:     func(a []Value) (Value, error) { return dot4Packed(false, a) },
}

// bitCount converts a bit count or index from math/bits (0 to 64).
func bitCount(n int) uint32 { return uint32(n) } //nolint:gosec // 0 to 64

// is32BitInt reports i32 or u32, the types of the WGSL bit built-ins.
func is32BitInt(s ir.ScalarType) bool {
	return (s.Kind == ir.ScalarSint || s.Kind == ir.ScalarUint) && s.Width == 4
}

// bitsLane applies f to the 32-bit pattern of each i32 or u32 component.
func bitsLane(args []Value, f func(uint32) uint32) (Value, error) {
	return componentWise(args, func(xs []ir.LiteralValue) (ir.LiteralValue, error) {
		s := ScalarOf(xs[0])
		if !is32BitInt(s) {
			return nil, errInvalidMathArg()
		}
		return makeInt(s, uint64(f(u32Low(intBits(xs[0]))))), nil
	})
}

// firstTrailingBit is the index of the lowest set bit, or all ones (-1 for
// i32) for zero.
func firstTrailingBit(u uint32) uint32 {
	if u == 0 {
		return ^uint32(0)
	}
	return bitCount(bits.TrailingZeros32(u))
}

// firstLeadingBitLane: for u32 and non-negative i32 the index of the highest
// set bit; for negative i32 the index of the highest clear bit; -1 (all ones)
// when there is none (0, or -1 for i32).
func firstLeadingBitLane(xs []ir.LiteralValue) (ir.LiteralValue, error) {
	s := ScalarOf(xs[0])
	if !is32BitInt(s) {
		return nil, errInvalidMathArg()
	}
	u := u32Low(intBits(xs[0]))
	if s.Kind == ir.ScalarSint && int32(u) < 0 { //nolint:gosec // the i32 bit pattern
		u = ^u
	}
	r := ^uint32(0)
	if u != 0 {
		r = bitCount(31 - bits.LeadingZeros32(u))
	}
	return makeInt(s, uint64(r)), nil
}

// bitfieldRange reads the scalar u32 offset and count of extractBits or
// insertBits and checks offset + count <= width, even when count is 0.
func bitfieldRange(builtin string, width uint64, offset, count Value) (uint32, uint32, error) {
	o, ok1 := u32Scalar(offset)
	c, ok2 := u32Scalar(count)
	if !ok1 || !ok2 {
		return 0, 0, errInvalidMathArg()
	}
	if uint64(o)+uint64(c) > width {
		return 0, 0, newError(ErrBitRange, "`offset` + `count` must be at most "+strconv.FormatUint(width, 10)+
			", the bit width of the operand of the "+builtin+" built-in function, but "+
			strconv.FormatUint(uint64(o), 10)+" + "+strconv.FormatUint(uint64(c), 10)+" is greater")
	}
	return o, c, nil
}

func u32Scalar(v Value) (uint32, bool) {
	if !v.IsScalar() {
		return 0, false
	}
	u, ok := v.comps[0].(ir.LiteralU32)
	return uint32(u), ok
}

func intWidth(s ir.ScalarType) uint64 { return uint64(s.Width) * 8 }

// extractBits(e, offset, count): count bits of e from offset; signed types
// sign-extend from the top bit of the field.
func extractBits(args []Value) (Value, error) {
	e := args[0]
	s := e.Scalar()
	if s.Kind != ir.ScalarSint && s.Kind != ir.ScalarUint {
		return Value{}, errInvalidMathArg()
	}
	w := intWidth(s)
	o, c, err := bitfieldRange("extractBits", w, args[1], args[2])
	if err != nil {
		return Value{}, err
	}
	out := make([]ir.LiteralValue, e.Len())
	for i, lit := range e.comps {
		var r uint64
		if c > 0 {
			shifted := intBits(lit) << (64 - uint(o) - uint(c)) // field at the top of 64 bits
			if s.Kind == ir.ScalarSint {
				r = u64FromI64(i64FromU64(shifted) >> (64 - uint(c)))
			} else {
				r = shifted >> (64 - uint(c))
			}
		}
		out[i] = makeInt(s, r)
	}
	return e.withComponents(out), nil
}

// insertBits(e, newbits, offset, count): e with count bits from offset
// replaced by the low bits of newbits.
func insertBits(args []Value) (Value, error) {
	e, nb := args[0], args[1]
	s := e.Scalar()
	if (s.Kind != ir.ScalarSint && s.Kind != ir.ScalarUint) || nb.Scalar() != s || nb.Len() != e.Len() {
		return Value{}, errInvalidMathArg()
	}
	o, c, err := bitfieldRange("insertBits", intWidth(s), args[2], args[3])
	if err != nil {
		return Value{}, err
	}
	out := make([]ir.LiteralValue, e.Len())
	for i := range e.comps {
		r := intBits(e.comps[i])
		if c > 0 {
			mask := (^uint64(0) >> (64 - uint(c))) << o
			r = (r &^ mask) | ((intBits(nb.comps[i]) << o) & mask)
		}
		out[i] = makeInt(s, r)
	}
	return e.withComponents(out), nil
}

// dot4Packed is the dot product of two vectors of four 8-bit integers packed
// into u32 values, the low byte first; signed bytes give an i32 result.
func dot4Packed(signed bool, args []Value) (Value, error) {
	a, ok1 := u32Scalar(args[0])
	b, ok2 := u32Scalar(args[1])
	if !ok1 || !ok2 {
		return Value{}, errInvalidMathArg()
	}
	var sum uint32 // wraps, like the u32/i32 result
	for k := 0; k < 32; k += 8 {
		x, y := (a>>k)&0xFF, (b>>k)&0xFF
		if signed {
			sum += uint32(int32(int8(x)) * int32(int8(y))) //nolint:gosec // byte reinterpretation and wrapping sum are the definition
		} else {
			sum += x * y
		}
	}
	if signed {
		return Scalar(ir.LiteralI32(i32Low(uint64(sum)))), nil
	}
	return Scalar(ir.LiteralU32(sum)), nil
}
