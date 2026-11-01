package irutil

import "github.com/gogpu/naga/ir"

// KeepUnused selects whether Compact keeps the functions, global variables
// and named overrides nothing uses, as Rust naga's compact::KeepUnused.
type KeepUnused bool

const (
	// KeepUnusedNo removes every function and global variable no entry point
	// reaches and every override nothing uses (Rust KeepUnused::No), as
	// pipeline-constant processing needs: an override no entry point uses
	// needs no value.
	KeepUnusedNo KeepUnused = false
	// KeepUnusedYes keeps every function, global variable and named override
	// (Rust KeepUnused::Yes), as after lowering.
	KeepUnusedYes KeepUnused = true
)

// compactConstants removes the constants, overrides and global expressions
// nothing uses and renumbers the handles of what is left, as Rust naga's
// compact does for Module.constants, Module.overrides and
// Module.global_expressions.
//
// Used by definition: every named constant whose type is not abstract (Rust
// keeps abstract-typed constants away from the validator), every named
// override when keep is KeepUnusedYes, the initializers of global variables,
// the workgroup sizes given by override-expressions,
// and the mesh-shader override expressions. Used through them: constants and
// overrides referred to by function expressions, by global variable
// initializers or by used global expressions, the initializers of used
// constants and overrides, and the operands of used global expressions. An
// override without a name is never used by definition.
//
// Run it after compactExpressions, so that a dead function expression does
// not keep a constant or an override alive.
func compactConstants(module *ir.Module, keep KeepUnused) {
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
	module    *ir.Module
	consts    []bool // constants used
	exprs     []bool // global expressions used
	overrides []bool // overrides used
}

func (t *constantTracer) override(o ir.OverrideHandle) {
	if int(o) >= len(t.overrides) || t.overrides[o] {
		return
	}
	t.overrides[o] = true
	t.exprPtr(t.module.Overrides[o].Init)
}

func (t *constantTracer) constant(c ir.ConstantHandle) {
	if int(c) >= len(t.consts) || t.consts[c] {
		return
	}
	t.consts[c] = true
	t.expr(t.module.Constants[c].Init)
}

func (t *constantTracer) expr(h ir.ExpressionHandle) {
	if int(h) >= len(t.exprs) || t.exprs[h] {
		return
	}
	t.exprs[h] = true
}

func (t *constantTracer) exprPtr(h *ir.ExpressionHandle) {
	if h != nil {
		t.expr(*h)
	}
}

func (t *constantTracer) markUsedByDefinition(keep KeepUnused) {
	m := t.module
	t.markNamedConstants()
	for i := range m.Functions {
		t.functionRefs(&m.Functions[i])
	}
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		t.functionRefs(&ep.Function)
		for _, size := range ep.WorkgroupSizeOverrides {
			t.exprPtr(size)
		}
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
	// An array sized by an override uses it. Every type counts here; Rust
	// traces types and expressions in tandem, so an override an unused type
	// keeps alive may stay where Rust removes it.
	for i := range m.Types {
		if at, ok := m.Types[i].Inner.(ir.ArrayType); ok && at.Size.Pending != nil {
			t.override(*at.Size.Pending)
		}
	}
	if keep == KeepUnusedYes {
		for i := range m.Overrides {
			if m.Overrides[i].Name != "" {
				t.override(ir.OverrideHandle(i))
			}
		}
	}
}

// markNamedConstants marks every named constant of a concrete type: Rust
// keeps abstract-typed constants away from the validator.
func (t *constantTracer) markNamedConstants() {
	m := t.module
	for i := range m.Constants {
		c := &m.Constants[i]
		abstract := c.IsAbstract
		if !abstract && int(c.Type) < len(m.Types) {
			abstract = isAbstractType(m.Types[c.Type].Inner, m.Types)
		}
		if c.Name != "" && c.Name != "_" && !abstract {
			t.constant(ir.ConstantHandle(i))
		}
	}
}

// functionRefs marks the constants and overrides f refers to.
func (t *constantTracer) functionRefs(f *ir.Function) {
	for _, e := range f.Expressions {
		switch k := e.Kind.(type) {
		case ir.ExprConstant:
			t.constant(k.Constant)
		case ir.ExprOverride:
			t.override(k.Override)
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
			case ir.ExprConstant:
				t.constant(k.Constant)
			case ir.ExprOverride:
				t.override(k.Override)
			default:
				markExprHandleRefs(k, t.exprs)
			}
		}
		for i := range m.Constants {
			if !t.consts[i] {
				continue
			}
			if cv, ok := m.Constants[i].Value.(ir.CompositeValue); ok {
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
	r := constantRemap{consts: constRemap, exprs: exprRemap, overrides: overrideRemap}

	t.applyGlobalExpressions(r, keptExprs)
	t.applyConstants(r, keptConsts)
	for i := range m.Functions {
		r.functionRefs(&m.Functions[i])
	}
	for i := range m.EntryPoints {
		r.entryPoint(&m.EntryPoints[i])
	}
	for i := range m.GlobalVariables {
		gv := &m.GlobalVariables[i]
		if gv.Init != nil {
			c := r.constant(*gv.Init)
			gv.Init = &c
		}
		gv.InitExpr = r.exprPtr(gv.InitExpr)
	}
	for i := range m.Types {
		if at, ok := m.Types[i].Inner.(ir.ArrayType); ok && at.Size.Pending != nil {
			o := r.override(*at.Size.Pending)
			at.Size.Pending = &o
			m.Types[i].Inner = at
		}
	}
	newOverrides := make([]ir.Override, 0, keptOverrides)
	for i, o := range m.Overrides {
		if !t.overrides[i] {
			continue
		}
		o.Init = r.exprPtr(o.Init)
		newOverrides = append(newOverrides, o)
	}
	m.Overrides = newOverrides
}

// applyGlobalExpressions keeps the used global expressions, renumbered.
func (t *constantTracer) applyGlobalExpressions(r constantRemap, kept int) {
	m := t.module
	exprTable := make([]ir.ExpressionHandle, len(r.exprs))
	for i, h := range r.exprs {
		exprTable[i] = ir.ExpressionHandle(h)
	}
	newExprs := make([]ir.Expression, 0, kept)
	for i, e := range m.GlobalExpressions {
		if !t.exprs[i] {
			continue
		}
		switch k := e.Kind.(type) {
		case ir.ExprConstant:
			e.Kind = ir.ExprConstant{Constant: r.constant(k.Constant)}
		case ir.ExprOverride:
			e.Kind = ir.ExprOverride{Override: r.override(k.Override)}
		default:
			e.Kind = RemapExpressionHandles(e.Kind, exprTable)
		}
		newExprs = append(newExprs, e)
	}
	m.GlobalExpressions = newExprs
}

// applyConstants keeps the used constants, renumbered.
func (t *constantTracer) applyConstants(r constantRemap, kept int) {
	m := t.module
	newConsts := make([]ir.Constant, 0, kept)
	for i, c := range m.Constants {
		if !t.consts[i] {
			continue
		}
		c.Init = r.expr(c.Init)
		if cv, ok := c.Value.(ir.CompositeValue); ok {
			comps := make([]ir.ConstantHandle, len(cv.Components))
			for j, comp := range cv.Components {
				comps[j] = r.constant(comp)
			}
			c.Value = ir.CompositeValue{Components: comps}
		}
		newConsts = append(newConsts, c)
	}
	m.Constants = newConsts
}

// constantRemap maps the constant, global-expression and override handles
// compaction keeps to their new values; a handle outside a table is kept.
type constantRemap struct {
	consts, exprs, overrides []uint32
}

func (r constantRemap) constant(c ir.ConstantHandle) ir.ConstantHandle {
	if int(c) < len(r.consts) {
		return ir.ConstantHandle(r.consts[c])
	}
	return c
}

func (r constantRemap) override(o ir.OverrideHandle) ir.OverrideHandle {
	if int(o) < len(r.overrides) {
		return ir.OverrideHandle(r.overrides[o])
	}
	return o
}

func (r constantRemap) expr(h ir.ExpressionHandle) ir.ExpressionHandle {
	if int(h) < len(r.exprs) {
		return ir.ExpressionHandle(r.exprs[h])
	}
	return h
}

func (r constantRemap) exprPtr(h *ir.ExpressionHandle) *ir.ExpressionHandle {
	if h == nil {
		return nil
	}
	v := r.expr(*h)
	return &v
}

// functionRefs renumbers the constants and overrides f refers to.
func (r constantRemap) functionRefs(f *ir.Function) {
	for i := range f.Expressions {
		switch k := f.Expressions[i].Kind.(type) {
		case ir.ExprConstant:
			f.Expressions[i].Kind = ir.ExprConstant{Constant: r.constant(k.Constant)}
		case ir.ExprOverride:
			f.Expressions[i].Kind = ir.ExprOverride{Override: r.override(k.Override)}
		}
	}
}

// entryPoint renumbers what ep refers to: its function's constants and
// overrides, and its workgroup-size and mesh-shader override expressions.
func (r constantRemap) entryPoint(ep *ir.EntryPoint) {
	r.functionRefs(&ep.Function)
	for d, size := range ep.WorkgroupSizeOverrides {
		ep.WorkgroupSizeOverrides[d] = r.exprPtr(size)
	}
	if ep.MeshInfo != nil {
		ep.MeshInfo.MaxVerticesOverride = r.exprPtr(ep.MeshInfo.MaxVerticesOverride)
		ep.MeshInfo.MaxPrimitivesOverride = r.exprPtr(ep.MeshInfo.MaxPrimitivesOverride)
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
