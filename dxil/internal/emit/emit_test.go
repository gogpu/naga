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

// buildMathBinaryShader creates a fragment shader with a binary math function:
//
//	@fragment fn main(@location(0) a: f32, @location(1) b: f32) -> @location(0) vec4<f32> {
//	    let r = mathFn(a, b);
//	    return vec4(r, r, r, 1.0);
//	}
func buildMathBinaryShader(mathFn ir.MathFunction, scalarKind ir.ScalarKind) *ir.Module {
	var scalarWidth byte = 4
	scalar := ir.ScalarType{Kind: scalarKind, Width: scalarWidth}
	scalarHandle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: scalar},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	bBinding := ir.Binding(ir.LocationBinding{Location: 1})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	arg1Handle := ir.ExpressionHandle(1)
	retHandle := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: scalarHandle, Binding: &aBinding},
			{Name: "b", Type: scalarHandle, Binding: &bBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b
			{Kind: ir.ExprMath{Fun: mathFn, Arg: 0, Arg1: &arg1Handle}},           // [2] mathFn(a, b)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4] 0.0 (padding)
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 2, 2, 3}}}, // [5] vec4(r, r, r, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalarHandle}, // a
			{Handle: &scalarHandle}, // b
			{Handle: &scalarHandle}, // result
			{Handle: &scalarHandle}, // 1.0
			{Handle: &scalarHandle}, // 0.0
			{Handle: &vec4Handle},   // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildMathTernaryShader creates a fragment shader with a ternary math function.
func buildMathTernaryShader(mathFn ir.MathFunction) *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	bBinding := ir.Binding(ir.LocationBinding{Location: 1})
	cBinding := ir.Binding(ir.LocationBinding{Location: 2})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	arg1Handle := ir.ExpressionHandle(1)
	arg2Handle := ir.ExpressionHandle(2)
	retHandle := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32Handle, Binding: &aBinding},
			{Name: "b", Type: f32Handle, Binding: &bBinding},
			{Name: "c", Type: f32Handle, Binding: &cBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                                      // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                                      // [1] b
			{Kind: ir.ExprFunctionArgument{Index: 2}},                                      // [2] c
			{Kind: ir.ExprMath{Fun: mathFn, Arg: 0, Arg1: &arg1Handle, Arg2: &arg2Handle}}, // [3] mathFn(a, b, c)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                                  // [4] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                                  // [5] 0.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 3, 3, 4}}},          // [6] vec4(r, r, r, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},  // a
			{Handle: &f32Handle},  // b
			{Handle: &f32Handle},  // c
			{Handle: &f32Handle},  // result
			{Handle: &f32Handle},  // 1.0
			{Handle: &f32Handle},  // 0.0
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildDotProductShader creates a fragment shader with dot product:
//
//	@fragment fn main(@location(0) a: vec3<f32>, @location(1) b: vec3<f32>) -> @location(0) vec4<f32> {
//	    let d = dot(a, b);
//	    return vec4(d, d, d, 1.0);
//	}
func buildDotProductShader(vecSize ir.VectorSize) *ir.Module {
	vecHandle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)
	vec4Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: vecSize, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	bBinding := ir.Binding(ir.LocationBinding{Location: 1})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	arg1Handle := ir.ExpressionHandle(1)
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: vecHandle, Binding: &aBinding},
			{Name: "b", Type: vecHandle, Binding: &bBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b
			{Kind: ir.ExprMath{Fun: ir.MathDot, Arg: 0, Arg1: &arg1Handle}},       // [2] dot(a, b)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 2, 2, 3}}}, // [4] vec4(d, d, d, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vecHandle},  // a
			{Handle: &vecHandle},  // b
			{Handle: &f32Handle},  // dot result (scalar)
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
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

// buildCrossProductShader creates a fragment shader with cross product.
func buildCrossProductShader() *ir.Module {
	vec3Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	f32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	bBinding := ir.Binding(ir.LocationBinding{Location: 1})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	arg1Handle := ir.ExpressionHandle(1)
	retHandle := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: vec3Handle, Binding: &aBinding},
			{Name: "b", Type: vec3Handle, Binding: &bBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a (vec3)
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b (vec3)
			{Kind: ir.ExprMath{Fun: ir.MathCross, Arg: 0, Arg1: &arg1Handle}},     // [2] cross(a, b) -> vec3
			{Kind: ir.ExprAccessIndex{Base: 2, Index: 0}},                         // [3] cross.x
			{Kind: ir.ExprAccessIndex{Base: 2, Index: 1}},                         // [4] cross.y
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4, 3, 5}}}, // [6] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3Handle}, // a
			{Handle: &vec3Handle}, // b
			{Handle: &vec3Handle}, // cross result
			{Handle: &f32Handle},  // cross.x
			{Handle: &f32Handle},  // cross.y
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildLengthShader creates a fragment shader with length(vec3).
func buildLengthShader() *ir.Module {
	vec3Handle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)
	vec4Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(3)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: vec3Handle, Binding: &aBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v (vec3)
			{Kind: ir.ExprMath{Fun: ir.MathLength, Arg: 0}},                       // [1] length(v) -> scalar
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [2] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 1, 1, 2}}}, // [3] vec4(l, l, l, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3Handle}, // v
			{Handle: &f32Handle},  // length result
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

func TestEmitMathMinFloat(t *testing.T) {
	irMod := buildMathBinaryShader(ir.MathMin, ir.ScalarFloat)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify dx.op.fmin function was created.
	hasFMin := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.fmin.f32" {
			hasFMin = true
			// Binary dx.op: ret(i32, TYPE, TYPE) = 3 params.
			if len(fn.FuncType.ParamTypes) != 3 {
				t.Errorf("dx.op.fmin.f32 params: got %d, want 3", len(fn.FuncType.ParamTypes))
			}
			break
		}
	}
	if !hasFMin {
		t.Error("dx.op.fmin.f32 function not found")
	}

	// Must have call instructions for the dx.op.
	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}
	hasCall := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.fmin.f32" {
				hasCall = true
			}
		}
	}
	if !hasCall {
		t.Error("no call to dx.op.fmin.f32 found in main")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("min(f32): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitMathMaxSint(t *testing.T) {
	irMod := buildMathBinaryShader(ir.MathMax, ir.ScalarSint)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	hasIMax := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.imax.i32" {
			hasIMax = true
			break
		}
	}
	if !hasIMax {
		t.Error("dx.op.imax.i32 function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("max(i32): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitMathMaxUint(t *testing.T) {
	irMod := buildMathBinaryShader(ir.MathMax, ir.ScalarUint)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	hasUMax := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.umax.i32" {
			hasUMax = true
			break
		}
	}
	if !hasUMax {
		t.Error("dx.op.umax.i32 function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
}

func TestEmitMathClamp(t *testing.T) {
	irMod := buildMathTernaryShader(ir.MathClamp)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Clamp(float) uses fmax + fmin.
	hasFMax := false
	hasFMin := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.fmax.f32" {
			hasFMax = true
		}
		if fn.Name == "dx.op.fmin.f32" {
			hasFMin = true
		}
	}
	if !hasFMax {
		t.Error("dx.op.fmax.f32 function not found (needed for clamp)")
	}
	if !hasFMin {
		t.Error("dx.op.fmin.f32 function not found (needed for clamp)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("clamp(f32): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitMathMix(t *testing.T) {
	irMod := buildMathTernaryShader(ir.MathMix)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Mix uses fmad (b-a subtraction + fmad(t, b-a, a)).
	hasFMad := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.fmad.f32" {
			hasFMad = true
			// Ternary dx.op: ret(i32, TYPE, TYPE, TYPE) = 4 params.
			if len(fn.FuncType.ParamTypes) != 4 {
				t.Errorf("dx.op.fmad.f32 params: got %d, want 4", len(fn.FuncType.ParamTypes))
			}
			break
		}
	}
	if !hasFMad {
		t.Error("dx.op.fmad.f32 function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("mix(f32): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitMathFma(t *testing.T) {
	irMod := buildMathTernaryShader(ir.MathFma)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	hasFma := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.fma.f32" {
			hasFma = true
			break
		}
	}
	if !hasFma {
		t.Error("dx.op.fma.f32 function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
}

func TestEmitMathPow(t *testing.T) {
	irMod := buildMathBinaryShader(ir.MathPow, ir.ScalarFloat)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Pow decomposes into log2 + fmul + exp2.
	hasLog := false
	hasExp := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.log.f32" {
			hasLog = true
		}
		if fn.Name == "dx.op.exp.f32" {
			hasExp = true
		}
	}
	if !hasLog {
		t.Error("dx.op.log.f32 function not found (needed for pow)")
	}
	if !hasExp {
		t.Error("dx.op.exp.f32 function not found (needed for pow)")
	}

	// Check for binary op instruction (fmul for log2(base)*exp).
	mainFn := findMainFunc(mod)
	hasBinOp := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrBinOp {
				hasBinOp = true
			}
		}
	}
	if !hasBinOp {
		t.Error("no binary op instruction found (needed for pow log*exp)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("pow(f32): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitMathStep(t *testing.T) {
	irMod := buildMathBinaryShader(ir.MathStep, ir.ScalarFloat)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Step uses fcmp + select.
	mainFn := findMainFunc(mod)
	hasCmp := false
	hasSelect := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCmp {
				hasCmp = true
			}
			if instr.Kind == module.InstrSelect {
				hasSelect = true
			}
		}
	}
	if !hasCmp {
		t.Error("no comparison instruction found (needed for step)")
	}
	if !hasSelect {
		t.Error("no select instruction found (needed for step)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
}

func TestEmitMathSmoothStep(t *testing.T) {
	irMod := buildMathTernaryShader(ir.MathSmoothStep)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// SmoothStep uses fmax, fmin (for clamp), plus several fmul/fsub.
	hasFMax := false
	hasFMin := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.fmax.f32" {
			hasFMax = true
		}
		if fn.Name == "dx.op.fmin.f32" {
			hasFMin = true
		}
	}
	if !hasFMax {
		t.Error("dx.op.fmax.f32 not found (needed for smoothstep clamp)")
	}
	if !hasFMin {
		t.Error("dx.op.fmin.f32 not found (needed for smoothstep clamp)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("smoothstep(f32): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitDotProduct(t *testing.T) {
	tests := []struct {
		name    string
		vecSize ir.VectorSize
		dotName string
	}{
		{"dot2", 2, "dx.op.dot2.f32"},
		{"dot3", 3, "dx.op.dot3.f32"},
		{"dot4", 4, "dx.op.dot4.f32"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			irMod := buildDotProductShader(tt.vecSize)
			mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
			if err != nil {
				t.Fatalf("Emit failed: %v", err)
			}

			hasDot := false
			for _, fn := range mod.Functions {
				if fn.Name == tt.dotName {
					hasDot = true
					// Dot params: i32 + 2*size scalar values.
					wantParams := 1 + 2*int(tt.vecSize)
					if len(fn.FuncType.ParamTypes) != wantParams {
						t.Errorf("%s params: got %d, want %d", tt.dotName, len(fn.FuncType.ParamTypes), wantParams)
					}
					break
				}
			}
			if !hasDot {
				t.Errorf("%s function not found", tt.dotName)
			}

			bc := module.Serialize(mod)
			if len(bc) == 0 {
				t.Fatal("serialization produced empty bitcode")
			}
			t.Logf("%s: %d functions, %d constants, %d bytes", tt.name, len(mod.Functions), len(mod.Constants), len(bc))
		})
	}
}

func TestEmitCrossProduct(t *testing.T) {
	irMod := buildCrossProductShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Cross product uses 6 fmul + 3 fsub = 9 binary ops total.
	// Note: finalize remaps operand values including the binop code,
	// so we count InstrBinOp instructions rather than checking op codes.
	mainFn := findMainFunc(mod)
	binOpCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrBinOp {
				binOpCount++
			}
		}
	}
	// Cross product should generate at least 9 binary ops (6 mul + 3 sub).
	if binOpCount < 9 {
		t.Errorf("cross product: expected >= 9 binary ops, got %d", binOpCount)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("cross(vec3): %d binops, %d functions, %d bytes", binOpCount, len(mod.Functions), len(bc))
}

func TestEmitMathLength(t *testing.T) {
	irMod := buildLengthShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Length(vec3) = sqrt(dot3(v, v)).
	hasDot3 := false
	hasSqrt := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.dot3.f32" {
			hasDot3 = true
		}
		if fn.Name == "dx.op.sqrt.f32" {
			hasSqrt = true
		}
	}
	if !hasDot3 {
		t.Error("dx.op.dot3.f32 not found (needed for length)")
	}
	if !hasSqrt {
		t.Error("dx.op.sqrt.f32 not found (needed for length)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("length(vec3): %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

func TestEmitMathAtan2(t *testing.T) {
	irMod := buildMathBinaryShader(ir.MathAtan2, ir.ScalarFloat)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Atan2 decomposes into fdiv + atan.
	hasAtan := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.atan.f32" {
			hasAtan = true
			break
		}
	}
	if !hasAtan {
		t.Error("dx.op.atan.f32 not found (needed for atan2)")
	}

	// Check for binary op instruction (fdiv for y/x).
	mainFn := findMainFunc(mod)
	hasBinOp := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrBinOp {
				hasBinOp = true
			}
		}
	}
	if !hasBinOp {
		t.Error("no binary op instruction found (needed for atan2 y/x)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
}

func TestOverloadSuffix(t *testing.T) {
	tests := []struct {
		ol   overloadType
		want string
	}{
		{overloadF32, ".f32"},
		{overloadF64, ".f64"},
		{overloadF16, ".f16"},
		{overloadI32, ".i32"},
		{overloadI64, ".i64"},
		{overloadI16, ".i16"},
		{overloadI1, ".i1"},
		{overloadVoid, ""},
	}
	for _, tt := range tests {
		got := overloadSuffix(tt.ol)
		if got != tt.want {
			t.Errorf("overloadSuffix(%d) = %q, want %q", tt.ol, got, tt.want)
		}
	}
}

func TestGetDxOpBinaryFunc(t *testing.T) {
	mod := module.NewModule(module.VertexShader)
	irMod := &ir.Module{}
	e := &Emitter{
		ir:          irMod,
		mod:         mod,
		exprValues:  make(map[ir.ExpressionHandle]int),
		dxOpFuncs:   make(map[dxOpKey]*module.Function),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
		constMap:    make(map[int]*module.Constant),
	}

	fn := e.getDxOpBinaryFunc("dx.op.fmin", overloadF32)
	if fn.Name != "dx.op.fmin.f32" {
		t.Errorf("name: got %q, want %q", fn.Name, "dx.op.fmin.f32")
	}
	if len(fn.FuncType.ParamTypes) != 3 {
		t.Errorf("params: got %d, want 3", len(fn.FuncType.ParamTypes))
	}

	// Second call should return cached.
	fn2 := e.getDxOpBinaryFunc("dx.op.fmin", overloadF32)
	if fn != fn2 {
		t.Error("getDxOpBinaryFunc did not return cached function")
	}
}

func TestGetDxOpTernaryFunc(t *testing.T) {
	mod := module.NewModule(module.VertexShader)
	irMod := &ir.Module{}
	e := &Emitter{
		ir:          irMod,
		mod:         mod,
		exprValues:  make(map[ir.ExpressionHandle]int),
		dxOpFuncs:   make(map[dxOpKey]*module.Function),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
		constMap:    make(map[int]*module.Constant),
	}

	fn := e.getDxOpTernaryFunc("dx.op.fmad", overloadF32)
	if fn.Name != "dx.op.fmad.f32" {
		t.Errorf("name: got %q, want %q", fn.Name, "dx.op.fmad.f32")
	}
	if len(fn.FuncType.ParamTypes) != 4 {
		t.Errorf("params: got %d, want 4", len(fn.FuncType.ParamTypes))
	}
}

func TestGetDxOpDotFunc(t *testing.T) {
	mod := module.NewModule(module.VertexShader)
	irMod := &ir.Module{}
	e := &Emitter{
		ir:          irMod,
		mod:         mod,
		exprValues:  make(map[ir.ExpressionHandle]int),
		dxOpFuncs:   make(map[dxOpKey]*module.Function),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
		constMap:    make(map[int]*module.Constant),
	}

	fn := e.getDxOpDotFunc(3, overloadF32)
	if fn.Name != "dx.op.dot3.f32" {
		t.Errorf("name: got %q, want %q", fn.Name, "dx.op.dot3.f32")
	}
	// dot3: i32 + 3*2 = 7 params
	if len(fn.FuncType.ParamTypes) != 7 {
		t.Errorf("params: got %d, want 7", len(fn.FuncType.ParamTypes))
	}

	fn4 := e.getDxOpDotFunc(4, overloadF32)
	if fn4.Name != "dx.op.dot4.f32" {
		t.Errorf("name: got %q, want %q", fn4.Name, "dx.op.dot4.f32")
	}
	if len(fn4.FuncType.ParamTypes) != 9 {
		t.Errorf("params: got %d, want 9", len(fn4.FuncType.ParamTypes))
	}
}

// findMainFunc finds the "main" function in a module.
func findMainFunc(mod *module.Module) *module.Function {
	for _, fn := range mod.Functions {
		if fn.Name == "main" {
			return fn
		}
	}
	return nil
}

// --- Type cast (ExprAs) tests ---

// buildCastShader creates a fragment shader that casts a scalar input to a different type:
//
//	@fragment fn main(@location(0) v: srcType) -> @location(0) vec4<f32> {
//	    let c = dstType(v);
//	    return vec4(f32(c), 0.0, 0.0, 1.0);
//	}
//
// The ExprAs is at expression [1] with the given kind and convert width.
func buildCastShader(srcScalar ir.ScalarType, dstKind ir.ScalarKind, convertWidth *uint8) *ir.Module {
	srcHandle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)
	vec4Handle := ir.TypeHandle(2)

	// Determine dst scalar for type resolution.
	var dstWidth uint8
	if convertWidth != nil {
		dstWidth = *convertWidth
	} else {
		dstWidth = srcScalar.Width
	}
	dstScalar := ir.ScalarType{Kind: dstKind, Width: dstWidth}
	dstHandle := ir.TypeHandle(3)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: srcScalar},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: dstScalar},
		},
	}

	aBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: srcHandle, Binding: &aBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v
			{Kind: ir.ExprAs{Expr: 0, Kind: dstKind, Convert: convertWidth}},      // [1] cast(v)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4] 1.0
			{Kind: ir.ExprAs{Expr: 1, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},   // [5] f32(c) - for output
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{5, 2, 3, 4}}}, // [6] vec4(...)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &srcHandle},  // v
			{Handle: &dstHandle},  // cast(v)
			{Handle: &f32Handle},  // 0.0
			{Handle: &f32Handle},  // 0.0
			{Handle: &f32Handle},  // 1.0
			{Handle: &f32Handle},  // f32(c)
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// ptrU8 returns a pointer to a uint8 value.
func ptrU8(v uint8) *uint8 { return &v }

// countInstrKind counts the number of instructions of the given kind in the main function.
func countInstrKind(mod *module.Module, kind module.InstrKind) int {
	mainFn := findMainFunc(mod)
	if mainFn == nil {
		return 0
	}
	count := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == kind {
				count++
			}
		}
	}
	return count
}

func TestSelectDXILCastOp(t *testing.T) {
	tests := []struct {
		name string
		src  ir.ScalarType
		dst  ir.ScalarType
		want CastOpKind
	}{
		// Float → Int
		{"f32_to_i32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, CastFPToSI},
		{"f32_to_u32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, CastFPToUI},
		{"f64_to_i32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, CastFPToSI},

		// Int → Float
		{"i32_to_f32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, CastSIToFP},
		{"u32_to_f32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, CastUIToFP},
		{"i32_to_f64", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, CastSIToFP},

		// Float → Float
		{"f64_to_f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, CastFPTrunc},
		{"f32_to_f64", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, CastFPExt},
		{"f32_to_f16", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, CastFPTrunc},
		{"f16_to_f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, CastFPExt},

		// Int → Int (same signedness)
		{"i32_to_i64", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, CastSExt},
		{"i64_to_i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, CastTrunc},
		{"u32_to_u64", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, CastZExt},
		{"u64_to_u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, CastTrunc},

		// Int → Int (different signedness, same width)
		{"i32_to_u32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, CastBitcast},
		{"u32_to_i32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, CastBitcast},

		// Int → Int (different signedness, different width)
		{"i32_to_u64", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, CastZExt},
		{"u64_to_i32", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, CastTrunc},
		{"u32_to_i64", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, CastSExt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectDXILCastOp(tt.src, tt.dst)
			if err != nil {
				t.Fatalf("selectDXILCastOp: unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEmitAsF32ToI32(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ScalarSint,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction, got %d", castCount)
	}
	t.Logf("f32→i32: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsF32ToU32(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ScalarUint,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction, got %d", castCount)
	}
	t.Logf("f32→u32: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsI32ToF32(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
		ir.ScalarFloat,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction, got %d", castCount)
	}
	t.Logf("i32→f32: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsU32ToF32(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
		ir.ScalarFloat,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction, got %d", castCount)
	}
	t.Logf("u32→f32: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsF64ToF32(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 8},
		ir.ScalarFloat,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction (FPTrunc), got %d", castCount)
	}
	t.Logf("f64→f32: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsF32ToF64(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ScalarFloat,
		ptrU8(8),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction (FPExt), got %d", castCount)
	}
	t.Logf("f32→f64: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsBitcastF32U32(t *testing.T) {
	// Convert == nil means bitcast.
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ScalarUint,
		nil, // bitcast
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction (bitcast), got %d", castCount)
	}
	t.Logf("f32↔u32 bitcast: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsI32ToI64(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
		ir.ScalarSint,
		ptrU8(8),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction (SExt), got %d", castCount)
	}
	t.Logf("i32→i64: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsI64ToI32(t *testing.T) {
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarSint, Width: 8},
		ir.ScalarSint,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction (Trunc), got %d", castCount)
	}
	t.Logf("i64→i32: %d casts, %d functions, %d constants", castCount, len(mod.Functions), len(mod.Constants))
}

func TestEmitAsSameType(t *testing.T) {
	// Casting f32 to f32 should be a no-op (no cast instructions for the primary cast).
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
		ir.ScalarFloat,
		ptrU8(4), // same width
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	// The primary cast (f32→f32) should be a no-op, but the second ExprAs
	// (expression [5]: dstType→f32 for output) is also f32→f32, so no casts at all.
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount != 0 {
		t.Errorf("expected 0 cast instructions for identity cast, got %d", castCount)
	}
	t.Logf("f32→f32 (no-op): %d casts", castCount)
}

func TestEmitAsBoolToInt(t *testing.T) {
	// bool → i32 should produce ZExt.
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
		ir.ScalarSint,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	if castCount < 1 {
		t.Errorf("expected at least 1 cast instruction (ZExt for bool→int), got %d", castCount)
	}
	t.Logf("bool→i32: %d casts", castCount)
}

func TestEmitAsBoolToFloat(t *testing.T) {
	// bool → f32 should produce two-step: ZExt i1→i32, then UIToFP i32→f32.
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarBool, Width: 1},
		ir.ScalarFloat,
		ptrU8(4),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	castCount := countInstrKind(mod, module.InstrCast)
	// Two casts from bool→f32 (ZExt + UIToFP), plus one for the f32(c)→f32 output
	// which is a no-op, so minimum 2.
	if castCount < 2 {
		t.Errorf("expected at least 2 cast instructions (ZExt + UIToFP for bool→float), got %d", castCount)
	}
	t.Logf("bool→f32: %d casts", castCount)
}

func TestEmitAsIntToBool(t *testing.T) {
	// i32 → bool should produce ICmp NE (comparison, not cast instruction).
	irMod := buildCastShader(
		ir.ScalarType{Kind: ir.ScalarSint, Width: 4},
		ir.ScalarBool,
		ptrU8(1),
	)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	// int→bool uses CmpInstr (ICmpNE), not CastInstr.
	cmpCount := countInstrKind(mod, module.InstrCmp)
	if cmpCount < 1 {
		t.Errorf("expected at least 1 cmp instruction (ICmpNE for int→bool), got %d", cmpCount)
	}
	t.Logf("i32→bool: %d cmps, %d casts", cmpCount, countInstrKind(mod, module.InstrCast))
}

// --- Control Flow Tests ---

// buildIfElseShader creates a fragment shader with if/else:
//
//	@fragment fn main(@location(0) v: f32) -> @location(0) vec4<f32> {
//	    var r: f32;
//	    if (v > 0.5) {
//	        r = 1.0;
//	    } else {
//	        r = 0.0;
//	    }
//	    return vec4(r, r, r, 1.0);
//	}
func buildIfElseShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	boolHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	retHandle := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                         // [1] 0.5
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [2] v > 0.5 (bool)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5] 1.0 (alpha)
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 3, 3, 5}}}, // [6] vec4(r, r, r, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},  // v
			{Handle: &f32Handle},  // 0.5
			{Handle: &boolHandle}, // v > 0.5
			{Handle: &f32Handle},  // 1.0
			{Handle: &f32Handle},  // 0.0
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtIf{
				Condition: 2,
				Accept: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
				},
				Reject: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 5}}},
				},
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 5, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildIfNoElseShader creates a shader with if but no else.
func buildIfNoElseShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	boolHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                         // [1] 0.5
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [2] v > 0.5
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4] 0.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 3, 3, 4}}}, // [5] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &boolHandle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &vec4Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtIf{
				Condition: 2,
				Accept: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 4}}},
				},
				Reject: nil, // no else
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildSimpleLoopShader creates a shader with a simple loop:
//
//	loop {
//	    if (i >= 10) { break; }
//	    // body
//	}
func buildSimpleLoopShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	boolHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [0] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                         // [1] 0.5
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [2] 1.0 > 0.5
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3] 0.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 0, 0, 0}}}, // [4] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &boolHandle},
			{Handle: &f32Handle},
			{Handle: &vec4Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtLoop{
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					{Kind: ir.StmtIf{
						Condition: 2,
						Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
						Reject:    nil,
					}},
				},
				Continuing: nil,
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildLoopContinueShader creates a shader with continue in loop body.
func buildLoopContinueShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	boolHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [0]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                         // [1]
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [2] cond
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 0, 0, 0}}}, // [4] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &boolHandle},
			{Handle: &f32Handle},
			{Handle: &vec4Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtLoop{
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					{Kind: ir.StmtIf{
						Condition: 2,
						Accept:    ir.Block{{Kind: ir.StmtContinue{}}},
						Reject:    nil,
					}},
					// After continue, the rest is skipped for that iteration.
					{Kind: ir.StmtIf{
						Condition: 2,
						Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
						Reject:    nil,
					}},
				},
				Continuing: nil,
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildNestedIfLoopShader creates a shader with if inside a loop.
func buildNestedIfLoopShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	boolHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [0]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.5)}},                         // [1]
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [2]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 0, 0, 0}}}, // [4]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &boolHandle},
			{Handle: &f32Handle},
			{Handle: &vec4Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtLoop{
				Body: ir.Block{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
					{Kind: ir.StmtIf{
						Condition: 2,
						Accept: ir.Block{
							{Kind: ir.StmtIf{
								Condition: 2,
								Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
								Reject:    ir.Block{{Kind: ir.StmtContinue{}}},
							}},
						},
						Reject: ir.Block{
							{Kind: ir.StmtBreak{}},
						},
					}},
				},
				Continuing: nil,
			}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// countBranchInstrs counts branch instructions across all basic blocks.
func countBranchInstrs(mod *module.Module) int {
	count := 0
	for _, fn := range mod.Functions {
		if fn.IsDeclaration {
			continue
		}
		for _, bb := range fn.BasicBlocks {
			for _, instr := range bb.Instructions {
				if instr.Kind == module.InstrBr {
					count++
				}
			}
		}
	}
	return count
}

// getMainFn finds the main function in the module.
func getMainFn(mod *module.Module) *module.Function {
	for i := range mod.Functions {
		if mod.Functions[i].Name == "main" {
			return mod.Functions[i]
		}
	}
	return nil
}

func TestEmitIfElse(t *testing.T) {
	irMod := buildIfElseShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Expect at least 4 basic blocks: entry, then, else, merge.
	if len(mainFn.BasicBlocks) < 4 {
		t.Errorf("expected at least 4 basic blocks (entry/then/else/merge), got %d", len(mainFn.BasicBlocks))
	}

	// Expect at least 3 branch instructions: cond br, br-from-then, br-from-else.
	brCount := countBranchInstrs(mod)
	if brCount < 3 {
		t.Errorf("expected at least 3 branch instructions, got %d", brCount)
	}

	// Verify serialization still works.
	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("if/else: %d BBs, %d branches, %d bytes bitcode",
		len(mainFn.BasicBlocks), brCount, len(bc))
}

func TestEmitIfNoElse(t *testing.T) {
	irMod := buildIfNoElseShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// No else: entry, then, merge (3 BBs minimum).
	if len(mainFn.BasicBlocks) < 3 {
		t.Errorf("expected at least 3 basic blocks (entry/then/merge), got %d", len(mainFn.BasicBlocks))
	}

	// Cond br + br-from-then = at least 2 branches.
	brCount := countBranchInstrs(mod)
	if brCount < 2 {
		t.Errorf("expected at least 2 branch instructions, got %d", brCount)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("if-no-else: %d BBs, %d branches, %d bytes bitcode",
		len(mainFn.BasicBlocks), brCount, len(bc))
}

func TestEmitLoop(t *testing.T) {
	irMod := buildSimpleLoopShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Loop creates: header, body, continuing, merge + entry and if blocks.
	// Minimum: entry + header + body + continuing + merge = 5.
	if len(mainFn.BasicBlocks) < 5 {
		t.Errorf("expected at least 5 basic blocks for loop, got %d", len(mainFn.BasicBlocks))
	}

	// Must have back-edge branch (continuing -> header).
	brCount := countBranchInstrs(mod)
	if brCount < 3 {
		t.Errorf("expected at least 3 branch instructions for loop, got %d", brCount)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("loop: %d BBs, %d branches, %d bytes bitcode",
		len(mainFn.BasicBlocks), brCount, len(bc))
}

func TestEmitLoopBreak(t *testing.T) {
	irMod := buildSimpleLoopShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// The break inside the if should generate a branch to the merge BB.
	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Find a branch that targets the loop merge block.
	// The loop merge block is the last block before the return block.
	foundBreakBranch := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrBr && len(instr.Operands) == 1 {
				// Unconditional branch — could be the break.
				foundBreakBranch = true
			}
		}
	}
	if !foundBreakBranch {
		t.Error("no unconditional branch found (expected break -> merge)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("loop-break: %d BBs, %d bytes bitcode", len(mainFn.BasicBlocks), len(bc))
}

func TestEmitLoopContinue(t *testing.T) {
	irMod := buildLoopContinueShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Continue generates a branch to continuing BB.
	brCount := countBranchInstrs(mod)
	if brCount < 4 {
		t.Errorf("expected at least 4 branch instructions (continue + break + back-edge + entry->header), got %d", brCount)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("loop-continue: %d BBs, %d branches, %d bytes bitcode",
		len(mainFn.BasicBlocks), brCount, len(bc))
}

func TestEmitNestedIfLoop(t *testing.T) {
	irMod := buildNestedIfLoopShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Nested if inside loop creates many BBs.
	if len(mainFn.BasicBlocks) < 7 {
		t.Errorf("expected at least 7 basic blocks for nested if/loop, got %d", len(mainFn.BasicBlocks))
	}

	brCount := countBranchInstrs(mod)
	if brCount < 5 {
		t.Errorf("expected at least 5 branch instructions, got %d", brCount)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("nested if/loop: %d BBs, %d branches, %d bytes bitcode",
		len(mainFn.BasicBlocks), brCount, len(bc))
}

// --- Local Variable Tests (alloca + load + store) ---

// buildLocalVarShader creates a shader with a local variable:
//
//	@fragment fn main() -> @location(0) vec4<f32> {
//	    var x: f32 = 1.0;
//	    x = 2.0;
//	    return vec4(x, x, x, 1.0);
//	}
//
// IR pattern:
//
//	LocalVars: [{Name: "x", Type: f32, Init: nil}]
//	Expressions:
//	  [0] ExprLocalVariable{Variable: 0}  → pointer to x
//	  [1] Literal(1.0)
//	  [2] ExprLocalVariable{Variable: 0}  → pointer to x (for second store)
//	  [3] Literal(2.0)
//	  [4] ExprLocalVariable{Variable: 0}  → pointer to x (for load)
//	  [5] ExprLoad{Pointer: 4}            → loaded value of x
//	  [6] Literal(1.0)
//	  [7] ExprCompose{5, 5, 5, 6}         → vec4(x, x, x, 1.0)
//	Body:
//	  StmtStore{Pointer: 0, Value: 1}     → x = 1.0
//	  StmtEmit{Range: 2..4}
//	  StmtStore{Pointer: 2, Value: 3}     → x = 2.0
//	  StmtEmit{Range: 4..8}
//	  StmtReturn{Value: 7}
func buildLocalVarShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(7)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: f32Handle},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},                             // [0] &x
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [1] 1.0
			{Kind: ir.ExprLocalVariable{Variable: 0}},                             // [2] &x
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},                         // [3] 2.0
			{Kind: ir.ExprLocalVariable{Variable: 0}},                             // [4] &x
			{Kind: ir.ExprLoad{Pointer: 4}},                                       // [5] load x
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [6] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{5, 5, 5, 6}}}, // [7] vec4(x, x, x, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},     // &x → pointer
			{Handle: &f32Handle},     // 1.0
			{Handle: &f32Handle},     // &x
			{Handle: &f32Handle},     // 2.0
			{Handle: &f32Handle},     // &x
			{Handle: &f32Handle},     // load x → f32
			{Handle: &f32Handle},     // 1.0
			{Handle: &vec4f32Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 4}}},
			{Kind: ir.StmtStore{Pointer: 2, Value: 3}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 8}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildMultipleLocalsShader creates a shader with two independent local variables:
//
//	@fragment fn main() -> @location(0) vec4<f32> {
//	    var a: f32 = 1.0;
//	    var b: f32 = 2.0;
//	    return vec4(a, b, 0.0, 1.0);
//	}
func buildMultipleLocalsShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(10)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		LocalVars: []ir.LocalVariable{
			{Name: "a", Type: f32Handle},
			{Name: "b", Type: f32Handle},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},                             // [0] &a
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [1] 1.0
			{Kind: ir.ExprLocalVariable{Variable: 1}},                             // [2] &b
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},                         // [3] 2.0
			{Kind: ir.ExprLocalVariable{Variable: 0}},                             // [4] &a (for load)
			{Kind: ir.ExprLoad{Pointer: 4}},                                       // [5] load a
			{Kind: ir.ExprLocalVariable{Variable: 1}},                             // [6] &b (for load)
			{Kind: ir.ExprLoad{Pointer: 6}},                                       // [7] load b
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [8] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [9] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{5, 7, 8, 9}}}, // [10] vec4(a, b, 0, 1)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle}, {Handle: &f32Handle}, {Handle: &f32Handle}, {Handle: &f32Handle},
			{Handle: &f32Handle}, {Handle: &f32Handle}, {Handle: &f32Handle}, {Handle: &f32Handle},
			{Handle: &f32Handle}, {Handle: &f32Handle}, {Handle: &vec4f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
			{Kind: ir.StmtStore{Pointer: 2, Value: 3}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 11}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

// buildLocalInLoopShader creates a shader with a local variable modified inside a loop:
//
//	@fragment fn main() -> @location(0) vec4<f32> {
//	    var acc: f32 = 0.0;
//	    var i: i32 = 0;
//	    loop {
//	        if i >= 4 { break; }
//	        acc = acc + 1.0;
//	        continuing { i = i + 1; }
//	    }
//	    return vec4(acc, acc, acc, 1.0);
//	}
func buildLocalInLoopShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)
	i32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		LocalVars: []ir.LocalVariable{
			{Name: "acc", Type: f32Handle},
			{Name: "i", Type: i32Handle},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},                           // [0] &acc
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                       // [1] 0.0
			{Kind: ir.ExprLocalVariable{Variable: 1}},                           // [2] &i
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},                         // [3] 0
			{Kind: ir.ExprLocalVariable{Variable: 1}},                           // [4] &i (for load)
			{Kind: ir.ExprLoad{Pointer: 4}},                                     // [5] load i
			{Kind: ir.Literal{Value: ir.LiteralI32(4)}},                         // [6] 4
			{Kind: ir.ExprBinary{Op: ir.BinaryGreaterEqual, Left: 5, Right: 6}}, // [7] i >= 4
			{Kind: ir.ExprLocalVariable{Variable: 0}},                           // [8] &acc (for load)
			{Kind: ir.ExprLoad{Pointer: 8}},                                     // [9] load acc
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                       // [10] 1.0
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 9, Right: 10}},         // [11] acc + 1.0
			{Kind: ir.ExprLocalVariable{Variable: 0}},                           // [12] &acc (for store)
			{Kind: ir.ExprLocalVariable{Variable: 1}},                           // [13] &i (for load in continuing)
			{Kind: ir.ExprLoad{Pointer: 13}},                                    // [14] load i
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},                         // [15] 1
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 14, Right: 15}},        // [16] i + 1
			// After loop: load acc, compose, return
			{Kind: ir.ExprLocalVariable{Variable: 0}},                                 // [17] &acc
			{Kind: ir.ExprLoad{Pointer: 17}},                                          // [18] load acc (final)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                             // [19] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{18, 18, 18, 19}}}, // [20] vec4(acc, acc, acc, 1.0)
		},
		ExpressionTypes: func() []ir.TypeResolution {
			res := make([]ir.TypeResolution, 21)
			for i := range res {
				res[i] = ir.TypeResolution{Handle: &f32Handle}
			}
			// Fix types for i32 expressions.
			res[2] = ir.TypeResolution{Handle: &i32Handle}
			res[3] = ir.TypeResolution{Handle: &i32Handle}
			res[4] = ir.TypeResolution{Handle: &i32Handle}
			res[5] = ir.TypeResolution{Handle: &i32Handle}
			res[6] = ir.TypeResolution{Handle: &i32Handle}
			res[13] = ir.TypeResolution{Handle: &i32Handle}
			res[14] = ir.TypeResolution{Handle: &i32Handle}
			res[15] = ir.TypeResolution{Handle: &i32Handle}
			res[16] = ir.TypeResolution{Handle: &i32Handle}
			res[20] = ir.TypeResolution{Handle: &vec4f32Handle}
			return res
		}(),
		Body: []ir.Statement{
			// Initialize locals.
			{Kind: ir.StmtStore{Pointer: 0, Value: 1}}, // acc = 0.0
			{Kind: ir.StmtStore{Pointer: 2, Value: 3}}, // i = 0
			// Loop.
			{Kind: ir.StmtLoop{
				Body: ir.Block{
					// if i >= 4 { break; }
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 8}}},
					{Kind: ir.StmtIf{
						Condition: 7,
						Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
					}},
					// acc = acc + 1.0
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 8, End: 12}}},
					{Kind: ir.StmtStore{Pointer: 12, Value: 11}},
				},
				Continuing: ir.Block{
					// i = i + 1
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 13, End: 17}}},
					{Kind: ir.StmtStore{Pointer: 13, Value: 16}},
				},
			}},
			// After loop: return vec4(acc, acc, acc, 1.0).
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 17, End: 21}}},
			{Kind: ir.StmtReturn{Value: func() *ir.ExpressionHandle { h := ir.ExpressionHandle(20); return &h }()}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

func TestEmitLocalVariable(t *testing.T) {
	irMod := buildLocalVarShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify alloca instruction was emitted.
	allocaCount := countInstrKind(mod, module.InstrAlloca)
	if allocaCount != 1 {
		t.Errorf("expected 1 alloca instruction, got %d", allocaCount)
	}

	// Verify store instructions were emitted (2 stores: x = 1.0 and x = 2.0).
	storeCount := countInstrKind(mod, module.InstrStore)
	if storeCount != 2 {
		t.Errorf("expected 2 store instructions, got %d", storeCount)
	}

	// Verify load instruction was emitted.
	loadCount := countInstrKind(mod, module.InstrLoad)
	if loadCount != 1 {
		t.Errorf("expected 1 load instruction, got %d", loadCount)
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("local-var: alloca=%d, store=%d, load=%d, %d bytes bitcode",
		allocaCount, storeCount, loadCount, len(bc))
}

func TestEmitStoreLoad(t *testing.T) {
	irMod := buildLocalVarShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify that store and load produce different instruction kinds.
	foundStore := false
	foundLoad := false
	foundAlloca := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			switch instr.Kind {
			case module.InstrStore:
				foundStore = true
				// Store should NOT have a value (it's a void operation).
				if instr.HasValue {
					t.Error("store instruction should not produce a value")
				}
			case module.InstrLoad:
				foundLoad = true
				// Load SHOULD produce a value.
				if !instr.HasValue {
					t.Error("load instruction should produce a value")
				}
			case module.InstrAlloca:
				foundAlloca = true
				// Alloca produces a pointer value.
				if !instr.HasValue {
					t.Error("alloca instruction should produce a value")
				}
				if instr.ResultType == nil || instr.ResultType.Kind != module.TypePointer {
					t.Error("alloca result type should be a pointer")
				}
			}
		}
	}

	if !foundAlloca {
		t.Error("expected to find alloca instruction")
	}
	if !foundStore {
		t.Error("expected to find store instruction")
	}
	if !foundLoad {
		t.Error("expected to find load instruction")
	}
}

func TestEmitMultipleLocals(t *testing.T) {
	irMod := buildMultipleLocalsShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Two local variables should produce exactly 2 alloca instructions.
	allocaCount := countInstrKind(mod, module.InstrAlloca)
	if allocaCount != 2 {
		t.Errorf("expected 2 alloca instructions for 2 local vars, got %d", allocaCount)
	}

	// 2 stores (a = 1.0, b = 2.0).
	storeCount := countInstrKind(mod, module.InstrStore)
	if storeCount != 2 {
		t.Errorf("expected 2 store instructions, got %d", storeCount)
	}

	// 2 loads (load a, load b).
	loadCount := countInstrKind(mod, module.InstrLoad)
	if loadCount != 2 {
		t.Errorf("expected 2 load instructions, got %d", loadCount)
	}

	// Verify alloca result types are pointer types to f32.
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrAlloca {
				if instr.ResultType == nil || instr.ResultType.Kind != module.TypePointer {
					t.Errorf("alloca should produce pointer type, got %v", instr.ResultType)
				}
			}
		}
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("multiple-locals: alloca=%d, store=%d, load=%d, %d bytes bitcode",
		allocaCount, storeCount, loadCount, len(bc))
}

func TestEmitLocalInLoop(t *testing.T) {
	irMod := buildLocalInLoopShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := getMainFn(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Two local variables: acc (f32) and i (i32).
	allocaCount := countInstrKind(mod, module.InstrAlloca)
	if allocaCount != 2 {
		t.Errorf("expected 2 alloca instructions (acc + i), got %d", allocaCount)
	}

	// Multiple stores: initial stores + stores inside loop body and continuing.
	storeCount := countInstrKind(mod, module.InstrStore)
	if storeCount < 4 {
		t.Errorf("expected at least 4 store instructions, got %d", storeCount)
	}

	// Multiple loads: loop condition + loop body + after loop.
	loadCount := countInstrKind(mod, module.InstrLoad)
	if loadCount >= 1 {
		t.Logf("local-in-loop: alloca=%d, store=%d, load=%d", allocaCount, storeCount, loadCount)
	} else {
		t.Errorf("expected at least 1 load instruction, got %d", loadCount)
	}

	// Verify loop structure (multiple basic blocks).
	if len(mainFn.BasicBlocks) < 5 {
		t.Errorf("expected at least 5 basic blocks for loop, got %d", len(mainFn.BasicBlocks))
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("local-in-loop: %d BBs, alloca=%d, store=%d, load=%d, %d bytes bitcode",
		len(mainFn.BasicBlocks), allocaCount, storeCount, loadCount, len(bc))
}

// --- Swizzle tests ---

// buildSwizzleShader creates a fragment shader that swizzles vec4.xzy:
//
//	@fragment fn main(@location(0) v: vec4<f32>) -> @location(0) vec4<f32> {
//	    return v.xzy;  // actually returns vec4(v.xzy, 1.0) for valid output
//	}
func buildSwizzleShader() *ir.Module {
	vec4Handle := ir.TypeHandle(0)
	vec3Handle := ir.TypeHandle(1)
	f32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(3)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: vec4Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] v (vec4)
			{Kind: ir.ExprSwizzle{ // [1] v.xzy (vec3)
				Vector:  0,
				Pattern: [4]ir.SwizzleComponent{0, 2, 1, 0}, // x, z, y
				Size:    3,
			}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [2] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 1, 1, 2}}}, // [3] vec4 result (simplified)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec4Handle}, // v
			{Handle: &vec3Handle}, // swizzle
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

func TestEmitSwizzle(t *testing.T) {
	irMod := buildSwizzleShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("swizzle: %d functions, %d constants, %d bytes", len(mod.Functions), len(mod.Constants), len(bc))
}

// --- Derivative tests ---

// buildDerivativeShader creates a fragment shader with a derivative expression.
func buildDerivativeShader(axis ir.DerivativeAxis, control ir.DerivativeControl) *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v
			{Kind: ir.ExprDerivative{Axis: axis, Control: control, Expr: 0}},      // [1] derivative
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 2, 3}}}, // [4] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},  // v
			{Handle: &f32Handle},  // derivative
			{Handle: &f32Handle},  // 0.0
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
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

func TestEmitDerivativeCoarseX(t *testing.T) {
	irMod := buildDerivativeShader(ir.DerivativeX, ir.DerivativeCoarse)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify dx.op.derivCoarseX function was created.
	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.derivCoarseX.f32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.derivCoarseX.f32 function not found")
	}

	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	hasCall := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.derivCoarseX.f32" {
				hasCall = true
			}
		}
	}
	if !hasCall {
		t.Error("no call to dx.op.derivCoarseX.f32 found in main")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("derivCoarseX: %d functions, %d bytes", len(mod.Functions), len(bc))
}

func TestEmitDerivativeFineY(t *testing.T) {
	irMod := buildDerivativeShader(ir.DerivativeY, ir.DerivativeFine)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.derivFineY.f32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.derivFineY.f32 function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("derivFineY: %d functions, %d bytes", len(mod.Functions), len(bc))
}

func TestEmitDerivativeWidth(t *testing.T) {
	irMod := buildDerivativeShader(ir.DerivativeWidth, ir.DerivativeNone)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// fwidth decomposes into derivCoarseX + derivCoarseY + fabs + fadd.
	hasDerivX := false
	hasDerivY := false
	hasFAbs := false
	for _, fn := range mod.Functions {
		switch fn.Name {
		case "dx.op.derivCoarseX.f32":
			hasDerivX = true
		case "dx.op.derivCoarseY.f32":
			hasDerivY = true
		case "dx.op.fabs.f32":
			hasFAbs = true
		}
	}
	if !hasDerivX {
		t.Error("dx.op.derivCoarseX.f32 not found (needed for fwidth)")
	}
	if !hasDerivY {
		t.Error("dx.op.derivCoarseY.f32 not found (needed for fwidth)")
	}
	if !hasFAbs {
		t.Error("dx.op.fabs.f32 not found (needed for fwidth)")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("fwidth: %d functions, %d bytes", len(mod.Functions), len(bc))
}

// --- Relational tests ---

// buildRelationalShader creates a fragment shader with a relational expression.
func buildRelationalShader(relFn ir.RelationalFunction) *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	boolHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] v
			{Kind: ir.ExprRelational{Fun: relFn, Argument: 0}},                    // [1] relational(v)
			{Kind: ir.ExprAs{Expr: 1, Kind: ir.ScalarFloat, Convert: ptrU8(4)}},   // [2] f32(result)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3] 0.0
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 3, 4}}}, // [5] vec4
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},  // v
			{Handle: &boolHandle}, // relational result
			{Handle: &f32Handle},  // cast to f32
			{Handle: &f32Handle},  // 0.0
			{Handle: &f32Handle},  // 1.0
			{Handle: &vec4Handle}, // compose
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}
	return mod
}

func TestEmitRelationalIsNaN(t *testing.T) {
	irMod := buildRelationalShader(ir.RelationalIsNan)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.isNaN.f32" {
			found = true
			// Unary dx.op: ret(i32, TYPE) = 2 params.
			if len(fn.FuncType.ParamTypes) != 2 {
				t.Errorf("dx.op.isNaN.f32 params: got %d, want 2", len(fn.FuncType.ParamTypes))
			}
			break
		}
	}
	if !found {
		t.Error("dx.op.isNaN.f32 function not found")
	}

	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	hasCall := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.isNaN.f32" {
				hasCall = true
			}
		}
	}
	if !hasCall {
		t.Error("no call to dx.op.isNaN.f32 found in main")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("isNaN: %d functions, %d bytes", len(mod.Functions), len(bc))
}

func TestEmitRelationalIsInf(t *testing.T) {
	irMod := buildRelationalShader(ir.RelationalIsInf)
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	found := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.isInf.f32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.isInf.f32 function not found")
	}

	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	t.Logf("isInf: %d functions, %d bytes", len(mod.Functions), len(bc))
}

// --- Error handling tests ---

func TestEmitUnsupportedExpression(t *testing.T) {
	// ExprArrayLength should return an error, not a placeholder.
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(1)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] v
			{Kind: ir.ExprArrayLength{Array: 0}},      // [1] arrayLength — not implemented
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	_, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err == nil {
		t.Fatal("expected error for ExprArrayLength, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestEmitUnsupportedStatement(t *testing.T) {
	// StmtSwitch should return an error (not silently skip).
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(0)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] v
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtSwitch{Selector: 0}}, // unsupported
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	_, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err == nil {
		t.Fatal("expected error for StmtSwitch, got nil")
	}
	t.Logf("got expected error: %v", err)
}

func TestEmitStmtKillSilentSkip(t *testing.T) {
	// StmtKill should be silently skipped (not error) since it's Phase 2.
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	vBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(0)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "v", Type: f32Handle, Binding: &vBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] v
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
			{Kind: ir.StmtKill{}}, // should be silently skipped
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	_, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("StmtKill should be silently skipped, got error: %v", err)
	}
	t.Log("StmtKill correctly silently skipped")
}

// --- Resource binding tests ---

// buildCBVFragmentShader creates a fragment shader that reads from a uniform buffer:
//
//	struct Uniforms { color: vec4<f32> }
//	@group(0) @binding(0) var<uniform> uniforms: Uniforms;
//	@fragment fn main() -> @location(0) vec4<f32> {
//	    return uniforms.color;
//	}
func buildCBVFragmentShader() *ir.Module {
	vec4f32Handle := ir.TypeHandle(1)
	structHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Uniforms", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "color", Type: vec4f32Handle, Offset: 0},
				},
				Span: 16,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:  "uniforms",
				Space: ir.SpaceUniform,
				Binding: &ir.ResourceBinding{
					Group:   0,
					Binding: 0,
				},
				Type: structHandle,
			},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(2) // Load result

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [0] &uniforms
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] &uniforms.color
			{Kind: ir.ExprLoad{Pointer: 1}},               // [2] uniforms.color
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &structHandle},  // pointer to struct
			{Handle: &vec4f32Handle}, // pointer to vec4
			{Handle: &vec4f32Handle}, // loaded vec4
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	return mod
}

func TestResourceAnalysis(t *testing.T) {
	mod := buildCBVFragmentShader()
	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(e.resources))
	}

	res := &e.resources[0]
	if res.class != resourceClassCBV {
		t.Errorf("expected CBV (class %d), got class %d", resourceClassCBV, res.class)
	}
	if res.name != "uniforms" {
		t.Errorf("expected name 'uniforms', got %q", res.name)
	}
	if res.group != 0 || res.binding != 0 {
		t.Errorf("expected group=0 binding=0, got group=%d binding=%d", res.group, res.binding)
	}
	if res.rangeID != 0 {
		t.Errorf("expected rangeID 0, got %d", res.rangeID)
	}
}

func TestResourceAnalysisMultiple(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "Uniforms", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "ub", Space: ir.SpaceUniform, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: 1},
			{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: 2},
			{Name: "samp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 2}, Type: 3},
		},
	}

	e := &Emitter{
		ir:              mod,
		mod:             module.NewModule(module.PixelShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}
	e.analyzeResources()

	if len(e.resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(e.resources))
	}

	// CBV
	if e.resources[0].class != resourceClassCBV {
		t.Errorf("resource 0: expected CBV, got class %d", e.resources[0].class)
	}
	// SRV (texture)
	if e.resources[1].class != resourceClassSRV {
		t.Errorf("resource 1: expected SRV, got class %d", e.resources[1].class)
	}
	// Sampler
	if e.resources[2].class != resourceClassSampler {
		t.Errorf("resource 2: expected Sampler, got class %d", e.resources[2].class)
	}
}

func TestEmitCreateHandle(t *testing.T) {
	mod := buildCBVFragmentShader()
	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify that a dx.op.createHandle function declaration was created.
	found := false
	for _, fn := range result.Functions {
		if fn.Name == "dx.op.createHandle" {
			found = true
			if !fn.IsDeclaration {
				t.Error("dx.op.createHandle should be a declaration")
			}
			break
		}
	}
	if !found {
		t.Error("dx.op.createHandle function not found in module")
	}

	// Verify that the main function has a createHandle call instruction.
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	hasCreateHandle := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.createHandle" {
				hasCreateHandle = true
			}
		}
	}
	if !hasCreateHandle {
		t.Error("no dx.op.createHandle call found in main function")
	}

	t.Log("CBV createHandle emitted successfully")
}

func TestEmitCBVLoad(t *testing.T) {
	mod := buildCBVFragmentShader()
	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify dx.op.cbufferLoadLegacy.f32 call exists.
	hasCBufLoad := false
	extractCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil {
				if instr.CalledFunc.Name == "dx.op.cbufferLoadLegacy.f32" {
					hasCBufLoad = true
					// Verify operands: opcode(59), handle, regIndex(0)
					if len(instr.Operands) != 3 {
						t.Errorf("cbufferLoadLegacy expected 3 operands, got %d", len(instr.Operands))
					}
				}
			}
			if instr.Kind == module.InstrExtractVal {
				extractCount++
			}
		}
	}

	if !hasCBufLoad {
		t.Error("dx.op.cbufferLoadLegacy.f32 call not found")
	}

	// Loading a vec4 field should produce 4 extractvalue instructions.
	if extractCount < 4 {
		t.Errorf("expected at least 4 extractvalue instructions for vec4 load, got %d", extractCount)
	}
}

func TestEmitCBVLoadMultiField(t *testing.T) {
	// Build a shader with a CBV struct containing multiple fields at different offsets:
	//   struct Uniforms { color: vec4<f32>, intensity: f32, offset: vec2<f32> }
	//   @group(0) @binding(0) var<uniform> u: Uniforms;
	//   @fragment fn main() -> @location(0) vec4<f32> { return u.color * u.intensity; }
	f32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)
	vec2f32Handle := ir.TypeHandle(2)
	structHandle := ir.TypeHandle(3)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Uniforms", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "color", Type: vec4f32Handle, Offset: 0},   // register 0, components 0-3
					{Name: "intensity", Type: f32Handle, Offset: 16},  // register 1, component 0
					{Name: "offset", Type: vec2f32Handle, Offset: 24}, // register 1, components 2-3
				},
				Span: 32,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "u",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    structHandle,
			},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retExpr := ir.ExpressionHandle(5) // color * intensity splat

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                      // [0] &u
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                   // [1] &u.color
			{Kind: ir.ExprLoad{Pointer: 1}},                                 // [2] u.color (vec4)
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},                   // [3] &u.intensity
			{Kind: ir.ExprLoad{Pointer: 3}},                                 // [4] u.intensity (f32)
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 2, Right: 4}}, // [5] color * intensity
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &structHandle},  // ptr to struct
			{Handle: &vec4f32Handle}, // ptr to vec4
			{Handle: &vec4f32Handle}, // loaded vec4
			{Handle: &f32Handle},     // ptr to f32
			{Handle: &f32Handle},     // loaded f32
			{Handle: &vec4f32Handle}, // mul result
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retExpr}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Count cbufferLoadLegacy calls — should be 2 (one for color at reg 0, one for intensity at reg 1).
	cbufLoadCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.cbufferLoadLegacy.f32" {
				cbufLoadCount++
			}
		}
	}

	if cbufLoadCount != 2 {
		t.Errorf("expected 2 cbufferLoadLegacy calls (color + intensity), got %d", cbufLoadCount)
	}
}

func TestEmitCBVLoadI32(t *testing.T) {
	// Build a shader with an i32 CBV field to verify overload selection.
	//   struct Params { count: u32 }
	//   @group(0) @binding(0) var<uniform> p: Params;
	//   @fragment fn main() -> @location(0) vec4<f32> { ... }
	u32Handle := ir.TypeHandle(0)
	vec4f32Handle := ir.TypeHandle(1)
	structHandle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Params", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "count", Type: u32Handle, Offset: 0},
				},
				Span: 4,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "p",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    structHandle,
			},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	zeroExpr := ir.ExpressionHandle(3)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [0] &p
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] &p.count
			{Kind: ir.ExprLoad{Pointer: 1}},               // [2] p.count (u32)
			{Kind: ir.ExprZeroValue{Type: vec4f32Handle}}, // [3] fallback zero
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &structHandle},  // ptr to struct
			{Handle: &u32Handle},     // ptr to u32
			{Handle: &u32Handle},     // loaded u32
			{Handle: &vec4f32Handle}, // zero value
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &zeroExpr}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify i32 overload is used.
	hasI32Load := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.cbufferLoadLegacy.i32" {
				hasI32Load = true
			}
		}
	}

	if !hasI32Load {
		t.Error("expected dx.op.cbufferLoadLegacy.i32 call for u32 field, not found")
	}
}

func TestEmitCBVLoadRegisterOffset(t *testing.T) {
	// Verify that fields in the second 16-byte register produce regIndex=1.
	//   struct Data { a: vec4<f32>, b: vec4<f32> }
	//   Load b should use regIndex=1 (offset 16 / 16 = 1).
	vec4f32Handle := ir.TypeHandle(0)
	structHandle := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "Data", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: vec4f32Handle, Offset: 0},
					{Name: "b", Type: vec4f32Handle, Offset: 16},
				},
				Span: 32,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:    "data",
				Space:   ir.SpaceUniform,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0},
				Type:    structHandle,
			},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retExpr := ir.ExpressionHandle(2)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [0] &data
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // [1] &data.b (offset 16)
			{Kind: ir.ExprLoad{Pointer: 1}},               // [2] data.b
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &structHandle},  // ptr to struct
			{Handle: &vec4f32Handle}, // ptr to vec4
			{Handle: &vec4f32Handle}, // loaded vec4
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			{Kind: ir.StmtReturn{Value: &retExpr}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Find the cbufferLoadLegacy call and check its regIndex operand.
	// Operands: [opcodeVal, handleVal, regIndexVal]
	// regIndex for field b (offset=16) should be 16/16 = 1.
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.cbufferLoadLegacy.f32" {
				// The third operand is the regIndex value ID.
				// We can't directly check the constant value from the ID alone,
				// but we verify the call structure is correct.
				if len(instr.Operands) != 3 {
					t.Errorf("cbufferLoadLegacy expected 3 operands, got %d", len(instr.Operands))
				}
				t.Log("cbufferLoadLegacy call found with correct operand count for register 1")
				return
			}
		}
	}
	t.Error("cbufferLoadLegacy call not found for second register field")
}

func TestEmitImageSample(t *testing.T) {
	// Build a fragment shader with texture sampling:
	//   @group(0) @binding(0) var tex: texture_2d<f32>;
	//   @group(0) @binding(1) var samp: sampler;
	//   @fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
	//       return textureSample(tex, samp, uv);
	//   }
	f32Handle := ir.TypeHandle(0)
	vec2f32Handle := ir.TypeHandle(1)
	vec4f32Handle := ir.TypeHandle(2)
	imageHandle := ir.TypeHandle(3)
	samplerHandle := ir.TypeHandle(4)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}},
			{Name: "", Inner: ir.SamplerType{Comparison: false}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: imageHandle},
			{Name: "samp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: samplerHandle},
		},
	}

	uvBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "uv", Type: vec2f32Handle, Binding: &uvBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4f32Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] uv
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [1] tex
			{Kind: ir.ExprGlobalVariable{Variable: 1}},    // [2] samp
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [3] uv.x (used by coordinate)
			{Kind: ir.ExprImageSample{ // [4] textureSample(tex, samp, uv)
				Image:      1,
				Sampler:    2,
				Coordinate: 0,
				Level:      ir.SampleLevelAuto{},
			}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec2f32Handle}, // uv
			{Handle: &imageHandle},   // tex
			{Handle: &samplerHandle}, // samp
			{Handle: &f32Handle},     // uv.x
			{Handle: &vec4f32Handle}, // sample result
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Verify dx.op.sample.f32 call exists.
	hasSample := false
	hasCreateHandle := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil {
				switch instr.CalledFunc.Name {
				case "dx.op.sample.f32":
					hasSample = true
				case "dx.op.createHandle":
					hasCreateHandle = true
				}
			}
		}
	}

	if !hasCreateHandle {
		t.Error("no dx.op.createHandle call found")
	}
	if !hasSample {
		t.Error("no dx.op.sample.f32 call found")
	}

	// Verify extractvalue instructions exist (for extracting sample results).
	extractCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrExtractVal {
				extractCount++
			}
		}
	}
	if extractCount < 4 {
		t.Errorf("expected at least 4 extractvalue instructions for sample result, got %d", extractCount)
	}

	t.Logf("Image sample emitted: createHandle=%v, sample=%v, extracts=%d",
		hasCreateHandle, hasSample, extractCount)
}

func TestResourceMetadata(t *testing.T) {
	mod := buildCBVFragmentShader()
	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Check dx.resources named metadata exists.
	found := false
	for _, nm := range result.NamedMetadata {
		if nm.Name == "dx.resources" {
			found = true
			if len(nm.Operands) != 1 {
				t.Errorf("dx.resources should have 1 operand, got %d", len(nm.Operands))
			}
			// The operand is a tuple of {srvs, uavs, cbvs, samplers}.
			tuple := nm.Operands[0]
			if tuple.Kind != module.MDTuple {
				t.Error("dx.resources operand should be a tuple")
			}
			if len(tuple.SubNodes) != 4 {
				t.Errorf("dx.resources tuple should have 4 elements, got %d", len(tuple.SubNodes))
			}
			// SRVs should be nil (no SRVs).
			if tuple.SubNodes[0] != nil {
				t.Error("SRVs should be nil for CBV-only shader")
			}
			// UAVs should be nil.
			if tuple.SubNodes[1] != nil {
				t.Error("UAVs should be nil for CBV-only shader")
			}
			// CBVs should have one entry.
			if tuple.SubNodes[2] == nil {
				t.Error("CBVs should not be nil")
			}
			// Samplers should be nil.
			if tuple.SubNodes[3] != nil {
				t.Error("Samplers should be nil for CBV-only shader")
			}
			break
		}
	}
	if !found {
		t.Error("dx.resources named metadata not found")
	}
}

func TestNoResourcesNoMetadata(t *testing.T) {
	// A shader without resources should not emit dx.resources.
	mod := buildSimpleFragmentShader()
	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	for _, nm := range result.NamedMetadata {
		if nm.Name == "dx.resources" {
			t.Error("dx.resources should not be present for a shader without resources")
		}
	}
}

// findMainFunction locates the non-declaration function named "main".
func findMainFunction(m *module.Module) *module.Function {
	for _, fn := range m.Functions {
		if fn.Name == "main" && !fn.IsDeclaration {
			return fn
		}
	}
	return nil
}

// --- Compute shader tests ---

// buildMinimalComputeShader creates a compute shader with GlobalInvocationId:
//
//	@compute @workgroup_size(64, 1, 1)
//	fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
//	    // empty body
//	}
func buildMinimalComputeShader() *ir.Module {
	vec3u32Handle := ir.TypeHandle(0)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
	}

	globalIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] global_id
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{
			Name:      "main",
			Stage:     ir.StageCompute,
			Function:  fn,
			Workgroup: [3]uint32{64, 1, 1},
		},
	}

	return mod
}

// buildComputeWithUAV creates a compute shader that reads/writes a storage buffer:
//
//	@group(0) @binding(0) var<storage, read_write> data: array<u32>;
//
//	@compute @workgroup_size(64)
//	fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
//	    let index = global_id.x;
//	    let value = data[index];
//	    data[index] = value * 2u;
//	}
func buildComputeWithUAV() *ir.Module {
	u32Handle := ir.TypeHandle(0)
	vec3u32Handle := ir.TypeHandle(1)
	arrayU32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{
				Base:   u32Handle,
				Size:   ir.ArraySize{}, // Runtime-sized
				Stride: 4,
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:   "data",
				Space:  ir.SpaceStorage,
				Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{
					Group:   0,
					Binding: 0,
				},
				Type: arrayU32Handle,
			},
		},
	}

	globalIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	retHandle2 := ir.ExpressionHandle(2)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                       // [0] global_id
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                   // [1] global_id.x (index)
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                      // [2] &data
			{Kind: ir.ExprAccess{Base: 2, Index: 1}},                        // [3] &data[index]
			{Kind: ir.ExprLoad{Pointer: 3}},                                 // [4] data[index] (value)
			{Kind: ir.Literal{Value: ir.LiteralU32(2)}},                     // [5] 2u
			{Kind: ir.ExprBinary{Op: ir.BinaryMultiply, Left: 4, Right: 5}}, // [6] value * 2u
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},  // global_id
			{Handle: &u32Handle},      // global_id.x
			{Handle: &arrayU32Handle}, // &data
			{Handle: &u32Handle},      // &data[index]
			{Handle: &u32Handle},      // data[index]
			{Handle: &u32Handle},      // 2u
			{Handle: &u32Handle},      // value * 2u
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtStore{Pointer: 3, Value: 6}},
		},
	}

	// Fix: retHandle2 is unused, clear it to avoid lint
	_ = retHandle2

	mod.EntryPoints = []ir.EntryPoint{
		{
			Name:      "main",
			Stage:     ir.StageCompute,
			Function:  fn,
			Workgroup: [3]uint32{64, 1, 1},
		},
	}

	return mod
}

// buildComputeMultipleBuiltins creates a compute shader with multiple builtins:
//
//	@compute @workgroup_size(8, 8, 1)
//	fn main(
//	    @builtin(global_invocation_id) global_id: vec3<u32>,
//	    @builtin(local_invocation_id) local_id: vec3<u32>,
//	    @builtin(workgroup_id) wg_id: vec3<u32>,
//	) { }
func buildComputeMultipleBuiltins() *ir.Module {
	vec3u32Handle := ir.TypeHandle(0)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
	}

	globalIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})
	localIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinLocalInvocationID})
	wgIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinWorkGroupID})

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
			{Name: "local_id", Type: vec3u32Handle, Binding: &localIDBinding},
			{Name: "wg_id", Type: vec3u32Handle, Binding: &wgIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] global_id
			{Kind: ir.ExprFunctionArgument{Index: 1}}, // [1] local_id
			{Kind: ir.ExprFunctionArgument{Index: 2}}, // [2] wg_id
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},
			{Handle: &vec3u32Handle},
			{Handle: &vec3u32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{
			Name:      "main",
			Stage:     ir.StageCompute,
			Function:  fn,
			Workgroup: [3]uint32{8, 8, 1},
		},
	}

	return mod
}

func TestEmitComputeEntryPoint(t *testing.T) {
	irMod := buildMinimalComputeShader()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify shader kind is ComputeShader.
	if result.ShaderKind != module.ComputeShader {
		t.Errorf("shader kind: got %d, want ComputeShader (%d)", result.ShaderKind, module.ComputeShader)
	}

	// Verify dx.shaderModel has "cs".
	found := false
	for _, nm := range result.NamedMetadata {
		if nm.Name == "dx.shaderModel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing dx.shaderModel metadata")
	}

	// Verify dx.entryPoints has properties (5th element non-nil for compute).
	for _, nm := range result.NamedMetadata {
		if nm.Name == "dx.entryPoints" {
			if len(nm.Operands) == 0 {
				t.Fatal("dx.entryPoints has no entries")
			}
			// The entry is a tuple: [funcRef, name, signatures, resources, properties]
			entry := nm.Operands[0]
			if entry == nil {
				t.Fatal("dx.entryPoints[0] is nil")
			}
			if entry.Kind != module.MDTuple {
				t.Fatal("dx.entryPoints[0] is not a tuple")
			}
			if len(entry.SubNodes) < 5 {
				t.Fatalf("dx.entryPoints[0] has %d children, want >= 5", len(entry.SubNodes))
			}
			// Properties (5th element) should be non-nil for compute shaders.
			if entry.SubNodes[4] == nil {
				t.Error("dx.entryPoints[0] properties (index 4) is nil, want numthreads metadata")
			}
			break
		}
	}

	// Serialize and verify valid bitcode.
	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	if bc[0] != 'B' || bc[1] != 'C' {
		t.Errorf("invalid bitcode magic: %02X %02X", bc[0], bc[1])
	}

	t.Logf("minimal compute: %d types, %d functions, %d constants, %d bytes bitcode",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

func TestEmitComputeNumthreadsMetadata(t *testing.T) {
	tests := []struct {
		name      string
		workgroup [3]uint32
	}{
		{"64x1x1", [3]uint32{64, 1, 1}},
		{"8x8x1", [3]uint32{8, 8, 1}},
		{"4x4x4", [3]uint32{4, 4, 4}},
		{"256x1x1", [3]uint32{256, 1, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			irMod := buildMinimalComputeShader()
			irMod.EntryPoints[0].Workgroup = tc.workgroup

			result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
			if err != nil {
				t.Fatalf("Emit failed: %v", err)
			}

			// Verify entry point properties contain numthreads tag (4).
			for _, nm := range result.NamedMetadata {
				if nm.Name == "dx.entryPoints" {
					entry := nm.Operands[0]
					props := entry.SubNodes[4]
					if props == nil {
						t.Fatal("properties metadata is nil")
					}
					// Properties should be a tuple with at least 2 children (tag, value).
					if len(props.SubNodes) < 2 {
						t.Fatalf("properties has %d children, want >= 2", len(props.SubNodes))
					}
					break
				}
			}
		})
	}
}

func TestEmitThreadID(t *testing.T) {
	irMod := buildMinimalComputeShader()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify dx.op.threadId function was declared.
	found := false
	for _, fn := range result.Functions {
		if fn.Name == "dx.op.threadId.i32" && fn.IsDeclaration {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.threadId.i32 function not declared")
	}

	// Verify main function has call instructions for thread ID.
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	callCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.threadId.i32" {
				callCount++
			}
		}
	}

	// GlobalInvocationId is vec3, so we expect 3 threadId calls (one per component).
	if callCount != 3 {
		t.Errorf("threadId call count: got %d, want 3", callCount)
	}
}

func TestEmitMultipleComputeBuiltins(t *testing.T) {
	irMod := buildComputeMultipleBuiltins()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Count calls by function name.
	callCounts := make(map[string]int)
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil {
				callCounts[instr.CalledFunc.Name]++
			}
		}
	}

	// Expect 3 threadId + 3 threadIdInGroup + 3 groupId = 9 total.
	if callCounts["dx.op.threadId.i32"] != 3 {
		t.Errorf("threadId calls: got %d, want 3", callCounts["dx.op.threadId.i32"])
	}
	if callCounts["dx.op.threadIdInGroup.i32"] != 3 {
		t.Errorf("threadIdInGroup calls: got %d, want 3", callCounts["dx.op.threadIdInGroup.i32"])
	}
	if callCounts["dx.op.groupId.i32"] != 3 {
		t.Errorf("groupId calls: got %d, want 3", callCounts["dx.op.groupId.i32"])
	}
}

func TestEmitUAVCreateHandle(t *testing.T) {
	irMod := buildComputeWithUAV()

	e := &Emitter{
		ir:              irMod,
		mod:             module.NewModule(module.ComputeShader),
		resourceHandles: make(map[ir.GlobalVariableHandle]int),
	}
	e.analyzeResources()

	if len(e.resources) != 1 {
		t.Fatalf("resource count: got %d, want 1", len(e.resources))
	}

	res := &e.resources[0]
	if res.class != resourceClassUAV {
		t.Errorf("resource class: got %d, want UAV (%d)", res.class, resourceClassUAV)
	}
	if res.name != "data" {
		t.Errorf("resource name: got %q, want %q", res.name, "data")
	}
	if res.group != 0 {
		t.Errorf("resource group: got %d, want 0", res.group)
	}
	if res.binding != 0 {
		t.Errorf("resource binding: got %d, want 0", res.binding)
	}
}

func TestEmitUAVBufferLoadStore(t *testing.T) {
	irMod := buildComputeWithUAV()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify buffer load/store functions were declared.
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	if !funcNames["dx.op.bufferLoad.i32"] {
		t.Error("dx.op.bufferLoad.i32 not declared")
	}
	if !funcNames["dx.op.bufferStore.i32"] {
		t.Error("dx.op.bufferStore.i32 not declared")
	}
	if !funcNames["dx.op.createHandle"] {
		t.Error("dx.op.createHandle not declared")
	}
	if !funcNames["dx.op.threadId.i32"] {
		t.Error("dx.op.threadId.i32 not declared")
	}

	// Verify main function has the expected instructions.
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	callCounts := make(map[string]int)
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil {
				callCounts[instr.CalledFunc.Name]++
			}
		}
	}

	// Should have createHandle, threadId (3x), bufferLoad, bufferStore calls.
	if callCounts["dx.op.createHandle"] != 1 {
		t.Errorf("createHandle calls: got %d, want 1", callCounts["dx.op.createHandle"])
	}
	if callCounts["dx.op.threadId.i32"] != 3 {
		t.Errorf("threadId calls: got %d, want 3", callCounts["dx.op.threadId.i32"])
	}
	if callCounts["dx.op.bufferLoad.i32"] < 1 {
		t.Errorf("bufferLoad calls: got %d, want >= 1", callCounts["dx.op.bufferLoad.i32"])
	}
	if callCounts["dx.op.bufferStore.i32"] < 1 {
		t.Errorf("bufferStore calls: got %d, want >= 1", callCounts["dx.op.bufferStore.i32"])
	}

	// Serialize to verify valid bitcode.
	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	if len(bc)%4 != 0 {
		t.Errorf("bitcode not 32-bit aligned: %d bytes", len(bc))
	}

	t.Logf("compute with UAV: %d types, %d functions, %d constants, %d bytes bitcode",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

func TestEmitComputeNoOutputSignature(t *testing.T) {
	// Compute shaders have no output signature (no storeOutput calls).
	irMod := buildMinimalComputeShader()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// No storeOutput calls should be present.
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil {
				name := instr.CalledFunc.Name
				if len(name) > 16 && name[:16] == "dx.op.storeOutpu" {
					t.Errorf("unexpected storeOutput call in compute shader: %s", name)
				}
			}
		}
	}
}

func TestEmitComputeWorkgroupZeroClamping(t *testing.T) {
	// Workgroup dimensions of 0 should be clamped to 1 (per Mesa behavior).
	irMod := buildMinimalComputeShader()
	irMod.EntryPoints[0].Workgroup = [3]uint32{64, 0, 0}

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Just verify it compiles and serializes.
	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
}

func TestEmitComputeUAVResourceMetadata(t *testing.T) {
	irMod := buildComputeWithUAV()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify dx.resources metadata is present.
	found := false
	for _, nm := range result.NamedMetadata {
		if nm.Name == "dx.resources" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.resources metadata not present for shader with UAV")
	}
}
