package dxil

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
)

// TestMissingBindings (ADR-009 §6 rows 5 and 21): what a non-nil map leaves
// out is an error wrapping backend.ErrMissingBinding, unless
// FakeMissingBindings; a nil map is the documented default. Before, a
// resource missing from a non-nil BindingMap kept its WGSL numbers
// silently, and num_workgroups without SpecialConstantsBinding read a
// constant buffer at b0 in space 0 that can collide with the caller's b0.
func TestMissingBindings(t *testing.T) {
	compile := func(src string, opts Options) error {
		t.Helper()
		_, err := Compile(lowerSource(t, src), opts)
		return err
	}
	const twoBuffers = `
@group(0) @binding(0) var<storage, read_write> a: array<u32, 4>;
@group(1) @binding(3) var<storage, read_write> b: array<u32, 4>;
@compute @workgroup_size(1) fn main() { a[0] = b[1]; }`
	partial := BindingMap{{Group: 0, Binding: 0}: {Register: 2}}
	opts := DefaultOptions()
	opts.BindingMap = partial
	if err := compile(twoBuffers, opts); !errors.Is(err, backend.ErrMissingBinding) || !strings.Contains(err.Error(), "group 1, binding 3") {
		t.Errorf("partial BindingMap: err = %v, want backend.ErrMissingBinding naming group 1, binding 3", err)
	}
	opts.FakeMissingBindings = true
	if err := compile(twoBuffers, opts); err != nil {
		t.Errorf("partial BindingMap, FakeMissingBindings: %v", err)
	}
	if err := compile(twoBuffers, DefaultOptions()); err != nil {
		t.Errorf("nil BindingMap: %v", err)
	}

	const numWorkgroups = `
@group(0) @binding(0) var<storage, read_write> out: vec3<u32>;
@compute @workgroup_size(1) fn main(@builtin(num_workgroups) n: vec3<u32>) { out = n; }`
	if err := compile(numWorkgroups, DefaultOptions()); !errors.Is(err, backend.ErrMissingBinding) ||
		!strings.Contains(err.Error(), "SpecialConstantsBinding") {
		t.Errorf("num_workgroups without SpecialConstantsBinding: err = %v, want it named", err)
	}
	opts = DefaultOptions()
	opts.FakeMissingBindings = true
	if err := compile(numWorkgroups, opts); err != nil {
		t.Errorf("num_workgroups, FakeMissingBindings: %v", err)
	}
	opts = DefaultOptions()
	opts.SpecialConstantsBinding = &BindTarget{Register: 1, Space: 2}
	if err := compile(numWorkgroups, opts); err != nil {
		t.Errorf("num_workgroups with SpecialConstantsBinding: %v", err)
	}

	const sampled = `
@group(2) @binding(0) var tex: texture_2d<f32>;
@group(2) @binding(1) var samp: sampler;
@group(0) @binding(0) var<storage, read_write> out: vec4<f32>;
@compute @workgroup_size(1) fn main() { out = textureSampleLevel(tex, samp, vec2<f32>(0.5), 0.0); }`
	opts = DefaultOptions()
	opts.SamplerBufferBindingMap = map[uint32]BindTarget{0: {}}
	if err := compile(sampled, opts); !errors.Is(err, backend.ErrMissingBinding) || !strings.Contains(err.Error(), "group 2") {
		t.Errorf("SamplerBufferBindingMap without group 2: err = %v, want backend.ErrMissingBinding naming group 2", err)
	}
	if err := compile(sampled, DefaultOptions()); err != nil {
		t.Errorf("nil SamplerBufferBindingMap: %v", err)
	}
}
