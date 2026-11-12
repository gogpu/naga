package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// typeModule is the shared preamble of the module-level tests: capabilities,
// memory model, one of each type and the constants array lengths need.
func typeModule() [][]uint32 {
	return [][]uint32{
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 7, uint32(spvenums.DecorationBlock)),
		instWords(spvenums.OpMemberDecorate, 7, 0, uint32(spvenums.DecorationOffset), 0),
		instWords(spvenums.OpMemberDecorate, 7, 1, uint32(spvenums.DecorationOffset), 16),
		instWords(spvenums.OpTypeVoid, 1),
		instWords(spvenums.OpTypeInt, 2, 32, 1), // i32
		instWords(spvenums.OpTypeInt, 3, 32, 0), // u32
		instWords(spvenums.OpTypeFloat, 4, 32),
		instWords(spvenums.OpTypeVector, 5, 4, 3),
		instWords(spvenums.OpTypeMatrix, 6, 5, 2),
		instWords(spvenums.OpTypeStruct, 7, 5, 2),
		instWords(spvenums.OpTypePointer, 8, uint32(spvenums.StorageClassStorageBuffer), 7),
		instWords(spvenums.OpConstant, 3, 9, 4),
		instWords(spvenums.OpTypeArray, 10, 2, 9),
	}
}

func TestParseTypes(t *testing.T) {
	module, _, err := parseWords(t, Options{}, typeModule()...)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := len(module.Types), 8; got != want {
		t.Fatalf("types = %d, want %d", got, want)
	}

	i32, ok := module.Types[0].Inner.(ir.ScalarType)
	if !ok || i32.Kind != ir.ScalarSint || i32.Width != 4 {
		t.Errorf("type 0 = %#v, want i32", module.Types[0].Inner)
	}
	u32, ok := module.Types[1].Inner.(ir.ScalarType)
	if !ok || u32.Kind != ir.ScalarUint || u32.Width != 4 {
		t.Errorf("type 1 = %#v, want u32", module.Types[1].Inner)
	}
	vec3, ok := module.Types[3].Inner.(ir.VectorType)
	if !ok || vec3.Size != ir.Vec3 || vec3.Scalar.Kind != ir.ScalarFloat {
		t.Errorf("type 3 = %#v, want vec3<f32>", module.Types[3].Inner)
	}
	matrix, ok := module.Types[4].Inner.(ir.MatrixType)
	if !ok || matrix.Columns != ir.Vec2 || matrix.Rows != ir.Vec3 {
		t.Errorf("type 4 = %#v, want mat2x3<f32>", module.Types[4].Inner)
	}
	structType, ok := module.Types[5].Inner.(ir.StructType)
	if !ok {
		t.Fatalf("type 5 = %T, want a struct", module.Types[5].Inner)
	}
	if len(structType.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(structType.Members))
	}
	if structType.Members[0].Offset != 0 || structType.Members[1].Offset != 16 {
		t.Errorf("offsets = %d, %d; want 0, 16", structType.Members[0].Offset, structType.Members[1].Offset)
	}
	if structType.Span != 32 {
		t.Errorf("span = %d, want 32", structType.Span)
	}
	pointer, ok := module.Types[6].Inner.(ir.PointerType)
	if !ok || pointer.Space != ir.SpaceStorage || pointer.Base != 5 {
		t.Errorf("type 6 = %#v, want a pointer to the struct in storage", module.Types[6].Inner)
	}
	array, ok := module.Types[7].Inner.(ir.ArrayType)
	if !ok {
		t.Fatalf("type 7 = %T, want an array", module.Types[7].Inner)
	}
	if array.Size.Constant == nil || *array.Size.Constant != 4 || array.Stride != 4 {
		t.Errorf("array = %#v, want four i32 with stride 4", array)
	}
}

func TestParseConstants(t *testing.T) {
	module, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 15, uint32(spvenums.DecorationSpecId), 7),
		instWords(spvenums.OpTypeBool, 1),
		instWords(spvenums.OpTypeInt, 2, 32, 0), // u32
		instWords(spvenums.OpTypeInt, 3, 32, 1), // i32
		instWords(spvenums.OpTypeFloat, 4, 32),  // f32
		instWords(spvenums.OpTypeFloat, 5, 64),  // f64
		instWords(spvenums.OpTypeFloat, 6, 16),  // f16
		instWords(spvenums.OpConstant, 2, 7, 42),
		instWords(spvenums.OpConstant, 3, 8, 0xffffffff),
		instWords(spvenums.OpConstant, 4, 9, 0x3fc00000),     // 1.5
		instWords(spvenums.OpConstant, 5, 10, 0, 0x40040000), // 2.5
		instWords(spvenums.OpConstant, 6, 11, 0x3c00),        // 1.0
		instWords(spvenums.OpConstantTrue, 1, 12),
		instWords(spvenums.OpConstantFalse, 1, 13),
		instWords(spvenums.OpConstantNull, 2, 14),
		instWords(spvenums.OpSpecConstant, 2, 15, 99),
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	wantLiterals := map[ir.ConstantHandle]ir.LiteralValue{
		0: ir.LiteralU32(42),
		1: ir.LiteralI32(-1),
		2: ir.LiteralF32(1.5),
		3: ir.LiteralF64(2.5),
		4: ir.LiteralF16(1),
		5: ir.LiteralBool(true),
		6: ir.LiteralBool(false),
	}
	for handle, want := range wantLiterals {
		init := module.Constants[handle].Init
		literal, ok := module.GlobalExpressions[init].Kind.(ir.Literal)
		if !ok || literal.Value != want {
			t.Errorf("constant %d = %#v, want %v", handle, module.GlobalExpressions[init].Kind, want)
		}
	}
	zero, ok := module.GlobalExpressions[module.Constants[7].Init].Kind.(ir.ExprZeroValue)
	if !ok || zero.Type != 1 {
		t.Errorf("constant 7 = %#v, want a zero value of u32", module.GlobalExpressions[module.Constants[7].Init].Kind)
	}
	if len(module.Overrides) != 1 {
		t.Fatalf("overrides = %d, want 1", len(module.Overrides))
	}
	override := module.Overrides[0]
	if override.ID == nil || *override.ID != 7 || override.Ty != 1 {
		t.Errorf("override = %#v, want id 7 of u32", override)
	}
}

func TestParseCompositeConstant(t *testing.T) {
	module, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpTypeFloat, 1, 32),
		instWords(spvenums.OpTypeVector, 2, 1, 2),
		instWords(spvenums.OpConstant, 1, 3, 0x3f800000), // 1.0
		instWords(spvenums.OpConstantComposite, 2, 4, 3, 3),
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	init := module.Constants[1].Init
	compose, ok := module.GlobalExpressions[init].Kind.(ir.ExprCompose)
	if !ok || compose.Type != 1 || len(compose.Components) != 2 {
		t.Fatalf("composite = %#v, want a vec2 of two components", module.GlobalExpressions[init].Kind)
	}
	for _, component := range compose.Components {
		if _, ok := module.GlobalExpressions[component].Kind.(ir.ExprConstant); !ok {
			t.Errorf("component %d = %#v, want a constant reference", component, module.GlobalExpressions[component].Kind)
		}
	}
}

func TestParseConstantErrors(t *testing.T) {
	t.Run("spec id too high", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpDecorate, 3, uint32(spvenums.DecorationSpecId), 70000),
			instWords(spvenums.OpTypeInt, 1, 32, 0),
			instWords(spvenums.OpSpecConstant, 1, 3, 0),
		)
		want := "naga only supports specialization constant IDs up to 65535 but was given 70000"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})

	t.Run("zero array length", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeInt, 1, 32, 0),
			instWords(spvenums.OpConstant, 1, 2, 0),
			instWords(spvenums.OpTypeArray, 3, 1, 2),
		)
		if got := requireError(t, err).Message; got != "invalid array size %2" {
			t.Errorf("message = %q, want %q", got, "invalid array size %2")
		}
	})

	t.Run("array too large", func(t *testing.T) {
		_, _, err := parseWords(t, Options{},
			headerWords(),
			instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
			instWords(spvenums.OpMemoryModel, 0, 1),
			instWords(spvenums.OpTypeInt, 1, 32, 0),
			instWords(spvenums.OpConstant, 1, 2, maxArrayElements+1),
			instWords(spvenums.OpTypeArray, 3, 1, 2),
		)
		want := "array size 16777217 exceeds the limit 16777216"
		if got := requireError(t, err).Message; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestParseGlobalVariableKinds(t *testing.T) {
	module, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 10, uint32(spvenums.DecorationDescriptorSet), 0),
		instWords(spvenums.OpDecorate, 10, uint32(spvenums.DecorationBinding), 1),
		instWords(spvenums.OpDecorate, 11, uint32(spvenums.DecorationNonWritable)),
		instWords(spvenums.OpDecorate, 10, uint32(spvenums.DecorationBlock)),
		instWords(spvenums.OpTypeFloat, 1, 32),
		instWords(spvenums.OpTypeInt, 2, 32, 0),
		instWords(spvenums.OpTypeStruct, 3, 2),
		instWords(spvenums.OpTypePointer, 4, uint32(spvenums.StorageClassStorageBuffer), 3),
		instWords(spvenums.OpTypePointer, 5, uint32(spvenums.StorageClassPrivate), 2),
		instWords(spvenums.OpConstant, 2, 6, 5),
		instWords(spvenums.OpVariable, 4, 10, uint32(spvenums.StorageClassStorageBuffer)),
		instWords(spvenums.OpVariable, 4, 11, uint32(spvenums.StorageClassStorageBuffer)),
		instWords(spvenums.OpVariable, 5, 12, uint32(spvenums.StorageClassPrivate), 6),
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.GlobalVariables) != 3 {
		t.Fatalf("globals = %d, want 3", len(module.GlobalVariables))
	}
	storage := module.GlobalVariables[0]
	if storage.Space != ir.SpaceStorage || storage.Binding == nil ||
		*storage.Binding != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Errorf("storage global = %#v, want storage at (0,1)", storage)
	}
	if storage.Access != ir.StorageAccessLoad|ir.StorageAccessStore {
		t.Errorf("storage access = %d, want read and write", storage.Access)
	}
	readOnly := module.GlobalVariables[1]
	if readOnly.Access != ir.StorageAccessLoad {
		t.Errorf("read-only access = %d, want load", readOnly.Access)
	}
	private := module.GlobalVariables[2]
	if private.Space != ir.SpacePrivate || private.InitExpr == nil {
		t.Errorf("private global = %#v, want a private global with an initializer", private)
	}
}

func TestParseInputOutputVariables(t *testing.T) {
	module, _, err := parseWords(t, Options{},
		headerWords(),
		instWords(spvenums.OpCapability, uint32(spvenums.CapabilityShader)),
		instWords(spvenums.OpMemoryModel, 0, 1),
		instWords(spvenums.OpDecorate, 4, uint32(spvenums.DecorationLocation), 0),
		instWords(spvenums.OpDecorate, 5, uint32(spvenums.DecorationBuiltIn), uint32(spvenums.BuiltInPosition)),
		instWords(spvenums.OpTypeFloat, 1, 32),
		instWords(spvenums.OpTypeVector, 2, 1, 2),
		instWords(spvenums.OpTypeVector, 3, 1, 4),
		instWords(spvenums.OpTypePointer, 6, uint32(spvenums.StorageClassInput), 2),
		instWords(spvenums.OpTypePointer, 7, uint32(spvenums.StorageClassOutput), 3),
		instWords(spvenums.OpVariable, 6, 4, uint32(spvenums.StorageClassInput)),
		instWords(spvenums.OpVariable, 7, 5, uint32(spvenums.StorageClassOutput)),
	)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(module.GlobalVariables) != 2 {
		t.Fatalf("globals = %d, want 2", len(module.GlobalVariables))
	}
	for i, global := range module.GlobalVariables {
		if global.Space != ir.SpacePrivate || global.Binding != nil {
			t.Errorf("global %d = %#v, want a private global without a resource binding", i, global)
		}
	}
	// The position output gets its default value: (0, 0, 0, 1).
	output := module.GlobalVariables[1]
	if output.InitExpr == nil {
		t.Fatal("output has no default initializer")
	}
	compose, ok := module.GlobalExpressions[*output.InitExpr].Kind.(ir.ExprCompose)
	if !ok || len(compose.Components) != 4 {
		t.Fatalf("output init = %#v, want a vec4 compose", module.GlobalExpressions[*output.InitExpr].Kind)
	}
}
