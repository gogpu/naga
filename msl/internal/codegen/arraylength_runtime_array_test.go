package codegen

import (
	"strings"
	"testing"
)

// TestArrayLengthRuntimeSizedArray pins arrayLength to the WGSL spec's
// NRuntime = truncate((EBBS - offset) / stride) (wgsl/index.bs, "element count
// of runtime-sized array"), written as 1 + (EBBS - offset - stride) / stride.
//
// Deviation from Rust naga 14328f49, which subtracts the element's size:
//
//	out.inner[0] = 1 + (_buffer_sizes.size0 - 0 - 12) / 16;
//	out.inner[1] = 1 + (_buffer_sizes.size1 - 16 - 12) / 16;
//	out.inner[2] = 1 + (_buffer_sizes.size2 - 0 - 4) / 4;
//
// For array<vec3<f32>> (size 12, stride 16) on a binding of 16N+12 bytes Rust
// returns N+1 and the spec N. Where size equals stride (array<u32>) both agree,
// and the line below is Rust's.
func TestArrayLengthRuntimeSizedArray(t *testing.T) {
	src := `
@group(0) @binding(0) var<storage, read_write> a: array<vec3<f32>>;
struct S { n: u32, d: array<vec3<f32>> }
@group(0) @binding(1) var<storage, read_write> s: S;
@group(0) @binding(2) var<storage, read_write> u: array<u32>;
@group(0) @binding(3) var<storage, read_write> out: array<u32, 3>;
@compute @workgroup_size(1) fn main() {
    out[0] = arrayLength(&a); out[1] = arrayLength(&s.d); out[2] = arrayLength(&u); }
`
	code := compileWGSL(t, src)
	for _, want := range []string{
		"out.inner[0] = 1 + (_buffer_sizes.size0 - 0 - 16) / 16;",
		"out.inner[1] = 1 + (_buffer_sizes.size1 - 16 - 16) / 16;",
		"out.inner[2] = 1 + (_buffer_sizes.size2 - 0 - 4) / 4;",
	} {
		mustContainMSL(t, code, want)
	}
}

// TestArrayLengthBindingArrayMemberIsAnError: the backend passes no
// per-element buffer sizes for a storage-buffer binding_array (P1-MSL-101), so
// arrayLength of a runtime-sized member has nothing to read. It must fail the
// compile, never return a made-up length. P1-MSL-103: it used to print
// `/* array length */ 0`. Rust naga 14328f49 writes
// `1 + (_buffer_sizes.size0[unsigned(gid.y)] - 4 - 4) / 4`.
func TestArrayLengthBindingArrayMemberIsAnError(t *testing.T) {
	src := `
enable wgpu_binding_array;
struct S { x: u32, far: array<u32> }
@group(0) @binding(0) var<storage, read_write> bufs: binding_array<S, 4>;
@group(0) @binding(1) var<storage, read_write> out: array<u32, 2>;
@compute @workgroup_size(1) fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    out[0] = arrayLength(&bufs[gid.y].far); }
`
	module := processOverridesForTest(t, lowerWGSLForTest(t, src), nil)
	code, _, err := Compile(module, DefaultOptions())
	if err == nil {
		t.Fatalf("want an error, got MSL:\n%s", code)
	}
	if want := "runtime-sized array inside binding_array is not supported"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not contain %q", err, want)
	}
}
