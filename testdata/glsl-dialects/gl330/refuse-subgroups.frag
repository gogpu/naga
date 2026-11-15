#version 330 core

// Design §15 allows subgroup operations in GL 3.30 through
// GL_KHR_shader_subgroup_*, and glslang accepts this source. naga's IR has no
// subgroup operations (design §13: they get a card of their own), so the
// operation is refused here with that recorded limitation.
#extension GL_KHR_shader_subgroup_ballot : require

layout(location = 0) out vec4 color;

void main() {
    color = subgroupBallotBitCount(subgroupBallot(true)) > 0u ? vec4(1.0) : vec4(0.0);
}
