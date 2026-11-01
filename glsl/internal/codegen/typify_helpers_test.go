package codegen

import (
	"testing"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// setTestFunction makes fn the function w writes, typed as Compile types it.
func setTestFunction(t *testing.T, w *Writer, fn *ir.Function) {
	t.Helper()
	types, err := typifier.Function(w.module, fn)
	if err != nil {
		t.Fatalf("typify %q: %v", fn.Name, err)
	}
	w.currentFunction, w.types = fn, types
}

// zeroValues returns one ZeroValue expression of each type: expressions the
// typifier types as those type handles.
func zeroValues(types ...ir.TypeHandle) []ir.Expression {
	exprs := make([]ir.Expression, len(types))
	for i, ty := range types {
		exprs[i] = ir.Expression{Kind: ir.ExprZeroValue{Type: ty}}
	}
	return exprs
}
