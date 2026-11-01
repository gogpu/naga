package snapshot_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// goldenBackend is one kind of golden snapshot: our own output, recorded to
// catch unintended changes (TestSnapshots). Unlike TestRustReference, goldens
// use our default backend options.
type goldenBackend struct {
	dir     string      // under testdata/golden/
	ext     string      // file extension
	target  rustTargets // the Rust target this backend corresponds to
	produce func(module *ir.Module, shader string) (string, error)
}

var goldenBackends = []goldenBackend{
	{"spv", ".spvasm", rustTargetSPIRV, goldenSPIRV},
	{"glsl", ".glsl", rustTargetGLSL, goldenGLSL},
	{"hlsl", ".hlsl", rustTargetHLSL, goldenHLSL},
	{"msl", ".msl", rustTargetMETAL, goldenMSL},
}

func (b goldenBackend) path(shader string) string {
	return filepath.Join(goldenDir, b.dir, shader+b.ext)
}

// goldenOutput produces a shader's golden output for one backend, after the
// same preparation as Rust's driver (prepareForBackend). It returns either
// the output, or a reason the backend legitimately has no output, or an
// error:
//
//   - GLSL without entry points: no output (GLSL is written per entry point);
//   - a compile error on a backend the shader targets in Rust (every backend
//     for our own shaders): an error — the test fails;
//   - a compile error on a backend Rust does not target for the shader: no
//     output, with the error as the reason.
func goldenOutput(params rustParams, shader shaderFile, module *ir.Module, b goldenBackend) (out, noOutput string, err error) {
	if b.target == rustTargetGLSL && len(module.EntryPoints) == 0 {
		return "", "no entry points: GLSL output is written per entry point", nil
	}
	prepared, err := prepareForBackend(shader.source, shader.name)
	if err != nil {
		return "", "", err
	}
	out, compileErr := b.produce(prepared, shader.name)
	switch {
	case compileErr == nil:
		return out, "", nil
	case params.targets.has(b.target):
		// The same error recorded as an unsupported Rust output (rustDeviations)
		// means the backend legitimately has no output yet.
		if dev := unsupportedDeviation(shader.name, b.dir, compileErr); dev != nil {
			return "", fmt.Sprintf("unsupported (%s): %s", dev.Card, dev.Reason), nil
		}
		return "", "", fmt.Errorf("%s compile error on a backend this shader targets: %w", b.dir, compileErr)
	default:
		return "", fmt.Sprintf("not a Rust target for this shader (%s): %v", params.targets, compileErr), nil
	}
}

// rustSingleOutputExt is the extension of the one output file Rust writes per
// shader for the backends that write one (GLSL writes one per entry point).
var rustSingleOutputExt = map[string]string{rustOutSPIRV: ".spvasm", rustOutMSL: ".metal", rustOutHLSL: ".hlsl"}

// unsupportedDeviation returns the unsupported rustDeviations entry for the
// shader's single output of one backend if err is the error it records.
func unsupportedDeviation(shader, kind string, err error) *rustDeviation {
	ext, ok := rustSingleOutputExt[kind]
	if !ok {
		return nil
	}
	dev := findDeviation(shader, "wgsl-"+shader+ext)
	if dev == nil || dev.Kind != deviationUnsupported || !strings.Contains(err.Error(), dev.ErrorContains) {
		return nil
	}
	return dev
}

// corpusParams returns the Rust test parameters of a corpus shader; our own
// shaders use Rust's defaults (every backend).
func corpusParams(shader shaderFile) (rustParams, error) {
	if shader.origin != originRust {
		return rustParams{targets: rustTargetsWGSLDefault}, nil
	}
	return loadRustParams(shader.name)
}

// TestSnapshots is the golden snapshot test: every corpus shader through every
// backend, compared with testdata/golden/. Regenerate after intentional
// changes with UPDATE_GOLDEN=1 and review the diff.
func TestSnapshots(t *testing.T) {
	for _, shader := range loadCorpus(t) {
		t.Run(shader.name, func(t *testing.T) {
			params, err := corpusParams(shader)
			if err != nil {
				t.Fatal(err)
			}
			module, ok := lowerCorpusShader(t, shader)
			if !ok {
				return
			}
			for _, b := range goldenBackends {
				t.Run(b.dir, func(t *testing.T) {
					out, noOutput, err := goldenOutput(params, shader, module, b)
					if err != nil {
						t.Fatal(err)
					}
					if noOutput != "" {
						t.Skip(noOutput)
					}
					compareGolden(t, b.path(shader.name), out)
				})
			}
		})
	}
}

// TestGoldenInventory fails on golden files no test compares: files of
// shaders that left the corpus, or of a backend that produces no output for
// the shader. Such files look like coverage but check nothing.
func TestGoldenInventory(t *testing.T) {
	compared := map[string]bool{}
	for _, shader := range loadCorpus(t) {
		params, err := corpusParams(shader)
		if err != nil {
			t.Fatal(err)
		}
		module, err := tryCompileToIR(shader.source)
		if err != nil {
			continue // reported by TestSnapshots
		}
		for _, b := range goldenBackends {
			if _, noOutput, err := goldenOutput(params, shader, module, b); err == nil && noOutput == "" {
				compared[filepath.ToSlash(b.path(shader.name))] = true
			}
		}
	}
	var orphans []string
	for _, b := range goldenBackends {
		entries, err := os.ReadDir(filepath.Join(goldenDir, b.dir))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			p := filepath.ToSlash(filepath.Join(goldenDir, b.dir, e.Name()))
			if !compared[p] {
				orphans = append(orphans, p)
			}
		}
	}
	sort.Strings(orphans)
	for _, p := range orphans {
		t.Errorf("golden file no test compares: %s", p)
	}
}

// goldenSPIRV uses Rust naga's default bounds-check policy, Unchecked, as the
// Rust snapshots of the same shaders do; the Restrict default is checked by
// spirv-val over the whole corpus (TestSpirvValBinary) and by the bounds
// tests of the spirv package.
func goldenSPIRV(module *ir.Module, shader string) (string, error) {
	// The goldens are SPIR-V 1.1 with native f16 inputs and outputs, the
	// options they were first written with; the zero value is 1.0 and the
	// f32 polyfill.
	opts := spirv.Options{Version: spirv.Version1_1, UseStorageInputOutput16: true}
	opts.BoundsCheckPolicies = uncheckedPolicies()
	binary, err := spirvBinary(module, opts)
	// A shader that needs a newer version than the default (subgroup
	// operations need 1.3), which the backend refuses to raise, takes the
	// one its Rust parameters give.
	if err != nil && strings.Contains(err.Error(), "need SPIR-V") {
		opts.Version = readRustSPVConfig(shader).Version
		binary, err = spirvBinary(module, opts)
	}
	if err != nil {
		return "", err
	}
	return disassembleSPIRV(binary), nil
}

// goldenGLSL compiles every entry point and joins them with a header per entry
// point when there are several. GLSL 330, the default, has neither compute
// shaders nor storage buffers, so compute entry points and those that use a
// storage buffer are written for GLSL 430. Like goldenSPIRV, it uses Rust
// naga's default bounds-check policy, Unchecked.
func goldenGLSL(module *ir.Module, _ string) (string, error) {
	optionsFor := func(ep *ir.EntryPoint) glsl.Options {
		opts := glsl.DefaultOptions()
		opts.LangVersion = glsl.Version330 // the zero version, which compile retries with 4.30
		opts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
		opts.BoundsCheckPolicies = uncheckedPolicies()
		if ep.Stage == ir.StageCompute || usesStorageBuffer(module, &ep.Function) {
			opts.LangVersion = glsl.Version430
		}
		return opts
	}
	// compile writes GLSL 330 when it has every feature the entry point
	// needs and GLSL 430 otherwise, as the GLSL backend refuses a version
	// that lacks one (Rust naga's check_availability).
	compile := func(opts glsl.Options) (string, error) {
		code, _, err := glsl.Compile(module, opts)
		if err != nil && opts.LangVersion == glsl.Version330 && strings.Contains(err.Error(), "doesn't support the required feature") {
			opts.LangVersion = glsl.Version430
			code, _, err = glsl.Compile(module, opts)
		}
		return code, err
	}
	if len(module.EntryPoints) == 1 {
		opts := optionsFor(&module.EntryPoints[0])
		opts.EntryPoint = nil // the only entry point
		return compile(opts)
	}
	parts := make([]string, 0, len(module.EntryPoints))
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		code, err := compile(optionsFor(ep))
		if err != nil {
			return "", fmt.Errorf("entry point %q: %w", ep.Name, err)
		}
		parts = append(parts, fmt.Sprintf("// === Entry Point: %s (%s) ===\n", ep.Name, stageName(ep.Stage))+code)
	}
	return strings.Join(parts, "\n"), nil
}

// usesStorageBuffer reports whether fn, or a function it calls, refers to a
// global in the storage address space.
func usesStorageBuffer(module *ir.Module, fn *ir.Function) bool {
	for _, e := range fn.Expressions {
		if gv, ok := e.Kind.(ir.ExprGlobalVariable); ok && module.GlobalVariables[gv.Variable].Space == ir.SpaceStorage {
			return true
		}
	}
	var calls func(ir.Block) bool
	calls = func(b ir.Block) bool {
		for _, st := range b {
			switch s := st.Kind.(type) {
			case ir.StmtCall:
				if usesStorageBuffer(module, &module.Functions[s.Function]) {
					return true
				}
			case ir.StmtBlock:
				if calls(s.Block) {
					return true
				}
			case ir.StmtIf:
				if calls(s.Accept) || calls(s.Reject) {
					return true
				}
			case ir.StmtLoop:
				if calls(s.Body) || calls(s.Continuing) {
					return true
				}
			case ir.StmtSwitch:
				for _, c := range s.Cases {
					if calls(c.Body) {
						return true
					}
				}
			}
		}
		return false
	}
	return calls(fn.Body)
}

func goldenHLSL(module *ir.Module, shader string) (string, error) {
	opts, err := rustHLSLOptions(shader)
	if err != nil {
		return "", err
	}
	// Shaders whose Rust parameters have no HLSL target give no
	// immediates_target; every one that does gives b0 in space 0.
	if opts.ImmediatesTarget == nil {
		opts.ImmediatesTarget = &hlsl.BindTarget{}
	}
	code, _, err := hlsl.Compile(module, opts)
	// A shader that needs a newer shader model (mesh shaders, the view
	// index) is written at it: the backend refuses rather than raises it.
	var smErr *backend.ShaderModelError
	if errors.As(err, &smErr) {
		opts.ShaderModel = smErr.Required
		code, _, err = hlsl.Compile(module, opts)
	}
	return code, err
}

func goldenMSL(module *ir.Module, _ string) (string, error) {
	opts := msl.DefaultOptions()
	opts.LangVersion = msl.Version1_0
	// The goldens check ReadZeroSkipWrite, the policy they were written with.
	opts.BoundsCheckPolicies = backend.BoundsCheckPolicies{
		Index:        backend.BoundsCheckReadZeroSkipWrite,
		Buffer:       backend.BoundsCheckReadZeroSkipWrite,
		ImageLoad:    backend.BoundsCheckReadZeroSkipWrite,
		BindingArray: backend.BoundsCheckReadZeroSkipWrite,
	}
	opts.FakeMissingBindings = true
	// Rust's empty per_entry_point_map: placeholders for every resource, as
	// the Rust snapshots of the same shaders have them.
	opts.PerEntryPointMap = map[string]msl.EntryPointResources{}
	code, _, err := msl.Compile(module, opts)
	return code, err
}

// uncheckedPolicies is Rust naga's default bounds-check policy, Unchecked, on
// every field; the zero value of backend.BoundsCheckPolicies is Restrict.
func uncheckedPolicies() backend.BoundsCheckPolicies {
	return backendBoundsCheckPolicies(rustBoundsCheckPolicies{})
}
