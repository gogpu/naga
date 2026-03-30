package spirv

import "testing"

func TestNewBlock(t *testing.T) {
	block := NewBlock(42)
	if block.LabelID != 42 {
		t.Errorf("LabelID = %d, want 42", block.LabelID)
	}
	if len(block.Body) != 0 {
		t.Errorf("Body length = %d, want 0", len(block.Body))
	}

	// Push instructions and verify order.
	inst1 := Instruction{Opcode: OpStore, Words: []uint32{1, 2}}
	inst2 := Instruction{Opcode: OpLoad, Words: []uint32{3, 4, 5}}
	block.Push(inst1)
	block.Push(inst2)

	if len(block.Body) != 2 {
		t.Fatalf("Body length = %d, want 2", len(block.Body))
	}
	if block.Body[0].Opcode != OpStore {
		t.Errorf("Body[0].Opcode = %d, want OpStore (%d)", block.Body[0].Opcode, OpStore)
	}
	if block.Body[1].Opcode != OpLoad {
		t.Errorf("Body[1].Opcode = %d, want OpLoad (%d)", block.Body[1].Opcode, OpLoad)
	}
}

func TestFunctionBuilder_Consume(t *testing.T) {
	var fb FunctionBuilder

	block := NewBlock(10)
	block.Push(Instruction{Opcode: OpStore, Words: []uint32{1, 2}})

	terminator := Instruction{Opcode: OpReturn, Words: nil}
	fb.Consume(block, terminator)

	if len(fb.Blocks) != 1 {
		t.Fatalf("Blocks length = %d, want 1", len(fb.Blocks))
	}

	tb := fb.Blocks[0]
	if tb.LabelID != 10 {
		t.Errorf("TerminatedBlock.LabelID = %d, want 10", tb.LabelID)
	}
	if len(tb.Body) != 2 {
		t.Fatalf("TerminatedBlock.Body length = %d, want 2 (1 instruction + terminator)", len(tb.Body))
	}
	if tb.Body[0].Opcode != OpStore {
		t.Errorf("Body[0].Opcode = %d, want OpStore (%d)", tb.Body[0].Opcode, OpStore)
	}
	if tb.Body[1].Opcode != OpReturn {
		t.Errorf("Body[1].Opcode = %d, want OpReturn (%d)", tb.Body[1].Opcode, OpReturn)
	}
}

func TestFunctionBuilder_ToInstructions(t *testing.T) {
	var fb FunctionBuilder

	fb.Signature = Instruction{Opcode: OpFunction, Words: []uint32{1, 0, 2}}
	fb.Parameters = []Instruction{
		{Opcode: OpFunctionParameter, Words: []uint32{3, 4}},
	}
	fb.Variables = []Instruction{
		{Opcode: OpVariable, Words: []uint32{5, 6, 7}},
	}

	// Block 0: entry block
	block0 := NewBlock(100)
	block0.Push(Instruction{Opcode: OpStore, Words: []uint32{6, 4}})
	fb.Consume(block0, Instruction{Opcode: OpBranch, Words: []uint32{200}})

	// Block 1: second block
	block1 := NewBlock(200)
	block1.Push(Instruction{Opcode: OpLoad, Words: []uint32{8, 9, 6}})
	fb.Consume(block1, Instruction{Opcode: OpReturn, Words: nil})

	result := fb.ToInstructions()

	// Expected order:
	// [0] OpFunction (signature)
	// [1] OpFunctionParameter
	// [2] OpLabel 100 (block 0)
	// [3] OpVariable (local, first block only)
	// [4] OpStore (block 0 body)
	// [5] OpBranch (block 0 terminator)
	// [6] OpLabel 200 (block 1)
	// [7] OpLoad (block 1 body)
	// [8] OpReturn (block 1 terminator)
	// [9] OpFunctionEnd

	expected := []struct {
		opcode  OpCode
		labelID uint32 // only checked for OpLabel
	}{
		{OpFunction, 0},
		{OpFunctionParameter, 0},
		{OpLabel, 100},
		{OpVariable, 0},
		{OpStore, 0},
		{OpBranch, 0},
		{OpLabel, 200},
		{OpLoad, 0},
		{OpReturn, 0},
		{OpFunctionEnd, 0},
	}

	if len(result) != len(expected) {
		t.Fatalf("result length = %d, want %d", len(result), len(expected))
	}

	for i, want := range expected {
		got := result[i]
		if got.Opcode != want.opcode {
			t.Errorf("result[%d].Opcode = %d, want %d", i, got.Opcode, want.opcode)
		}
		if want.opcode == OpLabel {
			if len(got.Words) < 1 || got.Words[0] != want.labelID {
				labelVal := uint32(0)
				if len(got.Words) > 0 {
					labelVal = got.Words[0]
				}
				t.Errorf("result[%d] OpLabel ID = %d, want %d", i, labelVal, want.labelID)
			}
		}
	}
}

func TestFunctionBuilder_VariablesInFirstBlock(t *testing.T) {
	var fb FunctionBuilder

	fb.Signature = Instruction{Opcode: OpFunction, Words: []uint32{1, 0, 2}}
	fb.Variables = []Instruction{
		{Opcode: OpVariable, Words: []uint32{10, 11, 7}},
		{Opcode: OpVariable, Words: []uint32{12, 13, 7}},
	}

	// Block 0
	block0 := NewBlock(100)
	fb.Consume(block0, Instruction{Opcode: OpBranch, Words: []uint32{200}})

	// Block 1
	block1 := NewBlock(200)
	fb.Consume(block1, Instruction{Opcode: OpReturn, Words: nil})

	// Block 2
	block2 := NewBlock(300)
	fb.Consume(block2, Instruction{Opcode: OpReturn, Words: nil})

	result := fb.ToInstructions()

	// Count OpVariable occurrences and verify they appear only after the first OpLabel.
	variableCount := 0
	firstLabelSeen := false
	secondLabelSeen := false
	for _, inst := range result {
		if inst.Opcode == OpLabel {
			if !firstLabelSeen {
				firstLabelSeen = true
			} else {
				secondLabelSeen = true
			}
		}
		if inst.Opcode == OpVariable {
			variableCount++
			if !firstLabelSeen {
				t.Error("OpVariable appears before first OpLabel")
			}
			if secondLabelSeen {
				t.Error("OpVariable appears after second OpLabel — should only be in first block")
			}
		}
	}
	if variableCount != 2 {
		t.Errorf("OpVariable count = %d, want 2", variableCount)
	}
}

func TestBlockExitKind_Constants(t *testing.T) {
	tests := []struct {
		name string
		kind BlockExitKind
		want int
	}{
		{"BlockExitReturn", BlockExitReturn, 0},
		{"BlockExitBranch", BlockExitBranch, 1},
		{"BlockExitBreakIf", BlockExitBreakIf, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.kind) != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.kind, tt.want)
			}
		})
	}
}

func TestBlockExitDisposition_Constants(t *testing.T) {
	if ExitUsed != 0 {
		t.Errorf("ExitUsed = %d, want 0", ExitUsed)
	}
	if ExitDiscarded != 1 {
		t.Errorf("ExitDiscarded = %d, want 1", ExitDiscarded)
	}
}

func TestBlockExit_Fields(t *testing.T) {
	exit := BlockExit{
		Kind:       BlockExitBreakIf,
		Target:     42,
		Condition:  99,
		PreambleID: 55,
	}
	if exit.Kind != BlockExitBreakIf {
		t.Errorf("Kind = %d, want BlockExitBreakIf", exit.Kind)
	}
	if exit.Target != 42 {
		t.Errorf("Target = %d, want 42", exit.Target)
	}
	if exit.Condition != 99 {
		t.Errorf("Condition = %d, want 99", exit.Condition)
	}
	if exit.PreambleID != 55 {
		t.Errorf("PreambleID = %d, want 55", exit.PreambleID)
	}
}

func TestLoopContext_ValueSemantics(t *testing.T) {
	original := LoopContext{
		ContinuingID: 10,
		BreakID:      20,
	}

	// Copy by value (Go default for structs).
	copied := original

	// Modify the copy.
	copied.ContinuingID = 99
	copied.BreakID = 88

	// Original must be unchanged.
	if original.ContinuingID != 10 {
		t.Errorf("original.ContinuingID = %d, want 10 (value semantics violated)", original.ContinuingID)
	}
	if original.BreakID != 20 {
		t.Errorf("original.BreakID = %d, want 20 (value semantics violated)", original.BreakID)
	}
}

func TestMakeLabelInstruction(t *testing.T) {
	inst := makeLabelInstruction(42)
	if inst.Opcode != OpLabel {
		t.Errorf("Opcode = %d, want OpLabel (%d)", inst.Opcode, OpLabel)
	}
	if len(inst.Words) != 1 || inst.Words[0] != 42 {
		t.Errorf("Words = %v, want [42]", inst.Words)
	}
}

func TestMakeFunctionEndInstruction(t *testing.T) {
	inst := makeFunctionEndInstruction()
	if inst.Opcode != OpFunctionEnd {
		t.Errorf("Opcode = %d, want OpFunctionEnd (%d)", inst.Opcode, OpFunctionEnd)
	}
	if inst.Words != nil {
		t.Errorf("Words = %v, want nil", inst.Words)
	}
}

func TestFunctionBuilder_EmptyFunction(t *testing.T) {
	// Edge case: function with no blocks (just signature + end).
	var fb FunctionBuilder
	fb.Signature = Instruction{Opcode: OpFunction, Words: []uint32{1, 0, 2}}

	result := fb.ToInstructions()

	if len(result) != 2 {
		t.Fatalf("result length = %d, want 2 (OpFunction + OpFunctionEnd)", len(result))
	}
	if result[0].Opcode != OpFunction {
		t.Errorf("result[0].Opcode = %d, want OpFunction", result[0].Opcode)
	}
	if result[1].Opcode != OpFunctionEnd {
		t.Errorf("result[1].Opcode = %d, want OpFunctionEnd", result[1].Opcode)
	}
}
