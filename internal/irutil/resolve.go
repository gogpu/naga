package irutil

import "github.com/gogpu/naga/ir"

// ResolveAtomicPointerScalar returns the scalar an atomic operation on
// pointer works with: the scalar of the atomic type the pointer points to (or
// of a scalar it points to), or nil when the pointer's type cannot be
// resolved or is neither. ir's typifier gives the result type of an atomic
// statement the same way.
func ResolveAtomicPointerScalar(module *ir.Module, fn *ir.Function, pointer ir.ExpressionHandle) *ir.ScalarType {
	ptrType, err := ir.ResolveExpressionType(module, fn, pointer)
	if err != nil {
		return nil
	}
	switch t := ptrType.Inner(module.Types).(type) {
	case ir.AtomicType:
		return &t.Scalar
	case ir.ScalarType:
		return &t
	}
	return nil
}
