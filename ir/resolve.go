package ir

import "fmt"

// ResolveExpressionType resolves the type of an expression in a function.
// Returns a TypeResolution that either references a module type or contains an inline type.
//
//nolint:gocyclo,cyclop,funlen // Type resolution requires handling all expression kinds
func ResolveExpressionType(module *Module, fn *Function, handle ExpressionHandle) (TypeResolution, error) {
	if int(handle) >= len(fn.Expressions) {
		return TypeResolution{}, fmt.Errorf("expression handle %d out of range (max %d)", handle, len(fn.Expressions))
	}

	expr := fn.Expressions[handle]

	switch kind := expr.Kind.(type) {
	case Literal:
		return resolveLiteralType(kind)
	case ExprConstant:
		return resolveConstantType(module, kind)
	case ExprZeroValue:
		h := kind.Type
		return TypeResolution{Handle: &h}, nil
	case ExprCompose:
		h := kind.Type
		return TypeResolution{Handle: &h}, nil
	case ExprAccess:
		return resolveAccessType(module, fn, kind)
	case ExprAccessIndex:
		return resolveAccessIndexType(module, fn, kind)
	case ExprSplat:
		return resolveSplatType(module, fn, kind)
	case ExprSwizzle:
		return resolveSwizzleType(module, fn, kind)
	case ExprFunctionArgument:
		if int(kind.Index) >= len(fn.Arguments) {
			return TypeResolution{}, fmt.Errorf("function argument index %d out of range", kind.Index)
		}
		h := fn.Arguments[kind.Index].Type
		return TypeResolution{Handle: &h}, nil
	case ExprGlobalVariable:
		if int(kind.Variable) >= len(module.GlobalVariables) {
			return TypeResolution{}, fmt.Errorf("global variable %d out of range", kind.Variable)
		}
		h := module.GlobalVariables[kind.Variable].Type
		return TypeResolution{Handle: &h}, nil
	case ExprLocalVariable:
		if int(kind.Variable) >= len(fn.LocalVars) {
			return TypeResolution{}, fmt.Errorf("local variable %d out of range", kind.Variable)
		}
		h := fn.LocalVars[kind.Variable].Type
		return TypeResolution{Handle: &h}, nil
	case ExprLoad:
		return resolveLoadType(module, fn, kind)
	case ExprImageSample:
		return resolveImageSampleType(module, fn, kind)
	case ExprImageLoad:
		return resolveImageLoadType(module, fn, kind)
	case ExprImageQuery:
		return resolveImageQueryType(kind)
	case ExprUnary:
		return resolveUnaryType(module, fn, kind)
	case ExprBinary:
		return resolveBinaryType(module, fn, kind)
	case ExprSelect:
		return resolveSelectType(module, fn, kind)
	case ExprDerivative:
		return resolveDerivativeType(module, fn, kind)
	case ExprRelational:
		return resolveRelationalType(module, fn, kind)
	case ExprMath:
		return resolveMathType(module, fn, kind)
	case ExprAs:
		return resolveAsType(module, fn, kind)
	case ExprCallResult:
		if int(kind.Function) >= len(module.Functions) {
			return TypeResolution{}, fmt.Errorf("function %d out of range", kind.Function)
		}
		result := module.Functions[kind.Function].Result
		if result == nil {
			return TypeResolution{}, fmt.Errorf("function has no return type")
		}
		h := result.Type
		return TypeResolution{Handle: &h}, nil
	case ExprArrayLength:
		// ArrayLength returns u32
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil
	default:
		return TypeResolution{}, fmt.Errorf("unsupported expression kind: %T", kind)
	}
}

func resolveLiteralType(lit Literal) (TypeResolution, error) {
	switch v := lit.Value.(type) {
	case LiteralF64:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 8}}, nil
	case LiteralF32:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}, nil
	case LiteralU32:
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil
	case LiteralI32:
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 4}}, nil
	case LiteralU64:
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 8}}, nil
	case LiteralI64:
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 8}}, nil
	case LiteralBool:
		return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil
	case LiteralAbstractInt:
		// Abstract int defaults to i32
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 4}}, nil
	case LiteralAbstractFloat:
		// Abstract float defaults to f32
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}, nil
	default:
		return TypeResolution{}, fmt.Errorf("unknown literal type: %T", v)
	}
}

func resolveConstantType(module *Module, expr ExprConstant) (TypeResolution, error) {
	if int(expr.Constant) >= len(module.Constants) {
		return TypeResolution{}, fmt.Errorf("constant %d out of range", expr.Constant)
	}
	h := module.Constants[expr.Constant].Type
	return TypeResolution{Handle: &h}, nil
}

func resolveAccessType(module *Module, fn *Function, expr ExprAccess) (TypeResolution, error) {
	baseType, err := ResolveExpressionType(module, fn, expr.Base)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("access base: %w", err)
	}

	// Get the actual type
	var inner TypeInner
	if baseType.Handle != nil {
		if int(*baseType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *baseType.Handle)
		}
		inner = module.Types[*baseType.Handle].Inner
	} else {
		inner = baseType.Value
	}

	// Access into array, vector, or matrix returns the element type
	switch t := inner.(type) {
	case ArrayType:
		h := t.Base
		return TypeResolution{Handle: &h}, nil
	case VectorType:
		return TypeResolution{Value: t.Scalar}, nil
	case MatrixType:
		// Matrix access returns a column vector
		return TypeResolution{Value: VectorType{Size: t.Rows, Scalar: t.Scalar}}, nil
	case PointerType:
		// If accessing through a pointer, dereference first
		if int(t.Base) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("pointer base type %d out of range", t.Base)
		}
		return resolveAccessType(module, fn, ExprAccess{Base: expr.Base, Index: expr.Index})
	default:
		return TypeResolution{}, fmt.Errorf("cannot index into type %T", t)
	}
}

func resolveAccessIndexType(module *Module, fn *Function, expr ExprAccessIndex) (TypeResolution, error) {
	baseType, err := ResolveExpressionType(module, fn, expr.Base)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("access index base: %w", err)
	}

	// Get the actual type
	var inner TypeInner
	if baseType.Handle != nil {
		if int(*baseType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *baseType.Handle)
		}
		inner = module.Types[*baseType.Handle].Inner
	} else {
		inner = baseType.Value
	}

	switch t := inner.(type) {
	case ArrayType:
		h := t.Base
		return TypeResolution{Handle: &h}, nil
	case VectorType:
		return TypeResolution{Value: t.Scalar}, nil
	case MatrixType:
		return TypeResolution{Value: VectorType{Size: t.Rows, Scalar: t.Scalar}}, nil
	case StructType:
		if int(expr.Index) >= len(t.Members) {
			return TypeResolution{}, fmt.Errorf("struct member index %d out of range", expr.Index)
		}
		h := t.Members[expr.Index].Type
		return TypeResolution{Handle: &h}, nil
	case PointerType:
		// Dereference pointer first
		if int(t.Base) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("pointer base type %d out of range", t.Base)
		}
		_ = module.Types[t.Base].Inner // Validate it exists
		return resolveAccessIndexType(module, fn, ExprAccessIndex{Base: expr.Base, Index: expr.Index})
	default:
		return TypeResolution{}, fmt.Errorf("cannot index into type %T", t)
	}
}

func resolveSplatType(module *Module, fn *Function, expr ExprSplat) (TypeResolution, error) {
	valueType, err := ResolveExpressionType(module, fn, expr.Value)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("splat value: %w", err)
	}

	// Get scalar type from value
	var scalar ScalarType
	//nolint:nestif // Type resolution requires nested type checking
	if valueType.Handle != nil {
		if int(*valueType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *valueType.Handle)
		}
		inner := module.Types[*valueType.Handle].Inner
		if s, ok := inner.(ScalarType); ok {
			scalar = s
		} else {
			return TypeResolution{}, fmt.Errorf("splat value must be scalar, got %T", inner)
		}
	} else {
		if s, ok := valueType.Value.(ScalarType); ok {
			scalar = s
		} else {
			return TypeResolution{}, fmt.Errorf("splat value must be scalar, got %T", valueType.Value)
		}
	}

	return TypeResolution{Value: VectorType{Size: expr.Size, Scalar: scalar}}, nil
}

func resolveSwizzleType(module *Module, fn *Function, expr ExprSwizzle) (TypeResolution, error) {
	vectorType, err := ResolveExpressionType(module, fn, expr.Vector)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("swizzle vector: %w", err)
	}

	// Get vector type
	var vec VectorType
	//nolint:nestif // Type resolution requires nested type checking
	if vectorType.Handle != nil {
		if int(*vectorType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *vectorType.Handle)
		}
		inner := module.Types[*vectorType.Handle].Inner
		if v, ok := inner.(VectorType); ok {
			vec = v
		} else {
			return TypeResolution{}, fmt.Errorf("swizzle base must be vector, got %T", inner)
		}
	} else {
		if v, ok := vectorType.Value.(VectorType); ok {
			vec = v
		} else {
			return TypeResolution{}, fmt.Errorf("swizzle base must be vector, got %T", vectorType.Value)
		}
	}

	// Swizzle returns a vector of the same scalar type with the swizzle size
	return TypeResolution{Value: VectorType{Size: expr.Size, Scalar: vec.Scalar}}, nil
}

func resolveLoadType(module *Module, fn *Function, expr ExprLoad) (TypeResolution, error) {
	pointerType, err := ResolveExpressionType(module, fn, expr.Pointer)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("load pointer: %w", err)
	}

	// Get the actual type
	var inner TypeInner
	if pointerType.Handle != nil {
		if int(*pointerType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *pointerType.Handle)
		}
		inner = module.Types[*pointerType.Handle].Inner
	} else {
		inner = pointerType.Value
	}

	// Load dereferences a pointer
	if ptr, ok := inner.(PointerType); ok {
		h := ptr.Base
		return TypeResolution{Handle: &h}, nil
	}

	return TypeResolution{}, fmt.Errorf("load requires pointer type, got %T", inner)
}

func resolveImageSampleType(module *Module, fn *Function, expr ExprImageSample) (TypeResolution, error) {
	imageType, err := ResolveExpressionType(module, fn, expr.Image)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("image sample image: %w", err)
	}

	// Get the image type
	var inner TypeInner
	if imageType.Handle != nil {
		if int(*imageType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *imageType.Handle)
		}
		inner = module.Types[*imageType.Handle].Inner
	} else {
		inner = imageType.Value
	}

	img, ok := inner.(ImageType)
	if !ok {
		return TypeResolution{}, fmt.Errorf("image sample requires image type, got %T", inner)
	}

	// Determine result type based on image class
	if img.Class == ImageClassDepth {
		// Depth images return f32
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}, nil
	}

	// Sampled images return vec4<f32> by default
	return TypeResolution{Value: VectorType{
		Size:   Vec4,
		Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
	}}, nil
}

func resolveImageLoadType(module *Module, fn *Function, expr ExprImageLoad) (TypeResolution, error) {
	imageType, err := ResolveExpressionType(module, fn, expr.Image)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("image load image: %w", err)
	}

	// Get the image type
	var inner TypeInner
	if imageType.Handle != nil {
		if int(*imageType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *imageType.Handle)
		}
		inner = module.Types[*imageType.Handle].Inner
	} else {
		inner = imageType.Value
	}

	_, ok := inner.(ImageType)
	if !ok {
		return TypeResolution{}, fmt.Errorf("image load requires image type, got %T", inner)
	}

	// Image load returns vec4<f32>
	return TypeResolution{Value: VectorType{
		Size:   Vec4,
		Scalar: ScalarType{Kind: ScalarFloat, Width: 4},
	}}, nil
}

func resolveImageQueryType(expr ExprImageQuery) (TypeResolution, error) {
	switch expr.Query.(type) {
	case ImageQuerySize:
		// Size returns u32 for 1D, vec2<u32> for 2D, vec3<u32> for 3D/Cube
		// For simplicity, return vec3<u32>
		return TypeResolution{Value: VectorType{
			Size:   Vec3,
			Scalar: ScalarType{Kind: ScalarUint, Width: 4},
		}}, nil
	case ImageQueryNumLevels, ImageQueryNumLayers, ImageQueryNumSamples:
		// These return u32
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil
	default:
		return TypeResolution{}, fmt.Errorf("unknown image query type: %T", expr.Query)
	}
}

func resolveUnaryType(module *Module, fn *Function, expr ExprUnary) (TypeResolution, error) {
	operandType, err := ResolveExpressionType(module, fn, expr.Expr)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("unary operand: %w", err)
	}

	// Unary operators preserve the operand type
	return operandType, nil
}

func resolveBinaryType(module *Module, fn *Function, expr ExprBinary) (TypeResolution, error) {
	leftType, err := ResolveExpressionType(module, fn, expr.Left)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("binary left: %w", err)
	}

	// Comparison operators return bool or vec<bool>
	switch expr.Op {
	case BinaryEqual, BinaryNotEqual, BinaryLess, BinaryLessEqual, BinaryGreater, BinaryGreaterEqual:
		// Get the left type to determine if it's a vector
		var inner TypeInner
		if leftType.Handle != nil {
			if int(*leftType.Handle) >= len(module.Types) {
				return TypeResolution{}, fmt.Errorf("type handle %d out of range", *leftType.Handle)
			}
			inner = module.Types[*leftType.Handle].Inner
		} else {
			inner = leftType.Value
		}

		if vec, ok := inner.(VectorType); ok {
			// Vector comparison returns vector of bools
			return TypeResolution{Value: VectorType{
				Size:   vec.Size,
				Scalar: ScalarType{Kind: ScalarBool, Width: 1},
			}}, nil
		}
		// Scalar comparison returns bool
		return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil

	case BinaryLogicalAnd, BinaryLogicalOr:
		// Logical operators return bool
		return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil

	case BinaryMultiply:
		// Multiplication result type depends on both operands:
		//   scalar * vector → vector
		//   scalar * matrix → matrix
		//   matrix * vector → vector(rows)
		//   vector * matrix → vector(columns)
		// For same-type multiplication, left type is correct.
		rightType, rightErr := ResolveExpressionType(module, fn, expr.Right)
		if rightErr != nil {
			return TypeResolution{}, fmt.Errorf("binary right: %w", rightErr)
		}
		return resolveMulResultType(module, leftType, rightType), nil

	default:
		// Arithmetic and bitwise operators: if one side is scalar and the other is vector,
		// the result is vector (WGSL broadcasts scalar to match vector size).
		rightType, rightErr := ResolveExpressionType(module, fn, expr.Right)
		if rightErr == nil {
			leftInner := typeResInner(module, leftType)
			rightInner := typeResInner(module, rightType)
			_, leftIsScalar := leftInner.(ScalarType)
			_, rightIsVec := rightInner.(VectorType)
			if leftIsScalar && rightIsVec {
				return rightType, nil
			}
		}
		return leftType, nil
	}
}

// resolveMulResultType determines the result type of a multiplication.
// Matches WGSL spec: scalar*vec→vec, scalar*mat→mat, mat*vec→vec(rows), vec*mat→vec(cols).
func resolveMulResultType(module *Module, left, right TypeResolution) TypeResolution {
	leftInner := typeResInner(module, left)
	rightInner := typeResInner(module, right)

	_, leftIsScalar := leftInner.(ScalarType)
	_, rightIsScalar := rightInner.(ScalarType)
	_, leftIsVec := leftInner.(VectorType)
	_, rightIsVec := rightInner.(VectorType)
	leftMat, leftIsMat := leftInner.(MatrixType)
	rightMat, rightIsMat := rightInner.(MatrixType)

	switch {
	case leftIsScalar && rightIsVec:
		return right
	case leftIsScalar && rightIsMat:
		return right
	case leftIsVec && rightIsScalar:
		return left
	case leftIsMat && rightIsScalar:
		return left
	case leftIsMat && rightIsVec:
		// mat(cols x rows) * vec(cols) → vec(rows)
		return TypeResolution{Value: VectorType{Size: leftMat.Rows, Scalar: leftMat.Scalar}}
	case leftIsVec && rightIsMat:
		// vec(rows) * mat(cols x rows) → vec(cols)
		return TypeResolution{Value: VectorType{Size: rightMat.Columns, Scalar: rightMat.Scalar}}
	case leftIsMat && rightIsMat:
		return left
	default:
		return left
	}
}

// typeResInner extracts the TypeInner from a TypeResolution.
func typeResInner(module *Module, res TypeResolution) TypeInner {
	if res.Handle != nil {
		return module.Types[*res.Handle].Inner
	}
	return res.Value
}

func resolveSelectType(module *Module, fn *Function, expr ExprSelect) (TypeResolution, error) {
	// Select returns the type of accept/reject (they must match)
	acceptType, err := ResolveExpressionType(module, fn, expr.Accept)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("select accept: %w", err)
	}
	return acceptType, nil
}

func resolveDerivativeType(module *Module, fn *Function, expr ExprDerivative) (TypeResolution, error) {
	// Derivative preserves the expression type
	exprType, err := ResolveExpressionType(module, fn, expr.Expr)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("derivative expr: %w", err)
	}
	return exprType, nil
}

func resolveRelationalType(module *Module, fn *Function, expr ExprRelational) (TypeResolution, error) {
	argType, err := ResolveExpressionType(module, fn, expr.Argument)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("relational argument: %w", err)
	}

	// Get the actual type
	var inner TypeInner
	if argType.Handle != nil {
		if int(*argType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *argType.Handle)
		}
		inner = module.Types[*argType.Handle].Inner
	} else {
		inner = argType.Value
	}

	// Relational functions return bool or vec<bool>
	if vec, ok := inner.(VectorType); ok {
		switch expr.Fun {
		case RelationalAll, RelationalAny:
			// all/any collapse vector to single bool
			return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil
		case RelationalIsNan, RelationalIsInf:
			// isnan/isinf return vector of bools
			return TypeResolution{Value: VectorType{
				Size:   vec.Size,
				Scalar: ScalarType{Kind: ScalarBool, Width: 1},
			}}, nil
		}
	}

	// Scalar relational returns bool
	return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil
}

func resolveMathType(module *Module, fn *Function, expr ExprMath) (TypeResolution, error) {
	argType, err := ResolveExpressionType(module, fn, expr.Arg)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("math argument: %w", err)
	}

	// Special cases for math functions
	switch expr.Fun {
	case MathDot, MathDot4I8Packed, MathDot4U8Packed:
		// Dot product returns scalar
		var inner TypeInner
		if argType.Handle != nil {
			if int(*argType.Handle) >= len(module.Types) {
				return TypeResolution{}, fmt.Errorf("type handle %d out of range", *argType.Handle)
			}
			inner = module.Types[*argType.Handle].Inner
		} else {
			inner = argType.Value
		}

		if vec, ok := inner.(VectorType); ok {
			return TypeResolution{Value: vec.Scalar}, nil
		}
		return argType, nil

	case MathLength, MathDistance:
		// Length and distance return f32
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}, nil

	case MathOuter:
		// Outer product returns matrix - complex, skip for now
		return argType, nil

	default:
		// Most math functions preserve the argument type
		return argType, nil
	}
}

func resolveAsType(module *Module, fn *Function, expr ExprAs) (TypeResolution, error) {
	exprType, err := ResolveExpressionType(module, fn, expr.Expr)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("as expr: %w", err)
	}

	// Get the actual type
	var inner TypeInner
	if exprType.Handle != nil {
		if int(*exprType.Handle) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("type handle %d out of range", *exprType.Handle)
		}
		inner = module.Types[*exprType.Handle].Inner
	} else {
		inner = exprType.Value
	}

	// Convert to target kind
	if expr.Convert != nil {
		// Type conversion
		targetScalar := ScalarType{Kind: expr.Kind, Width: *expr.Convert}
		if vec, ok := inner.(VectorType); ok {
			return TypeResolution{Value: VectorType{Size: vec.Size, Scalar: targetScalar}}, nil
		}
		return TypeResolution{Value: targetScalar}, nil
	}

	// Bitcast preserves the type structure
	return exprType, nil
}
