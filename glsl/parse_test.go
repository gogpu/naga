package glsl

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
	"github.com/gogpu/naga/valid"
)

const fragmentSource = `#version 450
layout(set = 0, binding = 0) uniform Globals {
    vec4 color;
} globals;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = globals.color;
}
`

// TestParseFragment checks the public entry point end to end: the module
// validates and the entry point carries the output.
func TestParseFragment(t *testing.T) {
	module, info, err := Parse(fragmentSource, ParseOptions{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, verr := valid.Validate(module, valid.Options{}); verr != nil {
		t.Fatalf("the parsed module does not validate: %v", verr)
	}
	if len(module.EntryPoints) != 1 {
		t.Fatalf("module has %d entry points, want 1", len(module.EntryPoints))
	}
	if len(info.Warnings) != 0 {
		t.Fatalf("warnings = %+v, want none", info.Warnings)
	}
}

// TestParseCombinedSamplerInfo checks the split report a caller needs to bind
// descriptors: it survives the public mapping.
func TestParseCombinedSamplerInfo(t *testing.T) {
	source := `#version 450
layout(set = 0, binding = 2) uniform sampler2D tex;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = vec4(1.0);
}
`
	_, info, err := Parse(source, ParseOptions{Stage: ir.StageFragment})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(info.CombinedSamplers) != 1 {
		t.Fatalf("CombinedSamplers = %+v, want one", info.CombinedSamplers)
	}
	split := info.CombinedSamplers[0]
	if split.Image != (ir.ResourceBinding{Group: 0, Binding: 2}) {
		t.Fatalf("image binding = %+v, want (0, 2)", split.Image)
	}
	if split.Sampler != split.Image {
		t.Fatalf("the declaration's binding = %+v, want the image's (0, 2)", split.Sampler)
	}
	if split.Split == split.Image {
		t.Fatalf("the sampler kept the image's binding %+v, want its own", split.Split)
	}

	// A supplied sampler binding wins.
	_, info, err = Parse(source, ParseOptions{
		Stage: ir.StageFragment,
		SamplerBindings: frontend.SamplerBindings{
			{Group: 0, Binding: 2}: {Group: 0, Binding: 5},
		},
	})
	if err != nil {
		t.Fatalf("Parse with SamplerBindings: %v", err)
	}
	if got := info.CombinedSamplers[0].Split; got != (ir.ResourceBinding{Group: 0, Binding: 5}) {
		t.Fatalf("sampler binding = %+v, want the supplied (0, 5)", got)
	}
}

// TestParsedModulesEmit is the Phase 1 acceptance shape (design §19): a
// vertex/fragment pair with a uniform block and a combined sampler parses,
// validates and every text backend emits it.
func TestParsedModulesEmit(t *testing.T) {
	vertex := `#version 450
layout(set = 0, binding = 0) uniform Globals {
    mat4 transform;
} globals;
layout(location = 0) in vec3 pos;
void main() {
    gl_Position = globals.transform * vec4(pos, 1.0);
}
`
	fragment := `#version 450
layout(set = 0, binding = 1) uniform sampler2D tex;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 outColor;
void main() {
    outColor = texture(tex, uv);
}
`
	for name, tt := range map[string]struct {
		stage  ir.ShaderStage
		source string
	}{
		"vertex":   {ir.StageVertex, vertex},
		"fragment": {ir.StageFragment, fragment},
	} {
		t.Run(name, func(t *testing.T) {
			module, _, err := Parse(tt.source, ParseOptions{Stage: tt.stage})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, verr := valid.Validate(module, valid.Options{}); verr != nil {
				t.Fatalf("the parsed module does not validate: %v", verr)
			}
			if _, _, err := spirv.Compile(module, spirv.Options{}); err != nil {
				t.Fatalf("the SPIR-V backend refused the module: %v", err)
			}
			if _, _, err := msl.Compile(module, msl.Options{}); err != nil {
				t.Fatalf("the MSL backend refused the module: %v", err)
			}
			if _, _, err := hlsl.Compile(module, hlsl.Options{}); err != nil {
				t.Fatalf("the HLSL backend refused the module: %v", err)
			}
		})
	}
}

// TestParseErrorShape checks the R1 contract on the errors the frontend
// collects rather than aborts on: one *ParseError, the first error as the
// message and label, the rest as line:col notes, rendered through the shared
// formatter. Two unused layout qualifiers are two independent errors
// (design §16: collected like Rust's frontend.errors).
func TestParseErrorShape(t *testing.T) {
	source := `#version 450
layout(bogus, other) out vec4 color;
void main() {
    color = vec4(1.0);
}
`
	_, _, err := Parse(source, ParseOptions{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("Parse accepted a shader with unknown layout qualifiers")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error is %T, want *ParseError", err)
	}
	if parseErr.Message != "Unexpected qualifier" {
		t.Fatalf("message = %q, want the unused-qualifier error", parseErr.Message)
	}
	if len(parseErr.Labels) != 1 {
		t.Fatalf("labels = %+v, want the primary label", parseErr.Labels)
	}
	if loc, ok := parseErr.Location(source); !ok || loc.Line != 2 {
		t.Fatalf("location = %+v, %t; want line 2", loc, ok)
	}
	if len(parseErr.Notes) != 1 {
		t.Fatalf("notes = %+v, want the second unused qualifier", parseErr.Notes)
	}
	if !strings.HasPrefix(parseErr.Notes[0], "2:") || !strings.Contains(parseErr.Notes[0], "Unexpected qualifier") {
		t.Fatalf("note = %q, want a line 2 qualifier error", parseErr.Notes[0])
	}
	formatted := parseErr.Format(source, "shader.frag")
	if !strings.Contains(formatted, "error: Unexpected qualifier") || !strings.Contains(formatted, "shader.frag:2:") {
		t.Fatalf("Format output = %q, want the message and the primary position", formatted)
	}
}

// TestParseUnknownVariableIsFatal checks that an unknown name aborts the
// lowering, as Rust's lookup_variable error propagates without collecting
// (variables.rs:101-107): one error, no notes.
func TestParseUnknownVariableIsFatal(t *testing.T) {
	source := `#version 450
void main() {
    a = 1.0;
    b = 2.0;
}
`
	_, _, err := Parse(source, ParseOptions{Stage: ir.StageFragment})
	if err == nil {
		t.Fatal("Parse accepted a shader with unknown variables")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error is %T, want *ParseError", err)
	}
	if !strings.Contains(parseErr.Message, "Unknown variable") {
		t.Fatalf("message = %q, want an unknown-variable error", parseErr.Message)
	}
	if len(parseErr.Notes) != 0 {
		t.Fatalf("notes = %+v, want none: the first unknown name is fatal", parseErr.Notes)
	}
}

// TestParseManyErrorsIsNotQuadratic pins the cost of error reporting: the
// public error path builds one line index (ir.SourceLines) instead of
// rescanning the source per diagnostic. One #error per line, 100k of them,
// used to take 8.2 s (deep validation, 2026-11-15) and takes well under a
// second with the index; the bound is loose on purpose.
func TestParseManyErrorsIsNotQuadratic(t *testing.T) {
	var b strings.Builder
	b.WriteString("#version 450\n")
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(&b, "#error boom%d\n", i)
	}
	b.WriteString("void main() {}\n")

	start := time.Now()
	_, _, err := Parse(b.String(), ParseOptions{Stage: ir.StageFragment})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("a source with 100000 #error directives parsed without an error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error is %T, want *ParseError", err)
	}
	if len(pe.Notes) != 99_999 {
		t.Fatalf("notes = %d, want 99999", len(pe.Notes))
	}
	if elapsed > 5*time.Second {
		t.Fatalf("reporting 100000 diagnostics took %s, want under 5s", elapsed)
	}
}
