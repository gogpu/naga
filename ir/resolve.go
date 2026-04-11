package ir

import "fmt"

// ResolveExpressionType resolves the type of an expression in a function.
// Returns a TypeResolution that either references a module type or contains an inline type.
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
	case ExprOverride:
		return resolveOverrideType(module, kind)
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
		gv := &module.GlobalVariables[kind.Variable]
		if gv.Space == SpaceHandle {
			h := gv.Type
			return TypeResolution{Handle: &h}, nil
		}
		// Non-Handle address space: variable expression is a pointer to the variable's type.
		// Matches Rust naga typifier: GlobalVariable -> Pointer { base: var.ty, space: var.space }
		return TypeResolution{Value: PointerType{Base: gv.Type, Space: gv.Space}}, nil
	case ExprLocalVariable:
		if int(kind.Variable) >= len(fn.LocalVars) {
			return TypeResolution{}, fmt.Errorf("local variable %d out of range", kind.Variable)
		}
		lv := &fn.LocalVars[kind.Variable]
		// Local variables are always pointers in function address space.
		// Matches Rust naga typifier: LocalVariable -> Pointer { base: var.ty, space: Function }
		return TypeResolution{Value: PointerType{Base: lv.Type, Space: SpaceFunction}}, nil
	case ExprLoad:
		return resolveLoadType(module, fn, kind)
	case ExprImageSample:
		return resolveImageSampleType(module, fn, kind)
	case ExprImageLoad:
		return resolveImageLoadType(module, fn, kind)
	case ExprImageQuery:
		return resolveImageQueryType(module, fn, kind)
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
	case ExprAtomicResult:
		// ExprAtomicResult now carries its type handle directly.
		h := kind.Ty
		return TypeResolution{Handle: &h}, nil
	case ExprSubgroupBallotResult:
		// SubgroupBallot always returns vec4<u32>
		return TypeResolution{Value: VectorType{
			Size:   4,
			Scalar: ScalarType{Kind: ScalarUint, Width: 4},
		}}, nil
	case ExprSubgroupOperationResult:
		h := kind.Type
		return TypeResolution{Handle: &h}, nil
	case ExprWorkGroupUniformLoadResult:
		return resolveWorkGroupUniformLoadResultType(module, fn, handle)
	case ExprRayQueryProceedResult:
		// RayQueryProceed returns bool
		return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil
	case ExprRayQueryGetIntersection:
		// Returns the RayIntersection struct type — find it in module types
		return resolveRayQueryIntersectionType(module)
	default:
		return TypeResolution{}, fmt.Errorf("unsupported expression kind: %T", kind)
	}
}

// ResolveLiteralType resolves the type of a literal expression.
func ResolveLiteralType(lit Literal) (TypeResolution, error) {
	return resolveLiteralType(lit)
}

func resolveLiteralType(lit Literal) (TypeResolution, error) {
	switch v := lit.Value.(type) {
	case LiteralF64:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 8}}, nil
	case LiteralF16:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 2}}, nil
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

func resolveOverrideType(module *Module, expr ExprOverride) (TypeResolution, error) {
	if int(expr.Override) >= len(module.Overrides) {
		return TypeResolution{}, fmt.Errorf("override %d out of range", expr.Override)
	}
	h := module.Overrides[expr.Override].Ty
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
		// Access through a pointer: resolve the pointee type and index into it.
		if int(t.Base) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("pointer base type %d out of range", t.Base)
		}
		pointeeInner := module.Types[t.Base].Inner
		switch pt := pointeeInner.(type) {
		case ArrayType:
			return TypeResolution{Value: PointerType{Base: pt.Base, Space: t.Space}}, nil
		case VectorType:
			// Pointer<Vector>[i] → ValuePointer(scalar) — pointer to element
			return TypeResolution{Value: ValuePointerType{Size: nil, Scalar: pt.Scalar, Space: t.Space}}, nil
		case MatrixType:
			// Pointer<Matrix>[i] → ValuePointer(vector) — pointer to column
			rows := pt.Rows
			return TypeResolution{Value: ValuePointerType{Size: &rows, Scalar: pt.Scalar, Space: t.Space}}, nil
		case BindingArrayType:
			return TypeResolution{Value: PointerType{Base: pt.Base, Space: t.Space}}, nil
		default:
			return TypeResolution{}, fmt.Errorf("cannot index through pointer into type %T", pt)
		}
	case ValuePointerType:
		// ValuePointer(vector)[i] → ValuePointer(scalar) — pointer to element
		if t.Size != nil {
			return TypeResolution{Value: ValuePointerType{Size: nil, Scalar: t.Scalar, Space: t.Space}}, nil
		}
		return TypeResolution{}, fmt.Errorf("cannot dynamically index into scalar value pointer")
	case BindingArrayType:
		h := t.Base
		return TypeResolution{Handle: &h}, nil
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
		// Access through a pointer: resolve the pointee type and index into it.
		// Results remain pointers (ValuePointerType) per Rust naga typifier behavior.
		if int(t.Base) >= len(module.Types) {
			return TypeResolution{}, fmt.Errorf("pointer base type %d out of range", t.Base)
		}
		pointeeInner := module.Types[t.Base].Inner
		switch pt := pointeeInner.(type) {
		case ArrayType:
			// Pointer<Array<T>>[i] → Pointer<T> (preserves pointer-ness)
			return TypeResolution{Value: PointerType{Base: pt.Base, Space: t.Space}}, nil
		case VectorType:
			// Pointer<Vector>[i] → ValuePointer(scalar) — pointer to element
			return TypeResolution{Value: ValuePointerType{Size: nil, Scalar: pt.Scalar, Space: t.Space}}, nil
		case MatrixType:
			// Pointer<Matrix>[i] → ValuePointer(vector) — pointer to column
			rows := pt.Rows
			return TypeResolution{Value: ValuePointerType{Size: &rows, Scalar: pt.Scalar, Space: t.Space}}, nil
		case StructType:
			if int(expr.Index) >= len(pt.Members) {
				return TypeResolution{}, fmt.Errorf("struct member index %d out of range through pointer", expr.Index)
			}
			// Pointer<Struct>.member → Pointer<MemberType> (preserves pointer-ness and address space)
			return TypeResolution{Value: PointerType{Base: pt.Members[expr.Index].Type, Space: t.Space}}, nil
		case BindingArrayType:
			return TypeResolution{Value: PointerType{Base: pt.Base, Space: t.Space}}, nil
		default:
			return TypeResolution{}, fmt.Errorf("cannot index through pointer into type %T", pt)
		}
	case ValuePointerType:
		// ValuePointer(vector)[i] → ValuePointer(scalar) — pointer to element
		if t.Size != nil {
			return TypeResolution{Value: ValuePointerType{Size: nil, Scalar: t.Scalar, Space: t.Space}}, nil
		}
		return TypeResolution{}, fmt.Errorf("cannot index into scalar value pointer")
	case BindingArrayType:
		h := t.Base
		return TypeResolution{Handle: &h}, nil
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
		// If the base type is Atomic(scalar), resolve to Scalar(scalar).
		// This matches Rust naga's typifier (proc/typifier.rs).
		if int(ptr.Base) < len(module.Types) {
			if at, ok := module.Types[ptr.Base].Inner.(AtomicType); ok {
				return TypeResolution{Value: at.Scalar}, nil
			}
		}
		h := ptr.Base
		return TypeResolution{Handle: &h}, nil
	}

	// Load on ValuePointer dereferences to scalar or vector value.
	// ValuePointer{Size: nil} → ScalarType, ValuePointer{Size: &s} → VectorType.
	if vp, ok := inner.(ValuePointerType); ok {
		if vp.Size != nil {
			return TypeResolution{Value: VectorType{Size: *vp.Size, Scalar: vp.Scalar}}, nil
		}
		return TypeResolution{Value: vp.Scalar}, nil
	}

	// If the pointer expression resolves to an Atomic type (e.g., from AccessIndex
	// through a pointer to a struct/array containing atomics), resolve to Scalar.
	// This matches Rust naga where Load on Atomic always gives Scalar.
	if at, ok := inner.(AtomicType); ok {
		return TypeResolution{Value: at.Scalar}, nil
	}

	// When the pointer expression is a variable reference (GlobalVariable, LocalVariable)
	// that resolves directly to the value type rather than a PointerType, the Load
	// just produces the same type. This supports the WGSL Load Rule pattern where
	// the lowerer inserts ExprLoad after variable references to match Rust naga's
	// expression handle numbering.
	return pointerType, nil
}

func resolveImageSampleType(module *Module, fn *Function, expr ExprImageSample) (TypeResolution, error) {
	// Gather operations always return vec4, even for depth textures.
	// This matches Rust naga typifier: ImageSample with gather: Some(_) -> Vec4.
	if expr.Gather != nil {
		return resolveImageGatherType(module, fn, expr.Image)
	}
	return resolveImageResultType(module, fn, expr.Image, "image sample")
}

// resolveImageGatherType resolves the return type for image gather operations.
// Gather always returns vec4, with scalar kind matching the image's sampled kind.
func resolveImageGatherType(module *Module, fn *Function, imageHandle ExpressionHandle) (TypeResolution, error) {
	imageType, err := ResolveExpressionType(module, fn, imageHandle)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("image gather image: %w", err)
	}

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
		return TypeResolution{}, fmt.Errorf("image gather requires image type, got %T", inner)
	}

	scalar := ScalarType{Kind: ScalarFloat, Width: 4}
	if img.Class == ImageClassSampled {
		scalar = ScalarType{Kind: img.SampledKind, Width: 4}
	}

	return TypeResolution{Value: VectorType{
		Size:   Vec4,
		Scalar: scalar,
	}}, nil
}

func resolveImageLoadType(module *Module, fn *Function, expr ExprImageLoad) (TypeResolution, error) {
	return resolveImageResultType(module, fn, expr.Image, "image load")
}

// resolveImageResultType resolves the return type for image sample/load operations.
// Depth images return scalar f32, sampled/storage images return vec4<f32>.
func resolveImageResultType(module *Module, fn *Function, imageHandle ExpressionHandle, context string) (TypeResolution, error) {
	imageType, err := ResolveExpressionType(module, fn, imageHandle)
	if err != nil {
		return TypeResolution{}, fmt.Errorf("%s image: %w", context, err)
	}

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
		return TypeResolution{}, fmt.Errorf("%s requires image type, got %T", context, inner)
	}

	if img.Class == ImageClassDepth {
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}, nil
	}

	// Determine the scalar type based on image class.
	// For storage images, use the format's scalar kind.
	// For sampled images, use the SampledKind.
	// Default to float for compatibility (many images don't have SampledKind set correctly).
	scalarKind := ScalarFloat
	width := uint8(4)
	switch img.Class {
	case ImageClassStorage:
		scalarKind = img.StorageFormat.ScalarKind()
		// R64Uint/R64Sint use 8-byte scalars
		if img.StorageFormat == StorageFormatR64Uint || img.StorageFormat == StorageFormatR64Sint {
			width = 8
		}
	case ImageClassSampled:
		// Only use SampledKind if explicitly set (non-zero and non-Sint default).
		// Many image types share the same type handle and don't have SampledKind set correctly.
		if img.SampledKind == ScalarUint {
			scalarKind = ScalarUint
		} else if img.SampledKind == ScalarFloat {
			scalarKind = ScalarFloat
		}
		// Default to Float for ScalarSint (which is the zero value / default)
	}

	return TypeResolution{Value: VectorType{
		Size:   Vec4,
		Scalar: ScalarType{Kind: scalarKind, Width: width},
	}}, nil
}

func resolveImageQueryType(module *Module, fn *Function, expr ExprImageQuery) (TypeResolution, error) {
	switch q := expr.Query.(type) {
	case ImageQuerySize:
		// Size returns the spatial dimensions only (no array layer count):
		//   u32 for 1D (including 1D array)
		//   vec2<u32> for 2D (including 2D array), Cube (including Cube array)
		//   vec3<u32> for 3D
		// Arrayed textures do NOT add an extra component for the layer count.
		// The array layer count is queried separately via ImageQueryNumLayers.
		// This matches Rust naga's proc::typifier which uses
		// image_query_size_result_type without adding array dimensions.
		_ = q
		dim := Dim2D // default

		// Try to resolve the image's dimension from its type.
		if fn != nil && int(expr.Image) < len(fn.Expressions) {
			imgType, err := ResolveExpressionType(module, fn, expr.Image)
			if err == nil {
				var inner TypeInner
				if imgType.Handle != nil && int(*imgType.Handle) < len(module.Types) {
					inner = module.Types[*imgType.Handle].Inner
				} else if imgType.Value != nil {
					inner = imgType.Value
				}
				if it, ok := inner.(ImageType); ok {
					dim = it.Dim
				}
			}
		}

		uintScalar := ScalarType{Kind: ScalarUint, Width: 4}
		var vecSize VectorSize
		switch dim {
		case Dim1D:
			// 1D: returns scalar u32
			return TypeResolution{Value: uintScalar}, nil
		case Dim2D:
			vecSize = Vec2
		case Dim3D:
			vecSize = Vec3
		case DimCube:
			vecSize = Vec2
		default:
			vecSize = Vec2
		}
		return TypeResolution{Value: VectorType{
			Size:   vecSize,
			Scalar: uintScalar,
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
			leftInner := TypeResInner(module, leftType)
			rightInner := TypeResInner(module, rightType)
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
	leftInner := TypeResInner(module, left)
	rightInner := TypeResInner(module, right)

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
		// mat(C1 x R1) * mat(C2 x R2) where C1==R2 → mat(C2 x R1)
		return TypeResolution{Value: MatrixType{
			Columns: rightMat.Columns,
			Rows:    leftMat.Rows,
			Scalar:  leftMat.Scalar,
		}}
	default:
		return left
	}
}

// findOrInferScalarHandle finds the type handle for a scalar type in the module.
// Returns 0 if not found (callers should handle gracefully).
func findOrInferScalarHandle(module *Module, scalar ScalarType) TypeHandle {
	for i, t := range module.Types {
		if s, ok := t.Inner.(ScalarType); ok && s == scalar {
			return TypeHandle(i)
		}
	}
	return 0
}

// findOrInferVectorHandle finds the type handle for a vector type in the module.
func findOrInferVectorHandle(module *Module, vec VectorType) TypeHandle {
	for i, t := range module.Types {
		if v, ok := t.Inner.(VectorType); ok && v.Size == vec.Size && v.Scalar == vec.Scalar {
			return TypeHandle(i)
		}
	}
	return 0
}

// TypeResInner returns the inner type of a TypeResolution.
func TypeResInner(module *Module, res TypeResolution) TypeInner {
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
	case MathDot4I8Packed:
		// Dot product of packed 4xi8 returns signed int
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 4}}, nil

	case MathDot4U8Packed:
		// Dot product of packed 4xu8 returns unsigned uint
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil

	case MathDot:
		// Dot product returns scalar of the vector's component type
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

	case MathUnpack4xI8:
		// unpack4xI8 returns vec4<i32>
		return TypeResolution{Value: VectorType{Size: 4, Scalar: ScalarType{Kind: ScalarSint, Width: 4}}}, nil

	case MathUnpack4xU8:
		// unpack4xU8 returns vec4<u32>
		return TypeResolution{Value: VectorType{Size: 4, Scalar: ScalarType{Kind: ScalarUint, Width: 4}}}, nil

	case MathPack4xI8, MathPack4xU8, MathPack4xI8Clamp, MathPack4xU8Clamp,
		MathPack4x8snorm, MathPack4x8unorm,
		MathPack2x16snorm, MathPack2x16unorm, MathPack2x16float:
		// All pack functions return u32
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil

	case MathUnpack4x8snorm, MathUnpack4x8unorm:
		// unpack4x8snorm/unorm returns vec4<f32>
		return TypeResolution{Value: VectorType{Size: 4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, nil

	case MathUnpack2x16snorm, MathUnpack2x16unorm, MathUnpack2x16float:
		// unpack2x16snorm/unorm/float returns vec2<f32>
		return TypeResolution{Value: VectorType{Size: 2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, nil

	case MathCountOneBits, MathReverseBits, MathCountTrailingZeros, MathCountLeadingZeros:
		// Bit operations preserve the argument type
		return argType, nil

	case MathModf:
		// Modf returns a struct __modf_result_* with {fract, whole}.
		// Search for the struct type by name in the module.
		structName := modfResultStructName(module, argType)
		if h := findNamedType(module, structName); h >= 0 {
			handle := TypeHandle(h)
			return TypeResolution{Handle: &handle}, nil
		}
		// Fallback: return the arg type (for cases where struct isn't created yet)
		return argType, nil

	case MathFrexp:
		// Frexp returns a struct __frexp_result_* with {fract, exp}.
		structName := frexpResultStructName(module, argType)
		if h := findNamedType(module, structName); h >= 0 {
			handle := TypeHandle(h)
			return TypeResolution{Handle: &handle}, nil
		}
		return argType, nil

	default:
		// Most math functions preserve the argument type
		return argType, nil
	}
}

// modfResultStructName returns the name of the modf result struct for a given arg type.
func modfResultStructName(module *Module, argType TypeResolution) string {
	inner := resolveInner(module, argType)
	switch t := inner.(type) {
	case VectorType:
		sizeStr := ""
		switch t.Size {
		case Vec2:
			sizeStr = "vec2"
		case Vec3:
			sizeStr = "vec3"
		case Vec4:
			sizeStr = "vec4"
		}
		widthStr := "f32"
		if t.Scalar.Width == 8 {
			widthStr = "f64"
		} else if t.Scalar.Width == 2 {
			widthStr = "f16"
		}
		return "__modf_result_" + sizeStr + "_" + widthStr
	default:
		return "__modf_result_f32"
	}
}

// frexpResultStructName returns the name of the frexp result struct for a given arg type.
func frexpResultStructName(module *Module, argType TypeResolution) string {
	inner := resolveInner(module, argType)
	switch t := inner.(type) {
	case VectorType:
		sizeStr := ""
		switch t.Size {
		case Vec2:
			sizeStr = "vec2"
		case Vec3:
			sizeStr = "vec3"
		case Vec4:
			sizeStr = "vec4"
		}
		widthStr := "f32"
		if t.Scalar.Width == 8 {
			widthStr = "f64"
		} else if t.Scalar.Width == 2 {
			widthStr = "f16"
		}
		return "__frexp_result_" + sizeStr + "_" + widthStr
	default:
		return "__frexp_result_f32"
	}
}

// FindModfResultType returns the TypeHandle for the modf result struct for a given argument type.
// Returns -1 if not found.
func FindModfResultType(module *Module, argType TypeResolution) TypeHandle {
	name := modfResultStructName(module, argType)
	h := findNamedType(module, name)
	return TypeHandle(h)
}

// FindFrexpResultType returns the TypeHandle for the frexp result struct for a given argument type.
// Returns -1 if not found.
func FindFrexpResultType(module *Module, argType TypeResolution) TypeHandle {
	name := frexpResultStructName(module, argType)
	h := findNamedType(module, name)
	return TypeHandle(h)
}

// resolveInner gets the TypeInner from a TypeResolution.
func resolveInner(module *Module, res TypeResolution) TypeInner {
	if res.Handle != nil && int(*res.Handle) < len(module.Types) {
		return module.Types[*res.Handle].Inner
	}
	return res.Value
}

// findNamedType searches for a type by name in the module's type arena.
func findNamedType(module *Module, name string) int {
	for i, t := range module.Types {
		if t.Name == name {
			return i
		}
	}
	return -1
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
		if mat, ok := inner.(MatrixType); ok {
			return TypeResolution{Value: MatrixType{Columns: mat.Columns, Rows: mat.Rows, Scalar: targetScalar}}, nil
		}
		return TypeResolution{Value: targetScalar}, nil
	}

	// Bitcast: reinterpret bits as target kind, preserving width and vector size.
	// as_type<float>(int_val) → Float scalar, as_type<int>(float_val) → Sint scalar.
	targetScalar := ScalarType{Kind: expr.Kind}
	switch t := inner.(type) {
	case ScalarType:
		targetScalar.Width = t.Width
		return TypeResolution{Value: targetScalar}, nil
	case VectorType:
		targetScalar.Width = t.Scalar.Width
		return TypeResolution{Value: VectorType{Size: t.Size, Scalar: targetScalar}}, nil
	default:
		// Fallback: preserve original type
		return exprType, nil
	}
}

// resolveAtomicResultType resolves the type of an atomic operation result.
// Searches the function body for the StmtAtomic that writes to this expression handle,
// then returns the scalar type of the atomic pointer.
func resolveAtomicResultType(module *Module, fn *Function, handle ExpressionHandle) (TypeResolution, error) {
	if atomicType := findAtomicTypeForResult(module, fn, fn.Body, handle); atomicType != nil {
		return TypeResolution{Value: *atomicType}, nil
	}
	// Fallback: most atomics operate on u32
	return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil
}

// findAtomicTypeForResult recursively searches statements for a StmtAtomic with matching Result.
func findAtomicTypeForResult(module *Module, fn *Function, stmts []Statement, handle ExpressionHandle) *ScalarType {
	for _, stmt := range stmts {
		if s, ok := stmt.Kind.(StmtAtomic); ok && s.Result != nil && *s.Result == handle {
			return ResolveAtomicPointerScalar(module, fn, s.Pointer)
		}
		for _, sub := range stmtSubBlocks(stmt) {
			if t := findAtomicTypeForResult(module, fn, sub, handle); t != nil {
				return t
			}
		}
	}
	return nil
}

// ResolveAtomicPointerScalar resolves a pointer expression to its atomic scalar type.
func ResolveAtomicPointerScalar(module *Module, fn *Function, pointer ExpressionHandle) *ScalarType {
	ptrType, err := ResolveExpressionType(module, fn, pointer)
	if err != nil {
		return nil
	}
	inner := TypeResInner(module, ptrType)
	if at, ok := inner.(AtomicType); ok {
		return &at.Scalar
	}
	if st, ok := inner.(ScalarType); ok {
		return &st
	}
	return nil
}

// resolveWorkGroupUniformLoadResultType resolves the type of a workgroup uniform load result.
// Searches the function body for the StmtWorkGroupUniformLoad that writes to this expression handle,
// then returns the pointee type of the pointer operand.
func resolveWorkGroupUniformLoadResultType(module *Module, fn *Function, handle ExpressionHandle) (TypeResolution, error) {
	if t := findWorkGroupUniformLoadType(module, fn, fn.Body, handle); t != nil {
		return *t, nil
	}
	return TypeResolution{}, fmt.Errorf("workgroup uniform load result type not found for expression %d", handle)
}

// findWorkGroupUniformLoadType recursively searches for a StmtWorkGroupUniformLoad targeting the given result handle.
func findWorkGroupUniformLoadType(module *Module, fn *Function, stmts []Statement, handle ExpressionHandle) *TypeResolution {
	for _, stmt := range stmts {
		if s, ok := stmt.Kind.(StmtWorkGroupUniformLoad); ok && s.Result == handle {
			// Resolve the pointer type, then get the pointee.
			ptrRes, err := ResolveExpressionType(module, fn, s.Pointer)
			if err != nil {
				return nil
			}
			inner := TypeResInner(module, ptrRes)
			if pt, ok := inner.(PointerType); ok {
				// Return the base type of the pointer.
				if int(pt.Base) < len(module.Types) {
					baseInner := module.Types[pt.Base].Inner
					if arrayType, ok := baseInner.(ArrayType); ok {
						// For array pointers, the load returns the element type.
						_ = arrayType
					}
					h := pt.Base
					return &TypeResolution{Handle: &h}
				}
			}
			return nil
		}
		for _, sub := range stmtSubBlocks(stmt) {
			if t := findWorkGroupUniformLoadType(module, fn, sub, handle); t != nil {
				return t
			}
		}
	}
	return nil
}

// resolveRayQueryIntersectionType finds the RayIntersection struct type in the module.
func resolveRayQueryIntersectionType(module *Module) (TypeResolution, error) {
	for i, t := range module.Types {
		if t.Name == "RayIntersection" {
			h := TypeHandle(i)
			return TypeResolution{Handle: &h}, nil
		}
	}
	// Fallback: not found, return a generic struct placeholder
	return TypeResolution{}, fmt.Errorf("RayIntersection type not found in module")
}

// stmtSubBlocks returns all nested statement blocks for a given statement.
func stmtSubBlocks(stmt Statement) []Block {
	switch s := stmt.Kind.(type) {
	case StmtBlock:
		return []Block{s.Block}
	case StmtIf:
		return []Block{s.Accept, s.Reject}
	case StmtLoop:
		return []Block{s.Body, s.Continuing}
	case StmtSwitch:
		blocks := make([]Block, len(s.Cases))
		for i, c := range s.Cases {
			blocks[i] = c.Body
		}
		return blocks
	default:
		return nil
	}
}
