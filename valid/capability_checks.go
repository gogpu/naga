package valid

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// This file holds the capability checks of statements, expressions and
// entry points: the Capabilities:: sites of Rust naga's valid/function.rs,
// valid/expression.rs and valid/interface.rs. The type and global variable
// checks are beside the other type and global checks in validate.go.

// missingCapability is Rust naga's FunctionError::MissingCapability.
func missingCapability(c Capabilities) string {
	return "Shader requires capability " + c.rustDebug()
}

// unsupportedSubgroupOperation is Rust naga's FunctionError::InvalidSubgroup
// of SubgroupError::UnsupportedOperation.
func unsupportedSubgroupOperation(ops SubgroupOperationSet) string {
	return "Subgroup operation is invalid: Support for subgroup operation " + ops.rustDebug() + " is required"
}

// expressionStages are the stages an expression can run in: the stages
// Rust's validate_expression returns (valid/expression.rs). Sampling with an
// implicit level of detail and derivatives need the fragment stage's
// neighboring invocations; a workgroup uniform load is a workgroup barrier.
func (v *validator) expressionStages(kind ir.ExpressionKind) ShaderStages {
	switch k := kind.(type) {
	case ir.ExprSubgroupBallotResult, ir.ExprSubgroupOperationResult:
		return v.subgroupStages
	case ir.ExprCallResult:
		return v.calleeStages(k.Function)
	case ir.ExprImageSample:
		switch k.Level.(type) {
		case ir.SampleLevelAuto, ir.SampleLevelBias:
			return ShaderStagesFragment
		}
	case ir.ExprDerivative:
		return ShaderStagesFragment
	case ir.ExprWorkGroupUniformLoadResult:
		return ShaderStagesComputeLike
	}
	return allStages
}

// checkBarrier checks a control or memory barrier: a subgroup barrier needs
// SUBGROUP and SUBGROUP_BARRIER and the BASIC operations (Rust's
// validate_block_impl, Statement::ControlBarrier | MemoryBarrier).
func (v *validator) checkBarrier(index int, flags ir.BarrierFlags) {
	if flags&ir.BarrierSubGroup == 0 {
		return
	}
	if need := CapSubgroup | CapSubgroupBarrier; !v.has(need) {
		v.addErrorInStatement(index, missingCapability(need))
		return
	}
	if v.subgroupOperations&SubgroupBasic == 0 {
		v.addErrorInStatement(index, unsupportedSubgroupOperation(SubgroupBasic))
	}
}

// validateSubgroupStatement checks a subgroup ballot, collective operation
// or gather: the stages it narrows the function to, SUBGROUP, the operation
// class it needs, its operands' scope and its result (Rust's
// validate_block_impl, Statement::Subgroup*).
func (v *validator) validateSubgroupStatement(index int, kind ir.StatementKind) {
	v.context.stages &= v.subgroupStages
	v.context.subgroupStages &= v.subgroupStages
	var (
		need     SubgroupOperationSet
		operands []ir.ExpressionHandle
		result   ir.ExpressionHandle
	)
	switch k := kind.(type) {
	case ir.StmtSubgroupBallot:
		need, result = SubgroupBallot, k.Result
		if k.Predicate != nil {
			operands = append(operands, *k.Predicate)
		}
	case ir.StmtSubgroupCollectiveOperation:
		need, result = SubgroupArithmetic, k.Result
		if k.Op == ir.SubgroupOperationAll || k.Op == ir.SubgroupOperationAny {
			need = SubgroupVote
		}
		operands = append(operands, k.Argument)
	case ir.StmtSubgroupGather:
		var lane *ir.ExpressionHandle
		need, lane = gatherOperation(k.Mode)
		if lane != nil {
			operands = append(operands, *lane)
		}
		operands = append(operands, k.Argument)
		result = k.Result
	default:
		return
	}
	switch {
	case !v.has(CapSubgroup):
		v.addErrorInStatement(index, missingCapability(CapSubgroup))
	case v.subgroupOperations&need != need:
		v.addErrorInStatement(index, unsupportedSubgroupOperation(need))
	}
	for _, h := range operands {
		v.useExpression(h)
	}
	v.introduce(result)
}

// gatherOperation returns the operation class a gather mode needs (Rust's
// GatherMode::required_operations) and its index operand, if it has one.
func gatherOperation(mode ir.GatherMode) (SubgroupOperationSet, *ir.ExpressionHandle) {
	switch m := mode.(type) {
	case ir.GatherBroadcast:
		return SubgroupBallot, &m.Index
	case ir.GatherShuffle:
		return SubgroupShuffle, &m.Index
	case ir.GatherShuffleXor:
		return SubgroupShuffle, &m.Mask
	case ir.GatherShuffleUp:
		return SubgroupShuffleRelative, &m.Delta
	case ir.GatherShuffleDown:
		return SubgroupShuffleRelative, &m.Delta
	case ir.GatherQuadBroadcast:
		return SubgroupQuadFragmentCompute, &m.Index
	case ir.GatherQuadSwap:
		return SubgroupQuadFragmentCompute, nil
	}
	// GatherBroadcastFirst.
	return SubgroupBallot, nil
}

// checkAtomic checks the capabilities of an atomic operation on a 64-bit
// integer or an f32 (Rust's validate_atomic): SHADER_INT64_ATOMIC_ALL_OPS
// allows every 64-bit operation, SHADER_INT64_ATOMIC_MIN_MAX only min and
// max in storage without a result; SHADER_FLOAT32_ATOMIC allows add,
// subtract and exchange on an f32.
func (v *validator) checkAtomic(index int, st ir.StmtAtomic) {
	ptr, scalar, ok := v.atomicPointer(st.Pointer)
	if !ok {
		return
	}
	invalid := func(msg string) {
		v.addErrorInStatement(index, "Atomic operation is invalid: "+msg)
	}
	switch {
	case (scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint) && scalar.Width == 8:
		if v.capabilities.Contains(CapShaderInt64AtomicAllOps) {
			return
		}
		need := int64AtomicNeed(ptr.Space, st.Fun, st.Result != nil)
		if !v.has(need) {
			invalid("Capability " + need.rustDebug() + " is required")
		}
	case scalar.Kind == ir.ScalarFloat && scalar.Width == 4:
		if !v.has(CapShaderFloat32Atomic) {
			invalid("Capability " + CapShaderFloat32Atomic.rustDebug() + " is required")
			return
		}
		switch f := st.Fun.(type) {
		case ir.AtomicAdd, ir.AtomicSubtract:
		case ir.AtomicExchange:
			if f.Compare != nil {
				invalid("Operator " + atomicFunctionDebug(f) + " is not supported")
			}
		default:
			invalid("Operator " + atomicFunctionDebug(f) + " is not supported")
		}
	}
}

// atomicPointer returns the pointer type of an atomic operation's pointer
// and the scalar of the atomic it points to; false when the operand is not
// a typed pointer to an atomic (other checks report that).
func (v *validator) atomicPointer(h ir.ExpressionHandle) (ir.PointerType, ir.ScalarType, bool) {
	types := v.context.types
	if types == nil || int(h) >= types.Len() {
		return ir.PointerType{}, ir.ScalarType{}, false
	}
	res := types.Get(h)
	if res.Handle != nil && !v.isValidTypeHandle(*res.Handle) {
		return ir.PointerType{}, ir.ScalarType{}, false
	}
	ptr, ok := res.Inner(v.module.Types).(ir.PointerType)
	if !ok || !v.isValidTypeHandle(ptr.Base) {
		return ir.PointerType{}, ir.ScalarType{}, false
	}
	at, ok := v.module.Types[ptr.Base].Inner.(ir.AtomicType)
	if !ok {
		return ir.PointerType{}, ir.ScalarType{}, false
	}
	return ptr, at.Scalar, true
}

// atomicFunctionDebug is an atomic function as Rust's Debug prints it.
func atomicFunctionDebug(f ir.AtomicFunction) string {
	switch f := f.(type) {
	case ir.AtomicAdd:
		return "Add"
	case ir.AtomicSubtract:
		return "Subtract"
	case ir.AtomicAnd:
		return "And"
	case ir.AtomicExclusiveOr:
		return "ExclusiveOr"
	case ir.AtomicInclusiveOr:
		return "InclusiveOr"
	case ir.AtomicMin:
		return "Min"
	case ir.AtomicMax:
		return "Max"
	case ir.AtomicExchange:
		if f.Compare != nil {
			return fmt.Sprintf("Exchange { compare: Some([%d]) }", *f.Compare)
		}
		return "Exchange { compare: None }"
	}
	return fmt.Sprintf("%T", f)
}

// checkImageAtomic checks the capability of an atomic on a storage texture
// (Rust's validate_block_impl, Statement::ImageAtomic): TEXTURE_ATOMIC for
// r32sint and r32uint, TEXTURE_INT64_ATOMIC for r64uint. The image is a
// global variable, or an element of a binding array one.
func (v *validator) checkImageAtomic(index int, st ir.StmtImageAtomic) {
	fn := v.context.function
	if fn == nil || int(st.Image) >= len(fn.Expressions) {
		return
	}
	image := fn.Expressions[st.Image].Kind
	switch k := image.(type) {
	case ir.ExprAccess:
		if int(k.Base) < len(fn.Expressions) {
			image = fn.Expressions[k.Base].Kind
		}
	case ir.ExprAccessIndex:
		if int(k.Base) < len(fn.Expressions) {
			image = fn.Expressions[k.Base].Kind
		}
	}
	gv, ok := image.(ir.ExprGlobalVariable)
	if !ok || !v.isValidGlobalVariableHandle(gv.Variable) {
		return
	}
	ty := v.module.GlobalVariables[gv.Variable].Type
	if !v.isValidTypeHandle(ty) {
		return
	}
	if ba, ok := v.module.Types[ty].Inner.(ir.BindingArrayType); ok && v.isValidTypeHandle(ba.Base) {
		ty = ba.Base
	}
	img, ok := v.module.Types[ty].Inner.(ir.ImageType)
	if !ok || img.Class != ir.ImageClassStorage {
		return
	}
	var need Capabilities
	switch img.StorageFormat {
	case ir.StorageFormatR64Uint:
		need = CapTextureInt64Atomic
	case ir.StorageFormatR32Sint, ir.StorageFormatR32Uint:
		need = CapTextureAtomic
	default:
		return
	}
	if !v.has(need) {
		v.addErrorInStatement(index, missingCapability(need))
	}
}

// stageBit is a shader stage as a ShaderStages bit.
func stageBit(stage ir.ShaderStage) ShaderStages {
	switch stage {
	case ir.StageVertex:
		return ShaderStagesVertex
	case ir.StageFragment:
		return ShaderStagesFragment
	case ir.StageCompute:
		return ShaderStagesCompute
	case ir.StageMesh:
		return ShaderStagesMesh
	case ir.StageTask:
		return ShaderStagesTask
	}
	return 0
}

// entryPointError reports an error of an entry point as Rust naga's
// ValidationError::EntryPoint renders it.
func (v *validator) entryPointError(ep *ir.EntryPoint, msg string) {
	stage := ep.Stage.String()
	v.addError(fmt.Sprintf("Entry point %s at %s is invalid: %s", ep.Name, strings.ToUpper(stage[:1])+stage[1:], msg))
}

// checkEntryPointCapabilities checks what an entry point needs as a whole
// (Rust's validate_entry_point): MESH_SHADER for a task or mesh shader,
// EARLY_DEPTH_TEST for an early depth test, MESH_SHADER_POINT_TOPOLOGY for
// a mesh shader that outputs points.
func (v *validator) checkEntryPointCapabilities(ep *ir.EntryPoint) {
	unsupported := func(c Capabilities) {
		v.entryPointError(ep, "Capability "+c.rustDebug()+" is not supported")
	}
	if (ep.Stage == ir.StageTask || ep.Stage == ir.StageMesh) && !v.has(CapMeshShader) {
		unsupported(CapMeshShader)
		return
	}
	if ep.EarlyDepthTest != nil && !v.has(CapEarlyDepthTest) {
		unsupported(CapEarlyDepthTest)
	}
	if ep.MeshInfo != nil && ep.MeshInfo.Topology == ir.MeshTopologyPoints &&
		!v.has(CapMeshShaderPointTopology) {
		unsupported(CapMeshShaderPointTopology)
	}
}

// builtinCapability is the capability a built-in needs (Rust's
// VaryingContext::validate_impl); 0 when it needs none the validator
// checks. sample_index needs MULTISAMPLED_SHADING in Rust, a core feature
// some devices lack, which is not a Capabilities bit here.
func builtinCapability(b ir.BuiltinValue) Capabilities {
	switch b {
	case ir.BuiltinClipDistances:
		return CapClipDistances
	case ir.BuiltinCullDistance:
		return CapCullDistances
	case ir.BuiltinPrimitiveIndex:
		return CapPrimitiveIndex
	case ir.BuiltinBarycentric, ir.BuiltinBarycentricNoPerspective:
		return CapShaderBarycentrics
	case ir.BuiltinViewIndex:
		return CapMultiview
	case ir.BuiltinNumSubgroups, ir.BuiltinSubgroupID, ir.BuiltinSubgroupSize, ir.BuiltinSubgroupInvocationID:
		return CapSubgroup
	case ir.BuiltinDrawIndex:
		return CapDrawIndex
	case ir.BuiltinCullPrimitive, ir.BuiltinPointIndex, ir.BuiltinLineIndices, ir.BuiltinTriangleIndices,
		ir.BuiltinMeshTaskSize, ir.BuiltinVertexCount, ir.BuiltinPrimitiveCount, ir.BuiltinVertices,
		ir.BuiltinPrimitives:
		return CapMeshShader
	}
	return 0
}

// narrowSubgroupStages narrows the function's subgroup stages by an
// expression: the result of a subgroup operation runs in the subgroup
// stages, the result of a call in the callee's.
func (v *validator) narrowSubgroupStages(kind ir.ExpressionKind) {
	switch k := kind.(type) {
	case ir.ExprSubgroupBallotResult, ir.ExprSubgroupOperationResult:
		v.context.subgroupStages &= v.subgroupStages
	case ir.ExprCallResult:
		v.narrowSubgroupStagesByCallee(k.Function)
	}
}

// narrowSubgroupStagesByCallee narrows the function's subgroup stages by
// those of a function it calls.
func (v *validator) narrowSubgroupStagesByCallee(fn ir.FunctionHandle) {
	if int(fn) < v.validatedFunctions {
		v.context.subgroupStages &= v.functionSubgroupStages[fn]
	}
}

// int64AtomicNeed is the capability a 64-bit integer atomic needs when the
// set lacks ALL_OPS: MIN_MAX for an atomicMin or atomicMax on a storage
// buffer whose result is unused, ALL_OPS otherwise (Rust's
// validate_atomic_operation).
func int64AtomicNeed(space ir.AddressSpace, fun ir.AtomicFunction, hasResult bool) Capabilities {
	_, isMin := fun.(ir.AtomicMin)
	_, isMax := fun.(ir.AtomicMax)
	if (isMin || isMax) && space == ir.SpaceStorage && !hasResult {
		return CapShaderInt64AtomicMinMax
	}
	return CapShaderInt64AtomicAllOps
}

// atomicNeeds asks for the capability of an atomic operation on a scalar:
// what checkAtomic checks, without its messages.
func (v *validator) atomicNeeds(space ir.AddressSpace, scalar ir.ScalarType, fun ir.AtomicFunction, hasResult bool) {
	switch {
	case (scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint) && scalar.Width == 8:
		if !v.capabilities.Contains(CapShaderInt64AtomicAllOps) {
			v.has(int64AtomicNeed(space, fun, hasResult))
		}
	case scalar.Kind == ir.ScalarFloat && scalar.Width == 4:
		v.has(CapShaderFloat32Atomic)
	}
}
