package hlsl

import (
	"runtime"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// ---------------------------------------------------------------------------
// Test shader sources for HLSL backend benchmarks
// ---------------------------------------------------------------------------

const hlslBenchSmall = `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`

const hlslBenchMedium = `
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var out: VertexOutput;
    var pos = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),
        vec2<f32>(-0.5, -0.5),
        vec2<f32>(0.5, -0.5)
    );
    out.position = vec4<f32>(pos[idx], 0.0, 1.0);
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`

const hlslBenchLarge = `
struct Camera {
    view_proj: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> camera: Camera;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) world_pos: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
}

@vertex
fn vs_main(
    @location(0) pos: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
) -> VertexOutput {
    var out: VertexOutput;
    out.position = vec4<f32>(pos.x, pos.y, pos.z, 1.0);
    out.world_pos = pos;
    out.normal = normal;
    out.uv = uv;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let N = normalize(in.normal);
    let light_pos = vec3<f32>(10.0, 10.0, 10.0);
    let light_color = vec3<f32>(1.0, 1.0, 1.0);
    let L = normalize(light_pos - in.world_pos);
    let NdotL = max(dot(N, L), 0.0);
    let diffuse = light_color * NdotL;
    let view_dir = normalize(vec3<f32>(0.0, 0.0, 5.0) - in.world_pos);
    let half_dir = normalize(L + view_dir);
    let NdotH = max(dot(N, half_dir), 0.0);
    let spec_power = pow(NdotH, 32.0);
    let specular = light_color * spec_power;
    let ambient = vec3<f32>(0.05, 0.05, 0.05);
    let base_color = vec3<f32>(0.8, 0.2, 0.2);
    let final_color = ambient + base_color * diffuse + specular * 0.5;
    return vec4<f32>(final_color.x, final_color.y, final_color.z, 1.0);
}
`

type hlslBenchCase struct {
	name   string
	source string
}

var hlslBenchShaders = []hlslBenchCase{
	{"small", hlslBenchSmall},
	{"medium", hlslBenchMedium},
	{"large", hlslBenchLarge},
}

// hlslParseToIR parses WGSL source and lowers to IR.
func hlslParseToIR(b *testing.B, source string) *ir.Module {
	b.Helper()
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		b.Fatalf("tokenize failed: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		b.Fatalf("lower failed: %v", err)
	}
	return module
}

// ---------------------------------------------------------------------------
// HLSL emit benchmarks
// ---------------------------------------------------------------------------

// BenchmarkHLSLEmit benchmarks HLSL code generation (IR to string)
// for shaders of different complexity.
func BenchmarkHLSLEmit(b *testing.B) {
	for _, bc := range hlslBenchShaders {
		b.Run(bc.name, func(b *testing.B) {
			module := hlslParseToIR(b, bc.source)
			opts := DefaultOptions()

			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			var result string
			for i := 0; i < b.N; i++ {
				var err error
				result, _, err = Compile(module, opts)
				if err != nil {
					b.Fatalf("hlsl emit failed: %v", err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}

// BenchmarkHLSLShaderModels benchmarks HLSL generation across different
// shader model targets for the same shader.
func BenchmarkHLSLShaderModels(b *testing.B) {
	module := hlslParseToIR(b, hlslBenchMedium)

	models := []struct {
		name  string
		model ShaderModel
	}{
		{"SM_5_0", ShaderModel5_0},
		{"SM_5_1", ShaderModel5_1},
		{"SM_6_0", ShaderModel6_0},
	}

	for _, sm := range models {
		b.Run(sm.name, func(b *testing.B) {
			opts := DefaultOptions()
			opts.ShaderModel = sm.model

			b.ReportAllocs()
			b.SetBytes(int64(len(hlslBenchMedium)))
			b.ResetTimer()

			var result string
			for i := 0; i < b.N; i++ {
				var err error
				result, _, err = Compile(module, opts)
				if err != nil {
					b.Fatalf("hlsl %s emit failed: %v", sm.name, err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}
