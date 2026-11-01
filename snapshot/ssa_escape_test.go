package snapshot_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gogpu/naga"
	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
	"github.com/gogpu/naga/internal/ssa"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// TestSSAKindsStayInDXIL checks that the DXIL backend's SSA kinds (package
// ssa: Alias and Phi, which its mem2reg pass adds) never reach a public path.
// For every corpus shader it requires that
//   - parsing produces none, so valid.Validate never sees one;
//   - dxil.Compile, for every entry point, leaves the caller's module exactly
//     as it was, so none is left behind in it: the backend promotes locals in
//     its own copy;
//   - naga.Compact, after those compilations, finds none and leaves none.
func TestSSAKindsStayInDXIL(t *testing.T) {
	for _, sh := range loadCorpus(t) {
		t.Run(sh.name, func(t *testing.T) {
			module, err := parseWGSL(sh.source)
			if err != nil {
				t.Skipf("parse error (covered by the snapshot tests): %v", err)
			}
			requireNoSSAKinds(t, "after parsing", module)
			// Validation errors are other tests' business; this one only
			// checks the validator is never handed an SSA kind.
			_, _ = valid.Validate(module, valid.Options{Capabilities: valid.CapAll})

			before := module.Clone()
			for i := range module.EntryPoints {
				opts := dxil.DefaultOptions()
				opts.EntryPoint = &backend.EntryPointSelector{
					Stage: module.EntryPoints[i].Stage, Name: module.EntryPoints[i].Name,
				}
				// A compile error is fine here: an error must not leave
				// anything behind either.
				_, _ = dxilAtNeededModel(module, opts)
				requireNoSSAKinds(t, "after dxil.Compile of "+module.EntryPoints[i].Name, module)
			}
			if !reflect.DeepEqual(before, module) {
				t.Fatalf("dxil.Compile modified the caller's module")
			}

			naga.Compact(module, naga.KeepUnusedYes)
			requireNoSSAKinds(t, "after naga.Compact", module)
		})
	}
}

// requireNoSSAKinds fails the test for every SSA kind ssaKindsIn finds.
func requireNoSSAKinds(t *testing.T, when string, module *ir.Module) {
	t.Helper()
	for _, found := range ssaKindsIn(module) {
		t.Errorf("%s: %s, a DXIL-internal kind", when, found)
	}
}

// ssaKindsIn names every expression of module, in a function, an entry point
// or the global arena, whose kind is one of package ssa's.
func ssaKindsIn(module *ir.Module) []string {
	var found []string
	check := func(where string, exprs []ir.Expression) {
		for h := range exprs {
			switch exprs[h].Kind.(type) {
			case ssa.Alias, ssa.Phi:
				found = append(found, fmt.Sprintf("%s expression %d is %T", where, h, exprs[h].Kind))
			}
		}
	}
	check("global", module.GlobalExpressions)
	for i := range module.Functions {
		check(fmt.Sprintf("function %d", i), module.Functions[i].Expressions)
	}
	for i := range module.EntryPoints {
		check("entry point "+module.EntryPoints[i].Name, module.EntryPoints[i].Function.Expressions)
	}
	return found
}

// TestSSAKindsInDetects proves the scan of TestSSAKindsStayInDXIL finds an
// SSA kind in every arena it can sit in.
func TestSSAKindsInDetects(t *testing.T) {
	for _, place := range []string{"global", "function", "entry point"} {
		for _, kind := range []ir.ExpressionKind{ssa.Alias{}, ssa.Phi{}} {
			module := &ir.Module{
				Functions:   []ir.Function{{}},
				EntryPoints: []ir.EntryPoint{{Name: "main"}},
			}
			leak := []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralU32(0)}}, {Kind: kind}}
			switch place {
			case "global":
				module.GlobalExpressions = leak
			case "function":
				module.Functions[0].Expressions = leak
			case "entry point":
				module.EntryPoints[0].Function.Expressions = leak
			}
			if found := ssaKindsIn(module); len(found) != 1 {
				t.Errorf("%T in the %s arena: found %v, want one", kind, place, found)
			}
		}
	}
}
