package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// Const fold: firstTrailingBit / firstLeadingBit / countOneBits / reverseBits
// ---------------------------------------------------------------------------

func TestLowerConstFoldFirstTrailingBit(t *testing.T) {
	src := `fn test() {
    const A: i32 = firstTrailingBit(8i);
    const B: u32 = firstTrailingBit(0u);
    const C: i32 = firstTrailingBit(0i);
    const D: u32 = firstTrailingBit(1u);
    var a = A; var b = B; var c = C; var d = D;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldFirstLeadingBit(t *testing.T) {
	src := `fn test() {
    const A: i32 = firstLeadingBit(8i);
    const B: u32 = firstLeadingBit(0u);
    const C: i32 = firstLeadingBit(-1i);
    const D: u32 = firstLeadingBit(255u);
    var a = A; var b = B; var c = C; var d = D;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldCountOneBits(t *testing.T) {
	src := `fn test() {
    const A: i32 = countOneBits(0xFi);
    const B: u32 = countOneBits(0xFFu);
    var a = A; var b = B;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldReverseBits(t *testing.T) {
	src := `fn test() {
    const A: u32 = reverseBits(1u);
    const B: i32 = reverseBits(1i);
    var a = A; var b = B;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// roundToF16 — half-precision rounding (via quantizeToF16 builtin)
// ---------------------------------------------------------------------------

func TestLowerConstFoldQuantizeToF16(t *testing.T) {
	src := `fn test() {
    const A: f32 = quantizeToF16(1.0);
    const B: f32 = quantizeToF16(0.0);
    const C: f32 = quantizeToF16(0.5);
    var a = A; var b = B; var c = C;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// constFoldCompose — compose const folding
// ---------------------------------------------------------------------------

func TestLowerConstFoldComposeVec(t *testing.T) {
	// const vector composed from literals
	src := `fn test() {
    const V = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    const A: f32 = V.x;
    const B: f32 = V.w;
    var a = A; var b = B;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// resolveTargetScalarKind — target scalar resolution in binary ops
// ---------------------------------------------------------------------------

func TestLowerResolveTargetScalarInBinary(t *testing.T) {
	src := `fn test(x: f32) -> f32 {
    var a: f32 = x + 1.0;
    var b: f32 = 2.0 * x;
    var c: f32 = x / 3.0;
    return a + b + c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeConstantScalar — type coercion in composite constants
// ---------------------------------------------------------------------------

func TestLowerConcretizeConstScalarVec(t *testing.T) {
	// Integer constants in a float vector context should concretize
	src := `const V: vec2<f32> = vec2<f32>(1, 2);
fn test() -> f32 { return V.x; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstantArgExpr — binary expr as const arg
// ---------------------------------------------------------------------------

func TestLowerEvalConstantArgExprFloat(t *testing.T) {
	src := `const A: f32 = 2.0;
const V: vec3<f32> = vec3<f32>(A + 1.0, A * 2.0, A / 3.0);
fn test() -> f32 { return V.x; }`
	mustCompile(t, src)
}

func TestLowerEvalConstantArgExprInt(t *testing.T) {
	src := `const A: i32 = 5;
const V: vec2<i32> = vec2<i32>(A + 1, A * 2);
fn test() -> i32 { return V.x; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// createZeroComponents — explicit exercise with typed zero-value constructors
// ---------------------------------------------------------------------------

func TestLowerCreateZeroComponentsAllTypes(t *testing.T) {
	src := `const ZV2: vec2<i32> = vec2<i32>();
const ZV3: vec3<u32> = vec3<u32>();
const ZV4: vec4<f32> = vec4<f32>();
const ZM: mat2x2<f32> = mat2x2<f32>();
fn test() { _ = ZV2; _ = ZV3; _ = ZV4; _ = ZM; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// abstractScalarKind — all literal kinds including suffixed
// ---------------------------------------------------------------------------

func TestLowerAbstractScalarKindAllSuffixes(t *testing.T) {
	// Typed literals in composite constant context
	src := `const A: vec2<f32> = vec2<f32>(1.0f, 2.0f);
const B: vec2<i32> = vec2<i32>(1i, 2i);
const C: vec2<u32> = vec2<u32>(1u, 2u);
fn test() { _ = A; _ = B; _ = C; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// groupMatrixConstantColumns — matrix constant column grouping
// ---------------------------------------------------------------------------

func TestLowerGroupMatrixConstColumns3x3(t *testing.T) {
	src := `const M: mat3x3<f32> = mat3x3<f32>(
    1.0, 0.0, 0.0,
    0.0, 1.0, 0.0,
    0.0, 0.0, 1.0
);
fn test() { _ = M; }`
	module := mustCompile(t, src)
	found := false
	for _, c := range module.Constants {
		if c.Name == "M" {
			found = true
		}
	}
	if !found {
		t.Error("expected constant 'M'")
	}
}

// ---------------------------------------------------------------------------
// inferCompositeConstantType — with array and matrix constructors
// ---------------------------------------------------------------------------

func TestLowerInferCompositeConstTypeArray(t *testing.T) {
	src := `const ARR: array<f32, 5> = array<f32, 5>(1.0, 2.0, 3.0, 4.0, 5.0);
fn test() -> f32 { return ARR[0]; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// literalScalar / literalScalarType — through const fold paths
// ---------------------------------------------------------------------------

func TestLowerLiteralScalarTypesAllKinds(t *testing.T) {
	src := `fn test() {
    const A: i32 = 42i;
    const B: u32 = 42u;
    const C: f32 = 3.14f;
    const D: bool = true;
    const E: i32 = clamp(A, 0, 100);
    const F: u32 = clamp(B, 0u, 100u);
    var a = A; var b = B; var c = C; var d = D; var e = E; var f = F;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// scalarValueToAbstractLiteral — through abstract constant usage
// ---------------------------------------------------------------------------

func TestLowerAbstractConstantUsageMultipleContexts(t *testing.T) {
	// Abstract constant used in different typed contexts
	src := `const VAL = 42;
fn test() {
    var a: i32 = VAL;
    var b: u32 = u32(VAL);
    var c: f32 = f32(VAL);
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// extractConstVectorLiterals — vector literal extraction
// ---------------------------------------------------------------------------

func TestLowerExtractConstVecLiteralsAllSwizzles(t *testing.T) {
	src := `fn test() {
    const V = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    const A: f32 = V.x;
    const B: f32 = V.y;
    const C: f32 = V.z;
    const D: f32 = V.w;
    const XY = V.xy;
    const ZW = V.zw;
    const XYZ = V.xyz;
    var a = A; var b = B; var c = C; var d = D;
    var xy = XY; var zw = ZW; var xyz = XYZ;
    _ = a; _ = b; _ = c; _ = d; _ = xy; _ = zw; _ = xyz;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// buildConstGlobalExpr — CompositeValue with named sub-constants
// ---------------------------------------------------------------------------

func TestLowerBuildConstGlobalExprComposite(t *testing.T) {
	src := `const V: vec4<f32> = vec4<f32>(1.0, 2.0, 3.0, 4.0);
fn test() -> vec4<f32> { return V; }`
	module := mustCompile(t, src)
	// Verify global expressions were built
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions for composite constant")
	}
}

// ---------------------------------------------------------------------------
// evalConstantIdent with abstract and concrete references
// ---------------------------------------------------------------------------

func TestLowerEvalConstantIdentAbstract(t *testing.T) {
	// Abstract constant used in const context
	src := `const SIZE = 64;
fn test() {
    var x: i32 = SIZE;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerEvalConstantIdentConcrete(t *testing.T) {
	src := `const SIZE: u32 = 32u;
@compute @workgroup_size(SIZE) fn main() {}`
	module := mustCompile(t, src)
	if module.EntryPoints[0].Workgroup[0] != 32 {
		t.Errorf("workgroup[0] = %d, want 32", module.EntryPoints[0].Workgroup[0])
	}
}

// ---------------------------------------------------------------------------
// evalExpressionAsConstantInt — complex const int expressions
// ---------------------------------------------------------------------------

func TestLowerConstIntExprNested(t *testing.T) {
	src := `const A: u32 = 8u;
const B: u32 = A * 2u;
const C: u32 = B + A;
@compute @workgroup_size(C) fn main() {}`
	module := mustCompile(t, src)
	// C = 8*2 + 8 = 24
	if module.EntryPoints[0].Workgroup[0] != 24 {
		t.Errorf("workgroup[0] = %d, want 24", module.EntryPoints[0].Workgroup[0])
	}
}

// ---------------------------------------------------------------------------
// typeName — type name resolution for all types
// ---------------------------------------------------------------------------

func TestLowerTypeNameResolutionAll(t *testing.T) {
	src := `fn test(
    a: bool,
    b: i32,
    c: u32,
    d: f32,
    e: vec2<f32>,
    f: vec3<i32>,
    g: vec4<u32>,
    h: mat2x2<f32>,
    i: mat3x3<f32>,
    j: mat4x4<f32>,
) {
    _ = a; _ = b; _ = c; _ = d; _ = e;
    _ = f; _ = g; _ = h; _ = i; _ = j;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	if len(fn.Arguments) != 10 {
		t.Errorf("expected 10 arguments, got %d", len(fn.Arguments))
	}
}

// ---------------------------------------------------------------------------
// Additional const fold builtins: mix, fma, step, smoothstep
// ---------------------------------------------------------------------------

func TestLowerConstFoldMixHalf(t *testing.T) {
	src := `fn test() {
    const A: f32 = mix(0.0, 1.0, 0.5);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldFmaMulAdd(t *testing.T) {
	src := `fn test() {
    const A: f32 = fma(2.0, 3.0, 1.0);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldStepEdge(t *testing.T) {
	src := `fn test() {
    const A: f32 = step(0.5, 1.0);
    const B: f32 = step(0.5, 0.3);
    var a = A; var b = B;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldSmoothstepHalf(t *testing.T) {
	src := `fn test() {
    const A: f32 = smoothstep(0.0, 1.0, 0.5);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Const fold: saturate, degrees, radians
// ---------------------------------------------------------------------------

func TestLowerConstFoldSaturateClamp(t *testing.T) {
	src := `fn test() {
    const A: f32 = saturate(1.5);
    const B: f32 = saturate(-0.5);
    const C: f32 = saturate(0.5);
    var a = A; var b = B; var c = C;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldDegrees(t *testing.T) {
	src := `fn test() {
    const A: f32 = degrees(3.14159);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldRadians(t *testing.T) {
	src := `fn test() {
    const A: f32 = radians(180.0);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Complex compound assignment paths
// ---------------------------------------------------------------------------

func TestLowerCompoundAssignVecElement(t *testing.T) {
	src := `fn test() {
    var v: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    v.x += 1.0;
    v.y -= 0.5;
    v.z *= 2.0;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerGlobalVarInit with zero value
// ---------------------------------------------------------------------------

func TestLowerGlobalVarInitZeroValue(t *testing.T) {
	src := `var<private> counter: i32;
fn test() { counter = 1; }`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "counter" {
			found = true
		}
	}
	if !found {
		t.Error("expected global variable 'counter'")
	}
}

// ---------------------------------------------------------------------------
// Struct with nested structs
// ---------------------------------------------------------------------------

func TestLowerNestedStructs(t *testing.T) {
	src := `struct Vec2 { x: f32, y: f32 }
struct Rect { min: Vec2, max: Vec2 }
fn test() {
    var r: Rect = Rect(Vec2(0.0, 0.0), Vec2(1.0, 1.0));
    r.min.x = -1.0;
    _ = r;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Array indexing with runtime and const indices
// ---------------------------------------------------------------------------

func TestLowerArrayIndexRuntimeAndConst(t *testing.T) {
	src := `fn test(idx: u32) -> f32 {
    var arr: array<f32, 4> = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    let first = arr[0];
    let dynamic = arr[idx];
    return first + dynamic;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have both AccessIndex (const) and Access (runtime)
	hasAccessIndex := false
	hasAccess := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprAccessIndex); ok {
			hasAccessIndex = true
		}
		if _, ok := expr.Kind.(ir.ExprAccess); ok {
			hasAccess = true
		}
	}
	if !hasAccessIndex {
		t.Error("expected AccessIndex for constant array index")
	}
	if !hasAccess {
		t.Error("expected Access for runtime array index")
	}
}

// ---------------------------------------------------------------------------
// buildOverrideInitExpr — override with complex init expressions
// ---------------------------------------------------------------------------

func TestLowerOverrideInitComplexExpr(t *testing.T) {
	src := `override base: f32 = 10.0;
override scale: f32 = 2.0;
override offset: f32 = base * scale + 1.0;
@compute @workgroup_size(1)
fn main() { _ = offset; }`
	module := mustCompile(t, src)
	if len(module.Overrides) < 3 {
		t.Errorf("expected 3 overrides, got %d", len(module.Overrides))
	}
}

// ---------------------------------------------------------------------------
// lowerAlias — type alias declarations
// ---------------------------------------------------------------------------

func TestLowerAliasAll(t *testing.T) {
	src := `alias F32 = f32;
alias Vec3F = vec3<F32>;
alias Mat4 = mat4x4<F32>;
fn test(v: Vec3F, m: Mat4) -> F32 {
    return v.x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Const fold: atan2, ldexp
// ---------------------------------------------------------------------------

func TestLowerConstFoldAtan2(t *testing.T) {
	src := `fn test() {
    const A: f32 = atan2(1.0, 1.0);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldLdexp(t *testing.T) {
	src := `fn test() {
    const A: f32 = ldexp(1.0, 4i);
    var x = A;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Bitwise operations in runtime context
// ---------------------------------------------------------------------------

func TestLowerBitwiseOpsRuntime(t *testing.T) {
	src := `fn test(a: u32, b: u32) -> u32 {
    let and = a & b;
    let or = a | b;
    let xor = a ^ b;
    let not = ~a;
    let shl = a << 2u;
    let shr = a >> 2u;
    return and + or + xor + not + shl + shr;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Const fold binary comparison operators (exercises foldBinaryLiterals comparison path)
// ---------------------------------------------------------------------------

func TestLowerConstFoldComparisonOps(t *testing.T) {
	src := `fn test() {
    const A = 5 == 5;
    const B = 5 != 3;
    const C = 3 < 5;
    const D = 5 <= 5;
    const E = 7 > 3;
    const F = 5 >= 5;
    const AF = 1.0 == 1.0;
    const BF = 1.0 < 2.0;
    const CF = 2.0 > 1.0;
    var a: bool = A; var b: bool = B; var c: bool = C;
    var d: bool = D; var e: bool = E; var f: bool = F;
    var af: bool = AF; var bf: bool = BF; var cf: bool = CF;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f;
    _ = af; _ = bf; _ = cf;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Multisampled textures
// ---------------------------------------------------------------------------

func TestLowerMultisampledTexture(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;
@fragment
fn main() -> @location(0) vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0i);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Storage texture
// ---------------------------------------------------------------------------

func TestLowerStorageTextureWrite(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main() {
    textureStore(tex, vec2<i32>(0, 0), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Depth texture
// ---------------------------------------------------------------------------

func TestLowerDepthTexture(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_depth_2d;
@group(0) @binding(1) var samp: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) f32 {
    return textureSampleCompare(tex, samp, uv, 0.5);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract constant binary op — exercises evalConstBinaryExpr
// ---------------------------------------------------------------------------

func TestLowerAbstractConstBinaryAllOps(t *testing.T) {
	src := `const A = 10;
const B = 3;
const ADD = A + B;
const SUB = A - B;
const MUL = A * B;
const DIV = A / B;
const MOD = A % B;
fn test() {
    var a: i32 = ADD;
    var b: i32 = SUB;
    var c: i32 = MUL;
    var d: i32 = DIV;
    var e: i32 = MOD;
    _ = a; _ = b; _ = c; _ = d; _ = e;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Abstract constant unary — exercises evalConstUnaryExpr
// ---------------------------------------------------------------------------

func TestLowerAbstractConstUnaryNegLiteral(t *testing.T) {
	// Abstract constant with unary negate on literal (supported path)
	src := `const NEG = -42;
const NEG_F = -3.14;
fn test() {
    var a: i32 = NEG;
    var b: f32 = NEG_F;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

func TestLowerAbstractConstUnaryBangLiteral(t *testing.T) {
	src := `const T = true;
const F = !true;
fn test() {
    var a: bool = T;
    var b: bool = F;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// negateScalarBits — through vector negation
// ---------------------------------------------------------------------------

func TestLowerNegateScalarBitsU32(t *testing.T) {
	src := `fn test() {
    const V = -vec4<i32>(1, 2, 3, 4);
    var x = V;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerExpressionForRef — reference context for store targets
// ---------------------------------------------------------------------------

func TestLowerExpressionForRefArray(t *testing.T) {
	src := `fn test() {
    var arr: array<i32, 4> = array<i32, 4>(1, 2, 3, 4);
    arr[0] = 100;
    arr[3] = 400;
    _ = arr;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Continue statement
// ---------------------------------------------------------------------------

func TestLowerContinueStatement(t *testing.T) {
	src := `fn test() {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 10; i++) {
        if i % 2 == 0 {
            continue;
        }
        sum += i;
    }
    _ = sum;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Break statement
// ---------------------------------------------------------------------------

func TestLowerBreakStatement(t *testing.T) {
	src := `fn test() {
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        i++;
    }
    _ = i;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Multiple return values from different paths
// ---------------------------------------------------------------------------

func TestLowerMultipleReturnTypes(t *testing.T) {
	src := `struct Result { value: f32, valid: bool }
fn test(x: f32) -> Result {
    if x > 0.0 {
        return Result(x, true);
    }
    return Result(0.0, false);
}`
	mustCompile(t, src)
}
