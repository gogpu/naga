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

// NeedsPreEmit reports whether an expression of this kind is evaluated before
// the function runs and must not be covered by an Emit statement (Rust naga's
// Expression::needs_pre_emit).
func NeedsPreEmit(kind ExpressionKind) bool {
	return isPreEmitExpression(kind)
}
