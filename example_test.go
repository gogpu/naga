package naga_test

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

// The README's examples, kept compiling and running here.

const exampleShader = `
override gain: f32 = 1.0;

@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    data[id.x] = data[id.x] * gain;
}
`

// Compile WGSL to SPIR-V with the safe defaults.
func ExampleCompile() {
	spirvBytes, err := naga.Compile(exampleShader)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(spirvBytes) > 0)
	// Output: true
}

// Parse, validate with the device's capabilities, then write any backend.
func Example_backends() {
	module, _, err := wgsl.Parse(exampleShader, wgsl.ParseOptions{})
	if err != nil {
		var pe *wgsl.ParseError
		if errors.As(err, &pe) {
			fmt.Println(pe.Format(exampleShader, "shader.wgsl"))
		}
		return
	}
	info, err := valid.Validate(module, valid.Options{Capabilities: valid.CapShaderFloat16})
	if err != nil {
		fmt.Println(err)
		return
	}

	// Backends take a module without overrides: give them their values first.
	module, _, err = naga.ProcessOverrides(module, info, naga.OverrideOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	_, _, errSPIRV := spirv.Compile(module, spirv.Options{})
	_, _, errMSL := msl.Compile(module, msl.Options{})
	_, _, errGLSL := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450})
	_, _, errHLSL := hlsl.Compile(module, hlsl.Options{})
	_, _, errDXIL := dxil.Compile(module, dxil.Options{})
	fmt.Println(errors.Join(errSPIRV, errMSL, errGLSL, errHLSL, errDXIL))
	// Output: <nil>
}

// Give overrides their values per pipeline, then compile.
func ExampleProcessOverrides() {
	module, _, err := wgsl.Parse(exampleShader, wgsl.ParseOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}
	info, err := valid.Validate(module, valid.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	pipelineModule, _, err := naga.ProcessOverrides(module, info, naga.OverrideOptions{
		Constants:  naga.PipelineConstants{"gain": 1.5},
		EntryPoint: &backend.EntryPointSelector{Stage: ir.StageCompute, Name: "main"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	code, _, err := msl.Compile(pipelineModule, msl.Options{})
	fmt.Println(err == nil && len(code) > 0)
	// Output: true
}
