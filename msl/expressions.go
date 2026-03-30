package msl

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gogpu/naga/ir"
)

// Type name constants for MSL codegen
const (
	mslFloat = "float"
	mslHalf  = "half"
	mslInt   = "int"
	mslUint  = "uint"
	mslBool  = "bool"

	// Packed vector types for dot4/pack/unpack operations
	mslPackedChar4  = "packed_char4"
	mslPackedUchar4 = "packed_uchar4"
)

// scalarCastTypeName returns the MSL type name for a scalar cast.
// Uses the convert width to distinguish 32-bit vs 64-bit integer/float types.
func (w *Writer) scalarCastTypeName(kind ir.ScalarKind, convert *uint8) string {
	width := uint8(4) // default 32-bit
	if convert != nil {
		width = *convert
	}
	switch kind {
	case ir.ScalarFloat:
		if width == 2 {
			return mslHalf
		}
		return mslFloat
	case ir.ScalarSint:
		if width == 8 {
			return "long"
		}
		return mslInt
	case ir.ScalarUint:
		if width == 8 {
			return "ulong"
		}
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

// writeAccessChain writes an Access/AccessIndex chain, recursing through the
// chain without checking named expressions at intermediate levels. This matches
// Rust naga's put_access_chain behavior where the full chain is always inlined,
// only falling back to put_expression for the terminal base expression.
func (w *Writer) writeAccessChain(handle ir.ExpressionHandle) error {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return w.writeExpression(handle)
	}

	expr := &w.currentFunction.Expressions[handle]

	switch k := expr.Kind.(type) {
	case ir.ExprAccessIndex:
		baseType := w.getExpressionType(k.Base)

		// Look through pointers.
		if pt, ok := baseType.(ir.PointerType); ok {
			if int(pt.Base) < len(w.module.Types) {
				baseType = w.module.Types[pt.Base].Inner
			}
		}

		switch bt := baseType.(type) {
		case ir.StructType:
			if err := w.writeAccessChain(k.Base); err != nil {
				return err
			}
			typeHandle := w.getExpressionTypeHandle(k.Base)
			if typeHandle != nil {
				memberName := w.getName(nameKey{kind: nameKeyStructMember, handle1: uint32(*typeHandle), handle2: k.Index})
				w.write(".%s", memberName)
				return nil
			}
			if int(k.Index) < len(bt.Members) {
				w.write(".%s", escapeName(bt.Members[k.Index].Name))
			} else {
				w.write(".member_%d", k.Index)
			}
			return nil
		case ir.VectorType:
			if err := w.writeAccessChain(k.Base); err != nil {
				return err
			}
			if !w.isPackedVec3Access(k.Base) {
				components := [4]string{"x", "y", "z", "w"}
				if k.Index < uint32(len(components)) {
					w.write(".%s", components[k.Index])
					return nil
				}
			}
			w.write("[%d]", k.Index)
			return nil
		}

		// For arrays and other types, fall through to writeExpression.
		return w.writeExpression(handle)

	case ir.ExprAccess:
		// For dynamic access, also walk the chain.
		return w.writeExpression(handle)

	default:
		// Terminal base: use writeExpression (respects named expressions).
		return w.writeExpression(handle)
	}
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
func (w *Writer) writeExpressionKind(kind ir.ExpressionKind, handle ir.ExpressionHandle) error {
	switch k := kind.(type) {
	case ir.Literal:
		return w.writeLiteral(k)

	case ir.ExprConstant:
		// Abstract constants (from untyped const declarations like "const one = 1;")
		// are removed by Rust naga's process_overrides pass and their values inlined.
		// We inline them here at output time to match Rust's behavior.
		if int(k.Constant) < len(w.module.Constants) {
			c := &w.module.Constants[k.Constant]
			if c.IsAbstract {
				if c.Value == nil {
					// Value is nil, use Init (GlobalExpressions handle) instead.
					return w.writeGlobalExpression(c.Init)
				}
				return w.writeConstantValueInline(c.Value, c.Type)
			}
		}
		name := w.getName(nameKey{kind: nameKeyConstant, handle1: uint32(k.Constant)})
		w.write("%s", name)
		return nil

	case ir.ExprOverride:
		// Overrides are written as constants in MSL output.
		// In Rust naga, process_overrides resolves these before the writer runs.
		// We handle them directly by referencing the override's assigned name.
		name := w.getName(nameKey{kind: nameKeyOverride, handle1: uint32(k.Override)})
		w.write("%s", name)
		return nil

	case ir.ExprZeroValue:
		return w.writeZeroValue(k.Type)

	case ir.ExprCompose:
		return w.writeCompose(k)

	case ir.ExprAccess:
		// Check if this is a top-level value access that needs RZSW wrapping.
		// Pointer-based accesses are handled by writeLoad/writeStore instead.
		// insideRZSW prevents nested wrapping for inner chain expressions.
		if !w.insideRZSW {
			policy := w.chooseBoundsCheckPolicy(k.Base)
			if policy == BoundsCheckReadZeroSkipWrite {
				if check, ok := w.buildRZSWBoundsCheck(handle); ok {
					w.write("%s ? ", check)
					w.insideRZSW = true
					if err := w.writeAccess(k); err != nil {
						w.insideRZSW = false
						return err
					}
					w.insideRZSW = false
					w.writeRZSWFallback(k.Base, handle)
					return nil
				}
			}
		}
		return w.writeAccess(k)

	case ir.ExprAccessIndex:
		// Check if this is a top-level value access that needs RZSW wrapping.
		// insideRZSW prevents nested wrapping for inner chain expressions.
		if !w.insideRZSW {
			policy := w.chooseBoundsCheckPolicy(k.Base)
			if policy == BoundsCheckReadZeroSkipWrite {
				if check, ok := w.buildRZSWBoundsCheck(handle); ok {
					w.write("%s ? ", check)
					w.insideRZSW = true
					if err := w.writeAccessIndex(k); err != nil {
						w.insideRZSW = false
						return err
					}
					w.insideRZSW = false
					w.writeRZSWFallback(k.Base, handle)
					return nil
				}
			}
		}
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
		return w.writeImageLoad(k, handle)

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

	case ir.ExprSubgroupBallotResult:
		w.write("/* subgroup ballot result */")
		return nil

	case ir.ExprSubgroupOperationResult:
		w.write("/* subgroup op result */")
		return nil

	case ir.ExprRayQueryProceedResult:
		w.write("/* ray query proceed result */")
		return nil

	case ir.ExprRayQueryGetIntersection:
		return w.writeRayQueryGetIntersection(k)

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
		// Rust naga uses (-MAX - 1) pattern for INT_MIN to avoid C++ literal overflow.
		// MSL treats -2147483648 as -(2147483648u) which overflows signed int.
		if int32(v) == -2147483648 {
			w.write("(-2147483647 - 1)")
		} else {
			w.write("%d", int32(v))
		}

	case ir.LiteralU32:
		w.write("%du", uint32(v))

	case ir.LiteralI64:
		// Same INT_MIN pattern for 64-bit.
		if int64(v) == -9223372036854775808 {
			w.write("(-9223372036854775807L - 1L)")
		} else {
			w.write("%dL", int64(v))
		}

	case ir.LiteralU64:
		w.write("%duL", uint64(v))

	case ir.LiteralF16:
		val := float32(v)
		if math.IsInf(float64(val), 0) {
			if val < 0 {
				w.write("-INFINITY")
			} else {
				w.write("INFINITY")
			}
		} else if math.IsNaN(float64(val)) {
			w.write("NAN")
		} else {
			// Format f16 with 'h' suffix, matching Rust naga.
			s := strconv.FormatFloat(float64(val), 'f', -1, 32)
			// If integer, add ".0h" suffix; otherwise just "h".
			if !strings.Contains(s, ".") {
				s += ".0h"
			} else {
				s += "h"
			}
			w.write("%s", s)
		}

	case ir.LiteralF32:
		val := float32(v)
		if math.IsInf(float64(val), 0) {
			if val < 0 {
				w.write("-INFINITY")
			} else {
				w.write("INFINITY")
			}
		} else if math.IsNaN(float64(val)) {
			w.write("NAN")
		} else {
			// Use strconv with 'f' format (no exponent) matching Rust's Display for f32.
			s := strconv.FormatFloat(float64(val), 'f', -1, 32)
			if !strings.Contains(s, ".") {
				s += ".0"
			}
			w.write("%s", s)
		}

	case ir.LiteralF64:
		val := float64(v)
		if math.IsInf(val, 0) {
			if val < 0 {
				w.write("-INFINITY")
			} else {
				w.write("INFINITY")
			}
		} else if math.IsNaN(val) {
			w.write("NAN")
		} else {
			s := strconv.FormatFloat(val, 'f', -1, 64)
			if !strings.Contains(s, ".") {
				s += ".0"
			}
			w.write("%s", s)
		}

	case ir.LiteralAbstractInt:
		w.write("%d", int64(v))

	case ir.LiteralAbstractFloat:
		// Abstract floats should ideally be concretized before reaching the backend,
		// but as a fallback emit them as f32 literals (matching Rust naga's behavior).
		val := float32(v)
		if math.IsInf(float64(val), 0) {
			if val < 0 {
				w.write("-INFINITY")
			} else {
				w.write("INFINITY")
			}
		} else if math.IsNaN(float64(val)) {
			w.write("NAN")
		} else {
			s := strconv.FormatFloat(float64(val), 'f', -1, 32)
			if !strings.Contains(s, ".") {
				s += ".0"
			}
			w.write("%s", s)
		}

	default:
		return fmt.Errorf("unsupported literal type: %T", lit.Value)
	}
	return nil
}

// writeZeroValue writes a zero-initialized value using brace initialization.
// Matches Rust naga output: metal::int2 {} instead of metal::int2().
func (w *Writer) writeZeroValue(typeHandle ir.TypeHandle) error {
	typeName := w.writeTypeName(typeHandle, StorageAccess(0))
	w.write("%s {}", typeName)
	return nil
}

// writeConstantValueInline writes an abstract constant's value inline,
// recursively expanding composite values. This matches Rust naga where
// abstract constants are removed by process_overrides and their values
// are inlined at each use site.
func (w *Writer) writeConstantValueInline(value ir.ConstantValue, typeHandle ir.TypeHandle) error {
	if value == nil {
		// Value is nil — this shouldn't happen for inline constants,
		// but handle gracefully with empty braces.
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		w.write("%s {}", typeName)
		return nil
	}
	switch v := value.(type) {
	case ir.ScalarValue:
		return w.writeScalarValue(v, typeHandle)
	case ir.CompositeValue:
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		// Check if it's a splat (all components are the same constant)
		isSplat := len(v.Components) > 1
		if isSplat {
			first := v.Components[0]
			for _, c := range v.Components[1:] {
				if c != first {
					isSplat = false
					break
				}
			}
		}
		if isSplat && len(v.Components) > 0 {
			w.write("%s(", typeName)
			comp := &w.module.Constants[v.Components[0]]
			if err := w.writeConstantValueInline(comp.Value, comp.Type); err != nil {
				return err
			}
			w.write(")")
		} else {
			w.write("%s(", typeName)
			for i, ch := range v.Components {
				if i > 0 {
					w.write(", ")
				}
				comp := &w.module.Constants[ch]
				if err := w.writeConstantValueInline(comp.Value, comp.Type); err != nil {
					return err
				}
			}
			w.write(")")
		}
		return nil
	case ir.ZeroConstantValue:
		return w.writeZeroValue(typeHandle)
	default:
		// Fallback: write the type with empty braces
		typeName := w.writeTypeName(typeHandle, StorageAccess(0))
		w.write("%s {}", typeName)
		return nil
	}
}

// writeCompose writes a composite construction.
func (w *Writer) writeCompose(compose ir.ExprCompose) error {
	typeName := w.writeTypeName(compose.Type, StorageAccess(0))
	useBraces := false
	if _, ok := w.arrayWrappers[compose.Type]; ok {
		useBraces = true
	} else if int(compose.Type) < len(w.module.Types) {
		if _, ok := w.module.Types[compose.Type].Inner.(ir.StructType); ok {
			useBraces = true
		}
	}

	// Detect vector splat: all components reference the same expression handle.
	// Emit single-arg constructor like metal::float4(1.0) instead of float4(1.0, 1.0, 1.0, 1.0).
	// Matches Rust naga MSL output.
	if !useBraces && len(compose.Components) > 1 && w.isVectorSplat(compose) {
		w.write("%s(", typeName)
		if err := w.writeExpression(compose.Components[0]); err != nil {
			return err
		}
		w.write(")")
		return nil
	}

	if useBraces {
		w.write("%s {", typeName)
	} else {
		w.write("%s(", typeName)
	}
	for i, component := range compose.Components {
		if i > 0 {
			w.write(", ")
		}
		// Insert padding initializer for struct members that have padding before them.
		// Rust naga adds {} for each padding member in aggregate struct construction.
		if useBraces && int(compose.Type) < len(w.module.Types) {
			padKey := nameKey{kind: nameKeyStructMember, handle1: uint32(compose.Type), handle2: uint32(i)}
			if _, hasPad := w.structPads[padKey]; hasPad {
				w.write("{}, ")
			}
		}
		if err := w.writeExpression(component); err != nil {
			return err
		}
	}
	if useBraces {
		w.write("}")
	} else {
		w.write(")")
	}
	return nil
}

// isVectorSplat returns true if all components in a compose reference the
// same expression handle, indicating a splat constructor.
func (w *Writer) isVectorSplat(compose ir.ExprCompose) bool {
	if int(compose.Type) >= len(w.module.Types) {
		return false
	}
	if _, ok := w.module.Types[compose.Type].Inner.(ir.VectorType); !ok {
		return false
	}
	first := compose.Components[0]
	for _, c := range compose.Components[1:] {
		if c != first {
			return false
		}
	}
	return true
}

// writeAccess writes a dynamic index access.
// When the current access chain has Restrict bounds checking policy,
// wraps the index in metal::min(unsigned(index), maxIndex) for fixed-size arrays.
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
				if err := w.writeAccessIndex_restricted(access.Base, access.Index); err != nil {
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
			if err := w.writeAccessIndex_restricted(access.Base, access.Index); err != nil {
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
			if err := w.writeAccessIndex_restricted(access.Base, access.Index); err != nil {
				return err
			}
			w.write("]")
			return nil
		}
	}

	// BindingArray access: element is wrapped in NagaArgumentBufferWrapper → need .inner
	isBindingArray := false
	if baseType != nil {
		switch bt := baseType.(type) {
		case ir.BindingArrayType:
			isBindingArray = true
		case ir.PointerType:
			if int(bt.Base) < len(w.module.Types) {
				if _, ok := w.module.Types[bt.Base].Inner.(ir.BindingArrayType); ok {
					isBindingArray = true
				}
			}
		}
	}

	if err := w.writeExpression(access.Base); err != nil {
		return err
	}
	w.write("[")
	if err := w.writeAccessIndex_restricted(access.Base, access.Index); err != nil {
		return err
	}
	w.write("]")
	if isBindingArray {
		w.write(".inner")
	}
	return nil
}

// writeAccessIndex_restricted writes an index expression, applying Restrict bounds
// clamping if the active bounds check policy requires it.
// Uses currentAccessPolicy if set (from writeLoad), otherwise determines policy
// from the base expression's address space (for direct value accesses).
func (w *Writer) writeAccessIndex_restricted(baseHandle ir.ExpressionHandle, indexHandle ir.ExpressionHandle) error {
	policy := w.currentAccessPolicy
	if policy == BoundsCheckUnchecked {
		// Not inside a writeLoad — determine policy from base expression.
		policy = w.chooseBoundsCheckPolicy(baseHandle)
	}
	if policy == BoundsCheckRestrict {
		if maxIndex, needsRestrict := w.accessNeedsRestrict(baseHandle, indexHandle); needsRestrict {
			return w.writeRestrictedIndex(indexHandle, maxIndex)
		}
		// Fallback: try to get length from the original type in the module.
		// When type resolution doesn't produce the right array type (e.g., it resolves to the
		// runtime-sized array instead of the fixed-size one), try resolving by traversing
		// the expression tree to find the actual array type.
		if length, ok := w.indexableLengthDeep(baseHandle); ok && length > 0 {
			return w.writeRestrictedIndex(indexHandle, length-1)
		}
		// Runtime-sized array: emit metal::min(unsigned(index), (_buffer_sizes.sizeN - offset - stride) / stride)
		if info, ok := w.resolveRuntimeArrayInfo(baseHandle); ok {
			return w.writeRestrictedDynamicIndex(indexHandle, info)
		}
	}
	return w.writeExpression(indexHandle)
}

// writeAccessIndex writes a constant index access.
func (w *Writer) writeAccessIndex(access ir.ExprAccessIndex) error {
	// Special handling for modf/frexp result member access.
	// In Rust naga, modf/frexp return a struct and AccessIndex extracts members:
	//   index 0 = fract, index 1 = whole (modf) or exp (frexp)
	if w.currentFunction != nil && int(access.Base) < len(w.currentFunction.Expressions) {
		if mathExpr, ok := w.currentFunction.Expressions[access.Base].Kind.(ir.ExprMath); ok {
			switch mathExpr.Fun {
			case ir.MathModf:
				if err := w.writeModf(mathExpr); err != nil {
					return err
				}
				switch access.Index {
				case 0:
					w.write(".fract")
				case 1:
					w.write(".whole")
				}
				return nil
			case ir.MathFrexp:
				if err := w.writeFrexp(mathExpr); err != nil {
					return err
				}
				switch access.Index {
				case 0:
					w.write(".fract")
				case 1:
					w.write(".exp")
				}
				return nil
			}
		}
	}

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
				innerType := w.module.Types[pt.Base].Inner
				if st, ok := innerType.(ir.StructType); ok {
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
				// Vector through pointer: use .x/.y/.z/.w or [index] for packed vec3.
				// MSL uses swizzle for vector elements even through pointers,
				// but packed vec3 (struct members in storage) need bracket notation.
				// Matches Rust naga: ValuePointer | Vector → if packed { [index] } else { .component }
				if _, ok := innerType.(ir.VectorType); ok {
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					if w.isPackedVec3Access(access.Base) {
						w.write("[%d]", access.Index)
					} else {
						components := []string{"x", "y", "z", "w"}
						if int(access.Index) < len(components) {
							w.write(".%s", components[access.Index])
						} else {
							w.write("[%d]", access.Index)
						}
					}
					return nil
				}
				// Matrix through pointer: use [N] column access
				if _, ok := innerType.(ir.MatrixType); ok {
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					w.write("[%d]", access.Index)
					return nil
				}
				// Array through pointer: may need Restrict bounds check
				if _, ok := innerType.(ir.ArrayType); ok {
					if err := w.writeExpression(access.Base); err != nil {
						return err
					}
					w.write("[")
					policy := w.currentAccessPolicy
					if policy == BoundsCheckUnchecked {
						policy = w.chooseBoundsCheckPolicy(access.Base)
					}
					if policy == BoundsCheckRestrict {
						if info, ok := w.resolveRuntimeArrayInfo(access.Base); ok {
							w.write("metal::min(unsigned(%d), (_buffer_sizes.size%d - %d - %d) / %d)",
								access.Index, info.globalIdx, info.memberOffset, info.elemStride, info.elemStride)
							w.write("]")
							return nil
						}
					}
					w.write("%d]", access.Index)
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
	// BindingArray access: element is wrapped in NagaArgumentBufferWrapper → need .inner
	isBindingArray := false
	if baseType != nil {
		switch bt := baseType.(type) {
		case ir.BindingArrayType:
			isBindingArray = true
		case ir.PointerType:
			if int(bt.Base) < len(w.module.Types) {
				if _, ok := w.module.Types[bt.Base].Inner.(ir.BindingArrayType); ok {
					isBindingArray = true
				}
			}
		}
	}
	if isBindingArray {
		if err := w.writeExpression(access.Base); err != nil {
			return err
		}
		w.write("[%d].inner", access.Index)
		return nil
	}
	if baseType != nil {
		if st, ok := baseType.(ir.StructType); ok {
			// Struct member access -- use writeAccessChain to walk the full
			// access chain without stopping at named expressions, matching
			// Rust naga's put_access_chain behavior.
			if err := w.writeAccessChain(access.Base); err != nil {
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

	// Array, vector, or matrix access.
	// Wrap in parens if base is a binary/select expression (matches Rust naga is_scoped=false).
	needParens := w.needsParensInContext(access.Base)
	if needParens {
		w.write("(")
	}
	if err := w.writeExpression(access.Base); err != nil {
		return err
	}
	if needParens {
		w.write(")")
	}

	// Use swizzle notation for vectors (Rust naga uses .x/.y/.z/.w),
	// but use bracket notation [idx] for packed vec3 members since packed types
	// don't support swizzle syntax in MSL.
	// ValuePointerType with Size also counts as vector (pointer to column from matrix access).
	isVec := false
	switch baseType.(type) {
	case ir.VectorType:
		isVec = true
	case ir.ValuePointerType:
		vp := baseType.(ir.ValuePointerType)
		isVec = vp.Size != nil
	}
	if isVec {
		if !w.isPackedVec3Access(access.Base) {
			components := [4]string{"x", "y", "z", "w"}
			if access.Index < uint32(len(components)) {
				w.write(".%s", components[access.Index])
				return nil
			}
		}
	}

	// Check if this is an array access that needs Restrict bounds checking.
	// For runtime-sized arrays, even constant indices need clamping.
	// Also handle PointerType{ArrayType} (struct member array through pointer).
	isArray := false
	switch bt := baseType.(type) {
	case ir.ArrayType:
		isArray = true
	case ir.PointerType:
		if int(bt.Base) < len(w.module.Types) {
			_, isArray = w.module.Types[bt.Base].Inner.(ir.ArrayType)
		}
	}
	if isArray {
		policy := w.currentAccessPolicy
		if policy == BoundsCheckUnchecked {
			policy = w.chooseBoundsCheckPolicy(access.Base)
		}
		if policy == BoundsCheckRestrict {
			if info, ok := w.resolveRuntimeArrayInfo(access.Base); ok {
				// Runtime-sized array: metal::min(unsigned(index), (_buffer_sizes.sizeN - offset - stride) / stride)
				w.write("[metal::min(unsigned(%d), (_buffer_sizes.size%d - %d - %d) / %d)]",
					access.Index, info.globalIdx, info.memberOffset, info.elemStride, info.elemStride)
				return nil
			}
			// Fixed-size array: clamp if index >= length
			if length, ok := w.indexableLengthDeep(access.Base); ok && length > 0 && access.Index >= length {
				w.write("[metal::min(unsigned(%d), %du)]", access.Index, length-1)
				return nil
			}
		}
	}

	w.write("[%d]", access.Index)
	return nil
}

// needsParens checks if a child expression needs parentheses in the context
// of a parent binary operator. Matches Rust naga's is_scoped approach: any
// binary expression used as an operand of another binary expression gets
// wrapped in parentheses, regardless of precedence.
// Exception: div/mod operations that are rewritten to naga_div/naga_mod helper
// function calls are already scoped by the function call parentheses.
func (w *Writer) needsParens(child ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(child) >= len(w.currentFunction.Expressions) {
		return false
	}
	// Named expressions are rendered as variable names, not inline expressions,
	// so they never need parentheses.
	if _, ok := w.namedExpressions[child]; ok {
		return false
	}
	childExpr := w.currentFunction.Expressions[child]
	switch k := childExpr.Kind.(type) {
	case ir.ExprBinary:
		// Div/mod on integers are rewritten to naga_div/naga_mod function calls,
		// which are already parenthesized. Don't add extra parens.
		if k.Op == ir.BinaryDivide || k.Op == ir.BinaryModulo {
			if _, isInt := w.getIntegerOverload(k.Left); isInt {
				return false
			}
		}
		return true
	case ir.ExprArrayLength:
		// ArrayLength expands to "1 + ..." which contains a binary operator.
		// Matches Rust naga: ArrayLength uses is_scoped wrapping.
		return true
	default:
		return false
	}
}

// needsParensInContext checks if an expression would need parentheses when
// rendered in a "non-scoped" context (matching Rust naga's is_scoped=false).
// In Rust naga, Binary and Select expressions add outer parens when !is_scoped.
func (w *Writer) needsParensInContext(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	// If this expression is a named expression (already baked), it's just a name reference
	// and doesn't need parens.
	if _, ok := w.namedExpressions[handle]; ok {
		return false
	}
	if w.currentFunction.NamedExpressions != nil {
		if _, ok := w.currentFunction.NamedExpressions[handle]; ok {
			return false
		}
	}
	expr := w.currentFunction.Expressions[handle]
	switch expr.Kind.(type) {
	case ir.ExprBinary:
		return true
	case ir.ExprSelect:
		return true
	case ir.ExprArrayLength:
		// ArrayLength expands to "1 + ..." which needs parenthesization.
		// Matches Rust naga: ArrayLength uses is_scoped wrapping.
		return true
	}
	return false
}

// isMatrixType checks if the given expression resolves to a matrix type.
func (w *Writer) isMatrixType(handle ir.ExpressionHandle) bool {
	t := w.getExpressionType(handle)
	_, ok := t.(ir.MatrixType)
	return ok
}

// writeWrappedBinaryOperand writes a binary operand, wrapping packed vec3 in a
// cast to regular vec3 if wrapPackedVec3 is true. Also adds parens if needed.
// Matches Rust naga put_wrapped_expression_for_packed_vec3_access.
func (w *Writer) writeWrappedBinaryOperand(handle ir.ExpressionHandle, wrapPackedVec3 bool) error {
	needsPacked := wrapPackedVec3 && w.isPackedVec3Access(handle)
	if needsPacked {
		// Get the scalar type of the packed vec3
		t := w.getExpressionType(handle)
		scalarName := "float"
		if vec, ok := t.(ir.VectorType); ok {
			scalarName = scalarTypeName(vec.Scalar)
		}
		w.write("%s%s3(", Namespace, scalarName)
	}

	if w.needsParens(handle) {
		w.write("(")
		if err := w.writeExpression(handle); err != nil {
			return err
		}
		w.write(")")
	} else {
		if err := w.writeExpression(handle); err != nil {
			return err
		}
	}

	if needsPacked {
		w.write(")")
	}
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
	// For packed vec3 members, swizzle requires wrapping in metal::float3(...)
	// because MSL packed types don't support multi-component swizzle syntax.
	needsUnpack := w.isPackedVec3Access(swizzle.Vector)
	if needsUnpack {
		// Determine the unpacked vector type name (e.g., metal::float3)
		vecType := w.getExpressionType(swizzle.Vector)
		if vt, ok := vecType.(ir.VectorType); ok {
			w.write("%s%s%d(", Namespace, scalarTypeName(vt.Scalar), vt.Size)
		} else {
			w.write("%sfloat3(", Namespace)
		}
	}
	if err := w.writeExpression(swizzle.Vector); err != nil {
		return err
	}
	if needsUnpack {
		w.write(")")
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

// writeLoad writes a pointer load with bounds checking.
// Matches Rust naga's put_load: determines bounds check policy from address space,
// then applies Restrict (index clamping) or ReadZeroSkipWrite (ternary) as needed.
func (w *Writer) writeLoad(load ir.ExprLoad) error {
	// Determine the bounds check policy for this access chain.
	policy := w.chooseBoundsCheckPolicy(load.Pointer)

	// Set the policy for the access chain traversal.
	prevPolicy := w.currentAccessPolicy
	w.currentAccessPolicy = policy
	defer func() { w.currentAccessPolicy = prevPolicy }()

	// ReadZeroSkipWrite: wrap in ternary if pointer chain has array access.
	if policy == BoundsCheckReadZeroSkipWrite {
		if check, ok := w.buildRZSWBoundsCheck(load.Pointer); ok {
			w.write("%s ? ", check)
			w.currentAccessPolicy = BoundsCheckUnchecked // inner access is unchecked
			prevRZSW := w.insideRZSW
			w.insideRZSW = true
			if err := w.writeUncheckedLoad(load.Pointer); err != nil {
				w.insideRZSW = prevRZSW
				return err
			}
			w.insideRZSW = prevRZSW
			w.write(" : DefaultConstructible()")
			return nil
		}
	}

	return w.writeUncheckedLoad(load.Pointer)
}

// writeUncheckedLoad writes a load without bounds checking.
// If the pointer is to an atomic type, wraps with metal::atomic_load_explicit.
// Matches Rust naga MSL put_unchecked_load.
func (w *Writer) writeUncheckedLoad(pointer ir.ExpressionHandle) error {
	if w.isAtomicPointer(pointer) {
		w.write("metal::atomic_load_explicit(&")
		if err := w.writeExpression(pointer); err != nil {
			return err
		}
		w.write(", metal::memory_order_relaxed)")
		return nil
	}

	if !w.shouldDerefPointer(pointer) {
		return w.writeExpression(pointer)
	}
	w.write("(*")
	if err := w.writeExpression(pointer); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeRZSWFallback writes the fallback value for RZSW ternary expressions.
// If the access chain originates from a pointer, uses a named oob local variable
// instead of DefaultConstructible(), because we need an actual addressable local
// for pointer targets. Matches Rust naga's put_expression for Access/AccessIndex.
func (w *Writer) writeRZSWFallback(base ir.ExpressionHandle, accessHandle ir.ExpressionHandle) {
	if w.accessChainRootIsPointer(accessHandle) {
		// Find the element type that the oob local should have.
		// Access through pointer returns PointerType as a Value resolution,
		// so we need to extract the Base from it.
		resultType := w.getExpressionType(accessHandle)
		if resultType != nil {
			var elementTypeHandle ir.TypeHandle
			found := false
			if pt, ok := resultType.(ir.PointerType); ok {
				elementTypeHandle = pt.Base
				found = true
			} else if h := w.getExpressionTypeHandle(accessHandle); h != nil {
				elementTypeHandle = *h
				found = true
			}
			if found {
				if name, exists := w.oobLocals[elementTypeHandle]; exists {
					w.write(" : %s", name)
					return
				}
			}
		}
	}
	w.write(" : DefaultConstructible()")
}

// accessChainRootIsPointer walks up an access chain (Access/AccessIndex) to find
// the root expression and checks if it has a pointer type.
func (w *Writer) accessChainRootIsPointer(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil {
		return false
	}
	cur := handle
	for {
		if int(cur) >= len(w.currentFunction.Expressions) {
			return false
		}
		expr := w.currentFunction.Expressions[cur]
		switch k := expr.Kind.(type) {
		case ir.ExprAccess:
			cur = k.Base
		case ir.ExprAccessIndex:
			cur = k.Base
		default:
			// Reached the root — check if it's a pointer
			rootType := w.getExpressionType(cur)
			if rootType == nil {
				return false
			}
			_, isPtr := rootType.(ir.PointerType)
			return isPtr
		}
	}
}

// buildRZSWBoundsCheck walks an access chain and builds a RZSW bounds check string.
// Returns ("uint(i) < N && uint(j) < M", true) for chains needing runtime checks,
// or ("", false) if no check is needed.
// Matches Rust naga's put_bounds_checks + bounds_check_iter.
func (w *Writer) buildRZSWBoundsCheck(handle ir.ExpressionHandle) (string, bool) {
	// Collect all checks by walking the access chain.
	type boundsCheck struct {
		indexExpr     *ir.ExpressionHandle // non-nil for dynamic Access
		indexConst    *uint32              // non-nil for constant AccessIndex
		lengthKnown   uint32               // > 0 for known lengths
		lengthDynamic bool                 // true for runtime-sized arrays
		base          ir.ExpressionHandle  // base of this access (for originating global)
	}

	var checks []boundsCheck
	chain := handle

	for {
		if w.currentFunction == nil || int(chain) >= len(w.currentFunction.Expressions) {
			break
		}
		expr := &w.currentFunction.Expressions[chain]
		switch k := expr.Kind.(type) {
		case ir.ExprAccess:
			length, isDynamic := w.indexableLengthOrDynamic(k.Base)
			if isDynamic {
				// Dynamic array — always needs check
				idx := k.Index
				checks = append(checks, boundsCheck{
					indexExpr:     &idx,
					lengthDynamic: true,
					base:          k.Base,
				})
			} else if length > 0 {
				// Check if index is statically in bounds
				needsCheck := true
				if int(k.Index) < len(w.currentFunction.Expressions) {
					indexExpr := &w.currentFunction.Expressions[k.Index]
					if lit, ok := indexExpr.Kind.(ir.Literal); ok {
						var indexVal uint32
						switch v := lit.Value.(type) {
						case ir.LiteralI32:
							if int32(v) >= 0 {
								indexVal = uint32(v)
							}
						case ir.LiteralU32:
							indexVal = uint32(v)
						}
						if indexVal < length {
							needsCheck = false
						}
					}
				}
				if needsCheck {
					idx := k.Index
					checks = append(checks, boundsCheck{
						indexExpr:   &idx,
						lengthKnown: length,
						base:        k.Base,
					})
				}
			}
			chain = k.Base
			continue

		case ir.ExprAccessIndex:
			// Skip struct field accesses — they never need bounds checks.
			baseType := w.getExpressionType(k.Base)
			if baseType != nil {
				inner := baseType
				if pt, ok := inner.(ir.PointerType); ok {
					if int(pt.Base) < len(w.module.Types) {
						inner = w.module.Types[pt.Base].Inner
					}
				}
				if _, isStruct := inner.(ir.StructType); isStruct {
					chain = k.Base
					continue
				}
			}
			// Non-struct AccessIndex — check if constant index is in bounds
			length, isDynamic := w.indexableLengthOrDynamic(k.Base)
			if isDynamic {
				idx := k.Index
				checks = append(checks, boundsCheck{
					indexConst:    &idx,
					lengthDynamic: true,
					base:          k.Base,
				})
			} else if length > 0 && k.Index >= length {
				idx := k.Index
				checks = append(checks, boundsCheck{
					indexConst:  &idx,
					lengthKnown: length,
					base:        k.Base,
				})
			}
			// If constant index < length, no check needed (in bounds)
			chain = k.Base
			continue
		}
		break
	}

	if len(checks) == 0 {
		return "", false
	}

	// Build the condition string. Checks are in outer-to-inner order (from the
	// initial walk), which matches Rust naga's bounds_check_iter ordering.
	savedOut := w.out
	w.out = strings.Builder{}

	for i, c := range checks {
		if i > 0 {
			w.write(" && ")
		}
		// Write "uint(index) < length"
		w.write("uint(")
		if c.indexExpr != nil {
			_ = w.writeExpression(*c.indexExpr)
		} else if c.indexConst != nil {
			w.write("%d", *c.indexConst)
		}
		w.write(") < ")

		if c.lengthDynamic {
			// Runtime-sized array: 1 + (_buffer_sizes.sizeN - offset - elemSize) / stride
			w.write("1 + ")
			w.writeRuntimeArrayMaxIndex(c.base)
		} else {
			w.write("%d", c.lengthKnown)
		}
	}

	result := w.out.String()
	w.out = savedOut
	return result, true
}

// writeRuntimeArrayMaxIndex emits the runtime array max index formula:
// (_buffer_sizes.sizeN - offset - elemSize) / stride
// The base expression should resolve to a global variable containing a runtime-sized array.
func (w *Writer) writeRuntimeArrayMaxIndex(base ir.ExpressionHandle) {
	// Find the global variable
	globalIdx, globalType, found := w.resolveGlobalVariable(base)
	if !found {
		w.write("0")
		return
	}
	global := &w.module.GlobalVariables[globalIdx]
	_ = global

	// Determine offset and array type
	var offset uint32
	var arrayTypeHandle ir.TypeHandle
	if int(globalType) < len(w.module.Types) {
		switch gt := w.module.Types[globalType].Inner.(type) {
		case ir.StructType:
			if len(gt.Members) > 0 {
				last := gt.Members[len(gt.Members)-1]
				offset = last.Offset
				arrayTypeHandle = last.Type
			}
		case ir.ArrayType:
			offset = 0
			arrayTypeHandle = globalType
		}
	}

	var elemSize, stride uint32
	if int(arrayTypeHandle) < len(w.module.Types) {
		if at, ok := w.module.Types[arrayTypeHandle].Inner.(ir.ArrayType); ok {
			stride = at.Stride
			elemSize = w.typeSize(at.Base)
		}
	}

	if stride == 0 {
		w.write("0")
		return
	}

	w.write("(_buffer_sizes.size%d - %d - %d) / %d",
		globalIdx, offset, elemSize, stride)
}

// writeUnary writes a unary operation.
func (w *Writer) writeUnary(unary ir.ExprUnary) error {
	var op string
	switch unary.Op {
	case ir.UnaryNegate:
		// Signed integer negation uses naga_neg() polyfill to avoid UB on INT_MIN.
		// Matches Rust naga: as_type<T>(-as_type<unsigned_T>(val))
		if argScalar := w.getExpressionScalarType(unary.Expr); argScalar != nil && argScalar.Kind == ir.ScalarSint {
			var vecSize ir.VectorSize
			if argType := w.getExpressionType(unary.Expr); argType != nil {
				if vec, ok := argType.(ir.VectorType); ok {
					vecSize = vec.Size
				}
			}
			w.registerNegHelper(*argScalar, vecSize)
			w.write("naga_neg(")
			if err := w.writeExpression(unary.Expr); err != nil {
				return err
			}
			w.write(")")
			return nil
		}
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
func (w *Writer) writeBinary(binary ir.ExprBinary, _ ir.ExpressionHandle) error {
	// Handle special cases
	switch binary.Op {
	case ir.BinaryDivide:
		// Use safe division helper for integers (typed overloads, not templates).
		// Matches Rust naga: per-type naga_div using metal::select.
		if o, ok := w.getIntegerOverload(binary.Left); ok {
			w.addDivOverload(o)
			w.write("naga_div(")
			if err := w.writeExpression(binary.Left); err != nil {
				return err
			}
			w.write(", ")
			if err := w.writeExpression(binary.Right); err != nil {
				return err
			}
			w.write(")")
			return nil
		}

	case ir.BinaryModulo:
		// Use safe modulo helper for integers (typed overloads, not templates).
		// Matches Rust naga: per-type naga_mod using metal::select.
		if o, ok := w.getIntegerOverload(binary.Left); ok {
			w.addModOverload(o)
			w.write("naga_mod(")
			if err := w.writeExpression(binary.Left); err != nil {
				return err
			}
			w.write(", ")
			if err := w.writeExpression(binary.Right); err != nil {
				return err
			}
			w.write(")")
			return nil
		}
		// Float modulo uses metal::fmod(), not the % operator.
		// Matches Rust naga MSL backend.
		w.write("%sfmod(", Namespace)
		if err := w.writeExpression(binary.Left); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(binary.Right); err != nil {
			return err
		}
		w.write(")")
		return nil
	}

	// Wrapping arithmetic for signed integers.
	// WGSL specifies wrapping semantics for i32 add/sub/mul, but C++ signed
	// overflow is UB. Rust naga emits as_type<int>(as_type<uint>(lhs) OP as_type<uint>(rhs))
	// to perform the operation in unsigned domain and reinterpret.
	if binary.Op == ir.BinaryAdd || binary.Op == ir.BinarySubtract || binary.Op == ir.BinaryMultiply {
		if signedType, unsignedType, ok := w.getSignedWrappingTypes(binary.Left); ok {
			// For scalar*vector, result type should match vector operand
			if resultSigned, _, rok := w.getSignedWrappingTypes(binary.Right); rok {
				if len(resultSigned) > len(signedType) {
					signedType = resultSigned
				}
			}
			var op string
			switch binary.Op {
			case ir.BinaryAdd:
				op = "+"
			case ir.BinarySubtract:
				op = "-"
			case ir.BinaryMultiply:
				op = "*"
			}
			// Determine right operand's unsigned type (may differ for vec*scalar)
			rightUnsignedType := unsignedType
			if _, rightUnsigned, ok := w.getSignedWrappingTypes(binary.Right); ok {
				rightUnsignedType = rightUnsigned
			}
			w.write("as_type<%s>(as_type<%s>(", signedType, unsignedType)
			if err := w.writeExpression(binary.Left); err != nil {
				return err
			}
			w.write(") %s as_type<%s>(", op, rightUnsignedType)
			if err := w.writeExpression(binary.Right); err != nil {
				return err
			}
			w.write("))")
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

	// Write left operand, with packed vec3 wrapping if multiplying by a matrix.
	// Packed vector - matrix multiplications are not supported in MSL.
	// Matches Rust naga put_wrapped_expression_for_packed_vec3_access.
	leftIsMatrix := w.isMatrixType(binary.Left)
	rightIsMatrix := w.isMatrixType(binary.Right)

	if err := w.writeWrappedBinaryOperand(binary.Left, binary.Op == ir.BinaryMultiply && rightIsMatrix); err != nil {
		return err
	}

	w.write(" %s ", op)

	// Write right operand, with packed vec3 wrapping if multiplying by a matrix.
	if err := w.writeWrappedBinaryOperand(binary.Right, binary.Op == ir.BinaryMultiply && leftIsMatrix); err != nil {
		return err
	}

	return nil
}

// writeSelect writes a select (ternary) operation.
// Rust naga: scalar bool condition uses ternary (condition ? accept : reject),
// vector bool condition uses metal::select(reject, accept, condition).
func (w *Writer) writeSelect(sel ir.ExprSelect) error {
	// Check if condition is vector bool → use metal::select
	condType := w.getExpressionType(sel.Condition)
	if vec, ok := condType.(ir.VectorType); ok && vec.Scalar.Kind == ir.ScalarBool {
		// metal::select(reject, accept, condition) — note Metal's arg order
		w.write("%sselect(", Namespace)
		if err := w.writeExpression(sel.Reject); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(sel.Accept); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(sel.Condition); err != nil {
			return err
		}
		w.write(")")
		return nil
	}

	// Scalar bool: ternary operator. Matches Rust naga: condition ? accept : reject
	// Rust naga passes is_scoped=false for the condition sub-expression, which adds
	// parens around binary operations. We check explicitly and wrap if needed.
	needCondParens := w.needsParensInContext(sel.Condition)
	if needCondParens {
		w.write("(")
	}
	if err := w.writeExpression(sel.Condition); err != nil {
		return err
	}
	if needCondParens {
		w.write(")")
	}
	w.write(" ? ")
	if err := w.writeExpression(sel.Accept); err != nil {
		return err
	}
	w.write(" : ")
	return w.writeExpression(sel.Reject)
}

// writeMath writes a math function call.
func (w *Writer) writeMath(mathExpr ir.ExprMath) error {
	funcName := mathFunctionName(mathExpr.Fun)

	// Handle special cases that don't follow the standard pattern
	switch mathExpr.Fun {
	case ir.MathSign:
		// MSL sign() is only defined for float types. For signed integers,
		// Rust naga emits: select(select(T(-1), T(1), (x > 0)), T(0), (x == 0))
		if argType := w.getExpressionType(mathExpr.Arg); argType != nil {
			var scalar ir.ScalarType
			var vecSize ir.VectorSize
			switch t := argType.(type) {
			case ir.ScalarType:
				scalar = t
			case ir.VectorType:
				scalar = t.Scalar
				vecSize = t.Size
			}
			if scalar.Kind == ir.ScalarSint {
				typeName := w.scalarCastTypeName(scalar.Kind, &scalar.Width)
				if vecSize > 0 {
					typeName = fmt.Sprintf("%s%s%d", Namespace, typeName, vecSize)
				}
				w.write("metal::select(metal::select(%s(-1), %s(1), (", typeName, typeName)
				if err := w.writeExpression(mathExpr.Arg); err != nil {
					return err
				}
				w.write(" > 0)), %s(0), (", typeName)
				if err := w.writeExpression(mathExpr.Arg); err != nil {
					return err
				}
				w.write(" == 0))")
				return nil
			}
		}

	case ir.MathAbs:
		// For signed integers, MSL abs() may not work correctly (especially for i64).
		// Rust naga emits naga_abs() polyfill using metal::select and as_type.
		if argType := w.getExpressionType(mathExpr.Arg); argType != nil {
			var scalar ir.ScalarType
			var vecSize ir.VectorSize
			switch t := argType.(type) {
			case ir.ScalarType:
				scalar = t
			case ir.VectorType:
				scalar = t.Scalar
				vecSize = t.Size
			}
			if scalar.Kind == ir.ScalarSint {
				w.registerAbsHelper(scalar, vecSize)
				w.write("naga_abs(")
				if err := w.writeExpression(mathExpr.Arg); err != nil {
					return err
				}
				w.write(")")
				return nil
			}
		}
		// Float types use metal::abs directly (fall through to generic handler)

	case ir.MathDot:
		// For integer vectors, MSL metal::dot doesn't support int/uint.
		// Rust naga emits naga_dot_{type}{size}() wrapper functions.
		if argType := w.getExpressionType(mathExpr.Arg); argType != nil {
			if vec, ok := argType.(ir.VectorType); ok && (vec.Scalar.Kind == ir.ScalarSint || vec.Scalar.Kind == ir.ScalarUint) {
				funcName := w.registerDotWrapper(vec.Scalar, vec.Size)
				w.write("%s(", funcName)
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
			}
		}
		// Float vectors use metal::dot directly
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

	case ir.MathUnpack4xI8, ir.MathUnpack4xU8:
		return w.writeUnpack4x8(mathExpr)

	case ir.MathPack4xI8, ir.MathPack4xU8, ir.MathPack4xI8Clamp, ir.MathPack4xU8Clamp:
		return w.writePack4x8(mathExpr)

	case ir.MathDot4I8Packed, ir.MathDot4U8Packed:
		return w.writeDot4Packed(mathExpr)

	case ir.MathModf:
		return w.writeModf(mathExpr)

	case ir.MathFrexp:
		return w.writeFrexp(mathExpr)

	case ir.MathLdexp:
		// MSL: metal::ldexp(mantissa, exponent)
		w.write("%sldexp(", Namespace)
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

	case ir.MathQuantizeF16:
		return w.writeQuantizeF16(mathExpr)

	case ir.MathFirstTrailingBit:
		// Rust naga: (((metal::ctz(x) + 1) % 33) - 1)
		// Maps -1 (all ones) for input 0.
		w.write("(((%sctz(", Namespace)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(") + 1) %s 33) - 1)", "%")
		return nil

	case ir.MathFirstLeadingBit:
		// Rust naga uses metal::select patterns:
		//   signed:   metal::select(31 - metal::clz(metal::select(v, ~v, v < 0)), T(-1), v == 0 || v == -1)
		//   unsigned: metal::select(31 - metal::clz(v), T(-1), v == 0 || v == -1)
		scalar := w.getExpressionScalarType(mathExpr.Arg)
		isSigned := scalar != nil && scalar.Kind == ir.ScalarSint
		bitWidth := uint8(4) // default 32-bit
		if scalar != nil {
			bitWidth = scalar.Width
		}
		maxBit := (uint32(bitWidth) * 8) - 1 // 31 for 32-bit, 63 for 64-bit
		// Get the result type name for T(-1) cast (without metal:: prefix)
		resultType := w.firstLeadingBitResultType(mathExpr.Arg)

		w.write("%sselect(", Namespace)
		// Value when non-zero: (maxBit - metal::clz(v)) or (maxBit - metal::clz(select(v, ~v, v < 0)))
		w.write("%d - %sclz(", maxBit, Namespace)
		if isSigned {
			w.write("%sselect(", Namespace)
			if err := w.writeExpression(mathExpr.Arg); err != nil {
				return err
			}
			w.write(", ~")
			if err := w.writeExpression(mathExpr.Arg); err != nil {
				return err
			}
			w.write(", ")
			if err := w.writeExpression(mathExpr.Arg); err != nil {
				return err
			}
			w.write(" < 0)")
		} else {
			if err := w.writeExpression(mathExpr.Arg); err != nil {
				return err
			}
		}
		w.write("), ")
		// Default value: T(-1)
		w.write("%s(-1), ", resultType)
		// Condition: v == 0 || v == -1
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(" == 0 || ")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(" == -1)")
		return nil

	case ir.MathExtractBits:
		// Rust naga: extract_bits(e, min(offset, 32u), min(count, 32u - min(offset, 32u)))
		w.write("%sextract_bits(", Namespace)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(", %smin(", Namespace)
		if mathExpr.Arg1 != nil {
			if err := w.writeExpression(*mathExpr.Arg1); err != nil {
				return err
			}
		}
		w.write(", 32u), %smin(", Namespace)
		if mathExpr.Arg2 != nil {
			if err := w.writeExpression(*mathExpr.Arg2); err != nil {
				return err
			}
		}
		w.write(", 32u - %smin(", Namespace)
		if mathExpr.Arg1 != nil {
			if err := w.writeExpression(*mathExpr.Arg1); err != nil {
				return err
			}
		}
		w.write(", 32u)))")
		return nil

	case ir.MathInsertBits:
		// Rust naga: insert_bits(e, newbits, min(offset, 32u), min(count, 32u - min(offset, 32u)))
		w.write("%sinsert_bits(", Namespace)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(", ")
		if mathExpr.Arg1 != nil {
			if err := w.writeExpression(*mathExpr.Arg1); err != nil {
				return err
			}
		}
		w.write(", %smin(", Namespace)
		if mathExpr.Arg2 != nil {
			if err := w.writeExpression(*mathExpr.Arg2); err != nil {
				return err
			}
		}
		w.write(", 32u), %smin(", Namespace)
		if mathExpr.Arg3 != nil {
			if err := w.writeExpression(*mathExpr.Arg3); err != nil {
				return err
			}
		}
		w.write(", 32u - %smin(", Namespace)
		if mathExpr.Arg2 != nil {
			if err := w.writeExpression(*mathExpr.Arg2); err != nil {
				return err
			}
		}
		w.write(", 32u)))")
		return nil

	case ir.MathPack2x16float:
		// Rust naga: as_type<uint>(half2(x))
		w.write("as_type<uint>(half2(")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write("))")
		return nil

	case ir.MathUnpack2x16float:
		// Rust naga: float2(as_type<half2>(x)) — no metal:: prefix
		w.write("float2(as_type<half2>(")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write("))")
		return nil

	case ir.MathDegrees:
		// Rust naga expands degrees to multiplication: ((x) * 57.295779513082322865)
		w.write("((")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(") * 57.295779513082322865)")
		return nil

	case ir.MathRadians:
		// Rust naga expands radians to multiplication: ((x) * 0.017453292519943295474)
		w.write("((")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(") * 0.017453292519943295474)")
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
		return "" // handled specially above
	case ir.MathDegrees:
		return "" // handled specially above
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
		return "" // handled specially with bounds clamping
	case ir.MathInsertBits:
		return "" // handled specially with bounds clamping
	case ir.MathFirstTrailingBit:
		return "" // handled specially: (((ctz(x)+1) % 33) - 1)
	case ir.MathFirstLeadingBit:
		return "" // handled specially: clz-based expansion
	case ir.MathPack4x8snorm:
		return "pack_float_to_snorm4x8"
	case ir.MathPack4x8unorm:
		return "pack_float_to_unorm4x8"
	case ir.MathPack2x16snorm:
		return "pack_float_to_snorm2x16"
	case ir.MathPack2x16unorm:
		return "pack_float_to_unorm2x16"
	case ir.MathPack2x16float:
		return "" // handled specially: as_type<uint>(half2(x))
	case ir.MathUnpack4x8snorm:
		return "unpack_snorm4x8_to_float"
	case ir.MathUnpack4x8unorm:
		return "unpack_unorm4x8_to_float"
	case ir.MathUnpack2x16snorm:
		return "unpack_snorm2x16_to_float"
	case ir.MathUnpack2x16unorm:
		return "unpack_unorm2x16_to_float"
	case ir.MathUnpack2x16float:
		return "" // handled specially: float2(as_type<half2>(x))
	case ir.MathPack4xI8:
		return "" // handled by writePack4x8
	case ir.MathPack4xU8:
		return "" // handled by writePack4x8
	case ir.MathPack4xI8Clamp:
		return "" // handled by writePack4x8
	case ir.MathPack4xU8Clamp:
		return "" // handled by writePack4x8
	case ir.MathUnpack4xI8:
		return "" // handled by writeUnpack4x8
	case ir.MathUnpack4xU8:
		return "" // handled by writeUnpack4x8
	case ir.MathDot4I8Packed:
		return "" // handled by writeDot4Packed
	case ir.MathDot4U8Packed:
		return "" // handled by writeDot4Packed
	case ir.MathModf:
		return "" // handled by writeModf
	case ir.MathFrexp:
		return "" // handled by writeFrexp
	case ir.MathLdexp:
		return "" // handled specially
	case ir.MathQuantizeF16:
		return "" // handled by writeQuantizeF16
	default:
		return fmt.Sprintf("unknown_math_%d", fun)
	}
}

// writeUnpack4x8 writes an unpack4xI8 or unpack4xU8 expansion.
// Matches Rust naga MSL writer: for Metal < 2.1, expands to bit manipulation;
// for Metal >= 2.1, uses as_type<packed_[u]char4>.
func (w *Writer) writeUnpack4x8(mathExpr ir.ExprMath) error {
	signed := mathExpr.Fun == ir.MathUnpack4xI8
	signPrefix := ""
	if !signed {
		signPrefix = "u"
	}

	if !w.options.LangVersion.Less(Version2_1) {
		// Metal >= 2.1: use as_type casting between packed chars and scalars
		w.write("%sint4(as_type<packed_%schar4>(", signPrefix, signPrefix)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write("))")
	} else {
		// Metal < 2.1: bit manipulation expansion
		// (int4(arg, arg >> 8, arg >> 16, arg >> 24) << 24 >> 24)
		w.write("(%sint4(", signPrefix)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(" >> 8, ")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(" >> 16, ")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write(" >> 24) << 24 >> 24)")
	}
	return nil
}

// writePack4x8 writes a pack4xI8, pack4xU8, pack4xI8Clamp, or pack4xU8Clamp expansion.
// Matches Rust naga MSL writer: for Metal < 2.1, expands to bit manipulation;
// for Metal >= 2.1, uses as_type<uint>(packed_[u]char4(...)).
func (w *Writer) writePack4x8(mathExpr ir.ExprMath) error {
	signed := mathExpr.Fun == ir.MathPack4xI8 || mathExpr.Fun == ir.MathPack4xI8Clamp
	clamped := mathExpr.Fun == ir.MathPack4xI8Clamp || mathExpr.Fun == ir.MathPack4xU8Clamp

	writeArg := func() error {
		if clamped {
			if signed {
				w.write("%sclamp(", Namespace)
				if err := w.writeExpression(mathExpr.Arg); err != nil {
					return err
				}
				w.write(", -128, 127)")
			} else {
				w.write("%sclamp(", Namespace)
				if err := w.writeExpression(mathExpr.Arg); err != nil {
					return err
				}
				w.write(", 0, 255)")
			}
		} else {
			return w.writeExpression(mathExpr.Arg)
		}
		return nil
	}

	if !w.options.LangVersion.Less(Version2_1) {
		// Metal >= 2.1: use as_type casting
		packedType := mslPackedUchar4
		if signed {
			packedType = mslPackedChar4
		}
		w.write("as_type<uint>(%s(", packedType)
		if err := writeArg(); err != nil {
			return err
		}
		w.write("))")
	} else {
		// Metal < 2.1: bit manipulation expansion
		if signed {
			w.write("uint(")
		}
		w.write("(")
		if err := writeArg(); err != nil {
			return err
		}
		w.write("[0] & 0xFF) | ((")
		if err := writeArg(); err != nil {
			return err
		}
		w.write("[1] & 0xFF) << 8) | ((")
		if err := writeArg(); err != nil {
			return err
		}
		w.write("[2] & 0xFF) << 16) | ((")
		if err := writeArg(); err != nil {
			return err
		}
		w.write("[3] & 0xFF) << 24)")
		if signed {
			w.write(")")
		}
	}
	return nil
}

// writeModf writes a modf() call. Rust naga emits naga_modf() which returns a
// _modf_result_* struct with {fract, whole} fields.
func (w *Writer) writeModf(mathExpr ir.ExprMath) error {
	scalar, vectorSize := w.getExprArgScalarAndVec(mathExpr.Arg)
	if scalar == nil {
		scalar = &ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	w.registerModfResult(*scalar, vectorSize)
	w.write("naga_modf(")
	if err := w.writeExpression(mathExpr.Arg); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeFrexp writes a frexp() call. Rust naga emits naga_frexp() which returns a
// _frexp_result_* struct with {fract, exp} fields.
func (w *Writer) writeFrexp(mathExpr ir.ExprMath) error {
	scalar, vectorSize := w.getExprArgScalarAndVec(mathExpr.Arg)
	if scalar == nil {
		scalar = &ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
	w.registerFrexpResult(*scalar, vectorSize)
	w.write("naga_frexp(")
	if err := w.writeExpression(mathExpr.Arg); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeQuantizeF16 writes a quantizeToF16 call.
// MSL: float(half(arg)) for scalar, float2(half2(arg)) for vector, etc.
func (w *Writer) writeQuantizeF16(mathExpr ir.ExprMath) error {
	_, vectorSize := w.getExprArgScalarAndVec(mathExpr.Arg)
	if vectorSize == 0 {
		w.write("float(half(")
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write("))")
	} else {
		w.write("%sfloat%d(%shalf%d(", Namespace, vectorSize, Namespace, vectorSize)
		if err := w.writeExpression(mathExpr.Arg); err != nil {
			return err
		}
		w.write("))")
	}
	return nil
}

// getExprArgScalarAndVec returns the scalar type and vector size of an expression argument.
// For scalar types, vectorSize is 0. For vector types, vectorSize is 2/3/4.
func (w *Writer) getExprArgScalarAndVec(handle ir.ExpressionHandle) (*ir.ScalarType, ir.VectorSize) {
	inner := w.getExpressionType(handle)
	if inner == nil {
		return nil, 0
	}
	switch t := inner.(type) {
	case ir.ScalarType:
		return &t, 0
	case ir.VectorType:
		return &t.Scalar, t.Size
	default:
		return nil, 0
	}
}

// writeDot4Packed writes a dot4I8Packed or dot4U8Packed expansion.
// Matches Rust naga MSL writer: for Metal < 2.1, expands to bit shift extraction;
// for Metal >= 2.1, uses as_type<packed_[u]char4> with indexed component access.
func (w *Writer) writeDot4Packed(mathExpr ir.ExprMath) error {
	signed := mathExpr.Fun == ir.MathDot4I8Packed

	if !w.options.LangVersion.Less(Version2_1) {
		// Metal >= 2.1: use reinterpreted packed char variables.
		// The reinterpreted variables were emitted by writeEmit before this expression.
		packedType := mslPackedUchar4
		if signed {
			packedType = mslPackedChar4
		}

		argName := w.reinterpretedVarName(packedType, mathExpr.Arg)
		arg1Name := ""
		if mathExpr.Arg1 != nil {
			arg1Name = w.reinterpretedVarName(packedType, *mathExpr.Arg1)
		}

		w.write("( + %s[0] * %s[0] + %s[1] * %s[1] + %s[2] * %s[2] + %s[3] * %s[3])",
			argName, arg1Name, argName, arg1Name, argName, arg1Name, argName, arg1Name)
	} else {
		// Metal < 2.1: bit shift extraction polyfill.
		// For signed: (int(a) << shift >> 24) * (int(b) << shift >> 24)
		// For unsigned: ((a) << shift >> 24) * ((b) << shift >> 24)
		conversion := ""
		if signed {
			conversion = mslInt
		}

		w.write("(")
		for i := 0; i < 4; i++ {
			w.write(" + ")
			// Write component extraction for arg
			w.writeDot4Component(mathExpr.Arg, conversion, i)
			w.write(" * ")
			// Write component extraction for arg1
			if mathExpr.Arg1 != nil {
				w.writeDot4Component(*mathExpr.Arg1, conversion, i)
			}
		}
		w.write(")")
	}
	return nil
}

// writeDot4Component writes one component extraction for the dot4 polyfill.
// For signed (conversion != ""): (int(expr) << shift >> 24)
// For unsigned (conversion == ""): ((expr) << shift >> 24)
// For index 3: >> 24 only (no left shift)
func (w *Writer) writeDot4Component(handle ir.ExpressionHandle, conversion string, index int) {
	w.write("(")
	if conversion != "" {
		// Signed: int(expr) wraps the expression
		w.write("%s(", conversion)
		_ = w.writeExpression(handle)
		w.write(")")
	} else {
		// Unsigned: (expr) — wrap in parens to match Rust naga
		w.write("(")
		_ = w.writeExpression(handle)
		w.write(")")
	}
	if index == 3 {
		w.write(" >> 24)")
	} else {
		shift := (3 - index) * 8
		w.write(" << %d >> 24)", shift)
	}
}

// reinterpretedVarName returns the variable name for a reinterpreted packed char.
// Format: "reinterpreted_{packedType}_e{handle}"
func (w *Writer) reinterpretedVarName(packedType string, handle ir.ExpressionHandle) string {
	return fmt.Sprintf("reinterpreted_%s_e%d", packedType, handle)
}

// writeAs writes a type cast/conversion.
func (w *Writer) writeAs(as ir.ExprAs) error {
	// Try to fold constant casts at output time.
	// When pipeline constants are applied, expressions like As(Constant(2_i32), uint)
	// should produce literal 2u instead of static_cast<uint>(o).
	// Matches Rust naga's constant_evaluator behavior in process_overrides.
	if as.Convert != nil {
		if folded, ok := w.tryFoldConstantCast(as); ok {
			return w.writeScalarValue(folded.scalar, folded.typeHandle)
		}
	}

	// Determine target scalar type name.
	// For bitcast (Convert==nil), use source width to match type sizes.
	convert := as.Convert
	if convert == nil {
		// Bitcast: target must have same byte width as source.
		// Infer width from source expression type.
		if srcScalar := w.getExpressionScalarType(as.Expr); srcScalar != nil {
			w := srcScalar.Width
			convert = &w
		}
	}
	scalarName := w.scalarCastTypeName(as.Kind, convert)

	// Resolve source expression type to determine if it's a vector/matrix cast.
	// Rust naga uses static_cast<metal::float3>(x) for vector conversions,
	// static_cast<float>(x) for scalar conversions, as_type<T>(x) for bitcasts,
	// metal::half2x2(x) for matrix conversions (constructor-style, NOT static_cast).
	typeName := scalarName
	isMatrixCast := false
	var matRows, matCols ir.VectorSize
	if srcType := w.getExpressionType(as.Expr); srcType != nil {
		if vec, ok := srcType.(ir.VectorType); ok {
			typeName = fmt.Sprintf("%s%s%d", Namespace, scalarName, vec.Size)
		} else if mat, ok := srcType.(ir.MatrixType); ok {
			// Matrix conversion uses constructor syntax: metal::half2x2(x)
			isMatrixCast = true
			matRows = mat.Rows
			matCols = mat.Columns
			typeName = fmt.Sprintf("%s%s%dx%d", Namespace, scalarName, matCols, matRows)
		}
	}

	if as.Convert != nil {
		// Check if this is a float-to-int conversion that needs a clamped helper.
		// Rust naga uses naga_f2i32/naga_f2u32 etc. instead of static_cast for these
		// to avoid undefined behavior when the float is out of range.
		// Emits separate overloads for each (srcScalar, vectorSize, dstScalar) combo.
		srcScalar := w.getExpressionScalarType(as.Expr)
		isFloatToInt := srcScalar != nil && srcScalar.Kind == ir.ScalarFloat &&
			(as.Kind == ir.ScalarSint || as.Kind == ir.ScalarUint)
		// Determine vector size of source expression
		var srcVecSize ir.VectorSize
		if srcType := w.getExpressionType(as.Expr); srcType != nil {
			if vec, ok := srcType.(ir.VectorType); ok {
				srcVecSize = vec.Size
			}
		}
		if isMatrixCast {
			// Matrix conversions use constructor syntax: metal::half2x2(x)
			// Matches Rust naga: put_numeric_type(target_scalar, &[rows, columns])
			w.write("%s(", typeName)
			if err := w.writeExpression(as.Expr); err != nil {
				return err
			}
			w.write(")")
		} else if isFloatToInt {
			dstScalar := ir.ScalarType{Kind: as.Kind, Width: *as.Convert}
			w.registerF2IHelper(*srcScalar, srcVecSize, dstScalar)
			w.write("%s(", f2iFunctionName(dstScalar))
			if err := w.writeExpression(as.Expr); err != nil {
				return err
			}
			w.write(")")
		} else {
			// Regular type conversion
			w.write("static_cast<%s>(", typeName)
			if err := w.writeExpression(as.Expr); err != nil {
				return err
			}
			w.write(")")
		}
	} else {
		// Bitcast (Rust naga uses as_type<T>(x))
		w.write("as_type<%s>(", typeName)
		if err := w.writeExpression(as.Expr); err != nil {
			return err
		}
		w.write(")")
	}
	return nil
}

// foldedScalar is the result of folding a constant cast at output time.
type foldedScalar struct {
	scalar     ir.ScalarValue
	typeHandle ir.TypeHandle
}

// tryFoldConstantCast attempts to fold As(Constant(value), targetKind) into a literal.
// Returns the folded scalar value and true if successful.
func (w *Writer) tryFoldConstantCast(as ir.ExprAs) (foldedScalar, bool) {
	if w.currentFunction == nil || as.Convert == nil {
		return foldedScalar{}, false
	}
	// Check if the inner expression is a Constant reference.
	if int(as.Expr) >= len(w.currentFunction.Expressions) {
		return foldedScalar{}, false
	}
	inner := &w.currentFunction.Expressions[as.Expr]
	constRef, ok := inner.Kind.(ir.ExprConstant)
	if !ok {
		return foldedScalar{}, false
	}
	if int(constRef.Constant) >= len(w.module.Constants) {
		return foldedScalar{}, false
	}
	c := &w.module.Constants[constRef.Constant]
	sv, ok := c.Value.(ir.ScalarValue)
	if !ok {
		return foldedScalar{}, false
	}

	// Convert the scalar value to the target type.
	targetWidth := *as.Convert
	switch as.Kind {
	case ir.ScalarUint:
		var val uint32
		switch sv.Kind {
		case ir.ScalarSint:
			val = uint32(int32(sv.Bits))
		case ir.ScalarUint:
			val = uint32(sv.Bits)
		case ir.ScalarFloat:
			val = uint32(math.Float32frombits(uint32(sv.Bits)))
		default:
			return foldedScalar{}, false
		}
		typeHandle := w.findOrRegisterScalarType(ir.ScalarUint, targetWidth)
		return foldedScalar{
			scalar:     ir.ScalarValue{Bits: uint64(val), Kind: ir.ScalarUint},
			typeHandle: typeHandle,
		}, true
	case ir.ScalarSint:
		var val int32
		switch sv.Kind {
		case ir.ScalarSint:
			val = int32(sv.Bits)
		case ir.ScalarUint:
			val = int32(uint32(sv.Bits))
		case ir.ScalarFloat:
			val = int32(math.Float32frombits(uint32(sv.Bits)))
		default:
			return foldedScalar{}, false
		}
		typeHandle := w.findOrRegisterScalarType(ir.ScalarSint, targetWidth)
		return foldedScalar{
			scalar:     ir.ScalarValue{Bits: uint64(val), Kind: ir.ScalarSint},
			typeHandle: typeHandle,
		}, true
	}
	return foldedScalar{}, false
}

// findOrRegisterScalarType finds a type handle for a scalar type with given kind and width.
func (w *Writer) findOrRegisterScalarType(kind ir.ScalarKind, width uint8) ir.TypeHandle {
	for i, t := range w.module.Types {
		if st, ok := t.Inner.(ir.ScalarType); ok {
			if st.Kind == kind && st.Width == width {
				return ir.TypeHandle(i)
			}
		}
	}
	// Fallback: use type 0 (should not happen in practice)
	return ir.TypeHandle(0)
}

// writeImageSample writes a texture sample operation.
func (w *Writer) writeImageSample(sample ir.ExprImageSample) error {
	// ClampToEdge: call helper function instead of direct .sample()
	// Matches Rust naga's nagaTextureSampleBaseClampToEdge wrapper.
	if sample.ClampToEdge {
		w.needsTextureSampleBaseClampToEdge = true
		w.write("nagaTextureSampleBaseClampToEdge(")
		if err := w.writeExpression(sample.Image); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(sample.Sampler); err != nil {
			return err
		}
		w.write(", ")
		if err := w.writeExpression(sample.Coordinate); err != nil {
			return err
		}
		w.write(")")
		return nil
	}

	if err := w.writeExpression(sample.Image); err != nil {
		return err
	}

	// Determine sampling method
	switch {
	case sample.DepthRef != nil && sample.Gather != nil:
		// Depth comparison gather (textureGatherCompare)
		w.write(".gather_compare(")
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
		// Matches Rust naga: SampleLevel::Zero is a no-op in MSL.
		// Depth textures use sample_compare which implicitly uses level 0.
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

	// Gather component (default X is omitted)
	if sample.Gather != nil && *sample.Gather != ir.SwizzleX {
		// If no offset and not cube map, need to insert a zero offset
		// before the component (Metal requires it for non-cube gather)
		if sample.Offset == nil && !w.isImageCubeMap(sample.Image) {
			w.write(", %sint2(0)", Namespace)
		}
		components := [4]string{"x", "y", "z", "w"}
		w.write(", %scomponent::%s", Namespace, components[*sample.Gather])
	}

	w.write(")")
	return nil
}

// isImageCubeMap checks if the image expression is a cube map texture.
func (w *Writer) isImageCubeMap(imageHandle ir.ExpressionHandle) bool {
	imgType := w.getImageType(imageHandle)
	if imgType == nil {
		return false
	}
	return imgType.Dim == ir.DimCube
}

// writeImageLoad writes a texture load operation with bounds checking.
// The handle is used for generating clamped_lod variable names (e.g., clamped_lod_e3).
func (w *Writer) writeImageLoad(load ir.ExprImageLoad, handle ir.ExpressionHandle) error {
	// External textures: call nagaTextureLoadExternal helper.
	imgType := w.getImageType(load.Image)
	if imgType != nil && imgType.Class == ir.ImageClassExternal {
		w.needsExternalTextureLoad = true
		w.write("nagaTextureLoadExternal(")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(", ")
		w.writeCastToUintScalarOrVector(load.Coordinate)
		w.write(")")
		return nil
	}

	policy := w.options.BoundsCheckPolicies.Image

	switch policy {
	case BoundsCheckRestrict:
		return w.writeImageLoadRestrict(load, handle, imgType)
	case BoundsCheckReadZeroSkipWrite:
		return w.writeImageLoadRZSW(load, imgType)
	default:
		return w.writeImageLoadUnchecked(load, imgType)
	}
}

// writeImageLoadUnchecked writes an unchecked texture load (no bounds checking).
func (w *Writer) writeImageLoadUnchecked(load ir.ExprImageLoad, imgType *ir.ImageType) error {
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".read(")

	// Coordinate
	if err := w.writeCoordCast(load.Coordinate, imgType); err != nil {
		return err
	}

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

	// Level (only for mipmapped non-1D images; 1D textures can't have mipmaps in MSL)
	if load.Level != nil && w.imageNeedsLod(load.Image) {
		w.write(", ")
		if err := w.writeExpression(*load.Level); err != nil {
			return err
		}
	}

	w.write(")")
	return nil
}

// imageNeedsLod returns true if the image supports level-of-detail arguments.
// 1D textures cannot have mipmaps in MSL, so LOD is always omitted for them.
// Matches Rust naga's image_needs_lod.
func (w *Writer) imageNeedsLod(imageHandle ir.ExpressionHandle) bool {
	imgType := w.resolveImageType(imageHandle)
	if imgType == nil {
		return true // default to needing LOD if we can't resolve
	}
	if imgType.Dim == ir.Dim1D {
		return false
	}
	// Only mipmapped images need LOD
	return imgType.Class == ir.ImageClassSampled || imgType.Class == ir.ImageClassDepth
}

// writeImageLoadRestrict writes a texture load with Restrict bounds checking.
// Coordinates and levels are clamped to valid ranges using metal::min.
// For images with mip levels, a clamped_lod_eN local variable is emitted before
// the expression (see writeImageLoadClampedLodIfNeeded).
func (w *Writer) writeImageLoadRestrict(load ir.ExprImageLoad, handle ir.ExpressionHandle, imgType *ir.ImageType) error {
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".read(")

	dim := ir.Dim2D
	if imgType != nil {
		dim = imgType.Dim
	}
	isMultisampled := imgType != nil && imgType.Multisampled

	if isMultisampled {
		// Multisampled: clamp coords, no level
		if err := w.writeRestrictCoords(load, imgType); err != nil {
			return err
		}

		// Sample (clamped)
		if load.Sample != nil {
			w.write(", %smin(uint(", Namespace)
			if err := w.writeExpression(*load.Sample); err != nil {
				return err
			}
			w.write("), ")
			if err := w.writeExpression(load.Image); err != nil {
				return err
			}
			w.write(".get_num_samples() - 1)")
		}
	} else if dim == ir.Dim1D {
		// 1D: clamp coord to width, no level
		w.write("%smin(uint(", Namespace)
		if err := w.writeExpression(load.Coordinate); err != nil {
			return err
		}
		w.write("), ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_width() - 1)")
	} else {
		// 2D/3D/Cube with level: use clamped_lod
		clampedLod := fmt.Sprintf("clamped_lod_e%d", handle)
		if err := w.writeRestrictCoordsWithLod(load, imgType, clampedLod); err != nil {
			return err
		}

		// Array index (clamped)
		if load.ArrayIndex != nil {
			w.write(", %smin(uint(", Namespace)
			if err := w.writeExpression(*load.ArrayIndex); err != nil {
				return err
			}
			w.write("), ")
			if err := w.writeExpression(load.Image); err != nil {
				return err
			}
			w.write(".get_array_size() - 1)")
		}

		// Clamped level
		w.write(", %s", clampedLod)
	}

	w.write(")")
	return nil
}

// writeRestrictCoords writes clamped coordinates for multisampled images.
func (w *Writer) writeRestrictCoords(load ir.ExprImageLoad, imgType *ir.ImageType) error {
	dim := ir.Dim2D
	if imgType != nil {
		dim = imgType.Dim
	}
	coordType := coordUintType(dim)
	w.write("%smin(%s(", Namespace, coordType)
	if err := w.writeExpression(load.Coordinate); err != nil {
		return err
	}
	w.write("), %s(", coordType)
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".get_width()")
	if dim != ir.Dim1D {
		w.write(", ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_height()")
	}
	if dim == ir.Dim3D {
		w.write(", ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_depth()")
	}
	w.write(") - 1)")
	return nil
}

// writeRestrictCoordsWithLod writes clamped coordinates using a clamped LOD variable.
func (w *Writer) writeRestrictCoordsWithLod(load ir.ExprImageLoad, imgType *ir.ImageType, clampedLod string) error {
	dim := ir.Dim2D
	if imgType != nil {
		dim = imgType.Dim
	}
	coordType := coordUintType(dim)
	w.write("%smin(%s(", Namespace, coordType)
	if err := w.writeExpression(load.Coordinate); err != nil {
		return err
	}
	w.write("), %s(", coordType)
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".get_width(%s)", clampedLod)
	if dim != ir.Dim1D {
		w.write(", ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_height(%s)", clampedLod)
	}
	if dim == ir.Dim3D {
		w.write(", ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_depth(%s)", clampedLod)
	}
	w.write(") - 1)")
	return nil
}

// writeImageLoadRZSW writes a texture load with ReadZeroSkipWrite bounds checking.
// Uses a ternary expression: (checks ? image.read(args) : DefaultConstructible())
func (w *Writer) writeImageLoadRZSW(load ir.ExprImageLoad, imgType *ir.ImageType) error {
	dim := ir.Dim2D
	if imgType != nil {
		dim = imgType.Dim
	}
	isMultisampled := imgType != nil && imgType.Multisampled
	coordType := coordUintType(dim)

	w.write("(")

	// Bounds checks
	if isMultisampled {
		// Multisampled: check sample < num_samples && coords < size
		if load.Sample != nil {
			w.write("uint(")
			if err := w.writeExpression(*load.Sample); err != nil {
				return err
			}
			w.write(") < ")
			if err := w.writeExpression(load.Image); err != nil {
				return err
			}
			w.write(".get_num_samples() && ")
		}
		if err := w.writeRZSWCoordCheck(load, imgType, coordType, nil); err != nil {
			return err
		}
	} else if dim == ir.Dim1D {
		// 1D: check level < num_mip_levels && coord < width
		if load.Level != nil {
			w.write("uint(")
			if err := w.writeExpression(*load.Level); err != nil {
				return err
			}
			w.write(") < ")
			if err := w.writeExpression(load.Image); err != nil {
				return err
			}
			w.write(".get_num_mip_levels() && ")
		}
		w.write("uint(")
		if err := w.writeExpression(load.Coordinate); err != nil {
			return err
		}
		w.write(") < ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_width()")
	} else {
		// 2D/3D: check level < num_mip_levels && [array_index < array_size &&] all(coords < size)
		if load.Level != nil {
			w.write("uint(")
			if err := w.writeExpression(*load.Level); err != nil {
				return err
			}
			w.write(") < ")
			if err := w.writeExpression(load.Image); err != nil {
				return err
			}
			w.write(".get_num_mip_levels() && ")
		}
		// Array index check
		if load.ArrayIndex != nil {
			w.write("uint(")
			if err := w.writeExpression(*load.ArrayIndex); err != nil {
				return err
			}
			w.write(") < ")
			if err := w.writeExpression(load.Image); err != nil {
				return err
			}
			w.write(".get_array_size() && ")
		}
		// Coordinate check: metal::all(coords < size)
		if err := w.writeRZSWCoordCheck(load, imgType, coordType, load.Level); err != nil {
			return err
		}
	}

	w.write(" ? ")

	// Actual read
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".read(")

	// Coordinate cast (no clamping for RZSW -- the ternary already checked bounds)
	if err := w.writeCoordCast(load.Coordinate, imgType); err != nil {
		return err
	}

	// Array index (unclamped)
	if load.ArrayIndex != nil {
		w.write(", ")
		if err := w.writeExpression(*load.ArrayIndex); err != nil {
			return err
		}
	}

	// Sample (unclamped for RZSW)
	if load.Sample != nil {
		w.write(", ")
		if err := w.writeExpression(*load.Sample); err != nil {
			return err
		}
	}

	// Level (only for non-1D, non-multisampled)
	if load.Level != nil && !isMultisampled && dim != ir.Dim1D {
		w.write(", ")
		if err := w.writeExpression(*load.Level); err != nil {
			return err
		}
	}

	w.write(")")

	w.write(": DefaultConstructible())")
	return nil
}

// writeRZSWCoordCheck writes the coordinate bounds check for RZSW policy.
// For 2D/3D: metal::all(uint2/3(coord) < uint2/3(w(level), h(level) [, d(level)]))
// The level parameter is passed inline to get_width/get_height/get_depth.
func (w *Writer) writeRZSWCoordCheck(load ir.ExprImageLoad, imgType *ir.ImageType, coordType string, level *ir.ExpressionHandle) error {
	dim := ir.Dim2D
	if imgType != nil {
		dim = imgType.Dim
	}

	// 2D/3D: metal::all(coordType(coords) < coordType(w, h [, d]))
	w.write("%sall(%s(", Namespace, coordType)
	if err := w.writeExpression(load.Coordinate); err != nil {
		return err
	}
	w.write(") < %s(", coordType)
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".get_width(")
	if level != nil {
		if err := w.writeExpression(*level); err != nil {
			return err
		}
	}
	w.write(")")
	w.write(", ")
	if err := w.writeExpression(load.Image); err != nil {
		return err
	}
	w.write(".get_height(")
	if level != nil {
		if err := w.writeExpression(*level); err != nil {
			return err
		}
	}
	w.write(")")
	if dim == ir.Dim3D {
		w.write(", ")
		if err := w.writeExpression(load.Image); err != nil {
			return err
		}
		w.write(".get_depth(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write(")")
	}
	w.write("))")
	return nil
}

// writeCoordCast writes the coordinate cast for an image read/write operation.
// Uses the correct uint type based on image dimension: uint for 1D, metal::uint2 for 2D, etc.
func (w *Writer) writeCoordCast(coord ir.ExpressionHandle, imgType *ir.ImageType) error {
	coordType := coordUintType(ir.Dim2D)
	if imgType != nil {
		coordType = coordUintType(imgType.Dim)
	}
	w.write("%s(", coordType)
	if err := w.writeExpression(coord); err != nil {
		return err
	}
	w.write(")")
	return nil
}

// writeCastToUintScalarOrVector writes a cast to uint or uint{N} for image coordinates.
// Matches Rust naga's put_cast_to_uint_scalar_or_vector.
func (w *Writer) writeCastToUintScalarOrVector(coord ir.ExpressionHandle) {
	exprType := w.getExpressionType(coord)
	switch t := exprType.(type) {
	case ir.ScalarType:
		w.write("uint(")
	case ir.VectorType:
		w.write("%suint%d(", Namespace, t.Size)
	default:
		w.write("%suint2(", Namespace)
	}
	_ = w.writeExpression(coord)
	w.write(")")
}

// getImageType returns the ImageType for an expression handle, or nil if not available.
func (w *Writer) getImageType(handle ir.ExpressionHandle) *ir.ImageType {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return nil
	}
	if img, ok := typeInner.(ir.ImageType); ok {
		return &img
	}
	return nil
}

// coordUintType returns the MSL unsigned integer type for image coordinates
// based on the image dimension.
func coordUintType(dim ir.ImageDimension) string {
	switch dim {
	case ir.Dim1D:
		return mslUint
	case ir.Dim2D, ir.DimCube:
		return Namespace + "uint2"
	case ir.Dim3D:
		return Namespace + "uint3"
	default:
		return Namespace + "uint2"
	}
}

// writeImageQuery writes an image query operation.
func (w *Writer) writeImageQuery(query ir.ExprImageQuery) error {
	switch q := query.Query.(type) {
	case ir.ImageQuerySize:
		return w.writeImageQuerySize(query.Image, q.Level)

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

// writeImageQuerySize writes the image size query, composing get_width/get_height/get_depth
// into a vector constructor matching the image dimension. Matches Rust naga's put_image_size_query.
func (w *Writer) writeImageQuerySize(image ir.ExpressionHandle, level *ir.ExpressionHandle) error {
	// External textures: call nagaTextureDimensionsExternal helper.
	if imgType := w.resolveImageType(image); imgType != nil && imgType.Class == ir.ImageClassExternal {
		w.needsExternalTextureDimensions = true
		w.write("nagaTextureDimensionsExternal(")
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(")")
		return nil
	}

	// Resolve the image type to get its dimension
	dim := ir.Dim2D // default to 2D
	if imgType := w.resolveImageType(image); imgType != nil {
		dim = imgType.Dim
	}

	switch dim {
	case ir.Dim1D:
		// 1D: just get_width(), no vector construction needed.
		// 1D textures never have mipmaps in MSL, omit level.
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_width()")

	case ir.Dim2D:
		// 2D: metal::uint2(get_width(level), get_height(level))
		w.write("%suint2(", Namespace)
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_width(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write("), ")
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_height(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write("))")

	case ir.Dim3D:
		// 3D: metal::uint3(get_width(level), get_height(level), get_depth(level))
		w.write("%suint3(", Namespace)
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_width(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write("), ")
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_height(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write("), ")
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_depth(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write("))")

	case ir.DimCube:
		// Cube: metal::uint2(get_width(level)) — only width matters, height equals width
		w.write("%suint2(", Namespace)
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_width(")
		if level != nil {
			if err := w.writeExpression(*level); err != nil {
				return err
			}
		}
		w.write("))")

	default:
		// Fallback
		if err := w.writeExpression(image); err != nil {
			return err
		}
		w.write(".get_width()")
	}
	return nil
}

// resolveImageType resolves the type of an image expression and returns the ImageType,
// or nil if the type cannot be resolved.
func (w *Writer) resolveImageType(image ir.ExpressionHandle) *ir.ImageType {
	exprType, err := ir.ResolveExpressionType(w.module, w.currentFunction, image)
	if err != nil {
		return nil
	}
	inner := ir.TypeResInner(w.module, exprType)
	if img, ok := inner.(ir.ImageType); ok {
		return &img
	}
	return nil
}

// writeDerivative writes a derivative operation.
// Rust naga ignores the DerivativeControl (coarse/fine/none) and always emits
// the base function name. MSL dfdx/dfdy/fwidth have implementation-defined
// precision, and _coarse/_fine suffixes are just hints that Rust drops.
func (w *Writer) writeDerivative(deriv ir.ExprDerivative) error {
	var funcName string
	switch deriv.Axis {
	case ir.DerivativeX:
		funcName = "dfdx"
	case ir.DerivativeY:
		funcName = "dfdy"
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

// writeArrayLength writes a runtime array length query.
// Emits "1 + (_buffer_sizes.sizeN - offset - elementSize) / stride".
// The Array expression refers to a GlobalVariable or AccessIndex into a struct
// containing a runtime-sized array.
func (w *Writer) writeArrayLength(expr ir.ExprArrayLength) error {
	// Find the global variable that owns the runtime array.
	globalHandle, ok := w.resolveArrayLengthGlobal(expr.Array)
	if !ok {
		w.write("/* array length */ 0")
		return nil
	}

	global := &w.module.GlobalVariables[globalHandle]

	// Determine offset and array type.
	var offset uint32
	var arrayTypeHandle ir.TypeHandle
	if int(global.Type) < len(w.module.Types) {
		switch gt := w.module.Types[global.Type].Inner.(type) {
		case ir.StructType:
			if len(gt.Members) > 0 {
				last := gt.Members[len(gt.Members)-1]
				offset = last.Offset
				arrayTypeHandle = last.Type
			}
		case ir.ArrayType:
			offset = 0
			arrayTypeHandle = global.Type
		}
	}

	// Get element size and stride from the array type.
	var elemSize, stride uint32
	if int(arrayTypeHandle) < len(w.module.Types) {
		if at, ok := w.module.Types[arrayTypeHandle].Inner.(ir.ArrayType); ok {
			stride = at.Stride
			// Compute element size from the base type using the stride-based heuristic.
			// For most types, element size equals stride (no trailing padding).
			// The precise calculation matches Rust's TypeInner::size().
			elemSize = w.typeSize(at.Base)
		}
	}

	if stride == 0 {
		w.write("/* array length */ 0")
		return nil
	}

	// Emit: 1 + (_buffer_sizes.sizeN - offset - elementSize) / stride
	w.write("1 + (_buffer_sizes.size%d - %d - %d) / %d",
		globalHandle, offset, elemSize, stride)
	return nil
}

// resolveArrayLengthGlobal traces an ExprArrayLength's array expression back
// to the global variable handle that owns the runtime-sized array.
func (w *Writer) resolveArrayLengthGlobal(handle ir.ExpressionHandle) (ir.GlobalVariableHandle, bool) {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return 0, false
	}
	expr := w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		return k.Variable, true
	case ir.ExprAccessIndex:
		// Struct field access: trace back to the base global.
		return w.resolveArrayLengthGlobal(k.Base)
	default:
		return 0, false
	}
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

// firstLeadingBitResultType returns the MSL type name for the result of
// firstLeadingBit (without metal:: prefix), e.g. "int", "uint", "int3", "uint3".
// Used for the T(-1) default value in the metal::select pattern.
func (w *Writer) firstLeadingBitResultType(handle ir.ExpressionHandle) string {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return "int"
	}
	switch t := typeInner.(type) {
	case ir.ScalarType:
		return scalarTypeName(t)
	case ir.VectorType:
		return fmt.Sprintf("%s%d", scalarTypeName(t.Scalar), t.Size)
	}
	return "int"
}

// getIntegerOverload returns a divModOverload for the expression type if it is
// an integer scalar or vector. Returns false if not an integer type.
func (w *Writer) getIntegerOverload(handle ir.ExpressionHandle) (divModOverload, bool) {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return divModOverload{}, false
	}
	switch t := typeInner.(type) {
	case ir.ScalarType:
		if t.Kind == ir.ScalarSint || t.Kind == ir.ScalarUint {
			return divModOverload{kind: t.Kind, width: t.Width, vectorSize: 0}, true
		}
	case ir.VectorType:
		if t.Scalar.Kind == ir.ScalarSint || t.Scalar.Kind == ir.ScalarUint {
			return divModOverload{kind: t.Scalar.Kind, width: t.Scalar.Width, vectorSize: t.Size}, true
		}
	}
	return divModOverload{}, false
}

// getSignedWrappingTypes returns the MSL signed and unsigned type names for
// wrapping arithmetic. Returns ("int", "uint", true) for i32 scalar,
// ("metal::int2", "metal::uint2", true) for vec2<i32>, etc.
// Returns ("", "", false) if the expression is not a signed integer type.
func (w *Writer) getSignedWrappingTypes(handle ir.ExpressionHandle) (string, string, bool) {
	typeInner := w.getExpressionType(handle)
	if typeInner == nil {
		return "", "", false
	}
	switch t := typeInner.(type) {
	case ir.ScalarType:
		if t.Kind == ir.ScalarSint {
			signed := scalarTypeName(t)
			unsigned := scalarTypeName(ir.ScalarType{Kind: ir.ScalarUint, Width: t.Width})
			return signed, unsigned, true
		}
	case ir.VectorType:
		if t.Scalar.Kind == ir.ScalarSint {
			signed := fmt.Sprintf("%s%s%d", Namespace, scalarTypeName(t.Scalar), t.Size)
			unsigned := fmt.Sprintf("%s%s%d", Namespace, scalarTypeName(ir.ScalarType{Kind: ir.ScalarUint, Width: t.Scalar.Width}), t.Size)
			return signed, unsigned, true
		}
	}
	return "", "", false
}

func (w *Writer) pointerNeedsDeref(pt ir.PointerType) bool {
	// Buffer parameters use references (&) in MSL, not pointers (*).
	// References don't need explicit dereference.
	return false
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

// isPackedVec3Access checks if the given expression handle is an access to a struct member
// that is a packed vec3 (packed_float3, packed_int3, etc.). MSL packed vec3 types don't
// support swizzle syntax (.x, .y, .z), so bracket notation ([0], [1], [2]) must be used.
func (w *Writer) isPackedVec3Access(handle ir.ExpressionHandle) bool {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return false
	}
	expr := &w.currentFunction.Expressions[handle]

	// Check if this is an AccessIndex into a struct member
	access, ok := expr.Kind.(ir.ExprAccessIndex)
	if !ok {
		return false
	}

	// Get the struct type of the base
	baseType := w.getExpressionType(access.Base)
	if baseType == nil {
		return false
	}

	// Unwrap pointer type if needed
	var structTypeHandle ir.TypeHandle
	switch bt := baseType.(type) {
	case ir.PointerType:
		structTypeHandle = bt.Base
	case ir.StructType:
		th := w.getExpressionTypeHandle(access.Base)
		if th == nil {
			return false
		}
		structTypeHandle = *th
	default:
		return false
	}

	if int(structTypeHandle) >= len(w.module.Types) {
		return false
	}
	st, ok := w.module.Types[structTypeHandle].Inner.(ir.StructType)
	if !ok {
		return false
	}

	return w.shouldPackMember(st, int(access.Index)) != nil
}

// getPointerAddressSpace walks the access chain to find the originating address space.
// Returns the address space and true if found, or (0, false) otherwise.
func (w *Writer) getPointerAddressSpace(handle ir.ExpressionHandle) (ir.AddressSpace, bool) {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return 0, false
	}
	expr := &w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(k.Variable) < len(w.module.GlobalVariables) {
			return w.module.GlobalVariables[k.Variable].Space, true
		}
	case ir.ExprLocalVariable:
		return ir.SpaceFunction, true
	case ir.ExprFunctionArgument:
		// Function arguments that are pointers carry their address space in the type.
		ty := w.getExpressionType(handle)
		if pt, ok := ty.(ir.PointerType); ok {
			return pt.Space, true
		}
		// Non-pointer arguments are values, use index policy.
		return ir.SpaceFunction, true
	case ir.ExprAccess:
		return w.getPointerAddressSpace(k.Base)
	case ir.ExprAccessIndex:
		return w.getPointerAddressSpace(k.Base)
	}
	return 0, false
}

// chooseBoundsCheckPolicy selects the appropriate bounds check policy for a pointer expression.
// Matches Rust naga's BoundsCheckPolicies::choose_policy.
func (w *Writer) chooseBoundsCheckPolicy(handle ir.ExpressionHandle) BoundsCheckPolicy {
	ty := w.getExpressionType(handle)
	if _, ok := ty.(ir.BindingArrayType); ok {
		return w.options.BoundsCheckPolicies.BindingArray
	}
	space, ok := w.getPointerAddressSpace(handle)
	if ok {
		switch space {
		case ir.SpaceStorage, ir.SpaceUniform:
			return w.options.BoundsCheckPolicies.Buffer
		}
	}
	return w.options.BoundsCheckPolicies.Index
}

// indexableLength returns the known length of an indexable base type.
// Returns (length, true) for fixed-size arrays and vectors, or (0, false) for unknown/dynamic.
func (w *Writer) indexableLength(baseHandle ir.ExpressionHandle) (uint32, bool) {
	baseType := w.getExpressionType(baseHandle)
	if baseType == nil {
		return 0, false
	}
	// Unwrap pointer types.
	if pt, ok := baseType.(ir.PointerType); ok {
		if int(pt.Base) < len(w.module.Types) {
			baseType = w.module.Types[pt.Base].Inner
		}
	}
	if vp, ok := baseType.(ir.ValuePointerType); ok {
		if vp.Size != nil {
			return uint32(*vp.Size), true
		}
		return 0, false
	}
	switch t := baseType.(type) {
	case ir.ArrayType:
		if t.Size.Constant != nil {
			return *t.Size.Constant, true
		}
	case ir.VectorType:
		return uint32(t.Size), true
	case ir.MatrixType:
		return uint32(t.Columns), true
	}
	return 0, false
}

// indexableLengthOrDynamic returns the known length for an indexable base type.
// Returns (length, false) for fixed-size arrays/vectors/matrices.
// Returns (0, true) for dynamic (runtime-sized) arrays.
// Returns (0, false) for unknown types.
func (w *Writer) indexableLengthOrDynamic(baseHandle ir.ExpressionHandle) (uint32, bool) {
	baseType := w.getExpressionType(baseHandle)
	if baseType == nil {
		return 0, false
	}
	// Unwrap pointer types.
	if pt, ok := baseType.(ir.PointerType); ok {
		if int(pt.Base) < len(w.module.Types) {
			baseType = w.module.Types[pt.Base].Inner
		}
	}
	if vp, ok := baseType.(ir.ValuePointerType); ok {
		if vp.Size != nil {
			return uint32(*vp.Size), false
		}
		return 0, false
	}
	switch t := baseType.(type) {
	case ir.ArrayType:
		if t.Size.Constant != nil {
			return *t.Size.Constant, false
		}
		return 0, true // dynamic array
	case ir.VectorType:
		return uint32(t.Size), false
	case ir.MatrixType:
		return uint32(t.Columns), false
	}
	return 0, false
}

// runtimeArrayInfo holds info needed to emit a runtime-sized array bound for Restrict policy.
type runtimeArrayInfo struct {
	globalIdx    uint32 // Global variable index (for _buffer_sizes.sizeN)
	memberOffset uint32 // Byte offset of the runtime-sized array member in the struct
	elemStride   uint32 // Element stride (bytes per element)
}

// resolveRuntimeArrayInfo checks if a base expression is an access into a runtime-sized array
// member of a struct in a global variable. Returns info needed for restrict bound computation.
func (w *Writer) resolveRuntimeArrayInfo(baseHandle ir.ExpressionHandle) (runtimeArrayInfo, bool) {
	if w.currentFunction == nil || int(baseHandle) >= len(w.currentFunction.Expressions) {
		return runtimeArrayInfo{}, false
	}
	expr := &w.currentFunction.Expressions[baseHandle]
	ai, ok := expr.Kind.(ir.ExprAccessIndex)
	if !ok {
		return runtimeArrayInfo{}, false
	}
	// The base should be a GlobalVariable (or chain to one).
	globalIdx, globalType, found := w.resolveGlobalVariable(ai.Base)
	if !found {
		return runtimeArrayInfo{}, false
	}
	// Get the struct type.
	if int(globalType) >= len(w.module.Types) {
		return runtimeArrayInfo{}, false
	}
	st, ok := w.module.Types[globalType].Inner.(ir.StructType)
	if !ok {
		return runtimeArrayInfo{}, false
	}
	if int(ai.Index) >= len(st.Members) {
		return runtimeArrayInfo{}, false
	}
	member := &st.Members[ai.Index]
	if int(member.Type) >= len(w.module.Types) {
		return runtimeArrayInfo{}, false
	}
	arr, ok := w.module.Types[member.Type].Inner.(ir.ArrayType)
	if !ok || arr.Size.Constant != nil {
		return runtimeArrayInfo{}, false // Not a runtime-sized array
	}
	return runtimeArrayInfo{
		globalIdx:    globalIdx,
		memberOffset: member.Offset,
		elemStride:   arr.Stride,
	}, true
}

// resolveGlobalVariable traces an expression chain to find the underlying GlobalVariable.
// Returns (globalVarIndex, globalVarTypeHandle, true) if found.
func (w *Writer) resolveGlobalVariable(handle ir.ExpressionHandle) (uint32, ir.TypeHandle, bool) {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return 0, 0, false
	}
	expr := &w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(k.Variable) < len(w.module.GlobalVariables) {
			return uint32(k.Variable), w.module.GlobalVariables[k.Variable].Type, true
		}
	case ir.ExprAccess:
		return w.resolveGlobalVariable(k.Base)
	case ir.ExprAccessIndex:
		return w.resolveGlobalVariable(k.Base)
	}
	return 0, 0, false
}

// writeRestrictedDynamicIndex writes a restricted index for runtime-sized arrays.
// Emits: metal::min(unsigned(index), (_buffer_sizes.sizeN - offset - stride) / stride)
func (w *Writer) writeRestrictedDynamicIndex(indexHandle ir.ExpressionHandle, info runtimeArrayInfo) error {
	w.write("metal::min(unsigned(")
	if err := w.writeExpression(indexHandle); err != nil {
		return err
	}
	w.write("), (_buffer_sizes.size%d - %d - %d) / %d)",
		info.globalIdx, info.memberOffset, info.elemStride, info.elemStride)
	return nil
}

// indexableLengthDeep resolves the length of an indexable type by traversing the expression tree.
// This is a fallback for when the type resolution system doesn't produce the correct type.
// It walks AccessIndex chains to find the actual struct member type.
func (w *Writer) indexableLengthDeep(baseHandle ir.ExpressionHandle) (uint32, bool) {
	if w.currentFunction == nil || int(baseHandle) >= len(w.currentFunction.Expressions) {
		return 0, false
	}
	expr := &w.currentFunction.Expressions[baseHandle]
	switch k := expr.Kind.(type) {
	case ir.ExprAccessIndex:
		// Get the type of the base, then look up the member at the index.
		baseType := w.resolveBaseType(k.Base)
		if baseType == nil {
			return 0, false
		}
		// Unwrap pointer.
		if pt, ok := baseType.(ir.PointerType); ok {
			if int(pt.Base) < len(w.module.Types) {
				baseType = w.module.Types[pt.Base].Inner
			}
		}
		switch t := baseType.(type) {
		case ir.StructType:
			if int(k.Index) < len(t.Members) {
				memberType := w.module.Types[t.Members[k.Index].Type].Inner
				return w.typeInnerLength(memberType)
			}
		case ir.ArrayType:
			if t.Size.Constant != nil {
				return *t.Size.Constant, true
			}
		case ir.VectorType:
			return uint32(t.Size), true
		case ir.MatrixType:
			return uint32(t.Columns), true
		}
	}
	return 0, false
}

// resolveBaseType resolves an expression to its type by traversing the expression tree.
func (w *Writer) resolveBaseType(handle ir.ExpressionHandle) ir.TypeInner {
	if w.currentFunction == nil || int(handle) >= len(w.currentFunction.Expressions) {
		return nil
	}
	expr := &w.currentFunction.Expressions[handle]
	switch k := expr.Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(k.Variable) < len(w.module.GlobalVariables) {
			gv := &w.module.GlobalVariables[k.Variable]
			return ir.PointerType{Base: gv.Type, Space: gv.Space}
		}
	case ir.ExprLocalVariable:
		// Try type resolution first
		return w.getExpressionType(handle)
	case ir.ExprFunctionArgument:
		return w.getExpressionType(handle)
	case ir.ExprAccessIndex:
		return w.getExpressionType(handle)
	}
	return w.getExpressionType(handle)
}

// typeInnerLength returns the length of an indexable TypeInner.
func (w *Writer) typeInnerLength(inner ir.TypeInner) (uint32, bool) {
	switch t := inner.(type) {
	case ir.ArrayType:
		if t.Size.Constant != nil {
			return *t.Size.Constant, true
		}
	case ir.VectorType:
		return uint32(t.Size), true
	case ir.MatrixType:
		return uint32(t.Columns), true
	}
	return 0, false
}

// accessNeedsRestrict checks if a dynamic access needs Restrict bounds clamping.
// Returns (maxIndex, true) if clamping is needed.
// If the index is a constant known to be in bounds, returns (0, false).
func (w *Writer) accessNeedsRestrict(baseHandle ir.ExpressionHandle, indexHandle ir.ExpressionHandle) (uint32, bool) {
	length, ok := w.indexableLength(baseHandle)
	if !ok || length == 0 {
		return 0, false
	}
	// Check if the index is a known constant that's in bounds.
	if w.currentFunction != nil && int(indexHandle) < len(w.currentFunction.Expressions) {
		idx := &w.currentFunction.Expressions[indexHandle]
		if lit, ok := idx.Kind.(ir.Literal); ok {
			if val, ok := w.literalAsUint(lit); ok && val < length {
				return 0, false // Statically in bounds, no check needed.
			}
		}
	}
	return length - 1, true
}

// accessIndexNeedsRestrict checks if a constant index access needs Restrict bounds clamping.
// Returns (maxIndex, true) if clamping is needed.
func (w *Writer) accessIndexNeedsRestrict(baseHandle ir.ExpressionHandle, index uint32) (uint32, bool) {
	length, ok := w.indexableLength(baseHandle)
	if !ok || length == 0 {
		return 0, false
	}
	if index < length {
		return 0, false // Statically in bounds.
	}
	return length - 1, true
}

// literalAsUint extracts a uint32 value from a literal expression.
func (w *Writer) literalAsUint(lit ir.Literal) (uint32, bool) {
	switch v := lit.Value.(type) {
	case ir.LiteralU32:
		return uint32(v), true
	case ir.LiteralI32:
		if v >= 0 {
			return uint32(v), true
		}
	}
	return 0, false
}

// writeRestrictedIndex writes `metal::min(unsigned(INDEX), MAXu)` for Restrict bounds clamping.
func (w *Writer) writeRestrictedIndex(indexHandle ir.ExpressionHandle, maxIndex uint32) error {
	w.write("metal::min(unsigned(")
	if err := w.writeExpression(indexHandle); err != nil {
		return err
	}
	w.write("), %du)", maxIndex)
	return nil
}

// writeRayQueryGetIntersection writes a RayIntersection struct construction
// from the Metal intersection result. Matches Rust naga MSL backend.
func (w *Writer) writeRayQueryGetIntersection(e ir.ExprRayQueryGetIntersection) error {
	if w.module.SpecialTypes.RayIntersection == nil {
		return fmt.Errorf("RayIntersection type not found in module SpecialTypes")
	}
	typeName := w.getTypeName(*w.module.SpecialTypes.RayIntersection)
	w.write("%s {_map_intersection_type(", typeName)
	if err := w.writeExpression(e.Query); err != nil {
		return err
	}
	w.write(".intersection.type)")
	fields := []string{
		"distance",
		"user_instance_id",
		"instance_id",
		"",
		"geometry_id",
		"primitive_id",
		"triangle_barycentric_coord",
		"triangle_front_facing",
		"",
		"object_to_world_transform",
		"world_to_object_transform",
	}
	for _, field := range fields {
		w.write(", ")
		if field == "" {
			w.write("{}")
		} else {
			if err := w.writeExpression(e.Query); err != nil {
				return err
			}
			w.write(".intersection.%s", field)
		}
	}
	w.write("}")
	return nil
}

// Unused but included for completeness
var _ = math.Float32frombits
