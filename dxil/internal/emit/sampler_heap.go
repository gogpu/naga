package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// Sampler heap binding model — DXIL backend mirror of the HLSL backend's
// nagaSamplerHeap pattern. Aligns with wgpu/hal/dx12 which binds samplers
// through descriptor heaps, not per-WGSL-binding root parameter slots.
//
// Model (matches HLSL backend hlsl/types.go writeSamplerHeaps +
// writeSamplerIndexBuffer):
//
//   * One global SamplerState array per sampler kind:
//     - nagaSamplerHeap[2048] at register(s0, space0) — non-comparison
//     - nagaComparisonSamplerHeap[2048] at register(s0, space1) — comparison
//   * Per-bind-group StructuredBuffer<uint> nagaGroup<N>SamplerIndexArray at
//     register(t<N>, space255) — each WGSL sampler @binding(M) reads slot M
//     of this buffer to discover its position in the global sampler heap.
//   * At each sample site, the sampler handle is created by:
//     1. dx.op.bufferLoad on the index buffer at offset M (where M is the
//        WGSL sampler's @binding number)
//     2. extractvalue + add (the dxc HLSL→DXIL lowering inserts add 0; we
//        match for byte-cmp parity though the add is a no-op semantically)
//     3. dx.op.createHandle with class=Sampler(3), rangeID = the heap's
//        rangeID, index = the loaded value
//
// Reference: dx.op.createHandle (opcode 57, class=Sampler) with a runtime
// index is the SM 6.0 path that DXC emits when lowering nagaSamplerHeap[].
// DXC's HLSL→DXIL lowerer (HLOperationLower.cpp) does NOT use
// createHandleFromHeap (opcode 218, SM 6.6+) for this pattern — that op is
// reserved for the explicit `SamplerDescriptorHeap[]` HLSL syntax. Mesa's
// nir_to_dxil.c does not emit this pattern at all (driver path).

// Default register/space targets for the synthesized heap entries. These
// match the HLSL backend defaults so dxc -dumpbin output matches the
// generated naga HLSL → DXC golden byte-for-byte.
const (
	samplerHeapDefaultRegister           uint32 = 0
	samplerHeapDefaultSpace              uint32 = 0
	comparisonSamplerHeapDefaultRegister uint32 = 0
	comparisonSamplerHeapDefaultSpace    uint32 = 1
	samplerIndexBufferDefaultSpace       uint32 = 255
)

// samplerHeapRegister returns the register for the standard sampler heap,
// consulting the caller-provided SamplerHeapTargets override first.
func (e *Emitter) samplerHeapRegister() uint32 {
	if t := e.opts.SamplerHeapTargets; t != nil {
		return t.StandardSamplers.Register
	}
	return samplerHeapDefaultRegister
}

// samplerHeapSpace returns the space for the standard sampler heap.
func (e *Emitter) samplerHeapSpace() uint32 {
	if t := e.opts.SamplerHeapTargets; t != nil {
		return t.StandardSamplers.Space
	}
	return samplerHeapDefaultSpace
}

// comparisonSamplerHeapRegister returns the register for the comparison
// sampler heap.
func (e *Emitter) comparisonSamplerHeapRegister() uint32 {
	if t := e.opts.SamplerHeapTargets; t != nil {
		return t.ComparisonSamplers.Register
	}
	return comparisonSamplerHeapDefaultRegister
}

// comparisonSamplerHeapSpace returns the space for the comparison sampler
// heap.
func (e *Emitter) comparisonSamplerHeapSpace() uint32 {
	if t := e.opts.SamplerHeapTargets; t != nil {
		return t.ComparisonSamplers.Space
	}
	return comparisonSamplerHeapDefaultSpace
}

// samplerIndexBufferTarget returns (space, register) for a per-group
// sampler index buffer SRV, consulting SamplerBufferBindingMap first.
func (e *Emitter) samplerIndexBufferTarget(group uint32) (space, register uint32) {
	if m := e.opts.SamplerBufferBindingMap; m != nil {
		if tgt, ok := m[group]; ok {
			return tgt.Space, tgt.Register
		}
	}
	return samplerIndexBufferDefaultSpace, group
}

// samplerHeapState tracks per-emit-pass sampler-heap rewriting state.
type samplerHeapState struct {
	// hasStandard is true when at least one non-comparison sampler is
	// present in the reachable globals. Causes a SamplerHeap entry at
	// (s0, space0) to be synthesized.
	hasStandard bool
	// hasComparison is true when at least one comparison sampler is
	// present. Causes a ComparisonSamplerHeap entry at (s0, space1).
	hasComparison bool

	// groupsWithSamplers is the deterministic-ordered list of bind
	// groups that contain at least one sampler. Each group gets its
	// own structured-buffer SRV index array.
	groupsWithSamplers []uint32

	// resIdxStandardHeap, resIdxComparisonHeap, and resIdxIndexBuffer
	// are indices into Emitter.resources after analyzeResources has
	// finished. -1 if not synthesized. resIdxIndexBuffer is keyed by
	// group number.
	resIdxStandardHeap   int
	resIdxComparisonHeap int
	resIdxIndexBuffer    map[uint32]int

	// samplerWGSLBinding maps each WGSL sampler global variable handle
	// to the (group, binding) pair it occupies in its index buffer.
	// Drives the per-sampler bufferLoad+createHandle pair emitted by
	// emitResourceHandles (which writes the resulting sampler handle
	// back into Emitter.resourceHandles so resolveResourceHandle picks
	// it up at sample sites).
	samplerWGSLBinding map[ir.GlobalVariableHandle]samplerHeapBinding
}

type samplerHeapBinding struct {
	group      uint32
	binding    uint32 // raw WGSL @binding number — used as the index-buffer offset
	comparison bool
}

// detectSamplerHeapNeeded walks the reachable globals once and returns a
// fully-populated samplerHeapState (with synthesized resource indices left
// as -1 placeholders) when at least one sampler is present, or nil when
// the module has no samplers and the direct-binding path is sufficient.
func (e *Emitter) detectSamplerHeapNeeded() *samplerHeapState {
	st := &samplerHeapState{
		resIdxStandardHeap:   -1,
		resIdxComparisonHeap: -1,
		resIdxIndexBuffer:    make(map[uint32]int),
		samplerWGSLBinding:   make(map[ir.GlobalVariableHandle]samplerHeapBinding),
	}

	groupSet := make(map[uint32]bool)

	for i := range e.ir.GlobalVariables {
		gv := &e.ir.GlobalVariables[i]
		if e.reachableGlobals != nil && !e.reachableGlobals[ir.GlobalVariableHandle(i)] {
			continue
		}
		if gv.Space != ir.SpaceHandle || gv.Binding == nil {
			continue
		}
		// Skip binding_array<sampler> — keep the existing direct-binding
		// path until we wire the per-sample sampler-array indirection
		// (HLSL backend does it through samplerBindingArrayInfoFromExpression
		// at every sample site; DXIL would mirror with a per-sample
		// bufferLoad indexed by the dynamic array index).
		if int(gv.Type) < len(e.ir.Types) {
			if _, isBA := e.ir.Types[gv.Type].Inner.(ir.BindingArrayType); isBA {
				continue
			}
		}
		samplerKind, ok := samplerKindOfGlobal(e.ir, gv)
		if !ok {
			continue
		}

		if samplerKind.comparison {
			st.hasComparison = true
		} else {
			st.hasStandard = true
		}
		group := gv.Binding.Group
		if !groupSet[group] {
			groupSet[group] = true
			st.groupsWithSamplers = append(st.groupsWithSamplers, group)
		}
		st.samplerWGSLBinding[ir.GlobalVariableHandle(i)] = samplerHeapBinding{
			group:      group,
			binding:    gv.Binding.Binding,
			comparison: samplerKind.comparison,
		}
	}

	if !st.hasStandard && !st.hasComparison {
		return nil
	}

	// Sort groupsWithSamplers ascending for deterministic register
	// assignment and metadata ordering. Bind groups are small (< 16)
	// so an O(n^2) insertion sort is fine and avoids pulling in sort.
	for i := 1; i < len(st.groupsWithSamplers); i++ {
		for j := i; j > 0 && st.groupsWithSamplers[j-1] > st.groupsWithSamplers[j]; j-- {
			st.groupsWithSamplers[j-1], st.groupsWithSamplers[j] = st.groupsWithSamplers[j], st.groupsWithSamplers[j-1]
		}
	}

	return st
}

type samplerKindInfo struct {
	comparison bool
}

// samplerKindOfGlobal returns (info, true) when gv's type is a SamplerType
// (possibly wrapped in a BindingArrayType), or (zero, false) otherwise.
func samplerKindOfGlobal(mod *ir.Module, gv *ir.GlobalVariable) (samplerKindInfo, bool) {
	if int(gv.Type) >= len(mod.Types) {
		return samplerKindInfo{}, false
	}
	inner := mod.Types[gv.Type].Inner
	if ba, ok := inner.(ir.BindingArrayType); ok {
		if int(ba.Base) >= len(mod.Types) {
			return samplerKindInfo{}, false
		}
		inner = mod.Types[ba.Base].Inner
	}
	st, ok := inner.(ir.SamplerType)
	if !ok {
		return samplerKindInfo{}, false
	}
	return samplerKindInfo{comparison: st.Comparison}, true
}

// insertSamplerIndexBufferSRV synthesizes the per-group StructuredBuffer<uint>
// SRV entry at the current position in the resource list. Called from the
// main classify loop in analyzeResources when the first sampler of a group
// is encountered — matching the HLSL backend which declares the index buffer
// at the point where the first sampler triggers writeSamplerIndexBuffer.
// This gives DXC the same SRV rangeID as our emitter.
func (e *Emitter) insertSamplerIndexBufferSRV(group uint32, rangeCounters *[4]int) {
	if e.samplerHeap == nil {
		return
	}
	st := e.samplerHeap

	idx := len(e.resources)
	st.resIdxIndexBuffer[group] = idx
	idxSpace, idxRegister := e.samplerIndexBufferTarget(group)
	info := resourceInfo{
		varHandle:    ir.GlobalVariableHandle(0xffffffff),
		name:         fmt.Sprintf("nagaGroup%dSamplerIndexArray", group),
		class:        resourceClassSRV,
		rangeID:      rangeCounters[resourceClassSRV],
		group:        idxSpace,
		binding:      idxRegister,
		typeHandle:   ir.TypeHandle(0xffffffff),
		handleID:     -1,
		kindOverride: 12, // StructuredBuffer
	}
	rangeCounters[resourceClassSRV]++
	e.resources = append(e.resources, info)
}

// appendSamplerHeapSamplers synthesizes the sampler array heap entries
// (nagaSamplerHeap and/or nagaComparisonSamplerHeap) AFTER the main classify
// loop. Sampler rangeIDs come after any real sampler resources (though in
// sampler-heap mode all real samplers are skipped, so these get rangeID=0).
func (e *Emitter) appendSamplerHeapSamplers(rangeCounters *[4]int) {
	if e.samplerHeap == nil {
		return
	}
	st := e.samplerHeap

	if st.hasStandard {
		st.resIdxStandardHeap = len(e.resources)
		info := resourceInfo{
			varHandle:      ir.GlobalVariableHandle(0xffffffff), // synthetic — no IR backing
			name:           "nagaSamplerHeap",
			class:          resourceClassSampler,
			rangeID:        rangeCounters[resourceClassSampler],
			group:          e.samplerHeapSpace(),
			binding:        e.samplerHeapRegister(),
			typeHandle:     ir.TypeHandle(0xffffffff),
			handleID:       -1,
			isBindingArray: true, // 2048-slot array — drives metadata struct wrapping
			arraySize:      2048,
		}
		rangeCounters[resourceClassSampler]++
		e.resources = append(e.resources, info)
	}
	if st.hasComparison {
		st.resIdxComparisonHeap = len(e.resources)
		info := resourceInfo{
			varHandle:         ir.GlobalVariableHandle(0xffffffff),
			name:              "nagaComparisonSamplerHeap",
			class:             resourceClassSampler,
			rangeID:           rangeCounters[resourceClassSampler],
			group:             e.comparisonSamplerHeapSpace(),
			binding:           e.comparisonSamplerHeapRegister(),
			typeHandle:        ir.TypeHandle(0xffffffff),
			handleID:          -1,
			isBindingArray:    true,
			arraySize:         2048,
			comparisonSampler: true,
		}
		rangeCounters[resourceClassSampler]++
		e.resources = append(e.resources, info)
	}
}

// emitSamplerHeapHandles materializes a per-WGSL-sampler %dx.types.Handle
// after the regular createHandle calls in emitResourceHandles. For each
// WGSL sampler global, emits:
//
//	%idx_load = call %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(
//	    i32 68, %dx.types.Handle %indexBufHandle, i32 <wgslBinding>, i32 0)
//	%idx_raw  = extractvalue %dx.types.ResRet.i32 %idx_load, 0
//	%idx      = add i32 %idx_raw, 0
//	%handle   = call %dx.types.Handle @dx.op.createHandle(
//	    i32 57, i8 3, i32 <heapRangeID>, i32 %idx, i1 false)
//
// and stores the handle's emitter value ID in e.resourceHandles[gvHandle]
// AND patches the resourceInfo.handleID for the synthesized "wgsl-binding"
// virtual resource. No-op if e.samplerHeap is nil. Called from
// emitResourceHandles AFTER the index-buffer SRV handle has been created.
func (e *Emitter) emitSamplerHeapHandles() {
	if e.samplerHeap == nil {
		return
	}
	st := e.samplerHeap

	// Walk WGSL samplers in IR-declaration order so the bufferLoad+createHandle
	// sequences are deterministic and match the byte order DXC emits when
	// lowering the corresponding HLSL.
	for i := range e.ir.GlobalVariables {
		gvHandle := ir.GlobalVariableHandle(i)
		bind, ok := st.samplerWGSLBinding[gvHandle]
		if !ok {
			continue
		}
		if e.reachableGlobals != nil && !e.reachableGlobals[gvHandle] {
			continue
		}

		indexBufResIdx, hasIndexBuf := st.resIdxIndexBuffer[bind.group]
		if !hasIndexBuf {
			continue
		}
		indexBufHandleID := e.resources[indexBufResIdx].handleID
		if indexBufHandleID < 0 {
			continue
		}

		var heapResIdx int
		if bind.comparison {
			heapResIdx = st.resIdxComparisonHeap
		} else {
			heapResIdx = st.resIdxStandardHeap
		}
		if heapResIdx < 0 {
			continue
		}
		heapRes := &e.resources[heapResIdx]

		samplerHandleID := e.emitSamplerHeapHandle(indexBufHandleID, heapRes.rangeID, bind.binding)
		e.resourceHandles[gvHandle] = e.allocSamplerHandleResIdx(gvHandle, samplerHandleID, heapResIdx, bind)
	}
}

// emitSamplerHeapHandle emits the bufferLoad + extractvalue + add +
// createHandle sequence and returns the value ID of the created sampler
// handle.
func (e *Emitter) emitSamplerHeapHandle(indexBufHandleID int, heapRangeID int, wgslBinding uint32) int {
	resRetTy := e.getDxResRetType(overloadI32)
	bufLoadFn := e.getDxOpBufferLoadFunc(overloadI32)
	loadOpcodeVal := e.getIntConstID(int64(OpBufferLoad))
	idxVal := e.getIntConstID(int64(wgslBinding))
	zeroI32 := e.getIntConstID(0)

	loadResultID := e.addCallInstr(bufLoadFn, resRetTy,
		[]int{loadOpcodeVal, indexBufHandleID, idxVal, zeroI32})

	// Extract component 0.
	i32Ty := e.mod.GetIntType(32)
	extractID := e.allocValue()
	extractInstr := &module.Instruction{
		Kind:       module.InstrExtractVal,
		HasValue:   true,
		ResultType: i32Ty,
		Operands:   []int{loadResultID, 0},
		ValueID:    extractID,
	}
	e.currentBB.AddInstruction(extractInstr)

	// add %idx_raw, 0 — DXC emits this no-op add as part of the heap
	// indirection lowering. Match for byte-cmp parity with goldens.
	addedID := e.addBinOpInstr(i32Ty, BinOpAdd, extractID, zeroI32)

	// createHandle(57, sampler=3, rangeID=heapRangeID, index=addedID, false)
	handleTy := e.getDxHandleType()
	createFn := e.getDxOpCreateHandleFunc()
	createOpcodeVal := e.getIntConstID(int64(OpCreateHandle))
	classVal := e.getI8ConstID(int64(resourceClassSampler))
	rangeIDVal := e.getIntConstID(int64(heapRangeID))
	nonUniformVal := e.getI1ConstID(0)
	return e.addCallInstr(createFn, handleTy,
		[]int{createOpcodeVal, classVal, rangeIDVal, addedID, nonUniformVal})
}

// allocSamplerHandleResIdx allocates a virtual resourceInfo entry that
// records the materialized sampler handle for a WGSL sampler binding. The
// virtual entry is what e.resourceHandles[gvHandle] resolves to; its
// handleID drives subsequent resolveResourceHandle calls at sample sites.
//
// The virtual entry is intentionally NOT emitted in dx.resources metadata
// (resourceMetadata iterates by class and only the heap entries appear in
// the sampler bucket). It carries class=resourceClassSampler so that
// downstream consumers (resolveResourceHandle, getResourceHandleID) treat
// it identically to a real sampler resource.
func (e *Emitter) allocSamplerHandleResIdx(_ ir.GlobalVariableHandle, handleID, heapResIdx int, bind samplerHeapBinding) int {
	idx := len(e.resources)
	heapRes := &e.resources[heapResIdx]
	e.resources = append(e.resources, resourceInfo{
		varHandle:         ir.GlobalVariableHandle(0xfffffffe), // virtual — must not collide with synthesized heap
		name:              "",                                  // intentionally empty so it cannot leak into metadata if walked
		class:             resourceClassSampler,
		rangeID:           heapRes.rangeID,
		group:             bind.group,
		binding:           bind.binding,
		typeHandle:        ir.TypeHandle(0xffffffff),
		handleID:          handleID,
		virtual:           true, // exclude from dx.resources metadata + PSV0
		comparisonSampler: bind.comparison,
	})
	return idx
}
