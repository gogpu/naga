package wgsl

import (
	"fmt"
)

// Parser parses WGSL tokens into an AST.
type Parser struct {
	tokens  []Token
	current int
	errors  []ParseError
}

// ParseError represents a parsing error.
type ParseError struct {
	Message string
	Token   Token
}

func (e ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Token.Line, e.Token.Column, e.Message)
}

// NewParser creates a new parser for the given tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{
		tokens:  tokens,
		current: 0,
	}
}

// Parse parses the tokens and returns a Module AST.
func (p *Parser) Parse() (*Module, error) {
	module := &Module{}

	for !p.isAtEnd() {
		decl, err := p.declaration()
		if err != nil {
			p.errors = append(p.errors, *err)
			p.synchronize()
			continue
		}
		if decl != nil {
			switch d := decl.(type) {
			case *FunctionDecl:
				module.Functions = append(module.Functions, d)
			case *StructDecl:
				module.Structs = append(module.Structs, d)
			case *VarDecl:
				module.GlobalVars = append(module.GlobalVars, d)
			case *ConstDecl:
				module.Constants = append(module.Constants, d)
			case *AliasDecl:
				module.Aliases = append(module.Aliases, d)
			}
		}
	}

	if len(p.errors) > 0 {
		return module, fmt.Errorf("parsing failed with %d error(s): %w", len(p.errors), p.errors[0])
	}

	return module, nil
}

// declaration parses a top-level declaration.
func (p *Parser) declaration() (Decl, *ParseError) {
	// Parse attributes first
	attrs := p.attributes()

	switch {
	case p.check(TokenFn):
		return p.functionDecl(attrs)
	case p.check(TokenStruct):
		return p.structDecl(attrs)
	case p.check(TokenVar):
		return p.varDecl(attrs)
	case p.check(TokenConst):
		return p.constDecl()
	case p.check(TokenLet):
		return p.letDecl()
	case p.check(TokenAlias):
		return p.aliasDecl()
	case p.check(TokenEnable):
		// Skip enable directives for now
		p.advance()
		for !p.check(TokenSemicolon) && !p.isAtEnd() {
			p.advance()
		}
		if p.check(TokenSemicolon) {
			p.advance()
		}
		return nil, nil
	case p.check(TokenEOF):
		return nil, nil
	default:
		tok := p.peek()
		return nil, &ParseError{
			Message: fmt.Sprintf("unexpected token %s, expected declaration", tok.Kind),
			Token:   tok,
		}
	}
}

// attributes parses a list of attributes (@location(0), @vertex, etc.)
func (p *Parser) attributes() []Attribute {
	var attrs []Attribute

	for p.check(TokenAt) {
		start := p.peek()
		p.advance() // consume @

		if !p.check(TokenIdent) {
			continue
		}

		name := p.advance()
		attr := Attribute{
			Name: name.Lexeme,
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
		}

		// Check for arguments
		if p.match(TokenLeftParen) {
			for !p.check(TokenRightParen) && !p.isAtEnd() {
				arg, err := p.expression()
				if err != nil {
					break
				}
				attr.Args = append(attr.Args, arg)

				if !p.match(TokenComma) {
					break
				}
			}
			p.expect(TokenRightParen)
		}

		attrs = append(attrs, attr)
	}

	return attrs
}

// functionDecl parses a function declaration.
func (p *Parser) functionDecl(attrs []Attribute) (*FunctionDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenFn) {
		return nil, &ParseError{Message: "expected 'fn'", Token: p.peek()}
	}

	// Function name
	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected function name", Token: p.peek()}
	}
	name := p.advance()

	// Parameters
	if err := p.expectErr(TokenLeftParen); err != nil {
		return nil, err
	}

	params := make([]*Parameter, 0, 4) // most functions have few params
	for !p.check(TokenRightParen) && !p.isAtEnd() {
		param, err := p.parameter()
		if err != nil {
			return nil, err
		}
		params = append(params, param)

		if !p.match(TokenComma) {
			break
		}
	}

	if err := p.expectErr(TokenRightParen); err != nil {
		return nil, err
	}

	// Return type (optional)
	var returnType Type
	var returnAttrs []Attribute
	if p.match(TokenArrow) {
		returnAttrs = p.attributes()
		rt, err := p.typeSpec()
		if err != nil {
			return nil, err
		}
		returnType = rt
	}

	// Function body
	body, err := p.block()
	if err != nil {
		return nil, err
	}

	return &FunctionDecl{
		Name:        name.Lexeme,
		Params:      params,
		ReturnType:  returnType,
		ReturnAttrs: returnAttrs,
		Attributes:  attrs,
		Body:        body,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// parameter parses a function parameter.
func (p *Parser) parameter() (*Parameter, *ParseError) {
	attrs := p.attributes()

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected parameter name", Token: p.peek()}
	}
	name := p.advance()

	if err := p.expectErr(TokenColon); err != nil {
		return nil, err
	}

	paramType, err := p.typeSpec()
	if err != nil {
		return nil, err
	}

	return &Parameter{
		Name:       name.Lexeme,
		Type:       paramType,
		Attributes: attrs,
		Span: Span{
			Start: Position{Line: name.Line, Column: name.Column},
		},
	}, nil
}

// structDecl parses a struct declaration.
func (p *Parser) structDecl(attrs []Attribute) (*StructDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenStruct) {
		return nil, &ParseError{Message: "expected 'struct'", Token: p.peek()}
	}

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected struct name", Token: p.peek()}
	}
	name := p.advance()

	if err := p.expectErr(TokenLeftBrace); err != nil {
		return nil, err
	}

	members := make([]*StructMember, 0, 4) // most structs have a few members
	for !p.check(TokenRightBrace) && !p.isAtEnd() {
		member, err := p.structMember()
		if err != nil {
			return nil, err
		}
		members = append(members, member)

		// Optional comma between members
		p.match(TokenComma)
	}

	if err := p.expectErr(TokenRightBrace); err != nil {
		return nil, err
	}

	return &StructDecl{
		Name:    name.Lexeme,
		Members: members,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// structMember parses a struct member.
func (p *Parser) structMember() (*StructMember, *ParseError) {
	attrs := p.attributes()

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected member name", Token: p.peek()}
	}
	name := p.advance()

	if err := p.expectErr(TokenColon); err != nil {
		return nil, err
	}

	memberType, err := p.typeSpec()
	if err != nil {
		return nil, err
	}

	return &StructMember{
		Name:       name.Lexeme,
		Type:       memberType,
		Attributes: attrs,
		Span: Span{
			Start: Position{Line: name.Line, Column: name.Column},
		},
	}, nil
}

// varDecl parses a variable declaration.
func (p *Parser) varDecl(attrs []Attribute) (*VarDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenVar) {
		return nil, &ParseError{Message: "expected 'var'", Token: p.peek()}
	}

	// Optional address space and access mode: var<storage, read_write>
	var addressSpace string
	var accessMode string
	if p.match(TokenLess) {
		if p.check(TokenIdent) {
			addressSpace = p.advance().Lexeme
		}
		// Optional access mode: var<storage, read_write>
		if p.match(TokenComma) {
			if p.check(TokenIdent) {
				accessMode = p.advance().Lexeme
			}
		}
		p.expect(TokenGreater)
	}

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected variable name", Token: p.peek()}
	}
	name := p.advance()

	// Optional type annotation
	var varType Type
	if p.match(TokenColon) {
		t, err := p.typeSpec()
		if err != nil {
			return nil, err
		}
		varType = t
	}

	// Optional initializer
	var init Expr
	if p.match(TokenEqual) {
		e, err := p.expression()
		if err != nil {
			return nil, err
		}
		init = e
	}

	p.match(TokenSemicolon)

	return &VarDecl{
		Name:         name.Lexeme,
		Type:         varType,
		Init:         init,
		AddressSpace: addressSpace,
		AccessMode:   accessMode,
		Attributes:   attrs,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// constDecl parses a const declaration.
func (p *Parser) constDecl() (*ConstDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenConst) {
		return nil, &ParseError{Message: "expected 'const'", Token: p.peek()}
	}

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected constant name", Token: p.peek()}
	}
	name := p.advance()

	// Optional type annotation
	var constType Type
	if p.match(TokenColon) {
		t, err := p.typeSpec()
		if err != nil {
			return nil, err
		}
		constType = t
	}

	if err := p.expectErr(TokenEqual); err != nil {
		return nil, err
	}

	init, err := p.expression()
	if err != nil {
		return nil, err
	}

	p.match(TokenSemicolon)

	return &ConstDecl{
		Name: name.Lexeme,
		Type: constType,
		Init: init,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// letDecl parses a let declaration (treated as const for simplicity).
func (p *Parser) letDecl() (*ConstDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenLet) {
		return nil, &ParseError{Message: "expected 'let'", Token: p.peek()}
	}

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected variable name", Token: p.peek()}
	}
	name := p.advance()

	// Optional type annotation
	var letType Type
	if p.match(TokenColon) {
		t, err := p.typeSpec()
		if err != nil {
			return nil, err
		}
		letType = t
	}

	if err := p.expectErr(TokenEqual); err != nil {
		return nil, err
	}

	init, err := p.expression()
	if err != nil {
		return nil, err
	}

	p.match(TokenSemicolon)

	return &ConstDecl{
		Name: name.Lexeme,
		Type: letType,
		Init: init,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// aliasDecl parses a type alias declaration.
func (p *Parser) aliasDecl() (*AliasDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenAlias) {
		return nil, &ParseError{Message: "expected 'alias'", Token: p.peek()}
	}

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected alias name", Token: p.peek()}
	}
	name := p.advance()

	if err := p.expectErr(TokenEqual); err != nil {
		return nil, err
	}

	aliasType, err := p.typeSpec()
	if err != nil {
		return nil, err
	}

	p.match(TokenSemicolon)

	return &AliasDecl{
		Name: name.Lexeme,
		Type: aliasType,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// typeSpec parses a type specification.
func (p *Parser) typeSpec() (Type, *ParseError) {
	tok := p.peek()

	// Array type: array<f32, 4> - must check before general type keywords
	if p.check(TokenArray) {
		p.advance() // consume 'array'
		if err := p.expectErr(TokenLess); err != nil {
			return nil, err
		}

		elemType, err := p.typeSpec()
		if err != nil {
			return nil, err
		}

		var size Expr
		if p.match(TokenComma) {
			// Parse only primary expression for size to avoid > being interpreted as comparison
			size, err = p.primary()
			if err != nil {
				return nil, err
			}
		}

		if err := p.expectErr(TokenGreater); err != nil {
			return nil, err
		}

		return &ArrayType{
			Element: elemType,
			Size:    size,
			Span: Span{
				Start: Position{Line: tok.Line, Column: tok.Column},
			},
		}, nil
	}

	// Pointer type: ptr<function, f32>
	if p.match(TokenPtr) {
		if err := p.expectErr(TokenLess); err != nil {
			return nil, err
		}

		if !p.check(TokenIdent) {
			return nil, &ParseError{Message: "expected address space", Token: p.peek()}
		}
		addressSpace := p.advance().Lexeme

		if err := p.expectErr(TokenComma); err != nil {
			return nil, err
		}

		pointeeType, err := p.typeSpec()
		if err != nil {
			return nil, err
		}

		var accessMode string
		if p.match(TokenComma) {
			if p.check(TokenIdent) {
				accessMode = p.advance().Lexeme
			}
		}

		if err := p.expectErr(TokenGreater); err != nil {
			return nil, err
		}

		return &PtrType{
			AddressSpace: addressSpace,
			PointeeType:  pointeeType,
			AccessMode:   accessMode,
			Span: Span{
				Start: Position{Line: tok.Line, Column: tok.Column},
			},
		}, nil
	}

	// Check for type keywords or identifiers (named types)
	if p.isTypeKeyword(tok.Kind) || p.check(TokenIdent) {
		name := p.advance()
		namedType := &NamedType{
			Name: name.Lexeme,
			Span: Span{
				Start: Position{Line: name.Line, Column: name.Column},
			},
		}

		// Check for generic parameters: vec3<f32>
		if p.match(TokenLess) {
			for !p.check(TokenGreater) && !p.isAtEnd() {
				paramType, err := p.typeSpec()
				if err != nil {
					return nil, err
				}
				namedType.TypeParams = append(namedType.TypeParams, paramType)

				if !p.match(TokenComma) {
					break
				}
			}
			p.expect(TokenGreater)
		}

		return namedType, nil
	}

	return nil, &ParseError{Message: "expected type", Token: tok}
}

// block parses a block statement.
func (p *Parser) block() (*BlockStmt, *ParseError) {
	start := p.peek()
	if err := p.expectErr(TokenLeftBrace); err != nil {
		return nil, err
	}

	stmts := make([]Stmt, 0, 4) // most blocks have a few statements
	for !p.check(TokenRightBrace) && !p.isAtEnd() {
		stmt, err := p.statement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
	}

	if err := p.expectErr(TokenRightBrace); err != nil {
		return nil, err
	}

	return &BlockStmt{
		Statements: stmts,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// statement parses a statement.
func (p *Parser) statement() (Stmt, *ParseError) {
	switch {
	case p.check(TokenReturn):
		return p.returnStmt()
	case p.check(TokenIf):
		return p.ifStmt()
	case p.check(TokenFor):
		return p.forStmt()
	case p.check(TokenWhile):
		return p.whileStmt()
	case p.check(TokenLoop):
		return p.loopStmt()
	case p.check(TokenBreak):
		return p.breakStmt()
	case p.check(TokenContinue):
		return p.continueStmt()
	case p.check(TokenDiscard):
		return p.discardStmt()
	case p.check(TokenSwitch):
		return p.switchStmt()
	case p.check(TokenVar):
		return p.varDecl(nil)
	case p.check(TokenLet):
		return p.letStmt()
	case p.check(TokenConst):
		return p.localConstStmt()
	case p.check(TokenLeftBrace):
		return p.block()
	default:
		return p.exprOrAssignStmt()
	}
}

// returnStmt parses a return statement.
func (p *Parser) returnStmt() (*ReturnStmt, *ParseError) {
	start := p.advance() // consume 'return'

	var value Expr
	if !p.check(TokenSemicolon) && !p.check(TokenRightBrace) {
		e, err := p.expression()
		if err != nil {
			return nil, err
		}
		value = e
	}

	p.match(TokenSemicolon)

	return &ReturnStmt{
		Value: value,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// ifStmt parses an if statement.
func (p *Parser) ifStmt() (*IfStmt, *ParseError) {
	start := p.advance() // consume 'if'

	cond, err := p.expression()
	if err != nil {
		return nil, err
	}

	body, err := p.block()
	if err != nil {
		return nil, err
	}

	var elseStmt Stmt
	if p.match(TokenElse) {
		if p.check(TokenIf) {
			elseStmt, err = p.ifStmt()
		} else {
			elseStmt, err = p.block()
		}
		if err != nil {
			return nil, err
		}
	}

	return &IfStmt{
		Condition: cond,
		Body:      body,
		Else:      elseStmt,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// forStmt parses a for statement.
func (p *Parser) forStmt() (*ForStmt, *ParseError) {
	start := p.advance() // consume 'for'

	if err := p.expectErr(TokenLeftParen); err != nil {
		return nil, err
	}

	// Init
	var init Stmt
	if !p.check(TokenSemicolon) {
		s, err := p.statement()
		if err != nil {
			return nil, err
		}
		init = s
	} else {
		p.advance()
	}

	// Condition
	var cond Expr
	if !p.check(TokenSemicolon) {
		e, err := p.expression()
		if err != nil {
			return nil, err
		}
		cond = e
	}
	p.match(TokenSemicolon)

	// Update
	var update Stmt
	if !p.check(TokenRightParen) {
		s, err := p.statement()
		if err != nil {
			return nil, err
		}
		update = s
	}

	if err := p.expectErr(TokenRightParen); err != nil {
		return nil, err
	}

	body, err := p.block()
	if err != nil {
		return nil, err
	}

	return &ForStmt{
		Init:      init,
		Condition: cond,
		Update:    update,
		Body:      body,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// whileStmt parses a while statement.
func (p *Parser) whileStmt() (*WhileStmt, *ParseError) {
	start := p.advance() // consume 'while'

	cond, err := p.expression()
	if err != nil {
		return nil, err
	}

	body, err := p.block()
	if err != nil {
		return nil, err
	}

	return &WhileStmt{
		Condition: cond,
		Body:      body,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// loopStmt parses a loop statement.
func (p *Parser) loopStmt() (*LoopStmt, *ParseError) {
	start := p.advance() // consume 'loop'

	body, err := p.block()
	if err != nil {
		return nil, err
	}

	return &LoopStmt{
		Body: body,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// switchStmt parses a switch statement.
func (p *Parser) switchStmt() (*SwitchStmt, *ParseError) {
	start := p.advance() // consume 'switch'

	// Parse selector expression
	selector, err := p.expression()
	if err != nil {
		return nil, err
	}

	if err := p.expectErr(TokenLeftBrace); err != nil {
		return nil, err
	}

	var cases []*SwitchCaseClause
	for !p.check(TokenRightBrace) && !p.isAtEnd() {
		caseClause, err := p.switchCaseClause()
		if err != nil {
			return nil, err
		}
		cases = append(cases, caseClause)
	}

	if err := p.expectErr(TokenRightBrace); err != nil {
		return nil, err
	}

	return &SwitchStmt{
		Selector: selector,
		Cases:    cases,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// switchCaseClause parses a case or default clause in a switch statement.
func (p *Parser) switchCaseClause() (*SwitchCaseClause, *ParseError) {
	start := p.peek()
	var selectors []Expr
	isDefault := false

	if p.match(TokenDefault) {
		isDefault = true
	} else if p.match(TokenCase) {
		// Parse comma-separated selectors: case 1u, 2u, 3u:
		for {
			expr, err := p.expression()
			if err != nil {
				return nil, err
			}
			selectors = append(selectors, expr)
			if !p.match(TokenComma) {
				break
			}
		}
	} else {
		return nil, &ParseError{Message: "expected 'case' or 'default'", Token: start}
	}

	if err := p.expectErr(TokenColon); err != nil {
		return nil, err
	}

	body, err := p.block()
	if err != nil {
		return nil, err
	}

	return &SwitchCaseClause{
		Selectors: selectors,
		IsDefault: isDefault,
		Body:      body,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// localConstStmt parses a local const declaration inside a function.
func (p *Parser) localConstStmt() (*ConstDecl, *ParseError) {
	return p.constDecl()
}

// breakStmt parses a break statement.
func (p *Parser) breakStmt() (*BreakStmt, *ParseError) {
	start := p.advance() // consume 'break'
	p.match(TokenSemicolon)
	return &BreakStmt{
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// continueStmt parses a continue statement.
func (p *Parser) continueStmt() (*ContinueStmt, *ParseError) {
	start := p.advance() // consume 'continue'
	p.match(TokenSemicolon)
	return &ContinueStmt{
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// discardStmt parses a discard statement.
func (p *Parser) discardStmt() (*DiscardStmt, *ParseError) {
	start := p.advance() // consume 'discard'
	p.match(TokenSemicolon)
	return &DiscardStmt{
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// letStmt parses a let statement (local variable).
func (p *Parser) letStmt() (*VarDecl, *ParseError) {
	start := p.peek()
	p.advance() // consume 'let'

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected variable name", Token: p.peek()}
	}
	name := p.advance()

	var varType Type
	if p.match(TokenColon) {
		t, err := p.typeSpec()
		if err != nil {
			return nil, err
		}
		varType = t
	}

	if err := p.expectErr(TokenEqual); err != nil {
		return nil, err
	}

	init, err := p.expression()
	if err != nil {
		return nil, err
	}

	p.match(TokenSemicolon)

	return &VarDecl{
		Name: name.Lexeme,
		Type: varType,
		Init: init,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// exprOrAssignStmt parses an expression statement or assignment.
func (p *Parser) exprOrAssignStmt() (Stmt, *ParseError) {
	start := p.peek()
	expr, err := p.expression()
	if err != nil {
		return nil, err
	}

	// Check for assignment operators
	if p.isAssignOp(p.peek().Kind) {
		op := p.advance()
		right, err := p.expression()
		if err != nil {
			return nil, err
		}
		p.match(TokenSemicolon)
		return &AssignStmt{
			Left:  expr,
			Op:    op.Kind,
			Right: right,
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
		}, nil
	}

	p.match(TokenSemicolon)
	return &ExprStmt{
		Expr: expr,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// expression parses an expression.
func (p *Parser) expression() (Expr, *ParseError) {
	return p.logicalOr()
}

// logicalOr parses || expressions.
func (p *Parser) logicalOr() (Expr, *ParseError) {
	left, err := p.logicalAnd()
	if err != nil {
		return nil, err
	}

	for p.match(TokenPipePipe) {
		right, err := p.logicalAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    TokenPipePipe,
			Right: right,
		}
	}

	return left, nil
}

// logicalAnd parses && expressions.
func (p *Parser) logicalAnd() (Expr, *ParseError) {
	left, err := p.bitwiseOr()
	if err != nil {
		return nil, err
	}

	for p.match(TokenAmpAmp) {
		right, err := p.bitwiseOr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    TokenAmpAmp,
			Right: right,
		}
	}

	return left, nil
}

// bitwiseOr parses | expressions.
func (p *Parser) bitwiseOr() (Expr, *ParseError) {
	left, err := p.bitwiseXor()
	if err != nil {
		return nil, err
	}

	for p.match(TokenPipe) {
		right, err := p.bitwiseXor()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    TokenPipe,
			Right: right,
		}
	}

	return left, nil
}

// bitwiseXor parses ^ expressions.
func (p *Parser) bitwiseXor() (Expr, *ParseError) {
	left, err := p.bitwiseAnd()
	if err != nil {
		return nil, err
	}

	for p.match(TokenCaret) {
		right, err := p.bitwiseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    TokenCaret,
			Right: right,
		}
	}

	return left, nil
}

// bitwiseAnd parses & expressions.
func (p *Parser) bitwiseAnd() (Expr, *ParseError) {
	left, err := p.equality()
	if err != nil {
		return nil, err
	}

	for p.match(TokenAmpersand) {
		right, err := p.equality()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    TokenAmpersand,
			Right: right,
		}
	}

	return left, nil
}

// equality parses == and != expressions.
func (p *Parser) equality() (Expr, *ParseError) {
	left, err := p.comparison()
	if err != nil {
		return nil, err
	}

	for p.check(TokenEqualEqual) || p.check(TokenBangEqual) {
		op := p.advance()
		right, err := p.comparison()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op.Kind,
			Right: right,
		}
	}

	return left, nil
}

// comparison parses <, >, <=, >= expressions.
func (p *Parser) comparison() (Expr, *ParseError) {
	left, err := p.shift()
	if err != nil {
		return nil, err
	}

	for p.check(TokenLess) || p.check(TokenGreater) ||
		p.check(TokenLessEqual) || p.check(TokenGreaterEqual) {
		op := p.advance()
		right, err := p.shift()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op.Kind,
			Right: right,
		}
	}

	return left, nil
}

// shift parses << and >> expressions.
func (p *Parser) shift() (Expr, *ParseError) {
	left, err := p.additive()
	if err != nil {
		return nil, err
	}

	for p.check(TokenLessLess) || p.check(TokenGreaterGreater) {
		op := p.advance()
		right, err := p.additive()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op.Kind,
			Right: right,
		}
	}

	return left, nil
}

// additive parses + and - expressions.
func (p *Parser) additive() (Expr, *ParseError) {
	left, err := p.multiplicative()
	if err != nil {
		return nil, err
	}

	for p.check(TokenPlus) || p.check(TokenMinus) {
		op := p.advance()
		right, err := p.multiplicative()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op.Kind,
			Right: right,
		}
	}

	return left, nil
}

// multiplicative parses *, /, % expressions.
func (p *Parser) multiplicative() (Expr, *ParseError) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}

	for p.check(TokenStar) || p.check(TokenSlash) || p.check(TokenPercent) {
		op := p.advance()
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op.Kind,
			Right: right,
		}
	}

	return left, nil
}

// unary parses unary expressions.
func (p *Parser) unary() (Expr, *ParseError) {
	if p.check(TokenMinus) || p.check(TokenBang) || p.check(TokenTilde) ||
		p.check(TokenAmpersand) || p.check(TokenStar) {
		op := p.advance()
		operand, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{
			Op:      op.Kind,
			Operand: operand,
			Span: Span{
				Start: Position{Line: op.Line, Column: op.Column},
			},
		}, nil
	}

	return p.postfix()
}

// postfix parses postfix expressions (calls, indexing, member access).
func (p *Parser) postfix() (Expr, *ParseError) {
	expr, err := p.primary()
	if err != nil {
		return nil, err
	}

	for {
		if p.match(TokenLeftParen) {
			// Function call
			args := make([]Expr, 0, 4)
			for !p.check(TokenRightParen) && !p.isAtEnd() {
				arg, err := p.expression()
				if err != nil {
					return nil, err
				}
				args = append(args, arg)
				if !p.match(TokenComma) {
					break
				}
			}
			p.expect(TokenRightParen)

			if ident, ok := expr.(*Ident); ok {
				expr = &CallExpr{
					Func: ident,
					Args: args,
				}
			} else {
				// Type constructor
				if namedType, ok := expr.(*ConstructExpr); ok {
					namedType.Args = args
				}
			}
		} else if p.match(TokenLeftBracket) {
			// Index expression
			index, err := p.expression()
			if err != nil {
				return nil, err
			}
			p.expect(TokenRightBracket)
			expr = &IndexExpr{
				Expr:  expr,
				Index: index,
			}
		} else if p.match(TokenDot) {
			// Member access
			if !p.check(TokenIdent) {
				return nil, &ParseError{Message: "expected member name", Token: p.peek()}
			}
			member := p.advance()
			expr = &MemberExpr{
				Expr:   expr,
				Member: member.Lexeme,
			}
		} else {
			break
		}
	}

	return expr, nil
}

// primary parses primary expressions.
func (p *Parser) primary() (Expr, *ParseError) {
	tok := p.peek()

	switch tok.Kind {
	case TokenIntLiteral, TokenFloatLiteral:
		p.advance()
		return &Literal{
			Kind:  tok.Kind,
			Value: tok.Lexeme,
			Span: Span{
				Start: Position{Line: tok.Line, Column: tok.Column},
			},
		}, nil

	case TokenTrue, TokenFalse, TokenBoolLiteral:
		p.advance()
		return &Literal{
			Kind:  TokenBoolLiteral,
			Value: tok.Lexeme,
			Span: Span{
				Start: Position{Line: tok.Line, Column: tok.Column},
			},
		}, nil

	case TokenIdent:
		p.advance()
		return &Ident{
			Name: tok.Lexeme,
			Span: Span{
				Start: Position{Line: tok.Line, Column: tok.Column},
			},
		}, nil

	case TokenLeftParen:
		p.advance()
		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		if err := p.expectErr(TokenRightParen); err != nil {
			return nil, err
		}
		return expr, nil

	default:
		// Check for type constructors: vec3<f32>(1.0, 2.0, 3.0)
		if p.isTypeKeyword(tok.Kind) {
			typeExpr, err := p.typeSpec()
			if err != nil {
				return nil, err
			}
			return &ConstructExpr{
				Type: typeExpr,
				Span: Span{
					Start: Position{Line: tok.Line, Column: tok.Column},
				},
			}, nil
		}

		return nil, &ParseError{
			Message: fmt.Sprintf("unexpected token %s in expression", tok.Kind),
			Token:   tok,
		}
	}
}

// Helper methods

func (p *Parser) advance() Token {
	if !p.isAtEnd() {
		p.current++
	}
	return p.previous()
}

func (p *Parser) peek() Token {
	return p.tokens[p.current]
}

func (p *Parser) previous() Token {
	return p.tokens[p.current-1]
}

func (p *Parser) isAtEnd() bool {
	return p.peek().Kind == TokenEOF
}

func (p *Parser) check(kind TokenKind) bool {
	if p.isAtEnd() {
		return false
	}
	return p.peek().Kind == kind
}

func (p *Parser) match(kind TokenKind) bool {
	if p.check(kind) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(kind TokenKind) bool {
	if p.check(kind) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expectErr(kind TokenKind) *ParseError {
	if p.check(kind) {
		p.advance()
		return nil
	}
	return &ParseError{
		Message: fmt.Sprintf("expected %s, got %s", kind, p.peek().Kind),
		Token:   p.peek(),
	}
}

func (p *Parser) synchronize() {
	p.advance()
	for !p.isAtEnd() {
		if p.previous().Kind == TokenSemicolon {
			return
		}
		switch p.peek().Kind {
		case TokenFn, TokenStruct, TokenVar, TokenConst, TokenLet, TokenAlias:
			return
		}
		p.advance()
	}
}

func (p *Parser) isTypeKeyword(kind TokenKind) bool {
	switch kind {
	case TokenBool, TokenF16, TokenF32, TokenI32, TokenU32,
		TokenVec2, TokenVec3, TokenVec4,
		TokenMat2x2, TokenMat2x3, TokenMat2x4,
		TokenMat3x2, TokenMat3x3, TokenMat3x4,
		TokenMat4x2, TokenMat4x3, TokenMat4x4,
		TokenArray, TokenAtomic, TokenPtr,
		TokenSampler, TokenSamplerComparison,
		TokenTexture1d, TokenTexture2d, TokenTexture2dArray, TokenTexture3d,
		TokenTextureCube, TokenTextureCubeArray, TokenTextureMultisampled2d,
		TokenTextureStorage1d, TokenTextureStorage2d, TokenTextureStorage2dArray, TokenTextureStorage3d,
		TokenTextureDepth2d, TokenTextureDepth2dArray, TokenTextureDepthCube,
		TokenTextureDepthCubeArray, TokenTextureDepthMultisampled2d:
		return true
	}
	return false
}

func (p *Parser) isAssignOp(kind TokenKind) bool {
	switch kind {
	case TokenEqual, TokenPlusEqual, TokenMinusEqual, TokenStarEqual,
		TokenSlashEqual, TokenPercentEqual, TokenAmpEqual, TokenPipeEqual,
		TokenCaretEqual, TokenLessLessEqual, TokenGreaterGreaterEqual:
		return true
	}
	return false
}
