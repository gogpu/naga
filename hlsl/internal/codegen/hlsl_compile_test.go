// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// =============================================================================
// Helpers
// =============================================================================

// compileWGSLToHLSL parses WGSL source and compiles to HLSL.
func compileWGSLToHLSL(t *testing.T, src string, opts *Options) string {
	t.Helper()
	module := parseWGSL(t, src)
	if opts == nil {
		opts = DefaultOptions()
		opts.FakeMissingBindings = true
	}
	code, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	return code
}

// parseWGSL parses WGSL source and lowers to IR.
func parseWGSL(t *testing.T, src string) *ir.Module {
	t.Helper()
	lexer := wgsl.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize failed: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	module, err := wgsl.LowerWithSource(ast, src)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}
	return module
}

// mustContain checks that code contains all expected substrings.
func mustContain(t *testing.T, code string, expects []string) {
	t.Helper()
	for _, s := range expects {
		if !strings.Contains(code, s) {
			t.Errorf("expected output to contain %q\n--- output ---\n%s", s, code)
		}
	}
}

// mustNotContain checks that code does not contain any of the listed substrings.
func mustNotContain(t *testing.T, code string, rejects []string) {
	t.Helper()
	for _, s := range rejects {
		if strings.Contains(code, s) {
			t.Errorf("expected output NOT to contain %q\n--- output ---\n%s", s, code)
		}
	}
}

// =============================================================================
// Entry Point Tests — covers writeEntryPoints, writeFunction, writeEntryPointWithIO
// =============================================================================

func TestCompile_VertexEntryPoint(t *testing.T) {
	src := `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    let x = f32(idx);
    return vec4<f32>(x, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"vs_main",
		"SV_VertexID",
		"SV_Position",
		"float4",
	})
}

func TestCompile_FragmentEntryPoint(t *testing.T) {
	src := `
@fragment
fn fs_main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return pos;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"fs_main",
		"SV_Position",
		"SV_Target0",
	})
}

func TestCompile_ComputeEntryPoint(t *testing.T) {
	src := `
@compute @workgroup_size(64)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    _ = gid;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"[numthreads(64, 1, 1)]",
		"cs_main",
		"SV_DispatchThreadID",
	})
}

// =============================================================================
// Statement Tests — covers writeIfStatement, writeSwitchStatement,
// writeLoopStatement, writeBreakStatement, writeContinueStatement,
// writeReturnStatement, writeKillStatement, writeBarrierStatement,
// writeStoreStatement, writeEmitStatement
// =============================================================================

func TestCompile_IfElseStatement(t *testing.T) {
	src := `
fn test_if(x: i32) -> i32 {
    if x > 0 {
        return 1;
    } else {
        return -1;
    }
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"if (",
		"} else {",
		"return",
	})
}

func TestCompile_SwitchStatement(t *testing.T) {
	src := `
fn test_switch(x: i32) -> i32 {
    var result: i32 = 0;
    switch x {
        case 0: {
            result = 10;
        }
        case 1: {
            result = 20;
        }
        default: {
            result = 30;
        }
    }
    return result;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"switch(",
		"case 0:",
		"case 1:",
		"default:",
		"break;",
	})
}

func TestCompile_ForLoopStatement(t *testing.T) {
	src := `
fn test_loop() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 10; i++) {
        sum = sum + i;
    }
    return sum;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"while(true)",
		"break;",
	})
}

func TestCompile_DiscardStatement(t *testing.T) {
	src := `
@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    if color.a < 0.5 {
        discard;
    }
    return color;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"discard;",
	})
}

func TestCompile_BarrierStatement(t *testing.T) {
	src := `
var<workgroup> shared_data: array<u32, 64>;

@compute @workgroup_size(64)
fn cs_main(@builtin(local_invocation_id) lid: vec3<u32>) {
    shared_data[lid.x] = lid.x;
    workgroupBarrier();
    _ = shared_data[0];
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"GroupMemoryBarrierWithGroupSync()",
		"groupshared",
	})
}

// =============================================================================
// Expression Tests — covers writeUnaryExpression, writeBinaryExpression,
// writeSelectExpression, writeRelationalExpression, writeMathExpression,
// writeSplatExpression, writeSwizzleExpression, writeAccessExpression,
// writeAccessIndexExpression, writeLoadExpression, writeLocalVariableExpression
// =============================================================================

func TestCompile_UnaryExpressions(t *testing.T) {
	src := `
fn test_unary(x: f32, b: bool, u: u32) -> f32 {
    let neg = -x;
    let not_b = !b;
    let bw_not = ~u;
    _ = not_b;
    _ = bw_not;
    return neg;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"-(", // negate
		"!(", // logical not
		"~(", // bitwise not
	})
}

func TestCompile_BinaryExpressions(t *testing.T) {
	src := `
fn test_binary(a: f32, b: f32) -> f32 {
    let sum = a + b;
    let diff = a - b;
    let prod = a * b;
    let quot = a / b;
    _ = diff;
    _ = prod;
    _ = quot;
    return sum;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"+",
		"-",
		"*",
		"/",
	})
}

func TestCompile_ComparisonExpressions(t *testing.T) {
	src := `
fn test_cmp(a: i32, b: i32) -> bool {
    let eq = a == b;
    let ne = a != b;
    let lt = a < b;
    let gt = a > b;
    _ = ne;
    _ = lt;
    _ = gt;
    return eq;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"==",
		"!=",
		"<",
		">",
	})
}

func TestCompile_LogicalExpressions(t *testing.T) {
	// WGSL short-circuit &&/|| is lowered to if/else in IR
	src := `
fn test_logical(a: bool, b: bool) -> bool {
    return a && b || !a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Short-circuit logic produces if/else pattern, not && / ||
	mustContain(t, code, []string{
		"if (",
		"!(a)",
	})
}

func TestCompile_BitwiseExpressions(t *testing.T) {
	src := `
fn test_bitwise(a: u32, b: u32) -> u32 {
    let and_r = a & b;
    let or_r = a | b;
    let xor_r = a ^ b;
    let shl = a << 2u;
    let shr = a >> 1u;
    _ = or_r;
    _ = xor_r;
    _ = shl;
    _ = shr;
    return and_r;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"&",
		"|",
		"^",
		"<<",
		">>",
	})
}

func TestCompile_SelectExpression(t *testing.T) {
	src := `
fn test_select(a: f32, b: f32, c: bool) -> f32 {
    return select(a, b, c);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"?",
		":",
	})
}

func TestCompile_SwizzleExpression(t *testing.T) {
	src := `
fn test_swizzle(v: vec4<f32>) -> vec2<f32> {
    return v.xy;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		".xy",
	})
}

func TestCompile_SplatExpression(t *testing.T) {
	src := `
fn test_splat(x: f32) -> vec3<f32> {
    return vec3<f32>(x);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Splat: float3(x, x, x) or (x).xxx
	if !strings.Contains(code, ".xxx") && !strings.Contains(code, "float3(") {
		t.Errorf("expected splat pattern, got:\n%s", code)
	}
}

func TestCompile_ArrayAccess(t *testing.T) {
	src := `
fn test_array_access(idx: i32) -> f32 {
    var arr: array<f32, 4> = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    return arr[idx];
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"[", // array access
	})
}

func TestCompile_StructAccess(t *testing.T) {
	src := `
struct MyData {
    x: f32,
    y: f32,
}

fn test_struct() -> f32 {
    var d: MyData;
    d.x = 1.0;
    d.y = 2.0;
    return d.x + d.y;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"struct MyData",
		".x",
		".y",
	})
}

// =============================================================================
// Math Function Tests — covers writeMathExpression, mathFunctionToHLSL
// =============================================================================

func TestCompile_MathFunctions(t *testing.T) {
	src := `
fn test_math(a: f32, b: f32) -> f32 {
    let v1 = abs(a);
    let v2 = min(a, b);
    let v3 = max(a, b);
    let v4 = clamp(a, 0.0, 1.0);
    let v5 = floor(a);
    let v6 = ceil(a);
    let v7 = sqrt(a);
    let v8 = sin(a);
    let v9 = cos(a);
    _ = v2; _ = v3; _ = v4; _ = v5;
    _ = v6; _ = v7; _ = v8; _ = v9;
    return v1;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"abs(",
		"min(",
		"max(",
		"clamp(",
		"floor(",
		"ceil(",
		"sqrt(",
		"sin(",
		"cos(",
	})
}

func TestCompile_MathFunctionsAdvanced(t *testing.T) {
	src := `
fn test_math_adv(a: f32, b: f32) -> f32 {
    let v1 = pow(a, b);
    let v2 = exp(a);
    let v3 = log(a);
    let v4 = exp2(a);
    let v5 = log2(a);
    let v6 = round(a);
    let v7 = trunc(a);
    let v8 = sign(a);
    _ = v2; _ = v3; _ = v4; _ = v5;
    _ = v6; _ = v7; _ = v8;
    return v1;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"pow(",
		"exp(",
		"log(",
		"exp2(",
		"log2(",
		"round(",
		"trunc(",
		"sign(",
	})
}

func TestCompile_MathVectorFunctions(t *testing.T) {
	src := `
fn test_vec_math(a: vec3<f32>, b: vec3<f32>) -> f32 {
    let d = dot(a, b);
    let c = cross(a, b);
    let l = length(a);
    let n = normalize(a);
    let dist = distance(a, b);
    _ = c; _ = l; _ = n; _ = dist;
    return d;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"dot(",
		"cross(",
		"length(",
		"normalize(",
		"distance(",
	})
}

func TestCompile_MathMixStepSmoothstep(t *testing.T) {
	src := `
fn test_interp(a: f32, b: f32, t: f32) -> f32 {
    let m = mix(a, b, t);
    let s = step(a, b);
    let ss = smoothstep(0.0, 1.0, t);
    _ = s; _ = ss;
    return m;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"lerp(", // mix -> lerp in HLSL
		"step(",
		"smoothstep(",
	})
}

func TestCompile_MathTrigFunctions(t *testing.T) {
	src := `
fn test_trig(a: f32) -> f32 {
    let v1 = tan(a);
    let v2 = asin(a);
    let v3 = acos(a);
    let v4 = atan(a);
    let v5 = atan2(a, 1.0);
    let v6 = sinh(a);
    let v7 = cosh(a);
    let v8 = tanh(a);
    _ = v2; _ = v3; _ = v4; _ = v5;
    _ = v6; _ = v7; _ = v8;
    return v1;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"tan(",
		"asin(",
		"acos(",
		"atan(",
		"atan2(",
		"sinh(",
		"cosh(",
		"tanh(",
	})
}

func TestCompile_MathFmaSaturate(t *testing.T) {
	src := `
fn test_fma(a: f32, b: f32, c: f32) -> f32 {
    let v1 = fma(a, b, c);
    let v2 = saturate(a);
    _ = v2;
    return v1;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"mad(", // fma -> mad
		"saturate(",
	})
}

func TestCompile_RelationalExpressions(t *testing.T) {
	src := `
fn test_relational(v: vec4<bool>) -> bool {
    let a = all(v);
    let b = any(v);
    _ = b;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"all(",
		"any(",
	})
}

// TestCompile_RelationalIsNanIsInf is tested via unit tests in expressions_test.go.
// WGSL does not have direct isNan/isInf builtins; they come from IR-level
// ExprRelational nodes generated by certain shader patterns.

// =============================================================================
// Type Emission Tests — covers writeStructDefinition, typeToHLSL, getTypeName
// =============================================================================

func TestCompile_AllScalarTypes(t *testing.T) {
	src := `
fn test_scalars() {
    var a: f32 = 1.0;
    var b: i32 = 1;
    var c: u32 = 1u;
    var d: bool = true;
    _ = a; _ = b; _ = c; _ = d;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float",
		"int",
		"uint",
		"bool",
	})
}

func TestCompile_VectorTypes(t *testing.T) {
	src := `
fn test_vectors() {
    var v2: vec2<f32> = vec2<f32>(1.0, 2.0);
    var v3: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);
    var v4: vec4<f32> = vec4<f32>(1.0, 2.0, 3.0, 4.0);
    var iv4: vec4<i32> = vec4<i32>(1, 2, 3, 4);
    _ = v2; _ = v3; _ = v4; _ = iv4;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float2",
		"float3",
		"float4",
		"int4",
	})
}

func TestCompile_MatrixTypes(t *testing.T) {
	src := `
fn test_matrices() {
    var m2: mat2x2<f32> = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    var m3: mat3x3<f32> = mat3x3<f32>(1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0);
    var m4: mat4x4<f32> = mat4x4<f32>(1.0,0.0,0.0,0.0, 0.0,1.0,0.0,0.0, 0.0,0.0,1.0,0.0, 0.0,0.0,0.0,1.0);
    _ = m2; _ = m3; _ = m4;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float2x2",
		"float3x3",
		"float4x4",
	})
}

func TestCompile_StructTypes(t *testing.T) {
	src := `
struct Light {
    position: vec3<f32>,
    color: vec4<f32>,
    intensity: f32,
}

fn test_struct_type() -> f32 {
    var light: Light;
    light.intensity = 1.0;
    return light.intensity;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"struct Light",
		"float3",
		"float4",
		"float",
	})
}

func TestCompile_ArrayTypes(t *testing.T) {
	src := `
fn test_arrays() -> f32 {
    var arr: array<f32, 4> = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    return arr[0];
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float",
		"[4]",
	})
}

// =============================================================================
// Uniform / Storage Buffer Tests — covers writeGlobalVariableExpression,
// getBindTarget, writeConstantBuffer, writeByteAddressBuffer
// =============================================================================

func TestCompile_UniformBuffer(t *testing.T) {
	src := `
struct Uniforms {
    mvp: mat4x4<f32>,
    color: vec4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return uniforms.mvp * vec4<f32>(f32(idx), 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"cbuffer",    // constant buffer
		"register(b", // buffer register
		"uniforms",
	})
}

func TestCompile_StorageBuffer(t *testing.T) {
	src := `
struct Data {
    values: array<f32>,
}

@group(0) @binding(0) var<storage, read> data: Data;

@compute @workgroup_size(64)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    _ = data.values[gid.x];
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"ByteAddressBuffer",
		"register(t",
	})
}

func TestCompile_StorageBufferReadWrite(t *testing.T) {
	src := `
struct Data {
    values: array<u32>,
}

@group(0) @binding(0) var<storage, read_write> data: Data;

@compute @workgroup_size(64)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    data.values[gid.x] = gid.x;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"RWByteAddressBuffer",
		"register(u",
	})
}

// =============================================================================
// Cast / As Expression Tests — covers writeAsExpression, scalarKindToHLSL
// =============================================================================

func TestCompile_CastExpressions(t *testing.T) {
	src := `
fn test_casts(i: i32, u: u32, f: f32) -> f32 {
    let fi = f32(i);
    let fu = f32(u);
    let iu = i32(u);
    let ui = u32(i);
    _ = fu; _ = iu; _ = ui;
    return fi;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float(",
		"int(",
		"uint(",
	})
}

func TestCompile_BitcastExpressions(t *testing.T) {
	src := `
fn test_bitcast(u: u32) -> f32 {
    return bitcast<f32>(u);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"asfloat(",
	})
}

// =============================================================================
// I32 Wrapping Arithmetic Tests — covers writeBinaryExpression i32 path,
// binaryOpStr, isI32ScalarOp, isIntegerBinaryOp
// =============================================================================

func TestCompile_I32WrappingArithmetic(t *testing.T) {
	src := `
fn test_i32_wrap(a: i32, b: i32) -> i32 {
    return a + b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"asint(asuint(",
	})
}

// =============================================================================
// Matrix Multiply Tests — covers writeBinaryExpression matrix mul() path
// =============================================================================

func TestCompile_MatrixMultiply(t *testing.T) {
	src := `
fn test_mat_mul(m: mat4x4<f32>, v: vec4<f32>) -> vec4<f32> {
    return m * v;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"mul(",
	})
}

// =============================================================================
// Derivative Tests — covers writeDerivativeExpression
// =============================================================================

func TestCompile_Derivatives(t *testing.T) {
	src := `
@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dx = dpdx(uv.x);
    let dy = dpdy(uv.y);
    let fw = fwidth(uv.x);
    return vec4<f32>(dx, dy, fw, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"ddx(",
		"ddy(",
		"fwidth(",
	})
}

// =============================================================================
// Function Call Tests — covers writeCallStatement, writeCallResultExpression
// =============================================================================

func TestCompile_FunctionCall(t *testing.T) {
	src := `
fn helper(x: f32) -> f32 {
    return x * 2.0;
}

fn main_fn() -> f32 {
    let result = helper(3.0);
    return result;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"helper(",
	})
}

func TestCompile_FunctionCallWithMultipleArgs(t *testing.T) {
	src := `
fn add3(a: f32, b: f32, c: f32) -> f32 {
    return a + b + c;
}

fn main_fn() -> f32 {
    return add3(1.0, 2.0, 3.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Namer may append underscore to avoid keyword conflicts
	mustContain(t, code, []string{
		"add3_(",
	})
}

// =============================================================================
// Pointer Argument Tests — covers inout pattern for pointer args
// =============================================================================

func TestCompile_PointerArguments(t *testing.T) {
	src := `
fn increment(p: ptr<function, i32>) {
    *p = *p + 1;
}

fn main_fn() -> i32 {
    var x: i32 = 0;
    increment(&x);
    return x;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"inout",
	})
}

// =============================================================================
// Workgroup Variables Tests — covers workgroup init, zero initialization
// =============================================================================

func TestCompile_WorkgroupVariables(t *testing.T) {
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.ZeroInitializeWorkgroupMemory = true

	src := `
var<workgroup> counter: u32;

@compute @workgroup_size(64)
fn cs_main(@builtin(local_invocation_id) lid: vec3<u32>) {
    counter = lid.x;
}
`
	code := compileWGSLToHLSL(t, src, opts)
	mustContain(t, code, []string{
		"groupshared",
	})
}

// =============================================================================
// Constant Tests — covers writeConstantExpression, writeLiteralValue
// =============================================================================

func TestCompile_Constants(t *testing.T) {
	src := `
const PI: f32 = 3.14159;
const MAX: i32 = 100;
const FLAG: bool = true;

fn test_const() -> f32 {
    return PI;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"static const",
		"3.14159",
	})
}

// =============================================================================
// Special Float Literals Tests — covers formatSpecialFloat (Inf, NaN)
// =============================================================================

func TestCompile_SpecialFloatLiterals(t *testing.T) {
	// Test that special float literals don't crash; WGSL doesn't
	// have direct Inf/NaN literals, but we can test via constants
	src := `
fn test_floats() -> f32 {
    let a: f32 = 0.0;
    let b: f32 = 1.0;
    return a / b;
}
`
	// Just ensure compilation doesn't crash
	_ = compileWGSLToHLSL(t, src, nil)
}

// =============================================================================
// Sampler / Texture Tests — covers writeImageQueryExpression (partial)
// =============================================================================

func TestCompile_TextureSample(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"Texture2D",
		"SamplerState",
		".Sample(",
	})
}

func TestCompile_TextureDimensions(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;

fn get_dims() -> vec2<u32> {
    return textureDimensions(tex);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"GetDimensions(",
	})
}

// =============================================================================
// Store Statement Tests — covers writeStoreStatement
// =============================================================================

func TestCompile_LocalVarStore(t *testing.T) {
	src := `
fn test_store() -> i32 {
    var x: i32 = 0;
    x = 42;
    return x;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// i32 literal is wrapped: int(42)
	mustContain(t, code, []string{
		"int(42)",
	})
}

// =============================================================================
// Continue Forwarding Tests — covers continueCtx, writeContinueStatement
// inside switch within loop
// =============================================================================

func TestCompile_ContinueInSwitchInLoop(t *testing.T) {
	src := `
fn test_continue_forward(n: i32) -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < n; i++) {
        switch i {
            case 3: {
                continue;
            }
            default: {
                sum = sum + i;
            }
        }
    }
    return sum;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Continue forwarding produces a bool variable and if-check after switch
	mustContain(t, code, []string{
		"bool ",
		"= true;",
		"continue;",
	})
}

// =============================================================================
// Compose Expression Tests — covers writeComposeExpression
// =============================================================================

func TestCompile_ComposeExpression(t *testing.T) {
	src := `
fn test_compose() -> vec4<f32> {
    let a = 1.0;
    let b = 2.0;
    return vec4<f32>(a, b, 3.0, 4.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float4(",
	})
}

// =============================================================================
// Let / Named Expression Tests — covers writeEmittedExpression, bakeRefCount
// =============================================================================

func TestCompile_LetBindings(t *testing.T) {
	src := `
fn test_let(x: f32) -> f32 {
    let a = x * 2.0;
    let b = a + 1.0;
    return b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Let bindings should produce named temporaries
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Struct Constructor Tests — covers writeStructConstructors,
// writeSingleStructConstructor, scanForStructConstructors
// =============================================================================

func TestCompile_StructConstructor(t *testing.T) {
	src := `
struct Point {
    x: f32,
    y: f32,
}

fn make_point() -> Point {
    return Point(1.0, 2.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"Point",
	})
}

// =============================================================================
// Quantize Expression Tests — covers MathQuantizeF16 special path
// =============================================================================

func TestCompile_QuantizeToF16(t *testing.T) {
	src := `
fn test_quantize(x: f32) -> f32 {
    return quantizeToF16(x);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"f16tof32(",
		"f32tof16(",
	})
}

// =============================================================================
// CountOneBits / ReverseBits Tests — covers i32 overload wrapping
// =============================================================================

func TestCompile_BitManipulation(t *testing.T) {
	src := `
fn test_bits(x: u32) -> u32 {
    let a = countOneBits(x);
    let b = reverseBits(x);
    _ = b;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"countbits(",
		"reversebits(",
	})
}

func TestCompile_BitManipulationSigned(t *testing.T) {
	src := `
fn test_bits_signed(x: i32) -> i32 {
    let a = countOneBits(x);
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// For signed types: asint(countbits(asuint(x)))
	mustContain(t, code, []string{
		"asint(",
		"countbits(",
		"asuint(",
	})
}

// =============================================================================
// Pack/Unpack Tests — covers writePack2x16snorm/unorm/float,
// writeUnpack2x16snorm/unorm/float
// =============================================================================

func TestCompile_Pack2x16(t *testing.T) {
	src := `
fn test_pack(v: vec2<f32>) -> u32 {
    let a = pack2x16snorm(v);
    let b = pack2x16unorm(v);
    let c = pack2x16float(v);
    _ = b; _ = c;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// These produce inline HLSL polyfills
	if code == "" {
		t.Error("expected non-empty output")
	}
}

func TestCompile_Unpack2x16(t *testing.T) {
	src := `
fn test_unpack(x: u32) -> vec2<f32> {
    let a = unpack2x16snorm(x);
    let b = unpack2x16unorm(x);
    let c = unpack2x16float(x);
    _ = b; _ = c;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

func TestCompile_Pack4x8(t *testing.T) {
	src := `
fn test_pack4x8(v: vec4<f32>) -> u32 {
    let a = pack4x8snorm(v);
    let b = pack4x8unorm(v);
    _ = b;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

func TestCompile_Unpack4x8(t *testing.T) {
	src := `
fn test_unpack4x8(x: u32) -> vec4<f32> {
    let a = unpack4x8snorm(x);
    let b = unpack4x8unorm(x);
    _ = b;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// I32 Negate Helper Test — covers writeUnaryExpression i32 negate path
// =============================================================================

func TestCompile_I32Negate(t *testing.T) {
	src := `
fn test_neg(x: i32) -> i32 {
    return -x;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"naga_neg(",
	})
}

// =============================================================================
// Nested Struct with Binding Tests — covers writeEPInputStruct,
// writeEPOutputStruct, writeInterfaceStruct
// =============================================================================

func TestCompile_VertexInputOutputStructs(t *testing.T) {
	src := `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) color: vec4<f32>,
}

struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.clip_position = vec4<f32>(in.position, 1.0);
    out.color = in.color;
    return out;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Location semantics are LOC0, LOC1 in our implementation
	mustContain(t, code, []string{
		"struct",
		"SV_Position",
		"LOC0",
		"LOC1",
	})
}

// =============================================================================
// Shader Model Selection Tests — covers ShaderProfile
// =============================================================================

func TestCompile_ShaderModelOptions(t *testing.T) {
	src := `
@compute @workgroup_size(1)
fn cs_main() {}
`
	models := []ShaderModel{ShaderModel5_0, ShaderModel5_1, ShaderModel6_0, ShaderModel6_5}
	for _, sm := range models {
		t.Run(sm.String(), func(t *testing.T) {
			opts := DefaultOptions()
			opts.FakeMissingBindings = true
			opts.ShaderModel = sm
			code := compileWGSLToHLSL(t, src, opts)
			if code == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

// =============================================================================
// Uniform with Custom Binding Map — covers getBindTarget
// =============================================================================

func TestCompile_CustomBindingMap(t *testing.T) {
	src := `
struct Params {
    value: f32,
}

@group(0) @binding(0) var<uniform> params: Params;

fn use_params() -> f32 {
    return params.value;
}
`
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.BindingMap = map[ResourceBinding]BindTarget{
		{Group: 0, Binding: 0}: {Space: 2, Register: 3},
	}
	code := compileWGSLToHLSL(t, src, opts)
	mustContain(t, code, []string{
		"register(b3, space2)",
	})
}

// =============================================================================
// Edge Cases — ensure no panics
// =============================================================================

func TestCompile_EmptyFunction(t *testing.T) {
	src := `
fn empty() {
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"void empty()",
	})
}

func TestCompile_MultipleEntryPoints(t *testing.T) {
	src := `
@vertex
fn vs_main() -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}

@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"vs_main",
		"fs_main",
	})
}

func TestCompile_NestedControlFlow(t *testing.T) {
	src := `
fn nested(x: i32) -> i32 {
    var result: i32 = 0;
    if x > 0 {
        for (var i: i32 = 0; i < x; i++) {
            if i == 5 {
                break;
            }
            result = result + i;
        }
    } else {
        result = -1;
    }
    return result;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"if (",
		"while(true)",
		"break;",
	})
}

// =============================================================================
// Return struct from entry point — covers writeStructReturn
// =============================================================================

func TestCompile_StructReturnFromVertex(t *testing.T) {
	src := `
struct VSOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) color: vec3<f32>,
}

@vertex
fn vs_main() -> VSOutput {
    var out: VSOutput;
    out.pos = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    out.color = vec3<f32>(1.0, 0.0, 0.0);
    return out;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"return",
	})
}

// =============================================================================
// Texture Store Tests — covers writeImageStoreStatement
// =============================================================================

func TestCompile_TextureStore(t *testing.T) {
	src := `
@group(0) @binding(0) var output_tex: texture_storage_2d<rgba8unorm, write>;

@compute @workgroup_size(8, 8)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    textureStore(output_tex, vec2<i32>(i32(gid.x), i32(gid.y)), vec4<f32>(1.0, 0.0, 0.0, 1.0));
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"RWTexture2D",
	})
}

// =============================================================================
// Multiple Return Values Test — covers void return
// =============================================================================

func TestCompile_VoidReturn(t *testing.T) {
	src := `
fn do_nothing() {
    return;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"void do_nothing",
		"return;",
	})
}

// =============================================================================
// Matrix Transpose Test — covers MathTranspose
// =============================================================================

func TestCompile_MatrixTranspose(t *testing.T) {
	src := `
fn test_transpose(m: mat4x4<f32>) -> mat4x4<f32> {
    return transpose(m);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"transpose(",
	})
}

// =============================================================================
// Matrix Determinant Test — covers MathDeterminant
// =============================================================================

func TestCompile_MatrixDeterminant(t *testing.T) {
	src := `
fn test_det(m: mat4x4<f32>) -> f32 {
    return determinant(m);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"determinant(",
	})
}

// =============================================================================
// Degrees / Radians Tests
// =============================================================================

func TestCompile_DegreesRadians(t *testing.T) {
	src := `
fn test_angle(a: f32) -> f32 {
    let d = degrees(a);
    let r = radians(a);
    _ = r;
    return d;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"degrees(",
		"radians(",
	})
}

// =============================================================================
// Fract / FMA test
// =============================================================================

func TestCompile_FractLdexp(t *testing.T) {
	src := `
fn test_fract(a: f32) -> f32 {
    let v1 = fract(a);
    let v2 = ldexp(a, 2);
    _ = v2;
    return v1;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"frac(", // fract -> frac in HLSL
		"ldexp(",
	})
}

// =============================================================================
// InverseSqrt test
// =============================================================================

func TestCompile_InverseSqrt(t *testing.T) {
	src := `
fn test_rsqrt(a: f32) -> f32 {
    return inverseSqrt(a);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"rsqrt(",
	})
}

// =============================================================================
// Reflect / Refract / FaceForward tests
// =============================================================================

func TestCompile_GeometricFunctions(t *testing.T) {
	src := `
fn test_geom(v: vec3<f32>, n: vec3<f32>) -> vec3<f32> {
    let r = reflect(v, n);
    let ff = faceForward(n, v, n);
    let rf = refract(v, n, 1.5);
    _ = ff; _ = rf;
    return r;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"reflect(",
		"faceforward(",
		"refract(",
	})
}

// =============================================================================
// FirstLeadingBit / FirstTrailingBit Tests
// =============================================================================

func TestCompile_FirstBitFunctions(t *testing.T) {
	src := `
fn test_firstbit(x: u32) -> u32 {
    let a = firstLeadingBit(x);
    let b = firstTrailingBit(x);
    _ = b;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"firstbithigh(",
		"firstbitlow(",
	})
}

// =============================================================================
// extractBits / insertBits Tests
// =============================================================================

func TestCompile_ExtractInsertBits(t *testing.T) {
	src := `
fn test_bits_extract(x: u32) -> u32 {
    let a = extractBits(x, 0u, 8u);
    let b = insertBits(x, 0xFFu, 8u, 8u);
    _ = b;
    return a;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// extractBits and insertBits use helper functions
	if code == "" {
		t.Error("expected non-empty output")
	}
}
