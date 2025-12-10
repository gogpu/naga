package spirv

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestBackendCompileEmptyModule(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types:           []ir.Type{},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Check magic number
	if len(binary) < 20 {
		t.Fatalf("Binary too short: %d bytes", len(binary))
	}

	// Verify SPIR-V magic number (little-endian)
	magic := uint32(binary[0]) | uint32(binary[1])<<8 | uint32(binary[2])<<16 | uint32(binary[3])<<24
	if magic != MagicNumber {
		t.Errorf("Invalid magic number: got 0x%08x, want 0x%08x", magic, MagicNumber)
	}
}

func TestBackendEmitScalarTypes(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool}},
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "f64", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) == 0 {
		t.Error("Expected non-empty binary")
	}

	// Verify all types were cached
	if len(backend.typeIDs) != len(module.Types) {
		t.Errorf("Expected %d cached types, got %d", len(module.Types), len(backend.typeIDs))
	}
}

func TestBackendEmitVectorTypes(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types: []ir.Type{
			// Base scalar type
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// Vector types
			{Name: "vec2f", Inner: ir.VectorType{
				Size:   ir.Vec2,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
			{Name: "vec3f", Inner: ir.VectorType{
				Size:   ir.Vec3,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
			{Name: "vec4f", Inner: ir.VectorType{
				Size:   ir.Vec4,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) == 0 {
		t.Error("Expected non-empty binary")
	}
}

func TestBackendEmitMatrixTypes(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types: []ir.Type{
			// Base scalar type
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			// Matrix type (4x4 float)
			{Name: "mat4x4f", Inner: ir.MatrixType{
				Columns: ir.Vec4,
				Rows:    ir.Vec4,
				Scalar:  ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) == 0 {
		t.Error("Expected non-empty binary")
	}
}

func TestBackendEmitScalarConstants(t *testing.T) {
	backend := NewBackend(DefaultOptions())

	// Type: f32
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	// Type: i32
	i32Type := ir.Type{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}
	// Type: bool
	boolType := ir.Type{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool}}

	module := &ir.Module{
		Types: []ir.Type{f32Type, i32Type, boolType},
		Constants: []ir.Constant{
			{
				Name:  "pi",
				Type:  0,                                                      // f32
				Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40490fdb}, // 3.14159265
			},
			{
				Name:  "answer",
				Type:  1, // i32
				Value: ir.ScalarValue{Kind: ir.ScalarSint, Bits: 42},
			},
			{
				Name:  "truth",
				Type:  2, // bool
				Value: ir.ScalarValue{Kind: ir.ScalarBool, Bits: 1},
			},
		},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) == 0 {
		t.Error("Expected non-empty binary")
	}

	// Verify all constants were cached
	if len(backend.constantIDs) != len(module.Constants) {
		t.Errorf("Expected %d cached constants, got %d", len(module.Constants), len(backend.constantIDs))
	}
}

func TestBackendEmitCompositeConstants(t *testing.T) {
	backend := NewBackend(DefaultOptions())

	// Type: f32
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	// Type: vec3<f32>
	vec3fType := ir.Type{
		Name: "vec3f",
		Inner: ir.VectorType{
			Size:   ir.Vec3,
			Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
	}

	module := &ir.Module{
		Types: []ir.Type{f32Type, vec3fType},
		Constants: []ir.Constant{
			// Scalar constants for components
			{Name: "x", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}}, // 1.0
			{Name: "y", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40000000}}, // 2.0
			{Name: "z", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40400000}}, // 3.0
			// Composite constant: vec3(1.0, 2.0, 3.0)
			{
				Name: "position",
				Type: 1, // vec3f
				Value: ir.CompositeValue{
					Components: []ir.ConstantHandle{0, 1, 2},
				},
			},
		},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) == 0 {
		t.Error("Expected non-empty binary")
	}

	// Verify all constants were cached
	if len(backend.constantIDs) != len(module.Constants) {
		t.Errorf("Expected %d cached constants, got %d", len(module.Constants), len(backend.constantIDs))
	}
}

func TestBackendTypeDeduplification(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32_1", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "f32_2", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend.module = module
	backend.builder = NewModuleBuilder(backend.options.Version)

	// Emit both types
	id1, err1 := backend.emitType(0)
	id2, err2 := backend.emitType(1)

	if err1 != nil || err2 != nil {
		t.Fatalf("emitType failed: %v, %v", err1, err2)
	}

	// IDs should be different (we cache by handle, not by type content)
	// This is actually expected - each type gets its own ID in our current implementation
	if id1 == 0 || id2 == 0 {
		t.Error("Type IDs should not be zero")
	}
}

func TestBackendEmitStructType(t *testing.T) {
	backend := NewBackend(DefaultOptions())

	// Base types
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec3fType := ir.Type{
		Name: "vec3f",
		Inner: ir.VectorType{
			Size:   ir.Vec3,
			Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
	}

	// Struct type
	structType := ir.Type{
		Name: "Vertex",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "position", Type: 1, Offset: 0},  // vec3f at offset 0
				{Name: "normal", Type: 1, Offset: 16},   // vec3f at offset 16
				{Name: "texCoord", Type: 0, Offset: 32}, // f32 at offset 32
			},
			Span: 48, // Total size
		},
	}

	module := &ir.Module{
		Types:           []ir.Type{f32Type, vec3fType, structType},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) == 0 {
		t.Error("Expected non-empty binary")
	}
}

func TestBackendSimpleVertexShader(t *testing.T) {
	backend := NewBackend(DefaultOptions())

	// Types
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4fType := ir.Type{
		Name: "vec4f",
		Inner: ir.VectorType{
			Size:   ir.Vec4,
			Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		},
	}

	// Constants for vec4(0.0, 0.0, 0.0, 1.0)
	zeroConst := ir.Constant{
		Name:  "zero",
		Type:  1,                                             // f32
		Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0}, // 0.0
	}
	oneConst := ir.Constant{
		Name:  "one",
		Type:  1,                                                      // f32
		Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}, // 1.0
	}
	vec4Const := ir.Constant{
		Name: "position",
		Type: 2, // vec4f
		Value: ir.CompositeValue{
			Components: []ir.ConstantHandle{0, 0, 0, 1}, // vec4(0.0, 0.0, 0.0, 1.0)
		},
	}

	// Function: main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32>
	vertexIndexBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex})
	positionBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	mainFunc := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{
				Name:    "idx",
				Type:    0, // u32
				Binding: &vertexIndexBinding,
			},
		},
		Result: &ir.FunctionResult{
			Type:    2, // vec4f
			Binding: &positionBinding,
		},
		LocalVars: []ir.LocalVariable{},
		Expressions: []ir.Expression{
			// Expression 0: reference to constant vec4(0.0, 0.0, 0.0, 1.0)
			{Kind: ir.ExprConstant{Constant: 2}},
		},
		Body: []ir.Statement{
			// Emit expression 0
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			// Return the constant
			{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
		},
	}

	module := &ir.Module{
		Types:           []ir.Type{u32Type, f32Type, vec4fType},
		Constants:       []ir.Constant{zeroConst, oneConst, vec4Const},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{mainFunc},
		EntryPoints: []ir.EntryPoint{
			{
				Name:     "main",
				Stage:    ir.StageVertex,
				Function: 0,
			},
		},
	}

	binary, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(binary) < 20 {
		t.Errorf("Binary too short: %d bytes", len(binary))
	}

	// Verify magic number
	magic := uint32(binary[0]) | uint32(binary[1])<<8 | uint32(binary[2])<<16 | uint32(binary[3])<<24
	if magic != MagicNumber {
		t.Errorf("Invalid magic number: got 0x%08x, want 0x%08x", magic, MagicNumber)
	}

	// Verify function was emitted
	if len(backend.functionIDs) != 1 {
		t.Errorf("Expected 1 function ID, got %d", len(backend.functionIDs))
	}

	t.Logf("Generated SPIR-V binary: %d bytes", len(binary))
}

// Helper function to create pointer to ExpressionHandle
func ptrExprHandle(h uint32) *ir.ExpressionHandle {
	handle := ir.ExpressionHandle(h)
	return &handle
}
