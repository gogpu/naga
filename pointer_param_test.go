package naga

import (
	"strings"
	"testing"
)

// TestStoragePointerParameterRejected: a user-defined function may not take
// a pointer to the storage, uniform or workgroup address space (WGSL spec,
// "Function Restrictions", lifted only by the unrestricted_pointer_parameters
// language extension). Rust naga's validator rejects it with
// FunctionError::InvalidArgumentPointerSpace. naga compiled it; the MSL
// backend then wrote an unclamped runtime-array index (P0-MSL-100).
func TestStoragePointerParameterRejected(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"storage runtime array", `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
fn f(p: ptr<storage, array<u32>, read_write>, i: u32) -> u32 { (*p)[i + 1u] = 2u; return (*p)[i]; }
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let r = f(&a, gid.x); a[0] = r; }`,
			"Argument 'p' at index 0 is a pointer of space Storage { .. }, which can't be passed into functions."},
		{"storage element", `
@group(0) @binding(0) var<storage, read_write> a: array<u32>;
fn f(p: ptr<storage, u32, read_write>) { *p = 2u; }
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    f(&a[gid.x]); }`,
			"Argument 'p' at index 0 is a pointer of space Storage { .. }, which can't be passed into functions."},
		{"workgroup", `
var<workgroup> w: u32;
fn f(x: u32, p: ptr<workgroup, u32>) { *p = x; }
@compute @workgroup_size(1) fn main() { f(1u, &w); }`,
			"Argument 'p' at index 1 is a pointer of space WorkGroup, which can't be passed into functions."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile(tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Compile error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}
