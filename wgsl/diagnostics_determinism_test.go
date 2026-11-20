package wgsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/wgsl"
)

// TestWarningsOrderIsDeterministic: a module's warnings come in a stable order
// across repeated compiles. The lowerer collected them by ranging over a Go
// map of local declarations, so the order changed between runs and a
// consumer's stderr-diff harness could not compare two runs of the same
// compiler (card P3-API-008: wgpu's REJ-MSG-DIFF flapped on it).
func TestWarningsOrderIsDeterministic(t *testing.T) {
	const src = `fn f() {
    var a: f32;
    var b: f32;
    var c: f32;
    var d: f32;
}
@compute @workgroup_size(1) fn main() { f(); }
`
	render := func(ws []frontend.Diagnostic) string {
		var b strings.Builder
		for _, w := range ws {
			b.WriteString(w.Message)
			b.WriteByte('\n')
		}
		return b.String()
	}

	var first string
	for i := 0; i < 200; i++ {
		_, info, err := wgsl.Parse(src, wgsl.ParseOptions{})
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if len(info.Warnings) != 4 {
			t.Fatalf("warnings = %d, want the four unused variables", len(info.Warnings))
		}
		got := render(info.Warnings)
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("warning order changed between compiles:\nfirst:\n%s\ngot:\n%s", first, got)
		}
	}
}
