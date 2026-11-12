package snapshot_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestSPVCorpusBinaries pins the derived half of the SPIR-V input corpus:
// every vendored .spvasm has a binary assembled by spirv-as in
// rust-derived/spv/, and every binary starts with the SPIR-V magic. It checks
// presence and the header, not the bytes: CI has no assembler to compare
// against. Regenerate with the command in testdata/rust-derived/README.md when
// the vendored tree moves.
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
