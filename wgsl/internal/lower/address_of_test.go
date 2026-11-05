package lower

import "testing"

// TestAddressOfVectorComponent: `&` of one component of a vector is a
// shader-creation error (WGSL spec, "Address-Of Expression": a vector
// component is not a memory view that a pointer can name). Rust naga
// rejects it in its lowerer (front/wgsl/lower/mod.rs, ast::Expression::
// AddrOf → Error::InvalidAddrOfOperand, "cannot take the address of a vector
// component"); naga accepted it and handed backends a pointer to a scalar
// inside a vector.
func TestAddressOfVectorComponent(t *testing.T) {
	const want = "cannot take the address of a vector component"
	tests := []struct {
		name string
		src  string
	}{
		{"dynamic index through a pointer", `
fn f(q: ptr<function, vec4<f32>>, k: i32) -> f32 {
    let r = &(*q)[k];
    return *r;
}`},
		{"constant index of a local", `
fn f() -> f32 {
    var v: vec4<f32>;
    let r = &v[1];
    return *r;
}`},
		{"member of a private", `
var<private> v: vec3<u32>;
fn f() -> u32 {
    let r = &v.y;
    return *r;
}`},
		{"element of a storage vector", `
@group(0) @binding(0) var<storage, read_write> s: vec4<f32>;
fn f(i: u32) {
    let r = &s[i];
    *r = 1.0;
}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectError(t, tt.src, want)
		})
	}

	// The address of a whole vector, of an array element and of a matrix
	// column stays valid (Rust checks only a base that points to a vector).
	for _, src := range []string{`
fn f(q: ptr<function, vec4<f32>>) -> vec4<f32> {
    let r = &(*q);
    return *r;
}`, `
fn f(k: i32) -> f32 {
    var a: array<f32, 4>;
    let r = &a[k];
    return *r;
}`, `
fn f(k: i32) -> vec2<f32> {
    var m: mat2x2<f32>;
    let r = &m[k];
    return *r;
}`} {
		mustCompile(t, src)
	}
}
