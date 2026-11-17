#version 330 core

// Design §15 allows subgroup operations in GL 3.30 through
// GL_KHR_shader_subgroup_*, and glslang accepts this source. The clustered
// forms are still refused: ir.CollectiveOperation has no ClusteredReduce and
// no cluster-size operand (card P2-IR-040, ADR-014 D4), so the operation is
// refused here with that recorded limitation.
#extension GL_KHR_shader_subgroup_clustered : require

layout(location = 0) out vec4 color;

void main() {
    color = vec4(float(subgroupClusteredAdd(1u, 4u)));
}