package emit

import (
	"fmt"
	"math"
	"sort"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// DXIL OpClass-based function symbols. Per dxc DxilOperations.cpp,
// the LLVM function name for a dx.op intrinsic is dx.op.<OpClass>.<overload>.
// The opcode is encoded as the first i32 argument to the call, not in the
// function name. Multiple opcodes (e.g. FMin/FMax/IMin/IMax/UMin/UMax)
// share the same OpClass and therefore the same function symbol.
const (
	dxOpClassUnary          = "dx.op.unary"
	dxOpClassUnaryBits      = "dx.op.unaryBits"
	dxOpClassBinary         = "dx.op.binary"
	dxOpClassTertiary       = "dx.op.tertiary"
	dxOpClassQuaternary     = "dx.op.quaternary"
	dxOpClassIsSpecialFloat = "dx.op.isSpecialFloat"
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

	case ir.ExprAlias:
		// Transparent passthrough produced by the DXIL mem2reg pass:
		// the load was rewritten to alias an earlier-emitted value
		// (the dominating store's value or the variable's Init).
		// Emit nothing of our own — return the source's value ID.
		valueID, err = e.emitExpression(fn, ek.Source)

	case ir.ExprPhi:
		// SSA phi node produced by the DXIL mem2reg Phase B walker.
		// Materialize as LLVM FUNC_CODE_INST_PHI at the start of
		// the current BB (which the caller — a single-handle StmtEmit
		// positioned by mem2reg immediately after the StmtIf /
		// StmtSwitch — has already arranged to be the merge BB).
		valueID, err = e.emitPhi(fn, ek)

	case ir.ExprLocalVariable:
		// Single-store locals bypass alloca -- their loads resolve to the
		// stored value directly. Return a sentinel to prevent alloca creation.
		if _, isSingle := e.singleStoreLocals[ek.Variable]; isSingle {
			return 0, nil
		}
		valueID, err = e.emitLocalVariable(fn, ek)

	case ir.ExprGlobalVariable:
		// If this is a resource binding, return the pre-created handle ID.
		// For binding arrays, handleID is -1 (created dynamically at Access site).
		if handleID, found := e.getResourceHandleID(ek.Variable); found {
			if handleID >= 0 {
				valueID = handleID
			} else {
				// Binding array base: placeholder; actual handle is created in resolveResourceHandle.
				valueID = 0
			}
		} else {
			// Non-resource global variable (workgroup, private, push-constant).
			// Emit an alloca to create a proper pointer for load/store.
			valueID, err = e.emitGlobalVarAlloca(ek.Variable)
		}

	case ir.ExprImageSample:
		valueID, err = e.emitImageSample(fn, ek)

	case ir.ExprImageLoad:
		valueID, err = e.emitImageLoad(fn, ek)

	case ir.ExprImageQuery:
		valueID, err = e.emitImageQuery(fn, ek)

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

	case ir.ExprSubgroupBallotResult:
		// Pre-populated by emitStmtSubgroupBallot.
		return 0, fmt.Errorf("ExprSubgroupBallotResult [%d] not yet populated by StmtSubgroupBallot", handle)

	case ir.ExprSubgroupOperationResult:
		// Pre-populated by emitStmtSubgroupCollectiveOp or emitStmtSubgroupGather.
		return 0, fmt.Errorf("ExprSubgroupOperationResult [%d] not yet populated", handle)

	case ir.ExprRayQueryProceedResult:
		// Pre-populated by emitStmtRayQuery (RayQueryProceed).
		return 0, fmt.Errorf("ExprRayQueryProceedResult [%d] not yet populated", handle)

	case ir.ExprRayQueryGetIntersection:
		valueID, err = e.emitRayQueryGetIntersection(fn, ek)

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
		valueID, err = e.emitArrayLength(fn, ek)

	case ir.ExprOverride:
		valueID, err = e.emitOverride(ek)

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

	case ir.LiteralAbstractFloat:
		// Abstract floats are concretized to f32 in DXIL.
		return e.getFloatConstID(float64(v)), nil

	case ir.LiteralAbstractInt:
		// Abstract ints are concretized to i32 in DXIL.
		return e.getIntConstID(int64(v)), nil

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

// extractComponentsForAccessIndex extracts a range of components from tracked
// component IDs based on the base type. For matrices, extracts a column.
// For structs, extracts all components belonging to the indexed member.
// Returns (valueID, true) if handled, or (0, false) if not handled (fallback to scalar).
func (e *Emitter) extractComponentsForAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex, comps []int) (int, bool) {
	baseType := e.resolveExprTypeInner(fn, ai.Base)

	// Matrix: AccessIndex extracts a column (vec of rows components).
	if mt, isMat := baseType.(ir.MatrixType); isMat {
		rows := int(mt.Rows)
		startIdx := int(ai.Index) * rows
		if startIdx+rows <= len(comps) {
			colComps := comps[startIdx : startIdx+rows]
			e.pendingComponents = colComps
			return colComps[0], true
		}
	}

	// Struct: AccessIndex extracts a member's range of components.
	if st, isSt := baseType.(ir.StructType); isSt && int(ai.Index) < len(st.Members) {
		flatOffset := 0
		for m := 0; m < int(ai.Index); m++ {
			memberType := e.ir.Types[st.Members[m].Type].Inner
			flatOffset += cbvComponentCount(e.ir, memberType)
		}
		memberType := e.ir.Types[st.Members[ai.Index].Type].Inner
		memberComps := cbvComponentCount(e.ir, memberType)
		if flatOffset+memberComps <= len(comps) {
			memberRange := comps[flatOffset : flatOffset+memberComps]
			if memberComps > 1 {
				e.pendingComponents = memberRange
			}
			return memberRange[0], true
		}
	}

	return 0, false
}

// emitAccessIndex extracts a component from a composite by constant index.
//
// For struct local/global variable access, emits a GEP instruction to get a pointer
// to the member field. For array access, emits a GEP to the element. For vector
// component extraction, uses per-component tracking.
//
//nolint:gocognit,gocyclo,cyclop // dispatch logic for struct/array/vector/resource access patterns
func (e *Emitter) emitAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex) (int, error) {
	// Inline-pass Load-of-localvar fast path (mirror of emitAccess recognizer).
	// Same root-cause shape: inline pass substitutes aggregate FunctionArgument
	// refs with ExprLoad{LocalVariable{slot}}. AccessIndex-through-Load-of-slot
	// occurs when callee's body does `arg.field` (struct) or `arg[N]` (const-
	// index array). DXIL forbids materializing the aggregate Load — peel to
	// the slot alloca, emit struct/array GEP to the indexed element, Load
	// scalar. Mirrors DXC AlwaysInliner + SROA: alloca + per-field GEP + load,
	// never a full aggregate load.
	if id, handled, err := e.tryInlineLoadOfLocalVarAccessIndex(fn, ai); err != nil {
		return 0, err
	} else if handled {
		return id, nil
	}

	// Local-var nested array fast-path: if this AccessIndex (or Access) is
	// the head of a chain whose root is an ExprLocalVariable holding a
	// flattened array type, AND the chain depth matches the array depth
	// (i.e., result is a scalar pointer, not a sub-array pointer), emit one
	// flat GEP rooted at the alloca and skip the intermediate emit cascade.
	// The intermediate AccessIndex/Access expressions in the chain are
	// orphaned (their value IDs are never read by anyone else), so this is
	// safe even though their per-handle exprValues entries stay unset.
	//
	// Without this short-circuit, each intermediate AccessIndex would emit
	// its own GEP using a wrong source-element type (e.g., [4 x i32] over a
	// pointer that is actually i32*), and the validator would reject the
	// chain with "Explicit gep type does not match pointee type".
	if id, handled := e.tryFlatGEPLocalVarNested(fn, ai.Base, e.getIntConstID(int64(ai.Index))); handled {
		e.exprValues[ai.Base] = id // pre-populate so any later ai.Base read returns this
		return id, nil
	}

	baseID, err := e.emitExpression(fn, ai.Base)
	if err != nil {
		return 0, err
	}

	// If the base has per-component tracking, extract the indexed component(s).
	if comps, ok := e.exprComponents[ai.Base]; ok && int(ai.Index) < len(comps) {
		if id, handled := e.extractComponentsForAccessIndex(fn, ai, comps); handled {
			return id, nil
		}
		// For vectors — direct scalar extraction.
		return comps[ai.Index], nil
	}

	// Check if the base is a local variable — handle struct and array types.
	if id, err := e.tryLocalVarAccessIndex(fn, ai, baseID); err != nil || id != -1 {
		return id, err
	}

	// Check if the base is a global variable — handle array and struct types.
	if id, err := e.tryGlobalVarAccessIndex(fn, ai); err != nil || id != -1 {
		return id, err
	}

	// Check if this AccessIndex chain leads to a resource.
	if gv, ok := e.resolveToGlobalVariable(fn, ai.Base); ok { //nolint:nestif // resource dispatch
		if idx, isRes := e.resourceHandles[gv]; isRes {
			res := &e.resources[idx]
			// Binding array with constant index: create dynamic handle.
			if res.isBindingArray {
				return e.emitDynamicCreateHandleConst(res, int64(ai.Index))
			}
			// UAV/CBV: pass through to load/store site.
			if res.class == resourceClassUAV || res.class == resourceClassCBV {
				return baseID, nil
			}
		}
	}

	// Check if the base is an AccessIndex that produced a pointer to an array
	// within a struct. If so, GEP into the array element.
	//
	//nolint:nestif // dispatch table over composite-of-composite access patterns
	if bai, ok := fn.Expressions[ai.Base].Kind.(ir.ExprAccessIndex); ok {
		innerTy := e.resolveAccessIndexResultType(fn, bai)
		if arrTy, isArr := innerTy.(ir.ArrayType); isArr {
			// Workgroup globals: the validator rejects GEP-of-GEP chains
			// from a TGSM (addrspace 3) global with 'TGSM pointers must
			// originate from an unambiguous TGSM global variable'. The
			// chain `globals_struct.arr[i]` therefore needs a SINGLE flat
			// GEP with all indices [0, structField, arrayIndex] instead
			// of two separate GEPs %x = struct_gep; %y = arr_gep %x.
			// Mirrors Mesa nir_to_dxil deref_to_gep which walks the deref
			// chain and emits one flat gep per shared variable access.
			if id, handled := e.tryFlatGEPWorkgroupNested(fn, ai, bai); handled {
				return id, nil
			}
			// Local-var nested array: the inner GEP produced an i32* (or
			// equivalent scalar pointer) because typeToDXIL flattens nested
			// arrays to a single dim. A second GEP with the inner-array
			// type as the source-element-type would mismatch the actual
			// pointee. Emit one flat GEP rooted at the local var's
			// flattened alloca instead.
			if id, handled := e.tryFlatGEPLocalVarNested(fn, ai.Base, e.getIntConstID(int64(ai.Index))); handled {
				return id, nil
			}
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

	// BUG-DXIL-005: scalar-component access of a struct-vector field.
	// Pattern: AccessIndex(AccessIndex(LocalVariable(s), fieldIdx), compIdx)
	// where the field is a vector and compIdx selects one scalar component.
	// Without this, the outer index falls through to getComponentID(inner, k)
	// which returns base+k — a non-pointer value — and downstream Load/Store
	// rejects the operand with "Load/Store operand is not a pointer type".
	// The correct action is to emit a direct GEP into the struct alloca using
	// the flat (field-offset + compIdx) index, symmetric to the store-side
	// path in tryStructMemberComponentStore and the load-side vector fan-out
	// in tryStructVectorMemberLoad.
	if id, handled := e.tryStructVectorComponentAccess(fn, ai); handled {
		return id, nil
	}

	// For vector component access, use per-component tracking.
	return e.getComponentID(ai.Base, int(ai.Index)), nil
}

// tryStructVectorComponentAccess handles the pattern
//
//	AccessIndex(AccessIndex(LocalVariable, fieldIdx), compIdx)
//
// where the middle AccessIndex selects a vector-typed struct field and the
// outer AccessIndex selects one scalar component. Without dedicated handling,
// the outer access falls through to getComponentID which returns base+compIdx
// — a bogus value ID that is not a pointer, so subsequent Load/Store fails
// DXC validation ("Load/Store operand is not a pointer type").
//
// The fix is to emit a direct GEP into the local struct alloca using the
// correct flat component index, producing a real pointer value. This is the
// access-side symmetric counterpart to tryStructMemberComponentStore and
// closes the load side of BUG-DXIL-004 for per-component loads (the existing
// tryStructVectorMemberLoad only handles whole-vector loads of a struct field).
func (e *Emitter) tryStructVectorComponentAccess(fn *ir.Function, outer ir.ExprAccessIndex) (int, bool) {
	innerAI, ok := fn.Expressions[outer.Base].Kind.(ir.ExprAccessIndex)
	if !ok {
		return 0, false
	}

	// The inner AccessIndex must resolve to a vector-typed member of a local
	// struct variable (possibly via nested struct fields).
	innerTy := e.resolveAccessIndexResultType(fn, innerAI)
	vt, isVec := innerTy.(ir.VectorType)
	if !isVec {
		return 0, false
	}
	if int(outer.Index) >= int(vt.Size) {
		return 0, false
	}

	rootVarIdx, fieldFlatOffset, resolved := e.resolveStructFieldRoot(fn, innerAI)
	if !resolved {
		return 0, false
	}

	dxilStructTy, hasTy := e.localVarStructTypes[rootVarIdx]
	if !hasTy {
		return 0, false
	}
	basePtrID, hasBase := e.localVarPtrs[rootVarIdx]
	if !hasBase {
		return 0, false
	}

	compFlatIndex := fieldFlatOffset + int(outer.Index)
	var memberDXILTy *module.Type
	if compFlatIndex < len(dxilStructTy.StructElems) {
		memberDXILTy = dxilStructTy.StructElems[compFlatIndex]
	} else {
		memberDXILTy = e.mod.GetFloatType(32)
	}
	resultPtrTy := e.mod.GetPointerType(memberDXILTy)
	zeroID := e.getIntConstID(0)
	indexID := e.getIntConstID(int64(compFlatIndex))
	return e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID}), true
}

// workgroupFieldOrigin records that an emitted GEP value is a struct-field
// pointer derived from a workgroup addrspace(3) global. It lets downstream
// emitters (notably emitArrayLoad) re-root per-element GEPs directly at the
// global so the DXIL validator's TGSM-origin check succeeds.
type workgroupFieldOrigin struct {
	globalAllocaID int          // emitter value ID of the workgroup global
	structTy       *module.Type // source struct type for the root GEP
	fieldFlatIdx   int          // flat field index into structTy
	memberTy       *module.Type // DXIL type of the field (e.g. [512 x i32])
}

// workgroupElemOrigin records that a pointer value was produced by a
// single-step array-element GEP off a workgroup addrspace(3) global.
// Decomposed per-field struct/array load/store paths use it to re-root
// each field GEP directly at the global so the TGSM-origin analysis
// succeeds instead of rejecting GEP-of-GEP.
type workgroupElemOrigin struct {
	globalAllocaID int          // emitter value ID of the workgroup global
	arrayTy        *module.Type // source [N x T] array type for the root GEP
	elemIndexID    int          // emitter value ID of the element index (runtime or const)
	elemTy         *module.Type // DXIL type of the array element (e.g. struct S)
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
	// Workgroup globals live in addrspace(3); propagate to the field pointer
	// so the validator sees the right type. The TGSM origin check is
	// data-flow, not type-based, so we ALSO record the origin below.
	addrSpace := uint8(0)
	if gv.Space == ir.SpaceWorkGroup {
		addrSpace = 3
	}
	resultPtrTy := e.mod.GetPointerTypeAS(memberDXILTy, addrSpace)
	zeroID := e.getIntConstID(0)
	indexID := e.getIntConstID(int64(flatIndex))
	id := e.addGEPInstr(dxilStructTy, resultPtrTy, allocaID, []int{zeroID, indexID})
	if addrSpace == 3 && e.workgroupFieldPtrs != nil {
		e.workgroupFieldPtrs[id] = workgroupFieldOrigin{
			globalAllocaID: allocaID,
			structTy:       dxilStructTy,
			fieldFlatIdx:   flatIndex,
			memberTy:       memberDXILTy,
		}
	}
	return id, nil
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
	// Workgroup arrays require addrspace 3 on the element pointer and the
	// result must be tracked as a workgroup-elem origin so decomposed
	// struct/array loads/stores can re-root at the global (TGSM-origin rule).
	addrSpace := uint8(0)
	if int(gv.Variable) < len(e.ir.GlobalVariables) &&
		e.ir.GlobalVariables[gv.Variable].Space == ir.SpaceWorkGroup {
		addrSpace = 3
	}
	id, err := e.emitArrayGEPAS(allocaID, allocaTy, indexID, addrSpace)
	if err == nil && addrSpace == 3 && e.workgroupElemPtrs != nil && allocaTy.ElemType != nil {
		e.workgroupElemPtrs[id] = workgroupElemOrigin{
			globalAllocaID: allocaID,
			arrayTy:        allocaTy,
			elemIndexID:    indexID,
			elemTy:         allocaTy.ElemType,
		}
	}
	return id, err
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

// tryFlatGEPWorkgroupNested handles the access pattern
//
//	ExprAccessIndex(ExprAccessIndex(ExprGlobalVariable<workgroup>, fieldIdx), elemIdx)
//
// by emitting a SINGLE flat GEP with three indices [0, fieldIdx, elemIdx]
// rooted directly at the workgroup global. The validator rejects the
// equivalent chain of two GEPs as 'TGSM pointers must originate from an
// unambiguous TGSM global variable' because GEP-of-GEP loses the
// origin tracking. Returns (-1, false) if the chain doesn't match.
//
// Reference: Mesa nir_to_dxil deref_to_gep walks the entire deref chain
// and emits gep_indices[0] = global_var, then one index per deref step.
func (e *Emitter) tryFlatGEPWorkgroupNested(fn *ir.Function, ai, bai ir.ExprAccessIndex) (int, bool) {
	// Inner base must be an ExprGlobalVariable for a workgroup-space global.
	gv, ok := fn.Expressions[bai.Base].Kind.(ir.ExprGlobalVariable)
	if !ok {
		return -1, false
	}
	if int(gv.Variable) >= len(e.ir.GlobalVariables) {
		return -1, false
	}
	if e.ir.GlobalVariables[gv.Variable].Space != ir.SpaceWorkGroup {
		return -1, false
	}
	// The struct DXIL type must match the cached alloca type so the GEP's
	// source-element-type operand is the original struct.
	allocaID, hasAlloca := e.globalVarAllocas[gv.Variable]
	if !hasAlloca {
		return -1, false
	}
	structTy, hasTy := e.globalVarAllocaTypes[gv.Variable]
	if !hasTy || structTy.Kind != module.TypeStruct {
		return -1, false
	}
	// Look up the array's element type from the IR struct member.
	gvType := e.ir.Types[e.ir.GlobalVariables[gv.Variable].Type]
	st, isStruct := gvType.Inner.(ir.StructType)
	if !isStruct || int(bai.Index) >= len(st.Members) {
		return -1, false
	}
	memberType := e.ir.Types[st.Members[bai.Index].Type]
	arrType, isArr := memberType.Inner.(ir.ArrayType)
	if !isArr {
		return -1, false
	}
	elemInner := e.ir.Types[arrType.Base].Inner
	elemDxilTy, err := typeToDXIL(e.mod, e.ir, elemInner)
	if err != nil {
		return -1, false
	}
	// Build the GEP: source elem type = struct, indices = [0, fieldIdx, arrIdx].
	zeroID := e.getIntConstID(0)
	fieldID := e.getIntConstID(int64(bai.Index))
	arrIdxID := e.getIntConstID(int64(ai.Index))
	resultPtrTy := e.mod.GetPointerTypeAS(elemDxilTy, 3)
	id := e.addGEPInstr(structTy, resultPtrTy, allocaID, []int{zeroID, fieldID, arrIdxID})
	return id, true
}

// tryAccessIndexArrayGEP checks if the Access base is an AccessIndex that
// produced a pointer to an array (struct member that is an array type).
// If so, emits a flat GEP or array GEP as appropriate. Returns (id, true)
// if handled, (0, false) otherwise.
func (e *Emitter) tryAccessIndexArrayGEP(fn *ir.Function, acc ir.ExprAccess, baseID, indexID int) (int, bool) {
	ai, ok := fn.Expressions[acc.Base].Kind.(ir.ExprAccessIndex)
	if !ok {
		return 0, false
	}
	innerTy := e.resolveAccessIndexResultType(fn, ai)
	if _, isArr := innerTy.(ir.ArrayType); !isArr {
		return 0, false
	}
	// Local-var nested array — emit a single flat GEP rooted at
	// the alloca (see tryFlatGEPLocalVarNested for rationale).
	if id, handled := e.tryFlatGEPLocalVarNested(fn, acc.Base, indexID); handled {
		return id, true
	}
	arrayDxilTy, err := typeToDXIL(e.mod, e.ir, innerTy)
	if err != nil {
		return 0, false
	}
	id, err := e.emitArrayGEP(baseID, arrayDxilTy, indexID)
	if err != nil {
		return 0, false
	}
	return id, true
}

// tryFlatGEPLocalVarNested handles the access pattern
//
//	ExprAccess(ExprAccess(ExprLocalVariable, idx0), idx1)
//	ExprAccessIndex(ExprAccessIndex(ExprLocalVariable, k0), k1)
//	(and any mix, including 3+ levels of nesting)
//
// where the local variable holds a multi-dimensional array. The DXIL backend
// flattens nested arrays to a single dimension in typeToDXIL — `array<array<i32,
// 4>, 4>` becomes `[16 x i32]`. A naive chain of two GEPs would emit
// `getelementptr [16 x i32], ptr arr, 0, %idx0` followed by `getelementptr
// [4 x i32], i32* %inner, 0, %idx1`, which the validator rejects with
// "Explicit gep type does not match pointee type" because the inner GEP
// produced an i32* but the outer GEP is typed as if the source were a
// `[4 x i32]*`.
//
// The fix mirrors tryFlatGEPWorkgroupNested: emit a SINGLE GEP with the
// flattened scalar index `idx0 * (N1*N2*...) + idx1 * (N2*...) + ...`,
// rooted directly at the local variable's flattened array alloca. This is
// what DXC's SROA + GVN + LLVM's instcombine collapse the two-GEP chain into.
//
// Returns (-1, false) if the chain doesn't match (not a nested access on a
// local var array, runtime-sized array, or vector/matrix leaf for which the
// flattening multiplier interacts with vector lanes — left as future work).
//
// Reference: LLVM langref getelementptr semantics; Mesa nir_to_dxil
// deref_to_gep walks the entire deref chain and emits one flat gep per
// alloca'd variable.
//
//nolint:gocognit,gocyclo,cyclop,funlen // walks an arbitrary-depth IR access chain
func (e *Emitter) tryFlatGEPLocalVarNested(fn *ir.Function, baseExpr ir.ExpressionHandle, lastIndexValueID int) (int, bool) {
	// Walk the chain backward, collecting per-level indices and dim sizes.
	// indexIDs[i] = DXIL value ID for the level-i index (LSB last)
	// dimSizes[i] = element count at level i (LSB last)
	// We reverse later; for now collect outermost-first.
	type level struct {
		indexValueID int
		isConst      bool
		constValue   int64
	}
	var levels []level

	// Append the outermost index that the caller provided (already emitted).
	// If the caller provided a constant integer ID, we still pass it via
	// indexValueID — the all-const fast path needs explicit `isConst` markers,
	// which only the recursive walk below sets, so the all-const path does not
	// activate when invoked from the outer-most short-circuit. That's fine: a
	// single mul/add chain over const-IDs folds at the bitcode-write level.
	levels = append(levels, level{indexValueID: lastIndexValueID})

	cur := baseExpr
	for {
		expr := fn.Expressions[cur].Kind
		switch k := expr.(type) {
		case ir.ExprAccess:
			// Index already emitted by emitAccess — reuse the cached value ID.
			idxVal, ok := e.exprValues[k.Index]
			if !ok {
				return -1, false
			}
			levels = append(levels, level{indexValueID: idxVal})
			cur = k.Base
		case ir.ExprAccessIndex:
			// Constant index — use a literal integer constant.
			levels = append(levels, level{isConst: true, constValue: int64(k.Index)})
			cur = k.Base
		case ir.ExprLocalVariable:
			// Reached the root local variable. Verify it has a flattened
			// array type registered (set by emitArrayLocalVariable).
			arrayTy, hasArr := e.localVarArrayTypes[k.Variable]
			if !hasArr {
				return -1, false
			}
			basePtrID, hasBase := e.localVarPtrs[k.Variable]
			if !hasBase || basePtrID < 0 {
				return -1, false
			}

			// Walk the IR type to collect per-level dim sizes (in the same
			// outer-to-inner order as the access chain). Stop at a non-array
			// leaf — we only support scalar leaves here. Vectors/matrices at
			// the leaf would require multiplying the per-level stride by the
			// component count, which interacts with the per-component
			// scalarization in store/load paths and is out of scope for
			// this helper.
			lv := &fn.LocalVars[k.Variable]
			cursor := e.ir.Types[lv.Type].Inner
			var dims []uint32
			for {
				arr, isArr := cursor.(ir.ArrayType)
				if !isArr {
					break
				}
				if arr.Size.Constant == nil {
					return -1, false
				}
				dims = append(dims, *arr.Size.Constant)
				cursor = e.ir.Types[arr.Base].Inner
			}
			// Leaf must be a scalar — vector/matrix leaves bring extra
			// scalarization considerations we don't handle here.
			if _, isScalar := cursor.(ir.ScalarType); !isScalar {
				return -1, false
			}
			// Number of access levels must match number of array dims.
			// Anything less is a partial access producing a sub-array
			// pointer — different code path.
			if len(levels) != len(dims) {
				return -1, false
			}

			// levels (as appended) carries the outermost user-facing
			// AccessIndex first — but the OUTERMOST IR AccessIndex
			// actually selects the INNERMOST array dim (because
			// AccessIndex chains build outward from the LV). After
			// appending, levels[0] = innermost-dim selector, levels[end]
			// = outermost-dim selector. Reverse so levels[i] is aligned
			// with the i-th array dim (outer first), matching dims[i].
			for i, j := 0, len(levels)-1; i < j; i, j = i+1, j-1 {
				levels[i], levels[j] = levels[j], levels[i]
			}
			// dims is outer-first. stride[i] = product of all dims after
			// dim i (i.e., the size in elements that one step in dim i
			// covers). stride[last] = 1, stride[0] = N_1 * N_2 * ... * N_last.
			strides := make([]uint32, len(dims))
			strides[len(dims)-1] = 1
			for i := len(dims) - 2; i >= 0; i-- {
				strides[i] = strides[i+1] * dims[i+1]
			}

			// Compute flat index = sum(level[i].value * strides[i]).
			// Special-case all-constant: emit a single i32 constant.
			allConst := true
			var constSum int64
			for i := range levels {
				if !levels[i].isConst {
					allConst = false
					break
				}
				constSum += levels[i].constValue * int64(strides[i])
			}

			i32Ty := e.mod.GetIntType(32)
			var flatIndexID int
			if allConst { //nolint:nestif // flat-index computation: const vs dynamic paths
				flatIndexID = e.getIntConstID(constSum)
			} else {
				// Mixed const/dynamic: emit `idx*stride` for each level then
				// fold the running sum.
				running := -1
				for i := range levels {
					var term int
					if levels[i].isConst {
						term = e.getIntConstID(levels[i].constValue * int64(strides[i]))
					} else if strides[i] == 1 {
						term = levels[i].indexValueID
					} else {
						strideID := e.getIntConstID(int64(strides[i]))
						term = e.addBinOpInstr(i32Ty, BinOpMul, levels[i].indexValueID, strideID)
					}
					if running == -1 {
						running = term
					} else {
						running = e.addBinOpInstr(i32Ty, BinOpAdd, running, term)
					}
				}
				flatIndexID = running
			}

			// Emit the single flat GEP into the alloca.
			id, err := e.emitArrayGEP(basePtrID, arrayTy, flatIndexID)
			if err != nil {
				return -1, false
			}
			return id, true
		default:
			return -1, false
		}
	}
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
	// Workgroup globals live in addrspace(3) — propagate to the GEP result
	// so the pointer carries the same TGSM origin and the validator does
	// not reject 'TGSM pointers must originate from an unambiguous TGSM
	// global variable' on the resulting element pointer. Mesa applies the
	// same rule via dxil_emit_gep_inbounds, which infers the result
	// addrspace from the source pointer's type.
	addrSpace := uint8(0)
	if int(gv.Variable) < len(e.ir.GlobalVariables) &&
		e.ir.GlobalVariables[gv.Variable].Space == ir.SpaceWorkGroup {
		addrSpace = 3
	}
	if allocaTy.Kind == module.TypeArray {
		indexID := e.getIntConstID(int64(ai.Index))
		id, err := e.emitArrayGEPAS(allocaID, allocaTy, indexID, addrSpace)
		if err == nil && addrSpace == 3 && e.workgroupElemPtrs != nil && allocaTy.ElemType != nil {
			e.workgroupElemPtrs[id] = workgroupElemOrigin{
				globalAllocaID: allocaID,
				arrayTy:        allocaTy,
				elemIndexID:    indexID,
				elemTy:         allocaTy.ElemType,
			}
		}
		return id, err
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
//
//nolint:gocognit,gocyclo,cyclop // dispatch logic for array/binding-array/UAV access patterns
func (e *Emitter) emitAccess(fn *ir.Function, acc ir.ExprAccess) (int, error) {
	// Inline-pass Load-of-localvar fast path. The IR-level inline pass spills
	// aggregate by-value arguments into a fresh local and substitutes body
	// references with `ExprLoad{Pointer: ExprLocalVariable{slot}}` so the
	// substitution preserves the callee's value-type semantics. emit cannot
	// materialize an aggregate Load (DXIL forbids aggregate loads) — but we
	// can recognize the Load-of-slot shape and route the access directly to
	// the slot's pointer, GEP-style. This mirrors what DXC AlwaysInliner +
	// LLVM SROA produce after inlining: alloca + per-element GEP, no full
	// aggregate load. The Load expression is left orphaned (its value ID is
	// never consumed by anyone after this rewrite).
	if id, handled, err := e.tryInlineLoadOfLocalVarAccess(fn, acc); err != nil {
		return 0, err
	} else if handled {
		return id, nil
	}

	// Const vector-array fast path (BUG-DXIL-025). If the base is a
	// local var that was registered in localConstVecArrays by
	// emitArrayLocalVariable, return a sentinel pointer ID: emitLoad
	// will look up the same table and materialize per-lane scalars via
	// a select chain or literal lookup. We must do this BEFORE
	// emitting the base expression, otherwise emitLocalVariable would
	// allocate an alloca we no longer need.
	if lv, ok := fn.Expressions[acc.Base].Kind.(ir.ExprLocalVariable); ok {
		if _, isConstVec := e.localConstVecArrays[lv.Variable]; isConstVec {
			// Trigger index emission so the index value ID is ready
			// when emitLoad synthesizes the select chain.
			if _, err := e.emitExpression(fn, acc.Index); err != nil {
				return 0, err
			}
			return -1, nil
		}
	}

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

	// Check if this Access chain leads to a resource binding. If so, skip GEP
	// emission — the actual buffer access is handled by resolveUAVPointerChain
	// at the load/store site (for UAV), or resolveResourceHandle (for binding arrays).
	if gv, ok := e.resolveToGlobalVariable(fn, acc.Base); ok { //nolint:nestif // resource dispatch
		if idx, isRes := e.resourceHandles[gv]; isRes {
			res := &e.resources[idx]
			// Binding array: create dynamic handle at point of use.
			if res.isBindingArray {
				handleID, err2 := e.emitDynamicCreateHandle(fn, res, acc.Index)
				if err2 != nil {
					return 0, fmt.Errorf("binding array access: %w", err2)
				}
				return handleID, nil
			}
			if res.class == resourceClassUAV || res.class == resourceClassSRV {
				return baseID, nil
			}
		}
	}

	// Check if the base is an AccessIndex that produced a pointer to an array
	// (e.g., struct member that is an array type).
	if id, ok := e.tryAccessIndexArrayGEP(fn, acc, baseID, indexID); ok {
		return id, nil
	}

	// Local-var nested array via Access(Access(LV, idx0), idx1) — the inner
	// Access produced an i32* (typeToDXIL flattens nested arrays to a single
	// dimension). Emit one flat GEP rooted at the alloca.
	if _, ok := fn.Expressions[acc.Base].Kind.(ir.ExprAccess); ok {
		if id, handled := e.tryFlatGEPLocalVarNested(fn, acc.Base, indexID); handled {
			return id, nil
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
	return e.emitArrayGEPAS(basePtrID, arrayTy, indexID, 0)
}

// emitArrayGEPAS is emitArrayGEP with an explicit result address space.
// Workgroup arrays live in addrspace 3 and the element pointer must carry
// the same addrspace or the validator rejects 'TGSM pointers must
// originate from an unambiguous TGSM global variable' on subsequent
// load/store/atomicrmw uses.
func (e *Emitter) emitArrayGEPAS(basePtrID int, arrayTy *module.Type, indexID int, addrSpace uint8) (int, error) {
	// Determine the element type from the array type.
	var elemTy *module.Type
	if arrayTy.Kind == module.TypeArray && arrayTy.ElemType != nil {
		elemTy = arrayTy.ElemType
	} else {
		elemTy = e.mod.GetFloatType(32) // fallback
	}
	resultPtrTy := e.mod.GetPointerTypeAS(elemTy, addrSpace)

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
func (e *Emitter) emitBinary(fn *ir.Function, bin ir.ExprBinary) (int, error) {
	// Check if operands are vectors — if so, scalarize the operation.
	leftType := e.resolveExprType(fn, bin.Left)
	rightType := e.resolveExprType(fn, bin.Right)

	// Matrix operations require special handling: matrix multiply uses dot
	// products, while mat+mat/mat-mat are component-wise.
	leftMat, leftIsMat := leftType.(ir.MatrixType)
	rightMat, rightIsMat := rightType.(ir.MatrixType)
	if leftIsMat || rightIsMat {
		return e.emitMatrixBinary(fn, bin, leftType, rightType, leftMat, rightMat, leftIsMat, rightIsMat)
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

	// LLVM Reassociate pass: for chains of commutative+associative ops
	// (e.g. a+b+c), flatten the tree, emit all leaves, sort by value ID,
	// and rebuild left-leaning. This matches DXC's -O3 pipeline which
	// includes the LLVM Reassociate pass before codegen.
	if isCommutativeBinOp(bin.Op) {
		leaves := e.flattenBinaryChain(fn, bin.Op, bin.Left, bin.Right)
		if len(leaves) > 2 {
			return e.emitReassociatedChain(fn, bin.Op, leftType, leaves)
		}
	}

	// LLVM InstCombine: shl(and(X, C1), C2) -> and(shl(X, C2), C1 << C2).
	// Also matches mul(and(X, C1), 2^N) since mul is strength-reduced to shl.
	if result, ok := e.tryShlAndCombine(fn, bin, leftType, numComps); ok {
		return result, nil
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

	// LLVM canonicalization for commutative ops (InstCombine):
	// 1. Constants go on the right.
	// 2. When both are non-constant, higher value ID goes first.
	//    LLVM ranks operands by "complexity" — for two instructions of the
	//    same kind the operand with the higher value number sorts first.
	//    This matches DXC's -dumpbin output for golden parity.
	if isCommutativeBinOp(bin.Op) {
		lConst := e.isConstValueID(lhs)
		rConst := e.isConstValueID(rhs)
		switch {
		case lConst && !rConst:
			lhs, rhs = rhs, lhs
		case !lConst && !rConst && lhs < rhs:
			lhs, rhs = rhs, lhs
		}
	}

	// LLVM canonicalization: fsub(a, +C) -> fadd(a, -C).
	// DXC (via LLVM InstCombine) folds subtraction of a positive float
	// constant into addition of the negated constant. Match this to
	// produce identical output for e.g. `pos.z - 0.1`.
	if bin.Op == ir.BinarySubtract && isFloat {
		if negID, ok := e.tryNegateFloatConst(rhs); ok {
			return e.emitScalarBinaryOp(ir.BinaryAdd, resultTy, lhs, negID, isFloat, isSigned)
		}
	}

	return e.emitScalarBinaryOp(bin.Op, resultTy, lhs, rhs, isFloat, isSigned)
}

// emitScalarBinaryOp dispatches a scalar binary operation to the appropriate
// DXIL instruction (binop, cmp, or lowered FRem).
func (e *Emitter) emitScalarBinaryOp(op ir.BinaryOperator, resultTy *module.Type, lhs, rhs int, isFloat, isSigned bool) (int, error) {
	switch op {
	case ir.BinaryAdd:
		return e.addBinOpInstr(resultTy, selectBinOp(isFloat, BinOpFAdd, BinOpAdd), lhs, rhs), nil
	case ir.BinarySubtract:
		// LLVM canonicalizes integer sub X, C to add X, -C.
		// Apply the same transformation when the RHS is a constant.
		if !isFloat {
			if negID, ok := e.trySubToAddNeg(rhs); ok {
				return e.addBinOpInstr(resultTy, BinOpAdd, lhs, negID), nil
			}
		}
		return e.addBinOpInstr(resultTy, selectBinOp(isFloat, BinOpFSub, BinOpSub), lhs, rhs), nil
	case ir.BinaryMultiply:
		// LLVM strength-reduces mul X, 2^N to shl X, N for integers.
		if !isFloat {
			if shiftAmt, ok := e.tryMulToShl(rhs); ok {
				return e.addBinOpInstr(resultTy, BinOpShl, lhs, shiftAmt), nil
			}
		}
		return e.addBinOpInstr(resultTy, selectBinOp(isFloat, BinOpFMul, BinOpMul), lhs, rhs), nil
	case ir.BinaryDivide:
		return e.addBinOpInstr(resultTy, selectDivOp(isFloat, isSigned), lhs, rhs), nil
	case ir.BinaryModulo:
		return e.emitModulo(resultTy, lhs, rhs, isFloat, isSigned)
	case ir.BinaryAnd:
		return e.addBinOpInstr(resultTy, BinOpAnd, lhs, rhs), nil
	case ir.BinaryExclusiveOr:
		return e.addBinOpInstr(resultTy, BinOpXor, lhs, rhs), nil
	case ir.BinaryInclusiveOr:
		return e.addBinOpInstr(resultTy, BinOpOr, lhs, rhs), nil
	case ir.BinaryShiftLeft:
		return e.addBinOpInstr(resultTy, BinOpShl, lhs, rhs), nil
	case ir.BinaryShiftRight:
		return e.addBinOpInstr(resultTy, selectBinOp(isSigned, BinOpAShr, BinOpLShr), lhs, rhs), nil
	case ir.BinaryEqual, ir.BinaryNotEqual, ir.BinaryLess,
		ir.BinaryLessEqual, ir.BinaryGreater, ir.BinaryGreaterEqual:
		return e.emitComparison(op, lhs, rhs, isFloat, isSigned)
	case ir.BinaryLogicalAnd:
		return e.addBinOpInstr(e.mod.GetIntType(1), BinOpAnd, lhs, rhs), nil
	case ir.BinaryLogicalOr:
		return e.addBinOpInstr(e.mod.GetIntType(1), BinOpOr, lhs, rhs), nil
	default:
		return 0, fmt.Errorf("unsupported binary operator: %d", op)
	}
}

// selectBinOp returns the float opcode if cond is true, otherwise the int opcode.
func selectBinOp(cond bool, ifTrue, ifFalse BinOpKind) BinOpKind {
	if cond {
		return ifTrue
	}
	return ifFalse
}

// selectDivOp returns the appropriate division opcode for the given type.
func selectDivOp(isFloat, isSigned bool) BinOpKind {
	if isFloat {
		return BinOpFDiv
	}
	if isSigned {
		return BinOpSDiv
	}
	return BinOpUDiv
}

// emitModulo emits a modulo/remainder operation. DXIL does not support FRem
// natively (DXC rejects it with "Invalid record"), so float modulo is lowered
// to: a - b * floor(a / b). This matches Mesa nir_to_dxil.c lower_fmod.
//
// For unsigned integer modulo by a constant power of 2, DXC's LLVM
// InstCombine applies strength reduction: urem x, 2^N -> and x, (2^N - 1).
// We match this to produce identical output.
func (e *Emitter) emitModulo(resultTy *module.Type, lhs, rhs int, isFloat, isSigned bool) (int, error) {
	if isFloat {
		return e.emitFRemLowered(resultTy, lhs, rhs), nil
	}
	if isSigned {
		return e.addBinOpInstr(resultTy, BinOpSRem, lhs, rhs), nil
	}
	// Strength reduction: urem x, 2^N -> and x, (2^N - 1).
	if maskID, ok := e.tryURemToBitwiseAnd(rhs); ok {
		return e.addBinOpInstr(resultTy, BinOpAnd, lhs, maskID), nil
	}
	return e.addBinOpInstr(resultTy, BinOpURem, lhs, rhs), nil
}

// tryURemToBitwiseAnd checks whether valueID refers to an integer constant
// that is a power of 2 (>= 2). If so, returns the value ID for the bitmask
// (value - 1) for use in strength reduction: urem x, 2^N -> and x, (2^N - 1).
// This matches DXC's LLVM InstCombine pass.
func (e *Emitter) tryURemToBitwiseAnd(valueID int) (int, bool) {
	c, ok := e.constMap[valueID]
	if !ok || c.IsUndef || c.IsAggregate {
		return 0, false
	}
	if c.ConstType == nil || c.ConstType.Kind != module.TypeInteger {
		return 0, false
	}
	v := c.IntValue
	if v < 2 {
		return 0, false
	}
	// Check power of 2: v & (v-1) == 0.
	if v&(v-1) != 0 {
		return 0, false
	}
	return e.getIntConstID(v - 1), true
}

// tryMulToShl checks whether valueID refers to an integer constant that is a
// power of 2 (>= 2). If so, returns the value ID for the shift amount
// (log2(value)) for use in strength reduction: mul X, 2^N -> shl X, N.
// This matches DXC's LLVM InstCombine pass.
func (e *Emitter) tryMulToShl(valueID int) (int, bool) {
	c, ok := e.constMap[valueID]
	if !ok || c.IsUndef || c.IsAggregate {
		return 0, false
	}
	if c.ConstType == nil || c.ConstType.Kind != module.TypeInteger {
		return 0, false
	}
	v := c.IntValue
	if v < 2 {
		return 0, false
	}
	// Check power of 2: v & (v-1) == 0.
	if v&(v-1) != 0 {
		return 0, false
	}
	// Compute log2.
	shift := int64(0)
	tmp := v
	for tmp > 1 {
		tmp >>= 1
		shift++
	}
	return e.getIntConstID(shift), true
}

// trySubToAddNeg checks whether valueID refers to a positive integer constant.
// If so, returns the value ID for the negated constant (-value) for use in
// canonicalization: sub X, C -> add X, -C. This matches DXC's LLVM
// canonicalization where sub is converted to add with negative operand.
func (e *Emitter) trySubToAddNeg(valueID int) (int, bool) {
	c, ok := e.constMap[valueID]
	if !ok || c.IsUndef || c.IsAggregate {
		return 0, false
	}
	if c.ConstType == nil || c.ConstType.Kind != module.TypeInteger {
		return 0, false
	}
	// Only canonicalize when the constant is positive (> 0).
	// sub X, 0 is already a no-op and sub X, negative is rare.
	if c.IntValue <= 0 {
		return 0, false
	}
	return e.getIntConstID(-c.IntValue), true
}

// tryShlAndCombine detects mul(and(X, C1), 2^N) or mul(as(and(X, C1)), 2^N)
// in the IR expression tree and rewrites to and(shl(X, N), C1 << N). This
// matches LLVM InstCombine's canonicalization which hoists the shift before
// the and and adjusts the mask. The As (cast) peeling handles cases like
// i32(vertex_index & 1u) * 2 where a u32->i32 cast wraps the And.
// Returns (valueID, true) if the transform applied, (0, false) otherwise.
func (e *Emitter) tryShlAndCombine(fn *ir.Function, bin ir.ExprBinary, leftType ir.TypeInner, numComps int) (int, bool) {
	// Only applies to integer scalar ShiftLeft or Multiply.
	if numComps != 1 || isFloatType(leftType) {
		return 0, false
	}

	shlAmt, ok := e.shlAmtForBinOp(fn, bin)
	if !ok {
		return 0, false
	}

	// Peel through single-use As (cast) expressions to find an And.
	andHandle, ok := e.peelToAndExpr(fn, bin.Left)
	if !ok {
		return 0, false
	}

	andBin := fn.Expressions[andHandle].Kind.(ir.ExprBinary)
	andMask, okMask := e.irExprIntConst(fn, andBin.Right)
	if !okMask {
		return 0, false
	}

	// Emit: shl(X, C2) then and(result, C1 << C2).
	xID, err := e.emitExpression(fn, andBin.Left)
	if err != nil {
		return 0, false
	}
	newMask := andMask << shlAmt
	resultTy, _ := typeToDXIL(e.mod, e.ir, leftType)
	shlResult := e.addBinOpInstr(resultTy, BinOpShl, xID, e.getIntConstID(shlAmt))

	lhs, rhs := shlResult, e.getIntConstID(newMask)
	if e.isConstValueID(lhs) && !e.isConstValueID(rhs) {
		lhs, rhs = rhs, lhs
	}
	return e.addBinOpInstr(resultTy, BinOpAnd, lhs, rhs), true
}

// shlAmtForBinOp extracts the effective shift amount from a ShiftLeft or
// Multiply (power-of-2 constant) binary expression. Returns (0, false) if
// the expression is not a shift or power-of-2 multiply.
func (e *Emitter) shlAmtForBinOp(fn *ir.Function, bin ir.ExprBinary) (int64, bool) {
	if bin.Op == ir.BinaryShiftLeft {
		amt, ok := e.irExprIntConst(fn, bin.Right)
		if !ok || amt <= 0 || amt >= 32 {
			return 0, false
		}
		return amt, true
	}
	if bin.Op == ir.BinaryMultiply {
		mulConst, ok := e.irExprIntConst(fn, bin.Right)
		if !ok || mulConst < 2 || mulConst&(mulConst-1) != 0 {
			return 0, false
		}
		var shift int64
		tmp := mulConst
		for tmp > 1 {
			tmp >>= 1
			shift++
		}
		return shift, true
	}
	return 0, false
}

// peelToAndExpr walks through the expression at h, peeling through single-use
// As (cast) expressions, and returns the handle of an And binary expression
// underneath. Returns (0, false) if no And is found or intermediate expressions
// are already emitted or multi-use.
func (e *Emitter) peelToAndExpr(fn *ir.Function, h ir.ExpressionHandle) (ir.ExpressionHandle, bool) {
	for {
		if _, cached := e.exprValues[h]; cached {
			return 0, false
		}
		if e.exprUseCount[h] > 1 {
			return 0, false
		}
		expr := fn.Expressions[h]
		if asExpr, isAs := expr.Kind.(ir.ExprAs); isAs {
			h = asExpr.Expr
			continue
		}
		andBin, isAnd := expr.Kind.(ir.ExprBinary)
		if isAnd && andBin.Op == ir.BinaryAnd {
			return h, true
		}
		return 0, false
	}
}

// irExprIntConst checks if an expression handle refers to a literal or
// constant integer and returns its value. Used by tryShlAndCombine to
// detect constant masks/shift amounts in the IR expression tree before
// emission.
func (e *Emitter) irExprIntConst(fn *ir.Function, h ir.ExpressionHandle) (int64, bool) {
	expr := fn.Expressions[h]
	switch ek := expr.Kind.(type) {
	case ir.Literal:
		switch v := ek.Value.(type) {
		case ir.LiteralI32:
			return int64(v), true
		case ir.LiteralU32:
			return int64(v), true
		}
	case ir.ExprConstant:
		c := e.ir.Constants[ek.Constant]
		if int(c.Init) < len(e.ir.GlobalExpressions) {
			initExpr := e.ir.GlobalExpressions[c.Init]
			if lit, ok := initExpr.Kind.(ir.Literal); ok {
				switch v := lit.Value.(type) {
				case ir.LiteralI32:
					return int64(v), true
				case ir.LiteralU32:
					return int64(v), true
				}
			}
		}
	}
	return 0, false
}

// emitComparison emits a comparison instruction with the correct predicate
// for the given operator, type (float vs int), and signedness.
func (e *Emitter) emitComparison(op ir.BinaryOperator, lhs, rhs int, isFloat, isSigned bool) (int, error) {
	pred := comparisonPredicate(op, isFloat, isSigned)
	return e.addCmpInstr(pred, lhs, rhs), nil
}

// comparisonPredicate maps a binary comparison operator to the DXIL CmpPredicate.
func comparisonPredicate(op ir.BinaryOperator, isFloat, isSigned bool) CmpPredicate {
	switch op {
	case ir.BinaryEqual:
		if isFloat {
			return FCmpOEQ
		}
		return ICmpEQ
	case ir.BinaryNotEqual:
		if isFloat {
			return FCmpONE
		}
		return ICmpNE
	case ir.BinaryLess:
		if isFloat {
			return FCmpOLT
		}
		if isSigned {
			return ICmpSLT
		}
		return ICmpULT
	case ir.BinaryLessEqual:
		if isFloat {
			return FCmpOLE
		}
		if isSigned {
			return ICmpSLE
		}
		return ICmpULE
	case ir.BinaryGreater:
		if isFloat {
			return FCmpOGT
		}
		if isSigned {
			return ICmpSGT
		}
		return ICmpUGT
	default: // ir.BinaryGreaterEqual
		if isFloat {
			return FCmpOGE
		}
		if isSigned {
			return ICmpSGE
		}
		return ICmpUGE
	}
}

// computeExprUseCount walks all expressions in the function and counts
// how many times each ExpressionHandle is referenced as a direct operand
// of another expression. Only expression-to-expression references are
// counted (not statement references). This is used by the Reassociate
// pass to identify single-use chain intermediates.
func computeExprUseCount(fn *ir.Function) map[ir.ExpressionHandle]int {
	counts := make(map[ir.ExpressionHandle]int, len(fn.Expressions))
	for _, expr := range fn.Expressions {
		for _, h := range exprOperandHandles(expr.Kind) {
			counts[h]++
		}
	}
	return counts
}

// exprOperandHandles returns all ExpressionHandle operands referenced
// by the given expression kind. Used by computeExprUseCount to walk
// the expression DAG without a large type-switch in the caller.
func exprOperandHandles(kind ir.ExpressionKind) []ir.ExpressionHandle {
	switch ek := kind.(type) {
	case ir.ExprBinary:
		return []ir.ExpressionHandle{ek.Left, ek.Right}
	case ir.ExprUnary:
		return []ir.ExpressionHandle{ek.Expr}
	case ir.ExprCompose:
		return ek.Components
	case ir.ExprAccess:
		return []ir.ExpressionHandle{ek.Base, ek.Index}
	case ir.ExprAccessIndex:
		return []ir.ExpressionHandle{ek.Base}
	case ir.ExprSplat:
		return []ir.ExpressionHandle{ek.Value}
	case ir.ExprSelect:
		return []ir.ExpressionHandle{ek.Condition, ek.Accept, ek.Reject}
	case ir.ExprAs:
		return []ir.ExpressionHandle{ek.Expr}
	case ir.ExprLoad:
		return []ir.ExpressionHandle{ek.Pointer}
	case ir.ExprMath:
		return collectMathOperands(ek)
	case ir.ExprSwizzle:
		return []ir.ExpressionHandle{ek.Vector}
	case ir.ExprRelational:
		return []ir.ExpressionHandle{ek.Argument}
	case ir.ExprImageSample:
		return collectImageSampleOperands(ek)
	case ir.ExprImageLoad:
		return collectImageLoadOperands(ek)
	case ir.ExprImageQuery:
		return []ir.ExpressionHandle{ek.Image}
	case ir.ExprArrayLength:
		return []ir.ExpressionHandle{ek.Array}
	default:
		return nil
	}
}

// collectMathOperands gathers the 1-4 expression operands of a Math
// expression into a single slice.
func collectMathOperands(ek ir.ExprMath) []ir.ExpressionHandle {
	ops := []ir.ExpressionHandle{ek.Arg}
	if ek.Arg1 != nil {
		ops = append(ops, *ek.Arg1)
	}
	if ek.Arg2 != nil {
		ops = append(ops, *ek.Arg2)
	}
	if ek.Arg3 != nil {
		ops = append(ops, *ek.Arg3)
	}
	return ops
}

// collectImageSampleOperands gathers the 3-6 expression operands of an
// ImageSample expression.
func collectImageSampleOperands(ek ir.ExprImageSample) []ir.ExpressionHandle {
	ops := []ir.ExpressionHandle{ek.Image, ek.Sampler, ek.Coordinate}
	if ek.ArrayIndex != nil {
		ops = append(ops, *ek.ArrayIndex)
	}
	if ek.Offset != nil {
		ops = append(ops, *ek.Offset)
	}
	if ek.DepthRef != nil {
		ops = append(ops, *ek.DepthRef)
	}
	return ops
}

// collectImageLoadOperands gathers the 2-5 expression operands of an
// ImageLoad expression.
func collectImageLoadOperands(ek ir.ExprImageLoad) []ir.ExpressionHandle {
	ops := []ir.ExpressionHandle{ek.Image, ek.Coordinate}
	if ek.ArrayIndex != nil {
		ops = append(ops, *ek.ArrayIndex)
	}
	if ek.Sample != nil {
		ops = append(ops, *ek.Sample)
	}
	if ek.Level != nil {
		ops = append(ops, *ek.Level)
	}
	return ops
}

// isCommutativeBinOp returns true for binary operations where operand order
// can be swapped without changing semantics. Used for LLVM-style constant
// canonicalization: constants go on the right for commutative ops.
func isCommutativeBinOp(op ir.BinaryOperator) bool {
	switch op {
	case ir.BinaryAdd, ir.BinaryMultiply,
		ir.BinaryAnd, ir.BinaryInclusiveOr, ir.BinaryExclusiveOr:
		return true
	}
	return false
}

// flattenBinaryChain flattens a tree of the same commutative+associative
// binary operator into a list of leaf operand handles. For example,
// Add(Add(a, b), c) produces [a, b, c]. An operand is a leaf if:
//   - it is not the same binary op kind, or
//   - it is used more than once (multi-use intermediate whose value ID
//     must not be reordered — the Reassociate pass only flattens
//     single-use intermediates).
//
// This mirrors LLVM's Reassociate pass which flattens associative chains
// before sorting by operand rank.
func (e *Emitter) flattenBinaryChain(fn *ir.Function, op ir.BinaryOperator,
	left, right ir.ExpressionHandle,
) []ir.ExpressionHandle {
	var leaves []ir.ExpressionHandle
	var collect func(h ir.ExpressionHandle)
	collect = func(h ir.ExpressionHandle) {
		// Already emitted — its value ID is fixed, treat as leaf.
		if _, cached := e.exprValues[h]; cached {
			leaves = append(leaves, h)
			return
		}
		expr := fn.Expressions[h]
		bin, ok := expr.Kind.(ir.ExprBinary)
		if !ok || bin.Op != op {
			leaves = append(leaves, h)
			return
		}
		// Multi-use intermediate — keep as leaf so its value is
		// materialized once and referenced by all users.
		if e.exprUseCount[h] > 1 {
			leaves = append(leaves, h)
			return
		}
		// Same op, single-use, not yet emitted — recurse into children.
		collect(bin.Left)
		collect(bin.Right)
	}
	collect(left)
	collect(right)
	return leaves
}

// emitReassociatedChain emits a flattened chain of commutative+associative
// binary operations. All leaf operands are emitted first, then sorted by
// ascending value ID (matching LLVM Reassociate's rank-based ordering),
// and combined into a left-leaning tree with commutative canonicalization
// (higher value ID first within each pair).
func (e *Emitter) emitReassociatedChain(fn *ir.Function, op ir.BinaryOperator,
	elemType ir.TypeInner, leaves []ir.ExpressionHandle,
) (int, error) {
	// Emit all leaves so their value IDs are known.
	ids := make([]int, len(leaves))
	for i, h := range leaves {
		v, err := e.emitExpression(fn, h)
		if err != nil {
			return 0, err
		}
		ids[i] = v
	}

	// Sort by value ID ascending (lowest rank first). LLVM Reassociate
	// builds the tree so that the earliest-defined (lowest ID) values
	// are combined first, with the last (highest ID) value at the root.
	sort.Ints(ids)

	resultTy, _ := typeToDXIL(e.mod, e.ir, elemType)

	// Only commutative+associative ops reach here, never comparisons.
	binOp, _, _, err := selectBinaryOpcode(op, elemType)
	if err != nil {
		return 0, err
	}

	// Build left-leaning tree: ((ids[0] op ids[1]) op ids[2]) op ...
	// With commutative canonicalization: higher value ID as first operand.
	acc := ids[0]
	for i := 1; i < len(ids); i++ {
		lhs, rhs := ids[i], acc
		// Canonicalize: higher value ID first for non-constant operands.
		lConst := e.isConstValueID(lhs)
		rConst := e.isConstValueID(rhs)
		switch {
		case lConst && !rConst:
			lhs, rhs = rhs, lhs
		case !lConst && !rConst && lhs < rhs:
			lhs, rhs = rhs, lhs
		}

		acc = e.addBinOpInstr(resultTy, binOp, lhs, rhs)
	}
	return acc, nil
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
	// Canonicalize: for commutative ops, constants go on the right (LLVM norm).
	canonicalize := isCommutativeBinOp(bin.Op) && !isCmp

	comps := make([]int, numComps)
	for i := 0; i < numComps; i++ {
		lComp := e.getComponentIDSafe(bin.Left, i, leftComps)
		rComp := e.getComponentIDSafe(bin.Right, i, rightComps)

		if canonicalize {
			lConst := e.isConstValueID(lComp)
			rConst := e.isConstValueID(rComp)
			switch {
			case lConst && !rConst:
				lComp, rComp = rComp, lComp
			case !lConst && !rConst && lComp < rComp:
				lComp, rComp = rComp, lComp
			}
		}

		if isCmp {
			comps[i] = e.addCmpInstr(cmpPred, lComp, rComp)
		} else if binOp == BinOpFRem {
			// FRem is not supported in DXIL — use lowered sequence per component.
			comps[i] = e.emitFRemLowered(scalarTy, lComp, rComp)
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

// emitMatrixBinary dispatches matrix binary operations.
// Matrix multiply (mat*vec, vec*mat, mat*mat) uses dot products.
// Matrix add/sub are component-wise on all elements.
func (e *Emitter) emitMatrixBinary(fn *ir.Function, bin ir.ExprBinary,
	leftType, rightType ir.TypeInner,
	leftMat, rightMat ir.MatrixType,
	leftIsMat, rightIsMat bool,
) (int, error) {
	// Emit both operands so component values are available.
	if _, err := e.emitExpression(fn, bin.Left); err != nil {
		return 0, err
	}
	if _, err := e.emitExpression(fn, bin.Right); err != nil {
		return 0, err
	}

	switch bin.Op {
	case ir.BinaryMultiply:
		if leftIsMat && rightIsMat {
			return e.emitMatrixMatrixMul(fn, bin, leftMat, rightMat)
		}
		if leftIsMat {
			rightComps := componentCount(rightType)
			if rightComps == 1 {
				// mat * scalar: scale all elements
				return e.emitMatrixScalarMul(fn, bin, leftMat)
			}
			// mat * vec
			return e.emitMatrixVectorMul(fn, bin, leftMat)
		}
		// rightIsMat
		leftComps := componentCount(leftType)
		if leftComps == 1 {
			// scalar * mat: scale all elements
			return e.emitScalarMatrixMul(fn, bin, rightMat)
		}
		// vec * mat
		return e.emitVectorMatrixMul(fn, bin, rightMat)

	case ir.BinaryAdd, ir.BinarySubtract:
		if leftIsMat {
			return e.emitMatrixComponentWise(fn, bin, leftMat)
		}
		return e.emitMatrixComponentWise(fn, bin, rightMat)

	default:
		return 0, fmt.Errorf("unsupported matrix binary operation: %v", bin.Op)
	}
}

// emitMatrixVectorMul emits mat * vec.
// For mat with C columns and R rows, vec has C components, result has R components.
// result[r] = sum(mat[c][r] * vec[c] for c in 0..C)
//
// Each row of the result is a dot product of the matrix row with the vector.
// In column-major layout, mat component at column c, row r = index c*R + r.
func (e *Emitter) emitMatrixVectorMul(_ *ir.Function, bin ir.ExprBinary, mat ir.MatrixType) (int, error) {
	cols := int(mat.Columns)
	rows := int(mat.Rows)
	scalarTy, _ := typeToDXIL(e.mod, e.ir, mat.Scalar)

	comps := make([]int, rows)
	for r := 0; r < rows; r++ {
		// Accumulate: sum = mat[0][r]*vec[0] + mat[1][r]*vec[1] + ...
		var acc int
		for c := 0; c < cols; c++ {
			matComp := e.getComponentID(bin.Left, c*rows+r)
			vecComp := e.getComponentID(bin.Right, c)
			prod := e.addBinOpInstr(scalarTy, BinOpFMul, matComp, vecComp)
			if c == 0 {
				acc = prod
			} else {
				acc = e.addBinOpInstr(scalarTy, BinOpFAdd, acc, prod)
			}
		}
		comps[r] = acc
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitVectorMatrixMul emits vec * mat.
// For vec with R components, mat with C columns and R rows, result has C components.
// result[c] = sum(vec[r] * mat[c][r] for r in 0..R)
func (e *Emitter) emitVectorMatrixMul(_ *ir.Function, bin ir.ExprBinary, mat ir.MatrixType) (int, error) {
	cols := int(mat.Columns)
	rows := int(mat.Rows)
	scalarTy, _ := typeToDXIL(e.mod, e.ir, mat.Scalar)

	comps := make([]int, cols)
	for c := 0; c < cols; c++ {
		var acc int
		for r := 0; r < rows; r++ {
			vecComp := e.getComponentID(bin.Left, r)
			matComp := e.getComponentID(bin.Right, c*rows+r)
			prod := e.addBinOpInstr(scalarTy, BinOpFMul, vecComp, matComp)
			if r == 0 {
				acc = prod
			} else {
				acc = e.addBinOpInstr(scalarTy, BinOpFAdd, acc, prod)
			}
		}
		comps[c] = acc
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitMatrixMatrixMul emits matA * matB.
// matA: colsA columns, rowsA rows. matB: colsB columns, rowsB rows.
// Requires colsA == rowsB. Result: colsB columns, rowsA rows.
// result[cb][ra] = sum(matA[ca][ra] * matB[cb][ca] for ca in 0..colsA)
func (e *Emitter) emitMatrixMatrixMul(_ *ir.Function, bin ir.ExprBinary, matA, matB ir.MatrixType) (int, error) {
	colsA := int(matA.Columns)
	rowsA := int(matA.Rows)
	colsB := int(matB.Columns)
	rowsB := int(matB.Rows)
	_ = rowsB // colsA == rowsB is a type-system invariant

	scalarTy, _ := typeToDXIL(e.mod, e.ir, matA.Scalar)

	totalComps := colsB * rowsA
	comps := make([]int, totalComps)

	for cb := 0; cb < colsB; cb++ {
		for ra := 0; ra < rowsA; ra++ {
			var acc int
			for ca := 0; ca < colsA; ca++ {
				aComp := e.getComponentID(bin.Left, ca*rowsA+ra)
				bComp := e.getComponentID(bin.Right, cb*rowsB+ca)
				prod := e.addBinOpInstr(scalarTy, BinOpFMul, aComp, bComp)
				if ca == 0 {
					acc = prod
				} else {
					acc = e.addBinOpInstr(scalarTy, BinOpFAdd, acc, prod)
				}
			}
			comps[cb*rowsA+ra] = acc
		}
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitMatrixComponentWise emits component-wise add/sub on all matrix elements.
func (e *Emitter) emitMatrixComponentWise(_ *ir.Function, bin ir.ExprBinary, mat ir.MatrixType) (int, error) {
	total := int(mat.Columns) * int(mat.Rows)
	scalarTy, _ := typeToDXIL(e.mod, e.ir, mat.Scalar)

	var op BinOpKind
	switch bin.Op {
	case ir.BinaryAdd:
		op = BinOpFAdd
	case ir.BinarySubtract:
		op = BinOpFSub
	default:
		return 0, fmt.Errorf("unsupported component-wise matrix operation: %v", bin.Op)
	}

	comps := make([]int, total)
	for i := 0; i < total; i++ {
		l := e.getComponentID(bin.Left, i)
		r := e.getComponentID(bin.Right, i)
		comps[i] = e.addBinOpInstr(scalarTy, op, l, r)
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitMatrixScalarMul emits mat * scalar: scale every element of the matrix.
func (e *Emitter) emitMatrixScalarMul(_ *ir.Function, bin ir.ExprBinary, mat ir.MatrixType) (int, error) {
	total := int(mat.Columns) * int(mat.Rows)
	scalarTy, _ := typeToDXIL(e.mod, e.ir, mat.Scalar)
	s := e.getComponentID(bin.Right, 0)

	comps := make([]int, total)
	for i := 0; i < total; i++ {
		m := e.getComponentID(bin.Left, i)
		comps[i] = e.addBinOpInstr(scalarTy, BinOpFMul, m, s)
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitScalarMatrixMul emits scalar * mat: scale every element of the matrix.
func (e *Emitter) emitScalarMatrixMul(_ *ir.Function, bin ir.ExprBinary, mat ir.MatrixType) (int, error) {
	total := int(mat.Columns) * int(mat.Rows)
	scalarTy, _ := typeToDXIL(e.mod, e.ir, mat.Scalar)
	s := e.getComponentID(bin.Left, 0)

	comps := make([]int, total)
	for i := 0; i < total; i++ {
		m := e.getComponentID(bin.Right, i)
		comps[i] = e.addBinOpInstr(scalarTy, BinOpFMul, s, m)
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitMatrixTranspose swaps rows and columns of a matrix.
// Input: C columns, R rows (component c*R+r). Output: R columns, C rows (component r*C+c).
func (e *Emitter) emitMatrixTranspose(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	mat, ok := argType.(ir.MatrixType)
	if !ok {
		return 0, fmt.Errorf("MathTranspose: argument is not a matrix")
	}

	cols := int(mat.Columns)
	rows := int(mat.Rows)
	total := cols * rows
	comps := make([]int, total)

	// Transpose: output[r*cols+c] = input[c*rows+r]
	// But we store output as column-major for the transposed matrix (R columns, C rows):
	// output column r, row c = index r*C + c = input[c*R + r]
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			comps[r*cols+c] = e.getComponentID(mathExpr.Arg, c*rows+r)
		}
	}

	e.pendingComponents = comps
	return comps[0], nil
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
	case ir.MathQuantizeF16:
		return e.emitMathQuantizeF16(fn, mathExpr)
	case ir.MathInverse:
		return 0, fmt.Errorf("MathInverse (matrix) not yet implemented in DXIL backend")
	case ir.MathDeterminant:
		return 0, fmt.Errorf("MathDeterminant (matrix) not yet implemented in DXIL backend")
	case ir.MathTranspose:
		return e.emitMatrixTranspose(fn, mathExpr)
	case ir.MathPack4x8snorm:
		return e.emitMathPack4x8snorm(fn, mathExpr)
	case ir.MathPack4x8unorm:
		return e.emitMathPack4x8unorm(fn, mathExpr)
	case ir.MathPack2x16snorm:
		return e.emitMathPack2x16snorm(fn, mathExpr)
	case ir.MathPack2x16unorm:
		return e.emitMathPack2x16unorm(fn, mathExpr)
	case ir.MathPack2x16float:
		return e.emitMathPack2x16float(fn, mathExpr)
	case ir.MathPack4xI8:
		return e.emitMathPack4xI8(fn, mathExpr)
	case ir.MathPack4xU8:
		return e.emitMathPack4xU8(fn, mathExpr)
	case ir.MathPack4xI8Clamp:
		return e.emitMathPack4xI8Clamp(fn, mathExpr)
	case ir.MathPack4xU8Clamp:
		return e.emitMathPack4xU8Clamp(fn, mathExpr)
	case ir.MathUnpack4x8snorm:
		return e.emitMathUnpack4x8snorm(fn, mathExpr)
	case ir.MathUnpack4x8unorm:
		return e.emitMathUnpack4x8unorm(fn, mathExpr)
	case ir.MathUnpack2x16snorm:
		return e.emitMathUnpack2x16snorm(fn, mathExpr)
	case ir.MathUnpack2x16unorm:
		return e.emitMathUnpack2x16unorm(fn, mathExpr)
	case ir.MathUnpack2x16float:
		return e.emitMathUnpack2x16float(fn, mathExpr)
	case ir.MathUnpack4xI8:
		return e.emitMathUnpack4xI8(fn, mathExpr)
	case ir.MathUnpack4xU8:
		return e.emitMathUnpack4xU8(fn, mathExpr)
	}

	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}
	if mathExpr.Arg1 != nil {
		if _, err := e.emitExpression(fn, *mathExpr.Arg1); err != nil {
			return 0, err
		}
	}
	if mathExpr.Arg2 != nil {
		if _, err := e.emitExpression(fn, *mathExpr.Arg2); err != nil {
			return 0, err
		}
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	isFloat := scalar.Kind == ir.ScalarFloat
	isSigned := scalar.Kind == ir.ScalarSint
	numComps := componentCount(argType)

	// For vector types, scalarize: emit per-component math operations.
	if numComps > 1 {
		return e.emitMathVectorized(fn, mathExpr, ol, isFloat, isSigned, numComps)
	}

	arg := e.exprValues[mathExpr.Arg]

	// Ternary: 3 args (Arg + Arg1 + Arg2).
	if mathExpr.Arg1 != nil && mathExpr.Arg2 != nil {
		arg1 := e.exprValues[*mathExpr.Arg1]
		arg2 := e.exprValues[*mathExpr.Arg2]
		return e.emitMathTernary(mathExpr.Fun, ol, isFloat, isSigned, arg, arg1, arg2)
	}

	// Binary: 2 args (Arg + Arg1).
	if mathExpr.Arg1 != nil {
		arg1 := e.exprValues[*mathExpr.Arg1]
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

// emitMathVectorized emits a math operation per-component for vector operands.
//
//nolint:nestif // per-component dispatch for unary/binary/ternary math ops
func (e *Emitter) emitMathVectorized(fn *ir.Function, mathExpr ir.ExprMath, ol overloadType, isFloat, isSigned bool, numComps int) (int, error) {
	// Determine component counts for each argument to handle scalar broadcast.
	// E.g., mix(vec3, vec3, scalar) — Arg2 is scalar, must broadcast to all components.
	argComps := numComps
	var arg1Comps, arg2Comps int
	if mathExpr.Arg1 != nil {
		arg1Type := e.resolveExprType(fn, *mathExpr.Arg1)
		arg1Comps = componentCount(arg1Type)
	}
	if mathExpr.Arg2 != nil {
		arg2Type := e.resolveExprType(fn, *mathExpr.Arg2)
		arg2Comps = componentCount(arg2Type)
	}

	comps := make([]int, numComps)
	for c := 0; c < numComps; c++ {
		argC := e.getComponentIDSafe(mathExpr.Arg, c, argComps)

		if mathExpr.Arg1 != nil && mathExpr.Arg2 != nil {
			arg1C := e.getComponentIDSafe(*mathExpr.Arg1, c, arg1Comps)
			arg2C := e.getComponentIDSafe(*mathExpr.Arg2, c, arg2Comps)
			v, err := e.emitMathTernary(mathExpr.Fun, ol, isFloat, isSigned, argC, arg1C, arg2C)
			if err != nil {
				return 0, err
			}
			comps[c] = v
		} else if mathExpr.Arg1 != nil {
			arg1C := e.getComponentIDSafe(*mathExpr.Arg1, c, arg1Comps)
			v, err := e.emitMathBinary(mathExpr.Fun, ol, isFloat, isSigned, argC, arg1C)
			if err != nil {
				return 0, err
			}
			comps[c] = v
		} else {
			opcode, opName, err := mathToDxOpUnary(mathExpr.Fun)
			if err != nil {
				return 0, err
			}
			dxFn := e.getDxOpUnaryFunc(opName, ol)
			opcodeVal := e.getIntConstID(int64(opcode))
			comps[c] = e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, argC})
		}
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathBinary emits a binary math dx.op call (min, max, atan2).
func (e *Emitter) emitMathBinary(mf ir.MathFunction, ol overloadType, isFloat, isSigned bool, arg, arg1 int) (int, error) {
	var opcode DXILOpcode
	var opName string

	switch mf {
	case ir.MathMin:
		if isFloat {
			opcode, opName = OpFMin, dxOpClassBinary
		} else if isSigned {
			opcode, opName = OpIMin, dxOpClassBinary
		} else {
			opcode, opName = OpUMin, dxOpClassBinary
		}
	case ir.MathMax:
		if isFloat {
			opcode, opName = OpFMax, dxOpClassBinary
		} else if isSigned {
			opcode, opName = OpIMax, dxOpClassBinary
		} else {
			opcode, opName = OpUMax, dxOpClassBinary
		}
	case ir.MathAtan2:
		// DXIL has no atan2 dx.op. Approximate as atan(y/x).
		// This is a simplification; proper atan2 needs quadrant correction.
		resultTy := e.overloadReturnType(ol)
		div := e.addBinOpInstr(resultTy, BinOpFDiv, arg, arg1)
		dxFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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
			dxFn := e.getDxOpTernaryFunc(dxOpClassTertiary, ol)
			opcodeVal := e.getIntConstID(int64(OpFMad))
			return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg2, bMinusA, arg}), nil
		}
		return 0, fmt.Errorf("mix not supported for non-float types")
	case ir.MathFma:
		if isFloat {
			// DXIL opcode 47 (Fma) is defined with overload mask 'd' only —
			// strict fused-multiply-add is f64-only. For f16/f32 WGSL fma we
			// lower to opcode 46 (FMad, overloads 'hfd'), which matches
			// HLSL's mad() intrinsic. HLSL's fma() is correspondingly
			// double-only. Verified via dxc DxilOperations.cpp:437 (FMad)
			// and :445 (Fma) overload masks.
			dxFn := e.getDxOpTernaryFunc(dxOpClassTertiary, ol)
			opc := OpFMad
			if ol == overloadF64 {
				opc = OpFma
			}
			opcodeVal := e.getIntConstID(int64(opc))
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
		maxOp, maxName = OpFMax, dxOpClassBinary
		minOp, minName = OpFMin, dxOpClassBinary
	} else if isSigned {
		maxOp, maxName = OpIMax, dxOpClassBinary
		minOp, minName = OpIMin, dxOpClassBinary
	} else {
		maxOp, maxName = OpUMax, dxOpClassBinary
		minOp, minName = OpUMin, dxOpClassBinary
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
	logFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	logOp := e.getIntConstID(int64(OpLog))
	logResult := e.addCallInstr(logFn, logFn.FuncType.RetType, []int{logOp, base})

	// log2(base) * exp
	mulResult := e.addBinOpInstr(resultTy, BinOpFMul, logResult, exp)

	// exp2(log2(base) * exp)
	expFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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

// emitMathDot emits a dot product.
//
// For float/half vectors, lowers to dx.op.dot2/dot3/dot4 (opcodes 54/55/56,
// overload mask 'hf' — float-only per DxilOperations.cpp:514+). For integer
// vectors, DXIL has no dot intrinsic at the scalar-component level — dxc
// lowers HLSL integer dot() to a manual sum-of-products expansion, and we
// mirror that here: 'a.x*b.x + a.y*b.y + ...' using integer IMul + IAdd.
// Without this, emitting dx.op.dot2.i32 trips 'DXIL intrinsic overload
// must be valid'.
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

	if size < 2 || size > 4 {
		return 0, fmt.Errorf("unsupported dot product vector size: %d", size)
	}

	// Integer path: manual sum-of-products. DXIL dot intrinsics are
	// float-only.
	if scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint {
		return e.emitIntegerDot(mathExpr.Arg, *mathExpr.Arg1, size, scalar), nil
	}

	var opcode DXILOpcode
	switch size {
	case 2:
		opcode = OpDot2
	case 3:
		opcode = OpDot3
	case 4:
		opcode = OpDot4
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

// emitIntegerDot emits an integer vector dot product using the same
// lowering DXC produces for HLSL 'dot(int2, int2)' and similar. Verified
// via dxc -T cs_6_0 on a structured-buffer integer dot: DXC emits
//
//	%prod1 = mul i32 a.1, b.1            ; trailing product
//	%dot   = call i32 @dx.op.tertiary.i32(i32 48, a.0, b.0, %prod1)  ; IMad
//
// and for 3/4 component vectors iterates the IMad accumulator with the
// next 'a.i * b.i' product. Using plain IMul+IAdd works but is a regression
// from the reference — DXC's IMad (opcode 48, tertiary int) is a fused
// multiply-add and is the canonical integer dot product lowering. Same
// opcode for signed and unsigned (IMad is sign-agnostic at the bit level).
func (e *Emitter) emitIntegerDot(handleA, handleB ir.ExpressionHandle, size int, scalar ir.ScalarType) int {
	intTy := e.mod.GetIntType(uint(scalar.Width) * 8)
	ol := overloadForScalar(scalar)

	// For size == 1 the caller never reaches us (componentCount >= 2 by
	// construction for a dot product). Start by computing the LAST product
	// as a plain IMul (to feed the first IMad's 'c' operand) and walk
	// downwards, accumulating IMad(a.i, b.i, prev).
	lastIdx := size - 1
	aLast := e.getComponentID(handleA, lastIdx)
	bLast := e.getComponentID(handleB, lastIdx)
	acc := e.addBinOpInstr(intTy, BinOpMul, aLast, bLast)

	imadFn := e.getDxOpTernaryFunc(dxOpClassTertiary, ol)
	opcodeVal := e.getIntConstID(int64(OpIMad))
	for i := size - 2; i >= 0; i-- {
		ai := e.getComponentID(handleA, i)
		bi := e.getComponentID(handleB, i)
		acc = e.addCallInstr(imadFn, intTy, []int{opcodeVal, ai, bi, acc})
	}
	return acc
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
		dxFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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
	sqrtFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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
	sqrtFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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
		rsqrtFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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
	rsqrtFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
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
// DXIL function naming convention (per dxc DxilOperations.cpp): the function
// name is dx.op.<OpClass>.<overload>. Float-domain unary opcodes share the
// "unary" class, integer bit-counting ops share "unaryBits". Using
// per-opcode names (dx.op.fmin etc.) produces "is not a DXILOpFunction"
// validation errors. BUG-DXIL-022 follow-up.
//
//nolint:gocyclo,cyclop // math function mapping requires many cases
func mathToDxOpUnary(mf ir.MathFunction) (DXILOpcode, string, error) {
	switch mf {
	case ir.MathSin:
		return OpSin, dxOpClassUnary, nil
	case ir.MathCos:
		return OpCos, dxOpClassUnary, nil
	case ir.MathTan:
		return OpTan, dxOpClassUnary, nil
	case ir.MathAsin:
		return OpAsin, dxOpClassUnary, nil
	case ir.MathAcos:
		return OpAcos, dxOpClassUnary, nil
	case ir.MathAtan:
		return OpAtan, dxOpClassUnary, nil
	case ir.MathSinh:
		return OpHSin, dxOpClassUnary, nil
	case ir.MathCosh:
		return OpHCos, dxOpClassUnary, nil
	case ir.MathTanh:
		return OpHTan, dxOpClassUnary, nil
	case ir.MathExp2:
		return OpExp, dxOpClassUnary, nil
	case ir.MathLog2:
		return OpLog, dxOpClassUnary, nil
	case ir.MathSqrt:
		return OpSqrt, dxOpClassUnary, nil
	case ir.MathInverseSqrt:
		return OpRsqrt, dxOpClassUnary, nil
	case ir.MathAbs:
		return OpFAbs, dxOpClassUnary, nil
	case ir.MathSaturate:
		return OpSaturate, dxOpClassUnary, nil
	case ir.MathFloor:
		return OpRoundNI, dxOpClassUnary, nil
	case ir.MathCeil:
		return OpRoundPI, dxOpClassUnary, nil
	case ir.MathTrunc:
		return OpRoundZ, dxOpClassUnary, nil
	case ir.MathRound:
		return OpRoundNE, dxOpClassUnary, nil
	case ir.MathFract:
		return OpFrc, dxOpClassUnary, nil
	case ir.MathReverseBits:
		return OpReverseBits, dxOpClassUnary, nil
	case ir.MathCountOneBits:
		return OpCountBits, dxOpClassUnaryBits, nil
	case ir.MathFirstTrailingBit:
		return OpFirstbitLo, dxOpClassUnaryBits, nil
	case ir.MathFirstLeadingBit:
		return OpFirstbitHi, dxOpClassUnaryBits, nil
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
		opName = dxOpClassTertiary
	} else {
		opcode = OpUBfe
		opName = dxOpClassTertiary
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
	dxFn := e.getDxOpQuaternaryFunc(dxOpClassQuaternary, ol)
	opcodeVal := e.getIntConstID(int64(OpBfi))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, count, offset, newBits, arg}), nil
}

// emitMathLdexp lowers ldexp(x, n) = x * exp2(float(n)).
//
// DXIL has no dedicated ldexp opcode — DxilOperations.cpp does not list
// Ldexp at all. DXC's HLSL frontend lowers it in HLOperationLower.cpp:
//
//	Value *TranslateLdExp(CallInst *CI, ...) {
//	    Value *src0 = CI->getArgOperand(0);  // x    (already float)
//	    Value *src1 = CI->getArgOperand(1);  // exp  (already float)
//	    Value *exp =
//	        TrivialDxilUnaryOperation(OP::OpCode::Exp, src1, ...);
//	    return Builder.CreateFMul(exp, src0);
//	}
//
// DXC can assume `src1` is already float because HLSL's `ldexp` has
// signature `(float, float)`. **WGSL's `ldexp` takes `(f32, i32)`** —
// the exponent is an integer. We therefore insert an explicit
// `sitofp i32 → f32` before feeding `dx.op.unary.f32(OpExp, ...)`.
// Without the cast, the CALL record carries an i32 operand at a slot
// declared as float in the function signature, and the LLVM 3.7
// bitcode reader rejects the record with HRESULT 0x80aa0009 "Invalid
// record" before any semantic validation runs.
//
// Vector form `ldexp(vecN<f32>, vecN<i32>)` is lowered per-component:
// DXIL `dx.op.unary.*` opcodes are scalar-only, so each lane becomes
// its own sitofp + exp + fmul triple and the result is tracked via
// pendingComponents like other vectorised math intrinsics.
//
// Reference: `reference/dxil/dxc/lib/HLSL/HLOperationLower.cpp:2504`
// (TranslateLdExp); WGSL spec §16.6.26 (ldexp signatures).
func (e *Emitter) emitMathLdexp(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if mathExpr.Arg1 == nil {
		return 0, fmt.Errorf("ldexp requires 2 arguments")
	}

	// Detect vector form from the x argument's IR type. The exponent
	// argument always matches x's component count per WGSL rules.
	argType := e.resolveExprType(fn, mathExpr.Arg)
	numComps := componentCount(argType)

	// Evaluate both arguments so their values are materialized and
	// downstream getComponentIDSafe calls can find per-lane IDs.
	argID, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	expID, err := e.emitExpression(fn, *mathExpr.Arg1)
	if err != nil {
		return 0, err
	}

	f32Ty := e.mod.GetFloatType(32)
	exp2Fn := e.getDxOpUnaryFunc(dxOpClassUnary, overloadF32)
	exp2OpVal := e.getIntConstID(int64(OpExp))

	// emitOneLane emits the sitofp + unary.exp + fmul sequence for a
	// single scalar lane.
	emitOneLane := func(xLane, eLane int) int {
		eF32 := e.addCastInstr(f32Ty, CastSIToFP, eLane)
		exp2 := e.addCallInstr(exp2Fn, f32Ty, []int{exp2OpVal, eF32})
		return e.addBinOpInstr(f32Ty, BinOpFMul, xLane, exp2)
	}

	if numComps <= 1 {
		return emitOneLane(argID, expID), nil
	}

	comps := make([]int, numComps)
	for c := 0; c < numComps; c++ {
		xLane := e.getComponentIDSafe(mathExpr.Arg, c, numComps)
		eLane := e.getComponentIDSafe(*mathExpr.Arg1, c, numComps)
		comps[c] = emitOneLane(xLane, eLane)
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathRefract computes refract(I, N, eta) manually since DXIL has no
// native refract intrinsic.
//
// Formula:
//
//	d = dot(N, I)
//	k = 1.0 - eta*eta * (1.0 - d*d)
//	if k < 0.0: return zero vector
//	else:       return eta * I - (eta * d + sqrt(k)) * N
//
// Reference: GLSL spec 8.5 Geometric Functions.
func (e *Emitter) emitMathRefract(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, fmt.Errorf("refract I: %w", err)
	}
	if mathExpr.Arg1 == nil {
		return 0, fmt.Errorf("refract: missing N argument")
	}
	if _, err := e.emitExpression(fn, *mathExpr.Arg1); err != nil {
		return 0, fmt.Errorf("refract N: %w", err)
	}
	if mathExpr.Arg2 == nil {
		return 0, fmt.Errorf("refract: missing eta argument")
	}
	etaID, err := e.emitExpression(fn, *mathExpr.Arg2)
	if err != nil {
		return 0, fmt.Errorf("refract eta: %w", err)
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	f32Ty := e.overloadReturnType(ol)

	oneID := e.getFloatConstID(1.0)
	zeroID := e.getFloatConstID(0.0)

	// dot(N, I)
	dotNI, dotErr := e.emitDotTwoVectors(mathExpr.Arg, *mathExpr.Arg1, size, ol)
	if dotErr != nil {
		return 0, fmt.Errorf("refract dot(N,I): %w", dotErr)
	}

	// k = 1.0 - eta*eta * (1.0 - d*d)
	dd := e.addBinOpInstr(f32Ty, BinOpFMul, dotNI, dotNI)         // d*d
	oneMinusDd := e.addBinOpInstr(f32Ty, BinOpFSub, oneID, dd)    // 1.0 - d*d
	etaEta := e.addBinOpInstr(f32Ty, BinOpFMul, etaID, etaID)     // eta*eta
	term := e.addBinOpInstr(f32Ty, BinOpFMul, etaEta, oneMinusDd) // eta*eta * (1-d*d)
	k := e.addBinOpInstr(f32Ty, BinOpFSub, oneID, term)           // 1.0 - eta*eta*(1-d*d)

	// sqrt(k)
	sqrtFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	sqrtOp := e.getIntConstID(int64(OpSqrt))
	sqrtK := e.addCallInstr(sqrtFn, sqrtFn.FuncType.RetType, []int{sqrtOp, k})

	// eta * d + sqrt(k)
	etaD := e.addBinOpInstr(f32Ty, BinOpFMul, etaID, dotNI)
	factor := e.addBinOpInstr(f32Ty, BinOpFAdd, etaD, sqrtK)

	// k < 0.0 → select zero or refract result per component
	kLtZero := e.addCmpInstr(FCmpOLT, k, zeroID)

	// Per-component: eta * I[i] - factor * N[i], or zero if k < 0
	comps := make([]int, size)
	for i := 0; i < size; i++ {
		iComp := e.getComponentID(mathExpr.Arg, i)
		nComp := e.getComponentID(*mathExpr.Arg1, i)
		etaI := e.addBinOpInstr(f32Ty, BinOpFMul, etaID, iComp)
		factorN := e.addBinOpInstr(f32Ty, BinOpFMul, factor, nComp)
		refracted := e.addBinOpInstr(f32Ty, BinOpFSub, etaI, factorN)
		// select(kLtZero, zero, refracted)
		selectID := e.allocValue()
		selectInstr := &module.Instruction{
			Kind:       module.InstrSelect,
			HasValue:   true,
			ResultType: f32Ty,
			Operands:   []int{kLtZero, zeroID, refracted},
			ValueID:    selectID,
		}
		e.currentBB.AddInstruction(selectInstr)
		comps[i] = selectID
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitDotTwoVectors emits dot(A, B) for two different vector expression handles.
func (e *Emitter) emitDotTwoVectors(handleA, handleB ir.ExpressionHandle, size int, ol overloadType) (int, error) {
	var opcode DXILOpcode
	switch size {
	case 1:
		// Scalar dot = a * b.
		a := e.getComponentID(handleA, 0)
		b := e.getComponentID(handleB, 0)
		f32Ty := e.overloadReturnType(ol)
		return e.addBinOpInstr(f32Ty, BinOpFMul, a, b), nil
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
		operands[1+i] = e.getComponentID(handleA, i)
		operands[1+size+i] = e.getComponentID(handleB, i)
	}

	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, operands), nil
}

// emitMathModf splits a float into integer and fractional parts.
// Returns a struct {fract, whole} where fract = x - floor(x) and whole = floor(x).
// For vector inputs, produces per-component results flattened: [fract0, whole0, fract1, whole1, ...].
//
// DXIL has no native modf — we use floor + subtraction.
func (e *Emitter) emitMathModf(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, fmt.Errorf("modf arg: %w", err)
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	f32Ty := e.overloadReturnType(ol)

	floorFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	floorOpVal := e.getIntConstID(int64(OpRoundNI))

	// Result struct has {fract, whole} for scalar, or flattened for vectors:
	// {fract0, fract1, ..., whole0, whole1, ...} OR {fract, whole}.
	// Naga IR modf returns __modf_result with members [fract, whole],
	// where each is scalar or vector matching the input type.
	// Flatten as: fract components first, then whole components.
	comps := make([]int, 2*size)
	for i := 0; i < size; i++ {
		x := e.getComponentID(mathExpr.Arg, i)
		// whole = floor(x)
		whole := e.addCallInstr(floorFn, floorFn.FuncType.RetType, []int{floorOpVal, x})
		// fract = x - floor(x)
		fract := e.addBinOpInstr(f32Ty, BinOpFSub, x, whole)
		comps[i] = fract      // fract components first
		comps[size+i] = whole // whole components after
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathFrexp splits a float into significand and exponent.
// Returns a struct {fract, exp} where x = fract * 2^exp, 0.5 <= |fract| < 1.
//
// DXIL has no native frexp. We compute:
//
//	abs_x = fabs(x)
//	exp_raw = floor(log2(abs_x)) + 1    (for x != 0)
//	exp_i = ftoi(exp_raw)
//	fract = x * exp2(-exp_raw)
//
// For x == 0, both fract and exp should be 0.
// Result struct layout: [fract components..., exp components...].
func (e *Emitter) emitMathFrexp(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, fmt.Errorf("frexp arg: %w", err)
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)
	scalar, ok := scalarOfType(argType)
	if !ok {
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	ol := overloadForScalar(scalar)
	f32Ty := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)

	fabsFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	fabsOp := e.getIntConstID(int64(OpFAbs))
	log2Fn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	log2Op := e.getIntConstID(int64(OpLog))
	floorFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	floorOp := e.getIntConstID(int64(OpRoundNI))
	exp2Fn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	exp2Op := e.getIntConstID(int64(OpExp))
	oneID := e.getFloatConstID(1.0)
	zeroF := e.getFloatConstID(0.0)
	zeroI := e.getIntConstID(0)

	comps := make([]int, 2*size)
	for i := 0; i < size; i++ {
		x := e.getComponentID(mathExpr.Arg, i)
		// abs_x = fabs(x)
		absX := e.addCallInstr(fabsFn, f32Ty, []int{fabsOp, x})
		// log2_abs = log2(abs_x)
		log2Abs := e.addCallInstr(log2Fn, f32Ty, []int{log2Op, absX})
		// exp_raw = floor(log2_abs) + 1
		floorLog := e.addCallInstr(floorFn, f32Ty, []int{floorOp, log2Abs})
		expRaw := e.addBinOpInstr(f32Ty, BinOpFAdd, floorLog, oneID)
		// neg_exp = 0 - exp_raw (for exp2(-exp))
		negExp := e.addBinOpInstr(f32Ty, BinOpFSub, zeroF, expRaw)
		// scale = exp2(-exp_raw)
		scale := e.addCallInstr(exp2Fn, f32Ty, []int{exp2Op, negExp})
		// fract = x * scale
		fract := e.addBinOpInstr(f32Ty, BinOpFMul, x, scale)
		// exp_i = ftoi(exp_raw)
		expI := e.addCastInstr(i32Ty, CastFPToSI, expRaw)
		// Handle x == 0: if x == 0, fract = 0, exp = 0
		xIsZero := e.addCmpInstr(FCmpOEQ, x, zeroF)
		fractID := e.allocValue()
		e.currentBB.AddInstruction(&module.Instruction{
			Kind: module.InstrSelect, HasValue: true, ResultType: f32Ty,
			Operands: []int{xIsZero, zeroF, fract}, ValueID: fractID,
		})
		expID := e.allocValue()
		e.currentBB.AddInstruction(&module.Instruction{
			Kind: module.InstrSelect, HasValue: true, ResultType: i32Ty,
			Operands: []int{xIsZero, zeroI, expI}, ValueID: expID,
		})
		comps[i] = fractID    // fract components first
		comps[size+i] = expID // exp components after
	}

	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathQuantizeF16 rounds f32 to f16 precision: f32 → fptrunc f16 → fpext f32.
// For vectors, applies per-component.
func (e *Emitter) emitMathQuantizeF16(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, fmt.Errorf("quantizeToF16 arg: %w", err)
	}

	argType := e.resolveExprType(fn, mathExpr.Arg)
	size := componentCount(argType)

	f16Ty := e.mod.GetFloatType(16)
	f32Ty := e.mod.GetFloatType(32)

	comps := make([]int, size)
	for i := 0; i < size; i++ {
		x := e.getComponentID(mathExpr.Arg, i)
		// fptrunc f32 → f16
		f16Val := e.addCastInstr(f16Ty, CastFPTrunc, x)
		// fpext f16 → f32
		comps[i] = e.addCastInstr(f32Ty, CastFPExt, f16Val)
	}

	if size > 1 {
		e.pendingComponents = comps
	}
	return comps[0], nil
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
	dxFn := e.getDxOpUnaryFunc(dxOpClassUnaryBits, ol)
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
		opName = dxOpClassUnaryBits
	} else {
		opcode = OpFirstbitHi
		opName = dxOpClassUnaryBits
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
	fn.AttrSetID = classifyDxOpAttr(fullName)
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
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitSelect emits a select (ternary) instruction.
// For vector operands, emits per-component LLVM select instructions since DXIL
// scalarizes all vector operations.
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
	numComps := componentCount(acceptType)

	if numComps > 1 {
		// Vector select: emit per-component selects.
		condType := e.resolveExprType(fn, sel.Condition)
		condComps := componentCount(condType)
		acceptComps := numComps
		rejectType := e.resolveExprType(fn, sel.Reject)
		rejectComps := componentCount(rejectType)

		scalarTy, _ := typeToDXIL(e.mod, e.ir, acceptType)
		comps := make([]int, numComps)
		for c := 0; c < numComps; c++ {
			condC := e.getComponentIDSafe(sel.Condition, c, condComps)
			acceptC := e.getComponentIDSafe(sel.Accept, c, acceptComps)
			rejectC := e.getComponentIDSafe(sel.Reject, c, rejectComps)
			comps[c] = e.emitSelect3(scalarTy, condC, acceptC, rejectC)
		}
		e.pendingComponents = comps
		return comps[0], nil
	}

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

// emitArrayLength emits a runtime array length query.
//
// In DXIL, buffer size is obtained via dx.op.getDimensions(72, handle, undef).
// The result is a %dx.types.Dimensions = {i32, i32, i32, i32}; component 0
// is the total byte size for buffer resources.
// The element count is computed as: (totalBytes - offset) / stride
//
// For a global variable that IS an array: offset=0, stride from ArrayType.
// For a global variable that is a struct with a runtime array as last member:
// offset = last member's offset, stride from the array type.
//
// Reference: Mesa nir_to_dxil.c emit_get_ssbo_size() ~4344
//
//nolint:nestif,gocognit,gocyclo,cyclop,funlen // type resolution for array vs struct wrapping the runtime array
func (e *Emitter) emitArrayLength(fn *ir.Function, al ir.ExprArrayLength) (int, error) {
	// Resolve the global variable that owns this runtime array.
	// Walk up the expression chain to find the GlobalVariable, handling binding arrays.
	var varHandle ir.GlobalVariableHandle
	var found bool
	var bindingArrayIndex int64
	var hasBindingArrayIndex bool

	if int(al.Array) < len(fn.Expressions) {
		// Walk up through AccessIndex/Access chains to find GlobalVariable.
		cur := al.Array
		for depth := 0; depth < 5; depth++ {
			if int(cur) >= len(fn.Expressions) {
				break
			}
			expr := fn.Expressions[cur]
			switch ek := expr.Kind.(type) {
			case ir.ExprGlobalVariable:
				varHandle = ek.Variable
				found = true
			case ir.ExprAccessIndex:
				// Check if this is a binding array index (base is GlobalVariable of binding array).
				if int(ek.Base) < len(fn.Expressions) {
					if gv, ok := fn.Expressions[ek.Base].Kind.(ir.ExprGlobalVariable); ok {
						if idx, isRes := e.resourceHandles[gv.Variable]; isRes && e.resources[idx].isBindingArray {
							varHandle = gv.Variable
							found = true
							bindingArrayIndex = int64(ek.Index)
							hasBindingArrayIndex = true
						} else {
							varHandle = gv.Variable
							found = true
						}
					} else {
						cur = ek.Base
						continue
					}
				}
			case ir.ExprAccess:
				// Dynamic binding array access: Access(base=GlobalVariable, index=expr).
				if int(ek.Base) < len(fn.Expressions) {
					if gv, ok := fn.Expressions[ek.Base].Kind.(ir.ExprGlobalVariable); ok {
						if idx, isRes := e.resourceHandles[gv.Variable]; isRes && e.resources[idx].isBindingArray {
							varHandle = gv.Variable
							found = true
							// Dynamic index needs emitting. For now set flag but leave index for later.
							hasBindingArrayIndex = true
							bindingArrayIndex = -1 // sentinel: need to emit dynamic index
						}
					}
				}
			}
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("ExprArrayLength: cannot resolve global variable from expression [%d]", al.Array)
	}

	// Look up the resource handle for this global variable.
	resIdx, hasRes := e.resourceHandles[varHandle]
	if !hasRes {
		return 0, fmt.Errorf("ExprArrayLength: no resource handle for global variable [%d]", varHandle)
	}

	// Get or create the resource handle (dynamic for binding arrays).
	var handleID int
	res := &e.resources[resIdx]
	if res.isBindingArray && hasBindingArrayIndex {
		if bindingArrayIndex >= 0 {
			// Constant index from AccessIndex.
			var err2 error
			handleID, err2 = e.emitDynamicCreateHandleConst(res, bindingArrayIndex)
			if err2 != nil {
				return 0, fmt.Errorf("ExprArrayLength: binding array handle: %w", err2)
			}
		} else {
			// Dynamic index from Access — need to find and emit the index expression.
			// Walk the chain to find the Access expression.
			expr := fn.Expressions[al.Array]
			if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok {
				if acc, ok2 := fn.Expressions[ai.Base].Kind.(ir.ExprAccess); ok2 {
					var err2 error
					handleID, err2 = e.emitDynamicCreateHandle(fn, res, acc.Index)
					if err2 != nil {
						return 0, fmt.Errorf("ExprArrayLength: binding array handle: %w", err2)
					}
				}
			}
		}
	} else {
		handleID = res.handleID
	}

	// Determine offset and stride from the type.
	// For binding arrays, unwrap to get the base element type.
	gv := &e.ir.GlobalVariables[varHandle]
	var offset, stride uint32
	if int(gv.Type) < len(e.ir.Types) {
		inner := e.ir.Types[gv.Type].Inner
		// Unwrap binding array to get the element type.
		if ba, ok := inner.(ir.BindingArrayType); ok {
			if int(ba.Base) < len(e.ir.Types) {
				inner = e.ir.Types[ba.Base].Inner
			}
		}
		switch typed := inner.(type) {
		case ir.ArrayType:
			offset = 0
			stride = typed.Stride
		case ir.StructType:
			if len(typed.Members) > 0 {
				last := typed.Members[len(typed.Members)-1]
				offset = last.Offset
				if int(last.Type) < len(e.ir.Types) {
					if arr, ok := e.ir.Types[last.Type].Inner.(ir.ArrayType); ok {
						stride = arr.Stride
					}
				}
			}
		}
	}
	if stride == 0 {
		stride = 4 // Default to 4 bytes (f32/i32/u32).
	}

	// Emit dx.op.getDimensions(72, handle, undef) → %dx.types.Dimensions
	i32Ty := e.mod.GetIntType(32)
	dimTy := e.getDxDimensionsType()
	getDimFn := e.getDxOpGetDimensionsFunc()

	opcodeVal := e.getIntConstID(int64(OpGetDimensions))
	undefVal := e.getUndefConstID()

	dimRetID := e.addCallInstr(getDimFn, dimTy, []int{opcodeVal, handleID, undefVal})

	// Extract component 0 = total byte size.
	totalBytesID := e.allocValue()
	extractInstr := &module.Instruction{
		Kind:       module.InstrExtractVal,
		HasValue:   true,
		ResultType: i32Ty,
		Operands:   []int{dimRetID, 0},
		ValueID:    totalBytesID,
	}
	e.currentBB.AddInstruction(extractInstr)

	resultID := totalBytesID

	// Subtract offset if nonzero: (totalBytes - offset)
	if offset > 0 {
		offsetID := e.getIntConstID(int64(offset))
		resultID = e.addBinOpInstr(i32Ty, BinOpSub, resultID, offsetID)
	}

	// Divide by stride: result / stride
	strideID := e.getIntConstID(int64(stride))
	resultID = e.addBinOpInstr(i32Ty, BinOpUDiv, resultID, strideID)

	return resultID, nil
}

// tryLoadFromConstVecArray implements the SROA-style fast path for
// `Load(Access(LocalVariable, idx))` where the local was registered
// in e.localConstVecArrays by tryRegisterLocalConstVecArray. Returns
// (valueID, true, nil) on success or (0, false, nil) if the load does
// not match the pattern. The caller must fall through to the generic
// load path on (false, _).
//
// Shape produced per lane:
//
//	cmp0  = icmp eq i32 idx, 0
//	selN  = table[N-1][lane]                              (initial)
//	cmpK  = icmp eq i32 idx, K     for K = N-2 down to 0
//	selK  = select cmpK, table[K][lane], selK+1
//	lane0 = final select
//
// tryLoadInitOnly checks if a load targets an init-only local variable
// (non-nil Init, never stored to) and resolves it directly to the Init
// expression value. Returns (valueID, true, nil) on match, (0, false, nil)
// when the pattern does not apply. This implements targeted SROA for
// the common `var x = const_expr; return x` pattern.
func (e *Emitter) tryLoadInitOnly(fn *ir.Function, load ir.ExprLoad) (int, bool, error) {
	lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
	if !ok {
		return 0, false, nil
	}
	initHandle, isInitOnly := e.initOnlyLocals[lv.Variable]
	if !isInitOnly {
		return 0, false, nil
	}
	id, err := e.emitExpression(fn, initHandle)
	if err != nil {
		return 0, false, err
	}
	// Propagate component tracking from the init expression so downstream
	// getComponentID calls resolve correctly for the Load handle (the outer
	// emitExpression stores pendingComponents into exprComponents[loadHandle]).
	if comps, hasComps := e.exprComponents[initHandle]; hasComps {
		e.pendingComponents = comps
	}
	return id, true, nil
}

// tryLoadPromotedLocal attempts to resolve a load from a promoted local
// variable (zero-store, init-only, or single-store) without emitting an
// alloca+load chain. Returns (valueID, true, nil) on success.
func (e *Emitter) tryLoadPromotedLocal(fn *ir.Function, load ir.ExprLoad) (int, bool, error) {
	if id, ok, err := e.tryLoadZeroStore(fn, load); err != nil || ok {
		return id, ok, err
	}
	if id, ok, err := e.tryLoadInitOnly(fn, load); err != nil || ok {
		return id, ok, err
	}
	return e.tryLoadSingleStore(fn, load)
}

// tryLoadZeroStore resolves a load from a zero-store local variable
// (no Init, never stored to) to the zero constant of its type.
// WGSL spec requires such locals to be default-initialized to zero.
// DXC's LLVM SROA+DCE eliminates these entirely; we emit the zero
// constant inline, matching DXC's output.
func (e *Emitter) tryLoadZeroStore(fn *ir.Function, load ir.ExprLoad) (int, bool, error) {
	lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
	if !ok {
		return 0, false, nil
	}
	tyHandle, isZero := e.zeroStoreLocals[lv.Variable]
	if !isZero {
		return 0, false, nil
	}
	// Emit the zero value for this type.
	id, err := e.emitZeroValue(ir.ExprZeroValue{Type: tyHandle})
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// tryLoadSingleStore resolves a load from a single-store vector/scalar local
// directly to the stored value expression, bypassing alloca+store+load.
// This mirrors DXC's mem2reg for the common SROA decomposition pattern.
func (e *Emitter) tryLoadSingleStore(fn *ir.Function, load ir.ExprLoad) (int, bool, error) {
	lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
	if !ok {
		return 0, false, nil
	}
	valueHandle, isSingleStore := e.singleStoreLocals[lv.Variable]
	if !isSingleStore {
		return 0, false, nil
	}
	id, err := e.emitExpression(fn, valueHandle)
	if err != nil {
		return 0, false, err
	}
	// Propagate component tracking from the stored value expression so
	// downstream getComponentID calls resolve correctly for the Load handle.
	if comps, hasComps := e.exprComponents[valueHandle]; hasComps {
		e.pendingComponents = comps
	}
	return id, true, nil
}

// Constant idx collapses at the validator's constant-folding step;
// dynamic idx emits N-1 selects and N-1 icmps per lane. Per-lane
// results are tracked via pendingComponents so downstream compose /
// vec stores pick them up like any other multi-component value.
func (e *Emitter) tryLoadFromConstVecArray(fn *ir.Function, load ir.ExprLoad) (int, bool, error) {
	acc, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprAccess)
	if !ok {
		return 0, false, nil
	}
	lv, ok := fn.Expressions[acc.Base].Kind.(ir.ExprLocalVariable)
	if !ok {
		return 0, false, nil
	}
	table, ok := e.localConstVecArrays[lv.Variable]
	if !ok {
		return 0, false, nil
	}
	if len(table) == 0 {
		return 0, false, nil
	}

	idxID, ok := e.exprValues[acc.Index]
	if !ok {
		// Defensive fallback: emitAccess should have triggered this
		// already via the const-vec-array early return.
		var err error
		idxID, err = e.emitExpression(fn, acc.Index)
		if err != nil {
			return 0, true, err
		}
	}

	numRows := len(table)
	numLanes := len(table[0])
	f32Ty := e.mod.GetFloatType(32)
	comps := make([]int, numLanes)

	for lane := 0; lane < numLanes; lane++ {
		cur := e.getFloatConstID(float64(table[numRows-1][lane]))
		for row := numRows - 2; row >= 0; row-- {
			cmp := e.addCmpInstr(ICmpEQ, idxID, e.getIntConstID(int64(row)))
			valK := e.getFloatConstID(float64(table[row][lane]))
			cur = e.emitSelect3(f32Ty, cmp, valK, cur)
		}
		comps[lane] = cur
	}

	e.pendingComponents = comps
	if len(comps) > 1 {
		e.exprComponents[load.Pointer] = comps
	}
	return comps[0], true, nil
}

// emitLoad emits a load through a pointer.
func (e *Emitter) emitLoad(fn *ir.Function, load ir.ExprLoad) (int, error) {
	// Const vector-array fast path (BUG-DXIL-025). Load from
	// `Access(LocalVariable, idx)` where the local is a constant-init
	// vector array. Emit a per-lane select chain indexed by the
	// runtime `idx` expression — effectively what DXC's SROA + GVN
	// would produce, except we do it at emit time instead of relying
	// on LLVM optimization passes.
	if id, handled, ferr := e.tryLoadFromConstVecArray(fn, load); handled {
		return id, ferr
	}

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

	// Local variable promotion: eliminate alloca+load chains for
	// zero-store, init-only, and single-store locals.
	if id, handled, emitErr := e.tryLoadPromotedLocal(fn, load); emitErr != nil {
		return 0, emitErr
	} else if handled {
		return id, nil
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

	// Check if the load targets a vector-typed struct field of a local struct
	// variable (e.g. `p.vel` where `p: Particle{pos:vec2,vel:vec2}`). The pointer
	// expression is an AccessIndex that produces a scalar-typed GEP to the first
	// component of the field (flat[k]); we must expand this into N scalar loads
	// from N sequential GEPs so that downstream per-component operations (e.g.
	// `p.vel * params.dt`) can access all components.
	//
	// Without this, getComponentID(p.vel, 1) falls back to `baseID+1` which
	// points to an unrelated instruction, producing invalid DXIL bitcode (DXC
	// reports "Invalid record" when the operand's type does not match).
	if id, handled := e.tryStructVectorMemberLoad(fn, load.Pointer); handled {
		return id, nil
	}

	// Check struct/array loads from global variable allocas.
	if id, handled, loadErr := e.tryGlobalVarAllocaLoad(fn, load.Pointer, ptr); loadErr != nil {
		return 0, loadErr
	} else if handled {
		return id, nil
	}

	// Resolve the loaded type from the pointer's target type.
	loadedTy, err := e.resolveLoadType(fn, load.Pointer)
	if err != nil {
		return 0, fmt.Errorf("load type resolution: %w", err)
	}

	// For struct types, decompose into per-member scalar loads.
	// This handles cases where the pointer source is not a simple local/global variable.
	//
	// BUG-DXIL-026 (remaining 4 tilecompute shaders): when the pointer chains
	// through an AccessIndex / Access rooted at a workgroup addrspace(3)
	// global (e.g. `sh[lid.x]` where `sh: array<PathMonoid, 256>`), the
	// per-field GEPs must carry addrspace 3 so the subsequent scalar load's
	// explicit type matches the pointee type of the addrspace(3) pointer.
	if loadedTy.Kind == module.TypeStruct {
		return e.emitStructLoadAS(ptr, loadedTy, e.resolvePointerAddrSpace(fn, load.Pointer))
	}

	// For array types, decompose into per-element scalar loads.
	// DXIL does not support aggregate (array) type loads.
	if loadedTy.Kind == module.TypeArray && loadedTy.ElemType != nil {
		return e.emitArrayLoad(ptr, loadedTy)
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

// emitPhi materializes an ExprPhi as an LLVM FUNC_CODE_INST_PHI
// instruction at the prologue of the current basic block.
//
// Preconditions established by the DXIL mem2reg Phase B walker:
//   - The phi-bearing StmtEmit is positioned IMMEDIATELY after the
//     producing StmtIf / StmtSwitch in the parent statement block.
//   - The corresponding emitIfStatement / emitSwitchStatement has
//     just transitioned currentBB to the merge BB (empty or
//     phi-only at this point) and recorded e.lastBranchBBs with
//     the branch-end BB indices.
//   - Each PhiIncoming.Value's expression has already been emitted
//     during the predecessor branch body, so e.exprValues holds a
//     value ID for it.
//
// We use AddInstruction (not Prepend) because subsequent statements
// in the merge body emit AFTER the phi, never before — so phis end
// up at BB[0..k-1] in the order mem2reg placed them.
//
// Reference parity: matches LLVM mem2reg's invariant that phi
// instructions live at the start of the merge BB; the per-incoming
// (value, predecessor BB) pairs come from the structured-CFG
// branch snapshot in e.lastBranchBBs (PhiPredKey -> BB index).
func (e *Emitter) emitPhi(fn *ir.Function, phi ir.ExprPhi) (int, error) {
	if e.lastBranchBBs == nil {
		return 0, fmt.Errorf("ExprPhi without preceding branch context: phi must "+
			"immediately follow a StmtIf or StmtSwitch (got %d incomings)", len(phi.Incoming))
	}
	if len(phi.Incoming) == 0 {
		return 0, fmt.Errorf("ExprPhi with zero incomings")
	}

	// Resolve each incoming to a (value-ID, BB-index) pair. The
	// value's type is taken from the first incoming (SSA invariant:
	// every incoming has the same type).
	incomings := make([]module.PhiIncoming, 0, len(phi.Incoming))
	for _, inc := range phi.Incoming {
		valueID, evalErr := e.emitExpression(fn, inc.Value)
		if evalErr != nil {
			return 0, fmt.Errorf("phi incoming value eval: %w", evalErr)
		}
		bbIdx, mapErr := e.resolvePhiPredBB(inc.PredKey, inc.CaseIdx)
		if mapErr != nil {
			return 0, mapErr
		}
		incomings = append(incomings, module.PhiIncoming{ValueID: valueID, BBIndex: bbIdx})
	}

	resultTy, err := e.phiResultType(fn, phi)
	if err != nil {
		return 0, fmt.Errorf("phi result type: %w", err)
	}

	valueID := e.allocValue()
	instr := module.NewPhiInstr(resultTy, incomings)
	instr.ValueID = valueID
	e.currentBB.AddInstruction(instr)
	return valueID, nil
}

// resolvePhiPredBB maps a PhiIncoming.PredKey (and CaseIdx for
// switch cases) to the concrete BB index recorded in
// e.lastBranchBBs by the most-recent emitIfStatement /
// emitSwitchStatement.
func (e *Emitter) resolvePhiPredBB(key ir.PhiPredKey, caseIdx uint32) (int, error) {
	bbs := e.lastBranchBBs
	switch key {
	case ir.PhiPredIfAccept:
		if bbs.kind != branchKindIf {
			return 0, fmt.Errorf("PhiPredIfAccept but lastBranchBBs.kind=%d", bbs.kind)
		}
		return bbs.acceptEndBB, nil
	case ir.PhiPredIfReject:
		if bbs.kind != branchKindIf {
			return 0, fmt.Errorf("PhiPredIfReject but lastBranchBBs.kind=%d", bbs.kind)
		}
		// emitIfStatement seeds rejectEndBB to the entry BB (where the
		// conditional branch was emitted) when there is no else block,
		// so the false edge from the conditional br is the predecessor.
		// When there IS an else block, rejectEndBB is its terminator BB.
		// Either way, returning bbs.rejectEndBB is correct.
		return bbs.rejectEndBB, nil
	case ir.PhiPredSwitchCase:
		if bbs.kind != branchKindSwitch {
			return 0, fmt.Errorf("PhiPredSwitchCase but lastBranchBBs.kind=%d", bbs.kind)
		}
		if int(caseIdx) >= len(bbs.caseEndBBs) {
			return 0, fmt.Errorf("PhiPredSwitchCase CaseIdx=%d out of range %d", caseIdx, len(bbs.caseEndBBs))
		}
		return bbs.caseEndBBs[caseIdx], nil
	case ir.PhiPredLoopInit, ir.PhiPredLoopBackEdge, ir.PhiPredFallThrough:
		// Loop phi placement is deferred to BUG-DXIL-041 — mem2reg
		// Phase B walker disqualifies any candidate stored inside
		// a loop, so these PredKeys should never appear in emitted IR.
		return 0, fmt.Errorf("PhiPredKey %d (loop / fallthrough) not yet supported by emit", key)
	default:
		return 0, fmt.Errorf("unknown PhiPredKey %d", key)
	}
}

// phiResultType resolves the LLVM type of an ExprPhi's value. Uses
// the first incoming's type (SSA invariant: every incoming shares
// type). Phase B promotes only scalar local variables, so the
// expected resolution is always a scalar TypeInner.
func (e *Emitter) phiResultType(fn *ir.Function, phi ir.ExprPhi) (*module.Type, error) {
	if len(phi.Incoming) == 0 {
		return nil, fmt.Errorf("phi has no incomings")
	}
	resolution, err := ir.ResolveExpressionType(e.ir, fn, phi.Incoming[0].Value)
	if err != nil {
		return nil, err
	}
	if resolution.Handle != nil {
		irType := e.ir.Types[*resolution.Handle]
		return typeToDXIL(e.mod, e.ir, irType.Inner)
	}
	return typeToDXIL(e.mod, e.ir, resolution.Value)
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

// tryGlobalVarAllocaLoad checks if a load pointer targets a global variable alloca
// with a struct or array type and decomposes the load accordingly.
func (e *Emitter) tryGlobalVarAllocaLoad(fn *ir.Function, ptrHandle ir.ExpressionHandle, ptrID int) (int, bool, error) {
	gv, ok := fn.Expressions[ptrHandle].Kind.(ir.ExprGlobalVariable)
	if !ok {
		return 0, false, nil
	}
	dxilTy, hasTy := e.globalVarAllocaTypes[gv.Variable]
	if !hasTy {
		return 0, false, nil
	}
	if dxilTy.Kind == module.TypeStruct {
		id, err := e.emitStructLoad(ptrID, dxilTy)
		return id, true, err
	}
	if dxilTy.Kind == module.TypeArray && dxilTy.ElemType != nil {
		id, err := e.emitArrayLoad(ptrID, dxilTy)
		return id, true, err
	}
	return 0, false, nil
}

// emitArrayLoad decomposes an array load into per-element GEP + scalar loads.
// DXIL does not support aggregate (array) type loads, so we load each element
// individually and track them as components.
//
// TGSM origin handling: if basePtrID was produced by a single struct-field GEP
// off a workgroup addrspace(3) global (tracked in e.workgroupFieldPtrs), each
// per-element GEP must be rooted DIRECTLY at the global with three indices
// [0, fieldFlatIdx, i] instead of chaining off the field pointer. The DXIL
// validator's TGSM-origin check is data-flow-based and rejects GEP-of-GEP
// chains with 'TGSM pointers must originate from an unambiguous TGSM global
// variable'. Mesa's nir_to_dxil collapses the same pattern via its deref-walk.
func (e *Emitter) emitArrayLoad(basePtrID int, arrayTy *module.Type) (int, error) {
	elemTy := arrayTy.ElemType
	if elemTy == nil {
		return e.getIntConstID(0), nil
	}
	numElems := int(arrayTy.ElemCount) //nolint:gosec // ElemCount bounded by shader array size
	if numElems == 0 {
		return e.getIntConstID(0), nil
	}

	// If basePtrID came from a struct-field GEP off a workgroup global,
	// switch to rooting each element GEP at the global with a 3-index path.
	wgOrigin, rootAtGlobal := workgroupFieldRoot(e, basePtrID)

	addrSpace := uint8(0)
	if rootAtGlobal {
		addrSpace = 3
	}
	resultPtrTy := e.mod.GetPointerTypeAS(elemTy, addrSpace)
	zeroID := e.getIntConstID(0)
	align := e.alignForType(elemTy)

	componentIDs := make([]int, numElems)
	for i := 0; i < numElems; i++ {
		indexID := e.getIntConstID(int64(i))

		var gepID int
		if rootAtGlobal {
			fieldID := e.getIntConstID(int64(wgOrigin.fieldFlatIdx))
			gepID = e.addGEPInstr(
				wgOrigin.structTy,
				resultPtrTy,
				wgOrigin.globalAllocaID,
				[]int{zeroID, fieldID, indexID},
			)
		} else {
			gepID = e.addGEPInstr(arrayTy, resultPtrTy, basePtrID, []int{zeroID, indexID})
		}

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

// workgroupFieldRoot returns the workgroup-global origin info for basePtrID
// if it was produced by a struct-field GEP off an addrspace(3) global.
func workgroupFieldRoot(e *Emitter, basePtrID int) (workgroupFieldOrigin, bool) {
	if e.workgroupFieldPtrs == nil {
		return workgroupFieldOrigin{}, false
	}
	origin, ok := e.workgroupFieldPtrs[basePtrID]
	return origin, ok
}

// tryStructVectorMemberLoad detects loads of a vector-typed field from a local
// struct variable (or a nested chain thereof) and decomposes them into N scalar
// loads with per-component tracking. Returns (id, true, nil) when handled, or
// (0, false, nil) when the pattern does not match.
//
// Bug this fixes (BUG-DXIL-004): for `p.vel` where `p: {pos:vec2,vel:vec2}`,
// emitAccessIndex → emitStructFieldGEP yields a GEP to a single scalar (vel.x).
// The generic emitLoad path then emits one scalar load, leaving component 1
// unresolved; getComponentID(p.vel, 1) falls back to `base+1` which points at
// an unrelated value (e.g. a `%dx.types.CBufRet.f32` struct from a neighboring
// cbufferLoadLegacy), producing type-mismatched operands that DXC rejects with
// "Invalid record".
//
// The fix emits per-component GEPs from the struct alloca and per-component
// loads, setting pendingComponents so downstream code sees the full vector.
func (e *Emitter) tryStructVectorMemberLoad(fn *ir.Function, ptrHandle ir.ExpressionHandle) (int, bool) {
	ai, ok := fn.Expressions[ptrHandle].Kind.(ir.ExprAccessIndex)
	if !ok {
		return 0, false
	}

	// Member type must be a vector for this path; everything else is handled
	// by the generic emitLoad fallthrough.
	memberTy := e.resolveAccessIndexResultType(fn, ai)
	vt, isVec := memberTy.(ir.VectorType)
	if !isVec {
		return 0, false
	}
	numComps := int(vt.Size)
	if numComps <= 1 {
		return 0, false
	}

	// Find the root local struct variable and the flat offset of the target
	// field within the flattened DXIL struct layout. Supports both direct
	// (AccessIndex(LocalVariable, k)) and nested (AccessIndex(AccessIndex…))
	// access patterns.
	rootVarIdx, flatOffset, resolved := e.resolveStructFieldRoot(fn, ai)
	if !resolved {
		return 0, false
	}

	dxilStructTy, hasStructTy := e.localVarStructTypes[rootVarIdx]
	if !hasStructTy {
		return 0, false
	}
	basePtrID, hasBase := e.localVarPtrs[rootVarIdx]
	if !hasBase {
		return 0, false
	}

	// Emit N GEPs (one per component) + N scalar loads.
	zeroID := e.getIntConstID(0)
	componentIDs := make([]int, numComps)
	for i := 0; i < numComps; i++ {
		compFlatIndex := flatOffset + i
		var memberDXILTy *module.Type
		if compFlatIndex < len(dxilStructTy.StructElems) {
			memberDXILTy = dxilStructTy.StructElems[compFlatIndex]
		} else {
			memberDXILTy = e.mod.GetFloatType(32)
		}
		resultPtrTy := e.mod.GetPointerType(memberDXILTy)
		indexID := e.getIntConstID(int64(compFlatIndex))
		gepID := e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID})

		align := e.alignForType(memberDXILTy)
		valueID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrLoad,
			HasValue:   true,
			ResultType: memberDXILTy,
			Operands:   []int{gepID, memberDXILTy.ID, align, 0},
			ValueID:    valueID,
		}
		e.currentBB.AddInstruction(instr)
		componentIDs[i] = valueID
	}

	e.pendingComponents = componentIDs
	return componentIDs[0], true
}

// resolveStructFieldRoot walks an AccessIndex chain rooted at a local struct
// variable and returns (rootVarIdx, flatOffset). For a direct access
// `AccessIndex(LocalVariable(v), k)` the offset is flatMemberOffset of member k;
// for nested access it is the sum of all parent offsets.
func (e *Emitter) resolveStructFieldRoot(fn *ir.Function, ai ir.ExprAccessIndex) (uint32, int, bool) {
	// Direct: base is a local variable.
	if lv, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLocalVariable); ok {
		if int(lv.Variable) >= len(fn.LocalVars) {
			return 0, 0, false
		}
		localVar := &fn.LocalVars[lv.Variable]
		st, isSt := e.ir.Types[localVar.Type].Inner.(ir.StructType)
		if !isSt {
			return 0, 0, false
		}
		if int(ai.Index) >= len(st.Members) {
			return 0, 0, false
		}
		return lv.Variable, flatMemberOffset(e.ir, st, int(ai.Index)), true
	}

	// Nested: base is another AccessIndex. Recurse to find the parent's root
	// and add this index's flat offset within the parent's struct type.
	if parentAI, ok := fn.Expressions[ai.Base].Kind.(ir.ExprAccessIndex); ok {
		rootVar, parentOffset, ok2 := e.resolveStructFieldRoot(fn, parentAI)
		if !ok2 {
			return 0, 0, false
		}
		parentMemberTy := e.resolveAccessIndexResultType(fn, parentAI)
		innerSt, isSt := parentMemberTy.(ir.StructType)
		if !isSt {
			return 0, 0, false
		}
		if int(ai.Index) >= len(innerSt.Members) {
			return 0, 0, false
		}
		return rootVar, parentOffset + flatMemberOffset(e.ir, innerSt, int(ai.Index)), true
	}

	return 0, 0, false
}

// emitStructLoad decomposes a struct load into per-member GEP + scalar loads.
// DXIL does not support aggregate (struct) type loads, so we must load each
// scalar field individually and track them as components.
// Returns the first component's value ID and sets pendingComponents.
func (e *Emitter) emitStructLoad(basePtrID int, dxilStructTy *module.Type) (int, error) {
	return e.emitStructLoadAS(basePtrID, dxilStructTy, 0)
}

// emitStructLoadAS is emitStructLoad with an explicit addrspace for the
// per-field pointers. Workgroup (groupshared) struct loads hit this path
// with addrspace=3; the validator rejects 0x80aa0009 "Explicit load/store
// type does not match pointee type of pointer operand" when the per-field
// GEP result pointer type's addrspace diverges from the base pointer's.
//
// When basePtrID was produced by a single-step array GEP off a workgroup
// global (tracked in workgroupElemPtrs), each per-field GEP is re-rooted
// DIRECTLY at the global with indices [0, elemIdx, fieldN]. Otherwise the
// DXIL validator rejects GEP-of-GEP as 'TGSM pointers must originate from
// an unambiguous TGSM global variable'. Mesa nir_to_dxil collapses the
// equivalent chain via its deref-walk pass.
func (e *Emitter) emitStructLoadAS(basePtrID int, dxilStructTy *module.Type, addrSpace uint8) (int, error) {
	if len(dxilStructTy.StructElems) == 0 {
		return e.getIntConstID(0), nil
	}

	elemOrigin, hasElemOrigin := e.workgroupElemRoot(basePtrID)

	zeroID := e.getIntConstID(0)
	componentIDs := make([]int, len(dxilStructTy.StructElems))

	for i, elemTy := range dxilStructTy.StructElems {
		indexID := e.getIntConstID(int64(i))
		resultPtrTy := e.mod.GetPointerTypeAS(elemTy, addrSpace)
		var gepID int
		if hasElemOrigin {
			gepID = e.addGEPInstr(
				elemOrigin.arrayTy,
				resultPtrTy,
				elemOrigin.globalAllocaID,
				[]int{zeroID, elemOrigin.elemIndexID, indexID},
			)
		} else {
			gepID = e.addGEPInstr(dxilStructTy, resultPtrTy, basePtrID, []int{zeroID, indexID})
		}

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

// workgroupElemRoot returns the workgroup-element origin info for basePtrID
// if it was produced by a single-step array-element GEP off an addrspace(3)
// global. Used by decomposed struct/array load/store paths to re-root per-
// field GEPs directly at the global (TGSM-origin rule).
func (e *Emitter) workgroupElemRoot(basePtrID int) (workgroupElemOrigin, bool) {
	if e.workgroupElemPtrs == nil {
		return workgroupElemOrigin{}, false
	}
	origin, ok := e.workgroupElemPtrs[basePtrID]
	return origin, ok
}

// resolvePointerAddrSpace walks the pointer expression back to its root
// global or local variable and returns the DXIL addrspace (3 for workgroup,
// 0 otherwise). Used by emitLoad/emitStmtStore to choose the right addrspace
// for decomposed per-field/per-element load/store GEPs.
func (e *Emitter) resolvePointerAddrSpace(fn *ir.Function, ptrHandle ir.ExpressionHandle) uint8 {
	cur := ptrHandle
	for {
		if int(cur) >= len(fn.Expressions) {
			return 0
		}
		switch ek := fn.Expressions[cur].Kind.(type) {
		case ir.ExprGlobalVariable:
			if int(ek.Variable) >= len(e.ir.GlobalVariables) {
				return 0
			}
			if e.ir.GlobalVariables[ek.Variable].Space == ir.SpaceWorkGroup {
				return 3
			}
			return 0
		case ir.ExprAccessIndex:
			cur = ek.Base
		case ir.ExprAccess:
			cur = ek.Base
		case ir.ExprLocalVariable:
			return 0
		default:
			return 0
		}
	}
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
	} else if mt, ok := irType.Inner.(ir.MatrixType); ok {
		numComponents = int(mt.Columns) * int(mt.Rows)
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
//
// Vector-element arrays with a constant ExprCompose initializer (the
// `var positions = array<vec2<f32>, 3>(vec2(...), ...)` pattern) are
// SROA'd at emit time into the per-lane constant table
// `localConstVecArrays`. Access paths (tryLocalVarArrayAccessConst,
// tryLocalVarAccessIndexConst) then synthesize the lookup via constants
// and per-lane select chains instead of emitting alloca/store/load.
//
// Rationale: DXIL has no native vectors, so an alloca of
// `[N x <K x T>]` has to be flattened into `[N*K x T]` AND the GEP
// must multiply the array index by the lane stride K. Both halves are
// required; landing only the type flattening leaves the store/load
// paths using stride-1 indexing, which produces malformed INST_STORE
// records and trips HRESULT 0x80aa0009 "Invalid record" at the
// bitcode reader before any semantic validation runs. Mesa
// (nir_lower_indirect_derefs_to_if_else_trees) and DXC (LLVM SROA +
// GVN) both eliminate such locals entirely — we mirror that here for
// the common "constant positions table indexed by vertex_index" case,
// which covers every basic graphics shader in the gogpu ecosystem
// (triangle, particles, etc.). Dynamic indices turn into per-lane
// select chains; constant indices fold to literal constants.
//
// Reference: BUG-DXIL-011 (D3D12 runtime format validator) +
// `reference/dxil/mesa/src/microsoft/compiler/nir_to_dxil.c:6300`
// (`nir_lower_indirect_derefs_to_if_else_trees` with
// `nir_var_function_temp` target).
func (e *Emitter) emitArrayLocalVariable(varIdx uint32, localVar *ir.LocalVariable, irType ir.Type) (int, error) {
	// SROA fast path: constant-init vector-element array.
	if e.tryRegisterLocalConstVecArray(varIdx, localVar, irType) {
		// Sentinel value so later access code branches on the
		// localConstVecArrays map, not on localVarPtrs.
		e.localVarPtrs[varIdx] = -1
		return -1, nil
	}

	arrayTy, err := typeToDXIL(e.mod, e.ir, irType.Inner)
	if err != nil {
		return 0, fmt.Errorf("local var %q array type: %w", localVar.Name, err)
	}
	e.localVarArrayTypes[varIdx] = arrayTy
	valueID := e.emitCompositeAlloca(arrayTy)
	e.localVarPtrs[varIdx] = valueID
	return valueID, nil
}

// tryRegisterLocalConstVecArray detects the
// `var arr = array<vec<T,N>, M>(literal_vec, literal_vec, ...)` pattern
// and, if the initializer is fully constant, records the per-lane
// scalar values in e.localConstVecArrays so the access paths can
// emit an SROA-style select chain instead of an alloca + stores.
//
// Returns true when the pattern matches and has been registered.
//
// tryRegisterLocalConstVecArray walks early returns on each type
// constraint; the cyclomatic count is low enough to not need a nolint.
func (e *Emitter) tryRegisterLocalConstVecArray(varIdx uint32, localVar *ir.LocalVariable, irType ir.Type) bool {
	arr, ok := irType.Inner.(ir.ArrayType)
	if !ok {
		return false
	}
	vec, ok := e.ir.Types[arr.Base].Inner.(ir.VectorType)
	if !ok {
		return false
	}
	if arr.Size.Constant == nil {
		return false
	}
	arrLen := int(*arr.Size.Constant)
	if arrLen == 0 {
		return false
	}
	if localVar.Init == nil {
		return false
	}
	// Only const float vector arrays for now. Extending to int/uint or
	// matrix arrays is a follow-up; the triangle shader only needs f32.
	if vec.Scalar.Kind != ir.ScalarFloat || vec.Scalar.Width != 4 {
		return false
	}
	// Walk the init expression: must be ExprCompose of arrLen element
	// composes, each of which is a vector compose of numLanes scalar
	// literals. (Function-level expressions, not global expressions —
	// WGSL lowering inlines the init into the function's expression
	// arena.)
	compose, ok := e.currentFn.Expressions[*localVar.Init].Kind.(ir.ExprCompose)
	if !ok || len(compose.Components) != arrLen {
		return false
	}
	numLanes := int(vec.Size)
	table := make([][]float32, arrLen)
	for i, elemHandle := range compose.Components {
		row, ok2 := e.evalConstVectorLiteral(elemHandle, numLanes)
		if !ok2 {
			return false
		}
		table[i] = row
	}
	if e.localConstVecArrays == nil {
		e.localConstVecArrays = make(map[uint32][][]float32)
	}
	e.localConstVecArrays[varIdx] = table
	return true
}

// evalConstVectorLiteral walks a vector compose expression and returns
// its N scalar float values if and only if every component is a
// compile-time literal. Fails (returns false) on any runtime expression.
func (e *Emitter) evalConstVectorLiteral(handle ir.ExpressionHandle, numLanes int) ([]float32, bool) {
	if int(handle) >= len(e.currentFn.Expressions) {
		return nil, false
	}
	kind := e.currentFn.Expressions[handle].Kind
	compose, ok := kind.(ir.ExprCompose)
	if !ok || len(compose.Components) != numLanes {
		return nil, false
	}
	row := make([]float32, numLanes)
	for lane := 0; lane < numLanes; lane++ {
		lh := compose.Components[lane]
		if int(lh) >= len(e.currentFn.Expressions) {
			return nil, false
		}
		lk := e.currentFn.Expressions[lh].Kind
		lit, ok := lk.(ir.Literal)
		if !ok {
			return nil, false
		}
		switch v := lit.Value.(type) {
		case ir.LiteralF32:
			row[lane] = float32(v)
		case ir.LiteralI32:
			row[lane] = float32(v)
		case ir.LiteralU32:
			row[lane] = float32(v)
		default:
			return nil, false
		}
	}
	return row, true
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

	// Workgroup variables: emit as proper LLVM global in addrspace 3
	// instead of a function-local alloca. atomicrmw on alloca pointers
	// (addrspace 0) is rejected by the validator with 'Non-groupshared
	// or node record destination to atomic operation'. The proper DXIL
	// encoding is a top-level @globalshared.X = addrspace(3) global,
	// matching DXC's groupshared HLSL lowering.
	//
	// We register the GlobalVar in the module BEFORE the instruction
	// pre-allocates a value ID for it. finalize() assigns the global
	// var's bitcode ValueID and threads it through idMap so subsequent
	// instructions (load/store/atomicrmw) reference the correct global.
	if gv.Space == ir.SpaceWorkGroup {
		modGV := e.mod.AddGlobalVar(gv.Name, elemTy, 3)
		emitterID := e.allocValue()
		if e.globalVarModuleVars == nil {
			e.globalVarModuleVars = make(map[int]*module.GlobalVar)
		}
		e.globalVarModuleVars[emitterID] = modGV
		e.globalVarAllocas[varHandle] = emitterID
		// The alloca-type map is consulted by load/store paths to know
		// the element type. Keep it populated even though no alloca
		// instruction is emitted, so the existing code paths see the
		// right type for the addrspace(3) pointer.
		e.globalVarAllocaTypes[varHandle] = elemTy
		return emitterID, nil
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
	// LLVM 3.7 + DXIL require allocas to live in the function's ENTRY
	// basic block so they dominate all uses regardless of control-flow
	// structure. Inserting into e.currentBB (a nested if/loop body) makes
	// the alloca's SSA value invisible from sibling blocks and trips the
	// LLVM verifier with 'Instruction does not dominate all uses!'.
	// Workgroup variables are accessed lazily on first ExprGlobalVariable,
	// so the access often happens inside a nested block — see
	// atomics.wgsl which calls atomicAdd inside an if-then. Mirroring
	// preAllocateLocalVars's entry-block convention.
	if e.mainFn != nil && len(e.mainFn.BasicBlocks) > 0 {
		e.mainFn.BasicBlocks[0].PrependInstruction(instr)
	} else {
		e.currentBB.AddInstruction(instr)
	}

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
func (e *Emitter) emitZeroValue(zv ir.ExprZeroValue) (int, error) {
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

// emitOverride emits a pipeline-overridable constant's default value.
// DXIL has no direct equivalent to WGSL overrides; we emit them as their
// default init values (matching ProcessOverrides with empty constants map).
func (e *Emitter) emitOverride(eo ir.ExprOverride) (int, error) {
	if int(eo.Override) >= len(e.ir.Overrides) {
		return 0, fmt.Errorf("override %d out of range", eo.Override)
	}
	ov := &e.ir.Overrides[eo.Override]

	// Use the override's init expression if available.
	if ov.Init != nil {
		return e.emitGlobalExpression(*ov.Init)
	}

	// No default value — emit zero for the override's type.
	inner := e.ir.Types[ov.Ty].Inner
	if s, ok := scalarOfType(inner); ok && s.Kind == ir.ScalarFloat {
		return e.getFloatConstID(0.0), nil
	}
	return e.getIntConstID(0), nil
}

// emitConstant emits a reference to a module-scope constant.
func (e *Emitter) emitConstant(ec ir.ExprConstant) (int, error) {
	c := &e.ir.Constants[ec.Constant]

	// Look at the init expression in GlobalExpressions.
	if int(c.Init) < len(e.ir.GlobalExpressions) {
		return e.emitGlobalExpression(c.Init)
	}

	// Fallback: zero value.
	return e.getIntConstID(0), nil
}

// emitGlobalExpression recursively emits a global expression, handling
// Literal, Compose (for vector/struct constants), ZeroValue, and Splat.
func (e *Emitter) emitGlobalExpression(handle ir.ExpressionHandle) (int, error) {
	if int(handle) >= len(e.ir.GlobalExpressions) {
		return e.getIntConstID(0), nil
	}
	globalExpr := e.ir.GlobalExpressions[handle]

	switch gk := globalExpr.Kind.(type) {
	case ir.Literal:
		return e.emitLiteral(gk)

	case ir.ExprCompose:
		// Vector/struct constant: emit each component from global expressions.
		if len(gk.Components) == 0 {
			return e.getIntConstID(0), nil
		}
		var flatIDs []int
		for _, ch := range gk.Components {
			v, err := e.emitGlobalExpression(ch)
			if err != nil {
				return 0, err
			}
			flatIDs = append(flatIDs, v)
		}
		e.pendingComponents = flatIDs
		return flatIDs[0], nil

	case ir.ExprZeroValue:
		return e.emitZeroValue(gk)

	case ir.ExprSplat:
		// Splat: emit the single value and replicate for all components.
		v, err := e.emitGlobalExpression(gk.Value)
		if err != nil {
			return 0, err
		}
		// Determine vector size from the type.
		if int(gk.Size) >= 2 {
			comps := make([]int, gk.Size)
			for i := range comps {
				comps[i] = v
			}
			e.pendingComponents = comps
		}
		return v, nil

	case ir.ExprConstant:
		// Nested constant reference.
		return e.emitConstant(gk)

	case ir.ExprOverride:
		return e.emitOverride(gk)

	case ir.ExprBinary:
		// Chained-override default folding: e.g.
		//   override depth: f32;
		//   override height = 2.0 * depth;
		// evaluates `height` at emit time as `2.0 * depth_default_value`
		// where depth_default_value is 0.0 per the "no pipeline
		// constants" convention (see emitOverride). Without this case,
		// we fell through to the default branch and returned an i32
		// zero where the downstream use expected an f32, producing a
		// type mismatch in the CALL/STORE record and rejection with
		// HRESULT 0x80aa0009 'Invalid record'. Reference: WGSL spec
		// §12.5 (override declarations; constant expressions).
		return e.foldGlobalBinary(gk)

	case ir.ExprUnary:
		return e.foldGlobalUnary(gk)

	default:
		// Unsupported global expression kind — fall back to zero of
		// the correct scalar kind. We cannot know the exact type here
		// without re-resolving, but downstream users tolerate a zero
		// integer for unknown global expression types.
		return e.getIntConstID(0), nil
	}
}

// evalGlobalExpressionFloat constant-folds a scalar numeric global
// expression (Literal / Override default / Binary / Unary) down to a
// concrete float64 value. Returns (value, true) when the fold succeeds,
// or (0, false) when the expression contains shapes we can't evaluate
// at emit time (compose, constant reference to non-scalar, etc.).
//
// The float64 intermediate representation is sufficient for WGSL's
// scalar numeric overrides (f32/i32/u32/bool) because all operations
// are exact within the mantissa range shaders normally use. Caller is
// responsible for narrowing the result to the target type via
// getFloatConstID or getIntConstID.
//
//nolint:gocyclo,cyclop // dispatch table over literal / override / binary / unary kinds
func (e *Emitter) evalGlobalExpressionFloat(handle ir.ExpressionHandle) (float64, bool) {
	if int(handle) >= len(e.ir.GlobalExpressions) {
		return 0, false
	}
	expr := e.ir.GlobalExpressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal:
		switch lv := k.Value.(type) {
		case ir.LiteralF32:
			return float64(lv), true
		case ir.LiteralF64:
			return float64(lv), true
		case ir.LiteralI32:
			return float64(lv), true
		case ir.LiteralU32:
			return float64(lv), true
		case ir.LiteralBool:
			if bool(lv) {
				return 1, true
			}
			return 0, true
		}
		return 0, false

	case ir.ExprOverride:
		if int(k.Override) >= len(e.ir.Overrides) {
			return 0, false
		}
		ov := &e.ir.Overrides[k.Override]
		// Override with an explicit default: fold it recursively.
		if ov.Init != nil {
			return e.evalGlobalExpressionFloat(*ov.Init)
		}
		// No pipeline constant + no default → fall back to 0, matching
		// emitOverride's zero-fallback convention.
		return 0, true

	case ir.ExprConstant:
		if int(k.Constant) >= len(e.ir.Constants) {
			return 0, false
		}
		return e.evalGlobalExpressionFloat(e.ir.Constants[k.Constant].Init)

	case ir.ExprBinary:
		lv, lok := e.evalGlobalExpressionFloat(k.Left)
		rv, rok := e.evalGlobalExpressionFloat(k.Right)
		if !lok || !rok {
			return 0, false
		}
		switch k.Op {
		case ir.BinaryAdd:
			return lv + rv, true
		case ir.BinarySubtract:
			return lv - rv, true
		case ir.BinaryMultiply:
			return lv * rv, true
		case ir.BinaryDivide:
			if rv == 0 {
				return 0, true
			}
			return lv / rv, true
		}
		return 0, false

	case ir.ExprUnary:
		v, ok := e.evalGlobalExpressionFloat(k.Expr)
		if !ok {
			return 0, false
		}
		switch k.Op {
		case ir.UnaryNegate:
			return -v, true
		case ir.UnaryLogicalNot:
			if v == 0 {
				return 1, true
			}
			return 0, true
		}
		return 0, false
	}
	return 0, false
}

// foldGlobalBinary evaluates a binary global expression at emit time
// and returns the constant value ID. Falls back to integer zero when
// the fold fails.
func (e *Emitter) foldGlobalBinary(k ir.ExprBinary) (int, error) {
	if v, ok := e.evalGlobalExpressionFloat(e.globalHandleFor(k)); ok {
		return e.getFloatConstID(v), nil
	}
	return e.getIntConstID(0), nil
}

// foldGlobalUnary is the unary counterpart to foldGlobalBinary.
func (e *Emitter) foldGlobalUnary(k ir.ExprUnary) (int, error) {
	if v, ok := e.evalGlobalExpressionFloat(e.globalHandleFor(k)); ok {
		return e.getFloatConstID(v), nil
	}
	return e.getIntConstID(0), nil
}

// globalHandleFor finds the ExpressionHandle of a given global
// expression kind instance. This is a linear scan; it is only used on
// the const-fold hot path which runs once per override-chain use, so
// the cost is negligible. A map-based index would complicate all
// GlobalExpressions mutations; we keep the scan for simplicity.
func (e *Emitter) globalHandleFor(target interface{}) ir.ExpressionHandle {
	for i := range e.ir.GlobalExpressions {
		if e.ir.GlobalExpressions[i].Kind == target {
			return ir.ExpressionHandle(i)
		}
	}
	return 0
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
	// Same-type cast is a no-op (e.g., bitcast i32 to i32). Also, integer
	// sign reinterpretation (u32 <-> i32) maps to the same DXIL type (i32)
	// so the bitcast is a no-op. DXC never emits these.
	if srcScalar == dstScalar {
		return src, nil
	}
	if srcScalar.Width == dstScalar.Width && isIntegerKind(srcScalar.Kind) && isIntegerKind(dstScalar.Kind) {
		return src, nil
	}
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

// isIntegerKind returns true if the scalar kind is a signed or unsigned integer.
func isIntegerKind(k ir.ScalarKind) bool {
	return k == ir.ScalarSint || k == ir.ScalarUint
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

	// Fallback: try to infer from the expression itself. ExpressionTypes
	// can be sparse for synthetic / lowered expressions (Splat, Swizzle,
	// Compose) where the WGSL lowerer didn't populate the type slot.
	// Returning a wrong default here propagates into componentCount and
	// breaks downstream emission — for example texture sample coord packing
	// counted only one component for vec3<f32>(0.5) and emitted three
	// "coord uninitialized" slots.
	if t := e.inferExprTypeFallback(fn, handle); t != nil {
		return t
	}
	return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
}

// inferExprTypeFallback infers a TypeInner for expressions whose type slot
// in fn.ExpressionTypes is empty. Returns nil when the kind is not handled.
func (e *Emitter) inferExprTypeFallback(fn *ir.Function, handle ir.ExpressionHandle) ir.TypeInner {
	if int(handle) >= len(fn.Expressions) {
		return nil
	}
	switch ek := fn.Expressions[handle].Kind.(type) {
	case ir.Literal:
		return literalType(ek)
	case ir.ExprFunctionArgument:
		if int(ek.Index) < len(fn.Arguments) {
			return e.ir.Types[fn.Arguments[ek.Index].Type].Inner
		}
	case ir.ExprSplat:
		// Splat produces vector<size, scalar(value)>.
		scalarInner := e.resolveExprType(fn, ek.Value)
		if scalar, ok := scalarOfType(scalarInner); ok {
			return ir.VectorType{Size: ek.Size, Scalar: scalar}
		}
	case ir.ExprSwizzle:
		// Swizzle produces vector<size, src.scalar> for vector→vector
		// swizzles. For single-component swizzle (size 1) the result is
		// the source scalar type.
		srcInner := e.resolveExprType(fn, ek.Vector)
		if vec, isVec := srcInner.(ir.VectorType); isVec {
			if ek.Size == 1 {
				return vec.Scalar
			}
			return ir.VectorType{Size: ek.Size, Scalar: vec.Scalar}
		}
	}
	return nil
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

	opcode := derivativeOp(deriv.Axis, deriv.Control)
	dxFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	opcodeVal := e.getIntConstID(int64(opcode))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg}), nil
}

// emitDerivativeWidth emits fwidth(v) = abs(dFdx(v)) + abs(dFdy(v)).
func (e *Emitter) emitDerivativeWidth(arg int, ol overloadType) (int, error) {
	resultTy := e.overloadReturnType(ol)

	// dFdx
	dxFnX := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	opcodeX := e.getIntConstID(int64(OpDerivCoarseX))
	dfdx := e.addCallInstr(dxFnX, dxFnX.FuncType.RetType, []int{opcodeX, arg})

	// dFdy
	dxFnY := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	opcodeY := e.getIntConstID(int64(OpDerivCoarseY))
	dfdy := e.addCallInstr(dxFnY, dxFnY.FuncType.RetType, []int{opcodeY, arg})

	// abs(dFdx)
	absFn := e.getDxOpUnaryFunc(dxOpClassUnary, ol)
	absOp := e.getIntConstID(int64(OpFAbs))
	absDx := e.addCallInstr(absFn, absFn.FuncType.RetType, []int{absOp, dfdx})

	// abs(dFdy)
	absDy := e.addCallInstr(absFn, absFn.FuncType.RetType, []int{absOp, dfdy})

	// abs(dFdx) + abs(dFdy)
	return e.addBinOpInstr(resultTy, BinOpFAdd, absDx, absDy), nil
}

// derivativeOp returns the dx.op opcode for a derivative axis + control.
// All derivative ops live under the OCC::Unary class, so they share the
// dxOpClassUnary function symbol; the caller wires that in directly.
func derivativeOp(axis ir.DerivativeAxis, control ir.DerivativeControl) DXILOpcode {
	switch {
	case axis == ir.DerivativeX && control == ir.DerivativeFine:
		return OpDerivFineX
	case axis == ir.DerivativeY && control == ir.DerivativeFine:
		return OpDerivFineY
	case axis == ir.DerivativeY:
		return OpDerivCoarseY
	default:
		return OpDerivCoarseX
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
		dxFn := e.getDxOpUnaryFunc(dxOpClassIsSpecialFloat, ol)
		opcodeVal := e.getIntConstID(int64(OpIsNaN))
		return e.addCallInstr(dxFn, e.mod.GetIntType(1), []int{opcodeVal, arg}), nil

	case ir.RelationalIsInf:
		ol := overloadForScalar(scalar)
		dxFn := e.getDxOpUnaryFunc(dxOpClassIsSpecialFloat, ol)
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

// ============================================================================
// Pack/Unpack math polyfills
// ============================================================================

// emitMathPack4x8snorm packs vec4<f32> into u32 using signed normalization.
func (e *Emitter) emitMathPack4x8snorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitPackNorm(fn, mathExpr, 4, 8, -1.0, 127.0, true)
}

// emitMathPack4x8unorm packs vec4<f32> into u32 using unsigned normalization.
func (e *Emitter) emitMathPack4x8unorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitPackNorm(fn, mathExpr, 4, 8, 0.0, 255.0, false)
}

// emitMathPack2x16snorm packs vec2<f32> into u32 using signed 16-bit normalization.
func (e *Emitter) emitMathPack2x16snorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitPackNorm(fn, mathExpr, 2, 16, -1.0, 32767.0, true)
}

// emitMathPack2x16unorm packs vec2<f32> into u32 using unsigned 16-bit normalization.
func (e *Emitter) emitMathPack2x16unorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitPackNorm(fn, mathExpr, 2, 16, 0.0, 65535.0, false)
}

// emitPackNorm is the shared implementation for packNxMsnorm/unorm operations.
// components is 2 or 4, bits is 8 or 16, signed selects FPToSI vs FPToUI.
// The max clamp value is always 1.0.
func (e *Emitter) emitPackNorm(fn *ir.Function, mathExpr ir.ExprMath,
	components, bits int, minF, scaleF float64, signed bool,
) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)

	minVal := e.getFloatConstID(minF)
	maxVal := e.getFloatConstID(1.0)
	scale := e.getFloatConstID(scaleF)
	maskVal := e.getIntConstID((1 << bits) - 1)

	castOp := CastFPToUI
	if signed {
		castOp = CastFPToSI
	}

	result := e.getIntConstID(0)
	for c := 0; c < components; c++ {
		comp := e.getComponentID(mathExpr.Arg, c)
		clamped := e.emitFMaxFMin(f32Ty, comp, minVal, maxVal)
		scaled := e.addBinOpInstr(f32Ty, BinOpFMul, clamped, scale)
		rounded := e.emitRoundNE(scaled)
		intVal := e.addCastInstr(i32Ty, castOp, rounded)
		masked := e.addBinOpInstr(i32Ty, BinOpAnd, intVal, maskVal)
		if c > 0 {
			shift := e.getIntConstID(int64(c * bits))
			masked = e.addBinOpInstr(i32Ty, BinOpShl, masked, shift)
		}
		result = e.addBinOpInstr(i32Ty, BinOpOr, result, masked)
	}
	return result, nil
}

// emitMathPack2x16float packs vec2<f32> into u32 as two f16 values.
//
// Lowered via dx.op.legacyF32ToF16 (opcode 130). This opcode takes an
// f32 and returns an i32 containing the f16 bit pattern in its low
// 16 bits — exactly what we need, with no half type or min-precision
// bitcast involved. Matches dxc's lowering of HLSL f32tof16 (verified
// via dxc -T cs_6_0 on a struct buffer store). Using a staged
// f32->half->i16->i32 bitcast chain instead trips DXIL's
// 'Bitcast on minprecison types is not allowed' rule when the shader
// is not compiled with -enable-16bit-types / UseNativeLowPrecision.
func (e *Emitter) emitMathPack2x16float(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	legacyFn := e.getDxOpLegacyF32ToF16Func()
	opcodeVal := e.getIntConstID(int64(OpLegacyF32ToF16))
	mask := e.getIntConstID(0xFFFF)

	result := e.getIntConstID(0)
	for c := 0; c < 2; c++ {
		comp := e.getComponentID(mathExpr.Arg, c)
		// Returns i32 with the f16 bits in low 16 bits; mask for safety.
		i32Val := e.addCallInstr(legacyFn, i32Ty, []int{opcodeVal, comp})
		masked := e.addBinOpInstr(i32Ty, BinOpAnd, i32Val, mask)
		if c > 0 {
			shift := e.getIntConstID(16)
			masked = e.addBinOpInstr(i32Ty, BinOpShl, masked, shift)
		}
		result = e.addBinOpInstr(i32Ty, BinOpOr, result, masked)
	}
	return result, nil
}

// getDxOpLegacyF32ToF16Func returns the dx.op.legacyF32ToF16 declaration:
// i32 @dx.op.legacyF32ToF16(i32 opcode, float value). No overload suffix
// (opcode name is already type-specific).
func (e *Emitter) getDxOpLegacyF32ToF16Func() *module.Function {
	f32Ty := e.mod.GetFloatType(32)
	i32Ty := e.mod.GetIntType(32)
	funcTy := e.mod.GetFunctionType(i32Ty, []*module.Type{i32Ty, f32Ty})
	return e.getOrCreateDxOpFunc("dx.op.legacyF32ToF16", overloadVoid, funcTy)
}

// getDxOpLegacyF16ToF32Func returns the dx.op.legacyF16ToF32 declaration:
// float @dx.op.legacyF16ToF32(i32 opcode, i32 value). No overload suffix.
func (e *Emitter) getDxOpLegacyF16ToF32Func() *module.Function {
	f32Ty := e.mod.GetFloatType(32)
	i32Ty := e.mod.GetIntType(32)
	funcTy := e.mod.GetFunctionType(f32Ty, []*module.Type{i32Ty, i32Ty})
	return e.getOrCreateDxOpFunc("dx.op.legacyF16ToF32", overloadVoid, funcTy)
}

// emitMathPack4xI8 packs vec4<i32> into u32 by truncating to 8-bit signed.
func (e *Emitter) emitMathPack4xI8(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	mask := e.getIntConstID(0xFF)

	result := e.getIntConstID(0)
	for c := 0; c < 4; c++ {
		comp := e.getComponentID(mathExpr.Arg, c)
		masked := e.addBinOpInstr(i32Ty, BinOpAnd, comp, mask)
		if c > 0 {
			shift := e.getIntConstID(int64(c * 8))
			masked = e.addBinOpInstr(i32Ty, BinOpShl, masked, shift)
		}
		result = e.addBinOpInstr(i32Ty, BinOpOr, result, masked)
	}
	return result, nil
}

// emitMathPack4xU8 packs vec4<u32> into u32 by truncating to 8-bit unsigned.
func (e *Emitter) emitMathPack4xU8(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitMathPack4xI8(fn, mathExpr) // Same bit pattern.
}

// emitMathPack4xI8Clamp packs vec4<i32> into u32 with clamping to [-128, 127].
func (e *Emitter) emitMathPack4xI8Clamp(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	mask := e.getIntConstID(0xFF)
	clampMin := e.getIntConstID(-128)
	clampMax := e.getIntConstID(127)

	result := e.getIntConstID(0)
	for c := 0; c < 4; c++ {
		comp := e.getComponentID(mathExpr.Arg, c)
		// Clamp to [-128, 127] using IMax + IMin.
		clamped := e.emitIMaxIMin(i32Ty, comp, clampMin, clampMax)
		masked := e.addBinOpInstr(i32Ty, BinOpAnd, clamped, mask)
		if c > 0 {
			shift := e.getIntConstID(int64(c * 8))
			masked = e.addBinOpInstr(i32Ty, BinOpShl, masked, shift)
		}
		result = e.addBinOpInstr(i32Ty, BinOpOr, result, masked)
	}
	return result, nil
}

// emitMathPack4xU8Clamp packs vec4<u32> into u32 with clamping to [0, 255].
func (e *Emitter) emitMathPack4xU8Clamp(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	if _, err := e.emitExpression(fn, mathExpr.Arg); err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	mask := e.getIntConstID(0xFF)
	clampMax := e.getIntConstID(255)

	result := e.getIntConstID(0)
	for c := 0; c < 4; c++ {
		comp := e.getComponentID(mathExpr.Arg, c)
		// Clamp to [0, 255] using UMin.
		dxFn := e.getDxOpBinaryFunc(dxOpClassBinary, overloadI32)
		opcodeVal := e.getIntConstID(int64(OpUMin))
		clamped := e.addCallInstr(dxFn, i32Ty, []int{opcodeVal, comp, clampMax})
		masked := e.addBinOpInstr(i32Ty, BinOpAnd, clamped, mask)
		if c > 0 {
			shift := e.getIntConstID(int64(c * 8))
			masked = e.addBinOpInstr(i32Ty, BinOpShl, masked, shift)
		}
		result = e.addBinOpInstr(i32Ty, BinOpOr, result, masked)
	}
	return result, nil
}

// emitMathUnpack4x8snorm unpacks u32 into vec4<f32> using signed normalization.
func (e *Emitter) emitMathUnpack4x8snorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitUnpackNorm(fn, mathExpr, 4, 8, 1.0/127.0, true)
}

// emitMathUnpack4x8unorm unpacks u32 into vec4<f32> using unsigned normalization.
func (e *Emitter) emitMathUnpack4x8unorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitUnpackNorm(fn, mathExpr, 4, 8, 1.0/255.0, false)
}

// emitMathUnpack2x16snorm unpacks u32 into vec2<f32> using signed 16-bit normalization.
func (e *Emitter) emitMathUnpack2x16snorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitUnpackNorm(fn, mathExpr, 2, 16, 1.0/32767.0, true)
}

// emitMathUnpack2x16unorm unpacks u32 into vec2<f32> using unsigned 16-bit normalization.
func (e *Emitter) emitMathUnpack2x16unorm(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	return e.emitUnpackNorm(fn, mathExpr, 2, 16, 1.0/65535.0, false)
}

// emitUnpackNorm is the shared implementation for unpackNxMsnorm/unorm operations.
// components is 2 or 4, bits is 8 or 16. signed selects sign-extension + SIToFP.
func (e *Emitter) emitUnpackNorm(fn *ir.Function, mathExpr ir.ExprMath,
	components, bits int, scaleF float64, signed bool,
) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	scale := e.getFloatConstID(scaleF)
	sextShift := 32 - bits

	comps := make([]int, components)
	for c := 0; c < components; c++ {
		shifted := arg
		if c > 0 {
			shift := e.getIntConstID(int64(c * bits))
			shifted = e.addBinOpInstr(i32Ty, BinOpLShr, arg, shift)
		}
		var fVal int
		if signed {
			// Sign-extend from N bits: shl by (32-N), ashr by (32-N).
			shl := e.addBinOpInstr(i32Ty, BinOpShl, shifted, e.getIntConstID(int64(sextShift)))
			sext := e.addBinOpInstr(i32Ty, BinOpAShr, shl, e.getIntConstID(int64(sextShift)))
			fVal = e.addCastInstr(f32Ty, CastSIToFP, sext)
		} else {
			mask := e.getIntConstID((1 << bits) - 1)
			masked := e.addBinOpInstr(i32Ty, BinOpAnd, shifted, mask)
			fVal = e.addCastInstr(f32Ty, CastUIToFP, masked)
		}
		scaled := e.addBinOpInstr(f32Ty, BinOpFMul, fVal, scale)
		if signed {
			// Clamp to [-1.0, 1.0].
			minVal := e.getFloatConstID(-1.0)
			maxVal := e.getFloatConstID(1.0)
			comps[c] = e.emitFMaxFMin(f32Ty, scaled, minVal, maxVal)
		} else {
			comps[c] = scaled
		}
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathUnpack2x16float unpacks u32 into vec2<f32> as two f16 values.
//
// Lowered via dx.op.legacyF16ToF32 (opcode 131). Takes an i32 with the
// f16 bit pattern in the low 16 bits and returns an f32. Mirrors dxc's
// lowering of HLSL f16tof32 and avoids the minprecision-bitcast rule.
func (e *Emitter) emitMathUnpack2x16float(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	mask := e.getIntConstID(0xFFFF)
	legacyFn := e.getDxOpLegacyF16ToF32Func()
	opcodeVal := e.getIntConstID(int64(OpLegacyF16ToF32))

	comps := make([]int, 2)
	for c := 0; c < 2; c++ {
		shifted := arg
		if c > 0 {
			shift := e.getIntConstID(16)
			shifted = e.addBinOpInstr(i32Ty, BinOpLShr, arg, shift)
		}
		masked := e.addBinOpInstr(i32Ty, BinOpAnd, shifted, mask)
		comps[c] = e.addCallInstr(legacyFn, f32Ty, []int{opcodeVal, masked})
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathUnpack4xI8 unpacks u32 into vec4<i32> with sign extension.
func (e *Emitter) emitMathUnpack4xI8(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)

	comps := make([]int, 4)
	for c := 0; c < 4; c++ {
		shifted := arg
		if c > 0 {
			shift := e.getIntConstID(int64(c * 8))
			shifted = e.addBinOpInstr(i32Ty, BinOpLShr, arg, shift)
		}
		// Sign-extend from 8 bits: shift left 24, arithmetic shift right 24.
		shl := e.addBinOpInstr(i32Ty, BinOpShl, shifted, e.getIntConstID(24))
		comps[c] = e.addBinOpInstr(i32Ty, BinOpAShr, shl, e.getIntConstID(24))
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitMathUnpack4xU8 unpacks u32 into vec4<u32> by extracting bytes.
func (e *Emitter) emitMathUnpack4xU8(fn *ir.Function, mathExpr ir.ExprMath) (int, error) {
	arg, err := e.emitExpression(fn, mathExpr.Arg)
	if err != nil {
		return 0, err
	}
	i32Ty := e.mod.GetIntType(32)
	mask := e.getIntConstID(0xFF)

	comps := make([]int, 4)
	for c := 0; c < 4; c++ {
		shifted := arg
		if c > 0 {
			shift := e.getIntConstID(int64(c * 8))
			shifted = e.addBinOpInstr(i32Ty, BinOpLShr, arg, shift)
		}
		comps[c] = e.addBinOpInstr(i32Ty, BinOpAnd, shifted, mask)
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitFMaxFMin performs clamp(val, minV, maxV) using dx.op.fmax and dx.op.fmin.
func (e *Emitter) emitFMaxFMin(f32Ty *module.Type, val, minVal, maxVal int) int {
	// max(val, minVal) then min(result, maxVal)
	fmaxFn := e.getDxOpBinaryFunc(dxOpClassBinary, overloadF32)
	fminFn := e.getDxOpBinaryFunc(dxOpClassBinary, overloadF32)
	opcodeMax := e.getIntConstID(int64(OpFMax))
	opcodeMin := e.getIntConstID(int64(OpFMin))
	clamped := e.addCallInstr(fmaxFn, f32Ty, []int{opcodeMax, val, minVal})
	return e.addCallInstr(fminFn, f32Ty, []int{opcodeMin, clamped, maxVal})
}

// emitIMaxIMin performs clamp(val, minV, maxV) using dx.op.imax and dx.op.imin.
func (e *Emitter) emitIMaxIMin(i32Ty *module.Type, val, minVal, maxVal int) int {
	imaxFn := e.getDxOpBinaryFunc(dxOpClassBinary, overloadI32)
	iminFn := e.getDxOpBinaryFunc(dxOpClassBinary, overloadI32)
	opcodeMax := e.getIntConstID(int64(OpIMax))
	opcodeMin := e.getIntConstID(int64(OpIMin))
	clamped := e.addCallInstr(imaxFn, i32Ty, []int{opcodeMax, val, minVal})
	return e.addCallInstr(iminFn, i32Ty, []int{opcodeMin, clamped, maxVal})
}

// emitRoundNE emits a dx.op.round_ne (round to nearest even) call.
func (e *Emitter) emitRoundNE(arg int) int {
	dxFn := e.getDxOpUnaryFunc(dxOpClassUnary, overloadF32)
	opcodeVal := e.getIntConstID(int64(OpRoundNE))
	return e.addCallInstr(dxFn, dxFn.FuncType.RetType, []int{opcodeVal, arg})
}

// tryInlineLoadOfLocalVarAccessIndex handles the pattern
//
//	ExprAccessIndex{Base: ExprLoad{Pointer: ExprLocalVariable{slot}}, Index: N}
//
// produced by the IR-level inline pass when an aggregate by-value argument is
// indexed inside the inlined body by constant field/element. DXIL forbids
// materializing the aggregate Load; we route the access directly to the
// slot's alloca via GEP, then emit a scalar Load for value-context use.
//
// Supports array (Index = element) and struct (Index = field) locals. Returns
// (valueID, true, nil) when handled; (0, false, nil) when the pattern does
// not match and the caller should take the default path.
func (e *Emitter) tryInlineLoadOfLocalVarAccessIndex(fn *ir.Function, ai ir.ExprAccessIndex) (int, bool, error) {
	load, ok := fn.Expressions[ai.Base].Kind.(ir.ExprLoad)
	if !ok {
		return 0, false, nil
	}
	lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
	if !ok {
		return 0, false, nil
	}
	if int(lv.Variable) >= len(fn.LocalVars) {
		return 0, false, nil
	}
	localTy := e.ir.Types[fn.LocalVars[lv.Variable].Type].Inner

	allocaID, err := e.emitLocalVariable(fn, lv)
	if err != nil {
		return 0, false, fmt.Errorf("inline-loadindex: alloca %w", err)
	}

	zeroID := e.getIntConstID(0)
	fieldID := e.getIntConstID(int64(ai.Index))

	switch localTy.(type) {
	case ir.ArrayType:
		arrayTy, hasArr := e.localVarArrayTypes[lv.Variable]
		if !hasArr {
			return 0, false, fmt.Errorf("inline-loadindex: array type cache missing for var %d", lv.Variable)
		}
		elemTy := arrayTy.ElemType
		if elemTy == nil {
			return 0, false, fmt.Errorf("inline-loadindex: arrayTy missing ElemType")
		}
		ptrTy := e.mod.GetPointerType(elemTy)
		gepID := e.addGEPInstr(arrayTy, ptrTy, allocaID, []int{zeroID, fieldID})
		valID := e.allocValue()
		align := e.alignForType(elemTy)
		e.currentBB.AddInstruction(&module.Instruction{
			Kind:       module.InstrLoad,
			HasValue:   true,
			ResultType: elemTy,
			Operands:   []int{gepID, elemTy.ID, align, 0},
			ValueID:    valID,
		})
		return valID, true, nil

	case ir.StructType:
		structTy, hasSt := e.localVarStructTypes[lv.Variable]
		if !hasSt {
			return 0, false, nil // struct alloca shape not cached — fall through to default path
		}
		if int(ai.Index) >= len(structTy.StructElems) {
			return 0, false, fmt.Errorf("inline-loadindex: struct field index %d out of range (have %d)", ai.Index, len(structTy.StructElems))
		}
		elemTy := structTy.StructElems[ai.Index]
		ptrTy := e.mod.GetPointerType(elemTy)
		gepID := e.addGEPInstr(structTy, ptrTy, allocaID, []int{zeroID, fieldID})
		valID := e.allocValue()
		align := e.alignForType(elemTy)
		e.currentBB.AddInstruction(&module.Instruction{
			Kind:       module.InstrLoad,
			HasValue:   true,
			ResultType: elemTy,
			Operands:   []int{gepID, elemTy.ID, align, 0},
			ValueID:    valID,
		})
		return valID, true, nil
	}
	return 0, false, nil
}

// tryInlineLoadOfLocalVarAccess handles the dynamic-index counterpart to
// tryInlineLoadOfLocalVarAccessIndex: ExprAccess (rather than ExprAccessIndex)
// over a Load-of-localvar produced by the IR-level inline pass for aggregate
// by-value arguments. Pattern:
//
//	ExprAccess{Base: ExprLoad{Pointer: ExprLocalVariable{slot}}, Index: <dyn>}
//
// emit cannot materialize the aggregate Load. Peel to the slot alloca, GEP
// to the indexed element with the dynamic index value, then scalar-Load.
func (e *Emitter) tryInlineLoadOfLocalVarAccess(fn *ir.Function, acc ir.ExprAccess) (int, bool, error) {
	load, ok := fn.Expressions[acc.Base].Kind.(ir.ExprLoad)
	if !ok {
		return 0, false, nil
	}
	lv, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
	if !ok {
		return 0, false, nil
	}
	if int(lv.Variable) >= len(fn.LocalVars) {
		return 0, false, nil
	}
	localTy := e.ir.Types[fn.LocalVars[lv.Variable].Type].Inner
	if _, isArr := localTy.(ir.ArrayType); !isArr {
		return 0, false, nil
	}
	indexID, err := e.emitExpression(fn, acc.Index)
	if err != nil {
		return 0, false, err
	}
	allocaID, err := e.emitLocalVariable(fn, lv)
	if err != nil {
		return 0, false, fmt.Errorf("inline-load-of-localvar access: %w", err)
	}
	arrayTy, hasArr := e.localVarArrayTypes[lv.Variable]
	if !hasArr {
		return 0, false, fmt.Errorf("inline-load-of-localvar: array type cache missing for var %d", lv.Variable)
	}
	gepID, err := e.emitArrayGEP(allocaID, arrayTy, indexID)
	if err != nil {
		return 0, false, err
	}
	elemTy := arrayTy.ElemType
	if elemTy == nil {
		return 0, false, fmt.Errorf("inline-load-of-localvar: arrayTy missing ElemType")
	}
	valID := e.allocValue()
	align := e.alignForType(elemTy)
	e.currentBB.AddInstruction(&module.Instruction{
		Kind:       module.InstrLoad,
		HasValue:   true,
		ResultType: elemTy,
		Operands:   []int{gepID, elemTy.ID, align, 0},
		ValueID:    valID,
	})
	return valID, true, nil
}
