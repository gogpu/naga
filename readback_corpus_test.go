package naga_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/dxcvalidator"
	"github.com/gogpu/naga/internal/readback"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// readbackDir is the GPU readback corpus. gogpu/wgpu finds it in the naga
// module directory (go list -m), so it stays at this path.
const readbackDir = "testdata/readback"

// TestReadbackCorpusCompile keeps the GPU readback corpus well formed: every
// case parses under format 1, its JSON bindings are the buffers its entry
// point binds, and it compiles on every naga backend a wgpu backend runs,
// with the external validator of that output where one is installed. A naga
// compile error is allowed only for a backend whose every adapter has a
// known failure. wgpu runs the cases on a GPU; this test runs none.
func TestReadbackCorpusCompile(t *testing.T) {
	cases, err := readback.Load(os.DirFS(readbackDir))
	if err != nil {
		t.Fatalf("load %s:\n%v", readbackDir, err)
	}
	v := newCorpusValidators(t)
	for i := range cases {
		c := &cases[i]
		t.Run(c.Name, func(t *testing.T) {
			m, err := readback.Module(c)
			if err != nil {
				t.Fatalf("frontend: %v", err)
			}
			if err := readback.CheckModule(&c.Spec, m); err != nil {
				t.Fatalf("%s.json does not match the shader:\n%v", c.Name, err)
			}
			for _, target := range corpusTargets {
				if f := target.neverRuns(&c.Spec); f != "" {
					t.Logf("%s: not compiled, the case requires %s and its backends never expose it", target.name, f)
					continue
				}
				err := target.compile(t, v, m, c.Spec.Entry)
				switch allowed := compileFailureAllowed(&c.Spec, target.backends); {
				case err != nil && allowed:
					t.Logf("%s: %v (known failure on every adapter)", target.name, err)
				case err != nil:
					t.Errorf("%s: %v", target.name, err)
				}
			}
		})
	}
}

// TestReadbackReadmeExample keeps the format specification honest: the case
// example in the corpus README parses under the current format.
func TestReadbackReadmeExample(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(readbackDir, readback.ReadmeName))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, ok := strings.Cut(string(readme), "```json\n")
	example, _, closed := strings.Cut(rest, "```")
	if !ok || !closed {
		t.Fatal("README has no ```json example")
	}
	if _, err := readback.ParseSpec([]byte(example)); err != nil {
		t.Errorf("README example does not parse: %v\n%s", err, example)
	}
}

// TestReadbackCorpusValidatorsReject proves each external check of
// TestReadbackCorpusCompile can fail: every validator, called exactly as the
// corpus test calls it, rejects a broken input. A check that cannot fail
// (wrong file extension, ignored argument) would pass every case silently.
func TestReadbackCorpusValidatorsReject(t *testing.T) {
	v := newCorpusValidators(t)
	tests := []struct {
		name, tool, file, src string
		args                  []string
	}{
		{"spirv-val", v.spirvVal, "case.spv", "\x03\x02\x23\x07", []string{"--target-env", "vulkan1.1"}},
		{"glslangValidator", v.glslang, "case.comp", "#version 310 es\nlayout(local_size_x = 1) in;\nvoid main() { x = 1; }\n", nil},
		{"dxc", v.dxc, "case.hlsl", "[numthreads(1, 1, 1)]\nvoid main() { x = 1; }\n", []string{"-T", "cs_6_0", "-E", "main"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool == "" {
				t.Skipf("%s not found", tt.name)
			}
			if err := runTool(t, tt.tool, tt.file, []byte(tt.src), tt.args...); err == nil {
				t.Errorf("%s accepted a broken input", tt.name)
			}
		})
	}
	t.Run("IDxcValidator", func(t *testing.T) {
		if !v.idxc {
			t.Skip("dxil.dll did not load")
		}
		m, err := readback.Module(&readback.Case{
			WGSL: "@group(0) @binding(0) var<storage, read_write> out: array<u32>;\n" +
				"@compute @workgroup_size(1) fn main() { out[0] = 1u; }",
			Spec: readback.Spec{Entry: "main"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := compileCorpusDXIL(t, v, m, "main"); err != nil {
			t.Fatalf("valid shader rejected: %v", err)
		}
		blob, err := dxil.Compile(m, dxil.DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		// Break a byte the structural and bitcode checks pass, so only
		// IDxcValidator can reject it.
		blob[len(blob)-8] ^= 0xFF
		if err := dxil.Validate(blob, dxil.ValidateBitcode); err != nil {
			t.Fatalf("the corruption is caught before IDxcValidator (%v): pick another byte", err)
		}
		err = dxil.Validate(blob, dxil.ValidateFull)
		if err == nil {
			t.Fatal("IDxcValidator accepted a corrupted DXIL container")
		}
		t.Logf("rejected: %v", err)
	})
}

func TestCompileFailureAllowed(t *testing.T) {
	kf := func(b readback.Backend, adapter string) readback.KnownFailure {
		return readback.KnownFailure{Backend: b, Adapter: adapter, Card: "BUG-X-001", Reason: "r"}
	}
	spirvBackends := []readback.Backend{readback.BackendVulkan, readback.BackendSoftware}
	tests := []struct {
		name string
		kfs  []readback.KnownFailure
		want bool
	}{
		{"none", nil, false},
		{"one backend of two", []readback.KnownFailure{kf(readback.BackendVulkan, "")}, false},
		{"both backends", []readback.KnownFailure{kf(readback.BackendVulkan, ""), kf(readback.BackendSoftware, "")}, true},
		{"one adapter only", []readback.KnownFailure{kf(readback.BackendVulkan, ""), kf(readback.BackendSoftware, "Intel")}, false},
		{"other backend", []readback.KnownFailure{kf(readback.BackendGLES, ""), kf(readback.BackendMetal, "")}, false},
	}
	for _, tt := range tests {
		spec := &readback.Spec{KnownFailures: tt.kfs}
		if got := compileFailureAllowed(spec, spirvBackends); got != tt.want {
			t.Errorf("%s: compileFailureAllowed = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCorpusTargetNeverRuns(t *testing.T) {
	gles := &corpusTarget{name: "glsl", never: []string{"shader-f16"}}
	spirv := &corpusTarget{name: "spirv"}
	f16 := &readback.Spec{Requires: []string{"shader-f16"}}
	plain := &readback.Spec{}
	if got := gles.neverRuns(f16); got != "shader-f16" {
		t.Errorf("glsl, f16 case: neverRuns = %q, want shader-f16", got)
	}
	if got := gles.neverRuns(plain); got != "" {
		t.Errorf("glsl, plain case: neverRuns = %q, want none", got)
	}
	if got := spirv.neverRuns(f16); got != "" {
		t.Errorf("spirv, f16 case: neverRuns = %q, want none", got)
	}
}

// corpusTarget is one naga backend and the wgpu backends that run its output.
type corpusTarget struct {
	name     string
	backends []readback.Backend
	compile  func(t *testing.T, v *corpusValidators, m *ir.Module, entry string) error
	// never lists the features no adapter of these backends exposes. A case
	// that requires one is skipped on them at run time, so it is not compiled
	// for them either.
	never []string
}

var corpusTargets = []corpusTarget{
	{"spirv", []readback.Backend{readback.BackendVulkan, readback.BackendSoftware}, compileCorpusSPIRV, nil},
	{"hlsl", []readback.Backend{readback.BackendDX12FXC}, compileCorpusHLSL, nil},
	{"dxil", []readback.Backend{readback.BackendDX12DXIL}, compileCorpusDXIL, nil},
	// GLES has no shader-f16: wgpu-hal's gles adapter offers only the
	// SHADER_F16_IN_F32 downlevel flag (gles/adapter.rs), and so does wgpu's.
	{"glsl", []readback.Backend{readback.BackendGLES}, compileCorpusGLSL, []string{"shader-f16"}},
	{"msl", []readback.Backend{readback.BackendMetal}, compileCorpusMSL, nil},
}

// neverRuns reports the first feature spec requires that target's backends
// never expose, or "" when the case can run there.
func (target *corpusTarget) neverRuns(spec *readback.Spec) string {
	for _, r := range spec.Requires {
		if slices.Contains(target.never, r) {
			return r
		}
	}
	return ""
}

// compileFailureAllowed reports whether every backend has a known failure
// on all of its adapters: only then may naga fail to compile for them.
func compileFailureAllowed(spec *readback.Spec, backends []readback.Backend) bool {
	for _, b := range backends {
		found := false
		for i := range spec.KnownFailures {
			kf := &spec.KnownFailures[i]
			if kf.Backend == b && kf.AllAdapters() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// corpusValidators holds the external validators found on this machine.
type corpusValidators struct {
	spirvVal, glslang, dxc string
	// idxc reports whether IDxcValidator (dxil.dll) loads.
	idxc bool
}

func newCorpusValidators(t *testing.T) *corpusValidators {
	v := &corpusValidators{}
	v.spirvVal, _ = testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools)
	v.glslang, _ = testtools.Optional(t, "glslangValidator", testtools.InstallSPIRVTools)
	v.dxc, _ = testtools.Optional(t, "dxc", testtools.InstallDXC)
	switch err := dxcvalidator.Run(func(*dxcvalidator.Validator) error { return nil }); {
	case err == nil:
		v.idxc = true
	case errors.Is(err, dxcvalidator.ErrUnsupported):
		t.Logf("IDxcValidator needs Windows: DXIL is checked structurally only")
	case testtools.Required("dxil.dll"):
		t.Fatalf("IDxcValidator (dxil.dll) required by %s: %v", testtools.EnvRequired, err)
	default:
		t.Logf("IDxcValidator (dxil.dll) did not load (%v): DXIL is checked structurally only", err)
	}
	return v
}

// runTool writes data to a file named name in a temporary directory and runs
// tool with args followed by the file's path.
func runTool(t *testing.T, tool, name string, data []byte, args ...string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(tool, append(args, path)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", filepath.Base(tool), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func compileCorpusSPIRV(t *testing.T, v *corpusValidators, m *ir.Module, _ string) error {
	code, err := spirv.NewBackend(spirv.DefaultOptions()).Compile(m)
	if err != nil || v.spirvVal == "" {
		return err
	}
	// spirv.DefaultOptions emits SPIR-V 1.1; Vulkan 1.1 is the first to take it.
	return runTool(t, v.spirvVal, "case.spv", code, "--target-env", "vulkan1.1")
}

func compileCorpusHLSL(t *testing.T, v *corpusValidators, m *ir.Module, entry string) error {
	src, info, err := hlsl.Compile(m, hlsl.DefaultOptions())
	if err != nil || v.dxc == "" {
		return err
	}
	name := entry
	if n, ok := info.EntryPointNames[entry]; ok {
		name = n
	}
	// wgpu compiles this HLSL with FXC, which is not a command-line tool
	// here; dxc at the lowest DXIL model still rejects broken source.
	return runTool(t, v.dxc, "case.hlsl", []byte(src), "-T", "cs_6_0", "-E", name)
}

func compileCorpusDXIL(_ *testing.T, v *corpusValidators, m *ir.Module, _ string) error {
	blob, err := dxil.Compile(m, dxil.DefaultOptions())
	if err != nil {
		return err
	}
	level := dxil.ValidateBitcode
	if v.idxc {
		level = dxil.ValidateFull
	}
	return dxil.Validate(blob, level)
}

// glslVersions are the GLSL versions with compute shaders that a wgpu GLES
// adapter compiles for: GLES 3.1 and desktop GL 4.3.
var glslVersions = []glsl.Version{glsl.VersionES310, glsl.Version430}

func compileCorpusGLSL(t *testing.T, v *corpusValidators, m *ir.Module, entry string) error {
	var errs []error
	for _, version := range glslVersions {
		src, _, err := glsl.Compile(m, glsl.Options{LangVersion: version, EntryPoint: entry, ForceHighPrecision: true})
		if err == nil && v.glslang != "" {
			err = runTool(t, v.glslang, "case.comp", []byte(src))
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("GLSL %v: %w", version, err))
		}
	}
	return errors.Join(errs...)
}

func compileCorpusMSL(_ *testing.T, _ *corpusValidators, m *ir.Module, _ string) error {
	_, _, err := msl.Compile(m, msl.DefaultOptions())
	return err
}
