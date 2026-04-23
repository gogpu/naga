package emit

import (
	"fmt"
	"sort"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/internal/backend"
	"github.com/gogpu/naga/ir"
)

// emitInputLoads emits dx.op.loadInput calls for each entry point argument.
// Each vector component becomes a separate loadInput call.
// For compute shaders, builtin arguments use dx.op.threadId/groupId/etc.
//
// DXC emits loadInput calls in reverse ISG1 row order (last signature element
// first). This is a consequence of DXC's expression-tree-driven code generation
// which evaluates inputs at point of first use. Our pre-load approach mimics
// this by iterating loadInput-producing args in reverse after handling special
// intrinsics (compute builtins, ViewID, coverage).
//
// Reference: Mesa nir_to_dxil.c emit_load_input_via_intrinsic(),
// emit_load_global_invocation_id(), emit_load_local_invocation_id()
func (e *Emitter) emitInputLoads(fn *ir.Function, stage ir.ShaderStage) error {
	// Precompute ISG1 row assignments that match the sorted order produced
	// by collectFlatArgBindings + PackSignatureElements. This ensures the
	// loadInput sigId values match the ISG1 register layout exactly.
	rowMap := e.buildInputRowMap(fn, stage)
	e.inputRowMap = rowMap

	// Phase 1: classify each argument. Identify which args need loadInput,
	// which need special intrinsics, and assign ISG1 row indices (sigId).
	type loadInputArg struct {
		argIdx   int
		arg      *ir.FunctionArgument
		inputRow int
	}
	var loadArgs []loadInputArg

	for argIdx := range fn.Arguments {
		arg := &fn.Arguments[argIdx]
		if arg.Binding == nil {
			// Struct-typed arguments: handled separately with their own reverse logic.
			argType := e.ir.Types[arg.Type]
			if st, isSt := argType.Inner.(ir.StructType); isSt {
				startRow := rowMap[inputRowKey{argIdx: argIdx, memberIdx: -1}]
				if err := e.emitStructInputLoads(fn, argIdx, &st, stage, startRow); err != nil {
					return err
				}
			}
			continue
		}

		if handled, err := e.tryEmitComputeBuiltin(fn, argIdx, arg, stage); err != nil {
			return err
		} else if handled {
			continue
		}

		// Vertex / fragment SV_ViewID: not a signature element. DXC reads
		// it via dx.op.viewID(i32 138) returning i32 — see
		// DxilOperations.cpp:ViewID. The skipBuiltin / addElement paths
		// already exclude it from ISG1 / PSV0; we must NOT emit a
		// loadInput call here either, and we must NOT advance inputRow.
		if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinViewIndex {
			if err := e.emitViewIDLoad(fn, argIdx); err != nil {
				return err
			}
			continue
		}

		// Fragment shader @builtin(sample_mask): read via dx.op.coverage
		// (opcode 91) instead of loadInput. SV_Coverage on PS input is
		// NotInSig (DxilSigPoint.inl:97-98) — the validator rejects an
		// SV_Coverage element in the input signature.
		if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinSampleMask && stage == ir.StageFragment {
			if err := e.emitCoverageLoad(fn, argIdx); err != nil {
				return err
			}
			continue
		}

		row := rowMap[inputRowKey{argIdx: argIdx, memberIdx: -1}]
		loadArgs = append(loadArgs, loadInputArg{argIdx: argIdx, arg: arg, inputRow: row})
	}

	// Phase 2: emit loadInput calls in reverse ISG1 row order to match DXC.
	// DCE: skip loadInput for arguments whose value is never consumed.
	// DXC's downstream LLVM ADCE removes these dead loads; the ISG1
	// signature element is retained (for pipeline compatibility) but the
	// bitcode body omits the call. loadInput is readnone so elision is
	// observably correct.
	// Sort by inputRow so reverse iteration matches reverse ISG1 order.
	sort.SliceStable(loadArgs, func(i, j int) bool {
		return loadArgs[i].inputRow < loadArgs[j].inputRow
	})
	for i := len(loadArgs) - 1; i >= 0; i-- {
		la := &loadArgs[i]
		if !isArgRead(fn, la.argIdx) {
			continue
		}
		if err := e.emitSingleInputLoad(fn, la.argIdx, la.arg, stage, la.inputRow); err != nil {
			return err
		}
	}
	return nil
}

// inputRowKey identifies a specific binding element in the flat input list.
// argIdx is the function argument index; memberIdx is the struct member index
// (or -1 for non-struct arguments or as a struct-level start row marker).
type inputRowKey struct {
	argIdx    int
	memberIdx int
}

// buildInputRowMap precomputes the ISG1 row assignment for every input binding
// element, matching the sorted order from collectFlatArgBindings + PackSignatureElements.
// Returns a map from (argIdx, memberIdx) to ISG1 row index.
//
// For struct arguments, the map contains an entry with memberIdx=-1 whose
// value is the starting row for that struct's first loadInput-producing member
// in the sorted order. emitStructInputLoads uses its own SortedMemberIndices
// walk, so the starting row is sufficient.
func (e *Emitter) buildInputRowMap(fn *ir.Function, stage ir.ShaderStage) map[inputRowKey]int {
	isFragment := stage == ir.StageFragment
	isVSInput := stage == ir.StageVertex
	isComputeLike := stage == ir.StageCompute || stage == ir.StageMesh || stage == ir.StageTask

	interpFn := func(loc ir.LocationBinding) backend.SigPackInterp {
		return backend.SigPackInterp(interpolationModeForBinding(loc.Interpolation, isFragment, false))
	}

	entries := e.collectFlatInputEntries(fn, isVSInput)

	// Build SigElementInfo for packing (mirrors collectGraphicsSignatures).
	infos := make([]backend.SigElementInfo, len(entries))
	for i, ent := range entries {
		infos[i] = backend.SigElementInfoForBinding(e.ir, ent.binding, ent.typeH, stage, false, interpFn)
		if isVSInput && infos[i].Kind == backend.SigPackLocation {
			infos[i].Kind = backend.SigPackBuiltinSystemValue
		}
	}
	packed := backend.PackSignatureElements(infos, true)

	// Map each entry to its packed register row.
	result := make(map[inputRowKey]int, len(entries))
	for i, ent := range entries {
		pe := packed[i]
		if pe.Rows == 0 {
			continue
		}
		if isSkippedInputBinding(ent.binding, stage, isComputeLike) {
			continue
		}
		result[inputRowKey{argIdx: ent.argIdx, memberIdx: ent.memberIdx}] = int(pe.Register)
	}

	// Add struct-level start row sentinels.
	addStructStartRows(e.ir, fn, result, stage, isComputeLike)
	return result
}

// inputFlatEntry represents one binding element in the flattened input list.
type inputFlatEntry struct {
	argIdx    int
	memberIdx int // -1 for direct-binding args
	binding   ir.Binding
	typeH     ir.TypeHandle
}

// collectFlatInputEntries builds the sorted flat binding list for all
// function arguments (same order as collectFlatArgBindings).
func (e *Emitter) collectFlatInputEntries(fn *ir.Function, isVSInput bool) []inputFlatEntry {
	var entries []inputFlatEntry
	for argIdx := range fn.Arguments {
		arg := &fn.Arguments[argIdx]
		if arg.Binding != nil {
			entries = append(entries, inputFlatEntry{argIdx: argIdx, memberIdx: -1, binding: *arg.Binding, typeH: arg.Type})
			continue
		}
		argType := e.ir.Types[arg.Type]
		st, ok := argType.Inner.(ir.StructType)
		if !ok {
			continue
		}
		order := backend.SortedMemberIndices(st.Members)
		for _, idx := range order {
			m := st.Members[idx]
			if m.Binding == nil {
				continue
			}
			entries = append(entries, inputFlatEntry{argIdx: argIdx, memberIdx: idx, binding: *m.Binding, typeH: m.Type})
		}
	}
	if !isVSInput {
		sort.SliceStable(entries, func(i, j int) bool {
			ki := backend.NewMemberInterfaceKey(&entries[i].binding)
			kj := backend.NewMemberInterfaceKey(&entries[j].binding)
			return backend.MemberInterfaceLess(ki, kj)
		})
	}
	return entries
}

// addStructStartRows adds memberIdx=-1 sentinel entries to the row map
// for each struct-typed argument, using the minimum row of any member.
func addStructStartRows(irMod *ir.Module, fn *ir.Function, result map[inputRowKey]int, stage ir.ShaderStage, isComputeLike bool) {
	for argIdx := range fn.Arguments {
		arg := &fn.Arguments[argIdx]
		if arg.Binding != nil {
			continue
		}
		argType := irMod.Types[arg.Type]
		st, ok := argType.Inner.(ir.StructType)
		if !ok {
			continue
		}
		minRow := -1
		order := backend.SortedMemberIndices(st.Members)
		for _, idx := range order {
			m := st.Members[idx]
			if m.Binding == nil {
				continue
			}
			if isSkippedInputBinding(*m.Binding, stage, isComputeLike) {
				continue
			}
			if r, ok := result[inputRowKey{argIdx: argIdx, memberIdx: idx}]; ok {
				if minRow < 0 || r < minRow {
					minRow = r
				}
			}
		}
		if minRow >= 0 {
			result[inputRowKey{argIdx: argIdx, memberIdx: -1}] = minRow
		}
	}
}

// isSkippedInputBinding returns true for bindings that don't produce
// loadInput calls and don't consume ISG1 rows. Used by buildInputRowMap
// to filter out ViewID, Coverage, and compute builtins.
func isSkippedInputBinding(binding ir.Binding, stage ir.ShaderStage, isComputeLike bool) bool {
	bb, ok := binding.(ir.BuiltinBinding)
	if !ok {
		return false
	}
	if bb.Builtin == ir.BuiltinViewIndex {
		return true
	}
	if bb.Builtin == ir.BuiltinSampleMask && stage == ir.StageFragment {
		return true
	}
	if isComputeLike {
		return true
	}
	return false
}

// emitCoverageLoad emits dx.op.coverage.i32(i32 91) and binds the result
// to the entry point's @builtin(sample_mask) input argument. DXC reads
// PS coverage via this dedicated intrinsic; SV_Coverage as PS input is
// NotInSig per DxilSigPoint.inl:97 'NotInSig _50'.
func (e *Emitter) emitCoverageLoad(fn *ir.Function, argIdx int) error {
	i32Ty := e.mod.GetIntType(32)
	covFn := e.getDxOpComputeBuiltinFunc("dx.op.coverage", false)
	opcodeVal := e.getIntConstID(int64(OpCoverage))
	valueID := e.addCallInstr(covFn, i32Ty, []int{opcodeVal})
	exprHandle := e.findArgExprHandle(fn, argIdx)
	e.exprValues[exprHandle] = valueID
	return nil
}

// emitViewIDLoad emits dx.op.viewID.i32(i32 138) and binds the result to
// the entry point's view_index argument. Mirrors DXC's lowering of
// SV_ViewID — the value is read via the dedicated intrinsic instead of
// participating in the input signature.
func (e *Emitter) emitViewIDLoad(fn *ir.Function, argIdx int) error {
	i32Ty := e.mod.GetIntType(32)
	viewFn := e.getDxOpComputeBuiltinFunc("dx.op.viewID", false)
	opcodeVal := e.getIntConstID(int64(OpViewID))
	valueID := e.addCallInstr(viewFn, i32Ty, []int{opcodeVal})
	exprHandle := e.findArgExprHandle(fn, argIdx)
	e.exprValues[exprHandle] = valueID
	return nil
}

// emitSingleInputLoad emits dx.op.loadInput calls for a single argument.
// inputRow is the ISG1 row index for this argument's signature element.
//
// Bool inputs (e.g. @builtin(front_facing) for SV_IsFrontFace) cannot use
// dx.op.loadInput.i1 — DXC's DxilOperations.cpp:LoadInput declares overload
// mask 0x63 = {f16, f32, i16, i32}, no i1. DXC instead loads the value as
// i32 and converts via `icmp ne i32 %loaded, 0` to produce the i1 the
// shader code expects. We mirror that lowering here.
func (e *Emitter) emitSingleInputLoad(fn *ir.Function, argIdx int, arg *ir.FunctionArgument, stage ir.ShaderStage, inputRow int) error {
	argType := e.ir.Types[arg.Type]
	scalar, ok := scalarOfType(argType.Inner)
	if !ok {
		if st, isSt := argType.Inner.(ir.StructType); isSt {
			return e.emitStructInputLoads(fn, argIdx, &st, stage, inputRow)
		}
		return fmt.Errorf("unsupported input type for argument %d", argIdx)
	}

	// Bool inputs are loaded as i32 then converted with icmp ne 0.
	isBoolInput := scalar.Kind == ir.ScalarBool
	loadOl := overloadForScalar(scalar)
	if isBoolInput {
		loadOl = overloadI32
	}

	numComps := componentCount(argType.Inner)
	loadFn := e.getDxOpLoadFunc(loadOl)
	exprHandle := e.findArgExprHandle(fn, argIdx)

	// Component-level DCE: determine which components are actually used.
	// DXC's ADCE only emits loadInput for consumed components. For vector
	// inputs accessed only via Swizzle/AccessIndex with known indices, we
	// can emit just the needed components.
	compMask := usedComponentMask(fn, exprHandle, numComps)

	// Initialize component storage if vector.
	if numComps > 1 {
		e.exprComponents[exprHandle] = make([]int, numComps)
	}

	for comp := 0; comp < numComps; comp++ {
		if compMask&(1<<comp) == 0 {
			continue
		}
		valueID := e.emitLoadInputCall(loadFn, inputRow, comp, isBoolInput)
		e.registerInputComponent(exprHandle, comp, numComps, valueID)
	}

	// Ensure exprValues points to the first loaded component.
	e.fixExprValuesForPartialLoad(exprHandle, numComps)
	return nil
}

// emitLoadInputCall emits a single dx.op.loadInput call for one component
// and returns the value ID. Handles bool-to-i32 conversion.
func (e *Emitter) emitLoadInputCall(loadFn *module.Function, inputID, comp int, isBoolInput bool) int {
	opcodeVal := e.getIntConstID(int64(OpLoadInput))
	inputIDVal := e.getIntConstID(int64(inputID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp))
	vertexIDVal := e.getUndefConstID()

	valueID := e.addCallInstr(loadFn, loadFn.FuncType.RetType,
		[]int{opcodeVal, inputIDVal, rowVal, colVal, vertexIDVal})

	if isBoolInput {
		zero := e.getIntConstID(0)
		valueID = e.addCmpInstr(ICmpNE, valueID, zero)
	}
	return valueID
}

// registerInputComponent stores a loaded component value in the expression
// value/component tracking maps.
func (e *Emitter) registerInputComponent(exprHandle ir.ExpressionHandle, comp, numComps, valueID int) {
	if numComps == 1 {
		e.exprValues[exprHandle] = valueID
		return
	}
	if comps, ok := e.exprComponents[exprHandle]; ok && comp < len(comps) {
		comps[comp] = valueID
	}
}

// fixExprValuesForPartialLoad ensures exprValues[handle] points to the first
// non-zero (loaded) component. Called after partial component loading where
// earlier components may have been skipped by DCE.
func (e *Emitter) fixExprValuesForPartialLoad(exprHandle ir.ExpressionHandle, numComps int) {
	if numComps <= 1 {
		return
	}
	comps, ok := e.exprComponents[exprHandle]
	if !ok {
		return
	}
	for _, v := range comps {
		if v != 0 {
			e.exprValues[exprHandle] = v
			return
		}
	}
}

// emitComputeBuiltinLoad emits dx.op thread ID intrinsic calls for compute
// shader builtins (GlobalInvocationId, LocalInvocationId, WorkGroupId, etc.).
//
// Each builtin maps to a specific dx.op intrinsic:
//   - GlobalInvocationId  → dx.op.threadId(i32 93, i32 component)
//   - LocalInvocationId   → dx.op.threadIdInGroup(i32 95, i32 component)
//   - WorkGroupId         → dx.op.groupId(i32 94, i32 component)
//   - LocalInvocationIndex → dx.op.flattenedThreadIdInGroup(i32 96)
//   - NumWorkGroups       → dx.op.groupId(i32 94, i32 component) [placeholder]
//
// Reference: Mesa nir_to_dxil.c emit_threadid_call() ~727,
// emit_threadidingroup_call() ~747, emit_groupid_call() ~789,
// emit_flattenedthreadidingroup_call() ~769
func (e *Emitter) emitComputeBuiltinLoad(fn *ir.Function, argIdx int, builtin ir.BuiltinValue) error {
	exprHandle := e.findArgExprHandle(fn, argIdx)

	switch builtin {
	case ir.BuiltinGlobalInvocationID:
		return e.emitVec3ThreadIDLoad(fn, exprHandle, "dx.op.threadId", OpThreadID)

	case ir.BuiltinLocalInvocationID:
		return e.emitVec3ThreadIDLoad(fn, exprHandle, "dx.op.threadIdInGroup", OpThreadIDInGroup)

	case ir.BuiltinWorkGroupID:
		return e.emitVec3ThreadIDLoad(fn, exprHandle, "dx.op.groupId", OpGroupID)

	case ir.BuiltinLocalInvocationIndex:
		return e.emitScalarThreadIDLoad(fn, exprHandle, "dx.op.flattenedThreadIdInGroup", OpFlattenedTIDInGroup)

	case ir.BuiltinNumWorkGroups:
		// NumWorkGroups is NOT a DXIL intrinsic. DXC implements it via a root
		// constant CBV injected by the runtime. We emit a synthetic CBV load.
		return e.emitNumWorkGroupsLoad(fn, exprHandle)

	case ir.BuiltinSubgroupInvocationID:
		// WaveGetLaneIndex() — returns the lane index within the wave.
		return e.emitScalarWaveLoad(fn, exprHandle, "dx.op.waveGetLaneIndex", OpWaveGetLaneIndex)

	case ir.BuiltinSubgroupSize:
		// WaveGetLaneCount() — returns the number of lanes in the wave.
		return e.emitScalarWaveLoad(fn, exprHandle, "dx.op.waveGetLaneCount", OpWaveGetLaneCount)

	case ir.BuiltinSubgroupID:
		// SubgroupId = flattenedThreadIdInGroup / WaveGetLaneCount()
		return e.emitSubgroupIDLoad(fn, exprHandle)

	case ir.BuiltinNumSubgroups:
		// NumSubgroups = (totalThreads + WaveGetLaneCount() - 1) / WaveGetLaneCount()
		return e.emitNumSubgroupsLoad(fn, exprHandle)

	default:
		return fmt.Errorf("unsupported compute builtin: %d", builtin)
	}
}

// emitVec3ThreadIDLoad emits 3 dx.op calls for a vec3<u32> thread ID builtin.
// Each component is loaded separately via: i32 @dx.op.NAME(i32 opcode, i32 component)
//
// Reference: Mesa nir_to_dxil.c emit_load_global_invocation_id() ~3131
func (e *Emitter) emitVec3ThreadIDLoad(_ *ir.Function, exprHandle ir.ExpressionHandle, funcName string, opcode DXILOpcode) error {
	i32Ty := e.mod.GetIntType(32)
	threadFn := e.getDxOpComputeBuiltinFunc(funcName, true)

	comps := make([]int, 3)
	for comp := 0; comp < 3; comp++ {
		opcodeVal := e.getIntConstID(int64(opcode))
		compVal := e.getIntConstID(int64(comp))
		valueID := e.addCallInstr(threadFn, i32Ty, []int{opcodeVal, compVal})
		comps[comp] = valueID
	}

	e.exprValues[exprHandle] = comps[0]
	e.exprComponents[exprHandle] = comps
	return nil
}

// emitScalarThreadIDLoad emits a single dx.op call for a scalar thread ID builtin.
// Signature: i32 @dx.op.NAME(i32 opcode)
//
// Reference: Mesa nir_to_dxil.c emit_flattenedthreadidingroup_call() ~769
func (e *Emitter) emitScalarThreadIDLoad(_ *ir.Function, exprHandle ir.ExpressionHandle, funcName string, opcode DXILOpcode) error {
	i32Ty := e.mod.GetIntType(32)
	threadFn := e.getDxOpComputeBuiltinFunc(funcName, false)

	opcodeVal := e.getIntConstID(int64(opcode))
	valueID := e.addCallInstr(threadFn, i32Ty, []int{opcodeVal})

	e.exprValues[exprHandle] = valueID
	return nil
}

// emitNumWorkGroupsLoad emits a synthetic CBV load for the NumWorkGroups builtin.
// DXIL has no intrinsic for num_workgroups. DXC implements it via a root constant
// CBV that the runtime fills with the dispatch dimensions (x, y, z as uint32).
// We create a synthetic CBV resource and load 3 components from it.
func (e *Emitter) emitNumWorkGroupsLoad(_ *ir.Function, exprHandle ir.ExpressionHandle) error {
	// Create or reuse a synthetic CBV for NumWorkGroups.
	handleID := e.getOrCreateNumWorkGroupsCBV()

	// Load register 0 from the CBV. The CBV contains {x, y, z} as 3 x i32.
	ol := overloadI32
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)

	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))
	regIdx := e.getIntConstID(0) // register 0 (first 16-byte slot)

	retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIdx})

	// Extract 3 components (x, y, z).
	i32Ty := e.mod.GetIntType(32)
	comps := make([]int, 3)
	for i := 0; i < 3; i++ {
		extractID := e.allocValue()
		e.currentBB.AddInstruction(&module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: i32Ty,
			Operands:   []int{retID, i},
			ValueID:    extractID,
		})
		comps[i] = extractID
	}

	e.exprValues[exprHandle] = comps[0]
	e.exprComponents[exprHandle] = comps
	return nil
}

// getOrCreateNumWorkGroupsCBV creates a synthetic CBV resource for NumWorkGroups.
// Returns the handle value ID. Uses a fixed register space and binding that
// the D3D12 runtime is expected to populate with dispatch dimensions.
func (e *Emitter) getOrCreateNumWorkGroupsCBV() int {
	if e.numWGHandleID >= 0 {
		return e.numWGHandleID
	}

	// Create a synthetic resource entry for the NumWorkGroups CBV.
	// Use a high register space (0xFFFFFFFF) to avoid colliding with user resources.
	// DXC uses $Globals CBV in space 0 — we follow the same pattern.
	res := resourceInfo{
		name:     "$Globals",
		class:    resourceClassCBV,
		rangeID:  len(e.resources), // next available rangeID
		group:    0,
		binding:  0, // b0 in space 0 (synthetic)
		handleID: -1,
	}

	// Check for collision with existing CBVs at b0/space0 — shift if needed.
	for _, r := range e.resources {
		if r.class == resourceClassCBV && r.group == 0 && r.binding == res.binding {
			res.binding++ // shift to next available binding
		}
	}

	e.resources = append(e.resources, res)

	// Create a static handle for this CBV.
	handleTy := e.getDxHandleType()
	createFn := e.getDxOpCreateHandleFunc()

	opcodeVal := e.getIntConstID(int64(OpCreateHandle))
	classVal := e.getI8ConstID(int64(resourceClassCBV))
	rangeIDVal := e.getIntConstID(int64(res.rangeID))
	indexVal := e.getIntConstID(int64(res.binding))
	nonUniformVal := e.getI1ConstID(0)

	handleID := e.addCallInstr(createFn, handleTy,
		[]int{opcodeVal, classVal, rangeIDVal, indexVal, nonUniformVal})

	e.numWGHandleID = handleID
	return handleID
}

// getDxOpComputeBuiltinFunc creates a dx.op function declaration for compute
// builtins. For vec3 builtins (hasComponent=true): i32 @name(i32, i32).
// For scalar builtins (hasComponent=false): i32 @name(i32).
func (e *Emitter) getDxOpComputeBuiltinFunc(name string, hasComponent bool) *module.Function {
	key := dxOpKey{name: name, overload: overloadI32}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	i32Ty := e.mod.GetIntType(32)
	fullName := name + suffixI32

	var params []*module.Type
	if hasComponent {
		params = []*module.Type{i32Ty, i32Ty} // opcode, component
	} else {
		params = []*module.Type{i32Ty} // opcode only
	}

	funcTy := e.mod.GetFunctionType(i32Ty, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStructInputLoads handles struct-typed inputs (e.g., fragment shader struct arguments).
// Each struct member with a binding becomes a separate loadInput call sequence.
// The inputRow parameter is the starting ISG1 row index; returns the number of
// signature elements consumed so the caller can advance the row counter.
//
// DXC emits loadInput calls for struct members in reverse ISG1 row order (last
// signature element first). We match this by first handling compute builtins
// (which don't produce loadInput), then emitting loadInput-producing members
// in reverse of their sorted (ISG1) order with correct sigId assignment.
//
// The loaded values are pre-registered on AccessIndex expressions that reference
// this struct's members, so downstream expression evaluation resolves them directly.
//
//nolint:gocognit,gocyclo,cyclop,funlen // dispatch over builtin/scalar/vector struct-member shapes
func (e *Emitter) emitStructInputLoads(fn *ir.Function, argIdx int, st *ir.StructType, stage ir.ShaderStage, inputRow int) error {
	// Find the expression handle for this FunctionArgument.
	argExprHandle := e.findArgExprHandle(fn, argIdx)

	type memberInfo struct {
		firstID int
		comps   []int
	}
	memberValues := make(map[int]*memberInfo, len(st.Members))

	isComputeLike := stage == ir.StageCompute || stage == ir.StageMesh || stage == ir.StageTask

	// Phase 1: Handle compute builtins (they don't produce loadInput).
	// Also identify which sorted members will produce loadInput calls and
	// their ISG1 row assignments.
	order := backend.SortedMemberIndices(st.Members)
	type loadMember struct {
		memberIdx int
		inputID   int // ISG1 row for this member
	}
	var loadMembers []loadMember
	rowCount := 0

	for _, memberIdx := range order {
		member := &st.Members[memberIdx]
		if member.Binding == nil {
			continue
		}

		// Compute-shader struct members carrying a compute builtin (e.g.
		// num_subgroups, subgroup_size) must NOT go through loadInput —
		// DXIL has no signature element for them and the validator
		// rejects the fabricated input IDs as out-of-range. Route each
		// one through its dedicated dx.op intrinsic and do NOT consume
		// an ISG1 row for it.
		if bb, ok := (*member.Binding).(ir.BuiltinBinding); ok && isComputeLike {
			valueID, err := e.emitComputeBuiltinMemberLoad(bb.Builtin)
			if err != nil {
				return fmt.Errorf("struct member %q: %w", member.Name, err)
			}
			memberValues[memberIdx] = &memberInfo{firstID: valueID, comps: []int{valueID}}
			continue
		}

		// Look up the ISG1 row from the precomputed map if available.
		// The map accounts for cross-argument sorting (locations before
		// builtins), so each member gets its correct packed register.
		// Fallback to sequential assignment for compute shaders and
		// cases where the map was not populated (emitSingleInputLoad path).
		memberRow := inputRow + rowCount
		if e.inputRowMap != nil {
			if r, ok := e.inputRowMap[inputRowKey{argIdx: argIdx, memberIdx: memberIdx}]; ok {
				memberRow = r
			}
		}
		loadMembers = append(loadMembers, loadMember{memberIdx: memberIdx, inputID: memberRow})
		rowCount++
	}

	// Phase 2: Emit loadInput calls in reverse ISG1 row order to match DXC.
	// DCE: if the entire struct argument is never referenced by any emitted
	// expression or statement, skip all loadInput calls for it.
	// DXC's LLVM ADCE removes dead loads; signature elements are retained.
	argReadable := isArgRead(fn, argIdx)
	for i := len(loadMembers) - 1; i >= 0; i-- {
		lm := loadMembers[i]
		member := &st.Members[lm.memberIdx]

		// Skip loading this member if the entire struct arg is dead.
		if !argReadable {
			continue
		}

		memberType := e.ir.Types[member.Type]
		scalar, ok := scalarOfType(memberType.Inner)
		if !ok {
			return fmt.Errorf("unsupported struct member type for input %q", member.Name)
		}

		numComps := componentCount(memberType.Inner)
		ol := overloadForScalar(scalar)
		loadFn := e.getDxOpLoadFunc(ol)

		info := &memberInfo{comps: make([]int, numComps)}
		for comp := 0; comp < numComps; comp++ {
			opcodeVal := e.getIntConstID(int64(OpLoadInput))
			inputIDVal := e.getIntConstID(int64(lm.inputID))
			rowVal := e.getIntConstID(0)
			colVal := e.getI8ConstID(int64(comp))
			vertexIDVal := e.getUndefConstID()

			valueID := e.addCallInstr(loadFn, loadFn.FuncType.RetType,
				[]int{opcodeVal, inputIDVal, rowVal, colVal, vertexIDVal})
			info.comps[comp] = valueID
			if comp == 0 {
				info.firstID = valueID
			}
		}
		memberValues[lm.memberIdx] = info
	}

	// Build a flat component list for the whole struct argument so that
	// downstream struct stores (e.g., "var input = input_original;") can
	// access all scalar components via getComponentID.
	var allComps []int
	for memberIdx := range st.Members {
		info, hasInfo := memberValues[memberIdx]
		if !hasInfo {
			// No binding — insert a placeholder zero per scalar component.
			memberType := e.ir.Types[st.Members[memberIdx].Type]
			numScalars := totalScalarCount(e.ir, memberType.Inner)
			for j := 0; j < numScalars; j++ {
				allComps = append(allComps, 0)
			}
			continue
		}
		allComps = append(allComps, info.comps...)
	}

	// Register the first loaded value on the FunctionArgument expression.
	// Also register all flat components so struct stores work correctly.
	if len(allComps) > 0 {
		e.exprValues[argExprHandle] = allComps[0]
		e.exprComponents[argExprHandle] = allComps
	} else {
		e.exprValues[argExprHandle] = 0
	}

	// Pre-register AccessIndex expressions that reference members of this struct.
	// This allows downstream expression evaluation to find loaded values directly.
	for exprIdx := range fn.Expressions {
		ai, ok := fn.Expressions[exprIdx].Kind.(ir.ExprAccessIndex)
		if !ok || ai.Base != argExprHandle {
			continue
		}
		info, hasInfo := memberValues[int(ai.Index)]
		if !hasInfo {
			continue
		}
		handle := ir.ExpressionHandle(exprIdx)
		e.exprValues[handle] = info.firstID
		if len(info.comps) > 1 {
			e.exprComponents[handle] = info.comps
		}
	}
	return nil
}

// emitOutputStore emits a dx.op.storeOutput call for a single output.
func (e *Emitter) emitOutputStore(outputID int, comp int, valueID int, ol overloadType) {
	storeFn := e.getDxOpStoreFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpStoreOutput))
	outputIDVal := e.getIntConstID(int64(outputID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp)) // col is i8

	e.addCallInstr(storeFn, e.mod.GetVoidType(),
		[]int{opcodeVal, outputIDVal, rowVal, colVal, valueID})
}

// findArgExprHandle finds the ExpressionHandle for a FunctionArgument with the given index.
func (e *Emitter) findArgExprHandle(fn *ir.Function, argIdx int) ir.ExpressionHandle {
	for exprIdx := range fn.Expressions {
		if fa, ok := fn.Expressions[exprIdx].Kind.(ir.ExprFunctionArgument); ok {
			if int(fa.Index) == argIdx {
				return ir.ExpressionHandle(exprIdx)
			}
		}
	}
	return 0
}

// builtinSemanticIndex maps naga builtins to DXIL semantic indices.
// For vertex shaders, SV_Position is output index 0.
func builtinSemanticIndex(b ir.BuiltinValue) int {
	switch b {
	case ir.BuiltinPosition:
		return 0
	case ir.BuiltinVertexIndex:
		return 1
	case ir.BuiltinInstanceIndex:
		return 2
	case ir.BuiltinFrontFacing:
		return 0
	case ir.BuiltinFragDepth:
		return 0
	default:
		return 0
	}
}

// --- Mesh Shader Intrinsic Emission ---

// emitSetMeshOutputCounts emits dx.op.setMeshOutputCounts(i32 168, i32 numVertices, i32 numPrimitives).
func (e *Emitter) emitSetMeshOutputCounts(vertexCountID, primitiveCountID int) {
	fn := e.getDxOpSetMeshOutputCountsFunc()
	opcodeVal := e.getIntConstID(int64(OpSetMeshOutputCounts))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, vertexCountID, primitiveCountID})
}

// getDxOpSetMeshOutputCountsFunc creates the dx.op.setMeshOutputCounts function declaration.
// void @dx.op.setMeshOutputCounts(i32 opcode, i32 numVertices, i32 numPrimitives)
func (e *Emitter) getDxOpSetMeshOutputCountsFunc() *module.Function {
	name := "dx.op.setMeshOutputCounts"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(name)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitEmitIndices emits dx.op.emitIndices(i32 169, i32 primIdx, i32 v0, i32 v1, i32 v2).
func (e *Emitter) emitEmitIndices(primitiveIdx, v0, v1, v2 int) {
	fn := e.getDxOpEmitIndicesFunc()
	opcodeVal := e.getIntConstID(int64(OpEmitIndices))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, primitiveIdx, v0, v1, v2})
}

// getDxOpEmitIndicesFunc creates the dx.op.emitIndices function declaration.
// void @dx.op.emitIndices(i32 opcode, i32 primIdx, i32 v0, i32 v1, i32 v2)
func (e *Emitter) getDxOpEmitIndicesFunc() *module.Function {
	name := "dx.op.emitIndices"
	key := dxOpKey{name: name, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(name, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(name)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStoreVertexOutput emits dx.op.storeVertexOutput.TYPE(i32 171, i32 sigId, i32 row, i8 col, TYPE value, i32 vertexIdx).
func (e *Emitter) emitStoreVertexOutput(sigID, comp, valueID, vertexIdx int, ol overloadType) {
	fn := e.getDxOpStoreVertexOutputFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpStoreVertexOutput))
	sigIDVal := e.getIntConstID(int64(sigID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, sigIDVal, rowVal, colVal, valueID, vertexIdx})
}

// getDxOpStoreVertexOutputFunc creates the dx.op.storeVertexOutput function declaration.
// void @dx.op.storeVertexOutput.TYPE(i32 opcode, i32 sigId, i32 row, i8 col, TYPE value, i32 vertexIdx)
func (e *Emitter) getDxOpStoreVertexOutputFunc(ol overloadType) *module.Function {
	name := "dx.op.storeVertexOutput"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valueTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, valueTy, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitStorePrimitiveOutput emits dx.op.storePrimitiveOutput.TYPE(i32 172, i32 sigId, i32 row, i8 col, TYPE value, i32 primIdx).
func (e *Emitter) emitStorePrimitiveOutput(sigID, comp, valueID, primIdx int, ol overloadType) {
	fn := e.getDxOpStorePrimitiveOutputFunc(ol)
	opcodeVal := e.getIntConstID(int64(OpStorePrimitiveOutput))
	sigIDVal := e.getIntConstID(int64(sigID))
	rowVal := e.getIntConstID(0)
	colVal := e.getI8ConstID(int64(comp))
	e.addCallInstr(fn, e.mod.GetVoidType(), []int{opcodeVal, sigIDVal, rowVal, colVal, valueID, primIdx})
}

// getDxOpStorePrimitiveOutputFunc creates the dx.op.storePrimitiveOutput function declaration.
func (e *Emitter) getDxOpStorePrimitiveOutputFunc(ol overloadType) *module.Function {
	name := "dx.op.storePrimitiveOutput"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valueTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, valueTy, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitScalarWaveLoad emits a scalar wave intrinsic that takes only (i32 opcode) and returns i32.
// Used for WaveGetLaneIndex (111) and WaveGetLaneCount (112).
func (e *Emitter) emitScalarWaveLoad(_ *ir.Function, exprHandle ir.ExpressionHandle, funcName string, opcode DXILOpcode) error {
	i32Ty := e.mod.GetIntType(32)
	key := dxOpKey{name: funcName, overload: overloadI32}
	fn, ok := e.dxOpFuncs[key]
	if !ok {
		params := []*module.Type{i32Ty}
		funcTy := e.mod.GetFunctionType(i32Ty, params)
		fn = e.mod.AddFunction(funcName, funcTy, true)
		e.dxOpFuncs[key] = fn
	}

	opcodeVal := e.getIntConstID(int64(opcode))
	resultID := e.addCallInstr(fn, i32Ty, []int{opcodeVal})
	e.exprValues[exprHandle] = resultID
	return nil
}

// getOrCreateWaveGetLaneCountFunc declares (or returns the cached)
// dx.op.waveGetLaneCount intrinsic. DXC's DxilOperations.cpp:1015 declares
// WaveGetLaneCount with overload count 0 (Overloads: v), so the function
// symbol carries NO type suffix — it is exactly "dx.op.waveGetLaneCount".
// Multiple call sites need this declaration; centralizing it here keeps
// the cache key consistent and avoids the .i32 trap that
// getDxOpComputeBuiltinFunc would introduce.
func (e *Emitter) getOrCreateWaveGetLaneCountFunc() *module.Function {
	key := dxOpKey{name: "dx.op.waveGetLaneCount", overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	funcTy := e.mod.GetFunctionType(i32Ty, []*module.Type{i32Ty})
	fn := e.mod.AddFunction("dx.op.waveGetLaneCount", funcTy, true)
	e.dxOpFuncs[key] = fn
	return fn
}

// emitSubgroupIDLoad emits: flattenedThreadIdInGroup / WaveGetLaneCount()
// DXC/HLSL translates SubgroupId as local_invocation_index / WaveGetLaneCount().
//
// Both intrinsic function symbols must include the ".i32" overload suffix per
// DXC OP::ConstructOverloadName ("dx.op." + className + "." + overloadTypeName,
// DxilOperations.cpp:3219). Historically this helper declared them without the
// suffix, which produced "'dx.op.flattenedThreadIdInGroup' is not a
// DXILOpFuncition" validator errors when this path ran before
// getDxOpComputeBuiltinFunc populated the shared dxOpFuncs cache with the
// correctly-suffixed variant. Use the shared helper to keep one source of
// truth for the declaration.
func (e *Emitter) emitSubgroupIDLoad(_ *ir.Function, exprHandle ir.ExpressionHandle) error {
	i32Ty := e.mod.GetIntType(32)

	// flattenedThreadIdInGroup (opcode 96) — DXC OpClass
	// "flattenedThreadIdInGroup" with overload mask 0x40 (i32), so the
	// canonical symbol is "dx.op.flattenedThreadIdInGroup.i32" per
	// OP::ConstructOverloadName. Routed through the shared compute-builtin
	// helper which appends the .i32 suffix.
	flatFn := e.getDxOpComputeBuiltinFunc("dx.op.flattenedThreadIdInGroup", false)
	flatOp := e.getIntConstID(int64(OpFlattenedTIDInGroup))
	flatID := e.addCallInstr(flatFn, i32Ty, []int{flatOp})

	// WaveGetLaneCount (opcode 112) — DxilOperations.cpp:1015 has overload
	// count 0 / "Overloads: v", meaning NO type overload — the canonical
	// symbol is just "dx.op.waveGetLaneCount" without any suffix. We
	// therefore declare it manually here (the compute-builtin helper would
	// always append .i32 and produce a name the validator does not know).
	lcFn := e.getOrCreateWaveGetLaneCountFunc()
	lcOp := e.getIntConstID(int64(OpWaveGetLaneCount))
	lcID := e.addCallInstr(lcFn, i32Ty, []int{lcOp})

	// UDiv: flattenedThreadIdInGroup / WaveGetLaneCount
	resultID := e.addBinOpInstr(i32Ty, BinOpUDiv, flatID, lcID)
	e.exprValues[exprHandle] = resultID
	return nil
}

// emitNumSubgroupsLoad emits: (totalThreads + WaveGetLaneCount() - 1) / WaveGetLaneCount()
// where totalThreads = workgroup_size.x * workgroup_size.y * workgroup_size.z.
func (e *Emitter) emitNumSubgroupsLoad(_ *ir.Function, exprHandle ir.ExpressionHandle) error {
	i32Ty := e.mod.GetIntType(32)

	// Get workgroup size from the entry point.
	ep := &e.ir.EntryPoints[0]
	totalThreads := int64(ep.Workgroup[0]) * int64(ep.Workgroup[1]) * int64(ep.Workgroup[2])
	totalID := e.getIntConstID(totalThreads)

	// WaveGetLaneCount (opcode 112) — no overload suffix. See
	// getOrCreateWaveGetLaneCountFunc for the rationale.
	lcFn := e.getOrCreateWaveGetLaneCountFunc()
	lcOp := e.getIntConstID(int64(OpWaveGetLaneCount))
	lcID := e.addCallInstr(lcFn, i32Ty, []int{lcOp})

	// (totalThreads + laneCount - 1) / laneCount
	oneID := e.getIntConstID(1)
	addID := e.addBinOpInstr(i32Ty, BinOpAdd, totalID, lcID)
	subID := e.addBinOpInstr(i32Ty, BinOpSub, addID, oneID)
	resultID := e.addBinOpInstr(i32Ty, BinOpUDiv, subID, lcID)

	e.exprValues[exprHandle] = resultID
	return nil
}

// emitComputeBuiltinMemberLoad emits the dx.op intrinsic for a compute
// builtin appearing as a *struct member* of the entry point argument, and
// returns the resulting scalar value ID. Mirrors the per-argument cases in
// emitComputeBuiltinLoad, but returns a value instead of binding it to a
// function-argument expression handle — the caller wires it up to the
// member's entry in the struct's pre-registered component list.
//
// Scope: scalar builtins only. Vector builtins (global_invocation_id etc.)
// are not expected as struct members in practice and are rejected with an
// unsupported error rather than silently producing a scalar for the first
// lane only.
func (e *Emitter) emitComputeBuiltinMemberLoad(builtin ir.BuiltinValue) (int, error) {
	i32Ty := e.mod.GetIntType(32)

	switch builtin {
	case ir.BuiltinSubgroupInvocationID:
		fn := e.getDxOpComputeBuiltinFunc("dx.op.waveGetLaneIndex", false)
		op := e.getIntConstID(int64(OpWaveGetLaneIndex))
		return e.addCallInstr(fn, i32Ty, []int{op}), nil

	case ir.BuiltinSubgroupSize:
		// DxilOperations.cpp:1015 — WaveGetLaneCount has overload count 0
		// so the symbol is exactly "dx.op.waveGetLaneCount" (no suffix).
		fn := e.getOrCreateWaveGetLaneCountFunc()
		op := e.getIntConstID(int64(OpWaveGetLaneCount))
		return e.addCallInstr(fn, i32Ty, []int{op}), nil

	case ir.BuiltinSubgroupID:
		// SubgroupId = flattenedThreadIdInGroup / WaveGetLaneCount
		flatFn := e.getDxOpComputeBuiltinFunc("dx.op.flattenedThreadIdInGroup", false)
		flatOp := e.getIntConstID(int64(OpFlattenedTIDInGroup))
		flatID := e.addCallInstr(flatFn, i32Ty, []int{flatOp})

		lcFn := e.getOrCreateWaveGetLaneCountFunc()
		lcOp := e.getIntConstID(int64(OpWaveGetLaneCount))
		lcID := e.addCallInstr(lcFn, i32Ty, []int{lcOp})

		return e.addBinOpInstr(i32Ty, BinOpUDiv, flatID, lcID), nil

	case ir.BuiltinNumSubgroups:
		// NumSubgroups = ceil(totalThreads / WaveGetLaneCount)
		//              = (totalThreads + lc - 1) / lc
		ep := &e.ir.EntryPoints[0]
		totalThreads := int64(ep.Workgroup[0]) * int64(ep.Workgroup[1]) * int64(ep.Workgroup[2])
		totalID := e.getIntConstID(totalThreads)

		lcFn := e.getOrCreateWaveGetLaneCountFunc()
		lcOp := e.getIntConstID(int64(OpWaveGetLaneCount))
		lcID := e.addCallInstr(lcFn, i32Ty, []int{lcOp})

		oneID := e.getIntConstID(1)
		addID := e.addBinOpInstr(i32Ty, BinOpAdd, totalID, lcID)
		subID := e.addBinOpInstr(i32Ty, BinOpSub, addID, oneID)
		return e.addBinOpInstr(i32Ty, BinOpUDiv, subID, lcID), nil

	case ir.BuiltinLocalInvocationIndex:
		fn := e.getDxOpComputeBuiltinFunc("dx.op.flattenedThreadIdInGroup", false)
		op := e.getIntConstID(int64(OpFlattenedTIDInGroup))
		return e.addCallInstr(fn, i32Ty, []int{op}), nil

	default:
		return 0, fmt.Errorf("compute builtin %d not supported as struct member (scalar only)", builtin)
	}
}

// TODO: emitGetMeshPayload — implement when task payload access is needed.
// Signature: TYPE* @dx.op.getMeshPayload.TYPE(i32 170)

// isArgRead reports whether any expression or statement in fn's body
// references ExprFunctionArgument(argIdx) — directly, or transitively via
// AccessIndex/Access chains, Loads, Composes, function call arguments, etc.
// Used by emitInputLoads to skip the dx.op intrinsic call for compute
// builtins whose result is never consumed (DCE parity with DXC's downstream
// LLVM ADCE pass).
//
// The check walks fn.Expressions and looks at every operand-style field that
// holds an ExpressionHandle. If any expression references the arg's handle
// AND that expression is itself reachable from a statement (via StmtEmit
// range or direct statement operand), the arg is "read".
//
// We use a coarse-but-safe approximation: any ExpressionHandle in any
// expression field that equals the arg's expression handle counts as a read.
// False positives are safe (we just emit the load when not strictly needed);
// false negatives would silently drop required values.
func isComputeLikeStage(stage ir.ShaderStage) bool {
	return stage == ir.StageCompute || stage == ir.StageMesh || stage == ir.StageTask
}

// tryEmitComputeBuiltin handles compute/mesh/task stage builtin args.
// Returns (handled=true, _) if the arg was a compute-like builtin (whether
// or not the dx.op intrinsic was actually emitted — DCE may elide it).
// Returns (false, nil) for non-builtin or non-compute-like cases so the
// caller can continue to other binding kinds.
func (e *Emitter) tryEmitComputeBuiltin(fn *ir.Function, argIdx int, arg *ir.FunctionArgument, stage ir.ShaderStage) (bool, error) {
	if !isComputeLikeStage(stage) {
		return false, nil
	}
	bb, ok := (*arg.Binding).(ir.BuiltinBinding)
	if !ok {
		return false, nil
	}
	// DCE-style elision: skip the dx.op intrinsic when the builtin arg
	// is unread. DXC's downstream LLVM ADCE drops these, so the roundtrip
	// golden lacks them. The compute thread/groupId intrinsics are
	// readnone so dropping is observably correct.
	if !isArgRead(fn, argIdx) {
		return true, nil
	}
	if err := e.emitComputeBuiltinLoad(fn, argIdx, bb.Builtin); err != nil {
		return true, err
	}
	return true, nil
}

func isArgRead(fn *ir.Function, argIdx int) bool {
	argHandle, ok := findArgHandle(fn, argIdx)
	if !ok {
		return false
	}
	// Check only expressions within live StmtEmit ranges in the
	// (possibly DCE-swept) body. Walking ALL expressions would
	// catch dead ones whose emit ranges were removed by the IR-level
	// DCE pass, causing the emitter to emit threadId/groupId calls
	// whose results are never consumed.
	emitted := collectEmittedExprs(fn.Body)
	for h := range emitted {
		if h == argHandle {
			continue
		}
		if expressionReferences(fn.Expressions[h].Kind, argHandle) {
			return true
		}
	}
	return statementsReference(fn.Body, argHandle)
}

// collectEmittedExprs gathers every expression handle that falls within
// a StmtEmit range anywhere in the (post-DCE) body. Only these handles
// will actually be emitted as DXIL instructions; expressions outside
// any emit range are dead.
func collectEmittedExprs(block ir.Block) map[ir.ExpressionHandle]bool {
	m := make(map[ir.ExpressionHandle]bool)
	collectEmitsBlock(block, m)
	return m
}

func collectEmitsBlock(block ir.Block, out map[ir.ExpressionHandle]bool) {
	for _, st := range block {
		switch sk := st.Kind.(type) {
		case ir.StmtEmit:
			for h := sk.Range.Start; h < sk.Range.End; h++ {
				out[h] = true
			}
		case ir.StmtIf:
			collectEmitsBlock(sk.Accept, out)
			collectEmitsBlock(sk.Reject, out)
		case ir.StmtLoop:
			collectEmitsBlock(sk.Body, out)
			collectEmitsBlock(sk.Continuing, out)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				collectEmitsBlock(sk.Cases[ci].Body, out)
			}
		case ir.StmtBlock:
			collectEmitsBlock(sk.Block, out)
		}
	}
}

// usedComponentMask determines which components of a vector-typed expression
// are actually consumed by live expressions/statements. Returns a bitmask
// where bit N means component N is used. If the vector is used in any way
// that requires all components (e.g., passed to a binary op, composed into
// another vector, used as a function argument), returns all-bits-set.
//
// This enables component-level DCE to match DXC's ADCE behavior: DXC only
// emits loadInput for vector components that are actually consumed.
func usedComponentMask(fn *ir.Function, exprHandle ir.ExpressionHandle, numComps int) uint32 {
	allMask := uint32((1 << numComps) - 1)
	emitted := collectEmittedExprs(fn.Body)

	var mask uint32
	for h := range emitted {
		kind := fn.Expressions[h].Kind
		switch k := kind.(type) {
		case ir.ExprSwizzle:
			if k.Vector == exprHandle {
				// Only the swizzled components are used.
				for i := 0; i < int(k.Size); i++ {
					mask |= 1 << uint(k.Pattern[i])
				}
			}
		case ir.ExprAccessIndex:
			if k.Base == exprHandle {
				// Direct index access -- only that component is used.
				if int(k.Index) < numComps {
					mask |= 1 << k.Index
				} else {
					return allMask
				}
			}
		case ir.ExprAccess:
			if k.Base == exprHandle {
				// Dynamic index -- all components might be needed.
				return allMask
			}
		default:
			// If the vector handle is referenced by any other expression kind
			// (binary op, compose, math, image sample, etc.), all components
			// are potentially needed.
			if expressionReferences(kind, exprHandle) {
				// Check if this is a Swizzle/AccessIndex (already handled above).
				// For all other reference types, assume all components needed.
				return allMask
			}
		}
	}
	// Also check statement-level references (store, call, return).
	if statementsReference(fn.Body, exprHandle) {
		return allMask
	}

	if mask == 0 {
		// No components referenced at all -- shouldn't happen if isArgRead was true,
		// but return all to be safe.
		return allMask
	}
	return mask
}

func findArgHandle(fn *ir.Function, argIdx int) (ir.ExpressionHandle, bool) {
	for h, expr := range fn.Expressions {
		if fa, isArg := expr.Kind.(ir.ExprFunctionArgument); isArg && int(fa.Index) == argIdx {
			return ir.ExpressionHandle(h), true
		}
	}
	return 0, false
}

func anyHandleEq(handles []ir.ExpressionHandle, target ir.ExpressionHandle) bool {
	for _, h := range handles {
		if h == target {
			return true
		}
	}
	return false
}

func ptrHandleEq(p *ir.ExpressionHandle, target ir.ExpressionHandle) bool {
	return p != nil && *p == target
}

func expressionReferences(kind ir.ExpressionKind, target ir.ExpressionHandle) bool {
	switch k := kind.(type) {
	case ir.ExprAccess:
		return k.Base == target || k.Index == target
	case ir.ExprAccessIndex:
		return k.Base == target
	case ir.ExprBinary:
		return k.Left == target || k.Right == target
	case ir.ExprSelect:
		return k.Condition == target || k.Accept == target || k.Reject == target
	case ir.ExprMath:
		return k.Arg == target || ptrHandleEq(k.Arg1, target) || ptrHandleEq(k.Arg2, target) || ptrHandleEq(k.Arg3, target)
	case ir.ExprCompose:
		return anyHandleEq(k.Components, target)
	case ir.ExprImageSample, ir.ExprImageLoad, ir.ExprImageQuery:
		return expressionReferencesImage(kind, target)
	case ir.ExprAlias:
		return k.Source == target
	}
	return expressionReferencesUnary(kind, target)
}

// expressionReferencesImage checks if an image expression references the target.
func expressionReferencesImage(kind ir.ExpressionKind, target ir.ExpressionHandle) bool {
	switch k := kind.(type) {
	case ir.ExprImageSample:
		return k.Image == target || k.Sampler == target || k.Coordinate == target ||
			ptrHandleEq(k.ArrayIndex, target) || ptrHandleEq(k.Offset, target) ||
			ptrHandleEq(k.DepthRef, target) || sampleLevelReferences(k.Level, target)
	case ir.ExprImageLoad:
		return k.Image == target || k.Coordinate == target ||
			ptrHandleEq(k.ArrayIndex, target) || ptrHandleEq(k.Sample, target) ||
			ptrHandleEq(k.Level, target)
	case ir.ExprImageQuery:
		return k.Image == target || imageQueryReferences(k.Query, target)
	}
	return false
}

// sampleLevelReferences checks if a SampleLevel references the target handle.
func sampleLevelReferences(level ir.SampleLevel, target ir.ExpressionHandle) bool {
	switch l := level.(type) {
	case ir.SampleLevelExact:
		return l.Level == target
	case ir.SampleLevelBias:
		return l.Bias == target
	case ir.SampleLevelGradient:
		return l.X == target || l.Y == target
	}
	return false
}

// imageQueryReferences checks if an ImageQuery references the target handle.
func imageQueryReferences(query ir.ImageQuery, target ir.ExpressionHandle) bool {
	switch q := query.(type) {
	case ir.ImageQuerySize:
		return ptrHandleEq(q.Level, target)
	}
	return false
}

func expressionReferencesUnary(kind ir.ExpressionKind, target ir.ExpressionHandle) bool {
	switch k := kind.(type) {
	case ir.ExprLoad:
		return k.Pointer == target
	case ir.ExprUnary:
		return k.Expr == target
	case ir.ExprAs:
		return k.Expr == target
	case ir.ExprSplat:
		return k.Value == target
	case ir.ExprDerivative:
		return k.Expr == target
	case ir.ExprRelational:
		return k.Argument == target
	case ir.ExprArrayLength:
		return k.Array == target
	case ir.ExprSwizzle:
		return k.Vector == target
	}
	return false
}

func statementReferences(kind ir.StatementKind, target ir.ExpressionHandle) bool {
	switch sk := kind.(type) {
	case ir.StmtStore:
		return sk.Pointer == target || sk.Value == target
	case ir.StmtCall:
		return anyHandleEq(sk.Arguments, target)
	case ir.StmtAtomic:
		return sk.Pointer == target
	case ir.StmtReturn:
		return ptrHandleEq(sk.Value, target)
	case ir.StmtIf:
		return sk.Condition == target ||
			statementsReference(sk.Accept, target) ||
			statementsReference(sk.Reject, target)
	case ir.StmtLoop:
		return statementsReference(sk.Body, target) || statementsReference(sk.Continuing, target)
	case ir.StmtBlock:
		return statementsReference(sk.Block, target)
	case ir.StmtSwitch:
		if sk.Selector == target {
			return true
		}
		for _, c := range sk.Cases {
			if statementsReference(c.Body, target) {
				return true
			}
		}
	}
	return false
}

func statementsReference(block ir.Block, target ir.ExpressionHandle) bool {
	for _, st := range block {
		if statementReferences(st.Kind, target) {
			return true
		}
	}
	return false
}
