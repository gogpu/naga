package snapshot_test

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestNoDxilOnlyKindsAfterParse enforces the architectural invariant that
// DXIL-internal IR expression kinds — currently ir.ExprAlias and
// ir.ExprPhi — are NEVER produced by the WGSL parser alone. They must be
// synthesized exclusively by the DXIL backend's mem2reg pass
// (dxil/internal/passes/mem2reg) on a CloneModuleForOverrides clone, so
// other backends (MSL/GLSL/HLSL/SPIR-V) never observe them.
//
// If this test ever fails, the invariant is broken and the offending
// path needs to be repaired before shipping — text backends do not
// implement these kinds and would fail with "unsupported expression
// kind" at compile time, but a silent regression would only surface on
// the offending shader-backend combination.
//
// Reference: see the doc comments on ir.ExprAlias / ir.ExprPhi for the
// invariant statement and rationale (resembling Mesa NIR's pattern of
// pass-introduced node kinds that backends opt-in to handle).
func TestNoDxilOnlyKindsAfterParse(t *testing.T) {
	shaders := loadInputShaders(t, "testdata/in")
	if len(shaders) == 0 {
		t.Fatal("no input shaders found in testdata/in/")
	}

	for _, sh := range shaders {
		t.Run(sh.name, func(t *testing.T) {
			lexer := wgsl.NewLexer(sh.source)
			tokens, lexErr := lexer.Tokenize()
			if lexErr != nil {
				t.Skipf("lex error (covered by snapshot tests): %v", lexErr)
				return
			}
			parser := wgsl.NewParser(tokens)
			ast, parseErr := parser.Parse()
			if parseErr != nil {
				t.Skipf("parse error (covered by snapshot tests): %v", parseErr)
				return
			}
			module, lowerErr := wgsl.LowerWithSource(ast, sh.source)
			if lowerErr != nil {
				t.Skipf("lower error (covered by snapshot tests): %v", lowerErr)
				return
			}

			for fnIdx := range module.Functions {
				assertNoDxilOnlyKinds(t, &module.Functions[fnIdx], "function", fnIdx)
			}
			for epIdx := range module.EntryPoints {
				assertNoDxilOnlyKinds(t, &module.EntryPoints[epIdx].Function, "entry_point", epIdx)
			}
		})
	}
}

func assertNoDxilOnlyKinds(t *testing.T, fn *ir.Function, scope string, scopeIdx int) {
	t.Helper()
	for h := range fn.Expressions {
		switch fn.Expressions[h].Kind.(type) {
		case ir.ExprAlias:
			t.Errorf("%s[%d] expression %d: ir.ExprAlias produced by WGSL parser; "+
				"this kind is reserved for the DXIL mem2reg pass",
				scope, scopeIdx, h)
		case ir.ExprPhi:
			t.Errorf("%s[%d] expression %d: ir.ExprPhi produced by WGSL parser; "+
				"this kind is reserved for the DXIL mem2reg pass",
				scope, scopeIdx, h)
		}
	}
}
