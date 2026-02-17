package spirv

import (
	"runtime"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// ---------------------------------------------------------------------------
// Test shader sources for SPIR-V backend benchmarks
// ---------------------------------------------------------------------------

const spirvBenchSmall = `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`

const spirvBenchMedium = `
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

const spirvBenchLarge = `
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

type spirvBenchCase struct {
	name   string
	source string
}

var spirvBenchShaders = []spirvBenchCase{
	{"small", spirvBenchSmall},
	{"medium", spirvBenchMedium},
	{"large", spirvBenchLarge},
}

// parseToIR is a test helper that parses WGSL source and lowers to IR.
func parseToIR(b *testing.B, source string) *ir.Module {
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
// SPIR-V emit benchmarks
// ---------------------------------------------------------------------------

// BenchmarkSPIRVEmit benchmarks SPIR-V code generation (IR to binary)
// for shaders of different complexity. Only the backend emit phase is measured.
func BenchmarkSPIRVEmit(b *testing.B) {
	for _, bc := range spirvBenchShaders {
		b.Run(bc.name, func(b *testing.B) {
			module := parseToIR(b, bc.source)
			opts := Options{
				Version: Version1_3,
				Debug:   false,
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			var result []byte
			for i := 0; i < b.N; i++ {
				backend := NewBackend(opts)
				var err error
				result, err = backend.Compile(module)
				if err != nil {
					b.Fatalf("spirv emit failed: %v", err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}

// BenchmarkSPIRVEmitWithDebug benchmarks SPIR-V code generation with debug
// info (OpName instructions) to measure the overhead of debug output.
func BenchmarkSPIRVEmitWithDebug(b *testing.B) {
	for _, bc := range spirvBenchShaders {
		b.Run(bc.name, func(b *testing.B) {
			module := parseToIR(b, bc.source)
			opts := Options{
				Version: Version1_3,
				Debug:   true,
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(bc.source)))
			b.ResetTimer()

			var result []byte
			for i := 0; i < b.N; i++ {
				backend := NewBackend(opts)
				var err error
				result, err = backend.Compile(module)
				if err != nil {
					b.Fatalf("spirv emit with debug failed: %v", err)
				}
			}
			runtime.KeepAlive(result)
		})
	}
}

// ---------------------------------------------------------------------------
// ModuleBuilder benchmarks
// ---------------------------------------------------------------------------

// BenchmarkModuleBuilderBuild benchmarks the ModuleBuilder.Build() method
// which serializes all accumulated instructions into the final SPIR-V binary.
func BenchmarkModuleBuilderBuild(b *testing.B) {
	// Build a representative module builder with types, constants, and functions
	setupBuilder := func() *ModuleBuilder {
		mb := NewModuleBuilder(Version1_3)

		mb.AddCapability(CapabilityShader)
		glslExt := mb.AddExtInstImport("GLSL.std.450")
		_ = glslExt
		mb.SetMemoryModel(AddressingModelLogical, MemoryModelGLSL450)

		// Types
		voidTy := mb.AddTypeVoid()
		f32Ty := mb.AddTypeFloat(32)
		vec4Ty := mb.AddTypeVector(f32Ty, 4)
		funcTy := mb.AddTypeFunction(voidTy)

		// Constants
		c0 := mb.AddConstantFloat32(f32Ty, 0.0)
		c1 := mb.AddConstantFloat32(f32Ty, 1.0)
		_ = mb.AddConstantComposite(vec4Ty, c0, c0, c0, c1)

		// Function
		_ = mb.AddFunction(funcTy, voidTy, FunctionControlNone)
		_ = mb.AddLabel()
		mb.AddReturn()
		mb.AddFunctionEnd()

		// Entry point
		mb.AddEntryPoint(ExecutionModelVertex, 0, "main", nil)

		return mb
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mb := setupBuilder()
		result := mb.Build()
		runtime.KeepAlive(result)
	}
}

// BenchmarkInstructionBuild benchmarks individual SPIR-V instruction
// building and encoding.
func BenchmarkInstructionBuild(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ib := NewInstructionBuilder()
		ib.AddWord(1) // result type
		ib.AddWord(2) // result id
		ib.AddWord(3) // operand 1
		ib.AddWord(4) // operand 2
		inst := ib.Build(OpFAdd)
		encoded := inst.Encode()
		runtime.KeepAlive(encoded)
	}
}

// BenchmarkInstructionBuildString benchmarks instruction building with
// string operands (used in OpName, OpEntryPoint, etc.).
func BenchmarkInstructionBuildString(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ib := NewInstructionBuilder()
		ib.AddWord(42)
		ib.AddString("vs_main_vertex_shader")
		inst := ib.Build(OpName)
		encoded := inst.Encode()
		runtime.KeepAlive(encoded)
	}
}

// BenchmarkModuleBuilderTypes benchmarks type registration throughput
// in the ModuleBuilder.
func BenchmarkModuleBuilderTypes(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mb := NewModuleBuilder(Version1_3)
		_ = mb.AddTypeVoid()
		_ = mb.AddTypeBool()
		f32 := mb.AddTypeFloat(32)
		i32 := mb.AddTypeInt(32, true)
		u32 := mb.AddTypeInt(32, false)
		v2 := mb.AddTypeVector(f32, 2)
		v3 := mb.AddTypeVector(f32, 3)
		v4 := mb.AddTypeVector(f32, 4)
		m4 := mb.AddTypeMatrix(v4, 4)
		_ = mb.AddTypePointer(StorageClassFunction, f32)
		_ = mb.AddTypePointer(StorageClassFunction, v4)
		_ = mb.AddTypePointer(StorageClassUniform, m4)
		_ = mb.AddTypeStruct(f32, f32, f32)
		_ = mb.AddTypeStruct(v4, v3, v2)
		runtime.KeepAlive(i32)
		runtime.KeepAlive(u32)
	}
}
