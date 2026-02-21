#version 430 core

struct Uniforms {
    vec4 color;
    float scale;
};

layout(std140, binding = 0) uniform _Uniforms_ubo {
    vec4 color;
    float scale;
} uniforms;
shared float[64] shared_data;

layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

void main() {
    shared_data[gl_LocalInvocationIndex] = (uniforms.scale * float(gl_LocalInvocationIndex));
    barrier();
}
