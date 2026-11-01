// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package irutil_test

import (
	"testing"

	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
)

// TestInlineRemapsEveryStatementOperand inlines helpers whose bodies hold the
// statements that produce a value (atomics, workgroupUniformLoad, ray-query
// proceed, subgroup operations) and requires each inlined statement to name
// the caller's expressions: its result is the caller's result expression of
// the right kind, and the compare operand of a compare-exchange is the
// caller's copy of the literal. Inlining once copied callee bodies through a
// walker of its own that knew none of these statements and the compare
// operand, so they kept callee handles that named unrelated caller
// expressions (the DXIL path failed on ray-query.wgsl with "ExprRayQueryProceedResult
// [31] not ..."); it now uses the shared statement walker.
func TestInlineRemapsEveryStatementOperand(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int // statements checked
	}{
		{"atomic compare-exchange", `
var<workgroup> a: atomic<u32>;
fn helper() -> u32 { let r = atomicCompareExchangeWeak(&a, 5u, 1u); return r.old_value; }
@compute @workgroup_size(1) fn main() { _ = helper(); }`, 1},
		{"workgroupUniformLoad", `
var<workgroup> w: u32;
fn helper() -> u32 { return workgroupUniformLoad(&w); }
@compute @workgroup_size(1) fn main() { _ = helper(); }`, 1},
		{"ray-query proceed", `
enable wgpu_ray_query;
@group(0) @binding(0) var acs: acceleration_structure;
fn helper(pos: vec3<f32>) -> bool {
    var rq: ray_query;
    rayQueryInitialize(&rq, acs, RayDesc(4u, 0xFFu, 0.1, 100.0, pos, vec3<f32>(0.0, 1.0, 0.0)));
    return rayQueryProceed(&rq);
}
@compute @workgroup_size(1) fn main() { _ = helper(vec3<f32>(0.0)); }`, 2},
		{"subgroup operations", `
enable subgroups;
fn helper(x: u32) -> u32 { let b = subgroupBallot(x > 3u); return subgroupAdd(x) + b.x; }
@compute @workgroup_size(64) fn main(@builtin(local_invocation_index) i: u32) { _ = helper(i); }`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod := lowerWGSL(t, tt.src)
			if err := irutil.InlineUserFunctions(mod, nil); err != nil {
				t.Fatal(err)
			}
			fn := &mod.EntryPoints[0].Function
			checked := 0
			forEachStatement(fn.Body, func(kind ir.StatementKind) {
				if checkInlinedOperands(t, fn, kind) {
					checked++
				}
			})
			if checked != tt.want {
				t.Errorf("checked %d inlined statements, want %d", checked, tt.want)
			}
		})
	}
}

// checkInlinedOperands checks the result and compare operands of one
// statement against the caller's arena and reports whether it had any.
func checkInlinedOperands(t *testing.T, fn *ir.Function, kind ir.StatementKind) bool {
	t.Helper()
	kindOf := func(h ir.ExpressionHandle) ir.ExpressionKind {
		if int(h) >= len(fn.Expressions) {
			t.Fatalf("%T: handle %d outside the caller's %d expressions", kind, h, len(fn.Expressions))
		}
		return fn.Expressions[h].Kind
	}
	expect := func(h ir.ExpressionHandle, ok bool) {
		if !ok {
			t.Errorf("%T: operand %d is %T in the caller", kind, h, kindOf(h))
		}
	}
	switch s := kind.(type) {
	case ir.StmtAtomic:
		_, ok := kindOf(*s.Result).(ir.ExprAtomicResult)
		expect(*s.Result, ok)
		if ex, isEx := s.Fun.(ir.AtomicExchange); isEx && ex.Compare != nil {
			lit, isLit := kindOf(*ex.Compare).(ir.Literal)
			expect(*ex.Compare, isLit && lit.Value == ir.LiteralU32(5))
		}
	case ir.StmtWorkGroupUniformLoad:
		_, ok := kindOf(s.Result).(ir.ExprWorkGroupUniformLoadResult)
		expect(s.Result, ok)
	case ir.StmtRayQuery:
		_, ok := kindOf(s.Query).(ir.ExprLocalVariable)
		expect(s.Query, ok)
		if p, isProceed := s.Fun.(ir.RayQueryProceed); isProceed {
			_, ok := kindOf(p.Result).(ir.ExprRayQueryProceedResult)
			expect(p.Result, ok)
		}
	case ir.StmtSubgroupBallot:
		_, ok := kindOf(s.Result).(ir.ExprSubgroupBallotResult)
		expect(s.Result, ok)
	case ir.StmtSubgroupCollectiveOperation:
		_, ok := kindOf(s.Result).(ir.ExprSubgroupOperationResult)
		expect(s.Result, ok)
	default:
		return false
	}
	return true
}

// forEachStatement calls visit for every statement of block and its nested
// blocks.
func forEachStatement(block ir.Block, visit func(ir.StatementKind)) {
	for _, s := range block {
		visit(s.Kind)
		switch k := s.Kind.(type) {
		case ir.StmtBlock:
			forEachStatement(k.Block, visit)
		case ir.StmtIf:
			forEachStatement(k.Accept, visit)
			forEachStatement(k.Reject, visit)
		case ir.StmtLoop:
			forEachStatement(k.Body, visit)
			forEachStatement(k.Continuing, visit)
		case ir.StmtSwitch:
			for _, c := range k.Cases {
				forEachStatement(c.Body, visit)
			}
		}
	}
}
