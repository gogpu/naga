// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package naga_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

func lowerSource(t *testing.T, src string) *ir.Module {
	t.Helper()
	ast, err := naga.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m, err := naga.LowerWithSource(ast, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	return m
}

func mslOf(t *testing.T, m *ir.Module) string {
	t.Helper()
	code, _, err := msl.Compile(m, msl.DefaultOptions())
	if err != nil {
		t.Fatalf("msl: %v", err)
	}
	return code
}

const twoEntryPoints = `
override n: u32 = 7u;
override onlyB: u32;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn a() { o[0] = n % 4u; }
@compute @workgroup_size(1) fn b() { o[0] = onlyB; }
`

var selectA = &naga.EntryPointSelector{Stage: ir.StageCompute, Name: "a"}

// TestProcessOverridesDoesNotModifyInput: the input is left as it was, so it
// can be processed again for other pipelines.
func TestProcessOverridesDoesNotModifyInput(t *testing.T) {
	m := lowerSource(t, twoEntryPoints)
	want := lowerSource(t, twoEntryPoints)
	out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"n": 9}, EntryPoint: selectA})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, want) {
		t.Error("ProcessOverrides changed its input")
	}
	if out == m {
		t.Error("a processed module must be a new module")
	}
	if len(out.EntryPoints) != 1 || out.EntryPoints[0].Name != "a" {
		t.Errorf("entry points: got %d, want only a as EntryPoints[0]", len(out.EntryPoints))
	}
	if !strings.Contains(mslOf(t, out), "o[0] = 1u;") {
		t.Errorf("n %% 4u with n = 9: want 1u\n%s", mslOf(t, out))
	}
}

// TestProcessOverridesIdentity: with nothing to do the module itself is
// returned (Rust Cow::Borrowed).
func TestProcessOverridesIdentity(t *testing.T) {
	m := lowerSource(t, `@compute @workgroup_size(1) fn main() {}`)
	for _, opts := range []naga.OverrideOptions{{}, {EntryPoint: &naga.EntryPointSelector{Stage: ir.StageCompute, Name: "main"}}} {
		out, err := naga.ProcessOverrides(m, opts)
		if err != nil {
			t.Fatal(err)
		}
		if out != m {
			t.Errorf("opts %+v: got a new module, want the input itself", opts)
		}
	}
}

// TestProcessOverridesConcurrent: one module processed for several pipelines
// at once (run with -race).
func TestProcessOverridesConcurrent(t *testing.T) {
	m := lowerSource(t, twoEntryPoints)
	var wg sync.WaitGroup
	errs := make([]error, 16)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n := uint32(i + 4)
			out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"n": float64(n)}, EntryPoint: selectA})
			if err != nil {
				errs[i] = err
				return
			}
			code, _, err := msl.Compile(out, msl.DefaultOptions())
			if err != nil {
				errs[i] = err
				return
			}
			if want := fmt.Sprintf("o[0] = %du;", n%4); !strings.Contains(code, want) {
				errs[i] = fmt.Errorf("n = %d: missing %q", n, want)
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// TestProcessOverridesErrors: error kinds (errors.Is), the key (errors.As)
// and Rust naga's messages (naga.exe --override, v30).
func TestProcessOverridesErrors(t *testing.T) {
	m := lowerSource(t, twoEntryPoints)
	tests := []struct {
		name    string
		opts    naga.OverrideOptions
		kind    error
		key     string
		message string
	}{
		{"unknown key", naga.OverrideOptions{Constants: naga.PipelineConstants{"zz": 1}}, naga.ErrOverrideNotFound, "zz",
			"pipeline-overridable constant 'zz' not found in the shader"},
		// Sorted order: "aa" is reported before "zz" whatever the map order.
		{"first key in sorted order", naga.OverrideOptions{Constants: naga.PipelineConstants{"zz": 1, "aa": 1, "mm": 1}}, naga.ErrOverrideNotFound, "aa",
			"pipeline-overridable constant 'aa' not found in the shader"},
		{"missing value", naga.OverrideOptions{EntryPoint: &naga.EntryPointSelector{Stage: ir.StageCompute, Name: "b"}}, naga.ErrOverrideMissingValue, "onlyB",
			"Missing value for pipeline-overridable constant with identifier string: 'onlyB'"},
		{"missing value, all entry points", naga.OverrideOptions{}, naga.ErrOverrideMissingValue, "onlyB",
			"Missing value for pipeline-overridable constant with identifier string: 'onlyB'"},
		{"out of range", naga.OverrideOptions{Constants: naga.PipelineConstants{"n": -1, "onlyB": 1}}, naga.ErrOverrideValueOutOfRange, "n",
			"Source f64 value doesn't fit in destination"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for range 5 { // map order varies between runs
				_, err := naga.ProcessOverrides(m, tc.opts)
				if !errors.Is(err, tc.kind) {
					t.Fatalf("err = %v, want %v", err, tc.kind)
				}
				var pce *naga.PipelineConstantError
				if !errors.As(err, &pce) || pce.Key != tc.key {
					t.Fatalf("err = %#v, want a *PipelineConstantError for %q", err, tc.key)
				}
				if err.Error() != tc.message {
					t.Fatalf("message %q, want %q", err.Error(), tc.message)
				}
			}
		})
	}
}

// TestProcessOverridesKeys: keys are compared exactly, as WGSL identifiers
// (WebGPU spec, GPUProgrammableStage.constants). Rust parses an id key as a
// number and so also accepts "013" for @id(13); the spec does not.
func TestProcessOverridesKeys(t *testing.T) {
	m := lowerSource(t, `
@id(13) override w: u32 = 1u;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn main() { o[0] = w; }
`)
	for _, k := range []string{"013", "+13", "w"} {
		_, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{k: 2}})
		if !errors.Is(err, naga.ErrOverrideNotFound) {
			t.Errorf("key %q: err = %v, want ErrOverrideNotFound (an @id override is named by its id only)", k, err)
		}
	}
	out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"13": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mslOf(t, out), "constant uint w = 2u;") {
		t.Errorf("key \"13\" for @id(13)\n%s", mslOf(t, out))
	}
}

// TestProcessOverridesKeyOfUnusedOverride: a key may name an override the
// selected entry point does not use (spec: "not required to be statically
// used"); Rust checks keys before compacting.
func TestProcessOverridesKeyOfUnusedOverride(t *testing.T) {
	m := lowerSource(t, twoEntryPoints)
	out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"onlyB": 3}, EntryPoint: selectA})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	for _, o := range out.Overrides {
		if o.Name == "onlyB" {
			t.Error("onlyB is unused by a and must be compacted away")
		}
	}
}

// TestProcessOverridesEntryPointNotFound: Rust returns a single-entry-point
// module unchanged whatever the selector; ours reports the mismatch.
func TestProcessOverridesEntryPointNotFound(t *testing.T) {
	m := lowerSource(t, `@compute @workgroup_size(1) fn main() {}`)
	for _, sel := range []naga.EntryPointSelector{{Stage: ir.StageCompute, Name: "other"}, {Stage: ir.StageVertex, Name: "main"}} {
		_, err := naga.ProcessOverrides(m, naga.OverrideOptions{EntryPoint: &sel})
		if !errors.Is(err, naga.ErrEntryPointNotFound) {
			t.Errorf("selector %+v: err = %v, want ErrEntryPointNotFound", sel, err)
		}
	}
}

// TestProcessOverridesEvaluationError: an expression that becomes constant is
// evaluated, and a shader-creation error (here integer division by zero) is an
// error, as in Rust ("Division by zero" for z = 1; 1 for z = 2).
func TestProcessOverridesEvaluationError(t *testing.T) {
	m := lowerSource(t, `
override z: i32 = 1;
@group(0) @binding(0) var<storage, read_write> o: array<i32>;
@compute @workgroup_size(1) fn main() { o[0] = 1 / (z - 1); }
`)
	if _, err := naga.ProcessOverrides(m, naga.OverrideOptions{}); err == nil || !strings.Contains(err.Error(), "Division by zero") {
		t.Errorf("z = 1: err = %v, want a division-by-zero error", err)
	}
	out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"z": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mslOf(t, out), "o[0] = 1;") {
		t.Errorf("z = 2: want o[0] = 1\n%s", mslOf(t, out))
	}
}

// TestProcessOverridesF16: an f16 value is rounded to nearest and must stay
// finite (Rust half::f16::from_f64: 0.1 → 0.099975586h; 70000 does not fit).
func TestProcessOverridesF16(t *testing.T) {
	m := lowerSource(t, `
enable f16;
override h: f16 = 1.0h;
@group(0) @binding(0) var<storage, read_write> o: array<f32>;
@compute @workgroup_size(1) fn main() { o[0] = f32(h); }
`)
	out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"h": 0.1}})
	if err != nil {
		t.Fatal(err)
	}
	code := mslOf(t, out)
	for _, want := range []string{"constant half h = 0.099975586h;", "o[0] = 0.099975586;"} {
		if !strings.Contains(code, want) {
			t.Errorf("missing %q\n%s", want, code)
		}
	}
	if _, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"h": 70000}}); !errors.Is(err, naga.ErrOverrideValueOutOfRange) {
		t.Errorf("h = 70000: err = %v, want ErrOverrideValueOutOfRange", err)
	}
}

const workgroupShader = `
override wg: u32 = 4u;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(wg, 2) fn main() { o[0] = 1u; }
`

// TestProcessOverridesWorkgroupSize: an override workgroup size is resolved
// (Rust process_workgroup_size_override); 0 is ErrNegativeWorkgroupSize.
func TestProcessOverridesWorkgroupSize(t *testing.T) {
	m := lowerSource(t, workgroupShader)
	out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"wg": 8}})
	if err != nil {
		t.Fatal(err)
	}
	ep := out.EntryPoints[0]
	if ep.Workgroup != [3]uint32{8, 2, 1} {
		t.Errorf("Workgroup = %v, want [8 2 1]", ep.Workgroup)
	}
	if ep.WorkgroupSizeOverrides != [3]*ir.ExpressionHandle{} {
		t.Errorf("WorkgroupSizeOverrides = %v, want all nil after processing", ep.WorkgroupSizeOverrides)
	}
	_, err = naga.ProcessOverrides(m, naga.OverrideOptions{Constants: naga.PipelineConstants{"wg": 0}})
	if !errors.Is(err, naga.ErrNegativeWorkgroupSize) || err.Error() != "workgroup_size override isn't strictly positive" {
		t.Errorf("wg = 0: err = %v, want ErrNegativeWorkgroupSize with Rust's message", err)
	}
}

// localSize returns the LocalSize execution mode of a SPIR-V module
// (OpExecutionMode = 16, LocalSize = 17).
func localSize(t *testing.T, spv []byte) [3]uint32 {
	t.Helper()
	words := make([]uint32, len(spv)/4)
	for i := range words {
		words[i] = uint32(spv[4*i]) | uint32(spv[4*i+1])<<8 | uint32(spv[4*i+2])<<16 | uint32(spv[4*i+3])<<24
	}
	for i := 5; i < len(words); {
		n, op := words[i]>>16, words[i]&0xffff
		if n == 0 {
			break
		}
		if op == 16 && n >= 6 && words[i+2] == 17 {
			return [3]uint32{words[i+3], words[i+4], words[i+5]}
		}
		i += int(n)
	}
	t.Fatal("no LocalSize execution mode")
	return [3]uint32{}
}

// TestCompileResolvesOverrideDefaults: naga.Compile processes overrides with
// their defaults before writing SPIR-V, as Rust's CLI does. The workgroup size
// given by an override was 1 2 1 (wgpu BUG-OVERRIDE-WORKGROUP-001: wgpu's
// Vulkan backend compiles through naga.Compile); Rust writes 4 2 1.
func TestCompileResolvesOverrideDefaults(t *testing.T) {
	spv, err := naga.Compile(workgroupShader)
	if err != nil {
		t.Fatal(err)
	}
	if got := localSize(t, spv); got != [3]uint32{4, 2, 1} {
		t.Errorf("LocalSize = %v, want [4 2 1]", got)
	}
	// An override used by an expression compiles too (it was "unsupported
	// expression kind: ir.ExprOverride").
	if _, err := naga.Compile(twoEntryPoints[:strings.Index(twoEntryPoints, "override onlyB")] +
		"@group(0) @binding(0) var<storage, read_write> o: array<u32>;\n@compute @workgroup_size(1) fn a() { o[0] = n % 4u; }\n"); err != nil {
		t.Errorf("override in an expression: %v", err)
	}
	// No default and no value: WebGPU requires a value.
	if _, err := naga.Compile("override m: u32;\n@group(0) @binding(0) var<storage, read_write> o: array<u32>;\n@compute @workgroup_size(1) fn main() { o[0] = m; }\n"); !errors.Is(err, naga.ErrOverrideMissingValue) {
		t.Errorf("override without default: err = %v, want ErrOverrideMissingValue", err)
	}
}

// TestProcessOverridesDerivedDefault: an override's default may use another
// override; it is evaluated with that override's value (Rust naga v30 MSL).
func TestProcessOverridesDerivedDefault(t *testing.T) {
	m := lowerSource(t, `
override n: u32 = 7u;
override m: u32 = n * 2u;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn main() { o[0] = m; o[1] = m + n; }
`)
	for _, tc := range []struct {
		constants naga.PipelineConstants
		want      []string
	}{
		{nil, []string{"constant uint n = 7u;", "constant uint m = 14u;", "o[0] = m;", "o[1] = 21u;"}},
		{naga.PipelineConstants{"n": 3}, []string{"constant uint n = 3u;", "constant uint m = 6u;", "o[1] = 9u;"}},
		{naga.PipelineConstants{"m": 5}, []string{"constant uint n = 7u;", "constant uint m = 5u;", "o[1] = 12u;"}},
		{naga.PipelineConstants{"n": 3, "m": 5}, []string{"constant uint n = 3u;", "constant uint m = 5u;", "o[1] = 8u;"}},
	} {
		out, err := naga.ProcessOverrides(m, naga.OverrideOptions{Constants: tc.constants})
		if err != nil {
			t.Fatalf("%v: %v", tc.constants, err)
		}
		code := mslOf(t, out)
		for _, w := range tc.want {
			if !strings.Contains(code, w) {
				t.Errorf("%v: missing %q\n%s", tc.constants, w, code)
			}
		}
	}
}

// TestBackendsRejectUnresolvedOverrides: every backend reports a module that
// still uses overrides with ir.ErrUnresolvedOverride (Rust Error::Override),
// for an override expression and for a workgroup size given by an override
// (which would otherwise be written as the placeholder 1). DXIL emitted the
// default value instead.
func TestBackendsRejectUnresolvedOverrides(t *testing.T) {
	shaders := map[string]string{
		"expression": `
override n: u32 = 7u;
@group(0) @binding(0) var<storage, read_write> o: array<u32>;
@compute @workgroup_size(1) fn main() { o[0] = n % 4u; }
`,
		"workgroup size": workgroupShader,
	}
	for name, src := range shaders {
		backends := map[string]func(*ir.Module) error{
			"spirv": func(m *ir.Module) error { _, err := spirv.NewBackend(spirv.DefaultOptions()).Compile(m); return err },
			"msl":   func(m *ir.Module) error { _, _, err := msl.Compile(m, msl.DefaultOptions()); return err },
			"hlsl":  func(m *ir.Module) error { _, _, err := hlsl.Compile(m, hlsl.DefaultOptions()); return err },
			"glsl": func(m *ir.Module) error {
				_, _, err := glsl.Compile(m, glsl.Options{LangVersion: glsl.VersionES310, EntryPoint: "main"})
				return err
			},
			"dxil": func(m *ir.Module) error { _, err := dxil.Compile(m, dxil.DefaultOptions()); return err },
		}
		for backend, compile := range backends {
			if err := compile(lowerSource(t, src)); !errors.Is(err, ir.ErrUnresolvedOverride) {
				t.Errorf("%s, %s: err = %v, want ir.ErrUnresolvedOverride", name, backend, err)
			}
			processed, err := naga.ProcessOverrides(lowerSource(t, src), naga.OverrideOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if err := compile(processed); err != nil {
				t.Errorf("%s, %s after ProcessOverrides: %v", name, backend, err)
			}
		}
	}
}
