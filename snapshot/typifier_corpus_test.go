package snapshot_test

import (
	"testing"

	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestTypifierCorpus: every function and entry point the WGSL frontend makes
// of the corpus types from its IR alone, which is what each backend's
// Compile does before it writes the module; and so does the global
// expression arena.
func TestTypifierCorpus(t *testing.T) {
	mods, exprs := 0, 0
	for _, s := range loadCorpus(t) {
		m, _, err := wgsl.Parse(s.source, wgsl.ParseOptions{})
		if err != nil {
			continue
		}
		mods++
		if _, err := typifier.ForModule(m); err != nil {
			t.Errorf("%s: %v", s.name, err)
		}
		if _, err := typifier.Function(m, &ir.Function{Expressions: m.GlobalExpressions}); err != nil {
			t.Errorf("%s: global expressions: %v", s.name, err)
		}
		exprs += len(m.GlobalExpressions)
		for i := range m.Functions {
			exprs += len(m.Functions[i].Expressions)
		}
		for i := range m.EntryPoints {
			exprs += len(m.EntryPoints[i].Function.Expressions)
		}
	}
	t.Logf("modules %d, expressions %d", mods, exprs)
}
