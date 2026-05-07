package codegen

import (
	"math"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: Atomic load/store operations (covers writeAtomicLoad, writeAtomicStore)
// =============================================================================

func TestIntegration5_AtomicLoad(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let val = atomicLoad(&data.counter);
    out.v = val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_load_explicit")
	mustContainMSL(t, code, "memory_order_relaxed")
}

func TestIntegration5_AtomicStore(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main() {
    atomicStore(&data.counter, 42u);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_store_explicit")
	mustContainMSL(t, code, "memory_order_relaxed")
	mustContainMSL(t, code, "42u")
}

func TestIntegration_AtomicLoadI32(t *testing.T) {
	src := `
struct Data { counter: atomic<i32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let val = atomicLoad(&data.counter);
    out.v = val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_load_explicit")
	// The result type should be int (i32), not uint
	mustContainMSL(t, code, "int")
}

// =============================================================================
// Test: Signed integer negation (covers writeUnary negate, registerNegHelper)
// =============================================================================

func TestIntegration_SignedIntegerNegate(t *testing.T) {
	src := `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = -inp.v;
}
`
	code := compileWGSL(t, src)
	// Signed integer negate uses naga_neg() polyfill to avoid UB on INT_MIN
	mustContainMSL(t, code, "naga_neg")
	mustContainMSL(t, code, "as_type")
}

func TestIntegration_SignedIntegerNegateVec(t *testing.T) {
	src := `
struct In { v: vec3<i32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<i32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = -inp.v;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_neg")
}

// =============================================================================
// Test: Integer division and modulus helpers (covers writeHelperFunctions,
// writeHelperSubset, addDivOverload, addModOverload, sintMinLiteral)
// =============================================================================

func TestIntegration_IntegerDivisionI32(t *testing.T) {
	src := `
struct In { a: i32, b: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.a / inp.b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_div")
	mustContainMSL(t, code, "metal::select")
}

func TestIntegration_IntegerModulusU32(t *testing.T) {
	src := `
struct In { a: u32, b: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.a % inp.b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_mod")
	mustContainMSL(t, code, "metal::select")
}

func TestIntegration_IntegerModulusI32(t *testing.T) {
	src := `
struct In { a: i32, b: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.a % inp.b;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_mod")
	// Signed mod includes INT_MIN guard
	mustContainMSL(t, code, "divisor")
}

// =============================================================================
// Test: Integer abs helper (covers registerAbsHelper, writeHelperSubsetAbs)
// =============================================================================

func TestIntegration_IntegerAbs(t *testing.T) {
	src := `
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = abs(inp.v);
}
`
	code := compileWGSL(t, src)
	// Signed integer abs uses naga_abs polyfill
	mustContainMSL(t, code, "naga_abs")
}

// =============================================================================
// Test: Image operations — texture sampling (covers writeImageSample paths)
// =============================================================================

func TestIntegration5_TextureSampleGrad(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
struct Out { v: vec4<f32> };
@group(0) @binding(2) var<storage, read_write> out: Out;
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(tex, samp, pos.xy, vec2(1.0, 0.0), vec2(0.0, 1.0));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "gradient2d")
}

func TestIntegration_TextureSampleCompare(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_depth_2d;
@group(0) @binding(1) var samp: sampler_comparison;
@fragment fn fs(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    let d = textureSampleCompare(tex, samp, pos.xy, 0.5);
    return vec4(d, d, d, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample_compare(")
}

// =============================================================================
// Test: Image load — unchecked policy (covers writeImageLoadUnchecked, imageNeedsLod)
// =============================================================================

func TestIntegration_ImageLoadUnchecked(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckUnchecked
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
}

func TestIntegration_ImageLoad1DNoLod(t *testing.T) {
	// 1D textures should NOT emit LOD argument
	src := `
@group(0) @binding(0) var tex: texture_1d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, 0i, 0i);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckUnchecked
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
}

// =============================================================================
// Test: Image load — Restrict policy (covers writeImageLoadRestrict,
// writeRestrictCoords, writeRestrictCoordsWithLod)
// =============================================================================

func TestIntegration_ImageLoadRestrict2D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
	mustContainMSL(t, code, "metal::min(")
	mustContainMSL(t, code, "get_width(")
	mustContainMSL(t, code, "get_height(")
}

func TestIntegration_ImageLoadRestrict1D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_1d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, 0i, 0i);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
	mustContainMSL(t, code, "get_width()")
}

// =============================================================================
// Test: Image load — ReadZeroSkipWrite policy
// (covers writeImageLoadRZSW, writeRZSWCoordCheck)
// =============================================================================

func TestIntegration_ImageLoadRZSW(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckReadZeroSkipWrite
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
	// RZSW wraps in ternary with zero fallback
	mustContainMSL(t, code, "?")
}

// =============================================================================
// Test: Image store (covers writeImageStore)
// =============================================================================

func TestIntegration_ImageStore2D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main() {
    textureStore(tex, vec2(0i, 0i), vec4(1.0, 0.0, 0.0, 1.0));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".write(")
}

// =============================================================================
// Test: Texture dimensions (covers writeImageQuery, writeImageQuerySize)
// =============================================================================

func TestIntegration5_TextureDimensions(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: vec2<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureDimensions(tex);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "get_width()")
	mustContainMSL(t, code, "get_height()")
}

func TestIntegration5_TextureNumLevels(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureNumLevels(tex);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "get_num_mip_levels()")
}

// =============================================================================
// Test: Pipeline constants — unary negation and complex expressions
// (covers evalUnaryOp, evalGlobalExpression, convertLiteralToType, literalToScalarValue)
// =============================================================================

func TestIntegration_PipelineConstantsNegation(t *testing.T) {
	src := `
override A: f32 = 5.0;
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v * (-A);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 3.0}
	code := compileWGSLWithOpts(t, src, opts)
	// -A with A=3.0 should produce -3.0
	mustContainMSL(t, code, "-3.0")
}

func TestIntegration_PipelineConstantsBinaryExpression(t *testing.T) {
	src := `
override W: u32 = 8u;
override H: u32 = 8u;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = W * H;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"W": 16, "H": 16}
	code := compileWGSLWithOpts(t, src, opts)
	// W*H with W=16, H=16 should produce 256u
	mustContainMSL(t, code, "256u")
}

// =============================================================================
// Test: Pipeline constants — type conversion at pipeline constant boundary
// (covers convertLiteralToType, scalarValueToLiteral edge cases)
// =============================================================================

func TestIntegration_PipelineConstantsF32ToI32(t *testing.T) {
	// Override of type i32 supplied as f64 pipeline constant
	src := `
override OFFSET: i32 = 0;
struct In { v: i32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v + OFFSET;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"OFFSET": 42.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "42")
}

// =============================================================================
// Test: Restrict bounds checking on buffer access
// (covers accessIndexNeedsRestrict, writeRestrictedIndex, literalAsUint)
// =============================================================================

func TestIntegration_RestrictBoundsArray(t *testing.T) {
	src := `
struct Data { arr: array<f32, 4> };
@group(0) @binding(0) var<storage, read> data: Data;
struct In { idx: u32 };
@group(0) @binding(1) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.arr[inp.idx];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Buffer = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "metal::min(")
}

func TestIntegration_RestrictBoundsStaticInBounds(t *testing.T) {
	// Static index within bounds should NOT emit metal::min
	src := `
struct Data { arr: array<f32, 4> };
@group(0) @binding(0) var<storage, read> data: Data;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.arr[2];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Buffer = BoundsCheckRestrict
	code := compileWGSLWithOpts(t, src, opts)
	// Static access at index 2 in array of size 4 — no clamping needed
	mustNotContainMSL(t, code, "metal::min(unsigned(")
}

// =============================================================================
// Test: ReadZeroSkipWrite bounds checking on buffer access
// (covers buildRZSWBoundsCheck, writeRZSWFallback, accessChainRootIsPointer)
// =============================================================================

func TestIntegration_RZSWBoundsArray(t *testing.T) {
	src := `
struct Data { arr: array<f32, 4> };
@group(0) @binding(0) var<storage, read> data: Data;
struct In { idx: u32 };
@group(0) @binding(1) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(2) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.arr[inp.idx];
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Buffer = BoundsCheckReadZeroSkipWrite
	code := compileWGSLWithOpts(t, src, opts)
	// RZSW should emit ternary with condition and DefaultConstructible fallback
	mustContainMSL(t, code, "DefaultConstructible()")
}

// =============================================================================
// Test: Unary operations (covers writeUnary — logical not, bitwise not)
// =============================================================================

func TestIntegration_UnaryLogicalNot(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    if !( inp.v > 0u ) { out.v = 1u; }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "!")
}

func TestIntegration_UnaryBitwiseNot(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = ~inp.v;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "~")
}

// =============================================================================
// Test: Select expression (covers writeSelect — bool scalar & vector)
// =============================================================================

func TestIntegration5_SelectScalar(t *testing.T) {
	src := `
struct In { a: f32, b: f32, c: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = select(inp.a, inp.b, inp.c > 0.5);
}
`
	code := compileWGSL(t, src)
	// Scalar select uses ternary operator in MSL
	mustContainMSL(t, code, "?")
}

// =============================================================================
// Test: Splat expression (covers writeSplat)
// =============================================================================

func TestIntegration_Splat(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = vec4(inp.v);
}
`
	code := compileWGSL(t, src)
	// Splat creates a single-arg constructor
	mustContainMSL(t, code, "float4(")
}

// =============================================================================
// Test: Swizzle expression (covers writeSwizzle)
// =============================================================================

func TestIntegration_Swizzle(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v.yx;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".yx")
}

// =============================================================================
// Test: Modf and frexp math (covers writeModf, writeFrexp)
// =============================================================================

func TestIntegration5_Modf(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let r = modf(inp.v);
    out.v = r.fract;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_modf")
}

func TestIntegration5_Frexp(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let r = frexp(inp.v);
    out.v = r.fract;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_frexp")
}

// =============================================================================
// Test: QuantizeToF16 (covers writeQuantizeF16)
// =============================================================================

func TestIntegration5_QuantizeToF16(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = quantizeToF16(inp.v);
}
`
	code := compileWGSL(t, src)
	// QuantizeToF16 uses half cast
	mustContainMSL(t, code, "half")
}

// =============================================================================
// Test: Pack/Unpack operations (covers writePack4x8, writeUnpack4x8)
// =============================================================================

func TestIntegration_Pack4x8snorm(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = pack4x8snorm(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "pack_float_to_snorm4x8")
}

func TestIntegration_Pack4x8unorm(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = pack4x8unorm(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "pack_float_to_unorm4x8")
}

func TestIntegration_Unpack4x8snorm(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = unpack4x8snorm(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "unpack_snorm4x8_to_float")
}

func TestIntegration_Unpack4x8unorm(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = unpack4x8unorm(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "unpack_unorm4x8_to_float")
}

func TestIntegration_Pack2x16float(t *testing.T) {
	src := `
struct In { v: vec2<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = pack2x16float(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "as_type<uint>(half2(")
}

func TestIntegration_Unpack2x16float(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec2<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = unpack2x16float(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "float2(as_type<half2>(")
}

// =============================================================================
// Test: Derivative expressions (covers writeDerivative)
// =============================================================================

func TestIntegration_DerivativeDpdx(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let c = textureSample(tex, samp, uv);
    let d = dpdx(c.x);
    return vec4(d, d, d, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "dfdx")
}

func TestIntegration_DerivativeDpdy(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let c = textureSample(tex, samp, uv);
    let d = dpdy(c.x);
    return vec4(d, d, d, 1.0);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "dfdy")
}

// =============================================================================
// Test: Relational expressions (covers writeRelational)
// =============================================================================

func TestIntegration_RelationalAny(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    if any(inp.v > vec4(0.5)) { out.v = 1u; }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "any")
}

func TestIntegration_RelationalAll(t *testing.T) {
	src := `
struct In { v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    if all(inp.v > vec4(0.5)) { out.v = 1u; }
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "all")
}

// =============================================================================
// Test: Binary wrapped operations (covers writeWrappedBinaryOperand)
// =============================================================================

func TestIntegration_BinaryShiftLeft(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v << 2u;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "<<")
}

func TestIntegration_BinaryShiftRight(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.v >> 2u;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ">>")
}

// =============================================================================
// Test: Type cast expressions (covers writeAs — bitcast, static_cast, matrix)
// =============================================================================

func TestIntegration_BitcastU32ToF32(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = bitcast<f32>(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "as_type<float>(")
}

func TestIntegration_CastF32ToI32(t *testing.T) {
	src := `
struct In { v: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = i32(inp.v);
}
`
	code := compileWGSL(t, src)
	// Float-to-int uses naga_f2i helper with clamping
	mustContainMSL(t, code, "naga_f2i")
}

func TestIntegration_CastVec3F32ToI32(t *testing.T) {
	src := `
struct In { v: vec3<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<i32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = vec3<i32>(inp.v);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_f2i")
}

// =============================================================================
// Test: Constant value inline emission (covers writeConstantValueInline)
// =============================================================================

func TestIntegration_ConstantValueInline(t *testing.T) {
	// Override with composite value that gets inlined
	src := `
override A: f32 = 1.0;
override B: f32 = 2.0;
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = A + B;
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 10.0, "B": 20.0}
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, "10.0")
	mustContainMSL(t, code, "20.0")
}

// =============================================================================
// Test: halfToFloat32 (covers types.go halfToFloat32)
// =============================================================================

func TestHalfToFloat32(t *testing.T) {
	tests := []struct {
		name string
		bits uint16
		want float32
	}{
		{"positive_zero", 0x0000, 0.0},
		{"negative_zero", 0x8000, float32(math.Copysign(0, -1))},
		{"one", 0x3C00, 1.0},
		{"negative_one", 0xBC00, -1.0},
		{"half", 0x3800, 0.5},
		{"two", 0x4000, 2.0},
		{"infinity", 0x7C00, float32(math.Inf(1))},
		{"negative_infinity", 0xFC00, float32(math.Inf(-1))},
		{"smallest_subnormal", 0x0001, 5.960464477539063e-8},
		{"largest_subnormal", 0x03FF, 6.097555160522461e-5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := halfToFloat32(tt.bits)
			if math.IsNaN(float64(tt.want)) {
				if !math.IsNaN(float64(got)) {
					t.Errorf("halfToFloat32(0x%04X) = %v, want NaN", tt.bits, got)
				}
			} else if got != tt.want {
				t.Errorf("halfToFloat32(0x%04X) = %v, want %v", tt.bits, got, tt.want)
			}
		})
	}
}

func TestHalfToFloat32_NaN(t *testing.T) {
	// NaN has exponent all 1s and non-zero fraction
	got := halfToFloat32(0x7E00) // quiet NaN
	if !math.IsNaN(float64(got)) {
		t.Errorf("halfToFloat32(0x7E00) = %v, want NaN", got)
	}
}

// =============================================================================
// Test: Pipeline constants — evalUnaryOp (covers pipeline_constants.go)
// =============================================================================

func TestEvalUnaryOp(t *testing.T) {
	tests := []struct {
		name     string
		op       ir.UnaryOperator
		operand  ir.LiteralValue
		wantNil  bool
		wantBool *bool
		wantI32  *int32
		wantF32  *float32
	}{
		{
			name:     "logical_not_true",
			op:       ir.UnaryLogicalNot,
			operand:  ir.LiteralBool(true),
			wantBool: boolPtr(false),
		},
		{
			name:     "logical_not_false",
			op:       ir.UnaryLogicalNot,
			operand:  ir.LiteralBool(false),
			wantBool: boolPtr(true),
		},
		{
			name:    "negate_f32",
			op:      ir.UnaryNegate,
			operand: ir.LiteralF32(3.14),
			wantF32: f32Ptr(-3.14),
		},
		{
			name:    "negate_i32",
			op:      ir.UnaryNegate,
			operand: ir.LiteralI32(42),
			wantI32: i32Ptr(-42),
		},
		{
			name:    "negate_f64",
			op:      ir.UnaryNegate,
			operand: ir.LiteralF64(2.71),
			wantNil: false,
		},
		{
			name:    "logical_not_on_i32_returns_nil",
			op:      ir.UnaryLogicalNot,
			operand: ir.LiteralI32(1),
			wantNil: true,
		},
		{
			name:    "negate_u32_returns_nil",
			op:      ir.UnaryNegate,
			operand: ir.LiteralU32(5),
			wantNil: true,
		},
		{
			name:    "unsupported_op_returns_nil",
			op:      ir.UnaryBitwiseNot,
			operand: ir.LiteralI32(1),
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalUnaryOp(tt.op, tt.operand)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				if !tt.wantNil {
					t.Errorf("expected non-nil result")
				}
				return
			}
			if tt.wantBool != nil {
				b, ok := got.(ir.LiteralBool)
				if !ok {
					t.Errorf("expected LiteralBool, got %T", got)
				} else if bool(b) != *tt.wantBool {
					t.Errorf("got %v, want %v", b, *tt.wantBool)
				}
			}
			if tt.wantI32 != nil {
				v, ok := got.(ir.LiteralI32)
				if !ok {
					t.Errorf("expected LiteralI32, got %T", got)
				} else if int32(v) != *tt.wantI32 {
					t.Errorf("got %v, want %v", v, *tt.wantI32)
				}
			}
			if tt.wantF32 != nil {
				v, ok := got.(ir.LiteralF32)
				if !ok {
					t.Errorf("expected LiteralF32, got %T", got)
				} else if float32(v) != *tt.wantF32 {
					t.Errorf("got %v, want %v", v, *tt.wantF32)
				}
			}
		})
	}
}

// =============================================================================
// Test: evalGlobalExpression (covers pipeline_constants.go)
// =============================================================================

func TestEvalGlobalExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	initExpr := ir.ExpressionHandle(0)

	// Module with: globalExpr[0]=Literal(1.0), globalExpr[1]=Override(0),
	// globalExpr[2]=Binary(Add, [1], [0])
	m := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "C", Type: tF32, Init: initExpr, Value: ir.ScalarValue{Bits: 0x3F800000, Kind: ir.ScalarFloat}},
		},
		Overrides: []ir.Override{
			{Name: "A", Ty: tF32},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},              // [0]
			{Kind: ir.ExprOverride{Override: 0}},                       // [1]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 1, Right: 0}}, // [2]
			{Kind: ir.ExprConstant{Constant: 0}},                       // [3] references constant 0
		},
	}

	overrideValues := map[ir.OverrideHandle]ir.LiteralValue{
		0: ir.LiteralF32(5.0),
	}

	t.Run("literal", func(t *testing.T) {
		got := evalGlobalExpression(m, 0, overrideValues)
		if v, ok := got.(ir.LiteralF32); !ok || float32(v) != 1.0 {
			t.Errorf("expected LiteralF32(1.0), got %v", got)
		}
	})

	t.Run("override", func(t *testing.T) {
		got := evalGlobalExpression(m, 1, overrideValues)
		if v, ok := got.(ir.LiteralF32); !ok || float32(v) != 5.0 {
			t.Errorf("expected LiteralF32(5.0), got %v", got)
		}
	})

	t.Run("binary_add", func(t *testing.T) {
		got := evalGlobalExpression(m, 2, overrideValues)
		if v, ok := got.(ir.LiteralF32); !ok || float32(v) != 6.0 {
			t.Errorf("expected LiteralF32(6.0), got %v (override=5.0 + literal=1.0)", got)
		}
	})

	t.Run("constant_ref", func(t *testing.T) {
		got := evalGlobalExpression(m, 3, overrideValues)
		if v, ok := got.(ir.LiteralF32); !ok || float32(v) != 1.0 {
			t.Errorf("expected LiteralF32(1.0) via constant, got %v", got)
		}
	})

	t.Run("out_of_bounds", func(t *testing.T) {
		got := evalGlobalExpression(m, 99, overrideValues)
		if got != nil {
			t.Errorf("expected nil for out-of-bounds, got %v", got)
		}
	})
}

// =============================================================================
// Test: convertLiteralToType (covers pipeline_constants.go)
// =============================================================================

func TestConvertLiteralToType(t *testing.T) {
	tests := []struct {
		name   string
		lit    ir.LiteralValue
		scalar ir.ScalarType
		check  func(ir.LiteralValue) bool
	}{
		{
			name:   "f32_to_i32",
			lit:    ir.LiteralF32(42.7),
			scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
			check:  func(v ir.LiteralValue) bool { i, ok := v.(ir.LiteralI32); return ok && int32(i) == 42 },
		},
		{
			name:   "f32_to_u32",
			lit:    ir.LiteralF32(100.0),
			scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			check:  func(v ir.LiteralValue) bool { u, ok := v.(ir.LiteralU32); return ok && uint32(u) == 100 },
		},
		{
			name:   "i32_to_f32",
			lit:    ir.LiteralI32(7),
			scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			check:  func(v ir.LiteralValue) bool { f, ok := v.(ir.LiteralF32); return ok && float32(f) == 7.0 },
		},
		{
			name:   "f32_to_bool_nonzero",
			lit:    ir.LiteralF32(1.0),
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			check:  func(v ir.LiteralValue) bool { b, ok := v.(ir.LiteralBool); return ok && bool(b) },
		},
		{
			name:   "f32_to_bool_zero",
			lit:    ir.LiteralF32(0.0),
			scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
			check:  func(v ir.LiteralValue) bool { b, ok := v.(ir.LiteralBool); return ok && !bool(b) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertLiteralToType(tt.lit, tt.scalar)
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if !tt.check(got) {
				t.Errorf("unexpected result: %v (%T)", got, got)
			}
		})
	}
}

// =============================================================================
// Test: literalToScalarValue (covers pipeline_constants.go)
// =============================================================================

func TestLiteralToScalarValue(t *testing.T) {
	tests := []struct {
		name string
		lit  ir.LiteralValue
		kind ir.ScalarKind
		bits uint64
	}{
		{"f32", ir.LiteralF32(1.0), ir.ScalarFloat, uint64(math.Float32bits(1.0))},
		{"f64", ir.LiteralF64(2.0), ir.ScalarFloat, math.Float64bits(2.0)},
		{"i32", ir.LiteralI32(-1), ir.ScalarSint, 0xFFFFFFFFFFFFFFFF},
		{"u32", ir.LiteralU32(42), ir.ScalarUint, 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := literalToScalarValue(tt.lit)
			if got == nil {
				t.Fatal("expected non-nil ScalarValue")
			}
			if got.Kind != tt.kind {
				t.Errorf("kind = %v, want %v", got.Kind, tt.kind)
			}
			if got.Bits != tt.bits {
				t.Errorf("bits = %v, want %v", got.Bits, tt.bits)
			}
		})
	}
}

func TestLiteralToScalarValue_Bool(t *testing.T) {
	got := literalToScalarValue(ir.LiteralBool(true))
	if got == nil {
		t.Fatal("expected non-nil for bool true")
	}
	if got.Bits != 1 || got.Kind != ir.ScalarBool {
		t.Errorf("got bits=%d kind=%v, want bits=1 kind=ScalarBool", got.Bits, got.Kind)
	}
	gotF := literalToScalarValue(ir.LiteralBool(false))
	if gotF == nil {
		t.Fatal("expected non-nil for bool false")
	}
	if gotF.Bits != 0 || gotF.Kind != ir.ScalarBool {
		t.Errorf("got bits=%d kind=%v, want bits=0 kind=ScalarBool", gotF.Bits, gotF.Kind)
	}
}

// =============================================================================
// Test: Atomic add/sub/and/or/xor/min/max (covers writeAtomic full switch)
// =============================================================================

func TestIntegration5_AtomicAddResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicAdd(&data.counter, 1u);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_add_explicit")
}

func TestIntegration5_AtomicSubResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicSub(&data.counter, 1u);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_sub_explicit")
}

func TestIntegration5_AtomicMinResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicMin(&data.counter, 5u);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_min_explicit")
}

func TestIntegration5_AtomicMaxResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicMax(&data.counter, 10u);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_max_explicit")
}

func TestIntegration5_AtomicAndResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicAnd(&data.counter, 0xFFu);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_and_explicit")
}

func TestIntegration5_AtomicOrResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicOr(&data.counter, 0x01u);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_or_explicit")
}

func TestIntegration5_AtomicXorResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicXor(&data.counter, 0xABu);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_fetch_xor_explicit")
}

func TestIntegration5_AtomicExchangeResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let old = atomicExchange(&data.counter, 99u);
    out.v = old;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "atomic_exchange_explicit")
}

func TestIntegration5_AtomicCompareExchangeResult(t *testing.T) {
	src := `
struct Data { counter: atomic<u32> };
@group(0) @binding(0) var<storage, read_write> data: Data;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    let result = atomicCompareExchangeWeak(&data.counter, 0u, 1u);
    out.v = result.old_value;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_atomic_compare_exchange_weak_explicit")
	mustContainMSL(t, code, "_atomic_compare_exchange_result")
}

// =============================================================================
// Test: Function call pass-through (covers writeFunctions, writeFunction)
// =============================================================================

func TestIntegration_FunctionCallPassThrough(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

fn sample_texture(uv: vec2<f32>) -> vec4<f32> {
    return textureSample(tex, samp, uv);
}

@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return sample_texture(uv);
}
`
	code := compileWGSL(t, src)
	// Non-entry-point function should receive pass-through texture/sampler params
	mustContainMSL(t, code, "sample_texture")
}

// =============================================================================
// Test: Integer dot product wrapping (covers registerDotWrapper, writeHelperSubsetDot)
// =============================================================================

func TestIntegration_IntegerDotProduct(t *testing.T) {
	src := `
struct In { a: vec3<i32>, b: vec3<i32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = dot(inp.a, inp.b);
}
`
	code := compileWGSL(t, src)
	// Integer dot uses manual wrapper since MSL metal::dot doesn't support int/uint
	mustContainMSL(t, code, "naga_dot")
}

// =============================================================================
// Test: WorkGroupUniformLoad (covers writeWorkGroupUniformLoad)
// =============================================================================

func TestIntegration_WorkGroupUniformLoad(t *testing.T) {
	src := `
var<workgroup> shared_val: u32;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    if lid == 0u {
        shared_val = 42u;
    }
    let val = workgroupUniformLoad(&shared_val);
    out.v = val;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "threadgroup_barrier")
}

// =============================================================================
// Test: Literal emission edge cases (covers writeLiteral)
// =============================================================================

func TestIntegration5_LiteralI64(t *testing.T) {
	tI64 := ir.TypeHandle(0)
	tVec4F32 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(0)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Result: &ir.FunctionResult{
					Type:    tVec4F32,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI64(9999999999)}},
					{Kind: ir.ExprZeroValue{Type: tVec4F32}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI64},
					{Handle: &tVec4F32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "9999999999")
}

// =============================================================================
// Test: ExpressionKind coverage — ArrayLength on runtime-sized arrays
// (covers writeArrayLength, resolveArrayLengthGlobal via direct IR)
// =============================================================================

func TestMSL_ArrayLengthDirect(t *testing.T) {
	tU32 := ir.TypeHandle(0)
	tF32 := ir.TypeHandle(1)
	tRuntimeArr := ir.TypeHandle(2)
	tStruct := ir.TypeHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: tF32, Stride: 4, Size: ir.ArraySize{}}}, // runtime-sized
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "data", Type: tRuntimeArr, Offset: 0},
				},
				Span: 4,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "buf",
				Type:    tStruct,
				Space:   ir.SpaceStorage,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		Functions: []ir.Function{
			{
				Name: "main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [0] global ref
					{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] access .data
					{Kind: ir.ExprArrayLength{Array: 1}},          // [2] arrayLength
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tStruct},
					{Handle: &tRuntimeArr},
					{Handle: &tU32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}
	result := compileModule(t, module)
	// arrayLength generates: 1 + (_buffer_sizes.size0 - offset - elemSize) / stride
	mustContainMSL(t, result, "_buffer_sizes")
}

// =============================================================================
// Test: Pointer type names (covers pointerTypeName, inlineArrayTypeName)
// =============================================================================

func TestIntegration_PointerTypeInFunction(t *testing.T) {
	src := `
fn increment(p: ptr<function, i32>) {
    *p = *p + 1;
}
struct Out { v: i32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var x: i32 = 10;
    increment(&x);
    out.v = x;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "increment")
	// pointer param should use thread address space
	mustContainMSL(t, code, "thread")
}

// =============================================================================
// Test: Math function coverage (covers writeMath various cases)
// =============================================================================

func TestIntegration_MathFunctions(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{"sqrt", "out.v = sqrt(inp.a);", "sqrt"},
		{"ceil", "out.v = ceil(inp.a);", "ceil"},
		{"floor", "out.v = floor(inp.a);", "floor"},
		{"round", "out.v = round(inp.a);", "round"},
		{"trunc", "out.v = trunc(inp.a);", "trunc"},
		{"sin", "out.v = sin(inp.a);", "sin"},
		{"cos", "out.v = cos(inp.a);", "cos"},
		{"tan", "out.v = tan(inp.a);", "tan"},
		{"asin", "out.v = asin(inp.a);", "asin"},
		{"acos", "out.v = acos(inp.a);", "acos"},
		{"atan", "out.v = atan(inp.a);", "atan"},
		{"atan2", "out.v = atan2(inp.a, inp.b);", "atan2"},
		{"exp", "out.v = exp(inp.a);", "exp"},
		{"exp2", "out.v = exp2(inp.a);", "exp2"},
		{"log", "out.v = log(inp.a);", "log"},
		{"log2", "out.v = log2(inp.a);", "log2"},
		{"pow", "out.v = pow(inp.a, inp.b);", "pow"},
		{"min", "out.v = min(inp.a, inp.b);", "min"},
		{"max", "out.v = max(inp.a, inp.b);", "max"},
		{"clamp", "out.v = clamp(inp.a, inp.b, inp.c);", "clamp"},
		{"fma", "out.v = fma(inp.a, inp.b, inp.c);", "fma"},
		{"sign", "out.v = sign(inp.a);", "sign"},
		{"step", "out.v = step(inp.a, inp.b);", "step"},
		{"fract", "out.v = fract(inp.a);", "fract"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := compileWGSL(t, computeWrapIO(tt.expr))
			mustContainMSL(t, code, tt.want)
		})
	}
}

// =============================================================================
// Test: Mixed vector math (covers writeMath on vector arguments)
// =============================================================================

func TestIntegration_MathMixVec(t *testing.T) {
	src := `
struct In { a: vec3<f32>, b: vec3<f32>, t: f32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec3<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = mix(inp.a, inp.b, inp.t);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "mix")
}

func TestIntegration_MathSmoothstep(t *testing.T) {
	src := computeWrapIO("out.v = smoothstep(0.0, 1.0, inp.a);")
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "smoothstep")
}

// =============================================================================
// Test: Matrix operations (covers writeExpressionKind for matrix paths)
// =============================================================================

func TestIntegration_MatrixMultiply(t *testing.T) {
	src := `
struct In { m: mat4x4<f32>, v: vec4<f32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = inp.m * inp.v;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "*")
}

// =============================================================================
// Test: isVariableReference, isPointerExpression edge cases
// =============================================================================

func TestIntegration_PointerExpression(t *testing.T) {
	src := `
struct Data { vals: array<f32, 8> };
@group(0) @binding(0) var<storage, read_write> data: Data;
@compute @workgroup_size(1)
fn main() {
    data.vals[0] = 1.0;
    data.vals[1] = 2.0;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "data")
}

// =============================================================================
// Test: Texture sampling with offset (covers writeImageSample offset path)
// =============================================================================

func TestIntegration_TextureSampleOffset(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(tex, samp, uv, 0.0, vec2(1i, 2i));
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".sample(")
	mustContainMSL(t, code, "level")
}

// =============================================================================
// Test: writeExpressionKind — constant expression via pipeline constants
// (covers tryFoldConstantCast)
// =============================================================================

func TestIntegration_TryFoldConstantCast(t *testing.T) {
	src := `
override A: i32 = 5;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = u32(A);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 42}
	code := compileWGSLWithOpts(t, src, opts)
	// Cast of constant i32(42) to u32 should fold to 42u
	mustContainMSL(t, code, "42u")
}

// =============================================================================
// Test: Struct padding verification (covers writeStructDefinition padding)
// =============================================================================

func TestIntegration_StructPadding(t *testing.T) {
	src := `
struct PaddedStruct {
    @size(8) a: f32,
    b: f32,
};
@group(0) @binding(0) var<storage, read> data: PaddedStruct;
struct Out { v: f32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = data.a + data.b;
}
`
	code := compileWGSL(t, src)
	// @size(8) on f32 (4 bytes) should create 4 bytes of padding
	mustContainMSL(t, code, "pad")
}

// =============================================================================
// Test: writeLocalVariable edge case — local variable with initializer
// =============================================================================

func TestIntegration_LocalVariableInit(t *testing.T) {
	src := `
struct Out { v: f32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var x: f32 = 3.14;
    x = x * 2.0;
    out.v = x;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "3.14")
}

// =============================================================================
// Test: Break if (covers emitBreakIfSubExpressions, isLiteralExpression)
// =============================================================================

func TestIntegration_BreakIf(t *testing.T) {
	src := `
struct In { v: u32 };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { v: u32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    var i: u32 = 0u;
    loop {
        i = i + 1u;
        continuing {
            break if i >= inp.v;
        }
    }
    out.v = i;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "while(true)")
	mustContainMSL(t, code, "break")
}

// =============================================================================
// Test: Texture gather (covers writeImageSample gather path)
// =============================================================================

func TestIntegration_TextureGather(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGather(1, tex, samp, uv);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".gather(")
	mustContainMSL(t, code, "component::y")
}

// =============================================================================
// Test: Texture depth gather compare
// =============================================================================

func TestIntegration_TextureGatherCompare(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_depth_2d;
@group(0) @binding(1) var samp: sampler_comparison;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGatherCompare(tex, samp, uv, 0.5);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, ".gather_compare(")
}

// =============================================================================
// Test: Image store with Restrict bounds (covers image store Restrict path)
// =============================================================================

func TestIntegration_ImageStoreRZSW(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_storage_2d<rgba8unorm, write>;
struct In { idx: vec2<i32> };
@group(0) @binding(1) var<storage, read> inp: In;
@compute @workgroup_size(1)
fn main() {
    textureStore(tex, inp.idx, vec4(1.0, 0.0, 0.0, 1.0));
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckReadZeroSkipWrite
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".write(")
}

// =============================================================================
// Test: Expression type handle coverage (covers getExpressionTypeHandle, coordUintType)
// =============================================================================

func TestIntegration_TextureDimensions3D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_3d<f32>;
struct Out { v: vec3<u32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureDimensions(tex);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "get_width()")
	mustContainMSL(t, code, "get_height()")
	mustContainMSL(t, code, "get_depth()")
}

// =============================================================================
// Test: Multisampled texture (covers multisampled paths)
// =============================================================================

func TestIntegration_MultisampledTextureLoad(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;
struct Out { v: vec4<f32> };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = textureLoad(tex, vec2(0i, 0i), 0i);
}
`
	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Image = BoundsCheckUnchecked
	code := compileWGSLWithOpts(t, src, opts)
	mustContainMSL(t, code, ".read(")
}

// =============================================================================
// Test: resolveImageType for typed texture (covers resolveImageType)
// =============================================================================

func TestIntegration_ImageTypeResolution(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv);
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "texture2d<float, metal::access::sample>")
}

// =============================================================================
// Test: Multiple helper functions in one shader
// (covers writeHelperFunctions with combined div+mod+dot+abs+neg)
// =============================================================================

func TestIntegration_MultipleHelpers(t *testing.T) {
	src := `
struct In { a: i32, b: i32, va: vec2<i32>, vb: vec2<i32> };
@group(0) @binding(0) var<storage, read> inp: In;
struct Out { div_val: i32, mod_val: i32, dot_val: i32, abs_val: i32, neg_val: i32 };
@group(0) @binding(1) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.div_val = inp.a / inp.b;
    out.mod_val = inp.a % inp.b;
    out.dot_val = dot(inp.va, inp.vb);
    out.abs_val = abs(inp.a);
    out.neg_val = -inp.a;
}
`
	code := compileWGSL(t, src)
	mustContainMSL(t, code, "naga_div")
	mustContainMSL(t, code, "naga_mod")
	mustContainMSL(t, code, "naga_dot")
	mustContainMSL(t, code, "naga_abs")
	mustContainMSL(t, code, "naga_neg")
}

// =============================================================================
// Test: findOrRegisterScalarType via direct IR (covers expressions.go)
// =============================================================================

func TestMSL_FindOrRegisterScalarType(t *testing.T) {
	// Exercise tryFoldConstantCast through WGSL, which calls findOrRegisterScalarType.
	src := `
override A: i32 = 5;
struct Out { v: u32 };
@group(0) @binding(0) var<storage, read_write> out: Out;
@compute @workgroup_size(1)
fn main() {
    out.v = u32(A);
}
`
	opts := DefaultOptions()
	opts.PipelineConstants = map[string]float64{"A": 42}
	code := compileWGSLWithOpts(t, src, opts)
	// A=42 as i32 cast to u32 should fold to 42u
	mustContainMSL(t, code, "42u")
}

// =============================================================================
// Test: reinterpretedVarName (covers expressions.go directly)
// =============================================================================

func TestReinterpretedVarName(t *testing.T) {
	w := &Writer{}
	name := w.reinterpretedVarName("packed_uchar4", ir.ExpressionHandle(5))
	expected := "reinterpreted_packed_uchar4_e5"
	if name != expected {
		t.Errorf("reinterpretedVarName = %q, want %q", name, expected)
	}
}

// =============================================================================
// Test: typeInnerLength (covers expressions.go)
// =============================================================================

func TestTypeInnerLength(t *testing.T) {
	w := &Writer{}
	tests := []struct {
		name   string
		inner  ir.TypeInner
		length uint32
		ok     bool
	}{
		{
			name:   "vector",
			inner:  ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			length: 4,
			ok:     true,
		},
		{
			name:   "matrix",
			inner:  ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			length: 3,
			ok:     true,
		},
		{
			name:   "array_fixed",
			inner:  ir.ArrayType{Size: ir.ArraySize{Constant: uint32Ptr(10)}, Stride: 4},
			length: 10,
			ok:     true,
		},
		{
			name:   "array_runtime",
			inner:  ir.ArrayType{Size: ir.ArraySize{}, Stride: 4},
			length: 0,
			ok:     false,
		},
		{
			name:   "scalar_not_indexable",
			inner:  ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			length: 0,
			ok:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLen, gotOk := w.typeInnerLength(tt.inner)
			if gotLen != tt.length || gotOk != tt.ok {
				t.Errorf("typeInnerLength = (%d, %v), want (%d, %v)", gotLen, gotOk, tt.length, tt.ok)
			}
		})
	}
}

// =============================================================================
// Test: literalAsUint (covers expressions.go)
// =============================================================================

func TestLiteralAsUint(t *testing.T) {
	w := &Writer{}
	tests := []struct {
		name string
		lit  ir.Literal
		val  uint32
		ok   bool
	}{
		{"u32_42", ir.Literal{Value: ir.LiteralU32(42)}, 42, true},
		{"i32_positive", ir.Literal{Value: ir.LiteralI32(7)}, 7, true},
		{"i32_zero", ir.Literal{Value: ir.LiteralI32(0)}, 0, true},
		{"i32_negative", ir.Literal{Value: ir.LiteralI32(-1)}, 0, false},
		{"f32", ir.Literal{Value: ir.LiteralF32(1.0)}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOk := w.literalAsUint(tt.lit)
			if gotVal != tt.val || gotOk != tt.ok {
				t.Errorf("literalAsUint = (%d, %v), want (%d, %v)", gotVal, gotOk, tt.val, tt.ok)
			}
		})
	}
}

// =============================================================================
// Helpers
// =============================================================================

func boolPtr(b bool) *bool       { return &b }
func i32Ptr(v int32) *int32      { return &v }
func f32Ptr(v float32) *float32  { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }

// requireContains is a helper that fails immediately if substring not found
func requireContains(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Fatalf("Output does not contain %q\nOutput:\n%s", substr, output)
	}
}
