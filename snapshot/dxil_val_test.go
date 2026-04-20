// Package snapshot_test provides DXIL validation tests using DXC dumpbin.
//
// TestDxilValSummary compiles each WGSL shader through the naga pipeline to DXIL
// and validates with dxc.exe -dumpbin. Shaders that fail DXIL compilation are
// expected (DXIL backend is experimental, not all features supported).
//
// Requirements: dxc.exe with dxil.dll from Windows SDK must be available.
package snapshot_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/ir"
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

// dxilExpectedCompileFail contains shaders that are EXPECTED to fail DXIL compilation.
// These verify graceful error handling (no panic, clear error message).
// If a shader in this list starts compiling successfully, the test fails — remove it from the list.
var dxilExpectedCompileFail = map[string]string{
	"abstract-types-const": "no entry points",
	"ptr-deref-test":       "no entry points",
}

// TestDxilValSummary compiles all WGSL shaders to DXIL and validates with DXC dumpbin.
// This is the DXIL equivalent of TestSpirvValBinarySummary.
func TestDxilValSummary(t *testing.T) {
	dxc := dxcPath()
	if dxc == "" {
		t.Skip("dxc.exe not found (install Windows SDK)")
	}

	shaders := loadInputShaders(t, "testdata/in")
	if len(shaders) == 0 {
		t.Fatal("no input shaders found in testdata/in/")
	}

	type result struct {
		name     string
		category string // "pass", "compile_fail", "val_fail"
		message  string
	}

	var results []result
	var passCount, compileFailCount, valFailCount, expectedFailCount int

	opts := dxil.DefaultOptions()

	for i := range shaders {
		shader := &shaders[i]

		// Step 1: Parse WGSL → IR.
		ast, parseErr := parseWGSL(shader.source)
		if parseErr != nil {
			compileFailCount++
			results = append(results, result{shader.name, "compile_fail", parseErr.Error()})
			continue
		}

		module, lowerErr := lowerToIR(ast, shader.source)
		if lowerErr != nil {
			compileFailCount++
			results = append(results, result{shader.name, "compile_fail", lowerErr.Error()})
			continue
		}

		// Process pipeline overrides (replaces ExprOverride → ExprConstant/Literal).
		pipelineConstants := readSPVPipelineConstants(shader.name)
		if len(pipelineConstants) > 0 || len(module.Overrides) > 0 {
			module = ir.CloneModuleForOverrides(module)
			if err := ir.ProcessOverrides(module, pipelineConstants); err != nil {
				compileFailCount++
				results = append(results, result{shader.name, "compile_fail", err.Error()})
				continue
			}
		}

		if len(module.EntryPoints) == 0 {
			if reason, ok := dxilExpectedCompileFail[shader.name]; ok {
				expectedFailCount++
				results = append(results, result{shader.name, "expected_fail", reason})
			} else {
				compileFailCount++
				results = append(results, result{shader.name, "compile_fail", "no entry points"})
			}
			continue
		}

		// Step 2: Compile each entry point to DXIL.
		// dxil.Compile processes EntryPoints[0], so we isolate each one.
		allEPsPass := true
		var firstErr string
		for j := range module.EntryPoints {
			singleEPModule := &ir.Module{
				Types:             module.Types,
				Constants:         module.Constants,
				GlobalVariables:   module.GlobalVariables,
				GlobalExpressions: module.GlobalExpressions,
				Functions:         module.Functions,
				EntryPoints:       []ir.EntryPoint{module.EntryPoints[j]},
				Overrides:         module.Overrides,
				SpecialTypes:      module.SpecialTypes,
			}

			dxilBytes, compileErr := dxil.Compile(singleEPModule, opts)
			if compileErr != nil {
				if firstErr == "" {
					firstErr = compileErr.Error()
				}
				allEPsPass = false
				continue
			}

			// Step 3: Validate with DXC dumpbin.
			tmpFile, tmpErr := os.CreateTemp("", "dxil-val-*.dxil")
			if tmpErr != nil {
				t.Fatalf("create temp file: %v", tmpErr)
			}
			tmpPath := tmpFile.Name()
			_, _ = tmpFile.Write(dxilBytes)
			tmpFile.Close()

			cmd := exec.Command(dxc, "-dumpbin", tmpPath)
			output, valErr := cmd.CombinedOutput()
			os.Remove(tmpPath)

			if valErr != nil {
				if firstErr == "" {
					firstErr = strings.TrimSpace(string(output))
				}
				allEPsPass = false
			}
		}

		if allEPsPass {
			// Regression check: expected-fail shader now passes → remove from list.
			if reason, ok := dxilExpectedCompileFail[shader.name]; ok {
				t.Errorf("shader %q was expected to fail (%s) but now passes DXC — remove from dxilExpectedCompileFail", shader.name, reason)
			}
			passCount++
			results = append(results, result{shader.name, "pass", ""})
		} else if firstErr != "" && strings.Contains(firstErr, "dxc failed") {
			valFailCount++
			results = append(results, result{shader.name, "val_fail", firstErr})
		} else {
			compileFailCount++
			results = append(results, result{shader.name, "compile_fail", firstErr})
		}
	}

	testable := len(results) - expectedFailCount
	t.Logf("=== DXIL Validation Summary (DXC dumpbin) ===")
	t.Logf("Total:        %d (%d testable, %d expected fail)", len(results), testable, expectedFailCount)
	t.Logf("Pass:         %d (%.1f%%)", passCount, pct(passCount, testable))
	t.Logf("Val fail:     %d (%.1f%%)", valFailCount, pct(valFailCount, testable))
	t.Logf("Compile fail: %d (%.1f%%)", compileFailCount, pct(compileFailCount, testable))

	if valFailCount > 0 {
		t.Logf("")
		t.Logf("=== DXC Validation Failures ===")
		for _, r := range results {
			if r.category == "val_fail" {
				t.Logf("  %s: %s", r.name, truncate(r.message, 120))
			}
		}
	}

	if compileFailCount > 0 {
		t.Logf("")
		t.Logf("=== DXIL Compile Failures (expected for unsupported features) ===")
		for _, r := range results {
			if r.category == "compile_fail" {
				t.Logf("  %s: %s", r.name, truncate(r.message, 120))
			}
		}
	}
}

// truncate is defined in snapshot_test.go

// ggProductionCorpora are the gg shader directories that TestDxilValGGProduction
// walks to guard against BUG-DXIL-026-class regressions. These are Vello-derived
// compute pipelines and compositor fragments that exercise patterns absent from
// the testdata/in/ baseline (packed struct UAV loads, workgroup struct arrays
// with TGSM-origin constraints, complex bind group layouts).
//
// Paths are relative to snapshot/. The go.work file puts ../../gg in the
// workspace so this test runs from the same repo checkout.
var ggProductionCorpora = []string{
	"../../gg/internal/gpu/shaders",
	"../../gg/internal/gpucore/shaders",
	"../../gg/internal/gpu/tilecompute/shaders",
}

// TestDxilValGGProduction walks the gg production shader corpora and requires
// every entry point to produce DXIL that DXC's IDxcValidator accepts. This is
// the permanent regression guard for BUG-DXIL-026 — the pre-fix baseline was
// 48 / 57 VALID (9 failures in workgroup struct/array store paths). Skips
// cleanly when the gg repo is not present in the workspace (e.g. CI builds
// that only check out naga).
func TestDxilValGGProduction(t *testing.T) {
	dxc := dxcPath()
	if dxc == "" {
		t.Skip("dxc.exe not found (install Windows SDK)")
	}

	opts := dxil.DefaultOptions()
	var failures []string
	var totalEPs int

	for _, corpusDir := range ggProductionCorpora {
		entries, err := os.ReadDir(corpusDir)
		if err != nil {
			t.Skipf("gg corpus %q unavailable: %v", corpusDir, err)
			return
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

			ast, perr := parseWGSL(string(src))
			if perr != nil {
				failures = append(failures, path+": parse: "+perr.Error())
				continue
			}
			module, lerr := lowerToIR(ast, string(src))
			if lerr != nil {
				failures = append(failures, path+": lower: "+lerr.Error())
				continue
			}
			if len(module.EntryPoints) == 0 {
				continue
			}

			for j := range module.EntryPoints {
				totalEPs++
				singleEP := &ir.Module{
					Types:             module.Types,
					Constants:         module.Constants,
					GlobalVariables:   module.GlobalVariables,
					GlobalExpressions: module.GlobalExpressions,
					Functions:         module.Functions,
					EntryPoints:       []ir.EntryPoint{module.EntryPoints[j]},
					Overrides:         module.Overrides,
					SpecialTypes:      module.SpecialTypes,
				}
				blob, cerr := dxil.Compile(singleEP, opts)
				if cerr != nil {
					failures = append(failures, path+"/"+module.EntryPoints[j].Name+": compile: "+cerr.Error())
					continue
				}
				tmp, terr := os.CreateTemp("", "ggprod-*.dxil")
				if terr != nil {
					t.Fatalf("tmp: %v", terr)
				}
				_, _ = tmp.Write(blob)
				tmp.Close()
				out, verr := exec.Command(dxc, "-dumpbin", tmp.Name()).CombinedOutput()
				os.Remove(tmp.Name())
				if verr != nil {
					failures = append(failures, path+"/"+module.EntryPoints[j].Name+": val: "+truncate(strings.TrimSpace(string(out)), 160))
				}
			}
		}
	}

	t.Logf("gg production DXIL validation: %d entry points, %d failures", totalEPs, len(failures))
	if len(failures) > 0 {
		for _, f := range failures {
			t.Errorf("  %s", f)
		}
	}
}
