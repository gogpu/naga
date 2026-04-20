package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gogpu/naga/internal/dxcvalidator"
	"github.com/gogpu/naga/ir"
)

// outcome categorizes a single entry point's fate.
type outcome struct {
	shader string
	ep     string
	stage  ir.ShaderStage
	kind   string // VALID / INVALID / COMPILE_FAIL / SKIP / VALIDATE_ERROR
	detail string
}

// compiled holds one shader file's compile results: per-entry-point blobs
// alongside their names and stages, plus any compile-time error.
type compiled struct {
	shader string
	blobs  [][]byte
	names  []string
	stages []ir.ShaderStage
	err    error
	// skipReason is set when the shader should be reported as SKIP rather
	// than compiled/validated (e.g. targets field excludes HLSL/DXIL).
	skipReason string
}

// runCorpus walks a directory of .wgsl files, compiles each through naga,
// validates every entry point, and prints a summary plus per-shader detail.
// When `quiet` is true only the summary is printed. When `save` is non-empty
// the same output is also written to that path.
func runCorpus(dir, save string, quiet bool, stdout, stderr io.Writer) int {
	shaderFiles, err := listShaders(dir)
	if err != nil {
		fmt.Fprintf(stderr, "dxilval: read dir %s: %v\n", dir, err)
		return ExitFail
	}
	fmt.Fprintf(stderr, "=== Corpus: %d .wgsl files in %s ===\n", len(shaderFiles), dir)

	precompiled := precompileAll(shaderFiles)

	var (
		outcomes   []outcome
		toValidate []compiled
	)
	for _, c := range precompiled {
		switch {
		case c.skipReason != "":
			outcomes = append(outcomes, outcome{shader: c.shader, kind: "SKIP", detail: c.skipReason})
		case c.err != nil:
			outcomes = append(outcomes, outcome{shader: c.shader, kind: "COMPILE_FAIL", detail: c.err.Error()})
		case len(c.blobs) == 0:
			outcomes = append(outcomes, outcome{shader: c.shader, kind: "SKIP", detail: "no entry points"})
		default:
			toValidate = append(toValidate, c)
		}
	}
	logPreCompile(stderr, outcomes, toValidate)

	validated, runErr := validateCompiled(toValidate, stderr)
	if runErr != nil {
		fmt.Fprintf(stderr, "dxilval: validator init: %v\n", runErr)
		return ExitFail
	}
	outcomes = append(outcomes, validated...)

	report := renderReport(outcomes, quiet)
	fmt.Fprint(stdout, report)
	if save != "" {
		if err := os.WriteFile(save, []byte(report), 0o644); err != nil {
			fmt.Fprintf(stderr, "dxilval: save %s: %v\n", save, err)
			return ExitFail
		}
		fmt.Fprintf(stderr, "saved to %s\n", save)
	}

	counts := tally(outcomes)
	if counts["INVALID"] > 0 || counts["VALIDATE_ERROR"] > 0 {
		return ExitValidation
	}
	return ExitOK
}

// listShaders returns sorted .wgsl paths inside dir.
func listShaders(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".wgsl") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// precompileAll compiles every shader file outside the validator worker
// thread (naga is pure Go, no COM involved). Cleanly separates compile-fail
// tally from validator-fail tally. Shaders whose sibling .toml restricts
// `targets` to backends other than HLSL/DXIL are marked skipReason instead
// of compiled — they are not meant to validate against dxil.dll. This
// mirrors Rust naga's test gating where `targets = "METAL"` excludes the
// shader from HLSL golden comparison.
func precompileAll(shaderFiles []string) []compiled {
	out := make([]compiled, 0, len(shaderFiles))
	for _, sf := range shaderFiles {
		if skipReason, skip := shouldSkipForTargets(sf); skip {
			out = append(out, compiled{
				shader:     filepath.Base(sf),
				skipReason: skipReason,
			})
			continue
		}
		blobs, names, stages, cerr := compileWGSL(sf, "")
		out = append(out, compiled{
			shader: filepath.Base(sf),
			blobs:  blobs,
			names:  names,
			stages: stages,
			err:    cerr,
		})
	}
	return out
}

// shouldSkipForTargets reads the sibling .toml of a .wgsl path and returns
// a skip reason when the shader declares backend-specific options that only
// apply to non-DXIL backends and would make DXIL validation meaningless.
//
// Current skip triggers:
//
//   - msl_pipeline.vertex_pulling_transform = true
//     An MSL-specific transform that rewrites per-vertex @location inputs
//     into buffer reads. Without it, a shader with 40+ vertex attributes
//     (like the msl-vpt-formats-xN tests) exceeds DXIL's 32-row signature
//     limit. Rust naga applies this transform only for the MSL backend;
//     DXIL has no equivalent (HLSL/D3D12 uses input layouts instead).
//
//   - WGSL source contains `atomic<f32>` / `atomic<f64>`
//     Floating-point atomics are a WGSL feature that DXIL has no standard
//     lowering for. DXC's HLSL `InterlockedAdd` only supports integer
//     types; float atomics require NVAPI vendor extensions that are not
//     part of standard DXIL. Rust naga's own TOML for atomicOps-float32
//     lists `targets = "SPIRV | METAL | WGSL"` — no HLSL golden exists
//     because the feature simply cannot round-trip through DXIL.
//
// A blanket `targets = "METAL"` check was considered and rejected: many
// shaders marked "SPIRV" or "METAL | SPIRV | WGSL" nevertheless compile
// to valid DXIL, and skipping them would hide real progress. Rust naga's
// `targets` field determines which golden files exist, not which backends
// the shader *can* compile to. Content-based triggers are preferred.
func shouldSkipForTargets(wgslPath string) (string, bool) {
	// Content-based trigger: atomic<f32>/atomic<f64> has no DXIL lowering.
	if wgslData, err := os.ReadFile(wgslPath); err == nil {
		src := string(wgslData)
		if strings.Contains(src, "atomic<f32>") || strings.Contains(src, "atomic<f64>") {
			return "atomic<f32>/<f64> has no standard DXIL lowering (NVAPI-only)", true
		}
	}

	tomlPath := strings.TrimSuffix(wgslPath, ".wgsl") + ".toml"
	data, err := os.ReadFile(tomlPath)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		// Strip any inline comment.
		if idx := strings.Index(trimmed, "#"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if !strings.HasPrefix(trimmed, "vertex_pulling_transform") {
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			continue
		}
		val := strings.TrimSpace(trimmed[eq+1:])
		if val == "true" {
			return "msl_pipeline.vertex_pulling_transform = true (MSL-only)", true
		}
	}
	return "", false
}

func logPreCompile(stderr io.Writer, outcomes []outcome, toValidate []compiled) {
	compileFail := 0
	skip := 0
	for _, o := range outcomes {
		switch o.kind {
		case "COMPILE_FAIL":
			compileFail++
		case "SKIP":
			skip++
		}
	}
	totalEPs := 0
	for _, c := range toValidate {
		totalEPs += len(c.blobs)
	}
	fmt.Fprintf(stderr, "Pre-compile: %d compile-fail, %d skip, %d shaders to validate (%d EPs)\n",
		compileFail, skip, len(toValidate), totalEPs)
}

// validateCompiled runs every entry point in toValidate through IDxcValidator
// inside one fresh OS thread. Per-shader stderr progress lets us identify
// which shader provoked an AV if dxil.dll aborts the process.
func validateCompiled(toValidate []compiled, stderr io.Writer) ([]outcome, error) {
	var validated []outcome
	runErr := dxcvalidator.Run(func(v *dxcvalidator.Validator) error {
		for _, c := range toValidate {
			for i, blob := range c.blobs {
				stage := c.stages[i]
				name := c.names[i]
				fmt.Fprintf(stderr, "validating %s / %s (stage=%s)...\n", c.shader, name, stageString(stage))

				res, vErr := v.Validate(blob)
				validated = append(validated, classify(c.shader, name, stage, res, vErr))
			}
		}
		return nil
	})
	return validated, runErr
}

// classify converts one Validate result into an outcome row.
func classify(shader, ep string, stage ir.ShaderStage, res dxcvalidator.Result, vErr error) outcome {
	o := outcome{shader: shader, ep: ep, stage: stage}
	switch {
	case vErr != nil:
		o.kind = "VALIDATE_ERROR"
		o.detail = vErr.Error()
	case res.OK:
		o.kind = "VALID"
	default:
		o.kind = fmt.Sprintf("INVALID-0x%08x", res.Status)
		o.detail = firstLine(res.Error)
	}
	return o
}

// tally bins outcomes into summary counts. INVALID-0xNNN variants collapse
// into a single "INVALID" key.
func tally(outcomes []outcome) map[string]int {
	counts := map[string]int{}
	for _, o := range outcomes {
		k := o.kind
		if strings.HasPrefix(k, "INVALID-") {
			k = "INVALID"
		}
		counts[k]++
	}
	return counts
}

// renderReport builds the human-readable text summary plus the per-shader
// detail block (omitted when quiet).
func renderReport(outcomes []outcome, quiet bool) string {
	counts := tally(outcomes)

	var b strings.Builder
	fmt.Fprintln(&b, "=== Validation summary ===")
	fmt.Fprintf(&b, "Total entry points : %d\n", len(outcomes))
	fmt.Fprintf(&b, "  VALID           : %d\n", counts["VALID"])
	fmt.Fprintf(&b, "  INVALID         : %d\n", counts["INVALID"])
	fmt.Fprintf(&b, "  COMPILE_FAIL    : %d\n", counts["COMPILE_FAIL"])
	fmt.Fprintf(&b, "  SKIP            : %d\n", counts["SKIP"])
	fmt.Fprintf(&b, "  VALIDATE_ERROR  : %d\n", counts["VALIDATE_ERROR"])

	if !quiet {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "=== Per-shader outcomes ===")
		for _, o := range outcomes {
			label := "-"
			if o.ep != "" {
				label = stageString(o.stage) + "/" + o.ep
			}
			fmt.Fprintf(&b, "%-44s %-14s %-22s %s\n",
				o.shader, label, o.kind, truncate(o.detail, 80))
		}
	}
	return b.String()
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
