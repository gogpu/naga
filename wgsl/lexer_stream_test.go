package wgsl

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// nestedParensSource returns about n bytes of parentheses inside a function
// body, so the parse fails at the expression depth limit. Top-level
// parentheses would be refused at the first token, before the depth limit.
func nestedParensSource(n int) string {
	return "fn f() { let x = " + strings.Repeat("(", n) + "; }"
}

// maxEarlyErrorAlloc caps what a parse refused at the expression depth limit
// may allocate, whatever the input size. It is an absolute bound, not a ratio:
// runtime.MemStats.TotalAlloc is process-wide and cumulative, and a few KB of
// background allocation inside the measured window flipped a 1.5x ratio on CI
// while both numbers stayed under 14 KB (P2-TEST-016, 2026-11-08). The
// regression this guards against is whole-input tokenizing, about 40x the
// source: 40 MB for 1 MB of input, so the cap sits an order of magnitude above
// the noise and two below the regression.
const maxEarlyErrorAlloc = 1 << 20

// TestEarlyErrorCostIndependentOfInputSize guards the streaming lexer
// (ADR-011): the allocated bytes of a parse refused by the expression depth
// limit must not grow with the input. Before the streaming lexer the whole
// source was tokenized first (~40x its size, and the slice growth on top), so
// the allocated bytes scaled with the input.
func TestEarlyErrorCostIndependentOfInputSize(t *testing.T) {
	sizes := []int{1 << 20, 2 << 20}
	for _, n := range sizes {
		src := nestedParensSource(n)

		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, _, err := Parse(src, ParseOptions{})
		runtime.ReadMemStats(&after)
		if err == nil {
			t.Fatalf("Parse of %d bytes of nested parentheses: expected an error", len(src))
		}
		var parseErr *ParseError
		if !errors.As(err, &parseErr) {
			t.Fatalf("Parse of %d bytes: got %T, want *ParseError", len(src), err)
		}
		if !refusedByRecursionLimit(parseErr) {
			t.Fatalf("Parse of %d bytes: got %q, want the recursion-limit refusal", len(src), err)
		}

		allocs := after.TotalAlloc - before.TotalAlloc
		t.Logf("%d bytes of input: %d bytes allocated", len(src), allocs)
		if allocs > maxEarlyErrorAlloc {
			t.Fatalf("Parse of %d bytes allocated %d bytes (cap %d): the lexer still tokenizes the whole input",
				len(src), allocs, maxEarlyErrorAlloc)
		}
	}
}

// refusedByRecursionLimit reports whether the parser refused the source at the
// expression depth limit, not earlier.
func refusedByRecursionLimit(e *ParseError) bool {
	if strings.Contains(e.Message, "recursion limit") {
		return true
	}
	for _, note := range e.Notes {
		if strings.Contains(note, "recursion limit") {
			return true
		}
	}
	return false
}

// BenchmarkParseEarlyError measures a parse rejected by the expression depth
// limit. With an eager lexer the allocation is proportional to the whole input;
// with the streaming lexer it is constant, and the remaining time is the linear
// scan of the parser's error recovery.
func BenchmarkParseEarlyError(b *testing.B) {
	for _, n := range []int{1 << 20, 16 << 20} {
		b.Run(fmt.Sprintf("%dMB", n>>20), func(b *testing.B) {
			src := nestedParensSource(n)
			b.ReportAllocs()
			b.SetBytes(int64(len(src)))
			b.ResetTimer()
			for range b.N {
				_, _, _ = Parse(src, ParseOptions{})
			}
		})
	}
}

// BenchmarkParseLargeValid measures a successful parse of a large module, where
// the streaming lexer still avoids materializing every token first.
func BenchmarkParseLargeValid(b *testing.B) {
	const target = 1 << 20
	var sb strings.Builder
	sb.Grow(target + 64)
	for i := 0; sb.Len() < target; i++ {
		// Not f%d: f16, f32 and f64 are WGSL type keywords.
		fmt.Fprintf(&sb, "fn shader%d() -> i32 { return %d; }\n", i, i)
	}
	src := sb.String()
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, _, err := Parse(src, ParseOptions{}); err != nil {
			b.Fatalf("Parse: %v", err)
		}
	}
}
