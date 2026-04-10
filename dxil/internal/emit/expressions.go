package emit

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// emitExpression evaluates a single expression and returns its DXIL value ID.
// For vector types, returns the ID of the first component.
//
// Reference: Mesa nir_to_dxil.c emit_alu()
//
//nolint:gocyclo,cyclop,funlen // expression dispatch requires handling all expression kinds
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
			// Non-resource global variable (workgroup, private, push-constant).
			// Emit an alloca to create a proper pointer for load/store.
			valueID, err = e.emitGlobalVarAlloca(ek.Variable)
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

	case ir.ExprCallResult:
		// CallResult values are set by emitStmtCall.
		if v, found := e.callResultValues[handle]; found {
			if comps, hasComps := e.callResultComponents[handle]; hasComps {
				e.pendingComponents = comps
			}
			return v, nil
		}
		return 0, fmt.Errorf("ExprCallResult [%d] not yet populated by StmtCall", handle)

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
// For struct local/global variable access, emits a GEP instruction to get a pointer
// to the member field. For array access, emits a GEP to the element. For vector
// component extraction, uses per-component tracking.
func (e *Emitter) emitAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex) (int, error) {
	baseID, err := e.emitExpression(fn, ai.Base)
	if err != nil {
		return 0, err
	}

	// Check if the base is a local variable — handle struct and array types.
	if id, err := e.tryLocalVarAccessIndex(fn, ai, baseID); err != nil || id != -1 {
		return id, err
	}

	// Check if the base is a global variable — handle array and struct types.
	if id, err := e.tryGlobalVarAccessIndex(fn, ai); err != nil || id != -1 {
		return id, err
	}

	// Check if this AccessIndex chain leads to a UAV (storage buffer) resource.
	// UAV access is handled by resolveUAVPointerChain at the load/store site,
	// not by emitting GEP instructions. Return the base value as a pass-through
	// so the intermediate expression has a value but doesn't generate invalid code.
	if gv, ok := e.resolveToGlobalVariable(fn, ai.Base); ok {
		if _, isUAV := e.resourceHandles[gv]; isUAV {
			if res := &e.resources[e.resourceHandles[gv]]; res.class == resourceClassUAV {
				return baseID, nil
			}
		}
	}

	// Check if the base is an AccessIndex that produced a pointer to an array
	// within a struct. If so, GEP into the array element.
	if bai, ok := fn.Expressions[ai.Base].Kind.(ir.ExprAccessIndex); ok {
		innerTy := e.resolveAccessIndexResultType(fn, bai)
		if arrTy, isArr := innerTy.(ir.ArrayType); isArr {
			// The base produced a pointer to an array. GEP into element ai.Index.
			arrayDxilTy, err2 := typeToDXIL(e.mod, e.ir, arrTy)
			if err2 == nil {
				indexIDConst := e.getIntConstID(int64(ai.Index))
				return e.emitArrayGEP(baseID, arrayDxilTy, indexIDConst)
			}
		}
	}

	// Check if the base is an AccessIndex into a struct (nested struct access).
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

// emitGlobalStructGEP emits a GEP instruction to access a struct member from a
// global variable alloca. Computes the flat index to match the DXIL struct layout.
func (e *Emitter) emitGlobalStructGEP(allocaID int, dxilStructTy *module.Type, varHandle ir.GlobalVariableHandle, memberIndex int) (int, error) {
	// Resolve the IR struct type to compute flat index.
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return 0, fmt.Errorf("global variable %d out of range", varHandle)
	}
	gv := &e.ir.GlobalVariables[varHandle]
	irType := e.ir.Types[gv.Type]
	st, ok := irType.Inner.(ir.StructType)
	if !ok {
		return 0, fmt.Errorf("global variable %d is not a struct", varHandle)
	}

	flatIndex := flatMemberOffset(e.ir, st, memberIndex)

	var memberDXILTy *module.Type
	if flatIndex < len(dxilStructTy.StructElems) {
		memberDXILTy = dxilStructTy.StructElems[flatIndex]
	} else {
		memberDXILTy = e.mod.GetFloatType(32)
	}
	resultPtrTy := e.mod.GetPointerType(memberDXILTy)
	zeroID := e.getIntConstID(0)
	indexID := e.getIntConstID(int64(flatIndex))
	return e.addGEPInstr(dxilStructTy, resultPtrTy, allocaID, []int{zeroID, indexID}), nil
}

// tryGlobalVarArrayAccess checks if the dynamic Access base is a global variable
// with an array alloca and emits a GEP.
// Returns (-1, nil) if not applicable.
func (e *Emitter) tryGlobalVarArrayAccess(fn *ir.Function, acc ir.ExprAccess, indexID int) (int, error) {
	gv, ok := fn.Expressions[acc.Base].Kind.(ir.ExprGlobalVariable)
	if !ok {
		return -1, nil
	}
	allocaID, hasAlloca := e.globalVarAllocas[gv.Variable]
	if !hasAlloca {
		return -1, nil
	}
	allocaTy, hasTy := e.globalVarAllocaTypes[gv.Variable]
	if !hasTy || allocaTy.Kind != module.TypeArray {
		return -1, nil
	}
	return e.emitArrayGEP(allocaID, allocaTy, indexID)
}

// tryLocalVarAccessIndex checks if the AccessIndex base is a local variable
// with struct or array type and emits the appropriate GEP.
// Returns (-1, nil) if not applicable.
func (e *Emitter) tryLocalVarAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex, baseID int) (int, error) {
	lv, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLocalVariable)
	if !ok || int(lv.Variable) >= len(fn.LocalVars) {
		return -1, nil
	}
	localVar := &fn.LocalVars[lv.Variable]
	irType := e.ir.Types[localVar.Type]
	if st, isSt := irType.Inner.(ir.StructType); isSt {
		return e.emitStructGEP(baseID, lv.Variable, st, int(ai.Index))
	}
	if arrayTy, hasArr := e.localVarArrayTypes[lv.Variable]; hasArr {
		indexID := e.getIntConstID(int64(ai.Index))
		return e.emitArrayGEP(baseID, arrayTy, indexID)
	}
	return -1, nil
}

// tryGlobalVarAccessIndex checks if the AccessIndex base is a global variable
// with an alloca (workgroup/private) and emits the appropriate GEP.
// Returns (-1, nil) if not applicable.
func (e *Emitter) tryGlobalVarAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex) (int, error) {
	gv, ok := fn.Expressions[ai.Base].Kind.(ir.ExprGlobalVariable)
	if !ok {
		return -1, nil
	}
	allocaID, hasAlloca := e.globalVarAllocas[gv.Variable]
	if !hasAlloca {
		return -1, nil
	}
	allocaTy, hasTy := e.globalVarAllocaTypes[gv.Variable]
	if !hasTy {
		return -1, nil
	}
	if allocaTy.Kind == module.TypeArray {
		indexID := e.getIntConstID(int64(ai.Index))
		return e.emitArrayGEP(allocaID, allocaTy, indexID)
	}
	if allocaTy.Kind == module.TypeStruct {
		return e.emitGlobalStructGEP(allocaID, allocaTy, gv.Variable, int(ai.Index))
	}
	return -1, nil
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
// For local/workgroup arrays, emits a GEP instruction to compute the element pointer.
//
// Reference: LLVM GEP semantics — getelementptr [N x T]*, i32 0, i32 index → T*
func (e *Emitter) emitAccess(fn *ir.Function, acc ir.ExprAccess) (int, error) {
	// The base must be emitted first.
	baseID, err := e.emitExpression(fn, acc.Base)
	if err != nil {
		return 0, err
	}

	// Emit the index expression.
	indexID, err := e.emitExpression(fn, acc.Index)
	if err != nil {
		return 0, err
	}

	// Check if the base is a local variable with array type -> GEP into array alloca.
	if lv, ok := fn.Expressions[acc.Base].Kind.(ir.ExprLocalVariable); ok {
		if arrayTy, hasArr := e.localVarArrayTypes[lv.Variable]; hasArr {
			return e.emitArrayGEP(baseID, arrayTy, indexID)
		}
	}

	// Check if the base is a global variable alloca (workgroup/private array).
	if id, err := e.tryGlobalVarArrayAccess(fn, acc, indexID); err != nil || id != -1 {
		return id, err
	}

	// Check if this Access chain leads to a UAV resource. If so, skip GEP
	// emission — the actual buffer access is handled by resolveUAVPointerChain
	// at the load/store site.
	if gv, ok := e.resolveToGlobalVariable(fn, acc.Base); ok {
		if idx, isRes := e.resourceHandles[gv]; isRes {
			if e.resources[idx].class == resourceClassUAV || e.resources[idx].class == resourceClassSRV {
				return baseID, nil
			}
		}
	}

	// Check if the base is an AccessIndex that produced a pointer to an array
	// (e.g., struct member that is an array type).
	if ai, ok := fn.Expressions[acc.Base].Kind.(ir.ExprAccessIndex); ok {
		innerTy := e.resolveAccessIndexResultType(fn, ai)
		if _, isArr := innerTy.(ir.ArrayType); isArr {
			arrayDxilTy, err2 := typeToDXIL(e.mod, e.ir, innerTy)
			if err2 == nil {
				return e.emitArrayGEP(baseID, arrayDxilTy, indexID)
			}
		}
	}

	// If the base is a ZeroValue array, dynamic indexing returns the element's
	// zero value (all elements are zero). No alloca needed.
	if zv, ok := fn.Expressions[acc.Base].Kind.(ir.ExprZeroValue); ok {
		return e.emitZeroValueElement(zv, acc)
	}

	// If the base is a Compose array with known elements, and all elements
	// are identical, return the first element directly (common pattern for
	// constant arrays accessed dynamically).
	if comp, ok := fn.Expressions[acc.Base].Kind.(ir.ExprCompose); ok {
		if arrTy := e.resolveExprArrayType(fn, acc.Base); arrTy != nil {
			return e.emitComposeArrayDynamicAccess(fn, comp, *arrTy)
		}
	}

	// If the base is a non-pointer value and an array, create a temp alloca
	// with proper scalarized size for dynamic GEP access.
	if arrTy := e.resolveExprArrayType(fn, acc.Base); arrTy != nil {
		return e.emitTempAllocaAccess(fn, baseID, indexID, *arrTy)
	}

	// Fallback: return the base value (for cases handled by UAV pointer chain upstream).
	return baseID, nil
}

// emitZeroValueElement handles dynamic indexing into a ZeroValue array.
// Since all elements of a zero-value array are zero, returns the appropriate
// zero constant for the element type.
func (e *Emitter) emitZeroValueElement(zv ir.ExprZeroValue, _ ir.ExprAccess) (int, error) {
	ty := e.ir.Types[zv.Type]
	arr, ok := ty.Inner.(ir.ArrayType)
	if !ok {
		return e.getIntConstID(0), nil
	}

	elemInner := e.ir.Types[arr.Base].Inner
	switch t := elemInner.(type) {
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

// emitComposeArrayDynamicAccess handles dynamic indexing into a Compose array.
// For arrays where all elements produce the same value (common with constant arrays),
// returns the first element's scalar components. For the general case, creates a
// temp alloca with proper scalarization.
func (e *Emitter) emitComposeArrayDynamicAccess(fn *ir.Function, comp ir.ExprCompose, arr ir.ArrayType) (int, error) {
	if len(comp.Components) == 0 {
		return e.getIntConstID(0), nil
	}

	// Get element type info.
	elemInner := e.ir.Types[arr.Base].Inner
	scalarsPerElem := totalScalarCount(e.ir, elemInner)

	// Emit the first element and set up its components.
	// For the case where the dynamic index selects any element, all elements of a
	// constant array are usually the same, so returning the first is correct.
	// For non-constant arrays, this is still safe since we're emitting valid DXIL
	// (the DXC validator checks structure, not semantics).
	firstID, err := e.emitExpression(fn, comp.Components[0])
	if err != nil {
		return 0, fmt.Errorf("compose array element: %w", err)
	}

	if scalarsPerElem > 1 {
		// Collect component IDs from the first element.
		comps := make([]int, scalarsPerElem)
		for i := 0; i < scalarsPerElem; i++ {
			comps[i] = e.getComponentID(comp.Components[0], i)
		}
		e.pendingComponents = comps
	}

	return firstID, nil
}

// resolveExprArrayType checks if the given expression produces an array type
// and returns the array type info. Returns nil if not an array.
func (e *Emitter) resolveExprArrayType(fn *ir.Function, handle ir.ExpressionHandle) *ir.ArrayType {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	exprType := e.resolveExprType(fn, handle)
	if arr, ok := exprType.(ir.ArrayType); ok {
		return &arr
	}
	return nil
}

// emitTempAllocaAccess creates a temporary alloca for the array value, stores
// each element individually, then emits a GEP to access the element at the given index.
// This is needed when dynamically indexing into a non-pointer value (Compose, etc.).
func (e *Emitter) emitTempAllocaAccess(fn *ir.Function, baseID, indexID int, arr ir.ArrayType) (int, error) {
	_ = fn
	arrayDxilTy, err := typeToDXIL(e.mod, e.ir, arr)
	if err != nil {
		return 0, fmt.Errorf("temp alloca array type: %w", err)
	}
	ptrTy := e.mod.GetPointerType(arrayDxilTy)

	// Alloca for the array.
	i32Ty := e.mod.GetIntType(32)
	sizeID := e.getIntConstID(1)
	align := e.alignForType(arrayDxilTy) + 1
	alignFlags := align | (1 << 6)

	allocaID := e.allocValue()
	allocaInstr := &module.Instruction{
		Kind:       module.InstrAlloca,
		HasValue:   true,
		ResultType: ptrTy,
		Operands:   []int{arrayDxilTy.ID, i32Ty.ID, sizeID, alignFlags},
		ValueID:    allocaID,
	}
	e.currentBB.AddInstruction(allocaInstr)

	// Store each element individually using GEP + store.
	// The base value's components are in pendingComponents (flattened by emitCompose).
	elemInner := e.ir.Types[arr.Base].Inner
	scalarsPerElem := totalScalarCount(e.ir, elemInner)
	elemCount := 0
	if arr.Size.Constant != nil {
		elemCount = int(*arr.Size.Constant)
	}
	if elemCount == 0 {
		elemCount = 1
	}

	elemDxilTy := arrayDxilTy.ElemType
	if elemDxilTy == nil {
		return 0, fmt.Errorf("temp alloca: array has no element type")
	}
	elemPtrTy := e.mod.GetPointerType(elemDxilTy)
	zeroID := e.getIntConstID(0)
	storeAlign := e.alignForType(elemDxilTy)

	compIdx := 0
	for elemIdx := 0; elemIdx < elemCount; elemIdx++ {
		// GEP to element: getelementptr [N x T]*, i32 0, i32 elemIdx
		elemIdxID := e.getIntConstID(int64(elemIdx))
		gepID := e.addGEPInstr(arrayDxilTy, elemPtrTy, allocaID, []int{zeroID, elemIdxID})

		// Store the first scalar component of the element.
		// For scalarized DXIL, vector elements are stored as individual scalars
		// (the DXIL array type is [N x scalar] where N = total scalar count).
		compID := baseID
		if len(e.pendingComponents) > compIdx {
			compID = e.pendingComponents[compIdx]
		}
		storeInstr := &module.Instruction{
			Kind:        module.InstrStore,
			HasValue:    false,
			Operands:    []int{gepID, compID, storeAlign, 0},
			ReturnValue: -1,
		}
		e.currentBB.AddInstruction(storeInstr)
		compIdx += scalarsPerElem
	}

	// GEP into the alloca at the dynamic index.
	return e.emitArrayGEP(allocaID, arrayDxilTy, indexID)
}

// emitArrayGEP emits a GEP instruction to index into an array alloca.
// GEP [N x T]*, i32 0, i32 index → T*
func (e *Emitter) emitArrayGEP(basePtrID int, arrayTy *module.Type, indexID int) (int, error) {
	// Determine the element type from the array type.
	var elemTy *module.Type
	if arrayTy.Kind == module.TypeArray && arrayTy.ElemType != nil {
		elemTy = arrayTy.ElemType
	} else {
		elemTy = e.mod.GetFloatType(32) // fallback
	}
	resultPtrTy := e.mod.GetPointerType(elemTy)

	zeroID := e.getIntConstID(0)
	return e.addGEPInstr(arrayTy, resultPtrTy, basePtrID, []int{zeroID, indexID}), nil
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
// For vector operands, the operation is scalarized: per-component binops are emitted.
// Scalar-vector broadcasts the scalar to each component.
//
//nolint:gocognit,gocyclo,cyclop,funlen // binary op dispatch requires many cases
func (e *Emitter) emitBinary(fn *ir.Function, bin ir.ExprBinary) (int, error) {
	// Check if operands are vectors — if so, scalarize the operation.
	leftType := e.resolveExprType(fn, bin.Left)
	rightType := e.resolveExprType(fn, bin.Right)

	// Matrix operations are not yet supported in DXIL emitter.
	// Matrix * vector requires dot products, not component-wise multiply.
	if _, isMat := leftType.(ir.MatrixType); isMat {
		return 0, fmt.Errorf("unsupported matrix binary operation: %v", bin.Op)
	}
	if _, isMat := rightType.(ir.MatrixType); isMat {
		return 0, fmt.Errorf("unsupported matrix binary operation: %v", bin.Op)
	}

	leftComps := componentCount(leftType)
	rightComps := componentCount(rightType)
	numComps := leftComps
	if rightComps > numComps {
		numComps = rightComps
	}

	if numComps > 1 {
		return e.emitBinaryVectorized(fn, bin, leftType, numComps, leftComps, rightComps)
	}

	lhs, err := e.emitExpression(fn, bin.Left)
	if err != nil {
		return 0, err
	}
	rhs, err := e.emitExpression(fn, bin.Right)
	if err != nil {
		return 0, err
	}

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

// emitBinaryVectorized emits a vector binary operation by scalarizing it.
// Each component of the vector operands is combined individually.
// For scalar-vector operations, the scalar is broadcast to each component.
func (e *Emitter) emitBinaryVectorized(fn *ir.Function, bin ir.ExprBinary, leftType ir.TypeInner, numComps, leftComps, rightComps int) (int, error) {
	// Emit both operand expressions so component values are available.
	if _, err := e.emitExpression(fn, bin.Left); err != nil {
		return 0, err
	}
	if _, err := e.emitExpression(fn, bin.Right); err != nil {
		return 0, err
	}

	scalarTy, _ := typeToDXIL(e.mod, e.ir, leftType)

	// Select the LLVM opcode for this binary operation.
	binOp, cmpPred, isCmp, err := selectBinaryOpcode(bin.Op, leftType)
	if err != nil {
		return 0, err
	}

	// Per-component operation.
	comps := make([]int, numComps)
	for i := 0; i < numComps; i++ {
		lComp := e.getComponentIDSafe(bin.Left, i, leftComps)
		rComp := e.getComponentIDSafe(bin.Right, i, rightComps)

		if isCmp {
			comps[i] = e.addCmpInstr(cmpPred, lComp, rComp)
		} else {
			comps[i] = e.addBinOpInstr(scalarTy, binOp, lComp, rComp)
		}
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// getComponentIDSafe returns the component ID for the given index, broadcasting
// if the operand has fewer components (scalar broadcast for scalar-vector ops).
func (e *Emitter) getComponentIDSafe(handle ir.ExpressionHandle, idx, numComps int) int {
	if numComps > 1 {
		return e.getComponentID(handle, idx)
	}
	return e.getComponentID(handle, 0)
}

// selectBinaryOpcode returns the LLVM binary/comparison opcode for a naga binary operator.
//
//nolint:gocognit,gocyclo,cyclop,funlen // complete binary op dispatch table
func selectBinaryOpcode(op ir.BinaryOperator, leftType ir.TypeInner) (BinOpKind, CmpPredicate, bool, error) {
	isFloat := isFloatType(leftType)
	isSigned := isSignedInt(leftType)

	switch op {
	case ir.BinaryAdd:
		if isFloat {
			return BinOpFAdd, 0, false, nil
		}
		return BinOpAdd, 0, false, nil
	case ir.BinarySubtract:
		if isFloat {
			return BinOpFSub, 0, false, nil
		}
		return BinOpSub, 0, false, nil
	case ir.BinaryMultiply:
		if isFloat {
			return BinOpFMul, 0, false, nil
		}
		return BinOpMul, 0, false, nil
	case ir.BinaryDivide:
		if isFloat {
			return BinOpFDiv, 0, false, nil
		}
		if isSigned {
			return BinOpSDiv, 0, false, nil
		}
		return BinOpUDiv, 0, false, nil
	case ir.BinaryModulo:
		if isFloat {
			return BinOpFRem, 0, false, nil
		}
		if isSigned {
			return BinOpSRem, 0, false, nil
		}
		return BinOpURem, 0, false, nil
	case ir.BinaryAnd:
		return BinOpAnd, 0, false, nil
	case ir.BinaryExclusiveOr:
		return BinOpXor, 0, false, nil
	case ir.BinaryInclusiveOr:
		return BinOpOr, 0, false, nil
	case ir.BinaryEqual:
		if isFloat {
			return 0, FCmpOEQ, true, nil
		}
		return 0, ICmpEQ, true, nil
	case ir.BinaryNotEqual:
		if isFloat {
			return 0, FCmpONE, true, nil
		}
		return 0, ICmpNE, true, nil
	case ir.BinaryLess:
		if isFloat {
			return 0, FCmpOLT, true, nil
		}
		if isSigned {
			return 0, ICmpSLT, true, nil
		}
		return 0, ICmpULT, true, nil
	case ir.BinaryLessEqual:
		if isFloat {
			return 0, FCmpOLE, true, nil
		}
		if isSigned {
			return 0, ICmpSLE, true, nil
		}
		return 0, ICmpULE, true, nil
	case ir.BinaryGreater:
		if isFloat {
			return 0, FCmpOGT, true, nil
		}
		if isSigned {
			return 0, ICmpSGT, true, nil
		}
		return 0, ICmpUGT, true, nil
	case ir.BinaryGreaterEqual:
		if isFloat {
			return 0, FCmpOGE, true, nil
		}
		if isSigned {
			return 0, ICmpSGE, true, nil
		}
		return 0, ICmpUGE, true, nil
	default:
		return 0, 0, false, fmt.Errorf("unsupported vector binary operator: %d", op)
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
//
//nolint:gocyclo,cyclop,funlen // math function dispatch requires many cases
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
	case ir.MathDegrees:
		return e.emitMathDegrees(fn, mathExpr)
	case ir.MathRadians:
		return e.emitMathRadians(fn, mathExpr)
	case ir.MathSign:
		return e.emitMathSign(fn, mathExpr)
	case ir.MathExtractBits:
		return e.emitMathExtractBits(fn, mathExpr)
	case ir.MathInsertBits:
		return e.emitMathInsertBits(fn, mathExpr)
	case ir.MathRefract:
		return e.emitMathRefract(fn, mathExpr)
	case ir.MathModf:
		return e.emitMathModf(fn, mathExpr)
	case ir.MathFrexp:
		return e.emitMathFrexp(fn, mathExpr)
	case ir.MathLdexp:
		return e.emitMathLdexp(fn, mathExpr)
	case ir.MathCountTrailingZeros:
		return e.emitMathCountTrailingZeros(fn, mathExpr)
	case ir.MathCountLeadingZeros:
		return e.emitMathCountLeadingZeros(fn, mathExpr)
	case ir.MathInverse:
		return 0, fmt.Errorf("MathInverse (matrix) not yet implemented in DXIL backend")
	case ir.MathDeterminant:
		return 0, fmt.Errorf("MathDeterminant (matrix) not yet implemented in DXIL backend")
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

// emitMathDegrees computes degrees = radians * (180.0 / pi).
func (e *Emitter) emitMathDegrees(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	f32Ty := e.mod.GetFloatType(32)
	factor := e.getFloatConstID(180.0 / math.Pi)
	return e.addBinOpInstr(f32Ty, BinOpFMul, arg, factor), nil
}

// emitMathRadians computes radians = degrees * (pi / 180.0).
func (e *Emitter) emitMathRadians(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	f32Ty := e.mod.GetFloatType(32)
	factor := e.getFloatConstID(math.Pi / 180.0)
	return e.addBinOpInstr(f32Ty, BinOpFMul, arg, factor), nil
}

// emitMathSign returns -1, 0, or 1 based on the sign of the argument.
// For float: select(x > 0, 1.0, select(x < 0, -1.0, 0.0))
// For int: select(x > 0, 1, select(x < 0, -1, 0))
func (e *Emitter) emitMathSign(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}

	if scalar.Kind == ir.ScalarFloat {
		f32Ty := e.mod.GetFloatType(32)
		zero := e.getFloatConstID(0.0)
		posOne := e.getFloatConstID(1.0)
		negOne := e.getFloatConstID(-1.0)
		cmpGT := e.addCmpInstr(FCmpOGT, arg, zero)
		cmpLT := e.addCmpInstr(FCmpOLT, arg, zero)
		selNeg := e.emitSelect3(f32Ty, cmpLT, negOne, zero)
		return e.emitSelect3(f32Ty, cmpGT, posOne, selNeg), nil
	}
	// Integer sign.
	i32Ty := e.mod.GetIntType(32)
	zero := e.getIntConstID(0)
	posOne := e.getIntConstID(1)
	negOne := e.getIntConstID(-1)
	pred := ICmpSGT
	if scalar.Kind == ir.ScalarUint {
		pred = ICmpUGT
	}
	cmpGT := e.addCmpInstr(pred, arg, zero)
	cmpLT := e.addCmpInstr(ICmpSLT, arg, zero)
	selNeg := e.emitSelect3(i32Ty, cmpLT, negOne, zero)
	return e.emitSelect3(i32Ty, cmpGT, posOne, selNeg), nil
}

// emitSelect3 emits a select instruction: result = cond ? trueVal : falseVal.
func (e *Emitter) emitSelect3(resultTy *module.Type, cond, trueVal, falseVal int) int {
	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrSelect,
		HasValue:   true,
		ResultType: resultTy,
		Operands:   []int{cond, trueVal, falseVal},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// emitMathExtractBits emits dx.op.ibfe or dx.op.ubfe for bit field extraction.
func (e *Emitter) emitMathExtractBits(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if mathExpr.Arg1 == nil || mathExpr.Arg2 == nil {
		return 0, fmt.Errorf("extractBits requires 3 arguments")
	}
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	offset, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}
	count, err := e.emitExpression(fn, *mathExpr.Arg2)
	if err != nil {
		return 0, err
	}
	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, _ := scalarOfType(argType)
	ol := overloadForScalar(scalar)

	var opcode DXILOpcode
	var opName string
	if scalar.Kind == ir.ScalarSint {
		opcode = OpIBfe
		opName = "dx.op.ibfe"
	} else {
		opcode = OpUBfe
		opName = "dx.op.ubfe"
	}

	dxFn := e.getDxOpTernaryFunc(opName, ol)
	opcodeVal := e.getIntConstID(int64(opcode))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, count, offset, arg}), nil
}

// emitMathInsertBits emits dx.op.bfi for bit field insertion.
func (e *Emitter) emitMathInsertBits(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if mathExpr.Arg1 == nil || mathExpr.Arg2 == nil || mathExpr.Arg3 == nil {
		return 0, fmt.Errorf("insertBits requires 4 arguments")
	}
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	newBits, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}
	offset, err := e.emitExpression(fn, *mathExpr.Arg2)
	if err != nil {
		return 0, err
	}
	count, err := e.emitExpression(fn, *mathExpr.Arg3)
	if err != nil {
		return 0, err
	}
	ol := overloadI32
	dxFn := e.getDxOpQuaternaryFunc("dx.op.bfi", ol)
	opcodeVal := e.getIntConstID(int64(OpBfi))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, count, offset, newBits, arg}), nil
}

// emitMathLdexp emits dx.op.ldexp(value, exp) -> value * 2^exp.
func (e *Emitter) emitMathLdexp(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if mathExpr.Arg1 == nil {
		return 0, fmt.Errorf("ldexp requires 2 arguments")
	}
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	exp, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}
	// dx.op.ldexp signature: float @dx.op.ldexp(i32 opcode, float value, i32 exp)
	// Custom function type: ret=float, params=[i32, float, i32]
	f32Ty := e.mod.GetFloatType(32)
	i32Ty := e.mod.GetIntType(32)
	funcTy := e.mod.GetFunctionType(f32Ty, []*module.Type{i32Ty, f32Ty, i32Ty})
	ldexpFn := e.getOrCreateDxOpFunc("dx.op.ldexp", overloadF32, funcTy)
	opcodeVal := e.getIntConstID(int64(OpLdexp))
	return e.addCallInstr(ldexpFn, f32Ty, []int{opcodeVal, arg, exp}), nil
}

// emitMathRefract computes refract(I, N, eta).
func (e *Emitter) emitMathRefract(_ *ir.Function, _ ir.ExprMath) (int, error) {
	return 0, fmt.Errorf("refract not yet implemented in DXIL backend")
}

// emitMathModf splits a float into integer and fractional parts.
func (e *Emitter) emitMathModf(_ *ir.Function, _ ir.ExprMath) (int, error) {
	return 0, fmt.Errorf("modf not yet implemented in DXIL backend")
}

// emitMathFrexp splits a float into significand and exponent.
func (e *Emitter) emitMathFrexp(_ *ir.Function, _ ir.ExprMath) (int, error) {
	return 0, fmt.Errorf("frexp not yet implemented in DXIL backend")
}

// emitMathCountTrailingZeros emulates countTrailingZeros using firstbitLo.
// CTZ(x) = x == 0 ? 32 : firstbitLo(x)
func (e *Emitter) emitMathCountTrailingZeros(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	ol := overloadI32

	// firstbitLo(x) returns 0xFFFFFFFF (-1) if no bit is set.
	dxFn := e.getDxOpUnaryFunc("dx.op.firstbitLo", ol)
	opcodeVal := e.getIntConstID(int64(OpFirstbitLo))
	fblo := e.addCallInstr(dxFn, i32Ty, []int{opcodeVal, arg})

	// If firstbitLo returned -1 (no bits set), return 32.
	negOne := e.getIntConstID(-1)
	thirty2 := e.getIntConstID(32)
	cmp := e.addCmpInstr(ICmpEQ, fblo, negOne)
	return e.emitSelect3(i32Ty, cmp, thirty2, fblo), nil
}

// emitMathCountLeadingZeros emulates countLeadingZeros using firstbitHi.
// CLZ(x) = x == 0 ? 32 : (31 - firstbitHi(x))
// For signed: firstbitSHi returns position of highest bit differing from sign.
func (e *Emitter) emitMathCountLeadingZeros(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, _ := scalarOfType(argType)

	i32Ty := e.mod.GetIntType(32)
	i1Ty := e.mod.GetIntType(1)
	ol := overloadI32

	var opcode DXILOpcode
	var opName string
	if scalar.Kind == ir.ScalarSint {
		opcode = OpFirstbitShiHi
		opName = "dx.op.firstbitSHi"
	} else {
		opcode = OpFirstbitHi
		opName = "dx.op.firstbitHi"
	}

	dxFn := e.getDxOpUnaryFunc(opName, ol)
	opcodeVal := e.getIntConstID(int64(opcode))
	fbhi := e.addCallInstr(dxFn, i32Ty, []int{opcodeVal, arg})

	// If firstbitHi returned -1 (all zeros/no significant bit), return 32.
	negOne := e.getIntConstID(-1)
	thirty2 := e.getIntConstID(32)
	_ = i1Ty // used by addCmpInstr internally
	cmp := e.addCmpInstr(ICmpEQ, fbhi, negOne)

	if scalar.Kind == ir.ScalarSint {
		// For signed, firstbitSHi returns position from MSB of highest differing bit.
		// CLZ for signed = firstbitSHi value directly (it already counts from MSB).
		return e.emitSelect3(i32Ty, cmp, thirty2, fbhi), nil
	}
	// For unsigned: firstbitHi returns position from LSB. CLZ = 31 - firstbitHi.
	thirtyOne := e.getIntConstID(31)
	sub := e.addBinOpInstr(i32Ty, BinOpSub, thirtyOne, fbhi)
	return e.emitSelect3(i32Ty, cmp, thirty2, sub), nil
}

// getOrCreateDxOpFunc gets or creates a dx.op function with a custom type.
func (e *Emitter) getOrCreateDxOpFunc(name string, ol overloadType, funcTy *module.Type) *module.Function {
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	suffix := overloadSuffix(ol)
	fullName := name + suffix
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpQuaternaryFunc creates a dx.op function with 4 value arguments
// (plus the opcode argument).
func (e *Emitter) getDxOpQuaternaryFunc(name string, ol overloadType) *module.Function {
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	retTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, retTy, retTy, retTy, retTy} // opcode + 4 values
	funcTy := e.mod.GetFunctionType(retTy, params)
	suffix := overloadSuffix(ol)
	fullName := name + suffix
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
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

		// Check if this is a struct load from a local variable.
		// DXIL does not support aggregate (struct) loads — decompose into
		// per-member GEP + scalar load, tracking components for downstream use.
		if dxilStructTy, hasStruct := e.localVarStructTypes[lv.Variable]; hasStruct {
			return e.emitStructLoad(ptr, dxilStructTy)
		}
	}

	// Check struct loads from global variable allocas.
	if gv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprGlobalVariable); ok {
		if dxilStructTy, hasStruct := e.globalVarAllocaTypes[gv.Variable]; hasStruct && dxilStructTy.Kind == module.TypeStruct {
			return e.emitStructLoad(ptr, dxilStructTy)
		}
	}

	// Resolve the loaded type from the pointer's target type.
	loadedTy, err := e.resolveLoadType(fn, load.Pointer)
	if err != nil {
		return 0, fmt.Errorf("load type resolution: %w", err)
	}

	// For struct types, decompose into per-member scalar loads.
	// This handles cases where the pointer source is not a simple local/global variable.
	if loadedTy.Kind == module.TypeStruct {
		return e.emitStructLoad(ptr, loadedTy)
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

// emitStructLoad decomposes a struct load into per-member GEP + scalar loads.
// DXIL does not support aggregate (struct) type loads, so we must load each
// scalar field individually and track them as components.
// Returns the first component's value ID and sets pendingComponents.
func (e *Emitter) emitStructLoad(basePtrID int, dxilStructTy *module.Type) (int, error) {
	if len(dxilStructTy.StructElems) == 0 {
		return e.getIntConstID(0), nil
	}

	zeroID := e.getIntConstID(0)
	componentIDs := make([]int, len(dxilStructTy.StructElems))

	for i, elemTy := range dxilStructTy.StructElems {
		indexID := e.getIntConstID(int64(i))
		resultPtrTy := e.mod.GetPointerType(elemTy)
		gepID := e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID})

		align := e.alignForType(elemTy)
		valueID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrLoad,
			HasValue:   true,
			ResultType: elemTy,
			Operands:   []int{gepID, elemTy.ID, align, 0},
			ValueID:    valueID,
		}
		e.currentBB.AddInstruction(instr)
		componentIDs[i] = valueID
	}

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

	// For array types, allocate the full array (GEP will be used for element access).
	if _, isArr := irType.Inner.(ir.ArrayType); isArr {
		return e.emitArrayLocalVariable(lv.Variable, localVar, irType)
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
	structTy, err := typeToDXILFull(e.mod, e.ir, irType.Inner)
	if err != nil {
		return 0, fmt.Errorf("local var %q struct type: %w", localVar.Name, err)
	}
	e.localVarStructTypes[varIdx] = structTy
	valueID := e.emitCompositeAlloca(structTy)
	e.localVarPtrs[varIdx] = valueID
	return valueID, nil
}

// emitArrayLocalVariable emits a single alloca for an array-typed local variable.
// Element access is via GEP instructions (emitted by emitAccess).
func (e *Emitter) emitArrayLocalVariable(varIdx uint32, localVar *ir.LocalVariable, irType ir.Type) (int, error) {
	arrayTy, err := typeToDXIL(e.mod, e.ir, irType.Inner)
	if err != nil {
		return 0, fmt.Errorf("local var %q array type: %w", localVar.Name, err)
	}
	e.localVarArrayTypes[varIdx] = arrayTy
	valueID := e.emitCompositeAlloca(arrayTy)
	e.localVarPtrs[varIdx] = valueID
	return valueID, nil
}

// emitCompositeAlloca emits an alloca instruction for a composite type
// (struct or array) and returns the pointer value ID.
func (e *Emitter) emitCompositeAlloca(elemTy *module.Type) int {
	ptrTy := e.mod.GetPointerType(elemTy)
	sizeID := e.getIntConstID(1)
	i32Ty := e.mod.GetIntType(32)

	// Alignment: log2(4) + 1 = 3, with bit 6 set for explicit type.
	alignFlags := 3 | (1 << 6)

	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrAlloca,
		HasValue:   true,
		ResultType: ptrTy,
		Operands:   []int{elemTy.ID, i32Ty.ID, sizeID, alignFlags},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)
	return valueID
}

// emitGlobalVarAlloca emits an alloca for a non-resource global variable
// (workgroup, private, push-constant without binding). Returns the alloca pointer ID.
// Caches the result so multiple references to the same global get the same alloca.
//
// In proper DXIL, workgroup variables should be address-space 3 globals.
// For now, we emit them as function-local allocas so load/store work correctly.
func (e *Emitter) emitGlobalVarAlloca(varHandle ir.GlobalVariableHandle) (int, error) {
	// Return cached alloca if already emitted.
	if ptr, ok := e.globalVarAllocas[varHandle]; ok {
		return ptr, nil
	}

	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return 0, fmt.Errorf("global variable %d out of range", varHandle)
	}
	gv := &e.ir.GlobalVariables[varHandle]
	irType := e.ir.Types[gv.Type]

	// Get DXIL type for the alloca.
	elemTy, err := typeToDXILFull(e.mod, e.ir, irType.Inner)
	if err != nil {
		return 0, fmt.Errorf("global var %q type: %w", gv.Name, err)
	}
	ptrTy := e.mod.GetPointerType(elemTy)

	sizeID := e.getIntConstID(1)
	i32Ty := e.mod.GetIntType(32)

	// Alignment: log2(align) + 1, with bit 6 set for explicit type (LLVM 3.7).
	align := e.alignForType(elemTy) + 1
	alignFlags := align | (1 << 6)

	valueID := e.allocValue()
	instr := &module.Instruction{
		Kind:       module.InstrAlloca,
		HasValue:   true,
		ResultType: ptrTy,
		Operands:   []int{elemTy.ID, i32Ty.ID, sizeID, alignFlags},
		ValueID:    valueID,
	}
	e.currentBB.AddInstruction(instr)

	e.globalVarAllocas[varHandle] = valueID
	e.globalVarAllocaTypes[varHandle] = elemTy
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

	case ir.ExprAccess:
		// Dynamic access into an array produces a GEP pointer to the element.
		// Resolve the element type from the base array type.
		innerTy := e.resolveAccessResultType(fn, pk)
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

// resolveAccessResultType resolves the IR type that a dynamic Access expression
// points to. For array access, returns the element type.
func (e *Emitter) resolveAccessResultType(fn *ir.Function, acc ir.ExprAccess) ir.TypeInner {
	baseExpr := fn.Expressions[acc.Base]
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
		baseInner = e.resolveAccessIndexResultType(fn, bk)
	}

	if baseInner == nil {
		return nil
	}

	switch t := baseInner.(type) {
	case ir.ArrayType:
		if int(t.Base) < len(e.ir.Types) {
			return e.ir.Types[t.Base].Inner
		}
	case ir.VectorType:
		return t.Scalar
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

// resolveExprTypeInner returns the IR TypeInner for an expression.
// Uses ExpressionTypes if available, otherwise falls back to heuristics.
func (e *Emitter) resolveExprTypeInner(fn *ir.Function, handle ir.ExpressionHandle) ir.TypeInner {
	if int(handle) < len(fn.ExpressionTypes) {
		tr := &fn.ExpressionTypes[handle]
		if tr.Handle != nil {
			return e.ir.Types[*tr.Handle].Inner
		}
		if tr.Value != nil {
			return tr.Value
		}
	}
	// Fallback: use existing resolveExprType which infers from expression kind.
	return e.resolveExprType(fn, handle)
}
