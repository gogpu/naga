package spirv

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/ir"
)

// entryPointInput describes how an entry point input argument is structured.
// For struct inputs with member bindings, we create separate Input variables
// for each member, then composite construct and store to a local variable.
type entryPointInput struct {
	// isStruct is true if the input is a struct with member bindings
	isStruct bool
	// singleVarID is the input variable ID for non-struct inputs
	singleVarID uint32
	// memberVarIDs maps struct member index to input variable ID
	memberVarIDs map[int]uint32
	// typeID is the SPIR-V type ID of the argument
	typeID uint32
}

// entryPointOutput describes how entry point outputs are structured.
// For struct outputs with member bindings, we create separate output
// variables for each member. For simple outputs, we have a single variable.
type entryPointOutput struct {
	// isStruct is true if the output is a struct with member bindings
	isStruct bool
	// singleVarID is the output variable ID for non-struct outputs
	singleVarID uint32
	// memberVarIDs maps struct member index to output variable ID
	memberVarIDs map[int]uint32
	// resultTypeID is the SPIR-V type ID of the result (for struct decomposition)
	resultTypeID uint32
}

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
	// Key: entry point function handle
	entryInputVars  map[ir.FunctionHandle]map[int]*entryPointInput // arg index → input info
	entryOutputVars map[ir.FunctionHandle]*entryPointOutput        // For function result

	// Cached sampled image type (for texture sampling operations)
	// Key: image dimension + arrayed, Value: SPIR-V type ID
	sampledImageTypeIDs map[uint32]uint32

	// Cached image type (for reuse)
	// Key: image dimension + arrayed, Value: SPIR-V type ID
	imageTypeIDs map[uint32]uint32

	// Cached scalar types (f32, i32, u32, bool, f16, etc.)
	// Key: (kind << 8) | width, Value: SPIR-V type ID
	scalarTypeIDs map[uint32]uint32

	// Cached pointer types
	// Key: (storageClass << 24) | baseTypeID, Value: SPIR-V type ID
	pointerTypeIDs map[uint32]uint32

	// Cached vector types
	// Key: (componentTypeID << 8) | componentCount, Value: SPIR-V type ID
	vectorTypeIDs map[uint32]uint32

	// Track used capabilities (to avoid duplicates)
	usedCapabilities map[Capability]bool

	// Cached void type ID (only one void type allowed in SPIR-V)
	voidTypeID uint32

	// Cached function types (key: concatenated return + param type IDs)
	funcTypeIDs map[string]uint32
}

// NewBackend creates a new SPIR-V backend.
func NewBackend(options Options) *Backend {
	return &Backend{
		options:             options,
		typeIDs:             make(map[ir.TypeHandle]uint32),
		constantIDs:         make(map[ir.ConstantHandle]uint32),
		globalIDs:           make(map[ir.GlobalVariableHandle]uint32),
		functionIDs:         make(map[ir.FunctionHandle]uint32),
		entryInputVars:      make(map[ir.FunctionHandle]map[int]*entryPointInput),
		entryOutputVars:     make(map[ir.FunctionHandle]*entryPointOutput),
		sampledImageTypeIDs: make(map[uint32]uint32),
		imageTypeIDs:        make(map[uint32]uint32),
		scalarTypeIDs:       make(map[uint32]uint32),
		pointerTypeIDs:      make(map[uint32]uint32),
		vectorTypeIDs:       make(map[uint32]uint32),
		usedCapabilities:    make(map[Capability]bool),
		funcTypeIDs:         make(map[string]uint32),
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

	// 8. Types and constants
	// Must be emitted before decorations because decorations need type IDs.
	if err := b.emitTypes(); err != nil {
		return nil, err
	}
	if err := b.emitConstants(); err != nil {
		return nil, err
	}

	// 9. Struct member decorations (offsets)
	// Must be after emitTypes() so typeIDs is populated.
	b.emitStructMemberDecorations()

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
	b.addCapability(CapabilityShader)

	// Add user-requested capabilities
	for _, cap := range b.options.Capabilities {
		b.addCapability(cap)
	}
}

// addCapability adds a capability if not already added.
func (b *Backend) addCapability(cap Capability) {
	if !b.usedCapabilities[cap] {
		b.usedCapabilities[cap] = true
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

// emitStructMemberDecorations adds offset decorations for struct members.
// Must be called after emitTypes() so that typeIDs is populated.
// Note: Global variable decorations (@group, @binding, Block) are added in emitGlobals().
func (b *Backend) emitStructMemberDecorations() {
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
		id = b.emitVectorType(scalarID, uint32(inner.Size))

	case ir.MatrixType:
		scalarID := b.emitScalarType(inner.Scalar)
		columnTypeID := b.emitVectorType(scalarID, uint32(inner.Rows))
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
		id = b.emitPointerType(storageClass, baseID)

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
// Uses cache to ensure type deduplication (SPIR-V requires unique types).
func (b *Backend) emitScalarType(scalar ir.ScalarType) uint32 {
	// Create cache key: (kind << 8) | width
	key := (uint32(scalar.Kind) << 8) | uint32(scalar.Width)

	// Check cache first
	if id, ok := b.scalarTypeIDs[key]; ok {
		return id
	}

	// Create new type
	var id uint32
	switch scalar.Kind {
	case ir.ScalarBool:
		id = b.builder.AddTypeBool()

	case ir.ScalarFloat:
		widthBits := uint32(scalar.Width) * 8 // bytes to bits
		// Add required capabilities for non-32-bit floats
		switch widthBits {
		case 16:
			b.addCapability(CapabilityFloat16)
		case 64:
			b.addCapability(CapabilityFloat64)
		}
		id = b.builder.AddTypeFloat(widthBits)

	case ir.ScalarSint:
		widthBits := uint32(scalar.Width) * 8
		// Add required capabilities for non-32-bit integers
		switch widthBits {
		case 8:
			b.addCapability(CapabilityInt8)
		case 16:
			b.addCapability(CapabilityInt16)
		case 64:
			b.addCapability(CapabilityInt64)
		}
		id = b.builder.AddTypeInt(widthBits, true)

	case ir.ScalarUint:
		widthBits := uint32(scalar.Width) * 8
		// Add required capabilities for non-32-bit integers
		switch widthBits {
		case 8:
			b.addCapability(CapabilityInt8)
		case 16:
			b.addCapability(CapabilityInt16)
		case 64:
			b.addCapability(CapabilityInt64)
		}
		id = b.builder.AddTypeInt(widthBits, false)

	default:
		panic(fmt.Sprintf("unknown scalar kind: %v", scalar.Kind))
	}

	// Cache and return
	b.scalarTypeIDs[key] = id
	return id
}

// getVoidType returns the cached void type ID, creating it if needed.
func (b *Backend) getVoidType() uint32 {
	if b.voidTypeID == 0 {
		b.voidTypeID = b.builder.AddTypeVoid()
	}
	return b.voidTypeID
}

// getFuncType returns a cached function type ID, creating it if needed.
func (b *Backend) getFuncType(returnTypeID uint32, paramTypeIDs []uint32) uint32 {
	// Build cache key from return type and param types
	key := fmt.Sprintf("%d", returnTypeID)
	for _, p := range paramTypeIDs {
		key += fmt.Sprintf("_%d", p)
	}

	if id, ok := b.funcTypeIDs[key]; ok {
		return id
	}

	id := b.builder.AddTypeFunction(returnTypeID, paramTypeIDs...)
	b.funcTypeIDs[key] = id
	return id
}

// emitPointerType emits a pointer type and returns its SPIR-V ID.
// Uses cache to ensure type deduplication.
func (b *Backend) emitPointerType(storageClass StorageClass, baseTypeID uint32) uint32 {
	// Create cache key: (storageClass << 24) | baseTypeID
	// This works for baseTypeID up to 16M which is plenty
	key := (uint32(storageClass) << 24) | baseTypeID

	// Check cache first
	if id, ok := b.pointerTypeIDs[key]; ok {
		return id
	}

	// Create new type
	id := b.builder.AddTypePointer(storageClass, baseTypeID)
	b.pointerTypeIDs[key] = id
	return id
}

// emitVectorType emits a vector type and returns its SPIR-V ID.
// Uses cache to ensure type deduplication.
func (b *Backend) emitVectorType(componentTypeID uint32, componentCount uint32) uint32 {
	// Create cache key: (componentTypeID << 8) | componentCount
	key := (componentTypeID << 8) | componentCount

	// Check cache first
	if id, ok := b.vectorTypeIDs[key]; ok {
		return id
	}

	// Create new type
	id := b.builder.AddTypeVector(componentTypeID, componentCount)
	b.vectorTypeIDs[key] = id
	return id
}

// imageTypeKey creates a cache key for an image type.
func imageTypeKey(img ir.ImageType) uint32 {
	// Pack dimension (3 bits), arrayed (1 bit), multisampled (1 bit), class (3 bits)
	key := uint32(img.Dim) & 0x07
	if img.Arrayed {
		key |= 0x08
	}
	if img.Multisampled {
		key |= 0x10
	}
	key |= (uint32(img.Class) & 0x07) << 5
	return key
}

// emitImageType emits OpTypeImage, with caching to avoid duplicates.
func (b *Backend) emitImageType(sampledTypeID uint32, img ir.ImageType) uint32 {
	// Check cache first
	cacheKey := imageTypeKey(img)
	if id, ok := b.imageTypeIDs[cacheKey]; ok {
		return id
	}

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

	// Cache the type for reuse
	b.imageTypeIDs[cacheKey] = id
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

		// Add Block decoration for struct types in Uniform/Storage/PushConstant address spaces.
		// This is required by Vulkan spec (VUID-StandaloneSpirv-Uniform-06676).
		if b.needsBlockDecoration(global.Space, global.Type) {
			b.builder.AddDecorate(varType, DecorationBlock)
		}

		// Create pointer type for the variable
		storageClass := addressSpaceToStorageClass(global.Space)
		ptrType := b.emitPointerType(storageClass, varType)

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

		// Add decorations for resource bindings (@group, @binding)
		// Must be done here because we now have the varID
		if global.Binding != nil {
			b.builder.AddDecorate(varID, DecorationDescriptorSet, global.Binding.Group)
			b.builder.AddDecorate(varID, DecorationBinding, global.Binding.Binding)
		}
	}
	return nil
}

// needsBlockDecoration returns true if a struct type in the given address space
// needs the Block decoration per Vulkan SPIR-V requirements.
func (b *Backend) needsBlockDecoration(space ir.AddressSpace, typeHandle ir.TypeHandle) bool {
	switch space {
	case ir.SpaceUniform, ir.SpaceStorage, ir.SpacePushConstant:
		typ := &b.module.Types[typeHandle]
		_, isStruct := typ.Inner.(ir.StructType)
		return isStruct
	default:
		return false
	}
}

// emitEntryPointInterfaceVars creates input/output variables for entry point builtins and locations.
// In SPIR-V, entry point functions don't receive builtins as parameters.
// Instead, builtins are global variables with Input/Output storage class.
func (b *Backend) emitEntryPointInterfaceVars() error {
	for _, entryPoint := range b.module.EntryPoints {
		fn := &b.module.Functions[entryPoint.Function]
		inputVars := make(map[int]*entryPointInput)

		// Create input variables for function arguments
		for i, arg := range fn.Arguments {
			argTypeID, err := b.emitType(arg.Type)
			if err != nil {
				return err
			}

			input := &entryPointInput{
				typeID:       argTypeID,
				memberVarIDs: make(map[int]uint32),
			}

			if arg.Binding != nil {
				// Simple case: argument has direct binding (scalar/vector/builtin)
				ptrType := b.emitPointerType(StorageClassInput, argTypeID)
				varID := b.builder.AddVariable(ptrType, StorageClassInput)
				input.singleVarID = varID

				switch binding := (*arg.Binding).(type) {
				case ir.BuiltinBinding:
					spirvBuiltin := builtinToSPIRV(binding.Builtin)
					b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
				case ir.LocationBinding:
					b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
				}

				if b.options.Debug && arg.Name != "" {
					b.builder.AddName(varID, arg.Name)
				}
			} else {
				// Check if argument is a struct with member bindings
				argTypeInner := b.module.Types[arg.Type].Inner
				if structType, ok := argTypeInner.(ir.StructType); ok {
					// Check if any member has a binding
					hasBindings := false
					for _, member := range structType.Members {
						if member.Binding != nil {
							hasBindings = true
							break
						}
					}
					if hasBindings {
						input.isStruct = true
						// Create separate Input variable for each member with a binding
						for j, member := range structType.Members {
							if member.Binding == nil {
								continue
							}
							memberTypeID, err := b.emitType(member.Type)
							if err != nil {
								return err
							}
							ptrType := b.emitPointerType(StorageClassInput, memberTypeID)
							varID := b.builder.AddVariable(ptrType, StorageClassInput)
							input.memberVarIDs[j] = varID

							if b.options.Debug && member.Name != "" {
								b.builder.AddName(varID, member.Name)
							}

							switch binding := (*member.Binding).(type) {
							case ir.BuiltinBinding:
								spirvBuiltin := builtinToSPIRV(binding.Builtin)
								b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
							case ir.LocationBinding:
								b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
							}
						}
					}
				}
				// If not a struct with bindings, input.singleVarID stays 0 (no input var needed)
			}

			inputVars[i] = input
		}

		b.entryInputVars[entryPoint.Function] = inputVars

		// Create output variables for function result
		if fn.Result != nil {
			resultTypeID, err := b.emitType(fn.Result.Type)
			if err != nil {
				return err
			}

			output := &entryPointOutput{
				resultTypeID: resultTypeID,
				memberVarIDs: make(map[int]uint32),
			}

			if fn.Result.Binding != nil {
				// Simple output with a single binding
				ptrType := b.emitPointerType(StorageClassOutput, resultTypeID)
				varID := b.builder.AddVariable(ptrType, StorageClassOutput)
				output.singleVarID = varID

				switch binding := (*fn.Result.Binding).(type) {
				case ir.BuiltinBinding:
					spirvBuiltin := builtinToSPIRV(binding.Builtin)
					b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
				case ir.LocationBinding:
					b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
				}
			} else {
				// Check if it's a struct with member bindings
				resultType := b.module.Types[fn.Result.Type].Inner
				if structType, ok := resultType.(ir.StructType); ok {
					output.isStruct = true
					// Create separate output variable for each member with a binding
					for i, member := range structType.Members {
						if member.Binding == nil {
							continue
						}
						memberTypeID, err := b.emitType(member.Type)
						if err != nil {
							return err
						}
						ptrType := b.emitPointerType(StorageClassOutput, memberTypeID)
						varID := b.builder.AddVariable(ptrType, StorageClassOutput)
						output.memberVarIDs[i] = varID

						// Add debug name if enabled
						if b.options.Debug && member.Name != "" {
							b.builder.AddName(varID, member.Name)
						}

						// Decorate based on binding type
						switch binding := (*member.Binding).(type) {
						case ir.BuiltinBinding:
							spirvBuiltin := builtinToSPIRV(binding.Builtin)
							b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
						case ir.LocationBinding:
							b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
						}
					}
				}
			}

			b.entryOutputVars[entryPoint.Function] = output
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
			for _, input := range inputVars {
				if input.isStruct {
					// Struct input: add all member variables
					for _, varID := range input.memberVarIDs {
						interfaces = append(interfaces, varID)
					}
				} else if input.singleVarID != 0 {
					interfaces = append(interfaces, input.singleVarID)
				}
			}
		}

		// Add entry point output variables
		if output, ok := b.entryOutputVars[entryPoint.Function]; ok {
			if output.isStruct {
				// Struct output: add all member variables
				for _, varID := range output.memberVarIDs {
					interfaces = append(interfaces, varID)
				}
			} else if output.singleVarID != 0 {
				// Single output variable
				interfaces = append(interfaces, output.singleVarID)
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
		returnTypeID = b.getVoidType()
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
			returnTypeID = b.getVoidType()
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
	funcTypeID := b.getFuncType(returnTypeID, paramTypeIDs)

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
		ptrType := b.emitPointerType(StorageClassFunction, varType)

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

	// 2. For entry points, handle input variables
	// For struct inputs with member bindings, we need to:
	// - Create a local variable for the struct (Function storage class)
	// - Load each member from its Input interface variable
	// - Composite construct the struct and store to local variable
	// - Use the local variable pointer as the parameter
	var output *entryPointOutput
	entryPointInputLocals := make(map[int]uint32) // arg index → local var ID
	var entryInputs map[int]*entryPointInput
	if isEntryPoint {
		if inputVars, ok := b.entryInputVars[handle]; ok {
			entryInputs = inputVars
			for i, input := range inputVars {
				if input.isStruct {
					// Struct input with member bindings - create local variable
					ptrType := b.emitPointerType(StorageClassFunction, input.typeID)
					localVarID := b.builder.AllocID()
					builder := NewInstructionBuilder()
					builder.AddWord(ptrType)
					builder.AddWord(localVarID)
					builder.AddWord(uint32(StorageClassFunction))
					b.builder.functions = append(b.builder.functions, builder.Build(OpVariable))

					entryPointInputLocals[i] = localVarID
					paramIDs[i] = localVarID
				} else if input.singleVarID != 0 {
					// Simple input - use directly
					paramIDs[i] = input.singleVarID
				}
			}
		}
		output = b.entryOutputVars[handle]
	}

	// 3. Create expression emitter for this function
	emitter := &ExpressionEmitter{
		backend:      b,
		function:     fn,
		exprIDs:      make(map[ir.ExpressionHandle]uint32),
		paramIDs:     paramIDs,
		isEntryPoint: isEntryPoint,
		output:       output,
	}
	emitter.localVarIDs = localVarIDs

	// 4. Initialize entry point struct inputs (load from Input interface variables)
	// This must happen after OpVariable but before other instructions
	if isEntryPoint && entryInputs != nil {
		for i, localVarID := range entryPointInputLocals {
			input := entryInputs[i]
			arg := fn.Arguments[i]
			argTypeInner := b.module.Types[arg.Type].Inner
			structType := argTypeInner.(ir.StructType)

			// Load each member from its Input interface variable and composite construct
			memberIDs := make([]uint32, len(structType.Members))
			for j, member := range structType.Members {
				memberTypeID, _ := b.emitType(member.Type)
				if memberVarID, ok := input.memberVarIDs[j]; ok {
					// Load from Input interface variable
					memberIDs[j] = b.builder.AddLoad(memberTypeID, memberVarID)
				} else {
					// Member without binding - use zero/default value
					memberIDs[j] = b.builder.AddConstantNull(memberTypeID)
				}
			}

			// Composite construct the struct
			structValue := b.builder.AddCompositeConstruct(input.typeID, memberIDs...)

			// Store to local variable
			b.builder.AddStore(localVarID, structValue)
		}
	}

	// 5. Initialize local variables (now that emitter is available)
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
	isEntryPoint bool              // True if this is an entry point function
	output       *entryPointOutput // Output variable(s) for entry point result

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
		return b.emitVectorType(scalarID, uint32(t.Size))

	case ir.MatrixType:
		scalarID := b.emitScalarType(t.Scalar)
		columnTypeID := b.emitVectorType(scalarID, uint32(t.Rows))
		return b.builder.AddTypeMatrix(columnTypeID, uint32(t.Columns))

	case ir.PointerType:
		// Emit the base type first
		baseID, err := b.emitType(t.Base)
		if err != nil {
			panic(fmt.Sprintf("failed to emit pointer base type: %v", err))
		}
		storageClass := addressSpaceToStorageClass(t.Space)
		return b.emitPointerType(storageClass, baseID)

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
		id, err = e.emitLiteral(kind.Value)
	case ir.ExprConstant:
		return e.emitConstantRef(kind)
	case ir.ExprCompose:
		id, err = e.emitCompose(kind)
	case ir.ExprAccess:
		id, err = e.emitAccess(kind)
	case ir.ExprAccessIndex:
		id, err = e.emitAccessIndex(kind)
	case ir.ExprFunctionArgument:
		// Auto-load: emitExpression returns VALUES, not pointers
		return e.emitFunctionArgValue(kind)
	case ir.ExprGlobalVariable:
		// Auto-load: emitExpression returns VALUES, not pointers
		return e.emitGlobalVarValue(kind)
	case ir.ExprLocalVariable:
		// Auto-load: emitExpression returns VALUES, not pointers
		return e.emitLocalVarValue(kind)
	case ir.ExprLoad:
		id, err = e.emitLoad(kind)
	case ir.ExprUnary:
		id, err = e.emitUnary(kind)
	case ir.ExprBinary:
		id, err = e.emitBinary(kind)
	case ir.ExprSelect:
		id, err = e.emitSelect(kind)
	case ir.ExprMath:
		id, err = e.emitMath(kind)
	case ir.ExprDerivative:
		id, err = e.emitDerivative(kind)
	case ir.ExprImageSample:
		id, err = e.emitImageSample(kind)
	case ir.ExprImageLoad:
		id, err = e.emitImageLoad(kind)
	case ir.ExprImageQuery:
		id, err = e.emitImageQuery(kind)
	case ir.ExprAtomicResult:
		return e.emitAtomicResultRef(handle)
	case ir.ExprSwizzle:
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

// emitConstantRef returns the SPIR-V ID for a module-level constant.
func (e *ExpressionEmitter) emitConstantRef(kind ir.ExprConstant) (uint32, error) {
	id, ok := e.backend.constantIDs[kind.Constant]
	if !ok {
		return 0, fmt.Errorf("constant not found: %v", kind.Constant)
	}
	return id, nil
}

// emitFunctionArgRef returns the SPIR-V ID for a function parameter.
func (e *ExpressionEmitter) emitFunctionArgRef(kind ir.ExprFunctionArgument) (uint32, error) {
	if int(kind.Index) >= len(e.paramIDs) {
		return 0, fmt.Errorf("function argument index out of range: %d", kind.Index)
	}
	return e.paramIDs[kind.Index], nil
}

// emitGlobalVarRef returns the SPIR-V ID for a global variable.
func (e *ExpressionEmitter) emitGlobalVarRef(kind ir.ExprGlobalVariable) (uint32, error) {
	id, ok := e.backend.globalIDs[kind.Variable]
	if !ok {
		return 0, fmt.Errorf("global variable not found: %v", kind.Variable)
	}
	return id, nil
}

// emitLocalVarRef returns the SPIR-V ID (pointer) for a local variable.
// This is used by emitPointerExpression for store destinations.
func (e *ExpressionEmitter) emitLocalVarRef(kind ir.ExprLocalVariable) (uint32, error) {
	if int(kind.Variable) >= len(e.localVarIDs) {
		return 0, fmt.Errorf("local variable index out of range: %d", kind.Variable)
	}
	return e.localVarIDs[kind.Variable], nil
}

// emitLocalVarValue returns the loaded VALUE for a local variable.
// This is used by emitExpression for value contexts.
func (e *ExpressionEmitter) emitLocalVarValue(kind ir.ExprLocalVariable) (uint32, error) {
	ptrID, err := e.emitLocalVarRef(kind)
	if err != nil {
		return 0, err
	}

	// Get the variable type and emit OpLoad
	if int(kind.Variable) >= len(e.function.LocalVars) {
		return 0, fmt.Errorf("local variable index out of range: %d", kind.Variable)
	}
	varType := e.function.LocalVars[kind.Variable].Type
	typeID, err := e.backend.emitType(varType)
	if err != nil {
		return 0, err
	}

	return e.backend.builder.AddLoad(typeID, ptrID), nil
}

// emitGlobalVarValue returns the loaded VALUE for a global variable.
// This is used by emitExpression for value contexts.
func (e *ExpressionEmitter) emitGlobalVarValue(kind ir.ExprGlobalVariable) (uint32, error) {
	ptrID, err := e.emitGlobalVarRef(kind)
	if err != nil {
		return 0, err
	}

	// Get the variable type and emit OpLoad
	gv := e.backend.module.GlobalVariables[kind.Variable]
	typeID, err := e.backend.emitType(gv.Type)
	if err != nil {
		return 0, err
	}

	return e.backend.builder.AddLoad(typeID, ptrID), nil
}

// emitFunctionArgValue returns the loaded VALUE for a function argument.
// This is used by emitExpression for value contexts.
func (e *ExpressionEmitter) emitFunctionArgValue(kind ir.ExprFunctionArgument) (uint32, error) {
	ptrID, err := e.emitFunctionArgRef(kind)
	if err != nil {
		return 0, err
	}

	// Get the argument type and emit OpLoad
	if int(kind.Index) >= len(e.function.Arguments) {
		return 0, fmt.Errorf("function argument index out of range: %d", kind.Index)
	}
	argType := e.function.Arguments[kind.Index].Type
	typeID, err := e.backend.emitType(argType)
	if err != nil {
		return 0, err
	}

	return e.backend.builder.AddLoad(typeID, ptrID), nil
}

// emitAtomicResultRef returns the SPIR-V ID for an atomic result (set by emitAtomic).
func (e *ExpressionEmitter) emitAtomicResultRef(handle ir.ExpressionHandle) (uint32, error) {
	if existingID, ok := e.exprIDs[handle]; ok {
		return existingID, nil
	}
	return 0, fmt.Errorf("atomic result expression not found - emitAtomic should have set it")
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

// getExpressionStorageClass returns the SPIR-V storage class for an expression.
// Returns StorageClassFunction as default for non-pointer expressions.
func (e *ExpressionEmitter) getExpressionStorageClass(handle ir.ExpressionHandle) StorageClass {
	expr := e.function.Expressions[handle]

	switch k := expr.Kind.(type) {
	case ir.ExprLocalVariable:
		return StorageClassFunction
	case ir.ExprGlobalVariable:
		gv := e.backend.module.GlobalVariables[k.Variable]
		return addressSpaceToStorageClass(gv.Space)
	case ir.ExprFunctionArgument:
		// Function arguments with bindings are typically Input
		arg := e.function.Arguments[k.Index]
		if arg.Binding != nil {
			return StorageClassInput
		}
		return StorageClassFunction
	case ir.ExprAccess:
		return e.getExpressionStorageClass(k.Base)
	case ir.ExprAccessIndex:
		return e.getExpressionStorageClass(k.Base)
	}

	return StorageClassFunction
}

// emitAccess emits a dynamic access operation.
// Returns the VALUE at the indexed location (not a pointer).
func (e *ExpressionEmitter) emitAccess(access ir.ExprAccess) (uint32, error) {
	// Get result type from type inference
	baseType, err := ir.ResolveExpressionType(e.backend.module, e.function, access.Base)
	if err != nil {
		return 0, fmt.Errorf("access base type: %w", err)
	}

	// Check if base is a pointer expression
	isPointerBase := e.isPointerExpression(access.Base)

	// Get index value (always need loaded value for OpAccessChain/OpCompositeExtract)
	indexID, err := e.emitExpression(access.Index)
	if err != nil {
		return 0, err
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

	if isPointerBase {
		// Base is a pointer - use emitPointerExpression to get SPIR-V pointer, then OpAccessChain
		baseID, err := e.emitPointerExpression(access.Base)
		if err != nil {
			return 0, err
		}

		// Create pointer type for OpAccessChain result
		storageClass := e.getExpressionStorageClass(access.Base)
		ptrType := e.backend.emitPointerType(storageClass, elementTypeID)

		// OpAccessChain returns a pointer, then we auto-load
		ptrID := e.backend.builder.AddAccessChain(ptrType, baseID, indexID)
		return e.backend.builder.AddLoad(elementTypeID, ptrID), nil
	}

	// Base is already a value - use OpVectorExtractDynamic (for vectors)
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}

	// For dynamic access on values, use OpVectorExtractDynamic
	return e.backend.builder.AddVectorExtractDynamic(elementTypeID, baseID, indexID), nil
}

// isPointerExpression recursively checks if an expression returns a SPIR-V pointer.
// This includes variable references and access chains on pointer bases.
func (e *ExpressionEmitter) isPointerExpression(handle ir.ExpressionHandle) bool {
	expr := e.function.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprLocalVariable, ir.ExprGlobalVariable, ir.ExprFunctionArgument:
		return true
	case ir.ExprAccessIndex:
		return e.isPointerExpression(k.Base)
	case ir.ExprAccess:
		return e.isPointerExpression(k.Base)
	}
	return false
}

// emitPointerExpression emits an expression as a SPIR-V pointer without loading.
// This is used for store destinations and other pointer contexts.
// Returns error if the expression is not a pointer expression.
func (e *ExpressionEmitter) emitPointerExpression(handle ir.ExpressionHandle) (uint32, error) {
	expr := e.function.Expressions[handle]

	switch k := expr.Kind.(type) {
	case ir.ExprLocalVariable:
		return e.emitLocalVarRef(k)
	case ir.ExprGlobalVariable:
		return e.emitGlobalVarRef(k)
	case ir.ExprFunctionArgument:
		return e.emitFunctionArgRef(k)
	case ir.ExprAccessIndex:
		return e.emitAccessIndexAsPointer(k)
	case ir.ExprAccess:
		return e.emitAccessAsPointer(k)
	default:
		return 0, fmt.Errorf("expression %T is not a pointer expression", k)
	}
}

// emitAccessIndexAsPointer emits an access index as a SPIR-V pointer without loading.
func (e *ExpressionEmitter) emitAccessIndexAsPointer(access ir.ExprAccessIndex) (uint32, error) {
	baseID, err := e.emitPointerExpression(access.Base)
	if err != nil {
		return 0, err
	}

	// Get result type from type inference
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
		default:
			return 0, fmt.Errorf("cannot index into type %T", t)
		}
	} else {
		return 0, fmt.Errorf("cannot access index on inline type for pointer")
	}

	elementTypeID := e.backend.resolveTypeResolution(elementType)
	u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	indexID := e.backend.builder.AddConstant(u32Type, access.Index)

	storageClass := e.getExpressionStorageClass(access.Base)
	ptrType := e.backend.emitPointerType(storageClass, elementTypeID)
	return e.backend.builder.AddAccessChain(ptrType, baseID, indexID), nil
}

// emitAccessAsPointer emits a dynamic access as a SPIR-V pointer without loading.
func (e *ExpressionEmitter) emitAccessAsPointer(access ir.ExprAccess) (uint32, error) {
	baseID, err := e.emitPointerExpression(access.Base)
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
		default:
			return 0, fmt.Errorf("cannot index into type %T", t)
		}
	} else {
		return 0, fmt.Errorf("cannot access on inline type for pointer")
	}

	elementTypeID := e.backend.resolveTypeResolution(elementType)
	storageClass := e.getExpressionStorageClass(access.Base)
	ptrType := e.backend.emitPointerType(storageClass, elementTypeID)
	return e.backend.builder.AddAccessChain(ptrType, baseID, indexID), nil
}

// emitAccessIndex emits a static index access operation.
// Returns a VALUE (auto-loads from pointers). For pointer destinations, use emitAccessIndexAsPointer.
func (e *ExpressionEmitter) emitAccessIndex(access ir.ExprAccessIndex) (uint32, error) {
	// Get result type from type inference
	baseType, err := ir.ResolveExpressionType(e.backend.module, e.function, access.Base)
	if err != nil {
		return 0, fmt.Errorf("access index base type: %w", err)
	}

	// Check if base is a pointer expression (variable reference or nested access)
	// If so, use OpAccessChain. Otherwise, use OpCompositeExtract on the loaded value.
	isPointerBase := e.isPointerExpression(access.Base)

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

	if isPointerBase {
		// Base is a pointer - use emitPointerExpression to get SPIR-V pointer,
		// then OpAccessChain, then auto-load to return a VALUE.
		baseID, err := e.emitPointerExpression(access.Base)
		if err != nil {
			return 0, err
		}

		u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		indexID := e.backend.builder.AddConstant(u32Type, access.Index)

		storageClass := e.getExpressionStorageClass(access.Base)
		ptrType := e.backend.emitPointerType(storageClass, elementTypeID)
		ptrID := e.backend.builder.AddAccessChain(ptrType, baseID, indexID)

		// Auto-load the value - emitExpression should return VALUES, not pointers
		return e.backend.builder.AddLoad(elementTypeID, ptrID), nil
	}

	// Base is already a value - use OpCompositeExtract
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}
	return e.backend.builder.AddCompositeExtract(elementTypeID, baseID, access.Index), nil
}

// emitSwizzle emits a vector swizzle operation (.xyz, .rgb, etc.).
// Uses OpVectorShuffle to rearrange vector components.
func (e *ExpressionEmitter) emitSwizzle(swizzle ir.ExprSwizzle) (uint32, error) {
	// emitExpression now auto-loads variable references, so vectorID is already a value
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
// NOTE: ExprLoad is explicit in IR when the compiler wants to explicitly load.
// Since emitExpression now auto-loads variable references, ExprLoad is mainly
// used for compound assignments or explicit dereferences.
func (e *ExpressionEmitter) emitLoad(load ir.ExprLoad) (uint32, error) {
	// Use emitPointerExpression to get the SPIR-V pointer (without auto-loading)
	pointerID, err := e.emitPointerExpression(load.Pointer)
	if err != nil {
		return 0, err
	}

	// Get result type by examining the pointer expression's type
	pointerType, err := ir.ResolveExpressionType(e.backend.module, e.function, load.Pointer)
	if err != nil {
		return 0, fmt.Errorf("load pointer type: %w", err)
	}

	// The IR type of pointer expressions (LocalVariable, GlobalVariable, AccessIndex, Access)
	// is the VALUE type, not a pointer type. So we use it directly.
	resultType := e.backend.resolveTypeResolution(pointerType)
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
			if e.isEntryPoint && e.output != nil {
				// For entry points, store to output variable(s) instead of returning
				if e.output.isStruct {
					// Struct output: extract each member and store to its variable
					for memberIdx, varID := range e.output.memberVarIDs {
						// Get the member type from the result type
						resultType := e.backend.module.Types[e.function.Result.Type].Inner.(ir.StructType)
						memberTypeID, err := e.backend.emitType(resultType.Members[memberIdx].Type)
						if err != nil {
							return err
						}
						// Extract member value using OpCompositeExtract
						memberValue := e.backend.builder.AddCompositeExtract(memberTypeID, valueID, uint32(memberIdx))
						// Store to output variable
						e.backend.builder.AddStore(varID, memberValue)
					}
				} else if e.output.singleVarID != 0 {
					// Single output variable
					e.backend.builder.AddStore(e.output.singleVarID, valueID)
				}
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
		// Use emitPointerExpression for store destination - we need a pointer, not a loaded value
		pointerID, err := e.emitPointerExpression(kind.Pointer)
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
	// Get the image and sampler pointer IDs
	imagePtrID, err := e.emitExpression(sample.Image)
	if err != nil {
		return 0, err
	}

	samplerPtrID, err := e.emitExpression(sample.Sampler)
	if err != nil {
		return 0, err
	}

	coordID, err := e.emitExpression(sample.Coordinate)
	if err != nil {
		return 0, err
	}

	// emitExpression now auto-loads, so imagePtrID and samplerPtrID are already
	// the loaded image/sampler handles, not pointers
	imageID := imagePtrID
	samplerID := samplerPtrID

	// Create SampledImage by combining image and sampler
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
		resultType := e.backend.emitVectorType(scalarID, uint32(ir.Vec3))
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
// Uses caching to ensure the same type is reused for identical image configurations.
func (b *Backend) getSampledImageType(_ ir.ExpressionHandle) uint32 {
	// Create the image type descriptor
	img := ir.ImageType{
		Dim:     ir.Dim2D,
		Class:   ir.ImageClassSampled,
		Arrayed: false,
	}
	cacheKey := imageTypeKey(img)

	// Check if we already have a sampled image type for this configuration
	if sampledID, ok := b.sampledImageTypeIDs[cacheKey]; ok {
		return sampledID
	}

	// Get or create the image type (will be cached by emitImageType)
	imageTypeID := b.emitImageType(
		b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}),
		img,
	)

	// OpTypeSampledImage
	resultID := b.builder.AllocID()
	builder := NewInstructionBuilder()
	builder.AddWord(resultID)
	builder.AddWord(imageTypeID)
	b.builder.types = append(b.builder.types, builder.Build(OpTypeSampledImage))

	// Cache the sampled image type
	b.sampledImageTypeIDs[cacheKey] = resultID

	return resultID
}

// emitVec4F32Type returns the type ID for vec4<f32>.
func (b *Backend) emitVec4F32Type() uint32 {
	scalarID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	return b.emitVectorType(scalarID, 4)
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
