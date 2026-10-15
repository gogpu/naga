package snapshot_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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
		return "", "", fmt.Errorf("%s compile error on a backend this shader targets: %w", b.dir, compileErr)
	default:
		return "", fmt.Sprintf("not a Rust target for this shader (%s): %v", params.targets, compileErr), nil
	}
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
			module, err := tryCompileToIR(shader.source)
			if err != nil {
				t.Fatalf("lowering failed: %v", err)
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

func goldenSPIRV(module *ir.Module, _ string) (string, error) {
	binary, err := spirv.NewBackend(spirv.DefaultOptions()).Compile(module)
	if err != nil {
		return "", err
	}
	return disassembleSPIRV(binary), nil
}

// goldenGLSL compiles every entry point (GLSL 430 for compute) and joins them
// with a header per entry point when there are several.
func goldenGLSL(module *ir.Module, _ string) (string, error) {
	optionsFor := func(ep *ir.EntryPoint) glsl.Options {
		opts := glsl.DefaultOptions()
		opts.EntryPoint = ep.Name
		if ep.Stage == ir.StageCompute {
			opts.LangVersion = glsl.Version430
		}
		return opts
	}
	if len(module.EntryPoints) == 1 {
		opts := optionsFor(&module.EntryPoints[0])
		opts.EntryPoint = "" // the only entry point
		code, _, err := glsl.Compile(module, opts)
		return code, err
	}
	parts := make([]string, 0, len(module.EntryPoints))
	for i := range module.EntryPoints {
		ep := &module.EntryPoints[i]
		code, _, err := glsl.Compile(module, optionsFor(ep))
		if err != nil {
			return "", fmt.Errorf("entry point %q: %w", ep.Name, err)
		}
		parts = append(parts, fmt.Sprintf("// === Entry Point: %s (%s) ===\n", ep.Name, stageName(ep.Stage))+code)
	}
	return strings.Join(parts, "\n"), nil
}

func goldenHLSL(module *ir.Module, shader string) (string, error) {
	opts, err := rustHLSLOptions(shader)
	if err != nil {
		return "", err
	}
	code, _, err := hlsl.Compile(module, opts)
	return code, err
}

func goldenMSL(module *ir.Module, _ string) (string, error) {
	opts := msl.DefaultOptions()
	opts.LangVersion = msl.Version1_0
	opts.FakeMissingBindings = true
	code, _, err := msl.Compile(module, opts)
	return code, err
}
