package snapshot_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

// spvTargetEnv is the environment the corpus binaries were assembled for. It
// only fixes the version word of the header; the README of the derived tree
// explains why it is explicit.
const spvTargetEnv = "spv1.6"

// TestSPVCorpusBinaries pins the derived half of the SPIR-V input corpus:
// every vendored .spvasm has a binary assembled by spirv-as in
// rust-derived/spv/, and every binary starts with the SPIR-V magic. It is the
// cheap presence check; TestSPVCorpusAssembly proves the bytes against the
// assembler. Regenerate with the command in testdata/rust-derived/README.md
// when the vendored tree moves.
func TestSPVCorpusBinaries(t *testing.T) {
	asmDir := filepath.Join(rustCorpusDir, "in", "spv")
	binDir := filepath.Join(rustDerivedDir, "spv")

	asms, err := filepath.Glob(filepath.Join(asmDir, "*.spvasm"))
	if err != nil {
		t.Fatal(err)
	}
	if len(asms) == 0 {
		t.Fatalf("no .spvasm inputs under %s", asmDir)
	}
	bins, err := filepath.Glob(filepath.Join(binDir, "*.spv"))
	if err != nil {
		t.Fatal(err)
	}

	want := make(map[string]struct{}, len(asms))
	for _, p := range asms {
		want[strings.TrimSuffix(filepath.Base(p), ".spvasm")] = struct{}{}
	}
	got := make(map[string]struct{}, len(bins))
	for _, p := range bins {
		got[strings.TrimSuffix(filepath.Base(p), ".spv")] = struct{}{}
	}

	var missing, extra []string
	for name := range want {
		if _, ok := got[name]; !ok {
			missing = append(missing, name)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("no assembled binary in %s for: %s", binDir, strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("assembled binary without a vendored .spvasm: %s", strings.Join(extra, ", "))
	}
	if len(missing) > 0 || len(extra) > 0 {
		return
	}

	const magic = 0x07230203
	for name := range want {
		data, err := os.ReadFile(filepath.Join(binDir, name+".spv"))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(data) < 4 {
			t.Errorf("%s.spv: %d bytes, shorter than the SPIR-V magic", name, len(data))
			continue
		}
		if word := binary.LittleEndian.Uint32(data); word != magic {
			t.Errorf("%s.spv: magic %#08x, want %#08x", name, word, magic)
		}
	}
}

// TestSPVCorpusAssembly ties every committed binary to its vendored assembly:
// spirv-as must reproduce the .spv byte for byte at the recorded target
// environment. Presence and the magic cannot see a binary replaced by another
// one; this check can. The assembler's output is machine-stable for a pinned
// SPIRV-Tools version, so the comparison is exact. CI installs spirv-as and
// requires it through NAGA_REQUIRE_TOOLS; without the tool the check skips
// locally with a reason.
func TestSPVCorpusAssembly(t *testing.T) {
	as := testtools.Look(t, "spirv-as", testtools.InstallSPIRVTools)

	asmDir := filepath.Join(rustCorpusDir, "in", "spv")
	binDir := filepath.Join(rustDerivedDir, "spv")
	asms, err := filepath.Glob(filepath.Join(asmDir, "*.spvasm"))
	if err != nil {
		t.Fatal(err)
	}
	if len(asms) == 0 {
		t.Fatalf("no .spvasm inputs under %s", asmDir)
	}

	out := t.TempDir()
	for _, asm := range asms {
		name := strings.TrimSuffix(filepath.Base(asm), ".spvasm")
		t.Run(name, func(t *testing.T) {
			built := filepath.Join(out, name+".spv")
			cmd := exec.Command(as, "--target-env", spvTargetEnv, "-o", built, asm)
			if msg, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("spirv-as %s: %v\n%s", name, err, msg)
			}
			got, err := os.ReadFile(built)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(binDir, name+".spv"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("assembled binary differs from %s.spv: %d vs %d bytes, %s",
					name, len(got), len(want), firstDifference(got, want))
			}
		})
	}
}

// firstDifference names where two binaries first diverge, for a report that
// points at the byte instead of the whole file.
func firstDifference(a, b []byte) string {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return fmt.Sprintf("first difference at byte %d (0x%02x != 0x%02x)", i, a[i], b[i])
		}
	}
	return fmt.Sprintf("one binary is a prefix of the other (%d and %d bytes)", len(a), len(b))
}
