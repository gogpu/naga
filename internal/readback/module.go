package readback

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/ir"
)

const requireF16 = "shader-f16"

// Module returns the IR a backend compiles for case c, as wgpu builds a
// compute pipeline from it: the WGSL frontend's module, validated with the
// capabilities the case requires, with its overrides processed for its entry
// point and pipeline constants. Processing for one entry point removes the
// globals it does not use, so the bindings left are the pipeline's.
func Module(c *Case) (*ir.Module, error) {
	ast, err := naga.Parse(c.WGSL)
	if err != nil {
		return nil, err
	}
	var caps ir.Capabilities
	if slices.Contains(c.Spec.Requires, requireF16) {
		caps |= naga.CapShaderFloat16
	}
	m, err := naga.LowerWithCapabilities(ast, c.WGSL, caps)
	if err != nil {
		return nil, err
	}
	return naga.ProcessOverrides(m, naga.OverrideOptions{
		Constants:  c.Spec.Constants,
		EntryPoint: &naga.EntryPointSelector{Stage: ir.StageCompute, Name: c.Spec.Entry},
	})
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
	ir.CompactUnused(m)

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
			errs = append(errs, fmt.Errorf("%s: group %d: format %d binds @group(0) only", g.Name, g.Binding.Group, FormatVersion))
			continue
		}
		used[g.Binding.Binding] = true
		if err := checkGlobal(m, g, json[g.Binding.Binding]); err != nil {
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
func checkGlobal(m *ir.Module, g *ir.GlobalVariable, b *Binding) error {
	var kind Kind
	switch {
	case g.Space == ir.SpaceUniform:
		kind = KindUniform
	case g.Space == ir.SpaceStorage && g.Access == ir.StorageRead:
		kind = KindStorageRead
	case g.Space == ir.SpaceStorage:
		kind = KindStorageRW
	default:
		return fmt.Errorf("not a buffer: format %d binds storage and uniform buffers only", FormatVersion)
	}
	if b == nil {
		return fmt.Errorf("no JSON binding for this %s buffer", kind)
	}
	if b.Kind != kind {
		return fmt.Errorf("kind %q, the shader declares %q", b.Kind, kind)
	}
	// A runtime-sized array counts one element, WebGPU's minimum binding size.
	if need, have := ir.TypeSize(m, g.Type), uint32(b.Size())*4; have < need { //nolint:gosec // Size is a JSON word count
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
