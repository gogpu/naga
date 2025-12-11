package ir

// Expression represents an expression in the IR.
// Expressions follow Single Static Assignment (SSA) form similar to SPIR-V.
type Expression struct {
	Kind ExpressionKind
}

// ExpressionKind represents the different kinds of expressions.
type ExpressionKind interface {
	expressionKind()
}

// Literal represents a literal constant value.
type Literal struct {
	Value LiteralValue
}

func (Literal) expressionKind() {}

// LiteralValue represents the value of a literal.
type LiteralValue interface {
	literalValue()
}

// LiteralF64 represents a 64-bit float literal (may not be NaN or infinity).
type LiteralF64 float64

func (LiteralF64) literalValue() {}

// LiteralF32 represents a 32-bit float literal (may not be NaN or infinity).
type LiteralF32 float32

func (LiteralF32) literalValue() {}

// LiteralU32 represents a 32-bit unsigned integer literal.
type LiteralU32 uint32

func (LiteralU32) literalValue() {}

// LiteralI32 represents a 32-bit signed integer literal.
type LiteralI32 int32

func (LiteralI32) literalValue() {}

// LiteralU64 represents a 64-bit unsigned integer literal.
type LiteralU64 uint64

func (LiteralU64) literalValue() {}

// LiteralI64 represents a 64-bit signed integer literal.
type LiteralI64 int64

func (LiteralI64) literalValue() {}

// LiteralBool represents a boolean literal.
type LiteralBool bool

func (LiteralBool) literalValue() {}

// LiteralAbstractInt represents an abstract integer literal.
type LiteralAbstractInt int64

func (LiteralAbstractInt) literalValue() {}

// LiteralAbstractFloat represents an abstract float literal.
type LiteralAbstractFloat float64

func (LiteralAbstractFloat) literalValue() {}

// ExprConstant references a module-scope constant.
type ExprConstant struct {
	Constant ConstantHandle
}

func (ExprConstant) expressionKind() {}

// ExprZeroValue represents a zero-initialized value of a given type.
type ExprZeroValue struct {
	Type TypeHandle
}

func (ExprZeroValue) expressionKind() {}

// ExprCompose constructs a composite value (vector, matrix, array, or struct).
type ExprCompose struct {
	Type       TypeHandle
	Components []ExpressionHandle
}

func (ExprCompose) expressionKind() {}

// ExprAccess performs array/vector/matrix access with a computed index.
// The index operand must be an integer type (signed or unsigned).
type ExprAccess struct {
	Base  ExpressionHandle
	Index ExpressionHandle
}

func (ExprAccess) expressionKind() {}

// ExprAccessIndex performs access with a compile-time constant index.
// Can access arrays, vectors, matrices, and struct fields.
type ExprAccessIndex struct {
	Base  ExpressionHandle
	Index uint32
}

func (ExprAccessIndex) expressionKind() {}

// ExprSplat broadcasts a scalar value to all components of a vector.
type ExprSplat struct {
	Size  VectorSize
	Value ExpressionHandle
}

func (ExprSplat) expressionKind() {}

// ExprSwizzle reorders or duplicates vector components.
type ExprSwizzle struct {
	Size    VectorSize
	Vector  ExpressionHandle
	Pattern [4]SwizzleComponent
}

func (ExprSwizzle) expressionKind() {}

// SwizzleComponent represents a single component in a vector swizzle.
type SwizzleComponent uint8

const (
	SwizzleX SwizzleComponent = 0
	SwizzleY SwizzleComponent = 1
	SwizzleZ SwizzleComponent = 2
	SwizzleW SwizzleComponent = 3
)

// ExprFunctionArgument references a function parameter by its index.
type ExprFunctionArgument struct {
	Index uint32
}

func (ExprFunctionArgument) expressionKind() {}

// ExprGlobalVariable references a global variable.
// For handle address space, produces the variable's value directly.
// For other address spaces, produces a pointer to the variable.
type ExprGlobalVariable struct {
	Variable GlobalVariableHandle
}

func (ExprGlobalVariable) expressionKind() {}

// ExprLocalVariable references a local variable.
// Produces a pointer to the variable's value.
type ExprLocalVariable struct {
	Variable uint32 // Index into Function.LocalVars
}

func (ExprLocalVariable) expressionKind() {}

// ExprLoad loads a value indirectly through a pointer.
type ExprLoad struct {
	Pointer ExpressionHandle
}

func (ExprLoad) expressionKind() {}

// ExprImageSample samples a point from a sampled or depth image.
type ExprImageSample struct {
	Image       ExpressionHandle
	Sampler     ExpressionHandle
	Gather      *SwizzleComponent // If set, perform a gather operation
	Coordinate  ExpressionHandle
	ArrayIndex  *ExpressionHandle
	Offset      *ExpressionHandle // Must be a const-expression
	Level       SampleLevel
	DepthRef    *ExpressionHandle
	ClampToEdge bool // Clamp coordinates to [half_texel, 1 - half_texel]
}

func (ExprImageSample) expressionKind() {}

// SampleLevel controls the level of detail for texture sampling.
type SampleLevel interface {
	sampleLevel()
}

// SampleLevelAuto uses automatic level of detail.
type SampleLevelAuto struct{}

func (SampleLevelAuto) sampleLevel() {}

// SampleLevelZero uses mipmap level 0.
type SampleLevelZero struct{}

func (SampleLevelZero) sampleLevel() {}

// SampleLevelExact uses an explicit level of detail.
type SampleLevelExact struct {
	Level ExpressionHandle
}

func (SampleLevelExact) sampleLevel() {}

// SampleLevelBias uses automatic level of detail with a bias.
type SampleLevelBias struct {
	Bias ExpressionHandle
}

func (SampleLevelBias) sampleLevel() {}

// SampleLevelGradient uses explicit gradients for level of detail.
type SampleLevelGradient struct {
	X ExpressionHandle
	Y ExpressionHandle
}

func (SampleLevelGradient) sampleLevel() {}

// ExprImageLoad loads a texel from an image.
type ExprImageLoad struct {
	Image      ExpressionHandle
	Coordinate ExpressionHandle
	ArrayIndex *ExpressionHandle
	Sample     *ExpressionHandle // For multisampled images
	Level      *ExpressionHandle // For mipmapped images
}

func (ExprImageLoad) expressionKind() {}

// ExprImageQuery queries information from an image.
type ExprImageQuery struct {
	Image ExpressionHandle
	Query ImageQuery
}

func (ExprImageQuery) expressionKind() {}

// ImageQuery represents the type of image query.
type ImageQuery interface {
	imageQuery()
}

// ImageQuerySize gets the image size at a specified level.
type ImageQuerySize struct {
	Level *ExpressionHandle // If nil, uses base level
}

func (ImageQuerySize) imageQuery() {}

// ImageQueryNumLevels gets the number of mipmap levels.
type ImageQueryNumLevels struct{}

func (ImageQueryNumLevels) imageQuery() {}

// ImageQueryNumLayers gets the number of array layers.
type ImageQueryNumLayers struct{}

func (ImageQueryNumLayers) imageQuery() {}

// ImageQueryNumSamples gets the number of samples.
type ImageQueryNumSamples struct{}

func (ImageQueryNumSamples) imageQuery() {}

// ExprUnary applies a unary operator to an expression.
type ExprUnary struct {
	Op   UnaryOperator
	Expr ExpressionHandle
}

func (ExprUnary) expressionKind() {}

// UnaryOperator represents unary operations.
type UnaryOperator uint8

const (
	UnaryNegate     UnaryOperator = iota // Arithmetic negation
	UnaryLogicalNot                      // Logical not (!)
	UnaryBitwiseNot                      // Bitwise not (~)
)

// ExprBinary applies a binary operator to two expressions.
type ExprBinary struct {
	Op    BinaryOperator
	Left  ExpressionHandle
	Right ExpressionHandle
}

func (ExprBinary) expressionKind() {}

// BinaryOperator represents binary operations.
type BinaryOperator uint8

const (
	// Arithmetic operations
	BinaryAdd      BinaryOperator = iota // Addition
	BinarySubtract                       // Subtraction
	BinaryMultiply                       // Multiplication
	BinaryDivide                         // Division
	BinaryModulo                         // Modulo (remainder)

	// Comparison operations
	BinaryEqual        // Equal (==)
	BinaryNotEqual     // Not equal (!=)
	BinaryLess         // Less than (<)
	BinaryLessEqual    // Less than or equal (<=)
	BinaryGreater      // Greater than (>)
	BinaryGreaterEqual // Greater than or equal (>=)

	// Bitwise operations
	BinaryAnd         // Bitwise AND
	BinaryExclusiveOr // Bitwise XOR
	BinaryInclusiveOr // Bitwise OR

	// Logical operations
	BinaryLogicalAnd // Logical AND (&&)
	BinaryLogicalOr  // Logical OR (||)

	// Shift operations
	BinaryShiftLeft  // Left shift (<<)
	BinaryShiftRight // Right shift (>>) - arithmetic for signed, logical for unsigned
)

// ExprSelect selects between two values based on a boolean condition.
// Equivalent to the ternary operator (condition ? accept : reject).
type ExprSelect struct {
	Condition ExpressionHandle
	Accept    ExpressionHandle
	Reject    ExpressionHandle
}

func (ExprSelect) expressionKind() {}

// ExprDerivative computes the derivative of an expression.
type ExprDerivative struct {
	Axis    DerivativeAxis
	Control DerivativeControl
	Expr    ExpressionHandle
}

func (ExprDerivative) expressionKind() {}

// DerivativeAxis specifies the axis for derivative computation.
type DerivativeAxis uint8

const (
	DerivativeX     DerivativeAxis = iota // Partial derivative with respect to X
	DerivativeY                           // Partial derivative with respect to Y
	DerivativeWidth                       // Sum of absolute derivatives (fwidth)
)

// DerivativeControl specifies the precision hint for derivative computation.
type DerivativeControl uint8

const (
	DerivativeCoarse DerivativeControl = iota // Coarse precision
	DerivativeFine                            // Fine precision
	DerivativeNone                            // No specific precision
)

// ExprRelational applies a relational function.
type ExprRelational struct {
	Fun      RelationalFunction
	Argument ExpressionHandle
}

func (ExprRelational) expressionKind() {}

// RelationalFunction represents built-in relational test functions.
type RelationalFunction uint8

const (
	RelationalAll   RelationalFunction = iota // All components are true
	RelationalAny                             // Any component is true
	RelationalIsNan                           // Test for NaN
	RelationalIsInf                           // Test for infinity
)

// ExprMath applies a mathematical function.
type ExprMath struct {
	Fun  MathFunction
	Arg  ExpressionHandle
	Arg1 *ExpressionHandle
	Arg2 *ExpressionHandle
	Arg3 *ExpressionHandle
}

func (ExprMath) expressionKind() {}

// MathFunction represents built-in mathematical functions.
type MathFunction uint8

const (
	// Comparison functions
	MathAbs      MathFunction = iota // Absolute value
	MathMin                          // Minimum
	MathMax                          // Maximum
	MathClamp                        // Clamp to range
	MathSaturate                     // Clamp to [0, 1]

	// Trigonometric functions
	MathCos   // Cosine
	MathCosh  // Hyperbolic cosine
	MathSin   // Sine
	MathSinh  // Hyperbolic sine
	MathTan   // Tangent
	MathTanh  // Hyperbolic tangent
	MathAcos  // Arc cosine
	MathAsin  // Arc sine
	MathAtan  // Arc tangent
	MathAtan2 // Two-argument arc tangent
	MathAsinh // Inverse hyperbolic sine
	MathAcosh // Inverse hyperbolic cosine
	MathAtanh // Inverse hyperbolic tangent

	// Angle conversion
	MathRadians // Convert degrees to radians
	MathDegrees // Convert radians to degrees

	// Decomposition functions
	MathCeil  // Round up to integer
	MathFloor // Round down to integer
	MathRound // Round to nearest integer
	MathFract // Fractional part
	MathTrunc // Truncate to integer
	MathModf  // Split into integer and fractional parts
	MathFrexp // Split into mantissa and exponent
	MathLdexp // Combine mantissa and exponent

	// Exponential functions
	MathExp  // Natural exponential (e^x)
	MathExp2 // Base-2 exponential (2^x)
	MathLog  // Natural logarithm
	MathLog2 // Base-2 logarithm
	MathPow  // Power (x^y)

	// Geometric functions
	MathDot          // Dot product
	MathDot4I8Packed // Dot product of packed 4xi8
	MathDot4U8Packed // Dot product of packed 4xu8
	MathOuter        // Outer product
	MathCross        // Cross product
	MathDistance     // Distance between points
	MathLength       // Vector length
	MathNormalize    // Normalize vector
	MathFaceForward  // Orient vector
	MathReflect      // Reflect vector
	MathRefract      // Refract vector

	// Computational functions
	MathSign        // Sign of value (-1, 0, or 1)
	MathFma         // Fused multiply-add
	MathMix         // Linear interpolation
	MathStep        // Step function
	MathSmoothStep  // Smooth step function
	MathSqrt        // Square root
	MathInverseSqrt // Inverse square root
	MathInverse     // Matrix inverse
	MathTranspose   // Matrix transpose
	MathDeterminant // Matrix determinant
	MathQuantizeF16 // Round to 16-bit float precision

	// Bit manipulation functions
	MathCountTrailingZeros // Count trailing zero bits
	MathCountLeadingZeros  // Count leading zero bits
	MathCountOneBits       // Count one bits
	MathReverseBits        // Reverse bit order
	MathExtractBits        // Extract bit range
	MathInsertBits         // Insert bit range
	MathFirstTrailingBit   // Find first trailing one bit
	MathFirstLeadingBit    // Find first leading one bit

	// Data packing functions
	MathPack4x8snorm  // Pack 4 normalized signed floats to bytes
	MathPack4x8unorm  // Pack 4 normalized unsigned floats to bytes
	MathPack2x16snorm // Pack 2 normalized signed floats to shorts
	MathPack2x16unorm // Pack 2 normalized unsigned floats to shorts
	MathPack2x16float // Pack 2 floats to half-precision shorts
	MathPack4xI8      // Pack 4 signed ints to bytes
	MathPack4xU8      // Pack 4 unsigned ints to bytes
	MathPack4xI8Clamp // Pack 4 signed ints to bytes with clamping
	MathPack4xU8Clamp // Pack 4 unsigned ints to bytes with clamping

	// Data unpacking functions
	MathUnpack4x8snorm  // Unpack bytes to 4 normalized signed floats
	MathUnpack4x8unorm  // Unpack bytes to 4 normalized unsigned floats
	MathUnpack2x16snorm // Unpack shorts to 2 normalized signed floats
	MathUnpack2x16unorm // Unpack shorts to 2 normalized unsigned floats
	MathUnpack2x16float // Unpack half-precision shorts to 2 floats
	MathUnpack4xI8      // Unpack bytes to 4 signed ints
	MathUnpack4xU8      // Unpack bytes to 4 unsigned ints
)

// ExprAs performs a type cast or conversion.
type ExprAs struct {
	Expr    ExpressionHandle
	Kind    ScalarKind
	Convert *uint8 // If set, convert to this byte width; otherwise bitcast
}

func (ExprAs) expressionKind() {}

// ExprCallResult represents the result of a function call.
type ExprCallResult struct {
	Function FunctionHandle
}

func (ExprCallResult) expressionKind() {}

// ExprArrayLength gets the length of a runtime-sized array.
// The expression must resolve to a pointer to an array with dynamic size.
type ExprArrayLength struct {
	Array ExpressionHandle
}

func (ExprArrayLength) expressionKind() {}

// ExprAtomicResult represents the result of an atomic operation.
// This is created by StmtAtomic and holds the previous value.
type ExprAtomicResult struct{}

func (ExprAtomicResult) expressionKind() {}
