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

	// Vertex entry point with input struct
	assertContains(t, code, "struct vs_main_Input")
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

	// Should have matrix type
	assertContains(t, code, "float4x4")

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

	// Input struct should have flattened struct members with semantics
	assertContains(t, code, "struct vs_main_Input")
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

	t.Logf("HLSL output:\n%s", code)
}
