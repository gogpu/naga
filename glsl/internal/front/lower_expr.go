package front

// This file is Rust naga's Context::lower_inner (context.rs:544-1394): it
// lowers one HIR expression in a given position to IR expressions, applying
// GLSL's implicit conversions and decomposing the operations the IR does not
// have (matrix division, matrix comparison, scalar-matrix arithmetic, vector
// relational results).

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// lower lowers expr in position pos. ok is false when the expression yields
// no value, as a call of a void function does, Rust naga's Context::lower
// (context.rs:420-433).
func (c *Context) lower(arena *hirArena, expr hirHandle, pos exprPos) (ir.ExpressionHandle, bool, ir.Span, *frontError) {
	return c.lowerInner(arena, expr, pos)
}

// lowerExpect is lower for an expression that must produce a value, Rust
// naga's Context::lower_expect (context.rs:440-453).
func (c *Context) lowerExpect(arena *hirArena, expr hirHandle, pos exprPos) (ir.ExpressionHandle, ir.Span, *frontError) {
	h, ok, span, err := c.lowerInner(arena, expr, pos)
	if err != nil {
		return 0, span, err
	}
	if !ok {
		return 0, span, errMessage(span, "Expression returns void")
	}
	return h, span, nil
}

// lowerInner is the one lowering function every expression goes through.
func (c *Context) lowerInner(e *hirArena, expr hirHandle, pos exprPos) (ir.ExpressionHandle, bool, ir.Span, *frontError) {
	node := e.get(expr)
	span := node.Span

	// The parser counts nesting, not chain length (it parses operator chains in
	// loops), so a very long chain arrives here as a very deep tree. Bound the
	// recursion the same way the WGSL frontend does, with the parser's own
	// limit and message: past it the walk would end in a stack overflow.
	c.exprDepth++
	defer func() { c.exprDepth-- }()
	if c.exprDepth > maxExprDepth {
		return 0, false, span, errMessage(span, errExprDepth)
	}

	switch k := node.Kind.(type) {
	case hirAccess:
		return c.lowerAccess(e, k, pos, span)
	case hirSelect:
		base, _, err := c.lowerExpect(e, k.Base, pos)
		if err != nil {
			return 0, false, span, err
		}
		h, err := c.fe.fieldSelection(c, pos, base, k.Field, span)
		return h, true, span, err
	case hirLiteral:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.addExpression(ir.Literal{Value: k.Value}, span)
		return h, true, span, err
	case hirBinary:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.lowerBinary(e, k, span)
		return h, true, span, err
	case hirUnary:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.lowerUnary(e, k, span)
		return h, true, span, err
	case hirVariable:
		h, err := c.lowerVariable(k, pos, span)
		return h, true, span, err
	case hirCall:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, ok, err := c.fe.functionOrConstructorCall(c, e, k.Callee, k.Args, span)
		return h, ok, span, err
	case hirConditional:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.lowerConditional(e, k, span)
		return h, true, span, err
	case hirAssign:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.lowerAssign(e, k, span)
		return h, true, span, err
	case hirPrePostfix:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.lowerPrePostfix(e, k, span)
		return h, true, span, err
	case hirMethod:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		h, err := c.lowerMethod(e, k, pos, span)
		return h, true, span, err
	case hirSequence:
		if pos.kind == posLHS {
			return c.lhsError(span)
		}
		var last ir.ExpressionHandle
		for _, item := range k.Exprs {
			h, _, err := c.lowerExpect(e, item, rhsPos)
			if err != nil {
				return 0, false, span, err
			}
			last = h
		}
		return last, true, span, nil
	default:
		return 0, false, span, errInternal(fmt.Sprintf("%T cannot be lowered", node.Kind), span)
	}
}

// lhsError is the error Rust naga reports for an expression that cannot be
// assigned through (context.rs:1380-1388).
func (c *Context) lhsError(span ir.Span) (ir.ExpressionHandle, bool, ir.Span, *frontError) {
	return 0, false, span, errMessage(span, "Expression cannot be in the left hand side")
}

// lowerAccess lowers a[index], Rust naga's ExprPos-driven access lowering
// (context.rs:556-595). A constant index becomes an AccessIndex; a dynamic
// one on a constant name goes through a proxy local, so the access has a
// pointer to work on.
func (c *Context) lowerAccess(e *hirArena, k hirAccess, pos exprPos, span ir.Span) (ir.ExpressionHandle, bool, ir.Span, *frontError) {
	index, _, err := c.lowerExpect(e, k.Index, rhsPos)
	if err != nil {
		return 0, false, span, err
	}
	var constIndex *uint32
	if pos.kind != posLHS {
		if v, ok := c.constantU32(index); ok {
			constIndex = &v
		}
	}

	base, _, err := c.lowerExpect(e, k.Base, pos.maybeAccessBase(constIndex != nil))
	if err != nil {
		return 0, false, span, err
	}

	var pointer ir.ExpressionHandle
	if constIndex != nil {
		pointer, err = c.addExpression(ir.ExprAccessIndex{Base: base, Index: *constIndex}, span)
	} else {
		pointer, err = c.addExpression(ir.ExprAccess{Base: base, Index: index}, span)
	}
	if err != nil {
		return 0, false, span, err
	}

	if pos.kind == posRHS {
		ty, err := c.resolveType(pointer, span)
		if err != nil {
			return 0, false, span, err
		}
		if isPointerInner(ty) {
			h, err := c.addExpression(ir.ExprLoad{Pointer: pointer}, span)
			return h, true, span, err
		}
	}
	return pointer, true, span, nil
}

// isPointerInner reports whether ty is a pointer, whose value must be loaded
// through it.
func isPointerInner(ty ir.TypeInner) bool {
	switch ty.(type) {
	case ir.PointerType, ir.ValuePointerType:
		return true
	default:
		return false
	}
}

// constantU32 returns the u32 value of a constant index expression, Rust
// naga's GlobalCtx::get_const_val_from::<u32> (proc/mod.rs:571-611): a
// literal, a zero value of an integer type, or a constant whose initializer
// is one of those. It deliberately does not fold arithmetic: Rust produces a
// dynamic Access for an index it cannot read this way, and so do we.
//
// Rust reads the expression from its function arena here even in a const
// context, where the expressions are in the module's global arena; a const
// array access would index the empty function arena there. We read the arena
// the expression lives in, so a valid shader lowers instead of panicking.
func (c *Context) constantU32(expr ir.ExpressionHandle) (uint32, bool) {
	switch kind := c.Expression(expr).(type) {
	case ir.Literal:
		return literalU32(kind.Value)
	case ir.ExprZeroValue:
		scalar, ok := scalarComponents(c.mod.Types[kind.Type].Inner)
		if !ok {
			return 0, false
		}
		return zeroLiteralU32(scalar)
	case ir.ExprConstant:
		if int(kind.Constant) >= len(c.mod.Constants) {
			return 0, false
		}
		init := c.mod.Constants[kind.Constant].Init
		if int(init) >= len(c.mod.GlobalExpressions) {
			return 0, false
		}
		return c.constantU32Global(init)
	default:
		return 0, false
	}
}

// constantU32Global reads a constant's initializer from the module's global
// expression arena.
func (c *Context) constantU32Global(expr ir.ExpressionHandle) (uint32, bool) {
	switch kind := c.mod.GlobalExpressions[expr].Kind.(type) {
	case ir.Literal:
		return literalU32(kind.Value)
	case ir.ExprZeroValue:
		scalar, ok := scalarComponents(c.mod.Types[kind.Type].Inner)
		if !ok {
			return 0, false
		}
		return zeroLiteralU32(scalar)
	default:
		return 0, false
	}
}

// literalU32 converts a literal to u32 as Rust's TryFrom<Literal> for u32
// does (proc/mod.rs:242-254): unsigned 16- and 32-bit values, and signed ones
// that are not negative.
func literalU32(lit ir.LiteralValue) (uint32, bool) {
	v, err := u32FromLiteral(lit)
	return v, err == nil
}

// zeroLiteralU32 is the value zero of an integer scalar type, as Rust's
// Literal::zero gives it (types.rs:587-590).
func zeroLiteralU32(s ir.ScalarType) (uint32, bool) {
	switch s.Kind {
	case ir.ScalarSint, ir.ScalarUint:
		return 0, true
	default:
		return 0, false
	}
}

// lowerVariable lowers a reference to a declared name, Rust naga's
// HirExprKind::Variable arm (context.rs:1024-1082).
func (c *Context) lowerVariable(k hirVariable, pos exprPos, span ir.Span) (ir.ExpressionHandle, *frontError) {
	ref, err := c.variable(k.Sym, span)
	if err != nil {
		return 0, err
	}
	switch pos.kind {
	case posLHS:
		if !ref.mutable {
			c.fe.addError(errMessage(span, "Variable cannot be used in LHS position"))
		}
		return ref.expr, nil
	case posAccessBase:
		// A name declared as a constant has no pointer to index dynamically,
		// so the access goes through a local initialized with the constant.
		if !pos.constantIndex && ref.constant != nil {
			init, e := c.addExpression(ir.ExprConstant{Constant: *ref.constant}, ir.Span{})
			if e != nil {
				return 0, e
			}
			h := ir.LocalVariableHandle(len(c.locals))
			c.locals = append(c.locals, ir.LocalVariable{Type: ref.constTy, Init: &init})
			return c.addExpression(ir.ExprLocalVariable{Variable: h}, ir.Span{})
		}
		return ref.expr, nil
	}

	if ref.load {
		return c.addExpression(ir.ExprLoad{Pointer: ref.expr}, span)
	}
	if c.isConst && ref.constant != nil {
		return c.addExpression(ir.ExprConstant{Constant: *ref.constant}, span)
	}
	if c.isConst {
		if ov, ok := c.Expression(ref.expr).(ir.ExprOverride); ok {
			// The name's override expression lives in the function arena; a
			// const context needs it in the module's global arena.
			return c.addExpression(ir.ExprOverride{Override: ov.Override}, span)
		}
	}
	return ref.expr, nil
}

// lowerUnary lowers a unary operator, turning matrix negation into a
// multiplication by -1, which is how Rust naga says it to the IR
// (context.rs:997-1023).
func (c *Context) lowerUnary(e *hirArena, k hirUnary, span ir.Span) (ir.ExpressionHandle, *frontError) {
	expr, _, err := c.lowerExpect(e, k.Expr, rhsPos)
	if err != nil {
		return 0, err
	}
	ty, err := c.resolveType(expr, span)
	if err != nil {
		return 0, err
	}
	matrix, ok := ty.(ir.MatrixType)
	if !ok {
		return c.addExpression(ir.ExprUnary{Op: k.Op, Expr: expr}, span)
	}
	minusOne, ok := minusOneLiteral(matrix.Scalar)
	if !ok {
		return 0, errSemantic(span, "Cannot apply operator %s to type %s", unaryOpDebug(k.Op), scalarDebug(matrix.Scalar))
	}
	lhs, err := c.addExpression(ir.Literal{Value: minusOne}, span)
	if err != nil {
		return 0, err
	}
	return c.addExpression(ir.ExprBinary{Op: ir.BinaryMultiply, Left: lhs, Right: expr}, span)
}

// minusOneLiteral is the value -1 of a float scalar type, Rust's
// Literal::minus_one.
func minusOneLiteral(s ir.ScalarType) (ir.LiteralValue, bool) {
	if s.Kind != ir.ScalarFloat {
		return nil, false
	}
	switch s.Width {
	case 2:
		return ir.LiteralF16(-1), true
	case 4:
		return ir.LiteralF32(-1), true
	case 8:
		return ir.LiteralF64(-1), true
	default:
		return nil, false
	}
}

// unaryOpDebug renders a unary operator as Rust's Debug does.
func unaryOpDebug(op ir.UnaryOperator) string {
	switch op {
	case ir.UnaryNegate:
		return "Negate"
	case ir.UnaryLogicalNot:
		return "LogicalNot"
	case ir.UnaryBitwiseNot:
		return "BitwiseNot"
	}
	return fmt.Sprintf("UnaryOperator(%d)", op)
}

// lowerBinary lowers a binary operator with GLSL's implicit conversion of the
// weaker side, Rust naga's HirExprKind::Binary arm (context.rs:604-996).
func (c *Context) lowerBinary(e *hirArena, k hirBinary, span ir.Span) (ir.ExpressionHandle, *frontError) {
	left, leftSpan, err := c.lowerExpect(e, k.Left, rhsPos)
	if err != nil {
		return 0, err
	}
	right, rightSpan, err := c.lowerExpect(e, k.Right, rhsPos)
	if err != nil {
		return 0, err
	}

	switch k.Op {
	case ir.BinaryShiftLeft, ir.BinaryShiftRight:
		right, err = c.implicitConversion(right, rightSpan, ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	default:
		left, right, err = c.binaryImplicitConversion(left, leftSpan, right, rightSpan)
	}
	if err != nil {
		return 0, err
	}

	if err := c.typifierGrow(leftSpan); err != nil {
		return 0, err
	}
	leftTy := c.getType(left)
	rightTy := c.getType(right)

	switch l := leftTy.(type) {
	case ir.MatrixType:
		switch r := rightTy.(type) {
		case ir.MatrixType:
			return c.lowerMatrixBinary(left, right, l, r, k.Op, span)
		case ir.ScalarType:
			return c.lowerMatrixScalarBinary(left, right, l, r, k.Op, span)
		}
	case ir.VectorType:
		switch rightTy.(type) {
		case ir.VectorType:
			return c.lowerVectorBinary(left, right, k.Op, span)
		case ir.ScalarType:
			return c.lowerVectorScalarBinary(left, right, l.Size, k.Op, span)
		}
	case ir.ScalarType:
		switch r := rightTy.(type) {
		case ir.VectorType:
			return c.lowerScalarVectorBinary(left, right, r.Size, k.Op, span)
		case ir.MatrixType:
			return c.lowerScalarMatrixBinary(left, right, l, r, k.Op, span)
		}
	}
	return c.addExpression(ir.ExprBinary{Op: k.Op, Left: left, Right: right}, span)
}

// lowerMatrixBinary lowers a matrix-matrix operation: division column by
// column, comparison through a fold of vector comparisons.
func (c *Context) lowerMatrixBinary(left, right ir.ExpressionHandle, l, r ir.MatrixType, op ir.BinaryOperator, span ir.Span) (ir.ExpressionHandle, *frontError) {
	dimensionsOK := l.Columns == r.Columns && l.Rows == r.Rows
	if op == ir.BinaryMultiply {
		dimensionsOK = l.Columns == r.Rows
	}
	if !dimensionsOK || l.Scalar != r.Scalar {
		c.fe.addError(errSemantic(span, "Cannot apply operation to %s and %s", typeInnerIRDebug(l), typeInnerIRDebug(r)))
	}

	switch op {
	case ir.BinaryDivide:
		// The IR has no matrix division: divide the columns and rebuild.
		components := make([]ir.ExpressionHandle, 0, l.Columns)
		for i := uint32(0); i < uint32(l.Columns); i++ {
			leftColumn, err := c.addExpression(ir.ExprAccessIndex{Base: left, Index: i}, span)
			if err != nil {
				return 0, err
			}
			rightColumn, err := c.addExpression(ir.ExprAccessIndex{Base: right, Index: i}, span)
			if err != nil {
				return 0, err
			}
			column, err := c.addExpression(ir.ExprBinary{Op: op, Left: leftColumn, Right: rightColumn}, span)
			if err != nil {
				return 0, err
			}
			components = append(components, column)
		}
		ty := c.fe.registerType(l)
		return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
	case ir.BinaryEqual, ir.BinaryNotEqual:
		// The IR has no matrix comparison: compare columns, fold with a
		// relational function and a logical operator.
		equals := op == ir.BinaryEqual
		vectorOp := ir.BinaryNotEqual
		combine := ir.BinaryLogicalOr
		fun := ir.RelationalAny
		if equals {
			vectorOp = ir.BinaryEqual
			combine = ir.BinaryLogicalAnd
			fun = ir.RelationalAll
		}
		var root ir.ExpressionHandle
		haveRoot := false
		for i := uint32(0); i < uint32(l.Columns); i++ {
			leftColumn, err := c.addExpression(ir.ExprAccessIndex{Base: left, Index: i}, span)
			if err != nil {
				return 0, err
			}
			rightColumn, err := c.addExpression(ir.ExprAccessIndex{Base: right, Index: i}, span)
			if err != nil {
				return 0, err
			}
			argument, err := c.addExpression(ir.ExprBinary{Op: vectorOp, Left: leftColumn, Right: rightColumn}, span)
			if err != nil {
				return 0, err
			}
			compare, err := c.addExpression(ir.ExprRelational{Fun: fun, Argument: argument}, span)
			if err != nil {
				return 0, err
			}
			if haveRoot {
				root, err = c.addExpression(ir.ExprBinary{Op: combine, Left: compare, Right: root}, span)
				if err != nil {
					return 0, err
				}
			} else {
				root, haveRoot = compare, true
			}
		}
		return root, nil
	default:
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: right}, span)
	}
}

// lowerVectorBinary lowers a vector-vector operation: a comparison becomes a
// relational All or Any over the component-wise comparison.
func (c *Context) lowerVectorBinary(left, right ir.ExpressionHandle, op ir.BinaryOperator, span ir.Span) (ir.ExpressionHandle, *frontError) {
	switch op {
	case ir.BinaryEqual, ir.BinaryNotEqual:
		vectorOp := ir.BinaryNotEqual
		fun := ir.RelationalAny
		if op == ir.BinaryEqual {
			vectorOp = ir.BinaryEqual
			fun = ir.RelationalAll
		}
		argument, err := c.addExpression(ir.ExprBinary{Op: vectorOp, Left: left, Right: right}, span)
		if err != nil {
			return 0, err
		}
		return c.addExpression(ir.ExprRelational{Fun: fun, Argument: argument}, span)
	default:
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: right}, span)
	}
}

// lowerVectorScalarBinary splats the scalar some vector-scalar operations need
// as a vector operand (context.rs:789-811).
func (c *Context) lowerVectorScalarBinary(left, right ir.ExpressionHandle, size ir.VectorSize, op ir.BinaryOperator, span ir.Span) (ir.ExpressionHandle, *frontError) {
	switch op {
	case ir.BinaryAdd, ir.BinarySubtract, ir.BinaryDivide, ir.BinaryAnd, ir.BinaryExclusiveOr,
		ir.BinaryInclusiveOr, ir.BinaryShiftLeft, ir.BinaryShiftRight:
		splat, err := c.addExpression(ir.ExprSplat{Size: size, Value: right}, span)
		if err != nil {
			return 0, err
		}
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: splat}, span)
	default:
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: right}, span)
	}
}

// lowerScalarVectorBinary splats the scalar some scalar-vector operations need
// as the left operand (context.rs:812-832).
func (c *Context) lowerScalarVectorBinary(left, right ir.ExpressionHandle, size ir.VectorSize, op ir.BinaryOperator, span ir.Span) (ir.ExpressionHandle, *frontError) {
	switch op {
	case ir.BinaryAdd, ir.BinarySubtract, ir.BinaryDivide, ir.BinaryAnd, ir.BinaryExclusiveOr,
		ir.BinaryInclusiveOr:
		splat, err := c.addExpression(ir.ExprSplat{Size: size, Value: left}, span)
		if err != nil {
			return 0, err
		}
		return c.addExpression(ir.ExprBinary{Op: op, Left: splat, Right: right}, span)
	default:
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: right}, span)
	}
}

// lowerScalarMatrixBinary lowers the scalar-matrix operations the IR lacks by
// splatting the scalar and applying it to each column (context.rs:833-912).
func (c *Context) lowerScalarMatrixBinary(left, right ir.ExpressionHandle, leftScalar ir.ScalarType, m ir.MatrixType, op ir.BinaryOperator, span ir.Span) (ir.ExpressionHandle, *frontError) {
	// Rust reports the mismatch and goes on lowering; the operation is the
	// shader's error either way.
	if leftScalar != m.Scalar {
		c.fe.addError(errSemantic(span, "Cannot apply operation to %s and %s",
			typeInnerIRDebug(leftScalar), typeInnerIRDebug(m)))
	}
	if op != ir.BinaryDivide && op != ir.BinaryAdd && op != ir.BinarySubtract {
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: right}, span)
	}
	splat, err := c.addExpression(ir.ExprSplat{Size: m.Rows, Value: left}, span)
	if err != nil {
		return 0, err
	}
	components := make([]ir.ExpressionHandle, 0, m.Columns)
	for i := uint32(0); i < uint32(m.Columns); i++ {
		column, err := c.addExpression(ir.ExprAccessIndex{Base: right, Index: i}, span)
		if err != nil {
			return 0, err
		}
		result, err := c.addExpression(ir.ExprBinary{Op: op, Left: splat, Right: column}, span)
		if err != nil {
			return 0, err
		}
		components = append(components, result)
	}
	ty := c.fe.registerType(m)
	return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
}

// lowerMatrixScalarBinary lowers the matrix-scalar operations the IR lacks by
// splatting the scalar and applying it to each column (context.rs:913-993).
func (c *Context) lowerMatrixScalarBinary(left, right ir.ExpressionHandle, m ir.MatrixType, rightScalar ir.ScalarType, op ir.BinaryOperator, span ir.Span) (ir.ExpressionHandle, *frontError) {
	if m.Scalar != rightScalar {
		c.fe.addError(errSemantic(span, "Cannot apply operation to %s and %s",
			typeInnerIRDebug(m), typeInnerIRDebug(rightScalar)))
	}
	if op != ir.BinaryDivide && op != ir.BinaryAdd && op != ir.BinarySubtract {
		return c.addExpression(ir.ExprBinary{Op: op, Left: left, Right: right}, span)
	}
	splat, err := c.addExpression(ir.ExprSplat{Size: m.Rows, Value: right}, span)
	if err != nil {
		return 0, err
	}
	components := make([]ir.ExpressionHandle, 0, m.Columns)
	for i := uint32(0); i < uint32(m.Columns); i++ {
		column, err := c.addExpression(ir.ExprAccessIndex{Base: left, Index: i}, span)
		if err != nil {
			return 0, err
		}
		result, err := c.addExpression(ir.ExprBinary{Op: op, Left: column, Right: splat}, span)
		if err != nil {
			return 0, err
		}
		components = append(components, result)
	}
	ty := c.fe.registerType(m)
	return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
}

// lowerAssign lowers an assignment expression, which yields the stored value,
// Rust naga's HirExprKind::Assign arm (context.rs:1213-1231).
func (c *Context) lowerAssign(e *hirArena, k hirAssign, span ir.Span) (ir.ExpressionHandle, *frontError) {
	pointer, ptrSpan, err := c.lowerExpect(e, k.Tgt, lhsPos)
	if err != nil {
		return 0, err
	}
	value, valueSpan, err := c.lowerExpect(e, k.Value, rhsPos)
	if err != nil {
		return 0, err
	}
	ty, err := c.resolveType(pointer, ptrSpan)
	if err != nil {
		return 0, err
	}
	if ptr, ok := ty.(ir.PointerType); ok {
		ty = c.mod.Types[ptr.Base].Inner
	}
	if scalar, ok := scalarComponents(ty); ok {
		value, err = c.implicitConversion(value, valueSpan, scalar)
		if err != nil {
			return 0, err
		}
	}
	if err := c.lowerStore(pointer, value, span); err != nil {
		return 0, err
	}
	return value, nil
}

// lowerConditional lowers the ternary operator to a temporary local and an If
// with both branches converted to the common type, Rust naga's
// HirExprKind::Conditional arm (context.rs:1093-1212).
func (c *Context) lowerConditional(e *hirArena, k hirConditional, span ir.Span) (ir.ExpressionHandle, *frontError) {
	condition, _, err := c.lowerExpect(e, k.Condition, rhsPos)
	if err != nil {
		return 0, err
	}

	var (
		accept, reject         ir.ExpressionHandle
		acceptSpan, rejectSpan ir.Span
	)
	acceptBody := make(ir.Block, 0, 1)
	err = c.swapBody(&acceptBody, func() *frontError {
		var e2 *frontError
		accept, acceptSpan, e2 = c.lowerExpect(e, k.Accept, rhsPos)
		return e2
	})
	if err != nil {
		return 0, err
	}
	rejectBody := make(ir.Block, 0, 1)
	err = c.swapBody(&rejectBody, func() *frontError {
		var e2 *frontError
		reject, rejectSpan, e2 = c.lowerExpect(e, k.Reject, rhsPos)
		return e2
	})
	if err != nil {
		return 0, err
	}

	// The two branches live in different bodies, so the conversion is applied
	// inside the branch that needs it.
	acceptScalar, acceptOK, err := c.exprScalarComponents(accept, acceptSpan)
	if err != nil {
		return 0, err
	}
	rejectScalar, rejectOK, err := c.exprScalarComponents(reject, rejectSpan)
	if err != nil {
		return 0, err
	}
	acceptPower, acceptRanked := typePower(acceptScalar)
	rejectPower, rejectRanked := typePower(rejectScalar)
	if acceptOK && rejectOK && acceptRanked && rejectRanked {
		switch {
		case acceptPower < rejectPower:
			err = c.swapBody(&acceptBody, func() *frontError {
				var e2 *frontError
				accept, e2 = c.conversion(accept, acceptSpan, rejectScalar)
				return e2
			})
		case acceptPower > rejectPower:
			err = c.swapBody(&rejectBody, func() *frontError {
				var e2 *frontError
				reject, e2 = c.conversion(reject, rejectSpan, acceptScalar)
				return e2
			})
		}
		if err != nil {
			return 0, err
		}
	}

	ty, err := c.resolveTypeHandle(accept, acceptSpan)
	if err != nil {
		return 0, err
	}

	local := ir.LocalVariableHandle(len(c.locals))
	c.locals = append(c.locals, ir.LocalVariable{Type: ty})
	localExpr, err := c.addExpression(ir.ExprLocalVariable{Variable: local}, span)
	if err != nil {
		return 0, err
	}

	acceptBody = append(acceptBody, ir.Statement{Kind: ir.StmtStore{Pointer: localExpr, Value: accept}})
	rejectBody = append(rejectBody, ir.Statement{Kind: ir.StmtStore{Pointer: localExpr, Value: reject}})
	c.body = append(c.body, ir.Statement{Kind: ir.StmtIf{
		Condition: condition,
		Accept:    acceptBody,
		Reject:    rejectBody,
	}})

	// The load must be emitted before it is used.
	return c.addExpression(ir.ExprLoad{Pointer: localExpr}, span)
}

// lowerPrePostfix lowers ++ and -- on a scalar, vector or matrix, Rust naga's
// HirExprKind::PrePostfix arm (context.rs:1232-1313).
func (c *Context) lowerPrePostfix(e *hirArena, k hirPrePostfix, span ir.Span) (ir.ExpressionHandle, *frontError) {
	pointer, _, err := c.lowerExpect(e, k.Expr, lhsPos)
	if err != nil {
		return 0, err
	}
	left := pointer
	if _, ok := c.Expression(pointer).(ir.ExprSwizzle); !ok {
		left, err = c.addExpression(ir.ExprLoad{Pointer: pointer}, span)
		if err != nil {
			return 0, err
		}
	}

	ty, err := c.resolveType(left, span)
	if err != nil {
		return 0, err
	}
	var (
		scalar  ir.ScalarType
		size    *ir.VectorSize
		columns *ir.VectorSize
	)
	switch t := ty.(type) {
	case ir.ScalarType:
		scalar = t
	case ir.VectorType:
		scalar, size = t.Scalar, &t.Size
	case ir.MatrixType:
		scalar, size, columns = t.Scalar, &t.Rows, &t.Columns
	default:
		c.fe.addError(errMessage(span, "Increment/decrement only works on scalar/vector/matrix"))
		return left, nil
	}

	one, ok := oneLiteral(scalar)
	if !ok {
		c.fe.addError(errMessage(span, "Increment/decrement only works on scalar/vector/matrix"))
		return left, nil
	}
	right, err := c.addExpression(ir.Literal{Value: one}, span)
	if err != nil {
		return 0, err
	}

	// A vector or matrix target needs the one splatted to its shape.
	if size != nil {
		right, err = c.addExpression(ir.ExprSplat{Size: *size, Value: right}, span)
		if err != nil {
			return 0, err
		}
		if columns != nil {
			ty := c.fe.registerType(ir.MatrixType{Columns: *columns, Rows: *size, Scalar: scalar})
			components := make([]ir.ExpressionHandle, int(*columns))
			for i := range components {
				components[i] = right
			}
			right, err = c.addExpression(ir.ExprCompose{Type: ty, Components: components}, span)
			if err != nil {
				return 0, err
			}
		}
	}

	value, err := c.addExpression(ir.ExprBinary{Op: k.Op, Left: left, Right: right}, span)
	if err != nil {
		return 0, err
	}
	if err := c.lowerStore(pointer, value, span); err != nil {
		return 0, err
	}
	if k.Postfix {
		return left, nil
	}
	return value, nil
}

// oneLiteral is the value one of a scalar type, Rust's Literal::one.
func oneLiteral(s ir.ScalarType) (ir.LiteralValue, bool) {
	switch s.Kind {
	case ir.ScalarSint:
		switch s.Width {
		case 2:
			return ir.LiteralI16(1), true
		case 4:
			return ir.LiteralI32(1), true
		case 8:
			return ir.LiteralI64(1), true
		}
	case ir.ScalarUint:
		switch s.Width {
		case 2:
			return ir.LiteralU16(1), true
		case 4:
			return ir.LiteralU32(1), true
		case 8:
			return ir.LiteralU64(1), true
		}
	case ir.ScalarFloat:
		switch s.Width {
		case 2:
			return ir.LiteralF16(1), true
		case 4:
			return ir.LiteralF32(1), true
		case 8:
			return ir.LiteralF64(1), true
		}
	}
	return nil, false
}

// lowerMethod lowers a method call, which in GLSL is only .length(), Rust
// naga's HirExprKind::Method arm (context.rs:1314-1379).
func (c *Context) lowerMethod(e *hirArena, k hirMethod, pos exprPos, span ir.Span) (ir.ExpressionHandle, *frontError) {
	args := make([]ir.ExpressionHandle, 0, len(k.Args))
	for _, arg := range k.Args {
		h, _, err := c.lowerExpect(e, arg, rhsPos)
		if err != nil {
			return 0, err
		}
		args = append(args, h)
	}
	if k.Name != "length" {
		return 0, errSemantic(span, "unknown method '%s'", k.Name)
	}

	if len(args) != 0 {
		c.fe.addError(errMessage(span, ".length() doesn't take any arguments"))
	}
	array, _, err := c.lowerExpect(e, k.Expr, pos)
	if err != nil {
		return 0, err
	}
	ty, err := c.resolveType(array, span)
	if err != nil {
		return 0, err
	}
	if arr, ok := ty.(ir.ArrayType); ok && arr.Size.Constant != nil {
		length, err := c.addExpression(ir.Literal{Value: ir.LiteralU32(*arr.Size.Constant)}, span)
		if err != nil {
			return 0, err
		}
		return c.forcedConversion(length, span, ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	}
	length, err := c.addExpression(ir.ExprArrayLength{Array: array}, span)
	if err != nil {
		return 0, err
	}
	return c.conversion(length, span, ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
}

// typeInnerIRDebug renders an IR type inner as Rust's Debug of TypeInner
// does, for the lowering errors that name the types they cannot combine.
func typeInnerIRDebug(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return "Scalar(" + scalarDebug(t) + ")"
	case ir.VectorType:
		return fmt.Sprintf("Vector { size: %s, scalar: %s }",
			vectorSizeNames[t.Size], scalarDebug(t.Scalar))
	case ir.MatrixType:
		return fmt.Sprintf("Matrix { columns: %s, rows: %s, scalar: %s }",
			vectorSizeNames[t.Columns], vectorSizeNames[t.Rows], scalarDebug(t.Scalar))
	default:
		return fmt.Sprintf("TypeInner(%T)", inner)
	}
}
