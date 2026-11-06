// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dce

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// nestedLoops builds depth loops nested in each other, each leaving through
// `if c {} else { break; }`, with one store to a storage global at the
// center: every loop holds that live store.
func nestedLoops(depth int) (*ir.Module, *ir.Function) {
	mod := &ir.Module{}
	storageOut(mod)
	fn := &ir.Function{Arguments: []ir.FunctionArgument{{Name: "c"}}}
	c := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	seven := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(7)})
	inner := ir.Block{{Kind: ir.StmtStore{Pointer: out, Value: seven}}}
	for range depth {
		inner = ir.Block{{Kind: ir.StmtLoop{Body: append(ir.Block{
			{Kind: ir.StmtIf{Condition: c, Reject: ir.Block{{Kind: ir.StmtBreak{}}}}},
		}, inner...)}}}
	}
	fn.Body = append(inner, ir.Statement{Kind: ir.StmtReturn{}})
	return mod, fn
}

// TestControlDepsLinearInNesting: one controlDeps walk visits each block a
// bounded number of times, whatever the loop nesting. loop walked its body
// twice, once to ask and once with its breaks counted, and the nested loops
// in it again each time, so a walk was about 2^depth blocks and dxil.Compile
// of a 1 KB shader of 26 nested loops took two minutes.
func TestControlDepsLinearInNesting(t *testing.T) {
	const depth = 30
	mod, fn := nestedLoops(depth)
	live := markLive(mod, fn)
	cd := controlDeps{mod: mod, fn: fn, live: live, deadLocals: findDeadLocals(fn, live), localPtrs: buildLocalPtrMap(fn)}
	cd.block(fn.Body, false, false)
	// Each loop has a body, a continuing and an if with two branches, each
	// walked for at most the two break/continue settings.
	if limit := 16 * (depth + 1); cd.walks > limit {
		t.Fatalf("one walk visited %d blocks for %d nested loops, want at most %d", cd.walks, depth, limit)
	}

	Run(mod, fn)
	if n := len(loops(fn.Body)); n != 1 {
		t.Errorf("outer loops after DCE = %d, want 1: the store at the center is live", n)
	}
}
