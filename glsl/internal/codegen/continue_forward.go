// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import "fmt"

// continueCtx tracks nesting of loops and switches to orchestrate forwarding
// of continue statements inside a do-while (single-body switch) to the enclosing loop.
//
// Unlike HLSL (where ALL switches need continue forwarding because FXC rejects
// continue in switch), GLSL only needs this for switches rendered as do-while loops.
// Regular GLSL switches can handle continue natively.
//
// Matches Rust naga's back::continue_forward::ContinueCtx (GLSL usage).
type continueCtx struct {
	stack []nesting
}

type nestingKind int

const (
	nestingLoop nestingKind = iota
	nestingSwitch
)

type nesting struct {
	kind                nestingKind
	variable            string // only for nestingSwitch
	continueEncountered bool   // only for nestingSwitch
}

// exitControlFlow describes what code to emit after a switch.
type exitControlFlow int

const (
	exitNone     exitControlFlow = iota
	exitContinue                 // emit: if (variable) { continue; }
	exitBreak                    // emit: if (variable) { break; }
)

type exitControlFlowResult struct {
	kind     exitControlFlow
	variable string
}

func (ctx *continueCtx) clear() {
	ctx.stack = ctx.stack[:0]
}

// enterLoop records entering a Loop statement.
func (ctx *continueCtx) enterLoop() {
	ctx.stack = append(ctx.stack, nesting{kind: nestingLoop})
}

// exitLoop records leaving a Loop statement.
// Returns an error if the nesting stack is out of sync (expected Loop on top).
func (ctx *continueCtx) exitLoop() error {
	if len(ctx.stack) == 0 || ctx.stack[len(ctx.stack)-1].kind != nestingLoop {
		return fmt.Errorf("glsl: internal error: continueCtx stack out of sync: expected Loop on top")
	}
	ctx.stack = ctx.stack[:len(ctx.stack)-1]
	return nil
}

// enterSwitch records entering a Switch statement (only do-while switches in GLSL).
// Returns non-empty variable name if a bool variable should be declared before
// the switch (only for the outermost switch within a loop).
func (ctx *continueCtx) enterSwitch(namer *namer) string {
	if len(ctx.stack) == 0 {
		// Not inside a loop, no forwarding needed.
		return ""
	}

	top := &ctx.stack[len(ctx.stack)-1]
	switch top.kind {
	case nestingLoop:
		variable := namer.call("should_continue")
		ctx.stack = append(ctx.stack, nesting{
			kind:     nestingSwitch,
			variable: variable,
		})
		return variable
	case nestingSwitch:
		// Nested switch: reuse the same variable, don't declare again.
		variable := top.variable
		ctx.stack = append(ctx.stack, nesting{
			kind:     nestingSwitch,
			variable: variable,
		})
		return ""
	}
	return ""
}

// exitSwitch records leaving a Switch statement.
// Returns what code should be emitted after the switch, or an error if the
// nesting stack is out of sync.
func (ctx *continueCtx) exitSwitch() (exitControlFlowResult, error) {
	if len(ctx.stack) == 0 {
		return exitControlFlowResult{kind: exitNone}, nil
	}

	top := ctx.stack[len(ctx.stack)-1]
	ctx.stack = ctx.stack[:len(ctx.stack)-1]

	if top.kind != nestingSwitch {
		return exitControlFlowResult{}, fmt.Errorf("glsl: internal error: continueCtx stack out of sync: expected Switch on top")
	}

	if !top.continueEncountered {
		return exitControlFlowResult{kind: exitNone}, nil
	}

	// Check if the new top is also a Switch (nested switches)
	if len(ctx.stack) > 0 {
		newTop := &ctx.stack[len(ctx.stack)-1]
		if newTop.kind == nestingSwitch {
			// Propagate continue_encountered upward
			newTop.continueEncountered = true
			return exitControlFlowResult{kind: exitBreak, variable: top.variable}, nil
		}
	}

	return exitControlFlowResult{kind: exitContinue, variable: top.variable}, nil
}

// continueEncountered is called when a Continue statement is encountered.
// Returns the variable name to set to true if we need to forward the continue,
// or empty string if a normal continue can be emitted.
func (ctx *continueCtx) continueEncountered() string {
	if len(ctx.stack) == 0 {
		return ""
	}
	top := &ctx.stack[len(ctx.stack)-1]
	if top.kind == nestingSwitch {
		top.continueEncountered = true
		return top.variable
	}
	return ""
}
