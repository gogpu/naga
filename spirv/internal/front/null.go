package front

import (
	"math"

	"github.com/gogpu/naga/ir"
)

// generateDefaultBuiltIn returns the default value of an output built-in, so
// that an output never written is not left uninitialized: Rust naga's
// null::generate_default_built_in. A clip distance gets the zero value, as
// there.
func (p *parser) generateDefaultBuiltIn(builtin *ir.BuiltinValue, ty ir.TypeHandle) ir.ExpressionHandle {
	if builtin != nil {
		switch *builtin {
		case ir.BuiltinPosition:
			zero := p.appendGlobalExpression(ir.Literal{Value: ir.LiteralF32(0)})
			one := p.appendGlobalExpression(ir.Literal{Value: ir.LiteralF32(1)})
			return p.appendGlobalExpression(ir.ExprCompose{
				Type:       ty,
				Components: []ir.ExpressionHandle{zero, zero, zero, one},
			})
		case ir.BuiltinPointSize:
			return p.appendGlobalExpression(ir.Literal{Value: ir.LiteralF32(1)})
		case ir.BuiltinFragDepth:
			return p.appendGlobalExpression(ir.Literal{Value: ir.LiteralF32(0)})
		case ir.BuiltinSampleMask:
			return p.appendGlobalExpression(ir.Literal{Value: ir.LiteralU32(math.MaxUint32)})
		}
	}
	return p.appendGlobalExpression(ir.ExprZeroValue{Type: ty})
}
