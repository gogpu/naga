// Package snapshot_test provides DXIL validation tests.
//
// TestDxilValSummary compiles every entry point of every corpus shader to
// DXIL and validates it with IDxcValidator (dxil.Validate at ValidateFull),
// the validator the D3D12 runtime runs. A pass is IDxcValidator's verdict, not
// a `dxc -dumpbin` parse: dumpbin only disassembles, so it never runs the
// container cross-checks (PSV0 against the module, signatures, ViewID state)
// that make a pipeline creation fail.
//
// Requirements: IDxcValidator (dxil.dll from the Windows SDK) must load.
package snapshot_test

import (
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/internal/dxcvalidator"
	"github.com/gogpu/naga/internal/testtools"
)

// dxcPath returns the path to dxc.exe from Windows SDK, or empty if not found.
func dxcPath() string {
	// Check Windows SDK location first (has dxil.dll for validation).
	sdkPath := `C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64\dxc.exe`
	if _, err := os.Stat(sdkPath); err == nil {
		return sdkPath
	}
	// Fall back to PATH.
	p, err := exec.LookPath("dxc")
	if err != nil {
		return ""
	}
	return p
}

// idxcTool is the tool name NAGA_REQUIRE_TOOLS uses for IDxcValidator.
const idxcTool = "dxil.dll"

// installIDxc tells where IDxcValidator comes from.
const installIDxc = "IDxcValidator needs Windows and dxil.dll from the Windows SDK"

// requireIDxcValidator skips the test where IDxcValidator cannot run (not
// Windows, or dxil.dll does not load) and fails it there when
// NAGA_REQUIRE_TOOLS lists dxil.dll. dxil.Validate degrades to the bitcode
// check without the validator, which would count every structurally sound
// blob as a pass.
func requireIDxcValidator(t *testing.T) {
	t.Helper()
	err := dxcvalidator.Run(func(*dxcvalidator.Validator) error { return nil })
	if err == nil {
		return
	}
	if errors.Is(err, dxcvalidator.ErrUnsupported) {
		testtools.Missing(t, idxcTool, installIDxc)
		return
	}
	testtools.Missing(t, idxcTool, installIDxc+": "+err.Error())
}

// knownFailures holds the entry points a gate expects to fail today, each with
// the card that tracks it. A failure outside the list fails the test, and so
// does a listed entry that no longer fails: a fixed entry leaves the list.
type knownFailures struct {
	name string            // the list's variable name, for messages
	list map[string]string // key → card
	seen map[string]bool
}

func newKnownFailures(name string, list map[string]string) *knownFailures {
	return &knownFailures{name: name, list: list, seen: make(map[string]bool)}
}

// fail records a failure of key and reports whether it was expected.
func (k *knownFailures) fail(t *testing.T, key, msg string) bool {
	t.Helper()
	card, known := k.list[key]
	if !known {
		return false
	}
	k.seen[key] = true
	t.Logf("known failure %s (%s): %s", key, card, truncate(msg, 200))
	return true
}

// checkFixed fails the test for every listed key that did not fail.
func (k *knownFailures) checkFixed(t *testing.T) {
	t.Helper()
	keys := make([]string, 0, len(k.list))
	for key := range k.list {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !k.seen[key] {
			t.Errorf("%s passes now: remove it from %s and close %s", key, k.name, k.list[key])
		}
	}
}

// dxilExpectedCompileFail contains shaders that are EXPECTED to fail DXIL compilation.
// These verify graceful error handling (no panic, clear error message).
// If a shader in this list starts compiling successfully, the test fails — remove it from the list.
var dxilExpectedCompileFail = map[string]string{
	"abstract-types-const": "no entry points",
	"ptr-deref-test":       "no entry points",
}

// dxilKnownFailures are the corpus entry points ("shader/entry", or "shader"
// when the shader does not get as far as its entry points) that DXIL does not
// produce a valid blob for today — a parse or dxil.Compile error, or an
// IDxcValidator rejection — each with its card.
var dxilKnownFailures = map[string]string{
	// The WGSL front end does not parse these shaders yet.
	"cooperative-matrix":                   "PARITY-UPSTREAM-001",
	"cooperative-matrix-mixed-accumulator": "PARITY-UPSTREAM-001",
	"debug-printf":                         "PARITY-UPSTREAM-001",
	"ray-tracing-pipeline":                 "PARITY-UPSTREAM-001",

	// Refused by dxil.Compile: no DXIL lowering yet, or no D3D semantic.
	// Native 16-bit integers (-enable-16bit-types at SM 6.2) are not emitted;
	// dxil.Compile refuses them (errInt16).
	"int16/main": "P2-WGSL-102",
	"8820-multiple-local-invocation-index-id/compute1": "P2-DXIL-093",
	"barycentrics/fs_main":                             "P2-DXIL-073",
	"barycentrics/fs_main_no_perspective":              "P2-DXIL-073",
	"draw-index/vertex":                                "P2-DXIL-073",
	"per-vertex/fs_main":                               "P2-DXIL-074",
	"mesh-shader/fs_main":                              "P2-DXIL-074",
	"texture-external/fragment_main":                   "P1-DXIL-056",
	"texture-external/vertex_main":                     "P1-DXIL-056",

	// dx.op.createHandle at SM 6.6, where the validator wants
	// createHandleFromBinding.
	"atomicCompareExchange-int64/test_atomic_compare_exchange_i64": "P1-DXIL-090",
	"atomicCompareExchange-int64/test_atomic_compare_exchange_u64": "P1-DXIL-090",
	"atomicOps-int64/cs_main":                                      "P1-DXIL-090",
	"atomicOps-int64-min-max/cs_main":                              "P1-DXIL-090",
	"atomicTexture-int64/cs_main":                                  "P1-DXIL-090",

	// atomicBinOp.f32: DXIL has no float atomics; the backend should refuse.
	"atomicOps-float32/cs_main": "P2-API-005",

	// Input D3D cannot take (register ranges that overlap without a
	// binding map, @location past the 32 signature rows) gives a blob
	// instead of a compile error.
	"binding-buffer-arrays/main":       "P1-DXIL-094",
	"msl-vpt-formats-x1/render_vertex": "P1-DXIL-094",
	"msl-vpt-formats-x2/render_vertex": "P1-DXIL-094",
	"msl-vpt-formats-x3/render_vertex": "P1-DXIL-094",
	"msl-vpt-formats-x4/render_vertex": "P1-DXIL-094",

	// Input signature part does not match the module.
	"f16-native/test_component_access":   "P1-DXIL-095",
	"f16-polyfill/test_component_access": "P1-DXIL-095",

	// Shader flags: TypedUAVLoadAdditionalFormats missing.
	"image/main": "P1-DXIL-096",

	"int64/main":                      "P1-DXIL-080",
	"ray-query/main":                  "P1-DXIL-075",
	"ray-query-no-init-tracking/main": "P1-DXIL-075",

	// Mesh and amplification shaders.
	"mesh-shader/ts_main":                 "P2-DXIL-097",
	"mesh-shader/ts_divergent":            "P2-DXIL-097",
	"mesh-shader/ms_main":                 "P2-DXIL-097",
	"mesh-shader/ms_no_ts":                "P2-DXIL-097",
	"mesh-shader/ms_divergent":            "P2-DXIL-097",
	"mesh-shader-empty/ts_main":           "P2-DXIL-097",
	"mesh-shader-empty/ms_main":           "P2-DXIL-097",
	"mesh-shader-lines/ts_main":           "P2-DXIL-097",
	"mesh-shader-lines/ms_main":           "P2-DXIL-097",
	"mesh-shader-points/ts_main":          "P2-DXIL-097",
	"mesh-shader-points/ms_main":          "P2-DXIL-097",
	"primitive-index-mesh-shader/ms_main": "P2-DXIL-097",
}

// TestDxilValSummary compiles every entry point of every corpus shader to DXIL
// and validates it with IDxcValidator. This is the DXIL equivalent of
// TestSpirvValBinarySummary.
func TestDxilValSummary(t *testing.T) {
	requireIDxcValidator(t)

	shaders := loadCorpus(t)
	known := newKnownFailures("dxilKnownFailures", dxilKnownFailures)

	var totalEPs, passCount, compileFailCount, valFailCount, knownCount, expectedFailCount int
	var unexpected []string
	fail := func(key, msg string) {
		if known.fail(t, key, msg) {
			knownCount++
			return
		}
		unexpected = append(unexpected, key+": "+truncate(msg, 300))
	}

	opts := dxil.DefaultOptions()
	// Immediate data at b0 in space 0 where the Rust parameters give no
	// immediates_target.
	opts.ImmediatesTarget = &dxil.BindTarget{}

	for i := range shaders {
		shader := &shaders[i]

		module, parseErr := parseWGSL(shader.source)
		if parseErr != nil {
			compileFailCount++
			fail(shader.name, "parse: "+parseErr.Error())
			continue
		}

		// Process pipeline overrides (replaces ExprOverride → ExprConstant/Literal).
		processed, _, err := naga.ProcessOverrides(module, unvalidated, naga.OverrideOptions{Constants: readSPVPipelineConstants(shader.name)})
		if err != nil {
			compileFailCount++
			fail(shader.name, "overrides: "+err.Error())
			continue
		}
		module = processed

		if len(module.EntryPoints) == 0 {
			if _, ok := dxilExpectedCompileFail[shader.name]; ok {
				expectedFailCount++
			} else {
				compileFailCount++
				fail(shader.name, "no entry points")
			}
			continue
		}
		if reason, ok := dxilExpectedCompileFail[shader.name]; ok {
			t.Errorf("shader %q was expected to have no entry points (%s) but has %d — remove it from dxilExpectedCompileFail",
				shader.name, reason, len(module.EntryPoints))
		}

		// The resource placement of the shader's Rust HLSL parameters.
		shaderOpts := withRustBindings(t, shader.name, opts)
		for j := range module.EntryPoints {
			ep := &module.EntryPoints[j]
			totalEPs++
			key := shader.name + "/" + ep.Name

			epOpts := shaderOpts
			epOpts.EntryPoint = &backend.EntryPointSelector{Stage: ep.Stage, Name: ep.Name}
			blob, compileErr := dxilAtNeededModel(module, epOpts)
			if compileErr != nil {
				compileFailCount++
				fail(key, "compile: "+compileErr.Error())
				continue
			}
			if verr := dxil.Validate(blob, dxil.ValidateFull); verr != nil {
				valFailCount++
				fail(key, "validate: "+verr.Error())
				continue
			}
			passCount++
		}
	}

	t.Logf("=== DXIL Validation Summary (IDxcValidator) ===")
	t.Logf("Shaders:      %d (%d with no entry point, expected)", len(shaders), expectedFailCount)
	t.Logf("Entry points: %d", totalEPs)
	t.Logf("Pass:         %d (%.1f%%)", passCount, pct(passCount, totalEPs))
	t.Logf("Val fail:     %d", valFailCount)
	t.Logf("Compile fail: %d (shader-level parse failures count once)", compileFailCount)
	t.Logf("Known:        %d of %d listed in dxilKnownFailures", knownCount, len(dxilKnownFailures))

	known.checkFixed(t)
	for _, u := range unexpected {
		t.Errorf("new DXIL failure (not in dxilKnownFailures): %s", u)
	}
}

// truncate is defined in snapshot_test.go

// ggProductionCorpora are the gg shader directories that TestDxilValGGProduction
// walks to guard against BUG-DXIL-026-class regressions. These are Vello-derived
// compute pipelines and compositor fragments that exercise patterns absent from
// the corpus baseline (packed struct UAV loads, workgroup struct arrays
// with TGSM-origin constraints, complex bind group layouts).
//
// They are a pinned copy of gg's shaders (testdata/upstream, see its
// PROVENANCE.md): tests read only this repository. Paths are relative to
// snapshot/.
var ggProductionCorpora = []string{
	ggUpstreamDir + "/internal/gpu/shaders",
	ggUpstreamDir + "/internal/gpu/tilecompute/shaders",
}

// ggUpstreamDir is the vendored gogpu/gg tree.
const ggUpstreamDir = "../testdata/upstream/github.com/gogpu/gg"

// ggKnownDXILFailures are the gg entry points (path in the gg tree / entry
// point) whose DXIL the validator rejects today, each with its card. The test
// fails when one of them validates, so a fixed entry leaves this list.
var ggKnownDXILFailures = map[string]string{
	// The vendored blend.wgsl is invalid WGSL (a float select condition),
	// which the validator rejects since P1-WGSL-095; gg fixes the shader.
	"internal/gpu/shaders/blend.wgsl/fs_main": "gogpu/gg:P1-BLEND-003",
}

// TestDxilValGGProduction walks the gg production shader corpora and requires
// every entry point to produce DXIL that DXC's IDxcValidator accepts. This is
// the permanent regression guard for BUG-DXIL-026 — the pre-fix baseline was
// 48 / 57 VALID (9 failures in workgroup struct/array store paths).
func TestDxilValGGProduction(t *testing.T) {
	requireIDxcValidator(t)

	opts := dxil.DefaultOptions()
	known := newKnownFailures("ggKnownDXILFailures", ggKnownDXILFailures)
	var failures []string
	var totalEPs, passCount int

	for _, corpusDir := range ggProductionCorpora {
		entries, err := os.ReadDir(corpusDir)
		if err != nil {
			t.Fatalf("gg corpus %q: %v", corpusDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wgsl") {
				continue
			}
			path := corpusDir + "/" + entry.Name()
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				t.Fatalf("read %s: %v", path, rerr)
			}

			module, perr := parseWGSL(string(src))
			if perr != nil {
				failures = append(failures, path+": parse: "+perr.Error())
				continue
			}

			for j := range module.EntryPoints {
				totalEPs++
				epOpts := opts
				epOpts.EntryPoint = &backend.EntryPointSelector{
					Stage: module.EntryPoints[j].Stage, Name: module.EntryPoints[j].Name,
				}
				key := strings.TrimPrefix(path, ggUpstreamDir+"/") + "/" + module.EntryPoints[j].Name
				fail := func(msg string) {
					if !known.fail(t, key, msg) {
						failures = append(failures, key+": "+truncate(msg, 300))
					}
				}
				blob, cerr := dxilAtNeededModel(module, epOpts)
				if cerr != nil {
					fail("compile: " + cerr.Error())
					continue
				}
				if verr := dxil.Validate(blob, dxil.ValidateFull); verr != nil {
					fail("validate: " + verr.Error())
					continue
				}
				passCount++
			}
		}
	}

	t.Logf("gg production DXIL (IDxcValidator): %d entry points, %d valid, %d known failures, %d new failures",
		totalEPs, passCount, len(known.seen), len(failures))
	known.checkFixed(t)
	for _, f := range failures {
		t.Errorf("new gg DXIL failure (not in ggKnownDXILFailures): %s", f)
	}
}
