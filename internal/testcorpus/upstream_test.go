package testcorpus

import (
	"os"
	"path/filepath"
	"testing"
)

// upstreamDir holds third-party test data, one tree per upstream repository
// at testdata/upstream/<host>/<owner>/<repo>.
const upstreamDir = "../../testdata/upstream"

// TestUpstreamTrees checks every vendored tree under testdata/upstream: it
// carries the upstream license and a PROVENANCE.md, and its files match its
// manifest byte for byte. Tests must not read third-party data from anywhere
// else (gogpu ecosystem rule: tests are hermetic).
func TestUpstreamTrees(t *testing.T) {
	trees, err := filepath.Glob(filepath.Join(upstreamDir, "*", "*", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(trees) == 0 {
		t.Fatalf("no trees under %s", upstreamDir)
	}
	for _, tree := range trees {
		rel, err := filepath.Rel(upstreamDir, tree)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.ToSlash(rel)
		t.Run(name, func(t *testing.T) {
			info, err := os.Stat(tree)
			if err != nil {
				t.Fatal(err)
			}
			if !info.IsDir() {
				t.Fatalf("%s is a file; trees live at <host>/<owner>/<repo>/", name)
			}
			if _, err := os.Stat(filepath.Join(tree, ProvenanceName)); err != nil {
				t.Errorf("no %s: %v", ProvenanceName, err)
			}
			licenses, err := filepath.Glob(filepath.Join(tree, "LICENSE*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(licenses) == 0 {
				t.Error("no LICENSE* file from upstream")
			}
			m, err := Load(tree)
			if err != nil {
				t.Fatal(err)
			}
			if m.Kind != KindVendored {
				t.Errorf("manifest kind = %q, want %q", m.Kind, KindVendored)
			}
			if m.Source.Commit == "" {
				t.Error("manifest records no upstream commit")
			}
			problems, err := Verify(tree)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range problems {
				t.Errorf("%s/%s", name, p)
			}
		})
	}
}
