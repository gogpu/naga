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
	var passCount, compileFailCount, valFailCount int

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
			compileFailCount++
			results = append(results, result{shader.name, "compile_fail", "no entry points"})
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

	t.Logf("=== DXIL Validation Summary (DXC dumpbin) ===")
	t.Logf("Total:        %d", len(results))
	t.Logf("Pass:         %d (%.1f%%)", passCount, pct(passCount, len(results)))
	t.Logf("Val fail:     %d (%.1f%%)", valFailCount, pct(valFailCount, len(results)))
	t.Logf("Compile fail: %d (%.1f%%)", compileFailCount, pct(compileFailCount, len(results)))

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
