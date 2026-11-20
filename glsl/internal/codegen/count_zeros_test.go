package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

// countZerosShader takes countLeadingZeros and countTrailingZeros of u32,
// i32 and vec3<i32>.
const countZerosShader = `
@group(0) @binding(0) var<storage, read_write> u: array<u32>;
@group(0) @binding(1) var<storage, read_write> i: array<i32>;
@group(0) @binding(2) var<storage, read_write> v: array<vec3<i32>>;

@compute @workgroup_size(1)
fn main() {
    u[1] = countLeadingZeros(u[0]);
    i[1] = countLeadingZeros(i[0]);
    v[1] = countLeadingZeros(v[0]);
    u[2] = countTrailingZeros(u[0]);
    i[2] = countTrailingZeros(i[0]);
    v[2] = countTrailingZeros(v[0]);
}
`

// countLeadingZeros(x) is 31 - (index of the highest set bit), which is 32
// for zero and 0 for a negative i32, and countTrailingZeros(x) is the index
// of the lowest set bit, 32 for zero. findMSB and findLSB return -1 for zero,
// and findMSB of a negative int counts from the sign bit, so the argument is
// read as unsigned, the leading count is 31 minus the result and the trailing
// count is clamped to 32 as unsigned.
func TestCountZeros(t *testing.T) {
	opts := DefaultOptions()
	opts.LangVersion = Version430
	g, _, err := compileWGSLHelper(countZerosShader, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"uint((31 - findMSB(", "(31 - findMSB(uint(", "(31 - findMSB(uvec3(",
		"min(uint(findLSB(", "int(min(uint(findLSB(", "ivec3(min(uvec3(findLSB(",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("GLSL lacks %q:\n%s", want, g)
		}
	}
}

// countZerosFragmentShader exercises countLeadingZeros of a u32 and an i32
// through a fragment entry point, so it compiles at versions that predate
// compute shaders (GLSL 3.30 desktop, ES 3.00). The values come from a uniform
// buffer so the evaluator cannot fold them into literals.
const countZerosFragmentShader = `
struct Inputs {
    u: u32,
    i: i32,
    uv: vec3<u32>,
    iv: vec3<i32>,
};

@group(0) @binding(0) var<uniform> inputs: Inputs;

@fragment
fn main() -> @location(0) vec4<f32> {
    let a = countLeadingZeros(inputs.u);
    let b = countLeadingZeros(inputs.i);
    let c = countLeadingZeros(inputs.uv);
    let d = countLeadingZeros(inputs.iv);
    return vec4<f32>(
        f32(a) + f32(c.x),
        f32(b) + f32(d.x),
        f32(c.y) + f32(d.y),
        f32(c.z) + f32(d.z),
    );
}
`

// countLeadingZeros is findMSB-based only where integer functions exist:
// desktop GLSL 4.00+ and ES 3.10+. Below that the built-in does not exist at
// all, so Rust naga (back/glsl/writer.rs CountLeadingZeros) emits a log2
// polyfill instead: 31 - floor(log2(float(x) + 0.5)), with the negative int
// case short-circuited to 0. Same boundary as Version::supports_integer_functions.
func TestCountZerosVersionGate(t *testing.T) {
	cases := []struct {
		name        string
		version     Version
		wantFindMSB bool
	}{
		{"glsl330", Version330, false},
		{"es300", VersionES300, false},
		{"es310", VersionES310, true},
		{"glsl400", Version400, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.LangVersion = tc.version
			g, _, err := compileWGSLHelper(countZerosFragmentShader, opts)
			if err != nil {
				t.Fatal(err)
			}
			// The real compiler proves the polyfill is valid GLSL at the
			// versions that lack findMSB, not merely free of the built-in;
			// NAGA_REQUIRE_TOOLS can make its absence a failure, as the other
			// glslang tests of the repo do.
			glslang, _ := testtools.Optional(t, "glslangValidator", testtools.InstallSPIRVTools)
			validateCountZerosGLSL(t, glslang, g)
			if tc.wantFindMSB {
				if !strings.Contains(g, "findMSB(") {
					t.Errorf("GLSL %s should use findMSB for countLeadingZeros:\n%s", tc.version, g)
				}
				if strings.Contains(g, "floor(log2(") {
					t.Errorf("GLSL %s should not use the log2 polyfill:\n%s", tc.version, g)
				}
				return
			}
			// Below integer functions: the built-in must be gone, replaced by
			// the polyfill for both the uint and the int argument.
			if strings.Contains(g, "findMSB(") {
				t.Errorf("GLSL %s emits findMSB for countLeadingZeros, which no such driver has:\n%s", tc.version, g)
			}
			for _, want := range []string{
				"uint(31.0 - floor(log2(float(",           // u32 argument
				"< 0 ? 0 : int(31.0 - floor(log2(float(",  // i32 argument
				") + 0.5)))",                              // the polyfill tail
				"uvec3(vec3(31.0) - floor(log2(vec3(",     // vec3<u32> argument
				"ivec3(mix(vec3(31.0) - floor(log2(vec3(", // vec3<i32> argument
				"lessThan(", // the i32 vector zero-select
			} {
				if !strings.Contains(g, want) {
					t.Errorf("GLSL %s lacks polyfill fragment %q:\n%s", tc.version, want, g)
				}
			}
		})
	}
}

// validateCountZerosGLSL runs glslangValidator over the emitted source as a
// fragment shader, when the tool was found.
func validateCountZerosGLSL(t *testing.T, glslang, source string) {
	t.Helper()
	if glslang == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "count_zeros.frag")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(glslang, path).CombinedOutput(); err != nil {
		t.Errorf("glslangValidator:\n%s\n%s", out, source)
	}
}
