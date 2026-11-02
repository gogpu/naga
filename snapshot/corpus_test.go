package snapshot_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gogpu/naga/internal/testcorpus"
	"github.com/gogpu/naga/valid"
)

// Test corpus layout (ADR-006). This file is the only place that knows it.
//
//	testdata/rust/          Rust naga corpus, vendored verbatim (MANIFEST.json)
//	  in/wgsl/<name>.wgsl   shader inputs
//	  in/wgsl/<name>.toml   Rust test parameters (targets, backend options)
//	  out/<kind>/...        Rust naga outputs: spv, msl (.metal), hlsl, glsl, ir
//	testdata/rust-derived/  Rust naga outputs upstream does not commit (MANIFEST.json)
//	  out/<kind>/...
//	testdata/own/in/        our own shaders (not part of Rust parity)
//	testdata/golden/        our own output snapshots
const (
	rustCorpusDir  = "testdata/rust"
	rustDerivedDir = "testdata/rust-derived"
	ownCorpusDir   = "testdata/own"
	goldenDir      = "testdata/golden"
)

var (
	// rustInputDir holds the vendored WGSL inputs and their TOML parameters.
	rustInputDir = filepath.Join(rustCorpusDir, "in", "wgsl")
	ownInputDir  = filepath.Join(ownCorpusDir, "in")
)

// unvalidated is the ModuleInfo the snapshot tests give ProcessOverrides:
// they compile corpus modules they do not validate first, so there is none.
// ProcessOverrides returns it as is for a module with nothing to process,
// and a fresh one for a processed module.
var unvalidated valid.ModuleInfo

// Rust naga output kinds: subdirectories of out/ in both Rust trees.
const (
	rustOutSPIRV = "spv"
	rustOutMSL   = "msl"
	rustOutHLSL  = "hlsl"
	rustOutGLSL  = "glsl"
	rustOutIR    = "ir"
)

// shaderOrigin says which corpus a shader belongs to.
type shaderOrigin int

const (
	originRust shaderOrigin = iota + 1
	originOwn
)

func (o shaderOrigin) String() string {
	if o == originRust {
		return "rust"
	}
	return "own"
}

// loadCorpus returns every corpus shader (Rust and own) sorted by name. Names
// are unique across both corpora: a shader is identified by name alone in
// reference and golden file names.
func loadCorpus(t *testing.T) []shaderFile {
	t.Helper()
	shaders := append(loadShaderDir(t, rustInputDir, originRust), loadShaderDir(t, ownInputDir, originOwn)...)
	if len(shaders) == 0 {
		t.Fatalf("no shaders found in %s or %s", rustInputDir, ownInputDir)
	}
	sort.Slice(shaders, func(i, j int) bool { return shaders[i].name < shaders[j].name })
	for i := 1; i < len(shaders); i++ {
		if shaders[i].name == shaders[i-1].name {
			t.Fatalf("shader %q exists in both the Rust and the own corpus", shaders[i].name)
		}
	}
	return shaders
}

func loadShaderDir(t *testing.T, dir string, origin shaderOrigin) []shaderFile {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read shader directory %q: %v", dir, err)
	}
	var shaders []shaderFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wgsl") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatalf("read shader %q: %v", entry.Name(), readErr)
		}
		shaders = append(shaders, newShaderFile(strings.TrimSuffix(entry.Name(), ".wgsl"), string(data), origin))
	}
	return shaders
}

// rustInputSpecDirectives are directives the WGSL spec requires that a vendored
// Rust input lacks because Rust's frontend does not implement them. They are
// put before the first line, on that line, so the input's line numbers stay
// Rust's; the vendored file stays as upstream has it. A redundant enable is
// valid WGSL, so an entry keeps working once Rust adds the directive.
var rustInputSpecDirectives = map[string]string{
	// The spec's enable-extensions table: subgroup built-in values and
	// functions and quad functions need `enable subgroups;`. Rust maps the
	// name to Unimplemented and rejects the directive
	// (front/wgsl/parse/directive/enable_extension.rs), so its test input
	// cannot carry it.
	"subgroup-operations": "enable subgroups; ",
}

// newShaderFile is the corpus shader read from a file: a Rust input gets the
// directives of rustInputSpecDirectives.
func newShaderFile(name, source string, origin shaderOrigin) shaderFile {
	if origin == originRust {
		source = rustInputSpecDirectives[name] + source
	}
	return shaderFile{name: name, source: source, origin: origin}
}

// shaderInputPath returns the WGSL input of a corpus shader by name.
func shaderInputPath(name string) (string, bool) {
	for _, dir := range []string{rustInputDir, ownInputDir} {
		p := filepath.Join(dir, name+".wgsl")
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// corpusShader loads a corpus shader by name.
func corpusShader(name string) (shaderFile, error) {
	for _, d := range []struct {
		dir    string
		origin shaderOrigin
	}{{rustInputDir, originRust}, {ownInputDir, originOwn}} {
		src, err := os.ReadFile(filepath.Join(d.dir, name+".wgsl"))
		if err == nil {
			return newShaderFile(name, string(src), d.origin), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return shaderFile{}, err
		}
	}
	return shaderFile{}, fmt.Errorf("no corpus shader %q", name)
}

// rustParamsPath returns the path of a shader's Rust test parameters (TOML).
// The file may not exist: upstream omits it when defaults apply.
func rustParamsPath(shaderName string) string {
	return filepath.Join(rustInputDir, shaderName+".toml")
}

// rustOutputDirs returns the directories holding Rust naga outputs of a kind:
// vendored first, then derived.
func rustOutputDirs(kind string) []string {
	return []string{
		filepath.Join(rustCorpusDir, "out", kind),
		filepath.Join(rustDerivedDir, "out", kind),
	}
}

// rustReferencePath returns the Rust naga output file of a kind (vendored or
// derived) and whether it exists.
func rustReferencePath(kind, fileName string) (string, bool) {
	for _, dir := range rustOutputDirs(kind) {
		p := filepath.Join(dir, fileName)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// rustReferenceFiles lists the Rust naga output files of a kind whose base
// name matches, from both trees, sorted by base name. A tree without outputs
// of that kind contributes nothing (the derived tree has no GLSL, for one).
func rustReferenceFiles(t *testing.T, kind string, match func(name string) bool) []string {
	t.Helper()
	var files []string
	for _, dir := range rustOutputDirs(kind) {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("read reference directory %q: %v", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && match(e.Name()) {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) < filepath.Base(files[j]) })
	return files
}

// TestRustCorpusIntegrity checks the Rust corpus trees against their
// manifests: vendored files must stay byte-identical to upstream, and derived
// files to what was generated. Update them with internal/cmd/corpussync, never
// by hand.
func TestRustCorpusIntegrity(t *testing.T) {
	for _, tree := range []struct {
		dir  string
		kind testcorpus.Kind
	}{
		{rustCorpusDir, testcorpus.KindVendored},
		{rustDerivedDir, testcorpus.KindDerived},
	} {
		t.Run(filepath.Base(tree.dir), func(t *testing.T) {
			m, err := testcorpus.Load(tree.dir)
			if err != nil {
				t.Fatal(err)
			}
			if m.Kind != tree.kind {
				t.Errorf("%s: manifest kind = %q, want %q", tree.dir, m.Kind, tree.kind)
			}
			problems, err := testcorpus.Verify(tree.dir)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range problems {
				t.Errorf("%s/%s", tree.dir, p)
			}
		})
	}
}
