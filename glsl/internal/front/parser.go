package front

import "github.com/gogpu/naga/ir"

// parser parses the classified tokens of one GLSL source into its AST. It is a
// recursive-descent parser with one token of backtracking, Rust naga's
// ParsingContext (parser.rs:24-166).
//
// The parser is one of two passes: it builds HIR expressions and declaration
// and statement trees, and lowering (a later step) turns them into IR. Names
// are resolved during parsing because GLSL requires declaration before use, so
// the parser keeps the symbol and type tables a reference looks names up in.
type parser struct {
	tokens []glslToken
	pos    int
	// backtracked is a token already consumed and handed back to the stream.
	// At most one token can be handed back in a row, as in Rust naga
	// (parser.rs:26-27).
	backtracked *glslToken
	// last is the span of the last consumed token; the end-of-file error points
	// at it (Rust's last_meta).
	last ir.Span

	// version is the shader's #version number; it decides whether a stray
	// module-scope semicolon is legal, which Rust allows in 460 only
	// (parser/declarations.rs:80).
	version uint16

	// types holds the struct type names declared so far: Rust naga's
	// lookup_type. A struct must be declared before it is used.
	types map[string]bool
	// symbols resolves ordinary names: the root scope holds the module-scope
	// names, and every compound statement pushes a scope of its own.
	symbols *symbolTable

	// errors collects the semantic errors, which do not stop the parse; a
	// syntax error does and is returned separately.
	errors []frontError

	exprDepth int
	stmtDepth int
	typeDepth int
	initDepth int
}

// parse parses the tokens of one source into the AST of its external
// declarations. A syntax error stops the parse; it and the semantic errors
// collected on the way are in the returned file, in occurrence order (Rust
// naga returns the fatal error after the collected ones, mod.rs:207-219).
func parse(tokens []glslToken, version uint16) *parsedFile {
	p := &parser{
		tokens:  tokens,
		version: version,
		types:   map[string]bool{},
		symbols: newSymbolTable(),
	}
	file := &parsedFile{}
	for p.peek() != nil {
		decl, err := p.parseExternalDeclaration()
		if err != nil {
			file.Errors = append(append(file.Errors, p.errors...), *err)
			return file
		}
		if decl != nil {
			file.Decls = append(file.Decls, *decl)
		}
	}
	file.Errors = p.errors
	return file
}

// peek returns the next token without consuming it, or nil at the end of the
// stream.
func (p *parser) peek() *glslToken {
	if p.backtracked != nil {
		return p.backtracked
	}
	if p.pos >= len(p.tokens) {
		return nil
	}
	return &p.tokens[p.pos]
}

// bump consumes and returns the next token. At the end of the stream it returns
// Rust naga's end-of-file error, which points at the last consumed token
// (parser.rs:111-116).
func (p *parser) bump() (glslToken, *frontError) {
	if p.backtracked != nil {
		tok := *p.backtracked
		p.backtracked = nil
		p.last = tok.span
		return tok, nil
	}
	if p.pos >= len(p.tokens) {
		return glslToken{}, errEOF(p.last)
	}
	tok := p.tokens[p.pos]
	p.pos++
	p.last = tok.span
	return tok, nil
}

// bumpIf consumes and returns the next token when it matches, and returns nil
// otherwise.
func (p *parser) bumpIf(match func(glslToken) bool) *glslToken {
	tok := p.peek()
	if tok == nil || !match(*tok) {
		return nil
	}
	consumed, err := p.bump()
	if err != nil {
		// peek already saw the token, so bump cannot fail here.
		return nil
	}
	return &consumed
}

// expectPunct consumes the punctuation token written s.
func (p *parser) expectPunct(s string) (glslToken, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return tok, err
	}
	if !tok.isPunct(s) {
		return tok, invalidToken(tok, wantPunct(s))
	}
	return tok, nil
}

// expectKeyword consumes the keyword of kind k.
func (p *parser) expectKeyword(k tokenKind) (glslToken, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return tok, err
	}
	if tok.kind != k {
		return tok, invalidToken(tok, wantKeyword(k))
	}
	return tok, nil
}

// expectIdent consumes an identifier.
func (p *parser) expectIdent() (glslToken, *frontError) {
	tok, err := p.bump()
	if err != nil {
		return tok, err
	}
	if tok.kind != tokIdent {
		return tok, invalidToken(tok, wantIdentifier)
	}
	return tok, nil
}

// expectPeek returns the next token without consuming it, or the end-of-file
// error when the stream is exhausted.
func (p *parser) expectPeek() (*glslToken, *frontError) {
	if tok := p.peek(); tok != nil {
		return tok, nil
	}
	return nil, errEOF(p.last)
}

// backtrack hands a consumed token back to the stream. Handing back two tokens
// in a row is a bug in the parser and becomes an internal error, as in Rust
// naga (parser.rs:40-59).
func (p *parser) backtrack(tok glslToken) *frontError {
	if p.backtracked != nil {
		return errInternal("The parser tried to backtrack twice in a row", tok.span)
	}
	p.backtracked = &tok
	return nil
}

// peekTypeQualifier reports whether the next token starts a type qualifier
// list, Rust naga's peek_type_qualifier (parser/types.rs:150-167).
func (p *parser) peekTypeQualifier() bool {
	tok := p.peek()
	if tok == nil {
		return false
	}
	switch tok.kind {
	case tokInvariant, tokFlat, tokSmooth, tokNoPerspective, tokCentroid, tokSample,
		tokHighP, tokMediumP, tokLowP, tokConst, tokIn, tokOut, tokUniform, tokShared,
		tokBuffer, tokRestrict, tokReadOnly, tokWriteOnly, tokLayout:
		return true
	default:
		return false
	}
}

// peekTypeName reports whether the next token starts a type, Rust naga's
// peek_type_name (parser/types.rs:391-398): a builtin type name, void, struct,
// or a struct name the shader declared.
func (p *parser) peekTypeName() bool {
	tok := p.peek()
	if tok == nil {
		return false
	}
	switch tok.kind {
	case tokTypeName, tokVoid, tokStruct:
		return true
	case tokIdent:
		return p.types[tok.text]
	default:
		return false
	}
}

// peekParameterQualifier reports whether the next token is a parameter
// qualifier (parser/functions.rs:19-24).
func (p *parser) peekParameterQualifier() bool {
	tok := p.peek()
	if tok == nil {
		return false
	}
	switch tok.kind {
	case tokIn, tokOut, tokInOut, tokConst:
		return true
	default:
		return false
	}
}

// atPunct reports whether the next token is the punctuation token written s.
func (p *parser) atPunct(s string) bool {
	tok := p.peek()
	return tok != nil && tok.isPunct(s)
}

// atKind reports whether the next token is the keyword of kind k.
func (p *parser) atKind(k tokenKind) bool {
	tok := p.peek()
	return tok != nil && tok.kind == k
}

// enterExpr counts one level of expression nesting and reports crossing the
// depth limit. Every recursive expression call pairs it with leaveExpr; the
// limit keeps arbitrary input from exhausting the stack in the parser and in
// every pass that walks the tree afterwards.
func (p *parser) enterExpr() *frontError {
	p.exprDepth++
	if p.exprDepth > maxExprDepth {
		return errMessage(p.last, errExprDepth)
	}
	return nil
}

// leaveExpr leaves one level of expression nesting.
func (p *parser) leaveExpr() { p.exprDepth-- }

// enterStmt counts one level of statement nesting and reports crossing the
// depth limit.
func (p *parser) enterStmt() *frontError {
	p.stmtDepth++
	if p.stmtDepth > maxStatementDepth {
		return errMessage(p.last, errStatementDepth)
	}
	return nil
}

// leaveStmt leaves one level of statement nesting.
func (p *parser) leaveStmt() { p.stmtDepth-- }

// enterType counts one level of type nesting and reports crossing the depth
// limit. Only a struct definition inside a struct definition grows the depth:
// sibling members and array dimensions of one declarator are not nested.
func (p *parser) enterType(span ir.Span) *frontError {
	p.typeDepth++
	if p.typeDepth > maxTypeDepth {
		return errMessage(span, errTypeDepth)
	}
	return nil
}

// leaveType leaves one level of type nesting.
func (p *parser) leaveType() { p.typeDepth-- }

// enterInit counts one level of braced-initializer nesting and reports crossing
// the depth limit: an initializer tree is walked recursively by lowering, so an
// arbitrarily deep one must be refused while parsing, not crashed on later.
func (p *parser) enterInit(span ir.Span) *frontError {
	p.initDepth++
	if p.initDepth > maxInitDepth {
		return errMessage(span, errInitDepth)
	}
	return nil
}

// leaveInit leaves one level of initializer nesting.
func (p *parser) leaveInit() { p.initDepth-- }

// parseExternalDeclaration parses one declaration at module scope. Rust naga
// accepts a stray semicolon there in #version 460 only
// (parser/declarations.rs:69-95).
func (p *parser) parseExternalDeclaration() (*astDecl, *frontError) {
	decl, err := p.parseDeclaration(true)
	if err != nil {
		return nil, err
	}
	if decl != nil {
		return decl, nil
	}
	tok, err := p.bump()
	if err != nil {
		return nil, err
	}
	if tok.isPunct(";") && p.version == 460 {
		return nil, nil
	}
	if p.version == 460 {
		return nil, invalidToken(tok, wantPunct(";"), wantEOF)
	}
	return nil, invalidToken(tok, wantEOF)
}
