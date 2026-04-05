package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// emitExpression evaluates a single expression and returns its DXIL value ID.
// For vector types, returns the ID of the first component.
//
// Reference: Mesa nir_to_dxil.c emit_alu()
func (e *Emitter) emitExpression(fn *ir.Function, handle ir.ExpressionHandle) (int, error) {
	// Check if already emitted.
	if v, ok := e.exprValues[handle]; ok {
		return v, nil
	}

	expr := fn.Expressions[handle]
	var valueID int
	var err error

	switch ek := expr.Kind.(type) {
	case ir.Literal:
		valueID, err = e.emitLiteral(ek)

	case ir.ExprFunctionArgument:
		// Should have been set up by emitInputLoads.
		return 0, fmt.Errorf("function argument %d not in value map", ek.Index)

	case ir.ExprCompose:
		valueID, err = e.emitCompose(fn, ek)

	case ir.ExprAccessIndex:
		valueID, err = e.emitAccessIndex(fn, ek)

	case ir.ExprSplat:
		valueID, err = e.emitSplat(fn, ek)

	case ir.ExprBinary:
		valueID, err = e.emitBinary(fn, ek)

	case ir.ExprUnary:
		valueID, err = e.emitUnary(fn, ek)

	case ir.ExprMath:
		valueID, err = e.emitMath(fn, ek)

	case ir.ExprSelect:
		valueID, err = e.emitSelect(fn, ek)

	case ir.ExprLoad:
		valueID, err = e.emitLoad(fn, ek)

	case ir.ExprLocalVariable:
		// Pointer to local variable. For now, treat as the value itself.
		valueID = e.allocValue()

	case ir.ExprGlobalVariable:
		// Global variable reference. For now, treat as a constant.
		valueID = e.allocValue()

	case ir.ExprZeroValue:
		valueID, err = e.emitZeroValue(ek)

	case ir.ExprConstant:
		valueID, err = e.emitConstant(ek)

	case ir.ExprAs:
		valueID, err = e.emitAs(fn, ek)

	default:
		// Unsupported expression kind — allocate a placeholder value.
		valueID = e.allocValue()
	}

	if err != nil {
		return 0, fmt.Errorf("expression [%d]: %w", handle, err)
	}

	e.exprValues[handle] = valueID

	// If compose produced per-component IDs, store them.
	if e.pendingComponents != nil {
		e.exprComponents[handle] = e.pendingComponents
		e.pendingComponents = nil
	}

	return valueID, nil
}

// emitLiteral emits a constant value and returns its value ID.
func (e *Emitter) emitLiteral(lit ir.Literal) (int, error) {
	switch v := lit.Value.(type) {
	case ir.LiteralF32:
		return e.getFloatConstID(float64(v)), nil

	case ir.LiteralF64:
		return e.addFloatConstID(e.mod.GetFloatType(64), float64(v)), nil

	case ir.LiteralI32:
		return e.getIntConstID(int64(v)), nil

	case ir.LiteralU32:
		return e.getIntConstID(int64(v)), nil

	case ir.LiteralI64:
		c := e.mod.AddIntConst(e.mod.GetIntType(64), int64(v))
		id := e.allocValue()
		e.constMap[id] = c
		return id, nil

	case ir.LiteralU64:
		c := e.mod.AddIntConst(e.mod.GetIntType(64), int64(v)) //nolint:gosec // value may exceed int64 range
		id := e.allocValue()
		e.constMap[id] = c
		return id, nil

	case ir.LiteralBool:
		val := int64(0)
		if bool(v) {
			val = 1
		}
		c := e.mod.AddIntConst(e.mod.GetIntType(1), val)
		id := e.allocValue()
		e.constMap[id] = c
		return id, nil

	case ir.LiteralF16:
		return e.addFloatConstID(e.mod.GetFloatType(16), float64(v)), nil

	default:
		return e.getIntConstID(0), nil
	}
}

// emitCompose builds a composite value from components.
// In DXIL, composites are scalarized — each component is a separate value.
// Returns the value ID of the first component. Also tracks per-component
// IDs for correct access later.
func (e *Emitter) emitCompose(fn *ir.Function, comp ir.ExprCompose) (int, error) {
	if len(comp.Components) == 0 {
		return e.getIntConstID(0), nil
	}

	componentIDs := make([]int, len(comp.Components))
	firstID := -1
	for i, ch := range comp.Components {
		v, err := e.emitExpression(fn, ch)
		if err != nil {
			return 0, err
		}
		componentIDs[i] = v
		if firstID < 0 {
			firstID = v
		}
	}

	// Store per-component IDs so getComponentID can resolve them.
	// The compose expression handle will be set by the caller
	// (emitExpression stores in exprValues).
	e.pendingComponents = componentIDs

	return firstID, nil
}

// emitAccessIndex extracts a component from a composite by constant index.
func (e *Emitter) emitAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex) (int, error) {
	_, err := e.emitExpression(fn, ai.Base)
	if err != nil {
		return 0, err
	}

	// Use per-component tracking for correct ID resolution.
	return e.getComponentID(ai.Base, int(ai.Index)), nil
}

// emitSplat broadcasts a scalar to all components of a vector.
func (e *Emitter) emitSplat(fn *ir.Function, sp ir.ExprSplat) (int, error) {
	v, err := e.emitExpression(fn, sp.Value)
	if err != nil {
		return 0, err
	}
	// In DXIL, all components share the same value — just return it.
	return v, nil
}

// emitBinary emits a binary operation.
//
//nolint:gocognit,gocyclo,cyclop,funlen // binary op dispatch requires many cases
func (e *Emitter) emitBinary(fn *ir.Function, bin ir.ExprBinary) (int, error) {
	lhs, err := e.emitExpression(fn, bin.Left)
	if err != nil {
		return 0, err
	}
	rhs, err := e.emitExpression(fn, bin.Right)
	if err != nil {
		return 0, err
	}

	// Determine result type from left operand.
	leftType := e.resolveExprType(fn, bin.Left)
	isFloat := isFloatType(leftType)
	isSigned := isSignedInt(leftType)
	resultTy, _ := typeToDXIL(e.mod, e.ir, leftType)

	switch bin.Op {
	case ir.BinaryAdd:
		if isFloat {
			return e.addBinOpInstr(resultTy, BinOpFAdd, lhs, rhs), nil
		}
		return e.addBinOpInstr(resultTy, BinOpAdd, lhs, rhs), nil

	case ir.BinarySubtract:
		if isFloat {
			return e.addBinOpInstr(resultTy, BinOpFSub, lhs, rhs), nil
		}
		return e.addBinOpInstr(resultTy, BinOpSub, lhs, rhs), nil

	case ir.BinaryMultiply:
		if isFloat {
			return e.addBinOpInstr(resultTy, BinOpFMul, lhs, rhs), nil
		}
		return e.addBinOpInstr(resultTy, BinOpMul, lhs, rhs), nil

	case ir.BinaryDivide:
		if isFloat {
			return e.addBinOpInstr(resultTy, BinOpFDiv, lhs, rhs), nil
		}
		if isSigned {
			return e.addBinOpInstr(resultTy, BinOpSDiv, lhs, rhs), nil
		}
		return e.addBinOpInstr(resultTy, BinOpUDiv, lhs, rhs), nil

	case ir.BinaryModulo:
		if isFloat {
			return e.addBinOpInstr(resultTy, BinOpFRem, lhs, rhs), nil
		}
		if isSigned {
			return e.addBinOpInstr(resultTy, BinOpSRem, lhs, rhs), nil
		}
		return e.addBinOpInstr(resultTy, BinOpURem, lhs, rhs), nil

	case ir.BinaryAnd:
		return e.addBinOpInstr(resultTy, BinOpAnd, lhs, rhs), nil
	case ir.BinaryExclusiveOr:
		return e.addBinOpInstr(resultTy, BinOpXor, lhs, rhs), nil
	case ir.BinaryInclusiveOr:
		return e.addBinOpInstr(resultTy, BinOpOr, lhs, rhs), nil

	case ir.BinaryShiftLeft:
		return e.addBinOpInstr(resultTy, BinOpShl, lhs, rhs), nil
	case ir.BinaryShiftRight:
		if isSigned {
			return e.addBinOpInstr(resultTy, BinOpAShr, lhs, rhs), nil
		}
		return e.addBinOpInstr(resultTy, BinOpLShr, lhs, rhs), nil

	// Comparison operations.
	case ir.BinaryEqual:
		if isFloat {
			return e.addCmpInstr(FCmpOEQ, lhs, rhs), nil
		}
		return e.addCmpInstr(ICmpEQ, lhs, rhs), nil

	case ir.BinaryNotEqual:
		if isFloat {
			return e.addCmpInstr(FCmpONE, lhs, rhs), nil
		}
		return e.addCmpInstr(ICmpNE, lhs, rhs), nil

	case ir.BinaryLess:
		if isFloat {
			return e.addCmpInstr(FCmpOLT, lhs, rhs), nil
		}
		if isSigned {
			return e.addCmpInstr(ICmpSLT, lhs, rhs), nil
		}
		return e.addCmpInstr(ICmpULT, lhs, rhs), nil

	case ir.BinaryLessEqual:
		if isFloat {
			return e.addCmpInstr(FCmpOLE, lhs, rhs), nil
		}
		if isSigned {
			return e.addCmpInstr(ICmpSLE, lhs, rhs), nil
		}
		return e.addCmpInstr(ICmpULE, lhs, rhs), nil

	case ir.BinaryGreater:
		if isFloat {
			return e.addCmpInstr(FCmpOGT, lhs, rhs), nil
		}
		if isSigned {
			return e.addCmpInstr(ICmpSGT, lhs, rhs), nil
		}
		return e.addCmpInstr(ICmpUGT, lhs, rhs), nil

	case ir.BinaryGreaterEqual:
		if isFloat {
			return e.addCmpInstr(FCmpOGE, lhs, rhs), nil
		}
		if isSigned {
			return e.addCmpInstr(ICmpSGE, lhs, rhs), nil
		}
		return e.addCmpInstr(ICmpUGE, lhs, rhs), nil

	case ir.BinaryLogicalAnd:
		return e.addBinOpInstr(e.mod.GetIntType(1), BinOpAnd, lhs, rhs), nil
	case ir.BinaryLogicalOr:
		return e.addBinOpInstr(e.mod.GetIntType(1), BinOpOr, lhs, rhs), nil

	default:
		return 0, fmt.Errorf("unsupported binary operator: %d", bin.Op)
	}
}

// emitUnary emits a unary operation.
func (e *Emitter) emitUnary(fn *ir.Function, un ir.ExprUnary) (int, error) {
	operand, err := e.emitExpression(fn, un.Expr)
	if err != nil {
		return 0, err
	}

	exprType := e.resolveExprType(fn, un.Expr)
	isFloat := isFloatType(exprType)
	resultTy, _ := typeToDXIL(e.mod, e.ir, exprType)

	switch un.Op {
	case ir.UnaryNegate:
		if isFloat {
			// fneg = fsub -0.0, operand
			zeroID := e.getFloatConstID(0.0)
			return e.addBinOpInstr(resultTy, BinOpFSub, zeroID, operand), nil
		}
		// neg = sub 0, operand
		zeroID := e.getIntConstID(0)
		return e.addBinOpInstr(resultTy, BinOpSub, zeroID, operand), nil

	case ir.UnaryBitwiseNot:
		// not = xor operand, -1
		allOnesID := e.getIntConstID(-1)
		return e.addBinOpInstr(resultTy, BinOpXor, operand, allOnesID), nil

	case ir.UnaryLogicalNot:
		// not = xor operand, true
		c := e.mod.AddIntConst(e.mod.GetIntType(1), 1)
		trueID := e.allocValue()
		e.constMap[trueID] = c
		return e.addBinOpInstr(e.mod.GetIntType(1), BinOpXor, operand, trueID), nil

	default:
		return 0, fmt.Errorf("unsupported unary operator: %d", un.Op)
	}
}

// emitMath emits a math intrinsic as a dx.op call.
func (e *Emitter) emitMath(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)

	opcode, opName, err := mathToDxOp(mathExpr.Fun)
	if err != nil {
		return 0, err
	}

	dxFn := e.getDxOpUnaryFunc(opName, ol)
	opcodeVal := e.getIntConstID(int64(opcode))

	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg}), nil
}

// mathToDxOp maps naga MathFunction to dx.op opcode and name.
//
//nolint:gocyclo,cyclop // math function mapping requires many cases
func mathToDxOp(mf ir.MathFunction) (DXILOpcode, string, error) {
	switch mf {
	case ir.MathSin:
		return OpSin, "dx.op.sin", nil
	case ir.MathCos:
		return OpCos, "dx.op.cos", nil
	case ir.MathTan:
		return OpTan, "dx.op.tan", nil
	case ir.MathAsin:
		return OpAsin, "dx.op.asin", nil
	case ir.MathAcos:
		return OpAcos, "dx.op.acos", nil
	case ir.MathAtan:
		return OpAtan, "dx.op.atan", nil
	case ir.MathSinh:
		return OpHSin, "dx.op.hsin", nil
	case ir.MathCosh:
		return OpHCos, "dx.op.hcos", nil
	case ir.MathTanh:
		return OpHTan, "dx.op.htan", nil
	case ir.MathExp2:
		return OpExp, "dx.op.exp", nil
	case ir.MathLog2:
		return OpLog, "dx.op.log", nil
	case ir.MathSqrt:
		return OpSqrt, "dx.op.sqrt", nil
	case ir.MathInverseSqrt:
		return OpRsqrt, "dx.op.rsqrt", nil
	case ir.MathAbs:
		return OpFAbs, "dx.op.fabs", nil
	case ir.MathSaturate:
		return OpSaturate, "dx.op.saturate", nil
	case ir.MathFloor:
		return OpRoundNI, "dx.op.round_ni", nil
	case ir.MathCeil:
		return OpRoundPI, "dx.op.round_pi", nil
	case ir.MathTrunc:
		return OpRoundZ, "dx.op.round_z", nil
	case ir.MathRound:
		return OpRoundNE, "dx.op.round_ne", nil
	case ir.MathFract:
		return OpFrc, "dx.op.frc", nil
	case ir.MathReverseBits:
		return OpReverseBits, "dx.op.reverseBits", nil
	case ir.MathCountOneBits:
		return OpCountBits, "dx.op.countBits", nil
	case ir.MathFirstTrailingBit:
		return OpFirstbitLo, "dx.op.firstbitLo", nil
	case ir.MathFirstLeadingBit:
		return OpFirstbitHi, "dx.op.firstbitHi", nil
	default:
		return 0, "", fmt.Errorf("unsupported math function: %d", mf)
	}
}

// emitSelect emits a select (ternary) instruction.
func (e *Emitter) emitSelect(fn *ir.Function, sel ir.ExprSelect) (int, error) {
	cond, err := e.emitExpression(fn, sel.Condition)
	if err != nil {
		return 0, err
	}
	accept, err := e.emitExpression(fn, sel.Accept)
	if err != nil {
		return 0, err
	}
	reject, err := e.emitExpression(fn, sel.Reject)
	if err != nil {
		return 0, err
	}

	acceptType := e.resolveExprType(fn, sel.Accept)
	resultTy, _ := typeToDXIL(e.mod, e.ir, acceptType)

	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrSelect,
		HasValue:   true,
		ResultType: resultTy,
		Operands:   []int{cond, accept, reject},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID, nil
}

// emitLoad emits a load through a pointer.
func (e *Emitter) emitLoad(fn *ir.Function, load ir.ExprLoad) (int, error) {
	ptr, err := e.emitExpression(fn, load.Pointer)
	if err != nil {
		return 0, err
	}
	// In the simplified model, just return the pointer value.
	return ptr, nil
}

// emitZeroValue emits a zero-initialized constant.
func (e *Emitter) emitZeroValue(zv ir.ExprZeroValue) (int, error) { //nolint:unparam // error return for interface consistency
	ty := e.ir.Types[zv.Type]
	inner := ty.Inner

	switch inner.(type) {
	case ir.ScalarType:
		s := inner.(ir.ScalarType)
		if s.Kind == ir.ScalarFloat {
			return e.getFloatConstID(0.0), nil
		}
		return e.getIntConstID(0), nil
	case ir.VectorType:
		vt := inner.(ir.VectorType)
		if vt.Scalar.Kind == ir.ScalarFloat {
			return e.getFloatConstID(0.0), nil
		}
		return e.getIntConstID(0), nil
	default:
		return e.getIntConstID(0), nil
	}
}

// emitConstant emits a reference to a module-scope constant.
func (e *Emitter) emitConstant(ec ir.ExprConstant) (int, error) {
	c := &e.ir.Constants[ec.Constant]

	// Look at the init expression in GlobalExpressions.
	if int(c.Init) < len(e.ir.GlobalExpressions) {
		globalExpr := e.ir.GlobalExpressions[c.Init]
		if lit, ok := globalExpr.Kind.(ir.Literal); ok {
			return e.emitLiteral(lit)
		}
	}

	// Fallback: zero value.
	return e.getIntConstID(0), nil
}

// emitAs emits a type cast expression.
func (e *Emitter) emitAs(fn *ir.Function, as ir.ExprAs) (int, error) {
	src, err := e.emitExpression(fn, as.Expr)
	if err != nil {
		return 0, err
	}
	// Simplified: for now just pass through (proper casts need LLVM cast instructions).
	return src, nil
}

// resolveExprType returns the TypeInner for an expression.
func (e *Emitter) resolveExprType(fn *ir.Function, handle ir.ExpressionHandle) ir.TypeInner {
	if int(handle) < len(fn.ExpressionTypes) {
		tr := fn.ExpressionTypes[handle]
		if tr.Handle != nil {
			return e.ir.Types[*tr.Handle].Inner
		}
		if tr.Value != nil {
			return tr.Value
		}
	}

	// Fallback: try to infer from the expression itself.
	if int(handle) < len(fn.Expressions) {
		expr := fn.Expressions[handle]
		switch ek := expr.Kind.(type) {
		case ir.Literal:
			return literalType(ek)
		case ir.ExprFunctionArgument:
			if int(ek.Index) < len(fn.Arguments) {
				return e.ir.Types[fn.Arguments[ek.Index].Type].Inner
			}
		}
	}

	return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
}

// literalType returns the TypeInner for a literal expression.
func literalType(lit ir.Literal) ir.TypeInner {
	switch lit.Value.(type) {
	case ir.LiteralF32:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	case ir.LiteralF64:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	case ir.LiteralI32:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	case ir.LiteralU32:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	case ir.LiteralI64:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 8}
	case ir.LiteralU64:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 8}
	case ir.LiteralBool:
		return ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	case ir.LiteralF16:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	default:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
}
