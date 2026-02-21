// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Package hlsl implements HLSL expression generation for all IR expression types.
// Expression functions are called transitively via writeBlock → writeStatement → writeExpression.
package hlsl

import (
	"fmt"
	"math"
	"strings"

	"github.com/gogpu/naga/ir"
)

// HLSL type constants for expressions.
const (
	hlslInt64  = "int64_t"
	hlslUint64 = "uint64_t"
	hlslHalf   = "half"
	hlslDouble = "double"
	hlslPosInf = "1.#INF"
	hlslNegInf = "-1.#INF"
	hlslNaN    = "0.0/0.0"
)

// =============================================================================
// Expression Writing
// =============================================================================

// writeExpression writes an IR expression to HLSL.
// Returns an error if the expression handle is invalid or the expression type
// is not supported.
func (w *Writer) writeExpression(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil {
		return fmt.Errorf("no current function context")
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return fmt.Errorf("invalid expression handle: %d", handle)
	}

	// Check if this expression has a cached name (for named expressions)
	if name, ok := w.namedExpressions[handle]; ok {
		w.out.WriteString(name)
		return nil
	}

	expr := &w.currentFunction.Expressions[handle]
	return w.writeExpressionKind(expr.Kind)
}

// writeExpressionKind writes an expression kind to HLSL.
//
//nolint:gocyclo,cyclop // Expression dispatch requires handling many cases (27 expression types)
func (w *Writer) writeExpressionKind(kind ir.ExpressionKind) error {
	switch e := kind.(type) {
	case ir.Literal:
		return w.writeLiteralExpression(e)
	case ir.ExprConstant:
		return w.writeConstantExpression(e)
	case ir.ExprZeroValue:
		return w.writeZeroValueExpression(e)
	case ir.ExprCompose:
		return w.writeComposeExpression(e)
	case ir.ExprAccess:
		return w.writeAccessExpression(e)
	case ir.ExprAccessIndex:
		return w.writeAccessIndexExpression(e)
	case ir.ExprSplat:
		return w.writeSplatExpression(e)
	case ir.ExprSwizzle:
		return w.writeSwizzleExpression(e)
	case ir.ExprFunctionArgument:
		return w.writeFunctionArgumentExpression(e)
	case ir.ExprGlobalVariable:
		return w.writeGlobalVariableExpression(e)
	case ir.ExprLocalVariable:
		return w.writeLocalVariableExpression(e)
	case ir.ExprLoad:
		return w.writeLoadExpression(e)
	case ir.ExprUnary:
		return w.writeUnaryExpression(e)
	case ir.ExprBinary:
		return w.writeBinaryExpression(e)
	case ir.ExprSelect:
		return w.writeSelectExpression(e)
	case ir.ExprRelational:
		return w.writeRelationalExpression(e)
	case ir.ExprMath:
		return w.writeMathExpression(e)
	case ir.ExprAs:
		return w.writeAsExpression(e)
	case ir.ExprDerivative:
		return w.writeDerivativeExpression(e)
	case ir.ExprImageSample:
		return w.writeImageSampleExpression(e)
	case ir.ExprImageLoad:
		return w.writeImageLoadExpression(e)
	case ir.ExprImageQuery:
		return w.writeImageQueryExpression(e)
	case ir.ExprCallResult:
		return w.writeCallResultExpression(e)
	case ir.ExprArrayLength:
		return w.writeArrayLengthExpression(e)
	case ir.ExprAtomicResult:
		// Atomic results are written by the atomic statement
		w.out.WriteString("_atomic_result")
		return nil
	default:
		return fmt.Errorf("unsupported expression type: %T", kind)
	}
}

// =============================================================================
// Literal Expressions
// =============================================================================

// writeLiteralExpression writes a literal value to HLSL.
func (w *Writer) writeLiteralExpression(lit ir.Literal) error {
	return w.writeLiteralValue(lit.Value)
}

// writeLiteralValue writes a literal value to HLSL.
func (w *Writer) writeLiteralValue(v ir.LiteralValue) error {
	switch val := v.(type) {
	case ir.LiteralBool:
		if bool(val) {
			w.out.WriteString("true")
		} else {
			w.out.WriteString("false")
		}

	case ir.LiteralI32:
		fmt.Fprintf(&w.out, "%d", int32(val))

	case ir.LiteralU32:
		fmt.Fprintf(&w.out, "%du", uint32(val))

	case ir.LiteralI64:
		fmt.Fprintf(&w.out, "%dL", int64(val))

	case ir.LiteralU64:
		fmt.Fprintf(&w.out, "%dUL", uint64(val))

	case ir.LiteralF32:
		w.out.WriteString(formatFloat32(float32(val)))

	case ir.LiteralF64:
		w.out.WriteString(formatFloat64(float64(val)))

	case ir.LiteralAbstractInt:
		fmt.Fprintf(&w.out, "%d", int64(val))

	case ir.LiteralAbstractFloat:
		w.out.WriteString(formatFloat64(float64(val)))

	default:
		return fmt.Errorf("unsupported literal type: %T", v)
	}
	return nil
}

// =============================================================================
// Constant and Zero Value Expressions
// =============================================================================

// writeConstantExpression writes a reference to a module constant.
func (w *Writer) writeConstantExpression(e ir.ExprConstant) error {
	name := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(e.Constant)}]
	if name == "" {
		name = fmt.Sprintf("const_%d", e.Constant)
	}
	w.out.WriteString(name)
	return nil
}

// writeZeroValueExpression writes a zero-initialized value.
func (w *Writer) writeZeroValueExpression(e ir.ExprZeroValue) error {
	typeName := w.getTypeName(e.Type)
	// HLSL uses (type)0 for zero initialization
	fmt.Fprintf(&w.out, "(%s)0", typeName)
	return nil
}

// =============================================================================
// Composite Expressions
// =============================================================================

// writeComposeExpression writes a composite construction (vector, matrix, array, struct).
func (w *Writer) writeComposeExpression(e ir.ExprCompose) error {
	// HLSL arrays use initializer list syntax { ... } not constructor syntax type(...)
	if isArrayType(w.module, e.Type) {
		w.out.WriteString("{")
		for i, comp := range e.Components {
			if i > 0 {
				w.out.WriteString(", ")
			}
			if err := w.writeExpression(comp); err != nil {
				return fmt.Errorf("compose component %d: %w", i, err)
			}
		}
		w.out.WriteString("}")
		return nil
	}

	typeName := w.getTypeName(e.Type)
	w.out.WriteString(typeName)
	w.out.WriteByte('(')

	for i, comp := range e.Components {
		if i > 0 {
			w.out.WriteString(", ")
		}
		if err := w.writeExpression(comp); err != nil {
			return fmt.Errorf("compose component %d: %w", i, err)
		}
	}

	w.out.WriteByte(')')
	return nil
}

// writeSplatExpression writes a scalar broadcast to vector.
func (w *Writer) writeSplatExpression(e ir.ExprSplat) error {
	// Get the vector type from the expression's resolved type
	// For now, use a generic approach
	size := e.Size
	if size < 2 || size > 4 {
		size = 4
	}

	// HLSL splat: float4(value, value, value, value) or (float4)value
	// We use the cast form for simplicity
	w.out.WriteByte('(')
	if err := w.writeExpression(e.Value); err != nil {
		return fmt.Errorf("splat value: %w", err)
	}
	w.out.WriteString(").xxxx"[:size+2]) // .xx, .xxx, or .xxxx
	return nil
}

// writeSwizzleExpression writes a vector swizzle operation.
func (w *Writer) writeSwizzleExpression(e ir.ExprSwizzle) error {
	if err := w.writeExpression(e.Vector); err != nil {
		return fmt.Errorf("swizzle vector: %w", err)
	}

	w.out.WriteByte('.')
	swizzleChars := [4]byte{'x', 'y', 'z', 'w'}
	for i := ir.VectorSize(0); i < e.Size; i++ {
		comp := e.Pattern[i]
		if comp > 3 {
			return fmt.Errorf("invalid swizzle component: %d", comp)
		}
		w.out.WriteByte(swizzleChars[comp])
	}
	return nil
}

// =============================================================================
// Access Expressions
// =============================================================================

// writeAccessExpression writes array/vector/matrix access with computed index.
func (w *Writer) writeAccessExpression(e ir.ExprAccess) error {
	if err := w.writeExpression(e.Base); err != nil {
		return fmt.Errorf("access base: %w", err)
	}
	w.out.WriteByte('[')
	if err := w.writeExpression(e.Index); err != nil {
		return fmt.Errorf("access index: %w", err)
	}
	w.out.WriteByte(']')
	return nil
}

// writeAccessIndexExpression writes access with compile-time constant index.
func (w *Writer) writeAccessIndexExpression(e ir.ExprAccessIndex) error {
	// Need to determine if this is a struct field access or array/vector access
	// For struct fields, use .member syntax; for arrays/vectors, use [index]

	// Get the base expression's type
	baseType := w.getExpressionType(e.Base)
	if baseType == nil {
		// Fallback to array syntax
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("access index base: %w", err)
		}
		fmt.Fprintf(&w.out, "[%d]", e.Index)
		return nil
	}

	switch inner := baseType.Inner.(type) {
	case ir.StructType:
		// Struct member access
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("struct access base: %w", err)
		}
		// Get member name from the struct type
		if int(e.Index) < len(inner.Members) {
			memberName := inner.Members[e.Index].Name
			if memberName == "" {
				memberName = fmt.Sprintf("member_%d", e.Index)
			}
			fmt.Fprintf(&w.out, ".%s", Escape(memberName))
		} else {
			fmt.Fprintf(&w.out, ".member_%d", e.Index)
		}

	case ir.VectorType:
		// Vector component access using swizzle
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("vector access base: %w", err)
		}
		swizzleChars := [4]byte{'x', 'y', 'z', 'w'}
		if e.Index < 4 {
			fmt.Fprintf(&w.out, ".%c", swizzleChars[e.Index])
		} else {
			fmt.Fprintf(&w.out, "[%d]", e.Index)
		}

	case ir.MatrixType:
		// Matrix column access
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("matrix access base: %w", err)
		}
		fmt.Fprintf(&w.out, "[%d]", e.Index)

	default:
		// Array or unknown - use bracket syntax
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("access index base: %w", err)
		}
		fmt.Fprintf(&w.out, "[%d]", e.Index)
	}

	return nil
}

// =============================================================================
// Variable Expressions
// =============================================================================

// writeFunctionArgumentExpression writes a reference to a function argument.
func (w *Writer) writeFunctionArgumentExpression(e ir.ExprFunctionArgument) error {
	name := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(w.currentFuncHandle), handle2: e.Index}]
	if name == "" {
		name = fmt.Sprintf("arg_%d", e.Index)
	}
	w.out.WriteString(name)
	return nil
}

// writeGlobalVariableExpression writes a reference to a global variable.
func (w *Writer) writeGlobalVariableExpression(e ir.ExprGlobalVariable) error {
	name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(e.Variable)}]
	if name == "" {
		name = fmt.Sprintf("global_%d", e.Variable)
	}
	w.out.WriteString(name)
	return nil
}

// writeLocalVariableExpression writes a reference to a local variable.
func (w *Writer) writeLocalVariableExpression(e ir.ExprLocalVariable) error {
	name := w.localNames[e.Variable]
	if name == "" {
		name = fmt.Sprintf("local_%d", e.Variable)
	}
	w.out.WriteString(name)
	return nil
}

// writeLoadExpression writes a load through a pointer.
func (w *Writer) writeLoadExpression(e ir.ExprLoad) error {
	// In HLSL, loads are implicit - just write the pointer expression
	return w.writeExpression(e.Pointer)
}

// =============================================================================
// Operator Expressions
// =============================================================================

// writeUnaryExpression writes a unary operation.
func (w *Writer) writeUnaryExpression(e ir.ExprUnary) error {
	var op string
	switch e.Op {
	case ir.UnaryNegate:
		op = "-"
	case ir.UnaryLogicalNot:
		op = "!"
	case ir.UnaryBitwiseNot:
		op = "~"
	default:
		return fmt.Errorf("unsupported unary operator: %d", e.Op)
	}

	w.out.WriteString(op)
	w.out.WriteByte('(')
	if err := w.writeExpression(e.Expr); err != nil {
		return fmt.Errorf("unary operand: %w", err)
	}
	w.out.WriteByte(')')
	return nil
}

// writeBinaryExpression writes a binary operation.
//
//nolint:gocyclo,cyclop,funlen // Binary operator mapping requires many cases (19 operators + matrix mul)
func (w *Writer) writeBinaryExpression(e ir.ExprBinary) error {
	var op string
	switch e.Op {
	case ir.BinaryAdd:
		op = "+"
	case ir.BinarySubtract:
		op = "-"
	case ir.BinaryMultiply:
		// When either operand is a matrix, use mul() with reversed args.
		// HLSL row_major storage transposes the matrix, so mul(right, left)
		// produces the correct result matching WGSL's left * right semantics.
		leftType := w.getExpressionTypeInner(e.Left)
		rightType := w.getExpressionTypeInner(e.Right)
		_, leftIsMatrix := leftType.(ir.MatrixType)
		_, rightIsMatrix := rightType.(ir.MatrixType)
		if leftIsMatrix || rightIsMatrix {
			w.out.WriteString("mul(")
			if err := w.writeExpression(e.Right); err != nil {
				return fmt.Errorf("binary right: %w", err)
			}
			w.out.WriteString(", ")
			if err := w.writeExpression(e.Left); err != nil {
				return fmt.Errorf("binary left: %w", err)
			}
			w.out.WriteString(")")
			return nil
		}
		op = "*"
	case ir.BinaryDivide:
		op = "/"
	case ir.BinaryModulo:
		op = "%"
	case ir.BinaryEqual:
		op = "=="
	case ir.BinaryNotEqual:
		op = "!="
	case ir.BinaryLess:
		op = "<"
	case ir.BinaryLessEqual:
		op = "<="
	case ir.BinaryGreater:
		op = ">"
	case ir.BinaryGreaterEqual:
		op = ">="
	case ir.BinaryAnd:
		op = "&"
	case ir.BinaryExclusiveOr:
		op = "^"
	case ir.BinaryInclusiveOr:
		op = "|"
	case ir.BinaryLogicalAnd:
		op = "&&"
	case ir.BinaryLogicalOr:
		op = "||"
	case ir.BinaryShiftLeft:
		op = "<<"
	case ir.BinaryShiftRight:
		op = ">>"
	default:
		return fmt.Errorf("unsupported binary operator: %d", e.Op)
	}

	w.out.WriteByte('(')
	if err := w.writeExpression(e.Left); err != nil {
		return fmt.Errorf("binary left: %w", err)
	}
	fmt.Fprintf(&w.out, " %s ", op)
	if err := w.writeExpression(e.Right); err != nil {
		return fmt.Errorf("binary right: %w", err)
	}
	w.out.WriteByte(')')
	return nil
}

// writeSelectExpression writes a ternary select operation.
func (w *Writer) writeSelectExpression(e ir.ExprSelect) error {
	w.out.WriteByte('(')
	if err := w.writeExpression(e.Condition); err != nil {
		return fmt.Errorf("select condition: %w", err)
	}
	w.out.WriteString(" ? ")
	if err := w.writeExpression(e.Accept); err != nil {
		return fmt.Errorf("select accept: %w", err)
	}
	w.out.WriteString(" : ")
	if err := w.writeExpression(e.Reject); err != nil {
		return fmt.Errorf("select reject: %w", err)
	}
	w.out.WriteByte(')')
	return nil
}

// =============================================================================
// Relational Expressions
// =============================================================================

// writeRelationalExpression writes a relational test function.
func (w *Writer) writeRelationalExpression(e ir.ExprRelational) error {
	var funcName string
	switch e.Fun {
	case ir.RelationalAll:
		funcName = "all"
	case ir.RelationalAny:
		funcName = "any"
	case ir.RelationalIsNan:
		funcName = "isnan"
	case ir.RelationalIsInf:
		funcName = "isinf"
	default:
		return fmt.Errorf("unsupported relational function: %d", e.Fun)
	}

	w.out.WriteString(funcName)
	w.out.WriteByte('(')
	if err := w.writeExpression(e.Argument); err != nil {
		return fmt.Errorf("relational argument: %w", err)
	}
	w.out.WriteByte(')')
	return nil
}

// =============================================================================
// Math Expressions
// =============================================================================

// writeMathExpression writes a mathematical function call.
func (w *Writer) writeMathExpression(e ir.ExprMath) error {
	funcName, err := mathFunctionToHLSL(e.Fun)
	if err != nil {
		return err
	}

	// Count arguments
	args := []ir.ExpressionHandle{e.Arg}
	if e.Arg1 != nil {
		args = append(args, *e.Arg1)
	}
	if e.Arg2 != nil {
		args = append(args, *e.Arg2)
	}
	if e.Arg3 != nil {
		args = append(args, *e.Arg3)
	}

	w.out.WriteString(funcName)
	w.out.WriteByte('(')
	for i, arg := range args {
		if i > 0 {
			w.out.WriteString(", ")
		}
		if err := w.writeExpression(arg); err != nil {
			return fmt.Errorf("math arg %d: %w", i, err)
		}
	}
	w.out.WriteByte(')')
	return nil
}

// mathFunctionToHLSL maps IR math functions to HLSL intrinsics.
//
//nolint:gocyclo,cyclop,funlen,maintidx // Math function mapping requires handling many cases (70+ functions)
func mathFunctionToHLSL(fun ir.MathFunction) (string, error) {
	switch fun {
	// Comparison functions
	case ir.MathAbs:
		return "abs", nil
	case ir.MathMin:
		return "min", nil
	case ir.MathMax:
		return "max", nil
	case ir.MathClamp:
		return "clamp", nil
	case ir.MathSaturate:
		return "saturate", nil

	// Trigonometric functions
	case ir.MathCos:
		return "cos", nil
	case ir.MathCosh:
		return "cosh", nil
	case ir.MathSin:
		return "sin", nil
	case ir.MathSinh:
		return "sinh", nil
	case ir.MathTan:
		return "tan", nil
	case ir.MathTanh:
		return "tanh", nil
	case ir.MathAcos:
		return "acos", nil
	case ir.MathAsin:
		return "asin", nil
	case ir.MathAtan:
		return "atan", nil
	case ir.MathAtan2:
		return "atan2", nil
	case ir.MathAsinh:
		return "asinh", nil // SM 6.0+ or emulated
	case ir.MathAcosh:
		return "acosh", nil // SM 6.0+ or emulated
	case ir.MathAtanh:
		return "atanh", nil // SM 6.0+ or emulated

	// Angle conversion
	case ir.MathRadians:
		return "radians", nil
	case ir.MathDegrees:
		return "degrees", nil

	// Decomposition functions
	case ir.MathCeil:
		return "ceil", nil
	case ir.MathFloor:
		return "floor", nil
	case ir.MathRound:
		return "round", nil
	case ir.MathFract:
		return "frac", nil
	case ir.MathTrunc:
		return "trunc", nil
	case ir.MathModf:
		return NagaModfFunction, nil
	case ir.MathFrexp:
		return NagaFrexpFunction, nil
	case ir.MathLdexp:
		return "ldexp", nil

	// Exponential functions
	case ir.MathExp:
		return "exp", nil
	case ir.MathExp2:
		return "exp2", nil
	case ir.MathLog:
		return "log", nil
	case ir.MathLog2:
		return "log2", nil
	case ir.MathPow:
		return "pow", nil

	// Geometric functions
	case ir.MathDot:
		return "dot", nil
	case ir.MathOuter:
		return "mul", nil // HLSL uses mul for outer product
	case ir.MathCross:
		return "cross", nil
	case ir.MathDistance:
		return "distance", nil
	case ir.MathLength:
		return "length", nil
	case ir.MathNormalize:
		return "normalize", nil
	case ir.MathFaceForward:
		return "faceforward", nil
	case ir.MathReflect:
		return "reflect", nil
	case ir.MathRefract:
		return "refract", nil

	// Computational functions
	case ir.MathSign:
		return "sign", nil
	case ir.MathFma:
		return "mad", nil // HLSL uses mad (multiply-add)
	case ir.MathMix:
		return "lerp", nil // HLSL uses lerp (linear interpolation)
	case ir.MathStep:
		return "step", nil
	case ir.MathSmoothStep:
		return "smoothstep", nil
	case ir.MathSqrt:
		return "sqrt", nil
	case ir.MathInverseSqrt:
		return "rsqrt", nil
	case ir.MathInverse:
		return "inverse", nil // SM 6.0+ or helper
	case ir.MathTranspose:
		return "transpose", nil
	case ir.MathDeterminant:
		return "determinant", nil
	case ir.MathQuantizeF16:
		return "f16tof32", nil // Requires appropriate conversion

	// Bit manipulation functions
	case ir.MathCountTrailingZeros:
		return "firstbitlow", nil
	case ir.MathCountLeadingZeros:
		return "firstbithigh", nil // Needs adjustment for actual CLZ
	case ir.MathCountOneBits:
		return "countbits", nil
	case ir.MathReverseBits:
		return "reversebits", nil
	case ir.MathExtractBits:
		return NagaExtractBitsFunction, nil
	case ir.MathInsertBits:
		return NagaInsertBitsFunction, nil
	case ir.MathFirstTrailingBit:
		return "firstbitlow", nil
	case ir.MathFirstLeadingBit:
		return "firstbithigh", nil

	// Packing functions
	case ir.MathPack4x8snorm:
		return "pack_s8", nil // Needs helper
	case ir.MathPack4x8unorm:
		return "pack_u8", nil // Needs helper
	case ir.MathPack2x16snorm:
		return "pack_snorm2x16", nil // Needs helper
	case ir.MathPack2x16unorm:
		return "pack_unorm2x16", nil // Needs helper
	case ir.MathPack2x16float:
		return "pack_half2x16", nil // Needs helper

	// Unpacking functions
	case ir.MathUnpack4x8snorm:
		return "unpack_s8", nil // Needs helper
	case ir.MathUnpack4x8unorm:
		return "unpack_u8", nil // Needs helper
	case ir.MathUnpack2x16snorm:
		return "unpack_snorm2x16", nil // Needs helper
	case ir.MathUnpack2x16unorm:
		return "unpack_unorm2x16", nil // Needs helper
	case ir.MathUnpack2x16float:
		return "unpack_half2x16", nil // Needs helper

	default:
		return "", fmt.Errorf("unsupported math function: %d", fun)
	}
}

// =============================================================================
// Cast/Conversion Expressions
// =============================================================================

// writeAsExpression writes a type cast or conversion.
func (w *Writer) writeAsExpression(e ir.ExprAs) error {
	if e.Convert != nil {
		// Explicit conversion with width
		typeName := scalarKindToHLSL(e.Kind, *e.Convert)
		fmt.Fprintf(&w.out, "(%s)(", typeName)
		if err := w.writeExpression(e.Expr); err != nil {
			return fmt.Errorf("as conversion: %w", err)
		}
		w.out.WriteByte(')')
	} else {
		// Bitcast - use asfloat/asint/asuint
		castFunc := ScalarCast(e.Kind)
		w.out.WriteString(castFunc)
		w.out.WriteByte('(')
		if err := w.writeExpression(e.Expr); err != nil {
			return fmt.Errorf("as bitcast: %w", err)
		}
		w.out.WriteByte(')')
	}
	return nil
}

// scalarKindToHLSL returns the HLSL type name for a scalar kind and width.
func scalarKindToHLSL(kind ir.ScalarKind, width uint8) string {
	switch kind {
	case ir.ScalarBool:
		return hlslTypeBool
	case ir.ScalarSint:
		if width == 8 {
			return hlslInt64
		}
		return hlslTypeInt
	case ir.ScalarUint:
		if width == 8 {
			return hlslUint64
		}
		return hlslTypeUint
	case ir.ScalarFloat:
		switch width {
		case 2:
			return hlslHalf
		case 8:
			return hlslDouble
		default:
			return hlslTypeFloat
		}
	default:
		return hlslTypeInt
	}
}

// =============================================================================
// Derivative Expressions
// =============================================================================

// writeDerivativeExpression writes a derivative (ddx/ddy/fwidth) operation.
func (w *Writer) writeDerivativeExpression(e ir.ExprDerivative) error {
	var funcName string

	switch e.Axis {
	case ir.DerivativeX:
		switch e.Control {
		case ir.DerivativeCoarse:
			funcName = "ddx_coarse"
		case ir.DerivativeFine:
			funcName = "ddx_fine"
		default:
			funcName = "ddx"
		}
	case ir.DerivativeY:
		switch e.Control {
		case ir.DerivativeCoarse:
			funcName = "ddy_coarse"
		case ir.DerivativeFine:
			funcName = "ddy_fine"
		default:
			funcName = "ddy"
		}
	case ir.DerivativeWidth:
		funcName = "fwidth"
	default:
		return fmt.Errorf("unsupported derivative axis: %d", e.Axis)
	}

	w.out.WriteString(funcName)
	w.out.WriteByte('(')
	if err := w.writeExpression(e.Expr); err != nil {
		return fmt.Errorf("derivative expr: %w", err)
	}
	w.out.WriteByte(')')
	return nil
}

// =============================================================================
// Image Expressions
// =============================================================================

// writeImageSampleExpression writes a texture sampling operation.
//
//nolint:gocyclo,gocognit,cyclop // Texture sampling has many parameters and branches to handle
func (w *Writer) writeImageSampleExpression(e ir.ExprImageSample) error {
	// Write image reference
	if err := w.writeExpression(e.Image); err != nil {
		return fmt.Errorf("image sample: image: %w", err)
	}

	// Determine sample method based on level and depth reference
	var method string
	switch e.Level.(type) {
	case ir.SampleLevelAuto:
		if e.DepthRef != nil {
			method = ".SampleCmp"
		} else {
			method = ".Sample"
		}
	case ir.SampleLevelZero:
		if e.DepthRef != nil {
			method = ".SampleCmpLevelZero"
		} else {
			method = ".SampleLevel"
		}
	case ir.SampleLevelExact:
		if e.DepthRef != nil {
			method = ".SampleCmpLevel"
		} else {
			method = ".SampleLevel"
		}
	case ir.SampleLevelBias:
		if e.DepthRef != nil {
			method = ".SampleCmpBias"
		} else {
			method = ".SampleBias"
		}
	case ir.SampleLevelGradient:
		if e.DepthRef != nil {
			method = ".SampleCmpGrad"
		} else {
			method = ".SampleGrad"
		}
	default:
		method = ".Sample"
	}

	// Handle gather operations
	if e.Gather != nil {
		comp := *e.Gather
		switch comp {
		case ir.SwizzleX:
			method = ".GatherRed"
		case ir.SwizzleY:
			method = ".GatherGreen"
		case ir.SwizzleZ:
			method = ".GatherBlue"
		case ir.SwizzleW:
			method = ".GatherAlpha"
		}
		if e.DepthRef != nil {
			method = ".GatherCmp"
		}
	}

	w.out.WriteString(method)
	w.out.WriteByte('(')

	// Sampler
	if err := w.writeExpression(e.Sampler); err != nil {
		return fmt.Errorf("image sample: sampler: %w", err)
	}
	w.out.WriteString(", ")

	// Coordinate
	if err := w.writeExpression(e.Coordinate); err != nil {
		return fmt.Errorf("image sample: coordinate: %w", err)
	}

	// Array index (combined with coordinate in HLSL)
	if e.ArrayIndex != nil {
		// In HLSL, array index is typically the last component of coordinates
		// This is a simplification - proper handling would require knowing dimensions
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.ArrayIndex); err != nil {
			return fmt.Errorf("image sample: array index: %w", err)
		}
	}

	// Depth reference for comparison samplers
	if e.DepthRef != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.DepthRef); err != nil {
			return fmt.Errorf("image sample: depth ref: %w", err)
		}
	}

	// Level of detail
	if err := w.writeSampleLevel(e.Level); err != nil {
		return err
	}

	// Offset
	if e.Offset != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.Offset); err != nil {
			return fmt.Errorf("image sample: offset: %w", err)
		}
	}

	w.out.WriteByte(')')
	return nil
}

// writeSampleLevel writes the level of detail argument.
func (w *Writer) writeSampleLevel(level ir.SampleLevel) error {
	switch l := level.(type) {
	case ir.SampleLevelAuto:
		// No additional arguments
	case ir.SampleLevelZero:
		w.out.WriteString(", 0.0")
	case ir.SampleLevelExact:
		w.out.WriteString(", ")
		if err := w.writeExpression(l.Level); err != nil {
			return fmt.Errorf("sample level: %w", err)
		}
	case ir.SampleLevelBias:
		w.out.WriteString(", ")
		if err := w.writeExpression(l.Bias); err != nil {
			return fmt.Errorf("sample bias: %w", err)
		}
	case ir.SampleLevelGradient:
		w.out.WriteString(", ")
		if err := w.writeExpression(l.X); err != nil {
			return fmt.Errorf("sample gradient x: %w", err)
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(l.Y); err != nil {
			return fmt.Errorf("sample gradient y: %w", err)
		}
	}
	return nil
}

// writeImageLoadExpression writes a texel load operation.
func (w *Writer) writeImageLoadExpression(e ir.ExprImageLoad) error {
	// Write image reference
	if err := w.writeExpression(e.Image); err != nil {
		return fmt.Errorf("image load: image: %w", err)
	}

	// Use Load method
	w.out.WriteString(".Load(")

	// For Load, HLSL requires (coord, mip) where coord includes array index
	// Use int3/int4 for coordinates
	if err := w.writeExpression(e.Coordinate); err != nil {
		return fmt.Errorf("image load: coordinate: %w", err)
	}

	// Mip level (default 0)
	if e.Level != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.Level); err != nil {
			return fmt.Errorf("image load: level: %w", err)
		}
	}

	// Sample index for MSAA
	if e.Sample != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.Sample); err != nil {
			return fmt.Errorf("image load: sample: %w", err)
		}
	}

	w.out.WriteByte(')')
	return nil
}

// writeImageQueryExpression writes an image query operation.
func (w *Writer) writeImageQueryExpression(e ir.ExprImageQuery) error {
	// Get dimensions - HLSL uses GetDimensions
	switch q := e.Query.(type) {
	case ir.ImageQuerySize:
		// texture.GetDimensions(width, height, ...) or use texture.get_dims approach
		// For simplicity, we'll use a helper pattern
		if err := w.writeExpression(e.Image); err != nil {
			return fmt.Errorf("image query: image: %w", err)
		}
		w.out.WriteString(".GetDimensions(")
		if q.Level != nil {
			if err := w.writeExpression(*q.Level); err != nil {
				return fmt.Errorf("image query: level: %w", err)
			}
			w.out.WriteString(", ")
		}
		w.out.WriteString("_dim_w, _dim_h)")
		// Note: This is incomplete - GetDimensions requires out parameters
		// Full implementation would need temporary variables

	case ir.ImageQueryNumLevels:
		if err := w.writeExpression(e.Image); err != nil {
			return fmt.Errorf("image query: image: %w", err)
		}
		w.out.WriteString(".GetDimensions(0, _dim_w, _dim_h, _num_levels)")

	case ir.ImageQueryNumLayers:
		if err := w.writeExpression(e.Image); err != nil {
			return fmt.Errorf("image query: image: %w", err)
		}
		w.out.WriteString(".GetDimensions(_dim_w, _dim_h, _num_layers)")

	case ir.ImageQueryNumSamples:
		if err := w.writeExpression(e.Image); err != nil {
			return fmt.Errorf("image query: image: %w", err)
		}
		w.out.WriteString(".GetDimensions(_dim_w, _dim_h, _num_samples)")

	default:
		return fmt.Errorf("unsupported image query: %T", e.Query)
	}

	return nil
}

// =============================================================================
// Call and Array Length Expressions
// =============================================================================

// writeCallResultExpression writes a reference to a call result.
// Normally this is NOT reached because writeCallStatement registers the
// result in namedExpressions, and writeExpression returns the cached name
// before dispatching here. This fallback exists for safety.
func (w *Writer) writeCallResultExpression(e ir.ExprCallResult) error {
	name := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(e.Function)}]
	fmt.Fprintf(&w.out, "_%s_result", name)
	return nil
}

// writeArrayLengthExpression writes a runtime array length query.
func (w *Writer) writeArrayLengthExpression(e ir.ExprArrayLength) error {
	// HLSL doesn't have a direct arrayLength equivalent
	// For RWStructuredBuffer, use GetDimensions
	if err := w.writeExpression(e.Array); err != nil {
		return fmt.Errorf("array length: %w", err)
	}
	w.out.WriteString(".GetDimensions(_array_len, _array_stride)")
	// Note: This is incomplete - would need to extract just the length
	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// getExpressionType returns the resolved type for an expression handle.
func (w *Writer) getExpressionType(handle ir.ExpressionHandle) *ir.Type {
	if w.currentFunction == nil {
		return nil
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return nil
	}

	resolution := &w.currentFunction.ExpressionTypes[handle]

	// If it references a module type, return that
	if resolution.Handle != nil {
		h := *resolution.Handle
		if int(h) < len(w.module.Types) {
			return &w.module.Types[h]
		}
	}

	// Otherwise, return a synthetic type from the value
	if resolution.Value != nil {
		return &ir.Type{Inner: resolution.Value}
	}

	return nil
}

// writeExpressionToString writes an expression to a string.
//
//nolint:unused // Helper prepared for integration when needed
func (w *Writer) writeExpressionToString(handle ir.ExpressionHandle) (string, error) {
	// Save current output
	oldOut := w.out
	w.out = strings.Builder{}

	// Write expression
	err := w.writeExpression(handle)

	// Get result and restore output
	result := w.out.String()
	w.out = oldOut

	return result, err
}

// getExpressionTypeInner returns just the TypeInner for an expression.
func (w *Writer) getExpressionTypeInner(handle ir.ExpressionHandle) ir.TypeInner {
	typ := w.getExpressionType(handle)
	if typ != nil {
		return typ.Inner
	}
	return nil
}

// formatSpecialFloat handles special float values like inf and nan.
//
//nolint:unused // Helper prepared for integration when needed
func formatSpecialFloat(f float64) (string, bool) {
	if math.IsInf(f, 1) {
		return hlslPosInf, true
	}
	if math.IsInf(f, -1) {
		return hlslNegInf, true
	}
	if math.IsNaN(f) {
		return hlslNaN, true
	}
	return "", false
}
