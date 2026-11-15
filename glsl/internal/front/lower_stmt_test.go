package front

import (
	"fmt"
	"slices"
	"testing"

	"github.com/gogpu/naga/ir"
)

// This file tests the statement lowering against the shapes Rust naga builds
// while parsing the same statements (front/glsl/parser/functions.rs). Each test
// names the Rust arm it mirrors, because the statement and expression order is
// the contract: the Emit ranges decide the IR and the backend output.

// stmtOf packs a statement kind as the AST node lowering consumes.
func stmtOf(kind astStmtKind) astStmt {
	return astStmt{Kind: kind}
}

// stmtPtr returns a pointer to a statement, for the optional branch fields.
func stmtPtr(s astStmt) *astStmt { return &s }

// exprRoot packs an arena and its root expression.
func exprRoot(a *hirArena, root hirHandle) exprs {
	return exprs{Arena: *a, Root: root}
}

// oneExpr returns an expression arena with one root node.
func oneExpr(kind hirExpr) exprs {
	a := &hirArena{}
	return exprRoot(a, a.add(kind, ir.Span{}))
}

// varExpr returns the HIR reference to a declared name.
func varExpr(sym *glslSymbol) exprs { return oneExpr(hirVariable{Sym: sym}) }

// exprsPtr returns a pointer to an expression arena value, for the optional
// expression fields of the AST.
func exprsPtr(e exprs) *exprs { return &e }

// litExpr returns a literal expression.
func litExpr(value ir.LiteralValue) exprs { return oneExpr(hirLiteral{Value: value}) }

// assignExpr builds `target = value` as the expression a statement evaluates.
func assignExpr(target, value hirExpr) exprs {
	a := &hirArena{}
	t := a.add(target, ir.Span{})
	v := a.add(value, ir.Span{})
	return exprRoot(a, a.add(hirAssign{Tgt: t, Value: v}, ir.Span{}))
}

// assignStmt builds `target = value;`.
func assignStmt(target, value hirExpr) astStmt {
	return stmtOf(stmtExpr{Expr: assignExpr(target, value)})
}

// typeSpecOf returns the type specification of a builtin type name.
func typeSpecOf(t *testing.T, name string) typeSpec {
	t.Helper()
	tn, ok := parseTypeName(name)
	if !ok {
		t.Fatalf("parseTypeName(%q) failed", name)
	}
	return typeSpec{Builtin: &tn}
}

// mustLower lowers stmts as a function body and fails the test on error.
func mustLower(t *testing.T, c *Context, stmts ...astStmt) {
	t.Helper()
	if err := c.lowerStatements(stmts); err != nil {
		t.Fatalf("lowerStatements: %v", err)
	}
}

// stmtName names a statement kind for a shape assertion.
func stmtName(kind ir.StatementKind) string {
	switch kind.(type) {
	case ir.StmtEmit:
		return "emit"
	case ir.StmtBlock:
		return "block"
	case ir.StmtIf:
		return "if"
	case ir.StmtSwitch:
		return "switch"
	case ir.StmtLoop:
		return "loop"
	case ir.StmtBreak:
		return "break"
	case ir.StmtContinue:
		return "continue"
	case ir.StmtReturn:
		return "return"
	case ir.StmtKill:
		return "kill"
	case ir.StmtStore:
		return "store"
	default:
		return fmt.Sprintf("%T", kind)
	}
}

// wantShape asserts the kinds of a block in order.
func wantShape(t *testing.T, what string, body ir.Block, want ...string) {
	t.Helper()
	got := make([]string, len(body))
	for i := range body {
		got[i] = stmtName(body[i].Kind)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s shape = %v, want %v", what, got, want)
	}
}

// stmtAs asserts the kind of one IR statement.
func stmtAs[T any](t *testing.T, s ir.Statement, what string) T {
	t.Helper()
	v, ok := s.Kind.(T)
	if !ok {
		var zero T
		t.Fatalf("%s is %T, want %T", s.Kind, s.Kind, zero)
	}
	return v
}

// exprAs asserts the kind of one IR expression.
func exprAs[T any](t *testing.T, c *Context, h ir.ExpressionHandle, what string) T {
	t.Helper()
	v, ok := c.Expression(h).(T)
	if !ok {
		var zero T
		t.Fatalf("%s is %T, want %T", what, c.Expression(h), zero)
	}
	return v
}

// TestStmtExpressionOrder checks that two expression statements are lowered in
// source order, each with its own Emit range before the store it ends in
// (parser/functions.rs:570-578).
func TestStmtExpressionOrder(t *testing.T) {
	fe, c := newTestContext(t)
	ty := fe.registerType(f32Scalar)
	a := declareLocal(t, c, "a", ty)
	b := declareLocal(t, c, "b", ty)
	d := declareLocal(t, c, "d", ty)

	mustLower(t, c,
		assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}),
		assignStmt(hirVariable{Sym: a}, hirVariable{Sym: d}),
	)
	wantShape(t, "function", c.body, "emit", "store", "emit", "store")

	first := stmtAs[ir.StmtStore](t, c.body[1], "first store")
	second := stmtAs[ir.StmtStore](t, c.body[3], "second store")
	if first.Pointer != c.vars[a].expr || second.Pointer != c.vars[a].expr {
		t.Fatalf("the stores target %d and %d, want the local a at %d",
			first.Pointer, second.Pointer, c.vars[a].expr)
	}
	if first.Value >= second.Value {
		t.Fatalf("the second value is expression %d and the first %d, want source order",
			second.Value, first.Value)
	}
	if load := exprAs[ir.ExprLoad](t, c, first.Value, "first value"); load.Pointer != c.vars[b].expr {
		t.Fatalf("the first store reads %d, want b at %d", load.Pointer, c.vars[b].expr)
	}
	if load := exprAs[ir.ExprLoad](t, c, second.Value, "second value"); load.Pointer != c.vars[d].expr {
		t.Fatalf("the second store reads %d, want d at %d", load.Pointer, c.vars[d].expr)
	}
	for _, emit := range []ir.StmtEmit{
		stmtAs[ir.StmtEmit](t, c.body[0], "first emit"),
		stmtAs[ir.StmtEmit](t, c.body[2], "second emit"),
	} {
		if emit.Range.Start >= emit.Range.End {
			t.Fatalf("emit range [%d, %d) is empty", emit.Range.Start, emit.Range.End)
		}
	}
}

// TestStmtLocalDeclarationInitializer checks a local declared with a constant
// initializer: the initializer is lowered first and becomes the LocalVariable's
// init, so no store is emitted (parser/declarations.rs:224-272,
// variables.rs:683-702).
func TestStmtLocalDeclarationInitializer(t *testing.T) {
	fe, c := newTestContext(t)
	f32Ty := fe.registerType(f32Scalar)
	sym := &glslSymbol{Kind: symLocal, Name: "x"}
	arena := &hirArena{}
	root := arena.add(hirLiteral{Value: ir.LiteralF32(1.5)}, ir.Span{})

	decl := declVarList{
		Typ: typeSpecOf(t, "float"),
		Declarators: []declarator{{
			Name: "x",
			Sym:  sym,
			Init: &initializer{Arena: *arena, Root: initNode{Expr: root}},
		}},
	}
	mustLower(t, c, stmtOf(stmtDecl{Decl: astDecl{Kind: decl}}))

	if len(c.locals) != 1 {
		t.Fatalf("context has %d locals, want 1", len(c.locals))
	}
	local := c.locals[0]
	if local.Name != "x" || local.Type != f32Ty {
		t.Fatalf("local = %+v, want x of f32", local)
	}
	if local.Init == nil {
		t.Fatal("the constant initializer is not the local's init")
	}
	// The initializer is appended before the local-variable expression.
	if *local.Init != 0 {
		t.Fatalf("init = expression %d, want the first expression appended", *local.Init)
	}
	lit := exprAs[ir.Literal](t, c, *local.Init, "initializer")
	if lit.Value != ir.LiteralF32(1.5) {
		t.Fatalf("init value = %v, want 1.5", lit.Value)
	}
	localExpr := exprAs[ir.ExprLocalVariable](t, c, 1, "local expression")
	if int(localExpr.Variable) != 0 {
		t.Fatalf("the local expression reads variable %d, want 0", localExpr.Variable)
	}
	if len(c.body) != 0 {
		t.Fatalf("body = %v, want no statement: a const init needs no store", c.body)
	}
	ref := c.vars[sym]
	if ref == nil || !ref.load || !ref.mutable || ref.expr != 1 {
		t.Fatalf("the name resolves to %+v, want a writable load of expression 1", ref)
	}
}

// TestStmtLocalDeclarationStore checks the two cases that store instead of
// initializing the declaration: a run-time initializer and any declaration
// inside a loop (parser/declarations.rs:254-261).
func TestStmtLocalDeclarationStore(t *testing.T) {
	t.Run("run-time initializer", func(t *testing.T) {
		fe, c := newTestContext(t)
		w := declareLocal(t, c, "w", fe.registerType(f32Scalar))
		sym := &glslSymbol{Kind: symLocal, Name: "x"}
		init := varExpr(w)
		decl := declVarList{
			Typ: typeSpecOf(t, "float"),
			Declarators: []declarator{{
				Name: "x",
				Sym:  sym,
				Init: &initializer{Arena: init.Arena, Root: initNode{Expr: init.Root}},
			}},
		}
		mustLower(t, c, stmtOf(stmtDecl{Decl: astDecl{Kind: decl}}))

		if len(c.locals) != 2 {
			t.Fatalf("context has %d locals, want the local and x", len(c.locals))
		}
		if c.locals[1].Init != nil {
			t.Fatal("a run-time initializer must not become the local's init")
		}
		wantShape(t, "function", c.body, "emit", "store")
		store := stmtAs[ir.StmtStore](t, c.body[1], "initializer store")
		if _, ok := c.Expression(store.Pointer).(ir.ExprLocalVariable); !ok {
			t.Fatalf("the store writes through %T, want the new local", c.Expression(store.Pointer))
		}
		if load := exprAs[ir.ExprLoad](t, c, store.Value, "stored value"); load.Pointer != c.vars[w].expr {
			t.Fatalf("the store value reads %d, want w at %d", load.Pointer, c.vars[w].expr)
		}
	})

	t.Run("inside a loop", func(t *testing.T) {
		_, c := newTestContext(t)
		sym := &glslSymbol{Kind: symLocal, Name: "x"}
		arena := &hirArena{}
		root := arena.add(hirLiteral{Value: ir.LiteralF32(2)}, ir.Span{})
		decl := declVarList{
			Typ: typeSpecOf(t, "float"),
			Declarators: []declarator{{
				Name: "x",
				Sym:  sym,
				Init: &initializer{Arena: *arena, Root: initNode{Expr: root}},
			}},
		}
		scope := stmtScope{insideLoop: true, loops: 1, terminator: &stmtTerminator{}}
		if err := c.lowerStatementIn(stmtOf(stmtDecl{Decl: astDecl{Kind: decl}}), scope); err != nil {
			t.Fatalf("lowerStatementIn: %v", err)
		}
		if c.locals[0].Init != nil {
			t.Fatal("a declaration inside a loop must not hoist its initializer")
		}
		wantShape(t, "loop body", c.body, "store")
	})
}

// TestStmtLocalDeclarationBracedInitializer checks that a braced initializer
// composes the declared type, element by element
// (parser/declarations.rs:97-162).
func TestStmtLocalDeclarationBracedInitializer(t *testing.T) {
	fe, c := newTestContext(t)
	sym := &glslSymbol{Kind: symLocal, Name: "xs"}
	size := litExpr(ir.LiteralU32(2))
	arena := &hirArena{}
	root := initNode{Items: []initNode{
		{Expr: arena.add(hirLiteral{Value: ir.LiteralF32(1)}, ir.Span{})},
		{Expr: arena.add(hirLiteral{Value: ir.LiteralF32(2)}, ir.Span{})},
	}}
	decl := declVarList{
		Typ: typeSpecOf(t, "float"),
		Declarators: []declarator{{
			Name:   "xs",
			Sym:    sym,
			Arrays: []arraySpec{{Size: &size}},
			Init:   &initializer{Arena: *arena, Root: root},
		}},
	}
	mustLower(t, c, stmtOf(stmtDecl{Decl: astDecl{Kind: decl}}))

	if len(c.locals) != 1 {
		t.Fatalf("context has %d locals, want 1", len(c.locals))
	}
	local := c.locals[0]
	arr, ok := fe.typeInner(local.Type).(ir.ArrayType)
	if !ok {
		t.Fatalf("the declared type is %T, want an array", fe.typeInner(local.Type))
	}
	if arr.Size.Constant == nil || *arr.Size.Constant != 2 {
		t.Fatalf("array size = %+v, want 2", arr.Size)
	}
	if local.Init == nil {
		t.Fatal("the braced initializer is not the local's init")
	}
	compose := exprAs[ir.ExprCompose](t, c, *local.Init, "initializer")
	if compose.Type != local.Type {
		t.Fatalf("the compose type is %d, want the declared type %d", compose.Type, local.Type)
	}
	if len(compose.Components) != 2 {
		t.Fatalf("the compose has %d components, want 2", len(compose.Components))
	}
	if first := exprAs[ir.Literal](t, c, compose.Components[0], "first element"); first.Value != ir.LiteralF32(1) {
		t.Fatalf("first element = %v, want 1", first.Value)
	}
}

// TestStmtLocalDeclarationConvertsInitializer checks the implicit conversion of
// a scalar initializer to the declared type
// (parser/declarations.rs:155-158, 232-235).
func TestStmtLocalDeclarationConvertsInitializer(t *testing.T) {
	fe, c := newTestContext(t)
	u := declareLocal(t, c, "u", fe.registerType(u32Scalar))
	sym := &glslSymbol{Kind: symLocal, Name: "x"}
	init := varExpr(u)
	decl := declVarList{
		Typ: typeSpecOf(t, "float"),
		Declarators: []declarator{{
			Name: "x",
			Sym:  sym,
			Init: &initializer{Arena: init.Arena, Root: initNode{Expr: init.Root}},
		}},
	}
	mustLower(t, c, stmtOf(stmtDecl{Decl: astDecl{Kind: decl}}))

	wantShape(t, "function", c.body, "emit", "store")
	store := stmtAs[ir.StmtStore](t, c.body[1], "initializer store")
	as := exprAs[ir.ExprAs](t, c, store.Value, "converted initializer")
	if as.Kind != ir.ScalarFloat || as.Convert == nil || *as.Convert != 4 {
		t.Fatalf("conversion = %+v, want a conversion to f32", as)
	}
}

// TestStmtBlockCullsAfterTerminator checks the compound statement's own body:
// the statements after the first terminator are lowered and then removed from
// the block, and the block's terminator ends the enclosing body too
// (parser/functions.rs:545-568, 611-613).
func TestStmtBlockCullsAfterTerminator(t *testing.T) {
	fe, c := newTestContext(t)
	ty := fe.registerType(f32Scalar)
	a := declareLocal(t, c, "a", ty)
	b := declareLocal(t, c, "b", ty)

	block := stmtOf(stmtBlock{Body: []astStmt{
		stmtOf(stmtReturn{}),
		assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}),
	}})
	mustLower(t, c, block, assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}))

	// The block ends the function body, so the assignment after it is culled.
	wantShape(t, "function", c.body, "block")
	inner := stmtAs[ir.StmtBlock](t, c.body[0], "block")
	wantShape(t, "block body", inner.Block, "return")
	if value := stmtAs[ir.StmtReturn](t, inner.Block[0], "return"); value.Value != nil {
		t.Fatalf("the bare return carries value %d, want none", *value.Value)
	}
}

// TestStmtIfWithoutElse checks the if shape: the condition is lowered into the
// enclosing body, each branch into a block of its own, and the absent else is
// an empty block (parser/functions.rs:123-167).
func TestStmtIfWithoutElse(t *testing.T) {
	fe, c := newTestContext(t)
	cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))

	mustLower(t, c, stmtOf(stmtIf{
		Cond:   varExpr(cond),
		Accept: stmtPtr(assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b})),
	}))

	// The condition's Emit range is closed before the If, as Rust's
	// new_body of the accept branch restarts the emitter on entry.
	wantShape(t, "function", c.body, "emit", "if")
	ifStmt := stmtAs[ir.StmtIf](t, c.body[1], "if")
	wantShape(t, "accept", ifStmt.Accept, "emit", "store")
	if len(ifStmt.Reject) != 0 {
		t.Fatalf("reject = %v, want the empty block of an absent else", ifStmt.Reject)
	}
	load := exprAs[ir.ExprLoad](t, c, ifStmt.Condition, "condition")
	if load.Pointer != c.vars[cond].expr {
		t.Fatalf("the condition reads %d, want c at %d", load.Pointer, c.vars[cond].expr)
	}
}

// TestStmtIfWithElse checks that the else branch is its own block
// (parser/functions.rs:146-155).
func TestStmtIfWithElse(t *testing.T) {
	fe, c := newTestContext(t)
	cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))

	mustLower(t, c, stmtOf(stmtIf{
		Cond:   varExpr(cond),
		Accept: stmtPtr(assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b})),
		Reject: stmtPtr(assignStmt(hirVariable{Sym: b}, hirVariable{Sym: a})),
	}))

	wantShape(t, "function", c.body, "emit", "if")
	ifStmt := stmtAs[ir.StmtIf](t, c.body[1], "if")
	wantShape(t, "accept", ifStmt.Accept, "emit", "store")
	wantShape(t, "reject", ifStmt.Reject, "emit", "store")
	if accept := stmtAs[ir.StmtStore](t, ifStmt.Accept[1], "accept store"); accept.Pointer != c.vars[a].expr {
		t.Fatalf("the accept branch writes %d, want a at %d", accept.Pointer, c.vars[a].expr)
	}
	if reject := stmtAs[ir.StmtStore](t, ifStmt.Reject[1], "reject store"); reject.Pointer != c.vars[b].expr {
		t.Fatalf("the reject branch writes %d, want b at %d", reject.Pointer, c.vars[b].expr)
	}
}

// TestStmtWhile checks the loop Rust builds for a while statement: the body
// starts with an If whose condition is the negated one and whose accept breaks,
// and the statement follows that If (parser/functions.rs:334-381).
func TestStmtWhile(t *testing.T) {
	fe, c := newTestContext(t)
	cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))

	body := stmtOf(stmtBlock{Body: []astStmt{
		assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}),
	}})
	mustLower(t, c, stmtOf(stmtWhile{Cond: varExpr(cond), Body: body}))

	wantShape(t, "function", c.body, "loop")
	loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
	if loop.BreakIf != nil {
		t.Fatalf("break_if = %d, want none", *loop.BreakIf)
	}
	if len(loop.Continuing) != 0 {
		t.Fatalf("continuing = %v, want empty", loop.Continuing)
	}
	wantShape(t, "loop body", loop.Body, "emit", "if", "block")
	guard := stmtAs[ir.StmtIf](t, loop.Body[1], "loop condition")
	wantShape(t, "condition accept", guard.Accept, "break")
	if len(guard.Reject) != 0 {
		t.Fatalf("condition reject = %v, want empty", guard.Reject)
	}
	not := exprAs[ir.ExprUnary](t, c, guard.Condition, "loop condition")
	if not.Op != ir.UnaryLogicalNot {
		t.Fatalf("condition operator = %v, want LogicalNot", not.Op)
	}
	if load := exprAs[ir.ExprLoad](t, c, not.Expr, "negated condition"); load.Pointer != c.vars[cond].expr {
		t.Fatalf("the negated condition reads %d, want c at %d", load.Pointer, c.vars[cond].expr)
	}
}

// TestStmtDoWhile checks that a do-while puts its condition at the end of the
// loop body, and that a body which ends the iteration culls that condition
// (parser/functions.rs:382-434).
func TestStmtDoWhile(t *testing.T) {
	t.Run("condition at the end of the body", func(t *testing.T) {
		fe, c := newTestContext(t)
		cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
		a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
		b := declareLocal(t, c, "b", fe.registerType(f32Scalar))

		body := stmtOf(stmtBlock{Body: []astStmt{
			assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}),
		}})
		mustLower(t, c, stmtOf(stmtDo{Body: body, Cond: varExpr(cond)}))

		wantShape(t, "function", c.body, "loop")
		loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
		wantShape(t, "loop body", loop.Body, "block", "emit", "if")
		guard := stmtAs[ir.StmtIf](t, loop.Body[2], "loop condition")
		wantShape(t, "condition accept", guard.Accept, "break")
		if not := exprAs[ir.ExprUnary](t, c, guard.Condition, "loop condition"); not.Op != ir.UnaryLogicalNot {
			t.Fatalf("condition operator = %v, want LogicalNot", not.Op)
		}
	})

	t.Run("terminated body culls the condition", func(t *testing.T) {
		fe, c := newTestContext(t)
		cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
		body := stmtOf(stmtBlock{Body: []astStmt{stmtOf(stmtReturn{})}})
		mustLower(t, c, stmtOf(stmtDo{Body: body, Cond: varExpr(cond)}))

		wantShape(t, "function", c.body, "loop")
		loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
		wantShape(t, "loop body", loop.Body, "block")
	})

	t.Run("break statement as the body", func(t *testing.T) {
		fe, c := newTestContext(t)
		cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
		mustLower(t, c, stmtOf(stmtDo{Body: stmtOf(stmtBreak{}), Cond: varExpr(cond)}))

		wantShape(t, "function", c.body, "loop")
		loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
		// The break ends the iteration, so the guard after it is unreachable.
		wantShape(t, "loop body", loop.Body, "break")
	})
}

// TestStmtFor checks the for shape: the init clause is lowered into the
// enclosing body before the loop, the update clause into Continuing, and the
// condition into the loop body. Rust lowers the condition, then the update
// clause and only then the body, so the expression arena holds them in that
// order (parser/functions.rs:435-544).
func TestStmtFor(t *testing.T) {
	fe, c := newTestContext(t)
	cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))
	iSym := &glslSymbol{Kind: symLocal, Name: "i"}

	initArena := &hirArena{}
	initRoot := initArena.add(hirLiteral{Value: ir.LiteralI32(0)}, ir.Span{})
	initDecl := astDecl{Kind: declVarList{
		Typ: typeSpecOf(t, "int"),
		Declarators: []declarator{{
			Name: "i",
			Sym:  iSym,
			Init: &initializer{Arena: *initArena, Root: initNode{Expr: initRoot}},
		}},
	}}
	condExpr := varExpr(cond)
	update := assignExpr(hirVariable{Sym: iSym}, hirLiteral{Value: ir.LiteralI32(1)})
	body := stmtOf(stmtBlock{Body: []astStmt{
		assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}),
	}})

	mustLower(t, c, stmtOf(stmtFor{
		InitDecl: &initDecl,
		Cond:     &condExpr,
		Update:   &update,
		Body:     body,
	}))

	if len(c.locals) != 4 {
		t.Fatalf("context has %d locals, want the three parameters and i", len(c.locals))
	}
	if c.locals[3].Name != "i" || c.locals[3].Init == nil {
		t.Fatalf("the init declaration did not become a const-initialized local: %+v", c.locals[3])
	}
	wantShape(t, "function", c.body, "loop")
	loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
	wantShape(t, "loop body", loop.Body, "emit", "if", "block")
	wantShape(t, "continuing", loop.Continuing, "store")

	guard := stmtAs[ir.StmtIf](t, loop.Body[1], "loop condition")
	updateStore := stmtAs[ir.StmtStore](t, loop.Continuing[0], "update")
	bodyEmit := stmtAs[ir.StmtEmit](t, loop.Body[2].Kind.(ir.StmtBlock).Block[0], "body emit")
	// Rust's order: condition expressions, then the update clause, then the
	// body (functions.rs:452-530).
	if !(guard.Condition < updateStore.Value && updateStore.Value < bodyEmit.Range.Start) {
		t.Fatalf("expression order is condition %d, update %d, body %d; want condition < update < body",
			guard.Condition, updateStore.Value, bodyEmit.Range.Start)
	}
}

// TestStmtForWithoutCondition checks a for statement whose condition is absent:
// the loop body starts with its statement, since there is no guard to break on
// (parser/functions.rs:452-509).
func TestStmtForWithoutCondition(t *testing.T) {
	fe, c := newTestContext(t)
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))
	body := stmtOf(stmtBlock{Body: []astStmt{
		assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b}),
	}})

	mustLower(t, c, stmtOf(stmtFor{Body: body}))

	wantShape(t, "function", c.body, "loop")
	loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
	wantShape(t, "loop body", loop.Body, "block")
	if len(loop.Continuing) != 0 {
		t.Fatalf("continuing = %v, want empty", loop.Continuing)
	}
}

// TestStmtSwitch checks the switch shape: the selector's Emit is closed before
// the statement, a trailing break clears the fall-through and is culled, a case
// that does not break keeps it, and the default case is written as given
// (parser/functions.rs:168-333).
func TestStmtSwitch(t *testing.T) {
	fe, c := newTestContext(t)
	selector := declareLocal(t, c, "s", fe.registerType(i32Scalar))
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))
	assign := assignStmt(hirVariable{Sym: a}, hirVariable{Sym: b})
	one := litExpr(ir.LiteralI32(1))
	two := litExpr(ir.LiteralI32(2))

	mustLower(t, c, stmtOf(stmtSwitch{
		Selector: varExpr(selector),
		Cases: []switchCase{
			{Label: &one, Body: []astStmt{assign, stmtOf(stmtBreak{})}},
			{Label: &two, Body: []astStmt{assign}},
			{Body: []astStmt{assign, stmtOf(stmtBreak{})}},
		},
	}))

	wantShape(t, "function", c.body, "emit", "switch")
	sw := stmtAs[ir.StmtSwitch](t, c.body[1], "switch")
	if len(sw.Cases) != 3 {
		t.Fatalf("switch has %d cases, want 3", len(sw.Cases))
	}
	if value, ok := sw.Cases[0].Value.(ir.SwitchValueI32); !ok || value != 1 {
		t.Fatalf("first case value = %#v, want I32(1)", sw.Cases[0].Value)
	}
	if value, ok := sw.Cases[1].Value.(ir.SwitchValueI32); !ok || value != 2 {
		t.Fatalf("second case value = %#v, want I32(2)", sw.Cases[1].Value)
	}
	if _, ok := sw.Cases[2].Value.(ir.SwitchValueDefault); !ok {
		t.Fatalf("third case value = %#v, want the default", sw.Cases[2].Value)
	}
	if sw.Cases[0].FallThrough {
		t.Fatal("a case ending in break must not fall through")
	}
	if !sw.Cases[1].FallThrough {
		t.Fatal("a case without break must fall through")
	}
	if sw.Cases[2].FallThrough {
		t.Fatal("the last case must not fall through")
	}
	for i := range sw.Cases {
		wantShape(t, fmt.Sprintf("case %d", i), sw.Cases[i].Body, "emit", "store")
	}
	if len(c.fe.errors) != 0 {
		t.Fatalf("errors = %+v, want none: break inside a switch is legal", c.fe.errors)
	}
}

// TestStmtSwitchCaseRules checks the rules Rust applies to the cases as a whole
// (parser/functions.rs:296-328) and the duplicate label it leaves to the
// validator with a text of its own (valid/function.rs:133-140).
func TestStmtSwitchCaseRules(t *testing.T) {
	newSwitch := func(t *testing.T) (*Frontend, *Context, *glslSymbol, *glslSymbol) {
		t.Helper()
		fe, c := newTestContext(t)
		selector := declareLocal(t, c, "s", fe.registerType(i32Scalar))
		a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
		return fe, c, selector, a
	}

	t.Run("absent default gets an empty case", func(t *testing.T) {
		_, c, selector, a := newSwitch(t)
		one := litExpr(ir.LiteralI32(1))
		mustLower(t, c, stmtOf(stmtSwitch{
			Selector: varExpr(selector),
			Cases: []switchCase{{
				Label: &one,
				Body:  []astStmt{stmtOf(stmtExpr{Expr: assignExpr(hirVariable{Sym: a}, hirLiteral{Value: ir.LiteralF32(0)})})},
			}},
		}))
		sw := stmtAs[ir.StmtSwitch](t, c.body[1], "switch")
		if len(sw.Cases) != 2 {
			t.Fatalf("switch has %d cases, want the case and the added default", len(sw.Cases))
		}
		last := sw.Cases[1]
		if _, ok := last.Value.(ir.SwitchValueDefault); !ok {
			t.Fatalf("the added case is %#v, want the default", last.Value)
		}
		if len(last.Body) != 0 || last.FallThrough {
			t.Fatalf("the added default = %+v, want an empty case that does not fall through", last)
		}
	})

	t.Run("last case must have statements", func(t *testing.T) {
		_, c, selector, _ := newSwitch(t)
		one := litExpr(ir.LiteralI32(1))
		empty := litExpr(ir.LiteralI32(2))
		mustLower(t, c, stmtOf(stmtSwitch{
			Selector: varExpr(selector),
			Cases: []switchCase{
				{Label: &one, Body: []astStmt{stmtOf(stmtBreak{})}},
				{Label: &empty},
			},
		}))
		if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "last case/default label must be followed by statements" {
			t.Fatalf("errors = %+v, want the last-case error", c.fe.errors)
		}
	})

	t.Run("duplicate case values", func(t *testing.T) {
		_, c, selector, a := newSwitch(t)
		one := litExpr(ir.LiteralI32(1))
		again := litExpr(ir.LiteralI32(1))
		body := []astStmt{stmtOf(stmtExpr{Expr: assignExpr(hirVariable{Sym: a}, hirLiteral{Value: ir.LiteralF32(0)})})}
		mustLower(t, c, stmtOf(stmtSwitch{
			Selector: varExpr(selector),
			Cases: []switchCase{
				{Label: &one, Body: body},
				{Label: &again, Body: body},
			},
		}))
		if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "Multiple `switch` cases for I32(1) are present" {
			t.Fatalf("errors = %+v, want the duplicate-case error", c.fe.errors)
		}
	})

	t.Run("unsigned selector casts signed labels", func(t *testing.T) {
		fe, c := newTestContext(t)
		selector := declareLocal(t, c, "s", fe.registerType(u32Scalar))
		a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
		one := litExpr(ir.LiteralI32(1))
		mustLower(t, c, stmtOf(stmtSwitch{
			Selector: varExpr(selector),
			Cases: []switchCase{{
				Label: &one,
				Body:  []astStmt{stmtOf(stmtExpr{Expr: assignExpr(hirVariable{Sym: a}, hirLiteral{Value: ir.LiteralF32(0)})})},
			}},
		}))
		sw := stmtAs[ir.StmtSwitch](t, c.body[1], "switch")
		if value, ok := sw.Cases[0].Value.(ir.SwitchValueU32); !ok || value != 1 {
			t.Fatalf("first case value = %#v, want U32(1)", sw.Cases[0].Value)
		}
	})

	t.Run("non-integer label", func(t *testing.T) {
		_, c, selector, a := newSwitch(t)
		label := litExpr(ir.LiteralF32(1))
		mustLower(t, c, stmtOf(stmtSwitch{
			Selector: varExpr(selector),
			Cases: []switchCase{{
				Label: &label,
				Body:  []astStmt{stmtOf(stmtExpr{Expr: assignExpr(hirVariable{Sym: a}, hirLiteral{Value: ir.LiteralF32(0)})})},
			}},
		}))
		if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "Case values can only be integers" {
			t.Fatalf("errors = %+v, want the case-value error", c.fe.errors)
		}
		sw := stmtAs[ir.StmtSwitch](t, c.body[1], "switch")
		if value, ok := sw.Cases[0].Value.(ir.SwitchValueI32); !ok || value != 0 {
			t.Fatalf("the refused case value = %#v, want I32(0) as Rust writes it", sw.Cases[0].Value)
		}
	})
}

// TestStmtJumps checks return, discard, break and continue, and the Emit range
// the return closes before itself (parser/functions.rs:82-122).
func TestStmtJumps(t *testing.T) {
	t.Run("return keeps its value", func(t *testing.T) {
		fe, c := newTestContext(t)
		a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
		mustLower(t, c, stmtOf(stmtReturn{Value: exprsPtr(varExpr(a))}))

		wantShape(t, "function", c.body, "emit", "return")
		ret := stmtAs[ir.StmtReturn](t, c.body[1], "return")
		if ret.Value == nil {
			t.Fatal("the return does not carry its value")
		}
		emit := stmtAs[ir.StmtEmit](t, c.body[0], "return emit")
		if *ret.Value < emit.Range.Start || *ret.Value >= emit.Range.End {
			t.Fatalf("the returned expression %d is outside the Emit range [%d, %d)",
				*ret.Value, emit.Range.Start, emit.Range.End)
		}
	})
}

// TestStmtReturnClosesPendingRange checks that the expressions of an expression
// statement are emitted before the return that follows it, as Rust's
// emit_restart at the return does (parser/functions.rs:109).
func TestStmtReturnClosesPendingRange(t *testing.T) {
	fe, c := newTestContext(t)
	a := declareLocal(t, c, "a", fe.registerType(f32Scalar))
	b := declareLocal(t, c, "b", fe.registerType(f32Scalar))
	arena := &hirArena{}
	left := arena.add(hirVariable{Sym: a}, ir.Span{})
	right := arena.add(hirVariable{Sym: b}, ir.Span{})
	sum := arena.add(hirBinary{Left: left, Op: ir.BinaryAdd, Right: right}, ir.Span{})

	mustLower(t, c, stmtOf(stmtExpr{Expr: exprRoot(arena, sum)}), stmtOf(stmtReturn{}))

	wantShape(t, "function", c.body, "emit", "return")
	emit := stmtAs[ir.StmtEmit](t, c.body[0], "emit")
	if int(emit.Range.End)-int(emit.Range.Start) != 3 {
		t.Fatalf("emit range = [%d, %d), want the two loads and the sum", emit.Range.Start, emit.Range.End)
	}
}

// TestStmtDiscard checks that discard becomes the fragment shader's kill
// (parser/functions.rs:116-122).
func TestStmtDiscard(t *testing.T) {
	_, c := newTestContext(t)
	mustLower(t, c, stmtOf(stmtDiscard{}))
	wantShape(t, "function", c.body, "kill")
}

// TestStmtBreakContinueInsideLoop checks the jumps a loop body may hold, in the
// loop the while statement builds (parser/functions.rs:82-92).
func TestStmtBreakContinueInsideLoop(t *testing.T) {
	fe, c := newTestContext(t)
	cond := declareLocal(t, c, "c", fe.registerType(boolScalar))
	body := stmtOf(stmtBlock{Body: []astStmt{
		stmtOf(stmtContinue{}),
		stmtOf(stmtBreak{}),
	}})
	mustLower(t, c, stmtOf(stmtWhile{Cond: varExpr(cond), Body: body}))

	if len(c.fe.errors) != 0 {
		t.Fatalf("errors = %+v, want none inside a loop", c.fe.errors)
	}
	loop := stmtAs[ir.StmtLoop](t, c.body[0], "loop")
	inner := stmtAs[ir.StmtBlock](t, loop.Body[2], "loop body block")
	wantShape(t, "loop body block", inner.Block, "continue")
}

// TestStmtBreakContinueOutsideControlFlow checks the jumps that have no target:
// Rust's frontend lowers them and its validator rejects the module
// (valid/function.rs:118-121), which is the verdict reported here.
func TestStmtBreakContinueOutsideControlFlow(t *testing.T) {
	t.Run("break", func(t *testing.T) {
		_, c := newTestContext(t)
		if err := c.lowerStatement(stmtOf(stmtBreak{})); err != nil {
			t.Fatalf("lowerStatement: %v", err)
		}
		if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "The `break` is used outside of a `loop` or `switch` context" {
			t.Fatalf("errors = %+v, want the break error", c.fe.errors)
		}
	})

	t.Run("continue", func(t *testing.T) {
		_, c := newTestContext(t)
		if err := c.lowerStatement(stmtOf(stmtContinue{})); err != nil {
			t.Fatalf("lowerStatement: %v", err)
		}
		if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "The `continue` is used outside of a `loop` context" {
			t.Fatalf("errors = %+v, want the continue error", c.fe.errors)
		}
	})

	t.Run("continue in a switch", func(t *testing.T) {
		fe, c := newTestContext(t)
		selector := declareLocal(t, c, "s", fe.registerType(i32Scalar))
		one := litExpr(ir.LiteralI32(1))
		mustLower(t, c, stmtOf(stmtSwitch{
			Selector: varExpr(selector),
			Cases: []switchCase{{
				Label: &one,
				Body:  []astStmt{stmtOf(stmtContinue{}), stmtOf(stmtBreak{})},
			}},
		}))
		if len(c.fe.errors) != 1 || c.fe.errors[0].Message != "The `continue` is used outside of a `loop` context" {
			t.Fatalf("errors = %+v, want the continue error: a switch is no loop", c.fe.errors)
		}
	})
}
