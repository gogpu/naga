package front

import (
	"math"

	"github.com/gogpu/naga/ir"
)

// lookupConstant is one entry of the SPIR-V constant id table: the IR constant
// or override and the constant's SPIR-V type id (Rust naga's LookupConstant).
type lookupConstant struct {
	kind   constantKind
	typeID uint32
}

// constantKind is Rust naga's Constant enum: an IR constant or an override.
type constantKind struct {
	constant *ir.ConstantHandle
	override *ir.OverrideHandle
}

// expressionKind returns the expression that refers to the constant.
func (c constantKind) expressionKind() ir.ExpressionKind {
	if c.override != nil {
		return ir.ExprOverride{Override: *c.override}
	}
	return ir.ExprConstant{Constant: *c.constant}
}

// appendGlobalExpression appends an expression to the module's global
// expressions and returns its handle.
func (p *parser) appendGlobalExpression(kind ir.ExpressionKind) ir.ExpressionHandle {
	p.module.GlobalExpressions = append(p.module.GlobalExpressions, ir.Expression{Kind: kind})
	return ir.ExpressionHandle(len(p.module.GlobalExpressions) - 1)
}

// parseConstant parses OpConstant and OpSpecConstant: a literal of a scalar
// type, whose SpecId decoration decides whether it is an override.
func (p *parser) parseConstant(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	lookup, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	ty := lookup.handle

	scalar, ok := p.module.Types[ty].Inner.(ir.ScalarType)
	if !ok {
		return errUnsupportedType(ty)
	}
	low, err := p.next()
	if err != nil {
		return err
	}
	var literal ir.LiteralValue
	switch scalar.Kind {
	case ir.ScalarUint:
		switch scalar.Width {
		case 4:
			literal = ir.LiteralU32(low)
		case 8:
			if err := inst.expect(5); err != nil {
				return err
			}
			high, err := p.next()
			if err != nil {
				return err
			}
			literal = ir.LiteralU64(uint64(high)<<32 | uint64(low))
		default:
			return errInvalidTypeWidth(uint32(scalar.Width))
		}
	case ir.ScalarSint:
		switch scalar.Width {
		case 4:
			literal = ir.LiteralI32(int32(low))
		case 8:
			if err := inst.expect(5); err != nil {
				return err
			}
			high, err := p.next()
			if err != nil {
				return err
			}
			literal = ir.LiteralI64(int64(uint64(high)<<32 | uint64(low)))
		default:
			return errInvalidTypeWidth(uint32(scalar.Width))
		}
	case ir.ScalarFloat:
		switch scalar.Width {
		case 2:
			// A float narrower than 32 bits sits in the low-order bits.
			literal = ir.LiteralF16(float16FromBits(uint16(low)))
		case 4:
			literal = ir.LiteralF32(math.Float32frombits(low))
		case 8:
			if err := inst.expect(5); err != nil {
				return err
			}
			high, err := p.next()
			if err != nil {
				return err
			}
			literal = ir.LiteralF64(math.Float64frombits(uint64(high)<<32 | uint64(low)))
		default:
			return errInvalidTypeWidth(uint32(scalar.Width))
		}
	default:
		return errUnsupportedType(ty)
	}

	init := p.appendGlobalExpression(ir.Literal{Value: literal})
	return p.insertParsedConstant(id, typeID, ty, init)
}

// parseCompositeConstant parses OpConstantComposite and
// OpSpecConstantComposite into a Compose expression.
func (p *parser) parseCompositeConstant(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	lookup, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	ty := lookup.handle

	components := make([]ir.ExpressionHandle, 0, inst.wc-3)
	for range inst.wc - 3 {
		componentID, err := p.next()
		if err != nil {
			return err
		}
		component, err := p.constantLookup(componentID)
		if err != nil {
			return err
		}
		components = append(components, p.appendGlobalExpression(component.kind.expressionKind()))
	}
	init := p.appendGlobalExpression(ir.ExprCompose{Type: ty, Components: components})
	return p.insertParsedConstant(id, typeID, ty, init)
}

// parseNullConstant parses OpConstantNull and OpUndef into a zero value.
func (p *parser) parseNullConstant(inst instruction) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(3); err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	lookup, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	ty := lookup.handle
	init := p.appendGlobalExpression(ir.ExprZeroValue{Type: ty})
	return p.insertParsedConstant(id, typeID, ty, init)
}

// parseBoolConstant parses the true and false constant instructions.
func (p *parser) parseBoolConstant(inst instruction, value bool) error {
	if err := p.switchTo(stateType, inst.op); err != nil {
		return err
	}
	if err := inst.expect(3); err != nil {
		return err
	}
	typeID, err := p.next()
	if err != nil {
		return err
	}
	id, err := p.next()
	if err != nil {
		return err
	}
	lookup, err := p.typeLookup(typeID)
	if err != nil {
		return err
	}
	ty := lookup.handle
	init := p.appendGlobalExpression(ir.Literal{Value: ir.LiteralBool(value)})
	return p.insertParsedConstant(id, typeID, ty, init)
}

// insertParsedConstant records a parsed constant: an override when its SpecId
// decoration names one, a constant otherwise.
func (p *parser) insertParsedConstant(id, typeID uint32, ty ir.TypeHandle, init ir.ExpressionHandle) error {
	dec := p.takeDecoration(id)
	kind := constantKind{}
	if dec.specID != nil {
		if *dec.specID > math.MaxUint16 {
			return errSpecIDTooHigh(*dec.specID)
		}
		specID := uint16(*dec.specID)
		initHandle := init
		handle := ir.OverrideHandle(len(p.module.Overrides))
		p.module.Overrides = append(p.module.Overrides, ir.Override{
			Name: dec.name,
			ID:   &specID,
			Ty:   ty,
			Init: &initHandle,
		})
		kind.override = &handle
	} else {
		handle := ir.ConstantHandle(len(p.module.Constants))
		p.module.Constants = append(p.module.Constants, ir.Constant{
			Name: dec.name,
			Type: ty,
			Init: init,
		})
		kind.constant = &handle
	}
	p.lookupConstant[id] = lookupConstant{kind: kind, typeID: typeID}
	return nil
}

// constantLookup returns the table entry of a constant id.
func (p *parser) constantLookup(id uint32) (lookupConstant, error) {
	constant, ok := p.lookupConstant[id]
	if !ok {
		return lookupConstant{}, errInvalidID(id)
	}
	return constant, nil
}

// constantArraySize resolves the element count of an array from its length
// constant: a plain u32 or i32 literal, nonzero and within maxArrayElements.
func (p *parser) constantArraySize(lengthID uint32) (uint32, error) {
	length, err := p.constantLookup(lengthID)
	if err != nil {
		return 0, err
	}
	if length.kind.constant == nil {
		return 0, errInvalidArraySize(lengthID)
	}
	init := p.module.Constants[*length.kind.constant].Init
	var count uint32
	switch literal := p.module.GlobalExpressions[init].Kind.(type) {
	case ir.Literal:
		switch value := literal.Value.(type) {
		case ir.LiteralU32:
			count = uint32(value)
		case ir.LiteralI32:
			count = uint32(value)
		default:
			return 0, errInvalidArraySize(lengthID)
		}
	default:
		return 0, errInvalidArraySize(lengthID)
	}
	if count == 0 {
		return 0, errInvalidArraySize(lengthID)
	}
	if count > maxArrayElements {
		return 0, errArrayTooLarge(count)
	}
	return count, nil
}

// float16FromBits widens an IEEE 754 half-precision value to float32,
// preserving every value a half can hold, including subnormals.
func float16FromBits(bits uint16) float32 {
	sign := uint32(bits>>15) << 31
	exponent := uint32(bits>>10) & 0x1f
	mantissa := uint32(bits) & 0x3ff
	var out uint32
	switch {
	case exponent == 0:
		if mantissa == 0 {
			out = sign
			break
		}
		// A subnormal half has no implicit bit: normalize the mantissa, one
		// exponent step per shift, from the smallest normal half's exponent.
		exp := uint32(127 - 14)
		for mantissa&0x400 == 0 {
			mantissa <<= 1
			exp--
		}
		mantissa &= 0x3ff
		out = sign | exp<<23 | mantissa<<13
	case exponent == 0x1f:
		out = sign | 0xff<<23 | mantissa<<13
	default:
		out = sign | (exponent+127-15)<<23 | mantissa<<13
	}
	return math.Float32frombits(out)
}
