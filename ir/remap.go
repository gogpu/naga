package ir

// RemapExpressionHandles returns kind with every operand handle h replaced by
// remap[h]; a handle outside remap is kept. Handles that name other arenas
// (constants, overrides, globals, locals, types) are not changed.
func RemapExpressionHandles(kind ExpressionKind, remap []ExpressionHandle) ExpressionKind {
	return remapExprHandles(kind, remap)
}

// RemapStatementExpressionHandles replaces, in place, every expression
// handle h in block and its nested blocks by remap[h]; a handle outside remap
// is kept. An Emit range is adjusted by its first and last expression, as
// Rust naga's adjust_stmt does.
func RemapStatementExpressionHandles(block []Statement, remap []ExpressionHandle) {
	remapStmtExprHandles(block, remap)
}

// NeedsPreEmit reports whether e is evaluated before the function runs and
// must not be covered by an Emit statement (Rust naga's
// Expression::needs_pre_emit): a literal, a constant, an override, a zero
// value, a global or local variable or a function argument.
func (e Expression) NeedsPreEmit() bool {
	switch e.Kind.(type) {
	case Literal, ExprConstant, ExprOverride, ExprZeroValue,
		ExprGlobalVariable, ExprFunctionArgument, ExprLocalVariable:
		return true
	default:
		return false
	}
}
