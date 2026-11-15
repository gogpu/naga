package front

// This file covers review finding F1 of the GLSL Phase 3 review: a
// const-qualified function-local variable is a constant expression, so a load
// of one must reach the constant evaluator as a constant (GLSL 4.50 §4.3.3).
// Rust naga's GLSL frontend has no const-local support to copy — it has no
// textureGather either — so the GLSL specification decides the values here.
//
// What a user sees today without the fix:
//   - const int c = 2; textureGather(s2, uv, c)          "must be a constant expression"
//   - const ivec2 off = ivec2(1,-1); textureGatherOffset(s2, uv, off, 2)  same
//   - const int N = 4; float a[N];                       "Unknown variable: N"

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// constLocalPrelude is a minimal fragment shader around the declaration under
// test: a combined 2D sampler and one color output.
const constLocalPrelude = `#version 450
layout(binding = 0) uniform sampler2D tex;
layout(location = 0) out vec4 color;
`

// parseFragmentConstLocal parses source as a fragment shader, requires the
// module to validate, and returns it.
func parseFragmentConstLocal(t *testing.T, source string) *ir.Module {
	t.Helper()
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{Capabilities: valid.CapAll}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	return mod
}

// gatherComponents returns the component argument of every gather in the
// module, entry-point wrappers and user functions alike, in arena order.
func gatherComponents(t *testing.T, mod *ir.Module) []ir.SwizzleComponent {
	t.Helper()
	fns := append([]ir.Function(nil), mod.Functions...)
	for _, ep := range mod.EntryPoints {
		fns = append(fns, ep.Function)
	}
	var comps []ir.SwizzleComponent
	for i := range fns {
		for _, expr := range fns[i].Expressions {
			if sample, ok := expr.Kind.(ir.ExprImageSample); ok && sample.Gather != nil {
				comps = append(comps, *sample.Gather)
			}
		}
	}
	return comps
}

// constArraySize returns the constant size of the first array type with an
// element type of f32, and whether one exists.
func constArraySize(t *testing.T, mod *ir.Module) (uint32, bool) {
	t.Helper()
	for _, ty := range mod.Types {
		array, ok := ty.Inner.(ir.ArrayType)
		if !ok || array.Size.Constant == nil {
			continue
		}
		base, ok := mod.Types[array.Base].Inner.(ir.ScalarType)
		if ok && base.Kind == ir.ScalarFloat && base.Width == 4 {
			return *array.Size.Constant, true
		}
	}
	return 0, false
}

// TestConstLocalGatherComp is the reported failure: a const-qualified local as
// textureGather's component argument.
func TestConstLocalGatherComp(t *testing.T) {
	mod := parseFragmentConstLocal(t, constLocalPrelude+`void main() {
    const int c = 2;
    color = textureGather(tex, vec2(0.5), c);
}
`)
	comps := gatherComponents(t, mod)
	if len(comps) != 1 {
		t.Fatalf("gather expressions = %d, want 1", len(comps))
	}
	if comps[0] != ir.SwizzleZ {
		t.Fatalf("gather component = %d, want %d (c = 2)", comps[0], ir.SwizzleZ)
	}
}

// TestConstLocalGatherCompInLoop covers the declaration path inside a loop,
// where the initializer is stored at run-time instead of becoming the local's
// initial value. The value is a constant either way, so the component stays
// constant.
func TestConstLocalGatherCompInLoop(t *testing.T) {
	mod := parseFragmentConstLocal(t, constLocalPrelude+`void main() {
    for (int i = 0; i < 1; i++) {
        const int c = 3;
        color = textureGather(tex, vec2(0.5), c);
    }
}
`)
	comps := gatherComponents(t, mod)
	if len(comps) != 1 {
		t.Fatalf("gather expressions = %d, want 1", len(comps))
	}
	if comps[0] != ir.SwizzleW {
		t.Fatalf("gather component = %d, want %d (c = 3)", comps[0], ir.SwizzleW)
	}
}

// TestConstLocalGatherOffset covers a const vector local as
// textureGatherOffset's offset, directly and through a chain of const locals.
// The offset must reach the IR as a constant the backends can read, so each
// case also compiles to SPIR-V (whose offset operand has to be a constant).
func TestConstLocalGatherOffset(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{
			name: "direct",
			source: constLocalPrelude + `void main() {
    const ivec2 off = ivec2(1, -1);
    color = textureGatherOffset(tex, vec2(0.5), off, 2);
}
`,
		},
		{
			name: "chain",
			source: constLocalPrelude + `void main() {
    const ivec2 first = ivec2(1, -1);
    const ivec2 off = first;
    color = textureGatherOffset(tex, vec2(0.5), off, 2);
}
`,
		},
		{
			name: "constructor of const locals",
			source: constLocalPrelude + `void main() {
    const int x = 1;
    const ivec2 off = ivec2(x, -x);
    color = textureGatherOffset(tex, vec2(0.5), off, 2);
}
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mod := parseFragmentConstLocal(t, tt.source)
			comps := gatherComponents(t, mod)
			if len(comps) != 1 {
				t.Fatalf("gather expressions = %d, want 1", len(comps))
			}
			if comps[0] != ir.SwizzleZ {
				t.Fatalf("gather component = %d, want %d", comps[0], ir.SwizzleZ)
			}
			if _, _, err := spirv.Compile(mod, spirv.Options{}); err != nil {
				t.Fatalf("spirv.Compile: %v", err)
			}
		})
	}
}

// TestConstLocalArraySize covers a const as an array size. A module-scope const
// already reaches the module-scope const context; a function-local const must
// too, which is the case GLSL 4.50 §4.3.3 allows and glslang accepts.
func TestConstLocalArraySize(t *testing.T) {
	for _, tt := range []struct {
		name   string
		source string
	}{
		{
			name: "module scope",
			source: `#version 450
const int N = 4;
layout(location = 0) out vec4 color;
void main() {
    float a[N];
    a[0] = 1.0;
    color = vec4(a[0]);
}
`,
		},
		{
			name: "function local",
			source: `#version 450
layout(location = 0) out vec4 color;
void main() {
    const int N = 4;
    float a[N];
    a[0] = 1.0;
    color = vec4(a[0]);
}
`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mod := parseFragmentConstLocal(t, tt.source)
			size, ok := constArraySize(t, mod)
			if !ok {
				t.Fatal("the module has no f32 array type")
			}
			if size != 4 {
				t.Fatalf("array size = %d, want 4", size)
			}
		})
	}
}

// TestConstLocalChain covers one const local initialized from another:
// b's initializer reads a through a load, so recording must happen in
// declaration order and the resolver must recurse.
func TestConstLocalChain(t *testing.T) {
	mod := parseFragmentConstLocal(t, constLocalPrelude+`void main() {
    const int a = 1;
    const int b = a + 1;
    color = textureGather(tex, vec2(0.5), b);
}
`)
	comps := gatherComponents(t, mod)
	if len(comps) != 1 {
		t.Fatalf("gather expressions = %d, want 1", len(comps))
	}
	if comps[0] != ir.SwizzleZ {
		t.Fatalf("gather component = %d, want %d (b = a + 1 = 2)", comps[0], ir.SwizzleZ)
	}
}

// TestConstLocalShadowing checks that a shadowed const keeps the inner value:
// the load resolves to the local the name was declared as, not to the outer
// one.
func TestConstLocalShadowing(t *testing.T) {
	mod := parseFragmentConstLocal(t, constLocalPrelude+`void main() {
    const int c = 1;
    {
        const int c = 2;
        color = textureGather(tex, vec2(0.5), c);
    }
}
`)
	comps := gatherComponents(t, mod)
	if len(comps) != 1 {
		t.Fatalf("gather expressions = %d, want 1", len(comps))
	}
	if comps[0] != ir.SwizzleZ {
		t.Fatalf("gather component = %d, want %d (the inner c = 2)", comps[0], ir.SwizzleZ)
	}
}

// TestMutableLocalStaysRuntime is the negative half: none of the fix applies to
// a mutable local. Its load stays a run-time expression, so the existing
// diagnostics must still fire.
func TestMutableLocalStaysRuntime(t *testing.T) {
	t.Run("gather comp", func(t *testing.T) {
		source := constLocalPrelude + `void main() {
    int c = 2;
    color = textureGather(tex, vec2(0.5), c);
}
`
		_, _, err := Parse(source, Options{Stage: ir.StageFragment})
		if err == nil {
			t.Fatal("Parse accepted a mutable local as textureGather's comp")
		}
		if !strings.Contains(err.Error(), "must be a constant expression") {
			t.Fatalf("error = %q, want the constant-expression diagnostic", err)
		}
	})

	t.Run("array size", func(t *testing.T) {
		source := `#version 450
layout(location = 0) out vec4 color;
void main() {
    int n = 4;
    float a[n];
    a[0] = 1.0;
    color = vec4(a[0]);
}
`
		_, _, err := Parse(source, Options{Stage: ir.StageFragment})
		if err == nil {
			t.Fatal("Parse accepted a mutable local as an array size")
		}
	})
}
