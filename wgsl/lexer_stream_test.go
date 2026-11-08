package wgsl

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// TestEarlyErrorCostIndependentOfInputSize guards the streaming lexer
// (ADR-011): the cost of a parse refused by the recursion limit must not grow
// with the input. Before the streaming lexer the whole source was tokenized
// first (~40x its size), so the allocated bytes roughly doubled when the input
// doubled.
func TestEarlyErrorCostIndependentOfInputSize(t *testing.T) {
	sizes := []int{1 << 20, 2 << 20}
	allocs := make([]uint64, len(sizes))
	for i, n := range sizes {
		src := strings.Repeat("(", n)

		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if _, _, err := Parse(src, ParseOptions{}); err == nil {
			t.Fatalf("Parse of %d bytes of '(': expected an error", n)
		}
		runtime.ReadMemStats(&after)

		allocs[i] = after.TotalAlloc - before.TotalAlloc
		t.Logf("%d bytes of input: %d bytes allocated", n, allocs[i])
	}

	ratio := float64(allocs[1]) / float64(allocs[0])
	if ratio > 1.5 {
		t.Fatalf("allocated bytes grew %.2fx when the input doubled (%d -> %d); the lexer still tokenizes the whole input",
			ratio, allocs[0], allocs[1])
	}
}

// BenchmarkParseEarlyError measures the cost of a parse rejected by the
// expression depth limit. With an eager lexer the work is proportional to the
// whole input; with the streaming lexer it stops at the depth limit.
func BenchmarkParseEarlyError(b *testing.B) {
	for _, n := range []int{1 << 20, 16 << 20} {
		b.Run(fmt.Sprintf("%dMB", n>>20), func(b *testing.B) {
			src := strings.Repeat("(", n)
			b.ReportAllocs()
			b.SetBytes(int64(n))
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
		fmt.Fprintf(&sb, "fn f%d() -> i32 { return %d; }\n", i, i)
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
