package valid_test

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// notInScope is Rust naga's ExpressionError::NotInScope text.
const notInScope = "Used by a statement before it was introduced into the scope by any of the dominating blocks"

// storeShader stores a computed value: its body is Emit(load, add), Store,
// Return.
const storeShader = `@compute @workgroup_size(1) fn main() { var x: f32; let a = x + 1.0; x = a; }`

// emitAndStore returns the entry point's function and the indices of its
// one Emit and its one Store.
func emitAndStore(t *testing.T, m *ir.Module) (*ir.Function, int, int) {
	t.Helper()
	fn := &m.EntryPoints[0].Function
	emit, store := -1, -1
	for i, st := range fn.Body {
		switch st.Kind.(type) {
		case ir.StmtEmit:
			emit = i
		case ir.StmtStore:
			store = i
		}
	}
	if emit < 0 || store < 0 || emit > store {
		t.Fatalf("body %#v: want an Emit before a Store", fn.Body)
	}
	return fn, emit, store
}

// TestNotInScope ports Rust naga's scope rule (valid/function.rs
// validate_block_impl, resolve_type with valid_expression_set): a statement
// may use an expression only after its Emit, in its block or a block around
// it, or the result of a statement before it.
func TestNotInScope(t *testing.T) {
	t.Run("valid as lowered", func(t *testing.T) {
		expectValid(t, parse(t, storeShader), valid.Options{})
	})
	t.Run("used before its emit", func(t *testing.T) {
		m := parse(t, storeShader)
		fn, emit, store := emitAndStore(t, m)
		fn.Body[emit], fn.Body[store] = fn.Body[store], fn.Body[emit]
		value := fn.Body[emit].Kind.(ir.StmtStore).Value
		expectRejected(t, m, valid.Options{}, fmt.Sprintf("in function main, expression %d: %s", value, notInScope))
	})
	t.Run("emitted in a block that ended", func(t *testing.T) {
		m := parse(t, storeShader)
		fn, emit, _ := emitAndStore(t, m)
		fn.Body[emit] = ir.Statement{Kind: ir.StmtBlock{Block: ir.Block{fn.Body[emit]}}}
		expectRejected(t, m, valid.Options{}, notInScope)
	})
	t.Run("emitted in the other branch", func(t *testing.T) {
		m := parse(t, storeShader)
		fn, emit, store := emitAndStore(t, m)
		cond := ir.ExpressionHandle(len(fn.Expressions))
		fn.Expressions = append(fn.Expressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralBool(true)}})
		fn.Body = ir.Block{{Kind: ir.StmtIf{
			Condition: cond,
			Accept:    ir.Block{fn.Body[emit]},
			Reject:    ir.Block{fn.Body[store]},
		}}}
		expectRejected(t, m, valid.Options{}, notInScope)
	})
	t.Run("store pointer is not checked", func(t *testing.T) {
		// Rust walks the pointer's access chain without its types, so only
		// the value must be in scope.
		m := parse(t, `@group(0) @binding(0) var<storage, read_write> b: array<f32>;
@compute @workgroup_size(1) fn main() { b[0] = 1.0; }`)
		fn := &m.EntryPoints[0].Function
		var body ir.Block
		for _, st := range fn.Body {
			if _, ok := st.Kind.(ir.StmtEmit); !ok {
				body = append(body, st)
			}
		}
		fn.Body = body
		expectValid(t, m, valid.Options{})
	})
}

// TestScopeOfLoopsAndResults: the continuing block and break-if see the
// loop body's scope (Rust's Statement::Loop); a call's result comes into
// scope at the call, and a call argument out of scope is
// CallError::Argument.
func TestScopeOfLoopsAndResults(t *testing.T) {
	expectValid(t, parse(t, `@compute @workgroup_size(1) fn main() {
    var i = 0u;
    loop {
        let j = i + 1u;
        continuing { i = j; break if j > 3u; }
    }
}`), valid.Options{})

	m := parse(t, `fn f(v: f32) -> f32 { return v; }
@compute @workgroup_size(1) fn main() { var x: f32; let r = f(x + 1.0); x = r; }`)
	expectValid(t, m, valid.Options{})
	fn := &m.EntryPoints[0].Function
	for i, st := range fn.Body {
		if _, ok := st.Kind.(ir.StmtCall); ok && i > 0 {
			if _, ok := fn.Body[i-1].Kind.(ir.StmtEmit); !ok {
				t.Fatalf("statement before the call is %T, want its argument's Emit", fn.Body[i-1].Kind)
			}
			fn.Body[i-1], fn.Body[i] = fn.Body[i], fn.Body[i-1]
			break
		}
	}
	expectRejected(t, m, valid.Options{}, "Call to [0] is invalid: Argument 0 expression is invalid: "+notInScope)
}

// TestExpressionAlreadyInScope: an expression comes into scope once (Rust's
// FunctionError::ExpressionAlreadyInScope): emitted twice, or emitted though
// it needs no Emit (a literal, a variable, an argument).
func TestExpressionAlreadyInScope(t *testing.T) {
	m := parse(t, storeShader)
	fn, emit, _ := emitAndStore(t, m)
	r := fn.Body[emit].Kind.(ir.StmtEmit).Range
	fn.Body = append(ir.Block{fn.Body[emit]}, fn.Body...)
	expectRejected(t, m, valid.Options{}, fmt.Sprintf("Expression [%d] can't be introduced - it's already in scope", r.Start))

	m = parse(t, storeShader)
	fn, _, _ = emitAndStore(t, m)
	variable := -1
	for h, e := range fn.Expressions {
		if _, ok := e.Kind.(ir.ExprLocalVariable); ok {
			variable = h
		}
	}
	h := ir.ExpressionHandle(variable)
	fn.Body = append(ir.Block{{Kind: ir.StmtEmit{Range: ir.Range{Start: h, End: h + 1}}}}, fn.Body...)
	expectRejected(t, m, valid.Options{}, fmt.Sprintf("Expression [%d] can't be introduced - it's already in scope", h))
}
