package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// lowerBuiltinConstructor — builtin constructor paths (50%)
// ---------------------------------------------------------------------------

func TestLowerBuiltinConstructorVec2i(t *testing.T) {
	src := `fn test() {
    let a = vec2i(1, 2);
    let b = vec3u(1u, 2u, 3u);
    let c = vec4f(1.0, 2.0, 3.0, 4.0);
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

func TestLowerBuiltinConstructorMatSuffix(t *testing.T) {
	src := `fn test() {
    let m = mat2x2f(1.0, 0.0, 0.0, 1.0);
    let n = mat3x3f(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0);
    _ = m; _ = n;
}`
	mustCompile(t, src)
}

func TestLowerBuiltinConstructorFromVec(t *testing.T) {
	src := `fn test() {
    let v2 = vec2f(1.0, 2.0);
    let v3 = vec3f(v2, 3.0);
    let v4 = vec4f(v2, v2);
    let v4b = vec4f(v3, 4.0);
    _ = v3; _ = v4; _ = v4b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Texture gather operations (56.1% and 54.5%)
// ---------------------------------------------------------------------------

func TestLowerTextureGatherWithCoords(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGather(0, tex, samp, uv);
}`
	mustCompile(t, src)
}

func TestLowerTextureGatherCompareDepth(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_depth_2d;
@group(0) @binding(1) var samp: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureGatherCompare(tex, samp, uv, 0.5);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// typeShapeMatches — type matching (56.2%)
// ---------------------------------------------------------------------------

func TestLowerTypeShapeMatchesVecConvert(t *testing.T) {
	src := `fn test() {
    let v = vec3<f32>(1.0, 2.0, 3.0);
    let w: vec3<i32> = vec3<i32>(v);
    _ = w;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// tryFoldAs — type conversion folding (56.7%)
// ---------------------------------------------------------------------------

func TestLowerTryFoldAsI32ToF32(t *testing.T) {
	src := `fn test() {
    let x: f32 = f32(42i);
    let y: i32 = i32(3.14f);
    let z: u32 = u32(100i);
    _ = x; _ = y; _ = z;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Atomic operations — more coverage for lowerAtomicCall (51.7%)
// ---------------------------------------------------------------------------

func TestLowerAtomicAllOpsExtended(t *testing.T) {
	src := `var<workgroup> counter: atomic<u32>;
@compute @workgroup_size(64)
fn main() {
    atomicStore(&counter, 0u);
    let val = atomicLoad(&counter);
    atomicAdd(&counter, 1u);
    atomicSub(&counter, 1u);
    atomicMax(&counter, 100u);
    atomicMin(&counter, 0u);
    atomicAnd(&counter, 0xFFu);
    atomicOr(&counter, 0x0Fu);
    atomicXor(&counter, 0xAAu);
    let old = atomicExchange(&counter, val);
    _ = old;
}`
	mustCompile(t, src)
}

func TestLowerAtomicCompareExchangeWeak(t *testing.T) {
	src := `var<workgroup> counter: atomic<u32>;
@compute @workgroup_size(1)
fn main() {
    let result = atomicCompareExchangeWeak(&counter, 0u, 1u);
    _ = result.old_value;
    _ = result.exchanged;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeAbstractFloat — abstract float concretization (52.6%)
// ---------------------------------------------------------------------------

func TestLowerConcretizeAbstractFloatBinaryOps(t *testing.T) {
	src := `fn test(x: f32) -> f32 {
    var a = x + 1.0;
    var b = x * 2.0;
    var c = x - 0.5;
    var d = x / 3.0;
    return a + b + c + d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// deepCopyConstExpr — deep copy paths (52.6%)
// ---------------------------------------------------------------------------

func TestLowerDeepCopyConstExprLiteral(t *testing.T) {
	src := `fn test() {
    const A = select(10, 20, true);
    const B = select(10, 20, false);
    const C = select(1.0, 2.0, true);
    const D = select(1.0, 2.0, false);
    var a = A; var b = B; var c = C; var d = D;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeComponentLiterals — component literal concretization (57.1%)
// ---------------------------------------------------------------------------

func TestLowerConcretizeComponentLiterals(t *testing.T) {
	src := `fn test() {
    let v: vec3<f32> = vec3<f32>(1, 2, 3);
    let w: vec3<i32> = vec3<i32>(1, 2, 3);
    let x: vec3<u32> = vec3<u32>(1, 2, 3);
    _ = v; _ = w; _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// buildGlobalExpressions — global expression building (57.1%)
// ---------------------------------------------------------------------------

func TestLowerBuildGlobalExpressionsComplex(t *testing.T) {
	src := `struct Light {
    pos: vec3<f32>,
    color: vec4<f32>,
    intensity: f32,
}
const DEFAULT_LIGHT: Light = Light(
    vec3<f32>(0.0, 10.0, 0.0),
    vec4<f32>(1.0, 1.0, 1.0, 1.0),
    1.0
);
var<private> light: Light = DEFAULT_LIGHT;
fn test() -> f32 { return light.intensity; }`
	module := mustCompile(t, src)
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions")
	}
}

// ---------------------------------------------------------------------------
// Texture sampling with various overloads
// ---------------------------------------------------------------------------

func TestLowerTextureSampleBiasValue(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(tex, samp, uv, 1.0);
}`
	mustCompile(t, src)
}

func TestLowerTextureSampleLevelExplicit(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(tex, samp, uv, 0.0);
}`
	mustCompile(t, src)
}

func TestLowerTextureSampleGradDerivatives(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(tex, samp, uv, vec2<f32>(0.1, 0.0), vec2<f32>(0.0, 0.1));
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerAlias — alias more complex types
// ---------------------------------------------------------------------------

func TestLowerAliasArrayType(t *testing.T) {
	src := `alias FloatArray = array<f32, 4>;
fn test() {
    var arr: FloatArray = FloatArray(1.0, 2.0, 3.0, 4.0);
    _ = arr;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// scalarValueToLiteralWithType — through global expression paths (55.6%)
// ---------------------------------------------------------------------------

func TestLowerScalarValueToLiteralWithType(t *testing.T) {
	src := `const A: i32 = 42;
const B: u32 = 100u;
const C: f32 = 3.14;
const D: bool = true;
fn test() { _ = A; _ = B; _ = C; _ = D; }`
	module := mustCompile(t, src)
	if len(module.GlobalExpressions) < 4 {
		t.Errorf("expected at least 4 global expressions for typed constants, got %d", len(module.GlobalExpressions))
	}
}

// ---------------------------------------------------------------------------
// concretizeTypeHandle — type handle concretization (55.6%)
// ---------------------------------------------------------------------------

func TestLowerConcretizeTypeHandleAbstractVec(t *testing.T) {
	src := `const V = vec3(1, 2, 3);
fn test() {
    var a: vec3<i32> = V;
    _ = a;
}`
	mustCompile(t, src)
}

func TestLowerConcretizeTypeHandleAbstractArray(t *testing.T) {
	src := `const ARR = array(1.0, 2.0, 3.0);
fn test() {
    var a: array<f32, 3> = ARR;
    _ = a;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerFor with continue statement
// ---------------------------------------------------------------------------

func TestLowerForWithContinue(t *testing.T) {
	src := `fn test() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 20; i++) {
        if i % 2 == 0 {
            continue;
        }
        if i > 15 {
            break;
        }
        sum += i;
    }
    return sum;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Multiple output attributes on fragment shader
// ---------------------------------------------------------------------------

func TestLowerFragmentMultipleOutputs(t *testing.T) {
	src := `struct Output {
    @location(0) color: vec4<f32>,
    @location(1) normal: vec4<f32>,
}
@fragment
fn main() -> Output {
    return Output(vec4<f32>(1.0, 0.0, 0.0, 1.0), vec4<f32>(0.0, 0.0, 1.0, 1.0));
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
	if module.EntryPoints[0].Stage != ir.StageFragment {
		t.Error("expected fragment stage")
	}
}

// ---------------------------------------------------------------------------
// Vertex shader with multiple inputs and struct output
// ---------------------------------------------------------------------------

func TestLowerVertexShaderComplex(t *testing.T) {
	src := `struct VertexOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) color: vec3<f32>,
}
@vertex
fn main(
    @location(0) position: vec3<f32>,
    @location(1) texcoord: vec2<f32>,
    @location(2) color: vec3<f32>,
) -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4<f32>(position, 1.0);
    out.uv = texcoord;
    out.color = color;
    return out;
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
	if module.EntryPoints[0].Stage != ir.StageVertex {
		t.Error("expected vertex stage")
	}
	fn := module.EntryPoints[0].Function
	if len(fn.Arguments) != 3 {
		t.Errorf("expected 3 arguments, got %d", len(fn.Arguments))
	}
}

// ---------------------------------------------------------------------------
// Compute shader with workgroup variables and barriers
// ---------------------------------------------------------------------------

func TestLowerComputeShaderBarrier(t *testing.T) {
	src := `var<workgroup> shared: array<f32, 64>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    shared[lid] = f32(lid);
    workgroupBarrier();
    let val = shared[63u - lid];
    _ = val;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// buildOverrideGlobalExpr — override global expression building (52.9%)
// ---------------------------------------------------------------------------

func TestLowerBuildOverrideGlobalExprVarious(t *testing.T) {
	src := `@id(0) override width: f32 = 800.0;
@id(1) override height: f32 = 600.0;
override aspect: f32 = width / height;
override scale: f32 = 2.0;
override offset: f32 = 0.0;
@compute @workgroup_size(1)
fn main() {
    let a = aspect;
    let s = scale + offset;
    _ = a; _ = s;
}`
	module := mustCompile(t, src)
	if len(module.Overrides) < 5 {
		t.Errorf("expected at least 5 overrides, got %d", len(module.Overrides))
	}
}

// ---------------------------------------------------------------------------
// Unary operations at runtime — NOT, negate
// ---------------------------------------------------------------------------

func TestLowerUnaryOpsRuntime(t *testing.T) {
	src := `fn test(x: i32, y: f32, b: bool) {
    let neg_x = -x;
    let neg_y = -y;
    let not_b = !b;
    let bit_not = ~x;
    _ = neg_x; _ = neg_y; _ = not_b; _ = bit_not;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Array from const expressions
// ---------------------------------------------------------------------------

func TestLowerArrayFromConstExprs(t *testing.T) {
	src := `const A: f32 = 1.0;
const B: f32 = 2.0;
const C: f32 = 3.0;
const ARR: array<f32, 3> = array<f32, 3>(A, B, C);
fn test() -> f32 { return ARR[0]; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Select with vector args
// ---------------------------------------------------------------------------

func TestLowerSelectVecRuntime(t *testing.T) {
	src := `fn test(a: vec3<f32>, b: vec3<f32>, c: vec3<bool>) -> vec3<f32> {
    return select(a, b, c);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Pack/unpack builtins
// ---------------------------------------------------------------------------

func TestLowerPack4x8snorm(t *testing.T) {
	src := `fn test(v: vec4<f32>) -> u32 {
    return pack4x8snorm(v);
}`
	mustCompile(t, src)
}

func TestLowerUnpack4x8snorm(t *testing.T) {
	src := `fn test(v: u32) -> vec4<f32> {
    return unpack4x8snorm(v);
}`
	mustCompile(t, src)
}

func TestLowerPack2x16float(t *testing.T) {
	src := `fn test(v: vec2<f32>) -> u32 {
    return pack2x16float(v);
}`
	mustCompile(t, src)
}

func TestLowerUnpack2x16float(t *testing.T) {
	src := `fn test(v: u32) -> vec2<f32> {
    return unpack2x16float(v);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// insertBits / extractBits
// ---------------------------------------------------------------------------

func TestLowerInsertBits(t *testing.T) {
	src := `fn test(v: u32) -> u32 {
    return insertBits(v, 0xFFu, 4u, 8u);
}`
	mustCompile(t, src)
}

func TestLowerExtractBits(t *testing.T) {
	src := `fn test(v: u32) -> u32 {
    return extractBits(v, 4u, 8u);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Multiple binding groups
// ---------------------------------------------------------------------------

func TestLowerMultipleBindingGroups(t *testing.T) {
	src := `struct Uniforms { mvp: mat4x4<f32> }
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(1) @binding(0) var tex: texture_2d<f32>;
@group(1) @binding(1) var samp: sampler;
@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    return uniforms.mvp * vec4<f32>(pos, 1.0);
}`
	module := mustCompile(t, src)
	// Check that global variables have correct groups
	groups := make(map[uint32]bool)
	for _, gv := range module.GlobalVariables {
		if gv.Binding != nil {
			groups[gv.Binding.Group] = true
		}
	}
	if !groups[0] || !groups[1] {
		t.Errorf("expected binding groups 0 and 1, got %v", groups)
	}
}

// ---------------------------------------------------------------------------
// Let declarations with complex expressions
// ---------------------------------------------------------------------------

func TestLowerLetComplexExpressions(t *testing.T) {
	src := `fn test(x: f32) -> f32 {
    let a = x * x;
    let b = a + 2.0 * x + 1.0;
    let c = max(a, b);
    let d = clamp(c, 0.0, 100.0);
    let e = mix(a, b, 0.5);
    return d + e;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Pointer semantics
// ---------------------------------------------------------------------------

func TestLowerPointerSemantics(t *testing.T) {
	src := `fn add_to(p: ptr<function, i32>, val: i32) {
    *p = *p + val;
}
fn test() -> i32 {
    var x: i32 = 10;
    add_to(&x, 5);
    return x;
}`
	mustCompile(t, src)
}
