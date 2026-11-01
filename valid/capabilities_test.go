package valid

import "testing"

// TestCapabilitiesAreRustBits: every capability has the bit of Rust naga's
// valid::Capabilities (valid/mod.rs at the pinned commit), so a value can be
// passed between the two, and the type holds Rust's bits above 31.
func TestCapabilitiesAreRustBits(t *testing.T) {
	for _, tt := range []struct {
		name string
		cap  Capabilities
		rust Capabilities
	}{
		{"FLOAT64", CapFloat64, 1 << 1},
		{"PRIMITIVE_INDEX", CapPrimitiveIndex, 1 << 2},
		{"TEXTURE_AND_SAMPLER_BINDING_ARRAY", CapTextureAndSamplerBindingArray, 1 << 3},
		{"BUFFER_BINDING_ARRAY", CapBufferBindingArray, 1 << 4},
		{"STORAGE_TEXTURE_BINDING_ARRAY", CapStorageTextureBindingArray, 1 << 5},
		{"STORAGE_BUFFER_BINDING_ARRAY", CapStorageBufferBindingArray, 1 << 6},
		{"CLIP_DISTANCES", CapClipDistances, 1 << 7},
		{"STORAGE_TEXTURE_16BIT_NORM_FORMATS", CapStorageTexture16BitNormFormats, 1 << 9},
		{"MULTIVIEW", CapMultiview, 1 << 10},
		{"EARLY_DEPTH_TEST", CapEarlyDepthTest, 1 << 11},
		{"RAY_QUERY", CapRayQuery, 1 << 13},
		{"DUAL_SOURCE_BLENDING", CapDualSourceBlending, 1 << 14},
		{"SHADER_INT64", CapShaderInt64, 1 << 16},
		{"SUBGROUP", CapSubgroup, 1 << 17},
		{"SUBGROUP_BARRIER", CapSubgroupBarrier, 1 << 18},
		{"SUBGROUP_VERTEX_STAGE", CapSubgroupVertexStage, 1 << 19},
		{"SHADER_INT64_ATOMIC_MIN_MAX", CapShaderInt64AtomicMinMax, 1 << 20},
		{"SHADER_INT64_ATOMIC_ALL_OPS", CapShaderInt64AtomicAllOps, 1 << 21},
		{"SHADER_FLOAT32_ATOMIC", CapShaderFloat32Atomic, 1 << 22},
		{"TEXTURE_ATOMIC", CapTextureAtomic, 1 << 23},
		{"TEXTURE_INT64_ATOMIC", CapTextureInt64Atomic, 1 << 24},
		{"SHADER_FLOAT16", CapShaderFloat16, 1 << 26},
		{"SHADER_BARYCENTRICS", CapShaderBarycentrics, 1 << 29},
		{"MESH_SHADER", CapMeshShader, 1 << 30},
		{"MESH_SHADER_POINT_TOPOLOGY", CapMeshShaderPointTopology, 1 << 31},
		{"PER_VERTEX", CapPerVertex, 1 << 37},
		{"RAY_TRACING_PIPELINE", capRayTracingPipeline, 1 << 38},
		{"DRAW_INDEX", CapDrawIndex, 1 << 39},
		{"ACCELERATION_STRUCTURE_BINDING_ARRAY", CapAccelerationStructureBindingArray, 1 << 40},
		{"SHADER_INT16", CapShaderInt16, 1 << 43},
	} {
		if tt.cap != tt.rust {
			t.Errorf("%s = %#x, Rust's is %#x", tt.name, uint64(tt.cap), uint64(tt.rust))
		}
		if got, want := tt.cap.rustDebug(), "Capabilities("+tt.name+")"; got != want {
			t.Errorf("%s prints %q, want %q", tt.name, got, want)
		}
	}
	if got, want := (CapSubgroup | CapSubgroupBarrier).rustDebug(), "Capabilities(SUBGROUP | SUBGROUP_BARRIER)"; got != want {
		t.Errorf("two flags print %q, want %q (Rust bitflags Debug)", got, want)
	}
	if got, want := (SubgroupBallot | SubgroupQuadFragmentCompute).rustDebug(),
		"SubgroupOperationSet(BALLOT | QUAD_FRAGMENT_COMPUTE)"; got != want {
		t.Errorf("subgroup operations print %q, want %q", got, want)
	}
	const debugPrintf Capabilities = 1 << 45 // Rust's highest bit at the pin
	if !CapAll.Contains(debugPrintf) {
		t.Error("CapAll does not hold Rust's bit 45")
	}
}

// TestSubgroupSetsDerivedAsRust: zero Options.SubgroupStages and
// SubgroupOperations are derived from the capabilities as Rust's
// Validator::new (valid/mod.rs:604-626); a non-zero field replaces the
// derived set, as Rust's subgroup_stages()/subgroup_operations().
func TestSubgroupSetsDerivedAsRust(t *testing.T) {
	allOps := SubgroupBasic | SubgroupVote | SubgroupArithmetic | SubgroupBallot | SubgroupShuffle |
		SubgroupShuffleRelative | SubgroupQuadFragmentCompute
	for _, tt := range []struct {
		name   string
		opts   Options
		stages ShaderStages
		ops    SubgroupOperationSet
	}{
		{"none", Options{}, 0, 0},
		{"SUBGROUP", Options{Capabilities: CapSubgroup}, ShaderStagesFragment | ShaderStagesComputeLike, allOps},
		{"SUBGROUP_VERTEX_STAGE alone", Options{Capabilities: CapSubgroupVertexStage}, ShaderStagesVertex, 0},
		{"both", Options{Capabilities: CapSubgroup | CapSubgroupVertexStage},
			ShaderStagesVertex | ShaderStagesFragment | ShaderStagesComputeLike, allOps},
		{"CapAll", Options{Capabilities: CapAll}, ShaderStagesVertex | ShaderStagesFragment | ShaderStagesComputeLike, allOps},
		{"given", Options{Capabilities: CapSubgroup, SubgroupStages: ShaderStagesCompute, SubgroupOperations: SubgroupVote},
			ShaderStagesCompute, SubgroupVote},
	} {
		stages, ops := subgroupSets(tt.opts)
		if stages != tt.stages || ops != tt.ops {
			t.Errorf("%s: stages %#x ops %#x, want %#x %#x", tt.name, stages, ops, tt.stages, tt.ops)
		}
	}
}
