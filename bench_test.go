package naga

import (
	"runtime"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// ---------------------------------------------------------------------------
// Test shader sources — realistic WGSL shaders at different complexity levels
// ---------------------------------------------------------------------------

// shaderSmallVertex is a minimal vertex shader (~4 lines of WGSL body).
const shaderSmallVertex = `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    var pos = array<vec2<f32>, 3>(
        vec2<f32>(-0.5, -0.5),
        vec2<f32>(0.5, -0.5),
        vec2<f32>(0.0, 0.5)
    );
    return vec4<f32>(pos[idx], 0.0, 1.0);
}
`

// shaderSmallFragment is a minimal fragment shader (~1 line body).
const shaderSmallFragment = `
@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`

// shaderMediumCompute is a medium-complexity compute shader with structs,
// math operations, and control flow (~20 lines of WGSL body).
const shaderMediumCompute = `
@compute @workgroup_size(64, 1, 1)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    var x: f32 = f32(gid.x);
    var y: f32 = f32(gid.y);

    let dist = sqrt(x * x + y * y);
    let angle = x / (dist + 0.001);

    var result: f32 = 0.0;
    if dist < 100.0 {
        result = sin(angle) * cos(angle);
    } else {
        result = clamp(dist / 200.0, 0.0, 1.0);
    }

    let final_val = mix(result, 1.0 - result, 0.5);
    var temp: f32 = abs(final_val);
    temp = max(temp, 0.01);
    temp = min(temp, 0.99);
}
`

// shaderMediumSDF is an SDF compute shader with distance field math.
const shaderMediumSDF = `
@compute @workgroup_size(8, 8, 1)
fn sdf_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let px: f32 = f32(gid.x) / 512.0 - 0.5;
    let py: f32 = f32(gid.y) / 512.0 - 0.5;
    let p = vec2<f32>(px, py);

    let circle_dist = length(p) - 0.3;
    let box_w: f32 = 0.2;
    let box_h: f32 = 0.2;
    let d = vec2<f32>(abs(px) - box_w, abs(py) - box_h);
    let box_dist = length(vec2<f32>(max(d.x, 0.0), max(d.y, 0.0))) + min(max(d.x, d.y), 0.0);

    let sdf = min(circle_dist, box_dist);
    let alpha = clamp(0.5 - sdf * 512.0, 0.0, 1.0);

    var color: vec4<f32>;
    if sdf < 0.0 {
        color = vec4<f32>(0.2, 0.6, 1.0, alpha);
    } else {
        color = vec4<f32>(0.0, 0.0, 0.0, alpha);
    }
}
`

// shaderLargeFragment is a large PBR-like fragment shader with multiple
// calculations, texture-like operations, lighting, and normal mapping (~60+ lines).
const shaderLargeFragment = `
struct Camera {
    view_proj: mat4x4<f32>,
}

struct Light {
    position: vec3<f32>,
    color: vec3<f32>,
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
    let shininess: f32 = 32.0;
    let spec_power = pow(NdotH, shininess);
    let specular = light_color * spec_power;

    let ambient = vec3<f32>(0.05, 0.05, 0.05);
    let base_color = vec3<f32>(0.8, 0.2, 0.2);

    let final_color = ambient + base_color * diffuse + specular * 0.5;

    let tone_mapped = final_color / (final_color + vec3<f32>(1.0, 1.0, 1.0));

    let gamma: f32 = 1.0 / 2.2;
    let corrected = vec3<f32>(
        pow(tone_mapped.x, gamma),
        pow(tone_mapped.y, gamma),
        pow(tone_mapped.z, gamma),
    );

    return vec4<f32>(corrected.x, corrected.y, corrected.z, 1.0);
}
`

// shaderTriangleVertexFragment is a complete vertex+fragment pipeline.
const shaderTriangleVertexFragment = `
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

// ---------------------------------------------------------------------------
// Complexity-grouped shaders for table-driven benchmarks
// ---------------------------------------------------------------------------

type shaderCase struct {
	name   string
	source string
}

var shadersByComplexity = []shaderCase{
	{"small_vertex", shaderSmallVertex},
	{"small_fragment", shaderSmallFragment},
	{"medium_compute", shaderMediumCompute},
	{"medium_sdf", shaderMediumSDF},
	{"large_pbr", shaderLargeFragment},
	{"triangle_pipeline", shaderTriangleVertexFragment},
}

// ---------------------------------------------------------------------------
// End-to-End: SPIR-V compilation benchmarks by complexity
// ---------------------------------------------------------------------------

// BenchmarkCompileSPIRV benchmarks full WGSL-to-SPIR-V compilation
// grouped by shader complexity. Reports allocations and throughput in bytes/sec.
func BenchmarkCompileSPIRV(b *testing.B) {
	for _, sc := range shadersByComplexity {
		b.Run(sc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(sc.source)))
			b.ResetTimer()

			var result []byte
			for i := 0; i < b.N; i++ {
				var err error
				result, err = CompileWithOptions(sc.source, CompileOptions{
					SPIRVVersion: spirv.Version1_3,
					Debug:        false,
					Validate:     false,
				})
				if err != nil {
					b.Fatalf("compile failed: %v", err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}

// BenchmarkCompileSPIRVWithValidation benchmarks compilation with IR validation
// enabled, measuring the overhead of the validation pass.
func BenchmarkCompileSPIRVWithValidation(b *testing.B) {
	for _, sc := range shadersByComplexity {
		b.Run(sc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(sc.source)))
			b.ResetTimer()

			var result []byte
			for i := 0; i < b.N; i++ {
				var err error
				result, err = CompileWithOptions(sc.source, CompileOptions{
					SPIRVVersion: spirv.Version1_3,
					Debug:        false,
					Validate:     true,
				})
				if err != nil {
					b.Fatalf("compile failed: %v", err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}

// BenchmarkCompileSPIRVWithDebug benchmarks compilation with debug info
// (OpName, OpLine instructions) to measure debug overhead.
func BenchmarkCompileSPIRVWithDebug(b *testing.B) {
	source := shaderTriangleVertexFragment
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	var result []byte
	for i := 0; i < b.N; i++ {
		var err error
		result, err = CompileWithOptions(source, CompileOptions{
			SPIRVVersion: spirv.Version1_3,
			Debug:        true,
			Validate:     false,
		})
		if err != nil {
			b.Fatalf("compile failed: %v", err)
		}
	}
	runtime.KeepAlive(result)
}

// ---------------------------------------------------------------------------
// Cross-backend comparison: same shader compiled to all 4 targets
// ---------------------------------------------------------------------------

// BenchmarkCompileAllBackends benchmarks the same medium shader compiled
// to SPIR-V, GLSL, HLSL, and MSL for cross-backend comparison.
func BenchmarkCompileAllBackends(b *testing.B) {
	source := shaderTriangleVertexFragment

	// Pre-parse and lower once; we benchmark only the backend emit phase
	ast, err := Parse(source)
	if err != nil {
		b.Fatalf("parse failed: %v", err)
	}
	module, err := Lower(ast)
	if err != nil {
		b.Fatalf("lower failed: %v", err)
	}

	b.Run("SPIRV", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result []byte
		for i := 0; i < b.N; i++ {
			opts := spirv.Options{
				Version: spirv.Version1_3,
				Debug:   false,
			}
			backend := spirv.NewBackend(opts)
			result, err = backend.Compile(module)
			if err != nil {
				b.Fatalf("spirv compile failed: %v", err)
			}
		}
		runtime.KeepAlive(result)
	})

	b.Run("GLSL", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result string
		for i := 0; i < b.N; i++ {
			var glslErr error
			result, _, glslErr = glsl.Compile(module, glsl.DefaultOptions())
			if glslErr != nil {
				b.Fatalf("glsl compile failed: %v", glslErr)
			}
		}
		runtime.KeepAlive(result)
	})

	b.Run("HLSL", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result string
		for i := 0; i < b.N; i++ {
			var hlslErr error
			result, _, hlslErr = hlsl.Compile(module, hlsl.DefaultOptions())
			if hlslErr != nil {
				b.Fatalf("hlsl compile failed: %v", hlslErr)
			}
		}
		runtime.KeepAlive(result)
	})

	b.Run("MSL", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result string
		for i := 0; i < b.N; i++ {
			var mslErr error
			result, _, mslErr = msl.Compile(module, msl.DefaultOptions())
			if mslErr != nil {
				b.Fatalf("msl compile failed: %v", mslErr)
			}
		}
		runtime.KeepAlive(result)
	})
}

// ---------------------------------------------------------------------------
// Full pipeline benchmarks: WGSL source to each backend output
// ---------------------------------------------------------------------------

// BenchmarkFullPipeline benchmarks the complete pipeline from WGSL source text
// through parsing, lowering, and code generation for each backend.
func BenchmarkFullPipeline(b *testing.B) {
	source := shaderTriangleVertexFragment

	b.Run("SPIRV", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result []byte
		for i := 0; i < b.N; i++ {
			var compErr error
			result, compErr = CompileWithOptions(source, CompileOptions{
				SPIRVVersion: spirv.Version1_3,
				Validate:     false,
			})
			if compErr != nil {
				b.Fatalf("spirv pipeline failed: %v", compErr)
			}
		}
		runtime.KeepAlive(result)
	})

	b.Run("GLSL", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result string
		for i := 0; i < b.N; i++ {
			ast, err := Parse(source)
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}
			module, err := Lower(ast)
			if err != nil {
				b.Fatalf("lower failed: %v", err)
			}
			var glslErr error
			result, _, glslErr = glsl.Compile(module, glsl.DefaultOptions())
			if glslErr != nil {
				b.Fatalf("glsl pipeline failed: %v", glslErr)
			}
		}
		runtime.KeepAlive(result)
	})

	b.Run("HLSL", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result string
		for i := 0; i < b.N; i++ {
			ast, err := Parse(source)
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}
			module, err := Lower(ast)
			if err != nil {
				b.Fatalf("lower failed: %v", err)
			}
			var hlslErr error
			result, _, hlslErr = hlsl.Compile(module, hlsl.DefaultOptions())
			if hlslErr != nil {
				b.Fatalf("hlsl pipeline failed: %v", hlslErr)
			}
		}
		runtime.KeepAlive(result)
	})

	b.Run("MSL", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(source)))
		b.ResetTimer()

		var result string
		for i := 0; i < b.N; i++ {
			ast, err := Parse(source)
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}
			module, err := Lower(ast)
			if err != nil {
				b.Fatalf("lower failed: %v", err)
			}
			var mslErr error
			result, _, mslErr = msl.Compile(module, msl.DefaultOptions())
			if mslErr != nil {
				b.Fatalf("msl pipeline failed: %v", mslErr)
			}
		}
		runtime.KeepAlive(result)
	})
}

// ---------------------------------------------------------------------------
// Individual pipeline stage benchmarks (parse, lower, generate)
// ---------------------------------------------------------------------------

// BenchmarkParse benchmarks WGSL parsing (tokenization + AST construction)
// for shaders of different complexity.
func BenchmarkParse(b *testing.B) {
	for _, sc := range shadersByComplexity {
		b.Run(sc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(sc.source)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				ast, err := Parse(sc.source)
				if err != nil {
					b.Fatalf("parse failed: %v", err)
				}
				runtime.KeepAlive(ast)
			}
		})
	}
}

// BenchmarkLower benchmarks AST-to-IR lowering for shaders of different complexity.
func BenchmarkLower(b *testing.B) {
	for _, sc := range shadersByComplexity {
		b.Run(sc.name, func(b *testing.B) {
			ast, err := Parse(sc.source)
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(sc.source)))
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				module, lErr := LowerWithSource(ast, sc.source)
				if lErr != nil {
					b.Fatalf("lower failed: %v", lErr)
				}
				runtime.KeepAlive(module)
			}
		})
	}
}

// BenchmarkValidate benchmarks IR validation for shaders of different complexity.
func BenchmarkValidate(b *testing.B) {
	for _, sc := range shadersByComplexity {
		b.Run(sc.name, func(b *testing.B) {
			ast, err := Parse(sc.source)
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}
			module, err := Lower(ast)
			if err != nil {
				b.Fatalf("lower failed: %v", err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				errs, vErr := Validate(module)
				if vErr != nil {
					b.Fatalf("validate failed: %v", vErr)
				}
				runtime.KeepAlive(errs)
			}
		})
	}
}

// BenchmarkGenerateSPIRV benchmarks only the SPIR-V code generation stage
// (IR to binary) for shaders of different complexity.
func BenchmarkGenerateSPIRV(b *testing.B) {
	for _, sc := range shadersByComplexity {
		b.Run(sc.name, func(b *testing.B) {
			ast, err := Parse(sc.source)
			if err != nil {
				b.Fatalf("parse failed: %v", err)
			}
			module, err := Lower(ast)
			if err != nil {
				b.Fatalf("lower failed: %v", err)
			}

			opts := spirv.Options{
				Version: spirv.Version1_3,
				Debug:   false,
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(sc.source)))
			b.ResetTimer()

			var result []byte
			for i := 0; i < b.N; i++ {
				backend := spirv.NewBackend(opts)
				result, err = backend.Compile(module)
				if err != nil {
					b.Fatalf("spirv generate failed: %v", err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}
