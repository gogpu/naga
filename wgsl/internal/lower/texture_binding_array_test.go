package lower

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/ir"
)

// describeImageCall summarizes the argument shape of the only image call
// of a function: which optional operands it has and what level it samples.
func describeImageCall(t *testing.T, f *ir.Function) string {
	t.Helper()
	var got []string
	for _, e := range f.Expressions {
		switch k := e.Kind.(type) {
		case ir.ExprImageLoad:
			got = append(got, fmt.Sprintf("load array=%v level=%v sample=%v",
				k.ArrayIndex != nil, k.Level != nil, k.Sample != nil))
		case ir.ExprImageSample:
			gather := "-"
			if k.Gather != nil {
				gather = fmt.Sprint(*k.Gather)
			}
			got = append(got, fmt.Sprintf("sample array=%v level=%T depth=%v gather=%s",
				k.ArrayIndex != nil, k.Level, k.DepthRef != nil, gather))
		case ir.ExprImageQuery:
			got = append(got, fmt.Sprintf("query %T", k.Query))
		}
	}
	if len(got) != 1 {
		t.Fatalf("%s: %d image calls %v, want 1", f.Name, len(got), got)
	}
	return got[0]
}

// TestTextureArgumentsOfBindingArrayElements: a texture call on an element
// of a binding_array takes its arguments by the element's type, as Rust
// naga resolves the type of the lowered image expression (resolve_inner! in
// front/wgsl/lower/mod.rs, texture_sample_helper and the textureLoad,
// textureStore and query arms). naga looked the texture up by name, found
// no image for `arr[i]` and read an arrayed texture's array index as its
// level, a multisampled texture's sample index as a level, and a depth
// texture passed to textureGather as the gather component.
func TestTextureArgumentsOfBindingArrayElements(t *testing.T) {
	m := helperCompileWGSL(t, `enable wgpu_binding_array;
@group(0) @binding(0) var ta: binding_array<texture_2d_array<f32>, 4>;
@group(0) @binding(1) var tms: binding_array<texture_multisampled_2d<f32>, 4>;
@group(0) @binding(2) var td: binding_array<texture_depth_2d, 4>;
@group(0) @binding(3) var tda: binding_array<texture_depth_2d_array, 4>;
@group(0) @binding(4) var s: sampler;
@group(0) @binding(5) var sc: sampler_comparison;

fn load_arrayed(i: u32, c: vec2<i32>) -> vec4<f32> { return textureLoad(ta[i], c, 1, 2); }
fn load_multisampled(i: u32, c: vec2<i32>) -> vec4<f32> { return textureLoad(tms[i], c, 3); }
fn load_depth_arrayed(i: u32, c: vec2<i32>) -> f32 { return textureLoad(tda[i], c, 1, 0); }
fn sample_arrayed(i: u32, uv: vec2<f32>) -> vec4<f32> { return textureSample(ta[i], s, uv, 1); }
fn sample_bias_arrayed(i: u32, uv: vec2<f32>) -> vec4<f32> { return textureSampleBias(ta[i], s, uv, 1, 0.5); }
fn sample_level_arrayed(i: u32, uv: vec2<f32>) -> vec4<f32> { return textureSampleLevel(ta[i], s, uv, 1, 2.0); }
fn sample_grad_arrayed(i: u32, uv: vec2<f32>) -> vec4<f32> {
    return textureSampleGrad(ta[i], s, uv, 1, vec2<f32>(), vec2<f32>());
}
fn sample_level_depth(i: u32, uv: vec2<f32>) -> f32 { return textureSampleLevel(td[i], s, uv, 1); }
fn compare_depth_arrayed(i: u32, uv: vec2<f32>) -> f32 { return textureSampleCompare(tda[i], sc, uv, 1, 0.5); }
fn gather_depth(i: u32, uv: vec2<f32>) -> vec4<f32> { return textureGather(td[i], s, uv); }
fn gather_compare_arrayed(i: u32, uv: vec2<f32>) -> vec4<f32> { return textureGatherCompare(tda[i], sc, uv, 1, 0.5); }
fn layers(i: u32) -> u32 { return textureNumLayers(ta[i]); }
`)
	want := map[string]string{
		"load_arrayed":           "load array=true level=true sample=false",
		"load_multisampled":      "load array=false level=false sample=true",
		"load_depth_arrayed":     "load array=true level=true sample=false",
		"sample_arrayed":         "sample array=true level=ir.SampleLevelAuto depth=false gather=-",
		"sample_bias_arrayed":    "sample array=true level=ir.SampleLevelBias depth=false gather=-",
		"sample_level_arrayed":   "sample array=true level=ir.SampleLevelExact depth=false gather=-",
		"sample_grad_arrayed":    "sample array=true level=ir.SampleLevelGradient depth=false gather=-",
		"sample_level_depth":     "sample array=false level=ir.SampleLevelExact depth=false gather=-",
		"compare_depth_arrayed":  "sample array=true level=ir.SampleLevelAuto depth=true gather=-",
		"gather_depth":           "sample array=false level=ir.SampleLevelZero depth=false gather=0",
		"gather_compare_arrayed": "sample array=true level=ir.SampleLevelZero depth=true gather=0",
		"layers":                 "query ir.ImageQueryNumLayers",
	}
	seen := 0
	for i := range m.Functions {
		f := &m.Functions[i]
		w, ok := want[f.Name]
		if !ok {
			continue
		}
		seen++
		if got := describeImageCall(t, f); got != w {
			t.Errorf("%s: %s, want %s", f.Name, got, w)
		}
	}
	if seen != len(want) {
		t.Fatalf("found %d of %d functions", seen, len(want))
	}
}
