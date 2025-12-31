package msl

import (
	"runtime"
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

	result, info, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(info.EntryPointNames) != 0 {
		t.Errorf("EntryPointNames length = %d, want 0", len(info.EntryPointNames))
	}

	// Check that struct is defined
	if !strings.Contains(result, "struct ") {
		t.Error("Expected struct definition in output")
	}
}

func TestCompile_ArrayWrapperAccess(t *testing.T) {
	size := uint32(3)
	tF32 := ir.TypeHandle(0)
	tVec2 := ir.TypeHandle(1)
	tArr := ir.TypeHandle(2)
	tU32 := ir.TypeHandle(3)

	expressions := []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                                  // 0
		{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                                  // 1
		{Kind: ir.Literal{Value: ir.LiteralF32(-0.5)}},                                 // 2
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{0, 1}}},   // 3
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{2, 2}}},   // 4
		{Kind: ir.ExprCompose{Type: tVec2, Components: []ir.ExpressionHandle{1, 2}}},   // 5
		{Kind: ir.ExprCompose{Type: tArr, Components: []ir.ExpressionHandle{3, 4, 5}}}, // 6
		{Kind: ir.ExprLocalVariable{Variable: 0}},                                      // 7
		{Kind: ir.ExprLoad{Pointer: 7}},                                                // 8
		{Kind: ir.Literal{Value: ir.LiteralU32(1)}},                                    // 9
		{Kind: ir.ExprAccess{Base: 8, Index: 9}},                                       // 10
	}

	expressionTypes := []ir.TypeResolution{
		{Handle: &tF32},  // 0
		{Handle: &tF32},  // 1
		{Handle: &tF32},  // 2
		{Handle: &tVec2}, // 3
		{Handle: &tVec2}, // 4
		{Handle: &tVec2}, // 5
		{Handle: &tArr},  // 6
		{Value: ir.PointerType{Base: tArr, Space: ir.SpaceFunction}}, // 7
		{Handle: &tArr},  // 8
		{Handle: &tU32},  // 9
		{Handle: &tVec2}, // 10
	}

	posInit := ir.ExpressionHandle(6)
	valInit := ir.ExpressionHandle(10)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Positions", Inner: ir.ArrayType{Base: tVec2, Size: ir.ArraySize{Constant: &size}, Stride: 8}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				LocalVars: []ir.LocalVariable{
					{Name: "positions", Type: tArr, Init: &posInit},
					{Name: "value", Type: tVec2, Init: &valInit},
				},
				Expressions:     expressions,
				ExpressionTypes: expressionTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "struct Positions") {
		t.Error("Expected array wrapper struct definition in output")
	}
	if !strings.Contains(result, "Positions positions = Positions{{") {
		t.Error("Expected array wrapper initialization with nested braces")
	}
	if !strings.Contains(result, ".inner[") {
		t.Error("Expected array wrapper indexing to use .inner")
	}
}

func TestCompile_EntryPointStructReturnMapping(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	tVec3 := ir.TypeHandle(1)
	tStruct := ir.TypeHandle(2)

	expressions := []ir.Expression{
		{Kind: ir.ExprLocalVariable{Variable: 0}},     // 0
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // 2
		{Kind: ir.ExprZeroValue{Type: tVec4}},         // 3
		{Kind: ir.ExprZeroValue{Type: tVec3}},         // 4
		{Kind: ir.ExprLoad{Pointer: 0}},               // 5
	}

	expressionTypes := []ir.TypeResolution{
		{Value: ir.PointerType{Base: tStruct, Space: ir.SpaceFunction}}, // 0
		{Value: ir.PointerType{Base: tVec4, Space: ir.SpaceFunction}},   // 1
		{Value: ir.PointerType{Base: tVec3, Space: ir.SpaceFunction}},   // 2
		{Handle: &tVec4},   // 3
		{Handle: &tVec3},   // 4
		{Handle: &tStruct}, // 5
	}

	retExpr := ir.ExpressionHandle(5)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{
				Name: "VertexOutput",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: tVec4, Offset: 0},
						{Name: "color", Type: tVec3, Offset: 16},
					},
					Span: 28,
				},
			},
		},
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type: tStruct,
				},
				LocalVars: []ir.LocalVariable{
					{Name: "output", Type: tStruct},
				},
				Expressions:     expressions,
				ExpressionTypes: expressionTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtStore{Pointer: 1, Value: 3}},
					{Kind: ir.StmtStore{Pointer: 2, Value: 4}},
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if strings.Contains(result, "return output;") {
		t.Error("Did not expect entry point to return the undecorated struct")
	}
	if !strings.Contains(result, "return _output;") {
		t.Error("Expected entry point to return the output struct with attributes")
	}
	if strings.Contains(result, "*output.position_") {
		t.Error("Did not expect pointer dereference for struct member stores")
	}
	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, result)
	}
}

func TestCompile_FragmentStageInStructInput(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	tVec3 := ir.TypeHandle(1)
	tStruct := ir.TypeHandle(2)

	expressions := []ir.Expression{
		{Kind: ir.ExprFunctionArgument{Index: 0}},     // 0
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // 1
	}

	expressionTypes := []ir.TypeResolution{
		{Handle: &tStruct}, // 0
		{Handle: &tVec3},   // 1
	}

	retExpr := ir.ExpressionHandle(1)
	var fragmentBinding ir.Binding = ir.LocationBinding{Location: 0}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{
				Name: "VertexOutput",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: tVec4, Offset: 0},
						{Name: "color", Type: tVec3, Offset: 16},
					},
					Span: 28,
				},
			},
		},
		Functions: []ir.Function{
			{
				Name: "fs_main",
				Arguments: []ir.FunctionArgument{
					{Name: "input", Type: tStruct},
				},
				Result: &ir.FunctionResult{
					Type:    tVec3,
					Binding: &fragmentBinding,
				},
				Expressions:     expressions,
				ExpressionTypes: expressionTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "fs_main_Input _input [[stage_in]]") {
		t.Error("Expected stage_in struct parameter for fragment input")
	}
	if !strings.Contains(result, "auto input = _input;") {
		t.Error("Expected fragment input alias to stage_in struct")
	}
	if !strings.Contains(result, "input.color_") {
		t.Error("Expected fragment shader to access input struct member")
	}
	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, result)
	}
}

func TestCompile_EntryPointReturnAttributePlacement(t *testing.T) {
	tVec4 := ir.TypeHandle(0)

	retExpr := ir.ExpressionHandle(0)
	expressions := []ir.Expression{
		{Kind: ir.ExprZeroValue{Type: tVec4}},
	}
	exprTypes := []ir.TypeResolution{
		{Handle: &tVec4},
	}

	var vertexBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var fragmentBinding ir.Binding = ir.LocationBinding{Location: 0}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name:            "vs_main",
				Result:          &ir.FunctionResult{Type: tVec4, Binding: &vertexBinding},
				Expressions:     expressions,
				ExpressionTypes: exprTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
			{
				Name:            "fs_main",
				Result:          &ir.FunctionResult{Type: tVec4, Binding: &fragmentBinding},
				Expressions:     expressions,
				ExpressionTypes: exprTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
			{Name: "fs_main", Stage: ir.StageFragment, Function: 1},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(result, "vertex metal::float4 vs_main(") {
		t.Error("Expected vertex entry point signature")
	}
	if !strings.Contains(result, "fragment metal::float4 fs_main(") {
		t.Error("Expected fragment entry point signature")
	}
	if strings.Contains(result, "[[position]]") {
		t.Error("Did not expect position attribute on scalar return")
	}
	if strings.Contains(result, "[[color(0)]]") {
		t.Error("Did not expect color attribute on scalar return")
	}
}
