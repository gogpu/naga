package ir

// CompactConstants removes the constants and global expressions nothing uses
// and renumbers the handles of what is left, as Rust naga's
// compact(KeepUnused::Yes) does for Module.constants and
// Module.global_expressions.
//
// Used by definition: every named constant whose type is not abstract (Rust
// keeps abstract-typed constants away from the validator), the initializers
// of overrides and global variables, and the mesh-shader override
// expressions. Used through them: constants referred to by function
// expressions, by global variable initializers or by used global
// expressions, the initializers of used constants, and the operands of used
// global expressions.
//
// Run it after CompactExpressions, so that a dead function expression does
// not keep a constant alive.
func CompactConstants(module *Module) {
	nc, ng := len(module.Constants), len(module.GlobalExpressions)
	if nc == 0 && ng == 0 {
		return
	}
	t := constantTracer{
		module: module,
		consts: make([]bool, nc),
		exprs:  make([]bool, ng),
	}
	t.markUsedByDefinition()
	t.traceToFixpoint()
	t.apply()
}

type constantTracer struct {
	module *Module
	consts []bool // constants used
	exprs  []bool // global expressions used
}

func (t *constantTracer) constant(c ConstantHandle) {
	if int(c) >= len(t.consts) || t.consts[c] {
		return
	}
	t.consts[c] = true
	t.expr(t.module.Constants[c].Init)
}

func (t *constantTracer) expr(h ExpressionHandle) {
	if int(h) >= len(t.exprs) || t.exprs[h] {
		return
	}
	t.exprs[h] = true
}

func (t *constantTracer) exprPtr(h *ExpressionHandle) {
	if h != nil {
		t.expr(*h)
	}
}

func (t *constantTracer) markUsedByDefinition() {
	m := t.module
	for i := range m.Constants {
		c := &m.Constants[i]
		abstract := c.IsAbstract
		if !abstract && int(c.Type) < len(m.Types) {
			abstract = IsAbstractType(m.Types[c.Type].Inner, m.Types)
		}
		if c.Name != "" && c.Name != "_" && !abstract {
			t.constant(ConstantHandle(i))
		}
	}
	functionRefs := func(f *Function) {
		for _, e := range f.Expressions {
			if ec, ok := e.Kind.(ExprConstant); ok {
				t.constant(ec.Constant)
			}
		}
	}
	for i := range m.Functions {
		functionRefs(&m.Functions[i])
	}
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		functionRefs(&ep.Function)
		if ep.MeshInfo != nil {
			t.exprPtr(ep.MeshInfo.MaxVerticesOverride)
			t.exprPtr(ep.MeshInfo.MaxPrimitivesOverride)
		}
	}
	for i := range m.GlobalVariables {
		gv := &m.GlobalVariables[i]
		if gv.Init != nil {
			t.constant(*gv.Init)
		}
		t.exprPtr(gv.InitExpr)
	}
	for i := range m.Overrides {
		t.exprPtr(m.Overrides[i].Init)
	}
}

// traceToFixpoint marks what the used constants and global expressions
// refer to. Expressions refer to earlier ones, so one back-to-front pass
// normally suffices (Rust's single pass); a constant whose initializer comes
// after the expression that refers to it needs another pass.
func (t *constantTracer) traceToFixpoint() {
	m := t.module
	for {
		before := t.count()
		for i := len(m.GlobalExpressions) - 1; i >= 0; i-- {
			if !t.exprs[i] {
				continue
			}
			kind := m.GlobalExpressions[i].Kind
			if ec, ok := kind.(ExprConstant); ok {
				t.constant(ec.Constant)
				continue
			}
			markExprHandleRefs(kind, t.exprs)
		}
		for i := range m.Constants {
			if !t.consts[i] {
				continue
			}
			if cv, ok := m.Constants[i].Value.(CompositeValue); ok {
				for _, comp := range cv.Components {
					t.constant(comp)
				}
			}
		}
		if t.count() == before {
			return
		}
	}
}

func (t *constantTracer) count() int {
	n := 0
	for _, u := range t.consts {
		if u {
			n++
		}
	}
	for _, u := range t.exprs {
		if u {
			n++
		}
	}
	return n
}

func (t *constantTracer) apply() {
	m := t.module
	constRemap, keptConsts := buildRemap(t.consts)
	exprRemap, keptExprs := buildRemap(t.exprs)
	if keptConsts == len(t.consts) && keptExprs == len(t.exprs) {
		return
	}
	remapC := func(c ConstantHandle) ConstantHandle {
		if int(c) < len(constRemap) {
			return ConstantHandle(constRemap[c])
		}
		return c
	}
	remapE := func(h ExpressionHandle) ExpressionHandle {
		if int(h) < len(exprRemap) {
			return ExpressionHandle(exprRemap[h])
		}
		return h
	}
	remapEPtr := func(h *ExpressionHandle) *ExpressionHandle {
		if h == nil {
			return nil
		}
		r := remapE(*h)
		return &r
	}

	exprTable := make([]ExpressionHandle, len(exprRemap))
	for i, r := range exprRemap {
		exprTable[i] = ExpressionHandle(r)
	}
	newExprs := make([]Expression, 0, keptExprs)
	for i, e := range m.GlobalExpressions {
		if !t.exprs[i] {
			continue
		}
		if ec, ok := e.Kind.(ExprConstant); ok {
			e.Kind = ExprConstant{Constant: remapC(ec.Constant)}
		} else {
			e.Kind = remapExprHandles(e.Kind, exprTable)
		}
		newExprs = append(newExprs, e)
	}
	m.GlobalExpressions = newExprs

	newConsts := make([]Constant, 0, keptConsts)
	for i, c := range m.Constants {
		if !t.consts[i] {
			continue
		}
		c.Init = remapE(c.Init)
		if cv, ok := c.Value.(CompositeValue); ok {
			comps := make([]ConstantHandle, len(cv.Components))
			for j, comp := range cv.Components {
				comps[j] = remapC(comp)
			}
			c.Value = CompositeValue{Components: comps}
		}
		newConsts = append(newConsts, c)
	}
	m.Constants = newConsts

	functionRefs := func(f *Function) {
		for i := range f.Expressions {
			if ec, ok := f.Expressions[i].Kind.(ExprConstant); ok {
				f.Expressions[i].Kind = ExprConstant{Constant: remapC(ec.Constant)}
			}
		}
	}
	for i := range m.Functions {
		functionRefs(&m.Functions[i])
	}
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		functionRefs(&ep.Function)
		if ep.MeshInfo != nil {
			ep.MeshInfo.MaxVerticesOverride = remapEPtr(ep.MeshInfo.MaxVerticesOverride)
			ep.MeshInfo.MaxPrimitivesOverride = remapEPtr(ep.MeshInfo.MaxPrimitivesOverride)
		}
	}
	for i := range m.GlobalVariables {
		gv := &m.GlobalVariables[i]
		if gv.Init != nil {
			r := remapC(*gv.Init)
			gv.Init = &r
		}
		gv.InitExpr = remapEPtr(gv.InitExpr)
	}
	for i := range m.Overrides {
		m.Overrides[i].Init = remapEPtr(m.Overrides[i].Init)
	}
}

// buildRemap maps each kept index to its new index; removed indices map to
// the all-ones sentinel. It returns the number kept.
func buildRemap(keep []bool) ([]uint32, int) {
	remap := make([]uint32, len(keep))
	n := 0
	for i, k := range keep {
		if k {
			remap[i] = uint32(n)
			n++
		} else {
			remap[i] = ^uint32(0)
		}
	}
	return remap, n
}
