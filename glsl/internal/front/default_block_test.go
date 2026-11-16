package front

// Contracts of the default uniform block the GL 3.30 core and ES 3.00 dialects
// synthesize for their plain (non-block) uniforms (ADR-013, card P1-GLSLIN-021):
// one generated block named gl_DefaultUniformBlock, members in source
// declaration order with std140 offsets, bound through the one-counter model of
// design §11 at the first plain declaration's position or by
// ParseOptions.ResourceBindings. Vulkan GLSL keeps its refusal.
//
// The offsets are cross-checked against glslangValidator member for member in
// snapshot/glsl_plain_uniform_test.go; the assertions here are the frontend's
// own contract, so they hold without an external tool.

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// plainUniformFragment is a GL 3.30 fragment shader whose plain uniforms cover
// the std140 cases: a scalar, a vector, an array, a non-square matrix and a
// four-component vector that has to be pushed past the vec3's 16-byte slot.
const plainUniformFragment = `#version 330 core
uniform float scale;
uniform vec3 tint;
uniform float weights[4];
uniform mat2x3 skew;
uniform vec4 tail;
layout(location = 0) out vec4 color;
void main() {
    color = vec4(scale) + vec4(tint, 1.0) + vec4(weights[0]) + vec4(skew[0], 1.0) + tail;
}
`

// defaultBlockOf returns the module's generated block: its global and its IR
// struct, which every assertion below reads.
func defaultBlockOf(t *testing.T, mod *ir.Module) (ir.GlobalVariable, ir.StructType) {
	t.Helper()
	for _, global := range mod.GlobalVariables {
		if global.Name != frontend.DefaultBlockName {
			continue
		}
		inner, ok := mod.Types[global.Type].Inner.(ir.StructType)
		if !ok {
			t.Fatalf("the block's type is %T, want a struct", inner)
		}
		return global, inner
	}
	t.Fatalf("the module has no %s global", frontend.DefaultBlockName)
	return ir.GlobalVariable{}, ir.StructType{}
}

// TestPlainUniformDefaultBlock is the synthesis of ADR-013: a module's plain
// uniforms become one uniform global whose struct carries them in source order
// with std140 offsets, and every name still resolves inside main.
func TestPlainUniformDefaultBlock(t *testing.T) {
	mod, info := parseOK(t, plainUniformFragment, Options{Stage: ir.StageFragment})
	validateOK(t, mod)

	global, block := defaultBlockOf(t, mod)
	if global.Space != ir.SpaceUniform {
		t.Errorf("the block's space is %v, want uniform", global.Space)
	}
	if global.Binding == nil {
		t.Fatal("the block has no binding")
	}
	if want := (ir.ResourceBinding{Group: 0, Binding: 0}); *global.Binding != want {
		t.Errorf("the block is bound at %+v, want %+v: the first plain declaration takes the first free slot", *global.Binding, want)
	}

	wantMembers := []struct {
		name   string
		offset uint32
		typ    func(ir.TypeInner) bool
	}{
		{"scale", 0, func(inner ir.TypeInner) bool {
			s, ok := inner.(ir.ScalarType)
			return ok && s.Kind == ir.ScalarFloat && s.Width == 4
		}},
		{"tint", 16, func(inner ir.TypeInner) bool {
			v, ok := inner.(ir.VectorType)
			return ok && v.Size == ir.Vec3 && v.Scalar.Kind == ir.ScalarFloat
		}},
		{"weights", 32, func(inner ir.TypeInner) bool {
			a, ok := inner.(ir.ArrayType)
			if !ok || a.Size.Constant == nil || *a.Size.Constant != 4 {
				return false
			}
			return a.Stride == 16
		}},
		{"skew", 96, func(inner ir.TypeInner) bool {
			m, ok := inner.(ir.MatrixType)
			return ok && m.Columns == 2 && m.Rows == ir.Vec3
		}},
		{"tail", 128, func(inner ir.TypeInner) bool {
			v, ok := inner.(ir.VectorType)
			return ok && v.Size == ir.Vec4
		}},
	}
	if len(block.Members) != len(wantMembers) {
		t.Fatalf("%d block members, want %d: %+v", len(block.Members), len(wantMembers), block.Members)
	}
	for i, want := range wantMembers {
		member := block.Members[i]
		if member.Name != want.name {
			t.Errorf("member %d is %q, want %q: source declaration order", i, member.Name, want.name)
		}
		if member.Offset != want.offset {
			t.Errorf("member %q is at offset %d, want %d (std140)", member.Name, member.Offset, want.offset)
		}
		if !want.typ(mod.Types[member.Type].Inner) {
			t.Errorf("member %q has type %T, not the declared one", member.Name, mod.Types[member.Type].Inner)
		}
	}
	if block.Span != 144 {
		t.Errorf("the block's span is %d, want 144 (tail ends at 144, the alignment is 16)", block.Span)
	}

	if info.DefaultBlock == nil {
		t.Fatal("Info.DefaultBlock is nil, want the generated block")
	}
	report := info.DefaultBlock
	if report.Name != frontend.DefaultBlockName {
		t.Errorf("reported name %q, want %q", report.Name, frontend.DefaultBlockName)
	}
	if report.Binding != *global.Binding {
		t.Errorf("reported binding %+v, want the global's %+v", report.Binding, *global.Binding)
	}
	if report.Size != block.Span {
		t.Errorf("reported size %d, want the block's span %d", report.Size, block.Span)
	}
	wantSizes := []uint32{4, 12, 64, 32, 16}
	if len(report.Members) != len(wantSizes) {
		t.Fatalf("%d reported members, want %d", len(report.Members), len(wantSizes))
	}
	for i, size := range wantSizes {
		if report.Members[i].Size != size {
			t.Errorf("reported member %q has size %d, want %d", report.Members[i].Name, report.Members[i].Size, size)
		}
		if report.Members[i].Offset != wantMembers[i].offset {
			t.Errorf("reported member %q is at offset %d, want %d", report.Members[i].Name, report.Members[i].Offset, wantMembers[i].offset)
		}
	}
}

// TestPlainUniformNoPlainUnnamed checks the negative case: a module without a
// plain uniform gets no block, consumes no binding and reports nil.
func TestPlainUniformNoPlainUnnamed(t *testing.T) {
	mod, info := parseOK(t, gl330Vertex, Options{Stage: ir.StageVertex})
	validateOK(t, mod)
	if info.DefaultBlock != nil {
		t.Errorf("Info.DefaultBlock = %+v, want nil: the module declares no plain uniform", info.DefaultBlock)
	}
	for _, global := range mod.GlobalVariables {
		if global.Name == frontend.DefaultBlockName {
			t.Fatal("a module with no plain uniform has the generated block")
		}
	}
	// The counter is untouched: the module's first bound resource takes the
	// first slot, as if the synthesis did not exist. (The in variable is a
	// private global without a binding, so it is not the one to look at.)
	for _, global := range mod.GlobalVariables {
		if global.Binding == nil {
			continue
		}
		if *global.Binding != (ir.ResourceBinding{Group: 0, Binding: 0}) {
			t.Errorf("the module's first bound resource %q is at %+v, want binding 0", global.Name, *global.Binding)
		}
		break
	}
}

// TestPlainUniformBindingPosition pins the counter's source-order rule: the
// block takes the slot the module's counter held at its first plain
// declaration, so a resource declared before it keeps its own and one declared
// after it takes the next.
func TestPlainUniformBindingPosition(t *testing.T) {
	source := `#version 330 core
uniform sampler2D first;
uniform float scale;
uniform sampler2D last;
layout(location = 0) out vec4 color;
void main() {
    color = vec4(scale) + texture(first, vec2(0.5)) + texture(last, vec2(0.5));
}
`
	mod, info := parseOK(t, source, Options{Stage: ir.StageFragment})
	validateOK(t, mod)
	if info.DefaultBlock == nil {
		t.Fatal("Info.DefaultBlock is nil")
	}
	// first is 0, the block is 1 (the first plain declaration's position), last
	// is 2 and the split samplers follow at 3 and 4.
	if want := (ir.ResourceBinding{Group: 0, Binding: 1}); info.DefaultBlock.Binding != want {
		t.Errorf("the block is at %+v, want %+v: its first plain declaration comes after the first sampler", info.DefaultBlock.Binding, want)
	}
	want := map[string]uint32{"first": 0, "last": 2}
	for _, global := range mod.GlobalVariables {
		slot, known := want[global.Name]
		if !known {
			continue
		}
		if global.Binding == nil || global.Binding.Binding != slot {
			t.Errorf("%q is at %v, want binding %d", global.Name, global.Binding, slot)
		}
	}
}

// TestPlainUniformBindingSupplied checks the explicit placement: a
// ResourceBindings entry keyed by the block's name wins over the counter, is
// reported, and keeps the counter off its slot.
func TestPlainUniformBindingSupplied(t *testing.T) {
	source := `#version 330 core
uniform float scale;
uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() { color = vec4(scale) + texture(tex, vec2(0.5)); }
`
	mod, info := parseOK(t, source, Options{
		Stage:            ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{frontend.DefaultBlockName: {Group: 0, Binding: 3}},
	})
	validateOK(t, mod)
	if info.DefaultBlock == nil {
		t.Fatal("Info.DefaultBlock is nil")
	}
	if want := (ir.ResourceBinding{Group: 0, Binding: 3}); info.DefaultBlock.Binding != want {
		t.Errorf("the block is at %+v, want the supplied %+v", info.DefaultBlock.Binding, want)
	}
	for _, global := range mod.GlobalVariables {
		if global.Name != "tex" {
			continue
		}
		if global.Binding == nil || global.Binding.Binding != 0 {
			t.Errorf("tex is at %v, want binding 0: the supplied block slot is reserved, not taken", global.Binding)
		}
	}
	if len(warnMessages(info)) != 0 {
		t.Errorf("warnings = %v, want none: the entry matched the block", warnMessages(info))
	}
}

// TestPlainUniformBindingSuppliedTwice checks that two claims on one slot stay
// a *ParseError: the caller asked for two globals to share a binding.
func TestPlainUniformBindingSuppliedTwice(t *testing.T) {
	source := `#version 330 core
uniform float scale;
uniform sampler2D tex;
layout(location = 0) out vec4 color;
void main() { color = vec4(scale) + texture(tex, vec2(0.5)); }
`
	_, _, err := Parse(source, Options{
		Stage: ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{
			frontend.DefaultBlockName: {Group: 0, Binding: 0},
			"tex":                     {Group: 0, Binding: 0},
		},
	})
	if err == nil {
		t.Fatal("Parse succeeded, want a refusal: the block and tex claim one slot")
	}
	if !strings.Contains(err.Error(), "another resource of the module already uses") {
		t.Errorf("error = %q, want the two-claims message", err)
	}
}

// TestPlainUniformUnmatchedBlockEntry checks the R5 warning: a
// ResourceBindings entry for the block of a module that has no plain uniform
// matches nothing and warns.
func TestPlainUniformUnmatchedBlockEntry(t *testing.T) {
	_, info := parseOK(t, gl330Vertex, Options{
		Stage:            ir.StageVertex,
		ResourceBindings: map[string]ir.ResourceBinding{frontend.DefaultBlockName: {Group: 0, Binding: 0}},
	})
	messages := warnMessages(info)
	if len(messages) != 1 || !strings.Contains(messages[0], frontend.DefaultBlockName) {
		t.Errorf("warnings = %v, want one about the unmatched %s entry", messages, frontend.DefaultBlockName)
	}
}

// TestPlainUniformRefusals pins every refusal of the synthesis.
func TestPlainUniformRefusals(t *testing.T) {
	tests := []struct {
		name   string
		source string
		stage  ir.ShaderStage
		want   string
	}{
		{
			name: "Vulkan GLSL keeps the refusal",
			// glslang: "'non-opaque uniforms outside a block' : not allowed when
			// using GLSL for Vulkan".
			source: `#version 450
uniform float scale;
void main() { gl_Position = vec4(scale); }
`,
			stage: ir.StageVertex,
			want:  "not allowed in Vulkan GLSL",
		},
		{
			name: "layout(std140) on a plain uniform",
			// glslang: "'layout' : cannot specify packing on a variable
			// declaration".
			source: `#version 330 core
layout(std140) uniform float scale;
void main() { gl_Position = vec4(scale); }
`,
			stage: ir.StageVertex,
			want:  "cannot specify packing on a variable declaration",
		},
		{
			name: "layout(binding) on a plain uniform",
			source: `#version 330 core
layout(binding = 0) uniform float scale;
void main() { gl_Position = vec4(scale); }
`,
			stage: ir.StageVertex,
			want:  "'binding' : not supported by GLSL 3.30 core",
		},
		{
			name: "an array of unknown length",
			// glslang accepts `uniform float w[];`, but the descriptor model has
			// no length to give the buffer: a recorded deviation.
			source: `#version 330 core
uniform float w[];
void main() { gl_Position = vec4(w[0]); }
`,
			stage: ir.StageVertex,
			want:  "array of unknown length",
		},
		{
			name: "a repeated plain uniform name",
			// glslang: "'a' : redefinition".
			source: `#version 330 core
uniform float a;
uniform float a;
void main() { gl_Position = vec4(a); }
`,
			stage: ir.StageVertex,
			want:  "redefinition",
		},
		{
			name: "the block's own name",
			// glslang: "'gl_DefaultUniformBlock' : identifiers starting with
			// "gl_" are reserved".
			source: `#version 330 core
uniform float gl_DefaultUniformBlock;
void main() { gl_Position = vec4(gl_DefaultUniformBlock); }
`,
			stage: ir.StageVertex,
			want:  "reserved for the block",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parseErr(t, tt.source, Options{Stage: tt.stage}, tt.want)
		})
	}
}

// TestPlainUniformES300 checks the ES dialect: the same synthesis, with the
// precision statements the dialect requires.
func TestPlainUniformES300(t *testing.T) {
	source := `#version 300 es
precision highp float;
uniform float scale;
uniform vec2 offset;
out vec4 color;
void main() { color = vec4(scale, offset, 1.0); }
`
	mod, info := parseOK(t, source, Options{Stage: ir.StageFragment})
	validateOK(t, mod)
	if info.DefaultBlock == nil {
		t.Fatal("Info.DefaultBlock is nil")
	}
	global, block := defaultBlockOf(t, mod)
	if global.Space != ir.SpaceUniform {
		t.Errorf("the block's space is %v, want uniform", global.Space)
	}
	if len(block.Members) != 2 || block.Members[0].Name != "scale" || block.Members[1].Name != "offset" {
		t.Fatalf("members = %+v, want scale then offset", block.Members)
	}
	if block.Members[1].Offset != 8 {
		t.Errorf("offset is at %d, want 8: a vec2 follows a float at its 8-byte alignment", block.Members[1].Offset)
	}
	if block.Span != 16 {
		t.Errorf("the block's span is %d, want 16", block.Span)
	}
}

// TestPlainUniformStructMember checks that a struct type is a member like any
// other, laid out by its own std140 rules, and that its fields are reachable
// through the uniform's name.
func TestPlainUniformStructMember(t *testing.T) {
	source := `#version 330 core
struct Light {
    vec2 pos;
    float power;
};
uniform Light light;
uniform float gain;
void main() { gl_Position = vec4(light.pos, light.power * gain, 1.0); }
`
	mod, info := parseOK(t, source, Options{Stage: ir.StageVertex})
	validateOK(t, mod)
	_, block := defaultBlockOf(t, mod)
	if len(block.Members) != 2 {
		t.Fatalf("members = %+v, want light and gain", block.Members)
	}
	if block.Members[0].Offset != 0 {
		t.Errorf("light is at %d, want 0", block.Members[0].Offset)
	}
	// A struct member's std140 size is carried by its own span: pos at 0, power
	// at 8, rounded up to the struct's 8-byte alignment (16 in std140, which
	// rounds a struct's alignment to a vec4's).
	if info.DefaultBlock.Members[0].Size != 16 {
		t.Errorf("light's std140 size is %d, want 16", info.DefaultBlock.Members[0].Size)
	}
	if block.Members[1].Offset != 16 {
		t.Errorf("gain is at %d, want 16: std140 rounds a struct member's alignment up to 16", block.Members[1].Offset)
	}
}

// TestPlainUniformBoolMember checks the bool rule: a bool member occupies four
// bytes in std140 (glslang puts `bool a; bool b;` at 0 and 4), which is not the
// IR width of a bool.
func TestPlainUniformBoolMember(t *testing.T) {
	source := `#version 330 core
uniform bool flag;
uniform bool other;
uniform float value;
void main() { gl_Position = vec4(float(flag), float(other), value, 1.0); }
`
	mod, info := parseOK(t, source, Options{Stage: ir.StageVertex})
	_, block := defaultBlockOf(t, mod)
	wantOffsets := []uint32{0, 4, 8}
	wantSizes := []uint32{4, 4, 4}
	for i, want := range wantOffsets {
		if block.Members[i].Offset != want {
			t.Errorf("member %q is at %d, want %d", block.Members[i].Name, block.Members[i].Offset, want)
		}
		if info.DefaultBlock.Members[i].Size != wantSizes[i] {
			t.Errorf("member %q has std140 size %d, want %d", block.Members[i].Name, info.DefaultBlock.Members[i].Size, wantSizes[i])
		}
	}
	// The member keeps its IR bool type: the synthesis carries the declared type,
	// and what a backend makes of a bool block member is the backend's rule.
	if inner, ok := mod.Types[block.Members[0].Type].Inner.(ir.ScalarType); !ok || inner.Kind != ir.ScalarBool {
		t.Errorf("the flag member's type is %T, want a bool scalar", mod.Types[block.Members[0].Type].Inner)
	}
}
