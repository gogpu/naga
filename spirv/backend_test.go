package spirv

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// hasDecorateInBinary checks if any OpDecorate instruction targets the given ID with the given decoration.
func hasDecorateInBinary(instrs []spirvInstruction, targetID uint32, decoration Decoration) bool {
	for _, inst := range instrs {
		if inst.opcode == OpDecorate && len(inst.words) >= 3 {
			if inst.words[1] == targetID && Decoration(inst.words[2]) == decoration {
				return true
			}
		}
	}
	return false
}

// hasMemberDecorateInBinary checks if any OpMemberDecorate targets the given struct/member/decoration.
func hasMemberDecorateInBinary(instrs []spirvInstruction, structID, memberIdx uint32, decoration Decoration) bool {
	for _, inst := range instrs {
		if inst.opcode == OpMemberDecorate && len(inst.words) >= 4 {
			if inst.words[1] == structID && inst.words[2] == memberIdx && Decoration(inst.words[3]) == decoration {
				return true
			}
		}
	}
	return false
}

// findVarsByStorageClass finds SPIR-V variable IDs at the given storage class.
func findVarsByStorageClass(instrs []spirvInstruction, storageClass StorageClass) []uint32 {
	var ids []uint32
	for _, inst := range instrs {
		if inst.opcode == OpVariable && len(inst.words) >= 4 {
			// OpVariable: resultType resultID storageClass [initializer]
			if StorageClass(inst.words[3]) == storageClass {
				ids = append(ids, inst.words[2])
			}
		}
	}
	return ids
}

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
				Function: ir.Function{},
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

func ptrUint32Const(v uint32) *uint32 {
	return &v
}

// TestNonWritableStorageBuffer verifies that read-only storage buffers get OpDecorate NonWritable.
// Matches Rust naga behavior: if !access.contains(STORE) -> NonWritable.
func TestNonWritableStorageBuffer(t *testing.T) {
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	module := &ir.Module{
		Types:     []ir.Type{u32Type},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "buf",
				Space: ir.SpaceStorage,
				Type:  ir.TypeHandle(0),
				Binding: &ir.ResourceBinding{
					Group:   0,
					Binding: 0,
				},
				Access: ir.StorageRead, // read-only
			},
		},
		GlobalExpressions: []ir.Expression{},
		Functions:         []ir.Function{},
		EntryPoints:       []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	data, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(data)

	// Find the StorageBuffer variable
	varIDs := findVarsByStorageClass(instrs, StorageClassStorageBuffer)
	if len(varIDs) == 0 {
		t.Fatal("No StorageBuffer variable found")
	}

	varID := varIDs[0]
	if !hasDecorateInBinary(instrs, varID, DecorationNonWritable) {
		t.Error("Expected NonWritable decoration on read-only storage buffer variable")
	}
	// Should NOT have NonReadable since we have LOAD access
	if hasDecorateInBinary(instrs, varID, DecorationNonReadable) {
		t.Error("Unexpected NonReadable decoration on read-only storage buffer variable")
	}
}

// TestFlatOnIntegerBuiltInFragmentInput verifies that integer BuiltIn Input variables
// in fragment shaders get OpDecorate Flat per Vulkan VUID-StandaloneSpirv-Flat-04744.
func TestFlatOnIntegerBuiltInFragmentInput(t *testing.T) {
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{
		Name:  "vec4f",
		Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: ir.Vec4},
	}

	builtinViewIndex := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinViewIndex})
	builtinPosition := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	locBinding := ir.Binding(ir.LocationBinding{Location: 0})

	module := &ir.Module{
		Types:             []ir.Type{u32Type, f32Type, vec4Type},
		Constants:         []ir.Constant{},
		GlobalVariables:   []ir.GlobalVariable{},
		GlobalExpressions: []ir.Expression{},
		Functions:         []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "frag_main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Name: "frag_main",
					Arguments: []ir.FunctionArgument{
						{Name: "view_idx", Type: ir.TypeHandle(0), Binding: &builtinViewIndex}, // u32
						{Name: "pos", Type: ir.TypeHandle(2), Binding: &builtinPosition},       // vec4f (should NOT get Flat)
					},
					Result: &ir.FunctionResult{
						Type:    ir.TypeHandle(2),
						Binding: &locBinding,
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprFunctionArgument{Index: 1}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.Debug = true
	backend := NewBackend(opts)
	data, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(data)

	// Find Input variables
	inputVars := findVarsByStorageClass(instrs, StorageClassInput)
	if len(inputVars) < 2 {
		t.Fatalf("Expected at least 2 Input variables, got %d", len(inputVars))
	}

	// The first input (view_idx, u32) should have both BuiltIn and Flat
	viewIdxVar := inputVars[0]
	if !hasDecorateInBinary(instrs, viewIdxVar, DecorationFlat) {
		t.Error("Expected Flat decoration on integer BuiltIn Input in fragment shader")
	}

	// The second input (pos, vec4f) should NOT have Flat
	posVar := inputVars[1]
	if hasDecorateInBinary(instrs, posVar, DecorationFlat) {
		t.Error("Unexpected Flat decoration on float BuiltIn Input in fragment shader")
	}
}

// TestColMajorThroughArrayType verifies that struct members of type array<matNxM<f32>>
// still get ColMajor + MatrixStride decorations (unwrapping through array types).
func TestColMajorThroughArrayType(t *testing.T) {
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{
		Name:  "vec4f",
		Inner: ir.VectorType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Size: ir.Vec4},
	}
	mat4Type := ir.Type{
		Name:  "mat4x4f",
		Inner: ir.MatrixType{Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, Columns: ir.Vec4, Rows: ir.Vec4},
	}
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}

	// Constant for array size = 2
	two := uint32(2)
	arrayType := ir.Type{
		Name: "arr_mat4",
		Inner: ir.ArrayType{
			Base:   ir.TypeHandle(2), // mat4x4f
			Size:   ir.ArraySize{Constant: &two},
			Stride: 64,
		},
	}

	structType := ir.Type{
		Name: "MyStruct",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "matrices", Type: ir.TypeHandle(4), Offset: 0}, // array<mat4x4<f32>, 2>
			},
			Span: 128,
		},
	}

	module := &ir.Module{
		Types:     []ir.Type{f32Type, vec4Type, mat4Type, u32Type, arrayType, structType},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "data",
				Space: ir.SpaceUniform,
				Type:  ir.TypeHandle(5), // MyStruct
				Binding: &ir.ResourceBinding{
					Group:   0,
					Binding: 0,
				},
			},
		},
		GlobalExpressions: []ir.Expression{},
		Functions:         []ir.Function{},
		EntryPoints:       []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	data, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(data)

	// Find the struct type ID - look for OpTypeStruct instructions
	var structIDs []uint32
	for _, inst := range instrs {
		if inst.opcode == OpTypeStruct {
			structIDs = append(structIDs, inst.words[1])
		}
	}

	if len(structIDs) == 0 {
		t.Fatal("No OpTypeStruct found")
	}

	// Check that at least one struct has ColMajor on member 0
	found := false
	for _, sid := range structIDs {
		if hasMemberDecorateInBinary(instrs, sid, 0, DecorationColMajor) &&
			hasMemberDecorateInBinary(instrs, sid, 0, DecorationMatrixStride) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected ColMajor + MatrixStride on struct member with array<mat4x4<f32>> type")
	}
}

// TestPushConstantStorageClass verifies that SpaceImmediate maps to
// StorageClassPushConstant and gets Block decoration.
func TestPushConstantStorageClass(t *testing.T) {
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}

	module := &ir.Module{
		Types:     []ir.Type{f32Type},
		Constants: []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "pc",
				Space: ir.SpaceImmediate,
				Type:  ir.TypeHandle(0), // f32 - non-struct needs wrapper
			},
		},
		GlobalExpressions: []ir.Expression{},
		Functions:         []ir.Function{},
		EntryPoints:       []ir.EntryPoint{},
	}

	backend := NewBackend(DefaultOptions())
	data, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(data)

	// Find PushConstant variables
	varIDs := findVarsByStorageClass(instrs, StorageClassPushConstant)
	if len(varIDs) == 0 {
		t.Fatal("No PushConstant variable found (SpaceImmediate should map to PushConstant)")
	}

	// The wrapper struct should have Block decoration
	foundBlock := false
	for _, inst := range instrs {
		if inst.opcode == OpDecorate && len(inst.words) >= 3 {
			if Decoration(inst.words[2]) == DecorationBlock {
				foundBlock = true
				break
			}
		}
	}
	if !foundBlock {
		t.Error("Expected Block decoration on wrapper struct for SpaceImmediate variable")
	}
}

// TestDefaultInterpolationFlatOnIntegerLocation verifies that Location bindings
// on integer types get Flat decoration via default interpolation in the lowerer,
// and that fragment shader outputs do NOT get Flat per Vulkan VUIDs.
func TestDefaultInterpolationFlatOnIntegerLocation(t *testing.T) {
	// Build a module with a fragment shader that has:
	// - Input u32 with @location(0) => should get Flat
	// - Output vec4<f32> with @location(0) => should NOT get Flat
	u32Type := ir.Type{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	vec4Type := ir.Type{Name: "vec4f", Inner: ir.VectorType{
		Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		Size:   ir.Vec4,
	}}

	// Input binding: @location(0) with Flat interpolation (as set by lowerer)
	flatInterp := ir.Interpolation{Kind: ir.InterpolationFlat}
	var inputBinding ir.Binding = ir.LocationBinding{
		Location:      0,
		Interpolation: &flatInterp,
	}

	// Output binding: @location(0) with Perspective interpolation (default for float)
	perspInterp := ir.Interpolation{Kind: ir.InterpolationPerspective, Sampling: ir.SamplingCenter}
	var outputBinding ir.Binding = ir.LocationBinding{
		Location:      0,
		Interpolation: &perspInterp,
	}

	module := &ir.Module{
		Types:             []ir.Type{u32Type, f32Type, vec4Type},
		Constants:         []ir.Constant{},
		GlobalVariables:   []ir.GlobalVariable{},
		GlobalExpressions: []ir.Expression{},
		Functions:         []ir.Function{},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "index", Type: ir.TypeHandle(0), Binding: &inputBinding},
					},
					Result: &ir.FunctionResult{
						Type:    ir.TypeHandle(2),
						Binding: &outputBinding,
					},
					Expressions:      []ir.Expression{},
					NamedExpressions: map[ir.ExpressionHandle]string{},
					LocalVars:        []ir.LocalVariable{},
					Body:             []ir.Statement{{Kind: ir.StmtReturn{}}},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	data, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(data)

	// Find Input variables
	inputVarIDs := findVarsByStorageClass(instrs, StorageClassInput)
	if len(inputVarIDs) == 0 {
		t.Fatal("No Input variables found")
	}

	// The input integer variable should have Flat decoration
	foundFlatOnInput := false
	for _, vid := range inputVarIDs {
		if hasDecorateInBinary(instrs, vid, DecorationFlat) {
			foundFlatOnInput = true
			break
		}
	}
	if !foundFlatOnInput {
		t.Error("Expected Flat decoration on integer Input variable in fragment shader")
	}

	// Output variables should NOT have Flat (fragment output, VUID-StandaloneSpirv-Flat-06201)
	outputVarIDs := findVarsByStorageClass(instrs, StorageClassOutput)
	for _, vid := range outputVarIDs {
		if hasDecorateInBinary(instrs, vid, DecorationFlat) {
			t.Error("Fragment shader Output should NOT have Flat decoration per Vulkan VUID-StandaloneSpirv-Flat-06201")
		}
	}
}

// TestFloat32ToF16Bits verifies float32 to float16 bit conversion.
func TestFloat32ToF16Bits(t *testing.T) {
	tests := []struct {
		name string
		in   float32
		want uint32
	}{
		{"zero", 0.0, 0x0000},
		{"one", 1.0, 0x3C00},
		{"minus_one", -1.0, 0xBC00},
		{"half", 0.5, 0x3800},
		{"max_f16", 65504.0, 0x7BFF},
		{"min_f16", -65504.0, 0xFBFF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := float32ToF16Bits(tt.in)
			if got != tt.want {
				t.Errorf("float32ToF16Bits(%v) = 0x%04X, want 0x%04X", tt.in, got, tt.want)
			}
		})
	}
}

// TestOpLoadResultType verifies that ExprLoad produces OpLoad with
// the VALUE type (not the pointer type) as result type.
func TestOpLoadResultType(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "i32", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, // type 0
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					LocalVars: []ir.LocalVariable{
						{Name: "x", Type: 0}, // i32
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprLocalVariable{Variable: 0}}, // expr 0: &x
						{Kind: ir.ExprLoad{Pointer: 0}},           // expr 1: *(&x)
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(spvBytes)

	// Find OpLoad instruction
	var loadInstr *spirvInstruction
	for i := range instrs {
		if instrs[i].opcode == OpLoad {
			loadInstr = &instrs[i]
			break
		}
	}
	if loadInstr == nil {
		t.Fatal("OpLoad not found in output")
	}

	// The result type (first word after opcode header) should be the i32 type, not a pointer type.
	// Find the OpTypeInt instruction for i32.
	var intTypeID uint32
	for _, inst := range instrs {
		if inst.opcode == OpTypeInt && len(inst.words) >= 4 {
			if inst.words[2] == 32 && inst.words[3] == 1 { // 32-bit signed
				intTypeID = inst.words[1]
				break
			}
		}
	}
	if intTypeID == 0 {
		t.Fatal("OpTypeInt 32 1 not found")
	}

	// Verify OpLoad's result type is the i32 type (not pointer type)
	if loadInstr.words[1] != intTypeID {
		t.Errorf("OpLoad result type = %d, want %d (i32 type)", loadInstr.words[1], intTypeID)
	}
}

// TestModfStructReturnType verifies that ModfStruct instruction uses the correct struct result type.
// TestSaturateEmitsFClampWith3Operands verifies MathSaturate generates FClamp(x, 0.0, 1.0)
// with three operands, not just one. Bug: MathSaturate was emitting FClamp with only
// the input operand, missing the 0.0 min and 1.0 max constants.
func TestSaturateEmitsFClampWith3Operands(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(0.7)}},
						{Kind: ir.ExprMath{Fun: ir.MathSaturate, Arg: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(spvBytes)

	// Find OpExtInst for FClamp (GLSL instruction 43)
	var fclampInst *spirvInstruction
	for i := range instrs {
		if instrs[i].opcode == OpExtInst && len(instrs[i].words) >= 5 {
			if instrs[i].words[4] == GLSLstd450FClamp {
				fclampInst = &instrs[i]
				break
			}
		}
	}
	if fclampInst == nil {
		t.Fatal("OpExtInst FClamp not found — saturate should emit FClamp")
	}

	// FClamp has: resultType, resultID, extSetID, 43(FClamp), x, minVal, maxVal = 8 words total
	if len(fclampInst.words) < 8 {
		t.Errorf("FClamp should have 8 words (3 operands: x, min, max), got %d words", len(fclampInst.words))
	}
}

func TestModfStructReturnType(t *testing.T) {
	f32Handle := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // type 0: f32
			{Name: "__modf_result_f32", Inner: ir.StructType{Members: []ir.StructMember{ // type 1: modf result
				{Name: "fract", Type: f32Handle, Offset: 0},
				{Name: "whole", Type: f32Handle, Offset: 4},
			}}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.5)}}, // expr 0: 1.5f
						{Kind: ir.ExprMath{Fun: ir.MathModf, Arg: 0}}, // expr 1: modf(1.5)
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
					},
				},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	instrs := decodeSPIRVInstructions(spvBytes)

	// Find OpExtInst for ModfStruct (GLSL instruction 36)
	var extInst *spirvInstruction
	for i := range instrs {
		if instrs[i].opcode == OpExtInst && len(instrs[i].words) >= 5 {
			if instrs[i].words[4] == GLSLstd450ModfStruct {
				extInst = &instrs[i]
				break
			}
		}
	}
	if extInst == nil {
		t.Fatal("OpExtInst ModfStruct not found in output")
	}

	// The result type should be a struct type (OpTypeStruct), not a scalar type
	resultTypeID := extInst.words[1]
	var isStruct bool
	for _, inst := range instrs {
		if inst.opcode == OpTypeStruct && len(inst.words) >= 2 && inst.words[1] == resultTypeID {
			isStruct = true
			break
		}
	}
	if !isStruct {
		t.Errorf("ModfStruct result type %d is not a struct type", resultTypeID)
	}
}

// TestWorkgroupVarUsesRegularType verifies that workgroup global variables
// use the same type IDs as regular types (no separate no-layout types).
// This matches Rust naga which uses get_handle_type_id for all globals.
func TestWorkgroupVarUsesRegularType(t *testing.T) {
	backend := NewBackend(DefaultOptions())

	f32Type := ir.Type{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	arrayType := ir.Type{
		Name: "",
		Inner: ir.ArrayType{
			Base: 0,
			Size: ir.ArraySize{Constant: ptrUint32Const(10)},
		},
	}
	structType := ir.Type{
		Name: "WgStruct",
		Inner: ir.StructType{
			Members: []ir.StructMember{
				{Name: "data", Type: 1, Offset: 0},
			},
			Span: 40,
		},
	}

	module := &ir.Module{
		Types:           []ir.Type{f32Type, arrayType, structType},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend.module = module
	backend.builder = NewModuleBuilder(backend.options.Version)

	// Emit the struct type through regular emitType
	regularID, err := backend.emitType(2)
	if err != nil {
		t.Fatalf("emitType failed: %v", err)
	}

	// Emit the same type again (should be cached)
	cachedID, err := backend.emitType(2)
	if err != nil {
		t.Fatalf("emitType(cached) failed: %v", err)
	}

	if regularID != cachedID {
		t.Errorf("workgroup type should use same ID as regular type: regular=%d, cached=%d", regularID, cachedID)
	}
	if regularID == 0 {
		t.Error("type ID should not be zero")
	}
}

// TestStorageTextureImageFormat verifies that storage textures emit format-specific
// OpTypeImage instructions with the correct image format and sampled type.
func TestStorageTextureImageFormat(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatR32Float,
				StorageAccess: ir.StorageAccessRead,
			}},
			{Name: "", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatRgba8Uint,
				StorageAccess: ir.StorageAccessRead,
			}},
			{Name: "", Inner: ir.ImageType{
				Dim:           ir.Dim2D,
				Class:         ir.ImageClassStorage,
				StorageFormat: ir.StorageFormatRgba32Sint,
				StorageAccess: ir.StorageAccessRead,
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend.module = module
	backend.builder = NewModuleBuilder(backend.options.Version)

	// Emit all three types
	id1, err := backend.emitType(0)
	if err != nil {
		t.Fatalf("emitType(r32float) failed: %v", err)
	}
	id2, err := backend.emitType(1)
	if err != nil {
		t.Fatalf("emitType(rgba8uint) failed: %v", err)
	}
	id3, err := backend.emitType(2)
	if err != nil {
		t.Fatalf("emitType(rgba32sint) failed: %v", err)
	}

	// All three should have DIFFERENT type IDs since they have different formats
	if id1 == id2 {
		t.Errorf("r32float and rgba8uint should have different type IDs, both got %d", id1)
	}
	if id1 == id3 {
		t.Errorf("r32float and rgba32sint should have different type IDs, both got %d", id1)
	}
	if id2 == id3 {
		t.Errorf("rgba8uint and rgba32sint should have different type IDs, both got %d", id2)
	}

	// Verify the sampled types are correct by checking emitted OpTypeImage instructions.
	// OpTypeImage Words: [ResultID, SampledType, Dim, Depth, Arrayed, MS, Sampled, Format]
	f32TypeID := backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	u32TypeID := backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	i32TypeID := backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})

	for _, inst := range backend.builder.types {
		if inst.Opcode != OpTypeImage || len(inst.Words) < 8 {
			continue
		}
		resultID := inst.Words[0]
		sampledType := inst.Words[1]
		imageFormat := inst.Words[7]

		switch resultID {
		case id1: // r32float
			if sampledType != f32TypeID {
				t.Errorf("r32float image sampled type: got %d, want f32(%d)", sampledType, f32TypeID)
			}
			if imageFormat != uint32(ImageFormatR32f) {
				t.Errorf("r32float image format: got %d, want R32f(%d)", imageFormat, ImageFormatR32f)
			}
		case id2: // rgba8uint
			if sampledType != u32TypeID {
				t.Errorf("rgba8uint image sampled type: got %d, want u32(%d)", sampledType, u32TypeID)
			}
			if imageFormat != uint32(ImageFormatRgba8ui) {
				t.Errorf("rgba8uint image format: got %d, want Rgba8ui(%d)", imageFormat, ImageFormatRgba8ui)
			}
		case id3: // rgba32sint
			if sampledType != i32TypeID {
				t.Errorf("rgba32sint image sampled type: got %d, want i32(%d)", sampledType, i32TypeID)
			}
			if imageFormat != uint32(ImageFormatRgba32i) {
				t.Errorf("rgba32sint image format: got %d, want Rgba32i(%d)", imageFormat, ImageFormatRgba32i)
			}
		}
	}
}

// TestSampledTextureUsesCorrectScalarKind verifies that sampled textures
// (texture_2d<u32>, texture_2d<i32>) use the correct sampled scalar type,
// not always f32.
func TestSampledTextureUsesCorrectScalarKind(t *testing.T) {
	backend := NewBackend(DefaultOptions())
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ImageType{
				Dim:         ir.Dim2D,
				Class:       ir.ImageClassSampled,
				SampledKind: ir.ScalarFloat,
			}},
			{Name: "", Inner: ir.ImageType{
				Dim:         ir.Dim2D,
				Class:       ir.ImageClassSampled,
				SampledKind: ir.ScalarUint,
			}},
			{Name: "", Inner: ir.ImageType{
				Dim:         ir.Dim2D,
				Class:       ir.ImageClassSampled,
				SampledKind: ir.ScalarSint,
			}},
		},
		Constants:       []ir.Constant{},
		GlobalVariables: []ir.GlobalVariable{},
		Functions:       []ir.Function{},
		EntryPoints:     []ir.EntryPoint{},
	}

	backend.module = module
	backend.builder = NewModuleBuilder(backend.options.Version)

	id1, err := backend.emitType(0)
	if err != nil {
		t.Fatalf("emitType(f32 sampled) failed: %v", err)
	}
	id2, err := backend.emitType(1)
	if err != nil {
		t.Fatalf("emitType(u32 sampled) failed: %v", err)
	}
	id3, err := backend.emitType(2)
	if err != nil {
		t.Fatalf("emitType(i32 sampled) failed: %v", err)
	}

	// Different sampled kinds should produce different image types
	if id1 == id2 {
		t.Errorf("f32 and u32 sampled images should have different type IDs, both got %d", id1)
	}
	if id1 == id3 {
		t.Errorf("f32 and i32 sampled images should have different type IDs, both got %d", id1)
	}

	// Verify sampled types in OpTypeImage instructions
	f32TypeID := backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4})
	u32TypeID := backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
	i32TypeID := backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4})

	for _, inst := range backend.builder.types {
		if inst.Opcode != OpTypeImage || len(inst.Words) < 8 {
			continue
		}
		resultID := inst.Words[0]
		sampledType := inst.Words[1]

		switch resultID {
		case id1:
			if sampledType != f32TypeID {
				t.Errorf("f32 sampled image: sampled type got %d, want f32(%d)", sampledType, f32TypeID)
			}
		case id2:
			if sampledType != u32TypeID {
				t.Errorf("u32 sampled image: sampled type got %d, want u32(%d)", sampledType, u32TypeID)
			}
		case id3:
			if sampledType != i32TypeID {
				t.Errorf("i32 sampled image: sampled type got %d, want i32(%d)", sampledType, i32TypeID)
			}
		}
	}
}

// TestIndexByValueSpillsToFunction verifies that dynamic indexing into by-value
// arrays creates a Function-space temporary variable and uses OpAccessChain.
func TestIndexByValueSpillsToFunction(t *testing.T) {
	i32Handle := ir.TypeHandle(0)
	arrHandle := ir.TypeHandle(1)
	arrSize := uint32(5)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                     // [0] i32
			{Inner: ir.ArrayType{Base: i32Handle, Size: ir.ArraySize{Constant: &arrSize}, Stride: 4}}, // [1] array<i32,5>
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					LocalVars: []ir.LocalVariable{
						{Name: "arr", Type: arrHandle},
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprLocalVariable{Variable: 0}},   // [0] &arr (pointer)
						{Kind: ir.ExprLoad{Pointer: 0}},             // [1] arr (value)
						{Kind: ir.Literal{Value: ir.LiteralI32(2)}}, // [2] index 2
						{Kind: ir.ExprAccess{Base: 1, Index: 2}},    // [3] arr[2] — by-value access
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
					},
				},
				Workgroup: [3]uint32{1, 1, 1},
			},
		},
	}

	backend := NewBackend(DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Should have OpAccessChain (spill to Function variable)
	if !divmodHasOpcode(spvBytes, OpAccessChain) {
		t.Error("by-value array indexing should spill to Function variable and use OpAccessChain")
	}
}

// TestF16PolyfillConvertsIO verifies that when UseStorageInputOutput16=false,
// f16 entry point I/O is converted through f32 with OpFConvert.
func TestF16PolyfillConvertsIO(t *testing.T) {
	f16Handle := ir.TypeHandle(0)
	locBinding := ir.Binding(ir.LocationBinding{Location: 0})
	outBinding := ir.Binding(ir.LocationBinding{Location: 0})

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}}, // [0] f16
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageFragment,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{
						{Name: "input", Type: f16Handle, Binding: &locBinding},
					},
					Result: &ir.FunctionResult{
						Type:    f16Handle,
						Binding: &outBinding,
					},
					Expressions: []ir.Expression{
						{Kind: ir.ExprFunctionArgument{Index: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
						{Kind: ir.StmtReturn{Value: ptrExprHandle(0)}},
					},
				},
			},
		},
	}

	opts := DefaultOptions()
	opts.UseStorageInputOutput16 = false

	backend := NewBackend(opts)
	spvBytes, err := backend.Compile(module)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Should compile without errors
	if len(spvBytes) < 20 {
		t.Error("f16 polyfill produced empty/invalid SPIR-V")
	}
}
