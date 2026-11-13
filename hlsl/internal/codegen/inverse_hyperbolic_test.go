package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// A constant handle outside the module is written by its fallback name, not
// dereferenced (the unnamed-constant check must not index past the arena).
func TestWriteConstantExpressionOutsideModule(t *testing.T) {
	w := &Writer{module: &ir.Module{}, names: map[nameKey]string{}}
	if err := w.writeConstantExpression(ir.ExprConstant{Constant: 0}); err != nil {
		t.Fatal(err)
	}
	if got := w.Out.String(); got != "const_0" {
		t.Errorf("wrote %q, want const_0", got)
	}
}

// An argument the writer cannot write is reported, not written as text.
func TestWriteInverseHyperbolicReportsArgumentError(t *testing.T) {
	for _, fun := range []ir.MathFunction{ir.MathAsinh, ir.MathAcosh, ir.MathAtanh} {
		w := &Writer{currentFunction: &ir.Function{}}
		if err := w.writeInverseHyperbolic(ir.ExprMath{Fun: fun, Arg: 0}); err == nil {
			t.Errorf("math function %d: no error for an argument handle outside the function", fun)
		}
	}
}
