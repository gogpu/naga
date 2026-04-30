package emit

import (
	"fmt"
	"sort"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
)

// dxcClassPriority maps DXIL resource class constants to DXC's handle emission
// priority order. DXC processes resources via GenerateDxilResourceHandles in
// the order CBV → Sampler → SRV → UAV, with each batch inserted sequentially
// at the entry block's alloca insertion point. The result in the bitcode is the
// same sequential order (CBV first). However, the LLVM bitcode writer serializes
// instructions in reverse basic-block order for the function body value table,
// producing the final disassembly order: UAV → SRV → CBV → Sampler. Within each
// class, resources are emitted in ascending ID order by DXC, which reverses to
// descending rangeID in the final output.
//
// Lower priority number = emitted FIRST in the bitcode output.
var dxcClassPriority = [4]int{
	resourceClassSRV:     1, // SRV (class 0) → second
	resourceClassUAV:     0, // UAV (class 1) → first
	resourceClassCBV:     2, // CBV (class 2) → third
	resourceClassSampler: 3, // Sampler (class 3) → last
}

// Resource handling for DXIL emission.
//
// This file handles CBV (constant buffer views), SRV (shader resource views),
// sampler bindings, and UAV stubs via dx.op intrinsics.
//
// Reference: Mesa nir_to_dxil.c emit_resources(), emit_createhandle_call_pre_6_6()

// dx.op function name for resource handle creation.
const dxOpCreateHandleName = "dx.op.createHandle"

// DXIL resource classes (matches DXIL spec D3D12_SHADER_INPUT_BIND_DESC).
const (
	resourceClassSRV     uint8 = 0
	resourceClassUAV     uint8 = 1
	resourceClassCBV     uint8 = 2
	resourceClassSampler uint8 = 3
)

// resourceInfo describes a single resource binding discovered during analysis.
type resourceInfo struct {
	varHandle      ir.GlobalVariableHandle
	name           string
	class          uint8 // resourceClassSRV, resourceClassUAV, resourceClassCBV, resourceClassSampler
	rangeID        int
	group          uint32
	binding        uint32
	typeHandle     ir.TypeHandle
	handleID       int    // emitter value ID of the created handle (-1 if not yet created)
	isBindingArray bool   // true if this resource is a binding_array<T, N>
	arraySize      uint32 // binding array size (0 = unbounded)
	// virtual is true for synthetic per-WGSL-binding sampler entries that
	// only carry a materialized handleID for downstream resolveResourceHandle
	// lookups; they MUST NOT appear in dx.resources metadata or PSV0
	// resource bindings (the actual heap entries already cover them).
	virtual bool
	// kindOverride, if non-zero, forces resourceKind() to return this
	// DXIL ResourceKind enum value instead of deriving from typeHandle.
	// Used for synthetic resources (sampler heap StructuredBuffer index
	// arrays) whose typeHandle doesn't point at a real IR type.
	kindOverride int
	// comparisonSampler forces buildSamplerMetadata to emit the
	// DXIL_SAMPLER_KIND_COMPARISON(1) shape tag even though the synthetic
	// resource has no backing IR SamplerType. Only meaningful when
	// class == resourceClassSampler.
	comparisonSampler bool
}

// analyzeResources scans the module's global variables and classifies them
// into resource categories. Must be called before emitting function bodies.
//
//nolint:gocognit // dispatch over resource classes + binding map + array sizes
func (e *Emitter) analyzeResources() {
	e.resources = nil
	e.resourceHandles = make(map[ir.GlobalVariableHandle]int)

	// Detect sampler-heap mode up front so per-WGSL sampler globals can
	// be skipped during the main classify loop.
	e.samplerHeap = e.detectSamplerHeapNeeded()

	// Track per-class range IDs.
	rangeCounters := [4]int{} // SRV, UAV, CBV, Sampler

	// Track which groups have already had their index buffer SRV inserted.
	// In the HLSL backend, the index buffer StructuredBuffer<uint> is declared
	// WHEN the first sampler of each group is encountered during globals
	// traversal. DXC assigns the SRV rangeID at that point. We mirror this
	// by inserting the index buffer SRV at the position of each group's
	// first sampler in the globals list.
	insertedIdxBufGroups := make(map[uint32]bool)

	for i := range e.ir.GlobalVariables {
		gv := &e.ir.GlobalVariables[i]

		// BUG-DXIL-016: skip globals not reachable from the current
		// entry point. Multi-EP modules share the global variable
		// arena across all entry points; without this filter, unrelated
		// resources from sibling EPs would be assigned binding slots
		// and create fake "Resource X overlap" errors at validation.
		if e.reachableGlobals != nil && !e.reachableGlobals[ir.GlobalVariableHandle(i)] {
			continue
		}

		// Sampler-heap mode: skip per-WGSL sampler entries. But FIRST,
		// when this is the first sampler of its group encountered in
		// traversal order, insert the per-group index buffer SRV at this
		// position. This mirrors the HLSL backend which writes the
		// StructuredBuffer<uint> declaration when it first encounters a
		// sampler from each group — giving DXC the same SRV rangeID
		// assignment point.
		if e.samplerHeap != nil {
			if bind, isSampler := e.samplerHeap.samplerWGSLBinding[ir.GlobalVariableHandle(i)]; isSampler {
				if !insertedIdxBufGroups[bind.group] {
					insertedIdxBufGroups[bind.group] = true
					e.insertSamplerIndexBufferSRV(bind.group, &rangeCounters)
				}
				continue
			}
		}

		class, ok := e.classifyGlobalVariable(gv)
		if !ok {
			continue
		}

		// Push constants / immediate data may not have explicit bindings.
		// Create a synthetic binding for them so they can be accessed as CBV.
		if gv.Binding == nil {
			if gv.Space == ir.SpacePushConstant || gv.Space == ir.SpaceImmediate {
				// Assign to group=0, binding=nextCBV to avoid conflict.
				gv.Binding = &ir.ResourceBinding{
					Group:   0,
					Binding: uint32(rangeCounters[resourceClassCBV]), //nolint:gosec // range counter is small
				}
			} else {
				continue
			}
		}

		rangeID := rangeCounters[class]
		rangeCounters[class]++

		// Detect binding arrays and record array size.
		var isBA bool
		var baSize uint32
		if int(gv.Type) < len(e.ir.Types) {
			if ba, ok := e.ir.Types[gv.Type].Inner.(ir.BindingArrayType); ok {
				isBA = true
				if ba.Size != nil {
					baSize = *ba.Size
				}
				// else baSize=0 means unbounded
			}
		}

		// Apply caller-supplied binding remap, if any. Bindings not
		// present in the map keep their raw WGSL numbers. See
		// EmitOptions.BindingMap (and dxil.Options.BindingMap) for
		// rationale — wgpu/hal/dx12 uses monotonic per-class register
		// counters that do not match raw @group/@binding numbers.
		//
		// When the map supplies BindingArraySize for an IR-unbounded
		// binding_array, use it as the concrete range size. DXIL PSV0
		// and bitcode resource metadata MUST agree on the size or the
		// validator trips 'ResourceBindInfo mismatch'; unbounded stays
		// as the fallback when no override is present.
		grp := gv.Binding.Group
		bnd := gv.Binding.Binding
		if e.opts.BindingMap != nil {
			if tgt, ok := e.opts.BindingMap[BindingLocation{Group: grp, Binding: bnd}]; ok {
				grp = tgt.Space
				bnd = tgt.Register
				if isBA && baSize == 0 && tgt.BindingArraySize != nil {
					baSize = *tgt.BindingArraySize
				}
			}
		}

		// Apply the HLSL namer suffix convention: names that end
		// with a digit or match an HLSL keyword get a trailing
		// underscore. DXC reads the resource name from dx.resources
		// metadata and displays it in the "Resource Bindings"
		// comment table emitted by dxc -dumpbin. Matching this
		// convention keeps the resource table identical to DXC.
		resName := gv.Name
		if hlsl.NeedsTrailingUnderscore(resName) {
			resName += "_"
		}

		info := resourceInfo{
			varHandle:      ir.GlobalVariableHandle(uint32(i)),
			name:           resName,
			class:          class,
			rangeID:        rangeID,
			group:          grp,
			binding:        bnd,
			typeHandle:     gv.Type,
			handleID:       -1,
			isBindingArray: isBA,
			arraySize:      baSize,
		}

		e.resourceHandles[info.varHandle] = len(e.resources)
		e.resources = append(e.resources, info)
	}

	// After the main loop: synthesize the sampler array heap entries.
	// Index buffer SRVs were already prepended above.
	e.appendSamplerHeapSamplers(&rangeCounters)
}

// classifyGlobalVariable determines the resource class of a global variable.
// Returns the class and true if it is a resource; false if it is not.
func (e *Emitter) classifyGlobalVariable(gv *ir.GlobalVariable) (uint8, bool) {
	switch gv.Space {
	case ir.SpaceUniform, ir.SpacePushConstant, ir.SpaceImmediate:
		// Uniform, push constant, and immediate data are all accessed as CBV in DXIL.
		// Push constants map to a constant buffer (b-register).
		return resourceClassCBV, true

	case ir.SpaceStorage:
		// Read-only storage buffers (var<storage, read>) map to SRV
		// (t-register, ByteAddressBuffer in HLSL terms). Read-write
		// storage buffers (var<storage, read_write>) map to UAV
		// (u-register, RWByteAddressBuffer). Matches the HLSL backend's
		// classifier (hlsl/storage.go getRegisterTypeForAddressSpace).
		if gv.Access == ir.StorageRead {
			return resourceClassSRV, true
		}
		return resourceClassUAV, true

	case ir.SpaceHandle:
		// Handle space: classify by the pointed-to type.
		// Unwrap BindingArrayType to find the base resource type.
		if int(gv.Type) < len(e.ir.Types) {
			inner := e.ir.Types[gv.Type].Inner
			// Unwrap binding array to get the base type.
			if ba, ok := inner.(ir.BindingArrayType); ok {
				if int(ba.Base) < len(e.ir.Types) {
					inner = e.ir.Types[ba.Base].Inner
				}
			}
			switch img := inner.(type) {
			case ir.ImageType:
				// Storage textures with write access are UAVs.
				if img.Class == ir.ImageClassStorage {
					return resourceClassUAV, true
				}
				return resourceClassSRV, true
			case ir.SamplerType:
				return resourceClassSampler, true
			case ir.AccelerationStructureType:
				// Acceleration structures are SRVs in DXIL (t-register).
				return resourceClassSRV, true
			}
		}
		return 0, false

	default:
		return 0, false
	}
}

// emitResourceHandles creates handle-creation calls for all analyzed
// resources. Must be called at function entry before any resource accesses.
//
// SM 6.0–6.5: dx.op.createHandle(i32 57, i8 class, i32 rangeID, i32 index, i1 false)
// SM 6.6+:    dx.op.createHandleFromBinding(i32 217, %dx.types.ResBind, i32 index, i1 false)
//
//	followed by
//	dx.op.annotateHandle(i32 216, %handle, %dx.types.ResourceProperties)
//
// The validator rejects createHandle from SM 6.6 onward
// ('opcode CreateHandle should only be used in Shader Model 6.5 and below'),
// while createHandleFromBinding+annotateHandle requires SM 6.2+ for the
// surrounding feature mask. We dispatch on opts.ShaderModelMinor.
//
// Reference: DXC DxilOperations.cpp:CreateHandleFromBinding/AnnotateHandle.
func (e *Emitter) emitResourceHandles() {
	if len(e.resources) == 0 {
		return
	}

	// Always use createHandle (opcode 57) — DXC uses this for all shader
	// models in its default compilation mode. createHandleFromBinding
	// (opcode 217, SM 6.6+) is only emitted when the shader explicitly
	// opts into descriptor heap resources via specific DXC flags. Our
	// DXC golden pipeline compiles with default flags, so createHandle
	// is the correct match.

	// Build the emission order indices. DXC emits createHandle calls sorted by:
	//   1. DXC class priority: UAV first, SRV second, CBV third, Sampler last
	//   2. Within each class: descending rangeID (highest rangeID first)
	// This order arises from DXC's GenerateDxilResourceHandles processing
	// (CBV→Sampler→SRV→UAV ascending) combined with LLVM bitcode serialization
	// that reverses the final instruction sequence in output.
	emitOrder := e.buildHandleEmitOrder()

	for _, idx := range emitOrder {
		res := &e.resources[idx]
		res.handleID = e.emitCreateHandleLegacy(res)
	}

	// Sampler-heap handles are emitted separately via emitSamplerHeapHandles()
	// AFTER emitInputLoads() in the entry-point emission sequence. DXC emits
	// loadInput calls before the sampler heap bufferLoad+createHandle sequence
	// (lazy evaluation places input reads before sampler index lookups). Matching
	// this order avoids instruction-scheduling diffs in the golden comparison.
}

// buildHandleEmitOrder returns indices into e.resources in DXC's createHandle
// emission order: UAV (class 1) first, SRV (class 0) second, CBV (class 2)
// third, Sampler (class 3) last. Within each class, descending rangeID.
// Binding arrays and virtual entries are excluded.
func (e *Emitter) buildHandleEmitOrder() []int {
	order := make([]int, 0, len(e.resources))
	for i := range e.resources {
		res := &e.resources[i]
		// Binding arrays get dynamic handles at point of use, not here.
		if res.isBindingArray {
			continue
		}
		// Virtual entries are placeholders that get their handleID set
		// later (emitSamplerHeapHandles). They do not emit createHandle.
		if res.virtual {
			continue
		}
		order = append(order, i)
	}

	sort.Slice(order, func(a, b int) bool {
		ra := &e.resources[order[a]]
		rb := &e.resources[order[b]]
		pa := dxcClassPriority[ra.class]
		pb := dxcClassPriority[rb.class]
		if pa != pb {
			return pa < pb
		}
		// Within the same class: descending rangeID (highest first).
		return ra.rangeID > rb.rangeID
	})
	return order
}

// emitCreateHandleLegacy emits the SM 6.0-6.5 dx.op.createHandle path.
func (e *Emitter) emitCreateHandleLegacy(res *resourceInfo) int {
	handleTy := e.getDxHandleType()
	createFn := e.getDxOpCreateHandleFunc()
	opcodeVal := e.getIntConstID(int64(OpCreateHandle))
	classVal := e.getI8ConstID(int64(res.class))
	rangeIDVal := e.getIntConstID(int64(res.rangeID))
	indexVal := e.getIntConstID(int64(res.binding))
	nonUniformVal := e.getI1ConstID(0)
	return e.addCallInstr(createFn, handleTy,
		[]int{opcodeVal, classVal, rangeIDVal, indexVal, nonUniformVal})
}

// getResourceHandleID returns the emitter value ID of the handle for a
// given global variable, or -1 if not a resource.
func (e *Emitter) getResourceHandleID(varHandle ir.GlobalVariableHandle) (int, bool) {
	idx, ok := e.resourceHandles[varHandle]
	if !ok {
		return -1, false
	}
	return e.resources[idx].handleID, true
}

// --- Texture sampling ---

// emitImageSample dispatches texture sampling to the appropriate dx.op intrinsic
// based on the sample level, depth reference, and gather parameters.
//
// Dispatching rules (matching DXC):
//
//	Gather != nil && DepthRef != nil  → OpTextureGatherCmp (74)
//	Gather != nil                     → OpTextureGather (73)
//	DepthRef != nil && SampleLevelZero → OpSampleCmpLevelZero (65)
//	DepthRef != nil                   → OpSampleCmp (64)
//	SampleLevelBias                   → OpSampleBias (61)
//	SampleLevelExact                  → OpSampleLevel (62)
//	SampleLevelGradient               → OpSampleGrad (63)
//	SampleLevelAuto / SampleLevelZero → OpSample (60)
//
// Reference: Mesa nir_to_dxil.c emit_sample(), emit_texture_gather_call()
// Reference: DXIL.rst Sample/SampleBias/SampleLevel/SampleGrad/SampleCmp/SampleCmpLevelZero/TextureGather/TextureGatherCmp
//
//nolint:gocognit,cyclop,gocyclo,funlen,maintidx // dispatch logic for 8 texture sample variants
func (e *Emitter) emitImageSample(fn *ir.Function, sample ir.ExprImageSample) (int, error) {
	// Resolve image and sampler handles.
	imageHandleID, err := e.resolveResourceHandle(fn, sample.Image)
	if err != nil {
		return 0, fmt.Errorf("image handle: %w", err)
	}
	samplerHandleID, err := e.resolveResourceHandle(fn, sample.Sampler)
	if err != nil {
		return 0, fmt.Errorf("sampler handle: %w", err)
	}

	// Emit coordinate expression.
	if _, err := e.emitExpression(fn, sample.Coordinate); err != nil {
		return 0, fmt.Errorf("coordinate: %w", err)
	}

	coordType := e.resolveExprType(fn, sample.Coordinate)
	coordComps := componentCount(coordType)

	// Determine overload from the texture's sampled kind. Most sampled
	// textures are f32 (ScalarFloat → overloadF32), but textureGather on
	// integer textures (texture_2d<u32>/texture_2d<i32>) requires the i32
	// overload so dx.op.textureGather.i32 returns i32 ResRet components.
	// Using f32 unconditionally caused downstream u32→f32 /i32→f32 casts
	// to compile as identity float→float uitofp/sitofp instructions,
	// tripping the LLVM verifier.
	ol := overloadF32
	var imgType ir.ImageType
	var haveImgType bool
	if imgInner, okImg := e.resolveExprType(fn, sample.Image).(ir.ImageType); okImg {
		ol = imageOverload(imgInner)
		imgType = imgInner
		haveImgType = true
	}

	// Out-of-bound coordinate / offset / clamp slots must be undef per the
	// DXIL validator rule 'out of bound X must be undef'. DXC itself emits
	// 'float undef' / 'i32 undef' for every unused slot (verified against
	// dxc -T ps_6_0 for tex.Sample(samp, uv, int2(3,1))), not zero.
	f32Ty := e.mod.GetFloatType(32)
	f32UndefID := e.getTypedUndefConstID(f32Ty)
	i32UndefID := e.getUndefConstID()

	// Scalarize coordinates into coord0..coord3. Slots past (coordComps +
	// array index) stay undef.
	coords := [4]int{f32UndefID, f32UndefID, f32UndefID, f32UndefID}
	for i := 0; i < 4 && i < coordComps; i++ {
		coords[i] = e.getComponentID(sample.Coordinate, i)
	}

	// Handle array index: appended after spatial coordinates.
	if sample.ArrayIndex != nil {
		arrID, err2 := e.emitExpression(fn, *sample.ArrayIndex)
		if err2 != nil {
			return 0, fmt.Errorf("array index: %w", err2)
		}
		// Convert array index to float for sample coordinates. WGSL array
		// indices are typed i32 or u32; DXIL sample intrinsics take all
		// coordinates as f32, so we need SIToFP/UIToFP depending on signedness
		// of the ArrayIndex expression. Default to UIToFP — WGSL array indices
		// are non-negative and the two produce identical bit patterns for the
		// in-range case, but signed sources should use SIToFP for clarity.
		arrCastOp := CastUIToFP
		if arrScalar, okScal := scalarOfType(e.resolveExprType(fn, *sample.ArrayIndex)); okScal && arrScalar.Kind == ir.ScalarSint {
			arrCastOp = CastSIToFP
		}
		castID := e.addCastInstr(f32Ty, arrCastOp, arrID)
		if coordComps < 4 {
			coords[coordComps] = castID
		}
	}

	// Emit depth reference if present.
	var depthRefID int
	if sample.DepthRef != nil {
		depthRefID, err = e.emitExpression(fn, *sample.DepthRef)
		if err != nil {
			return 0, fmt.Errorf("depth ref: %w", err)
		}
	}

	// Offset slots. Per DXC's lowering (verified via dxc -T ps_6_0 -dumpbin
	// on Texture1D.Sample / Texture2D.Sample / Texture3D.Sample), in-bound
	// offset slots always carry a real i32 value — defaulting to 'i32 0'
	// when the HLSL call has no offset argument — while out-of-bound slots
	// are 'i32 undef'. Cube textures do not allow any offset at all: all
	// three slots stay undef for them. Using 'i32 undef' on an in-bound
	// slot trips the DXIL validator rule 'coord uninitialized' (the rule
	// text is imprecise — it covers both coord and offset parameters).
	numOffsetDims := 0
	if haveImgType && imgType.Dim != ir.DimCube {
		numOffsetDims = imageDimSpatialComponents(imgType.Dim)
	}
	zeroI := e.getIntConstID(0)
	offsets := [3]int{i32UndefID, i32UndefID, i32UndefID}
	for i := 0; i < numOffsetDims; i++ {
		offsets[i] = zeroI
	}
	if sample.Offset != nil && numOffsetDims > 0 {
		if _, err2 := e.emitExpression(fn, *sample.Offset); err2 != nil {
			return 0, fmt.Errorf("sample offset: %w", err2)
		}
		offsetType := e.resolveExprType(fn, *sample.Offset)
		offsetComps := componentCount(offsetType)
		for i := 0; i < numOffsetDims && i < offsetComps && i < 3; i++ {
			offsets[i] = e.getComponentID(*sample.Offset, i)
		}
	}

	// Dispatch to appropriate intrinsic.
	resRetTy := e.getDxResRetType(ol)
	var retID int

	switch {
	case sample.Gather != nil && sample.DepthRef != nil:
		// TextureGatherCmp (74): opcode, tex, sampler, c0-c3, o0, o1, channel, compare
		channel := int(*sample.Gather)
		opFn := e.getDxOpTextureGatherCmpFunc(ol)
		retID = e.addCallInstr(opFn, resRetTy, []int{
			e.getIntConstID(int64(OpTextureGatherCmp)),
			imageHandleID, samplerHandleID,
			coords[0], coords[1], coords[2], coords[3],
			offsets[0], offsets[1],
			e.getIntConstID(int64(channel)),
			depthRefID,
		})

	case sample.Gather != nil:
		// TextureGather (73): opcode, tex, sampler, c0-c3, o0, o1, channel
		channel := int(*sample.Gather)
		opFn := e.getDxOpTextureGatherFunc(ol)
		retID = e.addCallInstr(opFn, resRetTy, []int{
			e.getIntConstID(int64(OpTextureGather)),
			imageHandleID, samplerHandleID,
			coords[0], coords[1], coords[2], coords[3],
			offsets[0], offsets[1],
			e.getIntConstID(int64(channel)),
		})

	case sample.DepthRef != nil:
		switch sample.Level.(type) {
		case ir.SampleLevelZero:
			// SampleCmpLevelZero (65): opcode, tex, sampler, c0-c3, o0-o2, compare
			opFn := e.getDxOpSampleCmpLevelZeroFunc(ol)
			retID = e.addCallInstr(opFn, resRetTy, []int{
				e.getIntConstID(int64(OpSampleCmpLevelZero)),
				imageHandleID, samplerHandleID,
				coords[0], coords[1], coords[2], coords[3],
				offsets[0], offsets[1], offsets[2],
				depthRefID,
			})
		default:
			// SampleCmp (64): opcode, tex, sampler, c0-c3, o0-o2, compare, clamp
			opFn := e.getDxOpSampleCmpFunc(ol)
			retID = e.addCallInstr(opFn, resRetTy, []int{
				e.getIntConstID(int64(OpSampleCmp)),
				imageHandleID, samplerHandleID,
				coords[0], coords[1], coords[2], coords[3],
				offsets[0], offsets[1], offsets[2],
				depthRefID,
				f32UndefID, // clamp
			})
		}

	default:
		switch lvl := sample.Level.(type) {
		case ir.SampleLevelBias:
			// SampleBias (61): opcode, tex, sampler, c0-c3, o0-o2, bias, clamp
			biasID, err2 := e.emitExpression(fn, lvl.Bias)
			if err2 != nil {
				return 0, fmt.Errorf("sample bias: %w", err2)
			}
			opFn := e.getDxOpSampleBiasFunc(ol)
			retID = e.addCallInstr(opFn, resRetTy, []int{
				e.getIntConstID(int64(OpSampleBias)),
				imageHandleID, samplerHandleID,
				coords[0], coords[1], coords[2], coords[3],
				offsets[0], offsets[1], offsets[2],
				biasID,
				f32UndefID, // clamp
			})

		case ir.SampleLevelExact:
			// SampleLevel (62): opcode, tex, sampler, c0-c3, o0-o2, LOD
			lodID, err2 := e.emitExpression(fn, lvl.Level)
			if err2 != nil {
				return 0, fmt.Errorf("sample level: %w", err2)
			}
			// Ensure LOD is float (WGSL allows integer level for depth textures).
			lodID = e.ensureFloat(fn, lodID, lvl.Level)
			opFn := e.getDxOpSampleLevelFunc(ol)
			retID = e.addCallInstr(opFn, resRetTy, []int{
				e.getIntConstID(int64(OpSampleLevel)),
				imageHandleID, samplerHandleID,
				coords[0], coords[1], coords[2], coords[3],
				offsets[0], offsets[1], offsets[2],
				lodID,
			})

		case ir.SampleLevelGradient:
			// SampleGrad (63): opcode, tex, sampler, c0-c3, o0-o2, ddx0-2, ddy0-2, clamp
			if _, err2 := e.emitExpression(fn, lvl.X); err2 != nil {
				return 0, fmt.Errorf("sample grad x: %w", err2)
			}
			if _, err2 := e.emitExpression(fn, lvl.Y); err2 != nil {
				return 0, fmt.Errorf("sample grad y: %w", err2)
			}
			ddxType := e.resolveExprType(fn, lvl.X)
			ddxComps := componentCount(ddxType)
			ddx := [3]int{f32UndefID, f32UndefID, f32UndefID}
			ddy := [3]int{f32UndefID, f32UndefID, f32UndefID}
			for i := 0; i < 3 && i < ddxComps; i++ {
				ddx[i] = e.getComponentID(lvl.X, i)
				ddy[i] = e.getComponentID(lvl.Y, i)
			}
			opFn := e.getDxOpSampleGradFunc(ol)
			retID = e.addCallInstr(opFn, resRetTy, []int{
				e.getIntConstID(int64(OpSampleGrad)),
				imageHandleID, samplerHandleID,
				coords[0], coords[1], coords[2], coords[3],
				offsets[0], offsets[1], offsets[2],
				ddx[0], ddx[1], ddx[2],
				ddy[0], ddy[1], ddy[2],
				f32UndefID, // clamp
			})

		default:
			// Sample (60): opcode, tex, sampler, c0-c3, o0-o2, clamp
			opFn := e.getDxOpSampleFunc(ol)
			retID = e.addCallInstr(opFn, resRetTy, []int{
				e.getIntConstID(int64(OpSample)),
				imageHandleID, samplerHandleID,
				coords[0], coords[1], coords[2], coords[3],
				offsets[0], offsets[1], offsets[2],
				f32UndefID, // clamp
			})
		}
	}

	// Extract 4 components from the result and store as pending components.
	scalarTy := e.overloadReturnType(ol)
	comps := make([]int, 4)
	for i := 0; i < 4; i++ {
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: scalarTy,
			Operands:   []int{retID, i},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		comps[i] = extractID
	}
	e.pendingComponents = comps

	return comps[0], nil
}

// emitImageQuery emits dx.op.getDimensions (opcode 72) for image size/level/sample queries.
//
// getDimensions returns {i32, i32, i32, i32} where the meaning depends on the
// resource type:
//
//	c0 = width, c1 = height (or array size for 1DArray), c2 = depth/array size, c3 = mip levels (or samples for MS)
//
// Query mapping:
//   - ImageQuerySize: extract spatial dimensions (c0..cN based on image dimension)
//   - ImageQueryNumLevels: extract c3
//   - ImageQueryNumSamples: extract c3
//   - ImageQueryNumLayers: extract array size component (c1 for 1DArray, c2 for 2DArray/CubeArray)
//
// Reference: Mesa nir_to_dxil.c emit_texture_size() ~4294, emit_image_size() ~4310
// Reference: DXIL.rst GetDimensions return table ~1360
func (e *Emitter) emitImageQuery(fn *ir.Function, query ir.ExprImageQuery) (int, error) {
	// Resolve the image handle.
	imageHandleID, err := e.resolveResourceHandle(fn, query.Image)
	if err != nil {
		return 0, fmt.Errorf("ExprImageQuery: image handle: %w", err)
	}

	// Resolve the image type to determine dimension and arrayed/multisampled properties.
	imgInner := e.resolveExprType(fn, query.Image)
	imgType, ok := imgInner.(ir.ImageType)
	if !ok {
		return 0, fmt.Errorf("ExprImageQuery: image expression is not ImageType, got %T", imgInner)
	}

	i32Ty := e.mod.GetIntType(32)
	dimTy := e.getDxDimensionsType()
	getDimFn := e.getDxOpGetDimensionsFunc()
	opcodeVal := e.getIntConstID(int64(OpGetDimensions))
	undefVal := e.getUndefConstID()

	switch q := query.Query.(type) {
	case ir.ImageQuerySize:
		// LOD argument: if Level is provided, emit it; otherwise use 0 for mipmapped or undef for non-mipmapped.
		var lodID int
		if q.Level != nil {
			lodID, err = e.emitExpression(fn, *q.Level)
			if err != nil {
				return 0, fmt.Errorf("ExprImageQuery size level: %w", err)
			}
		} else if imgType.Multisampled {
			lodID = undefVal
		} else {
			lodID = e.getIntConstID(0)
		}

		dimRetID := e.addCallInstr(getDimFn, dimTy, []int{opcodeVal, imageHandleID, lodID})

		// Determine how many spatial components to extract based on image dimension.
		numComps := imageDimSpatialComponents(imgType.Dim)

		if numComps == 1 {
			// Scalar result — extract component 0.
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: i32Ty,
				Operands:   []int{dimRetID, 0},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			return extractID, nil
		}

		// Vector result — extract N components.
		comps := make([]int, numComps)
		for i := 0; i < numComps; i++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: i32Ty,
				Operands:   []int{dimRetID, i},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps[i] = extractID
		}
		e.pendingComponents = comps
		return comps[0], nil

	case ir.ImageQueryNumLevels:
		// getDimensions(handle, 0) → extract c3 = MIP levels
		lodID := e.getIntConstID(0)
		dimRetID := e.addCallInstr(getDimFn, dimTy, []int{opcodeVal, imageHandleID, lodID})

		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: i32Ty,
			Operands:   []int{dimRetID, 3},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		return extractID, nil

	case ir.ImageQueryNumSamples:
		// getDimensions(handle, undef) → extract c3 = samples
		dimRetID := e.addCallInstr(getDimFn, dimTy, []int{opcodeVal, imageHandleID, undefVal})

		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: i32Ty,
			Operands:   []int{dimRetID, 3},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		return extractID, nil

	case ir.ImageQueryNumLayers:
		// getDimensions(handle, 0) → extract the array size component.
		// For 1DArray: c1. For 2DArray/CubeArray/2DMSArray: c2.
		lodID := e.getIntConstID(0)
		dimRetID := e.addCallInstr(getDimFn, dimTy, []int{opcodeVal, imageHandleID, lodID})

		arrayComp := imageDimArrayComponent(imgType.Dim)
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: i32Ty,
			Operands:   []int{dimRetID, arrayComp},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		return extractID, nil

	default:
		return 0, fmt.Errorf("ExprImageQuery: unsupported query type %T", query.Query)
	}
}

// imageDimSpatialComponents returns the number of spatial size components for an image dimension.
// This does NOT include array layers — those are queried separately via ImageQueryNumLayers.
func imageDimSpatialComponents(dim ir.ImageDimension) int {
	switch dim {
	case ir.Dim1D:
		return 1
	case ir.Dim2D, ir.DimCube:
		return 2
	case ir.Dim3D:
		return 3
	default:
		return 2
	}
}

// imageDimArrayComponent returns the getDimensions component index containing the array size.
// For 1DArray: c1 (after width). For 2DArray/CubeArray: c2 (after width, height).
func imageDimArrayComponent(dim ir.ImageDimension) int {
	switch dim {
	case ir.Dim1D:
		return 1
	default:
		return 2
	}
}

// imageOverload returns the DXIL overload type for an image's element type.
// For sampled images, the overload comes from SampledKind.
// For depth images, the overload is always f32 — depth textures are defined
// to return floating-point depth values; ImageType.SampledKind is not valid
// for ImageClassDepth (per the struct doc) and defaults to ScalarSint, which
// would incorrectly select i32 overload.
// For storage images, the overload comes from StorageFormat.
func imageOverload(img ir.ImageType) overloadType {
	switch img.Class {
	case ir.ImageClassStorage:
		return overloadForScalar(img.StorageFormat.Scalar())
	case ir.ImageClassDepth:
		return overloadF32
	default:
		return overloadForScalar(ir.ScalarType{Kind: img.SampledKind, Width: 4})
	}
}

// imageCoordCount returns the number of coordinate components for a textureLoad/textureStore call.
// This includes the array index if the image is arrayed.
func imageCoordCount(img ir.ImageType) int {
	n := imageDimSpatialComponents(img.Dim)
	if img.Arrayed {
		n++
	}
	return n
}

// emitImageLoad emits dx.op.textureLoad (opcode 66) for texel fetching.
//
// Signature: %dx.types.ResRet.XX @dx.op.textureLoad.XX(i32 opcode, %handle, i32 mip/sample, i32 c0, i32 c1, i32 c2, i32 o0, i32 o1, i32 o2)
//
// Reference: Mesa nir_to_dxil.c emit_image_load() ~4122, emit_texel_fetch() ~5376
func (e *Emitter) emitImageLoad(fn *ir.Function, load ir.ExprImageLoad) (int, error) {
	// Resolve the image handle.
	imageHandleID, err := e.resolveResourceHandle(fn, load.Image)
	if err != nil {
		return 0, fmt.Errorf("ExprImageLoad: image handle: %w", err)
	}

	// Resolve image type for overload and coordinate count.
	imgInner := e.resolveExprType(fn, load.Image)
	imgType, ok := imgInner.(ir.ImageType)
	if !ok {
		return 0, fmt.Errorf("ExprImageLoad: image expression is not ImageType, got %T", imgInner)
	}

	ol := imageOverload(imgType)

	// Emit coordinate expression.
	if _, err := e.emitExpression(fn, load.Coordinate); err != nil {
		return 0, fmt.Errorf("ExprImageLoad: coordinate: %w", err)
	}
	coordType := e.resolveExprType(fn, load.Coordinate)
	coordComps := componentCount(coordType)

	undefI32 := e.getUndefConstID()
	opcodeVal := e.getIntConstID(int64(OpTextureLoad))

	// MIP level / sample index.
	var lodOrSampleID int
	if load.Level != nil {
		lodOrSampleID, err = e.emitExpression(fn, *load.Level)
		if err != nil {
			return 0, fmt.Errorf("ExprImageLoad: level: %w", err)
		}
	} else if load.Sample != nil {
		lodOrSampleID, err = e.emitExpression(fn, *load.Sample)
		if err != nil {
			return 0, fmt.Errorf("ExprImageLoad: sample: %w", err)
		}
	} else {
		lodOrSampleID = undefI32
	}

	// Build coordinate array [c0, c1, c2], unused = undef.
	coords := [3]int{undefI32, undefI32, undefI32}
	numCoords := imageCoordCount(imgType)
	// First fill spatial coordinates from the Coordinate expression.
	spatialComps := imageDimSpatialComponents(imgType.Dim)
	for i := 0; i < spatialComps && i < coordComps; i++ {
		coords[i] = e.getComponentID(load.Coordinate, i)
	}
	// If arrayed, the array index comes from ArrayIndex or the last coordinate component.
	if imgType.Arrayed && load.ArrayIndex != nil {
		arrayIdx, err2 := e.emitExpression(fn, *load.ArrayIndex)
		if err2 != nil {
			return 0, fmt.Errorf("ExprImageLoad: array index: %w", err2)
		}
		if numCoords-1 < 3 {
			coords[numCoords-1] = arrayIdx
		}
	} else if imgType.Arrayed && coordComps > spatialComps {
		// Array index is packed into the last coordinate component.
		if numCoords-1 < 3 {
			coords[numCoords-1] = e.getComponentID(load.Coordinate, spatialComps)
		}
	}

	// textureLoad: opcode, handle, mip/sample, c0, c1, c2, o0, o1, o2
	operands := []int{
		opcodeVal,
		imageHandleID,
		lodOrSampleID,
		coords[0], coords[1], coords[2],
		undefI32, undefI32, undefI32, // offsets (undef for image loads)
	}

	resRetTy := e.getDxResRetType(ol)
	textureLoadFn := e.getDxOpTextureLoadFunc(ol)
	retID := e.addCallInstr(textureLoadFn, resRetTy, operands)

	// Extract 4 components.
	scalarTy := e.overloadReturnType(ol)
	comps := make([]int, 4)
	for i := 0; i < 4; i++ {
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: scalarTy,
			Operands:   []int{retID, i},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		comps[i] = extractID
	}
	e.pendingComponents = comps
	return comps[0], nil
}

// emitStmtImageStore emits dx.op.textureStore (opcode 67) for writing to storage textures.
//
// Signature: void @dx.op.textureStore.XX(i32 opcode, %handle, i32 c0, i32 c1, i32 c2, XX v0, XX v1, XX v2, XX v3, i8 mask)
//
// Reference: Mesa nir_to_dxil.c emit_image_store() ~4060, emit_texturestore_call() ~924
func (e *Emitter) emitStmtImageStore(fn *ir.Function, store ir.StmtImageStore) error {
	// Resolve the image handle.
	imageHandleID, err := e.resolveResourceHandle(fn, store.Image)
	if err != nil {
		return fmt.Errorf("StmtImageStore: image handle: %w", err)
	}

	// Resolve image type for overload and coordinate count.
	imgInner := e.resolveExprType(fn, store.Image)
	imgType, ok := imgInner.(ir.ImageType)
	if !ok {
		return fmt.Errorf("StmtImageStore: image expression is not ImageType, got %T", imgInner)
	}

	ol := imageOverload(imgType)

	// Emit coordinate expression.
	if _, err := e.emitExpression(fn, store.Coordinate); err != nil {
		return fmt.Errorf("StmtImageStore: coordinate: %w", err)
	}
	coordType := e.resolveExprType(fn, store.Coordinate)
	coordComps := componentCount(coordType)

	// Emit value expression.
	if _, err := e.emitExpression(fn, store.Value); err != nil {
		return fmt.Errorf("StmtImageStore: value: %w", err)
	}
	valType := e.resolveExprType(fn, store.Value)
	valComps := componentCount(valType)

	undefI32 := e.getUndefConstID()
	opcodeVal := e.getIntConstID(int64(OpTextureStore))

	// Coordinates [c0, c1, c2], unused = undef.
	coords := [3]int{undefI32, undefI32, undefI32}
	spatialComps := imageDimSpatialComponents(imgType.Dim)
	numCoords := imageCoordCount(imgType)
	for i := 0; i < spatialComps && i < coordComps; i++ {
		coords[i] = e.getComponentID(store.Coordinate, i)
	}
	if imgType.Arrayed && store.ArrayIndex != nil {
		arrayIdx, err2 := e.emitExpression(fn, *store.ArrayIndex)
		if err2 != nil {
			return fmt.Errorf("StmtImageStore: array index: %w", err2)
		}
		if numCoords-1 < 3 {
			coords[numCoords-1] = arrayIdx
		}
	} else if imgType.Arrayed && coordComps > spatialComps {
		if numCoords-1 < 3 {
			coords[numCoords-1] = e.getComponentID(store.Coordinate, spatialComps)
		}
	}

	// Value components [v0..v3], unused = undef of the value type.
	scalarTy := e.overloadReturnType(ol)
	undefValTy := e.getTypedUndefConstID(scalarTy)
	vals := [4]int{undefValTy, undefValTy, undefValTy, undefValTy}
	for i := 0; i < 4 && i < valComps; i++ {
		vals[i] = e.getComponentID(store.Value, i)
	}

	// Write mask: bit per component written.
	writeMask := (1 << valComps) - 1
	writeMaskID := e.getI8ConstID(int64(writeMask))

	// textureStore: opcode, handle, c0, c1, c2, v0, v1, v2, v3, mask
	operands := []int{
		opcodeVal,
		imageHandleID,
		coords[0], coords[1], coords[2],
		vals[0], vals[1], vals[2], vals[3],
		writeMaskID,
	}

	textureStoreFn := e.getDxOpTextureStoreFunc(ol)
	voidTy := e.mod.GetVoidType()
	e.addCallInstr(textureStoreFn, voidTy, operands)
	return nil
}

// getDxOpTextureLoadFunc returns the dx.op.textureLoad.XX function declaration.
// Signature: %dx.types.ResRet.XX @dx.op.textureLoad.XX(i32, %handle, i32, i32, i32, i32, i32, i32, i32)
func (e *Emitter) getDxOpTextureLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.textureLoad" + overloadSuffix(ol)
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	handleTy := e.getDxHandleType()
	resRetTy := e.getDxResRetType(ol)
	// 9 params: opcode, handle, mip/sample, c0, c1, c2, o0, o1, o2
	funcTy := e.mod.GetFunctionType(resRetTy, []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i32Ty, i32Ty, i32Ty, i32Ty, i32Ty})
	fn := e.mod.AddFunction(name, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(name)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpTextureStoreFunc returns the dx.op.textureStore.XX function declaration.
// Signature: void @dx.op.textureStore.XX(i32, %handle, i32, i32, i32, XX, XX, XX, XX, i8)
func (e *Emitter) getDxOpTextureStoreFunc(ol overloadType) *module.Function {
	name := "dx.op.textureStore" + overloadSuffix(ol)
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	handleTy := e.getDxHandleType()
	scalarTy := e.overloadReturnType(ol)
	voidTy := e.mod.GetVoidType()
	// 10 params: opcode, handle, c0, c1, c2, v0, v1, v2, v3, mask(i8)
	funcTy := e.mod.GetFunctionType(voidTy, []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i32Ty, scalarTy, scalarTy, scalarTy, scalarTy, i8Ty})
	fn := e.mod.AddFunction(name, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(name)
	e.dxOpFuncs[key] = fn
	return fn
}

// resolveResourceHandle evaluates the expression and returns the resource
// handle value ID. The expression must be an ExprGlobalVariable that was
// classified as a resource.
func (e *Emitter) resolveResourceHandle(fn *ir.Function, exprHandle ir.ExpressionHandle) (int, error) {
	// If this expression was already emitted (e.g., by emitAccess for a binding
	// array), reuse the cached value to avoid emitting duplicate createHandle calls.
	if v, ok := e.exprValues[exprHandle]; ok {
		return v, nil
	}

	expr := fn.Expressions[exprHandle]

	// Direct global variable reference (non-array resource).
	if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
		if handleID, found := e.getResourceHandleID(gv.Variable); found {
			if handleID >= 0 {
				return handleID, nil
			}
			// handleID == -1 means binding array base; this is reached when
			// the expression is ExprGlobalVariable directly (not via ExprAccess).
			// This shouldn't happen for properly formed IR, but return a safe error.
			return 0, fmt.Errorf("binding array base %d accessed without index", gv.Variable)
		}
		return 0, fmt.Errorf("global variable %d is not a resource", gv.Variable)
	}

	// Access into a binding array (dynamic index): Access(base=GlobalVariable(ba), index=expr).
	if acc, ok := expr.Kind.(ir.ExprAccess); ok { //nolint:nestif // binding array dispatch
		if gv, ok2 := fn.Expressions[acc.Base].Kind.(ir.ExprGlobalVariable); ok2 {
			if resIdx, found := e.resourceHandles[gv.Variable]; found {
				res := &e.resources[resIdx]
				if res.isBindingArray {
					return e.emitDynamicCreateHandle(fn, res, acc.Index)
				}
			}
		}
	}

	// AccessIndex into a binding array (constant index): AccessIndex(base=GlobalVariable(ba), index=N).
	if ai, ok := expr.Kind.(ir.ExprAccessIndex); ok { //nolint:nestif // binding array dispatch
		if gv, ok2 := fn.Expressions[ai.Base].Kind.(ir.ExprGlobalVariable); ok2 {
			if resIdx, found := e.resourceHandles[gv.Variable]; found {
				res := &e.resources[resIdx]
				if res.isBindingArray {
					return e.emitDynamicCreateHandleConst(res, int64(ai.Index))
				}
			}
		}
	}

	// Fallback: emit the expression and hope it resolves.
	return e.emitExpression(fn, exprHandle)
}

// emitDynamicCreateHandle creates a dx.op.createHandle call with a dynamic index
// for binding array resources. The index expression is emitted and used as the
// 4th parameter (resource index within the range).
//
// dx.op.createHandle(i32 57, i8 class, i32 rangeID, i32 index, i1 nonUniform)
//
// Reference: Mesa nir_to_dxil.c emit_createhandle_call_pre_6_6()
func (e *Emitter) emitDynamicCreateHandle(fn *ir.Function, res *resourceInfo, indexExpr ir.ExpressionHandle) (int, error) {
	// Emit the dynamic index expression.
	indexID, err := e.emitExpression(fn, indexExpr)
	if err != nil {
		return 0, fmt.Errorf("binding array index: %w", err)
	}

	// The index into createHandle is lower_bound + array_index.
	if res.binding > 0 {
		baseVal := e.getIntConstID(int64(res.binding))
		i32Ty := e.mod.GetIntType(32)
		indexID = e.addBinOpInstr(i32Ty, BinOpAdd, baseVal, indexID)
	}

	return e.emitCreateHandleCall(res, indexID)
}

// emitDynamicCreateHandleConst creates a dx.op.createHandle call with a constant index.
// Used for ExprAccessIndex on binding arrays where the index is known at compile time.
func (e *Emitter) emitDynamicCreateHandleConst(res *resourceInfo, constIndex int64) (int, error) {
	indexID := e.getIntConstID(constIndex + int64(res.binding))
	return e.emitCreateHandleCall(res, indexID)
}

// emitCreateHandleCall emits the dx.op.createHandle call with a prepared index value ID.
func (e *Emitter) emitCreateHandleCall(res *resourceInfo, indexID int) (int, error) {
	handleTy := e.getDxHandleType()
	createFn := e.getDxOpCreateHandleFunc()

	opcodeVal := e.getIntConstID(int64(OpCreateHandle))
	classVal := e.getI8ConstID(int64(res.class))
	rangeIDVal := e.getIntConstID(int64(res.rangeID))
	nonUniformVal := e.getI1ConstID(0) // TODO: detect non-uniform index from IR

	handleID := e.addCallInstr(createFn, handleTy,
		[]int{opcodeVal, classVal, rangeIDVal, indexID, nonUniformVal})

	return handleID, nil
}

// --- dx.op function declarations and DXIL types ---

// getDxHandleType returns the %dx.types.Handle opaque struct type.
func (e *Emitter) getDxHandleType() *module.Type {
	if e.dxHandleType != nil {
		return e.dxHandleType
	}
	// %dx.types.Handle = type { i8* }
	i8PtrTy := e.mod.GetPointerType(e.mod.GetIntType(8))
	e.dxHandleType = e.mod.GetStructType("dx.types.Handle", []*module.Type{i8PtrTy})
	return e.dxHandleType
}

// getDxResRetType returns the %dx.types.ResRet.XX struct type.
func (e *Emitter) getDxResRetType(ol overloadType) *module.Type {
	scalarTy := e.overloadReturnType(ol)
	i32Ty := e.mod.GetIntType(32)
	name := "dx.types.ResRet" + overloadSuffix(ol)
	// ResRet has 4 scalar components + 1 i32 status.
	return e.mod.GetStructType(name, []*module.Type{scalarTy, scalarTy, scalarTy, scalarTy, i32Ty})
}

// getDxDimensionsType returns the %dx.types.Dimensions = {i32, i32, i32, i32} struct type.
// Used by dx.op.getDimensions (opcode 72) for buffer/texture size queries.
func (e *Emitter) getDxDimensionsType() *module.Type {
	i32Ty := e.mod.GetIntType(32)
	return e.mod.GetStructType("dx.types.Dimensions", []*module.Type{i32Ty, i32Ty, i32Ty, i32Ty})
}

// getDxOpGetDimensionsFunc returns the dx.op.getDimensions function declaration.
// Signature: %dx.types.Dimensions @dx.op.getDimensions(i32, %dx.types.Handle, i32)
// Reference: Mesa nir_to_dxil.c emit_texture_size() ~4294
func (e *Emitter) getDxOpGetDimensionsFunc() *module.Function {
	key := dxOpKey{name: "dx.op.getDimensions", overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32Ty := e.mod.GetIntType(32)
	dimTy := e.getDxDimensionsType()
	handleTy := e.getDxHandleType()
	funcTy := e.mod.GetFunctionType(dimTy, []*module.Type{i32Ty, handleTy, i32Ty})
	fn := e.mod.AddFunction("dx.op.getDimensions", funcTy, true)
	fn.AttrSetID = classifyDxOpAttr("dx.op.getDimensions")
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpCreateHandleFunc creates the dx.op.createHandle function declaration.
// Signature: %dx.types.Handle @dx.op.createHandle(i32, i8, i32, i32, i1)
//
// DXC uses createHandle for all SM versions in its default compilation
// mode (matching our golden pipeline). createHandleFromBinding (SM 6.6+)
// is only used when specific DXC flags are set.
func (e *Emitter) getDxOpCreateHandleFunc() *module.Function {
	key := dxOpKey{name: dxOpCreateHandleName, overload: overloadVoid}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	i1Ty := e.mod.GetIntType(1)

	params := []*module.Type{i32Ty, i8Ty, i32Ty, i32Ty, i1Ty}
	funcTy := e.mod.GetFunctionType(handleTy, params)
	fn := e.mod.AddFunction(dxOpCreateHandleName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(dxOpCreateHandleName)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpSampleFunc creates the dx.op.sample function declaration.
// 11 params: opcode, tex, sampler, c0-c3, o0-o2, clamp
func (e *Emitter) getDxOpSampleFunc(ol overloadType) *module.Function {
	name := "dx.op.sample"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	params := e.sampleBaseParams(3)
	params = append(params, e.mod.GetFloatType(32)) // clamp
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpSampleBiasFunc: dx.op.sampleBias.XX
// 12 params: opcode, tex, sampler, c0-c3, o0-o2, bias, clamp
func (e *Emitter) getDxOpSampleBiasFunc(ol overloadType) *module.Function {
	name := "dx.op.sampleBias"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	f32 := e.mod.GetFloatType(32)
	params := e.sampleBaseParams(3)
	params = append(params, f32, f32) // bias + clamp
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpSampleLevelFunc: dx.op.sampleLevel.XX
// 11 params: opcode, tex, sampler, c0-c3, o0-o2, LOD
func (e *Emitter) getDxOpSampleLevelFunc(ol overloadType) *module.Function {
	name := "dx.op.sampleLevel"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	params := e.sampleBaseParams(3)
	params = append(params, e.mod.GetFloatType(32)) // LOD
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpSampleGradFunc: dx.op.sampleGrad.XX
// 17 params: opcode, tex, sampler, c0-c3, o0-o2, ddx0-2, ddy0-2, clamp
func (e *Emitter) getDxOpSampleGradFunc(ol overloadType) *module.Function {
	name := "dx.op.sampleGrad"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	f32 := e.mod.GetFloatType(32)
	params := e.sampleBaseParams(3)
	params = append(params, f32, f32, f32, f32, f32, f32, f32) // ddx0-2, ddy0-2, clamp
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpSampleCmpFunc: dx.op.sampleCmp.XX
// 12 params: opcode, tex, sampler, c0-c3, o0-o2, compare, clamp
func (e *Emitter) getDxOpSampleCmpFunc(ol overloadType) *module.Function {
	name := "dx.op.sampleCmp"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	f32 := e.mod.GetFloatType(32)
	params := e.sampleBaseParams(3)
	params = append(params, f32, f32) // compare + clamp
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpSampleCmpLevelZeroFunc: dx.op.sampleCmpLevelZero.XX
// 11 params: opcode, tex, sampler, c0-c3, o0-o2, compare
func (e *Emitter) getDxOpSampleCmpLevelZeroFunc(ol overloadType) *module.Function {
	name := "dx.op.sampleCmpLevelZero"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	params := e.sampleBaseParams(3)
	params = append(params, e.mod.GetFloatType(32)) // compare
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpTextureGatherFunc: dx.op.textureGather.XX
// 10 params: opcode, tex, sampler, c0-c3, o0, o1, channel
func (e *Emitter) getDxOpTextureGatherFunc(ol overloadType) *module.Function {
	name := "dx.op.textureGather"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	params := e.sampleBaseParams(2)               // only 2 offsets for gather
	params = append(params, e.mod.GetIntType(32)) // channel
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// getDxOpTextureGatherCmpFunc: dx.op.textureGatherCmp.XX
// 11 params: opcode, tex, sampler, c0-c3, o0, o1, channel, compare
func (e *Emitter) getDxOpTextureGatherCmpFunc(ol overloadType) *module.Function {
	name := "dx.op.textureGatherCmp"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	i32 := e.mod.GetIntType(32)
	f32 := e.mod.GetFloatType(32)
	params := e.sampleBaseParams(2)   // only 2 offsets for gather
	params = append(params, i32, f32) // channel + compare
	funcTy := e.mod.GetFunctionType(e.getDxResRetType(ol), params)
	return e.getOrCreateDxOpFunc(name, ol, funcTy)
}

// sampleBaseParams returns the common prefix for all sample-family function signatures:
// [i32 opcode, %handle tex, %handle sampler, float c0..c3, i32 offset x numOffsets]
func (e *Emitter) sampleBaseParams(numOffsets int) []*module.Type {
	i32 := e.mod.GetIntType(32)
	f32 := e.mod.GetFloatType(32)
	h := e.getDxHandleType()
	params := make([]*module.Type, 0, 7+numOffsets)
	params = append(params,
		i32,                // opcode
		h,                  // texture handle
		h,                  // sampler handle
		f32, f32, f32, f32, // coord0..3
	)
	for range numOffsets {
		params = append(params, i32)
	}
	return params
}

// ensureFloat checks if an expression's type is integer and casts to f32 if needed.
// DXIL sample intrinsics require float for LOD, bias, compare values, but WGSL
// may pass integer expressions (e.g., `let level = 1` for textureSampleLevel on depth textures).
func (e *Emitter) ensureFloat(fn *ir.Function, valueID int, handle ir.ExpressionHandle) int {
	exprType := e.resolveExprType(fn, handle)
	if sc, ok := exprType.(ir.ScalarType); ok {
		if sc.Kind == ir.ScalarSint || sc.Kind == ir.ScalarUint || sc.Kind == ir.ScalarAbstractInt {
			f32Ty := e.mod.GetFloatType(32)
			castOp := CastSIToFP
			if sc.Kind == ir.ScalarUint {
				castOp = CastUIToFP
			}
			return e.addCastInstr(f32Ty, castOp, valueID)
		}
	}
	return valueID
}

// getI1ConstID returns the emitter value ID for a cached i1 constant.
//
//nolint:unparam // generic helper kept for future i1 true/1 emission
func (e *Emitter) getI1ConstID(v int64) int {
	key := v + (1 << 41) // offset to distinguish from i32 and i8
	if id, ok := e.intConsts[key]; ok {
		return id
	}
	c := e.mod.AddIntConst(e.mod.GetIntType(1), v)
	id := e.allocValue()
	e.intConsts[key] = id
	e.constMap[id] = c
	return id
}

// --- dx.resources metadata ---

// emitResourceMetadata emits the !dx.resources named metadata node.
// Called from emitMetadata when resources are present.
//
// Format: !dx.resources = !{!srvs, !uavs, !cbvs, !samplers}
// Each class has different field counts matching the DXIL spec.
//
// Reference: Mesa nir_to_dxil.c emit_srv_metadata/emit_uav_metadata/emit_cbv_metadata/emit_sampler_metadata
func (e *Emitter) emitResourceMetadata() *module.MetadataNode {
	if len(e.resources) == 0 {
		return nil
	}

	// Group resources by class.
	var srvs, uavs, cbvs, samplers []*module.MetadataNode

	for i := range e.resources {
		res := &e.resources[i]

		// Skip virtual entries — synthetic per-WGSL-binding sampler slots
		// that only cache the materialized handleID for resolveResourceHandle.
		// The actual sampler heap resource is already present.
		if res.virtual {
			continue
		}

		switch res.class {
		case resourceClassSRV:
			srvs = append(srvs, e.buildSRVMetadata(res))
		case resourceClassUAV:
			uavs = append(uavs, e.buildUAVMetadata(res))
		case resourceClassCBV:
			cbvs = append(cbvs, e.buildCBVMetadata(res))
		case resourceClassSampler:
			samplers = append(samplers, e.buildSamplerMetadata(res))
		}
	}

	// Build the 4-element tuple: !{!srvs, !uavs, !cbvs, !samplers}
	// Each element is either a tuple of entries or null.
	elements := [4][]*module.MetadataNode{srvs, uavs, cbvs, samplers}
	mdParts := make([]*module.MetadataNode, 4)
	for i, list := range elements {
		if len(list) > 0 {
			mdParts[i] = e.mod.AddMetadataTuple(list)
		}
		// nil represents null in metadata
	}

	mdResources := e.mod.AddMetadataTuple(mdParts)
	e.mod.AddNamedMetadata("dx.resources", []*module.MetadataNode{mdResources})

	return mdResources
}

// fillResourceMetadataCommon builds the first 6 fields common to all resource classes.
// Returns [6]*MetadataNode: {rangeID, undefPtr, name, space, lowerBound, rangeSize}.
//
// Reference: Mesa nir_to_dxil.c fill_resource_metadata() line ~453
func (e *Emitter) fillResourceMetadataCommon(res *resourceInfo, structType *module.Type) [6]*module.MetadataNode {
	i32Ty := e.mod.GetIntType(32)

	// For binding arrays, wrap the resource struct in an LLVM array type
	// so the validator's HLSL-type walk recognizes it as array-backed
	// (DxilValidationUtils.cpp:209 checks
	// `Res->GetHLSLType()->getPointerElementType()->isArrayTy()` — if
	// false, a non-constant index into the resource is rejected with
	// InstrOpConstRange 'Constant values must be in-range for operation').
	// The element type for the LLVM pointer is therefore:
	//   non-array:          struct.SRVType
	//   binding_array<T,N>: [N x struct.SRVType]
	//   binding_array<T>:   [0 x struct.SRVType] (unbounded)
	elemTypeForPtr := structType
	if res.isBindingArray {
		arrLen := uint(res.arraySize)
		elemTypeForPtr = e.mod.GetArrayType(structType, arrLen)
	}

	// fields[1] = metadata value wrapping an undef pointer to the resource type.
	// DXC validator requires this non-null reference.
	// Reference: Mesa fill_resource_metadata() line ~457-458
	pointerType := e.mod.GetPointerType(elemTypeForPtr)
	pointerUndef := e.mod.AddUndefConst(pointerType)

	// Range size: 1 for non-array, N for bounded binding arrays, 0xFFFFFFFF for unbounded.
	rangeSize := int64(1)
	if res.isBindingArray {
		if res.arraySize > 0 {
			rangeSize = int64(res.arraySize)
		} else {
			rangeSize = int64(0xFFFFFFFF) // unbounded
		}
	}

	return [6]*module.MetadataNode{
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.rangeID))), // fields[0]: resource ID
		e.mod.AddMetadataValue(pointerType, pointerUndef),                // fields[1]: global constant symbol (undef ptr)
		e.mod.AddMetadataString(res.name),                                // fields[2]: name
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.group))),   // fields[3]: space ID
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(res.binding))), // fields[4]: lower bound
		e.mod.AddMetadataValue(i32Ty, e.getIntConst(rangeSize)),          // fields[5]: range size
	}
}

// getResourceStructType returns an LLVM struct type appropriate for this resource.
// For CBV: named struct wrapping a float array sized to the buffer. DXC uses
// "hostlayout.<varName>" for the outer wrapper when the type needs layout
// transformation (matrices, arrays); plain "<varName>" otherwise.
// For SRV/UAV textures: DXC uses HLSL-style class names like
// class.Texture2D<vector<float, 4>> with a nested ::mips_type member.
// For Sampler: struct.SamplerState { i32 }.
//
// Reference:
//   - DXC DxilCondenseResources.cpp:1794 — hostlayout prefix logic
//   - DXC DxilModule.cpp:87 — kHostLayoutTypePrefix = "hostlayout."
//   - Mesa nir_to_dxil.c emit_cbv() line ~1549-1552
func (e *Emitter) getResourceStructType(res *resourceInfo) *module.Type {
	switch res.class {
	case resourceClassCBV:
		return e.getCBVStructType(res)

	case resourceClassSampler:
		// Sampler: struct.SamplerState { i32 }
		// Reference: Mesa nir_to_dxil.c line ~1597-1598
		i32Ty := e.mod.GetIntType(32)
		return e.mod.GetStructType("struct.SamplerState", []*module.Type{i32Ty})

	case resourceClassSRV:
		return e.getSRVStructType(res)

	case resourceClassUAV:
		return e.getUAVStructType(res)

	default:
		// Fallback: i8 pointer struct.
		i8Ty := e.mod.GetIntType(8)
		return e.mod.GetStructType("struct.Resource", []*module.Type{i8Ty})
	}
}

// getCBVStructType builds the CBV wrapper struct type with DXC-matching naming
// and typed member layout.
//
// DXC creates a two-level type hierarchy for CBV resources:
//   - Outer: "hostlayout.<varName>" or just "<varName>" wrapping the inner struct
//   - Inner: "hostlayout.struct.<StructName>" for the concrete data type
//
// DXC applies the "hostlayout." prefix when the struct needs layout
// transformation for cbuffer packing (matrices, arrays with alignment padding).
// For all-scalar structs, no hostlayout prefix is used.
//
// DXC represents struct members with their LLVM types rather than flat float
// arrays. For example, mat4x4<f32> becomes [4 x <4 x float>], vec4<f32>
// becomes <4 x float>, and scalars remain as their primitive types.
func (e *Emitter) getCBVStructType(res *resourceInfo) *module.Type {
	varName := res.name
	if varName == "" {
		varName = "cb"
	}

	// Build typed member list matching DXC convention.
	memberTypes := e.buildCBVMemberTypes(res)

	// Determine whether the CBV type needs hostlayout prefix.
	// DXC applies hostlayout when the struct requires layout transformation
	// for cbuffer packing (matrices, arrays, vectors).
	needsHostLayout := e.cbvNeedsHostLayout(res)

	if needsHostLayout {
		// Build inner struct with hostlayout.struct.<Name> naming when the
		// IR type is a named struct.
		innerName := e.cbvInnerStructName(res)
		if innerName != "" {
			innerTy := e.mod.GetStructType("hostlayout.struct."+innerName, memberTypes)
			return e.mod.GetStructType("hostlayout."+varName, []*module.Type{innerTy})
		}
		return e.mod.GetStructType("hostlayout."+varName, memberTypes)
	}

	// No hostlayout needed: use plain variable name wrapping struct.S<N>.
	innerName := e.cbvInnerStructName(res)
	if innerName != "" {
		innerTy := e.mod.GetStructType("struct."+innerName, memberTypes)
		return e.mod.GetStructType(varName, []*module.Type{innerTy})
	}
	return e.mod.GetStructType(varName, memberTypes)
}

// buildCBVMemberTypes constructs the LLVM type list for a CBV struct's members,
// matching DXC's typed member convention.
//
// DXC convention:
//   - mat{C}x{R}<f32> -> [C x <R x float>]
//   - vec{N}<f32>     -> <N x float>
//   - vec{N}<u32>     -> <N x i32>
//   - scalar f32      -> float
//   - scalar u32/i32  -> i32
//
// For bare (non-struct) CBV types (e.g., var<uniform> m: mat4x4<f32>), the
// result is a single-element slice with that type's LLVM representation.
func (e *Emitter) buildCBVMemberTypes(res *resourceInfo) []*module.Type {
	if int(res.typeHandle) >= len(e.ir.Types) {
		// Fallback: single vec4 register as flat float array.
		f32Ty := e.mod.GetFloatType(32)
		return []*module.Type{e.mod.GetArrayType(f32Ty, 4)}
	}

	irType := e.ir.Types[res.typeHandle]
	if st, ok := irType.Inner.(ir.StructType); ok {
		return e.buildCBVStructMemberTypes(st)
	}
	// Bare non-struct type (e.g., mat4x4 directly as uniform).
	ty := e.irTypeToCBVLLVMType(res.typeHandle)
	if ty != nil {
		return []*module.Type{ty}
	}
	// Fallback for unrecognized types.
	f32Ty := e.mod.GetFloatType(32)
	numVec4 := e.computeCBVVec4Count(res)
	return []*module.Type{e.mod.GetArrayType(f32Ty, uint(numVec4))} //nolint:gosec // numVec4 always positive
}

// buildCBVStructMemberTypes converts each member of an IR struct to its
// corresponding LLVM type for CBV representation.
func (e *Emitter) buildCBVStructMemberTypes(st ir.StructType) []*module.Type {
	result := make([]*module.Type, 0, len(st.Members))
	for _, m := range st.Members {
		if int(m.Type) >= len(e.ir.Types) {
			continue
		}
		ty := e.irTypeToCBVLLVMType(m.Type)
		if ty != nil {
			result = append(result, ty)
		}
	}
	if len(result) == 0 {
		// Fallback: single float if no members could be converted.
		result = append(result, e.mod.GetFloatType(32))
	}
	return result
}

// irTypeToCBVLLVMType converts an IR type (by handle) to its DXC-matching
// LLVM type for CBV struct members.
//
// This follows the DXC convention observed in golden reference files:
//   - MatrixType{C,R,Float32} -> [C x <R x float>]
//   - VectorType{N,Float32}   -> <N x float>
//   - VectorType{N,Uint32}    -> <N x i32>
//   - ScalarType{Float,4}     -> float
//   - ScalarType{Uint/Sint,4} -> i32
func (e *Emitter) irTypeToCBVLLVMType(th ir.TypeHandle) *module.Type {
	if int(th) >= len(e.ir.Types) {
		return nil
	}
	irType := &e.ir.Types[th]
	switch t := irType.Inner.(type) {
	case ir.MatrixType:
		// mat{C}x{R}<T> -> [C x <R x scalarTy>]
		scalarTy := e.scalarToLLVMType(t.Scalar)
		vecTy := e.mod.GetVectorType(scalarTy, uint(t.Rows))
		return e.mod.GetArrayType(vecTy, uint(t.Columns))

	case ir.VectorType:
		// vec{N}<T> -> <N x scalarTy>
		scalarTy := e.scalarToLLVMType(t.Scalar)
		return e.mod.GetVectorType(scalarTy, uint(t.Size))

	case ir.ScalarType:
		return e.scalarToLLVMType(t)

	case ir.ArrayType:
		// Array of elements: [N x elemType]
		if int(t.Base) >= len(e.ir.Types) {
			return nil
		}
		elemTy := e.irTypeToCBVLLVMType(t.Base)
		if elemTy == nil {
			return nil
		}
		arrayLen := uint(0)
		if t.Size.Constant != nil {
			arrayLen = uint(*t.Size.Constant)
		}
		return e.mod.GetArrayType(elemTy, arrayLen)

	case ir.StructType:
		// Nested struct: build recursively with hostlayout.struct.<Name> naming.
		memberTypes := e.buildCBVStructMemberTypes(t)
		typeName := "hostlayout.struct.anon"
		if irType.Name != "" {
			typeName = "hostlayout.struct." + irType.Name
		}
		return e.mod.GetStructType(typeName, memberTypes)

	default:
		return nil
	}
}

// scalarToLLVMType converts an IR ScalarType to the corresponding LLVM type.
func (e *Emitter) scalarToLLVMType(s ir.ScalarType) *module.Type {
	switch s.Kind {
	case ir.ScalarFloat:
		return e.mod.GetFloatType(uint(s.Width) * 8)
	case ir.ScalarUint, ir.ScalarSint:
		return e.mod.GetIntType(uint(s.Width) * 8)
	case ir.ScalarBool:
		return e.mod.GetIntType(1)
	default:
		return e.mod.GetFloatType(32)
	}
}

// cbvNeedsHostLayout returns true if the CBV's underlying type contains
// vectors, matrices, or arrays that require layout transformation for
// cbuffer packing. DXC applies the "hostlayout." prefix in this case.
func (e *Emitter) cbvNeedsHostLayout(res *resourceInfo) bool {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return false
	}
	return e.typeNeedsHostLayout(e.ir.Types[res.typeHandle].Inner)
}

// typeNeedsHostLayout recursively checks if a type contains matrices or
// arrays that would cause DXC to apply hostlayout layout transformation for
// cbuffer packing. Plain vectors do NOT trigger hostlayout in DXC - only
// matrices (which get decomposed into arrays of vectors) and arrays (which
// get 16-byte element alignment padding).
func (e *Emitter) typeNeedsHostLayout(inner ir.TypeInner) bool {
	switch t := inner.(type) {
	case ir.MatrixType:
		return true
	case ir.ArrayType:
		return true
	case ir.StructType:
		for _, m := range t.Members {
			if int(m.Type) < len(e.ir.Types) {
				if e.typeNeedsHostLayout(e.ir.Types[m.Type].Inner) {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// cbvInnerStructName returns the WGSL struct name for the CBV's underlying type.
// Returns empty string if the type is not a named struct.
func (e *Emitter) cbvInnerStructName(res *resourceInfo) string {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return ""
	}
	ty := e.ir.Types[res.typeHandle]
	if _, ok := ty.Inner.(ir.StructType); ok && ty.Name != "" {
		return ty.Name
	}
	return ""
}

// getSRVStructType builds the SRV struct type with DXC-matching HLSL class
// names for textures and typed buffers.
//
// DXC naming conventions:
//   - RawBuffer(11):        struct.ByteAddressBuffer { i32 }
//   - StructuredBuffer(12): class.StructuredBuffer<unsigned int> { i32 }
//   - Texture1D:            class.Texture1D<ELEM> { LLVM_ELEM, ::mips_type { i32 } }
//   - Texture2D:            class.Texture2D<ELEM> { LLVM_ELEM, ::mips_type { i32 } }
//   - Texture3D:            class.Texture3D<ELEM> { LLVM_ELEM, ::mips_type { i32 } }
//   - TextureCube:          class.TextureCube<ELEM> { LLVM_ELEM }  (no mips_type)
//   - TextureCubeArray:     class.TextureCubeArray<ELEM> { LLVM_ELEM }
//   - Texture2DMS:          class.Texture2DMS<ELEM, 0> { LLVM_ELEM, ::sample_type { i32 } }
//   - Array variants:       class.Texture2DArray<ELEM> { LLVM_ELEM, ::mips_type { i32 } }
func (e *Emitter) getSRVStructType(res *resourceInfo) *module.Type {
	switch e.resourceKind(res) {
	case 11: // RawBuffer
		return e.mod.GetStructType("struct.ByteAddressBuffer", []*module.Type{e.mod.GetIntType(32)})
	case 12: // StructuredBuffer
		return e.mod.GetStructType("class.StructuredBuffer<unsigned int>", []*module.Type{e.mod.GetIntType(32)})
	}

	// Texture SRV: build class.TextureXD<ELEM> with proper members.
	return e.buildTextureClassType(res, false)
}

// getUAVStructType builds the UAV struct type with DXC-matching names.
//
// DXC naming:
//   - RawBuffer(11): struct.RWByteAddressBuffer { i32 }
//   - Texture UAV:   class.RWTexture2D<ELEM> { LLVM_ELEM }  (no mips/sample member)
func (e *Emitter) getUAVStructType(res *resourceInfo) *module.Type {
	switch e.resourceKind(res) {
	case 11: // RawBuffer
		return e.mod.GetStructType("struct.RWByteAddressBuffer", []*module.Type{e.mod.GetIntType(32)})
	}

	// Texture UAV: build class.RWTextureXD<ELEM>.
	return e.buildTextureClassType(res, true)
}

// buildTextureClassType constructs the DXC HLSL-style class type for a
// texture resource. For read-only textures (SRV), this includes a
// ::mips_type or ::sample_type member; for read-write textures (UAV),
// it's just the element type.
//
// The type name follows DXC's convention:
//
//	class.Texture2D<vector<float, 4>>         (sampled vec4 float)
//	class.TextureCube<float>                   (depth float)
//	class.RWTexture2D<vector<float, 4>>        (storage rgba float)
//	class.RWTexture2D<unsigned int>             (storage r32uint)
func (e *Emitter) buildTextureClassType(res *resourceInfo, isUAV bool) *module.Type {
	// Resolve the image type from the IR.
	var imgType *ir.ImageType
	if int(res.typeHandle) < len(e.ir.Types) {
		inner := unwrapBindingArray(e.ir, e.ir.Types[res.typeHandle].Inner)
		if img, ok := inner.(ir.ImageType); ok {
			imgType = &img
		}
	}

	// Build the HLSL element type name and corresponding LLVM type.
	elemName, elemTy := e.textureElementType(imgType)

	// Build the DXC texture dimension name.
	dimName := e.textureDimensionName(imgType)

	// Construct the full class name.
	var prefix string
	if isUAV {
		prefix = "class.RW"
	} else {
		prefix = "class."
	}

	// Multisampled textures include a sample count parameter in the class name.
	// DXC always uses 0 for the sample count in the type name.
	templateArgs := elemName
	if imgType != nil && imgType.Multisampled {
		templateArgs = elemName + ", 0"
	}

	// DXC adds a space before the closing ">" when the template argument ends
	// with ">" to avoid C++ ">>" tokenization ambiguity. For example:
	//   Texture2D<vector<float, 4> >     (space before outer >)
	//   Texture2DMS<vector<float, 4>, 0> (no space: last char is 0)
	//   TextureCube<float>               (no space: last char is t)
	closingBracket := ">"
	if len(templateArgs) > 0 && templateArgs[len(templateArgs)-1] == '>' {
		closingBracket = " >"
	}

	className := prefix + dimName + "<" + templateArgs + closingBracket

	if isUAV {
		// RW textures have no mips/sample member — just the element.
		return e.mod.GetStructType(className, []*module.Type{elemTy})
	}

	// Determine if this texture has a mips_type or sample_type sub-member.
	// TextureCube and TextureCubeArray have no mips_type in DXC output.
	// Texture2DMS uses ::sample_type instead of ::mips_type.
	hasMips := true
	subTypeSuffix := "mips_type"
	if imgType != nil {
		if imgType.Dim == ir.DimCube {
			hasMips = false
		}
		if imgType.Multisampled {
			subTypeSuffix = "sample_type"
		}
	}

	if !hasMips {
		return e.mod.GetStructType(className, []*module.Type{elemTy})
	}

	// Build the ::mips_type or ::sample_type sub-struct.
	i32Ty := e.mod.GetIntType(32)
	subClassName := className + "::" + subTypeSuffix
	subTy := e.mod.GetStructType(subClassName, []*module.Type{i32Ty})
	return e.mod.GetStructType(className, []*module.Type{elemTy, subTy})
}

// textureElementType returns the HLSL template element name and corresponding
// LLVM type for a texture resource. The naming follows DXC conventions:
//
//	Sampled vec4 float:   "vector<float, 4>"   → <4 x float>
//	Sampled vec4 int:     "vector<int, 4>"     → <4 x i32>
//	Sampled vec4 uint:    "vector<unsigned int, 4>" → <4 x i32>
//	Depth float:          "float"              → float
//	Storage r32float:     "float"              → float
//	Storage rgba8unorm:   "vector<float, 4>"   → <4 x float>
//	Storage r32uint:      "unsigned int"       → i32
//	Storage r64uint:      "unsigned long long" → i64
func (e *Emitter) textureElementType(imgType *ir.ImageType) (string, *module.Type) {
	if imgType == nil {
		// Fallback: assume vec4 float.
		name := "vector<float, 4>"
		ty := e.mod.GetVectorType(e.mod.GetFloatType(32), 4)
		return name, ty
	}

	// Depth textures: always scalar float.
	if imgType.Class == ir.ImageClassDepth {
		return "float", e.mod.GetFloatType(32)
	}

	// Storage textures: derive from storage format.
	if imgType.Class == ir.ImageClassStorage {
		return e.storageTextureElementType(imgType)
	}

	// Sampled textures: always vec4 of the sampled kind.
	return e.sampledTextureElementType(imgType.SampledKind)
}

// sampledTextureElementType returns the HLSL element name and LLVM type for
// a sampled texture. DXC always uses 4-component vectors for sampled textures.
func (e *Emitter) sampledTextureElementType(kind ir.ScalarKind) (string, *module.Type) {
	switch kind {
	case ir.ScalarSint:
		ty := e.mod.GetVectorType(e.mod.GetIntType(32), 4)
		return "vector<int, 4>", ty
	case ir.ScalarUint:
		ty := e.mod.GetVectorType(e.mod.GetIntType(32), 4)
		return "vector<unsigned int, 4>", ty
	default: // ScalarFloat
		ty := e.mod.GetVectorType(e.mod.GetFloatType(32), 4)
		return "vector<float, 4>", ty
	}
}

// storageTextureElementType returns the HLSL element name and LLVM type for
// a storage texture, derived from its StorageFormat.
// DXC promotes all multi-channel formats (2+ channels) to vec4 in the type
// declaration — HLSL has no float2 RWTexture types, only float and float4.
func (e *Emitter) storageTextureElementType(img *ir.ImageType) (string, *module.Type) {
	channels := storageFormatChannelCount(img.StorageFormat)
	scalar := img.StorageFormat.Scalar()

	scalarName := hlslScalarTypeName(scalar)
	llvmScalar := scalarToDXIL(e.mod, scalar)

	if channels == 1 {
		return scalarName, llvmScalar
	}

	// DXC promotes 2- and 3-channel formats to 4-component vectors.
	// HLSL RWTexture types only support scalar or float4/int4/uint4.
	vecName := fmt.Sprintf("vector<%s, 4>", scalarName)
	vecTy := e.mod.GetVectorType(llvmScalar, 4)
	return vecName, vecTy
}

// DXC texture dimension name constants used in HLSL class type names.
const dxcTexture2DName = "Texture2D"

// textureDimensionName returns the DXC HLSL texture dimension name (without
// the "Texture" prefix class wrapper).
func (e *Emitter) textureDimensionName(imgType *ir.ImageType) string {
	if imgType == nil {
		return dxcTexture2DName
	}

	switch imgType.Dim {
	case ir.Dim1D:
		if imgType.Arrayed {
			return "Texture1DArray"
		}
		return "Texture1D"
	case ir.Dim2D:
		if imgType.Multisampled {
			if imgType.Arrayed {
				return "Texture2DMSArray"
			}
			return "Texture2DMS"
		}
		if imgType.Arrayed {
			return "Texture2DArray"
		}
		return dxcTexture2DName
	case ir.Dim3D:
		return "Texture3D"
	case ir.DimCube:
		if imgType.Arrayed {
			return "TextureCubeArray"
		}
		return "TextureCube"
	default:
		return dxcTexture2DName
	}
}

// hlslScalarTypeName returns the HLSL scalar type name for use in DXC class
// template parameters.
func hlslScalarTypeName(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarSint:
		if s.Width == 8 {
			return "long long"
		}
		return "int"
	case ir.ScalarUint:
		if s.Width == 8 {
			return "unsigned long long"
		}
		return "unsigned int"
	default: // ScalarFloat
		if s.Width == 8 {
			return "double"
		}
		if s.Width == 2 {
			return "half"
		}
		return "float"
	}
}

// storageFormatChannelCount returns the number of channels in a storage format.
func storageFormatChannelCount(f ir.StorageFormat) int {
	switch f {
	// 1-channel formats
	case ir.StorageFormatR8Unorm, ir.StorageFormatR8Snorm,
		ir.StorageFormatR8Uint, ir.StorageFormatR8Sint,
		ir.StorageFormatR16Uint, ir.StorageFormatR16Sint, ir.StorageFormatR16Float,
		ir.StorageFormatR32Uint, ir.StorageFormatR32Sint, ir.StorageFormatR32Float,
		ir.StorageFormatR16Unorm, ir.StorageFormatR16Snorm,
		ir.StorageFormatR64Uint, ir.StorageFormatR64Sint:
		return 1

	// 2-channel formats
	case ir.StorageFormatRg8Unorm, ir.StorageFormatRg8Snorm,
		ir.StorageFormatRg8Uint, ir.StorageFormatRg8Sint,
		ir.StorageFormatRg16Uint, ir.StorageFormatRg16Sint, ir.StorageFormatRg16Float,
		ir.StorageFormatRg32Uint, ir.StorageFormatRg32Sint, ir.StorageFormatRg32Float,
		ir.StorageFormatRg16Unorm, ir.StorageFormatRg16Snorm:
		return 2

	// 4-channel formats (includes packed 3-channel which DXC treats as 4)
	default:
		return 4
	}
}

// buildSRVMetadata builds SRV metadata entry with 9 fields.
//
// Reference: Mesa nir_to_dxil.c emit_srv_metadata() line ~468-492
func (e *Emitter) buildSRVMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	resKind := e.resourceKind(res)
	compType := e.getResourceComponentType(res)

	// fields[6] = resource shape (kind)
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(resKind)))

	// fields[7] = sample count (i32 0). DXC emits this as i32 — not i1
	// boolean — because it represents a count (multi-sample textures carry
	// the MSAA sample count here). Mesa uses i1 but DXC uses i32 0.
	mdSampleCount := e.mod.AddMetadataValue(i32Ty, e.getIntConst(0))

	// fields[8] = element type tag metadata, or null for raw buffer.
	// For typed buffers / images: (0 = TypedBufferElementType, compType).
	// For structured buffers: (1 = StructuredBufferElementStride, stride).
	// Reference: DXC DxilMetadataHelper.cpp:EmitSRVProperties.
	var mdTag *module.MetadataNode
	switch resKind {
	case 11: // RawBuffer — no element tag
		mdTag = nil
	case 12: // StructuredBuffer — (kDxilStructuredBufferElementStrideTag=1, stride)
		stride := e.resourceStructuredStride(res)
		tagNodes := []*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(1)),
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(stride))),
		}
		mdTag = e.mod.AddMetadataTuple(tagNodes)
	default: // Typed buffer or image — (kDxilTypedBufferElementTypeTag=0, compType)
		tagNodes := []*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(0)),
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(compType))),
		}
		mdTag = e.mod.AddMetadataTuple(tagNodes)
	}

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdKind,        // fields[6]: resource shape
		mdSampleCount, // fields[7]: sample count
		mdTag,         // fields[8]: element type tag (or null)
	}

	return e.mod.AddMetadataTuple(fields)
}

// resourceStructuredStride returns the element stride in bytes for a
// StructuredBuffer SRV/UAV. For the synthesized sampler-heap index buffer
// the element type is u32 (4 bytes). For real IR-backed structured
// buffers this would derive from the struct size, but WGSL storage buffers
// currently lower to RawBuffer kind (11), not StructuredBuffer (12), so
// the synthetic path is the only caller today.
func (e *Emitter) resourceStructuredStride(res *resourceInfo) uint32 {
	// TODO: derive stride from res.typeHandle for real structured buffers
	// once the SRV classifier ever chooses kind 12 for user code.
	_ = res
	return 4
}

// buildUAVMetadata builds UAV metadata entry with 11 fields.
//
// Reference: Mesa nir_to_dxil.c emit_uav_metadata() line ~494-521
func (e *Emitter) buildUAVMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	resKind := e.resourceKind(res)
	compType := e.getResourceComponentType(res)

	// fields[6] = resource shape
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(resKind)))

	// fields[7] = globally coherent (i1 false)
	// Reference: Mesa emit_uav_metadata() line ~507
	mdCoherent := e.addMetadataI1(false)

	// fields[8] = has counter (i1 false)
	mdCounter := e.addMetadataI1(false)

	// fields[9] = is ROV (i1 false)
	mdROV := e.addMetadataI1(false)

	// fields[10] = element type tag (or null for raw/structured buffer)
	// Reference: Mesa emit_uav_metadata() line ~510-518
	var mdTag *module.MetadataNode
	if resKind != 11 && resKind != 12 { // not RawBuffer or StructuredBuffer
		tagNodes := []*module.MetadataNode{
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(0)), // DXIL_TYPED_BUFFER_ELEMENT_TYPE_TAG = 0
			e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(compType))),
		}
		mdTag = e.mod.AddMetadataTuple(tagNodes)
	}

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdKind,     // fields[6]: resource shape
		mdCoherent, // fields[7]: globally coherent
		mdCounter,  // fields[8]: has counter
		mdROV,      // fields[9]: is ROV
		mdTag,      // fields[10]: element type tag (or null)
	}

	return e.mod.AddMetadataTuple(fields)
}

// buildCBVMetadata builds CBV metadata entry with 8 fields.
// fields[6] = constant buffer size in bytes (NOT resource kind).
//
// Reference: Mesa nir_to_dxil.c emit_cbv_metadata() line ~523-535
func (e *Emitter) buildCBVMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	// fields[6] = constant buffer size in bytes. DXC uses the actual
	// struct size (not rounded to vec4 boundary). For push-constants
	// with a single f32, this is 4 (not 16). computeCBVSizeBytes
	// returns the raw struct byte count.
	cbvSizeBytes := e.computeCBVSizeBytes(res)
	mdSize := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(cbvSizeBytes)))

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdSize, // fields[6]: constant buffer size in bytes
		nil,    // fields[7]: null (metadata)
	}

	return e.mod.AddMetadataTuple(fields)
}

// buildSamplerMetadata builds Sampler metadata entry with 8 fields.
//
// Reference: Mesa nir_to_dxil.c emit_sampler_metadata() line ~537-551
func (e *Emitter) buildSamplerMetadata(res *resourceInfo) *module.MetadataNode {
	structType := e.getResourceStructType(res)
	common := e.fillResourceMetadataCommon(res, structType)
	i32Ty := e.mod.GetIntType(32)

	// fields[6] = sampler kind (0=default, 1=comparison).
	// Reference: Mesa emit_sampler_metadata() line ~545-547;
	// DXC DxilConstants.h enum SamplerKind. The validator enforces this
	// via the rule 'sample_c_*/gather_c instructions require sampler
	// declared in comparison mode' — any sampleCmp/gatherCmp touching
	// a Default-kind sampler is rejected. WGSL models this through
	// ir.SamplerType.Comparison (true for `sampler_comparison`), so read
	// it directly off the resource's type handle.
	samplerKind := 0 // DXIL_SAMPLER_KIND_DEFAULT
	if res.comparisonSampler {
		samplerKind = 1 // DXIL_SAMPLER_KIND_COMPARISON — synthetic sampler heap
	} else if int(res.typeHandle) < len(e.ir.Types) {
		inner := unwrapBindingArray(e.ir, e.ir.Types[res.typeHandle].Inner)
		if st, ok := inner.(ir.SamplerType); ok && st.Comparison {
			samplerKind = 1 // DXIL_SAMPLER_KIND_COMPARISON
		}
	}
	mdKind := e.mod.AddMetadataValue(i32Ty, e.getIntConst(int64(samplerKind)))

	fields := []*module.MetadataNode{
		common[0], common[1], common[2], common[3], common[4], common[5],
		mdKind, // fields[6]: sampler kind
		nil,    // fields[7]: null (metadata)
	}

	return e.mod.AddMetadataTuple(fields)
}

// addMetadataI1 creates a metadata value node wrapping an i1 boolean constant.
//
// Reference: Mesa dxil_module.c dxil_get_metadata_int1() line ~2897
func (e *Emitter) addMetadataI1(value bool) *module.MetadataNode {
	i1Ty := e.mod.GetIntType(1)
	var v int64
	if value {
		v = 1
	}
	c := e.mod.AddIntConst(i1Ty, v)
	return e.mod.AddMetadataValue(i1Ty, c)
}

// computeCBVSizeBytes returns the actual byte size of the CBV struct, without
// rounding to vec4 boundary. DXC uses this value directly in the resource
// metadata field[6]. For a struct with a single f32, this returns 4.
func (e *Emitter) computeCBVSizeBytes(res *resourceInfo) int {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return 16 // default: 1 vec4 register
	}
	sizeBytes := e.computeIRTypeSizeBytes(e.ir.Types[res.typeHandle].Inner)
	if sizeBytes == 0 {
		return 16 // default
	}
	return int(sizeBytes)
}

// computeCBVVec4Count returns the number of float elements in the CBV array
// representation. For the struct backing this CBV, we compute the total size
// in bytes and then convert to float count: numFloats = ceil(structSize / 4).
//
// Reference: Mesa nir_to_dxil.c emit_cbv() line ~1549-1550:
//
//	array_type = dxil_module_get_array_type(m, float32, size)
//	where size = number of float elements
func (e *Emitter) computeCBVVec4Count(res *resourceInfo) int {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return 4 // default: 1 vec4 register
	}
	sizeBytes := e.computeIRTypeSizeBytes(e.ir.Types[res.typeHandle].Inner)
	if sizeBytes == 0 {
		return 4 // default
	}
	// Round up to 16-byte boundary (cbuffer register size), then convert to float count.
	aligned := (sizeBytes + 15) &^ 15
	return int(aligned / 4)
}

// computeIRTypeSizeBytes computes the size in bytes of an IR type.
// Used for CBV buffer size calculation.
func (e *Emitter) computeIRTypeSizeBytes(inner ir.TypeInner) uint32 {
	switch t := inner.(type) {
	case ir.ScalarType:
		return uint32(t.Width)
	case ir.VectorType:
		return uint32(t.Size) * uint32(t.Scalar.Width)
	case ir.MatrixType:
		// Each column is a vector of t.Rows components.
		colSize := uint32(t.Rows) * uint32(t.Scalar.Width)
		// Columns are aligned to 16 bytes in cbuffers.
		colAligned := (colSize + 15) &^ 15
		return uint32(t.Columns) * colAligned
	case ir.ArrayType:
		if int(t.Base) < len(e.ir.Types) {
			elemSize := e.computeIRTypeSizeBytes(e.ir.Types[t.Base].Inner)
			arrayLen := uint32(0)
			if t.Size.Constant != nil {
				arrayLen = *t.Size.Constant
			}
			// Use stride if available, otherwise compute from element size.
			if t.Stride > 0 {
				return arrayLen * t.Stride
			}
			return arrayLen * elemSize
		}
		return 0
	case ir.StructType:
		if len(t.Members) == 0 {
			return 0
		}
		// Use the last member's offset + size.
		lastMember := &t.Members[len(t.Members)-1]
		lastSize := uint32(0)
		if int(lastMember.Type) < len(e.ir.Types) {
			lastSize = e.computeIRTypeSizeBytes(e.ir.Types[lastMember.Type].Inner)
		}
		return lastMember.Offset + lastSize
	default:
		return 4 // fallback: assume 4 bytes
	}
}

// DXIL component type constants.
// Reference: DXC include/dxc/DXIL/DxilConstants.h enum ComponentType, line ~164
const (
	dxilCompTypeI32      = 4  // DXIL_COMP_TYPE_I32
	dxilCompTypeU32      = 5  // DXIL_COMP_TYPE_U32
	dxilCompTypeF16      = 8  // DXIL_COMP_TYPE_F16
	dxilCompTypeF32      = 9  // DXIL_COMP_TYPE_F32
	dxilCompTypeF64      = 10 // DXIL_COMP_TYPE_F64
	dxilCompTypeSNormF16 = 11 // DXIL_COMP_TYPE_SNORM_F16
	dxilCompTypeUNormF16 = 12 // DXIL_COMP_TYPE_UNORM_F16
	dxilCompTypeSNormF32 = 13 // DXIL_COMP_TYPE_SNORM_F32
	dxilCompTypeUNormF32 = 14 // DXIL_COMP_TYPE_UNORM_F32
)

// getResourceComponentType returns the DXIL component type for a resource.
//
// ir.ImageType.SampledKind is documented as 'only valid when
// Class == ImageClassSampled' (ir.go:380). For depth and storage image
// classes the IR parser leaves it at the zero value (ScalarSint) which
// would otherwise incorrectly resolve to I32 here. Mirror imageOverload
// (commit 21466c5 for the same class of bug):
//   - Depth textures → always F32.
//   - Storage textures → derive from StorageFormat.Scalar().
//   - Sampled textures → SampledKind is authoritative.
//
// Getting this wrong trips 'sample_* instructions require resource to
// be declared to return UNORM, SNORM or FLOAT' in the validator.
func (e *Emitter) getResourceComponentType(res *resourceInfo) int {
	if int(res.typeHandle) >= len(e.ir.Types) {
		return dxilCompTypeF32 // default
	}
	inner := e.ir.Types[res.typeHandle].Inner
	// Unwrap binding array to get the base type.
	if ba, ok := inner.(ir.BindingArrayType); ok {
		if int(ba.Base) < len(e.ir.Types) {
			inner = e.ir.Types[ba.Base].Inner
		}
	}
	if img, ok := inner.(ir.ImageType); ok {
		switch img.Class {
		case ir.ImageClassDepth:
			return dxilCompTypeF32
		case ir.ImageClassStorage:
			return e.storageFormatToDxilCompType(img.StorageFormat)
		default:
			return e.sampledKindToDxilCompType(img.SampledKind)
		}
	}
	return dxilCompTypeF32
}

// storageFormatToDxilCompType maps an IR StorageFormat to DXIL ComponentType,
// distinguishing unorm/snorm/float. DXC marks unorm storage textures as
// UNormF32 (14) and snorm as SNormF32 (13) in the extended resource properties
// metadata. Getting this wrong causes dxc -dumpbin to show "UAV f32" instead
// of "UAVunorm_f32" in the Resource Bindings table and mismatches the metadata.
func (e *Emitter) storageFormatToDxilCompType(sf ir.StorageFormat) int {
	if sf.IsUnorm() {
		return dxilCompTypeUNormF32
	}
	if sf.IsSnorm() {
		return dxilCompTypeSNormF32
	}
	return e.sampledKindToDxilCompType(sf.Scalar().Kind)
}

// sampledKindToDxilCompType converts an IR ScalarKind to DXIL component type.
// Used for image/texture types where we only have the kind, not full scalar type.
func (e *Emitter) sampledKindToDxilCompType(kind ir.ScalarKind) int {
	switch kind {
	case ir.ScalarFloat:
		return dxilCompTypeF32
	case ir.ScalarSint:
		return dxilCompTypeI32
	case ir.ScalarUint:
		return dxilCompTypeU32
	default:
		return dxilCompTypeF32
	}
}

// --- CBV (constant buffer) loads ---

// cbvPointerChain describes a resolved pointer chain leading to a CBV field.
type cbvPointerChain struct {
	varHandle  ir.GlobalVariableHandle
	byteOffset uint32        // accumulated byte offset into the struct (static part)
	fieldType  ir.TypeInner  // the IR type of the final accessed field
	scalar     ir.ScalarType // scalar element type for overload selection
	// dynIndexExpr and dynStride are set when the chain contains a dynamic Access.
	// The total byte offset = dynIndex * dynStride + byteOffset.
	dynIndexExpr ir.ExpressionHandle // dynamic array index expression (0 = no dynamic)
	dynStride    uint32              // array element stride in bytes (0 = no dynamic)
	hasDynIndex  bool                // true if this chain has a dynamic index component
}

// resolveCBVPointerChain walks an expression chain (AccessIndex → GlobalVariable)
// and determines whether it leads to a CBV global variable. If so, it returns
// the CBV pointer chain info including the accumulated byte offset.
//
// Supported patterns:
//   - ExprGlobalVariable (load entire CBV struct — offset 0)
//   - ExprAccessIndex → ExprGlobalVariable (load struct field)
//   - ExprAccessIndex → ExprAccessIndex → ExprGlobalVariable (nested struct/vector)
//
// Reference: Mesa nir_to_dxil.c emit_load_ubo_vec4() — offset is passed as
// register index (byte_offset / 16).
func (e *Emitter) resolveCBVPointerChain(fn *ir.Function, ptrHandle ir.ExpressionHandle) (*cbvPointerChain, bool) {
	// Walk the expression chain to find the root global variable and accumulate offsets.
	var indices []uint32
	handle := ptrHandle
	var dynIndexExpr ir.ExpressionHandle
	var dynStride uint32
	hasDynIndex := false

	for {
		if int(handle) >= len(fn.Expressions) {
			return nil, false
		}
		expr := fn.Expressions[handle]

		switch ek := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			// Found root — check if it's a CBV.
			if _, found := e.resourceHandles[ek.Variable]; !found {
				return nil, false
			}
			idx := e.resourceHandles[ek.Variable]
			res := &e.resources[idx]
			if res.class != resourceClassCBV {
				return nil, false
			}

			// Now walk indices forward to compute byte offset.
			// When hasDynIndex is true, the dynamic Access already handles the
			// array dimension, so we start from the array's element type.
			byteOffset, fieldType, scalar, ok := e.computeCBVFieldOffset(ek.Variable, indices, hasDynIndex)
			if !ok {
				return nil, false
			}

			return &cbvPointerChain{
				varHandle:    ek.Variable,
				byteOffset:   byteOffset,
				fieldType:    fieldType,
				scalar:       scalar,
				dynIndexExpr: dynIndexExpr,
				dynStride:    dynStride,
				hasDynIndex:  hasDynIndex,
			}, true

		case ir.ExprAccessIndex:
			indices = append([]uint32{ek.Index}, indices...)
			handle = ek.Base

		case ir.ExprAccess:
			// Dynamic array access: we need the array stride to compute the offset.
			// The stride comes from the type of the base expression's inner array.
			if !hasDynIndex {
				// Only support one level of dynamic access.
				stride := e.resolveArrayStride(fn, ek.Base)
				if stride == 0 {
					return nil, false
				}
				dynIndexExpr = ek.Index
				dynStride = stride
				hasDynIndex = true
				handle = ek.Base
			} else {
				// Multiple dynamic accesses — not supported.
				return nil, false
			}

		default:
			return nil, false
		}
	}
}

// resolveArrayStride determines the array stride for a given expression's type.
// Used when computing dynamic CBV offsets for array[i] access patterns.
func (e *Emitter) resolveArrayStride(fn *ir.Function, exprHandle ir.ExpressionHandle) uint32 {
	if int(exprHandle) >= len(fn.Expressions) {
		return 0
	}
	expr := fn.Expressions[exprHandle]
	gv, ok := expr.Kind.(ir.ExprGlobalVariable)
	if !ok {
		return 0
	}
	if int(gv.Variable) >= len(e.ir.GlobalVariables) {
		return 0
	}
	gvDef := &e.ir.GlobalVariables[gv.Variable]
	if int(gvDef.Type) >= len(e.ir.Types) {
		return 0
	}
	inner := e.ir.Types[gvDef.Type].Inner
	if arr, ok := inner.(ir.ArrayType); ok {
		return arr.Stride
	}
	return 0
}

// resolveCBVRegIndex computes the CBV register index and component offset for
// a cbufferLoadLegacy call. Handles both static and dynamic (stride-based) indices.
// Returns (regIndexValueID, componentOffset, error).
func (e *Emitter) resolveCBVRegIndex(fn *ir.Function, chain *cbvPointerChain, scalarWidth uint32) (int, uint32, error) {
	if !chain.hasDynIndex {
		regIndex := chain.byteOffset / 16
		compOffset := (chain.byteOffset % 16) / scalarWidth
		return e.getIntConstID(int64(regIndex)), compOffset, nil
	}

	dynID, err := e.emitExpression(fn, chain.dynIndexExpr)
	if err != nil {
		return 0, 0, fmt.Errorf("CBV dynamic index: %w", err)
	}

	compOffset := (chain.byteOffset % 16) / scalarWidth
	i32Ty := e.mod.GetIntType(32)

	if chain.dynStride%16 == 0 {
		// Stride is register-aligned — simple multiply + constant offset.
		regsPerElem := chain.dynStride / 16
		regIdx := dynID
		if regsPerElem > 1 {
			mulID := e.getIntConstID(int64(regsPerElem))
			regIdx = e.addBinOpInstr(i32Ty, BinOpMul, dynID, mulID)
		}
		if staticRegOff := chain.byteOffset / 16; staticRegOff > 0 {
			offID := e.getIntConstID(int64(staticRegOff))
			regIdx = e.addBinOpInstr(i32Ty, BinOpAdd, regIdx, offID)
		}
		return regIdx, compOffset, nil
	}

	// Non-aligned stride: totalByteOff = dynIndex * stride + byteOffset, regIndex = totalByteOff >> 4.
	strideID := e.getIntConstID(int64(chain.dynStride))
	totalOff := e.addBinOpInstr(i32Ty, BinOpMul, dynID, strideID)
	if chain.byteOffset > 0 {
		offID := e.getIntConstID(int64(chain.byteOffset))
		totalOff = e.addBinOpInstr(i32Ty, BinOpAdd, totalOff, offID)
	}
	shiftID := e.getIntConstID(4) // >> 4 = / 16
	regIdx := e.addBinOpInstr(i32Ty, BinOpLShr, totalOff, shiftID)
	return regIdx, compOffset, nil
}

// computeCBVFieldOffset walks the type hierarchy using the given indices
// to compute the byte offset of the accessed field within the CBV struct.
// When skipArrayLevel is true, the top-level array type is skipped (the array
// dimension is handled by a dynamic index elsewhere).
// Returns (byteOffset, fieldType, scalarType, ok).
func (e *Emitter) computeCBVFieldOffset(varHandle ir.GlobalVariableHandle, indices []uint32, skipArrayLevel bool) (uint32, ir.TypeInner, ir.ScalarType, bool) {
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return 0, nil, ir.ScalarType{}, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return 0, nil, ir.ScalarType{}, false
	}

	currentType := e.ir.Types[gv.Type].Inner
	var byteOffset uint32

	// When a dynamic Access handles the array dimension, skip past the top-level array
	// so the static indices only traverse the element type (struct).
	if skipArrayLevel {
		if arr, ok := currentType.(ir.ArrayType); ok {
			if int(arr.Base) < len(e.ir.Types) {
				currentType = e.ir.Types[arr.Base].Inner
			}
		}
	}

	for _, idx := range indices {
		switch ct := currentType.(type) {
		case ir.StructType:
			if int(idx) >= len(ct.Members) {
				return 0, nil, ir.ScalarType{}, false
			}
			member := &ct.Members[idx]
			byteOffset += member.Offset
			if int(member.Type) >= len(e.ir.Types) {
				return 0, nil, ir.ScalarType{}, false
			}
			currentType = e.ir.Types[member.Type].Inner

		case ir.ArrayType:
			// Array element access: offset = idx * stride.
			byteOffset += idx * ct.Stride
			if int(ct.Base) >= len(e.ir.Types) {
				return 0, nil, ir.ScalarType{}, false
			}
			currentType = e.ir.Types[ct.Base].Inner

		case ir.VectorType:
			// Vector component access: offset = idx * scalar_width.
			byteOffset += idx * uint32(ct.Scalar.Width)
			currentType = ct.Scalar

		case ir.MatrixType:
			// Matrix column access: offset = idx * column_stride.
			// CBV matrix columns are 16-byte aligned (one register per column),
			// even if the column has fewer than 4 components.
			colBytes := uint32(ct.Rows) * uint32(ct.Scalar.Width)
			colStride := (colBytes + 15) &^ 15 // align to 16 bytes
			byteOffset += idx * colStride
			currentType = ir.VectorType{Size: ct.Rows, Scalar: ct.Scalar}

		default:
			return 0, nil, ir.ScalarType{}, false
		}
	}

	// Determine the scalar type of the final field.
	scalar, ok := scalarOfType(currentType)
	if !ok {
		// Default to f32 for struct types that we'll flatten.
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}

	return byteOffset, currentType, scalar, true
}

// emitCBVLoad emits a dx.op.cbufferLoadLegacy call and extracts the needed
// components based on the field type and byte offset.
//
// dx.op.cbufferLoadLegacy signature:
//
//	%dx.types.CBufRet.XX @dx.op.cbufferLoadLegacy.XX(i32 59, %dx.types.Handle handle, i32 regIndex)
//
// regIndex = byteOffset / 16 (each CBV register is 16 bytes = 4 floats).
// The result is a struct with 4 components (for f32/i32). Individual fields
// are extracted with extractvalue at index = (byteOffset % 16) / scalarWidth.
//
// Reference: Mesa nir_to_dxil.c load_ubo() line ~3061, emit_load_ubo_vec4() line ~3527
func (e *Emitter) emitCBVLoad(fn *ir.Function, chain *cbvPointerChain) (int, error) {
	// Get the resource handle for this CBV.
	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return 0, fmt.Errorf("CBV handle not found for global variable %d", chain.varHandle)
	}

	// Matrix types require special handling: each column is in a separate
	// 16-byte CBV register. We emit one cbufferLoadLegacy per column.
	if mt, isMat := chain.fieldType.(ir.MatrixType); isMat {
		return e.emitCBVMatrixLoad(handleID, chain, mt)
	}

	ol := overloadForScalar(chain.scalar)
	scalarWidth := uint32(chain.scalar.Width)

	// Get or create the CBufRet type and function declaration.
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)

	// Determine how many scalar components to extract based on the field type.
	// For structs, we recursively count all scalar members across register boundaries.
	numComps := cbvComponentCount(e.ir, chain.fieldType)

	// If the field spans multiple registers (e.g., struct with >4 scalars),
	// we need multiple cbufferLoadLegacy calls.
	if numComps > 4 {
		return e.emitCBVMultiRegLoad(handleID, chain, ol, scalarWidth, numComps)
	}

	// Single register path: scalar / vector / small struct.
	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))

	regIndexVal, compOffset, dynErr := e.resolveCBVRegIndex(fn, chain, scalarWidth)
	if dynErr != nil {
		return 0, dynErr
	}

	retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIndexVal})

	comps := make([]int, numComps)
	for i := 0; i < numComps; i++ {
		extractIdx := int(compOffset) + i
		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: scalarTy,
			Operands:   []int{retID, extractIdx},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		comps[i] = extractID
	}

	if numComps > 1 {
		e.pendingComponents = comps
	}

	return comps[0], nil
}

// emitCBVMultiRegLoad loads multiple CBV registers to fill all scalar components.
// Used when a struct or large vector spans multiple 16-byte registers.
func (e *Emitter) emitCBVMultiRegLoad(
	handleID int,
	chain *cbvPointerChain,
	ol overloadType,
	scalarWidth uint32,
	totalComps int,
) (int, error) {
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)
	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))

	compsPerReg := 16 / int(scalarWidth)
	baseRegIndex := chain.byteOffset / 16
	baseCompOffset := int((chain.byteOffset % 16) / scalarWidth)

	comps := make([]int, 0, totalComps)

	currentReg := int(baseRegIndex)
	currentComp := baseCompOffset
	remaining := totalComps

	for remaining > 0 {
		regIndexVal := e.getIntConstID(int64(currentReg))
		retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIndexVal})

		// Extract components from this register.
		available := compsPerReg - currentComp
		toExtract := remaining
		if toExtract > available {
			toExtract = available
		}

		for i := 0; i < toExtract; i++ {
			extractIdx := currentComp + i
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, extractIdx},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps = append(comps, extractID)
		}

		remaining -= toExtract
		currentReg++
		currentComp = 0 // subsequent registers start at component 0
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}

	return comps[0], nil
}

// emitCBVMatrixLoad loads a matrix from a CBV by issuing one cbufferLoadLegacy
// per column. Each column occupies one 16-byte register.
//
// For mat4x4<f32>: 4 registers, 4 components each = 16 total components.
// For mat3x3<f32>: 3 registers, 3 components each = 9 total components
// (but each register still returns 4 components; we only use the first 3).
//
// Reference: Mesa nir_to_dxil.c — matrix loads decompose to per-column loads.
func (e *Emitter) emitCBVMatrixLoad(
	handleID int,
	chain *cbvPointerChain,
	mt ir.MatrixType,
) (int, error) {
	ol := overloadForScalar(chain.scalar)
	scalarWidth := uint32(chain.scalar.Width)
	cbufRetTy := e.getDxCBufRetType(ol)
	cbufLoadFn := e.getDxOpCBufLoadFunc(ol)
	scalarTy := e.overloadReturnType(ol)

	cols := int(mt.Columns)
	rows := int(mt.Rows)
	totalComps := cols * rows

	// Each column is aligned to 16 bytes in a cbuffer.
	colAligned := (uint32(rows)*scalarWidth + 15) &^ 15 //nolint:gosec // rows is small (2-4)
	baseRegIndex := chain.byteOffset / 16

	opcodeVal := e.getIntConstID(int64(OpCBufferLoadLegacy))

	comps := make([]int, 0, totalComps)
	for col := 0; col < cols; col++ {
		regIdx := baseRegIndex + uint32(col)*(colAligned/16)
		regIndexVal := e.getIntConstID(int64(regIdx))

		retID := e.addCallInstr(cbufLoadFn, cbufRetTy, []int{opcodeVal, handleID, regIndexVal})

		for row := 0; row < rows; row++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, row},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps = append(comps, extractID)
		}
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}

	return comps[0], nil
}

// getDxCBufRetType returns the %dx.types.CBufRet.XX struct type.
// For f32/i32: 4 fields. For f64/i64: 2 fields. For f16/i16: 8 fields.
//
// Reference: Mesa dxil_module.c dxil_module_get_cbuf_ret_type() line ~706
func (e *Emitter) getDxCBufRetType(ol overloadType) *module.Type {
	scalarTy := e.overloadReturnType(ol)
	name := "dx.types.CBufRet" + overloadSuffix(ol)

	var numFields int
	switch ol {
	case overloadF64, overloadI64:
		numFields = 2
	case overloadF16, overloadI16:
		numFields = 8
		name += ".8"
	default: // f32, i32
		numFields = 4
	}

	fields := make([]*module.Type, numFields)
	for i := range fields {
		fields[i] = scalarTy
	}

	return e.mod.GetStructType(name, fields)
}

// getDxOpCBufLoadFunc creates the dx.op.cbufferLoadLegacy.XX function declaration.
// Signature: %dx.types.CBufRet.XX @dx.op.cbufferLoadLegacy.XX(i32, %dx.types.Handle, i32)
//
// Reference: Mesa nir_to_dxil.c load_ubo() — dxil_get_function("dx.op.cbufferLoadLegacy", overload)
func (e *Emitter) getDxOpCBufLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.cbufferLoadLegacy"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	cbufRetTy := e.getDxCBufRetType(ol)
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty}
	funcTy := e.mod.GetFunctionType(cbufRetTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// --- UAV (storage buffer) operations ---

// uavPointerChain describes a resolved pointer chain leading to a UAV element.
type uavPointerChain struct {
	varHandle  ir.GlobalVariableHandle
	indexExpr  ir.ExpressionHandle // array index expression (dynamic) — used when isConstIndex is false
	constIndex uint32              // constant array index — used when isConstIndex is true
	isConstIdx bool                // true if this is a constant-index access
	isWhole    bool                // true if this is a whole-buffer load (direct GlobalVariable reference)
	elemType   ir.TypeInner        // element type (what's being loaded/stored)
	scalar     ir.ScalarType       // scalar element type for overload selection
	// stride and fieldByteOffset together encode coord0 (in BYTES) per the
	// formula:  coord0 = (constIdx | dynIdx) * stride + fieldByteOffset
	//
	// stride > 0: dynamic-index byte stride (or constIdx byte multiplier).
	// stride == 0: no per-index multiplication; constIndex is treated as
	//              precomputed byte offset directly (used for direct struct field access).
	//
	// All values are in BYTES. The DXIL spec (DXIL.rst:1789) requires bufferStore
	// coord0 in bytes for RWRawBuffer (kind 11), which is how naga registers all
	// storage buffer UAVs/SRVs.
	stride          uint32 // byte stride per index unit (0 = constIndex is precomputed byte offset)
	fieldByteOffset uint32 // additional static byte offset (struct member offset, etc.)
	// dynamicHandleID is set for binding array UAVs where the handle is created
	// at point of use (not at function entry). -1 means use the static handle.
	dynamicHandleID int
	// bindingArrayIndexExpr holds the index into the binding array (for dynamic handle creation).
	bindingArrayIndexExpr ir.ExpressionHandle
	hasBindingArrayIndex  bool
	// isBindingArrayConstIdx is true when the binding array index is a constant (AccessIndex pattern).
	isBindingArrayConstIdx bool
	bindingArrayConstIndex int64
}

// isStorageBufferClass returns true if the given resource class represents
// a storage buffer in DXIL terms — either read-only (SRV, ByteAddressBuffer)
// or read-write (UAV, RWByteAddressBuffer). The dx.op.bufferLoad /
// dx.op.bufferStore opcodes are class-agnostic (they take a handle), so the
// same pointer-chain resolution and emit code applies to both.
// Atomics still require UAV — enforced at the atomic emit site.
func isStorageBufferClass(class uint8) bool {
	return class == resourceClassUAV || class == resourceClassSRV
}

// resolveUAVPointerChain walks an expression chain and determines whether it
// leads to a UAV (storage buffer) global variable. If so, returns chain info.
//
// Supported patterns:
//   - ExprAccess → ExprGlobalVariable (dynamic array index)
//   - ExprAccessIndex → ExprGlobalVariable (constant array index)
//   - ExprGlobalVariable (entire buffer — not typical)
//
// Reference: Mesa nir_to_dxil.c emit_store_ssbo() for storage buffer write patterns
func (e *Emitter) resolveUAVPointerChain(fn *ir.Function, ptrHandle ir.ExpressionHandle) (*uavPointerChain, bool) {
	if int(ptrHandle) >= len(fn.Expressions) {
		return nil, false
	}
	expr := fn.Expressions[ptrHandle]

	switch ek := expr.Kind.(type) {
	case ir.ExprAccess:
		// Dynamic index: data[index] where index is a runtime expression.
		return e.resolveUAVAccessChain(fn, ek.Base, ek.Index, true)

	case ir.ExprAccessIndex:
		// Check for binding array UAV pattern:
		// AccessIndex(fieldIdx, Access(base=GlobalVariable(binding_array), index=arrayIdx))
		if chain, ok := e.resolveBindingArrayUAVChain(fn, ek); ok {
			return chain, true
		}
		// Constant index: data[N] where N is compile-time constant.
		return e.resolveUAVAccessIndexChain(fn, ek.Base, ek.Index)

	case ir.ExprGlobalVariable:
		// Direct reference to the UAV global variable (whole-buffer load/store).
		// This handles patterns like: let x = storage_var; or output = value;
		return e.resolveUAVDirectGlobal(ek.Variable)

	default:
		return nil, false
	}
}

// resolveBindingArrayUAVChain detects the binding array UAV access pattern:
//
//	AccessIndex(fieldIdx, Access(base=GlobalVariable(binding_array), index=arrayIdx))
//
// This occurs for storage buffer arrays like storage_array[index].field.
// Returns a uavPointerChain with binding array index info for dynamic handle creation.
func (e *Emitter) resolveBindingArrayUAVChain(fn *ir.Function, ai ir.ExprAccessIndex) (*uavPointerChain, bool) {
	if int(ai.Base) >= len(fn.Expressions) {
		return nil, false
	}
	baseExpr := fn.Expressions[ai.Base]

	// Pattern 1: AccessIndex(fieldIdx, Access(base=GlobalVariable(binding_array), index=dynamicIdx))
	// This handles storage_array[dynamic_index].field
	if acc, ok := baseExpr.Kind.(ir.ExprAccess); ok {
		return e.resolveBindingArrayUAVChainFromGV(fn, ai, acc.Base, acc.Index, true)
	}

	// Pattern 2: AccessIndex(fieldIdx, AccessIndex(constArrayIdx, GlobalVariable(binding_array)))
	// This handles storage_array[0].field (constant index into binding array)
	if innerAI, ok := baseExpr.Kind.(ir.ExprAccessIndex); ok {
		return e.resolveBindingArrayUAVChainFromGV(fn, ai, innerAI.Base, ir.ExpressionHandle(0), false)
	}

	return nil, false
}

// resolveBindingArrayUAVChainFromGV is the common path for binding array UAV chain detection.
// It checks if gvBase resolves to a binding array UAV global variable and builds the chain.
// If isDynamicIndex is true, indexExpr is the dynamic array index expression.
// If isDynamicIndex is false, the array index comes from a constant AccessIndex (the parent caller).
func (e *Emitter) resolveBindingArrayUAVChainFromGV(
	fn *ir.Function, ai ir.ExprAccessIndex,
	gvBase ir.ExpressionHandle, indexExpr ir.ExpressionHandle,
	isDynamicIndex bool,
) (*uavPointerChain, bool) {
	if int(gvBase) >= len(fn.Expressions) {
		return nil, false
	}
	gvExpr, ok := fn.Expressions[gvBase].Kind.(ir.ExprGlobalVariable)
	if !ok {
		return nil, false
	}

	// Check if this is a binding array UAV.
	resIdx, found := e.resourceHandles[gvExpr.Variable]
	if !found {
		return nil, false
	}
	res := &e.resources[resIdx]
	if !isStorageBufferClass(res.class) || !res.isBindingArray {
		return nil, false
	}

	// Unwrap binding array to get element type.
	gv := &e.ir.GlobalVariables[gvExpr.Variable]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}
	inner := e.ir.Types[gv.Type].Inner
	ba, ok := inner.(ir.BindingArrayType)
	if !ok {
		return nil, false
	}
	if int(ba.Base) >= len(e.ir.Types) {
		return nil, false
	}
	elemInner := e.ir.Types[ba.Base].Inner

	// ai.Index is the struct field index into the element type.
	// For a struct element like Foo { x: u32, far: array<i32> }, field 0 = x, field 1 = far.
	var fieldType ir.TypeInner
	var fieldByteOffset uint32
	if st, ok := elemInner.(ir.StructType); ok && int(ai.Index) < len(st.Members) {
		member := st.Members[ai.Index]
		fieldByteOffset = member.Offset
		if int(member.Type) < len(e.ir.Types) {
			fieldType = e.ir.Types[member.Type].Inner
		}
	}
	if fieldType == nil {
		fieldType = elemInner
	}

	scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	if s, found := deepScalarOfType(e.ir, fieldType); found {
		scalar = s
	}

	chain := &uavPointerChain{
		varHandle:            gvExpr.Variable,
		isConstIdx:           true,
		constIndex:           0,
		elemType:             fieldType,
		scalar:               scalar,
		fieldByteOffset:      fieldByteOffset,
		dynamicHandleID:      -1, // will be created at emit time
		hasBindingArrayIndex: true,
	}

	if isDynamicIndex {
		chain.bindingArrayIndexExpr = indexExpr
	} else {
		// Constant index: get from the inner AccessIndex.
		innerAI := fn.Expressions[ai.Base].Kind.(ir.ExprAccessIndex)
		chain.bindingArrayConstIndex = int64(innerAI.Index)
		chain.isBindingArrayConstIdx = true
	}

	return chain, true
}

// resolveUAVDirectGlobal handles direct reference to a UAV global variable
// (loading/storing the entire buffer content, not a specific element).
// The element type and scalar are derived from the variable's type.
func (e *Emitter) resolveUAVDirectGlobal(varHandle ir.GlobalVariableHandle) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}

	// For a direct GlobalVariable reference, we load the entire type at index 0.
	// We use the full variable type (not element type) since we're loading everything.
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}

	fullType := e.ir.Types[gv.Type].Inner
	scalar, ok := scalarOfType(fullType)
	if !ok {
		// For struct/array types, default to f32 scalar for the overload.
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		// Try to find the actual scalar from nested types.
		if s, found := deepScalarOfType(e.ir, fullType); found {
			scalar = s
		}
	}

	return &uavPointerChain{
		varHandle:  varHandle,
		constIndex: 0,
		isConstIdx: true,
		elemType:   fullType,
		scalar:     scalar,
		isWhole:    true,
	}, true
}

// deepScalarOfType finds the scalar type buried inside structs, arrays, and vectors.
func deepScalarOfType(irMod *ir.Module, inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.AtomicType:
		return t.Scalar, true
	case ir.ArrayType:
		if int(t.Base) < len(irMod.Types) {
			return deepScalarOfType(irMod, irMod.Types[t.Base].Inner)
		}
	case ir.StructType:
		if len(t.Members) > 0 && int(t.Members[0].Type) < len(irMod.Types) {
			return deepScalarOfType(irMod, irMod.Types[t.Members[0].Type].Inner)
		}
	}
	return ir.ScalarType{}, false
}

// resolveUAVIndex computes the coord0 value ID (in BYTES) for a UAV access.
//
// Per DXIL spec (DXIL.rst:1789), bufferStore/bufferLoad on RWRawBuffer (kind 11)
// expect coord0 in BYTES. Naga registers all storage buffers as RWRawBuffer, so
// the raw element/scalar index from WGSL must be byte-scaled before emit.
//
// chain.stride is the byte-stride per dynamic-index unit (or per constIndex unit
// when isConstIdx). chain.fieldByteOffset is the additional static byte offset
// (struct member, etc.). When stride == 0, isConstIdx is treated as a precomputed
// byte offset directly (constIndex is bytes); otherwise constIndex is multiplied.
//
// Final formula:  coord0 = (constIdx | dynIdx) * stride + fieldByteOffset   [bytes]
//
// Reference: DXC HLOperationLower.cpp:4651 — `Coord0 (Index)` already byte-scaled
// from HLSL frontend; same expectation here.
func (e *Emitter) resolveUAVIndex(fn *ir.Function, chain *uavPointerChain) (int, error) {
	if chain.isConstIdx {
		var bytes uint32
		if chain.stride == 0 {
			// constIndex IS the byte offset (struct field direct, etc.)
			bytes = chain.constIndex + chain.fieldByteOffset
		} else {
			bytes = chain.constIndex*chain.stride + chain.fieldByteOffset
		}
		return e.getIntConstID(int64(bytes)), nil
	}

	// Try constant folding: if the index expression resolves to a literal
	// integer (possibly through ExprAlias chains from mem2reg), fold the
	// byte offset at compile time. This matches DXC's SimplifyInst pass
	// which constant-folds `1 * 4` to `4`.
	if constVal, ok := e.tryResolveConstInt(fn, chain.indexExpr); ok {
		var bytes uint64
		if chain.stride == 0 {
			bytes = constVal + uint64(chain.fieldByteOffset)
		} else {
			bytes = constVal*uint64(chain.stride) + uint64(chain.fieldByteOffset)
		}
		return e.getIntConstID(int64(bytes)), nil //nolint:gosec // bytes bounded by shader address space (32-bit)
	}

	dynID, err := e.emitExpression(fn, chain.indexExpr)
	if err != nil {
		return 0, fmt.Errorf("UAV index: %w", err)
	}
	i32Ty := e.mod.GetIntType(32)
	if chain.stride > 1 {
		strideID := e.getIntConstID(int64(chain.stride))
		dynID = e.addBinOpInstr(i32Ty, BinOpMul, dynID, strideID)
	}
	if chain.fieldByteOffset > 0 {
		offID := e.getIntConstID(int64(chain.fieldByteOffset))
		dynID = e.addBinOpInstr(i32Ty, BinOpAdd, dynID, offID)
	}
	return dynID, nil
}

// tryResolveConstInt follows ExprAlias chains from the given expression
// handle to see if it resolves to a constant integer literal. Returns
// the unsigned value and true if successful, (0, false) otherwise.
// This enables constant folding of byte-offset calculations after
// mem2reg promotes inlined function arguments to alias chains pointing
// at literal values.
func (e *Emitter) tryResolveConstInt(fn *ir.Function, h ir.ExpressionHandle) (uint64, bool) {
	const maxDepth = 16
	for depth := 0; depth < maxDepth; depth++ {
		if int(h) >= len(fn.Expressions) {
			return 0, false
		}
		switch ek := fn.Expressions[h].Kind.(type) {
		case ir.Literal:
			switch v := ek.Value.(type) {
			case ir.LiteralI32:
				return uint64(int32(v)), true //nolint:gosec // sign extension intentional for signed index values
			case ir.LiteralU32:
				return uint64(v), true
			case ir.LiteralI64:
				return uint64(int64(v)), true //nolint:gosec // sign extension for signed index
			case ir.LiteralU64:
				return uint64(v), true
			default:
				return 0, false
			}
		case ir.ExprAlias:
			h = ek.Source
		default:
			return 0, false
		}
	}
	return 0, false
}

// resolveUAVAccessChain resolves a dynamic-index access to a UAV global variable.
// Handles patterns:
//   - Access(dynIdx, GlobalVariable) — direct array element access
//   - Access(dynIdx, AccessIndex(structFieldIdx, GlobalVariable)) — struct-wrapped array access
//
// Sets stride to the BYTE stride per dynamic-index unit, and fieldByteOffset to
// the static struct field offset (if AccessIndex peeled a struct member).
//
// Matrix-column write: when the AccessIndex selects a matrix struct member, the
// outer Access dynamically indexes a column. The resolved chain reflects:
//   - elemType = column vector (vec<R>)
//   - stride   = column byte stride (R*scalarW or 16 for matCxR with R=3)
//   - fieldByteOffset = struct member byte offset
//
// This mirrors DXC HLOperationLower.cpp which lowers `m[i] = v` to a single
// rawBufferStore at byte offset (memberOffset + i*columnStride) with mask covering
// all column components — fixes BUG-DXIL-030.
func (e *Emitter) resolveUAVAccessChain(fn *ir.Function, baseHandle, indexHandle ir.ExpressionHandle, _ bool) (*uavPointerChain, bool) {
	if int(baseHandle) >= len(fn.Expressions) {
		return nil, false
	}
	baseExpr := fn.Expressions[baseHandle]

	var varHandle ir.GlobalVariableHandle
	var fieldOffset uint32 // struct member byte offset (0 if no struct wrap)
	var fieldType ir.TypeInner

	switch bk := baseExpr.Kind.(type) {
	case ir.ExprGlobalVariable:
		varHandle = bk.Variable

	case ir.ExprAccessIndex:
		// Access(dynIdx, AccessIndex(structFieldIdx, GlobalVariable))
		// The AccessIndex unwraps the struct wrapper (e.g., .particles field).
		gv, ok := e.resolveToGlobalVariable(fn, bk.Base)
		if !ok {
			return nil, false
		}
		varHandle = gv
		// Look up field byte offset and field type for stride computation below.
		fieldOffset, fieldType = e.resolveStructFieldInfo(varHandle, bk.Index)

	default:
		return nil, false
	}

	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}

	// Determine the effective element type after the dynamic dimension is peeled.
	//
	// Three cases:
	//   1. Field is array<T>: dyn dim peels array → elemType = T, stride = arr.Stride.
	//   2. Field is matrix<C,R>: dyn dim peels matrix → elemType = vec<R>, stride = column.
	//   3. No struct wrap: dyn dim peels the global's outer array. Use resolveUAVElementType.
	var elemType ir.TypeInner
	var scalar ir.ScalarType
	var stride uint32

	switch ft := fieldType.(type) {
	case ir.ArrayType:
		elemType = e.ir.Types[ft.Base].Inner
		stride = ft.Stride
		if stride == 0 {
			stride = elemByteSize(e.ir, elemType)
		}
		if s, ok := scalarOfType(elemType); ok {
			scalar = s
		} else if s, ok := deepScalarOfType(e.ir, elemType); ok {
			scalar = s
		}

	case ir.MatrixType:
		// Matrix column write: dynamic index selects a column (vec<R>).
		// Reference: DXC HLOperationLower.cpp `m[i] = v` lowering to a single
		// rawBufferStore at (memberOffset + i*columnStride), mask covers all
		// R components.
		elemType = ir.VectorType{Size: ft.Rows, Scalar: ft.Scalar}
		stride = matrixColumnByteStride(ft)
		scalar = ft.Scalar

	case ir.VectorType:
		// Dynamic index into a vector member (rare): scalar element with stride = scalar width.
		elemType = ft.Scalar
		stride = uint32(ft.Scalar.Width)
		scalar = ft.Scalar

	case nil:
		// No struct wrap — fall back to resolveUAVElementType for global's outer array.
		var ok bool
		elemType, scalar, ok = e.resolveUAVElementType(varHandle)
		if !ok {
			return nil, false
		}
		stride = elemByteSize(e.ir, elemType)

	default:
		// Other field types (struct member without dynamic indexing handled by other paths).
		var ok bool
		elemType, scalar, ok = e.resolveUAVElementType(varHandle)
		if !ok {
			return nil, false
		}
		stride = elemByteSize(e.ir, elemType)
	}

	return &uavPointerChain{
		varHandle:       varHandle,
		indexExpr:       indexHandle,
		elemType:        elemType,
		scalar:          scalar,
		stride:          stride,
		fieldByteOffset: fieldOffset,
	}, true
}

// resolveStructFieldInfo returns the byte offset and IR type of a struct member,
// unwrapping a top-level struct wrapper around the global variable. Returns
// (0, nil) when the global is not a struct or fieldIdx is out of range.
func (e *Emitter) resolveStructFieldInfo(varHandle ir.GlobalVariableHandle, fieldIdx uint32) (uint32, ir.TypeInner) {
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return 0, nil
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return 0, nil
	}
	st, ok := e.ir.Types[gv.Type].Inner.(ir.StructType)
	if !ok || int(fieldIdx) >= len(st.Members) {
		return 0, nil
	}
	m := &st.Members[fieldIdx]
	if int(m.Type) >= len(e.ir.Types) {
		return m.Offset, nil
	}
	return m.Offset, e.ir.Types[m.Type].Inner
}

// resolveToGlobalVariable walks through an expression to find the underlying GlobalVariable handle.
// Handles chains like AccessIndex(idx, Access(dynIdx, AccessIndex(idx, GlobalVariable)))
// by peeling off intermediate AccessIndex and Access expressions.
func (e *Emitter) resolveToGlobalVariable(fn *ir.Function, handle ir.ExpressionHandle) (ir.GlobalVariableHandle, bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, false
	}
	expr := fn.Expressions[handle]
	switch ek := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		return ek.Variable, true
	case ir.ExprAccessIndex:
		return e.resolveToGlobalVariable(fn, ek.Base)
	case ir.ExprAccess:
		return e.resolveToGlobalVariable(fn, ek.Base)
	default:
		return 0, false
	}
}

// resolveUAVAccessIndexChain resolves a constant-index access to a UAV.
// Handles patterns:
//   - AccessIndex(idx, GlobalVariable) — direct field/element access
//   - AccessIndex(fieldIdx, Access(dynIdx, ...)) — array[i].field
//   - AccessIndex(fieldIdx, AccessIndex(arrIdx, ...)) — array[N].field OR struct.data[N]
func (e *Emitter) resolveUAVAccessIndexChain(fn *ir.Function, baseHandle ir.ExpressionHandle, constIdx uint32) (*uavPointerChain, bool) {
	if int(baseHandle) >= len(fn.Expressions) {
		return nil, false
	}
	baseExpr := fn.Expressions[baseHandle]

	switch bk := baseExpr.Kind.(type) {
	case ir.ExprGlobalVariable:
		// Direct: AccessIndex(idx, GlobalVariable)
		// Check if the variable's type is a struct — if so, this is a struct field access.
		if chain, ok := e.resolveUAVStructFieldDirect(bk.Variable, constIdx); ok {
			return chain, true
		}
		return e.resolveUAVFromGlobal(bk.Variable, constIdx, true)

	case ir.ExprAccess:
		// Nested: AccessIndex(compIdx, Access(dynIdx, ...))
		// Multiple interpretations:
		// (a) Array-of-struct: dynIdx is array element, constIdx is struct field
		// (b) Matrix column+component: dynIdx is column, constIdx is component within column
		// (c) Struct-member array-of-struct: struct.arr[dynIdx].field
		gv, ok := e.resolveToGlobalVariable(fn, bk.Base)
		if !ok {
			return nil, false
		}
		// Try array-of-struct field chain. Also try via specific struct member if the
		// Access base is an AccessIndex selecting a specific struct member with the array.
		if chain, ok := e.resolveUAVStructFieldChainWithMember(fn, gv, bk.Base, constIdx); ok {
			chain.indexExpr = bk.Index
			chain.isConstIdx = false
			return chain, true
		}
		// Try matrix column+component within a struct field.
		// Pattern: AccessIndex(compIdx, Access(dynColIdx, AccessIndex(fieldIdx, GV)))
		if chain, ok := e.resolveUAVMatrixColumnComponent(fn, bk, constIdx); ok {
			return chain, true
		}
		return nil, false

	case ir.ExprAccessIndex:
		// Multiple sub-cases:
		// (a) AccessIndex(fieldIdx, AccessIndex(arrIdx, AccessIndex(structWrap, GV)))
		//     — struct element field access with constant array index (arr[N].field)
		// (b) AccessIndex(arrIdx, AccessIndex(memberIdx, GV))
		//     — array element access within specific struct member (struct.data[N])
		// (c) AccessIndex(arrIdx, AccessIndex(structWrap, GV))
		//     — simple array element access (struct.data[N]) via member[0]
		gv, ok := e.resolveToGlobalVariable(fn, bk.Base)
		if !ok {
			return nil, false
		}
		// First try as struct-in-array field access: constIdx is fieldIdx, bk.Index is arrIdx.
		if chain, ok := e.resolveUAVStructFieldChain(gv, constIdx); ok {
			chain.constIndex = bk.Index
			chain.isConstIdx = true
			return chain, true
		}
		// Try resolving via the specific struct member if the inner AccessIndex
		// selects a member (e.g., bar.data[0] where data is at member index 1).
		if chain, ok := e.resolveUAVFromStructMemberArray(fn, gv, bk, constIdx); ok {
			return chain, true
		}
		// Try matrix column+component with constant indices:
		// AccessIndex(compIdx, AccessIndex(colIdx, AccessIndex(fieldIdx, GV)))
		if chain, ok := e.resolveUAVMatrixConstAccess(fn, bk, constIdx); ok {
			return chain, true
		}
		// Fall back to simple array element access.
		return e.resolveUAVFromGlobal(gv, constIdx, true)
	}

	return nil, false
}

// resolveUAVFromStructMemberArray handles AccessIndex(elemIdx, AccessIndex(memberIdx, ...GV))
// where the struct member at memberIdx is an array type. This occurs in multi-member structs
// like struct Bar { _matrix: mat4x3, data: array<i32> } where data is NOT at member 0.
// The innerAI.Base can be either a direct GV or an AccessIndex chain leading to the GV.
func (e *Emitter) resolveUAVFromStructMemberArray(
	fn *ir.Function, gv ir.GlobalVariableHandle, innerAI ir.ExprAccessIndex, elemIdx uint32,
) (*uavPointerChain, bool) {
	// The inner AccessIndex base should resolve to the GV (direct or through chain).
	if int(innerAI.Base) >= len(fn.Expressions) {
		return nil, false
	}
	// Check if the base is a direct GV or another AccessIndex with the member index.
	baseExpr := fn.Expressions[innerAI.Base]
	memberIdx := innerAI.Index
	switch bk := baseExpr.Kind.(type) {
	case ir.ExprGlobalVariable:
		// Direct: AccessIndex(memberIdx, GV)
	case ir.ExprAccessIndex:
		// Chain: AccessIndex(elemIdx=innerAI.Index, AccessIndex(memberIdx=bk.Index, GV))
		// In this case, innerAI.Index is the element index and bk.Index is the member index.
		_, isInnerGV := fn.Expressions[bk.Base].Kind.(ir.ExprGlobalVariable)
		if !isInnerGV {
			return nil, false
		}
		// Swap: the member index is in the inner AccessIndex, element index is in the outer.
		memberIdx = bk.Index
		elemIdx = innerAI.Index
	default:
		return nil, false
	}

	idx, found := e.resourceHandles[gv]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}
	if int(gv) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gvVar := &e.ir.GlobalVariables[gv]
	if int(gvVar.Type) >= len(e.ir.Types) {
		return nil, false
	}

	inner := e.ir.Types[gvVar.Type].Inner
	st, ok := inner.(ir.StructType)
	if !ok || int(memberIdx) >= len(st.Members) {
		return nil, false
	}

	arrayMember := &st.Members[memberIdx]
	memberInner := e.ir.Types[arrayMember.Type].Inner
	arr, ok := memberInner.(ir.ArrayType)
	if !ok {
		return nil, false
	}

	// Get the element type from the array.
	elemInner := e.ir.Types[arr.Base].Inner
	scalar, ok := scalarOfType(elemInner)
	if !ok {
		if s, f := deepScalarOfType(e.ir, elemInner); f {
			scalar = s
		} else {
			return nil, false
		}
	}

	// constIndex carries the byte offset directly (stride==0 path in resolveUAVIndex).
	byteOffset := arrayMember.Offset + elemIdx*arr.Stride

	return &uavPointerChain{
		varHandle:  gv,
		constIndex: byteOffset,
		isConstIdx: true,
		elemType:   elemInner,
		scalar:     scalar,
	}, true
}

// resolveUAVStructFieldChainWithMember tries resolveUAVStructFieldChain first,
// then falls back to looking at the Access base expression to determine which
// struct member contains the array (for multi-member structs like Bar{_matrix, data}).
func (e *Emitter) resolveUAVStructFieldChainWithMember(
	fn *ir.Function, gv ir.GlobalVariableHandle, accessBase ir.ExpressionHandle, fieldIdx uint32,
) (*uavPointerChain, bool) {
	// First try the standard path (works when member[0] is the array).
	if chain, ok := e.resolveUAVStructFieldChain(gv, fieldIdx); ok {
		return chain, true
	}

	// If the Access base is AccessIndex(memberIdx, ...), use memberIdx to select
	// the correct struct member containing the array.
	if int(accessBase) >= len(fn.Expressions) {
		return nil, false
	}
	ai, ok := fn.Expressions[accessBase].Kind.(ir.ExprAccessIndex)
	if !ok {
		return nil, false
	}

	return e.resolveUAVStructFieldChainAtMember(gv, ai.Index, fieldIdx)
}

// resolveUAVStructFieldChainAtMember resolves an array[dynIdx].field pattern
// where the array is at a specific struct member index (not necessarily member 0).
func (e *Emitter) resolveUAVStructFieldChainAtMember(
	varHandle ir.GlobalVariableHandle, memberIdx, fieldIdx uint32,
) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}

	// Get the struct type and select the correct member.
	inner := e.ir.Types[gv.Type].Inner
	st, ok := inner.(ir.StructType)
	if !ok || int(memberIdx) >= len(st.Members) {
		return nil, false
	}

	arrayMember := &st.Members[memberIdx]
	memberInner := e.ir.Types[arrayMember.Type].Inner
	arr, ok := memberInner.(ir.ArrayType)
	if !ok {
		return nil, false
	}

	// Get the struct element type from the array.
	elemInner := e.ir.Types[arr.Base].Inner
	elemSt, ok := elemInner.(ir.StructType)
	if !ok || int(fieldIdx) >= len(elemSt.Members) {
		// Not a struct element — might be a scalar/vector array. Try as direct element.
		return e.resolveNonStructArrayElement(varHandle, elemInner, arr, arrayMember)
	}

	// Get the field within the element struct.
	member := &elemSt.Members[fieldIdx]
	fieldInner := e.ir.Types[member.Type].Inner
	scalar, ok := scalarOfType(fieldInner)
	if !ok {
		return nil, false
	}

	return &uavPointerChain{
		varHandle:       varHandle,
		elemType:        fieldInner,
		scalar:          scalar,
		stride:          arr.Stride,
		fieldByteOffset: arrayMember.Offset + member.Offset,
	}, true
}

// resolveNonStructArrayElement builds a UAV chain for a non-struct array element
// (scalar or vector element type). Used when AccessIndex targets a non-struct array.
func (e *Emitter) resolveNonStructArrayElement(
	varHandle ir.GlobalVariableHandle, elemInner ir.TypeInner, arr ir.ArrayType, arrayMember *ir.StructMember,
) (*uavPointerChain, bool) {
	scalar, sOk := scalarOfType(elemInner)
	if !sOk {
		scalar, sOk = deepScalarOfType(e.ir, elemInner)
	}
	if !sOk {
		return nil, false
	}
	return &uavPointerChain{
		varHandle:       varHandle,
		elemType:        elemInner,
		scalar:          scalar,
		stride:          arr.Stride,
		fieldByteOffset: arrayMember.Offset,
	}, true
}

// resolveUAVStructFieldChain resolves an array[dynIdx].field pattern on a UAV.
// varHandle is the UAV global variable, fieldIdx is the struct member index.
// Returns a chain with stride and fieldByteOffset set for index computation.
func (e *Emitter) resolveUAVStructFieldChain(varHandle ir.GlobalVariableHandle, fieldIdx uint32) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}

	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}

	// Get the outer type: should be array<Struct> or struct{array<Struct>}.
	inner := e.ir.Types[gv.Type].Inner
	if st, ok := inner.(ir.StructType); ok && len(st.Members) > 0 {
		inner = e.ir.Types[st.Members[0].Type].Inner
	}

	arr, ok := inner.(ir.ArrayType)
	if !ok {
		return nil, false
	}

	// Get the struct element type from the array.
	elemInner := e.ir.Types[arr.Base].Inner
	st, ok := elemInner.(ir.StructType)
	if !ok || int(fieldIdx) >= len(st.Members) {
		return nil, false
	}

	// Get the field type and its byte offset within the struct.
	member := &st.Members[fieldIdx]
	fieldInner := e.ir.Types[member.Type].Inner
	scalar, ok := scalarOfType(fieldInner)
	if !ok {
		return nil, false
	}

	return &uavPointerChain{
		varHandle:       varHandle,
		elemType:        fieldInner,
		scalar:          scalar,
		stride:          arr.Stride,
		fieldByteOffset: member.Offset,
	}, true
}

// resolveUAVMatrixColumnComponent handles the pattern:
//
//	AccessIndex(compIdx, Access(dynColIdx, AccessIndex(fieldIdx, GlobalVariable)))
//
// where the struct field is a matrix type. This occurs with expressions like
// bar._matrix[index].x — dynamic column selection followed by component extraction.
//
// The byte offset is: structFieldOffset + dynColIdx * columnStride + compIdx * scalarWidth.
// This maps to the strided index pattern: index = dynColIdx * (columnStride/scalarW) + staticOffset/scalarW.
func (e *Emitter) resolveUAVMatrixColumnComponent(
	fn *ir.Function, acc ir.ExprAccess, compIdx uint32,
) (*uavPointerChain, bool) {
	if int(acc.Base) >= len(fn.Expressions) {
		return nil, false
	}
	baseExpr := fn.Expressions[acc.Base]
	ai, ok := baseExpr.Kind.(ir.ExprAccessIndex)
	if !ok {
		return nil, false
	}

	// The AccessIndex base should be a GlobalVariable (or chain to one).
	gv, ok := e.resolveToGlobalVariable(fn, ai.Base)
	if !ok {
		return nil, false
	}
	idx, found := e.resourceHandles[gv]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}

	if int(gv) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gvVar := &e.ir.GlobalVariables[gv]
	if int(gvVar.Type) >= len(e.ir.Types) {
		return nil, false
	}

	// The global variable's type should be a struct.
	inner := e.ir.Types[gvVar.Type].Inner
	st, ok := inner.(ir.StructType)
	if !ok || int(ai.Index) >= len(st.Members) {
		return nil, false
	}

	// The accessed member should be a matrix type.
	member := &st.Members[ai.Index]
	fieldInner := e.ir.Types[member.Type].Inner
	mt, ok := fieldInner.(ir.MatrixType)
	if !ok {
		return nil, false
	}

	scalar := mt.Scalar
	scalarW := uint32(scalar.Width)
	if scalarW == 0 {
		scalarW = 4
	}

	// Column stride = rows * scalarWidth, padded to 16 bytes (vec4 alignment in storage layout).
	rows := uint32(mt.Rows)
	columnStride := rows * scalarW
	if columnStride < 16 {
		columnStride = 16 // DXIL raw buffer columns are 16-byte aligned
	}

	// Static byte offset = struct field offset + component * scalarWidth.
	staticByteOffset := member.Offset + compIdx*scalarW

	return &uavPointerChain{
		varHandle:       gv,
		indexExpr:       acc.Index,
		isConstIdx:      false,
		elemType:        scalar,
		scalar:          scalar,
		stride:          columnStride,
		fieldByteOffset: staticByteOffset,
	}, true
}

// resolveUAVMatrixConstAccess handles constant-index matrix column+component access:
//
//	AccessIndex(compIdx, AccessIndex(colIdx, AccessIndex(fieldIdx, GV)))
//
// where the struct field is a matrix type. This is the constant-index variant of
// resolveUAVMatrixColumnComponent (which handles dynamic column index).
// Used for stores like bar._matrix[1].z = 1.0.
func (e *Emitter) resolveUAVMatrixConstAccess(
	fn *ir.Function, outerAI ir.ExprAccessIndex, compIdx uint32,
) (*uavPointerChain, bool) {
	// outerAI = AccessIndex(colIdx, AccessIndex(fieldIdx, GV))
	if int(outerAI.Base) >= len(fn.Expressions) {
		return nil, false
	}
	innerExpr := fn.Expressions[outerAI.Base]
	innerAI, ok := innerExpr.Kind.(ir.ExprAccessIndex)
	if !ok {
		return nil, false
	}

	// innerAI = AccessIndex(fieldIdx, GV)
	gv, ok := e.resolveToGlobalVariable(fn, innerAI.Base)
	if !ok {
		return nil, false
	}
	idx, found := e.resourceHandles[gv]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}
	if int(gv) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gvVar := &e.ir.GlobalVariables[gv]
	if int(gvVar.Type) >= len(e.ir.Types) {
		return nil, false
	}

	inner := e.ir.Types[gvVar.Type].Inner
	st, ok := inner.(ir.StructType)
	if !ok || int(innerAI.Index) >= len(st.Members) {
		return nil, false
	}

	member := &st.Members[innerAI.Index]
	fieldInner := e.ir.Types[member.Type].Inner
	mt, ok := fieldInner.(ir.MatrixType)
	if !ok {
		return nil, false
	}

	scalar := mt.Scalar
	scalarW := uint32(scalar.Width)
	if scalarW == 0 {
		scalarW = 4
	}

	colIdx := outerAI.Index
	byteOffset := member.Offset + colIdx*matrixColumnByteStride(mt) + compIdx*scalarW

	// constIndex carries the byte offset directly (stride==0 path in resolveUAVIndex).
	return &uavPointerChain{
		varHandle:  gv,
		constIndex: byteOffset,
		isConstIdx: true,
		elemType:   scalar,
		scalar:     scalar,
	}, true
}

// resolveUAVStructFieldDirect handles AccessIndex(fieldIdx, GlobalVariable) when the
// UAV's type is a struct (not wrapped in an array). Computes byte offset from struct
// layout and uses the field's actual scalar type for the correct bufferStore overload.
//
// Example: var<storage, read_write> output: S; store to output.field_f16 needs f16 overload.
func (e *Emitter) resolveUAVStructFieldDirect(varHandle ir.GlobalVariableHandle, fieldIdx uint32) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, false
	}

	inner := e.ir.Types[gv.Type].Inner
	st, ok := inner.(ir.StructType)
	if !ok || int(fieldIdx) >= len(st.Members) {
		return nil, false
	}

	member := &st.Members[fieldIdx]
	fieldInner := e.ir.Types[member.Type].Inner
	scalar, ok := scalarOfType(fieldInner)
	if !ok {
		// For array/vector/matrix fields, use deep scalar.
		scalar, ok = deepScalarOfType(e.ir, fieldInner)
		if !ok {
			return nil, false
		}
	}

	// constIndex carries the byte offset directly (stride==0 path in resolveUAVIndex).
	// Per DXIL spec, bufferStore coord0 for RWRawBuffer is in bytes.
	return &uavPointerChain{
		varHandle:  varHandle,
		constIndex: member.Offset,
		isConstIdx: true,
		elemType:   fieldInner,
		scalar:     scalar,
	}, true
}

// resolveUAVFromGlobal checks if a global variable is a UAV and builds a chain.
func (e *Emitter) resolveUAVFromGlobal(varHandle ir.GlobalVariableHandle, constIdx uint32, isConst bool) (*uavPointerChain, bool) {
	idx, found := e.resourceHandles[varHandle]
	if !found {
		return nil, false
	}
	res := &e.resources[idx]
	if !isStorageBufferClass(res.class) {
		return nil, false
	}

	elemType, scalar, ok := e.resolveUAVElementType(varHandle)
	if !ok {
		return nil, false
	}

	// constIdx is the WGSL element index. Convert to byte offset by multiplying
	// by element size — chain.stride==0 means constIndex is precomputed bytes.
	elemBytes := elemByteSize(e.ir, elemType)
	return &uavPointerChain{
		varHandle:  varHandle,
		constIndex: constIdx * elemBytes,
		isConstIdx: isConst,
		elemType:   elemType,
		scalar:     scalar,
	}, true
}

// resolveUAVElementType determines the element type of a UAV's array contents.
// Storage buffers in naga IR are typed as array<T> or struct { array<T> }.
func (e *Emitter) resolveUAVElementType(varHandle ir.GlobalVariableHandle) (ir.TypeInner, ir.ScalarType, bool) {
	if int(varHandle) >= len(e.ir.GlobalVariables) {
		return nil, ir.ScalarType{}, false
	}
	gv := &e.ir.GlobalVariables[varHandle]
	if int(gv.Type) >= len(e.ir.Types) {
		return nil, ir.ScalarType{}, false
	}

	inner := e.ir.Types[gv.Type].Inner

	// Unwrap struct wrapper if present (e.g., struct { data: array<u32> }).
	if st, ok := inner.(ir.StructType); ok {
		if len(st.Members) > 0 {
			memberType := e.ir.Types[st.Members[0].Type]
			inner = memberType.Inner
		}
	}

	// Now inner should be an ArrayType — get the element type.
	if arr, ok := inner.(ir.ArrayType); ok {
		elemInner := e.ir.Types[arr.Base].Inner
		scalar, found := scalarOfType(elemInner)
		if found {
			return elemInner, scalar, true
		}
		// For struct/array element types, find the deep scalar.
		if scalar, found := deepScalarOfType(e.ir, elemInner); found {
			return elemInner, scalar, true
		}
	}

	// Direct scalar or vector type.
	scalar, found := scalarOfType(inner)
	if found {
		return inner, scalar, true
	}
	// For struct/array types at the top level, find the deep scalar.
	if scalar, found := deepScalarOfType(e.ir, inner); found {
		return inner, scalar, true
	}
	return inner, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, false
}

// resolveUAVHandleID returns the resource handle ID for a UAV pointer chain.
// For binding array UAVs, this creates a dynamic dx.op.createHandle at point of use.
// For regular UAVs, this returns the pre-created handle from emitResourceHandles.
func (e *Emitter) resolveUAVHandleID(fn *ir.Function, chain *uavPointerChain) (int, error) {
	// Binding array: create dynamic handle.
	if chain.hasBindingArrayIndex {
		resIdx, found := e.resourceHandles[chain.varHandle]
		if !found {
			return 0, fmt.Errorf("UAV handle not found for global variable %d", chain.varHandle)
		}
		res := &e.resources[resIdx]
		if chain.isBindingArrayConstIdx {
			return e.emitDynamicCreateHandleConst(res, chain.bindingArrayConstIndex)
		}
		return e.emitDynamicCreateHandle(fn, res, chain.bindingArrayIndexExpr)
	}

	// Regular resource: use pre-created static handle.
	handleID, found := e.getResourceHandleID(chain.varHandle)
	if !found {
		return 0, fmt.Errorf("UAV handle not found for global variable %d", chain.varHandle)
	}
	return handleID, nil
}

// emitUAVLoad emits a dx.op.bufferLoad call for reading from a UAV (storage buffer).
//
// dx.op.bufferLoad signature:
//
//	%dx.types.ResRet.XX @dx.op.bufferLoad.XX(i32 68, %handle, i32 index, i32 offset)
//
// Returns the loaded value ID (first component for vectors).
//
// Reference: Mesa nir_to_dxil.c emit_bufferload_call() ~833
func (e *Emitter) emitUAVLoad(fn *ir.Function, chain *uavPointerChain) (int, error) {
	handleID, err := e.resolveUAVHandleID(fn, chain)
	if err != nil {
		return 0, err
	}

	indexID, err := e.resolveUAVIndex(fn, chain)
	if err != nil {
		return 0, err
	}

	// Mixed-scalar struct/array element types must be decomposed into per-field
	// bufferLoads. Otherwise a single bufferLoad.i32 returns 4 i32s and the
	// integer field would need different handling than float fields. DXC uses
	// separate bufferLoad per field with i32 overload + bitcast for float fields.
	if hasMixedScalarTypes(e.ir, chain.elemType) {
		return e.emitUAVLoadDecomposed(chain, handleID, indexID)
	}

	ol := overloadForScalar(chain.scalar)

	// Count all scalar components. For struct/array element types, use deep
	// component counting to load all fields.
	numComps := componentCount(chain.elemType)
	if chain.isWhole || numComps <= 1 {
		// cbvComponentCount recursively counts through structs and arrays.
		deepComps := cbvComponentCount(e.ir, chain.elemType)
		if deepComps > numComps {
			numComps = deepComps
		}
	}

	// dx.op.bufferLoad supports overloads f16/f32/i16/i32 only (mask 0x63
	// in DxilOperations.cpp:BufferLoad). For 64-bit element types
	// (i64/u64/f64) we MUST use dx.op.rawBufferLoad (mask 0xe7) which
	// covers i16/i32/i64/f16/f32/f64. The atomic-int64 / int64 storage
	// buffer test corpus depends on this routing.
	is64Bit := chain.scalar.Width == 8

	// For raw buffer loads (all naga storage buffers are ByteAddressBuffer /
	// RWByteAddressBuffer, kind 11), DXC always uses the i32 overload.
	// HLSL ByteAddressBuffer.Load() returns uint; asfloat() wraps the result.
	// This mirrors the store side which already uses i32+bitcast. After
	// extractvalue we bitcast i32 to float to recover the original type.
	needsBitcast := !is64Bit && chain.scalar.Kind == ir.ScalarFloat
	if needsBitcast {
		ol = overloadI32
	}

	resRetTy := e.getDxResRetType(ol)
	scalarTy := e.overloadReturnType(ol)
	undefVal := e.getUndefConstID()

	// The original scalar type for bitcasting extracted i32 values back to float.
	var targetScalarTy *module.Type
	if needsBitcast {
		targetScalarTy = e.mod.GetFloatType(32)
	}

	if numComps <= 4 && !is64Bit {
		// Single bufferLoad call (≤4 components fit one ResRet).
		bufLoadFn := e.getDxOpBufferLoadFunc(ol)
		opcodeVal := e.getIntConstID(int64(OpBufferLoad))
		retID := e.addCallInstr(bufLoadFn, resRetTy, []int{opcodeVal, handleID, indexID, undefVal})
		comps := make([]int, numComps)
		// Extract all components first (DXC groups extractvalues together).
		for i := 0; i < numComps; i++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, i},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps[i] = extractID
		}
		// Then bitcast i32 to float for raw buffer loads of float data.
		// DXC groups bitcasts after all extractvalues from the same load.
		if needsBitcast {
			for i := 0; i < numComps; i++ {
				comps[i] = e.addCastInstr(targetScalarTy, CastBitcast, comps[i])
			}
		}
		if numComps > 1 {
			e.pendingComponents = comps
		}
		return comps[0], nil
	}

	return e.emitUAVLoadRaw(chain, handleID, indexID, numComps, ol, resRetTy, scalarTy, undefVal, needsBitcast)
}

// emitUAVLoadRaw emits chunked dx.op.rawBufferLoad calls for elements with
// more than 4 scalar components or 64-bit scalar width — dx.op.bufferLoad
// caps at 4 components per call and rejects coord1 for non-structured
// buffers, so we switch to rawBufferLoad which covers all buffer kinds and
// supports the full i16/i32/i64/f16/f32/f64 overload set.
//
// When needsBitcast is true, extracted i32 values are bitcast to float to
// recover the original type (matching DXC's raw buffer load convention where
// ByteAddressBuffer.Load returns uint and asfloat wraps it).
//
// Mesa nir_to_dxil.c:810 emit_raw_bufferload_call is the reference. coord0
// carries the current byte offset (our chain index is already in scalar
// units), coord1 is undef, mask = (1<<count)-1, alignment = scalarWidth.
func (e *Emitter) emitUAVLoadRaw(
	chain *uavPointerChain,
	handleID, indexID, numComps int,
	ol overloadType, resRetTy, scalarTy *module.Type, undefVal int,
	needsBitcast bool,
) (int, error) {
	rawLoadFn := e.getDxOpRawBufferLoadFunc(ol)
	rawOpcodeVal := e.getIntConstID(int64(OpRawBufferLoad))

	scalarWidth := uint32(chain.scalar.Width)
	if scalarWidth == 0 {
		scalarWidth = 4
	}
	alignmentID := e.getIntConstID(int64(scalarWidth))

	var targetScalarTy *module.Type
	if needsBitcast {
		targetScalarTy = e.mod.GetFloatType(32)
	}

	comps := make([]int, 0, numComps)
	remaining := numComps
	byteOffset := uint32(0)

	for remaining > 0 {
		count := 4
		if remaining < 4 {
			count = remaining
		}
		coord0 := indexID
		if byteOffset > 0 {
			ofsID := e.getIntConstID(int64(byteOffset))
			coord0 = e.addBinOpInstr(e.mod.GetIntType(32), BinOpAdd, indexID, ofsID)
		}
		mask := int64((1 << count) - 1)
		maskID := e.getI8ConstID(mask)
		retID := e.addCallInstr(rawLoadFn, resRetTy,
			[]int{rawOpcodeVal, handleID, coord0, undefVal, maskID, alignmentID})
		// Extract all components from this chunk first.
		chunkStart := len(comps)
		for i := 0; i < count; i++ {
			extractID := e.allocValue()
			instr := &module.Instruction{
				Kind:       module.InstrExtractVal,
				HasValue:   true,
				ResultType: scalarTy,
				Operands:   []int{retID, i},
				ValueID:    extractID,
			}
			e.currentBB.AddInstruction(instr)
			comps = append(comps, extractID)
		}
		// Then bitcast extracted i32 values to float (DXC groups bitcasts
		// after all extractvalues from the same load).
		if needsBitcast {
			for i := chunkStart; i < len(comps); i++ {
				comps[i] = e.addCastInstr(targetScalarTy, CastBitcast, comps[i])
			}
		}
		remaining -= count
		byteOffset += uint32(count) * scalarWidth
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}
	return comps[0], nil
}

// flatScalarField describes a single scalar field within a flattened
// struct/array element layout — used by per-field UAV decomposition.
type flatScalarField struct {
	offsetWords uint32        // byte offset of this scalar from element base, divided by min(scalarWidth)
	byteOffset  uint32        // raw byte offset from element base
	scalar      ir.ScalarType // scalar type of this field (drives overload + bitcast)
}

// flattenScalarFields walks a TypeInner in source order and emits a flat list
// of (offset, scalar) entries. Vectors expand into N consecutive scalars,
// matrices into Cols*Rows, arrays into ElemCount * inner. Struct member
// offsets respect the IR-supplied member.Offset (which encodes WGSL alignment
// and padding).
//
// All offsets are in BYTES from the element base. The caller divides by
// scalarWidth when constructing bufferLoad raw byte indices.
func flattenScalarFields(irMod *ir.Module, inner ir.TypeInner, baseByte uint32) []flatScalarField {
	switch t := inner.(type) {
	case ir.ScalarType:
		return []flatScalarField{{byteOffset: baseByte, scalar: t}}
	case ir.VectorType:
		w := uint32(t.Scalar.Width)
		size := uint32(t.Size)
		out := make([]flatScalarField, 0, size)
		for i := uint32(0); i < size; i++ {
			out = append(out, flatScalarField{byteOffset: baseByte + i*w, scalar: t.Scalar})
		}
		return out
	case ir.MatrixType:
		// Matrix layout in storage: column-major, each column padded to vec4
		// alignment when the row count is < 4. We walk columns then rows so
		// the flat order matches our compose/extract conventions elsewhere.
		w := uint32(t.Scalar.Width)
		cols := uint32(t.Columns)
		rows := uint32(t.Rows)
		colBytes := rows * w
		colStride := (colBytes + 15) &^ 15
		if rows == 4 {
			colStride = colBytes
		}
		out := make([]flatScalarField, 0, cols*rows)
		for c := uint32(0); c < cols; c++ {
			for r := uint32(0); r < rows; r++ {
				out = append(out, flatScalarField{
					byteOffset: baseByte + c*colStride + r*w,
					scalar:     t.Scalar,
				})
			}
		}
		return out
	case ir.AtomicType:
		return []flatScalarField{{byteOffset: baseByte, scalar: t.Scalar}}
	case ir.ArrayType:
		if t.Size.Constant == nil || int(t.Base) >= len(irMod.Types) {
			return nil
		}
		count := *t.Size.Constant
		stride := t.Stride
		elemInner := irMod.Types[t.Base].Inner
		out := make([]flatScalarField, 0, int(count))
		for i := uint32(0); i < count; i++ {
			out = append(out, flattenScalarFields(irMod, elemInner, baseByte+i*stride)...)
		}
		return out
	case ir.StructType:
		out := make([]flatScalarField, 0, len(t.Members))
		for _, m := range t.Members {
			if int(m.Type) >= len(irMod.Types) {
				continue
			}
			out = append(out, flattenScalarFields(irMod, irMod.Types[m.Type].Inner, baseByte+m.Offset)...)
		}
		return out
	}
	return nil
}

// hasMixedScalarTypes returns true when an aggregate element type contains
// scalar leaves of more than one DXIL type signature (kind+width).
//
// dx.op.bufferLoad uses a SINGLE overload for all 4 ResRet slots. When the
// flat scalar list mixes f32 and i32 leaves, the validator rejects downstream
// store/use of the mismatched component with 0x80aa0009 — see BUG-DXIL-026
// Group A reproducer in gpucore/fine.wgsl (struct Segment {x0:f32, w:i32}).
//
// Pure scalar/vector/matrix and homogeneous structs/arrays continue to use
// the single-overload fast path in emitUAVLoad.
func hasMixedScalarTypes(irMod *ir.Module, inner ir.TypeInner) bool {
	fields := flattenScalarFields(irMod, inner, 0)
	if len(fields) < 2 {
		return false
	}
	first := fields[0].scalar
	for i := 1; i < len(fields); i++ {
		s := fields[i].scalar
		if s.Kind != first.Kind || s.Width != first.Width {
			// Treat ScalarSint and ScalarUint as the same DXIL type — both
			// map to LLVM iN. Only differing Kind across {bool, int, float}
			// or differing widths force decomposition.
			if scalarsShareDXILType(first, s) {
				continue
			}
			return true
		}
	}
	return false
}

// scalarsShareDXILType returns true when two ScalarTypes lower to the same
// LLVM type (and thus the same dx.op overload). Sint and Uint of equal
// width both lower to iN — they share .iN overload and need no per-field
// decomposition.
func scalarsShareDXILType(a, b ir.ScalarType) bool {
	if a.Width != b.Width {
		return false
	}
	switch a.Kind {
	case ir.ScalarSint, ir.ScalarUint:
		return b.Kind == ir.ScalarSint || b.Kind == ir.ScalarUint
	case ir.ScalarBool:
		return b.Kind == ir.ScalarBool
	case ir.ScalarFloat:
		return b.Kind == ir.ScalarFloat
	}
	return false
}

// emitUAVLoadDecomposed emits one dx.op.bufferLoad per scalar field of a
// mixed-type aggregate element. Per-field component IDs are tracked in
// pendingComponents so downstream extract/AccessIndex paths see the
// correctly-typed value.
//
// Each per-field load is a single-component bufferLoad with byte offset
// computed from the element base via integer add. For raw buffer loads,
// DXC always uses the i32 overload (ByteAddressBuffer.Load returns uint)
// and then bitcasts to float when needed. We match that convention here.
//
// Reference: Mesa nir_to_dxil.c emit_load_ssbo (~3427) emits per-intrinsic
// loads after NIR scalarization passes; we do the equivalent decomposition
// at emit time because our IR retains aggregate loads.
func (e *Emitter) emitUAVLoadDecomposed(chain *uavPointerChain, handleID, indexID int) (int, error) {
	fields := flattenScalarFields(e.ir, chain.elemType, 0)
	if len(fields) == 0 {
		return 0, fmt.Errorf("UAV load: cannot flatten elem type %T", chain.elemType)
	}

	// indexID from resolveUAVIndex is the BYTE coord0 base (BUG-DXIL-031).
	// Each field's byteOffset adds directly without scalar-unit conversion.
	undefVal := e.getUndefConstID()
	i32Ty := e.mod.GetIntType(32)
	f32Ty := e.mod.GetFloatType(32)
	opcodeVal := e.getIntConstID(int64(OpBufferLoad))

	comps := make([]int, len(fields))
	for i, f := range fields {
		ol := overloadForScalar(f.scalar)
		// For raw buffer loads of float fields, use i32 overload + bitcast.
		fieldNeedsBitcast := f.scalar.Kind == ir.ScalarFloat && f.scalar.Width != 8
		if fieldNeedsBitcast {
			ol = overloadI32
		}
		resRetTy := e.getDxResRetType(ol)
		scalarTy := e.overloadReturnType(ol)
		bufLoadFn := e.getDxOpBufferLoadFunc(ol)

		coord := indexID
		if f.byteOffset > 0 {
			ofsID := e.getIntConstID(int64(f.byteOffset))
			coord = e.addBinOpInstr(i32Ty, BinOpAdd, indexID, ofsID)
		}

		retID := e.addCallInstr(bufLoadFn, resRetTy,
			[]int{opcodeVal, handleID, coord, undefVal})

		extractID := e.allocValue()
		instr := &module.Instruction{
			Kind:       module.InstrExtractVal,
			HasValue:   true,
			ResultType: scalarTy,
			Operands:   []int{retID, 0},
			ValueID:    extractID,
		}
		e.currentBB.AddInstruction(instr)
		// Bitcast i32 to float for raw buffer loads of float fields.
		if fieldNeedsBitcast {
			extractID = e.addCastInstr(f32Ty, CastBitcast, extractID)
		}
		comps[i] = extractID
		_ = f.offsetWords // reserved for stride-based path
	}

	if len(comps) > 1 {
		e.pendingComponents = comps
	}
	return comps[0], nil
}

// emitUAVStore emits a dx.op.bufferStore call for writing to a UAV (storage buffer).
//
// dx.op.bufferStore signature:
//
//	void @dx.op.bufferStore.XX(i32 69, %handle, i32 index, i32 offset,
//	                           XX val0, XX val1, XX val2, XX val3, i8 mask)
//
// For RWByteAddressBuffer (raw buffer, kind 11) stores, DXC always uses the
// i32 overload regardless of the source data type. Float values are bitcast
// to i32 before being passed as arguments. This matches the HLSL semantics
// where RWByteAddressBuffer.Store takes uint values and asuint() converts
// floats. Verified against DXC golden output for all compute-store shaders.
//
// Reference: Mesa nir_to_dxil.c emit_bufferstore_call() ~877
// Reference: DXC HLOperationLower.cpp:4590 (TranslateStore uses i32Ty for raw buffers)
func (e *Emitter) emitUAVStore(fn *ir.Function, chain *uavPointerChain, valueHandle ir.ExpressionHandle) error {
	// Large aggregate stores (e.g., output = w_mem.arr for array<u32, 512>) are
	// decomposed into multiple batched bufferStore calls below (4 components each).

	handleID, err := e.resolveUAVHandleID(fn, chain)
	if err != nil {
		return err
	}

	indexID, err := e.resolveUAVIndex(fn, chain)
	if err != nil {
		return err
	}

	// Emit the value expression.
	valueID, err := e.emitExpression(fn, valueHandle)
	if err != nil {
		return fmt.Errorf("UAV store value: %w", err)
	}

	ol := overloadForScalar(chain.scalar)
	// dx.op.bufferStore overload mask 0x63 covers f16/f32/i16/i32 only;
	// 64-bit element types must use dx.op.rawBufferStore (mask 0xe7) just
	// like the load side does. atomicOps-int64.wgsl trips
	// 'DXIL intrinsic overload must be valid' on bufferStore.i64 without
	// this routing.
	is64Bit := chain.scalar.Width == 8

	// For raw buffer stores (all naga storage buffers are RWByteAddressBuffer,
	// kind 11), DXC always uses the i32 overload. Float values are bitcast to
	// i32 (matching HLSL's asuint pattern). Force the overload to i32 for
	// non-64-bit float types so our output matches DXC's convention.
	needsBitcast := !is64Bit && chain.scalar.Kind == ir.ScalarFloat
	if needsBitcast {
		ol = overloadI32
	}

	var bufStoreFn *module.Function
	if is64Bit {
		bufStoreFn = e.getDxOpRawBufferStoreFunc(ol)
	} else {
		bufStoreFn = e.getDxOpBufferStoreFunc(ol)
	}
	storeOpcode := int64(OpBufferStore)
	if is64Bit {
		storeOpcode = int64(OpRawBufferStore)
	}
	opcodeVal := e.getIntConstID(storeOpcode)
	undefI32 := e.getUndefConstID()
	// Value channel undefs must match the overload type. Since we use i32
	// overload for raw buffer float stores, i32 undef is correct for both
	// integer and bitcast-float paths.
	valUndefID := e.getTypedUndefConstID(e.overloadReturnType(ol))

	// Determine total number of scalar components.
	//
	// For struct/array UAV element types (e.g. `pout: array<Particle>`
	// where Particle = {vec2<f32>, vec2<f32>}), componentCount returns 1 because
	// its default branch treats non-scalar/vector/matrix types as a single component.
	// cbvComponentCount recursively flattens and returns the true scalar count (4
	// for Particle), fixing the store to emit all components.
	numComps := cbvComponentCount(e.ir, chain.elemType)

	//nolint:nestif // dispatch over single-call vs multi-batch buffer store
	if numComps <= 4 {
		// Single bufferStore call.
		vals := [4]int{valUndefID, valUndefID, valUndefID, valUndefID}
		for i := 0; i < numComps && i < 4; i++ {
			if numComps == 1 {
				vals[i] = valueID
			} else {
				vals[i] = e.getComponentID(valueHandle, i)
			}
			// Bitcast float values to i32 for raw buffer stores.
			if needsBitcast {
				vals[i] = e.addCastInstr(e.mod.GetIntType(32), CastBitcast, vals[i])
			}
		}
		writeMask := (1 << numComps) - 1
		maskVal := e.getI8ConstID(int64(writeMask))
		args := []int{
			opcodeVal, handleID, indexID, undefI32,
			vals[0], vals[1], vals[2], vals[3], maskVal,
		}
		if is64Bit {
			// rawBufferStore takes an additional alignment operand.
			scalarWidth := uint32(chain.scalar.Width)
			if scalarWidth == 0 {
				scalarWidth = 8
			}
			args = append(args, e.getIntConstID(int64(scalarWidth)))
		}
		e.addCallInstr(bufStoreFn, e.mod.GetVoidType(), args)
		return nil
	}

	return e.emitUAVStoreBatched(chain, valueHandle, bufStoreFn, opcodeVal,
		handleID, indexID, undefI32, valUndefID, numComps, needsBitcast)
}

// emitUAVStoreBatched emits N bufferStore calls for element types with >4
// scalar components. coord0 advances by `count * scalarWidth` BYTES per batch
// per DXIL spec (RWRawBuffer coord0 in bytes). Mirrors DXC HLOperationLower.cpp
// :4722 `NewCoord = EltSize * MaxStoreElemCount * j`.
//
// When needsBitcast is true, float values are bitcast to i32 before storing
// (matching DXC's raw buffer store convention).
func (e *Emitter) emitUAVStoreBatched(
	chain *uavPointerChain, valueHandle ir.ExpressionHandle,
	bufStoreFn *module.Function, opcodeVal, handleID, indexID, undefI32, valUndefID, numComps int,
	needsBitcast bool,
) error {
	i32Ty := e.mod.GetIntType(32)
	scalarW := uint32(chain.scalar.Width)
	if scalarW == 0 {
		scalarW = 4
	}
	compIdx := 0
	batchIdx := 0
	remaining := numComps

	for remaining > 0 {
		count := 4
		if remaining < 4 {
			count = remaining
		}
		batchIndexID := indexID
		if batchIdx > 0 {
			batchByteOff := uint32(batchIdx) * 4 * scalarW
			ofsID := e.getIntConstID(int64(batchByteOff))
			batchIndexID = e.addBinOpInstr(i32Ty, BinOpAdd, indexID, ofsID)
		}
		vals := [4]int{valUndefID, valUndefID, valUndefID, valUndefID}
		for i := 0; i < count; i++ {
			vals[i] = e.getComponentID(valueHandle, compIdx+i)
			if needsBitcast {
				vals[i] = e.addCastInstr(i32Ty, CastBitcast, vals[i])
			}
		}
		writeMask := (1 << count) - 1
		maskVal := e.getI8ConstID(int64(writeMask))
		e.addCallInstr(bufStoreFn, e.mod.GetVoidType(), []int{
			opcodeVal, handleID, batchIndexID, undefI32,
			vals[0], vals[1], vals[2], vals[3], maskVal,
		})
		compIdx += count
		remaining -= count
		batchIdx++
	}
	return nil
}

// getDxOpBufferLoadFunc creates the dx.op.bufferLoad.XX function declaration.
// Signature: %dx.types.ResRet.XX @dx.op.bufferLoad.XX(i32, %handle, i32, i32)
//
// Reference: Mesa nir_to_dxil.c emit_bufferload_call() ~833
func (e *Emitter) getDxOpBufferLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.bufferLoad"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	resRetTy := e.getDxResRetType(ol)
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(resRetTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpRawBufferStoreFunc creates the dx.op.rawBufferStore.XX function
// declaration. SM 6.2+ replacement for bufferStore. Signature mirrors DXC
// DxilOperations.cpp:RawBufferStore + Mesa emit_raw_bufferstore_call:
//
//	void @dx.op.rawBufferStore.XX(
//	    i32 opcode,
//	    %dx.types.Handle handle,
//	    i32 coord0,
//	    i32 coord1,            // structured-buffer offset, or i32 undef
//	    XX v0, XX v1, XX v2, XX v3,
//	    i8  componentMask,
//	    i32 alignment)
//
// Overload mask 0xe7 = {f16,f32,f64,i16,i32,i64}. dx.op.bufferStore.i64
// is rejected by the validator with 'DXIL intrinsic overload must be
// valid' (BufferStore mask 0x63 = {f16,f32,i16,i32}); rawBufferStore
// is the only path for 64-bit element types.
func (e *Emitter) getDxOpRawBufferStoreFunc(ol overloadType) *module.Function {
	name := "dx.op.rawBufferStore"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}
	voidTy := e.mod.GetVoidType()
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valTy := e.overloadReturnType(ol)
	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, valTy, valTy, valTy, valTy, i8Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpRawBufferLoadFunc creates the dx.op.rawBufferLoad.XX function
// declaration. Signature mirrors DXC DxilOperations.cpp + Mesa
// emit_raw_bufferload_call (nir_to_dxil.c:810):
//
//	%dx.types.ResRet.XX @dx.op.rawBufferLoad.XX(
//	    i32 opcode,
//	    %dx.types.Handle handle,
//	    i32 coord0,           // byte offset (raw) or element index (structured)
//	    i32 coord1,           // structured-buffer element-offset, or i32 undef
//	    i8  componentMask,    // (1 << count) - 1
//	    i32 alignment)
//
// Available since SM 6.2. Overload mask 0xe7 = {f16,f32,f64,i16,i32,i64}.
// Use this for >4-component loads from typed/raw buffers (where
// dx.op.bufferLoad cannot represent the layout) and for native i64/f64
// access. The validator rejects i8/i1 overloads.
func (e *Emitter) getDxOpRawBufferLoadFunc(ol overloadType) *module.Function {
	name := "dx.op.rawBufferLoad"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	resRetTy := e.getDxResRetType(ol)
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, i8Ty, i32Ty}
	funcTy := e.mod.GetFunctionType(resRetTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// getDxOpBufferStoreFunc creates the dx.op.bufferStore.XX function declaration.
// Signature: void @dx.op.bufferStore.XX(i32, %handle, i32, i32, XX, XX, XX, XX, i8)
//
// Reference: Mesa nir_to_dxil.c emit_bufferstore_call() ~877
func (e *Emitter) getDxOpBufferStoreFunc(ol overloadType) *module.Function {
	name := "dx.op.bufferStore"
	key := dxOpKey{name: name, overload: ol}
	if fn, ok := e.dxOpFuncs[key]; ok {
		return fn
	}

	voidTy := e.mod.GetVoidType()
	handleTy := e.getDxHandleType()
	i32Ty := e.mod.GetIntType(32)
	i8Ty := e.mod.GetIntType(8)
	valTy := e.overloadReturnType(ol)

	fullName := name + overloadSuffix(ol)
	params := []*module.Type{i32Ty, handleTy, i32Ty, i32Ty, valTy, valTy, valTy, valTy, i8Ty}
	funcTy := e.mod.GetFunctionType(voidTy, params)
	fn := e.mod.AddFunction(fullName, funcTy, true)
	fn.AttrSetID = classifyDxOpAttr(fullName)
	e.dxOpFuncs[key] = fn
	return fn
}

// resourceKind returns the DXIL resource kind integer for metadata.
// Reference: D3D12_SRV_DIMENSION / DXIL resource kinds.
func (e *Emitter) resourceKind(res *resourceInfo) int {
	if res.kindOverride != 0 {
		return res.kindOverride
	}
	switch res.class {
	case resourceClassCBV:
		return 13 // CBuffer
	case resourceClassSampler:
		return 0 // Sampler (no dimension)
	case resourceClassSRV:
		// SRV: image (texture), storage buffer, or ray-tracing acceleration
		// structure. The DXIL enum, per
		// reference/dxil/dxc/include/dxc/DXIL/DxilConstants.h:433
		//   1=Texture1D, 2=Texture2D, 3=Texture2DMS, 4=Texture3D,
		//   10=TypedBuffer, 11=RawBuffer, 12=StructuredBuffer,
		//   16=RTAccelerationStructure.
		// The PSV0 side already picks RTAccelerationStructure for AS
		// bindings via classifyPSVResource in dxil.go. The bitcode side
		// must agree, otherwise the validator rejects with
		// 'Container mismatch for ResourceBindInfo between PSV0 part
		// (ResKind: RTAccelerationStructure) and DXIL module (ResKind:
		// RawBuffer)'.
		if int(res.typeHandle) < len(e.ir.Types) {
			inner := unwrapBindingArray(e.ir, e.ir.Types[res.typeHandle].Inner)
			switch t := inner.(type) {
			case ir.ImageType:
				return e.imageResourceKind(t)
			case ir.AccelerationStructureType:
				return 16 // RTAccelerationStructure
			}
		}
		return 11 // RawBuffer for read-only storage buffers
	case resourceClassUAV:
		// UAV: storage image, storage buffer, or atomic counter buffer.
		// Same RawBuffer (11) default as SRV — kept in sync so PSV0 and
		// bitcode metadata agree on resource kind.
		if int(res.typeHandle) < len(e.ir.Types) {
			inner := unwrapBindingArray(e.ir, e.ir.Types[res.typeHandle].Inner)
			if img, ok := inner.(ir.ImageType); ok {
				return e.imageResourceKind(img)
			}
		}
		return 11 // RawBuffer for read-write storage buffers
	default:
		return 0
	}
}

// unwrapBindingArray peels a single BindingArrayType layer if present.
// Used by resourceKind to look through binding_array<T, N> to the base
// resource type for shape classification.
func unwrapBindingArray(mod *ir.Module, inner ir.TypeInner) ir.TypeInner {
	if ba, ok := inner.(ir.BindingArrayType); ok {
		if int(ba.Base) < len(mod.Types) {
			return mod.Types[ba.Base].Inner
		}
	}
	return inner
}

// imageResourceKind returns the DXIL resource kind for an image type.
// imageResourceKind returns the DXIL ResourceKind enum value for an image.
// Values match reference/dxil/dxc/include/dxc/DXIL/DxilConstants.h
// `enum class ResourceKind`:
//
//	1=Texture1D, 2=Texture2D, 3=Texture2DMS, 4=Texture3D,
//	5=TextureCube, 6=Texture1DArray, 7=Texture2DArray,
//	8=Texture2DMSArray, 9=TextureCubeArray.
//
// BUG-DXIL-022 follow-up: the prior table used completely wrong
// numeric values (Dim2D→4 / Texture3D, Dim3D→7 / Texture2DArray, etc.)
// which made every image-resource shader fail
// 'ResourceBindInfo mismatch' between PSV0 (correct via
// container.PSVResKind*) and bitcode metadata.
func (e *Emitter) imageResourceKind(img ir.ImageType) int {
	switch img.Dim {
	case ir.Dim1D:
		if img.Arrayed {
			return 6 // Texture1DArray
		}
		return 1 // Texture1D
	case ir.Dim2D:
		if img.Multisampled {
			if img.Arrayed {
				return 8 // Texture2DMSArray
			}
			return 3 // Texture2DMS
		}
		if img.Arrayed {
			return 7 // Texture2DArray
		}
		return 2 // Texture2D
	case ir.Dim3D:
		return 4 // Texture3D
	case ir.DimCube:
		if img.Arrayed {
			return 9 // TextureCubeArray
		}
		return 5 // TextureCube
	default:
		return 2 // default Texture2D
	}
}
