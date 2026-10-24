package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

// Two-row matrices in uniform buffers are declared std140-compatible, which
// Vulkan's extended layout rules accept without uniformBufferStandardLayout:
// a MatrixStride of 8 in a Uniform block is invalid. Every way of reading
// such a matrix is covered: whole, by constant and dynamic column, by
// component, as a struct member, an array element, a member of a struct in
// an array and a member of a nested struct.
func TestUniformTwoRowMatricesAreStd140(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	spv := compileWGSLForCapabilityTestWithOpts(t, `
struct WithMat { a: f32, m: mat3x2<f32>, b: vec2<f32> }
struct Inner { m: mat2x2<f32> }
struct Elem { m: mat3x2<f32>, b: vec2<f32> }
struct Outer { a: f32, @align(16) inner: Inner, arr: array<Elem, 2> }
@group(0) @binding(0) var<uniform> u_m: mat4x2<f32>;
@group(0) @binding(1) var<uniform> u_s: WithMat;
@group(0) @binding(2) var<uniform> u_a: array<mat2x2<f32>, 3>;
@group(0) @binding(3) var<uniform> u_o: Outer;
@group(0) @binding(4) var<storage, read_write> out: array<vec2<f32>, 16>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
	let c = i & 1u;
	out[0] = u_m[1] + u_m[c] + vec2(u_m[c][1], u_m[2][c]);
	let m = u_m;
	out[1] = m[3];
	out[2] = u_s.m[2] + u_s.m[c] + u_s.b + vec2(u_s.a, u_s.m[c][c]);
	let s = u_s;
	out[3] = s.m[1];
	out[4] = u_a[c][1] + u_a[2][c] + u_a[c][c];
	let a = u_a;
	out[5] = a[1][0];
	out[6] = u_o.inner.m[1] + u_o.inner.m[c] + u_o.arr[c].m[2] + u_o.arr[1].m[c];
	let o = u_o;
	out[7] = o.inner.m[0] + o.arr[1].b + vec2(o.a);
}
`, Options{Version: Version1_3})

	path := filepath.Join(t.TempDir(), "std140.spv")
	if err := os.WriteFile(path, spv, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(spirvVal, "--target-env", "vulkan1.1", path).CombinedOutput(); err != nil {
		t.Fatalf("spirv-val --target-env vulkan1.1:\n%s", strings.TrimSpace(string(out)))
	}
}

// The columns of an f16 matrix of three or four rows are 8 bytes apart in
// WGSL, which a Uniform block can't hold as a MatrixStride either, so such
// matrices are declared std140-compatible like two-row ones, and read whole,
// by constant and dynamic column and as members.
func TestUniformF16MatricesAreStd140(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	spv := compileWGSLForCapabilityTestWithOpts(t, `
enable f16;
struct WithMat { a: f16, m: mat3x3<f16>, n: mat2x4<f16>, b: vec4<f16> }
@group(0) @binding(0) var<uniform> u_m: mat4x3<f16>;
@group(0) @binding(1) var<uniform> u_s: WithMat;
@group(0) @binding(2) var<uniform> u_a: array<mat4x4<f16>, 2>;
@group(0) @binding(3) var<storage, read_write> out: array<vec4<f16>, 8>;

@compute @workgroup_size(1)
fn main(@builtin(local_invocation_index) i: u32) {
	let c = i & 1u;
	out[0] = vec4(u_m[1] + u_m[c], u_m[c][2]);
	let m = u_m;
	out[1] = vec4(m[3], 1.0h);
	out[2] = vec4(u_s.m[2] + u_s.m[c], u_s.a) + u_s.n[c] + u_s.b;
	let s = u_s;
	out[3] = vec4(s.m[1], s.n[0][3]);
	out[4] = u_a[c][1] + u_a[1][c];
	let a = u_a;
	out[5] = a[1][2];
}
`, Options{Version: Version1_3})

	path := filepath.Join(t.TempDir(), "std140-f16.spv")
	if err := os.WriteFile(path, spv, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(spirvVal, "--target-env", "vulkan1.1", path).CombinedOutput(); err != nil {
		t.Fatalf("spirv-val --target-env vulkan1.1:\n%s", strings.TrimSpace(string(out)))
	}
}
