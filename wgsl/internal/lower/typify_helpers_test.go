package lower

import (
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// typeOfExpr returns the type of expression h of fn, a function of m, as the
// typifier gives it to every backend.
func typeOfExpr(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) (ir.TypeResolution, error) {
	types, err := typifier.Function(m, fn)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	return types.Get(h), nil
}
