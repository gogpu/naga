package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

// With ClampFragDepth, a fragment entry point stores frag_depth clamped to
// the range in two push constants at the offset given, which a WebGPU
// implementation sets to the viewport's depth range, whether frag_depth is
// the result or a member of it. Without it, nothing changes.
func TestClampFragDepth(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	for name, src := range map[string]string{
		"struct": `
struct Out { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out { return Out(p, p.z * 2.0); }`,
		"result": `
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 { return p.z * 2.0; }`,
	} {
		for _, clamp := range []bool{false, true} {
			opts := Options{Version: Version1_3}
			if clamp {
				opts.ClampFragDepth = &ClampFragDepth{Offset: 8}
			}
			spv := compileWGSLForCapabilityTestWithOpts(t, src, opts)
			path := filepath.Join(t.TempDir(), "frag.spv")
			if err := os.WriteFile(path, spv, 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(spirvVal, "--target-env", "vulkan1.1", path).CombinedOutput(); err != nil {
				t.Fatalf("%s, clamp %v: spirv-val:\n%s", name, clamp, out)
			}
			dis, err := exec.Command("spirv-dis", path).CombinedOutput()
			if err != nil {
				t.Fatalf("spirv-dis: %v\n%s", err, dis)
			}
			text := string(dis)
			for _, want := range []string{"FClamp", "PushConstant", "Offset 8", "Offset 12"} {
				if strings.Contains(text, want) != clamp {
					t.Errorf("%s, clamp %v: %q in the module: %v, want %v", name, clamp, want, !clamp, clamp)
				}
			}
		}
	}
}
