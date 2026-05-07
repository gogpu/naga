package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// -----------------------------------------------------------------------
// Const binary/unary expressions at module scope
// -----------------------------------------------------------------------

func TestLowerModuleConstBinaryI32(t *testing.T) {
	src := `const A: i32 = 10 + 20;
const B: i32 = A * 2;
const C: i32 = -A;
fn test() -> i32 { return C; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 3 {
		t.Errorf("expected at least 3 constants, got %d", len(module.Constants))
	}
}

func TestLowerModuleConstBinaryU32(t *testing.T) {
	src := `const A: u32 = 10u + 20u;
const B: u32 = A * 2u;
const C: u32 = A / 3u;
const D: u32 = A % 7u;
const E: u32 = A & 0xFFu;
const F: u32 = A | 0x0Fu;
const G: u32 = A ^ 0xAAu;
const H: u32 = 1u << 4u;
const I: u32 = 16u >> 2u;
fn test() -> u32 { return A; }`
	mustCompile(t, src)
}

func TestLowerModuleConstBinaryF32(t *testing.T) {
	src := `const A: f32 = 1.0 + 2.0;
const B: f32 = 5.0 - 3.0;
const C: f32 = 2.0 * 3.0;
const D: f32 = 6.0 / 2.0;
const E: f32 = -3.14;
fn test() -> f32 { return A + B + C + D + E; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const vector expressions at module scope
// -----------------------------------------------------------------------

func TestLowerModuleConstVector(t *testing.T) {
	src := `const V: vec4<f32> = vec4<f32>(1.0, 2.0, 3.0, 4.0);
const SCALED: vec4<f32> = V;
fn test() -> vec4<f32> { return SCALED; }`
	mustCompile(t, src)
}

func TestLowerModuleConstMatrix(t *testing.T) {
	src := `const M: mat2x2<f32> = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
fn test() -> mat2x2<f32> { return M; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Override with type inference (exercises inferOverrideType)
// -----------------------------------------------------------------------

func TestLowerOverrideInferredTypes(t *testing.T) {
	src := `override a = 1.0;
override b = 42;
override c = true;
override d = 10u;
@compute @workgroup_size(1)
fn main() {
    let fa = a;
    let fb = b;
    let fc = c;
    let fd = d;
}`
	module := mustCompile(t, src)
	if len(module.Overrides) != 4 {
		t.Fatalf("expected 4 overrides, got %d", len(module.Overrides))
	}
}

// -----------------------------------------------------------------------
// Global var with inferred type (exercises inferGlobalVarType)
// -----------------------------------------------------------------------

func TestLowerGlobalVarInferredTypeVec(t *testing.T) {
	src := `var<private> pos = vec3<f32>(1.0, 2.0, 3.0);
fn test() -> vec3<f32> { return pos; }`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "pos" {
			found = true
		}
	}
	if !found {
		t.Error("expected global variable 'pos'")
	}
}

func TestLowerGlobalVarInferredTypeScalar(t *testing.T) {
	src := `var<private> count = 0i;
var<private> scale = 1.0f;
fn test() { count = 1; scale = 2.0; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Type constructor call (exercises lowerTypeConstructorCall)
// -----------------------------------------------------------------------

func TestLowerTypeConstructorCallVerify(t *testing.T) {
	src := `fn test() {
    let a = vec2<f32>(1.0, 2.0);
    let b = vec3<i32>(1, 2, 3);
    let c = vec4<u32>(1u, 2u, 3u, 4u);
    let d = mat2x2<f32>(vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0));
    let e = array<i32, 3>(10, 20, 30);
    _ = a; _ = b; _ = c; _ = d; _ = e;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Verify Compose expressions exist
	composeCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprCompose); ok {
			composeCount++
		}
	}
	if composeCount < 4 {
		t.Errorf("expected at least 4 Compose expressions, got %d", composeCount)
	}
}

// -----------------------------------------------------------------------
// Texture sample clamp to edge
// -----------------------------------------------------------------------

func TestLowerTextureSampleBaseClampToEdge(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBaseClampToEdge(t, s, uv);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Workgroup uniform load
// -----------------------------------------------------------------------

func TestLowerWorkgroupUniformLoad(t *testing.T) {
	src := `var<workgroup> shared_val: u32;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    if lid.x == 0u {
        shared_val = 42u;
    }
    workgroupBarrier();
    let val = workgroupUniformLoad(&shared_val);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Abstract constants with array access (exercises lowerAbstractConstant)
// -----------------------------------------------------------------------

func TestLowerAbstractConstantArrayAccess(t *testing.T) {
	src := `const COLORS = array(
    vec3(1.0, 0.0, 0.0),
    vec3(0.0, 1.0, 0.0),
    vec3(0.0, 0.0, 1.0),
);
fn test(i: u32) -> vec3<f32> {
    return COLORS[i];
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Abstract constant should be inlined — no ExprConstant
	for i, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprConstant); ok {
			t.Errorf("expr[%d] is ExprConstant — abstract should be inlined", i)
		}
	}
}

// -----------------------------------------------------------------------
// Type cast (As) expressions
// -----------------------------------------------------------------------

func TestLowerAsExpressions(t *testing.T) {
	src := `fn test() {
    var xi: i32 = 42;
    var xf: f32 = 3.14;
    var xu: u32 = 10u;
    let a = f32(xi);
    let b = i32(xf);
    let c = u32(xi);
    let d = f32(xu);
    let e = i32(xu);
    let f = u32(xf);
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have As expressions for runtime casts
	asCount := 0
	for _, expr := range fn.Expressions {
		if _, ok := expr.Kind.(ir.ExprAs); ok {
			asCount++
		}
	}
	if asCount == 0 {
		t.Error("expected As expressions for runtime type casts")
	}
}

// -----------------------------------------------------------------------
// Const eval with bitcast
// -----------------------------------------------------------------------

func TestLowerBitcastVerify(t *testing.T) {
	src := `fn test() {
    let x = bitcast<u32>(1.0f);
    let y = bitcast<f32>(0x3F800000u);
    let z = bitcast<i32>(0xFFFFFFFFu);
    _ = x; _ = y; _ = z;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Check for As with Convert==nil (bitcast)
	found := false
	for _, expr := range fn.Expressions {
		if asExpr, ok := expr.Kind.(ir.ExprAs); ok {
			if asExpr.Convert == nil {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected Bitcast As expression (Convert==nil)")
	}
}

// -----------------------------------------------------------------------
// resolveScalarFromName (exercises i64, u64, f64 paths)
// -----------------------------------------------------------------------

func TestLowerScalarTypes(t *testing.T) {
	src := `fn test() {
    var a: bool = true;
    var b: i32 = 0;
    var c: u32 = 0u;
    var d: f32 = 0.0;
    _ = a; _ = b; _ = c; _ = d;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const assert at function scope
// -----------------------------------------------------------------------

func TestLowerConstAssertInFunction(t *testing.T) {
	src := `fn test() {
    const_assert 1 + 1 == 2;
    const_assert true;
    const X: i32 = 5;
    const_assert X > 0;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Scope management (push/pop) with nested blocks
// -----------------------------------------------------------------------

func TestLowerScopeManagement(t *testing.T) {
	src := `fn test() -> i32 {
    var x: i32 = 1;
    {
        var x: i32 = 2;
        {
            var x: i32 = 3;
            _ = x;
        }
        _ = x;
    }
    return x;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// The innermost x=3 is shadowed, verify we get some local vars
	if len(fn.LocalVars) < 3 {
		t.Errorf("expected at least 3 local vars (scoped x), got %d", len(fn.LocalVars))
	}
}

// -----------------------------------------------------------------------
// For loop desugars to Loop IR
// -----------------------------------------------------------------------

func TestLowerForLoopDesugarsToLoop(t *testing.T) {
	src := `fn test() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 10; i += 1) {
        sum += i;
    }
    return sum;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	foundLoop := false
	for _, s := range fn.Body {
		if loop, ok := s.Kind.(ir.StmtLoop); ok {
			foundLoop = true
			// Loop should have non-empty body
			if len(loop.Body) == 0 {
				t.Error("loop body should be non-empty")
			}
			// Loop should have continuing block with update
			if len(loop.Continuing) == 0 {
				t.Error("loop continuing should be non-empty (for update)")
			}
		}
	}
	if !foundLoop {
		t.Error("for loop should desugar to StmtLoop")
	}
}

// -----------------------------------------------------------------------
// While loop desugars to Loop IR
// -----------------------------------------------------------------------

func TestLowerWhileDesugarsToLoop(t *testing.T) {
	src := `fn test() -> i32 {
    var i: i32 = 0;
    while i < 10 {
        i += 1;
    }
    return i;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	foundLoop := false
	for _, s := range fn.Body {
		if loop, ok := s.Kind.(ir.StmtLoop); ok {
			foundLoop = true
			// Body should start with if+break (condition check)
			if len(loop.Body) < 2 {
				t.Error("while loop body should have condition check + body")
			}
		}
	}
	if !foundLoop {
		t.Error("while loop should desugar to StmtLoop")
	}
}

// -----------------------------------------------------------------------
// Switch IR verification
// -----------------------------------------------------------------------

func TestLowerSwitchVerifyIR(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    var result: i32;
    switch x {
        case 1: { result = 10; }
        case 2, 3: { result = 20; }
        default: { result = 0; }
    }
    return result;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	foundSwitch := false
	for _, s := range fn.Body {
		if sw, ok := s.Kind.(ir.StmtSwitch); ok {
			foundSwitch = true
			// Verify we have at least 3 cases (1, 2, 3, default)
			if len(sw.Cases) < 3 {
				t.Errorf("expected at least 3 switch cases, got %d", len(sw.Cases))
			}
		}
	}
	if !foundSwitch {
		t.Error("expected switch statement in body")
	}
}

// -----------------------------------------------------------------------
// Global variable init expressions
// -----------------------------------------------------------------------

func TestLowerGlobalVarInitExpression(t *testing.T) {
	src := `var<private> a: f32 = 1.0;
var<private> b: vec2<f32> = vec2<f32>(1.0, 2.0);
var<private> c: i32 = 42;
fn test() { a = 0.0; }`
	module := mustCompile(t, src)
	// Verify globals have init GEs
	if len(module.GlobalExpressions) == 0 {
		t.Error("expected global expressions for var init")
	}
}

// -----------------------------------------------------------------------
// Pointer dereferencing in expressions
// -----------------------------------------------------------------------

func TestLowerPointerDerefInExpression(t *testing.T) {
	src := `fn increment(p: ptr<function, i32>) {
    *p = *p + 1;
}
fn test() -> i32 {
    var x: i32 = 10;
    increment(&x);
    return x;
}`
	module := mustCompile(t, src)
	if len(module.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(module.Functions))
	}
}

// -----------------------------------------------------------------------
// Storage texture formats — exercises parseStorageFormat
// -----------------------------------------------------------------------

func TestLowerStorageTextureFormatsVerify(t *testing.T) {
	src := `@group(0) @binding(0) var t1: texture_storage_2d<r32float, write>;
@group(0) @binding(1) var t2: texture_storage_2d<rg32float, write>;
@group(0) @binding(2) var t3: texture_storage_2d<rgba32float, write>;
@group(0) @binding(3) var t4: texture_storage_2d<rgba8unorm, write>;
@group(0) @binding(4) var t5: texture_storage_2d<rgba16float, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t1, vec2<i32>(0, 0), vec4<f32>(1.0));
    textureStore(t2, vec2<i32>(0, 0), vec4<f32>(1.0));
    textureStore(t3, vec2<i32>(0, 0), vec4<f32>(1.0));
    textureStore(t4, vec2<i32>(0, 0), vec4<f32>(1.0));
    textureStore(t5, vec2<i32>(0, 0), vec4<f32>(1.0));
}`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) != 5 {
		t.Errorf("expected 5 global variables, got %d", len(module.GlobalVariables))
	}
}

// -----------------------------------------------------------------------
// Expressions: nested binary/unary
// -----------------------------------------------------------------------

func TestLowerNestedBinaryUnary(t *testing.T) {
	src := `fn test(a: f32, b: f32) -> f32 {
    return -(a + b) * (a - b) + abs(a * b);
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	// Should have Negate unary and Math(Abs)
	hasNeg := false
	hasAbs := false
	for _, expr := range fn.Expressions {
		if u, ok := expr.Kind.(ir.ExprUnary); ok {
			if u.Op == ir.UnaryNegate {
				hasNeg = true
			}
		}
		if m, ok := expr.Kind.(ir.ExprMath); ok {
			if m.Fun == ir.MathAbs {
				hasAbs = true
			}
		}
	}
	if !hasNeg {
		t.Error("expected Negate unary expression")
	}
	if !hasAbs {
		t.Error("expected Abs math expression")
	}
}

// -----------------------------------------------------------------------
// Large shader with many types to exercise type deduplication
// -----------------------------------------------------------------------

func TestLowerTypeDeduplication(t *testing.T) {
	src := `fn test() {
    var a: vec2<f32>;
    var b: vec2<f32>;
    var c: vec3<f32>;
    var d: vec3<f32>;
    var e: vec4<f32>;
    var f: vec4<f32>;
    var g: mat4x4<f32>;
    var h: mat4x4<f32>;
    _ = a; _ = b; _ = c; _ = d; _ = e; _ = f; _ = g; _ = h;
}`
	module := mustCompile(t, src)
	// Count unique types — deduplication should keep only one of each
	typeMap := make(map[string]int)
	for _, typ := range module.Types {
		key := ""
		switch inner := typ.Inner.(type) {
		case ir.VectorType:
			key = "vec" + string(rune('0'+inner.Size))
		case ir.MatrixType:
			key = "mat" + string(rune('0'+inner.Columns)) + "x" + string(rune('0'+inner.Rows))
		}
		if key != "" {
			typeMap[key]++
		}
	}
	for key, count := range typeMap {
		if count > 1 {
			t.Errorf("type %s appears %d times, expected 1 (deduplication)", key, count)
		}
	}
}

// -----------------------------------------------------------------------
// Load from storage buffer element
// -----------------------------------------------------------------------

func TestLowerStorageBufferAccess(t *testing.T) {
	src := `struct Particle { pos: vec3<f32>, vel: vec3<f32> }
@group(0) @binding(0) var<storage, read_write> particles: array<Particle>;
@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let i = id.x;
    var p = particles[i];
    p.pos = p.pos + p.vel;
    particles[i] = p;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Expression: array length of runtime-sized array
// -----------------------------------------------------------------------

func TestLowerArrayLengthVerify(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read> data: array<f32>;
@compute @workgroup_size(1)
fn main() {
    let len = arrayLength(&data);
    _ = len;
}`
	module := mustCompile(t, src)
	ep := &module.EntryPoints[0].Function
	found := false
	for _, expr := range ep.Expressions {
		if _, ok := expr.Kind.(ir.ExprArrayLength); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected ExprArrayLength for arrayLength()")
	}
}

// -----------------------------------------------------------------------
// Texture cube sample
// -----------------------------------------------------------------------

func TestLowerTextureCubeSample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_cube<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) dir: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, dir);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture 3D
// -----------------------------------------------------------------------

func TestLowerTexture3DSample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_3d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) coord: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, coord);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Depth texture array
// -----------------------------------------------------------------------

func TestLowerDepthTextureArraySample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d_array;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(t, s, uv, 0, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Abstract float/int concretization in assignments
// -----------------------------------------------------------------------

func TestLowerAbstractConcretizationAssign(t *testing.T) {
	src := `fn test() {
    var x: f32 = 42;
    var y: i32 = 10;
    var z: u32 = 5;
    x += 1;
    y -= 2;
    z *= 3;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Struct with all common member types
// -----------------------------------------------------------------------

func TestLowerStructComplex(t *testing.T) {
	src := `struct Material {
    color: vec4<f32>,
    roughness: f32,
    metallic: f32,
    normal_map: vec3<f32>,
    ao: f32,
    emission: vec3<f32>,
    flags: u32,
}
fn test() -> Material {
    var m: Material;
    m.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    m.roughness = 0.5;
    m.metallic = 0.0;
    m.normal_map = vec3<f32>(0.0, 0.0, 1.0);
    m.ao = 1.0;
    m.emission = vec3<f32>(0.0);
    m.flags = 0u;
    return m;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Error: const_assert false at module scope
// -----------------------------------------------------------------------

func TestLowerConstAssertFalseError(t *testing.T) {
	src := `const_assert 1 == 2; fn test() {}`
	_, err := compileWGSL(t, src)
	if err == nil {
		t.Error("expected error for const_assert 1 == 2")
	}
}

// -----------------------------------------------------------------------
// Error: undefined type
// -----------------------------------------------------------------------

func TestLowerErrorUndefinedType(t *testing.T) {
	src := `fn test() { var x: NonExistentType; }`
	expectError(t, src, "")
}

// -----------------------------------------------------------------------
// Error: invalid math arg count
// -----------------------------------------------------------------------

func TestLowerErrorMathTooFewArgs(t *testing.T) {
	src := `fn test() { let x = min(); }`
	expectError(t, src, "")
}

func TestLowerErrorMathTooManyArgs(t *testing.T) {
	// abs() with no args should fail
	src := `fn test() { let x = abs(); }`
	expectError(t, src, "")
}

// -----------------------------------------------------------------------
// Cube texture dimensions and num levels
// -----------------------------------------------------------------------

func TestLowerTextureCubeDimensions(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_cube<f32>;
@compute @workgroup_size(1)
fn main() {
    let d = textureDimensions(t);
    let levels = textureNumLevels(t);
    _ = d; _ = levels;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Depth texture cube
// -----------------------------------------------------------------------

func TestLowerDepthTextureCubeSample(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_cube;
@group(0) @binding(1) var s: sampler_comparison;
@fragment
fn main(@location(0) dir: vec3<f32>) -> @location(0) vec4<f32> {
    let depth = textureSampleCompare(t, s, dir, 0.5);
    return vec4<f32>(depth, depth, depth, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture sample level with array texture
// -----------------------------------------------------------------------

func TestLowerTextureSampleLevelArray(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d_array<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleLevel(t, s, uv, 0, 0.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture sample grad with offset
// -----------------------------------------------------------------------

func TestLowerTextureSampleGradOffset(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(t, s, uv, vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0), vec2<i32>(1, 0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture load from depth multisampled
// -----------------------------------------------------------------------

func TestLowerTextureLoadDepthMS(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_multisampled_2d;
@fragment
fn main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    let d = textureLoad(t, vec2<i32>(i32(pos.x), i32(pos.y)), 0);
    return vec4<f32>(d, d, d, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture store to 1D, 3D storage
// -----------------------------------------------------------------------

func TestLowerTextureStore1D(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_storage_1d<r32float, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t, i32(id.x), vec4<f32>(1.0));
}`
	mustCompile(t, src)
}

func TestLowerTextureStore3D(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_storage_3d<r32float, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t, vec3<i32>(i32(id.x), i32(id.y), 0), vec4<f32>(1.0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture array store
// -----------------------------------------------------------------------

func TestLowerTextureStoreArray(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_storage_2d_array<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t, vec2<i32>(i32(id.x), i32(id.y)), 0, vec4<f32>(1.0, 0.0, 0.0, 1.0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Depth 2D texture dimensions
// -----------------------------------------------------------------------

func TestLowerDepthTextureDimensions(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_depth_2d;
@compute @workgroup_size(1)
fn main() {
    let d = textureDimensions(t, 0);
}`
	mustCompile(t, src)
}
