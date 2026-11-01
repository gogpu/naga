package readback

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

const requireF16 = "shader-f16"

// Module returns the IR a backend compiles for case c, as wgpu builds a
// compute pipeline from it: the WGSL frontend's module, validated with the
// capabilities the case requires, with its overrides processed for its entry
// point and pipeline constants. Processing for one entry point removes the
// globals it does not use, so the bindings left are the pipeline's.
func Module(c *Case) (*ir.Module, error) {
	m, _, err := wgsl.Parse(c.WGSL, wgsl.ParseOptions{})
	if err != nil {
		return nil, err
	}
	var caps valid.Capabilities
	if slices.Contains(c.Spec.Requires, requireF16) {
		caps |= valid.CapShaderFloat16
	}
	info, err := valid.Validate(m, valid.Options{Capabilities: caps})
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	m, _, err = naga.ProcessOverrides(m, info, naga.OverrideOptions{
		Constants:  c.Spec.Constants,
		EntryPoint: &backend.EntryPointSelector{Stage: ir.StageCompute, Name: c.Spec.Entry},
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// CheckModule checks that spec describes the shader m, a module from Module:
// the entry point is a compute one, every buffer it binds has a JSON binding
// of the same kind and at least the type's size, and the other way round, and
// the case requires shader-f16 exactly when the shader uses f16.
func CheckModule(spec *Spec, m *ir.Module) error {
	if len(m.EntryPoints) != 1 || m.EntryPoints[0].Name != spec.Entry || m.EntryPoints[0].Stage != ir.StageCompute {
		return fmt.Errorf("entry point %s: no compute entry point of that name", spec.Entry)
	}
	// The pipeline binds only what the entry point uses (WebGPU's default
	// layout). ProcessOverrides leaves an unused global in place when it has
	// nothing to do, so prune a copy: m stays the module the backends compile.
	m = m.Clone()
	irutil.CompactUnused(m)
	var layouter ir.Layouter
	if err := layouter.Update(m); err != nil {
		return err
	}

	var errs []error
	json := map[uint32]*Binding{}
	for i := range spec.Bindings {
		json[spec.Bindings[i].Binding] = &spec.Bindings[i]
	}
	used := map[uint32]bool{}
	for i := range m.GlobalVariables {
		g := &m.GlobalVariables[i]
		if g.Binding == nil {
			continue
		}
		if g.Binding.Group != 0 {
			errs = append(errs, fmt.Errorf("%s: group %d: the corpus binds @group(0) only", g.Name, g.Binding.Group))
			continue
		}
		used[g.Binding.Binding] = true
		if err := checkGlobal(&layouter, g, json[g.Binding.Binding]); err != nil {
			errs = append(errs, fmt.Errorf("binding %d (%s): %w", g.Binding.Binding, g.Name, err))
		}
	}
	var extra []uint32
	for n := range json {
		if !used[n] {
			extra = append(extra, n)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i] < extra[j] })
	for _, n := range extra {
		errs = append(errs, fmt.Errorf("binding %d: no global of entry point %s binds it", n, spec.Entry))
	}

	usesF16 := moduleUsesF16(m)
	switch requires := slices.Contains(spec.Requires, requireF16); {
	case requires && !usesF16:
		errs = append(errs, fmt.Errorf("requires %s, the shader uses no f16", requireF16))
	case usesF16 && !requires:
		// Module validated without the capability, which should have failed;
		// the validator checks only arena types (card P2-IR-021).
		errs = append(errs, fmt.Errorf("the shader uses f16 and does not require %s", requireF16))
	}
	return errors.Join(errs...)
}

// checkGlobal checks the JSON binding b of the bound global g.
func checkGlobal(layouter *ir.Layouter, g *ir.GlobalVariable, b *Binding) error {
	var kind Kind
	switch {
	case g.Space == ir.SpaceUniform:
		kind = KindUniform
	case g.Space == ir.SpaceStorage && !g.Access.Contains(ir.StorageAccessStore):
		kind = KindStorageRead
	case g.Space == ir.SpaceStorage:
		kind = KindStorageRW
	default:
		return errors.New("not a buffer: the corpus binds storage and uniform buffers only")
	}
	if b == nil {
		return fmt.Errorf("no JSON binding for this %s buffer", kind)
	}
	if b.Kind != kind {
		return fmt.Errorf("kind %q, the shader declares %q", b.Kind, kind)
	}
	// A runtime-sized array counts one element, WebGPU's minimum binding size.
	if need, have := layouter.Layout(g.Type).Size, uint32(b.Size())*4; have < need { //nolint:gosec // Size is a JSON word count
		return fmt.Errorf("%d bytes, the type needs %d", have, need)
	}
	return nil
}

// moduleUsesF16 reports whether m uses f16 where Rust naga's validator checks
// the SHADER_FLOAT16 capability (check_width): a type of the arena, a literal
// (validate_literal) or a conversion (Expression::As). The arena alone is not
// enough: the type of `f16(x) + 0.5h` is never added to it.
func moduleUsesF16(m *ir.Module) bool {
	f16 := func(s ir.ScalarType) bool { return s.Kind == ir.ScalarFloat && s.Width == 2 }
	if exprsUseF16(m.GlobalExpressions) {
		return true
	}
	for i := range m.Functions {
		if exprsUseF16(m.Functions[i].Expressions) {
			return true
		}
	}
	for i := range m.EntryPoints {
		if exprsUseF16(m.EntryPoints[i].Function.Expressions) {
			return true
		}
	}
	for i := range m.Types {
		switch t := m.Types[i].Inner.(type) {
		case ir.ScalarType:
			if f16(t) {
				return true
			}
		case ir.VectorType:
			if f16(t.Scalar) {
				return true
			}
		case ir.MatrixType:
			if f16(t.Scalar) {
				return true
			}
		}
	}
	return false
}

// exprsUseF16 reports whether an expression arena has an f16 literal or a
// conversion to f16.
func exprsUseF16(exprs []ir.Expression) bool {
	for i := range exprs {
		switch e := exprs[i].Kind.(type) {
		case ir.Literal:
			if _, ok := e.Value.(ir.LiteralF16); ok {
				return true
			}
		case ir.ExprAs:
			if e.Kind == ir.ScalarFloat && e.Convert != nil && *e.Convert == 2 {
				return true
			}
		}
	}
	return false
}
