package snapshot_test

// The subgroup matrix of the GLSL frontend (card P1-GLSLIN-017): the
// GL_KHR_shader_subgroup_* builtins of GLSL 4.60 §8.18, each end to end.
//
// An "ok" row must parse, validate, compile to SPIR-V that spirv-val accepts,
// and compile through the MSL and HLSL backends; before any of that its source
// is compiled by glslangValidator (at --target-env vulkan1.1, the SPIR-V 1.3
// the subgroup operations need) when the tool is installed, so a row cannot
// assert support for a form the reference GLSL compiler rejects. A "refuse" row
// must fail with a *glsl.ParseError whose message carries the recorded text, and
// its reason names what justifies the refusal.
//
// Every name subgroupBuiltinFunction (glsl/internal/front/subgroup.go) handles
// has a row here, and TestGLSLSubgroupMatrixCoverage reads that switch's own
// source to check it, so a builtin added to the frontend fails the test until a
// row drives it. The IR shape is pinned in glsl/internal/front/subgroup_test.go;
// this matrix is the end-to-end gate.

import (
	"errors"
	"fmt"
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
	"github.com/gogpu/naga/hlsl"
	"github.com/gogpu/naga/internal/testtools"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/msl"
	"github.com/gogpu/naga/spirv"
)

// glslSubgroupMatrix is the matrix: the basic, vote, ballot, shuffle, quad and
// arithmetic forms the frontend lowers, and the refusals of the forms the IR or
// the text backends cannot express.
var glslSubgroupMatrix = []glslMatrixRow{
	// --- the basic family ----------------------------------------------------

	{
		name:      "subgroup-barrier",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBarrier"},
		targetEnv: "vulkan1.1",
		backendSkip: map[string]backendRefusal{
			"hlsl": {
				reason: "HLSL has no subgroup barrier; the backend refuses the SUBGROUP_BARRIER capability " +
					"(msl writes a threadgroup_barrier, a superset of the subgroup barrier, and is recorded here)",
				wantError: "SUBGROUP_BARRIER",
			},
		},
		source: `#version 450
#extension GL_KHR_shader_subgroup_basic : require
layout(local_size_x = 64) in;
shared uint counter;
void main() { subgroupBarrier(); counter = gl_GlobalInvocationID.x; }
`,
	},
	{
		name:      "subgroup-memory-barriers",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins: []string{
			"subgroupMemoryBarrier", "subgroupMemoryBarrierBuffer",
			"subgroupMemoryBarrierShared", "subgroupMemoryBarrierImage",
		},
		targetEnv: "vulkan1.1",
		backendSkip: map[string]backendRefusal{
			"hlsl": {
				reason: "HLSL has no subgroup memory barrier; the backend refuses the SUBGROUP_BARRIER " +
					"capability (msl writes a threadgroup_barrier, a superset, and is recorded here)",
				wantError: "SUBGROUP_BARRIER",
			},
		},
		source: `#version 450
#extension GL_KHR_shader_subgroup_basic : require
layout(local_size_x = 64) in;
shared uint counter;
void main() {
    subgroupMemoryBarrier();
    subgroupMemoryBarrierBuffer();
    subgroupMemoryBarrierShared();
    subgroupMemoryBarrierImage();
    counter = gl_GlobalInvocationID.x;
}
`,
	},
	{
		name:      "subgroup-builtin-variables",
		stage:     ir.StageCompute,
		extension: ".comp",
		targetEnv: "vulkan1.1",
		// The four scalar variables of GL_KHR_shader_subgroup_basic (GLSL 4.60
		// §8.18) are wired through the entry point as glslang compiles them:
		// Input variables decorated SubgroupSize, SubgroupLocalInvocationId,
		// SubgroupId and NumSubgroups (probed 2026-11-16).
		source: `#version 450
#extension GL_KHR_shader_subgroup_basic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uvec4 v; } o;
void main() {
    o.v = uvec4(gl_SubgroupSize, gl_SubgroupInvocationID, gl_SubgroupID, gl_NumSubgroups);
}
`,
	},
	{
		name:      "refuse-subgroup-builtin-variable-without-extension",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "'gl_SubgroupSize' : required extension not requested: GL_KHR_shader_subgroup_basic",
		reason: "the subgroup built-in variables follow the builtins' availability rule — the family's " +
			"extension must be enabled; glslang refuses the same source with the same message (probed " +
			"2026-11-16)",
		source: `#version 450
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uvec4 v; } o;
void main() { o.v = uvec4(gl_SubgroupSize); }
`,
	},
	{
		name:      "refuse-subgroup-ballot-mask-variable",
		stage:     ir.StageCompute,
		extension: ".comp",
		refuse:    "subgroup ballot mask built-in variables",
		reason: "gl_SubgroupEqMask and its siblings are GLSL and glslang accepts them, but ir.Builtin has " +
			"no subgroup ballot-mask value, so the frontend refuses them with the gap named instead of " +
			"lowering a mask the IR cannot carry (P2-GLSLIN-027)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uvec4 v; } o;
void main() { o.v = gl_SubgroupEqMask; }
`,
	},

	// --- the vote family -----------------------------------------------------

	{
		name:      "subgroup-all",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAll"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_vote : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { if (subgroupAll(gl_GlobalInvocationID.x > 3u)) { o.v = 1u; } }
`,
	},
	{
		name:      "subgroup-any",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAny"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_vote : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { if (subgroupAny(gl_GlobalInvocationID.x > 3u)) { o.v = 1u; } }
`,
	},

	// --- the ballot family ---------------------------------------------------

	{
		name:      "subgroup-ballot",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallot"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { uvec4 mask = subgroupBallot(gl_GlobalInvocationID.x > 3u); o.v = mask.x; }
`,
	},
	{
		name:      "subgroup-broadcast",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBroadcast"},
		targetEnv: "vulkan1.1",
		// glslang requires the broadcast index to be a compile-time constant
		// ("argument must be compile-time constant"); the frontend accepts any
		// uint, a superset of what glslang takes.
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBroadcast(gl_GlobalInvocationID.x, 0u); }
`,
	},
	{
		name:      "subgroup-broadcast-first",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBroadcastFirst"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBroadcastFirst(gl_GlobalInvocationID.x); }
`,
	},

	// --- the shuffle and shuffle_relative families ---------------------------

	{
		name:      "subgroup-shuffle",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupShuffle"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_shuffle : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupShuffle(gl_GlobalInvocationID.x, 1u); }
`,
	},
	{
		name:      "subgroup-shuffle-xor",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupShuffleXor"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_shuffle : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupShuffleXor(gl_GlobalInvocationID.x, 1u); }
`,
	},
	{
		name:      "subgroup-shuffle-up",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupShuffleUp"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_shuffle_relative : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupShuffleUp(gl_GlobalInvocationID.x, 1u); }
`,
	},
	{
		name:      "subgroup-shuffle-down",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupShuffleDown"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_shuffle_relative : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupShuffleDown(gl_GlobalInvocationID.x, 1u); }
`,
	},

	// --- the quad family -----------------------------------------------------

	{
		name:      "subgroup-quad-broadcast",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupQuadBroadcast"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_quad : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupQuadBroadcast(gl_GlobalInvocationID.x, 1u); }
`,
	},
	{
		name:      "subgroup-quad-swaps",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins: []string{
			"subgroupQuadSwapHorizontal", "subgroupQuadSwapVertical", "subgroupQuadSwapDiagonal",
		},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_quad : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() {
    uint x = gl_GlobalInvocationID.x;
    o.v = subgroupQuadSwapHorizontal(x) + subgroupQuadSwapVertical(x) + subgroupQuadSwapDiagonal(x);
}
`,
	},

	// --- the arithmetic reductions ------------------------------------------

	{
		name:      "subgroup-add",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAdd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupAdd(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "subgroup-mul",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupMul"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupMul(gl_GlobalInvocationID.x + 1u); }
`,
	},
	{
		name:      "subgroup-min",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupMin"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupMin(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "subgroup-max",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupMax"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupMax(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "subgroup-and",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAnd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupAnd(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "subgroup-or",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupOr"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupOr(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "subgroup-xor",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupXor"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupXor(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "subgroup-add-float",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAdd"},
		targetEnv: "vulkan1.1",
		// A float reduction keeps the argument's float type in the IR.
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { float f; } o;
void main() { o.f = subgroupAdd(1.5); }
`,
	},

	// --- the arithmetic scans -----------------------------------------------

	{
		name:      "subgroup-inclusive-add",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveAdd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveAdd(gl_GlobalInvocationID.x + 1u); }
`,
	},
	{
		name:      "subgroup-exclusive-add",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveAdd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveAdd(gl_GlobalInvocationID.x + 1u); }
`,
	},
	{
		name:      "subgroup-inclusive-mul",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveMul"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveMul(gl_GlobalInvocationID.x + 1u); }
`,
	},
	{
		name:      "subgroup-exclusive-mul",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveMul"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveMul(gl_GlobalInvocationID.x + 1u); }
`,
	},

	// --- vector operands (P2-GLSLIN-028) --------------------------------------
	//
	// GLSL takes vector operands for the collective families, and SPIR-V
	// defines the operation on a vector per component (glslang emits
	// OpGroupNonUniformFAdd on a vector type, probed 2026-11-16). The writers
	// carry it: SPIR-V directly, HLSL's Wave* intrinsics take vectors (dxc 6.x
	// compiles them, probed), and the MSL writer expands the operand into one
	// scalar simd call per component, because Metal's simd functions take
	// scalars only.

	{
		name:      "subgroup-add-vector",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAdd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { vec4 v; } o;
void main() { o.v = subgroupAdd(vec4(gl_GlobalInvocationID.x)); }
`,
	},
	{
		name:      "subgroup-and-vector",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAnd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uvec4 v; } o;
void main() { o.v = subgroupAnd(uvec4(gl_GlobalInvocationID.x)); }
`,
	},
	{
		name:      "subgroup-inclusive-add-vector",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveAdd"},
		targetEnv: "vulkan1.1",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uvec2 v; } o;
void main() { o.v = subgroupInclusiveAdd(uvec2(gl_GlobalInvocationID.xy)); }
`,
	},

	// --- the dialect cells ---------------------------------------------------

	{
		name:      "subgroup-gl330-arithmetic",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"subgroupAdd", "subgroupBallot"},
		targetEnv: "vulkan1.1",
		// The GL 3.30 core cell of design §15's matrix: the builtins are
		// available through the extensions (glslang compiles the same source
		// with -V --target-env vulkan1.1 and layout(location) on the output).
		source: `#version 330
#extension GL_KHR_shader_subgroup_arithmetic : require
#extension GL_KHR_shader_subgroup_ballot : require
layout(location = 0) out vec4 color;
void main() {
    uvec4 mask = subgroupBallot(gl_FragCoord.x > 0.5);
    color = vec4(float(subgroupAdd(uint(gl_FragCoord.x))) + float(mask.x));
}
`,
	},
	{
		name:      "subgroup-helper-function",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAdd", "subgroupMax", "subgroupShuffle"},
		targetEnv: "vulkan1.1",
		// A subgroup operation in a user function and in the entry point,
		// sharing the values the statements produce.
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
#extension GL_KHR_shader_subgroup_shuffle : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
uint combine(uint x) {
    return subgroupAdd(x) + subgroupMax(x);
}
void main() {
    uint lane = gl_GlobalInvocationID.x;
    o.v = combine(lane) + subgroupShuffle(lane, 0u);
}
`,
	},

	// --- refusals ------------------------------------------------------------

	{
		name:      "refuse-elect",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupElect"},
		refuse:    "no subgroup elect statement",
		reason: "the IR has no subgroup elect statement (ir has SubgroupBallot, SubgroupCollectiveOperation " +
			"and SubgroupGather); glslangValidator emits OpGroupNonUniformElect for the same source, which the " +
			"frontend cannot represent (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_basic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { if (subgroupElect()) { o.v = 1u; } }
`,
	},
	{
		name:      "refuse-memory-barrier-atomic-counter",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupMemoryBarrierAtomicCounter"},
		refuse:    "atomic counters do not exist in Vulkan GLSL",
		reason: "glslangValidator has no subgroupMemoryBarrierAtomicCounter overload ('no matching overloaded " +
			"function found'), Vulkan GLSL has no atomic counters, and ir.BarrierFlags has no " +
			"atomic-counter memory class (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_basic : require
layout(local_size_x = 64) in;
void main() { subgroupMemoryBarrierAtomicCounter(); }
`,
	},
	{
		name:      "refuse-all-equal",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAllEqual"},
		refuse:    "ir.SubgroupOperation has All and Any but no AllEqual",
		reason: "ir.SubgroupOperation has All and Any but no AllEqual; glslangValidator emits " +
			"OpGroupNonUniformAllEqual for the same source, which no IR statement or operation names " +
			"(card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_vote : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { if (subgroupAllEqual(gl_GlobalInvocationID.x)) { o.v = 1u; } }
`,
	},
	{
		name:      "refuse-ballot-bit-extract",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallotBitExtract"},
		refuse:    "no ballot bit extract, bit count or find LSB/MSB",
		reason: "glslangValidator emits OpGroupNonUniformBallotBitExtract for the same source; the IR builds " +
			"the ballot mask (ir.StmtSubgroupBallot) but has no bit extract expression (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBallotBitExtract(subgroupBallot(true), 1u) ? 1u : 0u; }
`,
	},
	{
		name:      "refuse-ballot-bit-count",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallotBitCount"},
		refuse:    "no ballot bit extract, bit count or find LSB/MSB",
		reason: "glslangValidator emits OpGroupNonUniformBallotBitCount Reduce for the same source; no IR " +
			"statement or expression names a ballot bit count (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBallotBitCount(subgroupBallot(true)); }
`,
	},
	{
		name:      "refuse-ballot-inclusive-bit-count",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallotInclusiveBitCount"},
		refuse:    "no ballot bit extract, bit count or find LSB/MSB",
		reason: "glslangValidator emits OpGroupNonUniformBallotBitCount InclusiveScan for the same source; no " +
			"IR statement or expression names it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBallotInclusiveBitCount(subgroupBallot(true)); }
`,
	},
	{
		name:      "refuse-ballot-exclusive-bit-count",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallotExclusiveBitCount"},
		refuse:    "no ballot bit extract, bit count or find LSB/MSB",
		reason: "glslangValidator emits OpGroupNonUniformBallotBitCount ExclusiveScan for the same source; no " +
			"IR statement or expression names it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBallotExclusiveBitCount(subgroupBallot(true)); }
`,
	},
	{
		name:      "refuse-ballot-find-lsb",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallotFindLSB"},
		refuse:    "no ballot bit extract, bit count or find LSB/MSB",
		reason: "glslangValidator emits OpGroupNonUniformBallotFindLSB for the same source; no IR statement or " +
			"expression names it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBallotFindLSB(subgroupBallot(true)); }
`,
	},
	{
		name:      "refuse-ballot-find-msb",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupBallotFindMSB"},
		refuse:    "no ballot bit extract, bit count or find LSB/MSB",
		reason: "glslangValidator emits OpGroupNonUniformBallotFindMSB for the same source; no IR statement or " +
			"expression names it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_ballot : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupBallotFindMSB(subgroupBallot(true)); }
`,
	},
	{
		name:      "refuse-clustered-add",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredAdd"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformIAdd ClusteredReduce with the " +
			"cluster size, but ir.CollectiveOperation has no ClusteredReduce and no cluster-size operand " +
			"(card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredAdd(gl_GlobalInvocationID.x, 4u); }
`,
	},
	{
		name:      "refuse-clustered-mul",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredMul"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformIMul ClusteredReduce; the IR has " +
			"no clustered collective (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredMul(gl_GlobalInvocationID.x + 1u, 4u); }
`,
	},
	{
		name:      "refuse-clustered-min",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredMin"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformUMin ClusteredReduce; the IR has " +
			"no clustered collective (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredMin(gl_GlobalInvocationID.x, 4u); }
`,
	},
	{
		name:      "refuse-clustered-max",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredMax"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformUMax ClusteredReduce; the IR has " +
			"no clustered collective (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredMax(gl_GlobalInvocationID.x, 4u); }
`,
	},
	{
		name:      "refuse-clustered-and",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredAnd"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformBitwiseAnd ClusteredReduce; the " +
			"IR has no clustered collective (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredAnd(gl_GlobalInvocationID.x, 4u); }
`,
	},
	{
		name:      "refuse-clustered-or",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredOr"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformBitwiseOr ClusteredReduce; the " +
			"IR has no clustered collective (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredOr(gl_GlobalInvocationID.x, 4u); }
`,
	},
	{
		name:      "refuse-clustered-xor",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupClusteredXor"},
		refuse:    "no ClusteredReduce",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformBitwiseXor ClusteredReduce; the " +
			"IR has no clustered collective (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_clustered : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupClusteredXor(gl_GlobalInvocationID.x, 4u); }
`,
	},
	{
		name:      "refuse-inclusive-min-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveMin"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form and emits OpGroupNonUniformUMin InclusiveScan; the IR carries " +
			"it, but the MSL writer emits a plain simd_min (a reduction) and the HLSL writer refuses an " +
			"inclusive scan other than Add and Mul, so no text backend writes the scan (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveMin(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-inclusive-max-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveMax"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_max (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveMax(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-inclusive-and-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveAnd"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_and (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveAnd(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-inclusive-or-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveOr"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_or (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveOr(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-inclusive-xor-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupInclusiveXor"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_xor (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupInclusiveXor(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-exclusive-min-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveMin"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_min (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveMin(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-exclusive-max-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveMax"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_max (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveMax(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-exclusive-and-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveAnd"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_and (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveAnd(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-exclusive-or-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveOr"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_or (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveOr(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-exclusive-xor-scan",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupExclusiveXor"},
		refuse:    "the Min/Max/And/Or/Xor scans are expressible",
		reason: "glslangValidator accepts the form; the MSL writer emits simd_xor (a reduction) and the HLSL " +
			"writer refuses the scan, so no text backend writes it (card P1-GLSLIN-017)",
		source: `#version 450
#extension GL_KHR_shader_subgroup_arithmetic : require
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupExclusiveXor(gl_GlobalInvocationID.x); }
`,
	},
	{
		name:      "refuse-missing-extension",
		stage:     ir.StageCompute,
		extension: ".comp",
		builtins:  []string{"subgroupAdd"},
		refuse:    "'subgroupAdd' : required extension not requested: GL_KHR_shader_subgroup_arithmetic",
		reason: "glslangValidator in Vulkan GLSL requires the family's directive too, even in #version 460 " +
			"(measured: 'subgroupAdd' : required extension not requested: " +
			"GL_KHR_shader_subgroup_arithmetic)",
		source: `#version 450
layout(local_size_x = 64) in;
layout(set = 0, binding = 0) buffer Out { uint v; } o;
void main() { o.v = subgroupAdd(1u); }
`,
	},
	{
		name:      "refuse-gl330-without-extension",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"subgroupAdd"},
		refuse:    "'subgroupAdd' : required extension not requested: GL_KHR_shader_subgroup_arithmetic",
		reason: "glslangValidator in 330 core reports the same rule without the directive ('subgroupAdd' : " +
			"required extension not requested: GL_KHR_shader_subgroup_arithmetic)",
		source: `#version 330
layout(location = 0) out vec4 color;
void main() { color = vec4(float(subgroupAdd(1u))); }
`,
	},
	{
		name:      "refuse-es300",
		stage:     ir.StageFragment,
		extension: ".frag",
		builtins:  []string{"subgroupAdd"},
		refuse:    "'subgroupAdd' : not supported by GLSL ES 3.00 (subgroup operations require GLSL ES 3.10)",
		reason: "GLSL ES has no subgroup operations before 3.10; glslangValidator in 300 es reports " +
			"'subgroupAdd' : no matching overloaded function found even with the extension requested " +
			"(measured in OpenGL mode, since -V refuses a 300 es source outright: 'ES shaders for SPIR-V " +
			"require version 310 or higher')",
		source: `#version 300 es
#extension GL_KHR_shader_subgroup_arithmetic : require
precision highp float;
out vec4 color;
void main() { color = vec4(float(subgroupAdd(1u))); }
`,
	},
}

// glslSubgroupBuiltinNames reads the subgroup builtin names out of
// subgroupBuiltinFunction's switch in glsl/internal/front/subgroup.go. The scan
// is the point: a name added to the catalog fails the coverage test until a
// row drives it.
func glslSubgroupBuiltinNames(t *testing.T) []string {
	t.Helper()
	const path = "../glsl/internal/front/subgroup.go"
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "subgroupBuiltinFunction" {
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
		t.Fatalf("no builtin names found in subgroupBuiltinFunction in %s; its switch shape changed, update this scan", path)
	}
	sort.Strings(names)
	return names
}

// compileGLSLSubgroupModule runs a parsed module through SPIR-V at version 1.3
// (the version glslangValidator emits for a subgroup operation at
// --target-env vulkan1.1; the backend refuses a lower one), through spirv-val
// when the tool is installed, and through the MSL and HLSL backends. It returns
// which step failed: "spirv", "spirv-val", "msl" or "hlsl". A backend the row
// records in backendSkip is asserted to still refuse the module, so a recorded
// limitation cannot rot into a silent skip.
func compileGLSLSubgroupModule(t *testing.T, row glslMatrixRow, module *ir.Module) (string, error) {
	t.Helper()
	binary, _, err := spirv.Compile(module, spirv.Options{Version: spirv.Version1_3})
	if err != nil {
		return "spirv", err
	}
	if validator, ok := testtools.Optional(t, "spirv-val", testtools.InstallSPIRVTools); ok {
		path := filepath.Join(t.TempDir(), "module.spv")
		if err := os.WriteFile(path, binary, 0o600); err != nil {
			return "spirv-val", err
		}
		if out, err := exec.Command(validator, path).CombinedOutput(); err != nil {
			return "spirv-val", fmt.Errorf("spirv-val: %w: %s", err, out)
		}
	}
	if step, err := textBackendStep(t, row, "msl", func() error {
		_, _, cerr := msl.Compile(module, msl.Options{})
		return cerr
	}); err != nil {
		return step, err
	}
	if step, err := textBackendStep(t, row, "hlsl", func() error {
		_, _, cerr := hlsl.Compile(module, hlsl.Options{})
		return cerr
	}); err != nil {
		return step, err
	}
	return "", nil
}

// textBackendStep runs one text backend's step for a subgroup row: an error is
// the step's failure unless the row records the backend as unable to write the
// module, in which case the failure must be the recorded refusal — the error
// must carry the recorded text — and the step passes; a backend the row
// records as unable that nonetheless writes the module, or that refuses with a
// different cause, fails the test.
func textBackendStep(t *testing.T, row glslMatrixRow, name string, compile func() error) (string, error) {
	t.Helper()
	if refusal, recorded := row.backendSkip[name]; recorded {
		assertRecordedRefusal(t, row, name, refusal, compile())
		return "", nil
	}
	if err := compile(); err != nil {
		return name, err
	}
	return "", nil
}

// TestGLSLSubgroupMatrix runs every row through the frontend and, for an ok
// row, through the reference GLSL compiler and all the backends.
func TestGLSLSubgroupMatrix(t *testing.T) {
	glslang, haveGLSLang := testtools.Optional(t, "glslangValidator", glslangInstall)

	for _, row := range glslSubgroupMatrix {
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
			if step, cerr := compileGLSLSubgroupModule(t, row, module); cerr != nil {
				t.Fatalf("%s step: %v\n\nsource:\n%s", step, cerr, row.source)
			}
			// The frontend's own output goes back through the GLSL backend at
			// Vulkan GLSL's version, where the writer emits the subgroup
			// statements: the round trip catches a form the writer cannot
			// express before a consumer meets it.
			if _, _, gerr := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version450}); gerr != nil {
				t.Fatalf("glsl backend round trip: %v\n\nsource:\n%s", gerr, row.source)
			}
		})
	}
}

// TestGLSLSubgroupMatrixCoverage audits the matrix itself: every row states
// exactly one outcome, every builtin it names is one subgroupBuiltinFunction
// handles, and every builtin that switch handles has at least one row.
func TestGLSLSubgroupMatrixCoverage(t *testing.T) {
	names := glslSubgroupBuiltinNames(t)
	catalog := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, duplicate := catalog[name]; duplicate {
			t.Errorf("subgroupBuiltinFunction's switch lists %q twice", name)
		}
		catalog[name] = struct{}{}
	}

	covered := make(map[string]string, len(catalog))
	seen := make(map[string]struct{}, len(glslSubgroupMatrix))
	for _, row := range glslSubgroupMatrix {
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
				t.Errorf("%s: drives %q, which subgroupBuiltinFunction does not handle", row.name, name)
				continue
			}
			if _, taken := covered[name]; !taken {
				covered[name] = row.name
			}
		}
	}

	for _, name := range names {
		if _, ok := covered[name]; !ok {
			t.Errorf("%s is handled by subgroupBuiltinFunction but no matrix row drives it; add a row to glslSubgroupMatrix", name)
		}
	}
}
