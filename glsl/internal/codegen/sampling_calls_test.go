package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

func lowerWGSL(t *testing.T, source string) *ir.Module {
	t.Helper()
	tokens, err := wgsl.NewLexer(source).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatal(err)
	}
	module, err := wgsl.Lower(ast)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// A texture and sampler a helper function samples are the globals its call
// sites pass. The pairs found only in the entry point left a texture sampled
// in a helper without a TextureMapping, so the GL backend never put the
// sampler on that texture's unit (Rust naga follows samplings through calls).
func TestTextureMappingsFollowCalls(t *testing.T) {
	module := lowerWGSL(t, `
@group(0) @binding(0) var t0: texture_2d<f32>;
@group(0) @binding(1) var t1: texture_2d<f32>;
@group(0) @binding(2) var t2: texture_2d<f32>;
@group(0) @binding(3) var s: sampler;
@group(0) @binding(4) var s2: sampler;

fn fetch(t: texture_2d<f32>, smp: sampler, uv: vec2<f32>) -> vec4<f32> {
    return textureSample(t, smp, uv);
}

fn fetch_t1(uv: vec2<f32>) -> vec4<f32> {
    return fetch(t1, s, uv);
}

fn fetch_global_sampler(t: texture_2d<f32>, uv: vec2<f32>) -> vec4<f32> {
    return textureSample(t, s2, uv);
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(t0, s, uv) + fetch_t1(uv) + fetch_global_sampler(t2, uv);
}
`)
	_, info, err := Compile(module, Options{LangVersion: Version330})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]ir.ResourceBinding{
		"_group_0_binding_0_fs": {Group: 0, Binding: 3},
		"_group_0_binding_1_fs": {Group: 0, Binding: 3},
		"_group_0_binding_2_fs": {Group: 0, Binding: 4},
	}
	for name, sampler := range want {
		tm, ok := info.TextureMappings[name]
		if !ok {
			t.Errorf("no TextureMapping for %s; have %v", name, info.TextureMappings)
			continue
		}
		if tm.SamplerBinding == nil || *tm.SamplerBinding != sampler {
			t.Errorf("%s: sampler binding %v, want %v", name, tm.SamplerBinding, sampler)
		}
	}
}

// A GL texture unit holds one sampler object, so a texture sampled with two
// samplers, directly or through a helper, is an error, as in Rust naga. It
// was declared a second time as a combined uniform named texture_sampler,
// without a binding, and the GL backend kept whichever sampler it mapped
// last for the texture's unit.
func TestTextureWithTwoSamplersIsAnError(t *testing.T) {
	for name, body := range map[string]string{
		"direct": `return textureSample(t, clamp, uv) + textureSample(t, repeat, uv);`,
		"helper": `return fetch(t, clamp, uv) + textureSample(t, repeat, uv);`,
	} {
		module := lowerWGSL(t, `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var clamp: sampler;
@group(0) @binding(2) var repeat: sampler;

fn fetch(tex: texture_2d<f32>, smp: sampler, uv: vec2<f32>) -> vec4<f32> {
    return textureSample(tex, smp, uv);
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    `+body+`
}
`)
		_, _, err := Compile(module, Options{LangVersion: Version330})
		if err == nil || !strings.Contains(err.Error(), "two samplers") {
			t.Errorf("%s: err = %v, want the two samplers error", name, err)
		}
	}

	module := lowerWGSL(t, `
@group(0) @binding(0) var t: texture_2d<f32>;
@group(0) @binding(1) var s: sampler;

fn fetch(tex: texture_2d<f32>, smp: sampler, uv: vec2<f32>) -> vec4<f32> {
    return textureSample(tex, smp, uv);
}

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return fetch(t, s, uv) + textureSample(t, s, uv * 2.0);
}
`)
	if _, _, err := Compile(module, Options{LangVersion: Version330}); err != nil {
		t.Errorf("one texture with one sampler in two places: %v", err)
	}
}
