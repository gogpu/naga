package front

import (
	"fmt"
	"strings"
	"testing"
)

// These tests pin the parser's recursion limits on untrusted input: a type
// nests through struct definitions and array dimensions, and a braced
// initializer nests through its items. Both are walked recursively by every
// later pass, so the parser refuses a depth beyond the limit instead of
// exhausting the stack (design §17).

// nestedStructSource builds a source whose struct definitions nest depth deep:
// each level's first member is defined inline, as in
// `struct S0 { struct S1 { float x; } m1; };`.
func nestedStructSource(depth int) string {
	var b strings.Builder
	b.WriteString("#version 460\n")
	for i := range depth {
		fmt.Fprintf(&b, "struct S%d { ", i)
	}
	b.WriteString("float x;")
	for i := depth - 1; i > 0; i-- {
		fmt.Fprintf(&b, " } m%d; ", i)
	}
	b.WriteString("};")
	return b.String()
}

// nestedInitializerSource builds a source with a depth-deep braced initializer
// list: float a = {{{…1.0…}}}.
func nestedInitializerSource(depth int) string {
	return "#version 460\nvoid main() { float a = " +
		strings.Repeat("{", depth) + "1.0" + strings.Repeat("}", depth) + "; }"
}

func TestTypeDepthLimit(t *testing.T) {
	// At the limit the declaration parses.
	ok := parseSource(t, nestedStructSource(maxTypeDepth))
	if len(ok.Errors) != 0 {
		t.Fatalf("depth %d errors = %v, want none", maxTypeDepth, ok.Errors)
	}
	// One level deeper is refused with the named limit's message.
	deep := parseSource(t, nestedStructSource(maxTypeDepth+1))
	if !hasError(deep, errTypeDepth) {
		t.Fatalf("depth %d errors = %v, want %q", maxTypeDepth+1, deep.Errors, errTypeDepth)
	}
	// A deep array nesting is a deep type too.
	arrays := parseSource(t, "#version 460\nfloat a"+strings.Repeat("[2]", maxTypeDepth+1)+";")
	if !hasError(arrays, errTypeDepth) {
		t.Fatalf("array depth %d errors = %v, want %q", maxTypeDepth+1, arrays.Errors, errTypeDepth)
	}
}

func TestInitializerDepthLimit(t *testing.T) {
	// At the limit the initializer parses.
	ok := parseSource(t, nestedInitializerSource(maxInitDepth))
	if len(ok.Errors) != 0 {
		t.Fatalf("depth %d errors = %v, want none", maxInitDepth, ok.Errors)
	}
	// Far past the limit the parser reports the error instead of recursing to
	// the stack limit, which would be a fatal, unrecoverable crash.
	deep := parseSource(t, nestedInitializerSource(5000))
	if !hasError(deep, errInitDepth) {
		t.Fatalf("depth 5000 errors = %v, want %q", deep.Errors, errInitDepth)
	}
}
