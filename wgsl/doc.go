// Package wgsl provides WGSL (WebGPU Shading Language) parsing.
//
// WGSL is the shader language for WebGPU, designed to be portable
// and map well to modern GPU APIs like Vulkan, Metal, and DX12.
//
// # Components
//
// The wgsl package consists of several components:
//
//   - Lexer: Tokenizes WGSL source code into tokens
//   - Parser: Parses tokens into an AST (Abstract Syntax Tree)
//   - AST: Type definitions for the abstract syntax tree
//
// # Usage
//
// To parse a WGSL shader:
//
//	source := `
//	@vertex
//	fn main() -> @builtin(position) vec4<f32> {
//	    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
//	}
//	`
//
//	lexer := wgsl.NewLexer(source)
//	tokens, err := lexer.Tokenize()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	parser := wgsl.NewParser(tokens)
//	module, err := parser.Parse()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # WGSL Specification
//
// This implementation follows the WGSL specification:
// https://www.w3.org/TR/WGSL/
//
// # Supported Features
//
//   - Full lexical analysis
//   - Type declarations (struct, alias)
//   - Function declarations
//   - Variable declarations (var, let, const)
//   - All standard types (scalars, vectors, matrices)
//   - Attributes (@vertex, @fragment, @compute, etc.)
//   - Control flow (if, for, while, loop, switch)
//   - All operators and expressions
package wgsl
