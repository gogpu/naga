package front

import "github.com/gogpu/naga/ir"

// This file parses declarations: variable declaration lists, function
// prototypes and definitions, interface blocks, precision statements and the
// declarations that carry only qualifiers, Rust naga's parser/declarations.rs.
//
// Everything here is syntax. No ir.Type is built and no constant expression is
// evaluated — an array size, a layout value and the types of an initializer's
// elements are lowered later — so the only names resolved are struct type names
// (p.types) and ordinary names (p.symbols), which GLSL requires to be declared
// before use and which Rust naga reports as unknown while parsing.
//
// Rust also reports the layout qualifiers a declaration leaves unconsumed
// (TypeQualifiers::unused_errors, parser/declarations.rs:439, 473). Qualifiers
// are consumed in lowering here, so that report belongs to lowering too.

// kindIs returns a token predicate that matches the token kind k; it is the
// counterpart of punct for the tokens that carry no spelling.
func kindIs(k tokenKind) func(glslToken) bool {
	return func(t glslToken) bool { return t.kind == k }
}

// parseDeclaration parses one declaration, Rust naga's parse_declaration
// (parser/declarations.rs:294-532): a function prototype or definition, a
// variable declaration list, an interface block, a precision statement, or a
// qualifier list with no type. It returns (nil, nil) when the next token starts
// no declaration, so a statement parser can fall through to expression parsing;
// a non-nil error aborts the parse.
//
// external says whether the declaration is at module scope: only there may a
// function carry a body, and only there do the declared names go into the root
// scope of the symbol table. Rust's is_inside_loop, which decides whether a
// local initializer becomes a Store statement instead of part of the
// declaration (declarations.rs:245-265), is derived by lowering from the
// statement tree, so this pass does not carry it.
func (p *parser) parseDeclaration(external bool) (*astDecl, *frontError) {
	if p.peekTypeQualifier() || p.peekTypeName() {
		qualifiers, err := p.parseTypeQualifiers()
		if err != nil {
			return nil, err
		}
		if p.peekTypeName() {
			return p.parseTypedDeclaration(qualifiers, external)
		}
		return p.parseQualifiedNameDeclaration(qualifiers)
	}
	if p.atKind(tokPrecision) {
		return p.parsePrecisionDeclaration()
	}
	return nil, nil
}

// parseTypedDeclaration parses a declaration that starts with a type: an
// init_declarator_list, or a function prototype or definition when the name is
// followed by a parenthesis, Rust naga's type-name branch of parse_declaration
// (parser/declarations.rs:316-417).
func (p *parser) parseTypedDeclaration(qualifiers typeQualifiers, external bool) (*astDecl, *frontError) {
	typ, typeSpan, err := p.parseTypeExemptingPrecision(qualifiers.Precision != precisionNone)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.bump()
	if err != nil {
		return nil, err
	}
	if nameTok.kind == tokIdent {
		next, err := p.expectPeek()
		if err != nil {
			return nil, err
		}
		if next.isPunct("(") {
			return p.parseFunctionDeclaration(qualifiers, typ, typeSpan, nameTok, external)
		}
	}
	if typ == nil {
		// `void` cannot be a variable's type. Rust reports the error at the type
		// span, keeps the bumped name consumed and returns without a declarator
		// list (declarations.rs:400-416), which is mirrored here with a node
		// lowering ignores.
		p.errors = append(p.errors, *errSemantic(typeSpan, "Declaration cannot have void type"))
		return &astDecl{
			Kind: declInvalid{},
			Span: subsumeSpan(typeSpan, nameTok.span),
		}, nil
	}
	// The name was bumped to look past it; the declarator list parses it again.
	if err := p.backtrack(nameTok); err != nil {
		return nil, err
	}
	declarators, err := p.parseInitDeclaratorList(external)
	if err != nil {
		return nil, err
	}
	return &astDecl{
		Kind: declVarList{Qualifiers: qualifiers, Typ: *typ, Declarators: declarators},
		Span: subsumeSpan(typeSpan, p.last),
	}, nil
}

// parseFunctionDeclaration parses a function prototype or definition after its
// name, which the caller has bumped and whose following parenthesis it has
// peeked, Rust naga's parenthesis arm of parse_declaration
// (parser/declarations.rs:322-386). typ is nil for a void return type.
//
// The parameters and the body share one scope, which Rust opens by parsing the
// parameters into a fresh Context (declarations.rs:330) and closes with the
// function; a prototype's scope ends at its semicolon.
func (p *parser) parseFunctionDeclaration(
	qualifiers typeQualifiers, typ *typeSpec, typeSpan ir.Span, nameTok glslToken, external bool,
) (*astDecl, *frontError) {
	if _, err := p.expectPunct("("); err != nil {
		return nil, err
	}
	p.pushScope()
	defer p.popScope()

	params, err := p.parseParamList()
	if err != nil {
		return nil, err
	}
	if _, err := p.expectPunct(")"); err != nil {
		return nil, err
	}
	fn := declFunc{Qualifiers: qualifiers, Return: typ, Name: nameTok.text, Params: params}

	tok, err := p.bump()
	if err != nil {
		return nil, err
	}
	switch {
	case tok.isPunct(";"):
		fn.Span = subsumeSpan(typeSpan, tok.span)
		return &astDecl{Kind: fn, Span: fn.Span}, nil
	case tok.isPunct("{") && external:
		// The opening brace is consumed, the body's statements follow as in any
		// other compound statement. A function body is never a loop body, which
		// is the false Rust passes (functions.rs:356-362).
		body, err := p.parseCompoundStatement()
		if err != nil {
			return nil, err
		}
		if body == nil {
			// A nil body is what marks a prototype (declFunc), so a definition
			// with an empty body records the empty slice.
			body = []astStmt{}
		}
		fn.Body = body
		fn.Span = subsumeSpan(typeSpan, p.last)
		return &astDecl{Kind: fn, Span: fn.Span}, nil
	case external:
		return nil, invalidToken(tok, wantPunct("{"), wantPunct(";"))
	default:
		return nil, invalidToken(tok, wantPunct(";"))
	}
}

// parseInitDeclaratorList parses the names of a declaration and their
// initializers up to and including the semicolon, Rust naga's
// parse_init_declarator_list (parser/declarations.rs:167-291).
//
// Each name is declared as soon as its own initializer is parsed, which is what
// Rust's ctx.add_var inside the loop does (declarations.rs:267) and what makes
// `int a = 1, b = a;` legal while `int b = a, a = 1;` is not. The qualifiers
// and the type, which Rust's DeclarationContext also threads through here, are
// recorded on the declaration node with the declarators: storage class,
// element typing and initializer placement are lowering's.
func (p *parser) parseInitDeclaratorList(external bool) ([]declarator, *frontError) {
	// Rust consumes one leading comma, e.g. this is valid: `float, a = 1;`
	// (declarations.rs:187-193).
	p.bumpIf(punct(","))

	var declarators []declarator
	for {
		tok, err := p.bump()
		if err != nil {
			return nil, err
		}
		if tok.isPunct(";") {
			return declarators, nil
		}
		if tok.kind != tokIdent {
			return nil, invalidToken(tok, wantIdentifier, wantPunct(";"))
		}
		declarator, err := p.parseDeclarator(tok)
		if err != nil {
			return nil, err
		}
		declarators = append(declarators, *declarator)
		declarators[len(declarators)-1].Sym = p.declareDeclarator(*declarator, external)

		tok, err = p.bump()
		if err != nil {
			return nil, err
		}
		switch {
		case tok.isPunct(";"):
			return declarators, nil
		case tok.isPunct(","):
		default:
			return nil, invalidToken(tok, wantPunct(","), wantPunct(";"))
		}
	}
}

// parseDeclarator parses one name of an init_declarator_list: the name its
// caller bumped, its dimensions and its initializer. The span covers the name
// through the initializer, as Rust's does (declarations.rs:210-243).
func (p *parser) parseDeclarator(nameTok glslToken) (*declarator, *frontError) {
	d := declarator{Name: nameTok.text, Span: nameTok.span}
	arrays, err := p.parseArraySpecifier()
	if err != nil {
		return nil, err
	}
	d.Arrays = arrays
	for _, spec := range arrays {
		d.Span = subsumeSpan(d.Span, spec.Span)
	}
	if p.bumpIf(punct("=")) != nil {
		init, err := p.parseInitializer()
		if err != nil {
			return nil, err
		}
		d.Init = init
		d.Span = subsumeSpan(d.Span, init.Root.Span)
	}
	return &d, nil
}

// declareDeclarator installs the name of one declarator in the scope that owns
// it — the root scope for a module-scope declaration, the innermost one for a
// local — and returns its symbol.
func (p *parser) declareDeclarator(d declarator, external bool) *glslSymbol {
	if external {
		// Rust adds a module-scope name unconditionally and discards the symbol
		// it replaces (context.rs:232, where add_global ignores the Option
		// SymbolTable::add returns), so a repeated global name is not an error
		// and the latest declaration wins.
		sym := &glslSymbol{Kind: symGlobal, Name: d.Name}
		p.symbols.declareGlobal(sym)
		return sym
	}
	return p.declareLocal(d.Name, symLocal, d.Span)
}

// declareLocal records a local or a parameter name in the innermost scope and
// reports a name already declared there, Rust naga's add_local_var
// (variables.rs:694-701): the new declaration replaces the old one and the
// replacement is reported. A name of an enclosing scope is unrelated, because
// shadowing is legal GLSL.
func (p *parser) declareLocal(name string, kind symbolKind, span ir.Span) *glslSymbol {
	sym := &glslSymbol{Kind: kind, Name: name}
	if p.symbols.declare(sym) != nil {
		p.errors = append(p.errors, *errVariableAlreadyDeclared(name, span))
	}
	return sym
}

// parseQualifiedNameDeclaration parses the declaration branch that starts with
// qualifiers and no type: an interface block, an invariant redeclaration, or a
// qualifier list that carries only module metadata such as
// `layout(local_size_x = 8) in;`. Rust naga's second branch of parse_declaration
// (parser/declarations.rs:418-485).
func (p *parser) parseQualifiedNameDeclaration(qualifiers typeQualifiers) (*astDecl, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return nil, err
	}
	if tok.isPunct(";") {
		// Rust reads local_size_x/y/z and early_fragment_tests into the module
		// metadata here (declarations.rs:453-476). They stay in the qualifier
		// list of the AST for lowering to read.
		return &astDecl{
			Kind: declQualifiers{Qualifiers: qualifiers},
			Span: subsumeSpan(qualifiers.Span, tok.span),
		}, nil
	}
	if tok.kind != tokIdent {
		return nil, invalidToken(tok, wantIdentifier, wantPunct(";"))
	}
	if p.bumpIf(punct("{")) != nil {
		return p.parseBlockDeclaration(qualifiers, tok.text, tok.span)
	}
	if qualifiers.Invariant {
		end, err := p.expectPunct(";")
		if err != nil {
			return nil, err
		}
		// Rust marks the variable's binding invariant and accepts a name it
		// cannot resolve (variables.rs:226-243, which reports nothing for an
		// unknown one); the name alone is what lowering needs. The span covers
		// the qualifier too, as Rust returns the qualifier span here
		// (declarations.rs:441).
		sym := p.symbols.lookup(tok.text)
		if sym == nil {
			sym = p.builtinSymbol(tok.text)
		}
		return &astDecl{
			Kind: declInvariant{Name: tok.text, Sym: sym},
			Span: subsumeSpan(qualifiers.Span, end.span),
		}, nil
	}
	// Rust's NotImplemented text for a qualified name that is not a block, the
	// declaration it refuses (declarations.rs:444-450). A type the frontend
	// deliberately does not model gets its own message first; `uniform
	// atomic_uint counter;` is the atomic counter declaration (atomics.go).
	if reason := unsupportedTypeText(tok.text); reason != "" {
		return nil, errSemantic(tok.span, "%s", reason)
	}
	return nil, errSemantic(tok.span, "Not implemented: variable qualifier")
}

// parseBlockDeclaration parses an interface block after its name and opening
// brace, `uniform Blk { ... } instance;`, Rust naga's parse_block_declaration
// (parser/declarations.rs:534-622). The opening brace is already consumed.
//
// The block's name is not a type: Rust never inserts it into lookup_type, so no
// declaration may use it as one, and p.types is left alone here. The instance
// name and every member name are module-scope names (declarations.rs:592-619).
func (p *parser) parseBlockDeclaration(qualifiers typeQualifiers, name string, nameSpan ir.Span) (*astDecl, *frontError) {
	members, err := p.parseStructMembers()
	if err != nil {
		return nil, err
	}
	end, err := p.expectPunct("}")
	if err != nil {
		return nil, err
	}
	block := declBlock{Qualifiers: qualifiers, Name: name, Members: members, Span: subsumeSpan(nameSpan, end.span)}

	tok, err := p.bump()
	if err != nil {
		return nil, err
	}
	switch {
	case tok.isPunct(";"):
	case tok.kind == tokIdent:
		block.Instance = tok.text
		arrays, err := p.parseArraySpecifier()
		if err != nil {
			return nil, err
		}
		block.Arrays = arrays
		if _, err := p.expectPunct(";"); err != nil {
			return nil, err
		}
	default:
		return nil, invalidToken(tok, wantIdentifier, wantPunct(";"))
	}
	block.Span = subsumeSpan(block.Span, p.last)
	p.declareBlockNames(&block)
	return &astDecl{Kind: block, Span: block.Span}, nil
}

// declareBlockNames installs a block instance and its members as module-scope
// names. Rust adds the instance as a global variable and each named member as a
// lookup that selects it from the block (declarations.rs:592-619); lowering
// resolves the members from the block declaration itself, so all this pass
// records is that the names exist.
func (p *parser) declareBlockNames(block *declBlock) {
	if block.Instance != "" {
		sym := &glslSymbol{Kind: symGlobal, Name: block.Instance}
		p.symbols.declareGlobal(sym)
		block.InstanceSym = sym
	}
	for i := range block.Members {
		sym := &glslSymbol{Kind: symGlobal, Name: block.Members[i].Name}
		p.symbols.declareGlobal(sym)
		block.Members[i].Sym = sym
	}
}

// parsePrecisionDeclaration parses `precision highp float;`, Rust naga's
// precision arm of parse_declaration (parser/declarations.rs:487-529).
//
// Rust rejects a precision statement on a type that is neither a float nor a
// signed integer scalar (declarations.rs:512-523). That needs the type resolved,
// so the check is deferred to lowering.
func (p *parser) parsePrecisionDeclaration() (*astDecl, *frontError) {
	start, err := p.expectKeyword(tokPrecision)
	if err != nil {
		return nil, err
	}
	tok, err := p.bump()
	if err != nil {
		return nil, err
	}
	switch tok.kind {
	case tokHighP, tokMediumP, tokLowP:
	default:
		return nil, invalidToken(tok,
			wantKeyword(tokHighP), wantKeyword(tokMediumP), wantKeyword(tokLowP))
	}
	p.inPrecisionDecl = true
	typ, _, err := p.parseTypeNonVoid()
	p.inPrecisionDecl = false
	if err != nil {
		return nil, err
	}
	end, err := p.expectPunct(";")
	if err != nil {
		return nil, err
	}
	// A default float precision statement in scope satisfies the ES 3.00
	// fragment rule of checkFloatPrecision for the declarations that follow
	// (GLSL ES 3.00 §4.5.4). Any other statement (int, a sampler type) leaves
	// the float default unchanged.
	if typ.Builtin != nil {
		if scalar, ok := typeScalar(typ.Builtin.Inner); ok && scalar.Kind == ir.ScalarFloat && scalar.Width == 4 {
			p.defaultFloatPrecision = true
		}
	}
	decl := declPrecision{Precision: precisionOf(tok.kind), Typ: typ}
	return &astDecl{Kind: decl, Span: subsumeSpan(start.span, end.span)}, nil
}

// pushScope opens a scope: the symbol table and the ES default float precision
// state travel together, because a `precision` declaration applies to the
// scope it is written in (GLSL ES 3.00 §4.5.4; a local precision statement
// inside a block covers that block only, p_scope.frag).
func (p *parser) pushScope() {
	p.symbols.pushScope()
	p.precisionScopes = append(p.precisionScopes, p.defaultFloatPrecision)
}

// popScope closes the scope pushScope opened.
func (p *parser) popScope() {
	p.symbols.popScope()
	last := len(p.precisionScopes) - 1
	p.defaultFloatPrecision = p.precisionScopes[last]
	p.precisionScopes = p.precisionScopes[:last]
}

// parseParamList parses a function's parameter list, Rust naga's
// parse_function_args (parser/functions.rs:620-661); the caller has consumed the
// opening parenthesis and consumes the closing one.
//
// An unnamed parameter is legal GLSL. Rust loses the type of one that is
// followed directly by the closing parenthesis (the `_ => break` arm of
// functions.rs:652, gfx-rs/wgpu#5388) where this keeps it. Named parameters are
// declared in the scope the caller opened for the function; a duplicated name is
// the error the spec requires and Rust does not report, since it adds every
// parameter without checking (context.rs:401).
func (p *parser) parseParamList() ([]param, *frontError) {
	if p.bumpIf(kindIs(tokVoid)) != nil {
		// `f(void)` is the empty list; a `void` parameter is not a parameter.
		return nil, nil
	}
	var params []param
	for p.peekTypeName() || p.peekParameterQualifier() {
		prm, more, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		if prm.Name != "" {
			prm.Sym = p.declareLocal(prm.Name, symParam, prm.Span)
		}
		params = append(params, *prm)
		if !more {
			break
		}
	}
	return params, nil
}

// parseParam parses one parameter and reports whether another one follows, Rust
// naga's loop body of parse_function_args (parser/functions.rs:629-657).
func (p *parser) parseParam() (*param, bool, *frontError) {
	qualifier := paramIn
	sawPrecision := false
	for p.peekParameterQualifier() {
		tok, err := p.bump()
		if err != nil {
			return nil, false, err
		}
		switch tok.kind {
		case tokHighP, tokMediumP, tokLowP:
			// A precision qualifier on a parameter is accepted; it carries no
			// IR (design §15), but in ES 3.00 it is the declaration's own
			// precision, which exempts the parameter from the default float
			// precision rule.
			sawPrecision = true
		default:
			qualifier = parameterQualifierOf(tok.kind)
		}
	}
	typ, span, err := p.parseTypeNonVoidExemptingPrecision(sawPrecision)
	if err != nil {
		return nil, false, err
	}
	prm := &param{Qualifier: qualifier, Typ: typ, Span: span}

	next, err := p.expectPeek()
	if err != nil {
		return nil, false, err
	}
	switch {
	case next.isPunct(","):
		// An unnamed parameter: the type stands alone before the comma.
		if _, err := p.bump(); err != nil {
			return nil, false, err
		}
		return prm, true, nil
	case next.kind == tokIdent:
		nameTok, err := p.expectIdent()
		if err != nil {
			return nil, false, err
		}
		arrays, err := p.parseArraySpecifier()
		if err != nil {
			return nil, false, err
		}
		prm.Name, prm.Arrays = nameTok.text, arrays
		prm.Span = subsumeSpan(prm.Span, nameTok.span)
		if p.bumpIf(punct(",")) != nil {
			return prm, true, nil
		}
		return prm, false, nil
	default:
		// An unnamed parameter at the end of the list: the caller's closing
		// parenthesis follows.
		return prm, false, nil
	}
}

// parameterQualifierOf maps a parameter qualifier keyword to the direction it
// gives the parameter.
func parameterQualifierOf(k tokenKind) parameterQualifier {
	switch k {
	case tokOut:
		return paramOut
	case tokInOut:
		return paramInOut
	case tokConst:
		return paramConst
	default:
		return paramIn
	}
}

// parseInitializer parses an initializer, Rust naga's parse_initializer
// (parser/declarations.rs:97-162): one assignment expression, or a braced list
// whose items are initializers in turn.
//
// Rust resolves each item against the element or member type of the initialized
// type and emits a Compose from it (declarations.rs:111-149). That is lowering's
// work, so the nodes below carry the syntax and no type is consulted.
func (p *parser) parseInitializer() (*initializer, *frontError) {
	// The nodes below hold handles into this arena, so it is filled through one
	// exprs value and moved into the initializer once the tree is complete.
	arena := &exprs{}
	root, err := p.parseInitNode(arena)
	if err != nil {
		return nil, err
	}
	return &initializer{Arena: arena.Arena, Root: *root}, nil
}

// parseInitNode parses one node of an initializer tree: a braced list of nested
// nodes, or one assignment expression. Every expression of a braced list shares
// the initializer's arena, which is why the arena is threaded in.
func (p *parser) parseInitNode(arena *exprs) (*initNode, *frontError) {
	if open := p.bumpIf(punct("{")); open != nil {
		if err := p.enterInit(open.span); err != nil {
			return nil, err
		}
		defer p.leaveInit()
		node := &initNode{Span: open.span}
		for {
			item, err := p.parseInitNode(arena)
			if err != nil {
				return nil, err
			}
			node.Items = append(node.Items, *item)
			node.Span = subsumeSpan(node.Span, item.Span)

			tok, err := p.bump()
			if err != nil {
				return nil, err
			}
			switch {
			case tok.isPunct(","):
				closeTok := p.bumpIf(punct("}"))
				if closeTok == nil {
					continue
				}
				node.Span = subsumeSpan(node.Span, closeTok.span)
				return node, nil
			case tok.isPunct("}"):
				node.Span = subsumeSpan(node.Span, tok.span)
				return node, nil
			default:
				return nil, invalidToken(tok, wantPunct(","), wantPunct("}"))
			}
		}
	}
	expr, err := p.parseAssignment(arena)
	if err != nil {
		return nil, err
	}
	return &initNode{Expr: expr, Span: arena.Arena.get(expr).Span}, nil
}
