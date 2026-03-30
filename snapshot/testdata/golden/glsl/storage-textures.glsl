// === Entry Point: csLoad (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(r32f) readonly uniform image2D _group_0_binding_0_cs;

readonly uniform image2D _group_0_binding_1_cs;

layout(rgba32f) readonly uniform image2D _group_0_binding_2_cs;


void main() {
    vec4 phony = imageLoad(_group_0_binding_0_cs, ivec2(uvec2(0u)));
    vec4 phony_1 = imageLoad(_group_0_binding_1_cs, ivec2(uvec2(0u)));
    vec4 phony_2 = imageLoad(_group_0_binding_2_cs, ivec2(uvec2(0u)));
    return;
}


// === Entry Point: csStore (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(r32f) writeonly uniform image2D _group_1_binding_0_cs;

writeonly uniform image2D _group_1_binding_1_cs;

layout(rgba32f) writeonly uniform image2D _group_1_binding_2_cs;


void main() {
    imageStore(_group_1_binding_0_cs, ivec2(uvec2(0u)), vec4(0.0));
    imageStore(_group_1_binding_1_cs, ivec2(uvec2(0u)), vec4(0.0));
    imageStore(_group_1_binding_2_cs, ivec2(uvec2(0u)), vec4(0.0));
    return;
}

