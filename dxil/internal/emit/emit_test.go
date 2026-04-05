package emit

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// buildPassthroughVertex creates a minimal vertex shader IR module:
//
//	@vertex fn main(@builtin(position) pos: vec4<f32>) -> @builtin(position) vec4<f32> {
//	    return pos;
//	}
func buildPassthroughVertex() *ir.Module {
	// Types: vec4<f32>
	vec4f32Handle := ir.TypeHandle(0)
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	// Entry point with one argument and a result.
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	resultBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	retHandle := ir.ExpressionHandle(0)
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: vec4f32Handle, Binding: &posBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec4f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: fn},
	}

	return mod
}

// buildSimpleTransformVertex creates a vertex shader that constructs vec4 from vec2:
//
//	@vertex fn main(@location(0) pos: vec2<f32>) -> @builtin(position) vec4<f32> {
//	    return vec4(pos, 0.0, 1.0);
//	}
func buildSimpleTransformVertex() *ir.Module {
	vec2f32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)
	f32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	posBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	retHandle := ir.ExpressionHandle(4) // compose result

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "pos", Type: vec2f32Handle, Binding: &posBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] pos
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                         // [1] pos.x
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},                         // [2] pos.y
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 3, 4}}}, // [5] vec4(pos.x, pos.y, 0.0, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec2f32Handle}, // pos
			{Handle: &f32Handle},     // pos.x
			{Handle: &f32Handle},     // pos.y
			{Handle: &f32Handle},     // 0.0
			{Handle: &f32Handle},     // 1.0
			{Handle: &vec4f32Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	// Fix: retHandle should point to compose (index 5)
	retHandle = 5
	fn.Body[1] = ir.Statement{Kind: ir.StmtReturn{Value: &retHandle}}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: fn},
	}

	return mod
}

// buildSimpleFragmentShader creates a minimal fragment shader:
//
//	@fragment fn main() -> @location(0) vec4<f32> {
//	    return vec4(1.0, 0.0, 0.0, 1.0);
//	}
func buildSimpleFragmentShader() *ir.Module {
	vec4f32Handle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4) // compose result

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [0] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [1] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1, 2, 3}}}, // [4] vec4(...)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &vec4f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	return mod
}

// buildBinaryArithmeticVertex creates a vertex shader with arithmetic:
//
//	@vertex fn main(@location(0) a: f32, @location(1) b: f32) -> @builtin(position) vec4<f32> {
//	    let sum = a + b;
//	    return vec4(sum, sum, 0.0, 1.0);
//	}
func buildBinaryArithmeticVertex() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	bBinding := ir.Binding(ir.LocationBinding{Location: 1})
	resultBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	retHandle := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32Handle, Binding: &aBinding},
			{Name: "b", Type: f32Handle, Binding: &bBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},            // [2] a + b
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4] 1.0
			{Kind: ir.ExprSplat{Value: 2, Size: ir.VectorSize(4)}},                // not used
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 2, 3, 4}}}, // [6] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &vec4f32Handle},
			{Handle: &vec4f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: fn},
	}

	return mod
}

func TestEmitPassthroughVertex(t *testing.T) {
	irMod := buildPassthroughVertex()

	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify module structure.
	if mod.ShaderKind != module.VertexShader {
		t.Errorf("shader kind: got %d, want VertexShader (%d)", mod.ShaderKind, module.VertexShader)
	}

	// Must have at least one function definition (main).
	if len(mod.Functions) == 0 {
		t.Fatal("no functions emitted")
	}

	// Main function should have a non-empty body.
	var mainFn *module.Function
	for i := range mod.Functions {
		if mod.Functions[i].Name == "main" {
			mainFn = mod.Functions[i]
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}
	if len(mainFn.BasicBlocks) == 0 {
		t.Fatal("main function has no basic blocks")
	}
	if len(mainFn.BasicBlocks[0].Instructions) == 0 {
		t.Fatal("entry block has no instructions")
	}

	// Must have named metadata.
	metadataNames := make(map[string]bool)
	for _, nm := range mod.NamedMetadata {
		metadataNames[nm.Name] = true
	}
	for _, required := range []string{"dx.version", "dx.shaderModel", "dx.entryPoints"} {
		if !metadataNames[required] {
			t.Errorf("missing required metadata: %s", required)
		}
	}

	// Serialize to verify it produces valid bitcode.
	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	if bc[0] != 'B' || bc[1] != 'C' || bc[2] != 0xC0 || bc[3] != 0xDE {
		t.Errorf("invalid bitcode magic: %02X %02X %02X %02X", bc[0], bc[1], bc[2], bc[3])
	}
	if len(bc)%4 != 0 {
		t.Errorf("bitcode not 32-bit aligned: %d bytes", len(bc))
	}

	t.Logf("passthrough vertex: %d types, %d functions, %d constants, %d metadata nodes, %d bytes bitcode",
		len(mod.Types), len(mod.Functions), len(mod.Constants), len(mod.Metadata), len(bc))
}

func TestEmitSimpleTransformVertex(t *testing.T) {
	irMod := buildSimpleTransformVertex()

	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	if mod.ShaderKind != module.VertexShader {
		t.Errorf("shader kind: got %d, want VertexShader", mod.ShaderKind)
	}

	// Must have float constants for 0.0 and 1.0.
	hasFloatConst := false
	for _, c := range mod.Constants {
		if c.ConstType.Kind == module.TypeFloat {
			hasFloatConst = true
			break
		}
	}
	if !hasFloatConst {
		t.Error("no float constants emitted (expected 0.0, 1.0)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("simple transform vertex: %d types, %d functions, %d constants, %d bytes bitcode",
		len(mod.Types), len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitSimpleFragment(t *testing.T) {
	irMod := buildSimpleFragmentShader()

	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	if mod.ShaderKind != module.PixelShader {
		t.Errorf("shader kind: got %d, want PixelShader (%d)", mod.ShaderKind, module.PixelShader)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("simple fragment: %d types, %d functions, %d constants, %d bytes bitcode",
		len(mod.Types), len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitBinaryArithmeticVertex(t *testing.T) {
	irMod := buildBinaryArithmeticVertex()

	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify that binary operations generated instructions.
	var mainFn *module.Function
	for i := range mod.Functions {
		if mod.Functions[i].Name == "main" {
			mainFn = mod.Functions[i]
			break
		}
	}
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	hasBinOp := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrBinOp {
				hasBinOp = true
			}
		}
	}
	if !hasBinOp {
		t.Error("no binary operation instructions found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("binary arithmetic vertex: %d types, %d functions, %d constants, %d bytes bitcode",
		len(mod.Types), len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitNoEntryPoints(t *testing.T) {
	irMod := &ir.Module{}
	_, err := Emit(irMod, EmitOptions{})
	if err == nil {
		t.Fatal("expected error for module with no entry points")
	}
}

func TestStageMapping(t *testing.T) {
	tests := []struct {
		stage ir.ShaderStage
		want  module.ShaderKind
	}{
		{ir.StageVertex, module.VertexShader},
		{ir.StageFragment, module.PixelShader},
		{ir.StageCompute, module.ComputeShader},
	}
	for _, tt := range tests {
		got := stageToShaderKind(tt.stage)
		if got != tt.want {
			t.Errorf("stageToShaderKind(%d) = %d, want %d", tt.stage, got, tt.want)
		}
	}
}

func TestShaderKindString(t *testing.T) {
	tests := []struct {
		kind module.ShaderKind
		want string
	}{
		{module.VertexShader, "vs"},
		{module.PixelShader, "ps"},
		{module.ComputeShader, "cs"},
		{module.GeometryShader, "gs"},
		{module.HullShader, "hs"},
		{module.DomainShader, "ds"},
	}
	for _, tt := range tests {
		got := shaderKindString(tt.kind)
		if got != tt.want {
			t.Errorf("shaderKindString(%d) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestOverloadForScalar(t *testing.T) {
	tests := []struct {
		scalar ir.ScalarType
		want   overloadType
	}{
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, overloadF32},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, overloadF64},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, overloadF16},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, overloadI32},
		{ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, overloadI32},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, overloadI64},
		{ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, overloadI1},
	}
	for _, tt := range tests {
		got := overloadForScalar(tt.scalar)
		if got != tt.want {
			t.Errorf("overloadForScalar(%v) = %d, want %d", tt.scalar, got, tt.want)
		}
	}
}

func TestComponentCount(t *testing.T) {
	tests := []struct {
		inner ir.TypeInner
		want  int
	}{
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, 1},
		{ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 2},
		{ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 3},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 4},
		{ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, 16},
	}
	for _, tt := range tests {
		got := componentCount(tt.inner)
		if got != tt.want {
			t.Errorf("componentCount(%T) = %d, want %d", tt.inner, got, tt.want)
		}
	}
}

func TestScalarOfType(t *testing.T) {
	tests := []struct {
		inner    ir.TypeInner
		wantOK   bool
		wantKind ir.ScalarKind
	}{
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, true, ir.ScalarFloat},
		{ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}, true, ir.ScalarSint},
		{ir.MatrixType{Columns: 2, Rows: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, true, ir.ScalarFloat},
	}
	for _, tt := range tests {
		s, ok := scalarOfType(tt.inner)
		if ok != tt.wantOK {
			t.Errorf("scalarOfType(%T): ok=%v, want %v", tt.inner, ok, tt.wantOK)
			continue
		}
		if ok && s.Kind != tt.wantKind {
			t.Errorf("scalarOfType(%T): kind=%v, want %v", tt.inner, s.Kind, tt.wantKind)
		}
	}
}

func TestTypeMappingScalar(t *testing.T) {
	mod := module.NewModule(module.VertexShader)
	irMod := &ir.Module{}

	tests := []struct {
		inner    ir.TypeInner
		wantKind module.TypeKind
	}{
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, module.TypeFloat},
		{ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, module.TypeInteger},
		{ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, module.TypeInteger},
	}
	for _, tt := range tests {
		ty, err := typeToDXIL(mod, irMod, tt.inner)
		if err != nil {
			t.Errorf("typeToDXIL(%T): %v", tt.inner, err)
			continue
		}
		if ty.Kind != tt.wantKind {
			t.Errorf("typeToDXIL(%T): kind=%v, want %v", tt.inner, ty.Kind, tt.wantKind)
		}
	}
}

func TestTypeMappingVectorReturnsScalar(t *testing.T) {
	mod := module.NewModule(module.VertexShader)
	irMod := &ir.Module{}

	// DXIL has no vectors — typeToDXIL should return the scalar element type.
	vec := ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	ty, err := typeToDXIL(mod, irMod, vec)
	if err != nil {
		t.Fatalf("typeToDXIL(vec4<f32>): %v", err)
	}
	if ty.Kind != module.TypeFloat {
		t.Errorf("typeToDXIL(vec4<f32>): kind=%v, want TypeFloat", ty.Kind)
	}
}

func TestIsFloatType(t *testing.T) {
	if !isFloatType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}) {
		t.Error("isFloatType(f32) should be true")
	}
	if isFloatType(ir.ScalarType{Kind: ir.ScalarSint, Width: 4}) {
		t.Error("isFloatType(i32) should be false")
	}
	if !isFloatType(ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}) {
		t.Error("isFloatType(vec4<f32>) should be true")
	}
}

func TestBuiltinSemanticIndex(t *testing.T) {
	tests := []struct {
		builtin ir.BuiltinValue
		want    int
	}{
		{ir.BuiltinPosition, 0},
		{ir.BuiltinVertexIndex, 1},
		{ir.BuiltinInstanceIndex, 2},
		{ir.BuiltinFrontFacing, 0},
		{ir.BuiltinFragDepth, 0},
	}
	for _, tt := range tests {
		got := builtinSemanticIndex(tt.builtin)
		if got != tt.want {
			t.Errorf("builtinSemanticIndex(%d) = %d, want %d", tt.builtin, got, tt.want)
		}
	}
}
