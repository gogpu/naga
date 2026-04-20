// Package snapshot_test — DXC golden parity tests for the DXIL backend.
//
// SPIR-V-equivalent flow: generate DXC golden DXIL from naga's HLSL output,
// then compare naga's direct DXIL output (via dxc -dumpbin LLVM IR text) line
// by line against the golden. Diff = 0 = success.
//
// References live under `snapshot/testdata/reference/dxil_dxc/<shader>_<stage>_<entry>.ll`.
//
// To regenerate after intentional emit changes:
//
//	UPDATE_DXC_GOLDEN=1 GOROOT="/c/Program Files/Go" go test \
//	    -run TestDxilDxcGolden -count=1 ./snapshot/
//
// The generation path requires `dxc.exe` in PATH (Windows SDK preferred for
// dxil.dll validation). The comparison path also requires `dxc.exe` for
// `-dumpbin` disassembly. Both skip cleanly when dxc is unavailable so this
// test is dev-machine focused (matches the existing TestDxilValSummary skip
// behavior).
package snapshot_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
)

const dxcGoldenDir = "testdata/reference/dxil_dxc"

// dxilDxcSkipShaders lists shaders excluded from DXC golden parity (e.g.,
// known DXIL backend gaps that produce unparseable blobs). These are tracked
// separately by TestDxilValSummary; excluding them here keeps the parity
// test focused on shaders both pipelines can compile.
var dxilDxcSkipShaders = map[string]string{
	// BUG-DXIL-029 (helper inline pass) — fragment shaders with helper calls.
	"7048-multiple-dynamic-3":       "BUG-DXIL-029 inline gap",
	"access":                        "BUG-DXIL-029 inline gap",
	"bounds-check-restrict":         "BUG-DXIL-029 inline gap",
	"bounds-check-zero":             "BUG-DXIL-029 inline gap",
	"hlsl_mat_cx2":                  "BUG-DXIL-029 inline gap",
	"hlsl_mat_cx3":                  "BUG-DXIL-029 inline gap",
	"index-by-value":                "BUG-DXIL-029 inline gap",
	"pointer-function-arg":          "BUG-DXIL-029 inline gap",
	"pointer-function-arg-restrict": "BUG-DXIL-029 inline gap",
	"pointer-function-arg-rzsw":     "BUG-DXIL-029 inline gap",
	"texture-arg":                   "BUG-DXIL-029 inline gap",
	"texture-external":              "BUG-DXIL-029 inline gap",
	"int64":                         "EXTRACTVAL invalid struct index",
	"policy-mix":                    "ICE Cast incompatible",
	// Backend gaps:
	"ray-query":                  "ray-query expressions not implemented",
	"ray-query-no-init-tracking": "ray-query expressions not implemented",
	"f16":                        "f16 multi-feature blocking",
	"globals":                    "complex globals access patterns",
	"arrays":                     "array record format gaps",
}

// TestDxilDxcGolden is the parity test. For each shader it loads the saved
// reference (LLVM IR text from dxc -dumpbin of DXC-compiled HLSL), runs naga
// DXIL, dxc -dumpbin, normalizes both, and reports any divergence. The set
// of shaders that has zero diff is the metric tracked over time.
func TestDxilDxcGolden(t *testing.T) {
	dxc := dxcPath()
	if dxc == "" {
		t.Skip("dxc.exe not found (install Windows SDK)")
	}
	updateGolden := os.Getenv("UPDATE_DXC_GOLDEN") == "1"

	if updateGolden {
		if err := os.MkdirAll(dxcGoldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	shaders := loadInputShaders(t, "testdata/in")
	if len(shaders) == 0 {
		t.Fatal("no input shaders found in testdata/in/")
	}

	var passCount, diffCount, skipCount, missingGoldenCount int
	var totalMatched, totalLines int
	for i := range shaders {
		shader := &shaders[i]
		if reason, ok := dxilDxcSkipShaders[shader.name]; ok {
			skipCount++
			t.Logf("SKIP %s: %s", shader.name, reason)
			continue
		}

		ast, parseErr := parseWGSL(shader.source)
		if parseErr != nil {
			skipCount++
			continue
		}
		module, lowerErr := lowerToIR(ast, shader.source)
		if lowerErr != nil {
			skipCount++
			continue
		}
		if len(module.EntryPoints) == 0 {
			skipCount++
			continue
		}
		if len(module.Overrides) > 0 {
			module = ir.CloneModuleForOverrides(module)
			if err := ir.ProcessOverrides(module, nil); err != nil {
				skipCount++
				continue
			}
		}

		for j := range module.EntryPoints {
			ep := module.EntryPoints[j]
			single := singleEntryPointModule(module, j)
			goldenFile := filepath.Join(dxcGoldenDir,
				fmt.Sprintf("%s_%s_%s.ll", shader.name, stageShortName(ep.Stage), ep.Name))

			if updateGolden {
				if err := writeDxcGolden(dxc, single, ep, goldenFile); err != nil {
					t.Logf("FAIL UPDATE %s/%s: %v", shader.name, ep.Name, err)
					diffCount++
					continue
				}
				passCount++
				continue
			}

			matched, total, diff, err := compareToGolden(dxc, single, ep, goldenFile)
			if err != nil {
				if os.IsNotExist(err) {
					missingGoldenCount++
					continue
				}
				t.Logf("ERROR %s/%s: %v", shader.name, ep.Name, err)
				diffCount++
				continue
			}
			totalMatched += matched
			totalLines += total
			if diff == "" {
				passCount++
			} else {
				diffCount++
				if testing.Verbose() {
					t.Logf("DIFF %s/%s (%d/%d lines match):\n%s",
						shader.name, ep.Name, matched, total, truncate(diff, 400))
				}
			}
		}
	}

	parityPct := 0.0
	if totalLines > 0 {
		parityPct = 100.0 * float64(totalMatched) / float64(totalLines)
	}

	t.Logf("")
	t.Logf("=== DXC Golden Parity Summary ===")
	t.Logf("Pass (diff=0):  %d", passCount)
	t.Logf("Diff:           %d", diffCount)
	t.Logf("Missing golden: %d (run with UPDATE_DXC_GOLDEN=1 to create)", missingGoldenCount)
	t.Logf("Skipped:        %d", skipCount)
	t.Logf("Line parity:    %d / %d  (%.1f%%)", totalMatched, totalLines, parityPct)

	if !updateGolden && diffCount > 0 {
		t.Logf("\nTip: review diffs above. To update goldens after intentional emit changes:")
		t.Logf("  UPDATE_DXC_GOLDEN=1 GOROOT=\"/c/Program Files/Go\" go test -run TestDxilDxcGolden -count=1 ./snapshot/")
	}
}

// singleEntryPointModule clones a module keeping only the j-th entry point.
func singleEntryPointModule(mod *ir.Module, j int) *ir.Module {
	return &ir.Module{
		Types:             mod.Types,
		Constants:         mod.Constants,
		GlobalVariables:   mod.GlobalVariables,
		GlobalExpressions: mod.GlobalExpressions,
		Functions:         mod.Functions,
		EntryPoints:       []ir.EntryPoint{mod.EntryPoints[j]},
		Overrides:         mod.Overrides,
		SpecialTypes:      mod.SpecialTypes,
	}
}

func stageShortName(s ir.ShaderStage) string {
	switch s {
	case ir.StageVertex:
		return "vs"
	case ir.StageFragment:
		return "fs"
	case ir.StageCompute:
		return "cs"
	case ir.StageMesh:
		return "ms"
	case ir.StageTask:
		return "as"
	}
	return "unk"
}

func stageDxcProfile(s ir.ShaderStage) string {
	switch s {
	case ir.StageVertex:
		return "vs_6_0"
	case ir.StageFragment:
		return "ps_6_0"
	case ir.StageCompute:
		return "cs_6_0"
	case ir.StageMesh:
		return "ms_6_5"
	case ir.StageTask:
		return "as_6_5"
	}
	return ""
}

// writeDxcGolden generates the reference golden by going naga HLSL → DXC → DXIL,
// then dxc -dumpbin → normalized LLVM IR text saved to goldenFile.
func writeDxcGolden(dxc string, mod *ir.Module, ep ir.EntryPoint, goldenFile string) error {
	hlslSrc, _, err := hlsl.Compile(mod, hlsl.DefaultOptions())
	if err != nil {
		return fmt.Errorf("hlsl compile: %w", err)
	}
	tmpHLSL, err := os.CreateTemp("", "dxc-golden-*.hlsl")
	if err != nil {
		return err
	}
	tmpHLSL.WriteString(hlslSrc)
	tmpHLSL.Close()
	defer os.Remove(tmpHLSL.Name())

	tmpBlob, err := os.CreateTemp("", "dxc-golden-*.dxil")
	if err != nil {
		return err
	}
	tmpBlob.Close()
	defer os.Remove(tmpBlob.Name())

	cmd := exec.Command(dxc, "-T", stageDxcProfile(ep.Stage), "-E", ep.Name,
		"-Vd", "-Fo", tmpBlob.Name(), tmpHLSL.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dxc compile: %w (%s)", err, stderr.String())
	}

	dumpCmd := exec.Command(dxc, "-dumpbin", tmpBlob.Name())
	out, err := dumpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dxc dumpbin: %w", err)
	}

	normalized := normalizeDxilDump(string(out))
	return os.WriteFile(goldenFile, []byte(normalized), 0o644)
}

// compareToGolden compiles via naga, dumps the DXIL, normalizes, and unified-diffs
// against the saved golden. Returns ("", nil) when matching, a (diff, nil)
// when differing, or ("", err) on hard failures (file missing, naga compile).
//
// In addition to the diff text it stores the per-shader matched/total line
// counts in the matchedLines/totalLines maps via a side channel — used by the
// test to compute parity percentage when full diff = 0 isn't yet achievable.
func compareToGolden(dxc string, mod *ir.Module, ep ir.EntryPoint, goldenFile string) (matched, total int, diff string, err error) {
	goldenBytes, err := os.ReadFile(goldenFile)
	if err != nil {
		return 0, 0, "", err
	}
	golden := string(goldenBytes)

	dxilBytes, compileErr := dxil.Compile(mod, dxil.DefaultOptions())
	if compileErr != nil {
		return 0, 0, "", fmt.Errorf("naga dxil compile: %w", compileErr)
	}

	tmpBlob, err := os.CreateTemp("", "naga-dxil-*.dxil")
	if err != nil {
		return 0, 0, "", err
	}
	tmpBlob.Write(dxilBytes)
	tmpBlob.Close()
	defer os.Remove(tmpBlob.Name())

	dumpCmd := exec.Command(dxc, "-dumpbin", tmpBlob.Name())
	out, err := dumpCmd.CombinedOutput()
	if err != nil {
		return 0, 0, string(out), fmt.Errorf("dxc dumpbin: %w", err)
	}

	naga := normalizeDxilDump(string(out))
	matched, total = countMatchedLines(golden, naga)
	if naga == golden {
		return matched, total, "", nil
	}
	return matched, total, simpleDiff(golden, naga), nil
}

// countMatchedLines returns (matchedLines, totalGoldenLines). Since the diff
// is line-based, "matched" counts golden lines that appear identically at the
// same position in the actual text (bounded by the shorter of the two).
func countMatchedLines(golden, actual string) (int, int) {
	g := strings.Split(golden, "\n")
	a := strings.Split(actual, "\n")
	matched := 0
	n := len(g)
	if len(a) < n {
		n = len(a)
	}
	for i := 0; i < n; i++ {
		if g[i] == a[i] {
			matched++
		}
	}
	return matched, len(g)
}

// Lines and tokens that are naturally per-build (hash, idents, file paths) or
// per-emitter (struct type names, function attribute group numbers) get
// rewritten to canonical placeholders so the parity test focuses on the
// semantically meaningful content (signatures, dx.op calls, control flow).
var (
	dxilDumpHashRE  = regexp.MustCompile(`(?m)^; shader hash:.*$`)
	dxilDumpIdentRE = regexp.MustCompile(`(?m)^!\d+ = !\{!".*"\}\s*; (?:llvm\.)?ident.*$`)
	dxilDumpFileRE  = regexp.MustCompile(`(?m)^.*; ModuleID = '.*$`)
	// LLVM auto-numbered IDs that differ between emitters (naga vs DXC).
	dxilLocalRegRE = regexp.MustCompile(`%(\d+)\b`)
	dxilAttrIDRE   = regexp.MustCompile(`#(\d+)\b`)
	dxilMetadataRE = regexp.MustCompile(`!(\d+)\b`)
	dxilStructRE   = regexp.MustCompile(`%(struct\.[A-Za-z_][A-Za-z_0-9]*)\b`)
	// Single-string metadata tuples like !{!"dxcoob 1.8..."} or !{!"dxil-go-naga"}
	// — emitter-specific compiler ident strings. Strip the contents so
	// per-emitter version stamps don't pollute the diff.
	dxilIdentTupleRE = regexp.MustCompile(`!\{!"[^"]*"\}`)
	// Resource name strings in dx.resources metadata tuples. DXC uses empty
	// strings (!"") because it gets names from HLSL type annotations; naga
	// embeds the variable name directly. Both are functionally equivalent —
	// the Resource Bindings reflection table (which IS compared) captures
	// names regardless. Normalize to empty to avoid false-positive diffs.
	dxilResNameRE = regexp.MustCompile(`(undef, )!"[^"]*"(, i32)`)
)

// stripReflectionSection removes a dxc -dumpbin reflection comment section
// (Buffer Definitions, Resource Bindings) that appears as a header followed
// by per-resource description lines. These are tooling-only annotations —
// the runtime DXIL container carries the same info in dx.resources metadata
// — and per-emitter content choices (naga's synthetic sampler-heap bindings,
// DXC's HLSL struct annotations) pollute structural diffs without affecting
// runtime behavior. Stripping is line-based since Go regex lacks lookahead.
//
// secHeader: the exact `;<space>` prefix that introduces the section
// (e.g. `; Buffer Definitions:` or `; Resource Bindings:`).
// nextHeaders: section headers that mark "stop skipping" boundaries.
func stripReflectionSection(s, secHeader string, nextHeaders []string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	skipping := false
	for _, ln := range lines {
		if !skipping {
			if strings.HasPrefix(strings.TrimSpace(ln), secHeader) {
				out = append(out, secHeader+" <stripped>")
				skipping = true
				continue
			}
			out = append(out, ln)
			continue
		}
		trim := strings.TrimSpace(ln)
		stop := false
		for _, h := range nextHeaders {
			if strings.HasPrefix(trim, h) {
				stop = true
				break
			}
		}
		if stop ||
			strings.HasPrefix(ln, "target ") ||
			strings.HasPrefix(ln, "%dx") ||
			strings.HasPrefix(ln, "define ") {
			out = append(out, ln)
			skipping = false
		}
	}
	return strings.Join(out, "\n")
}

var dxilSectionHeaders = []string{
	"; Buffer Definitions:",
	"; Resource Bindings:",
	"; Pipeline Runtime Information:",
	"; Input signature:",
	"; Output signature:",
	"; ViewId state:",
}

// normalizeDxilDump turns the dxc -dumpbin LLVM IR text into a canonical form
// invariant under emitter-specific naming choices: register IDs, attribute
// group IDs, metadata IDs, struct type names. Diff=0 against the DXC golden
// is then a meaningful "naga emits semantically-equivalent DXIL" gate.
func normalizeDxilDump(s string) string {
	s = dxilDumpHashRE.ReplaceAllString(s, "; shader hash: <stripped>")
	s = dxilDumpIdentRE.ReplaceAllString(s, "; ident <stripped>")
	s = dxilDumpFileRE.ReplaceAllString(s, "; ModuleID <stripped>")
	// Canonicalize compiler ident metadata strings (any single-string tuple).
	s = dxilIdentTupleRE.ReplaceAllString(s, `!{!"<ident>"}`)
	// Strip Buffer Definitions (naga-only synthetic content — DXC fills with
	// HLSL struct annotations, naga with opaque [N x i8]). Resource Bindings
	// is kept because its line format is consistent across emitters and the
	// table IS meaningful comparison surface (any diff there signals a real
	// binding mismatch worth catching).
	s = stripReflectionSection(s, "; Buffer Definitions:", dxilSectionHeaders)

	// Normalize resource name strings in dx.resources metadata (see dxilResNameRE).
	s = dxilResNameRE.ReplaceAllString(s, `${1}!""${2}`)

	s = renumberMatches(s, dxilLocalRegRE, "%R")
	s = renumberMatches(s, dxilAttrIDRE, "#A")
	s = renumberMatches(s, dxilMetadataRE, "!M")
	s = renameStructs(s, dxilStructRE)

	// Strip trailing whitespace per line.
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t\r")
	}
	return strings.Join(lines, "\n")
}

// renumberMatches replaces every occurrence of the regex (which must capture
// one numeric group) with prefix + sequential canonical ID assigned in first-
// use order. E.g., `%5, %3, %5, %1` becomes `%R0, %R1, %R0, %R2`.
func renumberMatches(s string, re *regexp.Regexp, prefix string) string {
	seq := map[string]int{}
	next := 0
	return re.ReplaceAllStringFunc(s, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := sub[1]
		if id, ok := seq[key]; ok {
			return fmt.Sprintf("%s%d", prefix, id)
		}
		id := next
		next++
		seq[key] = id
		return fmt.Sprintf("%s%d", prefix, id)
	})
}

// renameStructs canonicalizes %struct.<Name> identifiers in first-use order.
// DXC produces semantic names (RWByteAddressBuffer, struct.SomeType); naga
// produces synthetic names (UAVType, etc.). Both refer to the same underlying
// resource type — canonicalize so naming differences don't pollute the diff.
func renameStructs(s string, re *regexp.Regexp) string {
	seq := map[string]int{}
	next := 0
	return re.ReplaceAllStringFunc(s, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		key := sub[1]
		if id, ok := seq[key]; ok {
			return fmt.Sprintf("%%struct.S%d", id)
		}
		id := next
		next++
		seq[key] = id
		return fmt.Sprintf("%%struct.S%d", id)
	})
}

// simpleDiff returns a unified-style diff for golden vs actual line strings.
// Not a full diff algorithm — just shows the first few mismatched lines with
// surrounding context, which is enough for triage. For full diff use external tools.
func simpleDiff(golden, actual string) string {
	g := strings.Split(golden, "\n")
	a := strings.Split(actual, "\n")
	var b strings.Builder
	maxLen := len(g)
	if len(a) > maxLen {
		maxLen = len(a)
	}
	mismatchCount := 0
	for i := 0; i < maxLen && mismatchCount < 20; i++ {
		var gl, al string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(a) {
			al = a[i]
		}
		if gl != al {
			fmt.Fprintf(&b, "  L%d:\n    -%s\n    +%s\n", i+1, gl, al)
			mismatchCount++
		}
	}
	if mismatchCount >= 20 {
		fmt.Fprintf(&b, "  ... and more (truncated)\n")
	}
	return b.String()
}
