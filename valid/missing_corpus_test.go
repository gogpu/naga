package valid_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// TestMissingCapabilitiesMatchesValidation: over the WGSL corpus (Rust's
// test inputs and our own), the walk of MissingCapabilities finds the same
// missing capabilities as the whole validator does, for no capability, every
// one, and each single one removed from every one. The walk is the fast path
// backends take; the validator is the definition.
func TestMissingCapabilitiesMatchesValidation(t *testing.T) {
	var files []string
	for _, pattern := range []string{
		"../snapshot/testdata/rust/in/wgsl/*.wgsl",
		"../snapshot/testdata/own/in/*.wgsl",
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) < 100 {
		t.Fatalf("corpus has %d shaders, want at least 100: the glob moved", len(files))
	}
	sets := []valid.Capabilities{0, valid.CapAll}
	for bit := 0; bit < 46; bit++ {
		sets = append(sets, valid.CapAll&^(1<<bit))
	}
	checked := 0
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		module, _, err := wgsl.Parse(string(src), wgsl.ParseOptions{})
		if err != nil {
			continue // a shader the front end does not take is not this test's
		}
		checked++
		for _, supported := range sets {
			want := valid.MissingByValidation(module, supported)
			if got := valid.MissingCapabilities(module, supported); got != want {
				t.Errorf("%s with %#x: walk %v, validation %v", filepath.Base(file), uint64(supported), got, want)
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d shaders parsed, want at least 100", checked)
	}
}
