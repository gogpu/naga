package glsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
)

// TestBindingMapMissingEntry: on a version with explicit bindings, a
// resource a non-nil BindingMap leaves out is an error (it would get the
// driver's binding 0; Rust naga writes it without a binding). A sampler
// needs no entry: GLSL combines it with the texture, which carries the
// binding. A version without explicit bindings writes none, so the map is
// not checked there.
func TestBindingMapMissingEntry(t *testing.T) {
	module := lowerWGSL(t, `
@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;
@group(0) @binding(2) var<uniform> tint: vec4<f32>;
@fragment fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(tex, samp, uv) * tint;
}`)
	textureOnly := map[glsl.BindingMapKey]uint8{{Group: 0, Binding: 0}: 0}
	_, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES310, BindingMap: textureOnly})
	if err == nil || !strings.Contains(err.Error(), "tint at group 0, binding 2") {
		t.Errorf("uniform left out: err = %v, want tint named", err)
	}

	full := map[glsl.BindingMapKey]uint8{{Group: 0, Binding: 0}: 0, {Group: 0, Binding: 2}: 1}
	code, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES310, BindingMap: full})
	if err != nil {
		t.Fatalf("texture and uniform listed, sampler not: %v", err)
	}
	if !strings.Contains(code, "binding = 0") || !strings.Contains(code, "binding = 1") {
		t.Errorf("explicit bindings missing:\n%s", code)
	}

	if _, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.VersionES300, BindingMap: textureOnly}); err != nil {
		t.Errorf("ES 3.00 has no explicit bindings, the map is not checked: %v", err)
	}
}
