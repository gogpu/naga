package valid

import (
	"fmt"
	"strings"
)

// Capabilities represents hardware/API features available to the shader.
// Mirrors Rust naga valid::Capabilities bitflags.
//
// When compiling WGSL, types like f64, i64, u64, and f16 are only valid
// if the caller enables the corresponding capability. Without it, the
// validator rejects the shader with a descriptive error.
//
// Default value (0) means no extended capabilities are enabled,
// which matches Rust naga's default behavior where only f32/i32/u32/bool
// are allowed without explicit capability opt-in.
//
// The bits are Rust's, which go above bit 31 (DEBUG_PRINTF is 1 << 45 at
// the pin), so the type is as wide as Rust's u64. A constant exists for each
// capability Validate checks; the others arrive with the IR they gate.
//
// Not checked, by design: IMMEDIATES and TEXTURE_EXTERNAL are core WebGPU
// (var<immediate>, texture_external), so a core caller may use them without
// a capability; MULTISAMPLED_SHADING, CUBE_ARRAY_TEXTURES,
// LINEAR_INTERPOLATION and SHADER_FLOAT16_IN_FLOAT32 are core features some
// devices lack, for a future negative-polarity option; the four
// *_NON_UNIFORM_INDEXING capabilities need the uniformity analysis the
// validator does not have yet.
type Capabilities uint64

const (
	// CapFloat64 enables f64 scalar type (Float width=8).
	// Maps to Rust naga valid::Capabilities::FLOAT64 (1 << 1).
	CapFloat64 Capabilities = 1 << 1

	// CapPrimitiveIndex enables @builtin(primitive_index) (Rust
	// PRIMITIVE_INDEX).
	CapPrimitiveIndex Capabilities = 1 << 2

	// CapTextureAndSamplerBindingArray enables binding arrays of sampled
	// and depth textures and of samplers (Rust
	// TEXTURE_AND_SAMPLER_BINDING_ARRAY).
	CapTextureAndSamplerBindingArray Capabilities = 1 << 3

	// CapBufferBindingArray enables binding arrays of uniform buffers (Rust
	// BUFFER_BINDING_ARRAY).
	CapBufferBindingArray Capabilities = 1 << 4

	// CapStorageTextureBindingArray enables binding arrays of storage
	// textures (Rust STORAGE_TEXTURE_BINDING_ARRAY).
	CapStorageTextureBindingArray Capabilities = 1 << 5

	// CapStorageBufferBindingArray enables binding arrays of storage
	// buffers (Rust STORAGE_BUFFER_BINDING_ARRAY).
	CapStorageBufferBindingArray Capabilities = 1 << 6

	// CapClipDistances enables @builtin(clip_distances) (Rust
	// CLIP_DISTANCES).
	CapClipDistances Capabilities = 1 << 7

	// CapCullDistances enables @builtin(cull_distance), the SPIR-V frontend's
	// cull distance (Rust CULL_DISTANCE). Bit 8 already carries the name in
	// capabilityNames; this is its first exported constant.
	CapCullDistances Capabilities = 1 << 8

	// CapStorageTexture16BitNormFormats enables the r16unorm, r16snorm,
	// rg16unorm, rg16snorm, rgba16unorm and rgba16snorm storage texture
	// formats (Rust STORAGE_TEXTURE_16BIT_NORM_FORMATS).
	CapStorageTexture16BitNormFormats Capabilities = 1 << 9

	// CapMultiview enables @builtin(view_index) (Rust MULTIVIEW).
	CapMultiview Capabilities = 1 << 10

	// CapEarlyDepthTest enables a fragment entry point's early depth test
	// (Rust EARLY_DEPTH_TEST).
	CapEarlyDepthTest Capabilities = 1 << 11

	// CapRayQuery enables ray queries and acceleration structures (Rust
	// RAY_QUERY).
	CapRayQuery Capabilities = 1 << 13

	// CapDualSourceBlending enables @blend_src on fragment outputs (Rust
	// DUAL_SOURCE_BLENDING).
	CapDualSourceBlending Capabilities = 1 << 14

	// CapShaderInt64 enables i64 and u64 scalar types (Sint/Uint width=8).
	// Maps to Rust naga valid::Capabilities::SHADER_INT64 (1 << 16).
	CapShaderInt64 Capabilities = 1 << 16

	// CapSubgroup enables subgroup operations (barriers excepted) and the
	// subgroup built-ins in fragment and compute-like shaders (Rust
	// SUBGROUP). With it, a zero Options.SubgroupOperations allows every
	// operation and a zero Options.SubgroupStages the fragment and
	// compute-like stages.
	CapSubgroup Capabilities = 1 << 17

	// CapSubgroupBarrier enables subgroup barriers, together with
	// CapSubgroup (Rust SUBGROUP_BARRIER).
	CapSubgroupBarrier Capabilities = 1 << 18

	// CapSubgroupVertexStage adds the vertex stage to the stages a zero
	// Options.SubgroupStages allows (Rust SUBGROUP_VERTEX_STAGE).
	CapSubgroupVertexStage Capabilities = 1 << 19

	// CapShaderInt64AtomicMinMax enables atomicMin and atomicMax on 64-bit
	// integers in the storage address space, when the result is not used
	// (Rust SHADER_INT64_ATOMIC_MIN_MAX).
	CapShaderInt64AtomicMinMax Capabilities = 1 << 20

	// CapShaderInt64AtomicAllOps enables every atomic operation on 64-bit
	// integers (Rust SHADER_INT64_ATOMIC_ALL_OPS).
	CapShaderInt64AtomicAllOps Capabilities = 1 << 21

	// CapShaderFloat32Atomic enables atomic<f32> with atomicAdd, atomicSub
	// and atomicExchange in the storage address space (Rust
	// SHADER_FLOAT32_ATOMIC).
	CapShaderFloat32Atomic Capabilities = 1 << 22

	// CapTextureAtomic enables atomics on r32sint and r32uint storage
	// textures (Rust TEXTURE_ATOMIC).
	CapTextureAtomic Capabilities = 1 << 23

	// CapTextureInt64Atomic enables atomics on r64uint storage textures
	// (Rust TEXTURE_INT64_ATOMIC).
	CapTextureInt64Atomic Capabilities = 1 << 24

	// CapShaderFloat16 enables f16 scalar type (Float width=2).
	// Maps to Rust naga valid::Capabilities::SHADER_FLOAT16 (1 << 26).
	CapShaderFloat16 Capabilities = 1 << 26

	// CapShaderBarycentrics enables @builtin(barycentric) and
	// @builtin(barycentric_no_perspective) (Rust SHADER_BARYCENTRICS).
	CapShaderBarycentrics Capabilities = 1 << 29

	// CapMeshShader enables task and mesh shaders, the mesh built-ins, the
	// task_payload address space and @per_primitive (Rust MESH_SHADER).
	CapMeshShader Capabilities = 1 << 30

	// CapMeshShaderPointTopology enables mesh shaders that output points
	// (Rust MESH_SHADER_POINT_TOPOLOGY).
	CapMeshShaderPointTopology Capabilities = 1 << 31

	// CapPerVertex enables @interpolate(per_vertex) fragment inputs (Rust
	// PER_VERTEX).
	CapPerVertex Capabilities = 1 << 37

	// CapDrawIndex enables @builtin(draw_index) (Rust DRAW_INDEX).
	CapDrawIndex Capabilities = 1 << 39

	// CapAccelerationStructureBindingArray enables binding arrays of
	// acceleration structures (Rust ACCELERATION_STRUCTURE_BINDING_ARRAY).
	CapAccelerationStructureBindingArray Capabilities = 1 << 40

	// CapMemoryDecorationCoherent enables the Coherent memory decoration on
	// storage variables (Rust MEMORY_DECORATION_COHERENT).
	CapMemoryDecorationCoherent Capabilities = 1 << 41

	// CapMemoryDecorationVolatile enables the Volatile memory decoration on
	// storage variables (Rust MEMORY_DECORATION_VOLATILE).
	CapMemoryDecorationVolatile Capabilities = 1 << 42

	// CapShaderInt16 enables i16 and u16 scalar types (Sint/Uint width=2;
	// Rust SHADER_INT16).
	CapShaderInt16 Capabilities = 1 << 43

	// CapAll enables all capabilities, those added later included. Used by
	// test infrastructure and tools that need to accept any valid WGSL
	// without restriction.
	CapAll Capabilities = ^Capabilities(0)
)

// capRayTracingPipeline is Rust's RAY_TRACING_PIPELINE, which allows an
// acceleration structure as RAY_QUERY does. The IR has no ray tracing
// pipeline stages yet, so it has no exported constant.
const capRayTracingPipeline Capabilities = 1 << 38

// Contains reports whether c includes all the flags in flag.
func (c Capabilities) Contains(flag Capabilities) bool {
	return c&flag == flag
}

// capabilityNames are Rust's flag names, by bit, for the Debug text Rust's
// error messages print (bitflags: "Capabilities(SUBGROUP | SUBGROUP_BARRIER)").
var capabilityNames = [...]string{
	"IMMEDIATES", "FLOAT64", "PRIMITIVE_INDEX", "TEXTURE_AND_SAMPLER_BINDING_ARRAY",
	"BUFFER_BINDING_ARRAY", "STORAGE_TEXTURE_BINDING_ARRAY", "STORAGE_BUFFER_BINDING_ARRAY",
	"CLIP_DISTANCES", "CULL_DISTANCE", "STORAGE_TEXTURE_16BIT_NORM_FORMATS", "MULTIVIEW",
	"EARLY_DEPTH_TEST", "MULTISAMPLED_SHADING", "RAY_QUERY", "DUAL_SOURCE_BLENDING",
	"CUBE_ARRAY_TEXTURES", "SHADER_INT64", "SUBGROUP", "SUBGROUP_BARRIER",
	"SUBGROUP_VERTEX_STAGE", "SHADER_INT64_ATOMIC_MIN_MAX", "SHADER_INT64_ATOMIC_ALL_OPS",
	"SHADER_FLOAT32_ATOMIC", "TEXTURE_ATOMIC", "TEXTURE_INT64_ATOMIC",
	"RAY_HIT_VERTEX_POSITION", "SHADER_FLOAT16", "TEXTURE_EXTERNAL",
	"SHADER_FLOAT16_IN_FLOAT32", "SHADER_BARYCENTRICS", "MESH_SHADER",
	"MESH_SHADER_POINT_TOPOLOGY", "TEXTURE_AND_SAMPLER_BINDING_ARRAY_NON_UNIFORM_INDEXING",
	"BUFFER_BINDING_ARRAY_NON_UNIFORM_INDEXING", "STORAGE_TEXTURE_BINDING_ARRAY_NON_UNIFORM_INDEXING",
	"STORAGE_BUFFER_BINDING_ARRAY_NON_UNIFORM_INDEXING", "COOPERATIVE_MATRIX", "PER_VERTEX",
	"RAY_TRACING_PIPELINE", "DRAW_INDEX", "ACCELERATION_STRUCTURE_BINDING_ARRAY",
	"MEMORY_DECORATION_COHERENT", "MEMORY_DECORATION_VOLATILE", "SHADER_INT16",
	"LINEAR_INTERPOLATION", "DEBUG_PRINTF",
}

// rustDebug is c as Rust's Debug prints it in an error message.
func (c Capabilities) rustDebug() string {
	return flagsDebug("Capabilities", uint64(c), capabilityNames[:])
}

// String is c as Rust's bitflags Debug prints it, "Capabilities(SUBGROUP |
// SUBGROUP_BARRIER)": the text of every missing-capability error, which a
// caller can match on the flag names.
func (c Capabilities) String() string {
	return c.rustDebug()
}

// flagsDebug renders a bitflags value as Rust's bitflags Debug does: the
// set flags by name in declaration order, a leftover as hex.
func flagsDebug(typeName string, bits uint64, names []string) string {
	var parts []string
	for i, name := range names {
		if name != "" && bits&(1<<i) != 0 {
			parts = append(parts, name)
			bits &^= 1 << i
		}
	}
	if bits != 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%#x", bits))
	}
	return typeName + "(" + strings.Join(parts, " | ") + ")"
}

// ShaderStages is a set of shader stages: Rust naga's valid::ShaderStages,
// with Rust's bits. The ray tracing stages arrive with their IR.
type ShaderStages uint16

const (
	// ShaderStagesVertex is the vertex stage (Rust VERTEX).
	ShaderStagesVertex ShaderStages = 0x1
	// ShaderStagesFragment is the fragment stage (Rust FRAGMENT).
	ShaderStagesFragment ShaderStages = 0x2
	// ShaderStagesCompute is the compute stage (Rust COMPUTE).
	ShaderStagesCompute ShaderStages = 0x4
	// ShaderStagesMesh is the mesh stage (Rust MESH).
	ShaderStagesMesh ShaderStages = 0x8
	// ShaderStagesTask is the task stage (Rust TASK).
	ShaderStagesTask ShaderStages = 0x10
	// ShaderStagesComputeLike is the compute, task and mesh stages (Rust
	// COMPUTE_LIKE).
	ShaderStagesComputeLike = ShaderStagesCompute | ShaderStagesTask | ShaderStagesMesh
)

// SubgroupOperationSet is a set of subgroup operation classes: Rust naga's
// valid::SubgroupOperationSet, with Rust's bits (bit 6, clustered, and bit
// 8, quad in all stages, are not supported by Rust either).
type SubgroupOperationSet uint8

const (
	// SubgroupBasic is subgroup barriers (Rust BASIC).
	SubgroupBasic SubgroupOperationSet = 1 << 0
	// SubgroupVote is subgroupAll and subgroupAny (Rust VOTE).
	SubgroupVote SubgroupOperationSet = 1 << 1
	// SubgroupArithmetic is the reductions and scans (Rust ARITHMETIC).
	SubgroupArithmetic SubgroupOperationSet = 1 << 2
	// SubgroupBallot is ballot and broadcast (Rust BALLOT).
	SubgroupBallot SubgroupOperationSet = 1 << 3
	// SubgroupShuffle is shuffle and shuffle xor (Rust SHUFFLE).
	SubgroupShuffle SubgroupOperationSet = 1 << 4
	// SubgroupShuffleRelative is shuffle up and down (Rust
	// SHUFFLE_RELATIVE).
	SubgroupShuffleRelative SubgroupOperationSet = 1 << 5
	// SubgroupQuadFragmentCompute is the quad operations in fragment and
	// compute shaders (Rust QUAD_FRAGMENT_COMPUTE).
	SubgroupQuadFragmentCompute SubgroupOperationSet = 1 << 7
)

// subgroupOperationNames are Rust's flag names, by bit.
var subgroupOperationNames = [...]string{
	"BASIC", "VOTE", "ARITHMETIC", "BALLOT", "SHUFFLE", "SHUFFLE_RELATIVE", "", "QUAD_FRAGMENT_COMPUTE",
}

// rustDebug is s as Rust's Debug prints it in an error message.
func (s SubgroupOperationSet) rustDebug() string {
	return flagsDebug("SubgroupOperationSet", uint64(s), subgroupOperationNames[:])
}

// subgroupSets returns the subgroup stages and operations a validator with
// opts allows: a non-zero field as given (Rust's Validator::subgroup_stages
// and subgroup_operations), a zero one derived from the capabilities as
// Rust's Validator::new does.
func subgroupSets(opts Options) (ShaderStages, SubgroupOperationSet) {
	caps := opts.Capabilities
	stages := opts.SubgroupStages
	if stages == 0 {
		if caps.Contains(CapSubgroupVertexStage) {
			stages |= ShaderStagesVertex
		}
		if caps.Contains(CapSubgroup) {
			stages |= ShaderStagesFragment | ShaderStagesComputeLike
		}
	}
	ops := opts.SubgroupOperations
	if ops == 0 && caps.Contains(CapSubgroup) {
		ops = SubgroupBasic | SubgroupVote | SubgroupArithmetic | SubgroupBallot |
			SubgroupShuffle | SubgroupShuffleRelative | SubgroupQuadFragmentCompute
	}
	return stages, ops
}

// ParseCapabilities parses a comma-separated list of Rust naga capability flag
// names — the spelling Validate's errors print, e.g. "SHADER_INT64" — or
// "all" for every capability. An empty list is no capabilities; an unknown
// name is an error that names it.
func ParseCapabilities(list string) (Capabilities, error) {
	switch strings.TrimSpace(list) {
	case "":
		return 0, nil
	case "all":
		return CapAll, nil
	}
	var capabilities Capabilities
	for _, word := range strings.Split(list, ",") {
		name := strings.TrimSpace(word)
		if name == "" {
			continue
		}
		bit, ok := capabilityBit(name)
		if !ok {
			return 0, fmt.Errorf("unknown capability %q", name)
		}
		capabilities |= bit
	}
	return capabilities, nil
}

// capabilityBit returns the bit of a Rust flag name.
func capabilityBit(name string) (Capabilities, bool) {
	for bit, flag := range capabilityNames[:] {
		if flag == name {
			return Capabilities(1) << bit, true
		}
	}
	return 0, false
}
