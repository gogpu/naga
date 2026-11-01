package naga

import (
	"bytes"
	"sync"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// concurrentShader touches what backends rewrite on their way: a storage
// buffer, a workgroup array, a loop, an override and a helper function.
const concurrentShader = `
override scale: f32 = 2.0;
@group(0) @binding(0) var<storage, read_write> buf: array<f32, 64>;
var<workgroup> tile: array<f32, 64>;
fn twice(x: f32) -> f32 { return x * scale; }
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) i: u32) {
    tile[i] = twice(buf[i]);
    workgroupBarrier();
    var sum = 0.0;
    for (var k = 0u; k < 64u; k++) { sum += tile[k]; }
    buf[i] = sum;
}`

// TestCompileConcurrently compiles one module on every backend from many
// goroutines at once (ADR-010 §6: Compile never modifies its input and is
// safe for concurrent use). Run it with -race; every result must equal the
// one compiled alone.
func TestCompileConcurrently(t *testing.T) {
	parsed, _, err := wgsl.Parse(concurrentShader, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	module, _, err := ProcessOverrides(parsed, valid.ModuleInfo{}, OverrideOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := valid.Validate(module, valid.Options{}); err != nil {
		t.Fatal(err)
	}
	compilers := map[string]func() ([]byte, error){
		"spirv": func() ([]byte, error) { b, _, err := spirv.Compile(module, spirv.Options{}); return b, err },
		"msl": func() ([]byte, error) {
			s, _, err := msl.Compile(module, msl.Options{})
			return []byte(s), err
		},
		"glsl": func() ([]byte, error) {
			s, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version430})
			return []byte(s), err
		},
		"hlsl": func() ([]byte, error) {
			s, _, err := hlsl.Compile(module, hlsl.Options{})
			return []byte(s), err
		},
		"dxil": func() ([]byte, error) {
			b, _, err := dxil.Compile(module, dxil.Options{ShaderModel: backend.ShaderModel{Major: 6, Minor: 0}})
			return b, err
		},
	}
	want := map[string][]byte{}
	for name, compile := range compilers {
		out, err := compile()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want[name] = out
	}

	const rounds = 4
	var wg sync.WaitGroup
	errs := make(chan string, rounds*len(compilers))
	for range rounds {
		for name, compile := range compilers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				out, err := compile()
				if err != nil {
					errs <- name + ": " + err.Error()
				} else if !bytes.Equal(out, want[name]) {
					errs <- name + ": output differs from the one compiled alone"
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
