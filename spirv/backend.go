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
	// memberVarIDs holds input variable IDs indexed by struct member index.
	// Zero means no variable for that member. Only populated when isStruct is true.
	memberVarIDs []uint32
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
	// memberVarIDs holds output variable IDs indexed by struct member index.
	// Zero means no variable for that member. Only populated when isStruct is true.
	memberVarIDs []uint32
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
	// Key: entry point index in Module.EntryPoints[]
	entryInputVars  map[int][]*entryPointInput // index = arg index
	entryOutputVars map[int]*entryPointOutput  // For function result

	// Entry point function SPIR-V IDs (separate from regular functionIDs).
	// Key: entry point index in Module.EntryPoints[]
	entryPointFuncIDs map[int]uint32

	// Cached sampled image type (for texture sampling operations)
	// Key: image dimension + arrayed, Value: SPIR-V type ID
	sampledImageTypeIDs map[uint64]uint32

	// Cached image type (for reuse)
	// Key: image dimension + arrayed, Value: SPIR-V type ID
	imageTypeIDs map[uint64]uint32

	// Cached scalar types (f32, i32, u32, bool, f16, etc.)
	// Key: (kind << 8) | width, Value: SPIR-V type ID
	scalarTypeIDs map[uint32]uint32

	// Cached pointer types
	// Key: (storageClass << 24) | baseTypeID, Value: SPIR-V type ID
	pointerTypeIDs map[uint32]uint32

	// Cached vector types
	// Key: (componentTypeID << 8) | componentCount, Value: SPIR-V type ID
	vectorTypeIDs map[uint32]uint32

	// Cached matrix types
	// Key: (columnTypeID << 8) | columnCount, Value: SPIR-V type ID
	matrixTypeIDs map[uint32]uint32

	// Track used capabilities (to avoid duplicates)
	usedCapabilities map[Capability]bool

	// Track used extensions (to avoid duplicates)
	usedExtensions map[string]bool

	// Cached void type ID (only one void type allowed in SPIR-V)
	voidTypeID uint32

	// Cached function types (key: concatenated return + param type IDs)
	funcTypeIDs map[string]uint32

	// Storage buffer variables that were wrapped in a struct for Vulkan compliance.
	// When accessing these variables, an extra index 0 must be prepended to
	// OpAccessChain to go through the wrapper struct to the actual data.
	wrappedStorageVars map[ir.GlobalVariableHandle]bool

	// Track types already decorated with Block (to avoid duplicate decorations
	// when multiple variables share the same struct type).
	blockDecoratedTypes map[uint32]bool

	// Layout-free type cache (IR TypeHandle → SPIR-V ID without ArrayStride/Offset).
	// Used for Workgroup variables which must not have explicit layout decorations.
	layoutFreeTypeIDs map[ir.TypeHandle]uint32

	// ForcePointSize variable IDs per entry point index.
	// Only populated when Options.ForcePointSize is true and the entry
	// point is a vertex shader that doesn't already have a PointSize output.
	forcePointSizeVars map[int]uint32

	// Workgroup init polyfill: LocalInvocationId Input variable IDs per entry point.
	// Created when the entry point has workgroup variables that need zero-initialization.
	workgroupInitVars map[int]uint32

	// F16 I/O polyfill: maps an Input/Output variable ID to its f32 value type ID.
	// When UseStorageInputOutput16 is false, f16 I/O variables are declared with
	// f32 types, and OpFConvert is emitted at load/store time.
	f16PolyfillVars map[uint32]uint32

	// SampleMask builtin variables. Per SPIR-V spec (VUID-SampleMask-SampleMask-04359),
	// the SampleMask BuiltIn must be typed as array<u32, 1>, not scalar u32.
	// These variables need AccessChain[0] when loading/storing.
	sampleMaskVars map[uint32]bool

	// Shared instruction builder reused across emit methods that don't call
	// other emit functions between Reset() and Build().
	ib InstructionBuilder

	// Wrapped helper functions for integer div/mod safety.
	// Key: wrappedBinaryOp (op + left/right SPIR-V type IDs).
	// Value: SPIR-V function ID of the wrapper.
	wrappedFuncIDs map[wrappedBinaryOp]uint32

	// Ray query helper function cache.
	// Key: rayQueryFuncKind, Value: SPIR-V function ID.
	rayQueryFuncIDs map[rayQueryFuncKind]uint32

	// Cached OpTypeSampler ID (only one sampler type allowed in SPIR-V).
	samplerTypeID uint32

	// Set of struct type handles whose global variables use Uniform address space.
	// Used to apply std140 MatrixStride rules (column stride >= 16 for f32).
	uniformStructTypes map[ir.TypeHandle]bool
}

// wrappedBinaryOp is the dedup key for wrapped binary operation functions.
// Matches Rust naga's WrappedFunction::BinaryOp { op, left_type_id, right_type_id }.
type wrappedBinaryOp struct {
	op          ir.BinaryOperator
	leftTypeID  uint32
	rightTypeID uint32
}

// NewBackend creates a new SPIR-V backend.
func NewBackend(options Options) *Backend {
	return &Backend{
		options:             options,
		typeIDs:             make(map[ir.TypeHandle]uint32, 16),
		constantIDs:         make(map[ir.ConstantHandle]uint32, 16),
		globalIDs:           make(map[ir.GlobalVariableHandle]uint32, 4),
		functionIDs:         make(map[ir.FunctionHandle]uint32, 4),
		entryInputVars:      make(map[int][]*entryPointInput, 2),
		entryOutputVars:     make(map[int]*entryPointOutput, 2),
		entryPointFuncIDs:   make(map[int]uint32, 2),
		sampledImageTypeIDs: make(map[uint64]uint32, 4),
		imageTypeIDs:        make(map[uint64]uint32, 4),
		scalarTypeIDs:       make(map[uint32]uint32, 8),
		pointerTypeIDs:      make(map[uint32]uint32, 8),
		vectorTypeIDs:       make(map[uint32]uint32, 8),
		matrixTypeIDs:       make(map[uint32]uint32, 4),
		usedCapabilities:    make(map[Capability]bool, 4),
		usedExtensions:      make(map[string]bool, 2),
		funcTypeIDs:         make(map[string]uint32, 4),
		wrappedStorageVars:  make(map[ir.GlobalVariableHandle]bool, 2),
		blockDecoratedTypes: make(map[uint32]bool, 4),
		layoutFreeTypeIDs:   make(map[ir.TypeHandle]uint32, 4),
		forcePointSizeVars:  make(map[int]uint32, 2),
		workgroupInitVars:   make(map[int]uint32, 2),
		wrappedFuncIDs:      make(map[wrappedBinaryOp]uint32, 4),
		f16PolyfillVars:     make(map[uint32]uint32, 4),
		sampleMaskVars:      make(map[uint32]bool, 2),
		rayQueryFuncIDs:     make(map[rayQueryFuncKind]uint32, 6),
		uniformStructTypes:  make(map[ir.TypeHandle]bool, 4),
	}
}

// Reset clears all state in the Backend without deallocating.
// This allows reuse of the same Backend instance across compilations,
// avoiding repeated allocation of maps and slices. After Reset, the
// Backend is in the same logical state as a freshly created one.
//
// Reset is called automatically at the start of Compile, so callers
// do not need to call it explicitly.
func (b *Backend) Reset() {
	b.module = nil

	// Clear maps — Go 1.21+ clear() keeps capacity, removes all entries
	clear(b.typeIDs)
	clear(b.constantIDs)
	clear(b.globalIDs)
	clear(b.functionIDs)
	clear(b.entryInputVars)
	clear(b.entryOutputVars)
	clear(b.entryPointFuncIDs)
	clear(b.sampledImageTypeIDs)
	clear(b.imageTypeIDs)
	clear(b.scalarTypeIDs)
	clear(b.pointerTypeIDs)
	clear(b.vectorTypeIDs)
	clear(b.matrixTypeIDs)
	clear(b.usedCapabilities)
	clear(b.usedExtensions)
	clear(b.funcTypeIDs)
	clear(b.wrappedStorageVars)
	clear(b.blockDecoratedTypes)
	clear(b.layoutFreeTypeIDs)
	clear(b.forcePointSizeVars)
	clear(b.workgroupInitVars)
	clear(b.wrappedFuncIDs)
	clear(b.f16PolyfillVars)
	clear(b.sampleMaskVars)
	clear(b.rayQueryFuncIDs)
	clear(b.uniformStructTypes)

	// Reset scalar IDs
	b.glslExtID = 0
	b.voidTypeID = 0
	b.samplerTypeID = 0

	// Reset instruction builder scratch space
	b.ib.words = b.ib.words[:0]
}

// needsF16Polyfill checks if a type is f16-related and needs polyfill
// (converting to f32) for Input/Output variables when StorageInputOutput16
// is not available.
func (b *Backend) needsF16Polyfill(typeHandle ir.TypeHandle) bool {
	if b.options.UseStorageInputOutput16 {
		return false
	}
	inner := b.module.Types[typeHandle].Inner
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind == ir.ScalarFloat && t.Width == 2
	case ir.VectorType:
		return t.Scalar.Kind == ir.ScalarFloat && t.Scalar.Width == 2
	default:
		return false
	}
}

// getF16PolyfillTypeID returns the f32 equivalent type ID for an f16 type.
// For f16 -> f32, for vec2<f16> -> vec2<f32>, etc.
func (b *Backend) getF16PolyfillTypeID(typeHandle ir.TypeHandle) uint32 {
	inner := b.module.Types[typeHandle].Inner
	switch t := inner.(type) {
	case ir.ScalarType:
		return b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	case ir.VectorType:
		f32Scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		scalarID := b.emitScalarType(f32Scalar)
		return b.emitVectorType(scalarID, uint32(t.Size))
	default:
		// Not f16 -- return original type
		id, _ := b.emitType(typeHandle)
		return id
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
// The Backend is automatically reset before each compilation, so
// a single Backend instance can be reused across multiple Compile calls.
func (b *Backend) Compile(module *ir.Module) ([]byte, error) {
	// Reset all per-compilation state (maps cleared, slices truncated).
	b.Reset()
	b.module = module

	// Reuse or create the ModuleBuilder.
	if b.builder != nil {
		b.builder.Reset(b.options.Version)
	} else {
		b.builder = NewModuleBuilder(b.options.Version)
	}

	// Initialize shared instruction builder with module builder's arena for zero-alloc builds.
	b.ib = InstructionBuilder{
		words: b.ib.words[:0], // reuse existing scratch buffer
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

	// 13. Linkage capability for modules without compilable entry points
	// Count non-mesh/task entry points (mesh/task are skipped in SPIR-V)
	compilableEPs := 0
	for _, ep := range b.module.EntryPoints {
		if ep.Stage != ir.StageTask && ep.Stage != ir.StageMesh {
			compilableEPs++
		}
	}
	if compilableEPs == 0 {
		b.addCapability(CapabilityLinkage)
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
// langVersion returns the SPIR-V version as a packed uint32 (major<<16 | minor<<8).
func (b *Backend) langVersion() uint32 {
	return (uint32(b.options.Version.Major) << 16) | (uint32(b.options.Version.Minor) << 8)
}

func (b *Backend) addCapability(capability Capability) {
	if !b.usedCapabilities[capability] {
		b.usedCapabilities[capability] = true
		b.builder.AddCapability(capability)
	}
}

// capabilityAvailable checks whether a capability may be used.
// When CapabilitiesAvailable is nil (the default), all capabilities are
// available. Otherwise, the capability must be present in the set.
func (b *Backend) capabilityAvailable(capability Capability) bool {
	if b.options.CapabilitiesAvailable == nil {
		return true
	}
	_, ok := b.options.CapabilitiesAvailable[capability]
	return ok
}

// requireAllCapabilities checks whether all listed capabilities are available.
// Returns true if they are (and adds them to used set). Returns false if any
// capability is not available (adds nothing). Matches Rust naga's require_all.
func (b *Backend) requireAllCapabilities(caps ...Capability) bool {
	for _, cap := range caps {
		if !b.capabilityAvailable(cap) {
			return false
		}
	}
	for _, cap := range caps {
		b.addCapability(cap)
	}
	return true
}

// requireVersion bumps the SPIR-V module version to at least minVersion.
func (b *Backend) requireVersion(minVersion Version) {
	b.builder.RequireVersion(minVersion)
}

// addExtension adds a SPIR-V extension without duplicates.
func (b *Backend) addExtension(name string) {
	if !b.usedExtensions[name] {
		b.usedExtensions[name] = true
		b.builder.AddExtension(name)
	}
}

// decorateNonUniformBindingArrayAccess adds the ShaderNonUniform capability,
// the SPV_EXT_descriptor_indexing extension, and a NonUniform decoration to id.
// Matches Rust naga's decorate_non_uniform_binding_array_access.
func (b *Backend) decorateNonUniformBindingArrayAccess(id uint32) {
	b.addCapability(CapabilityShaderNonUniform)
	b.addExtension("SPV_EXT_descriptor_indexing")
	b.builder.AddDecorate(id, DecorationNonUniform)
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

	// Entry point function names
	for epIdx := range b.module.EntryPoints {
		ep := &b.module.EntryPoints[epIdx]
		if ep.Function.Name != "" {
			if id, ok := b.entryPointFuncIDs[epIdx]; ok {
				b.builder.AddName(id, ep.Function.Name)
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

			// Matrices and (potentially nested) arrays of matrices both require decorations,
			// so "see through" any arrays to determine if they're needed.
			// Matches Rust naga's decorate_struct_member (writer.rs ~line 2479-2505).
			memberInner := b.module.Types[member.Type].Inner
			for {
				if arr, ok := memberInner.(ir.ArrayType); ok {
					memberInner = b.module.Types[arr.Base].Inner
				} else {
					break
				}
			}
			if mat, ok := memberInner.(ir.MatrixType); ok {
				b.builder.AddMemberDecorate(structID, uint32(memberIndex), DecorationColMajor)
				// Column stride: vec2=2*width, vec3/vec4=4*width (WGSL alignment rules).
				var rowMul uint32
				switch mat.Rows {
				case ir.Vec2:
					rowMul = 2
				default:
					rowMul = 4
				}
				stride := rowMul * uint32(mat.Scalar.Width)
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
	// Emit void type first when the module has functions, matching Rust naga
	// which places OpTypeVoid before other type declarations. This ensures
	// consistent type numbering between our output and the Rust reference.
	if len(b.module.Functions) > 0 || len(b.module.EntryPoints) > 0 {
		b.getVoidType()
	}

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
		id = b.emitMatrixType(columnTypeID, uint32(inner.Columns))

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
		// Emit all member types first, checking for runtime arrays.
		// Matches Rust naga writer.rs:1534-1559.
		memberIDs := make([]uint32, len(inner.Members))
		hasRuntimeArray := false
		for i, member := range inner.Members {
			memberID, err := b.emitType(member.Type)
			if err != nil {
				return 0, err
			}
			memberIDs[i] = memberID
			// Check if this member is a runtime-sized array
			if arr, ok := b.module.Types[member.Type].Inner.(ir.ArrayType); ok {
				if arr.Size.Constant == nil {
					hasRuntimeArray = true
				}
			}
		}

		id = b.builder.AddTypeStruct(memberIDs...)
		// Structs with runtime arrays get Block decoration during type emission.
		// Matches Rust naga writer.rs:1556-1558.
		if hasRuntimeArray {
			b.builder.AddDecorate(id, DecorationBlock)
			b.blockDecoratedTypes[id] = true
		}

	case ir.PointerType:
		baseID, err := b.emitType(inner.Base)
		if err != nil {
			return 0, err
		}

		storageClass := addressSpaceToStorageClass(inner.Space)
		id = b.emitPointerType(storageClass, baseID)

	case ir.SamplerType:
		// OpTypeSampler must be emitted exactly once (SPIR-V forbids duplicate
		// non-aggregate type declarations). Cache and reuse.
		if b.samplerTypeID != 0 {
			b.typeIDs[handle] = b.samplerTypeID
			return b.samplerTypeID, nil
		}
		id = b.builder.AllocID()
		builder := b.newIB()
		builder.AddWord(id)
		b.builder.types = append(b.builder.types, builder.Build(OpTypeSampler))
		b.samplerTypeID = id

	case ir.ImageType:
		// Derive sampled type from image class.
		// Rust naga: Sampled{kind,multi} -> Scalar{kind, width:4}
		//            Depth{multi}        -> Scalar{Float, width:4}
		//            Storage{format,...}  -> format.into() (scalar from storage format)
		var sampledScalar ir.ScalarType
		switch inner.Class {
		case ir.ImageClassSampled:
			sampledScalar = ir.ScalarType{Kind: inner.SampledKind, Width: 4}
		case ir.ImageClassDepth:
			sampledScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		case ir.ImageClassStorage:
			sampledScalar = storageFormatToScalar(inner.StorageFormat)
		default:
			sampledScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		}
		sampledTypeID := b.emitScalarType(sampledScalar)
		id = b.emitImageType(sampledTypeID, inner)
		if inner.Class == ir.ImageClassStorage {
			b.requestImageFormatCapabilities(inner.StorageFormat)
		}

	case ir.AtomicType:
		// Atomic types in SPIR-V are just the underlying scalar type
		// The atomicity is expressed through OpAtomic* instructions
		if inner.Scalar.Width == 8 {
			b.addCapability(CapabilityInt64Atomics)
		}
		if inner.Scalar.Kind == ir.ScalarFloat && inner.Scalar.Width == 4 {
			b.addCapability(CapabilityAtomicFloat32AddEXT)
			b.addExtension("SPV_EXT_shader_atomic_float_add")
		}
		id = b.emitScalarType(inner.Scalar)

	case ir.BindingArrayType:
		baseID, err := b.emitType(inner.Base)
		if err != nil {
			return 0, err
		}
		if inner.Size != nil {
			// Fixed-size binding array
			u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			sizeID := b.builder.AddConstant(u32TypeID, *inner.Size)
			id = b.builder.AddTypeArray(baseID, sizeID)
		} else {
			// Unbounded binding array (runtime-sized)
			id = b.builder.AddTypeRuntimeArray(baseID)
		}

	case ir.AccelerationStructureType:
		// OpTypeAccelerationStructureKHR
		id = b.builder.AllocID()
		builder := b.newIB()
		builder.AddWord(id)
		b.builder.types = append(b.builder.types, builder.Build(OpTypeAccelerationStructureKHR))

	case ir.RayQueryType:
		// OpTypeRayQueryKHR
		id = b.builder.AllocID()
		builder := b.newIB()
		builder.AddWord(id)
		b.builder.types = append(b.builder.types, builder.Build(OpTypeRayQueryKHR))

	default:
		return 0, fmt.Errorf("unsupported type: %T", inner)
	}

	// Cache the result
	b.typeIDs[handle] = id
	return id, nil
}

// typeContainsRuntimeArray returns true if the given type handle refers to a
// runtime-sized array, or a struct whose last member is a runtime-sized array.
// SPIR-V forbids OpLoad on such types.
func (b *Backend) typeContainsRuntimeArray(handle ir.TypeHandle) bool {
	if int(handle) >= len(b.module.Types) {
		return false
	}
	inner := b.module.Types[handle].Inner
	switch t := inner.(type) {
	case ir.ArrayType:
		return t.Size.Constant == nil
	case ir.StructType:
		if len(t.Members) == 0 {
			return false
		}
		return b.typeContainsRuntimeArray(t.Members[len(t.Members)-1].Type)
	default:
		return false
	}
}

// emitTypeWithoutLayout emits a type suitable for Workgroup address space.
// Workgroup variables must NOT have explicit layout decorations (ArrayStride, Offset)
// per VUID-StandaloneSpirv-None-10684. If the type is an array (which normally gets
// ArrayStride), this creates a separate array type without the decoration.
// For non-array types, returns the normal emitted type.
func (b *Backend) emitTypeWithoutLayout(handle ir.TypeHandle) uint32 {
	// Check cache
	if id, ok := b.layoutFreeTypeIDs[handle]; ok {
		return id
	}

	typ := &b.module.Types[handle]
	var id uint32

	switch inner := typ.Inner.(type) {
	case ir.ArrayType:
		// Recursively emit base type without layout — handles nested structs/arrays.
		baseID := b.emitTypeWithoutLayout(inner.Base)

		if inner.Size.Constant != nil {
			// Fixed-size array without ArrayStride
			u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			sizeID := b.builder.AddConstant(u32TypeID, *inner.Size.Constant)
			id = b.builder.AddTypeArray(baseID, sizeID)
		} else {
			// Runtime-sized array (no ArrayStride needed)
			id = b.builder.AddTypeRuntimeArray(baseID)
		}

	case ir.StructType:
		// Struct types for Workgroup: emit members, but skip Offset decorations.
		memberIDs := make([]uint32, len(inner.Members))
		for i, member := range inner.Members {
			memberIDs[i] = b.emitTypeWithoutLayout(member.Type)
		}
		id = b.builder.AddTypeStruct(memberIDs...)

	default:
		// Non-array, non-struct types don't have layout decorations.
		id, _ = b.emitType(handle)
	}

	b.layoutFreeTypeIDs[handle] = id
	return id
}

// emitScalarType emits a scalar type and returns its SPIR-V ID.
// Uses cache to ensure type deduplication (SPIR-V requires unique types).
func (b *Backend) emitScalarType(scalar ir.ScalarType) uint32 {
	// Concretize abstract types — they should have been removed by compact,
	// but may survive in ExpressionTypes. AbstractInt→I32, AbstractFloat→F32.
	switch scalar.Kind {
	case ir.ScalarAbstractInt:
		scalar = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	case ir.ScalarAbstractFloat:
		scalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}

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
			b.addCapability(CapabilityStorageBuffer16BitAccess)
			b.addCapability(CapabilityUniformAndStorageBuffer16BitAccess)
			b.addExtension("SPV_KHR_16bit_storage")
			if b.options.UseStorageInputOutput16 {
				b.addCapability(CapabilityStorageInputOutput16)
			}
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

// emitMatrixType emits or returns cached matrix type.
func (b *Backend) emitMatrixType(columnTypeID uint32, columnCount uint32) uint32 {
	key := (columnTypeID << 8) | columnCount
	if id, ok := b.matrixTypeIDs[key]; ok {
		return id
	}
	id := b.builder.AddTypeMatrix(columnTypeID, columnCount)
	b.matrixTypeIDs[key] = id
	return id
}

// imageTypeKey creates a cache key for an image type.
func imageTypeKey(img ir.ImageType) uint64 {
	// Pack dimension (3 bits), arrayed (1 bit), multisampled (1 bit), class (3 bits),
	// storage format (8 bits), sampled scalar kind (4 bits).
	// Storage format is needed because different formats produce different OpTypeImage
	// instructions (different image format and potentially different sampled type).
	key := uint64(img.Dim) & 0x07
	if img.Arrayed {
		key |= 0x08
	}
	if img.Multisampled {
		key |= 0x10
	}
	key |= (uint64(img.Class) & 0x07) << 5
	if img.Class == ir.ImageClassStorage {
		key |= uint64(img.StorageFormat) << 8
	}
	if img.Class == ir.ImageClassSampled {
		key |= uint64(img.SampledKind) << 16
	}
	return key
}

// emitImageType emits OpTypeImage, with caching to avoid duplicates.
func (b *Backend) emitImageType(sampledTypeID uint32, img ir.ImageType) uint32 {
	// Check cache first
	cacheKey := imageTypeKey(img)
	if id, ok := b.imageTypeIDs[cacheKey]; ok {
		return id
	}

	// Add required capabilities based on image properties
	isSampled := img.Class == ir.ImageClassSampled || img.Class == ir.ImageClassDepth
	switch img.Dim {
	case ir.Dim1D:
		if isSampled {
			b.addCapability(CapabilitySampled1D)
		} else {
			b.addCapability(CapabilityImage1D)
		}
	case ir.DimCube:
		if img.Arrayed {
			if isSampled {
				b.addCapability(CapabilitySampledCubeArray)
			} else {
				b.addCapability(CapabilityImageCubeArray)
			}
		}
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
	return format.Scalar()
}

// requestImageFormatCapabilities adds StorageImageExtendedFormats capability
// for storage image formats that are not in the basic set.
// Basic formats (no extra capability needed): Rgba32f, Rgba16f, R32f,
// Rgba8, Rgba8Snorm, Rgba32i, Rgba16i, Rgba8i, R32i,
// Rgba32ui, Rgba16ui, Rgba8ui, R32ui.
func (b *Backend) requestImageFormatCapabilities(format ir.StorageFormat) {
	switch format {
	case ir.StorageFormatRgba32Float, ir.StorageFormatRgba16Float, ir.StorageFormatR32Float,
		ir.StorageFormatRgba8Unorm, ir.StorageFormatRgba8Snorm,
		ir.StorageFormatRgba32Sint, ir.StorageFormatRgba16Sint, ir.StorageFormatRgba8Sint, ir.StorageFormatR32Sint,
		ir.StorageFormatRgba32Uint, ir.StorageFormatRgba16Uint, ir.StorageFormatRgba8Uint, ir.StorageFormatR32Uint:
		// Basic formats — no extra capability needed
	case ir.StorageFormatR64Uint, ir.StorageFormatR64Sint:
		// 64-bit integer storage image formats require extension and capability
		b.addExtension("SPV_EXT_shader_image_int64")
		b.addCapability(CapabilityInt64ImageEXT)
	default:
		b.addCapability(CapabilityStorageImageExtendedFormats)
	}
}

// entryPointWritesFragDepth checks if a fragment shader entry point writes FragDepth.
// Vulkan requires ExecutionModeDepthReplacing when FragDepth is written.
func (b *Backend) entryPointWritesFragDepth(ep ir.EntryPoint) bool {
	fn := &ep.Function
	if fn.Result == nil {
		return false
	}

	// Check direct binding (scalar result with BuiltinBinding).
	if fn.Result.Binding != nil {
		if bb, ok := (*fn.Result.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinFragDepth {
			return true
		}
	}

	// Check struct members (result struct may contain FragDepth as a member).
	typeHandle := fn.Result.Type
	if int(typeHandle) < len(b.module.Types) {
		if st, ok := b.module.Types[typeHandle].Inner.(ir.StructType); ok {
			for _, member := range st.Members {
				if member.Binding == nil {
					continue
				}
				if bb, ok := (*member.Binding).(ir.BuiltinBinding); ok && bb.Builtin == ir.BuiltinFragDepth {
					return true
				}
			}
		}
	}

	return false
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
	case ir.SpaceImmediate:
		return StorageClassPushConstant
	case ir.SpaceTaskPayload:
		return StorageClassTaskPayloadWorkgroupEXT
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

	case ir.ZeroConstantValue:
		// Zero-initialized constant -> OpConstantNull
		id = b.builder.AddConstantNull(typeID)

	case nil:
		// Constant with no inline value — emit OpConstantNull as fallback.
		// In Rust naga, the init expression in GlobalExpressions would be used,
		// but our backend doesn't yet process GlobalExpressions fully.
		id = b.builder.AddConstantNull(typeID)

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
		scalarType := b.resolveScalarType(typeID)
		switch scalarType.Width {
		case 2:
			// 16-bit float: bits contain f16 bit pattern in low 16 bits,
			// stored in a single 32-bit literal word
			return b.builder.AddConstant(typeID, uint32(value.Bits)&0xFFFF)
		case 4:
			// 32-bit float
			return b.builder.AddConstantFloat32(typeID, math.Float32frombits(uint32(value.Bits)))
		default:
			// 64-bit float
			return b.builder.AddConstantFloat64(typeID, math.Float64frombits(value.Bits))
		}

	case ir.ScalarSint, ir.ScalarUint, ir.ScalarAbstractInt:
		// Concretize abstract int to i32 for SPIR-V
		if value.Kind == ir.ScalarAbstractInt {
			return b.builder.AddConstant(typeID, uint32(value.Bits))
		}
		// For integers, just pass the bits directly
		// Handle 64-bit integers (need two words)
		scalarType := b.resolveScalarType(typeID)
		if scalarType.Width == 8 {
			// 64-bit integer
			lowBits := uint32(value.Bits & 0xFFFFFFFF)
			highBits := uint32(value.Bits >> 32)
			return b.builder.AddConstant(typeID, lowBits, highBits)
		}
		// 32-bit or smaller integer
		return b.builder.AddConstant(typeID, uint32(value.Bits))

	case ir.ScalarAbstractFloat:
		// Concretize abstract float to f32 for SPIR-V
		return b.builder.AddConstantFloat32(typeID, math.Float32frombits(uint32(value.Bits)))

	default:
		panic(fmt.Sprintf("unknown scalar kind: %v", value.Kind))
	}
}

// resolveScalarType finds the ScalarType for a SPIR-V type ID, unwrapping AtomicType if needed.
func (b *Backend) resolveScalarType(typeID uint32) ir.ScalarType {
	handle := b.findTypeHandleByID(typeID)
	typ := &b.module.Types[handle]
	switch inner := typ.Inner.(type) {
	case ir.ScalarType:
		return inner
	case ir.AtomicType:
		return inner.Scalar
	default:
		panic(fmt.Sprintf("expected ScalarType or AtomicType, got %T", typ.Inner))
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

// OpTypeAccelerationStructureKHR represents OpTypeAccelerationStructureKHR.
const OpTypeAccelerationStructureKHR OpCode = 5341

// OpTypeRayQueryKHR represents OpTypeRayQueryKHR.
const OpTypeRayQueryKHR OpCode = 4472

// OpArrayLength gets the length of a runtime-sized array in a storage buffer struct.
// Result type must be u32. Operands: struct pointer, member index.
const OpArrayLength OpCode = 68

// OpCopyLogical performs a logical copy between composite types with different
// decorations (e.g., layout-free Workgroup type → decorated Storage type).
// Requires SPIR-V 1.4+. Both types must be logically equivalent (same structure).
const OpCopyLogical OpCode = 400

// emitGlobals emits all global variables to SPIR-V.
func (b *Backend) emitGlobals() error {
	for handle, global := range b.module.GlobalVariables {
		// Skip TaskPayload globals — mesh/task shaders not supported in SPIR-V (matches Rust naga)
		if global.Space == ir.SpaceTaskPayload {
			continue
		}

		// Get the variable type.
		// Workgroup variables must NOT have layout decorations (ArrayStride, Offset,
		// MatrixStride) per VUID-StandaloneSpirv-None-10684. Use layout-free types.
		var varType uint32
		if global.Space == ir.SpaceWorkGroup {
			varType = b.emitTypeWithoutLayout(global.Type)
		} else {
			var err error
			varType, err = b.emitType(global.Type)
			if err != nil {
				return err
			}
		}

		// Determine if this variable needs a wrapper struct (matching Rust naga's
		// global_needs_wrapper logic). In SPIR-V, Uniform/Storage variables must be
		// typed as OpTypeStruct with Block decoration.
		needsWrap := b.globalNeedsWrapper(global)
		if needsWrap {
			wrapperStruct := b.builder.AddTypeStruct(varType)
			b.builder.AddDecorate(wrapperStruct, DecorationBlock)
			b.builder.AddMemberDecorate(wrapperStruct, 0, DecorationOffset, 0)
			// Add matrix layout decorations if member is a matrix type
			b.addMatrixLayoutIfNeeded(wrapperStruct, 0, global.Type)
			varType = wrapperStruct
			b.wrappedStorageVars[ir.GlobalVariableHandle(handle)] = true
		} else if b.needsBlockDecoration(global.Space, global.Type) && !b.blockDecoratedTypes[varType] {
			// Struct that doesn't need wrapping (e.g., has dynamic array as last member)
			// still needs Block decoration directly.
			b.builder.AddDecorate(varType, DecorationBlock)
			b.blockDecoratedTypes[varType] = true
		} else if global.Space == ir.SpaceStorage {
			// For Storage BindingArray globals, the base type needs Block decoration.
			// Matches Rust naga writer.rs:2404-2428.
			if ba, ok := b.module.Types[global.Type].Inner.(ir.BindingArrayType); ok {
				shouldDecorate := true
				// Check if the base type is a struct with a runtime array as last member
				if st, ok := b.module.Types[ba.Base].Inner.(ir.StructType); ok {
					if len(st.Members) > 0 {
						lastMember := st.Members[len(st.Members)-1]
						if arr, ok := b.module.Types[lastMember.Type].Inner.(ir.ArrayType); ok {
							if arr.Size.Constant == nil { // Dynamic (runtime-sized)
								shouldDecorate = false
							}
						}
					}
				}
				if shouldDecorate {
					baseTypeID, err := b.emitType(ba.Base)
					if err != nil {
						return err
					}
					if !b.blockDecoratedTypes[baseTypeID] {
						b.builder.AddDecorate(baseTypeID, DecorationBlock)
						b.blockDecoratedTypes[baseTypeID] = true
					}
				}
			}
		}

		// Create pointer type for the variable
		storageClass := addressSpaceToStorageClass(global.Space)

		// StorageBuffer storage class is core in SPIR-V 1.3+.
		// For earlier versions, declare the required extension.
		// Matches Rust naga writer.rs:2580.
		if storageClass == StorageClassStorageBuffer && b.langVersion() < 0x00010300 {
			b.addExtension("SPV_KHR_storage_buffer_storage_class")
		}

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

		// Add NonReadable/NonWritable decorations for storage images and storage buffers.
		// SPIR-V requires these decorations to match the access mode.
		// Matches Rust naga: if !access.contains(LOAD) -> NonReadable,
		// if !access.contains(STORE) -> NonWritable.
		hasLoad, hasStore, applicable := b.getStorageAccessFlags(global)
		if applicable {
			if !hasLoad {
				b.builder.AddDecorate(varID, DecorationNonReadable)
			}
			if !hasStore {
				b.builder.AddDecorate(varID, DecorationNonWritable)
			}
		}
	}
	return nil
}

// getStorageAccessFlags returns (hasLoad, hasStore) for a global variable.
// Checks both Storage address space and storage image types.
// Matches Rust naga's write_global_variable logic for NonReadable/NonWritable.
func (b *Backend) getStorageAccessFlags(global ir.GlobalVariable) (hasLoad bool, hasStore bool, applicable bool) {
	// Check Storage address space first (mirrors Rust: Storage { access })
	if global.Space == ir.SpaceStorage {
		switch global.Access {
		case ir.StorageRead:
			return true, false, true // LOAD only
		case ir.StorageReadWrite:
			return true, true, true // LOAD | STORE
		default:
			return true, true, true // default read_write
		}
	}
	// Check storage image types
	if int(global.Type) < len(b.module.Types) {
		if img, ok := b.module.Types[global.Type].Inner.(ir.ImageType); ok {
			if img.Class == ir.ImageClassStorage {
				switch img.StorageAccess {
				case ir.StorageAccessRead:
					return true, false, true
				case ir.StorageAccessWrite:
					return false, true, true
				case ir.StorageAccessReadWrite:
					return true, true, true
				default:
					return true, true, true
				}
			}
		}
	}
	return false, false, false
}

// needsBlockDecoration returns true if a struct type in the given address space
// needs the Block decoration per Vulkan SPIR-V requirements.
func (b *Backend) needsBlockDecoration(space ir.AddressSpace, typeHandle ir.TypeHandle) bool {
	switch space {
	case ir.SpaceUniform, ir.SpaceStorage, ir.SpacePushConstant, ir.SpaceImmediate:
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

// globalNeedsWrapper determines if a global variable needs to be wrapped in a
// synthetic struct. Matches Rust naga's global_needs_wrapper logic.
// Uniform/Storage/Immediate variables need wrapping UNLESS they are:
// - A struct whose last member is a dynamically-sized array
// - A BindingArray type
func (b *Backend) globalNeedsWrapper(gv ir.GlobalVariable) bool {
	switch gv.Space {
	case ir.SpaceUniform, ir.SpaceStorage, ir.SpaceImmediate:
		// These spaces need wrapping
	default:
		return false
	}

	inner := b.module.Types[gv.Type].Inner
	switch t := inner.(type) {
	case ir.StructType:
		if len(t.Members) == 0 {
			return false
		}
		lastMember := t.Members[len(t.Members)-1]
		lastInner := b.module.Types[lastMember.Type].Inner
		if arr, ok := lastInner.(ir.ArrayType); ok {
			if arr.Size.Constant == nil { // Dynamic array (nil = runtime-sized)
				return false
			}
		}
		return true
	case ir.BindingArrayType:
		return false
	default:
		// Non-struct, non-binding-array types need wrapping
		return true
	}
}

// addMatrixLayoutIfNeeded adds ColMajor + MatrixStride decorations for a wrapper struct member
// if the member's IR type is a matrix (or array of matrices).
// Unwraps through array types to find inner matrices, matching Rust naga.
func (b *Backend) addMatrixLayoutIfNeeded(structID uint32, memberIdx uint32, typeHandle ir.TypeHandle) {
	inner := b.module.Types[typeHandle].Inner
	// Unwrap through arrays to find matrix type
	for {
		if arr, ok := inner.(ir.ArrayType); ok {
			inner = b.module.Types[arr.Base].Inner
		} else {
			break
		}
	}
	if mat, ok := inner.(ir.MatrixType); ok {
		b.builder.AddMemberDecorate(structID, memberIdx, DecorationColMajor)
		// Column stride: each column is a vector, stride = alignment of that vector.
		// vec2 stride = 2*width, vec3/vec4 stride = 4*width (vec3 padded to vec4 alignment).
		// Matches WGSL spec and Rust naga MatrixStride decoration.
		var rowMultiplier uint32
		switch mat.Rows {
		case ir.Vec2:
			rowMultiplier = 2
		default: // Vec3, Vec4
			rowMultiplier = 4
		}
		stride := rowMultiplier * uint32(mat.Scalar.Width)
		b.builder.AddMemberDecorate(structID, memberIdx, DecorationMatrixStride, stride)
	}
}

// emitEntryPointInterfaceVars creates input/output variables for entry point builtins and locations.
// In SPIR-V, entry point functions don't receive builtins as parameters.
// Instead, builtins are global variables with Input/Output storage class.
func (b *Backend) emitEntryPointInterfaceVars() error {
	for epIdx := range b.module.EntryPoints {
		entryPoint := &b.module.EntryPoints[epIdx]
		// Rust naga does not support mesh/task shaders in SPIR-V backend
		// (marked unreachable! in write_entry_point). Skip them gracefully.
		if entryPoint.Stage == ir.StageTask || entryPoint.Stage == ir.StageMesh {
			continue
		}
		fn := &entryPoint.Function
		inputVars := make([]*entryPointInput, len(fn.Arguments))

		// Create input variables for function arguments
		for i, arg := range fn.Arguments {
			argTypeID, err := b.emitType(arg.Type)
			if err != nil {
				return err
			}

			input := &entryPointInput{
				typeID: argTypeID,
			}

			if arg.Binding != nil {
				// Simple case: argument has direct binding (scalar/vector/builtin)
				// F16 polyfill: use f32 types for the variable declaration
				ioTypeID := argTypeID
				if b.needsF16Polyfill(arg.Type) {
					ioTypeID = b.getF16PolyfillTypeID(arg.Type)
				}
				// SampleMask BuiltIn must be array<u32, 1> per Vulkan spec.
				if isSampleMaskBinding(arg.Binding) {
					ioTypeID = b.emitSampleMaskArrayType()
				}
				ptrType := b.emitPointerType(StorageClassInput, ioTypeID)
				varID := b.builder.AddVariable(ptrType, StorageClassInput)
				input.singleVarID = varID
				if isSampleMaskBinding(arg.Binding) {
					b.sampleMaskVars[varID] = true
				}
				if ioTypeID != argTypeID && !isSampleMaskBinding(arg.Binding) {
					b.f16PolyfillVars[varID] = ioTypeID
				}

				switch binding := (*arg.Binding).(type) {
				case ir.BuiltinBinding:
					b.addBuiltinCapabilities(binding.Builtin)
					spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassInput)
					b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
					// Per Vulkan VUID-StandaloneSpirv-Flat-04744:
					// Integer/bool Input variables in fragment shaders must be Flat.
					if entryPoint.Stage == ir.StageFragment {
						if b.typeNeedsFlat(arg.Type) {
							b.builder.AddDecorate(varID, DecorationFlat)
						}
					}
				case ir.LocationBinding:
					b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
					b.addInterpolationDecorations(varID, binding, entryPoint.Stage, StorageClassInput)
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
						input.memberVarIDs = make([]uint32, len(structType.Members))
						// Create separate Input variable for each member with a binding
						for j, member := range structType.Members {
							if member.Binding == nil {
								continue
							}
							memberTypeID, err := b.emitType(member.Type)
							if err != nil {
								return err
							}
							// F16 polyfill: use f32 types for the variable declaration
							ioMemberTypeID := memberTypeID
							if b.needsF16Polyfill(member.Type) {
								ioMemberTypeID = b.getF16PolyfillTypeID(member.Type)
							}
							// SampleMask BuiltIn must be array<u32, 1> per Vulkan spec.
							isSM := isSampleMaskBinding(member.Binding)
							if isSM {
								ioMemberTypeID = b.emitSampleMaskArrayType()
							}
							ptrType := b.emitPointerType(StorageClassInput, ioMemberTypeID)
							varID := b.builder.AddVariable(ptrType, StorageClassInput)
							input.memberVarIDs[j] = varID
							if isSM {
								b.sampleMaskVars[varID] = true
							}
							if ioMemberTypeID != memberTypeID && !isSM {
								b.f16PolyfillVars[varID] = ioMemberTypeID
							}

							if b.options.Debug && member.Name != "" {
								b.builder.AddName(varID, member.Name)
							}

							switch binding := (*member.Binding).(type) {
							case ir.BuiltinBinding:
								b.addBuiltinCapabilities(binding.Builtin)
								spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassInput)
								b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
								// Per Vulkan VUID-StandaloneSpirv-Flat-04744:
								// Integer/bool Input variables in fragment shaders must be Flat.
								if entryPoint.Stage == ir.StageFragment {
									if b.typeNeedsFlat(member.Type) {
										b.builder.AddDecorate(varID, DecorationFlat)
									}
								}
							case ir.LocationBinding:
								b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
								b.addInterpolationDecorations(varID, binding, entryPoint.Stage, StorageClassInput)
							}
						}
					}
				}
				// If not a struct with bindings, input.singleVarID stays 0 (no input var needed)
			}

			inputVars[i] = input
		}

		b.entryInputVars[epIdx] = inputVars

		// Create output variables for function result
		if fn.Result != nil {
			resultTypeID, err := b.emitType(fn.Result.Type)
			if err != nil {
				return err
			}

			output := &entryPointOutput{
				resultTypeID: resultTypeID,
			}

			if fn.Result.Binding != nil {
				// Simple output with a single binding
				// F16 polyfill: use f32 types for the variable declaration
				ioResultTypeID := resultTypeID
				if b.needsF16Polyfill(fn.Result.Type) {
					ioResultTypeID = b.getF16PolyfillTypeID(fn.Result.Type)
				}
				// SampleMask BuiltIn must be array<u32, 1> per Vulkan spec.
				if isSampleMaskBinding(fn.Result.Binding) {
					ioResultTypeID = b.emitSampleMaskArrayType()
				}
				ptrType := b.emitPointerType(StorageClassOutput, ioResultTypeID)
				varID := b.builder.AddVariable(ptrType, StorageClassOutput)
				output.singleVarID = varID
				if isSampleMaskBinding(fn.Result.Binding) {
					b.sampleMaskVars[varID] = true
				}
				if ioResultTypeID != resultTypeID && !isSampleMaskBinding(fn.Result.Binding) {
					b.f16PolyfillVars[varID] = ioResultTypeID
				}

				switch binding := (*fn.Result.Binding).(type) {
				case ir.BuiltinBinding:
					b.addBuiltinCapabilities(binding.Builtin)
					spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassOutput)
					b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
				case ir.LocationBinding:
					b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
					b.addInterpolationDecorations(varID, binding, entryPoint.Stage, StorageClassOutput)
				}
			} else {
				// Check if it's a struct with member bindings
				resultType := b.module.Types[fn.Result.Type].Inner
				if structType, ok := resultType.(ir.StructType); ok {
					output.isStruct = true
					output.memberVarIDs = make([]uint32, len(structType.Members))
					// Create separate output variable for each member with a binding
					for i, member := range structType.Members {
						if member.Binding == nil {
							continue
						}
						memberTypeID, err := b.emitType(member.Type)
						if err != nil {
							return err
						}
						// F16 polyfill: use f32 types for the variable declaration
						ioMemberTypeID := memberTypeID
						if b.needsF16Polyfill(member.Type) {
							ioMemberTypeID = b.getF16PolyfillTypeID(member.Type)
						}
						// SampleMask BuiltIn must be array<u32, 1> per Vulkan spec.
						isSM := isSampleMaskBinding(member.Binding)
						if isSM {
							ioMemberTypeID = b.emitSampleMaskArrayType()
						}
						ptrType := b.emitPointerType(StorageClassOutput, ioMemberTypeID)
						varID := b.builder.AddVariable(ptrType, StorageClassOutput)
						output.memberVarIDs[i] = varID
						if isSM {
							b.sampleMaskVars[varID] = true
						}
						if ioMemberTypeID != memberTypeID && !isSM {
							b.f16PolyfillVars[varID] = ioMemberTypeID
						}

						// Add debug name if enabled
						if b.options.Debug && member.Name != "" {
							b.builder.AddName(varID, member.Name)
						}

						// Decorate based on binding type
						switch binding := (*member.Binding).(type) {
						case ir.BuiltinBinding:
							b.addBuiltinCapabilities(binding.Builtin)
							spirvBuiltin := builtinToSPIRV(binding.Builtin, StorageClassOutput)
							b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(spirvBuiltin))
						case ir.LocationBinding:
							b.builder.AddDecorate(varID, DecorationLocation, binding.Location)
							b.addInterpolationDecorations(varID, binding, entryPoint.Stage, StorageClassOutput)
						}
					}
				}
			}

			b.entryOutputVars[epIdx] = output
		}

		// ForcePointSize: add BuiltIn PointSize output variable for vertex shaders
		// that don't already have one. This writes 1.0 to gl_PointSize.
		if b.options.ForcePointSize && entryPoint.Stage == ir.StageVertex {
			hasPointSize := false
			// Check if any output already has PointSize builtin
			if fn.Result != nil && fn.Result.Binding != nil {
				if bb, ok := (*fn.Result.Binding).(ir.BuiltinBinding); ok {
					if bb.Builtin == ir.BuiltinPointSize {
						hasPointSize = true
					}
				}
			}
			if !hasPointSize && fn.Result != nil {
				resultType := b.module.Types[fn.Result.Type].Inner
				if structType, ok := resultType.(ir.StructType); ok {
					for _, member := range structType.Members {
						if member.Binding != nil {
							if bb, ok := (*member.Binding).(ir.BuiltinBinding); ok {
								if bb.Builtin == ir.BuiltinPointSize {
									hasPointSize = true
									break
								}
							}
						}
					}
				}
			}
			if !hasPointSize {
				// Create f32 Output pointer variable for PointSize
				f32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
				ptrType := b.emitPointerType(StorageClassOutput, f32TypeID)
				varID := b.builder.AddVariable(ptrType, StorageClassOutput)
				b.builder.AddDecorate(varID, DecorationBuiltIn, uint32(BuiltInPointSize))
				b.forcePointSizeVars[epIdx] = varID
			}
		}
	}
	return nil
}

// addInterpolationDecorations adds interpolation-related decorations for a location binding.
// Per Vulkan VUIDs, Flat/NoPerspective/Sample/Centroid decorations must NOT be used on:
// - Input variables in vertex shaders (VUID-StandaloneSpirv-Flat-06202)
// - Output variables in fragment shaders (VUID-StandaloneSpirv-Flat-06201)
func (b *Backend) addInterpolationDecorations(varID uint32, loc ir.LocationBinding, stage ir.ShaderStage, class StorageClass) {
	// Per Vulkan VUIDs, skip interpolation decorations in these cases
	noDecorations := (class == StorageClassInput && stage == ir.StageVertex) ||
		(class == StorageClassOutput && stage == ir.StageFragment)

	if !noDecorations && loc.Interpolation != nil {
		// Interpolation kind
		switch loc.Interpolation.Kind {
		case ir.InterpolationFlat:
			b.builder.AddDecorate(varID, DecorationFlat)
		case ir.InterpolationLinear:
			b.builder.AddDecorate(varID, DecorationNoPerspective)
		case ir.InterpolationPerspective:
			// Perspective is the default, no decoration needed
		}

		// Sampling
		switch loc.Interpolation.Sampling {
		case ir.SamplingCentroid:
			b.builder.AddDecorate(varID, DecorationCentroid)
		case ir.SamplingSample:
			b.addCapability(CapabilitySampleRateShading)
			b.builder.AddDecorate(varID, DecorationSample)
		case ir.SamplingCenter:
			// Center is the default, no decoration needed
		}
	}

	// Dual-source blending
	if loc.BlendSrc != nil {
		b.builder.AddDecorate(varID, DecorationIndex, *loc.BlendSrc)
	}
}

// typeNeedsFlat returns true if the type is integer or bool (Scalar or Vector of Sint/Uint/Bool).
// Per Vulkan VUID-StandaloneSpirv-Flat-04744, such Input variables in fragment shaders must be Flat.
// Matches Rust naga's logic in write_varying (writer.rs ~line 2238-2256).
func (b *Backend) typeNeedsFlat(typeHandle ir.TypeHandle) bool {
	inner := b.module.Types[typeHandle].Inner
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind == ir.ScalarUint || t.Kind == ir.ScalarSint || t.Kind == ir.ScalarBool
	case ir.VectorType:
		return t.Scalar.Kind == ir.ScalarUint || t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarBool
	default:
		return false
	}
}

// isSampleMaskBinding returns true if the binding is a BuiltinSampleMask builtin.
func isSampleMaskBinding(binding *ir.Binding) bool {
	if binding == nil {
		return false
	}
	if bb, ok := (*binding).(ir.BuiltinBinding); ok {
		return bb.Builtin == ir.BuiltinSampleMask
	}
	return false
}

// emitSampleMaskArrayType returns the SPIR-V type ID for array<u32, 1>,
// which is required for SampleMask BuiltIn variables per the Vulkan spec.
func (b *Backend) emitSampleMaskArrayType() uint32 {
	u32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	sizeID := b.builder.AddConstant(u32TypeID, 1)
	return b.builder.AddTypeArray(u32TypeID, sizeID)
}

// addBuiltinCapabilities adds required capabilities for specific builtins.
func (b *Backend) addBuiltinCapabilities(builtin ir.BuiltinValue) {
	switch builtin {
	case ir.BuiltinSampleIndex, ir.BuiltinSampleMask:
		b.addCapability(CapabilitySampleRateShading)
	case ir.BuiltinViewIndex:
		b.addCapability(CapabilityMultiView)
		b.addExtension("SPV_KHR_multiview")
	case ir.BuiltinBarycentric:
		b.addCapability(CapabilityFragmentBarycentricKHR)
		b.addExtension("SPV_KHR_fragment_shader_barycentric")
	case ir.BuiltinClipDistance:
		b.addCapability(CapabilityClipDistance)
	case ir.BuiltinPrimitiveIndex:
		b.addCapability(CapabilityGeometry)
	case ir.BuiltinNumSubgroups, ir.BuiltinSubgroupID,
		ir.BuiltinSubgroupSize, ir.BuiltinSubgroupInvocationID:
		// Rust require_any picks first from [GroupNonUniform, SubgroupBallotKHR]
		// when capabilities_available is None (default), so only GroupNonUniform.
		b.addCapability(CapabilityGroupNonUniform)
		b.requireVersion(Version1_3)
	}
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
	case ir.BuiltinNumSubgroups:
		return BuiltInNumSubgroups
	case ir.BuiltinSubgroupID:
		return BuiltInSubgroupID
	case ir.BuiltinSubgroupSize:
		return BuiltInSubgroupSize
	case ir.BuiltinSubgroupInvocationID:
		return BuiltInSubgroupLocalInvID
	case ir.BuiltinClipDistance:
		return BuiltInClipDistance
	case ir.BuiltinPrimitiveIndex:
		return BuiltInPrimitiveID
	case ir.BuiltinBarycentric:
		return BuiltInBaryCoordKHR
	case ir.BuiltinViewIndex:
		return BuiltInViewIndex
	default:
		return BuiltInPosition // Fallback
	}
}

// emitEntryPoints emits all entry points with their execution modes.
func (b *Backend) emitEntryPoints() error {
	for epIdx, entryPoint := range b.module.EntryPoints {
		// Skip mesh/task entry points — not supported in SPIR-V (matches Rust naga)
		if entryPoint.Stage == ir.StageTask || entryPoint.Stage == ir.StageMesh {
			continue
		}

		// Get function ID (entry point functions have their own ID map)
		funcID, ok := b.entryPointFuncIDs[epIdx]
		if !ok {
			return fmt.Errorf("entry point function not found: %s", entryPoint.Name)
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

		// SPIR-V 1.3 (Vulkan 1.2): Only Input/Output storage classes are
		// allowed in the entry point interface list. Global variables in other
		// storage classes (Uniform, StorageBuffer, WorkGroup, PushConstant, etc.)
		// are NOT listed. Input/Output builtins are handled separately below.

		// Add entry point input variables (builtins, locations)
		if inputVars, ok := b.entryInputVars[epIdx]; ok {
			for _, input := range inputVars {
				if input == nil {
					continue
				}
				if input.isStruct {
					// Struct input: add all member variables
					for _, varID := range input.memberVarIDs {
						if varID == 0 {
							continue
						}
						interfaces = append(interfaces, varID)
					}
				} else if input.singleVarID != 0 {
					interfaces = append(interfaces, input.singleVarID)
				}
			}
		}

		// Add entry point output variables
		if output, ok := b.entryOutputVars[epIdx]; ok {
			if output.isStruct {
				// Struct output: add all member variables
				for _, varID := range output.memberVarIDs {
					if varID == 0 {
						continue
					}
					interfaces = append(interfaces, varID)
				}
			} else if output.singleVarID != 0 {
				// Single output variable
				interfaces = append(interfaces, output.singleVarID)
			}
		}

		// For SPIR-V 1.4+, add ALL used global variables to the interface.
		// Per the SPIR-V spec, version 1.4 requires all global variables
		// (not just Input/Output) to be listed in OpEntryPoint.
		spvVersionWord := (uint32(b.options.Version.Major) << 16) | (uint32(b.options.Version.Minor) << 8)
		if spvVersionWord >= 0x10400 {
			usedGlobals := b.collectUsedGlobalVars(&entryPoint.Function)
			for _, gvHandle := range usedGlobals {
				if varID, ok := b.globalIDs[gvHandle]; ok {
					interfaces = append(interfaces, varID)
				}
			}
		}

		// Add force_point_size output variable to interface if present
		if psVarID, ok := b.forcePointSizeVars[epIdx]; ok {
			interfaces = append(interfaces, psVarID)
		}

		// Add workgroup init polyfill LocalInvocationId variable to interface
		if wgVarID, ok := b.workgroupInitVars[epIdx]; ok {
			interfaces = append(interfaces, wgVarID)
		}

		// Add entry point
		b.builder.AddEntryPoint(execModel, funcID, entryPoint.Name, interfaces)

		// Add execution modes based on stage
		switch entryPoint.Stage {
		case ir.StageFragment:
			// Fragment shaders need OriginUpperLeft
			b.builder.AddExecutionMode(funcID, ExecutionModeOriginUpperLeft)
			// DepthReplacing is required when fragment shader writes FragDepth.
			// Matches Rust naga writer.rs execution mode emission.
			if b.entryPointWritesFragDepth(entryPoint) {
				b.builder.AddExecutionMode(funcID, ExecutionModeDepthReplacing)
			}

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

// collectUsedGlobalVars scans a function's expressions for ExprGlobalVariable
// references and returns the set of used global variable handles. This also
// transitively scans called functions via Statement::Call.
func (b *Backend) collectUsedGlobalVars(fn *ir.Function) []ir.GlobalVariableHandle {
	seen := make(map[ir.GlobalVariableHandle]bool)
	b.collectGlobalVarsFromFunction(fn, seen, make(map[ir.FunctionHandle]bool))

	result := make([]ir.GlobalVariableHandle, 0, len(seen))
	for h := range seen {
		result = append(result, h)
	}
	// Sort for deterministic output (by handle index)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// collectGlobalVarsFromFunction scans a function for ExprGlobalVariable references
// and recursively scans called functions.
func (b *Backend) collectGlobalVarsFromFunction(fn *ir.Function, seen map[ir.GlobalVariableHandle]bool, visitedFuncs map[ir.FunctionHandle]bool) {
	// Scan all expressions for GlobalVariable references
	for _, expr := range fn.Expressions {
		if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
			seen[gv.Variable] = true
		}
	}

	// Scan statements for function calls and recurse
	b.collectGlobalVarsFromStatements(fn.Body, seen, visitedFuncs)
}

// collectGlobalVarsFromStatements recursively scans statements for Call statements
// and collects global variables from called functions.
func (b *Backend) collectGlobalVarsFromStatements(stmts []ir.Statement, seen map[ir.GlobalVariableHandle]bool, visitedFuncs map[ir.FunctionHandle]bool) {
	for _, stmt := range stmts {
		switch s := stmt.Kind.(type) {
		case ir.StmtCall:
			if !visitedFuncs[s.Function] {
				visitedFuncs[s.Function] = true
				if int(s.Function) < len(b.module.Functions) {
					calledFn := &b.module.Functions[s.Function]
					b.collectGlobalVarsFromFunction(calledFn, seen, visitedFuncs)
				}
			}
		case ir.StmtBlock:
			b.collectGlobalVarsFromStatements(s.Block, seen, visitedFuncs)
		case ir.StmtIf:
			b.collectGlobalVarsFromStatements(s.Accept, seen, visitedFuncs)
			b.collectGlobalVarsFromStatements(s.Reject, seen, visitedFuncs)
		case ir.StmtSwitch:
			for _, c := range s.Cases {
				b.collectGlobalVarsFromStatements(c.Body, seen, visitedFuncs)
			}
		case ir.StmtLoop:
			b.collectGlobalVarsFromStatements(s.Body, seen, visitedFuncs)
			b.collectGlobalVarsFromStatements(s.Continuing, seen, visitedFuncs)
		}
	}
}

// emitWorkgroupInitPolyfill generates the zero-initialization polyfill for workgroup
// variables in compute shader entry points. It matches Rust naga's
// ZeroInitializeWorkgroupMemoryMode::Polyfill behavior.
//
// The generated code:
//  1. If local_invocation_id == (0,0,0): store zero to all workgroup variables
//  2. Memory/control barrier (WorkGroup scope, WorkgroupMemory semantics)
func (b *Backend) emitWorkgroupInitPolyfill(epIdx int, fn *ir.Function, emitter *ExpressionEmitter, fb *FunctionBuilder) {
	// Find workgroup global variables used by this entry point
	type wgVar struct {
		varID  uint32
		typeID uint32
	}
	var workgroupVars []wgVar
	usedGlobals := b.collectUsedGlobalVars(fn)
	for _, gvHandle := range usedGlobals {
		gv := b.module.GlobalVariables[gvHandle]
		if gv.Space == ir.SpaceWorkGroup {
			varID, ok := b.globalIDs[gvHandle]
			if !ok {
				continue
			}
			// Get the type of the variable (not the pointer type, the actual type).
			// Use layout-free type for Workgroup — must match what emitGlobals used.
			typeID := b.emitTypeWithoutLayout(gv.Type)
			workgroupVars = append(workgroupVars, wgVar{varID: varID, typeID: typeID})
		}
	}

	if len(workgroupVars) == 0 {
		return
	}

	// Check if the entry point already has a LocalInvocationId argument
	var localInvocID uint32 // the loaded vec3<u32> value
	hasLocalInvocIDArg := false
	u32Type := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	vec3uType := b.emitInlineType(ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}})

	for i, arg := range fn.Arguments {
		if arg.Binding != nil {
			if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				if bb.Builtin == ir.BuiltinLocalInvocationID {
					hasLocalInvocIDArg = true
					// Load from the corresponding input variable
					inputVars := b.entryInputVars[epIdx]
					if i < len(inputVars) && inputVars[i] != nil && inputVars[i].singleVarID != 0 {
						localInvocID = b.builder.AddLoad(vec3uType, inputVars[i].singleVarID)
					}
					break
				}
			}
		}
	}

	if !hasLocalInvocIDArg {
		// Create a new Input variable for LocalInvocationId
		ptrType := b.emitPointerType(StorageClassInput, vec3uType)
		varyingID := b.builder.AddVariable(ptrType, StorageClassInput)
		b.builder.AddDecorate(varyingID, DecorationBuiltIn, uint32(BuiltInLocalInvocationID))

		// Track this variable for the entry point interface
		if _, ok := b.workgroupInitVars[epIdx]; !ok {
			b.workgroupInitVars[epIdx] = varyingID
		}

		// Load the value
		localInvocID = b.builder.AddLoad(vec3uType, varyingID)
	}

	if localInvocID == 0 {
		return
	}

	// Generate: if (all(local_invocation_id == vec3(0))) { zero-init } barrier
	_ = u32Type

	// vec3<bool> type
	boolType := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	vec3boolType := b.emitInlineType(ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}})

	// zero vec3<u32>
	zeroVec3u := b.builder.AddConstantNull(vec3uType)

	// IEqual: local_invocation_id == vec3(0)
	eqID := b.builder.AllocID()
	{
		ib := b.newIB()
		ib.AddWord(vec3boolType)
		ib.AddWord(eqID)
		ib.AddWord(localInvocID)
		ib.AddWord(zeroVec3u)
		emitter.currentBlock.Body = append(emitter.currentBlock.Body, ib.Build(OpIEqual))
	}

	// All: reduce vec3<bool> to bool
	condID := b.builder.AllocID()
	{
		ib := b.newIB()
		ib.AddWord(boolType)
		ib.AddWord(condID)
		ib.AddWord(eqID)
		emitter.currentBlock.Body = append(emitter.currentBlock.Body, ib.Build(OpAll))
	}

	// Allocate block IDs
	mergeBlockID := b.builder.AllocID()
	acceptBlockID := b.builder.AllocID()

	// OpSelectionMerge
	{
		ib := b.newIB()
		ib.AddWord(mergeBlockID)
		ib.AddWord(0) // SelectionControl::None
		emitter.currentBlock.Body = append(emitter.currentBlock.Body, ib.Build(OpSelectionMerge))
	}

	// Consume current block with OpBranchConditional
	{
		ib := b.newIB()
		ib.AddWord(condID)
		ib.AddWord(acceptBlockID)
		ib.AddWord(mergeBlockID)
		terminator := ib.Build(OpBranchConditional)
		fb.Consume(*emitter.currentBlock, terminator)
		emitter.currentBlock = nil
		b.builder.funcSink = nil
	}

	// Accept block: store zero to all workgroup variables
	acceptBlock := NewBlock(acceptBlockID)
	for _, wv := range workgroupVars {
		nullID := b.builder.AddConstantNull(wv.typeID)
		storeIB := b.newIB()
		storeIB.AddWord(wv.varID)
		storeIB.AddWord(nullID)
		acceptBlock.Body = append(acceptBlock.Body, storeIB.Build(OpStore))
	}
	{
		ib := b.newIB()
		ib.AddWord(mergeBlockID)
		fb.Consume(acceptBlock, ib.Build(OpBranch))
	}

	// Merge block: control barrier + branch to main body
	mergeBlock := NewBlock(mergeBlockID)

	// OpControlBarrier(Workgroup, Workgroup, WorkgroupMemory | AcquireRelease)
	// Scope: Workgroup = 2
	// Memory semantics: WorkgroupMemory (0x100) | AcquireRelease (0x8) = 0x108
	workgroupScopeID := b.builder.AddConstant(u32Type, 2) // Scope::Workgroup
	semanticsID := b.builder.AddConstant(u32Type, 0x108)  // WorkgroupMemory | AcquireRelease
	{
		ib := b.newIB()
		ib.AddWord(workgroupScopeID) // execution scope
		ib.AddWord(workgroupScopeID) // memory scope
		ib.AddWord(semanticsID)      // memory semantics
		mergeBlock.Body = append(mergeBlock.Body, ib.Build(OpControlBarrier))
	}

	// Branch to next block (which will be the main body)
	mainBodyID := b.builder.AllocID()
	{
		ib := b.newIB()
		ib.AddWord(mainBodyID)
		fb.Consume(mergeBlock, ib.Build(OpBranch))
	}

	// Set up the new current block for the main body
	mainBodyBlock := NewBlock(mainBodyID)
	emitter.setCurrentBlock(&mainBodyBlock)
}

// emitWrappedFunctions scans a function for integer div/mod expressions
// and emits wrapper helper functions (naga_div, naga_mod) with safety checks.
// Matches Rust naga's write_wrapped_functions behavior.
func (b *Backend) emitWrappedFunctions(fn *ir.Function) error {
	for _, expr := range fn.Expressions {
		binary, ok := expr.Kind.(ir.ExprBinary)
		if !ok {
			continue
		}
		if binary.Op != ir.BinaryDivide && binary.Op != ir.BinaryModulo {
			continue
		}
		// Resolve left operand type to get scalar kind
		leftType, err := ir.ResolveExpressionType(b.module, fn, binary.Left)
		if err != nil {
			continue
		}
		rightType, err := ir.ResolveExpressionType(b.module, fn, binary.Right)
		if err != nil {
			continue
		}
		var scalarKind ir.ScalarKind
		leftInner := typeResolutionInner(b.module, leftType)
		switch t := leftInner.(type) {
		case ir.ScalarType:
			scalarKind = t.Kind
		case ir.VectorType:
			scalarKind = t.Scalar.Kind
		default:
			continue
		}
		if scalarKind != ir.ScalarSint && scalarKind != ir.ScalarUint {
			continue // float div/mod doesn't need wrappers
		}
		leftTypeID := b.resolveTypeResolution(leftType)
		rightTypeID := b.resolveTypeResolution(rightType)
		if err := b.emitWrappedBinaryOp(binary.Op, leftInner, leftTypeID, rightTypeID); err != nil {
			return err
		}
	}
	return nil
}

// emitWrappedBinaryOp emits a single wrapped binary operation helper function.
// The function protects against division by zero and signed overflow (INT_MIN / -1).
// Matches Rust naga's write_wrapped_binary_op.
func (b *Backend) emitWrappedBinaryOp(op ir.BinaryOperator, returnTypeInner ir.TypeInner, leftTypeID, rightTypeID uint32) error {
	// Return type ID is the same as left type ID for div/mod
	returnTypeID := leftTypeID

	// Check dedup cache
	key := wrappedBinaryOp{op: op, leftTypeID: leftTypeID, rightTypeID: rightTypeID}
	if _, exists := b.wrappedFuncIDs[key]; exists {
		return nil
	}

	// Extract scalar info
	var scalar ir.ScalarType
	var isVector bool
	var vecSize uint32
	switch t := returnTypeInner.(type) {
	case ir.ScalarType:
		scalar = t
	case ir.VectorType:
		scalar = t.Scalar
		isVector = true
		vecSize = uint32(t.Size)
	default:
		return fmt.Errorf("emitWrappedBinaryOp: unsupported type %T", returnTypeInner)
	}

	// Allocate the function ID and register in cache
	functionID := b.builder.AllocID()
	b.wrappedFuncIDs[key] = functionID

	// Debug name
	if b.options.Debug {
		var name string
		if op == ir.BinaryDivide {
			name = "naga_div"
		} else {
			name = "naga_mod"
		}
		b.builder.AddName(functionID, name)
	}

	// Build the function using FunctionBuilder for consistent output
	var fb FunctionBuilder

	// Function type
	funcTypeID := b.getFuncType(returnTypeID, []uint32{leftTypeID, rightTypeID})
	// OpFunction signature
	fb.Signature = Instruction{
		Opcode: OpFunction,
		Words:  []uint32{returnTypeID, functionID, uint32(FunctionControlNone), funcTypeID},
	}

	// Parameters
	lhsID := b.builder.AllocID()
	rhsID := b.builder.AllocID()
	if b.options.Debug {
		b.builder.AddName(lhsID, "lhs")
		b.builder.AddName(rhsID, "rhs")
	}
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{leftTypeID, lhsID}},
		{Opcode: OpFunctionParameter, Words: []uint32{rightTypeID, rhsID}},
	}

	// Entry block
	labelID := b.builder.AllocID()
	block := NewBlock(labelID)

	// Bool type (scalar or vector<bool> matching the return type shape)
	boolScalar := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	var boolTypeID uint32
	if isVector {
		boolScalarID := b.emitScalarType(boolScalar)
		boolTypeID = b.emitVectorType(boolScalarID, vecSize)
	} else {
		boolTypeID = b.emitScalarType(boolScalar)
	}

	// Helper: splat a scalar constant to a vector if needed
	maybeSplat := func(scalarConstID uint32) uint32 {
		if !isVector {
			return scalarConstID
		}
		constituents := make([]uint32, vecSize)
		for i := range constituents {
			constituents[i] = scalarConstID
		}
		return b.builder.AddConstantComposite(returnTypeID, constituents...)
	}

	// const 0
	constZeroID := b.builder.AddConstant(b.emitScalarType(scalar), 0)
	compositeZeroID := maybeSplat(constZeroID)

	// rhs == 0
	rhsEqZeroID := b.builder.AllocID()
	block.Body = append(block.Body, Instruction{
		Opcode: OpIEqual,
		Words:  []uint32{boolTypeID, rhsEqZeroID, rhsID, compositeZeroID},
	})

	// Determine the divisor selector (condition for replacing rhs with 1)
	var divisorSelectorID uint32
	if scalar.Kind == ir.ScalarSint {
		// Signed: also check for overflow (lhs == INT_MIN && rhs == -1)
		var constMinID, constNegOneID uint32
		scalarTypeID := b.emitScalarType(scalar)
		if scalar.Width == 4 {
			// i32: INT_MIN = 0x80000000
			constMinID = b.builder.AddConstant(scalarTypeID, uint32(0x80000000))
			constNegOneID = b.builder.AddConstant(scalarTypeID, uint32(0xFFFFFFFF))
		} else if scalar.Width == 8 {
			// i64: INT_MIN = 0x8000000000000000
			constMinID = b.builder.AddConstant(scalarTypeID, 0, 0x80000000)
			constNegOneID = b.builder.AddConstant(scalarTypeID, 0xFFFFFFFF, 0xFFFFFFFF)
		} else {
			return fmt.Errorf("emitWrappedBinaryOp: unsupported scalar width %d", scalar.Width)
		}
		compositeMinID := maybeSplat(constMinID)
		compositeNegOneID := maybeSplat(constNegOneID)

		// lhs == INT_MIN
		lhsEqMinID := b.builder.AllocID()
		block.Body = append(block.Body, Instruction{
			Opcode: OpIEqual,
			Words:  []uint32{boolTypeID, lhsEqMinID, lhsID, compositeMinID},
		})

		// rhs == -1
		rhsEqNegOneID := b.builder.AllocID()
		block.Body = append(block.Body, Instruction{
			Opcode: OpIEqual,
			Words:  []uint32{boolTypeID, rhsEqNegOneID, rhsID, compositeNegOneID},
		})

		// lhs == INT_MIN && rhs == -1
		overflowID := b.builder.AllocID()
		block.Body = append(block.Body, Instruction{
			Opcode: OpLogicalAnd,
			Words:  []uint32{boolTypeID, overflowID, lhsEqMinID, rhsEqNegOneID},
		})

		// rhs == 0 || (lhs == INT_MIN && rhs == -1)
		divisorSelectorID = b.builder.AllocID()
		block.Body = append(block.Body, Instruction{
			Opcode: OpLogicalOr,
			Words:  []uint32{boolTypeID, divisorSelectorID, rhsEqZeroID, overflowID},
		})
	} else {
		// Unsigned: just check for zero
		divisorSelectorID = rhsEqZeroID
	}

	// const 1
	constOneID := b.builder.AddConstant(b.emitScalarType(scalar), 1)
	compositeOneID := maybeSplat(constOneID)

	// select(should_replace, 1, rhs) — if should_replace is true, use 1; else use rhs
	divisorID := b.builder.AllocID()
	block.Body = append(block.Body, Instruction{
		Opcode: OpSelect,
		Words:  []uint32{rightTypeID, divisorID, divisorSelectorID, compositeOneID, rhsID},
	})

	// Perform the actual operation with safe divisor
	var divOpcode OpCode
	switch {
	case op == ir.BinaryDivide && scalar.Kind == ir.ScalarSint:
		divOpcode = OpSDiv
	case op == ir.BinaryDivide && scalar.Kind == ir.ScalarUint:
		divOpcode = OpUDiv
	case op == ir.BinaryModulo && scalar.Kind == ir.ScalarSint:
		divOpcode = OpSRem // Rust uses OpSRem, not OpSMod
	case op == ir.BinaryModulo && scalar.Kind == ir.ScalarUint:
		divOpcode = OpUMod
	}

	returnID := b.builder.AllocID()
	block.Body = append(block.Body, Instruction{
		Opcode: divOpcode,
		Words:  []uint32{returnTypeID, returnID, lhsID, divisorID},
	})

	// Terminate with OpReturnValue
	fb.Consume(block, Instruction{
		Opcode: OpReturnValue,
		Words:  []uint32{returnID},
	})

	// Serialize to the module's function definitions
	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)

	return nil
}

// emitFunctions emits all functions (both regular and entry point).
func (b *Backend) emitFunctions() error {
	// First, scan all functions and entry points for integer div/mod,
	// and emit wrapper helper functions. This must happen before emitting
	// any regular functions, matching Rust naga's write_wrapped_functions
	// which is called at the start of each write_function.
	for handle := range b.module.Functions {
		fn := &b.module.Functions[handle]
		if err := b.emitWrappedFunctions(fn); err != nil {
			return err
		}
	}
	for epIdx := range b.module.EntryPoints {
		// Skip mesh/task entry points — not supported in SPIR-V (matches Rust naga)
		if b.module.EntryPoints[epIdx].Stage == ir.StageTask || b.module.EntryPoints[epIdx].Stage == ir.StageMesh {
			continue
		}
		fn := &b.module.EntryPoints[epIdx].Function
		if err := b.emitWrappedFunctions(fn); err != nil {
			return err
		}
	}

	// Emit regular functions
	for handle := range b.module.Functions {
		fn := &b.module.Functions[handle]
		if err := b.emitRegularFunction(ir.FunctionHandle(handle), fn); err != nil {
			return err
		}
	}
	// Emit entry point functions (stored inline in EntryPoints, not in Functions[])
	for epIdx := range b.module.EntryPoints {
		// Skip mesh/task entry points — not supported in SPIR-V (matches Rust naga)
		if b.module.EntryPoints[epIdx].Stage == ir.StageTask || b.module.EntryPoints[epIdx].Stage == ir.StageMesh {
			continue
		}
		fn := &b.module.EntryPoints[epIdx].Function
		if err := b.emitEntryPointFunction(epIdx, fn); err != nil {
			return err
		}
	}
	return nil
}

// emitRegularFunction emits a regular (non-entry-point) function.
func (b *Backend) emitRegularFunction(handle ir.FunctionHandle, fn *ir.Function) error {
	return b.emitFunctionImpl(fn, false, handle, -1)
}

// emitEntryPointFunction emits an entry point function (inline in EntryPoint, not in Module.Functions[]).
func (b *Backend) emitEntryPointFunction(epIdx int, fn *ir.Function) error {
	return b.emitFunctionImpl(fn, true, 0, epIdx)
}

// emitFunctionImpl emits a single function.
// For entry points, epIdx is the index into Module.EntryPoints[].
// For regular functions, handle is the index into Module.Functions[].
func (b *Backend) emitFunctionImpl(fn *ir.Function, isEntryPoint bool, handle ir.FunctionHandle, epIdx int) error {
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

	// Build OpFunction instruction (without appending to functions yet)
	funcID := b.builder.AllocID()
	if isEntryPoint {
		b.entryPointFuncIDs[epIdx] = funcID
	} else {
		b.functionIDs[handle] = funcID
	}
	var fb FunctionBuilder
	{
		ib := b.newIB()
		ib.AddWord(returnTypeID)
		ib.AddWord(funcID)
		ib.AddWord(uint32(FunctionControlNone))
		ib.AddWord(funcTypeID)
		fb.Signature = ib.Build(OpFunction)
	}

	// Build function parameters (only for non-entry-point functions)
	if !isEntryPoint {
		for i, arg := range fn.Arguments {
			paramID := b.builder.AllocID()
			ib := b.newIB()
			ib.AddWord(paramTypeIDs[i])
			ib.AddWord(paramID)
			fb.Parameters = append(fb.Parameters, ib.Build(OpFunctionParameter))
			paramIDs[i] = paramID

			// Add debug name if enabled
			if b.options.Debug && arg.Name != "" {
				b.builder.AddName(paramID, arg.Name)
			}
		}
	}

	// Allocate entry block label ID now (matching old AddLabel() position)
	// to preserve identical ID allocation order for snapshot tests.
	entryBlockLabel := b.builder.AllocID()

	// IMPORTANT: SPIR-V requires all OpVariable instructions at the START of the
	// first block, BEFORE any other instructions (including OpLoad).
	// FunctionBuilder.Variables are emitted in the first block by ToInstructions().

	// 1. First, emit local variables (OpVariable) into FunctionBuilder.Variables
	localVarIDs := make([]uint32, len(fn.LocalVars))
	// rayQueryLocalTrackers: maps local var index to tracker IDs.
	// Populated for local vars whose type is RayQuery.
	rayQueryLocalTrackers := make(map[int]rayQueryTrackerIDs)
	for i, localVar := range fn.LocalVars {
		varType, err := b.emitType(localVar.Type)
		if err != nil {
			return err
		}

		// Create pointer to function storage class
		ptrType := b.emitPointerType(StorageClassFunction, varType)

		// Allocate variable (OpVariable for function builder)
		varID := b.builder.AllocID()
		ib := b.newIB()
		ib.AddWord(ptrType)
		ib.AddWord(varID)
		ib.AddWord(uint32(StorageClassFunction))
		fb.Variables = append(fb.Variables, ib.Build(OpVariable))

		localVarIDs[i] = varID

		// Add debug name if enabled
		if b.options.Debug && localVar.Name != "" {
			b.builder.AddName(varID, localVar.Name)
		}

		// For ray_query local variables, create tracker variables
		if _, isRQ := b.module.Types[localVar.Type].Inner.(ir.RayQueryType); isRQ {
			trackers := b.emitRayQueryTrackerVars(&fb)
			rayQueryLocalTrackers[i] = trackers
		}
	}

	// 2. For entry points, handle input variables
	var output *entryPointOutput
	entryPointInputLocals := make([]uint32, len(fn.Arguments))
	var entryInputs []*entryPointInput
	if isEntryPoint {
		if inputVars, ok := b.entryInputVars[epIdx]; ok {
			entryInputs = inputVars
			for i, input := range inputVars {
				if input == nil {
					continue
				}
				if input.isStruct {
					// Struct input: compose value from input vars using
					// OpCompositeConstruct (matching Rust naga). The composed
					// value ID is assigned to paramIDs later during block setup,
					// avoiding Function-space pointer types entirely.
					entryPointInputLocals[i] = 0xFFFFFFFF // sentinel: struct input pending compose
					paramIDs[i] = 0                       // will be set after compose
				} else if input.singleVarID != 0 {
					// Simple input - use directly
					paramIDs[i] = input.singleVarID
				}
			}
		}
		output = b.entryOutputVars[epIdx]
	}

	// Create the entry block. The label was allocated earlier to match ID ordering.
	entryBlock := NewBlock(entryBlockLabel)

	// 3. Create expression emitter for this function
	emitter := &ExpressionEmitter{
		backend:               b,
		function:              fn,
		exprIDs:               make(map[ir.ExpressionHandle]uint32, len(fn.Expressions)),
		paramIDs:              paramIDs,
		isEntryPoint:          isEntryPoint,
		epIdx:                 epIdx,
		output:                output,
		funcBuilder:           &fb,
		callResultIDs:         make(map[ir.ExpressionHandle]uint32, 4),
		deferredCallStores:    make(map[ir.ExpressionHandle]uint32, 4),
		deferredComplexStores: make(map[ir.ExpressionHandle][]deferredComplexStore),
		spilledComposites:     make(map[ir.ExpressionHandle]uint32),
		spilledAccesses:       make(map[ir.ExpressionHandle]bool),
		accessUses:            make(map[ir.ExpressionHandle]int),
		rayQueryTrackers:      make(map[ir.ExpressionHandle]rayQueryTrackerIDs),
	}
	emitter.localVarIDs = localVarIDs

	// Associate ray query tracker variables with expression handles.
	// When an expression is ExprLocalVariable referencing a ray_query local var,
	// record the tracker IDs for that expression handle (matching Rust naga).
	for exprIdx, expr := range fn.Expressions {
		if lv, ok := expr.Kind.(ir.ExprLocalVariable); ok {
			if trackers, hasTracker := rayQueryLocalTrackers[int(lv.Variable)]; hasTracker {
				emitter.rayQueryTrackers[ir.ExpressionHandle(exprIdx)] = trackers
			}
		}
	}

	// Pre-scan: count Access/AccessIndex references to each base expression,
	// and total references to each expression. Used to determine tips of
	// spilled access chains (matching Rust naga's access_uses + ref_count).
	exprRefCount := make(map[ir.ExpressionHandle]int, len(fn.Expressions))
	for _, expr := range fn.Expressions {
		switch k := expr.Kind.(type) {
		case ir.ExprAccess:
			emitter.accessUses[k.Base]++
			exprRefCount[k.Base]++
			exprRefCount[k.Index]++
		case ir.ExprAccessIndex:
			emitter.accessUses[k.Base]++
			exprRefCount[k.Base]++
		case ir.ExprBinary:
			exprRefCount[k.Left]++
			exprRefCount[k.Right]++
		case ir.ExprUnary:
			exprRefCount[k.Expr]++
		case ir.ExprCompose:
			for _, c := range k.Components {
				exprRefCount[c]++
			}
		case ir.ExprSplat:
			exprRefCount[k.Value]++
		case ir.ExprLoad:
			exprRefCount[k.Pointer]++
		case ir.ExprSelect:
			exprRefCount[k.Condition]++
			exprRefCount[k.Accept]++
			exprRefCount[k.Reject]++
		case ir.ExprSwizzle:
			exprRefCount[k.Vector]++
		case ir.ExprAs:
			exprRefCount[k.Expr]++
		case ir.ExprMath:
			exprRefCount[k.Arg]++
			if k.Arg1 != nil {
				exprRefCount[*k.Arg1]++
			}
			if k.Arg2 != nil {
				exprRefCount[*k.Arg2]++
			}
			if k.Arg3 != nil {
				exprRefCount[*k.Arg3]++
			}
		}
	}
	// Also count references from statements (Return, Store, etc.)
	var countStmtRefs func(stmts ir.Block)
	countStmtRefs = func(stmts ir.Block) {
		for _, stmt := range stmts {
			switch s := stmt.Kind.(type) {
			case ir.StmtReturn:
				if s.Value != nil {
					exprRefCount[*s.Value]++
				}
			case ir.StmtStore:
				exprRefCount[s.Pointer]++
				exprRefCount[s.Value]++
			case ir.StmtCall:
				for _, arg := range s.Arguments {
					exprRefCount[arg]++
				}
			case ir.StmtIf:
				exprRefCount[s.Condition]++
				countStmtRefs(s.Accept)
				countStmtRefs(s.Reject)
			case ir.StmtSwitch:
				exprRefCount[s.Selector]++
				for _, c := range s.Cases {
					countStmtRefs(c.Body)
				}
			case ir.StmtLoop:
				countStmtRefs(s.Body)
				countStmtRefs(s.Continuing)
			case ir.StmtBlock:
				countStmtRefs(s.Block)
			}
		}
	}
	countStmtRefs(fn.Body)
	emitter.exprRefCount = exprRefCount

	// Activate block model: route all function-body emissions to the entry block.
	emitter.setCurrentBlock(&entryBlock)

	// 4. Initialize entry point struct inputs (load from Input interface variables)
	// This must happen after OpVariable but before other instructions.
	// Matching Rust naga: compose struct as SSA value via OpCompositeConstruct,
	// then use OpCompositeExtract for member access (no Function-space variable).
	if isEntryPoint && entryInputs != nil {
		for i, sentinel := range entryPointInputLocals {
			if sentinel != 0xFFFFFFFF {
				continue
			}
			input := entryInputs[i]
			arg := fn.Arguments[i]
			argTypeInner := b.module.Types[arg.Type].Inner
			structType := argTypeInner.(ir.StructType)

			// Load each member from its Input interface variable and composite construct
			memberIDs := make([]uint32, len(structType.Members))
			for j, member := range structType.Members {
				memberTypeID, _ := b.emitType(member.Type)
				memberVarID := input.memberVarIDs[j]
				if memberVarID != 0 {
					// Load from Input interface variable
					if f32TypeID, ok := b.f16PolyfillVars[memberVarID]; ok {
						// F16 polyfill: load as f32, then convert to f16
						f32Value := b.builder.AddLoad(f32TypeID, memberVarID)
						convertedID := b.builder.AllocID()
						b.ib.Reset()
						b.ib.AddWord(memberTypeID) // f16 result type
						b.ib.AddWord(convertedID)
						b.ib.AddWord(f32Value)
						b.builder.funcAppend(b.ib.Build(OpFConvert))
						memberIDs[j] = convertedID
					} else if b.sampleMaskVars[memberVarID] {
						// SampleMask: variable is array<u32, 1>, need AccessChain[0]
						elemPtrType := b.emitPointerType(StorageClassInput, memberTypeID)
						u32Type := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
						idx0 := b.builder.AddConstant(u32Type, 0)
						elemPtr := b.builder.AddAccessChain(elemPtrType, memberVarID, idx0)
						memberIDs[j] = b.builder.AddLoad(memberTypeID, elemPtr)
					} else {
						memberIDs[j] = b.builder.AddLoad(memberTypeID, memberVarID)
					}
				} else {
					// Member without binding - use zero/default value
					memberIDs[j] = b.builder.AddConstantNull(memberTypeID)
				}
			}

			// Composite construct the struct as an SSA value (no Function variable)
			structValue := b.builder.AddCompositeConstruct(input.typeID, memberIDs...)

			// Set param directly to the composed value (not a pointer)
			paramIDs[i] = structValue
			emitter.paramIDs[i] = structValue
			if emitter.ssaEntryArgs == nil {
				emitter.ssaEntryArgs = make(map[int]bool)
			}
			emitter.ssaEntryArgs[i] = true
		}
	}

	// 4b. ForcePointSize: store 1.0 into the PointSize output variable
	if isEntryPoint {
		if psVarID, ok := b.forcePointSizeVars[epIdx]; ok {
			f32TypeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
			oneF32 := b.builder.AddConstantFloat32(f32TypeID, 1.0)
			b.builder.AddStore(psVarID, oneF32)
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
	//
	// Additionally, variables that reference other deferred variables (via
	// ExprLocalVariable) must also be deferred, since the referenced variable
	// won't be initialized until the body runs.

	// Pass 1: classify each var — find which call result handle it depends on
	// (directly or transitively through other deferred vars).
	type varDeferInfo struct {
		isDeferred     bool
		callResultH    ir.ExpressionHandle // the call result to wait for
		hasCallResult  bool                // whether callResultH is valid
		isDirectCall   bool                // direct ExprCallResult/ExprAtomicResult
		depsOnVarIndex int                 // index of deferred var it depends on (-1 if none)
	}
	varInfo := make([]varDeferInfo, len(fn.LocalVars))

	for i, localVar := range fn.LocalVars {
		if localVar.Init == nil {
			continue
		}
		initExpr := fn.Expressions[*localVar.Init]
		if _, isCallResult := initExpr.Kind.(ir.ExprCallResult); isCallResult {
			varInfo[i] = varDeferInfo{isDeferred: true, isDirectCall: true}
			continue
		}
		if _, isAtomicResult := initExpr.Kind.(ir.ExprAtomicResult); isAtomicResult {
			varInfo[i] = varDeferInfo{isDeferred: true, isDirectCall: true}
			continue
		}
		if _, isSubgroupBallot := initExpr.Kind.(ir.ExprSubgroupBallotResult); isSubgroupBallot {
			varInfo[i] = varDeferInfo{isDeferred: true, isDirectCall: true}
			continue
		}
		if _, isSubgroupOp := initExpr.Kind.(ir.ExprSubgroupOperationResult); isSubgroupOp {
			varInfo[i] = varDeferInfo{isDeferred: true, isDirectCall: true}
			continue
		}
		if h, ok := findLastDeferredResultInTree(fn.Expressions, *localVar.Init); ok {
			varInfo[i] = varDeferInfo{isDeferred: true, callResultH: h, hasCallResult: true}
		}
	}

	// Pass 2: propagate — vars referencing deferred vars become deferred too.
	for changed := true; changed; {
		changed = false
		for i, localVar := range fn.LocalVars {
			if localVar.Init == nil || varInfo[i].isDeferred {
				continue
			}
			deferredSet := make(map[int]bool, len(varInfo))
			for j := range varInfo {
				if varInfo[j].isDeferred {
					deferredSet[j] = true
				}
			}
			if depIdx := findDeferredLocalVarRef(fn.Expressions, *localVar.Init, deferredSet); depIdx >= 0 {
				varInfo[i] = varDeferInfo{isDeferred: true, depsOnVarIndex: depIdx}
				changed = true
			}
		}
	}

	// Resolve transitive dependencies: find the ultimate call result handle
	// for vars that depend on other deferred vars.
	for i := range varInfo {
		if !varInfo[i].isDeferred || varInfo[i].isDirectCall || varInfo[i].hasCallResult {
			continue
		}
		// Walk the dependency chain to find the call result.
		visited := make(map[int]bool)
		j := varInfo[i].depsOnVarIndex
		for j >= 0 && !visited[j] {
			visited[j] = true
			if varInfo[j].hasCallResult {
				varInfo[i].callResultH = varInfo[j].callResultH
				varInfo[i].hasCallResult = true
				break
			}
			if varInfo[j].isDirectCall && fn.LocalVars[j].Init != nil {
				varInfo[i].callResultH = *fn.LocalVars[j].Init
				varInfo[i].hasCallResult = true
				break
			}
			j = varInfo[j].depsOnVarIndex
		}
	}

	// Pass 3: emit stores.
	for i, localVar := range fn.LocalVars {
		if localVar.Init == nil {
			continue
		}
		if !varInfo[i].isDeferred {
			initID, err := emitter.emitExpression(*localVar.Init)
			if err != nil {
				return fmt.Errorf("local var %q init: %w", localVar.Name, err)
			}
			b.builder.AddStore(localVarIDs[i], initID)
			continue
		}
		if varInfo[i].isDirectCall {
			emitter.deferredCallStores[*localVar.Init] = localVarIDs[i]
			continue
		}
		if varInfo[i].hasCallResult {
			emitter.deferredComplexStores[varInfo[i].callResultH] = append(
				emitter.deferredComplexStores[varInfo[i].callResultH],
				deferredComplexStore{varPtrID: localVarIDs[i], initExpr: *localVar.Init},
			)
			continue
		}
		// Fallback: deferred but no call result found — shouldn't happen,
		// but emit in prologue to avoid silently dropping the init.
		initID, err := emitter.emitExpression(*localVar.Init)
		if err != nil {
			return fmt.Errorf("local var %q init: %w", localVar.Name, err)
		}
		b.builder.AddStore(localVarIDs[i], initID)
	}

	// 5b. Workgroup variable zero-initialization polyfill.
	// For compute entry points, generate code that zero-initializes all workgroup
	// variables when local_invocation_id == (0,0,0), followed by a barrier.
	// This matches Rust naga's ZeroInitializeWorkgroupMemoryMode::Polyfill.
	if isEntryPoint && epIdx >= 0 {
		entryPoint := &b.module.EntryPoints[epIdx]
		if entryPoint.Stage == ir.StageCompute {
			b.emitWorkgroupInitPolyfill(epIdx, fn, emitter, &fb)
		}
	}

	// Emit function body statements
	for _, stmt := range fn.Body {
		if err := emitter.emitStatement(stmt); err != nil {
			return err
		}
	}

	// Finalize the last block. If the body didn't end with a terminator,
	// add OpReturn. If it did, the block was already consumed by emitStatement.
	if emitter.currentBlock != nil {
		if fn.Result != nil {
			// Non-void function without explicit return — should not happen in valid WGSL
			return fmt.Errorf("non-void function missing return statement")
		}
		emitter.consumeBlock(Instruction{Opcode: OpReturn})
	}

	// Deactivate block model
	b.builder.funcSink = nil

	// Serialize all blocks into the flat instruction list
	b.builder.functions = append(b.builder.functions, fb.ToInstructions()...)

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
	epIdx        int               // Entry point index (valid only when isEntryPoint is true)
	output       *entryPointOutput // Output variable(s) for entry point result
	ssaEntryArgs map[int]bool      // Entry point args that are SSA values (composed structs, not pointers)

	// Block model: currentBlock is the block being built, funcBuilder collects
	// terminated blocks for the current function.
	currentBlock *Block
	funcBuilder  *FunctionBuilder

	// Loop context using value-semantic LoopContext (replaces loopStack/breakStack).
	// Passed by value to ensure nested loops get isolated copies.
	loopCtx LoopContext

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

	// Spilled composites: maps expression handle to the SPIR-V variable ID of
	// the Function-space temporary variable that holds the spilled value.
	// Used when dynamically indexing by-value arrays or matrices -- SPIR-V
	// has no instructions for that, so we spill to a variable and use
	// OpAccessChain + OpLoad instead.
	spilledComposites map[ir.ExpressionHandle]uint32

	// Set of expression handles that are spilled or refer to components of
	// spilled composites. Used to propagate spill status through chains of
	// Access/AccessIndex expressions.
	spilledAccesses map[ir.ExpressionHandle]bool

	// Count of Access/AccessIndex expressions that reference each base
	// expression. Used to determine whether a spilled access is the "tip"
	// of a chain (i.e., needs to actually load) or an intermediate node.
	accessUses map[ir.ExpressionHandle]int

	// Total reference count for each expression (from both expressions and statements).
	// Compared with accessUses to determine intermediate vs tip accesses.
	exprRefCount map[ir.ExpressionHandle]int

	// Ray query tracker variables: maps the expression handle of the ray_query
	// local variable pointer to its tracker IDs (initialized_tracker u32 + t_max_tracker f32).
	rayQueryTrackers map[ir.ExpressionHandle]rayQueryTrackerIDs
}

// deferredComplexStore represents a local variable whose init expression
// contains an ExprCallResult somewhere in its expression tree.
type deferredComplexStore struct {
	varPtrID uint32
	initExpr ir.ExpressionHandle
}

// isNonUniformBindingArrayAccess returns true if accessing a BindingArray global
// variable with a non-uniform index. Matches Rust naga's is_nonuniform_binding_array_access.
func (e *ExpressionEmitter) isNonUniformBindingArrayAccess(base, index ir.ExpressionHandle) bool {
	// Check that the base is a GlobalVariable with a BindingArray type
	baseExpr := e.function.Expressions[base]
	gv, ok := baseExpr.Kind.(ir.ExprGlobalVariable)
	if !ok {
		return false
	}
	globalVar := e.backend.module.GlobalVariables[gv.Variable]
	inner := e.backend.module.Types[globalVar.Type].Inner
	if _, isBA := inner.(ir.BindingArrayType); !isBA {
		return false
	}

	// Check if the index expression is non-uniform.
	// Rust uses full uniformity analysis (fun_info[index].uniformity.non_uniform_result).
	// We use a simplified trace: an expression is non-uniform if it ultimately derives
	// from a fragment/compute shader input (FunctionArgument).
	return e.isNonUniformExpression(index)
}

// isNonUniformExpression returns true if the expression is non-uniform (varies across
// invocations in the same subgroup). This is a simplified uniformity analysis:
// - FunctionArgument in a fragment shader entry point -> non-uniform
// - Constants, Literals -> uniform
// - GlobalVariable (uniform/storage buffer) loads -> uniform
// - Expressions derived from non-uniform sources -> non-uniform
func (e *ExpressionEmitter) isNonUniformExpression(handle ir.ExpressionHandle) bool {
	if int(handle) >= len(e.function.Expressions) {
		return false
	}
	expr := e.function.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprFunctionArgument:
		// Fragment shader inputs are non-uniform (vary per fragment).
		// In Rust naga, these get non_uniform_result set during uniformity analysis.
		if !e.isEntryPoint {
			return false
		}
		ep := &e.backend.module.EntryPoints[e.epIdx]
		return ep.Stage == ir.StageFragment || ep.Stage == ir.StageCompute
	case ir.ExprAccess:
		// Propagate non-uniformity from the index
		return e.isNonUniformExpression(k.Index) || e.isNonUniformExpression(k.Base)
	case ir.ExprAccessIndex:
		// AccessIndex with constant index: non-uniformity comes from base only
		return e.isNonUniformExpression(k.Base)
	case ir.ExprLoad:
		return e.isNonUniformExpression(k.Pointer)
	case ir.ExprBinary:
		return e.isNonUniformExpression(k.Left) || e.isNonUniformExpression(k.Right)
	case ir.ExprUnary:
		return e.isNonUniformExpression(k.Expr)
	case ir.ExprAs:
		return e.isNonUniformExpression(k.Expr)
	case ir.ExprGlobalVariable:
		// Global variables themselves are uniform (uniform/storage buffer)
		return false
	case ir.ExprLocalVariable:
		// Local variables are conservatively uniform (they could be assigned non-uniform
		// values, but tracking that requires full data-flow analysis). For the binding-arrays
		// test case, the non-uniform index is used directly from a let-binding which goes
		// through Load, not LocalVariable stores.
		return false
	case ir.Literal:
		return false
	case ir.ExprConstant:
		return false
	case ir.ExprZeroValue:
		return false
	case ir.ExprCompose:
		for _, arg := range k.Components {
			if e.isNonUniformExpression(arg) {
				return true
			}
		}
		return false
	case ir.ExprSplat:
		return e.isNonUniformExpression(k.Value)
	default:
		// Conservative: treat unknown expressions as potentially non-uniform
		// This is safe (may produce extra decorations but never misses one)
		return false
	}
}

// setCurrentBlock switches the instruction emission sink to a new block.
// All subsequent Add* calls on the ModuleBuilder will append to this block's body.
func (e *ExpressionEmitter) setCurrentBlock(block *Block) {
	e.currentBlock = block
	e.backend.builder.funcSink = &block.Body
}

// consumeBlock finalizes the current block with the given terminator and
// appends it to the function builder. Returns the consumed block (for callers
// that need to inspect it). After this call, currentBlock is nil and funcSink
// points nowhere — the caller MUST call setCurrentBlock before emitting more.
func (e *ExpressionEmitter) consumeBlock(terminator Instruction) {
	e.funcBuilder.Consume(*e.currentBlock, terminator)
	e.currentBlock = nil
	e.backend.builder.funcSink = nil
}

// makeBranchInstruction creates an OpBranch instruction to the given target.
func makeBranchInstruction(target uint32) Instruction {
	return Instruction{
		Opcode: OpBranch,
		Words:  []uint32{target},
	}
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

// resolveTypeForStorageClass resolves a type resolution, using layout-free types
// for Workgroup storage class per VUID-StandaloneSpirv-None-10684.
func (b *Backend) resolveTypeForStorageClass(res ir.TypeResolution, storageClass StorageClass) uint32 {
	if storageClass == StorageClassWorkgroup && res.Handle != nil {
		return b.emitTypeWithoutLayout(*res.Handle)
	}
	return b.resolveTypeResolution(res)
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
		return b.emitMatrixType(columnTypeID, uint32(t.Columns))

	case ir.PointerType:
		// Emit the base type first
		baseID, err := b.emitType(t.Base)
		if err != nil {
			panic(fmt.Sprintf("failed to emit pointer base type: %v", err))
		}
		storageClass := addressSpaceToStorageClass(t.Space)
		return b.emitPointerType(storageClass, baseID)

	case ir.ValuePointerType:
		// ValuePointer is a transient pointer to scalar/vector (from matrix/vector access through pointer).
		// Emit as OpTypePointer to the pointee type.
		var pointeeID uint32
		if t.Size != nil {
			scalarID := b.emitScalarType(t.Scalar)
			pointeeID = b.emitVectorType(scalarID, uint32(*t.Size))
		} else {
			pointeeID = b.emitScalarType(t.Scalar)
		}
		storageClass := addressSpaceToStorageClass(t.Space)
		return b.emitPointerType(storageClass, pointeeID)

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
	case ir.ExprZeroValue:
		// OpConstantNull — zero value for any type (matches Rust naga)
		typeID, err := e.backend.emitType(kind.Type)
		if err != nil {
			return 0, fmt.Errorf("zero value type: %w", err)
		}
		id = e.backend.builder.AddConstantNull(typeID)
	case ir.ExprCompose:
		id, err = e.emitCompose(kind)
	case ir.ExprSplat:
		id, err = e.emitSplat(kind)
	case ir.ExprAccess:
		id, err = e.emitAccess(handle, kind)
	case ir.ExprAccessIndex:
		id, err = e.emitAccessIndex(handle, kind)
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
	case ir.ExprSubgroupBallotResult:
		return e.emitSubgroupResultRef(handle)
	case ir.ExprSubgroupOperationResult:
		return e.emitSubgroupResultRef(handle)
	case ir.ExprRayQueryProceedResult:
		return e.emitRayQueryResultRef(handle)
	case ir.ExprRayQueryGetIntersection:
		id, err = e.emitRayQueryGetIntersection(kind)
	case ir.ExprWorkGroupUniformLoadResult:
		// Result was pre-cached by emitWorkGroupUniformLoad
		if cached, ok := e.exprIDs[handle]; ok && cached != 0 {
			return cached, nil
		}
		return 0, fmt.Errorf("WorkGroupUniformLoadResult not yet cached for handle %d", handle)
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

// emitConstExpression emits an expression as a SPIR-V constant (in the declarations section).
// This is required for SPIR-V operands that must be constant, such as ConstOffset for image sampling.
// WGSL guarantees texture offsets are const-expressions, so this handles Literal, Compose of constants,
// ZeroValue, and Constant expression kinds.
func (e *ExpressionEmitter) emitConstExpression(handle ir.ExpressionHandle) (uint32, error) {
	// NOTE: We intentionally do NOT check the expression cache here.
	// The cache may contain a runtime OpCompositeConstruct ID for the same expression
	// if it was previously emitted via emitExpression. For SPIR-V ConstOffset operands,
	// we must produce OpConstantComposite (in the declarations section), so we always
	// emit fresh constants. Literals and scalar constants are already deduplicated
	// by the builder's AddConstant methods.

	expr := &e.function.Expressions[handle]
	var id uint32
	var err error

	switch kind := expr.Kind.(type) {
	case ir.Literal:
		// Literals already emit as constants
		id, err = e.emitLiteral(kind.Value)
	case ir.ExprConstant:
		return e.emitConstantRef(kind)
	case ir.ExprZeroValue:
		typeID, zErr := e.backend.emitType(kind.Type)
		if zErr != nil {
			return 0, fmt.Errorf("zero value type: %w", zErr)
		}
		id = e.backend.builder.AddConstantNull(typeID)
	case ir.ExprCompose:
		typeID, tErr := e.backend.emitType(kind.Type)
		if tErr != nil {
			return 0, tErr
		}
		componentIDs := make([]uint32, len(kind.Components))
		for i, component := range kind.Components {
			componentIDs[i], err = e.emitConstExpression(component)
			if err != nil {
				return 0, err
			}
		}
		id = e.backend.builder.AddConstantComposite(typeID, componentIDs...)
	case ir.ExprSplat:
		// Splat as constant composite
		valueType, rErr := ir.ResolveExpressionType(e.backend.module, e.function, kind.Value)
		if rErr != nil {
			return 0, fmt.Errorf("splat value type: %w", rErr)
		}
		var scalar ir.ScalarType
		inner := ir.TypeResInner(e.backend.module, valueType)
		if s, ok := inner.(ir.ScalarType); ok {
			scalar = s
		} else {
			return 0, fmt.Errorf("splat value must be scalar, got %T", inner)
		}
		scalarID := e.backend.emitScalarType(scalar)
		typeID := e.backend.emitVectorType(scalarID, uint32(kind.Size))
		valueID, vErr := e.emitConstExpression(kind.Value)
		if vErr != nil {
			return 0, vErr
		}
		n := int(kind.Size)
		componentIDs := make([]uint32, n)
		for i := 0; i < n; i++ {
			componentIDs[i] = valueID
		}
		id = e.backend.builder.AddConstantComposite(typeID, componentIDs...)
	default:
		// Fallback to runtime emission for unsupported const expression kinds
		return e.emitExpression(handle)
	}

	if err != nil {
		return 0, err
	}
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

	case ir.LiteralAbstractInt:
		// Abstract integers default to i32 in SPIR-V
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
		return e.backend.builder.AddConstant(typeID, uint32(int32(v))), nil

	case ir.LiteralAbstractFloat:
		// Abstract floats default to f32 in SPIR-V
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		return e.backend.builder.AddConstantFloat32(typeID, float32(v)), nil

	case ir.LiteralF16:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 2})
		// F16 is stored as a 32-bit word with the 16-bit float in the low bits.
		// Convert float32 value to float16 bit representation (IEEE 754 half-precision).
		return e.backend.builder.AddConstant(typeID, float32ToF16Bits(float32(v))), nil

	case ir.LiteralI64:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 8})
		bits := uint64(v)
		return e.backend.builder.AddConstant(typeID, uint32(bits&0xFFFFFFFF), uint32(bits>>32)), nil

	case ir.LiteralU64:
		typeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 8})
		bits := uint64(v)
		return e.backend.builder.AddConstant(typeID, uint32(bits&0xFFFFFFFF), uint32(bits>>32)), nil

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
// For types containing runtime-sized arrays, returns the pointer instead
// (SPIR-V forbids OpLoad on runtime-sized arrays).
func (e *ExpressionEmitter) emitGlobalVarValue(kind ir.ExprGlobalVariable) (uint32, error) {
	gv := e.backend.module.GlobalVariables[kind.Variable]

	// SPIR-V forbids OpLoad on types containing runtime-sized arrays.
	// Return the pointer (with wrapper unwrapping) so downstream Access/AccessIndex
	// can use OpAccessChain on it.
	if e.backend.typeContainsRuntimeArray(gv.Type) {
		ptrID, err := e.emitGlobalVarRef(kind)
		if err != nil {
			return 0, err
		}
		if e.backend.wrappedStorageVars[kind.Variable] {
			innerTypeID, err := e.backend.emitType(gv.Type)
			if err != nil {
				return 0, err
			}
			sc := addressSpaceToStorageClass(gv.Space)
			ptrType := e.backend.emitPointerType(sc, innerTypeID)
			u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			const0 := e.backend.builder.AddConstant(u32Type, 0)
			ptrID = e.backend.builder.AddAccessChain(ptrType, ptrID, const0)
		}
		return ptrID, nil
	}

	// Get pointer to the actual data (handles wrapped uniform/storage variables)
	ptrID, err := e.emitGlobalVarRef(kind)
	if err != nil {
		return 0, err
	}
	if e.backend.wrappedStorageVars[kind.Variable] {
		// Wrapped variable: emit AccessChain to member 0 to get past wrapper struct
		innerTypeID, err := e.backend.emitType(gv.Type)
		if err != nil {
			return 0, err
		}
		sc := addressSpaceToStorageClass(gv.Space)
		ptrType := e.backend.emitPointerType(sc, innerTypeID)
		u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		const0 := e.backend.builder.AddConstant(u32Type, 0)
		ptrID = e.backend.builder.AddAccessChain(ptrType, ptrID, const0)
	}

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

	// SSA composed struct args: already a value, no OpLoad needed
	if e.ssaEntryArgs != nil && e.ssaEntryArgs[int(kind.Index)] {
		return ptrID, nil
	}

	// Entry point: paramIDs holds Input variable pointers — need OpLoad
	if int(kind.Index) >= len(e.function.Arguments) {
		return 0, fmt.Errorf("function argument index out of range: %d", kind.Index)
	}
	arg := e.function.Arguments[kind.Index]
	typeID, err := e.backend.emitType(arg.Type)
	if err != nil {
		return 0, err
	}

	// SampleMask: variable is array<u32, 1>, need AccessChain[0] to get u32 element
	if e.backend.sampleMaskVars[ptrID] {
		elemPtrType := e.backend.emitPointerType(StorageClassInput, typeID)
		u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		idx0 := e.backend.builder.AddConstant(u32Type, 0)
		elemPtr := e.backend.builder.AddAccessChain(elemPtrType, ptrID, idx0)
		return e.backend.builder.AddLoad(typeID, elemPtr), nil
	}

	// Check if the variable needs f16 polyfill conversion
	if f32TypeID, ok := e.backend.f16PolyfillVars[ptrID]; ok {
		// Load as f32, then convert to f16
		f32Value := e.backend.builder.AddLoad(f32TypeID, ptrID)
		convertedID := e.backend.builder.AllocID()
		e.backend.ib.Reset()
		e.backend.ib.AddWord(typeID) // f16 result type
		e.backend.ib.AddWord(convertedID)
		e.backend.ib.AddWord(f32Value)
		e.backend.builder.funcAppend(e.backend.ib.Build(OpFConvert))
		return convertedID, nil
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

// emitRayQueryResultRef returns the SPIR-V ID for a ray query result expression.
// The ID is set when the corresponding RayQuery statement is emitted.
func (e *ExpressionEmitter) emitRayQueryResultRef(handle ir.ExpressionHandle) (uint32, error) {
	if existingID, ok := e.exprIDs[handle]; ok {
		return existingID, nil
	}
	return 0, fmt.Errorf("ray query result expression not found - ray query statement should have set it")
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

// emitSplat emits a vector splat (scalar broadcast to all components).
// In SPIR-V, this is OpCompositeConstruct with the same scalar ID repeated.
func (e *ExpressionEmitter) emitSplat(splat ir.ExprSplat) (uint32, error) {
	// Resolve the scalar type from the splat value
	valueType, err := ir.ResolveExpressionType(e.backend.module, e.function, splat.Value)
	if err != nil {
		return 0, fmt.Errorf("splat value type: %w", err)
	}
	var scalar ir.ScalarType
	inner := ir.TypeResInner(e.backend.module, valueType)
	if s, ok := inner.(ir.ScalarType); ok {
		scalar = s
	} else {
		return 0, fmt.Errorf("splat value must be scalar, got %T", inner)
	}

	// Build the vector type ID directly using SPIR-V type emission
	scalarID := e.backend.emitScalarType(scalar)
	typeID := e.backend.emitVectorType(scalarID, uint32(splat.Size))

	valueID, err := e.emitExpression(splat.Value)
	if err != nil {
		return 0, err
	}

	n := int(splat.Size)
	componentIDs := make([]uint32, n)
	for i := 0; i < n; i++ {
		componentIDs[i] = valueID
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
	case ir.ExprLoad:
		// Load dereferences a pointer — propagate storage class from the pointer source
		return e.getExpressionStorageClass(k.Pointer)
	}

	return StorageClassFunction
}

// maybeCopyLogicalForStore inserts OpCopyLogical when a store crosses the Workgroup
// boundary (VUID-StandaloneSpirv-None-10684). Workgroup variables use layout-free types
// (no ArrayStride/Offset), while Storage/Uniform use decorated types. When storing a
// Workgroup-loaded value into a non-Workgroup pointer (or vice versa), the SPIR-V type
// IDs differ even though the types are logically equivalent. OpCopyLogical (SPIR-V 1.4+)
// converts between them. Returns the (possibly converted) value ID.
func (e *ExpressionEmitter) maybeCopyLogicalForStore(pointerExpr, valueExpr ir.ExpressionHandle, valueID uint32) uint32 {
	valueSC := e.getExpressionStorageClass(valueExpr)
	pointerSC := e.getExpressionStorageClass(pointerExpr)

	valueIsWG := valueSC == StorageClassWorkgroup
	pointerIsWG := pointerSC == StorageClassWorkgroup

	// Only need conversion when crossing the Workgroup boundary.
	if valueIsWG == pointerIsWG {
		return valueID
	}

	// Resolve the value expression's IR type to check if it's a composite needing layout.
	valueTypeRes, err := ir.ResolveExpressionType(e.backend.module, e.function, valueExpr)
	if err != nil {
		return valueID
	}

	// Unwrap pointer types to get the value type handle.
	valueTypeHandle := e.unwrapToValueTypeHandle(valueTypeRes)
	if valueTypeHandle == nil {
		return valueID
	}

	// Only composite types (arrays, structs) have layout decorations.
	if !e.backend.typeNeedsLayoutDecoration(*valueTypeHandle) {
		return valueID
	}

	// Determine the target SPIR-V type ID (what the store pointer expects).
	var targetTypeID uint32
	if valueIsWG {
		// Value is layout-free (from Workgroup), target needs decorated type.
		var emitErr error
		targetTypeID, emitErr = e.backend.emitType(*valueTypeHandle)
		if emitErr != nil {
			return valueID
		}
	} else {
		// Value is decorated, target needs layout-free type (for Workgroup pointer).
		targetTypeID = e.backend.emitTypeWithoutLayout(*valueTypeHandle)
	}

	// Bump SPIR-V version to 1.4 (minimum for OpCopyLogical).
	e.backend.requireSpirvVersion14()

	return e.backend.builder.AddCopyLogical(targetTypeID, valueID)
}

// unwrapToValueTypeHandle resolves a TypeResolution to the underlying value
// type handle, unwrapping pointer types. Returns nil if no handle can be determined.
func (e *ExpressionEmitter) unwrapToValueTypeHandle(res ir.TypeResolution) *ir.TypeHandle {
	if res.Handle != nil {
		inner := e.backend.module.Types[*res.Handle].Inner
		if pt, ok := inner.(ir.PointerType); ok {
			return &pt.Base
		}
		return res.Handle
	}
	if res.Value != nil {
		if pt, ok := res.Value.(ir.PointerType); ok {
			return &pt.Base
		}
	}
	return nil
}

// typeNeedsLayoutDecoration returns true if the type has layout decorations
// (ArrayStride, Offset, MatrixStride) that differ between Workgroup and non-Workgroup.
func (b *Backend) typeNeedsLayoutDecoration(handle ir.TypeHandle) bool {
	if int(handle) >= len(b.module.Types) {
		return false
	}
	switch b.module.Types[handle].Inner.(type) {
	case ir.ArrayType:
		return true
	case ir.StructType:
		return true
	default:
		return false
	}
}

// requireSpirvVersion14 ensures the output SPIR-V version is at least 1.4.
// Called when OpCopyLogical or other 1.4+ features are needed.
func (b *Backend) requireSpirvVersion14() {
	b.builder.RequireVersion(Version1_4)
	// Also update options so emitEntryPoints sees the bumped version
	// for SPIR-V 1.4+ interface variable requirements.
	if b.options.Version.Major < 1 || (b.options.Version.Major == 1 && b.options.Version.Minor < 4) {
		b.options.Version = Version1_4
	}
}

// resolveAccessElementType determines the element type when indexing into a base type.
// It handles both handle-based and inline types, and unwraps pointer types.
// For dynamic access (Access), pass index=-1. For static access (AccessIndex), pass the index.
func (e *ExpressionEmitter) resolveAccessElementType(baseType ir.TypeResolution, index int) (ir.TypeResolution, error) {
	// Get the inner type, resolving through handles and unwrapping pointers
	var inner ir.TypeInner
	if baseType.Handle != nil {
		inner = e.backend.module.Types[*baseType.Handle].Inner
	} else if baseType.Value != nil {
		inner = baseType.Value
	} else {
		return ir.TypeResolution{}, fmt.Errorf("nil base type for access")
	}

	// Unwrap pointer types to get the pointee type
	if pt, ok := inner.(ir.PointerType); ok {
		if int(pt.Base) < len(e.backend.module.Types) {
			inner = e.backend.module.Types[pt.Base].Inner
		}
	}

	// Handle ValuePointerType (transient pointer to vector/scalar from matrix/vector access)
	if vp, ok := inner.(ir.ValuePointerType); ok {
		if vp.Size != nil {
			// Pointer to vector — indexing gives scalar
			return ir.TypeResolution{Value: vp.Scalar}, nil
		}
		return ir.TypeResolution{}, fmt.Errorf("cannot index into scalar value pointer")
	}

	switch t := inner.(type) {
	case ir.ArrayType:
		h := t.Base
		return ir.TypeResolution{Handle: &h}, nil
	case ir.VectorType:
		return ir.TypeResolution{Value: t.Scalar}, nil
	case ir.MatrixType:
		return ir.TypeResolution{Value: ir.VectorType{Size: t.Rows, Scalar: t.Scalar}}, nil
	case ir.StructType:
		if index >= 0 {
			if index >= len(t.Members) {
				return ir.TypeResolution{}, fmt.Errorf("struct member index %d out of range (max %d)", index, len(t.Members))
			}
			h := t.Members[index].Type
			return ir.TypeResolution{Handle: &h}, nil
		}
		return ir.TypeResolution{}, fmt.Errorf("dynamic access on struct type is not supported")
	case ir.BindingArrayType:
		h := t.Base
		return ir.TypeResolution{Handle: &h}, nil
	default:
		return ir.TypeResolution{}, fmt.Errorf("cannot index into type %T", inner)
	}
}

// emitAccess emits a dynamic access operation.
// Returns the VALUE at the indexed location (not a pointer).
func (e *ExpressionEmitter) emitAccess(exprHandle ir.ExpressionHandle, access ir.ExprAccess) (uint32, error) {
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

	// Determine the element type, unwrapping pointer types
	elementType, err := e.resolveAccessElementType(baseType, -1)
	if err != nil {
		return 0, fmt.Errorf("access element type: %w", err)
	}

	// Get the element type ID
	elementTypeID := e.backend.resolveTypeResolution(elementType)

	if isPointerBase {
		// Check if this is a non-uniform binding array access before emitting.
		// Matches Rust naga's is_nonuniform_binding_array_access + decorate pattern.
		isNonUniformBA := e.isNonUniformBindingArrayAccess(access.Base, access.Index)

		// Base is a pointer - use emitPointerExpression to get SPIR-V pointer, then OpAccessChain
		baseID, err := e.emitPointerExpression(access.Base)
		if err != nil {
			return 0, err
		}

		storageClass := e.getExpressionStorageClass(access.Base)
		// Use layout-free element type for Workgroup (VUID-StandaloneSpirv-None-10684).
		accessElementTypeID := e.backend.resolveTypeForStorageClass(elementType, storageClass)

		// Create pointer type for OpAccessChain result
		ptrType := e.backend.emitPointerType(storageClass, accessElementTypeID)

		// OpAccessChain returns a pointer, then we auto-load
		ptrID := e.backend.builder.AddAccessChain(ptrType, baseID, indexID)

		// For non-uniform binding array access, decorate the pointer (AccessChain result)
		// with NonUniform. See VUID-RuntimeSpirv-NonUniform-06274.
		if isNonUniformBA {
			e.backend.decorateNonUniformBindingArrayAccess(ptrID)
		}

		// SPIR-V forbids OpLoad on runtime-sized arrays and types containing them.
		// Return the pointer directly; downstream Access/AccessIndex will use
		// OpAccessChain to reach individual elements.
		if elementType.Handle != nil && e.backend.typeContainsRuntimeArray(*elementType.Handle) {
			return ptrID, nil
		}

		loadID := e.backend.builder.AddLoad(accessElementTypeID, ptrID)

		// For non-uniform binding array access, also decorate the load result.
		// Subsequent image operations require the image/sampler to be decorated.
		// See VUID-RuntimeSpirv-NonUniform-06274.
		if isNonUniformBA {
			e.backend.decorateNonUniformBindingArrayAccess(loadID)
		}

		return loadID, nil
	}

	// Check if the base was already spilled by a previous access
	if e.spilledAccesses[access.Base] {
		// Propagate spill status to this access
		e.spilledAccesses[exprHandle] = true
		return e.maybeAccessSpilledComposite(exprHandle, elementTypeID)
	}

	// Get the base's type inner to decide strategy
	baseTypeInner := e.resolveTypeInner(baseType)

	switch baseTypeInner.(type) {
	case ir.VectorType:
		// Vectors: use OpVectorExtractDynamic
		baseID, err := e.emitExpression(access.Base)
		if err != nil {
			return 0, err
		}
		return e.backend.builder.AddVectorExtractDynamic(elementTypeID, baseID, indexID), nil

	case ir.ArrayType, ir.MatrixType:
		// Arrays/matrices with dynamic index: SPIR-V has no instructions for
		// dynamic by-value indexing. Spill to a Function-space temporary variable,
		// then use OpAccessChain + OpLoad.
		e.spillToInternalVariable(access.Base)
		e.spilledAccesses[exprHandle] = true
		return e.maybeAccessSpilledComposite(exprHandle, elementTypeID)

	default:
		// Fallback: OpVectorExtractDynamic (e.g. for unknown types)
		baseID, err := e.emitExpression(access.Base)
		if err != nil {
			return 0, err
		}
		return e.backend.builder.AddVectorExtractDynamic(elementTypeID, baseID, indexID), nil
	}
}

// isVariableReference returns true if the expression is a variable reference
// (GlobalVariable or LocalVariable). These are pointer expressions in the WGSL
// Load Rule model and should not be emitted directly in the Emit loop.
func (e *ExpressionEmitter) isVariableReference(handle ir.ExpressionHandle) bool {
	if int(handle) >= len(e.function.Expressions) {
		return false
	}
	switch e.function.Expressions[handle].Kind.(type) {
	case ir.ExprGlobalVariable, ir.ExprLocalVariable:
		return true
	default:
		return false
	}
}

// isPointerExpression recursively checks if an expression returns a SPIR-V pointer.
// This includes variable references and access chains on pointer bases.
func (e *ExpressionEmitter) isPointerExpression(handle ir.ExpressionHandle) bool {
	expr := e.function.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprLocalVariable, ir.ExprGlobalVariable:
		return true
	case ir.ExprFunctionArgument:
		if !e.isEntryPoint {
			// For regular functions, check if the argument type is a pointer.
			// WGSL allows ptr<function, T> parameters — OpFunctionParameter
			// with pointer type produces a pointer, not a value.
			if int(k.Index) < len(e.function.Arguments) {
				argType := e.backend.module.Types[e.function.Arguments[k.Index].Type].Inner
				switch argType.(type) {
				case ir.PointerType, ir.ValuePointerType:
					return true
				}
			}
			return false
		}
		// For entry points, paramIDs holds Input variable pointers, EXCEPT for
		// struct arguments that are composed via CompositeConstruct (SSA values).
		if e.ssaEntryArgs != nil && e.ssaEntryArgs[int(k.Index)] {
			return false
		}
		return true
	case ir.ExprAccessIndex:
		return e.isPointerExpression(k.Base)
	case ir.ExprAccess:
		return e.isPointerExpression(k.Base)
	case ir.ExprLoad:
		// ExprLoad dereferences a pointer. If the pointer source is a pointer expression
		// (e.g., a function argument with pointer type), then the load result can be
		// treated as a pointer for access chain purposes. In SPIR-V, we skip the load
		// and use OpAccessChain directly on the underlying pointer.
		return e.isPointerExpression(k.Pointer)
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
	case ir.ExprLoad:
		// ExprLoad dereferences a pointer. For pointer chain contexts (store targets,
		// access chains), skip the load and pass through to the underlying pointer.
		// This allows (*p).x to emit OpAccessChain on p directly.
		return e.emitPointerExpression(k.Pointer)
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

	// Determine the element type.
	// Determine the element type, unwrapping pointer types
	elementType, err := e.resolveAccessElementType(baseType, int(access.Index))
	if err != nil {
		return 0, fmt.Errorf("access index as pointer element type: %w", err)
	}

	storageClass := e.getExpressionStorageClass(access.Base)
	// Use layout-free element type for Workgroup (VUID-StandaloneSpirv-None-10684).
	elementTypeID := e.backend.resolveTypeForStorageClass(elementType, storageClass)

	u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	indexID := e.backend.builder.AddConstant(u32Type, access.Index)

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

	// Determine the element type, unwrapping pointer types
	elementType, err := e.resolveAccessElementType(baseType, -1)
	if err != nil {
		return 0, fmt.Errorf("access as pointer element type: %w", err)
	}

	storageClass := e.getExpressionStorageClass(access.Base)
	// Use layout-free element type for Workgroup (VUID-StandaloneSpirv-None-10684).
	elementTypeID := e.backend.resolveTypeForStorageClass(elementType, storageClass)

	ptrType := e.backend.emitPointerType(storageClass, elementTypeID)
	return e.backend.builder.AddAccessChain(ptrType, baseID, indexID), nil
}

// emitAccessIndex emits a static index access operation.
// Returns a VALUE (auto-loads from pointers). For pointer destinations, use emitAccessIndexAsPointer.
func (e *ExpressionEmitter) emitAccessIndex(exprHandle ir.ExpressionHandle, access ir.ExprAccessIndex) (uint32, error) {
	// Get result type from type inference
	baseType, err := ir.ResolveExpressionType(e.backend.module, e.function, access.Base)
	if err != nil {
		return 0, fmt.Errorf("access index base type: %w", err)
	}

	// Check if base is a pointer expression (variable reference or nested access)
	// If so, use OpAccessChain. Otherwise, use OpCompositeExtract on the loaded value.
	isPointerBase := e.isPointerExpression(access.Base)

	// Determine the element type, unwrapping pointer types
	elementType, err := e.resolveAccessElementType(baseType, int(access.Index))
	if err != nil {
		return 0, fmt.Errorf("access index element type: %w", err)
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
		// Use layout-free element type for Workgroup (VUID-StandaloneSpirv-None-10684).
		accessElementTypeID := e.backend.resolveTypeForStorageClass(elementType, storageClass)

		ptrType := e.backend.emitPointerType(storageClass, accessElementTypeID)
		ptrID := e.backend.builder.AddAccessChain(ptrType, baseID, indexID)

		// SPIR-V forbids OpLoad on runtime-sized arrays and types containing them.
		// Return the pointer directly; downstream Access/AccessIndex will use
		// OpAccessChain to reach individual elements.
		if elementType.Handle != nil && e.backend.typeContainsRuntimeArray(*elementType.Handle) {
			return ptrID, nil
		}

		// Auto-load the value - emitExpression should return VALUES, not pointers
		return e.backend.builder.AddLoad(accessElementTypeID, ptrID), nil
	}

	// Check if the base was spilled by a previous Access expression
	if e.spilledAccesses[access.Base] {
		// Propagate spill status
		e.spilledAccesses[exprHandle] = true
		return e.maybeAccessSpilledComposite(exprHandle, elementTypeID)
	}

	// Base is already a value - use OpCompositeExtract
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}
	return e.backend.builder.AddCompositeExtract(elementTypeID, baseID, access.Index), nil
}

// resolveTypeInner extracts the TypeInner from a TypeResolution.
func (e *ExpressionEmitter) resolveTypeInner(res ir.TypeResolution) ir.TypeInner {
	if res.Handle != nil {
		return e.backend.module.Types[*res.Handle].Inner
	}
	return res.Value
}

// spillToInternalVariable spills a by-value composite expression to a
// Function-space temporary variable so it can be indexed with OpAccessChain.
// If the expression was already spilled, it just re-stores the current value.
func (e *ExpressionEmitter) spillToInternalVariable(base ir.ExpressionHandle) {
	if _, alreadySpilled := e.spilledComposites[base]; !alreadySpilled {
		// Create new Function-space variable for the base type.
		baseType, _ := ir.ResolveExpressionType(e.backend.module, e.function, base)
		baseTypeID := e.backend.resolveTypeResolution(baseType)
		ptrTypeID := e.backend.emitPointerType(StorageClassFunction, baseTypeID)

		varID := e.backend.builder.AllocID()
		ib := e.backend.newIB()
		ib.AddWord(ptrTypeID)
		ib.AddWord(varID)
		ib.AddWord(uint32(StorageClassFunction))
		e.funcBuilder.Variables = append(e.funcBuilder.Variables, ib.Build(OpVariable))

		e.spilledComposites[base] = varID
	}

	// Always store the current value (even if variable existed), matching Rust.
	baseID := e.exprIDs[base]
	if baseID == 0 {
		// Expression hasn't been emitted yet -- emit it now.
		var err error
		baseID, err = e.emitExpression(base)
		if err != nil {
			return
		}
	}
	spillVarID := e.spilledComposites[base]
	e.backend.builder.AddStore(spillVarID, baseID)

	// Mark base as spilled
	e.spilledAccesses[base] = true
}

// maybeAccessSpilledComposite generates an access to a spilled temporary.
// If the access is only used by other Access/AccessIndex expressions
// (i.e., it's an intermediate in a chain), it returns 0 without loading.
// Otherwise, it performs the actual OpAccessChain + OpLoad.
func (e *ExpressionEmitter) maybeAccessSpilledComposite(
	access ir.ExpressionHandle,
	resultTypeID uint32,
) (uint32, error) {
	// Check if this is the tip of the chain or an intermediate.
	// If ALL references to this expression come from Access/AccessIndex,
	// this is intermediate -- no load needed. The chain will be built
	// at the tip which will include all indices.
	accessUsesCount := e.accessUses[access]
	totalRefs := e.exprRefCount[access]
	if accessUsesCount > 0 && accessUsesCount == totalRefs {
		// Intermediate: all uses are from further Access/AccessIndex.
		// Don't load, don't cache a value -- the tip will build the chain.
		return 0, nil
	}

	// Tip of chain: build access chain and load the final value.
	ptrTypeID := e.backend.emitPointerType(StorageClassFunction, resultTypeID)
	ptrID, err := e.writeSpilledAccessChain(access, ptrTypeID)
	if err != nil {
		return 0, err
	}
	return e.backend.builder.AddLoad(resultTypeID, ptrID), nil
}

// writeSpilledAccessChain walks the chain of Access/AccessIndex expressions
// from the spilled composite root down to the given access expression,
// building a single OpAccessChain instruction with all index operands.
func (e *ExpressionEmitter) writeSpilledAccessChain(
	access ir.ExpressionHandle,
	resultPtrTypeID uint32,
) (uint32, error) {
	// Collect indices from access chain (in reverse order)
	var indices []uint32
	current := access
	for {
		expr := e.function.Expressions[current]
		switch k := expr.Kind.(type) {
		case ir.ExprAccess:
			indexID, err := e.emitExpression(k.Index)
			if err != nil {
				return 0, err
			}
			indices = append(indices, indexID)
			current = k.Base
		case ir.ExprAccessIndex:
			u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
			indexID := e.backend.builder.AddConstant(u32Type, k.Index)
			indices = append(indices, indexID)
			current = k.Base
		default:
			// Reached the spilled composite root
			goto done
		}
		// If we reached a spilled composite root, stop
		if _, isRoot := e.spilledComposites[current]; isRoot {
			goto done
		}
	}
done:
	// Reverse indices (we collected them from leaf to root)
	for i, j := 0, len(indices)-1; i < j; i, j = i+1, j-1 {
		indices[i], indices[j] = indices[j], indices[i]
	}

	// Get the spill variable
	spillVarID := e.spilledComposites[current]

	// Build OpAccessChain with all indices
	return e.backend.builder.AddAccessChain(resultPtrTypeID, spillVarID, indices...), nil
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
	var isMatrix bool
	var matrixSrc ir.MatrixType
	switch t := srcInner.(type) {
	case ir.ScalarType:
		srcScalar = t
	case ir.VectorType:
		srcScalar = t.Scalar
	case ir.MatrixType:
		srcScalar = t.Scalar
		isMatrix = true
		matrixSrc = t
	default:
		return 0, fmt.Errorf("as: unsupported source type %T", srcInner)
	}

	if as.Convert == nil {
		// Bitcast — preserve vector structure if source is a vector
		targetScalar := ir.ScalarType{Kind: as.Kind, Width: srcScalar.Width}
		var targetType uint32
		if vec, ok := srcInner.(ir.VectorType); ok {
			targetType = e.backend.emitInlineType(ir.VectorType{Size: vec.Size, Scalar: targetScalar})
		} else {
			targetType = e.backend.emitScalarType(targetScalar)
		}
		return e.emitConversionOp(OpBitcast, targetType, exprID), nil
	}

	targetScalar := ir.ScalarType{Kind: as.Kind, Width: *as.Convert}

	// Matrix conversion: convert column by column
	if isMatrix {
		srcColumnType := e.backend.emitInlineType(ir.VectorType{Size: matrixSrc.Rows, Scalar: srcScalar})
		dstColumnType := e.backend.emitInlineType(ir.VectorType{Size: matrixSrc.Rows, Scalar: targetScalar})
		dstMatrixType := e.backend.emitInlineType(ir.MatrixType{Columns: matrixSrc.Columns, Rows: matrixSrc.Rows, Scalar: targetScalar})

		columnIDs := make([]uint32, matrixSrc.Columns)
		convOp, convErr := selectConversionOp(srcScalar.Kind, targetScalar.Kind, srcScalar.Width, targetScalar.Width)
		if convErr != nil {
			return 0, convErr
		}
		for col := ir.VectorSize(0); col < matrixSrc.Columns; col++ {
			// Extract column
			colID := e.backend.builder.AddCompositeExtract(srcColumnType, exprID, uint32(col))
			// Convert column vector
			columnIDs[col] = e.emitConversionOp(convOp, dstColumnType, colID)
		}
		// Compose converted columns into matrix
		return e.backend.builder.AddCompositeConstruct(dstMatrixType, columnIDs...), nil
	}

	var targetTypeID uint32
	if vec, ok := srcInner.(ir.VectorType); ok {
		targetTypeID = e.backend.emitInlineType(ir.VectorType{Size: vec.Size, Scalar: targetScalar})
	} else {
		targetTypeID = e.backend.emitScalarType(targetScalar)
	}

	// Identity conversion: same kind and width — just return the expression unchanged
	if srcScalar.Kind == as.Kind && srcScalar.Width == *as.Convert {
		return exprID, nil
	}

	// Bool conversions require special handling — no single SPIR-V opcode exists.
	if srcScalar.Kind == ir.ScalarBool {
		return e.emitBoolToNumeric(targetTypeID, targetScalar, srcInner, exprID)
	}
	if as.Kind == ir.ScalarBool {
		return e.emitNumericToBool(targetTypeID, srcScalar, srcInner, exprID)
	}

	// Select conversion opcode based on source → target scalar kinds
	op, err := selectConversionOp(srcScalar.Kind, as.Kind, srcScalar.Width, targetScalar.Width)
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
	e.backend.builder.funcAppend(builder.Build(cmpOp))
	return resultID, nil
}

// selectConversionOp returns the SPIR-V opcode for a scalar type conversion.
// srcWidth and dstWidth are the byte widths of the source and destination scalars.
// When widths differ for int↔int conversions, OpSConvert/OpUConvert must be used
// instead of OpBitcast (which requires matching total bit width per SPIR-V spec).
func selectConversionOp(src, dst ir.ScalarKind, srcWidth, dstWidth uint8) (OpCode, error) {
	switch {
	case src == ir.ScalarUint && dst == ir.ScalarFloat:
		return OpConvertUToF, nil
	case src == ir.ScalarSint && dst == ir.ScalarFloat:
		return OpConvertSToF, nil
	case src == ir.ScalarFloat && dst == ir.ScalarUint:
		return OpConvertFToU, nil
	case src == ir.ScalarFloat && dst == ir.ScalarSint:
		return OpConvertFToS, nil
	case src == ir.ScalarSint && dst == ir.ScalarUint:
		if srcWidth != dstWidth {
			return OpUConvert, nil
		}
		return OpBitcast, nil
	case src == ir.ScalarUint && dst == ir.ScalarSint:
		if srcWidth != dstWidth {
			return OpSConvert, nil
		}
		return OpBitcast, nil
	case src == ir.ScalarFloat && dst == ir.ScalarFloat:
		// Same kind, different width (e.g. f32→f16, f32→f64)
		return OpFConvert, nil
	case src == ir.ScalarSint && dst == ir.ScalarSint:
		return OpSConvert, nil
	case src == ir.ScalarUint && dst == ir.ScalarUint:
		return OpUConvert, nil
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
	e.backend.builder.funcAppend(builder.Build(op))
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

	// Get the pointer expression's type and dereference it to find the loaded value type.
	// Pointer expressions (ExprLocalVariable, ExprGlobalVariable, etc.) resolve to
	// PointerType/ValuePointerType. OpLoad needs the pointed-TO type, not the pointer type.
	pointerType, err := ir.ResolveExpressionType(e.backend.module, e.function, load.Pointer)
	if err != nil {
		return 0, fmt.Errorf("load pointer type: %w", err)
	}

	// For Workgroup pointers, use layout-free types (VUID-StandaloneSpirv-None-10684).
	// The OpLoad result type must match the pointer's pointee type, which is layout-free.
	storageClass := e.getExpressionStorageClass(load.Pointer)
	resultType := e.dereferencePointerTypeForStorageClass(pointerType, storageClass)
	return e.backend.builder.AddLoad(resultType, pointerID), nil
}

// dereferencePointerTypeForStorageClass extracts the base type, using layout-free types
// for Workgroup storage class per VUID-StandaloneSpirv-None-10684.
func (e *ExpressionEmitter) dereferencePointerTypeForStorageClass(res ir.TypeResolution, storageClass StorageClass) uint32 {
	if storageClass == StorageClassWorkgroup {
		var inner ir.TypeInner
		if res.Handle != nil {
			if int(*res.Handle) < len(e.backend.module.Types) {
				inner = e.backend.module.Types[*res.Handle].Inner
			}
		} else {
			inner = res.Value
		}
		if pt, ok := inner.(ir.PointerType); ok {
			// For Atomic base types, resolve to the underlying scalar (matches Rust).
			if int(pt.Base) < len(e.backend.module.Types) {
				if at, ok := e.backend.module.Types[pt.Base].Inner.(ir.AtomicType); ok {
					return e.backend.emitInlineType(at.Scalar)
				}
			}
			return e.backend.emitTypeWithoutLayout(pt.Base)
		}
	}
	return e.dereferencePointerType(res)
}

// dereferencePointerType extracts the base (value) type from a pointer type resolution.
// For PointerType → base type handle, for ValuePointerType → scalar or vector type.
// If not a pointer type, returns the type as-is (backwards compat).
func (e *ExpressionEmitter) dereferencePointerType(res ir.TypeResolution) uint32 {
	var inner ir.TypeInner
	if res.Handle != nil {
		if int(*res.Handle) < len(e.backend.module.Types) {
			inner = e.backend.module.Types[*res.Handle].Inner
		}
	} else {
		inner = res.Value
	}

	switch pt := inner.(type) {
	case ir.PointerType:
		// Dereference: Pointer{base} → base type
		// For Atomic base types, resolve to the underlying scalar (matches Rust).
		if int(pt.Base) < len(e.backend.module.Types) {
			if at, ok := e.backend.module.Types[pt.Base].Inner.(ir.AtomicType); ok {
				return e.backend.emitInlineType(at.Scalar)
			}
		}
		id, err := e.backend.emitType(pt.Base)
		if err != nil {
			panic(fmt.Sprintf("failed to emit pointer base type: %v", err))
		}
		return id
	case ir.ValuePointerType:
		// Dereference: ValuePointer{size, scalar} → scalar or vector
		if pt.Size != nil {
			scalarID := e.backend.emitScalarType(pt.Scalar)
			return e.backend.emitVectorType(scalarID, uint32(*pt.Size))
		}
		return e.backend.emitScalarType(pt.Scalar)
	default:
		// Not a pointer type — return as-is (fallback for backwards compat)
		return e.backend.resolveTypeResolution(res)
	}
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
			// Matrix + Matrix: decompose into column-wise FAdd, then reassemble.
			// SPIR-V FAdd only works on scalar/vector, not matrix types.
			// Matches Rust naga's write_matrix_matrix_column_op (block.rs:2493).
			leftInner := typeResolutionInner(e.backend.module, leftType)
			if mat, ok := leftInner.(ir.MatrixType); ok {
				return e.emitMatrixColumnOp(OpFAdd, resultType, leftID, rightID, mat), nil
			}
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
			// Matrix - Matrix: decompose into column-wise FSub
			leftInner := typeResolutionInner(e.backend.module, leftType)
			if mat, ok := leftInner.(ir.MatrixType); ok {
				return e.emitMatrixColumnOp(OpFSub, resultType, leftID, rightID, mat), nil
			}
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
			rightMat, rightIsMat := rightInner.(ir.MatrixType)

			switch {
			case leftIsMat && rightIsVec:
				// mat * vec -> OpMatrixTimesVector
				// Result type is vec<Rows> (number of rows in the matrix).
				vecScalarID := e.backend.emitScalarType(leftMat.Scalar)
				vecTypeID := e.backend.emitVectorType(vecScalarID, uint32(leftMat.Rows))
				return e.backend.builder.AddBinaryOp(OpMatrixTimesVector, vecTypeID, leftID, rightID), nil
			case leftIsVec && rightIsMat:
				// vec * mat -> OpVectorTimesMatrix
				// Result type is a vector with size = number of columns in the matrix.
				vecScalarID := e.backend.emitScalarType(rightMat.Scalar)
				vecTypeID := e.backend.emitVectorType(vecScalarID, uint32(rightMat.Columns))
				return e.backend.builder.AddBinaryOp(OpVectorTimesMatrix, vecTypeID, leftID, rightID), nil
			case leftIsMat && rightIsMat:
				// mat * mat -> OpMatrixTimesMatrix
				// Result type is mat<Columns=right.Columns, Rows=left.Rows>
				colScalarID := e.backend.emitScalarType(leftMat.Scalar)
				colVecID := e.backend.emitVectorType(colScalarID, uint32(leftMat.Rows))
				matTypeID := e.backend.emitMatrixType(colVecID, uint32(rightMat.Columns))
				return e.backend.builder.AddBinaryOp(OpMatrixTimesMatrix, matTypeID, leftID, rightID), nil
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
			// Integer multiplication: OpIMul requires matching types.
			// For vector*scalar or scalar*vector, splat the scalar to match.
			// Matches Rust naga's write_vector_scalar_mult (block.rs:2548).
			rightType, _ := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			leftInner := typeResolutionInner(e.backend.module, leftType)
			rightInner := typeResolutionInner(e.backend.module, rightType)
			leftVec, leftIsVec := leftInner.(ir.VectorType)
			rightVec, rightIsVec := rightInner.(ir.VectorType)
			_, leftIsScalar := leftInner.(ir.ScalarType)
			_, rightIsScalar := rightInner.(ir.ScalarType)

			switch {
			case leftIsVec && rightIsScalar:
				// vec * scalar -> splat scalar, then IMul
				vecTypeID := e.backend.resolveTypeResolution(leftType)
				splatID := e.splatScalarToVector(rightID, leftVec)
				return e.backend.builder.AddBinaryOp(OpIMul, vecTypeID, leftID, splatID), nil
			case leftIsScalar && rightIsVec:
				// scalar * vec -> splat scalar, then IMul
				vecTypeID := e.backend.resolveTypeResolution(rightType)
				splatID := e.splatScalarToVector(leftID, rightVec)
				return e.backend.builder.AddBinaryOp(OpIMul, vecTypeID, splatID, rightID), nil
			default:
				opcode = OpIMul
			}
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
		} else {
			// Integer divide: use wrapped function for safety
			rightType, _ := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			leftTypeID := e.backend.resolveTypeResolution(leftType)
			rightTypeID := e.backend.resolveTypeResolution(rightType)
			key := wrappedBinaryOp{op: binary.Op, leftTypeID: leftTypeID, rightTypeID: rightTypeID}
			if wrapperID, ok := e.backend.wrappedFuncIDs[key]; ok {
				return e.emitFunctionCallWrapped(resultType, wrapperID, leftID, rightID), nil
			}
			// Fallback (shouldn't happen if scanning was correct)
			if scalarKind == ir.ScalarSint {
				opcode = OpSDiv
			} else {
				opcode = OpUDiv
			}
		}
	case ir.BinaryModulo:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFMod
			rightType, rErr := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			if rErr == nil {
				leftID, rightID, resultType = e.promoteScalarToVector(leftType, rightType, leftID, rightID, resultType)
			}
		} else {
			// Integer modulo: use wrapped function for safety
			rightType, _ := ir.ResolveExpressionType(e.backend.module, e.function, binary.Right)
			leftTypeID := e.backend.resolveTypeResolution(leftType)
			rightTypeID := e.backend.resolveTypeResolution(rightType)
			key := wrappedBinaryOp{op: binary.Op, leftTypeID: leftTypeID, rightTypeID: rightTypeID}
			if wrapperID, ok := e.backend.wrappedFuncIDs[key]; ok {
				return e.emitFunctionCallWrapped(resultType, wrapperID, leftID, rightID), nil
			}
			// Fallback (shouldn't happen if scanning was correct)
			if scalarKind == ir.ScalarSint {
				opcode = OpSRem // Match Rust: Modulo on Sint uses OpSRem
			} else {
				opcode = OpUMod
			}
		}
	case ir.BinaryEqual:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdEqual
		} else if scalarKind == ir.ScalarBool {
			opcode = OpLogicalEqual
		} else {
			opcode = OpIEqual
		}
	case ir.BinaryNotEqual:
		if scalarKind == ir.ScalarFloat {
			opcode = OpFOrdNotEqual
		} else if scalarKind == ir.ScalarBool {
			opcode = OpLogicalNotEqual
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
		if scalarKind == ir.ScalarBool {
			opcode = OpLogicalAnd
		} else {
			opcode = OpBitwiseAnd
		}
	case ir.BinaryExclusiveOr:
		opcode = OpBitwiseXor
	case ir.BinaryInclusiveOr:
		if scalarKind == ir.ScalarBool {
			opcode = OpLogicalOr
		} else {
			opcode = OpBitwiseOr
		}
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

// emitFunctionCallWrapped emits an OpFunctionCall to a wrapped helper function.
func (e *ExpressionEmitter) emitFunctionCallWrapped(resultTypeID, funcID, leftID, rightID uint32) uint32 {
	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultTypeID)
	ib.AddWord(resultID)
	ib.AddWord(funcID)
	ib.AddWord(leftID)
	ib.AddWord(rightID)
	e.backend.builder.funcAppend(ib.Build(OpFunctionCall))
	return resultID
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
	e.backend.builder.funcAppend(builder.Build(OpCompositeConstruct))
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
		e.backend.builder.funcAppend(builder.Build(OpFOrdNotEqual))
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
		e.backend.builder.funcAppend(zb.Build(OpCompositeConstruct))
		// Compare
		cmpID := e.backend.builder.AllocID()
		cb := e.newIB()
		cb.AddWord(boolVecTypeID)
		cb.AddWord(cmpID)
		cb.AddWord(conditionID)
		cb.AddWord(zeroVecID)
		e.backend.builder.funcAppend(cb.Build(OpFOrdNotEqual))
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
			e.backend.builder.funcAppend(builder.Build(OpCompositeConstruct))
			conditionID = splatID
		}
	}

	return e.backend.builder.AddSelect(resultType, conditionID, acceptID, rejectID), nil
}

// emitStatement emits a statement.
func (e *ExpressionEmitter) emitStatement(stmt ir.Statement) error {
	// If currentBlock is nil, we're in dead code (after break/continue/return/kill).
	// Skip emission entirely.
	if e.currentBlock == nil {
		return nil
	}

	switch kind := stmt.Kind.(type) {
	case ir.StmtEmit:
		// Emit all expressions in range.
		// Skip variable reference expressions (GlobalVariable, LocalVariable) —
		// they are pointer expressions in the WGSL Load Rule model and should
		// not be auto-loaded here. The ExprLoad wrapping them handles the load.
		for handle := kind.Range.Start; handle < kind.Range.End; handle++ {
			if e.isVariableReference(handle) {
				continue
			}
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
		if e.loopCtx.BreakID == 0 {
			return fmt.Errorf("break statement outside of loop or switch")
		}
		e.consumeBlock(makeBranchInstruction(e.loopCtx.BreakID))
		return nil

	case ir.StmtContinue:
		if e.loopCtx.ContinuingID == 0 {
			return fmt.Errorf("continue statement outside of loop")
		}
		e.consumeBlock(makeBranchInstruction(e.loopCtx.ContinuingID))
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
						if varID == 0 {
							continue
						}
						// Get the member type from the result type
						resultType := e.backend.module.Types[e.function.Result.Type].Inner.(ir.StructType)
						memberTypeID, err := e.backend.emitType(resultType.Members[memberIdx].Type)
						if err != nil {
							return err
						}
						// Extract member value using OpCompositeExtract
						memberValue := e.backend.builder.AddCompositeExtract(memberTypeID, valueID, uint32(memberIdx))
						// F16 polyfill: convert f16 -> f32 before storing
						if f32TypeID, ok := e.backend.f16PolyfillVars[varID]; ok {
							convertedID := e.backend.builder.AllocID()
							e.backend.ib.Reset()
							e.backend.ib.AddWord(f32TypeID) // f32 result type
							e.backend.ib.AddWord(convertedID)
							e.backend.ib.AddWord(memberValue)
							e.backend.builder.funcAppend(e.backend.ib.Build(OpFConvert))
							memberValue = convertedID
						}
						// Store to output variable (SampleMask needs AccessChain[0])
						if e.backend.sampleMaskVars[varID] {
							elemPtrType := e.backend.emitPointerType(StorageClassOutput, memberTypeID)
							u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
							idx0 := e.backend.builder.AddConstant(u32Type, 0)
							elemPtr := e.backend.builder.AddAccessChain(elemPtrType, varID, idx0)
							e.backend.builder.AddStore(elemPtr, memberValue)
						} else {
							e.backend.builder.AddStore(varID, memberValue)
						}
					}
				} else if e.output.singleVarID != 0 {
					// Single output variable
					storeValue := valueID
					if f32TypeID, ok := e.backend.f16PolyfillVars[e.output.singleVarID]; ok {
						// F16 polyfill: convert f16 -> f32 before storing
						convertedID := e.backend.builder.AllocID()
						e.backend.ib.Reset()
						e.backend.ib.AddWord(f32TypeID)
						e.backend.ib.AddWord(convertedID)
						e.backend.ib.AddWord(valueID)
						e.backend.builder.funcAppend(e.backend.ib.Build(OpFConvert))
						storeValue = convertedID
					}
					// SampleMask output needs AccessChain[0]
					if e.backend.sampleMaskVars[e.output.singleVarID] {
						resultTypeID, _ := e.backend.emitType(e.function.Result.Type)
						elemPtrType := e.backend.emitPointerType(StorageClassOutput, resultTypeID)
						u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
						idx0 := e.backend.builder.AddConstant(u32Type, 0)
						elemPtr := e.backend.builder.AddAccessChain(elemPtrType, e.output.singleVarID, idx0)
						e.backend.builder.AddStore(elemPtr, storeValue)
					} else {
						e.backend.builder.AddStore(e.output.singleVarID, storeValue)
					}
				}
				e.consumeBlock(Instruction{Opcode: OpReturn})
			} else {
				e.consumeBlock(Instruction{Opcode: OpReturnValue, Words: []uint32{valueID}})
			}
		} else {
			// Return void
			e.consumeBlock(Instruction{Opcode: OpReturn})
		}
		return nil

	case ir.StmtKill:
		e.consumeBlock(Instruction{Opcode: OpKill})
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

		// Check for Workgroup ↔ non-Workgroup type mismatch (VUID-StandaloneSpirv-None-10684).
		// When storing a value loaded from Workgroup (layout-free types) into a non-Workgroup
		// pointer (decorated types), or vice versa, the SPIR-V type IDs differ even though
		// the types are logically equivalent. OpCopyLogical (SPIR-V 1.4+) bridges this gap.
		valueID = e.maybeCopyLogicalForStore(kind.Pointer, kind.Value, valueID)

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

	case ir.StmtImageAtomic:
		return e.emitImageAtomic(kind)

	case ir.StmtSubgroupBallot:
		return e.emitSubgroupBallot(kind)

	case ir.StmtSubgroupCollectiveOperation:
		return e.emitSubgroupCollectiveOperation(kind)

	case ir.StmtSubgroupGather:
		return e.emitSubgroupGather(kind)

	case ir.StmtRayQuery:
		return e.emitRayQuery(kind)

	case ir.StmtWorkGroupUniformLoad:
		return e.emitWorkGroupUniformLoad(kind)

	default:
		return fmt.Errorf("unsupported statement kind: %T", kind)
	}
}

// emitIf emits an if statement using the block model.
func (e *ExpressionEmitter) emitIf(stmt ir.StmtIf) error {
	// Evaluate condition (emitted into current block)
	conditionID, err := e.emitExpression(stmt.Condition)
	if err != nil {
		return err
	}

	// Allocate labels
	acceptLabel := e.backend.builder.AllocID()
	rejectLabel := e.backend.builder.AllocID()
	mergeLabel := e.backend.builder.AllocID()

	// Push SelectionMerge into current block body, then consume with BranchConditional
	e.backend.builder.AddSelectionMerge(mergeLabel, SelectionControlNone)
	e.consumeBlock(Instruction{
		Opcode: OpBranchConditional,
		Words:  []uint32{conditionID, acceptLabel, rejectLabel},
	})

	// Accept block
	acceptBlock := NewBlock(acceptLabel)
	e.setCurrentBlock(&acceptBlock)
	for _, acceptStmt := range stmt.Accept {
		if err := e.emitStatement(acceptStmt); err != nil {
			return err
		}
	}
	// If accept block is still live (not consumed by break/continue/return/kill),
	// terminate it with branch to merge.
	acceptTerminated := e.currentBlock == nil
	if !acceptTerminated {
		e.consumeBlock(makeBranchInstruction(mergeLabel))
	}

	// Reject block
	rejectBlock := NewBlock(rejectLabel)
	e.setCurrentBlock(&rejectBlock)
	for _, rejectStmt := range stmt.Reject {
		if err := e.emitStatement(rejectStmt); err != nil {
			return err
		}
	}
	rejectTerminated := e.currentBlock == nil
	if !rejectTerminated {
		e.consumeBlock(makeBranchInstruction(mergeLabel))
	}

	// Merge block — always created (SPIR-V requires it for SelectionMerge).
	mergeBlock := NewBlock(mergeLabel)
	e.setCurrentBlock(&mergeBlock)

	// If both branches terminated (return/kill/break/continue), the merge block
	// is unreachable but SPIR-V still requires a terminator.
	if acceptTerminated && rejectTerminated {
		e.consumeBlock(Instruction{Opcode: OpUnreachable})
	}

	return nil
}

// findLastDeferredResultInTree finds the ExprCallResult with the highest expression
// handle in an expression tree. When an init expression contains multiple call
// results (e.g., `var x = f() + g()`), the deferred store must be triggered by
// the LAST call to complete. Since StmtCalls are emitted in expression handle
// order, the highest handle corresponds to the last call emitted — by which
// point all earlier call results are already cached in callResultIDs.
func findLastDeferredResultInTree(expressions []ir.Expression, handle ir.ExpressionHandle) (ir.ExpressionHandle, bool) {
	if int(handle) >= len(expressions) {
		return 0, false
	}
	expr := expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprCallResult:
		return handle, true
	case ir.ExprAtomicResult:
		return handle, true
	case ir.ExprSubgroupBallotResult:
		return handle, true
	case ir.ExprSubgroupOperationResult:
		return handle, true
	case ir.ExprBinary:
		best, found := findLastDeferredResultInTree(expressions, k.Left)
		if h, ok := findLastDeferredResultInTree(expressions, k.Right); ok {
			if !found || h > best {
				best = h
			}
			found = true
		}
		return best, found
	case ir.ExprUnary:
		return findLastDeferredResultInTree(expressions, k.Expr)
	case ir.ExprAccessIndex:
		return findLastDeferredResultInTree(expressions, k.Base)
	case ir.ExprAccess:
		return findLastDeferredResultInTree(expressions, k.Base)
	case ir.ExprSwizzle:
		return findLastDeferredResultInTree(expressions, k.Vector)
	case ir.ExprLoad:
		return findLastDeferredResultInTree(expressions, k.Pointer)
	case ir.ExprCompose:
		best := ir.ExpressionHandle(0)
		found := false
		for _, comp := range k.Components {
			if h, ok := findLastDeferredResultInTree(expressions, comp); ok {
				if !found || h > best {
					best = h
				}
				found = true
			}
		}
		return best, found
	case ir.ExprAs:
		return findLastDeferredResultInTree(expressions, k.Expr)
	case ir.ExprSplat:
		return findLastDeferredResultInTree(expressions, k.Value)
	case ir.ExprSelect:
		best, found := findLastDeferredResultInTree(expressions, k.Condition)
		if h, ok := findLastDeferredResultInTree(expressions, k.Accept); ok {
			if !found || h > best {
				best = h
			}
			found = true
		}
		if h, ok := findLastDeferredResultInTree(expressions, k.Reject); ok {
			if !found || h > best {
				best = h
			}
			found = true
		}
		return best, found
	case ir.ExprMath:
		best, found := findLastDeferredResultInTree(expressions, k.Arg)
		if k.Arg1 != nil {
			if h, ok := findLastDeferredResultInTree(expressions, *k.Arg1); ok {
				if !found || h > best {
					best = h
				}
				found = true
			}
		}
		if k.Arg2 != nil {
			if h, ok := findLastDeferredResultInTree(expressions, *k.Arg2); ok {
				if !found || h > best {
					best = h
				}
				found = true
			}
		}
		if k.Arg3 != nil {
			if h, ok := findLastDeferredResultInTree(expressions, *k.Arg3); ok {
				if !found || h > best {
					best = h
				}
				found = true
			}
		}
		return best, found
	case ir.ExprDerivative:
		return findLastDeferredResultInTree(expressions, k.Expr)
	case ir.ExprRelational:
		return findLastDeferredResultInTree(expressions, k.Argument)
	case ir.ExprArrayLength:
		return findLastDeferredResultInTree(expressions, k.Array)
	default:
		return 0, false
	}
}

// findDeferredLocalVarRef checks if an expression tree contains a reference
// (ExprLocalVariable) to any local variable in the deferred set. Returns the
// index of the first deferred var found, or -1 if none.
func findDeferredLocalVarRef(expressions []ir.Expression, handle ir.ExpressionHandle, deferredSet map[int]bool) int {
	if int(handle) >= len(expressions) {
		return -1
	}
	expr := expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprLocalVariable:
		if deferredSet[int(k.Variable)] {
			return int(k.Variable)
		}
		return -1
	case ir.ExprBinary:
		if idx := findDeferredLocalVarRef(expressions, k.Left, deferredSet); idx >= 0 {
			return idx
		}
		return findDeferredLocalVarRef(expressions, k.Right, deferredSet)
	case ir.ExprUnary:
		return findDeferredLocalVarRef(expressions, k.Expr, deferredSet)
	case ir.ExprLoad:
		return findDeferredLocalVarRef(expressions, k.Pointer, deferredSet)
	case ir.ExprAccessIndex:
		return findDeferredLocalVarRef(expressions, k.Base, deferredSet)
	case ir.ExprAccess:
		return findDeferredLocalVarRef(expressions, k.Base, deferredSet)
	case ir.ExprAs:
		return findDeferredLocalVarRef(expressions, k.Expr, deferredSet)
	default:
		return -1
	}
}

// emitLoop emits a loop statement using the block model.
func (e *ExpressionEmitter) emitLoop(stmt ir.StmtLoop) error {
	// Allocate labels
	headerLabel := e.backend.builder.AllocID()
	bodyLabel := e.backend.builder.AllocID()
	continuingLabel := e.backend.builder.AllocID()
	mergeLabel := e.backend.builder.AllocID()

	// Consume current block with branch to header
	e.consumeBlock(makeBranchInstruction(headerLabel))

	// Header block: LoopMerge + Branch to body (or to loop bounding check)
	headerBlock := NewBlock(headerLabel)
	e.setCurrentBlock(&headerBlock)
	e.backend.builder.AddLoopMerge(mergeLabel, continuingLabel, LoopControlNone)

	if e.backend.options.ForceLoopBounding {
		e.emitForceLoopBounding(mergeLabel, bodyLabel)
	} else {
		e.consumeBlock(makeBranchInstruction(bodyLabel))
	}

	// Save outer loop context and set new one (value semantics = isolated copy)
	outerLoopCtx := e.loopCtx
	e.loopCtx = LoopContext{
		ContinuingID: continuingLabel,
		BreakID:      mergeLabel,
	}

	// Body block
	bodyBlock := NewBlock(bodyLabel)
	e.setCurrentBlock(&bodyBlock)
	for _, bodyStmt := range stmt.Body {
		if err := e.emitStatement(bodyStmt); err != nil {
			return err
		}
	}
	// If body block is still live, branch to continuing
	if e.currentBlock != nil {
		e.consumeBlock(makeBranchInstruction(continuingLabel))
	}

	// Continuing block
	continuingBlock := NewBlock(continuingLabel)
	e.setCurrentBlock(&continuingBlock)
	for _, continueStmt := range stmt.Continuing {
		if err := e.emitStatement(continueStmt); err != nil {
			return err
		}
	}

	// Terminate continuing block: break-if or unconditional back-edge
	if stmt.BreakIf != nil {
		breakCondID, err := e.emitExpression(*stmt.BreakIf)
		if err != nil {
			return err
		}
		e.consumeBlock(Instruction{
			Opcode: OpBranchConditional,
			Words:  []uint32{breakCondID, mergeLabel, headerLabel},
		})
	} else {
		e.consumeBlock(makeBranchInstruction(headerLabel))
	}

	// Restore outer loop context
	e.loopCtx = outerLoopCtx

	// Merge block — continuation after the loop
	mergeBlock := NewBlock(mergeLabel)
	e.setCurrentBlock(&mergeBlock)

	return nil
}

// emitForceLoopBounding inserts a decrementing counter check that breaks out
// of the loop when the counter reaches zero, preventing infinite loops from
// hanging the GPU. This matches Rust naga's write_force_bounded_loop_instructions.
//
// The counter is a vec2<u32> initialized to (u32::MAX, u32::MAX), simulating
// a ~64-bit counter. Each iteration decrements the low word, and when it
// underflows, also decrements the high word.
//
// Must be called after OpLoopMerge is emitted in the header block and before
// the loop body. The current block (header) is consumed; after this method,
// the current block is a new block that should branch to bodyLabel.
func (e *ExpressionEmitter) emitForceLoopBounding(mergeLabel, bodyLabel uint32) {
	// Get type IDs
	u32Type := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	vec2u32Type := e.backend.resolveTypeResolution(ir.TypeResolution{
		Value: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, Size: 2},
	})
	vec2u32PtrType := e.backend.emitPointerType(StorageClassFunction, vec2u32Type)
	boolType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	vec2BoolType := e.backend.resolveTypeResolution(ir.TypeResolution{
		Value: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, Size: 2},
	})

	// Constants
	zeroU32 := e.backend.builder.AddConstant(u32Type, 0)
	oneU32 := e.backend.builder.AddConstant(u32Type, 1)
	maxU32 := e.backend.builder.AddConstant(u32Type, 0xFFFFFFFF)
	zeroVec2 := e.backend.builder.AddConstantComposite(vec2u32Type, zeroU32, zeroU32)
	maxVec2 := e.backend.builder.AddConstantComposite(vec2u32Type, maxU32, maxU32)

	// Create loop counter variable in function prologue (initialized to maxVec2)
	counterVarID := e.backend.builder.AllocID()
	ib := e.backend.newIB()
	ib.AddWord(vec2u32PtrType)
	ib.AddWord(counterVarID)
	ib.AddWord(uint32(StorageClassFunction))
	ib.AddWord(maxVec2) // initializer
	e.funcBuilder.Variables = append(e.funcBuilder.Variables, ib.Build(OpVariable))

	// Allocate block labels
	breakCheckLabel := e.backend.builder.AllocID()
	decBlockLabel := e.backend.builder.AllocID()

	// Header block -> branch to break check
	e.consumeBlock(makeBranchInstruction(breakCheckLabel))

	// Break-check block
	breakCheckBlock := NewBlock(breakCheckLabel)
	e.setCurrentBlock(&breakCheckBlock)

	// Load the counter
	loadID := e.backend.builder.AddLoad(vec2u32Type, counterVarID)

	// Check if both components are zero: IEqual(vec2(0,0), counter) -> vec2<bool>
	eqID := e.backend.builder.AddBinaryOp(OpIEqual, vec2BoolType, zeroVec2, loadID)

	// All(vec2<bool>) -> bool
	allEqID := e.backend.builder.AddUnaryOp(OpAll, boolType, eqID)

	// SelectionMerge to decrement block
	e.backend.builder.AddSelectionMerge(decBlockLabel, SelectionControlNone)

	// BranchConditional: if all zero -> break (merge), else -> decrement
	e.consumeBlock(Instruction{
		Opcode: OpBranchConditional,
		Words:  []uint32{allEqID, mergeLabel, decBlockLabel},
	})

	// Decrement block: decrement the counter
	decBlock := NewBlock(decBlockLabel)
	e.setCurrentBlock(&decBlock)

	// Extract low word (index 1): counter.y
	lowID := e.backend.builder.AddCompositeExtract(u32Type, loadID, 1)

	// Check if low overflows: IEqual(low, 0) -> bool
	lowOverflowID := e.backend.builder.AddBinaryOp(OpIEqual, boolType, lowID, zeroU32)

	// Select carry bit: select(lowOverflow, 1, 0) -> u32
	carryBitID := e.backend.builder.AddSelect(u32Type, lowOverflowID, oneU32, zeroU32)

	// Construct decrement vector: vec2(carryBit, 1)
	decrementID := e.backend.builder.AddCompositeConstruct(vec2u32Type, carryBitID, oneU32)

	// Subtract: counter - decrement
	resultID := e.backend.builder.AddBinaryOp(OpISub, vec2u32Type, loadID, decrementID)

	// Store result back
	e.backend.builder.AddStore(counterVarID, resultID)

	// Branch to body
	e.consumeBlock(makeBranchInstruction(bodyLabel))
}

// emitSwitch emits a switch statement using the block model.
func (e *ExpressionEmitter) emitSwitch(stmt ir.StmtSwitch) error {
	// Evaluate selector (emitted into current block)
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

	// Push SelectionMerge into current block body
	e.backend.builder.AddSelectionMerge(mergeLabel, SelectionControlNone)

	// Build OpSwitch terminator instruction
	switchWords := []uint32{selectorID, defaultLabel}
	for i, c := range stmt.Cases {
		switch v := c.Value.(type) {
		case ir.SwitchValueI32:
			// #nosec G115 - int32 to uint32 conversion for SPIR-V literal encoding
			switchWords = append(switchWords, uint32(v)&0xFFFFFFFF, caseLabels[i])
		case ir.SwitchValueU32:
			switchWords = append(switchWords, uint32(v), caseLabels[i])
		case ir.SwitchValueDefault:
			continue
		}
	}
	e.consumeBlock(Instruction{Opcode: OpSwitch, Words: switchWords})

	// Save outer break target and set new one for this switch
	outerLoopCtx := e.loopCtx
	e.loopCtx.BreakID = mergeLabel

	// Emit each case block
	allCasesTerminated := true
	for i, c := range stmt.Cases {
		caseBlock := NewBlock(caseLabels[i])
		e.setCurrentBlock(&caseBlock)

		for _, bodyStmt := range c.Body {
			if err := e.emitStatement(bodyStmt); err != nil {
				return fmt.Errorf("switch case body: %w", err)
			}
		}

		// If case block is still live, branch to appropriate target
		if e.currentBlock != nil {
			allCasesTerminated = false
			var targetLabel uint32
			if c.FallThrough && i < len(stmt.Cases)-1 {
				targetLabel = caseLabels[i+1]
			} else {
				targetLabel = mergeLabel
			}
			e.consumeBlock(makeBranchInstruction(targetLabel))
		}
	}

	// Restore outer loop context
	e.loopCtx = outerLoopCtx

	// Merge block
	mergeBlock := NewBlock(mergeLabel)
	e.setCurrentBlock(&mergeBlock)

	// If all cases terminated, merge block is unreachable
	if allCasesTerminated {
		e.consumeBlock(Instruction{Opcode: OpUnreachable})
	}

	return nil
}

// emitMath emits a math built-in function using GLSL.std.450.
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
	// Integer dot product expansion (when OpDot can't be used)
	var useIntegerDot bool
	var intDotScalar ir.ScalarType
	var intDotSize int
	// FMix: may need to splat scalar selector to vector
	var needsMixSplat bool
	// Pack/Unpack 4x8 integer polyfill (matching Rust naga block.rs:2725/2869)
	var usePack4x8 bool
	var pack4x8Signed bool
	var pack4x8Clamp bool
	var useUnpack4x8 bool
	var unpack4x8Signed bool

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
		// Saturate = clamp(x, 0, 1). Construct 0 and 1 constants and emit FClamp.
		{
			inner := ir.TypeResInner(e.backend.module, argType)
			var floatScalar ir.ScalarType
			var maybeSize uint8
			switch t := inner.(type) {
			case ir.ScalarType:
				floatScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: t.Width}
			case ir.VectorType:
				floatScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: t.Scalar.Width}
				maybeSize = uint8(t.Size)
			}
			zeroTypeID := e.backend.emitScalarType(floatScalar)
			oneTypeID := zeroTypeID
			var zeroID, oneID uint32
			if floatScalar.Width == 2 {
				zeroID = e.backend.builder.AddConstant(zeroTypeID, 0)
				oneID = e.backend.builder.AddConstant(oneTypeID, float32ToF16Bits(1.0))
			} else if floatScalar.Width == 8 {
				zeroID = e.backend.builder.AddConstantFloat64(zeroTypeID, 0.0)
				oneID = e.backend.builder.AddConstantFloat64(oneTypeID, 1.0)
			} else {
				zeroID = e.backend.builder.AddConstantFloat32(zeroTypeID, 0.0)
				oneID = e.backend.builder.AddConstantFloat32(oneTypeID, 1.0)
			}
			if maybeSize > 0 {
				// Vector: create composite constants
				vecTypeID := e.backend.emitVectorType(zeroTypeID, uint32(maybeSize))
				zeroComponents := make([]uint32, maybeSize)
				oneComponents := make([]uint32, maybeSize)
				for i := range zeroComponents {
					zeroComponents[i] = zeroID
					oneComponents[i] = oneID
				}
				zeroID = e.backend.builder.AddConstantComposite(vecTypeID, zeroComponents...)
				oneID = e.backend.builder.AddConstantComposite(vecTypeID, oneComponents...)
			}
			return e.backend.builder.AddExtInst(resultType, e.backend.glslExtID, GLSLstd450FClamp, argID, zeroID, oneID), nil
		}

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
		glslInst = GLSLstd450ModfStruct
	case ir.MathFrexp:
		glslInst = GLSLstd450FrexpStruct
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
		// OpDot is a native SPIR-V instruction, but ONLY for float vectors.
		// For integer vectors, we must manually expand: sum of component-wise products.
		// Matches Rust naga's write_dot_product fallback (block.rs).
		argInner := ir.TypeResInner(e.backend.module, argType)
		if vecType, ok := argInner.(ir.VectorType); ok && vecType.Scalar.Kind != ir.ScalarFloat {
			// Integer dot product — manual expansion
			useIntegerDot = true
			intDotScalar = vecType.Scalar
			intDotSize = int(vecType.Size)
		} else {
			useNativeOpcode = true
			nativeOpcode = OpDot
		}
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
		// FMix requires all operands to match Result Type.
		// When selector (arg2) is scalar but args are vector, splat the selector.
		// Matches Rust naga's Mix handling (block.rs:1263).
		needsMixSplat = true
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
	case ir.MathTranspose:
		// OpTranspose is a native SPIR-V instruction (unary)
		useNativeOpcode = true
		nativeOpcode = OpTranspose

	// QuantizeToF16: native SPIR-V instruction
	case ir.MathQuantizeF16:
		useNativeOpcode = true
		nativeOpcode = OpQuantizeToF16

	// Bit manipulation functions
	case ir.MathCountTrailingZeros:
		// WGSL countTrailingZeros -> GLSL.std.450 FindILsb
		glslInst = GLSLstd450FindILsb
	case ir.MathCountLeadingZeros:
		// WGSL countLeadingZeros -> polyfill: (31 - FindUMsb(x)) for u32, or FindSMsb for i32
		// Simplified: use FindUMsb for unsigned, FindSMsb for signed
		// Note: GLSL FindUMsb returns the bit position of the MSB (undefined for 0)
		// WGSL countLeadingZeros expects the count. We emit FindUMsb/FindSMsb and
		// the result subtraction is done at IR level by the lowerer.
		if scalarKind == ir.ScalarSint {
			glslInst = GLSLstd450FindSMsb
		} else {
			glslInst = GLSLstd450FindUMsb
		}
	case ir.MathCountOneBits:
		// OpBitCount is a native SPIR-V instruction
		useNativeOpcode = true
		nativeOpcode = OpBitCount
	case ir.MathReverseBits:
		// OpBitReverse is a native SPIR-V instruction
		useNativeOpcode = true
		nativeOpcode = OpBitReverse
	case ir.MathFirstTrailingBit:
		// WGSL firstTrailingBit -> GLSL.std.450 FindILsb
		glslInst = GLSLstd450FindILsb
	case ir.MathFirstLeadingBit:
		// WGSL firstLeadingBit -> FindSMsb (signed) or FindUMsb (unsigned)
		if scalarKind == ir.ScalarSint {
			glslInst = GLSLstd450FindSMsb
		} else {
			glslInst = GLSLstd450FindUMsb
		}

	// ExtractBits and InsertBits: native SPIR-V opcodes with 3 or 4 operands.
	// These are handled specially below because they have more than 2 operands.
	case ir.MathExtractBits:
		// OpBitFieldSExtract (signed) or OpBitFieldUExtract (unsigned):
		// result-type result-id base offset count
		if scalarKind == ir.ScalarSint {
			nativeOpcode = OpBitFieldSExtract
		} else {
			nativeOpcode = OpBitFieldUExtract
		}
		useNativeOpcode = true
	case ir.MathInsertBits:
		// OpBitFieldInsert: result-type result-id base insert offset count
		nativeOpcode = OpBitFieldInsert
		useNativeOpcode = true

	// Data packing functions (GLSL.std.450)
	case ir.MathPack4x8snorm:
		glslInst = GLSLstd450PackSnorm4x8
	case ir.MathPack4x8unorm:
		glslInst = GLSLstd450PackUnorm4x8
	case ir.MathPack2x16snorm:
		glslInst = GLSLstd450PackSnorm2x16
	case ir.MathPack2x16unorm:
		glslInst = GLSLstd450PackUnorm2x16
	case ir.MathPack2x16float:
		glslInst = GLSLstd450PackHalf2x16

	// Data unpacking functions (GLSL.std.450)
	case ir.MathUnpack4x8snorm:
		glslInst = GLSLstd450UnpackSnorm4x8
	case ir.MathUnpack4x8unorm:
		glslInst = GLSLstd450UnpackUnorm4x8
	case ir.MathUnpack2x16snorm:
		glslInst = GLSLstd450UnpackSnorm2x16
	case ir.MathUnpack2x16unorm:
		glslInst = GLSLstd450UnpackUnorm2x16
	case ir.MathUnpack2x16float:
		glslInst = GLSLstd450UnpackHalf2x16

	// Pack 4x8 integer functions (polyfill via BitFieldInsert)
	// Matches Rust naga block.rs:1554 (write_pack4x8_polyfill)
	case ir.MathPack4xI8:
		usePack4x8 = true
		pack4x8Signed = true
	case ir.MathPack4xU8:
		usePack4x8 = true
	case ir.MathPack4xI8Clamp:
		usePack4x8 = true
		pack4x8Signed = true
		pack4x8Clamp = true
	case ir.MathPack4xU8Clamp:
		usePack4x8 = true
		pack4x8Clamp = true

	// Unpack 4x8 integer functions (polyfill via BitFieldExtract)
	// Matches Rust naga block.rs:1586 (write_unpack4x8_polyfill)
	case ir.MathUnpack4xI8:
		useUnpack4x8 = true
		unpack4x8Signed = true
	case ir.MathUnpack4xU8:
		useUnpack4x8 = true

	// Packed dot product (SPV_KHR_integer_dot_product extension)
	case ir.MathDot4I8Packed:
		nativeOpcode = OpSDotKHR
		useNativeOpcode = true
	case ir.MathDot4U8Packed:
		nativeOpcode = OpUDotKHR
		useNativeOpcode = true

	default:
		return 0, fmt.Errorf("unsupported math function: %v", mathExpr.Fun)
	}

	// Functions with special result types (not same as argument type).
	switch mathExpr.Fun {
	case ir.MathModf:
		// ModfStruct returns a struct type from the module's type arena.
		if h := ir.FindModfResultType(e.backend.module, argType); h >= 0 {
			if id, err := e.backend.emitType(h); err == nil {
				resultType = id
			}
		} else {
			// Fallback: create inline struct type
			resultType = e.backend.emitModfStructType(argType)
		}
	case ir.MathFrexp:
		// FrexpStruct returns a struct type from the module's type arena.
		if h := ir.FindFrexpResultType(e.backend.module, argType); h >= 0 {
			if id, err := e.backend.emitType(h); err == nil {
				resultType = id
			}
		} else {
			// Fallback: create inline struct type
			resultType = e.backend.emitFrexpStructType(argType)
		}
	case ir.MathLength, ir.MathDistance, ir.MathDeterminant:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{Value: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
	case ir.MathDot:
		// Dot product result type matches the scalar kind of the input vector.
		argInner := ir.TypeResInner(e.backend.module, argType)
		if vecType, ok := argInner.(ir.VectorType); ok {
			resultType = e.backend.resolveTypeResolution(ir.TypeResolution{Value: vecType.Scalar})
		} else {
			resultType = e.backend.resolveTypeResolution(ir.TypeResolution{Value: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}})
		}
	case ir.MathTranspose:
		// transpose(matRxC) -> matCxR: swap columns and rows
		inner := ir.TypeResInner(e.backend.module, argType)
		if mat, ok := inner.(ir.MatrixType); ok {
			resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
				Value: ir.MatrixType{Columns: mat.Rows, Rows: mat.Columns, Scalar: mat.Scalar},
			})
		}

	// Packing functions: vec -> u32
	case ir.MathPack4x8snorm, ir.MathPack4x8unorm,
		ir.MathPack2x16snorm, ir.MathPack2x16unorm, ir.MathPack2x16float,
		ir.MathPack4xI8, ir.MathPack4xU8, ir.MathPack4xI8Clamp, ir.MathPack4xU8Clamp:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
		})

	// Unpacking 4x8 functions: u32 -> vec4<f32>
	case ir.MathUnpack4x8snorm, ir.MathUnpack4x8unorm:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 4},
		})

	// Unpack 4xI8: u32 -> vec4<i32>
	case ir.MathUnpack4xI8:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, Size: 4},
		})
	// Unpack 4xU8: u32 -> vec4<u32>
	case ir.MathUnpack4xU8:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, Size: 4},
		})

	// Unpacking 2x16 functions: u32 -> vec2<f32>
	case ir.MathUnpack2x16snorm, ir.MathUnpack2x16unorm, ir.MathUnpack2x16float:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: 2},
		})

	// Packed dot product: u32, u32 -> i32 (signed) or u32 (unsigned)
	case ir.MathDot4I8Packed:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
		})
	case ir.MathDot4U8Packed:
		resultType = e.backend.resolveTypeResolution(ir.TypeResolution{
			Value: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
		})
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

	// Pack 4x8 integer polyfill: extract components, bitcast if signed, clamp if needed,
	// then BitFieldInsert to build u32. Matches Rust naga write_pack4x8_polyfill (block.rs:2725).
	if usePack4x8 {
		return e.emitPack4x8Polyfill(resultType, operands[0], pack4x8Signed, pack4x8Clamp)
	}

	// Unpack 4x8 integer polyfill: bitcast if signed, then BitFieldExtract for each byte,
	// then CompositeConstruct. Matches Rust naga write_unpack4x8_polyfill (block.rs:2869).
	if useUnpack4x8 {
		return e.emitUnpack4x8Polyfill(resultType, operands[0], unpack4x8Signed)
	}

	// Integer dot product: manual expansion (extract + multiply + accumulate).
	// Matches Rust naga's write_dot_product (block.rs:2596).
	if useIntegerDot {
		arg0ID := operands[0]
		arg1ID := operands[1]
		scalarTypeID := e.backend.emitScalarType(intDotScalar)
		// Start with zero
		partialSum := e.backend.builder.AddConstantNull(scalarTypeID)
		mulOp := OpIMul
		addOp := OpIAdd
		for i := 0; i < intDotSize; i++ {
			// Extract components
			aID := e.backend.builder.AllocID()
			ib := e.newIB()
			ib.AddWord(scalarTypeID)
			ib.AddWord(aID)
			ib.AddWord(arg0ID)
			ib.AddWord(uint32(i))
			e.backend.builder.funcAppend(ib.Build(OpCompositeExtract))

			bID := e.backend.builder.AllocID()
			ib2 := e.newIB()
			ib2.AddWord(scalarTypeID)
			ib2.AddWord(bID)
			ib2.AddWord(arg1ID)
			ib2.AddWord(uint32(i))
			e.backend.builder.funcAppend(ib2.Build(OpCompositeExtract))

			// Multiply
			prodID := e.backend.builder.AddBinaryOp(mulOp, scalarTypeID, aID, bID)

			// Accumulate
			sumID := e.backend.builder.AddBinaryOp(addOp, scalarTypeID, partialSum, prodID)
			partialSum = sumID
		}
		return partialSum, nil
	}

	// Emit instruction
	if useNativeOpcode {
		// Special case: packed dot product needs extension and capability
		if mathExpr.Fun == ir.MathDot4I8Packed || mathExpr.Fun == ir.MathDot4U8Packed {
			if e.backend.requireAllCapabilities(CapabilityDotProduct, CapabilityDotProductInput4x8BitPacked) {
				// Optimized path: use native packed dot product opcodes
				if e.backend.langVersion() < 0x00010600 {
					e.backend.addExtension("SPV_KHR_integer_dot_product")
				}
				resultID := e.backend.builder.AllocID()
				ib := e.newIB()
				ib.AddWord(resultType)
				ib.AddWord(resultID)
				for _, op := range operands {
					ib.AddWord(op)
				}
				ib.AddWord(PackedVectorFormat4x8Bit)
				e.backend.builder.funcAppend(ib.Build(nativeOpcode))
				return resultID, nil
			}
			// Polyfill: extract 4 bytes, multiply, accumulate
			return e.emitDot4PackedPolyfill(mathExpr.Fun, resultType, operands[0], operands[1])
		}

		if len(operands) == 1 {
			return e.backend.builder.AddUnaryOp(nativeOpcode, resultType, operands[0]), nil
		}
		if len(operands) == 2 {
			return e.backend.builder.AddBinaryOp(nativeOpcode, resultType, operands[0], operands[1]), nil
		}
		// 3+ operands (ExtractBits, InsertBits): build manually
		resultID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(resultType)
		ib.AddWord(resultID)
		for _, op := range operands {
			ib.AddWord(op)
		}
		e.backend.builder.funcAppend(ib.Build(nativeOpcode))
		return resultID, nil
	}

	// FMix: if selector (arg2) is scalar but result is vector, splat the selector.
	// SPIR-V FMix requires all operands to match Result Type.
	if needsMixSplat && len(operands) >= 3 && mathExpr.Arg2 != nil {
		selectorType, _ := ir.ResolveExpressionType(e.backend.module, e.function, *mathExpr.Arg2)
		selectorInner := ir.TypeResInner(e.backend.module, selectorType)
		argInner2 := ir.TypeResInner(e.backend.module, argType)
		if _, isScalar := selectorInner.(ir.ScalarType); isScalar {
			if vecType, isVec := argInner2.(ir.VectorType); isVec {
				// Splat scalar to vector: OpCompositeConstruct with N copies
				vecTypeID := e.backend.resolveTypeResolution(ir.TypeResolution{Value: vecType})
				splatID := e.backend.builder.AllocID()
				ib := e.newIB()
				ib.AddWord(vecTypeID)
				ib.AddWord(splatID)
				for j := 0; j < int(vecType.Size); j++ {
					ib.AddWord(operands[2])
				}
				e.backend.builder.funcAppend(ib.Build(OpCompositeConstruct))
				operands[2] = splatID
			}
		}
	}

	// Use GLSL.std.450 extended instruction
	return e.backend.builder.AddExtInst(resultType, e.backend.glslExtID, glslInst, operands...), nil
}

// emitMatrixColumnOp decomposes a matrix binary operation into column-wise vector ops.
// SPIR-V FAdd/FSub don't work on matrix types directly.
// Matches Rust naga's write_matrix_matrix_column_op (block.rs:2493).
func (e *ExpressionEmitter) emitMatrixColumnOp(op OpCode, resultTypeID, leftID, rightID uint32, mat ir.MatrixType) uint32 {
	// Get column vector type
	colVecType := ir.VectorType{Scalar: mat.Scalar, Size: mat.Rows}
	colVecTypeID := e.backend.resolveTypeResolution(ir.TypeResolution{Value: colVecType})

	// Process each column
	columnIDs := make([]uint32, int(mat.Columns))
	for i := 0; i < int(mat.Columns); i++ {
		// Extract column from left matrix
		leftColID := e.backend.builder.AllocID()
		ib1 := e.newIB()
		ib1.AddWord(colVecTypeID)
		ib1.AddWord(leftColID)
		ib1.AddWord(leftID)
		ib1.AddWord(uint32(i))
		e.backend.builder.funcAppend(ib1.Build(OpCompositeExtract))

		// Extract column from right matrix
		rightColID := e.backend.builder.AllocID()
		ib2 := e.newIB()
		ib2.AddWord(colVecTypeID)
		ib2.AddWord(rightColID)
		ib2.AddWord(rightID)
		ib2.AddWord(uint32(i))
		e.backend.builder.funcAppend(ib2.Build(OpCompositeExtract))

		// Apply op to columns
		columnIDs[i] = e.backend.builder.AddBinaryOp(op, colVecTypeID, leftColID, rightColID)
	}

	// Reassemble matrix from columns
	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultTypeID)
	ib.AddWord(resultID)
	for _, colID := range columnIDs {
		ib.AddWord(colID)
	}
	e.backend.builder.funcAppend(ib.Build(OpCompositeConstruct))
	return resultID
}

// splatScalarToVector creates an OpCompositeConstruct that replicates a scalar ID
// to fill a vector type. Used when SPIR-V requires matching vector operands but
// WGSL allows mixed scalar/vector (e.g., integer vec*scalar multiply).
func (e *ExpressionEmitter) splatScalarToVector(scalarID uint32, vecType ir.VectorType) uint32 {
	vecTypeID := e.backend.resolveTypeResolution(ir.TypeResolution{Value: vecType})
	splatID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(vecTypeID)
	ib.AddWord(splatID)
	for i := 0; i < int(vecType.Size); i++ {
		ib.AddWord(scalarID)
	}
	e.backend.builder.funcAppend(ib.Build(OpCompositeConstruct))
	return splatID
}

// emitPack4x8Polyfill emits a polyfill for pack4xI8/U8/I8Clamp/U8Clamp.
// Extracts each component, optionally clamps, bitcasts if signed, then uses
// OpBitFieldInsert to build a u32. Matches Rust naga write_pack4x8_polyfill (block.rs:2725).
func (e *ExpressionEmitter) emitPack4x8Polyfill(resultTypeID, arg0ID uint32, isSigned, shouldClamp bool) (uint32, error) {
	uint32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	var intTypeID uint32
	if isSigned {
		intTypeID = e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	} else {
		intTypeID = uint32TypeID
	}

	eight := e.backend.builder.AddConstant(uint32TypeID, 8)
	zero := e.backend.builder.AddConstant(uint32TypeID, 0)
	preresult := zero

	for i := uint32(0); i < 4; i++ {
		offset := e.backend.builder.AddConstant(uint32TypeID, i*8)

		// Extract component
		extracted := e.backend.builder.AddCompositeExtract(intTypeID, arg0ID, i)

		// Bitcast to u32 if signed
		if isSigned {
			extracted = e.backend.builder.AddUnaryOp(OpBitcast, uint32TypeID, extracted)
		}

		// Clamp if needed
		if shouldClamp {
			var clampOp uint32
			var minID, maxID uint32
			if isSigned {
				// SClamp to [-128, 127] — clamp operates on the original int type
				// But Rust clamps on result_type_id (u32) with SClamp... actually Rust
				// clamps on int_type_id BEFORE bitcast. Let me re-check.
				// Rust: clamps extracted (before bitcast for signed), with result_type_id.
				// Wait — for the clamp variant, Rust does clamp BEFORE bitcast.
				// Let me re-read: in Rust polyfill, extraction is done first, then
				// if is_signed, bitcast. But clamp happens AFTER extraction AND bitcast.
				// Actually no, let me re-read Rust carefully:
				//   1. CompositeExtract -> extracted (int_type_id)
				//   2. if is_signed: Bitcast -> casted (uint_type_id); extracted = casted
				//   3. if should_clamp: clamp extracted
				// So clamp operates on the bitcasted uint if signed. And uses result_type_id.
				// Clamp SClamp on i32: min=-128, max=127
				clampOp = GLSLstd450SClamp
				sint32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
				minID = e.backend.builder.AddConstant(sint32TypeID, 0xFFFFFF80) // -128 as u32 bits
				maxID = e.backend.builder.AddConstant(sint32TypeID, uint32(int32(127)))
			} else {
				// UClamp to [0, 255]
				clampOp = GLSLstd450UClamp
				minID = e.backend.builder.AddConstant(uint32TypeID, 0)
				maxID = e.backend.builder.AddConstant(uint32TypeID, 255)
			}
			clampID := e.backend.builder.AddExtInst(resultTypeID, e.backend.glslExtID, clampOp, extracted, minID, maxID)
			extracted = clampID
		}

		// BitFieldInsert: insert 8 bits at offset
		if i == 3 {
			// Last iteration: this is the final result with the target ID
			resultID := e.backend.builder.AllocID()
			ib := e.newIB()
			ib.AddWord(resultTypeID)
			ib.AddWord(resultID)
			ib.AddWord(preresult)
			ib.AddWord(extracted)
			ib.AddWord(offset)
			ib.AddWord(eight)
			e.backend.builder.funcAppend(ib.Build(OpBitFieldInsert))
			return resultID, nil
		}
		// Intermediate results
		newPreresult := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(resultTypeID)
		ib.AddWord(newPreresult)
		ib.AddWord(preresult)
		ib.AddWord(extracted)
		ib.AddWord(offset)
		ib.AddWord(eight)
		e.backend.builder.funcAppend(ib.Build(OpBitFieldInsert))
		preresult = newPreresult
	}
	// Should not reach here
	return 0, fmt.Errorf("pack4x8 polyfill: unexpected end of loop")
}

// emitUnpack4x8Polyfill emits a polyfill for unpack4xI8/U8.
// Uses BitFieldSExtract (signed) or BitFieldUExtract (unsigned) for each byte,
// then CompositeConstruct. Matches Rust naga write_unpack4x8_polyfill (block.rs:2869).
func (e *ExpressionEmitter) emitUnpack4x8Polyfill(resultTypeID, arg0ID uint32, isSigned bool) (uint32, error) {
	var extractOp OpCode
	var intKind ir.ScalarKind
	if isSigned {
		extractOp = OpBitFieldSExtract
		intKind = ir.ScalarSint
	} else {
		extractOp = OpBitFieldUExtract
		intKind = ir.ScalarUint
	}

	uint32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	sint32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	intTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: intKind, Width: 4})
	eight := e.backend.builder.AddConstant(uint32TypeID, 8)

	// If signed, bitcast input u32 to i32 first (Rust: block.rs:2893-2901)
	argID := arg0ID
	if isSigned {
		argID = e.backend.builder.AddUnaryOp(OpBitcast, sint32TypeID, arg0ID)
	}

	// Extract 4 bytes
	var parts [4]uint32
	for i := uint32(0); i < 4; i++ {
		index := e.backend.builder.AddConstant(uint32TypeID, i*8)
		partID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(intTypeID)
		ib.AddWord(partID)
		ib.AddWord(argID)
		ib.AddWord(index)
		ib.AddWord(eight)
		e.backend.builder.funcAppend(ib.Build(extractOp))
		parts[i] = partID
	}

	// CompositeConstruct vec4
	return e.backend.builder.AddCompositeConstruct(resultTypeID, parts[0], parts[1], parts[2], parts[3]), nil
}

// emitDot4PackedPolyfill emits a software polyfill for dot4I8Packed/dot4U8Packed
// when DotProduct/DotProductInput4x8BitPacked capabilities are not available.
// Algorithm: extract 4 bytes from each packed arg, multiply pairwise, accumulate.
// Matches Rust naga's write_dot_product fallback in block.rs.
func (e *ExpressionEmitter) emitDot4PackedPolyfill(fun ir.MathFunction, resultTypeID, arg0ID, arg1ID uint32) (uint32, error) {
	isSigned := fun == ir.MathDot4I8Packed

	// For signed: bitcast args from u32 to i32 first
	if isSigned {
		newArg0 := e.backend.builder.AddUnaryOp(OpBitcast, resultTypeID, arg0ID)
		newArg1 := e.backend.builder.AddUnaryOp(OpBitcast, resultTypeID, arg1ID)
		arg0ID = newArg0
		arg1ID = newArg1
	}

	// Choose extract op
	var extractOp OpCode
	if isSigned {
		extractOp = OpBitFieldSExtract
	} else {
		extractOp = OpBitFieldUExtract
	}

	// Get u32 type for shift/count constants
	u32Type := e.backend.resolveTypeResolution(ir.TypeResolution{
		Value: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
	})

	// Constant: bit width of each byte = 8
	eightID := e.backend.builder.AddConstant(u32Type, 8)

	// Bit shift constants: 0, 8, 16, 24
	shiftIDs := [4]uint32{
		e.backend.builder.AddConstant(u32Type, 0),
		e.backend.builder.AddConstant(u32Type, 8),
		e.backend.builder.AddConstant(u32Type, 16),
		e.backend.builder.AddConstant(u32Type, 24),
	}

	// Start with zero
	partialSum := e.backend.builder.AddConstantNull(resultTypeID)

	for i := uint32(0); i < 4; i++ {
		// Extract byte i from arg0: BitFieldExtract(arg0, shift[i], 8)
		aID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(resultTypeID)
		ib.AddWord(aID)
		ib.AddWord(arg0ID)
		ib.AddWord(shiftIDs[i])
		ib.AddWord(eightID)
		e.backend.builder.funcAppend(ib.Build(extractOp))

		// Extract byte i from arg1
		bID := e.backend.builder.AllocID()
		ib2 := e.newIB()
		ib2.AddWord(resultTypeID)
		ib2.AddWord(bID)
		ib2.AddWord(arg1ID)
		ib2.AddWord(shiftIDs[i])
		ib2.AddWord(eightID)
		e.backend.builder.funcAppend(ib2.Build(extractOp))

		// Multiply: IMul
		prodID := e.backend.builder.AddBinaryOp(OpIMul, resultTypeID, aID, bID)

		// Accumulate: IAdd (last iteration uses result ID directly in Rust,
		// but for simplicity we always allocate a new ID)
		sumID := e.backend.builder.AddBinaryOp(OpIAdd, resultTypeID, partialSum, prodID)
		partialSum = sumID
	}

	return partialSum, nil
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

	// Fine/Coarse derivatives require DerivativeControl capability
	if deriv.Control == ir.DerivativeFine || deriv.Control == ir.DerivativeCoarse {
		e.backend.addCapability(CapabilityDerivativeControl)
	}

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

// OpTranspose represents OpTranspose opcode (matrix transpose).
const OpTranspose OpCode = 84

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
	OpImageTexelPointer           OpCode = 60
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

	// Determine if this is a depth image — affects result type.
	// SPIR-V Dref sampling instructions return scalar f32, not vec4.
	// For non-Dref depth sampling (without gather), SPIR-V returns vec4
	// but we need scalar, so we CompositeExtract the first component.
	isDepthImage := false
	exprType, resolveErr := ir.ResolveExpressionType(e.backend.module, e.function, sample.Image)
	if resolveErr == nil {
		inner := typeResolutionInner(e.backend.module, exprType)
		if imgType, ok := inner.(ir.ImageType); ok {
			isDepthImage = imgType.Class == ir.ImageClassDepth
		}
	}

	// Determine the proper result type for the sample operation.
	// Rust naga (image.rs line 830): if needs_sub_access, use vec4<f32>;
	// otherwise, use the expression's result type (result_type_id).
	// - Dref sampling (non-Gather) → scalar f32
	// - Depth image without Dref/Gather → vec4<f32> (then extract component 0)
	// - Everything else → use the actual result type (vec4<f32>, vec4<u32>, vec4<i32>, etc.)
	needsSubAccess := isDepthImage && sample.DepthRef == nil && sample.Gather == nil
	var sampleResultType uint32
	if sample.DepthRef != nil && sample.Gather == nil {
		// Dref sampling (non-Gather) → scalar f32 result
		sampleResultType = e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	} else if needsSubAccess {
		// Depth image without Dref or Gather → sample as vec4<f32>, extract later
		sampleResultType = e.backend.emitVec4F32Type()
	} else {
		// Use the image's sampled type to build the correct vec4 result type.
		// For float images → vec4<f32>, for uint → vec4<u32>, for int → vec4<i32>.
		sampleResultType = e.backend.emitVec4F32Type() // default
		if resolveErr == nil {
			inner := typeResolutionInner(e.backend.module, exprType)
			if imgType, ok := inner.(ir.ImageType); ok {
				if imgType.Class == ir.ImageClassSampled {
					texelScalar := ir.ScalarType{Kind: imgType.SampledKind, Width: 4}
					scalarID := e.backend.emitScalarType(texelScalar)
					sampleResultType = e.backend.emitVectorType(scalarID, 4)
				}
				// For depth/external/storage, vec4<f32> is correct
			}
		}
	}
	resultType := sampleResultType
	resultID := e.backend.builder.AllocID()

	// Create SampledImage by combining image and sampler
	sampledImageTypeID := e.backend.getSampledImageType(e.function, sample.Image)
	sampledImageID := e.backend.builder.AllocID()

	sampledImageBuilder := e.newIB()
	sampledImageBuilder.AddWord(sampledImageTypeID)
	sampledImageBuilder.AddWord(sampledImageID)
	sampledImageBuilder.AddWord(imageID)
	sampledImageBuilder.AddWord(samplerID)
	e.backend.builder.funcAppend(sampledImageBuilder.Build(OpSampledImage))

	// Handle gather operations (textureGather / textureGatherCompare)
	if sample.Gather != nil {
		builder := e.newIB()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(sampledImageID)
		builder.AddWord(coordID)

		var opcode OpCode
		if sample.DepthRef != nil {
			// OpImageDrefGather: result-type result-id sampled-image coordinate dref [image-operands]
			drefID, drefErr := e.emitExpression(*sample.DepthRef)
			if drefErr != nil {
				return 0, drefErr
			}
			builder.AddWord(drefID)
			opcode = OpImageDrefGather
		} else {
			// OpImageGather: result-type result-id sampled-image coordinate component [image-operands]
			componentID := e.backend.builder.AddConstant(
				e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4}),
				uint32(*sample.Gather),
			)
			builder.AddWord(componentID)
			opcode = OpImageGather
		}

		// Append optional image operands (Offset)
		if sample.Offset != nil {
			offsetID, offsetErr := e.emitConstExpression(*sample.Offset)
			if offsetErr != nil {
				return 0, offsetErr
			}
			builder.AddWord(0x08) // ImageOperands::ConstOffset (bit 3)
			builder.AddWord(offsetID)
		}

		e.backend.builder.funcAppend(builder.Build(opcode))
		return resultID, nil
	}

	// Standard sampling path
	builder := e.newIB()
	builder.AddWord(resultType)
	builder.AddWord(resultID)
	builder.AddWord(sampledImageID)
	builder.AddWord(coordID)

	// Collect image operand mask and values for Offset (can combine with level operands)
	var imageOperandMask uint32
	var imageOperandValues []uint32
	if sample.Offset != nil {
		offsetID, offsetErr := e.emitConstExpression(*sample.Offset)
		if offsetErr != nil {
			return 0, offsetErr
		}
		imageOperandMask |= 0x08 // ImageOperands::ConstOffset (bit 3)
		imageOperandValues = append(imageOperandValues, offsetID)
	}

	// Choose opcode based on sample level
	switch level := sample.Level.(type) {
	case ir.SampleLevelAuto:
		if sample.DepthRef != nil {
			drefID, drefErr := e.emitExpression(*sample.DepthRef)
			if drefErr != nil {
				return 0, drefErr
			}
			builder.AddWord(drefID)
			if imageOperandMask != 0 {
				builder.AddWord(imageOperandMask)
				for _, v := range imageOperandValues {
					builder.AddWord(v)
				}
			}
			e.backend.builder.funcAppend(builder.Build(OpImageSampleDrefImplicitLod))
		} else {
			if imageOperandMask != 0 {
				builder.AddWord(imageOperandMask)
				for _, v := range imageOperandValues {
					builder.AddWord(v)
				}
			}
			e.backend.builder.funcAppend(builder.Build(OpImageSampleImplicitLod))
		}

	case ir.SampleLevelExact:
		// OpImageSampleExplicitLod with Lod operand
		levelID, levelErr := e.emitExpression(level.Level)
		if levelErr != nil {
			return 0, levelErr
		}
		// SPIR-V requires the Lod operand to be float for ExplicitLod.
		// For depth images, the WGSL Lod is integer (i32/u32), so we must convert.
		// Matches Rust naga image.rs line 1010-1044.
		if isDepthImage {
			lodType, lodErr := ir.ResolveExpressionType(e.backend.module, e.function, level.Level)
			if lodErr == nil {
				lodInner := ir.TypeResInner(e.backend.module, lodType)
				if sc, ok := lodInner.(ir.ScalarType); ok && (sc.Kind == ir.ScalarSint || sc.Kind == ir.ScalarUint) {
					f32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
					convertedID := e.backend.builder.AllocID()
					convertOp := OpConvertSToF
					if sc.Kind == ir.ScalarUint {
						convertOp = OpConvertUToF
					}
					ib := e.newIB()
					ib.AddWord(f32TypeID)
					ib.AddWord(convertedID)
					ib.AddWord(levelID)
					e.backend.builder.funcAppend(ib.Build(convertOp))
					levelID = convertedID
				}
			}
		}
		imageOperandMask |= 0x02 // ImageOperands::Lod
		builder.AddWord(imageOperandMask)
		builder.AddWord(levelID)
		for _, v := range imageOperandValues {
			builder.AddWord(v)
		}
		e.backend.builder.funcAppend(builder.Build(OpImageSampleExplicitLod))

	case ir.SampleLevelBias:
		// OpImageSampleImplicitLod with Bias operand
		biasID, biasErr := e.emitExpression(level.Bias)
		if biasErr != nil {
			return 0, biasErr
		}
		imageOperandMask |= 0x01 // ImageOperands::Bias
		builder.AddWord(imageOperandMask)
		builder.AddWord(biasID)
		for _, v := range imageOperandValues {
			builder.AddWord(v)
		}
		e.backend.builder.funcAppend(builder.Build(OpImageSampleImplicitLod))

	case ir.SampleLevelGradient:
		// OpImageSampleExplicitLod with Grad operand
		gradXID, gradXErr := e.emitExpression(level.X)
		if gradXErr != nil {
			return 0, gradXErr
		}
		gradYID, gradYErr := e.emitExpression(level.Y)
		if gradYErr != nil {
			return 0, gradYErr
		}
		imageOperandMask |= 0x04 // ImageOperands::Grad
		builder.AddWord(imageOperandMask)
		builder.AddWord(gradXID)
		builder.AddWord(gradYID)
		for _, v := range imageOperandValues {
			builder.AddWord(v)
		}
		e.backend.builder.funcAppend(builder.Build(OpImageSampleExplicitLod))

	case ir.SampleLevelZero:
		// OpImageSampleExplicitLod with Lod = 0
		zeroID := e.backend.builder.AddConstantFloat32(
			e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}),
			0.0)
		imageOperandMask |= 0x02 // ImageOperands::Lod
		if sample.DepthRef != nil {
			drefID, drefErr := e.emitExpression(*sample.DepthRef)
			if drefErr != nil {
				return 0, drefErr
			}
			builder.AddWord(drefID)
			builder.AddWord(imageOperandMask)
			builder.AddWord(zeroID)
			for _, v := range imageOperandValues {
				builder.AddWord(v)
			}
			e.backend.builder.funcAppend(builder.Build(OpImageSampleDrefExplicitLod))
		} else {
			builder.AddWord(imageOperandMask)
			builder.AddWord(zeroID)
			for _, v := range imageOperandValues {
				builder.AddWord(v)
			}
			e.backend.builder.funcAppend(builder.Build(OpImageSampleExplicitLod))
		}

	default:
		return 0, fmt.Errorf("unsupported sample level: %T", level)
	}

	// For depth images without Dref (non-comparison sampling), SPIR-V returns
	// vec4<f32> but naga IR expects scalar f32. Extract the first component.
	if needsSubAccess {
		scalarF32 := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
		extractedID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(scalarF32)
		ib.AddWord(extractedID)
		ib.AddWord(resultID)
		ib.AddWord(0) // index 0 = first component
		e.backend.builder.funcAppend(ib.Build(OpCompositeExtract))
		return extractedID, nil
	}

	return resultID, nil
}

// imageCoordinates holds information about a combined coordinate/array-index vector.
type imageCoordinates struct {
	valueID uint32
	typeID  uint32
	size    int // 0 for scalar, 2/3/4 for vector
}

// emitImageCoordinates builds a SPIR-V coordinate vector, combining coordinates
// and array index (if any). For image load/store, coordinates are integers.
// Matching Rust naga's write_image_coordinates.
func (e *ExpressionEmitter) emitImageCoordinates(
	coordExpr ir.ExpressionHandle,
	arrayIndex *ir.ExpressionHandle,
) (imageCoordinates, error) {
	coordID, err := e.emitExpression(coordExpr)
	if err != nil {
		return imageCoordinates{}, err
	}

	coordType, err := ir.ResolveExpressionType(e.backend.module, e.function, coordExpr)
	if err != nil {
		return imageCoordinates{}, err
	}
	coordInner := typeResolutionInner(e.backend.module, coordType)

	if arrayIndex == nil {
		typeID := e.backend.resolveTypeResolution(coordType)
		size := 0
		if vec, ok := coordInner.(ir.VectorType); ok {
			size = int(vec.Size)
		}
		return imageCoordinates{valueID: coordID, typeID: typeID, size: size}, nil
	}

	arrayIndexID, err := e.emitExpression(*arrayIndex)
	if err != nil {
		return imageCoordinates{}, err
	}

	// Determine component scalar and new size
	var componentScalar ir.ScalarType
	var newSize int
	switch inner := coordInner.(type) {
	case ir.ScalarType:
		componentScalar = inner
		newSize = 2
	case ir.VectorType:
		componentScalar = inner.Scalar
		newSize = int(inner.Size) + 1
	default:
		return imageCoordinates{}, fmt.Errorf("unexpected coordinate type: %T", coordInner)
	}

	// Resolve array index type and bitcast if needed
	arrayIndexType, _ := ir.ResolveExpressionType(e.backend.module, e.function, *arrayIndex)
	arrayIndexInner := typeResolutionInner(e.backend.module, arrayIndexType)
	if scalar, ok := arrayIndexInner.(ir.ScalarType); ok {
		if scalar.Kind != componentScalar.Kind {
			// Need bitcast (e.g. u32 to i32 or vice versa)
			targetTypeID := e.backend.emitScalarType(componentScalar)
			castID := e.backend.builder.AllocID()
			ib := e.newIB()
			ib.AddWord(targetTypeID)
			ib.AddWord(castID)
			ib.AddWord(arrayIndexID)
			e.backend.builder.funcAppend(ib.Build(OpBitcast))
			arrayIndexID = castID
		}
	}

	// Build combined vector type
	scalarTypeID := e.backend.emitScalarType(componentScalar)
	combinedTypeID := e.backend.emitVectorType(scalarTypeID, uint32(newSize))

	// OpCompositeConstruct
	combinedID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(combinedTypeID)
	ib.AddWord(combinedID)
	ib.AddWord(coordID)
	ib.AddWord(arrayIndexID)
	e.backend.builder.funcAppend(ib.Build(OpCompositeConstruct))

	return imageCoordinates{valueID: combinedID, typeID: combinedTypeID, size: newSize}, nil
}

// emitImageFetchOrRead emits the actual image access instruction.
// Returns the SPIR-V result ID of the texel.
func (e *ExpressionEmitter) emitImageFetchOrRead(
	opcode OpCode, instrTypeID, imageID, coordID uint32,
	levelID *uint32, sampleID *uint32,
) uint32 {
	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(instrTypeID)
	ib.AddWord(resultID)
	ib.AddWord(imageID)
	ib.AddWord(coordID)

	switch {
	case levelID != nil && sampleID == nil:
		ib.AddWord(0x02) // ImageOperands::Lod
		ib.AddWord(*levelID)
	case levelID == nil && sampleID != nil:
		ib.AddWord(0x40) // ImageOperands::Sample
		ib.AddWord(*sampleID)
	}

	e.backend.builder.funcAppend(ib.Build(opcode))
	return resultID
}

// emitImageLoad emits a texture load operation.
// Implements bounds checking matching Rust naga's write_image_load.
func (e *ExpressionEmitter) emitImageLoad(load ir.ExprImageLoad) (uint32, error) {
	imageID, err := e.emitExpression(load.Image)
	if err != nil {
		return 0, err
	}

	// Determine image class from type
	imageType, err := ir.ResolveExpressionType(e.backend.module, e.function, load.Image)
	if err != nil {
		return 0, err
	}
	imageInner := typeResolutionInner(e.backend.module, imageType)
	imgType, ok := imageInner.(ir.ImageType)
	if !ok {
		return 0, fmt.Errorf("emitImageLoad: expected image type, got %T", imageInner)
	}

	// Choose opcode: storage images use OpImageRead, sampled/depth use OpImageFetch
	opcode := OpImageFetch
	if imgType.Class == ir.ImageClassStorage {
		opcode = OpImageRead
	}

	// Determine result type from image's sampled kind.
	// The OpImageFetch/OpImageRead result type must be vec4 of the image's sampled type
	// (e.g., vec4<u32> for texture_2d<u32>). Matches Rust naga image.rs Load::from_image_expr.
	var instrTypeID, resultTypeID uint32
	isDepth := imgType.Class == ir.ImageClassDepth
	if isDepth {
		// Depth images always produce vec4<f32> from the instruction,
		// but the expression result is scalar f32 (extracted via CompositeExtract).
		vec4f32TypeID := e.backend.emitVec4F32Type()
		instrTypeID = vec4f32TypeID
		resultTypeID = e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	} else {
		// Non-depth: result type matches the image's sampled kind.
		var sampledScalar ir.ScalarType
		switch imgType.Class {
		case ir.ImageClassStorage:
			sampledScalar = storageFormatToScalar(imgType.StorageFormat)
		case ir.ImageClassExternal:
			// External textures always use float sampled type
			sampledScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		default:
			sampledScalar = ir.ScalarType{Kind: imgType.SampledKind, Width: 4}
		}
		scalarID := e.backend.emitScalarType(sampledScalar)
		vec4TypeID := e.backend.emitVectorType(scalarID, 4)
		instrTypeID = vec4TypeID
		resultTypeID = vec4TypeID
	}

	// Build combined coordinates with array index
	coords, err := e.emitImageCoordinates(load.Coordinate, load.ArrayIndex)
	if err != nil {
		return 0, err
	}

	// Get level and sample IDs
	var levelID, sampleID *uint32
	if load.Level != nil {
		lid, err := e.emitExpression(*load.Level)
		if err != nil {
			return 0, err
		}
		levelID = &lid
	}
	if load.Sample != nil {
		sid, err := e.emitExpression(*load.Sample)
		if err != nil {
			return 0, err
		}
		sampleID = &sid
	}

	// Apply bounds check policy
	var accessID uint32
	policy := e.backend.options.BoundsCheckPolicies.ImageLoad

	switch policy {
	case BoundsCheckRestrict:
		accessID, err = e.emitImageLoadRestrict(imageID, opcode, instrTypeID, coords, levelID, sampleID)
		if err != nil {
			return 0, err
		}

	case BoundsCheckReadZeroSkipWrite:
		accessID, err = e.emitImageLoadRZSW(imageID, opcode, instrTypeID, coords, levelID, sampleID)
		if err != nil {
			return 0, err
		}

	default: // Unchecked
		accessID = e.emitImageFetchOrRead(opcode, instrTypeID, imageID, coords.valueID, levelID, sampleID)
	}

	// For depth images, extract the first component (scalar f32 from vec4<f32>)
	if isDepth {
		componentID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(resultTypeID)
		ib.AddWord(componentID)
		ib.AddWord(accessID)
		ib.AddWord(0) // index 0
		e.backend.builder.funcAppend(ib.Build(OpCompositeExtract))
		return componentID, nil
	}

	_ = resultTypeID
	return accessID, nil
}

// emitImageLoadRestrict implements Restrict bounds checking for image loads.
// Clamps level/sample to valid range, queries image size, clamps coordinates.
// Matching Rust naga's write_restricted_coordinates.
func (e *ExpressionEmitter) emitImageLoadRestrict(
	imageID uint32, opcode OpCode, instrTypeID uint32,
	coords imageCoordinates, levelID, sampleID *uint32,
) (uint32, error) {
	e.backend.addCapability(CapabilityImageQuery)

	i32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	oneID := e.backend.builder.AddConstant(i32TypeID, 1)

	// Clamp level if present
	if levelID != nil {
		// OpImageQueryLevels
		numLevelsID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(i32TypeID)
		ib.AddWord(numLevelsID)
		ib.AddWord(imageID)
		e.backend.builder.funcAppend(ib.Build(OpImageQueryLevels))

		// max_level = numLevels - 1
		maxLevelID := e.backend.builder.AllocID()
		ib = e.newIB()
		ib.AddWord(i32TypeID)
		ib.AddWord(maxLevelID)
		ib.AddWord(numLevelsID)
		ib.AddWord(oneID)
		e.backend.builder.funcAppend(ib.Build(OpISub))

		// clamped = UMin(level, maxLevel)
		clampedLevelID := e.backend.builder.AddExtInst(i32TypeID, e.backend.glslExtID, GLSLstd450UMin, *levelID, maxLevelID)
		levelID = &clampedLevelID
	}

	// Clamp sample if present
	if sampleID != nil {
		// OpImageQuerySamples
		numSamplesID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(i32TypeID)
		ib.AddWord(numSamplesID)
		ib.AddWord(imageID)
		e.backend.builder.funcAppend(ib.Build(OpImageQuerySamples))

		// max_sample = numSamples - 1
		maxSampleID := e.backend.builder.AllocID()
		ib = e.newIB()
		ib.AddWord(i32TypeID)
		ib.AddWord(maxSampleID)
		ib.AddWord(numSamplesID)
		ib.AddWord(oneID)
		e.backend.builder.funcAppend(ib.Build(OpISub))

		// clamped = UMin(sample, maxSample)
		clampedSampleID := e.backend.builder.AddExtInst(i32TypeID, e.backend.glslExtID, GLSLstd450UMin, *sampleID, maxSampleID)
		sampleID = &clampedSampleID
	}

	// Query image size (using clamped level)
	coordBoundsID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(coords.typeID)
	ib.AddWord(coordBoundsID)
	ib.AddWord(imageID)
	if levelID != nil {
		ib.AddWord(*levelID)
		e.backend.builder.funcAppend(ib.Build(OpImageQuerySizeLod))
	} else {
		e.backend.builder.funcAppend(ib.Build(OpImageQuerySize))
	}

	// Build "ones" for coordinate bounds: scalar 1 or vec of 1s
	var onesID uint32
	if coords.size == 0 {
		onesID = oneID
	} else {
		ones := make([]uint32, coords.size)
		for i := range ones {
			ones[i] = oneID
		}
		onesID = e.backend.builder.AddConstantComposite(coords.typeID, ones...)
	}

	// coordLimit = size - ones
	coordLimitID := e.backend.builder.AllocID()
	ib = e.newIB()
	ib.AddWord(coords.typeID)
	ib.AddWord(coordLimitID)
	ib.AddWord(coordBoundsID)
	ib.AddWord(onesID)
	e.backend.builder.funcAppend(ib.Build(OpISub))

	// restricted = UMin(coords, coordLimit)
	restrictedCoordsID := e.backend.builder.AddExtInst(coords.typeID, e.backend.glslExtID, GLSLstd450UMin, coords.valueID, coordLimitID)

	return e.emitImageFetchOrRead(opcode, instrTypeID, imageID, restrictedCoordsID, levelID, sampleID), nil
}

// emitImageLoadRZSW implements ReadZeroSkipWrite bounds checking for image loads.
// Uses nested selection merge with Phi to return zero for out-of-bounds reads.
// Matching Rust naga's write_conditional_image_access.
func (e *ExpressionEmitter) emitImageLoadRZSW(
	imageID uint32, opcode OpCode, instrTypeID uint32,
	coords imageCoordinates, levelID, sampleID *uint32,
) (uint32, error) {
	e.backend.addCapability(CapabilityImageQuery)

	boolTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	i32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})

	// Null value for out-of-bounds
	nullID := e.backend.builder.AddConstantNull(instrTypeID)

	// Merge block ID (shared target for all false branches)
	mergeBlockID := e.backend.builder.AllocID()

	// Track all (value, block) pairs for the Phi instruction
	type phiEntry struct {
		valueID uint32
		blockID uint32
	}
	var phiEntries []phiEntry

	// Current block ID for tracking Phi source blocks
	// We record the block ID where we branch to merge (false path)
	entryBlockID := e.currentBlock.LabelID

	// Check level bounds
	if levelID != nil {
		// OpImageQueryLevels
		numLevelsID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(i32TypeID)
		ib.AddWord(numLevelsID)
		ib.AddWord(imageID)
		e.backend.builder.funcAppend(ib.Build(OpImageQueryLevels))

		// level < numLevels?
		lodCondID := e.backend.builder.AllocID()
		ib = e.newIB()
		ib.AddWord(boolTypeID)
		ib.AddWord(lodCondID)
		ib.AddWord(*levelID)
		ib.AddWord(numLevelsID)
		e.backend.builder.funcAppend(ib.Build(OpULessThan))

		// SelectionMerge + BranchConditional
		trueBlockID := e.backend.builder.AllocID()

		ib = e.newIB()
		ib.AddWord(mergeBlockID)
		ib.AddWord(0) // SelectionControl::None
		e.backend.builder.funcAppend(ib.Build(OpSelectionMerge))

		// False path goes to merge with null
		phiEntries = append(phiEntries, phiEntry{nullID, e.currentBlock.LabelID})

		// Consume current block with BranchConditional
		e.consumeBlock(Instruction{
			Opcode: OpBranchConditional,
			Words:  []uint32{lodCondID, trueBlockID, mergeBlockID},
		})

		// Start true block
		trueBlock := &Block{LabelID: trueBlockID}
		e.setCurrentBlock(trueBlock)
	} else {
		// No level check needed, but we still record entry block for Phi
	}

	// Check sample bounds
	if sampleID != nil {
		// OpImageQuerySamples
		numSamplesID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(i32TypeID)
		ib.AddWord(numSamplesID)
		ib.AddWord(imageID)
		e.backend.builder.funcAppend(ib.Build(OpImageQuerySamples))

		// sample < numSamples?
		sampleCondID := e.backend.builder.AllocID()
		ib = e.newIB()
		ib.AddWord(boolTypeID)
		ib.AddWord(sampleCondID)
		ib.AddWord(*sampleID)
		ib.AddWord(numSamplesID)
		e.backend.builder.funcAppend(ib.Build(OpULessThan))

		// False path goes to merge with null
		phiEntries = append(phiEntries, phiEntry{nullID, e.currentBlock.LabelID})

		trueBlockID := e.backend.builder.AllocID()

		// BranchConditional (no SelectionMerge for nested checks)
		e.consumeBlock(Instruction{
			Opcode: OpBranchConditional,
			Words:  []uint32{sampleCondID, trueBlockID, mergeBlockID},
		})

		trueBlock := &Block{LabelID: trueBlockID}
		e.setCurrentBlock(trueBlock)
	}

	// Check coordinate bounds
	{
		// Query image size
		coordBoundsID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(coords.typeID)
		ib.AddWord(coordBoundsID)
		ib.AddWord(imageID)
		if levelID != nil {
			ib.AddWord(*levelID)
			e.backend.builder.funcAppend(ib.Build(OpImageQuerySizeLod))
		} else {
			e.backend.builder.funcAppend(ib.Build(OpImageQuerySize))
		}

		// coords < size? (component-wise ULessThan)
		var coordsBoolTypeID uint32
		if coords.size > 1 {
			boolScalarID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
			coordsBoolTypeID = e.backend.emitVectorType(boolScalarID, uint32(coords.size))
		} else {
			coordsBoolTypeID = boolTypeID
		}

		coordsCondsID := e.backend.builder.AllocID()
		ib = e.newIB()
		ib.AddWord(coordsBoolTypeID)
		ib.AddWord(coordsCondsID)
		ib.AddWord(coords.valueID)
		ib.AddWord(coordBoundsID)
		e.backend.builder.funcAppend(ib.Build(OpULessThan))

		// If vector comparison, reduce with OpAll
		coordCondID := coordsCondsID
		if coordsBoolTypeID != boolTypeID {
			allID := e.backend.builder.AllocID()
			ib = e.newIB()
			ib.AddWord(boolTypeID)
			ib.AddWord(allID)
			ib.AddWord(coordsCondsID)
			e.backend.builder.funcAppend(ib.Build(OpAll))
			coordCondID = allID
		}

		// False path goes to merge with null
		phiEntries = append(phiEntries, phiEntry{nullID, e.currentBlock.LabelID})

		accessBlockID := e.backend.builder.AllocID()

		// BranchConditional
		e.consumeBlock(Instruction{
			Opcode: OpBranchConditional,
			Words:  []uint32{coordCondID, accessBlockID, mergeBlockID},
		})

		// Access block — do the actual fetch
		accessBlock := &Block{LabelID: accessBlockID}
		e.setCurrentBlock(accessBlock)
	}

	// Perform the actual image access
	texelID := e.emitImageFetchOrRead(opcode, instrTypeID, imageID, coords.valueID, levelID, sampleID)

	// Record the success result
	phiEntries = append(phiEntries, phiEntry{texelID, e.currentBlock.LabelID})

	// Branch to merge
	e.consumeBlock(makeBranchInstruction(mergeBlockID))

	// Merge block with Phi
	mergeBlock := &Block{LabelID: mergeBlockID}
	e.setCurrentBlock(mergeBlock)

	// OpPhi: result_type result_id [value block]+
	phiResultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(instrTypeID)
	ib.AddWord(phiResultID)
	for _, entry := range phiEntries {
		ib.AddWord(entry.valueID)
		ib.AddWord(entry.blockID)
	}
	e.backend.builder.funcAppend(ib.Build(OpPhi))

	_ = entryBlockID
	return phiResultID, nil
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

	// Resolve image type for dimension-dependent queries.
	imageType, err := ir.ResolveExpressionType(e.backend.module, e.function, query.Image)
	if err != nil {
		return 0, fmt.Errorf("emitImageQuery: resolve image type: %w", err)
	}
	imageInner := typeResolutionInner(e.backend.module, imageType)
	imgType, _ := imageInner.(ir.ImageType)

	switch q := query.Query.(type) {
	case ir.ImageQuerySize:
		// Determine the number of coordinate components based on image dimension.
		// Matches Rust naga: dim_coords + array_coords for the extended SPIR-V result,
		// then shuffle down to dim_coords for the IR result type.
		dimCoords := 2 // default for Dim2D
		switch imgType.Dim {
		case ir.Dim1D:
			dimCoords = 1
		case ir.Dim2D, ir.DimCube:
			dimCoords = 2
		case ir.Dim3D:
			dimCoords = 3
		}
		arrayCoords := 0
		if imgType.Arrayed {
			arrayCoords = 1
		}
		extendedSize := dimCoords + arrayCoords

		scalarID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})

		// The type for OpImageQuerySize result (includes array dimension if arrayed).
		var extendedTypeID uint32
		if extendedSize == 1 {
			extendedTypeID = scalarID
		} else {
			extendedTypeID = e.backend.emitVectorType(scalarID, uint32(extendedSize))
		}

		// The type for the IR result (spatial dimensions only, no array layer).
		var resultTypeID uint32
		if dimCoords == 1 {
			resultTypeID = scalarID
		} else {
			resultTypeID = e.backend.emitVectorType(scalarID, uint32(dimCoords))
		}

		extendedID := e.backend.builder.AllocID()
		builder.AddWord(extendedTypeID)
		builder.AddWord(extendedID)
		builder.AddWord(imageID)

		// Matching Rust naga: multisampled or storage images use OpImageQuerySize (no level).
		// Sampled/depth non-multisampled images use OpImageQuerySizeLod (with explicit or default level 0).
		// SPIR-V spec: OpImageQuerySize requires MS=1 or Sampled!=1.
		useQuerySize := false
		switch imgType.Class {
		case ir.ImageClassSampled:
			useQuerySize = imgType.Multisampled
		case ir.ImageClassDepth:
			useQuerySize = imgType.Multisampled
		case ir.ImageClassExternal:
			// External textures have Sampled=1, need OpImageQuerySizeLod like sampled
			useQuerySize = false
		case ir.ImageClassStorage:
			useQuerySize = true
		default:
			useQuerySize = true
		}

		if useQuerySize {
			e.backend.builder.funcAppend(builder.Build(OpImageQuerySize))
		} else {
			// Sampled/depth non-multisampled: use OpImageQuerySizeLod
			var levelID uint32
			if q.Level != nil {
				lid, err := e.emitExpression(*q.Level)
				if err != nil {
					return 0, err
				}
				levelID = lid
			} else {
				// Default level 0 (matching Rust naga)
				i32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
				levelID = e.backend.builder.AddConstant(i32TypeID, 0)
			}
			builder.AddWord(levelID)
			e.backend.builder.funcAppend(builder.Build(OpImageQuerySizeLod))
		}

		// If arrayed, we need to shuffle to extract only spatial dimensions.
		if resultTypeID != extendedTypeID {
			var components []uint32
			if imgType.Dim == ir.DimCube {
				// Cube: always pick first component duplicated for both dims
				components = []uint32{0, 0}
			} else {
				components = make([]uint32, dimCoords)
				for i := 0; i < dimCoords; i++ {
					components[i] = uint32(i)
				}
			}
			resultID = e.backend.builder.AddVectorShuffle(resultTypeID, extendedID, extendedID, components)
		} else {
			resultID = extendedID
		}

	case ir.ImageQueryNumLevels:
		resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		resultID = e.backend.builder.AllocID()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(imageID)
		e.backend.builder.funcAppend(builder.Build(OpImageQueryLevels))

	case ir.ImageQueryNumLayers:
		// NumLayers uses OpImageQuerySizeLod to get the extended size vector,
		// then extracts the last component (the layer count).
		// Matches Rust naga: vec_size based on dim, then CompositeExtract last element.
		var vecSize uint32
		switch imgType.Dim {
		case ir.Dim1D:
			vecSize = 2 // Bi
		case ir.Dim2D, ir.DimCube:
			vecSize = 3 // Tri
		case ir.Dim3D:
			vecSize = 4 // Quad
		default:
			vecSize = 3
		}

		scalarID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		extendedTypeID := e.backend.emitVectorType(scalarID, vecSize)

		extendedID := e.backend.builder.AllocID()
		builder.AddWord(extendedTypeID)
		builder.AddWord(extendedID)
		builder.AddWord(imageID)
		// Always use level 0 for NumLayers query
		zeroID := e.backend.builder.AddConstant(scalarID, 0)
		builder.AddWord(zeroID)
		e.backend.builder.funcAppend(builder.Build(OpImageQuerySizeLod))

		// Extract the last component (layer count)
		resultType := scalarID
		resultID = e.backend.builder.AddCompositeExtract(resultType, extendedID, vecSize-1)

	case ir.ImageQueryNumSamples:
		resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
		resultID = e.backend.builder.AllocID()
		builder.AddWord(resultType)
		builder.AddWord(resultID)
		builder.AddWord(imageID)
		e.backend.builder.funcAppend(builder.Build(OpImageQuerySamples))

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

	// Get or create the image type (will be cached by emitImageType).
	// Use the correct sampled type based on image class (matches Rust naga).
	var sampledScalar ir.ScalarType
	switch img.Class {
	case ir.ImageClassSampled:
		sampledScalar = ir.ScalarType{Kind: img.SampledKind, Width: 4}
	case ir.ImageClassDepth:
		sampledScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	case ir.ImageClassStorage:
		sampledScalar = storageFormatToScalar(img.StorageFormat)
	default:
		sampledScalar = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	imageTypeID := b.emitImageType(
		b.emitScalarType(sampledScalar),
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

	// Memory scope: Device if STORAGE, Subgroup if SUB_GROUP, else Workgroup.
	// Matches Rust naga writer.rs:1816-1822.
	var memoryScope uint32
	if stmt.Flags&ir.BarrierStorage != 0 {
		memoryScope = ScopeDevice
	} else if stmt.Flags&ir.BarrierSubGroup != 0 {
		memoryScope = ScopeSubgroup
	} else {
		memoryScope = ScopeWorkgroup
	}

	// Memory semantics based on barrier flags.
	// Matches Rust naga writer.rs:1823-1839.
	semantics := MemorySemanticsAcquireRelease
	if stmt.Flags&ir.BarrierStorage != 0 {
		semantics |= MemorySemanticsUniformMemory
	}
	if stmt.Flags&ir.BarrierWorkGroup != 0 {
		semantics |= MemorySemanticsWorkgroupMemory
	}
	if stmt.Flags&ir.BarrierSubGroup != 0 {
		semantics |= MemorySemanticsSubgroupMemory
	}
	if stmt.Flags&ir.BarrierTexture != 0 {
		semantics |= MemorySemanticsImageMemory
	}

	// Execution scope: Subgroup if SUB_GROUP, else Workgroup.
	// Matches Rust naga writer.rs:1840-1844.
	var execScope uint32
	if stmt.Flags&ir.BarrierSubGroup != 0 {
		execScope = ScopeSubgroup
	} else {
		execScope = ScopeWorkgroup
	}

	execScopeID := e.backend.builder.AddConstant(u32TypeID, execScope)
	memScopeID := e.backend.builder.AddConstant(u32TypeID, memoryScope)
	semanticsID := e.backend.builder.AddConstant(u32TypeID, semantics)

	// OpControlBarrier Execution Memory Semantics
	builder := e.newIB()
	builder.AddWord(execScopeID)
	builder.AddWord(memScopeID)
	builder.AddWord(semanticsID)
	e.backend.builder.funcAppend(builder.Build(OpControlBarrier))

	return nil
}

// emitWorkGroupUniformLoad emits a workgroup uniform load:
// barrier -> load -> barrier. Matches Rust naga block.rs:3614.
func (e *ExpressionEmitter) emitWorkGroupUniformLoad(stmt ir.StmtWorkGroupUniformLoad) error {
	// Emit workgroup barrier before load
	_ = e.emitBarrier(ir.StmtBarrier{Flags: ir.BarrierWorkGroup})

	// Resolve the result type from the result expression
	resultType, err := ir.ResolveExpressionType(e.backend.module, e.function, stmt.Result)
	if err != nil {
		return fmt.Errorf("workgroup uniform load: cannot resolve result type: %w", err)
	}
	resultTypeID := e.backend.resolveTypeResolution(resultType)

	// Load from the pointer (need raw pointer, not loaded value)
	pointerID, err := e.emitPointerExpression(stmt.Pointer)
	if err != nil {
		return fmt.Errorf("workgroup uniform load: cannot emit pointer: %w", err)
	}
	loadID := e.backend.builder.AddLoad(resultTypeID, pointerID)

	// Cache the result
	e.exprIDs[stmt.Result] = loadID

	// Emit workgroup barrier after load
	_ = e.emitBarrier(ir.StmtBarrier{Flags: ir.BarrierWorkGroup})

	return nil
}

// resolveAtomicScalar extracts the full scalar type (kind + width) from an atomic pointer expression.
// Returns {ScalarUint, 4} as default if the type cannot be resolved.
func (e *ExpressionEmitter) resolveAtomicScalar(pointer ir.ExpressionHandle) ir.ScalarType {
	defaultScalar := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}

	pointerType, err := ir.ResolveExpressionType(e.backend.module, e.function, pointer)
	if err != nil {
		return defaultScalar
	}

	// Get the inner type from TypeResolution (either from Handle or Value)
	var inner ir.TypeInner
	if pointerType.Handle != nil && int(*pointerType.Handle) < len(e.backend.module.Types) {
		inner = e.backend.module.Types[*pointerType.Handle].Inner
	} else {
		inner = pointerType.Value
	}

	// ResolveExpressionType may return the atomic type directly (e.g., for
	// struct field access like tiles[i].backdrop) or wrapped in a PointerType.
	if atomicType, ok := inner.(ir.AtomicType); ok {
		return atomicType.Scalar
	}

	ptrType, ok := inner.(ir.PointerType)
	if !ok || int(ptrType.Base) >= len(e.backend.module.Types) {
		return defaultScalar
	}

	atomicType, ok := e.backend.module.Types[ptrType.Base].Inner.(ir.AtomicType)
	if !ok {
		return defaultScalar
	}

	return atomicType.Scalar
}

// resolveAtomicScalarKind extracts the scalar kind from an atomic pointer expression.
// Returns ScalarUint as default if the type cannot be resolved.
func (e *ExpressionEmitter) resolveAtomicScalarKind(pointer ir.ExpressionHandle) ir.ScalarKind {
	return e.resolveAtomicScalar(pointer).Kind
}

// atomicOpcode returns the SPIR-V opcode for an atomic function.
// Returns 0 if the function is AtomicExchange with a compare value (handled separately).
func atomicOpcode(fun ir.AtomicFunction, scalarKind ir.ScalarKind) (OpCode, bool) {
	switch fun.(type) {
	case ir.AtomicAdd:
		if scalarKind == ir.ScalarFloat {
			return OpAtomicFAddEXT, true
		}
		return OpAtomicIAdd, true
	case ir.AtomicSubtract:
		// Float subtract uses FNegate + AtomicFAddEXT (handled in emitAtomic).
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
	// Atomic ops need a POINTER, not a loaded value.
	pointerID, err := e.emitPointerExpression(stmt.Pointer)
	if err != nil {
		return err
	}

	// Determine scalar type from pointer type (atomic<i32> vs atomic<u32> vs atomic<i64>)
	atomicScalar := e.resolveAtomicScalar(stmt.Pointer)
	scalarKind := atomicScalar.Kind
	resultTypeID := e.backend.emitScalarType(atomicScalar)

	// Scope and memory semantics constants
	scopeID := e.backend.builder.AddConstant(
		e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
		ScopeDevice,
	)
	semanticsID := e.backend.builder.AddConstant(
		e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
		MemorySemanticsAcquireRelease|MemorySemanticsUniformMemory,
	)

	// Handle AtomicLoad: OpAtomicLoad ResultType Result Pointer Scope Semantics (no value)
	// SPIR-V requires Acquire semantics (not AcquireRelease) for loads.
	if _, ok := stmt.Fun.(ir.AtomicLoad); ok {
		acquireSemID := e.backend.builder.AddConstant(
			e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
			MemorySemanticsAcquire|MemorySemanticsUniformMemory,
		)
		resultID := e.backend.builder.AllocID()
		builder := e.newIB()
		builder.AddWord(resultTypeID)
		builder.AddWord(resultID)
		builder.AddWord(pointerID)
		builder.AddWord(scopeID)
		builder.AddWord(acquireSemID)
		e.backend.builder.funcAppend(builder.Build(OpAtomicLoad))
		if stmt.Result != nil {
			e.exprIDs[*stmt.Result] = resultID
			if err := e.processDeferredStores(*stmt.Result, resultID); err != nil {
				return err
			}
		}
		return nil
	}

	// All remaining atomic ops need a value operand
	valueID, err := e.emitExpression(stmt.Value)
	if err != nil {
		return err
	}

	// Handle compare-exchange separately
	if exchange, ok := stmt.Fun.(ir.AtomicExchange); ok && exchange.Compare != nil {
		return e.emitAtomicCompareExchange(stmt, pointerID, valueID, resultTypeID, scopeID, semanticsID, *exchange.Compare)
	}

	// Handle AtomicStore: OpAtomicStore Pointer Scope Semantics Value (no result)
	// SPIR-V requires Release semantics (not AcquireRelease) for stores.
	if _, ok := stmt.Fun.(ir.AtomicStore); ok {
		releaseSemID := e.backend.builder.AddConstant(
			e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
			MemorySemanticsRelease|MemorySemanticsUniformMemory,
		)
		builder := e.newIB()
		builder.AddWord(pointerID)
		builder.AddWord(scopeID)
		builder.AddWord(releaseSemID)
		builder.AddWord(valueID)
		e.backend.builder.funcAppend(builder.Build(OpAtomicStore))
		return nil
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
	e.backend.builder.funcAppend(builder.Build(opcode))

	if stmt.Result != nil {
		e.exprIDs[*stmt.Result] = resultID
		if err := e.processDeferredStores(*stmt.Result, resultID); err != nil {
			return err
		}
	}
	return nil
}

// emitAtomicCompareExchange emits an atomic compare-exchange operation.
// SPIR-V OpAtomicCompareExchange returns a scalar (the old value), not a struct.
// WGSL wraps the result in a struct {old_value: T, exchanged: bool}.
// We emit: OpAtomicCompareExchange (scalar), OpIEqual (bool), OpCompositeConstruct (struct).
// This matches Rust naga's approach in back/spv/block.rs.
func (e *ExpressionEmitter) emitAtomicCompareExchange(
	stmt ir.StmtAtomic,
	pointerID, valueID, scalarTypeID, scopeID, semanticsID uint32,
	compare ir.ExpressionHandle,
) error {
	compareID, err := e.emitExpression(compare)
	if err != nil {
		return err
	}

	// 1. OpAtomicCompareExchange returns the OLD scalar value
	casResultID := e.backend.builder.AllocID()
	builder := e.newIB()
	builder.AddWord(scalarTypeID)
	builder.AddWord(casResultID)
	builder.AddWord(pointerID)
	builder.AddWord(scopeID)
	builder.AddWord(semanticsID) // MemSemEqual
	// MemSemUnequal: SPIR-V spec (VUID-10875) forbids Release/AcquireRelease
	// for the "unequal" operand. Use Acquire instead (satisfies VUID-10871 which
	// requires non-relaxed order when storage class semantics bits are set).
	unequalSemID := e.backend.builder.AddConstant(
		e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
		MemorySemanticsAcquire|MemorySemanticsUniformMemory,
	)
	builder.AddWord(unequalSemID) // MemSemUnequal (Acquire, not AcquireRelease)
	builder.AddWord(valueID)
	builder.AddWord(compareID)
	e.backend.builder.funcAppend(builder.Build(OpAtomicCompareExch))

	// 2. OpIEqual: exchanged = (old_value == compare)
	boolTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	equalityResultID := e.backend.builder.AllocID()
	eqBuilder := e.newIB()
	eqBuilder.AddWord(boolTypeID)
	eqBuilder.AddWord(equalityResultID)
	eqBuilder.AddWord(casResultID)
	eqBuilder.AddWord(compareID)
	e.backend.builder.funcAppend(eqBuilder.Build(OpIEqual))

	// 3. OpCompositeConstruct: build the result struct {old_value, exchanged}
	if stmt.Result != nil {
		resultExpr := e.function.Expressions[*stmt.Result]
		atomicResult, ok := resultExpr.Kind.(ir.ExprAtomicResult)
		if !ok {
			return fmt.Errorf("atomic compare-exchange result expression is not ExprAtomicResult")
		}
		structTypeID, err := e.backend.emitType(atomicResult.Ty)
		if err != nil {
			return fmt.Errorf("failed to emit atomic result struct type: %w", err)
		}

		compositeID := e.backend.builder.AllocID()
		ccBuilder := e.newIB()
		ccBuilder.AddWord(structTypeID)
		ccBuilder.AddWord(compositeID)
		ccBuilder.AddWord(casResultID)
		ccBuilder.AddWord(equalityResultID)
		e.backend.builder.funcAppend(ccBuilder.Build(OpCompositeConstruct))

		e.exprIDs[*stmt.Result] = compositeID
		if err := e.processDeferredStores(*stmt.Result, compositeID); err != nil {
			return err
		}
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
		e.backend.builder.funcAppend(cb.Build(opcode))
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
		e.backend.builder.funcAppend(eb.Build(OpCompositeConstruct))
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

	// Build combined coordinates with array index (integer coords, matching Rust naga).
	// Image store uses integer coordinates, NOT float — so we use emitImageCoordinates
	// which does bitcast (e.g. u32→i32) instead of appendArrayIndex which converts to float.
	coords, err := e.emitImageCoordinates(store.Coordinate, store.ArrayIndex)
	if err != nil {
		return fmt.Errorf("image store coordinate: %w", err)
	}

	valueID, err := e.emitExpression(store.Value)
	if err != nil {
		return fmt.Errorf("image store value: %w", err)
	}

	// OpImageWrite: no result type, no result ID
	builder := e.newIB()
	builder.AddWord(imageID)
	builder.AddWord(coords.valueID)
	builder.AddWord(valueID)
	e.backend.builder.funcAppend(builder.Build(OpImageWrite))

	return nil
}

// emitImageAtomic emits an atomic operation on a storage texture texel.
// Uses OpImageTexelPointer to get a pointer to the texel, then a standard
// atomic op (OpAtomicIAdd, etc.) on that pointer.
// Matches Rust naga back/spv/image.rs write_image_atomic.
func (e *ExpressionEmitter) emitImageAtomic(stmt ir.StmtImageAtomic) error {
	// Find the global variable for the image (OpImageTexelPointer needs the variable, not loaded value).
	imageGlobalHandle, err := e.resolveImageGlobalVar(stmt.Image)
	if err != nil {
		return fmt.Errorf("image atomic: %w", err)
	}
	imageVarID, ok := e.backend.globalIDs[imageGlobalHandle]
	if !ok {
		return fmt.Errorf("image atomic: global variable %d not found", imageGlobalHandle)
	}

	// Get the storage format from the image type to determine scalar type.
	gv := e.backend.module.GlobalVariables[imageGlobalHandle]
	imageInner := e.backend.module.Types[gv.Type].Inner
	imgType, ok := imageInner.(ir.ImageType)
	if !ok {
		return fmt.Errorf("image atomic: expected ImageType, got %T", imageInner)
	}
	scalar := imgType.StorageFormat.Scalar()
	scalarTypeID := e.backend.emitScalarType(scalar)

	// For 64-bit image atomics, require Int64Atomics capability.
	if scalar.Width == 8 {
		e.backend.addCapability(CapabilityInt64Atomics)
	}

	// Build pointer type: OpTypePointer Image <scalar>
	pointerTypeID := e.backend.emitPointerType(StorageClassImage, scalarTypeID)

	// Build combined coordinates (integer coords with optional array index).
	coords, err := e.emitImageCoordinates(stmt.Coordinate, stmt.ArrayIndex)
	if err != nil {
		return fmt.Errorf("image atomic coordinate: %w", err)
	}

	// Sample index is always 0 for storage textures.
	sampleID := e.backend.emitU32Constant(0)

	// Emit OpImageTexelPointer: result is a pointer to the texel.
	pointerID := e.backend.builder.AllocID()
	{
		ib := e.newIB()
		ib.AddWord(pointerTypeID)
		ib.AddWord(pointerID)
		ib.AddWord(imageVarID)
		ib.AddWord(coords.valueID)
		ib.AddWord(sampleID)
		e.backend.builder.funcAppend(ib.Build(OpImageTexelPointer))
	}

	// Scope and memory semantics for Handle address space:
	// Rust naga uses (MemorySemantics::empty(), Scope::Device) for Handle space.
	scopeID := e.backend.emitI32Constant(int32(ScopeDevice))
	semanticsID := e.backend.emitU32Constant(0) // MemorySemantics::empty()

	// Emit the value operand.
	valueID, err := e.emitExpression(stmt.Value)
	if err != nil {
		return fmt.Errorf("image atomic value: %w", err)
	}

	// Determine the atomic opcode.
	opcode, ok := atomicOpcode(stmt.Fun, scalar.Kind)
	if !ok {
		return fmt.Errorf("image atomic: unsupported atomic function: %T", stmt.Fun)
	}

	// Emit the atomic operation: OpAtomic* ResultType Result Pointer Scope Semantics Value
	resultID := e.backend.builder.AllocID()
	{
		ib := e.newIB()
		ib.AddWord(scalarTypeID)
		ib.AddWord(resultID)
		ib.AddWord(pointerID)
		ib.AddWord(scopeID)
		ib.AddWord(semanticsID)
		ib.AddWord(valueID)
		e.backend.builder.funcAppend(ib.Build(opcode))
	}

	// Image atomic results are unused (void statement), so we don't cache the result.
	return nil
}

// resolveImageGlobalVar walks through the expression tree to find the
// GlobalVariable handle that an image expression refers to.
func (e *ExpressionEmitter) resolveImageGlobalVar(handle ir.ExpressionHandle) (ir.GlobalVariableHandle, error) {
	expr := e.function.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		return k.Variable, nil
	default:
		return 0, fmt.Errorf("expected GlobalVariable for image, got %T", expr.Kind)
	}
}

// emitI32Constant emits or reuses a signed 32-bit integer constant.
func (b *Backend) emitI32Constant(val int32) uint32 {
	typeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	return b.builder.AddConstant(typeID, uint32(val))
}

// emitU32Constant emits or reuses an unsigned 32-bit integer constant.
func (b *Backend) emitU32Constant(val uint32) uint32 {
	typeID := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	return b.builder.AddConstant(typeID, val)
}

// spilledPointerArg tracks a pointer argument that was spilled to a temporary
// variable for an OpFunctionCall. After the call, we must write-back from the
// temp to the original location (copy-out).
type spilledPointerArg struct {
	tempVarID     uint32 // OpVariable in Function storage
	originalPtrID uint32 // original OpAccessChain result (or similar)
	pointeeTypeID uint32 // the pointee type (for OpLoad)
}

// isMemoryObjectDeclaration returns true if the expression produces a SPIR-V
// memory object declaration (OpVariable or OpFunctionParameter), which can be
// passed directly as pointer arguments to OpFunctionCall.
// Returns false for OpAccessChain results (ExprAccess, ExprAccessIndex) which
// are NOT valid pointer operands per the SPIR-V spec.
func (e *ExpressionEmitter) isMemoryObjectDeclaration(handle ir.ExpressionHandle) bool {
	expr := e.function.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprLocalVariable:
		return true
	case ir.ExprGlobalVariable:
		return true
	case ir.ExprFunctionArgument:
		return true
	case ir.ExprLoad:
		// ExprLoad chains through -- check the underlying pointer
		return e.isMemoryObjectDeclaration(k.Pointer)
	default:
		return false
	}
}

// emitCall emits a function call statement (OpFunctionCall).
// If the call has a result, the SPIR-V result ID is cached for later ExprCallResult lookup.
//
// For pointer parameters whose arguments are not memory object declarations
// (e.g. OpAccessChain results from array/struct indexing), we use the
// copy-in/copy-out (spill) pattern:
//  1. Create a temporary OpVariable in Function storage
//  2. OpLoad from the original location and OpStore into the temp
//  3. Pass the temp variable to OpFunctionCall
//  4. After the call, OpLoad from temp and OpStore back (write-back)
func (e *ExpressionEmitter) emitCall(call ir.StmtCall) error {
	// Look up the SPIR-V function ID
	funcID, ok := e.backend.functionIDs[call.Function]
	if !ok {
		return fmt.Errorf("function %d not found in functionIDs", call.Function)
	}

	// Collect argument SPIR-V IDs.
	// For pointer parameters, pass the pointer directly (emitPointerExpression).
	// For value parameters, pass the loaded value (emitExpression).
	targetFn := e.backend.module.Functions[call.Function]
	argIDs := make([]uint32, 0, len(call.Arguments))
	var spills []spilledPointerArg
	for i, arg := range call.Arguments {
		// Check if the corresponding parameter expects a pointer type
		var ptrType ir.PointerType
		isPointerParam := false
		if i < len(targetFn.Arguments) {
			paramType := e.backend.module.Types[targetFn.Arguments[i].Type].Inner
			switch pt := paramType.(type) {
			case ir.PointerType:
				isPointerParam = true
				ptrType = pt
			case ir.ValuePointerType:
				isPointerParam = true
			}
		}

		var argID uint32
		var err error
		if isPointerParam {
			argID, err = e.emitPointerExpression(arg)
			if err != nil {
				return fmt.Errorf("call argument: %w", err)
			}

			// Check if this pointer is a memory object declaration.
			// If not (e.g. OpAccessChain from array indexing), we must spill
			// to a temporary variable per the SPIR-V spec.
			if !e.isMemoryObjectDeclaration(arg) {
				// Get the pointee type ID
				pointeeTypeID, typeErr := e.backend.emitType(ptrType.Base)
				if typeErr != nil {
					return fmt.Errorf("call spill pointee type: %w", typeErr)
				}

				// Create a Function-scope temporary variable
				ptrTypeID := e.backend.emitPointerType(StorageClassFunction, pointeeTypeID)
				tempVarID := e.backend.builder.AllocID()
				ib := e.newIB()
				ib.AddWord(ptrTypeID)
				ib.AddWord(tempVarID)
				ib.AddWord(uint32(StorageClassFunction))
				e.funcBuilder.Variables = append(e.funcBuilder.Variables, ib.Build(OpVariable))

				// Copy-in: OpLoad from original, OpStore to temp
				loadedValue := e.backend.builder.AddLoad(pointeeTypeID, argID)
				e.backend.builder.AddStore(tempVarID, loadedValue)

				// Track for write-back after the call
				spills = append(spills, spilledPointerArg{
					tempVarID:     tempVarID,
					originalPtrID: argID,
					pointeeTypeID: pointeeTypeID,
				})

				// Use temp variable as the argument
				argID = tempVarID
			}
		} else {
			argID, err = e.emitExpression(arg)
			if err != nil {
				return fmt.Errorf("call argument: %w", err)
			}
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
	e.backend.builder.funcAppend(builder.Build(OpFunctionCall))

	// Copy-out (write-back): for each spilled pointer argument, load from
	// the temp variable and store back to the original location.
	for _, spill := range spills {
		wb := e.backend.builder.AddLoad(spill.pointeeTypeID, spill.tempVarID)
		e.backend.builder.AddStore(spill.originalPtrID, wb)
	}

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
	// Walk the Array expression to find the global variable, optional member index,
	// and optional binding array index. Matches Rust naga back/spv/index.rs.
	var globalHandle ir.GlobalVariableHandle
	var optLastMemberIndex *uint32
	var bindingArrayIndexID *uint32

	arrayExpr := e.function.Expressions[expr.Array]
	switch k := arrayExpr.Kind.(type) {
	case ir.ExprAccessIndex:
		baseExpr := e.function.Expressions[k.Base]
		switch bk := baseExpr.Kind.(type) {
		case ir.ExprAccessIndex:
			// AccessIndex(AccessIndex(Global)) -- binding array with constant index
			baseOuterExpr := e.function.Expressions[bk.Base]
			gv, ok := baseOuterExpr.Kind.(ir.ExprGlobalVariable)
			if !ok {
				return 0, fmt.Errorf("array length: AccessIndex(AccessIndex(x)): expected GlobalVariable, got %T", baseOuterExpr.Kind)
			}
			indexID := e.backend.emitU32Constant(bk.Index)
			bindingArrayIndexID = &indexID
			globalHandle = gv.Variable
			idx := k.Index
			optLastMemberIndex = &idx

		case ir.ExprAccess:
			// AccessIndex(Access(Global)) -- binding array with dynamic index
			baseOuterExpr := e.function.Expressions[bk.Base]
			gv, ok := baseOuterExpr.Kind.(ir.ExprGlobalVariable)
			if !ok {
				return 0, fmt.Errorf("array length: AccessIndex(Access(x)): expected GlobalVariable, got %T", baseOuterExpr.Kind)
			}
			dynIndexID, err := e.emitExpression(bk.Index)
			if err != nil {
				return 0, fmt.Errorf("array length: dynamic index: %w", err)
			}
			bindingArrayIndexID = &dynIndexID
			globalHandle = gv.Variable
			idx := k.Index
			optLastMemberIndex = &idx

		case ir.ExprGlobalVariable:
			globalHandle = bk.Variable
			idx := k.Index
			optLastMemberIndex = &idx

		default:
			return 0, fmt.Errorf("array length: AccessIndex base: expected GlobalVariable, got %T", bk)
		}
	case ir.ExprGlobalVariable:
		globalHandle = k.Variable
		optLastMemberIndex = nil
	default:
		return 0, fmt.Errorf("array length: expected GlobalVariable or AccessIndex, got %T", k)
	}

	varID, ok := e.backend.globalIDs[globalHandle]
	if !ok {
		return 0, fmt.Errorf("array length: global variable %d not found", globalHandle)
	}

	isWrapped := e.backend.wrappedStorageVars[globalHandle]

	var lastMemberIndex uint32
	var gvarID uint32

	switch {
	case optLastMemberIndex != nil && !isWrapped:
		lastMemberIndex = *optLastMemberIndex
		gvarID = varID
	case optLastMemberIndex == nil && isWrapped:
		lastMemberIndex = 0
		gvarID = varID
	case optLastMemberIndex != nil && isWrapped:
		return 0, fmt.Errorf("array length: unexpected wrapped variable with AccessIndex")
	default:
		return 0, fmt.Errorf("array length: global variable is not a struct and was not wrapped")
	}

	// If we have a binding array index, emit OpAccessChain to index into the array.
	var structID uint32
	if bindingArrayIndexID != nil {
		gv := e.backend.module.GlobalVariables[globalHandle]
		ba, ok := e.backend.module.Types[gv.Type].Inner.(ir.BindingArrayType)
		if !ok {
			return 0, fmt.Errorf("array length: expected BindingArray type for binding array access")
		}
		baseTypeID, err := e.backend.emitType(ba.Base)
		if err != nil {
			return 0, fmt.Errorf("array length: emit base type: %w", err)
		}
		sc := addressSpaceToStorageClass(gv.Space)
		ptrTypeID := e.backend.emitPointerType(sc, baseTypeID)

		structID = e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(ptrTypeID)
		ib.AddWord(structID)
		ib.AddWord(gvarID)
		ib.AddWord(*bindingArrayIndexID)
		e.backend.builder.funcAppend(ib.Build(OpAccessChain))
	} else {
		structID = gvarID
	}

	resultType := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})

	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultType)
	ib.AddWord(resultID)
	ib.AddWord(structID)
	ib.AddWord(lastMemberIndex)
	e.backend.builder.funcAppend(ib.Build(OpArrayLength))

	return resultID, nil
}

// emitSubgroupResultRef returns the SPIR-V ID for a subgroup result expression.
// The ID is set by the corresponding subgroup statement emit function.
func (e *ExpressionEmitter) emitSubgroupResultRef(handle ir.ExpressionHandle) (uint32, error) {
	if existingID, ok := e.exprIDs[handle]; ok {
		return existingID, nil
	}
	return 0, fmt.Errorf("subgroup result expression %d not yet computed", handle)
}

// emitSubgroupBallot emits a SubgroupBallot statement.
func (e *ExpressionEmitter) emitSubgroupBallot(stmt ir.StmtSubgroupBallot) error {
	e.backend.requireVersion(Version1_3)
	e.backend.addCapability(CapabilityGroupNonUniformBallot)
	u32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	vec4u32TypeID := e.backend.emitVectorType(u32TypeID, 4)
	scopeID := e.backend.builder.AddConstant(u32TypeID, ScopeSubgroup)

	var predicateID uint32
	if stmt.Predicate != nil {
		var err error
		predicateID, err = e.emitExpression(*stmt.Predicate)
		if err != nil {
			return err
		}
	} else {
		// Default predicate: true (OpConstantTrue)
		boolTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
		predicateID = e.backend.builder.AllocID()
		trueIB := e.newIB()
		trueIB.AddWord(boolTypeID)
		trueIB.AddWord(predicateID)
		e.backend.builder.types = append(e.backend.builder.types, trueIB.Build(OpConstantTrue))
	}

	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(vec4u32TypeID)
	ib.AddWord(resultID)
	ib.AddWord(scopeID)
	ib.AddWord(predicateID)
	e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformBallot))

	e.exprIDs[stmt.Result] = resultID
	return nil
}

// emitSubgroupCollectiveOperation emits a SubgroupCollectiveOperation statement.
func (e *ExpressionEmitter) emitSubgroupCollectiveOperation(stmt ir.StmtSubgroupCollectiveOperation) error {
	e.backend.requireVersion(Version1_3)
	u32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	scopeID := e.backend.builder.AddConstant(u32TypeID, ScopeSubgroup)

	argID, err := e.emitExpression(stmt.Argument)
	if err != nil {
		return err
	}

	resultTypeID, err := e.resolveSubgroupTypeID(stmt.Result)
	if err != nil {
		return err
	}

	// Determine the scalar kind of the argument for selecting the right opcode
	scalarKind := e.resolveSubgroupScalarKind(stmt.Argument)

	var groupOp uint32
	switch stmt.CollectiveOp {
	case ir.CollectiveReduce:
		groupOp = GroupOperationReduce
	case ir.CollectiveInclusiveScan:
		groupOp = GroupOperationInclusiveScan
	case ir.CollectiveExclusiveScan:
		groupOp = GroupOperationExclusiveScan
	}

	var opcode OpCode
	switch stmt.Op {
	case ir.SubgroupOperationAll:
		e.backend.addCapability(CapabilityGroupNonUniformVote)
		opcode = OpGroupNonUniformAll
	case ir.SubgroupOperationAny:
		e.backend.addCapability(CapabilityGroupNonUniformVote)
		opcode = OpGroupNonUniformAny
	case ir.SubgroupOperationAdd:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		if scalarKind == ir.ScalarFloat {
			opcode = OpGroupNonUniformFAdd
		} else {
			opcode = OpGroupNonUniformIAdd
		}
	case ir.SubgroupOperationMul:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		if scalarKind == ir.ScalarFloat {
			opcode = OpGroupNonUniformFMul
		} else {
			opcode = OpGroupNonUniformIMul
		}
	case ir.SubgroupOperationMin:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		switch scalarKind {
		case ir.ScalarFloat:
			opcode = OpGroupNonUniformFMin
		case ir.ScalarUint:
			opcode = OpGroupNonUniformUMin
		default:
			opcode = OpGroupNonUniformSMin
		}
	case ir.SubgroupOperationMax:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		switch scalarKind {
		case ir.ScalarFloat:
			opcode = OpGroupNonUniformFMax
		case ir.ScalarUint:
			opcode = OpGroupNonUniformUMax
		default:
			opcode = OpGroupNonUniformSMax
		}
	case ir.SubgroupOperationAnd:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		if scalarKind == ir.ScalarBool {
			opcode = OpGroupNonUniformLogicalAnd
		} else {
			opcode = OpGroupNonUniformBitwiseAnd
		}
	case ir.SubgroupOperationOr:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		if scalarKind == ir.ScalarBool {
			opcode = OpGroupNonUniformLogicalOr
		} else {
			opcode = OpGroupNonUniformBitwiseOr
		}
	case ir.SubgroupOperationXor:
		e.backend.addCapability(CapabilityGroupNonUniformArithmetic)
		if scalarKind == ir.ScalarBool {
			opcode = OpGroupNonUniformLogicalXor
		} else {
			opcode = OpGroupNonUniformBitwiseXor
		}
	}

	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultTypeID)
	ib.AddWord(resultID)
	ib.AddWord(scopeID)

	// All/Any don't use GroupOperation, they just take scope + predicate
	if stmt.Op == ir.SubgroupOperationAll || stmt.Op == ir.SubgroupOperationAny {
		ib.AddWord(argID)
	} else {
		ib.AddWord(groupOp)
		ib.AddWord(argID)
	}

	e.backend.builder.funcAppend(ib.Build(opcode))
	e.exprIDs[stmt.Result] = resultID
	return nil
}

// emitSubgroupGather emits a SubgroupGather statement.
func (e *ExpressionEmitter) emitSubgroupGather(stmt ir.StmtSubgroupGather) error {
	e.backend.requireVersion(Version1_3)
	u32TypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	scopeID := e.backend.builder.AddConstant(u32TypeID, ScopeSubgroup)

	argID, err := e.emitExpression(stmt.Argument)
	if err != nil {
		return err
	}

	resultTypeID, err := e.resolveSubgroupTypeID(stmt.Result)
	if err != nil {
		return err
	}

	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(resultTypeID)
	ib.AddWord(resultID)
	ib.AddWord(scopeID)
	ib.AddWord(argID)

	switch mode := stmt.Mode.(type) {
	case ir.GatherBroadcastFirst:
		e.backend.addCapability(CapabilityGroupNonUniformBallot)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformBroadcastFirst))

	case ir.GatherBroadcast:
		e.backend.addCapability(CapabilityGroupNonUniformBallot)
		indexID, err := e.emitExpression(mode.Index)
		if err != nil {
			return err
		}
		ib.AddWord(indexID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformBroadcast))

	case ir.GatherShuffle:
		e.backend.addCapability(CapabilityGroupNonUniformShuffle)
		indexID, err := e.emitExpression(mode.Index)
		if err != nil {
			return err
		}
		ib.AddWord(indexID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformShuffle))

	case ir.GatherShuffleDown:
		e.backend.addCapability(CapabilityGroupNonUniformShuffleRel)
		deltaID, err := e.emitExpression(mode.Delta)
		if err != nil {
			return err
		}
		ib.AddWord(deltaID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformShuffleDown))

	case ir.GatherShuffleUp:
		e.backend.addCapability(CapabilityGroupNonUniformShuffleRel)
		deltaID, err := e.emitExpression(mode.Delta)
		if err != nil {
			return err
		}
		ib.AddWord(deltaID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformShuffleUp))

	case ir.GatherShuffleXor:
		e.backend.addCapability(CapabilityGroupNonUniformShuffle)
		maskID, err := e.emitExpression(mode.Mask)
		if err != nil {
			return err
		}
		ib.AddWord(maskID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformShuffleXor))

	case ir.GatherQuadBroadcast:
		e.backend.addCapability(CapabilityGroupNonUniformQuad)
		indexID, err := e.emitExpression(mode.Index)
		if err != nil {
			return err
		}
		ib.AddWord(indexID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformQuadBroadcast))

	case ir.GatherQuadSwap:
		e.backend.addCapability(CapabilityGroupNonUniformQuad)
		dirID := e.backend.builder.AddConstant(u32TypeID, uint32(mode.Direction))
		ib.AddWord(dirID)
		e.backend.builder.funcAppend(ib.Build(OpGroupNonUniformQuadSwap))
	}

	e.exprIDs[stmt.Result] = resultID
	return nil
}

// resolveSubgroupTypeID resolves the SPIR-V type ID for a subgroup result expression.
func (e *ExpressionEmitter) resolveSubgroupTypeID(handle ir.ExpressionHandle) (uint32, error) {
	typeRes, err := ir.ResolveExpressionType(e.backend.module, e.function, handle)
	if err != nil {
		return 0, fmt.Errorf("cannot resolve subgroup result type: %w", err)
	}

	if typeRes.Handle != nil {
		return e.backend.emitType(*typeRes.Handle)
	}

	// Inline type - emit based on TypeInner
	switch t := typeRes.Value.(type) {
	case ir.ScalarType:
		return e.backend.emitScalarType(t), nil
	case ir.VectorType:
		scalarID := e.backend.emitScalarType(t.Scalar)
		return e.backend.emitVectorType(scalarID, uint32(t.Size)), nil
	default:
		return 0, fmt.Errorf("unsupported inline subgroup result type: %T", typeRes.Value)
	}
}

// resolveSubgroupScalarKind extracts the scalar kind from a subgroup argument expression.
func (e *ExpressionEmitter) resolveSubgroupScalarKind(handle ir.ExpressionHandle) ir.ScalarKind {
	typeRes, err := ir.ResolveExpressionType(e.backend.module, e.function, handle)
	if err != nil {
		return ir.ScalarUint
	}

	var inner ir.TypeInner
	if typeRes.Handle != nil && int(*typeRes.Handle) < len(e.backend.module.Types) {
		inner = e.backend.module.Types[*typeRes.Handle].Inner
	} else {
		inner = typeRes.Value
	}

	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind
	case ir.VectorType:
		return t.Scalar.Kind
	}
	return ir.ScalarUint
}

// emitRayQueryGetIntersection emits the expression to retrieve a RayIntersection struct.
// Generates a helper function and calls it with the ray query pointer and tracker.
func (e *ExpressionEmitter) emitRayQueryGetIntersection(expr ir.ExprRayQueryGetIntersection) (uint32, error) {
	e.backend.addCapability(CapabilityRayQueryKHR)
	e.backend.addExtension("SPV_KHR_ray_query")

	queryID, err := e.emitPointerExpression(expr.Query)
	if err != nil {
		return 0, fmt.Errorf("ray query get intersection: %w", err)
	}

	trackers, ok := e.rayQueryTrackers[expr.Query]
	if !ok {
		return 0, fmt.Errorf("ray query get intersection: no tracker found for query expression %d", expr.Query)
	}

	funcID := e.backend.writeRayQueryGetIntersection(expr.Committed)

	// Find RayIntersection type
	var riTypeID uint32
	if e.backend.module.SpecialTypes.RayIntersection != nil {
		riTypeID, err = e.backend.emitType(*e.backend.module.SpecialTypes.RayIntersection)
		if err != nil {
			return 0, fmt.Errorf("RayIntersection type: %w", err)
		}
	}

	resultID := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(riTypeID)
	ib.AddWord(resultID)
	ib.AddWord(funcID)
	ib.AddWord(queryID)
	ib.AddWord(trackers.initializedTracker)
	e.backend.builder.funcAppend(ib.Build(OpFunctionCall))

	return resultID, nil
}

// SPIR-V ray query opcodes.
const (
	OpRayQueryInitializeKHR                                            OpCode     = 4473
	OpRayQueryTerminateKHR                                             OpCode     = 4474
	OpRayQueryGenerateIntersectionKHR                                  OpCode     = 4475
	OpRayQueryConfirmIntersectionKHR                                   OpCode     = 4476
	OpRayQueryProceedKHR                                               OpCode     = 4477
	OpRayQueryGetIntersectionTypeKHR                                   OpCode     = 4479
	OpRayQueryGetRayTMinKHR                                            OpCode     = 6016
	OpRayQueryGetRayFlagsKHR                                           OpCode     = 6017
	OpRayQueryGetIntersectionTKHR                                      OpCode     = 6018
	OpRayQueryGetIntersectionInstanceCustomIndexKHR                    OpCode     = 6019
	OpRayQueryGetIntersectionInstanceIdKHR                             OpCode     = 6020
	OpRayQueryGetIntersectionInstanceShaderBindingTableRecordOffsetKHR OpCode     = 6021
	OpRayQueryGetIntersectionGeometryIndexKHR                          OpCode     = 6022
	OpRayQueryGetIntersectionPrimitiveIndexKHR                         OpCode     = 6023
	OpRayQueryGetIntersectionBarycentricsKHR                           OpCode     = 6024
	OpRayQueryGetIntersectionFrontFaceKHR                              OpCode     = 6025
	OpRayQueryGetIntersectionObjectToWorldKHR                          OpCode     = 6031
	OpRayQueryGetIntersectionWorldToObjectKHR                          OpCode     = 6032
	CapabilityRayQueryKHR                                              Capability = 4472
	RayFlagsNone                                                       uint32     = 0
)

// emitRayQuery emits a ray query statement to SPIR-V.
// Requires SPV_KHR_ray_query extension and RayQueryKHR capability.
// Ray query operations are emitted as calls to generated helper functions.
func (e *ExpressionEmitter) emitRayQuery(stmt ir.StmtRayQuery) error {
	e.backend.addCapability(CapabilityRayQueryKHR)
	e.backend.addExtension("SPV_KHR_ray_query")

	queryID, err := e.emitPointerExpression(stmt.Query)
	if err != nil {
		return fmt.Errorf("ray query expression: %w", err)
	}

	trackers, ok := e.rayQueryTrackers[stmt.Query]
	if !ok {
		return fmt.Errorf("ray query: no tracker found for query expression %d", stmt.Query)
	}

	voidTypeID := e.backend.getVoidType()

	switch fun := stmt.Fun.(type) {
	case ir.RayQueryInitialize:
		accelID, err := e.emitExpression(fun.AccelerationStructure)
		if err != nil {
			return fmt.Errorf("ray query acceleration structure: %w", err)
		}
		descID, err := e.emitExpression(fun.Descriptor)
		if err != nil {
			return fmt.Errorf("ray query descriptor: %w", err)
		}

		helperFuncID := e.backend.writeRayQueryInitialize()

		callResultID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(voidTypeID)
		ib.AddWord(callResultID)
		ib.AddWord(helperFuncID)
		ib.AddWord(queryID)
		ib.AddWord(accelID)
		ib.AddWord(descID)
		ib.AddWord(trackers.initializedTracker)
		ib.AddWord(trackers.tMaxTracker)
		e.backend.builder.funcAppend(ib.Build(OpFunctionCall))

	case ir.RayQueryProceed:
		boolTypeID := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
		helperFuncID := e.backend.writeRayQueryProceed()

		resultID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(boolTypeID)
		ib.AddWord(resultID)
		ib.AddWord(helperFuncID)
		ib.AddWord(queryID)
		ib.AddWord(trackers.initializedTracker)
		e.backend.builder.funcAppend(ib.Build(OpFunctionCall))
		e.exprIDs[fun.Result] = resultID

	case ir.RayQueryTerminate:
		// Terminate is a no-op in SPIR-V with init tracking
		// (matching Rust naga: RayQueryFunction::Terminate => {})

	case ir.RayQueryGenerateIntersection:
		hitTID, err := e.emitExpression(fun.HitT)
		if err != nil {
			return fmt.Errorf("ray query generate intersection hit_t: %w", err)
		}

		helperFuncID := e.backend.writeRayQueryGenerateIntersection()

		callResultID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(voidTypeID)
		ib.AddWord(callResultID)
		ib.AddWord(helperFuncID)
		ib.AddWord(queryID)
		ib.AddWord(trackers.initializedTracker)
		ib.AddWord(hitTID)
		ib.AddWord(trackers.tMaxTracker)
		e.backend.builder.funcAppend(ib.Build(OpFunctionCall))

	case ir.RayQueryConfirmIntersection:
		helperFuncID := e.backend.writeRayQueryConfirmIntersection()

		callResultID := e.backend.builder.AllocID()
		ib := e.newIB()
		ib.AddWord(voidTypeID)
		ib.AddWord(callResultID)
		ib.AddWord(helperFuncID)
		ib.AddWord(queryID)
		ib.AddWord(trackers.initializedTracker)
		e.backend.builder.funcAppend(ib.Build(OpFunctionCall))

	default:
		return fmt.Errorf("unsupported ray query function: %T", stmt.Fun)
	}

	return nil
}

// emitModfStructType emits the SPIR-V struct type for ModfStruct results.
// ModfStruct returns struct{fract: T, whole: T} where T matches the argument type.
func (b *Backend) emitModfStructType(argType ir.TypeResolution) uint32 {
	memberType := b.resolveTypeResolution(argType)
	return b.builder.AddTypeStruct(memberType, memberType)
}

// emitFrexpStructType emits the SPIR-V struct type for FrexpStruct results.
// FrexpStruct returns struct{fract: T, exp: intT} where intT has same width/size as T.
func (b *Backend) emitFrexpStructType(argType ir.TypeResolution) uint32 {
	floatType := b.resolveTypeResolution(argType)

	// Determine the integer type matching the arg's structure
	inner := ir.TypeResInner(b.module, argType)
	var intType uint32
	switch t := inner.(type) {
	case ir.ScalarType:
		intType = b.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	case ir.VectorType:
		intScalarType := b.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
		intType = b.emitVectorType(intScalarType, uint32(t.Size))
	default:
		intType = b.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	}

	return b.builder.AddTypeStruct(floatType, intType)
}

// float32ToF16Bits converts a float32 value to IEEE 754 half-precision (float16)
// bit representation stored in the low 16 bits of a uint32.
// Matches Rust's half::f16::from_f32().to_bits() behavior.
func float32ToF16Bits(f float32) uint32 {
	bits := math.Float32bits(f)
	sign := (bits >> 16) & 0x8000
	exp := int((bits>>23)&0xFF) - 127
	frac := bits & 0x7FFFFF

	switch {
	case exp == 128: // inf or NaN
		if frac != 0 {
			// NaN: preserve some mantissa bits
			return uint32(sign | 0x7C00 | (frac >> 13))
		}
		return uint32(sign | 0x7C00) // inf
	case exp > 15:
		// Overflow → infinity
		return uint32(sign | 0x7C00)
	case exp > -15:
		// Normal range for f16
		// Round to nearest even
		f16Frac := frac >> 13
		remainder := frac & 0x1FFF
		if remainder > 0x1000 || (remainder == 0x1000 && f16Frac&1 != 0) {
			f16Frac++
			if f16Frac >= 0x400 {
				f16Frac = 0
				exp++
				if exp > 15 {
					return uint32(sign | 0x7C00)
				}
			}
		}
		return uint32(sign | uint32(exp+15)<<10 | f16Frac)
	case exp >= -24:
		// Subnormal
		shift := uint(-14 - exp)
		f16Frac := (frac | 0x800000) >> (shift + 13)
		// Round
		remainder := (frac | 0x800000) >> shift & 0x1FFF
		if remainder > 0x1000 || (remainder == 0x1000 && f16Frac&1 != 0) {
			f16Frac++
		}
		return uint32(sign | f16Frac)
	default:
		// Too small → zero
		return uint32(sign)
	}
}
