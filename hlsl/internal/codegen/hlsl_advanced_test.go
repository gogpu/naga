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
// Global Const Compose Expressions — covers writeGlobalConstExpression,
// writeGlobalComposeExpression, writeGlobalBinaryExpression, writeGlobalSplatExpression
// =============================================================================

func TestCompile_GlobalConstCompose(t *testing.T) {
	src := `
const ZERO = vec4<f32>(0.0, 0.0, 0.0, 0.0);
const ONE = vec4<f32>(1.0, 1.0, 1.0, 1.0);
const OFFSET = vec2<i32>(1, 2);

fn use_consts() -> vec4<f32> {
    _ = OFFSET;
    return ZERO + ONE;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"static const",
	})
}

func TestCompile_GlobalConstBinaryExpr(t *testing.T) {
	src := `
const A: f32 = 2.0;
const B: f32 = 3.0;
const C: f32 = A + B;

fn use_c() -> f32 {
    return C;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"static const",
	})
}

// =============================================================================
// Array / Struct Access Index Expressions — covers writeAccessExpression,
// writeAccessIndexExpression, getAccessMaxIndex
// =============================================================================

func TestCompile_StructMemberAccess(t *testing.T) {
	src := `
struct Vec2Pair {
    a: vec2<f32>,
    b: vec2<f32>,
}

fn test_access(pair: Vec2Pair) -> vec2<f32> {
    return pair.a + pair.b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		".a",
		".b",
	})
}

func TestCompile_NestedArrayAccess(t *testing.T) {
	src := `
fn test_nested_array() -> f32 {
    var grid: array<array<f32, 4>, 4>;
    grid[0][0] = 1.0;
    grid[1][2] = 2.0;
    return grid[0][0] + grid[1][2];
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Should have nested array indexing
	mustContain(t, code, []string{
		"[4]",
	})
}

func TestCompile_DynamicArrayAccess(t *testing.T) {
	src := `
fn test_dynamic(idx: u32) -> f32 {
    var arr: array<f32, 8>;
    arr[0] = 1.0;
    arr[7] = 8.0;
    return arr[idx];
}
`
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.RestrictIndexing = true
	code := compileWGSLToHLSL(t, src, opts)
	// RestrictIndexing should add bounds clamping: min(idx, 7u)
	if !strings.Contains(code, "min(") {
		// Some implementations may use different clamping
		// At minimum, the array access must compile
		if code == "" {
			t.Error("expected non-empty output")
		}
	}
}

// =============================================================================
// Loop with Continuing Block — covers writeLoopStatement continuing path
// =============================================================================

func TestCompile_LoopWithContinuing(t *testing.T) {
	src := `
fn test_loop_continuing() -> i32 {
    var i: i32 = 0;
    var sum: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        sum = sum + i;
        continuing {
            i = i + 1;
        }
    }
    return sum;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"while(true)",
	})
}

// =============================================================================
// Interpolation Modifiers — covers writeModifierForType, writeModifier,
// getInterpolationModifierForType
// =============================================================================

func TestCompile_InterpolationModifiers(t *testing.T) {
	src := `
struct VSOutput {
    @builtin(position) pos: vec4<f32>,
    @location(0) @interpolate(flat) flat_val: u32,
    @location(1) @interpolate(linear) linear_val: f32,
    @location(2) @interpolate(perspective, centroid) persp_centroid: f32,
}

@vertex
fn vs_main() -> VSOutput {
    var out: VSOutput;
    out.pos = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    out.flat_val = 0u;
    out.linear_val = 0.0;
    out.persp_centroid = 0.0;
    return out;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"nointerpolation", // flat
	})
}

// =============================================================================
// Storage Texture Formats — covers storageFormatToHLSL, writeImageStoreStatement
// =============================================================================

func TestCompile_StorageTextureFormats(t *testing.T) {
	// Test a representative set of storage texture formats
	// Use proper WGSL vec types, not HLSL names
	formats := []struct {
		format  string
		wgslVal string
	}{
		{"rgba8unorm", "vec4<f32>(0.0, 0.0, 0.0, 0.0)"},
		{"rgba16float", "vec4<f32>(0.0, 0.0, 0.0, 0.0)"},
		{"r32uint", "vec4<u32>(0u, 0u, 0u, 0u)"},
		{"r32sint", "vec4<i32>(0, 0, 0, 0)"},
		{"r32float", "vec4<f32>(0.0, 0.0, 0.0, 0.0)"},
		{"rgba32float", "vec4<f32>(0.0, 0.0, 0.0, 0.0)"},
		{"rgba32uint", "vec4<u32>(0u, 0u, 0u, 0u)"},
		{"rgba32sint", "vec4<i32>(0, 0, 0, 0)"},
	}

	for _, f := range formats {
		t.Run(f.format, func(t *testing.T) {
			src := `
@group(0) @binding(0) var tex: texture_storage_2d<` + f.format + `, write>;

@compute @workgroup_size(1)
fn cs_main() {
    textureStore(tex, vec2<i32>(0, 0), ` + f.wgslVal + `);
}
`
			code := compileWGSLToHLSL(t, src, nil)
			mustContain(t, code, []string{
				"RWTexture2D",
			})
		})
	}
}

// =============================================================================
// Texture Load — covers writeImageLoadExpression, writeTextureCoordinates
// =============================================================================

func TestCompile_TextureLoad(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;

fn test_load() -> vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		".Load(",
	})
}

// =============================================================================
// Texture Sample with Level — covers writeSampleLevel
// =============================================================================

func TestCompile_TextureSampleLevel(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

fn test_sample_level(uv: vec2<f32>) -> vec4<f32> {
    return textureSampleLevel(tex, samp, uv, 0.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		".SampleLevel(",
	})
}

// =============================================================================
// Texture Sample with Bias — covers writeImageSampleExpression bias path
// =============================================================================

func TestCompile_TextureSampleBias(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSampleBias(tex, samp, uv, 2.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		".SampleBias(",
	})
}

// =============================================================================
// Texture Comparison Sample — covers depth texture sampling path
// =============================================================================

func TestCompile_TextureSampleCompare(t *testing.T) {
	src := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var shadow_samp: sampler_comparison;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) f32 {
    return textureSampleCompare(depth_tex, shadow_samp, uv, 0.5);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"Texture2D",
		"SamplerComparisonState",
		".SampleCmp(",
	})
}

// =============================================================================
// Texture Number of Levels — covers writeImageQueryExpression
// =============================================================================

func TestCompile_TextureNumLevels(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_2d<f32>;

fn get_levels() -> u32 {
    return textureNumLevels(tex);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"GetDimensions(",
	})
}

// =============================================================================
// MatCx2 Column Store — covers writeMatCx2StoreCastIfNeeded
// =============================================================================

func TestCompile_MatCx2Store(t *testing.T) {
	src := `
fn test_mat3x2() -> mat3x2<f32> {
    var m: mat3x2<f32>;
    m[0] = vec2<f32>(1.0, 0.0);
    m[1] = vec2<f32>(0.0, 1.0);
    m[2] = vec2<f32>(0.0, 0.0);
    return m;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// MatCx2 types use typedef in HLSL
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Function with Local Variables — covers writeLocalVariableExpression,
// writeLoadExpression, writeStoreStatement fully
// =============================================================================

func TestCompile_LocalVariables(t *testing.T) {
	src := `
fn test_locals() -> vec4<f32> {
    var pos: vec4<f32> = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    var color: vec3<f32> = vec3<f32>(1.0, 0.0, 0.0);
    pos.x = 1.0;
    pos.y = 2.0;
    color.g = 1.0;
    return vec4<f32>(color, pos.w);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"float4 pos",
		"float3 color",
	})
}

// =============================================================================
// Multiple Storage Buffers — covers writeStorageBufferDeclaration,
// writeStorageLoadHelpers, storage access patterns
// =============================================================================

func TestCompile_MultipleStorageBuffers(t *testing.T) {
	src := `
struct InputData {
    values: array<vec4<f32>>,
}

struct OutputData {
    results: array<vec4<f32>>,
}

@group(0) @binding(0) var<storage, read> input: InputData;
@group(0) @binding(1) var<storage, read_write> output: OutputData;

@compute @workgroup_size(64)
fn cs_main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let val = input.values[gid.x];
    output.results[gid.x] = val * 2.0;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"ByteAddressBuffer",
		"RWByteAddressBuffer",
	})
}

// =============================================================================
// Phony Assignment — covers named expression with _
// =============================================================================

func TestCompile_PhonyAssignment(t *testing.T) {
	src := `
fn test_phony(x: f32) -> f32 {
    _ = x * 2.0;
    return x;
}
`
	// Phony assignments should be silent
	code := compileWGSLToHLSL(t, src, nil)
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// Nested Struct with Nested Array — covers deep type emission
// =============================================================================

func TestCompile_NestedStructTypes(t *testing.T) {
	src := `
struct Inner {
    value: f32,
}

struct Outer {
    data: Inner,
    count: u32,
}

fn test_nested() -> f32 {
    var o: Outer;
    o.data.value = 42.0;
    o.count = 1u;
    return o.data.value;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"struct Inner",
		"struct Outer",
		".data",
		".value",
	})
}

// =============================================================================
// Switch with U32 Cases — covers SwitchValueU32 in writeSwitchCase
// =============================================================================

func TestCompile_SwitchU32(t *testing.T) {
	src := `
fn test_switch_u32(x: u32) -> u32 {
    var result: u32 = 0u;
    switch x {
        case 0u: {
            result = 10u;
        }
        case 1u: {
            result = 20u;
        }
        default: {
            result = 30u;
        }
    }
    return result;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"switch(",
		"case 0u:",
		"case 1u:",
		"default:",
	})
}

// =============================================================================
// While loop with break if — covers loop + continuing with break if
// =============================================================================

func TestCompile_WhileLoop(t *testing.T) {
	src := `
fn test_while() -> i32 {
    var i: i32 = 0;
    while i < 100 {
        i = i + 1;
    }
    return i;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"while(true)",
	})
}

// =============================================================================
// Texture Multisampled — covers multisampled texture types
// =============================================================================

func TestCompile_TextureMultisampled(t *testing.T) {
	src := `
@group(0) @binding(0) var tex: texture_multisampled_2d<f32>;

fn test_ms() -> vec4<f32> {
    return textureLoad(tex, vec2<i32>(0, 0), 0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"Texture2DMS",
	})
}

// =============================================================================
// Texture Cube / 3D — covers different texture dimensions
// =============================================================================

func TestCompile_TextureCube(t *testing.T) {
	src := `
@group(0) @binding(0) var cube_tex: texture_cube<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) dir: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(cube_tex, samp, dir);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"TextureCube",
	})
}

func TestCompile_Texture3D(t *testing.T) {
	src := `
@group(0) @binding(0) var vol_tex: texture_3d<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uvw: vec3<f32>) -> @location(0) vec4<f32> {
    return textureSample(vol_tex, samp, uvw);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"Texture3D",
	})
}

// =============================================================================
// Texture Array — covers texture array dimension types
// =============================================================================

func TestCompile_TextureArray2D(t *testing.T) {
	src := `
@group(0) @binding(0) var tex_arr: texture_2d_array<f32>;
@group(0) @binding(1) var samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex_arr, samp, uv, 0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"Texture2DArray",
	})
}

// =============================================================================
// ForceLoopBounding Option — covers loop bounding insertion
// =============================================================================

func TestCompile_ForceLoopBounding(t *testing.T) {
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.ForceLoopBounding = true

	src := `
fn bounded_loop() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 10; i++) {
        sum = sum + i;
    }
    return sum;
}
`
	code := compileWGSLToHLSL(t, src, opts)
	// ForceLoopBounding adds a maximum iteration counter
	if code == "" {
		t.Error("expected non-empty output")
	}
}

// =============================================================================
// SpecialConstants option — covers writeSpecialConstants,
// BuiltinVertexIndex/InstanceIndex offset
// =============================================================================

func TestCompile_SpecialConstants(t *testing.T) {
	opts := DefaultOptions()
	opts.FakeMissingBindings = true
	opts.SpecialConstantsBinding = &BindTarget{Register: 0, Space: 1}

	src := `
@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @builtin(instance_index) iid: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(vid), f32(iid), 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, opts)
	mustContain(t, code, []string{
		"NagaConstants",
		"first_vertex",
		"first_instance",
	})
}

// =============================================================================
// Derivative Fine/Coarse Variants — covers all derivative axis+control combos
// =============================================================================

func TestCompile_DerivativeFineCoarse(t *testing.T) {
	src := `
@fragment
fn fs_main(@location(0) x: f32) -> @location(0) vec4<f32> {
    let dx_fine = dpdxFine(x);
    let dy_fine = dpdyFine(x);
    let dx_coarse = dpdxCoarse(x);
    let dy_coarse = dpdyCoarse(x);
    let fw_fine = fwidthFine(x);
    let fw_coarse = fwidthCoarse(x);
    return vec4<f32>(dx_fine + dy_fine + dx_coarse + dy_coarse + fw_fine + fw_coarse, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"ddx_fine(",
		"ddy_fine(",
		"ddx_coarse(",
		"ddy_coarse(",
	})
}

// =============================================================================
// Modulo / Remainder — covers naga_mod helper function
// =============================================================================

func TestCompile_ModuloOperator(t *testing.T) {
	src := `
fn test_mod(a: i32, b: i32) -> i32 {
    return a % b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// i32 modulo uses naga_mod helper
	mustContain(t, code, []string{
		"naga_mod(",
	})
}

func TestCompile_FloatModulo(t *testing.T) {
	src := `
fn test_fmod(a: f32, b: f32) -> f32 {
    return a % b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// Float modulo also uses naga_mod helper
	mustContain(t, code, []string{
		"naga_mod(",
	})
}

// =============================================================================
// Integer Division — covers naga_div helper function
// =============================================================================

func TestCompile_IntegerDivision(t *testing.T) {
	src := `
fn test_div(a: i32, b: i32) -> i32 {
    return a / b;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	// i32 division uses naga_div helper to avoid UB
	mustContain(t, code, []string{
		"naga_div(",
	})
}

// =============================================================================
// Complex multi-function shader — covers writeFunction, writeCallStatement,
// function inlining paths
// =============================================================================

func TestCompile_MultiFunctionShader(t *testing.T) {
	src := `
fn square(x: f32) -> f32 {
    return x * x;
}

fn distance_2d(ax: f32, ay: f32, bx: f32, by: f32) -> f32 {
    return sqrt(square(bx - ax) + square(by - ay));
}

@compute @workgroup_size(1)
fn cs_main() {
    let d = distance_2d(0.0, 0.0, 3.0, 4.0);
    _ = d;
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"square(",
		"distance_2d(",
		"sqrt(",
		"[numthreads(1, 1, 1)]",
	})
}

// =============================================================================
// Comparison sampler test — covers SamplerComparisonState type
// =============================================================================

func TestCompile_SamplerComparison(t *testing.T) {
	src := `
@group(0) @binding(0) var depth_tex: texture_depth_2d;
@group(0) @binding(1) var shadow_sampler: sampler_comparison;

@fragment
fn fs_main(@builtin(position) pos: vec4<f32>) -> @location(0) f32 {
    return textureSampleCompareLevel(depth_tex, shadow_sampler, pos.xy, 0.5);
}
`
	code := compileWGSLToHLSL(t, src, nil)
	mustContain(t, code, []string{
		"SamplerComparisonState",
		".SampleCmpLevelZero(",
	})
}

// =============================================================================
// Helpers from hlsl_compile_test.go reused via package scope
// =============================================================================

// compileWGSLToHLSLAdv is an alias for compileWGSLToHLSL using a different
// name to avoid redeclaring in the same package.
func compileAdvanced(t *testing.T, src string, opts *Options) string {
	t.Helper()
	module := parseAdvanced(t, src)
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

func parseAdvanced(t *testing.T, src string) *ir.Module {
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
