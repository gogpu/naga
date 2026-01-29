package spirv

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/ir"
)

// Backend translates IR to SPIR-V.
type Backend struct {
	module  *ir.Module
	builder *ModuleBuilder
	options Options

	// Type cache (IR TypeHandle → SPIR-V ID)
	typeIDs map[ir.TypeHandle]uint32

	// Constant cache (IR ConstantHandle → SPIR-V ID)
	constantIDs map[ir.ConstantHandle]uint32

	// Global variable cache
	globalIDs map[ir.GlobalVariableHandle]uint32

	// Function cache
	functionIDs map[ir.FunctionHandle]uint32

	// GLSL.std.450 import ID (for math functions)
	glslExtID uint32

	// Entry point interface variables (for builtins and locations).
	// Key: entry point function handle, Value: map of arg index → variable ID
	entryInputVars  map[ir.FunctionHandle]map[int]uint32
	entryOutputVars map[ir.FunctionHandle]uint32 // For function result
}

// NewBackend creates a new SPIR-V backend.
func NewBackend(options Options) *Backend {
	return &Backend{
		options:         options,
		typeIDs:         make(map[ir.TypeHandle]uint32),
		constantIDs:     make(map[ir.ConstantHandle]uint32),
		globalIDs:       make(map[ir.GlobalVariableHandle]uint32),
		functionIDs:     make(map[ir.FunctionHandle]uint32),
		entryInputVars:  make(map[ir.FunctionHandle]map[int]uint32),
		entryOutputVars: make(map[ir.FunctionHandle]uint32),
	}
}

// Compile translates an IR module to SPIR-V binary.
func (b *Backend) Compile(module *ir.Module) ([]byte, error) {
	b.module = module
	b.builder = NewModuleBuilder(b.options.Version)

	// 1. Capabilities
	b.emitCapabilities()

	// 2. Extensions (if needed)
	// b.emitExtensions()

	// 3. Extended instruction sets
	b.glslExtID = b.builder.AddExtInstImport("GLSL.std.450")

	// 4. Memory model
	b.builder.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

	// 5. Entry points (deferred until we know function IDs)
	// Will be added after emitting functions

	// 6. Execution modes (deferred)
	// Will be added after entry points

	// 7. Debug names (if debug enabled)
	if b.options.Debug {
		b.emitDebugNames()
	}

	// 8. Decorations
	b.emitDecorations()

	// 9. Types and constants
	if err := b.emitTypes(); err != nil {
		return nil, err
	}
	if err := b.emitConstants(); err != nil {
		return nil, err
	}

	// 10. Global variables
	if err := b.emitGlobals(); err != nil {
		return nil, err
	}

	// 10.5. Entry point interface variables (builtins, locations)
	if err := b.emitEntryPointInterfaceVars(); err != nil {
		return nil, err
	}

	// 11. Functions
	if err := b.emitFunctions(); err != nil {
		return nil, err
	}

	// 12. Entry points (now that we have function IDs)
	if err := b.emitEntryPoints(); err != nil {
		return nil, err
	}

	return b.builder.Build(), nil
}

// emitCapabilities adds required SPIR-V capabilities.
func (b *Backend) emitCapabilities() {
	// Shader capability is required for all shader stages
	b.builder.AddCapability(CapabilityShader)

	// Add user-requested capabilities
	for _, cap := range b.options.Capabilities {
		b.builder.AddCapability(cap)
	}
}

// emitDebugNames adds debug names for types, constants, globals, and functions.
func (b *Backend) emitDebugNames() {
	// Type names
	for handle, typ := range b.module.Types {
		if typ.Name != "" {
			if id, ok := b.typeIDs[ir.TypeHandle(handle)]; ok {
				b.builder.AddName(id, typ.Name)
			}
		}
	}

	// Constant names
	for handle, constant := range b.module.Constants {
		if constant.Name != "" {
			if id, ok := b.constantIDs[ir.ConstantHandle(handle)]; ok {
				b.builder.AddName(id, constant.Name)
			}
		}
	}

	// Global variable names
	for handle, global := range b.module.GlobalVariables {
		if global.Name != "" {
			if id, ok := b.globalIDs[ir.GlobalVariableHandle(handle)]; ok {
				b.builder.AddName(id, global.Name)
			}
		}
	}

	// Function names
	for handle := range b.module.Functions {
		fn := &b.module.Functions[handle]
		if fn.Name != "" {
			if id, ok := b.functionIDs[ir.FunctionHandle(handle)]; ok {
				b.builder.AddName(id, fn.Name)
			}
		}
	}
}

// emitDecorations adds decorations for globals and entry points.
func (b *Backend) emitDecorations() {
	// Global variable decorations (bindings, locations, built-ins)
	for handle, global := range b.module.GlobalVariables {
		id, ok := b.globalIDs[ir.GlobalVariableHandle(handle)]
		if !ok {
			continue
		}

		// Resource bindings (@group, @binding)
		if global.Binding != nil {
			b.builder.AddDecorate(id, DecorationDescriptorSet, global.Binding.Group)
			b.builder.AddDecorate(id, DecorationBinding, global.Binding.Binding)
		}
	}

	// Struct member decorations (offsets)
	for handle, typ := range b.module.Types {
		structType, ok := typ.Inner.(ir.StructType)
		if !ok {
			continue
		}

		structID, ok := b.typeIDs[ir.TypeHandle(handle)]
		if !ok {
			continue
		}

		for memberIndex, member := range structType.Members {
			b.builder.AddMemberDecorate(structID, uint32(memberIndex), DecorationOffset, member.Offset)

			// Add member names if debug enabled
			if b.options.Debug && member.Name != "" {
				b.builder.AddMemberName(structID, uint32(memberIndex), member.Name)
			}
		}
	}
}

// emitTypes emits all IR types to SPIR-V.
func (b *Backend) emitTypes() error {
	for handle := range b.module.Types {
		if _, err := b.emitType(ir.TypeHandle(handle)); err != nil {
			return err
		}
	}
	return nil
}

// emitType emits a single IR type and returns its SPIR-V ID.
// Uses caching to ensure type deduplication.
func (b *Backend) emitType(handle ir.TypeHandle) (uint32, error) {
	// Check cache
	if id, ok := b.typeIDs[handle]; ok {
		return id, nil
	}

	typ := &b.module.Types[handle]
	var id uint32

	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		id = b.emitScalarType(inner)

	case ir.VectorType:
		scalarID := b.emitScalarType(inner.Scalar)
		id = b.builder.AddTypeVector(scalarID, uint32(inner.Size))

	case ir.MatrixType:
		scalarID := b.emitScalarType(inner.Scalar)
		columnTypeID := b.builder.AddTypeVector(scalarID, uint32(inner.Rows))
		id = b.builder.AddTypeMatrix(columnTypeID, uint32(inner.Columns))

	case ir.ArrayType:
		baseID, err := b.emitType(inner.Base)
		if err != nil {
			return 0, err
		}

		// Array size (constant or runtime-sized)
		var sizeID uint32
		if inner.Size.Constant != nil {
			// Fixed-size array: create u32 constant for length
			u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			sizeID = b.builder.AddConstant(u32TypeID, *inner.Size.Constant)
		} else {
			// Runtime-sized array: use OpTypeRuntimeArray (not implemented yet)
			return 0, fmt.Errorf("runtime-sized arrays not yet implemented")
		}

		id = b.builder.AddTypeArray(baseID, sizeID)

		// Add ArrayStride decoration if stride > 0
		if inner.Stride > 0 {
			b.builder.AddDecorate(id, DecorationArrayStride, inner.Stride)
		}

	case ir.StructType:
		// Emit all member types first
		memberIDs := make([]uint32, len(inner.Members))
		for i, member := range inner.Members {
			memberID, err := b.emitType(member.Type)
			if err != nil {
				return 0, err
			}
			memberIDs[i] = memberID
		}

		id = b.builder.AddTypeStruct(memberIDs...)

	case ir.PointerType:
		baseID, err := b.emitType(inner.Base)
		if err != nil {
			return 0, err
		}

		storageClass := addressSpaceToStorageClass(inner.Space)
		id = b.builder.AddTypePointer(storageClass, baseID)

	case ir.SamplerType:
		// OpTypeSampler has no operands
		id = b.builder.AllocID()
		builder := NewInstructionBuilder()
		builder.AddWord(id)
		b.builder.types = append(b.builder.types, builder.Build(OpTypeSampler))

	case ir.ImageType:
		// OpTypeImage (simplified - full implementation would need more work)
		sampledTypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		id = b.emitImageType(sampledTypeID, inner)

	case ir.AtomicType:
		// Atomic types in SPIR-V are just the underlying scalar type
		// The atomicity is expressed through OpAtomic* instructions
		id = b.emitScalarType(inner.Scalar)

	default:
		return 0, fmt.Errorf("unsupported type: %T", inner)
	}

	// Cache the result
	b.typeIDs[handle] = id
	return id, nil
}

// emitScalarType emits a scalar type and returns its SPIR-V ID.
// This is a helper that doesn't use the type cache.
func (b *Backend) emitScalarType(scalar ir.ScalarType) uint32 {
	switch scalar.Kind {
	case ir.ScalarBool:
		return b.builder.AddTypeBool()

	case ir.ScalarFloat:
		return b.builder.AddTypeFloat(uint32(scalar.Width) * 8) // bytes to bits

	case ir.ScalarSint:
		return b.builder.AddTypeInt(uint32(scalar.Width)*8, true)

	case ir.ScalarUint:
		return b.builder.AddTypeInt(uint32(scalar.Width)*8, false)

	default:
		panic(fmt.Sprintf("unknown scalar kind: %v", scalar.Kind))
	}
}

// emitImageType emits OpTypeImage.
func (b *Backend) emitImageType(sampledTypeID uint32, img ir.ImageType) uint32 {
	id := b.builder.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(id)
	builder.AddWord(sampledTypeID)

	// Dimensionality
	var dim uint32
	switch img.Dim {
	case ir.Dim1D:
		dim = 0 // spirv::Dim::Dim1D
	case ir.Dim2D:
		dim = 1 // spirv::Dim::Dim2D
	case ir.Dim3D:
		dim = 2 // spirv::Dim::Dim3D
	case ir.DimCube:
		dim = 3 // spirv::Dim::Cube
	}
	builder.AddWord(dim)

	// Depth (0 = no depth, 1 = depth, 2 = unknown)
	var depth uint32
	switch img.Class {
	case ir.ImageClassDepth:
		depth = 1
	case ir.ImageClassSampled, ir.ImageClassStorage:
		depth = 0
	default:
		depth = 2
	}
	builder.AddWord(depth)

	// Arrayed
	if img.Arrayed {
		builder.AddWord(1)
	} else {
		builder.AddWord(0)
	}

	// Multisampled
	if img.Multisampled {
		builder.AddWord(1)
	} else {
		builder.AddWord(0)
	}

	// Sampled (1 = sampled, 2 = storage)
	sampled := uint32(1)
	if img.Class == ir.ImageClassStorage {
		sampled = 2
	}
	builder.AddWord(sampled)

	// Image format (for storage images; Unknown for sampled)
	builder.AddWord(0) // spirv::ImageFormat::Unknown

	b.builder.types = append(b.builder.types, builder.Build(OpTypeImage))
	return id
}

// addressSpaceToStorageClass converts IR AddressSpace to SPIR-V StorageClass.
func addressSpaceToStorageClass(space ir.AddressSpace) StorageClass {
	switch space {
	case ir.SpaceFunction:
		return StorageClassFunction
	case ir.SpacePrivate:
		return StorageClassPrivate
	case ir.SpaceWorkGroup:
		return StorageClassWorkgroup
	case ir.SpaceUniform:
		return StorageClassUniform
	case ir.SpaceStorage:
		return StorageClassStorageBuffer
	case ir.SpacePushConstant:
		return StorageClassPushConstant
	case ir.SpaceHandle:
		return StorageClassUniformConstant
	default:
		panic(fmt.Sprintf("unknown address space: %v", space))
	}
}

// emitConstants emits all IR constants to SPIR-V.
func (b *Backend) emitConstants() error {
	for handle := range b.module.Constants {
		if _, err := b.emitConstant(ir.ConstantHandle(handle)); err != nil {
			return err
		}
	}
	return nil
}

// emitConstant emits a single IR constant and returns its SPIR-V ID.
func (b *Backend) emitConstant(handle ir.ConstantHandle) (uint32, error) {
	// Check cache
	if id, ok := b.constantIDs[handle]; ok {
		return id, nil
	}

	constant := &b.module.Constants[handle]

	// Get type ID
	typeID, err := b.emitType(constant.Type)
	if err != nil {
		return 0, err
	}

	var id uint32

	switch value := constant.Value.(type) {
	case ir.ScalarValue:
		id = b.emitScalarConstant(typeID, value)

	case ir.CompositeValue:
		// Emit all component constants first
		componentIDs := make([]uint32, len(value.Components))
		for i, componentHandle := range value.Components {
			componentID, err := b.emitConstant(componentHandle)
			if err != nil {
				return 0, err
			}
			componentIDs[i] = componentID
		}

		id = b.builder.AddConstantComposite(typeID, componentIDs...)

	default:
		return 0, fmt.Errorf("unsupported constant value type: %T", value)
	}

	// Cache the result
	b.constantIDs[handle] = id
	return id, nil
}

// emitScalarConstant emits a scalar constant.
func (b *Backend) emitScalarConstant(typeID uint32, value ir.ScalarValue) uint32 {
	switch value.Kind {
	case ir.ScalarBool:
		if value.Bits != 0 {
			// OpConstantTrue
			id := b.builder.AllocID()
			builder := NewInstructionBuilder()
			builder.AddWord(typeID)
			builder.AddWord(id)
			b.builder.types = append(b.builder.types, builder.Build(OpConstantTrue))
			return id
		}
		// OpConstantFalse
		id := b.builder.AllocID()
		builder := NewInstructionBuilder()
		builder.AddWord(typeID)
		builder.AddWord(id)
		b.builder.types = append(b.builder.types, builder.Build(OpConstantFalse))
		return id

	case ir.ScalarFloat:
		// Determine width from type
		typ := &b.module.Types[b.findTypeHandleByID(typeID)]
		scalarType := typ.Inner.(ir.ScalarType)
		if scalarType.Width == 4 {
			// 32-bit float
			return b.builder.AddConstantFloat32(typeID, math.Float32frombits(uint32(value.Bits)))
		}
		// 64-bit float
		return b.builder.AddConstantFloat64(typeID, math.Float64frombits(value.Bits))

	case ir.ScalarSint, ir.ScalarUint:
		// For integers, just pass the bits directly
		// Handle 64-bit integers (need two words)
		typ := &b.module.Types[b.findTypeHandleByID(typeID)]
		scalarType := typ.Inner.(ir.ScalarType)
		if scalarType.Width == 8 {
			// 64-bit integer
			lowBits := uint32(value.Bits & 0xFFFFFFFF)
			highBits := uint32(value.Bits >> 32)
			return b.builder.AddConstant(typeID, lowBits, highBits)
		}
		// 32-bit or smaller integer
		return b.builder.AddConstant(typeID, uint32(value.Bits))

	default:
		panic(fmt.Sprintf("unknown scalar kind: %v", value.Kind))
	}
}

// findTypeHandleByID finds the IR TypeHandle for a given SPIR-V type ID.
func (b *Backend) findTypeHandleByID(id uint32) ir.TypeHandle {
	for handle, typeID := range b.typeIDs {
		if typeID == id {
			return handle
		}
	}
	panic(fmt.Sprintf("type ID %d not found in cache", id))
}

// OpConstantTrue represents OpConstantTrue opcode.
const OpConstantTrue OpCode = 41

// OpConstantFalse represents OpConstantFalse opcode.
const OpConstantFalse OpCode = 42

// OpTypeSampler represents OpTypeSampler opcode.
const OpTypeSampler OpCode = 26

// OpTypeImage represents OpTypeImage opcode.
const OpTypeImage OpCode = 25

// emitGlobals emits all global variables to SPIR-V.
func (b *Backend) emitGlobals() error {
	for handle, global := range b.module.GlobalVariables {
		// Get the variable type
		varType, err := b.emitType(global.Type)
		if err != nil {
			return err
		}

		// Create pointer type for the variable
		storageClass := addressSpaceToStorageClass(global.Space)
		ptrType := b.builder.AddTypePointer(storageClass, varType)

		// Emit the variable
		var varID uint32
		if global.Init != nil {
			// Variable with initializer
			initID, err := b.emitConstant(*global.Init)
			if err != nil {
				return err
			}
			varID = b.builder.AddVariableWithInit(ptrType, storageClass, initID)
		} else {
			// Variable without initializer
			varID = b.builder.AddVariable(ptrType, storageClass)
		}

		// Cache the variable ID
		b.globalIDs[ir.GlobalVariableHandle(handle)] = varID
	}
	return nil
}

// emitEntryPointInterfaceVars creates input/output variables for entry point builtins and locations.
// In SPIR-V, entry point functions don't receive builtins as parameters.
// Instead, builtins are global variables with Input/Output storage class.
func (b *Backend) emitEntryPointInterfaceVars() error {
	for _, entryPoint := range b.module.EntryPoints {
		fn := &b.module.Functions[entryPoint.Function]
		inputVars := make(map[int]uint32)

		// Create input variables for function arguments with bindings
		for i, arg := range fn.Arguments {
			if arg.Binding == nil {
				continue
			}

			argTypeID, err := b.emitType(arg.Type)
			if err != nil {
				return err
			}

			// Create pointer type with Input storage class
			ptrType := b.builder.AddTypePointer(StorageClassInput, argTypeID)

			// Create variable
			varID := b.builder.AddVariable(ptrType, StorageClassInput)
			inputVars[i] = varID

			// Add decoration based on binding type
			switch binding := (*arg.Binding).(type) {
			case ir.BuiltinBinding:
				spirvBuiltin := builtinToSPIRV(binding.Builtin)
				b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
			case ir.LocationBinding:
				b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
			}

			// Add debug name if enabled
			if b.options.Debug && arg.Name != "" {
				b.builder.AddName(varID, arg.Name)
			}
		}

		b.entryInputVars[entryPoint.Function] = inputVars

		// Create output variable for function result with binding
		if fn.Result != nil && fn.Result.Binding != nil {
			resultTypeID, err := b.emitType(fn.Result.Type)
			if err != nil {
				return err
			}

			// Create pointer type with Output storage class
			ptrType := b.builder.AddTypePointer(StorageClassOutput, resultTypeID)

			// Create variable
			varID := b.builder.AddVariable(ptrType, StorageClassOutput)
			b.entryOutputVars[entryPoint.Function] = varID

			// Add decoration based on binding type
			switch binding := (*fn.Result.Binding).(type) {
			case ir.BuiltinBinding:
				spirvBuiltin := builtinToSPIRV(binding.Builtin)
				b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
			case ir.LocationBinding:
				b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
			}
		}
	}
	return nil
}

// builtinToSPIRV converts IR builtin value to SPIR-V BuiltIn.
func builtinToSPIRV(builtin ir.BuiltinValue) BuiltIn {
	switch builtin {
	case ir.BuiltinPosition:
		return BuiltInPosition
	case ir.BuiltinVertexIndex:
		return BuiltInVertexIndex
	case ir.BuiltinInstanceIndex:
		return BuiltInInstanceIndex
	case ir.BuiltinFrontFacing:
		return BuiltInFrontFacing
	case ir.BuiltinFragDepth:
		return BuiltInFragDepth
	case ir.BuiltinSampleIndex:
		return BuiltInSampleID
	case ir.BuiltinSampleMask:
		return BuiltInSampleMask
	case ir.BuiltinLocalInvocationID:
		return BuiltInLocalInvocationID
	case ir.BuiltinLocalInvocationIndex:
		return BuiltInLocalInvocationIndex
	case ir.BuiltinGlobalInvocationID:
		return BuiltInGlobalInvocationID
	case ir.BuiltinWorkGroupID:
		return BuiltInWorkgroupID
	case ir.BuiltinNumWorkGroups:
		return BuiltInNumWorkgroups
	default:
		return BuiltInPosition // Fallback
	}
}

// emitEntryPoints emits all entry points with their execution modes.
func (b *Backend) emitEntryPoints() error {
	for _, entryPoint := range b.module.EntryPoints {
		// Get function ID
		funcID, ok := b.functionIDs[entryPoint.Function]
		if !ok {
			return fmt.Errorf("entry point function not found: %v", entryPoint.Function)
		}

		// Determine execution model
		var execModel ExecutionModel
		switch entryPoint.Stage {
		case ir.StageVertex:
			execModel = ExecutionModelVertex
		case ir.StageFragment:
			execModel = ExecutionModelFragment
		case ir.StageCompute:
			execModel = ExecutionModelGLCompute
		default:
			return fmt.Errorf("unsupported shader stage: %v", entryPoint.Stage)
		}

		// Collect interface variables (inputs/outputs used by entry point)
		var interfaces []uint32

		// Add regular global variables with Input/Output storage class
		for handle, global := range b.module.GlobalVariables {
			if global.Space == ir.SpaceFunction || global.Space == ir.SpacePrivate {
				continue // Not interface variables
			}
			if varID, ok := b.globalIDs[ir.GlobalVariableHandle(handle)]; ok {
				interfaces = append(interfaces, varID)
			}
		}

		// Add entry point input variables (builtins, locations)
		if inputVars, ok := b.entryInputVars[entryPoint.Function]; ok {
			for _, varID := range inputVars {
				interfaces = append(interfaces, varID)
			}
		}

		// Add entry point output variable
		if outputVar, ok := b.entryOutputVars[entryPoint.Function]; ok {
			interfaces = append(interfaces, outputVar)
		}

		// Add entry point
		b.builder.AddEntryPoint(execModel, funcID, entryPoint.Name, interfaces)

		// Add execution modes based on stage
		switch entryPoint.Stage {
		case ir.StageFragment:
			// Fragment shaders need OriginUpperLeft
			b.builder.AddExecutionMode(funcID, ExecutionModeOriginUpperLeft)

		case ir.StageCompute:
			// Compute shaders need LocalSize
			b.builder.AddExecutionMode(funcID, ExecutionModeLocalSize,
				entryPoint.Workgroup[0],
				entryPoint.Workgroup[1],
				entryPoint.Workgroup[2])
		}
	}
	return nil
}

// emitFunctions emits all functions.
func (b *Backend) emitFunctions() error {
	for handle := range b.module.Functions {
		fn := &b.module.Functions[handle]
		if err := b.emitFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
	}
	return nil
}

// emitFunction emits a single function.
//
//nolint:gocognit,gocyclo,cyclop,nestif // SPIR-V generation has inherent complexity from spec requirements
func (b *Backend) emitFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	// Check if this is an entry point function
	isEntryPoint := false
	for _, ep := range b.module.EntryPoints {
		if ep.Function == handle {
			isEntryPoint = true
			break
		}
	}

	var returnTypeID uint32
	var paramTypeIDs []uint32
	paramIDs := make([]uint32, len(fn.Arguments))

	if isEntryPoint {
		// Entry point functions are void with no parameters in SPIR-V.
		// They use Input/Output global variables instead.
		returnTypeID = b.builder.AddTypeVoid()
		paramTypeIDs = nil
	} else {
		// Regular function - determine return type
		if fn.Result != nil {
			var err error
			returnTypeID, err = b.emitType(fn.Result.Type)
			if err != nil {
				return err
			}
		} else {
			returnTypeID = b.builder.AddTypeVoid()
		}

		// Emit parameter types
		paramTypeIDs = make([]uint32, len(fn.Arguments))
		for i, arg := range fn.Arguments {
			var err error
			paramTypeIDs[i], err = b.emitType(arg.Type)
			if err != nil {
				return err
			}
		}
	}

	// Create function type
	funcTypeID := b.builder.AddTypeFunction(returnTypeID, paramTypeIDs...)

	// Emit function declaration
	funcID := b.builder.AddFunction(funcTypeID, returnTypeID, FunctionControlNone)
	b.functionIDs[handle] = funcID

	// Emit function parameters (only for non-entry-point functions)
	if !isEntryPoint {
		for i, arg := range fn.Arguments {
			paramID := b.builder.AddFunctionParameter(paramTypeIDs[i])
			paramIDs[i] = paramID

			// Add debug name if enabled
			if b.options.Debug && arg.Name != "" {
				b.builder.AddName(paramID, arg.Name)
			}
		}
	}

	// Emit function body
	b.builder.AddLabel() // Entry block

	// IMPORTANT: SPIR-V requires all OpVariable instructions at the START of the
	// first block, BEFORE any other instructions (including OpLoad).
	// Order: OpLabel -> OpVariable(s) -> OpLoad(s) -> other instructions

	// 1. First, emit local variables (OpVariable)
	localVarIDs := make([]uint32, len(fn.LocalVars))
	for i, localVar := range fn.LocalVars {
		varType, err := b.emitType(localVar.Type)
		if err != nil {
			return err
		}

		// Create pointer to function storage class
		ptrType := b.builder.AddTypePointer(StorageClassFunction, varType)

		// Allocate variable (OpVariable in function body)
		varID := b.builder.AllocID()
		builder := NewInstructionBuilder()
		builder.AddWord(ptrType)
		builder.AddWord(varID)
		builder.AddWord(uint32(StorageClassFunction))
		b.builder.functions = append(b.builder.functions, builder.Build(OpVariable))

		localVarIDs[i] = varID

		// Add debug name if enabled
		if b.options.Debug && localVar.Name != "" {
			b.builder.AddName(varID, localVar.Name)
		}
	}

	// 2. Now, load input variables for entry points (OpLoad comes AFTER OpVariable)
	var outputVarID uint32
	if isEntryPoint {
		if inputVars, ok := b.entryInputVars[handle]; ok {
			for i, varID := range inputVars {
				// Get the type for loading
				argTypeID, _ := b.emitType(fn.Arguments[i].Type)
				// Load from input variable
				loadedID := b.builder.AddLoad(argTypeID, varID)
				paramIDs[i] = loadedID
			}
		}
		outputVarID = b.entryOutputVars[handle]
	}

	// 3. Create expression emitter for this function
	emitter := &ExpressionEmitter{
		backend:      b,
		function:     fn,
		exprIDs:      make(map[ir.ExpressionHandle]uint32),
		paramIDs:     paramIDs,
		isEntryPoint: isEntryPoint,
		outputVarID:  outputVarID,
	}
	emitter.localVarIDs = localVarIDs

	// 4. Initialize local variables (now that emitter is available)
	for i, localVar := range fn.LocalVars {
		if localVar.Init != nil {
			initID, err := emitter.emitExpression(*localVar.Init)
			if err != nil {
				return err
			}
			b.builder.AddStore(localVarIDs[i], initID)
		}
	}

	// Emit function body statements
	for _, stmt := range fn.Body {
		if err := emitter.emitStatement(stmt); err != nil {
			return err
		}
	}

	// Add OpFunctionEnd
	b.builder.AddFunctionEnd()

	return nil
}

// ExpressionEmitter handles expression emission within a function context.
type ExpressionEmitter struct {
	backend     *Backend
	function    *ir.Function // Renamed from fn for consistency
	exprIDs     map[ir.ExpressionHandle]uint32
	paramIDs    []uint32 // Function parameter IDs (or loaded input values for entry points)
	localVarIDs []uint32 // Local variable IDs

	// Entry point context
	isEntryPoint bool   // True if this is an entry point function
	outputVarID  uint32 // Output variable ID for entry point result

	// Loop context stack for break/continue
	loopStack []loopContext
}

// loopContext tracks merge and continue labels for loop statements.
type loopContext struct {
	mergeLabel    uint32 // Label to branch to on break
	continueLabel uint32 // Label to branch to on continue
}

// resolveTypeResolution converts a TypeResolution to a SPIR-V type ID.
// Handles both type handles (references to module types) and inline types.
func (b *Backend) resolveTypeResolution(res ir.TypeResolution) uint32 {
	if res.Handle != nil {
		// Type handle - look up in cache
		if id, ok := b.typeIDs[*res.Handle]; ok {
			return id
		}
		// Not in cache - emit the type
		id, err := b.emitType(*res.Handle)
		if err != nil {
			// This shouldn't happen if types were properly registered
			panic(fmt.Sprintf("failed to emit type handle %d: %v", *res.Handle, err))
		}
		return id
	}

	// Inline type - emit and cache
	return b.emitInlineType(res.Value)
}

// emitInlineType emits an inline TypeInner and returns its SPIR-V ID.
// Used for types that don't exist in the module's type arena (e.g., temporary vector types).
func (b *Backend) emitInlineType(inner ir.TypeInner) uint32 {
	switch t := inner.(type) {
	case ir.ScalarType:
		return b.emitScalarType(t)

	case ir.VectorType:
		scalarID := b.emitScalarType(t.Scalar)
		return b.builder.AddTypeVector(scalarID, uint32(t.Size))

	case ir.MatrixType:
		scalarID := b.emitScalarType(t.Scalar)
		columnTypeID := b.builder.AddTypeVector(scalarID, uint32(t.Rows))
		return b.builder.AddTypeMatrix(columnTypeID, uint32(t.Columns))

	case ir.PointerType:
		// Emit the base type first
		baseID, err := b.emitType(t.Base)
		if err != nil {
			panic(fmt.Sprintf("failed to emit pointer base type: %v", err))
		}
		storageClass := addressSpaceToStorageClass(t.Space)
		return b.builder.AddTypePointer(storageClass, baseID)

	default:
		// For complex types that need handles, we should panic
		panic(fmt.Sprintf("cannot emit inline type: %T (should be in module types)", inner))
	}
}

// emitExpression emits an expression and returns its SPIR-V ID.
//
//nolint:gocyclo,cyclop // Expression dispatch requires high cyclomatic complexity
func (e *ExpressionEmitter) emitExpression(handle ir.ExpressionHandle) (uint32, error) {
	// Check cache
	if id, ok := e.exprIDs[handle]; ok {
		return id, nil
	}

	expr := &e.function.Expressions[handle]
	var id uint32
	var err error

	switch kind := expr.Kind.(type) {
	case ir.Literal:
		// Emit literal as constant
		id, err = e.emitLiteral(kind.Value)

	case ir.ExprConstant:
		// Reference module-level constant
		id, ok := e.backend.constantIDs[kind.Constant]
		if !ok {
			return 0, fmt.Errorf("constant not found: %v", kind.Constant)
		}
		return id, nil

	case ir.ExprCompose:
		// Composite construction
		id, err = e.emitCompose(kind)

	case ir.ExprAccess:
		// Dynamic array/vector access
		id, err = e.emitAccess(kind)

	case ir.ExprAccessIndex:
		// Static array/vector/struct access
		id, err = e.emitAccessIndex(kind)

	case ir.ExprFunctionArgument:
		// Function parameter reference
		if int(kind.Index) >= len(e.paramIDs) {
			return 0, fmt.Errorf("function argument index out of range: %d", kind.Index)
		}
		return e.paramIDs[kind.Index], nil

	case ir.ExprGlobalVariable:
		// Global variable reference
		id, ok := e.backend.globalIDs[kind.Variable]
		if !ok {
			return 0, fmt.Errorf("global variable not found: %v", kind.Variable)
		}
		return id, nil

	case ir.ExprLocalVariable:
		// Local variable reference
		if int(kind.Variable) >= len(e.localVarIDs) {
			return 0, fmt.Errorf("local variable index out of range: %d", kind.Variable)
		}
		return e.localVarIDs[kind.Variable], nil

	case ir.ExprLoad:
		// Load from pointer
		id, err = e.emitLoad(kind)

	case ir.ExprUnary:
		// Unary operation
		id, err = e.emitUnary(kind)

	case ir.ExprBinary:
		// Binary operation
		id, err = e.emitBinary(kind)

	case ir.ExprSelect:
		// Select (ternary operator)
		id, err = e.emitSelect(kind)

	case ir.ExprMath:
		// Math built-in functions
		id, err = e.emitMath(kind)

	case ir.ExprDerivative:
		// Derivative functions
		id, err = e.emitDerivative(kind)

	case ir.ExprImageSample:
		// Texture sampling
		id, err = e.emitImageSample(kind)

	case ir.ExprImageLoad:
		// Texture load
		id, err = e.emitImageLoad(kind)

	case ir.ExprImageQuery:
		// Image query
		id, err = e.emitImageQuery(kind)

	case ir.ExprAtomicResult:
		// Atomic result - should have been set by emitAtomic
		if existingID, ok := e.exprIDs[handle]; ok {
			return existingID, nil
		}
		return 0, fmt.Errorf("atomic result expression not found - emitAtomic should have set it")

	case ir.ExprSwizzle:
		// Vector swizzle (.xyz, .rgb, etc.)
		id, err = e.emitSwizzle(kind)

	default:
		return 0, fmt.Errorf("unsupported expression kind: %T", kind)
	}

	if err != nil {
		return 0, err
	}

	// Cache the result
	e.exprIDs[handle] = id
	return id, nil
}

// emitLiteral emits a literal value.
func (e *ExpressionEmitter) emitLiteral(value ir.LiteralValue) (uint32, error) {
	switch v := value.(type) {
	case ir.LiteralF32:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		return e.backend.builder.AddConstantFloat32(typeID, float32(v)), nil

	case ir.LiteralF64:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 8})
		return e.backend.builder.AddConstantFloat64(typeID, float64(v)), nil

	case ir.LiteralU32:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		return e.backend.builder.AddConstant(typeID, uint32(v)), nil

	case ir.LiteralI32:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
		return e.backend.builder.AddConstant(typeID, uint32(v)), nil

	case ir.LiteralBool:
		typeID := e.backend.builder.AddTypeBool()
		if v {
			// OpConstantTrue
			resultID := e.backend.builder.AllocID()
			builder := NewInstructionBuilder()
			builder.AddWord(typeID)
			builder.AddWord(resultID)
			e.backend.builder.types = append(e.backend.builder.types, builder.Build(OpConstantTrue))
			return resultID, nil
		}
		// OpConstantFalse
		resultID := e.backend.builder.AllocID()
		builder := NewInstructionBuilder()
		builder.AddWord(typeID)
		builder.AddWord(resultID)
		e.backend.builder.types = append(e.backend.builder.types, builder.Build(OpConstantFalse))
		return resultID, nil

	default:
		return 0, fmt.Errorf("unsupported literal type: %T", v)
	}
}

// emitCompose emits a composite construction.
func (e *ExpressionEmitter) emitCompose(compose ir.ExprCompose) (uint32, error) {
	typeID, err := e.backend.emitType(compose.Type)
	if err != nil {
		return 0, err
	}

	// Emit all components
	componentIDs := make([]uint32, len(compose.Components))
	for i, component := range compose.Components {
		componentIDs[i], err = e.emitExpression(component)
		if err != nil {
			return 0, err
		}
	}

	return e.backend.builder.AddCompositeConstruct(typeID, componentIDs...), nil
}

// emitAccess emits a dynamic access operation.
// Returns the VALUE at the indexed location (not a pointer).
func (e *ExpressionEmitter) emitAccess(access ir.ExprAccess) (uint32, error) {
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}

	indexID, err := e.emitExpression(access.Index)
	if err != nil {
		return 0, err
	}

	// Get result type from type inference
	baseType, err := ir.ResolveExpressionType(e.backend.module, e.function, access.Base)
	if err != nil {
		return 0, fmt.Errorf("access base type: %w", err)
	}

	// Determine the element type
	var elementType ir.TypeResolution
	if baseType.Handle != nil {
		inner := e.backend.module.Types[*baseType.Handle].Inner
		switch t := inner.(type) {
		case ir.ArrayType:
			h := t.Base
			elementType = ir.TypeResolution{Handle: &h}
		case ir.VectorType:
			elementType = ir.TypeResolution{Value: t.Scalar}
		case ir.MatrixType:
			elementType = ir.TypeResolution{Value: ir.VectorType{Size: t.Rows, Scalar: t.Scalar}}
		case ir.PointerType:
			// If base is pointer, we need to emit OpAccessChain which returns pointer
			// The pointee is what we're indexing into
			h := t.Base
			elementType = ir.TypeResolution{Handle: &h}
		default:
			return 0, fmt.Errorf("cannot index into type %T", t)
		}
	} else {
		inner := baseType.Value
		switch t := inner.(type) {
		case ir.VectorType:
			elementType = ir.TypeResolution{Value: t.Scalar}
		case ir.MatrixType:
			elementType = ir.TypeResolution{Value: ir.VectorType{Size: t.Rows, Scalar: t.Scalar}}
		default:
			return 0, fmt.Errorf("cannot index into inline type %T", t)
		}
	}

	// Get the element type ID
	elementTypeID := e.backend.resolveTypeResolution(elementType)

	// Create pointer type for OpAccessChain result
	// Determine storage class from base (for now, assume Function)
	ptrType := e.backend.builder.AddTypePointer(StorageClassFunction, elementTypeID)

	// OpAccessChain returns a pointer
	ptrID := e.backend.builder.AddAccessChain(ptrType, baseID, indexID)

	// Load the value from the pointer (expressions should return values, not pointers)
	return e.backend.builder.AddLoad(elementTypeID, ptrID), nil
}

// emitAccessIndex emits a static index access operation.
func (e *ExpressionEmitter) emitAccessIndex(access ir.ExprAccessIndex) (uint32, error) {
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}

	// Create constant for index
	u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	indexID := e.backend.builder.AddConstant(u32Type, access.Index)

	// Get result type from type inference (similar to emitAccess)
	baseType, err := ir.ResolveExpressionType(e.backend.module, e.function, access.Base)
	if err != nil {
		return 0, fmt.Errorf("access index base type: %w", err)
	}

	// Determine the element type
	var elementType ir.TypeResolution
	if baseType.Handle != nil {
		inner := e.backend.module.Types[*baseType.Handle].Inner
		switch t := inner.(type) {
		case ir.ArrayType:
			h := t.Base
			elementType = ir.TypeResolution{Handle: &h}
		case ir.VectorType:
			elementType = ir.TypeResolution{Value: t.Scalar}
		case ir.MatrixType:
			elementType = ir.TypeResolution{Value: ir.VectorType{Size: t.Rows, Scalar: t.Scalar}}
		case ir.StructType:
			if int(access.Index) >= len(t.Members) {
				return 0, fmt.Errorf("struct member index %d out of range", access.Index)
			}
			h := t.Members[access.Index].Type
			elementType = ir.TypeResolution{Handle: &h}
		case ir.PointerType:
			h := t.Base
			elementType = ir.TypeResolution{Handle: &h}
		default:
			return 0, fmt.Errorf("cannot index into type %T", t)
		}
	} else {
		inner := baseType.Value
		switch t := inner.(type) {
		case ir.VectorType:
			elementType = ir.TypeResolution{Value: t.Scalar}
		case ir.MatrixType:
			elementType = ir.TypeResolution{Value: ir.VectorType{Size: t.Rows, Scalar: t.Scalar}}
		default:
			return 0, fmt.Errorf("cannot index into inline type %T", t)
		}
	}

	// Get the element type ID
	elementTypeID := e.backend.resolveTypeResolution(elementType)

	// Create pointer type for OpAccessChain result
	ptrType := e.backend.builder.AddTypePointer(StorageClassFunction, elementTypeID)

	// OpAccessChain returns a pointer
	ptrID := e.backend.builder.AddAccessChain(ptrType, baseID, indexID)

	// Load the value from the pointer (expressions should return values, not pointers)
	return e.backend.builder.AddLoad(elementTypeID, ptrID), nil
}

// emitSwizzle emits a vector swizzle operation (.xyz, .rgb, etc.).
// Uses OpVectorShuffle to rearrange vector components.
func (e *ExpressionEmitter) emitSwizzle(swizzle ir.ExprSwizzle) (uint32, error) {
	vectorID, err := e.emitExpression(swizzle.Vector)
	if err != nil {
		return 0, err
	}

	scalar, err := e.extractVectorScalar(swizzle.Vector)
	if err != nil {
		return 0, err
	}

	// Create result type (vector with swizzle.Size components)
	resultTypeID := e.backend.emitInlineType(ir.VectorType{
		Size:   swizzle.Size,
		Scalar: scalar,
	})

	// Build component indices for OpVectorShuffle
	components := make([]uint32, swizzle.Size)
	for i := ir.VectorSize(0); i < swizzle.Size; i++ {
		components[i] = uint32(swizzle.Pattern[i])
	}

	// OpVectorShuffle: shuffles components from one or two vectors
	// We use the same vector for both operands when doing a simple swizzle
	return e.backend.builder.AddVectorShuffle(resultTypeID, vectorID, vectorID, components), nil
}

// extractVectorScalar extracts the scalar type from a vector expression.
func (e *ExpressionEmitter) extractVectorScalar(handle ir.ExpressionHandle) (ir.ScalarType, error) {
	vectorType, err := ir.ResolveExpressionType(e.backend.module, e.function, handle)
	if err != nil {
		return ir.ScalarType{}, fmt.Errorf("swizzle vector type: %w", err)
	}

	if vectorType.Handle != nil {
		inner := e.backend.module.Types[*vectorType.Handle].Inner
		if vec, ok := inner.(ir.VectorType); ok {
			return vec.Scalar, nil
		}
		return ir.ScalarType{}, fmt.Errorf("swizzle requires vector type, got %T", inner)
	}

	if vec, ok := vectorType.Value.(ir.VectorType); ok {
		return vec.Scalar, nil
	}
	return ir.ScalarType{}, fmt.Errorf("swizzle requires vector type, got %T", vectorType.Value)
}

// emitLoad emits a load operation.
func (e *ExpressionEmitter) emitLoad(load ir.ExprLoad) (uint32, error) {
	pointerID, err := e.emitExpression(load.Pointer)
	if err != nil {
		return 0, err
	}

	// Get result type by dereferencing the pointer type
	pointerType, err := ir.ResolveExpressionType(e.backend.module, e.function, load.Pointer)
	if err != nil {
		return 0, fmt.Errorf("load pointer type: %w", err)
	}

	// Extract the pointee type
	var pointeeType ir.TypeResolution
	if pointerType.Handle != nil {
		inner := e.backend.module.Types[*pointerType.Handle].Inner
		ptr, ok := inner.(ir.PointerType)
		if !ok {
			return 0, fmt.Errorf("load requires pointer type, got %T", inner)
		}
		h := ptr.Base
		pointeeType = ir.TypeResolution{Handle: &h}
	} else {
		ptr, ok := pointerType.Value.(ir.PointerType)
		if !ok {
			return 0, fmt.Errorf("load requires pointer type, got %T", pointerType.Value)
		}
		h := ptr.Base
		pointeeType = ir.TypeResolution{Handle: &h}
	}

	resultType := e.backend.resolveTypeResolution(pointeeType)
	return e.backend.builder.AddLoad(resultType, pointerID), nil
}

// emitUnary emits a unary operation.
func (e *ExpressionEmitter) emitUnary(unary ir.ExprUnary) (uint32, error) {
	operandID, err := e.emitExpression(unary.Expr)
	if err != nil {
		return 0, err
	}

	// Get operand type to determine correct opcode
	operandType, err := ir.ResolveExpressionType(e.backend.module, e.function, unary.Expr)
	if err != nil {
		return 0, fmt.Errorf("unary operand type: %w", err)
	}

	// Result type is same as operand type
	resultType := e.backend.resolveTypeResolution(operandType)

	// Determine scalar kind for choosing int vs float opcodes
	var scalarKind ir.ScalarKind
	if operandType.Handle != nil {
		inner := e.backend.module.Types[*operandType.Handle].Inner
		switch t := inner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			return 0, fmt.Errorf("unary operator on non-numeric type: %T", t)
		}
	} else {
		inner := operandType.Value
		switch t := inner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			return 0, fmt.Errorf("unary operator on non-numeric type: %T", t)
		}
	}

	var opcode OpCode
	switch unary.Op {
	case ir.UnaryNegate:
		// Choose float or int negation based on scalar kind
		if scalarKind == ir.ScalarFloat {
			opcode = OpFNegate
		} else {
			opcode = OpSNegate // Signed integer negation
		}
	case ir.UnaryLogicalNot:
		opcode = OpLogicalNot
	case ir.UnaryBitwiseNot:
		opcode = OpNot
	default:
		return 0, fmt.Errorf("unsupported unary operator: %v", unary.Op)
	}

	return e.backend.builder.AddUnaryOp(opcode, resultType, operandID), nil
}

// emitBinary emits a binary operation.
//
//nolint:gocyclo,gocognit,cyclop,funlen,gocritic,staticcheck // Binary operator dispatch requires handling 20+ SPIR-V opcodes
func (e *ExpressionEmitter) emitBinary(binary ir.ExprBinary) (uint32, error) {
	leftID, err := e.emitExpression(binary.Left)
	if err != nil {
		return 0, err
	}

	rightID, err := e.emitExpression(binary.Right)
	if err != nil {
		return 0, err
	}

	// Get left operand type to determine correct opcode
	leftType, err := ir.ResolveExpressionType(e.backend.module, e.function, binary.Left)
	if err != nil {
		return 0, fmt.Errorf("binary left type: %w", err)
	}

	// Determine result type (for most operators, same as operand type; for comparisons, bool)
	var resultType uint32
	var scalarKind ir.ScalarKind

	// Extract scalar kind from left operand
	if leftType.Handle != nil {
		inner := e.backend.module.Types[*leftType.Handle].Inner
		switch t := inner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			return 0, fmt.Errorf("binary operator on non-numeric type: %T", t)
		}
	} else {
		inner := leftType.Value
		switch t := inner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			return 0, fmt.Errorf("binary operator on non-numeric type: %T", t)
		}
	}

	// Determine result type based on operator
	switch binary.Op {
	case ir.BinaryEqual, ir.BinaryNotEqual, ir.BinaryLess, ir.BinaryLessEqual, ir.BinaryGreater, ir.BinaryGreaterEqual:
		// Comparison operators return bool (or vec<bool> for vector operands)
		//nolint:nestif // Type checking requires nested conditionals
		if leftType.Handle != nil {
			inner := e.backend.module.Types[*leftType.Handle].Inner
			if vec, ok := inner.(ir.VectorType); ok {
				// Vector comparison returns vec<bool>
				boolVec := ir.VectorType{
					Size:   vec.Size,
					Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
				}
				resultType = e.backend.emitInlineType(boolVec)
			} else {
				// Scalar comparison returns bool
				resultType = e.backend.builder.AddTypeBool()
			}
		} else {
			inner := leftType.Value
			if vec, ok := inner.(ir.VectorType); ok {
				boolVec := ir.VectorType{
					Size:   vec.Size,
					Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
				}
				resultType = e.backend.emitInlineType(boolVec)
			} else {
				resultType = e.backend.builder.AddTypeBool()
			}
		}
	case ir.BinaryLogicalAnd, ir.BinaryLogicalOr:
		// Logical operators return bool
		resultType = e.backend.builder.AddTypeBool()
	default:
		// Arithmetic and bitwise operators preserve operand type
		resultType = e.backend.resolveTypeResolution(leftType)
	}

	// Map IR operator to SPIR-V opcode based on scalar kind
	var opcode OpCode
	switch binary.Op {
	case ir.BinaryAdd:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFAdd
		} else {
			opcode = OpIAdd
		}
	case ir.BinarySubtract:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFSub
		} else {
			opcode = OpISub
		}
	case ir.BinaryMultiply:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFMul
		} else {
			opcode = OpIMul
		}
	case ir.BinaryDivide:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFDiv
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSDiv
		} else {
			opcode = OpUDiv
		}
	case ir.BinaryModulo:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFMod
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSMod
		} else {
			opcode = OpUMod
		}
	case ir.BinaryEqual:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdEqual
		} else {
			opcode = OpIEqual
		}
	case ir.BinaryNotEqual:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdNotEqual
		} else {
			opcode = OpINotEqual
		}
	case ir.BinaryLess:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdLessThan
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSLessThan
		} else {
			opcode = OpULessThan
		}
	case ir.BinaryLessEqual:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdLessThanEqual
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSLessThanEqual
		} else {
			opcode = OpULessThanEqual
		}
	case ir.BinaryGreater:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdGreaterThan
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSGreaterThan
		} else {
			opcode = OpUGreaterThan
		}
	case ir.BinaryGreaterEqual:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdGreaterThanEqual
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSGreaterThanEqual
		} else {
			opcode = OpUGreaterThanEqual
		}
	case ir.BinaryAnd:
		opcode = OpBitwiseAnd
	case ir.BinaryExclusiveOr:
		opcode = OpBitwiseXor
	case ir.BinaryInclusiveOr:
		opcode = OpBitwiseOr
	case ir.BinaryLogicalAnd:
		opcode = OpLogicalAnd
	case ir.BinaryLogicalOr:
		opcode = OpLogicalOr
	case ir.BinaryShiftLeft:
		opcode = OpShiftLeftLogical
	case ir.BinaryShiftRight:
		if scalarKind == ir.ScalarSint {
			opcode = OpShiftRightArithmetic // Sign-extending
		} else {
			opcode = OpShiftRightLogical // Zero-filling
		}
	default:
		return 0, fmt.Errorf("unsupported binary operator: %v", binary.Op)
	}

	return e.backend.builder.AddBinaryOp(opcode, resultType, leftID, rightID), nil
}

// emitSelect emits a select operation.
func (e *ExpressionEmitter) emitSelect(sel ir.ExprSelect) (uint32, error) {
	conditionID, err := e.emitExpression(sel.Condition)
	if err != nil {
		return 0, err
	}

	acceptID, err := e.emitExpression(sel.Accept)
	if err != nil {
		return 0, err
	}

	rejectID, err := e.emitExpression(sel.Reject)
	if err != nil {
		return 0, err
	}

	// Result type is same as accept/reject branches
	acceptType, err := ir.ResolveExpressionType(e.backend.module, e.function, sel.Accept)
	if err != nil {
		return 0, fmt.Errorf("select accept type: %w", err)
	}
	resultType := e.backend.resolveTypeResolution(acceptType)

	return e.backend.builder.AddSelect(resultType, conditionID, acceptID, rejectID), nil
}

// emitStatement emits a statement.
//
//nolint:cyclop,gocyclo,nestif // Statement dispatch requires high cyclomatic complexity
func (e *ExpressionEmitter) emitStatement(stmt ir.Statement) error {
	switch kind := stmt.Kind.(type) {
	case ir.StmtEmit:
		// Emit all expressions in range
		for handle := kind.Range.Start; handle < kind.Range.End; handle++ {
			_, err := e.emitExpression(handle)
			if err != nil {
				return err
			}
		}
		return nil

	case ir.StmtBlock:
		// Emit all statements in the block
		for _, blockStmt := range kind.Block {
			if err := e.emitStatement(blockStmt); err != nil {
				return err
			}
		}
		return nil

	case ir.StmtIf:
		return e.emitIf(kind)

	case ir.StmtLoop:
		return e.emitLoop(kind)

	case ir.StmtBreak:
		if len(e.loopStack) == 0 {
			return fmt.Errorf("break statement outside of loop")
		}
		ctx := e.loopStack[len(e.loopStack)-1]
		builder := NewInstructionBuilder()
		builder.AddWord(ctx.mergeLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
		return nil

	case ir.StmtContinue:
		if len(e.loopStack) == 0 {
			return fmt.Errorf("continue statement outside of loop")
		}
		ctx := e.loopStack[len(e.loopStack)-1]
		builder := NewInstructionBuilder()
		builder.AddWord(ctx.continueLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
		return nil

	case ir.StmtReturn:
		if kind.Value != nil {
			// Return with value
			valueID, err := e.emitExpression(*kind.Value)
			if err != nil {
				return err
			}
			if e.isEntryPoint && e.outputVarID != 0 {
				// For entry points, store to output variable instead of returning
				e.backend.builder.AddStore(e.outputVarID, valueID)
				e.backend.builder.AddReturn()
			} else {
				e.backend.builder.AddReturnValue(valueID)
			}
		} else {
			// Return void
			e.backend.builder.AddReturn()
		}
		return nil

	case ir.StmtKill:
		e.backend.builder.AddKill()
		return nil

	case ir.StmtStore:
		pointerID, err := e.emitExpression(kind.Pointer)
		if err != nil {
			return err
		}

		valueID, err := e.emitExpression(kind.Value)
		if err != nil {
			return err
		}

		e.backend.builder.AddStore(pointerID, valueID)
		return nil

	case ir.StmtAtomic:
		return e.emitAtomic(kind)

	case ir.StmtBarrier:
		return e.emitBarrier(kind)

	default:
		return fmt.Errorf("unsupported statement kind: %T", kind)
	}
}

// emitIf emits an if statement.
func (e *ExpressionEmitter) emitIf(stmt ir.StmtIf) error {
	// Evaluate condition
	conditionID, err := e.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}

	// Allocate labels
	acceptLabel := e.backend.builder.AllocID()
	rejectLabel := e.backend.builder.AllocID()
	mergeLabel := e.backend.builder.AllocID()

	// OpSelectionMerge declares the merge point
	e.backend.builder.AddSelectionMerge(mergeLabel, SelectionControlNone)

	// OpBranchConditional branches based on condition
	e.backend.builder.AddBranchConditional(conditionID, acceptLabel, rejectLabel)

	// Accept block
	e.backend.builder.AddLabel()
	for _, acceptStmt := range stmt.Accept {
		if err := e.emitStatement(acceptStmt); err != nil {
			return err
		}
	}
	// Branch to merge (unless block already terminated with return/kill)
	builder := NewInstructionBuilder()
	builder.AddWord(mergeLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Reject block
	e.backend.builder.AddLabel()
	for _, rejectStmt := range stmt.Reject {
		if err := e.emitStatement(rejectStmt); err != nil {
			return err
		}
	}
	// Branch to merge (unless block already terminated with return/kill)
	builder = NewInstructionBuilder()
	builder.AddWord(mergeLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Merge label
	e.backend.builder.AddLabel()

	return nil
}

// emitLoop emits a loop statement.
func (e *ExpressionEmitter) emitLoop(stmt ir.StmtLoop) error {
	// Allocate labels
	headerLabel := e.backend.builder.AllocID()
	bodyLabel := e.backend.builder.AllocID()
	continueLabel := e.backend.builder.AllocID()
	mergeLabel := e.backend.builder.AllocID()

	// Push loop context for break/continue
	e.loopStack = append(e.loopStack, loopContext{
		mergeLabel:    mergeLabel,
		continueLabel: continueLabel,
	})
	defer func() {
		e.loopStack = e.loopStack[:len(e.loopStack)-1]
	}()

	// Branch to header
	builder := NewInstructionBuilder()
	builder.AddWord(headerLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Header label
	e.backend.builder.AddLabel()

	// OpLoopMerge declares merge and continue targets
	e.backend.builder.AddLoopMerge(mergeLabel, continueLabel, LoopControlNone)

	// Branch to body
	builder = NewInstructionBuilder()
	builder.AddWord(bodyLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Body label
	e.backend.builder.AddLabel()

	// Emit body statements
	for _, bodyStmt := range stmt.Body {
		if err := e.emitStatement(bodyStmt); err != nil {
			return err
		}
	}

	// Branch to continue block
	builder = NewInstructionBuilder()
	builder.AddWord(continueLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Continue label
	e.backend.builder.AddLabel()

	// Emit continuing statements
	for _, continueStmt := range stmt.Continuing {
		if err := e.emitStatement(continueStmt); err != nil {
			return err
		}
	}

	// Check break-if condition
	if stmt.BreakIf != nil {
		breakCondID, err := e.emitExpression(*stmt.BreakIf)
		if err != nil {
			return err
		}
		// If break condition is true, branch to merge; otherwise back to header
		e.backend.builder.AddBranchConditional(breakCondID, mergeLabel, headerLabel)
	} else {
		// Unconditional back-edge to header
		builder = NewInstructionBuilder()
		builder.AddWord(headerLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
	}

	// Merge label
	e.backend.builder.AddLabel()

	return nil
}

// emitMath emits a math built-in function using GLSL.std.450.
//
//nolint:gocyclo,gocognit,cyclop,funlen,gocritic,staticcheck // Math function dispatch requires handling 40+ GLSL.std.450 instructions
func (e *ExpressionEmitter) emitMath(mathExpr ir.ExprMath) (uint32, error) {
	// Emit first argument
	argID, err := e.emitExpression(mathExpr.Arg)
	if err != nil {
		return 0, err
	}

	// Get argument type to determine result type and correct opcodes
	argType, err := ir.ResolveExpressionType(e.backend.module, e.function, mathExpr.Arg)
	if err != nil {
		return 0, fmt.Errorf("math argument type: %w", err)
	}

	// Determine scalar kind for choosing int vs float functions
	var scalarKind ir.ScalarKind
	if argType.Handle != nil {
		inner := e.backend.module.Types[*argType.Handle].Inner
		switch t := inner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			scalarKind = ir.ScalarFloat // Default for complex types
		}
	} else {
		inner := argType.Value
		switch t := inner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			scalarKind = ir.ScalarFloat
		}
	}

	// Most math functions preserve the argument type
	resultType := e.backend.resolveTypeResolution(argType)

	// Map IR MathFunction to GLSL.std.450 instruction
	var glslInst uint32
	var useNativeOpcode bool
	var nativeOpcode OpCode

	switch mathExpr.Fun {
	// Comparison functions
	case ir.MathAbs:
		if scalarKind == ir.ScalarFloat {
			glslInst = GLSLstd450FAbs
		} else {
			glslInst = GLSLstd450SAbs
		}
	case ir.MathMin:
		if scalarKind == ir.ScalarFloat {
			glslInst = GLSLstd450FMin
		} else if scalarKind == ir.ScalarSint {
			glslInst = GLSLstd450SMin
		} else {
			glslInst = GLSLstd450UMin
		}
	case ir.MathMax:
		if scalarKind == ir.ScalarFloat {
			glslInst = GLSLstd450FMax
		} else if scalarKind == ir.ScalarSint {
			glslInst = GLSLstd450SMax
		} else {
			glslInst = GLSLstd450UMax
		}
	case ir.MathClamp:
		if scalarKind == ir.ScalarFloat {
			glslInst = GLSLstd450FClamp
		} else if scalarKind == ir.ScalarSint {
			glslInst = GLSLstd450SClamp
		} else {
			glslInst = GLSLstd450UClamp
		}
	case ir.MathSaturate:
		// Saturate is clamp(x, 0, 1) - need to construct
		glslInst = GLSLstd450FClamp

	// Trigonometric functions
	case ir.MathCos:
		glslInst = GLSLstd450Cos
	case ir.MathCosh:
		glslInst = GLSLstd450Cosh
	case ir.MathSin:
		glslInst = GLSLstd450Sin
	case ir.MathSinh:
		glslInst = GLSLstd450Sinh
	case ir.MathTan:
		glslInst = GLSLstd450Tan
	case ir.MathTanh:
		glslInst = GLSLstd450Tanh
	case ir.MathAcos:
		glslInst = GLSLstd450Acos
	case ir.MathAsin:
		glslInst = GLSLstd450Asin
	case ir.MathAtan:
		glslInst = GLSLstd450Atan
	case ir.MathAtan2:
		glslInst = GLSLstd450Atan2
	case ir.MathAsinh:
		glslInst = GLSLstd450Asinh
	case ir.MathAcosh:
		glslInst = GLSLstd450Acosh
	case ir.MathAtanh:
		glslInst = GLSLstd450Atanh

	// Angle conversion
	case ir.MathRadians:
		glslInst = GLSLstd450Radians
	case ir.MathDegrees:
		glslInst = GLSLstd450Degrees

	// Decomposition functions
	case ir.MathCeil:
		glslInst = GLSLstd450Ceil
	case ir.MathFloor:
		glslInst = GLSLstd450Floor
	case ir.MathRound:
		glslInst = GLSLstd450Round
	case ir.MathFract:
		glslInst = GLSLstd450Fract
	case ir.MathTrunc:
		glslInst = GLSLstd450Trunc
	case ir.MathModf:
		glslInst = GLSLstd450Modf
	case ir.MathFrexp:
		glslInst = GLSLstd450Frexp
	case ir.MathLdexp:
		glslInst = GLSLstd450Ldexp

	// Exponential functions
	case ir.MathExp:
		glslInst = GLSLstd450Exp
	case ir.MathExp2:
		glslInst = GLSLstd450Exp2
	case ir.MathLog:
		glslInst = GLSLstd450Log
	case ir.MathLog2:
		glslInst = GLSLstd450Log2
	case ir.MathPow:
		glslInst = GLSLstd450Pow

	// Geometric functions
	case ir.MathDot:
		// OpDot is a native SPIR-V instruction
		useNativeOpcode = true
		nativeOpcode = OpDot
	case ir.MathCross:
		glslInst = GLSLstd450Cross
	case ir.MathDistance:
		glslInst = GLSLstd450Distance
	case ir.MathLength:
		glslInst = GLSLstd450Length
	case ir.MathNormalize:
		glslInst = GLSLstd450Normalize
	case ir.MathFaceForward:
		glslInst = GLSLstd450FaceForward
	case ir.MathReflect:
		glslInst = GLSLstd450Reflect
	case ir.MathRefract:
		glslInst = GLSLstd450Refract

	// Computational functions
	case ir.MathSign:
		if scalarKind == ir.ScalarFloat {
			glslInst = GLSLstd450FSign
		} else {
			glslInst = GLSLstd450SSign
		}
	case ir.MathFma:
		glslInst = GLSLstd450Fma
	case ir.MathMix:
		glslInst = GLSLstd450FMix
	case ir.MathStep:
		glslInst = GLSLstd450Step
	case ir.MathSmoothStep:
		glslInst = GLSLstd450SmoothStep
	case ir.MathSqrt:
		glslInst = GLSLstd450Sqrt
	case ir.MathInverseSqrt:
		glslInst = GLSLstd450InverseSqrt
	case ir.MathInverse:
		glslInst = GLSLstd450MatrixInverse
	case ir.MathDeterminant:
		glslInst = GLSLstd450Determinant

	default:
		return 0, fmt.Errorf("unsupported math function: %v", mathExpr.Fun)
	}

	// Collect all operands
	operands := []uint32{argID}
	if mathExpr.Arg1 != nil {
		arg1ID, err := e.emitExpression(*mathExpr.Arg1)
		if err != nil {
			return 0, err
		}
		operands = append(operands, arg1ID)
	}
	if mathExpr.Arg2 != nil {
		arg2ID, err := e.emitExpression(*mathExpr.Arg2)
		if err != nil {
			return 0, err
		}
		operands = append(operands, arg2ID)
	}
	if mathExpr.Arg3 != nil {
		arg3ID, err := e.emitExpression(*mathExpr.Arg3)
		if err != nil {
			return 0, err
		}
		operands = append(operands, arg3ID)
	}

	// Emit instruction
	if useNativeOpcode {
		// Use native SPIR-V opcode (e.g., OpDot)
		return e.backend.builder.AddBinaryOp(nativeOpcode, resultType, operands[0], operands[1]), nil
	}

	// Use GLSL.std.450 extended instruction
	return e.backend.builder.AddExtInst(resultType, e.backend.glslExtID, glslInst, operands...), nil
}

// emitDerivative emits a derivative function.
func (e *ExpressionEmitter) emitDerivative(deriv ir.ExprDerivative) (uint32, error) {
	// Emit expression to take derivative of
	exprID, err := e.emitExpression(deriv.Expr)
	if err != nil {
		return 0, err
	}

	// Get result type from expression (derivative preserves type)
	exprType, err := ir.ResolveExpressionType(e.backend.module, e.function, deriv.Expr)
	if err != nil {
		return 0, fmt.Errorf("derivative expression type: %w", err)
	}
	resultType := e.backend.resolveTypeResolution(exprType)

	// Map axis and control to SPIR-V opcode
	var opcode OpCode
	switch deriv.Axis {
	case ir.DerivativeX:
		switch deriv.Control {
		case ir.DerivativeCoarse:
			opcode = OpDPdxCoarse
		case ir.DerivativeFine:
			opcode = OpDPdxFine
		default:
			opcode = OpDPdx
		}
	case ir.DerivativeY:
		switch deriv.Control {
		case ir.DerivativeCoarse:
			opcode = OpDPdyCoarse
		case ir.DerivativeFine:
			opcode = OpDPdyFine
		default:
			opcode = OpDPdy
		}
	case ir.DerivativeWidth:
		switch deriv.Control {
		case ir.DerivativeCoarse:
			opcode = OpFwidthCoarse
		case ir.DerivativeFine:
			opcode = OpFwidthFine
		default:
			opcode = OpFwidth
		}
	default:
		return 0, fmt.Errorf("unsupported derivative axis: %v", deriv.Axis)
	}

	return e.backend.builder.AddUnaryOp(opcode, resultType, exprID), nil
}

// OpDot represents OpDot opcode (dot product).
const OpDot OpCode = 148

// Image instruction opcodes
const (
	OpSampledImage                OpCode = 86
	OpImageSampleImplicitLod      OpCode = 87
	OpImageSampleExplicitLod      OpCode = 88
	OpImageSampleDrefImplicitLod  OpCode = 89
	OpImageSampleDrefExplicitLod  OpCode = 90
	OpImageSampleProjImplicitLod  OpCode = 91
	OpImageSampleProjExplicitLod  OpCode = 92
	OpImageSampleProjDrefImplicit OpCode = 93
	OpImageSampleProjDrefExplicit OpCode = 94
	OpImageFetch                  OpCode = 95
	OpImageGather                 OpCode = 96
	OpImageDrefGather             OpCode = 97
	OpImageRead                   OpCode = 98
	OpImageWrite                  OpCode = 99
	OpImageQuerySizeLod           OpCode = 103
	OpImageQuerySize              OpCode = 104
	OpImageQueryLod               OpCode = 105
	OpImageQueryLevels            OpCode = 106
	OpImageQuerySamples           OpCode = 107
)

// emitImageSample emits a texture sampling operation.
func (e *ExpressionEmitter) emitImageSample(sample ir.ExprImageSample) (uint32, error) {
	// Get the sampled image (combination of image + sampler)
	imageID, err := e.emitExpression(sample.Image)
	if err != nil {
		return 0, err
	}

	samplerID, err := e.emitExpression(sample.Sampler)
	if err != nil {
		return 0, err
	}

	coordID, err := e.emitExpression(sample.Coordinate)
	if err != nil {
		return 0, err
	}

	// Create SampledImage by combining image and sampler
	// First, get the image type to construct the sampled image type
	sampledImageTypeID := e.backend.getSampledImageType(sample.Image)
	sampledImageID := e.backend.builder.AllocID()

	sampledImageBuilder := NewInstructionBuilder()
	sampledImageBuilder.AddWord(sampledImageTypeID)
	sampledImageBuilder.AddWord(sampledImageID)
	sampledImageBuilder.AddWord(imageID)
	sampledImageBuilder.AddWord(samplerID)
	e.backend.builder.functions = append(e.backend.builder.functions, sampledImageBuilder.Build(OpSampledImage))

	// Result type is vec4<f32> for sampled images
	resultType := e.backend.emitVec4F32Type()
	resultID := e.backend.builder.AllocID()

	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(sampledImageID)
	builder.AddWord(coordID)

	// Choose opcode based on sample level
	switch level := sample.Level.(type) {
	case ir.SampleLevelAuto:
		// OpImageSampleImplicitLod (no extra operands for basic case)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageSampleImplicitLod))

	case ir.SampleLevelExact:
		// OpImageSampleExplicitLod with Lod operand
		levelID, err := e.emitExpression(level.Level)
		if err != nil {
			return 0, err
		}
		builder.AddWord(0x02) // ImageOperands::Lod
		builder.AddWord(levelID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageSampleExplicitLod))

	case ir.SampleLevelBias:
		// OpImageSampleImplicitLod with Bias operand
		biasID, err := e.emitExpression(level.Bias)
		if err != nil {
			return 0, err
		}
		builder.AddWord(0x01) // ImageOperands::Bias
		builder.AddWord(biasID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageSampleImplicitLod))

	case ir.SampleLevelGradient:
		// OpImageSampleExplicitLod with Grad operand
		gradXID, err := e.emitExpression(level.X)
		if err != nil {
			return 0, err
		}
		gradYID, err := e.emitExpression(level.Y)
		if err != nil {
			return 0, err
		}
		builder.AddWord(0x04) // ImageOperands::Grad
		builder.AddWord(gradXID)
		builder.AddWord(gradYID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageSampleExplicitLod))

	case ir.SampleLevelZero:
		// OpImageSampleExplicitLod with Lod = 0
		zeroID := e.backend.builder.AddConstantFloat32(
			e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}),
			0.0)
		builder.AddWord(0x02) // ImageOperands::Lod
		builder.AddWord(zeroID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageSampleExplicitLod))

	default:
		return 0, fmt.Errorf("unsupported sample level: %T", level)
	}

	return resultID, nil
}

// emitImageLoad emits a texture load operation.
func (e *ExpressionEmitter) emitImageLoad(load ir.ExprImageLoad) (uint32, error) {
	imageID, err := e.emitExpression(load.Image)
	if err != nil {
		return 0, err
	}

	coordID, err := e.emitExpression(load.Coordinate)
	if err != nil {
		return 0, err
	}

	// Result type is vec4<f32>
	resultType := e.backend.emitVec4F32Type()
	resultID := e.backend.builder.AllocID()

	builder := NewInstructionBuilder()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(imageID)
	builder.AddWord(coordID)

	// Add Lod operand if specified
	if load.Level != nil {
		levelID, err := e.emitExpression(*load.Level)
		if err != nil {
			return 0, err
		}
		builder.AddWord(0x02) // ImageOperands::Lod
		builder.AddWord(levelID)
	}

	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageFetch))
	return resultID, nil
}

// emitImageQuery emits an image query operation.
func (e *ExpressionEmitter) emitImageQuery(query ir.ExprImageQuery) (uint32, error) {
	imageID, err := e.emitExpression(query.Image)
	if err != nil {
		return 0, err
	}

	var resultID uint32
	builder := NewInstructionBuilder()

	switch q := query.Query.(type) {
	case ir.ImageQuerySize:
		// Returns uvec2 or uvec3 depending on image dimension
		scalarID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		resultType := e.backend.builder.AddTypeVector(scalarID, uint32(ir.Vec3))
		resultID = e.backend.builder.AllocID()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(imageID)

		if q.Level != nil {
			levelID, err := e.emitExpression(*q.Level)
			if err != nil {
				return 0, err
			}
			builder.AddWord(levelID)
			e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageQuerySizeLod))
		} else {
			e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageQuerySize))
		}

	case ir.ImageQueryNumLevels:
		resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		resultID = e.backend.builder.AllocID()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(imageID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageQueryLevels))

	case ir.ImageQueryNumLayers:
		// NumLayers is part of ImageQuerySize for array textures
		resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		resultID = e.backend.builder.AllocID()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(imageID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageQuerySize))

	case ir.ImageQueryNumSamples:
		resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		resultID = e.backend.builder.AllocID()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(imageID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageQuerySamples))

	default:
		return 0, fmt.Errorf("unsupported image query: %T", q)
	}

	return resultID, nil
}

// getSampledImageType returns the type ID for a sampled image.
func (b *Backend) getSampledImageType(_ ir.ExpressionHandle) uint32 {
	// For now, create a generic sampled image type
	// In a full implementation, we'd look up the actual image type
	imageTypeID := b.emitImageType(
		b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}),
		ir.ImageType{
			Dim:     ir.Dim2D,
			Class:   ir.ImageClassSampled,
			Arrayed: false,
		},
	)

	// OpTypeSampledImage
	resultID := b.builder.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultID)
	builder.AddWord(imageTypeID)
	b.builder.types = append(b.builder.types, builder.Build(OpTypeSampledImage))

	return resultID
}

// emitVec4F32Type returns the type ID for vec4<f32>.
func (b *Backend) emitVec4F32Type() uint32 {
	scalarID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	return b.builder.AddTypeVector(scalarID, 4)
}

// OpTypeSampledImage represents OpTypeSampledImage opcode.
const OpTypeSampledImage OpCode = 27

// emitBarrier emits a barrier statement.
func (e *ExpressionEmitter) emitBarrier(stmt ir.StmtBarrier) error {
	u32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})

	// Execution scope - workgroup for workgroupBarrier
	execScopeID := e.backend.builder.AddConstant(u32TypeID, ScopeWorkgroup)

	// Memory scope - also workgroup
	memScopeID := e.backend.builder.AddConstant(u32TypeID, ScopeWorkgroup)

	// Memory semantics based on barrier flags
	semantics := MemorySemanticsAcquireRelease
	if stmt.Flags&ir.BarrierWorkGroup != 0 {
		semantics |= MemorySemanticsWorkgroupMemory
	}
	if stmt.Flags&ir.BarrierStorage != 0 {
		semantics |= MemorySemanticsUniformMemory
	}
	if stmt.Flags&ir.BarrierTexture != 0 {
		semantics |= MemorySemanticsImageMemory
	}
	semanticsID := e.backend.builder.AddConstant(u32TypeID, semantics)

	// OpControlBarrier Execution Memory Semantics
	builder := NewInstructionBuilder()
	builder.AddWord(execScopeID)
	builder.AddWord(memScopeID)
	builder.AddWord(semanticsID)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpControlBarrier))

	return nil
}

// resolveAtomicScalarKind extracts the scalar kind from an atomic pointer expression.
// Returns ScalarUint as default if the type cannot be resolved.
func (e *ExpressionEmitter) resolveAtomicScalarKind(pointer ir.ExpressionHandle) ir.ScalarKind {
	pointerType, err := ir.ResolveExpressionType(e.backend.module, e.function, pointer)
	if err != nil {
		return ir.ScalarUint
	}

	// Get the inner type from TypeResolution (either from Handle or Value)
	var inner ir.TypeInner
	if pointerType.Handle != nil && int(*pointerType.Handle) < len(e.backend.module.Types) {
		inner = e.backend.module.Types[*pointerType.Handle].Inner
	} else {
		inner = pointerType.Value
	}

	ptrType, ok := inner.(ir.PointerType)
	if !ok || int(ptrType.Base) >= len(e.backend.module.Types) {
		return ir.ScalarUint
	}

	atomicType, ok := e.backend.module.Types[ptrType.Base].Inner.(ir.AtomicType)
	if !ok {
		return ir.ScalarUint
	}

	return atomicType.Scalar.Kind
}

// atomicOpcode returns the SPIR-V opcode for an atomic function.
// Returns 0 if the function is AtomicExchange with a compare value (handled separately).
func atomicOpcode(fun ir.AtomicFunction, scalarKind ir.ScalarKind) (OpCode, bool) {
	switch fun.(type) {
	case ir.AtomicAdd:
		return OpAtomicIAdd, true
	case ir.AtomicSubtract:
		return OpAtomicISub, true
	case ir.AtomicAnd:
		return OpAtomicAnd, true
	case ir.AtomicInclusiveOr:
		return OpAtomicOr, true
	case ir.AtomicExclusiveOr:
		return OpAtomicXor, true
	case ir.AtomicMin:
		if scalarKind == ir.ScalarSint {
			return OpAtomicSMin, true
		}
		return OpAtomicUMin, true
	case ir.AtomicMax:
		if scalarKind == ir.ScalarSint {
			return OpAtomicSMax, true
		}
		return OpAtomicUMax, true
	case ir.AtomicExchange:
		return OpAtomicExchange, true
	default:
		return 0, false
	}
}

// emitAtomic emits an atomic operation statement.
func (e *ExpressionEmitter) emitAtomic(stmt ir.StmtAtomic) error {
	pointerID, err := e.emitExpression(stmt.Pointer)
	if err != nil {
		return err
	}

	valueID, err := e.emitExpression(stmt.Value)
	if err != nil {
		return err
	}

	// Determine scalar kind from pointer type (atomic<i32> vs atomic<u32>)
	scalarKind := e.resolveAtomicScalarKind(stmt.Pointer)
	resultTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: scalarKind, Width: 4})

	// Scope and memory semantics constants
	scopeID := e.backend.builder.AddConstant(
		e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
		ScopeDevice,
	)
	semanticsID := e.backend.builder.AddConstant(
		e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
		MemorySemanticsAcquireRelease|MemorySemanticsUniformMemory,
	)

	// Handle compare-exchange separately
	if exchange, ok := stmt.Fun.(ir.AtomicExchange); ok && exchange.Compare != nil {
		return e.emitAtomicCompareExchange(stmt, pointerID, valueID, resultTypeID, scopeID, semanticsID, *exchange.Compare)
	}

	opcode, ok := atomicOpcode(stmt.Fun, scalarKind)
	if !ok {
		return fmt.Errorf("unsupported atomic function: %T", stmt.Fun)
	}

	// Emit the atomic operation: OpAtomic* ResultType Result Pointer Scope Semantics Value
	resultID := e.backend.builder.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultTypeID)
	builder.AddWord(resultID)
	builder.AddWord(pointerID)
	builder.AddWord(scopeID)
	builder.AddWord(semanticsID)
	builder.AddWord(valueID)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(opcode))

	if stmt.Result != nil {
		e.exprIDs[*stmt.Result] = resultID
	}
	return nil
}

// emitAtomicCompareExchange emits an atomic compare-exchange operation.
func (e *ExpressionEmitter) emitAtomicCompareExchange(
	stmt ir.StmtAtomic,
	pointerID, valueID, resultTypeID, scopeID, semanticsID uint32,
	compare ir.ExpressionHandle,
) error {
	compareID, err := e.emitExpression(compare)
	if err != nil {
		return err
	}

	resultID := e.backend.builder.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultTypeID)
	builder.AddWord(resultID)
	builder.AddWord(pointerID)
	builder.AddWord(scopeID)
	builder.AddWord(semanticsID) // MemSemEqual
	builder.AddWord(semanticsID) // MemSemUnequal (same for simplicity)
	builder.AddWord(valueID)
	builder.AddWord(compareID)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpAtomicCompareExch))

	if stmt.Result != nil {
		e.exprIDs[*stmt.Result] = resultID
	}
	return nil
}
