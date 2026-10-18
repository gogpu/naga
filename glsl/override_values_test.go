// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl_test

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/ir"
)

// overrideOpsShader (testdata/overrides/ops.wgsl) stores the value of every
// operator, conversion and built-in applied to overrides, and reads two
// var<private> globals whose initializers use overrides. After the overrides
// are replaced by their values each store is a constant, which Rust naga's
// process_overrides evaluates (BUG-IR-OVERRIDES-001: ir.ProcessOverrides
// evaluated only + - * / and wrote 0 for every other operator).
var overrideOpsShader = mustReadFile("testdata/overrides/ops.wgsl")

func mustReadFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// overrideOpsCases: the expected lines are Rust naga v30's GLSL output
// (naga.exe --compact --entry-point main [--override k=v ...] in.wgsl out.comp;
// --compact makes the CLI pass the entry point to process_overrides, as
// wgpu-hal does).
var overrideOpsCases = []struct {
	name      string
	constants naga.PipelineConstants
	want      []string
}{
	{
		name: "defaults",
		want: []string{
			"layout(local_size_x = 4, local_size_y = 1, local_size_z = 1) in;",
			"bool pv = true;",
			"uint pu = 3u;",
			"_group_0_binding_0_cs[0] = 3u;",
			"_group_0_binding_0_cs[1] = 14u;",
			"_group_0_binding_0_cs[2] = 3u;",
			"_group_0_binding_0_cs[3] = 3u;",
			"_group_0_binding_0_cs[4] = 15u;",
			"_group_0_binding_0_cs[5] = 5u;",
			"_group_0_binding_0_cs[6] = 3u;",
			"_group_0_binding_0_cs[7] = 4294967295u;",
			"_group_0_binding_0_cs[8] = 1u;",
			"_group_0_binding_0_cs[9] = 0u;",
			"_group_0_binding_0_cs[10] = 0u;",
			"_group_0_binding_0_cs[11] = 1u;",
			"_group_0_binding_0_cs[12] = 1u;",
			"_group_0_binding_0_cs[13] = 0u;",
			"_group_0_binding_0_cs[16] = 0u;",
			"_group_0_binding_0_cs[17] = 9u;",
			"_group_0_binding_0_cs[18] = 2u;",
			"_group_0_binding_0_cs[19] = 2u;",
			"_group_0_binding_0_cs[20] = 4294967288u;",
			"_group_0_binding_1_cs[0] = -3;",
			"_group_0_binding_1_cs[1] = -3;",
			"_group_0_binding_1_cs[2] = -4;",
			"_group_0_binding_1_cs[3] = 7;",
			"_group_0_binding_1_cs[4] = 6;",
			"_group_0_binding_1_cs[5] = 7;",
			"_group_0_binding_2_cs[0] = 0.5;",
			"_group_0_binding_2_cs[1] = -2.5;",
			"_group_0_binding_2_cs[2] = 5.0;",
			"_group_0_binding_2_cs[3] = 7.0;",
			"_group_0_binding_2_cs[4] = 2.0;",
		},
	},
	{
		name: "given",
		// b = 2 is true (WebIDL boolean conversion: not 0 and not NaN);
		// 13 is wg's @id.
		constants: naga.PipelineConstants{"n": 9, "s": 5, "f": -1.25, "b": 2, "13": 8},
		want: []string{
			"layout(local_size_x = 8, local_size_y = 1, local_size_z = 1) in;",
			"bool pv = true;",
			"uint pu = 1u;",
			"_group_0_binding_0_cs[0] = 1u;",
			"_group_0_binding_0_cs[1] = 18u;",
			"_group_0_binding_0_cs[2] = 4u;",
			"_group_0_binding_0_cs[3] = 1u;",
			"_group_0_binding_0_cs[4] = 9u;",
			"_group_0_binding_0_cs[5] = 11u;",
			"_group_0_binding_0_cs[6] = 4u;",
			"_group_0_binding_0_cs[7] = 1u;",
			"_group_0_binding_0_cs[8] = 0u;",
			"_group_0_binding_0_cs[9] = 1u;",
			"_group_0_binding_0_cs[10] = 0u;",
			"_group_0_binding_0_cs[11] = 0u;",
			"_group_0_binding_0_cs[12] = 1u;",
			"_group_0_binding_0_cs[13] = 1u;",
			"_group_0_binding_0_cs[16] = 0u;",
			"_group_0_binding_0_cs[17] = 9u;",
			"_group_0_binding_0_cs[18] = 2u;",
			"_group_0_binding_0_cs[19] = 0u;",
			"_group_0_binding_0_cs[20] = 4294967286u;",
			"_group_0_binding_1_cs[0] = 1;",
			"_group_0_binding_1_cs[1] = 2;",
			"_group_0_binding_1_cs[2] = 2;",
			"_group_0_binding_1_cs[3] = -5;",
			"_group_0_binding_1_cs[4] = -6;",
			"_group_0_binding_1_cs[5] = 5;",
			"_group_0_binding_2_cs[0] = -0.25;",
			"_group_0_binding_2_cs[1] = 1.25;",
			"_group_0_binding_2_cs[2] = -2.5;",
			"_group_0_binding_2_cs[3] = 9.0;",
			"_group_0_binding_2_cs[4] = -2.0;",
		},
	},
}

// compileOverrides processes the overrides of source for one compute entry
// point, as wgpu-hal does per pipeline, and writes GLSL ES 3.1.
func compileOverrides(t *testing.T, source, entry string, constants naga.PipelineConstants) (string, error) {
	t.Helper()
	module, err := naga.ProcessOverrides(lowerWGSL(t, source), naga.OverrideOptions{
		Constants:  constants,
		EntryPoint: &naga.EntryPointSelector{Stage: ir.StageCompute, Name: entry},
	})
	if err != nil {
		return "", err
	}
	code, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES310, EntryPoint: entry})
	return code, err
}

// TestOverrideValues: every override-dependent value matches Rust naga.
func TestOverrideValues(t *testing.T) {
	for _, tc := range overrideOpsCases {
		t.Run(tc.name, func(t *testing.T) {
			code, err := compileOverrides(t, overrideOpsShader, "main", tc.constants)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			lines := make(map[string]bool)
			for _, l := range strings.Split(code, "\n") {
				lines[strings.TrimSpace(l)] = true
			}
			for _, w := range tc.want {
				if !lines[w] {
					t.Errorf("missing %q", w)
				}
			}
			// A uint used as a condition is not valid GLSL.
			if strings.Contains(code, "(0u ?") || strings.Contains(code, "(1u ?") {
				t.Errorf("uint literal used as a select condition:\n%s", code)
			}
		})
	}
}

// TestOverrideBoolFromNaN: NaN converts to false (WebIDL), Rust
// map_value_to_literal.
func TestOverrideBoolFromNaN(t *testing.T) {
	code, err := compileOverrides(t, overrideOpsShader, "main", naga.PipelineConstants{"b": math.NaN()})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !strings.Contains(code, "_group_0_binding_0_cs[18] = 1u;") {
		t.Errorf("select(1u, 2u, b) with b = NaN: want 1u\n%s", code)
	}
}

// TestOverrideConstantErrors: pipeline constants Rust rejects
// (PipelineConstantError) are errors, not silently ignored or truncated.
func TestOverrideConstantErrors(t *testing.T) {
	tests := []struct {
		name      string
		constants naga.PipelineConstants
		want      string // substring of Rust's message
	}{
		{"unknown key", naga.PipelineConstants{"nope": 1}, "not found in the shader"},
		{"unknown id", naga.PipelineConstants{"99": 1}, "not found in the shader"},
		{"u32 negative", naga.PipelineConstants{"n": -1}, "doesn't fit in destination"},
		{"u32 too big", naga.PipelineConstants{"n": 4294967296}, "doesn't fit in destination"},
		{"i32 too big", naga.PipelineConstants{"s": 2147483648}, "doesn't fit in destination"},
		{"u32 NaN", naga.PipelineConstants{"n": math.NaN()}, "needs to be finite"},
		{"f32 infinite", naga.PipelineConstants{"f": math.Inf(1)}, "needs to be finite"},
		{"f32 too big", naga.PipelineConstants{"f": 1e39}, "doesn't fit in destination"},
		{"workgroup size zero", naga.PipelineConstants{"13": 0}, "workgroup_size override isn't strictly positive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := compileOverrides(t, overrideOpsShader, "main", tc.constants)
			if err == nil {
				t.Fatalf("Compile succeeded, want an error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}

// TestOverrideMissingValueOnlyWhenReachable: an override without a value
// and without a default is an error only when the processed entry point
// uses it (Rust compacts the module first).
func TestOverrideMissingValueOnlyWhenReachable(t *testing.T) {
	const src = `
override used: u32;
override unused: u32;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn a() { o[0] = used; }
@compute @workgroup_size(1) fn b() { o[0] = unused; }
`
	code, err := compileOverrides(t, src, "a", naga.PipelineConstants{"used": 5})
	if err != nil {
		t.Fatalf("Compile a: %v", err)
	}
	// Rust naga v30 (--compact --entry-point a --override used=5).
	for _, want := range []string{"const uint used = 5u;", "_group_0_binding_0_cs[0] = used;"} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q\n%s", want, code)
		}
	}
	if strings.Contains(code, "unused") {
		t.Errorf("override unreachable from a is still in the output\n%s", code)
	}
	_, err = compileOverrides(t, src, "b", naga.PipelineConstants{"used": 5})
	if err == nil || !strings.Contains(err.Error(), "Missing value for pipeline-overridable constant") {
		t.Errorf("Compile b: err = %v, want a missing-value error for unused", err)
	}
}

// TestConstantF64RemainderNotFolded: the GLSL writer must not fold
// arithmetic on named constants itself. Rust keeps concrete f64 arithmetic in
// the IR and writes WGSL's remainder; ours folded it with float math that
// knows only + - * / and wrote 0.0.
func TestConstantF64RemainderNotFolded(t *testing.T) {
	const src = `
const d: f64 = 5.0lf;
@group(0) @binding(0) var<storage, read_write> o: array<f64>;
@compute @workgroup_size(1) fn main() { o[0] = d % 3.0lf; }
`
	code, _, err := glsl.Compile(lowerWGSL(t, src), glsl.Options{LangVersion: glsl.Version450, EntryPoint: "main"})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// Rust naga v30 (--profile core450).
	want := "_group_0_binding_0_cs[0] = (d - 3.0LF * trunc(d / 3.0LF));"
	if !strings.Contains(code, want) {
		t.Errorf("missing %q\n%s", want, code)
	}
}

// TestOverrideOutputMatchesRust: the whole GLSL output equals Rust naga v30's
// (testdata/overrides/ops.*.comp, written by naga.exe --compact --entry-point
// main [--override n=9 --override s=5 --override f=-1.25 --override b=2
// --override 13=8] ops.wgsl): values, declarations and expression numbering.
func TestOverrideOutputMatchesRust(t *testing.T) {
	for _, tc := range []struct {
		golden    string
		constants naga.PipelineConstants
	}{
		{"testdata/overrides/ops.defaults.comp", nil},
		{"testdata/overrides/ops.given.comp", naga.PipelineConstants{"n": 9, "s": 5, "f": -1.25, "b": 2, "13": 8}},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			got, err := compileOverrides(t, overrideOpsShader, "main", tc.constants)
			if err != nil {
				t.Fatal(err)
			}
			want := mustReadFile(tc.golden)
			if got == want {
				return
			}
			g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
			for i := 0; i < len(g) || i < len(w); i++ {
				var gl, wl string
				if i < len(g) {
					gl = g[i]
				}
				if i < len(w) {
					wl = w[i]
				}
				if gl != wl {
					t.Errorf("line %d:\n got: %q\nwant: %q", i+1, gl, wl)
				}
			}
		})
	}
}
