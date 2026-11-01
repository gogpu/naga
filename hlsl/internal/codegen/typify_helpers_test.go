package codegen

import "github.com/gogpu/naga/ir"

// zeroValues returns one ZeroValue expression of each type: expressions the
// typifier types as those type handles.
func zeroValues(types ...ir.TypeHandle) []ir.Expression {
	exprs := make([]ir.Expression, len(types))
	for i, ty := range types {
		exprs[i] = ir.Expression{Kind: ir.ExprZeroValue{Type: ty}}
	}
	return exprs
}
