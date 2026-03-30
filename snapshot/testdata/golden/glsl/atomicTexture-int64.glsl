#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_OES_shader_image_atomic : require
layout(local_size_x = 2, local_size_y = 1, local_size_z = 1) in;

uniform uimage2D _group_0_binding_0_cs;


void main() {
    uvec3 id = gl_LocalInvocationID;
    imageAtomicMax(_group_0_binding_0_cs, ivec2(0, 0), 1uL);
    memoryBarrierShared();
    barrier();
    imageAtomicMin(_group_0_binding_0_cs, ivec2(0, 0), 1uL);
    return;
}

