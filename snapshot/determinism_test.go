package snapshot_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// determinismRuns is how many times TestDeterministicOutput compiles each
// shader. Go starts every map iteration at a random element, so an output
// that depends on map order differs between runs of one process; the gg
// shaders that showed it (P1-DXIL-063) differed on nearly every run.
const determinismRuns = 4

// TestDeterministicOutput compiles every corpus shader and every vendored gg
// shader with each backend several times and requires the same bytes, or the
// same error, each time. A compiler's output must depend on its input alone:
// pipeline caches key on it, and an order that changes from run to run hides
// which order a test checked.
func TestDeterministicOutput(t *testing.T) {
	type shader struct{ name, source string }
	var shaders []shader
	for _, s := range loadCorpus(t) {
		shaders = append(shaders, shader{s.name, s.source})
	}
	for _, dir := range ggProductionCorpora {
		files, err := filepath.Glob(filepath.Join(dir, "*.wgsl"))
		if err != nil || len(files) == 0 {
			t.Fatalf("gg corpus %q: %d files, %v", dir, len(files), err)
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			rel := strings.TrimPrefix(filepath.ToSlash(f), ggUpstreamDir+"/")
			shaders = append(shaders, shader{"gg/" + rel, string(src)})
		}
	}

	for _, s := range shaders {
		module, err := parseWGSL(s.source)
		if err != nil {
			continue // parse failures are other tests' business
		}
		for _, c := range determinismCompilers(module) {
			first, firstErr := c.compile(module)
			for run := 1; run < determinismRuns; run++ {
				out, err := c.compile(module)
				if errText(err) != errText(firstErr) {
					t.Errorf("%s %s: run %d error %q, run 0 error %q", s.name, c.name, run, errText(err), errText(firstErr))
					break
				}
				if !bytes.Equal(out, first) {
					t.Errorf("%s %s: run %d output differs from run 0", s.name, c.name, run)
					break
				}
			}
		}
	}
}

type determinismCompiler struct {
	name    string
	compile func(*ir.Module) ([]byte, error)
}

// determinismCompilers returns one compiler per backend, and per entry point
// for the backends that write one entry point.
func determinismCompilers(module *ir.Module) []determinismCompiler {
	cs := []determinismCompiler{
		{"spirv", func(m *ir.Module) ([]byte, error) {
			return spirvBinary(m, spirv.Options{Version: spirv.Version1_3})
		}},
		{"msl", func(m *ir.Module) ([]byte, error) {
			out, _, err := msl.Compile(m, msl.Options{})
			return []byte(out), err
		}},
		{"hlsl", func(m *ir.Module) ([]byte, error) {
			out, _, err := hlsl.Compile(m, hlsl.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 6}})
			return []byte(out), err
		}},
	}
	for i := range module.EntryPoints {
		ep := &backend.EntryPointSelector{Stage: module.EntryPoints[i].Stage, Name: module.EntryPoints[i].Name}
		cs = append(cs,
			determinismCompiler{"glsl/" + ep.Name, func(m *ir.Module) ([]byte, error) {
				out, _, err := glsl.Compile(m, glsl.Options{LangVersion: glsl.Version450, EntryPoint: ep})
				return []byte(out), err
			}},
			determinismCompiler{"dxil/" + ep.Name, func(m *ir.Module) ([]byte, error) {
				opts := dxil.DefaultOptions()
				opts.EntryPoint = ep
				return dxilAtNeededModel(m, opts)
			}},
		)
	}
	return cs
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
