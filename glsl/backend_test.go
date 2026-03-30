// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Version Tests
// =============================================================================

func TestVersion_String(t *testing.T) {
	tests := []struct {
		version Version
		want    string
	}{
		{Version330, "330 core"},
		{Version400, "400 core"},
		{Version410, "410 core"},
		{Version420, "420 core"},
		{Version430, "430 core"},
		{Version450, "450 core"},
		{Version460, "460 core"},
		{VersionES300, "300 es"},
		{VersionES310, "310 es"},
		{VersionES320, "320 es"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.version.String()
			if got != tt.want {
				t.Errorf("Version.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersion_VersionNumber(t *testing.T) {
	tests := []struct {
		version Version
		want    string
	}{
		{Version330, "330"},
		{Version450, "450"},
		{VersionES300, "300"},
		{VersionES310, "310"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.version.VersionNumber()
			if got != tt.want {
				t.Errorf("Version.VersionNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersion_SupportsCompute(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version330, false},
		{Version400, false},
		{Version420, false},
		{Version430, true},
		{Version450, true},
		{Version460, true},
		{VersionES300, false},
		{VersionES310, true},
		{VersionES320, true},
	}

	for _, tt := range tests {
		t.Run(tt.version.String(), func(t *testing.T) {
			got := tt.version.SupportsCompute()
			if got != tt.want {
				t.Errorf("SupportsCompute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVersion_versionLessThan(t *testing.T) {
	tests := []struct {
		version Version
		number  int
		want    bool
	}{
		{Version330, 410, true},   // 330 < 410
		{Version400, 410, true},   // 400 < 410
		{Version410, 410, false},  // 410 == 410, not less
		{Version420, 410, false},  // 420 > 410
		{Version450, 410, false},  // 450 > 410
		{Version460, 410, false},  // 460 > 410
		{VersionES300, 410, true}, // 300 < 410 (but ES check is separate)
		{VersionES310, 410, true}, // 310 < 410
	}

	for _, tt := range tests {
		t.Run(tt.version.String(), func(t *testing.T) {
			got := tt.version.versionLessThan(tt.number)
			if got != tt.want {
				t.Errorf("versionLessThan(%d) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}

func TestVersion_SupportsStorageBuffers(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version330, false},
		{Version430, true},
		{Version450, true},
		{VersionES300, false},
		{VersionES310, true},
	}

	for _, tt := range tests {
		t.Run(tt.version.String(), func(t *testing.T) {
			got := tt.version.SupportsStorageBuffers()
			if got != tt.want {
				t.Errorf("SupportsStorageBuffers() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Options Tests
// =============================================================================

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.LangVersion != Version330 {
		t.Errorf("Expected LangVersion Version330, got %v", opts.LangVersion)
	}

	if !opts.ForceHighPrecision {
		t.Error("Expected ForceHighPrecision to be true")
	}

	if opts.WriterFlags != WriterFlagNone {
		t.Errorf("Expected WriterFlags to be None, got %v", opts.WriterFlags)
	}
}

// =============================================================================
// Type Conversion Tests
// =============================================================================

func TestScalarToGLSL(t *testing.T) {
	tests := []struct {
		scalar ir.ScalarType
		want   string
	}{
		{ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, "int"},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "int64_t"},
		{ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "uint64_t"},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "float16_t"},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "double"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := scalarToGLSL(tt.scalar)
			if got != tt.want {
				t.Errorf("scalarToGLSL(%+v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

func TestVectorToGLSL(t *testing.T) {
	tests := []struct {
		vector ir.VectorType
		want   string
	}{
		{ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec2"},
		{ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec3"},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "vec4"},
		{ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "ivec2"},
		{ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "ivec3"},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "ivec4"},
		{ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uvec2"},
		{ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uvec3"},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uvec4"},
		{ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bvec2"},
		{ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bvec3"},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "bvec4"},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dvec4"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := vectorToGLSL(tt.vector)
			if got != tt.want {
				t.Errorf("vectorToGLSL(%+v) = %q, want %q", tt.vector, got, tt.want)
			}
		})
	}
}

func TestMatrixToGLSL(t *testing.T) {
	tests := []struct {
		matrix ir.MatrixType
		want   string
	}{
		// Rust naga always uses matCxR form, never shorthand matN
		{ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2x2"},
		{ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat3x3"},
		{ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat4x4"},
		{ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2x3"},
		{ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat3x4"},
		{ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat4x4"},
		{ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat2x3"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := matrixToGLSL(tt.matrix)
			if got != tt.want {
				t.Errorf("matrixToGLSL(%+v) = %q, want %q", tt.matrix, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Keyword Tests
// =============================================================================

func TestEscapeKeyword(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo", "foo"},
		{"myVariable", "myVariable"},
		{"color_out", "color_out"},
		// Keywords that need escaping
		{"main", "_main"},
		{"gl_Position", "_gl_Position"},
		{"gl_FragCoord", "_gl_FragCoord"},
		{"in", "_in"},
		{"out", "_out"},
		{"uniform", "_uniform"},
		{"texture", "_texture"},
		{"void", "_void"},
		{"float", "_float"},
		{"int", "_int"},
		{"bool", "_bool"},
		{"vec2", "_vec2"},
		{"vec3", "_vec3"},
		{"vec4", "_vec4"},
		{"mat4", "_mat4"},
		{"if", "_if"},
		{"else", "_else"},
		{"for", "_for"},
		{"while", "_while"},
		{"return", "_return"},
		{"discard", "_discard"},
		// Empty string case
		{"", "_unnamed"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeKeyword(tt.input)
			if got != tt.want {
				t.Errorf("escapeKeyword(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsKeyword(t *testing.T) {
	keywords := []string{
		// Types
		"void", "int", "uint", "float", "double", "bool",
		"vec2", "vec3", "vec4", "ivec2", "ivec3", "ivec4",
		"uvec2", "uvec3", "uvec4", "bvec2", "bvec3", "bvec4",
		"mat2", "mat3", "mat4", "mat2x2", "mat2x3", "mat2x4",
		"mat3x2", "mat3x3", "mat3x4", "mat4x2", "mat4x3", "mat4x4",
		"sampler2D", "sampler3D", "samplerCube",
		// Qualifiers
		"uniform", "in", "out", "inout", "varying", "attribute",
		"layout", "flat", "smooth", "noperspective",
		// Control flow
		"if", "else", "for", "while", "do", "switch", "case", "default",
		"break", "continue", "return", "discard",
		// Built-ins
		"gl_Position", "gl_FragCoord", "gl_VertexID", "gl_InstanceID",
		"main", "texture",
	}

	for _, kw := range keywords {
		if !isKeyword(kw) {
			t.Errorf("%q should be a keyword", kw)
		}
	}

	nonKeywords := []string{
		"myVariable", "foo", "bar", "customFunc", "color_output",
		"position", "normal", "texCoord", "fragColor",
	}

	for _, nkw := range nonKeywords {
		if isKeyword(nkw) {
			t.Errorf("%q should not be a keyword", nkw)
		}
	}
}

// =============================================================================
// Namer Tests
// =============================================================================

func TestNamer_UniqueNames(t *testing.T) {
	n := newNamer()

	name1 := n.call("foo")
	name2 := n.call("foo")
	name3 := n.call("foo")

	if name1 != "foo" {
		t.Errorf("First name should be 'foo', got %q", name1)
	}
	if name2 == name1 {
		t.Error("Second name should be different from first")
	}
	if name3 == name1 || name3 == name2 {
		t.Error("Third name should be different from others")
	}
}

func TestNamer_EscapesKeywords(t *testing.T) {
	n := newNamer()

	name := n.call("main")
	if name != "main_" {
		t.Errorf("Expected 'main_', got %q", name)
	}

	// Should still generate unique names for escaped keywords
	name2 := n.call("main")
	if name2 == name {
		t.Error("Second 'main' should get a unique name")
	}
}

func TestNamer_MultipleKeywords(t *testing.T) {
	n := newNamer()

	names := []string{
		n.call("float"),
		n.call("int"),
		n.call("vec4"),
		n.call("mat4"),
	}

	// All should be escaped (keywords get '_' suffix)
	for i, name := range names {
		if !strings.HasSuffix(name, "_") {
			t.Errorf("Name %d (%q) should be escaped with '_' suffix", i, name)
		}
	}

	// All should be unique
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("Duplicate name: %q", name)
		}
		seen[name] = true
	}
}

// =============================================================================
// Format Tests
// =============================================================================

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float32
		contains string
	}{
		{1.0, "."},      // Should have decimal point
		{0.5, "0.5"},    // Exact value
		{1.5e10, "e10"}, // Scientific notation (no '+' in exponent)
		{0.0, "0.0"},    // Zero with decimal
	}

	for _, tt := range tests {
		got := formatFloat(tt.input)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("formatFloat(%v) = %q, should contain %q", tt.input, got, tt.contains)
		}
	}
}

func TestFormatFloat64(t *testing.T) {
	tests := []struct {
		input    float64
		contains string
	}{
		{1.0, "."},
		{0.5, "0.5"},
		{1.5e100, "e+"},
	}

	for _, tt := range tests {
		got := formatFloat64(tt.input)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("formatFloat64(%v) = %q, should contain %q", tt.input, got, tt.contains)
		}
	}
}

// =============================================================================
// Compile Tests - Empty Module
// =============================================================================

func TestCompile_DefaultOptions(t *testing.T) {
	module := &ir.Module{}

	source, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Should have version directive
	if !strings.HasPrefix(source, "#version 330 core") {
		t.Errorf("Expected version directive, got: %s", source[:minInt(50, len(source))])
	}

	// Info should be populated
	if info.EntryPointNames == nil {
		t.Error("EntryPointNames should not be nil")
	}
}

func TestCompile_ES300(t *testing.T) {
	module := &ir.Module{}

	source, _, err := Compile(module, Options{
		LangVersion: VersionES300,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Should have ES version directive
	if !strings.HasPrefix(source, "#version 300 es") {
		t.Errorf("Expected ES version directive, got: %s", source[:minInt(50, len(source))])
	}

	// Should have precision qualifiers
	if !strings.Contains(source, "precision highp float;") {
		t.Error("Expected precision qualifier for ES")
	}

	// ES versions must NOT include GL_ARB_separate_shader_objects (core in ES 3.00+)
	if strings.Contains(source, "GL_ARB_separate_shader_objects") {
		t.Error("ES 300 should NOT include GL_ARB_separate_shader_objects extension")
	}
}

func TestCompile_ES310(t *testing.T) {
	module := &ir.Module{}

	source, _, err := Compile(module, Options{
		LangVersion: VersionES310,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !strings.HasPrefix(source, "#version 310 es") {
		t.Errorf("Expected ES 3.10 version directive, got: %s", source[:minInt(50, len(source))])
	}

	// ES versions must NOT include GL_ARB_separate_shader_objects
	if strings.Contains(source, "GL_ARB_separate_shader_objects") {
		t.Error("ES 310 should NOT include GL_ARB_separate_shader_objects extension")
	}
}

func TestCompile_Version450(t *testing.T) {
	module := &ir.Module{}

	source, _, err := Compile(module, Options{
		LangVersion: Version450,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !strings.HasPrefix(source, "#version 450 core") {
		t.Errorf("Expected 450 core version directive, got: %s", source[:minInt(50, len(source))])
	}

	// GLSL 450 >= 410: must NOT include GL_ARB_separate_shader_objects (core)
	if strings.Contains(source, "GL_ARB_separate_shader_objects") {
		t.Error("GLSL 450 should NOT include GL_ARB_separate_shader_objects extension")
	}
}

func TestCompile_Version410(t *testing.T) {
	module := &ir.Module{}

	source, _, err := Compile(module, Options{
		LangVersion: Version410,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !strings.HasPrefix(source, "#version 410 core") {
		t.Errorf("Expected 410 core version directive, got: %s", source[:minInt(50, len(source))])
	}

	// GLSL 410 is where layout(location) on varyings became core.
	// Must NOT include the extension.
	if strings.Contains(source, "GL_ARB_separate_shader_objects") {
		t.Error("GLSL 410 should NOT include GL_ARB_separate_shader_objects extension")
	}
}

// =============================================================================
// Compile Tests - Struct Types
// =============================================================================

func TestCompile_SimpleStruct(t *testing.T) {
	f32Type := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32Type}, // Type 0: f32
			{
				Name: "VertexOutput",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: 0, Offset: 0},
					},
					Span: 4,
				},
			},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Check that struct is defined
	if !strings.Contains(source, "struct ") {
		t.Error("Expected struct definition in output")
	}
}

func TestCompile_StructWithVectors(t *testing.T) {
	f32Type := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vec4Type := ir.VectorType{Size: 4, Scalar: f32Type}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32Type},  // Type 0: f32
			{Name: "", Inner: vec4Type}, // Type 1: vec4<f32>
			{
				Name: "VertexData",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: 1, Offset: 0},
						{Name: "color", Type: 1, Offset: 16},
					},
					Span: 32,
				},
			},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Should contain vec4
	if !strings.Contains(source, "vec4") {
		t.Error("Expected vec4 in struct definition")
	}
}

// =============================================================================
// Compile Tests - Global Variables
// =============================================================================

func TestCompile_UniformBuffer(t *testing.T) {
	f32Type := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	mat4Type := ir.MatrixType{Columns: 4, Rows: 4, Scalar: f32Type}

	uniformStruct := ir.StructType{
		Members: []ir.StructMember{
			{Name: "model", Type: 1, Offset: 0},
			{Name: "view", Type: 1, Offset: 64},
			{Name: "projection", Type: 1, Offset: 128},
		},
		Span: 192,
	}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32Type},               // Type 0: f32
			{Name: "", Inner: mat4Type},              // Type 1: mat4
			{Name: "Uniforms", Inner: uniformStruct}, // Type 2: Uniforms struct
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "uniforms",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    2,
				Init:    nil,
			},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Should have uniform declaration
	if !strings.Contains(source, "uniform") {
		t.Error("Expected uniform keyword in output")
	}

	// Should have mat4
	if !strings.Contains(source, "mat4") {
		t.Error("Expected mat4 in output")
	}
}

// =============================================================================
// Compile Tests - Constants
// =============================================================================

func TestCompile_ScalarConstants(t *testing.T) {
	f32Type := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	i32Type := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	u32Type := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32Type}, // Type 0
			{Name: "", Inner: i32Type}, // Type 1
			{Name: "", Inner: u32Type}, // Type 2
		},
		Constants: []ir.Constant{
			{Name: "PI", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40490fdb}}, // 3.14159
			{Name: "MAX_COUNT", Type: 1, Value: ir.ScalarValue{Kind: ir.ScalarSint, Bits: 100}},  // 100
			{Name: "FLAGS", Type: 2, Value: ir.ScalarValue{Kind: ir.ScalarUint, Bits: 0xFF}},     // 255
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Should have const declarations
	if !strings.Contains(source, "const") {
		t.Error("Expected const keyword in output")
	}
}

// =============================================================================
// Image Type Tests
// =============================================================================

func TestImageToGLSL(t *testing.T) {
	tests := []struct {
		name  string
		image ir.ImageType
		want  string
	}{
		{
			"sampler2D",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			"sampler2D",
		},
		{
			"sampler3D",
			ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			"sampler3D",
		},
		{
			"samplerCube",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat},
			"samplerCube",
		},
		{
			"sampler2DArray",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Arrayed: true, SampledKind: ir.ScalarFloat},
			"sampler2DArray",
		},
		{
			"sampler2DMS",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Multisampled: true, SampledKind: ir.ScalarFloat},
			"sampler2DMS",
		},
		{
			"sampler2DShadow",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth},
			"sampler2DShadow",
		},
		{
			"samplerCubeShadow",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassDepth},
			"samplerCubeShadow",
		},
		{
			"image2D",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage},
			"image2D",
		},
	}

	// Create a minimal writer for testing
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, &opts)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.imageToGLSL(tt.image)
			if got != tt.want {
				t.Errorf("imageToGLSL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Atomic Type Tests
// =============================================================================

func TestAtomicToGLSL(t *testing.T) {
	module := &ir.Module{}
	opts := DefaultOptions()
	w := newWriter(module, &opts)

	tests := []struct {
		atomic ir.AtomicType
		want   string
	}{
		{ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "uint"},
		{ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "int"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := w.atomicToGLSL(tt.atomic)
			if got != tt.want {
				t.Errorf("atomicToGLSL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Struct Equality Tests
// =============================================================================

func TestStructsEqual(t *testing.T) {
	struct1 := ir.StructType{
		Members: []ir.StructMember{
			{Name: "a", Type: 0, Offset: 0},
			{Name: "b", Type: 1, Offset: 4},
		},
	}

	struct2 := ir.StructType{
		Members: []ir.StructMember{
			{Name: "a", Type: 0, Offset: 0},
			{Name: "b", Type: 1, Offset: 4},
		},
	}

	struct3 := ir.StructType{
		Members: []ir.StructMember{
			{Name: "x", Type: 0, Offset: 0},
			{Name: "y", Type: 1, Offset: 4},
		},
	}

	struct4 := ir.StructType{
		Members: []ir.StructMember{
			{Name: "a", Type: 0, Offset: 0},
		},
	}

	if !structsEqual(struct1, struct2) {
		t.Error("struct1 and struct2 should be equal")
	}

	if structsEqual(struct1, struct3) {
		t.Error("struct1 and struct3 should not be equal (different names)")
	}

	if structsEqual(struct1, struct4) {
		t.Error("struct1 and struct4 should not be equal (different lengths)")
	}
}

// =============================================================================
// Translation Info Tests
// =============================================================================

func TestTranslationInfo(t *testing.T) {
	module := &ir.Module{}

	_, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// EntryPointNames should be initialized
	if info.EntryPointNames == nil {
		t.Error("EntryPointNames should not be nil")
	}

	// RequiredVersion should be set
	if info.RequiredVersion.Major == 0 && info.RequiredVersion.Minor == 0 {
		t.Error("RequiredVersion should be set")
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestCompile_ZeroVersion(t *testing.T) {
	module := &ir.Module{}

	// Zero version should default to 330
	source, _, err := Compile(module, Options{})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if !strings.HasPrefix(source, "#version 330 core") {
		t.Errorf("Expected default 330 core, got: %s", source[:minInt(50, len(source))])
	}
}

func TestVectorToGLSL_InvalidSize(t *testing.T) {
	// Invalid size should clamp to 4
	vec := ir.VectorType{Size: 10, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	got := vectorToGLSL(vec)
	if got != "vec4" {
		t.Errorf("Invalid size should clamp to vec4, got %q", got)
	}

	vec = ir.VectorType{Size: 0, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	got = vectorToGLSL(vec)
	if got != "vec4" {
		t.Errorf("Zero size should clamp to vec4, got %q", got)
	}
}

func TestMatrixToGLSL_InvalidSize(t *testing.T) {
	// Invalid dimensions should clamp to 4
	mat := ir.MatrixType{Columns: 10, Rows: 10, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	got := matrixToGLSL(mat)
	if got != "mat4x4" {
		t.Errorf("Invalid size should clamp to mat4x4, got %q", got)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Namer Tests — matches Rust naga's Namer behavior
// ---------------------------------------------------------------------------

func TestNamer_BasicNames(t *testing.T) {
	n := newNamer()

	// First use returns the base name
	if got := n.call("foo"); got != "foo" {
		t.Errorf("first 'foo' = %q, want 'foo'", got)
	}

	// Second use gets _1 suffix
	if got := n.call("foo"); got != "foo_1" {
		t.Errorf("second 'foo' = %q, want 'foo_1'", got)
	}

	// Third use gets _2
	if got := n.call("foo"); got != "foo_2" {
		t.Errorf("third 'foo' = %q, want 'foo_2'", got)
	}
}

func TestNamer_DigitEnding(t *testing.T) {
	n := newNamer()

	// Names ending in digits get trailing underscore
	if got := n.call("v3"); got != "v3_" {
		t.Errorf("'v3' = %q, want 'v3_'", got)
	}

	// Second use gets _1
	if got := n.call("v3"); got != "v3_1" {
		t.Errorf("second 'v3' = %q, want 'v3_1'", got)
	}
}

func TestNamer_Keywords(t *testing.T) {
	n := newNamer()

	// Keywords get trailing underscore
	if got := n.call("main"); got != "main_" {
		t.Errorf("'main' = %q, want 'main_'", got)
	}
}

func TestNamer_PerNameCounters(t *testing.T) {
	n := newNamer()

	// Different names get independent counters
	n.call("a") // a
	n.call("b") // b
	if got := n.call("a"); got != "a_1" {
		t.Errorf("second 'a' = %q, want 'a_1'", got)
	}
	if got := n.call("b"); got != "b_1" {
		t.Errorf("second 'b' = %q, want 'b_1'", got)
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo", "foo"},
		{"", "unnamed"},
		{"abc123", "abc123"},
		{"v3____", "v3"},   // Trailing underscores stripped
		{"__ab__", "_ab"},  // Consecutive __ collapsed, trailing stripped
		{"123abc", "abc"},  // Leading digits stripped
		{"a_b_c", "a_b_c"}, // Valid name preserved
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNamer_StructMemberNamespace(t *testing.T) {
	// Each struct gets its own namer namespace
	n1 := newNamer()
	n2 := newNamer()

	// Same name in different namespaces → no collision
	if got := n1.call("x"); got != "x" {
		t.Errorf("n1 'x' = %q, want 'x'", got)
	}
	if got := n2.call("x"); got != "x" {
		t.Errorf("n2 'x' = %q, want 'x'", got)
	}
}

// ---------------------------------------------------------------------------
// ZeroValue Tests
// ---------------------------------------------------------------------------

func TestZeroInitValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},                                  // [0]
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                  // [1]
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                  // [2]
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                 // [3]
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [4]
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},  // [5]
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	tests := []struct {
		typeHandle ir.TypeHandle
		want       string
	}{
		{0, "false"},
		{1, "0"},
		{2, "0u"},
		{3, "0.0"},
		{4, "vec3(0.0)"},
		{5, "ivec2(0)"},
	}

	for _, tt := range tests {
		got := w.zeroInitValue(tt.typeHandle)
		if got != tt.want {
			t.Errorf("zeroInitValue(%d) = %q, want %q", tt.typeHandle, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Version capability tests
// ---------------------------------------------------------------------------

func TestVersionSupportsDerivativeControl(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 10, ES: true}, false}, // ES 310
		{Version{Major: 3, Minor: 20, ES: true}, true},  // ES 320
		{Version{Major: 3, Minor: 30}, false},           // Desktop 330
		{Version{Major: 4, Minor: 50}, true},            // Desktop 450
	}
	for _, tt := range tests {
		got := tt.version.supportsDerivativeControl()
		if got != tt.want {
			t.Errorf("Version(%v).supportsDerivativeControl() = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestVersionSupportsExplicitLocations(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 0, ES: true}, false}, // ES 300
		{Version{Major: 3, Minor: 10, ES: true}, true}, // ES 310
		{Version{Major: 4, Minor: 10}, false},          // Desktop 410
		{Version{Major: 4, Minor: 20}, true},           // Desktop 420
	}
	for _, tt := range tests {
		got := tt.version.supportsExplicitLocations()
		if got != tt.want {
			t.Errorf("Version(%v).supportsExplicitLocations() = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestVersionSupportsIOLocations(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 0, ES: true}, true}, // ES 300
		{Version{Major: 3, Minor: 30}, true},          // Desktop 330
		{Version{Major: 3, Minor: 20}, false},         // Desktop 320
	}
	for _, tt := range tests {
		got := tt.version.supportsIOLocations()
		if got != tt.want {
			t.Errorf("Version(%v).supportsIOLocations() = %v, want %v", tt.version, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Do-while switch optimization test
// ---------------------------------------------------------------------------

func TestIsSingleBodySwitch(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})

	// Single default case → single body
	single := ir.StmtSwitch{
		Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtBreak{}}}, FallThrough: false},
		},
	}
	if !w.isSingleBodySwitch(single) {
		t.Error("single default case should be single body")
	}

	// Multiple non-empty cases → not single body
	multi := ir.StmtSwitch{
		Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueI32(1), Body: ir.Block{{Kind: ir.StmtBreak{}}}, FallThrough: false},
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtBreak{}}}, FallThrough: false},
		},
	}
	if w.isSingleBodySwitch(multi) {
		t.Error("multiple non-empty cases should not be single body")
	}

	// Empty fallthrough + last case → single body
	fallthrough_ := ir.StmtSwitch{
		Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueI32(1), Body: nil, FallThrough: true},
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtBreak{}}}, FallThrough: false},
		},
	}
	if !w.isSingleBodySwitch(fallthrough_) {
		t.Error("empty fallthrough + last case should be single body")
	}
}

// ---------------------------------------------------------------------------
// Feature detection tests
// ---------------------------------------------------------------------------

func TestFeatureDetection_ComputeShader(t *testing.T) {
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageCompute, Function: ir.Function{}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.collectFeatures()
	if !w.features.contains(FeatureComputeShader) {
		t.Error("compute shader should request FeatureComputeShader")
	}
}

func TestFeatureDetection_MultiView(t *testing.T) {
	viewBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinViewIndex})
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "view", Type: 0, Binding: &viewBinding},
					},
				},
			},
		},
		Types: []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
	}
	w := newWriter(module, &Options{LangVersion: Version{Major: 3, Minor: 0, ES: true}})
	w.collectFeatures()
	if !w.features.contains(FeatureMultiView) {
		t.Error("BuiltinViewIndex should request FeatureMultiView")
	}
}

// ---------------------------------------------------------------------------
// Varying name generation tests
// ---------------------------------------------------------------------------

func TestVaryingName(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})

	tests := []struct {
		location int
		stage    ir.ShaderStage
		isOutput bool
		want     string
	}{
		{0, ir.StageVertex, false, "_p2vs_location0"},
		{0, ir.StageVertex, true, "_vs2fs_location0"},
		{0, ir.StageFragment, false, "_vs2fs_location0"},
		{0, ir.StageFragment, true, "_fs2p_location0"},
		{3, ir.StageFragment, true, "_fs2p_location3"},
	}
	for _, tt := range tests {
		got := w.varyingName(tt.location, tt.stage, tt.isOutput)
		if got != tt.want {
			t.Errorf("varyingName(%d, %v, %v) = %q, want %q",
				tt.location, tt.stage, tt.isOutput, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Scalar vec prefix test
// ---------------------------------------------------------------------------

func TestFeatureDetection_TextureShadowLod(t *testing.T) {
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageFragment, Function: ir.Function{}},
		},
	}
	w := newWriter(module, &Options{
		LangVersion: Version{Major: 3, Minor: 20, ES: true},
		WriterFlags: WriterFlagTextureShadowLod,
	})
	w.collectFeatures()
	if !w.features.contains(FeatureTextureShadowLod) {
		t.Error("WriterFlagTextureShadowLod should request FeatureTextureShadowLod")
	}
}

func TestCoordinateAdjustInStructReturn(t *testing.T) {
	// Verify writeCoordinateAdjustIfNeeded detects gl_Position member
	info := &epStructInfo{
		members: []epStructMemberInfo{
			{glslName: "_vs2fs_location0"},
			{isBuiltin: true, builtinName: "gl_Position", glslName: "gl_Position"},
		},
	}

	w := newWriter(&ir.Module{}, &Options{WriterFlags: WriterFlagAdjustCoordinateSpace})
	w.writeCoordinateAdjustIfNeeded(info)
	output := w.String()
	if !strings.Contains(output, "gl_Position.yz") {
		t.Errorf("expected coordinate adjust, got: %q", output)
	}

	// No adjust when flag not set
	w2 := newWriter(&ir.Module{}, &Options{})
	w2.writeCoordinateAdjustIfNeeded(info)
	output2 := w2.String()
	if strings.Contains(output2, "gl_Position.yz") {
		t.Error("should not adjust when flag not set")
	}
}

func TestForcePointSizeInReturn(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	w := newWriter(module, &Options{
		LangVersion: Version{Major: 3, Minor: 0, ES: true},
		WriterFlags: WriterFlagForcePointSize,
	})
	w.inEntryPoint = true
	w.entryPointResult = &ir.FunctionResult{Type: 0, Binding: &posBinding}
	w.currentFunction = &ir.Function{
		Expressions:     []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}},
		ExpressionTypes: []ir.TypeResolution{{Handle: func() *ir.TypeHandle { h := ir.TypeHandle(0); return &h }()}},
	}
	handle := ir.ExpressionHandle(0)
	ret := ir.StmtReturn{Value: &handle}
	err := w.writeReturn(ret)
	if err != nil {
		t.Fatal(err)
	}
	output := w.String()
	if !strings.Contains(output, "gl_PointSize = 1.0;") {
		t.Errorf("expected gl_PointSize = 1.0; in output:\n%s", output)
	}
}

func TestInvariantOnlyForOutput(t *testing.T) {
	// invariant should only be emitted for output builtins, not input
	module := &ir.Module{}
	w := newWriter(module, &Options{LangVersion: Version330})

	posOut := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition, Invariant: true})
	w.writeSingleVarying(&posOut, 0, ir.StageVertex, true)
	output := w.String()
	if !strings.Contains(output, "invariant gl_Position;") {
		t.Errorf("expected invariant for vertex output, got: %q", output)
	}

	// Fragment input should NOT get invariant
	w2 := newWriter(module, &Options{LangVersion: Version330})
	posIn := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition, Invariant: true})
	w2.writeSingleVarying(&posIn, 0, ir.StageFragment, false)
	output2 := w2.String()
	if strings.Contains(output2, "invariant") {
		t.Errorf("should not emit invariant for fragment input, got: %q", output2)
	}
}

func TestVaryingInterpolationSampling(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}},
	}
	// noperspective centroid
	w := newWriter(module, &Options{LangVersion: Version{Major: 3, Minor: 0, ES: true}})
	loc := ir.Binding(ir.LocationBinding{
		Location:      4,
		Interpolation: &ir.Interpolation{Kind: ir.InterpolationLinear, Sampling: ir.SamplingCentroid},
	})
	w.writeSingleVarying(&loc, 0, ir.StageFragment, false)
	output := w.String()
	if !strings.Contains(output, "noperspective centroid in") {
		t.Errorf("expected 'noperspective centroid in', got: %q", output)
	}
}

func TestScalarVecPrefix(t *testing.T) {
	tests := []struct {
		kind ir.ScalarKind
		want string
	}{
		{ir.ScalarBool, "b"},
		{ir.ScalarSint, "i"},
		{ir.ScalarUint, "u"},
		{ir.ScalarFloat, ""},
	}
	for _, tt := range tests {
		got := scalarVecPrefix(tt.kind)
		if got != tt.want {
			t.Errorf("scalarVecPrefix(%v) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestFormatLiteral_F64NoDoubleSuffix(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	// F64 literal should have exactly one LF suffix, not LFLF
	got := w.formatLiteral(ir.Literal{Value: ir.LiteralF64(1.0)})
	if !strings.HasSuffix(got, "LF") {
		t.Errorf("f64 literal should end with LF, got %q", got)
	}
	if strings.HasSuffix(got, "LFLF") {
		t.Errorf("f64 literal should not have double LF suffix, got %q", got)
	}
}

func TestSanitizeName_TemplateChars(t *testing.T) {
	// <>,: should become underscores, matching Rust naga
	tests := []struct {
		input string
		want  string
	}{
		{"_atomic_compare_exchange_result<Sint, 4>", "_atomic_compare_exchange_result_Sint_4"},
		{"vec<f32, 3>", "vec_f32_3"},
		{"type::inner", "type_inner"},
	}
	for _, tt := range tests {
		got := sanitizeName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSupportsFma(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 10, ES: true}, false}, // ES 310 — no fma
		{Version{Major: 3, Minor: 20, ES: true}, true},  // ES 320 — has fma
		{Version{Major: 3, Minor: 30}, false},           // Desktop 330 — no fma
		{Version{Major: 4, Minor: 0}, true},             // Desktop 400 — has fma
	}
	for _, tt := range tests {
		got := tt.version.supportsFma()
		if got != tt.want {
			t.Errorf("Version(%v).supportsFma() = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestWriteConstExpr_Splat(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                 // [0]
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [1]
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                // [0]
			{Kind: ir.ExprSplat{Size: 3, Value: ir.ExpressionHandle(0)}}, // [1]
		},
		Constants: []ir.Constant{
			{Name: "c", Type: 1, Init: ir.ExpressionHandle(1)},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	got := w.writeConstExpr(ir.ExpressionHandle(1))
	if got != "vec3(0.0)" {
		t.Errorf("writeConstExpr(Splat) = %q, want %q", got, "vec3(0.0)")
	}
}

func TestWriteConstExpr_Compose(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                 // [0]
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [1]
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                            // [0]
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},                            // [1]
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}}, // [2]
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	got := w.writeConstExpr(ir.ExpressionHandle(2))
	if got != "vec2(1.0, 2.0)" {
		t.Errorf("writeConstExpr(Compose) = %q, want %q", got, "vec2(1.0, 2.0)")
	}
}

func TestWriteArrayLength_UintWrap(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{}, Stride: 4}}, // dynamic array
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Type: 1},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageCompute, Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprArrayLength{Array: 0}},
				},
			}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &module.EntryPoints[0].Function
	got, err := w.writeArrayLength(ir.ExprArrayLength{Array: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "uint(") {
		t.Errorf("arrayLength should wrap in uint(), got %q", got)
	}
}

func TestArrayToGLSL_NestedDimOrder(t *testing.T) {
	two := uint32(2)
	three := uint32(3)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                          // [0] float
			{Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &two}, Stride: 4}},   // [1] float[2]
			{Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &three}, Stride: 8}}, // [2] float[2][3] → array<array<float,2>,3>
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	// Outer array: array<array<float,2>,3> → float[3][2] (outer first)
	got := w.typeToGLSL(module.Types[2])
	if got != "float[3][2]" {
		t.Errorf("nested array type = %q, want %q", got, "float[3][2]")
	}

	// Simple array: array<float,2> → float[2]
	got2 := w.typeToGLSL(module.Types[1])
	if got2 != "float[2]" {
		t.Errorf("simple array type = %q, want %q", got2, "float[2]")
	}
}

func TestBooleanBinaryOps(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, // [0]
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, // [1]
		},
	}
	boolHandle := ir.TypeHandle(0)
	intHandle := ir.TypeHandle(1)

	w := newWriter(module, &Options{LangVersion: Version330})

	// Boolean And → &&
	boolBin := ir.ExprBinary{Left: 0, Right: 1, Op: ir.BinaryAnd}
	w.currentFunction = &ir.Function{
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &boolHandle}, {Handle: &boolHandle},
		},
	}
	if w.isBooleanBinaryExpr(boolBin) != true {
		t.Error("bool And should be detected as boolean")
	}

	// Int And → NOT boolean
	w.currentFunction = &ir.Function{
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &intHandle}, {Handle: &intHandle},
		},
	}
	if w.isBooleanBinaryExpr(boolBin) != false {
		t.Error("int And should NOT be detected as boolean")
	}
}

func TestSupportsPack4x8(t *testing.T) {
	tests := []struct {
		version Version
		want    bool
	}{
		{Version{Major: 3, Minor: 30}, false},          // Desktop 330
		{Version{Major: 4, Minor: 0}, true},            // Desktop 400
		{Version{Major: 3, Minor: 0, ES: true}, false}, // ES 300
		{Version{Major: 3, Minor: 10, ES: true}, true}, // ES 310
	}
	for _, tt := range tests {
		got := tt.version.supportsPack4x8()
		if got != tt.want {
			t.Errorf("Version(%v).supportsPack4x8() = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestVectorComparisonDetection(t *testing.T) {
	vecHandle := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		ExpressionTypes: []ir.TypeResolution{{Handle: &vecHandle}, {Handle: &vecHandle}},
	}
	b := ir.ExprBinary{Left: 0, Right: 1, Op: ir.BinaryEqual}
	if !w.isVectorBinaryExpr(b) {
		t.Error("ivec2 should be detected as vector binary expr")
	}
}

func TestFloatModuloDetection(t *testing.T) {
	floatHandle := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		ExpressionTypes: []ir.TypeResolution{{Handle: &floatHandle}, {Handle: &floatHandle}},
	}
	b := ir.ExprBinary{Left: 0, Right: 1, Op: ir.BinaryModulo}
	if !w.isFloatBinaryExpr(b) {
		t.Error("float should be detected as float binary expr")
	}
}

// =============================================================================
// scanNeedBakeExpressions Tests
// =============================================================================

func TestScanNeedBakeExpressions_RefCounting(t *testing.T) {
	// Expressions used 2+ times should be baked (threshold = 2).
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	// Expression 0 is used twice (in Add left and right) -> should be baked
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},              // [0]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}}, // [1] uses [0] twice
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
			{Handle: &tF32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
		},
	}

	w.scanNeedBakeExpressions(fn)

	// Expression 0 has refcount = 2 (used by both Left and Right of expr 1),
	// so it should be marked for baking.
	if _, ok := w.needBakeExpression[0]; !ok {
		t.Error("expression 0 (used 2 times) should be marked for baking")
	}
}

func TestScanNeedBakeExpressions_AccessNeverBaked(t *testing.T) {
	// Access/AccessIndex should never be baked (threshold = MAX).
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // [0]
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] AccessIndex
			// Use AccessIndex twice to push refcount above 2
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 1, Right: 1}}, // [2]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
			{Handle: &tF32},
			{Handle: &tF32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
		},
	}

	w.scanNeedBakeExpressions(fn)

	// Expression 1 (AccessIndex) used 2 times but should NOT be baked (threshold = MAX)
	if _, ok := w.needBakeExpression[1]; ok {
		t.Error("AccessIndex expression should never be baked regardless of ref count")
	}
}

func TestScanNeedBakeExpressions_SingleUseNotBaked(t *testing.T) {
	// Expression used only once should NOT be baked.
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},              // [0]
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},              // [1]
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}}, // [2] each used once
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
			{Handle: &tF32},
			{Handle: &tF32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
		},
	}

	w.scanNeedBakeExpressions(fn)

	if _, ok := w.needBakeExpression[0]; ok {
		t.Error("expression 0 (single use) should NOT be baked")
	}
	if _, ok := w.needBakeExpression[1]; ok {
		t.Error("expression 1 (single use) should NOT be baked")
	}
}

func TestScanNeedBakeExpressions_MathQuantizeForced(t *testing.T) {
	// MathQuantizeF16 forces its argument to be baked regardless of ref count.
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},        // [0]
			{Kind: ir.ExprMath{Fun: ir.MathQuantizeF16, Arg: 0}}, // [1]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
			{Handle: &tF32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
		},
	}

	w.scanNeedBakeExpressions(fn)

	// MathQuantizeF16 forces baking of its argument
	if _, ok := w.needBakeExpression[0]; !ok {
		t.Error("MathQuantizeF16 argument should be force-baked")
	}
}

func TestScanNeedBakeExpressions_StmtRefCounting(t *testing.T) {
	// Expressions referenced from statements should also be counted.
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	// Expression 0 is used in a Store and also in a Return -> 2 refs from stmts
	retExpr := ir.ExpressionHandle(0)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // [0]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 0, Value: 0}}, // refs [0] twice (pointer + value)
			{Kind: ir.StmtReturn{Value: &retExpr}},     // refs [0] once more
		},
	}

	w.scanNeedBakeExpressions(fn)

	// Expression 0 has 3 refs from statements -> baked
	if _, ok := w.needBakeExpression[0]; !ok {
		t.Error("expression 0 (used 3 times from statements) should be baked")
	}
}

func TestScanNeedBakeExpressions_Dot4PackedForced(t *testing.T) {
	// MathDot4I8Packed/MathDot4U8Packed force baking of their arguments.
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	w := newWriter(module, &Options{LangVersion: Version330})

	arg1 := ir.ExpressionHandle(1)
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                      // [0]
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},                      // [1]
			{Kind: ir.ExprMath{Fun: ir.MathDot4I8Packed, Arg: 0, Arg1: &arg1}}, // [2]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
			{Handle: &tF32},
			{Handle: &tF32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
		},
	}

	w.scanNeedBakeExpressions(fn)

	if _, ok := w.needBakeExpression[0]; !ok {
		t.Error("Dot4I8Packed Arg should be force-baked")
	}
	if _, ok := w.needBakeExpression[1]; !ok {
		t.Error("Dot4I8Packed Arg1 should be force-baked")
	}
}

// =============================================================================
// BindingMap Tests
// =============================================================================

func TestBindingMap_LookupBinding(t *testing.T) {
	module := &ir.Module{}
	opts := Options{
		LangVersion: Version{Major: 4, Minor: 50}, // supports explicit locations
		BindingMap: map[BindingMapKey]uint8{
			{Group: 0, Binding: 0}: 5,
			{Group: 1, Binding: 2}: 10,
		},
	}
	w := newWriter(module, &opts)

	t.Run("found", func(t *testing.T) {
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
		}
		binding, ok := w.lookupBinding(gv)
		if !ok {
			t.Fatal("expected binding to be found")
		}
		if binding != 5 {
			t.Errorf("binding = %d, want 5", binding)
		}
	})

	t.Run("found_group1", func(t *testing.T) {
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 1, Binding: 2},
		}
		binding, ok := w.lookupBinding(gv)
		if !ok {
			t.Fatal("expected binding to be found")
		}
		if binding != 10 {
			t.Errorf("binding = %d, want 10", binding)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 9, Binding: 9},
		}
		_, ok := w.lookupBinding(gv)
		if ok {
			t.Error("expected binding NOT to be found")
		}
	})

	t.Run("nil_binding", func(t *testing.T) {
		gv := ir.GlobalVariable{
			Binding: nil,
		}
		_, ok := w.lookupBinding(gv)
		if ok {
			t.Error("expected false for nil binding")
		}
	})

	t.Run("nil_map", func(t *testing.T) {
		noMapOpts := Options{LangVersion: Version{Major: 4, Minor: 50}}
		w2 := newWriter(module, &noMapOpts)
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
		}
		_, ok := w2.lookupBinding(gv)
		if ok {
			t.Error("expected false when BindingMap is nil")
		}
	})

	t.Run("version_too_low", func(t *testing.T) {
		lowOpts := Options{
			LangVersion: Version330, // 330 does not support explicit locations
			BindingMap: map[BindingMapKey]uint8{
				{Group: 0, Binding: 0}: 5,
			},
		}
		w3 := newWriter(module, &lowOpts)
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
		}
		_, ok := w3.lookupBinding(gv)
		if ok {
			t.Error("expected false when GLSL version doesn't support explicit locations")
		}
	})
}

func TestBindingMap_StorageLayoutPrefix(t *testing.T) {
	module := &ir.Module{}

	t.Run("with_binding", func(t *testing.T) {
		opts := Options{
			LangVersion: Version{Major: 4, Minor: 50},
			BindingMap: map[BindingMapKey]uint8{
				{Group: 0, Binding: 3}: 7,
			},
		}
		w := newWriter(module, &opts)
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 3},
		}
		got := w.storageLayoutPrefix(gv)
		if got != "layout(std430, binding = 7) " {
			t.Errorf("got %q, want %q", got, "layout(std430, binding = 7) ")
		}
	})

	t.Run("without_binding", func(t *testing.T) {
		opts := Options{LangVersion: Version{Major: 4, Minor: 50}}
		w := newWriter(module, &opts)
		gv := ir.GlobalVariable{
			Binding: &ir.ResourceBinding{Group: 0, Binding: 3},
		}
		got := w.storageLayoutPrefix(gv)
		if got != "layout(std430) " {
			t.Errorf("got %q, want %q", got, "layout(std430) ")
		}
	})
}

// =============================================================================
// naga_modf / naga_frexp Helper Tests
// =============================================================================

func TestPredeclaredHelpers_Modf(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32}, // 0: f32
			{Name: "__modf_result_f32", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "fract", Type: 0},
					{Name: "whole", Type: 0},
				},
			}}, // 1: modf result struct
		},
	}

	opts := DefaultOptions()
	w := newWriter(module, &opts)
	// Register type names (the writer normally does this during writeModule)
	w.typeNames[0] = "float"
	w.typeNames[1] = "modf_result_f32_"

	w.writePredeclaredHelpers()
	output := w.String()

	mustContainStr(t, output, "naga_modf(float arg)")
	mustContainStr(t, output, "modf(arg, other)")
	mustContainStr(t, output, "return modf_result_f32_(fract, other);")
}

func TestPredeclaredHelpers_Frexp(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	i32 := ir.ScalarType{Kind: ir.ScalarSint, Width: 4}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32}, // 0: f32
			{Name: "", Inner: i32}, // 1: i32
			{Name: "__frexp_result_f32", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "fract", Type: 0},
					{Name: "exp", Type: 1},
				},
			}}, // 2: frexp result struct
		},
	}

	opts := DefaultOptions()
	w := newWriter(module, &opts)
	w.typeNames[0] = "float"
	w.typeNames[1] = "int"
	w.typeNames[2] = "frexp_result_f32_"

	w.writePredeclaredHelpers()
	output := w.String()

	mustContainStr(t, output, "naga_frexp(float arg)")
	mustContainStr(t, output, "int other;")
	mustContainStr(t, output, "frexp(arg, other)")
	mustContainStr(t, output, "return frexp_result_f32_(fract, other);")
}

func TestPredeclaredHelpers_VecVariants(t *testing.T) {
	tests := []struct {
		name    string
		suffix  string
		argType string
	}{
		{"modf_vec2", "__modf_result_vec2_f32", "vec2"},
		{"modf_vec3", "__modf_result_vec3_f32", "vec3"},
		{"modf_vec4", "__modf_result_vec4_f32", "vec4"},
		{"frexp_vec2", "__frexp_result_vec2_f32", "vec2"},
		{"frexp_vec4", "__frexp_result_vec4_f32", "vec4"},
		{"modf_f64", "__modf_result_f64", "double"},
		{"frexp_vec3_f64", "__frexp_result_vec3_f64", "dvec3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: f32},
					{Name: tt.suffix, Inner: ir.StructType{
						Members: []ir.StructMember{{Name: "a", Type: 0}},
					}},
				},
			}

			opts := DefaultOptions()
			w := newWriter(module, &opts)
			w.typeNames[0] = "float"
			w.typeNames[1] = "ResultType"

			w.writePredeclaredHelpers()
			output := w.String()

			isModf := strings.HasPrefix(tt.suffix, "__modf")
			if isModf {
				mustContainStr(t, output, "naga_modf("+tt.argType+" arg)")
			} else {
				mustContainStr(t, output, "naga_frexp("+tt.argType+" arg)")
			}
		})
	}
}

func TestPredeclaredHelpers_SkipsUnrelated(t *testing.T) {
	// Types without __modf_result_ or __frexp_result_ prefix should be ignored.
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "RegularStruct", Inner: f32},
			{Name: "MyCustomType", Inner: f32},
			{Name: "", Inner: f32}, // empty name
		},
	}

	opts := DefaultOptions()
	w := newWriter(module, &opts)
	w.typeNames[0] = "float"
	w.typeNames[1] = "MyCustomType"
	w.typeNames[2] = "float"

	w.writePredeclaredHelpers()
	output := w.String()

	if strings.Contains(output, "naga_modf") || strings.Contains(output, "naga_frexp") {
		t.Errorf("should not generate helpers for unrelated types, got: %s", output)
	}
}

// mustContainStr is a helper for non-compile tests.
func mustContainStr(t *testing.T, source, expected string) {
	t.Helper()
	if !strings.Contains(source, expected) {
		t.Errorf("expected output to contain %q.\nOutput:\n%s", expected, source)
	}
}
