package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/wgsl"
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

// A fragment entry point that writes frag_depth and has push constants of
// its own finds the clamp's range in its own block, after its data, and
// the module declares no second block; an offset inside that data is an
// error.
func TestClampFragDepthWithPushConstants(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	for _, space := range []string{"immediate", "push_constant"} {
		t.Run(space, func(t *testing.T) { clampFragDepthWithPushConstants(t, spirvVal, space) })
	}
}

func clampFragDepthWithPushConstants(t *testing.T, spirvVal, space string) {
	src := `
struct Immediates { tint: vec4<f32> }
var<` + space + `> im: Immediates;
@vertex fn vs_main() -> @builtin(position) vec4<f32> { return im.tint; }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 { return p.z * im.tint.x; }`
	spv := compileWGSLForCapabilityTestWithOpts(t, src, Options{Version: Version1_3, ClampFragDepth: &ClampFragDepth{Offset: 16}})
	path := filepath.Join(t.TempDir(), "frag.spv")
	if err := os.WriteFile(path, spv, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(spirvVal, "--target-env", "vulkan1.1", path).CombinedOutput(); err != nil {
		t.Fatalf("spirv-val:\n%s", out)
	}
	dis, err := exec.Command("spirv-dis", path).CombinedOutput()
	if err != nil {
		t.Fatalf("spirv-dis: %v\n%s", err, dis)
	}
	text := strings.ReplaceAll(string(dis), "\r\n", "\n")
	for _, want := range []string{"FClamp", " 1 Offset 16", " 2 Offset 20"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q not in the module:\n%s", want, text)
		}
	}
	if n := strings.Count(text, "PushConstant\n"); n != 1 {
		t.Errorf("%d push constant variables, want 1:\n%s", n, text)
	}

	tokens, err := wgsl.NewLexer(src).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatal(err)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewBackend(Options{Version: Version1_3, ClampFragDepth: &ClampFragDepth{Offset: 8}}).Compile(module)
	if err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Errorf("offset 8 inside 16 bytes of push constants: err %v, want an overlap", err)
	}
}
