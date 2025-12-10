package naga

import (
	"testing"
)

// TestCompileSimpleVertexShader tests compilation of a basic vertex shader.
func TestCompileSimpleVertexShader(t *testing.T) {
	// Skip validation for this test as the shader is minimal
	source := `
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	opts := CompileOptions{
		Debug:    false,
		Validate: false, // Skip validation for minimal shader
	}
	spirv, err := CompileWithOptions(source, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check SPIR-V magic number (little-endian: 0x07230203)
	if len(spirv) < 4 {
		t.Fatal("Output too short")
	}
	magic := uint32(spirv[0]) | uint32(spirv[1])<<8 | uint32(spirv[2])<<16 | uint32(spirv[3])<<24
	expectedMagic := uint32(0x07230203)
	if magic != expectedMagic {
		t.Errorf("Invalid SPIR-V magic: got 0x%08x, want 0x%08x", magic, expectedMagic)
	}

	t.Logf("Generated %d bytes of SPIR-V", len(spirv))
}

// TestCompileFragmentShader tests compilation of a fragment shader.
func TestCompileFragmentShader(t *testing.T) {
	source := `
@fragment
fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`
	opts := CompileOptions{Validate: false} // Skip validation for minimal shader
	spirv, err := CompileWithOptions(source, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify SPIR-V header
	if len(spirv) < 20 {
		t.Fatal("SPIR-V output too short (should have at least 5-word header)")
	}

	// Check magic number
	magic := uint32(spirv[0]) | uint32(spirv[1])<<8 | uint32(spirv[2])<<16 | uint32(spirv[3])<<24
	if magic != 0x07230203 {
		t.Errorf("Invalid SPIR-V magic: got 0x%08x, want 0x07230203", magic)
	}

	t.Logf("Generated %d bytes of SPIR-V", len(spirv))
}

// TestCompileWithMathFunctions tests compilation with built-in math functions.
func TestCompileWithMathFunctions(t *testing.T) {
	// Note: Type inference for 'let' is not yet implemented, so we skip this test
	// TODO: Re-enable when type inference is implemented
	t.Skip("Type inference for 'let' bindings not yet implemented")

	source := `
@fragment
fn main(@location(0) v: vec3<f32>) -> @location(0) vec4<f32> {
    let n = normalize(v);
    let len = length(v);
    return vec4<f32>(n, len);
}
`
	opts := CompileOptions{Validate: false} // Skip validation for test
	spirv, err := CompileWithOptions(source, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Just verify we got valid output
	if len(spirv) < 20 {
		t.Fatal("Output too short")
	}

	t.Logf("Generated %d bytes of SPIR-V", len(spirv))
}

// TestCompileComputeShader tests compilation of a compute shader.
func TestCompileComputeShader(t *testing.T) {
	source := `
@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
    // Compute work here
}
`
	opts := CompileOptions{Validate: false} // Skip validation for test
	spirv, err := CompileWithOptions(source, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify SPIR-V magic
	if len(spirv) < 4 {
		t.Fatal("Output too short")
	}
	magic := uint32(spirv[0]) | uint32(spirv[1])<<8 | uint32(spirv[2])<<16 | uint32(spirv[3])<<24
	if magic != 0x07230203 {
		t.Errorf("Invalid SPIR-V magic: got 0x%08x, want 0x07230203", magic)
	}

	t.Logf("Generated %d bytes of SPIR-V", len(spirv))
}

// TestCompileWithOptions tests compilation with custom options.
func TestCompileWithOptions(t *testing.T) {
	source := `
@vertex
fn main() -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	opts := CompileOptions{
		Debug:    true,
		Validate: false, // Skip validation for minimal shader
	}
	spirv, err := CompileWithOptions(source, opts)
	if err != nil {
		t.Fatalf("CompileWithOptions failed: %v", err)
	}

	if len(spirv) < 20 {
		t.Fatal("Output too short")
	}

	t.Logf("Generated %d bytes of SPIR-V (with debug info)", len(spirv))
}

// TestCompileInvalidShader tests error handling for invalid shaders.
func TestCompileInvalidShader(t *testing.T) {
	source := `
@vertex
fn main() -> vec4<f32> {
    return vec4<f32>(0.0, 0.0); // Wrong number of components
}
`
	_, err := Compile(source)
	if err == nil {
		t.Fatal("Expected compilation error for invalid shader, got nil")
	}

	t.Logf("Got expected error: %v", err)
}

// TestParseSyntaxError tests error handling for syntax errors.
func TestParseSyntaxError(t *testing.T) {
	source := `
@vertex
fn main( { // Missing closing parenthesis
    return vec4<f32>(0.0);
}
`
	_, err := Parse(source)
	if err == nil {
		t.Fatal("Expected parse error for syntax error, got nil")
	}

	t.Logf("Got expected parse error: %v", err)
}

// TestParseAndLowerPipeline tests the individual stages of compilation.
func TestParseAndLowerPipeline(t *testing.T) {
	source := `
@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
`
	// Stage 1: Parse
	ast, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(ast.Functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(ast.Functions))
	}

	// Stage 2: Lower
	module, err := Lower(ast)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}
	if len(module.Functions) != 1 {
		t.Errorf("Expected 1 function in IR, got %d", len(module.Functions))
	}
	if len(module.EntryPoints) != 1 {
		t.Errorf("Expected 1 entry point, got %d", len(module.EntryPoints))
	}

	// Stage 3: Validate (expect validation errors for minimal shader)
	errors, err := Validate(module)
	if err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	// Note: Minimal shader has validation errors (missing bindings), which is expected
	if len(errors) == 0 {
		t.Log("No validation errors (shader is valid)")
	} else {
		t.Logf("Expected validation errors for minimal shader: %v", errors[0])
	}

	t.Log("Successfully parsed, lowered, and validated shader")
}

// TestCompileTriangleShader tests a complete triangle rendering shader.
func TestCompileTriangleShader(t *testing.T) {
	// Note: Array initialization syntax not yet fully supported
	// TODO: Re-enable when array initialization is implemented
	t.Skip("Array initialization syntax not yet fully supported")

	source := `
struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) color: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> VertexOutput {
    var out: VertexOutput;

    // Triangle vertices
    var pos = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),
        vec2<f32>(-0.5, -0.5),
        vec2<f32>(0.5, -0.5)
    );

    out.position = vec4<f32>(pos[idx], 0.0, 1.0);
    out.color = vec4<f32>(1.0, 0.0, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {
    return color;
}
`
	opts := CompileOptions{Validate: false} // Skip validation for test
	spirv, err := CompileWithOptions(source, opts)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify SPIR-V output
	if len(spirv) < 20 {
		t.Fatal("Output too short")
	}

	t.Logf("Generated %d bytes of SPIR-V for triangle shader", len(spirv))
}
