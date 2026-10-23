package snapshot_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestRONParserSmoke checks that the RON parser can parse all reference IR snapshots.
func TestRONParserSmoke(t *testing.T) {
	ronFiles := rustReferenceFiles(t, rustOutIR, func(name string) bool {
		return strings.HasPrefix(name, "wgsl-") && strings.HasSuffix(name, ".ron")
	})

	passed := 0
	total := 0
	for _, ronFile := range ronFiles {
		name := filepath.Base(ronFile)
		if strings.Contains(name, "compact") {
			continue
		}
		total++

		shaderName := strings.TrimPrefix(strings.TrimSuffix(name, ".ron"), "wgsl-")
		t.Run(shaderName, func(t *testing.T) {
			data, err := os.ReadFile(ronFile)
			if err != nil {
				t.Fatal(err)
			}
			mod, err := parseRONModule(string(data))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			_ = mod
			passed++
		})
	}
	t.Logf("RON Parser: %d/%d parsed successfully", passed, total)
}

// resolveEPFunction gets the Function for an EntryPoint (inline in EntryPoint.Function).
func resolveEPFunction(_ *ir.Module, ep ir.EntryPoint) *ir.Function {
	return &ep.Function
}

// countOverrides counts how many overrides exist in the module.
func countOverrides(mod *ir.Module) int {
	return len(mod.Overrides) // Now a []Override slice, same as Rust Arena<Override>
}

// TestIRReference compares our IR Module structure against Rust RON snapshots.
// This is the IR parity quality gate — must reach 18/18 before MSL/GLSL work.
// knownGlobalExpressionDiff lists shaders whose GlobalExpressions arena still
// differs from Rust naga's, with the exact number of differences. Each entry
// must say why. More differences than recorded fail the test (a regression
// hidden behind a known entry); fewer fail it too, so the count is lowered and
// the list can only shrink.
var knownGlobalExpressionDiff = map[string]struct {
	reason string
	diffs  int
}{}

func TestIRReference(t *testing.T) {
	ronFiles := rustReferenceFiles(t, rustOutIR, func(name string) bool {
		return strings.HasPrefix(name, "wgsl-") && strings.HasSuffix(name, ".ron")
	})

	passed := 0
	usedIRDeviations := map[string]bool{}
	total := 0
	for _, ronFile := range ronFiles {
		name := filepath.Base(ronFile)
		if strings.Contains(name, "compact") {
			continue
		}
		total++

		shaderName := strings.TrimPrefix(strings.TrimSuffix(name, ".ron"), "wgsl-")

		// Known optimization differences (documented, not bugs). None are left
		// for the uncompacted IR; TestIRReferenceCompact still has some.
		knownOptimizationDiff := map[string]bool{}

		t.Run(shaderName, func(t *testing.T) {
			// 1. Parse Rust RON reference
			ronData, err := os.ReadFile(ronFile)
			if err != nil {
				t.Fatal(err)
			}
			rustMod, err := parseRONModule(string(ronData))
			if err != nil {
				t.Fatalf("RON parse error: %v", err)
			}

			// 2. Compile our WGSL
			shader, err := corpusShader(shaderName)
			if err != nil {
				t.Fatalf("IR reference without a corpus shader: %v (see TestRustReferenceInventory)", err)
			}
			ourMod, ok := lowerCorpusShader(t, shader)
			if !ok {
				return
			}

			// 3. Deep comparison: types + expressions + statements
			var allDiffs []string

			// Types
			allDiffs = append(allDiffs, deepCompareTypes(rustMod.Types, ourMod.Types)...)

			// Functions: expressions (tag + fields) + statements
			for i := 0; i < len(rustMod.Functions) && i < len(ourMod.Functions); i++ {
				prefix := fmt.Sprintf("func[%d] %q", i, rustMod.Functions[i].Name)
				allDiffs = append(allDiffs, deepCompareExpressions(prefix, rustMod.Functions[i].Expressions, ourMod.Functions[i].Expressions)...)
				allDiffs = append(allDiffs, deepCompareExpressionFields(prefix, rustMod.Functions[i].Expressions, ourMod.Functions[i].Expressions)...)
				allDiffs = append(allDiffs, deepCompareStatements(prefix, rustMod.Functions[i].Body, ourMod.Functions[i].Body)...)
			}

			// Entry points: expressions (tag + fields) + statements
			for i := 0; i < len(rustMod.EntryPoints) && i < len(ourMod.EntryPoints); i++ {
				prefix := fmt.Sprintf("ep[%d] %q", i, rustMod.EntryPoints[i].Name)
				allDiffs = append(allDiffs, deepCompareExpressions(prefix, rustMod.EntryPoints[i].Function.Expressions, ourMod.EntryPoints[i].Function.Expressions)...)
				allDiffs = append(allDiffs, deepCompareExpressionFields(prefix, rustMod.EntryPoints[i].Function.Expressions, ourMod.EntryPoints[i].Function.Expressions)...)
				allDiffs = append(allDiffs, deepCompareStatements(prefix, rustMod.EntryPoints[i].Function.Body, ourMod.EntryPoints[i].Function.Body)...)
			}

			// Structural counts (functions/ep count, global vars, etc.)
			allDiffs = append(allDiffs, compareIRStructure(rustMod, ourMod)...)

			// Module-scope initializers (GlobalExpressions): tags + fields,
			// including Compose/ZeroValue type handles. Without this, a type
			// compacted away from under an initializer went unnoticed.
			geDiffs := deepCompareExpressions("global_expressions", rustMod.GlobalExpressions, ourMod.GlobalExpressions)
			geDiffs = append(geDiffs, deepCompareExpressionFields("global_expressions", rustMod.GlobalExpressions, ourMod.GlobalExpressions)...)
			if known, ok := knownGlobalExpressionDiff[shaderName]; ok {
				switch n := len(geDiffs); {
				case n > known.diffs:
					t.Errorf("  global_expressions: %d differences, %d known (%s) — regression:", n, known.diffs, known.reason)
					for _, d := range geDiffs {
						t.Errorf("    %s", d)
					}
				case n < known.diffs:
					t.Errorf("  global_expressions: %d differences, %d known (%s) — lower the count in knownGlobalExpressionDiff", n, known.diffs, known.reason)
				default:
					t.Logf("  [known global_expressions diff: %s] %d differences", known.reason, n)
				}
			} else {
				allDiffs = append(allDiffs, geDiffs...)
			}

			switch {
			case knownOptimizationDiff[shaderName] && len(allDiffs) == 0:
				t.Errorf("%s matches Rust now: remove it from knownOptimizationDiff", shaderName)
			case knownOptimizationDiff[shaderName]:
				t.Logf("  [known optimization diff] %d differences (our const inlining is more direct)", len(allDiffs))
				passed++ // Count as pass — optimization, not bug
			case judgeIRDiffs(t, shaderName, allDiffs, usedIRDeviations):
				passed++
			}
		})
	}
	checkIRDeviationsUsed(t, "", usedIRDeviations)
	t.Logf("\nIR Reference: %d/%d match (deep types + expressions + statements)", passed, total)
}

// irDeviation records a known difference from one Rust IR snapshot. It is
// pinned by the fingerprint of the reported differences, so a new, changed or
// vanished difference fails until the entry is reviewed again or removed.
type irDeviation struct {
	Card        string
	Reason      string
	Fingerprint string
}

// Reasons shared by several IR deviations.
const (
	reasonIRTypeOrder = "type arena order: Rust registers a component type (the scalar of atomic<T> / vecN<T>, " +
		"the image of binding_array<texture>) before the type containing it, we register the containing type " +
		"first; the type handles that follow shift"
	reasonIRMemoryDecorations = "@coherent / @volatile memory decorations are not supported: our IR has no " +
		"GlobalVariable.memory_decorations"
)

// irReferenceDeviations is keyed by "<shader>" (TestIRReference) and
// "<shader>.compact" (TestIRReferenceCompact).
var irReferenceDeviations = map[string]irDeviation{
	"9883-atomic_sub-negative": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "471e199c4c239f12a8f92b68aa618fc21683abcf6e4096aedc29cc8ec65ae174"}, // 4 differences
	"9883-atomic_sub-negative.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "471e199c4c239f12a8f92b68aa618fc21683abcf6e4096aedc29cc8ec65ae174"}, // 4 differences
	"abstract-types-atomic": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "4c181102c038746fcad8e30499e534acfbb3b7e248cf424e1c6abdf829969a89"}, // 17 differences
	"abstract-types-atomic.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "4c181102c038746fcad8e30499e534acfbb3b7e248cf424e1c6abdf829969a89"}, // 17 differences
	"atomicCompareExchange": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "0634f63810a5e09c81088e132343747a733a14ebf0812841fa28c2e6d29c1ebe"}, // 11 differences
	"atomicCompareExchange-int64": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "59fab5b8c647f67796a9ddca6e6c856518461f1670a2a982278200039d939851"}, // 22 differences
	"atomicCompareExchange-int64.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "59fab5b8c647f67796a9ddca6e6c856518461f1670a2a982278200039d939851"}, // 22 differences
	"atomicCompareExchange.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "0634f63810a5e09c81088e132343747a733a14ebf0812841fa28c2e6d29c1ebe"}, // 11 differences
	"atomicOps": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "381acd1507804d54b1226df6f152bfd86c3e77b920beb921b0ba35d59f105db0"}, // 19 differences
	"atomicOps-float32": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "1fc57418d9c3e55319789e85c295495bef41ca23db296d3cbc4a375118cc5362"}, // 14 differences
	"atomicOps-float32.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "1fc57418d9c3e55319789e85c295495bef41ca23db296d3cbc4a375118cc5362"}, // 14 differences
	"atomicOps-int64": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "07bc36f337dda054eb5efaabb0b05579a628c07f855d4b6f2fad7c37133e38b1"}, // 19 differences
	"atomicOps-int64-min-max": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "2cba90cd66520392a74fff750f4753eac9635cb979d9f59e6b6681ca033055bb"}, // 12 differences
	"atomicOps-int64-min-max.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "2cba90cd66520392a74fff750f4753eac9635cb979d9f59e6b6681ca033055bb"}, // 12 differences
	"atomicOps-int64.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "07bc36f337dda054eb5efaabb0b05579a628c07f855d4b6f2fad7c37133e38b1"}, // 19 differences
	"atomicOps.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "381acd1507804d54b1226df6f152bfd86c3e77b920beb921b0ba35d59f105db0"}, // 19 differences
	"binding-arrays": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "743570ea9418f6ddabc1721083d5c8716d45267bcb6a861e0d0580f8bf00f76e"}, // 37 differences
	"binding-arrays.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "743570ea9418f6ddabc1721083d5c8716d45267bcb6a861e0d0580f8bf00f76e"}, // 37 differences
	"bounds-check-zero-atomic": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "6a1c8b11efb265bcabf205cd1c59f53bcd4253e58fe68fdf3f16007328e9ce7a"}, // 14 differences
	"bounds-check-zero-atomic.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "6a1c8b11efb265bcabf205cd1c59f53bcd4253e58fe68fdf3f16007328e9ce7a"}, // 14 differences
	"const-exprs": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "48e4b2d2ba92d2220fbd81115a1a60114cbab28568ab0643578922acf5e3656d"}, // 7 differences
	"const-exprs.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "ecd6356542af727dca3ff90215aac0eb12473f5ce76e54d5e14a3d74bf17323e"}, // 6 differences
	"memory-decorations": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRMemoryDecorations,
		Fingerprint: "e30a7b4a1de2f36d09933d384cbfa2d119410c7a3f42220c6580fb1df5d445f3"}, // 3 differences
	"memory-decorations-coherent": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRMemoryDecorations,
		Fingerprint: "c20a228edf00ac278844c09216ed06af20174776efb577017e1f3cfb0ff6deb4"}, // 1 differences
	"memory-decorations-coherent.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRMemoryDecorations,
		Fingerprint: "c20a228edf00ac278844c09216ed06af20174776efb577017e1f3cfb0ff6deb4"}, // 1 differences
	"memory-decorations.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRMemoryDecorations,
		Fingerprint: "e30a7b4a1de2f36d09933d384cbfa2d119410c7a3f42220c6580fb1df5d445f3"}, // 3 differences
	"overrides-atomicCompareExchangeWeak": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "263008a96abf44627f69b83c48b392338a534e7839ffa1f5b32631bbe27fbd96"}, // 5 differences
	"overrides-atomicCompareExchangeWeak.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "263008a96abf44627f69b83c48b392338a534e7839ffa1f5b32631bbe27fbd96"}, // 5 differences
	"struct-layout": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "1604e528b1d4d5028c8f894ff8d55c594f69f25906e6094a80b25d5508d2fe03"}, // 9 differences
	"struct-layout.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "1604e528b1d4d5028c8f894ff8d55c594f69f25906e6094a80b25d5508d2fe03"}, // 9 differences
	"workgroup-uniform-load-atomic": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "b9c699baff80b5ff2bfe0ebb28ea66f0693b121d5a979b0e669a5543e7c9a54a"}, // 17 differences
	"workgroup-uniform-load-atomic.compact": {Card: "PARITY-UPSTREAM-001", Reason: reasonIRTypeOrder,
		Fingerprint: "b9c699baff80b5ff2bfe0ebb28ea66f0693b121d5a979b0e669a5543e7c9a54a"}, // 17 differences
}

// judgeIRDiffs reports diffs for one IR comparison unless they are exactly a
// recorded deviation. It returns whether the comparison counts as passed.
func judgeIRDiffs(t *testing.T, key string, diffs []string, used map[string]bool) bool {
	t.Helper()
	dev, ok := irReferenceDeviations[key]
	if ok {
		used[key] = true
	}
	fp := ""
	if len(diffs) > 0 {
		sum := sha256.Sum256([]byte(strings.Join(diffs, "\n")))
		fp = hex.EncodeToString(sum[:])
	}
	switch {
	case !ok && len(diffs) == 0:
		return true
	case ok && len(diffs) == 0:
		t.Errorf("%s matches Rust now: remove its irReferenceDeviations entry (%s)", key, dev.Card)
	case ok && dev.Fingerprint != fp:
		t.Errorf("%s: IR deviation no longer matches (fingerprint %s, recorded %s): re-review", key, fp, dev.Fingerprint)
	case ok:
		t.Logf("IR deviation (%s): %s", dev.Card, dev.Reason)
		return true
	default:
		t.Errorf("%s: %d IR differences (fingerprint %s)", key, len(diffs), fp)
	}
	for _, d := range diffs {
		t.Errorf("  %s", d)
	}
	return false
}

// checkIRDeviationsUsed fails on entries of one test kind that no comparison
// exercised (suffix "" for TestIRReference, ".compact" for the compact test).
func checkIRDeviationsUsed(t *testing.T, suffix string, used map[string]bool) {
	t.Helper()
	for key := range irReferenceDeviations {
		compact := strings.HasSuffix(key, ".compact")
		if compact == (suffix == ".compact") && !used[key] {
			t.Errorf("irReferenceDeviations entry %q was not exercised: remove it", key)
		}
	}
}

// TestIRReferenceCompact compares our IR after compaction against Rust compact snapshots.
// This tests the IR that actually goes into backends.
func TestIRReferenceCompact(t *testing.T) {
	ronFiles := rustReferenceFiles(t, rustOutIR, func(name string) bool {
		return strings.HasPrefix(name, "wgsl-") && strings.HasSuffix(name, ".compact.ron")
	})

	passed := 0
	usedIRDeviations := map[string]bool{}
	total := 0
	for _, ronFile := range ronFiles {
		total++

		name := filepath.Base(ronFile)
		shaderName := strings.TrimPrefix(strings.TrimSuffix(name, ".compact.ron"), "wgsl-")

		// Known diffs in compact comparison (same as non-compact + compact-specific)
		knownOptimizationDiff := map[string]bool{
			"types_with_comments": true, // CompactTypes keeps named types; Rust KeepUnused::No removes them
			"module-scope":        true, // CompactTypes does not yet remove unused types (Vec2)
			"type-alias":          true, // CompactTypes does not yet remove unused types
		}

		t.Run(shaderName, func(t *testing.T) {
			ronData, err := os.ReadFile(ronFile)
			if err != nil {
				t.Fatal(err)
			}
			rustMod, err := parseRONModule(string(ronData))
			if err != nil {
				t.Fatalf("RON parse error: %v", err)
			}

			shader, err := corpusShader(shaderName)
			if err != nil {
				t.Fatalf("IR reference without a corpus shader: %v (see TestRustReferenceInventory)", err)
			}
			ourMod, ok := lowerCorpusShader(t, shader)
			if !ok {
				return
			}

			// Apply CompactUnused + CompactTypes to match Rust's second compact(KeepUnused::No).
			// Rust's compact removes unused globals/functions AND their orphaned types.
			ir.CompactUnused(ourMod)
			ir.CompactExpressions(ourMod)
			ir.CompactTypes(ourMod)

			var allDiffs []string
			allDiffs = append(allDiffs, deepCompareTypes(rustMod.Types, ourMod.Types)...)
			for i := 0; i < len(rustMod.Functions) && i < len(ourMod.Functions); i++ {
				prefix := fmt.Sprintf("func[%d] %q", i, rustMod.Functions[i].Name)
				allDiffs = append(allDiffs, deepCompareExpressions(prefix, rustMod.Functions[i].Expressions, ourMod.Functions[i].Expressions)...)
				allDiffs = append(allDiffs, deepCompareExpressionFields(prefix, rustMod.Functions[i].Expressions, ourMod.Functions[i].Expressions)...)
				allDiffs = append(allDiffs, deepCompareStatements(prefix, rustMod.Functions[i].Body, ourMod.Functions[i].Body)...)
			}
			for i := 0; i < len(rustMod.EntryPoints) && i < len(ourMod.EntryPoints); i++ {
				prefix := fmt.Sprintf("ep[%d] %q", i, rustMod.EntryPoints[i].Name)
				allDiffs = append(allDiffs, deepCompareExpressions(prefix, rustMod.EntryPoints[i].Function.Expressions, ourMod.EntryPoints[i].Function.Expressions)...)
				allDiffs = append(allDiffs, deepCompareExpressionFields(prefix, rustMod.EntryPoints[i].Function.Expressions, ourMod.EntryPoints[i].Function.Expressions)...)
				allDiffs = append(allDiffs, deepCompareStatements(prefix, rustMod.EntryPoints[i].Function.Body, ourMod.EntryPoints[i].Function.Body)...)
			}
			allDiffs = append(allDiffs, compareIRStructure(rustMod, ourMod)...)

			switch {
			case knownOptimizationDiff[shaderName] && len(allDiffs) == 0:
				t.Errorf("%s matches Rust now: remove it from knownOptimizationDiff", shaderName)
			case knownOptimizationDiff[shaderName]:
				t.Logf("  [known optimization diff] %d differences", len(allDiffs))
				passed++
			case judgeIRDiffs(t, shaderName+".compact", allDiffs, usedIRDeviations):
				passed++
			}
		})
	}
	checkIRDeviationsUsed(t, ".compact", usedIRDeviations)
	t.Logf("\nIR Reference Compact: %d/%d match", passed, total)
}

// countEffectiveStatements counts statements excluding empty Emit(N..N) which are
// compaction artifacts in Rust naga. Empty Emits are semantically no-ops.
func countEffectiveRustStatements(stmts []ronStatement) int {
	count := 0
	for _, s := range stmts {
		if s.Tag == "Emit" {
			// Emit((start: N, end: M)) is parsed as:
			// Fields["_args"] = []interface{}{map[string]interface{}{"start": "N", "end": "M"}}
			if args, ok := s.Fields["_args"].([]interface{}); ok && len(args) == 1 {
				if inner, ok := args[0].(map[string]interface{}); ok {
					start := fmt.Sprint(inner["start"])
					end := fmt.Sprint(inner["end"])
					if start == end {
						continue // skip empty emit
					}
				}
			}
		}
		count++
	}
	return count
}

func countEffectiveGoStatements(stmts []ir.Statement) int {
	count := 0
	for _, s := range stmts {
		if emit, ok := s.Kind.(ir.StmtEmit); ok {
			if emit.Range.Start == emit.Range.End {
				continue // skip empty emit
			}
		}
		count++
	}
	return count
}

// compareIRStructure compares ronModule (Rust reference) with ir.Module (our output).
// Returns list of differences. Empty = perfect match.
func compareIRStructure(rust *ronModule, ours *ir.Module) []string {
	var diffs []string

	// Module-level counts
	if len(rust.Types) != len(ours.Types) {
		diffs = append(diffs, fmt.Sprintf("types: rust=%d go=%d", len(rust.Types), len(ours.Types)))
	}
	if len(rust.Constants) != len(ours.Constants) {
		diffs = append(diffs, fmt.Sprintf("constants: rust=%d go=%d", len(rust.Constants), len(ours.Constants)))
	}
	if len(rust.Overrides) != countOverrides(ours) {
		diffs = append(diffs, fmt.Sprintf("overrides: rust=%d go=%d", len(rust.Overrides), countOverrides(ours)))
	}
	if len(rust.GlobalVariables) != len(ours.GlobalVariables) {
		diffs = append(diffs, fmt.Sprintf("global_variables: rust=%d go=%d", len(rust.GlobalVariables), len(ours.GlobalVariables)))
	}
	if len(rust.GlobalExpressions) != len(ours.GlobalExpressions) {
		diffs = append(diffs, fmt.Sprintf("global_expressions: rust=%d go=%d", len(rust.GlobalExpressions), len(ours.GlobalExpressions)))
	}
	if len(rust.Functions) != len(ours.Functions) {
		diffs = append(diffs, fmt.Sprintf("functions: rust=%d go=%d", len(rust.Functions), len(ours.Functions)))
	}
	if len(rust.EntryPoints) != len(ours.EntryPoints) {
		diffs = append(diffs, fmt.Sprintf("entry_points: rust=%d go=%d", len(rust.EntryPoints), len(ours.EntryPoints)))
	}

	// Fields our IR does not model: a set value cannot match our module.
	for i, gv := range rust.GlobalVariables {
		if gv.MemoryDecorations != "" {
			diffs = append(diffs, fmt.Sprintf("global_variables[%d].memory_decorations: rust=%q, our IR has none",
				i, gv.MemoryDecorations))
		}
	}
	for i, ep := range rust.EntryPoints {
		if ep.IncomingRayPayload != nil {
			diffs = append(diffs, fmt.Sprintf("ep[%d] %q incoming_ray_payload: rust=%d, our IR has none",
				i, ep.Name, *ep.IncomingRayPayload))
		}
	}

	// Per-function comparison
	for i := 0; i < len(rust.Functions) && i < len(ours.Functions); i++ {
		rf := rust.Functions[i]
		of := ours.Functions[i]
		prefix := fmt.Sprintf("func[%d] %q", i, rf.Name)

		if rf.Name != of.Name {
			diffs = append(diffs, fmt.Sprintf("%s name: rust=%q go=%q", prefix, rf.Name, of.Name))
		}
		if len(rf.Arguments) != len(of.Arguments) {
			diffs = append(diffs, fmt.Sprintf("%s args: rust=%d go=%d", prefix, len(rf.Arguments), len(of.Arguments)))
		}
		if len(rf.Expressions) != len(of.Expressions) {
			diffs = append(diffs, fmt.Sprintf("%s expressions: rust=%d go=%d", prefix, len(rf.Expressions), len(of.Expressions)))
		}
		// Compare statement counts excluding empty Emit compaction artifacts.
		// Rust naga's lowerer calls compact() which removes unused expressions
		// but keeps Emit statements (adjusting ranges to empty). These empty
		// Emit(N..N) are semantically no-ops and don't affect backend output.
		rustStmts := countEffectiveRustStatements(rf.Body)
		goStmts := countEffectiveGoStatements(of.Body)
		if rustStmts != goStmts {
			diffs = append(diffs, fmt.Sprintf("%s statements: rust=%d go=%d", prefix, rustStmts, goStmts))
		}
		if len(rf.LocalVariables) != len(of.LocalVars) {
			diffs = append(diffs, fmt.Sprintf("%s locals: rust=%d go=%d", prefix, len(rf.LocalVariables), len(of.LocalVars)))
		}
		if len(rf.NamedExpressions) != len(of.NamedExpressions) {
			diffs = append(diffs, fmt.Sprintf("%s named_exprs: rust=%d go=%d", prefix, len(rf.NamedExpressions), len(of.NamedExpressions)))
		}
	}

	// Per-entry-point comparison (entry points have inline functions in Rust RON)
	for i := 0; i < len(rust.EntryPoints) && i < len(ours.EntryPoints); i++ {
		re := rust.EntryPoints[i]
		oe := ours.EntryPoints[i]
		prefix := fmt.Sprintf("ep[%d] %q", i, re.Name)

		if re.Name != oe.Name {
			diffs = append(diffs, fmt.Sprintf("%s name: rust=%q go=%q", prefix, re.Name, oe.Name))
		}

		// Both Rust RON and our Go IR have function inlined in entry point
		rf := re.Function
		of := &oe.Function

		if len(rf.Arguments) != len(of.Arguments) {
			diffs = append(diffs, fmt.Sprintf("%s args: rust=%d go=%d", prefix, len(rf.Arguments), len(of.Arguments)))
		}
		if len(rf.Expressions) != len(of.Expressions) {
			diffs = append(diffs, fmt.Sprintf("%s expressions: rust=%d go=%d", prefix, len(rf.Expressions), len(of.Expressions)))
		}
		rustStmts := countEffectiveRustStatements(rf.Body)
		goStmts := countEffectiveGoStatements(of.Body)
		if rustStmts != goStmts {
			diffs = append(diffs, fmt.Sprintf("%s statements: rust=%d go=%d", prefix, rustStmts, goStmts))
		}
		if len(rf.LocalVariables) != len(of.LocalVars) {
			diffs = append(diffs, fmt.Sprintf("%s locals: rust=%d go=%d", prefix, len(rf.LocalVariables), len(of.LocalVars)))
		}
	}

	// Type-level comparison (names and variants)
	for i := 0; i < len(rust.Types) && i < len(ours.Types); i++ {
		rt := rust.Types[i]
		ot := ours.Types[i]
		prefix := fmt.Sprintf("type[%d]", i)

		// Compare names
		rustName := ""
		if rt.Name != nil {
			rustName = *rt.Name
		}
		if rustName != ot.Name {
			diffs = append(diffs, fmt.Sprintf("%s name: rust=%q go=%q", prefix, rustName, ot.Name))
		}

		// Compare type variant tags
		goTag := typeInnerTag(ot)
		if rt.Inner.Tag != goTag {
			diffs = append(diffs, fmt.Sprintf("%s kind: rust=%s go=%s", prefix, rt.Inner.Tag, goTag))
		}
	}

	return diffs
}

// typeInnerTag returns the Rust-equivalent tag name for a Go type inner.
func typeInnerTag(t ir.Type) string {
	switch t.Inner.(type) {
	case ir.ScalarType:
		return "Scalar"
	case ir.VectorType:
		return "Vector"
	case ir.MatrixType:
		return "Matrix"
	case ir.ArrayType:
		return "Array"
	case ir.StructType:
		return "Struct"
	case ir.PointerType:
		return "Pointer"
	case ir.ImageType:
		return "Image"
	case ir.SamplerType:
		return "Sampler"
	case *ir.AtomicType:
		return "Atomic"
	case ir.AtomicType:
		return "Atomic"
	case ir.BindingArrayType:
		return "BindingArray"
	case *ir.AccelerationStructureType:
		return "AccelerationStructure"
	case ir.AccelerationStructureType:
		return "AccelerationStructure"
	case *ir.RayQueryType:
		return "RayQuery"
	case ir.RayQueryType:
		return "RayQuery"
	default:
		return fmt.Sprintf("Unknown(%T)", t.Inner)
	}
}

// Helper to parse shader name from RON file
func wgsl_Lower(t *testing.T, source string) *ir.Module {
	t.Helper()
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	return module
}
