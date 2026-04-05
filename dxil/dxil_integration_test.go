package dxil

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/dxil/internal/container"
	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// TestCompile_EmptyVertex tests the simplest possible vertex shader:
// @vertex fn main() {}
func TestCompile_EmptyVertex(t *testing.T) {
	irMod := &ir.Module{
		Types: []ir.Type{},
	}

	fn := ir.Function{
		Name: "main",
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: fn},
	}

	data, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if len(data) < 32 {
		t.Fatalf("output too small: %d bytes", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != container.FourCCDXBC {
		t.Errorf("container magic: got 0x%08X, want DXBC", magic)
	}

	// Write for DXC validation.
	writeToTmp(t, data, "test_empty_vs.dxil")
}

// TestDXC_ManualModuleWithConstants builds a module by hand with constants
// to verify DXC accepts our constant encoding.
func TestDXC_ManualModuleWithConstants(t *testing.T) {
	mod := module.NewModule(module.VertexShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	funcTy := mod.GetFunctionType(voidTy, nil)

	mainFn := mod.AddFunction("main", funcTy, false)
	bb := mainFn.AddBasicBlock("entry")
	bb.AddInstruction(module.NewRetVoidInstr())

	// Add a constant that won't be referenced by the function body.
	mod.AddIntConst(i32Ty, 42)

	// Metadata.
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)

	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})

	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})

	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_manual_constants.dxil")
}

// TestDXC_ManualModuleWithDeclOnly builds a module with a dx.op function
// declaration but no call, to verify function declaration encoding.
func TestDXC_ManualModuleWithDeclOnly(t *testing.T) {
	mod := module.NewModule(module.VertexShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	f32Ty := mod.GetFloatType(32)
	i8Ty := mod.GetIntType(8)

	mainFuncTy := mod.GetFunctionType(voidTy, nil)
	loadInputFuncTy := mod.GetFunctionType(f32Ty, []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, i32Ty})

	// Note: in DXIL, function declarations come before definitions.
	mod.AddFunction("dx.op.loadInput.f32", loadInputFuncTy, true)
	mainFn := mod.AddFunction("main", mainFuncTy, false)

	bb := mainFn.AddBasicBlock("entry")
	bb.AddInstruction(module.NewRetVoidInstr())

	// Metadata.
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)
	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})
	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})
	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_manual_decl_only.dxil")
}

// TestDXC_ManualModuleWithDeclAndCall builds a module with a dx.op function
// declaration and a call to it, to isolate the call encoding.
func TestDXC_ManualModuleWithDeclAndCall(t *testing.T) {
	mod := module.NewModule(module.VertexShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	f32Ty := mod.GetFloatType(32)
	i8Ty := mod.GetIntType(8)

	// Function types.
	mainFuncTy := mod.GetFunctionType(voidTy, nil) // void()

	// dx.op.loadInput.f32: float(i32, i32, i32, i8, i32)
	loadInputFuncTy := mod.GetFunctionType(f32Ty, []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, i32Ty})

	// Add functions. DXC expects declarations BEFORE definitions.
	loadInputFn := mod.AddFunction("dx.op.loadInput.f32", loadInputFuncTy, true)
	mainFn := mod.AddFunction("main", mainFuncTy, false)

	// Constants for the call.
	_ = mod.AddIntConst(i32Ty, 4) // opcodeConst: OpLoadInput = 4
	_ = mod.AddIntConst(i32Ty, 0) // inputIDConst: input 0
	_ = mod.AddIntConst(i32Ty, 0) // rowConst: row 0 (dedup'd with inputID)
	_ = mod.AddIntConst(i32Ty, 0) // colConst: col 0 (dedup'd)
	mod.Constants = append(mod.Constants, &module.Constant{ConstType: i32Ty, IsUndef: true})

	// Build main function body: just a loadInput call + ret void.
	bb := mainFn.AddBasicBlock("entry")

	// The call instruction operands use global value IDs.
	// Global: loadInputFn(0), mainFn(1), opcodeConst(2), inputIDConst(3),
	//         rowConst(4), colConst(5), undefConst(6)
	// Function body starts at value ID 7.
	callInstr := &module.Instruction{
		Kind:       module.InstrCall,
		HasValue:   true,
		ResultType: f32Ty,
		CalledFunc: loadInputFn,
		Operands:   []int{2, 3, 4, 5, 6}, // opcodeConst, inputID, row, col, undef
		ValueID:    7,                    // first value in function body
	}
	bb.AddInstruction(callInstr)
	bb.AddInstruction(module.NewRetVoidInstr())

	// Metadata.
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)

	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})

	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})

	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_manual_call.dxil")
}

// TestDXC_MinimalCall tests the absolute simplest call instruction:
// declare void @foo()
// define void @main() { call void @foo(); ret void }
func TestDXC_MinimalCall(t *testing.T) {
	mod := module.NewModule(module.VertexShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	voidFuncTy := mod.GetFunctionType(voidTy, nil)

	fooFn := mod.AddFunction("foo", voidFuncTy, true)
	mainFn := mod.AddFunction("main", voidFuncTy, false)

	bb := mainFn.AddBasicBlock("entry")

	// Call foo(). fooFn has ValueID=0, mainFn has ValueID=1.
	// No constants, so globalValueCount = 0 (globalvars) + 2 (functions) + 0 (constants before metadata).
	// But wait, metadata creates constants too.
	callInstr := &module.Instruction{
		Kind:       module.InstrCall,
		HasValue:   false,
		ResultType: voidTy,
		CalledFunc: fooFn,
		Operands:   nil, // no args
	}
	bb.AddInstruction(callInstr)
	bb.AddInstruction(module.NewRetVoidInstr())

	// Metadata (creates constants).
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)
	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})
	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})
	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_minimal_call.dxil")
}

// TestDXC_CallWithOneArg tests a call with one i32 argument.
func TestDXC_CallWithOneArg(t *testing.T) {
	mod := module.NewModule(module.VertexShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	// bar takes one i32 argument.
	barFuncTy := mod.GetFunctionType(voidTy, []*module.Type{i32Ty})
	mainFuncTy := mod.GetFunctionType(voidTy, nil)

	barFn := mod.AddFunction("bar", barFuncTy, true)
	mainFn := mod.AddFunction("main", mainFuncTy, false)

	// Constants.
	_ = mod.AddIntConst(i32Ty, 42) // argConst, gets ValueID=2 after assignIDs

	bb := mainFn.AddBasicBlock("entry")

	// Values: barFn(0), mainFn(1), argConst(2)
	// Metadata consts added below: const0(3), const1(4), const6(5)
	// globalValueCount = 2 + 4 = 6 (after metadata consts added)
	// Actually: globalValueCount = len(globalVars) + len(functions) + len(constants)
	// At serialization time: 0 gv + 2 fn + N consts

	// Call bar(42). currentValueID at first instr = globalValueCount.
	callInstr := &module.Instruction{
		Kind:       module.InstrCall,
		HasValue:   false,
		ResultType: voidTy,
		CalledFunc: barFn,
		Operands:   []int{2}, // argConst value ID = 2
	}
	bb.AddInstruction(callInstr)
	bb.AddInstruction(module.NewRetVoidInstr())

	// Metadata.
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)

	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})
	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})
	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_call_one_arg.dxil")
}

// TestDXC_CallWithFiveArgs tests a dx.op.loadInput-style call with 5 args.
func TestDXC_CallWithFiveArgs(t *testing.T) {
	mod := module.NewModule(module.VertexShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	f32Ty := mod.GetFloatType(32)
	i8Ty := mod.GetIntType(8)
	mainFuncTy := mod.GetFunctionType(voidTy, nil)
	loadInputFuncTy := mod.GetFunctionType(f32Ty, []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, i32Ty})

	loadInputFn := mod.AddFunction("dx.op.loadInput.f32", loadInputFuncTy, true)
	mainFn := mod.AddFunction("main", mainFuncTy, false)

	// Constants: 5 of them.
	_ = mod.AddIntConst(i32Ty, 4)                                                            // [0] opcode = LoadInput
	_ = mod.AddIntConst(i32Ty, 0)                                                            // [1] inputID
	_ = mod.AddIntConst(i32Ty, 0)                                                            // [2] row
	mod.Constants = append(mod.Constants, &module.Constant{ConstType: i8Ty, IntValue: 0})    // [3] col (i8)
	mod.Constants = append(mod.Constants, &module.Constant{ConstType: i32Ty, IsUndef: true}) // [4] vertexID (undef)

	bb := mainFn.AddBasicBlock("entry")

	// ValueIDs after assignIDs: loadInputFn(0), mainFn(1), consts(2,3,4,5,6)
	// globalValueCount = 2 + 5 + metadata_consts

	// Metadata constants:
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)

	// Now total constants = 5 + 3 = 8. GlobalValueCount = 2 + 8 = 10.
	// Call instruction at currentValueID = 10.
	callInstr := &module.Instruction{
		Kind:       module.InstrCall,
		HasValue:   true,
		ResultType: f32Ty,
		CalledFunc: loadInputFn,
		Operands:   []int{2, 3, 4, 5, 6}, // opcode, inputID, row, col, undef
		ValueID:    10,
	}
	bb.AddInstruction(callInstr)
	bb.AddInstruction(module.NewRetVoidInstr())

	// Metadata.
	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})
	mdVS := mod.AddMetadataString("vs")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdVS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})
	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.VertexShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_call_five_args.dxil")
}

// TestDebug_PassthroughEmitterIDs inspects value IDs after emission
// to verify correctness.
func TestDebug_PassthroughEmitterIDs(t *testing.T) {
	vec4f32Handle := ir.TypeHandle(0)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

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
	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "main", Stage: ir.StageVertex, Function: fn},
	}

	data, err := Compile(irMod, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Dump the DXIL module internals after emission.
	// Re-create the module for inspection.
	emitOpts := emit.EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0}
	mod, err := emit.Emit(irMod, emitOpts)
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	t.Logf("Functions (%d):", len(mod.Functions))
	for i, fn := range mod.Functions {
		t.Logf("  [%d] %s (decl=%v, valueID=%d, typeID=%d)", i, fn.Name, fn.IsDeclaration, fn.ValueID, fn.FuncType.ID)
	}

	t.Logf("Constants (%d):", len(mod.Constants))
	for i, c := range mod.Constants {
		if c.IsUndef {
			t.Logf("  [%d] undef (type=%v, valueID=%d)", i, c.ConstType.Kind, c.ValueID)
		} else if c.ConstType.Kind == module.TypeFloat {
			t.Logf("  [%d] float %f (valueID=%d)", i, c.FloatValue, c.ValueID)
		} else {
			t.Logf("  [%d] int %d (valueID=%d)", i, c.IntValue, c.ValueID)
		}
	}

	globalCount := len(mod.GlobalVars) + len(mod.Functions) + len(mod.Constants)
	t.Logf("globalValueCount=%d", globalCount)

	var mainFn *module.Function
	for _, fn := range mod.Functions {
		if fn.Name == "main" {
			mainFn = fn
			break
		}
	}
	if mainFn != nil {
		for _, bb := range mainFn.BasicBlocks {
			for i, instr := range bb.Instructions {
				t.Logf("  instr[%d]: kind=%d, hasValue=%v, valueID=%d, operands=%v, callee=%v",
					i, instr.Kind, instr.HasValue, instr.ValueID, instr.Operands,
					func() string {
						if instr.CalledFunc != nil {
							return instr.CalledFunc.Name
						}
						return "<nil>"
					}())
			}
		}
	}

	_ = data // already verified
}

// TestDXC_StoreOutputShader tests a minimal fragment shader that only
// calls storeOutput — matching DXC's own output pattern.
func TestDXC_StoreOutputShader(t *testing.T) {
	mod := module.NewModule(module.PixelShader)

	voidTy := mod.GetVoidType()
	i32Ty := mod.GetIntType(32)
	f32Ty := mod.GetFloatType(32)
	i8Ty := mod.GetIntType(8)

	mainFuncTy := mod.GetFunctionType(voidTy, nil)
	// storeOutput: void(i32 opcode, i32 outputID, i32 row, i8 col, float value)
	storeOutFuncTy := mod.GetFunctionType(voidTy, []*module.Type{i32Ty, i32Ty, i32Ty, i8Ty, f32Ty})

	storeOutFn := mod.AddFunction("dx.op.storeOutput.f32", storeOutFuncTy, true)
	mainFn := mod.AddFunction("main", mainFuncTy, false)

	// Constants (in order they'll be referenced).
	_ = mod.AddIntConst(i32Ty, 5) // constOpcode: StoreOutput opcode = 5
	_ = mod.AddIntConst(i32Ty, 0) // constZeroI32: outputID=0, row=0
	_ = mod.AddIntConst(i8Ty, 0)  // constZeroI8: col=0
	_ = mod.AddIntConst(i8Ty, 1)  // constOneI8: col=1
	_ = mod.AddIntConst(i8Ty, 2)  // constTwoI8: col=2
	_ = mod.AddIntConst(i8Ty, 3)  // constThreeI8: col=3

	// Float constants for the output color.
	constOneF32 := &module.Constant{ConstType: f32Ty, FloatValue: 1.0}
	constZeroF32 := &module.Constant{ConstType: f32Ty, FloatValue: 0.0}
	mod.Constants = append(mod.Constants, constOneF32, constZeroF32)

	// Metadata constants.
	const0 := mod.AddIntConst(i32Ty, 0)
	const1 := mod.AddIntConst(i32Ty, 1)
	const6 := mod.AddIntConst(i32Ty, 6)

	// Value IDs (after assignIDs):
	// storeOutFn=0, mainFn=1
	// constOpcode=2, constZeroI32=3, constZeroI8=4, constOneI8=5,
	// constTwoI8=6, constThreeI8=7, constOneF32=8, constZeroF32=9,
	// const0=10, const1=11, const6=12
	// globalValueCount=13

	bb := mainFn.AddBasicBlock("entry")

	// call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 0, float 1.0)
	bb.AddInstruction(&module.Instruction{
		Kind: module.InstrCall, HasValue: false, ResultType: voidTy,
		CalledFunc: storeOutFn,
		Operands:   []int{2, 3, 3, 4, 8}, // opcode, 0, 0, col0, 1.0
	})
	// call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 1, float 0.0)
	bb.AddInstruction(&module.Instruction{
		Kind: module.InstrCall, HasValue: false, ResultType: voidTy,
		CalledFunc: storeOutFn,
		Operands:   []int{2, 3, 3, 5, 9}, // opcode, 0, 0, col1, 0.0
	})
	// call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 2, float 0.0)
	bb.AddInstruction(&module.Instruction{
		Kind: module.InstrCall, HasValue: false, ResultType: voidTy,
		CalledFunc: storeOutFn,
		Operands:   []int{2, 3, 3, 6, 9}, // opcode, 0, 0, col2, 0.0
	})
	// call void @dx.op.storeOutput.f32(i32 5, i32 0, i32 0, i8 3, float 1.0)
	bb.AddInstruction(&module.Instruction{
		Kind: module.InstrCall, HasValue: false, ResultType: voidTy,
		CalledFunc: storeOutFn,
		Operands:   []int{2, 3, 3, 7, 8}, // opcode, 0, 0, col3, 1.0
	})
	bb.AddInstruction(module.NewRetVoidInstr())

	// Metadata.
	mdVer1 := mod.AddMetadataValue(i32Ty, const1)
	mdVer0 := mod.AddMetadataValue(i32Ty, const0)
	mdVersionTuple := mod.AddMetadataTuple([]*module.MetadataNode{mdVer1, mdVer0})
	mod.AddNamedMetadata("dx.version", []*module.MetadataNode{mdVersionTuple})
	mdPS := mod.AddMetadataString("ps")
	mdSM6 := mod.AddMetadataValue(i32Ty, const6)
	mdSM0 := mod.AddMetadataValue(i32Ty, const0)
	mdSM := mod.AddMetadataTuple([]*module.MetadataNode{mdPS, mdSM6, mdSM0})
	mod.AddNamedMetadata("dx.shaderModel", []*module.MetadataNode{mdSM})
	mdMainName := mod.AddMetadataString("main")
	mdEntry := mod.AddMetadataTuple([]*module.MetadataNode{nil, mdMainName, nil, nil, nil})
	mod.AddNamedMetadata("dx.entryPoints", []*module.MetadataNode{mdEntry})

	bc := module.Serialize(mod)
	c := container.New()
	c.AddFeaturesPart(0)
	c.AddDXILPart(uint32(module.PixelShader), 1, 0, bc)
	c.AddHashPart()
	data := c.Bytes()
	container.SetBypassHash(data)

	writeToTmp(t, data, "test_storeoutput_ps.dxil")
}

func writeToTmp(t *testing.T, data []byte, name string) {
	t.Helper()
	tmpDir := filepath.Join("..", "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Logf("warning: could not create tmp dir: %v", err)
		return
	}
	outPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		t.Logf("warning: could not write: %v", err)
	} else {
		t.Logf("wrote %d bytes to %s", len(data), outPath)
	}
}
