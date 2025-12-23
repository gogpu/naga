package msl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestCompile_EmptyModule(t *testing.T) {
	module := &ir.Module{
		Types:           []ir.Type{},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	result, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check that header is present
	if !strings.Contains(result, "#include <metal_stdlib>") {
		t.Error("Expected #include <metal_stdlib> in output")
	}

	if !strings.Contains(result, "using metal::uint;") {
		t.Error("Expected 'using metal::uint;' in output")
	}

	// Info should be empty for empty module
	if len(info.EntryPointNames) != 0 {
		t.Errorf("Expected no entry point names, got %d", len(info.EntryPointNames))
	}
}

func TestVersion_String(t *testing.T) {
	tests := []struct {
		version Version
		want    string
	}{
		{Version{1, 2}, "1.2"},
		{Version{2, 0}, "2.0"},
		{Version{2, 1}, "2.1"},
		{Version{3, 0}, "3.0"},
	}

	for _, tt := range tests {
		got := tt.version.String()
		if got != tt.want {
			t.Errorf("Version{%d, %d}.String() = %q, want %q",
				tt.version.Major, tt.version.Minor, got, tt.want)
		}
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.LangVersion != Version2_1 {
		t.Errorf("Expected LangVersion 2.1, got %v", opts.LangVersion)
	}

	if !opts.ZeroInitializeWorkgroupMemory {
		t.Error("Expected ZeroInitializeWorkgroupMemory to be true")
	}

	if !opts.ForceLoopBounding {
		t.Error("Expected ForceLoopBounding to be true")
	}
}

func TestScalarTypeName(t *testing.T) {
	tests := []struct {
		scalar ir.ScalarType
		want   string
	}{
		{ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "half"},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, "short"},
		{ir.ScalarType{Kind: ir.ScalarUint, Width: 2}, "ushort"},
	}

	for _, tt := range tests {
		got := scalarTypeName(tt.scalar)
		if got != tt.want {
			t.Errorf("scalarTypeName(%+v) = %q, want %q", tt.scalar, got, tt.want)
		}
	}
}

func TestVectorTypeName(t *testing.T) {
	tests := []struct {
		vector ir.VectorType
		want   string
	}{
		{
			ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			"metal::float2",
		},
		{
			ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			"metal::float3",
		},
		{
			ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			"metal::float4",
		},
		{
			ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			"metal::int4",
		},
	}

	for _, tt := range tests {
		got := vectorTypeName(tt.vector)
		if got != tt.want {
			t.Errorf("vectorTypeName(%+v) = %q, want %q", tt.vector, got, tt.want)
		}
	}
}

func TestMatrixTypeName(t *testing.T) {
	tests := []struct {
		matrix ir.MatrixType
		want   string
	}{
		{
			ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			"metal::float4x4",
		},
		{
			ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			"metal::float3x3",
		},
		{
			ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}},
			"metal::half2x2",
		},
	}

	for _, tt := range tests {
		got := matrixTypeName(tt.matrix)
		if got != tt.want {
			t.Errorf("matrixTypeName(%+v) = %q, want %q", tt.matrix, got, tt.want)
		}
	}
}

func TestIsReserved(t *testing.T) {
	reserved := []string{"float", "int", "void", "struct", "class", "return", "if", "else"}
	for _, word := range reserved {
		if !isReserved(word) {
			t.Errorf("Expected %q to be reserved", word)
		}
	}

	notReserved := []string{"myVar", "foo", "color_output", "x123"}
	for _, word := range notReserved {
		if isReserved(word) {
			t.Errorf("Expected %q to NOT be reserved", word)
		}
	}
}

func TestEscapeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myVar", "myVar"},
		{"float", "float_"},
		{"int", "int_"},
		{"class", "class_"},
	}

	for _, tt := range tests {
		got := escapeName(tt.input)
		if got != tt.want {
			t.Errorf("escapeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestAddressSpaceName(t *testing.T) {
	tests := []struct {
		space ir.AddressSpace
		want  string
	}{
		{ir.SpaceUniform, "constant"},
		{ir.SpaceStorage, "device"},
		{ir.SpacePrivate, "thread"},
		{ir.SpaceFunction, "thread"},
		{ir.SpaceWorkGroup, "threadgroup"},
		{ir.SpaceHandle, ""},
	}

	for _, tt := range tests {
		got := addressSpaceName(tt.space)
		if got != tt.want {
			t.Errorf("addressSpaceName(%v) = %q, want %q", tt.space, got, tt.want)
		}
	}
}

func TestCompile_SimpleStruct(t *testing.T) {
	// Create a simple struct type
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
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check that struct is defined
	if !strings.Contains(result, "struct ") {
		t.Error("Expected struct definition in output")
	}
}
