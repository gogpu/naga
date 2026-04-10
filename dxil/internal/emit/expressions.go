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
//
//nolint:gocyclo,cyclop // expression dispatch requires handling all expression kinds
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

	case ir.ExprAccess:
		valueID, err = e.emitAccess(fn, ek)

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
		valueID, err = e.emitLocalVariable(fn, ek)

	case ir.ExprGlobalVariable:
		// If this is a resource binding, return the pre-created handle ID.
		if handleID, found := e.getResourceHandleID(ek.Variable); found {
			valueID = handleID
		} else {
			// Non-resource global variable — allocate a placeholder value.
			valueID = e.allocValue()
		}

	case ir.ExprImageSample:
		valueID, err = e.emitImageSample(fn, ek)

	case ir.ExprZeroValue:
		valueID, err = e.emitZeroValue(ek)

	case ir.ExprConstant:
		valueID, err = e.emitConstant(ek)

	case ir.ExprAs:
		valueID, err = e.emitAs(fn, ek)

	case ir.ExprSwizzle:
		valueID, err = e.emitSwizzle(fn, ek)

	case ir.ExprDerivative:
		valueID, err = e.emitDerivative(fn, ek)

	case ir.ExprRelational:
		valueID, err = e.emitRelational(fn, ek)

	case ir.ExprAtomicResult:
		// AtomicResult values are pre-populated by emitStmtAtomic.
		// If we reach here, the value should already be in exprValues.
		return 0, fmt.Errorf("ExprAtomicResult [%d] not yet populated by StmtAtomic", handle)

	case ir.ExprArrayLength:
		return 0, fmt.Errorf("ExprArrayLength not yet implemented in DXIL backend")

	default:
		return 0, fmt.Errorf("unsupported expression kind: %T", expr.Kind)
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
//
// For struct composites, flattens nested composes into a flat scalar list
// so that storeStructFields can correctly index into the flattened components.
func (e *Emitter) emitCompose(fn *ir.Function, comp ir.ExprCompose) (int, error) {
	if len(comp.Components) == 0 {
		return e.getIntConstID(0), nil
	}

	// Build a flattened list of all scalar component IDs.
	// When a component itself is a compose (struct or vector), include all its sub-components.
	var flatIDs []int
	for _, ch := range comp.Components {
		v, err := e.emitExpression(fn, ch)
		if err != nil {
			return 0, err
		}

		// Check if this component has sub-components (vector/struct compose).
		if subComps, ok := e.exprComponents[ch]; ok {
			flatIDs = append(flatIDs, subComps...)
		} else {
			flatIDs = append(flatIDs, v)
		}
	}

	// Store the flattened component IDs.
	e.pendingComponents = flatIDs

	if len(flatIDs) > 0 {
		return flatIDs[0], nil
	}
	return e.getIntConstID(0), nil
}

// emitAccessIndex extracts a component from a composite by constant index.
//
// For struct local variable access, emits a GEP instruction to get a pointer
// to the member field. For vector component extraction, uses per-component tracking.
//
func (e *Emitter) emitAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex) (int, error) {
	baseID, err := e.emitExpression(fn, ai.Base)
	if err != nil {
		return 0, err
	}

	// Check if the base is a local variable with a struct type.
	// If so, emit GEP to get a pointer to the struct member.
	if lv, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLocalVariable); ok {
		if int(lv.Variable) < len(fn.LocalVars) {
			localVar := &fn.LocalVars[lv.Variable]
			irType := e.ir.Types[localVar.Type]
			if st, isSt := irType.Inner.(ir.StructType); isSt {
				return e.emitStructGEP(baseID, lv.Variable, st, int(ai.Index))
			}
		}
	}

	// Check if the base is an AccessIndex into a struct (nested struct access).
	// Since struct types are fully flattened, nested access is resolved to a
	// single GEP at the correct flat index from the root struct alloca.
	// resolveNestedStructAccess already includes ai.Index in the flat offset.
	if rootVarIdx, rootAllocaID, flatOffset, ok := e.resolveNestedStructAccess(fn, ai); ok {
		dxilStructTy, hasTy := e.localVarStructTypes[rootVarIdx]
		if hasTy {
			var memberDXILTy *module.Type
			if flatOffset < len(dxilStructTy.StructElems) {
				memberDXILTy = dxilStructTy.StructElems[flatOffset]
			} else {
				memberDXILTy = e.mod.GetFloatType(32)
			}
			resultPtrTy := e.mod.GetPointerType(memberDXILTy)
			zeroID := e.getIntConstID(0)
			indexID := e.getIntConstID(int64(flatOffset))
			return e.addGEPInstr(dxilStructTy, resultPtrTy, rootAllocaID, []int{zeroID, indexID}), nil
		}
	}

	// For vector component access, use per-component tracking.
	return e.getComponentID(ai.Base, int(ai.Index)), nil
}

// emitStructGEP emits a GEP instruction to access a struct member from
// a local variable alloca pointer.
func (e *Emitter) emitStructGEP(baseAllocaID int, varIdx uint32, st ir.StructType, memberIndex int) (int, error) {
	// Use the cached DXIL struct type to ensure type IDs match the alloca.
	dxilStructTy, ok := e.localVarStructTypes[varIdx]
	if !ok {
		return 0, fmt.Errorf("struct GEP: no cached DXIL type for local var %d", varIdx)
	}

	return e.emitStructFieldGEP(baseAllocaID, dxilStructTy, st, memberIndex)
}

// emitStructFieldGEP emits a GEP instruction for a specific struct member.
// Since struct types are flattened (vectors expanded to N scalars), the GEP
// index is the flat field index, not the IR member index.
func (e *Emitter) emitStructFieldGEP(basePtrID int, dxilStructTy *module.Type, st ir.StructType, memberIndex int) (int, error) {
	if memberIndex >= len(st.Members) {
		return 0, fmt.Errorf("struct member index %d out of range (struct has %d members)", memberIndex, len(st.Members))
	}

	// Compute flat index by summing component counts of preceding members.
	flatIndex := flatMemberOffset(e.ir, st, memberIndex)

	// Get the member's DXIL type. For scalar members, this is a single element.
	// For vector members, we need the scalar type (first element).
	var memberDXILTy *module.Type
	if flatIndex < len(dxilStructTy.StructElems) {
		memberDXILTy = dxilStructTy.StructElems[flatIndex]
	} else {
		memberDXILTy = e.mod.GetFloatType(32) // fallback
	}
	resultPtrTy := e.mod.GetPointerType(memberDXILTy)

	// GEP indices: [0, flatIndex] — first 0 is the array element (struct is element 0),
	// second is the flat field index in the DXIL struct.
	zeroID := e.getIntConstID(0)
	indexID := e.getIntConstID(int64(flatIndex))

	return e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID}), nil
}

// emitAccess performs a dynamic-index access on a composite (array/vector).
// For UAV pointer chains, this is handled by resolveUAVPointerChain.
// For vector component access, this extracts the component at a runtime index.
func (e *Emitter) emitAccess(fn *ir.Function, acc ir.ExprAccess) (int, error) {
	// The base must be emitted first.
	_, err := e.emitExpression(fn, acc.Base)
	if err != nil {
		return 0, err
	}

	// Emit the index expression.
	indexID, err := e.emitExpression(fn, acc.Index)
	if err != nil {
		return 0, err
	}

	// For simple cases, return the index ID as-is since the actual load/store
	// will be handled by UAV pointer chain resolution higher up.
	// This is a placeholder: more complex access patterns would need GEP.
	_ = indexID
	return e.exprValues[acc.Base], nil
}

// emitSplat broadcasts a scalar to all components of a vector.
// In DXIL, composites are scalarized — all components share the same value ID.
// We set up pendingComponents so getComponentID resolves correctly.
func (e *Emitter) emitSplat(fn *ir.Function, sp ir.ExprSplat) (int, error) {
	v, err := e.emitExpression(fn, sp.Value)
	if err != nil {
		return 0, err
	}
	// All components point to the same scalar value.
	size := int(sp.Size)
	if size > 1 {
		comps := make([]int, size)
		for i := range comps {
			comps[i] = v
		}
		e.pendingComponents = comps
	}
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
// Dispatches based on argument count and special-case operations.
func (e *Emitter) emitMath(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	// Special vector operations that need scalarized handling.
	switch mathExpr.Fun {
	case ir.MathDot:
		return e.emitMathDot(fn, mathExpr)
	case ir.MathCross:
		return e.emitMathCross(fn, mathExpr)
	case ir.MathLength:
		return e.emitMathLength(fn, mathExpr)
	case ir.MathDistance:
		return e.emitMathDistance(fn, mathExpr)
	case ir.MathNormalize:
		return e.emitMathNormalize(fn, mathExpr)
	}

	// Composite operations without direct dx.op.
	switch mathExpr.Fun {
	case ir.MathPow:
		return e.emitMathPow(fn, mathExpr)
	case ir.MathStep:
		return e.emitMathStep(fn, mathExpr)
	case ir.MathSmoothStep:
		return e.emitMathSmoothStep(fn, mathExpr)
	}

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
	isFloat := scalar.Kind == ir.ScalarFloat
	isSigned := scalar.Kind == ir.ScalarSint

	// Ternary: 3 args (Arg + Arg1 + Arg2).
	if mathExpr.Arg1 != nil && mathExpr.Arg2 != nil {
		arg1, err2 := e.emitExpression(fn, *mathExpr.Arg1)
		if err2 != nil {
			return 0, err2
		}
		arg2, err2 := e.emitExpression(fn, *mathExpr.Arg2)
		if err2 != nil {
			return 0, err2
		}
		return e.emitMathTernary(mathExpr.Fun, ol, isFloat, isSigned, arg, arg1, arg2)
	}

	// Binary: 2 args (Arg + Arg1).
	if mathExpr.Arg1 != nil {
		arg1, err2 := e.emitExpression(fn, *mathExpr.Arg1)
		if err2 != nil {
			return 0, err2
		}
		return e.emitMathBinary(mathExpr.Fun, ol, isFloat, isSigned, arg, arg1)
	}

	// Unary: 1 arg.
	opcode, opName, err := mathToDxOpUnary(mathExpr.Fun)
	if err != nil {
		return 0, err
	}

	dxFn := e.getDxOpUnaryFunc(opName, ol)
	opcodeVal := e.getIntConstID(int64(opcode))

	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg}), nil
}

// emitMathBinary emits a binary math dx.op call (min, max, atan2).
func (e *Emitter) emitMathBinary(mf ir.MathFunction, ol overloadType, isFloat, isSigned bool, arg, arg1 int) (int, error) {
	var opcode DXILOpcode
	var opName string

	switch mf {
	case ir.MathMin:
		if isFloat {
			opcode, opName = OpFMin, "dx.op.fmin"
		} else if isSigned {
			opcode, opName = OpIMin, "dx.op.imin"
		} else {
			opcode, opName = OpUMin, "dx.op.umin"
		}
	case ir.MathMax:
		if isFloat {
			opcode, opName = OpFMax, "dx.op.fmax"
		} else if isSigned {
			opcode, opName = OpIMax, "dx.op.imax"
		} else {
			opcode, opName = OpUMax, "dx.op.umax"
		}
	case ir.MathAtan2:
		// DXIL has no atan2 dx.op. Approximate as atan(y/x).
		// This is a simplification; proper atan2 needs quadrant correction.
		resultTy := e.overloadReturnType(ol)
		div := e.addBinOpInstr(resultTy, BinOpFDiv, arg, arg1)
		dxFn := e.getDxOpUnaryFunc("dx.op.atan", ol)
		opcodeVal := e.getIntConstID(int64(OpAtan))
		return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, div}), nil
	default:
		return 0, fmt.Errorf("unsupported binary math function: %d", mf)
	}

	dxFn := e.getDxOpBinaryFunc(opName, ol)
	opcodeVal := e.getIntConstID(int64(opcode))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg, arg1}), nil
}

// emitMathTernary emits a ternary math dx.op call (clamp, mix, fma).
func (e *Emitter) emitMathTernary(mf ir.MathFunction, ol overloadType, isFloat, isSigned bool, arg, arg1, arg2 int) (int, error) {
	switch mf {
	case ir.MathClamp:
		// clamp(x, lo, hi) = min(max(x, lo), hi)
		return e.emitClamp(ol, isFloat, isSigned, arg, arg1, arg2)
	case ir.MathMix:
		// mix(a, b, t) = a + t*(b-a) = fmad(t, b-a, a)
		if isFloat {
			resultTy := e.overloadReturnType(ol)
			bMinusA := e.addBinOpInstr(resultTy, BinOpFSub, arg1, arg)
			dxFn := e.getDxOpTernaryFunc("dx.op.fmad", ol)
			opcodeVal := e.getIntConstID(int64(OpFMad))
			return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg2, bMinusA, arg}), nil
		}
		return 0, fmt.Errorf("mix not supported for non-float types")
	case ir.MathFma:
		if isFloat {
			dxFn := e.getDxOpTernaryFunc("dx.op.fma", ol)
			opcodeVal := e.getIntConstID(int64(OpFma))
			return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg, arg1, arg2}), nil
		}
		return 0, fmt.Errorf("fma not supported for non-float types")
	default:
		return 0, fmt.Errorf("unsupported ternary math function: %d", mf)
	}
}

// emitClamp emits clamp(x, lo, hi) = min(max(x, lo), hi).
func (e *Emitter) emitClamp(ol overloadType, isFloat, isSigned bool, x, lo, hi int) (int, error) {
	var maxOp, minOp DXILOpcode
	var maxName, minName string
	if isFloat {
		maxOp, maxName = OpFMax, "dx.op.fmax"
		minOp, minName = OpFMin, "dx.op.fmin"
	} else if isSigned {
		maxOp, maxName = OpIMax, "dx.op.imax"
		minOp, minName = OpIMin, "dx.op.imin"
	} else {
		maxOp, maxName = OpUMax, "dx.op.umax"
		minOp, minName = OpUMin, "dx.op.umin"
	}

	maxFn := e.getDxOpBinaryFunc(maxName, ol)
	maxOpcodeVal := e.getIntConstID(int64(maxOp))
	maxResult := e.addCallInstr(maxFn, maxFn.FuncType.RetType, []int{maxOpcodeVal, x, lo})

	minFn := e.getDxOpBinaryFunc(minName, ol)
	minOpcodeVal := e.getIntConstID(int64(minOp))
	return e.addCallInstr(minFn, minFn.FuncType.RetType, []int{minOpcodeVal, maxResult, hi}), nil
}

// emitMathPow emits pow(base, exp) = exp2(log2(base) * exp).
// DXIL has no pow dx.op; decompose into log2 + fmul + exp2.
func (e *Emitter) emitMathPow(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	base, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	exp, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	resultTy := e.overloadReturnType(ol)

	// log2(base)
	logFn := e.getDxOpUnaryFunc("dx.op.log", ol)
	logOp := e.getIntConstID(int64(OpLog))
	logResult := e.addCallInstr(logFn, logFn.FuncType.RetType, []int{logOp, base})

	// log2(base) * exp
	mulResult := e.addBinOpInstr(resultTy, BinOpFMul, logResult, exp)

	// exp2(log2(base) * exp)
	expFn := e.getDxOpUnaryFunc("dx.op.exp", ol)
	expOp := e.getIntConstID(int64(OpExp))
	return e.addCallInstr(expFn, expFn.FuncType.RetType, []int{expOp, mulResult}), nil
}

// emitMathStep emits step(edge, x) = select(x >= edge, 1.0, 0.0).
func (e *Emitter) emitMathStep(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	edge, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	x, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	resultTy := e.overloadReturnType(ol)

	// x >= edge
	cmp := e.addCmpInstr(FCmpOGE, x, edge)

	// select(cmp, 1.0, 0.0)
	oneID := e.getFloatConstID(1.0)
	zeroID := e.getFloatConstID(0.0)

	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrSelect,
		HasValue:   true,
		ResultType: resultTy,
		Operands:   []int{cmp, oneID, zeroID},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID, nil
}

// emitMathSmoothStep emits smoothstep(edge0, edge1, x).
// t = clamp((x - edge0) / (edge1 - edge0), 0.0, 1.0)
// result = t * t * (3.0 - 2.0 * t)
func (e *Emitter) emitMathSmoothStep(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	edge0, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	edge1, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}
	x, err := e.emitExpression(fn, *mathExpr.Arg2)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	resultTy := e.overloadReturnType(ol)

	// x - edge0
	xMinusE0 := e.addBinOpInstr(resultTy, BinOpFSub, x, edge0)
	// edge1 - edge0
	e1MinusE0 := e.addBinOpInstr(resultTy, BinOpFSub, edge1, edge0)
	// (x - edge0) / (edge1 - edge0)
	ratio := e.addBinOpInstr(resultTy, BinOpFDiv, xMinusE0, e1MinusE0)

	// clamp(ratio, 0.0, 1.0)
	zeroID := e.getFloatConstID(0.0)
	oneID := e.getFloatConstID(1.0)
	t, err := e.emitClamp(ol, true, false, ratio, zeroID, oneID)
	if err != nil {
		return 0, err
	}

	// t * t
	tSquared := e.addBinOpInstr(resultTy, BinOpFMul, t, t)
	// 2.0 * t
	twoID := e.getFloatConstID(2.0)
	twoT := e.addBinOpInstr(resultTy, BinOpFMul, twoID, t)
	// 3.0 - 2.0 * t
	threeID := e.getFloatConstID(3.0)
	threeMinusTwoT := e.addBinOpInstr(resultTy, BinOpFSub, threeID, twoT)
	// t * t * (3.0 - 2.0 * t)
	return e.addBinOpInstr(resultTy, BinOpFMul, tSquared, threeMinusTwoT), nil
}

// emitMathDot emits a dot product using dx.op.dot2/dot3/dot4.
// Takes two vectors, extracts components, calls the appropriate dot intrinsic.
func (e *Emitter) emitMathDot(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	_, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	_, err = e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)

	var opcode DXILOpcode
	switch size {
	case 2:
		opcode = OpDot2
	case 3:
		opcode = OpDot3
	case 4:
		opcode = OpDot4
	default:
		return 0, fmt.Errorf("unsupported dot product vector size: %d", size)
	}

	dxFn := e.getDxOpDotFunc(size, ol)
	opcodeVal := e.getIntConstID(int64(opcode))

	// Build operands: opcode, a.x, a.y, ..., b.x, b.y, ...
	operands := make([]int, 1+2*size)
	operands[0] = opcodeVal
	for i := 0; i < size; i++ {
		operands[1+i] = e.getComponentID(mathExpr.Arg, i)
	}
	for i := 0; i < size; i++ {
		operands[1+size+i] = e.getComponentID(*mathExpr.Arg1, i)
	}

	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, operands), nil
}

// emitMathCross emits cross(a, b) = vec3(a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x).
// No dx.op for cross — decompose into 6 fmul + 3 fsub.
func (e *Emitter) emitMathCross(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	_, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	_, err = e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	resultTy := e.overloadReturnType(overloadForScalar(scalar))

	ax := e.getComponentID(mathExpr.Arg, 0)
	ay := e.getComponentID(mathExpr.Arg, 1)
	az := e.getComponentID(mathExpr.Arg, 2)
	bx := e.getComponentID(*mathExpr.Arg1, 0)
	by := e.getComponentID(*mathExpr.Arg1, 1)
	bz := e.getComponentID(*mathExpr.Arg1, 2)

	// x = a.y*b.z - a.z*b.y
	ayBz := e.addBinOpInstr(resultTy, BinOpFMul, ay, bz)
	azBy := e.addBinOpInstr(resultTy, BinOpFMul, az, by)
	cx := e.addBinOpInstr(resultTy, BinOpFSub, ayBz, azBy)

	// y = a.z*b.x - a.x*b.z
	azBx := e.addBinOpInstr(resultTy, BinOpFMul, az, bx)
	axBz := e.addBinOpInstr(resultTy, BinOpFMul, ax, bz)
	cy := e.addBinOpInstr(resultTy, BinOpFSub, azBx, axBz)

	// z = a.x*b.y - a.y*b.x
	axBy := e.addBinOpInstr(resultTy, BinOpFMul, ax, by)
	ayBx := e.addBinOpInstr(resultTy, BinOpFMul, ay, bx)
	cz := e.addBinOpInstr(resultTy, BinOpFSub, axBy, ayBx)

	e.pendingComponents = []int{cx, cy, cz}
	return cx, nil
}

// emitDotForVector emits a dot product of a vector expression with itself.
// Helper used by length and normalize.
func (e *Emitter) emitDotForVector(handle ir.ExpressionHandle, size int, ol overloadType) (int, error) {
	var opcode DXILOpcode
	switch size {
	case 2:
		opcode = OpDot2
	case 3:
		opcode = OpDot3
	case 4:
		opcode = OpDot4
	default:
		return 0, fmt.Errorf("unsupported vector size for dot: %d", size)
	}

	dxFn := e.getDxOpDotFunc(size, ol)
	opcodeVal := e.getIntConstID(int64(opcode))

	operands := make([]int, 1+2*size)
	operands[0] = opcodeVal
	for i := 0; i < size; i++ {
		c := e.getComponentID(handle, i)
		operands[1+i] = c
		operands[1+size+i] = c
	}

	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, operands), nil
}

// emitMathLength emits length(v) = sqrt(dot(v, v)).
func (e *Emitter) emitMathLength(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	_, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)

	// Scalar length = abs(x).
	if size == 1 {
		scalar, ok := scalarOfType(argType)
		if !ok {
			scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		}
		ol := overloadForScalar(scalar)
		arg, _ := e.emitExpression(fn, mathExpr.Arg)
		dxFn := e.getDxOpUnaryFunc("dx.op.fabs", ol)
		opcodeVal := e.getIntConstID(int64(OpFAbs))
		return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg}), nil
	}

	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)

	// dot(v, v)
	dotResult, err := e.emitDotForVector(mathExpr.Arg, size, ol)
	if err != nil {
		return 0, err
	}

	// sqrt(dot(v, v))
	sqrtFn := e.getDxOpUnaryFunc("dx.op.sqrt", ol)
	sqrtOp := e.getIntConstID(int64(OpSqrt))
	return e.addCallInstr(sqrtFn, sqrtFn.FuncType.RetType, []int{sqrtOp, dotResult}), nil
}

// emitMathDistance emits distance(a, b) = length(a - b).
func (e *Emitter) emitMathDistance(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	_, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	_, err = e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	resultTy := e.overloadReturnType(ol)

	// Compute per-component (a - b) and store as a synthetic expression.
	diffHandle := ir.ExpressionHandle(uint32(len(fn.Expressions)) + 1000) //nolint:gosec // synthetic handle, no overflow risk
	diffComps := make([]int, size)
	var firstID int
	for i := 0; i < size; i++ {
		ai := e.getComponentID(mathExpr.Arg, i)
		bi := e.getComponentID(*mathExpr.Arg1, i)
		diffComps[i] = e.addBinOpInstr(resultTy, BinOpFSub, ai, bi)
		if i == 0 {
			firstID = diffComps[i]
		}
	}
	e.exprValues[diffHandle] = firstID
	e.exprComponents[diffHandle] = diffComps

	// dot(diff, diff)
	dotResult, err := e.emitDotForVector(diffHandle, size, ol)
	if err != nil {
		return 0, err
	}

	// sqrt(dot)
	sqrtFn := e.getDxOpUnaryFunc("dx.op.sqrt", ol)
	sqrtOp := e.getIntConstID(int64(OpSqrt))
	return e.addCallInstr(sqrtFn, sqrtFn.FuncType.RetType, []int{sqrtOp, dotResult}), nil
}

// emitMathNormalize emits normalize(v) = v * rsqrt(dot(v, v)).
// Each component is multiplied by rsqrt(dot(v,v)), producing a vector result.
func (e *Emitter) emitMathNormalize(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	_, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	resultTy := e.overloadReturnType(ol)

	// Scalar normalize = sign(x): 1.0 for x>0, -1.0 for x<0, 0 for 0.
	if size == 1 {
		// Simplified: rsqrt(x*x) * x. For proper sign handling this works for nonzero.
		arg, _ := e.emitExpression(fn, mathExpr.Arg)
		mulSelf := e.addBinOpInstr(resultTy, BinOpFMul, arg, arg)
		rsqrtFn := e.getDxOpUnaryFunc("dx.op.rsqrt", ol)
		rsqrtOp := e.getIntConstID(int64(OpRsqrt))
		rsqrt := e.addCallInstr(rsqrtFn, rsqrtFn.FuncType.RetType, []int{rsqrtOp, mulSelf})
		return e.addBinOpInstr(resultTy, BinOpFMul, arg, rsqrt), nil
	}

	// dot(v, v)
	dotResult, err := e.emitDotForVector(mathExpr.Arg, size, ol)
	if err != nil {
		return 0, err
	}

	// rsqrt(dot(v, v))
	rsqrtFn := e.getDxOpUnaryFunc("dx.op.rsqrt", ol)
	rsqrtOp := e.getIntConstID(int64(OpRsqrt))
	invLen := e.addCallInstr(rsqrtFn, rsqrtFn.FuncType.RetType, []int{rsqrtOp, dotResult})

	// v[i] * invLen for each component
	comps := make([]int, size)
	for i := 0; i < size; i++ {
		vi := e.getComponentID(mathExpr.Arg, i)
		comps[i] = e.addBinOpInstr(resultTy, BinOpFMul, vi, invLen)
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// mathToDxOpUnary maps naga MathFunction to dx.op opcode and name for unary functions.
//
//nolint:gocyclo,cyclop // math function mapping requires many cases
func mathToDxOpUnary(mf ir.MathFunction) (DXILOpcode, string, error) {
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
		return 0, "", fmt.Errorf("unsupported unary math function: %d", mf)
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
	// Check if this load is from a CBV (constant buffer) pointer chain.
	// CBV loads use dx.op.cbufferLoadLegacy instead of LLVM load instructions.
	// Reference: Mesa nir_to_dxil.c emit_load_ubo_vec4() line ~3527
	if chain, ok := e.resolveCBVPointerChain(fn, load.Pointer); ok {
		return e.emitCBVLoad(fn, chain)
	}

	// Check if this load is from a UAV (storage buffer) pointer chain.
	// UAV loads use dx.op.bufferLoad instead of LLVM load instructions.
	// Reference: Mesa nir_to_dxil.c emit_bufferload_call() line ~833
	if chain, ok := e.resolveUAVPointerChain(fn, load.Pointer); ok {
		return e.emitUAVLoad(fn, chain)
	}

	// Emit the pointer expression first (may create allocas for local vars).
	ptr, err := e.emitExpression(fn, load.Pointer)
	if err != nil {
		return 0, err
	}

	// Check if this is a vector load from a local variable with per-component allocas.
	// This check is AFTER emitting the pointer so that allocas are created.
	if lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable); ok {
		if compPtrs, hasComps := e.localVarComponentPtrs[lv.Variable]; hasComps {
			return e.emitVectorLoad(fn, compPtrs, load.Pointer)
		}
	}

	// Resolve the loaded type from the pointer's target type.
	loadedTy, err := e.resolveLoadType(fn, load.Pointer)
	if err != nil {
		return 0, fmt.Errorf("load type resolution: %w", err)
	}

	// Emit LLVM load instruction: %val = load TYPE, TYPE* %ptr, align N
	valueID := e.allocValue()
	align := e.alignForType(loadedTy)
	instr := &module.Instruction{
		Kind:       module.InstrLoad,
		HasValue:   true,
		ResultType: loadedTy,
		Operands:   []int{ptr, loadedTy.ID, align, 0}, // ptr, typeID, align, isVolatile
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID, nil
}

// emitVectorLoad loads each component from separate allocas and tracks them
// as per-component values. Returns the first component's value ID.
func (e *Emitter) emitVectorLoad(fn *ir.Function, compPtrs []int, ptrHandle ir.ExpressionHandle) (int, error) {
	// Resolve the scalar element type.
	loadedTy, err := e.resolveLoadType(fn, ptrHandle)
	if err != nil {
		return 0, fmt.Errorf("vector load type resolution: %w", err)
	}
	align := e.alignForType(loadedTy)

	componentIDs := make([]int, len(compPtrs))
	for i, ptr := range compPtrs {
		valueID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrLoad,
			HasValue:   true,
			ResultType: loadedTy,
			Operands:   []int{ptr, loadedTy.ID, align, 0},
			ValueID:    valueID,
		}
		e.currentBB.AddInstruction(instr)
		componentIDs[i] = valueID
	}

	// Store per-component IDs so getComponentID can resolve them.
	e.pendingComponents = componentIDs
	return componentIDs[0], nil
}

// emitLocalVariable emits alloca(s) for the local variable (on first use)
// and returns the pointer value ID of the first component.
//
// For scalar types, a single alloca is emitted.
// For vector types (vec2/vec3/vec4), one alloca per component is emitted
// since DXIL scalarizes all vector operations.
//
// Reference: Mesa nir_to_dxil.c emit_scratch() — dxil_emit_alloca(mod, type, 1, 16)
func (e *Emitter) emitLocalVariable(fn *ir.Function, lv ir.ExprLocalVariable) (int, error) {
	// Return cached alloca pointer if already emitted.
	if ptr, ok := e.localVarPtrs[lv.Variable]; ok {
		return ptr, nil
	}

	if int(lv.Variable) >= len(fn.LocalVars) {
		return 0, fmt.Errorf("local variable index %d out of range (function has %d vars)", lv.Variable, len(fn.LocalVars))
	}

	localVar := &fn.LocalVars[lv.Variable]
	irType := e.ir.Types[localVar.Type]

	// For struct types, allocate the full struct (GEP will be used for member access).
	if _, isSt := irType.Inner.(ir.StructType); isSt {
		return e.emitStructLocalVariable(lv.Variable, localVar, irType)
	}

	// Determine number of components and scalar type.
	numComponents := 1
	if vt, ok := irType.Inner.(ir.VectorType); ok {
		numComponents = int(vt.Size)
	}

	// Resolve the DXIL scalar element type for the allocation.
	elemTy, err := typeToDXIL(e.mod, e.ir, irType.Inner)
	if err != nil {
		return 0, fmt.Errorf("local var %q type: %w", localVar.Name, err)
	}

	// Create pointer type for the result.
	ptrTy := e.mod.GetPointerType(elemTy)

	// Size operand: i32 constant 1 (allocate a single element).
	sizeID := e.getIntConstID(1)

	// Alignment: log2(align) + 1, with bit 6 set for explicit type (LLVM 3.7).
	// Mesa: util_logbase2(align) + 1, then |= (1 << 6).
	// Reference: Mesa dxil_module.c dxil_emit_alloca().
	align := e.alignForType(elemTy) + 1 // log2(bytes) + 1
	alignFlags := align | (1 << 6)

	i32Ty := e.mod.GetIntType(32)
	componentPtrs := make([]int, numComponents)

	for i := 0; i < numComponents; i++ {
		valueID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrAlloca,
			HasValue:   true,
			ResultType: ptrTy,
			Operands:   []int{elemTy.ID, i32Ty.ID, sizeID, alignFlags},
			ValueID:    valueID,
		}
		e.currentBB.AddInstruction(instr)
		componentPtrs[i] = valueID
	}

	e.localVarPtrs[lv.Variable] = componentPtrs[0]
	if numComponents > 1 {
		e.localVarComponentPtrs[lv.Variable] = componentPtrs
	}
	return componentPtrs[0], nil
}

// emitStructLocalVariable emits a single alloca for a struct-typed local variable.
// Member access is via GEP instructions (emitted by emitAccessIndex).
func (e *Emitter) emitStructLocalVariable(varIdx uint32, localVar *ir.LocalVariable, irType ir.Type) (int, error) {
	// Use typeToDXILFull to get the full struct type (not scalarized).
	// Cache it so GEP source element type IDs match.
	structTy, err := typeToDXILFull(e.mod, e.ir, irType.Inner)
	if err != nil {
		return 0, fmt.Errorf("local var %q struct type: %w", localVar.Name, err)
	}
	e.localVarStructTypes[varIdx] = structTy

	ptrTy := e.mod.GetPointerType(structTy)

	sizeID := e.getIntConstID(1)
	i32Ty := e.mod.GetIntType(32)

	// Alignment for struct: use 4-byte default.
	align := 2 + 1 // log2(4) + 1 = 3
	alignFlags := align | (1 << 6)

	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrAlloca,
		HasValue:   true,
		ResultType: ptrTy,
		Operands:   []int{structTy.ID, i32Ty.ID, sizeID, alignFlags},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)

	e.localVarPtrs[varIdx] = valueID
	return valueID, nil
}

// resolveLoadType determines the DXIL type produced by loading from the given
// pointer expression. It examines the pointer expression kind (local variable,
// global variable, or access index) to find the element type.
func (e *Emitter) resolveLoadType(fn *ir.Function, ptrHandle ir.ExpressionHandle) (*module.Type, error) {
	expr := fn.Expressions[ptrHandle]
	switch pk := expr.Kind.(type) {
	case ir.ExprLocalVariable:
		if int(pk.Variable) >= len(fn.LocalVars) {
			return nil, fmt.Errorf("local variable %d out of range", pk.Variable)
		}
		// For struct local vars, return the cached flattened type to match the alloca.
		if cachedTy, ok := e.localVarStructTypes[pk.Variable]; ok {
			return cachedTy, nil
		}
		lv := &fn.LocalVars[pk.Variable]
		irType := e.ir.Types[lv.Type]
		return typeToDXIL(e.mod, e.ir, irType.Inner)

	case ir.ExprGlobalVariable:
		if int(pk.Variable) >= len(e.ir.GlobalVariables) {
			return nil, fmt.Errorf("global variable %d out of range", pk.Variable)
		}
		gv := &e.ir.GlobalVariables[pk.Variable]
		irType := e.ir.Types[gv.Type]
		return typeToDXIL(e.mod, e.ir, irType.Inner)

	case ir.ExprAccessIndex:
		// AccessIndex into a struct produces a GEP pointer to the member.
		// Resolve the member type.
		innerTy := e.resolveAccessIndexResultType(fn, pk)
		if innerTy != nil {
			return typeToDXIL(e.mod, e.ir, innerTy)
		}
		return e.resolveLoadTypeFromExpressionInfo(fn, ptrHandle)

	default:
		return e.resolveLoadTypeFromExpressionInfo(fn, ptrHandle)
	}
}

// resolveNestedStructAccess walks an AccessIndex chain to find the root local
// variable and compute the cumulative flat offset for nested struct access.
// Returns (rootVarIdx, rootAllocaID, flatOffset, ok).
//
func (e *Emitter) resolveNestedStructAccess(fn *ir.Function, ai ir.ExprAccessIndex) (uint32, int, int, bool) {
	baseExpr := fn.Expressions[ai.Base]

	bk, isAI := baseExpr.Kind.(ir.ExprAccessIndex)
	if !isAI {
		return 0, 0, 0, false
	}

	// Try to resolve from a 2-level chain: AccessIndex → AccessIndex → LocalVariable.
	if result, ok := e.resolveNestedFromLocalVar(fn, bk, ai); ok {
		return result.varIdx, result.allocaID, result.flatOffset, true
	}

	// Deeper nesting: recursively resolve.
	if rootVar, rootAlloca, parentOffset, ok := e.resolveNestedStructAccess(fn, bk); ok {
		return rootVar, rootAlloca, parentOffset + int(ai.Index), true
	}

	return 0, 0, 0, false
}

// nestedStructResult holds the result of nested struct access resolution.
type nestedStructResult struct {
	varIdx     uint32
	allocaID   int
	flatOffset int
}

// resolveNestedFromLocalVar resolves a 2-level chain: parentAI → LocalVariable.
func (e *Emitter) resolveNestedFromLocalVar(fn *ir.Function, parentAI ir.ExprAccessIndex, childAI ir.ExprAccessIndex) (nestedStructResult, bool) {
	innerExpr := fn.Expressions[parentAI.Base]
	lv, ok := innerExpr.Kind.(ir.ExprLocalVariable)
	if !ok || int(lv.Variable) >= len(fn.LocalVars) {
		return nestedStructResult{}, false
	}

	localVar := &fn.LocalVars[lv.Variable]
	irType := e.ir.Types[localVar.Type]
	st, isSt := irType.Inner.(ir.StructType)
	if !isSt || int(parentAI.Index) >= len(st.Members) {
		return nestedStructResult{}, false
	}

	parentFlat := flatMemberOffset(e.ir, st, int(parentAI.Index))
	parentMemberTy := e.ir.Types[st.Members[parentAI.Index].Type]
	innerSt, ok := parentMemberTy.Inner.(ir.StructType)
	if !ok {
		return nestedStructResult{}, false
	}

	innerFlat := flatMemberOffset(e.ir, innerSt, int(childAI.Index))
	allocaID := e.localVarPtrs[lv.Variable]
	return nestedStructResult{
		varIdx:     lv.Variable,
		allocaID:   allocaID,
		flatOffset: parentFlat + innerFlat,
	}, true
}

// resolveAccessIndexResultType resolves the IR type that an AccessIndex expression
// points to, by walking the base expression chain to find the struct and extracting
// the member type at the given index.
func (e *Emitter) resolveAccessIndexResultType(fn *ir.Function, ai ir.ExprAccessIndex) ir.TypeInner {
	// Walk to find the base type.
	baseExpr := fn.Expressions[ai.Base]
	var baseInner ir.TypeInner

	switch bk := baseExpr.Kind.(type) {
	case ir.ExprLocalVariable:
		if int(bk.Variable) < len(fn.LocalVars) {
			lv := &fn.LocalVars[bk.Variable]
			baseInner = e.ir.Types[lv.Type].Inner
		}
	case ir.ExprGlobalVariable:
		if int(bk.Variable) < len(e.ir.GlobalVariables) {
			gv := &e.ir.GlobalVariables[bk.Variable]
			baseInner = e.ir.Types[gv.Type].Inner
		}
	case ir.ExprAccessIndex:
		// Nested access — recursively resolve.
		baseInner = e.resolveAccessIndexResultType(fn, bk)
	}

	if baseInner == nil {
		return nil
	}

	// Extract the member type at the given index.
	switch t := baseInner.(type) {
	case ir.StructType:
		if int(ai.Index) < len(t.Members) {
			return e.ir.Types[t.Members[ai.Index].Type].Inner
		}
	case ir.VectorType:
		return t.Scalar
	case ir.ArrayType:
		return e.ir.Types[t.Base].Inner
	}
	return nil
}

// resolveLoadTypeFromExpressionInfo resolves the loaded type by examining
// the ExpressionTypes metadata for the pointer expression.
func (e *Emitter) resolveLoadTypeFromExpressionInfo(fn *ir.Function, ptrHandle ir.ExpressionHandle) (*module.Type, error) {
	if int(ptrHandle) >= len(fn.ExpressionTypes) {
		return e.mod.GetIntType(32), nil
	}
	res := fn.ExpressionTypes[ptrHandle]

	if res.Handle != nil && int(*res.Handle) < len(e.ir.Types) {
		irType := e.ir.Types[*res.Handle]
		if pt, ok := irType.Inner.(ir.PointerType); ok {
			baseType := e.ir.Types[pt.Base]
			return typeToDXIL(e.mod, e.ir, baseType.Inner)
		}
		return typeToDXIL(e.mod, e.ir, irType.Inner)
	}

	if res.Value != nil {
		if pt, ok := res.Value.(ir.PointerType); ok {
			baseType := e.ir.Types[pt.Base]
			return typeToDXIL(e.mod, e.ir, baseType.Inner)
		}
	}

	return e.mod.GetIntType(32), nil
}

// alignForType returns the alignment value for a DXIL type.
// For alloca this is encoded as log2(bytes)+1, for load/store it's log2(bytes).
// Uses the natural alignment of the type.
func (e *Emitter) alignForType(ty *module.Type) int {
	switch ty.Kind {
	case module.TypeFloat:
		switch ty.FloatBits {
		case 16:
			return 1 // log2(2) = 1
		case 64:
			return 3 // log2(8) = 3
		default:
			return 2 // log2(4) = 2 for f32
		}
	case module.TypeInteger:
		switch ty.IntBits {
		case 1, 8:
			return 0 // log2(1) = 0
		case 16:
			return 1 // log2(2) = 1
		case 64:
			return 3 // log2(8) = 3
		default:
			return 2 // log2(4) = 2 for i32
		}
	default:
		return 2 // default 4-byte alignment
	}
}

// emitZeroValue emits a zero-initialized constant.
func (e *Emitter) emitZeroValue(zv ir.ExprZeroValue) (int, error) { //nolint:unparam // error return for interface consistency
	ty := e.ir.Types[zv.Type]
	inner := ty.Inner

	switch t := inner.(type) {
	case ir.ScalarType:
		if t.Kind == ir.ScalarFloat {
			return e.getFloatConstID(0.0), nil
		}
		return e.getIntConstID(0), nil
	case ir.VectorType:
		var zeroID int
		if t.Scalar.Kind == ir.ScalarFloat {
			zeroID = e.getFloatConstID(0.0)
		} else {
			zeroID = e.getIntConstID(0)
		}
		// Set up per-component tracking — all components are the same zero value.
		size := int(t.Size)
		if size > 1 {
			comps := make([]int, size)
			for i := range comps {
				comps[i] = zeroID
			}
			e.pendingComponents = comps
		}
		return zeroID, nil
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

// emitAs emits a type cast expression using LLVM cast instructions.
//
// Reference: Mesa nir_to_dxil.c emit_cast(), SPIR-V backend emitAs().
func (e *Emitter) emitAs(fn *ir.Function, as ir.ExprAs) (int, error) {
	src, err := e.emitExpression(fn, as.Expr)
	if err != nil {
		return 0, err
	}

	// Resolve source type.
	srcInner := e.resolveExprType(fn, as.Expr)
	srcScalar, ok := scalarOfType(srcInner)
	if !ok {
		// Non-scalar/vector/matrix source — pass through.
		return src, nil
	}

	// Determine target scalar.
	var dstScalar ir.ScalarType
	if as.Convert == nil {
		// Bitcast: same width, different interpretation.
		dstScalar = ir.ScalarType{Kind: as.Kind, Width: srcScalar.Width}
	} else {
		dstScalar = ir.ScalarType{Kind: as.Kind, Width: *as.Convert}
	}

	// Identity: same kind and width — no-op.
	if srcScalar.Kind == dstScalar.Kind && srcScalar.Width == dstScalar.Width {
		return src, nil
	}

	// Check for vector type — need per-component cast.
	if vec, ok := srcInner.(ir.VectorType); ok {
		return e.emitVectorCast(fn, as.Expr, vec, srcScalar, dstScalar, as.Convert == nil)
	}

	// Scalar cast.
	return e.emitScalarCast(src, srcScalar, dstScalar, as.Convert == nil)
}

// emitScalarCast emits a single scalar cast instruction.
func (e *Emitter) emitScalarCast(src int, srcScalar, dstScalar ir.ScalarType, isBitcast bool) (int, error) {
	if isBitcast {
		destTy := scalarToDXIL(e.mod, dstScalar)
		return e.addCastInstr(destTy, CastBitcast, src), nil
	}

	// Bool source: bool → numeric requires special handling.
	if srcScalar.Kind == ir.ScalarBool {
		return e.emitBoolToNumericDXIL(src, dstScalar)
	}

	// Bool target: numeric → bool requires comparison with zero.
	if dstScalar.Kind == ir.ScalarBool {
		return e.emitNumericToBoolDXIL(src, srcScalar)
	}

	op, err := selectDXILCastOp(srcScalar, dstScalar)
	if err != nil {
		return 0, err
	}

	destTy := scalarToDXIL(e.mod, dstScalar)
	return e.addCastInstr(destTy, op, src), nil
}

// emitVectorCast emits per-component casts for a vector expression.
func (e *Emitter) emitVectorCast(_ *ir.Function, handle ir.ExpressionHandle, vec ir.VectorType, srcScalar, dstScalar ir.ScalarType, isBitcast bool) (int, error) {
	size := int(vec.Size)
	componentIDs := make([]int, size)

	for i := range size {
		compSrc := e.getComponentID(handle, i)
		compDst, err := e.emitScalarCast(compSrc, srcScalar, dstScalar, isBitcast)
		if err != nil {
			return 0, fmt.Errorf("vector component %d: %w", i, err)
		}
		componentIDs[i] = compDst
	}

	e.pendingComponents = componentIDs
	return componentIDs[0], nil
}

// emitBoolToNumericDXIL converts a bool (i1) to a numeric type.
// bool → int:   ZExt i1 → i32 (produces 0 or 1)
// bool → float: ZExt i1 → i32, then UIToFP i32 → float
func (e *Emitter) emitBoolToNumericDXIL(src int, dstScalar ir.ScalarType) (int, error) {
	switch dstScalar.Kind {
	case ir.ScalarSint, ir.ScalarUint:
		destTy := scalarToDXIL(e.mod, dstScalar)
		return e.addCastInstr(destTy, CastZExt, src), nil
	case ir.ScalarFloat:
		// Two-step: i1 → i32 → float
		i32Ty := e.mod.GetIntType(32)
		intVal := e.addCastInstr(i32Ty, CastZExt, src)
		destTy := scalarToDXIL(e.mod, dstScalar)
		return e.addCastInstr(destTy, CastUIToFP, intVal), nil
	default:
		return 0, fmt.Errorf("bool to %v: unsupported", dstScalar.Kind)
	}
}

// emitNumericToBoolDXIL converts a numeric type to bool (i1) via comparison with zero.
// int → bool:   ICmp NE val, 0
// float → bool: FCmp ONE val, 0.0
func (e *Emitter) emitNumericToBoolDXIL(src int, srcScalar ir.ScalarType) (int, error) {
	switch srcScalar.Kind {
	case ir.ScalarSint, ir.ScalarUint:
		zero := e.getIntConstID(0)
		return e.addCmpInstr(ICmpNE, src, zero), nil
	case ir.ScalarFloat:
		zero := e.getFloatConstID(0.0)
		return e.addCmpInstr(FCmpONE, src, zero), nil
	default:
		return 0, fmt.Errorf("%v to bool: unsupported", srcScalar.Kind)
	}
}

// selectDXILCastOp selects the LLVM cast opcode for a scalar conversion.
// This does NOT handle bool conversions (caller must check).
//
//nolint:gocyclo,cyclop // type cast dispatch requires handling many src/dst kind combinations
func selectDXILCastOp(src, dst ir.ScalarType) (CastOpKind, error) {
	srcBits := uint(src.Width) * 8
	dstBits := uint(dst.Width) * 8

	switch {
	// Float → Int
	case src.Kind == ir.ScalarFloat && dst.Kind == ir.ScalarSint:
		return CastFPToSI, nil
	case src.Kind == ir.ScalarFloat && dst.Kind == ir.ScalarUint:
		return CastFPToUI, nil

	// Int → Float
	case src.Kind == ir.ScalarSint && dst.Kind == ir.ScalarFloat:
		return CastSIToFP, nil
	case src.Kind == ir.ScalarUint && dst.Kind == ir.ScalarFloat:
		return CastUIToFP, nil

	// Float → Float (different width)
	case src.Kind == ir.ScalarFloat && dst.Kind == ir.ScalarFloat:
		if dstBits < srcBits {
			return CastFPTrunc, nil
		}
		return CastFPExt, nil

	// Int → Int (same signedness, different width)
	case src.Kind == ir.ScalarSint && dst.Kind == ir.ScalarSint:
		if dstBits < srcBits {
			return CastTrunc, nil
		}
		return CastSExt, nil
	case src.Kind == ir.ScalarUint && dst.Kind == ir.ScalarUint:
		if dstBits < srcBits {
			return CastTrunc, nil
		}
		return CastZExt, nil

	// Int → Int (different signedness)
	case src.Kind == ir.ScalarSint && dst.Kind == ir.ScalarUint:
		if srcBits != dstBits {
			if dstBits < srcBits {
				return CastTrunc, nil
			}
			return CastZExt, nil
		}
		return CastBitcast, nil
	case src.Kind == ir.ScalarUint && dst.Kind == ir.ScalarSint:
		if srcBits != dstBits {
			if dstBits < srcBits {
				return CastTrunc, nil
			}
			return CastSExt, nil
		}
		return CastBitcast, nil

	default:
		return 0, fmt.Errorf("unsupported cast: %v(%d) → %v(%d)", src.Kind, src.Width, dst.Kind, dst.Width)
	}
}

// addCastInstr adds a cast instruction to the current basic block.
func (e *Emitter) addCastInstr(destTy *module.Type, op CastOpKind, src int) int {
	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrCast,
		HasValue:   true,
		ResultType: destTy,
		Operands:   []int{src, int(op)},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
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

// emitSwizzle reorders or duplicates vector components.
// In DXIL (scalarized), this remaps component IDs from the source vector.
func (e *Emitter) emitSwizzle(fn *ir.Function, sw ir.ExprSwizzle) (int, error) {
	// Ensure the source vector is emitted.
	if _, err := e.emitExpression(fn, sw.Vector); err != nil {
		return 0, err
	}

	size := int(sw.Size)
	comps := make([]int, size)
	for i := 0; i < size; i++ {
		srcComp := int(sw.Pattern[i]) // 0=X, 1=Y, 2=Z, 3=W
		comps[i] = e.getComponentID(sw.Vector, srcComp)
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitDerivative emits a fragment shader derivative as a dx.op call.
//
// Maps naga DerivativeAxis + DerivativeControl to dx.op opcodes:
//
//	X + Coarse → dx.op.derivCoarseX (83)
//	Y + Coarse → dx.op.derivCoarseY (84)
//	X + Fine   → dx.op.derivFineX   (85)
//	Y + Fine   → dx.op.derivFineY   (86)
//	Width      → derivCoarseX + derivCoarseY (abs sum, fwidth)
func (e *Emitter) emitDerivative(fn *ir.Function, deriv ir.ExprDerivative) (int, error) {
	arg, err := e.emitExpression(fn, deriv.Expr)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, deriv.Expr)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)

	// fwidth = abs(dFdx) + abs(dFdy) — needs decomposition.
	if deriv.Axis == ir.DerivativeWidth {
		return e.emitDerivativeWidth(arg, ol)
	}

	opcode, opName := derivativeOp(deriv.Axis, deriv.Control)
	dxFn := e.getDxOpUnaryFunc(opName, ol)
	opcodeVal := e.getIntConstID(int64(opcode))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg}), nil
}

// emitDerivativeWidth emits fwidth(v) = abs(dFdx(v)) + abs(dFdy(v)).
func (e *Emitter) emitDerivativeWidth(arg int, ol overloadType) (int, error) {
	resultTy := e.overloadReturnType(ol)

	// dFdx
	dxFnX := e.getDxOpUnaryFunc("dx.op.derivCoarseX", ol)
	opcodeX := e.getIntConstID(int64(OpDerivCoarseX))
	dfdx := e.addCallInstr(dxFnX, dxFnX.FuncType.RetType, []int{opcodeX, arg})

	// dFdy
	dxFnY := e.getDxOpUnaryFunc("dx.op.derivCoarseY", ol)
	opcodeY := e.getIntConstID(int64(OpDerivCoarseY))
	dfdy := e.addCallInstr(dxFnY, dxFnY.FuncType.RetType, []int{opcodeY, arg})

	// abs(dFdx)
	absFn := e.getDxOpUnaryFunc("dx.op.fabs", ol)
	absOp := e.getIntConstID(int64(OpFAbs))
	absDx := e.addCallInstr(absFn, absFn.FuncType.RetType, []int{absOp, dfdx})

	// abs(dFdy)
	absDy := e.addCallInstr(absFn, absFn.FuncType.RetType, []int{absOp, dfdy})

	// abs(dFdx) + abs(dFdy)
	return e.addBinOpInstr(resultTy, BinOpFAdd, absDx, absDy), nil
}

// derivativeOp returns the dx.op opcode and name for a derivative axis + control.
func derivativeOp(axis ir.DerivativeAxis, control ir.DerivativeControl) (DXILOpcode, string) {
	switch {
	case axis == ir.DerivativeX && control == ir.DerivativeFine:
		return OpDerivFineX, "dx.op.derivFineX"
	case axis == ir.DerivativeY && control == ir.DerivativeFine:
		return OpDerivFineY, "dx.op.derivFineY"
	case axis == ir.DerivativeX: // Coarse or None
		return OpDerivCoarseX, "dx.op.derivCoarseX"
	case axis == ir.DerivativeY: // Coarse or None
		return OpDerivCoarseY, "dx.op.derivCoarseY"
	default:
		return OpDerivCoarseX, "dx.op.derivCoarseX"
	}
}

// emitRelational emits a relational test function as a dx.op call.
//
// Maps naga RelationalFunction to dx.op:
//
//	IsNan → dx.op.isNaN (8)
//	IsInf → dx.op.isInf (9)
//
// All/Any are boolean vector reductions (not dx.op intrinsics).
func (e *Emitter) emitRelational(fn *ir.Function, rel ir.ExprRelational) (int, error) {
	arg, err := e.emitExpression(fn, rel.Argument)
	if err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, rel.Argument)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}

	switch rel.Fun {
	case ir.RelationalIsNan:
		ol := overloadForScalar(scalar)
		dxFn := e.getDxOpUnaryFunc("dx.op.isNaN", ol)
		opcodeVal := e.getIntConstID(int64(OpIsNaN))
		return e.addCallInstr(dxFn, e.mod.GetIntType(1), []int{opcodeVal, arg}), nil

	case ir.RelationalIsInf:
		ol := overloadForScalar(scalar)
		dxFn := e.getDxOpUnaryFunc("dx.op.isInf", ol)
		opcodeVal := e.getIntConstID(int64(OpIsInf))
		return e.addCallInstr(dxFn, e.mod.GetIntType(1), []int{opcodeVal, arg}), nil

	case ir.RelationalAll:
		// All components are true: for scalar bool, just return the value.
		return arg, nil

	case ir.RelationalAny:
		// Any component is true: for scalar bool, just return the value.
		return arg, nil

	default:
		return 0, fmt.Errorf("unsupported relational function: %d", rel.Fun)
	}
}
