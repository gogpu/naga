package parser

import (
	"fmt"
)

// Parser error message constants.
const (
	errExpectedParameterName = "expected parameter name"
	errExpectedMemberName    = "expected member name"
	errExpectedVariableName  = "expected variable name"
	errExpectedType          = "expected type"
)

// Nesting limits. Without them a deeply nested source (a few hundred KB of
// `(`, `{` or `array<`) recursed until Go's fatal, unrecoverable "stack
// overflow" killed the host process.
const (
	// maxRecursionDepth is Rust naga's Parser::track_recursion bound
	// (front/wgsl/parse/mod.rs): the parse fails once the depth reaches it.
	// Rust counts expression, lhs_expression and statement; this parser
	// counts the same productions (see enterRecursion's callers).
	maxRecursionDepth = 200

	// maxBraceNesting is the WGSL limit "maximum nesting depth of
	// brace-enclosed statements in a function" (WGSL §2.4 Limits), Rust's
	// BRACE_NESTING_MAXIMUM; the function body's own braces are level 1.
	maxBraceNesting = 127

	// Rust's Error::Internal("Parser recursion limit exceeded") renders as
	// this message with the text as its note (front/wgsl/error.rs).
	errInternalFrontEnd = "internal WGSL front end error"
	noteRecursionLimit  = "Parser recursion limit exceeded"

	// Rust's Error::ExceededLimitForNestedBraces texts.
	errBraceNestingLimit   = "brace nesting limit reached"
	labelBraceNestingLimit = "limit reached at this brace"
)

// Parser parses WGSL tokens into an AST.
type Parser struct {
	tokens         []Token
	current        int
	errors         []ParseError
	inForHeader    bool // true when parsing for-loop init/update (no trailing semicolon)
	pendingEnables []Enable
	// seenDecl is set by the first global declaration, an empty one (`;`)
	// included: no directive may follow it.
	seenDecl bool
	// recursionDepth counts the nested productions Rust's track_recursion
	// counts; braceNesting the open braces of the current function.
	recursionDepth int
	braceNesting   int
}

// Errors returns every error Parse found, in source order.
func (p *Parser) Errors() []ParseError {
	return p.errors
}

// ParseError represents a parsing error.
type ParseError struct {
	Message string
	Token   Token
	// Label is the text of the label at Token, as Rust's; usually empty.
	Label string
	// Notes are Rust's notes for this error, if it has any.
	Notes []string
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
	// Estimate declaration counts from token count for pre-allocation.
	// Only pre-allocate for shaders with enough tokens to benefit from
	// avoiding slice regrowth. Small shaders (< ~100 tokens) are fine
	// with nil slice append growth.
	nTokens := len(p.tokens)
	module := &Module{}
	if nTokens > 100 {
		estDecls := nTokens/20 + 1
		module.Declarations = make([]Decl, 0, estDecls)
		module.Functions = make([]*FunctionDecl, 0, estDecls/3+1)
	}

	for !p.isAtEnd() {
		// Skip optional semicolons between declarations (e.g., struct Foo { ... };)
		for p.match(TokenSemicolon) {
			p.seenDecl = true
		}
		if p.isAtEnd() {
			break
		}

		decl, err := p.declaration()
		if err != nil {
			p.errors = append(p.errors, *err)
			p.synchronize()
			continue
		}
		if decl != nil {
			p.seenDecl = true
			// Preserve source order for all declarations.
			module.Declarations = append(module.Declarations, decl)
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
			case *OverrideDecl:
				module.Overrides = append(module.Overrides, d)
			}
		}
	}

	// Store parsed enable directives on the module.
	if len(p.pendingEnables) > 0 {
		module.Enables = p.pendingEnables
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

	if p.seenDecl && (p.check(TokenEnable) || p.check(TokenDiagnostic)) {
		// The grammar has every global_directive before the first
		// global_decl (Rust's Error::DirectiveAfterFirstGlobalDecl).
		return nil, &ParseError{Message: "expected global declaration, but found a global directive", Token: p.peek()}
	}

	if err := p.checkGlobalAttributeTargets(attrs); err != nil {
		return nil, err
	}

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
	case p.check(TokenConstAssert):
		return p.constAssertDecl()
	case p.check(TokenEnable):
		enable, err := p.enableDirective()
		if err != nil {
			return nil, err
		}
		// Store on module but don't return as a Decl (enable is not a declaration).
		// The caller (Parse) will store it via enableDirectives accumulated in the parser.
		p.pendingEnables = append(p.pendingEnables, enable)
		return nil, nil
	case p.check(TokenDiagnostic):
		// Skip diagnostic directives for now
		p.advance()
		for !p.check(TokenSemicolon) && !p.isAtEnd() {
			p.advance()
		}
		if p.check(TokenSemicolon) {
			p.advance()
		}
		return nil, nil
	case p.check(TokenOverride):
		return p.overrideDecl(attrs)
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

// attributeTarget is the kind of module-scope declaration an attribute
// applies to.
type attributeTarget uint8

const (
	targetFunction attributeTarget = iota + 1
	targetVariable
	targetOverride
)

// attributeTargetNames name each target in the error text, after Rust
// naga's FunctionMustUseOnNonFunction ("attribute `@must_use` is only valid
// on function declarations").
var attributeTargetNames = [...]string{
	targetFunction: "function",
	targetVariable: "variable",
	targetOverride: TokenOverride.String(),
}

// globalAttributeTargets maps each attribute that applies to one kind of
// module-scope declaration to that kind (WGSL spec, "Attributes"; the
// stages and entry-point attributes of the wgpu mesh-shader and
// ray-tracing extensions are function attributes as in Rust naga's
// Parser::global_decl). Attributes of parameters, results and struct
// members never reach a module-scope declaration; @diagnostic is checked by
// its own rules.
var globalAttributeTargets = map[string]attributeTarget{
	"vertex":           targetFunction,
	"fragment":         targetFunction,
	"compute":          targetFunction,
	"task":             targetFunction,
	"mesh":             targetFunction,
	"payload":          targetFunction,
	"ray_generation":   targetFunction,
	"any_hit":          targetFunction,
	"closest_hit":      targetFunction,
	"miss":             targetFunction,
	"incoming_payload": targetFunction,
	"workgroup_size":   targetFunction,
	"early_depth_test": targetFunction,
	"must_use":         targetFunction,
	"group":            targetVariable,
	"binding":          targetVariable,
	"id":               targetOverride,
}

// checkGlobalAttributeTargets rejects an attribute on a module-scope
// declaration it does not apply to, a shader-creation error by the WGSL
// spec. The next token starts the declaration. Rust naga rejects only
// @must_use on a non-function and @group/@binding without a `var`, and
// drops the other misplaced attributes silently; naga follows the spec.
func (p *Parser) checkGlobalAttributeTargets(attrs []Attribute) *ParseError {
	var target attributeTarget
	switch p.peek().Kind {
	case TokenFn:
		target = targetFunction
	case TokenVar:
		target = targetVariable
	case TokenOverride:
		target = targetOverride
	}
	for i := range attrs {
		want, ok := globalAttributeTargets[attrs[i].Name]
		if !ok || want == target {
			continue
		}
		name := attrs[i].NameSpan.Start
		return &ParseError{
			Message: fmt.Sprintf("attribute `@%s` is only valid on %s declarations", attrs[i].Name, attributeTargetNames[want]),
			Token:   Token{Kind: TokenIdent, Lexeme: attrs[i].Name, Line: name.Line, Column: name.Column},
		}
	}
	return nil
}

// attributes parses a list of attributes (@location(0), @vertex, etc.)
func (p *Parser) attributes() []Attribute {
	var attrs []Attribute

	for p.check(TokenAt) {
		start := p.peek()
		p.advance() // consume @

		// Accept both identifiers and keyword tokens as attribute names.
		// E.g., @diagnostic(...) — "diagnostic" is a keyword but valid as attr name.
		if !p.check(TokenIdent) && !p.check(TokenDiagnostic) {
			continue
		}

		name := p.advance()
		attr := Attribute{
			Name: name.Lexeme,
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
			NameSpan: WordSpan(Position{Line: name.Line, Column: name.Column}, name.Lexeme),
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
		return nil, &ParseError{Message: errExpectedParameterName, Token: p.peek()}
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
func (p *Parser) structDecl(_ []Attribute) (*StructDecl, *ParseError) {
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
		return nil, &ParseError{Message: errExpectedMemberName, Token: p.peek()}
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
	var addressSpaceSpan Span
	var accessMode string
	if p.match(TokenLess) {
		if p.check(TokenIdent) {
			asTok := p.advance()
			addressSpace = asTok.Lexeme
			addressSpaceSpan = WordSpan(Position{Line: asTok.Line, Column: asTok.Column}, asTok.Lexeme)
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
		return nil, &ParseError{Message: errExpectedVariableName, Token: p.peek()}
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

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &VarDecl{
		Name:             name.Lexeme,
		Type:             varType,
		Init:             init,
		AddressSpace:     addressSpace,
		AddressSpaceSpan: addressSpaceSpan,
		AccessMode:       accessMode,
		Attributes:       attrs,
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

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &ConstDecl{
		Name:    name.Lexeme,
		Type:    constType,
		Init:    init,
		IsConst: true,
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
		return nil, &ParseError{Message: errExpectedVariableName, Token: p.peek()}
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

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &ConstDecl{
		Name: name.Lexeme,
		Type: letType,
		Init: init,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// overrideDecl parses an override declaration (pipeline-overridable constant).
// WGSL spec: @id(N) override name: type = default;
// The initializer is optional (overrides without defaults must be set at pipeline creation).
func (p *Parser) overrideDecl(attrs []Attribute) (*OverrideDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenOverride) {
		return nil, &ParseError{Message: "expected 'override'", Token: p.peek()}
	}

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: "expected override name", Token: p.peek()}
	}
	name := p.advance()

	// Optional type annotation
	var overrideType Type
	if p.match(TokenColon) {
		t, err := p.typeSpec()
		if err != nil {
			return nil, err
		}
		overrideType = t
	}

	// Optional initializer
	var init Expr
	if p.match(TokenEqual) {
		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		init = expr
	}

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &OverrideDecl{
		Name:       name.Lexeme,
		Type:       overrideType,
		Init:       init,
		Attributes: attrs,
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

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &AliasDecl{
		Name: name.Lexeme,
		Type: aliasType,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// constAssertDecl parses a const_assert declaration.
// WGSL spec: const_assert expr; or const_assert(expr);
// enableDirective parses an enable directive: enable ext1, ext2;
// Returns the Enable with all listed extension names. A name that is no
// enable-extension is Rust naga's UnknownEnableExtension (EnableExtension::
// from_ident, naga/src/front/wgsl/parse/directive/enable_extension.rs).
func (p *Parser) enableDirective() (Enable, *ParseError) {
	start := p.peek()
	p.advance() // consume 'enable'

	var extensions []string
	for {
		if p.check(TokenSemicolon) || p.isAtEnd() {
			break
		}
		tok := p.peek()
		if !IsEnableExtension(tok.Lexeme) {
			return Enable{}, &ParseError{Message: fmt.Sprintf("unknown enable-extension `%s`", tok.Lexeme), Token: tok}
		}
		extensions = append(extensions, tok.Lexeme)
		p.advance()
		if !p.match(TokenComma) {
			break
		}
	}
	if p.check(TokenSemicolon) {
		p.advance()
	}

	return Enable{
		Extensions: extensions,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

func (p *Parser) constAssertDecl() (*ConstAssertDecl, *ParseError) {
	start := p.peek()
	if !p.match(TokenConstAssert) {
		return nil, &ParseError{Message: "expected 'const_assert'", Token: p.peek()}
	}

	// const_assert can optionally have parentheses: const_assert(expr) or const_assert expr
	hasParen := p.match(TokenLeftParen)
	cond, err := p.expression()
	if err != nil {
		return nil, err
	}
	if hasParen {
		if err := p.expectErr(TokenRightParen); err != nil {
			return nil, err
		}
	}

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &ConstAssertDecl{
		Condition: cond,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// typeSpec parses a type specification.
func (p *Parser) typeSpec() (Type, *ParseError) {
	tok := p.peek()

	// Array type: array<f32, 4> or array (without template args for inferred type)
	if p.check(TokenArray) {
		p.advance() // consume 'array'
		if p.check(TokenLess) {
			p.advance() // consume '<'

			elemType, err := p.templateArgType()
			if err != nil {
				return nil, err
			}

			var size Expr
			if p.match(TokenComma) {
				// Trailing comma without size: array<u32,>
				if !p.check(TokenGreater) {
					// Parse a template argument expression for size.
					// This uses templateArgExpr which handles shift/arithmetic but
					// stops before > or >= to avoid consuming the template closing >.
					size, err = p.templateArgExpr()
					if err != nil {
						return nil, err
					}
					// Allow trailing comma after size: array<u32, 1,>
					p.match(TokenComma)
				}
			}

			if err := p.expectTemplateClose(); err != nil {
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
		// No template args: array(...) with inferred type — return as NamedType
		return &NamedType{
			Name: "array", //nolint:goconst // WGSL intrinsic type name, also used in lower.go
			Span: Span{
				Start: Position{Line: tok.Line, Column: tok.Column},
			},
		}, nil
	}

	// Binding array type: binding_array<texture_2d<f32>> or binding_array<texture_2d<f32>, 5>
	if p.check(TokenIdent) && tok.Lexeme == "binding_array" {
		p.advance() // consume 'binding_array'
		if err := p.expectErr(TokenLess); err != nil {
			return nil, err
		}

		elemType, err := p.templateArgType()
		if err != nil {
			return nil, err
		}

		var size Expr
		if p.match(TokenComma) {
			size, err = p.primary()
			if err != nil {
				return nil, err
			}
		}

		if err := p.expectErr(TokenGreater); err != nil {
			return nil, err
		}

		return &BindingArrayType{
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
		asTok := p.advance()
		addressSpace := asTok.Lexeme

		if err := p.expectErr(TokenComma); err != nil {
			return nil, err
		}

		pointeeType, err := p.templateArgType()
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
			AddressSpace:     addressSpace,
			AddressSpaceSpan: WordSpan(Position{Line: asTok.Line, Column: asTok.Column}, asTok.Lexeme),
			PointeeType:      pointeeType,
			AccessMode:       accessMode,
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
				paramType, err := p.templateArgType()
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

	return nil, &ParseError{Message: errExpectedType, Token: tok}
}

// block parses a block statement.
func (p *Parser) block() (*BlockStmt, *ParseError) {
	start := p.peek()
	if err := p.openBrace(); err != nil {
		return nil, err
	}
	defer p.closeBrace()

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

// statement parses a statement. It counts toward the recursion limit, as
// Rust's statement().
func (p *Parser) statement() (Stmt, *ParseError) {
	err := p.enterRecursion()
	defer p.leaveRecursion()
	if err != nil {
		return nil, err
	}
	return p.statementKind()
}

// statementKind parses a statement without counting a recursion level: the
// init and update clauses of a for header, which Rust parses without
// statement().
func (p *Parser) statementKind() (Stmt, *ParseError) {
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
	case p.check(TokenConstAssert):
		return p.constAssertDecl()
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

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &ReturnStmt{
		Value: value,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// ifStmt parses an if statement with its else-if chain.
//
// The chain is read in a loop, as Rust's statement() does, and nested
// afterwards: `else if` blocks are siblings, so a long chain neither
// recurses here nor counts toward the recursion or brace limits.
func (p *Parser) ifStmt() (*IfStmt, *ParseError) {
	var chain []*IfStmt
	var elseBlock *BlockStmt
	for {
		start := p.advance() // consume 'if'

		cond, err := p.expression()
		if err != nil {
			return nil, err
		}

		body, err := p.block()
		if err != nil {
			return nil, err
		}

		chain = append(chain, &IfStmt{
			Condition: cond,
			Body:      body,
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
		})

		if !p.match(TokenElse) {
			break
		}
		if !p.check(TokenIf) {
			elseBlock, err = p.block()
			if err != nil {
				return nil, err
			}
			break
		}
	}

	// Nest the chain: each `if` is the else of the one before it.
	if elseBlock != nil {
		chain[len(chain)-1].Else = elseBlock
	}
	for i := len(chain) - 1; i > 0; i-- {
		chain[i-1].Else = chain[i]
	}
	return chain[0], nil
}

// forStmt parses a for statement.
func (p *Parser) forStmt() (*ForStmt, *ParseError) {
	start := p.advance() // consume 'for'

	if err := p.expectErr(TokenLeftParen); err != nil {
		return nil, err
	}

	// Init — parsed without trailing semicolon (for-loop uses ; as separator)
	var init Stmt
	if !p.check(TokenSemicolon) {
		p.inForHeader = true
		s, err := p.statementKind()
		p.inForHeader = false
		if err != nil {
			return nil, err
		}
		init = s
	}
	if err := p.expectErr(TokenSemicolon); err != nil {
		return nil, err
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
	if err := p.expectErr(TokenSemicolon); err != nil {
		return nil, err
	}

	// Update — parsed without trailing semicolon (for-loop ends with ))
	var update Stmt
	if !p.check(TokenRightParen) {
		p.inForHeader = true
		s, err := p.statementKind()
		p.inForHeader = false
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
// WGSL loop syntax: loop { body_stmts... continuing { stmts... } }
// The continuing block is optional and appears at the end of the loop body.
func (p *Parser) loopStmt() (*LoopStmt, *ParseError) {
	start := p.advance() // consume 'loop'

	if err := p.openBrace(); err != nil {
		return nil, err
	}
	defer p.closeBrace()

	// Parse body statements, stopping at 'continuing' or '}'
	bodyStmts := make([]Stmt, 0, 4)
	for !p.check(TokenRightBrace) && !p.check(TokenContinuing) && !p.isAtEnd() {
		stmt, err := p.statement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			bodyStmts = append(bodyStmts, stmt)
		}
	}

	body := &BlockStmt{
		Statements: bodyStmts,
		Span:       Span{Start: Position{Line: start.Line, Column: start.Column}},
	}

	// Parse optional continuing block
	var continuing *BlockStmt
	if p.check(TokenContinuing) {
		p.advance() // consume 'continuing'
		var err *ParseError
		continuing, err = p.block()
		if err != nil {
			return nil, err
		}
	}

	if err := p.expectErr(TokenRightBrace); err != nil {
		return nil, err
	}

	return &LoopStmt{
		Body:       body,
		Continuing: continuing,
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

	if err := p.openBrace(); err != nil {
		return nil, err
	}
	defer p.closeBrace()

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
//
// WGSL switch clause syntax:
//
//	case expr1, expr2, default: { ... }   -- mixed selectors with default
//	case expr1, expr2: { ... }            -- comma-separated selectors
//	case expr1, { ... }                   -- trailing comma, no colon
//	default: { ... }                      -- standalone default with colon
//	default { ... }                       -- standalone default without colon
//
// The colon before the block is optional in modern WGSL.
func (p *Parser) switchCaseClause() (*SwitchCaseClause, *ParseError) {
	start := p.peek()
	var selectors []Expr
	isDefault := false
	defaultFirst := false

	if p.match(TokenDefault) {
		isDefault = true
	} else if p.match(TokenCase) {
		// Parse comma-separated selectors, which may include 'default'.
		// Examples: case 0, 1:   case default, 6:   case 1, default:
		for {
			// Check for 'default' keyword as a selector
			if p.check(TokenDefault) {
				p.advance()
				isDefault = true
				if len(selectors) == 0 {
					defaultFirst = true
				}
			} else {
				// Stop if the next token starts the body (trailing comma case)
				if p.check(TokenColon) || p.check(TokenLeftBrace) {
					break
				}
				expr, err := p.expression()
				if err != nil {
					return nil, err
				}
				selectors = append(selectors, expr)
			}
			if !p.match(TokenComma) {
				break
			}
		}
	} else {
		return nil, &ParseError{Message: "expected 'case' or 'default'", Token: start}
	}

	// Colon is optional in modern WGSL
	p.match(TokenColon)

	body, err := p.block()
	if err != nil {
		return nil, err
	}

	return &SwitchCaseClause{
		Selectors:    selectors,
		IsDefault:    isDefault,
		DefaultFirst: defaultFirst,
		Body:         body,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// localConstStmt parses a local const declaration inside a function.
func (p *Parser) localConstStmt() (*ConstDecl, *ParseError) {
	return p.constDecl()
}

// breakStmt parses a break or "break if" statement.
// WGSL spec: "break if expr;" is valid inside a continuing block.
func (p *Parser) breakStmt() (Stmt, *ParseError) {
	start := p.advance() // consume 'break'

	// Check for "break if expr;" syntax (inside continuing block)
	if p.check(TokenIf) {
		p.advance() // consume 'if'
		cond, err := p.expression()
		if err != nil {
			return nil, err
		}
		if err := p.expectSemicolon(); err != nil {
			return nil, err
		}
		return &BreakIfStmt{
			Condition: cond,
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
		}, nil
	}

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}
	return &BreakStmt{
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// continueStmt parses a continue statement.
func (p *Parser) continueStmt() (*ContinueStmt, *ParseError) {
	start := p.advance() // consume 'continue'
	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}
	return &ContinueStmt{
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// discardStmt parses a discard statement.
func (p *Parser) discardStmt() (*DiscardStmt, *ParseError) {
	start := p.advance() // consume 'discard'
	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}
	return &DiscardStmt{
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// letStmt parses a let statement (local variable).
func (p *Parser) letStmt() (*ConstDecl, *ParseError) {
	start := p.peek()
	p.advance() // consume 'let'

	if !p.check(TokenIdent) {
		return nil, &ParseError{Message: errExpectedVariableName, Token: p.peek()}
	}
	name := p.advance()

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

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}

	return &ConstDecl{
		Name: name.Lexeme,
		Type: letType,
		Init: init,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// exprOrAssignStmt parses an expression statement or assignment.
func (p *Parser) exprOrAssignStmt() (Stmt, *ParseError) {
	start := p.peek()
	// Not expression(): Rust parses a call statement's arguments one
	// recursion level below the statement, and counting this outer
	// expression would put them two below. An assignment's left side then
	// counts one level fewer than Rust's lhs_expression, which only accepts
	// more, never less.
	expr, err := p.logicalOr()
	if err != nil {
		return nil, err
	}

	// Check for increment/decrement: i++ or i--
	// Desugar to compound assignment: i += 1 or i -= 1
	if p.check(TokenPlusPlus) || p.check(TokenMinusMinus) {
		op := TokenPlusEqual
		if p.peek().Kind == TokenMinusMinus {
			op = TokenMinusEqual
		}
		p.advance() // consume ++ or --
		if err := p.expectSemicolon(); err != nil {
			return nil, err
		}
		return &AssignStmt{
			Left: expr,
			Op:   op,
			Right: &Literal{
				Kind:  TokenIntLiteral,
				Value: "1",
			},
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
			Increment: true,
		}, nil
	}

	// Check for assignment operators
	if p.isAssignOp(p.peek().Kind) {
		op := p.advance()
		right, err := p.expression()
		if err != nil {
			return nil, err
		}
		if err := p.expectSemicolon(); err != nil {
			return nil, err
		}
		return &AssignStmt{
			Left:  expr,
			Op:    op.Kind,
			Right: right,
			Span: Span{
				Start: Position{Line: start.Line, Column: start.Column},
			},
		}, nil
	}

	if err := p.expectSemicolon(); err != nil {
		return nil, err
	}
	return &ExprStmt{
		Expr: expr,
		Span: Span{
			Start: Position{Line: start.Line, Column: start.Column},
		},
	}, nil
}

// expression parses an expression.
func (p *Parser) expression() (Expr, *ParseError) {
	err := p.enterRecursion()
	defer p.leaveRecursion()
	if err != nil {
		return nil, err
	}
	return p.logicalOr()
}

// templateArgExpr parses an expression inside a template argument list.
// It handles shift/arithmetic operators but stops at > and >= tokens
// to avoid consuming the template-closing >. It is Rust's expression() in a
// template list, so it counts toward the recursion limit.
func (p *Parser) templateArgExpr() (Expr, *ParseError) {
	err := p.enterRecursion()
	defer p.leaveRecursion()
	if err != nil {
		return nil, err
	}
	return p.templateShift()
}

// templateShift parses << expressions inside template args (>> would be template close).
func (p *Parser) templateShift() (Expr, *ParseError) {
	left, err := p.additive()
	if err != nil {
		return nil, err
	}

	// Only allow left-shift (<<) inside template args, not right-shift (>>)
	for p.check(TokenLessLess) {
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
//
// The prefix operators are collected in a loop and applied innermost first,
// as Rust's unary_expression: a chain of them neither recurses nor counts
// toward the recursion limit.
func (p *Parser) unary() (Expr, *ParseError) {
	var ops []Token
	for p.check(TokenMinus) || p.check(TokenBang) || p.check(TokenTilde) ||
		p.check(TokenAmpersand) || p.check(TokenStar) {
		ops = append(ops, p.advance())
	}

	expr, err := p.postfix()
	if err != nil {
		return nil, err
	}
	for i := len(ops) - 1; i >= 0; i-- {
		op := ops[i]
		expr = &UnaryExpr{
			Op:      op.Kind,
			Operand: expr,
			Span: Span{
				Start: Position{Line: op.Line, Column: op.Column},
			},
		}
	}
	return expr, nil
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
				return nil, &ParseError{Message: errExpectedMemberName, Token: p.peek()}
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
		// Handle bitcast<Type>(expr) — special syntax
		if tok.Lexeme == "bitcast" {
			p.advance() // consume 'bitcast'
			if err := p.expectErr(TokenLess); err != nil {
				return nil, err
			}
			targetType, err := p.templateArgType()
			if err != nil {
				return nil, err
			}
			if err := p.expectErr(TokenGreater); err != nil {
				return nil, err
			}
			if err := p.expectErr(TokenLeftParen); err != nil {
				return nil, err
			}
			arg, err := p.expression()
			if err != nil {
				return nil, err
			}
			if err := p.expectErr(TokenRightParen); err != nil {
				return nil, err
			}
			return &BitcastExpr{
				Type: targetType,
				Expr: arg,
				Span: Span{
					Start: Position{Line: tok.Line, Column: tok.Column},
				},
			}, nil
		}
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

// Nesting limits

// enterRecursion counts one more nested production, as Rust's
// Parser::track_recursion, and fails once the depth reaches
// maxRecursionDepth. Every call is paired with a deferred leaveRecursion,
// so the depth unwinds on error paths too.
func (p *Parser) enterRecursion() *ParseError {
	p.recursionDepth++
	if p.recursionDepth >= maxRecursionDepth {
		return &ParseError{
			Message: errInternalFrontEnd,
			Token:   p.peek(),
			Notes:   []string{noteRecursionLimit},
		}
	}
	return nil
}

func (p *Parser) leaveRecursion() {
	p.recursionDepth--
}

// openBrace consumes the `{` of a brace-enclosed statement and enters one
// more brace level, as Rust's increase_brace_nesting. On success the caller
// defers closeBrace.
func (p *Parser) openBrace() *ParseError {
	brace := p.peek()
	if err := p.expectErr(TokenLeftBrace); err != nil {
		return err
	}
	if p.braceNesting+1 > maxBraceNesting {
		return &ParseError{
			Message: errBraceNestingLimit,
			Token:   brace,
			Label:   labelBraceNestingLimit,
			Notes:   []string{fmt.Sprintf("nesting limit is currently set to %d", maxBraceNesting)},
		}
	}
	p.braceNesting++
	return nil
}

func (p *Parser) closeBrace() {
	p.braceNesting--
}

// templateArgType parses a type written as a template argument
// (`array<T>`, `vec3<T>`, `ptr<function, T>`, `bitcast<T>`). Rust parses
// every template argument with expression(), so each one counts toward the
// recursion limit; the outermost type of a declaration does not.
func (p *Parser) templateArgType() (Type, *ParseError) {
	err := p.enterRecursion()
	defer p.leaveRecursion()
	if err != nil {
		return nil, err
	}
	return p.typeSpec()
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

// expectSemicolon requires a semicolon unless inside a for-loop header
// (where semicolons are separators consumed by the for-loop parser).
func (p *Parser) expectSemicolon() *ParseError {
	if p.inForHeader {
		return nil
	}
	return p.expectErr(TokenSemicolon)
}

func (p *Parser) match(kind TokenKind) bool {
	if p.check(kind) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) expect(kind TokenKind) {
	if p.check(kind) {
		p.advance()
		return
	}
	// Handle >> splitting: when expecting >, accept >> and split it
	if kind == TokenGreater && p.check(TokenGreaterGreater) {
		p.splitGreaterGreater()
	}
}

func (p *Parser) expectErr(kind TokenKind) *ParseError {
	if p.check(kind) {
		p.advance()
		return nil
	}
	// Handle >> splitting: when expecting >, accept >> and split it
	if kind == TokenGreater && p.check(TokenGreaterGreater) {
		p.splitGreaterGreater()
		return nil
	}
	return &ParseError{
		Message: fmt.Sprintf("expected %s, got %s", kind, p.peek().Kind),
		Token:   p.peek(),
	}
}

// expectTemplateClose expects a > token to close a template argument list.
// Handles >= disambiguation: splits >= into > and = tokens per WGSL spec.
// Also handles >> splitting like expectErr(TokenGreater).
func (p *Parser) expectTemplateClose() *ParseError {
	if p.check(TokenGreater) {
		p.advance()
		return nil
	}
	if p.check(TokenGreaterGreater) {
		p.splitGreaterGreater()
		return nil
	}
	if p.check(TokenGreaterEqual) {
		p.splitGreaterEqual()
		return nil
	}
	if p.check(TokenGreaterGreaterEqual) {
		p.splitGreaterGreaterEqual()
		return nil
	}
	return &ParseError{
		Message: fmt.Sprintf("expected >, got %s", p.peek().Kind),
		Token:   p.peek(),
	}
}

// splitGreaterGreater splits a >> token into two > tokens, consuming the first.
// This handles the WGSL angle bracket ambiguity in nested template args (e.g., vec3<f32>>).
func (p *Parser) splitGreaterGreater() {
	tok := p.tokens[p.current]
	// Replace >> with a single > at position+1
	p.tokens[p.current] = Token{
		Kind:   TokenGreater,
		Lexeme: ">",
		Line:   tok.Line,
		Column: tok.Column + 1,
	}
	// Don't advance — the remaining > stays for the outer template close
}

// splitGreaterEqual splits a >= token into > and = tokens, consuming the >.
// This handles the WGSL template disambiguation: array<i32, 1 << 1>=...
func (p *Parser) splitGreaterEqual() {
	tok := p.tokens[p.current]
	// Replace >= with = at position+1
	p.tokens[p.current] = Token{
		Kind:   TokenEqual,
		Lexeme: "=",
		Line:   tok.Line,
		Column: tok.Column + 1,
	}
	// Don't advance — the = stays for the next parse
}

// splitGreaterGreaterEqual splits a >>= token into > and >= tokens, consuming the >.
func (p *Parser) splitGreaterGreaterEqual() {
	tok := p.tokens[p.current]
	// Replace >>= with >= at position+1
	p.tokens[p.current] = Token{
		Kind:   TokenGreaterEqual,
		Lexeme: ">=",
		Line:   tok.Line,
		Column: tok.Column + 1,
	}
	// Don't advance — the >= stays for the next parse
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
	case TokenBool, TokenF16, TokenF32, TokenF64, TokenI32, TokenI64, TokenU32, TokenU64,
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
