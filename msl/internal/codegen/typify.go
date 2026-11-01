package codegen

import (
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

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

// resolutionInner returns the type res stands for, nil for none.
func (w *Writer) resolutionInner(res ir.TypeResolution) ir.TypeInner {
	if res.Handle != nil {
		if int(*res.Handle) < len(w.module.Types) {
			return w.module.Types[*res.Handle].Inner
		}
		return nil
	}
	return res.Value
}

func typeIn(t *typifier.Typifier, h ir.ExpressionHandle) ir.TypeResolution {
	if t == nil || int(h) >= t.Len() {
		return ir.TypeResolution{}
	}
	return t.Get(h)
}
