// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl_test

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestEntryPointCapabilities: with Options.EntryPoint set, Compile refuses
// only what the selected entry point's output needs. Another entry point
// that uses a capability the backend lacks does not block it, as in v0.21.2
// and Rust naga, whose writers never refuse by capability.
func TestEntryPointCapabilities(t *testing.T) {
	tests := []struct {
		name string
		src  string
		// csRefused: cs's output still needs the capability.
		csRefused bool
	}{
		{
			// vs needs SUBGROUP_VERTEX_STAGE, which HLSL lacks.
			name: "subgroup in another entry point",
			src: `enable subgroups;
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@vertex fn vs(@builtin(vertex_index) i: u32) -> @builtin(position) vec4<f32> {
    return vec4<f32>(f32(subgroupAdd(i)), 0.0, 0.0, 1.0);
}
@compute @workgroup_size(64) fn cs(@builtin(global_invocation_id) g: vec3<u32>) { out[g.x] = g.x; }`,
		},
		{
			// Only vs reads the uniform binding array (BUFFER_BINDING_ARRAY,
			// which HLSL lacks), but HLSL writes every global whichever entry
			// point is selected (Rust naga's back/hlsl/writer.rs), so cs's
			// output declares it too and is refused.
			name:      "binding array used by another entry point",
			csRefused: true,
			src: `enable wgpu_binding_array;
struct U { v: vec4<f32> }
@group(0) @binding(0) var<uniform> us: binding_array<U, 2>;
@group(0) @binding(1) var<storage, read_write> out: array<u32>;
@vertex fn vs() -> @builtin(position) vec4<f32> { return us[0].v; }
@compute @workgroup_size(64) fn cs(@builtin(global_invocation_id) g: vec3<u32>) { out[g.x] = g.x; }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module, _, err := wgsl.Parse(tt.src, wgsl.ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			cs := &backend.EntryPointSelector{Stage: ir.StageCompute, Name: "cs"}
			_, _, err = hlsl.Compile(module, hlsl.Options{EntryPoint: cs})
			switch {
			case tt.csRefused && !errors.Is(err, backend.ErrUnsupported):
				t.Errorf("cs: err = %v, want backend.ErrUnsupported", err)
			case !tt.csRefused && err != nil:
				t.Errorf("cs is refused: %v", err)
			}
			vs := &backend.EntryPointSelector{Stage: ir.StageVertex, Name: "vs"}
			if _, _, err := hlsl.Compile(module, hlsl.Options{EntryPoint: vs}); !errors.Is(err, backend.ErrUnsupported) {
				t.Errorf("vs: err = %v, want backend.ErrUnsupported", err)
			}
			if _, _, err := hlsl.Compile(module, hlsl.Options{}); !errors.Is(err, backend.ErrUnsupported) {
				t.Errorf("every entry point: err = %v, want backend.ErrUnsupported", err)
			}
		})
	}
}
