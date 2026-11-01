package naga_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// rayExamplesDir holds gfx-rs/wgpu's ray query example shaders (vendored,
// see its PROVENANCE.md).
const rayExamplesDir = "testdata/upstream/github.com/gfx-rs/wgpu/examples/features/src"

// rayExamplesUnsupported are the examples that use a feature naga does not
// parse yet, each with its card.
var rayExamplesUnsupported = map[string]string{
	// acceleration_structure<vertex_return> (Rust's EXPERIMENTAL_RAY_HIT_VERTEX_RETURN).
	"ray_cube_normals": "P2-WGSL-100",
}

// TestRayQueryExamplesCompile compiles gfx-rs/wgpu's ray query examples with
// every backend that writes ray queries, at the target versions ray queries
// need. Compaction left SpecialTypes.RayIntersection pointing past the type
// arena, so every backend panicked on ray_scene and ray_traced_triangle
// (P0-IR-025, found by ahmedtd on gogpu/wgpu#369).
func TestRayQueryExamplesCompile(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join(rayExamplesDir, "ray_*"))
	if err != nil || len(dirs) != 7 {
		t.Fatalf("ray examples: %d dirs, %v", len(dirs), err)
	}
	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, "shader.wgsl"))
			if err != nil {
				t.Fatal(err)
			}
			m, _, err := wgsl.Parse(string(src), wgsl.ParseOptions{})
			card, unsupported := rayExamplesUnsupported[filepath.Base(dir)]
			switch {
			case err != nil && unsupported:
				t.Skipf("not supported yet (%s): %v", card, err)
			case err != nil:
				t.Fatalf("parse: %v", err)
			case unsupported:
				t.Fatalf("parses now: remove it from rayExamplesUnsupported and close %s", card)
			}
			if _, err := valid.Validate(m, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("validate: %v", err)
			}
			for _, c := range rayCompilers(m) {
				if err := noPanic(func() error { return c.compile(m) }); err != nil {
					t.Errorf("%s: %v", c.name, err)
				}
			}
		})
	}
}

type rayCompiler struct {
	name    string
	compile func(*ir.Module) error
}

// rayCompilers returns each backend with ray queries at its lowest target:
// SPIR-V 1.4 (SPV_KHR_ray_query), MSL 2.4, shader model 6.5. GLSL has no ray
// queries in this backend, as in Rust naga.
func rayCompilers(m *ir.Module) []rayCompiler {
	sm65 := backend.ShaderModel{Major: 6, Minor: 5}
	cs := []rayCompiler{
		{"spirv", func(m *ir.Module) error {
			_, _, err := spirv.Compile(m, spirv.Options{Version: spirv.Version1_4})
			return err
		}},
		{"msl", func(m *ir.Module) error {
			_, _, err := msl.Compile(m, msl.Options{LangVersion: msl.Version2_4})
			return err
		}},
		{"hlsl", func(m *ir.Module) error {
			_, _, err := hlsl.Compile(m, hlsl.Options{ShaderModel: sm65, ImmediatesTarget: &hlsl.BindTarget{}})
			return err
		}},
	}
	for i := range m.EntryPoints {
		ep := &backend.EntryPointSelector{Stage: m.EntryPoints[i].Stage, Name: m.EntryPoints[i].Name}
		cs = append(cs, rayCompiler{"dxil/" + ep.Name, func(m *ir.Module) error {
			_, _, err := dxil.Compile(m, dxil.Options{ShaderModel: sm65, EntryPoint: ep, ImmediatesTarget: &dxil.BindTarget{}})
			return err
		}})
	}
	return cs
}

// noPanic runs f and turns a panic into an error.
func noPanic(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}
