package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
	"github.com/gogpu/naga/wgsl"
)

// The SPIR-V backend had no case for ir.ExprRelational, so `any()` and `all()`
// over a vector of bools — the natural way to write a rect rejection in a
// material — parsed, validated, lowered, and then died at pipeline creation with
// "unsupported expression kind: ir.ExprRelational". Every other backend lowered
// it; only the Vulkan path failed, and only once a pipeline was actually built.
//
// These tests pin the whole family: any/all over vec2/vec3/vec4<bool>, the
// scalar-bool form that has no vector to reduce, and isnan/isinf which keep the
// vector shape instead of collapsing it.

// relationalRectShader is the shape from the issue: a frame-rect rejection in a
// fragment shader, written with any() instead of spelled out component-wise.
const relationalRectShader = `
struct Frame { rect: vec4<f32> }
@group(0) @binding(0) var<uniform> frame: Frame;

@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let uv = p.xy;
    if (any(uv < frame.rect.xy) || any(uv > frame.rect.zw)) {
        return vec4<f32>(0.0, 0.0, 0.0, 0.0);
    }
    return vec4<f32>(1.0, 1.0, 1.0, 1.0);
}
`

func TestRelationalRectRejectionCompilesToSPIRV(t *testing.T) {
	spirvBytes := compileRelationalShader(t, "rect", relationalRectShader)
	assertHasOpcode(t, spirvBytes, spvenums.OpAny, "any() over a vec2<bool>")
}

// TestRelationalAnyAllOverVectors covers both reductions at every vector width.
func TestRelationalAnyAllOverVectors(t *testing.T) {
	cases := []struct {
		name    string
		fun     string
		size    string
		swizzle string
		opcode  spvenums.Op
	}{
		{"any/vec2", "any", "vec2<f32>", ".xy", spvenums.OpAny},
		{"any/vec3", "any", "vec3<f32>", ".xyz", spvenums.OpAny},
		{"any/vec4", "any", "vec4<f32>", "", spvenums.OpAny},
		{"all/vec2", "all", "vec2<f32>", ".xy", spvenums.OpAll},
		{"all/vec3", "all", "vec3<f32>", ".xyz", spvenums.OpAll},
		{"all/vec4", "all", "vec4<f32>", "", spvenums.OpAll},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `
@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let v = p` + tc.swizzle + `;
    let hit = ` + tc.fun + `(v > ` + tc.size + `(0.5));
    return select(vec4<f32>(0.0), vec4<f32>(1.0), hit);
}
`
			spirvBytes := compileRelationalShader(t, tc.name, src)
			assertHasOpcode(t, spirvBytes, tc.opcode, tc.fun+" over "+tc.size)
		})
	}
}

// TestRelationalAnyAllOverScalarBool covers the degenerate form. any(bool) and
// all(bool) are no-ops, so, as in Rust naga, the WGSL lowerer returns the
// argument and no Relational reaches the IR — whatever the argument is, not only
// a literal. Backends then never see a scalar any/all; SPIR-V's OpAny/OpAll
// would reject one, and GLSL has no any(bool) overload.
func TestRelationalAnyAllOverScalarBool(t *testing.T) {
	args := []struct {
		name string
		decl string
		arg  string
	}{
		{"comparison", "", "p.x > 0.5"},
		{"let", "let b = p.x > 0.5;", "b"},
		{"literal", "", "true"},
	}
	for _, fun := range []string{"any", "all"} {
		for _, a := range args {
			t.Run(fun+"/"+a.name, func(t *testing.T) {
				src := `
@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    ` + a.decl + `
    let hit = ` + fun + `(` + a.arg + `);
    return select(vec4<f32>(0.0), vec4<f32>(1.0), hit);
}
`
				assertNoRelationalInIR(t, src)
				spirvBytes := compileRelationalShader(t, fun+"/"+a.name, src)
				assertLacksOpcode(t, spirvBytes, spvenums.OpAny, fun+" over a scalar bool")
				assertLacksOpcode(t, spirvBytes, spvenums.OpAll, fun+" over a scalar bool")
			})
		}
	}
}

// assertNoRelationalInIR lowers source and fails if any function or entry point
// holds an ir.ExprRelational.
func assertNoRelationalInIR(t *testing.T, source string) {
	t.Helper()
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}
	functions := make([]*ir.Function, 0, len(module.Functions)+len(module.EntryPoints))
	for i := range module.Functions {
		functions = append(functions, &module.Functions[i])
	}
	for i := range module.EntryPoints {
		functions = append(functions, &module.EntryPoints[i].Function)
	}
	for _, fn := range functions {
		for h, expr := range fn.Expressions {
			if rel, ok := expr.Kind.(ir.ExprRelational); ok {
				t.Fatalf("expression %d is %#v; a scalar any/all should not reach the IR", h, rel)
			}
		}
	}
}

// TestRelationalIsNanIsInf covers the two that keep the argument's shape: over a
// vector they produce a vec<bool>, not a single bool.
func TestRelationalIsNanIsInf(t *testing.T) {
	cases := []struct {
		name   string
		fun    string
		arg    string
		opcode spvenums.Op
	}{
		{"isnan/scalar", "isnan", "p.x", spvenums.OpIsNan},
		{"isnan/vector", "isnan", "p.xy", spvenums.OpIsNan},
		{"isinf/scalar", "isinf", "p.x", spvenums.OpIsInf},
		{"isinf/vector", "isinf", "p.xy", spvenums.OpIsInf},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `
@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    let bad = any(vec2<bool>(` + tc.fun + `(` + tc.arg + `)));
    return select(vec4<f32>(0.0), vec4<f32>(1.0), bad);
}
`
			spirvBytes := compileRelationalShader(t, tc.name, src)
			assertHasOpcode(t, spirvBytes, tc.opcode, tc.fun+"("+tc.arg+")")
		})
	}
}

// compileRelationalShader compiles a shader and runs spirv-val over the result.
// The bug this file pins never showed up in parsing or validation — it surfaced
// only once something actually consumed the SPIR-V — so these tests check the
// binary a driver would be handed, not just that compilation returned.
func compileRelationalShader(t *testing.T, label, source string) []byte {
	t.Helper()
	spirvBytes := compileWGSLToSPIRV(t, label, source)
	validateWithVulkanSDK(t, spirvBytes)
	return spirvBytes
}

func assertHasOpcode(t *testing.T, spirvBytes []byte, op spvenums.Op, what string) {
	t.Helper()
	if !hasOpcode(spirvBytes, op) {
		t.Fatalf("%s emitted no %v\n%s", what, op, disassembleSPIRV(spirvBytes))
	}
}

func assertLacksOpcode(t *testing.T, spirvBytes []byte, op spvenums.Op, what string) {
	t.Helper()
	if hasOpcode(spirvBytes, op) {
		t.Fatalf("%s emitted %v, which requires a vector operand\n%s",
			what, op, disassembleSPIRV(spirvBytes))
	}
}
