// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"
	"strings"

	"github.com/gogpu/naga/ir"
)

// isUnsigned checks if an expression resolves to an unsigned integer type.
// Returns (vecSize, true) where vecSize is 0 for scalar, 2/3/4 for vectors.
// Returns (0, false) if not unsigned.
func (w *Writer) isUnsignedExpr(handle ir.ExpressionHandle) (int, bool) {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return 0, false
	}
	res := &w.currentFunction.ExpressionTypes[handle]
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner := w.module.Types[*res.Handle].Inner
		switch t := inner.(type) {
		case ir.ScalarType:
			if t.Kind == ir.ScalarUint {
				return 0, true
			}
		case ir.VectorType:
			if t.Scalar.Kind == ir.ScalarUint {
				return int(t.Size), true
			}
		}
	} else if res.Value != nil {
		switch t := res.Value.(type) {
		case ir.ScalarType:
			if t.Kind == ir.ScalarUint {
				return 0, true
			}
		case ir.VectorType:
			if t.Scalar.Kind == ir.ScalarUint {
				return int(t.Size), true
			}
		}
	}
	return 0, false
}

// wrapUintCast wraps an expression in uint()/uvecN() if the argument is unsigned.
// GLSL functions like findLSB/findMSB/bitCount always return signed ints,
// so when the argument is unsigned, the result needs casting back to unsigned.
func (w *Writer) wrapUintCast(expr string, argHandle ir.ExpressionHandle) string {
	vecSize, isUint := w.isUnsignedExpr(argHandle)
	if !isUint {
		return expr
	}
	if vecSize == 0 {
		return fmt.Sprintf("uint(%s)", expr)
	}
	return fmt.Sprintf("uvec%d(%s)", vecSize, expr)
}

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

// writeExpressionInline writes an expression bypassing baked names.
// Used for const expressions like texture offsets that Rust writes inline
// via write_const_expr, even when the expression has been baked.
func (w *Writer) writeExpressionInline(handle ir.ExpressionHandle) (string, error) {
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
func (w *Writer) writeExpressionKind(kind ir.ExpressionKind, handle ir.ExpressionHandle) (string, error) {
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
		return w.writeImageLoad(k, handle)
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
	case ir.ExprSubgroupBallotResult:
		return "/* subgroup ballot result */", nil
	case ir.ExprSubgroupOperationResult:
		return "/* subgroup op result */", nil
	case ir.ExprRayQueryProceedResult:
		return "/* ray query proceed result */", nil
	case ir.ExprRayQueryGetIntersection:
		return "/* ray query get intersection */", nil
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
// Uses the proper zero literal for the type (0.0 for float, 0u for uint, etc.)
func (w *Writer) writeZeroValue(z ir.ExprZeroValue) (string, error) {
	zv := w.zeroInitValue(z.Type)
	if zv != "" {
		return zv, nil
	}
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
func (w *Writer) writeAccessIndex(a ir.ExprAccessIndex) (string, error) {
	// Check if this accesses a flattened entry point struct argument.
	if name, ok := w.resolveEPStructAccess(a); ok {
		return name, nil
	}

	base, err := w.writeExpression(a.Base)
	if err != nil {
		return "", err
	}

	// Check if this is a struct member access via type resolution.
	if name, ok := w.resolveStructMemberAccess(a, base); ok {
		return name, nil
	}

	// For vectors, use swizzle notation (.x/.y/.z/.w) instead of index notation.
	// Rust naga always uses swizzle for vector AccessIndex.
	if w.isVectorBase(a.Base) && a.Index < 4 {
		components := [4]string{"x", "y", "z", "w"}
		return fmt.Sprintf("%s.%s", base, components[a.Index]), nil
	}

	// For matrices, use column index notation.
	return fmt.Sprintf("%s[%d]", base, a.Index), nil
}

// isVectorBase returns true if the expression resolves to a Vector type (direct or through Pointer).
// Returns false for ValuePointerType (pointer to scalar/vector element from matrix/vector access) — those
// use bracket [index] notation. Matches Rust naga GLSL writer:
//   - Pointer{Vector} → unwrap → Vector → .x swizzle
//   - ValuePointer → [index] bracket
//   - Vector → .x swizzle
func (w *Writer) isVectorBase(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return false
	}
	res := &w.currentFunction.ExpressionTypes[handle]

	var checkType func(ir.TypeHandle) bool
	checkType = func(h ir.TypeHandle) bool {
		if int(h) >= len(w.module.Types) {
			return false
		}
		inner := w.module.Types[h].Inner
		if _, ok := inner.(ir.VectorType); ok {
			return true
		}
		// Unwrap Pointer types (Pointer{Vector} → true)
		if ptr, ok := inner.(ir.PointerType); ok {
			return checkType(ptr.Base)
		}
		return false
	}

	if res.Handle != nil {
		return checkType(*res.Handle)
	}
	if res.Value != nil {
		// Direct VectorType value → swizzle
		if _, ok := res.Value.(ir.VectorType); ok {
			return true
		}
		// PointerType value → unwrap to check if pointee is Vector
		if ptr, ok := res.Value.(ir.PointerType); ok {
			return checkType(ptr.Base)
		}
		// ValuePointerType → NOT vector (use bracket notation)
		// This is the key difference: ValuePointer comes from matrix/vector access
		// through pointer, and Rust GLSL uses [index] for it.
	}
	return false
}

// resolveEPStructAccess checks if an AccessIndex targets a flattened entry point
// struct argument and returns the GLSL variable name for that member.
func (w *Writer) resolveEPStructAccess(a ir.ExprAccessIndex) (string, bool) {
	if !w.inEntryPoint || w.currentFunction == nil {
		return "", false
	}
	if int(a.Base) >= len(w.currentFunction.Expressions) {
		return "", false
	}
	baseExpr := &w.currentFunction.Expressions[a.Base]
	funcArg, ok := baseExpr.Kind.(ir.ExprFunctionArgument)
	if !ok {
		return "", false
	}
	info, found := w.epStructArgs[funcArg.Index]
	if !found || int(a.Index) >= len(info.members) {
		return "", false
	}

	// If the struct argument was reconstructed as a local variable,
	// access through the local: structName.memberName
	if localName, ok := w.namedExpressions[a.Base]; ok {
		memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(info.structType), handle2: a.Index}]
		if memberName != "" {
			return fmt.Sprintf("%s.%s", localName, memberName), true
		}
	}

	// Otherwise use the varying name directly
	return info.members[a.Index].glslName, true
}

// resolveStructMemberAccess checks if an AccessIndex targets a struct type
// and returns "base.memberName" instead of "base[index]".
func (w *Writer) resolveStructMemberAccess(a ir.ExprAccessIndex, base string) (string, bool) {
	if w.currentFunction == nil || int(a.Base) >= len(w.currentFunction.Expressions) {
		return "", false
	}

	// Try to resolve base type from ExpressionTypes first (handles all expression types)
	var baseTypeHandle *ir.TypeHandle
	if int(a.Base) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[a.Base]
		if res.Handle != nil {
			h := *res.Handle
			// Unwrap pointer types (Handle path)
			if int(h) < len(w.module.Types) {
				if ptr, ok := w.module.Types[h].Inner.(ir.PointerType); ok {
					baseTypeHandle = &ptr.Base
				} else {
					baseTypeHandle = &h
				}
			}
		} else if res.Value != nil {
			// Unwrap pointer types (Value path — e.g., struct member through pointer
			// now returns PointerType{Base: memberType} as Value)
			if ptr, ok := res.Value.(ir.PointerType); ok {
				baseTypeHandle = &ptr.Base
			}
		}
	}
	// Fall back to expression kind-based resolution
	if baseTypeHandle == nil {
		baseExpr := &w.currentFunction.Expressions[a.Base]
		baseTypeHandle = w.getExpressionTypeHandle(baseExpr.Kind)
	}
	if baseTypeHandle == nil || int(*baseTypeHandle) >= len(w.module.Types) {
		return "", false
	}
	baseType := &w.module.Types[*baseTypeHandle]
	st, ok := baseType.Inner.(ir.StructType)
	if !ok || int(a.Index) >= len(st.Members) {
		return "", false
	}
	// Use the namer-registered member name (handles trailing _ for digits, disambiguation)
	registeredName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(*baseTypeHandle), handle2: a.Index}]
	if registeredName == "" {
		// Fallback to raw name
		registeredName = st.Members[a.Index].Name
		if registeredName == "" {
			return "", false
		}
	}
	return fmt.Sprintf("%s.%s", base, registeredName), true
}

// writeSplat writes a splat expression (scalar to vector).
func (w *Writer) writeSplat(s ir.ExprSplat) (string, error) {
	value, err := w.writeExpression(s.Value)
	if err != nil {
		return "", err
	}

	// Determine vector prefix from the scalar type of the splatted value.
	prefix := ""
	if w.currentFunction != nil && int(s.Value) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[s.Value]
		if res.Value != nil {
			if st, ok := res.Value.(ir.ScalarType); ok {
				prefix = scalarVecPrefix(st.Kind)
			}
		} else if res.Handle != nil {
			if int(*res.Handle) < len(w.module.Types) {
				if st, ok := w.module.Types[*res.Handle].Inner.(ir.ScalarType); ok {
					prefix = scalarVecPrefix(st.Kind)
				}
			}
		}
	}

	return fmt.Sprintf("%svec%d(%s)", prefix, s.Size, value), nil
}

// scalarVecPrefix returns the GLSL vector prefix for a scalar kind.
func scalarVecPrefix(kind ir.ScalarKind) string {
	switch kind {
	case ir.ScalarBool:
		return "b"
	case ir.ScalarSint:
		return "i"
	case ir.ScalarUint:
		return "u"
	case ir.ScalarFloat:
		return ""
	default:
		return ""
	}
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
// For globals that are part of a combined texture-sampler pair, this returns
// the combined sampler name so that texture() calls use the correct uniform.
func (w *Writer) writeGlobalVariable(g ir.ExprGlobalVariable) (string, error) {
	// If this global is part of a combined pair, we need to find which pair
	// uses it. For texture globals, the combined name is used directly in
	// writeImageSample via resolveCombinedSamplerName, but if someone
	// references the texture/sampler global outside of a sample expression
	// (e.g., textureSize), we still return the first matching combined name.
	if w.globalIsCombined[g.Variable] {
		for _, info := range w.combinedSamplers {
			if info.textureHandle == g.Variable || info.samplerHandle == g.Variable {
				return info.glslName, nil
			}
		}
	}

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
	// Try const-evaluation for LogicalNot on constant booleans
	if result, ok := w.tryConstEvalUnary(u); ok {
		return result, nil
	}

	operand, err := w.writeExpression(u.Expr)
	if err != nil {
		return "", err
	}

	switch u.Op {
	case ir.UnaryNegate:
		return fmt.Sprintf("-(%s)", operand), nil
	case ir.UnaryLogicalNot:
		// Rust naga: "not" for vector booleans, "!" for scalars
		isVec := false
		if w.currentFunction != nil && int(u.Expr) < len(w.currentFunction.ExpressionTypes) {
			res := &w.currentFunction.ExpressionTypes[u.Expr]
			if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
				_, isVec = w.module.Types[*res.Handle].Inner.(ir.VectorType)
			} else if res.Value != nil {
				_, isVec = res.Value.(ir.VectorType)
			}
		}
		if isVec {
			return fmt.Sprintf("not(%s)", operand), nil
		}
		return fmt.Sprintf("!(%s)", operand), nil
	case ir.UnaryBitwiseNot:
		return fmt.Sprintf("~(%s)", operand), nil
	default:
		return "", fmt.Errorf("unsupported unary operator: %v", u.Op)
	}
}

// writeBinary writes a binary expression.
func (w *Writer) writeBinary(b ir.ExprBinary) (string, error) {
	// Try const-evaluation: if both operands resolve to constant values,
	// write the evaluated result directly. Matches Rust naga GLSL writer
	// which evaluates constant Binary expressions at write time.
	if result, ok := w.tryConstEvalBinary(b); ok {
		return result, nil
	}

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
		// Rust naga: float modulo → (a - b * trunc(a / b)), integer → native %
		if w.isFloatBinaryExpr(b) {
			return fmt.Sprintf("(%s - %s * trunc(%s / %s))", left, right, left, right), nil
		}
		return fmt.Sprintf("(%s %% %s)", left, right), nil
	case ir.BinaryEqual:
		if w.isVectorBinaryExpr(b) {
			return fmt.Sprintf("equal(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s == %s)", left, right), nil
	case ir.BinaryNotEqual:
		if w.isVectorBinaryExpr(b) {
			return fmt.Sprintf("notEqual(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s != %s)", left, right), nil
	case ir.BinaryLess:
		if w.isVectorBinaryExpr(b) {
			return fmt.Sprintf("lessThan(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s < %s)", left, right), nil
	case ir.BinaryLessEqual:
		if w.isVectorBinaryExpr(b) {
			return fmt.Sprintf("lessThanEqual(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s <= %s)", left, right), nil
	case ir.BinaryGreater:
		if w.isVectorBinaryExpr(b) {
			return fmt.Sprintf("greaterThan(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s > %s)", left, right), nil
	case ir.BinaryGreaterEqual:
		if w.isVectorBinaryExpr(b) {
			return fmt.Sprintf("greaterThanEqual(%s, %s)", left, right), nil
		}
		return fmt.Sprintf("(%s >= %s)", left, right), nil
	case ir.BinaryAnd:
		// Rust naga: boolean scalar And → &&, boolean vector → component-wise
		if w.isBoolVectorBinaryExpr(b) {
			return w.expandBoolVectorOp(b, left, right, "&&")
		}
		if w.isBooleanBinaryExpr(b) {
			return fmt.Sprintf("(%s && %s)", left, right), nil
		}
		return fmt.Sprintf("(%s & %s)", left, right), nil
	case ir.BinaryExclusiveOr:
		return fmt.Sprintf("(%s ^ %s)", left, right), nil
	case ir.BinaryInclusiveOr:
		// Rust naga: boolean scalar Or → ||, boolean vector → component-wise
		if w.isBoolVectorBinaryExpr(b) {
			return w.expandBoolVectorOp(b, left, right, "||")
		}
		if w.isBooleanBinaryExpr(b) {
			return fmt.Sprintf("(%s || %s)", left, right), nil
		}
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
		// Matches Rust naga: for vectors, use vec{N}(0.0)/vec{N}(1.0) bounds
		vecSize, isVec := w.getExprVectorSize(m.Arg)
		if isVec {
			return fmt.Sprintf("clamp(%s, vec%d(0.0), vec%d(1.0))", args[0], vecSize, vecSize), nil
		}
		return fmt.Sprintf("clamp(%s, 0.0, 1.0)", args[0]), nil
	case ir.MathMix:
		return fmt.Sprintf("mix(%s)", argStr), nil
	case ir.MathStep:
		return fmt.Sprintf("step(%s)", argStr), nil
	case ir.MathSmoothStep:
		return fmt.Sprintf("smoothstep(%s)", argStr), nil
	case ir.MathFma:
		if w.options.LangVersion.supportsFma() {
			return fmt.Sprintf("fma(%s)", argStr), nil
		}
		// Fallback: (a * b + c)
		return fmt.Sprintf("(%s * %s + %s)", args[0], args[1], args[2]), nil
	case ir.MathModf:
		// modf needs special handling — returns struct
		return fmt.Sprintf("naga_modf(%s)", args[0]), nil
	case ir.MathFrexp:
		// frexp needs special handling — returns struct
		return fmt.Sprintf("naga_frexp(%s)", args[0]), nil
	case ir.MathLdexp:
		return fmt.Sprintf("ldexp(%s)", argStr), nil
	case ir.MathQuantizeF16:
		// QuantizeToF16 must split by vector size, processing pairs of components
		// via packHalf2x16/unpackHalf2x16. Matches Rust naga behavior.
		vecSize, isVec := w.getExprVectorSize(m.Arg)
		if !isVec {
			// Scalar: wrap in vec2, unpack, take .x
			return fmt.Sprintf("unpackHalf2x16(packHalf2x16(vec2(%s))).x", args[0]), nil
		}
		switch vecSize {
		case 2:
			// Vec2: pack/unpack directly
			return fmt.Sprintf("unpackHalf2x16(packHalf2x16(%s))", args[0]), nil
		case 3:
			// Vec3: split into .xy pair and .zz pair, take .x from second
			return fmt.Sprintf("vec3(unpackHalf2x16(packHalf2x16(%s.xy)), unpackHalf2x16(packHalf2x16(%s.zz)).x)", args[0], args[0]), nil
		case 4:
			// Vec4: split into .xy pair and .zw pair
			return fmt.Sprintf("vec4(unpackHalf2x16(packHalf2x16(%s.xy)), unpackHalf2x16(packHalf2x16(%s.zw)))", args[0], args[0]), nil
		default:
			return fmt.Sprintf("unpackHalf2x16(packHalf2x16(vec2(%s))).x", args[0]), nil
		}

	// Geometric
	case ir.MathLength:
		return fmt.Sprintf("length(%s)", argStr), nil
	case ir.MathDistance:
		return fmt.Sprintf("distance(%s)", argStr), nil
	case ir.MathDot:
		// GLSL dot() only works for float vectors. For int/uint vectors, expand manually.
		isIntDot := false
		if w.currentFunction != nil && int(m.Arg) < len(w.currentFunction.ExpressionTypes) {
			res := &w.currentFunction.ExpressionTypes[m.Arg]
			var inner ir.TypeInner
			if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
				inner = w.module.Types[*res.Handle].Inner
			} else if res.Value != nil {
				inner = res.Value
			}
			if inner != nil {
				if vt, ok := inner.(ir.VectorType); ok {
					isIntDot = vt.Scalar.Kind == ir.ScalarSint || vt.Scalar.Kind == ir.ScalarUint
				}
			}
		}
		if isIntDot {
			// Expand: ( + a.x*b.x + a.y*b.y [+ a.z*b.z [+ a.w*b.w]])
			components := []string{"x", "y", "z", "w"}
			size := 2 // default vec2
			if w.currentFunction != nil && int(m.Arg) < len(w.currentFunction.ExpressionTypes) {
				res := &w.currentFunction.ExpressionTypes[m.Arg]
				var inner ir.TypeInner
				if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
					inner = w.module.Types[*res.Handle].Inner
				} else if res.Value != nil {
					inner = res.Value
				}
				if vt, ok := inner.(ir.VectorType); ok {
					size = int(vt.Size)
				}
			}
			var parts []string
			for i := 0; i < size && i < 4; i++ {
				parts = append(parts, fmt.Sprintf("%s.%s * %s.%s", args[0], components[i], args[1], components[i]))
			}
			return fmt.Sprintf("( + %s)", strings.Join(parts, " + ")), nil
		}
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
		// bitCount always returns signed; cast to unsigned if arg is unsigned
		return w.wrapUintCast(fmt.Sprintf("bitCount(%s)", argStr), m.Arg), nil
	case ir.MathReverseBits:
		return fmt.Sprintf("bitfieldReverse(%s)", argStr), nil
	case ir.MathFirstLeadingBit:
		// findMSB always returns signed; cast to unsigned if arg is unsigned
		return w.wrapUintCast(fmt.Sprintf("findMSB(%s)", argStr), m.Arg), nil
	case ir.MathFirstTrailingBit:
		// findLSB always returns signed; cast to unsigned if arg is unsigned
		return w.wrapUintCast(fmt.Sprintf("findLSB(%s)", argStr), m.Arg), nil
	case ir.MathCountLeadingZeros:
		// GLSL doesn't have direct clz, use workaround
		return fmt.Sprintf("(31 - findMSB(%s))", args[0]), nil
	case ir.MathCountTrailingZeros:
		// GLSL doesn't have direct ctz, use findLSB
		return fmt.Sprintf("findLSB(%s)", argStr), nil
	case ir.MathExtractBits:
		// Rust naga: clamp offset and count for safety
		// bitfieldExtract(val, int(min(offset, 32u)), int(min(count, 32u - min(offset, 32u))))
		if len(args) >= 3 {
			return fmt.Sprintf("bitfieldExtract(%s, int(min(%s, 32u)), int(min(%s, 32u - min(%s, 32u))))",
				args[0], args[1], args[2], args[1]), nil
		}
		return fmt.Sprintf("bitfieldExtract(%s)", argStr), nil
	case ir.MathInsertBits:
		// Rust naga: clamp offset and count for safety
		if len(args) >= 4 {
			return fmt.Sprintf("bitfieldInsert(%s, %s, int(min(%s, 32u)), int(min(%s, 32u - min(%s, 32u))))",
				args[0], args[1], args[2], args[3], args[2]), nil
		}
		return fmt.Sprintf("bitfieldInsert(%s)", argStr), nil

	// Pack/Unpack
	case ir.MathDot4I8Packed:
		// Matches Rust naga: expand to sum of bitfieldExtract products with sign extension.
		// (bitfieldExtract(int(a), 0, 8) * bitfieldExtract(int(b), 0, 8) + ... for i in 0..4)
		var parts []string
		for i := 0; i < 4; i++ {
			parts = append(parts, fmt.Sprintf("bitfieldExtract(int(%s), %d, 8) * bitfieldExtract(int(%s), %d, 8)", args[0], i*8, args[1], i*8))
		}
		return fmt.Sprintf("(%s)", strings.Join(parts, " + ")), nil
	case ir.MathDot4U8Packed:
		// Matches Rust naga: expand to sum of bitfieldExtract products (unsigned).
		// (bitfieldExtract((a), 0, 8) * bitfieldExtract((b), 0, 8) + ... for i in 0..4)
		var parts []string
		for i := 0; i < 4; i++ {
			parts = append(parts, fmt.Sprintf("bitfieldExtract((%s), %d, 8) * bitfieldExtract((%s), %d, 8)", args[0], i*8, args[1], i*8))
		}
		return fmt.Sprintf("(%s)", strings.Join(parts, " + ")), nil
	case ir.MathPack4xI8:
		return fmt.Sprintf("uint((%s[0] & 0xFF) | ((%s[1] & 0xFF) << 8) | ((%s[2] & 0xFF) << 16) | ((%s[3] & 0xFF) << 24))",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathPack4xU8:
		return fmt.Sprintf("(%s[0] & 0xFFu) | ((%s[1] & 0xFFu) << 8) | ((%s[2] & 0xFFu) << 16) | ((%s[3] & 0xFFu) << 24)",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathPack4xI8Clamp:
		return fmt.Sprintf("uint((clamp(%s, -128, 127)[0] & 0xFF) | ((clamp(%s, -128, 127)[1] & 0xFF) << 8) | ((clamp(%s, -128, 127)[2] & 0xFF) << 16) | ((clamp(%s, -128, 127)[3] & 0xFF) << 24))",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathPack4xU8Clamp:
		return fmt.Sprintf("(clamp(%s, 0u, 255u)[0] & 0xFFu) | ((clamp(%s, 0u, 255u)[1] & 0xFFu) << 8) | ((clamp(%s, 0u, 255u)[2] & 0xFFu) << 16) | ((clamp(%s, 0u, 255u)[3] & 0xFFu) << 24)",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathUnpack4xI8:
		return fmt.Sprintf("ivec4(bitfieldExtract(int(%s), 0, 8), bitfieldExtract(int(%s), 8, 8), bitfieldExtract(int(%s), 16, 8), bitfieldExtract(int(%s), 24, 8))",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathUnpack4xU8:
		return fmt.Sprintf("uvec4(bitfieldExtract(%s, 0, 8), bitfieldExtract(%s, 8, 8), bitfieldExtract(%s, 16, 8), bitfieldExtract(%s, 24, 8))",
			args[0], args[0], args[0], args[0]), nil
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
		if w.options.LangVersion.supportsPack4x8() {
			return fmt.Sprintf("unpackSnorm4x8(%s)", argStr), nil
		}
		// Fallback: bit manipulation
		return fmt.Sprintf("(vec4(ivec4(%s << 24, %s << 16, %s << 8, %s) >> 24) / 127.0)",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathUnpack4x8unorm:
		if w.options.LangVersion.supportsPack4x8() {
			return fmt.Sprintf("unpackUnorm4x8(%s)", argStr), nil
		}
		// Fallback: extract bytes LSB first (matching Rust naga)
		return fmt.Sprintf("(vec4(%s & 0xFFu, %s >> 8 & 0xFFu, %s >> 16 & 0xFFu, %s >> 24) / 255.0)",
			args[0], args[0], args[0], args[0]), nil
	case ir.MathUnpack2x16snorm:
		if w.options.LangVersion.supportsPack2x16snorm() {
			return fmt.Sprintf("unpackSnorm2x16(%s)", argStr), nil
		}
		return fmt.Sprintf("(vec2(ivec2(%s << 16, %s) >> 16) / 32767.0)", args[0], args[0]), nil
	case ir.MathUnpack2x16unorm:
		if w.options.LangVersion.supportsPack2x16unorm() {
			return fmt.Sprintf("unpackUnorm2x16(%s)", argStr), nil
		}
		return fmt.Sprintf("(vec2(%s & 0xFFFFu, %s >> 16) / 65535.0)", args[0], args[0]), nil
	case ir.MathUnpack2x16float:
		return fmt.Sprintf("unpackHalf2x16(%s)", argStr), nil

	default:
		return "", fmt.Errorf("unsupported math function: %v", m.Fun)
	}
}

// writeDerivative writes a derivative expression.
// Matches Rust naga: Coarse/Fine only when version supports_derivative_control,
// otherwise fall back to dFdx/dFdy/fwidth.
func (w *Writer) writeDerivative(d ir.ExprDerivative) (string, error) {
	expr, err := w.writeExpression(d.Expr)
	if err != nil {
		return "", err
	}

	supportsControl := w.options.LangVersion.supportsDerivativeControl()

	switch d.Axis {
	case ir.DerivativeX:
		if supportsControl {
			switch d.Control {
			case ir.DerivativeCoarse:
				return fmt.Sprintf("dFdxCoarse(%s)", expr), nil
			case ir.DerivativeFine:
				return fmt.Sprintf("dFdxFine(%s)", expr), nil
			}
		}
		return fmt.Sprintf("dFdx(%s)", expr), nil
	case ir.DerivativeY:
		if supportsControl {
			switch d.Control {
			case ir.DerivativeCoarse:
				return fmt.Sprintf("dFdyCoarse(%s)", expr), nil
			case ir.DerivativeFine:
				return fmt.Sprintf("dFdyFine(%s)", expr), nil
			}
		}
		return fmt.Sprintf("dFdy(%s)", expr), nil
	case ir.DerivativeWidth:
		if supportsControl {
			switch d.Control {
			case ir.DerivativeCoarse:
				return fmt.Sprintf("fwidthCoarse(%s)", expr), nil
			case ir.DerivativeFine:
				return fmt.Sprintf("fwidthFine(%s)", expr), nil
			}
		}
		return fmt.Sprintf("fwidth(%s)", expr), nil
	default:
		return "", fmt.Errorf("unsupported derivative axis: %v", d.Axis)
	}
}

// writeImageSample writes an image sample expression.
// In GLSL, textures and samplers must be combined. The combined name is resolved
// from the pre-scanned texture-sampler pairs in combinedSamplers.
func (w *Writer) writeImageSample(s ir.ExprImageSample) (string, error) {
	coordExpr, err := w.writeExpression(s.Coordinate)
	if err != nil {
		return "", err
	}

	// Resolve the combined sampler name from the pre-scanned pairs.
	combinedName := w.resolveCombinedSamplerName(s.Image, s.Sampler)

	// Build coordinate vector (Rust naga wraps in vecN when needed)
	coordDim := w.getCoordDim(s.Coordinate)
	if s.ArrayIndex != nil {
		coordDim++
	}
	// Depth ref merged into coords when < 4 dims
	mergeDepthRef := s.DepthRef != nil && s.Gather == nil && coordDim < 4
	if mergeDepthRef {
		coordDim++
	}

	// Build the coordinate string
	coordinate := coordExpr
	if coordDim > 1 {
		parts := []string{coordExpr}
		if s.ArrayIndex != nil {
			idx, idxErr := w.writeExpression(*s.ArrayIndex)
			if idxErr != nil {
				return "", idxErr
			}
			parts = append(parts, idx)
		}
		if mergeDepthRef {
			ref, refErr := w.writeExpression(*s.DepthRef)
			if refErr != nil {
				return "", refErr
			}
			parts = append(parts, ref)
		}
		if len(parts) > 1 {
			coordinate = fmt.Sprintf("vec%d(%s)", coordDim, strings.Join(parts, ", "))
		} else {
			coordinate = fmt.Sprintf("vec%d(%s)", coordDim, coordExpr)
		}
	}

	// Depth ref not merged (separate parameter)
	depthRefStr := ""
	if s.DepthRef != nil && !mergeDepthRef {
		ref, refErr := w.writeExpression(*s.DepthRef)
		if refErr != nil {
			return "", refErr
		}
		depthRefStr = ", " + ref
	}

	// Handle gather operation
	if s.Gather != nil {
		offsetStr := ""
		offsetSuffix := ""
		if s.Offset != nil {
			off, offErr := w.writeExpression(*s.Offset)
			if offErr != nil {
				return "", offErr
			}
			offsetStr = ", " + off
			offsetSuffix = "Offset"
		}
		// For depth comparison gather, the depth_ref replaces the component
		if s.DepthRef != nil {
			return fmt.Sprintf("textureGather%s(%s, %s, %s%s)", offsetSuffix, combinedName, coordinate, depthRefStr[2:], offsetStr), nil
		}
		// Non-depth gather: component is the gathered channel
		component := int(*s.Gather)
		return fmt.Sprintf("textureGather%s(%s, %s%s, %d)", offsetSuffix, combinedName, coordinate, offsetStr, component), nil
	}

	// Determine function name base from sample level
	// Matches Rust naga: texture/textureLod/textureGrad + optional "Offset" suffix
	workaroundLodWithGrad := false
	funName := "texture"
	switch s.Level.(type) {
	case ir.SampleLevelExact:
		funName = "textureLod"
	case ir.SampleLevelBias:
		funName = "texture"
	case ir.SampleLevelGradient:
		funName = "textureGrad"
	case ir.SampleLevelZero:
		// Check for shadow LOD workaround
		if s.DepthRef != nil && !w.features.contains(FeatureTextureShadowLod) {
			imgType := w.resolveImageType(s.Image)
			if imgType != nil {
				isCubeShadow := imgType.Dim == ir.DimCube && !imgType.Arrayed && imgType.Class == ir.ImageClassDepth
				is2DArrayShadow := imgType.Dim == ir.Dim2D && imgType.Arrayed && imgType.Class == ir.ImageClassDepth
				if isCubeShadow || is2DArrayShadow {
					workaroundLodWithGrad = true
					funName = "textureGrad"
				}
			}
		}
		if !workaroundLodWithGrad {
			funName = "textureLod"
		}
	default:
		funName = "texture"
	}

	// Append "Offset" suffix if offset is present
	offsetSuffix := ""
	if s.Offset != nil {
		offsetSuffix = "Offset"
	}

	// Build the call: funName[Offset](sampler, coord[, depthRef][, level/grad][, offset][, bias])
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s(%s, %s%s", funName, offsetSuffix, combinedName, coordinate, depthRefStr)

	// Write level/gradient arguments
	switch level := s.Level.(type) {
	case ir.SampleLevelExact:
		levelExpr, err := w.writeExpression(level.Level)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, ", %s", levelExpr)
	case ir.SampleLevelBias:
		// Bias goes AFTER offset (written below)
	case ir.SampleLevelGradient:
		gradX, err := w.writeExpression(level.X)
		if err != nil {
			return "", err
		}
		gradY, err := w.writeExpression(level.Y)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, ", %s, %s", gradX, gradY)
	case ir.SampleLevelZero:
		if workaroundLodWithGrad {
			imgType := w.resolveImageType(s.Image)
			gradDim := 2
			if imgType != nil && imgType.Dim == ir.DimCube {
				gradDim = 3
			}
			fmt.Fprintf(&b, ", vec%d(0.0), vec%d(0.0)", gradDim, gradDim)
		} else {
			b.WriteString(", 0.0")
		}
	default:
		// SampleLevelAuto - no extra args
	}

	// Write offset (after level/grad, before bias)
	// Offset must be a const expression -- write inline, not through baked name
	// (matches Rust naga's write_const_expr for offset)
	if s.Offset != nil {
		off, err := w.writeExpressionInline(*s.Offset)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, ", %s", off)
	}

	// Bias is always the last argument (after offset)
	if bias, ok := s.Level.(ir.SampleLevelBias); ok {
		biasExpr, err := w.writeExpression(bias.Bias)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, ", %s", biasExpr)
	}

	b.WriteString(")")
	return b.String(), nil
}

// getExprVectorSize returns (vector_size, true) if the expression resolves to a vector type.
// Returns (0, false) for non-vector types.
func (w *Writer) getExprVectorSize(handle ir.ExpressionHandle) (int, bool) {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return 0, false
	}
	res := &w.currentFunction.ExpressionTypes[handle]
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		if v, ok := w.module.Types[*res.Handle].Inner.(ir.VectorType); ok {
			return int(v.Size), true
		}
	} else if res.Value != nil {
		if v, ok := res.Value.(ir.VectorType); ok {
			return int(v.Size), true
		}
	}
	return 0, false
}

// resolveImageType resolves the image type from an expression handle.
func (w *Writer) resolveImageType(exprHandle ir.ExpressionHandle) *ir.ImageType {
	if w.currentFunction == nil {
		return nil
	}
	gvHandle := w.resolveGlobalVarHandle(w.currentFunction, exprHandle)
	if gvHandle == nil {
		return nil
	}
	if int(*gvHandle) >= len(w.module.GlobalVariables) {
		return nil
	}
	g := &w.module.GlobalVariables[*gvHandle]
	if int(g.Type) >= len(w.module.Types) {
		return nil
	}
	if imgType, ok := w.module.Types[g.Type].Inner.(ir.ImageType); ok {
		return &imgType
	}
	return nil
}

// resolveCombinedSamplerName resolves the combined sampler name for a texture-sampler
// pair used in an ExprImageSample. It looks up the global variable handles and checks
// the pre-scanned combinedSamplers map.
func (w *Writer) resolveCombinedSamplerName(imageExpr, samplerExpr ir.ExpressionHandle) string {
	if w.currentFunction == nil {
		return w.fallbackCombinedName(imageExpr, samplerExpr)
	}

	// If image is a function argument, use it directly — in GLSL, texture function
	// arguments are already combined samplers (sampler params are filtered out).
	// Matches Rust naga: FunctionArgument(pos) → just writes the argument name.
	if int(imageExpr) < len(w.currentFunction.Expressions) {
		if _, ok := w.currentFunction.Expressions[imageExpr].Kind.(ir.ExprFunctionArgument); ok {
			name, _ := w.writeExpression(imageExpr)
			return name
		}
	}

	imageHandle := w.resolveGlobalVarHandle(w.currentFunction, imageExpr)
	samplerHandle := w.resolveGlobalVarHandle(w.currentFunction, samplerExpr)

	if imageHandle != nil && samplerHandle != nil {
		if name := w.getCombinedSamplerName(*imageHandle, *samplerHandle); name != "" {
			return name
		}
	}

	return w.fallbackCombinedName(imageExpr, samplerExpr)
}

// fallbackCombinedName generates a combined name by writing both expressions.
// Used when the pair was not found in the pre-scanned map.
func (w *Writer) fallbackCombinedName(imageExpr, samplerExpr ir.ExpressionHandle) string {
	image, _ := w.writeExpression(imageExpr)
	sampler, _ := w.writeExpression(samplerExpr)
	return fmt.Sprintf("%s_%s", image, sampler)
}

// writeImageLoad writes an image load expression with optional bounds checking.
// Matches Rust naga: uses texelFetch for sampled images, imageLoad for storage.
// Applies BoundsCheckPolicy: Restrict (clamp), ReadZeroSkipWrite (ternary), or Unchecked.
func (w *Writer) writeImageLoad(l ir.ExprImageLoad, handle ir.ExpressionHandle) (string, error) {
	image, err := w.writeExpression(l.Image)
	if err != nil {
		return "", err
	}
	coordinate, err := w.writeExpression(l.Coordinate)
	if err != nil {
		return "", err
	}

	imgType := w.resolveImageType(l.Image)
	isStorage := imgType != nil && imgType.Class == ir.ImageClassStorage

	// Storage images: unchecked on desktop GL (spec guarantees zero for invalid),
	// use configured policy on ES only.
	policy := w.options.BoundsCheckPolicies.ImageLoad
	if isStorage && !w.options.LangVersion.ES {
		policy = BoundsCheckUnchecked
	}

	coordStr := w.buildTextureCoord(coordinate, l.Coordinate, l.ArrayIndex, imgType)

	if isStorage {
		// Storage images: imageLoad(image, coord) — no level/sample
		return fmt.Sprintf("imageLoad(%s, %s)", image, coordStr), nil
	}

	// Determine coordinate vector size for ivecN constructors
	coordVecSize := w.getCoordVectorSize(imgType, l.ArrayIndex != nil)

	switch policy {
	case BoundsCheckRestrict:
		return w.writeImageLoadRestrict(l, handle, image, coordStr, coordVecSize, imgType)
	case BoundsCheckReadZeroSkipWrite:
		return w.writeImageLoadReadZero(l, image, coordStr, coordVecSize, imgType)
	default:
		return w.writeImageLoadUnchecked(l, image, coordStr)
	}
}

// writeImageLoadUnchecked writes texelFetch without bounds checking.
func (w *Writer) writeImageLoadUnchecked(l ir.ExprImageLoad, image, coordStr string) (string, error) {
	sampleOrLevel := l.Sample
	if sampleOrLevel == nil {
		sampleOrLevel = l.Level
	}
	if sampleOrLevel != nil {
		arg, err := w.writeExpression(*sampleOrLevel)
		if err != nil {
			return "", err
		}
		if _, isUint := w.isUnsignedExpr(*sampleOrLevel); isUint {
			arg = fmt.Sprintf("int(%s)", arg)
		}
		return fmt.Sprintf("texelFetch(%s, %s, %s)", image, coordStr, arg), nil
	}
	return fmt.Sprintf("texelFetch(%s, %s)", image, coordStr), nil
}

// writeImageLoadRestrict writes texelFetch with Restrict bounds checking.
// Clamps coordinates to textureSize-1 and level to textureQueryLevels-1.
// Uses pre-emitted _eN_clamped_lod variable for the level.
func (w *Writer) writeImageLoadRestrict(l ir.ExprImageLoad, handle ir.ExpressionHandle, image, coordStr string, coordVecSize int, imgType *ir.ImageType) (string, error) {
	var out strings.Builder

	// Determine clamped LOD or sample argument
	var thirdArg string
	isMulti := imgType != nil && imgType.Multisampled

	if isMulti && l.Sample != nil {
		// Multisampled: clamp sample
		sampleExpr, err := w.writeExpression(*l.Sample)
		if err != nil {
			return "", err
		}
		thirdArg = fmt.Sprintf("clamp(%s, 0, textureSamples(%s) - 1)", sampleExpr, image)
	} else if l.Level != nil {
		// Mipmapped: use pre-emitted clamped lod variable
		thirdArg = fmt.Sprintf("_e%d_clamped_lod", handle)
	}

	// Build clamped coordinate
	var clampedCoord string
	texSizeArg := ""
	if !isMulti && l.Level != nil {
		texSizeArg = fmt.Sprintf(", _e%d_clamped_lod", handle)
	}

	if coordVecSize <= 1 {
		// Scalar coordinate
		clampedCoord = fmt.Sprintf("clamp(%s, 0, textureSize(%s%s) - 1)", coordStr, image, texSizeArg)
	} else {
		// Vector coordinate
		zeroVec := fmt.Sprintf("ivec%d(0)", coordVecSize)
		oneVec := fmt.Sprintf("ivec%d(1)", coordVecSize)
		clampedCoord = fmt.Sprintf("clamp(%s, %s, textureSize(%s%s) - %s)", coordStr, zeroVec, image, texSizeArg, oneVec)
	}

	fmt.Fprintf(&out, "texelFetch(%s, %s", image, clampedCoord)
	if thirdArg != "" {
		fmt.Fprintf(&out, ", %s", thirdArg)
	}
	// Multisampled: Rust naga puts closing paren on next line (writeln before close)
	if isMulti {
		out.WriteString("\n)")
	} else {
		out.WriteString(")")
	}

	return out.String(), nil
}

// writeImageLoadReadZero writes texelFetch with ReadZeroSkipWrite policy.
// Returns zero vec if any coordinate is out of bounds.
func (w *Writer) writeImageLoadReadZero(l ir.ExprImageLoad, image, coordStr string, coordVecSize int, imgType *ir.ImageType) (string, error) {
	var out strings.Builder
	isMulti := imgType != nil && imgType.Multisampled

	// Build condition: (level < levels && coords < size) or (sample < samples && coords < size)
	out.WriteString("(")

	// Level/sample bounds check
	if !isMulti && l.Level != nil {
		levelExpr, err := w.writeExpression(*l.Level)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "%s < textureQueryLevels(%s) && ", levelExpr, image)
	}
	if isMulti && l.Sample != nil {
		sampleExpr, err := w.writeExpression(*l.Sample)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "%s < textureSamples(%s) && ", sampleExpr, image)
	}

	// Coordinate bounds check
	texSizeLevel := ""
	if !isMulti && l.Level != nil {
		levelExpr, _ := w.writeExpression(*l.Level)
		texSizeLevel = fmt.Sprintf(", %s", levelExpr)
	}
	if coordVecSize <= 1 {
		fmt.Fprintf(&out, "%s < textureSize(%s%s)", coordStr, image, texSizeLevel)
	} else {
		fmt.Fprintf(&out, "all(lessThan(%s, textureSize(%s%s)))", coordStr, image, texSizeLevel)
	}

	// True branch: texelFetch
	out.WriteString(" ? texelFetch(")
	out.WriteString(image)
	out.WriteString(", ")
	out.WriteString(coordStr)

	if isMulti && l.Sample != nil {
		sampleExpr, _ := w.writeExpression(*l.Sample)
		fmt.Fprintf(&out, ", %s", sampleExpr)
	} else if l.Level != nil {
		levelExpr, _ := w.writeExpression(*l.Level)
		fmt.Fprintf(&out, ", %s", levelExpr)
	}
	out.WriteString(")")

	// False branch: zero value
	// Determine scalar kind from image type for correct zero constructor
	zeroPrefix := "vec4"
	if imgType != nil && imgType.Class == ir.ImageClassSampled {
		switch imgType.SampledKind {
		case ir.ScalarSint:
			zeroPrefix = "ivec4"
		case ir.ScalarUint:
			zeroPrefix = "uvec4"
		}
	}
	// Zero value: use 0.0 for float, 0 for int, 0u for uint
	zeroVal := "0.0"
	if imgType != nil && imgType.Class == ir.ImageClassSampled {
		switch imgType.SampledKind {
		case ir.ScalarSint:
			zeroVal = "0"
		case ir.ScalarUint:
			zeroVal = "0u"
		}
	}
	fmt.Fprintf(&out, " : %s(%s)", zeroPrefix, zeroVal)
	out.WriteString(")")

	return out.String(), nil
}

// getCoordVectorSize returns the coordinate vector size for an image type.
// Includes array index dimension if present.
func (w *Writer) getCoordVectorSize(imgType *ir.ImageType, hasArrayIndex bool) int {
	size := 2 // default 2D
	if imgType != nil {
		switch imgType.Dim {
		case ir.Dim1D:
			size = 1
		case ir.Dim2D:
			size = 2
		case ir.Dim3D:
			size = 3
		case ir.DimCube:
			size = 2
		}
	}
	if hasArrayIndex {
		size++
	}
	return size
}

// buildTextureCoord builds the GLSL coordinate string for image operations.
// Matches Rust naga's write_texture_coord: merges array_index into coordinate
// vector and handles uint-to-int conversion.
func (w *Writer) buildTextureCoord(coordExpr string, coordHandle ir.ExpressionHandle, arrayIndex *ir.ExpressionHandle, imgType *ir.ImageType) string {
	// Calculate vector size
	coordDim := 1
	if imgType != nil {
		switch imgType.Dim {
		case ir.Dim1D:
			coordDim = 1
		case ir.Dim2D:
			coordDim = 2
		case ir.Dim3D:
			coordDim = 3
		case ir.DimCube:
			coordDim = 2
		}
	}

	if arrayIndex != nil {
		// With array index: wrap in ivecN(coord, arrayIndex)
		vectorSize := coordDim + 1
		arrayStr, err := w.writeExpression(*arrayIndex)
		if err != nil {
			return coordExpr
		}
		return fmt.Sprintf("ivec%d(%s, %s)", vectorSize, coordExpr, arrayStr)
	}

	// Without array index: check if coordinate is uint (needs signed conversion)
	vecSize, isUint := w.isUnsignedExpr(coordHandle)
	if isUint {
		if vecSize == 0 {
			return fmt.Sprintf("int(%s)", coordExpr)
		}
		return fmt.Sprintf("ivec%d(%s)", vecSize, coordExpr)
	}

	return coordExpr
}

// writeImageQuery writes an image query expression.
// Matches Rust naga: all image query results are wrapped in uint()/uvecN() casts,
// storage images use imageSize/imageSamples, size queries append .xy/.xyz swizzle.
func (w *Writer) writeImageQuery(q ir.ExprImageQuery) (string, error) {
	image, err := w.writeExpression(q.Image)
	if err != nil {
		return "", err
	}

	// Resolve image type to determine storage vs sampled and dimension
	imgType := w.resolveImageType(q.Image)
	isStorage := imgType != nil && imgType.Class == ir.ImageClassStorage
	isMulti := imgType != nil && imgType.Multisampled

	// Compute spatial component count from dimension
	components := 2 // default for 2D
	if imgType != nil {
		switch imgType.Dim {
		case ir.Dim1D:
			components = 1
		case ir.Dim2D:
			components = 2
		case ir.Dim3D:
			components = 3
		case ir.DimCube:
			components = 2
		}
	}

	// Component swizzle strings
	swizzles := []string{"", ".x", ".xy", ".xyz"}

	switch query := q.Query.(type) {
	case ir.ImageQuerySize:
		// Outer cast: uint() for 1D, uvecN() for 2D/3D
		var outerCast string
		if components == 1 {
			outerCast = "uint("
		} else {
			outerCast = fmt.Sprintf("uvec%d(", components)
		}

		// Inner function: imageSize for storage, textureSize for sampled/depth
		var inner string
		if isStorage {
			inner = fmt.Sprintf("imageSize(%s)", image)
		} else {
			// Build level argument
			levelStr := ""
			if query.Level != nil {
				level, err := w.writeExpression(*query.Level)
				if err != nil {
					return "", err
				}
				// If level expression is uint, wrap in int()
				if _, isUint := w.isUnsignedExpr(*query.Level); isUint {
					levelStr = fmt.Sprintf(", int(%s)", level)
				} else {
					levelStr = fmt.Sprintf(", %s", level)
				}
			} else if !isMulti {
				levelStr = ", 0"
			}
			inner = fmt.Sprintf("textureSize(%s%s)", image, levelStr)
		}

		// Append swizzle inside the cast, then close cast
		swizzle := ""
		if components > 1 {
			swizzle = swizzles[components]
		}
		return fmt.Sprintf("%s%s%s)", outerCast, inner, swizzle), nil

	case ir.ImageQueryNumLevels:
		return fmt.Sprintf("uint(textureQueryLevels(%s))", image), nil

	case ir.ImageQueryNumLayers:
		// NumLayers uses textureSize/imageSize and takes the NEXT component after spatial dims
		funName := "textureSize"
		if isStorage {
			funName = "imageSize"
		}
		var levelArg string
		if !isMulti {
			levelArg = ", 0"
		}
		// Layer component is one beyond the spatial components
		layerSwizzle := ""
		if components >= 1 {
			layerSwizzle = "." + string("xyz"[components])
		}
		return fmt.Sprintf("uint(%s(%s%s)%s)", funName, image, levelArg, layerSwizzle), nil

	case ir.ImageQueryNumSamples:
		funName := "textureSamples"
		if isStorage {
			funName = "imageSamples"
		}
		return fmt.Sprintf("uint(%s(%s))", funName, image), nil

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
		// Bitcast — use GLSL bitcast functions
		// Matches Rust naga: floatBitsToInt, floatBitsToUint, intBitsToFloat, uintBitsToFloat
		sourceKind := ir.ScalarFloat // default
		if w.currentFunction != nil && int(a.Expr) < len(w.currentFunction.ExpressionTypes) {
			res := &w.currentFunction.ExpressionTypes[a.Expr]
			if res.Value != nil {
				if st, ok := res.Value.(ir.ScalarType); ok {
					sourceKind = st.Kind
				} else if vt, ok := res.Value.(ir.VectorType); ok {
					sourceKind = vt.Scalar.Kind
				}
			} else if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
				switch inner := w.module.Types[*res.Handle].Inner.(type) {
				case ir.ScalarType:
					sourceKind = inner.Kind
				case ir.VectorType:
					sourceKind = inner.Scalar.Kind
				}
			}
		}
		switch {
		case sourceKind == ir.ScalarFloat && a.Kind == ir.ScalarSint:
			return fmt.Sprintf("floatBitsToInt(%s)", expr), nil
		case sourceKind == ir.ScalarFloat && a.Kind == ir.ScalarUint:
			return fmt.Sprintf("floatBitsToUint(%s)", expr), nil
		case sourceKind == ir.ScalarSint && a.Kind == ir.ScalarFloat:
			return fmt.Sprintf("intBitsToFloat(%s)", expr), nil
		case sourceKind == ir.ScalarUint && a.Kind == ir.ScalarFloat:
			return fmt.Sprintf("uintBitsToFloat(%s)", expr), nil
		default:
			// sint↔uint or same-type: use target type constructor
			// For vectors, use uvec2/ivec3 etc, not scalar uint/int
			targetType := w.resolveBitcastTargetType(a)
			return fmt.Sprintf("%s(%s)", targetType, expr), nil
		}
	}

	// Regular conversion — use target type constructor
	// For vectors: ivec2(expr), uvec3(expr), etc.
	if w.currentFunction != nil && int(a.Expr) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[a.Expr]
		var inner ir.TypeInner
		if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
			inner = w.module.Types[*res.Handle].Inner
		} else if res.Value != nil {
			inner = res.Value
		}
		if inner != nil {
			scalar := ir.ScalarType{Kind: a.Kind, Width: uint8(*a.Convert)}
			switch v := inner.(type) {
			case ir.VectorType:
				typeName := vectorToGLSL(ir.VectorType{Size: v.Size, Scalar: scalar})
				return fmt.Sprintf("%s(%s)", typeName, expr), nil
			case ir.MatrixType:
				typeName := matrixToGLSL(ir.MatrixType{Columns: v.Columns, Rows: v.Rows, Scalar: scalar})
				return fmt.Sprintf("%s(%s)", typeName, expr), nil
			}
		}
	}
	typeName := w.scalarKindToGLSL(a.Kind)
	return fmt.Sprintf("%s(%s)", typeName, expr), nil
}

// resolveBitcastTargetType determines the GLSL type for a bitcast's target.
// For vectors: builds uvec2/ivec3 etc from the source vector size + target scalar kind.
func (w *Writer) resolveBitcastTargetType(a ir.ExprAs) string {
	if w.currentFunction != nil && int(a.Expr) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[a.Expr]
		var inner ir.TypeInner
		if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
			inner = w.module.Types[*res.Handle].Inner
		} else if res.Value != nil {
			inner = res.Value
		}
		if inner != nil {
			if vt, ok := inner.(ir.VectorType); ok {
				targetScalar := ir.ScalarType{Kind: a.Kind, Width: vt.Scalar.Width}
				return vectorToGLSL(ir.VectorType{Size: vt.Size, Scalar: targetScalar})
			}
		}
	}
	return w.scalarKindToGLSL(a.Kind)
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

// getCoordDim returns the dimensionality of a coordinate expression (1 for scalar, 2-4 for vectors).
func (w *Writer) getCoordDim(handle ir.ExpressionHandle) uint8 {
	if w.currentFunction != nil && int(handle) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[handle]
		inner := w.resolveTypeInner(res, handle)
		if inner != nil {
			if vt, ok := inner.(ir.VectorType); ok {
				return uint8(vt.Size)
			}
			if _, ok := inner.(ir.ScalarType); ok {
				return 1
			}
		}
	}
	return 2 // default
}

// isVectorBinaryExpr checks if a binary expression operates on vector types.
func (w *Writer) isVectorBinaryExpr(b ir.ExprBinary) bool {
	if w.currentFunction == nil || int(b.Left) >= len(w.currentFunction.ExpressionTypes) {
		return false
	}
	res := &w.currentFunction.ExpressionTypes[b.Left]
	var inner ir.TypeInner
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner = w.module.Types[*res.Handle].Inner
	} else if res.Value != nil {
		inner = res.Value
	}
	_, isVec := inner.(ir.VectorType)
	return isVec
}

// isFloatBinaryExpr checks if a binary expression operates on float types.
func (w *Writer) isFloatBinaryExpr(b ir.ExprBinary) bool {
	if w.currentFunction == nil || int(b.Left) >= len(w.currentFunction.ExpressionTypes) {
		return false
	}
	res := &w.currentFunction.ExpressionTypes[b.Left]
	var inner ir.TypeInner
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner = w.module.Types[*res.Handle].Inner
	} else if res.Value != nil {
		inner = res.Value
	}
	if inner != nil {
		switch t := inner.(type) {
		case ir.ScalarType:
			return t.Kind == ir.ScalarFloat
		case ir.VectorType:
			return t.Scalar.Kind == ir.ScalarFloat
		}
	}
	return false
}

// isBoolVectorBinaryExpr checks if a binary expression operates on boolean VECTOR types.
func (w *Writer) isBoolVectorBinaryExpr(b ir.ExprBinary) bool {
	if w.currentFunction == nil || int(b.Left) >= len(w.currentFunction.ExpressionTypes) {
		return false
	}
	res := &w.currentFunction.ExpressionTypes[b.Left]
	var inner ir.TypeInner
	if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
		inner = w.module.Types[*res.Handle].Inner
	} else if res.Value != nil {
		inner = res.Value
	}
	if inner != nil {
		if vt, ok := inner.(ir.VectorType); ok {
			return vt.Scalar.Kind == ir.ScalarBool
		}
	}
	return false
}

// expandBoolVectorOp expands a boolean vector operation component-wise.
// e.g., bvec3(a.x || b.x, a.y || b.y, a.z || b.z)
func (w *Writer) expandBoolVectorOp(b ir.ExprBinary, left, right, op string) (string, error) {
	size := uint8(2) // default
	if w.currentFunction != nil && int(b.Left) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[b.Left]
		var inner ir.TypeInner
		if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
			inner = w.module.Types[*res.Handle].Inner
		} else if res.Value != nil {
			inner = res.Value
		}
		if vt, ok := inner.(ir.VectorType); ok {
			size = uint8(vt.Size)
		}
	}
	components := []string{"x", "y", "z", "w"}
	parts := make([]string, size)
	for i := uint8(0); i < size && i < 4; i++ {
		parts[i] = fmt.Sprintf("%s.%s %s %s.%s", left, components[i], op, right, components[i])
	}
	return fmt.Sprintf("bvec%d(%s)", size, strings.Join(parts, ", ")), nil
}

// isBooleanBinaryExpr checks if a binary expression operates on boolean types.
func (w *Writer) isBooleanBinaryExpr(b ir.ExprBinary) bool {
	if w.currentFunction == nil {
		return false
	}
	if int(b.Left) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[b.Left]
		if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
			if st, ok := w.module.Types[*res.Handle].Inner.(ir.ScalarType); ok {
				return st.Kind == ir.ScalarBool
			}
		}
		if res.Value != nil {
			if st, ok := res.Value.(ir.ScalarType); ok {
				return st.Kind == ir.ScalarBool
			}
		}
	}
	return false
}

// writeArrayLength writes an array length expression.
func (w *Writer) writeArrayLength(a ir.ExprArrayLength) (string, error) {
	expr, err := w.writeExpression(a.Array)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("uint(%s.length())", expr), nil
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
	case ir.ExprFunctionArgument:
		if w.currentFunction != nil && int(k.Index) < len(w.currentFunction.Arguments) {
			return &w.currentFunction.Arguments[k.Index].Type
		}
	case ir.ExprCompose:
		return &k.Type
	case ir.ExprZeroValue:
		return &k.Type
	}
	return nil
}

// tryConstEvalBinary tries to const-evaluate a binary expression at write time.
// If both operands are constant/literal, computes the result and formats as GLSL literal.
// Matches Rust naga GLSL writer behavior for constant expressions.
func (w *Writer) tryConstEvalBinary(b ir.ExprBinary) (string, bool) {
	if w.currentFunction == nil {
		return "", false
	}
	// Only const-eval when at least one operand involves a named constant (ExprConstant).
	// Plain Literal+Literal should NOT be folded — that's not what Rust does.
	if !w.involvesExprConstant(b.Left) && !w.involvesExprConstant(b.Right) {
		return "", false
	}
	leftVal, leftOk := w.exprConstValue(b.Left)
	rightVal, rightOk := w.exprConstValue(b.Right)
	if !leftOk || !rightOk {
		return "", false
	}
	result := ir.EvalBinaryFloat(b.Op, leftVal, rightVal)
	return w.formatConstResult(b.Left, result), true
}

// tryConstEvalUnary tries to const-evaluate a unary expression at write time.
func (w *Writer) tryConstEvalUnary(u ir.ExprUnary) (string, bool) {
	if w.currentFunction == nil {
		return "", false
	}
	if !w.involvesExprConstant(u.Expr) {
		return "", false
	}
	val, ok := w.exprConstValue(u.Expr)
	if !ok {
		return "", false
	}
	switch u.Op {
	case ir.UnaryLogicalNot:
		if val == 1.0 {
			return "false", true
		}
		return "true", true
	case ir.UnaryNegate:
		return w.formatConstResult(u.Expr, -val), true
	}
	return "", false
}

// involvesExprConstant checks if an expression references an ExprConstant (named constant).
func (w *Writer) involvesExprConstant(handle ir.ExpressionHandle) bool {
	if int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	expr := &w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprConstant:
		return true
	case ir.ExprBinary:
		return w.involvesExprConstant(k.Left) || w.involvesExprConstant(k.Right)
	case ir.ExprUnary:
		return w.involvesExprConstant(k.Expr)
	}
	return false
}

// exprConstValue tries to evaluate a function expression to a float64 constant.
// Handles Literal, ExprConstant (via global expression init), and recursively Binary/Unary.
func (w *Writer) exprConstValue(handle ir.ExpressionHandle) (float64, bool) {
	if int(handle) >= len(w.currentFunction.Expressions) {
		return 0, false
	}
	expr := &w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal:
		return ir.LiteralToFloat(k.Value), true
	case ir.ExprConstant:
		if int(k.Constant) < len(w.module.Constants) {
			c := &w.module.Constants[k.Constant]
			if int(c.Init) < len(w.module.GlobalExpressions) {
				ge := &w.module.GlobalExpressions[c.Init]
				if lit, ok := ge.Kind.(ir.Literal); ok {
					return ir.LiteralToFloat(lit.Value), true
				}
			}
		}
	case ir.ExprBinary:
		left, leftOk := w.exprConstValue(k.Left)
		right, rightOk := w.exprConstValue(k.Right)
		if leftOk && rightOk {
			return ir.EvalBinaryFloat(k.Op, left, right), true
		}
	case ir.ExprUnary:
		val, ok := w.exprConstValue(k.Expr)
		if ok {
			return ir.EvalUnaryFloat(k.Op, val), true
		}
	}
	return 0, false
}

// formatConstResult formats a const-evaluated float64 as the correct GLSL literal type.
func (w *Writer) formatConstResult(protoExpr ir.ExpressionHandle, val float64) string {
	if int(protoExpr) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[protoExpr]
		var inner ir.TypeInner
		if res.Handle != nil && int(*res.Handle) < len(w.module.Types) {
			inner = w.module.Types[*res.Handle].Inner
		} else {
			inner = res.Value
		}
		if s, ok := inner.(ir.ScalarType); ok {
			switch s.Kind {
			case ir.ScalarBool:
				if val == 1.0 {
					return "true"
				}
				return "false"
			case ir.ScalarSint:
				return fmt.Sprintf("%d", int32(val))
			case ir.ScalarUint:
				return fmt.Sprintf("%du", uint32(val))
			}
		}
	}
	return formatFloat(float32(val))
}
