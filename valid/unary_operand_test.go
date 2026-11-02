package valid_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestUnaryOperandTypes: a unary operator takes a scalar or a vector of the
// kinds the WGSL spec gives it, as Rust naga's validate_expression checks
// Expression::Unary (ExpressionError::InvalidUnaryOperandType, "Operation
// Negate can't work with [h]"). The frontend lowered `-m` of a matrix and the
// backends made different things of it (P1-WGSL-096).
func TestUnaryOperandTypes(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
	}{
		{"negate matrix", "let n = -u.m;", "Operation Negate can't work with ["},
		{"negate u32", "let n = -u.x;", "Operation Negate can't work with ["},
		{"bitwise not f32", "let n = ~u.f;", "Operation BitwiseNot can't work with ["},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := parse(t, `struct U { m: mat2x2f, x: u32, f: f32 }
@group(0) @binding(0) var<uniform> u: U;
@compute @workgroup_size(1) fn main() { `+tt.body+` }`)
			expectRejected(t, m, valid.Options{}, tt.want)
		})
	}
	expectValid(t, parse(t, `struct U { v: vec3f, i: vec2i, b: bool, x: u32 }
@group(0) @binding(0) var<uniform> u: U;
@compute @workgroup_size(1) fn main() {
    let a = -u.v;
    let b = -u.i;
    let c = !u.b;
    let d = ~u.x;
    let e = ~u.i;
}`), valid.Options{})
}
