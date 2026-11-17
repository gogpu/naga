package snapshot_test

// The GL 3.30 core and GLSL ES 3.00 dialect corpus of the GLSL input frontend
// (card NAGA-FEAT-002, phase 4; design §15 and §18.4).
//
// These dialects have no Rust naga oracle: Rust's GLSL frontend reads Vulkan
// GLSL only, so the expected behavior of every row comes from the GLSL
// language rules of design §15 and from the reference GLSL compiler,
// glslangValidator, which is run over each corpus file when the tool is
// installed. A row records that verdict, and the coverage test refuses an ok
// row the reference rejects and a refusal of a form the reference accepts
// unless the refusal records why (a named card, or a language rule the
// reference does not enforce).
//
// An ok row must parse with the row's ResourceBindings and SamplerBindings
// (the dialects have no layout(binding), design §11), validate with
// valid.Validate, and compile through the SPIR-V (spirv-val included), MSL and
// HLSL backends. A refuse row must fail with a *glsl.ParseError whose message
// carries the recorded fragment; its reason names the §15 cell that justifies
// the refusal. A row may differ from the reference in either direction, and the
// difference must be recorded: a refusal glslangValidator accepts needs a
// limitation, and an acceptance it rejects needs a deviation.
//
// Every file under testdata/glsl-dialects is named by exactly one row, and
// every capability of design §15's matrix that these two dialects have is
// driven by at least one row per dialect (TestGLSLDialectMatrixCoverage). The
// checklist is glslDialectCapabilities, written out by hand from §15.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/frontend"
	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
)

// glslDialectCorpusDir holds one subdirectory per dialect; every file in it is
// a self-contained shader of that dialect.
const glslDialectCorpusDir = "../testdata/glsl-dialects"

// glslDialect is one OpenGL dialect of the corpus.
type glslDialect struct {
	// dir is the corpus subdirectory and the dialect's label in test names.
	dir string
	// version is the #version line every file of the dialect starts with,
	// checked so a file copied from the other directory fails loudly.
	version string
	// label names the dialect in failure messages.
	label string
}

var (
	// glslDialectGL330 is GL 3.30 core: desktop OpenGL, vertex and fragment
	// stages, no binding qualifier (design §15's third column).
	glslDialectGL330 = glslDialect{dir: "gl330", version: "#version 330 core", label: "GL 3.30 core"}
	// glslDialectES300 is GLSL ES 3.00: WebGL 2.0 and OpenGL ES 3.0, vertex
	// and fragment stages, no binding qualifier (design §15's fourth column).
	glslDialectES300 = glslDialect{dir: "es300", version: "#version 300 es", label: "GLSL ES 3.00"}
)

// glslBothDialects is the pair every dialect-independent cell of §15 covers.
var glslBothDialects = []glslDialect{glslDialectGL330, glslDialectES300}

// glslangVerdict is what the reference GLSL compiler made of a row's source.
// It is recorded per row and re-checked when the tool is installed, so the
// corpus cannot drift away from the dialect the reference defines.
type glslangVerdict int

const (
	// glslangAccepts is a source glslangValidator compiles as its dialect.
	glslangAccepts glslangVerdict = iota + 1
	// glslangRejects is a source glslangValidator refuses as outside its
	// dialect, so the frontend must refuse it too.
	glslangRejects
)

// String names the verdict in failure messages.
func (v glslangVerdict) String() string {
	switch v {
	case glslangAccepts:
		return "accepts"
	case glslangRejects:
		return "rejects"
	default:
		return "unknown verdict"
	}
}

// glslDialectRow is one corpus file and what the frontend must do with it.
type glslDialectRow struct {
	// dialect is the corpus directory the file lives in.
	dialect glslDialect
	// file is the file name inside that directory; it is also the row's
	// identity (the subtest name is dialect.dir/file).
	file string
	// stage is the GLSL stage the source is parsed as; it must agree with the
	// file's extension, as it does for every other GLSL corpus in this package.
	stage ir.ShaderStage
	// capabilities are the design §15 cells the row drives; they must be
	// entries of glslDialectCapabilities.
	capabilities []string
	// bindings places the module-scope resources the source cannot bind
	// itself: these dialects have no layout(binding), so the caller supplies
	// the map (design §11, design §15's first row).
	bindings map[string]ir.ResourceBinding
	// samplers places the sampler of each split combined image sampler, when
	// the row supplies it rather than taking the next free binding.
	samplers frontend.SamplerBindings
	// refuse is the fragment the *glsl.ParseError message must contain; empty
	// means the row must parse, validate and compile.
	refuse string
	// refuseNot is a fragment the message must not contain: it keeps a row
	// from passing on a diagnostic that is not the dialect's. The storage
	// block rows use it, because a block without a binding fails with the
	// generic layout(binding) error before any dialect gate can speak.
	refuseNot string
	// reason justifies a refusal: the design §15 cell and the language rule
	// that back it. Empty for an ok row.
	reason string
	// limitation records why a form glslangValidator accepts is refused anyway:
	// a named card, or a language rule the reference does not enforce. It is
	// required exactly for a refuse row whose glslang verdict is accepts.
	limitation string
	// deviation records why an ok row accepts a form glslangValidator rejects:
	// a named card or a rule the reference cannot apply to an offline frontend.
	// It is required exactly for an ok row whose glslang verdict is rejects.
	deviation string
	// glslang is the reference compiler's verdict, re-checked by the matrix.
	glslang glslangVerdict
	// wantWarnings is the fewest ParseInfo warnings the row accepts; 0
	// asserts nothing.
	wantWarnings int
	// noWarningAbout is a text no ParseInfo warning may carry; it pins the
	// absence of a diagnostic for a row that must be silent about it.
	noWarningAbout string
}

// glslDialectMatrix is the corpus: the ok rows of each dialect first, then its
// refusals, then the #extension behavior rows of both.
var glslDialectMatrix = []glslDialectRow{
	// --- GL 3.30 core: what the dialect has --------------------------------

	{
		dialect:      glslDialectGL330,
		file:         "vertex-input-locations.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"stage-vertex", "vertex-input-location"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "ubo-std140.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"uniform-block-std140", "resource-bindings"},
		bindings: map[string]ir.ResourceBinding{
			"Globals": {Group: 0, Binding: 0},
		},
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "combined-sampler.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"stage-fragment", "combined-sampler", "resource-bindings"},
		bindings: map[string]ir.ResourceBinding{
			"tex": {Group: 0, Binding: 0},
		},
		// The sampler the split creates takes the binding the map gives it, so
		// the whole pipeline layout is deterministic (design §11, R2).
		samplers: frontend.SamplerBindings{
			{Group: 0, Binding: 0}: {Group: 0, Binding: 1},
		},
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "ubo-and-sampler.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"uniform-block-std140", "combined-sampler", "resource-bindings"},
		bindings: map[string]ir.ResourceBinding{
			"Globals": {Group: 0, Binding: 0},
			"tex":     {Group: 0, Binding: 1},
		},
		samplers: frontend.SamplerBindings{
			{Group: 0, Binding: 1}: {Group: 0, Binding: 2},
		},
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "fragment-output-locations.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"fragment-output-location"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "precision-statements.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"precision-statement"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "float16-extension.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"float16-extension"},
		glslang:      glslangAccepts,
	},

	// --- GL 3.30 core: refusals of the Vulkan-dialect features -------------

	{
		dialect:      glslDialectGL330,
		file:         "refuse-compute.comp",
		stage:        ir.StageCompute,
		capabilities: []string{"refuse-compute"},
		refuse:       "compute",
		reason: "design §15's Stages row: GL 3.30 core has vertex and fragment stages only; compute " +
			"is GLSL 4.30 (and ES 3.10). glslangValidator: \"compute shaders require es profile with " +
			"version 310 or above, or non-es profile with version 420 or above\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-storage-buffer.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"refuse-storage-buffer"},
		refuse:       "buffer",
		refuseNot:    "layout(binding",
		reason: "design §15's Storage buffers row: the `buffer` block is GLSL 4.30 / ES 3.10; 3.30 " +
			"core does not have the keyword. glslangValidator: \"syntax error, unexpected IDENTIFIER\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-storage-image.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-storage-image"},
		refuse:       "image",
		reason: "design §15's Images row: image load/store is GLSL 4.20 / ES 3.10. glslangValidator: " +
			"\"image load store : not supported for this version or the enabled extensions\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-double.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-double"},
		refuse:       "double",
		reason: "design §15's double row: double is GLSL 4.00. glslangValidator: \"double : Reserved " +
			"word\" and \"not supported for this version\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-push-constant.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-push-constant"},
		refuse:       "push_constant",
		reason: "design §15's Push constants row: layout(push_constant) is Vulkan GLSL only. " +
			"glslangValidator: \"push_constant : only allowed when using GLSL for Vulkan\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-layout-binding.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-layout-binding"},
		refuse:       "binding",
		reason: "design §15's first row: GL 3.30 core has no layout(binding=) — that is GLSL 4.20 — " +
			"and the dialect's binding model is ResourceBindings. glslangValidator: \"binding : not " +
			"supported for this version or the enabled extensions\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-texture-gather.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-texture-gather"},
		refuse:       "textureGather",
		reason: "textureGather is GLSL 4.00: design §15 has no gather for this dialect, and the " +
			"frontend learned it for Vulkan GLSL in phase 3. glslangValidator: \"textureGather(...) : " +
			"not supported for this version or the enabled extensions\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-texture-query-levels.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-texture-query-levels"},
		refuse:       "textureQueryLevels",
		reason: "textureQueryLevels is GLSL 4.30: design §15 has no query for this dialect, and the " +
			"frontend implements it for Vulkan GLSL. glslangValidator: \"textureQueryLevels : no " +
			"matching overloaded function found\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-subgroups.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-subgroups"},
		refuse:       "subgroup",
		reason: "design §15's Subgroups row allows them in GL 3.30 through GL_KHR_shader_subgroup_*, " +
			"and glslangValidator accepts this source; the clustered form is refused because " +
			"ir.CollectiveOperation has no ClusteredReduce (the ballot bit operations and the other " +
			"implemented families are accepted)",
		limitation: "ir.CollectiveOperation has no ClusteredReduce and no cluster-size operand " +
			"(card P2-IR-040, ADR-014 D4; GL_KHR_shader_subgroup_clustered, Rust #7571)",
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-gl-fragcolor.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-gl-fragcolor"},
		refuse:       "gl_FragColor",
		reason: "design §15's gl_FragColor row: 3.30 core drops the deprecated builtins, so a fragment " +
			"shader writes its outputs with out variables",
		limitation: "glslangValidator is laxer than the language here and still accepts gl_FragColor " +
			"in 3.30 core; the GLSL 3.30 rule decides",
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "extension-require-unknown.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-require-unknown"},
		wantWarnings: 1,
		deviation: "glslangValidator, and GLSL 4.60 §3.3, make require of an unsupported extension " +
			"an error; an offline frontend cannot enumerate the Khronos extension registry, so the " +
			"directive is accepted as a no-op with a warning (card P2-GLSLIN-022)",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-extension-disabled-use.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-disabled-use"},
		refuse:       "float16",
		reason: "design §15's #extension semantics: disable turns a supported extension off, so the " +
			"float16_t type it provides is unknown afterwards. glslangValidator: \"float16_t : " +
			"undeclared identifier\"",
		glslang: glslangRejects,
	},

	{
		dialect:      glslDialectGL330,
		file:         "plain-uniform.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"plain-uniform-default-block", "resource-bindings"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-cube-array-sampler.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-cube-array-sampler"},
		refuse:       "samplerCubeArray",
		reason: "design §15: cube array samplers are GLSL 4.00 / ES 3.20. glslangValidator: " +
			"\"'samplerCubeArray' : Reserved word\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectGL330,
		file:         "refuse-push-constant-value.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"refuse-push-constant-value"},
		refuse:       "Qualifier doesn't expect a value",
		reason: "layout(push_constant) is Vulkan GLSL only and takes no value; the value is Rust " +
			"naga's none_layout_qualifier error and the space must not become immediate. " +
			"glslangValidator: \"'push_constant' : there is no such layout identifier for this " +
			"stage taking an assigned value\"",
		glslang: glslangRejects,
	},

	// --- GL 3.30 core: #extension behavior ---------------------------------

	{
		dialect:        glslDialectGL330,
		file:           "extension-enable.frag",
		stage:          ir.StageFragment,
		capabilities:   []string{"extension-enable-warning"},
		wantWarnings:   1,
		noWarningAbout: "",
		glslang:        glslangAccepts,
	},
	{
		dialect:      glslDialectGL330,
		file:         "extension-warn.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-warn-warning"},
		wantWarnings: 1,
		glslang:      glslangAccepts,
	},
	{
		dialect:        glslDialectGL330,
		file:           "extension-disable.frag",
		stage:          ir.StageFragment,
		capabilities:   []string{"extension-disable"},
		noWarningAbout: "GL_EXT_shader_explicit_arithmetic_types_float16",
		glslang:        glslangAccepts,
	},

	// --- GLSL ES 3.00: what the dialect has --------------------------------

	{
		dialect:      glslDialectES300,
		file:         "vertex-input-locations.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"stage-vertex", "vertex-input-location"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "ubo-std140.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"uniform-block-std140", "resource-bindings"},
		bindings: map[string]ir.ResourceBinding{
			"Globals": {Group: 0, Binding: 0},
		},
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "combined-sampler.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"stage-fragment", "combined-sampler", "resource-bindings"},
		bindings: map[string]ir.ResourceBinding{
			"tex": {Group: 0, Binding: 0},
		},
		samplers: frontend.SamplerBindings{
			{Group: 0, Binding: 0}: {Group: 0, Binding: 1},
		},
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "precision-statements.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"precision-statement"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "precision-sampler-type.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"precision-sampler-type"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "fragment-output-sequential.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"fragment-output-sequential"},
		glslang:      glslangAccepts,
	},

	// --- GLSL ES 3.00: refusals of the Vulkan and desktop features ---------

	{
		dialect:      glslDialectES300,
		file:         "refuse-compute.comp",
		stage:        ir.StageCompute,
		capabilities: []string{"refuse-compute"},
		refuse:       "compute",
		reason: "design §15's Stages row: ES 3.00 has vertex and fragment stages only; compute is " +
			"ES 3.10. glslangValidator: \"compute shaders require es profile with version 310 or " +
			"above, or non-es profile with version 420 or above\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-storage-buffer.vert",
		stage:        ir.StageVertex,
		capabilities: []string{"refuse-storage-buffer"},
		refuse:       "buffer",
		refuseNot:    "layout(binding",
		reason: "design §15's Storage buffers row: the `buffer` block is ES 3.10; ES 3.00 does not have " +
			"the keyword. glslangValidator: \"syntax error, unexpected IDENTIFIER\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-storage-image.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-storage-image"},
		refuse:       "image",
		reason: "design §15's Images row: image load/store is ES 3.10. glslangValidator: \"image load " +
			"store : not supported for this version or the enabled extensions\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-double.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-double"},
		refuse:       "double",
		reason: "design §15's double row: ES 3.00 has no double type. glslangValidator: \"double : " +
			"Reserved word\" and \"not supported with this profile: es\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-push-constant.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-push-constant"},
		refuse:       "push_constant",
		reason: "design §15's Push constants row: layout(push_constant) is Vulkan GLSL only. " +
			"glslangValidator: \"push_constant : only allowed when using GLSL for Vulkan\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-layout-binding.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-layout-binding"},
		refuse:       "binding",
		reason: "design §15's first row: ES 3.00 has no layout(binding=), and the dialect's binding " +
			"model is ResourceBindings. glslangValidator: \"binding : not supported for this version " +
			"or the enabled extensions\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-texture-gather.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-texture-gather"},
		refuse:       "textureGather",
		reason: "textureGather is ES 3.10: design §15 has no gather for this dialect, and the frontend " +
			"learned it for Vulkan GLSL in phase 3. glslangValidator: \"textureGather(...) : not " +
			"supported for this version or the enabled extensions\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-texture-query-levels.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-texture-query-levels"},
		refuse:       "textureQueryLevels",
		reason: "textureQueryLevels is ES 3.10: design §15 has no query for this dialect. " +
			"glslangValidator: \"textureQueryLevels : no matching overloaded function found\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-subgroups.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-subgroups"},
		refuse:       "subgroup",
		reason: "design §15's Subgroups row: no subgroups in ES 3.00 (they arrive with ES 3.10), and " +
			"ES 3.00 has no extension that could enable them. glslangValidator: \"subgroupBallot : no " +
			"matching overloaded function found\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-gl-fragcolor.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-gl-fragcolor"},
		refuse:       "gl_FragColor",
		reason: "design §15's gl_FragColor row: ES 3.00 writes fragment outputs with out variables. " +
			"glslangValidator: \"gl_FragColor : undeclared identifier\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-no-default-precision.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-default-precision"},
		refuse:       "precision",
		reason: "design §15's Precision qualifiers row: the ES fragment stage's float type has no " +
			"default, so a fragment shader using it needs a precision statement. glslangValidator: " +
			"\"float : type requires declaration of default precision qualifier\"",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-fragment-output-location.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-fragment-output-location"},
		refuse:       "fragment output locations require ES 3.10",
		reason: "design §15's layout(location=) row: ES 3.00 has no location qualifier on fragment " +
			"outputs (ES 3.10 introduced it); an unlocated output keeps its sequential slot. " +
			"glslangValidator 16.2 is lax and accepts the qualifier in 300 es",
		limitation: "glslangValidator is laxer than the language here: the ES 3.00 specification has " +
			"no location on fragment outputs, so the ES 3.10 rule decides",
		glslang: glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "extension-require-unknown.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-require-unknown"},
		wantWarnings: 1,
		deviation: "glslangValidator, and GLSL 4.60 §3.3, make require of an unsupported extension " +
			"an error; an offline frontend cannot enumerate the Khronos extension registry, so the " +
			"directive is accepted as a no-op with a warning (card P2-GLSLIN-022)",
		glslang: glslangRejects,
	},
	{
		dialect:      glslDialectES300,
		file:         "plain-uniform.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"plain-uniform-default-block", "resource-bindings"},
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "refuse-cube-array-sampler.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"refuse-cube-array-sampler"},
		refuse:       "samplerCubeArray",
		reason: "design §15: cube array samplers are ES 3.20. glslangValidator: " +
			"\"'samplerCubeArray' : Reserved word\"",
		glslang: glslangRejects,
	},

	// --- GLSL ES 3.00: #extension behavior ---------------------------------

	{
		dialect:      glslDialectES300,
		file:         "extension-enable.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-enable-warning"},
		wantWarnings: 1,
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "extension-warn.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-warn-warning"},
		wantWarnings: 1,
		glslang:      glslangAccepts,
	},
	{
		dialect:      glslDialectES300,
		file:         "extension-disable.frag",
		stage:        ir.StageFragment,
		capabilities: []string{"extension-disable"},
		// glslangValidator warns about disabling an extension it does not
		// support; design §15 makes disable never an error, and says nothing
		// about a warning, so the row asserts only that the source parses.
		glslang: glslangAccepts,
	},
}

// TestGLSLDialectMatrix runs every corpus row through the frontend and, when
// glslangValidator is installed, through the reference compiler as well.
func TestGLSLDialectMatrix(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)

	for _, row := range glslDialectMatrix {
		t.Run(row.id(), func(t *testing.T) {
			source := readGLSLDialectSource(t, row)
			if haveGLSLang {
				checkGLSLDialectWithGLSLang(t, glslang, row)
			}

			module, info, err := glsl.Parse(source, glsl.ParseOptions{
				Stage:            row.stage,
				ResourceBindings: row.bindings,
				SamplerBindings:  row.samplers,
			})
			if row.refuse != "" {
				checkGLSLDialectRefusal(t, row, source, err)
				return
			}
			if err != nil {
				t.Fatalf("Parse (%s): %v\n\nsource:\n%s", row.dialect.label, err, source)
			}
			checkGLSLDialectWarnings(t, row, info)
			if verr := validateGLSL(t, module); verr != nil {
				t.Fatalf("the parsed module does not validate: %v\n\nsource:\n%s", verr, source)
			}
			if step, cerr := compileGLSLModule(t, row.id(), module); cerr != nil {
				t.Fatalf("%s step: %v\n\nsource:\n%s", step, cerr, source)
			}
		})
	}
}

// id is the row's identity: the dialect directory and the file, as the subtest
// name and in failure messages.
func (r glslDialectRow) id() string { return r.dialect.dir + "/" + r.file }

// path is the row's corpus file, relative to the snapshot package.
func (r glslDialectRow) path() string {
	return filepath.Join(glslDialectCorpusDir, r.dialect.dir, r.file)
}

// readGLSLDialectSource reads the row's file and checks that it really is a
// file of the row's dialect: a file copied into the wrong directory would
// otherwise be tested as the wrong dialect.
func readGLSLDialectSource(t *testing.T, row glslDialectRow) string {
	t.Helper()
	data, err := os.ReadFile(row.path())
	if err != nil {
		t.Fatalf("read %s: %v", row.path(), err)
	}
	source := string(data)
	first, _, _ := strings.Cut(source, "\n")
	if got := strings.TrimSuffix(first, "\r"); got != row.dialect.version {
		t.Fatalf("%s starts with %q, want the dialect's %q", row.path(), got, row.dialect.version)
	}
	return source
}

// checkGLSLDialectRefusal asserts that a refuse row failed with the recorded
// *glsl.ParseError message.
func checkGLSLDialectRefusal(t *testing.T, row glslDialectRow, source string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("parsed, want a refusal containing %q (%s)\n\nsource:\n%s",
			row.refuse, row.reason, source)
	}
	var parseErr *glsl.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("error is %T, want *glsl.ParseError: %v", err, err)
	}
	if !strings.Contains(parseErr.Message, row.refuse) {
		t.Fatalf("refusal is %q, want it to contain %q\nreason: %s\n\nsource:\n%s",
			parseErr.Message, row.refuse, row.reason, source)
	}
	if row.refuseNot != "" && strings.Contains(parseErr.Message, row.refuseNot) {
		t.Fatalf("refusal is %q: it carries %q, so it is not the dialect's diagnosis\nreason: %s\n\nsource:\n%s",
			parseErr.Message, row.refuseNot, row.reason, source)
	}
}

// checkGLSLDialectWarnings asserts the row's ParseInfo warning contract: at
// least wantWarnings warnings, and none carrying noWarningAbout.
func checkGLSLDialectWarnings(t *testing.T, row glslDialectRow, info glsl.ParseInfo) {
	t.Helper()
	if len(info.Warnings) < row.wantWarnings {
		t.Fatalf("%d warnings, want at least %d: %s", len(info.Warnings), row.wantWarnings, row.file)
	}
	if row.noWarningAbout == "" {
		return
	}
	for _, warning := range info.Warnings {
		if strings.Contains(warning.Message, row.noWarningAbout) {
			t.Fatalf("warning about %q, want none: %s", row.noWarningAbout, warning.Message)
		}
	}
}

// checkGLSLDialectWithGLSLang compiles the row's corpus file with the reference
// GLSL compiler and checks the row's recorded verdict. The file itself is
// compiled, not a copy, so the test proves what is on disk; and glslangValidator
// runs without -G: that flag asks for OpenGL SPIR-V, whose extra requirements
// (a location on every user input and output, ES 3.10 or later) are not rules
// of these two dialects.
func checkGLSLDialectWithGLSLang(t *testing.T, glslang string, row glslDialectRow) {
	t.Helper()
	out, err := exec.Command(glslang, row.path()).CombinedOutput()
	accepted := err == nil
	switch {
	case row.glslang == glslangAccepts && !accepted:
		t.Fatalf("glslangValidator rejects a row recorded as accepted by it: %v\n%s", err, out)
	case row.glslang == glslangRejects && accepted:
		t.Fatalf("glslangValidator accepts a row recorded as rejected by it:\n%s", out)
	case row.glslang == glslangRejects && !strings.Contains(string(out), "ERROR:"):
		// A non-zero exit without a diagnostic is a missing file or a bad
		// invocation, not the reference refusing the source.
		t.Fatalf("glslangValidator failed without a diagnostic: %v\n%s", err, out)
	}
}

// glslDialectCapability is one cell of design §15's matrix, as the checklist
// the corpus must cover. Keep it in step with the design's table by hand: a
// cell added there is an entry here and at least one row per listed dialect.
type glslDialectCapability struct {
	// name is the capability's key, named in the rows that drive it.
	name string
	// dialects are the dialects in which the corpus must drive the cell.
	dialects []glslDialect
	// note points at the design's row.
	note string
}

// glslDialectCapabilities is design §15's matrix for the two OpenGL dialects:
// the capabilities they have and the ones they refuse.
var glslDialectCapabilities = []glslDialectCapability{
	{name: "stage-vertex", dialects: glslBothDialects, note: "§15 Stages: vertex"},
	{name: "stage-fragment", dialects: glslBothDialects, note: "§15 Stages: fragment"},
	{name: "vertex-input-location", dialects: glslBothDialects, note: "§15 layout(location=) on vertex inputs: yes"},
	{name: "fragment-output-location", dialects: []glslDialect{glslDialectGL330}, note: "§15 layout(location=) on fragment outputs: yes in 3.30"},
	{name: "fragment-output-sequential", dialects: []glslDialect{glslDialectES300}, note: "§15 layout(location=) on fragment outputs: sequential in ES 3.00"},
	{name: "refuse-fragment-output-location", dialects: []glslDialect{glslDialectES300}, note: "§15 layout(location=) on fragment outputs: none in ES 3.00 (ES 3.10 added it; glslang is lax)"},
	{name: "uniform-block-std140", dialects: glslBothDialects, note: "§15 Uniform blocks"},
	{name: "combined-sampler", dialects: glslBothDialects, note: "§15 Combined samplers (split)"},
	{name: "resource-bindings", dialects: glslBothDialects, note: "§15 no layout(binding): ResourceBindings and the one counter"},
	{name: "precision-statement", dialects: glslBothDialects, note: "§15 Precision qualifiers"},
	{name: "precision-sampler-type", dialects: []glslDialect{glslDialectES300}, note: "§15 Precision qualifiers; GLSL ES 3.00 §4.5.4"},
	{name: "float16-extension", dialects: []glslDialect{glslDialectGL330}, note: "§15 float16_t: with extension"},
	{name: "extension-require-unknown", dialects: glslBothDialects, note: "§15 #extension: require of an unknown extension is a warning (deliberate deviation from GLSL 4.60 §3.3, card P2-GLSLIN-022)"},
	{name: "extension-enable-warning", dialects: glslBothDialects, note: "§15 #extension: enable produces a warning"},
	{name: "extension-warn-warning", dialects: glslBothDialects, note: "§15 #extension: warn produces a warning"},
	{name: "extension-disable", dialects: glslBothDialects, note: "§15 #extension: disable turns a supported extension off"},
	{name: "extension-disabled-use", dialects: []glslDialect{glslDialectGL330}, note: "§15 #extension: a disabled extension's type is unknown"},
	{name: "refuse-compute", dialects: glslBothDialects, note: "§15 Stages: no compute"},
	{name: "refuse-storage-buffer", dialects: glslBothDialects, note: "§15 Storage buffers: no"},
	{name: "refuse-storage-image", dialects: glslBothDialects, note: "§15 Images: no"},
	{name: "refuse-cube-array-sampler", dialects: glslBothDialects, note: "§15: cube array samplers are GLSL 4.00 / ES 3.20"},
	{name: "plain-uniform-default-block", dialects: glslBothDialects, note: "plain uniforms outside a block: one synthesized std140 block (ADR-013)"},
	{name: "refuse-double", dialects: glslBothDialects, note: "§15 double: no"},
	{name: "refuse-push-constant", dialects: glslBothDialects, note: "§15 Push constants: no"},
	{name: "refuse-push-constant-value", dialects: []glslDialect{glslDialectGL330}, note: "push_constant takes no value: Rust naga's \"Qualifier doesn't expect a value\""},
	{name: "refuse-layout-binding", dialects: glslBothDialects, note: "§15 layout(binding=)/set=: no"},
	{name: "refuse-texture-gather", dialects: glslBothDialects, note: "§15: textureGather is 4.00 / ES 3.10"},
	{name: "refuse-texture-query-levels", dialects: glslBothDialects, note: "§15: textureQueryLevels is 4.30 / ES 3.10"},
	{name: "refuse-subgroups", dialects: glslBothDialects, note: "§15 Subgroups: no in ES 3.00, extension-only in 3.30"},
	{name: "refuse-gl-fragcolor", dialects: glslBothDialects, note: "§15 gl_FragColor: no"},
	{name: "refuse-default-precision", dialects: []glslDialect{glslDialectES300}, note: "§15 Precision qualifiers: fragment float needs a default in ES 3.00"},
}

// TestGLSLDialectMatrixCoverage audits the corpus itself: every row states
// exactly one outcome, its stage matches its extension, its capability names
// are design §15's, every §15 cell of these dialects has a row per dialect,
// and every file in the corpus directory is named by a row.
func TestGLSLDialectMatrixCoverage(t *testing.T) {
	known := make(map[string]glslDialectCapability, len(glslDialectCapabilities))
	for _, capability := range glslDialectCapabilities {
		if _, duplicate := known[capability.name]; duplicate {
			t.Errorf("glslDialectCapabilities lists %q twice", capability.name)
		}
		known[capability.name] = capability
	}

	type cell struct{ capability, dialect string }
	covered := make(map[cell]string, len(glslDialectCapabilities)*len(glslBothDialects))
	named := make(map[string]struct{}, len(glslDialectMatrix))
	for _, row := range glslDialectMatrix {
		if _, duplicate := named[row.id()]; duplicate {
			t.Errorf("two corpus rows name %s", row.id())
		}
		named[row.id()] = struct{}{}

		if !row.dialect.known() {
			t.Errorf("%s: unknown dialect %q", row.id(), row.dialect.dir)
		}
		wantStage, knownExtension := glslStageByExtension[filepath.Ext(row.file)]
		if !knownExtension {
			t.Errorf("%s: extension %q is not .vert, .frag or .comp", row.id(), filepath.Ext(row.file))
		} else if row.stage != wantStage {
			t.Errorf("%s: stage %v does not match the source's %q extension", row.id(), row.stage, filepath.Ext(row.file))
		}
		switch row.glslang {
		case glslangAccepts, glslangRejects:
		default:
			t.Errorf("%s: records no glslang verdict", row.id())
		}

		switch {
		case row.refuse != "" && row.reason == "":
			t.Errorf("%s: refuses with %q but records no reason", row.id(), row.refuse)
		case row.refuse == "" && row.reason != "":
			t.Errorf("%s: states no refusal but records a reason: %s", row.id(), row.reason)
		}
		if row.refuse == "" && row.limitation != "" {
			t.Errorf("%s: accepts but records a limitation: %s", row.id(), row.limitation)
		}
		if row.refuse == "" && row.refuseNot != "" {
			t.Errorf("%s: accepts but excludes a refusal text: %q", row.id(), row.refuseNot)
		}
		if row.refuse != "" && row.deviation != "" {
			t.Errorf("%s: states a refusal and a deviation: %s", row.id(), row.deviation)
		}
		if row.refuse != "" && row.glslang == glslangAccepts && row.limitation == "" {
			t.Errorf("%s: glslangValidator accepts this form, so the refusal needs a named-card or "+
				"language-rule limitation", row.id())
		}
		if row.refuse == "" && row.glslang == glslangRejects && row.deviation == "" {
			t.Errorf("%s: glslangValidator rejects this form, so an ok row needs a recorded "+
				"deviation", row.id())
		}
		if row.refuse == "" && row.glslang == glslangAccepts && row.deviation != "" {
			t.Errorf("%s: glslangValidator accepts this form, so the ok row needs no deviation: %s",
				row.id(), row.deviation)
		}

		for _, name := range row.capabilities {
			if _, ok := known[name]; !ok {
				t.Errorf("%s: drives %q, which glslDialectCapabilities does not list", row.id(), name)
				continue
			}
			key := cell{capability: name, dialect: row.dialect.dir}
			if _, taken := covered[key]; !taken {
				covered[key] = row.id()
			}
		}
	}

	for _, capability := range glslDialectCapabilities {
		for _, dialect := range capability.dialects {
			key := cell{capability: capability.name, dialect: dialect.dir}
			if _, ok := covered[key]; !ok {
				t.Errorf("%s (%s) has no %s row; add one to glslDialectMatrix",
					dialect.label, capability.name, capability.note)
			}
		}
	}

	for _, dialect := range glslBothDialects {
		entries, err := os.ReadDir(filepath.Join(glslDialectCorpusDir, dialect.dir))
		if err != nil {
			t.Fatalf("read the %s corpus directory: %v", dialect.label, err)
		}
		for _, entry := range entries {
			id := dialect.dir + "/" + entry.Name()
			if entry.IsDir() {
				t.Errorf("%s: unexpected subdirectory in the corpus", id)
				continue
			}
			if _, ok := named[id]; !ok {
				t.Errorf("%s is in the corpus but no row drives it; add a row to glslDialectMatrix", id)
			}
		}
	}
}

// known reports whether the dialect is one of the corpus's two.
func (d glslDialect) known() bool {
	return d.dir == glslDialectGL330.dir || d.dir == glslDialectES300.dir
}
