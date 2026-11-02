package dxil

import (
	"slices"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/dxil/internal/module"
)

// A binding that restricts indexing, a root CBV, which D3D12 doesn't bounds
// check, has every run-time index of its loads clamped to its array or
// matrix, as the HLSL writer clamps it with min(uint(i), n - 1). Without
// RestrictIndexing, or under the Buffer policy Unchecked, the index is used as
// it is.
func TestRestrictIndexingClampsConstantBufferIndices(t *testing.T) {
	src := `
struct L { color: vec4<f32>, m: mat4x4<f32> }
struct U { a: vec4<f32>, lights: array<L, 8> }
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var<storage, read_write> out: array<vec4<f32>>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
	out[0] = u.lights[i].color + u.lights[i].m[i & 3u];
}
`
	for _, tc := range []struct {
		restrict bool
		buffer   backend.BoundsCheckPolicy
		clamped  bool
	}{
		{false, backend.BoundsCheckRestrict, false},
		{true, backend.BoundsCheckRestrict, true},
		{true, backend.BoundsCheckUnchecked, false},
	} {
		opts := DefaultOptions()
		opts.BoundsCheckPolicies.Buffer = tc.buffer
		opts.BindingMap = BindingMap{{Group: 0, Binding: 0}: {Register: 2, RestrictIndexing: tc.restrict}}
		mod, fn := emitMainModuleOpts(t, src, opts)
		limits := uminLimits(mod, fn)
		slices.Sort(limits)
		var want []int64
		if tc.clamped {
			want = []int64{3, 7, 7}
		}
		if !slices.Equal(limits, want) {
			t.Errorf("BindTarget.RestrictIndexing %v, Buffer %v: indices clamped to %v, want %v", tc.restrict, tc.buffer, limits, want)
		}
	}
}

// The Index policy Restrict clamps a run-time index into a function, private or
// workgroup array, nested ones included, or into a vector to its last
// element, as the HLSL writer does, and leaves an index into a storage
// buffer, which D3D12 bounds checks, and a constant index in bounds as they
// are. Without it every index is used as it is.
func TestRestrictIndexingClampsArrayIndices(t *testing.T) {
	src := `
var<private> p: array<u32, 6>;
var<workgroup> w: array<u32, 8>;
@group(0) @binding(0) var<storage, read_write> data: array<u32>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
	var a: array<u32, 4>;
	var m: array<array<u32, 3>, 5>;
	var v = vec4<u32>(data[2]);
	a[i] = 1u;
	a[2] = 2u;
	data[i] = a[data[0]] + p[i] + w[i] + m[i][data[1]] + v[i];
}
`
	for _, restrict := range []bool{false, true} {
		opts := DefaultOptions()
		if !restrict {
			opts.BoundsCheckPolicies.Index = backend.BoundsCheckUnchecked
		}
		// Only the shader's own indices: zeroing w loops over its elements
		// with an index of its own, clamped too.
		opts.ZeroInitializeWorkgroupMemory = backend.ZeroInitWorkgroupNone
		mod, fn := emitMainModuleOpts(t, src, opts)
		limits := uminLimits(mod, fn)
		slices.Sort(limits)
		var want []int64
		if restrict {
			want = []int64{2, 3, 3, 3, 4, 5, 7}
		}
		if !slices.Equal(limits, want) {
			t.Errorf("Index Restrict %v: indices clamped to %v, want %v", restrict, limits, want)
		}
	}
}

// uminLimits returns the constant upper bound of every dx.op UMin in fn.
func uminLimits(mod *module.Module, fn *module.Function) []int64 {
	constants := map[int]int64{}
	for _, c := range mod.Constants {
		if c.ConstType != nil && c.ConstType.Kind == module.TypeInteger {
			constants[c.ValueID] = c.IntValue
		}
	}
	var limits []int64
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind != module.InstrCall || in.CalledFunc == nil || in.CalledFunc.Name != "dx.op.binary.i32" || len(in.Operands) != 3 {
				continue
			}
			if op, ok := constants[in.Operands[0]]; ok && op == int64(emit.OpUMin) {
				if limit, ok := constants[in.Operands[2]]; ok {
					limits = append(limits, limit)
				}
			}
		}
	}
	return limits
}

// The BindingArray policy Restrict clamps a run-time index into a binding
// array to its size, or to BindTarget.BindingArraySize for an unbounded one;
// an unbounded array without it is an error, and Unchecked uses the index as
// it is. Rust naga's HLSL writer never checks binding arrays; the DXIL
// backend follows the HLSL backend here.
func TestRestrictIndexingClampsBindingArrayIndices(t *testing.T) {
	const body = `
@group(0) @binding(1) var<storage, read_write> out: array<vec4<f32>>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
	out[i] = textureLoad(textures[i], vec2<i32>(0), 0);
}
`
	sized := "enable wgpu_binding_array;\n@group(0) @binding(0) var textures: binding_array<texture_2d<f32>, 8>;" + body
	unbounded := "enable wgpu_binding_array;\n@group(0) @binding(0) var textures: binding_array<texture_2d<f32>>;" + body

	opts := DefaultOptions()
	opts.BoundsCheckPolicies.Index = backend.BoundsCheckUnchecked
	mod, fn := emitMainModuleOpts(t, sized, opts)
	if limits := uminLimits(mod, fn); !slices.Equal(limits, []int64{7}) {
		t.Errorf("sized binding array: indices clamped to %v, want [7]", limits)
	}

	size := uint32(16)
	opts.BindingMap = BindingMap{{Group: 0, Binding: 0}: {BindingArraySize: &size}}
	mod, fn = emitMainModuleOpts(t, unbounded, opts)
	if limits := uminLimits(mod, fn); !slices.Equal(limits, []int64{15}) {
		t.Errorf("unbounded binding array with BindingArraySize 16: indices clamped to %v, want [15]", limits)
	}

	opts.BindingMap = nil
	irMod, err := prepareModule(lowerSource(t, unbounded))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lowerForEmit(irMod, opts); err == nil || !strings.Contains(err.Error(), "BindingArraySize") {
		t.Errorf("unbounded binding array without a size: err = %v, want BindingArraySize named", err)
	}

	opts.BoundsCheckPolicies.BindingArray = backend.BoundsCheckUnchecked
	mod, fn = emitMainModuleOpts(t, sized, opts)
	if limits := uminLimits(mod, fn); len(limits) != 0 {
		t.Errorf("Unchecked: indices clamped to %v", limits)
	}
}
