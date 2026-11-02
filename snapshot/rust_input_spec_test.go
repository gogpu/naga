package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/wgsl"
)

// TestRustInputSpecDirectives: each directive the harness adds names a
// vendored Rust input that the frontend rejects without it and parses with
// it, so an entry that is no longer needed, or names a renamed input, fails
// here instead of hiding.
func TestRustInputSpecDirectives(t *testing.T) {
	for name, directive := range rustInputSpecDirectives {
		data, err := os.ReadFile(filepath.Join(rustInputDir, name+".wgsl"))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, _, err := wgsl.Parse(string(data), wgsl.ParseOptions{}); err == nil {
			t.Errorf("%s parses without %q: drop the entry", name, directive)
		}
		if _, _, err := wgsl.Parse(directive+string(data), wgsl.ParseOptions{}); err != nil {
			t.Errorf("%s with %q: %v", name, directive, err)
		}
	}
}
