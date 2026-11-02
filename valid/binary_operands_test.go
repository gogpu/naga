package valid_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestBinaryOperandTypes: the operands of a binary operator have the types
// the WGSL spec gives it, as Rust naga's validate_expression checks
// Expression::Binary (ExpressionError::InvalidBinaryOperandTypes). Every
// released version accepted all of these and each backend wrote something
// different (P1-IR-029).
func TestBinaryOperandTypes(t *testing.T) {
	const prefix = `@group(0) @binding(0) var<storage, read_write> o: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> f: vec3f;
@compute @workgroup_size(1) fn main() { `
	for _, tt := range []struct{ name, body, want string }{
		{"float bitwise and", "f = f & f;",
			"Operation And can't work with ["},
		{"float xor", "let x = f.x ^ 1.0;",
			"Operation ExclusiveOr can't work with ["},
		{"matrix sizes", "let m = mat2x2f() * mat3x3f();",
			"Operation Multiply can't work with ["},
		{"u32 plus i32", "o[0] = u32(i32(o[1]) + i32(o[2]) + i32(o[1] + o[3]));",
			""},
		{"bool vector less", "let b = vec2(true, false) < vec2(false, true);",
			"Operation Less can't work with ["},
		// Beyond Rust: a let is run time (WGSL), so dividing by or shifting by
		// a let is valid whatever its value. Rust's validate_constant_divisor
		// and validate_constant_shift_amounts see the let's literal and reject
		// both; the spec's const-expression checks belong in the frontend
		// (P1-WGSL-101).
		{"integer division by a zero let", "let z = 0u; o[0] = o[1] / z;", ""},
		{"shift by a let of the bit width", "let s = 40u; o[0] = o[1] << s;", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := parse(t, prefix+tt.body+" }")
			if tt.want == "" {
				expectValid(t, m, valid.Options{})
				return
			}
			expectRejected(t, m, valid.Options{}, tt.want)
		})
	}
	expectValid(t, parse(t, prefix+`
    let a = f + f * 2.0;
    let b = mat3x3f() * f;
    let c = f * mat3x3f();
    let d = mat2x3f() * mat3x2f();
    let e = o[0] << 31u;
    let g = (o[1] & 3u) | (o[2] ^ 1u);
    let h = o[1] / 3u + o[2] % 2u;
    let i = f.x / 0.0;
    let j = vec2(true, false) == vec2(false, true);
    let k = (f.x < 1.0) && (o[0] != 0u);
    let l = vec3i(1) << vec3u(2u);
}`), valid.Options{})
}
