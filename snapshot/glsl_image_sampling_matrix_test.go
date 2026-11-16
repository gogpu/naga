package snapshot_test

// The image and sampling matrix of the GLSL frontend (card NAGA-FEAT-002,
// phase 3): the texture, texel fetch, texture query and storage image forms
// this frontend claims, each end to end.
//
// An "ok" row must parse, validate, compile to SPIR-V that spirv-val accepts,
// and compile through the MSL and HLSL backends. Before any of that its source
// is compiled by glslangValidator when the tool is installed, so a row cannot
// assert support for a form the reference GLSL compiler rejects. A "refuse" row
// must fail with a *glsl.ParseError whose message carries the recorded text;
// its reason names what justifies the refusal — a GLSL rule, or the Rust naga
// limitation this frontend mirrors — and records the reference evidence
// (glslang's verdict) that was checked when the row was written.
//
// Every texture and image builtin handled by injectTextureBuiltin
// (glsl/internal/front/builtins.go) has at least one row here, and
// TestGLSLImageSamplingMatrixCoverage reads that switch's own source to check
// it, so a builtin added to the frontend fails the test until a row drives it.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gogpu/naga/glsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
)

// glslangInstall is the install hint printed when glslangValidator is missing,
// as every tool lookup in the snapshot package prints one.
const glslangInstall = "install glslang, e.g. from the Vulkan SDK"

// glslMatrixRow is one form of the image and sampling surface, with exactly one
// of the two outcomes stated: it parses and compiles (refuse is empty), or it
// is refused with the message fragment and the reason.
type glslMatrixRow struct {
	// name is the subtest name and the glslang file stem.
	name string
	// stage is the GLSL stage the source is parsed as; GLSL does not name it.
	stage ir.ShaderStage
	// extension is what glslangValidator uses to pick the stage: .frag, .vert
	// or .comp.
	extension string
	// builtins are the catalog names (injectTextureBuiltin) the row drives.
	// It may be empty for a row that tests the frontend without a texture
	// builtin, such as barrier().
	builtins []string
	// source is a self-contained Vulkan GLSL shader: #version 450 and, for the
	// Vulkan execution model, an explicit set and binding on every resource.
	source string
	// refuse is the fragment the *glsl.ParseError message must contain; empty
	// means the row must parse and compile.
	refuse string
	// reason justifies a refusal. Empty for an ok row.
	reason string
	// targetEnv is the glslangValidator --target-env of an ok row whose source
	// needs a newer target than the default. A subgroup row needs vulkan1.1
	// (SPIR-V 1.3): glslang refuses a subgroup operation with "'subgroup op' :
	// requires SPIR-V 1.3". Empty means glslang's default.
	targetEnv string
	// backendSkip records a text backend that cannot write an ok row's module,
	// keyed by "msl" or "hlsl", with the reason. The matrix asserts the backend
	// still refuses the module, so a limitation that stops applying fails the
	// test; it is recorded, never silently skipped. Empty for a row every
	// backend writes.
	backendSkip map[string]string
}

// glslImageSamplingMatrix is the matrix: the sampling families first, then the
// shadow comparisons, the fetch and size queries, the storage images and the
// two rows that put the family to work in a compute shader and in a
// user-defined function, and finally the refusals.
var glslImageSamplingMatrix = []glslMatrixRow{
	// --- texture() and its level, offset, gradient and projection forms -------

	{
		name:      "texture-auto",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texture(s, uv); }
`,
	},
	{
		name:      "texture-bias",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		// texture(gsampler2D, vec2, float) is the implicit-derivative form with
		// a bias, GLSL 4.50 §8.9; the fragment stage is the only one that has
		// derivatives to bias.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texture(s, uv, 1.5); }
`,
	},
	{
		name:      "texture-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureOffset(s, uv, ivec2(1, -1)); }
`,
	},
	{
		name:      "texture-offset-const-local",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureOffset"},
		// A function-local const is a constant expression (GLSL 4.50 §4.3.3):
		// the offset rule accepts it, and the whole pipeline runs on it.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { const ivec2 off = ivec2(1, -1); fragColor = textureOffset(s, uv, off); }
`,
	},
	{
		name:      "texture-grad",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGrad"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGrad(s, uv, dFdx(uv), dFdy(uv)); }
`,
	},
	{
		name:      "texture-grad-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGradOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGradOffset(s, uv, dFdx(uv), dFdy(uv), ivec2(1, 0)); }
`,
	},
	{
		name:      "texture-lod",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureLod(s, uv, 2.0); }
`,
	},
	{
		name:      "texture-lod-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureLodOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureLodOffset(s, uv, 2.0, ivec2(0, 1)); }
`,
	},
	{
		name:      "texture-proj",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProj"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureProj(s, vec3(uv, 2.0)); }
`,
	},
	{
		name:      "texture-proj-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureProjOffset(s, vec3(uv, 2.0), ivec2(1, 1)); }
`,
	},
	{
		name:      "texture-proj-lod",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureProjLod(s, vec3(uv, 2.0), 1.0); }
`,
	},
	{
		name:      "texture-proj-lod-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjLodOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureProjLodOffset(s, vec3(uv, 2.0), 1.0, ivec2(0, 1)); }
`,
	},
	{
		name:      "texture-proj-grad",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjGrad"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureProjGrad(s, vec3(uv, 2.0), dFdx(uv), dFdy(uv)); }
`,
	},
	{
		name:      "texture-proj-grad-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjGradOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureProjGradOffset(s, vec3(uv, 2.0), dFdx(uv), dFdy(uv), ivec2(0, 1)); }
`,
	},

	// --- shadow comparison sampling -----------------------------------------
	//
	// A depth sampler's coordinate vector carries the reference value as its
	// last component, and the result is the comparison's scalar, GLSL 4.50
	// §8.9. Cube-array shadow is the one form whose reference travels as a
	// separate argument, because the coordinate would otherwise take five
	// components.

	{
		name:      "shadow-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out float fragDepth;
void main() { fragDepth = texture(s, vec3(uv, 0.5)); }
`,
	},
	{
		name:      "shadow-2d-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		// vec4 = (u, v, layer, reference).
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DArrayShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out float fragDepth;
void main() { fragDepth = texture(s, vec4(uv, 1.0, 0.5)); }
`,
	},
	{
		name:      "shadow-cube",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		// vec4 = (direction.xyz, reference).
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCubeShadow s;
layout(location = 0) in vec3 dir;
layout(location = 0) out float fragDepth;
void main() { fragDepth = texture(s, vec4(dir, 0.5)); }
`,
	},
	{
		name:      "shadow-cube-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		// vec4 = (direction.xyz, layer); the reference is the third argument.
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCubeArrayShadow s;
layout(location = 0) in vec3 dir;
layout(location = 0) out float fragDepth;
void main() { fragDepth = texture(s, vec4(dir, 1.0), 0.5); }
`,
	},
	{
		name:      "shadow-proj",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProj"},
		// The projection divides by w; with a depth sampler all four
		// components are consumed by the projection and the reference.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out float fragDepth;
void main() { fragDepth = textureProj(s, vec4(uv, 0.5, 2.0)); }
`,
	},
	{
		name:      "shadow-proj-lod",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjLod"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out float fragDepth;
void main() { fragDepth = textureProjLod(s, vec4(uv, 0.5, 2.0), 1.0); }
`,
	},
	{
		name:      "shadow-proj-lod-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureProjLodOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out float fragDepth;
void main() { fragDepth = textureProjLodOffset(s, vec4(uv, 0.5, 2.0), 1.0, ivec2(0, 1)); }
`,
	},

	// --- texelFetch and texelFetchOffset -------------------------------------
	//
	// A fetch takes an integer coordinate and an explicit level, or a sample
	// index for a multisampled texture, GLSL 4.50 §8.9.

	{
		name:      "texel-fetch-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texelFetch"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texelFetch(s, ivec2(uv), 0); }
`,
	},
	{
		name:      "texel-fetch-3d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texelFetch"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler3D s;
layout(location = 0) in vec3 p;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texelFetch(s, ivec3(p), 1); }
`,
	},
	{
		name:      "texel-fetch-2d-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texelFetch"},
		// ivec3 = (x, y, layer).
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DArray s;
layout(location = 0) in vec3 p;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texelFetch(s, ivec3(p), 0); }
`,
	},
	{
		name:      "texel-fetch-2d-ms",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texelFetch"},
		// A multisampled sampler's last argument is the sample index, not a
		// level; glslangValidator -V accepts this form.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DMS s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texelFetch(s, ivec2(uv), 2); }
`,
	},
	{
		name:      "texel-fetch-offset-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texelFetchOffset"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = texelFetchOffset(s, ivec2(uv), 0, ivec2(1, -1)); }
`,
	},

	// --- textureGather, the 2x2-footprint sampling of GLSL 4.40 §8.9 ---------
	//
	// The component argument selects which channel the four texels carry; a
	// depth sampler compares against its own reference argument and takes no
	// component. The offset forms take a constant offset only.

	{
		name:      "gather-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGather"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGather(s, uv); }
`,
	},
	{
		name:      "gather-2d-component",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGather"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGather(s, uv, 2); }
`,
	},
	{
		name:      "gather-2d-array-offset",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGatherOffset"},
		// ivec3 = (uv, layer); the offset is a constant expression.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DArray s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGatherOffset(s, vec3(uv, 0.0), ivec2(1, -1), 1); }
`,
	},
	{
		name:      "gather-shadow-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGather"},
		// A depth gather compares each texel against refZ; vec4<f32> result.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGather(s, uv, 0.5); }
`,
	},
	{
		name:      "gather-cube",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureGather"},
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCube s;
layout(location = 0) in vec3 dir;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureGather(s, dir, 0); }
`,
	},

	// --- textureSize and textureQueryLevels ----------------------------------
	//
	// textureSize returns the base level's extent and, for an arrayed image,
	// the layer count as the last component (GLSL 4.50 §8.9); a multisampled
	// image takes no level. The rows below cover the dimensions WGSL has (1D,
	// 2D, 3D and cube, with the arrayed 2D and cube forms) and the two shapes
	// only GLSL reaches: sampler1DArray and sampler2DMSArray. WGSL has neither
	// (Rust naga's GLSL corpus disables them for the same reason, samplers.frag
	// HAS_1D_ARRAY_TEXTURES and HAS_2D_MS_ARRAY_TEXTURES); the SPIR-V writer
	// originally produced invalid code for both — a scalar-result
	// OpVectorShuffle and an OpImageQuerySizeLod on a multisampled image — and
	// now emits the composite extract and the level-free query glslang does
	// (spirv/internal/codegen/image_query_test.go pins both).

	{
		name:      "texture-size-1d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler1D s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0, 0, 0); }
`,
	},
	{
		name:      "texture-size-2d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0, 0); }
`,
	},
	{
		name:      "texture-size-3d",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler3D s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0); }
`,
	},
	{
		name:      "texture-size-cube",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCube s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0, 0); }
`,
	},
	{
		name:      "texture-size-2d-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		// ivec3 = (width, height, layers).
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DArray s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0); }
`,
	},
	{
		name:      "texture-size-cube-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		// ivec3 = (face size, face size, layers).
		source: `#version 450
layout(set = 0, binding = 0) uniform samplerCubeArray s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0); }
`,
	},
	{
		name:      "texture-size-2d-ms",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		// The multisampled overload takes no level argument.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DMS s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s), 0, 0); }
`,
	},
	{
		name:      "texture-size-1d-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		// ivec2 = (width, layers). The scalar spatial size comes from an
		// OpCompositeExtract of the (width, layers) query.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler1DArray s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s, 0), 0, 0); }
`,
	},
	{
		name:      "texture-size-2d-ms-array",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureSize"},
		// ivec3 = (width, height, layers), no level: a multisampled image's
		// size query is OpImageQuerySize.
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DMSArray s;
layout(location = 0) out ivec4 fragSize;
void main() { fragSize = ivec4(textureSize(s), 0); }
`,
	},
	{
		name:      "texture-query-levels",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLevels"},
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) out ivec4 fragLevels;
void main() { fragLevels = ivec4(textureQueryLevels(s)); }
`,
	},

	// --- storage images ------------------------------------------------------
	//
	// A storage image is an image2D/3D/2DArray with a layout() format, read
	// with imageLoad, written with imageStore and measured with imageSize; the
	// format's scalar kind decides whether the texel is vec4, uvec4 or ivec4
	// (GLSL 4.50 §8.14, §8.15). The four formats below are the float, unorm,
	// uint and int kinds the frontend's format table maps.

	{
		name:      "image-2d-r32f",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad", "imageStore"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32f, set = 0, binding = 0) uniform image2D img;
void main() {
    vec4 texel = imageLoad(img, ivec2(1, 2));
    imageStore(img, ivec2(0, 0), texel + vec4(1.0));
}
`,
	},
	{
		name:      "image-2d-rgba8",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad", "imageStore"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(rgba8, set = 0, binding = 0) uniform image2D img;
void main() {
    vec4 texel = imageLoad(img, ivec2(1, 2));
    imageStore(img, ivec2(0, 0), texel);
}
`,
	},
	{
		name:      "image-2d-r32ui",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad", "imageStore"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32ui, set = 0, binding = 0) uniform uimage2D img;
void main() {
    uvec4 texel = imageLoad(img, ivec2(1, 2));
    imageStore(img, ivec2(0, 0), texel + uvec4(1u));
}
`,
	},
	{
		name:      "image-2d-r32i",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad", "imageStore"},
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32i, set = 0, binding = 0) uniform iimage2D img;
void main() {
    ivec4 texel = imageLoad(img, ivec2(1, 2));
    imageStore(img, ivec2(0, 0), texel + ivec4(1));
}
`,
	},
	{
		name:      "image-3d-r32f",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad", "imageStore", "imageSize"},
		// imageSize of an image3D is ivec3.
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32f, set = 0, binding = 0) uniform image3D img;
void main() {
    vec4 texel = imageLoad(img, ivec3(1, 2, 3));
    imageStore(img, ivec3(0, 0, 0), texel);
    ivec3 size = imageSize(img);
    imageStore(img, ivec3(0), vec4(float(size.x)));
}
`,
	},
	{
		name:      "image-2d-array-r32i",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad", "imageStore", "imageSize"},
		// ivec3 coordinate = (x, y, layer); imageSize of an arrayed image is
		// (width, height, layers).
		source: `#version 450
layout(local_size_x = 1) in;
layout(r32i, set = 0, binding = 0) uniform iimage2DArray img;
void main() {
    ivec4 texel = imageLoad(img, ivec3(1, 2, 0));
    imageStore(img, ivec3(0, 0, 0), texel);
    ivec3 size = imageSize(img);
    imageStore(img, ivec3(0), ivec4(size.x));
}
`,
	},
	{
		name:      "image-size-2d-rgba8",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageSize"},
		// imageSize alone, without a load or a store, is the query a shader
		// that dispatches over an image needs.
		source: `#version 450
layout(local_size_x = 1) in;
layout(rgba8, set = 0, binding = 0) uniform image2D img;
void main() { ivec2 size = imageSize(img); imageStore(img, ivec2(0), vec4(float(size.x))); }
`,
	},

	// --- the family at work --------------------------------------------------

	{
		name:      "compute-barrier",
		stage:     ir.StageCompute,
		extension: ".comp",
		// barrier() is GLSL's workgroup execution and memory barrier: glslang
		// compiles it to OpControlBarrier with Workgroup as both the execution
		// and the memory scope (GLSL 4.50 §8.15).
		source: `#version 450
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Data { float values[]; } data;
shared float scratch[64];
void main() {
    uint i = gl_GlobalInvocationID.x;
    scratch[i] = float(i);
    barrier();
    data.values[i] = scratch[(i + 1u) % 64u];
}
`,
	},
	{
		name:      "sampler-parameter",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		// A texture and a sampler are opaque types, so they can be function
		// parameters (GLSL 4.50 §4.1.7); the combined form the call site writes
		// is constructed at the point of use, as glslang requires.
		source: `#version 450
layout(set = 0, binding = 0) uniform texture2D t;
layout(set = 0, binding = 1) uniform sampler s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
vec4 sampleThing(texture2D tex, sampler samp, vec2 coord) {
    return texture(sampler2D(tex, samp), coord);
}
void main() { fragColor = sampleThing(t, s, uv); }
`,
	},

	// --- refusals ------------------------------------------------------------

	{
		name:      "refuse-shadow-bias",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"texture"},
		refuse:    "Unknown function 'texture'",
		reason: "GLSL has a bias overload for depth samplers, but Rust naga leaves it out on " +
			"purpose (front/glsl/builtins.rs:183, \"glsl supports using bias with depth samplers " +
			"but naga doesn't\", then `if bias && shadow { continue; }`), and this catalog " +
			"mirrors that; glslangValidator -V accepts the form",
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DShadow s;
layout(location = 0) in vec2 uv;
layout(location = 0) out float fragDepth;
void main() { fragDepth = texture(s, vec3(uv, 0.5), 1.5); }
`,
	},
	{
		name:      "refuse-query-levels-multisampled",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureQueryLevels"},
		refuse:    "Unknown function 'textureQueryLevels'",
		reason: "textureQueryLevels' parameter is a gsampler, which excludes the multisampled " +
			"samplers (GLSL 4.50 §8.9, the overload takes no sample count); glslangValidator -V " +
			"rejects the form with 'no matching overloaded function found'. The catalog has no " +
			"multisampled variation for this query, as Rust naga's has none " +
			"(front/glsl/builtins.rs:297-316 generates without TextureArgsOptions::MULTI)",
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2DMS s;
layout(location = 0) out ivec4 fragLevels;
void main() { fragLevels = ivec4(textureQueryLevels(s)); }
`,
	},
	{
		name:      "refuse-combined-sampler-parameter",
		stage:     ir.StageFragment,
		extension: ".frag",
		// A combined sampler is valid GLSL (glslangValidator -V accepts this
		// source), but the IR has no combined image sampler type: a combined
		// declaration becomes a pair of module-scope globals, so it cannot be a
		// parameter (glsl/internal/front/types_lower.go). Rust naga has no
		// combined sampler support at all (gfx-rs/wgpu#4342).
		refuse: "combined image sampler types are only supported for module-scope uniform variables",
		reason: "a combined sampler is not an IR type: the declaration creates a texture and a " +
			"sampler global, and neither can stand for a sampler2D parameter; glslangValidator " +
			"-V accepts the form, so this is a frontend limit and not a GLSL rule",
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
vec4 sampleThing(sampler2D tex, vec2 coord) { return texture(tex, coord); }
void main() { fragColor = sampleThing(s, uv); }
`,
	},
	{
		name:      "refuse-image-load-on-sampled-texture",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"imageLoad"},
		refuse:    "Unknown function 'imageLoad'",
		reason: "imageLoad reads a storage image, whose type is image2D and its 3D and arrayed " +
			"forms, never a sampled texture (GLSL 4.50 §8.14 defines imageLoad over image " +
			"types); glslangValidator -V rejects this form and the equivalent imageStore with " +
			"'no matching overloaded function found'",
		source: `#version 450
layout(local_size_x = 1) in;
layout(set = 0, binding = 0) uniform sampler2D s;
layout(rgba8, set = 0, binding = 1) uniform image2D outImage;
void main() { imageStore(outImage, ivec2(0), imageLoad(s, ivec2(0))); }
`,
	},
	{
		name:      "refuse-texture-offset-not-constant",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"textureOffset"},
		refuse:    "must be a constant expression",
		reason: "GLSL 4.50 §8.9 requires the offset of a textureOffset call to be a constant " +
			"expression, and the IR carries it as a const-expression (a run-time one would be the " +
			"ConstOffset operand of an OpImageSample* instruction, which spirv-val rejects). " +
			"glslangValidator -V is laxer and accepts the source; the spec decides",
		source: `#version 450
layout(set = 0, binding = 0) uniform sampler2D s;
layout(set = 0, binding = 1) uniform Offsets { ivec2 value; } offsets;
layout(location = 0) in vec2 uv;
layout(location = 0) out vec4 fragColor;
void main() { fragColor = textureOffset(s, uv, offsets.value); }
`,
	},
}

// glslTextureBuiltinNames reads the texture, image and texture-query builtin
// names out of injectTextureBuiltin's switch in glsl/internal/front/builtins.go.
// The scan is the point: a hand-kept list drifts the moment the frontend learns
// a builtin (the Phase 3 gather arrived in that same switch after this matrix
// was written), while the scan fails the coverage test until a row drives the
// new name.
func glslTextureBuiltinNames(t *testing.T) []string {
	t.Helper()
	const path = "../glsl/internal/front/builtins.go"
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != "injectTextureBuiltin" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, ok := n.(*ast.SwitchStmt)
			if !ok {
				return true
			}
			for _, clause := range sw.Body.List {
				cc, ok := clause.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range cc.List {
					lit, ok := expr.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if name, uerr := strconv.Unquote(lit.Value); uerr == nil {
						names = append(names, name)
					}
				}
			}
			return false
		})
		return false
	})
	if len(names) == 0 {
		t.Fatalf("no builtin names found in injectTextureBuiltin in %s; its switch shape changed, update this scan", path)
	}
	sort.Strings(names)
	return names
}

// TestGLSLImageSamplingMatrix runs every row of the matrix through the frontend
// and, for an ok row, through the reference GLSL compiler and all the backends.
func TestGLSLImageSamplingMatrix(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)

	for _, row := range glslImageSamplingMatrix {
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
		})
	}
}

// glslStageByExtension is the stage each source extension names, the same
// mapping glslStageOf applies to the corpus and glslangValidator to a file.
var glslStageByExtension = map[string]ir.ShaderStage{
	".vert": ir.StageVertex,
	".frag": ir.StageFragment,
	".comp": ir.StageCompute,
}

// checkGLSLSourceWithGLSLang compiles an ok row's source with the reference GLSL
// compiler for Vulkan. A form the reference compiler rejects is not GLSL, so
// the matrix must not claim it; a missing tool is reported by testtools.Optional
// and leaves the row's own checks to run.
func checkGLSLSourceWithGLSLang(t *testing.T, glslang string, available bool, row glslMatrixRow) {
	t.Helper()
	if !available {
		return
	}
	dir := t.TempDir()
	source := filepath.Join(dir, row.name+row.extension)
	if err := os.WriteFile(source, []byte(row.source), 0o600); err != nil {
		t.Fatalf("write the row's source: %v", err)
	}
	args := []string{"-V"}
	if row.targetEnv != "" {
		args = append(args, "--target-env", row.targetEnv)
	}
	args = append(args, source, "-o", filepath.Join(dir, "row.spv"))
	out, err := exec.Command(glslang, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("glslangValidator rejects an ok row's source: %v\n%s\n\nsource:\n%s",
			err, out, row.source)
	}
}

// TestGLSLImageSamplingMatrixCoverage audits the matrix itself: every row states
// exactly one outcome, every builtin it names is one injectTextureBuiltin
// handles, and every builtin that switch handles has at least one row.
func TestGLSLImageSamplingMatrixCoverage(t *testing.T) {
	names := glslTextureBuiltinNames(t)
	catalog := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, duplicate := catalog[name]; duplicate {
			t.Errorf("injectTextureBuiltin's switch lists %q twice", name)
		}
		catalog[name] = struct{}{}
	}

	covered := make(map[string]string, len(catalog))
	seen := make(map[string]struct{}, len(glslImageSamplingMatrix))
	for _, row := range glslImageSamplingMatrix {
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

		for _, name := range row.builtins {
			if _, known := catalog[name]; !known {
				t.Errorf("%s: drives %q, which injectTextureBuiltin does not handle", row.name, name)
				continue
			}
			if _, taken := covered[name]; !taken {
				covered[name] = row.name
			}
		}
	}

	for _, name := range names {
		if _, ok := covered[name]; !ok {
			t.Errorf("%s is handled by injectTextureBuiltin but no matrix row drives it; add a row to glslImageSamplingMatrix", name)
		}
	}
}
