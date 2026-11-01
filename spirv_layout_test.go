package naga

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/spirv"
)

// TestSPIRVLayoutFreeLocals: from SPIR-V 1.5 Vulkan forbids ArrayStride,
// Offset and MatrixStride on the types of Function and Private variables
// (VUID-StandaloneSpirv-None-10684); they take layout-free types, copied to
// and from the decorated ones with OpCopyLogical. Rust naga decorates them
// (gfx-rs/wgpu#7696, open): the three shapes reported there, and wgpu's
// repro, at SPIR-V 1.5 and 1.6 under the Vulkan rules of the API version
// that takes each.
func TestSPIRVLayoutFreeLocals(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	shaders := map[string]string{
		// wgpu's repro: a Function array of vectors, indexed dynamically.
		"function array": `
@vertex fn vs(@builtin(vertex_index) i: u32) -> @builtin(position) vec4<f32> {
    var pos = array<vec2<f32>, 3>(vec2(0.0, 1.0), vec2(-1.0, -1.0), vec2(1.0, -1.0));
    return vec4(pos[i], 0.0, 1.0);
}`,
		// A Function array whose type a storage buffer also uses.
		"array shared with a buffer": `
@group(0) @binding(0) var<storage, read_write> buf: array<array<u32, 4>, 2>;
@compute @workgroup_size(1) fn main() {
    var local: array<u32, 4> = buf[0];
    local[1] = 7u;
    buf[1] = local;
}`,
		// A Function struct whose members carry Offset (and a matrix's
		// MatrixStride), copied to and from a uniform and a buffer.
		"struct with offsets": `
struct S { a: f32, m: mat3x3<f32>, b: array<vec4<f32>, 2> }
@group(0) @binding(0) var<uniform> u: S;
@group(0) @binding(1) var<storage, read_write> out: S;
var<private> p: S;
@compute @workgroup_size(1) fn main() {
    var s = u;
    s.a = 2.0;
    p = s;
    out = p;
}`,
		// A Workgroup array (layout-free at every version).
		"workgroup array": `
var<workgroup> w: array<vec4<f32>, 8>;
@group(0) @binding(0) var<storage, read_write> out: array<vec4<f32>, 8>;
@compute @workgroup_size(8) fn main(@builtin(local_invocation_index) i: u32) {
    w[i] = vec4(f32(i));
    workgroupBarrier();
    out[i] = w[7u - i];
}`,
	}
	for name, src := range shaders {
		for _, target := range []struct {
			version spirv.Version
			env     string
		}{{spirv.Version1_3, "vulkan1.1"}, {spirv.Version1_5, "vulkan1.2"}, {spirv.Version1_6, "vulkan1.3"}} {
			bin, err := CompileWithOptions(src, CompileOptions{SPIRV: spirv.Options{Version: target.version}})
			if err != nil {
				t.Fatalf("%s at %s: %v", name, target.env, err)
			}
			path := filepath.Join(t.TempDir(), "shader.spv")
			if err := os.WriteFile(path, bin, 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(spirvVal, "--target-env", target.env, path).CombinedOutput(); err != nil {
				t.Errorf("%s at %s: %s", name, target.env, out)
			}
		}
	}
}
