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

	// MSL requires [[position]] on struct member, not on function return
	// So we expect an output struct to be generated
	if !strings.Contains(result, "struct vs_main_Output") {
		t.Error("Expected output struct for vertex shader with builtin position")
	}
	if !strings.Contains(result, "[[position]]") {
		t.Error("Expected position attribute in output struct")
	}
	if !strings.Contains(result, "vertex vs_main_Output vs_main(") {
		t.Error("Expected vertex entry point to return output struct")
	}
	if !strings.Contains(result, "fragment metal::float4 fs_main(") {
		t.Error("Expected fragment entry point signature")
	}
	// [[position]] should NOT be on function signature, only on struct member
	if strings.Contains(result, ") [[position]] {") {
		t.Error("[[position]] should be on struct member, not on function signature")
	}
}

// TestMSL_PassThroughGlobals verifies that helper functions receiving
// texture/sampler globals get them as extra parameters, and call sites
// pass them as extra arguments. This is the fix for gogpu/ui#23 where
// MSL helper functions couldn't access entry point resource bindings.
func TestMSL_PassThroughGlobals(t *testing.T) {
	// Type handles:
	// 0: f32, 1: vec2f, 2: vec4f, 3: texture2d, 4: sampler
	tF32 := ir.TypeHandle(0)
	tVec2 := ir.TypeHandle(1)
	tVec4 := ir.TypeHandle(2)
	tTex := ir.TypeHandle(3)
	tSamp := ir.TypeHandle(4)

	binding0 := &ir.ResourceBinding{Group: 0, Binding: 0}
	binding1 := &ir.ResourceBinding{Group: 0, Binding: 1}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec2f", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "tex2d", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
			{Name: "samp", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "my_texture", Space: ir.SpaceHandle, Type: tTex, Binding: binding0},
			{Name: "my_sampler", Space: ir.SpaceHandle, Type: tSamp, Binding: binding1},
		},
		Functions: []ir.Function{
			{
				// Helper function: sample_tex(uv: vec2f) -> vec4f
				// References globals: my_texture (0), my_sampler (1)
				Name: "sample_tex",
				Arguments: []ir.FunctionArgument{
					{Name: "uv", Type: tVec2},
				},
				Result: &ir.FunctionResult{Type: tVec4},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},  // 0: uv
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // 1: my_texture
					{Kind: ir.ExprGlobalVariable{Variable: 1}}, // 2: my_sampler
					{Kind: ir.ExprImageSample{ // 3: textureSample
						Image: 1, Sampler: 2, Coordinate: 0,
						Level: ir.SampleLevelAuto{},
					}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec2}, // 0: uv
					{Handle: &tTex},  // 1: texture
					{Handle: &tSamp}, // 2: sampler
					{Handle: &tVec4}, // 3: sample result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			},
			{
				// Entry point function: calls sample_tex with hardcoded UV
				Name: "fs_entry",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: bindingPtr(ir.LocationBinding{Location: 0}),
				},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}}, // 0: 0.5
					{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}}, // 1: 0.5
					{Kind: ir.ExprCompose{ // 2: vec2(0.5, 0.5)
						Type: tVec2, Components: []ir.ExpressionHandle{0, 1},
					}},
					{Kind: ir.ExprCallResult{Function: 0}}, // 3: result of call
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},  // 0: literal
					{Handle: &tF32},  // 1: literal
					{Handle: &tVec2}, // 2: compose
					{Handle: &tVec4}, // 3: call result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					{Kind: ir.StmtCall{
						Function:  0, // call sample_tex
						Arguments: []ir.ExpressionHandle{2},
						Result:    ptrExpr(3),
					}},
					{Kind: ir.StmtReturn{Value: ptrExpr(3)}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name: "fs_main", Stage: ir.StageFragment,
				Function: 1, // references Functions[1]
			},
		},
	}

	result, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Helper function should have texture and sampler as extra params (no [[binding]])
	if !strings.Contains(result, "sample_tex(") {
		t.Fatal("Expected helper function sample_tex in output")
	}

	// Check that helper function has pass-through params
	if !strings.Contains(result, "my_texture") {
		t.Error("Expected my_texture in helper function params")
	}
	if !strings.Contains(result, "my_sampler") {
		t.Error("Expected my_sampler in helper function params")
	}

	// Helper function params should NOT have [[texture(N)]] or [[sampler(N)]] attributes
	// (those belong only on entry point params)
	lines := strings.Split(result, "\n")
	inHelper := false
	for _, line := range lines {
		if strings.Contains(line, "sample_tex(") {
			inHelper = true
		}
		if inHelper && strings.Contains(line, ") {") {
			inHelper = false
		}
		if inHelper {
			if strings.Contains(line, "[[texture(") || strings.Contains(line, "[[sampler(") {
				t.Errorf("Helper function should not have binding attributes, got: %s", line)
			}
		}
	}

	// Entry point should have [[texture(0)]] and [[sampler(0)]]
	// Sampler gets index 0 because Metal indices are sequential per resource type
	// (not the raw WGSL binding number), matching Rust wgpu-hal behavior.
	if !strings.Contains(result, "[[texture(0)]]") {
		t.Error("Expected [[texture(0)]] on entry point param")
	}
	if !strings.Contains(result, "[[sampler(0)]]") {
		t.Error("Expected [[sampler(0)]] on entry point param")
	}

	// Call site should pass globals as extra arguments
	// Look for something like: sample_tex(vec2f_expr, my_texture, my_sampler)
	foundCallWithGlobals := false
	for _, line := range lines {
		if strings.Contains(line, "sample_tex(") && strings.Contains(line, "my_texture") && strings.Contains(line, "my_sampler") {
			foundCallWithGlobals = true
			break
		}
	}
	if !foundCallWithGlobals {
		t.Error("Expected call site to pass my_texture and my_sampler as extra arguments")
	}

	t.Logf("Generated MSL:\n%s", result)
}

// TestMSL_MultiGroupBindingIndices verifies that globals from different bind
// groups get unique Metal buffer indices. Before the fix, @group(0) @binding(0)
// and @group(1) @binding(0) both mapped to [[buffer(0)]], causing a Metal
// shader compilation error. After the fix, they get sequential indices:
// [[buffer(0)]] and [[buffer(1)]].
//
// This is the regression test for gogpu/gg#209.
func TestMSL_MultiGroupBindingIndices(t *testing.T) {
	// Types: 0=f32, 1=vec4f, 2=Uniforms{viewport:vec4f}, 3=ClipParams{rect:vec4f}
	tVec4 := ir.TypeHandle(1)
	tUniforms := ir.TypeHandle(2)
	tClipParams := ir.TypeHandle(3)

	// Two buffers in different bind groups with the same binding number.
	group0Binding0 := &ir.ResourceBinding{Group: 0, Binding: 0}
	group1Binding0 := &ir.ResourceBinding{Group: 1, Binding: 0}

	// Fragment shader: reads from both uniforms and clip params.
	expressions := []ir.Expression{
		{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: uniforms
		{Kind: ir.ExprGlobalVariable{Variable: 1}},    // 1: clip_params
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 2: uniforms.viewport
		{Kind: ir.ExprLoad{Pointer: 2}},               // 3: load viewport
	}
	expressionTypes := []ir.TypeResolution{
		{Value: ir.PointerType{Base: tUniforms, Space: ir.SpaceUniform}},   // 0
		{Value: ir.PointerType{Base: tClipParams, Space: ir.SpaceUniform}}, // 1
		{Value: ir.PointerType{Base: tVec4, Space: ir.SpaceUniform}},       // 2
		{Handle: &tVec4}, // 3
	}

	retExpr := ir.ExpressionHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{
				Name: "Uniforms",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "viewport", Type: tVec4, Offset: 0},
					},
					Span: 16,
				},
			},
			{
				Name: "ClipParams",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "clip_rect", Type: tVec4, Offset: 0},
					},
					Span: 16,
				},
			},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "uniforms", Space: ir.SpaceUniform, Type: tUniforms, Binding: group0Binding0},
			{Name: "clip_params", Space: ir.SpaceUniform, Type: tClipParams, Binding: group1Binding0},
		},
		Functions: []ir.Function{
			{
				Name: "fs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: bindingPtr(ir.LocationBinding{Location: 0}),
				},
				Expressions:     expressions,
				ExpressionTypes: expressionTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
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

	// Key assertion: the two buffers MUST have different indices.
	if !strings.Contains(result, "[[buffer(0)]]") {
		t.Error("Expected [[buffer(0)]] for first buffer (group 0)")
	}
	if !strings.Contains(result, "[[buffer(1)]]") {
		t.Error("Expected [[buffer(1)]] for second buffer (group 1)")
	}

	// Count occurrences of [[buffer(0)]] — must be exactly 1.
	count := strings.Count(result, "[[buffer(0)]]")
	if count != 1 {
		t.Errorf("Expected exactly 1 occurrence of [[buffer(0)]], got %d", count)
	}

	// On macOS, verify the generated MSL actually compiles with Metal compiler.
	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, result)
	}

	t.Logf("Generated MSL:\n%s", result)
}

// TestMSL_MultiGroupMixedResourceTypes verifies sequential index assignment
// across multiple groups with mixed resource types (buffer + texture + sampler).
func TestMSL_MultiGroupMixedResourceTypes(t *testing.T) {
	tVec4 := ir.TypeHandle(1)
	tUniforms := ir.TypeHandle(3)
	tClipParams := ir.TypeHandle(4)
	tTex := ir.TypeHandle(5)
	tSamp := ir.TypeHandle(6)

	// group(0): binding(0)=Uniforms(buffer), binding(1)=texture, binding(2)=sampler
	// group(1): binding(0)=ClipParams(buffer)
	g0b0 := &ir.ResourceBinding{Group: 0, Binding: 0}
	g0b1 := &ir.ResourceBinding{Group: 0, Binding: 1}
	g0b2 := &ir.ResourceBinding{Group: 0, Binding: 2}
	g1b0 := &ir.ResourceBinding{Group: 1, Binding: 0}

	expressions := []ir.Expression{
		{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: uniforms
		{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // 1: uniforms.viewport
		{Kind: ir.ExprLoad{Pointer: 1}},               // 2: load viewport
	}
	expressionTypes := []ir.TypeResolution{
		{Value: ir.PointerType{Base: tUniforms, Space: ir.SpaceUniform}}, // 0
		{Value: ir.PointerType{Base: tVec4, Space: ir.SpaceUniform}},     // 1
		{Handle: &tVec4}, // 2
	}

	retExpr := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec2f", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{
				Name: "Uniforms",
				Inner: ir.StructType{
					Members: []ir.StructMember{{Name: "viewport", Type: tVec4, Offset: 0}},
					Span:    16,
				},
			},
			{
				Name: "ClipParams",
				Inner: ir.StructType{
					Members: []ir.StructMember{{Name: "clip_rect", Type: tVec4, Offset: 0}},
					Span:    16,
				},
			},
			{Name: "tex2d", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}},
			{Name: "samp", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "uniforms", Space: ir.SpaceUniform, Type: tUniforms, Binding: g0b0},
			{Name: "atlas", Space: ir.SpaceHandle, Type: tTex, Binding: g0b1},
			{Name: "atlas_sampler", Space: ir.SpaceHandle, Type: tSamp, Binding: g0b2},
			{Name: "clip_params", Space: ir.SpaceUniform, Type: tClipParams, Binding: g1b0},
		},
		Functions: []ir.Function{
			{
				Name: "fs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: bindingPtr(ir.LocationBinding{Location: 0}),
				},
				Expressions:     expressions,
				ExpressionTypes: expressionTypes,
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
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

	// Buffers: sequential across groups.
	// group(0) binding(0) Uniforms → buffer(0)
	// group(1) binding(0) ClipParams → buffer(1)
	if strings.Count(result, "[[buffer(0)]]") != 1 {
		t.Errorf("Expected exactly 1 [[buffer(0)]], got %d", strings.Count(result, "[[buffer(0)]]"))
	}
	if strings.Count(result, "[[buffer(1)]]") != 1 {
		t.Errorf("Expected exactly 1 [[buffer(1)]], got %d", strings.Count(result, "[[buffer(1)]]"))
	}

	// Textures and samplers: independent namespaces, each starts at 0.
	if !strings.Contains(result, "[[texture(0)]]") {
		t.Error("Expected [[texture(0)]] for atlas")
	}
	if !strings.Contains(result, "[[sampler(0)]]") {
		t.Error("Expected [[sampler(0)]] for atlas_sampler")
	}

	// Verify no duplicate indices within same resource type.
	if strings.Count(result, "[[buffer(0)]]") > 1 {
		t.Error("Duplicate [[buffer(0)]] — bind group collision!")
	}

	if runtime.GOOS == "darwin" {
		verifyMSLWithXcrun(t, result)
	}

	t.Logf("Generated MSL:\n%s", result)
}

func ptrExpr(h ir.ExpressionHandle) *ir.ExpressionHandle {
	return &h
}

func bindingPtr(b ir.Binding) *ir.Binding {
	return &b
}
