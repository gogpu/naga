package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// The spill and phi machinery (getExprHandle's out-of-scope path and
// appendPhiStores) only runs once control flow gives a function more than one
// body, which Phase 1 refuses. It is Phase 2's foundation, ported from Rust
// naga's get_expr_handle and parse_function, so it is tested here on a
// synthetic body tree:
//
//	body 0 (function top) ── body 1 (block 10) ── body 3 (block 30)
//	                      └─ body 2 (block 20)
//
// Type id 3 is i32 (IR type 0).

// spillFixture returns a parser and a function state with that tree.
func spillFixture() (*parser, *functionState) {
	p := newParser(nil, Options{})
	p.module = &ir.Module{Types: []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}}}}
	p.lookupType[3] = lookupType{handle: 0}
	p.lookupExpression = make(map[uint32]lookupExpression)
	fs := &functionState{
		fn:           &ir.Function{},
		blocks:       map[uint32]ir.Block{10: nil, 20: nil, 30: nil},
		bodyForLabel: map[uint32]int{10: 1, 20: 2, 30: 3},
		bodies: []body{
			{parent: 0},
			{parent: 0},
			{parent: 0},
			{parent: 1},
		},
	}
	return p, fs
}

// defineValue appends an expression standing for SPIR-V id `id`, defined in
// `blockID`, and records it.
func defineValue(p *parser, fs *functionState, id, blockID uint32) ir.ExpressionHandle {
	handle := appendExpression(fs.fn, ir.Literal{Value: ir.LiteralI32(int32(id))})
	p.lookupExpression[id] = lookupExpression{handle: handle, typeID: 3, blockID: blockID}
	return handle
}

func TestIsParent(t *testing.T) {
	_, fs := spillFixture()
	cases := []struct {
		child, parent int
		want          bool
	}{
		{3, 1, true},  // grandchild inside its parent
		{3, 0, true},  // everything is inside the top body
		{1, 1, true},  // a body is inside itself
		{2, 1, false}, // siblings
		{1, 3, false}, // a parent is not inside its child
		{0, 2, false}, // the top body is inside nothing else
	}
	for _, tc := range cases {
		if got := isParent(tc.child, tc.parent, fs); got != tc.want {
			t.Errorf("isParent(%d, %d) = %v, want %v", tc.child, tc.parent, got, tc.want)
		}
	}
}

func TestGetExprHandleInScope(t *testing.T) {
	p, fs := spillFixture()
	handle := defineValue(p, fs, 100, 10)
	var consumer emitter
	var block ir.Block
	// Used in body 3, nested in body 1 where it is defined: the expression
	// itself is in scope.
	got, err := p.getExprHandle(100, p.lookupExpression[100], fs, &consumer, &block, 3)
	if err != nil {
		t.Fatalf("getExprHandle: %v", err)
	}
	if got != handle {
		t.Errorf("handle = %d, want the expression %d itself", got, handle)
	}
	if len(fs.fn.LocalVars) != 0 || len(fs.phis) != 0 || len(block) != 0 {
		t.Errorf("an in-scope use spilled: locals %d, phis %d, statements %d", len(fs.fn.LocalVars), len(fs.phis), len(block))
	}
}

func TestGetExprHandleSpillsOutOfScope(t *testing.T) {
	p, fs := spillFixture()
	handle := defineValue(p, fs, 100, 10)
	var consumer emitter
	consumer.startWith(fs.fn.Expressions)
	// An expression of the current block, pending in the emitter: the spill
	// must flush it before its own pre-emit pointer.
	pending := appendExpression(fs.fn, ir.ExprUnary{Op: ir.UnaryNegate, Expr: handle})
	var block ir.Block

	// Used in body 2, a sibling of body 1: out of scope, so it is spilled.
	load, err := p.getExprHandle(100, p.lookupExpression[100], fs, &consumer, &block, 2)
	if err != nil {
		t.Fatalf("getExprHandle: %v", err)
	}
	if len(fs.fn.LocalVars) != 1 || fs.fn.LocalVars[0].Type != 0 {
		t.Fatalf("locals = %#v, want one i32 temporary", fs.fn.LocalVars)
	}
	if len(block) != 1 {
		t.Fatalf("statements = %d, want the flushed Emit of the pending expression", len(block))
	}
	emit, ok := block[0].Kind.(ir.StmtEmit)
	if !ok || emit.Range.Start != pending || emit.Range.End != pending+1 {
		t.Errorf("flush = %#v, want Emit of [%d, %d)", block[0].Kind, pending, pending+1)
	}
	loaded, ok := fs.fn.Expressions[load].Kind.(ir.ExprLoad)
	if !ok {
		t.Fatalf("returned expression = %#v, want a Load", fs.fn.Expressions[load].Kind)
	}
	if pointer, ok := fs.fn.Expressions[loaded.Pointer].Kind.(ir.ExprLocalVariable); !ok || pointer.Variable != 0 {
		t.Errorf("load pointer = %#v, want the temporary", fs.fn.Expressions[loaded.Pointer].Kind)
	}
	// The Load is collected by the restarted emitter; the pointer is not.
	if !consumer.running || consumer.start != int(load) {
		t.Errorf("emitter restarted at %d (running %v), want at the Load %d", consumer.start, consumer.running, load)
	}
	if len(fs.phis) != 1 || fs.phis[0].local != 0 ||
		len(fs.phis[0].expressions) != 1 || fs.phis[0].expressions[0] != (phiSource{id: 100, blockID: 10}) {
		t.Errorf("phis = %#v, want one synthetic phi spilling id 100 from block 10", fs.phis)
	}

	// Completing the phis stores the value into the temporary at the end of
	// its own block, where it is in scope.
	if err := p.appendPhiStores(fs); err != nil {
		t.Fatalf("appendPhiStores: %v", err)
	}
	defining := fs.blocks[10]
	if len(defining) != 1 {
		t.Fatalf("block 10 = %d statements, want the spill store", len(defining))
	}
	store, ok := defining[0].Kind.(ir.StmtStore)
	if !ok || store.Value != handle {
		t.Errorf("spill = %#v, want a store of the value %d", defining[0].Kind, handle)
	}
	if pointer, ok := fs.fn.Expressions[store.Pointer].Kind.(ir.ExprLocalVariable); !ok || pointer.Variable != 0 {
		t.Errorf("spill pointer = %#v, want the temporary", fs.fn.Expressions[store.Pointer].Kind)
	}
}

func TestGetExprHandleUnknownType(t *testing.T) {
	p, fs := spillFixture()
	defineValue(p, fs, 100, 10)
	lookup := p.lookupExpression[100]
	lookup.typeID = 99
	var consumer emitter
	var block ir.Block
	if _, err := p.getExprHandle(100, lookup, fs, &consumer, &block, 2); err == nil || err.Error() != "invalid id %99" {
		t.Errorf("error = %v, want invalid id %%99", err)
	}
}

func TestAppendPhiStoresSpillsAnOutOfScopeSource(t *testing.T) {
	p, fs := spillFixture()
	// A phi in some later block, written from predecessor block 10 with a
	// value defined in block 20: body 1 is not inside body 2, so the value
	// is spilled in block 20 and reloaded at the end of block 10.
	source := defineValue(p, fs, 200, 20)
	fs.fn.LocalVars = append(fs.fn.LocalVars, ir.LocalVariable{Type: 0})
	fs.phis = []phiExpression{{local: 0, expressions: []phiSource{{id: 200, blockID: 10}}}}

	if err := p.appendPhiStores(fs); err != nil {
		t.Fatalf("appendPhiStores: %v", err)
	}
	if len(fs.fn.LocalVars) != 2 {
		t.Fatalf("locals = %d, want the phi and one spill temporary", len(fs.fn.LocalVars))
	}

	defining := fs.blocks[20]
	if len(defining) != 1 {
		t.Fatalf("block 20 = %d statements, want the spill store", len(defining))
	}
	spill, ok := defining[0].Kind.(ir.StmtStore)
	if !ok || spill.Value != source {
		t.Fatalf("block 20 = %#v, want a store of the source", defining[0].Kind)
	}
	if pointer, ok := fs.fn.Expressions[spill.Pointer].Kind.(ir.ExprLocalVariable); !ok || pointer.Variable != 1 {
		t.Errorf("spill pointer = %#v, want the temporary 1", fs.fn.Expressions[spill.Pointer].Kind)
	}

	predecessor := fs.blocks[10]
	if len(predecessor) != 2 {
		t.Fatalf("block 10 = %d statements, want the reload's Emit and the phi store", len(predecessor))
	}
	emit, ok := predecessor[0].Kind.(ir.StmtEmit)
	if !ok || emit.Range.End != emit.Range.Start+1 {
		t.Fatalf("block 10[0] = %#v, want an Emit of the one reload", predecessor[0].Kind)
	}
	reload, ok := fs.fn.Expressions[emit.Range.Start].Kind.(ir.ExprLoad)
	if !ok || reload.Pointer != spill.Pointer {
		t.Errorf("reload = %#v, want a Load of the spill temporary", fs.fn.Expressions[emit.Range.Start].Kind)
	}
	phiStore, ok := predecessor[1].Kind.(ir.StmtStore)
	if !ok || phiStore.Value != emit.Range.Start {
		t.Errorf("block 10[1] = %#v, want the phi store of the reload", predecessor[1].Kind)
	}
	if pointer, ok := fs.fn.Expressions[phiStore.Pointer].Kind.(ir.ExprLocalVariable); !ok || pointer.Variable != 0 {
		t.Errorf("phi store pointer = %#v, want the phi local 0", fs.fn.Expressions[phiStore.Pointer].Kind)
	}
}

func TestAppendPhiStoresInScopeSource(t *testing.T) {
	p, fs := spillFixture()
	// Predecessor block 30 (body 3) is inside body 1, where block 10 defines
	// the value: it is stored directly, without a spill.
	source := defineValue(p, fs, 300, 10)
	fs.fn.LocalVars = append(fs.fn.LocalVars, ir.LocalVariable{Type: 0})
	fs.phis = []phiExpression{{local: 0, expressions: []phiSource{{id: 300, blockID: 30}}}}

	if err := p.appendPhiStores(fs); err != nil {
		t.Fatalf("appendPhiStores: %v", err)
	}
	if len(fs.fn.LocalVars) != 1 {
		t.Errorf("locals = %d, want no spill temporary", len(fs.fn.LocalVars))
	}
	if len(fs.blocks[10]) != 0 {
		t.Errorf("block 10 = %d statements, want none", len(fs.blocks[10]))
	}
	predecessor := fs.blocks[30]
	if len(predecessor) != 1 {
		t.Fatalf("block 30 = %d statements, want the phi store", len(predecessor))
	}
	if store, ok := predecessor[0].Kind.(ir.StmtStore); !ok || store.Value != source {
		t.Errorf("block 30 = %#v, want a store of the source itself", predecessor[0].Kind)
	}
}

func TestAppendPhiStoresErrors(t *testing.T) {
	t.Run("unknown source id", func(t *testing.T) {
		p, fs := spillFixture()
		fs.phis = []phiExpression{{expressions: []phiSource{{id: 999, blockID: 10}}}}
		if err := p.appendPhiStores(fs); err == nil || err.Error() != "invalid id %999" {
			t.Errorf("error = %v, want invalid id %%999", err)
		}
	})

	t.Run("spill of an untyped source", func(t *testing.T) {
		p, fs := spillFixture()
		defineValue(p, fs, 200, 20)
		lookup := p.lookupExpression[200]
		lookup.typeID = 98
		p.lookupExpression[200] = lookup
		fs.phis = []phiExpression{{expressions: []phiSource{{id: 200, blockID: 10}}}}
		if err := p.appendPhiStores(fs); err == nil || err.Error() != "invalid id %98" {
			t.Errorf("error = %v, want invalid id %%98", err)
		}
	})

	t.Run("predecessor outside every body is the top body", func(t *testing.T) {
		p, fs := spillFixture()
		// Block 77 has no body (a function's first block): it counts as the
		// top body, which is not inside body 2 where the value lives.
		defineValue(p, fs, 200, 20)
		fs.phis = []phiExpression{{expressions: []phiSource{{id: 200, blockID: 77}}}}
		if err := p.appendPhiStores(fs); err != nil {
			t.Fatalf("appendPhiStores: %v", err)
		}
		if len(fs.blocks[20]) != 1 || len(fs.blocks[77]) != 2 {
			t.Errorf("blocks 20/77 = %d/%d statements, want a spill and a reload with the store",
				len(fs.blocks[20]), len(fs.blocks[77]))
		}
	})
}
