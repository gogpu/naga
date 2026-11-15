package front

// GLSL forbids recursion, and Rust naga rejects a call cycle at validation.
// Without a check the module lowers, and then the SPIR-V backend fails with an
// internal error while MSL/HLSL emit a call to a function declared later
// (deep validation, 2026-11-15, card P1-GLSLIN-005).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

const recursionPrelude = "#version 450\n"

// TestMutualRecursionIsRefused: f and g calling each other is one *ParseError
// at the declaration of the function the cycle closes on.
func TestMutualRecursionIsRefused(t *testing.T) {
	const src = recursionPrelude + `void f(int x);
void g(int x);
void f(int x) { g(x); }
void g(int x) { f(x); }
void main() { f(1); }
`
	_, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatalf("mutual recursion parsed without an error")
	}
	if msg := err.Errors[0].Message; !strings.Contains(msg, "Recursion is not allowed") {
		t.Fatalf("error = %q, want the recursion message", msg)
	}
}

// TestThreeFunctionCycleIsRefused: a cycle that does not involve the first
// call still closes somewhere and is reported.
func TestThreeFunctionCycleIsRefused(t *testing.T) {
	const src = recursionPrelude + `void a(int x);
void b(int x);
void c(int x);
void a(int x) { b(x); }
void b(int x) { c(x); }
void c(int x) { a(x); }
void main() { a(1); }
`
	_, _, err := Parse(src, Options{Stage: ir.StageFragment})
	if err == nil || !strings.Contains(err.Errors[0].Message, "Recursion is not allowed") {
		t.Fatalf("three-function cycle: err = %v, want the recursion message", err)
	}
}

// TestCallDiamondIsAccepted keeps the check honest: a function called from two
// places, and a chain f -> g -> h, are not cycles.
func TestCallDiamondIsAccepted(t *testing.T) {
	const src = recursionPrelude + `int h(int x) { return x + 1; }
int g(int x) { return h(x); }
int f(int x) { return g(x) + h(x); }
void main() { int v = f(1) + g(2); }
`
	if _, _, err := Parse(src, Options{Stage: ir.StageFragment}); err != nil {
		t.Fatalf("a legal call graph was refused: %v", err)
	}
}
