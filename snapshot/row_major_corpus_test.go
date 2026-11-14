package snapshot_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// rowMajorDir holds the row-major corpus: the authored GLSL sources, their
// binaries, and Rust naga's MSL/HLSL outputs as goldens. Upstream has no
// row-major rows, so the corpus lives in the derived tree with its
// PROVENANCE.md (ADR-006). The store fixture is the corpus's expected
// refusal: its golden is the error, not text output (card P3-SPVIN-007).
var rowMajorDir = filepath.Join(rustDerivedDir, "row-major")

// TestRowMajorCorpus reads every row-major fixture through the SPIR-V
// frontend and compares the MSL and HLSL it writes with the Rust naga goldens,
// the same way TestSPVTextReference does for the vendored corpus. The options
// mirror Rust's defaults at the pin: MSL language version 1.0 with fake
// missing bindings, the HLSL writer's default fake bindings.
func TestRowMajorCorpus(t *testing.T) {
	for _, name := range []string{"rm-member", "rm-column", "rm-array-element", "rm-whole-array"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(rowMajorDir, "spv", name+".spv"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			// Rust's front/spv default negates the Y of Position on the way in;
			// our zero ParseOptions does the same.
			module, _, err := spirv.Parse(data, spirv.ParseOptions{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			naga.Compact(module, naga.KeepUnusedNo)

			compare := func(kind, extension, got string, err error) {
				t.Helper()
				if err != nil {
					t.Fatalf("%s: %v", kind, err)
				}
				want, readErr := os.ReadFile(filepath.Join(rowMajorDir, "out", kind, name+extension))
				if readErr != nil {
					t.Fatalf("read %s golden: %v", kind, readErr)
				}
				// Our expression arena numbers _eN differently from Rust's
				// (see TestSPVTextReference): rename both by first use.
				normalizedGot, normalizedWant := normalizeExprNames(got), normalizeExprNames(string(want))
				if normalizedGot != normalizedWant {
					t.Errorf("%s differs from Rust naga:\n%s", kind, firstDiff(normalizedWant, normalizedGot))
				}
			}

			// Rust's Oracle ran with Options::default(): an empty per-entry-point
			// map and fake_missing_bindings, so every resource gets
			// [[user(fake0)]]. A nil map would take our automatic slot numbering
			// instead, which is a convenience Rust does not have.
			mslText, _, err := msl.Compile(module, msl.Options{
				LangVersion:         msl.Version1_0,
				PerEntryPointMap:    map[string]msl.EntryPointResources{},
				FakeMissingBindings: true,
			})
			compare("msl", ".metal", mslText, err)

			hlslText, _, err := hlsl.Compile(module, hlsl.Options{FakeMissingBindings: true})
			compare("hlsl", ".hlsl", hlslText, err)

			// Rust's GLSL defaults: GLES 3.10 and no bounds checks (its
			// glsl::Options has no policy field), which our Restrict-by-default
			// zero value would not reproduce.
			glslText, _, err := glsl.Compile(module, glsl.Options{
				LangVersion: glsl.Version{Major: 3, Minor: 10, ES: true},
				BoundsCheckPolicies: backend.BoundsCheckPolicies{
					Index:        backend.BoundsCheckUnchecked,
					Buffer:       backend.BoundsCheckUnchecked,
					ImageLoad:    backend.BoundsCheckUnchecked,
					BindingArray: backend.BoundsCheckUnchecked,
				},
			})
			compare("glsl", ".glsl", glslText, err)
		})
	}

	// The store fixture pins the refusal F1 restored: a store through a
	// row-major matrix must not parse until the spec-correct store lands.
	t.Run("rm-store", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(rowMajorDir, "spv", "rm-store.spv"))
		if err != nil {
			t.Fatalf("read fixture: %v", err)
		}
		if _, _, err := spirv.Parse(data, spirv.ParseOptions{}); err == nil {
			t.Fatal("the store through a row-major matrix parsed; want the refusal")
		} else if !strings.Contains(err.Error(), "a store through a row-major matrix is not supported") {
			t.Errorf("error = %v, want the row-major store refusal", err)
		}
	})
}
