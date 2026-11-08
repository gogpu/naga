package wgsl

import (
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

// TestEarlyErrorCostIndependentOfInputSize guards the streaming lexer
// (ADR-011): the allocated bytes of a parse refused by the expression depth
// limit must not grow with the input. Before the streaming lexer the whole
// source was tokenized first (~40x its size, and the slice growth on top), so
// the allocated bytes scaled with the input.
func TestEarlyErrorCostIndependentOfInputSize(t *testing.T) {
	sizes := []int{1 << 20, 2 << 20}
	allocs := make([]uint64, len(sizes))
	for i, n := range sizes {
		src := nestedParensSource(n)

		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if _, _, err := Parse(src, ParseOptions{}); err == nil {
			t.Fatalf("Parse of %d bytes of nested parentheses: expected an error", len(src))
		}
		runtime.ReadMemStats(&after)

		allocs[i] = after.TotalAlloc - before.TotalAlloc
		t.Logf("%d bytes of input: %d bytes allocated", len(src), allocs[i])
	}

	ratio := float64(allocs[1]) / float64(allocs[0])
	if ratio > 1.5 {
		t.Fatalf("allocated bytes grew %.2fx when the input doubled (%d -> %d); the lexer still tokenizes the whole input",
			ratio, allocs[0], allocs[1])
	}
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
