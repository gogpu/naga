package front

// This file is Rust naga's statement lowering: the arms of
// ParsingContext::parse_statement (front/glsl/parser/functions.rs:41-583) and
// parse_compound_statement (functions.rs:585-618), together with the local
// variable declaration Rust lowers while it parses
// (parser/declarations.rs:97-291 and variables.rs:665-707).
//
// Rust lowers while it parses, so the IR statement order and the expression
// arena order below are Rust's order, Emit flush points included: an Emit range
// decides what a backend may name, so a statement lowered in a different order
// would produce different output for the same shader. The comments name the
// Rust lines each shape comes from.

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// The two jumps that need a target carry Rust naga's validator texts
// (valid/function.rs:118-121). Rust's GLSL frontend does not check either — it
// lowers the statement and the validator rejects the module — so checking here
// moves the report, never the verdict: a shader Rust accepts is still accepted.
const (
	breakOutsideLoopText    = "The `break` is used outside of a `loop` or `switch` context"
	continueOutsideLoopText = "The `continue` is used outside of a `loop` context"
)

// stmtTerminator records the first statement that ends a body, Rust naga's
// `terminator: &mut Option<usize>` (parser/functions.rs:45): the index one past
// the statement, which is where the body is culled. A nil recorder belongs to a
// body nobody culls, Rust's `&mut None`.
type stmtTerminator struct {
	index int
	set   bool
}

// record notes a terminating statement pushed at index n-1, keeping the first
// one as Rust's Option::get_or_insert does (functions.rs:85, 91).
func (t *stmtTerminator) record(n int) {
	if t != nil && !t.set {
		t.index, t.set = n, true
	}
}

// stmtScope is the control flow a statement is lowered in. Rust threads
// is_inside_loop through parse_statement (functions.rs:46) and knows the
// enclosing loops from its call stack; the AST keeps neither, so the walk
// carries both.
type stmtScope struct {
	// insideLoop marks a statement lexically inside a loop body, Rust's
	// is_inside_loop: a local declaration there is initialized by a store
	// instead of by its LocalVariable (declarations.rs:254-261).
	insideLoop bool
	// loops and switches count the enclosing loop and switch statements, which
	// decide whether break and continue have a target.
	loops, switches int
	// terminator receives the end of the body being lowered, nil when nobody
	// culls that body.
	terminator *stmtTerminator
}

// unculled returns the scope of a body that is not culled, Rust's `&mut None`
// (functions.rs:139, 365, 526).
func (s stmtScope) unculled() stmtScope {
	s.terminator = nil
	return s
}

// inLoop returns the scope of a loop body: it is inside a loop for both its
// declarations and its jumps, and nothing culls its end.
func (s stmtScope) inLoop() stmtScope {
	return stmtScope{insideLoop: true, loops: s.loops + 1, switches: s.switches}
}

// inSwitch returns the scope of a switch case: a switch is a break target but
// neither a loop nor a continue target, and it is not is_inside_loop
// (functions.rs:265-270).
func (s stmtScope) inSwitch() stmtScope {
	return stmtScope{insideLoop: s.insideLoop, loops: s.loops, switches: s.switches + 1}
}

// lowerStatements lowers stmts into c.body in order, the statements of a
// function body as Rust's parse_compound_statement lowers them
// (parser/functions.rs:585-618): every statement is lowered, then the ones
// after the first terminator are culled from the body, which is Rust's
// Block::cull(idx..) at the end of the compound statement (functions.rs:611-613).
func (c *Context) lowerStatements(stmts []astStmt) *frontError {
	t := &stmtTerminator{}
	if err := c.lowerStatementList(stmts, stmtScope{terminator: t}); err != nil {
		return err
	}
	c.cull(t)
	return nil
}

// lowerStatement lowers one statement into c.body with no enclosing control
// flow, Rust naga's parse_statement as a loop body calls it (functions.rs:41).
func (c *Context) lowerStatement(stmt astStmt) *frontError {
	return c.lowerStatementIn(stmt, stmtScope{})
}

// lowerStatementList lowers stmts in order, recording the first terminator in
// the scope's recorder.
func (c *Context) lowerStatementList(stmts []astStmt, scope stmtScope) *frontError {
	for i := range stmts {
		if err := c.lowerStatementIn(stmts[i], scope); err != nil {
			return err
		}
	}
	return nil
}

// cull drops the statements after the first terminator of the body, Rust naga's
// Block::cull(idx..) (ir/block.rs:59-62).
func (c *Context) cull(t *stmtTerminator) {
	if t != nil && t.set {
		c.body = c.body[:t.index]
	}
}

// lowerStatementIn lowers one statement with its control flow, Rust naga's
// parse_statement (parser/functions.rs:41-583).
func (c *Context) lowerStatementIn(stmt astStmt, scope stmtScope) *frontError {
	switch k := stmt.Kind.(type) {
	case stmtEmpty:
		// A lone semicolon lowers to nothing (functions.rs:569).
		return nil
	case stmtBlock:
		return c.lowerBlock(k, scope)
	case stmtDecl:
		return c.lowerDeclaration(&k.Decl, scope)
	case stmtExpr:
		// An expression statement is lowered for its side effects. Rust uses
		// lower, not lower_expect: a call that yields nothing is fine and a
		// value is dropped without a warning (functions.rs:570-578).
		_, _, _, err := c.lower(&k.Expr.Arena, k.Expr.Root, rhsPos)
		return err
	case stmtIf:
		return c.lowerIf(k, scope)
	case stmtSwitch:
		return c.lowerSwitch(k, stmt.Span, scope)
	case stmtWhile:
		return c.lowerWhile(k, scope)
	case stmtDo:
		return c.lowerDoWhile(k, scope)
	case stmtFor:
		return c.lowerFor(k, scope)
	case stmtReturn:
		return c.lowerReturn(k, scope)
	case stmtBreak:
		if scope.loops == 0 && scope.switches == 0 {
			c.fe.addError(errMessage(stmt.Span, breakOutsideLoopText))
		}
		c.body = append(c.body, ir.Statement{Kind: ir.StmtBreak{}})
		scope.terminator.record(len(c.body))
		return nil
	case stmtContinue:
		if scope.loops == 0 {
			c.fe.addError(errMessage(stmt.Span, continueOutsideLoopText))
		}
		c.body = append(c.body, ir.Statement{Kind: ir.StmtContinue{}})
		scope.terminator.record(len(c.body))
		return nil
	case stmtDiscard:
		// discard is the fragment shader's kill (functions.rs:116-122).
		c.body = append(c.body, ir.Statement{Kind: ir.StmtKill{}})
		scope.terminator.record(len(c.body))
		return nil
	default:
		return errInternal(fmt.Sprintf("%T cannot be lowered as a statement", stmt.Kind), stmt.Span)
	}
}

// lowerBlock lowers a compound statement: its statements go into a block of
// their own, whose end is culled, and the block's terminator ends the enclosing
// body as well (functions.rs:545-568 and 585-618).
func (c *Context) lowerBlock(k stmtBlock, scope stmtScope) *frontError {
	block := ir.Block{}
	inner := scope.unculled()
	inner.terminator = &stmtTerminator{}
	err := c.swapBody(&block, func() *frontError {
		if err := c.lowerStatementList(k.Body, inner); err != nil {
			return err
		}
		c.cull(inner.terminator)
		return nil
	})
	if err != nil {
		return err
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtBlock{Block: block}})
	if inner.terminator.set {
		scope.terminator.record(len(c.body))
	}
	return nil
}

// lowerDeclaration lowers a declaration used as a statement, Rust naga's
// parse_declaration with external = false (parser/declarations.rs:294-531): in a
// function body only a variable declaration list reaches the IR.
func (c *Context) lowerDeclaration(decl *astDecl, scope stmtScope) *frontError {
	switch k := decl.Kind.(type) {
	case declVarList:
		return c.lowerVarList(&k, scope)
	case declInvalid:
		// The parser's recovery node for a declaration it already reported, as
		// Rust returns after reporting one (declarations.rs:400-416).
		return nil
	case declPrecision, declQualifiers, declInvariant:
		// None of these carries an IR statement. Rust reaches them in the same
		// place for a local position and only touches its shader metadata and
		// its error list there (declarations.rs:436-530); the metadata is the
		// frontend's, not the body's, and our parser already holds it in the
		// declaration node for the module-scope lowering to apply.
		return nil
	default:
		return errInternal(fmt.Sprintf("%T cannot be declared in a statement", decl.Kind), decl.Span)
	}
}

// lowerVarList lowers a variable declaration list inside a function body: one
// local per declarator, Rust naga's parse_init_declarator_list for a local
// declaration (parser/declarations.rs:167-291).
func (c *Context) lowerVarList(list *declVarList, scope stmtScope) *frontError {
	base, err := c.fe.resolveTypeSpec(&list.Typ, structLayoutNone)
	if err != nil {
		return err
	}
	for i := range list.Declarators {
		if err := c.lowerDeclarator(&list.Qualifiers, base, &list.Declarators[i], scope); err != nil {
			return err
		}
	}
	return nil
}

// lowerDeclarator lowers one name of a declaration list: its dimensions, its
// initializer and the local variable that carries them
// (declarations.rs:195-272).
func (c *Context) lowerDeclarator(qualifiers *typeQualifiers, base ir.TypeHandle, d *declarator, scope stmtScope) *frontError {
	ty, err := c.applyArrays(base, d.Arrays)
	if err != nil {
		return err
	}
	// The initializer is lowered before the variable exists, so the expression
	// that reads the name is appended after it (declarations.rs:224-272).
	var value ir.ExpressionHandle
	constant := false
	resolveConstant := false
	if d.Init != nil {
		value, err = c.lowerInitializer(&d.Init.Arena, d.Init.Root, ty)
		if err != nil {
			return err
		}
		constant = c.eval.IsConst(value)
		resolveConstant = c.eval.IsConstWithConstLocals(value)
	}
	// A constant initializer outside a loop is the local's initial value; any
	// other one is stored where the declaration is written, so a loop
	// re-initializes the variable every iteration (declarations.rs:254-261).
	// This decision stays strict: an initializer that is constant only through
	// a const local (`const int b = a + 1`) keeps the run-time store, so no load
	// is ever written into LocalVariable.Init.
	var init *ir.ExpressionHandle
	late := false
	if d.Init != nil {
		if scope.insideLoop || !constant {
			late = true
		} else {
			init = &value
		}
	}
	mutable := c.localMutability(qualifiers)
	handle, pointer, err := c.addLocalVariable(d.Sym, d.Name, ty, init, mutable, d.Span)
	if err != nil {
		return err
	}
	// A const-qualified local is a constant expression (GLSL 4.50 §4.3.3), so
	// record the initializer of every const local whose initializer is constant
	// in the resolving view, which lets a chain through another const local
	// (`const int b = a + 1`) resolve too. Only call sites that require a
	// constant expression consult the record; the lowering path stays strict,
	// so the local and its loads stay in the IR exactly as the parity goldens
	// record them. A mutable local, and a const one whose initializer is not
	// constant (which GLSL forbids but which we do not reject), is never
	// recorded, keeping the pre-existing run-time behavior.
	if !mutable && resolveConstant {
		c.localConsts[handle] = value
	}
	if late {
		c.emitRestart()
		c.body = append(c.body, ir.Statement{Kind: ir.StmtStore{Pointer: pointer, Value: value}})
	}
	return nil
}

// localMutability reads the storage qualifier of a local declaration, Rust
// naga's Frontend::add_local_var (variables.rs:670-681): a plain local is
// writable, a const local is not, and any other storage class is an error that
// still leaves a writable local behind.
func (c *Context) localMutability(qualifiers *typeQualifiers) bool {
	switch qualifiers.Storage {
	case storageNone:
		return true
	case storageConst:
		return false
	default:
		c.fe.addError(errMessage(qualifiers.StorageSpan, "Locals cannot have a storage qualifier"))
		return true
	}
}

// addLocalVariable declares one local and returns its handle together with the
// expression that reads the name, Rust naga's Frontend::add_local_var
// (variables.rs:683-706): the LocalVariable first, then the expression that
// names it.
func (c *Context) addLocalVariable(sym *glslSymbol, name string, ty ir.TypeHandle, init *ir.ExpressionHandle, mutable bool, span ir.Span) (ir.LocalVariableHandle, ir.ExpressionHandle, *frontError) {
	h := ir.LocalVariableHandle(len(c.locals))
	c.locals = append(c.locals, ir.LocalVariable{Name: name, Type: ty, Init: init})
	expr, err := c.addExpression(ir.ExprLocalVariable{Variable: h}, span)
	if err != nil {
		return 0, 0, err
	}
	// A repeated name in the same scope is reported where the parser declares
	// it (parser_decl.go's declareLocal), so the replacement is silent here.
	c.addLocalVar(sym, expr, mutable)
	return h, expr, nil
}

// lowerInitializer lowers one initializer to the value of ty, Rust naga's
// parse_initializer (parser/declarations.rs:97-162): a braced list composes the
// type element by element, and a single expression is implicitly converted to
// the type's scalar components.
func (c *Context) lowerInitializer(arena *hirArena, node initNode, ty ir.TypeHandle) (ir.ExpressionHandle, *frontError) {
	if node.Items != nil {
		components := make([]ir.ExpressionHandle, 0, len(node.Items))
		for i := range node.Items {
			elem := c.initializerElementType(ty, i)
			h, err := c.lowerInitializer(arena, node.Items[i], elem)
			if err != nil {
				return 0, err
			}
			components = append(components, h)
		}
		return c.addExpression(ir.ExprCompose{Type: ty, Components: components}, node.Span)
	}
	h, span, err := c.lowerExpect(arena, node.Expr, rhsPos)
	if err != nil {
		return 0, err
	}
	if scalar, ok := scalarComponents(c.mod.Types[ty].Inner); ok {
		return c.implicitConversion(h, span, scalar)
	}
	return h, nil
}

// initializerElementType returns the type of the i-th element of a braced
// initializer, Rust naga's element_or_member_type (parser/declarations.rs:31-66):
// the element of an array, the member of a struct, the column vector of a
// matrix, the scalar of a vector, and the type itself for anything else.
func (c *Context) initializerElementType(ty ir.TypeHandle, i int) ir.TypeHandle {
	switch t := c.mod.Types[ty].Inner.(type) {
	case ir.VectorType:
		return c.fe.registerType(t.Scalar)
	case ir.MatrixType:
		return c.fe.registerType(ir.VectorType{Size: t.Rows, Scalar: t.Scalar})
	case ir.ArrayType:
		return t.Base
	case ir.StructType:
		if i < len(t.Members) {
			return t.Members[i].Type
		}
		return ty
	default:
		return ty
	}
}

// lowerIf lowers an if statement: the condition first, then each branch into a
// block of its own, the absent else as an empty block
// (parser/functions.rs:123-167).
func (c *Context) lowerIf(k stmtIf, scope stmtScope) *frontError {
	condition, _, err := c.lowerExpect(&k.Cond.Arena, k.Cond.Root, rhsPos)
	if err != nil {
		return err
	}
	accept := ir.Block{}
	err = c.swapBody(&accept, func() *frontError {
		return c.lowerStatementIn(*k.Accept, scope.unculled())
	})
	if err != nil {
		return err
	}
	reject := ir.Block{}
	err = c.swapBody(&reject, func() *frontError {
		if k.Reject == nil {
			return nil
		}
		return c.lowerStatementIn(*k.Reject, scope.unculled())
	})
	if err != nil {
		return err
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtIf{
		Condition: condition,
		Accept:    accept,
		Reject:    reject,
	}})
	return nil
}

// lowerWhile lowers a while statement to a loop whose body starts with an If
// that breaks on the negated condition and then holds the statement
// (parser/functions.rs:334-381).
func (c *Context) lowerWhile(k stmtWhile, scope stmtScope) *frontError {
	loopBody := ir.Block{}
	err := c.swapBody(&loopBody, func() *frontError {
		guard, err := c.lowerLoopGuard(&k.Cond)
		if err != nil {
			return err
		}
		c.body = append(c.body, guard)
		return c.lowerStatementIn(k.Body, scope.inLoop())
	})
	if err != nil {
		return err
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtLoop{Body: loopBody, Continuing: ir.Block{}}})
	return nil
}

// lowerDoWhile lowers a do-while statement: the body first, then the guard at
// the end of the loop body, then the cull of everything the body's terminator
// made unreachable — the guard included (parser/functions.rs:382-434).
func (c *Context) lowerDoWhile(k stmtDo, scope stmtScope) *frontError {
	loopBody := ir.Block{}
	err := c.swapBody(&loopBody, func() *frontError {
		body := scope.inLoop()
		body.terminator = &stmtTerminator{}
		if err := c.lowerStatementIn(k.Body, body); err != nil {
			return err
		}
		guard, err := c.lowerLoopGuard(&k.Cond)
		if err != nil {
			return err
		}
		c.body = append(c.body, guard)
		c.cull(body.terminator)
		return nil
	})
	if err != nil {
		return err
	}
	c.body = append(c.body, ir.Statement{Kind: ir.StmtLoop{Body: loopBody, Continuing: ir.Block{}}})
	return nil
}

// lowerFor lowers a for statement. The init clause goes into the enclosing body
// before the loop; inside the loop body come the guard and then the statement;
// the update clause is lowered into Continuing before the statement is lowered,
// which is Rust's order and therefore the expression arena's order
// (parser/functions.rs:435-544).
func (c *Context) lowerFor(k stmtFor, scope stmtScope) *frontError {
	if k.InitDecl != nil {
		if err := c.lowerDeclaration(k.InitDecl, scope.unculled()); err != nil {
			return err
		}
	} else if k.InitExpr != nil {
		// Rust lowers the init expression without expecting a value
		// (functions.rs:445-448).
		if _, _, _, err := c.lower(&k.InitExpr.Arena, k.InitExpr.Root, rhsPos); err != nil {
			return err
		}
	}

	loopBody := ir.Block{}
	err := c.swapBody(&loopBody, func() *frontError {
		if k.Cond == nil {
			return nil
		}
		guard, err := c.lowerLoopGuard(k.Cond)
		if err != nil {
			return err
		}
		c.body = append(c.body, guard)
		return nil
	})
	if err != nil {
		return err
	}

	continuing := ir.Block{}
	err = c.swapBody(&continuing, func() *frontError {
		if k.Update == nil {
			return nil
		}
		// The update clause is lowered for its side effects, as Rust does with
		// ctx.lower (functions.rs:511-521).
		_, _, _, err := c.lower(&k.Update.Arena, k.Update.Root, rhsPos)
		return err
	})
	if err != nil {
		return err
	}

	// Rust re-enters the loop body it already started, Rust's
	// Context::with_body (functions.rs:525-530).
	err = c.swapBody(&loopBody, func() *frontError {
		return c.lowerStatementIn(k.Body, scope.inLoop())
	})
	if err != nil {
		return err
	}

	c.body = append(c.body, ir.Statement{Kind: ir.StmtLoop{
		Body:       loopBody,
		Continuing: continuing,
	}})
	return nil
}

// lowerLoopGuard lowers a loop condition and the If that breaks out of the loop
// when it fails, Rust's negated condition (parser/functions.rs:343-361): the
// condition is negated with a logical not expression of the condition's own
// span, and the Emit range of both is closed before the If.
func (c *Context) lowerLoopGuard(cond *exprs) (ir.Statement, *frontError) {
	value, span, err := c.lowerExpect(&cond.Arena, cond.Root, rhsPos)
	if err != nil {
		return ir.Statement{}, err
	}
	condition, err := c.addExpression(ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: value}, span)
	if err != nil {
		return ir.Statement{}, err
	}
	c.emitRestart()
	return ir.Statement{Kind: ir.StmtIf{
		Condition: condition,
		Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
		Reject:    ir.Block{},
	}}, nil
}

// lowerSwitch lowers a switch statement, applying the GLSL fall-through rules
// as Rust does (parser/functions.rs:168-333): a break at the end of a case is
// culled and clears the fall-through, the last case must not fall through, and
// a switch without a default gets an empty one, which the IR requires.
func (c *Context) lowerSwitch(k stmtSwitch, span ir.Span, scope stmtScope) *frontError {
	selector, selectorSpan, err := c.lowerExpect(&k.Selector.Arena, k.Selector.Root, rhsPos)
	if err != nil {
		return err
	}
	unsigned := false
	if scalar, ok, serr := c.exprScalarComponents(selector, selectorSpan); serr != nil {
		return serr
	} else if ok {
		unsigned = scalar.Kind == ir.ScalarUint
	}
	c.emitRestart()

	cases := make([]ir.SwitchCase, 0, len(k.Cases))
	defaultPresent := false
	for i := range k.Cases {
		one := &k.Cases[i]
		var value ir.SwitchValue
		if one.Label == nil {
			defaultPresent = true
			value = ir.SwitchValueDefault{}
		} else if value, err = c.caseLabel(one.Label, unsigned); err != nil {
			return err
		}

		body := ir.Block{}
		caseScope := scope.inSwitch()
		caseScope.terminator = &stmtTerminator{}
		fallThrough := true
		err = c.swapBody(&body, func() *frontError {
			if err := c.lowerStatementList(one.Body, caseScope); err != nil {
				return err
			}
			fallThrough = !c.cullCase(caseScope.terminator)
			return nil
		})
		if err != nil {
			return err
		}
		cases = append(cases, ir.SwitchCase{Value: value, Body: body, FallThrough: fallThrough})
	}

	if len(cases) > 0 {
		last := &cases[len(cases)-1]
		// GLSL requires that the last case not be empty; a break counts as a
		// statement but was culled, so the fall-through is checked too
		// (functions.rs:298-310).
		if len(last.Body) == 0 && last.FallThrough {
			c.fe.addError(errMessage(span, "last case/default label must be followed by statements"))
		}
		// The IR requires that the last case not fall through, while GLSL
		// allows the last case to omit its break (functions.rs:312-317).
		last.FallThrough = false
	}
	// The IR requires a default case, GLSL does not (functions.rs:319-328).
	if !defaultPresent {
		cases = append(cases, ir.SwitchCase{Value: ir.SwitchValueDefault{}, Body: ir.Block{}})
	}
	c.checkSwitchCases(cases, span)

	c.body = append(c.body, ir.Statement{Kind: ir.StmtSwitch{Selector: selector, Cases: cases}})
	return nil
}

// cullCase culls a case body from the first terminator, as Rust does for a
// switch case (functions.rs:275-282): a trailing break clears the fall-through
// and is removed with everything after it, and any other terminator keeps its
// statement but drops what follows. endedInBreak reports a terminator that was
// a break, which clears the case's fall-through.
func (c *Context) cullCase(t *stmtTerminator) (endedInBreak bool) {
	if t == nil || !t.set {
		return false
	}
	idx := t.index
	if _, ok := c.body[idx-1].Kind.(ir.StmtBreak); ok {
		endedInBreak = true
		idx--
	}
	c.body = c.body[:idx]
	return endedInBreak
}

// caseLabel evaluates a case label, a constant expression of the module scope,
// Rust naga's parse_constant_expression with the literal check of the switch arm
// (parser/functions.rs:197-226). A label Rust cannot read as an integer is
// reported and becomes I32(0), as Rust goes on with it.
func (c *Context) caseLabel(label *exprs, unsigned bool) (ir.SwitchValue, *frontError) {
	ctx, err := newContext(c.fe, true)
	if err != nil {
		return nil, err
	}
	root, span, err := ctx.lowerExpect(&label.Arena, label.Root, rhsPos)
	if err != nil {
		return nil, err
	}
	lit, ok := ctx.mod.GlobalExpressions[root].Kind.(ir.Literal)
	if ok {
		switch v := lit.Value.(type) {
		case ir.LiteralI32:
			if unsigned {
				// Rust casts the signed label of an unsigned selector without a
				// check, and leaves the mismatch to validation, which matches
				// GLSL's rules (functions.rs:204-212).
				return ir.SwitchValueU32(uint32(v)), nil
			}
			return ir.SwitchValueI32(v), nil
		case ir.LiteralU32:
			return ir.SwitchValueU32(v), nil
		}
	}
	c.fe.addError(errMessage(span, "Case values can only be integers"))
	return ir.SwitchValueI32(0), nil
}

// checkSwitchCases reports duplicate labels. Rust naga's GLSL frontend keeps a
// repeated label and its validator rejects the module
// (valid/function.rs:133-140, 897-912), which is the verdict reported here with
// the validator's texts.
func (c *Context) checkSwitchCases(cases []ir.SwitchCase, span ir.Span) {
	seen := make(map[ir.SwitchValue]struct{}, len(cases))
	for i := range cases {
		value := cases[i].Value
		if _, dup := seen[value]; !dup {
			seen[value] = struct{}{}
			continue
		}
		if _, isDefault := value.(ir.SwitchValueDefault); isDefault {
			c.fe.addError(errMessage(span, "Multiple `default` cases are present"))
			continue
		}
		c.fe.addError(errSemantic(span, "Multiple `switch` cases for %s are present", switchValueDebug(value)))
	}
}

// switchValueDebug renders a switch value as Rust's Debug does, for the
// duplicate-case text: a case label is I32(n), U32(n) or Default
// (ir/statement.rs SwitchValue).
func switchValueDebug(value ir.SwitchValue) string {
	switch v := value.(type) {
	case ir.SwitchValueI32:
		return fmt.Sprintf("I32(%d)", int32(v))
	case ir.SwitchValueU32:
		return fmt.Sprintf("U32(%d)", uint32(v))
	case ir.SwitchValueDefault:
		return "Default"
	default:
		return fmt.Sprintf("SwitchValue(%T)", value)
	}
}

// lowerReturn lowers a return statement (parser/functions.rs:94-115). Rust
// converts no value here — implicit conversions are marked unported upstream —
// and restarts the emitter before the statement, so the returned value's Emit
// range is closed by it.
func (c *Context) lowerReturn(k stmtReturn, scope stmtScope) *frontError {
	var value *ir.ExpressionHandle
	if k.Value != nil {
		h, _, err := c.lowerExpect(&k.Value.Arena, k.Value.Root, rhsPos)
		if err != nil {
			return err
		}
		value = &h
	}
	c.emitRestart()
	c.body = append(c.body, ir.Statement{Kind: ir.StmtReturn{Value: value}})
	scope.terminator.record(len(c.body))
	return nil
}
