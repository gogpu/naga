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

	// Handle special constants (NagaConstants) for vertex_index, instance_index, num_workgroups.
	// Matches Rust naga's ff_input check in write_expr.
	closingBracket := ""
	if w.options.SpecialConstantsBinding != nil {
		if bi := w.getFixedFunctionInput(handle); bi != nil {
			switch *bi {
			case ir.BuiltinVertexIndex:
				w.out.WriteString("(_NagaConstants.first_vertex + ")
				closingBracket = ")"
			case ir.BuiltinInstanceIndex:
				w.out.WriteString("(_NagaConstants.first_instance + ")
				closingBracket = ")"
			case ir.BuiltinNumWorkGroups:
				w.out.WriteString("uint3(_NagaConstants.first_vertex, _NagaConstants.first_instance, _NagaConstants.other)")
				return nil
			}
		}
	}

	// Check if this expression has a cached name (for named expressions)
	if name, ok := w.namedExpressions[handle]; ok {
		w.out.WriteString(name)
		w.out.WriteString(closingBracket)
		return nil
	}

	expr := &w.currentFunction.Expressions[handle]

	// Storage pointer interception: Access/AccessIndex on storage pointers are no-ops.
	// The access chain is built at Load/Store time via fillAccessChain.
	switch expr.Kind.(type) {
	case ir.ExprAccess, ir.ExprAccessIndex:
		if w.isStoragePointer(handle) {
			// Do nothing - the chain is written on Load/Store
			return nil
		}
	case ir.ExprLoad:
		load := expr.Kind.(ir.ExprLoad)
		if w.isStoragePointer(load.Pointer) {
			// Storage load: build access chain and emit ByteAddressBuffer Load
			varHandle, err := w.fillAccessChain(load.Pointer)
			if err != nil {
				return fmt.Errorf("storage load: %w", err)
			}
			// Get the result type of this Load expression
			resultTy := w.getExpressionTypeInner(handle)
			if resultTy == nil {
				return fmt.Errorf("storage load: cannot resolve result type for expression %d", handle)
			}
			// Also get the type handle for struct/array constructor names
			tyHandle := w.getExpressionTypeHandle(handle)
			return w.writeStorageLoad(varHandle, resultTy, tyHandle)
		}
	}

	if err := w.writeExpressionKind(expr.Kind); err != nil {
		return err
	}
	w.out.WriteString(closingBracket)
	return nil
}

// writeExpressionKind writes an expression kind to HLSL.
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
	case ir.ExprSubgroupBallotResult:
		// Subgroup ballot results are written by the subgroup statement
		w.out.WriteString("_subgroup_ballot_result")
		return nil
	case ir.ExprSubgroupOperationResult:
		// Subgroup operation results are written by the subgroup statement
		w.out.WriteString("_subgroup_op_result")
		return nil
	case ir.ExprRayQueryProceedResult:
		w.out.WriteString("_rq_proceed_result")
		return nil
	case ir.ExprRayQueryGetIntersection:
		return w.writeRayQueryGetIntersection(e)
	default:
		return fmt.Errorf("unsupported expression type: %T", kind)
	}
}

// writeConstExpression writes a const expression directly, bypassing the named
// expression cache. Matches Rust naga's write_const_expression which always
// writes the expression inline from the arena. Used for image sample offsets.
func (w *Writer) writeConstExpression(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil {
		return fmt.Errorf("no current function context")
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return fmt.Errorf("invalid const expression handle: %d", handle)
	}

	expr := &w.currentFunction.Expressions[handle]
	kind := expr.Kind

	switch e := kind.(type) {
	case ir.Literal:
		return w.writeLiteralValue(e.Value)
	case ir.ExprCompose:
		return w.writeComposeExpression(e)
	case ir.ExprSplat:
		return w.writeSplatExpression(e)
	case ir.ExprZeroValue:
		return w.writeZeroValueExpression(e)
	default:
		// Fall back to normal writeExpression for other types
		return w.writeExpression(handle)
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
		// Rust naga wraps I32 in int() constructor for unambiguous overload resolution
		i := int32(val)
		if i == math.MinInt32 {
			fmt.Fprintf(&w.out, "int(%d - 1)", i+1)
		} else {
			fmt.Fprintf(&w.out, "int(%d)", i)
		}

	case ir.LiteralU32:
		fmt.Fprintf(&w.out, "%du", uint32(val))

	case ir.LiteralI64:
		i := int64(val)
		if i == math.MinInt64 {
			fmt.Fprintf(&w.out, "(%dL - 1L)", i+1)
		} else {
			fmt.Fprintf(&w.out, "%dL", i)
		}

	case ir.LiteralU64:
		fmt.Fprintf(&w.out, "%duL", uint64(val))

	case ir.LiteralF32:
		w.out.WriteString(formatFloat32(float32(val)))

	case ir.LiteralF64:
		w.out.WriteString(formatFloat64(float64(val)))
		w.out.WriteByte('L')

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
// Global Constant Expressions
// =============================================================================

// writeGlobalConstExpression writes a constant expression from Module.GlobalExpressions.
// Used for global variable initializers and constant definitions.
func (w *Writer) writeGlobalConstExpression(handle ir.ExpressionHandle) error {
	if int(handle) >= len(w.module.GlobalExpressions) {
		return fmt.Errorf("global expression handle %d out of range", handle)
	}
	expr := &w.module.GlobalExpressions[handle]
	switch e := expr.Kind.(type) {
	case ir.Literal:
		return w.writeLiteralExpression(e)
	case ir.ExprZeroValue:
		return w.writeZeroValueExpression(e)
	case ir.ExprCompose:
		return w.writeGlobalComposeExpression(e)
	case ir.ExprConstant:
		return w.writeConstantExpression(e)
	case ir.ExprSplat:
		return w.writeGlobalSplatExpression(e)
	default:
		return fmt.Errorf("unsupported global expression type: %T", expr.Kind)
	}
}

// writeGlobalComposeExpression writes a compose from global expressions.
func (w *Writer) writeGlobalComposeExpression(e ir.ExprCompose) error {
	// Arrays use constructor function ConstructarrayN_type_()
	if isArrayType(w.module, e.Type) {
		typeId := w.hlslTypeId(e.Type)
		fmt.Fprintf(&w.out, "Construct%s(", typeId)
		for i, comp := range e.Components {
			if i > 0 {
				w.out.WriteString(", ")
			}
			if err := w.writeGlobalConstExpression(comp); err != nil {
				return err
			}
		}
		w.out.WriteByte(')')
		return nil
	}

	// Structs use constructor function ConstructTypeName()
	if int(e.Type) < len(w.module.Types) {
		if _, isStruct := w.module.Types[e.Type].Inner.(ir.StructType); isStruct {
			typeName := w.typeNames[e.Type]
			if typeName == "" {
				typeName = w.getTypeName(e.Type)
			}
			fmt.Fprintf(&w.out, "Construct%s(", typeName)
			for i, comp := range e.Components {
				if i > 0 {
					w.out.WriteString(", ")
				}
				if err := w.writeGlobalConstExpression(comp); err != nil {
					return err
				}
			}
			w.out.WriteByte(')')
			return nil
		}
	}

	typeName := w.getTypeName(e.Type)
	fmt.Fprintf(&w.out, "%s(", typeName)
	if len(e.Components) == 0 {
		// Empty compose (e.g., vec2()) — expand to explicit zero values
		if int(e.Type) < len(w.module.Types) {
			w.writeExplicitZeroArgs(w.module.Types[e.Type].Inner)
		}
	} else {
		for i, comp := range e.Components {
			if i > 0 {
				w.out.WriteString(", ")
			}
			if err := w.writeGlobalConstExpression(comp); err != nil {
				return err
			}
		}
	}
	w.out.WriteByte(')')
	return nil
}

// writeGlobalSplatExpression writes a splat from global expressions.
func (w *Writer) writeGlobalSplatExpression(e ir.ExprSplat) error {
	// Splat has Size and Value, need to construct a vector type name
	// Get the scalar type from the value expression
	w.out.WriteByte('(')
	if err := w.writeGlobalConstExpression(e.Value); err != nil {
		return err
	}
	// Use swizzle-style splat
	components := [4]string{"x", "xx", "xxx", "xxxx"}
	if int(e.Size) > 0 && int(e.Size) <= 4 {
		fmt.Fprintf(&w.out, ").%s", components[e.Size-1])
	} else {
		w.out.WriteByte(')')
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
// Rust naga wraps these in helper functions to avoid parse issues like `(float4)0.y`.
func (w *Writer) writeZeroValueExpression(e ir.ExprZeroValue) error {
	typeId := w.hlslTypeId(e.Type)
	fmt.Fprintf(&w.out, "ZeroValue%s()", typeId)
	return nil
}

// =============================================================================
// Composite Expressions
// =============================================================================

// writeComposeExpression writes a composite construction (vector, matrix, array, struct).
func (w *Writer) writeComposeExpression(e ir.ExprCompose) error {
	// HLSL arrays use constructor function Constructarray{N}_{type}_()
	if isArrayType(w.module, e.Type) {
		typeId := w.hlslTypeId(e.Type)
		fmt.Fprintf(&w.out, "Construct%s(", typeId)
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

	typeName := w.getTypeName(e.Type)

	// Struct types use Construct{Type}() function (matches Rust naga)
	if _, needsConstructor := w.structConstructors[e.Type]; needsConstructor {
		fmt.Fprintf(&w.out, "Construct%s(", typeName)
	} else {
		w.out.WriteString(typeName)
		w.out.WriteByte('(')
	}

	if len(e.Components) == 0 {
		// Empty compose (e.g., vec2()) — expand to explicit zero values
		// HLSL doesn't support empty constructor calls like float2()
		if int(e.Type) < len(w.module.Types) {
			w.writeExplicitZeroArgs(w.module.Types[e.Type].Inner)
		}
	} else {
		for i, comp := range e.Components {
			if i > 0 {
				w.out.WriteString(", ")
			}
			if err := w.writeExpression(comp); err != nil {
				return fmt.Errorf("compose component %d: %w", i, err)
			}
		}
	}

	w.out.WriteByte(')')
	return nil
}

// writeExplicitZeroArgs writes explicit zero arguments for an empty Compose.
// HLSL doesn't support empty constructor calls like float2(), so we expand to float2(0.0, 0.0).
func (w *Writer) writeExplicitZeroArgs(inner ir.TypeInner) {
	switch t := inner.(type) {
	case ir.VectorType:
		count := int(t.Size)
		for i := 0; i < count; i++ {
			if i > 0 {
				w.out.WriteString(", ")
			}
			w.writeScalarZeroLiteral(t.Scalar)
		}
	case ir.MatrixType:
		// Matrix with zero columns
		cols := int(t.Columns)
		for i := 0; i < cols; i++ {
			if i > 0 {
				w.out.WriteString(", ")
			}
			w.writeScalarZeroLiteral(t.Scalar)
		}
	default:
		w.out.WriteString("0")
	}
}

// writeScalarZeroLiteral writes a zero literal for a scalar type.
func (w *Writer) writeScalarZeroLiteral(scalar ir.ScalarType) {
	switch scalar.Kind {
	case ir.ScalarFloat:
		w.out.WriteString("0.0")
	case ir.ScalarSint:
		w.out.WriteString("0")
	case ir.ScalarUint:
		w.out.WriteString("0u")
	case ir.ScalarBool:
		w.out.WriteString("false")
	default:
		w.out.WriteString("0")
	}
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
	// Dynamic column access on matCx2 inside array-of-matCx2 struct members:
	// use __get_col_of_matCx2(base, index) instead of base[index].
	// Matches Rust naga's find_matrix_in_access_chain path.
	if m := w.getInnerMatrixOfStructArrayMember(e.Base); m != nil && m.isMatCx2() {
		// Check if the base type is a matCx2 (meaning we're accessing columns)
		baseInner := w.getExpressionTypeInner(e.Base)
		if baseInner != nil {
			if ptr, ok := baseInner.(ir.PointerType); ok {
				if int(ptr.Base) < len(w.module.Types) {
					baseInner = w.module.Types[ptr.Base].Inner
				}
			}
			if mat, ok := baseInner.(ir.MatrixType); ok && mat.Rows == 2 {
				fmt.Fprintf(&w.out, "__get_col_of_mat%dx2(", mat.Columns)
				if err := w.writeExpression(e.Base); err != nil {
					return fmt.Errorf("matCx2 dynamic col base: %w", err)
				}
				w.out.WriteString(", ")
				if err := w.writeExpression(e.Index); err != nil {
					return fmt.Errorf("matCx2 dynamic col index: %w", err)
				}
				w.out.WriteByte(')')
				return nil
			}
		}
	}

	// Determine if this is a binding array access, and whether the index is non-uniform.
	// Matches Rust naga: indexing_binding_array + non_uniform_qualifier detection.
	indexingBindingArray := w.isBindingArrayAccess(e.Base)
	nonUniformQualifier := indexingBindingArray && w.isNonUniform(e.Index)

	if err := w.writeExpression(e.Base); err != nil {
		return fmt.Errorf("access base: %w", err)
	}

	// Check if this is a sampler binding array access -- use heap pattern.
	samplerInfo := w.samplerBindingArrayInfoFromExpression(e.Base)
	if samplerInfo != nil {
		fmt.Fprintf(&w.out, "%s[", samplerInfo.samplerHeapName)
	} else {
		w.out.WriteByte('[')
	}

	// When restrict_indexing is enabled, clamp dynamic indices to valid range
	// matching Rust naga: min(uint(index), maxIndex)
	// Skip check when indexing a binding array (they don't get restrict_indexing).
	needsBoundCheck := w.options.RestrictIndexing && !indexingBindingArray && w.needsRestrictIndexing(e.Base)
	if needsBoundCheck {
		if maxIdx, ok := w.getAccessMaxIndex(e.Base); ok {
			if !w.isConstantIndexInBounds(e.Index, maxIdx+1) {
				fmt.Fprintf(&w.out, "min(uint(")
				if err := w.writeExpression(e.Index); err != nil {
					return fmt.Errorf("access index: %w", err)
				}
				fmt.Fprintf(&w.out, "), %du)", maxIdx)
				w.out.WriteByte(']')
				return nil
			}
		}
	}

	// Write the index, possibly wrapped with NonUniformResourceIndex and/or sampler buffer lookup
	if nonUniformQualifier {
		w.out.WriteString("NonUniformResourceIndex(")
	}
	if samplerInfo != nil {
		fmt.Fprintf(&w.out, "%s[%s + ", samplerInfo.samplerIndexBufferName, samplerInfo.bindingArrayBaseIndexName)
	}
	if err := w.writeExpression(e.Index); err != nil {
		return fmt.Errorf("access index: %w", err)
	}
	if samplerInfo != nil {
		w.out.WriteByte(']') // close sampler index buffer lookup
	}
	if nonUniformQualifier {
		w.out.WriteByte(')') // close NonUniformResourceIndex
	}

	w.out.WriteByte(']')
	return nil
}

// getAccessMaxIndex returns the maximum valid index for an Access expression's base.
// Returns (maxIndex, true) if the base has a known size, (0, false) otherwise.
func (w *Writer) getAccessMaxIndex(base ir.ExpressionHandle) (uint32, bool) {
	baseType := w.getExpressionType(base)
	if baseType == nil {
		baseType = w.inferExpressionType(base)
	}
	if baseType == nil {
		return 0, false
	}
	inner := baseType.Inner
	// Dereference pointers
	if ptr, ok := inner.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			inner = w.module.Types[ptr.Base].Inner
		}
	}
	switch t := inner.(type) {
	case ir.ArrayType:
		if t.Size.Constant != nil && *t.Size.Constant > 0 {
			return *t.Size.Constant - 1, true
		}
	case ir.VectorType:
		if t.Size > 0 {
			return uint32(t.Size) - 1, true
		}
	case ir.MatrixType:
		if t.Columns > 0 {
			return uint32(t.Columns) - 1, true
		}
	}
	return 0, false
}

// needsRestrictIndexing returns true if bounds checking should be applied for
// this base expression's address space. Matches Rust naga's logic:
// Function/Private/WorkGroup/Immediate/TaskPayload/None -> true
// Uniform -> check per-binding BindTarget.RestrictIndexing
// Storage/Handle -> unreachable (handled by storage load path)
func (w *Writer) needsRestrictIndexing(base ir.ExpressionHandle) bool {
	space := w.resolveAccessBaseSpace(base)
	switch space {
	case ir.SpaceUniform:
		// Check if the per-binding restrict_indexing is set.
		// Matches Rust: resolve the global variable's binding, look up BindTarget.restrict_indexing.
		if gvHandle, ok := w.resolveGlobalVariableHandle(base); ok {
			gv := w.module.GlobalVariables[gvHandle]
			if gv.Binding != nil {
				rb := ResourceBinding{Group: gv.Binding.Group, Binding: gv.Binding.Binding}
				if bt, found := w.options.BindingMap[rb]; found {
					return bt.RestrictIndexing
				}
			}
		}
		return false // Default: no restrict_indexing for uniform
	case ir.SpaceStorage:
		return false // Storage: handled by ByteAddressBuffer path, not restrict_indexing
	default:
		return true // Function, Private, WorkGroup, etc.
	}
}

// resolveGlobalVariableHandle walks the access chain from an expression to find
// the root GlobalVariable handle. Returns (handle, true) if found.
func (w *Writer) resolveGlobalVariableHandle(handle ir.ExpressionHandle) (ir.GlobalVariableHandle, bool) {
	if w.currentFunction == nil {
		return 0, false
	}
	cur := handle
	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return 0, false
		}
		expr := w.currentFunction.Expressions[cur]
		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			return e.Variable, true
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		case ir.ExprLoad:
			cur = e.Pointer
		default:
			return 0, false
		}
	}
}

// resolveAccessBaseSpace resolves the address space for an access expression's base,
// walking through Load, Access, AccessIndex, and GlobalVariable expressions.
func (w *Writer) resolveAccessBaseSpace(handle ir.ExpressionHandle) ir.AddressSpace {
	if w.currentFunction == nil {
		return ir.SpaceFunction
	}
	cur := handle
	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return ir.SpaceFunction
		}
		expr := w.currentFunction.Expressions[cur]
		switch e := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			if int(e.Variable) < len(w.module.GlobalVariables) {
				return w.module.GlobalVariables[e.Variable].Space
			}
			return ir.SpaceFunction
		case ir.ExprLocalVariable:
			return ir.SpaceFunction
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		case ir.ExprLoad:
			cur = e.Pointer
		default:
			// Check type resolution for pointer type
			space := w.getPointerAddressSpace(cur)
			return space
		}
	}
}

// isConstantIndexInBounds checks if the index expression is a compile-time constant
// whose value is less than `length`. Matches Rust naga's access_needs_check:
// if the index is Known and < length, no bounds check is needed.
func (w *Writer) isConstantIndexInBounds(index ir.ExpressionHandle, length uint32) bool {
	if w.currentFunction == nil || int(index) >= len(w.currentFunction.Expressions) {
		return false
	}
	expr := w.currentFunction.Expressions[index]
	// Direct literal constant
	if lit, ok := expr.Kind.(ir.Literal); ok {
		switch v := lit.Value.(type) {
		case ir.LiteralU32:
			return uint32(v) < length
		case ir.LiteralI32:
			return v >= 0 && uint32(v) < length
		}
	}
	// Constant expression (ExprConstant) - check global constants for scalar value
	if c, ok := expr.Kind.(ir.ExprConstant); ok {
		if int(c.Constant) < len(w.module.Constants) {
			cnst := &w.module.Constants[c.Constant]
			if sv, ok := cnst.Value.(ir.ScalarValue); ok {
				switch sv.Kind {
				case ir.ScalarUint:
					return sv.Bits < uint64(length)
				case ir.ScalarSint:
					return int64(sv.Bits) >= 0 && sv.Bits < uint64(length)
				}
			}
		}
	}
	return false
}

// writeAccessIndexExpression writes access with compile-time constant index.
func (w *Writer) writeAccessIndexExpression(e ir.ExprAccessIndex) error {
	// Need to determine if this is a struct field access or array/vector access
	// For struct fields, use .member syntax; for arrays/vectors, use [index]

	// Get the base expression's type
	baseType := w.getExpressionType(e.Base)
	// If type resolution failed, try to infer through the expression chain
	if baseType == nil {
		baseType = w.inferExpressionType(e.Base)
	}
	if baseType == nil {
		// Fallback to array syntax
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("access index base: %w", err)
		}
		fmt.Fprintf(&w.out, "[%d]", e.Index)
		return nil
	}

	// Dereference pointer types (common when ExpressionTypes returns ptr for loads)
	resolvedInner := baseType.Inner
	if ptr, ok := resolvedInner.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			resolvedInner = w.module.Types[ptr.Base].Inner
		}
	}

	// Resolve the TypeHandle for the base (needed for matCx2 and member name lookup)
	baseTyHandle := w.resolveStructTypeHandleForAccess(e.Base)
	// Dereference pointer to get the actual struct TypeHandle
	var resolvedBaseTyHandle *ir.TypeHandle
	if baseTyHandle != nil {
		th := *baseTyHandle
		if ptr, ok := w.module.Types[th].Inner.(ir.PointerType); ok {
			resolvedBaseTyHandle = &ptr.Base
		} else {
			resolvedBaseTyHandle = baseTyHandle
		}
	}

	switch inner := resolvedInner.(type) {
	case ir.StructType:
		// Check if accessing a matCx2 member — use GetMat helper instead of direct access
		if resolvedBaseTyHandle != nil && int(e.Index) < len(inner.Members) {
			member := inner.Members[e.Index]
			if member.Binding == nil {
				if mat, ok := w.module.Types[member.Type].Inner.(ir.MatrixType); ok && mat.Rows == 2 {
					structName := w.typeNames[*resolvedBaseTyHandle]
					fieldName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(*resolvedBaseTyHandle), handle2: uint32(e.Index)}]
					fmt.Fprintf(&w.out, "GetMat%sOn%s(", fieldName, structName)
					if err := w.writeExpression(e.Base); err != nil {
						return fmt.Errorf("matCx2 get base: %w", err)
					}
					w.out.WriteByte(')')
					return nil
				}
			}
		}

		// Struct member access
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("struct access base: %w", err)
		}
		// Get struct type handle for member name lookup.
		if resolvedBaseTyHandle != nil && int(e.Index) < len(inner.Members) {
			memberName := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(*resolvedBaseTyHandle), handle2: uint32(e.Index)}]
			if memberName == "" {
				memberName = Escape(inner.Members[e.Index].Name)
			}
			fmt.Fprintf(&w.out, ".%s", memberName)
		} else if int(e.Index) < len(inner.Members) {
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
		// Matrix column access. For matCx2 inside array-of-matCx2 struct members,
		// use ._N notation matching the __matCx2 decomposed struct layout.
		if inner.Rows == 2 {
			if m := w.getInnerMatrixOfStructArrayMember(e.Base); m != nil && m.isMatCx2() {
				if err := w.writeExpression(e.Base); err != nil {
					return fmt.Errorf("matCx2 column access base: %w", err)
				}
				fmt.Fprintf(&w.out, "._%d", e.Index)
				return nil
			}
		}
		if err := w.writeExpression(e.Base); err != nil {
			return fmt.Errorf("matrix access base: %w", err)
		}
		fmt.Fprintf(&w.out, "[%d]", e.Index)

	case ir.BindingArrayType, ir.ArrayType:
		// Check for sampler binding array pattern -- use heap access.
		// Matches Rust naga AccessIndex handling for BindingArray of samplers.
		samplerInfo := w.samplerBindingArrayInfoFromExpression(e.Base)
		if samplerInfo != nil {
			// Sampler binding array: heap[indexBuffer[baseName + index]]
			if err := w.writeExpression(e.Base); err != nil {
				return fmt.Errorf("sampler binding array base: %w", err)
			}
			fmt.Fprintf(&w.out, "%s[%s[%s + %d]]",
				samplerInfo.samplerHeapName,
				samplerInfo.samplerIndexBufferName,
				samplerInfo.bindingArrayBaseIndexName,
				e.Index)
		} else {
			if err := w.writeExpression(e.Base); err != nil {
				return fmt.Errorf("access index base: %w", err)
			}
			fmt.Fprintf(&w.out, "[%d]", e.Index)
		}

	default:
		// Unknown - use bracket syntax
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

// getFixedFunctionInput checks if an expression is a fixed-function input
// (VertexIndex, InstanceIndex, NumWorkGroups) from an entry point argument.
// Returns the builtin kind if found, nil otherwise.
// Matches Rust naga's FunctionCtx::is_fixed_function_input.
func (w *Writer) getFixedFunctionInput(handle ir.ExpressionHandle) *ir.BuiltinValue {
	if w.currentFunction == nil || w.currentEPIndex < 0 {
		return nil
	}
	if int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}

	expr := w.currentFunction.Expressions[handle].Kind

	// Walk through AccessIndex chain to find the root FunctionArgument.
	// Matches Rust naga's is_fixed_function_input which only walks through
	// struct AccessIndex (not vector/matrix AccessIndex).
	var builtIn *ir.BuiltinValue
	for {
		switch e := expr.(type) {
		case ir.ExprFunctionArgument:
			if int(e.Index) < len(w.currentFunction.Arguments) {
				arg := &w.currentFunction.Arguments[e.Index]
				if arg.Binding != nil {
					if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
						bi := bb.Builtin
						if builtIn != nil {
							return builtIn
						}
						return &bi
					}
				}
			}
			return builtIn
		case ir.ExprAccessIndex:
			// Only walk through struct member access (not vector/matrix element access)
			if int(e.Base) >= len(w.currentFunction.Expressions) {
				return nil
			}
			baseType := w.getExpressionTypeInner(e.Base)
			if st, ok := baseType.(ir.StructType); ok {
				// Struct member access -- check if member has a builtin binding
				if int(e.Index) < len(st.Members) {
					if st.Members[e.Index].Binding != nil {
						if bb, ok := (*st.Members[e.Index].Binding).(ir.BuiltinBinding); ok {
							bi := bb.Builtin
							builtIn = &bi
						}
					}
				}
				expr = w.currentFunction.Expressions[e.Base].Kind
				continue
			}
			// Non-struct access (vector/matrix element) -- stop walking
			return nil
		default:
			return nil
		}
	}
}

// writeFunctionArgumentExpression writes a reference to a function argument.
// External texture arguments are expanded into comma-separated plane + params names.
func (w *Writer) writeFunctionArgumentExpression(e ir.ExprFunctionArgument) error {
	// Check if this is an external texture argument
	key := externalTextureFuncArgKey{funcHandle: w.currentFuncHandle, argIndex: e.Index}
	if names, ok := w.externalTextureFuncArgNames[key]; ok {
		fmt.Fprintf(&w.out, "%s, %s, %s, %s", names[0], names[1], names[2], names[3])
		return nil
	}

	name := w.names[nameKey{kind: nameKeyFunctionArgument, handle1: uint32(w.currentFuncHandle), handle2: e.Index}]
	if name == "" {
		name = fmt.Sprintf("arg_%d", e.Index)
	}
	w.out.WriteString(name)
	return nil
}

// writeGlobalVariableExpression writes a reference to a global variable.
// For binding arrays of samplers, nothing is written -- the access is handled
// entirely by writeAccessExpression/writeAccessIndexExpression.
// For external textures, emits the expanded plane + params names.
// Matches Rust naga: is_binding_array_of_samplers check in Expression::GlobalVariable.
func (w *Writer) writeGlobalVariableExpression(e ir.ExprGlobalVariable) error {
	// Skip binding arrays of samplers -- writing is done by Access/AccessIndex
	if w.isBindingArrayOfSamplers(e.Variable) {
		return nil
	}

	// External textures: emit expanded comma-separated names
	if _, ok := w.externalTextureGlobalNames[e.Variable]; ok {
		w.writeExternalTextureGlobalExpression(e.Variable)
		return nil
	}

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
// For matCx2 types in uniform buffers, wraps with a cast to the native HLSL matrix type.
// Matches Rust naga back/hlsl/writer.rs Expression::Load handling.
func (w *Writer) writeLoadExpression(e ir.ExprLoad) error {
	// Check if this load needs a matCx2-to-matrix cast.
	// Applies to: global uniform matCx2 and struct member array-of-matCx2.
	// Matches Rust naga: get_inner_matrix_of_struct_array_member || get_inner_matrix_of_global_uniform.
	m := w.getInnerMatrixOfStructArrayMember(e.Pointer)
	if m == nil || !m.isMatCx2() {
		m = w.getInnerMatrixOfGlobalUniform(e.Pointer)
	}
	if m != nil && m.isMatCx2() {
		// Determine the pointer's resolved type to know if it's array or matrix
		resolved := w.getExpressionTypeInner(e.Pointer)
		if resolved != nil {
			if ptr, ok := resolved.(ir.PointerType); ok {
				if int(ptr.Base) < len(w.module.Types) {
					resolved = w.module.Types[ptr.Base].Inner
				}
			}
		}

		w.out.WriteString("((")
		if arr, ok := resolved.(ir.ArrayType); ok {
			// Array: cast to native matrix array, e.g. ((float4x2[2])...)
			w.writeMatrixValueType(m)
			if arr.Size.Constant != nil {
				fmt.Fprintf(&w.out, "[%d]", *arr.Size.Constant)
			}
		} else {
			// Direct matrix: cast to native matrix, e.g. ((float4x2)...)
			w.writeMatrixValueType(m)
		}
		w.out.WriteString(")")
		if err := w.writeExpression(e.Pointer); err != nil {
			return err
		}
		w.out.WriteString(")")
		return nil
	}
	// In HLSL, loads are implicit - just write the pointer expression
	return w.writeExpression(e.Pointer)
}

// getInnerMatrixOfGlobalUniform walks an access chain to determine if it
// ultimately references a uniform global variable containing a matCx2.
// Matches Rust naga get_inner_matrix_of_global_uniform.
func (w *Writer) getInnerMatrixOfGlobalUniform(handle ir.ExpressionHandle) *matrixTypeInfo {
	if w.currentFunction == nil {
		return nil
	}
	var matData *matrixTypeInfo
	var arrayBase *ir.TypeHandle

	cur := handle
	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return nil
		}
		expr := w.currentFunction.Expressions[cur]

		// Resolve the type of the current expression
		resolved := w.getExpressionTypeInner(cur)
		if resolved == nil {
			return nil
		}

		// Unwrap pointer
		if ptr, ok := resolved.(ir.PointerType); ok {
			if int(ptr.Base) < len(w.module.Types) {
				resolved = w.module.Types[ptr.Base].Inner
			}
		}

		switch resolved.(type) {
		case ir.MatrixType:
			mat := resolved.(ir.MatrixType)
			matData = &matrixTypeInfo{
				columns: mat.Columns,
				rows:    mat.Rows,
				width:   mat.Scalar.Width,
			}
		case ir.ArrayType:
			arr := resolved.(ir.ArrayType)
			ab := arr.Base
			arrayBase = &ab
		default:
			return nil
		}

		switch e := expr.Kind.(type) {
		case ir.ExprAccess:
			cur = e.Base
		case ir.ExprAccessIndex:
			cur = e.Base
		case ir.ExprGlobalVariable:
			if int(e.Variable) < len(w.module.GlobalVariables) {
				global := &w.module.GlobalVariables[e.Variable]
				if global.Space == ir.SpaceUniform {
					if matData != nil {
						return matData
					}
					if arrayBase != nil {
						return getInnerMatrixData(w.module, *arrayBase)
					}
				}
			}
			return nil
		default:
			return nil
		}
	}
}

// writeMatrixValueType writes the HLSL value type for a matrix (e.g., "float3x2").
func (w *Writer) writeMatrixValueType(m *matrixTypeInfo) {
	fmt.Fprintf(&w.out, "float%dx%d", m.columns, m.rows)
}

// =============================================================================
// Operator Expressions
// =============================================================================

// writeUnaryExpression writes a unary operation.
// For signed integer negation, uses naga_neg() helper to avoid UB.
// Matches Rust naga's Unary handling in HLSL backend.
func (w *Writer) writeUnaryExpression(e ir.ExprUnary) error {
	var op string
	switch e.Op {
	case ir.UnaryNegate:
		// Check if the operand is I32 scalar or vector — use naga_neg
		if w.isI32Negate(e.Expr) {
			op = "naga_neg"
		} else {
			op = "-"
		}
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

// isI32Negate returns true if the expression has I32 scalar type (scalar or vector).
func (w *Writer) isI32Negate(expr ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	inner := w.getExpressionTypeInner(expr)
	if inner == nil {
		return false
	}
	var scalar *ir.ScalarType
	switch t := inner.(type) {
	case ir.ScalarType:
		scalar = &t
	case ir.VectorType:
		scalar = &t.Scalar
	default:
		return false
	}
	return scalar.Kind == ir.ScalarSint && scalar.Width == 4
}

// writeBinaryExpression writes a binary operation.
func (w *Writer) writeBinaryExpression(e ir.ExprBinary) error {
	// Avoid undefined behavior for addition, subtraction, and multiplication
	// of signed 32-bit integers by casting operands to unsigned, performing
	// the operation, then casting back. Matches Rust naga's wrapping pattern.
	// TODO(#7109): This relies on asint()/asuint() which only work for 32-bit types.
	if (e.Op == ir.BinaryAdd || e.Op == ir.BinarySubtract || e.Op == ir.BinaryMultiply) && w.isI32ScalarOp(e) {
		var opStr string
		switch e.Op {
		case ir.BinaryAdd:
			opStr = "+"
		case ir.BinarySubtract:
			opStr = "-"
		case ir.BinaryMultiply:
			opStr = "*"
		}

		// Check for matrix multiply (takes priority over wrapping)
		if e.Op == ir.BinaryMultiply {
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
		}

		w.out.WriteString("asint(asuint(")
		if err := w.writeExpression(e.Left); err != nil {
			return fmt.Errorf("binary left: %w", err)
		}
		fmt.Fprintf(&w.out, ") %s asuint(", opStr)
		if err := w.writeExpression(e.Right); err != nil {
			return fmt.Errorf("binary right: %w", err)
		}
		w.out.WriteString("))")
		return nil
	}

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
		// Integer division uses naga_div for safety (matches Rust naga)
		if w.isIntegerBinaryOp(e) {
			fmt.Fprintf(&w.out, "%s(", NagaDivFunction)
			if err := w.writeExpression(e.Left); err != nil {
				return fmt.Errorf("binary left: %w", err)
			}
			w.out.WriteString(", ")
			if err := w.writeExpression(e.Right); err != nil {
				return fmt.Errorf("binary right: %w", err)
			}
			w.out.WriteByte(')')
			return nil
		}
		op = "/"
	case ir.BinaryModulo:
		// Integer/float modulo uses naga_mod for safety (matches Rust naga)
		if w.isIntOrFloatBinaryOp(e) {
			fmt.Fprintf(&w.out, "%s(", NagaModFunction)
			if err := w.writeExpression(e.Left); err != nil {
				return fmt.Errorf("binary left: %w", err)
			}
			w.out.WriteString(", ")
			if err := w.writeExpression(e.Right); err != nil {
				return fmt.Errorf("binary right: %w", err)
			}
			w.out.WriteByte(')')
			return nil
		}
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

// isIntegerBinaryOp checks if a binary op's result type is integer (Sint or Uint).
func (w *Writer) isIntegerBinaryOp(e ir.ExprBinary) bool {
	// Check left operand type for scalar kind
	leftInner := w.getExpressionTypeInner(e.Left)
	if leftInner == nil {
		return false
	}
	switch t := leftInner.(type) {
	case ir.ScalarType:
		return t.Kind == ir.ScalarSint || t.Kind == ir.ScalarUint
	case ir.VectorType:
		return t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint
	}
	return false
}

// isI32ScalarOp checks if a binary op's result type has I32 scalar component.
// Matches Rust naga's check: func_ctx.resolve_type(expr).scalar() == Some(Scalar::I32).
func (w *Writer) isI32ScalarOp(e ir.ExprBinary) bool {
	leftInner := w.getExpressionTypeInner(e.Left)
	if leftInner == nil {
		return false
	}
	switch t := leftInner.(type) {
	case ir.ScalarType:
		return t.Kind == ir.ScalarSint && t.Width == 4
	case ir.VectorType:
		return t.Scalar.Kind == ir.ScalarSint && t.Scalar.Width == 4
	}
	return false
}

// isIntOrFloatBinaryOp checks if a binary op's result type is integer or float.
func (w *Writer) isIntOrFloatBinaryOp(e ir.ExprBinary) bool {
	leftInner := w.getExpressionTypeInner(e.Left)
	if leftInner == nil {
		return false
	}
	switch t := leftInner.(type) {
	case ir.ScalarType:
		return t.Kind == ir.ScalarSint || t.Kind == ir.ScalarUint || t.Kind == ir.ScalarFloat
	case ir.VectorType:
		return t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint || t.Scalar.Kind == ir.ScalarFloat
	}
	return false
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
	// Special case: QuantizeToF16 wraps with f16tof32(f32tof16(expr))
	// Matches Rust naga's Function::QuantizeToF16 handling.
	if e.Fun == ir.MathQuantizeF16 {
		w.out.WriteString("f16tof32(f32tof16(")
		if err := w.writeExpression(e.Arg); err != nil {
			return fmt.Errorf("quantize arg: %w", err)
		}
		w.out.WriteString("))")
		return nil
	}

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
// Matches Rust naga's Expression::As handling: resolves source type shape
// (scalar/vector/matrix) to produce correct target type name.
func (w *Writer) writeAsExpression(e ir.ExprAs) error {
	if e.Convert != nil {
		// Resolve source expression type to determine shape
		srcInner := w.getExpressionTypeInner(e.Expr)

		// Check if this is a float-to-int conversion that needs a clamped helper.
		// Matches Rust naga: float -> sint/uint uses naga_f2i32/f2u32/f2i64/f2u64
		// to avoid undefined behavior for out-of-range values.
		srcIsFloat := false
		if srcInner != nil {
			switch t := srcInner.(type) {
			case ir.ScalarType:
				srcIsFloat = t.Kind == ir.ScalarFloat
			case ir.VectorType:
				srcIsFloat = t.Scalar.Kind == ir.ScalarFloat
			}
		}
		if srcIsFloat && (e.Kind == ir.ScalarSint || e.Kind == ir.ScalarUint) {
			var funcName string
			switch {
			case e.Kind == ir.ScalarSint && *e.Convert == 4:
				funcName = "naga_f2i32"
			case e.Kind == ir.ScalarUint && *e.Convert == 4:
				funcName = "naga_f2u32"
			case e.Kind == ir.ScalarSint && *e.Convert == 8:
				funcName = "naga_f2i64"
			case e.Kind == ir.ScalarUint && *e.Convert == 8:
				funcName = "naga_f2u64"
			}
			if funcName != "" {
				w.needsF2ICast = true
				if w.f2iCastFunctions == nil {
					w.f2iCastFunctions = make(map[string]struct{})
				}
				w.f2iCastFunctions[funcName] = struct{}{}
				w.out.WriteString(funcName)
				w.out.WriteByte('(')
				if err := w.writeExpression(e.Expr); err != nil {
					return fmt.Errorf("as conversion f2i: %w", err)
				}
				w.out.WriteByte(')')
				return nil
			}
		}

		scalarName := scalarKindToHLSL(e.Kind, *e.Convert)

		switch src := srcInner.(type) {
		case ir.VectorType:
			fmt.Fprintf(&w.out, "%s%d(", scalarName, src.Size)
		case ir.MatrixType:
			fmt.Fprintf(&w.out, "%s%dx%d(", scalarName, src.Columns, src.Rows)
		default:
			// Scalar or unknown — just scalar cast
			fmt.Fprintf(&w.out, "%s(", scalarName)
		}
		if err := w.writeExpression(e.Expr); err != nil {
			return fmt.Errorf("as conversion: %w", err)
		}
		w.out.WriteByte(')')
	} else {
		// Bitcast - use asfloat/asint/asuint
		// For 64-bit types, skip the cast (identity bitcast in HLSL).
		// Matches Rust naga: inner.scalar_width() == Some(8) => no cast.
		srcInner := w.getExpressionTypeInner(e.Expr)
		is64Bit := false
		if srcInner != nil {
			switch t := srcInner.(type) {
			case ir.ScalarType:
				is64Bit = t.Width == 8
			case ir.VectorType:
				is64Bit = t.Scalar.Width == 8
			}
		}
		if is64Bit {
			if err := w.writeExpression(e.Expr); err != nil {
				return fmt.Errorf("as bitcast 64-bit: %w", err)
			}
		} else {
			castFunc := ScalarCast(e.Kind)
			w.out.WriteString(castFunc)
			w.out.WriteByte('(')
			if err := w.writeExpression(e.Expr); err != nil {
				return fmt.Errorf("as bitcast: %w", err)
			}
			w.out.WriteByte(')')
		}
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
// Matches Rust naga: Width+Coarse/Fine is expanded to abs(ddx_X(e)) + abs(ddy_X(e)).
func (w *Writer) writeDerivativeExpression(e ir.ExprDerivative) error {
	// Special case: Width + Coarse/Fine -> expanded form
	if e.Axis == ir.DerivativeWidth && (e.Control == ir.DerivativeCoarse || e.Control == ir.DerivativeFine) {
		tail := "coarse"
		if e.Control == ir.DerivativeFine {
			tail = "fine"
		}
		fmt.Fprintf(&w.out, "abs(ddx_%s(", tail)
		if err := w.writeExpression(e.Expr); err != nil {
			return fmt.Errorf("derivative expr: %w", err)
		}
		fmt.Fprintf(&w.out, ")) + abs(ddy_%s(", tail)
		if err := w.writeExpression(e.Expr); err != nil {
			return fmt.Errorf("derivative expr: %w", err)
		}
		w.out.WriteString("))")
		return nil
	}

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
func (w *Writer) writeImageSampleExpression(e ir.ExprImageSample) error {
	// Handle ClampToEdge with SampleLevelZero: emit nagaTextureSampleBaseClampToEdge helper call.
	// Matches Rust naga: this case is handled before the generic ImageSample path.
	if e.ClampToEdge {
		if _, isZero := e.Level.(ir.SampleLevelZero); isZero &&
			e.DepthRef == nil && e.Gather == nil && e.ArrayIndex == nil && e.Offset == nil {
			w.needsClampToEdgeHelper = true
			w.out.WriteString("nagaTextureSampleBaseClampToEdge(")
			if err := w.writeExpression(e.Image); err != nil {
				return fmt.Errorf("clamp to edge: image: %w", err)
			}
			w.out.WriteString(", ")
			if err := w.writeExpression(e.Sampler); err != nil {
				return fmt.Errorf("clamp to edge: sampler: %w", err)
			}
			w.out.WriteString(", ")
			if err := w.writeExpression(e.Coordinate); err != nil {
				return fmt.Errorf("clamp to edge: coordinate: %w", err)
			}
			w.out.WriteByte(')')
			return nil
		}
		// ClampToEdge with non-Zero level should have been validated out
		return fmt.Errorf("ImageSample::clamp_to_edge should have been validated out")
	}

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

	// Handle gather operations.
	// Rust naga uses Gather{component}{Cmp} naming:
	// component 0 (X) = "" -> .Gather or .GatherCmp
	// component 1 (Y) = "Green" -> .GatherGreen or .GatherCmpGreen
	// etc.
	isGather := false
	if e.Gather != nil {
		isGather = true
		comp := *e.Gather
		components := [4]string{"", "Green", "Blue", "Alpha"}
		compStr := ""
		if int(comp) < 4 {
			compStr = components[comp]
		}
		cmpStr := ""
		if e.DepthRef != nil {
			cmpStr = "Cmp"
		}
		method = fmt.Sprintf(".Gather%s%s", cmpStr, compStr)
	}

	w.out.WriteString(method)
	w.out.WriteByte('(')

	// Sampler
	if err := w.writeExpression(e.Sampler); err != nil {
		return fmt.Errorf("image sample: sampler: %w", err)
	}
	w.out.WriteString(", ")

	// Coordinate (merged with array index per Rust naga's write_texture_coordinates)
	if err := w.writeTextureCoordinates("float", e.Coordinate, e.ArrayIndex, nil); err != nil {
		return fmt.Errorf("image sample: coordinate: %w", err)
	}

	// Depth reference for comparison samplers
	if e.DepthRef != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.DepthRef); err != nil {
			return fmt.Errorf("image sample: depth ref: %w", err)
		}
	}

	// Level of detail
	// For Gather operations: level is never written (Zero/Auto both produce "").
	// For SampleCmpLevelZero (depth_ref + Zero), Rust omits the level arg.
	// For SampleLevel (no depth_ref + Zero), Rust writes ", 0.0".
	if !isGather {
		if err := w.writeSampleLevel(e.Level, e.DepthRef != nil); err != nil {
			return err
		}
	}

	// Offset — wrap in int2() to work around DXC bug
	// https://github.com/microsoft/DirectXShaderCompiler/issues/5082#issuecomment-1540147807
	// Use writeConstExpression to bypass named expression baking (matches Rust naga).
	if e.Offset != nil {
		w.out.WriteString(", int2(")
		if err := w.writeConstExpression(*e.Offset); err != nil {
			return fmt.Errorf("image sample: offset: %w", err)
		}
		w.out.WriteByte(')')
	}

	w.out.WriteByte(')')
	return nil
}

// writeSampleLevel writes the level of detail argument.
// hasDepthRef indicates whether this is a comparison sample (SampleCmp*).
// For SampleCmpLevelZero, the level is implicit — no ", 0.0" is emitted.
func (w *Writer) writeSampleLevel(level ir.SampleLevel, hasDepthRef bool) error {
	switch l := level.(type) {
	case ir.SampleLevelAuto:
		// No additional arguments
	case ir.SampleLevelZero:
		if !hasDepthRef {
			// SampleLevel(..., 0.0) needs explicit level
			w.out.WriteString(", 0.0")
		}
		// For SampleCmpLevelZero: level is implicit
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
	// Check if this is an external texture -- use nagaTextureLoadExternal helper
	imgType := w.getImageTypeFromExpr(e.Image)
	if imgType != nil && imgType.Class == ir.ImageClassExternal {
		w.out.WriteString("nagaTextureLoadExternal(")
		if err := w.writeExpression(e.Image); err != nil {
			return fmt.Errorf("image load external: image: %w", err)
		}
		w.out.WriteString(", ")
		if err := w.writeExpression(e.Coordinate); err != nil {
			return fmt.Errorf("image load external: coordinate: %w", err)
		}
		w.out.WriteByte(')')
		return nil
	}

	// Check if this is a scalar storage texture that needs LoadedStorageValueFrom wrapper
	scalarHelper := w.getStorageLoadHelper(e.Image)
	if scalarHelper != "" {
		fmt.Fprintf(&w.out, "LoadedStorageValueFrom%s(", scalarHelper)
	}

	// Write image reference
	if err := w.writeExpression(e.Image); err != nil {
		return fmt.Errorf("image load: image: %w", err)
	}

	// Use Load method
	w.out.WriteString(".Load(")

	// Bundle coordinates, array index, and mip level into a single vector.
	// Matches Rust naga write_texture_coordinates("int", ...).
	if err := w.writeTextureCoordinates("int", e.Coordinate, e.ArrayIndex, e.Level); err != nil {
		return fmt.Errorf("image load: coordinates: %w", err)
	}

	// Sample index for MSAA (not bundled, passed as separate parameter)
	if e.Sample != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*e.Sample); err != nil {
			return fmt.Errorf("image load: sample: %w", err)
		}
	}

	w.out.WriteByte(')')
	if scalarHelper != "" {
		w.out.WriteByte(')') // close LoadedStorageValueFrom wrapper
	}

	// Append .x if the result type is scalar (depth textures return float, not float4).
	// Matches Rust naga: "return x component if return type is scalar".
	if w.isImageLoadResultScalar(e.Image) {
		w.out.WriteString(".x")
	}

	return nil
}

// writeTextureCoordinates bundles texture coordinates, array index, and mip level
// into a single vector parameter. Matches Rust naga help.rs write_texture_coordinates.
func (w *Writer) writeTextureCoordinates(kind string, coordinate ir.ExpressionHandle, arrayIndex *ir.ExpressionHandle, mipLevel *ir.ExpressionHandle) error {
	extra := 0
	if arrayIndex != nil {
		extra++
	}
	if mipLevel != nil {
		extra++
	}

	if extra == 0 {
		return w.writeExpression(coordinate)
	}

	// Determine coordinate dimensions
	numCoords := w.getExpressionVectorSize(coordinate)
	totalSize := numCoords + extra

	fmt.Fprintf(&w.out, "%s%d(", kind, totalSize)
	if err := w.writeExpression(coordinate); err != nil {
		return err
	}
	if arrayIndex != nil {
		w.out.WriteString(", ")
		if err := w.writeExpression(*arrayIndex); err != nil {
			return err
		}
	}
	if mipLevel != nil {
		w.out.WriteString(", ")
		// Cast uint mip level to int if needed.
		// Also wrap abstract int literals with int() to match Rust naga's behavior
		// (Rust concretizes abstract ints to i32 before writing).
		needCast := w.isExpressionUint(*mipLevel) || w.isAbstractIntLiteral(*mipLevel)
		if needCast {
			w.out.WriteString("int(")
		}
		if err := w.writeExpression(*mipLevel); err != nil {
			return err
		}
		if needCast {
			w.out.WriteString(")")
		}
	}
	w.out.WriteString(")")
	return nil
}

// getExpressionVectorSize returns the number of components of an expression (1 for scalar, 2-4 for vectors).
func (w *Writer) getExpressionVectorSize(handle ir.ExpressionHandle) int {
	inner := w.getExpressionTypeInner(handle)
	if inner != nil {
		switch t := inner.(type) {
		case ir.ScalarType:
			return 1
		case ir.VectorType:
			return int(t.Size)
		}
	}

	// Fallback: infer from expression shape (e.g., Splat, Compose)
	if w.currentFunction != nil && int(handle) < len(w.currentFunction.Expressions) {
		switch e := w.currentFunction.Expressions[handle].Kind.(type) {
		case ir.ExprSplat:
			return int(e.Size)
		case ir.ExprCompose:
			return len(e.Components)
		case ir.ExprSwizzle:
			return int(e.Size)
		}
	}

	return 2 // default assumption
}

// isExpressionUint checks if an expression has unsigned int type.
// Checks ExpressionTypes first, then falls back to examining the expression itself.
func (w *Writer) isExpressionUint(handle ir.ExpressionHandle) bool {
	inner := w.getExpressionTypeInner(handle)
	if inner != nil {
		switch t := inner.(type) {
		case ir.ScalarType:
			return t.Kind == ir.ScalarUint
		}
		return false
	}

	// Fallback: check expression kind directly for Literal and ZeroValue
	if w.currentFunction != nil && int(handle) < len(w.currentFunction.Expressions) {
		expr := w.currentFunction.Expressions[handle]
		switch e := expr.Kind.(type) {
		case ir.Literal:
			switch e.Value.(type) {
			case ir.LiteralU32, ir.LiteralU64:
				return true
			}
		case ir.ExprZeroValue:
			if int(e.Type) < len(w.module.Types) {
				if sc, ok := w.module.Types[e.Type].Inner.(ir.ScalarType); ok {
					return sc.Kind == ir.ScalarUint
				}
			}
		}
	}
	return false
}

// isAbstractIntLiteral checks if an expression is a Literal with an AbstractInt value.
// Used to wrap abstract int literals with int() in texture coordinates,
// matching Rust naga which concretizes abstract ints to i32.
func (w *Writer) isAbstractIntLiteral(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	expr := w.currentFunction.Expressions[handle]
	if lit, ok := expr.Kind.(ir.Literal); ok {
		_, isAbstract := lit.Value.(ir.LiteralAbstractInt)
		return isAbstract
	}
	return false
}

// isImageLoadResultScalar checks if an ImageLoad expression returns a scalar (depth textures).
func (w *Writer) isImageLoadResultScalar(imageHandle ir.ExpressionHandle) bool {
	imgType := w.getImageTypeFromExpr(imageHandle)
	if imgType == nil {
		return false
	}
	// Depth textures return float (scalar), not float4
	return imgType.Class == ir.ImageClassDepth
}

// getStorageLoadHelper returns the scalar type name if the image expression
// refers to a scalar storage texture that needs LoadedStorageValueFrom wrapper.
func (w *Writer) getStorageLoadHelper(imageHandle ir.ExpressionHandle) string {
	if w.currentFunction == nil || int(imageHandle) >= len(w.currentFunction.Expressions) {
		return ""
	}
	expr := w.currentFunction.Expressions[imageHandle]
	gvExpr, ok := expr.Kind.(ir.ExprGlobalVariable)
	if !ok {
		return ""
	}
	if int(gvExpr.Variable) >= len(w.module.GlobalVariables) {
		return ""
	}
	gv := &w.module.GlobalVariables[gvExpr.Variable]
	if int(gv.Type) >= len(w.module.Types) {
		return ""
	}
	img, ok := w.module.Types[gv.Type].Inner.(ir.ImageType)
	if !ok || img.Class != ir.ImageClassStorage {
		return ""
	}
	return storageFormatScalarName(img.StorageFormat)
}

// writeImageQueryExpression writes an image query operation using wrapper functions.
// Matches Rust naga: generates NagaDimensions2D(), NagaNumLevels2D(), etc.
func (w *Writer) writeImageQueryExpression(e ir.ExprImageQuery) error {
	// Get image type info
	imgType := w.getImageTypeFromExpr(e.Image)
	if imgType == nil {
		return fmt.Errorf("image query: cannot resolve image type for expression %d", e.Image)
	}

	// Determine query type
	var qt imageQueryType
	switch q := e.Query.(type) {
	case ir.ImageQuerySize:
		if q.Level != nil {
			qt = imageQuerySizeLevel
		} else {
			qt = imageQuerySize
		}
	case ir.ImageQueryNumLevels:
		qt = imageQueryNumLevels
	case ir.ImageQueryNumLayers:
		qt = imageQueryNumLayers
	case ir.ImageQueryNumSamples:
		qt = imageQueryNumSamples
	default:
		return fmt.Errorf("unsupported image query: %T", e.Query)
	}

	// Build the wrapper key
	key := wrappedImageQueryKey{
		dim:     imgType.Dim,
		arrayed: imgType.Arrayed,
		class:   imgType.Class,
		multi:   imgType.Multisampled,
		query:   qt,
	}

	// Ensure the wrapper function exists
	if _, exists := w.wrappedImageQueries[key]; !exists {
		w.wrappedImageQueries[key] = struct{}{}
	}

	// Write the call: NagaXxxDimensionsYY(tex) or NagaXxxDimensionsYY(tex, mip_level)
	w.writeImageQueryFunctionName(key)
	w.out.WriteString("(")
	if err := w.writeExpression(e.Image); err != nil {
		return fmt.Errorf("image query: image: %w", err)
	}
	if qt == imageQuerySizeLevel {
		w.out.WriteString(", ")
		q := e.Query.(ir.ImageQuerySize)
		if err := w.writeExpression(*q.Level); err != nil {
			return fmt.Errorf("image query: level: %w", err)
		}
	}
	w.out.WriteString(")")

	return nil
}

// getImageTypeFromExpr resolves the ImageType from an expression handle.
func (w *Writer) getImageTypeFromExpr(handle ir.ExpressionHandle) *ir.ImageType {
	inner := w.getExpressionTypeInner(handle)
	if inner == nil {
		return nil
	}
	if img, ok := inner.(ir.ImageType); ok {
		return &img
	}
	return nil
}

// writeImageQueryFunctionName writes the name of a wrapped image query function.
// Matches Rust naga: Naga{Class}{Query}{Dim}{Array}
func (w *Writer) writeImageQueryFunctionName(key wrappedImageQueryKey) {
	classStr := ""
	switch key.class {
	case ir.ImageClassSampled:
		if key.multi {
			classStr = "MS"
		}
	case ir.ImageClassDepth:
		if key.multi {
			classStr = "DepthMS"
		} else {
			classStr = "Depth"
		}
	case ir.ImageClassStorage:
		classStr = "RW"
	case ir.ImageClassExternal:
		classStr = "External"
	}

	queryStr := ""
	switch key.query {
	case imageQuerySize:
		queryStr = "Dimensions"
	case imageQuerySizeLevel:
		queryStr = "MipDimensions"
	case imageQueryNumLevels:
		queryStr = "NumLevels"
	case imageQueryNumLayers:
		queryStr = "NumLayers"
	case imageQueryNumSamples:
		queryStr = "NumSamples"
	}

	dimStr := ""
	switch key.dim {
	case ir.Dim1D:
		dimStr = "1D"
	case ir.Dim2D:
		dimStr = "2D"
	case ir.Dim3D:
		dimStr = "3D"
	case ir.DimCube:
		dimStr = "Cube"
	}

	arrayedStr := ""
	if key.arrayed {
		arrayedStr = "Array"
	}

	fmt.Fprintf(&w.out, "Naga%s%s%s%s", classStr, queryStr, dimStr, arrayedStr)
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
// Matches Rust naga: `((NagaBufferLength[RW](var) - offset) / stride)`
func (w *Writer) writeArrayLengthExpression(e ir.ExprArrayLength) error {
	// Resolve the global variable handle and compute offset/stride
	var varHandle ir.GlobalVariableHandle
	if w.currentFunction != nil && int(e.Array) < len(w.currentFunction.Expressions) {
		expr := w.currentFunction.Expressions[e.Array]
		switch kind := expr.Kind.(type) {
		case ir.ExprGlobalVariable:
			varHandle = kind.Variable
		case ir.ExprAccessIndex:
			// AccessIndex { base: GlobalVariable, index }
			if int(kind.Base) < len(w.currentFunction.Expressions) {
				baseExpr := w.currentFunction.Expressions[kind.Base]
				if gv, ok := baseExpr.Kind.(ir.ExprGlobalVariable); ok {
					varHandle = gv.Variable
				}
			}
		}
	}

	if int(varHandle) >= len(w.module.GlobalVariables) {
		// Fallback
		if err := w.writeExpression(e.Array); err != nil {
			return err
		}
		w.out.WriteString(".Length")
		return nil
	}

	gv := &w.module.GlobalVariables[varHandle]
	varName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(varHandle)}]

	// Determine offset and stride
	var offset, stride uint32
	if int(gv.Type) < len(w.module.Types) {
		switch inner := w.module.Types[gv.Type].Inner.(type) {
		case ir.ArrayType:
			offset = 0
			stride = inner.Stride
		case ir.StructType:
			if len(inner.Members) > 0 {
				last := inner.Members[len(inner.Members)-1]
				offset = last.Offset
				if int(last.Type) < len(w.module.Types) {
					if arr, ok := w.module.Types[last.Type].Inner.(ir.ArrayType); ok {
						stride = arr.Stride
					}
				}
			}
		}
	}

	// Determine if writable
	writable := gv.Space == ir.SpaceStorage && gv.Access == ir.StorageReadWrite

	// Mark that we need the NagaBufferLength helper
	if w.nagaBufferLengthWritten == nil {
		w.nagaBufferLengthWritten = make(map[bool]struct{})
	}
	w.nagaBufferLengthWritten[writable] = struct{}{}

	// Write: ((NagaBufferLength[RW](var) - offset) / stride)
	funcName := "NagaBufferLength"
	if writable {
		funcName = "NagaBufferLengthRW"
	}
	fmt.Fprintf(&w.out, "((%s(%s) - %d) / %d)", funcName, varName, offset, stride)
	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

// getExpressionType returns the resolved type for an expression handle.
// inferExpressionType attempts to infer the type of an expression by examining
// the expression tree. Used as fallback when ExpressionTypes is not populated.
func (w *Writer) inferExpressionType(handle ir.ExpressionHandle) *ir.Type {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}
	expr := &w.currentFunction.Expressions[handle]
	switch e := expr.Kind.(type) {
	case ir.ExprLoad:
		// Load dereferences a pointer - get the pointee type
		return w.inferPointeeType(e.Pointer)
	case ir.ExprLocalVariable:
		// Local variable type, dereferenced
		if int(e.Variable) < len(w.currentFunction.LocalVars) {
			local := &w.currentFunction.LocalVars[e.Variable]
			if int(local.Type) < len(w.module.Types) {
				ty := &w.module.Types[local.Type]
				if ptr, ok := ty.Inner.(ir.PointerType); ok {
					if int(ptr.Base) < len(w.module.Types) {
						return &w.module.Types[ptr.Base]
					}
				}
				return ty
			}
		}
	case ir.ExprFunctionArgument:
		if int(e.Index) < len(w.currentFunction.Arguments) {
			arg := &w.currentFunction.Arguments[e.Index]
			if int(arg.Type) < len(w.module.Types) {
				return &w.module.Types[arg.Type]
			}
		}
	case ir.ExprGlobalVariable:
		if int(e.Variable) < len(w.module.GlobalVariables) {
			global := &w.module.GlobalVariables[e.Variable]
			if int(global.Type) < len(w.module.Types) {
				ty := &w.module.Types[global.Type]
				if ptr, ok := ty.Inner.(ir.PointerType); ok {
					if int(ptr.Base) < len(w.module.Types) {
						return &w.module.Types[ptr.Base]
					}
				}
				return ty
			}
		}
	}
	return nil
}

// inferPointeeType gets the type that a pointer expression points to.
func (w *Writer) inferPointeeType(handle ir.ExpressionHandle) *ir.Type {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}
	expr := &w.currentFunction.Expressions[handle]
	switch e := expr.Kind.(type) {
	case ir.ExprLocalVariable:
		if int(e.Variable) < len(w.currentFunction.LocalVars) {
			local := &w.currentFunction.LocalVars[e.Variable]
			if int(local.Type) < len(w.module.Types) {
				return &w.module.Types[local.Type]
			}
		}
	case ir.ExprGlobalVariable:
		if int(e.Variable) < len(w.module.GlobalVariables) {
			global := &w.module.GlobalVariables[e.Variable]
			typeHandle := global.Type
			if int(typeHandle) < len(w.module.Types) {
				ty := &w.module.Types[typeHandle]
				if ptr, ok := ty.Inner.(ir.PointerType); ok {
					if int(ptr.Base) < len(w.module.Types) {
						return &w.module.Types[ptr.Base]
					}
				}
				return ty
			}
		}
	case ir.ExprFunctionArgument:
		if int(e.Index) < len(w.currentFunction.Arguments) {
			arg := &w.currentFunction.Arguments[e.Index]
			if int(arg.Type) < len(w.module.Types) {
				return &w.module.Types[arg.Type]
			}
		}
	case ir.ExprAccessIndex:
		// AccessIndex on a pointer: get the member type
		parentType := w.inferPointeeType(e.Base)
		if parentType != nil {
			if st, ok := parentType.Inner.(ir.StructType); ok {
				if int(e.Index) < len(st.Members) {
					if int(st.Members[e.Index].Type) < len(w.module.Types) {
						return &w.module.Types[st.Members[e.Index].Type]
					}
				}
			}
		}
	}
	return nil
}

// getExpressionTypeHandle returns the TypeHandle for an expression, if available.
func (w *Writer) getExpressionTypeHandle(handle ir.ExpressionHandle) *ir.TypeHandle {
	if w.currentFunction == nil {
		return nil
	}
	if int(handle) >= len(w.currentFunction.ExpressionTypes) {
		return nil
	}
	resolution := &w.currentFunction.ExpressionTypes[handle]
	return resolution.Handle
}

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

// resolveStructTypeHandleForAccess resolves the struct type handle for a base expression
// in an AccessIndex. This handles the case where the base expression's type is stored as
// a Value (e.g., PointerType for GlobalVariable) rather than a Handle.
func (w *Writer) resolveStructTypeHandleForAccess(base ir.ExpressionHandle) *ir.TypeHandle {
	// Path 1: Try the expression type Handle directly
	th := w.getExpressionTypeHandle(base)
	if th != nil {
		h := *th
		if int(h) < len(w.module.Types) {
			// If it's a pointer type, dereference
			if ptr, ok := w.module.Types[h].Inner.(ir.PointerType); ok {
				return &ptr.Base
			}
			// If it's already a struct type, return as-is
			if _, ok := w.module.Types[h].Inner.(ir.StructType); ok {
				return th
			}
		}
	}

	// Path 2: Resolve through Value (PointerType stored as Value, not Handle)
	if w.currentFunction != nil && int(base) < len(w.currentFunction.ExpressionTypes) {
		res := &w.currentFunction.ExpressionTypes[base]
		if res.Value != nil {
			if ptr, ok := res.Value.(ir.PointerType); ok {
				return &ptr.Base
			}
		}
	}

	// Path 3: Trace through the expression chain (GlobalVariable -> global type)
	if w.currentFunction != nil && int(base) < len(w.currentFunction.Expressions) {
		expr := w.currentFunction.Expressions[base]
		if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
			if int(gv.Variable) < len(w.module.GlobalVariables) {
				g := &w.module.GlobalVariables[gv.Variable]
				return &g.Type
			}
		}
	}

	return nil
}

// writeRayQueryGetIntersection writes a ray query intersection access.
// Matches Rust naga: `GetCommittedIntersection(rq)` or `GetCandidateIntersection(rq)`.
func (w *Writer) writeRayQueryGetIntersection(e ir.ExprRayQueryGetIntersection) error {
	if e.Committed {
		w.out.WriteString("GetCommittedIntersection(")
		w.needsCommittedIntersectionHelper = true
	} else {
		w.out.WriteString("GetCandidateIntersection(")
		w.needsCandidateIntersectionHelper = true
	}
	if err := w.writeExpression(e.Query); err != nil {
		return fmt.Errorf("ray query get intersection query: %w", err)
	}
	w.out.WriteByte(')')
	return nil
}

// formatSpecialFloat handles special float values like inf and nan.
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

// =============================================================================
// Binding Array and Non-Uniformity Support
// =============================================================================

// bindingArraySamplerInfo holds information for generating sampler heap access
// for a binding_array<sampler> or binding_array<sampler_comparison>.
// Matches Rust naga's BindingArraySamplerInfo.
type bindingArraySamplerInfo struct {
	// samplerHeapName is the variable name of the sampler heap
	// ("nagaSamplerHeap" or "nagaComparisonSamplerHeap")
	samplerHeapName string
	// samplerIndexBufferName is the variable name of the sampler index buffer
	// (e.g., "nagaGroup0SamplerIndexArray")
	samplerIndexBufferName string
	// bindingArrayBaseIndexName is the variable name of the base index into
	// the sampler index buffer (the global variable name, e.g., "samp")
	bindingArrayBaseIndexName string
}

// samplerBindingArrayInfoFromExpression determines if an expression is a binding
// array of samplers and returns the info needed to generate the heap access pattern.
// Matches Rust naga's sampler_binding_array_info_from_expression.
func (w *Writer) samplerBindingArrayInfoFromExpression(base ir.ExpressionHandle) *bindingArraySamplerInfo {
	if w.currentFunction == nil || int(base) >= len(w.currentFunction.Expressions) {
		return nil
	}

	// Resolve the base expression's type
	baseType := w.getExpressionType(base)
	if baseType == nil {
		baseType = w.inferExpressionType(base)
	}
	if baseType == nil {
		return nil
	}
	inner := baseType.Inner
	if ptr, ok := inner.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			inner = w.module.Types[ptr.Base].Inner
		}
	}

	ba, ok := inner.(ir.BindingArrayType)
	if !ok {
		return nil
	}
	if int(ba.Base) >= len(w.module.Types) {
		return nil
	}
	samplerType, ok := w.module.Types[ba.Base].Inner.(ir.SamplerType)
	if !ok {
		return nil
	}

	// The base expression must be a GlobalVariable
	baseExpr := w.currentFunction.Expressions[base]
	gvExpr, ok := baseExpr.Kind.(ir.ExprGlobalVariable)
	if !ok {
		return nil
	}
	gv := &w.module.GlobalVariables[gvExpr.Variable]

	heapName := "nagaSamplerHeap"
	if samplerType.Comparison {
		heapName = "nagaComparisonSamplerHeap"
	}

	group := uint32(0)
	if gv.Binding != nil {
		group = gv.Binding.Group
	}

	indexBufName := w.samplerIndexBuffers[group]
	if indexBufName == "" {
		indexBufName = fmt.Sprintf("nagaGroup%dSamplerIndexArray", group)
	}

	baseName := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(gvExpr.Variable)}]

	return &bindingArraySamplerInfo{
		samplerHeapName:           heapName,
		samplerIndexBufferName:    indexBufName,
		bindingArrayBaseIndexName: baseName,
	}
}

// isBindingArrayAccess returns true if base expression resolves to a BindingArray type.
func (w *Writer) isBindingArrayAccess(base ir.ExpressionHandle) bool {
	baseType := w.getExpressionType(base)
	if baseType == nil {
		baseType = w.inferExpressionType(base)
	}
	if baseType == nil {
		return false
	}
	inner := baseType.Inner
	if ptr, ok := inner.(ir.PointerType); ok {
		if int(ptr.Base) < len(w.module.Types) {
			inner = w.module.Types[ptr.Base].Inner
		}
	}
	_, ok := inner.(ir.BindingArrayType)
	return ok
}

// isNonUniform checks if an expression's value is non-uniform (varies across invocations
// in a subgroup/workgroup). This is a simplified version of Rust naga's uniformity analysis.
// Returns true if the expression is potentially non-uniform.
// Matches Rust naga's Uniformity.non_uniform_result.is_some() check.
func (w *Writer) isNonUniform(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	return w.isNonUniformRecursive(handle, 0)
}

func (w *Writer) isNonUniformRecursive(handle ir.ExpressionHandle, depth int) bool {
	if depth > 20 {
		return false // Prevent infinite recursion
	}
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}

	expr := w.currentFunction.Expressions[handle]
	switch e := expr.Kind.(type) {
	case ir.Literal:
		return false // Constants are uniform
	case ir.ExprConstant:
		return false // Constants are uniform
	case ir.ExprOverride:
		return false // Overrides are uniform
	case ir.ExprZeroValue:
		return false // Zero values are uniform
	case ir.ExprSplat:
		return w.isNonUniformRecursive(e.Value, depth+1)
	case ir.ExprFunctionArgument:
		// Function arguments are non-uniform unless they're specific per-workgroup builtins
		if int(e.Index) < len(w.currentFunction.Arguments) {
			arg := &w.currentFunction.Arguments[e.Index]
			if arg.Binding != nil {
				if bb, ok := (*arg.Binding).(ir.BuiltinBinding); ok {
					switch bb.Builtin {
					case ir.BuiltinWorkGroupID, ir.BuiltinNumWorkGroups:
						return false
					}
				}
			}
		}
		return true // Most function arguments are non-uniform (fragment inputs, etc.)
	case ir.ExprGlobalVariable:
		if int(e.Variable) < len(w.module.GlobalVariables) {
			gv := &w.module.GlobalVariables[e.Variable]
			switch gv.Space {
			case ir.SpaceUniform:
				return false // Uniform data
			case ir.SpaceWorkGroup:
				return false // Workgroup memory is per-group (uniform within group)
			case ir.SpaceHandle:
				return false // Handle space (textures/samplers) is uniform
			case ir.SpaceStorage:
				if gv.Access == ir.StorageRead {
					return false // Read-only storage is uniform
				}
				return true // Read-write storage is non-uniform
			default:
				return true // Private, Function are non-uniform
			}
		}
		return false
	case ir.ExprLocalVariable:
		return true // Local variables are non-uniform (could have been written with non-uniform data)
	case ir.ExprLoad:
		return w.isNonUniformRecursive(e.Pointer, depth+1)
	case ir.ExprAccess:
		// Non-uniform if either base or index is non-uniform
		return w.isNonUniformRecursive(e.Base, depth+1) || w.isNonUniformRecursive(e.Index, depth+1)
	case ir.ExprAccessIndex:
		return w.isNonUniformRecursive(e.Base, depth+1)
	case ir.ExprUnary:
		return w.isNonUniformRecursive(e.Expr, depth+1)
	case ir.ExprBinary:
		return w.isNonUniformRecursive(e.Left, depth+1) || w.isNonUniformRecursive(e.Right, depth+1)
	case ir.ExprSelect:
		return w.isNonUniformRecursive(e.Condition, depth+1) ||
			w.isNonUniformRecursive(e.Accept, depth+1) ||
			w.isNonUniformRecursive(e.Reject, depth+1)
	case ir.ExprAs:
		return w.isNonUniformRecursive(e.Expr, depth+1)
	case ir.ExprCompose:
		for _, comp := range e.Components {
			if w.isNonUniformRecursive(comp, depth+1) {
				return true
			}
		}
		return false
	case ir.ExprSwizzle:
		return w.isNonUniformRecursive(e.Vector, depth+1)
	default:
		return true // Conservative: unknown expressions are non-uniform
	}
}

// isBindingArrayOfSamplers checks if a global variable is a binding array of samplers.
// Used to skip writing the global variable name in expressions (Rust naga does the same).
func (w *Writer) isBindingArrayOfSamplers(gvHandle ir.GlobalVariableHandle) bool {
	if int(gvHandle) >= len(w.module.GlobalVariables) {
		return false
	}
	gv := &w.module.GlobalVariables[gvHandle]
	if int(gv.Type) >= len(w.module.Types) {
		return false
	}
	ba, ok := w.module.Types[gv.Type].Inner.(ir.BindingArrayType)
	if !ok {
		return false
	}
	if int(ba.Base) >= len(w.module.Types) {
		return false
	}
	_, isSampler := w.module.Types[ba.Base].Inner.(ir.SamplerType)
	return isSampler
}
