package snapshot_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

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
			for _, op := range opcodes {
				word := regexp.MustCompile(`\b` + op + `\b`)
				got, want := len(word.FindAllString(ours, -1)), len(word.FindAllString(string(ref), -1))
				if got != want {
					t.Errorf("%s: %d, Rust %d", op, got, want)
				}
			}
		})
	}
}
