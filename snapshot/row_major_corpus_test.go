package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// rowMajorDir holds the row-major corpus: the authored GLSL sources, their
// binaries, and Rust naga's MSL/HLSL outputs as goldens. Upstream has no
// row-major rows, so the corpus lives in the derived tree with its
// PROVENANCE.md (ADR-006). The store fixtures have no goldens: Rust naga
// miscompiles a row-major store, so their expected result is the translated IR
// the store corpus test checks (card P3-SPVIN-007).
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

	testRowMajorStoreCorpus(t)
}

// rowMajorStoreFixtures are the corpus's store cases. Rust naga writes a
// row-major store untransposed (front/spv/next_block.rs:771-793), so no golden
// can be taken from it: the expected result is the translated IR — checked here
// — and every backend accepting the module.
var rowMajorStoreFixtures = []struct {
	name string
	// stores is the number of writes into the buffer: the entry point's output
	// write lives in the wrapper, not in the function this counts.
	stores int
	// transposed: the whole-matrix store writes Transpose(value).
	transposed bool
	// components: a column store is one store per component, in row order.
	components bool
	// readBack: the fixture reads the memory after the store, so a side-load
	// has to be evaluated after it.
	readBack bool
}{
	{name: "rm-store", stores: 1, transposed: true},
	{name: "rm-store-column", stores: 4, components: true, readBack: true},
	{name: "rm-store-element", stores: 1, readBack: true},
	{name: "rm-store-roundtrip", stores: 1, transposed: true, readBack: true},
}

// testRowMajorStoreCorpus reads every store fixture and checks the translation:
// the IR validates, every backend writes it, and the stores have the shape the
// row-major layout requires (card P3-SPVIN-007).
func testRowMajorStoreCorpus(t *testing.T) {
	t.Helper()
	for _, fixture := range rowMajorStoreFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(rowMajorDir, "spv", fixture.name+".spv"))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			module, _, err := spirv.Parse(data, spirv.ParseOptions{})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, err := valid.Validate(module, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("valid.Validate: %v", err)
			}
			// Every backend must write the translated module. The store fixtures
			// write a storage buffer, which MSL 1.0 cannot express.
			if _, _, err := msl.Compile(module, msl.Options{
				LangVersion:         msl.Version1_2,
				PerEntryPointMap:    map[string]msl.EntryPointResources{},
				FakeMissingBindings: true,
			}); err != nil {
				t.Fatalf("msl: %v", err)
			}
			if _, _, err := hlsl.Compile(module, hlsl.Options{FakeMissingBindings: true}); err != nil {
				t.Fatalf("hlsl: %v", err)
			}
			if _, _, err := glsl.Compile(module, glsl.Options{
				LangVersion: glsl.Version{Major: 3, Minor: 10, ES: true},
				BoundsCheckPolicies: backend.BoundsCheckPolicies{
					Index:        backend.BoundsCheckUnchecked,
					Buffer:       backend.BoundsCheckUnchecked,
					ImageLoad:    backend.BoundsCheckUnchecked,
					BindingArray: backend.BoundsCheckUnchecked,
				},
			}); err != nil {
				t.Fatalf("glsl: %v", err)
			}

			fn := &module.Functions[0]
			stores := rowMajorStores(fn, fn.Body)
			if len(stores) != fixture.stores {
				t.Fatalf("stores = %d, want %d: %#v", len(stores), fixture.stores, fn.Body)
			}
			for i, store := range stores {
				math, ok := fn.Expressions[store.Value].Kind.(ir.ExprMath)
				switch {
				case fixture.transposed:
					if !ok || math.Fun != ir.MathTranspose {
						t.Errorf("store %d value = %#v, want a Transpose", i, fn.Expressions[store.Value].Kind)
					}
				case fixture.components:
					access, ok := fn.Expressions[store.Value].Kind.(ir.ExprAccessIndex)
					if !ok || access.Index != uint32(i) {
						t.Errorf("store %d value = %#v, want component %d of the column", i, fn.Expressions[store.Value].Kind, i)
					}
				}
			}
			if fixture.readBack && !transposeAfterFirstStore(fn, stores[0]) {
				t.Errorf("no side-load is evaluated after the store: the read-back cannot see the written memory")
			}
		})
	}
}

// rowMajorStores returns every store into the buffer in a block, nested blocks
// included. The entry point's color output is written through its global
// directly, not through an access chain, and does not count.
func rowMajorStores(fn *ir.Function, block ir.Block) []ir.StmtStore {
	var stores []ir.StmtStore
	for _, statement := range block {
		switch kind := statement.Kind.(type) {
		case ir.StmtStore:
			switch fn.Expressions[kind.Pointer].Kind.(type) {
			case ir.ExprAccess, ir.ExprAccessIndex:
				stores = append(stores, kind)
			}
		case ir.StmtBlock:
			stores = append(stores, rowMajorStores(fn, kind.Block)...)
		case ir.StmtIf:
			stores = append(stores, rowMajorStores(fn, kind.Accept)...)
			stores = append(stores, rowMajorStores(fn, kind.Reject)...)
		case ir.StmtLoop:
			stores = append(stores, rowMajorStores(fn, kind.Body)...)
			stores = append(stores, rowMajorStores(fn, kind.Continuing)...)
		case ir.StmtSwitch:
			for _, c := range kind.Cases {
				stores = append(stores, rowMajorStores(fn, c.Body)...)
			}
		}
	}
	return stores
}

// transposeAfterFirstStore reports whether a Transpose expression is evaluated
// in an Emit that follows the first store: the side-load a read-back needs.
func transposeAfterFirstStore(fn *ir.Function, store ir.StmtStore) bool {
	storeIndex := -1
	for i, statement := range fn.Body {
		if kind, ok := statement.Kind.(ir.StmtStore); ok && kind == store {
			storeIndex = i
			break
		}
	}
	if storeIndex < 0 {
		return false
	}
	for i, statement := range fn.Body {
		emit, ok := statement.Kind.(ir.StmtEmit)
		if !ok || i <= storeIndex {
			continue
		}
		for handle := emit.Range.Start; handle < emit.Range.End; handle++ {
			if math, ok := fn.Expressions[handle].Kind.(ir.ExprMath); ok && math.Fun == ir.MathTranspose {
				return true
			}
		}
	}
	return false
}
