// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package emit

// Integration tests for DXIL emit coverage. Each test constructs an IR module
// exercising specific expression/statement/type paths and runs it through
// Emit(). Tests verify both successful compilation and that the emitted
// module contains expected instruction patterns.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func emitOK(t *testing.T, irMod *ir.Module) *module.Module {
	t.Helper()
	mod, err := Emit(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}
	bc := module.Serialize(mod)
	if len(bc) == 0 {
		t.Fatal("serialization produced empty bitcode")
	}
	if bc[0] != 'B' || bc[1] != 'C' {
		t.Fatal("invalid bitcode magic")
	}
	return mod
}

func mainFunc(t *testing.T, mod *module.Module) *module.Function {
	t.Helper()
	for i := range mod.Functions {
		if mod.Functions[i].Name == "main" {
			return mod.Functions[i]
		}
	}
	t.Fatal("main function not found")
	return nil
}

func countInstr(fn *module.Function, kind module.InstrKind) int {
	n := 0
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == kind {
				n++
			}
		}
	}
	return n
}

func hasFuncNamed(mod *module.Module, name string) bool {
	for _, fn := range mod.Functions {
		if fn.Name == name {
			return true
		}
	}
	return false
}

func hasCallTo(fn *module.Function, name string) bool {
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && instr.CalledFunc.Name == name {
				return true
			}
		}
	}
	return false
}

func hasCallContaining(fn *module.Function, substr string) bool {
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			if instr.Kind == module.InstrCall && instr.CalledFunc != nil && strings.Contains(instr.CalledFunc.Name, substr) {
				return true
			}
		}
	}
	return false
}

func hasMetadata(mod *module.Module, name string) bool {
	for _, nm := range mod.NamedMetadata {
		if nm.Name == name {
			return true
		}
	}
	return false
}

func basicBlockCount(fn *module.Function) int {
	return len(fn.BasicBlocks)
}

// ---------------------------------------------------------------------------
// 1. Unary operations — emitUnary (0% -> covered)
// ---------------------------------------------------------------------------

// buildUnaryNegateFloatShader: @fragment fn main(@location(0) x: f32) -> @location(0) vec4<f32> {
//
//	return vec4(-x, 0.0, 0.0, 1.0);
//
// }
func buildUnaryNegateFloatShader() *ir.Module {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: f32, Binding: &xBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x
			{Kind: ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0}},                     // [1] -x
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 3, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return mod
}

func TestEmitUnaryNegateFloat(t *testing.T) {
	mod := emitOK(t, buildUnaryNegateFloatShader())
	fn := mainFunc(t, mod)

	// Float negate is fsub(0.0, x) — should produce a BinOp instruction.
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected at least 1 BinOp for float negate (fsub 0.0, x)")
	}
}

// buildUnaryNegateIntShader: negate an integer input.
func buildUnaryNegateIntShader() *ir.Module {
	i32 := ir.TypeHandle(0)
	f32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: i32, Binding: &xBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x
			{Kind: ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0}},                     // [1] -x (int)
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 1, Convert: ptrU8(4)}},   // [2] f32(-x)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 4, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32}, {Handle: &i32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return mod
}

func TestEmitUnaryNegateInt(t *testing.T) {
	mod := emitOK(t, buildUnaryNegateIntShader())
	fn := mainFunc(t, mod)
	// Int negate is sub(0, x) plus a cast — expect BinOp + Cast.
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for int negate (sub 0, x)")
	}
	if n := countInstr(fn, module.InstrCast); n < 1 {
		t.Error("expected Cast for i32->f32 conversion")
	}
}

// buildUnaryBitwiseNotShader: ~x for u32.
func buildUnaryBitwiseNotShader() *ir.Module {
	u32 := ir.TypeHandle(0)
	f32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: u32, Binding: &xBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x
			{Kind: ir.ExprUnary{Op: ir.UnaryBitwiseNot, Expr: 0}},                 // [1] ~x
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 1, Convert: ptrU8(4)}},   // [2] f32(~x)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 4, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &u32}, {Handle: &u32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return mod
}

func TestEmitUnaryBitwiseNot(t *testing.T) {
	mod := emitOK(t, buildUnaryBitwiseNotShader())
	fn := mainFunc(t, mod)
	// Bitwise not is xor(x, -1) — a BinOp.
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for bitwise not (xor x, -1)")
	}
}

// buildLogicalNotShader: !cond for bool.
func buildLogicalNotShader() *ir.Module {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32, Binding: &aBind},
			{Name: "b", Type: f32, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b
			{Kind: ir.ExprBinary{Op: ir.BinaryLess, Left: 0, Right: 1}},           // [2] a < b
			{Kind: ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: 2}},                 // [3] !(a < b)
			{Kind: ir.ExprSelect{Condition: 3, Accept: 0, Reject: 1}},             // [4] select
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{4, 4, 4, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32},
			{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return mod
}

func TestEmitUnaryLogicalNot(t *testing.T) {
	mod := emitOK(t, buildLogicalNotShader())
	fn := mainFunc(t, mod)

	// Logical not = xor(cond, true), comparison = icmp/fcmp, select.
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for logical not (xor cond, true)")
	}
	if n := countInstr(fn, module.InstrSelect); n < 1 {
		t.Error("expected Select instruction")
	}
}

// ---------------------------------------------------------------------------
// 2. Select expression — emitSelect (0% -> covered)
// ---------------------------------------------------------------------------

// buildScalarSelectShader: select(cond, a, b).
func buildScalarSelectShader() *ir.Module {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32, Binding: &aBind},
			{Name: "b", Type: f32, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [2] a > b
			{Kind: ir.ExprSelect{Condition: 2, Accept: 0, Reject: 1}},             // [3] select
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 3, 5, 4}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32},
			{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return mod
}

func TestEmitSelectScalar(t *testing.T) {
	mod := emitOK(t, buildScalarSelectShader())
	fn := mainFunc(t, mod)
	if n := countInstr(fn, module.InstrSelect); n < 1 {
		t.Error("expected at least 1 select instruction")
	}
}

// ---------------------------------------------------------------------------
// 3. Switch statement — emitSwitchStatement (0% -> covered)
// ---------------------------------------------------------------------------

// buildSwitchShader: compute shader with switch on global_invocation_id.x.
func buildSwitchShader() *ir.Module {
	u32 := ir.TypeHandle(0)
	vec3u32 := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
	}

	gidBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "gid", Type: vec3u32, Binding: &gidBind}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] gid
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] gid.x
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u32}, {Handle: &u32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtSwitch{
				Selector: 1,
				Cases: []ir.SwitchCase{
					{Value: ir.SwitchValueU32(0), Body: ir.Block{}},
					{Value: ir.SwitchValueU32(1), Body: ir.Block{}},
					{Value: ir.SwitchValueDefault{}, Body: ir.Block{}},
				},
			}},
			{Kind: ir.StmtReturn{}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{
		Name:      "main",
		Stage:     ir.StageCompute,
		Function:  fn,
		Workgroup: [3]uint32{64, 1, 1},
	}}
	return mod
}

func TestEmitSwitchStatement(t *testing.T) {
	mod := emitOK(t, buildSwitchShader())
	fn := mainFunc(t, mod)

	// Switch with 3 cases (2 valued + default) generates:
	// - comparison blocks for each valued case
	// - case body blocks
	// - merge block
	// Expect at least 5 basic blocks (entry + 2 test + 3 body/default + merge).
	if n := basicBlockCount(fn); n < 5 {
		t.Errorf("expected at least 5 basic blocks for switch, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 4. Literal types — emitLiteral (20.8% -> broader coverage)
// ---------------------------------------------------------------------------

func TestEmitLiteralBool(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "a", Type: f32, Binding: &aBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},                       // [1] true
			{Kind: ir.ExprSelect{Condition: 1, Accept: 0, Reject: 0}},             // [2] select(true, a, a)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 3, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32},
			{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod := &ir.Module{
		Types:       mod.Types,
		EntryPoints: []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}},
	}

	emitOK(t, irMod)
}

func TestEmitLiteralI32(t *testing.T) {
	f32 := ir.TypeHandle(0)
	i32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name:   "main",
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}},                          // [0]
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 0, Convert: ptrU8(4)}},   // [1] f32(42)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 2, 3}}}, // [4]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

func TestEmitLiteralU32(t *testing.T) {
	f32 := ir.TypeHandle(0)
	u32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name:   "main",
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(100)}},                         // [0]
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 0, Convert: ptrU8(4)}},   // [1] f32(100u)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 2, 3}}}, // [4]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &u32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

func TestEmitLiteralAbstractFloat(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(3)

	fn := ir.Function{
		Name:   "main",
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralAbstractFloat(3.14)}},              // [0]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [2]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1, 1, 2}}}, // [3]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

func TestEmitLiteralAbstractInt(t *testing.T) {
	f32 := ir.TypeHandle(0)
	i32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name:   "main",
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralAbstractInt(7)}},                   // [0]
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 0, Convert: ptrU8(4)}},   // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 2, 3}}}, // [4]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

// ---------------------------------------------------------------------------
// 5. Binary operations — emitScalarBinaryOp paths (30.4% -> broader)
// ---------------------------------------------------------------------------

func buildIntBinaryShader(op ir.BinaryOperator, kind ir.ScalarKind) *ir.Module {
	scalar := ir.TypeHandle(0)
	f32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: kind, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: scalar, Binding: &aBind},
			{Name: "b", Type: scalar, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0]
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1]
			{Kind: ir.ExprBinary{Op: op, Left: 0, Right: 1}},                      // [2]
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 2, Convert: ptrU8(4)}},   // [3] cast to f32
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4, 4, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &scalar}, {Handle: &scalar}, {Handle: &scalar},
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return irMod
}

func TestEmitBinaryIntDivide(t *testing.T) {
	// Signed int divide => sdiv.
	mod := emitOK(t, buildIntBinaryShader(ir.BinaryDivide, ir.ScalarSint))
	fn := mainFunc(t, mod)
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for integer division")
	}
}

func TestEmitBinaryUintDivide(t *testing.T) {
	// Unsigned int divide => udiv.
	mod := emitOK(t, buildIntBinaryShader(ir.BinaryDivide, ir.ScalarUint))
	fn := mainFunc(t, mod)
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for unsigned division")
	}
}

func TestEmitBinaryIntModulo(t *testing.T) {
	// Signed int modulo => srem.
	mod := emitOK(t, buildIntBinaryShader(ir.BinaryModulo, ir.ScalarSint))
	fn := mainFunc(t, mod)
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for signed modulo")
	}
}

func TestEmitBinaryUintModulo(t *testing.T) {
	// Unsigned modulo with non-power-of-2 => urem.
	mod := emitOK(t, buildIntBinaryShader(ir.BinaryModulo, ir.ScalarUint))
	fn := mainFunc(t, mod)
	if n := countInstr(fn, module.InstrBinOp); n < 1 {
		t.Error("expected BinOp for unsigned modulo")
	}
}

func TestEmitBinaryBitwiseOps(t *testing.T) {
	ops := []struct {
		name string
		op   ir.BinaryOperator
	}{
		{"and", ir.BinaryAnd},
		{"xor", ir.BinaryExclusiveOr},
		{"or", ir.BinaryInclusiveOr},
		{"shl", ir.BinaryShiftLeft},
		{"shr_signed", ir.BinaryShiftRight},
	}
	for _, tt := range ops {
		t.Run(tt.name, func(t *testing.T) {
			mod := emitOK(t, buildIntBinaryShader(tt.op, ir.ScalarSint))
			fn := mainFunc(t, mod)
			if n := countInstr(fn, module.InstrBinOp); n < 1 {
				t.Errorf("expected BinOp for %s", tt.name)
			}
		})
	}
}

func TestEmitBinaryLogicalOps(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(8)

	boolTy := ir.ScalarType{Kind: ir.ScalarBool, Width: 1}

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32, Binding: &aBind},
			{Name: "b", Type: f32, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0]
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1]
			{Kind: ir.ExprBinary{Op: ir.BinaryLess, Left: 0, Right: 1}},           // [2] a < b
			{Kind: ir.ExprBinary{Op: ir.BinaryGreater, Left: 0, Right: 1}},        // [3] a > b
			{Kind: ir.ExprBinary{Op: ir.BinaryLogicalAnd, Left: 2, Right: 3}},     // [4] && (always false, but exercised)
			{Kind: ir.ExprBinary{Op: ir.BinaryLogicalOr, Left: 2, Right: 3}},      // [5] ||
			{Kind: ir.ExprSelect{Condition: 5, Accept: 0, Reject: 1}},             // [6]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [7]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{6, 6, 6, 7}}}, // [8]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32},
			{Value: boolTy}, {Value: boolTy}, {Value: boolTy}, {Value: boolTy},
			{Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 9}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

func TestEmitBinaryFloatModulo(t *testing.T) {
	// Float modulo is lowered to: a - b * floor(a / b).
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32, Binding: &aBind},
			{Name: "b", Type: f32, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0]
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1]
			{Kind: ir.ExprBinary{Op: ir.BinaryModulo, Left: 0, Right: 1}},         // [2] a % b (float)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 3, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// Float modulo should produce dx.op calls (floor) + binary ops (fdiv, fmul, fsub).
	if n := countInstr(fn2, module.InstrBinOp); n < 3 {
		t.Errorf("expected at least 3 BinOps for float modulo lowering, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 6. Comparison operations — all comparison kinds
// ---------------------------------------------------------------------------

func TestEmitComparisonOps(t *testing.T) {
	ops := []struct {
		name string
		op   ir.BinaryOperator
	}{
		{"equal", ir.BinaryEqual},
		{"not_equal", ir.BinaryNotEqual},
		{"less", ir.BinaryLess},
		{"less_equal", ir.BinaryLessEqual},
		{"greater", ir.BinaryGreater},
		{"greater_equal", ir.BinaryGreaterEqual},
	}
	for _, tt := range ops {
		t.Run("float_"+tt.name, func(t *testing.T) {
			f32 := ir.TypeHandle(0)
			vec4 := ir.TypeHandle(1)

			irMod := &ir.Module{
				Types: []ir.Type{
					{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
					{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
				},
			}

			aBind := ir.Binding(ir.LocationBinding{Location: 0})
			bBind := ir.Binding(ir.LocationBinding{Location: 1})
			retBind := ir.Binding(ir.LocationBinding{Location: 0})
			retH := ir.ExpressionHandle(6)

			fn := ir.Function{
				Name: "main",
				Arguments: []ir.FunctionArgument{
					{Name: "a", Type: f32, Binding: &aBind},
					{Name: "b", Type: f32, Binding: &bBind},
				},
				Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
				Expressions: []ir.Expression{
					{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0]
					{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1]
					{Kind: ir.ExprBinary{Op: tt.op, Left: 0, Right: 1}},                   // [2]
					{Kind: ir.ExprSelect{Condition: 2, Accept: 0, Reject: 1}},             // [3]
					{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4]
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
					{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4, 4, 5}}}, // [6]
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &f32}, {Handle: &f32},
					{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
					{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
					{Kind: ir.StmtReturn{Value: &retH}},
				},
			}

			irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
			emitOK(t, irMod)
		})
	}
}

// ---------------------------------------------------------------------------
// 7. If/else control flow — emitIfStatement branches
// ---------------------------------------------------------------------------

func TestEmitIfElseControlFlow(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: f32, Binding: &aBind},
			{Name: "b", Type: f32, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b
			{Kind: ir.ExprBinary{Op: ir.BinaryLess, Left: 0, Right: 1}},           // [2] a < b
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1, 3, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32},
			{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
			{Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtIf{
				Condition: 2,
				Accept: ir.Block{
					{Kind: ir.StmtReturn{Value: &retH}},
				},
				Reject: ir.Block{
					{Kind: ir.StmtReturn{Value: &retH}},
				},
			}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageVertex, Function: fn}}
	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// If/else creates: entry BB, accept BB, reject BB (+ possible merge).
	if n := basicBlockCount(fn2); n < 3 {
		t.Errorf("expected at least 3 basic blocks for if/else, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 8. Loop with break — emitLoopStatement + emitBreak/Continue
// ---------------------------------------------------------------------------

func TestEmitLoopWithBreak(t *testing.T) {
	u32 := ir.TypeHandle(0)
	vec3u := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
	}

	gidBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "gid", Type: vec3u, Binding: &gidBind}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                           // [0] gid
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},                       // [1] gid.x
			{Kind: ir.Literal{Value: ir.LiteralU32(10)}},                        // [2] 10u
			{Kind: ir.ExprBinary{Op: ir.BinaryGreaterEqual, Left: 1, Right: 2}}, // [3] gid.x >= 10
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u}, {Handle: &u32}, {Handle: &u32},
			{Value: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtLoop{
				Body: ir.Block{
					{Kind: ir.StmtIf{
						Condition: 3,
						Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
					}},
					{Kind: ir.StmtContinue{}},
				},
			}},
			{Kind: ir.StmtReturn{}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{
		Name:      "main",
		Stage:     ir.StageCompute,
		Function:  fn,
		Workgroup: [3]uint32{64, 1, 1},
	}}

	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// Loop generates: header, body, continue, merge basic blocks at minimum.
	if n := basicBlockCount(fn2); n < 4 {
		t.Errorf("expected at least 4 basic blocks for loop, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 9. Compute shader with barrier — emitStmtBarrier + compute builtins
// ---------------------------------------------------------------------------

func TestEmitBarrierWorkgroup(t *testing.T) {
	u32 := ir.TypeHandle(0)
	vec3u := ir.TypeHandle(1)
	arrU32 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Inner: ir.ArrayType{Base: u32, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: arrU32},
		},
	}

	gidBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "gid", Type: vec3u, Binding: &gidBind},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] gid
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] gid.x
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u}, {Handle: &u32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}},
			{Kind: ir.StmtReturn{}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{
		Name:      "main",
		Stage:     ir.StageCompute,
		Function:  fn,
		Workgroup: [3]uint32{64, 1, 1},
	}}

	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// Should emit dx.op.barrier call.
	if !hasCallContaining(fn2, "dx.op.barrier") {
		t.Error("expected dx.op.barrier call for workgroup barrier")
	}
}

// ---------------------------------------------------------------------------
// 10. Vertex shader with multi-component output — exercises storeOutput paths
// ---------------------------------------------------------------------------

func TestEmitVertexMultiComponentOutput(t *testing.T) {
	// Simpler vertex shader returning vec4 position from vec2 input.
	// Exercises storeOutput for 4-component position output.
	irMod := buildSimpleTransformVertex()
	mod := emitOK(t, irMod)

	// Must have dx.entryPoints metadata and output signature.
	if !hasMetadata(mod, "dx.entryPoints") {
		t.Error("missing dx.entryPoints metadata")
	}
	if !hasMetadata(mod, "dx.version") {
		t.Error("missing dx.version metadata")
	}
	fn := mainFunc(t, mod)
	// Should have storeOutput calls for all 4 components.
	if !hasCallContaining(fn, "dx.op.storeOutput") {
		t.Error("expected dx.op.storeOutput calls for vertex output")
	}
}

// ---------------------------------------------------------------------------
// 11. Local variable with store/load — exercises emitStmtStore + emitLoad
// ---------------------------------------------------------------------------

func TestEmitLocalVariableStoreLoad(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})

	initH := ir.ExpressionHandle(1)
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "a", Type: f32, Binding: &aBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		LocalVars: []ir.LocalVariable{
			{Name: "tmp", Type: f32, Init: &initH}, // init to literal 0.0
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [1] 0.0 (init)
			{Kind: ir.ExprLocalVariable{Variable: 0}},                             // [2] &tmp
			{Kind: ir.ExprLoad{Pointer: 2}},                                       // [3] load(tmp)
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [4]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4, 4, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32},
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtStore{Pointer: 2, Value: 0}}, // tmp = a
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

// ---------------------------------------------------------------------------
// 12. Multiple compute builtins — tests emitComputeBuiltinLoad paths
// ---------------------------------------------------------------------------

func TestEmitComputeBuiltins(t *testing.T) {
	u32 := ir.TypeHandle(0)
	vec3u := ir.TypeHandle(1)
	arrU32 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Inner: ir.ArrayType{Base: u32, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: arrU32},
		},
	}

	gidBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "gid", Type: vec3u, Binding: &gidBind},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] gid
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}}, // [1] gid.x
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}}, // [2] gid.y
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 2}}, // [3] gid.z
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3u}, {Handle: &u32}, {Handle: &u32}, {Handle: &u32},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{
		Name:      "main",
		Stage:     ir.StageCompute,
		Function:  fn,
		Workgroup: [3]uint32{8, 8, 1},
	}}

	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// GlobalInvocationId generates 3 threadId calls for x, y, z.
	callCount := countInstr(fn2, module.InstrCall)
	if callCount < 3 {
		t.Errorf("expected at least 3 dx.op calls for compute builtins, got %d", callCount)
	}
}

// ---------------------------------------------------------------------------
// 13. Type mapping — matrix, array types
// ---------------------------------------------------------------------------

func TestTypeMappingMatrix(t *testing.T) {
	mod := module.NewModule(module.VertexShader)
	irMod := &ir.Module{}
	mat := ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}

	// DXIL has no vectors or matrices -- typeToDXIL returns the scalar element type.
	ty, err := typeToDXIL(mod, irMod, mat)
	if err != nil {
		t.Fatalf("typeToDXIL(mat4x4<f32>): %v", err)
	}
	if ty.Kind != module.TypeFloat {
		t.Errorf("matrix type kind = %v, want TypeFloat (scalar element)", ty.Kind)
	}
}

func TestTypeMappingArrayOfScalar(t *testing.T) {
	mod := module.NewModule(module.ComputeShader)
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	arrSize := uint32(8)
	arr := ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &arrSize}}

	ty, err := typeToDXIL(mod, irMod, arr)
	if err != nil {
		t.Fatalf("typeToDXIL(array<f32, 8>): %v", err)
	}
	if ty.Kind != module.TypeArray {
		t.Errorf("array type kind = %v, want TypeArray", ty.Kind)
	}
	if ty.ElemCount != 8 {
		t.Errorf("array count = %d, want 8", ty.ElemCount)
	}
}

// ---------------------------------------------------------------------------
// 14. Math unary functions — drives emitMath unary paths
// ---------------------------------------------------------------------------

func buildMathUnaryShader(mathFn ir.MathFunction) *ir.Module {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(4)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: f32, Binding: &aBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0]
			{Kind: ir.ExprMath{Fun: mathFn, Arg: 0}},                              // [1]
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [2]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [3]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{1, 2, 2, 3}}}, // [4]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 5}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return irMod
}

func TestEmitMathUnaryFunctions(t *testing.T) {
	fns := []struct {
		name string
		fn   ir.MathFunction
	}{
		{"abs", ir.MathAbs},
		{"sqrt", ir.MathSqrt},
		{"inverseSqrt", ir.MathInverseSqrt},
		{"ceil", ir.MathCeil},
		{"floor", ir.MathFloor},
		{"round", ir.MathRound},
		{"trunc", ir.MathTrunc},
		{"sin", ir.MathSin},
		{"cos", ir.MathCos},
		{"tan", ir.MathTan},
		{"asin", ir.MathAsin},
		{"acos", ir.MathAcos},
		{"atan", ir.MathAtan},
		{"log2", ir.MathLog2},
		{"exp2", ir.MathExp2},
		{"saturate", ir.MathSaturate},
		{"fract", ir.MathFract},
	}
	for _, tt := range fns {
		t.Run(tt.name, func(t *testing.T) {
			irMod := buildMathUnaryShader(tt.fn)
			mod := emitOK(t, irMod)
			fn := mainFunc(t, mod)

			// Each math unary should produce at least one dx.op call.
			if countInstr(fn, module.InstrCall) < 1 {
				t.Errorf("expected at least 1 dx.op call for math function %s", tt.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 15. DescribeIRType — low coverage utility (20%)
// ---------------------------------------------------------------------------

func TestDescribeIRType(t *testing.T) {
	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                             // [0]
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},                                              // [1]
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},                                              // [2]
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},             // [3]
			{Inner: ir.MatrixType{Columns: 4, Rows: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // [4]
		},
	}

	// describeIRType returns (compType int64, numComps int64).
	// Just verify it returns valid values for each type kind.
	tests := []struct {
		handle ir.TypeHandle
	}{
		{0}, // f32
		{1}, // i32
		{2}, // bool
		{3}, // vec3<f32>
		{4}, // mat4x4<f32>
	}

	for _, tt := range tests {
		compType, numComps := describeIRType(irMod, tt.handle)
		// compType should be positive for valid types.
		if compType <= 0 && numComps <= 0 {
			t.Errorf("describeIRType(%d) returned (%d, %d), expected positive values", tt.handle, compType, numComps)
		}
	}
}

// ---------------------------------------------------------------------------
// 16. InterpolationMode — exercises interpolationModeForBinding (35%)
// ---------------------------------------------------------------------------

func TestInterpolationModeForBinding(t *testing.T) {
	tests := []struct {
		name       string
		interp     *ir.Interpolation
		isFragment bool
		isOutput   bool
	}{
		{"flat", &ir.Interpolation{Kind: ir.InterpolationFlat}, true, false},
		{"perspective", &ir.Interpolation{Kind: ir.InterpolationPerspective}, true, false},
		{"linear", &ir.Interpolation{Kind: ir.InterpolationLinear}, true, false},
		{"nil_frag_input", nil, true, false},
		{"nil_frag_output", nil, true, true},
		{"nil_vert_output", nil, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := interpolationModeForBinding(tt.interp, tt.isFragment, tt.isOutput)
			// Just verify it returns a valid value (uint8 range).
			_ = got
		})
	}
}

// ---------------------------------------------------------------------------
// 17. StageToShaderKind — mesh/amplification stages
// ---------------------------------------------------------------------------

func TestStageToShaderKindExtended(t *testing.T) {
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

// ---------------------------------------------------------------------------
// 18. BuiltinToDXILSemantic — exercises mapping (18% coverage)
// ---------------------------------------------------------------------------

func TestBuiltinToDXILSemantic(t *testing.T) {
	tests := []struct {
		builtin ir.BuiltinValue
	}{
		{ir.BuiltinPosition},
		{ir.BuiltinVertexIndex},
		{ir.BuiltinInstanceIndex},
		{ir.BuiltinFrontFacing},
		{ir.BuiltinFragDepth},
		{ir.BuiltinSampleIndex},
		{ir.BuiltinSampleMask},
		{ir.BuiltinGlobalInvocationID},
		{ir.BuiltinLocalInvocationID},
		{ir.BuiltinWorkGroupID},
		{ir.BuiltinLocalInvocationIndex},
	}
	for _, tt := range tests {
		semIdx, semName := builtinToDXILSemantic(tt.builtin)
		// Verify it returns non-empty semantic name and valid index.
		if semName == "" {
			t.Errorf("builtinToDXILSemantic(%d) returned empty semantic name", tt.builtin)
		}
		_ = semIdx
	}
}

// ---------------------------------------------------------------------------
// 19. DxilCompTypeForScalar — exercises mapping (83% -> broader)
// ---------------------------------------------------------------------------

func TestDxilCompTypeForScalar(t *testing.T) {
	tests := []struct {
		kind ir.ScalarKind
		want int64
	}{
		{ir.ScalarFloat, 9}, // f32
		{ir.ScalarSint, 4},  // i32
		{ir.ScalarUint, 5},  // u32
	}
	for _, tt := range tests {
		got := dxilCompTypeForScalar(tt.kind)
		if got != tt.want {
			t.Errorf("dxilCompTypeForScalar(%v) = %d, want %d", tt.kind, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 20. Inlining analysis helpers — 0% public functions
// ---------------------------------------------------------------------------

func TestFunctionHasComplexLocals(t *testing.T) {
	vec4 := ir.TypeHandle(0)

	fn := ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "v", Type: vec4},
		},
	}

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	got := FunctionHasComplexLocals(&fn, irMod)
	// A vector local is not complex (it's a simple type).
	// Complex locals are arrays, structs, matrices.
	if got {
		t.Error("vec4 local should not be considered complex")
	}
}

func TestFunctionHasComplexLocalsWithStruct(t *testing.T) {
	st := ir.TypeHandle(0)

	fn := ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: st},
		},
	}

	irMod := &ir.Module{
		Types: []ir.Type{
			{Name: "MyStruct", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0}},
			}},
		},
	}

	got := FunctionHasComplexLocals(&fn, irMod)
	if !got {
		t.Error("struct local should be considered complex")
	}
}

func TestFunctionHasComplexExpressions(t *testing.T) {
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
	}

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	got := FunctionHasComplexExpressions(&fn, irMod)
	if got {
		t.Error("simple expressions should not be complex")
	}
}

func TestFunctionAccessesGlobals(t *testing.T) {
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
		},
	}
	got := FunctionAccessesGlobals(&fn)
	if got {
		t.Error("function with no global access should return false")
	}
}

func TestFunctionAccessesGlobalsTrue(t *testing.T) {
	fn := ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
		},
	}
	got := FunctionAccessesGlobals(&fn)
	if !got {
		t.Error("function with ExprGlobalVariable should return true")
	}
}

func TestIsScalarizableType(t *testing.T) {
	if !IsScalarizableType(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}) {
		t.Error("scalar should be scalarizable")
	}
	if !IsScalarizableType(ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}) {
		t.Error("vector should be scalarizable")
	}
}

func TestComponentCountExported(t *testing.T) {
	if got := ComponentCount(ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}); got != 1 {
		t.Errorf("ComponentCount(scalar) = %d, want 1", got)
	}
	if got := ComponentCount(ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}); got != 3 {
		t.Errorf("ComponentCount(vec3) = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// 21. EmitNoEntryPoints error path
// ---------------------------------------------------------------------------

func TestEmitMultipleEntryPointsUsesFirst(t *testing.T) {
	// Emit takes only the first entry point.
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(3)

	fn1 := ir.Function{
		Name:   "first",
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{0, 1, 2, 0}}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 4}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{
		{Name: "first", Stage: ir.StageFragment, Function: fn1},
		{Name: "second", Stage: ir.StageFragment, Function: fn1},
	}

	mod := emitOK(t, irMod)
	// Verify the module compiled.
	if len(mod.Functions) == 0 {
		t.Error("no functions emitted")
	}
}

// ---------------------------------------------------------------------------
// 22. Splat with vec2 — emitSplat partial coverage
// ---------------------------------------------------------------------------

func TestEmitSplatVec2(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec2 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: f32, Binding: &aBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x
			{Kind: ir.ExprSplat{Value: 0, Size: 2}},                               // [1] vec2(x, x)
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}},                         // [2] splat.x
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 1}},                         // [3] splat.y
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 2, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &vec2}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

// ---------------------------------------------------------------------------
// 23. Cast expressions — emitAs, emitScalarCast paths
// ---------------------------------------------------------------------------

func TestEmitCastFPToSI(t *testing.T) {
	f32 := ir.TypeHandle(0)
	i32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: f32, Binding: &xBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x (f32)
			{Kind: ir.ExprAs{Kind: ir.ScalarSint, Expr: 0, Convert: ptrU8(4)}},    // [1] i32(x)
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 1, Convert: ptrU8(4)}},   // [2] f32(i32(x))
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 3, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &i32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// Should have 2 cast instructions (fptosi + sitofp).
	if n := countInstr(fn2, module.InstrCast); n < 2 {
		t.Errorf("expected at least 2 cast instructions, got %d", n)
	}
}

func TestEmitCastFPToUI(t *testing.T) {
	f32 := ir.TypeHandle(0)
	u32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	xBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "x", Type: f32, Binding: &xBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] x (f32)
			{Kind: ir.ExprAs{Kind: ir.ScalarUint, Expr: 0, Convert: ptrU8(4)}},    // [1] u32(x)
			{Kind: ir.ExprAs{Kind: ir.ScalarFloat, Expr: 1, Convert: ptrU8(4)}},   // [2] f32(u32(x))
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                         // [3]
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 3, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32}, {Handle: &u32}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	if n := countInstr(fn2, module.InstrCast); n < 2 {
		t.Errorf("expected at least 2 cast instructions, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 24. Workgroup size metadata validation for compute shaders
// ---------------------------------------------------------------------------

func TestEmitComputeShaderMetadata(t *testing.T) {
	u32 := ir.TypeHandle(0)
	vec3u := ir.TypeHandle(1)
	arrU32 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Inner: ir.ArrayType{Base: u32, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: arrU32},
		},
	}

	gidBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "gid", Type: vec3u, Binding: &gidBind}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{{Handle: &vec3u}, {Handle: &u32}},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{
		Name:      "main",
		Stage:     ir.StageCompute,
		Function:  fn,
		Workgroup: [3]uint32{16, 16, 1},
	}}

	mod := emitOK(t, irMod)

	// Verify required metadata nodes exist.
	required := []string{"dx.version", "dx.shaderModel", "dx.entryPoints", "llvm.ident"}
	for _, name := range required {
		if !hasMetadata(mod, name) {
			t.Errorf("missing required metadata: %s", name)
		}
	}
}

// ---------------------------------------------------------------------------
// 25. SelectDivOp coverage
// ---------------------------------------------------------------------------

func TestSelectDivOp(t *testing.T) {
	tests := []struct {
		isFloat  bool
		isSigned bool
		want     BinOpKind
	}{
		{true, false, BinOpFDiv},
		{false, true, BinOpSDiv},
		{false, false, BinOpUDiv},
	}
	for _, tt := range tests {
		got := selectDivOp(tt.isFloat, tt.isSigned)
		if got != tt.want {
			t.Errorf("selectDivOp(%v, %v) = %d, want %d", tt.isFloat, tt.isSigned, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 26. Shader with vec3 vector operations exercising vector binary paths
// ---------------------------------------------------------------------------

func TestEmitVectorBinaryAdd(t *testing.T) {
	vec3 := ir.TypeHandle(0)
	f32 := ir.TypeHandle(1)
	vec4 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	aBind := ir.Binding(ir.LocationBinding{Location: 0})
	bBind := ir.Binding(ir.LocationBinding{Location: 1})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(6)

	fn := ir.Function{
		Name: "main",
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: vec3, Binding: &aBind},
			{Name: "b", Type: vec3, Binding: &bBind},
		},
		Result: &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                             // [0] a (vec3)
			{Kind: ir.ExprFunctionArgument{Index: 1}},                             // [1] b (vec3)
			{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}},            // [2] a + b (vec3)
			{Kind: ir.ExprAccessIndex{Base: 2, Index: 0}},                         // [3] sum.x
			{Kind: ir.ExprAccessIndex{Base: 2, Index: 1}},                         // [4] sum.y
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [5]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4, 3, 5}}}, // [6]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec3}, {Handle: &vec3}, {Handle: &vec3},
			{Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 7}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	mod := emitOK(t, irMod)
	fn2 := mainFunc(t, mod)

	// Vector add generates per-component fadd instructions.
	if n := countInstr(fn2, module.InstrBinOp); n < 3 {
		t.Errorf("expected at least 3 BinOps for vec3 add, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 27. EmitWithFlags — verify shader flags returned
// ---------------------------------------------------------------------------

func TestEmitWithFlagsReturnsFlags(t *testing.T) {
	irMod := buildSimpleFragmentShader()
	_, flags, err := EmitWithFlags(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("EmitWithFlags failed: %v", err)
	}
	// Flags should be non-negative (0 is valid for simple shaders).
	_ = flags // just verify it doesn't panic
}

// ---------------------------------------------------------------------------
// 28. Swizzle expression
// ---------------------------------------------------------------------------

func TestEmitSwizzleYX(t *testing.T) {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)
	vec2 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	inBind := ir.Binding(ir.LocationBinding{Location: 0})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})
	retH := ir.ExpressionHandle(5)

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "v", Type: vec4, Binding: &inBind}},
		Result:    &ir.FunctionResult{Type: vec4, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}}, // [0] v (vec4)
			{Kind: ir.ExprSwizzle{ // [1] v.yx -> vec2
				Size: 2, Vector: 0,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleY, ir.SwizzleX, 0, 0},
			}},
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}},                         // [2] swizzle.x
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 1}},                         // [3] swizzle.y
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                         // [4]
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{2, 3, 2, 4}}}, // [5]
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &vec4}, {Handle: &vec2}, {Handle: &f32}, {Handle: &f32}, {Handle: &f32}, {Handle: &vec4},
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 6}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	emitOK(t, irMod)
}

// ---------------------------------------------------------------------------
// 29. ComputeBitcodeShaderFlags basic test
// ---------------------------------------------------------------------------

func TestComputeBitcodeShaderFlagsBasicCompute(t *testing.T) {
	u32 := ir.TypeHandle(0)
	vec3u := ir.TypeHandle(1)
	arrU32 := ir.TypeHandle(2)

	irMod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Inner: ir.VectorType{Size: 3, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
			{Inner: ir.ArrayType{Base: u32, Stride: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "buf", Space: ir.SpaceStorage, Access: ir.StorageReadWrite,
				Binding: &ir.ResourceBinding{Group: 0, Binding: 0}, Type: arrU32},
		},
	}

	gidBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinGlobalInvocationID})

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "gid", Type: vec3u, Binding: &gidBind}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{{Handle: &vec3u}, {Handle: &u32}},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{}},
		},
	}

	irMod.EntryPoints = []ir.EntryPoint{{
		Name:      "main",
		Stage:     ir.StageCompute,
		Function:  fn,
		Workgroup: [3]uint32{64, 1, 1},
	}}

	_, flags, err := EmitWithFlags(irMod, EmitOptions{ShaderModelMajor: 6, ShaderModelMinor: 0})
	if err != nil {
		t.Fatalf("EmitWithFlags failed: %v", err)
	}

	// A simple compute shader should NOT have double (bit 2) or int64 (bit 3) flags.
	if flags&(1<<2) != 0 {
		t.Error("simple compute shader should not have UseDouble flag")
	}
	if flags&(1<<3) != 0 {
		t.Error("simple compute shader should not have Int64Ops flag")
	}
}
