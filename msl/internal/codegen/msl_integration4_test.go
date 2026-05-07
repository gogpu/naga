package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// =============================================================================
// Test: Vertex pulling transform (covers vertex_pulling.go)
// =============================================================================

func TestIntegration_VertexPullingBasic(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(@location(0) pos: vec3<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(pos, 1.0);
    return out;
}
`
	opts := DefaultOptions()
	opts.VertexPullingTransform = true
	opts.VertexBufferMappings = []VertexBufferMapping{
		{
			ID:       0,
			Stride:   12,
			StepMode: VertexStepModeByVertex,
			Attributes: []AttributeMapping{
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatFloat32x3},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	// VPT should generate buffer type structs and vertex_id.
	mustContainMSL(t, code, "[[vertex_id]]")
}

func TestIntegration_VertexPullingMultipleAttributes(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
};
@vertex fn vs_main(@location(0) pos: vec3<f32>, @location(1) uv: vec2<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(pos, 1.0);
    out.uv = uv;
    return out;
}
`
	opts := DefaultOptions()
	opts.VertexPullingTransform = true
	opts.VertexBufferMappings = []VertexBufferMapping{
		{
			ID:       0,
			Stride:   20,
			StepMode: VertexStepModeByVertex,
			Attributes: []AttributeMapping{
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatFloat32x3},
				{ShaderLocation: 1, Offset: 12, Format: VertexFormatFloat32x2},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "[[vertex_id]]")
	mustContainMSL(t, code, "[[buffer(")
}

func TestIntegration_VertexPullingWithInstance(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(
    @location(0) pos: vec3<f32>,
    @location(1) instance_offset: vec3<f32>,
) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(pos + instance_offset, 1.0);
    return out;
}
`
	opts := DefaultOptions()
	opts.VertexPullingTransform = true
	opts.VertexBufferMappings = []VertexBufferMapping{
		{
			ID:       0,
			Stride:   12,
			StepMode: VertexStepModeByVertex,
			Attributes: []AttributeMapping{
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatFloat32x3},
			},
		},
		{
			ID:       1,
			Stride:   12,
			StepMode: VertexStepModeByInstance,
			Attributes: []AttributeMapping{
				{ShaderLocation: 1, Offset: 0, Format: VertexFormatFloat32x3},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "[[vertex_id]]")
	mustContainMSL(t, code, "[[instance_id]]")
}

func TestIntegration_VertexPullingUint8Format(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(@location(0) color: vec4<u32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(f32(color.x) / 255.0, f32(color.y) / 255.0, f32(color.z) / 255.0, 1.0);
    return out;
}
`
	opts := DefaultOptions()
	opts.VertexPullingTransform = true
	opts.VertexBufferMappings = []VertexBufferMapping{
		{
			ID:       0,
			Stride:   4,
			StepMode: VertexStepModeByVertex,
			Attributes: []AttributeMapping{
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatUint8x4},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	// Should generate unpacking function for uint8x4.
	mustContainMSL(t, code, "unpack")
}

// =============================================================================
// Test: Pipeline constants with workgroup_size override
// =============================================================================

func TestIntegration_PipelineConstantsWorkgroupSize(t *testing.T) {
	src := `
override WG_SIZE: u32 = 64u;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(WG_SIZE)
fn main(@builtin(local_invocation_index) lid: u32) {
    out.v = lid;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"WG_SIZE": 128}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "kernel")
}

// =============================================================================
// Test: CompileWithPipeline — targeted entry point compilation
// =============================================================================

func TestIntegration_CompileWithPipelineVertex(t *testing.T) {
	src := `
@vertex fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4(0.0, 0.0, 0.0, 1.0);
}
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4(1.0);
}
`
	lexer := wgsl.NewLexer(src)
	tokens, _ := lexer.Tokenize()
	parser := wgsl.NewParser(tokens)
	ast, _ := parser.Parse()
	module, _ := wgsl.Lower(ast)

	opts := DefaultOptions()
	pipeline := PipelineOptions{
		EntryPoint: &EntryPointSelector{
			Name:  "vs",
			Stage: ir.StageVertex,
		},
	}

	code, _, err := CompileWithPipeline(module, opts, pipeline)
	if err != nil {
		t.Fatalf("CompileWithPipeline failed: %v", err)
	}
	mustContainMSL(t, code, "vertex")
	mustContainMSL(t, code, "vs")
	// Fragment shader should NOT be in the output when filtering.
	mustNotContainMSL(t, code, "fragment")
}

func TestIntegration_CompileWithPipelineFragment(t *testing.T) {
	src := `
@vertex fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4(0.0, 0.0, 0.0, 1.0);
}
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4(1.0);
}
`
	lexer := wgsl.NewLexer(src)
	tokens, _ := lexer.Tokenize()
	parser := wgsl.NewParser(tokens)
	ast, _ := parser.Parse()
	module, _ := wgsl.Lower(ast)

	opts := DefaultOptions()
	pipeline := PipelineOptions{
		EntryPoint: &EntryPointSelector{
			Name:  "fs",
			Stage: ir.StageFragment,
		},
	}

	code, _, err := CompileWithPipeline(module, opts, pipeline)
	if err != nil {
		t.Fatalf("CompileWithPipeline failed: %v", err)
	}
	mustContainMSL(t, code, "fragment")
	mustContainMSL(t, code, "fs")
	mustNotContainMSL(t, code, "vertex")
}

// =============================================================================
// Test: Float division (covers naga_div for float which doesn't emit helper)
// =============================================================================

func TestIntegration_FloatDivision(t *testing.T) {
	src := computeWrapIO(`out.v = inp.a / inp.b;`)
	code := compileWGSL(t, src)
	// Float division doesn't need naga_div wrapper.
	mustContainMSL(t, code, "/")
}

func TestIntegration_FloatModulo(t *testing.T) {
	src := computeWrapIO(`out.v = inp.a % inp.b;`)
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::fmod(")
}

// =============================================================================
// Test: Various expression edge cases
// =============================================================================

func TestIntegration_VectorNegate(t *testing.T) {
	src := `
struct In { v: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = -inp.v; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "-(")
}

func TestIntegration_VectorBitwiseNot(t *testing.T) {
	src := `
struct In { v: vec4<u32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = ~inp.v; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "~")
}

// =============================================================================
// Test: Image query with LOD parameter
// =============================================================================

func TestIntegration_TextureDimensionsWithLevel(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec2<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureDimensions(tex, 1i); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".get_width(")
}

// =============================================================================
// Test: Texture depth cube array
// =============================================================================

func TestIntegration_DepthTextureCubeArray(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_depth_cube_array;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureNumLayers(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "depthcube_array<float")
}

// =============================================================================
// Test: Texture depth 2d array
// =============================================================================

func TestIntegration_DepthTexture2DArray(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_depth_2d_array;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = textureNumLayers(tex); }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "depth2d_array<float")
}

// =============================================================================
// Test: Interpolation qualifiers
// =============================================================================

func TestIntegration_InterpolationQualifiers(t *testing.T) {
	src := `
struct FragInput {
    @builtin(position) pos: vec4<f32>,
    @location(0) @interpolate(linear) linear_val: f32,
    @location(1) @interpolate(perspective, centroid) persp_centroid: f32,
    @location(2) @interpolate(flat) flat_val: u32,
};
@fragment fn fs(inp: FragInput) -> @location(0) vec4<f32> {
    return vec4(inp.linear_val, inp.persp_centroid, f32(inp.flat_val), 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "no_perspective")
	mustContainMSL(t, code, "flat")
	mustContainMSL(t, code, "centroid")
}

// =============================================================================
// Test: Nested loop with continuing block
// =============================================================================

func TestIntegration_NestedLoops(t *testing.T) {
	src := `
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    var total = 0u;
    for (var i = 0u; i < 4u; i = i + 1u) {
        for (var j = 0u; j < 4u; j = j + 1u) {
            total = total + 1u;
        }
    }
    out.v = total;
}
`
	code := compileWGSL(t, src)
	// Should have two while(true) loops.
	count := strings.Count(code, "while(true)")
	if count < 2 {
		t.Errorf("Expected at least 2 while(true) loops, got %d", count)
	}
}

// =============================================================================
// Test: Complex switch with multiple cases mapping to same body
// =============================================================================

func TestIntegration_SwitchMultipleBodies(t *testing.T) {
	src := `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    switch inp.v {
        case 0i: { out.v = 0i; }
        case 1i: { out.v = 10i; }
        case 2i: { out.v = 20i; }
        case 3i: { out.v = 30i; }
        default: { out.v = -1i; }
    }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "case 0:")
	mustContainMSL(t, code, "case 1:")
	mustContainMSL(t, code, "case 2:")
	mustContainMSL(t, code, "case 3:")
	mustContainMSL(t, code, "default:")
}

// =============================================================================
// Test: Multiple storage buffers with different groups
// =============================================================================

func TestIntegration_MultipleBindGroups(t *testing.T) {
	src := `
struct A { v: f32 };
struct B { v: f32 };
@group(0) @binding(0) var<storage, read> a: A;
@group(1) @binding(0) var<storage, read> b: B;
struct Out { v: f32 };
@group(2) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = a.v + b.v; }
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[[buffer(")
}

// =============================================================================
// Test: f16 extension usage
// =============================================================================

func TestIntegration_F16Extension(t *testing.T) {
	src := `
enable f16;
struct In { a: f16, b: f16 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f16 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = inp.a * inp.b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "half")
}

// =============================================================================
// Test: vec4<bool> relational operations
// =============================================================================

func TestIntegration_VectorBoolRelational(t *testing.T) {
	src := `
struct In { a: vec4<f32>, b: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let mask = inp.a > inp.b;
    out.v = select(inp.a, inp.b, mask);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::select(")
}

// =============================================================================
// Test: Multiple textures and samplers
// =============================================================================

func TestIntegration_MultipleTexturesAndSamplers(t *testing.T) {
	src := `
@group(0) @binding(0) var tex_a: texture_2d<f32>;
@group(0) @binding(1) var tex_b: texture_2d<f32>;
@group(0) @binding(2) var samp_a: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(3) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let a = textureSampleLevel(tex_a, samp_a, vec2(0.5f), 0.0f);
    let b = textureSampleLevel(tex_b, samp_a, vec2(0.5f), 0.0f);
    out.v = a + b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
}

// =============================================================================
// Test: Dot4I8Packed and Dot4U8Packed
// =============================================================================

func TestIntegration_Unpack4xI8(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<i32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = unpack4xI8(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "int4")
	mustContainMSL(t, code, "char4")
}

func TestIntegration_Unpack4xU8(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = unpack4xU8(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "uint4")
	mustContainMSL(t, code, "uchar4")
}

func TestIntegration_Pack4xI8(t *testing.T) {
	src := `
struct In { v: vec4<i32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = pack4xI8(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "as_type<uint>")
}

func TestIntegration_Pack4xU8(t *testing.T) {
	src := `
struct In { v: vec4<u32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = pack4xU8(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "as_type<uint>")
}

func TestIntegration_Pack4xI8Clamp(t *testing.T) {
	src := `
struct In { v: vec4<i32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = pack4xI8Clamp(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::clamp(")
}

func TestIntegration_Pack4xU8Clamp(t *testing.T) {
	src := `
struct In { v: vec4<u32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    out.v = pack4xU8Clamp(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "metal::clamp(")
}

// =============================================================================
// Test: Modf and frexp with vector arguments
// =============================================================================

func TestIntegration_ModfVector(t *testing.T) {
	src := `
struct In { v: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let r = modf(inp.v);
    out.v = r.fract;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_modf")
	mustContainMSL(t, code, "_modf_result")
}

func TestIntegration_FrexpVector(t *testing.T) {
	src := `
struct In { v: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() {
    let r = frexp(inp.v);
    out.v = r.fract;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_frexp")
	mustContainMSL(t, code, "_frexp_result")
}

// =============================================================================
// Test: CompileWithPipeline using pipeline constants
// =============================================================================

func TestIntegration_CompileWithPipelineAndConstants(t *testing.T) {
	src := `
override BRIGHTNESS: f32 = 1.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1) fn main() { out.v = inp.v * BRIGHTNESS; }
`
	lexer := wgsl.NewLexer(src)
	tokens, _ := lexer.Tokenize()
	parser := wgsl.NewParser(tokens)
	ast, _ := parser.Parse()
	module, _ := wgsl.Lower(ast)

	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"BRIGHTNESS": 2.5}
	pipeline := PipelineOptions{}

	code, _, err := CompileWithPipeline(module, opts, pipeline)
	if err != nil {
		t.Fatalf("CompileWithPipeline failed: %v", err)
	}
	mustContainMSL(t, code, "2.5")
}
