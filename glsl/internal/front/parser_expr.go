package front

import "github.com/gogpu/naga/ir"

// This file parses expressions: primary, postfix, unary, binary, ternary,
// assignment and the comma operator, Rust naga's parser/expressions.rs. The
// grammar is GLSL's precedence climbing over the C operator table; every parsed
// expression is a HIR node in the statement's arena, and lowering turns them
// into IR later.

// parsePrimary parses a literal or a parenthesised expression, Rust naga's
// parse_primary (parser/expressions.rs:17-86).
func (p *parser) parsePrimary(e *exprs) (hirHandle, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return 0, err
	}
	switch tok.kind {
	case tokInt:
		if tok.intWidth != 32 {
			p.errors = append(p.errors, *errSemantic(tok.span, "Unsupported non-32bit integer"))
		}
		var value ir.LiteralValue = ir.LiteralU32(uint32(tok.intValue))
		if tok.intSigned {
			value = ir.LiteralI32(int32(tok.intValue))
		}
		return e.Arena.add(hirLiteral{Value: value}, tok.span), nil
	case tokFloat:
		if tok.floatWidth != 32 {
			p.errors = append(p.errors, *errSemantic(tok.span,
				"Unsupported floating-point value (expected single-precision floating-point number)"))
		}
		return e.Arena.add(hirLiteral{Value: ir.LiteralF32(tok.floatValue)}, tok.span), nil
	case tokBool:
		return e.Arena.add(hirLiteral{Value: ir.LiteralBool(tok.boolValue)}, tok.span), nil
	case tokPunct:
		if tok.text == "(" {
			expr, err := p.parseExpression(e)
			if err != nil {
				return 0, err
			}
			if _, err := p.expectPunct(")"); err != nil {
				return 0, err
			}
			return expr, nil
		}
	}
	return 0, invalidToken(tok, wantPunct("("), expected{Kind: expIntLiteral},
		expected{Kind: expFloatLiteral}, expected{Kind: expBoolLiteral})
}

// parseFunctionCallArgs parses a call's argument list after its opening
// parenthesis, Rust naga's parse_function_call_args
// (parser/expressions.rs:88-123). The span grows to cover the closing
// parenthesis.
func (p *parser) parseFunctionCallArgs(e *exprs, span *ir.Span) ([]hirHandle, *frontError) {
	var args []hirHandle
	if closeTok := p.bumpIf(punct(")")); closeTok != nil {
		*span = subsumeSpan(*span, closeTok.span)
		return nil, nil
	}
	for {
		arg, err := p.parseAssignment(e)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		tok, err := p.bump()
		if err != nil {
			return nil, err
		}
		switch {
		case tok.isPunct(","):
		case tok.isPunct(")"):
			*span = subsumeSpan(*span, tok.span)
			return args, nil
		default:
			return nil, invalidToken(tok, wantPunct(","), wantPunct(")"))
		}
	}
}

// parsePostfix parses a primary expression followed by any number of index,
// field, swizzle, method and postfix increment operators, Rust naga's
// parse_postfix (parser/expressions.rs:125-286). A type name followed by a
// parenthesis is a type constructor here, which is how vec4(...), float[3](...)
// and S(...) start; an identifier followed by one is a function call.
func (p *parser) parsePostfix(e *exprs) (hirHandle, *frontError) {
	var base hirHandle
	switch {
	case p.peekTypeName():
		// The written spelling is kept: a combined sampler constructor
		// (sampler2D(tex, samp)) is a call of the builtin of that name in the
		// IR, even though the parser meets it as a type (design §11).
		written := p.peek().text
		spec, span, err := p.parseConstructorType()
		if err != nil {
			return 0, err
		}
		if _, err := p.expectPunct("("); err != nil {
			return 0, err
		}
		args, err := p.parseFunctionCallArgs(e, &span)
		if err != nil {
			return 0, err
		}
		kind := calleeStruct
		if spec.Builtin != nil {
			kind = calleeTypeName
		}
		base = e.Arena.add(hirCall{Callee: callee{Kind: kind, Name: written, Typ: &spec}, Args: args}, span)
	case p.atIdent():
		nameTok, err := p.bump()
		if err != nil {
			return 0, err
		}
		name, span := nameTok.text, nameTok.span
		if p.bumpIf(punct("(")) != nil {
			args, err := p.parseFunctionCallArgs(e, &span)
			if err != nil {
				return 0, err
			}
			base = e.Arena.add(hirCall{Callee: callee{Kind: calleeFunction, Name: name}, Args: args}, span)
			break
		}
		sym := p.symbols.lookup(name)
		if sym == nil {
			sym = p.builtinSymbol(name)
		}
		if sym == nil {
			if refusal := subgroupRefusal(name, span); refusal != nil {
				return 0, refusal
			}
			return 0, errUnknownVariable(name, span)
		}
		base = e.Arena.add(hirVariable{Sym: sym}, span)
	default:
		var err *frontError
		base, err = p.parsePrimary(e)
		if err != nil {
			return 0, err
		}
	}

	for {
		tok, err := p.expectPeek()
		if err != nil {
			return 0, err
		}
		switch {
		case tok.isPunct("["):
			open, _ := p.bump()
			index, err := p.parseExpression(e)
			if err != nil {
				return 0, err
			}
			closeTok, err := p.expectPunct("]")
			if err != nil {
				return 0, err
			}
			span := subsumeSpan(open.span, closeTok.span)
			base = e.Arena.add(hirAccess{Base: base, Index: index}, span)
		case tok.isPunct("."):
			dot, _ := p.bump()
			field, err := p.expectIdent()
			if err != nil {
				return 0, err
			}
			span := subsumeSpan(dot.span, field.span)
			if p.bumpIf(punct("(")) != nil {
				args, err := p.parseFunctionCallArgs(e, &span)
				if err != nil {
					return 0, err
				}
				base = e.Arena.add(hirMethod{Expr: base, Name: field.text, Args: args}, span)
				continue
			}
			base = e.Arena.add(hirSelect{Base: base, Field: field.text}, span)
		case tok.isPunct("++") || tok.isPunct("--"):
			opTok, _ := p.bump()
			op := ir.BinaryAdd
			if opTok.isPunct("--") {
				op = ir.BinarySubtract
			}
			base = e.Arena.add(hirPrePostfix{Op: op, Postfix: true, Expr: base}, opTok.span)
		default:
			return base, nil
		}
	}
}

// parseUnary parses a unary expression, Rust naga's parse_unary
// (parser/expressions.rs:288-343). The depth of a run of unary operators and
// nested parentheses is bounded here, because this is the recursion every
// expression path passes through.
func (p *parser) parseUnary(e *exprs) (hirHandle, *frontError) {
	if err := p.enterExpr(); err != nil {
		return 0, err
	}
	defer p.leaveExpr()

	tok, err := p.expectPeek()
	if err != nil {
		return 0, err
	}
	switch {
	case tok.isPunct("+"), tok.isPunct("-"), tok.isPunct("!"), tok.isPunct("~"):
		opTok, err := p.bump()
		if err != nil {
			return 0, err
		}
		expr, err := p.parseUnary(e)
		if err != nil {
			return 0, err
		}
		span := subsumeSpan(opTok.span, e.Arena.get(expr).Span)
		switch {
		case opTok.isPunct("-"):
			return e.Arena.add(hirUnary{Op: ir.UnaryNegate, Expr: expr}, span), nil
		case opTok.isPunct("!"):
			return e.Arena.add(hirUnary{Op: ir.UnaryLogicalNot, Expr: expr}, span), nil
		case opTok.isPunct("~"):
			return e.Arena.add(hirUnary{Op: ir.UnaryBitwiseNot, Expr: expr}, span), nil
		default:
			// Unary plus is the identity, as in Rust: the operand is the
			// expression.
			return expr, nil
		}
	case tok.isPunct("++"), tok.isPunct("--"):
		opTok, err := p.bump()
		if err != nil {
			return 0, err
		}
		expr, err := p.parseUnary(e)
		if err != nil {
			return 0, err
		}
		op := ir.BinaryAdd
		if opTok.isPunct("--") {
			op = ir.BinarySubtract
		}
		return e.Arena.add(hirPrePostfix{Op: op, Postfix: false, Expr: expr}, opTok.span), nil
	default:
		return p.parsePostfix(e)
	}
}

// bindingPower returns the left and right binding powers of a binary operator,
// Rust naga's binding_power table (parser/expressions.rs:550-568), which is
// GLSL's precedence order.
func bindingPower(t glslToken) (lBP, rBP uint8, ok bool) {
	if t.kind != tokPunct {
		return 0, 0, false
	}
	switch t.text {
	case "||":
		return 1, 2, true
	case "^^":
		return 3, 4, true
	case "&&":
		return 5, 6, true
	case "|":
		return 7, 8, true
	case "^":
		return 9, 10, true
	case "&":
		return 11, 12, true
	case "==", "!=":
		return 13, 14, true
	case "<=", ">=", "<", ">":
		return 15, 16, true
	case "<<", ">>":
		return 17, 18, true
	case "+", "-":
		return 19, 20, true
	case "*", "/", "%":
		return 21, 22, true
	default:
		return 0, 0, false
	}
}

// binaryOp maps a binary operator token to its IR operator.
func binaryOp(t glslToken) (ir.BinaryOperator, bool) {
	switch t.text {
	case "||":
		return ir.BinaryLogicalOr, true
	case "^^":
		// GLSL's logical exclusive or is an inequality of booleans, as in Rust.
		return ir.BinaryNotEqual, true
	case "&&":
		return ir.BinaryLogicalAnd, true
	case "|":
		return ir.BinaryInclusiveOr, true
	case "^":
		return ir.BinaryExclusiveOr, true
	case "&":
		return ir.BinaryAnd, true
	case "==":
		return ir.BinaryEqual, true
	case "!=":
		return ir.BinaryNotEqual, true
	case ">=":
		return ir.BinaryGreaterEqual, true
	case "<=":
		return ir.BinaryLessEqual, true
	case "<":
		return ir.BinaryLess, true
	case ">":
		return ir.BinaryGreater, true
	case "<<":
		return ir.BinaryShiftLeft, true
	case ">>":
		return ir.BinaryShiftRight, true
	case "+":
		return ir.BinaryAdd, true
	case "-":
		return ir.BinarySubtract, true
	case "*":
		return ir.BinaryMultiply, true
	case "/":
		return ir.BinaryDivide, true
	case "%":
		return ir.BinaryModulo, true
	default:
		return 0, false
	}
}

// compoundAssignOp maps a compound assignment operator to the binary operator
// it expands to, Rust naga's list in parse_assignment
// (parser/expressions.rs:463-472).
func compoundAssignOp(t glslToken) (ir.BinaryOperator, bool) {
	switch t.text {
	case "|=":
		return ir.BinaryInclusiveOr, true
	case "&=":
		return ir.BinaryAnd, true
	case "+=":
		return ir.BinaryAdd, true
	case "/=":
		return ir.BinaryDivide, true
	case "%=":
		return ir.BinaryModulo, true
	case "-=":
		return ir.BinarySubtract, true
	case "*=":
		return ir.BinaryMultiply, true
	case "<<=":
		return ir.BinaryShiftLeft, true
	case ">>=":
		return ir.BinaryShiftRight, true
	case "^=":
		return ir.BinaryExclusiveOr, true
	default:
		return 0, false
	}
}

// parseBinary parses a binary expression with precedence climbing, Rust naga's
// parse_binary (parser/expressions.rs:345-404). passthrough lets a caller hand
// in the left operand it already parsed.
func (p *parser) parseBinary(e *exprs, passthrough *hirHandle, minBP uint8) (hirHandle, *frontError) {
	left := hirHandle(0)
	if passthrough != nil {
		left = *passthrough
	} else {
		var err *frontError
		left, err = p.parseUnary(e)
		if err != nil {
			return 0, err
		}
	}
	span := e.Arena.get(left).Span

	for {
		tok, err := p.expectPeek()
		if err != nil {
			return 0, err
		}
		lBP, rBP, ok := bindingPower(*tok)
		if !ok || lBP < minBP {
			return left, nil
		}
		opTok, err := p.bump()
		if err != nil {
			return 0, err
		}
		op, _ := binaryOp(opTok)
		right, err := p.parseBinary(e, nil, rBP)
		if err != nil {
			return 0, err
		}
		span = subsumeSpan(span, e.Arena.get(right).Span)
		left = e.Arena.add(hirBinary{Left: left, Op: op, Right: right}, span)
	}
}

// parseConstantExpr parses a constant expression: a conditional expression,
// which is the production Rust naga's parse_constant_expression uses
// (parser.rs:231-244). The value must be a constant, but that is lowering's
// check; the parser only fixes the syntax.
func (p *parser) parseConstantExpr(e *exprs) (hirHandle, *frontError) {
	return p.parseConditional(e, nil)
}

// parseConditional parses the ternary operator, Rust naga's parse_conditional
// (parser/expressions.rs:406-437). The accept branch may contain a comma
// operator, the reject branch may not, as in GLSL.
func (p *parser) parseConditional(e *exprs, passthrough *hirHandle) (hirHandle, *frontError) {
	cond, err := p.parseBinary(e, passthrough, 0)
	if err != nil {
		return 0, err
	}
	if p.bumpIf(punct("?")) == nil {
		return cond, nil
	}
	accept, err := p.parseExpression(e)
	if err != nil {
		return 0, err
	}
	if _, err := p.expectPunct(":"); err != nil {
		return 0, err
	}
	reject, err := p.parseAssignment(e)
	if err != nil {
		return 0, err
	}
	span := subsumeSpan(e.Arena.get(cond).Span, e.Arena.get(reject).Span)
	return e.Arena.add(hirConditional{Condition: cond, Accept: accept, Reject: reject}, span), nil
}

// parseAssignment parses an assignment, expanding a compound assignment into
// the binary operation it stands for, Rust naga's parse_assignment
// (parser/expressions.rs:439-512). Assignment is right-associative.
func (p *parser) parseAssignment(e *exprs) (hirHandle, *frontError) {
	tgt, err := p.parseUnary(e)
	if err != nil {
		return 0, err
	}
	tok, err := p.expectPeek()
	if err != nil {
		return 0, err
	}
	if tok.isPunct("=") {
		if _, err := p.bump(); err != nil {
			return 0, err
		}
		value, err := p.parseAssignment(e)
		if err != nil {
			return 0, err
		}
		span := subsumeSpan(e.Arena.get(tgt).Span, e.Arena.get(value).Span)
		return e.Arena.add(hirAssign{Tgt: tgt, Value: value}, span), nil
	}
	if op, ok := compoundAssignOp(*tok); ok {
		if _, err := p.bump(); err != nil {
			return 0, err
		}
		right, err := p.parseAssignment(e)
		if err != nil {
			return 0, err
		}
		span := subsumeSpan(e.Arena.get(tgt).Span, e.Arena.get(right).Span)
		value := e.Arena.add(hirBinary{Left: tgt, Op: op, Right: right}, span)
		return e.Arena.add(hirAssign{Tgt: tgt, Value: value}, span), nil
	}
	return p.parseConditional(e, &tgt)
}

// parseExpression parses a full expression, the comma operator included, Rust
// naga's parse_expression (parser/expressions.rs:514-547).
func (p *parser) parseExpression(e *exprs) (hirHandle, *frontError) {
	first, err := p.parseAssignment(e)
	if err != nil {
		return 0, err
	}
	handles := []hirHandle{first}
	for p.bumpIf(punct(",")) != nil {
		next, err := p.parseAssignment(e)
		if err != nil {
			return 0, err
		}
		handles = append(handles, next)
	}
	if len(handles) == 1 {
		return first, nil
	}
	span := e.Arena.get(handles[0]).Span
	for _, h := range handles[1:] {
		span = subsumeSpan(span, e.Arena.get(h).Span)
	}
	return e.Arena.add(hirSequence{Exprs: handles}, span), nil
}

// atIdent reports whether the next token is an identifier.
func (p *parser) atIdent() bool {
	tok := p.peek()
	return tok != nil && tok.kind == tokIdent
}
