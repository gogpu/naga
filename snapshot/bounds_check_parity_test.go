package snapshot_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

// boundsCheckDeviation records known differing opcode counts for one shader:
// opcode → {ours, Rust}. A changed or vanished difference fails the test.
type boundsCheckDeviation struct {
	Card   string
	Reason string
	Counts map[string][2]int
}

var boundsCheckDeviations = map[string]boundsCheckDeviation{
	"binding-buffer-arrays": {
		Card: "BUG-SPV-BINDING-ARRAY-BOUNDS-001",
		Reason: "upstream extended the shader (runtime-sized binding_array<Foo>, nested member access, constant " +
			"index 7, plain storage buffer); we emit 4 more index checks and 2 OpLogicalAnd. Not analyzed yet",
		Counts: map[string][2]int{
			"OpULessThan": {11, 7}, "OpLogicalAnd": {2, 0}, "OpBranchConditional": {9, 7},
			"OpSelectionMerge": {9, 7}, "OpPhi": {9, 7}, "OpArrayLength": {10, 6},
		},
	},
}

// TestBoundsCheckInstructionsMatchRust compares the bounds-check instructions
// in our SPIR-V with Rust naga's reference, opcode by opcode, for the corpus
// shaders whose TOML sets index or buffer policies. TestRustReference
// compares SPIR-V by counts of types, decorations and functions, which do not
// see these instructions: bounds-check-restrict and bounds-check-zero
// "passed" there while no check was emitted (BUG-SPV-INDEX-BOUNDS-001).
func TestBoundsCheckInstructionsMatchRust(t *testing.T) {
	disassembler := testtools.Look(t, "spirv-dis", testtools.InstallSPIRVTools)
	opcodes := []string{"UMin", "OpULessThan", "OpLogicalAnd", "OpBranchConditional", "OpSelectionMerge",
		"OpPhi", "OpArrayLength", "OpISub"}
	for _, shader := range []string{"bounds-check-restrict", "bounds-check-zero", "binding-buffer-arrays", "policy-mix"} {
		t.Run(shader, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(rustInputDir, shader+".wgsl"))
			if err != nil {
				t.Fatal(err)
			}
			refPath, ok := rustReferencePath(rustOutSPIRV, "wgsl-"+shader+".spvasm")
			if !ok {
				t.Fatalf("no Rust SPIR-V reference for %s", shader)
			}
			ref, err := os.ReadFile(refPath)
			if err != nil {
				t.Fatal(err)
			}
			module, err := prepareForBackend(string(source), shader)
			if err != nil {
				t.Fatal(err)
			}
			ours, err := ourSPIRVDisassembly(module, shader, rustOutput{kind: rustOutSPIRV}, disassembler)
			if err != nil {
				t.Fatal(err)
			}
			dev, known := boundsCheckDeviations[shader]
			differs := false
			for _, op := range opcodes {
				word := regexp.MustCompile(`\b` + op + `\b`)
				got, want := len(word.FindAllString(ours, -1)), len(word.FindAllString(string(ref), -1))
				if got != want {
					differs = true
				}
				if expected, ok := dev.Counts[op]; known && (ok || got != want) {
					if expected != [2]int{got, want} {
						t.Errorf("%s: %d, Rust %d; deviation records %d, Rust %d (%s): re-review",
							op, got, want, expected[0], expected[1], dev.Card)
					}
					continue
				}
				if got != want {
					t.Errorf("%s: %d, Rust %d", op, got, want)
				}
			}
			switch {
			case known && !differs:
				t.Errorf("matches Rust now: remove the boundsCheckDeviations entry (%s)", dev.Card)
			case known:
				t.Logf("known deviation (%s): %s", dev.Card, dev.Reason)
			}
		})
	}
}
