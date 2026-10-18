package ir

// KeepUnused selects whether compaction keeps named overrides nothing uses,
// as Rust naga's compact::KeepUnused.
type KeepUnused bool

const (
	// KeepUnusedNo removes every override nothing uses (Rust KeepUnused::No),
	// as pipeline-constant processing needs: an override no entry point uses
	// needs no value.
	KeepUnusedNo KeepUnused = false
	// KeepUnusedYes keeps every named override (Rust KeepUnused::Yes), as
	// after lowering.
	KeepUnusedYes KeepUnused = true
)

// CompactConstants removes the constants, overrides and global expressions
// nothing uses and renumbers the handles of what is left, as Rust naga's
// compact does for Module.constants, Module.overrides and
// Module.global_expressions.
//
// Used by definition: every named constant whose type is not abstract (Rust
// keeps abstract-typed constants away from the validator), every named
// override when keep is KeepUnusedYes, the initializers of global variables,
// and the mesh-shader override expressions. Used through them: constants and
// overrides referred to by function expressions, by global variable
// initializers or by used global expressions, the initializers of used
// constants and overrides, and the operands of used global expressions. An
// override without a name is never used by definition.
//
// Run it after CompactExpressions, so that a dead function expression does
// not keep a constant or an override alive.
func CompactConstants(module *Module, keep KeepUnused) {
	nc, ng, no := len(module.Constants), len(module.GlobalExpressions), len(module.Overrides)
	if nc == 0 && ng == 0 && no == 0 {
		return
	}
	t := constantTracer{
		module:    module,
		consts:    make([]bool, nc),
		exprs:     make([]bool, ng),
		overrides: make([]bool, no),
	}
	t.markUsedByDefinition(keep)
	t.traceToFixpoint()
	t.apply()
}

type constantTracer struct {
	module    *Module
	consts    []bool // constants used
	exprs     []bool // global expressions used
	overrides []bool // overrides used
}

func (t *constantTracer) override(o OverrideHandle) {
	if int(o) >= len(t.overrides) || t.overrides[o] {
		return
	}
	t.overrides[o] = true
	t.exprPtr(t.module.Overrides[o].Init)
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

func (t *constantTracer) markUsedByDefinition(keep KeepUnused) {
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
			switch k := e.Kind.(type) {
			case ExprConstant:
				t.constant(k.Constant)
			case ExprOverride:
				t.override(k.Override)
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
	if keep == KeepUnusedYes {
		for i := range m.Overrides {
			if m.Overrides[i].Name != "" {
				t.override(OverrideHandle(i))
			}
		}
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
			switch k := m.GlobalExpressions[i].Kind.(type) {
			case ExprConstant:
				t.constant(k.Constant)
			case ExprOverride:
				t.override(k.Override)
			default:
				markExprHandleRefs(k, t.exprs)
			}
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
	for _, u := range t.overrides {
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
	overrideRemap, keptOverrides := buildRemap(t.overrides)
	if keptConsts == len(t.consts) && keptExprs == len(t.exprs) && keptOverrides == len(t.overrides) {
		return
	}
	remapO := func(o OverrideHandle) OverrideHandle {
		if int(o) < len(overrideRemap) {
			return OverrideHandle(overrideRemap[o])
		}
		return o
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
		switch k := e.Kind.(type) {
		case ExprConstant:
			e.Kind = ExprConstant{Constant: remapC(k.Constant)}
		case ExprOverride:
			e.Kind = ExprOverride{Override: remapO(k.Override)}
		default:
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
			switch k := f.Expressions[i].Kind.(type) {
			case ExprConstant:
				f.Expressions[i].Kind = ExprConstant{Constant: remapC(k.Constant)}
			case ExprOverride:
				f.Expressions[i].Kind = ExprOverride{Override: remapO(k.Override)}
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
	newOverrides := make([]Override, 0, keptOverrides)
	for i, o := range m.Overrides {
		if !t.overrides[i] {
			continue
		}
		o.Init = remapEPtr(o.Init)
		newOverrides = append(newOverrides, o)
	}
	m.Overrides = newOverrides
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
