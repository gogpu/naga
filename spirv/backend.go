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
}

// NewBackend creates a new SPIR-V backend.
func NewBackend(options Options) *Backend {
	return &Backend{
		options:     options,
		typeIDs:     make(map[ir.TypeHandle]uint32),
		constantIDs: make(map[ir.ConstantHandle]uint32),
		globalIDs:   make(map[ir.GlobalVariableHandle]uint32),
		functionIDs: make(map[ir.FunctionHandle]uint32),
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
	for handle, function := range b.module.Functions {
		if function.Name != "" {
			if id, ok := b.functionIDs[ir.FunctionHandle(handle)]; ok {
				b.builder.AddName(id, function.Name)
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
		// For now, we collect all global variables with Input/Output storage class
		var interfaces []uint32
		for handle, global := range b.module.GlobalVariables {
			if global.Space == ir.SpaceFunction || global.Space == ir.SpacePrivate {
				continue // Not interface variables
			}
			if varID, ok := b.globalIDs[ir.GlobalVariableHandle(handle)]; ok {
				interfaces = append(interfaces, varID)
			}
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
	for handle, function := range b.module.Functions {
		if err := b.emitFunction(ir.FunctionHandle(handle), &function); err != nil {
			return err
		}
	}
	return nil
}

// emitFunction emits a single function.
func (b *Backend) emitFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	// Determine return type
	var returnTypeID uint32
	if fn.Result != nil {
		var err error
		returnTypeID, err = b.emitType(fn.Result.Type)
		if err != nil {
			return err
		}
	} else {
		// void return type
		returnTypeID = b.builder.AddTypeVoid()
	}

	// Emit parameter types
	paramTypeIDs := make([]uint32, len(fn.Arguments))
	for i, arg := range fn.Arguments {
		var err error
		paramTypeIDs[i], err = b.emitType(arg.Type)
		if err != nil {
			return err
		}
	}

	// Create function type
	funcTypeID := b.builder.AddTypeFunction(returnTypeID, paramTypeIDs...)

	// Emit function declaration
	funcID := b.builder.AddFunction(funcTypeID, returnTypeID, FunctionControlNone)
	b.functionIDs[handle] = funcID

	// Emit function parameters
	paramIDs := make([]uint32, len(fn.Arguments))
	for i, arg := range fn.Arguments {
		paramID := b.builder.AddFunctionParameter(paramTypeIDs[i])
		paramIDs[i] = paramID

		// Add debug name if enabled
		if b.options.Debug && arg.Name != "" {
			b.builder.AddName(paramID, arg.Name)
		}
	}

	// Emit function body
	b.builder.AddLabel() // Entry block

	// Create expression emitter for this function
	emitter := &ExpressionEmitter{
		backend:  b,
		fn:       fn,
		exprIDs:  make(map[ir.ExpressionHandle]uint32),
		paramIDs: paramIDs,
	}

	// Emit local variables
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

		// Initialize if needed
		if localVar.Init != nil {
			initID, err := emitter.emitExpression(*localVar.Init)
			if err != nil {
				return err
			}
			b.builder.AddStore(varID, initID)
		}
	}

	emitter.localVarIDs = localVarIDs

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
	fn          *ir.Function
	exprIDs     map[ir.ExpressionHandle]uint32
	paramIDs    []uint32 // Function parameter IDs
	localVarIDs []uint32 // Local variable IDs

	// Loop context stack for break/continue
	loopStack []loopContext
}

// loopContext tracks merge and continue labels for loop statements.
type loopContext struct {
	mergeLabel    uint32 // Label to branch to on break
	continueLabel uint32 // Label to branch to on continue
}

// emitExpression emits an expression and returns its SPIR-V ID.
//
//nolint:gocyclo,cyclop // Expression dispatch requires high cyclomatic complexity
func (e *ExpressionEmitter) emitExpression(handle ir.ExpressionHandle) (uint32, error) {
	// Check cache
	if id, ok := e.exprIDs[handle]; ok {
		return id, nil
	}

	expr := &e.fn.Expressions[handle]
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
func (e *ExpressionEmitter) emitAccess(access ir.ExprAccess) (uint32, error) {
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}

	indexID, err := e.emitExpression(access.Index)
	if err != nil {
		return 0, err
	}

	// Determine result type (pointer to element type)
	// This is simplified - full implementation would need type inference
	// For now, assume base is a pointer to array/vector
	resultType := e.backend.builder.AllocID() // Placeholder
	return e.backend.builder.AddAccessChain(resultType, baseID, indexID), nil
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

	// Determine result type (pointer to element type)
	resultType := e.backend.builder.AllocID() // Placeholder
	return e.backend.builder.AddAccessChain(resultType, baseID, indexID), nil
}

// emitLoad emits a load operation.
func (e *ExpressionEmitter) emitLoad(load ir.ExprLoad) (uint32, error) {
	pointerID, err := e.emitExpression(load.Pointer)
	if err != nil {
		return 0, err
	}

	// Determine result type by looking at pointer type
	// This is simplified - full implementation would need type inference
	resultType := e.backend.builder.AllocID() // Placeholder
	return e.backend.builder.AddLoad(resultType, pointerID), nil
}

// emitUnary emits a unary operation.
func (e *ExpressionEmitter) emitUnary(unary ir.ExprUnary) (uint32, error) {
	operandID, err := e.emitExpression(unary.Expr)
	if err != nil {
		return 0, err
	}

	// Determine result type (same as operand type)
	// This is simplified - full implementation would need type inference
	resultType := e.backend.builder.AllocID() // Placeholder

	var opcode OpCode
	switch unary.Op {
	case ir.UnaryNegate:
		// Need to determine if float or int based on type
		// For now, assume float
		opcode = OpFNegate
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
//nolint:gocyclo,cyclop // Binary operator dispatch requires high cyclomatic complexity
func (e *ExpressionEmitter) emitBinary(binary ir.ExprBinary) (uint32, error) {
	leftID, err := e.emitExpression(binary.Left)
	if err != nil {
		return 0, err
	}

	rightID, err := e.emitExpression(binary.Right)
	if err != nil {
		return 0, err
	}

	// Determine result type
	// This is simplified - full implementation would need type inference
	resultType := e.backend.builder.AllocID() // Placeholder

	// Map IR operator to SPIR-V opcode
	// This is simplified - real implementation needs to check operand types
	var opcode OpCode
	switch binary.Op {
	case ir.BinaryAdd:
		opcode = OpFAdd // Assume float for now
	case ir.BinarySubtract:
		opcode = OpFSub
	case ir.BinaryMultiply:
		opcode = OpFMul
	case ir.BinaryDivide:
		opcode = OpFDiv
	case ir.BinaryModulo:
		opcode = OpFMod
	case ir.BinaryEqual:
		opcode = OpFOrdEqual
	case ir.BinaryNotEqual:
		opcode = OpFOrdNotEqual
	case ir.BinaryLess:
		opcode = OpFOrdLessThan
	case ir.BinaryLessEqual:
		opcode = OpFOrdLessThanEqual
	case ir.BinaryGreater:
		opcode = OpFOrdGreaterThan
	case ir.BinaryGreaterEqual:
		opcode = OpFOrdGreaterThanEqual
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
		opcode = OpShiftRightLogical // Assume unsigned for now
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

	// Determine result type (same as accept/reject)
	resultType := e.backend.builder.AllocID() // Placeholder

	return e.backend.builder.AddSelect(resultType, conditionID, acceptID, rejectID), nil
}

// emitStatement emits a statement.
//
//nolint:cyclop // Statement dispatch requires high cyclomatic complexity
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
			e.backend.builder.AddReturnValue(valueID)
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
//nolint:gocyclo,cyclop,funlen // Math function dispatch requires high cyclomatic complexity and many statements
func (e *ExpressionEmitter) emitMath(mathExpr ir.ExprMath) (uint32, error) {
	// Emit first argument
	argID, err := e.emitExpression(mathExpr.Arg)
	if err != nil {
		return 0, err
	}

	// Determine result type (placeholder - should infer from arguments)
	resultType := e.backend.builder.AllocID()

	// Map IR MathFunction to GLSL.std.450 instruction
	var glslInst uint32
	var useNativeOpcode bool
	var nativeOpcode OpCode

	switch mathExpr.Fun {
	// Comparison functions
	case ir.MathAbs:
		glslInst = GLSLstd450FAbs // TODO: Check if float or int
	case ir.MathMin:
		glslInst = GLSLstd450FMin // TODO: Check if float or int
	case ir.MathMax:
		glslInst = GLSLstd450FMax // TODO: Check if float or int
	case ir.MathClamp:
		glslInst = GLSLstd450FClamp // TODO: Check if float or int
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
		glslInst = GLSLstd450FSign // TODO: Check if float or int
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

	// Determine result type (placeholder - should infer from expression)
	resultType := e.backend.builder.AllocID()

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
