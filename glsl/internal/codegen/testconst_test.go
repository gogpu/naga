package codegen

import "github.com/gogpu/naga/ir"

// testConst describes a constant of a test module by its value: a literal,
// the zero value of its type, or a composite of earlier constants' values.
type testConst struct {
	name  string
	ty    ir.TypeHandle
	lit   ir.LiteralValue     // a literal init
	comps []ir.ConstantHandle // a Compose of these constants' inits
	zero  bool                // a ZeroValue init
}

// constantsOf builds Module.Constants and the Module.GlobalExpressions their
// Init handles index, as Rust naga's Constant { name, ty, init }: a constant
// has no value but its init expression.
func constantsOf(cs ...testConst) ([]ir.Constant, []ir.Expression) {
	consts := make([]ir.Constant, 0, len(cs))
	exprs := make([]ir.Expression, 0, len(cs))
	for _, c := range cs {
		var kind ir.ExpressionKind
		switch {
		case c.zero:
			kind = ir.ExprZeroValue{Type: c.ty}
		case c.comps != nil:
			comps := make([]ir.ExpressionHandle, len(c.comps))
			for i, h := range c.comps {
				comps[i] = consts[h].Init
			}
			kind = ir.ExprCompose{Type: c.ty, Components: comps}
		default:
			kind = ir.Literal{Value: c.lit}
		}
		consts = append(consts, ir.Constant{Name: c.name, Type: c.ty, Init: ir.ExpressionHandle(len(exprs))})
		exprs = append(exprs, ir.Expression{Kind: kind})
	}
	return consts, exprs
}
