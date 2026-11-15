package errfmt

import (
	"math"
	"strconv"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestNarrow32 pins the checked narrowing every location field goes through:
// exact within the uint32 domain, reported (never truncated) outside it.
func TestNarrow32(t *testing.T) {
	type testCase struct {
		name   string
		in     int
		want   uint32
		wantOK bool
	}
	tests := []testCase{
		{"zero", 0, 0, true},
		{"one", 1, 1, true},
		{"negative", -1, 0, false},
	}
	if strconv.IntSize == 64 {
		// A 32-bit int cannot hold the boundary values of the uint32 domain.
		var max uint64 = math.MaxUint32
		tests = append(tests,
			testCase{"uint32 max", int(max), math.MaxUint32, true},
			// The first value outside the uint32 domain.
			testCase{"uint32 max + 1", int(max) + 1, 0, false},
		)
	}
	for _, tt := range tests {
		got, ok := narrow32(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("%s: narrow32(%d) = (%d, %t), want (%d, %t)", tt.name, tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

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
