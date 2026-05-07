package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// -----------------------------------------------------------------------
// Const binary/unary with vectors (exercises tryFoldVectorBinaryOp etc.)
// -----------------------------------------------------------------------

func TestLowerConstFoldVecBinOpI32(t *testing.T) {
	src := `fn test() {
    const A = vec2<i32>(1, 2) + vec2<i32>(3, 4);
    const B = vec2<i32>(5, 6) - vec2<i32>(1, 2);
    const C = vec2<i32>(2, 3) * vec2<i32>(4, 5);
    var x = A;
    _ = x; _ = B; _ = C;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldVecBinOpF32(t *testing.T) {
	src := `fn test() {
    const A = vec3<f32>(1.0, 2.0, 3.0) + vec3<f32>(4.0, 5.0, 6.0);
    const B = vec3<f32>(1.0, 2.0, 3.0) * vec3<f32>(2.0, 2.0, 2.0);
    var x = A;
    _ = x; _ = B;
}`
	mustCompile(t, src)
}

func TestLowerConstFoldVecUnaryNeg(t *testing.T) {
	src := `fn test() {
    const V = vec3<i32>(1, 2, 3);
    const NEG = -V;
    var x = NEG;
    _ = x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Vector from scalar splat (exercises concretizeLiteralToScalar paths)
// -----------------------------------------------------------------------

func TestLowerSplatAbstractConcretization(t *testing.T) {
	src := `fn test() {
    var v2f: vec2<f32> = vec2(1);
    var v3i: vec3<i32> = vec3(0);
    var v4u: vec4<u32> = vec4(0);
    _ = v2f; _ = v3i; _ = v4u;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Abstract int→float promotion in mixed expressions
// -----------------------------------------------------------------------

func TestLowerAbstractIntFloatPromotion(t *testing.T) {
	src := `fn test() {
    var x: f32 = 1 + 2.0;
    var y: f32 = 3.0 * 4;
    _ = x; _ = y;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Construct with partial types (exercises inferConstructorType)
// -----------------------------------------------------------------------

func TestLowerPartialConstruct(t *testing.T) {
	src := `fn test() {
    let v = vec4(1.0, 2.0, 3.0, 4.0);
    let a = array(1, 2, 3, 4, 5);
    let m = mat2x2(1.0, 0.0, 0.0, 1.0);
    _ = v; _ = a; _ = m;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Nested function calls in const context
// -----------------------------------------------------------------------

func TestLowerNestedFnInConst(t *testing.T) {
	src := `fn test() {
    const A: f32 = min(max(1.0, 2.0), 3.0);
    const B: f32 = abs(min(-5.0, -3.0));
    var x = A + B;
    _ = x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const folding of vec swizzle components
// -----------------------------------------------------------------------

func TestLowerConstFoldSwizzleComponents(t *testing.T) {
	src := `fn test() {
    const V = vec4<i32>(10, 20, 30, 40);
    const X = V.x;
    const Z = V.z;
    const XY = V.xy;
    var x = X;
    _ = x; _ = Z; _ = XY;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const vec fold with As (type conversion)
// -----------------------------------------------------------------------

func TestLowerConstFoldAs(t *testing.T) {
	src := `fn test() {
    const A: f32 = f32(42);
    const B: i32 = i32(3.14);
    const C: u32 = u32(42);
    var x = A;
    _ = x; _ = B; _ = C;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const bitwise NOT vector fold
// -----------------------------------------------------------------------

func TestLowerConstFoldBitwiseNotVec(t *testing.T) {
	src := `fn test() {
    const V = vec2<u32>(0xFFu, 0x00u);
    const INV = ~V;
    var x = INV;
    _ = x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Multiple function params and return types
// -----------------------------------------------------------------------

func TestLowerFunctionMultiParams(t *testing.T) {
	src := `fn compute(a: vec3<f32>, b: vec3<f32>, c: f32, d: bool) -> vec3<f32> {
    if d {
        return a * c;
    }
    return b * c;
}
fn test() -> vec3<f32> {
    return compute(vec3<f32>(1.0), vec3<f32>(2.0), 0.5, true);
}`
	module := mustCompile(t, src)
	if len(module.Functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(module.Functions))
	}
	fn := &module.Functions[0]
	if len(fn.Arguments) != 4 {
		t.Errorf("expected 4 arguments, got %d", len(fn.Arguments))
	}
}

// -----------------------------------------------------------------------
// Pointer to struct member
// -----------------------------------------------------------------------

func TestLowerPointerToStructMember(t *testing.T) {
	src := `struct S { x: f32, y: f32 }
fn set_x(p: ptr<function, f32>) { *p = 42.0; }
fn test() -> f32 {
    var s: S;
    set_x(&s.x);
    return s.x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Global const with complex expression tree
// -----------------------------------------------------------------------

func TestLowerGlobalConstComplexTree(t *testing.T) {
	src := `const A: i32 = 2;
const B: i32 = 3;
const C: i32 = A * B + A - B;
const D: i32 = (A + B) * (A - B);
const E: i32 = -D;
fn test() -> i32 { return C + D + E; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 5 {
		t.Errorf("expected at least 5 constants, got %d", len(module.Constants))
	}
}

// -----------------------------------------------------------------------
// Large switch (many cases)
// -----------------------------------------------------------------------

func TestLowerLargeSwitch(t *testing.T) {
	src := `fn test(x: u32) -> u32 {
    switch x {
        case 0u: { return 0u; }
        case 1u: { return 1u; }
        case 2u: { return 4u; }
        case 3u: { return 9u; }
        case 4u: { return 16u; }
        case 5u: { return 25u; }
        case 6u: { return 36u; }
        case 7u: { return 49u; }
        case 8u: { return 64u; }
        case 9u: { return 81u; }
        default: { return x * x; }
    }
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Nested struct construction
// -----------------------------------------------------------------------

func TestLowerNestedStructConstruction(t *testing.T) {
	src := `struct Vec2 { x: f32, y: f32 }
struct Rect { origin: Vec2, size: Vec2 }
fn test() -> Rect {
    var r: Rect;
    r.origin.x = 0.0;
    r.origin.y = 0.0;
    r.size.x = 100.0;
    r.size.y = 50.0;
    return r;
}`
	module := mustCompile(t, src)
	if len(module.Types) < 2 {
		t.Error("expected at least 2 types (Vec2, Rect)")
	}
}

// -----------------------------------------------------------------------
// Texture sample bias with array texture
// -----------------------------------------------------------------------

func TestLowerTextureSampleBiasArray(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d_array<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(t, s, uv, 0, 0.5);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// If-else chain with multiple returns
// -----------------------------------------------------------------------

func TestLowerIfElseChainReturns(t *testing.T) {
	src := `fn classify(x: f32) -> i32 {
    if x < 0.0 {
        return -1;
    } else if x == 0.0 {
        return 0;
    } else if x < 1.0 {
        return 1;
    } else if x < 10.0 {
        return 2;
    } else {
        return 3;
    }
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// All paths return — function should end properly
	if len(fn.Body) == 0 {
		t.Error("expected non-empty body")
	}
}

// -----------------------------------------------------------------------
// Abstract constant inlining with member access
// -----------------------------------------------------------------------

func TestLowerAbstractConstMemberAccess(t *testing.T) {
	src := `const V = vec4(1.0, 2.0, 3.0, 4.0);
fn test() -> f32 {
    return V.x + V.y + V.z + V.w;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Abstract constant inlining with index access
// -----------------------------------------------------------------------

func TestLowerAbstractConstIndexAccess(t *testing.T) {
	src := `const ARR = array(10, 20, 30);
fn test(i: u32) -> i32 {
    let a = ARR[0];
    let b = ARR[i];
    return a + b;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture sample with offset
// -----------------------------------------------------------------------

func TestLowerTextureSampleLevelOffset(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(t, s, uv, 0.0, vec2<i32>(1, 0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Interleaved loads and stores
// -----------------------------------------------------------------------

func TestLowerInterleavedLoadStore(t *testing.T) {
	src := `fn test() -> i32 {
    var a: i32 = 1;
    var b: i32 = 2;
    var c: i32 = a + b;
    a = c * 2;
    b = a - c;
    c = a + b;
    return c;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Verify we have Store statements
	storeCount := 0
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtStore); ok {
			storeCount++
		}
	}
	if storeCount == 0 {
		t.Error("expected Store statements for var assignments")
	}
}

// -----------------------------------------------------------------------
// Nested blocks with local const
// -----------------------------------------------------------------------

func TestLowerNestedBlocksLocalConst(t *testing.T) {
	src := `fn test() -> i32 {
    const A = 10;
    {
        const B = A + 5;
        {
            const C = B * 2;
            return C;
        }
    }
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Mix of entry points and helper functions
// -----------------------------------------------------------------------

func TestLowerMixedEntryAndHelper(t *testing.T) {
	src := `fn helper(x: f32) -> f32 { return x * x; }
@vertex
fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    let x = helper(f32(idx));
    return vec4<f32>(x, 0.0, 0.0, 1.0);
}
@compute @workgroup_size(1)
fn cs() {
    let y = helper(42.0);
}`
	module := mustCompile(t, src)
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 helper function, got %d", len(module.Functions))
	}
	if len(module.EntryPoints) != 2 {
		t.Errorf("expected 2 entry points, got %d", len(module.EntryPoints))
	}
}

// -----------------------------------------------------------------------
// Scalar constructors (bool, f32, i32, u32 from various input types)
// -----------------------------------------------------------------------

func TestLowerScalarConstructors(t *testing.T) {
	src := `fn test() {
    var xi: i32 = 10;
    var xf: f32 = 3.14;
    var xu: u32 = 42u;
    var xb: bool = true;

    let to_f32_from_i = f32(xi);
    let to_f32_from_u = f32(xu);
    let to_i32_from_f = i32(xf);
    let to_i32_from_u = i32(xu);
    let to_u32_from_i = u32(xi);
    let to_u32_from_f = u32(xf);
    let to_bool = bool(xi);

    _ = to_f32_from_i; _ = to_f32_from_u;
    _ = to_i32_from_f; _ = to_i32_from_u;
    _ = to_u32_from_i; _ = to_u32_from_f;
    _ = to_bool; _ = xb;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Expression: Access with dynamic index
// -----------------------------------------------------------------------

func TestLowerDynamicArrayAccess(t *testing.T) {
	src := `fn test(idx: u32) -> i32 {
    var arr = array<i32, 8>(0, 1, 2, 3, 4, 5, 6, 7);
    return arr[idx];
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have Access expression for dynamic index
	found := false
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprAccess); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected ExprAccess for dynamic array index")
	}
}

// -----------------------------------------------------------------------
// Matrix vector multiply (exercises binary lowering paths)
// -----------------------------------------------------------------------

func TestLowerMatVecMultiply(t *testing.T) {
	src := `fn test() -> vec4<f32> {
    var m: mat4x4<f32>;
    var v: vec4<f32> = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    return m * v;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have Binary expression for matrix multiply
	found := false
	for _, expr := range fn.Expressions {
		if b, ok := expr.Kind.(ir.ExprBinary); ok {
			if b.Op == ir.BinaryMultiply {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected Binary(Multiply) for matrix*vector")
	}
}

// -----------------------------------------------------------------------
// Atomic I32 operations
// -----------------------------------------------------------------------

func TestLowerAtomicI32(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read_write> counter: atomic<i32>;
@compute @workgroup_size(1)
fn main() {
    atomicStore(&counter, 0);
    let v = atomicLoad(&counter);
    let added = atomicAdd(&counter, 1);
    let subbed = atomicSub(&counter, 1);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Various interpolation sampling types
// -----------------------------------------------------------------------

func TestLowerInterpolationSampling(t *testing.T) {
	src := `struct VOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) @interpolate(flat) flat_val: i32,
    @location(1) @interpolate(linear, sample) sample_val: f32,
    @location(2) @interpolate(perspective, centroid) persp_centroid: f32,
    @location(3) color: vec4<f32>,
}
@vertex
fn vs(@builtin(vertex_index) idx: u32) -> VOut {
    var out: VOut;
    out.pos = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    out.flat_val = 0;
    out.sample_val = 0.0;
    out.persp_centroid = 0.0;
    out.color = vec4<f32>(1.0);
    return out;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture load from storage texture (read)
// -----------------------------------------------------------------------

func TestLowerTextureLoadStorage(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, read>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let v = textureLoad(t, vec2<i32>(i32(id.x), i32(id.y)));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Comparison operators with unsigned
// -----------------------------------------------------------------------

func TestLowerUnsignedComparisons(t *testing.T) {
	src := `fn test(a: u32, b: u32) -> bool {
    let lt = a < b;
    let gt = a > b;
    let le = a <= b;
    let ge = a >= b;
    let eq = a == b;
    let ne = a != b;
    return lt || gt || le || ge || eq || ne;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Float comparison operators
// -----------------------------------------------------------------------

func TestLowerFloatComparisons(t *testing.T) {
	src := `fn test(a: f32, b: f32) -> bool {
    let lt = a < b;
    let gt = a > b;
    let le = a <= b;
    let ge = a >= b;
    return lt || gt || le || ge;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Empty entry point (no body statements)
// -----------------------------------------------------------------------

func TestLowerEmptyEntryPoint(t *testing.T) {
	src := `@compute @workgroup_size(1) fn main() {}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
}

// -----------------------------------------------------------------------
// Let with type annotation
// -----------------------------------------------------------------------

func TestLowerLetWithTypeAnnotation(t *testing.T) {
	src := `fn test() {
    let x: f32 = 42;
    let y: u32 = 10;
    let z: i32 = 5;
    _ = x; _ = y; _ = z;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const assert true (should pass)
// -----------------------------------------------------------------------

func TestLowerConstAssertTrue(t *testing.T) {
	src := `const_assert true;
const_assert 1 < 2;
const A = 10;
const_assert A > 0;
fn test() {}`
	mustCompile(t, src)
}
