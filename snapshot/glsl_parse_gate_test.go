package snapshot_test

// The GLSL corpus parse gate of design §18.1: every vendored GLSL source either
// parses into a module valid.Validate accepts or is refused with a
// *glsl.ParseError. The must-parse set below pins what the frontend handles
// today; it grows with each phase, and a shader in it that stops parsing or
// stops validating fails the gate. A source outside the set is reported, never
// silently ignored.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/overrides"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// glslMustParse is the set of vendored GLSL sources the frontend must parse
// into a module the validator accepts. Phase 1 set: filled from the sweep of
// 2026-11-15 (44 of the 47 sources); later phases move the remaining sources in
// as their features land.
var glslMustParse = map[string]struct{}{
	"210-bevy-2d-shader.frag":       {},
	"210-bevy-2d-shader.vert":       {},
	"210-bevy-shader.vert":          {},
	"246-collatz.comp":              {},
	"277-casting.frag":              {},
	"280-matrix-cast.frag":          {},
	"484-preprocessor-if.frag":      {},
	"800-out-of-bounds-panic.vert":  {},
	"896-push-constant.frag":        {},
	"900-implicit-conversions.frag": {},
	"901-lhs-field-select.frag":     {},
	"931-constant-emitting.frag":    {},
	"932-for-loop-if.frag":          {},
	"bevy-pbr.frag":                 {},
	"bevy-pbr.vert":                 {},
	"bits.frag":                     {},
	"bool-select.frag":              {},
	"buffer.frag":                   {},
	"clamp-splat.vert":              {},
	"const-global-swizzle.frag":     {},
	"constant-array-size.frag":      {},
	"declarations.frag":             {},
	"double-math-functions.frag":    {},
	"dual-source-blending.frag":     {},
	"expressions.frag":              {},
	"f16-glsl.comp":                 {},
	"fma.frag":                      {},
	"functions_call.frag":           {},
	"global-constant-array.frag":    {},
	"images.frag":                   {},
	"interpolate.frag":              {},
	"inverse-polyfill.frag":         {},
	"long-form-matrix.frag":         {},
	"math-functions.frag":           {},
	"multipart-for-loop.frag":       {},
	"prepostfix.frag":               {},
	"quad_glsl.frag":                {},
	"quad_glsl.vert":                {},
	"sampler-functions.frag":        {},
	"samplers.frag":                 {},
	"spec-constant.frag":            {},
	"statements.frag":               {},
	"variations.frag":               {},
	"vector-functions.frag":         {},
}

// glslKnownRefusals are the sources this frontend refuses on purpose, with the
// reason. The design's R5 decision: an absent #version directive is a
// *ParseError (a #version-less shader is not valid GLSL, even though Rust naga
// parses it with version 0). Recorded with that decision: glslangValidator
// accepts 5246-dual-iteration.frag and anonymous-entry-point-type.frag without
// a #version, so the refusal is the spec's requirement as we read it (design
// §14, decided by the lead in LEAD-024), not a limitation of the lowering;
// local-var-init-in-loop.comp is refused by glslang too.
var glslKnownRefusals = map[string]string{
	"anonymous-entry-point-type.frag": "no #version directive (design §14, R5)",
	"local-var-init-in-loop.comp":     "no #version directive (design §14, R5)",
	"5246-dual-iteration.frag":        "no #version directive (design §14, R5)",
}

// glslStageOf returns the stage a corpus file name names by its extension.
func glslStageOf(t *testing.T, path string) ir.ShaderStage {
	t.Helper()
	switch filepath.Ext(path) {
	case ".vert":
		return ir.StageVertex
	case ".frag":
		return ir.StageFragment
	case ".comp":
		return ir.StageCompute
	default:
		t.Fatalf("%s: unknown GLSL source extension", path)
		return 0
	}
}

// TestGLSLParseGate parses every vendored GLSL source and checks the pinned
// must-parse set.
func TestGLSLParseGate(t *testing.T) {
	dir := filepath.Join(rustCorpusDir, "in", "glsl")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the GLSL corpus is missing: %v", err)
	}

	var sources []string
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if ext == ".vert" || ext == ".frag" || ext == ".comp" {
			sources = append(sources, entry.Name())
		}
	}
	sort.Strings(sources)
	if len(sources) == 0 {
		t.Fatalf("no GLSL sources under %s", dir)
	}

	var parsed, refused []string
	compiledExceptions := make(map[string]struct{})
	lastBackend := make(map[string]string)
	for _, name := range sources {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			module, _, parseErr := glsl.Parse(string(data), glsl.ParseOptions{Stage: glslStageOf(t, name)})
			_, pinned := glslMustParse[name]
			if parseErr != nil {
				var wantErr *glsl.ParseError
				if !errors.As(parseErr, &wantErr) {
					t.Fatalf("error is %T, want *glsl.ParseError", parseErr)
				}
				refused = append(refused, name)
				if pinned {
					t.Fatalf("pinned must-parse source was refused: %v", parseErr)
				}
				t.Logf("refused (not yet in the must-parse set): %v", parseErr)
				return
			}
			if verr := validateGLSL(t, module); verr != nil {
				if pinned {
					t.Fatalf("pinned must-parse source produced an invalid module: %v", verr)
				}
				t.Logf("parsed but invalid (not yet in the must-parse set): %v", verr)
				return
			}
			// A module that validates is not yet a module a backend accepts:
			// the review of 2026-11-15 found an out-argument lowered to a
			// value that passed validation and was refused by the SPIR-V
			// backend (finding P0-1), so every pinned source compiles here.
			backend, cerr := compileGLSLModule(t, name, module)
			lastBackend[name] = backend
			if cerr != nil {
				if reason, excepted := glslCompileExceptions[name][backend]; excepted {
					t.Logf("%s: %s refuses it (recorded: %s): %v", name, backend, reason, cerr)
					compiledExceptions[name+"|"+backend] = struct{}{}
				} else if pinned {
					t.Fatalf("%s: a pinned source does not compile: %v", name, cerr)
				} else {
					t.Logf("does not compile (not yet in the must-parse set): %v", cerr)
					return
				}
			}

			parsed = append(parsed, name)
			if !pinned {
				t.Logf("parses, validates and compiles, add it to glslMustParse: %s", name)
			}
		})
	}

	// A recorded backend exception that no longer applies is stale; one whose
	// step was not reached because an earlier step failed is only reported.
	for name, byBackend := range glslCompileExceptions {
		for backend, reason := range byBackend {
			if _, applied := compiledExceptions[name+"|"+backend]; applied {
				continue
			}
			if lastBackend[name] == "" {
				t.Errorf("%s: the recorded %s exception (%s) no longer applies; remove it", name, backend, reason)
			} else {
				t.Logf("%s: the recorded %s exception (%s) was not reached (%s failed first)", name, backend, reason, lastBackend[name])
			}
		}
	}
	t.Logf("%d sources: %d parse and validate, %d refused", len(sources), len(parsed), len(refused))

	// A recorded refusal that stops being refused is stale bookkeeping, and a
	// source that is neither pinned nor recorded is reported so it cannot be
	// forgotten.
	refusedSet := make(map[string]struct{}, len(refused))
	for _, name := range refused {
		refusedSet[name] = struct{}{}
		if _, known := glslKnownRefusals[name]; !known {
			t.Errorf("%s is refused without a recorded reason; add it to glslKnownRefusals or fix the frontend", name)
		}
	}
	for name, reason := range glslKnownRefusals {
		if _, ok := refusedSet[name]; !ok {
			t.Errorf("%s is recorded as refused (%s) but no longer is; move it to glslMustParse", name, reason)
		}
	}
}

// validateGLSL validates a parsed module with every capability: the corpus
// sources use f16, f64 and dual-source blending, and the capability check is
// the device's business (valid.Options comes from the caller), not the parse's.
func validateGLSL(t *testing.T, module *ir.Module) error {
	t.Helper()
	_, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll})
	return err
}

// glslValExceptions are the pinned sources whose SPIR-V output spirv-val
// refuses today, with the reason. Both are backend defects, not frontend ones:
//   - images.frag: imageSize on an arrayed storage image is written as
//     OpImageQuerySizeLod (VUID-OpImageQuerySizeLod-04659); Rust naga's output
//     fails the same VUID.
//   - double-math-functions.frag: the SPIR-V backend hard-codes f32 for
//     length/distance/determinant (reachable from WGSL too).
//
// glslCompileExceptions are the pinned sources one backend refuses for a
// reason that is not a frontend defect, keyed by source; the entry names the
// backend and the reason. Each is checked against the reason's backend, so a
// different failure on the same source still fails the gate, and an exception
// that stops applying fails it too.
var glslCompileExceptions = map[string]map[string]string{
	"800-out-of-bounds-panic.vert": {
		"hlsl": "push constants need the caller's ImmediatesTarget (the pinned naga panics on this shader too, SKIPPED.md)",
	},
	"896-push-constant.frag": {
		"hlsl": "push constants need the caller's ImmediatesTarget",
	},
	"double-math-functions.frag": {
		"spirv-val": "the SPIR-V backend types Length/Distance/Determinant as f32 (card P2-SPV-001)",
		"msl":       "the MSL backend cannot write f64 (the pinned naga refuses it too, SKIPPED.md)",
	},
	"vector-functions.frag": {
		"msl": "the MSL backend cannot write f64 (the pinned naga refuses it too, SKIPPED.md)",
	},
}

// compileGLSLModule runs a parsed module through the three text backends and,
// when spirv-val is installed, through the SPIR-V validator. It returns which
// step failed: "spirv", "spirv-val", "msl" or "hlsl". A module with pipeline
// overrides is processed with their default values first, as every consumer
// must (the backends refuse a module that still carries ExprOverride).
func compileGLSLModule(t *testing.T, name string, module *ir.Module) (string, error) {
	t.Helper()
	if len(module.Overrides) != 0 {
		info, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll})
		if err != nil {
			return "overrides", err
		}
		processed, _, err := overrides.Process(module, info, nil, nil)
		if err != nil {
			return "overrides", err
		}
		module = processed
	}
	binary, _, err := spirv.Compile(module, spirv.Options{})
	if err != nil {
		return "spirv", err
	}
	if validator, ok := testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools); ok {
		path := filepath.Join(t.TempDir(), "module.spv")
		if err := os.WriteFile(path, binary, 0o600); err != nil {
			return "spirv-val", err
		}
		if out, err := exec.Command(validator, path).CombinedOutput(); err != nil {
			return "spirv-val", fmt.Errorf("spirv-val: %w: %s", err, out)
		}
	}
	if _, _, err := msl.Compile(module, msl.Options{}); err != nil {
		return "msl", err
	}
	if _, _, err := hlsl.Compile(module, hlsl.Options{}); err != nil {
		return "hlsl", err
	}
	return "", nil
}

// silence the unused-import check while the set is being filled.
