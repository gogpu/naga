// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/module"
)

// resourceKindsCS uses one resource of every class and the common kinds: a
// read-write and a read-only storage buffer (raw buffers), a uniform buffer,
// a sampled, a depth and a storage texture, samplers of both kinds, a
// binding array indexed at run time, and num_workgroups (a synthetic
// constant buffer).
const resourceKindsCS = `
enable wgpu_binding_array;
struct U { a: vec4<f32>, b: u32 }
@group(0) @binding(0) var<storage, read_write> rw: array<u32>;
@group(0) @binding(1) var<storage, read> ro: array<u32>;
@group(0) @binding(2) var<uniform> u: U;
@group(0) @binding(3) var tex: texture_2d<f32>;
@group(0) @binding(4) var depth: texture_depth_2d;
@group(0) @binding(5) var img: texture_storage_2d<r32uint, read_write>;
@group(0) @binding(6) var samp: sampler;
@group(0) @binding(7) var cmp: sampler_comparison;
@group(0) @binding(8) var arr: binding_array<texture_2d<f32>, 4>;

@compute @workgroup_size(1)
fn main(@builtin(global_invocation_id) id: vec3<u32>, @builtin(num_workgroups) n: vec3<u32>) {
    let c = textureSampleLevel(tex, samp, vec2(0.5), 0.0)
          + textureLoad(arr[id.x & 3u], vec2(0), 0);
    let d = textureSampleCompareLevel(depth, cmp, vec2(0.5), 0.5);
    let i = textureLoad(img, vec2(0)).x;
    textureStore(img, vec2(1), vec4(i + 1u));
    rw[id.x] = ro[id.x] + u.b + i + n.x + u32(c.x + d + u.a.y);
}
`

// TestHandlesFromBindingAtSM66 (P1-DXIL-090): from shader model 6.6 a
// resource handle is created with dx.op.createHandleFromBinding and typed
// with dx.op.annotateHandle; the validator rejects dx.op.createHandle there
// ("opcode 'CreateHandle' should only be used in 'Shader model 6.5 and
// below'"). DXC emits the pair at SM 6.6 (DxilOperations.cpp
// CreateHandleFromBinding 217, AnnotateHandle 216; the properties as
// resource_helper::loadPropsFromResourceBase builds them,
// DxilResourceProperties.cpp:139-213). Below 6.6 createHandle stays.
func TestHandlesFromBindingAtSM66(t *testing.T) {
	m := lowerSource(t, resourceKindsCS)
	for minor := uint8(0); minor <= 8; minor++ {
		t.Run(fmt.Sprintf("6.%d", minor), func(t *testing.T) {
			opts := DefaultOptions()
			opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: minor}
			opts.SpecialConstantsBinding = &BindTarget{Register: 1, Space: 2}
			blob, _, err := Compile(m, opts)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Errorf("Validate: %v", err)
			}
		})
	}

	// The emitted calls: createHandle below 6.6, the pair from 6.6.
	for _, c := range []struct {
		minor      uint8
		want, none string
	}{
		{5, "dx.op.createHandle", "dx.op.createHandleFromBinding"},
		{6, "dx.op.createHandleFromBinding", "dx.op.createHandle"},
	} {
		opts := DefaultOptions()
		opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: c.minor}
		opts.SpecialConstantsBinding = &BindTarget{Register: 1, Space: 2}
		mod, _ := emitMainModuleOpts(t, resourceKindsCS, opts)
		calls := calledFunctions(mod)
		if calls[c.want] == 0 {
			t.Errorf("SM 6.%d: no %s call", c.minor, c.want)
		}
		if calls[c.none] != 0 {
			t.Errorf("SM 6.%d: %d %s calls", c.minor, calls[c.none], c.none)
		}
		if c.minor >= 6 && calls["dx.op.annotateHandle"] != calls["dx.op.createHandleFromBinding"] {
			t.Errorf("SM 6.%d: %d annotateHandle for %d createHandleFromBinding",
				c.minor, calls["dx.op.annotateHandle"], calls["dx.op.createHandleFromBinding"])
		}
	}
}

// TestSamplerHeapHandlesAtSM66: the sampler heap's dynamically indexed
// handles take the same path.
func TestSamplerHeapHandlesAtSM66(t *testing.T) {
	m := lowerSource(t, `
@group(0) @binding(0) var t: texture_depth_2d;
@group(0) @binding(1) var s: sampler;
@group(0) @binding(2) var sc: sampler_comparison;
@fragment fn main() -> @location(0) vec4<f32> {
	return vec4(textureSample(t, s, vec2(0.5)), textureSampleCompare(t, sc, vec2(0.5), 0.5), 0.0, 1.0);
}
`)
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 6}
	opts.BindingMap = BindingMap{
		{Group: 0, Binding: 0}: {Register: 0},
		{Group: 0, Binding: 1}: {Space: 255, Register: 0},
		{Group: 0, Binding: 2}: {Space: 255, Register: 1},
	}
	opts.SamplerHeapTargets = &SamplerHeapBindTargets{
		StandardSamplers:   BindTarget{Register: 0},
		ComparisonSamplers: BindTarget{Register: 2048},
	}
	blob, _, err := Compile(m, opts)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if err := Validate(blob, ValidateFull); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// calledFunctions counts the calls to each function in the module.
func calledFunctions(mod *module.Module) map[string]int {
	calls := make(map[string]int)
	for _, fn := range mod.Functions {
		for _, bb := range fn.BasicBlocks {
			for _, in := range bb.Instructions {
				if in.Kind == module.InstrCall && in.CalledFunc != nil {
					calls[in.CalledFunc.Name]++
				}
			}
		}
	}
	return calls
}

// TestAnnotateHandleProperties pins the ResourceProperties of each resource
// kind to what DXC writes for the same HLSL at cs_6_6 (Texture2D<float4>
// {2, 1033}, RWTexture2D<float4> {4098, 1033}, ByteAddressBuffer {11, 0},
// RWByteAddressBuffer {4107, 0}, SamplerState {14, 0}, cbuffer {13, size}):
// dword 0 is the kind with IsUAV at bit 12 and a comparison sampler's bit
// 15, dword 1 a texture's component type and count.
func TestAnnotateHandleProperties(t *testing.T) {
	opts := DefaultOptions()
	opts.ShaderModel = backend.ShaderModel{Major: 6, Minor: 6}
	opts.SpecialConstantsBinding = &BindTarget{Register: 1, Space: 2}
	mod, _ := emitMainModuleOpts(t, resourceKindsCS, opts)

	aggregates := make(map[int]*module.Constant)
	for _, c := range mod.Constants {
		if c.IsAggregate {
			aggregates[c.ValueID] = c
		}
	}
	got := make(map[[2]int64]int)
	for _, fn := range mod.Functions {
		for _, bb := range fn.BasicBlocks {
			for _, in := range bb.Instructions {
				if in.Kind != module.InstrCall || in.CalledFunc == nil || in.CalledFunc.Name != "dx.op.annotateHandle" {
					continue
				}
				c := aggregates[in.Operands[len(in.Operands)-1]]
				if c == nil || len(c.Elements) != 2 {
					t.Fatalf("annotateHandle without a ResourceProperties constant")
				}
				got[[2]int64{c.Elements[0].IntValue, c.Elements[1].IntValue}]++
			}
		}
	}
	want := [][2]int64{
		{4107, 0},        // rw: RWByteAddressBuffer
		{11, 0},          // ro: ByteAddressBuffer
		{2, 1033},        // tex and arr: Texture2D<float4>
		{2, 9 | 1<<8},    // depth: Texture2D<float>
		{4098, 5 | 1<<8}, // img: RWTexture2D<uint>
		{14, 0},          // samp
		{14 | 1<<15, 0},  // cmp: comparison sampler
		{13, 12},         // num_workgroups: three u32 in a constant buffer
	}
	for _, w := range want {
		if got[w] == 0 {
			t.Errorf("no annotateHandle with properties {%d, %d}; got %v", w[0], w[1], got)
		}
	}
}
