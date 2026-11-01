package codegen

import (
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// hasExprType reports whether expression h of the function being written
// has a type: whether h is one of its expressions.
func (w *Writer) hasExprType(h ir.ExpressionHandle) bool {
	return w.types != nil && int(h) < w.types.Len()
}

// exprType returns the type of expression h of the function being written,
// the zero TypeResolution outside one.
func (w *Writer) exprType(h ir.ExpressionHandle) ir.TypeResolution {
	return typeIn(w.types, h)
}

// fnExprType returns the type of expression h of fn, a function or entry
// point function of the module.
func (w *Writer) fnExprType(fn *ir.Function, h ir.ExpressionHandle) ir.TypeResolution {
	return typeIn(w.typifiers.Function(fn), h)
}

func typeIn(t *typifier.Typifier, h ir.ExpressionHandle) ir.TypeResolution {
	if t == nil || int(h) >= t.Len() {
		return ir.TypeResolution{}
	}
	return t.Get(h)
}
