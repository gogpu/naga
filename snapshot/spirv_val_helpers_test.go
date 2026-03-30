package snapshot_test

import (
	"fmt"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/wgsl"
)

// parseWGSL tokenizes and parses WGSL source, returning the AST or an error.
func parseWGSL(source string) (*wgsl.Module, error) {
	lexer := wgsl.NewLexer(source)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, fmt.Errorf("parse error: tokenize: %w", err)
	}

	parser := wgsl.NewParser(tokens)
	module, err := parser.Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	return module, nil
}

// lowerToIR converts a WGSL AST to the IR module, returning an error on failure.
func lowerToIR(ast *wgsl.Module, source string) (*ir.Module, error) {
	module, err := wgsl.LowerWithSource(ast, source)
	if err != nil {
		return nil, fmt.Errorf("lowering error: %w", err)
	}
	return module, nil
}

// generateSPIRVBinary compiles an IR module to SPIR-V binary bytes.
func generateSPIRVBinary(module *ir.Module) ([]byte, error) {
	backend := spirv.NewBackend(spirv.DefaultOptions())
	spvBytes, err := backend.Compile(module)
	if err != nil {
		return nil, fmt.Errorf("SPIR-V generation error: %w", err)
	}
	return spvBytes, nil
}
