// Package ir defines the intermediate representation for naga.
//
// The IR is a shader-agnostic representation that can be translated
// from various source languages (WGSL, GLSL) and compiled to
// various target languages (SPIR-V, GLSL, MSL, HLSL).
package ir

// Module represents a shader module in IR form.
type Module struct {
	// Types holds all type definitions
	Types []Type

	// Constants holds module-scope constants
	Constants []Constant

	// GlobalVariables holds module-scope variables
	GlobalVariables []GlobalVariable

	// Functions holds all function definitions
	Functions []Function

	// EntryPoints holds shader entry points
	EntryPoints []EntryPoint
}

// EntryPoint represents a shader entry point.
type EntryPoint struct {
	Name      string
	Stage     ShaderStage
	Function  FunctionHandle
	Workgroup [3]uint32 // For compute shaders
}

// ShaderStage represents a shader stage.
type ShaderStage uint8

const (
	StageVertex ShaderStage = iota
	StageFragment
	StageCompute
)

// Handle types for referencing IR objects
type (
	TypeHandle           uint32
	FunctionHandle       uint32
	GlobalVariableHandle uint32
	ConstantHandle       uint32
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

// AtomicType represents atomic types for thread-safe operations.
type AtomicType struct {
	Scalar ScalarType
}

func (AtomicType) typeInner() {}

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
)

// SamplerType represents sampler types.
type SamplerType struct {
	Comparison bool
}

func (SamplerType) typeInner() {}

// ImageType represents image/texture types.
type ImageType struct {
	Dim          ImageDimension
	Arrayed      bool
	Class        ImageClass
	Multisampled bool
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
	ImageClassStorage
)

// Constant represents a constant value.
type Constant struct {
	Name  string
	Type  TypeHandle
	Value ConstantValue
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

// GlobalVariable represents a global variable.
type GlobalVariable struct {
	Name    string
	Space   AddressSpace
	Binding *ResourceBinding
	Type    TypeHandle
	Init    *ConstantHandle
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
	Builtin BuiltinValue
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
)

// LocationBinding represents a location binding.
type LocationBinding struct {
	Location      uint32
	Interpolation *Interpolation
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
