// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
)

const clampedStore = "gl_FragDepth = clamp(%s, _immediates_binding_fs.naga_frag_depth_min, _immediates_binding_fs.naga_frag_depth_max);"

// TestClampFragDepth: with ClampFragDepth, a fragment entry point that
// returns frag_depth, alone or in a struct, and has no immediate data
// declares an immediates uniform holding just the range, lists its two
// floats in ImmediateItems, and stores frag_depth clamped to it, on desktop
// GL and ES. Without the option, or in
// an entry point that doesn't write frag_depth, nothing changes.
func TestClampFragDepth(t *testing.T) {
	glslang, _ := testtools.Optional(t, "glslangValidator", testtools.InstallSPIRVTools)
	for name, tc := range map[string]struct {
		source string
		values []string
	}{
		"result": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 {
	if p.x > 2.0 { return 0.25; }
	return p.z * 2.0;
}`, []string{"0.25", "(p.z * 2.0)"}},
		"struct": {`
struct Out { @location(0) color: vec4<f32>, @builtin(frag_depth) depth: f32 }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> Out { return Out(p, p.z * 2.0); }`, []string{"_tmp_return.depth"}},
		"none": {`
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> { return p; }`, nil},
	} {
		for _, version := range []glsl.Version{glsl.Version330, glsl.VersionES300} {
			for _, clamp := range []*glsl.ClampFragDepth{nil, {Offset: 0}} {
				got, info, err := glsl.Compile(lowerWGSL(t, tc.source), glsl.Options{
					LangVersion:        version,
					EntryPoint:         "fs_main",
					ForceHighPrecision: true,
					ClampFragDepth:     clamp,
				})
				if err != nil {
					t.Fatalf("%s, %v, clamp %v: %v", name, version, clamp, err)
				}
				precision := ""
				if version.ES {
					precision = "highp "
				}
				decl := fmt.Sprintf("struct naga_immediates {\n    %sfloat naga_frag_depth_min;\n    %sfloat naga_frag_depth_max;\n};\nuniform naga_immediates _immediates_binding_fs;",
					precision, precision)
				want := clamp != nil && tc.values != nil
				if strings.Contains(got, decl) != want {
					t.Errorf("%s, %v, clamp %v: declares the range: %v, want %v:\n%s", name, version, clamp, !want, want, got)
				}
				if (len(info.ImmediateItems) == 2) != want {
					t.Errorf("%s, %v, clamp %v: ImmediateItems %+v", name, version, clamp, info.ImmediateItems)
				}
				for _, v := range tc.values {
					store := fmt.Sprintf(clampedStore, v)
					if strings.Contains(got, store) != want {
						t.Errorf("%s, %v, clamp %v: %q: %v, want %v:\n%s", name, version, clamp, store, !want, want, got)
					}
				}
				validateGLSL(t, glslang, "frag", got)
			}
		}
	}
}

// TestClampFragDepthWithImmediates: a fragment entry point with immediate
// data of its own finds the range in the same uniform, after that data,
// which it then reads through the uniform's data member, and ImmediateItems
// lists that data, then the range at the offset. A vertex entry
// point using the same immediates is left as it was. An offset inside the
// data, or one that isn't a multiple of 4, is an error.
func TestClampFragDepthWithImmediates(t *testing.T) {
	glslang, _ := testtools.Optional(t, "glslangValidator", testtools.InstallSPIRVTools)
	const src = `
struct Immediates { tint: vec4<f32> }
var<immediate> im: Immediates;
@vertex fn vs_main() -> @builtin(position) vec4<f32> { return im.tint; }
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @builtin(frag_depth) f32 { return p.z * im.tint.x; }`
	compile := func(ep string, offset uint32) (string, glsl.TranslationInfo, error) {
		return glsl.Compile(lowerWGSL(t, src), glsl.Options{
			LangVersion:    glsl.Version330,
			EntryPoint:     ep,
			ClampFragDepth: &glsl.ClampFragDepth{Offset: offset},
		})
	}

	got, info, err := compile("fs_main", 16)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"struct naga_immediates {\n    Immediates data;\n    float naga_frag_depth_min;\n    float naga_frag_depth_max;\n};\nuniform naga_immediates _immediates_binding_fs;",
		" = _immediates_binding_fs.data.tint.x;",
		", _immediates_binding_fs.naga_frag_depth_min, _immediates_binding_fs.naga_frag_depth_max);",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q not in:\n%s", want, got)
		}
	}
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	items := []glsl.ImmediateItem{
		{AccessPath: "_immediates_binding_fs.data.tint", Type: ir.VectorType{Size: ir.Vec4, Scalar: f32}, Offset: 0, SizeBytes: 16},
		{AccessPath: "_immediates_binding_fs.naga_frag_depth_min", Type: f32, Offset: 16, SizeBytes: 4},
		{AccessPath: "_immediates_binding_fs.naga_frag_depth_max", Type: f32, Offset: 20, SizeBytes: 4},
	}
	if !reflect.DeepEqual(info.ImmediateItems, items) {
		t.Errorf("ImmediateItems:\n got %+v\nwant %+v", info.ImmediateItems, items)
	}
	validateGLSL(t, glslang, "frag", got)

	got, _, err = compile("vs_main", 16)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "uniform Immediates _immediates_binding_vs;") || strings.Contains(got, "naga_immediates") {
		t.Errorf("vertex stage changed:\n%s", got)
	}
	validateGLSL(t, glslang, "vert", got)

	for offset, want := range map[uint32]string{8: "overlaps the 16 bytes", 18: "not a multiple of 4"} {
		if _, _, err := compile("fs_main", offset); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("offset %d: error %v, want %q", offset, err, want)
		}
	}
}

// validateGLSL runs glslangValidator over source as a shader of stage,
// when the tool was found.
func validateGLSL(t *testing.T, glslang, stage, source string) {
	t.Helper()
	if glslang == "" {
		return
	}
	path := filepath.Join(t.TempDir(), "shader."+stage)
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(glslang, path).CombinedOutput(); err != nil {
		t.Errorf("glslangValidator:\n%s\n%s", out, source)
	}
}
