package valid_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestSelectTypes: select takes a bool condition with scalar or vector
// values, or a vecN<bool> condition with vecN values, and both values of one
// type, as the WGSL spec's select overloads and Rust naga's validate_expression
// for Expression::Select (ExpressionError::SelectConditionNotABool,
// SelectValuesTypeMismatch). The backends disagreed on a float-vector
// condition: SPIR-V compared it with zero, HLSL wrote a ternary DXC rejects,
// GLSL a mix() and DXIL invalid bitcode (P1-WGSL-095).
func TestSelectTypes(t *testing.T) {
	const prefix = `@group(0) @binding(0) var<storage, read_write> out: vec3f;
@compute @workgroup_size(1) fn main() { `
	for _, tt := range []struct{ name, body, want string }{
		{
			// gg's blend_soft_light shape: step() as the condition.
			"float vector condition",
			"out = select(vec3f(1.0), vec3f(2.0), step(vec3f(0.5), out));",
			"Expected selection condition to be a boolean value, got Vector { size: Tri, scalar: Scalar { kind: Float, width: 4 } }",
		},
		{
			"u32 condition",
			"out = select(vec3f(1.0), vec3f(2.0), u32(out.x));",
			"Expected selection condition to be a boolean value, got Scalar(Scalar { kind: Uint, width: 4 })",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectRejected(t, parse(t, prefix+tt.body+" }"), valid.Options{}, tt.want)
		})
	}
	expectValid(t, parse(t, prefix+`
    out = select(vec3f(1.0), vec3f(2.0), out <= vec3f(0.5));
    out = select(vec3f(1.0), vec3f(2.0), true);
    out.x = select(1.0, 2.0, out.y > 0.0);
}`), valid.Options{})
}
