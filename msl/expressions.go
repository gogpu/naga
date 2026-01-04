package msl

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/ir"
)

// Type name constants for MSL codegen
const (
	mslFloat = "float"
	mslHalf  = "half"
	mslInt   = "int"
	mslUint  = "uint"
	mslBool  = "bool"
)

// scalarCastTypeName returns the MSL type name for a scalar cast.
func (w *Writer) scalarCastTypeName(kind ir.ScalarKind, convert *uint8) string {
	switch kind {
	case ir.ScalarFloat:
		if convert != nil && *convert == 2 {
			return mslHalf
		}
		return mslFloat
	case ir.ScalarSint:
		return mslInt
	case ir.ScalarUint:
		return mslUint
	case ir.ScalarBool:
		return mslBool
	default:
		return mslInt
	}
}

// writeExpression writes an expression to the output.
// If the expression is already named (assigned to a variable), just writes the name.
func (w *Writer) writeExpression(handle ir.ExpressionHandle) error {
	// Check if this expression has been named
	if name, ok := w.namedExpressions[handle]; ok {
		w.write("%s", name)
		return nil
	}

	// Otherwise write inline
	return w.writeExpressionInline(handle)
}

// writeExpressionInline writes an expression inline without looking up names.
func (w *Writer) writeExpressionInline(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil {
		return fmt.Errorf("no current function context")
	}

	if int(handle) >= len(w.currentFunction.Expressions) {
		return fmt.Errorf("invalid expression handle: %d", handle)
	}

	expr := &w.currentFunction.Expressions[handle]
	return w.writeExpressionKind(expr.Kind, handle)
}

// writeExpressionKind writes the expression based on its kind.
//
//nolint:gocyclo,cyclop // Expression dispatch requires handling all expression kinds
func (w *Writer) writeExpressionKind(kind ir.ExpressionKind, handle ir.ExpressionHandle) error {
	switch k := kind.(type) {
	case ir.Literal:
		return w.writeLiteral(k)

	case ir.ExprConstant:
		name := w.getName(nameKey{kind: nameKeyConstant, handle1: uint32(k.Constant)})
		w.write("%s", name)
		return nil

	case ir.ExprZeroValue:
		return w.writeZeroValue(k.Type)

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
		return w.writeBinary(k, handle)

	case ir.ExprSelect:
		return w.writeSelect(k)

	case ir.ExprMath:
		return w.writeMath(k)

	case ir.ExprAs:
		return w.writeAs(k)

	case ir.ExprImageSample:
		return w.writeImageSample(k)

	case ir.ExprImageLoad:
		return w.writeImageLoad(k)

	case ir.ExprImageQuery:
		return w.writeImageQuery(k)

	case ir.ExprDerivative:
		return w.writeDerivative(k)

	case ir.ExprRelational:
		return w.writeRelational(k)

	case ir.ExprCallResult:
		// Call results are handled by the Call statement
		w.write("/* call result */")
		return nil

	case ir.ExprArrayLength:
		return w.writeArrayLength(k)

	case ir.ExprAtomicResult:
		// Atomic results are handled by the Atomic statement
		w.write("/* atomic result */")
		return nil

	default:
		return fmt.Errorf("unsupported expression kind: %T", kind)
	}
}

// writeLiteral writes a literal value.
func (w *Writer) writeLiteral(lit ir.Literal) error {
	switch v := lit.Value.(type) {
	case ir.LiteralBool:
		if v {
			w.write("true")
		} else {
			w.write("false")
		}

	case ir.LiteralI32:
		w.write("%d", int32(v))

	case ir.LiteralU32:
		w.write("%du", uint32(v))

	case ir.LiteralI64:
		w.write("%dL", int64(v))

	case ir.LiteralU64:
		w.write("%duL", uint64(v))

	case ir.LiteralF32:
		val := float32(v)
		if val == float32(int32(val)) {
			w.write("%.1f", val)
		} else {
			w.write("%g", val)
		}

	case ir.LiteralF64:
		val := float64(v)
		w.write("%g", val)

	case ir.LiteralAbstractInt:
		w.write("%d", int64(v))

	case ir.LiteralAbstractFloat:
		val := float64(v)
		w.write("%g", val)

	default:
		return fmt.Errorf("unsupported literal type: %T", lit.Value)
	}
	return nil
}

// writeZeroValue writes a zero-initialized value.
func (w *Writer) writeZeroValue(typeHandle ir.TypeHandle) error {
	typeName := w.writeTypeName(typeHandle, StorageAccess(0))
	w.write("%s()", typeName)
	return nil
}

// writeCompose writes a composite construction.
func (w *Writer) writeCompose(compose ir.ExprCompose) error {
	typeName := w.writeTypeName(compose.Type, StorageAccess(0))
	useBraces := false
	isArrayWrapper := false
	if _, ok := w.arrayWrappers[compose.Type]; ok {
		useBraces = true
		isArrayWrapper = true
	} else if int(compose.Type) < len(w.module.Types) {
		if _, ok := w.module.Types[compose.Type].Inner.(ir.StructType); ok {
			useBraces = true
		}
	}

	if useBraces {
		w.write("%s{", typeName)
		if isArrayWrapper {
			w.write("{")
		}
	} else {
		w.write("%s(", typeName)
	}
	for i, component := range compose.Components {
		if i > 0 {
			w.write(", ")
		}
		if err := w.writeExpression(component); err != nil {
			return err
		}
	}
	if useBraces {
		if isArrayWrapper {
			w.write("}")
		}
		w.write("}")
	} else {
		w.write(")")
	}
	return nil
}

// writeAccess writes a dynamic index access.
//
//nolint:nestif,gocognit // Type/array-wrapper cases require nested checks
func (w *Writer) writeAccess(access ir.ExprAccess) error {
	baseType := w.getExpressionType(access.Base)
	if baseType != nil {
		if pt, ok := baseType.(ir.PointerType); ok {
			if _, ok := w.arrayWrappers[pt.Base]; ok {
				if w.pointerNeedsDeref(pt) {
					w.write("(*")
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					w.write(").inner[")
				} else {
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					w.write(".inner[")
				}
				if err := w.writeExpression(access.Index); err != nil {
					return err
				}
				w.write("]")
				return nil
			}

			if w.pointerNeedsDeref(pt) {
				w.write("(*")
				if err := w.writeExpression(access.Base); err != nil {
					return err
				}
				w.write(")")
			} else {
				if err := w.writeExpression(access.Base); err != nil {
					return err
				}
			}
			w.write("[")
			if err := w.writeExpression(access.Index); err != nil {
				return err
			}
			w.write("]")
			return nil
		}
	}

	if baseHandle := w.getExpressionTypeHandle(access.Base); baseHandle != nil {
		if _, ok := w.arrayWrappers[*baseHandle]; ok {
			if err := w.writeExpression(access.Base); err != nil {
				return err
			}
			w.write(".inner[")
			if err := w.writeExpression(access.Index); err != nil {
				return err
			}
			w.write("]")
			return nil
		}
	}

	if err := w.writeExpression(access.Base); err != nil {
		return err
	}
	w.write("[")
	if err := w.writeExpression(access.Index); err != nil {
		return err
	}
	w.write("]")
	return nil
}

// writeAccessIndex writes a constant index access.
//
//nolint:nestif,gocognit,gocyclo,cyclop // Struct member access requires nested type checking
func (w *Writer) writeAccessIndex(access ir.ExprAccessIndex) error {
	// Get base type to determine if this is struct access or array/vector access
	baseType := w.getExpressionType(access.Base)
	if baseType != nil {
		if pt, ok := baseType.(ir.PointerType); ok {
			if _, ok := w.arrayWrappers[pt.Base]; ok {
				if w.pointerNeedsDeref(pt) {
					w.write("(*")
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					w.write(").inner[%d]", access.Index)
				} else {
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					w.write(".inner[%d]", access.Index)
				}
				return nil
			}

			if int(pt.Base) < len(w.module.Types) {
				if st, ok := w.module.Types[pt.Base].Inner.(ir.StructType); ok {
					_ = st
					if w.pointerNeedsDeref(pt) {
						w.write("(*")
						if err := w.writeExpression(access.Base); err != nil {
							return err
						}
						w.write(")")
					} else {
						if err := w.writeExpression(access.Base); err != nil {
							return err
						}
					}

					memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(pt.Base), handle2: access.Index})
					w.write(".%s", memberName)
					return nil
				}
			}

			if w.pointerNeedsDeref(pt) {
				w.write("(*")
				if err := w.writeExpression(access.Base); err != nil {
					return err
				}
				w.write(")")
			} else {
				if err := w.writeExpression(access.Base); err != nil {
					return err
				}
			}
			w.write("[%d]", access.Index)
			return nil
		}
	}
	if baseHandle := w.getExpressionTypeHandle(access.Base); baseHandle != nil {
		if _, ok := w.arrayWrappers[*baseHandle]; ok {
			if err := w.writeExpression(access.Base); err != nil {
				return err
			}
			w.write(".inner[%d]", access.Index)
			return nil
		}
	}
	if baseType != nil {
		if st, ok := baseType.(ir.StructType); ok {
			// Struct member access
			if err := w.writeExpression(access.Base); err != nil {
				return err
			}
			// Get member name
			typeHandle := w.getExpressionTypeHandle(access.Base)
			if typeHandle != nil {
				memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(*typeHandle), handle2: access.Index})
				w.write(".%s", memberName)
				return nil
			}
			// Fallback
			if int(access.Index) < len(st.Members) {
				w.write(".%s", escapeName(st.Members[access.Index].Name))
			} else {
				w.write(".member_%d", access.Index)
			}
			return nil
		}
	}

	// Array, vector, or matrix access
	if err := w.writeExpression(access.Base); err != nil {
		return err
	}
	w.write("[%d]", access.Index)
	return nil
}

// writeSplat writes a vector splat.
func (w *Writer) writeSplat(splat ir.ExprSplat) error {
	// In MSL, we can construct a vector from a single scalar
	scalarType := w.getExpressionScalarType(splat.Value)
	if scalarType != nil {
		typeName := fmt.Sprintf("%s%s%d", Namespace, scalarTypeName(*scalarType), splat.Size)
		w.write("%s(", typeName)
		if err := w.writeExpression(splat.Value); err != nil {
			return err
		}
		w.write(")")
		return nil
	}

	// Fallback
	w.write("/* splat */ (")
	if err := w.writeExpression(splat.Value); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeSwizzle writes a vector swizzle.
func (w *Writer) writeSwizzle(swizzle ir.ExprSwizzle) error {
	if err := w.writeExpression(swizzle.Vector); err != nil {
		return err
	}
	w.write(".")
	components := "xyzw"
	for i := ir.VectorSize(0); i < swizzle.Size; i++ {
		w.write("%c", components[swizzle.Pattern[i]])
	}
	return nil
}

// writeFunctionArgument writes a function argument reference.
func (w *Writer) writeFunctionArgument(arg ir.ExprFunctionArgument) error {
	name := w.getName(nameKey{kind: nameKeyFunctionArgument, handle1: uint32(w.currentFuncHandle), handle2: arg.Index})
	w.write("%s", name)
	return nil
}

// writeGlobalVariable writes a global variable reference.
func (w *Writer) writeGlobalVariable(global ir.ExprGlobalVariable) error {
	name := w.getName(nameKey{kind: nameKeyGlobalVariable, handle1: uint32(global.Variable)})
	w.write("%s", name)
	return nil
}

// writeLocalVariable writes a local variable reference.
func (w *Writer) writeLocalVariable(local ir.ExprLocalVariable) error {
	if name, ok := w.localNames[local.Variable]; ok {
		w.write("%s", name)
		return nil
	}
	w.write("local_%d", local.Variable)
	return nil
}

// writeLoad writes a pointer load.
func (w *Writer) writeLoad(load ir.ExprLoad) error {
	if !w.shouldDerefPointer(load.Pointer) {
		return w.writeExpression(load.Pointer)
	}

	w.write("(*")
	if err := w.writeExpression(load.Pointer); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeUnary writes a unary operation.
func (w *Writer) writeUnary(unary ir.ExprUnary) error {
	var op string
	switch unary.Op {
	case ir.UnaryNegate:
		op = "-"
	case ir.UnaryLogicalNot:
		op = "!"
	case ir.UnaryBitwiseNot:
		op = "~"
	default:
		return fmt.Errorf("unsupported unary operator: %v", unary.Op)
	}

	w.write("%s(", op)
	if err := w.writeExpression(unary.Expr); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeBinary writes a binary operation.
//
//nolint:gocyclo,cyclop,funlen // Binary operations require handling all operators
func (w *Writer) writeBinary(binary ir.ExprBinary, _ ir.ExpressionHandle) error {
	// Handle special cases
	switch binary.Op {
	case ir.BinaryDivide:
		// Use safe division helper for integers
		if w.isIntegerExpression(binary.Left) {
			w.write("_naga_div(")
			if err := w.writeExpression(binary.Left); err != nil {
				return err
			}
			w.write(", ")
			if err := w.writeExpression(binary.Right); err != nil {
				return err
			}
			w.write(")")
			w.needsDivHelper = true
			return nil
		}

	case ir.BinaryModulo:
		// Use safe modulo helper for integers
		if w.isIntegerExpression(binary.Left) {
			w.write("_naga_mod(")
			if err := w.writeExpression(binary.Left); err != nil {
				return err
			}
			w.write(", ")
			if err := w.writeExpression(binary.Right); err != nil {
				return err
			}
			w.write(")")
			w.needsModHelper = true
			return nil
		}
	}

	// Regular binary operation
	var op string
	switch binary.Op {
	case ir.BinaryAdd:
		op = "+"
	case ir.BinarySubtract:
		op = "-"
	case ir.BinaryMultiply:
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
		return fmt.Errorf("unsupported binary operator: %v", binary.Op)
	}

	w.write("(")
	if err := w.writeExpression(binary.Left); err != nil {
		return err
	}
	w.write(" %s ", op)
	if err := w.writeExpression(binary.Right); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeSelect writes a select (ternary) operation.
func (w *Writer) writeSelect(sel ir.ExprSelect) error {
	w.write("(")
	if err := w.writeExpression(sel.Condition); err != nil {
		return err
	}
	w.write(" ? ")
	if err := w.writeExpression(sel.Accept); err != nil {
		return err
	}
	w.write(" : ")
	if err := w.writeExpression(sel.Reject); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeMath writes a math function call.
//
//nolint:gocognit // Math function dispatch requires special case handling
func (w *Writer) writeMath(mathExpr ir.ExprMath) error {
	funcName := mathFunctionName(mathExpr.Fun)

	// Handle special cases that don't follow the standard pattern
	switch mathExpr.Fun {
	case ir.MathDot:
		// dot(a, b)
		w.write("%sdot(", Namespace)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		if mathExpr.Arg1 != nil {
			w.write(", ")
			if err := w.writeExpression(*mathExpr.Arg1); err != nil {
				return err
			}
		}
		w.write(")")
		return nil

	case ir.MathCross:
		w.write("%scross(", Namespace)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		if mathExpr.Arg1 != nil {
			w.write(", ")
			if err := w.writeExpression(*mathExpr.Arg1); err != nil {
				return err
			}
		}
		w.write(")")
		return nil

	case ir.MathOuter:
		// Matrix outer product: outer(a, b) becomes the component-wise construction
		// For simplicity, we'll use a helper or expand
		w.write("/* outer product */ (")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(" * ")
		if mathExpr.Arg1 != nil {
			if err := w.writeExpression(*mathExpr.Arg1); err != nil {
				return err
			}
		}
		w.write(")")
		return nil
	}

	// Standard function call
	w.write("%s%s(", Namespace, funcName)
	if err := w.writeExpression(mathExpr.Arg); err != nil {
		return err
	}
	if mathExpr.Arg1 != nil {
		w.write(", ")
		if err := w.writeExpression(*mathExpr.Arg1); err != nil {
			return err
		}
	}
	if mathExpr.Arg2 != nil {
		w.write(", ")
		if err := w.writeExpression(*mathExpr.Arg2); err != nil {
			return err
		}
	}
	if mathExpr.Arg3 != nil {
		w.write(", ")
		if err := w.writeExpression(*mathExpr.Arg3); err != nil {
			return err
		}
	}
	w.write(")")
	return nil
}

// mathFunctionName returns the MSL name for a math function.
//
//nolint:gocyclo,cyclop,funlen // Math function lookup requires handling all functions
func mathFunctionName(fun ir.MathFunction) string {
	switch fun {
	case ir.MathAbs:
		return "abs"
	case ir.MathMin:
		return "min"
	case ir.MathMax:
		return "max"
	case ir.MathClamp:
		return "clamp"
	case ir.MathSaturate:
		return "saturate"
	case ir.MathCos:
		return "cos"
	case ir.MathCosh:
		return "cosh"
	case ir.MathSin:
		return "sin"
	case ir.MathSinh:
		return "sinh"
	case ir.MathTan:
		return "tan"
	case ir.MathTanh:
		return "tanh"
	case ir.MathAcos:
		return "acos"
	case ir.MathAsin:
		return "asin"
	case ir.MathAtan:
		return "atan"
	case ir.MathAtan2:
		return "atan2"
	case ir.MathAsinh:
		return "asinh"
	case ir.MathAcosh:
		return "acosh"
	case ir.MathAtanh:
		return "atanh"
	case ir.MathRadians:
		return "radians"
	case ir.MathDegrees:
		return "degrees"
	case ir.MathCeil:
		return "ceil"
	case ir.MathFloor:
		return "floor"
	case ir.MathRound:
		return "round"
	case ir.MathFract:
		return "fract"
	case ir.MathTrunc:
		return "trunc"
	case ir.MathExp:
		return "exp"
	case ir.MathExp2:
		return "exp2"
	case ir.MathLog:
		return "log"
	case ir.MathLog2:
		return "log2"
	case ir.MathPow:
		return "pow"
	case ir.MathDot:
		return "dot"
	case ir.MathCross:
		return "cross"
	case ir.MathDistance:
		return "distance"
	case ir.MathLength:
		return "length"
	case ir.MathNormalize:
		return "normalize"
	case ir.MathFaceForward:
		return "faceforward"
	case ir.MathReflect:
		return "reflect"
	case ir.MathRefract:
		return "refract"
	case ir.MathSign:
		return "sign"
	case ir.MathFma:
		return "fma"
	case ir.MathMix:
		return "mix"
	case ir.MathStep:
		return "step"
	case ir.MathSmoothStep:
		return "smoothstep"
	case ir.MathSqrt:
		return "sqrt"
	case ir.MathInverseSqrt:
		return "rsqrt"
	case ir.MathTranspose:
		return "transpose"
	case ir.MathDeterminant:
		return "determinant"
	case ir.MathCountTrailingZeros:
		return "ctz"
	case ir.MathCountLeadingZeros:
		return "clz"
	case ir.MathCountOneBits:
		return "popcount"
	case ir.MathReverseBits:
		return "reverse_bits"
	case ir.MathExtractBits:
		return "extract_bits"
	case ir.MathInsertBits:
		return "insert_bits"
	default:
		return fmt.Sprintf("unknown_math_%d", fun)
	}
}

// writeAs writes a type cast/conversion.
func (w *Writer) writeAs(as ir.ExprAs) error {
	// Determine target type based on scalar kind and width
	typeName := w.scalarCastTypeName(as.Kind, as.Convert)

	if as.Convert != nil {
		// Type conversion
		w.write("%s(", typeName)
		if err := w.writeExpression(as.Expr); err != nil {
			return err
		}
		w.write(")")
	} else {
		// Bitcast
		w.write("as_type<%s>(", typeName)
		if err := w.writeExpression(as.Expr); err != nil {
			return err
		}
		w.write(")")
	}
	return nil
}

// writeImageSample writes a texture sample operation.
//
//nolint:gocyclo,cyclop // Image sampling has many parameters to handle
func (w *Writer) writeImageSample(sample ir.ExprImageSample) error {
	if err := w.writeExpression(sample.Image); err != nil {
		return err
	}

	// Determine sampling method
	switch {
	case sample.DepthRef != nil:
		// Depth comparison sample
		w.write(".sample_compare(")
	case sample.Gather != nil:
		// Gather operation
		w.write(".gather(")
	default:
		// Regular sample
		w.write(".sample(")
	}

	// Sampler
	if err := w.writeExpression(sample.Sampler); err != nil {
		return err
	}

	// Coordinate
	w.write(", ")
	if err := w.writeExpression(sample.Coordinate); err != nil {
		return err
	}

	// Array index
	if sample.ArrayIndex != nil {
		w.write(", ")
		if err := w.writeExpression(*sample.ArrayIndex); err != nil {
			return err
		}
	}

	// Depth reference
	if sample.DepthRef != nil {
		w.write(", ")
		if err := w.writeExpression(*sample.DepthRef); err != nil {
			return err
		}
	}

	// Level of detail
	switch level := sample.Level.(type) {
	case ir.SampleLevelAuto:
		// Default, no argument needed
	case ir.SampleLevelZero:
		w.write(", %slevel(0.0)", Namespace)
	case ir.SampleLevelExact:
		w.write(", %slevel(", Namespace)
		if err := w.writeExpression(level.Level); err != nil {
			return err
		}
		w.write(")")
	case ir.SampleLevelBias:
		w.write(", %sbias(", Namespace)
		if err := w.writeExpression(level.Bias); err != nil {
			return err
		}
		w.write(")")
	case ir.SampleLevelGradient:
		w.write(", %sgradient2d(", Namespace)
		if err := w.writeExpression(level.X); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(level.Y); err != nil {
			return err
		}
		w.write(")")
	}

	// Offset
	if sample.Offset != nil {
		w.write(", ")
		if err := w.writeExpression(*sample.Offset); err != nil {
			return err
		}
	}

	w.write(")")
	return nil
}

// writeImageLoad writes a texture load operation.
func (w *Writer) writeImageLoad(load ir.ExprImageLoad) error {
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".read(")

	// Coordinate (converted to uint)
	w.write("uint2(")
	if err := w.writeExpression(load.Coordinate); err != nil {
		return err
	}
	w.write(")")

	// Array index
	if load.ArrayIndex != nil {
		w.write(", ")
		if err := w.writeExpression(*load.ArrayIndex); err != nil {
			return err
		}
	}

	// Sample (for multisampled)
	if load.Sample != nil {
		w.write(", ")
		if err := w.writeExpression(*load.Sample); err != nil {
			return err
		}
	}

	// Level
	if load.Level != nil {
		w.write(", ")
		if err := w.writeExpression(*load.Level); err != nil {
			return err
		}
	}

	w.write(")")
	return nil
}

// writeImageQuery writes an image query operation.
func (w *Writer) writeImageQuery(query ir.ExprImageQuery) error {
	switch q := query.Query.(type) {
	case ir.ImageQuerySize:
		if err := w.writeExpression(query.Image); err != nil {
			return err
		}
		w.write(".get_width(")
		if q.Level != nil {
			if err := w.writeExpression(*q.Level); err != nil {
				return err
			}
		}
		w.write(")")
		// Note: This only gets width. For full size, we'd need to construct a vector
		// with width, height, etc.

	case ir.ImageQueryNumLevels:
		if err := w.writeExpression(query.Image); err != nil {
			return err
		}
		w.write(".get_num_mip_levels()")

	case ir.ImageQueryNumLayers:
		if err := w.writeExpression(query.Image); err != nil {
			return err
		}
		w.write(".get_array_size()")

	case ir.ImageQueryNumSamples:
		if err := w.writeExpression(query.Image); err != nil {
			return err
		}
		w.write(".get_num_samples()")

	default:
		return fmt.Errorf("unsupported image query: %T", query.Query)
	}
	return nil
}

// writeDerivative writes a derivative operation.
func (w *Writer) writeDerivative(deriv ir.ExprDerivative) error {
	var funcName string
	switch deriv.Axis {
	case ir.DerivativeX:
		switch deriv.Control {
		case ir.DerivativeFine:
			funcName = "dfdx_fine"
		case ir.DerivativeCoarse:
			funcName = "dfdx_coarse"
		default:
			funcName = "dfdx"
		}
	case ir.DerivativeY:
		switch deriv.Control {
		case ir.DerivativeFine:
			funcName = "dfdy_fine"
		case ir.DerivativeCoarse:
			funcName = "dfdy_coarse"
		default:
			funcName = "dfdy"
		}
	case ir.DerivativeWidth:
		funcName = "fwidth"
	}

	w.write("%s%s(", Namespace, funcName)
	if err := w.writeExpression(deriv.Expr); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeRelational writes a relational function.
func (w *Writer) writeRelational(rel ir.ExprRelational) error {
	var funcName string
	switch rel.Fun {
	case ir.RelationalAll:
		funcName = "all"
	case ir.RelationalAny:
		funcName = "any"
	case ir.RelationalIsNan:
		funcName = "isnan"
	case ir.RelationalIsInf:
		funcName = "isinf"
	}

	w.write("%s%s(", Namespace, funcName)
	if err := w.writeExpression(rel.Argument); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeArrayLength writes an array length query.
func (w *Writer) writeArrayLength(_ ir.ExprArrayLength) error {
	// For runtime-sized arrays, we need the sizes buffer
	// This is a simplified implementation
	w.write("/* array length */ 0")
	w.needsSizesBuffer = true
	return nil
}

// Helper methods for type information

// getExpressionType returns the type of an expression.
// getExpressionType returns the type of an expression.
// Returns the TypeInner or nil if not found.
func (w *Writer) getExpressionType(handle ir.ExpressionHandle) ir.TypeInner {
	if w.currentFunction == nil {
		return nil
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return nil
	}
	resolution := &w.currentFunction.ExpressionTypes[handle]
	if resolution.Handle != nil {
		if int(*resolution.Handle) < len(w.module.Types) {
			return w.module.Types[*resolution.Handle].Inner
		}
	}
	if resolution.Value != nil {
		return resolution.Value
	}
	return nil
}

// getExpressionTypeHandle returns the type handle of an expression.
func (w *Writer) getExpressionTypeHandle(handle ir.ExpressionHandle) *ir.TypeHandle {
	if w.currentFunction == nil {
		return nil
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return nil
	}
	return w.currentFunction.ExpressionTypes[handle].Handle
}

// getExpressionScalarType returns the scalar type of an expression (for scalars and vectors).
func (w *Writer) getExpressionScalarType(handle ir.ExpressionHandle) *ir.ScalarType {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return nil
	}
	switch t := typeInner.(type) {
	case ir.ScalarType:
		return &t
	case ir.VectorType:
		return &t.Scalar
	}
	return nil
}

// isIntegerExpression checks if an expression is an integer type.
func (w *Writer) isIntegerExpression(handle ir.ExpressionHandle) bool {
	scalar := w.getExpressionScalarType(handle)
	if scalar == nil {
		return false
	}
	return scalar.Kind == ir.ScalarSint || scalar.Kind == ir.ScalarUint
}

func (w *Writer) pointerNeedsDeref(pt ir.PointerType) bool {
	switch pt.Space {
	case ir.SpaceUniform, ir.SpaceStorage, ir.SpacePushConstant:
		return true
	default:
		return false
	}
}

func (w *Writer) shouldDerefPointer(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	switch w.currentFunction.Expressions[handle].Kind.(type) {
	case ir.ExprAccess, ir.ExprAccessIndex:
		return false
	}
	if pt, ok := w.getExpressionType(handle).(ir.PointerType); ok {
		return w.pointerNeedsDeref(pt)
	}
	return false
}

// Unused but included for completeness
var _ = math.Float32frombits
