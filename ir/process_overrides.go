// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import (
	"fmt"
)

// CloneModuleForOverrides creates a deep enough copy of a module for ProcessOverrides
// to safely mutate. Clones: GlobalExpressions, Constants, Functions (expressions),
// EntryPoints (expressions). Shared immutable data (Types, GlobalVariables) is not copied.
func CloneModuleForOverrides(src *Module) *Module {
	dst := *src // shallow copy

	// Clone Overrides (ProcessOverrides reads Init pointers which are shared)
	dst.Overrides = make([]Override, len(src.Overrides))
	copy(dst.Overrides, src.Overrides)
	for i := range dst.Overrides {
		if dst.Overrides[i].Init != nil {
			initCopy := *dst.Overrides[i].Init
			dst.Overrides[i].Init = &initCopy
		}
		if dst.Overrides[i].ID != nil {
			idCopy := *dst.Overrides[i].ID
			dst.Overrides[i].ID = &idCopy
		}
	}

	// Clone GlobalExpressions (we append to it)
	dst.GlobalExpressions = make([]Expression, len(src.GlobalExpressions))
	copy(dst.GlobalExpressions, src.GlobalExpressions)

	// Clone Constants (we append to it)
	dst.Constants = make([]Constant, len(src.Constants))
	copy(dst.Constants, src.Constants)

	// Clone Functions — we modify expressions, local vars, named expressions in place
	dst.Functions = make([]Function, len(src.Functions))
	for i := range src.Functions {
		dst.Functions[i] = src.Functions[i]
		dst.Functions[i].Expressions = make([]Expression, len(src.Functions[i].Expressions))
		copy(dst.Functions[i].Expressions, src.Functions[i].Expressions)
		if len(src.Functions[i].ExpressionTypes) > 0 {
			dst.Functions[i].ExpressionTypes = make([]TypeResolution, len(src.Functions[i].ExpressionTypes))
			copy(dst.Functions[i].ExpressionTypes, src.Functions[i].ExpressionTypes)
		}
		if len(src.Functions[i].LocalVars) > 0 {
			dst.Functions[i].LocalVars = make([]LocalVariable, len(src.Functions[i].LocalVars))
			copy(dst.Functions[i].LocalVars, src.Functions[i].LocalVars)
			for j := range dst.Functions[i].LocalVars {
				if dst.Functions[i].LocalVars[j].Init != nil {
					cp := *dst.Functions[i].LocalVars[j].Init
					dst.Functions[i].LocalVars[j].Init = &cp
				}
			}
		}
		if src.Functions[i].NamedExpressions != nil {
			dst.Functions[i].NamedExpressions = make(map[ExpressionHandle]string, len(src.Functions[i].NamedExpressions))
			for k, v := range src.Functions[i].NamedExpressions {
				dst.Functions[i].NamedExpressions[k] = v
			}
		}
		if len(src.Functions[i].Body) > 0 {
			dst.Functions[i].Body = make(Block, len(src.Functions[i].Body))
			copy(dst.Functions[i].Body, src.Functions[i].Body)
		}
	}

	// Clone EntryPoints — we modify expressions, local vars, named expressions in place
	dst.EntryPoints = make([]EntryPoint, len(src.EntryPoints))
	for i := range src.EntryPoints {
		dst.EntryPoints[i] = src.EntryPoints[i]
		dst.EntryPoints[i].Function.Expressions = make([]Expression, len(src.EntryPoints[i].Function.Expressions))
		copy(dst.EntryPoints[i].Function.Expressions, src.EntryPoints[i].Function.Expressions)
		if len(src.EntryPoints[i].Function.ExpressionTypes) > 0 {
			dst.EntryPoints[i].Function.ExpressionTypes = make([]TypeResolution, len(src.EntryPoints[i].Function.ExpressionTypes))
			copy(dst.EntryPoints[i].Function.ExpressionTypes, src.EntryPoints[i].Function.ExpressionTypes)
		}
		// Deep copy LocalVars (Init pointers are shared)
		if len(src.EntryPoints[i].Function.LocalVars) > 0 {
			dst.EntryPoints[i].Function.LocalVars = make([]LocalVariable, len(src.EntryPoints[i].Function.LocalVars))
			copy(dst.EntryPoints[i].Function.LocalVars, src.EntryPoints[i].Function.LocalVars)
			for j := range dst.EntryPoints[i].Function.LocalVars {
				if dst.EntryPoints[i].Function.LocalVars[j].Init != nil {
					cp := *dst.EntryPoints[i].Function.LocalVars[j].Init
					dst.EntryPoints[i].Function.LocalVars[j].Init = &cp
				}
			}
		}
		// Deep copy NamedExpressions map
		if src.EntryPoints[i].Function.NamedExpressions != nil {
			dst.EntryPoints[i].Function.NamedExpressions = make(map[ExpressionHandle]string, len(src.EntryPoints[i].Function.NamedExpressions))
			for k, v := range src.EntryPoints[i].Function.NamedExpressions {
				dst.EntryPoints[i].Function.NamedExpressions[k] = v
			}
		}
		// Deep copy Body (statements contain expression handles that get remapped)
		if len(src.EntryPoints[i].Function.Body) > 0 {
			dst.EntryPoints[i].Function.Body = make(Block, len(src.EntryPoints[i].Function.Body))
			copy(dst.EntryPoints[i].Function.Body, src.EntryPoints[i].Function.Body)
		}
	}

	return &dst
}

// PipelineConstants maps override keys (ID as string or name) to float64 values.
// NaN means "not set" (use default initializer).
// Matches Rust naga's back::PipelineConstants = HashMap<String, f64>.
type PipelineConstants map[string]float64

// ProcessOverrides resolves all overrides in the module to concrete constants
// using provided pipeline constant values. Modifies the module in place:
// - Overrides become Constants with resolved values
// - ExprOverride in global expressions become ExprConstant
// - ExprOverride in function expressions become Literal with resolved values
// - Global variable initializers using overrides are evaluated
//
// Matches Rust naga's back::pipeline_constants::process_overrides.
func ProcessOverrides(module *Module, constants PipelineConstants) error {
	if len(module.Overrides) == 0 {
		return nil
	}

	// Phase 1: Resolve each override to a concrete value
	resolvedValues := make([]float64, len(module.Overrides))
	for i := range module.Overrides {
		val, err := resolveOverrideValue(module, i, constants, resolvedValues)
		if err != nil {
			return fmt.Errorf("override %q: %w", module.Overrides[i].Name, err)
		}
		resolvedValues[i] = val
	}

	// Phase 2: Create constants for each override and replace ExprOverride
	// in global expressions with the resolved values
	overrideToConstant := make(map[OverrideHandle]ConstantHandle, len(module.Overrides))
	for i, ov := range module.Overrides {
		lit := makeOverrideLiteral(module, ov.Ty, resolvedValues[i])
		// Add literal as a new global expression
		geHandle := ExpressionHandle(len(module.GlobalExpressions))
		module.GlobalExpressions = append(module.GlobalExpressions, Expression{Kind: lit})
		// Create constant pointing to that global expression.
		// Value is nil — writer should use Init (GlobalExpression) path.
		ch := ConstantHandle(len(module.Constants))
		module.Constants = append(module.Constants, Constant{
			Name: ov.Name,
			Type: ov.Ty,
			Init: geHandle,
		})
		overrideToConstant[OverrideHandle(i)] = ch
	}

	// Phase 3: Replace ExprOverride in global expressions
	for i := range module.GlobalExpressions {
		if eo, ok := module.GlobalExpressions[i].Kind.(ExprOverride); ok {
			if ch, ok := overrideToConstant[eo.Override]; ok {
				module.GlobalExpressions[i].Kind = ExprConstant{Constant: ch}
			}
		}
	}

	// Phase 4: Replace ExprOverride in function expressions with ExprConstant
	for fi := range module.Functions {
		rebuildFunctionExpressions(&module.Functions[fi], module, overrideToConstant)
	}
	for ei := range module.EntryPoints {
		rebuildFunctionExpressions(&module.EntryPoints[ei].Function, module, overrideToConstant)
	}

	// Note: NO compact after const-fold. Rust runs compact BEFORE const-eval,
	// so folded expressions stay in the arena at their original indices.
	// This preserves expression handle numbering for baked variable names (_eN).

	// Phase 5: Evaluate global variable initializers that reference overrides
	// (e.g., var<private> gain_x_10: f32 = gain * 10.)
	evaluateGlobalInitializers(module, resolvedValues)

	return nil
}

// resolveOverrideValue determines the concrete value for an override.
func resolveOverrideValue(module *Module, idx int, constants PipelineConstants, resolved []float64) (float64, error) {
	ov := &module.Overrides[idx]

	// Check pipeline constants by ID first, then by name.
	// NaN IS a valid value — for bools, NaN converts to false (not 0.0 and not 1.0).
	if ov.ID != nil {
		key := fmt.Sprintf("%d", *ov.ID)
		if val, ok := constants[key]; ok {
			return val, nil
		}
	}
	if ov.Name != "" {
		if val, ok := constants[ov.Name]; ok {
			return val, nil
		}
	}

	// Use default initializer if available
	if ov.Init != nil {
		return evaluateGlobalExprAsFloat(module, *ov.Init, resolved)
	}

	return 0, fmt.Errorf("no value provided and no default initializer")
}

// evaluateGlobalExprAsFloat evaluates a global expression to a float64 value.
// Handles Literal, Override references (using already-resolved values), and binary ops.
func evaluateGlobalExprAsFloat(module *Module, handle ExpressionHandle, resolved []float64) (float64, error) {
	if int(handle) >= len(module.GlobalExpressions) {
		return 0, fmt.Errorf("global expression %d out of range", handle)
	}
	expr := &module.GlobalExpressions[handle]
	switch k := expr.Kind.(type) {
	case Literal:
		return LiteralToFloat(k.Value), nil
	case ExprOverride:
		if int(k.Override) < len(resolved) {
			return resolved[k.Override], nil
		}
		return 0, fmt.Errorf("override %d not yet resolved", k.Override)
	case ExprBinary:
		left, err := evaluateGlobalExprAsFloat(module, k.Left, resolved)
		if err != nil {
			return 0, err
		}
		right, err := evaluateGlobalExprAsFloat(module, k.Right, resolved)
		if err != nil {
			return 0, err
		}
		return EvalBinaryFloat(k.Op, left, right), nil
	case ExprUnary:
		val, err := evaluateGlobalExprAsFloat(module, k.Expr, resolved)
		if err != nil {
			return 0, err
		}
		return EvalUnaryFloat(k.Op, val), nil
	case ExprConstant:
		// Evaluate constant by looking at its init expression
		if int(k.Constant) < len(module.Constants) {
			c := &module.Constants[k.Constant]
			return evaluateGlobalExprAsFloat(module, c.Init, resolved)
		}
		return 0, fmt.Errorf("cannot evaluate constant %d", k.Constant)
	default:
		return 0, fmt.Errorf("cannot evaluate global expression of kind %T", k)
	}
}

// LiteralToFloat converts a LiteralValue to float64.
func LiteralToFloat(v LiteralValue) float64 {
	switch val := v.(type) {
	case LiteralF32:
		return float64(val)
	case LiteralF64:
		return float64(val)
	case LiteralI32:
		return float64(val)
	case LiteralU32:
		return float64(val)
	case LiteralBool:
		if bool(val) {
			return 1.0
		}
		return 0.0
	case LiteralAbstractInt:
		return float64(val)
	case LiteralAbstractFloat:
		return float64(val)
	default:
		return 0
	}
}

func makeOverrideLiteral(module *Module, typeHandle TypeHandle, val float64) Literal {
	if int(typeHandle) < len(module.Types) {
		switch module.Types[typeHandle].Inner.(type) {
		case ScalarType:
			scalar := module.Types[typeHandle].Inner.(ScalarType)
			switch scalar.Kind {
			case ScalarBool:
				// NaN converts to false (Rust: f64 → bool is val == 1.0)
				return Literal{Value: LiteralBool(val == 1.0)}
			case ScalarSint:
				return Literal{Value: LiteralI32(int32(val))}
			case ScalarUint:
				return Literal{Value: LiteralU32(uint32(val))}
			case ScalarFloat:
				if scalar.Width == 8 {
					return Literal{Value: LiteralF64(val)}
				}
				return Literal{Value: LiteralF32(float32(val))}
			}
		}
	}
	return Literal{Value: LiteralF32(float32(val))}
}

// EvalBinaryFloat evaluates a binary operation on two float64 values.
func EvalBinaryFloat(op BinaryOperator, left, right float64) float64 {
	switch op {
	case BinaryAdd:
		return left + right
	case BinarySubtract:
		return left - right
	case BinaryMultiply:
		return left * right
	case BinaryDivide:
		if right == 0 {
			return 0
		}
		return left / right
	default:
		return 0
	}
}

// EvalUnaryFloat evaluates a unary operation on a float64 value.
func EvalUnaryFloat(op UnaryOperator, val float64) float64 {
	switch op {
	case UnaryNegate:
		return -val
	case UnaryLogicalNot:
		if val == 0 {
			return 1
		}
		return 0
	case UnaryBitwiseNot:
		return float64(^int64(val))
	default:
		return val
	}
}

// rebuildFunctionExpressions rebuilds the function expression arena from scratch.
// Matches Rust naga's process_function: drain all expressions, for each one
// replace Override→Constant, remap handles, try const-evaluate, append to new arena.
// Then remap ALL handles in statements, local vars, and named expressions.
func rebuildFunctionExpressions(fn *Function, module *Module, overrideToConstant map[OverrideHandle]ConstantHandle) {
	oldExprs := fn.Expressions
	newExprs := make([]Expression, 0, len(oldExprs)+4) // +4 for potential const-eval additions
	handleMap := make([]ExpressionHandle, len(oldExprs))

	for i, expr := range oldExprs {
		kind := expr.Kind

		// Replace ExprOverride → ExprConstant
		if eo, ok := kind.(ExprOverride); ok {
			if ch, ok := overrideToConstant[eo.Override]; ok {
				kind = ExprConstant{Constant: ch}
			}
		}

		// Remap sub-expression handles to new arena indices
		kind = overrideRemapExprHandles(kind, handleMap[:i])

		// Const-evaluate matching Rust's try_eval_and_append behavior:
		// - ExprConstant → keep original, append Literal copy, handle maps to ORIGINAL
		//   (constant names preserved in output, Literal is for downstream eval chain)
		// - Binary/Unary on known operands → REPLACE with evaluated Literal
		// - Everything else → keep as-is
		if _, isConst := kind.(ExprConstant); isConst {
			// Append ExprConstant — handle maps here (preserves name)
			newH := ExpressionHandle(len(newExprs))
			newExprs = append(newExprs, Expression{Kind: kind})
			handleMap[i] = newH
			// Also append evaluated Literal (for downstream Binary eval to find)
			if evaluated, ok := tryConstEval(kind, newExprs, module); ok {
				newExprs = append(newExprs, Expression{Kind: evaluated})
			}
		} else if evaluated, ok := tryConstEval(kind, newExprs, module); ok {
			// Binary/Unary: replace with evaluated Literal
			newH := ExpressionHandle(len(newExprs))
			newExprs = append(newExprs, Expression{Kind: evaluated})
			handleMap[i] = newH
		} else {
			newH := ExpressionHandle(len(newExprs))
			newExprs = append(newExprs, Expression{Kind: kind})
			handleMap[i] = newH
		}
	}

	fn.Expressions = newExprs

	// Rebuild ExpressionTypes for new arena
	fn.ExpressionTypes = make([]TypeResolution, len(newExprs))
	for i := range newExprs {
		res, err := ResolveExpressionType(module, fn, ExpressionHandle(i))
		if err == nil {
			fn.ExpressionTypes[i] = res
		}
	}

	// Remap ALL handles in function body statements
	remapBlockHandles(fn.Body, handleMap)

	// Remap local var init handles
	for i := range fn.LocalVars {
		if fn.LocalVars[i].Init != nil {
			old := *fn.LocalVars[i].Init
			if int(old) < len(handleMap) {
				*fn.LocalVars[i].Init = handleMap[old]
			}
		}
	}

	// Rebuild named expressions with new handles
	if fn.NamedExpressions != nil {
		newNamed := make(map[ExpressionHandle]string, len(fn.NamedExpressions))
		for oldH, name := range fn.NamedExpressions {
			if int(oldH) < len(handleMap) {
				newNamed[handleMap[oldH]] = name
			}
		}
		fn.NamedExpressions = newNamed
	}
}

// tryConstEval tries to const-evaluate an expression kind given the current new arena.
func tryConstEval(kind ExpressionKind, arena []Expression, module *Module) (ExpressionKind, bool) {
	switch k := kind.(type) {
	case ExprBinary:
		leftVal, leftLit, leftOk := arenaExprAsFloat(arena, module, k.Left)
		rightVal, _, rightOk := arenaExprAsFloat(arena, module, k.Right)
		if leftOk && rightOk {
			result := EvalBinaryFloat(k.Op, leftVal, rightVal)
			return makeLiteralFromProto(leftLit, result), true
		}
	case ExprUnary:
		val, lit, ok := arenaExprAsFloat(arena, module, k.Expr)
		if ok {
			switch k.Op {
			case UnaryNegate:
				return makeLiteralFromProto(lit, -val), true
			case UnaryLogicalNot:
				if _, isBool := lit.Value.(LiteralBool); isBool {
					return Literal{Value: LiteralBool(!bool(lit.Value.(LiteralBool)))}, true
				}
				return makeLiteralFromProto(lit, EvalUnaryFloat(k.Op, val)), true
			}
		}
	case ExprConstant:
		// Evaluate to its literal value (for appending as shadow Literal)
		if int(k.Constant) < len(module.Constants) {
			c := &module.Constants[k.Constant]
			if int(c.Init) < len(module.GlobalExpressions) {
				if lit, ok := module.GlobalExpressions[c.Init].Kind.(Literal); ok {
					return lit, true
				}
			}
		}
	}
	return nil, false
}

// arenaExprAsFloat resolves an expression in the new arena to a float64 value.
func arenaExprAsFloat(arena []Expression, module *Module, handle ExpressionHandle) (float64, Literal, bool) {
	if int(handle) >= len(arena) {
		return 0, Literal{}, false
	}
	expr := &arena[handle]
	if lit, ok := expr.Kind.(Literal); ok {
		return LiteralToFloat(lit.Value), lit, true
	}
	if ec, ok := expr.Kind.(ExprConstant); ok {
		if int(ec.Constant) < len(module.Constants) {
			c := &module.Constants[ec.Constant]
			if int(c.Init) < len(module.GlobalExpressions) {
				if lit, ok := module.GlobalExpressions[c.Init].Kind.(Literal); ok {
					return LiteralToFloat(lit.Value), lit, true
				}
			}
		}
	}
	return 0, Literal{}, false
}

// remapExprHandles remaps sub-expression handles in an expression kind.
func overrideRemapExprHandles(kind ExpressionKind, handleMap []ExpressionHandle) ExpressionKind {
	remap := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(handleMap) {
			return handleMap[h]
		}
		return h
	}
	remapPtr := func(p *ExpressionHandle) {
		if p != nil && int(*p) < len(handleMap) {
			*p = handleMap[*p]
		}
	}
	switch k := kind.(type) {
	case ExprAccess:
		return ExprAccess{Base: remap(k.Base), Index: remap(k.Index)}
	case ExprAccessIndex:
		return ExprAccessIndex{Base: remap(k.Base), Index: k.Index}
	case ExprBinary:
		return ExprBinary{Op: k.Op, Left: remap(k.Left), Right: remap(k.Right)}
	case ExprUnary:
		return ExprUnary{Op: k.Op, Expr: remap(k.Expr)}
	case ExprLoad:
		return ExprLoad{Pointer: remap(k.Pointer)}
	case ExprSelect:
		return ExprSelect{Condition: remap(k.Condition), Accept: remap(k.Accept), Reject: remap(k.Reject)}
	case ExprSplat:
		return ExprSplat{Size: k.Size, Value: remap(k.Value)}
	case ExprSwizzle:
		return ExprSwizzle{Size: k.Size, Vector: remap(k.Vector), Pattern: k.Pattern}
	case ExprCompose:
		comps := make([]ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			comps[i] = remap(c)
		}
		return ExprCompose{Type: k.Type, Components: comps}
	case ExprAs:
		return ExprAs{Expr: remap(k.Expr), Kind: k.Kind, Convert: k.Convert}
	case ExprImageSample:
		s := k
		s.Image = remap(s.Image)
		s.Sampler = remap(s.Sampler)
		s.Coordinate = remap(s.Coordinate)
		remapPtr(s.ArrayIndex)
		remapPtr(s.DepthRef)
		remapPtr(s.Offset)
		// Remap SampleLevel handles
		switch lv := s.Level.(type) {
		case SampleLevelExact:
			lv.Level = remap(lv.Level)
			s.Level = lv
		case SampleLevelBias:
			lv.Bias = remap(lv.Bias)
			s.Level = lv
		}
		return s
	case ExprImageLoad:
		l := k
		l.Image = remap(l.Image)
		l.Coordinate = remap(l.Coordinate)
		remapPtr(l.ArrayIndex)
		remapPtr(l.Sample)
		remapPtr(l.Level)
		return l
	case ExprImageQuery:
		q := k
		q.Image = remap(q.Image)
		// ImageQuery.Query may contain handles
		switch qv := q.Query.(type) {
		case ImageQuerySize:
			remapPtr(qv.Level)
			q.Query = qv
		}
		return q
	case ExprDerivative:
		return ExprDerivative{Axis: k.Axis, Control: k.Control, Expr: remap(k.Expr)}
	case ExprMath:
		m := k
		m.Arg = remap(m.Arg)
		if m.Arg1 != nil {
			h := remap(*m.Arg1)
			m.Arg1 = &h
		}
		if m.Arg2 != nil {
			h := remap(*m.Arg2)
			m.Arg2 = &h
		}
		if m.Arg3 != nil {
			h := remap(*m.Arg3)
			m.Arg3 = &h
		}
		return m
	case ExprRelational:
		return ExprRelational{Fun: k.Fun, Argument: remap(k.Argument)}
	case ExprArrayLength:
		return ExprArrayLength{Array: remap(k.Array)}
	}
	// Literal, ExprConstant, ExprGlobalVariable, ExprLocalVariable, ExprFunctionArgument,
	// ExprCallResult, ExprAtomicResult, etc. — no sub-expression handles to remap
	return kind
}

// remapBlockHandles remaps all expression handles in a block of statements.
func remapBlockHandles(block Block, handleMap []ExpressionHandle) {
	remap := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(handleMap) {
			return handleMap[h]
		}
		return h
	}
	remapPtr := func(p *ExpressionHandle) {
		if p != nil && int(*p) < len(handleMap) {
			*p = handleMap[*p]
		}
	}

	for i := range block {
		switch k := block[i].Kind.(type) {
		case StmtEmit:
			// Remap emit range
			k.Range.Start = remap(k.Range.Start)
			k.Range.End = remap(k.Range.End)
			block[i].Kind = k
		case StmtStore:
			k.Pointer = remap(k.Pointer)
			k.Value = remap(k.Value)
			block[i].Kind = k
		case StmtCall:
			for j := range k.Arguments {
				k.Arguments[j] = remap(k.Arguments[j])
			}
			if k.Result != nil {
				*k.Result = remap(*k.Result)
			}
			block[i].Kind = k
		case StmtIf:
			k.Condition = remap(k.Condition)
			remapBlockHandles(k.Accept, handleMap)
			remapBlockHandles(k.Reject, handleMap)
			block[i].Kind = k
		case StmtSwitch:
			k.Selector = remap(k.Selector)
			for j := range k.Cases {
				remapBlockHandles(k.Cases[j].Body, handleMap)
			}
			block[i].Kind = k
		case StmtLoop:
			remapBlockHandles(k.Body, handleMap)
			remapBlockHandles(k.Continuing, handleMap)
			if k.BreakIf != nil {
				*k.BreakIf = remap(*k.BreakIf)
			}
			block[i].Kind = k
		case StmtReturn:
			if k.Value != nil {
				*k.Value = remap(*k.Value)
			}
			block[i].Kind = k
		case StmtImageStore:
			k.Image = remap(k.Image)
			k.Coordinate = remap(k.Coordinate)
			remapPtr(k.ArrayIndex)
			k.Value = remap(k.Value)
			block[i].Kind = k
		case StmtAtomic:
			k.Pointer = remap(k.Pointer)
			k.Value = remap(k.Value)
			if k.Result != nil {
				*k.Result = remap(*k.Result)
			}
			// Remap Compare handle inside AtomicExchange
			if exchange, ok := k.Fun.(AtomicExchange); ok && exchange.Compare != nil {
				*exchange.Compare = remap(*exchange.Compare)
				k.Fun = exchange
			}
			block[i].Kind = k
		case StmtBlock:
			remapBlockHandles(k.Block, handleMap)
			block[i].Kind = k
		case StmtWorkGroupUniformLoad:
			k.Pointer = remap(k.Pointer)
			k.Result = remap(k.Result)
			block[i].Kind = k
		case StmtRayQuery:
			k.Query = remap(k.Query)
			switch f := k.Fun.(type) {
			case RayQueryInitialize:
				f.AccelerationStructure = remap(f.AccelerationStructure)
				f.Descriptor = remap(f.Descriptor)
				k.Fun = f
			case RayQueryProceed:
				f.Result = remap(f.Result)
				k.Fun = f
			case RayQueryGenerateIntersection:
				f.HitT = remap(f.HitT)
				k.Fun = f
			}
			block[i].Kind = k
		}
	}
}

// evalFuncExprAsFloat evaluates a function expression to a float64, recursing through
// Binary/Unary/ExprConstant/Literal. Returns (value, true) if evaluable.
func evalFuncExprAsFloat(fn *Function, module *Module, handle ExpressionHandle) (float64, bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, false
	}
	expr := &fn.Expressions[handle]
	switch k := expr.Kind.(type) {
	case Literal:
		return LiteralToFloat(k.Value), true
	case ExprConstant:
		if int(k.Constant) < len(module.Constants) {
			c := &module.Constants[k.Constant]
			if int(c.Init) < len(module.GlobalExpressions) {
				ge := &module.GlobalExpressions[c.Init]
				if lit, ok := ge.Kind.(Literal); ok {
					return LiteralToFloat(lit.Value), true
				}
			}
		}
		return 0, false
	case ExprBinary:
		left, leftOk := evalFuncExprAsFloat(fn, module, k.Left)
		right, rightOk := evalFuncExprAsFloat(fn, module, k.Right)
		if !leftOk || !rightOk {
			return 0, false
		}
		return EvalBinaryFloat(k.Op, left, right), true
	case ExprUnary:
		val, ok := evalFuncExprAsFloat(fn, module, k.Expr)
		if !ok {
			return 0, false
		}
		return EvalUnaryFloat(k.Op, val), true
	}
	return 0, false
}

// tryConstFoldExpr tries to fold a binary/unary expression with constant/literal operands.
func tryConstFoldExpr(fn *Function, module *Module, idx int) (ExpressionKind, bool) {
	expr := &fn.Expressions[idx]
	switch k := expr.Kind.(type) {
	case ExprBinary:
		leftVal, leftLit, leftOk := exprAsLiteral(fn, module, k.Left)
		rightVal, _, rightOk := exprAsLiteral(fn, module, k.Right)
		if !leftOk || !rightOk {
			return nil, false
		}
		result := EvalBinaryFloat(k.Op, leftVal, rightVal)
		return makeLiteralFromProto(leftLit, result), true
	case ExprUnary:
		innerVal, innerLit, ok := exprAsLiteral(fn, module, k.Expr)
		if !ok {
			return nil, false
		}
		switch k.Op {
		case UnaryNegate:
			return makeLiteralFromProto(innerLit, -innerVal), true
		case UnaryLogicalNot:
			if _, isBool := innerLit.Value.(LiteralBool); isBool {
				return Literal{Value: LiteralBool(!bool(innerLit.Value.(LiteralBool)))}, true
			}
		}
	}
	return nil, false
}

// exprAsLiteral resolves an expression to a float64 value and Literal prototype.
// Handles Literal directly, and ExprConstant by following the init chain.
func exprAsLiteral(fn *Function, module *Module, handle ExpressionHandle) (float64, Literal, bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, Literal{}, false
	}
	expr := &fn.Expressions[handle]
	if lit, ok := expr.Kind.(Literal); ok {
		return LiteralToFloat(lit.Value), lit, true
	}
	if ec, ok := expr.Kind.(ExprConstant); ok {
		if int(ec.Constant) < len(module.Constants) {
			c := &module.Constants[ec.Constant]
			if int(c.Init) < len(module.GlobalExpressions) {
				ge := &module.GlobalExpressions[c.Init]
				if lit, ok := ge.Kind.(Literal); ok {
					return LiteralToFloat(lit.Value), lit, true
				}
			}
		}
	}
	return 0, Literal{}, false
}

// makeLiteralFromProto creates a Literal with the same type as proto but with a new value.
func makeLiteralFromProto(proto Literal, val float64) Literal {
	switch proto.Value.(type) {
	case LiteralBool:
		return Literal{Value: LiteralBool(val == 1.0)}
	case LiteralI32:
		return Literal{Value: LiteralI32(int32(val))}
	case LiteralU32:
		return Literal{Value: LiteralU32(uint32(val))}
	case LiteralF32:
		return Literal{Value: LiteralF32(float32(val))}
	case LiteralF64:
		return Literal{Value: LiteralF64(val)}
	default:
		return Literal{Value: LiteralF32(float32(val))}
	}
}

func resolveLiteralTypeInner(lit Literal) TypeResolution {
	switch lit.Value.(type) {
	case LiteralBool:
		return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}
	case LiteralI32:
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 4}}
	case LiteralU32:
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}
	case LiteralF32:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}
	case LiteralF64:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 8}}
	default:
		return TypeResolution{}
	}
}

// evaluateGlobalInitializers re-evaluates global variable initializers that
// may reference override-dependent expressions.
func evaluateGlobalInitializers(module *Module, resolved []float64) {
	// For each global var with an init expression that points to a global expression,
	// try to const-fold the expression now that overrides are resolved.
	// This handles patterns like: var<private> gain_x_10: f32 = gain * 10.
	for i := range module.GlobalVariables {
		gv := &module.GlobalVariables[i]
		if gv.InitExpr == nil {
			continue
		}
		initHandle := *gv.InitExpr
		if int(initHandle) >= len(module.GlobalExpressions) {
			continue
		}
		// Try to evaluate the initializer expression to a constant
		val, err := evaluateGlobalExprAsFloat(module, initHandle, resolved)
		if err != nil {
			continue // Can't evaluate — leave as-is
		}
		// Replace the global expression with a literal
		lit := makeOverrideLiteral(module, gv.Type, val)
		module.GlobalExpressions[initHandle] = Expression{Kind: lit}
	}
}
