package errfmt

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestSourceLinesLocationMatchesSpan pins SourceLines.Location to exactly
// Span.Location's semantics on the edge cases (empty source, trailing newline,
// CRLF, a span past the end) and on a deterministic sweep of offsets: the
// index exists only to make repeated lookups cheaper, so any divergence from
// the reference semantics is a defect.
func TestSourceLinesLocationMatchesSpan(t *testing.T) {
	sources := []string{
		"",
		"a",
		"a\n",
		"a\nb",
		"a\r\nb\r\n",
		"\n\n\n",
		"fn main() {\n  let x = 1;\n}\n",
	}
	for _, source := range sources {
		lines := NewSourceLines(source)
		for start := 0; start <= len(source)+2; start++ {
			for _, end := range []uint32{0, uint32(start), uint32(start) + 3} {
				s := ir.Span{Start: uint32(start), End: end}
				want := s.Location(source)
				got := lines.Location(source, s)
				if got != want {
					t.Fatalf("source %q span %+v: SourceLines = %+v, Span = %+v", source, s, got, want)
				}
			}
		}
	}
}
