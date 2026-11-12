package front

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// FuzzParse feeds arbitrary bytes to Parse: it must return a module or an
// *Error, never panic. The seeds are the SPIR-V corpus the snapshot tests
// read plus the modules that once crashed the parser.
//
//	go test -run '^$' -fuzz '^FuzzParse$' -fuzztime 60s ./spirv/internal/front/
func FuzzParse(f *testing.F) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "snapshot", "testdata", "rust-derived", "spv", "*.spv"))
	if err != nil {
		f.Fatal(err)
	}
	if len(paths) == 0 {
		f.Fatal("no SPIR-V seeds found")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	// A five-component OpVectorShuffle wrote past the four-slot swizzle
	// pattern (index out of range [4]).
	f.Add(newBodyModule().build(
		instWords(spvenums.OpCompositeConstruct, 9, 200, 22, 22, 22, 22),
		instWords(spvenums.OpVectorShuffle, 9, 201, 200, 200, 0, 1, 2, 3, 0),
	))

	f.Fuzz(func(t *testing.T, data []byte) {
		module, _, err := Parse(data, Options{})
		if err != nil {
			var parseErr *Error
			if !errors.As(err, &parseErr) {
				t.Fatalf("Parse returned %T, want *Error: %v", err, err)
			}
			return
		}
		if module == nil {
			t.Fatal("Parse returned neither a module nor an error")
		}
	})
}
