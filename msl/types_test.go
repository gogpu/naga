package msl

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: Struct type emission
// =============================================================================

func TestMSL_StructEmission(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{
				Name: "MyStruct",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "x", Type: 0, Offset: 0},
						{Name: "y", Type: 0, Offset: 4},
						{Name: "z", Type: 0, Offset: 8},
					},
					Span: 12,
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "struct MyStruct")
	mustContainMSL(t, result, "float x")
	mustContainMSL(t, result, "float y")
	mustContainMSL(t, result, "float z")
}

// =============================================================================
// Test: Array type (with wrapper struct for MSL)
// =============================================================================

func TestMSL_ArrayType(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	size := uint32(10)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "MyArray", Inner: ir.ArrayType{Base: tF32, Size: ir.ArraySize{Constant: &size}, Stride: 4}},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "struct MyArray")
}

// =============================================================================
// Test: Scalar type name helper
// =============================================================================

func TestMSL_ScalarTypeName_Extended(t *testing.T) {
	tests := []struct {
		name   string
		scalar ir.ScalarType
		want   string
	}{
		{"bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, "bool"},
		{"float16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, "half"},
		{"float32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "float"},
		{"float64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "double"},
		{"int8", ir.ScalarType{Kind: ir.ScalarSint, Width: 1}, "char"},
		{"int16", ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, "short"},
		{"int32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "int"},
		{"int64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "long"},
		{"uint8", ir.ScalarType{Kind: ir.ScalarUint, Width: 1}, "uchar"},
		{"uint16", ir.ScalarType{Kind: ir.ScalarUint, Width: 2}, "ushort"},
		{"uint32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "uint"},
		{"uint64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "ulong"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarTypeName(tt.scalar)
			if got != tt.want {
				t.Errorf("scalarTypeName(%+v) = %q, want %q", tt.scalar, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Vector type name helper
// =============================================================================

func TestMSL_VectorTypeName_Extended(t *testing.T) {
	tests := []struct {
		name   string
		vector ir.VectorType
		want   string
	}{
		{"float2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float2"},
		{"float3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float3"},
		{"float4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float4"},
		{"int2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "metal::int2"},
		{"int3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "metal::int3"},
		{"int4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "metal::int4"},
		{"uint2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "metal::uint2"},
		{"uint3", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "metal::uint3"},
		{"uint4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "metal::uint4"},
		{"bool2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}, "metal::bool2"},
		{"half2", ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "metal::half2"},
		{"half4", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "metal::half4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vectorTypeName(tt.vector)
			if got != tt.want {
				t.Errorf("vectorTypeName(%+v) = %q, want %q", tt.vector, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Matrix type name helper
// =============================================================================

func TestMSL_MatrixTypeName_Extended(t *testing.T) {
	tests := []struct {
		name   string
		matrix ir.MatrixType
		want   string
	}{
		{"float2x2", ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float2x2"},
		{"float3x3", ir.MatrixType{Columns: 3, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float3x3"},
		{"float4x4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float4x4"},
		{"float2x3", ir.MatrixType{Columns: 2, Rows: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float2x3"},
		{"float3x4", ir.MatrixType{Columns: 3, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "metal::float3x4"},
		{"half2x2", ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "metal::half2x2"},
		{"half4x4", ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, "metal::half4x4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matrixTypeName(tt.matrix)
			if got != tt.want {
				t.Errorf("matrixTypeName(%+v) = %q, want %q", tt.matrix, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Sampler and Image types in MSL
// =============================================================================

func TestMSL_SamplerType(t *testing.T) {
	tSampler := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	retExpr := ir.ExpressionHandle(1)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "s",
				Type:    tSampler,
				Space:   ir.SpaceHandle,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 1},
			},
		},
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tSampler},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "sampler")
}

// =============================================================================
// Test: writeTypeName for various types via Compile
// =============================================================================

func TestMSL_TypeInCompile(t *testing.T) {
	tests := []struct {
		name    string
		inner   ir.TypeInner
		members []ir.StructMember
		want    string
	}{
		{
			"vec4_in_struct",
			ir.StructType{
				Members: []ir.StructMember{
					{Name: "pos", Type: 0, Offset: 0},
				},
				Span: 16,
			},
			nil,
			"struct TestStruct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
					{Name: "TestStruct", Inner: tt.inner},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
		})
	}
}

// =============================================================================
// Test: Pointer type in expressions (local variable access)
// =============================================================================

func TestMSL_LocalVariableType(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(1)
	init0 := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				LocalVars: []ir.LocalVariable{
					{Name: "myvar", Type: tF32, Init: &init0},
				},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(3.14)}},
					{Kind: ir.ExprLocalVariable{Variable: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Value: ir.PointerType{Base: tF32, Space: ir.SpaceFunction}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "myvar")
}

// =============================================================================
// Test: Compute entry point with workgroup size
// =============================================================================

func TestMSL_ComputeEntryPoint(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "cs_main",
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "cs_main",
				Stage:     ir.StageCompute,
				Function:  0,
				Workgroup: [3]uint32{64, 1, 1},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "kernel")
	mustContainMSL(t, result, "cs_main")
}

// =============================================================================
// Test: Fragment entry point
// =============================================================================

func TestMSL_FragmentEntryPoint(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)
	var fragBinding ir.Binding = ir.LocationBinding{Location: 0}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "fs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &fragBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:     "fs_main",
				Stage:    ir.StageFragment,
				Function: 0,
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "fragment")
	mustContainMSL(t, result, "fs_main")
}

// =============================================================================
// Test: Global variable emission with binding (as entry point parameter)
// =============================================================================

func TestMSL_GlobalVariableAsEntryParam(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(1)
	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	tVec4 := ir.TypeHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "uniform_data",
				Type:    tF32,
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
			},
		},
		Functions: []ir.Function{
			{
				Name: "vs_main",
				Result: &ir.FunctionResult{
					Type:    tVec4,
					Binding: &posBinding,
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprZeroValue{Type: tVec4}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}
	result := compileModule(t, module)
	// Global variables appear as entry point function parameters in MSL
	mustContainMSL(t, result, "uniform_data")
	mustContainMSL(t, result, "[[buffer(")
}

// =============================================================================
// Test: Address space name
// =============================================================================

func TestMSL_AddressSpaceName_Extended(t *testing.T) {
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
		{ir.SpacePushConstant, "constant"},
	}

	for _, tt := range tests {
		got := addressSpaceName(tt.space)
		if got != tt.want {
			t.Errorf("addressSpaceName(%v) = %q, want %q", tt.space, got, tt.want)
		}
	}
}

// =============================================================================
// Test: Image type names
// =============================================================================

func TestMSL_ImageType(t *testing.T) {
	tests := []struct {
		name string
		img  ir.ImageType
		want string
	}{
		{"texture2d_sampled",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled},
			"texture2d"},
		{"texture3d_sampled",
			ir.ImageType{Dim: ir.Dim3D, Class: ir.ImageClassSampled},
			"texture3d"},
		{"texture1d_sampled",
			ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled},
			"texture1d"},
		{"texturecube_sampled",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled},
			"texturecube"},
		{"depth2d",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth},
			"depth2d"},
		{"depth2d_ms",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassDepth, Multisampled: true},
			"depth2d_ms"},
		{"depthcube",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassDepth},
			"depthcube"},
		{"texture2d_ms",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Multisampled: true},
			"texture2d_ms"},
		{"texture2d_array",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, Arrayed: true},
			"texture2d_array"},
		{"texturecube_array",
			ir.ImageType{Dim: ir.DimCube, Class: ir.ImageClassSampled, Arrayed: true},
			"texturecube_array"},
		{"texture1d_array",
			ir.ImageType{Dim: ir.Dim1D, Class: ir.ImageClassSampled, Arrayed: true},
			"texture1d_array"},
		{"storage_rw",
			ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassStorage},
			"read_write"},
	}

	// Create a minimal writer with an empty module
	w := &Writer{
		module: &ir.Module{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.imageTypeName(tt.img, StorageAccess(0))
			if !containsStr(got, tt.want) {
				t.Errorf("imageTypeName(%+v) = %q, want substring %q", tt.img, got, tt.want)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && containsSubstring(s, sub)
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// =============================================================================
// Test: Atomic type names
// =============================================================================

func TestMSL_AtomicTypeName(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	tests := []struct {
		name   string
		atomic ir.AtomicType
		want   string
	}{
		{"sint32", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, "atomic_int"},
		{"sint64", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}}, "atomic<long>"},
		{"uint32", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, "atomic_uint"},
		{"uint64", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 8}}, "atomic<ulong>"},
		{"default", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, "atomic_uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.atomicTypeName(tt.atomic)
			if !containsStr(got, tt.want) {
				t.Errorf("atomicTypeName(%+v) = %q, want substring %q", tt.atomic, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Test: Packed vector type name
// =============================================================================

func TestMSL_PackedVectorTypeName(t *testing.T) {
	w := &Writer{module: &ir.Module{}}

	got := w.packedVectorTypeName(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	if !containsStr(got, "packed_float3") {
		t.Errorf("packedVectorTypeName(float) = %q, want substring %q", got, "packed_float3")
	}

	got = w.packedVectorTypeName(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})
	if !containsStr(got, "packed_int3") {
		t.Errorf("packedVectorTypeName(int) = %q, want substring %q", got, "packed_int3")
	}
}

// =============================================================================
// Test: Constants emission
// =============================================================================

func TestMSL_ConstantEmission(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "PI", Type: tF32, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40490fdb}}, // ~3.14
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "constant")
	mustContainMSL(t, result, "PI")
}

// =============================================================================
// Test: Vertex entry point with location inputs and builtin output
// =============================================================================

func TestMSL_VertexEntryPointWithInputs(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	tVec2 := ir.TypeHandle(2)

	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var loc0Binding ir.Binding = ir.LocationBinding{Location: 0}
	var loc1Binding ir.Binding = ir.LocationBinding{Location: 1}
	var vertIdxBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinVertexIndex}

	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec2f", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "vs_main",
			Arguments: []ir.FunctionArgument{
				{Name: "position", Type: tVec4, Binding: &loc0Binding},
				{Name: "texcoord", Type: tVec2, Binding: &loc1Binding},
				{Name: "vid", Type: tF32, Binding: &vertIdxBinding},
			},
			Result: &ir.FunctionResult{
				Type:    tVec4,
				Binding: &posBinding,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tVec4},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "vertex")
	mustContainMSL(t, result, "[[stage_in]]")
	mustContainMSL(t, result, "[[attribute(0)]]")
	mustContainMSL(t, result, "[[attribute(1)]]")
	mustContainMSL(t, result, "[[vertex_id]]")
	mustContainMSL(t, result, "[[position]]")
}

// =============================================================================
// Test: Fragment entry point with location inputs
// =============================================================================

func TestMSL_FragmentEntryPointWithInputs(t *testing.T) {
	tVec4 := ir.TypeHandle(0)
	tVec2 := ir.TypeHandle(1)

	var loc0Binding ir.Binding = ir.LocationBinding{Location: 0}
	var fragPosBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var fragFacingBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinFrontFacing}

	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec2f", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{{
			Name: "fs_main",
			Arguments: []ir.FunctionArgument{
				{Name: "uv", Type: tVec2, Binding: &loc0Binding},
				{Name: "frag_pos", Type: tVec4, Binding: &fragPosBinding},
				{Name: "front_facing", Type: ir.TypeHandle(2), Binding: &fragFacingBinding},
			},
			Result: &ir.FunctionResult{
				Type:    tVec4,
				Binding: &loc0Binding,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprFunctionArgument{Index: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tVec2},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "fs_main", Stage: ir.StageFragment, Function: 0},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "fragment")
	mustContainMSL(t, result, "[[position]]")
	mustContainMSL(t, result, "[[front_facing]]")
}

// =============================================================================
// Test: Compute entry point with workgroup attributes
// =============================================================================

func TestMSL_ComputeEntryPointWithBuiltins(t *testing.T) {
	tVec3U32 := ir.TypeHandle(0)

	var globalIDBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID}
	var localIDBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinLocalInvocationID}
	var workgroupIDBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinWorkGroupID}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "vec3u", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		Functions: []ir.Function{{
			Name: "cs_main",
			Arguments: []ir.FunctionArgument{
				{Name: "global_id", Type: tVec3U32, Binding: &globalIDBinding},
				{Name: "local_id", Type: tVec3U32, Binding: &localIDBinding},
				{Name: "workgroup_id", Type: tVec3U32, Binding: &workgroupIDBinding},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtReturn{}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "cs_main", Stage: ir.StageCompute, Function: 0, Workgroup: [3]uint32{64, 1, 1}},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "kernel")
	mustContainMSL(t, result, "[[thread_position_in_grid]]")
	mustContainMSL(t, result, "[[thread_position_in_threadgroup]]")
	mustContainMSL(t, result, "[[threadgroup_position_in_grid]]")
}

// =============================================================================
// Test: Vertex entry point with struct output
// =============================================================================

func TestMSL_VertexStructOutput(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	tVec2 := ir.TypeHandle(2)

	var posBinding ir.Binding = ir.BuiltinBinding{Builtin: ir.BuiltinPosition}
	var loc0Binding ir.Binding = ir.LocationBinding{Location: 0}

	outStructIdx := ir.TypeHandle(3)

	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec4f", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "vec2f", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{
				Name: "VsOutput",
				Inner: ir.StructType{
					Members: []ir.StructMember{
						{Name: "position", Type: tVec4, Binding: &posBinding},
						{Name: "uv", Type: tVec2, Binding: &loc0Binding},
					},
				},
			},
		},
		Functions: []ir.Function{{
			Name: "vs_main",
			Result: &ir.FunctionResult{
				Type: outStructIdx,
			},
			Expressions: []ir.Expression{
				{Kind: ir.ExprZeroValue{Type: outStructIdx}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &outStructIdx},
			},
			Body: []ir.Statement{
				{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				{Kind: ir.StmtReturn{Value: &retExpr}},
			},
		}},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs_main", Stage: ir.StageVertex, Function: 0},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "vertex")
	mustContainMSL(t, result, "[[position]]")
	_ = tF32
}
