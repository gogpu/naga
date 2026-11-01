package hlsl_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
)

// TestMissingBindings (ADR-009 §6 rows 5 and 21): a nil BindingMap places
// every resource at its WGSL group and binding; a non-nil map without an
// entry for one is an error wrapping backend.ErrMissingBinding, unless
// FakeMissingBindings. Before, the resource silently went to
// register(0, space0).
func TestMissingBindings(t *testing.T) {
	const source = `
@group(0) @binding(0) var<storage, read_write> a: array<u32, 4>;
@group(1) @binding(3) var<storage, read_write> b: array<u32, 4>;
@compute @workgroup_size(1) fn main() { a[0] = b[1]; }`
	module := lowerBoundsSource(t, source)
	partial := map[ir.ResourceBinding]hlsl.BindTarget{{Group: 0, Binding: 0}: {Register: 5}}

	_, _, err := hlsl.Compile(module, hlsl.Options{BindingMap: partial})
	if !errors.Is(err, backend.ErrMissingBinding) || !strings.Contains(err.Error(), "group 1, binding 3") {
		t.Errorf("partial map: err = %v, want backend.ErrMissingBinding naming group 1, binding 3", err)
	}
	code, _, err := hlsl.Compile(module, hlsl.Options{BindingMap: partial, FakeMissingBindings: true})
	if err != nil || !strings.Contains(code, "register(u5)") || !strings.Contains(code, "register(u3, space1)") {
		t.Errorf("partial map, FakeMissingBindings: err = %v\n%s", err, code)
	}
	code, _, err = hlsl.Compile(module, hlsl.Options{})
	if err != nil || !strings.Contains(code, "register(u0)") || !strings.Contains(code, "register(u3, space1)") {
		t.Errorf("nil map: err = %v, want WGSL group and binding\n%s", err, code)
	}
}

// TestNumWorkgroupsNeedsSpecialConstants: HLSL has no system value for
// num_workgroups, which comes from the special constants buffer; without
// SpecialConstantsBinding it is an error, also under FakeMissingBindings.
// Rust naga writes it as SV_GroupID then, the workgroup's ID instead of the
// count.
func TestNumWorkgroupsNeedsSpecialConstants(t *testing.T) {
	module := lowerBoundsSource(t, `
@group(0) @binding(0) var<storage, read_write> out: vec3<u32>;
@compute @workgroup_size(1) fn main(@builtin(num_workgroups) n: vec3<u32>) { out = n; }`)
	for _, fake := range []bool{false, true} {
		_, _, err := hlsl.Compile(module, hlsl.Options{FakeMissingBindings: fake})
		if !errors.Is(err, backend.ErrMissingBinding) || !strings.Contains(err.Error(), "SpecialConstantsBinding") {
			t.Errorf("FakeMissingBindings %v: err = %v, want SpecialConstantsBinding named", fake, err)
		}
	}
	code, _, err := hlsl.Compile(module, hlsl.Options{SpecialConstantsBinding: &hlsl.BindTarget{Register: 1}})
	if err != nil || !strings.Contains(code, "_NagaConstants.first_vertex") {
		t.Errorf("with SpecialConstantsBinding: err = %v\n%s", err, code)
	}
}

// TestSamplerBufferBindingMap: the sampler index buffer of a group that a
// non-nil SamplerBufferBindingMap leaves out is an error; a nil map puts it
// at register(t<group>, space255), Rust naga's fallback. Before, a nil map
// put it at register(t0, space0) silently.
func TestSamplerBufferBindingMap(t *testing.T) {
	module := lowerBoundsSource(t, `
@group(2) @binding(0) var tex: texture_2d<f32>;
@group(2) @binding(1) var samp: sampler;
@fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> { return textureSample(tex, samp, uv); }`)
	code, _, err := hlsl.Compile(module, hlsl.Options{})
	if err != nil || !strings.Contains(code, "register(t2, space255)") {
		t.Errorf("nil SamplerBufferBindingMap: err = %v, want register(t2, space255)\n%s", err, code)
	}
	if !strings.Contains(code, "nagaSamplerHeap[2048]: register(s0, space0)") || !strings.Contains(code, "nagaComparisonSamplerHeap[2048]: register(s0, space1)") {
		t.Errorf("nil SamplerHeapTargets: want the default heaps\n%s", code)
	}
	_, _, err = hlsl.Compile(module, hlsl.Options{SamplerBufferBindingMap: map[uint32]hlsl.BindTarget{0: {}}})
	if !errors.Is(err, backend.ErrMissingBinding) {
		t.Errorf("SamplerBufferBindingMap without group 2: err = %v, want backend.ErrMissingBinding", err)
	}
}
