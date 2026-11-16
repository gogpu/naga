package snapshot_test

// The declaration-rule matrix of the GLSL frontend (cards P2-GLSLIN-024..026):
// the rules a module-scope or local declaration must satisfy before anything
// is built from it — no initializer on a shared variable (GLSL 4.60 §4.3.8),
// no user identifier with the reserved gl_ prefix (§3.7), and no repeated
// module-scope name (glslang's "'a' : redefinition").
//
// The driver is the image and atomics matrices' driver: an ok row is compiled
// by glslangValidator when the tool is installed, then parsed, validated and
// written by every backend, and its module goes back through the GLSL backend;
// a refuse row must fail with a *glsl.ParseError whose message carries the
// recorded text, and its reason names what justifies the refusal. The image
// declaration's class against its layout format (P2-GLSLIN-023) lives in the
// image matrix, next to the storage-image rows it constrains.
//
// Every refusal here was probed against glslangValidator 11:16.2.0: each one
// is refused by the reference compiler with the message the row pins, except
// the builtin redeclaration row, which glslang accepts and the frontend
// refuses as a recorded limitation (card P2-GLSLIN-029).

import (
	"errors"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
)

// glslDeclarationRulesMatrix is the matrix: the shared-variable rule, the
// reserved gl_ prefix at every declaration site (with the two declarations the
// rule does not reach), and the module-scope redefinition rule per declaration
// form (with the two legal forms it must not refuse).
var glslDeclarationRulesMatrix = []glslMatrixRow{
	// --- a shared variable cannot have an initializer (P2-GLSLIN-024) --------

	{
		name:      "refuse-shared-initializer",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "'shared' : a shared variable cannot have an initializer (GLSL 4.60 §4.3.8)",
		reason: "GLSL 4.60 §4.3.8: \"Variables declared as shared may not have initializers and " +
			"their contents are undefined at the beginning of shader execution\"; glslang refuses " +
			"the initializer (\"'shared' : initializer can only be a null initializer ('{}')\") " +
			"and refuses the null initializer too",
		source: `#version 450
layout(local_size_x = 1) in;
shared uint c = 0u;
void main() { c = 1u; }
`,
	},
	{
		name:      "refuse-shared-vector-initializer",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "'shared' : a shared variable cannot have an initializer (GLSL 4.60 §4.3.8)",
		reason: "the rule is the declaration's, not the scalar type's (GLSL 4.60 §4.3.8); " +
			"glslang refuses the initializer of a shared vec4 the same way",
		source: `#version 450
layout(local_size_x = 1) in;
shared vec4 v = vec4(0.0);
void main() { v = vec4(1.0); }
`,
	},
	{
		name:      "refuse-shared-array-initializer",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "'shared' : a shared variable cannot have an initializer (GLSL 4.60 §4.3.8)",
		reason: "an array's initializer is no more allowed than a scalar's (GLSL 4.60 §4.3.8); " +
			"glslang refuses it",
		source: `#version 450
layout(local_size_x = 1) in;
shared uint arr[2] = uint[](1u, 2u);
void main() { arr[0] = 1u; }
`,
	},
	{
		name:      "shared-plain-declaration",
		stage:     ir.StageCompute,
		extension: ".comp",
		// A shared variable without an initializer stays legal; glslang
		// accepts it, and this is where the matrix says so.
		source: `#version 450
layout(local_size_x = 1) in;
shared uint c;
void main() { c = 1u; }
`,
	},

	// --- the reserved gl_ prefix (P2-GLSLIN-025) -----------------------------

	{
		name:      "refuse-gl-module-variable",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_x' : identifiers starting with \"gl_\" are reserved",
		reason: "GLSL 4.60 §3.7: identifiers starting with gl_ are reserved and may not be " +
			"declared; glslang refuses the variable",
		source: `#version 450
float gl_x;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(gl_x); }
`,
	},
	{
		name:      "refuse-gl-local-variable",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_x' : identifiers starting with \"gl_\" are reserved",
		reason: "a local variable is a variable declaration under GLSL 4.60 §3.7's reservation; " +
			"glslang refuses it",
		source: `#version 450
layout(location = 0) out vec4 fragColor;
void main() { float gl_x = 1.0; fragColor = vec4(gl_x); }
`,
	},
	{
		name:      "refuse-gl-function",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_f' : identifiers starting with \"gl_\" are reserved",
		reason: "the reservation names variables and functions (GLSL 4.60 §3.7); glslang refuses " +
			"the function",
		source: `#version 450
layout(location = 0) out vec4 fragColor;
float gl_f() { return 1.0; }
void main() { fragColor = vec4(gl_f()); }
`,
	},
	{
		name:      "refuse-gl-parameter",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_p' : identifiers starting with \"gl_\" are reserved",
		reason: "a parameter is a variable declaration under GLSL 4.60 §3.7's reservation; " +
			"glslang refuses it",
		source: `#version 450
layout(location = 0) out vec4 fragColor;
float f(float gl_p) { return gl_p; }
void main() { fragColor = vec4(f(1.0)); }
`,
	},
	{
		name:      "refuse-gl-block-member",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_z' : identifiers starting with \"gl_\" are reserved",
		reason: "the members of an instance-less block are scoped at the global level (GLSL 4.60 " +
			"§4.3.9), so the reservation reaches them; glslang refuses the member",
		source: `#version 450
layout(set = 0, binding = 0) uniform Blk { float gl_z; };
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(gl_z); }
`,
	},
	{
		name:      "refuse-gl-named-block-member",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_z' : identifiers starting with \"gl_\" are reserved",
		reason: "a named block's member is not a module-scope name (GLSL 4.60 §4.3.9 scopes it to " +
			"the instance), but it is still a variable declaration, and glslang refuses the " +
			"reserved prefix on it too (probed 2026-11-16)",
		source: `#version 450
layout(set = 0, binding = 0) uniform Blk { float gl_z; } blk;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(blk.gl_z); }
`,
	},
	{
		name:      "refuse-gl-block-instance",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_inst' : identifiers starting with \"gl_\" are reserved",
		reason:    "a block instance is a variable declaration (GLSL 4.60 §3.7); glslang refuses it",
		source: `#version 450
layout(set = 0, binding = 0) uniform Blk { float x; } gl_inst;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(gl_inst.x); }
`,
	},
	{
		name:      "refuse-gl-block-name",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_Blk' : identifiers starting with \"gl_\" are reserved",
		reason: "glslang refuses a gl_-prefixed block name too (\"'block declaration' : cannot " +
			"redeclare block: gl_Blk\"); the reservation's own message states the rule",
		source: `#version 450
layout(set = 0, binding = 0) uniform gl_Blk { float x; };
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(x); }
`,
	},
	{
		name:      "refuse-gl-opaque-resource",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_s' : identifiers starting with \"gl_\" are reserved",
		reason:    "an opaque uniform is a variable declaration (GLSL 4.60 §3.7); glslang refuses it",
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D gl_s;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texture(gl_s, vec2(0.5)); }
`,
	},
	{
		name:      "struct-names-with-the-gl-prefix",
		stage:     ir.StageFragment,
		extension: ".frag",
		// A struct type name and a struct member are namespace of their own: the
		// reservation covers names declared as variables and functions (GLSL
		// 4.60 §3.7), and glslang accepts the form in both its GL and Vulkan
		// modes (probed 2026-11-16), so the frontend accepts it too rather than
		// rejecting valid GLSL.
		source: `#version 450
struct gl_S { float gl_y; };
layout(location = 0) out vec4 fragColor;
void main() { gl_S s; s.gl_y = 1.0; fragColor = vec4(s.gl_y); }
`,
	},
	{
		name:      "builtin-variables-resolve",
		stage:     ir.StageFragment,
		extension: ".frag",
		// The builtins are references, not declarations: resolving gl_FragCoord
		// and gl_FragDepth is unaffected by the reservation.
		source: `#version 450
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(gl_FragCoord.xy, gl_FragDepth, 1.0); }
`,
	},
	{
		name:      "refuse-gl-builtin-redeclaration",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'gl_FragCoord' : identifiers starting with \"gl_\" are reserved",
		reason: "the frontend does not model the redeclaration of a predeclared builtin, which " +
			"GLSL 4.60 §3.7 allows for specific purposes and glslang accepts here " +
			"(\"layout(origin_upper_left) in vec4 gl_FragCoord;\" compiles -V, probed 2026-11-16); " +
			"refusing it beats lowering it as a location input the shader never meant " +
			"(limitation recorded on card P2-GLSLIN-029)",
		source: `#version 450
layout(origin_upper_left) in vec4 gl_FragCoord;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = gl_FragCoord; }
`,
	},

	// --- the module-scope redefinition rule (P2-GLSLIN-026) ------------------

	{
		name:      "refuse-redefinition-module-variable",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'a' : redefinition",
		reason: "a second module-scope declaration of a name is glslang's \"'a' : redefinition\"; " +
			"accepting it lowered the later declaration while the earlier one's references still " +
			"resolved to it",
		source: `#version 450
float a;
float a;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(a); }
`,
	},
	{
		name:      "refuse-redefinition-constant",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'k' : redefinition",
		reason: "a module-scope const shares the module's name space (glslang: \"'k' : " +
			"redefinition\", probed in GL 3.30 core and Vulkan GLSL)",
		source: `#version 450
const float k = 1.0;
const float k = 2.0;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(k); }
`,
	},
	{
		name:      "refuse-redefinition-sampler",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'s' : redefinition",
		reason: "an opaque resource is a module-scope name like any other; glslang refuses the " +
			"second declaration",
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(set = 0, binding = 1) uniform sampler2D s;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texture(s, vec2(0.5)); }
`,
	},
	{
		name:      "refuse-redefinition-in-one-declarator-list",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'b' : redefinition",
		reason: "two declarators of one declaration name the same variable; glslang: \"'b' : " +
			"redefinition\"",
		source: `#version 450
float b, b;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(b); }
`,
	},
	{
		name:      "refuse-redefinition-block-instance",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'a' : redefinition",
		reason: "a block instance is a module-scope variable; glslang refuses a global of the " +
			"same name",
		source: `#version 450
layout(set = 0, binding = 0) uniform Blk { float x; } a;
float a;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(a.x + a); }
`,
	},
	{
		name:      "refuse-redefinition-nameless-block-member",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'a' : redefinition",
		reason: "the member of an instance-less block is scoped at the global level (GLSL 4.60 " +
			"§4.3.9), so it redefines the global; glslang names the block's member with its own " +
			"wording (\"'Blk' : nameless block contains a member that already has a name at " +
			"global scope\") — the pinned message states the same rule",
		source: `#version 450
float a;
layout(set = 0, binding = 0) uniform Blk { float a; };
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(a); }
`,
	},
	{
		name:      "refuse-redefinition-two-nameless-blocks",
		stage:     ir.StageFragment,
		extension: ".frag",
		refuse:    "'a' : redefinition",
		reason: "two instance-less blocks cannot put one name at global scope; glslang refuses " +
			"the second block (\"nameless block contains a member that already has a name at " +
			"global scope\")",
		source: `#version 450
layout(set = 0, binding = 0) uniform BlkA { float a; };
layout(set = 0, binding = 1) uniform BlkB { float a; };
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(1.0); }
`,
	},
	{
		name:      "shadowing-local",
		stage:     ir.StageFragment,
		extension: ".frag",
		// A local name is unrelated to a module-scope one: shadowing is legal
		// GLSL, and glslang accepts it.
		source: `#version 450
float a;
layout(location = 0) out vec4 fragColor;
void main() { float a = 1.0; fragColor = vec4(a); }
`,
	},
	{
		name:      "named-block-members-are-not-module-scope",
		stage:     ir.StageFragment,
		extension: ".frag",
		// A named block's members live in the instance's own scope (GLSL 4.60
		// §4.3.9), so two blocks may share a member name and a member may repeat
		// a global; glslang accepts both (probed 2026-11-16), and the module
		// table must not refuse them.
		source: `#version 450
float y;
layout(set = 0, binding = 0) uniform BlkA { float y; } blockA;
layout(set = 0, binding = 1) uniform BlkB { float y; } blockB;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = vec4(y + blockA.y + blockB.y); }
`,
	},
}

// TestGLSLDeclarationRulesMatrix runs every row through the frontend and, for
// an ok row, through the reference GLSL compiler and all the backends.
func TestGLSLDeclarationRulesMatrix(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)

	for _, row := range glslDeclarationRulesMatrix {
		t.Run(row.name, func(t *testing.T) {
			if row.refuse == "" {
				checkGLSLSourceWithGLSLang(t, glslang, haveGLSLang, row)
			}

			module, _, err := glsl.Parse(row.source, glsl.ParseOptions{Stage: row.stage})
			if row.refuse != "" {
				if err == nil {
					t.Fatalf("parsed, want a refusal containing %q (%s)\n\nsource:\n%s",
						row.refuse, row.reason, row.source)
				}
				var parseErr *glsl.ParseError
				if !errors.As(err, &parseErr) {
					t.Fatalf("error is %T, want *glsl.ParseError: %v", err, err)
				}
				if !strings.Contains(parseErr.Message, row.refuse) {
					t.Fatalf("refusal is %q, want it to contain %q\nreason: %s\n\nsource:\n%s",
						parseErr.Message, row.refuse, row.reason, row.source)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v\n\nsource:\n%s", err, row.source)
			}
			if verr := validateGLSL(t, module); verr != nil {
				t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, row.source)
			}
			if step, cerr := compileGLSLModule(t, row.name, module); cerr != nil {
				t.Fatalf("%s step: %v\n\nsource:\n%s", step, cerr, row.source)
			}
			// The frontend's own output goes back through the GLSL backend at
			// Vulkan GLSL's version, the dialect these forms exist in.
			if _, _, gerr := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450}); gerr != nil {
				t.Fatalf("glsl backend round trip: %v\n\nsource:\n%s", gerr, row.source)
			}
		})
	}
}

// TestGLSLDeclarationRulesMatrixCoverage audits the matrix itself: every row
// states exactly one outcome, every refusal records why, and the stage agrees
// with the source's extension.
func TestGLSLDeclarationRulesMatrixCoverage(t *testing.T) {
	seen := make(map[string]struct{}, len(glslDeclarationRulesMatrix))
	for _, row := range glslDeclarationRulesMatrix {
		if _, duplicate := seen[row.name]; duplicate {
			t.Errorf("two matrix rows are named %q", row.name)
		}
		seen[row.name] = struct{}{}

		switch {
		case row.refuse == "" && row.reason != "":
			t.Errorf("%s: states no refusal but records a reason: %s", row.name, row.reason)
		case row.refuse != "" && row.reason == "":
			t.Errorf("%s: refuses with %q but records no reason", row.name, row.refuse)
		}
		wantStage, known := glslStageByExtension[row.extension]
		if !known {
			t.Errorf("%s: extension %q is not .vert, .frag or .comp", row.name, row.extension)
		} else if row.stage != wantStage {
			t.Errorf("%s: stage %v does not match the source's %q extension", row.name, row.stage, row.extension)
		}
	}
}
