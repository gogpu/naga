package lower

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// globalVarInit returns the flattened module-scope initializer of a private
// global variable, evaluated by the shared const evaluator.
func globalVarInit(t *testing.T, m *ir.Module, name string) []string {
	t.Helper()
	for i := range m.GlobalVariables {
		gv := &m.GlobalVariables[i]
		if gv.Name != name {
			continue
		}
		if gv.InitExpr == nil {
			t.Fatalf("global variable %q has no initializer", name)
		}
		return flattenConstGE(t, m, *gv.InitExpr)
	}
	t.Fatalf("global variable %q not in the module", name)
	return nil
}

// TestGlobalConstructorShapes covers BUG-WGSL-GE-SHAPES-001: module-scope
// constructor initializers go through the shared evaluator and its shapes, as
// in Rust naga's front/wgsl/lower/construction.rs. A single-argument vector
// constructor is a splat, a zero-argument vector component is a zero value of
// that vector type, and a const identifier operand contributes its value.
// Before the evaluator, three of these became silent zeros in every backend
// and one was a scalar-typed zero value.
func TestGlobalConstructorShapes(t *testing.T) {
	src := `
const A: vec2<f32> = vec2<f32>(0.25, 0.75);
const D: vec4<f32> = vec4<f32>(vec3<f32>(0.625), 0.875);
var<private> E: vec3<f32> = vec3<f32>(A, 0.1875);
var<private> F: vec4<f32> = vec4<f32>(vec2<f32>(), 0.3125, 0.4375);
var<private> G: vec4<f32> = vec4<f32>(vec3<f32>(0.5625), 0.6875);

@compute @workgroup_size(1)
fn main() {
    E = E + vec3<f32>(D.x, D.y, F.z + G.w);
}
`
	m, err := compileWGSL(t, src)
	if err != nil {
		t.Fatalf("lowering failed: %v", err)
	}

	d := constByName(t, m, "D")
	if got, want := flattenConstGE(t, m, d.Init), f32s(0.625, 0.625, 0.625, 0.875); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("D = %v, want %v (a single-argument vec3 constructor is a splat)", got, want)
	}
	if got, want := globalVarInit(t, m, "E"), f32s(0.25, 0.75, 0.1875); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("E = %v, want %v (a const identifier operand contributes its value)", got, want)
	}
	if got, want := globalVarInit(t, m, "F"), f32s(0, 0, 0.3125, 0.4375); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("F = %v, want %v (vec2<f32>() is a zero value of the vector type)", got, want)
	}
	if got, want := globalVarInit(t, m, "G"), f32s(0.5625, 0.5625, 0.5625, 0.6875); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("G = %v, want %v (a single-argument vec3 constructor is a splat)", got, want)
	}

	if _, err := valid.Validate(m, valid.Options{Capabilities: valid.CapAll}); err != nil {
		t.Errorf("validation: %v", err)
	}
}
