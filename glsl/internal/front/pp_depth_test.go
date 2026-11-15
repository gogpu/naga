package front

// Regression tests for the macro-expansion memory defects found by the deep
// validation of 2026-11-15:
//   - the expansion depth was reset to 0 when a function-like macro's argument
//     was pre-expanded, so maxExpansionDepth never fired for F(F(F(...))): the
//     work grew quadratically in the input size (63 KB of source reached
//     7.1 GB of memory, 300 KB was an OOM kill);
//   - the token budget compared the length of one output slice, so tokens
//     produced into macro-argument slices were never charged at all.

import (
	"fmt"
	"strings"
	"testing"
)

// Source templates of the tests below: the tests vary only the nesting depth,
// so the surrounding text is named once.
const (
	ppNestedMacroTemplate = "#version 450\n#define F(x) x\nvoid main() { int v = %s; }\n"
	ppIfTemplate          = "#version 450\n#if %s\nint a = 1;\n#endif\nvoid main() {}\n"
)

// nestedCall returns depth nested calls of F around value: F(F(...F(value))).
func nestedCall(depth int, value string) string {
	return strings.Repeat("F(", depth) + value + strings.Repeat(")", depth)
}

// nestedParens returns depth nested parentheses around value.
func nestedParens(depth int, value string) string {
	return strings.Repeat("(", depth) + value + strings.Repeat(")", depth)
}

// TestMacroExpansionDepthThroughArguments pins the depth of an argument's
// pre-expansion: F(F(...F(1)...)) nested past maxExpansionDepth must be an
// error, not a memory blow-up. Before the fix the nested identity macros
// expanded at depth 0 at every level and produced no diagnostic.
func TestMacroExpansionDepthThroughArguments(t *testing.T) {
	source := fmt.Sprintf(ppNestedMacroTemplate, nestedCall(maxExpansionDepth+1, "1"))
	res := preprocess(source, nil)
	if len(res.Errors) == 0 {
		t.Fatalf("nested macro arguments %d deep produced no error", maxExpansionDepth+1)
	}
	if msg := res.Errors[0].Message; !strings.Contains(msg, "macro expansion nests deeper than") {
		t.Fatalf("error = %q, want the expansion-depth message", msg)
	}
}

// TestMacroExpansionDepthArgumentBoundary keeps the check honest: nesting that
// stays within maxExpansionDepth still expands.
func TestMacroExpansionDepthArgumentBoundary(t *testing.T) {
	source := fmt.Sprintf(ppNestedMacroTemplate, nestedCall(maxExpansionDepth/2, "1"))
	res := preprocess(source, nil)
	if len(res.Errors) != 0 {
		t.Fatalf("nesting %d deep was refused: %v", maxExpansionDepth/2, res.Errors)
	}
	// A number token carries its value in intValue, not text.
	found := false
	for _, tok := range res.Tokens {
		if tok.kind == ppInt && tok.intValue == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expanded tokens do not hold the value 1: %v", tokenStrings(res.Tokens))
	}
}

// TestPreprocessorExpressionDepth pins the #if/#elif expression recursion
// bound: a 500k-parenthesis expression used to end the process with a fatal
// stack overflow.
func TestPreprocessorExpressionDepth(t *testing.T) {
	source := fmt.Sprintf(ppIfTemplate, nestedParens(maxPreprocExprDepth+1, "1"))
	res := preprocess(source, nil)
	if len(res.Errors) == 0 {
		t.Fatalf("a #if expression %d levels deep produced no error", maxPreprocExprDepth+1)
	}
	if msg := res.Errors[0].Message; !strings.Contains(msg, "preprocessor expression nests deeper than") {
		t.Fatalf("error = %q, want the expression-depth message", msg)
	}
}

// TestPreprocessorExpressionDepthBoundary keeps the check honest: an
// expression within the bound still evaluates.
func TestPreprocessorExpressionDepthBoundary(t *testing.T) {
	source := fmt.Sprintf(ppIfTemplate, nestedParens(maxPreprocExprDepth-1, "1"))
	res := preprocess(source, nil)
	if len(res.Errors) != 0 {
		t.Fatalf("an expression %d levels deep was refused: %v", maxPreprocExprDepth-1, res.Errors)
	}
}

// TestTokenBudgetIsChargedAcrossSlices pins the budget's meaning: it counts
// every token the walk produces, whichever output slice it lands in. The old
// check compared len(*out), so a token written into a macro-argument slice was
// free until it reached the outer slice.
func TestTokenBudgetIsChargedAcrossSlices(t *testing.T) {
	d := &directiveState{budget: 2}
	var first, second []ppToken
	tok := ppToken{kind: ppInt, text: "1"}
	d.emit(&first, tok)
	d.emit(&first, tok)
	if d.budgetHit {
		t.Fatalf("budget hit before it was spent")
	}
	d.emit(&second, tok)
	if !d.budgetHit {
		t.Fatalf("a third token in another slice was not charged")
	}
	if len(second) != 0 {
		t.Fatalf("the over-budget token was written anyway")
	}
}

// TestTokenBudgetStopsDoublingMacros pins maxPPTokens with the design's macro
// bomb: `#define D(x) x x` nested doubles the token count per level, and the
// budget (four times the source tokens plus 64K) stops it long before the
// absolute cap.
func TestTokenBudgetStopsDoublingMacros(t *testing.T) {
	const depth = 18 // 2^18 tokens, far past 4*source+64K
	source := "#version 450\n#define D(x) x x\nvoid main() { int v = " +
		nestedCallD(depth, "1") + "; }\n"
	res := preprocess(source, nil)
	if len(res.Errors) == 0 {
		t.Fatalf("a doubling macro %d levels deep produced no error", depth)
	}
	if msg := res.Errors[0].Message; !strings.Contains(msg, "preprocessed token limit exceeded") {
		t.Fatalf("error = %q, want the token-limit message", msg)
	}
}

// nestedCallD returns depth nested calls of the doubling macro D.
func nestedCallD(depth int, value string) string {
	return strings.Repeat("D(", depth) + value + strings.Repeat(")", depth)
}
