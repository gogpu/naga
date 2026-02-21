// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/wgsl"
)

// compileWGSLToHLSL is a test helper that compiles WGSL source to HLSL.
func compileWGSLToHLSL(t *testing.T, source string) string {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	opts := hlsl.DefaultOptions()
	code, _, err := hlsl.Compile(module, opts)
	if err != nil {
		t.Fatalf("HLSL Compile failed: %v", err)
	}

	return code
}

// assertContains checks that the HLSL output contains the expected substring.
func assertContains(t *testing.T, code, expected string) {
	t.Helper()
	if !strings.Contains(code, expected) {
		t.Errorf("expected HLSL output to contain %q\n\nGot:\n%s", expected, code)
	}
}

// assertNotContains checks that the HLSL output does NOT contain the given substring.
func assertNotContains(t *testing.T, code, unexpected string) {
	t.Helper()
	if strings.Contains(code, unexpected) {
		t.Errorf("expected HLSL output NOT to contain %q\n\nGot:\n%s", unexpected, code)
	}
}

// =============================================================================
// Triangle Shader — the primary BSOD-causing shader
// =============================================================================

func TestE2E_TriangleShader(t *testing.T) {
	source := `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),
        vec2<f32>(-0.5, -0.5),
        vec2<f32>(0.5, -0.5)
    );
    return vec4<f32>(positions[idx], 0.0, 1.0);
}

@fragment
fn ps_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Vertex entry point with input struct.
	// Entry point naming: stage prefix + original name.
	// WGSL "vs_main" -> HLSL "vs_vs_main", WGSL "ps_main" -> HLSL "ps_ps_main".
	assertContains(t, code, "struct vs_vs_main_Input")
	assertContains(t, code, "SV_VertexID")

	// Vertex output semantic
	assertContains(t, code, ": SV_Position")

	// Fragment output semantic
	assertContains(t, code, ": SV_Target0")

	// Array syntax: HLSL uses {elem1, elem2} not type(elem1, elem2)
	assertContains(t, code, "{")
	assertNotContains(t, code, "float2[3](")

	// Function bodies should NOT be stubs
	assertNotContains(t, code, "// Function body (to be implemented)")
	assertNotContains(t, code, "// Entry point body (to be implemented)")

	// Should contain actual return statements
	assertContains(t, code, "return float4(")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Simple Vertex Shader
// =============================================================================

func TestE2E_SimpleVertexShader(t *testing.T) {
	source := `
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have SV_Position semantic on return
	assertContains(t, code, ": SV_Position")

	// Should have SV_VertexID in input struct
	assertContains(t, code, "SV_VertexID")

	// Should have actual return
	assertContains(t, code, "return float4(")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Simple Fragment Shader
// =============================================================================

func TestE2E_SimpleFragmentShader(t *testing.T) {
	source := `
@fragment
fn main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Fragment output should have SV_Target0
	assertContains(t, code, ": SV_Target0")

	// Should have actual return
	assertContains(t, code, "return float4(")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Compute Shader
// =============================================================================

func TestE2E_ComputeShader(t *testing.T) {
	source := `
@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var x: u32 = id.x * 2u;
}
`
	code := compileWGSLToHLSL(t, source)

	// numthreads attribute
	assertContains(t, code, "[numthreads(64, 1, 1)]")

	// SV_DispatchThreadID for global_invocation_id
	assertContains(t, code, "SV_DispatchThreadID")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Vertex + Fragment with struct output
// =============================================================================

func TestE2E_VertexFragmentWithStruct(t *testing.T) {
	source := `
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var out: VertexOutput;
    out.position = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have SV_Position in struct
	assertContains(t, code, "SV_Position")

	// Should have TEXCOORD for location(0) color
	// (or SV_Target0 on fragment output)
	assertContains(t, code, "SV_Target0")

	// Function bodies should not be stubs
	assertNotContains(t, code, "// Function body (to be implemented)")
	assertNotContains(t, code, "// Entry point body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Uniform buffer
// =============================================================================

func TestE2E_UniformBuffer(t *testing.T) {
	source := `
struct Camera {
    view_proj: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> camera: Camera;

@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    return vec4<f32>(pos.x, pos.y, pos.z, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have cbuffer or ConstantBuffer
	hasCBuffer := strings.Contains(code, "cbuffer") || strings.Contains(code, "ConstantBuffer")
	if !hasCBuffer {
		t.Errorf("expected cbuffer or ConstantBuffer declaration\n\nGot:\n%s", code)
	}

	// Should have register binding
	assertContains(t, code, "register(")

	// Should have row_major matrix type in struct
	assertContains(t, code, "row_major float4x4")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Math functions
// =============================================================================

func TestE2E_MathFunctions(t *testing.T) {
	source := `
@fragment
fn main(@location(0) v: vec3<f32>) -> @location(0) vec4<f32> {
    let n = normalize(v);
    let l = length(v);
    let d = dot(n, v);
    let c = cross(n, v);
    let s = sqrt(l);
    let a = abs(d);
    let mx = max(s, a);
    let mn = min(s, a);
    let cl = clamp(d, 0.0, 1.0);
    return vec4<f32>(mx, mn, cl, c.x);
}
`
	code := compileWGSLToHLSL(t, source)

	// HLSL built-in functions
	assertContains(t, code, "normalize(")
	assertContains(t, code, "length(")
	assertContains(t, code, "dot(")
	assertContains(t, code, "cross(")
	assertContains(t, code, "sqrt(")
	assertContains(t, code, "abs(")
	assertContains(t, code, "max(")
	assertContains(t, code, "min(")
	assertContains(t, code, "clamp(")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// If/Else control flow
// =============================================================================

func TestE2E_IfElse(t *testing.T) {
	source := `
@fragment
fn main(@location(0) x: f32) -> @location(0) vec4<f32> {
    var color: vec4<f32>;
    if x > 0.5 {
        color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    } else {
        color = vec4<f32>(0.0, 0.0, 1.0, 1.0);
    }
    return color;
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have if/else
	assertContains(t, code, "if (")
	assertContains(t, code, "} else {")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Switch statement
// =============================================================================

func TestE2E_Switch(t *testing.T) {
	source := `
@fragment
fn main(@location(0) idx: u32) -> @location(0) vec4<f32> {
    var color: vec4<f32>;
    switch idx {
        case 0u: { color = vec4<f32>(1.0, 0.0, 0.0, 1.0); }
        case 1u: { color = vec4<f32>(0.0, 1.0, 0.0, 1.0); }
        default: { color = vec4<f32>(0.0, 0.0, 1.0, 1.0); }
    }
    return color;
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have switch/case/default
	assertContains(t, code, "switch")
	assertContains(t, code, "case ")
	assertContains(t, code, "default:")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Local const
// =============================================================================

func TestE2E_LocalConst(t *testing.T) {
	source := `
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    const PI = 3.14159;
    let x = PI * 2.0;
    return vec4<f32>(x, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have SV_Position
	assertContains(t, code, ": SV_Position")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Multiple entry points (vertex + fragment in same module)
// =============================================================================

func TestE2E_NoEntryPointDuplication(t *testing.T) {
	source := `
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}

@fragment
fn ps_main() -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Each entry point should appear exactly once
	vsCount := strings.Count(code, "vs_main(")
	psCount := strings.Count(code, "ps_main(")

	// vs_main appears in: definition signature + possibly input struct name
	// ps_main appears in: definition signature
	// But neither should appear more than twice (definition + possible struct)
	if vsCount > 2 {
		t.Errorf("vs_main appears %d times (expected at most 2), duplication detected\n\n%s", vsCount, code)
	}
	if psCount > 2 {
		t.Errorf("ps_main appears %d times (expected at most 2), duplication detected\n\n%s", psCount, code)
	}

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Header check
// =============================================================================

func TestE2E_HeaderComment(t *testing.T) {
	source := `
@vertex
fn main() -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have naga header
	assertContains(t, code, "Generated by naga")

	// Should have shader model comment
	assertContains(t, code, "SM 5.1")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Swizzle
// =============================================================================

func TestE2E_Swizzle(t *testing.T) {
	source := `
@fragment
fn main(@location(0) v: vec4<f32>) -> @location(0) vec4<f32> {
    let xy = v.xy;
    return vec4<f32>(xy.x, xy.y, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have swizzle access
	assertContains(t, code, ".xy")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Loop
// =============================================================================

func TestE2E_ForLoop(t *testing.T) {
	source := `
@fragment
fn main(@location(0) x: f32) -> @location(0) vec4<f32> {
    var sum: f32 = 0.0;
    for (var i: u32 = 0u; i < 10u; i = i + 1u) {
        sum = sum + x;
    }
    return vec4<f32>(sum, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// Should have loop construct
	hasLoop := strings.Contains(code, "for") || strings.Contains(code, "while") || strings.Contains(code, "loop")
	if !hasLoop {
		t.Errorf("expected loop construct in HLSL output\n\nGot:\n%s", code)
	}

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Struct argument entry point (the gogpu shader pattern)
// =============================================================================

func TestE2E_StructArgumentEntryPoint(t *testing.T) {
	source := `
struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) color: vec4<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.position = vec4<f32>(input.position, 0.0, 1.0);
    output.color = input.color;
    return output;
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    return input.color;
}
`
	code := compileWGSLToHLSL(t, source)

	// Input struct should have flattened struct members with semantics.
	// Entry point naming: stage prefix + original name.
	// WGSL "vs_main" -> HLSL "vs_vs_main".
	assertContains(t, code, "struct vs_vs_main_Input")
	assertContains(t, code, "TEXCOORD0")
	assertContains(t, code, "TEXCOORD1")

	// The struct variable should be declared and populated from _input
	assertContains(t, code, "VertexInput input")
	assertContains(t, code, "input.position = _input.position")
	assertContains(t, code, "input.color = _input.color")

	// Output struct should have SV_Position
	assertContains(t, code, "SV_Position")

	// Fragment output
	assertContains(t, code, "SV_Target0")

	// No stubs
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("HLSL struct arg output:\n%s", code)
}

// =============================================================================
// MSDF text shader -- multiple entry points + function calls with results
// =============================================================================

func TestE2E_MSDFTextShader(t *testing.T) {
	source := `
struct TextUniforms {
    transform: mat4x4<f32>,
    color: vec4<f32>,
    msdf_params: vec4<f32>,
}

struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) tex_coord: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
    @location(1) color: vec4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: TextUniforms;
@group(0) @binding(1) var msdf_atlas: texture_2d<f32>;
@group(0) @binding(2) var msdf_sampler: sampler;

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let p = vec4<f32>(in.position, 0.0, 1.0);
    let col0 = uniforms.transform[0];
    let col1 = uniforms.transform[1];
    let col2 = uniforms.transform[2];
    let col3 = uniforms.transform[3];
    let pos = p.x * col0 + p.y * col1 + p.z * col2 + p.w * col3;
    out.position = pos;
    out.tex_coord = in.tex_coord;
    out.color = uniforms.color;
    return out;
}

fn median3(r: f32, g: f32, b: f32) -> f32 {
    return max(min(r, g), min(max(r, g), b));
}

fn sampleSD(uv: vec2<f32>) -> f32 {
    let msdf = textureSample(msdf_atlas, msdf_sampler, uv).rgb;
    return median3(msdf.r, msdf.g, msdf.b) - 0.5;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let px_range = uniforms.msdf_params.x;
    let atlas_size = uniforms.msdf_params.y;
    let unit_range = vec2<f32>(px_range / atlas_size, px_range / atlas_size);
    let screen_tex_size = vec2<f32>(1.0, 1.0) / fwidth(in.tex_coord);
    let screen_px_range = max(0.5 * dot(unit_range, screen_tex_size), 1.0);
    let offset = fwidth(in.tex_coord) * 0.25;
    let sd0 = sampleSD(in.tex_coord + vec2<f32>(-offset.x, -offset.y));
    let sd1 = sampleSD(in.tex_coord + vec2<f32>( offset.x, -offset.y));
    let sd2 = sampleSD(in.tex_coord + vec2<f32>(-offset.x,  offset.y));
    let sd3 = sampleSD(in.tex_coord + vec2<f32>( offset.x,  offset.y));
    let sd = (sd0 + sd1 + sd2 + sd3) * 0.25;
    let alpha = clamp(screen_px_range * sd + 0.5, 0.0, 1.0);
    return vec4<f32>(in.color.rgb * alpha, in.color.a * alpha);
}

@fragment
fn fs_main_outline(in: VertexOutput) -> @location(0) vec4<f32> {
    let msdf = textureSample(msdf_atlas, msdf_sampler, in.tex_coord).rgb;
    let sd = median3(msdf.r, msdf.g, msdf.b) - 0.5;
    let unit_range = vec2<f32>(uniforms.msdf_params.x / uniforms.msdf_params.y,
                              uniforms.msdf_params.x / uniforms.msdf_params.y);
    let screen_tex_size = vec2<f32>(1.0, 1.0) / fwidth(in.tex_coord);
    let screen_px_range = max(0.5 * dot(unit_range, screen_tex_size), 1.0);
    let screen_px_distance = screen_px_range * sd;
    let outline_width = uniforms.msdf_params.z;
    let fill_alpha = clamp(screen_px_distance + 0.5, 0.0, 1.0);
    let outline_alpha = clamp(screen_px_distance + outline_width + 0.5, 0.0, 1.0);
    let outline_color = vec4<f32>(1.0 - in.color.rgb, 1.0);
    let outline_diff = outline_alpha - fill_alpha;
    let fill = vec4<f32>(in.color.rgb * fill_alpha, in.color.a * fill_alpha);
    let outline = vec4<f32>(outline_color.rgb * outline_diff, outline_color.a * outline_diff);
    return fill + outline;
}

@fragment
fn fs_main_shadow(in: VertexOutput) -> @location(0) vec4<f32> {
    let shadow_offset = vec2<f32>(0.002, 0.002);
    let shadow_color = vec4<f32>(0.0, 0.0, 0.0, 0.5);
    let unit_range = vec2<f32>(uniforms.msdf_params.x / uniforms.msdf_params.y,
                              uniforms.msdf_params.x / uniforms.msdf_params.y);
    let screen_tex_size = vec2<f32>(1.0, 1.0) / fwidth(in.tex_coord);
    let screen_px_range = max(0.5 * dot(unit_range, screen_tex_size), 1.0);
    let shadow_msdf = textureSample(msdf_atlas, msdf_sampler, in.tex_coord - shadow_offset).rgb;
    let shadow_sd = median3(shadow_msdf.r, shadow_msdf.g, shadow_msdf.b) - 0.5;
    let shadow_alpha = clamp(screen_px_range * shadow_sd + 0.5, 0.0, 1.0);
    let msdf = textureSample(msdf_atlas, msdf_sampler, in.tex_coord).rgb;
    let fill_sd = median3(msdf.r, msdf.g, msdf.b) - 0.5;
    let fill_alpha = clamp(screen_px_range * fill_sd + 0.5, 0.0, 1.0);
    let shadow = vec4<f32>(shadow_color.rgb * shadow_alpha, shadow_color.a * shadow_alpha);
    let fill = vec4<f32>(in.color.rgb * fill_alpha, in.color.a * fill_alpha);
    return fill + shadow * (1.0 - fill.a);
}
`
	code := compileWGSLToHLSL(t, source)

	// Bug 1: All entry point names must be unique.
	// vs_main -> vs_vs_main, fs_main -> ps_fs_main,
	// fs_main_outline -> ps_fs_main_outline, fs_main_shadow -> ps_fs_main_shadow.
	assertContains(t, code, "vs_vs_main")
	assertContains(t, code, "ps_fs_main")
	assertContains(t, code, "ps_fs_main_outline")
	assertContains(t, code, "ps_fs_main_shadow")

	// Verify uniqueness: no generic ps_main that would collide.
	assertNotContains(t, code, "ps_main(") // old broken name should not appear
	assertNotContains(t, code, "ps_main ") // no generic ps_main prefix

	// Bug 2: Function call results must be declared with type and unique names.
	// Each call site gets a unique variable _crN (N = expression handle).
	// Verify typed declarations exist (float _crNN = funcname(...)).
	assertContains(t, code, "float _cr")           // at least one typed call result
	assertNotContains(t, code, "_sampleSD_result") // old broken global pattern gone
	assertNotContains(t, code, "_median3_result")  // old broken global pattern gone

	// Input/output structs should have unique names per entry point.
	assertContains(t, code, "struct vs_vs_main_Input")
	assertContains(t, code, "struct ps_fs_main_Input")
	assertContains(t, code, "struct ps_fs_main_outline_Input")
	assertContains(t, code, "struct ps_fs_main_shadow_Input")

	// Regular functions should be declared (not entry points).
	assertContains(t, code, "float median3(")
	assertContains(t, code, "float sampleSD(")

	// Semantic bindings for vertex output.
	assertContains(t, code, "SV_Position")
	assertContains(t, code, "SV_Target0")

	// Matrix members in uniform structs must have row_major qualifier.
	assertContains(t, code, "row_major float4x4 transform")

	// No stubs.
	assertNotContains(t, code, "// Function body (to be implemented)")
	assertNotContains(t, code, "// Entry point body (to be implemented)")

	t.Logf("MSDF text HLSL output:\n%s", code)
}

// TestE2E_MSDFTextShader_PerEntryPoint tests per-entry-point compilation as DX12 backend does.
func TestE2E_MSDFTextShader_PerEntryPoint(t *testing.T) {
	source := `
struct TextUniforms {
    transform: mat4x4<f32>,
    color: vec4<f32>,
    msdf_params: vec4<f32>,
}
struct VertexInput {
    @location(0) position: vec2<f32>,
    @location(1) tex_coord: vec2<f32>,
}
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
    @location(1) color: vec4<f32>,
}
@group(0) @binding(0) var<uniform> uniforms: TextUniforms;
@group(0) @binding(1) var msdf_atlas: texture_2d<f32>;
@group(0) @binding(2) var msdf_sampler: sampler;

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    let p = vec4<f32>(in.position, 0.0, 1.0);
    let col0 = uniforms.transform[0];
    let col1 = uniforms.transform[1];
    let col2 = uniforms.transform[2];
    let col3 = uniforms.transform[3];
    let pos = p.x * col0 + p.y * col1 + p.z * col2 + p.w * col3;
    out.position = pos;
    out.tex_coord = in.tex_coord;
    out.color = uniforms.color;
    return out;
}

fn median3(r: f32, g: f32, b: f32) -> f32 {
    return max(min(r, g), min(max(r, g), b));
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    return vec4<f32>(1.0, 0.0, 0.0, 1.0);
}
`
	// Compile per-entry-point (as DX12 backend does)
	for _, epName := range []string{"vs_main", "fs_main"} {
		lexer := wgsl.NewLexer(source)
		tokens, err := lexer.Tokenize()
		if err != nil {
			t.Fatalf("Tokenize failed: %v", err)
		}
		parser := wgsl.NewParser(tokens)
		ast, err := parser.Parse()
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}
		module, err := wgsl.LowerWithSource(ast, source)
		if err != nil {
			t.Fatalf("Lower failed: %v", err)
		}
		opts := hlsl.DefaultOptions()
		opts.EntryPoint = epName
		code, _, err := hlsl.Compile(module, opts)
		if err != nil {
			t.Fatalf("HLSL Compile (entry=%s) failed: %v", epName, err)
		}
		t.Logf("=== HLSL for entry point '%s' ===\n%s", epName, code)

		// Verify key elements present
		if !strings.Contains(code, "register(b0") {
			t.Errorf("entry=%s: missing cbuffer register(b0)", epName)
		}
		// Matrix member must have row_major qualifier
		assertContains(t, code, "row_major float4x4 transform")
	}
}

// =============================================================================
// Function call with result (typed variable declaration)
// =============================================================================

func TestE2E_FunctionCallWithResult(t *testing.T) {
	source := `
fn helper(x: f32) -> f32 {
    return x * 2.0;
}

@fragment
fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
    let result = helper(v);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// The call result must be declared with its type (float _crN = helper(...)).
	assertContains(t, code, "float _cr")
	assertContains(t, code, "= helper(")

	// Must NOT contain old-style bare undeclared assignment.
	assertNotContains(t, code, "_helper_result = helper(")

	// No stubs.
	assertNotContains(t, code, "// Function body (to be implemented)")

	t.Logf("Function call result HLSL output:\n%s", code)
}

// =============================================================================
// row_major on uniform matrix members
// =============================================================================

func TestE2E_MatrixRowMajor(t *testing.T) {
	source := `
struct Uniforms {
    model: mat4x4<f32>,
    view: mat4x4<f32>,
    proj: mat4x4<f32>,
    scale: f32,
}

@group(0) @binding(0) var<uniform> u: Uniforms;

@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    return vec4<f32>(pos * u.scale, 1.0);
}
`
	code := compileWGSLToHLSL(t, source)

	// All matrix members must have row_major prefix
	assertContains(t, code, "row_major float4x4 model")
	assertContains(t, code, "row_major float4x4 view")
	assertContains(t, code, "row_major float4x4 proj")

	// Non-matrix members must NOT have row_major prefix
	assertNotContains(t, code, "row_major float scale")

	t.Logf("HLSL output:\n%s", code)
}

// =============================================================================
// Matrix multiply reversal: mat * vec → mul(vec, mat)
// =============================================================================

func TestE2E_MatrixMulReversed(t *testing.T) {
	source := `
struct Camera {
    mvp: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> camera: Camera;

@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    let p = vec4<f32>(pos, 1.0);
    return camera.mvp * p;
}
`
	code := compileWGSLToHLSL(t, source)

	// mat4x4 * vec4 should become mul(vec4, mat4x4) — reversed args
	assertContains(t, code, "mul(")

	// The row_major qualifier should be present on the matrix member
	assertContains(t, code, "row_major float4x4 mvp")

	t.Logf("HLSL output:\n%s", code)
}
