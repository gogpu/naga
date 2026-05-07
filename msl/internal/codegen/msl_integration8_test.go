package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: Pipeline constants with diverse expression kinds
// (exercise adjustExprHandles for each ExprKind case)
// =============================================================================

func TestIntegration8_PipelineConstantsWithStore(t *testing.T) {
	// Exercises adjustBlockHandles for StmtStore and ExprLoad
	src := `
override INIT: f32 = 0.0;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var x: f32 = INIT;
    x = x + 1.0;
    out.v = x;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"INIT": 100.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "100.0")
}

func TestIntegration8_PipelineConstantsWithIf(t *testing.T) {
	// Exercises adjustBlockHandles for StmtIf
	src := `
override MODE: i32 = 0;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    if MODE == 1 {
        out.v = 1.0;
    } else {
        out.v = 0.0;
    }
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"MODE": 1}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "if (")
}

func TestIntegration8_PipelineConstantsWithLoop(t *testing.T) {
	// Exercises adjustBlockHandles for StmtLoop
	src := `
override COUNT: u32 = 10u;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var sum: f32 = 0.0;
    var i: u32 = 0u;
    loop {
        if i >= COUNT { break; }
        sum = sum + 1.0;
        i = i + 1u;
    }
    out.v = sum;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"COUNT": 5}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "5u")
}

func TestIntegration8_PipelineConstantsWithSwitch(t *testing.T) {
	// Exercises adjustBlockHandles for StmtSwitch
	src := `
override SEL: i32 = 0;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    switch SEL {
        case 0: { out.v = 10.0; }
        case 1: { out.v = 20.0; }
        default: { out.v = 0.0; }
    }
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"SEL": 1}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "switch")
}

func TestIntegration8_PipelineConstantsWithMultipleOverrides(t *testing.T) {
	// Multiple overrides in one function body
	src := `
override A: f32 = 1.0;
override B: f32 = 2.0;
override C: f32 = 3.0;
override D: f32 = 4.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v * A + B * C - D;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 10, "B": 20, "C": 30, "D": 40}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "10.0")
	mustContainMSL(t, code, "20.0")
	mustContainMSL(t, code, "30.0")
	mustContainMSL(t, code, "40.0")
}

func TestIntegration8_PipelineConstantsWithCast(t *testing.T) {
	// ExprAs path in adjustExprHandles
	src := `
override A: i32 = 5;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = f32(A);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 7}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "7.0")
}

// =============================================================================
// Test: Pipeline constants with function calls
// (covers adjustBlockHandles for StmtCall)
// =============================================================================

func TestIntegration8_PipelineConstantsWithCall(t *testing.T) {
	src := `
override FACTOR: f32 = 1.0;

fn apply_factor(v: f32) -> f32 {
    return v * FACTOR;
}

struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = apply_factor(inp.v);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"FACTOR": 5.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "apply_factor")
	mustContainMSL(t, code, "5.0")
}

// =============================================================================
// Test: Texture sample with pipeline-constant bias
// (covers adjustExprHandles SampleLevelBias path)
// =============================================================================

func TestIntegration8_PipelineConstantsBias(t *testing.T) {
	src := `
override BIAS: f32 = 0.0;
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(tex, samp, uv, BIAS);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"BIAS": 1.5}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "bias(")
	mustContainMSL(t, code, "1.5")
}

// =============================================================================
// Test: Texture sample grad with pipeline constants
// (covers adjustExprHandles SampleLevelGradient path)
// =============================================================================

func TestIntegration8_PipelineConstantsGrad(t *testing.T) {
	src := `
override SCALE_X: f32 = 1.0;
override SCALE_Y: f32 = 1.0;
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(tex, samp, uv, vec2(SCALE_X, 0.0), vec2(0.0, SCALE_Y));
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"SCALE_X": 2.0, "SCALE_Y": 3.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "gradient2d")
}

// =============================================================================
// Test: Atomic operations with pipeline constants
// (covers adjustBlockHandles for StmtAtomic)
// =============================================================================

func TestIntegration8_PipelineConstantsAtomic(t *testing.T) {
	src := `
override INIT_VAL: u32 = 0u;
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicAdd(&data.counter, INIT_VAL);
    out.v = old;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"INIT_VAL": 42}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "atomic_fetch_add_explicit")
	mustContainMSL(t, code, "42u")
}

// =============================================================================
// Test: Pipeline constants with image store
// (covers adjustBlockHandles for StmtImageStore)
// =============================================================================

func TestIntegration8_PipelineConstantsImageStore(t *testing.T) {
	src := `
override R: f32 = 1.0;
@group(0) @binding(0) var tex: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main() {
    textureStore(tex, vec2(0i, 0i), vec4(R, 0.0, 0.0, 1.0));
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"R": 0.5}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".write(")
	mustContainMSL(t, code, "0.5")
}

// =============================================================================
// Test: Pipeline constants with Emit + Return
// (covers adjustBlockHandles for StmtEmit, StmtReturn)
// =============================================================================

func TestIntegration8_PipelineConstantsReturn(t *testing.T) {
	src := `
override RESULT: f32 = 0.0;
@vertex fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4(RESULT, 0.0, 0.0, 1.0);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"RESULT": 0.99}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "0.99")
}

// =============================================================================
// Test: Global expression — struct compose
// (covers writeGlobalExpression struct compose path)
// =============================================================================

func TestIntegration8_PrivateVarStructInit(t *testing.T) {
	src := `
struct Point { x: f32, y: f32 };
var<private> origin: Point = Point(0.0, 0.0);
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = origin.x + origin.y;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "origin")
}

// =============================================================================
// Test: Multiple private variables
// =============================================================================

func TestIntegration8_MultiplePrivateVars(t *testing.T) {
	src := `
var<private> counter: u32 = 0u;
var<private> scale: f32 = 1.0;
var<private> flag: bool = false;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    counter = counter + 1u;
    if flag { scale = 2.0; }
    out.v = scale * f32(counter);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "counter")
	mustContainMSL(t, code, "scale")
}

// =============================================================================
// Test: Constant structs (covers writeConstant for struct types)
// =============================================================================

func TestIntegration8_ConstantStruct(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tStruct := ir.TypeHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "Config", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "speed", Type: tF32, Offset: 0},
					{Name: "strength", Type: tF32, Offset: 4},
				},
				Span: 8,
			}},
		},
		Constants: []ir.Constant{
			{Name: "", Type: tF32, Value: ir.ScalarValue{Bits: 0x41200000, Kind: ir.ScalarFloat}}, // 10.0
			{Name: "", Type: tF32, Value: ir.ScalarValue{Bits: 0x41A00000, Kind: ir.ScalarFloat}}, // 20.0
			{Name: "DEFAULT_CONFIG", Type: tStruct, Value: ir.CompositeValue{Components: []ir.ConstantHandle{0, 1}}},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "DEFAULT_CONFIG")
}

// =============================================================================
// Test: Vertex pulling — multiple buffer formats
// (covers writeUnpackingFunction, vptVertexInputDimension for more formats)
// =============================================================================

func TestIntegration8_VertexPullingFloat16(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(@location(0) pos: vec2<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(pos, 0.0, 1.0);
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
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatFloat16x2},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "[[vertex_id]]")
}

func TestIntegration8_VertexPullingFloat32x4(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(@location(0) pos: vec4<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = pos;
    return out;
}
`
	opts := DefaultOptions()
	opts.VertexPullingTransform = true
	opts.VertexBufferMappings = []VertexBufferMapping{
		{
			ID:       0,
			Stride:   16,
			StepMode: VertexStepModeByVertex,
			Attributes: []AttributeMapping{
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatFloat32x4},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "[[vertex_id]]")
}

// =============================================================================
// Test: Multisampled texture with Restrict bounds
// (covers writeImageLoadRestrict multisampled path and writeRestrictCoords)
// =============================================================================

func TestIntegration8_MultisampledTextureRestrict(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;
struct In { sample_idx: i32 };
@group(0) @binding(1) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), inp.sample_idx);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
	mustContainMSL(t, code, "metal::min(")
	mustContainMSL(t, code, "get_num_samples()")
}

// =============================================================================
// Test: Local variable without initializer (zero-init)
// =============================================================================

func TestIntegration8_LocalVarZeroInit(t *testing.T) {
	src := `
struct Out { v: vec4<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var v: vec4<f32>;
    v.x = 1.0;
    out.v = v;
}
`
	code := compileWGSL(t, src)
	// Zero-initialized local should use {}
	mustContainMSL(t, code, "= {}")
}

// =============================================================================
// Test: Named expressions (let bindings) — covers named expression handling
// =============================================================================

func TestIntegration8_NamedExpressions(t *testing.T) {
	src := `
struct In { a: f32, b: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let x = inp.a;
    let y = inp.b;
    let sum = x + y;
    let product = x * y;
    out.v = sum + product;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "x")
	mustContainMSL(t, code, "sum")
}

// =============================================================================
// Test: Type emission coverage — half types
// =============================================================================

func TestIntegration8_HalfVecType(t *testing.T) {
	src := `
struct Data { v: vec4<f16> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;

enable f16;

@compute @workgroup_size(1)
fn main() {
    out.v = vec4<f32>(data.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "half")
}

// =============================================================================
// Test: WriteLiteral f16 (covers writeLiteral half path)
// =============================================================================

func TestIntegration8_LiteralF16(t *testing.T) {
	src := `
enable f16;
struct Out { v: f16 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = 1.5h;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "1.5")
}

// =============================================================================
// Test: Saturate and inverseSqrt (covers more math functions)
// =============================================================================

func TestIntegration8_InverseSqrt(t *testing.T) {
	src := computeWrapIO("out.v = inverseSqrt(inp.a);")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "rsqrt")
}

func TestIntegration8_Ldexp(t *testing.T) {
	src := `
struct In { v: f32, e: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = ldexp(inp.v, inp.e);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "ldexp")
}

// =============================================================================
// Test: Texture arrayed type  (covers texture type arrayed path)
// =============================================================================

func TestIntegration8_TextureArrayed2D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d_array<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv, 0i);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d_array<float")
}

// =============================================================================
// Test: Multiple function calls in one shader
// (covers collectCalls, collectCallsFromBlock, pass-through propagation)
// =============================================================================

func TestIntegration8_MultipleFunctionCalls(t *testing.T) {
	src := `
fn add(a: f32, b: f32) -> f32 { return a + b; }
fn mul(a: f32, b: f32) -> f32 { return a * b; }

struct In { a: f32, b: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = add(mul(inp.a, inp.b), inp.a);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float add(")
	mustContainMSL(t, code, "float mul(")
}
