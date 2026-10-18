// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package naga_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
)

// TestProcessOverridesLetIsRuntime: an expression that refers to a let is not
// a const- or override-expression (WGSL: let declarations are run-time), so
// substituting an override's value must not make it one. Rust naga 30
// (process_function's fresh ExpressionKindTracker drops the front end's
// force_non_const, gfx-rs/wgpu#10646 case 1 and #9002) evaluates such an
// expression and rejects the valid shaders below with a domain or
// division-by-zero error; at run time they are NaN or a defined integer
// result.
func TestProcessOverridesLetIsRuntime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		src       string
		constants naga.PipelineConstants
		fun       ir.MathFunction // must stay a run-time call, if set
	}{
		{
			name: "sqrt of a negative let",
			src: `
override k: f32 = -1.0;
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main() {
    let a = min(k, 0.0);
    o[0] = sqrt(a);
}`,
			fun: ir.MathSqrt,
		},
		{
			name: "sqrt of a negative let, given constant",
			src: `
override k: f32 = 1.0;
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main() {
    let a = min(k, 0.0);
    o[0] = sqrt(a);
}`,
			constants: naga.PipelineConstants{"k": -4},
			fun:       ir.MathSqrt,
		},
		{
			name: "let chain into log",
			src: `
override k: f32 = 1.0;
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main() {
    let a = k * -1.0;
    let b = a - 2.0;
    o[0] = log(b);
}`,
			fun: ir.MathLog,
		},
		{
			name: "integer division by a zero let",
			src: `
override k: i32 = 3;
@group(0) @binding(0) var<storage, read_write> o: array<i32>;
@compute @workgroup_size(1) fn main() {
    let d = k - 3;
    o[0] = 10 / d;
}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := lowerSource(t, tc.src)
			out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: tc.constants})
			if err != nil {
				t.Fatalf("ProcessOverrides: %v", err)
			}
			if tc.fun != 0 && !hasMath(out, tc.fun) {
				t.Errorf("math function %d was folded; it uses a let and must stay a run-time call", tc.fun)
			}
			if _, err := naga.GenerateSPIRV(out, spirv.Options{Version: spirv.Version1_3}); err != nil {
				t.Errorf("SPIR-V: %v", err)
			}
		})
	}
}

// TestProcessOverridesOverrideExpressionErrors: without a let, the same
// operations are override-expressions, and an error in their evaluation is a
// pipeline-creation error (WGSL), as in Rust.
func TestProcessOverridesOverrideExpressionErrors(t *testing.T) {
	m := lowerSource(t, `
override k: f32 = -1.0;
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main() {
    o[0] = sqrt(min(k, 0.0));
}`)
	_, err := naga.ProcessOverrides(m, naga.OverrideOptions{})
	if err == nil || !strings.Contains(err.Error(), "sqrt") {
		t.Errorf("err = %v, want the sqrt domain error", err)
	}
}

func hasMath(m *ir.Module, fun ir.MathFunction) bool {
	for _, ep := range m.EntryPoints {
		for _, e := range ep.Function.Expressions {
			if mf, ok := e.Kind.(ir.ExprMath); ok && mf.Fun == fun {
				return true
			}
		}
	}
	return false
}

// TestProcessOverridesIndexIntoConstantComposite: a literal index into a
// constant composite in a function that uses an override. Rust naga 30 breaks
// the Emit ranges here ("Expression [1] can't be introduced - it's already in
// scope", gfx-rs/wgpu#10646 cases 2 and 3); the processed module must stay
// valid.
func TestProcessOverridesIndexIntoConstantComposite(t *testing.T) {
	for name, body := range map[string]string{
		"let array":     "let a = array(vec2f(1.0), vec2f(2.0));\n    o[0] = vec4f(a[0], k, k);",
		"inline vector": "o[0] = vec4f(vec4f(1.0, k, 2.0, 3.0)[0]);",
		"swizzle":       "let a = array(vec2f(1.0), vec2f(2.0));\n    o[0] = vec4f(a[1].yx, k, k);",
	} {
		t.Run(name, func(t *testing.T) {
			m := lowerSource(t, `
override k: f32 = 1.0;
@group(0) @binding(0) var<storage, read_write> o: array<vec4f>;
@compute @workgroup_size(1) fn main() {
    `+body+`
}`)
			out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"k": 3}})
			if err != nil {
				t.Fatalf("ProcessOverrides: %v", err)
			}
			if _, err := naga.GenerateSPIRV(out, spirv.Options{Version: spirv.Version1_3}); err != nil {
				t.Errorf("SPIR-V: %v", err)
			}
		})
	}
}
