package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// -----------------------------------------------------------------------
// Constant folding: scalar math
// -----------------------------------------------------------------------

func TestLowerConstFoldMinMax(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = min(3.0, 5.0);
    const B: f32 = max(3.0, 5.0);
    const C: i32 = min(10, 20);
    const D: i32 = max(10, 20);
    const E: u32 = min(10u, 20u);
    const F: u32 = max(10u, 20u);
    return A + B;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldClamp(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = clamp(5.0, 0.0, 1.0);
    const B: i32 = clamp(50, 0, 100);
    const C: u32 = clamp(5u, 0u, 10u);
    return A;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldAbs(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = abs(-3.14);
    const B: i32 = abs(-42);
    return A;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldSign(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = sign(-3.14);
    const B: f32 = sign(0.0);
    const C: f32 = sign(3.14);
    const D: i32 = sign(-42);
    return A + B + C + f32(D);
}`
	mustCompile(t, src)
}

func TestLowerConstFoldTrigonometry(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = sin(0.0);
    const B: f32 = cos(0.0);
    const C: f32 = tan(0.0);
    const D: f32 = asin(0.0);
    const E: f32 = acos(1.0);
    const F: f32 = atan(0.0);
    return A + B + C + D + E + F;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldExpLog(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = exp(0.0);
    const B: f32 = exp2(0.0);
    const C: f32 = log(1.0);
    const D: f32 = log2(1.0);
    return A + B + C + D;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldSqrt(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = sqrt(4.0);
    const B: f32 = inverseSqrt(4.0);
    const C: f32 = pow(2.0, 3.0);
    return A + B + C;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldRound(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = ceil(1.3);
    const B: f32 = floor(1.7);
    const C: f32 = round(1.5);
    const D: f32 = fract(1.75);
    const E: f32 = trunc(1.9);
    return A + B + C + D + E;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldAngle(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = radians(180.0);
    const B: f32 = degrees(3.14159);
    return A + B;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldSaturate(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = saturate(1.5);
    const B: f32 = saturate(-0.5);
    const C: f32 = saturate(0.5);
    return A + B + C;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldFma(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = fma(2.0, 3.0, 4.0);
    return A;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldStep(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = step(0.5, 0.3);
    const B: f32 = step(0.5, 0.7);
    return A + B;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldMix(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = mix(0.0, 1.0, 0.5);
    return A;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldSmoothstep(t *testing.T) {
	src := `fn test() -> f32 {
    const A: f32 = smoothstep(0.0, 1.0, 0.5);
    return A;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldBitOps(t *testing.T) {
	src := `fn test() {
    let a: u32 = countOneBits(0xFFu);
    let b: u32 = countOneBits(0u);
    let c: u32 = reverseBits(1u);
    let d: u32 = firstTrailingBit(8u);
    let e: u32 = firstLeadingBit(8u);
    let f: i32 = countOneBits(0xFFi);
    let g: i32 = firstTrailingBit(8i);
    let h: i32 = firstLeadingBit(8i);
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f; _ = g; _ = h;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldCross(t *testing.T) {
	src := `fn test() -> vec3<f32> {
    let v1 = vec3<f32>(1.0, 0.0, 0.0);
    let v2 = vec3<f32>(0.0, 1.0, 0.0);
    return cross(v1, v2);
}`
	mustCompile(t, src)
}

func TestLowerConstFoldDotConst(t *testing.T) {
	src := `fn test() -> f32 {
    let v1 = vec3<f32>(1.0, 2.0, 3.0);
    let v2 = vec3<f32>(4.0, 5.0, 6.0);
    return dot(v1, v2);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Constant folding: binary operations
// -----------------------------------------------------------------------

func TestLowerConstFoldBinaryOps(t *testing.T) {
	src := `const A: i32 = 10 + 20;
const B: i32 = 30 - 10;
const C: i32 = 5 * 6;
const D: i32 = 30 / 5;
const E: i32 = 17 % 5;
const F: u32 = 0xFFu & 0x0Fu;
const G: u32 = 0xF0u | 0x0Fu;
const H: u32 = 0xFFu ^ 0x0Fu;
const I: u32 = 1u << 4u;
const J: u32 = 16u >> 2u;
fn test() -> i32 { return A + B + C + D + E; }`
	mustCompile(t, src)
}

func TestLowerConstFoldFloatBinaryOps(t *testing.T) {
	src := `const A: f32 = 1.0 + 2.0;
const B: f32 = 5.0 - 3.0;
const C: f32 = 2.0 * 3.0;
const D: f32 = 6.0 / 2.0;
fn test() -> f32 { return A + B + C + D; }`
	mustCompile(t, src)
}

func TestLowerConstFoldBoolOps(t *testing.T) {
	src := `const A: bool = true;
const B: bool = false;
fn test() -> bool {
    let eq = (1 == 1);
    let ne = (1 != 2);
    let lt = (1 < 2);
    let gt = (2 > 1);
    let le = (1 <= 1);
    let ge = (2 >= 1);
    return eq;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Constant folding: vector math
// -----------------------------------------------------------------------

func TestLowerConstFoldVectorMath(t *testing.T) {
	src := `fn test() {
    let v1 = vec3<f32>(1.0, 2.0, 3.0);
    let v2 = vec3<f32>(4.0, 5.0, 6.0);
    let vmin = min(v1, v2);
    let vmax = max(v1, v2);
    let vabs = abs(vec3<f32>(-1.0, -2.0, -3.0));
    _ = vmin; _ = vmax; _ = vabs;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldVectorBinaryOps(t *testing.T) {
	src := `fn test() {
    var v1 = vec2<f32>(1.0, 2.0);
    var v2 = vec2<f32>(3.0, 4.0);
    let vsum = v1 + v2;
    let vdiff = v2 - v1;
    let vprod = v1 * v2;
    let vdiv = v2 / v1;
    _ = vsum; _ = vdiff; _ = vprod; _ = vdiv;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Constant folding: unary
// -----------------------------------------------------------------------

func TestLowerConstFoldUnary(t *testing.T) {
	src := `const A: i32 = -42;
const B: f32 = -3.14;
const C: bool = !true;
const D: u32 = ~0u;
const E: i32 = ~0i;
fn test() -> i32 { return A; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Constant folding: select
// -----------------------------------------------------------------------

func TestLowerConstFoldSelect(t *testing.T) {
	src := `fn test() -> i32 {
    let a = select(10, 20, true);
    let b = select(10, 20, false);
    return a + b;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture operations (textureSample variants, textureLoad, etc.)
// -----------------------------------------------------------------------

func TestLowerTextureSample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, uv);
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
	fn := &module.EntryPoints[0].Function
	// Should have ImageSample in expressions
	found := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprImageSample); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected ExprImageSample for textureSample")
	}
}

func TestLowerTextureSampleBias(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(t, s, uv, 0.5);
}`
	mustCompile(t, src)
}

func TestLowerTextureSampleLevel(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(t, s, uv, 0.0);
}`
	mustCompile(t, src)
}

func TestLowerTextureSampleGrad(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(t, s, uv, vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0));
}`
	mustCompile(t, src)
}

func TestLowerTextureSampleCompare(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(t, s, uv, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

func TestLowerTextureSampleCompareLevel(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompareLevel(t, s, uv, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

func TestLowerTextureLoad(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let v = textureLoad(t, vec2<i32>(i32(id.x), i32(id.y)), 0);
}`
	mustCompile(t, src)
}

func TestLowerTextureStore(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t, vec2<i32>(i32(id.x), i32(id.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}`
	mustCompile(t, src)
}

func TestLowerTextureDimensions(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@compute @workgroup_size(1)
fn main() {
    let dims = textureDimensions(t);
    let dims_lod = textureDimensions(t, 0);
}`
	mustCompile(t, src)
}

func TestLowerTextureNumLevels(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@compute @workgroup_size(1)
fn main() {
    let levels = textureNumLevels(t);
}`
	mustCompile(t, src)
}

func TestLowerTextureNumSamples(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_multisampled_2d<f32>;
@compute @workgroup_size(1)
fn main() {
    let samples = textureNumSamples(t);
}`
	mustCompile(t, src)
}

func TestLowerTextureNumLayers(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d_array<f32>;
@compute @workgroup_size(1)
fn main() {
    let layers = textureNumLayers(t);
}`
	mustCompile(t, src)
}

func TestLowerTextureGather(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGather(0, t, s, uv);
}`
	mustCompile(t, src)
}

func TestLowerTextureGatherCompare(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGatherCompare(t, s, uv, 0.5);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Global variable type inference
// -----------------------------------------------------------------------

func TestLowerGlobalVarTypeInference(t *testing.T) {
	src := `var<private> x = 42;
var<private> y = 3.14;
fn test() { x = 1; y = 2.0; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Override type inference
// -----------------------------------------------------------------------

func TestLowerOverrideTypeInference(t *testing.T) {
	src := `override scale = 2.0;
override count = 10;
@compute @workgroup_size(1)
fn main() {
    let s = scale;
    let c = count;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const assert failure
// -----------------------------------------------------------------------

func TestLowerConstAssertFailure(t *testing.T) {
	src := `const_assert false;
fn test() {}`
	expectError(t, src, "")
}

// -----------------------------------------------------------------------
// For loop with compound operators
// -----------------------------------------------------------------------

func TestLowerForCompoundAssign(t *testing.T) {
	src := `fn test() -> i32 {
    var total: i32 = 0;
    for (var i: i32 = 0; i < 10; i += 1) {
        total += i;
    }
    for (var j: i32 = 100; j > 0; j -= 10) {
        total += j;
    }
    return total;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Switch with multiple selectors per case
// -----------------------------------------------------------------------

func TestLowerSwitchMultipleSelectors(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    switch x {
        case 1, 2, 3: { return 10; }
        case 4, 5: { return 20; }
        default: { return 0; }
    }
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture array
// -----------------------------------------------------------------------

func TestLowerTextureArraySample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d_array<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, uv, 0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Atomic compare exchange
// -----------------------------------------------------------------------

func TestLowerAtomicCompareExchange(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read_write> a: atomic<u32>;
@compute @workgroup_size(1)
fn main() {
    let result = atomicCompareExchangeWeak(&a, 0u, 1u);
    let exchanged = result.exchanged;
    let old_value = result.old_value;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Atomic operations (sub, max, min, and, or, xor)
// -----------------------------------------------------------------------

func TestLowerAtomicAllOps(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read_write> a: atomic<u32>;
@compute @workgroup_size(1)
fn main() {
    atomicStore(&a, 100u);
    let v0 = atomicLoad(&a);
    let v1 = atomicAdd(&a, 1u);
    let v2 = atomicSub(&a, 1u);
    let v3 = atomicMax(&a, 50u);
    let v4 = atomicMin(&a, 50u);
    let v5 = atomicAnd(&a, 0xFFu);
    let v6 = atomicOr(&a, 0x0Fu);
    let v7 = atomicXor(&a, 0xAAu);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture load from multisampled
// -----------------------------------------------------------------------

func TestLowerTextureLoadMultisampled(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_multisampled_2d<f32>;
@fragment
fn main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return textureLoad(t, vec2<i32>(i32(pos.x), i32(pos.y)), 0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Depth texture load
// -----------------------------------------------------------------------

func TestLowerTextureLoadDepth(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d;
@compute @workgroup_size(1)
fn main() {
    let d = textureLoad(t, vec2<i32>(0, 0), 0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture sample with offset
// -----------------------------------------------------------------------

func TestLowerTextureSampleOffset(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, uv, vec2<i32>(1, 0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Short alias type constructors (vec2i, mat2x2f, etc.)
// -----------------------------------------------------------------------

func TestLowerShortAliasConstructors(t *testing.T) {
	src := `fn test() {
    let a = vec2i(1, 2);
    let b = vec3u(1u, 2u, 3u);
    let c = vec4f(1.0, 2.0, 3.0, 4.0);
    let d = mat2x2f(1.0, 0.0, 0.0, 1.0);
    let e = mat3x3f(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Length and distance
// -----------------------------------------------------------------------

func TestLowerLengthDistance(t *testing.T) {
	src := `fn test() -> f32 {
    let v = vec3<f32>(1.0, 2.0, 3.0);
    let len = length(v);
    let d = distance(v, vec3<f32>(4.0, 5.0, 6.0));
    return len + d;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Normalize
// -----------------------------------------------------------------------

func TestLowerNormalize(t *testing.T) {
	src := `fn test() -> vec3<f32> {
    let v = vec3<f32>(1.0, 2.0, 3.0);
    return normalize(v);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Reflect and refract
// -----------------------------------------------------------------------

func TestLowerReflectRefract(t *testing.T) {
	src := `fn test() -> vec3<f32> {
    let n = vec3<f32>(0.0, 1.0, 0.0);
    let i = vec3<f32>(1.0, -1.0, 0.0);
    let r = reflect(i, n);
    let f = refract(i, n, 1.0);
    return r + f;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Determinant and transpose
// -----------------------------------------------------------------------

func TestLowerDeterminantTranspose(t *testing.T) {
	src := `fn test() {
    let m = mat2x2<f32>(1.0, 2.0, 3.0, 4.0);
    let d = determinant(m);
    let t = transpose(m);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Multiple entry points with different stages
// -----------------------------------------------------------------------

func TestLowerMultipleEntryPoints(t *testing.T) {
	src := `struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    out.uv = vec2<f32>(0.0, 0.0);
    return out;
}

@fragment
fn fs(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 0.0, 1.0);
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 2 {
		t.Errorf("expected 2 entry points, got %d", len(module.EntryPoints))
	}
}

// -----------------------------------------------------------------------
// Complex struct with arrays
// -----------------------------------------------------------------------

func TestLowerStructWithArray(t *testing.T) {
	src := `struct Particle {
    position: vec3<f32>,
    velocity: vec3<f32>,
    life: f32,
}
struct Particles {
    count: u32,
    data: array<Particle, 1024>,
}
fn test() {
    var p: Particles;
    p.count = 0u;
    p.data[0].position = vec3<f32>(0.0, 0.0, 0.0);
    p.data[0].velocity = vec3<f32>(1.0, 0.0, 0.0);
    p.data[0].life = 1.0;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Global constant arrays
// -----------------------------------------------------------------------

func TestLowerGlobalConstArray(t *testing.T) {
	src := `const VERTICES = array<vec2<f32>, 3>(
    vec2<f32>(-1.0, -1.0),
    vec2<f32>( 1.0, -1.0),
    vec2<f32>( 0.0,  1.0),
);
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(VERTICES[idx], 0.0, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Cast between numeric types
// -----------------------------------------------------------------------

func TestLowerNumericCasts(t *testing.T) {
	src := `fn test() {
    let a: f32 = f32(42i);
    let b: i32 = i32(3.14f);
    let c: u32 = u32(42i);
    let d: f32 = f32(42u);
    let e: i32 = i32(42u);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Assign to index expression
// -----------------------------------------------------------------------

func TestLowerAssignToIndex(t *testing.T) {
	src := `fn test() {
    var arr: array<i32, 4>;
    arr[0] = 10;
    arr[1] = 20;
    arr[2] = 30;
    arr[3] = 40;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Assign to member expression
// -----------------------------------------------------------------------

func TestLowerAssignToMember(t *testing.T) {
	src := `struct S { x: f32, y: f32 }
fn test() {
    var s: S;
    s.x = 1.0;
    s.y = 2.0;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Complex chained member/index
// -----------------------------------------------------------------------

func TestLowerChainedMemberIndex(t *testing.T) {
	src := `struct Inner { values: array<f32, 4> }
struct Outer { inner: Inner }
fn test() -> f32 {
    var o: Outer;
    o.inner.values[2] = 42.0;
    return o.inner.values[2];
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const expressions with type annotations
// -----------------------------------------------------------------------

func TestLowerConstWithTypeAnnotations(t *testing.T) {
	src := `const A: u32 = 42u;
const B: i32 = -10;
const C: f32 = 3.14;
const D: bool = true;
fn test() -> u32 { return A; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Vector binary constant eval
// -----------------------------------------------------------------------

func TestLowerVectorBinaryConstEval(t *testing.T) {
	src := `const A = vec2<i32>(1, 2) + vec2<i32>(3, 4);
const B = vec2<i32>(5, 6) - vec2<i32>(1, 2);
const C = vec2<i32>(2, 3) * vec2<i32>(4, 5);
fn test() -> vec2<i32> { return A; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const array size from constant
// -----------------------------------------------------------------------

func TestLowerConstArraySize(t *testing.T) {
	src := `const SIZE: u32 = 16u;
fn test() {
    var arr: array<f32, SIZE>;
    arr[0] = 1.0;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Storage format types
// -----------------------------------------------------------------------

func TestLowerStorageFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"rgba8unorm", "rgba8unorm"},
		{"rgba8snorm", "rgba8snorm"},
		{"rgba8uint", "rgba8uint"},
		{"rgba8sint", "rgba8sint"},
		{"rgba16uint", "rgba16uint"},
		{"rgba16sint", "rgba16sint"},
		{"rgba16float", "rgba16float"},
		{"r32uint", "r32uint"},
		{"r32sint", "r32sint"},
		{"r32float", "r32float"},
		{"rg32uint", "rg32uint"},
		{"rg32sint", "rg32sint"},
		{"rg32float", "rg32float"},
		{"rgba32uint", "rgba32uint"},
		{"rgba32sint", "rgba32sint"},
		{"rgba32float", "rgba32float"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `@group(0) @binding(0) var t: texture_storage_2d<` + tt.format + `, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t, vec2<i32>(0, 0), vec4<f32>(1.0));
}`
			mustCompile(t, src)
		})
	}
}

// -----------------------------------------------------------------------
// Nesting: if inside for inside while
// -----------------------------------------------------------------------

func TestLowerDeepNesting(t *testing.T) {
	src := `fn test() -> i32 {
    var result: i32 = 0;
    var k: i32 = 0;
    while k < 3 {
        for (var i: i32 = 0; i < 3; i++) {
            if i == k {
                result += 1;
            } else {
                result += 2;
            }
        }
        k += 1;
    }
    return result;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Multiple location outputs
// -----------------------------------------------------------------------

func TestLowerMultipleLocationOutputs(t *testing.T) {
	src := `struct FragOutput {
    @location(0) color: vec4<f32>,
    @location(1) normal: vec4<f32>,
    @location(2) position: vec4<f32>,
}
@fragment
fn main() -> FragOutput {
    var out: FragOutput;
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    out.normal = vec4<f32>(0.0, 1.0, 0.0, 1.0);
    out.position = vec4<f32>(0.0, 0.0, 1.0, 1.0);
    return out;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Matrix with all sizes
// -----------------------------------------------------------------------

func TestLowerAllMatrixSizes(t *testing.T) {
	src := `fn test() {
    var m22: mat2x2<f32>;
    var m23: mat2x3<f32>;
    var m24: mat2x4<f32>;
    var m32: mat3x2<f32>;
    var m33: mat3x3<f32>;
    var m34: mat3x4<f32>;
    var m42: mat4x2<f32>;
    var m43: mat4x3<f32>;
    var m44: mat4x4<f32>;
    _ = m22; _ = m23; _ = m24;
    _ = m32; _ = m33; _ = m34;
    _ = m42; _ = m43; _ = m44;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Pointer to array element
// -----------------------------------------------------------------------

func TestLowerPointerToArrayElement(t *testing.T) {
	src := `fn test() {
    var arr: array<i32, 4> = array<i32, 4>(1, 2, 3, 4);
    let p = &arr[2];
    *p = 42;
}`
	mustCompile(t, src)
}
