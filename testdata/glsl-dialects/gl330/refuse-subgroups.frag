#version 330 core

// Design §15 allows subgroup operations in GL 3.30 through
// GL_KHR_shader_subgroup_*, and glslang accepts this source. The Min/Max/
// And/Or/Xor scans are still refused: the IR carries them, but Metal's simd
// functions have no prefix form for them and the HLSL writer refuses every
// scan other than the Add and Mul ones (card P1-GLSLIN-017), so the operation
// is refused here with that recorded limitation.
#extension GL_KHR_shader_subgroup_arithmetic : require

layout(location = 0) out vec4 color;

void main() {
    color = vec4(float(subgroupInclusiveMin(1u)));
}
