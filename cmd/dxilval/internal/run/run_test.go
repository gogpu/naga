package run

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMainUsage verifies that calling without any args exits with usage code
// and prints the usage banner to stderr.
func TestMainUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main(nil, &stdout, &stderr)
	if code != ExitUsage {
		t.Errorf("exit code: got %d, want %d", code, ExitUsage)
	}
	if !strings.Contains(stderr.String(), "Usage: dxilval") {
		t.Errorf("stderr missing usage banner:\n%s", stderr.String())
	}
}

// TestMainMutuallyExclusive verifies that combining mode flags is rejected.
func TestMainMutuallyExclusive(t *testing.T) {
	cases := [][]string{
		{"--wgsl", "a.wgsl", "--corpus", "dir/"},
		{"--wgsl", "a.wgsl", "b.dxil"},
		{"--corpus", "dir/", "b.dxil"},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		code := Main(args, &stdout, &stderr)
		if code != ExitUsage {
			t.Errorf("args=%v: exit code %d, want %d (stderr=%q)", args, code, ExitUsage, stderr.String())
		}
		if !strings.Contains(stderr.String(), "mutually exclusive") {
			t.Errorf("args=%v: stderr missing 'mutually exclusive': %q", args, stderr.String())
		}
	}
}

// TestMainUnknownFlag verifies that flag parsing errors return ExitUsage.
func TestMainUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"--no-such-flag"}, &stdout, &stderr)
	if code != ExitUsage {
		t.Errorf("exit code: got %d, want %d", code, ExitUsage)
	}
}

// TestShouldSkipForTargets_FloatAtomics verifies WGSL containing
// atomic<f32>/atomic<f64> is skipped because DXIL has no standard lowering.
func TestShouldSkipForTargets_FloatAtomics(t *testing.T) {
	tmp := t.TempDir()

	cases := []struct {
		name     string
		source   string
		wantSkip bool
	}{
		{"f32_atomic", "var<storage> x: atomic<f32>;", true},
		{"f64_atomic", "var<storage> x: atomic<f64>;", true},
		{"i32_atomic_ok", "var<storage> x: atomic<i32>;", false},
		{"no_atomic", "@compute fn main() {}", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wgsl := filepath.Join(tmp, tc.name+".wgsl")
			if err := os.WriteFile(wgsl, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			reason, skip := shouldSkipForTargets(wgsl)
			if skip != tc.wantSkip {
				t.Errorf("skip = %v, want %v (reason=%q)", skip, tc.wantSkip, reason)
			}
			if tc.wantSkip && !strings.Contains(reason, "atomic") {
				t.Errorf("reason should mention atomics, got %q", reason)
			}
		})
	}
}

// TestLoadHLSLBindingMap verifies the narrow TOML parser recognizes
// Rust naga's [[hlsl.binding_map]] entries and returns a dxil.BindingMap
// with the correct (group, binding) -> (space, register, arraySize)
// mapping. The parser must:
//   - honor binding_array_size on unbounded arrays
//   - skip non-HLSL binding maps ([[spv.binding_map]], [[msl...]])
//   - return nil when the sibling .toml is absent or has no HLSL map
func TestLoadHLSLBindingMap(t *testing.T) {
	tmp := t.TempDir()

	t.Run("hlsl binding map with array size override", func(t *testing.T) {
		wgsl := filepath.Join(tmp, "bma.wgsl")
		toml := filepath.Join(tmp, "bma.toml")
		if err := os.WriteFile(wgsl, []byte("@compute fn main() {}"), 0o644); err != nil {
			t.Fatal(err)
		}
		content := `
[[hlsl.binding_map]]
bind_target = { binding_array_size = 10, register = 0, space = 0 }
resource_binding = { group = 0, binding = 0 }

[[hlsl.binding_map]]
bind_target = { register = 5, space = 3 }
resource_binding = { group = 0, binding = 7 }
`
		if err := os.WriteFile(toml, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		got := loadHLSLBindingMap(wgsl)
		if len(got) != 2 {
			t.Fatalf("want 2 entries, got %d: %+v", len(got), got)
		}
		e0, ok := got[struct{ Group, Binding uint32 }{Group: 0, Binding: 0}]
		_ = e0
		_ = ok
		// Use the real types to index into the map (struct literal won't
		// match the exported type identity).
		for k, v := range got {
			switch {
			case k.Group == 0 && k.Binding == 0:
				if v.Space != 0 || v.Register != 0 {
					t.Errorf("entry0: space/register = %d/%d, want 0/0", v.Space, v.Register)
				}
				if v.BindingArraySize == nil || *v.BindingArraySize != 10 {
					t.Errorf("entry0: BindingArraySize = %v, want *10", v.BindingArraySize)
				}
			case k.Group == 0 && k.Binding == 7:
				if v.Space != 3 || v.Register != 5 {
					t.Errorf("entry1: space/register = %d/%d, want 3/5", v.Space, v.Register)
				}
				if v.BindingArraySize != nil {
					t.Errorf("entry1: BindingArraySize = %v, want nil", v.BindingArraySize)
				}
			}
		}
	})

	t.Run("spv only map returns nil", func(t *testing.T) {
		wgsl := filepath.Join(tmp, "spv.wgsl")
		toml := filepath.Join(tmp, "spv.toml")
		if err := os.WriteFile(wgsl, []byte("@compute fn main() {}"), 0o644); err != nil {
			t.Fatal(err)
		}
		content := `
[[spv.binding_map]]
bind_target = { descriptor_set = 0, binding = 0 }
resource_binding = { group = 0, binding = 0 }
`
		if err := os.WriteFile(toml, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := loadHLSLBindingMap(wgsl); got != nil {
			t.Errorf("spv-only map must not surface as HLSL map, got %+v", got)
		}
	})

	t.Run("missing toml returns nil", func(t *testing.T) {
		wgsl := filepath.Join(tmp, "notoml.wgsl")
		if err := os.WriteFile(wgsl, []byte("@compute fn main() {}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := loadHLSLBindingMap(wgsl); got != nil {
			t.Errorf("missing toml must return nil, got %+v", got)
		}
	})
}

// TestMainSingleMissingFile verifies that single-mode reports a clean
// failure (not a panic) when the file does not exist.
func TestMainSingleMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main([]string{"./does-not-exist.dxil"}, &stdout, &stderr)
	if code != ExitFail {
		t.Errorf("exit code: got %d, want %d", code, ExitFail)
	}
	if !strings.Contains(stderr.String(), "does-not-exist.dxil") {
		t.Errorf("stderr missing path: %q", stderr.String())
	}
}
