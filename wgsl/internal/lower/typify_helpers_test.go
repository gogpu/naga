package lower

import (
	"testing"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// typesOf types fn, a function of m, as every backend types it.
func typesOf(t *testing.T, m *ir.Module, fn *ir.Function) *typifier.Typifier {
	t.Helper()
	types, err := typifier.Function(m, fn)
	if err != nil {
		t.Fatalf("typify %q: %v", fn.Name, err)
	}
	return types
}

// typeOfExpr returns the type of expression h of fn, a function of m, as the
// typifier gives it to every backend.
func typeOfExpr(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) (ir.TypeResolution, error) {
	types, err := typifier.Function(m, fn)
	if err != nil {
		return ir.TypeResolution{}, err
	}
	return types.Get(h), nil
}
