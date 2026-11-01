// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"

	"github.com/gogpu/naga/internal/typifier"
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
	var globals globalExprTypes
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
			init, hasInit, err := privateInitializer(m, fn, gv, &globals)
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
	return nil
}

// globalExprTypes types the module's global expressions on first use.
type globalExprTypes struct {
	types *typifier.Typifier
}

// get returns the type of global expression h.
func (g *globalExprTypes) get(m *ir.Module, h ir.ExpressionHandle) (ir.TypeResolution, error) {
	if g.types == nil {
		t, err := typifier.Function(m, &ir.Function{Expressions: m.GlobalExpressions})
		if err != nil {
			return ir.TypeResolution{}, fmt.Errorf("global expressions: %w", err)
		}
		g.types = t
	}
	return g.types.Get(h), nil
}

// privateInitializer copies the variable's initializer into fn's expression
// arena (a local's Init is a function expression) and returns its handle;
// ok is false without an initializer, which is the zero value as for any
// local.
func privateInitializer(m *ir.Module, fn *ir.Function, gv *ir.GlobalVariable, globals *globalExprTypes) (h ir.ExpressionHandle, ok bool, err error) {
	switch {
	case gv.Init != nil:
		return appendFnExpr(fn, ir.ExprConstant{Constant: *gv.Init}), true, nil
	case gv.InitExpr != nil:
		h, err = copyGlobalExpr(m, fn, *gv.InitExpr, globals)
		return h, err == nil, err
	default:
		return 0, false, nil
	}
}

// copyGlobalExpr copies a constant expression of Module.GlobalExpressions,
// with its operands, into fn. The kinds are the ones a folded WGSL
// initializer is made of; anything else is refused rather than guessed.
func copyGlobalExpr(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle, globals *globalExprTypes) (ir.ExpressionHandle, error) {
	if int(h) >= len(m.GlobalExpressions) {
		return 0, fmt.Errorf("initializer expression %d out of range", h)
	}
	switch k := m.GlobalExpressions[h].Kind.(type) {
	case ir.Literal, ir.ExprZeroValue, ir.ExprConstant:
		return appendFnExpr(fn, k), nil
	case ir.ExprCompose:
		comps := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			var err error
			if comps[i], err = copyGlobalExpr(m, fn, c, globals); err != nil {
				return 0, err
			}
		}
		return appendFnExpr(fn, ir.ExprCompose{Type: k.Type, Components: comps}), nil
	case ir.ExprSplat:
		res, err := globals.get(m, k.Value)
		if err != nil {
			return 0, err
		}
		if _, ok := scalarOf(m, res); !ok {
			return 0, fmt.Errorf("splat of a non-scalar initializer")
		}
		v, err := copyGlobalExpr(m, fn, k.Value, globals)
		if err != nil {
			return 0, err
		}
		return appendFnExpr(fn, ir.ExprSplat{Size: k.Size, Value: v}), nil
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

func appendFnExpr(fn *ir.Function, kind ir.ExpressionKind) ir.ExpressionHandle {
	h := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
	return h
}
