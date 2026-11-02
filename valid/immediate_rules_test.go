package valid_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestImmediateTypes: the store type of a var<immediate> holds no array, as
// the WGSL immediate-data proposal requires and Rust naga checks
// (valid/type.rs ImmediateError::InvalidArray, reported by
// valid/interface.rs as GlobalVariableError::InvalidImmediateType). Every
// backend compiled them before (P1-WGSL-097).
func TestImmediateTypes(t *testing.T) {
	const want = "Global variable [0] 'im' is invalid: Type is not valid for use as a immediate data"
	for _, tt := range []struct{ name, decl string }{
		{"array", "var<immediate> im: array<vec4f, 2>;"},
		{"struct with array", "struct S { a: vec4f, b: array<f32, 4> }\nvar<immediate> im: S;"},
		{"nested struct with array", "struct A { b: array<u32, 2> }\nstruct S { a: A }\nvar<immediate> im: S;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			expectRejected(t, parse(t, tt.decl+"\n@compute @workgroup_size(1) fn main() { _ = im; }"), valid.Options{}, want)
		})
	}
	expectValid(t, parse(t, `struct S { a: vec4f, m: mat4x4f, s: u32 }
var<immediate> im: S;
@compute @workgroup_size(1) fn main() { _ = im; }`), valid.Options{})
}

// TestOneImmediatePerEntryPoint: an entry point uses at most one immediate
// variable, its callees included (the proposal's rule; Rust's
// EntryPointError::MoreThanOneImmediateUsed, valid/interface.rs). Two entry
// points may each use their own.
func TestOneImmediatePerEntryPoint(t *testing.T) {
	m := parse(t, `var<immediate> a: vec4f;
var<immediate> b: vec4f;
fn f() -> vec4f { return b; }
@compute @workgroup_size(1) fn main() { _ = a + f(); }`)
	expectRejected(t, m, valid.Options{}, "Entry point main at Compute is invalid: More than 1 immediate data variable is used")

	expectValid(t, parse(t, `var<immediate> a: vec4f;
var<immediate> b: vec4f;
@compute @workgroup_size(1) fn one() { _ = a; }
@compute @workgroup_size(1) fn two() { _ = b; }`), valid.Options{})
}
