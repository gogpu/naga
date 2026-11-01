// Package entrypoint selects the entry point a backend writes, the meaning of
// every backend's Options.EntryPoint (backend.EntryPointSelector).
package entrypoint

import (
	"fmt"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// All is the index Index returns for a nil selector in an output that holds
// every entry point.
const All = -1

// Index returns the index in m.EntryPoints of the entry point sel names, or
// All when sel is nil. single is set for an output that holds one entry point
// (GLSL, a DXIL container): there nil is accepted only for a module with
// exactly one entry point, and Index returns 0. The errors wrap
// backend.ErrEntryPointNotFound.
//
// A text backend writes the selected entry point but names identifiers over
// the whole module, as Rust naga does, so the output for one entry point
// does not depend on which others the module holds.
func Index(m *ir.Module, sel *backend.EntryPointSelector, single bool) (int, error) {
	if sel == nil {
		if !single {
			return All, nil
		}
		if len(m.EntryPoints) != 1 {
			return 0, fmt.Errorf("%w: the module has %d entry points and Options.EntryPoint is nil; the output holds one",
				backend.ErrEntryPointNotFound, len(m.EntryPoints))
		}
		return 0, nil
	}
	for i := range m.EntryPoints {
		if ep := &m.EntryPoints[i]; ep.Name == sel.Name && ep.Stage == sel.Stage {
			return i, nil
		}
	}
	return 0, fmt.Errorf("%w: %s entry point %q", backend.ErrEntryPointNotFound, sel.Stage, sel.Name)
}

// Select returns m with only the entry point sel names, or m itself when sel
// is nil, with Index's rules for single. The copy shares everything with m
// but its EntryPoints. It suits a backend whose output does not depend on the
// names of other entry points (DXIL); a text backend uses Index.
func Select(m *ir.Module, sel *backend.EntryPointSelector, single bool) (*ir.Module, error) {
	i, err := Index(m, sel, single)
	if err != nil {
		return nil, err
	}
	if i == All || len(m.EntryPoints) == 1 {
		return m, nil
	}
	selected := *m
	selected.EntryPoints = []ir.EntryPoint{m.EntryPoints[i]}
	return &selected, nil
}
