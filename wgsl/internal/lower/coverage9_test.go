package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// inferGlobalVarType — more types to boost from 29.6%
// ---------------------------------------------------------------------------

func TestLowerInferGlobalVarTypeI32(t *testing.T) {
	src := `var<private> x = 42i;
fn test() { x = 0; }`
	mustCompile(t, src)
}

func TestLowerInferGlobalVarTypeU32(t *testing.T) {
	src := `var<private> x = 42u;
fn test() { x = 0u; }`
	mustCompile(t, src)
}

func TestLowerInferGlobalVarTypeF32Explicit(t *testing.T) {
	src := `var<private> x = 3.14f;
fn test() { x = 0.0; }`
	mustCompile(t, src)
}

func TestLowerInferGlobalVarTypeVec2(t *testing.T) {
	src := `var<private> v = vec2<f32>(1.0, 2.0);
fn test() { _ = v; }`
	mustCompile(t, src)
}

func TestLowerInferGlobalVarTypeVec4(t *testing.T) {
	src := `var<private> v = vec4<i32>(1, 2, 3, 4);
fn test() { _ = v; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerGlobalVarInit — more init types to boost from 30%
// ---------------------------------------------------------------------------

func TestLowerGlobalVarInitBool(t *testing.T) {
	src := `var<private> flag: bool = true;
fn test() { flag = false; }`
	mustCompile(t, src)
}

func TestLowerGlobalVarInitVec(t *testing.T) {
	src := `var<private> v: vec2<f32> = vec2<f32>(1.0, 2.0);
fn test() { v.x = 0.0; }`
	mustCompile(t, src)
}

func TestLowerGlobalVarInitArray(t *testing.T) {
	src := `var<private> arr: array<i32, 3> = array<i32, 3>(1, 2, 3);
fn test() { arr[0] = 0; }`
	mustCompile(t, src)
}

func TestLowerGlobalVarInitMat(t *testing.T) {
	src := `var<private> m: mat3x3<f32> = mat3x3<f32>(
    1.0, 0.0, 0.0,
    0.0, 1.0, 0.0,
    0.0, 0.0, 1.0
);
fn test() { _ = m; }`
	mustCompile(t, src)
}

func TestLowerGlobalVarInitStructNested(t *testing.T) {
	src := `struct Inner { x: f32, y: f32 }
struct Outer { inner: Inner, z: f32 }
var<private> obj: Outer = Outer(Inner(1.0, 2.0), 3.0);
fn test() -> f32 { return obj.z; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstantIntExpr — all expression types to boost from 27.8%
// ---------------------------------------------------------------------------

func TestLowerEvalConstIntExprBinaryOps(t *testing.T) {
	src := `const A: i32 = 10;
const B: i32 = 3;
const ADD: i32 = A + B;
const SUB: i32 = A - B;
const MUL: i32 = A * B;
const DIV: i32 = A / B;
const MOD: i32 = A % B;
const BAND: i32 = A & B;
const BOR: i32 = A | B;
const BXOR: i32 = A ^ B;
const SHL: i32 = 1 << 4u;
const SHR: i32 = 16 >> 2u;
fn test() { _ = ADD; _ = SUB; _ = MUL; _ = DIV; _ = MOD; _ = BAND; _ = BOR; _ = BXOR; _ = SHL; _ = SHR; }`
	mustCompile(t, src)
}

func TestLowerEvalConstIntExprNegation(t *testing.T) {
	src := `const A: i32 = 42;
const NEG: i32 = -A;
fn test() -> i32 { return NEG; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// astLiteralToIRValue — all literal types to boost from 40.4%
// ---------------------------------------------------------------------------

func TestLowerASTLiteralToIRAllTypes(t *testing.T) {
	src := `fn test() {
    let a: i32 = 0;
    let b: u32 = 0u;
    let c: f32 = 0.0;
    let d: f32 = 0.0f;
    let e: bool = true;
    let f: bool = false;
    let g: i32 = 0x1F;
    let h: u32 = 0xFFu;
    let i: i32 = -100;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f; _ = g; _ = h; _ = i;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// inferOverrideType — more override types to boost from 40.9%
// ---------------------------------------------------------------------------

func TestLowerInferOverrideTypeAllScalars(t *testing.T) {
	src := `@id(0) override a: f32 = 1.0;
@id(1) override b: i32 = 1;
@id(2) override c: u32 = 1u;
@id(3) override d: bool = true;
override e = 2.0;
override f = 42;
override g = false;
override h = 10u;
@compute @workgroup_size(1)
fn main() { _ = a; _ = b; _ = c; _ = d; _ = e; _ = f; _ = g; _ = h; }`
	module := mustCompile(t, src)
	if len(module.Overrides) != 8 {
		t.Errorf("expected 8 overrides, got %d", len(module.Overrides))
	}
}

func TestLowerOverrideWithBinaryInit(t *testing.T) {
	src := `override a: f32 = 1.0 + 2.0;
override b: i32 = 10 - 5;
override c: u32 = 3u * 4u;
@compute @workgroup_size(1)
fn main() { _ = a; _ = b; _ = c; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// tryFoldBinaryOp — more binary op paths to boost from 43.8%
// ---------------------------------------------------------------------------

func TestLowerConstFoldVecBinaryF32Division(t *testing.T) {
	src := `fn test() {
    const A = vec3<f32>(10.0, 20.0, 30.0) / vec3<f32>(2.0, 4.0, 5.0);
    const B = vec2<f32>(6.0, 9.0) % vec2<f32>(4.0, 4.0);
    var a = A; var b = B;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldVecBinaryBool(t *testing.T) {
	src := `fn test() {
    const A = vec2<i32>(1, 2) == vec2<i32>(1, 3);
    const B = vec2<i32>(1, 2) != vec2<i32>(1, 3);
    const C = vec2<i32>(1, 2) < vec2<i32>(2, 1);
    var a = A; var b = B; var c = C;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// concretizeExpressionToScalar — more paths to boost from 43.6%
// ---------------------------------------------------------------------------

func TestLowerConcretizeExprToScalarAllTypes(t *testing.T) {
	src := `fn test(x: f32, y: i32, z: u32) {
    var a: f32 = x + 1;
    var b: i32 = y + 1;
    var c: u32 = z + 1u;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// resolveExprScalar — through various expression kinds to boost from 45%
// ---------------------------------------------------------------------------

func TestLowerResolveExprScalarFromVar(t *testing.T) {
	src := `fn test() {
    var a: f32 = 1.0;
    var b: i32 = 2;
    var c: u32 = 3u;
    var d: bool = true;
    a += 1.0;
    b += 1;
    c += 1u;
    d = !d;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// widenScalar — type widening in binary expressions to boost from 46.2%
// ---------------------------------------------------------------------------

func TestLowerWidenScalarInBinaryOps(t *testing.T) {
	src := `fn test() {
    var x: f32 = 1.0;
    var y = x + 1;
    var z = 2 + x;
    var w = x * 3;
    _ = y; _ = z; _ = w;
}`
	mustCompile(t, src)
}

func TestLowerWidenScalarVecPlusScalar(t *testing.T) {
	src := `fn test() {
    var v: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    var a = v + 1.0;
    var b = 2.0 * v;
    _ = a; _ = b;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// zeroLiteral — default zero value for different types to boost from 25%
// ---------------------------------------------------------------------------

func TestLowerZeroLiteralAllTypes(t *testing.T) {
	src := `fn test() {
    var a: f32;
    var b: i32;
    var c: u32;
    var d: bool;
    var e: vec2<f32>;
    var f: vec3<i32>;
    var g: vec4<u32>;
    var h: mat2x2<f32>;
    a = 0.0; b = 0; c = 0u; d = false;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f; _ = g; _ = h;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// convertLiteral — more conversion paths to boost from 23.8%
// ---------------------------------------------------------------------------

func TestLowerConvertLiteralAllPathsBool(t *testing.T) {
	src := `fn test() {
    const FI: i32 = i32(3.14f);
    const FU: u32 = u32(42f);
    const IF: f32 = f32(42i);
    const UF: f32 = f32(42u);
    const IB: bool = bool(1i);
    const UB: bool = bool(0u);
    const BB: bool = bool(true);
    var a = FI; var b = FU; var c = IF; var d = UF; var e = IB; var f = UB; var g = BB;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f; _ = g;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// tryConstantArrayIndex — compile-time array index evaluation to boost from 33.3%
// ---------------------------------------------------------------------------

func TestLowerTryConstantArrayIndexConst(t *testing.T) {
	src := `const ARR: array<f32, 4> = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
fn test() -> f32 {
    return ARR[0] + ARR[1] + ARR[2] + ARR[3];
}`
	mustCompile(t, src)
}

func TestLowerTryConstantArrayIndexNamed(t *testing.T) {
	src := `const IDX: u32 = 2u;
fn test() {
    var arr: array<f32, 4> = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    let v = arr[IDX];
    _ = v;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// resolveScalarFromName — more scalar type name resolution to boost from 38.5%
// ---------------------------------------------------------------------------

func TestLowerResolveScalarFromNameVecMatTypes(t *testing.T) {
	src := `fn test() {
    let a: vec2<f32> = vec2<f32>(1.0, 2.0);
    let b: vec3<i32> = vec3<i32>(1, 2, 3);
    let c: vec4<u32> = vec4<u32>(1u, 2u, 3u, 4u);
    let d: vec2<bool> = vec2<bool>(true, false);
    let e: mat2x2<f32> = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    let f: mat3x3<f32> = mat3x3<f32>(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0);
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// negateScalarBits — negate for all scalar kinds to boost from 44.4%
// ---------------------------------------------------------------------------

func TestLowerNegateVecU32(t *testing.T) {
	src := `fn test() {
    const V = -vec2<i32>(10, 20);
    const W = -vec3<f32>(1.0, 2.0, 3.0);
    const X = -vec4<i32>(1, 2, 3, 4);
    var a = V; var b = W; var c = X;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// constFoldCompose — compose folding paths to boost from 44.4%
// ---------------------------------------------------------------------------

func TestLowerConstFoldComposeSwizzle(t *testing.T) {
	src := `fn test() {
    const V = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    const XY = V.xy;
    const ZW = V.zw;
    const RECONSTRUCTED = vec4<f32>(XY, ZW);
    var x = RECONSTRUCTED;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstantArgs — all arg types to boost from 45.6%
// ---------------------------------------------------------------------------

func TestLowerEvalConstantArgsAllScalars(t *testing.T) {
	src := `const VI: vec4<i32> = vec4<i32>(1, 2, 3, 4);
const VU: vec4<u32> = vec4<u32>(1u, 2u, 3u, 4u);
const VF: vec4<f32> = vec4<f32>(1.0, 2.0, 3.0, 4.0);
fn test() { _ = VI; _ = VU; _ = VF; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// lowerTypeConstructorCall — type constructor in runtime context to boost from 47.1%
// ---------------------------------------------------------------------------

func TestLowerTypeConstructorRuntime(t *testing.T) {
	src := `fn test(x: f32, y: i32) {
    let a = vec2<f32>(x, x);
    let b = vec3<i32>(y, y, y);
    let c = f32(y);
    let d = i32(x);
    let e = u32(y);
    let f = bool(y);
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f;
}`
	mustCompile(t, src)
}

func TestLowerTypeConstructorVecFromVec(t *testing.T) {
	src := `fn test(v: vec3<f32>) {
    let w = vec3<i32>(v);
    let u = vec3<u32>(v);
    _ = w; _ = u;
}`
	mustCompile(t, src)
}

func TestLowerTypeConstructorMatFromMat(t *testing.T) {
	src := `fn test(m: mat2x2<f32>) {
    let n = mat2x2<f32>(m);
    _ = n;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// buildConstGlobalExpr composite path — to boost from 15.4%
// ---------------------------------------------------------------------------

func TestLowerBuildConstGlobalExprVec(t *testing.T) {
	src := `const V: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
const W: vec4<i32> = vec4<i32>(1, 2, 3, 4);
const X: vec2<u32> = vec2<u32>(10u, 20u);
fn test() { _ = V; _ = W; _ = X; }`
	module := mustCompile(t, src)
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions for vector constants")
	}
}

func TestLowerBuildConstGlobalExprArray(t *testing.T) {
	src := `const ARR: array<f32, 3> = array<f32, 3>(1.0, 2.0, 3.0);
fn test() -> f32 { return ARR[0]; }`
	module := mustCompile(t, src)
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions for array constant")
	}
}

// ---------------------------------------------------------------------------
// inferCompositeConstantType — more paths to boost from 24.4%
// ---------------------------------------------------------------------------

func TestLowerInferCompositeConstTypeWithBinaryArgs(t *testing.T) {
	src := `const A: i32 = 5;
const V: vec3<i32> = vec3<i32>(A, A + 1, A + 2);
fn test() { _ = V; }`
	mustCompile(t, src)
}

func TestLowerInferCompositeConstTypeFromUnaryArgs(t *testing.T) {
	src := `const V: vec2<f32> = vec2<f32>(-1.0, -2.0);
fn test() { _ = V; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// deepCopyConstantValue — through select on composite to boost from 25%
// ---------------------------------------------------------------------------

func TestLowerDeepCopyConstValueVecSelect(t *testing.T) {
	src := `fn test() {
    const A = vec4<i32>(1, 2, 3, 4);
    const B = vec4<i32>(5, 6, 7, 8);
    const C = select(A, B, vec4<bool>(true, false, true, false));
    var x = C;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// evalConstU32Expr — constant u32 expression to boost from 38.1%
// ---------------------------------------------------------------------------

func TestLowerEvalConstU32ExprComplex(t *testing.T) {
	src := `const N: u32 = 8u;
const M: u32 = N / 2u;
const K: u32 = M + N;
var<private> data: array<f32, K>;
fn test() -> f32 { return data[0]; }`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Ternary expressions in various contexts
// ---------------------------------------------------------------------------

func TestLowerTernaryInAssignment(t *testing.T) {
	src := `fn test(cond: bool) -> i32 {
    let a = select(1, 2, cond);
    let b = select(3.0f, 4.0f, cond);
    return a + i32(b);
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Matrix operations at runtime
// ---------------------------------------------------------------------------

func TestLowerMatrixMultiply(t *testing.T) {
	src := `fn test(m: mat4x4<f32>, v: vec4<f32>) -> vec4<f32> {
    return m * v;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	hasBinary := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprBinary); ok {
			hasBinary = true
		}
	}
	if !hasBinary {
		t.Error("expected Binary expression for matrix multiply")
	}
}

// ---------------------------------------------------------------------------
// Comparison operators at runtime
// ---------------------------------------------------------------------------

func TestLowerComparisonOpsRuntime(t *testing.T) {
	src := `fn test(a: f32, b: f32) -> bool {
    let eq = a == b;
    let ne = a != b;
    let lt = a < b;
    let le = a <= b;
    let gt = a > b;
    let ge = a >= b;
    return eq || ne || lt || le || gt || ge;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// constFoldAs — const type conversion for vector to boost from 19%
// ---------------------------------------------------------------------------

func TestLowerConstFoldAsVecFullCoverage(t *testing.T) {
	src := `fn test() {
    const V = vec3<f32>(1.5, 2.5, 3.5);
    const W = vec3<i32>(V);
    const X = vec3<u32>(W);
    const Y = vec3<f32>(X);
    var a = W; var b = X; var c = Y;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// resolveTargetScalarKind — through compound assignments to boost from 45.5%
// ---------------------------------------------------------------------------

func TestLowerResolveTargetScalarInCompound(t *testing.T) {
	src := `fn test() {
    var a: f32 = 1.0;
    var b: i32 = 10;
    var c: u32 = 20u;
    a += 1.0;
    b -= 3;
    c *= 2u;
    a /= 2.0;
    _ = a; _ = b; _ = c;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// extractVec3Floats / tryFoldCross / tryFoldDot with i32 vectors
// ---------------------------------------------------------------------------

func TestLowerConstFoldDotI32(t *testing.T) {
	src := `fn test() {
    const V = vec3<i32>(1, 0, 0);
    const W = vec3<i32>(0, 1, 0);
    const D = dot(V, W);
    var x = D;
    _ = x;
}`
	mustCompile(t, src)
}

// ---------------------------------------------------------------------------
// Texture3D and cube textures
// ---------------------------------------------------------------------------

func TestLowerTexture3D(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_3d<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main() -> @location(0) vec4<f32> {
    return textureSample(tex, samp, vec3<f32>(0.5, 0.5, 0.5));
}`
	mustCompile(t, src)
}

func TestLowerTextureCube(t *testing.T) {
	src := `@group(0) @binding(0) var tex: texture_cube<f32>;
@group(0) @binding(1) var samp: sampler;
@fragment
fn main() -> @location(0) vec4<f32> {
    return textureSample(tex, samp, vec3<f32>(0.0, 1.0, 0.0));
}`
	mustCompile(t, src)
}
