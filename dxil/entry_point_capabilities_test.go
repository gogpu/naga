// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil_test

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil"
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
	}{
		{
			// vs needs SUBGROUP_VERTEX_STAGE, which DXIL lacks.
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
			// which DXIL lacks); a DXIL container holds only the globals its
			// entry point reaches, so cs's container has none.
			name: "binding array used by another entry point",
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
			if _, _, err := dxil.Compile(module, dxil.Options{EntryPoint: cs}); err != nil {
				t.Errorf("cs is refused: %v", err)
			}
			vs := &backend.EntryPointSelector{Stage: ir.StageVertex, Name: "vs"}
			if _, _, err := dxil.Compile(module, dxil.Options{EntryPoint: vs}); !errors.Is(err, backend.ErrUnsupported) {
				t.Errorf("vs: err = %v, want backend.ErrUnsupported", err)
			}
		})
	}
}
