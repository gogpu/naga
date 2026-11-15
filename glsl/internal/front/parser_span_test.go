package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// spanText returns the source text a span covers.
func spanText(src string, span ir.Span) string {
	return src[span.Start:span.End]
}

func TestParseDeclarationSpans(t *testing.T) {
	src := versioned("float a = 1.0;\nfloat b[3];\nfloat c = { 1.0, 2.0 };")
	file := wantParsed(t, src)
	if len(file.Decls) != 3 {
		t.Fatalf("%d declarations, want 3", len(file.Decls))
	}

	t.Run("whole declaration", func(t *testing.T) {
		const want = "float a = 1.0;"
		if got := spanText(src, file.Decls[0].Span); got != want {
			t.Errorf("declaration span covers %q, want %q", got, want)
		}
	})

	t.Run("declarator through initializer", func(t *testing.T) {
		list := wantDeclKind[declVarList](t, file.Decls[0])
		if len(list.Declarators) != 1 {
			t.Fatalf("%d declarators, want 1", len(list.Declarators))
		}
		const want = "a = 1.0"
		if got := spanText(src, list.Declarators[0].Span); got != want {
			t.Errorf("declarator span covers %q, want %q", got, want)
		}
	})

	t.Run("array specifier", func(t *testing.T) {
		list := wantDeclKind[declVarList](t, file.Decls[1])
		if len(list.Declarators) != 1 || len(list.Declarators[0].Arrays) != 1 {
			t.Fatalf("declarators = %+v, want one with one dimension", list.Declarators)
		}
		const want = "[3]"
		if got := spanText(src, list.Declarators[0].Arrays[0].Span); got != want {
			t.Errorf("array specifier span covers %q, want %q", got, want)
		}
	})

	t.Run("braced initializer", func(t *testing.T) {
		list := wantDeclKind[declVarList](t, file.Decls[2])
		if len(list.Declarators) != 1 || list.Declarators[0].Init == nil {
			t.Fatalf("declarators = %+v, want one initialized", list.Declarators)
		}
		const want = "{ 1.0, 2.0 }"
		if got := spanText(src, list.Declarators[0].Init.Root.Span); got != want {
			t.Errorf("initializer span covers %q, want %q", got, want)
		}
	})
}

func TestParseSwitchSpans(t *testing.T) {
	const body = "switch (x) { case 0: x = 1; break; case 1: break; default: break; }"
	src := versioned("void main() {\nint x = 0;\n" + body + "\n}")
	stmts := funcDecl(t, wantParsed(t, src), "main").Body
	if len(stmts) != 2 {
		t.Fatalf("%d statements, want a declaration and a switch", len(stmts))
	}
	sw := wantStmtKind[stmtSwitch](t, stmts[1])
	if got := spanText(src, stmts[1].Span); got != body {
		t.Errorf("switch span covers %q, want %q", got, body)
	}

	wantCases := []string{"case 0: x = 1; break;", "case 1: break;", "default: break;"}
	if len(sw.Cases) != len(wantCases) {
		t.Fatalf("%d cases, want %d", len(sw.Cases), len(wantCases))
	}
	for i, want := range wantCases {
		if got := spanText(src, sw.Cases[i].Span); got != want {
			t.Errorf("case %d span covers %q, want %q", i, got, want)
		}
	}
}

func TestParseLoopSpans(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "do-while",
			src:  versioned("void main() { do { break; } while (false); }"),
			want: "do { break; } while (false);",
		},
		{
			name: "for",
			src:  versioned("void main() { for (int i = 0; i < 2; i++) { } }"),
			want: "for (int i = 0; i < 2; i++) { }",
		},
		{
			name: "while",
			src:  versioned("void main() { while (true) { } }"),
			want: "while (true) { }",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := spanText(tt.src, onlyStatementFrom(t, tt.src, "main").Span); got != tt.want {
				t.Errorf("statement span covers %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseInvariantSpan(t *testing.T) {
	// A review fix: the span covers the invariant qualifier, not just the name.
	src := versioned("invariant gl_Position;")
	decl := onlyDecl(t, wantParsed(t, src))
	const want = "invariant gl_Position;"
	if got := spanText(src, decl.Span); got != want {
		t.Errorf("declaration span covers %q, want %q", got, want)
	}
}
