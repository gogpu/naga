package irutil

import (
	"math"

	"github.com/gogpu/naga/ir"
)

// ArenaIndex returns n, the length of an IR arena, as the 32-bit index the
// next element appended to it gets. IR handles are 32-bit and an arena never
// holds 2^32 entries; one that would is a bug, and ArenaIndex panics on it
// instead of wrapping the handle around.
func ArenaIndex(n int) uint32 {
	if n < 0 || uint64(n) > math.MaxUint32 {
		panic("irutil: arena index out of the 32-bit handle range")
	}
	return uint32(n)
}

// appendExpression appends an expression of kind to fn and returns its
// handle.
func appendExpression(fn *ir.Function, kind ir.ExpressionKind) ir.ExpressionHandle {
	h := ir.ExpressionHandle(ArenaIndex(len(fn.Expressions)))
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
	return h
}

// appendLocal appends lv to fn's local variables and returns its handle.
func appendLocal(fn *ir.Function, lv ir.LocalVariable) ir.LocalVariableHandle {
	h := ir.LocalVariableHandle(ArenaIndex(len(fn.LocalVars)))
	fn.LocalVars = append(fn.LocalVars, lv)
	return h
}
