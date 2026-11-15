package front

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

// TestParseMinimalFragment is the first end-to-end check of the frontend: a
// #version 450 fragment shader with one output, a constructor and an
// assignment lowers to a module the validator accepts.
func TestParseMinimalFragment(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 color;
void main() {
    color = vec4(1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	if len(mod.EntryPoints) != 1 || mod.EntryPoints[0].Stage != ir.StageFragment {
		t.Fatalf("entry points = %+v, want one fragment entry point", mod.EntryPoints)
	}
	if len(mod.EntryPoints[0].Function.Arguments) != 0 {
		t.Fatalf("the fragment entry point has %d arguments, want none", len(mod.EntryPoints[0].Function.Arguments))
	}
	if mod.EntryPoints[0].Function.Result == nil {
		t.Fatal("the fragment entry point has no result, want the output struct")
	}
}

// TestParseMinimalVertexIO checks an input argument and a builtin output: the
// position becomes a builtin member of the result struct and the location
// input an entry-point argument.
func TestParseMinimalVertexIO(t *testing.T) {
	source := `#version 450
layout(location = 0) in vec3 pos;
void main() {
    gl_Position = vec4(pos, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	entry := mod.EntryPoints[0]
	if len(entry.Function.Arguments) != 1 {
		t.Fatalf("entry point has %d arguments, want 1 (pos)", len(entry.Function.Arguments))
	}
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	foundPosition := false
	for _, member := range result.Members {
		if binding, ok := (*member.Binding).(ir.BuiltinBinding); ok && binding.Builtin == ir.BuiltinPosition {
			foundPosition = true
		}
	}
	if !foundPosition {
		t.Fatalf("the result struct has no Position member: %+v", result.Members)
	}
}

// TestParseUniformBlock checks a module-scope uniform block: the instance name
// reaches the block global and the member keeps its std140 offset.
func TestParseUniformBlock(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform Globals {
    vec4 color;
    vec3 tint;
} globals;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = globals.color + vec4(globals.tint, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	block := findGlobal(t, mod, "globals")
	if block.Space != ir.SpaceUniform {
		t.Fatalf("block space = %v, want uniform", block.Space)
	}
	if block.Binding == nil || block.Binding.Group != 0 || block.Binding.Binding != 0 {
		t.Fatalf("block binding = %+v, want set 0 binding 0", block.Binding)
	}
	st := mod.Types[block.Type].Inner.(ir.StructType)
	if st.Members[0].Offset != 0 || st.Members[1].Offset != 16 {
		t.Fatalf("member offsets = %d, %d; want std140 0 and 16", st.Members[0].Offset, st.Members[1].Offset)
	}
}

// TestParseBlockWithoutInstance checks that a block without an instance name
// publishes its members as module-scope names.
func TestParseBlockWithoutInstance(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform Globals {
    vec4 color;
};
layout(location = 0) out vec4 outColor;
void main() {
    outColor = color;
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
}

// findGlobal returns the global variable of a name.
func findGlobal(t *testing.T, mod *ir.Module, name string) ir.GlobalVariable {
	t.Helper()
	for _, g := range mod.GlobalVariables {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("no global named %q", name)
	return ir.GlobalVariable{}
}

// TestParseCombinedSamplerSplit checks the design §11 declaration rule: a
// combined sampler becomes an image global on its declared binding and a
// sampler global of its own, reported in ParseInfo.
func TestParseCombinedSamplerSplit(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 1) uniform sampler2D tex;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(1.0);
}
`
	mod, info, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	image := findGlobal(t, mod, "tex")
	if image.Space != ir.SpaceHandle || image.Binding == nil || *image.Binding != (ir.ResourceBinding{Group: 0, Binding: 1}) {
		t.Fatalf("image global = %+v, want handle space at (0, 1)", image)
	}
	sampler := findGlobal(t, mod, "tex_sampler")
	if sampler.Space != ir.SpaceHandle || sampler.Binding == nil {
		t.Fatalf("sampler global = %+v, want a bound handle-space global", sampler)
	}
	if sampler.Binding.Group != 0 || sampler.Binding.Binding == 1 {
		t.Fatalf("sampler binding = %+v, want another binding of group 0", sampler.Binding)
	}
	if len(info.CombinedSamplers) != 1 {
		t.Fatalf("ParseInfo.CombinedSamplers = %+v, want one split", info.CombinedSamplers)
	}
	split := info.CombinedSamplers[0]
	if split.Image != (ir.ResourceBinding{Group: 0, Binding: 1}) || split.Split != *sampler.Binding {
		t.Fatalf("reported split = %+v, want image (0,1) and the sampler's binding", split)
	}

	// A supplied sampler binding wins over the next free one.
	_, info2, err2 := Parse(source, Options{
		Stage: ir.StageFragment,
		SamplerBindings: map[ir.ResourceBinding]ir.ResourceBinding{
			{Group: 0, Binding: 1}: {Group: 0, Binding: 7},
		},
	})
	if err2 != nil {
		t.Fatalf("Parse with SamplerBindings: %v", err2)
	}
	if got := info2.CombinedSamplers[0].Split; got != (ir.ResourceBinding{Group: 0, Binding: 7}) {
		t.Fatalf("sampler binding = %+v, want the supplied (0, 7)", got)
	}
}

// TestParseIOLocations checks design §10's location rule: an explicit location
// is kept, and an unlocated variable of the same class takes a free one
// instead of Rust's colliding 0.
func TestParseIOLocations(t *testing.T) {
	source := `#version 450
layout(location = 1) out vec4 a;
out vec4 b;
void main() {
    a = vec4(1.0);
    b = vec4(2.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	entry := mod.EntryPoints[0]
	result := mod.Types[entry.Function.Result.Type].Inner.(ir.StructType)
	locations := map[string]uint32{}
	for _, member := range result.Members {
		if binding, ok := (*member.Binding).(ir.LocationBinding); ok {
			locations[member.Name] = binding.Location
		}
	}
	if locations["a"] != 1 {
		t.Fatalf("a's location = %d, want the declared 1", locations["a"])
	}
	if locations["b"] != 0 {
		t.Fatalf("b's location = %d, want the free 0 (1 is taken)", locations["b"])
	}
}

// TestParseTextureThroughCombinedSampler is the Phase 1 acceptance shape: a
// fragment shader samples a combined image sampler, which the frontend split
// into an image and a sampler, and the module validates.
func TestParseTextureThroughCombinedSampler(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform sampler2D tex;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = texture(tex, uv);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	// The sample expression names the image global and the sampler global.
	// GLSL main is an ordinary function, so the sample lives in its arena.
	functions := append([]ir.Function{}, mod.Functions...)
	for _, entry := range mod.EntryPoints {
		functions = append(functions, entry.Function)
	}
	hasSample := false
	for _, function := range functions {
		for _, expr := range function.Expressions {
			sample, ok := expr.Kind.(ir.ExprImageSample)
			if !ok {
				continue
			}
			image, ok := function.Expressions[sample.Image].Kind.(ir.ExprGlobalVariable)
			if !ok || mod.GlobalVariables[image.Variable].Name != "tex" {
				t.Fatalf("the sample's image is %#v, want the tex global", function.Expressions[sample.Image].Kind)
			}
			sampler, ok := function.Expressions[sample.Sampler].Kind.(ir.ExprGlobalVariable)
			if !ok || mod.GlobalVariables[sampler.Variable].Name != "tex_sampler" {
				t.Fatalf("the sample's sampler is %#v, want the tex_sampler global", function.Expressions[sample.Sampler].Kind)
			}
			hasSample = true
		}
	}
	if !hasSample {
		t.Fatal("the module has no image sample expression")
	}
}

// TestParseInOutArgumentIsAPointer is review finding P0-1: an out or inout
// argument is re-lowered in Lhs position, so the call passes a pointer and the
// write-back stores through it (Rust functions.rs:789-805). A value expression
// there makes the SPIR-V backend refuse the module and MSL write a temporary.
func TestParseInOutArgumentIsAPointer(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 oColor;
void f(inout vec2 v) { v = v * 2.0; }
void main() {
    vec2 p = vec2(1.0, 2.0);
    f(p);
    oColor = vec4(p, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	if _, _, cerr := spirv.Compile(mod, spirv.Options{}); cerr != nil {
		t.Fatalf("the SPIR-V backend refused the module: %v", cerr)
	}
}

// TestParseStructConstructorConvertsMembers is review finding P0-2: a struct
// constructor converts each argument to its member's type, as Rust's
// constructor_many does (functions.rs:474-510); the IR then carries f32
// components for f32 members even when the source wrote ints.
func TestParseStructConstructorConvertsMembers(t *testing.T) {
	source := `#version 450
layout(location = 0) out vec4 oColor;
struct S { float a; float b; };
void main() {
    S s = S(1, 2);
    oColor = vec4(s.a, s.b, 0.0, 1.0);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}

	// The struct's Compose must carry f32 components: the int literals are
	// converted to the members' type, and the evaluator folds the conversion
	// into f32 literals.
	var compose *ir.ExprCompose
	for _, function := range mod.Functions {
		for i := range function.Expressions {
			kind := function.Expressions[i].Kind
			if k, ok := kind.(ir.ExprCompose); ok {
				if _, isStruct := mod.Types[k.Type].Inner.(ir.StructType); isStruct {
					compose = &k
				}
			}
		}
	}
	if compose == nil {
		t.Fatal("no struct compose expression in the module")
	}
	for i, component := range compose.Components {
		lit, ok := findExpressionKind(mod, component).(ir.Literal)
		if !ok {
			t.Fatalf("component %d is %T, want a folded f32 literal", i, findExpressionKind(mod, component))
		}
		if _, ok := lit.Value.(ir.LiteralF32); !ok {
			t.Fatalf("component %d = %T, want LiteralF32 (the member's type)", i, lit.Value)
		}
	}
}

// findExpressionKind returns the expression kind of a handle, searching the
// module's functions and entry points.
func findExpressionKind(mod *ir.Module, handle ir.ExpressionHandle) ir.ExpressionKind {
	functions := append([]ir.Function{}, mod.Functions...)
	for _, entry := range mod.EntryPoints {
		functions = append(functions, entry.Function)
	}
	for _, function := range functions {
		if int(handle) < len(function.Expressions) {
			return function.Expressions[handle].Kind
		}
	}
	return nil
}

// TestParseClipDistanceReachesResult is review finding P1-4: a dynamic-sized
// builtin array (gl_ClipDistance) is a leaf of the entry-point walk, so the
// wrapper's result struct carries it (Rust functions.rs:1406-1483 falls
// through to the leaf for every array whose size is not a constant).
func TestParseClipDistanceReachesResult(t *testing.T) {
	source := `#version 450
void main() {
    gl_Position = vec4(0.0, 0.0, 0.0, 1.0);
    gl_ClipDistance[0] = 1.0;
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageVertex})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	result := mod.Types[mod.EntryPoints[0].Function.Result.Type].Inner.(ir.StructType)
	found := false
	for _, member := range result.Members {
		if binding, ok := (*member.Binding).(ir.BuiltinBinding); ok && binding.Builtin == ir.BuiltinClipDistances {
			found = true
		}
	}
	if !found {
		t.Fatalf("the result struct has no ClipDistances member: %+v", result.Members)
	}
}

// TestParseRejects checks the loud failures of the public contract: an absent
// #version, an unsupported stage and an invalid version are *ParseErrors.
func TestParseRejects(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stage  ir.ShaderStage
		want   string
	}{
		{"no version", "void main() {}\n", ir.StageFragment, "Missing #version directive"},
		{"bad version", "#version 420\nvoid main() {}\n", ir.StageFragment, "Invalid version: 420"},
		{"task stage", "#version 450\nvoid main() {}\n", ir.StageTask,
			"the GLSL frontend supports the vertex, fragment and compute stages, not task"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.source, Options{Stage: tt.stage})
			if err == nil {
				t.Fatalf("Parse accepted %q", tt.source)
			}
			if err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

// TestConstContextDoesNotGrowGlobalExpressions is review finding P1-5: the
// module's arena holds one expression per constant initializer and nothing
// per constant evaluation. Before the shared const context, every layout
// value, array size and case label appended one reference per visible
// constant — 84 KB of GLSL grew the arena to 1.1M dead expressions and 2.1 GB
// of allocation.
func TestConstContextDoesNotGrowGlobalExpressions(t *testing.T) {
	var source strings.Builder
	source.WriteString("#version 450\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&source, "const int C%d = %d;\n", i, i+2)
	}
	source.WriteString("void main() {\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&source, "  float a%d[C%d];\n", i, i%20)
	}
	source.WriteString("}\n")

	mod, _, err := Parse(source.String(), Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// One initializer per constant; the 200 array sizes resolve through the
	// constants without adding to the arena.
	if len(mod.GlobalExpressions) != 20 {
		t.Fatalf("module carries %d global expressions, want 20 (one per constant)", len(mod.GlobalExpressions))
	}
}

// TestConstructorScalarFromMatrix pins the two-step extract of a matrix to a
// scalar constructor (Rust functions.rs:116-130): the first component of the
// first column, not the column itself. A single extract left a vec2 where the
// scalar belonged, which spirv-val rejects at the store (review finding on
// expressions.frag).
func TestConstructorScalarFromMatrix(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 0) uniform M { mat3 m; } u;
layout(location = 0) out vec4 o;
void main() {
    float f = float(u.m);
    o = vec4(f);
}
`
	mod, _, err := Parse(source, Options{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// The As (the conversion to float) must read an access of an access.
	found := false
	for _, function := range mod.Functions {
		for _, expr := range function.Expressions {
			as, ok := expr.Kind.(ir.ExprAs)
			if !ok || as.Kind != ir.ScalarFloat {
				continue
			}
			outer, ok := function.Expressions[as.Expr].Kind.(ir.ExprAccessIndex)
			if !ok {
				continue
			}
			if _, nested := function.Expressions[outer.Base].Kind.(ir.ExprAccessIndex); nested {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no As over two nested extracts: the matrix-to-scalar constructor extracts only once")
	}
	if _, verr := valid.Validate(mod, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
}
