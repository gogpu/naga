package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// The arguments of a texture call depend on the texture's type: an arrayed
// texture takes an array index before the level, a multisampled one a
// sample index instead of it. A texture a function takes as an argument has
// the argument's type, also where a global of another type has its name.
func TestTextureArgumentsTakeTheirType(t *testing.T) {
	m := helperCompileWGSL(t, `
@group(0) @binding(0) var t: texture_2d<f32>;
fn arrayed(t: texture_2d_array<f32>, c: vec2<i32>) -> vec4<f32> {
    return textureLoad(t, c, 1, 2);
}
fn multisampled(ms: texture_multisampled_2d<f32>, c: vec2<i32>) -> vec4<f32> {
    return textureLoad(ms, c, 3);
}
@fragment fn fs_main(@builtin(position) p: vec4<f32>) -> @location(0) vec4<f32> {
    return textureLoad(t, vec2<i32>(p.xy), 0);
}
`)
	for _, tc := range []struct {
		function          string
		arrayIndex, level bool
		sample            bool
	}{
		{"arrayed", true, true, false},
		{"multisampled", false, false, true},
	} {
		for _, f := range m.Functions {
			if f.Name != tc.function {
				continue
			}
			for _, e := range f.Expressions {
				load, ok := e.Kind.(ir.ExprImageLoad)
				if !ok {
					continue
				}
				if (load.ArrayIndex != nil) != tc.arrayIndex || (load.Level != nil) != tc.level || (load.Sample != nil) != tc.sample {
					t.Errorf("%s: array index %v, level %v, sample %v; want %v, %v, %v", tc.function,
						load.ArrayIndex != nil, load.Level != nil, load.Sample != nil, tc.arrayIndex, tc.level, tc.sample)
				}
			}
		}
	}
}
