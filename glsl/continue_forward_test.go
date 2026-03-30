// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestContinueCtx_BasicStack tests the basic stack operations.
func TestContinueCtx_BasicStack(t *testing.T) {
	var ctx continueCtx
	n := newNamer()

	// Outside any loop, enterSwitch returns empty (no forwarding needed)
	v := ctx.enterSwitch(n)
	if v != "" {
		t.Errorf("enterSwitch outside loop should return empty, got %q", v)
	}
	result := ctx.exitSwitch()
	if result.kind != exitNone {
		t.Errorf("exitSwitch outside loop should return exitNone, got %v", result.kind)
	}

	// Inside a loop, enterSwitch returns a variable name
	ctx.enterLoop()
	v = ctx.enterSwitch(n)
	if v == "" {
		t.Error("enterSwitch inside loop should return a variable name")
	}
	if !strings.Contains(v, "should_continue") {
		t.Errorf("variable name should contain 'should_continue', got %q", v)
	}

	// No continue encountered → exitNone
	result = ctx.exitSwitch()
	if result.kind != exitNone {
		t.Errorf("exitSwitch without continue should return exitNone, got %v", result.kind)
	}
	ctx.exitLoop()
}

// TestContinueCtx_ContinueInSwitch tests continue forwarding inside a switch.
func TestContinueCtx_ContinueInSwitch(t *testing.T) {
	var ctx continueCtx
	n := newNamer()

	ctx.enterLoop()
	variable := ctx.enterSwitch(n)
	if variable == "" {
		t.Fatal("expected variable name from enterSwitch")
	}

	// Encounter a continue
	v := ctx.continueEncountered()
	if v == "" {
		t.Error("continueEncountered inside switch should return variable")
	}
	if v != variable {
		t.Errorf("continueEncountered should return same variable: got %q, want %q", v, variable)
	}

	// Exit switch should now return exitContinue
	result := ctx.exitSwitch()
	if result.kind != exitContinue {
		t.Errorf("expected exitContinue, got %v", result.kind)
	}
	if result.variable != variable {
		t.Errorf("variable mismatch: got %q, want %q", result.variable, variable)
	}

	ctx.exitLoop()
}

// TestContinueCtx_NestedSwitches tests nested switches with continue forwarding.
func TestContinueCtx_NestedSwitches(t *testing.T) {
	var ctx continueCtx
	n := newNamer()

	ctx.enterLoop()

	// Outermost switch gets variable
	outerVar := ctx.enterSwitch(n)
	if outerVar == "" {
		t.Fatal("outer switch should get variable")
	}

	// Inner switch reuses variable, returns empty
	innerVar := ctx.enterSwitch(n)
	if innerVar != "" {
		t.Errorf("inner switch should return empty (reuse variable), got %q", innerVar)
	}

	// Continue in inner switch
	v := ctx.continueEncountered()
	if v != outerVar {
		t.Errorf("inner continue should return outer variable: got %q, want %q", v, outerVar)
	}

	// Exit inner switch should return exitBreak (propagate to outer)
	result := ctx.exitSwitch()
	if result.kind != exitBreak {
		t.Errorf("inner switch exit should return exitBreak, got %v", result.kind)
	}

	// Exit outer switch should now return exitContinue (propagated from inner)
	result = ctx.exitSwitch()
	if result.kind != exitContinue {
		t.Errorf("outer switch exit should return exitContinue, got %v", result.kind)
	}

	ctx.exitLoop()
}

// TestContinueCtx_ContinueOutsideSwitch tests that continue outside switch is normal.
func TestContinueCtx_ContinueOutsideSwitch(t *testing.T) {
	var ctx continueCtx

	// Outside any nesting
	v := ctx.continueEncountered()
	if v != "" {
		t.Errorf("continueEncountered outside nesting should return empty, got %q", v)
	}

	// Inside loop but outside switch
	ctx.enterLoop()
	v = ctx.continueEncountered()
	if v != "" {
		t.Errorf("continueEncountered inside loop (no switch) should return empty, got %q", v)
	}
	ctx.exitLoop()
}

// TestContinueForward_DoWhileOutput tests that continue inside a do-while switch
// is correctly converted to should_continue = true + break pattern.
func TestContinueForward_DoWhileOutput(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Body: ir.Block{
						// while(true) {
						//   switch(0) { default: { continue; } }
						// }
						{Kind: ir.StmtLoop{
							Body: ir.Block{
								{Kind: ir.StmtSwitch{
									Selector: ir.ExpressionHandle(0),
									Cases: []ir.SwitchCase{
										{
											Value:       ir.SwitchValueDefault{},
											Body:        ir.Block{{Kind: ir.StmtContinue{}}},
											FallThrough: false,
										},
									},
								}},
							},
						}},
					},
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
					},
				},
			},
		},
	}

	source, _, err := Compile(module, Options{
		LangVersion: Version430,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Must contain the should_continue pattern
	if !strings.Contains(source, "bool should_continue") {
		t.Errorf("output should contain 'bool should_continue' declaration\nGot:\n%s", source)
	}
	if !strings.Contains(source, "should_continue = true;") {
		t.Errorf("output should contain 'should_continue = true;' assignment\nGot:\n%s", source)
	}
	if !strings.Contains(source, "if (should_continue)") {
		t.Errorf("output should contain 'if (should_continue)' check\nGot:\n%s", source)
	}
	if !strings.Contains(source, "do {") {
		t.Errorf("output should contain 'do {' for single-body switch\nGot:\n%s", source)
	}
}

// TestContinueForward_RegularSwitchNoContinueForward tests that regular multi-case
// switches inside loops do NOT get continue forwarding in GLSL.
func TestContinueForward_RegularSwitchNoContinueForward(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:      "main",
				Stage:     ir.StageCompute,
				Workgroup: [3]uint32{1, 1, 1},
				Function: ir.Function{
					Body: ir.Block{
						{Kind: ir.StmtLoop{
							Body: ir.Block{
								{Kind: ir.StmtSwitch{
									Selector: ir.ExpressionHandle(0),
									Cases: []ir.SwitchCase{
										{
											Value:       ir.SwitchValueI32(1),
											Body:        ir.Block{{Kind: ir.StmtContinue{}}},
											FallThrough: false,
										},
										{
											Value:       ir.SwitchValueDefault{},
											Body:        ir.Block{{Kind: ir.StmtBreak{}}},
											FallThrough: false,
										},
									},
								}},
							},
						}},
					},
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
					},
				},
			},
		},
	}

	source, _, err := Compile(module, Options{
		LangVersion: Version430,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Regular switch should NOT use continue forwarding in GLSL
	if strings.Contains(source, "should_continue") {
		t.Errorf("regular switch should NOT use continue forwarding in GLSL\nGot:\n%s", source)
	}
	// Should have normal continue
	if !strings.Contains(source, "continue;") {
		t.Errorf("regular switch should emit normal continue;\nGot:\n%s", source)
	}
}
