package naga

import (
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// TestEarlyDepthTest: @early_depth_test is Rust naga's EarlyDepthTest,
// Force or Allow{conservative} (front/wgsl/parse/mod.rs:2017-2029), written
// as Rust's SPIR-V (back/spv/writer.rs:1821-1842) and GLSL
// (back/glsl/writer.rs:179-193) writers do. force and unchanged were one
// value, so unchanged wrote early_fragment_tests, and SPIR-V wrote no mode.
func TestEarlyDepthTest(t *testing.T) {
	const (
		modeEarlyFragmentTests = 9
		modeDepthGreater       = 14
		modeDepthLess          = 15
		modeDepthUnchanged     = 16
		opExecutionMode        = 16
	)
	for _, tc := range []struct {
		arg      string
		spvMode  uint32
		glslLine string
	}{
		{"force", modeEarlyFragmentTests, "layout(early_fragment_tests) in;"},
		{"greater_equal", modeDepthGreater, "layout (depth_greater) out float gl_FragDepth;"},
		{"less_equal", modeDepthLess, "layout (depth_less) out float gl_FragDepth;"},
		{"unchanged", modeDepthUnchanged, "layout (depth_unchanged) out float gl_FragDepth;"},
	} {
		t.Run(tc.arg, func(t *testing.T) {
			src := "@fragment @early_depth_test(" + tc.arg + ")\n" +
				"fn main(@builtin(position) pos: vec4<f32>) -> @builtin(frag_depth) f32 { return pos.z - 0.1; }"
			module, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}

			bin, err := spirvBinary(module, spirv.Options{})
			if err != nil {
				t.Fatal(err)
			}
			ops, operands := spirvOps(t, bin)
			var modes []uint32
			for i, op := range ops {
				if op == opExecutionMode {
					modes = append(modes, operands[i][1])
				}
			}
			if !slices.Contains(modes, tc.spvMode) {
				t.Errorf("SPIR-V execution modes %v, want %d", modes, tc.spvMode)
			}

			code, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(code, tc.glslLine) {
				t.Errorf("GLSL lacks %q:\n%s", tc.glslLine, code)
			}
			if tc.arg != "force" && strings.Contains(code, "early_fragment_tests") {
				t.Errorf("GLSL forces early tests for %s:\n%s", tc.arg, code)
			}
		})
	}
}

// TestEarlyDepthTestUnknown: Rust's UnknownConservativeDepth; the lowerer
// treated an unknown word as force.
func TestEarlyDepthTestUnknown(t *testing.T) {
	src := "@fragment @early_depth_test(sometimes)\nfn main() {}"
	_, _, err := wgsl.Parse(src, wgsl.ParseOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown conservative depth: `sometimes`") {
		t.Fatalf("error %v, want unknown conservative depth", err)
	}
}
