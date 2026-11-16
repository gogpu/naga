package front

// Tests for the declaration-time rules of P2-GLSLIN-023..026: the image
// declaration's class against its layout format, the refusal of an initializer
// on a shared variable, the reserved gl_ prefix, and the module-scope name
// table that refuses a repeated declaration.
//
// Every refusal here was probed against the reference GLSL compiler,
// glslangValidator 11:16.2.0, and the message is glslang's where it has one
// (the probe sources and their verdicts are recorded on the cards). Rust naga's
// GLSL frontend accepts all four forms — it compares neither an image's class
// with its format, nor a shared variable's storage with an initializer, and it
// declares reserved and repeated names as they come — so each refusal is a
// deliberate difference from Rust in the direction the specification decides.
// The end-to-end matrix of the same forms is
// snapshot.TestGLSLDeclarationRulesMatrix.

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestImageDeclarationClassFormat pins the class/format rule of P2-GLSLIN-023:
// a storage image's declared class fixes the sign of what it loads and stores,
// so the layout format must agree with it. glslang, for iimage2D with r32ui:
// "'r32ui' : does not apply to signed integer images".
func TestImageDeclarationClassFormat(t *testing.T) {
	refusals := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "signed image with an unsigned format",
			source: "layout(set = 0, binding = 0, r32ui) uniform iimage2D img;",
			want:   "'r32ui' : does not apply to signed integer images",
		},
		{
			name:   "unsigned image with a signed format",
			source: "layout(set = 0, binding = 0, r32i) uniform uimage2D img;",
			want:   "'r32i' : does not apply to unsigned integer images",
		},
		{
			name:   "floating point image with an unsigned format",
			source: "layout(set = 0, binding = 0, r32ui) uniform image2D img;",
			want:   "'r32ui' : does not apply to floating point images",
		},
		{
			name:   "signed image with a float format",
			source: "layout(set = 0, binding = 0, r32f) uniform iimage2D img;",
			want:   "'r32f' : does not apply to signed integer images",
		},
		{
			// A unorm format is a floating-point format to the rule.
			name:   "floating point image with a signed integer format",
			source: "layout(set = 0, binding = 0, rgba8i) uniform image2D img;",
			want:   "'rgba8i' : does not apply to floating point images",
		},
		{
			// r11f_g11f_b10f is a floating-point format; the name is quoted as
			// the source spells it.
			name:   "unsigned image with a packed float format",
			source: "layout(set = 0, binding = 0, r11f_g11f_b10f) uniform uimage2D img;",
			want:   "'r11f_g11f_b10f' : does not apply to unsigned integer images",
		},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			parseErr(t, "#version 450\n"+tt.source+"\nlayout(location = 0) out vec4 o;\n"+
				"void main() { o = vec4(1.0); }\n", Options{Stage: ir.StageFragment}, tt.want)
		})
	}

	accepted := []struct {
		name   string
		source string
		want   ir.StorageFormat
	}{
		{
			name:   "unsigned image with an unsigned format",
			source: "layout(set = 0, binding = 0, r32ui) uniform uimage2D img;",
			want:   ir.StorageFormatR32Uint,
		},
		{
			name:   "signed image with a signed format",
			source: "layout(set = 0, binding = 0, r32i) uniform iimage2D img;",
			want:   ir.StorageFormatR32Sint,
		},
		{
			name:   "floating point image with a float format",
			source: "layout(set = 0, binding = 0, rgba32f) uniform image2D img;",
			want:   ir.StorageFormatRgba32Float,
		},
		{
			// A normalized format is a floating-point format to the rule.
			name:   "floating point image with a normalized format",
			source: "layout(set = 0, binding = 0, rgba8) uniform image2D img;",
			want:   ir.StorageFormatRgba8Unorm,
		},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			mod, _, err := Parse("#version 450\n"+tt.source+
				"\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(1.0); }\n",
				Options{Stage: ir.StageFragment})
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			validateOK(t, mod)
			image, ok := mod.Types[findGlobal(t, mod, "img").Type].Inner.(ir.ImageType)
			if !ok {
				t.Fatal("the image global does not carry an image type")
			}
			if image.StorageFormat != tt.want {
				t.Fatalf("format = %v, want %v", image.StorageFormat, tt.want)
			}
			if image.Class != ir.ImageClassStorage {
				t.Fatalf("class = %v, want storage", image.Class)
			}
		})
	}
}

// TestSharedInitializerRefused pins P2-GLSLIN-024: GLSL 4.60 §4.3.8, "Variables
// declared as shared may not have initializers and their contents are undefined
// at the beginning of shader execution". glslang refuses the initializer
// outright.
func TestSharedInitializerRefused(t *testing.T) {
	const want = "'shared' : a shared variable cannot have an initializer (GLSL 4.60 §4.3.8)"
	refusals := []struct {
		name   string
		source string
	}{
		{
			name:   "scalar",
			source: "shared uint c = 0u;\nvoid main() { c = 1u; }",
		},
		{
			name:   "vector",
			source: "shared vec4 v = vec4(0.0);\nvoid main() { v = vec4(1.0); }",
		},
		{
			name:   "array",
			source: "shared uint arr[2] = uint[](1u, 2u);\nvoid main() { arr[0] = 1u; }",
		},
		{
			// Every declarator of a list is checked, not only the first.
			name:   "second declarator",
			source: "shared uint a, b = 1u;\nvoid main() { a = 1u; b = 2u; }",
		},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			parseErr(t, "#version 450\nlayout(local_size_x = 1) in;\n"+tt.source+"\n",
				Options{Stage: ir.StageCompute}, want)
		})
	}

	// A shared variable without an initializer stays accepted end to end.
	mod, _, err := Parse("#version 450\nlayout(local_size_x = 1) in;\nshared uint c;\n"+
		"void main() { c = 1u; }\n", Options{Stage: ir.StageCompute})
	if err != nil {
		t.Fatalf("shared without an initializer: %v", err)
	}
	validateOK(t, mod)
	if space := findGlobal(t, mod, "c").Space; space != ir.SpaceWorkGroup {
		t.Fatalf("shared variable's space = %v, want workgroup", space)
	}
}

// TestSharedInitializerIsOneDiagnostic pins the P2-GLSLIN-024 acceptance item
// that a shared variable with an initializer reports the declaration's refusal
// alone: a variable also accessed with atomic memory functions must not grow a
// second error from the atomic upgrade, which runs after every function is
// lowered.
func TestSharedInitializerIsOneDiagnostic(t *testing.T) {
	_, _, err := Parse(`#version 450
layout(local_size_x = 1) in;
shared uint counter = 0u;
void main() { atomicAdd(counter, 1u); }
`, Options{Stage: ir.StageCompute})
	if err == nil {
		t.Fatal("Parse succeeded, want the shared-initializer refusal")
	}
	var parseErrValue *ParseError
	if !errors.As(err, &parseErrValue) {
		t.Fatalf("error is %T, want *ParseError", err)
	}
	if len(parseErrValue.Errors) != 1 {
		t.Fatalf("%d diagnostics, want the shared-initializer refusal alone: %v", len(parseErrValue.Errors), parseErrValue.Errors)
	}
	if !strings.Contains(parseErrValue.Errors[0].Message, "a shared variable cannot have an initializer") {
		t.Fatalf("message = %q, want the shared-initializer refusal", parseErrValue.Errors[0].Message)
	}
}

// TestReservedGlPrefix pins P2-GLSLIN-025: GLSL 4.60 §3.7, identifiers starting
// with gl_ are reserved and may not be declared. glslang refuses each of these
// declarations with "'gl_x' : identifiers starting with "gl_" are reserved".
func TestReservedGlPrefix(t *testing.T) {
	const want = `identifiers starting with "gl_" are reserved`
	refusals := []struct {
		name   string
		stage  ir.ShaderStage
		source string
	}{
		{
			name:   "module-scope variable",
			stage:  ir.StageFragment,
			source: "#version 450\nfloat gl_x;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(gl_x); }\n",
		},
		{
			name:   "local variable",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(location = 0) out vec4 o;\nvoid main() { float gl_x = 1.0; o = vec4(gl_x); }\n",
		},
		{
			name:   "function",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(location = 0) out vec4 o;\nfloat gl_f() { return 1.0; }\nvoid main() { o = vec4(gl_f()); }\n",
		},
		{
			name:   "parameter",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(location = 0) out vec4 o;\nfloat f(float gl_p) { return gl_p; }\nvoid main() { o = vec4(f(1.0)); }\n",
		},
		{
			name:   "block member",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform Blk { float gl_z; };\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(gl_z); }\n",
		},
		{
			// The member of a block with an instance name is not a module-scope
			// name, but it is still a variable declaration, and glslang refuses
			// the reserved prefix on it too (probed 2026-11-16).
			name:   "named block member",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform Blk { float gl_z; } blk;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(blk.gl_z); }\n",
		},
		{
			name:   "block instance",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform Blk { float x; } gl_inst;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(gl_inst.x); }\n",
		},
		{
			name:   "block name",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform gl_Blk { float x; };\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(x); }\n",
		},
		{
			name:   "opaque resource",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform sampler2D gl_s;\nlayout(location = 0) out vec4 o;\nvoid main() { o = texture(gl_s, vec2(0.5)); }\n",
		},
		{
			name:   "plain uniform",
			stage:  ir.StageFragment,
			source: "#version 330 core\nuniform float gl_x;\nout vec4 o;\nvoid main() { o = vec4(gl_x); }\n",
		},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			parseErr(t, tt.source, Options{Stage: tt.stage}, want)
		})
	}

	// The builtin namespace is not a set of declarations: the builtins resolve
	// as before, and the invariant redeclaration path, which names a builtin
	// rather than declaring a variable, keeps working.
	t.Run("builtins still resolve", func(t *testing.T) {
		mod, _, err := Parse(`#version 450
layout(location = 0) in vec4 color;
layout(location = 0) out vec4 o;
void main() { o = color + vec4(gl_FragCoord.xy, gl_FragDepth, 1.0); }
`, Options{Stage: ir.StageFragment})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		validateOK(t, mod)
	})

	// A struct type name and a struct member are a namespace of their own: the
	// spec's reservation covers names declared as variables and functions
	// (GLSL 4.60 §3.7), and glslang accepts both forms in its GL and Vulkan
	// modes (probed 2026-11-16). Refusing them would reject valid GLSL.
	t.Run("struct names and members stay accepted", func(t *testing.T) {
		mod, _, err := Parse(`#version 450
struct gl_S { float gl_y; };
layout(location = 0) out vec4 o;
void main() { gl_S s; s.gl_y = 1.0; o = vec4(s.gl_y); }
`, Options{Stage: ir.StageFragment})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		validateOK(t, mod)
	})

	// The name of the synthesized default block keeps its own, more specific
	// refusal, which runs before the reserved-prefix rule.
	t.Run("the default block's own name", func(t *testing.T) {
		parseErr(t, `#version 330 core
uniform float gl_DefaultUniformBlock;
void main() { gl_Position = vec4(gl_DefaultUniformBlock); }
`, Options{Stage: ir.StageVertex}, "reserved for the block")
	})
}

// TestModuleScopeRedefinition pins P2-GLSLIN-026: a second module-scope
// declaration of a name is refused with glslang's "'a' : redefinition".
func TestModuleScopeRedefinition(t *testing.T) {
	const want = "'a' : redefinition"
	refusals := []struct {
		name   string
		stage  ir.ShaderStage
		source string
	}{
		{
			name:   "module-scope variables",
			stage:  ir.StageFragment,
			source: "#version 450\nfloat a;\nfloat a;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(a); }\n",
		},
		{
			name:   "module-scope constants",
			stage:  ir.StageFragment,
			source: "#version 450\nconst float a = 1.0;\nconst float a = 2.0;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(a); }\n",
		},
		{
			name:   "opaque resources",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform sampler2D a;\nlayout(set = 0, binding = 1) uniform sampler2D a;\nlayout(location = 0) out vec4 o;\nvoid main() { o = texture(a, vec2(0.5)); }\n",
		},
		{
			name:   "one declarator list",
			stage:  ir.StageFragment,
			source: "#version 450\nfloat a, a;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(a); }\n",
		},
		{
			name:   "block instance and a variable",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform Blk { float x; } a;\nfloat a;\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(a.x + a); }\n",
		},
		{
			name:   "a nameless block's member and a variable",
			stage:  ir.StageFragment,
			source: "#version 450\nfloat a;\nlayout(set = 0, binding = 0) uniform Blk { float a; };\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(a); }\n",
		},
		{
			name:   "two nameless blocks with one member name",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(set = 0, binding = 0) uniform BlkA { float a; };\nlayout(set = 0, binding = 1) uniform BlkB { float a; };\nlayout(location = 0) out vec4 o;\nvoid main() { o = vec4(1.0); }\n",
		},
		{
			name:   "plain uniforms",
			stage:  ir.StageVertex,
			source: "#version 330 core\nuniform float a;\nuniform float a;\nvoid main() { gl_Position = vec4(a); }\n",
		},
		{
			name:   "a plain uniform and a module-scope variable",
			stage:  ir.StageVertex,
			source: "#version 330 core\nuniform float a;\nfloat a;\nvoid main() { gl_Position = vec4(a); }\n",
		},
		{
			name:   "IO variables",
			stage:  ir.StageFragment,
			source: "#version 450\nlayout(location = 0) out vec4 a;\nlayout(location = 1) out vec4 a;\nvoid main() { a = vec4(1.0); }\n",
		},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			parseErr(t, tt.source, Options{Stage: tt.stage}, want)
		})
	}

	// Legal shadowing: a local name is unrelated to the module-scope one, and
	// the module table only holds module-scope names.
	t.Run("a local shadows a module-scope name", func(t *testing.T) {
		mod, _, err := Parse(`#version 450
float a;
layout(location = 0) out vec4 o;
void main() { float a = 1.0; o = vec4(a); }
`, Options{Stage: ir.StageFragment})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		validateOK(t, mod)
	})

	// A named block's members are reached through its instance and are not
	// module-scope names (GLSL 4.60 §4.3.9), so two such blocks may share a
	// member name, and a member may repeat a global — or even its own instance's
	// name: glslang accepts all three (probed 2026-11-16), and refusing them
	// would reject valid GLSL.
	t.Run("a named block's members are not module-scope names", func(t *testing.T) {
		mod, _, err := Parse(`#version 450
float x;
layout(set = 0, binding = 0) uniform BlkA { float y; } blockA;
layout(set = 0, binding = 1) uniform BlkB { float y; } blockB;
layout(location = 0) out vec4 o;
void main() { o = vec4(x + blockA.y + blockB.y); }
`, Options{Stage: ir.StageFragment})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		validateOK(t, mod)
	})
	// A member equal to its own instance's name is valid GLSL (glslang accepts
	// `uniform Blk { float blk; } blk;`, probed 2026-11-16) but the frontend
	// still declares every named block's member at module scope (Rust parity),
	// so the member shadows the instance's symbol and `blk.blk` cannot resolve.
	// The resolution gap is carded as P2-GLSLIN-030; the module-name table
	// correctly does not refuse the pair, which is all this batch owns.
}
