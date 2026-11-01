// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"

	"github.com/gogpu/naga/internal/unsupported"
	"github.com/gogpu/naga/ir"
)

// lowerPrivatesToLocals turns every var<private> an entry point uses into a
// local variable of that entry point. A private variable is one variable per
// invocation, initialized at its start, which is what a local of the entry
// function is; after InlineUserFunctions no other function the entry point
// calls touches a global (a helper that does is always inlined), so this is
// exact. Mesa does the same before its SSA pass (nir_lower_global_vars_to_local,
// then nir_lower_vars_to_ssa; radv_shader.c). Locals then get the emitter's
// per-component allocas, SROA and mem2reg, where a private vector used to get
// one scalar alloca and lose its other components (P0-DXIL-059).
func lowerPrivatesToLocals(m *ir.Module) error {
	for i := range m.EntryPoints {
		if err := privatesToLocals(m, &m.EntryPoints[i].Function); err != nil {
			return fmt.Errorf("entry point %q: %w", m.EntryPoints[i].Name, err)
		}
	}
	return nil
}

func privatesToLocals(m *ir.Module, fn *ir.Function) error {
	locals := make(map[ir.GlobalVariableHandle]uint32)
	n := len(fn.Expressions) // initializers appended below are not rewritten
	for h := 0; h < n; h++ {
		g, ok := fn.Expressions[h].Kind.(ir.ExprGlobalVariable)
		if !ok || m.GlobalVariables[g.Variable].Space != ir.SpacePrivate {
			continue
		}
		idx, seen := locals[g.Variable]
		if !seen {
			gv := &m.GlobalVariables[g.Variable]
			lv := ir.LocalVariable{Name: gv.Name, Type: gv.Type}
			init, hasInit, err := privateInitializer(m, fn, gv)
			if err != nil {
				return fmt.Errorf("private %q: %w", gv.Name, err)
			}
			if hasInit {
				lv.Init = &init
			}
			idx = uint32(len(fn.LocalVars)) //nolint:gosec // local count fits uint32 by IR design
			fn.LocalVars = append(fn.LocalVars, lv)
			locals[g.Variable] = idx
		}
		fn.Expressions[h].Kind = ir.ExprLocalVariable{Variable: ir.LocalVariableHandle(idx)}
	}
	if len(locals) == 0 {
		return nil
	}
	// Pointers into the variables are function-space pointers now.
	for h := range fn.ExpressionTypes {
		fn.ExpressionTypes[h] = functionSpacePointer(m, fn.ExpressionTypes[h])
	}
	return nil
}

// functionSpacePointer returns res with a private-space pointer type made a
// function-space one; any other resolution unchanged.
func functionSpacePointer(m *ir.Module, res ir.TypeResolution) ir.TypeResolution {
	inner := res.Value
	if res.Handle != nil && int(*res.Handle) < len(m.Types) {
		inner = m.Types[*res.Handle].Inner
	}
	switch p := inner.(type) {
	case ir.PointerType:
		if p.Space == ir.SpacePrivate {
			return ir.TypeResolution{Value: ir.PointerType{Base: p.Base, Space: ir.SpaceFunction}}
		}
	case ir.ValuePointerType:
		if p.Space == ir.SpacePrivate {
			return ir.TypeResolution{Value: ir.ValuePointerType{Size: p.Size, Scalar: p.Scalar, Space: ir.SpaceFunction}}
		}
	}
	return res
}

// privateInitializer copies the variable's initializer into fn's expression
// arena (a local's Init is a function expression) and returns its handle;
// ok is false without an initializer, which is the zero value as for any
// local.
func privateInitializer(m *ir.Module, fn *ir.Function, gv *ir.GlobalVariable) (h ir.ExpressionHandle, ok bool, err error) {
	switch {
	case gv.Init != nil:
		return appendFnExpr(fn, ir.ExprConstant{Constant: *gv.Init}, ir.TypeResolution{Handle: &gv.Type}), true, nil
	case gv.InitExpr != nil:
		h, err = copyGlobalExpr(m, fn, *gv.InitExpr)
		return h, err == nil, err
	default:
		return 0, false, nil
	}
}

// copyGlobalExpr copies a constant expression of Module.GlobalExpressions,
// with its operands, into fn. The kinds are the ones a folded WGSL
// initializer is made of; anything else is refused rather than guessed.
func copyGlobalExpr(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) (ir.ExpressionHandle, error) {
	if int(h) >= len(m.GlobalExpressions) {
		return 0, fmt.Errorf("initializer expression %d out of range", h)
	}
	switch k := m.GlobalExpressions[h].Kind.(type) {
	case ir.Literal:
		return appendFnExpr(fn, k, ir.TypeResolution{Value: literalScalar(k.Value)}), nil
	case ir.ExprZeroValue:
		return appendFnExpr(fn, k, ir.TypeResolution{Handle: &k.Type}), nil
	case ir.ExprConstant:
		ty := m.Constants[k.Constant].Type
		return appendFnExpr(fn, k, ir.TypeResolution{Handle: &ty}), nil
	case ir.ExprCompose:
		res := ir.TypeResolution{Handle: &k.Type}
		comps := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			var err error
			if comps[i], err = copyGlobalExpr(m, fn, c); err != nil {
				return 0, err
			}
		}
		return appendFnExpr(fn, ir.ExprCompose{Type: k.Type, Components: comps}, res), nil
	case ir.ExprSplat:
		v, err := copyGlobalExpr(m, fn, k.Value)
		if err != nil {
			return 0, err
		}
		scalar, ok := scalarOf(m, fn.ExpressionTypes[v])
		if !ok {
			return 0, fmt.Errorf("splat of a non-scalar initializer")
		}
		res := ir.TypeResolution{Value: ir.VectorType{Size: k.Size, Scalar: scalar}}
		return appendFnExpr(fn, ir.ExprSplat{Size: k.Size, Value: v}, res), nil
	default:
		return 0, unsupported.Errorf("initializer expression %T is not supported", k)
	}
}

func scalarOf(m *ir.Module, res ir.TypeResolution) (ir.ScalarType, bool) {
	inner := res.Value
	if res.Handle != nil {
		inner = m.Types[*res.Handle].Inner
	}
	s, ok := inner.(ir.ScalarType)
	return s, ok
}

// literalScalar is a literal's scalar type, as the typifier resolves it.
func literalScalar(v ir.LiteralValue) ir.ScalarType {
	switch v.(type) {
	case ir.LiteralBool:
		return ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
	case ir.LiteralI32:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	case ir.LiteralU32:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	case ir.LiteralI64:
		return ir.ScalarType{Kind: ir.ScalarSint, Width: 8}
	case ir.LiteralU64:
		return ir.ScalarType{Kind: ir.ScalarUint, Width: 8}
	case ir.LiteralF16:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}
	case ir.LiteralF64:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}
	default:
		return ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	}
}

func appendFnExpr(fn *ir.Function, kind ir.ExpressionKind, res ir.TypeResolution) ir.ExpressionHandle {
	h := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
	for len(fn.ExpressionTypes) < int(h) {
		fn.ExpressionTypes = append(fn.ExpressionTypes, ir.TypeResolution{})
	}
	fn.ExpressionTypes = append(fn.ExpressionTypes, res)
	return h
}
