package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// FuzzParseGLSLModule runs the whole frontend — preprocessor, parser and
// lowering — over arbitrary input: the pipeline the public glsl.Parse exposes
// (design §17). It complements FuzzParseGLSL, which stops at the syntax layer,
// and asserts only that untrusted input never panics and that a produced module
// is structurally sound enough to type.
func FuzzParseGLSLModule(f *testing.F) {
	seeds := []string{
		"#version 450\nvoid main() {}\n",
		"#version 450\nlayout(location = 0) in vec3 p;\nlayout(location = 0) out vec4 c;\nvoid main() { c = vec4(p, 1.0); }\n",
		"#version 450\nlayout(set = 0, binding = 0) uniform B { vec4 c; } b;\nvoid main() { gl_Position = b.c; }\n",
		"#version 450\nlayout(set = 0, binding = 1) uniform sampler2D t;\nlayout(location = 0) out vec4 c;\nlayout(location = 0) in vec2 uv;\nvoid main() { c = texture(t, uv); }\n",
		"#version 450\nstruct S { float x; };\nvoid main() { S s; s.x = 1.0; }\n",
		"#version 450\nvoid main() { int i = 0; while (i < 4) { i++; } switch (i) { case 0: break; default: break; } }\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	stages := [...]ir.ShaderStage{ir.StageVertex, ir.StageFragment, ir.StageCompute}
	f.Fuzz(func(t *testing.T, src string) {
		// The stage is derived from the input so one fuzz target covers all
		// three stages: f.Fuzz may be called only once per function.
		stage := stages[len(src)%len(stages)]
		// Parse must not panic on any input, and a module it returns must
		// have an entry point.
		module, _, err := Parse(src, Options{Stage: stage})
		if err != nil {
			return
		}
		if module == nil || len(module.EntryPoints) == 0 {
			t.Fatalf("Parse returned no module or no entry point for %q", src)
		}
	})
}
