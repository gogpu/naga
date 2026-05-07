package wgsl

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/lower"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// Module represents a parsed WGSL module (abstract syntax tree).
// It is produced by [Parser.Parse] and consumed by [Lower]/[LowerWithSource].
type Module struct {
	inner *parser.Module
}

// Lexer tokenizes WGSL source code into tokens.
type Lexer struct {
	inner *parser.Lexer
}

// Tokens holds the result of lexical analysis. Pass it to [NewParser].
type Tokens struct {
	inner []parser.Token
}

// Parser parses WGSL tokens into an AST.
type Parser struct {
	inner *parser.Parser
}

// ParseError represents a parsing error with location information.
type ParseError struct {
	Message string
	Line    int
	Column  int
}

// Error implements the error interface.
func (e ParseError) Error() string {
	return (&parser.ParseError{
		Message: e.Message,
		Token:   parser.Token{Line: e.Line, Column: e.Column},
	}).Error()
}

// Warning represents a compiler warning (not an error).
type Warning struct {
	Message string
	Span    Span
}

// LowerResult contains the result of lowering, including any warnings.
type LowerResult struct {
	Module   *ir.Module
	Warnings []Warning
}

// Span represents a source code location span.
type Span struct {
	Start  Position
	End    Position
	Source string
}

// Position represents a position in source code.
type Position struct {
	Line   int
	Column int
	Offset int
}

// NewLexer creates a new lexer for the given source.
func NewLexer(source string) *Lexer {
	return &Lexer{inner: parser.NewLexer(source)}
}

// Tokenize returns all tokens from the source.
func (l *Lexer) Tokenize() (*Tokens, error) {
	tokens, err := l.inner.Tokenize()
	if err != nil {
		return nil, err
	}
	return &Tokens{inner: tokens}, nil
}

// NewParser creates a new parser for the given tokens.
func NewParser(tokens *Tokens) *Parser {
	return &Parser{inner: parser.NewParser(tokens.inner)}
}

// Parse parses the tokens and returns a Module AST.
func (p *Parser) Parse() (*Module, error) {
	m, err := p.inner.Parse()
	if err != nil {
		return nil, err
	}
	return &Module{inner: m}, nil
}

// Lower converts a WGSL AST module to Naga IR.
func Lower(ast *Module) (*ir.Module, error) {
	return LowerWithSource(ast, "")
}

// LowerWithSource converts a WGSL AST module to Naga IR,
// keeping source for error messages.
func LowerWithSource(ast *Module, source string) (*ir.Module, error) {
	result, err := LowerWithWarnings(ast, source)
	if err != nil {
		return nil, err
	}
	return result.Module, nil
}

// LowerWithWarnings converts a WGSL AST module to Naga IR,
// returning warnings alongside the module.
func LowerWithWarnings(ast *Module, source string) (*LowerResult, error) {
	lr, err := lower.LowerWithWarnings(ast.inner, source)
	if err != nil {
		return nil, err
	}

	// Convert lower.Warning to wgsl.Warning
	warnings := make([]Warning, len(lr.Warnings))
	for i, w := range lr.Warnings {
		warnings[i] = Warning{
			Message: w.Message,
			Span: Span{
				Start: Position{
					Line:   w.Span.Start.Line,
					Column: w.Span.Start.Column,
					Offset: w.Span.Start.Offset,
				},
				End: Position{
					Line:   w.Span.End.Line,
					Column: w.Span.End.Column,
					Offset: w.Span.End.Offset,
				},
				Source: w.Span.Source,
			},
		}
	}

	return &LowerResult{
		Module:   lr.Module,
		Warnings: warnings,
	}, nil
}
