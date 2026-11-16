package snapshot_test

// The synthesized default uniform block of the GL 3.30 core and ES 3.00
// dialects (ADR-013, card P1-GLSLIN-021): every plain (non-block) uniform of a
// module becomes one member of one generated uniform block, laid out std140.
//
// These tests are the ADR's verification: a dialect shader parses, validates
// with every capability, compiles through SPIR-V (spirv-val included), MSL and
// HLSL, and every reported member offset matches glslangValidator's for the same
// declarations wrapped by hand in an explicit std140 block — read from the
// SPIR-V's OpMemberDecorate %B ... Offset and, as a second witness, from
// glslangValidator -q reflection.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
)

// plainUniformCase is one dialect's plain-uniform shader together with the same
// declarations wrapped by hand in an explicit std140 block for glslangValidator.
// The wrapper is a Vulkan GLSL source because these two dialects cannot spell
// layout(std140): the layout rules are std140's either way, which is the point
// of the comparison.
type plainUniformCase struct {
	// id is the subtest name.
	id string
	// stage is the stage the source is parsed as.
	stage ir.ShaderStage
	// source is the dialect shader under test.
	source string
	// glslang is the same member declarations in an explicit std140 block.
	glslang string
	// members are the expected offsets and sizes. They were probed from
	// glslangValidator (see the comments in the sources) and are asserted twice:
	// once here, once against the reference compiler at test time.
	members []frontend.DefaultBlockMember
	// size is the expected std140 size of the whole block: the end of the last
	// member rounded up to the block's 16-byte alignment.
	size uint32
}

var plainUniformCases = []plainUniformCase{
	{
		id:    "gl330/plain-uniform",
		stage: ir.StageFragment,
		source: `#version 330 core

uniform float scale;
uniform vec3 tint;
uniform float weights[4];
uniform mat2x3 skew;
uniform vec4 tail;

layout(location = 0) out vec4 color;

void main() {
    color = vec4(scale) + vec4(tint, 1.0) + vec4(weights[0]) + vec4(skew[0], 1.0) + tail;
}
`,
		// glslangValidator -V on this block: scale 0, tint 16, weights 32
		// (ArrayStride 16), skew 96 (MatrixStride 16), tail 128; block size 144.
		glslang: `#version 450
layout(std140, binding = 0) uniform B {
    float scale;
    vec3 tint;
    float weights[4];
    mat2x3 skew;
    vec4 tail;
};
layout(location = 0) out vec4 color;
void main() {
    color = vec4(scale) + vec4(tint, 1.0) + vec4(weights[0]) + vec4(skew[0], 1.0) + tail;
}
`,
		members: []frontend.DefaultBlockMember{
			{Name: "scale", Offset: 0, Size: 4},
			{Name: "tint", Offset: 16, Size: 12},
			{Name: "weights", Offset: 32, Size: 64},
			{Name: "skew", Offset: 96, Size: 32},
			{Name: "tail", Offset: 128, Size: 16},
		},
		size: 144,
	},
	{
		id:    "es300/plain-uniform",
		stage: ir.StageFragment,
		source: `#version 300 es
precision highp float;

uniform float scale;
uniform vec2 offset;
uniform float weights[4];

out vec4 color;

void main() {
    color = vec4(scale, offset, 1.0) + vec4(weights[0]) + vec4(weights[1]);
}
`,
		// glslangValidator -V on this block: scale 0, offset 8, weights 16
		// (ArrayStride 16); block size 80.
		glslang: `#version 450
layout(std140, binding = 0) uniform B {
    float scale;
    vec2 offset;
    float weights[4];
};
layout(location = 0) out vec4 color;
void main() {
    color = vec4(scale, offset, 1.0) + vec4(weights[0]) + vec4(weights[1]);
}
`,
		members: []frontend.DefaultBlockMember{
			{Name: "scale", Offset: 0, Size: 4},
			{Name: "offset", Offset: 8, Size: 8},
			{Name: "weights", Offset: 16, Size: 64},
		},
		size: 80,
	},
}

// TestGLSLPlainUniformDefaultBlock is the end-to-end contract of ADR-013 for
// both dialects: the report, the module, validation and every backend.
func TestGLSLPlainUniformDefaultBlock(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)
	for _, tc := range plainUniformCases {
		t.Run(tc.id, func(t *testing.T) {
			module, info, err := glsl.Parse(tc.source, glsl.ParseOptions{Stage: tc.stage})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			report := checkPlainUniformReport(t, module, info, tc)
			if report.Binding != (ir.ResourceBinding{Group: 0, Binding: 0}) {
				t.Errorf("the block is at %+v, want the first slot of group 0: it is the module's only resource", report.Binding)
			}

			if err := validateGLSL(t, module); err != nil {
				t.Fatalf("the parsed module does not validate: %v", err)
			}
			if step, err := compileGLSLModule(t, tc.id, module); err != nil {
				t.Fatalf("%s step: %v", step, err)
			}

			if haveGLSLang {
				checkPlainUniformOffsetsWithGLSLang(t, glslang, tc)
			}
		})
	}
}

// checkPlainUniformReport checks that the module's generated block and the
// report agree with the case's std140 table, and returns the report.
func checkPlainUniformReport(t *testing.T, module *ir.Module, info glsl.ParseInfo, tc plainUniformCase) frontend.DefaultBlockInfo {
	t.Helper()
	if info.DefaultBlock == nil {
		t.Fatal("ParseInfo.DefaultBlock is nil, want the synthesized block")
	}
	block := *info.DefaultBlock
	if block.Name != frontend.DefaultBlockName {
		t.Errorf("the block's name is %q, want %q", block.Name, frontend.DefaultBlockName)
	}
	if len(block.Members) != len(tc.members) {
		t.Fatalf("%d reported members, want %d: %+v", len(block.Members), len(tc.members), block.Members)
	}
	for i, want := range tc.members {
		got := block.Members[i]
		if got != want {
			t.Errorf("member %d is %+v, want %+v (std140, glslangValidator's layout)", i, got, want)
		}
	}
	if block.Size != tc.size {
		t.Errorf("the block's size is %d, want %d", block.Size, tc.size)
	}

	// The report must describe the module's global, not a parallel model: the
	// global carries the binding, the struct carries the offsets.
	var global *ir.GlobalVariable
	for i := range module.GlobalVariables {
		if module.GlobalVariables[i].Name == frontend.DefaultBlockName {
			global = &module.GlobalVariables[i]
		}
	}
	if global == nil {
		t.Fatalf("the module has no %s global", frontend.DefaultBlockName)
	}
	if global.Space != ir.SpaceUniform {
		t.Errorf("the block's space is %v, want uniform", global.Space)
	}
	if global.Binding == nil || *global.Binding != block.Binding {
		t.Errorf("the global is bound at %v, want the reported %+v", global.Binding, block.Binding)
	}
	structType, ok := module.Types[global.Type].Inner.(ir.StructType)
	if !ok {
		t.Fatalf("the block's type is %T, want a struct", module.Types[global.Type].Inner)
	}
	if structType.Span != block.Size {
		t.Errorf("the block struct's span is %d, want the reported size %d", structType.Span, block.Size)
	}
	if len(structType.Members) != len(block.Members) {
		t.Fatalf("%d struct members, %d reported", len(structType.Members), len(block.Members))
	}
	for i, member := range structType.Members {
		if member.Name != block.Members[i].Name || member.Offset != block.Members[i].Offset {
			t.Errorf("struct member %d is %q at %d, reported %+v", i, member.Name, member.Offset, block.Members[i])
		}
	}
	return block
}

// plainUniformReflectionLine matches a member of glslangValidator -q reflection,
// as in "scale: offset 0, type 1406, size 1, ...".
var plainUniformReflectionLine = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_.]*): offset (-?\d+),`)

// plainUniformBlockSize matches the block's line of the same reflection, as in
// "B: offset -1, type ffffffff, size 144, index 0, binding 0, stages 1, numMembers 5".
var plainUniformBlockSize = regexp.MustCompile(`^B: offset -?\d+, type [0-9a-f]+, size (\d+),`)

// reflectedAnyPrefix reports whether the reflection named a field of member, as
// it does for a struct member's own fields ("light.pos" for "light").
func reflectedAnyPrefix(reflected map[string]uint32, member string) bool {
	prefix := member + "."
	for name := range reflected {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// plainUniformDecoration matches one member's offset in disassembled SPIR-V, as
// in "OpMemberDecorate %B 0 Offset 0".
var plainUniformDecoration = regexp.MustCompile(`OpMemberDecorate %B (\d+) Offset (\d+)`)

// plainUniformReflectionLines splits a tool's output into lines, whatever line
// ending the platform gave it.
var plainUniformReflectionLines = regexp.MustCompile(`\r?\n`)

// checkPlainUniformOffsetsWithGLSLang compiles the case's hand-written std140
// block with the reference compiler and compares every offset with the case's
// table: the SPIR-V decorations are the first witness, the -q reflection the
// second. A mismatch fails either way, so the table the frontend is judged by is
// never just the frontend's own number.
func checkPlainUniformOffsetsWithGLSLang(t *testing.T, glslang string, tc plainUniformCase) {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "block.vert")
	if err := os.WriteFile(source, []byte(tc.glslang), 0o600); err != nil {
		t.Fatalf("write the glslang source: %v", err)
	}

	binary := filepath.Join(dir, "block.spv")
	if out, err := exec.Command(glslang, "-V", source, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("glslangValidator -V: %v\n%s", err, out)
	}

	if disassembler, ok := testtools.Optional(t, "spirv-dis", testtools.InstallSPIRVTools); ok {
		out, err := exec.Command(disassembler, binary).Output()
		if err != nil {
			t.Fatalf("spirv-dis: %v", err)
		}
		offsets := map[int]uint32{}
		for _, match := range plainUniformDecoration.FindAllStringSubmatch(string(out), -1) {
			index, ierr := strconv.Atoi(match[1])
			if ierr != nil {
				t.Fatalf("member index %q: %v", match[1], ierr)
			}
			offset, oerr := strconv.ParseUint(match[2], 10, 32)
			if oerr != nil {
				t.Fatalf("member offset %q: %v", match[2], oerr)
			}
			offsets[index] = uint32(offset)
		}
		if len(offsets) != len(tc.members) {
			t.Fatalf("glslangValidator's block has %d members, the case %d", len(offsets), len(tc.members))
		}
		for i, want := range tc.members {
			if offsets[i] != want.Offset {
				t.Errorf("glslangValidator puts %q at %d, the case says %d", want.Name, offsets[i], want.Offset)
			}
		}
	}

	// The second witness: reflection. It reports each member's offset under its
	// own name and the block's size, which covers the trailing padding our
	// reported size rounds up to.
	out, err := exec.Command(glslang, "-q", "-l", source).CombinedOutput()
	if err != nil {
		t.Fatalf("glslangValidator -q -l: %v\n%s", err, out)
	}
	reflected := map[string]uint32{}
	var blockSize uint32
	var haveBlockSize bool
	for _, line := range plainUniformReflectionLines.Split(string(out), -1) {
		// The block's own line is checked first: it is an offset line too, with
		// offset -1, and its name is the block's.
		if match := plainUniformBlockSize.FindStringSubmatch(line); match != nil {
			size, serr := strconv.ParseUint(match[1], 10, 32)
			if serr != nil {
				t.Fatalf("reflected block size %q: %v", match[1], serr)
			}
			blockSize, haveBlockSize = uint32(size), true
			continue
		}
		if match := plainUniformReflectionLine.FindStringSubmatch(line); match != nil {
			offset, oerr := strconv.ParseUint(match[2], 10, 32)
			if oerr != nil {
				t.Fatalf("reflected offset %q: %v", match[2], oerr)
			}
			reflected[match[1]] = uint32(offset)
		}
	}
	for _, want := range tc.members {
		offset, ok := reflected[want.Name]
		if !ok {
			// Reflection reports a struct member's own fields rather than the
			// member (`light.pos`, `light.q`): the SPIR-V witness above is the
			// one that sees the member itself.
			if reflectedAnyPrefix(reflected, want.Name) {
				continue
			}
			t.Errorf("glslangValidator's reflection reports no member %q", want.Name)
			continue
		}
		if offset != want.Offset {
			t.Errorf("glslangValidator's reflection puts %q at %d, the case says %d", want.Name, offset, want.Offset)
		}
	}
	if !haveBlockSize {
		t.Fatal("glslangValidator's reflection reports no block size")
	}
	// Our reported size rounds the block up to its alignment, so it is never
	// smaller than the reference's, and never more than one alignment past it.
	if tc.size < blockSize || tc.size >= blockSize+16 {
		t.Errorf("the reported size %d does not cover glslangValidator's block of %d bytes", tc.size, blockSize)
	}
}

// TestGLSLPlainUniformStructMember is the ADR's nested-struct case: a struct
// type used as a plain uniform, laid out by its own std140 rules and read
// through the uniform's name, through the whole backend path.
func TestGLSLPlainUniformStructMember(t *testing.T) {
	const source = `#version 330 core
struct Light {
    vec2 pos;
    float power;
};
uniform Light light;
uniform float gain;
layout(location = 0) out vec4 color;
void main() {
    color = vec4(light.pos, light.power * gain, 1.0);
}
`
	// glslangValidator -V on the same block: light 0 (its own members at 0 and
	// 8), gain 16 — a struct member's alignment is rounded up to a vec4's under
	// std140, so gain does not follow light at 12; block size 32.
	const glslang = `#version 450
struct Light {
    vec2 pos;
    float power;
};
layout(std140, binding = 0) uniform B {
    Light light;
    float gain;
};
layout(location = 0) out vec4 color;
void main() {
    color = vec4(light.pos, light.power * gain, 1.0);
}
`
	tc := plainUniformCase{
		id:      "gl330/plain-uniform-struct",
		stage:   ir.StageFragment,
		source:  source,
		glslang: glslang,
		members: []frontend.DefaultBlockMember{
			{Name: "light", Offset: 0, Size: 16},
			{Name: "gain", Offset: 16, Size: 4},
		},
		size: 32,
	}
	glslangTool, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)
	module, info, err := glsl.Parse(source, glsl.ParseOptions{Stage: tc.stage})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	checkPlainUniformReport(t, module, info, tc)
	if err := validateGLSL(t, module); err != nil {
		t.Fatalf("the parsed module does not validate: %v", err)
	}
	if step, cerr := compileGLSLModule(t, tc.id, module); cerr != nil {
		t.Fatalf("%s step: %v", step, cerr)
	}
	if haveGLSLang {
		checkPlainUniformOffsetsWithGLSLang(t, glslangTool, tc)
	}
}

// TestGLSLPlainUniformBoolMember pins the bool case: a bool member keeps its IR
// type and takes the four bytes std140 gives it, which is what the reported
// offsets say.
//
// The module validates and the HLSL and MSL targets compile it, but the SPIR-V
// target does not: it emits an OpTypeBool inside the block, and spirv-val
// refuses that with "If OpTypeBool is stored in conjunction with OpVariable, it
// can only be used with non-externally visible shader Storage Classes" (probed
// 2026-11-16; glslang avoids it by lowering a bool block member to a uint). The
// synthesis does not rewrite the declared type — ADR-013 keeps it and records
// the constraint — so this test stops at the targets that accept it.
func TestGLSLPlainUniformBoolMember(t *testing.T) {
	const source = `#version 330 core
uniform bool enabled;
uniform float gain;
layout(location = 0) out vec4 color;
void main() { color = enabled ? vec4(gain) : vec4(0.0); }
`
	tc := plainUniformCase{
		id:     "gl330/plain-uniform-bool",
		stage:  ir.StageFragment,
		source: source,
		members: []frontend.DefaultBlockMember{
			{Name: "enabled", Offset: 0, Size: 4},
			{Name: "gain", Offset: 4, Size: 4},
		},
		size: 8,
	}
	module, info, err := glsl.Parse(source, glsl.ParseOptions{Stage: tc.stage})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	checkPlainUniformReport(t, module, info, tc)
	if err := validateGLSL(t, module); err != nil {
		t.Fatalf("the parsed module does not validate: %v", err)
	}
	if _, _, err := msl.Compile(module, msl.Options{}); err != nil {
		t.Fatalf("msl: %v", err)
	}
	if _, _, err := hlsl.Compile(module, hlsl.Options{}); err != nil {
		t.Fatalf("hlsl: %v", err)
	}
}

// TestGLSLPlainUniformVulkanUnchanged pins the dialect the decision does not
// touch: a plain uniform in Vulkan GLSL stays refused, and the message is the
// rule rather than a pending feature.
func TestGLSLPlainUniformVulkanUnchanged(t *testing.T) {
	source := `#version 450
uniform float scale;
void main() { gl_Position = vec4(scale); }
`
	_, info, err := glsl.Parse(source, glsl.ParseOptions{Stage: ir.StageVertex})
	if err == nil {
		t.Fatal("Parse succeeded, want a refusal: the Vulkan profile has no default uniform block")
	}
	if info.DefaultBlock != nil {
		t.Errorf("ParseInfo.DefaultBlock = %+v after a refusal, want nil", info.DefaultBlock)
	}
	var parseErr *glsl.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error is %T, want *glsl.ParseError: %v", err, err)
	}
	for _, want := range []string{"non-opaque uniforms outside a block", "declare the uniform inside a uniform block"} {
		if !strings.Contains(parseErr.Message, want) {
			t.Errorf("refusal %q does not contain %q", parseErr.Message, want)
		}
	}
	if strings.Contains(parseErr.Message, "default-block synthesis") || strings.Contains(parseErr.Message, "P1-GLSLIN-021") {
		t.Errorf("refusal %q still points at the pending synthesis", parseErr.Message)
	}
}

// TestGLSLPlainUniformRefusedFormsWithoutTools is the pinned half of the
// behavior matrix: the dialects that refuse, and the forms the synthesis
// refuses, hold with no external tool at all.
func TestGLSLPlainUniformRefusedFormsWithoutTools(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "unsized array",
			source: `#version 330 core
uniform float w[];
void main() { gl_Position = vec4(w[0]); }
`,
			want: "array of unknown length",
		},
		{
			name: "packing on a plain uniform",
			source: `#version 330 core
layout(std140) uniform float scale;
void main() { gl_Position = vec4(scale); }
`,
			want: "cannot specify packing on a variable declaration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := glsl.Parse(tt.source, glsl.ParseOptions{Stage: ir.StageVertex})
			if err == nil {
				t.Fatalf("Parse succeeded, want a refusal containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
