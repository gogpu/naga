package codegen

import (
	"fmt"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
)

// typify types the expressions of every function and entry point of the
// module, once per Compile. Rust's writer reads them from the validator's
// ModuleInfo; Compile takes no such info, so it types the module it writes.
func (b *Backend) typify() error {
	types, err := typifier.ForModule(b.module)
	if err != nil {
		return fmt.Errorf("spirv: %w", err)
	}
	b.typifiers = types
	b.globalTypes = nil
	return nil
}

// exprType returns the type of expression h of fn, a function of the module
// or the global expression arena (globalExpressions).
func (b *Backend) exprType(fn *ir.Function, h ir.ExpressionHandle) (ir.TypeResolution, error) {
	t := b.typifiers.Function(fn)
	if fn == b.globalExprFn && b.globalExprFn != nil {
		if b.globalTypes == nil {
			gt, err := typifier.Function(b.module, fn)
			if err != nil {
				return ir.TypeResolution{}, fmt.Errorf("global expressions: %w", err)
			}
			b.globalTypes = gt
		}
		t = b.globalTypes
	}
	if t == nil {
		return ir.TypeResolution{}, fmt.Errorf("function %q is not typed", fn.Name)
	}
	if int(h) >= t.Len() {
		return ir.TypeResolution{}, fmt.Errorf("expression %d out of range (%d expressions)", h, t.Len())
	}
	return t.Get(h), nil
}

// globalExpressions returns the module's global expression arena as a
// function, for the typifier.
func (b *Backend) globalExpressions() *ir.Function {
	if b.globalExprFn == nil {
		b.globalExprFn = &ir.Function{Expressions: b.module.GlobalExpressions}
	}
	return b.globalExprFn
}
