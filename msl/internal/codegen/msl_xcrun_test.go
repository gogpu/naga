//go:build darwin

package codegen

import (
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/ir"
)

func TestMSLCompilesWithXcrun(t *testing.T) {
	const wgslSource = `
struct VertexOutput {
	@builtin(position) position: vec4<f32>,
	@location(0) color: vec3<f32>
}

@vertex
fn vs_main(@builtin(vertex_index) vertex_index: u32) -> VertexOutput {
	let positions = array<vec2<f32>, 3>(
		vec2<f32>(0.0, 0.5),
		vec2<f32>(-0.5, -0.5),
		vec2<f32>(0.5, -0.5)
	);
	let colors = array<vec3<f32>, 3>(
		vec3<f32>(1.0, 0.0, 0.0),
		vec3<f32>(0.0, 1.0, 0.0),
		vec3<f32>(0.0, 0.0, 1.0)
	);
	var out: VertexOutput;
	out.position = vec4<f32>(positions[vertex_index], 0.0, 1.0);
	out.color = colors[vertex_index];
	return out;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
	return vec4<f32>(input.color, 1.0);
}
`

	ast, err := naga.Parse(wgslSource)
	if err != nil {
		t.Fatalf("naga.Parse failed: %v", err)
	}

	module, err := naga.LowerWithSource(ast, wgslSource)
	if err != nil {
		t.Fatalf("naga.LowerWithSource failed: %v", err)
	}

	mslSource, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("msl.Compile failed: %v", err)
	}
	verifyMSLWithXcrun(t, mslSource)
}

func TestMSLInt64AtomicMinMaxCompilesWithXcrun(t *testing.T) {
	const wgslSource = `
@group(0) @binding(0)
var<storage, read_write> value: atomic<u64>;

@compute @workgroup_size(1)
fn main() {
    atomicMin(&value, 1lu);
    atomicMax(&value, 2lu);
}
`

	ast, err := naga.Parse(wgslSource)
	if err != nil {
		t.Fatalf("naga.Parse failed: %v", err)
	}
	module, err := naga.LowerWithSource(ast, wgslSource)
	if err != nil {
		t.Fatalf("naga.LowerWithSource failed: %v", err)
	}

	bufferSlot := uint8(0)
	options := DefaultOptions()
	options.LangVersion = Version2_4
	options.PerEntryPointMap = map[string]EntryPointResources{
		"main": {
			Resources: map[ir.ResourceBinding]BindTarget{
				{Group: 0, Binding: 0}: {
					Buffer:  &bufferSlot,
					Mutable: true,
				},
			},
		},
	}
	mslSource, _, err := Compile(module, options)
	if err != nil {
		t.Fatalf("msl.Compile failed: %v", err)
	}
	verifyMSLWithXcrun(t, mslSource)
}
