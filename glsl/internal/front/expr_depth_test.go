package front

// The lowering recurses per expression node. The parser bounds nesting but
// parses operator chains in loops, so a long chain reached the lowering as a
// deep tree and a 2M-term chain ended the process with Go's fatal stack
// overflow (deep validation, 2026-11-15). lowerInner now counts its own depth
// against maxExprDepth, with the parser's limit and message.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

const glslChainTemplate = "#version 450\nvoid main() { float v = 0.0%s; }\n"

// TestLoweringChainDepth: a chain of maxExprDepth+1 operators is one error, not
// a stack overflow. Before the bound the chain lowered without a diagnostic.
func TestLoweringChainDepth(t *testing.T) {
	source := fmt.Sprintf(glslChainTemplate, strings.Repeat(" + 1.0", maxExprDepth+1))
	_, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatalf("a chain of %d operators parsed without an error", maxExprDepth+1)
	}
	if got := err.Errors[0].Message; got != errExprDepth {
		t.Fatalf("error = %q, want %q", got, errExprDepth)
	}
}

// TestLoweringChainDepthBoundary keeps the check honest: a chain within the
// bound still lowers.
func TestLoweringChainDepthBoundary(t *testing.T) {
	source := fmt.Sprintf(glslChainTemplate, strings.Repeat(" + 1.0", maxExprDepth-1))
	if _, _, err := Parse(source, Options{Stage: ir.StageFragment}); err != nil {
		t.Fatalf("a chain of %d operators was refused: %v", maxExprDepth-1, err)
	}
}

// TestLoweringHugeChainFailsFast: the crasher shape of the deep validation is a
// chain far past the limit; it must produce the same error immediately, not
// walk millions of nodes first.
func TestLoweringHugeChainFailsFast(t *testing.T) {
	source := fmt.Sprintf(glslChainTemplate, strings.Repeat(" + 1.0", 100_000))
	_, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err == nil {
		t.Fatalf("a chain of 100000 operators parsed without an error")
	}
	if got := err.Errors[0].Message; got != errExprDepth {
		t.Fatalf("error = %q, want %q", got, errExprDepth)
	}
}
