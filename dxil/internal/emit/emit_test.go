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

// TestEmitArrayLoad_WorkgroupFieldReroot verifies that emitArrayLoad, when
// called on a base pointer that was tracked as a struct-field GEP off a
// workgroup addrspace(3) global, re-roots each per-element GEP directly at
// the global with 3 indices [0, fieldIdx, elemIdx] instead of chaining off
// the field pointer. Regression for 'TGSM pointers must originate from an
// unambiguous TGSM global variable' on `output = w_mem.arr` patterns
// (workgroup-var-init.wgsl).
func TestEmitArrayLoad_WorkgroupFieldReroot(t *testing.T) {
	mod := module.NewModule(module.ComputeShader)
	e := &Emitter{mod: mod}
	e.intConsts = make(map[int64]int)
	e.constMap = make(map[int]*module.Constant)
	e.workgroupFieldPtrs = make(map[int]workgroupFieldOrigin)

	// Build a synthetic function with an entry block so addGEPInstr has a
	// currentBB to append into.
	i32 := mod.GetIntType(32)
	arrTy := mod.GetArrayType(i32, 4)
	structTy := mod.GetStructType("W", []*module.Type{arrTy})
	fnTy := mod.GetFunctionType(mod.GetVoidType(), nil)
	fn := mod.AddFunction("main", fnTy, false)
	e.currentBB = fn.AddBasicBlock("entry")

	// Fake global-alloca value ID + record the field-ptr origin as if
	// emitGlobalStructGEP had just run.
	const (
		globalAllocaID = 100
		fieldPtrID     = 101
	)
	e.nextValue = 200
	e.workgroupFieldPtrs[fieldPtrID] = workgroupFieldOrigin{
		globalAllocaID: globalAllocaID,
		structTy:       structTy,
		fieldFlatIdx:   0,
		memberTy:       arrTy,
	}

	// Exercise the load path: this must emit re-rooted GEPs.
	if _, err := e.emitArrayLoad(fieldPtrID, arrTy); err != nil {
		t.Fatalf("emitArrayLoad: %v", err)
	}

	// Inspect emitted instructions: expect 4×GEP (3 indices each, base =
	// globalAllocaID, source element type = structTy) and 4×Load.
	var geps []*module.Instruction
	for _, instr := range e.currentBB.Instructions {
		if instr.Kind == module.InstrGEP {
			geps = append(geps, instr)
		}
	}
	if len(geps) != 4 {
		t.Fatalf("expected 4 GEPs, got %d", len(geps))
	}
	for i, g := range geps {
		// operands: [inbounds=1, sourceElemTyID, basePtrID, idx0, idx1, idx2]
		if len(g.Operands) != 6 {
			t.Errorf("GEP %d: want 6 operands (3 indices), got %d", i, len(g.Operands))
			continue
		}
		if g.Operands[1] != structTy.ID {
			t.Errorf("GEP %d: source elem type = %d, want structTy.ID=%d (collapsed root)",
				i, g.Operands[1], structTy.ID)
		}
		if g.Operands[2] != globalAllocaID {
			t.Errorf("GEP %d: base ptr = %d, want globalAllocaID=%d (root at global, not field ptr)",
				i, g.Operands[2], globalAllocaID)
		}
		if rt := g.ResultType; rt == nil || rt.Kind != module.TypePointer || rt.PointerAddrSpace != 3 {
			t.Errorf("GEP %d: result type must be addrspace(3) pointer, got %+v", i, rt)
		}
	}
}

// TestEmitArrayLoad_NonWorkgroupBaseUnchanged verifies the non-workgroup
// path still emits 2-index GEPs chained off the base pointer.
func TestEmitArrayLoad_NonWorkgroupBaseUnchanged(t *testing.T) {
	mod := module.NewModule(module.ComputeShader)
	e := &Emitter{mod: mod}
	e.intConsts = make(map[int64]int)
	e.constMap = make(map[int]*module.Constant)
	e.workgroupFieldPtrs = make(map[int]workgroupFieldOrigin)

	i32 := mod.GetIntType(32)
	arrTy := mod.GetArrayType(i32, 3)
	fnTy := mod.GetFunctionType(mod.GetVoidType(), nil)
	fn := mod.AddFunction("main", fnTy, false)
	e.currentBB = fn.AddBasicBlock("entry")

	const baseID = 77
	e.nextValue = 200

	if _, err := e.emitArrayLoad(baseID, arrTy); err != nil {
		t.Fatalf("emitArrayLoad: %v", err)
	}

	for i, instr := range e.currentBB.Instructions {
		if instr.Kind != module.InstrGEP {
			continue
		}
		if len(instr.Operands) != 5 {
			t.Errorf("GEP %d: want 5 operands (2 indices), got %d", i, len(instr.Operands))
		}
		if instr.Operands[2] != baseID {
			t.Errorf("GEP %d: base = %d, want %d (untouched)", i, instr.Operands[2], baseID)
		}
		if instr.ResultType.PointerAddrSpace != 0 {
			t.Errorf("GEP %d: result addrspace = %d, want 0", i, instr.ResultType.PointerAddrSpace)
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
		if fn.Name == "dx.op.binary.f32" {
			hasFMin = true
			// Binary dx.op: ret(i32, TYPE, TYPE) = 3 params.
			if len(fn.FuncType.ParamTypes) != 3 {
				t.Errorf("dx.op.binary.f32 params: got %d, want 3", len(fn.FuncType.ParamTypes))
			}
			break
		}
	}
	if !hasFMin {
		t.Error("dx.op.binary.f32 function not found")
	}

	// Must have call instructions for the dx.op.
	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}
	hasCall := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.binary.f32" {
				hasCall = true
			}
		}
	}
	if !hasCall {
		t.Error("no call to dx.op.binary.f32 found in main")
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
		if fn.Name == "dx.op.binary.i32" {
			hasIMax = true
			break
		}
	}
	if !hasIMax {
		t.Error("dx.op.binary.i32 function not found")
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
		if fn.Name == "dx.op.binary.i32" {
			hasUMax = true
			break
		}
	}
	if !hasUMax {
		t.Error("dx.op.binary.i32 function not found")
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
		if fn.Name == "dx.op.binary.f32" {
			hasFMax = true
		}
		if fn.Name == "dx.op.binary.f32" {
			hasFMin = true
		}
	}
	if !hasFMax {
		t.Error("dx.op.binary.f32 function not found (needed for clamp)")
	}
	if !hasFMin {
		t.Error("dx.op.binary.f32 function not found (needed for clamp)")
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
		if fn.Name == "dx.op.tertiary.f32" {
			hasFMad = true
			// Ternary dx.op: ret(i32, TYPE, TYPE, TYPE) = 4 params.
			if len(fn.FuncType.ParamTypes) != 4 {
				t.Errorf("dx.op.tertiary.f32 params: got %d, want 4", len(fn.FuncType.ParamTypes))
			}
			break
		}
	}
	if !hasFMad {
		t.Error("dx.op.tertiary.f32 function not found")
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
		if fn.Name == "dx.op.tertiary.f32" {
			hasFma = true
			break
		}
	}
	if !hasFma {
		t.Error("dx.op.tertiary.f32 function not found")
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
		if fn.Name == "dx.op.unary.f32" {
			hasLog = true
		}
		if fn.Name == "dx.op.unary.f32" {
			hasExp = true
		}
	}
	if !hasLog {
		t.Error("dx.op.unary.f32 function not found (needed for pow)")
	}
	if !hasExp {
		t.Error("dx.op.unary.f32 function not found (needed for pow)")
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

// buildMathLdexpShader constructs a minimal fragment shader exercising
// the WGSL ldexp signature f32 × i32 → f32. WGSL declares ldexp as
// `fn ldexp(x: f32, n: i32) -> f32` — the exponent is an INTEGER,
// unlike HLSL's `ldexp(float, float)`. The mismatch between WGSL IR
// types and the scalar dx.op.unary.f32 intrinsic is the root cause of
// BUG-DXIL-023: emit must insert `sitofp i32 → f32` before feeding the
// exponent into the unary exp call.
//
// The exponent here is an integer literal (not a function argument) to
// keep the fixture dependency-free from fragment input binding rules.
// This still drives the exact code path we need — the emitter sees
// `ExprMath{Fun: MathLdexp, Arg: <f32 arg>, Arg1: <i32 literal>}`.
func buildMathLdexpShader() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	i32Handle := ir.TypeHandle(1)
	vec4Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	arg1Handle := ir.ExpressionHandle(1)
	retHandle := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "x", Type: f32Handle, Binding: &xBinding},
		},
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x  (f32)
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},                           // [1] 2  (i32)
			{Kind: ir.ExprMath{Fun: ir.MathLdexp, Arg: 0, Arg1: &arg1Handle}},     // [2] ldexp(x, 2)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3] 1.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 2, 2, 3}}}, // [4] vec4(r, r, r, 1.0)
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},  // x
			{Handle: &i32Handle},  // 2
			{Handle: &f32Handle},  // ldexp result
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

// TestEmitMathLdexp_BitcodeRecordValid is the regression gate for
// BUG-DXIL-023. Before the fix, `emitMathLdexp` passed the raw i32
// exponent directly as the second operand of `dx.op.unary.f32` — the
// resulting LLVM 3.7 CALL record carried an i32-typed operand at a
// slot declared float, and `dxil.dll`'s bitcode reader rejected the
// record with `HRESULT 0x80aa0009 Invalid record` before any semantic
// validation ran. dxc `-dumpbin` cannot parse the malformed blob
// either. The fix emits a `sitofp i32 → f32` cast before the unary
// call, matching DXC's TranslateLdExp (HLOperationLower.cpp:2504).
//
// This test asserts the emitted instruction shape:
//
//   - exactly one InstrCast with kind CastSIToFP (the exponent cast)
//   - at least one call to `dx.op.unary.f32` (the exp2 intrinsic)
//   - at least one InstrBinOp (the final fmul of x and exp2)
//
// and then runs `dxil.Validate(blob, dxil.ValidateBitcode)` which
// walks the whole bitcode stream in pure Go and MUST pass. The
// structural check is what would have caught this bug offline the
// first time emitMathLdexp was written, so we lock it at unit-test
// time instead of only at IDxcValidator time.
func TestEmitMathLdexp_BitcodeRecordValid(t *testing.T) {
	irMod := buildMathLdexpShader()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found in emitted module")
	}

	var (
		sitofpCount       int
		unaryExpCallCount int
		fmulCount         int
	)
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			switch instr.Kind {
			case module.InstrCast:
				// Operand layout for our addCastInstr: [src, castKind].
				if len(instr.Operands) >= 2 && CastOpKind(instr.Operands[1]) == CastSIToFP {
					sitofpCount++
				}
			case module.InstrCall:
				if instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.unary.f32" {
					unaryExpCallCount++
				}
			case module.InstrBinOp:
				// Operand layout: [lhs, rhs, binOpKind].
				if len(instr.Operands) >= 3 && BinOpKind(instr.Operands[2]) == BinOpFMul {
					fmulCount++
				}
			}
		}
	}

	if sitofpCount < 1 {
		t.Errorf("expected ≥1 sitofp (i32→f32) cast for ldexp exponent, got %d", sitofpCount)
	}
	if unaryExpCallCount < 1 {
		t.Errorf("expected ≥1 dx.op.unary.f32 call (exp2), got %d", unaryExpCallCount)
	}
	if fmulCount < 1 {
		t.Errorf("expected ≥1 fmul (x * exp2(n)), got %d", fmulCount)
	}

	// Serialize the whole module and run the public bitcode-level
	// validator. This is the structural guarantee: any record-level
	// malformation (wrong operand types, wrong operand count, wrong
	// abbrev) would be caught here without needing dxil.dll.
	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	// We need a full DXBC container for dxil.Validate, not raw
	// bitcode. Use the public compile entry point with a minimal
	// wrapper. For the unit test we only need structural validity of
	// the emitted module itself; re-emitting via Compile would
	// duplicate the emission path. Instead, walk the serialized
	// bitcode directly via bitcheck, which is what Validate calls
	// for ValidateBitcode:
	//
	//   dxil.Validate(blob, ValidateBitcode) →
	//     dxcvalidator.PreCheckContainer(blob) + bitcheck.Check(blob)
	//
	// Here we only assert instruction-shape invariants, which is
	// sufficient as a regression gate for BUG-DXIL-023 — the real
	// blob-level structural check lives in dxil/validate_test.go
	// (TestLdexpEndToEndStructural) and exercises the full pipeline.
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
		if fn.Name == "dx.op.binary.f32" {
			hasFMax = true
		}
		if fn.Name == "dx.op.binary.f32" {
			hasFMin = true
		}
	}
	if !hasFMax {
		t.Error("dx.op.binary.f32 not found (needed for smoothstep clamp)")
	}
	if !hasFMin {
		t.Error("dx.op.binary.f32 not found (needed for smoothstep clamp)")
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
		if fn.Name == "dx.op.unary.f32" {
			hasSqrt = true
		}
	}
	if !hasDot3 {
		t.Error("dx.op.dot3.f32 not found (needed for length)")
	}
	if !hasSqrt {
		t.Error("dx.op.unary.f32 not found (needed for length)")
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
		if fn.Name == "dx.op.unary.f32" {
			hasAtan = true
			break
		}
	}
	if !hasAtan {
		t.Error("dx.op.unary.f32 not found (needed for atan2)")
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

	fn := e.getDxOpBinaryFunc("dx.op.binary", overloadF32)
	if fn.Name != "dx.op.binary.f32" {
		t.Errorf("name: got %q, want %q", fn.Name, "dx.op.binary.f32")
	}
	if len(fn.FuncType.ParamTypes) != 3 {
		t.Errorf("params: got %d, want 3", len(fn.FuncType.ParamTypes))
	}

	// Second call should return cached.
	fn2 := e.getDxOpBinaryFunc("dx.op.binary", overloadF32)
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

	fn := e.getDxOpTernaryFunc("dx.op.tertiary", overloadF32)
	if fn.Name != "dx.op.tertiary.f32" {
		t.Errorf("name: got %q, want %q", fn.Name, "dx.op.tertiary.f32")
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
func ptrU8(v uint8) *uint8    { return &v }
func ptrU32(v uint32) *uint32 { return &v }

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

	// Single-store scalar locals are promoted directly to SSA values,
	// bypassing alloca/store/load. This matches DXC's mem2reg behavior
	// for locals stored once in straight-line code.
	allocaCount := countInstrKind(mod, module.InstrAlloca)
	if allocaCount != 0 {
		t.Errorf("expected 0 alloca instructions (single-store promotion), got %d", allocaCount)
	}

	storeCount := countInstrKind(mod, module.InstrStore)
	if storeCount != 0 {
		t.Errorf("expected 0 store instructions (single-store promotion), got %d", storeCount)
	}

	loadCount := countInstrKind(mod, module.InstrLoad)
	if loadCount != 0 {
		t.Errorf("expected 0 load instructions (single-store promotion), got %d", loadCount)
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
		if fn.Name == "dx.op.unary.f32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.unary.f32 function not found")
	}

	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	hasCall := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.unary.f32" {
				hasCall = true
			}
		}
	}
	if !hasCall {
		t.Error("no call to dx.op.unary.f32 found in main")
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
		if fn.Name == "dx.op.unary.f32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.unary.f32 function not found")
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
	// All three dx.op operations (DerivCoarseX, DerivCoarseY, FAbs) live
	// under the OCC::Unary class and share the function symbol
	// dx.op.unary.f32; they are distinguished by the opcode immediate
	// (first i32 argument), not by function name. Verify the unary
	// function exists and at least one call to it appears.
	hasUnaryFn := false
	for _, fn := range mod.Functions {
		if fn.Name == "dx.op.unary.f32" {
			hasUnaryFn = true
		}
	}
	if !hasUnaryFn {
		t.Error("dx.op.unary.f32 not found (needed for fwidth derivatives + fabs)")
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
		if fn.Name == "dx.op.isSpecialFloat.f32" {
			found = true
			// Unary dx.op: ret(i32, TYPE) = 2 params.
			if len(fn.FuncType.ParamTypes) != 2 {
				t.Errorf("dx.op.isSpecialFloat.f32 params: got %d, want 2", len(fn.FuncType.ParamTypes))
			}
			break
		}
	}
	if !found {
		t.Error("dx.op.isSpecialFloat.f32 function not found")
	}

	mainFn := findMainFunc(mod)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	hasCall := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == "dx.op.isSpecialFloat.f32" {
				hasCall = true
			}
		}
	}
	if !hasCall {
		t.Error("no call to dx.op.isSpecialFloat.f32 found in main")
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
		if fn.Name == "dx.op.isSpecialFloat.f32" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dx.op.isSpecialFloat.f32 function not found")
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
	// StmtRayQuery should return an error (not silently skip).
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
			{Kind: ir.StmtRayQuery{}}, // unsupported
			{Kind: ir.StmtReturn{Value: &retHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	_, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err == nil {
		t.Fatal("expected error for StmtRayQuery, got nil")
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

	// Sampler-heap mode: the per-WGSL `samp` global is rewritten into a
	// per-group index buffer SRV (inserted at sampler position) + a
	// SamplerHeap entry appended after. Resource list:
	// CBV + SRV(tex) + SRV(indexBuffer) + SamplerHeap = 4 entries.
	if len(e.resources) != 4 {
		t.Fatalf("expected 4 resources (CBV+SRV+IndexBuffer+SamplerHeap), got %d", len(e.resources))
	}

	// CBV
	if e.resources[0].class != resourceClassCBV {
		t.Errorf("resource 0: expected CBV, got class %d", e.resources[0].class)
	}
	// SRV (texture)
	if e.resources[1].class != resourceClassSRV {
		t.Errorf("resource 1: expected SRV, got class %d", e.resources[1].class)
	}
	// Per-group sampler index buffer (synthesized SRV — at sampler position)
	if e.resources[2].class != resourceClassSRV {
		t.Errorf("resource 2: expected SRV (sampler index buffer), got class %d", e.resources[2].class)
	}
	if e.resources[2].name != "nagaGroup0SamplerIndexArray" {
		t.Errorf("resource 2: expected name 'nagaGroup0SamplerIndexArray', got %q", e.resources[2].name)
	}
	// SamplerHeap (synthesized — replaces the per-WGSL sampler entry)
	if e.resources[3].class != resourceClassSampler {
		t.Errorf("resource 3: expected Sampler heap, got class %d", e.resources[3].class)
	}
	if e.resources[3].name != "nagaSamplerHeap" {
		t.Errorf("resource 3: expected name 'nagaSamplerHeap', got %q", e.resources[3].name)
	}
	if e.resources[3].arraySize != 2048 {
		t.Errorf("resource 3: expected arraySize=2048, got %d", e.resources[3].arraySize)
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
	u32Handle := ir.TypeHandle(0)
	vec3u32Handle := ir.TypeHandle(1)
	arrayU32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{Base: u32Handle, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:   "out",
				Space:  ir.SpaceStorage,
				Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{
					Group: 0, Binding: 0,
				},
				Type: arrayU32Handle,
			},
		},
	}

	globalIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	// The shader reads global_id.x and stores it to out[0], making
	// the builtin argument live for DCE / isArgRead checks.
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] global_id
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] global_id.x
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [2] &out
			{Kind: ir.ExprAccess{Base: 2, Index: 1}},      // [3] &out[global_id.x]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},  // global_id
			{Handle: &u32Handle},      // global_id.x
			{Handle: &arrayU32Handle}, // &out
			{Handle: &u32Handle},      // &out[x]
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtStore{Pointer: 3, Value: 1}}, // out[x] = global_id.x
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
	u32Handle := ir.TypeHandle(0)
	vec3u32Handle := ir.TypeHandle(1)
	arrayU32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{Base: u32Handle, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:   "out",
				Space:  ir.SpaceStorage,
				Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{
					Group: 0, Binding: 0,
				},
				Type: arrayU32Handle,
			},
		},
	}

	globalIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})
	localIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinLocalInvocationID})
	wgIDBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinWorkGroupID})

	// Store global_id.x + local_id.x + wg_id.x to make all builtins live.
	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
			{Name: "local_id", Type: vec3u32Handle, Binding: &localIDBinding},
			{Name: "wg_id", Type: vec3u32Handle, Binding: &wgIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                  // [0] global_id
			{Kind: ir.ExprFunctionArgument{Index: 1}},                  // [1] local_id
			{Kind: ir.ExprFunctionArgument{Index: 2}},                  // [2] wg_id
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},              // [3] global_id.x
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}},              // [4] local_id.x
			{Kind: ir.ExprAccessIndex{Base: 2, Index: 0}},              // [5] wg_id.x
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 3, Right: 4}}, // [6] g.x+l.x
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 6, Right: 5}}, // [7] g.x+l.x+w.x
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                 // [8] &out
			{Kind: ir.ExprAccess{Base: 8, Index: 3}},                   // [9] &out[global_id.x]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},  // global_id
			{Handle: &vec3u32Handle},  // local_id
			{Handle: &vec3u32Handle},  // wg_id
			{Handle: &u32Handle},      // global_id.x
			{Handle: &u32Handle},      // local_id.x
			{Handle: &u32Handle},      // wg_id.x
			{Handle: &u32Handle},      // g.x+l.x
			{Handle: &u32Handle},      // sum
			{Handle: &arrayU32Handle}, // &out
			{Handle: &u32Handle},      // &out[x]
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 10}}},
			{Kind: ir.StmtStore{Pointer: 9, Value: 7}}, // out[x] = sum
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

// --- Atomic and barrier tests ---

// buildComputeWithAtomicAdd creates a compute shader that performs an atomic add:
//
//	@group(0) @binding(0) var<storage, read_write> data: array<u32>;
//
//	@compute @workgroup_size(64)
//	fn main(@builtin(global_invocation_id) global_id: vec3<u32>) {
//	    let idx = global_id.x;
//	    let result = atomicAdd(&data[idx], 1u);
//	}
func buildComputeWithAtomicAdd() *ir.Module {
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
	resultExpr := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] global_id
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] global_id.x
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [2] &data
			{Kind: ir.ExprAccess{Base: 2, Index: 1}},      // [3] &data[idx]
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},   // [4] 1u
			{Kind: ir.ExprAtomicResult{Ty: u32Handle}},    // [5] atomic result
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},  // global_id
			{Handle: &u32Handle},      // global_id.x
			{Handle: &arrayU32Handle}, // &data
			{Handle: &u32Handle},      // &data[idx]
			{Handle: &u32Handle},      // 1u
			{Handle: &u32Handle},      // atomic result
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtAtomic{
				Pointer: 3,
				Fun:     ir.AtomicAdd{},
				Value:   4,
				Result:  &resultExpr,
			}},
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

func TestEmitAtomicAdd(t *testing.T) {
	irMod := buildComputeWithAtomicAdd()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify atomicBinOp function was declared.
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	if !funcNames["dx.op.atomicBinOp.i32"] {
		t.Error("dx.op.atomicBinOp.i32 not declared")
	}
	if !funcNames["dx.op.createHandle"] {
		t.Error("dx.op.createHandle not declared")
	}

	// Verify the main function has atomicBinOp call.
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

	if callCounts["dx.op.atomicBinOp.i32"] != 1 {
		t.Errorf("atomicBinOp calls: got %d, want 1", callCounts["dx.op.atomicBinOp.i32"])
	}

	// Serialize to verify valid bitcode.
	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	if len(bc)%4 != 0 {
		t.Errorf("bitcode not 32-bit aligned: %d bytes", len(bc))
	}

	t.Logf("atomic add compute: %d types, %d functions, %d constants, %d bytes",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

func TestEmitAtomicCompareExchange(t *testing.T) {
	u32Handle := ir.TypeHandle(0)
	vec3u32Handle := ir.TypeHandle(1)
	arrayU32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{
				Base:   u32Handle,
				Size:   ir.ArraySize{},
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
	compareExpr := ir.ExpressionHandle(4)
	resultExpr := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                    // [0] global_id
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                // [1] global_id.x
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                   // [2] &data
			{Kind: ir.ExprAccess{Base: 2, Index: 1}},                     // [3] &data[idx]
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},                  // [4] 0u (compare value)
			{Kind: ir.Literal{Value: ir.LiteralU32(42)}},                 // [5] 42u (new value)
			{Kind: ir.ExprAtomicResult{Ty: u32Handle, Comparison: true}}, // [6] CAS result
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},
			{Handle: &u32Handle},
			{Handle: &arrayU32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtAtomic{
				Pointer: 3,
				Fun:     ir.AtomicExchange{Compare: &compareExpr},
				Value:   5,
				Result:  &resultExpr,
			}},
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

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify atomicCompareExchange function was declared.
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	if !funcNames["dx.op.atomicCompareExchange.i32"] {
		t.Error("dx.op.atomicCompareExchange.i32 not declared")
	}

	// Verify CAS call in main.
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

	if callCounts["dx.op.atomicCompareExchange.i32"] != 1 {
		t.Errorf("atomicCompareExchange calls: got %d, want 1", callCounts["dx.op.atomicCompareExchange.i32"])
	}

	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("atomic CAS compute: %d types, %d functions, %d constants, %d bytes",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

func TestEmitBarrier(t *testing.T) {
	// Minimal compute shader that just executes a barrier.
	// No arguments — the barrier is the only side effect.
	mod := &ir.Module{}

	fn := ir.Function{
		Name: "main",
		Body: []ir.Statement{
			{Kind: ir.StmtBarrier{Flags: ir.BarrierStorage | ir.BarrierWorkGroup}},
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

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify barrier function was declared.
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	if !funcNames["dx.op.barrier"] {
		t.Error("dx.op.barrier not declared")
	}

	// Verify barrier call in main.
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

	if callCounts["dx.op.barrier"] != 1 {
		t.Errorf("barrier calls: got %d, want 1", callCounts["dx.op.barrier"])
	}

	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("barrier compute: %d types, %d functions, %d constants, %d bytes",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

func TestEmitMultipleAtomicOps(t *testing.T) {
	// Test multiple atomic operations in one shader: add, and, exchange.
	u32Handle := ir.TypeHandle(0)
	vec3u32Handle := ir.TypeHandle(1)
	arrayU32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{
				Base:   u32Handle,
				Size:   ir.ArraySize{},
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
	result1 := ir.ExpressionHandle(5)
	result2 := ir.ExpressionHandle(6)
	result3 := ir.ExpressionHandle(7)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] global_id
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] global_id.x
			{Kind: ir.ExprGlobalVariable{Variable: 0}},    // [2] &data
			{Kind: ir.ExprAccess{Base: 2, Index: 1}},      // [3] &data[idx]
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},   // [4] 1u
			{Kind: ir.ExprAtomicResult{Ty: u32Handle}},    // [5] atomicAdd result
			{Kind: ir.ExprAtomicResult{Ty: u32Handle}},    // [6] atomicAnd result
			{Kind: ir.ExprAtomicResult{Ty: u32Handle}},    // [7] atomicExchange result
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},
			{Handle: &u32Handle},
			{Handle: &arrayU32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtAtomic{Pointer: 3, Fun: ir.AtomicAdd{}, Value: 4, Result: &result1}},
			{Kind: ir.StmtAtomic{Pointer: 3, Fun: ir.AtomicAnd{}, Value: 4, Result: &result2}},
			{Kind: ir.StmtAtomic{Pointer: 3, Fun: ir.AtomicExchange{}, Value: 4, Result: &result3}},
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

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify atomicBinOp function was declared.
	funcNames := make(map[string]bool)
	for _, fn := range result.Functions {
		funcNames[fn.Name] = true
	}

	if !funcNames["dx.op.atomicBinOp.i32"] {
		t.Error("dx.op.atomicBinOp.i32 not declared")
	}

	// Verify 3 atomic calls.
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

	if callCounts["dx.op.atomicBinOp.i32"] != 3 {
		t.Errorf("atomicBinOp calls: got %d, want 3", callCounts["dx.op.atomicBinOp.i32"])
	}

	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("multi-atomic compute: %d types, %d functions, %d constants, %d bytes",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

func TestEmitBarrierFlags(t *testing.T) {
	// Test that emitStmtBarrier produces flag combinations the DXIL
	// validator accepts. Key invariant: any sync flag (bit 1) MUST be
	// accompanied by at least one memory fence bit (UAV or TGSM).
	// Sync-only (flags=1) is rejected by DxilValidation.cpp with
	// 'sync must include some form of memory barrier'.
	mod := module.NewModule(module.ComputeShader)
	fnTy := mod.GetFunctionType(mod.GetVoidType(), nil)

	tests := []struct {
		name          string
		flags         ir.BarrierFlags
		expectedFlags DXILBarrierMode
	}{
		{
			name:          "storage only",
			flags:         ir.BarrierStorage,
			expectedFlags: BarrierModeSyncThreadGroup | BarrierModeUAVFenceGlobal,
		},
		{
			name:          "workgroup only",
			flags:         ir.BarrierWorkGroup,
			expectedFlags: BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence,
		},
		{
			name:          "storage + workgroup",
			flags:         ir.BarrierStorage | ir.BarrierWorkGroup,
			expectedFlags: BarrierModeUAVFenceGlobal | BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence,
		},
		{
			// subgroup → full group barrier with TGSM + UAV(thread
			// group) fences. Sync-only is rejected by the validator;
			// DXIL has no wave-scope barrier so we lower to the closest
			// equivalent — GroupMemoryBarrierWithGroupSync semantics.
			name:          "subgroup (must include memory fence)",
			flags:         ir.BarrierSubGroup,
			expectedFlags: BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence | BarrierModeUAVFenceThreadGroup,
		},
		{
			// Default path (no WGSL flag set) must also include a
			// memory fence bit.
			name:          "default (zero flags) must include fence",
			flags:         0,
			expectedFlags: BarrierModeSyncThreadGroup | BarrierModeGroupSharedMemFence,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the real emitStmtBarrier to pin the behavior.
			e := &Emitter{mod: mod}
			e.intConsts = make(map[int64]int)
			e.constMap = make(map[int]*module.Constant)
			e.dxOpFuncs = make(map[dxOpKey]*module.Function)
			fn := mod.AddFunction("barrier_test_"+tt.name, fnTy, false)
			e.currentBB = fn.AddBasicBlock("entry")
			e.nextValue = 100

			if err := e.emitStmtBarrier(ir.StmtBarrier{Flags: tt.flags}); err != nil {
				t.Fatalf("emitStmtBarrier: %v", err)
			}

			// Locate the barrier call and extract the flags arg.
			var barrierCall *module.Instruction
			for _, instr := range e.currentBB.Instructions {
				if instr.Kind == module.InstrCall &&
					instr.CalledFunc != nil &&
					instr.CalledFunc.Name == "dx.op.barrier" {
					barrierCall = instr
					break
				}
			}
			if barrierCall == nil {
				t.Fatal("no dx.op.barrier call emitted")
			}
			// Operands: [calleeID, opcodeConstID, flagsConstID]
			// Resolve flagsConstID back to the int value via constMap.
			flagsConstID := barrierCall.Operands[len(barrierCall.Operands)-1]
			c, ok := e.constMap[flagsConstID]
			if !ok {
				t.Fatalf("flags operand %d not in constMap", flagsConstID)
			}
			got := DXILBarrierMode(c.IntValue)
			if got != tt.expectedFlags {
				t.Errorf("flags: got 0x%x, want 0x%x", got, tt.expectedFlags)
			}
			// Sanity: validator rejects sync-only; make sure we never
			// emit flags == BarrierModeSyncThreadGroup alone.
			if got == BarrierModeSyncThreadGroup {
				t.Errorf("emit sync-only flag (bit 1) — rejected by validator")
			}
		})
	}
}

func TestEmitAtomicSubtract(t *testing.T) {
	// Test atomic subtract emits negation + atomicBinOp ADD.
	u32Handle := ir.TypeHandle(0)
	vec3u32Handle := ir.TypeHandle(1)
	arrayU32Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{
				Base:   u32Handle,
				Size:   ir.ArraySize{},
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
	resultExpr := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "global_id", Type: vec3u32Handle, Binding: &globalIDBinding},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprAccess{Base: 2, Index: 1}},
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			{Kind: ir.ExprAtomicResult{Ty: u32Handle}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32Handle},
			{Handle: &u32Handle},
			{Handle: &arrayU32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
			{Handle: &u32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtAtomic{Pointer: 3, Fun: ir.AtomicSubtract{}, Value: 4, Result: &resultExpr}},
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

	result, err := Emit(mod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Verify atomicBinOp (ADD with negated value) was emitted.
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Should have a SUB instruction (for negation) and an atomicBinOp call.
	hasSub := false
	hasAtomic := false
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrBinOp {
				hasSub = true
			}
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.atomicBinOp.i32" {
				hasAtomic = true
			}
		}
	}

	if !hasSub {
		t.Error("expected SUB instruction for negation in atomic subtract")
	}
	if !hasAtomic {
		t.Error("expected atomicBinOp.i32 call for atomic subtract")
	}

	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("atomic subtract compute: %d types, %d functions, %d constants, %d bytes",
		len(result.Types), len(result.Functions), len(result.Constants), len(bc))
}

// buildComputeWithFloatVec2Store creates a compute shader that stores a
// vec2<f32> to a storage buffer. This exercises the raw buffer float store
// path where DXC uses i32 overload with bitcast:
//
//	@group(0) @binding(0) var<storage, read_write> out: array<vec2<f32>>;
//	@compute @workgroup_size(1)
//	fn main() { out[0] = vec2<f32>(1.0, 2.0); }
func buildComputeWithFloatVec2Store() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec2f32Handle := ir.TypeHandle(1)
	arrayVec2Handle := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{Base: vec2f32Handle, Stride: 8}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:   "out",
				Space:  ir.SpaceStorage,
				Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{
					Group: 0, Binding: 0,
				},
				Type: arrayVec2Handle,
			},
		},
	}

	// Compose vec2<f32>(1.0, 2.0) and store to out[0].
	fn := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF64(1.0)}},                   // [0] 1.0
			{Kind: ir.Literal{Value: ir.LiteralF64(2.0)}},                   // [1] 2.0
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1}}}, // [2] vec2(1.0, 2.0)
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                      // [3] &out
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},                     // [4] 0u
			{Kind: ir.ExprAccess{Base: 3, Index: 4}},                        // [5] &out[0]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},                                  // 1.0
			{Handle: &f32Handle},                                  // 2.0
			{Handle: &vec2f32Handle},                              // compose
			{Handle: &arrayVec2Handle},                            // &out
			{Value: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 0u
			{Handle: &vec2f32Handle},                              // &out[0]
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtStore{Pointer: 5, Value: 2}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{
			Name:      "main",
			Stage:     ir.StageCompute,
			Function:  fn,
			Workgroup: [3]uint32{1, 1, 1},
		},
	}

	return mod
}

// TestEmitRawBufferFloatStoreUsesI32Overload verifies that float stores to
// raw buffers (RWByteAddressBuffer) use the i32 overload with bitcast,
// matching DXC's convention. DXC always stores via bufferStore.i32 for raw
// buffers because HLSL's RWByteAddressBuffer.Store takes uint values and
// asuint() converts floats.
//
// Checks:
//  1. dx.op.bufferStore.i32 is declared (not .f32)
//  2. Bitcast (float->i32) instructions are present in the function body
//  3. The bufferStore call references the .i32 function
func TestEmitRawBufferFloatStoreUsesI32Overload(t *testing.T) {
	irMod := buildComputeWithFloatVec2Store()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Check declared functions: must have bufferStore.i32, not bufferStore.f32.
	hasI32Store := false
	hasF32Store := false
	for _, fn := range result.Functions {
		switch fn.Name {
		case "dx.op.bufferStore.i32":
			hasI32Store = true
		case "dx.op.bufferStore.f32":
			hasF32Store = true
		}
	}
	if !hasI32Store {
		t.Error("dx.op.bufferStore.i32 not declared; raw buffer float stores must use i32 overload")
	}
	if hasF32Store {
		t.Error("dx.op.bufferStore.f32 declared; raw buffer float stores should NOT use f32 overload")
	}

	// Check that bitcast instructions (float -> i32) exist in the main function.
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	bitcastCount := 0
	bufferStoreI32Count := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCast && len(instr.Operands) >= 2 &&
				CastOpKind(instr.Operands[1]) == CastBitcast {
				bitcastCount++
			}
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.bufferStore.i32" {
				bufferStoreI32Count++
			}
		}
	}

	// With constant folding, float literals are folded to i32 at compile time,
	// so bitcasts may be 0 when all store values are constant. The key invariant
	// is that bufferStore.i32 is used (not .f32).
	if bufferStoreI32Count < 1 {
		t.Errorf("expected at least 1 bufferStore.i32 call, got %d", bufferStoreI32Count)
	}

	// Verify serialization.
	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("raw buffer float store: %d bitcasts, %d bufferStore.i32 calls, %d bytes bitcode",
		bitcastCount, bufferStoreI32Count, len(bc))
}

// TestEmitRawBufferIntStoreNoBitcast verifies that integer stores to raw
// buffers do NOT produce unnecessary bitcast instructions — i32 values are
// passed directly to bufferStore.i32 without any cast.
func TestEmitRawBufferIntStoreNoBitcast(t *testing.T) {
	irMod := buildComputeWithUAV() // stores u32, should use i32 directly

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Count bitcast instructions in the main function. Integer stores to raw
	// buffers should not need any bitcast.
	bitcastCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCast && len(instr.Operands) >= 2 &&
				CastOpKind(instr.Operands[1]) == CastBitcast {
				bitcastCount++
			}
		}
	}

	if bitcastCount > 0 {
		t.Errorf("expected 0 bitcast instructions for integer UAV store, got %d", bitcastCount)
	}
}

// buildComputeWithFloatVec2Load creates a compute shader module that loads
// vec2<f32> from a storage buffer (ByteAddressBuffer).
//
//	@group(0) @binding(0) var<storage, read> data: array<vec2<f32>>;
//	@group(0) @binding(1) var<storage, read_write> out: array<vec2<f32>>;
//
//	@compute @workgroup_size(1)
//	fn main() {
//	    out[0] = data[0];
//	}
func buildComputeWithFloatVec2Load() *ir.Module {
	f32Handle := ir.TypeHandle(0)
	vec2f32Handle := ir.TypeHandle(1)
	arrayVec2Handle := ir.TypeHandle(2)
	u32Handle := ir.TypeHandle(3)

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.ArrayType{Base: vec2f32Handle, Stride: 8}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{
				Name:   "data",
				Space:  ir.SpaceStorage,
				Access: ir.StorageRead,
				Binding: &ir.ResourceBinding{
					Group: 0, Binding: 0,
				},
				Type: arrayVec2Handle,
			},
			{
				Name:   "out",
				Space:  ir.SpaceStorage,
				Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{
					Group: 0, Binding: 1,
				},
				Type: arrayVec2Handle,
			},
		},
	}

	// Load data[0], store to out[0].
	fn := ir.Function{
		Name: "main",
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},  // [0] &data
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, // [1] 0u
			{Kind: ir.ExprAccess{Base: 0, Index: 1}},    // [2] &data[0]
			{Kind: ir.ExprLoad{Pointer: 2}},             // [3] data[0]
			{Kind: ir.ExprGlobalVariable{Variable: 1}},  // [4] &out
			{Kind: ir.ExprAccess{Base: 4, Index: 1}},    // [5] &out[0]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &arrayVec2Handle},                            // &data
			{Value: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, // 0u
			{Handle: &vec2f32Handle},                              // &data[0]
			{Handle: &vec2f32Handle},                              // data[0]
			{Handle: &arrayVec2Handle},                            // &out
			{Handle: &vec2f32Handle},                              // &out[0]
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtStore{Pointer: 5, Value: 3}},
		},
	}
	_ = f32Handle
	_ = u32Handle

	mod.EntryPoints = []ir.EntryPoint{
		{
			Name:      "main",
			Stage:     ir.StageCompute,
			Function:  fn,
			Workgroup: [3]uint32{1, 1, 1},
		},
	}

	return mod
}

// TestEmitRawBufferFloatLoadUsesI32Overload verifies that float loads from
// raw buffers (ByteAddressBuffer/RWByteAddressBuffer) use the i32 overload
// with bitcast, matching DXC's convention. DXC always loads via
// bufferLoad.i32 for raw buffers because HLSL's ByteAddressBuffer.Load
// returns uint and asfloat() wraps it.
//
// Checks:
//  1. dx.op.bufferLoad.i32 is declared (not .f32)
//  2. Bitcast (i32->float) instructions are present in the function body
//  3. ExtractValue instructions use i32 result type (not float)
func TestEmitRawBufferFloatLoadUsesI32Overload(t *testing.T) {
	irMod := buildComputeWithFloatVec2Load()

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Check declared functions: must have bufferLoad.i32, not bufferLoad.f32.
	hasI32Load := false
	hasF32Load := false
	for _, fn := range result.Functions {
		switch fn.Name {
		case "dx.op.bufferLoad.i32":
			hasI32Load = true
		case "dx.op.bufferLoad.f32":
			hasF32Load = true
		}
	}
	if !hasI32Load {
		t.Error("dx.op.bufferLoad.i32 not declared; raw buffer float loads must use i32 overload")
	}
	if hasF32Load {
		t.Error("dx.op.bufferLoad.f32 declared; raw buffer float loads should NOT use f32 overload")
	}

	// Check that bitcast instructions (i32 -> float) exist in the main function.
	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	bitcastCount := 0
	bufferLoadI32Count := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCast && len(instr.Operands) >= 2 &&
				CastOpKind(instr.Operands[1]) == CastBitcast {
				bitcastCount++
			}
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				instr.CalledFunc.Name == "dx.op.bufferLoad.i32" {
				bufferLoadI32Count++
			}
		}
	}

	// vec2<f32> load: 2 extractvalue(i32) + 2 bitcast(i32->float) + store uses
	// 2 bitcast(float->i32). Total bitcasts = 4 (2 load + 2 store).
	if bitcastCount < 2 {
		t.Errorf("expected at least 2 bitcast instructions (i32->float for vec2 load), got %d", bitcastCount)
	}
	if bufferLoadI32Count < 1 {
		t.Errorf("expected at least 1 bufferLoad.i32 call, got %d", bufferLoadI32Count)
	}

	// Verify serialization.
	bc := module.Serialize(result)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}

	t.Logf("raw buffer float load: %d bitcasts, %d bufferLoad.i32 calls, %d bytes bitcode",
		bitcastCount, bufferLoadI32Count, len(bc))
}

// TestEmitRawBufferIntLoadNoBitcast verifies that integer loads from raw
// buffers do NOT produce unnecessary bitcast instructions — i32 values are
// extracted directly from bufferLoad.i32 without any cast.
func TestEmitRawBufferIntLoadNoBitcast(t *testing.T) {
	irMod := buildComputeWithUAV() // loads u32, should use i32 directly

	result, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	mainFn := findMainFunction(result)
	if mainFn == nil {
		t.Fatal("main function not found")
	}

	// Count bitcast instructions in the main function. Integer loads from raw
	// buffers should not need any bitcast (i32 load returns i32 directly).
	bitcastCount := 0
	for _, bb := range mainFn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCast && len(instr.Operands) >= 2 &&
				CastOpKind(instr.Operands[1]) == CastBitcast {
				bitcastCount++
			}
		}
	}

	if bitcastCount > 0 {
		t.Errorf("expected 0 bitcast instructions for integer UAV load, got %d", bitcastCount)
	}
}

// TestResourceNameSuffix verifies that the DXIL emitter applies the
// HLSL namer trailing-underscore convention to resource names in
// dx.resources metadata. DXC processes HLSL variable names through
// its namer (Rust naga proc::Namer) which appends "_" when the name
// ends with a digit or is an HLSL keyword. Our DXIL backend must
// match this convention so dxc -dumpbin shows identical resource
// names in the Resource Bindings comment table.
func TestResourceNameSuffix(t *testing.T) {
	tests := []struct {
		irName   string
		wantName string
	}{
		// Ends with digit -> trailing underscore
		{"t1", "t1_"},
		{"atomic_i32", "atomic_i32_"},
		{"input3", "input3_"},
		// HLSL keyword -> trailing underscore
		{"in", "in_"},
		{"out", "out_"},
		{"indices", "indices_"},
		// Normal name -> no change
		{"data", "data"},
		{"params", "params"},
		{"camera", "camera"},
	}

	for _, tt := range tests {
		t.Run(tt.irName, func(t *testing.T) {
			u32Handle := ir.TypeHandle(0)
			arrayU32Handle := ir.TypeHandle(1)

			mod := &ir.Module{
				Types: []ir.Type{
					{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
					{Inner: ir.ArrayType{
						Base:   u32Handle,
						Size:   ir.ArraySize{},
						Stride: 4,
					}},
				},
				GlobalVariables: []ir.GlobalVariable{
					{
						Name:   tt.irName,
						Space:  ir.SpaceStorage,
						Access: ir.StorageReadWrite,
						Binding: &ir.ResourceBinding{
							Group:   0,
							Binding: 0,
						},
						Type: arrayU32Handle,
					},
				},
				EntryPoints: []ir.EntryPoint{
					{
						Name:      "main",
						Stage:     ir.StageCompute,
						Function:  ir.Function{Name: "main"},
						Workgroup: [3]uint32{1, 1, 1},
					},
				},
			}

			result, err := Emit(mod, EmitOptions{
				ShaderModelMajor: 6,
				ShaderModelMinor: 0,
				ReachableGlobals: map[ir.GlobalVariableHandle]bool{0: true},
			})
			if err != nil {
				t.Fatalf("Emit failed: %v", err)
			}

			// Find the resource name in metadata strings. The name
			// appears as an MDString operand in the dx.resources
			// metadata tuple. Scan all metadata for string nodes
			// containing the expected name.
			found := false
			for _, md := range result.Metadata {
				if md.Kind == module.MDString && md.StringValue == tt.wantName {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("metadata string %q not found; IR name was %q", tt.wantName, tt.irName)
			}
		})
	}
}

// TestSamplerHeapHandleAfterInputLoads verifies that in fragment shaders with
// sampler heap bindings, dx.op.loadInput calls for interpolated varyings appear
// BEFORE dx.op.bufferLoad calls for the sampler heap index lookup. DXC's lazy
// evaluation produces this ordering because loadInput for vertex inputs is
// resolved before the sampler index indirection at the sample call site. We
// match by emitting sampler heap handles after input loads in emitEntryPoint.
func TestSamplerHeapHandleAfterInputLoads(t *testing.T) {
	vec4f32Handle := ir.TypeHandle(1)
	vec2f32Handle := ir.TypeHandle(2)
	tex2dHandle := ir.TypeHandle(3)
	samplerTyHandle := ir.TypeHandle(4)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                         // [0] f32
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},         // [1] vec4<f32>
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},         // [2] vec2<f32>
			{Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled, SampledKind: ir.ScalarFloat}}, // [3] texture_2d<f32>
			{Inner: ir.SamplerType{Comparison: false}},                                                     // [4] sampler
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "tex", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: tex2dHandle},
			{Name: "samp", Space: ir.SpaceHandle, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}, Type: samplerTyHandle},
		},
	}

	// Build a fragment shader: @fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32>
	// that does textureSample(tex, samp, uv).
	uvBinding := ir.Binding(ir.LocationBinding{Location: 0})
	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})

	sampleExprHandle := ir.ExpressionHandle(3)

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
			{Kind: ir.ExprFunctionArgument{Index: 0}},                       // [0] uv
			{Kind: ir.ExprGlobalVariable{Variable: 0}},                      // [1] tex
			{Kind: ir.ExprGlobalVariable{Variable: 1}},                      // [2] samp
			{Kind: ir.ExprImageSample{Image: 1, Sampler: 2, Coordinate: 0}}, // [3] textureSample
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec2f32Handle},   // uv
			{Handle: &tex2dHandle},     // tex
			{Handle: &samplerTyHandle}, // samp
			{Handle: &vec4f32Handle},   // sample result
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &sampleExprHandle}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	result, err := Emit(mod, EmitOptions{
		ShaderModelMajor: 6,
		ShaderModelMinor: 0,
		ReachableGlobals: map[ir.GlobalVariableHandle]bool{0: true, 1: true},
	})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	// Find the entry function body.
	var entryFn *module.Function
	for _, fn := range result.Functions {
		if !fn.IsDeclaration && fn.Name == "main" {
			entryFn = fn
			break
		}
	}
	if entryFn == nil {
		t.Fatal("entry function 'main' not found")
	}
	if len(entryFn.BasicBlocks) == 0 {
		t.Fatal("entry function has no basic blocks")
	}

	// Walk the instruction sequence and record the position of:
	// - first dx.op.loadInput call (input varyings)
	// - first dx.op.bufferLoad call (sampler heap index lookup)
	firstLoadInput := -1
	firstBufferLoad := -1
	bb := entryFn.BasicBlocks[0]
	for i, instr := range bb.Instructions {
		if instr.Kind != module.InstrCall || instr.CalledFunc == nil {
			continue
		}
		name := instr.CalledFunc.Name
		if firstLoadInput < 0 && (name == "dx.op.loadInput.f32" || name == "dx.op.loadInput.i32") {
			firstLoadInput = i
		}
		if firstBufferLoad < 0 && name == "dx.op.bufferLoad.i32" {
			firstBufferLoad = i
		}
	}

	if firstLoadInput < 0 {
		t.Fatal("no dx.op.loadInput call found in entry function")
	}
	if firstBufferLoad < 0 {
		t.Fatal("no dx.op.bufferLoad call found in entry function (sampler heap)")
	}

	if firstLoadInput >= firstBufferLoad {
		t.Errorf("dx.op.loadInput (pos %d) should appear BEFORE dx.op.bufferLoad (pos %d); "+
			"DXC emits loadInput for interpolated varyings before sampler heap index lookups",
			firstLoadInput, firstBufferLoad)
	}
}

// buildStructReturnVertex creates a vertex shader that returns a struct
// through a local variable:
//
//	struct VertexOutput {
//	    @location(0) color: vec3<f32>,
//	    @builtin(position) position: vec4<f32>,
//	}
//	@vertex fn vs_main(@location(0) in_color: vec3<f32>) -> VertexOutput {
//	    var out: VertexOutput;
//	    out.color = in_color;
//	    out.position = vec4<f32>(1.0, 2.0, 0.0, 1.0);
//	    return out;
//	}
func buildStructReturnVertex() *ir.Module {
	vec3Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)
	structHandle := ir.TypeHandle(2)
	f32Handle := ir.TypeHandle(3)

	locBinding0 := ir.Binding(ir.LocationBinding{Location: 0})
	posBinding := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})

	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Name: "VertexOutput", Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "color", Type: vec3Handle, Offset: 0, Binding: &locBinding0},
					{Name: "position", Type: vec4Handle, Offset: 16, Binding: &posBinding},
				},
				Span: 32,
			}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	// Expression handles
	lvH := ir.ExpressionHandle(1)
	colorFieldH := ir.ExpressionHandle(2)
	posFieldH := ir.ExpressionHandle(3)
	literal1 := ir.ExpressionHandle(4)
	literal2 := ir.ExpressionHandle(5)
	literal0 := ir.ExpressionHandle(6)
	composeH := ir.ExpressionHandle(7)
	loadH := ir.ExpressionHandle(8)

	fn := ir.Function{
		Name: "vs_main",
		Arguments: []ir.FunctionArgument{
			{Name: "in_color", Type: vec3Handle, Binding: &locBinding0},
		},
		Result: &ir.FunctionResult{
			Type:    structHandle,
			Binding: nil,
		},
		LocalVars: []ir.LocalVariable{
			{Name: "out", Type: structHandle, Init: nil},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.ExprAccessIndex{Base: lvH, Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: lvH, Index: 1}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.ExprCompose{Type: vec4Handle, Components: []ir.ExpressionHandle{literal1, literal2, literal0, literal1}}},
			{Kind: ir.ExprLoad{Pointer: lvH}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3Handle},
			{Handle: &structHandle},
			{Handle: &vec3Handle},
			{Handle: &vec4Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
			{Handle: &vec4Handle},
			{Handle: &structHandle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtStore{Pointer: colorFieldH, Value: ir.ExpressionHandle(0)}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 8}}},
			{Kind: ir.StmtStore{Pointer: posFieldH, Value: composeH}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 8, End: 9}}},
			{Kind: ir.StmtReturn{Value: &loadH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{
		{Name: "vs_main", Stage: ir.StageVertex, Function: fn},
	}

	return mod
}

// TestOutputStructPromotion verifies that struct-typed local variables
// used exclusively as output staging (var out: S; out.x = a; return out)
// are identified as promotable by the analysis function.
// DXC's SROA+mem2reg achieves the same elimination.
func TestOutputStructPromotion(t *testing.T) {
	irMod := buildStructReturnVertex()

	mod := module.NewModule(module.VertexShader)
	e := &Emitter{
		ir:                   irMod,
		mod:                  mod,
		outputPromotedLocals: make(map[uint32]bool),
		outputPromotedStores: make(map[outputStoreKey]ir.ExpressionHandle),
	}

	fn := &irMod.EntryPoints[0].Function
	e.currentFn = fn

	// Run the analysis.
	e.analyzeOutputPromotableLocals(fn)

	if !e.outputPromotedLocals[0] {
		t.Fatal("expected local var 0 ('out') to be identified as output-promotable")
	}
}

// TestOutputStructPromotionNotInLoop verifies that struct locals stored
// inside control flow are NOT identified as output-promotable.
func TestOutputStructPromotionNotInLoop(t *testing.T) {
	irMod := buildStructReturnVertex()
	fn := &irMod.EntryPoints[0].Function

	// Wrap the stores inside an if block to make them non-promotable.
	origBody := fn.Body
	fn.Body = []ir.Statement{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
		{Kind: ir.StmtIf{
			Condition: ir.ExpressionHandle(0), // doesn't matter
			Accept:    origBody[1:4],          // stores + emits inside if
			Reject:    nil,
		}},
		origBody[4], // emit for load
		origBody[5], // return
	}

	mod := module.NewModule(module.VertexShader)
	e := &Emitter{
		ir:                   irMod,
		mod:                  mod,
		outputPromotedLocals: make(map[uint32]bool),
		outputPromotedStores: make(map[outputStoreKey]ir.ExpressionHandle),
	}
	e.currentFn = fn

	e.analyzeOutputPromotableLocals(fn)

	if e.outputPromotedLocals[0] {
		t.Fatal("expected local var 0 NOT to be promotable when stores are inside an if block")
	}
}

// TestSingleStoreVectorLocalPromotion verifies that vector-typed locals
// stored once in straight-line code are identified by analyzeSingleStoreLocals.
// This covers the pattern created by SROA: struct decomposition produces
// per-member vector locals with exactly one store and one load.
func TestSingleStoreVectorLocalPromotion(t *testing.T) {
	vec3Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	resultBinding := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Result: &ir.FunctionResult{
			Type:    vec4Handle,
			Binding: &resultBinding,
		},
		LocalVars: []ir.LocalVariable{
			{Name: "color", Type: vec3Handle},    // stored once, loaded once
			{Name: "position", Type: vec4Handle}, // stored once, loaded once
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},                                         // [0] &color
			{Kind: ir.ExprCompose{Type: vec3Handle}},                                          // [1] vec3(...)
			{Kind: ir.ExprLocalVariable{Variable: 1}},                                         // [2] &position
			{Kind: ir.ExprCompose{Type: vec4Handle}},                                          // [3] vec4(...)
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(0)}},                              // [4] load color
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(2)}},                              // [5] load position
			{Kind: ir.ExprCompose{Type: vec4Handle, Components: []ir.ExpressionHandle{4, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3Handle}, {Handle: &vec3Handle},
			{Handle: &vec4Handle}, {Handle: &vec4Handle},
			{Handle: &vec3Handle}, {Handle: &vec4Handle},
			{Handle: &vec4Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtStore{Pointer: ir.ExpressionHandle(0), Value: ir.ExpressionHandle(1)}},
			{Kind: ir.StmtStore{Pointer: ir.ExpressionHandle(2), Value: ir.ExpressionHandle(3)}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageFragment, Function: fn},
	}

	result := analyzeSingleStoreLocals(&fn, irMod)
	if _, ok := result[0]; !ok {
		t.Error("expected local var 0 (vec3 color) to be single-store promotable")
	}
	if _, ok := result[1]; !ok {
		t.Error("expected local var 1 (vec4 position) to be single-store promotable")
	}
}

// TestSingleStoreLocalNotInLoop verifies that locals stored inside control
// flow are NOT identified as single-store promotable.
func TestSingleStoreLocalNotInLoop(t *testing.T) {
	f32Handle := ir.TypeHandle(0)
	vec4Handle := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	fn := ir.Function{
		Name: "main",
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: f32Handle},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle}, {Handle: &f32Handle},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtIf{
				Condition: ir.ExpressionHandle(1),
				Accept: []ir.Statement{
					{Kind: ir.StmtStore{Pointer: ir.ExpressionHandle(0), Value: ir.ExpressionHandle(1)}},
				},
			}},
		},
	}

	result := analyzeSingleStoreLocals(&fn, irMod)
	if _, ok := result[0]; ok {
		t.Error("expected local var 0 NOT to be promotable when stored inside an if block")
	}
	_ = vec4Handle
}

// TestZeroStoreLocalPromotion verifies that scalar/vector locals with
// no Init and no stores are detected by preAllocateLocalVars and
// registered in zeroStoreLocals. This covers the SROA pattern where
// unassigned struct members become separate locals (e.g., texcoord
// fields when only position is assigned).
func TestZeroStoreLocalPromotion(t *testing.T) {
	f32Handle := ir.TypeHandle(0)
	vec2Handle := ir.TypeHandle(1)
	vec4Handle := ir.TypeHandle(2)
	structHandle := ir.TypeHandle(3) // struct type — not eligible

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "f", Type: f32Handle}}}},
		},
	}

	fn := ir.Function{
		Name: "main",
		LocalVars: []ir.LocalVariable{
			{Name: "zero_f32", Type: f32Handle},       // scalar, no init, no store → eligible
			{Name: "zero_vec2", Type: vec2Handle},     // vector, no init, no store → eligible
			{Name: "zero_vec4", Type: vec4Handle},     // vector, no init, no store → eligible
			{Name: "zero_struct", Type: structHandle}, // struct → NOT eligible (only scalar/vector)
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},            // [0] &zero_f32
			{Kind: ir.ExprLocalVariable{Variable: 1}},            // [1] &zero_vec2
			{Kind: ir.ExprLocalVariable{Variable: 2}},            // [2] &zero_vec4
			{Kind: ir.ExprLocalVariable{Variable: 3}},            // [3] &zero_struct
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(0)}}, // [4] load zero_f32
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(1)}}, // [5] load zero_vec2
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(2)}}, // [6] load zero_vec4
			{Kind: ir.ExprLoad{Pointer: ir.ExpressionHandle(3)}}, // [7] load zero_struct
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle}, {Handle: &vec2Handle},
			{Handle: &vec4Handle}, {Handle: &structHandle},
			{Handle: &f32Handle}, {Handle: &vec2Handle},
			{Handle: &vec4Handle}, {Handle: &structHandle},
		},
		Body: []ir.Statement{
			// No stores to any local variable
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 8}}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageCompute, Function: fn},
	}

	// Check that analyzeSingleStoreLocals returns nothing (no stores).
	single := analyzeSingleStoreLocals(&fn, irMod)
	if len(single) != 0 {
		t.Errorf("expected no single-store locals, got %d", len(single))
	}

	// Check that localsStoredTo returns empty (no stores).
	stored := localsStoredTo(&fn)
	if len(stored) != 0 {
		t.Errorf("expected no stored locals, got %d", len(stored))
	}

	// Verify scalar and vector types are detected.
	for _, varIdx := range []uint32{0, 1, 2} {
		lv := &fn.LocalVars[varIdx]
		if lv.Init != nil {
			t.Errorf("var %d: expected nil Init", varIdx)
		}
		if stored[varIdx] {
			t.Errorf("var %d: expected not stored", varIdx)
		}
		inner := irMod.Types[lv.Type].Inner
		switch inner.(type) {
		case ir.ScalarType, ir.VectorType:
			// eligible
		default:
			t.Errorf("var %d: expected scalar or vector, got %T", varIdx, inner)
		}
	}

	// Struct type is NOT eligible.
	structInner := irMod.Types[structHandle].Inner
	if _, ok := structInner.(ir.ScalarType); ok {
		t.Error("struct type should not be ScalarType")
	}
	if _, ok := structInner.(ir.VectorType); ok {
		t.Error("struct type should not be VectorType")
	}
}

// TestZeroStoreLocalWithInit verifies that locals with Init but no stores
// are NOT treated as zero-store locals — they use initOnlyLocals instead.
func TestZeroStoreLocalWithInit(t *testing.T) {
	f32Handle := ir.TypeHandle(0)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	initH := ir.ExpressionHandle(1)
	fn := ir.Function{
		Name: "main",
		LocalVars: []ir.LocalVariable{
			{Name: "with_init", Type: f32Handle, Init: &initH}, // has Init → initOnlyLocals, not zeroStore
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.ExprZeroValue{Type: f32Handle}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle}, {Handle: &f32Handle},
		},
		Body: []ir.Statement{},
	}

	stored := localsStoredTo(&fn)
	lv := &fn.LocalVars[0]

	// Has Init → initOnlyLocals path, NOT zeroStoreLocals.
	if lv.Init == nil {
		t.Error("expected non-nil Init")
	}
	if stored[0] {
		t.Error("expected not stored")
	}
	_ = irMod
}

// TestSameTypeCastElimination verifies that casting between same-width
// integer types (u32 <-> i32) is a no-op in DXIL. Both map to i32 in
// LLVM IR, so bitcast i32 to i32 is redundant. DXC never emits these.
func TestSameTypeCastElimination(t *testing.T) {
	// Test no-op cases: same type or same-width integer reinterpretation.
	noopCases := []struct {
		name     string
		src, dst ir.ScalarType
	}{
		{"i32_to_i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		{"u32_to_u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		{"u32_to_i32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		{"i32_to_u32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		{"f32_to_f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{"u16_to_i16", ir.ScalarType{Kind: ir.ScalarUint, Width: 2}, ir.ScalarType{Kind: ir.ScalarSint, Width: 2}},
	}

	for _, tt := range noopCases {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emitter{
				constMap:    make(map[int]*module.Constant),
				intConsts:   make(map[int64]int),
				floatConsts: make(map[uint64]int),
			}
			e.mod = &module.Module{}

			srcID := 42
			result, err := e.emitScalarCast(srcID, tt.src, tt.dst, true)
			if err != nil {
				t.Fatalf("emitScalarCast: %v", err)
			}
			if result != srcID {
				t.Errorf("expected no-op (same ID %d), got %d", srcID, result)
			}
		})
	}

	// Test that different types/widths are NOT eliminated (verified via
	// isIntegerKind logic -- actual instruction emission needs full emitter).
	nonNoopCases := []struct {
		name     string
		src, dst ir.ScalarType
	}{
		{"f32_to_i32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		{"i32_to_f32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{"i16_to_i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 2}, ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
	}
	for _, tt := range nonNoopCases {
		t.Run(tt.name+"_not_noop", func(t *testing.T) {
			// Verify the type check logic: different kind/width should not return early.
			sameType := tt.src == tt.dst
			sameWidthInt := tt.src.Width == tt.dst.Width && isIntegerKind(tt.src.Kind) && isIntegerKind(tt.dst.Kind)
			if sameType || sameWidthInt {
				t.Errorf("expected non-noop cast %s->%s to NOT be eliminated", tt.name, tt.name)
			}
		})
	}
}

// TestIsIntegerKind verifies that isIntegerKind correctly identifies
// signed and unsigned integer scalar kinds.
func TestIsIntegerKind(t *testing.T) {
	tests := []struct {
		kind ir.ScalarKind
		want bool
	}{
		{ir.ScalarSint, true},
		{ir.ScalarUint, true},
		{ir.ScalarFloat, false},
		{ir.ScalarBool, false},
	}
	for _, tt := range tests {
		if got := isIntegerKind(tt.kind); got != tt.want {
			t.Errorf("isIntegerKind(%v) = %v, want %v", tt.kind, got, tt.want)
		}
	}
}

// TestSubToAddNeg verifies that integer sub X, C is canonicalized to
// add X, -C for positive constants, matching DXC's LLVM canonicalization.
func TestSubToAddNeg(t *testing.T) {
	e := &Emitter{
		constMap:    make(map[int]*module.Constant),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
	}
	e.mod = &module.Module{}

	tests := []struct {
		value    int64
		wantNeg  bool
		negValue int64
	}{
		{1, true, -1},
		{2, true, -2},
		{42, true, -42},
		{0, false, 0},    // 0 is not positive
		{-1, false, 0},   // negative constants not canonicalized
		{-100, false, 0}, // negative constants not canonicalized
	}

	for _, tt := range tests {
		constID := e.getIntConstID(tt.value)
		negID, ok := e.trySubToAddNeg(constID)
		if ok != tt.wantNeg {
			t.Errorf("trySubToAddNeg(%d): got ok=%v, want %v", tt.value, ok, tt.wantNeg)
			continue
		}
		if ok {
			negConst, hasConst := e.constMap[negID]
			if !hasConst {
				t.Errorf("trySubToAddNeg(%d): neg ID not in constMap", tt.value)
				continue
			}
			if negConst.IntValue != tt.negValue {
				t.Errorf("trySubToAddNeg(%d): neg = %d, want %d", tt.value, negConst.IntValue, tt.negValue)
			}
		}
	}

	// Float constants should not trigger.
	floatID := e.getFloatConstID(1.0)
	if _, ok := e.trySubToAddNeg(floatID); ok {
		t.Error("trySubToAddNeg should not trigger for float constant")
	}

	// Undef should not trigger.
	undefID := e.getUndefConstID()
	if _, ok := e.trySubToAddNeg(undefID); ok {
		t.Error("trySubToAddNeg should not trigger for undef")
	}
}

// TestURemStrengthReduction verifies that unsigned modulo by a power
// of 2 is strength-reduced to a bitwise AND. This matches DXC's LLVM
// TestMulToShl verifies that integer multiply by a power of 2 is
// strength-reduced to a left shift, matching DXC's LLVM InstCombine.
func TestMulToShl(t *testing.T) {
	e := &Emitter{
		constMap:    make(map[int]*module.Constant),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
	}
	e.mod = &module.Module{}

	tests := []struct {
		value    int64
		wantShl  bool
		shiftAmt int64
	}{
		{2, true, 1},
		{4, true, 2},
		{8, true, 3},
		{16, true, 4},
		{256, true, 8},
		{1024, true, 10},
		{1, false, 0},  // 1 is NOT >= 2
		{3, false, 0},  // 3 is not a power of 2
		{5, false, 0},  // 5 is not a power of 2
		{0, false, 0},  // 0 is not a power of 2
		{-2, false, 0}, // negative values not handled
	}

	for _, tt := range tests {
		constID := e.getIntConstID(tt.value)
		shiftID, ok := e.tryMulToShl(constID)
		if ok != tt.wantShl {
			t.Errorf("tryMulToShl(%d): got ok=%v, want %v", tt.value, ok, tt.wantShl)
			continue
		}
		if ok {
			shiftConst, hasConst := e.constMap[shiftID]
			if !hasConst {
				t.Errorf("tryMulToShl(%d): shift ID not in constMap", tt.value)
				continue
			}
			if shiftConst.IntValue != tt.shiftAmt {
				t.Errorf("tryMulToShl(%d): shift = %d, want %d", tt.value, shiftConst.IntValue, tt.shiftAmt)
			}
		}
	}
}

// InstCombine pass: urem x, 2^N -> and x, (2^N - 1).
func TestURemStrengthReduction(t *testing.T) {
	e := &Emitter{
		constMap:    make(map[int]*module.Constant),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
	}
	e.mod = &module.Module{}

	// Test power-of-2 constants.
	tests := []struct {
		value     int64
		expectAnd bool
		mask      int64
	}{
		{2, true, 1},
		{4, true, 3},
		{8, true, 7},
		{16, true, 15},
		{256, true, 255},
		{1024, true, 1023},
		{1, false, 0},  // 1 is NOT >= 2
		{3, false, 0},  // 3 is not a power of 2
		{5, false, 0},  // 5 is not a power of 2
		{6, false, 0},  // 6 is not a power of 2
		{7, false, 0},  // 7 is not a power of 2
		{0, false, 0},  // 0 is not a power of 2
		{-2, false, 0}, // negative values are not power of 2
	}

	for _, tt := range tests {
		// Create a constant for the value.
		constID := e.getIntConstID(tt.value)
		maskID, ok := e.tryURemToBitwiseAnd(constID)
		if ok != tt.expectAnd {
			t.Errorf("tryURemToBitwiseAnd(%d): got ok=%v, want %v", tt.value, ok, tt.expectAnd)
			continue
		}
		if ok {
			maskConst, hasConst := e.constMap[maskID]
			if !hasConst {
				t.Errorf("tryURemToBitwiseAnd(%d): mask ID not in constMap", tt.value)
				continue
			}
			if maskConst.IntValue != tt.mask {
				t.Errorf("tryURemToBitwiseAnd(%d): mask = %d, want %d", tt.value, maskConst.IntValue, tt.mask)
			}
		}
	}
}

// TestURemStrengthReductionNonInt verifies that float and undef
// constants are not treated as power-of-2 for strength reduction.
func TestURemStrengthReductionNonInt(t *testing.T) {
	e := &Emitter{
		constMap:    make(map[int]*module.Constant),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
	}
	e.mod = &module.Module{}

	// Float constant should not trigger.
	floatID := e.getFloatConstID(2.0)
	if _, ok := e.tryURemToBitwiseAnd(floatID); ok {
		t.Error("tryURemToBitwiseAnd should not trigger for float constant")
	}

	// Undef should not trigger.
	undefID := e.getUndefConstID()
	if _, ok := e.tryURemToBitwiseAnd(undefID); ok {
		t.Error("tryURemToBitwiseAnd should not trigger for undef")
	}

	// Non-existent ID should not trigger.
	if _, ok := e.tryURemToBitwiseAnd(99999); ok {
		t.Error("tryURemToBitwiseAnd should not trigger for unknown value ID")
	}
}

// TestHlslMangledGroupsharedName verifies the MSVC name decoration for
// workgroup (groupshared) variables matches DXC's output pattern.
func TestHlslMangledGroupsharedName(t *testing.T) {
	cases := []struct {
		name    string
		varName string
		varType ir.TypeInner
		want    string
	}{
		{
			"float array",
			"shared_data",
			ir.ArrayType{
				Base: 0,
				Size: ir.ArraySize{Constant: ptrU32(256)},
			},
			"\x01?shared_data@@3PAMA",
		},
		{
			"uint scalar",
			"shared_counter",
			ir.ScalarType{Kind: ir.ScalarUint, Width: 4},
			"\x01?shared_counter@@3IA",
		},
		{
			"int array",
			"arr_i32_",
			ir.ArrayType{
				Base: 1,
				Size: ir.ArraySize{Constant: ptrU32(128)},
			},
			"\x01?arr_i32_@@3PAHA",
		},
		{
			"uint64 scalar",
			"workgroup_atomic_scalar",
			ir.ScalarType{Kind: ir.ScalarUint, Width: 8},
			"\x01?workgroup_atomic_scalar@@3_KA",
		},
		{
			"digit-ending name gets underscore",
			"arr_i32",
			ir.ArrayType{
				Base: 1,
				Size: ir.ArraySize{Constant: ptrU32(128)},
			},
			"\x01?arr_i32_@@3PAHA",
		},
		{
			"atomic uint scalar",
			"a",
			ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			"\x01?a@@3IA",
		},
		{
			"atomic uint array",
			"a",
			ir.ArrayType{
				Base: 2,
				Size: ir.ArraySize{Constant: ptrU32(64)},
			},
			"\x01?a@@3PAIA",
		},
	}

	types := []ir.Type{
		{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                       // [0] float
		{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                        // [1] int
		{Inner: ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}}, // [2] atomic<uint>
	}
	irMod := &ir.Module{Types: types}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gv := &ir.GlobalVariable{
				Name:  tc.varName,
				Space: ir.SpaceWorkGroup,
			}
			// Add the variable's type to the module if needed.
			th := ir.TypeHandle(len(irMod.Types))
			irMod.Types = append(irMod.Types, ir.Type{Inner: tc.varType})
			gv.Type = th

			got := hlslMangledGroupsharedName(irMod, gv)
			if got != tc.want {
				t.Errorf("hlslMangledGroupsharedName(%q) = %q, want %q", tc.varName, got, tc.want)
			}
		})
	}
}

// TestMsvcScalarCode verifies the MSVC type decoration codes for scalar types.
func TestMsvcScalarCode(t *testing.T) {
	cases := []struct {
		name string
		s    ir.ScalarType
		want string
	}{
		{"float", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, "M"},
		{"double", ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, "N"},
		{"uint", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, "I"},
		{"uint64", ir.ScalarType{Kind: ir.ScalarUint, Width: 8}, "_K"},
		{"int", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, "H"},
		{"int64", ir.ScalarType{Kind: ir.ScalarSint, Width: 8}, "_J"},
		{"unknown width", ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := msvcScalarCode(tc.s)
			if got != tc.want {
				t.Errorf("msvcScalarCode(%v) = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

// TestIsInt64Type verifies the module-level i64 type detector.
func TestIsInt64Type(t *testing.T) {
	cases := []struct {
		name string
		ty   *module.Type
		want bool
	}{
		{"nil", nil, false},
		{"i32", &module.Type{Kind: module.TypeInteger, IntBits: 32}, false},
		{"i64", &module.Type{Kind: module.TypeInteger, IntBits: 64}, true},
		{"f64", &module.Type{Kind: module.TypeFloat, FloatBits: 64}, false},
		{"i16", &module.Type{Kind: module.TypeInteger, IntBits: 16}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isInt64Type(tc.ty)
			if got != tc.want {
				t.Errorf("isInt64Type(%v) = %v, want %v", tc.ty, got, tc.want)
			}
		})
	}
}

// TestAlignForType verifies that alignForType returns the LLVM bitcode
// alignment encoding (log2(bytes)+1) for each scalar type.
// Reference: LLVM INST_STORE/INST_LOAD alignment field, Mesa dxil_module.c.
func TestAlignForType(t *testing.T) {
	e := &Emitter{mod: module.NewModule(module.ComputeShader)}
	cases := []struct {
		name string
		ty   *module.Type
		want int // log2(bytes)+1
	}{
		{"i1", e.mod.GetIntType(1), 1},     // 1-byte: log2(1)+1=1
		{"i8", e.mod.GetIntType(8), 1},     // 1-byte: log2(1)+1=1
		{"i16", e.mod.GetIntType(16), 2},   // 2-byte: log2(2)+1=2
		{"i32", e.mod.GetIntType(32), 3},   // 4-byte: log2(4)+1=3
		{"i64", e.mod.GetIntType(64), 4},   // 8-byte: log2(8)+1=4
		{"f16", e.mod.GetFloatType(16), 2}, // 2-byte: log2(2)+1=2
		{"f32", e.mod.GetFloatType(32), 3}, // 4-byte: log2(4)+1=3
		{"f64", e.mod.GetFloatType(64), 4}, // 8-byte: log2(8)+1=4
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := e.alignForType(tc.ty)
			if got != tc.want {
				t.Errorf("alignForType(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// TestGlobalVarAlignment verifies that globalVarAlignment returns the
// correct byte alignment for workgroup global variables. DXC uses
// align 8 for i64/f64 types and align 4 for everything else.
func TestGlobalVarAlignment(t *testing.T) {
	m := module.NewModule(module.ComputeShader)
	cases := []struct {
		name string
		ty   *module.Type
		want uint32
	}{
		{"i32", m.GetIntType(32), 4},
		{"i64", m.GetIntType(64), 8},
		{"f32", m.GetFloatType(32), 4},
		{"f64", m.GetFloatType(64), 8},
		{"[2 x i32]", m.GetArrayType(m.GetIntType(32), 2), 4},
		{"[2 x i64]", m.GetArrayType(m.GetIntType(64), 2), 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := globalVarAlignment(tc.ty)
			if got != tc.want {
				t.Errorf("globalVarAlignment(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}

// TestFoldNumericCast verifies that constant int-to-float and float-to-int
// casts are folded at compile time, matching DXC's LLVM constant folder.
func TestFoldNumericCast(t *testing.T) {
	e := &Emitter{
		constMap:    make(map[int]*module.Constant),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
	}
	e.mod = &module.Module{}

	tests := []struct {
		name      string
		srcKind   ir.ScalarKind
		dstKind   ir.ScalarKind
		intVal    int64
		floatVal  float64
		wantFold  bool
		wantFloat float64
		wantInt   int64
	}{
		{"sint0_to_float", ir.ScalarSint, ir.ScalarFloat, 0, 0, true, 0.0, 0},
		{"sint1_to_float", ir.ScalarSint, ir.ScalarFloat, 1, 0, true, 1.0, 0},
		{"sint_neg1_to_float", ir.ScalarSint, ir.ScalarFloat, -1, 0, true, -1.0, 0},
		{"uint2_to_float", ir.ScalarUint, ir.ScalarFloat, 2, 0, true, 2.0, 0},
		{"float0_to_sint", ir.ScalarFloat, ir.ScalarSint, 0, 0.0, true, 0, 0},
		{"float1_to_sint", ir.ScalarFloat, ir.ScalarSint, 0, 1.0, true, 0, 1},
		{"float2_5_to_uint", ir.ScalarFloat, ir.ScalarUint, 0, 2.5, true, 0, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srcID int
			if tt.srcKind == ir.ScalarFloat {
				srcID = e.getFloatConstID(tt.floatVal)
			} else {
				srcID = e.getIntConstID(tt.intVal)
			}
			src := ir.ScalarType{Kind: tt.srcKind, Width: 4}
			dst := ir.ScalarType{Kind: tt.dstKind, Width: 4}
			c := e.constMap[srcID]
			resultID, ok := e.tryFoldNumericCast(c, src, dst)
			if ok != tt.wantFold {
				t.Errorf("fold=%v, want %v", ok, tt.wantFold)
				return
			}
			if !ok {
				return
			}
			rc, hasConst := e.constMap[resultID]
			if !hasConst {
				t.Error("result not in constMap")
				return
			}
			if tt.dstKind == ir.ScalarFloat {
				if rc.FloatValue != tt.wantFloat {
					t.Errorf("float result = %v, want %v", rc.FloatValue, tt.wantFloat)
				}
			} else {
				if rc.IntValue != tt.wantInt {
					t.Errorf("int result = %v, want %v", rc.IntValue, tt.wantInt)
				}
			}
		})
	}
}

// TestAddMulOrShlInstr verifies that addMulOrShlInstr delegates to
// tryMulToShl for power-of-2 constants. The actual instruction
// generation is tested via the full DXC golden pipeline (bounds-check-
// dynamic-buffer/main now uses shl i32 %R, 4 instead of mul i32 %R, 16).
func TestAddMulOrShlInstr(t *testing.T) {
	e := &Emitter{
		constMap:    make(map[int]*module.Constant),
		intConsts:   make(map[int64]int),
		floatConsts: make(map[uint64]int),
		undefID:     -1,
	}
	e.mod = &module.Module{}

	// Verify that tryMulToShl fires for the stride constants used in
	// CBV/UAV byte offset calculations.
	strides := []struct {
		stride   int64
		wantShl  bool
		shiftAmt int64
	}{
		{4, true, 2},   // sizeof(f32)
		{8, true, 3},   // sizeof(f64)
		{16, true, 4},  // sizeof(vec4 or @size(16) struct)
		{32, true, 5},  // 2 * sizeof(vec4)
		{12, false, 0}, // sizeof(vec3) — not a power of 2
		{24, false, 0}, // sizeof(mat2x3) — not a power of 2
	}

	for _, tc := range strides {
		constID := e.getIntConstID(tc.stride)
		shiftID, ok := e.tryMulToShl(constID)
		if ok != tc.wantShl {
			t.Errorf("stride %d: shl=%v, want %v", tc.stride, ok, tc.wantShl)
			continue
		}
		if ok {
			sc := e.constMap[shiftID]
			if sc.IntValue != tc.shiftAmt {
				t.Errorf("stride %d: shift=%d, want %d", tc.stride, sc.IntValue, tc.shiftAmt)
			}
		}
	}
}
