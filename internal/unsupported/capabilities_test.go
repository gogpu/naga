// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package unsupported

import (
	"errors"
	"slices"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/internal/entrypoint"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
	"github.com/gogpu/naga/wgsl"
)

func TestCapabilities(t *testing.T) {
	f64 := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}}
	module := &ir.Module{Types: []ir.Type{f64}}

	if err := Capabilities("x", module, valid.CapFloat64); err != nil {
		t.Errorf("a supported capability is refused: %v", err)
	}
	if err := Capabilities("x", nil, 0); err != nil {
		t.Errorf("a nil module is refused: %v", err)
	}
	err := Capabilities("x", module, 0)
	if err == nil {
		t.Fatal("a module that needs FLOAT64 is accepted by a backend without it")
	}
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Errorf("error %q does not wrap backend.ErrUnsupported", err)
	}
	const want = "x: the module needs Capabilities(FLOAT64), which the x backend cannot write"
	if err.Error() != want {
		t.Errorf("error %q, want %q", err, want)
	}
}

// TestOutputCapabilities: a capability that only the entry points, functions
// or globals left out of the output need does not refuse it.
func TestOutputCapabilities(t *testing.T) {
	const src = `enable wgpu_binding_array;
struct U { v: vec4<f32> }
@group(0) @binding(0) var<uniform> us: binding_array<U, 2>;
@group(0) @binding(1) var<storage, read_write> a: array<f32>;
@group(0) @binding(2) var<storage, read_write> b: array<f32>;
fn h() { a[0] = f32(subgroupAdd(1u)); }
@compute @workgroup_size(1) fn cs1() { h(); a[1] = us[0].v.x; }
@compute @workgroup_size(1) fn cs2() { b[0] = 1.0; }`
	module, _, err := wgsl.Parse("enable subgroups;\n"+src, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// h needs SUBGROUP and only cs1 uses a and us (BUFFER_BINDING_ARRAY).
	const supported = valid.CapAll &^ (valid.CapSubgroup | valid.CapBufferBindingArray)
	cs2 := slices.IndexFunc(module.EntryPoints, func(ep ir.EntryPoint) bool { return ep.Name == "cs2" })
	for _, tt := range []struct {
		name    string
		out     Output
		refused bool
	}{
		{"whole module", Output{EntryPoint: entrypoint.All}, true},
		// Every function and global with cs2 alone (HLSL): h and us are written.
		{"cs2, every function and global", Output{EntryPoint: cs2}, true},
		// What SPIR-V, GLSL and DXIL write for cs2: neither h nor us.
		{"cs2, filtered", FilteredOutput(module, cs2), false},
		// h left out, us declared.
		{"cs2, h left out", Output{EntryPoint: cs2, Functions: FilteredOutput(module, cs2).Functions}, true},
		// us left out, h written.
		{"cs2, us left out", Output{EntryPoint: cs2, Globals: FilteredOutput(module, cs2).Globals}, true},
	} {
		err := OutputCapabilities("x", module, tt.out, supported)
		if got := errors.Is(err, backend.ErrUnsupported); got != tt.refused {
			t.Errorf("%s: err = %v, want refused = %v", tt.name, err, tt.refused)
		}
	}
	err = OutputCapabilities("x", module, FilteredOutput(module, 1-cs2), supported)
	const want = `x: entry point "cs1" needs Capabilities(BUFFER_BINDING_ARRAY | SUBGROUP), which the x backend cannot write`
	if err == nil || err.Error() != want {
		t.Errorf("cs1: err = %v, want %q", err, want)
	}
}
