package msl

import (
	"fmt"
	"math"

	"github.com/gogpu/naga/ir"
)

// applyPipelineConstants creates a copy of the module with override values
// substituted according to the provided pipeline constants map.
//
// Pipeline constants are matched to overrides by:
//   - Numeric @id: the key is the decimal string of the @id value (e.g., "0", "1300")
//   - Name: the key is the override's identifier name (e.g., "depth", "width")
//
// The f64 value is converted to the override's declared scalar type following
// the WebIDL conversion rules (same as Rust naga's map_value_to_literal).
//
// This function also updates GlobalExpressions to replace Override init expressions
// with resolved literal values, matching Rust naga's process_overrides pass.
func applyPipelineConstants(module *ir.Module, constants map[string]float64) *ir.Module {
	// Create a shallow copy of the module so we don't mutate the original.
	m := *module
	// Deep copy the overrides slice since we'll modify them.
	// Must also clone Init pointers — copy() only copies the pointer values,
	// not the pointed-to data, so modifying *Init would corrupt the original.
	m.Overrides = make([]ir.Override, len(module.Overrides))
	copy(m.Overrides, module.Overrides)
	for i := range m.Overrides {
		if m.Overrides[i].Init != nil {
			initCopy := *m.Overrides[i].Init
			m.Overrides[i].Init = &initCopy
		}
		if m.Overrides[i].ID != nil {
			idCopy := *m.Overrides[i].ID
			m.Overrides[i].ID = &idCopy
		}
	}
	// Deep copy Functions and EntryPoints — rebuildFunctionExpressions modifies
	// LocalVars.Init, NamedExpressions, Expressions, Body, ExpressionTypes.
	m.Functions = make([]ir.Function, len(module.Functions))
	for i := range module.Functions {
		m.Functions[i] = module.Functions[i]
		m.Functions[i].Expressions = append([]ir.Expression(nil), module.Functions[i].Expressions...)
		m.Functions[i].LocalVars = append([]ir.LocalVariable(nil), module.Functions[i].LocalVars...)
		for j := range m.Functions[i].LocalVars {
			if m.Functions[i].LocalVars[j].Init != nil {
				cp := *m.Functions[i].LocalVars[j].Init
				m.Functions[i].LocalVars[j].Init = &cp
			}
		}
		if module.Functions[i].NamedExpressions != nil {
			m.Functions[i].NamedExpressions = make(map[ir.ExpressionHandle]string)
			for k, v := range module.Functions[i].NamedExpressions {
				m.Functions[i].NamedExpressions[k] = v
			}
		}
		m.Functions[i].Body = append(ir.Block(nil), module.Functions[i].Body...)
	}
	m.EntryPoints = make([]ir.EntryPoint, len(module.EntryPoints))
	for i := range module.EntryPoints {
		m.EntryPoints[i] = module.EntryPoints[i]
		m.EntryPoints[i].Function.Expressions = append([]ir.Expression(nil), module.EntryPoints[i].Function.Expressions...)
		m.EntryPoints[i].Function.LocalVars = append([]ir.LocalVariable(nil), module.EntryPoints[i].Function.LocalVars...)
		for j := range m.EntryPoints[i].Function.LocalVars {
			if m.EntryPoints[i].Function.LocalVars[j].Init != nil {
				cp := *m.EntryPoints[i].Function.LocalVars[j].Init
				m.EntryPoints[i].Function.LocalVars[j].Init = &cp
			}
		}
		if module.EntryPoints[i].Function.NamedExpressions != nil {
			m.EntryPoints[i].Function.NamedExpressions = make(map[ir.ExpressionHandle]string)
			for k, v := range module.EntryPoints[i].Function.NamedExpressions {
				m.EntryPoints[i].Function.NamedExpressions[k] = v
			}
		}
		m.EntryPoints[i].Function.Body = append(ir.Block(nil), module.EntryPoints[i].Function.Body...)
	}

	// Deep copy the constants slice.
	m.Constants = make([]ir.Constant, len(module.Constants))
	copy(m.Constants, module.Constants)
	// Deep copy GlobalExpressions since we'll modify them.
	m.GlobalExpressions = make([]ir.Expression, len(module.GlobalExpressions))
	copy(m.GlobalExpressions, module.GlobalExpressions)

	// Resolve override values: either from pipeline constants or from defaults.
	overrideValues := make(map[ir.OverrideHandle]ir.LiteralValue, len(m.Overrides))

	// Phase 1: Resolve each override's literal value.
	// Process in order since derived overrides may reference earlier ones.
	for oh := ir.OverrideHandle(0); int(oh) < len(m.Overrides); oh++ {
		ov := &m.Overrides[oh]

		// Check if pipeline constants provide a value for this override.
		var value float64
		var found bool

		if ov.ID != nil {
			key := fmt.Sprintf("%d", *ov.ID)
			value, found = constants[key]
		}
		if !found {
			value, found = constants[ov.Name]
		}

		if found {
			// Convert pipeline constant value to the override's type.
			var scalar ir.ScalarType
			if int(ov.Ty) < len(m.Types) {
				if st, ok := m.Types[ov.Ty].Inner.(ir.ScalarType); ok {
					scalar = st
				}
			}
			lit := scalarValueToLiteral(value, scalar)
			if lit != nil {
				overrideValues[oh] = lit
			}
		} else if ov.Init != nil {
			// No pipeline constant: try to evaluate the default init expression.
			lit := evalGlobalExpression(&m, *ov.Init, overrideValues)
			if lit != nil {
				// Convert to the override's declared type if needed.
				var scalar ir.ScalarType
				if int(ov.Ty) < len(m.Types) {
					if st, ok := m.Types[ov.Ty].Inner.(ir.ScalarType); ok {
						scalar = st
					}
				}
				if scalar.Kind != 0 {
					if typed := convertLiteralToType(lit, scalar); typed != nil {
						lit = typed
					}
				}
				overrideValues[oh] = lit
			}
		}
	}

	// Phase 2: Update GlobalExpressions to replace Override init expressions
	// with resolved literal values. Create new expressions for overrides without Init.
	for oh := ir.OverrideHandle(0); int(oh) < len(m.Overrides); oh++ {
		ov := &m.Overrides[oh]
		lit, ok := overrideValues[oh]
		if !ok {
			continue
		}
		if ov.Init != nil {
			// Replace the existing init expression with a Literal expression.
			initHandle := *ov.Init
			if int(initHandle) < len(m.GlobalExpressions) {
				m.GlobalExpressions[initHandle] = ir.Expression{
					Kind: ir.Literal{Value: lit},
				}
			}
		} else {
			// No Init — create a new GlobalExpression with the literal value.
			newHandle := ir.ExpressionHandle(len(m.GlobalExpressions))
			m.GlobalExpressions = append(m.GlobalExpressions, ir.Expression{
				Kind: ir.Literal{Value: lit},
			})
			ov.Init = &newHandle
		}
	}

	// Phase 3: Replace ExprOverride references in GlobalExpressions with the
	// resolved literal values. This handles cases where one global expression
	// references an override (e.g., for derived values like `gain * 10.0`).
	for i := range m.GlobalExpressions {
		replaceOverrideRefs(&m, i, overrideValues)
	}

	// Phase 4: Constant-fold binary expressions in GlobalExpressions that
	// now have all-literal operands (after override substitution).
	// E.g., Binary(Multiply, Literal(1.1), Literal(10.0)) → Literal(11.0)
	// Repeat until no more folding is possible (for chained expressions).
	for changed := true; changed; {
		changed = false
		for i := range m.GlobalExpressions {
			if tryFoldGlobalExpr(&m, i) {
				changed = true
			}
		}
	}

	// EntryPoints and Functions already deep-copied above (lines 39-80).
	// Phase 5: Rebuild function expression arenas, matching Rust naga's
	// process_function behavior. This replaces Override expressions with
	// resolved constant references, evaluates expressions with constant
	// operands (deep-copying constant values into the arena first), and
	// renumbers all expression handles. This produces the same expression
	// indices as Rust naga.
	for epIdx := range m.EntryPoints {
		ep := &m.EntryPoints[epIdx]
		processFunctionOverrides(&m, &ep.Function, overrideValues)
	}
	for fIdx := range m.Functions {
		processFunctionOverrides(&m, &m.Functions[fIdx], overrideValues)
	}

	return &m
}

// replaceOverrideRefs replaces ExprOverride references in a global expression
// with the resolved literal values from overrideValues.
func replaceOverrideRefs(m *ir.Module, exprIdx int, overrideValues map[ir.OverrideHandle]ir.LiteralValue) {
	expr := &m.GlobalExpressions[exprIdx]
	switch k := expr.Kind.(type) {
	case ir.ExprOverride:
		// Replace Override reference with the override's init expression value.
		if lit, ok := overrideValues[k.Override]; ok {
			expr.Kind = ir.Literal{Value: lit}
		}
	default:
		_ = k
	}
}

// tryFoldGlobalExpr attempts to constant-fold a binary expression in GlobalExpressions.
// Returns true if the expression was folded.
func tryFoldGlobalExpr(m *ir.Module, exprIdx int) bool {
	expr := &m.GlobalExpressions[exprIdx]
	bin, ok := expr.Kind.(ir.ExprBinary)
	if !ok {
		return false
	}
	// Check if both operands are now literals.
	if int(bin.Left) >= len(m.GlobalExpressions) || int(bin.Right) >= len(m.GlobalExpressions) {
		return false
	}
	leftLit, lok := m.GlobalExpressions[bin.Left].Kind.(ir.Literal)
	rightLit, rok := m.GlobalExpressions[bin.Right].Kind.(ir.Literal)
	if !lok || !rok {
		return false
	}
	result := evalBinaryOp(bin.Op, leftLit.Value, rightLit.Value)
	if result == nil {
		return false
	}
	expr.Kind = ir.Literal{Value: result}
	return true
}

// evalUnaryOp evaluates a unary operation on a literal value.
func evalUnaryOp(op ir.UnaryOperator, operand ir.LiteralValue) ir.LiteralValue {
	switch op {
	case ir.UnaryLogicalNot:
		if b, ok := operand.(ir.LiteralBool); ok {
			return ir.LiteralBool(!bool(b))
		}
		return nil
	case ir.UnaryNegate:
		switch v := operand.(type) {
		case ir.LiteralF32:
			return ir.LiteralF32(-float32(v))
		case ir.LiteralF64:
			return ir.LiteralF64(-float64(v))
		case ir.LiteralI32:
			return ir.LiteralI32(-int32(v))
		default:
			return nil
		}
	default:
		return nil
	}
}

// evalTypeConversion evaluates a type conversion (ExprAs) on a literal value.
func evalTypeConversion(lit ir.LiteralValue, kind ir.ScalarKind, width uint8) ir.LiteralValue {
	f, ok := literalToFloat64(lit)
	if !ok {
		// Handle bool
		if b, bOk := lit.(ir.LiteralBool); bOk {
			if bool(b) {
				f = 1
			} else {
				f = 0
			}
			ok = true
		}
		if !ok {
			return nil
		}
	}
	scalar := ir.ScalarType{Kind: kind, Width: width}
	return scalarValueToLiteral(f, scalar)
}

// evalGlobalExpression tries to evaluate a global expression to a literal value,
// resolving Override references using the provided overrideValues map.
func evalGlobalExpression(m *ir.Module, handle ir.ExpressionHandle, overrideValues map[ir.OverrideHandle]ir.LiteralValue) ir.LiteralValue {
	if int(handle) >= len(m.GlobalExpressions) {
		return nil
	}
	expr := m.GlobalExpressions[handle]
	switch k := expr.Kind.(type) {
	case ir.Literal:
		return k.Value
	case ir.ExprOverride:
		if lit, ok := overrideValues[k.Override]; ok {
			return lit
		}
		return nil
	case ir.ExprBinary:
		left := evalGlobalExpression(m, k.Left, overrideValues)
		right := evalGlobalExpression(m, k.Right, overrideValues)
		if left == nil || right == nil {
			return nil
		}
		return evalBinaryOp(k.Op, left, right)
	case ir.ExprConstant:
		if int(k.Constant) < len(m.Constants) {
			c := &m.Constants[k.Constant]
			return evalGlobalExpression(m, c.Init, overrideValues)
		}
		return nil
	default:
		return nil
	}
}

// evalBinaryOp evaluates a binary operation on two literal values.
func evalBinaryOp(op ir.BinaryOperator, left, right ir.LiteralValue) ir.LiteralValue {
	// Get f64 values from both operands.
	lf, lok := literalToFloat64(left)
	rf, rok := literalToFloat64(right)
	if !lok || !rok {
		return nil
	}

	var result float64
	switch op {
	case ir.BinaryAdd:
		result = lf + rf
	case ir.BinarySubtract:
		result = lf - rf
	case ir.BinaryMultiply:
		result = lf * rf
	case ir.BinaryDivide:
		if rf == 0 {
			return nil
		}
		result = lf / rf
	default:
		return nil
	}

	// Return the result in the same type as the left operand.
	switch left.(type) {
	case ir.LiteralF32:
		return ir.LiteralF32(float32(result))
	case ir.LiteralF64:
		return ir.LiteralF64(result)
	case ir.LiteralI32:
		return ir.LiteralI32(int32(result))
	case ir.LiteralU32:
		return ir.LiteralU32(uint32(result))
	default:
		return ir.LiteralF32(float32(result))
	}
}

// literalToFloat64 converts a literal value to float64 for arithmetic.
func literalToFloat64(lit ir.LiteralValue) (float64, bool) {
	switch v := lit.(type) {
	case ir.LiteralF32:
		return float64(v), true
	case ir.LiteralF64:
		return float64(v), true
	case ir.LiteralI32:
		return float64(v), true
	case ir.LiteralI64:
		return float64(v), true
	case ir.LiteralU32:
		return float64(v), true
	case ir.LiteralU64:
		return float64(v), true
	default:
		return 0, false
	}
}

// scalarValueToLiteral converts a float64 pipeline constant value to a LiteralValue
// matching the target scalar type. Follows WebIDL conversion rules.
func scalarValueToLiteral(value float64, scalar ir.ScalarType) ir.LiteralValue {
	switch scalar.Kind {
	case ir.ScalarBool:
		// NaN and 0 are false, everything else is true.
		return ir.LiteralBool(value != 0.0 && !math.IsNaN(value))
	case ir.ScalarSint:
		if !math.IsInf(value, 0) && !math.IsNaN(value) {
			v := math.Trunc(value)
			if v >= math.MinInt32 && v <= math.MaxInt32 {
				return ir.LiteralI32(int32(v))
			}
		}
		return nil
	case ir.ScalarUint:
		if !math.IsInf(value, 0) && !math.IsNaN(value) {
			v := math.Trunc(value)
			if v >= 0 && v <= math.MaxUint32 {
				return ir.LiteralU32(uint32(v))
			}
		}
		return nil
	case ir.ScalarFloat:
		if scalar.Width == 8 {
			return ir.LiteralF64(value)
		}
		return ir.LiteralF32(float32(value))
	default:
		return nil
	}
}

// convertLiteralToType converts a literal value to match the target scalar type.
func convertLiteralToType(lit ir.LiteralValue, scalar ir.ScalarType) ir.LiteralValue {
	f, ok := literalToFloat64(lit)
	if !ok {
		return nil
	}
	return scalarValueToLiteral(f, scalar)
}

// literalToScalarValue converts a LiteralValue to a ScalarValue.
func literalToScalarValue(lit ir.LiteralValue) *ir.ScalarValue {
	switch v := lit.(type) {
	case ir.LiteralF32:
		return &ir.ScalarValue{Bits: uint64(math.Float32bits(float32(v))), Kind: ir.ScalarFloat}
	case ir.LiteralF64:
		return &ir.ScalarValue{Bits: math.Float64bits(float64(v)), Kind: ir.ScalarFloat}
	case ir.LiteralI32:
		return &ir.ScalarValue{Bits: uint64(int32(v)), Kind: ir.ScalarSint}
	case ir.LiteralI64:
		return &ir.ScalarValue{Bits: uint64(int64(v)), Kind: ir.ScalarSint}
	case ir.LiteralU32:
		return &ir.ScalarValue{Bits: uint64(uint32(v)), Kind: ir.ScalarUint}
	case ir.LiteralU64:
		return &ir.ScalarValue{Bits: uint64(v), Kind: ir.ScalarUint}
	case ir.LiteralBool:
		if bool(v) {
			return &ir.ScalarValue{Bits: 1, Kind: ir.ScalarBool}
		}
		return &ir.ScalarValue{Bits: 0, Kind: ir.ScalarBool}
	}
	return nil
}

// processFunctionOverrides rebuilds a function's expression arena after override
// resolution, matching Rust naga's process_function behavior. This:
//   - Replaces ExprOverride with ExprOverride (kept for MSL writer to emit constant name)
//   - Deep-copies resolved override literal values into the arena when needed for
//     constant evaluation (matching Rust's check_and_get behavior)
//   - Evaluates Binary/Unary expressions with all-const operands to literals
//   - Renumbers all expression handles to match Rust's sequential arena rebuild
//   - Updates statements, local variable inits, and named expressions
func processFunctionOverrides(m *ir.Module, fn *ir.Function, overrideValues map[ir.OverrideHandle]ir.LiteralValue) {
	if len(fn.Expressions) == 0 {
		return
	}

	// Check if this function references any overrides. If not, skip rebuild.
	hasOverride := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprOverride); ok {
			hasOverride = true
			break
		}
	}
	if !hasOverride {
		return
	}

	oldExprs := fn.Expressions
	newExprs := make([]ir.Expression, 0, len(oldExprs)+8)
	// Map from old expression handle to new expression handle.
	handleMap := make([]ir.ExpressionHandle, len(oldExprs))

	// isConstKind tracks which expressions in the NEW arena are const-evaluable.
	// This includes Literal, ZeroValue, ExprOverride (resolved), and expressions
	// derived from them.
	isConst := make(map[ir.ExpressionHandle]bool)

	appendExpr := func(kind ir.ExpressionKind) ir.ExpressionHandle {
		h := ir.ExpressionHandle(len(newExprs))
		newExprs = append(newExprs, ir.Expression{Kind: kind})
		return h
	}

	// resolveOverrideValue returns the resolved literal for an override, or nil.
	resolveOverrideValue := func(oh ir.OverrideHandle) ir.LiteralValue {
		if lit, ok := overrideValues[oh]; ok {
			return lit
		}
		// Try to get from the override's init global expression.
		if int(oh) < len(m.Overrides) {
			ov := &m.Overrides[oh]
			if ov.Init != nil {
				if int(*ov.Init) < len(m.GlobalExpressions) {
					if lit, ok := m.GlobalExpressions[*ov.Init].Kind.(ir.Literal); ok {
						return lit.Value
					}
				}
			}
		}
		return nil
	}

	// checkAndGetConst resolves an expression handle in the new arena to a literal
	// value for evaluation. If the expression is an Override or Constant, deep-copies
	// the resolved literal into the arena first (matching Rust's check_and_get).
	// Returns the handle to use for evaluation and whether it resolved to a literal.
	checkAndGetConst := func(h ir.ExpressionHandle) (ir.ExpressionHandle, ir.LiteralValue) {
		if int(h) >= len(newExprs) {
			return h, nil
		}
		switch k := newExprs[h].Kind.(type) {
		case ir.Literal:
			return h, k.Value
		case ir.ExprOverride:
			// Deep-copy the override's resolved value into the arena.
			lit := resolveOverrideValue(k.Override)
			if lit != nil {
				newH := appendExpr(ir.Literal{Value: lit})
				isConst[newH] = true
				return newH, lit
			}
			return h, nil
		case ir.ExprConstant:
			// Deep-copy the constant's init value from global expressions.
			if int(k.Constant) < len(m.Constants) {
				c := &m.Constants[k.Constant]
				initH := c.Init
				if int(initH) < len(m.GlobalExpressions) {
					if lit, ok := m.GlobalExpressions[initH].Kind.(ir.Literal); ok {
						newH := appendExpr(ir.Literal{Value: lit.Value})
						isConst[newH] = true
						return newH, lit.Value
					}
				}
			}
			return h, nil
		}
		return h, nil
	}

	// Process each expression, building the new arena.
	for oldH := ir.ExpressionHandle(0); int(oldH) < len(oldExprs); oldH++ {
		expr := oldExprs[oldH]

		switch k := expr.Kind.(type) {
		case ir.ExprOverride:
			// Keep as ExprOverride (MSL writer needs it for constant name).
			// Mark as const since it has a resolved value.
			newH := appendExpr(ir.ExprOverride{Override: k.Override})
			handleMap[oldH] = newH
			if resolveOverrideValue(k.Override) != nil {
				isConst[newH] = true
			}

		case ir.Literal:
			newH := appendExpr(k)
			handleMap[oldH] = newH
			isConst[newH] = true

		case ir.ExprZeroValue:
			newH := appendExpr(k)
			handleMap[oldH] = newH
			isConst[newH] = true

		case ir.ExprConstant:
			newH := appendExpr(k)
			handleMap[oldH] = newH
			isConst[newH] = true

		case ir.ExprBinary:
			// Adjust handles.
			newLeft := handleMap[k.Left]
			newRight := handleMap[k.Right]
			// Check if both operands are const → try to evaluate.
			if isConst[newLeft] && isConst[newRight] {
				leftH, leftLit := checkAndGetConst(newLeft)
				rightH, rightLit := checkAndGetConst(newRight)
				if leftLit != nil && rightLit != nil {
					result := evalBinaryOp(k.Op, leftLit, rightLit)
					if result != nil {
						newH := appendExpr(ir.Literal{Value: result})
						handleMap[oldH] = newH
						isConst[newH] = true
						continue
					}
				}
				// Couldn't evaluate, append adjusted binary with the
				// deep-copied handles.
				newH := appendExpr(ir.ExprBinary{Op: k.Op, Left: leftH, Right: rightH})
				handleMap[oldH] = newH
				isConst[newH] = true
			} else {
				newH := appendExpr(ir.ExprBinary{Op: k.Op, Left: newLeft, Right: newRight})
				handleMap[oldH] = newH
			}

		case ir.ExprUnary:
			newExpr := handleMap[k.Expr]
			if isConst[newExpr] {
				exprH, lit := checkAndGetConst(newExpr)
				if lit != nil {
					result := evalUnaryOp(k.Op, lit)
					if result != nil {
						newH := appendExpr(ir.Literal{Value: result})
						handleMap[oldH] = newH
						isConst[newH] = true
						continue
					}
				}
				// Couldn't evaluate.
				newH := appendExpr(ir.ExprUnary{Op: k.Op, Expr: exprH})
				handleMap[oldH] = newH
				isConst[newH] = true
			} else {
				newH := appendExpr(ir.ExprUnary{Op: k.Op, Expr: newExpr})
				handleMap[oldH] = newH
			}

		case ir.ExprAs:
			newExpr := handleMap[k.Expr]
			if isConst[newExpr] && k.Convert != nil {
				exprH, lit := checkAndGetConst(newExpr)
				if lit != nil {
					result := evalTypeConversion(lit, k.Kind, *k.Convert)
					if result != nil {
						newH := appendExpr(ir.Literal{Value: result})
						handleMap[oldH] = newH
						isConst[newH] = true
						continue
					}
				}
				newH := appendExpr(ir.ExprAs{Expr: exprH, Kind: k.Kind, Convert: k.Convert})
				handleMap[oldH] = newH
				isConst[newH] = true
			} else {
				newH := appendExpr(ir.ExprAs{Expr: newExpr, Kind: k.Kind, Convert: k.Convert})
				handleMap[oldH] = newH
			}

		default:
			// Runtime expression: adjust all expression handle references.
			adjusted := adjustExprHandles(expr.Kind, handleMap)
			newH := appendExpr(adjusted)
			handleMap[oldH] = newH
		}
	}

	// Replace the function's expressions with the rebuilt arena.
	fn.Expressions = newExprs

	// Rebuild ExpressionTypes to match the new expression arena.
	// For old expressions, copy their type resolution to the new position.
	// For new expressions (deep-copies and eval results), resolve from the literal type.
	if len(fn.ExpressionTypes) > 0 {
		oldTypes := fn.ExpressionTypes
		newTypes := make([]ir.TypeResolution, len(newExprs))
		for oldH := ir.ExpressionHandle(0); int(oldH) < len(oldExprs); oldH++ {
			newH := handleMap[oldH]
			if int(oldH) < len(oldTypes) {
				newTypes[newH] = oldTypes[oldH]
			}
		}
		// Fill in types for new expressions that don't have a mapping from old.
		for i := range newExprs {
			if newTypes[i].Handle == nil && newTypes[i].Value == nil {
				// Try to resolve from the expression itself.
				if res, err := ir.ResolveExpressionType(m, fn, ir.ExpressionHandle(i)); err == nil {
					newTypes[i] = res
				}
			}
		}
		fn.ExpressionTypes = newTypes
	}

	// Update local variable init handles.
	for i := range fn.LocalVars {
		if fn.LocalVars[i].Init != nil {
			newH := handleMap[*fn.LocalVars[i].Init]
			fn.LocalVars[i].Init = &newH
		}
	}

	// Update named expressions.
	if len(fn.NamedExpressions) > 0 {
		newNamed := make(map[ir.ExpressionHandle]string, len(fn.NamedExpressions))
		for oldH, name := range fn.NamedExpressions {
			newNamed[handleMap[oldH]] = name
		}
		fn.NamedExpressions = newNamed
	}

	// Update all statement expression handles and fix Emit ranges.
	fn.Body = adjustBlockHandles(fn.Body, handleMap, newExprs)
}

// adjustExprHandles adjusts all expression handle references in an expression kind
// using the provided handle map.
func adjustExprHandles(kind ir.ExpressionKind, handleMap []ir.ExpressionHandle) ir.ExpressionKind {
	adjust := func(h ir.ExpressionHandle) ir.ExpressionHandle {
		if int(h) < len(handleMap) {
			return handleMap[h]
		}
		return h
	}
	adjustPtr := func(h *ir.ExpressionHandle) *ir.ExpressionHandle {
		if h == nil {
			return nil
		}
		v := adjust(*h)
		return &v
	}

	switch k := kind.(type) {
	case ir.ExprLoad:
		return ir.ExprLoad{Pointer: adjust(k.Pointer)}
	case ir.ExprAccess:
		return ir.ExprAccess{Base: adjust(k.Base), Index: adjust(k.Index)}
	case ir.ExprAccessIndex:
		return ir.ExprAccessIndex{Base: adjust(k.Base), Index: k.Index}
	case ir.ExprSplat:
		return ir.ExprSplat{Value: adjust(k.Value), Size: k.Size}
	case ir.ExprSwizzle:
		return ir.ExprSwizzle{Vector: adjust(k.Vector), Size: k.Size, Pattern: k.Pattern}
	case ir.ExprCompose:
		newComponents := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			newComponents[i] = adjust(c)
		}
		return ir.ExprCompose{Type: k.Type, Components: newComponents}
	case ir.ExprBinary:
		return ir.ExprBinary{Op: k.Op, Left: adjust(k.Left), Right: adjust(k.Right)}
	case ir.ExprUnary:
		return ir.ExprUnary{Op: k.Op, Expr: adjust(k.Expr)}
	case ir.ExprSelect:
		return ir.ExprSelect{Condition: adjust(k.Condition), Accept: adjust(k.Accept), Reject: adjust(k.Reject)}
	case ir.ExprDerivative:
		return ir.ExprDerivative{Expr: adjust(k.Expr), Axis: k.Axis, Control: k.Control}
	case ir.ExprRelational:
		return ir.ExprRelational{Fun: k.Fun, Argument: adjust(k.Argument)}
	case ir.ExprMath:
		result := ir.ExprMath{Fun: k.Fun, Arg: adjust(k.Arg)}
		if k.Arg1 != nil {
			v := adjust(*k.Arg1)
			result.Arg1 = &v
		}
		if k.Arg2 != nil {
			v := adjust(*k.Arg2)
			result.Arg2 = &v
		}
		if k.Arg3 != nil {
			v := adjust(*k.Arg3)
			result.Arg3 = &v
		}
		return result
	case ir.ExprAs:
		return ir.ExprAs{Expr: adjust(k.Expr), Kind: k.Kind, Convert: k.Convert}
	case ir.ExprArrayLength:
		return ir.ExprArrayLength{Array: adjust(k.Array)}
	case ir.ExprImageSample:
		return ir.ExprImageSample{
			Image: adjust(k.Image), Sampler: adjust(k.Sampler),
			Gather: k.Gather, Coordinate: adjust(k.Coordinate),
			ArrayIndex: adjustPtr(k.ArrayIndex), Offset: adjustPtr(k.Offset),
			Level:    adjustImageSampleLevel(k.Level, adjust),
			DepthRef: adjustPtr(k.DepthRef), ClampToEdge: k.ClampToEdge,
		}
	case ir.ExprImageLoad:
		return ir.ExprImageLoad{
			Image: adjust(k.Image), Coordinate: adjust(k.Coordinate),
			ArrayIndex: adjustPtr(k.ArrayIndex), Sample: adjustPtr(k.Sample),
			Level: adjustPtr(k.Level),
		}
	case ir.ExprImageQuery:
		return adjustImageQuery(k, adjust)
	case ir.ExprCallResult:
		return k // No expression handles
	case ir.ExprAtomicResult:
		return k
	case ir.ExprFunctionArgument:
		return k
	case ir.ExprGlobalVariable:
		return k
	case ir.ExprLocalVariable:
		return k
	case ir.Literal:
		return k
	case ir.ExprZeroValue:
		return k
	case ir.ExprConstant:
		return k
	case ir.ExprOverride:
		return k
	case ir.ExprWorkGroupUniformLoadResult:
		return k
	case ir.ExprRayQueryProceedResult:
		return k
	case ir.ExprRayQueryGetIntersection:
		return ir.ExprRayQueryGetIntersection{Query: adjust(k.Query), Committed: k.Committed}
	case ir.ExprSubgroupBallotResult:
		return k
	case ir.ExprSubgroupOperationResult:
		return k
	default:
		return kind
	}
}

// adjustImageSampleLevel adjusts expression handles in a SampleLevel.
func adjustImageSampleLevel(level ir.SampleLevel, adjust func(ir.ExpressionHandle) ir.ExpressionHandle) ir.SampleLevel {
	if level == nil {
		return nil
	}
	switch l := level.(type) {
	case ir.SampleLevelExact:
		return ir.SampleLevelExact{Level: adjust(l.Level)}
	case ir.SampleLevelBias:
		return ir.SampleLevelBias{Bias: adjust(l.Bias)}
	case ir.SampleLevelGradient:
		return ir.SampleLevelGradient{X: adjust(l.X), Y: adjust(l.Y)}
	default:
		return level
	}
}

// adjustImageQuery adjusts expression handles in an ExprImageQuery.
func adjustImageQuery(k ir.ExprImageQuery, adjust func(ir.ExpressionHandle) ir.ExpressionHandle) ir.ExprImageQuery {
	result := ir.ExprImageQuery{Image: adjust(k.Image), Query: k.Query}
	return result
}

// adjustBlockHandles updates all expression handles in a block of statements
// using the handle map, and fixes Emit ranges for expressions that became literals.
func adjustBlockHandles(block ir.Block, handleMap []ir.ExpressionHandle, exprs []ir.Expression) ir.Block {
	adjust := func(h ir.ExpressionHandle) ir.ExpressionHandle {
		if int(h) < len(handleMap) {
			return handleMap[h]
		}
		return h
	}
	adjustPtr := func(h *ir.ExpressionHandle) *ir.ExpressionHandle {
		if h == nil {
			return nil
		}
		v := adjust(*h)
		return &v
	}

	var result ir.Block
	for _, stmt := range block {
		switch k := stmt.Kind.(type) {
		case ir.StmtEmit:
			// Map the emit range to new handles and split around pre-emit expressions.
			newStart := adjust(k.Range.Start)
			newEnd := adjust(k.Range.End-1) + 1 // Map last included expr, then +1
			// Split the emit range, excluding pre-emit expressions.
			start := newStart
			for i := newStart; i < newEnd; i++ {
				isPreEmit := false
				if int(i) < len(exprs) {
					switch exprs[i].Kind.(type) {
					case ir.Literal, ir.ExprZeroValue, ir.ExprConstant, ir.ExprOverride:
						isPreEmit = true
					}
				}
				if isPreEmit {
					if i > start {
						result = append(result, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: start, End: i}}})
					}
					start = i + 1
				}
			}
			if start < newEnd {
				result = append(result, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: start, End: newEnd}}})
			}

		case ir.StmtStore:
			result = append(result, ir.Statement{Kind: ir.StmtStore{
				Pointer: adjust(k.Pointer), Value: adjust(k.Value),
			}})

		case ir.StmtReturn:
			result = append(result, ir.Statement{Kind: ir.StmtReturn{Value: adjustPtr(k.Value)}})

		case ir.StmtIf:
			result = append(result, ir.Statement{Kind: ir.StmtIf{
				Condition: adjust(k.Condition),
				Accept:    adjustBlockHandles(k.Accept, handleMap, exprs),
				Reject:    adjustBlockHandles(k.Reject, handleMap, exprs),
			}})

		case ir.StmtSwitch:
			newCases := make([]ir.SwitchCase, len(k.Cases))
			for i, c := range k.Cases {
				newCases[i] = ir.SwitchCase{
					Value:       c.Value,
					Body:        adjustBlockHandles(c.Body, handleMap, exprs),
					FallThrough: c.FallThrough,
				}
			}
			result = append(result, ir.Statement{Kind: ir.StmtSwitch{
				Selector: adjust(k.Selector), Cases: newCases,
			}})

		case ir.StmtLoop:
			result = append(result, ir.Statement{Kind: ir.StmtLoop{
				Body:       adjustBlockHandles(k.Body, handleMap, exprs),
				Continuing: adjustBlockHandles(k.Continuing, handleMap, exprs),
				BreakIf:    adjustPtr(k.BreakIf),
			}})

		case ir.StmtBlock:
			result = append(result, ir.Statement{Kind: ir.StmtBlock{
				Block: adjustBlockHandles(k.Block, handleMap, exprs),
			}})

		case ir.StmtCall:
			newArgs := make([]ir.ExpressionHandle, len(k.Arguments))
			for i, a := range k.Arguments {
				newArgs[i] = adjust(a)
			}
			result = append(result, ir.Statement{Kind: ir.StmtCall{
				Function:  k.Function,
				Arguments: newArgs,
				Result:    adjustPtr(k.Result),
			}})

		case ir.StmtAtomic:
			result = append(result, ir.Statement{Kind: ir.StmtAtomic{
				Pointer: adjust(k.Pointer),
				Fun:     adjustAtomicFun(k.Fun, adjust),
				Value:   adjust(k.Value),
				Result:  adjustPtr(k.Result),
			}})

		case ir.StmtImageStore:
			result = append(result, ir.Statement{Kind: ir.StmtImageStore{
				Image: adjust(k.Image), Coordinate: adjust(k.Coordinate),
				ArrayIndex: adjustPtr(k.ArrayIndex), Value: adjust(k.Value),
			}})

		case ir.StmtImageAtomic:
			result = append(result, ir.Statement{Kind: ir.StmtImageAtomic{
				Image: adjust(k.Image), Coordinate: adjust(k.Coordinate),
				ArrayIndex: adjustPtr(k.ArrayIndex), Fun: k.Fun, Value: adjust(k.Value),
			}})

		case ir.StmtBarrier:
			result = append(result, stmt)

		case ir.StmtWorkGroupUniformLoad:
			result = append(result, ir.Statement{Kind: ir.StmtWorkGroupUniformLoad{
				Pointer: adjust(k.Pointer), Result: adjust(k.Result),
			}})

		case ir.StmtRayQuery:
			result = append(result, ir.Statement{Kind: adjustRayQueryStmt(k, adjust)})

		case ir.StmtSubgroupBallot:
			result = append(result, ir.Statement{Kind: ir.StmtSubgroupBallot{
				Result: adjust(k.Result), Predicate: adjustPtr(k.Predicate),
			}})

		case ir.StmtSubgroupCollectiveOperation:
			result = append(result, ir.Statement{Kind: ir.StmtSubgroupCollectiveOperation{
				Op: k.Op, CollectiveOp: k.CollectiveOp,
				Argument: adjust(k.Argument), Result: adjust(k.Result),
			}})

		case ir.StmtSubgroupGather:
			result = append(result, ir.Statement{Kind: adjustSubgroupGatherStmt(k, adjust)})

		case ir.StmtKill:
			result = append(result, stmt)

		case ir.StmtBreak:
			result = append(result, stmt)

		case ir.StmtContinue:
			result = append(result, stmt)

		default:
			result = append(result, stmt)
		}
	}
	return result
}

// adjustRayQueryStmt adjusts expression handles in a RayQuery statement.
func adjustRayQueryStmt(k ir.StmtRayQuery, adjust func(ir.ExpressionHandle) ir.ExpressionHandle) ir.StmtRayQuery {
	result := ir.StmtRayQuery{Query: adjust(k.Query)}
	if k.Fun != nil {
		switch f := k.Fun.(type) {
		case ir.RayQueryInitialize:
			result.Fun = ir.RayQueryInitialize{
				AccelerationStructure: adjust(f.AccelerationStructure),
				Descriptor:            adjust(f.Descriptor),
			}
		case ir.RayQueryProceed:
			result.Fun = ir.RayQueryProceed{Result: adjust(f.Result)}
		case ir.RayQueryGenerateIntersection:
			result.Fun = ir.RayQueryGenerateIntersection{HitT: adjust(f.HitT)}
		case ir.RayQueryTerminate:
			result.Fun = f
		case ir.RayQueryConfirmIntersection:
			result.Fun = f
		default:
			result.Fun = f
		}
	}
	return result
}

// adjustAtomicFun adjusts expression handles in an AtomicFunction variant.
func adjustAtomicFun(fun ir.AtomicFunction, adjust func(ir.ExpressionHandle) ir.ExpressionHandle) ir.AtomicFunction {
	if fun == nil {
		return nil
	}
	switch f := fun.(type) {
	case ir.AtomicExchange:
		if f.Compare != nil {
			v := adjust(*f.Compare)
			return ir.AtomicExchange{Compare: &v}
		}
		return f
	default:
		return fun
	}
}

// adjustSubgroupGatherStmt adjusts expression handles in a SubgroupGather statement.
func adjustSubgroupGatherStmt(k ir.StmtSubgroupGather, adjust func(ir.ExpressionHandle) ir.ExpressionHandle) ir.StmtSubgroupGather {
	result := ir.StmtSubgroupGather{
		Argument: adjust(k.Argument),
		Result:   adjust(k.Result),
	}
	if k.Mode != nil {
		switch m := k.Mode.(type) {
		case ir.GatherBroadcast:
			result.Mode = ir.GatherBroadcast{Index: adjust(m.Index)}
		case ir.GatherShuffle:
			result.Mode = ir.GatherShuffle{Index: adjust(m.Index)}
		case ir.GatherShuffleDown:
			result.Mode = ir.GatherShuffleDown{Delta: adjust(m.Delta)}
		case ir.GatherShuffleUp:
			result.Mode = ir.GatherShuffleUp{Delta: adjust(m.Delta)}
		case ir.GatherShuffleXor:
			result.Mode = ir.GatherShuffleXor{Mask: adjust(m.Mask)}
		case ir.GatherQuadBroadcast:
			result.Mode = ir.GatherQuadBroadcast{Index: adjust(m.Index)}
		default:
			result.Mode = k.Mode // BroadcastFirst, QuadSwap etc. have no handles
		}
	}
	return result
}

// mapValueToScalar converts a float64 pipeline constant value to an IR ScalarValue
// matching the target scalar type. Follows WebIDL conversion rules.
func mapValueToScalar(value float64, scalar ir.ScalarType) (ir.ScalarValue, error) {
	switch scalar.Kind {
	case ir.ScalarBool:
		// https://webidl.spec.whatwg.org/#js-boolean
		// NaN and 0 are false, everything else is true.
		var bits uint64
		if value != 0.0 && !math.IsNaN(value) {
			bits = 1
		}
		return ir.ScalarValue{Bits: bits, Kind: ir.ScalarBool}, nil

	case ir.ScalarSint:
		if !math.IsInf(value, 0) && !math.IsNaN(value) {
			v := math.Trunc(value)
			if v >= math.MinInt32 && v <= math.MaxInt32 {
				return ir.ScalarValue{Bits: uint64(int32(v)), Kind: ir.ScalarSint}, nil
			}
		}
		return ir.ScalarValue{}, fmt.Errorf("f64 value %v cannot be converted to i32", value)

	case ir.ScalarUint:
		if !math.IsInf(value, 0) && !math.IsNaN(value) {
			v := math.Trunc(value)
			if v >= 0 && v <= math.MaxUint32 {
				return ir.ScalarValue{Bits: uint64(uint32(v)), Kind: ir.ScalarUint}, nil
			}
		}
		return ir.ScalarValue{}, fmt.Errorf("f64 value %v cannot be converted to u32", value)

	case ir.ScalarFloat:
		if scalar.Width == 8 {
			// f64
			return ir.ScalarValue{Bits: math.Float64bits(value), Kind: ir.ScalarFloat}, nil
		}
		// f32 (default)
		f32 := float32(value)
		return ir.ScalarValue{Bits: uint64(math.Float32bits(f32)), Kind: ir.ScalarFloat}, nil

	default:
		return ir.ScalarValue{}, fmt.Errorf("unsupported scalar kind for pipeline constant: %d", scalar.Kind)
	}
}
