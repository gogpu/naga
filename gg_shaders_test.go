package naga

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// TestGGShadersNeedNoCapability: gg compiles its production shaders with
// Compile and the zero CompileOptions, so every capability gate must leave
// them valid with the zero valid.Options. The shaders are gg's, vendored in
// testdata/upstream/github.com/gogpu/gg (PROVENANCE.md there).
func TestGGShadersNeedNoCapability(t *testing.T) {
	var files []string
	for _, dir := range []string{"internal/gpu/shaders", "internal/gpu/tilecompute/shaders"} {
		matches, err := filepath.Glob(filepath.Join("testdata/upstream/github.com/gogpu/gg", dir, "*.wgsl"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) != 25 {
		t.Fatalf("found %d gg shaders, want the 25 vendored", len(files))
	}
	for _, path := range files {
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path)))+"/"+filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m, _, err := wgsl.Parse(string(src), wgsl.ParseOptions{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if _, err := valid.Validate(m, valid.Options{}); err != nil {
				t.Fatalf("validate with zero options: %v", err)
			}
			if _, err := Compile(string(src)); err != nil {
				t.Fatalf("Compile: %v", err)
			}
		})
	}
}
