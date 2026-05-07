// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

// Additional tests to achieve 80%+ coverage.
// Focuses on expression codegen paths, helper functions, and writer infrastructure.
package codegen

import (
	"math"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// wgslToHLSL compiles WGSL to HLSL for coverage tests.
func wgslToHLSL(t *testing.T, src string) string {
	t.Helper()
	lexer := wgsl.NewLexer(src)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	module, err := wgsl.LowerWithSource(ast, src)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	code, _, err := Compile(module, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return code
}

// =============================================================================
// Splat expression — covers writeSplatExpression for all vector sizes
// =============================================================================

func TestCov_SplatVec2(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: f32) -> vec2<f32> { return vec2<f32>(x); }`)
	if !strings.Contains(code, ".xx") && !strings.Contains(code, "float2(") {
		t.Errorf("expected splat pattern, got:\n%s", code)
	}
}

func TestCov_SplatVec4(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: f32) -> vec4<f32> { return vec4<f32>(x); }`)
	if !strings.Contains(code, ".xxxx") && !strings.Contains(code, "float4(") {
		t.Errorf("expected splat pattern, got:\n%s", code)
	}
}

func TestCov_SplatIntVec(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: i32) -> vec3<i32> { return vec3<i32>(x); }`)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Swizzle write all components — covers writeSwizzleExpression
// =============================================================================

func TestCov_SwizzleAllComponents(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(v: vec4<f32>) -> f32 {
    let x = v.x;
    let y = v.y;
    let z = v.z;
    let w = v.w;
    let xy = v.xy;
    let xyz = v.xyz;
    let xyzw = v.xyzw;
    let yzw = v.yzw;
    let zw = v.zw;
    _ = y; _ = z; _ = w; _ = xy; _ = xyz; _ = xyzw; _ = yzw; _ = zw;
    return x;
}`)
	mustContain(t, code, []string{
		".x",
		".y",
		".z",
		".w",
		".xy",
		".xyz",
	})
}

// =============================================================================
// Access and AccessIndex on various types
// =============================================================================

func TestCov_StructMemberAccessMultiple(t *testing.T) {
	code := wgslToHLSL(t, `
struct S { a: f32, b: f32, c: f32, d: f32 }
fn f(s: S) -> f32 { return s.a + s.b + s.c + s.d; }`)
	mustContain(t, code, []string{".a", ".b", ".c", ".d"})
}

func TestCov_ArrayOfStructAccess(t *testing.T) {
	code := wgslToHLSL(t, `
struct Item { value: f32 }
fn f() -> f32 {
    var items: array<Item, 3>;
    items[0].value = 1.0;
    items[1].value = 2.0;
    items[2].value = 3.0;
    return items[0].value + items[1].value + items[2].value;
}`)
	mustContain(t, code, []string{"[0]", "[1]", "[2]", ".value"})
}

// =============================================================================
// Binary operators — covers all paths in writeBinaryExpression
// =============================================================================

func TestCov_BinaryU32Arithmetic(t *testing.T) {
	// u32 arithmetic does NOT use wrapping (unlike i32)
	code := wgslToHLSL(t, `fn f(a: u32, b: u32) -> u32 { return a + b; }`)
	mustNotContain(t, code, []string{"asint(asuint("})
	mustContain(t, code, []string{"+"})
}

func TestCov_BinaryF32Arithmetic(t *testing.T) {
	code := wgslToHLSL(t, `fn f(a: f32, b: f32) -> f32 { return a * b; }`)
	mustNotContain(t, code, []string{"asint("})
	mustContain(t, code, []string{"*"})
}

func TestCov_BinaryVecI32Arithmetic(t *testing.T) {
	// vec4<i32> add should also use wrapping
	code := wgslToHLSL(t, `
fn f(a: vec4<i32>, b: vec4<i32>) -> vec4<i32> { return a + b; }`)
	mustContain(t, code, []string{"asint(asuint("})
}

func TestCov_MatrixMatrixMultiply(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(a: mat4x4<f32>, b: mat4x4<f32>) -> mat4x4<f32> { return a * b; }`)
	mustContain(t, code, []string{"mul("})
}

func TestCov_VectorMatrixMultiply(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(v: vec4<f32>, m: mat4x4<f32>) -> vec4<f32> { return v * m; }`)
	mustContain(t, code, []string{"mul("})
}

// =============================================================================
// Unary on i32 — covers naga_neg helper path
// =============================================================================

func TestCov_UnaryI32Negate(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: i32) -> i32 { return -x; }`)
	mustContain(t, code, []string{"naga_neg("})
}

func TestCov_UnaryVec4I32Negate(t *testing.T) {
	code := wgslToHLSL(t, `fn f(v: vec4<i32>) -> vec4<i32> { return -v; }`)
	mustContain(t, code, []string{"naga_neg("})
}

// =============================================================================
// Cast expression paths — covers writeAsExpression for vector types
// =============================================================================

func TestCov_CastVecFloatToInt(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(v: vec4<f32>) -> vec4<i32> { return vec4<i32>(v); }`)
	// Should use naga_f2i32 for float-to-int conversion
	mustContain(t, code, []string{"naga_f2i32("})
}

func TestCov_CastVecIntToFloat(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(v: vec4<i32>) -> vec4<f32> { return vec4<f32>(v); }`)
	mustContain(t, code, []string{"float4("})
}

func TestCov_BitcastIntToFloat(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: i32) -> f32 { return bitcast<f32>(x); }`)
	mustContain(t, code, []string{"asfloat("})
}

func TestCov_BitcastFloatToUint(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: f32) -> u32 { return bitcast<u32>(x); }`)
	mustContain(t, code, []string{"asuint("})
}

func TestCov_BitcastUintToInt(t *testing.T) {
	code := wgslToHLSL(t, `fn f(x: u32) -> i32 { return bitcast<i32>(x); }`)
	mustContain(t, code, []string{"asint("})
}

// =============================================================================
// Select expression — covers writeSelectExpression via WGSL
// =============================================================================

func TestCov_SelectExpression(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(a: vec4<f32>, b: vec4<f32>, c: bool) -> vec4<f32> {
    return select(a, b, c);
}`)
	mustContain(t, code, []string{"?"})
}

// =============================================================================
// Compose expression — covers writeComposeExpression for structs/arrays
// =============================================================================

func TestCov_ComposeVec(t *testing.T) {
	code := wgslToHLSL(t, `
fn f() -> vec4<f32> {
    let a = vec2<f32>(1.0, 2.0);
    return vec4<f32>(a, 3.0, 4.0);
}`)
	mustContain(t, code, []string{"float4("})
}

func TestCov_ComposeStructWithReturn(t *testing.T) {
	code := wgslToHLSL(t, `
struct S { x: f32, y: f32 }
fn f() -> S { return S(1.0, 2.0); }`)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Load expression — covers writeLoadExpression
// =============================================================================

func TestCov_LoadFromPointer(t *testing.T) {
	code := wgslToHLSL(t, `
fn inc(p: ptr<function, i32>) {
    let val = *p;
    *p = val + 1;
}
fn f() -> i32 {
    var x: i32 = 10;
    inc(&x);
    return x;
}`)
	mustContain(t, code, []string{"inout int"})
}

// =============================================================================
// Texture operations — covers image codegen paths
// =============================================================================

func TestCov_TextureSampleGrad(t *testing.T) {
	code := wgslToHLSL(t, `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn f(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleGrad(tex, samp, uv, vec2<f32>(1.0, 0.0), vec2<f32>(0.0, 1.0));
}`)
	mustContain(t, code, []string{".SampleGrad("})
}

func TestCov_TextureNumSamples(t *testing.T) {
	code := wgslToHLSL(t, `
@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;

fn f() -> u32 {
    return textureNumSamples(tex);
}`)
	mustContain(t, code, []string{"GetDimensions("})
}

// =============================================================================
// Global variable types — covers writeGlobalVariableExpression
// =============================================================================

func TestCov_GlobalVariableHandle(t *testing.T) {
	code := wgslToHLSL(t, `
@group(0) @binding(0) var tex: texture_2d<f32>;

fn f() -> u32 {
    let dims = textureDimensions(tex, 0);
    return dims.x;
}`)
	mustContain(t, code, []string{"GetDimensions("})
}

// =============================================================================
// Struct padding and alignment — covers writeStructDefinition padding logic
// =============================================================================

func TestCov_StructEndPadding(t *testing.T) {
	code := wgslToHLSL(t, `
struct Aligned {
    x: f32,
    @align(16) y: f32,
    @align(16) z: f32,
}
fn f() -> f32 {
    var a: Aligned;
    a.x = 1.0;
    a.y = 2.0;
    a.z = 3.0;
    return a.x + a.y + a.z;
}`)
	mustContain(t, code, []string{"struct Aligned"})
}

// =============================================================================
// Comparison operations — covers cmp binary ops
// =============================================================================

func TestCov_VectorComparison(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(a: vec4<f32>, b: vec4<f32>) -> vec4<bool> {
    return a < b;
}`)
	// Vector comparison produces per-component comparison
	mustContain(t, code, []string{"<"})
}

// =============================================================================
// Local variable declaration — covers writeEmittedExpression fully
// =============================================================================

func TestCov_ManyLocalVars(t *testing.T) {
	code := wgslToHLSL(t, `
fn f() -> f32 {
    var a: f32 = 1.0;
    var b: f32 = 2.0;
    var c: f32 = 3.0;
    let d = a + b;
    let e = c + d;
    let g = e * 2.0;
    return g;
}`)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Array length — covers writeArrayLengthExpression
// =============================================================================

func TestCov_ArrayLength(t *testing.T) {
	code := wgslToHLSL(t, `
struct Data { vals: array<u32> }
@group(0) @binding(0) var<storage, read> data: Data;

fn f() -> u32 {
    return arrayLength(&data.vals);
}`)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Function argument expression — covers writeFunctionArgumentExpression
// =============================================================================

func TestCov_FunctionArgExpression(t *testing.T) {
	code := wgslToHLSL(t, `
fn add(a: f32, b: f32) -> f32 { return a + b; }
fn sub(a: f32, b: f32) -> f32 { return a - b; }
fn mul(a: f32, b: f32) -> f32 { return a * b; }
fn combined(x: f32, y: f32, z: f32) -> f32 {
    return add(mul(x, y), sub(y, z));
}`)
	// Only mul is renamed (HLSL keyword); add/sub are kept as-is
	mustContain(t, code, []string{"add(", "sub(", "mul_("})
}

// =============================================================================
// Helper function generation — covers writeModHelper, writeDivHelper,
// writeAbsHelper, writeNegHelper, writeModfHelper, writeFrexpHelper,
// writeF2I32Helper, writeF2U32Helper
// =============================================================================

func TestCov_HelperFunctionGeneration(t *testing.T) {
	// This shader uses several operations that generate helper functions:
	// i32 division -> naga_div, i32 modulo -> naga_mod, i32 negate -> naga_neg
	// float-to-int -> naga_f2i32, float-to-uint -> naga_f2u32
	code := wgslToHLSL(t, `
fn f(a: i32, b: i32, c: f32) -> i32 {
    let div = a / b;
    let mod = a % b;
    let neg = -a;
    let f2i = i32(c);
    let f2u = u32(c);
    _ = mod; _ = neg; _ = f2i; _ = f2u;
    return div;
}`)
	// Verify that helper functions are generated
	mustContain(t, code, []string{
		"naga_div(",
		"naga_mod(",
		"naga_neg(",
		"naga_f2i32(",
	})
}

// =============================================================================
// ExtractBits / InsertBits — covers helper function generation
// =============================================================================

func TestCov_ExtractBitsInsertBits(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(x: u32) -> u32 {
    let a = extractBits(x, 4u, 8u);
    let b = insertBits(x, 255u, 0u, 8u);
    return a + b;
}`)
	// extractBits and insertBits use NagaExtractBits/NagaInsertBits helpers
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// CountOneBits with i32 — covers MissingIntOverload pattern
// =============================================================================

func TestCov_CountOneBitsI32(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(x: i32) -> i32 { return countOneBits(x); }`)
	// For i32: asint(countbits(asuint(x)))
	mustContain(t, code, []string{"asint(", "countbits(", "asuint("})
}

func TestCov_ReverseBitsI32(t *testing.T) {
	code := wgslToHLSL(t, `
fn f(x: i32) -> i32 { return reverseBits(x); }`)
	mustContain(t, code, []string{"asint(", "reversebits(", "asuint("})
}

// =============================================================================
// Texture 1D — covers ImageDim1D path
// =============================================================================

func TestCov_Texture1D(t *testing.T) {
	code := wgslToHLSL(t, `
@group(0) @binding(0) var tex: texture_1d<f32>;
@group(0) @binding(1) var samp: sampler;

fn f(u: f32) -> vec4<f32> {
    return textureSampleLevel(tex, samp, u, 0.0);
}`)
	mustContain(t, code, []string{"Texture1D"})
}

// =============================================================================
// Compute shader with multiple builtins
// =============================================================================

func TestCov_ComputeMultipleBuiltins(t *testing.T) {
	code := wgslToHLSL(t, `
@compute @workgroup_size(8, 8, 1)
fn f(
    @builtin(global_invocation_id) gid: vec3<u32>,
    @builtin(local_invocation_id) lid: vec3<u32>,
    @builtin(workgroup_id) wid: vec3<u32>,
    @builtin(local_invocation_index) li: u32,
    @builtin(num_workgroups) nwg: vec3<u32>,
) {
    _ = gid; _ = lid; _ = wid; _ = li; _ = nwg;
}`)
	mustContain(t, code, []string{
		"[numthreads(8, 8, 1)]",
		"SV_DispatchThreadID",
		"SV_GroupThreadID",
		"SV_GroupID",
		"SV_GroupIndex",
	})
}

// =============================================================================
// Vertex shader with instance_index — covers BuiltinInstanceIndex
// =============================================================================

func TestCov_VertexInstanceIndex(t *testing.T) {
	code := wgslToHLSL(t, `
@vertex
fn vs(@builtin(vertex_index) vid: u32, @builtin(instance_index) iid: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(vid), f32(iid), 0.0, 1.0);
}`)
	mustContain(t, code, []string{
		"SV_VertexID",
		"SV_InstanceID",
	})
}

// =============================================================================
// Fragment shader with front_facing — covers BuiltinFrontFacing
// =============================================================================

func TestCov_FragmentFrontFacing(t *testing.T) {
	code := wgslToHLSL(t, `
@fragment
fn fs(@builtin(front_facing) ff: bool, @builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    if ff {
        return vec4<f32>(1.0, 0.0, 0.0, 1.0);
    }
    return vec4<f32>(0.0, 0.0, 1.0, 1.0);
}`)
	mustContain(t, code, []string{"SV_IsFrontFace"})
}

// =============================================================================
// Fragment shader with sample_index — covers BuiltinSampleIndex
// =============================================================================

func TestCov_FragmentSampleIndex(t *testing.T) {
	code := wgslToHLSL(t, `
@fragment
fn fs(@builtin(sample_index) si: u32) -> @location(0) vec4<f32> {
    return vec4<f32>(f32(si), 0.0, 0.0, 1.0);
}`)
	mustContain(t, code, []string{"SV_SampleIndex"})
}

// =============================================================================
// I32 min literal edge case — covers i32MinLiteral
// =============================================================================

func TestCov_I32MinLiteral(t *testing.T) {
	code := wgslToHLSL(t, `
fn f() -> i32 {
    return -2147483647 - 1;
}`)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// hlslBlockEndsWithReturn — covers the check function
// =============================================================================

func TestHLSLBlockEndsWithReturn(t *testing.T) {
	retStmt := ir.StmtReturn{}
	breakStmt := ir.StmtBreak{}

	block := ir.Block{{Kind: retStmt}}
	if !hlslBlockEndsWithReturn(block) {
		t.Error("block ending with return should be detected")
	}

	block2 := ir.Block{{Kind: breakStmt}}
	if hlslBlockEndsWithReturn(block2) {
		t.Error("block ending with break should not be detected as return")
	}

	emptyBlock := ir.Block{}
	if hlslBlockEndsWithReturn(emptyBlock) {
		t.Error("empty block should not be detected as return")
	}
}

// =============================================================================
// formatSpecialFloat — covers Inf/NaN formatting
// =============================================================================

func TestFormatSpecialFloat(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		want   string
		isSpec bool
	}{
		{"pos_inf", math.Inf(1), hlslPosInf, true},
		{"neg_inf", math.Inf(-1), hlslNegInf, true},
		{"nan", math.NaN(), hlslNaN, true},
		{"normal", 1.0, "", false},
		{"zero", 0.0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, isSpecial := formatSpecialFloat(tt.value)
			if isSpecial != tt.isSpec {
				t.Errorf("formatSpecialFloat(%v) isSpecial=%v, want %v", tt.value, isSpecial, tt.isSpec)
			}
			if isSpecial && got != tt.want {
				t.Errorf("formatSpecialFloat(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// =============================================================================
// hlslTypeId — covers type identification for helpers
// =============================================================================

func TestHLSLTypeId(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                       // 0: f32
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1: float4
			{Name: "MyStruct", Inner: ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}}},   // 2: struct
		},
	}

	w := newTestWriter(module, nil, map[ir.TypeHandle]string{
		0: "float",
		1: "float4",
		2: "MyStruct",
	})

	got0 := w.hlslTypeId(0)
	if got0 != "float" {
		t.Errorf("hlslTypeId(0) = %q, want 'float'", got0)
	}
	got1 := w.hlslTypeId(1)
	if got1 != "float4" {
		t.Errorf("hlslTypeId(1) = %q, want 'float4'", got1)
	}
	got2 := w.hlslTypeId(2)
	if got2 != "MyStruct" {
		t.Errorf("hlslTypeId(2) = %q, want 'MyStruct'", got2)
	}
}

// =============================================================================
// writeExpressionToString — covers the string-returning expression wrapper
// =============================================================================

func TestWriteExpressionToString(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	f32Handle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	got, err := w.writeExpressionToString(0)
	if err != nil {
		t.Fatalf("writeExpressionToString: %v", err)
	}
	if !strings.Contains(got, "42.0") {
		t.Errorf("expected '42.0' in output, got %q", got)
	}
}

// indentStr is already tested in storage_test.go

// =============================================================================
// toLowerFirst — covers helper for stage names
// =============================================================================

func TestToLowerFirst(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Vertex", "vertex"},
		{"Fragment", "fragment"},
		{"Compute", "compute"},
		{"", ""},
		{"a", "a"},
		{"ABC", "aBC"},
	}
	for _, tt := range tests {
		got := toLowerFirst(tt.input)
		if got != tt.want {
			t.Errorf("toLowerFirst(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// =============================================================================
// stageName — covers stage name mapping
// =============================================================================

func TestStageName(t *testing.T) {
	tests := []struct {
		stage ir.ShaderStage
		want  string
	}{
		{ir.StageVertex, "Vertex"},
		{ir.StageFragment, "Fragment"},
		{ir.StageCompute, "Compute"},
	}
	for _, tt := range tests {
		got := stageName(tt.stage)
		if got != tt.want {
			t.Errorf("stageName(%d) = %q, want %q", tt.stage, got, tt.want)
		}
	}
}
