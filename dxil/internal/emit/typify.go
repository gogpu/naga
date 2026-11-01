package emit

import (
	"fmt"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// typifierOf returns the typifier of fn, grown over every expression fn has
// now: the emitter appends expressions while it emits (appendZeroValue).
// A failure to type is recorded in typeErr and leaves the typifier as far
// as it got.
func (e *Emitter) typifierOf(fn *ir.Function) *typifier.Typifier {
	if e.types == nil {
		e.types = make(map[*ir.Function]*typifier.Typifier)
	}
	t := e.types[fn]
	if t == nil {
		if t = e.opts.Types.Function(fn); t == nil {
			t = &typifier.Typifier{}
		}
		e.types[fn] = t
	}
	if t.Len() < len(fn.Expressions) {
		if err := t.Grow(e.ir, fn, ir.ExpressionHandle(len(fn.Expressions))); err != nil && e.typeErr == nil { //nolint:gosec // expression count fits a 32-bit handle by IR design
			e.typeErr = fmt.Errorf("function %q: %w", fn.Name, err)
		}
	}
	return t
}

// exprType returns the type of expression h of fn, the zero TypeResolution
// when fn could not be typed (typeErr).
func (e *Emitter) exprType(fn *ir.Function, h ir.ExpressionHandle) ir.TypeResolution {
	t := e.typifierOf(fn)
	if int(h) >= t.Len() {
		return ir.TypeResolution{}
	}
	return t.Get(h)
}

// exprInner returns the type of expression h of fn as a TypeInner, nil
// when it has none.
func (e *Emitter) exprInner(fn *ir.Function, h ir.ExpressionHandle) ir.TypeInner {
	res := e.exprType(fn, h)
	if res.Handle != nil {
		if int(*res.Handle) < len(e.ir.Types) {
			return e.ir.Types[*res.Handle].Inner
		}
		return nil
	}
	return res.Value
}
