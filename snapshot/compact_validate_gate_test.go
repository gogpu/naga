package snapshot_test

// TestCompactKeepsModulesValid is the compact-and-validate sweep the review of
// PR #235 asked for: every vendored WGSL shader our frontend parses is compacted
// with both KeepUnused values and the result is validated. Compaction must not
// produce an invalid module — before this gate nothing in the suite validated a
// compacted WGSL module, and a type-reachability change that dropped a named
// type the typifier resolves by name (the predeclared __modf/__frexp result
// structs, card P2-IR-034) passed every other check while making valid shaders
// fail to compile.
//
// The GLSL side is covered by TestGLSLTextReference, whose pipeline validates
// after the compaction it mirrors from the pinned naga-cli.

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

func TestCompactKeepsModulesValid(t *testing.T) {
	dir := filepath.Join(rustCorpusDir, "in", "wgsl")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the WGSL corpus is missing: %v", err)
	}
	var sources []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".wgsl" {
			sources = append(sources, entry.Name())
		}
	}
	sort.Strings(sources)
	if len(sources) == 0 {
		t.Fatalf("no WGSL sources under %s", dir)
	}

	keeps := []struct {
		name string
		keep naga.KeepUnused
	}{
		{"KeepUnusedYes", naga.KeepUnusedYes},
		{"KeepUnusedNo", naga.KeepUnusedNo},
	}
	parsed, skipped := 0, 0
	for _, name := range sources {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			module, _, parseErr := wgsl.Parse(string(source), wgsl.ParseOptions{})
			if parseErr != nil {
				// The frontend refusing a corpus shader is another gate's
				// business (TestRustReference reports it); this sweep only
				// covers modules it produces.
				skipped++
				t.Skipf("the WGSL frontend refuses it: %v", parseErr)
			}
			parsed++
			for _, k := range keeps {
				compacted := module.Clone()
				naga.Compact(compacted, k.keep)
				if _, err := valid.Validate(compacted, valid.Options{Capabilities: valid.CapAll}); err != nil {
					t.Errorf("%s: the compacted module is invalid: %v", k.name, err)
				}
			}
		})
	}
	t.Logf("%d sources: %d compacted and validated with both keep values, %d refused by the frontend",
		len(sources), parsed, skipped)
}
