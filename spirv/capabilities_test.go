package spirv

import (
	"encoding/binary"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// compileWGSLForCapabilityTest compiles a WGSL source string to SPIR-V binary bytes.
// It runs the full pipeline: tokenize -> parse -> lower -> SPIR-V backend.
func compileWGSLForCapabilityTest(t *testing.T, source string) []byte {
	t.Helper()

	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	parser := wgsl.NewParser(tokens)
	ast, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatalf("Lower: %v", err)
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	return spvBytes
}

// extractCapabilities parses a SPIR-V binary and returns the set of all
// OpCapability operand values found.
//
// SPIR-V binary layout:
//   - Header: 5 words (20 bytes) — magic, version, generator, bound, schema
//   - Instructions: each starts with (wordCount << 16 | opcode)
//   - OpCapability (opcode 17) is 2 words: [header, capability_id]
func extractCapabilities(spvBytes []byte) map[uint32]bool {
	caps := make(map[uint32]bool)
	if len(spvBytes) < 20 {
		return caps
	}

	offset := 20 // skip 5-word header
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := word >> 16

		if wordCount == 0 || offset+int(wordCount)*4 > len(spvBytes) {
			break
		}

		if opcode == uint32(OpCapability) && wordCount >= 2 {
			capID := binary.LittleEndian.Uint32(spvBytes[offset+4:])
			caps[capID] = true
		}

		offset += int(wordCount) * 4
	}
	return caps
}

// capabilityName returns a human-readable name for a capability ID.
// Used in test error messages for clarity.
func capabilityNameForTest(c uint32) string {
	switch Capability(c) {
	case CapabilityMatrix:
		return "Matrix"
	case CapabilityShader:
		return "Shader"
	case CapabilityFloat16:
		return "Float16"
	case CapabilityFloat64:
		return "Float64"
	case CapabilityInt64:
		return "Int64"
	case CapabilityInt16:
		return "Int16"
	case CapabilityInt8:
		return "Int8"
	case CapabilityImageQuery:
		return "ImageQuery"
	case CapabilityDotProductInput4x8BitPacked:
		return "DotProductInput4x8BitPacked"
	case CapabilityDotProduct:
		return "DotProduct"
	default:
		return "Unknown"
	}
}

// assertCapability checks that a specific capability is present in the set.
func assertCapability(t *testing.T, caps map[uint32]bool, capability Capability) {
	t.Helper()
	if !caps[uint32(capability)] {
		t.Errorf("expected capability %s (%d) to be present", capabilityNameForTest(uint32(capability)), capability)
	}
}

// assertNoCapability checks that a specific capability is NOT present in the set.
func assertNoCapability(t *testing.T, caps map[uint32]bool, capability Capability) {
	t.Helper()
	if caps[uint32(capability)] {
		t.Errorf("expected capability %s (%d) to NOT be present", capabilityNameForTest(uint32(capability)), capability)
	}
}

// TestCapability_ShaderAlwaysPresent verifies that the Shader capability is
// emitted for all shader stages. Every valid SPIR-V module targeting Vulkan
// must declare OpCapability Shader.
func TestCapability_ShaderAlwaysPresent(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "compute shader",
			source: `@compute @workgroup_size(1) fn main() {}`,
		},
		{
			name: "vertex shader",
			source: `@vertex
fn main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}`,
		},
		{
			name: "fragment shader",
			source: `@fragment
fn main(@location(0) color: vec3<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(color.x, color.y, color.z, 1.0);
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spvBytes := compileWGSLForCapabilityTest(t, tt.source)
			caps := extractCapabilities(spvBytes)

			assertCapability(t, caps, CapabilityShader)
		})
	}
}

// TestCapability_Float16 verifies that the Float16 capability is emitted when
// the shader uses f16 types (enabled via "enable f16;" in WGSL).
func TestCapability_Float16(t *testing.T) {
	source := `enable f16;

@compute @workgroup_size(1)
fn main() {
    var x: f16 = 1.0h;
    _ = x;
}
`
	spvBytes := compileWGSLForCapabilityTest(t, source)
	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityFloat16)
}

// TestCapability_Float64_ViaIR verifies that the Float64 capability is emitted
// when a 64-bit float type appears in the IR module. WGSL does not expose f64,
// so we construct the IR directly.
func TestCapability_Float64_ViaIR(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f64", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityFloat64)
}

// TestCapability_Int16_ViaIR verifies that the Int16 capability is emitted
// when a 16-bit integer type appears in the IR module.
func TestCapability_Int16_ViaIR(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i16", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 2}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityInt16)
}

// TestCapability_Int64_ViaIR verifies that the Int64 capability is emitted
// when a 64-bit integer type appears in the IR module.
func TestCapability_Int64_ViaIR(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i64", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityInt64)
}

// TestCapability_Int8_ViaIR verifies that the Int8 capability is emitted
// when an 8-bit integer type appears in the IR module.
func TestCapability_Int8_ViaIR(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u8", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 1}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityInt8)
}

// TestCapability_ImageQuery verifies that the ImageQuery capability is emitted
// when the shader uses textureDimensions or similar image query builtins.
func TestCapability_ImageQuery(t *testing.T) {
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
	spvBytes := compileWGSLForCapabilityTest(t, source)
	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityImageQuery)
}

// TestCapability_DotProduct verifies that the DotProduct and
// DotProductInput4x8BitPacked capabilities are emitted when the shader
// uses dot4I8Packed or dot4U8Packed builtins.
func TestCapability_DotProduct(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "dot4I8Packed",
			source: `@compute @workgroup_size(1)
fn main() {
    let a: u32 = 0x01020304u;
    let b: u32 = 0x05060708u;
    let result: i32 = dot4I8Packed(a, b);
    _ = result;
}`,
		},
		{
			name: "dot4U8Packed",
			source: `@compute @workgroup_size(1)
fn main() {
    let a: u32 = 0x01020304u;
    let b: u32 = 0x05060708u;
    let result: u32 = dot4U8Packed(a, b);
    _ = result;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spvBytes := compileWGSLForCapabilityTest(t, tt.source)
			caps := extractCapabilities(spvBytes)

			assertCapability(t, caps, CapabilityShader)
			assertCapability(t, caps, CapabilityDotProduct)
			assertCapability(t, caps, CapabilityDotProductInput4x8BitPacked)
		})
	}
}

// TestCapability_NotEmittedWhenUnused verifies that optional capabilities
// are NOT emitted when the corresponding features are not used.
// A minimal compute shader should only have the Shader capability.
func TestCapability_NotEmittedWhenUnused(t *testing.T) {
	source := `@compute @workgroup_size(1) fn main() {}`
	spvBytes := compileWGSLForCapabilityTest(t, source)
	caps := extractCapabilities(spvBytes)

	// Shader must be present.
	assertCapability(t, caps, CapabilityShader)

	// All optional capabilities must NOT be present in a minimal shader.
	optionalCaps := []struct {
		cap  Capability
		name string
	}{
		{CapabilityFloat16, "Float16"},
		{CapabilityFloat64, "Float64"},
		{CapabilityInt64, "Int64"},
		{CapabilityInt16, "Int16"},
		{CapabilityInt8, "Int8"},
		{CapabilityImageQuery, "ImageQuery"},
		{CapabilityDotProductInput4x8BitPacked, "DotProductInput4x8BitPacked"},
		{CapabilityDotProduct, "DotProduct"},
	}

	for _, oc := range optionalCaps {
		assertNoCapability(t, caps, oc.cap)
	}
}

// TestCapability_UserRequestedViaOptions verifies that capabilities specified
// in Options.Capabilities are included in the output even if the shader
// does not use the corresponding features.
func TestCapability_UserRequestedViaOptions(t *testing.T) {
	module := &ir.Module{
		Types:           []ir.Type{},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	opts := DefaultOptions()
	opts.Capabilities = []Capability{CapabilityFloat64, CapabilityInt64}

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityFloat64)
	assertCapability(t, caps, CapabilityInt64)

	// Features not requested should not appear.
	assertNoCapability(t, caps, CapabilityFloat16)
	assertNoCapability(t, caps, CapabilityInt8)
}

// TestCapability_NoDuplicates verifies that each capability appears exactly
// once in the SPIR-V binary, even when the same type is used multiple times
// or both via Options and feature usage.
func TestCapability_NoDuplicates(t *testing.T) {
	// Request Float64 via options AND use it in types.
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f64_a", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
			{Name: "f64_b", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	opts := DefaultOptions()
	opts.Capabilities = []Capability{CapabilityFloat64}

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Count OpCapability instructions for Float64.
	count := 0
	offset := 20 // skip header
	for offset+4 <= len(spvBytes) {
		word := binary.LittleEndian.Uint32(spvBytes[offset:])
		opcode := word & 0xFFFF
		wordCount := word >> 16

		if wordCount == 0 || offset+int(wordCount)*4 > len(spvBytes) {
			break
		}

		if opcode == uint32(OpCapability) && wordCount >= 2 {
			capID := binary.LittleEndian.Uint32(spvBytes[offset+4:])
			if Capability(capID) == CapabilityFloat64 {
				count++
			}
		}

		offset += int(wordCount) * 4
	}

	if count != 1 {
		t.Errorf("expected exactly 1 OpCapability Float64, got %d", count)
	}
}

// TestCapability_ExtractFromEmptyBinary verifies that extractCapabilities
// handles edge cases gracefully.
func TestCapability_ExtractEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantSize int
	}{
		{"nil input", nil, 0},
		{"empty input", []byte{}, 0},
		{"too short for header", make([]byte, 10), 0},
		{"header only (20 bytes)", make([]byte, 20), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := extractCapabilities(tt.input)
			if len(caps) != tt.wantSize {
				t.Errorf("expected %d capabilities, got %d", tt.wantSize, len(caps))
			}
		})
	}
}

// TestCapability_MultipleNonStandardTypes verifies that when multiple
// non-standard types are used, all corresponding capabilities are emitted.
func TestCapability_MultipleNonStandardTypes(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f64", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
			{Name: "i16", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 2}},
			{Name: "u8", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 1}},
			{Name: "i64", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	caps := extractCapabilities(spvBytes)

	assertCapability(t, caps, CapabilityShader)
	assertCapability(t, caps, CapabilityFloat64)
	assertCapability(t, caps, CapabilityInt16)
	assertCapability(t, caps, CapabilityInt8)
	assertCapability(t, caps, CapabilityInt64)

	// Standard types should not trigger extra capabilities.
	assertNoCapability(t, caps, CapabilityFloat16)
	assertNoCapability(t, caps, CapabilityImageQuery)
}
