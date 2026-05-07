// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// blockEndsWithTerminator Tests
// =============================================================================

func TestBlockEndsWithTerminator(t *testing.T) {
	tests := []struct {
		name  string
		block ir.Block
		want  bool
	}{
		{
			"empty_block",
			ir.Block{},
			false,
		},
		{
			"break",
			ir.Block{{Kind: ir.StmtBreak{}}},
			true,
		},
		{
			"continue",
			ir.Block{{Kind: ir.StmtContinue{}}},
			true,
		},
		{
			"return_no_value",
			ir.Block{{Kind: ir.StmtReturn{}}},
			true,
		},
		{
			"kill",
			ir.Block{{Kind: ir.StmtKill{}}},
			true,
		},
		{
			"store_not_terminator",
			ir.Block{{Kind: ir.StmtStore{Pointer: 0, Value: 1}}},
			false,
		},
		{
			"multiple_stmts_terminator_last",
			ir.Block{
				{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
				{Kind: ir.StmtReturn{}},
			},
			true,
		},
		{
			"multiple_stmts_non_terminator_last",
			ir.Block{
				{Kind: ir.StmtReturn{}},
				{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockEndsWithTerminator(tt.block)
			if got != tt.want {
				t.Errorf("blockEndsWithTerminator() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// joinStrings Tests
// =============================================================================

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name string
		strs []string
		sep  string
		want string
	}{
		{"empty", nil, ", ", ""},
		{"single", []string{"a"}, ", ", "a"},
		{"two", []string{"a", "b"}, ", ", "a, b"},
		{"three", []string{"x", "y", "z"}, " + ", "x + y + z"},
		{"empty_sep", []string{"a", "b"}, "", "ab"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinStrings(tt.strs, tt.sep)
			if got != tt.want {
				t.Errorf("joinStrings() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// writeBarrier Tests
// =============================================================================

func TestWriteBarrier(t *testing.T) {
	tests := []struct {
		name    string
		flags   ir.BarrierFlags
		want    []string
		notWant []string
	}{
		{
			"storage_only",
			ir.BarrierStorage,
			[]string{"memoryBarrierBuffer();", "barrier();"},
			[]string{"memoryBarrierShared();", "subgroupMemoryBarrier();"},
		},
		{
			"workgroup_only",
			ir.BarrierWorkGroup,
			[]string{"memoryBarrierShared();", "barrier();"},
			[]string{"memoryBarrierBuffer();"},
		},
		{
			"subgroup_only",
			ir.BarrierSubGroup,
			[]string{"subgroupMemoryBarrier();", "barrier();"},
			nil,
		},
		{
			"texture_only",
			ir.BarrierTexture,
			[]string{"memoryBarrierImage();", "barrier();"},
			nil,
		},
		{
			"all_flags",
			ir.BarrierStorage | ir.BarrierWorkGroup | ir.BarrierSubGroup | ir.BarrierTexture,
			[]string{
				"memoryBarrierBuffer();",
				"memoryBarrierShared();",
				"subgroupMemoryBarrier();",
				"memoryBarrierImage();",
				"barrier();",
			},
			nil,
		},
		{
			"no_flags_still_barrier",
			0,
			[]string{"barrier();"},
			[]string{"memoryBarrierBuffer();", "memoryBarrierShared();"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter(&ir.Module{}, &Options{LangVersion: Version430})
			err := w.writeBarrier(ir.StmtBarrier{Flags: tt.flags})
			if err != nil {
				t.Fatalf("writeBarrier() error = %v", err)
			}
			output := w.String()
			for _, s := range tt.want {
				if !strings.Contains(output, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, output)
				}
			}
			for _, s := range tt.notWant {
				if strings.Contains(output, s) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", s, output)
				}
			}
		})
	}
}

// =============================================================================
// writeStore Tests
// =============================================================================

func TestWriteStore(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
			{Handle: &tF32},
		},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 0},
		},
	}
	w.localNames = map[uint32]string{0: "x"}

	err := w.writeStore(ir.StmtStore{Pointer: 0, Value: 1})
	if err != nil {
		t.Fatalf("writeStore() error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "=") {
		t.Errorf("expected assignment in output, got: %s", output)
	}
}

// =============================================================================
// writeIf Tests
// =============================================================================

func TestWriteIf(t *testing.T) {
	tBool := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	t.Run("if_only", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
			},
		}
		w.namedExpressions = make(map[ir.ExpressionHandle]string)
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		err := w.writeIf(ir.StmtIf{
			Condition: 0,
			Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
			Reject:    nil,
		})
		if err != nil {
			t.Fatalf("writeIf() error = %v", err)
		}
		output := w.String()
		if !strings.Contains(output, "if (") {
			t.Errorf("expected 'if (' in output, got: %s", output)
		}
		if !strings.Contains(output, "break;") {
			t.Errorf("expected 'break;' in output, got: %s", output)
		}
		if strings.Contains(output, "} else {") {
			t.Error("expected no else branch")
		}
	})

	t.Run("if_else", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(false)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
			},
		}
		w.namedExpressions = make(map[ir.ExpressionHandle]string)
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		err := w.writeIf(ir.StmtIf{
			Condition: 0,
			Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
			Reject:    ir.Block{{Kind: ir.StmtContinue{}}},
		})
		if err != nil {
			t.Fatalf("writeIf() error = %v", err)
		}
		output := w.String()
		if !strings.Contains(output, "} else {") {
			t.Errorf("expected else branch, got:\n%s", output)
		}
		if !strings.Contains(output, "continue;") {
			t.Errorf("expected 'continue;' in else branch, got:\n%s", output)
		}
	})
}

// =============================================================================
// writeLoop Tests
// =============================================================================

func TestWriteLoop(t *testing.T) {
	tBool := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}

	t.Run("simple_loop", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions:     []ir.Expression{},
			ExpressionTypes: []ir.TypeResolution{},
		}
		w.namedExpressions = make(map[ir.ExpressionHandle]string)
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		err := w.writeLoop(ir.StmtLoop{
			Body:       ir.Block{{Kind: ir.StmtBreak{}}},
			Continuing: nil,
			BreakIf:    nil,
		})
		if err != nil {
			t.Fatalf("writeLoop() error = %v", err)
		}
		output := w.String()
		if !strings.Contains(output, "while(true) {") {
			t.Errorf("expected 'while(true) {', got:\n%s", output)
		}
		if strings.Contains(output, "loop_init") {
			t.Error("simple loop should not have loop_init gate")
		}
	})

	t.Run("loop_with_continuing", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions:     []ir.Expression{},
			ExpressionTypes: []ir.TypeResolution{},
		}
		w.namedExpressions = make(map[ir.ExpressionHandle]string)
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		err := w.writeLoop(ir.StmtLoop{
			Body:       ir.Block{{Kind: ir.StmtBreak{}}},
			Continuing: ir.Block{{Kind: ir.StmtKill{}}},
			BreakIf:    nil,
		})
		if err != nil {
			t.Fatalf("writeLoop() error = %v", err)
		}
		output := w.String()
		if !strings.Contains(output, "loop_init") {
			t.Errorf("loop with continuing should have loop_init gate, got:\n%s", output)
		}
		if !strings.Contains(output, "while(true) {") {
			t.Errorf("expected while(true), got:\n%s", output)
		}
	})

	t.Run("loop_with_break_if", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tBool},
			},
		}
		w.namedExpressions = make(map[ir.ExpressionHandle]string)
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		breakIf := ir.ExpressionHandle(0)
		err := w.writeLoop(ir.StmtLoop{
			Body:       ir.Block{{Kind: ir.StmtReturn{}}},
			Continuing: nil,
			BreakIf:    &breakIf,
		})
		if err != nil {
			t.Fatalf("writeLoop() error = %v", err)
		}
		output := w.String()
		if !strings.Contains(output, "loop_init") {
			t.Errorf("loop with break_if should have loop_init, got:\n%s", output)
		}
		if !strings.Contains(output, "break;") {
			t.Errorf("expected break inside break_if block, got:\n%s", output)
		}
	})
}

// =============================================================================
// writeSwitch Tests (multi-case)
// =============================================================================

func TestWriteSwitch_MultiCase(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tI32},
		},
	}
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

	err := w.writeSwitch(ir.StmtSwitch{
		Selector: 0,
		Cases: []ir.SwitchCase{
			{
				Value:       ir.SwitchValueI32(1),
				Body:        ir.Block{{Kind: ir.StmtBreak{}}},
				FallThrough: false,
			},
			{
				Value:       ir.SwitchValueI32(2),
				Body:        ir.Block{{Kind: ir.StmtBreak{}}},
				FallThrough: false,
			},
			{
				Value:       ir.SwitchValueDefault{},
				Body:        ir.Block{{Kind: ir.StmtBreak{}}},
				FallThrough: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("writeSwitch() error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "switch(") {
		t.Errorf("expected 'switch(' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "case 1:") {
		t.Errorf("expected 'case 1:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "case 2:") {
		t.Errorf("expected 'case 2:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "default:") {
		t.Errorf("expected 'default:' in output, got:\n%s", output)
	}
}

func TestWriteSwitch_UnsignedCase(t *testing.T) {
	tU32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tU32},
		},
	}
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

	err := w.writeSwitch(ir.StmtSwitch{
		Selector: 0,
		Cases: []ir.SwitchCase{
			{
				Value:       ir.SwitchValueU32(42),
				Body:        ir.Block{{Kind: ir.StmtBreak{}}},
				FallThrough: false,
			},
			{
				Value:       ir.SwitchValueDefault{},
				Body:        ir.Block{{Kind: ir.StmtBreak{}}},
				FallThrough: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("writeSwitch() error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "case 42u:") {
		t.Errorf("expected 'case 42u:' in output, got:\n%s", output)
	}
}

func TestWriteSwitch_FallthroughCase(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tI32},
		},
	}
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

	err := w.writeSwitch(ir.StmtSwitch{
		Selector: 0,
		Cases: []ir.SwitchCase{
			{
				Value:       ir.SwitchValueI32(1),
				Body:        nil,
				FallThrough: true,
			},
			{
				Value:       ir.SwitchValueI32(2),
				Body:        nil,
				FallThrough: true,
			},
			{
				Value:       ir.SwitchValueDefault{},
				Body:        ir.Block{{Kind: ir.StmtBreak{}}},
				FallThrough: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("writeSwitch() error = %v", err)
	}
	output := w.String()
	// Empty fallthrough cases are rendered as do-while
	if !strings.Contains(output, "do {") {
		t.Errorf("expected do-while for single-body switch, got:\n%s", output)
	}
}

// =============================================================================
// writeReturn Tests
// =============================================================================

func TestWriteReturn_NoValue(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	err := w.writeReturn(ir.StmtReturn{Value: nil})
	if err != nil {
		t.Fatalf("writeReturn() error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "return;") {
		t.Errorf("expected 'return;', got: %s", output)
	}
}

func TestWriteReturn_WithValue(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tF32},
		},
	}
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.inEntryPoint = false

	val := ir.ExpressionHandle(0)
	err := w.writeReturn(ir.StmtReturn{Value: &val})
	if err != nil {
		t.Fatalf("writeReturn() error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "return ") {
		t.Errorf("expected 'return <value>', got: %s", output)
	}
}

// =============================================================================
// writeStatementKind Tests — unsupported
// =============================================================================

// Note: writeStatementKind unsupported paths are tested indirectly via
// snapshot tests. Direct testing requires implementing ir.StatementKind
// marker interface which is not feasible in test code.

// =============================================================================
// writeStatementKind Tests — Kill / Break / Continue
// =============================================================================

func TestWriteStatementKind_Kill(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	err := w.writeStatementKind(ir.StmtKill{})
	if err != nil {
		t.Fatalf("writeStatementKind(Kill) error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "discard;") {
		t.Errorf("expected 'discard;', got: %s", output)
	}
}

func TestWriteStatementKind_Break(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	err := w.writeStatementKind(ir.StmtBreak{})
	if err != nil {
		t.Fatalf("writeStatementKind(Break) error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "break;") {
		t.Errorf("expected 'break;', got: %s", output)
	}
}

func TestWriteStatementKind_Continue(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	err := w.writeStatementKind(ir.StmtContinue{})
	if err != nil {
		t.Fatalf("writeStatementKind(Continue) error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "continue;") {
		t.Errorf("expected 'continue;', got: %s", output)
	}
}

func TestWriteStatementKind_ContinueInsideSwitch(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	// Set up continue forwarding context: loop -> switch
	w.continueCtx.enterLoop()
	variable := w.continueCtx.enterSwitch(w.namer)

	err := w.writeStatementKind(ir.StmtContinue{})
	if err != nil {
		t.Fatalf("writeStatementKind(Continue) error = %v", err)
	}
	output := w.String()
	if variable == "" {
		t.Fatal("expected non-empty variable name from enterSwitch")
	}
	if !strings.Contains(output, variable+" = true;") {
		t.Errorf("expected '%s = true;' in output, got:\n%s", variable, output)
	}
	if !strings.Contains(output, "break;") {
		t.Errorf("expected 'break;' after setting continue var, got:\n%s", output)
	}
}

// =============================================================================
// writeRayQuery Tests
// =============================================================================

func TestWriteRayQuery_ReturnsError(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	err := w.writeRayQuery(ir.StmtRayQuery{})
	if err == nil {
		t.Error("expected error for ray query in GLSL")
	}
	if !strings.Contains(err.Error(), "ray query") {
		t.Errorf("expected ray query error, got: %v", err)
	}
}

// =============================================================================
// writeBlock Tests
// =============================================================================

func TestWriteBlock_Empty(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	err := w.writeBlock(ir.Block{})
	if err != nil {
		t.Fatalf("writeBlock(empty) error = %v", err)
	}
	output := w.String()
	if output != "" {
		t.Errorf("expected empty output for empty block, got: %q", output)
	}
}

// =============================================================================
// writeStmtBlock Tests
// =============================================================================

func TestWriteStatementKind_Block(t *testing.T) {
	w := newWriter(&ir.Module{}, &Options{LangVersion: Version330})
	w.currentFunction = &ir.Function{}

	err := w.writeStatementKind(ir.StmtBlock{
		Block: ir.Block{{Kind: ir.StmtBreak{}}},
	})
	if err != nil {
		t.Fatalf("writeStatementKind(Block) error = %v", err)
	}
	output := w.String()
	if !strings.Contains(output, "{") {
		t.Errorf("expected '{' in block output, got: %s", output)
	}
	if !strings.Contains(output, "break;") {
		t.Errorf("expected 'break;' in block output, got: %s", output)
	}
	if !strings.Contains(output, "}") {
		t.Errorf("expected '}' in block output, got: %s", output)
	}
}

// =============================================================================
// writeWorkGroupUniformLoad Tests
// =============================================================================

func TestWriteWorkGroupUniformLoad(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "shared_data", Space: ir.SpaceWorkGroup, Type: 0},
		},
	}

	w := newWriter(module, &Options{LangVersion: Version430})
	w.currentFunction = &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.ExprGlobalVariable{Variable: 0}},
			{Kind: ir.ExprLoad{Pointer: 0}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &tI32},
			{Handle: &tI32},
		},
	}
	w.namedExpressions = make(map[ir.ExpressionHandle]string)
	w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})
	w.localNames = map[uint32]string{}
	w.names = map[nameKey]string{
		{kind: nameKeyGlobalVariable, handle1: 0}: "shared_data",
	}

	err := w.writeWorkGroupUniformLoad(ir.StmtWorkGroupUniformLoad{
		Pointer: 0,
		Result:  1,
	})
	if err != nil {
		t.Fatalf("writeWorkGroupUniformLoad() error = %v", err)
	}
	output := w.String()

	// Should have two barrier pairs
	barrierCount := strings.Count(output, "memoryBarrierShared();")
	if barrierCount != 2 {
		t.Errorf("expected 2 memoryBarrierShared() calls, got %d in:\n%s", barrierCount, output)
	}
	barriersCount := strings.Count(output, "barrier();")
	if barriersCount != 2 {
		t.Errorf("expected 2 barrier() calls, got %d in:\n%s", barriersCount, output)
	}
}

// =============================================================================
// shouldBakeExpression Tests
// =============================================================================

func TestShouldBakeExpression(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	t.Run("load_always_baked", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprLoad{Pointer: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
		}
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		if !w.shouldBakeExpression(0) {
			t.Error("Load expression should always be baked")
		}
	})

	t.Run("derivative_always_baked", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprDerivative{Axis: ir.DerivativeX, Expr: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
		}
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		if !w.shouldBakeExpression(0) {
			t.Error("Derivative expression should always be baked")
		}
	})

	t.Run("literal_not_baked", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
		}
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		if w.shouldBakeExpression(0) {
			t.Error("Literal expression should not be baked by default")
		}
	})

	t.Run("named_literal_not_baked", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
			NamedExpressions: map[ir.ExpressionHandle]string{
				0: "my_const",
			},
		}
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		// Named literal should NOT be baked (post-override const-eval)
		if w.shouldBakeExpression(0) {
			t.Error("Named Literal expression should not be baked")
		}
	})

	t.Run("named_non_literal_baked", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &tF32},
			},
			NamedExpressions: map[ir.ExpressionHandle]string{
				0: "my_var",
			},
		}
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		if !w.shouldBakeExpression(0) {
			t.Error("Named non-Literal expression should be baked")
		}
	})

	t.Run("nil_function", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = nil
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		if w.shouldBakeExpression(0) {
			t.Error("Should return false with nil function")
		}
	})

	t.Run("out_of_range", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{},
		}
		w.needBakeExpression = make(map[ir.ExpressionHandle]struct{})

		if w.shouldBakeExpression(999) {
			t.Error("Should return false for out-of-range handle")
		}
	})
}

// =============================================================================
// isVariableReference Tests
// =============================================================================

func TestIsVariableReference(t *testing.T) {
	module := &ir.Module{}

	tests := []struct {
		name string
		kind ir.ExpressionKind
		want bool
	}{
		{"global_var", ir.ExprGlobalVariable{Variable: 0}, true},
		{"local_var", ir.ExprLocalVariable{Variable: 0}, true},
		{"literal", ir.Literal{Value: ir.LiteralF32(1.0)}, false},
		{"binary", ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWriter(module, &Options{LangVersion: Version330})
			w.currentFunction = &ir.Function{
				Expressions: []ir.Expression{
					{Kind: tt.kind},
				},
			}

			got := w.isVariableReference(0)
			if got != tt.want {
				t.Errorf("isVariableReference() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("nil_function", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = nil
		if w.isVariableReference(0) {
			t.Error("Should return false with nil function")
		}
	})
}

// =============================================================================
// isPointerExpression Tests
// =============================================================================

func TestIsPointerExpression(t *testing.T) {
	ptrType := ir.TypeHandle(0)
	scalarType := ir.TypeHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.PointerType{Base: 1, Space: ir.SpaceFunction}},
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	t.Run("pointer_type_handle", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &ptrType},
			},
		}
		if !w.isPointerExpression(0) {
			t.Error("expression with pointer type handle should be detected as pointer")
		}
	})

	t.Run("pointer_type_value", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Value: ir.PointerType{Base: 1, Space: ir.SpaceFunction}},
			},
		}
		if !w.isPointerExpression(0) {
			t.Error("expression with pointer type value should be detected as pointer")
		}
	})

	t.Run("non_pointer", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = &ir.Function{
			Expressions: []ir.Expression{
				{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			},
			ExpressionTypes: []ir.TypeResolution{
				{Handle: &scalarType},
			},
		}
		if w.isPointerExpression(0) {
			t.Error("scalar type should not be detected as pointer")
		}
	})

	t.Run("nil_function", func(t *testing.T) {
		w := newWriter(module, &Options{LangVersion: Version330})
		w.currentFunction = nil
		if w.isPointerExpression(0) {
			t.Error("Should return false with nil function")
		}
	})
}
