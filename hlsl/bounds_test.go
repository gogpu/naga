package hlsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

func lowerBoundsSource(t *testing.T, source string) *ir.Module {
	t.Helper()
	module, _, err := wgsl.Parse(source, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func uncheckedPolicies() backend.BoundsCheckPolicies {
	u := backend.BoundsCheckUnchecked
	return backend.BoundsCheckPolicies{Index: u, Buffer: u, ImageLoad: u, BindingArray: u}
}

// TestBindingArrayRestrict: Rust naga's HLSL writer never checks a binding
// array index. Here the zero policy, Restrict, clamps it to the array's size,
// inside NonUniformResourceIndex, or to BindTarget.BindingArraySize for an
// unbounded array; an unbounded array without it is an error, as in the
// SPIR-V backend, and Unchecked writes the index as it is.
func TestBindingArrayRestrict(t *testing.T) {
	const sized = `
enable wgpu_binding_array;
@group(0) @binding(0) var textures: binding_array<texture_2d<f32>, 8>;
@fragment fn main(@location(0) @interpolate(flat) i: u32) -> @location(0) vec4<f32> {
    return textureLoad(textures[i], vec2<i32>(0), 0);
}`
	const unbounded = `
enable wgpu_binding_array;
@group(0) @binding(0) var textures: binding_array<texture_2d<f32>>;
@fragment fn main(@location(0) @interpolate(flat) i: u32) -> @location(0) vec4<f32> {
    return textureLoad(textures[i], vec2<i32>(0), 0);
}`
	binding := ir.ResourceBinding{Group: 0, Binding: 0}

	code, _, err := hlsl.Compile(lowerBoundsSource(t, sized), hlsl.Options{FakeMissingBindings: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "textures[NonUniformResourceIndex(min(uint(i), 7u))]") {
		t.Errorf("sized binding array, Restrict: no clamp to 7u:\n%s", code)
	}

	size := uint32(16)
	code, _, err = hlsl.Compile(lowerBoundsSource(t, unbounded), hlsl.Options{
		BindingMap: map[ir.ResourceBinding]hlsl.BindTarget{binding: {BindingArraySize: &size}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "min(uint(i), 15u)") {
		t.Errorf("unbounded binding array with BindingArraySize 16: no clamp to 15u:\n%s", code)
	}

	_, _, err = hlsl.Compile(lowerBoundsSource(t, unbounded), hlsl.Options{FakeMissingBindings: true})
	if err == nil || !strings.Contains(err.Error(), "BindingArraySize") {
		t.Errorf("unbounded binding array without a size: err = %v, want BindingArraySize named", err)
	}

	code, _, err = hlsl.Compile(lowerBoundsSource(t, unbounded), hlsl.Options{FakeMissingBindings: true, BoundsCheckPolicies: uncheckedPolicies()})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(code, "min(uint(") {
		t.Errorf("Unchecked: clamp written:\n%s", code)
	}
}

// TestIndexAndBufferPolicies: Index clamps a local array, Buffer a uniform
// array whose BindTarget.RestrictIndexing is set (Rust's restrict_indexing),
// each independently; ReadZeroSkipWrite is refused there.
func TestIndexAndBufferPolicies(t *testing.T) {
	const source = `
struct U { values: array<vec4<f32>, 4> }
@group(0) @binding(0) var<uniform> u: U;
@fragment fn main(@location(0) @interpolate(flat) i: u32) -> @location(0) vec4<f32> {
    var local = array<f32, 3>(1.0, 2.0, 3.0);
    return u.values[i] * local[i];
}`
	binding := ir.ResourceBinding{Group: 0, Binding: 0}
	compile := func(index, buffer backend.BoundsCheckPolicy, restrictBinding bool) string {
		t.Helper()
		p := uncheckedPolicies()
		p.Index, p.Buffer = index, buffer
		code, _, err := hlsl.Compile(lowerBoundsSource(t, source), hlsl.Options{
			BindingMap:          map[ir.ResourceBinding]hlsl.BindTarget{binding: {RestrictIndexing: restrictBinding}},
			BoundsCheckPolicies: p,
		})
		if err != nil {
			t.Fatal(err)
		}
		return code
	}
	r, u := backend.BoundsCheckRestrict, backend.BoundsCheckUnchecked
	for _, tc := range []struct {
		index, buffer   backend.BoundsCheckPolicy
		restrictBinding bool
		local, uniform  bool
	}{
		{r, r, true, true, true},
		{r, r, false, true, false},
		{r, u, true, true, false},
		{u, r, true, false, true},
		{u, u, true, false, false},
	} {
		code := compile(tc.index, tc.buffer, tc.restrictBinding)
		if got := strings.Contains(code, "min(uint(i), 2u)"); got != tc.local {
			t.Errorf("Index %v: local clamp %v, want %v:\n%s", tc.index, got, tc.local, code)
		}
		if got := strings.Contains(code, "min(uint(i), 3u)"); got != tc.uniform {
			t.Errorf("Buffer %v, BindTarget.RestrictIndexing %v: uniform clamp %v, want %v:\n%s",
				tc.buffer, tc.restrictBinding, got, tc.uniform, code)
		}
	}

	for _, field := range []string{"Index", "Buffer", "BindingArray"} {
		p := uncheckedPolicies()
		switch field {
		case "Index":
			p.Index = backend.BoundsCheckReadZeroSkipWrite
		case "Buffer":
			p.Buffer = backend.BoundsCheckReadZeroSkipWrite
		case "BindingArray":
			p.BindingArray = backend.BoundsCheckReadZeroSkipWrite
		}
		_, _, err := hlsl.Compile(lowerBoundsSource(t, source), hlsl.Options{FakeMissingBindings: true, BoundsCheckPolicies: p})
		if err == nil || !strings.Contains(err.Error(), "BoundsCheckPolicies."+field+": ReadZeroSkipWrite is not supported") {
			t.Errorf("%s ReadZeroSkipWrite: err = %v, want a refusal naming it", field, err)
		}
	}
}
