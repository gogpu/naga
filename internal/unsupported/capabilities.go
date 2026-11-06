// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package unsupported

import (
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// Capabilities refuses a module that needs an optional capability outside
// supported, the SupportedCapabilities of the backend named by name. The
// error wraps backend.ErrUnsupported and names the capabilities as Rust
// prints them ("Capabilities(SHADER_FLOAT32_ATOMIC)"), the text wgpu matches
// a missing-capability error on. It returns nil when the module needs
// nothing the backend lacks.
//
// It checks the whole module: the call of a backend that writes every entry
// point. A backend writing one entry point calls OutputCapabilities.
func Capabilities(name string, module *ir.Module, supported valid.Capabilities) error {
	return OutputCapabilities(name, module, Output{EntryPoint: entrypoint.All}, supported)
}

// Output is the part of a module a backend writes, for OutputCapabilities.
// The module's types and constant expressions are always part of it: every
// backend writes them whichever entry point it selects, and valid checks them
// for the whole module.
type Output struct {
	// EntryPoint is the index of the one entry point written, or
	// entrypoint.All.
	EntryPoint int
	// Functions[h] is set when function h is written; nil writes every
	// function.
	Functions []bool
	// Globals[h] is set when global h is declared; nil declares every global.
	Globals []bool
}

// FilteredOutput is the Output of a backend that writes entry point ep and
// what entrypoint.NewFilter keeps for it: the globals it uses and the
// functions that can serve it (SPIR-V with an entry point selected, GLSL,
// DXIL). ep is an index in module.EntryPoints, or entrypoint.All for the
// whole module.
func FilteredOutput(module *ir.Module, ep int) Output {
	if ep == entrypoint.All {
		return Output{EntryPoint: entrypoint.All}
	}
	f := entrypoint.NewFilter(module, ep)
	return Output{EntryPoint: ep, Functions: f.Functions, Globals: f.Globals}
}

// OutputCapabilities is Capabilities for the part of module out describes:
// a capability only the entry points, functions or globals the backend
// leaves out need does not refuse the module, since nothing of them reaches
// the output. With an entry point selected the error names it.
func OutputCapabilities(name string, module *ir.Module, out Output, supported valid.Capabilities) error {
	if module == nil {
		return nil
	}
	missing := valid.MissingCapabilities(outputView(module, out), supported)
	if missing == 0 {
		return nil
	}
	if out.EntryPoint != entrypoint.All {
		return Errorf("%s: entry point %q needs %s, which the %s backend cannot write",
			name, module.EntryPoints[out.EntryPoint].Name, missing, name)
	}
	return Errorf("%s: the module needs %s, which the %s backend cannot write", name, missing, name)
}

// outputView returns module as valid.MissingCapabilities should see it for
// out. The view keeps every handle valid: a function left out keeps its slot
// with an empty body, and a global left out keeps its slot and type as a
// private variable, which needs no capability of its own (a resource's
// binding-array and storage-format capabilities are the ones that depend on
// its address space); its type is checked with the module's types. module
// itself is not changed.
func outputView(module *ir.Module, out Output) *ir.Module {
	view := *module
	if i := out.EntryPoint; i != entrypoint.All {
		view.EntryPoints = module.EntryPoints[i : i+1 : i+1]
	}
	if out.Functions != nil {
		view.Functions = make([]ir.Function, len(module.Functions))
		for h := range module.Functions {
			if h < len(out.Functions) && out.Functions[h] {
				view.Functions[h] = module.Functions[h]
			}
		}
	}
	if out.Globals != nil {
		view.GlobalVariables = make([]ir.GlobalVariable, len(module.GlobalVariables))
		for h := range module.GlobalVariables {
			if h < len(out.Globals) && out.Globals[h] {
				view.GlobalVariables[h] = module.GlobalVariables[h]
				continue
			}
			view.GlobalVariables[h] = ir.GlobalVariable{
				Name:  module.GlobalVariables[h].Name,
				Space: ir.SpacePrivate,
				Type:  module.GlobalVariables[h].Type,
			}
		}
	}
	return &view
}
