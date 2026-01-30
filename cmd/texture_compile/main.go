package main

import (
	"fmt"
	"os"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/spirv"
)

const textureShader = `
@group(0) @binding(0) var texSampler: sampler;
@group(0) @binding(1) var tex: texture_2d<f32>;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 6>(
        vec2<f32>(-1.0,  1.0),
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 1.0, -1.0),
        vec2<f32>(-1.0,  1.0),
        vec2<f32>( 1.0, -1.0),
        vec2<f32>( 1.0,  1.0)
    );

    var uvs = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0),
        vec2<f32>(0.0, 1.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(0.0, 0.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(1.0, 0.0)
    );

    var output: VertexOutput;
    output.position = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    output.uv = uvs[vertexIndex];
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    return textureSample(tex, texSampler, input.uv);
}
`

func main() {
	// Parse WGSL
	ast, err := naga.Parse(textureShader)
	if err != nil {
		fmt.Println("Parse error:", err)
		os.Exit(1)
	}

	// Lower to IR
	ir, err := naga.Lower(ast)
	if err != nil {
		fmt.Println("Lower error:", err)
		os.Exit(1)
	}

	fmt.Println("=== IR Module ===")
	fmt.Printf("Types: %d\n", len(ir.Types))
	fmt.Printf("GlobalVariables: %d\n", len(ir.GlobalVariables))
	fmt.Printf("Functions: %d\n", len(ir.Functions))
	fmt.Printf("EntryPoints: %d\n", len(ir.EntryPoints))

	for i, gv := range ir.GlobalVariables {
		fmt.Printf("  GlobalVar[%d]: name=%s, space=%v, binding=%v\n", i, gv.Name, gv.Space, gv.Binding)
	}

	// Compile to SPIR-V (use DefaultOptions for proper version)
	backend := spirv.NewBackend(spirv.DefaultOptions())
	spv, err := backend.Compile(ir)
	if err != nil {
		fmt.Println("Compile error:", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== SPIR-V ===\n")
	fmt.Printf("Size: %d bytes\n", len(spv))

	// Save to file
	err = os.WriteFile("test_texture.spv", spv, 0600)
	if err != nil {
		fmt.Println("Write error:", err)
		os.Exit(1)
	}
	fmt.Println("Saved to test_texture.spv")
}
