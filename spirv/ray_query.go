package spirv

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// rayQueryFuncKind identifies the type of ray query helper function.
type rayQueryFuncKind int

const (
	rqFuncInitialize rayQueryFuncKind = iota
	rqFuncProceed
	rqFuncGetIntersectionCommitted
	rqFuncGetIntersectionCandidate
	rqFuncGenerateIntersection
	rqFuncConfirmIntersection
)

// rayQueryTrackerIDs holds the SPIR-V variable IDs for ray query initialization tracking.
type rayQueryTrackerIDs struct {
	initializedTracker uint32 // u32 variable tracking init/proceed/finished state
	tMaxTracker        uint32 // f32 variable tracking the original t_max value
}

// RayQueryPoint bitflags matching Rust naga's RayQueryPoint.
const (
	rqPointInitialized       uint32 = 1 << 0
	rqPointProceed           uint32 = 1 << 1
	rqPointFinishedTraversal uint32 = 1 << 2
)

// emitRayQueryTrackerVars creates the two tracker variables for a ray_query local variable.
// Returns the tracker IDs. Variables are added to FunctionBuilder.Variables.
func (b *Backend) emitRayQueryTrackerVars(fb *FunctionBuilder) rayQueryTrackerIDs {
	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	u32PtrTypeID := b.emitPointerType(StorageClassFunction, u32TypeID)
	f32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	f32PtrTypeID := b.emitPointerType(StorageClassFunction, f32TypeID)

	// initialized_tracker: u32, init to 0 (RayQueryPoint::empty)
	initTrackerID := b.builder.AllocID()
	initVal := b.builder.AddConstant(u32TypeID, 0)
	fb.Variables = append(fb.Variables, Instruction{
		Opcode: OpVariable,
		Words:  []uint32{u32PtrTypeID, initTrackerID, uint32(StorageClassFunction), initVal},
	})

	// t_max_tracker: f32, init to 0.0
	tMaxTrackerID := b.builder.AllocID()
	tMaxInitVal := b.builder.AddConstantFloat32(f32TypeID, 0.0)
	fb.Variables = append(fb.Variables, Instruction{
		Opcode: OpVariable,
		Words:  []uint32{f32PtrTypeID, tMaxTrackerID, uint32(StorageClassFunction), tMaxInitVal},
	})

	return rayQueryTrackerIDs{
		initializedTracker: initTrackerID,
		tMaxTracker:        tMaxTrackerID,
	}
}

// getRayQueryPointerTypeID returns the OpTypePointer(Function, OpTypeRayQueryKHR) type ID.
func (b *Backend) getRayQueryPointerTypeID() uint32 {
	// Find or create the ray query type
	var rqTypeID uint32
	for i, t := range b.module.Types {
		if _, ok := t.Inner.(ir.RayQueryType); ok {
			var err error
			rqTypeID, err = b.emitType(ir.TypeHandle(i))
			if err != nil {
				// Should not happen for RayQueryType
				panic(fmt.Sprintf("failed to emit RayQueryType: %v", err))
			}
			break
		}
	}
	if rqTypeID == 0 {
		// Fallback: emit the type directly
		rqTypeID = b.builder.AllocID()
		ib := b.newIB()
		ib.AddWord(rqTypeID)
		b.builder.types = append(b.builder.types, ib.Build(OpTypeRayQueryKHR))
	}
	return b.emitPointerType(StorageClassFunction, rqTypeID)
}

// writeRayQueryInitialize generates the ray_query_initialize helper function.
// Matches Rust naga's write_ray_query_initialize.
func (b *Backend) writeRayQueryInitialize() uint32 {
	if id, ok := b.rayQueryFuncIDs[rqFuncInitialize]; ok {
		return id
	}

	rqPtrTypeID := b.getRayQueryPointerTypeID()

	// Find acceleration structure type ID
	var accelTypeID uint32
	for i, t := range b.module.Types {
		if _, ok := t.Inner.(ir.AccelerationStructureType); ok {
			var err error
			accelTypeID, err = b.emitType(ir.TypeHandle(i))
			if err != nil {
				panic(fmt.Sprintf("failed to emit AccelerationStructureType: %v", err))
			}
			break
		}
	}

	// Find RayDesc type ID by searching for it in the type arena.
	// RayDesc is a struct with 6 members: flags(u32), cull_mask(u32), t_min(f32), t_max(f32), origin(vec3f32), dir(vec3f32)
	var rayDescTypeID uint32
	for i, t := range b.module.Types {
		if t.Name == "RayDesc" {
			var err error
			rayDescTypeID, err = b.emitType(ir.TypeHandle(i))
			if err != nil {
				panic(fmt.Sprintf("failed to emit RayDesc type: %v", err))
			}
			break
		}
	}

	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	u32PtrTypeID := b.emitPointerType(StorageClassFunction, u32TypeID)
	f32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	f32PtrTypeID := b.emitPointerType(StorageClassFunction, f32TypeID)
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	boolVec3TypeID := b.emitVectorType(boolTypeID, 3)
	vec3f32TypeID := b.emitVectorType(f32TypeID, 3)

	voidTypeID := b.getVoidType()

	// Function signature: void func(ray_query_ptr, accel_struct, ray_desc, u32_ptr, f32_ptr)
	funcTypeID := b.getFuncType(voidTypeID, []uint32{rqPtrTypeID, accelTypeID, rayDescTypeID, u32PtrTypeID, f32PtrTypeID})

	var fb FunctionBuilder
	funcID := b.builder.AllocID()
	fb.Signature = Instruction{
		Opcode: OpFunction,
		Words:  []uint32{voidTypeID, funcID, uint32(FunctionControlNone), funcTypeID},
	}

	// Parameters
	queryID := b.builder.AllocID()
	accelID := b.builder.AllocID()
	descID := b.builder.AllocID()
	initTrackerID := b.builder.AllocID()
	tMaxTrackerID := b.builder.AllocID()
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{rqPtrTypeID, queryID}},
		{Opcode: OpFunctionParameter, Words: []uint32{accelTypeID, accelID}},
		{Opcode: OpFunctionParameter, Words: []uint32{rayDescTypeID, descID}},
		{Opcode: OpFunctionParameter, Words: []uint32{u32PtrTypeID, initTrackerID}},
		{Opcode: OpFunctionParameter, Words: []uint32{f32PtrTypeID, tMaxTrackerID}},
	}

	// Entry block
	labelID := b.builder.AllocID()
	block := NewBlock(labelID)

	// Extract RayDesc fields
	rayFlagsID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{u32TypeID, rayFlagsID, descID, 0}})
	cullMaskID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{u32TypeID, cullMaskID, descID, 1}})
	tMinID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{f32TypeID, tMinID, descID, 2}})
	tMaxID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{f32TypeID, tMaxID, descID, 3}})
	// Store tmax to tracker
	block.Push(Instruction{Opcode: OpStore, Words: []uint32{tMaxTrackerID, tMaxID}})
	rayOriginID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{vec3f32TypeID, rayOriginID, descID, 4}})
	rayDirID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpCompositeExtract, Words: []uint32{vec3f32TypeID, rayDirID, descID, 5}})

	// Validation checks (only when init tracking is enabled)
	var validID uint32
	if b.options.RayQueryInitTracking {
		zeroF32ID := b.builder.AddConstantFloat32(f32TypeID, 0.0)
		zeroU32ID := b.builder.AddConstant(u32TypeID, 0)

		// tmin <= tmax
		tMinLeTMaxID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpFOrdLessThanEqual, Words: []uint32{boolTypeID, tMinLeTMaxID, tMinID, tMaxID}})

		// tmin >= 0
		tMinGeZeroID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpFOrdGreaterThanEqual, Words: []uint32{boolTypeID, tMinGeZeroID, tMinID, zeroF32ID}})

		// Check ray origin finite
		originInfID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpIsInf, Words: []uint32{boolVec3TypeID, originInfID, rayOriginID}})
		anyOriginInfID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpAny, Words: []uint32{boolTypeID, anyOriginInfID, originInfID}})
		originNanID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpIsNan, Words: []uint32{boolVec3TypeID, originNanID, rayOriginID}})
		anyOriginNanID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpAny, Words: []uint32{boolTypeID, anyOriginNanID, originNanID}})
		originNotFiniteID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalOr, Words: []uint32{boolTypeID, originNotFiniteID, anyOriginNanID, anyOriginInfID}})
		allOriginFiniteID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalNot, Words: []uint32{boolTypeID, allOriginFiniteID, originNotFiniteID}})

		// Check ray dir finite
		dirInfID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpIsInf, Words: []uint32{boolVec3TypeID, dirInfID, rayDirID}})
		anyDirInfID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpAny, Words: []uint32{boolTypeID, anyDirInfID, dirInfID}})
		dirNanID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpIsNan, Words: []uint32{boolVec3TypeID, dirNanID, rayDirID}})
		anyDirNanID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpAny, Words: []uint32{boolTypeID, anyDirNanID, dirNanID}})
		dirNotFiniteID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalOr, Words: []uint32{boolTypeID, dirNotFiniteID, anyDirNanID, anyDirInfID}})
		allDirFiniteID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalNot, Words: []uint32{boolTypeID, allDirFiniteID, dirNotFiniteID}})

		// Check flag combinations with writeLessThan2True pattern
		containsSkipTriangles := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 256) // SKIP_TRIANGLES = 0x100
		containsSkipAABBs := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 512)     // SKIP_AABBS = 0x200
		notSkipTriAABB := writeLessThan2True(b, &block, []uint32{containsSkipTriangles, containsSkipAABBs})

		containsCullBack := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 16)  // CULL_BACK_FACING = 0x10
		containsCullFront := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 32) // CULL_FRONT_FACING = 0x20
		notSkipTriCull := writeLessThan2True(b, &block, []uint32{containsSkipTriangles, containsCullBack, containsCullFront})

		containsOpaque := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 1)         // FORCE_OPAQUE = 0x01
		containsNoOpaque := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 2)       // FORCE_NO_OPAQUE = 0x02
		containsCullOpaque := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 64)    // CULL_OPAQUE = 0x40
		containsCullNoOpaque := writeRayFlagsContainsFlag(b, &block, rayFlagsID, 128) // CULL_NO_OPAQUE = 0x80
		notMultipleOpaque := writeLessThan2True(b, &block, []uint32{containsOpaque, containsNoOpaque, containsCullOpaque, containsCullNoOpaque})

		// Combine all checks: reduce_and in reverse order (matching Rust)
		_ = zeroU32ID
		checks := []uint32{
			tMinLeTMaxID,
			tMinGeZeroID,
			allOriginFiniteID,
			allDirFiniteID,
			notSkipTriAABB,
			notSkipTriCull,
			notMultipleOpaque,
		}
		validID = writeReduceAnd(b, &block, checks)
	}

	mergeLabelID := b.builder.AllocID()
	mergeBlock := NewBlock(mergeLabelID)

	invalidLabelID := b.builder.AllocID()
	invalidBlock := NewBlock(invalidLabelID)

	validLabelID := b.builder.AllocID()
	validBlock := NewBlock(validLabelID)

	if b.options.RayQueryInitTracking {
		block.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{mergeLabelID, 0}})
		fb.Consume(block, Instruction{Opcode: OpBranchConditional, Words: []uint32{validID, validLabelID, invalidLabelID}})
	} else {
		fb.Consume(block, Instruction{Opcode: OpBranch, Words: []uint32{validLabelID}})
	}

	// Valid block: perform the actual initialization
	validBlock.Push(Instruction{
		Opcode: OpRayQueryInitializeKHR,
		Words:  []uint32{queryID, accelID, rayFlagsID, cullMaskID, rayOriginID, tMinID, rayDirID, tMaxID},
	})
	constInitialized := b.builder.AddConstant(u32TypeID, rqPointInitialized)
	validBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{initTrackerID, constInitialized}})
	fb.Consume(validBlock, Instruction{Opcode: OpBranch, Words: []uint32{mergeLabelID}})

	if b.options.RayQueryInitTracking {
		fb.Consume(invalidBlock, Instruction{Opcode: OpBranch, Words: []uint32{mergeLabelID}})
	}

	fb.Consume(mergeBlock, Instruction{Opcode: OpReturn})

	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	b.rayQueryFuncIDs[rqFuncInitialize] = funcID
	return funcID
}

// writeRayQueryProceed generates the ray_query_proceed helper function.
func (b *Backend) writeRayQueryProceed() uint32 {
	if id, ok := b.rayQueryFuncIDs[rqFuncProceed]; ok {
		return id
	}

	rqPtrTypeID := b.getRayQueryPointerTypeID()
	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	u32PtrTypeID := b.emitPointerType(StorageClassFunction, u32TypeID)
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	boolPtrTypeID := b.emitPointerType(StorageClassFunction, boolTypeID)

	funcTypeID := b.getFuncType(boolTypeID, []uint32{rqPtrTypeID, u32PtrTypeID})

	var fb FunctionBuilder
	funcID := b.builder.AllocID()
	fb.Signature = Instruction{
		Opcode: OpFunction,
		Words:  []uint32{boolTypeID, funcID, uint32(FunctionControlNone), funcTypeID},
	}

	queryID := b.builder.AllocID()
	initTrackerID := b.builder.AllocID()
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{rqPtrTypeID, queryID}},
		{Opcode: OpFunctionParameter, Words: []uint32{u32PtrTypeID, initTrackerID}},
	}

	blockID := b.builder.AllocID()
	block := NewBlock(blockID)

	// var proceeded: bool = false
	constFalse := b.builder.AllocID()
	b.builder.types = append(b.builder.types, Instruction{
		Opcode: OpConstantFalse,
		Words:  []uint32{boolTypeID, constFalse},
	})
	proceededID := b.builder.AllocID()
	fb.Variables = append(fb.Variables, Instruction{
		Opcode: OpVariable,
		Words:  []uint32{boolPtrTypeID, proceededID, uint32(StorageClassFunction), constFalse},
	})

	// Load initialized tracker
	loadedTrackerID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpLoad, Words: []uint32{u32TypeID, loadedTrackerID, initTrackerID}})

	mergeID := b.builder.AllocID()
	mergeBlock := NewBlock(mergeID)

	validBlockID := b.builder.AllocID()
	validBlock := NewBlock(validBlockID)

	if b.options.RayQueryInitTracking {
		isInitialized := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointInitialized)
		block.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{mergeID, 0}})
		fb.Consume(block, Instruction{Opcode: OpBranchConditional, Words: []uint32{isInitialized, validBlockID, mergeID}})
	} else {
		fb.Consume(block, Instruction{Opcode: OpBranch, Words: []uint32{validBlockID}})
	}

	// Valid block: call OpRayQueryProceedKHR
	hasProceededID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpRayQueryProceedKHR, Words: []uint32{boolTypeID, hasProceededID, queryID}})
	validBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{proceededID, hasProceededID}})

	// Update tracker: add PROCEED flag, and FINISHED_TRAVERSAL if proceed returned false
	addFlagFinished := b.builder.AddConstant(u32TypeID, rqPointProceed|rqPointFinishedTraversal)
	addFlagContinuing := b.builder.AddConstant(u32TypeID, rqPointProceed)

	addFlagsID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpSelect, Words: []uint32{u32TypeID, addFlagsID, hasProceededID, addFlagContinuing, addFlagFinished}})
	finalFlagsID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpBitwiseOr, Words: []uint32{u32TypeID, finalFlagsID, loadedTrackerID, addFlagsID}})
	validBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{initTrackerID, finalFlagsID}})

	fb.Consume(validBlock, Instruction{Opcode: OpBranch, Words: []uint32{mergeID}})

	// Merge: load and return proceeded value
	loadedProceededID := b.builder.AllocID()
	mergeBlock.Push(Instruction{Opcode: OpLoad, Words: []uint32{boolTypeID, loadedProceededID, proceededID}})
	fb.Consume(mergeBlock, Instruction{Opcode: OpReturnValue, Words: []uint32{loadedProceededID}})

	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	b.rayQueryFuncIDs[rqFuncProceed] = funcID
	return funcID
}

// writeRayQueryGetIntersection generates the committed or candidate get_intersection helper.
func (b *Backend) writeRayQueryGetIntersection(committed bool) uint32 {
	kind := rqFuncGetIntersectionCommitted
	if !committed {
		kind = rqFuncGetIntersectionCandidate
	}
	if id, ok := b.rayQueryFuncIDs[kind]; ok {
		return id
	}

	// Find RayIntersection type
	var riTypeID uint32
	if b.module.SpecialTypes.RayIntersection != nil {
		var err error
		riTypeID, err = b.emitType(*b.module.SpecialTypes.RayIntersection)
		if err != nil {
			panic(fmt.Sprintf("failed to emit RayIntersection type: %v", err))
		}
	}

	riPtrTypeID := b.emitPointerType(StorageClassFunction, riTypeID)

	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	u32PtrTypeID := b.emitPointerType(StorageClassFunction, u32TypeID)
	f32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	f32PtrTypeID := b.emitPointerType(StorageClassFunction, f32TypeID)
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	boolPtrTypeID := b.emitPointerType(StorageClassFunction, boolTypeID)
	vec2f32TypeID := b.emitVectorType(f32TypeID, 2)
	vec2f32PtrTypeID := b.emitPointerType(StorageClassFunction, vec2f32TypeID)
	vec3f32TypeID := b.emitVectorType(f32TypeID, 3)
	mat4x3TypeID := b.emitMatrixType(vec3f32TypeID, 4)
	mat4x3PtrTypeID := b.emitPointerType(StorageClassFunction, mat4x3TypeID)

	rqPtrTypeID := b.getRayQueryPointerTypeID()

	funcTypeID := b.getFuncType(riTypeID, []uint32{rqPtrTypeID, u32PtrTypeID})

	var fb FunctionBuilder
	funcID := b.builder.AllocID()
	fb.Signature = Instruction{
		Opcode: OpFunction,
		Words:  []uint32{riTypeID, funcID, uint32(FunctionControlNone), funcTypeID},
	}

	queryID := b.builder.AllocID()
	initTrackerID := b.builder.AllocID()
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{rqPtrTypeID, queryID}},
		{Opcode: OpFunctionParameter, Words: []uint32{u32PtrTypeID, initTrackerID}},
	}

	labelID := b.builder.AllocID()
	block := NewBlock(labelID)

	// Create null-initialized intersection variable
	blankIntersection := b.builder.AddConstantNull(riTypeID)
	blankIntersectionID := b.builder.AllocID()
	fb.Variables = append(fb.Variables, Instruction{
		Opcode: OpVariable,
		Words:  []uint32{riPtrTypeID, blankIntersectionID, uint32(StorageClassFunction), blankIntersection},
	})

	// Intersection index: committed=1, candidate=0
	var intersectionIdx uint32
	if committed {
		intersectionIdx = 1 // RayQueryCommittedIntersectionKHR
	}
	intersectionID := b.builder.AddConstant(u32TypeID, intersectionIdx)

	// Load tracker and check validity
	loadedTrackerID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpLoad, Words: []uint32{u32TypeID, loadedTrackerID, initTrackerID}})
	proceededID := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointProceed)
	finishedID := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointFinishedTraversal)

	var isValidID uint32
	if committed {
		isValidID = writeLogicalAnd(b, &block, finishedID, proceededID)
	} else {
		notFinishedID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalNot, Words: []uint32{boolTypeID, notFinishedID, finishedID}})
		isValidID = writeLogicalAnd(b, &block, notFinishedID, proceededID)
	}

	validLabelID := b.builder.AllocID()
	validBlock := NewBlock(validLabelID)
	finalLabelID := b.builder.AllocID()
	finalBlock := NewBlock(finalLabelID)

	block.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{finalLabelID, 0}})
	fb.Consume(block, Instruction{Opcode: OpBranchConditional, Words: []uint32{isValidID, validLabelID, finalLabelID}})

	// Valid block: get intersection type
	rawKindID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTypeKHR, Words: []uint32{u32TypeID, rawKindID, queryID, intersectionID}})

	var kindID uint32
	if committed {
		kindID = rawKindID
	} else {
		// Remap candidate kind to IR kind
		candidateTriID := b.builder.AddConstant(u32TypeID, 0) // CandidateTriangle = 0
		conditionID := b.builder.AllocID()
		validBlock.Push(Instruction{Opcode: OpIEqual, Words: []uint32{boolTypeID, conditionID, rawKindID, candidateTriID}})
		kindID = b.builder.AllocID()
		triKindID := b.builder.AddConstant(u32TypeID, 1)  // Triangle = 1
		aabbKindID := b.builder.AddConstant(u32TypeID, 3) // Aabb = 3
		validBlock.Push(Instruction{Opcode: OpSelect, Words: []uint32{u32TypeID, kindID, conditionID, triKindID, aabbKindID}})
	}

	// Store kind to intersection[0]
	idx0 := b.builder.AddConstant(u32TypeID, 0)
	accessKindID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{u32PtrTypeID, accessKindID, blankIntersectionID, idx0}})
	validBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{accessKindID, kindID}})

	// Check if not none
	noneID := b.builder.AddConstant(u32TypeID, 0) // None = 0
	notNoneID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpINotEqual, Words: []uint32{boolTypeID, notNoneID, kindID, noneID}})

	notNoneLabelID := b.builder.AllocID()
	notNoneBlock := NewBlock(notNoneLabelID)
	outerMergeLabelID := b.builder.AllocID()
	outerMergeBlock := NewBlock(outerMergeLabelID)

	validBlock.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{outerMergeLabelID, 0}})
	fb.Consume(validBlock, Instruction{Opcode: OpBranchConditional, Words: []uint32{notNoneID, notNoneLabelID, outerMergeLabelID}})

	// notNone block: get all intersection properties
	instanceCustomIndexID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionInstanceCustomIndexKHR, Words: []uint32{u32TypeID, instanceCustomIndexID, queryID, intersectionID}})
	instanceIdID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionInstanceIdKHR, Words: []uint32{u32TypeID, instanceIdID, queryID, intersectionID}})
	sbtRecordOffsetID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionInstanceShaderBindingTableRecordOffsetKHR, Words: []uint32{u32TypeID, sbtRecordOffsetID, queryID, intersectionID}})
	geometryIndexID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionGeometryIndexKHR, Words: []uint32{u32TypeID, geometryIndexID, queryID, intersectionID}})
	primitiveIndexID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionPrimitiveIndexKHR, Words: []uint32{u32TypeID, primitiveIndexID, queryID, intersectionID}})
	objectToWorldID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionObjectToWorldKHR, Words: []uint32{mat4x3TypeID, objectToWorldID, queryID, intersectionID}})
	worldToObjectID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionWorldToObjectKHR, Words: []uint32{mat4x3TypeID, worldToObjectID, queryID, intersectionID}})

	// Store to struct fields
	idx2 := b.builder.AddConstant(u32TypeID, 2)
	idx3 := b.builder.AddConstant(u32TypeID, 3)
	idx4 := b.builder.AddConstant(u32TypeID, 4)
	idx5 := b.builder.AddConstant(u32TypeID, 5)
	idx6 := b.builder.AddConstant(u32TypeID, 6)
	idx9 := b.builder.AddConstant(u32TypeID, 9)
	idx10 := b.builder.AddConstant(u32TypeID, 10)

	storeU32Field := func(block *Block, fieldIdx, valueID uint32) {
		accessID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{u32PtrTypeID, accessID, blankIntersectionID, fieldIdx}})
		block.Push(Instruction{Opcode: OpStore, Words: []uint32{accessID, valueID}})
	}
	storeMatField := func(block *Block, fieldIdx, valueID uint32) {
		accessID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{mat4x3PtrTypeID, accessID, blankIntersectionID, fieldIdx}})
		block.Push(Instruction{Opcode: OpStore, Words: []uint32{accessID, valueID}})
	}

	storeU32Field(&notNoneBlock, idx2, instanceCustomIndexID)
	storeU32Field(&notNoneBlock, idx3, instanceIdID)
	storeU32Field(&notNoneBlock, idx4, sbtRecordOffsetID)
	storeU32Field(&notNoneBlock, idx5, geometryIndexID)
	storeU32Field(&notNoneBlock, idx6, primitiveIndexID)
	storeMatField(&notNoneBlock, idx9, objectToWorldID)
	storeMatField(&notNoneBlock, idx10, worldToObjectID)

	// Check if triangle
	triID := b.builder.AddConstant(u32TypeID, 1) // Triangle = 1
	triCompID := b.builder.AllocID()
	notNoneBlock.Push(Instruction{Opcode: OpIEqual, Words: []uint32{boolTypeID, triCompID, kindID, triID}})

	// For committed: t goes in notNone block. For candidate: t goes in tri block.
	triLabelID := b.builder.AllocID()
	triBlock := NewBlock(triLabelID)
	innerMergeLabelID := b.builder.AllocID()
	innerMergeBlock := NewBlock(innerMergeLabelID)

	if committed {
		// Store t in notNone block
		tID := b.builder.AllocID()
		notNoneBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTKHR, Words: []uint32{f32TypeID, tID, queryID, intersectionID}})
		idx1 := b.builder.AddConstant(u32TypeID, 1)
		accessTID := b.builder.AllocID()
		notNoneBlock.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{f32PtrTypeID, accessTID, blankIntersectionID, idx1}})
		notNoneBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{accessTID, tID}})
	}

	notNoneBlock.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{innerMergeLabelID, 0}})
	fb.Consume(notNoneBlock, Instruction{Opcode: OpBranchConditional, Words: []uint32{notNoneID, triLabelID, innerMergeLabelID}})

	if !committed {
		// Store t in tri block for candidate
		tID := b.builder.AllocID()
		triBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTKHR, Words: []uint32{f32TypeID, tID, queryID, intersectionID}})
		idx1 := b.builder.AddConstant(u32TypeID, 1)
		accessTID := b.builder.AllocID()
		triBlock.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{f32PtrTypeID, accessTID, blankIntersectionID, idx1}})
		triBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{accessTID, tID}})
	}

	// Tri block: barycentrics and front_face
	barycentricsID := b.builder.AllocID()
	triBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionBarycentricsKHR, Words: []uint32{vec2f32TypeID, barycentricsID, queryID, intersectionID}})
	frontFaceID := b.builder.AllocID()
	triBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionFrontFaceKHR, Words: []uint32{boolTypeID, frontFaceID, queryID, intersectionID}})

	idx7 := b.builder.AddConstant(u32TypeID, 7)
	idx8 := b.builder.AddConstant(u32TypeID, 8)
	accessBarycentricsID := b.builder.AllocID()
	triBlock.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{vec2f32PtrTypeID, accessBarycentricsID, blankIntersectionID, idx7}})
	triBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{accessBarycentricsID, barycentricsID}})
	accessFrontFaceID := b.builder.AllocID()
	triBlock.Push(Instruction{Opcode: OpAccessChain, Words: []uint32{boolPtrTypeID, accessFrontFaceID, blankIntersectionID, idx8}})
	triBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{accessFrontFaceID, frontFaceID}})

	fb.Consume(triBlock, Instruction{Opcode: OpBranch, Words: []uint32{innerMergeLabelID}})
	fb.Consume(innerMergeBlock, Instruction{Opcode: OpBranch, Words: []uint32{outerMergeLabelID}})
	fb.Consume(outerMergeBlock, Instruction{Opcode: OpBranch, Words: []uint32{finalLabelID}})

	// Final block: load and return
	loadedIntersectionID := b.builder.AllocID()
	finalBlock.Push(Instruction{Opcode: OpLoad, Words: []uint32{riTypeID, loadedIntersectionID, blankIntersectionID}})
	fb.Consume(finalBlock, Instruction{Opcode: OpReturnValue, Words: []uint32{loadedIntersectionID}})

	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	b.rayQueryFuncIDs[kind] = funcID

	_ = boolPtrTypeID
	return funcID
}

// writeRayQueryGenerateIntersection generates the generate_intersection helper.
func (b *Backend) writeRayQueryGenerateIntersection() uint32 {
	if id, ok := b.rayQueryFuncIDs[rqFuncGenerateIntersection]; ok {
		return id
	}

	rqPtrTypeID := b.getRayQueryPointerTypeID()
	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	u32PtrTypeID := b.emitPointerType(StorageClassFunction, u32TypeID)
	f32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	f32PtrTypeID := b.emitPointerType(StorageClassFunction, f32TypeID)
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	voidTypeID := b.getVoidType()

	funcTypeID := b.getFuncType(voidTypeID, []uint32{rqPtrTypeID, u32PtrTypeID, f32TypeID, f32PtrTypeID})

	var fb FunctionBuilder
	funcID := b.builder.AllocID()
	fb.Signature = Instruction{
		Opcode: OpFunction,
		Words:  []uint32{voidTypeID, funcID, uint32(FunctionControlNone), funcTypeID},
	}

	queryID := b.builder.AllocID()
	initTrackerID := b.builder.AllocID()
	depthID := b.builder.AllocID()
	tMaxTrackerID := b.builder.AllocID()
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{rqPtrTypeID, queryID}},
		{Opcode: OpFunctionParameter, Words: []uint32{u32PtrTypeID, initTrackerID}},
		{Opcode: OpFunctionParameter, Words: []uint32{f32TypeID, depthID}},
		{Opcode: OpFunctionParameter, Words: []uint32{f32PtrTypeID, tMaxTrackerID}},
	}

	blockID := b.builder.AllocID()
	block := NewBlock(blockID)

	// Two local f32 variables
	currentT1 := b.builder.AllocID()
	fb.Variables = append(fb.Variables, Instruction{
		Opcode: OpVariable,
		Words:  []uint32{f32PtrTypeID, currentT1, uint32(StorageClassFunction)},
	})
	currentT2 := b.builder.AllocID()
	fb.Variables = append(fb.Variables, Instruction{
		Opcode: OpVariable,
		Words:  []uint32{f32PtrTypeID, currentT2, uint32(StorageClassFunction)},
	})
	currentT := currentT2 // Use the second one as the actual currentT (matching Rust's shadowing)

	validLabelID := b.builder.AllocID()
	validBlock := NewBlock(validLabelID)
	finalLabelID := b.builder.AllocID()
	finalBlock := NewBlock(finalLabelID)

	if b.options.RayQueryInitTracking {
		loadedTrackerID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLoad, Words: []uint32{u32TypeID, loadedTrackerID, initTrackerID}})
		proceededID := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointProceed)
		finishedID := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointFinishedTraversal)
		notFinishedID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalNot, Words: []uint32{boolTypeID, notFinishedID, finishedID}})
		isValidID := writeLogicalAnd(b, &block, notFinishedID, proceededID)
		block.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{finalLabelID, 0}})
		fb.Consume(block, Instruction{Opcode: OpBranchConditional, Words: []uint32{isValidID, validLabelID, finalLabelID}})
	} else {
		fb.Consume(block, Instruction{Opcode: OpBranch, Words: []uint32{validLabelID}})
	}

	// Valid block: check candidate type is AABB
	candidateIntersectionID := b.builder.AddConstant(u32TypeID, 0) // CandidateIntersection
	committedIntersectionID := b.builder.AddConstant(u32TypeID, 1) // CommittedIntersection

	rawKindID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTypeKHR, Words: []uint32{u32TypeID, rawKindID, queryID, candidateIntersectionID}})

	candidateAABBID := b.builder.AddConstant(u32TypeID, 1) // CandidateAABB = 1
	isAABBID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpIEqual, Words: []uint32{boolTypeID, isAABBID, rawKindID, candidateAABBID}})

	// Get tmin
	tMinID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpRayQueryGetRayTMinKHR, Words: []uint32{f32TypeID, tMinID, queryID}})

	// Get committed intersection type
	committedTypeID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTypeKHR, Words: []uint32{u32TypeID, committedTypeID, queryID, committedIntersectionID}})

	committedNoneID := b.builder.AddConstant(u32TypeID, 0) // CommittedNone = 0
	noCommittedID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpIEqual, Words: []uint32{boolTypeID, noCommittedID, committedTypeID, committedNoneID}})

	// Branch: if no committed, use t_max_tracker; else use committed t
	nextValidBlockID := b.builder.AllocID()
	noCommittedBlockID := b.builder.AllocID()
	noCommittedBlock := NewBlock(noCommittedBlockID)
	committedBlockID := b.builder.AllocID()
	committedBlock := NewBlock(committedBlockID)

	validBlock.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{nextValidBlockID, 0}})
	fb.Consume(validBlock, Instruction{Opcode: OpBranchConditional, Words: []uint32{noCommittedID, noCommittedBlockID, committedBlockID}})

	// No committed: load t_max
	tMaxLoadID := b.builder.AllocID()
	noCommittedBlock.Push(Instruction{Opcode: OpLoad, Words: []uint32{f32TypeID, tMaxLoadID, tMaxTrackerID}})
	noCommittedBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{currentT, tMaxLoadID}})
	fb.Consume(noCommittedBlock, Instruction{Opcode: OpBranch, Words: []uint32{nextValidBlockID}})

	// Has committed: get committed t
	latestTID := b.builder.AllocID()
	committedBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTKHR, Words: []uint32{f32TypeID, latestTID, queryID, candidateIntersectionID}})
	committedBlock.Push(Instruction{Opcode: OpStore, Words: []uint32{currentT, latestTID}})
	fb.Consume(committedBlock, Instruction{Opcode: OpBranch, Words: []uint32{nextValidBlockID}})

	// Next valid block: check depth in range
	nextValidBlock := NewBlock(nextValidBlockID)
	tGeTMinID := b.builder.AllocID()
	nextValidBlock.Push(Instruction{Opcode: OpFOrdGreaterThanEqual, Words: []uint32{boolTypeID, tGeTMinID, depthID, tMinID}})
	tCurrentLoadID := b.builder.AllocID()
	nextValidBlock.Push(Instruction{Opcode: OpLoad, Words: []uint32{f32TypeID, tCurrentLoadID, currentT}})
	tLeTCurrentID := b.builder.AllocID()
	nextValidBlock.Push(Instruction{Opcode: OpFOrdLessThanEqual, Words: []uint32{boolTypeID, tLeTCurrentID, depthID, tCurrentLoadID}})
	tInRangeID := writeLogicalAnd(b, &nextValidBlock, tGeTMinID, tLeTCurrentID)
	callValidID := writeLogicalAnd(b, &nextValidBlock, tInRangeID, isAABBID)

	generateLabelID := b.builder.AllocID()
	generateBlock := NewBlock(generateLabelID)
	genMergeLabelID := b.builder.AllocID()
	genMergeBlock := NewBlock(genMergeLabelID)

	nextValidBlock.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{genMergeLabelID, 0}})
	fb.Consume(nextValidBlock, Instruction{Opcode: OpBranchConditional, Words: []uint32{callValidID, generateLabelID, genMergeLabelID}})

	generateBlock.Push(Instruction{Opcode: OpRayQueryGenerateIntersectionKHR, Words: []uint32{queryID, depthID}})
	fb.Consume(generateBlock, Instruction{Opcode: OpBranch, Words: []uint32{genMergeLabelID}})
	fb.Consume(genMergeBlock, Instruction{Opcode: OpBranch, Words: []uint32{finalLabelID}})

	fb.Consume(finalBlock, Instruction{Opcode: OpReturn})

	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	b.rayQueryFuncIDs[rqFuncGenerateIntersection] = funcID
	return funcID
}

// writeRayQueryConfirmIntersection generates the confirm_intersection helper.
func (b *Backend) writeRayQueryConfirmIntersection() uint32 {
	if id, ok := b.rayQueryFuncIDs[rqFuncConfirmIntersection]; ok {
		return id
	}

	rqPtrTypeID := b.getRayQueryPointerTypeID()
	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	u32PtrTypeID := b.emitPointerType(StorageClassFunction, u32TypeID)
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	voidTypeID := b.getVoidType()

	funcTypeID := b.getFuncType(voidTypeID, []uint32{rqPtrTypeID, u32PtrTypeID})

	var fb FunctionBuilder
	funcID := b.builder.AllocID()
	fb.Signature = Instruction{
		Opcode: OpFunction,
		Words:  []uint32{voidTypeID, funcID, uint32(FunctionControlNone), funcTypeID},
	}

	queryID := b.builder.AllocID()
	initTrackerID := b.builder.AllocID()
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{rqPtrTypeID, queryID}},
		{Opcode: OpFunctionParameter, Words: []uint32{u32PtrTypeID, initTrackerID}},
	}

	blockID := b.builder.AllocID()
	block := NewBlock(blockID)

	validLabelID := b.builder.AllocID()
	validBlock := NewBlock(validLabelID)
	finalLabelID := b.builder.AllocID()
	finalBlock := NewBlock(finalLabelID)

	if b.options.RayQueryInitTracking {
		loadedTrackerID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLoad, Words: []uint32{u32TypeID, loadedTrackerID, initTrackerID}})
		proceededID := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointProceed)
		finishedID := writeRayFlagsContainsFlag(b, &block, loadedTrackerID, rqPointFinishedTraversal)
		notFinishedID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalNot, Words: []uint32{boolTypeID, notFinishedID, finishedID}})
		isValidID := writeLogicalAnd(b, &block, notFinishedID, proceededID)
		block.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{finalLabelID, 0}})
		fb.Consume(block, Instruction{Opcode: OpBranchConditional, Words: []uint32{isValidID, validLabelID, finalLabelID}})
	} else {
		fb.Consume(block, Instruction{Opcode: OpBranch, Words: []uint32{validLabelID}})
	}

	// Check candidate is triangle
	candidateIntersectionID := b.builder.AddConstant(u32TypeID, 0) // CandidateIntersection
	rawKindID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpRayQueryGetIntersectionTypeKHR, Words: []uint32{u32TypeID, rawKindID, queryID, candidateIntersectionID}})

	candidateTriID := b.builder.AddConstant(u32TypeID, 0) // CandidateTriangle = 0
	isTriID := b.builder.AllocID()
	validBlock.Push(Instruction{Opcode: OpIEqual, Words: []uint32{boolTypeID, isTriID, rawKindID, candidateTriID}})

	confirmLabelID := b.builder.AllocID()
	confirmBlock := NewBlock(confirmLabelID)
	confirmMergeLabelID := b.builder.AllocID()
	confirmMergeBlock := NewBlock(confirmMergeLabelID)

	validBlock.Push(Instruction{Opcode: OpSelectionMerge, Words: []uint32{confirmMergeLabelID, 0}})
	fb.Consume(validBlock, Instruction{Opcode: OpBranchConditional, Words: []uint32{isTriID, confirmLabelID, confirmMergeLabelID}})

	confirmBlock.Push(Instruction{Opcode: OpRayQueryConfirmIntersectionKHR, Words: []uint32{queryID}})
	fb.Consume(confirmBlock, Instruction{Opcode: OpBranch, Words: []uint32{confirmMergeLabelID}})
	fb.Consume(confirmMergeBlock, Instruction{Opcode: OpBranch, Words: []uint32{finalLabelID}})

	fb.Consume(finalBlock, Instruction{Opcode: OpReturn})

	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)
	b.rayQueryFuncIDs[rqFuncConfirmIntersection] = funcID
	return funcID
}

// writeRayFlagsContainsFlag checks if a u32 value has a particular bit set.
func writeRayFlagsContainsFlag(b *Backend, block *Block, id uint32, flag uint32) uint32 {
	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	bitID := b.builder.AddConstant(u32TypeID, flag)
	zeroID := b.builder.AddConstant(u32TypeID, 0)

	andID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpBitwiseAnd, Words: []uint32{u32TypeID, andID, id, bitID}})
	eqID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpINotEqual, Words: []uint32{boolTypeID, eqID, andID, zeroID}})
	return eqID
}

// writeLogicalAnd emits a logical AND of two booleans.
func writeLogicalAnd(b *Backend, block *Block, one, two uint32) uint32 {
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	id := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpLogicalAnd, Words: []uint32{boolTypeID, id, one, two}})
	return id
}

// writeReduceAnd reduces a list of booleans via logical AND (from last to first).
func writeReduceAnd(b *Backend, block *Block, bools []uint32) uint32 {
	current := bools[len(bools)-1]
	for i := len(bools) - 2; i >= 0; i-- {
		current = writeLogicalAnd(b, block, current, bools[i])
	}
	return current
}

// writeLessThan2True checks that fewer than 2 of the given booleans are true.
func writeLessThan2True(b *Backend, block *Block, bools []uint32) uint32 {
	boolTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})

	var eachTwoTrue []uint32
	// For each pair of booleans, check if both are true
	for i := len(bools) - 1; i >= 0; i-- {
		for j := 0; j < i; j++ {
			bothTrueID := writeLogicalAnd(b, block, bools[i], bools[j])
			eachTwoTrue = append(eachTwoTrue, bothTrueID)
		}
	}

	// OR all the pairs together
	allOrID := eachTwoTrue[len(eachTwoTrue)-1]
	for i := len(eachTwoTrue) - 2; i >= 0; i-- {
		newAllOrID := b.builder.AllocID()
		block.Push(Instruction{Opcode: OpLogicalOr, Words: []uint32{boolTypeID, newAllOrID, allOrID, eachTwoTrue[i]}})
		allOrID = newAllOrID
	}

	// NOT to get "less than 2"
	lessThan2ID := b.builder.AllocID()
	block.Push(Instruction{Opcode: OpLogicalNot, Words: []uint32{boolTypeID, lessThan2ID, allOrID}})
	return lessThan2ID
}
