package codegen

import (
	"strings"
	"testing"
)

// =============================================================================
// Test: Private variables with constant initializers
// (covers writeConstExpression, writeGlobalExpression)
// =============================================================================

func TestIntegration6_PrivateVariableInit(t *testing.T) {
	src := `
var<private> offset: f32 = 1.5;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = offset;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "1.5")
}

func TestIntegration6_PrivateVariableVecInit(t *testing.T) {
	src := `
var<private> color: vec3<f32> = vec3(1.0, 0.0, 0.0);
struct Out { v: vec3<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = color;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float3(")
}

func TestIntegration6_PrivateVariableZeroInit(t *testing.T) {
	src := `
var<private> counter: u32 = 0u;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    counter = counter + 1u;
    out.v = counter;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "counter")
}

// =============================================================================
// Test: Runtime-sized array with arrayLength
// (covers writeArrayLength, resolveArrayLengthGlobal, computeDynamicArrayLength)
// =============================================================================

func TestIntegration6_ArrayLengthRuntime(t *testing.T) {
	src := `
struct Buffer { data: array<f32> };
@group(0) @binding(0) var<storage, read> buf: Buffer;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = arrayLength(&buf.data);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "_buffer_sizes")
}

// =============================================================================
// Test: Dynamic access with bounds checking
// (covers computeDynamicArrayLength, isStaticallyInBounds, writeBoundsCheckItem)
// =============================================================================

func TestIntegration6_DynamicAccessRZSW(t *testing.T) {
	src := `
struct Buffer { data: array<f32> };
@group(0) @binding(0) var<storage, read> buf: Buffer;
struct In { idx: u32 };
@group(0) @binding(1) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = buf.data[inp.idx];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Buffer = BoundsCheckReadZeroSkipWrite
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "_buffer_sizes")
	mustContainMSL(t, code, "?")
}

func TestIntegration6_DynamicAccessRestrict(t *testing.T) {
	src := `
struct Buffer { data: array<f32> };
@group(0) @binding(0) var<storage, read> buf: Buffer;
struct In { idx: u32 };
@group(0) @binding(1) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = buf.data[inp.idx];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Buffer = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "_buffer_sizes")
	mustContainMSL(t, code, "metal::min(")
}

// =============================================================================
// Test: Regular (non-entry-point) function calls
// (covers writeFunction, pass-through globals, funcNeedsBufferSizes)
// =============================================================================

func TestIntegration6_HelperFunctionWithTexture(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

fn do_sample(uv: vec2<f32>) -> vec4<f32> {
    return textureSample(tex, samp, uv);
}

@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = do_sample(uv);
    let b = do_sample(uv + vec2(0.1, 0.0));
    return a + b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "do_sample")
	// The helper function should receive texture and sampler as pass-through params
	// Count occurrences of do_sample — should be at least 3 (definition + 2 calls)
	count := strings.Count(code, "do_sample")
	if count < 3 {
		t.Errorf("expected do_sample to appear at least 3 times (def + 2 calls), got %d", count)
	}
}

func TestIntegration6_HelperFunctionWithStorageBuffer(t *testing.T) {
	src := `
struct Data { vals: array<f32, 8> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;

fn sum_data() -> f32 {
    var total: f32 = 0.0;
    total = data.vals[0] + data.vals[1] + data.vals[2] + data.vals[3];
    return total;
}

@compute @workgroup_size(1)
fn main() {
    out.v = sum_data();
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "sum_data")
}

// =============================================================================
// Test: Helper function that needs buffer sizes pass-through
// (covers funcNeedsBufferSizes, _buffer_sizes param)
// =============================================================================

func TestIntegration6_HelperFuncBufferSizes(t *testing.T) {
	src := `
struct Buffer { data: array<f32> };
@group(0) @binding(0) var<storage, read> buf: Buffer;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;

fn get_len() -> u32 {
    return arrayLength(&buf.data);
}

@compute @workgroup_size(1)
fn main() {
    out.v = get_len();
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "_buffer_sizes")
	mustContainMSL(t, code, "get_len")
}

// =============================================================================
// Test: Multiple entry points (covers pipeline entry point selection)
// =============================================================================

func TestIntegration6_MultipleEntryPoints(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(1)
fn compute_main() {
    out.v = 1.0;
}

@vertex fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4(0.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "compute_main")
	mustContainMSL(t, code, "vs_main")
	mustContainMSL(t, code, "kernel")
	mustContainMSL(t, code, "vertex")
}

// =============================================================================
// Test: Workgroup memory with ZeroInitialize
// (covers workgroup zero-init path in writeEntryPoint)
// =============================================================================

func TestIntegration6_WorkgroupZeroInit(t *testing.T) {
	src := `
var<workgroup> shared_data: array<u32, 64>;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    shared_data[lid] = lid;
    workgroupBarrier();
    out.v = shared_data[0];
}
`
	opts := DefaultOptions()
	opts.ZeroInitializeWorkgroupMemory = true
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "threadgroup")
	mustContainMSL(t, code, "shared_data")
}

// =============================================================================
// Test: Matrix access patterns (covers writeAccessChain for matrix/vector)
// =============================================================================

func TestIntegration6_MatrixColumnAccess(t *testing.T) {
	src := `
struct In { m: mat4x4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.m[2];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "[2]")
}

func TestIntegration6_VectorComponentAccess(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v.z;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".z")
}

// =============================================================================
// Test: Deep struct access chains (covers writeAccessChain struct path)
// =============================================================================

func TestIntegration6_DeepStructAccess(t *testing.T) {
	src := `
struct Inner { val: f32 };
struct Middle { inner: Inner };
struct Outer { middle: Middle };
@group(0) @binding(0) var<storage, read> data: Outer;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.middle.inner.val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".middle")
	mustContainMSL(t, code, ".inner")
	mustContainMSL(t, code, ".val")
}

// =============================================================================
// Test: For loop (Loop + Continuing + Break If patterns)
// (covers writeLoop, writeContinuing, emitBreakIfSubExpressions)
// =============================================================================

func TestIntegration6_ForLoop(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 10u; i = i + 1u) {
        sum = sum + 1.0;
    }
    out.v = sum;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "while(true)")
}

// =============================================================================
// Test: Switch with multiple cases and default
// =============================================================================

func TestIntegration6_SwitchMultipleCases(t *testing.T) {
	src := `
struct In { sel: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    switch inp.sel {
        case 0: { out.v = 1.0; }
        case 1: { out.v = 2.0; }
        case 2: { out.v = 3.0; }
        default: { out.v = 0.0; }
    }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "switch")
	mustContainMSL(t, code, "case 0")
	mustContainMSL(t, code, "case 1")
	mustContainMSL(t, code, "case 2")
	mustContainMSL(t, code, "default")
}

// =============================================================================
// Test: Nested if-else
// =============================================================================

func TestIntegration6_NestedIfElse(t *testing.T) {
	src := `
struct In { a: f32, b: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    if inp.a > 0.0 {
        if inp.b > 0.0 {
            out.v = 1.0;
        } else {
            out.v = 2.0;
        }
    } else {
        out.v = 3.0;
    }
}
`
	code := compileWGSL(t, src)
	count := strings.Count(code, "if (")
	if count < 2 {
		t.Errorf("expected at least 2 if statements, got %d", count)
	}
}

// =============================================================================
// Test: Discard statement (fragment shader)
// =============================================================================

func TestIntegration6_Discard(t *testing.T) {
	src := `
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    if pos.x < 100.0 {
        discard;
    }
    return vec4(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "discard_fragment()")
}

// =============================================================================
// Test: StorageBarrier / WorkgroupBarrier
// =============================================================================

func TestIntegration6_Barriers(t *testing.T) {
	src := `
var<workgroup> shared_val: u32;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    if lid == 0u {
        shared_val = 42u;
    }
    workgroupBarrier();
    storageBarrier();
    out.v = shared_val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "threadgroup_barrier")
}

// =============================================================================
// Test: Array constructor (covers writeCompose array path)
// =============================================================================

func TestIntegration6_ArrayConstructor(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let arr = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    out.v = arr[0];
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "1.0")
	mustContainMSL(t, code, "4.0")
}

// =============================================================================
// Test: Multiple storage buffers with different access modes
// (covers isStorageGlobalReadOnly)
// =============================================================================

func TestIntegration6_ReadOnlyVsReadWrite(t *testing.T) {
	src := `
struct Data { v: f32 };
@group(0) @binding(0) var<storage, read> input_buf: Data;
@group(0) @binding(1) var<storage, read_write> output_buf: Data;
@compute @workgroup_size(1)
fn main() {
    output_buf.v = input_buf.v * 2.0;
}
`
	code := compileWGSL(t, src)
	// Read-only buffer should use 'const&' qualifier
	mustContainMSL(t, code, "const&")
	// Read-write buffer should use 'device' with '&' (no const)
	mustContainMSL(t, code, "device Data&")
}

// =============================================================================
// Test: Struct with array member (covers type emission paths)
// =============================================================================

func TestIntegration6_StructWithArray(t *testing.T) {
	src := `
struct Container {
    count: u32,
    data: array<f32, 16>,
};
@group(0) @binding(0) var<storage, read> container: Container;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = container.data[container.count];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Buffer = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "Container")
	mustContainMSL(t, code, "data")
}

// =============================================================================
// Test: Uniform buffers (covers SpaceUniform address space)
// =============================================================================

func TestIntegration6_UniformBuffer(t *testing.T) {
	src := `
struct Uniforms {
    mvp: mat4x4<f32>,
    color: vec4<f32>,
};
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@vertex fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return uniforms.mvp * uniforms.color;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "constant")
	mustContainMSL(t, code, "Uniforms")
}

// =============================================================================
// Test: Mixed scalar/vector math builtins (covers writeMath edge cases)
// =============================================================================

func TestIntegration6_MathCross(t *testing.T) {
	src := `
struct In { a: vec3<f32>, b: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = cross(inp.a, inp.b);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "cross")
}

func TestIntegration6_MathDot(t *testing.T) {
	src := `
struct In { a: vec3<f32>, b: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = dot(inp.a, inp.b);
}
`
	code := compileWGSL(t, src)
	// Float dot uses metal::dot directly (not wrapped)
	mustContainMSL(t, code, "dot")
}

func TestIntegration6_MathLength(t *testing.T) {
	src := `
struct In { v: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = length(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "length")
}

func TestIntegration6_MathNormalize(t *testing.T) {
	src := `
struct In { v: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = normalize(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "normalize")
}

func TestIntegration6_MathDistance(t *testing.T) {
	src := `
struct In { a: vec3<f32>, b: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = distance(inp.a, inp.b);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "distance")
}

func TestIntegration6_MathReflect(t *testing.T) {
	src := `
struct In { v: vec3<f32>, n: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = reflect(inp.v, inp.n);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "reflect")
}

// =============================================================================
// Test: Texture sample level (covers writeImageSample SampleLevelExact path)
// =============================================================================

func TestIntegration6_TextureSampleLevel(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureSampleLevel(tex, samp, vec2(0.5, 0.5), 2.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "level(")
}

func TestIntegration6_TextureSampleBias(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(tex, samp, uv, 0.5);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "bias(")
}

// =============================================================================
// Test: Vertex pulling with various formats
// (covers writeUnpackingFunction, vptVertexInputDimension)
// =============================================================================

func TestIntegration6_VertexPullingSint16(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(@location(0) pos: vec2<i32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(f32(pos.x), f32(pos.y), 0.0, 1.0);
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
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatSint16x2},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "[[vertex_id]]")
}

func TestIntegration6_VertexPullingUnorm8(t *testing.T) {
	src := `
struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
};
@vertex fn vs_main(@location(0) color: vec4<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4(color.rgb, 1.0);
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
				{ShaderLocation: 0, Offset: 0, Format: VertexFormatUnorm8x4},
			},
		},
	}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "[[vertex_id]]")
	mustContainMSL(t, code, "unpack")
}

// =============================================================================
// Test: Pipeline constants with workgroup_size x, y, z
// (covers pipeline constants adjustWorkgroupSize)
// =============================================================================

func TestIntegration6_PipelineConstantsWorkgroupXYZ(t *testing.T) {
	src := `
override X: u32 = 1u;
override Y: u32 = 1u;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(X, Y, 1)
fn main(@builtin(local_invocation_index) lid: u32) {
    out.v = lid;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"X": 8, "Y": 8}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "kernel")
}

// =============================================================================
// Test: Constant with vector type (covers writeConstant for vectors)
// =============================================================================

func TestIntegration6_ConstantVector(t *testing.T) {
	src := `
const UP: vec3<f32> = vec3(0.0, 1.0, 0.0);
struct Out { v: vec3<f32> };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = UP;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "UP")
}

// =============================================================================
// Test: Multiple texture types in one shader (covers various getImageType paths)
// =============================================================================

func TestIntegration6_MultipleTextureTypes(t *testing.T) {
	src := `
@group(0) @binding(0) var tex2d: texture_2d<f32>;
@group(0) @binding(1) var tex_depth: texture_depth_2d;
@group(0) @binding(2) var samp: sampler;
@group(0) @binding(3) var samp_cmp: sampler_comparison;

@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let color = textureSample(tex2d, samp, uv);
    let depth = textureSampleCompare(tex_depth, samp_cmp, uv, 0.5);
    return color * depth;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d<float")
	mustContainMSL(t, code, "depth2d<float")
	mustContainMSL(t, code, "sample_compare")
}

// =============================================================================
// Test: Storage texture formats (covers type emission for storage textures)
// =============================================================================

func TestIntegration6_StorageTextureR32Float(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_storage_2d<r32float, write>;
@compute @workgroup_size(1)
fn main() {
    textureStore(tex, vec2(0i, 0i), vec4(1.0, 0.0, 0.0, 0.0));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".write(")
}

func TestIntegration6_StorageTextureRGBA32Uint(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_storage_2d<rgba32uint, write>;
@compute @workgroup_size(1)
fn main() {
    textureStore(tex, vec2(0i, 0i), vec4(1u, 2u, 3u, 4u));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".write(")
}

// =============================================================================
// Test: InsertBits / ExtractBits (covers writeMath for bit manipulation)
// =============================================================================

func TestIntegration6_InsertBits(t *testing.T) {
	src := `
struct In { base: u32, insert: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = insertBits(inp.base, inp.insert, 4u, 8u);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "insert_bits")
}

func TestIntegration6_ExtractBits(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = extractBits(inp.v, 4u, 8u);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "extract_bits")
}

// =============================================================================
// Test: CountOneBits / ReverseBits / FirstLeadingBit / FirstTrailingBit
// =============================================================================

func TestIntegration6_CountOneBits(t *testing.T) {
	src := computeWrapIO("out.v = f32(countOneBits(u32(inp.a)));")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "popcount")
}

func TestIntegration6_ReverseBits(t *testing.T) {
	src := computeWrapIO("out.v = f32(reverseBits(u32(inp.a)));")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "reverse_bits")
}

func TestIntegration6_FirstLeadingBit(t *testing.T) {
	src := computeWrapIO("out.v = f32(firstLeadingBit(u32(inp.a)));")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "clz")
}

func TestIntegration6_FirstTrailingBit(t *testing.T) {
	src := computeWrapIO("out.v = f32(firstTrailingBit(u32(inp.a)));")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "ctz")
}

// =============================================================================
// Test: Saturate (covers writeMath Saturate path)
// =============================================================================

func TestIntegration6_Saturate(t *testing.T) {
	src := computeWrapIO("out.v = saturate(inp.a);")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "saturate")
}

// =============================================================================
// Test: Pack2x16snorm / unorm (covers additional pack paths)
// =============================================================================

func TestIntegration6_Pack2x16snorm(t *testing.T) {
	src := `
struct In { v: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = pack2x16snorm(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "pack_float_to_snorm2x16")
}

func TestIntegration6_Unpack2x16snorm(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = unpack2x16snorm(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "unpack_snorm2x16_to_float")
}
