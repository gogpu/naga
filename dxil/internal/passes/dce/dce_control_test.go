// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dce

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// emitted reports whether some Emit of block, or of a block in it, covers h.
func emitted(block ir.Block, h ir.ExpressionHandle) bool {
	for _, st := range block {
		switch k := st.Kind.(type) {
		case ir.StmtEmit:
			if k.Range.Start <= h && h < k.Range.End {
				return true
			}
		case ir.StmtIf:
			if emitted(k.Accept, h) || emitted(k.Reject, h) {
				return true
			}
		case ir.StmtBlock:
			if emitted(k.Block, h) {
				return true
			}
		}
	}
	return false
}

// ifs returns the ifs of block, not looking into nested blocks.
func ifs(block ir.Block) []ir.StmtIf {
	var out []ir.StmtIf
	for _, st := range block {
		if k, ok := st.Kind.(ir.StmtIf); ok {
			out = append(out, k)
		}
	}
	return out
}

// TestSharedConditionOfADeadIf (P1-DXIL-061): two ifs on one condition, the
// first with nothing live in it.
//
//	let c = li > 2u;
//	if c { var t = 1; }
//	if c { out = 7; }
//
// The first if goes, and c stays emitted: it is the condition of the second.
// DCE used to take the liveness of c back when it found the first if dead,
// because no other expression read c (an if's condition is a statement use),
// and the emitter met the second if with no value for c.
func TestSharedConditionOfADeadIf(t *testing.T) {
	mod := &ir.Module{}
	u32 := scalarTypeHandle(mod, ir.ScalarUint, 4)
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	mod.GlobalVariables = []ir.GlobalVariable{{Name: "out", Space: ir.SpaceStorage, Access: ir.StorageAccessLoad | ir.StorageAccessStore, Type: i32}}

	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{{Name: "li", Type: u32}},
		LocalVars: []ir.LocalVariable{{Name: "t", Type: i32}},
	}
	li := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	two := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(2)})
	c := appendExpr(fn, ir.ExprBinary{Op: ir.BinaryGreater, Left: li, Right: two})
	tPtr := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	one := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	seven := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(7)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: c, End: c + 1}}},
		{Kind: ir.StmtIf{Condition: c, Accept: ir.Block{{Kind: ir.StmtStore{Pointer: tPtr, Value: one}}}}},
		{Kind: ir.StmtIf{Condition: c, Accept: ir.Block{{Kind: ir.StmtStore{Pointer: out, Value: seven}}}}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	got := ifs(fn.Body)
	if len(got) != 1 {
		t.Fatalf("ifs after DCE = %d, want 1 (the dead one goes)", len(got))
	}
	if len(got[0].Accept) != 1 {
		t.Fatalf("live if's accept = %d statements, want the store", len(got[0].Accept))
	}
	if !emitted(fn.Body, c) {
		t.Errorf("condition [%d] is not emitted, but the live if reads it", c)
	}
}

// TestSharedConditionOfADeadSwitch: the same with a switch selector shared
// with an if.
func TestSharedConditionOfADeadSwitch(t *testing.T) {
	mod := &ir.Module{}
	u32 := scalarTypeHandle(mod, ir.ScalarUint, 4)
	i32 := scalarTypeHandle(mod, ir.ScalarSint, 4)
	mod.GlobalVariables = []ir.GlobalVariable{{Name: "out", Space: ir.SpaceStorage, Access: ir.StorageAccessLoad | ir.StorageAccessStore, Type: i32}}

	fn := &ir.Function{
		Arguments: []ir.FunctionArgument{{Name: "li", Type: u32}},
		LocalVars: []ir.LocalVariable{{Name: "t", Type: i32}},
	}
	li := appendExpr(fn, ir.ExprFunctionArgument{Index: 0})
	two := appendExpr(fn, ir.Literal{Value: ir.LiteralU32(2)})
	sel := appendExpr(fn, ir.ExprBinary{Op: ir.BinaryGreater, Left: li, Right: two})
	tPtr := appendExpr(fn, ir.ExprLocalVariable{Variable: 0})
	one := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(1)})
	out := appendExpr(fn, ir.ExprGlobalVariable{Variable: 0})
	seven := appendExpr(fn, ir.Literal{Value: ir.LiteralI32(7)})

	fn.Body = ir.Block{
		{Kind: ir.StmtEmit{Range: ir.Range{Start: sel, End: sel + 1}}},
		{Kind: ir.StmtSwitch{Selector: sel, Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtStore{Pointer: tPtr, Value: one}}}},
		}}},
		{Kind: ir.StmtIf{Condition: sel, Accept: ir.Block{{Kind: ir.StmtStore{Pointer: out, Value: seven}}}}},
		{Kind: ir.StmtReturn{}},
	}

	Run(mod, fn)

	for _, st := range fn.Body {
		if _, ok := st.Kind.(ir.StmtSwitch); ok {
			t.Errorf("the dead switch is kept")
		}
	}
	if len(ifs(fn.Body)) != 1 {
		t.Fatalf("the live if is gone")
	}
	if !emitted(fn.Body, sel) {
		t.Errorf("selector [%d] is not emitted, but the live if reads it", sel)
	}
}
