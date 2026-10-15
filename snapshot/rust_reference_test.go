package snapshot_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// errSPIRVPerEntryPoint: Rust's driver writes one SPIR-V module per entry
// point (spv::PipelineOptions) for shaders with [spv] separate_entry_points.
// Our spirv package cannot restrict output to one entry point yet.
var errSPIRVPerEntryPoint = errors.New("per-entry-point SPIR-V output (Rust spv::PipelineOptions) is not implemented")

// referenceTally counts outcomes for one backend.
type referenceTally struct {
	pass, fail, notTarget int
	deviations            map[deviationKind]int
}

func (r *referenceTally) String() string {
	return fmt.Sprintf("%d pass, %d deviation (%d intentional, %d bug, %d unsupported), %d fail, %d not a Rust target",
		r.pass, r.deviations[deviationIntentional]+r.deviations[deviationBug]+r.deviations[deviationUnsupported],
		r.deviations[deviationIntentional], r.deviations[deviationBug], r.deviations[deviationUnsupported],
		r.fail, r.notTarget)
}

var referenceBackends = []struct {
	kind   string
	target rustTargets
	label  string
}{
	{rustOutSPIRV, rustTargetSPIRV, "SPIR-V"},
	{rustOutMSL, rustTargetMETAL, "MSL"},
	{rustOutGLSL, rustTargetGLSL, "GLSL"},
	{rustOutHLSL, rustTargetHLSL, "HLSL"},
}

// TestRustReference compares our output with Rust naga's for every output
// Rust's snapshot driver writes (see expectedRustOutputs). Rust naga is the
// ground truth: an output either matches, or is a recorded deviation
// (rustDeviations), or the test fails. A compile error is a failure unless
// recorded as unsupported. Backends a shader does not target in Rust are
// counted, not compared.
//
// Our own shaders take part only where a Rust-derived reference exists; they
// are reported separately and never count toward Rust parity.
//
// SPIR-V is still compared structurally (compareStructural, counts of
// capabilities, decorations, functions and types) until the exact comparison
// of ADR-006 phase 3 replaces it.
func TestRustReference(t *testing.T) {
	rust := map[string]*referenceTally{}
	own := map[string]*referenceTally{}
	for _, b := range referenceBackends {
		rust[b.kind] = &referenceTally{deviations: map[deviationKind]int{}}
		own[b.kind] = &referenceTally{deviations: map[deviationKind]int{}}
	}
	usedDeviations := map[*rustDeviation]bool{}

	for _, shader := range loadCorpus(t) {
		params := rustParams{targets: rustTargetsWGSLDefault}
		tally := own
		if shader.origin == originRust {
			tally = rust
			var err error
			if params, err = loadRustParams(shader.name); err != nil {
				t.Errorf("%s: %v", shader.name, err)
				continue
			}
			for _, b := range referenceBackends {
				if !params.targets.has(b.target) {
					tally[b.kind].notTarget++
				}
			}
		}

		t.Run(shader.name, func(t *testing.T) {
			module, err := tryCompileToIR(shader.source)
			if err != nil {
				if shader.origin == originRust {
					// Rust parses and validates every corpus shader.
					t.Fatalf("lowering failed: %v", err)
				}
				return
			}
			for _, o := range expectedRustOutputs(shader.name, params, module.EntryPoints) {
				refPath, ok := rustReferencePath(o.kind, o.file)
				if !ok {
					if shader.origin == originRust {
						t.Errorf("%s: Rust reference missing (see TestRustReferenceInventory)", o.file)
						tally[o.kind].fail++
					}
					continue // own shaders: derived references are optional
				}
				t.Run(o.file, func(t *testing.T) {
					dev := findDeviation(shader.name, o.file)
					if dev != nil {
						usedDeviations[dev] = true
					}
					verdict := compareWithRust(t, shader, o, refPath, dev)
					switch verdict.result {
					case resultPass:
						tally[o.kind].pass++
					case resultDeviation:
						tally[o.kind].deviations[verdict.kind]++
						t.Logf("%v deviation: %s", verdict.kind, dev.Reason)
					default:
						tally[o.kind].fail++
						t.Error(verdict.message)
					}
				})
			}
		})
	}

	for i := range rustDeviations {
		d := &rustDeviations[i]
		if !usedDeviations[d] {
			t.Errorf("deviation %s/%s was not exercised: remove it or fix its shader/file", d.Shader, d.File)
		}
	}

	t.Log("=== Rust reference summary (Rust corpus) ===")
	for _, b := range referenceBackends {
		t.Logf("%-7s %v", b.label+":", rust[b.kind])
	}
	t.Log("=== Own shaders vs Rust-derived references (not part of parity) ===")
	for _, b := range referenceBackends {
		if r := own[b.kind]; r.pass+r.fail+len(r.deviations) > 0 {
			t.Logf("%-7s %d pass, %d deviation, %d fail", b.label+":", r.pass,
				r.deviations[deviationIntentional]+r.deviations[deviationBug]+r.deviations[deviationUnsupported], r.fail)
		}
	}
}

// compareWithRust produces our output for o, compares it with the reference
// and judges the result. It may skip the test when a required external tool
// is missing (see lookTool).
func compareWithRust(t *testing.T, shader shaderFile, o rustOutput, refPath string, dev *rustDeviation) referenceVerdict {
	t.Helper()
	refBytes, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatal(err)
	}
	reference := string(refBytes)

	prepared, err := prepareForBackend(shader.source, shader.name)
	if err != nil {
		return judgeReference(dev, err, false, "")
	}

	if o.kind == rustOutSPIRV {
		disassembler := testtools.Look(t, "spirv-dis", testtools.InstallSPIRVTools)
		ours, compileErr := ourSPIRVDisassembly(prepared, shader.name, o, disassembler)
		if compileErr != nil {
			return judgeReference(dev, compileErr, false, "")
		}
		diffs := compareStructural(ours, reference)
		summary := strings.Join(diffs, "\n")
		verdict := judgeReference(dev, nil, len(diffs) > 0, referenceFingerprint(reference, summary))
		if verdict.result == resultFail && len(diffs) > 0 {
			verdict.message += "\n" + summary
		}
		return verdict
	}

	ours, compileErr := ourTextOutput(prepared, shader.name, o)
	if compileErr != nil {
		return judgeReference(dev, compileErr, false, "")
	}
	expected := strings.ReplaceAll(reference, "\r\n", "\n")
	actual := strings.ReplaceAll(ours, "\r\n", "\n")
	verdict := judgeReference(dev, nil, expected != actual, referenceFingerprint(expected, actual))
	if verdict.result == resultFail && expected != actual {
		verdict.message += "\n" + diffStrings(expected, actual)
	}
	return verdict
}

// prepareForBackend mirrors Rust's driver (naga/tests/naga/snapshots.rs),
// where every backend first runs process_overrides with the shader's
// pipeline_constants. Our counterpart is ir.ProcessOverrides; where it still
// differs from Rust, rustDeviations records the effect (BUG-IR-OVERRIDES-001).
//
// Rust's driver also compacts the module with KeepUnused::No before the
// backends. We do not: our compiler has no such step, so users get the
// uncompacted output, and ir.CompactUnused is a partial, test-only helper
// (BUG-IR-COMPACT-001). Where the missing compaction changes an output,
// rustDeviations records it.
//
// Each call lowers the source afresh: preparing one backend's module must not
// affect another's, and ir.CloneModuleForOverrides does not isolate them
// (it shares nested statement data that ir.ProcessOverrides rewrites in place,
// BUG-IR-CLONE-001).
func prepareForBackend(source, shader string) (*ir.Module, error) {
	prepared, err := tryCompileToIR(source)
	if err != nil {
		return nil, err
	}
	if err := ir.ProcessOverrides(prepared, readSPVPipelineConstants(shader)); err != nil {
		return nil, fmt.Errorf("process overrides: %w", err)
	}
	return prepared, nil
}

// ourSPIRVDisassembly compiles SPIR-V with the shader's Rust options and
// disassembles it.
func ourSPIRVDisassembly(module *ir.Module, shader string, o rustOutput, disassembler string) (string, error) {
	if o.entryPoint != "" {
		return "", errSPIRVPerEntryPoint
	}
	binary, err := spirv.NewBackend(readRustSPVConfig(shader)).Compile(module)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "naga-ref-*.spv")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	out, err := exec.Command(disassembler, "--no-header", tmp.Name()).Output()
	if err != nil {
		return "", fmt.Errorf("spirv-dis: %w", err)
	}
	return string(out), nil
}

// ourTextOutput compiles MSL, HLSL or GLSL with the shader's Rust options.
// Pipeline constants are already applied by prepareForBackend.
func ourTextOutput(module *ir.Module, shader string, o rustOutput) (string, error) {
	switch o.kind {
	case rustOutMSL:
		opts := readRustMSLConfig(shader)
		opts.PipelineConstants = nil
		code, _, err := msl.Compile(module, opts)
		return code, err
	case rustOutHLSL:
		opts, err := rustHLSLOptions(shader)
		if err != nil {
			return "", err
		}
		code, _, err := hlsl.Compile(module, opts)
		return code, err
	case rustOutGLSL:
		opts := readGLSLConfig(shader).toOptions()
		opts.PipelineConstants = nil
		opts.EntryPoint = o.entryPoint
		code, _, err := glsl.Compile(module, opts)
		return code, err
	default:
		return "", fmt.Errorf("unexpected output kind %q (%s)", o.kind, filepath.Base(o.file))
	}
}
