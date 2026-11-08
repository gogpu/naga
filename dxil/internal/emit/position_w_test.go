// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package emit

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// buildFragmentPositionWShader builds
//
//	@fragment fn main(@builtin(position) pos: vec4<f32>) -> @location(0) f32 {
//	    return pos.w;
//	}
//
// whose only consumed component of the position input is w.
func buildFragmentPositionWShader() *ir.Module {
	f32 := ir.TypeHandle(0)
	vec4 := ir.TypeHandle(1)
	retH := ir.ExpressionHandle(1)

	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	posBind := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	retBind := ir.Binding(ir.LocationBinding{Location: 0})

	fn := ir.Function{
		Name:      "main",
		Arguments: []ir.FunctionArgument{{Name: "pos", Type: vec4, Binding: &posBind}},
		Result:    &ir.FunctionResult{Type: f32, Binding: &retBind},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},     // [0] pos
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 3}}, // [1] pos.w
		},
		Body: []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtReturn{Value: &retH}},
		},
	}

	mod.EntryPoints = []ir.EntryPoint{{Name: "main", Stage: ir.StageFragment, Function: fn}}
	return mod
}

// TestEmitFragmentPositionWInverted verifies that a fragment shader reading
// SV_Position.w receives 1.0 / w, as WGSL defines fragment position.w. DXC's
// reference DXIL for `float4(pos.xyz, 1.0 / pos.w)` is exactly
// `%1 = loadInput(..., i8 3)` followed by `%2 = fdiv fast float 1.0, %1`,
// with only the w component loaded.
func TestEmitFragmentPositionWInverted(t *testing.T) {
	mod := emitOK(t, buildFragmentPositionWShader())
	fn := mainFunc(t, mod)

	var loads []*module.Instruction
	var div *module.Instruction
	var stores []*module.Instruction
	for _, bb := range fn.BasicBlocks {
		for _, instr := range bb.Instructions {
			switch {
			case instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				strings.Contains(instr.CalledFunc.Name, "dx.op.loadInput"):
				loads = append(loads, instr)
			case instr.Kind == module.InstrCall && instr.CalledFunc != nil &&
				strings.Contains(instr.CalledFunc.Name, "dx.op.storeOutput"):
				stores = append(stores, instr)
			case instr.Kind == module.InstrBinOp && instr.Operands[2] == int(BinOpFDiv):
				if div != nil {
					t.Fatalf("more than one fdiv: %d and %d", div.ValueID, instr.ValueID)
				}
				div = instr
			}
		}
	}

	// Only pos.w is read, so component-level DCE keeps a single loadInput,
	// as DXC's ADCE does.
	if len(loads) != 1 {
		t.Fatalf("want 1 loadInput for the w component, got %d", len(loads))
	}
	if div == nil {
		t.Fatal("no fdiv: fragment position.w is not inverted to 1.0 / w")
	}
	if div.Operands[1] != loads[0].ValueID {
		t.Errorf("fdiv divides value %d, want the loaded w %d", div.Operands[1], loads[0].ValueID)
	}
	if len(stores) != 1 {
		t.Fatalf("want 1 storeOutput, got %d", len(stores))
	}
	if got := stores[0].Operands[len(stores[0].Operands)-1]; got != div.ValueID {
		t.Errorf("storeOutput stores value %d, want the inverted w %d", got, div.ValueID)
	}
}

// TestInvertFragmentPositionW verifies the inversion is applied to the w of a
// fragment position input only: a position output (a vertex shader's clip
// position), another builtin, and a non-fragment stage stay untouched.
func TestInvertFragmentPositionW(t *testing.T) {
	position := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinPosition})
	other := ir.Binding(ir.BuiltinBinding{Builtin: ir.BuiltinFrontFacing})

	tests := []struct {
		name    string
		binding *ir.Binding
		stage   ir.ShaderStage
		invert  bool
	}{
		{"fragment position", &position, ir.StageFragment, true},
		{"vertex position output", &position, ir.StageVertex, false},
		{"fragment other builtin", &other, ir.StageFragment, false},
		{"no binding", nil, ir.StageFragment, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Emitter{
				constMap:    make(map[int]*module.Constant),
				intConsts:   make(map[int64]int),
				floatConsts: make(map[uint64]int),
				undefID:     -1,
			}
			e.mod = &module.Module{}
			e.currentBB = &module.BasicBlock{}

			w := e.getIntConstID(7)
			out := e.invertFragmentPositionW(tt.binding, tt.stage, w)

			if !tt.invert {
				if out != w {
					t.Errorf("value changed to %d, want the original %d", out, w)
				}
				if len(e.currentBB.Instructions) != 0 {
					t.Errorf("emitted %d instructions, want none", len(e.currentBB.Instructions))
				}
				return
			}
			if out == w {
				t.Fatal("value not inverted")
			}
			if len(e.currentBB.Instructions) != 1 {
				t.Fatalf("want 1 instruction, got %d", len(e.currentBB.Instructions))
			}
			div := e.currentBB.Instructions[0]
			if div.Kind != module.InstrBinOp || div.Operands[2] != int(BinOpFDiv) {
				t.Fatalf("want fdiv, got %v op %d", div.Kind, div.Operands[2])
			}
			if div.Operands[1] != w {
				t.Errorf("divides %d, want w %d", div.Operands[1], w)
			}
			one, ok := e.constMap[div.Operands[0]]
			if !ok {
				t.Fatalf("numerator %d is not a constant", div.Operands[0])
			}
			if one.ConstType == nil || one.ConstType.Kind != module.TypeFloat || one.FloatValue != 1.0 {
				t.Errorf("numerator is %v %v, want float 1.0", one.ConstType, one.FloatValue)
			}
		})
	}
}
