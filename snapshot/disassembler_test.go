package snapshot_test

import (
	"os"
	"os/exec"
	"regexp"
	"slices"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

var builtinOperand = regexp.MustCompile(`BuiltIn [A-Za-z]+`)

// TestDisassemblerBuiltinNames: the golden disassembler names every BuiltIn
// decoration of the corpus as spirv-dis does. Its table was shifted by one
// from ClipDistance to SampleMask, so the goldens called FragCoord PointCoord
// and PrimitiveId InvocationId.
func TestDisassemblerBuiltinNames(t *testing.T) {
	spirvDis := testtools.Look(t, "spirv-dis", testtools.InstallSPIRVTools)
	checked := 0
	for _, shader := range loadCorpus(t) {
		bin, err := compileSpirvBinary(shader.name, shader.source)
		if err != nil {
			continue // TestSpirvValBinary reports compile failures
		}
		ours := builtinOperand.FindAllString(disassembleSPIRV(bin), -1)
		if len(ours) == 0 {
			continue
		}
		tmp, err := os.CreateTemp("", "naga-builtins-*.spv")
		if err != nil {
			t.Fatal(err)
		}
		_, werr := tmp.Write(bin)
		if cerr := tmp.Close(); werr != nil || cerr != nil {
			t.Fatal(werr, cerr)
		}
		out, err := exec.Command(spirvDis, "--no-header", tmp.Name()).Output()
		_ = os.Remove(tmp.Name())
		if err != nil {
			t.Fatalf("%s: spirv-dis: %v", shader.name, err)
		}
		want := builtinOperand.FindAllString(string(out), -1)
		slices.Sort(ours)
		slices.Sort(want)
		if !slices.Equal(ours, want) {
			t.Errorf("%s: built-ins %v, spirv-dis %v", shader.name, ours, want)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no shader of the corpus decorates a built-in")
	}
}
