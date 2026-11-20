package front

// Contracts of the GL 3.30 core and ES 3.00 dialects (design §15). Every rule
// here was confirmed with glslangValidator 11:16.2.0 on the same source before
// it was implemented; the comment above each test carries the oracle's output.
// glslangValidator without -G validates the language itself (with -G it builds
// SPIR-V, which rejects ES sources below 3.10 before the rule under test runs).

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// parseOK parses a shader that must succeed and returns its module and info.
func parseOK(t *testing.T, src string, opts Options) (*ir.Module, Info) {
	t.Helper()
	mod, info, err := Parse(src, opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return mod, info
}

// parseErr asserts that parsing fails with a message containing want.
func parseErr(t *testing.T, src string, opts Options, want string) {
	t.Helper()
	_, _, err := Parse(src, opts)
	if err == nil {
		t.Fatalf("Parse succeeded, want an error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want it to contain %q", err, want)
	}
}

// validateOK validates a module the way a caller must after Parse.
func validateOK(t *testing.T, mod *ir.Module) {
	t.Helper()
	if _, err := valid.Validate(mod, valid.Options{}); err != nil {
		t.Fatalf("the parsed module does not validate: %v", err)
	}
}

// warnMessages returns the messages of the parse's warnings.
func warnMessages(info Info) []string {
	messages := make([]string, 0, len(info.Warnings))
	for _, w := range info.Warnings {
		messages = append(messages, w.Message)
	}
	return messages
}

const gl330Vertex = `#version 330 core
layout(location = 0) in vec3 pos;
uniform Common { mat4 mvp; } common;
out vec2 uv;
void main() {
    gl_Position = common.mvp * vec4(pos, 1.0);
    uv = pos.xy;
}
`

const gl330Fragment = `#version 330 core
precision mediump float;
uniform Common { mat4 mvp; } common_frag;
uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() {
    color = texture(tex, vec2(0.5)) * common_frag.mvp[0][0];
}
`

const es300Fragment = `#version 300 es
precision mediump float;
uniform Common { mat4 mvp; } common;
uniform sampler2D tex;
out vec4 color;
void main() {
    color = texture(tex, vec2(0.5)) + common.mvp[0];
}
`

// TestGL330ShadersParseWithoutBindings is the dialect's core case: a plain GL
// 3.30 vertex and fragment pair, no layout(binding) anywhere, one counter
// places the uniform block, the split image and the split sampler (design §11,
// R2). glslang: both sources compile cleanly (frag330_loc.frag, vari300.vert
// and the block/sampler sources of the same batch).
func TestGL330ShadersParseWithoutBindings(t *testing.T) {
	mod, info := parseOK(t, gl330Vertex, Options{Stage: ir.StageVertex})
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the 330 vertex module does not validate: %v", verr)
	}
	if got := globalBinding(t, mod, "common"); got != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Fatalf("uniform block binding = %+v, want (0, 0)", got)
	}

	frag, fInfo := parseOK(t, gl330Fragment, Options{Stage: ir.StageFragment})
	validateOK(t, frag)
	if got := globalBinding(t, frag, "common_frag"); got != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Fatalf("fragment uniform block binding = %+v, want (0, 0)", got)
	}
	if got := globalBinding(t, frag, "tex"); got != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Fatalf("image binding = %+v, want (0, 1)", got)
	}
	if got := globalBinding(t, frag, "tex_sampler"); got != (ir.ResourceBinding{Group: 0, Binding: 2}) {
		t.Fatalf("split sampler binding = %+v, want (0, 2)", got)
	}
	if len(fInfo.CombinedSamplers) != 1 {
		t.Fatalf("CombinedSamplers = %+v, want one split", fInfo.CombinedSamplers)
	}
	if len(info.CombinedSamplers) != 0 {
		t.Fatalf("the vertex source has no combined sampler, reported %+v", info.CombinedSamplers)
	}
	if len(warnMessages(info)) != 0 || len(warnMessages(fInfo)) != 0 {
		t.Fatalf("unexpected warnings: %v %v", warnMessages(info), warnMessages(fInfo))
	}
}

// TestES300ShaderParsesWithoutBindings is the ES 3.00 arm: the default float
// precision statement is present, the combined sampler and the uniform block
// are placed by the same counter, and the module validates. glslang compiles
// the same source (frag300_loc.frag, ext_f16_300_require.frag batch).
func TestES300ShaderParsesWithoutBindings(t *testing.T) {
	mod, info := parseOK(t, es300Fragment, Options{Stage: ir.StageFragment})
	validateOK(t, mod)
	if got := globalBinding(t, mod, "common"); got != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Fatalf("uniform block binding = %+v, want (0, 0)", got)
	}
	if got := globalBinding(t, mod, "tex"); got != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Fatalf("image binding = %+v, want (0, 1)", got)
	}
	if got := globalBinding(t, mod, "tex_sampler"); got != (ir.ResourceBinding{Group: 0, Binding: 2}) {
		t.Fatalf("split sampler binding = %+v, want (0, 2)", got)
	}
	if len(info.CombinedSamplers) != 1 {
		t.Fatalf("CombinedSamplers = %+v, want one split", info.CombinedSamplers)
	}
}

// TestResourceBindingsByName places resources the caller names and lets the
// counter skip their slots. A supplied slot is reserved before any declaration
// is lowered, so an earlier automatic allocation cannot take it.
func TestResourceBindingsByName(t *testing.T) {
	mod, _ := parseOK(t, es300Fragment, Options{
		Stage: ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{
			"common": {Group: 0, Binding: 3},
			"tex":    {Group: 0, Binding: 5},
		},
	})
	validateOK(t, mod)
	if got := globalBinding(t, mod, "common"); got != (ir.ResourceBinding{Group: 0, Binding: 3}) {
		t.Fatalf("uniform block binding = %+v, want the supplied (0, 3)", got)
	}
	if got := globalBinding(t, mod, "tex"); got != (ir.ResourceBinding{Group: 0, Binding: 5}) {
		t.Fatalf("image binding = %+v, want the supplied (0, 5)", got)
	}
	// The split sampler takes the lowest free slot, which skips both
	// reservations: 0.
	if got := globalBinding(t, mod, "tex_sampler"); got != (ir.ResourceBinding{Group: 0, Binding: 0}) {
		t.Fatalf("split sampler binding = %+v, want the first free (0, 0)", got)
	}
}

// TestResourceBindingsCollision: two supplied names claiming one slot is the
// caller asking two globals to share a binding, which is an error at the second
// declaration (design §11, R2).
func TestResourceBindingsCollision(t *testing.T) {
	src := `#version 300 es
precision mediump float;
uniform sampler2D texA;
uniform sampler2D texB;
out vec4 color;
void main() { color = texture(texA, vec2(0.5)) + texture(texB, vec2(0.5)); }
`
	parseErr(t, src, Options{
		Stage: ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{
			"texA": {Group: 0, Binding: 4},
			"texB": {Group: 0, Binding: 4},
		},
	}, "which another resource of the module already uses")
}

// TestResourceBindingsUnmatchedIsWarning: an entry naming nothing in the source
// is a warning, never a silent fallback (design §11, R5).
func TestResourceBindingsUnmatchedIsWarning(t *testing.T) {
	_, info := parseOK(t, es300Fragment, Options{
		Stage:            ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{"absent": {Group: 0, Binding: 1}},
	})
	warnings := strings.Join(warnMessages(info), "\n")
	if !strings.Contains(warnings, `"absent"`) || !strings.Contains(warnings, "did not match a resource") {
		t.Fatalf("warnings = %q, want the unmatched ResourceBindings entry reported", warnings)
	}
}

// TestResourceBindingsIgnoredForVulkan is the decision for a source that binds
// itself: layout(binding) wins and the map is not consulted at all — an entry
// can neither claim a slot nor move the automatic placement of a split sampler,
// and no warning is produced. The module is byte-for-byte the same with and
// without the map, which keeps the 440/450/460 path identical.
func TestResourceBindingsIgnoredForVulkan(t *testing.T) {
	src := `#version 450
layout(set = 0, binding = 2) uniform sampler2D tex;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 color;
void main() { color = texture(tex, uv); }
`
	plain, plainInfo := parseOK(t, src, Options{Stage: ir.StageFragment})
	// The map names a resource at a free slot, which would move the split
	// sampler's automatic placement if reservations were taken on this path.
	mapped, mappedInfo := parseOK(t, src, Options{
		Stage:            ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{"ghost": {Group: 0, Binding: 0}},
	})
	validateOK(t, plain)
	validateOK(t, mapped)
	if got := globalBinding(t, mapped, "tex"); got != (ir.ResourceBinding{Group: 0, Binding: 2}) {
		t.Fatalf("image binding = %+v, want the declared (0, 2)", got)
	}
	want := globalBinding(t, plain, "tex_sampler")
	if got := globalBinding(t, mapped, "tex_sampler"); got != want {
		t.Fatalf("split sampler binding = %+v with a map, %+v without; the map must not move it", got, want)
	}
	if len(plainInfo.Warnings) != 0 || len(mappedInfo.Warnings) != 0 {
		t.Fatalf("warnings = %v / %v, want none on the Vulkan path", warnMessages(plainInfo), warnMessages(mappedInfo))
	}
}

// TestSamplerBindingsPlacementWins: SamplerBindings is the more specific
// placement of a split sampler and wins over the automatic counter, after the
// image binding is fixed by ResourceBindings or the counter.
func TestSamplerBindingsPlacementWins(t *testing.T) {
	mod, info := parseOK(t, es300Fragment, Options{
		Stage:           ir.StageFragment,
		SamplerBindings: frontend.SamplerBindings{{Group: 0, Binding: 1}: {Group: 0, Binding: 7}},
	})
	validateOK(t, mod)
	if got := globalBinding(t, mod, "tex"); got != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Fatalf("image binding = %+v, want the automatic (0, 1)", got)
	}
	if got := globalBinding(t, mod, "tex_sampler"); got != (ir.ResourceBinding{Group: 0, Binding: 7}) {
		t.Fatalf("split sampler binding = %+v, want the mapped (0, 7)", got)
	}
	if len(info.CombinedSamplers) != 1 || info.CombinedSamplers[0].Split != (ir.ResourceBinding{Group: 0, Binding: 7}) {
		t.Fatalf("CombinedSamplers = %+v, want split (0, 7)", info.CombinedSamplers)
	}
}

// TestDialectGates is the refusal table of design §15. Each row's comment is
// the glslang output for the same source (glslangValidator 11:16.2.0, no -G).
//
// A plain uniform is no longer one of them: GL 3.30 core and ES 3.00 collect
// theirs into a generated default uniform block (ADR-013), and the accepted
// forms are in default_block_test.go.
func TestDialectGates(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stage  ir.ShaderStage
		want   string
	}{
		{
			name: "compute in 330",
			// ERROR: #version: compute shaders require es profile with version
			// 310 or above, or non-es profile with version 420 or above
			source: `#version 330 core
layout(local_size_x = 1) in;
void main() {}
`,
			stage: ir.StageCompute,
			want:  "compute shaders require es profile with version 310 or above",
		},
		{
			name: "compute in 300 es",
			// ERROR: #version: compute shaders require es profile with version
			// 310 or above, or non-es profile with version 420 or above
			source: `#version 300 es
layout(local_size_x = 1) in;
void main() {}
`,
			stage: ir.StageCompute,
			want:  "compute shaders require es profile with version 310 or above",
		},
		{
			name: "storage buffer in 330",
			// ERROR: 0:2: '' :  syntax error, unexpected IDENTIFIER (at `buffer`)
			source: `#version 330 core
buffer B { float x; } b;
void main() { gl_Position = vec4(b.x); }
`,
			stage: ir.StageVertex,
			want:  "'buffer' : not supported by GLSL 3.30 core",
		},
		{
			name: "storage buffer in 300 es",
			source: `#version 300 es
buffer B { float x; } b;
void main() { gl_Position = vec4(b.x); }
`,
			stage: ir.StageVertex,
			want:  "'buffer' : not supported by GLSL ES 3.00",
		},
		{
			name: "storage image in 330",
			// ERROR: 0:2: 'image load store' : not supported for this version
			// or the enabled extensions
			source: `#version 330 core
layout(rgba32f) uniform image2D img;
out vec4 color;
void main() { color = imageLoad(img, ivec2(0)); }
`,
			stage: ir.StageFragment,
			want:  "'image load store' : not supported by GLSL 3.30 core",
		},
		{
			name: "storage image in 300 es",
			source: `#version 300 es
precision highp float;
layout(rgba32f) uniform image2D img;
out vec4 color;
void main() { color = imageLoad(img, ivec2(0)); }
`,
			stage: ir.StageFragment,
			want:  "'image load store' : not supported by GLSL ES 3.00",
		},
		{
			name: "push constant in 330",
			// ERROR: 0:2: 'push_constant' : only allowed when using GLSL for Vulkan
			source: `#version 330 core
layout(push_constant) uniform P { float x; } p;
void main() { gl_Position = vec4(p.x); }
`,
			stage: ir.StageVertex,
			want:  "'push_constant' : only allowed when using GLSL for Vulkan",
		},
		{
			name: "double in 330",
			// ERROR: 0:3: 'double' : not supported for this version or the
			// enabled extensions. The constructor reaches the type gate; a
			// 1.0lf literal would stop earlier, since the frontend's lexer does
			// not accept 64-bit float literals yet.
			source: `#version 330 core
out vec4 color;
void main() { double d = double(1.0); color = vec4(float(d)); }
`,
			stage: ir.StageFragment,
			want:  "'double' : not supported by GLSL 3.30 core",
		},
		{
			name: "dvec in 300 es",
			// ERROR: 0:4: 'double' : not supported with this profile: es
			source: `#version 300 es
precision highp float;
out vec4 color;
void main() { dvec3 d = dvec3(1.0); color = vec4(float(d.x)); }
`,
			stage: ir.StageFragment,
			want:  "'double' : not supported by GLSL ES 3.00",
		},
		{
			name: "float16 in 330 without the extension",
			source: `#version 330 core
out vec4 color;
void main() { float16_t x = float16_t(1.0); color = vec4(float(x)); }
`,
			stage: ir.StageFragment,
			want:  "'float16_t' : not supported by GLSL 3.30 core",
		},
		{
			name: "layout binding in 330",
			// ERROR: 0:2: 'binding' : not supported for this version or the
			// enabled extensions
			source: `#version 330 core
layout(binding = 0) uniform sampler2D tex;
out vec4 color;
void main() { color = texture(tex, vec2(0.5)); }
`,
			stage: ir.StageFragment,
			want:  "'binding' : not supported by GLSL 3.30 core",
		},
		{
			name: "layout binding in 300 es",
			source: `#version 300 es
precision mediump float;
layout(binding = 0) uniform sampler2D tex;
out vec4 color;
void main() { color = texture(tex, vec2(0.5)); }
`,
			stage: ir.StageFragment,
			want:  "'binding' : not supported by GLSL ES 3.00",
		},
		{
			name: "layout set in 330",
			source: `#version 330 core
layout(set = 0) uniform Common { mat4 m; } common;
void main() { gl_Position = common.m * vec4(1.0); }
`,
			stage: ir.StageVertex,
			want:  "'set' : only allowed when using GLSL for Vulkan",
		},
		{
			name: "fragment output location in 300 es",
			// glslang 16.2 is lax here and accepts the qualifier
			// (frag300_loc.frag); the ES 3.00 specification has no location on
			// fragment outputs, which design §15 records as "no" (ES 3.10
			// introduced it), so the specification decides.
			source: `#version 300 es
precision mediump float;
layout(location = 0) out vec4 color;
void main() { color = vec4(1.0); }
`,
			stage: ir.StageFragment,
			want:  "'location qualifier on output' : not supported by GLSL ES 3.00",
		},
		{
			name: "varying location in 330",
			// ERROR: 0:2: 'location qualifier on output' : not supported for
			// this version or the enabled extensions
			source: `#version 330 core
layout(location = 0) out vec3 v;
void main() { v = vec3(1.0); gl_Position = vec4(1.0); }
`,
			stage: ir.StageVertex,
			want:  "'location qualifier on output' : not supported by GLSL 3.30 core",
		},
		{
			name: "varying location in 300 es",
			// ERROR: 0:3: 'location qualifier on input' : not supported in
			// this stage: fragment
			source: `#version 300 es
precision mediump float;
layout(location = 0) in vec3 v;
out vec4 color;
void main() { color = vec4(v, 1.0); }
`,
			stage: ir.StageFragment,
			want:  "'location qualifier on input' : not supported by GLSL ES 3.00",
		},
		{
			name: "textureGather in 330",
			// ERROR: 0:4: 'textureGather(...)' : not supported for this version
			// or the enabled extensions
			source: `#version 330 core
uniform sampler2D tex;
out vec4 color;
void main() { color = textureGather(tex, vec2(0.5), 0); }
`,
			stage: ir.StageFragment,
			want:  "'textureGather' : not supported by GLSL 3.30 core",
		},
		{
			name: "textureGather in 300 es",
			source: `#version 300 es
precision highp float;
uniform sampler2D tex;
out vec4 color;
void main() { color = textureGather(tex, vec2(0.5), 0); }
`,
			stage: ir.StageFragment,
			want:  "'textureGather' : not supported by GLSL ES 3.00",
		},
		{
			name: "textureQueryLevels in 330",
			// ERROR: 0:4: 'textureQueryLevels' : no matching overloaded
			// function found
			source: `#version 330 core
uniform sampler2D tex;
out vec4 color;
void main() { color = vec4(float(textureQueryLevels(tex))); }
`,
			stage: ir.StageFragment,
			want:  "'textureQueryLevels' : not supported by GLSL 3.30 core",
		},
		{
			name: "textureQueryLevels in 300 es",
			source: `#version 300 es
precision highp float;
uniform sampler2D tex;
out vec4 color;
void main() { color = vec4(float(textureQueryLevels(tex))); }
`,
			stage: ir.StageFragment,
			want:  "'textureQueryLevels' : not supported by GLSL ES 3.00",
		},
		{
			name: "textureSamples in 330",
			// ERROR: 0:4: 'textureSamples' : no matching overloaded function
			// found
			source: `#version 330 core
uniform sampler2DMS tex;
out ivec4 count;
void main() { count = ivec4(textureSamples(tex), 0, 0, 0); }
`,
			stage: ir.StageFragment,
			want:  "'textureSamples' : not supported by GLSL 3.30 core",
		},
		{
			name: "textureSamples in 300 es",
			// ERROR: 0:3: 'sampler2DMS' : Reserved word
			source: `#version 300 es
precision highp float;
uniform sampler2DMS tex;
out ivec4 count;
void main() { count = ivec4(textureSamples(tex), 0, 0, 0); }
`,
			stage: ir.StageFragment,
			want:  "'textureSamples' : not supported by GLSL ES 3.00",
		},
		{
			name: "workgroup size outside compute",
			// ERROR: 0:2: 'local_size_x' : there is no such layout identifier
			// for this stage taking an assigned value
			source: `#version 330 core
layout(local_size_x = 1) in;
void main() { gl_Position = vec4(1.0); }
`,
			stage: ir.StageVertex,
			want:  "there is no such layout identifier for this stage taking an assigned value",
		},
		{
			name: "cube array sampler in 330",
			// ERROR: 0:2: 'samplerCubeArray' : Reserved word
			source: `#version 330 core
uniform samplerCubeArray tex;
out vec4 color;
void main() { color = vec4(1.0); }
`,
			stage: ir.StageFragment,
			want:  "'samplerCubeArray' : not supported by GLSL 3.30 core",
		},
		{
			name: "cube array shadow sampler in 300 es",
			source: `#version 300 es
precision mediump float;
uniform samplerCubeArrayShadow tex;
out vec4 color;
void main() { color = vec4(1.0); }
`,
			stage: ir.StageFragment,
			want:  "'samplerCubeArrayShadow' : not supported by GLSL ES 3.00",
		},
		{
			name: "push_constant with a value in 330",
			// The qualifier takes no value: Rust naga's "Qualifier doesn't
			// expect a value" wins over the Vulkan-only dialect message, and
			// the space must not become immediate.
			source: `#version 330 core
layout(push_constant = 1) uniform P { float x; } p;
void main() { gl_Position = vec4(p.x); }
`,
			stage: ir.StageVertex,
			want:  "Qualifier doesn't expect a value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.stage == ir.StageCompute {
				// The stage gate runs before the parser sees a token; the
				// message is the one glslang prints for the #version line.
				parseErr(t, tt.source, Options{Stage: tt.stage}, tt.want)
				return
			}
			parseErr(t, tt.source, Options{Stage: tt.stage}, tt.want)
		})
	}
}

// TestIOFloorLocations: the four IO classes of the two dialects. glslang
// accepts `layout(location = 0) out vec4 color;` in a 330 and a 300 es
// fragment shader and `layout(location = 2) in vec3 pos;` in a 330 and a 300 es
// vertex shader, so explicit locations are taken as written; an unlocated
// output still gets the sequential slot (design §10).
func TestIOFloorLocations(t *testing.T) {
	vertexInput := `#version %s
layout(location = 2) in vec3 pos;
void main() { gl_Position = vec4(pos, 1.0); }
`
	for _, header := range []string{"330 core", "300 es"} {
		t.Run("vertex input "+header, func(t *testing.T) {
			mod, _ := parseOK(t, strings.Replace(vertexInput, "%s", header, 1), Options{Stage: ir.StageVertex})
			validateOK(t, mod)
			var location uint32
			found := false
			for _, argument := range mod.EntryPoints[0].Function.Arguments {
				if argument.Name != "pos" {
					continue
				}
				binding, ok := (*argument.Binding).(ir.LocationBinding)
				if !ok {
					t.Fatalf("pos binding = %T, want a location binding", *argument.Binding)
				}
				location, found = binding.Location, true
			}
			if !found {
				t.Fatalf("the entry point has no argument named pos")
			}
			if location != 2 {
				t.Fatalf("pos location = %d, want the written 2", location)
			}
		})
	}

	fragmentOutput := `#version %s
precision mediump float;
layout(location = 1) out vec4 color;
void main() { color = vec4(1.0); }
`
	// GL 3.30 core takes the qualifier; ES 3.00 refuses it (the refusal is in
	// TestDialectGates), so only the desktop dialect is asserted here.
	{
		header := "330 core"
		t.Run("fragment output "+header, func(t *testing.T) {
			mod, _ := parseOK(t, strings.Replace(fragmentOutput, "%s", header, 1), Options{Stage: ir.StageFragment})
			validateOK(t, mod)
			entry := mod.EntryPoints[0]
			result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
			if len(result.Members) != 1 {
				t.Fatalf("result members = %d, want one", len(result.Members))
			}
			binding, ok := (*result.Members[0].Binding).(ir.LocationBinding)
			if !ok {
				t.Fatalf("color binding = %T, want a location binding", *result.Members[0].Binding)
			}
			if binding.Location != 1 {
				t.Fatalf("color location = %d, want the written 1", binding.Location)
			}
		})
	}
}

// TestES300DefaultFloatPrecision is the precision table (GLSL ES 3.00 §4.5.3):
// the fragment language has no default precision for float, so the first
// declared float type without a default needs the statement. glslang:
// "'float' : type requires declaration of default precision qualifier"
// (frag300_noprec.frag), at the first float use in source order
// (prec_after.frag, prec_fn_before.frag), while a shader with no float type
// (noprec_nofloat.frag), a vertex shader (GLSL ES 3.00 §4.5.3 default highp)
// and a declaration naming its own precision (noprec_int.frag's int statement
// does not count) follow the oracle.
func TestES300DefaultFloatPrecision(t *testing.T) {
	const message = "'float' : type requires declaration of default precision qualifier"

	t.Run("missing in a fragment shader", func(t *testing.T) {
		parseErr(t, `#version 300 es
out vec4 color;
void main() { color = vec4(1.0); }
`, Options{Stage: ir.StageFragment}, message)
	})

	t.Run("present", func(t *testing.T) {
		mod, _ := parseOK(t, es300Fragment, Options{Stage: ir.StageFragment})
		validateOK(t, mod)
	})

	t.Run("no float type needs none", func(t *testing.T) {
		// noprec_nofloat.frag: accepted, because no float type is declared.
		mod, _ := parseOK(t, `#version 300 es
out uvec4 color;
void main() { color = uvec4(1, 0, 0, 1); }
`, Options{Stage: ir.StageFragment})
		validateOK(t, mod)
	})

	t.Run("explicit precision on the declaration", func(t *testing.T) {
		// A declaration that names its own precision does not need the default.
		mod, _ := parseOK(t, `#version 300 es
out highp vec4 color;
void main() { color = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
		validateOK(t, mod)
	})

	t.Run("local precision covers its block only", func(t *testing.T) {
		// p_scope.frag: the local statement covers the block, not the outer
		// declaration.
		parseErr(t, `#version 300 es
out uvec4 color;
void main() {
    { precision mediump float; float x = 1.0; color = uvec4(int(x)); }
    float y = 2.0; color = uvec4(int(y));
}
`, Options{Stage: ir.StageFragment}, message)
	})

	t.Run("statement after the first use", func(t *testing.T) {
		// prec_after.frag: the statement must precede the use.
		parseErr(t, `#version 300 es
out vec4 color;
precision mediump float;
void main() { color = vec4(1.0); }
`, Options{Stage: ir.StageFragment}, message)
	})

	t.Run("vertex has a default", func(t *testing.T) {
		// The vertex language's default float precision is highp, so no
		// statement is needed.
		mod, _ := parseOK(t, `#version 300 es
out vec3 v;
void main() { v = vec3(1.0); gl_Position = vec4(1.0); }
`, Options{Stage: ir.StageVertex})
		validateOK(t, mod)
	})

	t.Run("desktop ignores precision", func(t *testing.T) {
		// 330 core accepts and ignores precision statements, so no statement
		// is needed.
		mod, _ := parseOK(t, `#version 330 core
out vec4 color;
void main() { color = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
		validateOK(t, mod)
	})
}

// TestExtensionBehaviour is the #extension matrix of design §15. glslang, for a
// name it does not support (ext_unknown_330_*.frag, ext_unknown_300_*.frag):
// `require` is an error, `enable`, `warn` and `disable` are warnings with the
// same text; `#extension all : require` is an error ("extension 'all' cannot
// have 'require' or 'enable' behavior", ext_all_req.frag) and
// `all : disable`/`all : warn` are accepted silently (ext_all_warn.frag).
func TestExtensionBehaviour(t *testing.T) {
	const header = "#version 330 core\nout vec4 color;\n"
	t.Run("require unknown is a warning", func(t *testing.T) {
		// A deliberate deviation from GLSL 4.60 §3.3 (card P2-GLSLIN-022): the
		// frontend cannot enumerate the registry, so a required name it does not
		// know is accepted as a no-op with a warning. glslang errors here
		// (ext_unknown_330_require.frag); the deviation is recorded.
		_, info := parseOK(t, header+"#extension GL_ARB_not_a_real_extension : require\nvoid main() { color = vec4(1.0); }\n",
			Options{Stage: ir.StageFragment})
		warnings := strings.Join(warnMessages(info), "\n")
		if !strings.Contains(warnings, "GL_ARB_not_a_real_extension") ||
			!strings.Contains(warnings, "P2-GLSLIN-022") {
			t.Fatalf("warnings = %q, want the accepted no-op with the deviation recorded", warnings)
		}
	})
	for _, behavior := range []string{"enable", "warn", "disable"} {
		t.Run(behavior+" unknown is a warning", func(t *testing.T) {
			_, info := parseOK(t, header+"#extension GL_ARB_not_a_real_extension : "+behavior+"\nvoid main() { color = vec4(1.0); }\n",
				Options{Stage: ir.StageFragment})
			warnings := strings.Join(warnMessages(info), "\n")
			if !strings.Contains(warnings, "GL_ARB_not_a_real_extension") {
				t.Fatalf("warnings = %q, want the unsupported extension reported", warnings)
			}
		})
	}
	t.Run("require a known extension", func(t *testing.T) {
		_, info := parseOK(t, header+"#extension GL_EXT_shader_explicit_arithmetic_types_float16 : require\nvoid main() { color = vec4(1.0); }\n",
			Options{Stage: ir.StageFragment})
		if len(info.Warnings) != 0 {
			t.Fatalf("warnings = %v, want none for a known extension", warnMessages(info))
		}
	})
	t.Run("subgroup names are listed exactly", func(t *testing.T) {
		_, info := parseOK(t, header+"#extension GL_KHR_shader_subgroup_ballot : require\nvoid main() { color = vec4(1.0); }\n",
			Options{Stage: ir.StageFragment})
		if len(info.Warnings) != 0 {
			t.Fatalf("warnings = %v, want none for a known extension", warnMessages(info))
		}
		// A typo must not ride on a prefix match.
		_, typo := parseOK(t, header+"#extension GL_KHR_shader_subgroup_balot : require\nvoid main() { color = vec4(1.0); }\n",
			Options{Stage: ir.StageFragment})
		if !strings.Contains(strings.Join(warnMessages(typo), "\n"), "subgroup_balot") {
			t.Fatalf("warnings = %v, want the misspelled name reported", warnMessages(typo))
		}
	})
	t.Run("all require is an error", func(t *testing.T) {
		parseErr(t, header+"#extension all : require\nvoid main() { color = vec4(1.0); }\n",
			Options{Stage: ir.StageFragment}, "extension 'all' cannot have 'require' or 'enable' behavior")
	})
	t.Run("all disable is accepted", func(t *testing.T) {
		_, info := parseOK(t, header+"#extension all : disable\nvoid main() { color = vec4(1.0); }\n",
			Options{Stage: ir.StageFragment})
		if len(info.Warnings) != 0 {
			t.Fatalf("warnings = %v, want none", warnMessages(info))
		}
	})
	t.Run("Vulkan gets no extension diagnostics", func(t *testing.T) {
		// Rust naga never examines the directive, and the 440/450/460 path stays
		// identical: GL_EXT_control_flow_attributes is accepted by glslang and by
		// Rust (cfattr450.frag).
		_, info := parseOK(t, `#version 450
#extension GL_EXT_control_flow_attributes : require
layout(location = 0) out vec4 color;
void main() { color = vec4(1.0); }
`, Options{Stage: ir.StageFragment})
		if len(info.Warnings) != 0 {
			t.Fatalf("warnings = %v, want none on the Vulkan path", warnMessages(info))
		}
	})
}

// TestPushConstantValueRejectedOnVulkan pins the base behavior on the path the
// dialect work must not change: a value on a qualifier that takes none is Rust
// naga's "Qualifier doesn't expect a value", never an immediate-space global.
func TestPushConstantValueRejectedOnVulkan(t *testing.T) {
	parseErr(t, `#version 450
layout(push_constant = 1) uniform P { float x; } p;
layout(location = 0) out vec4 color;
void main() { color = vec4(p.x); }
`, Options{Stage: ir.StageFragment}, "Qualifier doesn't expect a value")
}

// TestLocalPrecisionStatementInGLES: the GLSL 4.60 and GLSL ES 3.00 languages
// allow a precision statement in a nested scope (§4.5.4; glslang accepts it in
// 330 core and 450), and the GL/ES dialects parse one. Vulkan GLSL keeps Rust
// naga's parse error (TestPrecisionStatementInAFunctionIsRejected), which the
// 440/450/460 path must not change.
func TestLocalPrecisionStatementInGLES(t *testing.T) {
	_, _ = parseOK(t, `#version 330 core
out vec4 color;
void main() {
    { precision mediump float; float x = 1.0; color = vec4(x); }
}
`, Options{Stage: ir.StageFragment})
	_, _ = parseOK(t, `#version 300 es
precision mediump float;
out vec4 color;
void main() {
    { precision lowp float; float x = 1.0; color = vec4(x); }
}
`, Options{Stage: ir.StageFragment})
}

// TestGL330TextureLimitsStayForVulkan keeps the 440 path unchanged: the same
// textureGather and layout(binding) sources parse in Vulkan GLSL, where both
// builtins and the qualifier exist.
func TestGL330TextureLimitsStayForVulkan(t *testing.T) {
	mod, _ := parseOK(t, `#version 450
layout(set = 0, binding = 0) uniform sampler2D tex;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 color;
void main() {
    color = textureGather(tex, uv, 0) + vec4(float(textureQueryLevels(tex)));
}
`, Options{Stage: ir.StageFragment})
	validateOK(t, mod)
}

// TestGL330Float16Extension is design §15's "with extension" cell: float16_t
// needs GL_EXT_shader_explicit_arithmetic_types_float16 in GL 3.30 core, and
// the corpus file gl330/float16-extension.frag is accepted by glslang with the
// directive required. Without the directive the type is refused (TestDialectGates),
// and ES 3.00 refuses it either way.
func TestGL330Float16Extension(t *testing.T) {
	const header = `#version 330 core
#extension GL_EXT_shader_explicit_arithmetic_types_float16 : require
out vec4 color;
`
	// The parse itself is the assertion: the type gate lets float16_t through
	// once the directive is required.
	parseOK(t, header+`void main() { float16_t x = float16_t(1.0); color = vec4(float(x)); }
`, Options{Stage: ir.StageFragment})

	// A later `#extension all : disable` turns it off again.
	parseErr(t, `#version 330 core
#extension GL_EXT_shader_explicit_arithmetic_types_float16 : require
#extension all : disable
out vec4 color;
void main() { float16_t x = float16_t(1.0); color = vec4(float(x)); }
`, Options{Stage: ir.StageFragment}, "'float16_t' : not supported by GLSL 3.30 core")
}

// TestES300PrecisionSamplerType: `precision lowp sampler2D;` is a valid ES 3.00
// declaration (GLSL ES 3.00 §4.5.4; glslang accepts prec_sampler.frag) and must
// not be read as a combined-sampler variable.
func TestES300PrecisionSamplerType(t *testing.T) {
	mod, _ := parseOK(t, `#version 300 es
precision lowp sampler2D;
precision mediump float;
uniform sampler2D tex;
out vec4 color;
void main() { color = texture(tex, vec2(0.5)); }
`, Options{Stage: ir.StageFragment})
	validateOK(t, mod)
}

// TestPrecisionStatementTypes is the precision statement's type rule:
// glslang "cannot apply precision statement to this type; use 'float', 'int'
// or a sampler type" for vec4, mat4, bool and uint (prec_vec.frag,
// prec_mat.frag, prec_bool.frag, prec_uint.frag), while sampler2D passes.
func TestPrecisionStatementTypes(t *testing.T) {
	const header = "#version 450\n"
	bad := []string{"vec4", "mat4", "bool", "uint"}
	for _, name := range bad {
		t.Run(name, func(t *testing.T) {
			parseErr(t, header+"precision highp "+name+";\nvoid main() {}\n",
				Options{Stage: ir.StageVertex}, "Precision statement can only work on floats, ints and sampler types")
		})
	}
	for _, name := range []string{"float", "int", "sampler2D"} {
		t.Run(name, func(t *testing.T) {
			parseOK(t, header+"precision highp "+name+";\nvoid main() {}\n", Options{Stage: ir.StageVertex})
		})
	}
}

// TestMemberPrecisionQualifiers: a struct or block member may name its own
// precision (GLSL ES 3.00 §4.5.4), which the parser accepts and discards.
func TestMemberPrecisionQualifiers(t *testing.T) {
	_, _ = parseOK(t, `#version 300 es
struct S { highp vec4 a; mediump float b; };
precision mediump float;
uniform B { highp vec4 a; lowp int i; } b;
out vec4 color;
void main() { S s = S(vec4(a), b.a.x); color = s.a + vec4(s.b); }
`, Options{Stage: ir.StageFragment})
}

// TestSubgroupBuiltinsRefused: the subgroup forms the IR or a text backend
// cannot express are refused with a message naming the card that implements
// the missing IR capability rather than an unknown-name text; an unimplemented
// form reaches that message only after its family's extension is enabled, as
// glslang resolves availability first. The ballot bit operations are
// implemented (card P2-IR-039) and available in the GL 3.30 core dialect
// through the ballot extension, as glslang accepts them there (probed
// 2026-11-17). The subgroup built-in variables are implemented and have their
// own tests (subgroup_test.go).
func TestSubgroupBuiltinsRefused(t *testing.T) {
	const header = `#version 330 core
#extension GL_KHR_shader_subgroup_basic : require
#extension GL_KHR_shader_subgroup_ballot : require
#extension GL_KHR_shader_subgroup_arithmetic : require
#extension GL_KHR_shader_subgroup_clustered : require
layout(location = 0) out vec4 color;
`
	t.Run("builtin function", func(t *testing.T) {
		parseErr(t, header+`void main() { color = vec4(float(subgroupInclusiveMin(1u))); }
`, Options{Stage: ir.StageFragment}, "card P1-GLSLIN-017")
	})
	t.Run("ballot bit count accepted in GL 3.30", func(t *testing.T) {
		parseOK(t, header+`void main() { color = vec4(float(subgroupBallotBitCount(subgroupBallot(true)))); }
`, Options{Stage: ir.StageFragment})
	})
	t.Run("clustered add accepted in GL 3.30", func(t *testing.T) {
		parseOK(t, header+`void main() { color = vec4(float(subgroupClusteredAdd(1u, 4u))); }
`, Options{Stage: ir.StageFragment})
	})
}
