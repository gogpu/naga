package front

import "github.com/gogpu/naga/ir"

// This file parses statements, Rust naga's parser/functions.rs: the compound
// statement and its scope, declarations used as statements, expressions used as
// statements, and the control flow of if, switch, while, do and for.
//
// Rust lowers most of these while parsing — a loop becomes IR Loop with an If
// that breaks, a while condition is negated, a case body is culled after its
// terminator — and applies the GLSL fall-through rules to a switch as it goes.
// All of that is lowering here: the statements below keep the shape that was
// written, and the AST is walked in source order, which is the order Rust emits
// the IR in.

// exprSpan returns the span of the expression an arena evaluates.
func exprSpan(e *exprs) ir.Span { return e.Arena.get(e.Root).Span }

// parseStatement parses one statement, Rust naga's parse_statement
// (parser/functions.rs:41-583). Rust threads is_inside_loop into the
// declarations it parses (functions.rs:46) to decide a local initializer's
// placement; lowering derives that from the statement tree, so the parser does
// not carry the flag.
func (p *parser) parseStatement() (astStmt, *frontError) {
	if err := p.enterStmt(); err != nil {
		return astStmt{}, err
	}
	defer p.leaveStmt()

	// A type qualifier always identifies a declaration statement
	// (functions.rs:48-51).
	if p.peekTypeQualifier() {
		return p.parseDeclarationStatement()
	}
	// A type name identifies a declaration or a type constructor depending on
	// whether a parenthesis follows it (functions.rs:53-68).
	if p.peekTypeName() {
		declaration, err := p.typeNameStartsDeclaration()
		if err != nil {
			return astStmt{}, err
		}
		if declaration {
			return p.parseDeclarationStatement()
		}
	}
	return p.parseKeywordOrExprStatement()
}

// typeNameStartsDeclaration reports whether a statement that starts with a type
// name is a declaration or a constructor expression such as `vec4(1.0)`. It
// consumes the type name to see past it and hands it back, as Rust does
// (parser/functions.rs:55-64).
func (p *parser) typeNameStartsDeclaration() (bool, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return false, err
	}
	next, err := p.expectPeek()
	if err != nil {
		return false, err
	}
	declaration := !next.isPunct("(")
	if err := p.backtrack(tok); err != nil {
		return false, err
	}
	return declaration, nil
}

// parseDeclarationStatement parses a declaration used as a statement. A
// qualifier or a type name always starts a declaration, so its absence here is
// an inconsistency rather than input.
func (p *parser) parseDeclarationStatement() (astStmt, *frontError) {
	decl, err := p.parseDeclaration(false)
	if err != nil {
		return astStmt{}, err
	}
	if decl == nil {
		return astStmt{}, errInternal("A qualified or typed statement started no declaration", p.last)
	}
	return astStmt{Kind: stmtDecl{Decl: *decl}, Span: decl.Span}, nil
}

// parseKeywordOrExprStatement parses the statements that start with a keyword,
// a block or an expression, Rust naga's keyword match of parse_statement
// (parser/functions.rs:70-578).
func (p *parser) parseKeywordOrExprStatement() (astStmt, *frontError) {
	tok, err := p.expectPeek()
	if err != nil {
		return astStmt{}, err
	}
	switch tok.kind {
	case tokContinue:
		return p.parseJumpStatement(stmtContinue{})
	case tokBreak:
		return p.parseJumpStatement(stmtBreak{})
	case tokDiscard:
		return p.parseJumpStatement(stmtDiscard{})
	case tokReturn:
		return p.parseReturnStatement()
	case tokIf:
		return p.parseIfStatement()
	case tokSwitch:
		return p.parseSwitchStatement()
	case tokWhile:
		return p.parseWhileStatement()
	case tokDo:
		return p.parseDoStatement()
	case tokFor:
		return p.parseForStatement()
	}
	if tok.isPunct("{") {
		return p.parseBlockStatement()
	}
	if tok.isPunct(";") {
		semi, err := p.bump()
		if err != nil {
			return astStmt{}, err
		}
		return astStmt{Kind: stmtEmpty{}, Span: semi.span}, nil
	}
	return p.parseExprStatement()
}

// parseJumpStatement parses break, continue and discard: the keyword, which the
// caller has already seen, and its semicolon
// (parser/functions.rs:82-92, 116-122). Rust records each of them as a
// terminator of the enclosing body; whether statements after one are reachable
// is lowering's question.
func (p *parser) parseJumpStatement(kind astStmtKind) (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	end, err := p.expectPunct(";")
	if err != nil {
		return astStmt{}, err
	}
	return astStmt{Kind: kind, Span: subsumeSpan(keyword.span, end.span)}, nil
}

// parseReturnStatement parses a return with or without a value
// (parser/functions.rs:94-115).
func (p *parser) parseReturnStatement() (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	var value *exprs
	span := keyword.span
	if !p.atPunct(";") {
		value, err = p.parseExprInArena(p.parseExpression)
		if err != nil {
			return astStmt{}, err
		}
		span = subsumeSpan(span, exprSpan(value))
	}
	end, err := p.expectPunct(";")
	if err != nil {
		return astStmt{}, err
	}
	return astStmt{Kind: stmtReturn{Value: value}, Span: subsumeSpan(span, end.span)}, nil
}

// parseCondition parses a parenthesised condition, `( expression )`, which if,
// switch, while and do share (parser/functions.rs:126-135).
func (p *parser) parseCondition() (*exprs, *frontError) {
	if _, err := p.expectPunct("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExprInArena(p.parseExpression)
	if err != nil {
		return nil, err
	}
	if _, err := p.expectPunct(")"); err != nil {
		return nil, err
	}
	return cond, nil
}

// parseIfStatement parses an if statement with its optional else branch
// (parser/functions.rs:123-167). Rust keeps the two branches as separate bodies
// of an IR If; the AST keeps them as statements.
func (p *parser) parseIfStatement() (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	cond, err := p.parseCondition()
	if err != nil {
		return astStmt{}, err
	}
	accept, err := p.parseStatement()
	if err != nil {
		return astStmt{}, err
	}
	ifStmt := stmtIf{Cond: *cond, Accept: &accept}
	span := subsumeSpan(subsumeSpan(keyword.span, exprSpan(cond)), accept.Span)

	if p.bumpIf(kindIs(tokElse)) != nil {
		reject, err := p.parseStatement()
		if err != nil {
			return astStmt{}, err
		}
		ifStmt.Reject = &reject
		span = subsumeSpan(span, reject.Span)
	}
	return astStmt{Kind: ifStmt, Span: span}, nil
}

// parseSwitchStatement parses a switch statement: the selector and the labeled
// cases up to the closing brace (parser/functions.rs:168-333).
func (p *parser) parseSwitchStatement() (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	selector, err := p.parseCondition()
	if err != nil {
		return astStmt{}, err
	}
	if _, err := p.expectPunct("{"); err != nil {
		return astStmt{}, err
	}
	var cases []switchCase
	for {
		if closeTok := p.bumpIf(punct("}")); closeTok != nil {
			switchStmt := stmtSwitch{Selector: *selector, Cases: cases}
			return astStmt{Kind: switchStmt, Span: subsumeSpan(keyword.span, closeTok.span)}, nil
		}
		oneCase, err := p.parseSwitchCase()
		if err != nil {
			return astStmt{}, err
		}
		cases = append(cases, *oneCase)
	}
}

// parseSwitchCase parses one `case <constant expression> :` or `default :` label
// and the statements up to the next label or the closing brace, Rust naga's case
// arm of the switch (parser/functions.rs:192-292).
//
// The GLSL fall-through rules Rust applies as it parses — a trailing break
// clears fall_through, the last case must have statements and must not fall
// through, an absent default gets an empty one (functions.rs:296-328) — decide
// the shape of the IR SwitchCase and belong to lowering. The AST keeps every
// label and its statements as written.
func (p *parser) parseSwitchCase() (*switchCase, *frontError) {
	label, err := p.bump()
	if err != nil {
		return nil, err
	}
	oneCase := &switchCase{}
	switch label.kind {
	case tokCase:
		// A constant expression, which Rust parses with
		// parse_constant_expression (functions.rs:197-201); the value is
		// checked in lowering.
		oneCase.Label, err = p.parseExprInArena(p.parseConstantExpr)
		if err != nil {
			return nil, err
		}
	case tokDefault:
	default:
		return nil, invalidToken(label,
			wantKeyword(tokCase), wantKeyword(tokDefault), wantPunct("}"))
	}
	colon, err := p.expectPunct(":")
	if err != nil {
		return nil, err
	}
	oneCase.Span = subsumeSpan(label.span, colon.span)

	for !p.atKind(tokCase) && !p.atKind(tokDefault) && !p.atPunct("}") {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		oneCase.Body = append(oneCase.Body, stmt)
		oneCase.Span = subsumeSpan(oneCase.Span, stmt.Span)
	}
	return oneCase, nil
}

// parseWhileStatement parses a while statement (parser/functions.rs:334-381).
func (p *parser) parseWhileStatement() (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	cond, err := p.parseCondition()
	if err != nil {
		return astStmt{}, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return astStmt{}, err
	}
	whileStmt := stmtWhile{Cond: *cond, Body: body}
	return astStmt{
		Kind: whileStmt,
		Span: subsumeSpan(subsumeSpan(keyword.span, exprSpan(cond)), body.Span),
	}, nil
}

// parseDoStatement parses a do-while statement: the body first, then the
// condition and its semicolon (parser/functions.rs:382-434).
func (p *parser) parseDoStatement() (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return astStmt{}, err
	}
	if _, err := p.expectKeyword(tokWhile); err != nil {
		return astStmt{}, err
	}
	cond, err := p.parseCondition()
	if err != nil {
		return astStmt{}, err
	}
	end, err := p.expectPunct(";")
	if err != nil {
		return astStmt{}, err
	}
	doStmt := stmtDo{Body: body, Cond: *cond}
	return astStmt{Kind: doStmt, Span: subsumeSpan(subsumeSpan(keyword.span, body.Span), end.span)}, nil
}

// parseForStatement parses a for statement (parser/functions.rs:435-544).
func (p *parser) parseForStatement() (astStmt, *frontError) {
	keyword, err := p.bump()
	if err != nil {
		return astStmt{}, err
	}
	// The whole statement is one scope, so the names of the init clause reach
	// the condition, the update clause and the body
	// (parser/functions.rs:438, 541).
	p.symbols.pushScope()
	defer p.symbols.popScope()

	forStmt, err := p.parseForClauses()
	if err != nil {
		return astStmt{}, err
	}
	body, err := p.parseStatement()
	if err != nil {
		return astStmt{}, err
	}
	forStmt.Body = body
	return astStmt{Kind: *forStmt, Span: subsumeSpan(keyword.span, body.Span)}, nil
}

// parseForClauses parses `( init ; cond ; update )` of a for statement. Any of
// the three clauses may be empty.
//
// Rust also accepts a declaration as the condition clause
// (parser/functions.rs:453-485), which the AST of stmtFor cannot carry and GLSL
// does not allow: a declaration there reaches expression parsing and is a syntax
// error.
func (p *parser) parseForClauses() (*stmtFor, *frontError) {
	if _, err := p.expectPunct("("); err != nil {
		return nil, err
	}
	forStmt := &stmtFor{}
	if err := p.parseForInit(forStmt); err != nil {
		return nil, err
	}
	if err := p.parseForCondition(forStmt); err != nil {
		return nil, err
	}
	if err := p.parseForUpdate(forStmt); err != nil {
		return nil, err
	}
	if _, err := p.expectPunct(")"); err != nil {
		return nil, err
	}
	return forStmt, nil
}

// parseForInit parses the init clause: an empty clause, a declaration, or an
// expression and its semicolon (parser/functions.rs:441-450).
func (p *parser) parseForInit(forStmt *stmtFor) *frontError {
	if p.bumpIf(punct(";")) != nil {
		return nil
	}
	if !p.peekTypeName() && !p.peekTypeQualifier() {
		expr, err := p.parseExprInArena(p.parseExpression)
		if err != nil {
			return err
		}
		forStmt.InitExpr = expr
		_, err = p.expectPunct(";")
		return err
	}
	decl, err := p.parseDeclaration(false)
	if err != nil {
		return err
	}
	if decl == nil {
		return errInternal("A typed for-init clause started no declaration", p.last)
	}
	// A declaration consumes its own semicolon.
	forStmt.InitDecl = decl
	return nil
}

// parseForCondition parses the condition clause: an empty clause or an
// expression and its semicolon.
func (p *parser) parseForCondition(forStmt *stmtFor) *frontError {
	if p.bumpIf(punct(";")) != nil {
		return nil
	}
	cond, err := p.parseExprInArena(p.parseExpression)
	if err != nil {
		return err
	}
	forStmt.Cond = cond
	_, err = p.expectPunct(";")
	return err
}

// parseForUpdate parses the update clause, which ends at the closing
// parenthesis (parser/functions.rs:511-521).
func (p *parser) parseForUpdate(forStmt *stmtFor) *frontError {
	if p.atPunct(")") {
		return nil
	}
	update, err := p.parseExprInArena(p.parseExpression)
	if err != nil {
		return err
	}
	forStmt.Update = update
	return nil
}

// parseBlockStatement parses a compound statement as a statement of its own
// (parser/functions.rs:545-568).
func (p *parser) parseBlockStatement() (astStmt, *frontError) {
	open, err := p.expectPunct("{")
	if err != nil {
		return astStmt{}, err
	}
	body, err := p.parseCompoundStatement()
	if err != nil {
		return astStmt{}, err
	}
	// parseCompoundStatement returns right after consuming the closing brace, so
	// the last consumed token is the block's end.
	return astStmt{Kind: stmtBlock{Body: body}, Span: subsumeSpan(open.span, p.last)}, nil
}

// parseCompoundStatement parses the statements of a block whose opening brace
// the caller consumed, up to and including the closing brace, Rust naga's
// parse_compound_statement (parser/functions.rs:585-618). The block's scope is
// opened and closed here.
//
// Rust also culls the statements that follow a terminator (break, continue,
// return or discard) from the body (functions.rs:611-613). That is the
// reachability of the emitted IR, which lowering decides; the AST keeps every
// statement as written.
func (p *parser) parseCompoundStatement() ([]astStmt, *frontError) {
	p.symbols.pushScope()
	defer p.symbols.popScope()

	var body []astStmt
	for {
		if closeTok := p.bumpIf(punct("}")); closeTok != nil {
			return body, nil
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmt)
	}
}

// parseExprStatement parses an expression statement, which is what any statement
// that is not a declaration, a block or one of the keywords above becomes
// (parser/functions.rs:569-578).
func (p *parser) parseExprStatement() (astStmt, *frontError) {
	expr, err := p.parseExprInArena(p.parseExpression)
	if err != nil {
		return astStmt{}, err
	}
	end, err := p.expectPunct(";")
	if err != nil {
		return astStmt{}, err
	}
	return astStmt{Kind: stmtExpr{Expr: *expr}, Span: subsumeSpan(exprSpan(expr), end.span)}, nil
}
