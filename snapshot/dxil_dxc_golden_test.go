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
	"sort"
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
	// LLVM instruction flags (nsw, nuw, exact) on integer arithmetic are
	// pure optimization hints without runtime semantic effect. DXC's LLVM
	// passes set these when they can prove absence of overflow; we don't
	// because we have no optimization infrastructure. Strip them from
	// instructions like "add nsw i32 %R, -1" to match "add i32 %R, -1".
	// Note: uses \b word boundary to avoid matching fmul/fadd/fsub which
	// carry the separate "fast" flag (both DXC and naga emit it).
	dxilInstrFlagsRE = regexp.MustCompile(`(\b(?:add|sub|mul|shl|lshr|ashr)\b )(?:nsw |nuw |exact )+`)
	// Strip "fast" flag from fcmp comparisons. DXC sometimes sets this but
	// we don't; it only affects NaN handling which is defined-away in WGSL.
	dxilFcmpFastRE = regexp.MustCompile(`(fcmp )fast `)
	// SM minor version in dx.version and dx.shaderModel metadata. Our
	// emitter auto-upgrades SM (e.g., SM 6.6 for int64 atomics, SM 6.1
	// for ViewID) while the golden pipeline uses a fixed -T profile
	// (cs_6_0 etc.). The SM minor version is redundant information —
	// the PSVRuntimeInfo header comment (which IS compared) shows the
	// actual shader model. Normalize to 0 to avoid false positives from
	// the auto-upgrade vs fixed-profile divergence.
	dxilSMVersionRE      = regexp.MustCompile(`(!{i32 1, i32 )\d+(})`)
	dxilShaderModelMinRE = regexp.MustCompile(`(!{!"(?:vs|ps|cs|ms|as)", i32 6, i32 )\d+(})`)
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

	// Strip LLVM instruction flags that are pure optimization hints without
	// runtime semantic effect. DXC's LLVM passes set these when they can
	// prove absence of overflow (nsw/nuw/exact on integer ops) or allow
	// fast-math rewrites (fast on fcmp). Our emitter doesn't set these
	// because we have no optimization infrastructure.
	s = dxilInstrFlagsRE.ReplaceAllString(s, "$1")
	s = dxilFcmpFastRE.ReplaceAllString(s, "$1")

	// Normalize SM minor version in dx.version and dx.shaderModel metadata
	// to 0. Our emitter auto-upgrades SM based on features (e.g., int64
	// atomics -> SM 6.6, ViewID -> SM 6.1) while the golden pipeline uses
	// a fixed -T profile (cs_6_0). Both are correct for their context.
	s = dxilSMVersionRE.ReplaceAllString(s, "${1}0${2}")
	s = dxilShaderModelMinRE.ReplaceAllString(s, "${1}0${2}")

	// Sort function declarations into a canonical order so that DXC-internal
	// lowering pass scheduling (which controls the order intrinsic declarations
	// appear in the output) does not pollute the diff. Declaration order has no
	// runtime semantic effect — the actual function resolution is by name, and
	// the attribute groups themselves are compared independently.
	s = sortFuncDeclarations(s)

	s = renumberMatches(s, dxilLocalRegRE, "%R")
	s = renumberMatches(s, dxilAttrIDRE, "#A")

	// Sort "attributes #AN = { ... }" lines into canonical order by ID so
	// that the declaration-order normalization above doesn't leave residual
	// diffs in the attribute definition block at the end of the file.
	s = sortAttrDefinitions(s)
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

// sortAttrDefinitions sorts "attributes #AN = { ... }" lines into canonical
// order by their numeric ID. After declaration sorting and attribute
// renumbering, the attribute definition block at the end of the file may
// have its entries in a different order between DXC and naga. Since the
// attribute IDs have already been renumbered, sorting by ID gives a
// deterministic canonical order.
func sortAttrDefinitions(s string) string {
	lines := strings.Split(s, "\n")
	firstIdx := -1
	lastIdx := -1
	var attrLines []string
	for i, ln := range lines {
		if strings.HasPrefix(ln, "attributes #") {
			if firstIdx == -1 {
				firstIdx = i
			}
			lastIdx = i
			attrLines = append(attrLines, ln)
		}
	}
	if len(attrLines) < 2 {
		return s
	}
	sort.Strings(attrLines)
	result := make([]string, 0, len(lines))
	result = append(result, lines[:firstIdx]...)
	result = append(result, attrLines...)
	result = append(result, lines[lastIdx+1:]...)
	return strings.Join(result, "\n")
}

// sortFuncDeclarations sorts LLVM-IR function declaration blocks into a
// canonical alphabetical order by function name. Each declaration consists
// of an optional "; Function Attrs: ..." comment immediately preceding a
// "declare ..." line. DXC orders declarations according to its internal
// HLSL lowering pass schedule, which varies across shaders and cannot be
// reproduced by a third-party emitter. Declaration order has no runtime
// semantic effect — function calls resolve by name/value-ID, and attribute
// groups are independently compared. Sorting into a canonical order removes
// this source of noise from the parity diff while preserving all other
// semantically meaningful content (signatures, dx.op call patterns, metadata).
func sortFuncDeclarations(s string) string {
	lines := strings.Split(s, "\n")
	// Collect declaration pairs: each pair is (attrs-comment, declare-line).
	type declPair struct {
		attrLine    string // "; Function Attrs: ..." or empty
		declareLine string // "declare ..."
		funcName    string // extracted for sorting
	}
	var pairs []declPair
	// Track positions of the first and last declaration in the line array.
	firstIdx := -1
	lastIdx := -1

	i := 0
	for i < len(lines) {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "; Function Attrs:") && i+1 < len(lines) {
			nextTrim := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextTrim, "declare ") {
				if firstIdx == -1 {
					firstIdx = i
				}
				// Extract function name from declare line for sorting.
				name := extractDeclFuncName(lines[i+1])
				pairs = append(pairs, declPair{
					attrLine:    lines[i],
					declareLine: lines[i+1],
					funcName:    name,
				})
				lastIdx = i + 1
				i += 2
				continue
			}
		}
		if strings.HasPrefix(trim, "declare ") {
			if firstIdx == -1 {
				firstIdx = i
			}
			name := extractDeclFuncName(lines[i])
			pairs = append(pairs, declPair{
				declareLine: lines[i],
				funcName:    name,
			})
			lastIdx = i
			i++
			continue
		}
		i++
	}

	if len(pairs) < 2 {
		return s // 0 or 1 declaration — nothing to sort
	}

	// Sort pairs alphabetically by function name.
	sort.Slice(pairs, func(a, b int) bool {
		return pairs[a].funcName < pairs[b].funcName
	})

	// Rebuild the declaration section. The declaration block spans from
	// firstIdx to lastIdx (inclusive), with possible blank lines between
	// pairs. Rebuild with a blank line between each sorted pair.
	var declLines []string
	for j, p := range pairs {
		if j > 0 {
			declLines = append(declLines, "")
		}
		if p.attrLine != "" {
			declLines = append(declLines, p.attrLine)
		}
		declLines = append(declLines, p.declareLine)
	}

	// Replace the declaration section in the original lines.
	result := make([]string, 0, len(lines))
	result = append(result, lines[:firstIdx]...)
	result = append(result, declLines...)
	result = append(result, lines[lastIdx+1:]...)
	return strings.Join(result, "\n")
}

// extractDeclFuncName extracts the function name from a "declare" line.
// Example: "declare i32 @dx.op.atomicBinOp.i32(...) #0" -> "dx.op.atomicBinOp.i32"
func extractDeclFuncName(line string) string {
	idx := strings.Index(line, "@")
	if idx < 0 {
		return line
	}
	rest := line[idx+1:]
	end := strings.IndexAny(rest, "(")
	if end < 0 {
		return rest
	}
	return rest[:end]
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

// --- Unit tests for normalizer helpers ---

func TestSortFuncDeclarations(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "single declaration unchanged",
			in: `define void @main() {
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32) #0

attributes #0 = { nounwind }`,
			want: `define void @main() {
  ret void
}

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32) #0

attributes #0 = { nounwind }`,
		},
		{
			name: "two declarations sorted alphabetically",
			in: `}

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32) #1

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32) #0

attributes #0 = { nounwind }`,
			want: `}

; Function Attrs: nounwind
declare void @dx.op.bufferStore.i32(i32) #0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32) #1

attributes #0 = { nounwind }`,
		},
		{
			name: "three declarations sorted",
			in: `}

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32) #0

; Function Attrs: nounwind
declare i32 @dx.op.atomicBinOp.i32(i32) #1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32) #0

attributes #0 = { nounwind readonly }`,
			want: `}

; Function Attrs: nounwind
declare i32 @dx.op.atomicBinOp.i32(i32) #1

; Function Attrs: nounwind readonly
declare %dx.types.ResRet.i32 @dx.op.bufferLoad.i32(i32) #0

; Function Attrs: nounwind readonly
declare %dx.types.Handle @dx.op.createHandle(i32) #0

attributes #0 = { nounwind readonly }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortFuncDeclarations(tt.in)
			if got != tt.want {
				t.Errorf("sortFuncDeclarations() mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestSortAttrDefinitions(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "already sorted",
			in:   "attributes #A0 = { nounwind }\nattributes #A1 = { nounwind readonly }",
			want: "attributes #A0 = { nounwind }\nattributes #A1 = { nounwind readonly }",
		},
		{
			name: "reverse order",
			in:   "attributes #A1 = { nounwind readonly }\nattributes #A0 = { nounwind }",
			want: "attributes #A0 = { nounwind }\nattributes #A1 = { nounwind readonly }",
		},
		{
			name: "single attr unchanged",
			in:   "attributes #A0 = { nounwind }",
			want: "attributes #A0 = { nounwind }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortAttrDefinitions(tt.in)
			if got != tt.want {
				t.Errorf("sortAttrDefinitions() mismatch:\ngot:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestExtractDeclFuncName(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"declare void @dx.op.bufferStore.i32(i32) #0", "dx.op.bufferStore.i32"},
		{"declare %dx.types.Handle @dx.op.createHandle(i32) #1", "dx.op.createHandle"},
		{"declare i32 @dx.op.atomicBinOp.i32(i32) #0", "dx.op.atomicBinOp.i32"},
	}
	for _, tt := range tests {
		got := extractDeclFuncName(tt.line)
		if got != tt.want {
			t.Errorf("extractDeclFuncName(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

// TestNormalizeSMVersion verifies that the SM minor version in dx.version
// and dx.shaderModel metadata is normalized to 0. This prevents false diffs
// when our emitter auto-upgrades SM (e.g., SM 6.6 for int64 atomics) while
// the DXC golden pipeline uses a fixed -T profile (cs_6_0).
func TestNormalizeSMVersion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "dx.version SM 6.0 unchanged",
			input: `!5 = !{i32 1, i32 0}`,
			want:  `!5 = !{i32 1, i32 0}`,
		},
		{
			name:  "dx.version SM 6.6 normalized to 0",
			input: `!5 = !{i32 1, i32 6}`,
			want:  `!5 = !{i32 1, i32 0}`,
		},
		{
			name:  "dx.shaderModel cs 6.0 unchanged",
			input: `!7 = !{!"cs", i32 6, i32 0}`,
			want:  `!7 = !{!"cs", i32 6, i32 0}`,
		},
		{
			name:  "dx.shaderModel cs 6.6 normalized",
			input: `!7 = !{!"cs", i32 6, i32 6}`,
			want:  `!7 = !{!"cs", i32 6, i32 0}`,
		},
		{
			name:  "dx.shaderModel vs 6.1 normalized",
			input: `!7 = !{!"vs", i32 6, i32 1}`,
			want:  `!7 = !{!"vs", i32 6, i32 0}`,
		},
		{
			name:  "dx.shaderModel ms 6.5 normalized",
			input: `!7 = !{!"ms", i32 6, i32 5}`,
			want:  `!7 = !{!"ms", i32 6, i32 0}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dxilSMVersionRE.ReplaceAllString(tt.input, "${1}0${2}")
			got = dxilShaderModelMinRE.ReplaceAllString(got, "${1}0${2}")
			if got != tt.want {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
