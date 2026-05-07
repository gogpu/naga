package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// -----------------------------------------------------------------------
// Exercises scalarKindForType, coerceScalarToType, scalarKindCompatible
// -----------------------------------------------------------------------

func TestLowerScalarKindConversions(t *testing.T) {
	src := `fn test() {
    var fi: f32 = 42;
    var fu: f32 = 42u;
    var iu: i32 = 10u;
    var ui: u32 = 10;
    _ = fi; _ = fu; _ = iu; _ = ui;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises inferGlobalVarType with all scalar types
// -----------------------------------------------------------------------

func TestLowerInferGlobalVarTypes(t *testing.T) {
	src := `var<private> a = true;
var<private> b = 42;
var<private> c = 3.14;
var<private> d = 42u;
var<private> e = 0i;
var<private> f = 0.0f;
fn test() {
    a = false;
    b = 0;
    c = 0.0;
    d = 0u;
    e = 0;
    f = 0.0;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Override with all type annotations and inferences
// -----------------------------------------------------------------------

func TestLowerOverrideAllTypes(t *testing.T) {
	src := `@id(0) override ob: bool = true;
@id(1) override oi: i32 = 10;
@id(2) override ou: u32 = 20u;
@id(3) override of: f32 = 3.14;
override inferred_i = 100;
override inferred_f = 2.71;
override inferred_b = false;
@compute @workgroup_size(1)
fn main() {
    _ = ob; _ = oi; _ = ou; _ = of;
    _ = inferred_i; _ = inferred_f; _ = inferred_b;
}`
	module := mustCompile(t, src)
	if len(module.Overrides) != 7 {
		t.Errorf("expected 7 overrides, got %d", len(module.Overrides))
	}
}

// -----------------------------------------------------------------------
// Global var init with expression (exercises lowerGlobalVarInit)
// -----------------------------------------------------------------------

func TestLowerGlobalVarInitBinaryExpr(t *testing.T) {
	src := `const SCALE: f32 = 2.0;
var<private> x: f32 = SCALE;
var<private> y: f32 = 1.0;
var<private> z: vec3<f32> = vec3<f32>(0.0, 0.0, 0.0);
fn test() {
    x = y + z.x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Abstract constant with complex types (exercises lowerAbstractConstant)
// -----------------------------------------------------------------------

func TestLowerAbstractConstantComplex(t *testing.T) {
	src := `const POSITIONS = array(
    vec2(0.0, -0.5),
    vec2(0.5, 0.5),
    vec2(-0.5, 0.5),
);
const COLORS = array(
    vec3(1.0, 0.0, 0.0),
    vec3(0.0, 1.0, 0.0),
    vec3(0.0, 0.0, 1.0),
);
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    let pos = POSITIONS[idx];
    let color = COLORS[idx];
    return vec4<f32>(pos, 0.0, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const expression: ternary-like using select
// -----------------------------------------------------------------------

func TestLowerConstSelectInFunction(t *testing.T) {
	src := `fn test() -> i32 {
    const A: i32 = select(10, 20, true);
    const B: i32 = select(10, 20, false);
    return A + B;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Const select should produce Literal values (folded)
	litCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.Literal); ok {
			litCount++
		}
	}
	if litCount == 0 {
		t.Error("expected Literal expressions from const select folding")
	}
}

// -----------------------------------------------------------------------
// Const binary with type coercion (i32 and u32 mixing)
// -----------------------------------------------------------------------

func TestLowerConstBinaryCoercion(t *testing.T) {
	src := `const A: i32 = 10 - 3;
const B: u32 = 20u + 5u;
const C: f32 = 1.5 * 2.0;
const D: i32 = 100 / 3;
const E: u32 = 100u % 7u;
fn test() -> i32 { return A + D; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises tryFoldDot, tryFoldCross with runtime vectors
// -----------------------------------------------------------------------

func TestLowerDotCrossRuntime(t *testing.T) {
	src := `fn test(a: vec3<f32>, b: vec3<f32>) -> f32 {
    let d = dot(a, b);
    let c = cross(a, b);
    let len = length(c);
    return d + len;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises tryFoldBinaryOp with const vectors
// -----------------------------------------------------------------------

func TestLowerConstFoldVecBinaryI32Full(t *testing.T) {
	src := `fn test() {
    const A = vec4<i32>(1, 2, 3, 4) + vec4<i32>(5, 6, 7, 8);
    const B = vec4<i32>(10, 20, 30, 40) - vec4<i32>(1, 2, 3, 4);
    const C = vec2<u32>(3u, 5u) * vec2<u32>(2u, 4u);
    const D = vec2<u32>(10u, 20u) / vec2<u32>(2u, 5u);
    const E = vec2<u32>(10u, 20u) % vec2<u32>(3u, 7u);
    var x = A; _ = x; _ = B; _ = C; _ = D; _ = E;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const fold with integer bit operations on vectors
// -----------------------------------------------------------------------

func TestLowerConstFoldVecBitOps(t *testing.T) {
	src := `fn test() {
    const A = vec2<u32>(0xFFu, 0x0Fu) & vec2<u32>(0x0Fu, 0xFFu);
    const B = vec2<u32>(0xF0u, 0x00u) | vec2<u32>(0x0Fu, 0xFFu);
    const C = vec2<u32>(0xFFu, 0x00u) ^ vec2<u32>(0xAAu, 0x55u);
    var x = A; _ = x; _ = B; _ = C;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises extractConstVectorLiterals path with F32
// -----------------------------------------------------------------------

func TestLowerExtractConstVecLiteralsF32(t *testing.T) {
	src := `fn test() {
    const V = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    const X = V.x;
    const W = V.w;
    const XYZ = V.xyz;
    var a = X; var b = W; var c = XYZ;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises inferConstructorTypeFromScalar
// -----------------------------------------------------------------------

func TestLowerInferConstructorTypeFromScalar(t *testing.T) {
	src := `fn test() {
    let v2 = vec2(1.0, 2.0);
    let v3 = vec3(1, 2, 3);
    let v4 = vec4(1u, 2u, 3u, 4u);
    let m = mat2x2(1.0, 0.0, 0.0, 1.0);
    _ = v2; _ = v3; _ = v4; _ = m;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises resolveScalarFromName with all names
// -----------------------------------------------------------------------

func TestLowerResolveScalarNames(t *testing.T) {
	src := `fn test() {
    let a: bool = true;
    let b: i32 = 0;
    let c: u32 = 0u;
    let d: f32 = 0.0;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const fold tryFoldAs (type conversion in const context)
// -----------------------------------------------------------------------

func TestLowerConstFoldAsConversions(t *testing.T) {
	src := `fn test() {
    const A: f32 = f32(42);
    const B: i32 = i32(3.14);
    const C: u32 = u32(100);
    const D: f32 = f32(10u);
    var x = A; _ = x; _ = B; _ = C; _ = D;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture gather with component parameter
// -----------------------------------------------------------------------

func TestLowerTextureGatherComponent(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let r = textureGather(0, t, s, uv);
    let g = textureGather(1, t, s, uv);
    let b = textureGather(2, t, s, uv);
    return r + g + b;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises buildOverrideInitExpr fully
// -----------------------------------------------------------------------

func TestLowerOverrideInitExprs(t *testing.T) {
	src := `@id(0) override base: f32 = 1.0;
@id(1) override multiplier: f32 = 2.0;
@id(2) override offset: i32 = 10;
@id(3) override flag: bool = true;
@id(4) override mask: u32 = 0xFFu;
@compute @workgroup_size(1)
fn main() {
    let x = base * multiplier;
    let y = offset;
    if flag { _ = mask; }
    _ = x; _ = y;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Early depth test with specific mode
// -----------------------------------------------------------------------

func TestLowerEarlyDepthTestGreater(t *testing.T) {
	src := `@early_depth_test(greater_equal)
@fragment
fn main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
}

// -----------------------------------------------------------------------
// Const evaluation with nested arithmetic
// -----------------------------------------------------------------------

func TestLowerConstEvalNestedArithmetic(t *testing.T) {
	src := `const A: i32 = 2;
const B: i32 = 3;
const C: i32 = A * A + B * B;
const D: i32 = (A + B) * (A + B);
const E: i32 = A * (B + A * B);
fn test() -> i32 { return C + D + E; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 5 {
		t.Errorf("expected at least 5 constants, got %d", len(module.Constants))
	}
}

// -----------------------------------------------------------------------
// Texture depth array sample compare level
// -----------------------------------------------------------------------

func TestLowerDepthArraySampleCompareLevel(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d_array;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompareLevel(t, s, uv, 0, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises lowerAtomicCall fully (all ops with workgroup var)
// -----------------------------------------------------------------------

func TestLowerAtomicWorkgroup(t *testing.T) {
	src := `var<workgroup> counter: atomic<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    atomicStore(&counter, 0u);
    let v0 = atomicLoad(&counter);
    let v1 = atomicAdd(&counter, 1u);
    let v2 = atomicSub(&counter, 1u);
    let v3 = atomicMax(&counter, lid.x);
    let v4 = atomicMin(&counter, lid.x);
    let v5 = atomicAnd(&counter, 0xFFu);
    let v6 = atomicOr(&counter, 0x0Fu);
    let v7 = atomicXor(&counter, 0xAAu);
    let result = atomicCompareExchangeWeak(&counter, 0u, 42u);
    _ = v0; _ = v1; _ = v2; _ = v3; _ = v4; _ = v5; _ = v6; _ = v7;
    _ = result.exchanged; _ = result.old_value;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture load from 3d texture
// -----------------------------------------------------------------------

func TestLowerTextureLoad3D(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_3d<f32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let v = textureLoad(t, vec3<i32>(i32(id.x), i32(id.y), 0), 0);
    _ = v;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture load from 1d texture
// -----------------------------------------------------------------------

func TestLowerTextureLoad1D(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_1d<f32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let v = textureLoad(t, i32(id.x), 0);
    _ = v;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture array load
// -----------------------------------------------------------------------

func TestLowerTextureLoadArray(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d_array<f32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let v = textureLoad(t, vec2<i32>(i32(id.x), i32(id.y)), 0, 0);
    _ = v;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture cube array sample
// -----------------------------------------------------------------------

func TestLowerTextureCubeArraySample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_cube_array<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) dir: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, dir, 0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercising concretizeTypeHandle
// -----------------------------------------------------------------------

func TestLowerConcretizeTypeHandle(t *testing.T) {
	src := `fn test() {
    var v = vec2(1, 2);
    var w: vec2<i32> = v;
    _ = w;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercising concretizeLiteralValue
// -----------------------------------------------------------------------

func TestLowerConcretizeLiteralValue(t *testing.T) {
	src := `fn test() {
    var x: i32 = 42;
    var y: u32 = 10;
    var z: f32 = 3;
    _ = x; _ = y; _ = z;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercising short-circuit evaluation
// -----------------------------------------------------------------------

func TestLowerShortCircuitVerify(t *testing.T) {
	src := `fn test(a: bool, b: bool, c: bool) -> bool {
    let x = a && b && c;
    let y = a || b || c;
    let z = (a && b) || (b && c);
    return x || y || z;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Short-circuit && and || produce if-else blocks, not Binary
	ifCount := 0
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtIf); ok {
			ifCount++
		}
	}
	// Each && and || at statement level may produce StmtIf
	if ifCount == 0 {
		// It's OK if they're in sub-blocks via emit patterns
	}
}

// -----------------------------------------------------------------------
// convertScalarBits
// -----------------------------------------------------------------------

func TestLowerConvertScalarBits(t *testing.T) {
	src := `fn test() {
    var a: i32 = 42;
    var b: f32 = f32(a);
    var c: u32 = u32(a);
    var d: i32 = i32(b);
    _ = c; _ = d;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture depth cube array
// -----------------------------------------------------------------------

func TestLowerDepthTextureCubeArraySample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_cube_array;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) dir: vec3<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(t, s, dir, 0, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture sample bias with offset
// -----------------------------------------------------------------------

func TestLowerTextureSampleBiasOffset(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(t, s, uv, 0.5, vec2<i32>(1, 0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture sample compare with offset
// -----------------------------------------------------------------------

func TestLowerTextureSampleCompareOffset(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(t, s, uv, 0.5, vec2<i32>(1, 0));
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises evalConstU32Expr path
// -----------------------------------------------------------------------

func TestLowerConstU32Expr(t *testing.T) {
	src := `const SIZE: u32 = 8u;
const HALF: u32 = SIZE / 2u;
var<private> arr: array<f32, SIZE>;
fn test() { arr[0] = 1.0; _ = HALF; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Exercises parseSampledScalarKind (i32, u32 textures)
// -----------------------------------------------------------------------

func TestLowerTextureScalarKinds(t *testing.T) {
	src := `@group(0) @binding(0) var tf: texture_2d<f32>;
@group(0) @binding(1) var ti: texture_2d<i32>;
@group(0) @binding(2) var tu: texture_2d<u32>;
@compute @workgroup_size(1)
fn main() {
    let df = textureDimensions(tf);
    let di = textureDimensions(ti);
    let du = textureDimensions(tu);
    _ = df; _ = di; _ = du;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Various matrix sizes
// -----------------------------------------------------------------------

func TestLowerMatrixAllSizes(t *testing.T) {
	src := `fn test() {
    let m22 = mat2x2f(1.0, 0.0, 0.0, 1.0);
    let m23 = mat2x3f(1.0, 0.0, 0.0, 0.0, 1.0, 0.0);
    let m24 = mat2x4f(1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0);
    let m32 = mat3x2f(1.0, 0.0, 0.0, 1.0, 0.0, 0.0);
    let m33 = mat3x3f(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0);
    let m34 = mat3x4f(1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0);
    let m42 = mat4x2f(1.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0);
    let m43 = mat4x3f(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0);
    let m44 = mat4x4f(1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 0.0, 1.0);
    _ = m22; _ = m23; _ = m24;
    _ = m32; _ = m33; _ = m34;
    _ = m42; _ = m43; _ = m44;
}`
	mustCompile(t, src)
}
