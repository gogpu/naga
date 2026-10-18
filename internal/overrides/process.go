// Package overrides replaces pipeline-overridable constants by their values:
// a port of Rust naga's back::pipeline_constants::process_overrides on the
// shared constant evaluator (ADR-008).
package overrides

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/internal/registry"
	"github.com/gogpu/naga/ir"
)

// Selector selects one entry point by stage and name.
type Selector struct {
	Stage ir.ShaderStage
	Name  string
}

// Process returns module with every override expression replaced by a
// constant holding the override's pipeline value or default, and every
// expression that becomes constant evaluated. module is not modified.
//
// The result keeps the overrides arena, each initializer now a fully
// evaluated expression, as Rust does: override-sized arrays stay pending and
// are sized from it. It has no ExprOverride left. With ep the other entry
// points are removed and the selected one is EntryPoints[0]; only overrides
// an entry point uses need a value. module itself is returned when there is
// nothing to do (no overrides, and no selector or at most one entry point).
func Process(module *ir.Module, constants map[string]float64, ep *Selector) (*ir.Module, error) {
	if err := checkKeys(module, constants); err != nil {
		return nil, err
	}
	if ep != nil && findEntryPoint(module, ep) < 0 {
		return nil, fmt.Errorf("%w: %q (%s stage)", ErrEntryPointNotFound, ep.Name, stageName(ep.Stage))
	}
	if (ep == nil || len(module.EntryPoints) <= 1) && len(module.Overrides) == 0 {
		return module, nil
	}

	m := module.Clone()
	if ep != nil {
		m.EntryPoints = []ir.EntryPoint{m.EntryPoints[findEntryPoint(m, ep)]}
	}
	// Rust compact(KeepUnused::No): an override no entry point uses needs no
	// value.
	ir.CompactUnused(m)
	ir.CompactExpressions(m)
	ir.CompactConstants(m, ir.KeepUnusedNo)
	ir.CompactTypes(m)

	if len(m.Overrides) > 0 {
		if err := newProcessor(m, constants).run(); err != nil {
			return nil, err
		}
	}
	if err := revalidate(m); err != nil {
		return nil, err
	}
	return m, nil
}

// checkKeys reports the first key, in sorted order, that names no override:
// by its @id as a decimal string when it has one, otherwise by its name
// (WebGPU: compared as WGSL identifiers, exactly; Rust parses the key as a
// number, so it also takes "013" for @id(13)). A key may name an override no
// entry point uses.
func checkKeys(m *ir.Module, constants map[string]float64) error {
	if len(constants) == 0 {
		return nil
	}
	known := make(map[string]bool, len(m.Overrides))
	for i := range m.Overrides {
		if k, ok := key(&m.Overrides[i]); ok {
			known[k] = true
		}
	}
	keys := make([]string, 0, len(constants))
	for k := range constants {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !known[k] {
			return &KeyError{Key: k, Err: ErrNotFound}
		}
	}
	return nil
}

// key is the pipeline-overridable constant identifier string of o; ok is
// false for an override without name and id (one the frontend makes for an
// override-expression array size).
func key(o *ir.Override) (string, bool) {
	switch {
	case o.ID != nil:
		return strconv.FormatUint(uint64(*o.ID), 10), true
	case o.Name != "":
		return o.Name, true
	}
	return "", false
}

func findEntryPoint(m *ir.Module, ep *Selector) int {
	for i := range m.EntryPoints {
		if m.EntryPoints[i].Stage == ep.Stage && m.EntryPoints[i].Name == ep.Name {
			return i
		}
	}
	return -1
}

func stageName(s ir.ShaderStage) string {
	switch s {
	case ir.StageVertex:
		return "vertex"
	case ir.StageFragment:
		return "fragment"
	case ir.StageCompute:
		return "compute"
	case ir.StageTask:
		return "task"
	case ir.StageMesh:
		return "mesh"
	}
	return "stage " + strconv.Itoa(int(s))
}

type processor struct {
	m         *ir.Module
	constants map[string]float64
	types     *types
	eval      *consteval.Evaluator // over the rebuilt global expressions

	// constantOf[o] is the constant that replaces override o, plus one; 0 if
	// none yet.
	constantOf   []ir.ConstantHandle
	nextOverride int // overrides before it have been processed

	// adjusted[h] is the new handle of global expression h; done[h] reports
	// that h has been rebuilt.
	adjusted []ir.ExpressionHandle
	done     []bool
	// initAdjusted[c] reports that constant c's Init is a new handle.
	initAdjusted []bool
}

func newProcessor(m *ir.Module, constants map[string]float64) *processor {
	t := &types{m: m, reg: registry.NewTypeRegistryFrom(m.Types)}
	return &processor{
		m:            m,
		constants:    constants,
		types:        t,
		eval:         &consteval.Evaluator{Module: m, Arena: globalArena{m}, Types: t, Behavior: consteval.BehaviorConst},
		constantOf:   make([]ir.ConstantHandle, len(m.Overrides)),
		initAdjusted: make([]bool, len(m.Constants)),
	}
}

// run replaces the overrides of the module: global expressions first, then
// every function and entry point.
func (p *processor) run() error {
	m := p.m
	if err := p.globalExpressions(); err != nil {
		return err
	}
	for i := range m.Functions {
		if err := p.function(&m.Functions[i]); err != nil {
			return err
		}
	}
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		if err := p.function(&ep.Function); err != nil {
			return err
		}
		if err := p.workgroupSize(ep); err != nil {
			return err
		}
		if err := p.meshOutputs(ep); err != nil {
			return err
		}
	}
	return checkNoOverrideExpressions(m)
}

// workgroupSize resolves the workgroup sizes given by overrides (Rust
// process_workgroup_size_override): each must be a positive integer. The
// override handles are cleared, as Rust sets workgroup_size_overrides to None.
func (p *processor) workgroupSize(ep *ir.EntryPoint) error {
	for d, h := range ep.WorkgroupSizeOverrides {
		if h == nil {
			continue
		}
		n, ok := p.u32(*h)
		if !ok || n == 0 {
			return ErrNegativeWorkgroupSize
		}
		ep.Workgroup[d] = n
	}
	ep.WorkgroupSizeOverrides = [3]*ir.ExpressionHandle{}
	return nil
}

// globalExpressions rebuilds Module.GlobalExpressions with every expression
// evaluated, replacing each override by a new constant on the way (Rust
// process_overrides 185-270): overrides and expressions form a DAG in arena
// order, so an override's initializer is always rebuilt before the first
// expression that refers to the override.
func (p *processor) globalExpressions() error {
	m := p.m
	old := m.GlobalExpressions
	m.GlobalExpressions = make([]ir.Expression, 0, len(old))
	p.adjusted = make([]ir.ExpressionHandle, len(old))
	p.done = make([]bool, len(old))
	for i, e := range old {
		kind := e.Kind
		switch k := kind.(type) {
		case ir.ExprOverride:
			c, err := p.constantFor(k.Override)
			if err != nil {
				return err
			}
			kind = ir.ExprConstant{Constant: c}
		case ir.ExprConstant:
			// The evaluator sees through a constant to its initializer,
			// which must already be a new handle.
			if err := p.adjustConstantInit(k.Constant); err != nil {
				return err
			}
		}
		// Operands precede the expression in the arena, so they are rebuilt.
		kind = ir.RemapExpressionHandles(kind, p.adjusted[:i])
		h, err := p.eval.TryEvalAndAppend(kind)
		if err != nil {
			return fmt.Errorf("global expression %d: %w", i, err)
		}
		p.adjusted[i], p.done[i] = h, true
	}

	// Overrides no expression refers to (used by an array size, by a
	// workgroup size or by a mesh output limit) still get their constant.
	for p.nextOverride < len(m.Overrides) {
		if err := p.processOverride(p.nextOverride); err != nil {
			return err
		}
		p.nextOverride++
	}
	return p.adjustGlobalReferences()
}

// adjustGlobalReferences points everything outside the arena that refers to
// a global expression at the rebuilt arena: constant and global-variable
// initializers, override workgroup sizes and mesh output limits.
func (p *processor) adjustGlobalReferences() error {
	m := p.m
	for c := range p.initAdjusted {
		if err := p.adjustConstantInit(ir.ConstantHandle(c)); err != nil {
			return err
		}
	}
	refs := make([]**ir.ExpressionHandle, 0, len(m.GlobalVariables)+5*len(m.EntryPoints))
	for i := range m.GlobalVariables {
		refs = append(refs, &m.GlobalVariables[i].InitExpr)
	}
	for i := range m.EntryPoints {
		ep := &m.EntryPoints[i]
		for d := range ep.WorkgroupSizeOverrides {
			refs = append(refs, &ep.WorkgroupSizeOverrides[d])
		}
		if mi := ep.MeshInfo; mi != nil {
			refs = append(refs, &mi.MaxVerticesOverride, &mi.MaxPrimitivesOverride)
		}
	}
	for _, r := range refs {
		if err := p.adjustGlobalPtr(r); err != nil {
			return err
		}
	}
	return nil
}

// adjustGlobal returns the new handle of global expression h.
func (p *processor) adjustGlobal(h ir.ExpressionHandle) (ir.ExpressionHandle, error) {
	if int(h) >= len(p.done) || !p.done[h] {
		return 0, fmt.Errorf("pipeline constants: global expression %d is used before it is defined", h)
	}
	return p.adjusted[h], nil
}

func (p *processor) adjustGlobalPtr(h **ir.ExpressionHandle) error {
	if *h == nil {
		return nil
	}
	n, err := p.adjustGlobal(**h)
	if err != nil {
		return err
	}
	*h = &n
	return nil
}

func (p *processor) adjustConstantInit(c ir.ConstantHandle) error {
	if int(c) >= len(p.initAdjusted) || p.initAdjusted[c] {
		return nil
	}
	n, err := p.adjustGlobal(p.m.Constants[c].Init)
	if err != nil {
		return err
	}
	p.m.Constants[c].Init = n
	p.initAdjusted[c] = true
	return nil
}

// constantFor returns the constant replacing override o, processing the
// overrides up to o first (Rust's tandem walk).
func (p *processor) constantFor(o ir.OverrideHandle) (ir.ConstantHandle, error) {
	if int(o) >= len(p.m.Overrides) {
		return 0, fmt.Errorf("pipeline constants: unknown override %d", o)
	}
	for p.nextOverride <= int(o) {
		if err := p.processOverride(p.nextOverride); err != nil {
			return 0, err
		}
		p.nextOverride++
	}
	if c := p.constantOf[o]; c != 0 {
		return c - 1, nil
	}
	// An override without name and id is referred to by array sizes only;
	// should an expression refer to one, it gets an unnamed constant.
	ov := &p.m.Overrides[o]
	if ov.Init == nil {
		return 0, fmt.Errorf("pipeline constants: override %d has no name, id or initializer", o)
	}
	return p.newConstant(int(o), "", *ov.Init), nil
}

// processOverride gives override i its value (Rust process_override): the
// pipeline constant converted to the override's type, else its rebuilt
// initializer. An override without name and id (made for an array size) has
// no key: its initializer is only rebuilt — Rust panics on it here
// (pipeline_constants.rs:376, unreachable!) and handles it in its second loop.
func (p *processor) processOverride(i int) error {
	m := p.m
	o := &m.Overrides[i]
	k, named := key(o)
	if !named {
		return p.adjustGlobalPtr(&o.Init)
	}
	var init ir.ExpressionHandle
	var err error
	switch value, given := p.constants[k]; {
	case given:
		init, err = p.pipelineValue(k, value, m.Types[o.Ty].Inner)
	case o.Init != nil:
		init, err = p.adjustGlobal(*o.Init)
	default:
		err = &KeyError{Key: k, Err: ErrMissingValue}
	}
	if err != nil {
		return err
	}
	p.newConstant(i, o.Name, init)
	return nil
}

// pipelineValue appends the pipeline-constant value for key, converted to
// the override's type.
func (p *processor) pipelineValue(k string, value float64, ty ir.TypeInner) (ir.ExpressionHandle, error) {
	scalar, isScalar := ty.(ir.ScalarType)
	if !isScalar {
		return 0, fmt.Errorf("pipeline constants: override %q is not a scalar", k)
	}
	lit, err := literalOf(value, scalar)
	if err != nil {
		return 0, &KeyError{Key: k, Err: err}
	}
	h, err := p.eval.TryEvalAndAppend(ir.Literal{Value: lit})
	if err != nil {
		return 0, fmt.Errorf("pipeline constant %q: %w", k, err)
	}
	return h, nil
}

func (p *processor) newConstant(o int, name string, init ir.ExpressionHandle) ir.ConstantHandle {
	c := ir.ConstantHandle(arenaIndex(len(p.m.Constants)))
	p.m.Constants = append(p.m.Constants, ir.Constant{Name: name, Type: p.m.Overrides[o].Ty, Init: init})
	p.initAdjusted = append(p.initAdjusted, true)
	p.constantOf[o] = c + 1
	p.m.Overrides[o].Init = &init
	return c
}

// function rebuilds fn's expression arena, replacing override expressions by
// their constants and evaluating every expression whose operands became
// constant (Rust process_function 420-484).
func (p *processor) function(fn *ir.Function) error {
	old := fn.Expressions
	fn.Expressions = make([]ir.Expression, 0, len(old))
	adjusted := make([]ir.ExpressionHandle, len(old))
	ev := &consteval.Evaluator{Module: p.m, Arena: fnArena{fn}, Types: p.types, Behavior: consteval.BehaviorRuntime}
	for i, e := range old {
		kind := e.Kind
		if eo, ok := kind.(ir.ExprOverride); ok {
			c, err := p.constantFor(eo.Override)
			if err != nil {
				return err
			}
			kind = ir.ExprConstant{Constant: c}
		}
		kind = ir.RemapExpressionHandles(kind, adjusted[:i])
		h, err := ev.TryEvalAndAppend(kind)
		if err != nil {
			return fmt.Errorf("function %q, expression %d: %w", fn.Name, i, err)
		}
		adjusted[i] = h
	}

	ir.RemapStatementExpressionHandles(fn.Body, adjusted)
	fn.Body = filterEmits(fn.Body, fn.Expressions)
	for i := range fn.LocalVars {
		if init := fn.LocalVars[i].Init; init != nil {
			n := adjusted[*init]
			fn.LocalVars[i].Init = &n
		}
	}
	if fn.NamedExpressions != nil {
		named := make(map[ir.ExpressionHandle]string, len(fn.NamedExpressions))
		for h, name := range fn.NamedExpressions {
			named[adjusted[h]] = name
		}
		fn.NamedExpressions = named
	}
	fn.ExpressionTypes = make([]ir.TypeResolution, len(fn.Expressions))
	for i := range fn.Expressions {
		res, err := ir.ResolveExpressionType(p.m, fn, ir.ExpressionHandle(i))
		if err != nil {
			return fmt.Errorf("function %q, expression %d: %w", fn.Name, i, err)
		}
		fn.ExpressionTypes[i] = res
	}
	return nil
}

// meshOutputs resolves mesh output limits given by overrides (Rust
// process_mesh_shader_overrides).
func (p *processor) meshOutputs(ep *ir.EntryPoint) error {
	mi := ep.MeshInfo
	if mi == nil {
		return nil
	}
	for _, f := range []struct {
		h   *ir.ExpressionHandle
		out *uint32
	}{{mi.MaxVerticesOverride, &mi.MaxVertices}, {mi.MaxPrimitivesOverride, &mi.MaxPrimitives}} {
		if f.h == nil {
			continue
		}
		n, ok := p.u32(*f.h)
		if !ok {
			return ErrNegativeMeshOutputMax
		}
		*f.out = n
	}
	return nil
}

// u32 is the value of the rebuilt global expression h as a non-negative
// integer (Rust get_const_val).
func (p *processor) u32(h ir.ExpressionHandle) (uint32, bool) {
	lit, ok := p.eval.Literal(h)
	if !ok {
		return 0, false
	}
	switch v := lit.(type) {
	case ir.LiteralU32:
		return uint32(v), true
	case ir.LiteralI32:
		if v < 0 {
			return 0, false
		}
		return uint32(v), true
	}
	return 0, false
}

// filterEmits splits every Emit so that it covers no expression that needs
// no Emit (Rust filter_emits_in_block): evaluation turns override uses and
// what depends on them into constants.
func filterEmits(block ir.Block, exprs []ir.Expression) ir.Block {
	out := make(ir.Block, 0, len(block))
	for _, stmt := range block {
		switch s := stmt.Kind.(type) {
		case ir.StmtEmit:
			start, open := ir.ExpressionHandle(0), false
			for h := s.Range.Start; h < s.Range.End; h++ {
				if int(h) < len(exprs) && ir.NeedsPreEmit(exprs[h].Kind) {
					if open {
						out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: start, End: h}}})
						open = false
					}
					continue
				}
				if !open {
					start, open = h, true
				}
			}
			if open {
				out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: start, End: s.Range.End}}})
			}
			continue
		case ir.StmtBlock:
			s.Block = filterEmits(s.Block, exprs)
			stmt.Kind = s
		case ir.StmtIf:
			s.Accept = filterEmits(s.Accept, exprs)
			s.Reject = filterEmits(s.Reject, exprs)
			stmt.Kind = s
		case ir.StmtSwitch:
			for i := range s.Cases {
				s.Cases[i].Body = filterEmits(s.Cases[i].Body, exprs)
			}
			stmt.Kind = s
		case ir.StmtLoop:
			s.Body = filterEmits(s.Body, exprs)
			s.Continuing = filterEmits(s.Continuing, exprs)
			stmt.Kind = s
		}
		out = append(out, stmt)
	}
	return out
}

// checkNoOverrideExpressions: the processed module must not refer to an
// override from any expression (Rust validate_resolved_overrides).
func checkNoOverrideExpressions(m *ir.Module) error {
	check := func(where string, exprs []ir.Expression) error {
		for i, e := range exprs {
			if _, ok := e.Kind.(ir.ExprOverride); ok {
				return fmt.Errorf("pipeline constants: %s expression %d still refers to an override", where, i)
			}
		}
		return nil
	}
	if err := check("global", m.GlobalExpressions); err != nil {
		return err
	}
	for i := range m.Functions {
		if err := check("function "+m.Functions[i].Name, m.Functions[i].Expressions); err != nil {
			return err
		}
	}
	for i := range m.EntryPoints {
		if err := check("entry point "+m.EntryPoints[i].Name, m.EntryPoints[i].Function.Expressions); err != nil {
			return err
		}
	}
	return nil
}

// revalidate validates the processed module (Rust revalidate with all
// capabilities).
func revalidate(m *ir.Module) error {
	errs, err := ir.Validate(m)
	if err != nil {
		return fmt.Errorf("pipeline constants: validating the processed module: %w", err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("pipeline constants: the processed module is invalid: %s", errs[0].Message)
	}
	return nil
}

// globalArena is Module.GlobalExpressions as the evaluator's arena.
type globalArena struct{ m *ir.Module }

func (a globalArena) Expression(h ir.ExpressionHandle) ir.ExpressionKind {
	return a.m.GlobalExpressions[h].Kind
}

func (a globalArena) Append(kind ir.ExpressionKind) ir.ExpressionHandle {
	h := ir.ExpressionHandle(arenaIndex(len(a.m.GlobalExpressions)))
	a.m.GlobalExpressions = append(a.m.GlobalExpressions, ir.Expression{Kind: kind})
	return h
}

func (globalArena) IsGlobal() bool { return true }

// fnArena is a function's expressions as the evaluator's arena. Emit ranges
// are fixed afterwards by filterEmits, as Rust uses a dummy emitter here.
type fnArena struct{ fn *ir.Function }

func (a fnArena) Expression(h ir.ExpressionHandle) ir.ExpressionKind {
	return a.fn.Expressions[h].Kind
}

func (a fnArena) Append(kind ir.ExpressionKind) ir.ExpressionHandle {
	h := ir.ExpressionHandle(arenaIndex(len(a.fn.Expressions)))
	a.fn.Expressions = append(a.fn.Expressions, ir.Expression{Kind: kind})
	return h
}

func (fnArena) IsGlobal() bool { return false }

// types registers the evaluator's result types in the module, deduplicated
// against the module's types.
type types struct {
	m   *ir.Module
	reg *registry.TypeRegistry
}

func (t *types) Type(inner ir.TypeInner) ir.TypeHandle {
	h := t.reg.GetOrCreate("", inner)
	t.m.Types = t.reg.GetTypes()
	return h
}

// Array registers the array of base with the given size; the stride is
// base's size rounded up to its alignment, as Rust's layouter computes it.
func (t *types) Array(base ir.TypeHandle, size ir.ArraySize) ir.TypeHandle {
	align := ir.TypeAlignment(t.m, base)
	stride := (ir.TypeSize(t.m, base) + align - 1) / align * align
	return t.Type(ir.ArrayType{Base: base, Size: size, Stride: stride})
}

// arenaIndex is n as an arena index. IR handles are 32-bit; an arena never
// holds 2^32 entries.
func arenaIndex(n int) uint32 {
	if n < 0 || n > math.MaxUint32 {
		panic("pipeline constants: arena index out of the 32-bit handle range")
	}
	return uint32(n)
}
