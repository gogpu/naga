// Package ir defines the intermediate representation for naga.
//
// The IR is a shader-agnostic representation that can be translated
// from various source languages (WGSL, GLSL) and compiled to
// various target languages (SPIR-V, GLSL, MSL, HLSL).
package ir

// Module represents a shader module in IR form.
// Module is the IR representation of a shader module.
// Structurally verified against Rust naga via TestIRReference (18/18 deep match).
// See docs/dev/research/IR-DEEP-ANALYSIS.md for Go vs Rust comparison.
//
// Key architectural difference from Rust naga:
// - Entry point functions are inline in EntryPoint.Function (not in Functions[])
// - Go slices instead of Rust Arena<T> — cache-friendly, GC-managed
// - Const array inlining is 1-step (Rust: 3-step create→evaluate→compact)
type Module struct {
	// Types holds all type definitions. Order matches Rust naga's type arena.
	Types []Type

	// Constants holds module-scope `const` declarations (NOT overrides).
	// Rust naga also separates constants from overrides.
	Constants []Constant

	// GlobalVariables holds module-scope variables (var<storage>, var<uniform>, etc.)
	GlobalVariables []GlobalVariable

	// GlobalExpressions holds expressions used at module scope:
	// Constant.Init, Override.Init, and GlobalVariable.Init reference into this.
	// Mirrors Rust naga's Module.global_expressions arena.
	GlobalExpressions []Expression

	// Functions holds regular (non-entry-point) function definitions.
	// Entry point functions are NOT here — they're inline in EntryPoints[].Function.
	Functions []Function

	// EntryPoints holds shader entry points with inline Function bodies.
	// Unlike Rust naga which uses FunctionHandle into functions arena,
	// our entry points contain the full Function struct inline.
	EntryPoints []EntryPoint

	// Overrides holds pipeline-overridable constants (WGSL `override` declarations).
	// Separate from Constants — mirrors Rust naga's Module.overrides arena.
	Overrides []Override

	// SpecialTypes holds handles to compiler-generated types (external textures, etc.)
	SpecialTypes SpecialTypes

	// TypeAliasNames records names from type alias declarations so the namer can register them and detect collisions with variables sharing the same name.
	TypeAliasNames []string

	// TypeUseOrder records the order in which types were first registered
	// during lowering. Used by ReorderTypes to reorder the type arena
	// to match Rust naga's dependency-ordered type registration.
	TypeUseOrder []TypeHandle
}

// SpecialTypes holds handles to compiler-generated types used by backends.
// Mirrors Rust naga's SpecialTypes struct.
type SpecialTypes struct {
	// ExternalTextureParams is the handle of the NagaExternalTextureParams struct type.
	ExternalTextureParams *TypeHandle

	// ExternalTextureTransferFunction is the handle of the NagaExternalTextureTransferFn struct type.
	ExternalTextureTransferFunction *TypeHandle

	// RayIntersection is the handle of the RayIntersection struct type used by ray query get intersection expressions. Mirrors Rust naga SpecialTypes ray_intersection.
	RayIntersection *TypeHandle
}

// Override represents a pipeline-overridable constant.
// Mirrors Rust naga's Override struct.
type Override struct {
	// Name is the identifier name of the override.
	Name string
	// ID is the numeric @id attribute value, if specified.
	// In Rust naga this is Option<u16>; we use *uint16 for nil-ability.
	ID *uint16
	// Ty is the type of this override (handle into Module.Types).
	Ty TypeHandle
	// Init is an optional handle into Module.GlobalExpressions that holds the
	// default value expression. None if the override has no default.
	Init *ExpressionHandle
}

// OverrideInitExpr represents a simplified expression for override init re-evaluation.
// Used during pipeline constant processing to re-evaluate derived overrides.
type OverrideInitExpr interface {
	overrideInitExpr()
}

// OverrideInitLiteral represents a literal float value.
type OverrideInitLiteral struct {
	Value float64
}

func (OverrideInitLiteral) overrideInitExpr() {}

// OverrideInitRef represents a reference to another override.
type OverrideInitRef struct {
	Handle OverrideHandle
}

func (OverrideInitRef) overrideInitExpr() {}

// OverrideInitBinary represents a binary operation on two override init expressions.
type OverrideInitBinary struct {
	Op    BinaryOperator
	Left  OverrideInitExpr
	Right OverrideInitExpr
}

func (OverrideInitBinary) overrideInitExpr() {}

// OverrideInitUnary represents a unary operation on an override init expression.
type OverrideInitUnary struct {
	Op   UnaryOperator
	Expr OverrideInitExpr
}

func (OverrideInitUnary) overrideInitExpr() {}

// OverrideInitBoolLiteral represents a literal bool value for override init.
type OverrideInitBoolLiteral struct {
	Value bool
}

func (OverrideInitBoolLiteral) overrideInitExpr() {}

// OverrideInitUintLiteral represents a literal uint value for override init.
type OverrideInitUintLiteral struct {
	Value uint32
}

func (OverrideInitUintLiteral) overrideInitExpr() {}

// EntryPoint represents a shader entry point.
// The Function is stored inline (not via FunctionHandle) because Rust naga
// keeps entry-point functions separate from Module.functions[].
type EntryPoint struct {
	Name           string
	Stage          ShaderStage
	Function       Function              // Inline function (NOT in Module.Functions[])
	Workgroup      [3]uint32             // For compute/mesh/task shaders
	EarlyDepthTest *EarlyDepthTest       // For fragment shaders with early depth testing
	MeshInfo       *MeshStageInfo        // For mesh shaders
	TaskPayload    *GlobalVariableHandle // For mesh/task shaders referencing task payload variable
}

// MeshOutputTopology specifies the primitive topology for mesh shader output.
type MeshOutputTopology uint8

const (
	MeshTopologyPoints MeshOutputTopology = iota
	MeshTopologyLines
	MeshTopologyTriangles
)

// MeshStageInfo holds information specific to mesh shader entry points.
type MeshStageInfo struct {
	Topology              MeshOutputTopology
	MaxVertices           uint32
	MaxVerticesOverride   *ExpressionHandle
	MaxPrimitives         uint32
	MaxPrimitivesOverride *ExpressionHandle
	VertexOutputType      TypeHandle
	PrimitiveOutputType   TypeHandle
	OutputVariable        GlobalVariableHandle
}

// EarlyDepthTest represents early fragment test configuration.
type EarlyDepthTest struct {
	Conservative ConservativeDepth
}

// ConservativeDepth specifies how the depth value may be modified.
type ConservativeDepth uint8

const (
	// ConservativeDepthUnchanged means the depth value will not be modified.
	ConservativeDepthUnchanged ConservativeDepth = iota
	// ConservativeDepthGreaterEqual means the depth value may be increased.
	ConservativeDepthGreaterEqual
	// ConservativeDepthLessEqual means the depth value may be decreased.
	ConservativeDepthLessEqual
)

// ShaderStage represents a shader stage.
type ShaderStage uint8

const (
	StageVertex ShaderStage = iota
	StageTask
	StageMesh
	StageFragment
	StageCompute
)

// Handle types for referencing IR objects
type (
	TypeHandle           uint32
	FunctionHandle       uint32
	GlobalVariableHandle uint32
	ConstantHandle       uint32
	OverrideHandle       uint32
	ExpressionHandle     uint32
)

// Type represents a type in the IR.
type Type struct {
	Name  string
	Inner TypeInner
}

// TypeInner represents the inner type kind.
type TypeInner interface {
	typeInner()
}

// ScalarType represents scalar types.
type ScalarType struct {
	Kind  ScalarKind
	Width uint8 // in bytes
}

func (ScalarType) typeInner() {}

// ScalarKind represents scalar type kinds.
type ScalarKind uint8

const (
	ScalarSint  ScalarKind = iota // Signed integer
	ScalarUint                    // Unsigned integer
	ScalarFloat                   // Floating point
	ScalarBool                    // Boolean

	// Abstract types: used during WGSL lowering, removed by compact before backends.
	// Matches Rust naga: forbidden by validation, never reach backends.
	ScalarAbstractInt   // WGSL abstract integer (unsuffixed int literals)
	ScalarAbstractFloat // WGSL abstract float (unsuffixed float literals)
)

// VectorType represents vector types.
type VectorType struct {
	Size   VectorSize
	Scalar ScalarType
}

func (VectorType) typeInner() {}

// VectorSize represents vector sizes.
type VectorSize uint8

const (
	Vec2 VectorSize = 2
	Vec3 VectorSize = 3
	Vec4 VectorSize = 4
)

// MatrixType represents matrix types.
type MatrixType struct {
	Columns VectorSize
	Rows    VectorSize
	Scalar  ScalarType
}

func (MatrixType) typeInner() {}

// ArrayType represents array types.
type ArrayType struct {
	Base   TypeHandle
	Size   ArraySize
	Stride uint32
}

func (ArrayType) typeInner() {}

// ArraySize represents array size.
type ArraySize struct {
	Constant *uint32 // nil for runtime-sized arrays
}

// StructType represents struct types.
type StructType struct {
	Members []StructMember
	Span    uint32 // Size in bytes
}

func (StructType) typeInner() {}

// StructMember represents a struct member.
type StructMember struct {
	Name    string
	Type    TypeHandle
	Binding *Binding // @builtin(position), @location(0), etc.
	Offset  uint32
}

// PointerType represents pointer types.
type PointerType struct {
	Base  TypeHandle
	Space AddressSpace
}

func (PointerType) typeInner() {}

// ValuePointerType represents a pointer to a scalar or vector value.
// Unlike PointerType (whose Base is a TypeHandle in the arena), ValuePointerType
// stores the pointee type inline. This exists only in TypeResolution — never in
// the type arena. Matches Rust naga's TypeInner::ValuePointer.
//
// Produced by the typifier when accessing components through pointers:
//   - Pointer<Matrix>[i] → ValuePointerType{Size: &rows, Scalar, Space} (pointer to column vector)
//   - Pointer<Vector>[i] → ValuePointerType{Size: nil, Scalar, Space} (pointer to scalar)
//   - ValuePointerType{Size: &s}[i] → ValuePointerType{Size: nil, Scalar, Space} (pointer to element)
type ValuePointerType struct {
	Size   *VectorSize // nil for pointer-to-scalar, non-nil for pointer-to-vector
	Scalar ScalarType
	Space  AddressSpace
}

func (ValuePointerType) typeInner() {}

// AtomicType represents atomic types for thread-safe operations.
type AtomicType struct {
	Scalar ScalarType
}

func (AtomicType) typeInner() {}

// BindingArrayType represents a binding array type (binding_array<T, N>).
// In SPIR-V, this maps to an array of uniform resources (textures, samplers).
type BindingArrayType struct {
	Base TypeHandle
	Size *uint32 // nil for unbounded
}

func (BindingArrayType) typeInner() {}

// AccelerationStructureType represents an opaque acceleration structure for ray tracing.
// In SPIR-V, this maps to OpTypeAccelerationStructureKHR.
type AccelerationStructureType struct{}

func (AccelerationStructureType) typeInner() {}

// RayQueryType represents an opaque ray query handle for ray tracing.
// In SPIR-V, this maps to OpTypeRayQueryKHR.
type RayQueryType struct{}

func (RayQueryType) typeInner() {}

// AddressSpace represents memory address spaces.
type AddressSpace uint8

const (
	SpaceFunction AddressSpace = iota
	SpacePrivate
	SpaceWorkGroup
	SpaceUniform
	SpaceStorage
	SpacePushConstant
	SpaceHandle
	SpaceImmediate
	SpaceTaskPayload
)

// SamplerType represents sampler types.
type SamplerType struct {
	Comparison bool
}

func (SamplerType) typeInner() {}

// ImageType represents image/texture types.
type ImageType struct {
	Dim           ImageDimension
	Arrayed       bool
	Class         ImageClass
	Multisampled  bool
	SampledKind   ScalarKind    // Kind of values for sampled textures (only valid when Class == ImageClassSampled)
	StorageFormat StorageFormat // Format for storage textures (only valid when Class == ImageClassStorage)
	StorageAccess StorageAccess // Access mode for storage textures (only valid when Class == ImageClassStorage)
}

func (ImageType) typeInner() {}

// ImageDimension represents image dimensions.
type ImageDimension uint8

const (
	Dim1D ImageDimension = iota
	Dim2D
	Dim3D
	DimCube
)

// ImageClass represents image classification.
type ImageClass uint8

const (
	ImageClassSampled ImageClass = iota
	ImageClassDepth
	ImageClassExternal
	ImageClassStorage
)

// StorageFormat represents storage texture formats.
// These are the formats that can be used with storage textures in WGSL.
type StorageFormat uint8

const (
	StorageFormatUnknown StorageFormat = iota

	// 8-bit formats
	StorageFormatR8Unorm
	StorageFormatR8Snorm
	StorageFormatR8Uint
	StorageFormatR8Sint

	// 16-bit formats
	StorageFormatR16Uint
	StorageFormatR16Sint
	StorageFormatR16Float
	StorageFormatRg8Unorm
	StorageFormatRg8Snorm
	StorageFormatRg8Uint
	StorageFormatRg8Sint

	// 32-bit formats
	StorageFormatR32Uint
	StorageFormatR32Sint
	StorageFormatR32Float
	StorageFormatRg16Uint
	StorageFormatRg16Sint
	StorageFormatRg16Float
	StorageFormatRgba8Unorm
	StorageFormatRgba8Snorm
	StorageFormatRgba8Uint
	StorageFormatRgba8Sint
	StorageFormatBgra8Unorm

	// Packed 32-bit formats
	StorageFormatRgb10a2Uint
	StorageFormatRgb10a2Unorm
	StorageFormatRg11b10Ufloat

	// 64-bit formats
	StorageFormatRg32Uint
	StorageFormatRg32Sint
	StorageFormatRg32Float
	StorageFormatRgba16Uint
	StorageFormatRgba16Sint
	StorageFormatRgba16Float

	// 128-bit formats
	StorageFormatRgba32Uint
	StorageFormatRgba32Sint
	StorageFormatRgba32Float

	// Normalized 16-bit per channel formats
	StorageFormatR16Unorm
	StorageFormatR16Snorm
	StorageFormatRg16Unorm
	StorageFormatRg16Snorm
	StorageFormatRgba16Unorm
	StorageFormatRgba16Snorm

	// 64-bit storage formats (require Metal 3.1 for atomic textures)
	StorageFormatR64Uint
	StorageFormatR64Sint
)

// ScalarKind returns the scalar kind associated with this storage format.
// Unorm/Snorm/Float formats return ScalarFloat, Uint formats return ScalarUint,
// Sint formats return ScalarSint.
func (f StorageFormat) ScalarKind() ScalarKind {
	switch f {
	case StorageFormatR8Uint, StorageFormatR16Uint, StorageFormatRg8Uint,
		StorageFormatR32Uint, StorageFormatRg16Uint, StorageFormatRgba8Uint,
		StorageFormatRgb10a2Uint, StorageFormatRg32Uint, StorageFormatRgba16Uint,
		StorageFormatRgba32Uint, StorageFormatR64Uint:
		return ScalarUint
	case StorageFormatR8Sint, StorageFormatR16Sint, StorageFormatRg8Sint,
		StorageFormatR32Sint, StorageFormatRg16Sint, StorageFormatRgba8Sint,
		StorageFormatRg32Sint, StorageFormatRgba16Sint, StorageFormatRgba32Sint,
		StorageFormatR64Sint:
		return ScalarSint
	default:
		// All unorm, snorm, float, ufloat formats
		return ScalarFloat
	}
}

// StorageAccess represents access modes for storage textures.
type StorageAccess uint8

const (
	StorageAccessRead StorageAccess = iota
	StorageAccessWrite
	StorageAccessReadWrite
	StorageAccessAtomic
)

// Constant represents a constant value.
type Constant struct {
	Name  string
	Type  TypeHandle
	Value ConstantValue

	// Init is a handle into Module.GlobalExpressions that holds the init expression
	// for this constant. This mirrors Rust naga's Constant.init field.
	// When GlobalExpressions is populated, this is the canonical init reference.
	Init ExpressionHandle

	// IsAbstract indicates this constant originated from a WGSL `const` declaration
	// without an explicit type (e.g., `const ONE = 1;`). In Rust naga, such constants
	// retain abstract types and are removed by the compact pass before reaching backends.
	// The MSL writer should skip abstract constants.
	IsAbstract bool
}

// ConstantValue represents constant values.
type ConstantValue interface {
	constantValue()
}

// ScalarValue represents a scalar constant.
type ScalarValue struct {
	Bits uint64 // Bit representation
	Kind ScalarKind
}

func (ScalarValue) constantValue() {}

// CompositeValue represents a composite constant.
type CompositeValue struct {
	Components []ConstantHandle
}

func (CompositeValue) constantValue() {}

// ZeroConstantValue represents a zero-initialized constant.
// In MSL, this renders as "type {}" (brace initialization).
// Matches Rust naga's use of ZeroValue for constant init expressions.
type ZeroConstantValue struct{}

func (ZeroConstantValue) constantValue() {}

// StorageAccessMode represents access modes for storage buffers.
// This is separate from StorageAccess (used for storage textures).
type StorageAccessMode uint8

const (
	// StorageReadWrite indicates read-write access (default for storage buffers).
	StorageReadWrite StorageAccessMode = iota
	// StorageRead indicates read-only access.
	StorageRead
)

// GlobalVariable represents a global variable.
type GlobalVariable struct {
	Name    string
	Space   AddressSpace
	Binding *ResourceBinding
	Type    TypeHandle
	Init    *ConstantHandle
	// InitExpr is an optional handle into Module.GlobalExpressions for the
	// init expression. This mirrors Rust naga's GlobalVariable.init field.
	// When set, this is the canonical init reference into GlobalExpressions.
	InitExpr *ExpressionHandle
	// Access stores the access mode for storage address space variables.
	// Only meaningful when Space == SpaceStorage.
	// Rust naga: Storage { access: StorageAccess::LOAD } vs Storage { access: StorageAccess::LOAD | StorageAccess::STORE }.
	Access StorageAccessMode
}

// ResourceBinding represents a resource binding.
type ResourceBinding struct {
	Group   uint32
	Binding uint32
}

// Function represents a function definition.
type Function struct {
	Name            string
	Arguments       []FunctionArgument
	Result          *FunctionResult
	LocalVars       []LocalVariable
	Expressions     []Expression
	ExpressionTypes []TypeResolution // Type of each expression (parallel to Expressions)
	Body            []Statement

	// NamedExpressions maps expression handles to user-given names.
	// This is used for let bindings and phony assignments (_ = expr).
	// Backends use these names when baking (materializing) expressions,
	// producing e.g. "float a = ..." instead of "float _e3 = ...".
	// Matches Rust naga's Function::named_expressions.
	NamedExpressions map[ExpressionHandle]string
}

// FunctionArgument represents a function argument.
type FunctionArgument struct {
	Name    string
	Type    TypeHandle
	Binding *Binding
}

// FunctionResult represents a function return type.
type FunctionResult struct {
	Type    TypeHandle
	Binding *Binding
}

// LocalVariable represents a function-local variable.
type LocalVariable struct {
	Name string
	Type TypeHandle
	Init *ExpressionHandle
}

// Binding represents shader bindings.
type Binding interface {
	binding()
}

// BuiltinBinding represents a built-in binding.
type BuiltinBinding struct {
	Builtin   BuiltinValue
	Invariant bool // only meaningful for Position built-in
}

func (BuiltinBinding) binding() {}

// BuiltinValue represents built-in values.
type BuiltinValue uint8

const (
	BuiltinPosition BuiltinValue = iota
	BuiltinVertexIndex
	BuiltinInstanceIndex
	BuiltinFrontFacing
	BuiltinFragDepth
	BuiltinSampleIndex
	BuiltinSampleMask
	BuiltinLocalInvocationID
	BuiltinLocalInvocationIndex
	BuiltinGlobalInvocationID
	BuiltinWorkGroupID
	BuiltinNumWorkGroups
	BuiltinNumSubgroups
	BuiltinSubgroupID
	BuiltinSubgroupSize
	BuiltinSubgroupInvocationID
	BuiltinBarycentric
	BuiltinViewIndex
	BuiltinPrimitiveIndex
	BuiltinPointSize
	BuiltinMeshTaskSize
	BuiltinCullPrimitive
	BuiltinPointIndex
	BuiltinLineIndices
	BuiltinTriangleIndices
	BuiltinVertexCount
	BuiltinVertices
	BuiltinPrimitiveCount
	BuiltinPrimitives
	BuiltinClipDistance
)

// LocationBinding represents a location binding.
type LocationBinding struct {
	Location      uint32
	Interpolation *Interpolation
	// BlendSrc is the dual-source blending index (@blend_src attribute).
	// Nil when not using dual-source blending.
	BlendSrc *uint32
}

func (LocationBinding) binding() {}

// Interpolation represents interpolation settings.
type Interpolation struct {
	Kind     InterpolationKind
	Sampling InterpolationSampling
}

// InterpolationKind represents interpolation kinds.
type InterpolationKind uint8

const (
	InterpolationFlat InterpolationKind = iota
	InterpolationLinear
	InterpolationPerspective
)

// InterpolationSampling represents interpolation sampling.
type InterpolationSampling uint8

const (
	SamplingCenter InterpolationSampling = iota
	SamplingCentroid
	SamplingSample
)

// TypeResolution represents the resolved type of an expression.
// It can either reference a type in the module's type arena (Handle)
// or represent an inline/computed type (Value).
type TypeResolution struct {
	Handle *TypeHandle // If set, references a module type
	Value  TypeInner   // If Handle is nil, this is the inline type
}

// Expression types are defined in expression.go
// Statement types are defined in statement.go
