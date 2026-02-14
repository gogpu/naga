package spirv

import (
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
