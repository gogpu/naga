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
		{ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2"},
		{ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat3"},
		{ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat4"},
		{ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat2x3"},
		{ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "mat3x4"},
		{ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}, "dmat4"},
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
	if name != "_main" {
		t.Errorf("Expected '_main', got %q", name)
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

	// All should be escaped
	for i, name := range names {
		if !strings.HasPrefix(name, "_") {
			t.Errorf("Name %d (%q) should be escaped", i, name)
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
		{1.0, "."},       // Should have decimal point
		{0.5, "0.5"},     // Exact value
		{1.5e10, "e+10"}, // Scientific notation
		{0.0, "0.0"},     // Zero with decimal
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

func TestCompile_EmptyModule(t *testing.T) {
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
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled},
			"sampler2D",
		},
		{
			"sampler3D",
			ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled},
			"sampler3D",
		},
		{
			"samplerCube",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled},
			"samplerCube",
		},
		{
			"sampler2DArray",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Arrayed: true},
			"sampler2DArray",
		},
		{
			"sampler2DMS",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Multisampled: true},
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
	if got != "mat4" {
		t.Errorf("Invalid size should clamp to mat4, got %q", got)
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
