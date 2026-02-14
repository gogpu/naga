// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// GLSL type name constants for repeated use.
const (
	glslTypeInt   = "int"
	glslTypeUint  = "uint"
	glslTypeFloat = "float"
)

// writeExpression writes an expression and returns its GLSL representation.
func (w *Writer) writeExpression(handle ir.ExpressionHandle) (string, error) {
	// Check if this expression was already named
	if name, ok := w.namedExpressions[handle]; ok {
		return name, nil
	}

	if w.currentFunction == nil {
		return "", fmt.Errorf("no current function context")
	}

	if int(handle) >= len(w.currentFunction.Expressions) {
		return "", fmt.Errorf("invalid expression handle: %d", handle)
	}

	expr := &w.currentFunction.Expressions[handle]
	return w.writeExpressionKind(expr.Kind, handle)
}

// writeExpressionKind writes the expression based on its kind.
//
//nolint:gocyclo,cyclop // Expression handling requires many cases
func (w *Writer) writeExpressionKind(kind ir.ExpressionKind, _ ir.ExpressionHandle) (string, error) {
	switch k := kind.(type) {
	case ir.Literal:
		return w.writeLiteral(k)
	case ir.ExprConstant:
		return w.writeConstant(k)
	case ir.ExprZeroValue:
		return w.writeZeroValue(k)
	case ir.ExprCompose:
		return w.writeCompose(k)
	case ir.ExprAccess:
		return w.writeAccess(k)
	case ir.ExprAccessIndex:
		return w.writeAccessIndex(k)
	case ir.ExprSplat:
		return w.writeSplat(k)
	case ir.ExprSwizzle:
		return w.writeSwizzle(k)
	case ir.ExprFunctionArgument:
		return w.writeFunctionArgument(k)
	case ir.ExprGlobalVariable:
		return w.writeGlobalVariable(k)
	case ir.ExprLocalVariable:
		return w.writeLocalVariable(k)
	case ir.ExprLoad:
		return w.writeLoad(k)
	case ir.ExprUnary:
		return w.writeUnary(k)
	case ir.ExprBinary:
		return w.writeBinary(k)
	case ir.ExprSelect:
		return w.writeSelect(k)
	case ir.ExprRelational:
		return w.writeRelational(k)
	case ir.ExprMath:
		return w.writeMath(k)
	case ir.ExprDerivative:
		return w.writeDerivative(k)
	case ir.ExprImageSample:
		return w.writeImageSample(k)
	case ir.ExprImageLoad:
		return w.writeImageLoad(k)
	case ir.ExprImageQuery:
		return w.writeImageQuery(k)
	case ir.ExprAs:
		return w.writeAs(k)
	case ir.ExprCallResult:
		return w.writeCallResult(k)
	case ir.ExprAtomicResult:
		return w.writeAtomicResult(k)
	case ir.ExprArrayLength:
		return w.writeArrayLength(k)
	default:
		return "", fmt.Errorf("unsupported expression kind: %T", kind)
	}
}

// writeLiteral writes a literal expression.
func (w *Writer) writeLiteral(lit ir.Literal) (string, error) {
	switch v := lit.Value.(type) {
	case ir.LiteralBool:
		if v {
			return "true", nil
		}
		return "false", nil
	case ir.LiteralI32:
		return fmt.Sprintf("%d", int32(v)), nil
	case ir.LiteralU32:
		return fmt.Sprintf("%du", uint32(v)), nil
	case ir.LiteralI64:
		return fmt.Sprintf("%dL", int64(v)), nil
	case ir.LiteralU64:
		return fmt.Sprintf("%duL", uint64(v)), nil
	case ir.LiteralF32:
		return formatFloat(float32(v)), nil
	case ir.LiteralF64:
		return formatFloat64(float64(v)), nil
	case ir.LiteralAbstractInt:
		return fmt.Sprintf("%d", int64(v)), nil
	case ir.LiteralAbstractFloat:
		return formatFloat64(float64(v)), nil
	default:
		return "0", nil
	}
}

// writeConstant writes a constant reference.
func (w *Writer) writeConstant(c ir.ExprConstant) (string, error) {
	name := w.names[nameKey{kind: nameKeyConstant, handle1: uint32(c.Constant)}]
	return name, nil
}

// writeZeroValue writes a zero-initialized value.
func (w *Writer) writeZeroValue(z ir.ExprZeroValue) (string, error) {
	typeName := w.getTypeName(z.Type)
	return fmt.Sprintf("%s(0)", typeName), nil
}

// writeCompose writes a composite construction expression.
func (w *Writer) writeCompose(c ir.ExprCompose) (string, error) {
	typeName := w.getTypeName(c.Type)

	components := make([]string, 0, len(c.Components))
	for _, comp := range c.Components {
		compStr, err := w.writeExpression(comp)
		if err != nil {
			return "", err
		}
		components = append(components, compStr)
	}

	return fmt.Sprintf("%s(%s)", typeName, strings.Join(components, ", ")), nil
}

// writeAccess writes an array/struct access expression with dynamic index.
func (w *Writer) writeAccess(a ir.ExprAccess) (string, error) {
	base, err := w.writeExpression(a.Base)
	if err != nil {
		return "", err
	}
	index, err := w.writeExpression(a.Index)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s[%s]", base, index), nil
}

// writeAccessIndex writes a constant-index access expression.
//
//nolint:nestif // Struct member lookup requires nested type checking
func (w *Writer) writeAccessIndex(a ir.ExprAccessIndex) (string, error) {
	base, err := w.writeExpression(a.Base)
	if err != nil {
		return "", err
	}

	// Check if this is a struct member access
	if w.currentFunction != nil && int(a.Base) < len(w.currentFunction.Expressions) {
		baseExpr := &w.currentFunction.Expressions[a.Base]
		if baseTypeHandle := w.getExpressionTypeHandle(baseExpr.Kind); baseTypeHandle != nil {
			if int(*baseTypeHandle) < len(w.module.Types) {
				baseType := &w.module.Types[*baseTypeHandle]
				if st, ok := baseType.Inner.(ir.StructType); ok {
					if int(a.Index) < len(st.Members) {
						memberName := st.Members[a.Index].Name
						if memberName != "" {
							return fmt.Sprintf("%s.%s", base, escapeKeyword(memberName)), nil
						}
					}
				}
			}
		}
	}

	return fmt.Sprintf("%s[%d]", base, a.Index), nil
}

// writeSplat writes a splat expression (scalar to vector).
func (w *Writer) writeSplat(s ir.ExprSplat) (string, error) {
	value, err := w.writeExpression(s.Value)
	if err != nil {
		return "", err
	}
	// In GLSL, vec constructors accept scalar and broadcast
	return fmt.Sprintf("vec%d(%s)", s.Size, value), nil
}

// writeSwizzle writes a swizzle expression.
func (w *Writer) writeSwizzle(s ir.ExprSwizzle) (string, error) {
	vector, err := w.writeExpression(s.Vector)
	if err != nil {
		return "", err
	}

	components := "xyzw"
	var swizzle string
	for i := ir.VectorSize(0); i < s.Size; i++ {
		if int(s.Pattern[i]) < len(components) {
			swizzle += string(components[s.Pattern[i]])
		}
	}

	return fmt.Sprintf("%s.%s", vector, swizzle), nil
}

// writeFunctionArgument writes a function argument reference.
func (w *Writer) writeFunctionArgument(a ir.ExprFunctionArgument) (string, error) {
	// In entry points, builtin arguments map to GLSL built-in variables.
	if w.inEntryPoint && w.currentFunction != nil && int(a.Index) < len(w.currentFunction.Arguments) {
		arg := &w.currentFunction.Arguments[a.Index]
		if arg.Binding != nil {
			if b, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
				return glslBuiltIn(b.Builtin, false), nil
			}
		}
	}
	name := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(w.currentFuncHandle), handle2: a.Index}]
	return name, nil
}

// writeGlobalVariable writes a global variable reference.
func (w *Writer) writeGlobalVariable(g ir.ExprGlobalVariable) (string, error) {
	name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(g.Variable)}]
	return name, nil
}

// writeLocalVariable writes a local variable reference.
func (w *Writer) writeLocalVariable(l ir.ExprLocalVariable) (string, error) {
	if name, ok := w.localNames[l.Variable]; ok {
		return name, nil
	}
	return fmt.Sprintf("local_%d", l.Variable), nil
}

// writeLoad writes a load expression (dereference).
func (w *Writer) writeLoad(l ir.ExprLoad) (string, error) {
	// In GLSL, loading is implicit
	return w.writeExpression(l.Pointer)
}

// writeUnary writes a unary expression.
func (w *Writer) writeUnary(u ir.ExprUnary) (string, error) {
	operand, err := w.writeExpression(u.Expr)
	if err != nil {
		return "", err
	}

	switch u.Op {
	case ir.UnaryNegate:
		return fmt.Sprintf("-(%s)", operand), nil
	case ir.UnaryLogicalNot:
		return fmt.Sprintf("!(%s)", operand), nil
	case ir.UnaryBitwiseNot:
		return fmt.Sprintf("~(%s)", operand), nil
	default:
		return "", fmt.Errorf("unsupported unary operator: %v", u.Op)
	}
}

// writeBinary writes a binary expression.
//
//nolint:gocyclo,cyclop // Binary operators require many cases
func (w *Writer) writeBinary(b ir.ExprBinary) (string, error) {
	left, err := w.writeExpression(b.Left)
	if err != nil {
		return "", err
	}
	right, err := w.writeExpression(b.Right)
	if err != nil {
		return "", err
	}

	switch b.Op {
	case ir.BinaryAdd:
		return fmt.Sprintf("(%s + %s)", left, right), nil
	case ir.BinarySubtract:
		return fmt.Sprintf("(%s - %s)", left, right), nil
	case ir.BinaryMultiply:
		return fmt.Sprintf("(%s * %s)", left, right), nil
	case ir.BinaryDivide:
		return fmt.Sprintf("(%s / %s)", left, right), nil
	case ir.BinaryModulo:
		// Use helper for integer modulo to match WGSL semantics
		w.needsModHelper = true
		return fmt.Sprintf("_naga_mod(%s, %s)", left, right), nil
	case ir.BinaryEqual:
		return fmt.Sprintf("(%s == %s)", left, right), nil
	case ir.BinaryNotEqual:
		return fmt.Sprintf("(%s != %s)", left, right), nil
	case ir.BinaryLess:
		return fmt.Sprintf("(%s < %s)", left, right), nil
	case ir.BinaryLessEqual:
		return fmt.Sprintf("(%s <= %s)", left, right), nil
	case ir.BinaryGreater:
		return fmt.Sprintf("(%s > %s)", left, right), nil
	case ir.BinaryGreaterEqual:
		return fmt.Sprintf("(%s >= %s)", left, right), nil
	case ir.BinaryAnd:
		return fmt.Sprintf("(%s & %s)", left, right), nil
	case ir.BinaryExclusiveOr:
		return fmt.Sprintf("(%s ^ %s)", left, right), nil
	case ir.BinaryInclusiveOr:
		return fmt.Sprintf("(%s | %s)", left, right), nil
	case ir.BinaryLogicalAnd:
		return fmt.Sprintf("(%s && %s)", left, right), nil
	case ir.BinaryLogicalOr:
		return fmt.Sprintf("(%s || %s)", left, right), nil
	case ir.BinaryShiftLeft:
		return fmt.Sprintf("(%s << %s)", left, right), nil
	case ir.BinaryShiftRight:
		return fmt.Sprintf("(%s >> %s)", left, right), nil
	default:
		return "", fmt.Errorf("unsupported binary operator: %v", b.Op)
	}
}

// writeSelect writes a select (ternary) expression.
func (w *Writer) writeSelect(s ir.ExprSelect) (string, error) {
	condition, err := w.writeExpression(s.Condition)
	if err != nil {
		return "", err
	}
	accept, err := w.writeExpression(s.Accept)
	if err != nil {
		return "", err
	}
	reject, err := w.writeExpression(s.Reject)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("(%s ? %s : %s)", condition, accept, reject), nil
}

// writeRelational writes a relational expression.
func (w *Writer) writeRelational(r ir.ExprRelational) (string, error) {
	argument, err := w.writeExpression(r.Argument)
	if err != nil {
		return "", err
	}

	switch r.Fun {
	case ir.RelationalAll:
		return fmt.Sprintf("all(%s)", argument), nil
	case ir.RelationalAny:
		return fmt.Sprintf("any(%s)", argument), nil
	case ir.RelationalIsNan:
		return fmt.Sprintf("isnan(%s)", argument), nil
	case ir.RelationalIsInf:
		return fmt.Sprintf("isinf(%s)", argument), nil
	default:
		return "", fmt.Errorf("unsupported relational function: %v", r.Fun)
	}
}

// writeMath writes a math function expression.
//
//nolint:gocyclo,cyclop,funlen,maintidx // Math functions require many cases
func (w *Writer) writeMath(m ir.ExprMath) (string, error) {
	// Collect arguments
	arg, err := w.writeExpression(m.Arg)
	if err != nil {
		return "", err
	}

	var args []string
	args = append(args, arg)

	if m.Arg1 != nil {
		a, err := w.writeExpression(*m.Arg1)
		if err != nil {
			return "", err
		}
		args = append(args, a)
	}
	if m.Arg2 != nil {
		a, err := w.writeExpression(*m.Arg2)
		if err != nil {
			return "", err
		}
		args = append(args, a)
	}
	if m.Arg3 != nil {
		a, err := w.writeExpression(*m.Arg3)
		if err != nil {
			return "", err
		}
		args = append(args, a)
	}

	argStr := strings.Join(args, ", ")

	switch m.Fun {
	// Trigonometric
	case ir.MathCos:
		return fmt.Sprintf("cos(%s)", argStr), nil
	case ir.MathCosh:
		return fmt.Sprintf("cosh(%s)", argStr), nil
	case ir.MathSin:
		return fmt.Sprintf("sin(%s)", argStr), nil
	case ir.MathSinh:
		return fmt.Sprintf("sinh(%s)", argStr), nil
	case ir.MathTan:
		return fmt.Sprintf("tan(%s)", argStr), nil
	case ir.MathTanh:
		return fmt.Sprintf("tanh(%s)", argStr), nil
	case ir.MathAcos:
		return fmt.Sprintf("acos(%s)", argStr), nil
	case ir.MathAsin:
		return fmt.Sprintf("asin(%s)", argStr), nil
	case ir.MathAtan:
		return fmt.Sprintf("atan(%s)", argStr), nil
	case ir.MathAtan2:
		return fmt.Sprintf("atan(%s)", argStr), nil // GLSL atan takes two args
	case ir.MathAsinh:
		return fmt.Sprintf("asinh(%s)", argStr), nil
	case ir.MathAcosh:
		return fmt.Sprintf("acosh(%s)", argStr), nil
	case ir.MathAtanh:
		return fmt.Sprintf("atanh(%s)", argStr), nil
	case ir.MathRadians:
		return fmt.Sprintf("radians(%s)", argStr), nil
	case ir.MathDegrees:
		return fmt.Sprintf("degrees(%s)", argStr), nil

	// Exponential
	case ir.MathExp:
		return fmt.Sprintf("exp(%s)", argStr), nil
	case ir.MathExp2:
		return fmt.Sprintf("exp2(%s)", argStr), nil
	case ir.MathLog:
		return fmt.Sprintf("log(%s)", argStr), nil
	case ir.MathLog2:
		return fmt.Sprintf("log2(%s)", argStr), nil
	case ir.MathPow:
		return fmt.Sprintf("pow(%s)", argStr), nil
	case ir.MathSqrt:
		return fmt.Sprintf("sqrt(%s)", argStr), nil
	case ir.MathInverseSqrt:
		return fmt.Sprintf("inversesqrt(%s)", argStr), nil

	// Common
	case ir.MathAbs:
		return fmt.Sprintf("abs(%s)", argStr), nil
	case ir.MathSign:
		return fmt.Sprintf("sign(%s)", argStr), nil
	case ir.MathFloor:
		return fmt.Sprintf("floor(%s)", argStr), nil
	case ir.MathCeil:
		return fmt.Sprintf("ceil(%s)", argStr), nil
	case ir.MathTrunc:
		return fmt.Sprintf("trunc(%s)", argStr), nil
	case ir.MathRound:
		return fmt.Sprintf("round(%s)", argStr), nil
	case ir.MathFract:
		return fmt.Sprintf("fract(%s)", argStr), nil
	case ir.MathMin:
		return fmt.Sprintf("min(%s)", argStr), nil
	case ir.MathMax:
		return fmt.Sprintf("max(%s)", argStr), nil
	case ir.MathClamp:
		return fmt.Sprintf("clamp(%s)", argStr), nil
	case ir.MathSaturate:
		return fmt.Sprintf("clamp(%s, 0.0, 1.0)", args[0]), nil
	case ir.MathMix:
		return fmt.Sprintf("mix(%s)", argStr), nil
	case ir.MathStep:
		return fmt.Sprintf("step(%s)", argStr), nil
	case ir.MathSmoothStep:
		return fmt.Sprintf("smoothstep(%s)", argStr), nil
	case ir.MathFma:
		return fmt.Sprintf("fma(%s)", argStr), nil

	// Geometric
	case ir.MathLength:
		return fmt.Sprintf("length(%s)", argStr), nil
	case ir.MathDistance:
		return fmt.Sprintf("distance(%s)", argStr), nil
	case ir.MathDot:
		return fmt.Sprintf("dot(%s)", argStr), nil
	case ir.MathCross:
		return fmt.Sprintf("cross(%s)", argStr), nil
	case ir.MathNormalize:
		return fmt.Sprintf("normalize(%s)", argStr), nil
	case ir.MathFaceForward:
		return fmt.Sprintf("faceforward(%s)", argStr), nil
	case ir.MathReflect:
		return fmt.Sprintf("reflect(%s)", argStr), nil
	case ir.MathRefract:
		return fmt.Sprintf("refract(%s)", argStr), nil

	// Matrix
	case ir.MathTranspose:
		return fmt.Sprintf("transpose(%s)", argStr), nil
	case ir.MathDeterminant:
		return fmt.Sprintf("determinant(%s)", argStr), nil
	case ir.MathInverse:
		return fmt.Sprintf("inverse(%s)", argStr), nil
	case ir.MathOuter:
		return fmt.Sprintf("outerProduct(%s)", argStr), nil

	// Bitwise
	case ir.MathCountOneBits:
		return fmt.Sprintf("bitCount(%s)", argStr), nil
	case ir.MathReverseBits:
		return fmt.Sprintf("bitfieldReverse(%s)", argStr), nil
	case ir.MathFirstLeadingBit:
		return fmt.Sprintf("findMSB(%s)", argStr), nil
	case ir.MathFirstTrailingBit:
		return fmt.Sprintf("findLSB(%s)", argStr), nil
	case ir.MathCountLeadingZeros:
		// GLSL doesn't have direct clz, use workaround
		return fmt.Sprintf("(31 - findMSB(%s))", args[0]), nil
	case ir.MathCountTrailingZeros:
		// GLSL doesn't have direct ctz, use findLSB
		return fmt.Sprintf("findLSB(%s)", argStr), nil
	case ir.MathExtractBits:
		return fmt.Sprintf("bitfieldExtract(%s)", argStr), nil
	case ir.MathInsertBits:
		return fmt.Sprintf("bitfieldInsert(%s)", argStr), nil

	// Pack/Unpack
	case ir.MathPack4x8snorm:
		return fmt.Sprintf("packSnorm4x8(%s)", argStr), nil
	case ir.MathPack4x8unorm:
		return fmt.Sprintf("packUnorm4x8(%s)", argStr), nil
	case ir.MathPack2x16snorm:
		return fmt.Sprintf("packSnorm2x16(%s)", argStr), nil
	case ir.MathPack2x16unorm:
		return fmt.Sprintf("packUnorm2x16(%s)", argStr), nil
	case ir.MathPack2x16float:
		return fmt.Sprintf("packHalf2x16(%s)", argStr), nil
	case ir.MathUnpack4x8snorm:
		return fmt.Sprintf("unpackSnorm4x8(%s)", argStr), nil
	case ir.MathUnpack4x8unorm:
		return fmt.Sprintf("unpackUnorm4x8(%s)", argStr), nil
	case ir.MathUnpack2x16snorm:
		return fmt.Sprintf("unpackSnorm2x16(%s)", argStr), nil
	case ir.MathUnpack2x16unorm:
		return fmt.Sprintf("unpackUnorm2x16(%s)", argStr), nil
	case ir.MathUnpack2x16float:
		return fmt.Sprintf("unpackHalf2x16(%s)", argStr), nil

	default:
		return "", fmt.Errorf("unsupported math function: %v", m.Fun)
	}
}

// writeDerivative writes a derivative expression.
func (w *Writer) writeDerivative(d ir.ExprDerivative) (string, error) {
	expr, err := w.writeExpression(d.Expr)
	if err != nil {
		return "", err
	}

	switch d.Axis {
	case ir.DerivativeX:
		switch d.Control {
		case ir.DerivativeCoarse:
			return fmt.Sprintf("dFdxCoarse(%s)", expr), nil
		case ir.DerivativeFine:
			return fmt.Sprintf("dFdxFine(%s)", expr), nil
		default:
			return fmt.Sprintf("dFdx(%s)", expr), nil
		}
	case ir.DerivativeY:
		switch d.Control {
		case ir.DerivativeCoarse:
			return fmt.Sprintf("dFdyCoarse(%s)", expr), nil
		case ir.DerivativeFine:
			return fmt.Sprintf("dFdyFine(%s)", expr), nil
		default:
			return fmt.Sprintf("dFdy(%s)", expr), nil
		}
	case ir.DerivativeWidth:
		switch d.Control {
		case ir.DerivativeCoarse:
			return fmt.Sprintf("fwidthCoarse(%s)", expr), nil
		case ir.DerivativeFine:
			return fmt.Sprintf("fwidthFine(%s)", expr), nil
		default:
			return fmt.Sprintf("fwidth(%s)", expr), nil
		}
	default:
		return "", fmt.Errorf("unsupported derivative axis: %v", d.Axis)
	}
}

// writeImageSample writes an image sample expression.
func (w *Writer) writeImageSample(s ir.ExprImageSample) (string, error) {
	image, err := w.writeExpression(s.Image)
	if err != nil {
		return "", err
	}
	sampler, err := w.writeExpression(s.Sampler)
	if err != nil {
		return "", err
	}
	coordinate, err := w.writeExpression(s.Coordinate)
	if err != nil {
		return "", err
	}

	// In GLSL, textures and samplers are combined
	combinedName := fmt.Sprintf("%s_%s", image, sampler)
	w.textureSamplerPairs = append(w.textureSamplerPairs, combinedName)

	// Handle different sampling modes based on Level type
	switch level := s.Level.(type) {
	case ir.SampleLevelExact:
		levelExpr, err := w.writeExpression(level.Level)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("textureLod(%s, %s, %s)", combinedName, coordinate, levelExpr), nil
	case ir.SampleLevelBias:
		biasExpr, err := w.writeExpression(level.Bias)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("texture(%s, %s, %s)", combinedName, coordinate, biasExpr), nil
	case ir.SampleLevelGradient:
		gradX, err := w.writeExpression(level.X)
		if err != nil {
			return "", err
		}
		gradY, err := w.writeExpression(level.Y)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("textureGrad(%s, %s, %s, %s)", combinedName, coordinate, gradX, gradY), nil
	case ir.SampleLevelZero:
		return fmt.Sprintf("textureLod(%s, %s, 0.0)", combinedName, coordinate), nil
	default:
		// SampleLevelAuto or nil - implicit LOD
		return fmt.Sprintf("texture(%s, %s)", combinedName, coordinate), nil
	}
}

// writeImageLoad writes an image load expression.
func (w *Writer) writeImageLoad(l ir.ExprImageLoad) (string, error) {
	image, err := w.writeExpression(l.Image)
	if err != nil {
		return "", err
	}
	coordinate, err := w.writeExpression(l.Coordinate)
	if err != nil {
		return "", err
	}

	if l.Level != nil {
		level, err := w.writeExpression(*l.Level)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("texelFetch(%s, %s, %s)", image, coordinate, level), nil
	}

	if l.Sample != nil {
		sample, err := w.writeExpression(*l.Sample)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("texelFetch(%s, %s, %s)", image, coordinate, sample), nil
	}

	return fmt.Sprintf("imageLoad(%s, %s)", image, coordinate), nil
}

// writeImageQuery writes an image query expression.
func (w *Writer) writeImageQuery(q ir.ExprImageQuery) (string, error) {
	image, err := w.writeExpression(q.Image)
	if err != nil {
		return "", err
	}

	switch query := q.Query.(type) {
	case ir.ImageQuerySize:
		if query.Level != nil {
			level, err := w.writeExpression(*query.Level)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("textureSize(%s, %s)", image, level), nil
		}
		return fmt.Sprintf("textureSize(%s, 0)", image), nil
	case ir.ImageQueryNumLevels:
		return fmt.Sprintf("textureQueryLevels(%s)", image), nil
	case ir.ImageQueryNumLayers:
		return fmt.Sprintf("textureSize(%s, 0).z", image), nil
	case ir.ImageQueryNumSamples:
		return fmt.Sprintf("textureSamples(%s)", image), nil
	default:
		return "", fmt.Errorf("unsupported image query: %T", q.Query)
	}
}

// writeAs writes a type cast expression.
func (w *Writer) writeAs(a ir.ExprAs) (string, error) {
	expr, err := w.writeExpression(a.Expr)
	if err != nil {
		return "", err
	}

	if a.Convert == nil {
		// Bitcast
		typeName := w.scalarKindToGLSL(a.Kind)
		return fmt.Sprintf("%sBitsTo%s(%s)", w.getInverseScalarKind(a.Kind), typeName, expr), nil
	}

	// Regular conversion
	typeName := w.scalarKindToGLSL(a.Kind)
	return fmt.Sprintf("%s(%s)", typeName, expr), nil
}

// writeCallResult writes a call result expression.
func (w *Writer) writeCallResult(c ir.ExprCallResult) (string, error) {
	// Call results are stored in named expressions by writeCallStatement
	name := w.names[nameKey{kind: nameKeyFunction, handle1: uint32(c.Function)}]
	if name == "" {
		return fmt.Sprintf("call_result_%d", c.Function), nil
	}
	return name, nil
}

// writeAtomicResult writes an atomic result expression.
func (w *Writer) writeAtomicResult(_ ir.ExprAtomicResult) (string, error) {
	// Atomic results are handled by the atomic statement
	return "/* atomic result */", nil
}

// writeArrayLength writes an array length expression.
func (w *Writer) writeArrayLength(a ir.ExprArrayLength) (string, error) {
	expr, err := w.writeExpression(a.Array)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.length()", expr), nil
}

// scalarKindToGLSL converts a scalar kind to GLSL type name.
func (w *Writer) scalarKindToGLSL(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarBool:
		return "bool"
	case ir.ScalarSint:
		return glslTypeInt
	case ir.ScalarUint:
		return glslTypeUint
	case ir.ScalarFloat:
		return glslTypeFloat
	default:
		return glslTypeInt
	}
}

// getInverseScalarKind returns the source type for bitcast operations.
func (w *Writer) getInverseScalarKind(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarSint, ir.ScalarUint:
		return glslTypeFloat
	case ir.ScalarFloat:
		return glslTypeInt
	default:
		return glslTypeInt
	}
}

// getExpressionTypeHandle attempts to get the type handle of an expression.
func (w *Writer) getExpressionTypeHandle(kind ir.ExpressionKind) *ir.TypeHandle {
	switch k := kind.(type) {
	case ir.ExprLocalVariable:
		if w.currentFunction != nil && int(k.Variable) < len(w.currentFunction.LocalVars) {
			return &w.currentFunction.LocalVars[k.Variable].Type
		}
	case ir.ExprGlobalVariable:
		if int(k.Variable) < len(w.module.GlobalVariables) {
			return &w.module.GlobalVariables[k.Variable].Type
		}
	case ir.ExprCompose:
		return &k.Type
	case ir.ExprZeroValue:
		return &k.Type
	}
	return nil
}
