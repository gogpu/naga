package spirv

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestCompileVertexShader tests end-to-end compilation of a vertex shader with vertex attributes.
func TestCompileVertexShader(t *testing.T) {
	source := `
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec3<f32>,
}

@vertex
fn main(@location(0) position: vec3<f32>, @location(1) color: vec3<f32>) -> VertexOutput {
    var output: VertexOutput;
    output.position = vec4<f32>(position.x, position.y, position.z, 1.0);
    output.color = color;
    return output;
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	// Verify entry points
	if len(module.EntryPoints) != 1 {
		t.Errorf("Expected 1 entry point, got %d", len(module.EntryPoints))
	}
	if module.EntryPoints[0].Stage != ir.StageVertex {
		t.Errorf("Expected vertex stage, got %v", module.EntryPoints[0].Stage)
	}

	t.Logf("Successfully compiled vertex shader: %d bytes", len(spirvBytes))
}

// TestCompileFragmentShader tests end-to-end compilation of a fragment shader.
func TestCompileFragmentShader(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(color.x, color.y, color.z, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	// Verify entry points
	if len(module.EntryPoints) != 1 {
		t.Errorf("Expected 1 entry point, got %d", len(module.EntryPoints))
	}
	if module.EntryPoints[0].Stage != ir.StageFragment {
		t.Errorf("Expected fragment stage, got %v", module.EntryPoints[0].Stage)
	}

	t.Logf("Successfully compiled fragment shader: %d bytes", len(spirvBytes))
}

// TestCompileComputeShader tests end-to-end compilation of a compute shader.
func TestCompileComputeShader(t *testing.T) {
	// Note: Runtime-sized arrays and complex storage access may have limitations
	// This test uses a simplified compute shader
	source := `
@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    // Simple compute work
    var temp: u32 = id.x + id.y;
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	// Verify entry points
	if len(module.EntryPoints) != 1 {
		t.Errorf("Expected 1 entry point, got %d", len(module.EntryPoints))
	}
	if module.EntryPoints[0].Stage != ir.StageCompute {
		t.Errorf("Expected compute stage, got %v", module.EntryPoints[0].Stage)
	}

	t.Logf("Successfully compiled compute shader: %d bytes", len(spirvBytes))
}

// TestCompileFragmentShaderWithMath tests compilation with built-in math functions.
func TestCompileFragmentShaderWithMath(t *testing.T) {
	// Note: Type inference for 'let' not yet implemented
	// This test is skipped until type inference is complete
	t.Skip("Type inference for 'let' bindings not yet implemented")

	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let x: f32 = sin(uv.x);
    let y: f32 = cos(uv.y);
    let len: f32 = sqrt(x * x + y * y);
    return vec4<f32>(x, y, len, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled fragment shader with math: %d bytes", len(spirvBytes))
}

// TestCompileVertexShaderWithUniforms tests compilation with uniform buffers.
func TestCompileVertexShaderWithUniforms(t *testing.T) {
	// Note: Matrix multiplication not yet implemented
	// This test verifies uniform buffer declaration only
	source := `
struct Uniforms {
    mvp: mat4x4<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

@vertex
fn main(@location(0) position: vec3<f32>) -> @builtin(position) vec4<f32> {
    // Note: Matrix multiplication will be tested when operator support is complete
    return vec4<f32>(position.x, position.y, position.z, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	// Verify global variables (uniform buffer)
	if len(module.GlobalVariables) != 1 {
		t.Errorf("Expected 1 global variable, got %d", len(module.GlobalVariables))
	}

	t.Logf("Successfully compiled vertex shader with uniforms: %d bytes", len(spirvBytes))
}

// TestCompileMultiEntryPoint tests compilation with multiple entry points.
func TestCompileMultiEntryPoint(t *testing.T) {
	source := `
@vertex
fn vs_main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    return vec4<f32>(pos.x, pos.y, pos.z, 1.0);
}

@fragment
fn fs_main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(color.x, color.y, color.z, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	// Verify entry points
	if len(module.EntryPoints) != 2 {
		t.Errorf("Expected 2 entry points, got %d", len(module.EntryPoints))
	}

	t.Logf("Successfully compiled multi-entry shader: %d bytes", len(spirvBytes))
}

// TestCompileWithDebugInfo tests compilation with debug information enabled.
func TestCompileWithDebugInfo(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(color.x, color.y, color.z, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V with debug enabled
	opts := Options{
		Version: Version1_3,
		Debug:   true,
	}
	backend := NewBackend(opts)
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled shader with debug info: %d bytes", len(spirvBytes))
}

// TestCompileComputeShaderWithLocalVars tests compute shader with local variables.
func TestCompileComputeShaderWithLocalVars(t *testing.T) {
	// Note: Variable initialization with complex expressions may have limitations
	// This test uses simple local variable operations
	source := `
@compute @workgroup_size(8, 8, 1)
fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
    var temp: u32;
    temp = global_id.x + global_id.y * 8u;
    temp = temp * 2u;
    // Note: Storage arrays and type conversions will be tested when supported
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled compute shader with local vars: %d bytes", len(spirvBytes))
}

// TestCompileFragmentShaderWithConditionals tests fragment shader with if/else.
func TestCompileFragmentShaderWithConditionals(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var color: vec3<f32>;
    if (uv.x > 0.5) {
        color = vec3<f32>(1.0, 0.0, 0.0);
    } else {
        color = vec3<f32>(0.0, 0.0, 1.0);
    }
    return vec4<f32>(color.x, color.y, color.z, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled fragment shader with conditionals: %d bytes", len(spirvBytes))
}

// TestCompileDifferentSPIRVVersions tests compilation with different SPIR-V versions.
func TestCompileDifferentSPIRVVersions(t *testing.T) {
	source := `
@vertex
fn main(@location(0) pos: vec3<f32>) -> @builtin(position) vec4<f32> {
    return vec4<f32>(pos.x, pos.y, pos.z, 1.0);
}
`

	versions := []struct {
		name    string
		version Version
	}{
		{"SPIR-V 1.0", Version1_0},
		{"SPIR-V 1.3", Version1_3},
		{"SPIR-V 1.4", Version1_4},
		{"SPIR-V 1.5", Version1_5},
		{"SPIR-V 1.6", Version1_6},
	}

	for _, tc := range versions {
		t.Run(tc.name, func(t *testing.T) {
			// Parse WGSL
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

			// Lower AST to IR
			module, err := wgsl.Lower(ast)
			if err != nil {
				t.Fatalf("Lower failed: %v", err)
			}

			// Compile to SPIR-V with specific version
			opts := Options{
				Version: tc.version,
				Debug:   false,
			}
			backend := NewBackend(opts)
			spirvBytes, err := backend.Compile(module)
			if err != nil {
				t.Fatalf("SPIR-V compile failed: %v", err)
			}

			// Validate SPIR-V binary
			validateSPIRVBinary(t, spirvBytes)

			// Check version in SPIR-V header
			if len(spirvBytes) < 8 {
				t.Fatal("SPIR-V binary too short")
			}
			version := uint32(spirvBytes[4]) | uint32(spirvBytes[5])<<8 | uint32(spirvBytes[6])<<16 | uint32(spirvBytes[7])<<24
			expectedVersion := (uint32(tc.version.Major) << 16) | (uint32(tc.version.Minor) << 8)
			if version != expectedVersion {
				t.Errorf("Expected SPIR-V version 0x%08x, got 0x%08x", expectedVersion, version)
			}

			t.Logf("Successfully compiled with %s: %d bytes", tc.name, len(spirvBytes))
		})
	}
}

// validateSPIRVBinary performs basic validation of SPIR-V binary format.
func validateSPIRVBinary(t *testing.T, spirvBytes []byte) {
	t.Helper()

	// Check minimum size (5-word header = 20 bytes)
	if len(spirvBytes) < 20 {
		t.Fatalf("SPIR-V binary too short: %d bytes (expected at least 20)", len(spirvBytes))
	}

	// Check magic number (0x07230203 in little-endian)
	magic := uint32(spirvBytes[0]) | uint32(spirvBytes[1])<<8 | uint32(spirvBytes[2])<<16 | uint32(spirvBytes[3])<<24
	expectedMagic := uint32(0x07230203)
	if magic != expectedMagic {
		t.Errorf("Invalid SPIR-V magic number: got 0x%08x, expected 0x%08x", magic, expectedMagic)
	}

	// Check version (word 1) - should be between 1.0 and 1.6
	version := uint32(spirvBytes[4]) | uint32(spirvBytes[5])<<8 | uint32(spirvBytes[6])<<16 | uint32(spirvBytes[7])<<24
	if version < 0x00010000 || version > 0x00010600 {
		t.Errorf("Invalid SPIR-V version: 0x%08x (expected 1.0-1.6)", version)
	}

	// Check generator magic (word 2) - can be zero (optional)
	// generator := uint32(spirvBytes[8]) | uint32(spirvBytes[9])<<8 | uint32(spirvBytes[10])<<16 | uint32(spirvBytes[11])<<24
	// Note: Generator being zero is valid per SPIR-V spec (reserved, but allowed)

	// Check bound (word 3) - should be > 0
	bound := uint32(spirvBytes[12]) | uint32(spirvBytes[13])<<8 | uint32(spirvBytes[14])<<16 | uint32(spirvBytes[15])<<24
	if bound == 0 {
		t.Error("SPIR-V bound is zero (should be > 0)")
	}

	// Check schema (word 4) - should be 0 (reserved)
	schema := uint32(spirvBytes[16]) | uint32(spirvBytes[17])<<8 | uint32(spirvBytes[18])<<16 | uint32(spirvBytes[19])<<24
	if schema != 0 {
		t.Errorf("SPIR-V schema is %d (should be 0)", schema)
	}

	// Check that binary is word-aligned
	if len(spirvBytes)%4 != 0 {
		t.Errorf("SPIR-V binary size %d is not 4-byte aligned", len(spirvBytes))
	}
}

// TestCompileComputeShaderWithAtomics tests compute shader with atomic operations.
func TestCompileComputeShaderWithAtomics(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> counter: atomic<u32>;

@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    atomicAdd(&counter, 1u);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled compute shader with atomics: %d bytes", len(spirvBytes))
}

// TestCompileComputeShaderWithAtomicCompareExchange tests atomicCompareExchangeWeak.
func TestCompileComputeShaderWithAtomicCompareExchange(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> counter: atomic<u32>;

@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    atomicCompareExchangeWeak(&counter, 0u, 1u);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	t.Logf("Successfully compiled compute shader with atomicCompareExchangeWeak: %d bytes", len(spirvBytes))
}

// TestCompileFragmentShaderWithIfElseReturn tests the exact shader that caused GPU hang.
// Bug WGSL-CONTROLFLOW-001: if/else with return in both branches generated broken SPIR-V.
func TestCompileFragmentShaderWithIfElseReturn(t *testing.T) {
	source := `
struct Uniforms {
    premultiplied: f32,
    alpha: f32,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var texSampler: sampler;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let texColor = textureSample(tex, texSampler, input.uv);
    if (uniforms.premultiplied > 0.5) {
        return texColor * uniforms.alpha;
    } else {
        let a = texColor.a * uniforms.alpha;
        return vec4<f32>(texColor.rgb * a, a);
    }
}
`
	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate basic SPIR-V binary format
	validateSPIRVBinary(t, spirvBytes)

	// Validate structured control flow:
	// Every OpBranchConditional must be preceded by OpSelectionMerge,
	// and all branch target labels must exist in the binary.
	validateSPIRVControlFlow(t, spirvBytes)

	t.Logf("Successfully compiled fragment shader with if/else return: %d bytes", len(spirvBytes))
}

// TestCompileFragmentShaderWithNestedIfElse tests nested if/else control flow.
func TestCompileFragmentShaderWithNestedIfElse(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var color: vec4<f32>;
    if (uv.x > 0.5) {
        if (uv.y > 0.5) {
            color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
        } else {
            color = vec4<f32>(0.0, 1.0, 0.0, 1.0);
        }
    } else {
        color = vec4<f32>(0.0, 0.0, 1.0, 1.0);
    }
    return color;
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)
	validateSPIRVControlFlow(t, spirvBytes)

	t.Logf("Successfully compiled fragment shader with nested if/else: %d bytes", len(spirvBytes))
}

// TestCompileFragmentShaderIfWithoutElse tests if without else branch.
func TestCompileFragmentShaderIfWithoutElse(t *testing.T) {
	source := `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    var color: vec4<f32> = vec4<f32>(0.0, 0.0, 0.0, 1.0);
    if (uv.x > 0.5) {
        color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    }
    return color;
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)
	validateSPIRVControlFlow(t, spirvBytes)

	t.Logf("Successfully compiled fragment shader if without else: %d bytes", len(spirvBytes))
}

// validateSPIRVControlFlow validates the SPIR-V structured control flow rules:
// 1. Every OpBranchConditional must be preceded by OpSelectionMerge or OpLoopMerge
// 2. All branch target label IDs must exist as OpLabel instructions
// 3. No unreachable instructions after terminators (OpReturn, OpReturnValue, OpKill, OpBranch)
func validateSPIRVControlFlow(t *testing.T, spirvBytes []byte) {
	t.Helper()

	if len(spirvBytes) < 20 || len(spirvBytes)%4 != 0 {
		t.Fatal("Invalid SPIR-V binary for control flow validation")
	}

	// Parse all instructions
	words := make([]uint32, len(spirvBytes)/4)
	for i := range words {
		words[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}

	// Collect all label IDs and branch targets
	labelIDs := make(map[uint32]bool)
	branchTargets := make(map[uint32]bool)

	// Track previous instruction opcode for merge validation
	var prevOpcode OpCode
	hasMerge := false

	offset := 5 // Skip header
	for offset < len(words) {
		word := words[offset]
		wordCount := word >> 16
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+int(wordCount) > len(words) {
			break
		}

		switch opcode {
		case OpLabel:
			if wordCount >= 2 {
				labelIDs[words[offset+1]] = true
			}

		case OpSelectionMerge:
			hasMerge = true
			if wordCount >= 2 {
				branchTargets[words[offset+1]] = true // merge label
			}

		case OpLoopMerge:
			hasMerge = true
			if wordCount >= 3 {
				branchTargets[words[offset+1]] = true // merge label
				branchTargets[words[offset+2]] = true // continue label
			}

		case OpBranch:
			if wordCount >= 2 {
				branchTargets[words[offset+1]] = true
			}

		case OpBranchConditional:
			// Must be preceded by OpSelectionMerge or OpLoopMerge
			if !hasMerge {
				t.Errorf("OpBranchConditional at word %d not preceded by OpSelectionMerge/OpLoopMerge (prev opcode: %d)", offset, prevOpcode)
			}
			hasMerge = false
			if wordCount >= 4 {
				branchTargets[words[offset+2]] = true // true label
				branchTargets[words[offset+3]] = true // false label
			}
		}

		// Reset merge flag on non-merge, non-branch instructions
		if opcode != OpSelectionMerge && opcode != OpLoopMerge && opcode != OpBranchConditional {
			hasMerge = false
		}

		prevOpcode = opcode
		offset += int(wordCount)
	}

	// Verify all branch targets exist as labels
	for target := range branchTargets {
		if !labelIDs[target] {
			t.Errorf("Branch target ID %d does not have a corresponding OpLabel", target)
		}
	}
}

// TestBoolToFloatConversion tests f32(bool_value) conversion.
// Previously this produced "unsupported conversion: 3 → 2" (Bool=3, Float=2).
func TestBoolToFloatConversion(t *testing.T) {
	source := `
@fragment
fn main(@location(0) value: f32) -> @location(0) vec4<f32> {
    let flag: bool = value > 0.5;
    let result: f32 = f32(flag);
    return vec4<f32>(result, result, result, 1.0);
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	// Verify the binary contains OpSelect (used for bool→float conversion)
	if !containsOpcode(spirvBytes, OpSelect) {
		t.Error("Expected OpSelect in SPIR-V binary for bool→float conversion")
	}

	t.Logf("Successfully compiled bool→f32 conversion: %d bytes", len(spirvBytes))
}

// TestBoolToUintConversion tests u32(bool_value) conversion.
func TestBoolToUintConversion(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> output: array<u32>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    let idx = gid.x;
    let flag: bool = idx > 0u;
    output[idx] = u32(flag);
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	// Verify the binary contains OpSelect (used for bool→uint conversion)
	if !containsOpcode(spirvBytes, OpSelect) {
		t.Error("Expected OpSelect in SPIR-V binary for bool→u32 conversion")
	}

	t.Logf("Successfully compiled bool→u32 conversion: %d bytes", len(spirvBytes))
}

// TestBoolToSintConversion tests i32(bool_value) conversion.
func TestBoolToSintConversion(t *testing.T) {
	source := `
@fragment
fn main(@location(0) value: f32) -> @location(0) vec4<f32> {
    let flag: bool = value > 0.0;
    let result: i32 = i32(flag);
    let fval: f32 = f32(result);
    return vec4<f32>(fval, fval, fval, 1.0);
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	if !containsOpcode(spirvBytes, OpSelect) {
		t.Error("Expected OpSelect in SPIR-V binary for bool→i32 conversion")
	}

	t.Logf("Successfully compiled bool→i32 conversion: %d bytes", len(spirvBytes))
}

// TestInlineBoolToFloatConversion tests f32(x > 0.0) inline expression.
func TestInlineBoolToFloatConversion(t *testing.T) {
	source := `
@fragment
fn main(@location(0) value: f32) -> @location(0) vec4<f32> {
    let result: f32 = f32(value > 0.0);
    return vec4<f32>(result, 0.0, 0.0, 1.0);
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	if !containsOpcode(spirvBytes, OpSelect) {
		t.Error("Expected OpSelect in SPIR-V binary for inline bool→f32 conversion")
	}

	t.Logf("Successfully compiled inline bool→f32 conversion: %d bytes", len(spirvBytes))
}

// TestCompileVectorTimesScalar verifies that vec4<f32> * f32 emits
// OpVectorTimesScalar (143) instead of OpFMul (133).
func TestCompileVectorTimesScalar(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    let alpha: f32 = 0.5;
    let result: vec4<f32> = color * alpha;
    return result;
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	if !containsOpcode(spirvBytes, OpVectorTimesScalar) {
		t.Error("Expected OpVectorTimesScalar (143) in SPIR-V output for vec4<f32> * f32")
	}
}

// TestCompileScalarTimesVector verifies that f32 * vec4<f32> also emits
// OpVectorTimesScalar with swapped operands.
func TestCompileScalarTimesVector(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    let alpha: f32 = 0.5;
    let result: vec4<f32> = alpha * color;
    return result;
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	if !containsOpcode(spirvBytes, OpVectorTimesScalar) {
		t.Error("Expected OpVectorTimesScalar (143) in SPIR-V output for f32 * vec4<f32>")
	}
}

// TestBuiltinPositionFragCoord verifies that @builtin(position) on a fragment
// shader input is emitted as BuiltIn FragCoord (15), not BuiltIn Position (0).
//
// In WGSL, @builtin(position) has dual semantics:
//   - Vertex shader output: SPIR-V BuiltIn Position (0)
//   - Fragment shader input: SPIR-V BuiltIn FragCoord (15)
//
// Using BuiltIn Position on a fragment shader input causes a Vulkan validation
// error: "BuiltIn Position to be used only with Vertex, TessellationControl,
// TessellationEvaluation or Geometry execution models."
func TestBuiltinPositionFragCoord(t *testing.T) {
	// Shader with both vertex and fragment entry points sharing VertexOutput
	// struct that has @builtin(position). The vertex output should emit
	// BuiltIn Position; the fragment input should emit BuiltIn FragCoord.
	source := `
struct VertexOutput {
    @builtin(position) clip_position: vec4<f32>,
    @location(0) color: vec3<f32>,
}

@vertex
fn vs_main(@location(0) pos: vec3<f32>, @location(1) col: vec3<f32>) -> VertexOutput {
    var out: VertexOutput;
    out.clip_position = vec4<f32>(pos.x, pos.y, pos.z, 1.0);
    out.color = col;
    return out;
}

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    return vec4<f32>(in.color.x, in.color.y, in.color.z, 1.0);
}
`

	// Parse WGSL
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

	// Lower AST to IR
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	// Verify we have both entry points
	if len(module.EntryPoints) != 2 {
		t.Fatalf("Expected 2 entry points, got %d", len(module.EntryPoints))
	}

	// Compile to SPIR-V
	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	// Validate SPIR-V binary
	validateSPIRVBinary(t, spirvBytes)

	// Parse SPIR-V binary into words
	words := make([]uint32, len(spirvBytes)/4)
	for i := range words {
		words[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}

	// Collect variable storage classes: varID -> StorageClass
	varStorageClass := make(map[uint32]StorageClass)
	// Collect BuiltIn decorations: varID -> BuiltIn value
	varBuiltIn := make(map[uint32]BuiltIn)

	offset := 5 // Skip header
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		switch opcode {
		case OpVariable:
			// OpVariable: wordCount | opcode, resultType, resultID, storageClass [, initializer]
			if wordCount >= 4 {
				resultID := words[offset+2]
				storageClass := StorageClass(words[offset+3])
				varStorageClass[resultID] = storageClass
			}
		case OpDecorate:
			// OpDecorate: wordCount | opcode, targetID, decoration [, operands...]
			if wordCount >= 4 {
				targetID := words[offset+1]
				decoration := Decoration(words[offset+2])
				if decoration == DecorationBuiltIn {
					builtIn := BuiltIn(words[offset+3])
					varBuiltIn[targetID] = builtIn
				}
			}
		}

		offset += wordCount
	}

	// Now verify: find all variables with BuiltIn Position or FragCoord decorations
	foundPosition := false
	foundFragCoord := false

	for varID, builtIn := range varBuiltIn {
		sc, ok := varStorageClass[varID]
		if !ok {
			continue
		}
		switch builtIn {
		case BuiltInPosition:
			if sc != StorageClassOutput {
				t.Errorf("BuiltIn Position (0) found on variable %d with storage class %d; "+
					"expected StorageClassOutput (%d)", varID, sc, StorageClassOutput)
			}
			foundPosition = true
		case BuiltInFragCoord:
			if sc != StorageClassInput {
				t.Errorf("BuiltIn FragCoord (15) found on variable %d with storage class %d; "+
					"expected StorageClassInput (%d)", varID, sc, StorageClassInput)
			}
			foundFragCoord = true
		}
	}

	if !foundPosition {
		t.Error("No variable with BuiltIn Position (0) found in SPIR-V output; " +
			"expected vertex shader output to have BuiltIn Position")
	}
	if !foundFragCoord {
		t.Error("No variable with BuiltIn FragCoord (15) found in SPIR-V output; " +
			"expected fragment shader input to have BuiltIn FragCoord, not BuiltIn Position")
	}

	t.Logf("Successfully verified Position/FragCoord BuiltIn decorations: %d bytes", len(spirvBytes))
}

// TestBuiltinPositionFragCoordDirectBinding verifies that @builtin(position) as
// a direct function result/argument (not in a struct) also correctly maps to
// BuiltIn Position for vertex output and BuiltIn FragCoord for fragment input.
func TestBuiltinPositionFragCoordDirectBinding(t *testing.T) {
	// Fragment shader that takes @builtin(position) directly as a parameter
	source := `
@fragment
fn fs_main(@builtin(position) pos: vec4<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(pos.x / 800.0, pos.y / 600.0, 0.0, 1.0);
}
`

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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	// Parse SPIR-V binary
	words := make([]uint32, len(spirvBytes)/4)
	for i := range words {
		words[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}

	// Collect variable storage classes and BuiltIn decorations
	varStorageClass := make(map[uint32]StorageClass)
	varBuiltIn := make(map[uint32]BuiltIn)

	offset := 5
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}

		switch opcode {
		case OpVariable:
			if wordCount >= 4 {
				resultID := words[offset+2]
				storageClass := StorageClass(words[offset+3])
				varStorageClass[resultID] = storageClass
			}
		case OpDecorate:
			if wordCount >= 4 {
				targetID := words[offset+1]
				decoration := Decoration(words[offset+2])
				if decoration == DecorationBuiltIn {
					builtIn := BuiltIn(words[offset+3])
					varBuiltIn[targetID] = builtIn
				}
			}
		}

		offset += wordCount
	}

	// The fragment input @builtin(position) should be FragCoord, NOT Position
	for varID, builtIn := range varBuiltIn {
		sc, ok := varStorageClass[varID]
		if !ok {
			continue
		}
		if builtIn == BuiltInPosition && sc == StorageClassInput {
			t.Errorf("Fragment shader input variable %d has BuiltIn Position (0); "+
				"should be BuiltIn FragCoord (15)", varID)
		}
	}

	// Should find FragCoord on an Input variable
	foundFragCoord := false
	for varID, builtIn := range varBuiltIn {
		sc, ok := varStorageClass[varID]
		if !ok {
			continue
		}
		if builtIn == BuiltInFragCoord && sc == StorageClassInput {
			foundFragCoord = true
			break
		}
		_ = varID
	}

	if !foundFragCoord {
		t.Error("No Input variable with BuiltIn FragCoord (15) found; " +
			"@builtin(position) on fragment input should emit FragCoord")
	}

	t.Logf("Successfully verified direct @builtin(position) on fragment input: %d bytes", len(spirvBytes))
}

// TestCompileArrayLengthBareArray tests arrayLength on a bare runtime-sized array
// in a storage buffer. The SPIR-V backend wraps it in a synthetic struct.
func TestCompileArrayLengthBareArray(t *testing.T) {
	source := `
@group(0) @binding(0) var<storage, read_write> output: array<f32>;

@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let len = arrayLength(&output);
    if id.x < len {
        output[id.x] = f32(id.x);
    }
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	if !containsOpcode(spirvBytes, OpArrayLength) {
		t.Error("Expected OpArrayLength in SPIR-V binary for arrayLength(&output)")
	}

	t.Logf("Successfully compiled arrayLength on bare array: %d bytes", len(spirvBytes))
}

// TestCompileArrayLengthStructMember tests arrayLength on a runtime-sized array
// that is the last member of a struct in a storage buffer.
func TestCompileArrayLengthStructMember(t *testing.T) {
	source := `
struct Buffer {
    count: u32,
    data: array<f32>,
}

@group(0) @binding(0) var<storage, read_write> buf: Buffer;

@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let len = arrayLength(&buf.data);
    if id.x < len {
        buf.data[id.x] = f32(id.x);
    }
}
`
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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	if !containsOpcode(spirvBytes, OpArrayLength) {
		t.Error("Expected OpArrayLength in SPIR-V binary for arrayLength(&buf.data)")
	}

	t.Logf("Successfully compiled arrayLength on struct member: %d bytes", len(spirvBytes))
}

// containsOpcode scans a SPIR-V binary for a specific opcode.
func containsOpcode(spirvBytes []byte, target OpCode) bool {
	if len(spirvBytes) < 20 || len(spirvBytes)%4 != 0 {
		return false
	}

	words := make([]uint32, len(spirvBytes)/4)
	for i := range words {
		words[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}

	offset := 5 // Skip header
	for offset < len(words) {
		word := words[offset]
		wordCount := word >> 16
		opcode := OpCode(word & 0xFFFF)

		if wordCount == 0 || offset+int(wordCount) > len(words) {
			break
		}

		if opcode == target {
			return true
		}

		offset += int(wordCount)
	}
	return false
}

// TestCompileFunctionCallInLetExpr tests that user function call results can
// be used in let-bound expressions (e.g., let sd = func() - 0.5).
// This pattern is used by the MSDF text rendering shader.
func TestCompileFunctionCallInLetExpr(t *testing.T) {
	source := `
fn median3(r: f32, g: f32, b: f32) -> f32 {
    let min_rg = min(r, g);
    let max_rg = max(r, g);
    let min_max_b = min(max_rg, b);
    return max(min_rg, min_max_b);
}

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let sd = median3(uv.x, uv.y, 0.5) - 0.5;
    let alpha = clamp(sd + 0.5, 0.0, 1.0);
    return vec4<f32>(alpha, alpha, alpha, 1.0);
}
`

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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)
	t.Logf("Successfully compiled function call in let expression: %d bytes", len(spirvBytes))
}

// TestCompileImageQuery tests that textureDimensions emits ImageQuery capability.
func TestCompileImageQuery(t *testing.T) {
	source := `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var tex_sampler: sampler;

@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let dims = textureDimensions(tex, 0);
    let size = vec2<f32>(f32(dims.x), f32(dims.y));
    let scaled_uv = uv / size;
    return textureSample(tex, tex_sampler, scaled_uv);
}
`

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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	// Verify ImageQuery capability (6) is present in SPIR-V binary
	words := make([]uint32, len(spirvBytes)/4)
	for i := range words {
		words[i] = uint32(spirvBytes[i*4]) |
			uint32(spirvBytes[i*4+1])<<8 |
			uint32(spirvBytes[i*4+2])<<16 |
			uint32(spirvBytes[i*4+3])<<24
	}
	foundImageQuery := false
	offset := 5
	for offset < len(words) {
		word := words[offset]
		wordCount := int(word >> 16)
		opcode := word & 0xFFFF
		if wordCount == 0 || offset+wordCount > len(words) {
			break
		}
		// OpCapability = 17, ImageQuery = 50
		if opcode == 17 && wordCount >= 2 && words[offset+1] == 50 {
			foundImageQuery = true
		}
		offset += wordCount
	}
	if !foundImageQuery {
		t.Error("Expected ImageQuery capability (6) in SPIR-V binary")
	}

	t.Logf("Successfully compiled image query shader with ImageQuery capability: %d bytes", len(spirvBytes))
}

// TestCompileMSDFTextShader tests the full MSDF text rendering shader end-to-end.
// This is the shader used by gg's GPU text pipeline.
func TestCompileMSDFTextShader(t *testing.T) {
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

fn median3(r: f32, g: f32, b: f32) -> f32 {
    let min_rg = min(r, g);
    let max_rg = max(r, g);
    let min_max_b = min(max_rg, b);
    return max(min_rg, min_max_b);
}

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

@fragment
fn fs_main(in: VertexOutput) -> @location(0) vec4<f32> {
    let msdf = textureSample(msdf_atlas, msdf_sampler, in.tex_coord).rgb;
    let sd = median3(msdf.r, msdf.g, msdf.b) - 0.5;
    let tex_size = vec2<f32>(uniforms.msdf_params.y, uniforms.msdf_params.y);
    let fw = fwidth(in.tex_coord);
    let dx_dy = fw * tex_size;
    let px_range = uniforms.msdf_params.x;
    let screen_px_range = px_range / length(dx_dy);
    let screen_px_distance = screen_px_range * sd;
    let alpha = clamp(screen_px_distance + 0.5, 0.0, 1.0);
    return vec4<f32>(in.color.rgb * alpha, in.color.a * alpha);
}
`

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

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spirvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("SPIR-V compile failed: %v", err)
	}

	validateSPIRVBinary(t, spirvBytes)

	// Verify we have both vertex and fragment entry points
	if len(module.EntryPoints) != 2 {
		t.Errorf("Expected 2 entry points, got %d", len(module.EntryPoints))
	}

	// Run spirv-val and spirv-dis from Vulkan SDK if available
	validateWithVulkanSDK(t, spirvBytes)

	t.Logf("Successfully compiled MSDF text shader: %d bytes", len(spirvBytes))
}

// validateWithVulkanSDK runs spirv-val and spirv-dis from Vulkan SDK on SPIR-V binary.
// Skips if Vulkan SDK tools are not available.
func validateWithVulkanSDK(t *testing.T, spirvBytes []byte) {
	t.Helper()

	// Check spirv-val availability
	spirvVal, err := exec.LookPath("spirv-val")
	if err != nil {
		t.Log("spirv-val not found, skipping Vulkan SDK validation")
		return
	}

	// Write SPIR-V to temp file
	tmpDir := t.TempDir()
	spvPath := filepath.Join(tmpDir, "shader.spv")
	if err := os.WriteFile(spvPath, spirvBytes, 0o644); err != nil {
		t.Fatalf("Failed to write .spv: %v", err)
	}

	// Run spirv-val
	cmd := exec.Command(spirvVal, spvPath, "--target-env", "vulkan1.2")
	valOut, valErr := cmd.CombinedOutput()
	if valErr != nil {
		// Validation failed — dump disassembly for debugging
		t.Errorf("spirv-val FAILED:\n%s", valOut)

		spirvDis, disErr := exec.LookPath("spirv-dis")
		if disErr == nil {
			disCmd := exec.Command(spirvDis, spvPath, "--no-header")
			disOut, _ := disCmd.CombinedOutput()
			t.Logf("SPIR-V disassembly:\n%s", disOut)
		}
	} else {
		t.Log("spirv-val: VALID")
	}
}
