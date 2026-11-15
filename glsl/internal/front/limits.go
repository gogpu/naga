package front

import "fmt"

// Security limits of the GLSL frontend (design §17). GLSL is untrusted input:
// every limit bounds work an attacker controls, and each one is a named
// constant with a test that asserts its error.
const (
	// maxExprDepth bounds the height of an expression tree, counted the way the
	// WGSL parser counts it (wgsl/internal/parser/parser.go): operator chains are
	// parsed in loops, so this bounds nesting (parentheses, unary runs, call
	// arguments), not chain length. The number and the reasoning are the WGSL
	// parser's: Tint refuses past 512, Rust naga has no limit and overflows its
	// stack, and 4096 sits above Rust's crash point while needing about 4 MiB of
	// stack through the validator and every backend.
	maxExprDepth = 4096

	// maxStatementDepth bounds how deep statements nest, one level per block,
	// if, loop or switch. The number and the reasoning are the WGSL parser's:
	// 1024 is above Rust naga's crash point (about 650 else-if links), needs
	// about 4 KiB of stack per level in the deepest pass, and keeps text-backend
	// output, which indents every level, from growing out of hand.
	maxStatementDepth = 1024

	// maxTypeDepth bounds how deeply types nest: struct definitions inside
	// struct definitions, and array dimensions around each other. GLSL's grammar
	// lets nested struct definitions recurse without end, Rust naga has no limit
	// (its CLI overflows its stack), and the parser, the lowering passes and the
	// validator all walk the type tree recursively. 64 is far beyond anything a
	// real shader nests (design §17).
	maxTypeDepth = 64

	// maxInitDepth bounds how deeply a braced initializer nests. An initializer
	// mirrors the type it initializes, so it is bounded by the same number as the
	// type: a deeper list cannot be valid GLSL and must not exhaust the stack on
	// the way to being refused.
	maxInitDepth = 64
)

var (
	// errExprDepth is Tint's expression-depth text, used by the WGSL parser too.
	errExprDepth = fmt.Sprintf("reached max expression depth of %d", maxExprDepth)
	// errStatementDepth is the statement-depth text of the WGSL parser.
	errStatementDepth = fmt.Sprintf("statement nesting depth / chaining length exceeds limit of %d", maxStatementDepth)
	// errTypeDepth is the type-nesting text of the maxTypeDepth limit.
	errTypeDepth = fmt.Sprintf("reached max type depth of %d", maxTypeDepth)
	// errInitDepth is the initializer-nesting text of the maxInitDepth limit.
	errInitDepth = fmt.Sprintf("reached max initializer depth of %d", maxInitDepth)
)
