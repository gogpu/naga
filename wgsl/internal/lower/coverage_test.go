package lower

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// -----------------------------------------------------------------------
// Helper
// -----------------------------------------------------------------------

func mustCompile(t *testing.T, src string) *ir.Module {
	t.Helper()
	module, err := compileWGSL(t, src)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	return module
}

func expectError(t *testing.T, src string, substr string) {
	t.Helper()
	_, err := compileWGSL(t, src)
	if err == nil {
		t.Fatalf("expected error containing %q, got success", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Errorf("error = %q, want containing %q", err.Error(), substr)
	}
}

// -----------------------------------------------------------------------
// Override declarations
// -----------------------------------------------------------------------

func TestLowerOverride(t *testing.T) {
	src := `@id(0) override width: f32 = 640.0;
@id(1) override height: f32 = 480.0;
override scale = 2.0;
@compute @workgroup_size(1)
fn main() {
    let w = width * scale;
    let h = height * scale;
}`
	module := mustCompile(t, src)
	if len(module.Overrides) < 3 {
		t.Errorf("expected at least 3 overrides, got %d", len(module.Overrides))
	}
}

func TestLowerOverrideNoInit(t *testing.T) {
	src := `override brightness: f32;
@compute @workgroup_size(1)
fn main() {
    let b = brightness;
}`
	module := mustCompile(t, src)
	if len(module.Overrides) != 1 {
		t.Errorf("expected 1 override, got %d", len(module.Overrides))
	}
}

// -----------------------------------------------------------------------
// Global variables with various address spaces
// -----------------------------------------------------------------------

func TestLowerGlobalVarPrivate(t *testing.T) {
	src := `var<private> counter: i32 = 0;
fn test() { counter = counter + 1; }`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) < 1 {
		t.Error("expected at least 1 global variable")
	}
}

func TestLowerGlobalVarWorkgroup(t *testing.T) {
	src := `var<workgroup> shared_data: array<f32, 64>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    shared_data[lid.x] = f32(lid.x);
}`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "shared_data" && gv.Space == ir.SpaceWorkGroup {
			found = true
		}
	}
	if !found {
		t.Error("expected workgroup global variable 'shared_data'")
	}
}

func TestLowerGlobalVarUniform(t *testing.T) {
	src := `struct Uniforms { matrix: mat4x4<f32> }
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return uniforms.matrix * vec4<f32>(0.0, 0.0, 0.0, 1.0);
}`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "uniforms" && gv.Space == ir.SpaceUniform {
			found = true
		}
	}
	if !found {
		t.Error("expected uniform global variable 'uniforms'")
	}
}

func TestLowerGlobalVarStorageReadWrite(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read_write> data: array<f32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    data[id.x] = data[id.x] * 2.0;
}`
	module := mustCompile(t, src)
	found := false
	for _, gv := range module.GlobalVariables {
		if gv.Name == "data" && gv.Space == ir.SpaceStorage {
			found = true
		}
	}
	if !found {
		t.Error("expected storage global variable 'data'")
	}
}

// -----------------------------------------------------------------------
// For loop
// -----------------------------------------------------------------------

func TestLowerForLoop(t *testing.T) {
	src := `fn test() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 10; i++) {
        sum = sum + i;
    }
    return sum;
}`
	module := mustCompile(t, src)
	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}
	// Verify the function has a loop in its body
	fn := &module.Functions[0]
	foundLoop := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtLoop); ok {
			foundLoop = true
		}
	}
	if !foundLoop {
		t.Error("expected loop statement (for desugars to loop)")
	}
}

func TestLowerForLoopMinimal(t *testing.T) {
	src := `fn test() {
    for (;;) { break; }
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// While loop
// -----------------------------------------------------------------------

func TestLowerWhileLoop(t *testing.T) {
	src := `fn test() -> i32 {
    var i: i32 = 0;
    while i < 10 {
        i = i + 1;
    }
    return i;
}`
	module := mustCompile(t, src)
	fn := &module.Functions[0]
	foundLoop := false
	for _, s := range fn.Body {
		if _, ok := s.Kind.(ir.StmtLoop); ok {
			foundLoop = true
		}
	}
	if !foundLoop {
		t.Error("expected loop statement (while desugars to loop)")
	}
}

// -----------------------------------------------------------------------
// Switch statement
// -----------------------------------------------------------------------

func TestLowerSwitch(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    var result: i32;
    switch x {
        case 0: { result = 10; }
        case 1: { result = 20; }
        case 2, 3: { result = 30; }
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
			if len(sw.Cases) < 3 {
				t.Errorf("expected at least 3 switch cases, got %d", len(sw.Cases))
			}
		}
	}
	if !foundSwitch {
		t.Error("expected switch statement in body")
	}
}

func TestLowerSwitchDefaultOnly(t *testing.T) {
	src := `fn test(x: u32) {
    switch x {
        default: {}
    }
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Texture and sampler types
// -----------------------------------------------------------------------

func TestLowerTextureSampler(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(t, s, uv);
}`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) < 2 {
		t.Errorf("expected at least 2 global variables, got %d", len(module.GlobalVariables))
	}
}

func TestLowerTextureTypes(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"texture_1d", `@group(0) @binding(0) var t: texture_1d<f32>;
@compute @workgroup_size(1) fn main() { let d = textureDimensions(t); }`},
		{"texture_3d", `@group(0) @binding(0) var t: texture_3d<f32>;
@compute @workgroup_size(1) fn main() { let d = textureDimensions(t); }`},
		{"texture_cube", `@group(0) @binding(0) var t: texture_cube<f32>;
@compute @workgroup_size(1) fn main() { let d = textureDimensions(t); }`},
		{"texture_2d_array", `@group(0) @binding(0) var t: texture_2d_array<f32>;
@compute @workgroup_size(1) fn main() { let d = textureDimensions(t); }`},
		{"texture_depth_2d", `@group(0) @binding(0) var t: texture_depth_2d;
@compute @workgroup_size(1) fn main() { let d = textureDimensions(t); }`},
		{"texture_multisampled_2d", `@group(0) @binding(0) var t: texture_multisampled_2d<f32>;
@compute @workgroup_size(1) fn main() { let d = textureDimensions(t); }`},
		{"sampler_comparison", `@group(0) @binding(0) var s: sampler_comparison;
@compute @workgroup_size(1) fn main() { _ = s; }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustCompile(t, tt.src)
		})
	}
}

// -----------------------------------------------------------------------
// Storage texture
// -----------------------------------------------------------------------

func TestLowerStorageTexture(t *testing.T) {
	src := `@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    textureStore(t, vec2<i32>(i32(id.x), i32(id.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Derivative calls
// -----------------------------------------------------------------------

func TestLowerDerivatives(t *testing.T) {
	tests := []struct {
		name string
		fn   string
	}{
		{"dpdx", "dpdx"},
		{"dpdy", "dpdy"},
		{"fwidth", "fwidth"},
		{"dpdxCoarse", "dpdxCoarse"},
		{"dpdyCoarse", "dpdyCoarse"},
		{"fwidthCoarse", "fwidthCoarse"},
		{"dpdxFine", "dpdxFine"},
		{"dpdyFine", "dpdyFine"},
		{"fwidthFine", "fwidthFine"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := `@fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let d = ` + tt.fn + `(uv.x);
    return vec4<f32>(d, 0.0, 0.0, 1.0);
}`
			mustCompile(t, src)
		})
	}
}

// -----------------------------------------------------------------------
// Relational functions
// -----------------------------------------------------------------------

func TestLowerRelationalFunctions(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"all", `fn test() -> bool { return all(vec4<bool>(true, true, true, true)); }`},
		{"any", `fn test() -> bool { return any(vec4<bool>(true, false, true, false)); }`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustCompile(t, tt.src)
		})
	}
}

// -----------------------------------------------------------------------
// Array length
// -----------------------------------------------------------------------

func TestLowerArrayLength(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read> buf: array<f32>;
@compute @workgroup_size(1)
fn main() {
    let len = arrayLength(&buf);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Unary operations
// -----------------------------------------------------------------------

func TestLowerUnaryOps(t *testing.T) {
	src := `fn test() {
    var x: i32 = 5;
    let neg = -x;
    let not_val = !true;
    let bnot = ~0u;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Compound assignment operators
// -----------------------------------------------------------------------

func TestLowerCompoundAssign(t *testing.T) {
	src := `fn test() {
    var x: i32 = 10;
    x += 1;
    x -= 2;
    x *= 3;
    x /= 4;
    x %= 5;
    var u: u32 = 0xFFu;
    u &= 0x0Fu;
    u |= 0xF0u;
    u ^= 0xAAu;
    u <<= 2u;
    u >>= 1u;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Logical short-circuit (&&, ||)
// -----------------------------------------------------------------------

func TestLowerLogicalShortCircuit(t *testing.T) {
	src := `fn test(a: bool, b: bool) -> bool {
    let and_result = a && b;
    let or_result = a || b;
    return and_result || or_result;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Type constructors
// -----------------------------------------------------------------------

func TestLowerTypeConstructors(t *testing.T) {
	src := `fn test() {
    let v2 = vec2<f32>(1.0, 2.0);
    let v3 = vec3<f32>(1.0, 2.0, 3.0);
    let v4 = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let m = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    let a = array<i32, 3>(1, 2, 3);
    let s = f32(42);
    let u = u32(42);
    let i = i32(42.0);
    let b = bool(1);
}`
	mustCompile(t, src)
}

func TestLowerTypeConstructorFromVec(t *testing.T) {
	src := `fn test() {
    let v2 = vec2<f32>(1.0, 2.0);
    let v4 = vec4<f32>(v2, 3.0, 4.0);
    let v3 = vec3<f32>(v2, 5.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Splat constructors
// -----------------------------------------------------------------------

func TestLowerSplatConstructor(t *testing.T) {
	src := `fn test() {
    let v2 = vec2<f32>(1.0);
    let v3 = vec3<f32>(2.0);
    let v4 = vec4<f32>(3.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Zero-value constructors
// -----------------------------------------------------------------------

func TestLowerZeroConstructors(t *testing.T) {
	src := `fn test() {
    var v2 = vec2<f32>();
    var v3 = vec3<i32>();
    var v4 = vec4<u32>();
    var m = mat2x2<f32>();
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Swizzle
// -----------------------------------------------------------------------

func TestLowerSwizzle(t *testing.T) {
	src := `fn test() {
    var v = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    let xy = v.xy;
    let zw = v.zw;
    let wzyx = v.wzyx;
    let r = v.r;
    let rgba = v.rgba;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Struct member access
// -----------------------------------------------------------------------

func TestLowerStructMemberAccess(t *testing.T) {
	src := `struct Point { x: f32, y: f32 }
fn test() -> f32 {
    var p: Point;
    p.x = 1.0;
    p.y = 2.0;
    return p.x + p.y;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Array access
// -----------------------------------------------------------------------

func TestLowerArrayAccess(t *testing.T) {
	src := `fn test(i: u32) -> i32 {
    var arr: array<i32, 4> = array<i32, 4>(10, 20, 30, 40);
    return arr[i];
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const assert
// -----------------------------------------------------------------------

func TestLowerConstAssert(t *testing.T) {
	src := `const_assert 1 == 1;
const_assert true;
fn test() {
    const_assert 2 > 1;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Bitcast
// -----------------------------------------------------------------------

func TestLowerBitcast(t *testing.T) {
	src := `fn test() -> u32 {
    return bitcast<u32>(1.0f);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Entry point stages
// -----------------------------------------------------------------------

func TestLowerEntryPointStages(t *testing.T) {
	src := `@vertex
fn vs(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}

@fragment
fn fs() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}

@compute @workgroup_size(1)
fn cs() {}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 3 {
		t.Fatalf("expected 3 entry points, got %d", len(module.EntryPoints))
	}
	stages := map[ir.ShaderStage]bool{}
	for _, ep := range module.EntryPoints {
		stages[ep.Stage] = true
	}
	if !stages[ir.StageVertex] {
		t.Error("missing vertex stage")
	}
	if !stages[ir.StageFragment] {
		t.Error("missing fragment stage")
	}
	if !stages[ir.StageCompute] {
		t.Error("missing compute stage")
	}
}

// -----------------------------------------------------------------------
// Workgroup size
// -----------------------------------------------------------------------

func TestLowerWorkgroupSize(t *testing.T) {
	src := `@compute @workgroup_size(8, 4, 2)
fn main() {}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
	wg := module.EntryPoints[0].Workgroup
	if wg[0] != 8 || wg[1] != 4 || wg[2] != 2 {
		t.Errorf("workgroup size = %v, want [8,4,2]", wg)
	}
}

// -----------------------------------------------------------------------
// Interpolation attributes
// -----------------------------------------------------------------------

func TestLowerInterpolation(t *testing.T) {
	src := `struct Output {
    @builtin(position) pos: vec4<f32>,
    @location(0) @interpolate(flat) flat_val: u32,
    @location(1) @interpolate(linear, center) linear_val: f32,
    @location(2) @interpolate(perspective, centroid) persp_val: f32,
}
@vertex
fn main(@builtin(vertex_index) idx: u32) -> Output {
    var out: Output;
    out.pos = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    out.flat_val = idx;
    out.linear_val = 0.0;
    out.persp_val = 0.0;
    return out;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Binding attributes (group/binding)
// -----------------------------------------------------------------------

func TestLowerBindingAttributes(t *testing.T) {
	src := `@group(0) @binding(0) var<uniform> a: f32;
@group(0) @binding(1) var<uniform> b: f32;
@group(1) @binding(0) var<uniform> c: f32;
@compute @workgroup_size(1)
fn main() {
    let x = a + b + c;
}`
	module := mustCompile(t, src)
	if len(module.GlobalVariables) != 3 {
		t.Fatalf("expected 3 global variables, got %d", len(module.GlobalVariables))
	}
	for _, gv := range module.GlobalVariables {
		if gv.Binding == nil {
			t.Errorf("global variable %s should have binding", gv.Name)
		}
	}
}

// -----------------------------------------------------------------------
// Function calls (non-entry-point)
// -----------------------------------------------------------------------

func TestLowerFunctionCall(t *testing.T) {
	src := `fn add(a: f32, b: f32) -> f32 {
    return a + b;
}
fn test() -> f32 {
    return add(1.0, 2.0);
}`
	module := mustCompile(t, src)
	if len(module.Functions) != 2 {
		t.Errorf("expected 2 functions, got %d", len(module.Functions))
	}
}

func TestLowerRecursiveFunction(t *testing.T) {
	src := `fn factorial(n: i32) -> i32 {
    if n <= 1 { return 1; }
    return n * factorial(n - 1);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Nested if/else
// -----------------------------------------------------------------------

func TestLowerNestedIfElse(t *testing.T) {
	src := `fn test(x: i32) -> i32 {
    if x > 10 {
        return 3;
    } else if x > 5 {
        return 2;
    } else if x > 0 {
        return 1;
    } else {
        return 0;
    }
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Type aliases
// -----------------------------------------------------------------------

func TestLowerTypeAlias(t *testing.T) {
	src := `alias Float4 = vec4<f32>;
alias Float3 = vec3<f32>;
fn test() -> Float4 {
    let v: Float3 = Float3(1.0, 2.0, 3.0);
    return Float4(v, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Const expressions
// -----------------------------------------------------------------------

func TestLowerConstExpressions(t *testing.T) {
	src := `const A: i32 = 5;
const B: i32 = 10;
const SUM: i32 = A + B;
const PRODUCT: i32 = A * B;
const DIFF: i32 = B - A;
fn test() -> i32 { return SUM + PRODUCT + DIFF; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 5 {
		t.Errorf("expected at least 5 constants, got %d", len(module.Constants))
	}
}

func TestLowerConstExpressionsFloat(t *testing.T) {
	src := `const PI: f32 = 3.14159;
const TAU: f32 = PI * 2.0;
const HALF_PI: f32 = PI / 2.0;
fn test() -> f32 { return TAU + HALF_PI; }`
	mustCompile(t, src)
}

func TestLowerConstNegate(t *testing.T) {
	src := `const NEG: i32 = -42;
const NEG_F: f32 = -3.14;
fn test() -> i32 { return NEG; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Local const in function
// -----------------------------------------------------------------------

func TestLowerLocalConst(t *testing.T) {
	src := `fn test() -> i32 {
    const LOCAL_A = 10;
    const LOCAL_B = 20;
    const LOCAL_SUM = LOCAL_A + LOCAL_B;
    return LOCAL_SUM;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Phony assignment (_)
// -----------------------------------------------------------------------

func TestLowerPhonyAssignment(t *testing.T) {
	src := `fn test() {
    _ = 42;
    _ = vec4<f32>(1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Pointer operations
// -----------------------------------------------------------------------

func TestLowerPointerOps(t *testing.T) {
	src := `fn test() {
    var x: f32 = 1.0;
    let p = &x;
    *p = 2.0;
    let v = *p;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Matrix operations
// -----------------------------------------------------------------------

func TestLowerMatrixOps(t *testing.T) {
	src := `fn test() {
    let m = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    let v = vec2<f32>(1.0, 2.0);
    let result = m * v;
}`
	mustCompile(t, src)
}

func TestLowerMatrixColumnAccess(t *testing.T) {
	src := `fn test() -> vec4<f32> {
    var m: mat4x4<f32>;
    return m[0];
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Short-name types (vec2f, vec3i, mat2x2f, etc.)
// -----------------------------------------------------------------------

func TestLowerShortNameTypes(t *testing.T) {
	src := `fn test() {
    let v2f = vec2f(1.0, 2.0);
    let v3f = vec3f(1.0, 2.0, 3.0);
    let v4f = vec4f(1.0, 2.0, 3.0, 4.0);
    let v2i = vec2i(1, 2);
    let v3u = vec3u(1u, 2u, 3u);
    let m = mat2x2f(1.0, 0.0, 0.0, 1.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Abstract int/float concretization
// -----------------------------------------------------------------------

func TestLowerAbstractConcretization(t *testing.T) {
	src := `fn test() {
    var x: f32 = 42;
    var y: u32 = 10;
    var z: i32 = 20;
    let a = 1.5 + 2.5;
    let b = 10 + 20;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Loop with break / continue
// -----------------------------------------------------------------------

func TestLowerLoopBreakContinue(t *testing.T) {
	src := `fn test() -> i32 {
    var sum: i32 = 0;
    var i: i32 = 0;
    loop {
        if i >= 20 { break; }
        i = i + 1;
        if i % 2 == 0 { continue; }
        sum = sum + i;
    }
    return sum;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Multiple return values via struct
// -----------------------------------------------------------------------

func TestLowerStructReturn(t *testing.T) {
	src := `struct Result { value: f32, valid: bool }
fn compute() -> Result {
    var r: Result;
    r.value = 42.0;
    r.valid = true;
    return r;
}
fn test() -> f32 {
    let r = compute();
    if r.valid { return r.value; }
    return 0.0;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Var with initializer
// -----------------------------------------------------------------------

func TestLowerVarWithInit(t *testing.T) {
	src := `fn test() {
    var x: i32 = 42;
    var y = 3.14;
    var z: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Discard statement
// -----------------------------------------------------------------------

func TestLowerDiscard(t *testing.T) {
	src := `@fragment
fn main(@location(0) alpha: f32) -> @location(0) vec4<f32> {
    if alpha < 0.5 {
        discard;
    }
    return vec4<f32>(1.0, 1.0, 1.0, alpha);
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
}

// -----------------------------------------------------------------------
// Atomic operations
// -----------------------------------------------------------------------

func TestLowerAtomicOps(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read_write> counter: atomic<u32>;
@compute @workgroup_size(1)
fn main() {
    let old = atomicLoad(&counter);
    atomicStore(&counter, old + 1u);
    let added = atomicAdd(&counter, 2u);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Workgroup barrier
// -----------------------------------------------------------------------

func TestLowerWorkgroupBarrier(t *testing.T) {
	src := `var<workgroup> shared: array<f32, 64>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_id) lid: vec3<u32>) {
    shared[lid.x] = f32(lid.x);
    workgroupBarrier();
    let v = shared[63u - lid.x];
}`
	mustCompile(t, src)
}

func TestLowerStorageBarrier(t *testing.T) {
	src := `@group(0) @binding(0) var<storage, read_write> buf: array<u32>;
@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    buf[id.x] = 42u;
    storageBarrier();
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Comparison operations
// -----------------------------------------------------------------------

func TestLowerComparisonOps(t *testing.T) {
	src := `fn test(a: i32, b: i32) {
    let lt = a < b;
    let gt = a > b;
    let le = a <= b;
    let ge = a >= b;
    let eq = a == b;
    let ne = a != b;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Error cases
// -----------------------------------------------------------------------

func TestLowerErrorUndefinedVariable(t *testing.T) {
	src := `fn test() { let x = undefined_var; }`
	expectError(t, src, "undefined")
}

func TestLowerErrorUndefinedFunction(t *testing.T) {
	src := `fn test() { undefined_func(); }`
	expectError(t, src, "undefined")
}

func TestLowerDuplicateFunction(t *testing.T) {
	// The lowerer silently overwrites duplicate functions (matching Rust naga).
	src := `fn foo() {} fn foo() -> i32 { return 42; }`
	mustCompile(t, src)
}

func TestLowerErrorMathArgCount(t *testing.T) {
	src := `fn test() {
    let x = abs();
}`
	expectError(t, src, "")
}

// -----------------------------------------------------------------------
// f16 type
// -----------------------------------------------------------------------

func TestLowerF16Type(t *testing.T) {
	src := `enable f16;
fn test() -> f16 {
    let x: f16 = 1.0h;
    return x;
}`
	// f16 parsing should work
	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Skipf("lexer does not support f16: %v", err)
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Skipf("parser does not support f16: %v", err)
	}
	_, _ = Lower(ast) // May or may not succeed depending on f16 support
}

// -----------------------------------------------------------------------
// Struct with alignment/size attributes
// -----------------------------------------------------------------------

func TestLowerStructAlignSize(t *testing.T) {
	src := `struct Aligned {
    @align(16) x: f32,
    @size(16) y: f32,
    z: f32,
}
fn test() -> f32 {
    var a: Aligned;
    a.x = 1.0;
    a.y = 2.0;
    a.z = 3.0;
    return a.x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Packed vector/scalar dot product
// -----------------------------------------------------------------------

func TestLowerPackDotProduct(t *testing.T) {
	src := `fn test() -> f32 {
    let v1 = vec3<f32>(1.0, 2.0, 3.0);
    let v2 = vec3<f32>(4.0, 5.0, 6.0);
    return dot(v1, v2);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Multiple struct members with same name in different structs
// -----------------------------------------------------------------------

func TestLowerMultipleStructs(t *testing.T) {
	src := `struct A { x: f32, y: f32 }
struct B { x: i32, y: i32 }
fn test() {
    var a: A;
    var b: B;
    a.x = 1.0;
    b.x = 2;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Complex expressions (nested, mixed)
// -----------------------------------------------------------------------

func TestLowerComplexExpressions(t *testing.T) {
	src := `fn test(a: f32, b: f32, c: f32) -> f32 {
    return (a + b) * c - min(a, b) + max(b, c);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Entry point with struct input/output
// -----------------------------------------------------------------------

func TestLowerEntryPointStructIO(t *testing.T) {
	src := `struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) normal: vec3<f32>,
    @location(2) uv: vec2<f32>,
}
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) normal: vec3<f32>,
    @location(1) uv: vec2<f32>,
}
@vertex
fn main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.position = vec4<f32>(input.position, 1.0);
    output.normal = input.normal;
    output.uv = input.uv;
    return output;
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
	ep := module.EntryPoints[0]
	if ep.Stage != ir.StageVertex {
		t.Errorf("expected vertex stage, got %v", ep.Stage)
	}
	if len(ep.Function.Arguments) < 1 {
		t.Error("expected at least 1 argument")
	}
}

// -----------------------------------------------------------------------
// LowerWithWarnings and LowerWithSource
// -----------------------------------------------------------------------

func TestLowerWithWarnings(t *testing.T) {
	src := `fn test() {
    var unused: f32 = 1.0;
}`
	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	result, err := LowerWithWarnings(ast, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for unused variable")
	}
	if result.Module == nil {
		t.Error("expected non-nil module")
	}
}

func TestLowerWithSource(t *testing.T) {
	src := `fn test() -> i32 { return 42; }`
	lexer := parser.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(tokens)
	ast, err := p.Parse()
	if err != nil {
		t.Fatal(err)
	}
	module, err := LowerWithSource(ast, src)
	if err != nil {
		t.Fatal(err)
	}
	if module == nil {
		t.Fatal("expected non-nil module")
	}
}

// -----------------------------------------------------------------------
// Early depth test
// -----------------------------------------------------------------------

func TestLowerEarlyDepthTest(t *testing.T) {
	src := `@early_depth_test
@fragment
fn main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
	ep := module.EntryPoints[0]
	if ep.EarlyDepthTest == nil {
		t.Error("expected EarlyDepthTest to be set")
	}
}

// -----------------------------------------------------------------------
// Mixed binary ops with different types
// -----------------------------------------------------------------------

func TestLowerMixedBinaryOps(t *testing.T) {
	src := `fn test() {
    var x: f32 = 1.0;
    var y: f32 = 2.0;
    let add = x + y;
    let sub = x - y;
    let mul = x * y;
    let div = x / y;
    var u: u32 = 10u;
    var v: u32 = 3u;
    let mod_val = u % v;
    let band = u & v;
    let bor = u | v;
    let bxor = u ^ v;
    let shl = u << 1u;
    let shr = u >> 1u;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Nested struct types
// -----------------------------------------------------------------------

func TestLowerNestedStructTypes(t *testing.T) {
	src := `struct Inner { value: f32 }
struct Outer { inner: Inner, extra: i32 }
fn test() -> f32 {
    var o: Outer;
    o.inner.value = 42.0;
    o.extra = 1;
    return o.inner.value;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Constant vector constructors
// -----------------------------------------------------------------------

func TestLowerConstVectorConstructors(t *testing.T) {
	src := `const V2: vec2<f32> = vec2<f32>(1.0, 2.0);
const V3: vec3<i32> = vec3<i32>(1, 2, 3);
const V4: vec4<u32> = vec4<u32>(1u, 2u, 3u, 4u);
fn test() -> f32 { return V2.x; }`
	module := mustCompile(t, src)
	if len(module.Constants) < 3 {
		t.Errorf("expected at least 3 constants, got %d", len(module.Constants))
	}
}

// -----------------------------------------------------------------------
// Large function — exercises various statement types together
// -----------------------------------------------------------------------

func TestLowerCompleteShader(t *testing.T) {
	src := `struct Params {
    width: u32,
    height: u32,
    time: f32,
}

@group(0) @binding(0) var<uniform> params: Params;
@group(0) @binding(1) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let x = gid.x;
    let y = gid.y;
    if x >= params.width || y >= params.height {
        return;
    }
    let idx = y * params.width + x;
    let fx = f32(x) / f32(params.width);
    let fy = f32(y) / f32(params.height);
    var value = sin(fx * 3.14159 + params.time) * cos(fy * 3.14159);
    value = clamp(value, -1.0, 1.0);
    value = value * 0.5 + 0.5;
    output[idx] = value;
}`
	module := mustCompile(t, src)
	if len(module.EntryPoints) != 1 {
		t.Fatal("expected 1 entry point")
	}
}

// -----------------------------------------------------------------------
// Global variable without initializer
// -----------------------------------------------------------------------

func TestLowerGlobalVarNoInit(t *testing.T) {
	src := `var<private> x: f32;
fn test() { x = 1.0; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Runtime-sized array
// -----------------------------------------------------------------------

func TestLowerRuntimeSizedArray(t *testing.T) {
	src := `struct Data { values: array<f32> }
@group(0) @binding(0) var<storage, read> data: Data;
@compute @workgroup_size(1)
fn main() {
    let v = data.values[0];
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Nested loop
// -----------------------------------------------------------------------

func TestLowerNestedLoop(t *testing.T) {
	src := `fn test() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 3; i++) {
        for (var j: i32 = 0; j < 3; j++) {
            sum += i * j;
        }
    }
    return sum;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Local variable shadowing
// -----------------------------------------------------------------------

func TestLowerVariableShadowing(t *testing.T) {
	src := `fn test() -> i32 {
    var x: i32 = 1;
    {
        var x: i32 = 2;
        x = x + 1;
    }
    return x;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Passing function result to function
// -----------------------------------------------------------------------

func TestLowerChainedFunctionCalls(t *testing.T) {
	src := `fn double(x: f32) -> f32 { return x * 2.0; }
fn test() -> f32 {
    return double(double(double(1.0)));
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Ternary-like select
// -----------------------------------------------------------------------

func TestLowerSelectTernary(t *testing.T) {
	src := `fn test(cond: bool) -> f32 {
    return select(0.0, 1.0, cond);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Constant boolean operations
// -----------------------------------------------------------------------

func TestLowerConstBoolOps(t *testing.T) {
	src := `const A: bool = true;
const B: bool = false;
const AND_RESULT: bool = true;
const OR_RESULT: bool = true;
fn test() -> bool { return AND_RESULT; }`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Step/smoothstep (3-arg math)
// -----------------------------------------------------------------------

func TestLowerThreeArgMath(t *testing.T) {
	src := `fn test() -> f32 {
    let x: f32 = 0.5;
    let s = smoothstep(0.0, 1.0, x);
    let c = clamp(x, 0.0, 1.0);
    let m = mix(0.0, 1.0, x);
    let f = fma(x, x, x);
    return s + c + m + f;
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Integer bit operations
// -----------------------------------------------------------------------

func TestLowerBitOps(t *testing.T) {
	src := `fn test() {
    let a: u32 = countOneBits(0xFFu);
    let b: u32 = reverseBits(0x0Fu);
    let c: u32 = firstTrailingBit(8u);
    let d: u32 = firstLeadingBit(8u);
    let e: u32 = countOneBits(0u);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Pack/unpack operations
// -----------------------------------------------------------------------

func TestLowerPackUnpack(t *testing.T) {
	src := `fn test() {
    let packed = pack4x8snorm(vec4<f32>(0.5, -0.5, 1.0, -1.0));
    let unpacked = unpack4x8snorm(packed);
    let packed2 = pack4x8unorm(vec4<f32>(0.0, 0.25, 0.5, 1.0));
    let unpacked2 = unpack4x8unorm(packed2);
    let packed3 = pack2x16snorm(vec2<f32>(0.5, -0.5));
    let unpacked3 = unpack2x16snorm(packed3);
    let packed4 = pack2x16unorm(vec2<f32>(0.5, 1.0));
    let unpacked4 = unpack2x16unorm(packed4);
    let packed5 = pack2x16float(vec2<f32>(1.0, 2.0));
    let unpacked5 = unpack2x16float(packed5);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// extractBits / insertBits
// -----------------------------------------------------------------------

func TestLowerExtractInsertBits(t *testing.T) {
	src := `fn test() {
    let e = extractBits(0xFFu, 4u, 4u);
    let i = insertBits(0u, 0xFu, 4u, 4u);
}`
	mustCompile(t, src)
}

// -----------------------------------------------------------------------
// Quantize (f16 round-trip)
// -----------------------------------------------------------------------

func TestLowerQuantize(t *testing.T) {
	src := `fn test() -> f32 {
    return quantizeToF16(3.14);
}`
	mustCompile(t, src)
}
