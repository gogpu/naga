package glsl

// The public contract of the synthesized default uniform block (ADR-013, card
// P1-GLSLIN-021): ParseInfo.DefaultBlock, the name a caller keys
// ParseOptions.ResourceBindings with, and the three cases the report is pinned
// on — a module with a plain uniform, one without, and Vulkan GLSL, which
// refuses them.
//
// The layout itself, and its agreement with glslangValidator member for member,
// is checked in the frontend and snapshot packages; these tests check that the
// public mapping carries the report through unchanged.

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/ir"
)

// plainUniformSource is a GL 3.30 fragment shader whose plain uniforms come
// after a sampler, so the block's binding shows the source-order rule.
const plainUniformSource = `#version 330 core
uniform sampler2D tex;
uniform vec2 offset;
uniform float scale;
layout(location = 0) out vec4 color;
void main() {
    color = texture(tex, vec2(0.5)) + vec4(offset * scale, 0.0, 0.0);
}
`

// TestParseDefaultBlockInfo checks the report a caller fills the buffer from.
func TestParseDefaultBlockInfo(t *testing.T) {
	module, info, err := Parse(plainUniformSource, ParseOptions{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if info.DefaultBlock == nil {
		t.Fatal("ParseInfo.DefaultBlock is nil, want the synthesized block")
	}
	block := info.DefaultBlock
	if block.Name != frontend.DefaultBlockName {
		t.Errorf("the block's name is %q, want %q", block.Name, frontend.DefaultBlockName)
	}
	// tex is the module's first resource, so the block, declared next, takes
	// the following slot.
	if want := (ir.ResourceBinding{Group: 0, Binding: 1}); block.Binding != want {
		t.Errorf("the block is at %+v, want %+v", block.Binding, want)
	}
	if block.Size != 16 {
		t.Errorf("the block's size is %d, want 16: offset at 0 (8 bytes), scale at 8, and the span rounds up to the block's 8-byte alignment", block.Size)
	}
	want := []frontend.DefaultBlockMember{
		{Name: "offset", Offset: 0, Size: 8},
		{Name: "scale", Offset: 8, Size: 4},
	}
	if len(block.Members) != len(want) {
		t.Fatalf("%d members, want %d: %+v", len(block.Members), len(want), block.Members)
	}
	for i, member := range block.Members {
		if member != want[i] {
			t.Errorf("member %d is %+v, want %+v", i, member, want[i])
		}
	}

	// The report describes a global of the module: a caller binding the block
	// at the reported slot is binding the module's own resource.
	var found bool
	for _, global := range module.GlobalVariables {
		if global.Name != frontend.DefaultBlockName {
			continue
		}
		found = true
		if global.Binding == nil || *global.Binding != block.Binding {
			t.Errorf("the module's block is bound at %v, want the reported %+v", global.Binding, block.Binding)
		}
	}
	if !found {
		t.Errorf("the module has no %s global", frontend.DefaultBlockName)
	}
}

// TestParseDefaultBlockExplicitBinding checks the explicit placement a caller
// uses when it builds a pipeline layout itself: the entry keyed by
// [frontend.DefaultBlockName] wins, and the automatic counter never takes it.
func TestParseDefaultBlockExplicitBinding(t *testing.T) {
	module, info, err := Parse(plainUniformSource, ParseOptions{
		Stage:            ir.StageFragment,
		ResourceBindings: map[string]ir.ResourceBinding{frontend.DefaultBlockName: {Group: 0, Binding: 5}},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if info.DefaultBlock == nil {
		t.Fatal("ParseInfo.DefaultBlock is nil")
	}
	if want := (ir.ResourceBinding{Group: 0, Binding: 5}); info.DefaultBlock.Binding != want {
		t.Errorf("the block is at %+v, want the supplied %+v", info.DefaultBlock.Binding, want)
	}
	for _, global := range module.GlobalVariables {
		if global.Name != "tex" {
			continue
		}
		if global.Binding == nil || global.Binding.Binding != 0 {
			t.Errorf("tex is at %v, want binding 0: the supplied slot is reserved for the block", global.Binding)
		}
	}
	if len(info.Warnings) != 0 {
		t.Errorf("warnings = %+v, want none: the entry matched the block", info.Warnings)
	}
}

// TestParseDefaultBlockAbsent pins the nil cases: no plain uniform, and Vulkan
// GLSL, where the declaration is refused.
func TestParseDefaultBlockAbsent(t *testing.T) {
	t.Run("no plain uniform", func(t *testing.T) {
		_, info, err := Parse(fragmentSource, ParseOptions{Stage: ir.StageFragment})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if info.DefaultBlock != nil {
			t.Errorf("ParseInfo.DefaultBlock = %+v, want nil", info.DefaultBlock)
		}
	})

	t.Run("Vulkan GLSL refuses it", func(t *testing.T) {
		source := `#version 450
uniform float scale;
void main() { gl_Position = vec4(scale); }
`
		_, info, err := Parse(source, ParseOptions{Stage: ir.StageVertex})
		if err == nil {
			t.Fatal("Parse succeeded, want the Vulkan refusal")
		}
		if !strings.Contains(err.Error(), "not allowed in Vulkan GLSL") {
			t.Errorf("error = %q, want the Vulkan rule", err)
		}
		var parseErr *ParseError
		if !errors.As(err, &parseErr) {
			t.Fatalf("error is %T, want *ParseError", err)
		}
		if info.DefaultBlock != nil {
			t.Errorf("ParseInfo.DefaultBlock = %+v after a refusal, want nil", info.DefaultBlock)
		}
	})
}
