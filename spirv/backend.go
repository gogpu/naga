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

	// Storage buffer variables that were wrapped in a struct for Vulkan compliance.
	// When accessing these variables, an extra index 0 must be prepended to
	// OpAccessChain to go through the wrapper struct to the actual data.
	wrappedStorageVars map[ir.GlobalVariableHandle]bool

	// Shared instruction builder reused across emit methods that don't call
	// other emit functions between Reset() and Build().
	ib InstructionBuilder
}

// NewBackend creates a new SPIR-V backend.
func NewBackend(options Options) *Backend {
	return &Backend{
		options:             options,
		typeIDs:             make(map[ir.TypeHandle]uint32, 16),
		constantIDs:         make(map[ir.ConstantHandle]uint32, 16),
		globalIDs:           make(map[ir.GlobalVariableHandle]uint32, 4),
		functionIDs:         make(map[ir.FunctionHandle]uint32, 4),
		entryInputVars:      make(map[ir.FunctionHandle]map[int]*entryPointInput, 2),
		entryOutputVars:     make(map[ir.FunctionHandle]*entryPointOutput, 2),
		sampledImageTypeIDs: make(map[uint32]uint32, 4),
		imageTypeIDs:        make(map[uint32]uint32, 4),
		scalarTypeIDs:       make(map[uint32]uint32, 8),
		pointerTypeIDs:      make(map[uint32]uint32, 8),
		vectorTypeIDs:       make(map[uint32]uint32, 8),
		usedCapabilities:    make(map[Capability]bool, 4),
		funcTypeIDs:         make(map[string]uint32, 4),
		wrappedStorageVars:  make(map[ir.GlobalVariableHandle]bool, 2),
	}
}

// newIB creates a new InstructionBuilder that allocates from the module's arena.
// Use this for cases where the builder must survive across calls to other emit
// methods (e.g. when emitExpression is called between AddWord and Build).
// For simple cases (build immediately after adding words), use b.ib instead.
func (b *Backend) newIB() *InstructionBuilder {
	return &InstructionBuilder{
		words: make([]uint32, 0, 8),
		arena: &b.builder.arena,
	}
}

// Compile translates an IR module to SPIR-V binary.
func (b *Backend) Compile(module *ir.Module) ([]byte, error) {
	b.module = module
	b.builder = NewModuleBuilder(b.options.Version)
	// Initialize shared instruction builder with module builder's arena for zero-alloc builds.
	b.ib = InstructionBuilder{
		words: make([]uint32, 0, 16),
		arena: &b.builder.arena,
	}

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
func (b *Backend) addCapability(capability Capability) {
	if !b.usedCapabilities[capability] {
		b.usedCapabilities[capability] = true
		b.builder.AddCapability(capability)
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

			// Matrix members in uniform blocks require ColMajor + MatrixStride decorations.
			memberInner := b.module.Types[member.Type].Inner
			if mat, ok := memberInner.(ir.MatrixType); ok {
				b.builder.AddMemberDecorate(structID, uint32(memberIndex), DecorationColMajor)
				stride := uint32(mat.Rows) * uint32(mat.Scalar.Width)
				b.builder.AddMemberDecorate(structID, uint32(memberIndex), DecorationMatrixStride, stride)
			}

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

		if inner.Size.Constant != nil {
			// Fixed-size array
			u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			sizeID := b.builder.AddConstant(u32TypeID, *inner.Size.Constant)
			id = b.builder.AddTypeArray(baseID, sizeID)
		} else {
			// Runtime-sized array (storage buffers)
			id = b.builder.AddTypeRuntimeArray(baseID)
		}

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
		builder := b.newIB()
		builder.AddWord(id)
		b.builder.types = append(b.builder.types, builder.Build(OpTypeSampler))

	case ir.ImageType:
		// Derive sampled type from image class and storage format
		sampledScalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		if inner.Class == ir.ImageClassStorage {
			sampledScalar = storageFormatToScalar(inner.StorageFormat)
		}
		sampledTypeID := b.emitScalarType(sampledScalar)
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
	builder := b.newIB()
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
	var imageFormat ImageFormat
	if img.Class == ir.ImageClassStorage {
		imageFormat = StorageFormatToImageFormat(img.StorageFormat)
	}
	builder.AddWord(uint32(imageFormat))

	b.builder.types = append(b.builder.types, builder.Build(OpTypeImage))

	// Cache the type for reuse
	b.imageTypeIDs[cacheKey] = id
	return id
}

// storageFormatToScalar maps a storage texture format to its sampled scalar type.
// Float formats → f32, unsigned int formats → u32, signed int formats → i32.
func storageFormatToScalar(format ir.StorageFormat) ir.ScalarType {
	switch format {
	case ir.StorageFormatR8Uint, ir.StorageFormatRg8Uint, ir.StorageFormatR16Uint,
		ir.StorageFormatRg16Uint, ir.StorageFormatR32Uint, ir.StorageFormatRg32Uint,
		ir.StorageFormatRgba8Uint, ir.StorageFormatRgba16Uint, ir.StorageFormatRgba32Uint,
		ir.StorageFormatRgb10a2Uint:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	case ir.StorageFormatR8Sint, ir.StorageFormatRg8Sint, ir.StorageFormatR16Sint,
		ir.StorageFormatRg16Sint, ir.StorageFormatR32Sint, ir.StorageFormatRg32Sint,
		ir.StorageFormatRgba8Sint, ir.StorageFormatRgba16Sint, ir.StorageFormatRgba32Sint:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	default:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
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
			builder := b.newIB()
			builder.AddWord(typeID)
			builder.AddWord(id)
			b.builder.types = append(b.builder.types, builder.Build(OpConstantTrue))
			return id
		}
		// OpConstantFalse
		id := b.builder.AllocID()
		builder := b.newIB()
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

// OpArrayLength gets the length of a runtime-sized array in a storage buffer struct.
// Result type must be u32. Operands: struct pointer, member index.
const OpArrayLength OpCode = 68

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

		// Vulkan VUID-StandaloneSpirv-Uniform-06807: StorageBuffer variables must be
		// typed as OpTypeStruct (with Block decoration). If the variable type is NOT a
		// struct (e.g., a runtime array like array<u32>), wrap it in a struct.
		if global.Space == ir.SpaceStorage && !b.isStructType(global.Type) {
			wrapperStruct := b.builder.AddTypeStruct(varType)
			b.builder.AddDecorate(wrapperStruct, DecorationBlock)
			b.builder.AddMemberDecorate(wrapperStruct, 0, DecorationOffset, 0)
			varType = wrapperStruct
			b.wrappedStorageVars[ir.GlobalVariableHandle(handle)] = true
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
		return b.isStructType(typeHandle)
	default:
		return false
	}
}

// isStructType returns true if the type at the given handle is a struct.
func (b *Backend) isStructType(typeHandle ir.TypeHandle) bool {
	typ := &b.module.Types[typeHandle]
	_, isStruct := typ.Inner.(ir.StructType)
	return isStruct
}

// emitEntryPointInterfaceVars creates input/output variables for entry point builtins and locations.
// In SPIR-V, entry point functions don't receive builtins as parameters.
// Instead, builtins are global variables with Input/Output storage class.
//
//nolint:gocognit,nestif,gocyclo,cyclop,funlen // SPIR-V entry points require complex logic
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
					spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassInput)
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
								spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassInput)
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
					spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassOutput)
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
							spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassOutput)
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
// The storageClass parameter is needed because some builtins map to different
// SPIR-V BuiltIn values depending on whether they are inputs or outputs.
// In particular, ir.BuiltinPosition maps to:
//   - BuiltInPosition (SPIR-V 0) when used as a vertex shader output (StorageClassOutput)
//   - BuiltInFragCoord (SPIR-V 15) when used as a fragment shader input (StorageClassInput)
func builtinToSPIRV(builtin ir.BuiltinValue, storageClass StorageClass) BuiltIn {
	switch builtin {
	case ir.BuiltinPosition:
		if storageClass == StorageClassOutput {
			return BuiltInPosition
		}
		return BuiltInFragCoord
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
//
//nolint:gocognit,gocyclo,cyclop // SPIR-V entry points have many cases
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

		// Add regular global variables — for SPIR-V 1.3, only Input/Output
		// storage classes are allowed in the entry point interface.
		// Uniform and StorageBuffer variables are NOT listed.
		for handle, global := range b.module.GlobalVariables {
			if global.Space != ir.SpaceFunction && global.Space != ir.SpacePrivate &&
				global.Space != ir.SpaceUniform && global.Space != ir.SpaceStorage &&
				global.Space != ir.SpaceHandle {
				if varID, ok := b.globalIDs[ir.GlobalVariableHandle(handle)]; ok {
					interfaces = append(interfaces, varID)
				}
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
//nolint:gocognit,gocyclo,cyclop,nestif,funlen // SPIR-V generation has inherent complexity from spec requirements
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
		builder := b.newIB()
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
					builder := b.newIB()
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
		backend:               b,
		function:              fn,
		exprIDs:               make(map[ir.ExpressionHandle]uint32, len(fn.Expressions)),
		paramIDs:              paramIDs,
		isEntryPoint:          isEntryPoint,
		output:                output,
		callResultIDs:         make(map[ir.ExpressionHandle]uint32, 4),
		deferredCallStores:    make(map[ir.ExpressionHandle]uint32, 4),
		deferredComplexStores: make(map[ir.ExpressionHandle][]deferredComplexStore),
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

	// 5. Initialize local variables (now that emitter is available).
	// OpVariable (emitted in prologue) doesn't include the initializer value,
	// so we need explicit OpStore instructions.
	//
	// Variables whose init expression tree contains an ExprCallResult anywhere
	// (directly or inside Binary/Unary/etc.) must be deferred: the StmtCall in
	// the body must run first to produce the result. After the body is emitted,
	// we evaluate the deferred inits and store them.
	for i, localVar := range fn.LocalVars {
		if localVar.Init == nil {
			continue
		}
		initExpr := fn.Expressions[*localVar.Init]
		if _, isCallResult := initExpr.Kind.(ir.ExprCallResult); isCallResult {
			// Direct call result: register deferred store for emitCall
			emitter.deferredCallStores[*localVar.Init] = localVarIDs[i]
			continue
		}
		if callResultHandle, ok := findCallResultInTree(fn.Expressions, *localVar.Init); ok {
			// Init contains a call result somewhere in the tree — defer
			// until the corresponding StmtCall runs.
			emitter.deferredComplexStores[callResultHandle] = append(
				emitter.deferredComplexStores[callResultHandle],
				deferredComplexStore{varPtrID: localVarIDs[i], initExpr: *localVar.Init},
			)
			continue
		}
		initID, err := emitter.emitExpression(*localVar.Init)
		if err != nil {
			return fmt.Errorf("local var %q init: %w", localVar.Name, err)
		}
		b.builder.AddStore(localVarIDs[i], initID)
	}

	// Emit function body statements
	for _, stmt := range fn.Body {
		if err := emitter.emitStatement(stmt); err != nil {
			return err
		}
	}

	// Add OpReturn if the function body doesn't end with a terminator.
	// Every SPIR-V basic block must end with a terminator instruction.
	if !blockEndsWithTerminator(fn.Body) {
		if fn.Result != nil {
			// Non-void function without explicit return — should not happen in valid WGSL
			return fmt.Errorf("non-void function missing return statement")
		}
		b.builder.AddReturn()
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

	// Cached call result IDs (set by emitCall, read by ExprCallResult)
	callResultIDs map[ir.ExpressionHandle]uint32

	// Deferred local variable stores for call results.
	// Maps ExprCallResult handle to local variable SPIR-V pointer ID.
	// When emitCall produces a result, it stores into the mapped variable.
	deferredCallStores map[ir.ExpressionHandle]uint32

	// Deferred complex stores: init expressions that contain a CallResult.
	// Maps ExprCallResult handle to a list of (varPtrID, initExprHandle) pairs.
	// After emitCall caches the result, these are evaluated and stored.
	deferredComplexStores map[ir.ExpressionHandle][]deferredComplexStore
}

// deferredComplexStore represents a local variable whose init expression
// contains an ExprCallResult somewhere in its expression tree.
type deferredComplexStore struct {
	varPtrID uint32
	initExpr ir.ExpressionHandle
}

// loopContext tracks merge and continue labels for loop statements.
type loopContext struct {
	mergeLabel    uint32 // Label to branch to on break
	continueLabel uint32 // Label to branch to on continue
}

// typeResolutionInner extracts the ir.TypeInner from a TypeResolution.
// Returns the concrete type (ScalarType, VectorType, MatrixType, etc.)
// regardless of whether the resolution uses a Handle or inline Value.
func typeResolutionInner(module *ir.Module, res ir.TypeResolution) ir.TypeInner {
	if res.Handle != nil {
		return module.Types[*res.Handle].Inner
	}
	return res.Value
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

// newIB creates a new InstructionBuilder that allocates from the module's arena.
func (e *ExpressionEmitter) newIB() *InstructionBuilder {
	return e.backend.newIB()
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
	case ir.ExprCallResult:
		return e.emitCallResultRef(handle)
	case ir.ExprSwizzle:
		id, err = e.emitSwizzle(kind)
	case ir.ExprAs:
		id, err = e.emitAs(kind)
	case ir.ExprArrayLength:
		id, err = e.emitArrayLength(kind)
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
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
		if v {
			// OpConstantTrue
			resultID := e.backend.builder.AllocID()
			builder := e.newIB()
			builder.AddWord(typeID)
			builder.AddWord(resultID)
			e.backend.builder.types = append(e.backend.builder.types, builder.Build(OpConstantTrue))
			return resultID, nil
		}
		// OpConstantFalse
		resultID := e.backend.builder.AllocID()
		builder := e.newIB()
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

// emitFunctionArgValue returns the VALUE of a function argument.
// In SPIR-V, OpFunctionParameter for scalar/vector types produces a value
// directly (not a pointer), so no OpLoad is needed for regular functions.
// For entry point functions, arguments are backed by Input/Function variables
// (pointers), so we must OpLoad to get the value.
func (e *ExpressionEmitter) emitFunctionArgValue(kind ir.ExprFunctionArgument) (uint32, error) {
	ptrID, err := e.emitFunctionArgRef(kind)
	if err != nil {
		return 0, err
	}

	if !e.isEntryPoint {
		// Regular function: OpFunctionParameter produces a value, not a pointer
		return ptrID, nil
	}

	// Entry point: paramIDs holds Input/Function variable pointers — need OpLoad
	if int(kind.Index) >= len(e.function.Arguments) {
		return 0, fmt.Errorf("function argument index out of range: %d", kind.Index)
	}
	arg := e.function.Arguments[kind.Index]
	typeID, err := e.backend.emitType(arg.Type)
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
	case ir.ExprLocalVariable, ir.ExprGlobalVariable:
		return true
	case ir.ExprFunctionArgument:
		// For regular functions, OpFunctionParameter produces values, not pointers.
		// For entry points, paramIDs holds Input/Function variable pointers.
		return e.isEntryPoint
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
		varID, err := e.emitGlobalVarRef(k)
		if err != nil {
			return 0, err
		}
		// If this storage buffer variable was wrapped in a struct,
		// emit AccessChain to member 0 to get the actual data pointer.
		if e.backend.wrappedStorageVars[k.Variable] {
			gv := e.backend.module.GlobalVariables[k.Variable]
			innerTypeID, err := e.backend.emitType(gv.Type)
			if err != nil {
				return 0, err
			}
			sc := addressSpaceToStorageClass(gv.Space)
			ptrType := e.backend.emitPointerType(sc, innerTypeID)
			u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			const0 := e.backend.builder.AddConstant(u32Type, 0)
			return e.backend.builder.AddAccessChain(ptrType, varID, const0), nil
		}
		return varID, nil
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

// emitAs emits a type conversion or bitcast.
func (e *ExpressionEmitter) emitAs(as ir.ExprAs) (uint32, error) {
	exprID, err := e.emitExpression(as.Expr)
	if err != nil {
		return 0, err
	}

	// Resolve source expression type to get source scalar kind
	srcType, err := ir.ResolveExpressionType(e.backend.module, e.function, as.Expr)
	if err != nil {
		return 0, fmt.Errorf("as source type: %w", err)
	}

	var srcScalar ir.ScalarType
	var srcInner ir.TypeInner
	if srcType.Handle != nil {
		srcInner = e.backend.module.Types[*srcType.Handle].Inner
	} else {
		srcInner = srcType.Value
	}
	switch t := srcInner.(type) {
	case ir.ScalarType:
		srcScalar = t
	case ir.VectorType:
		srcScalar = t.Scalar
	default:
		return 0, fmt.Errorf("as: unsupported source type %T", srcInner)
	}

	if as.Convert == nil {
		// Bitcast
		targetType := e.backend.emitScalarType(ir.ScalarType{Kind: as.Kind, Width: srcScalar.Width})
		return e.emitConversionOp(OpBitcast, targetType, exprID), nil
	}

	targetScalar := ir.ScalarType{Kind: as.Kind, Width: *as.Convert}
	var targetTypeID uint32
	if vec, ok := srcInner.(ir.VectorType); ok {
		targetTypeID = e.backend.emitInlineType(ir.VectorType{Size: vec.Size, Scalar: targetScalar})
	} else {
		targetTypeID = e.backend.emitScalarType(targetScalar)
	}

	// Bool conversions require special handling — no single SPIR-V opcode exists.
	if srcScalar.Kind == ir.ScalarBool {
		return e.emitBoolToNumeric(targetTypeID, targetScalar, srcInner, exprID)
	}
	if as.Kind == ir.ScalarBool {
		return e.emitNumericToBool(targetTypeID, srcScalar, srcInner, exprID)
	}

	// Select conversion opcode based on source → target scalar kinds
	op, err := selectConversionOp(srcScalar.Kind, as.Kind)
	if err != nil {
		return 0, err
	}

	return e.emitConversionOp(op, targetTypeID, exprID), nil
}

// emitBoolToNumeric converts a bool (or bool vector) to a numeric type using OpSelect.
// SPIR-V has no direct bool→numeric conversion opcode.
// bool → float:  OpSelect(floatType, boolVal, 1.0, 0.0)
// bool → int:    OpSelect(intType,   boolVal, 1,   0)
func (e *ExpressionEmitter) emitBoolToNumeric(targetTypeID uint32, targetScalar ir.ScalarType, srcInner ir.TypeInner, exprID uint32) (uint32, error) {
	// Create scalar constants for one and zero
	scalarTypeID := e.backend.emitScalarType(targetScalar)
	var oneID, zeroID uint32
	switch targetScalar.Kind {
	case ir.ScalarFloat:
		oneID = e.backend.builder.AddConstantFloat32(scalarTypeID, 1.0)
		zeroID = e.backend.builder.AddConstantFloat32(scalarTypeID, 0.0)
	case ir.ScalarUint, ir.ScalarSint:
		oneID = e.backend.builder.AddConstant(scalarTypeID, 1)
		zeroID = e.backend.builder.AddConstant(scalarTypeID, 0)
	default:
		return 0, fmt.Errorf("bool conversion: unsupported target kind %v", targetScalar.Kind)
	}

	// For vector types, replicate constants into composite vectors
	if vec, ok := srcInner.(ir.VectorType); ok {
		size := int(vec.Size)
		oneComponents := make([]uint32, size)
		zeroComponents := make([]uint32, size)
		for i := range size {
			oneComponents[i] = oneID
			zeroComponents[i] = zeroID
		}
		vecTypeID := e.backend.emitInlineType(ir.VectorType{Size: vec.Size, Scalar: targetScalar})
		oneID = e.backend.builder.AddConstantComposite(vecTypeID, oneComponents...)
		zeroID = e.backend.builder.AddConstantComposite(vecTypeID, zeroComponents...)
		return e.backend.builder.AddSelect(vecTypeID, exprID, oneID, zeroID), nil
	}

	return e.backend.builder.AddSelect(targetTypeID, exprID, oneID, zeroID), nil
}

// emitNumericToBool converts a numeric (or numeric vector) to bool using comparison with zero.
// SPIR-V has no direct numeric→bool conversion opcode.
// float → bool:  OpFOrdNotEqual(boolType, val, 0.0)
// int   → bool:  OpINotEqual(boolType, val, 0)
func (e *ExpressionEmitter) emitNumericToBool(targetTypeID uint32, srcScalar ir.ScalarType, srcInner ir.TypeInner, exprID uint32) (uint32, error) {
	// Create zero constant matching source type
	srcScalarTypeID := e.backend.emitScalarType(srcScalar)
	var zeroID uint32
	var cmpOp OpCode
	switch srcScalar.Kind {
	case ir.ScalarFloat:
		zeroID = e.backend.builder.AddConstantFloat32(srcScalarTypeID, 0.0)
		cmpOp = OpFOrdNotEqual
	case ir.ScalarUint, ir.ScalarSint:
		zeroID = e.backend.builder.AddConstant(srcScalarTypeID, 0)
		cmpOp = OpINotEqual
	default:
		return 0, fmt.Errorf("bool conversion: unsupported source kind %v", srcScalar.Kind)
	}

	// For vector types, replicate zero constant into composite vector
	if vec, ok := srcInner.(ir.VectorType); ok {
		size := int(vec.Size)
		zeroComponents := make([]uint32, size)
		for i := range size {
			zeroComponents[i] = zeroID
		}
		srcVecTypeID := e.backend.emitInlineType(ir.VectorType{Size: vec.Size, Scalar: srcScalar})
		zeroID = e.backend.builder.AddConstantComposite(srcVecTypeID, zeroComponents...)
	}

	// Emit comparison: value != 0
	resultID := e.backend.builder.AllocID()
	builder := e.newIB()
	builder.AddWord(targetTypeID)
	builder.AddWord(resultID)
	builder.AddWord(exprID)
	builder.AddWord(zeroID)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(cmpOp))
	return resultID, nil
}

// selectConversionOp returns the SPIR-V opcode for a scalar type conversion.
func selectConversionOp(src, dst ir.ScalarKind) (OpCode, error) {
	switch {
	case src == ir.ScalarUint && dst == ir.ScalarFloat:
		return OpConvertUToF, nil
	case src == ir.ScalarSint && dst == ir.ScalarFloat:
		return OpConvertSToF, nil
	case src == ir.ScalarFloat && dst == ir.ScalarUint:
		return OpConvertFToU, nil
	case src == ir.ScalarFloat && dst == ir.ScalarSint:
		return OpConvertFToS, nil
	case (src == ir.ScalarUint && dst == ir.ScalarSint) || (src == ir.ScalarSint && dst == ir.ScalarUint):
		return OpBitcast, nil
	case src == dst:
		// Same kind, different width (e.g. f32→f64) — bitcast for now
		return OpBitcast, nil
	default:
		return 0, fmt.Errorf("unsupported conversion: %v → %v", src, dst)
	}
}

// emitConversionOp emits a unary conversion instruction.
func (e *ExpressionEmitter) emitConversionOp(op OpCode, resultType, operand uint32) uint32 {
	resultID := e.backend.builder.AllocID()
	builder := e.newIB()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(operand)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(op))
	return resultID
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
		case ir.MatrixType:
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
		case ir.MatrixType:
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
				resultType = e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
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
				resultType = e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
			}
		}
	case ir.BinaryLogicalAnd, ir.BinaryLogicalOr:
		// Logical operators return bool
		resultType = e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
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
			// vec + scalar or scalar + vec: splat scalar to matching vector
			rightType, rErr := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			if rErr == nil {
				leftID, rightID, resultType = e.promoteScalarToVector(leftType, rightType, leftID, rightID, resultType)
			}
		} else {
			opcode = OpIAdd
		}
	case ir.BinarySubtract:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFSub
			// vec - scalar or scalar - vec: splat scalar to matching vector
			rightType, rErr := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			if rErr == nil {
				leftID, rightID, resultType = e.promoteScalarToVector(leftType, rightType, leftID, rightID, resultType)
			}
		} else {
			opcode = OpISub
		}
	case ir.BinaryMultiply:
		if scalarKind == ir.ScalarFloat {
			// Check for special multiplication cases (vector-scalar, matrix-vector, etc.)
			// that require dedicated SPIR-V opcodes.
			rightType, rightErr := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			if rightErr != nil {
				return 0, fmt.Errorf("binary right type: %w", rightErr)
			}
			leftInner := typeResolutionInner(e.backend.module, leftType)
			rightInner := typeResolutionInner(e.backend.module, rightType)
			_, leftIsVec := leftInner.(ir.VectorType)
			_, rightIsVec := rightInner.(ir.VectorType)
			_, leftIsScalar := leftInner.(ir.ScalarType)
			_, rightIsScalar := rightInner.(ir.ScalarType)
			leftMat, leftIsMat := leftInner.(ir.MatrixType)
			_, rightIsMat := rightInner.(ir.MatrixType)

			switch {
			case leftIsMat && rightIsVec:
				// mat * vec -> OpMatrixTimesVector
				// Result type is vec<Rows> (number of rows in the matrix).
				vecScalarID := e.backend.emitScalarType(leftMat.Scalar)
				vecTypeID := e.backend.emitVectorType(vecScalarID, uint32(leftMat.Rows))
				return e.backend.builder.AddBinaryOp(OpMatrixTimesVector, vecTypeID, leftID, rightID), nil
			case leftIsVec && rightIsMat:
				// vec * mat -> OpVectorTimesMatrix
				// Result type is the vector type matching matrix columns.
				vecResultType := e.backend.resolveTypeResolution(leftType)
				return e.backend.builder.AddBinaryOp(OpVectorTimesMatrix, vecResultType, leftID, rightID), nil
			case leftIsMat && rightIsMat:
				// mat * mat -> OpMatrixTimesMatrix
				return e.backend.builder.AddBinaryOp(OpMatrixTimesMatrix, resultType, leftID, rightID), nil
			case leftIsMat && rightIsScalar:
				// mat * scalar -> OpMatrixTimesScalar
				return e.backend.builder.AddBinaryOp(OpMatrixTimesScalar, resultType, leftID, rightID), nil
			case leftIsScalar && rightIsMat:
				// scalar * mat -> OpMatrixTimesScalar (swapped operands)
				matResultType := e.backend.resolveTypeResolution(rightType)
				return e.backend.builder.AddBinaryOp(OpMatrixTimesScalar, matResultType, rightID, leftID), nil
			case leftIsVec && rightIsScalar:
				// vec * scalar -> OpVectorTimesScalar(vec, scalar)
				return e.backend.builder.AddBinaryOp(OpVectorTimesScalar, resultType, leftID, rightID), nil
			case leftIsScalar && rightIsVec:
				// scalar * vec -> OpVectorTimesScalar(vec, scalar) with swapped operands.
				vecResultType := e.backend.resolveTypeResolution(rightType)
				return e.backend.builder.AddBinaryOp(OpVectorTimesScalar, vecResultType, rightID, leftID), nil
			default:
				// Both scalar or both vector -> standard OpFMul.
				opcode = OpFMul
			}
		} else {
			opcode = OpIMul
		}
	case ir.BinaryDivide:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFDiv
			// Check for vec / scalar — SPIR-V has no OpVectorDivideScalar.
			// Splat the scalar to a matching vector.
			rightType, rErr := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			if rErr == nil {
				leftID, rightID, resultType = e.promoteScalarToVector(leftType, rightType, leftID, rightID, resultType)
			}
		} else if scalarKind == ir.ScalarSint {
			opcode = OpSDiv
		} else {
			opcode = OpUDiv
		}
	case ir.BinaryModulo:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFMod
			rightType, rErr := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			if rErr == nil {
				leftID, rightID, resultType = e.promoteScalarToVector(leftType, rightType, leftID, rightID, resultType)
			}
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

// promoteScalarToVector handles vec/scalar mixed operands for binary operations
// that have no dedicated SPIR-V opcode (divide, modulo, add, subtract).
// It splats the scalar operand to a vector of matching size using OpCompositeConstruct.
// Returns potentially updated leftID, rightID, and resultType.
func (e *ExpressionEmitter) promoteScalarToVector(
	leftType, rightType ir.TypeResolution,
	leftID, rightID, resultType uint32,
) (uint32, uint32, uint32) {
	var leftInner, rightInner ir.TypeInner
	if leftType.Handle != nil {
		leftInner = e.backend.module.Types[*leftType.Handle].Inner
	} else {
		leftInner = leftType.Value
	}
	if rightType.Handle != nil {
		rightInner = e.backend.module.Types[*rightType.Handle].Inner
	} else {
		rightInner = rightType.Value
	}

	leftVec, leftIsVec := leftInner.(ir.VectorType)
	_, rightIsScalar := rightInner.(ir.ScalarType)
	rightVec, rightIsVec := rightInner.(ir.VectorType)
	_, leftIsScalar := leftInner.(ir.ScalarType)

	if leftIsVec && rightIsScalar {
		// vec op scalar → splat scalar to matching vector
		rightID = e.splatScalar(rightID, leftVec)
		return leftID, rightID, resultType
	}
	if leftIsScalar && rightIsVec {
		// scalar op vec → splat scalar to matching vector, result type is vec
		leftID = e.splatScalar(leftID, rightVec)
		resultType = e.backend.resolveTypeResolution(rightType)
		return leftID, rightID, resultType
	}
	return leftID, rightID, resultType
}

// splatScalar creates an OpCompositeConstruct that replicates a scalar to a vector.
func (e *ExpressionEmitter) splatScalar(scalarID uint32, vecType ir.VectorType) uint32 {
	vecTypeID := e.backend.emitInlineType(vecType)
	splatID := e.backend.builder.AllocID()
	builder := e.newIB()
	builder.AddWord(vecTypeID)
	builder.AddWord(splatID)
	for range vecType.Size {
		builder.AddWord(scalarID)
	}
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpCompositeConstruct))
	return splatID
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

	// SPIR-V OpSelect requires the condition to be the same size as the result.
	// WGSL allows scalar bool condition with vector operands (broadcast).
	// When condition is scalar bool but result is vector, splat the condition.
	condType, err := ir.ResolveExpressionType(e.backend.module, e.function, sel.Condition)
	if err != nil {
		return 0, fmt.Errorf("select condition type: %w", err)
	}
	condInner := typeResolutionInner(e.backend.module, condType)
	acceptInner := typeResolutionInner(e.backend.module, acceptType)

	// If condition is float (scalar or vector), convert to bool by comparing != 0.0.
	// Common in shaders: select(a, b, step(...)) where step returns float, not bool.
	if condScalar, ok := condInner.(ir.ScalarType); ok && condScalar.Kind == ir.ScalarFloat {
		// scalar float → scalar bool via OpFOrdNotEqual(cond, 0.0)
		boolTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
		zeroID := e.backend.builder.AddConstantFloat32(
			e.backend.emitScalarType(condScalar), 0.0)
		cmpID := e.backend.builder.AllocID()
		builder := e.newIB()
		builder.AddWord(boolTypeID)
		builder.AddWord(cmpID)
		builder.AddWord(conditionID)
		builder.AddWord(zeroID)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpFOrdNotEqual))
		conditionID = cmpID
		condInner = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	} else if condVec, ok := condInner.(ir.VectorType); ok && condVec.Scalar.Kind == ir.ScalarFloat {
		// vec<float> → vec<bool> via OpFOrdNotEqual(cond, vec(0.0))
		boolVecTypeID := e.backend.emitVectorType(
			e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1}),
			uint32(condVec.Size))
		floatScalarID := e.backend.emitScalarType(condVec.Scalar)
		zeroScalarID := e.backend.builder.AddConstantFloat32(floatScalarID, 0.0)
		// Build zero vector
		zeroVecID := e.backend.builder.AllocID()
		zb := e.newIB()
		floatVecTypeID := e.backend.emitVectorType(floatScalarID, uint32(condVec.Size))
		zb.AddWord(floatVecTypeID)
		zb.AddWord(zeroVecID)
		for range condVec.Size {
			zb.AddWord(zeroScalarID)
		}
		e.backend.builder.functions = append(e.backend.builder.functions, zb.Build(OpCompositeConstruct))
		// Compare
		cmpID := e.backend.builder.AllocID()
		cb := e.newIB()
		cb.AddWord(boolVecTypeID)
		cb.AddWord(cmpID)
		cb.AddWord(conditionID)
		cb.AddWord(zeroVecID)
		e.backend.builder.functions = append(e.backend.builder.functions, cb.Build(OpFOrdNotEqual))
		conditionID = cmpID
		condInner = ir.VectorType{Size: condVec.Size, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}
	}

	// SPIR-V OpSelect requires the condition to be the same size as the result.
	// WGSL allows scalar bool condition with vector operands (broadcast).
	if _, isBoolScalar := condInner.(ir.ScalarType); isBoolScalar {
		if vecType, isVec := acceptInner.(ir.VectorType); isVec {
			// Splat scalar bool to vector bool
			boolVecTypeID := e.backend.emitVectorType(
				e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1}),
				uint32(vecType.Size))
			splatID := e.backend.builder.AllocID()
			builder := e.newIB()
			builder.AddWord(boolVecTypeID)
			builder.AddWord(splatID)
			for range vecType.Size {
				builder.AddWord(conditionID)
			}
			e.backend.builder.functions = append(
				e.backend.builder.functions,
				builder.Build(OpCompositeConstruct),
			)
			conditionID = splatID
		}
	}

	return e.backend.builder.AddSelect(resultType, conditionID, acceptID, rejectID), nil
}

// emitStatement emits a statement.
//
//nolint:cyclop,gocyclo,nestif,gocognit,funlen // Statement dispatch requires high cyclomatic complexity
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
		builder := e.newIB()
		builder.AddWord(ctx.mergeLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
		return nil

	case ir.StmtContinue:
		if len(e.loopStack) == 0 {
			return fmt.Errorf("continue statement outside of loop")
		}
		ctx := e.loopStack[len(e.loopStack)-1]
		builder := e.newIB()
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

	case ir.StmtSwitch:
		return e.emitSwitch(kind)

	case ir.StmtCall:
		return e.emitCall(kind)

	case ir.StmtImageStore:
		return e.emitImageStore(kind)

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

	// Accept block - use pre-allocated label ID so it matches the branch target
	e.backend.builder.AddLabelWithID(acceptLabel)
	for _, acceptStmt := range stmt.Accept {
		if err := e.emitStatement(acceptStmt); err != nil {
			return err
		}
	}
	// Branch to merge only if block didn't already terminate (return/kill/break/continue)
	if !blockEndsWithTerminator(stmt.Accept) {
		builder := e.newIB()
		builder.AddWord(mergeLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
	}

	// Reject block - use pre-allocated label ID so it matches the branch target
	e.backend.builder.AddLabelWithID(rejectLabel)
	for _, rejectStmt := range stmt.Reject {
		if err := e.emitStatement(rejectStmt); err != nil {
			return err
		}
	}
	// Branch to merge only if block didn't already terminate
	if !blockEndsWithTerminator(stmt.Reject) {
		builder := e.newIB()
		builder.AddWord(mergeLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
	}

	// Merge label - use pre-allocated label ID so it matches the selection merge target
	e.backend.builder.AddLabelWithID(mergeLabel)

	// If both branches terminated (return/kill), the merge block is unreachable
	// but SPIR-V requires every basic block to end with a terminator instruction.
	if blockEndsWithTerminator(stmt.Accept) && blockEndsWithTerminator(stmt.Reject) {
		e.backend.builder.AddUnreachable()
	}

	return nil
}

// blockEndsWithTerminator checks if a block ends with a terminator instruction
// (return, kill, break, continue) that makes subsequent OpBranch unreachable.
// In SPIR-V, each basic block must end with exactly one terminator instruction.
// findCallResultInTree checks if an expression tree contains an ExprCallResult.
// Returns the ExprCallResult handle if found. This is used to detect local variable
// inits like `var x = func() - 0.5` where the call result is nested inside a
// binary expression, not the top-level init expression.
func findCallResultInTree(expressions []ir.Expression, handle ir.ExpressionHandle) (ir.ExpressionHandle, bool) {
	if int(handle) >= len(expressions) {
		return 0, false
	}
	expr := expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprCallResult:
		return handle, true
	case ir.ExprBinary:
		if h, ok := findCallResultInTree(expressions, k.Left); ok {
			return h, true
		}
		return findCallResultInTree(expressions, k.Right)
	case ir.ExprUnary:
		return findCallResultInTree(expressions, k.Expr)
	default:
		return 0, false
	}
}

func blockEndsWithTerminator(block ir.Block) bool {
	if len(block) == 0 {
		return false
	}
	lastStmt := block[len(block)-1]
	switch kind := lastStmt.Kind.(type) {
	case ir.StmtReturn, ir.StmtKill, ir.StmtBreak, ir.StmtContinue:
		return true
	case ir.StmtBlock:
		// Unwrap block wrapper (generated by lowerStatement for else clauses)
		return blockEndsWithTerminator(kind.Block)
	case ir.StmtIf:
		// An if statement is a terminator if BOTH branches end with terminators
		return blockEndsWithTerminator(kind.Accept) && blockEndsWithTerminator(kind.Reject)
	case ir.StmtSwitch:
		// A switch is a terminator if it has a default case and all cases end with terminators
		hasDefault := false
		for _, c := range kind.Cases {
			if _, ok := c.Value.(ir.SwitchValueDefault); ok {
				hasDefault = true
			}
			if !blockEndsWithTerminator(c.Body) {
				return false
			}
		}
		return hasDefault
	default:
		return false
	}
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
	builder := e.newIB()
	builder.AddWord(headerLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Header label - use pre-allocated ID
	e.backend.builder.AddLabelWithID(headerLabel)

	// OpLoopMerge declares merge and continue targets
	e.backend.builder.AddLoopMerge(mergeLabel, continueLabel, LoopControlNone)

	// Branch to body
	builder = e.newIB()
	builder.AddWord(bodyLabel)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))

	// Body label - use pre-allocated ID
	e.backend.builder.AddLabelWithID(bodyLabel)

	// Emit body statements
	for _, bodyStmt := range stmt.Body {
		if err := e.emitStatement(bodyStmt); err != nil {
			return err
		}
	}

	// Branch to continue block only if body didn't already terminate
	if !blockEndsWithTerminator(stmt.Body) {
		builder = e.newIB()
		builder.AddWord(continueLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
	}

	// Continue label - use pre-allocated ID
	e.backend.builder.AddLabelWithID(continueLabel)

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
		builder = e.newIB()
		builder.AddWord(headerLabel)
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpBranch))
	}

	// Merge label - use pre-allocated ID
	e.backend.builder.AddLabelWithID(mergeLabel)

	return nil
}

// emitSwitch emits a switch statement.
func (e *ExpressionEmitter) emitSwitch(stmt ir.StmtSwitch) error {
	// Evaluate selector
	selectorID, err := e.emitExpression(stmt.Selector)
	if err != nil {
		return fmt.Errorf("switch selector: %w", err)
	}

	// Allocate labels for merge and each case block
	mergeLabel := e.backend.builder.AllocID()
	caseLabels := make([]uint32, len(stmt.Cases))
	for i := range stmt.Cases {
		caseLabels[i] = e.backend.builder.AllocID()
	}

	// Find default case index
	defaultIdx := -1
	for i, c := range stmt.Cases {
		if _, isDefault := c.Value.(ir.SwitchValueDefault); isDefault {
			defaultIdx = i
			break
		}
	}
	if defaultIdx == -1 {
		return fmt.Errorf("switch statement has no default case")
	}
	defaultLabel := caseLabels[defaultIdx]

	// OpSelectionMerge declares the merge point
	e.backend.builder.AddSelectionMerge(mergeLabel, SelectionControlNone)

	// Build OpSwitch instruction
	// Format: OpSwitch Selector Default (Literal Label)*
	switchBuilder := e.newIB()
	switchBuilder.AddWord(selectorID)
	switchBuilder.AddWord(defaultLabel)

	// Add literal/label pairs for non-default cases
	for i, c := range stmt.Cases {
		switch v := c.Value.(type) {
		case ir.SwitchValueI32:
			// #nosec G115 - int32 to uint32 conversion for SPIR-V literal encoding
			switchBuilder.AddWord(uint32(v) & 0xFFFFFFFF)
			switchBuilder.AddWord(caseLabels[i])
		case ir.SwitchValueU32:
			switchBuilder.AddWord(uint32(v))
			switchBuilder.AddWord(caseLabels[i])
		case ir.SwitchValueDefault:
			// Default is handled via defaultLabel parameter, skip here
			continue
		}
	}
	e.backend.builder.functions = append(e.backend.builder.functions, switchBuilder.Build(OpSwitch))

	// Emit each case block
	for i, c := range stmt.Cases {
		// Case label
		e.backend.builder.AddLabelWithID(caseLabels[i])

		// Emit case body statements
		for _, bodyStmt := range c.Body {
			if err := e.emitStatement(bodyStmt); err != nil {
				return fmt.Errorf("switch case body: %w", err)
			}
		}

		// Branch to appropriate target
		// Note: WGSL doesn't support fallthrough, but IR allows it for other frontends
		var targetLabel uint32
		if c.FallThrough && i < len(stmt.Cases)-1 {
			// Fallthrough to next case
			targetLabel = caseLabels[i+1]
		} else {
			// Branch to merge (normal case, or last case with fallthrough)
			targetLabel = mergeLabel
		}
		// Branch to target only if case body didn't already terminate
		if !blockEndsWithTerminator(c.Body) {
			branchBuilder := e.newIB()
			branchBuilder.AddWord(targetLabel)
			e.backend.builder.functions = append(e.backend.builder.functions, branchBuilder.Build(OpBranch))
		}
	}

	// Merge label - use pre-allocated ID
	e.backend.builder.AddLabelWithID(mergeLabel)

	// If all cases terminate (return/break/etc), the merge block is unreachable.
	// SPIR-V still requires every block to end with a terminator instruction.
	allCasesTerminate := true
	for _, c := range stmt.Cases {
		if !blockEndsWithTerminator(c.Body) {
			allCasesTerminate = false
			break
		}
	}
	if allCasesTerminate {
		builder := e.newIB()
		e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpUnreachable))
	}

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

	// Functions that return a scalar float even when given a vector argument:
	// length(vecN) -> float, distance(vecN, vecN) -> float, dot(vecN, vecN) -> float,
	// determinant(matNxN) -> float.
	switch mathExpr.Fun {
	case ir.MathLength, ir.MathDistance, ir.MathDot, ir.MathDeterminant:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{Value: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
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
//
//nolint:funlen // texture sampling has many SPIR-V operands
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

	// For arrayed textures, append array index to coordinate vector.
	if sample.ArrayIndex != nil {
		coordID, err = e.appendArrayIndex(coordID, *sample.ArrayIndex, sample.Coordinate)
		if err != nil {
			return 0, err
		}
	}

	// emitExpression now auto-loads, so imagePtrID and samplerPtrID are already
	// the loaded image/sampler handles, not pointers
	imageID := imagePtrID
	samplerID := samplerPtrID

	// Create SampledImage by combining image and sampler
	sampledImageTypeID := e.backend.getSampledImageType(e.function, sample.Image)
	sampledImageID := e.backend.builder.AllocID()

	sampledImageBuilder := e.newIB()
	sampledImageBuilder.AddWord(sampledImageTypeID)
	sampledImageBuilder.AddWord(sampledImageID)
	sampledImageBuilder.AddWord(imageID)
	sampledImageBuilder.AddWord(samplerID)
	e.backend.builder.functions = append(e.backend.builder.functions, sampledImageBuilder.Build(OpSampledImage))

	// Result type is vec4<f32> for sampled images
	resultType := e.backend.emitVec4F32Type()
	resultID := e.backend.builder.AllocID()

	builder := e.newIB()
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

	builder := e.newIB()
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
	// Image query operations require the ImageQuery capability.
	e.backend.addCapability(CapabilityImageQuery)

	imageID, err := e.emitExpression(query.Image)
	if err != nil {
		return 0, err
	}

	var resultID uint32
	builder := e.newIB()

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
func (b *Backend) getSampledImageType(fn *ir.Function, imageExpr ir.ExpressionHandle) uint32 {
	// Resolve the actual image type from the expression
	img := ir.ImageType{
		Dim:   ir.Dim2D,
		Class: ir.ImageClassSampled,
	}
	exprType, err := ir.ResolveExpressionType(b.module, fn, imageExpr)
	if err == nil {
		inner := typeResolutionInner(b.module, exprType)
		if imgType, ok := inner.(ir.ImageType); ok {
			img = imgType
		}
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
	builder := b.newIB()
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
	builder := e.newIB()
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
	builder := e.newIB()
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
	builder := e.newIB()
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

// appendArrayIndex converts an integer array index to float and appends it to the coordinate vector.
// Returns the new coordinate ID with the array index appended.
func (e *ExpressionEmitter) appendArrayIndex(coordID uint32, arrayIndexExpr, coordExpr ir.ExpressionHandle) (uint32, error) {
	arrayIndexID, err := e.emitExpression(arrayIndexExpr)
	if err != nil {
		return 0, err
	}
	// Convert array index to float if it's integer
	arrayIndexType, _ := ir.ResolveExpressionType(e.backend.module, e.function, arrayIndexExpr)
	indexInner := typeResolutionInner(e.backend.module, arrayIndexType)
	if scalar, ok := indexInner.(ir.ScalarType); ok && scalar.Kind != ir.ScalarFloat {
		floatTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		convertedID := e.backend.builder.AllocID()
		cb := e.newIB()
		cb.AddWord(floatTypeID)
		cb.AddWord(convertedID)
		cb.AddWord(arrayIndexID)
		opcode := OpConvertSToF
		if scalar.Kind == ir.ScalarUint {
			opcode = OpConvertUToF
		}
		e.backend.builder.functions = append(e.backend.builder.functions, cb.Build(opcode))
		arrayIndexID = convertedID
	}
	// Extend coordinate vector: e.g. vec2(x,y) + arrayIndex → vec3(x,y,arrayIndex)
	coordType, _ := ir.ResolveExpressionType(e.backend.module, e.function, coordExpr)
	coordInner := typeResolutionInner(e.backend.module, coordType)
	if coordVec, ok := coordInner.(ir.VectorType); ok {
		floatScalarID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		newVecTypeID := e.backend.emitVectorType(floatScalarID, uint32(coordVec.Size+1))
		extCoordID := e.backend.builder.AllocID()
		eb := e.newIB()
		eb.AddWord(newVecTypeID)
		eb.AddWord(extCoordID)
		eb.AddWord(coordID)
		eb.AddWord(arrayIndexID)
		e.backend.builder.functions = append(e.backend.builder.functions, eb.Build(OpCompositeConstruct))
		return extCoordID, nil
	}
	return coordID, nil
}

// emitImageStore emits an OpImageWrite instruction for textureStore().
func (e *ExpressionEmitter) emitImageStore(store ir.StmtImageStore) error {
	imageID, err := e.emitExpression(store.Image)
	if err != nil {
		return fmt.Errorf("image store image: %w", err)
	}

	coordID, err := e.emitExpression(store.Coordinate)
	if err != nil {
		return fmt.Errorf("image store coordinate: %w", err)
	}

	// Handle array index for arrayed storage textures
	if store.ArrayIndex != nil {
		coordID, err = e.appendArrayIndex(coordID, *store.ArrayIndex, store.Coordinate)
		if err != nil {
			return err
		}
	}

	valueID, err := e.emitExpression(store.Value)
	if err != nil {
		return fmt.Errorf("image store value: %w", err)
	}

	// OpImageWrite: no result type, no result ID
	builder := e.newIB()
	builder.AddWord(imageID)
	builder.AddWord(coordID)
	builder.AddWord(valueID)
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpImageWrite))

	return nil
}

// emitCall emits a function call statement (OpFunctionCall).
// If the call has a result, the SPIR-V result ID is cached for later ExprCallResult lookup.
func (e *ExpressionEmitter) emitCall(call ir.StmtCall) error {
	// Look up the SPIR-V function ID
	funcID, ok := e.backend.functionIDs[call.Function]
	if !ok {
		return fmt.Errorf("function %d not found in functionIDs", call.Function)
	}

	// Collect argument SPIR-V IDs
	argIDs := make([]uint32, 0, len(call.Arguments))
	for _, arg := range call.Arguments {
		argID, err := e.emitExpression(arg)
		if err != nil {
			return fmt.Errorf("call argument: %w", err)
		}
		argIDs = append(argIDs, argID)
	}

	// Determine result type
	var resultTypeID uint32
	fn := e.backend.module.Functions[call.Function]
	if fn.Result != nil {
		var err error
		resultTypeID, err = e.backend.emitType(fn.Result.Type)
		if err != nil {
			return fmt.Errorf("call result type: %w", err)
		}
	} else {
		resultTypeID = e.backend.getVoidType()
	}

	// Generate result ID and emit OpFunctionCall
	resultID := e.backend.builder.AllocID()
	builder := e.newIB()
	builder.AddWord(resultTypeID)
	builder.AddWord(resultID)
	builder.AddWord(funcID)
	for _, argID := range argIDs {
		builder.AddWord(argID)
	}
	e.backend.builder.functions = append(e.backend.builder.functions, builder.Build(OpFunctionCall))

	// Cache the result ID for ExprCallResult and handle deferred stores.
	if call.Result != nil {
		e.callResultIDs[*call.Result] = resultID
		if err := e.processDeferredStores(*call.Result, resultID); err != nil {
			return err
		}
	}

	return nil
}

// processDeferredStores handles deferred local variable stores after a call result
// is available. Handles both direct (var x = func()) and complex (var x = func() - 0.5)
// deferred stores.
func (e *ExpressionEmitter) processDeferredStores(resultHandle ir.ExpressionHandle, resultID uint32) error {
	// Direct: local var is initialized with this call result directly.
	if varPtrID, ok := e.deferredCallStores[resultHandle]; ok {
		e.backend.builder.AddStore(varPtrID, resultID)
		delete(e.deferredCallStores, resultHandle)
	}

	// Complex: local var init expression CONTAINS this call result.
	stores, ok := e.deferredComplexStores[resultHandle]
	if !ok {
		return nil
	}
	for _, s := range stores {
		initID, err := e.emitExpression(s.initExpr)
		if err != nil {
			return fmt.Errorf("deferred complex store: %w", err)
		}
		e.backend.builder.AddStore(s.varPtrID, initID)
	}
	delete(e.deferredComplexStores, resultHandle)
	return nil
}

// emitCallResultRef returns the SPIR-V ID for a function call result (set by emitCall).
func (e *ExpressionEmitter) emitCallResultRef(handle ir.ExpressionHandle) (uint32, error) {
	if id, ok := e.callResultIDs[handle]; ok {
		// Cache in exprIDs so subsequent references use the fast path.
		e.exprIDs[handle] = id
		return id, nil
	}
	return 0, fmt.Errorf("call result for expression %d not found (was StmtCall emitted?)", handle)
}

// emitArrayLength emits OpArrayLength for runtime-sized arrays in storage buffers.
//
// SPIR-V requires OpArrayLength to operate on a pointer to the struct that
// contains the runtime-sized array as its last member, plus the member index.
//
// The expression chain from the IR is one of:
//   - ExprArrayLength { Array: ExprGlobalVariable } -- global IS the runtime array
//     (backend wraps it in a synthetic struct, so member index = 0)
//   - ExprArrayLength { Array: ExprAccessIndex { Base: ExprGlobalVariable, Index: N } }
//     -- global is a struct whose member N is the runtime array
func (e *ExpressionEmitter) emitArrayLength(expr ir.ExprArrayLength) (uint32, error) {
	// Walk the Array expression to find the global variable and optional member index.
	var globalHandle ir.GlobalVariableHandle
	var memberIndex *uint32 // nil if the array expression refers to the global directly

	arrayExpr := e.function.Expressions[expr.Array]
	switch k := arrayExpr.Kind.(type) {
	case ir.ExprAccessIndex:
		// Array is a member of a struct: arrayLength(&buf.data)
		// The base must be a global variable.
		baseExpr := e.function.Expressions[k.Base]
		switch bk := baseExpr.Kind.(type) {
		case ir.ExprGlobalVariable:
			globalHandle = bk.Variable
			idx := k.Index
			memberIndex = &idx
		default:
			return 0, fmt.Errorf("array length: AccessIndex base must be GlobalVariable, got %T", bk)
		}
	case ir.ExprGlobalVariable:
		// Array IS the global variable directly: arrayLength(&output)
		globalHandle = k.Variable
		memberIndex = nil
	default:
		return 0, fmt.Errorf("array length: expected GlobalVariable or AccessIndex, got %T", k)
	}

	// Get the SPIR-V variable ID for the global.
	varID, ok := e.backend.globalIDs[globalHandle]
	if !ok {
		return 0, fmt.Errorf("array length: global variable %d not found", globalHandle)
	}

	// Determine the struct pointer ID and member index for OpArrayLength.
	//
	// SPIR-V requires that OpArrayLength operates on a pointer to an
	// OpTypeStruct whose last member is a runtime-sized array.
	//
	// Case 1: The Naga IR global type is already a struct.
	//   The struct was decorated with Block directly. Use the global var
	//   pointer and the member index from the AccessIndex expression.
	//
	// Case 2: The Naga IR global type is a bare runtime array.
	//   The backend wrapped it in a synthetic struct (see emitGlobals).
	//   The wrapper struct has the array at member 0. Use the global var
	//   pointer (which points to the wrapper struct) and member index 0.
	var structID uint32
	var lastMemberIndex uint32

	isWrapped := e.backend.wrappedStorageVars[globalHandle]

	switch {
	case memberIndex != nil && !isWrapped:
		// Naga struct type, not wrapped. The runtime array is at the given member index.
		structID = varID
		lastMemberIndex = *memberIndex
	case memberIndex == nil && isWrapped:
		// Bare runtime array, wrapped in a synthetic struct. Member index is 0.
		structID = varID
		lastMemberIndex = 0
	case memberIndex != nil && isWrapped:
		return 0, fmt.Errorf("array length: unexpected wrapped variable with AccessIndex")
	default:
		// memberIndex == nil && !isWrapped: the global is not wrapped and not accessed
		// through a struct member. This means the global type itself must be a struct
		// whose last member is the runtime array. This shouldn't happen in valid IR
		// since the lowerer always creates an AccessIndex for struct members.
		return 0, fmt.Errorf("array length: global variable is not a struct and was not wrapped")
	}

	// Result type is always u32.
	resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})

	// Emit OpArrayLength: result-type result-id struct-pointer member-index
	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultType)
	ib.AddWord(resultID)
	ib.AddWord(structID)
	ib.AddWord(lastMemberIndex)
	e.backend.builder.functions = append(e.backend.builder.functions, ib.Build(OpArrayLength))

	return resultID, nil
}
