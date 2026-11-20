package naga

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// ggInvalidShaders are vendored gg shaders the WGSL spec makes invalid, with
// the error each must give and gg's card for its fix. An entry goes when the
// vendored copy is re-pinned to the gg commit with the fix; the test fails if
// a listed shader validates.
//
// Empty since 2026-11-20: the only entry, gpu/blend.wgsl, was re-pinned to gg
// 3b2cd10, which gives blend_soft_light's select() a vecN<bool> condition
// (gogpu/gg:P1-BLEND-003).
var ggInvalidShaders = map[string]struct{ want, card string }{}

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
		name := filepath.Base(filepath.Dir(filepath.Dir(path))) + "/" + filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			m, _, err := wgsl.Parse(string(src), wgsl.ParseOptions{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = valid.Validate(m, valid.Options{})
			if invalid, ok := ggInvalidShaders[name]; ok {
				if err == nil || !strings.Contains(err.Error(), invalid.want) {
					t.Fatalf("want %q (fix: %s), got %v", invalid.want, invalid.card, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validate with zero options: %v", err)
			}
			if _, err := Compile(string(src)); err != nil {
				t.Fatalf("Compile: %v", err)
			}
		})
	}
}
