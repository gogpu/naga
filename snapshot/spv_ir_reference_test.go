package snapshot_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
)

// spvIRAwaitingPhase lists the Rust IR snapshots of SPIR-V input whose module
// the frontend still refuses, with the refusal it gives. A module that starts
// to parse fails here until its entry is removed and its IR is compared.
var spvIRAwaitingPhase = map[string]string{
	"fetch_depth": "unsupported instruction ImageFetch",   // Phase 3: images
	"shadow":      "unsupported instruction SampledImage", // Phase 3: combined samplers
}

// spvIRDeviations records the known differences from one Rust IR snapshot of
// SPIR-V input, pinned by the fingerprint of the reported differences as
// irReferenceDeviations does for WGSL: a new, changed or vanished difference
// fails until the entry is reviewed again.
var spvIRDeviations = map[string]irDeviation{
	"spec-constants": {
		Card: "deliberate deviation: expression storage order",
		Reason: "the global and constant expressions every function starts with are in ascending id order here, " +
			"in FxHashMap iteration order in Rust naga (make_expression_storage): the same expressions, other " +
			"handles, so the references between them shift with the order; the types, the constants and the " +
			"wrapper (after P1-SPVIN-003 ported gl_PerVertex culling) match Rust",
		Fingerprint: "10dc02444320277a35fd669e25c6dfc1704c17ed7df1ca1aff5e93fa4343eb14", // 16 differences
	},
}

// TestSPVIRReference compares the IR the SPIR-V frontend builds with Rust
// naga's IR snapshots of the same modules (tests/out/ir/spv-*.ron, the
// uncompacted ones: the frontend does not compact). The comparison is the
// one TestIRReference makes for WGSL: types, every function's and entry
// point's expressions and statements, the structural counts and the global
// expressions.
//
// It covers no control flow yet: the one module it compares, spec-constants,
// has no branch. The evidence for structured control flow and calls is the
// hand-built tests of spirv/internal/front and the WGSL corpus round trip.
// Of the snapshots awaiting Phase 3, shadow (combined samplers) brings
// control flow (two Ifs and a Loop in Rust's IR); fetch_depth has none.
func TestSPVIRReference(t *testing.T) {
	ronFiles := rustReferenceFiles(t, rustOutIR, func(name string) bool {
		return strings.HasPrefix(name, "spv-") && strings.HasSuffix(name, ".ron") && !strings.HasSuffix(name, ".compact.ron")
	})
	if len(ronFiles) == 0 {
		t.Fatal("no spv IR snapshots")
	}
	compared := 0
	seen := map[string]bool{}
	for _, ronFile := range ronFiles {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(ronFile), "spv-"), ".ron")
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(rustDerivedDir, "spv", name+".spv"))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			ours, _, err := spirv.Parse(data, spirv.ParseOptions{})
			if refusal, awaiting := spvIRAwaitingPhase[name]; awaiting {
				var parseErr *spirv.ParseError
				switch {
				case err == nil:
					t.Fatalf("%s parses now: remove it from spvIRAwaitingPhase and compare its IR", name)
				case !errors.As(err, &parseErr) || !strings.Contains(err.Error(), refusal):
					t.Fatalf("%s: refused with %v, recorded %q", name, err, refusal)
				}
				t.Logf("awaiting a later phase: %v", err)
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ronData, err := os.ReadFile(ronFile)
			if err != nil {
				t.Fatal(err)
			}
			rust, err := parseRONModule(string(ronData))
			if err != nil {
				t.Fatalf("RON parse error: %v", err)
			}
			diffs := compareSPVIR(rust, ours)
			fingerprint := ""
			if len(diffs) > 0 {
				sum := sha256.Sum256([]byte(strings.Join(diffs, "\n")))
				fingerprint = hex.EncodeToString(sum[:])
			}
			deviation, known := spvIRDeviations[name]
			switch {
			case !known && len(diffs) == 0:
				compared++
				return
			case known && len(diffs) == 0:
				t.Errorf("%s matches Rust now: remove its spvIRDeviations entry", name)
				return
			case known && deviation.Fingerprint == fingerprint:
				t.Logf("known deviation (%s): %d differences: %s", deviation.Card, len(diffs), deviation.Reason)
				return
			case known:
				t.Errorf("%s: the deviation changed (fingerprint %s, recorded %s): re-review", name, fingerprint, deviation.Fingerprint)
			default:
				t.Errorf("%d IR differences from Rust naga (fingerprint %s):", len(diffs), fingerprint)
			}
			for _, d := range diffs {
				t.Errorf("  %s", d)
			}
		})
	}
	for name := range spvIRAwaitingPhase {
		if !seen[name] {
			t.Errorf("spvIRAwaitingPhase entry %q has no IR snapshot: remove it", name)
		}
	}
	for name := range spvIRDeviations {
		if !seen[name] {
			t.Errorf("spvIRDeviations entry %q has no IR snapshot: remove it", name)
		}
	}
	t.Logf("SPIR-V IR reference: %d of %d snapshots match, %d deviate as recorded, %d await a later phase",
		compared, len(ronFiles), len(spvIRDeviations), len(spvIRAwaitingPhase))
}

// compareSPVIR lists the differences between Rust naga's module and ours.
func compareSPVIR(rust *ronModule, ours *ir.Module) []string {
	diffs := deepCompareTypes(rust.Types, ours.Types)
	for i := 0; i < len(rust.Functions) && i < len(ours.Functions); i++ {
		prefix := fmt.Sprintf("func[%d] %q", i, rust.Functions[i].Name)
		if rust.Functions[i].Name != ours.Functions[i].Name {
			diffs = append(diffs, fmt.Sprintf("%s: name %q", prefix, ours.Functions[i].Name))
		}
		diffs = append(diffs, deepCompareExpressions(prefix, rust.Functions[i].Expressions, ours.Functions[i].Expressions)...)
		diffs = append(diffs, deepCompareExpressionFields(prefix, rust.Functions[i].Expressions, ours.Functions[i].Expressions)...)
		diffs = append(diffs, deepCompareStatements(prefix, rust.Functions[i].Body, ours.Functions[i].Body)...)
	}
	for i := 0; i < len(rust.EntryPoints) && i < len(ours.EntryPoints); i++ {
		prefix := fmt.Sprintf("ep[%d] %q", i, rust.EntryPoints[i].Name)
		rf, of := &rust.EntryPoints[i].Function, &ours.EntryPoints[i].Function
		diffs = append(diffs, deepCompareExpressions(prefix, rf.Expressions, of.Expressions)...)
		diffs = append(diffs, deepCompareExpressionFields(prefix, rf.Expressions, of.Expressions)...)
		diffs = append(diffs, deepCompareStatements(prefix, rf.Body, of.Body)...)
	}
	diffs = append(diffs, compareIRStructure(rust, ours)...)
	diffs = append(diffs, deepCompareExpressions("global_expressions", rust.GlobalExpressions, ours.GlobalExpressions)...)
	return append(diffs, deepCompareExpressionFields("global_expressions", rust.GlobalExpressions, ours.GlobalExpressions)...)
}
