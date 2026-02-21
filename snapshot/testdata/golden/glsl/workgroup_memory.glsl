#version 430 core

struct Params {
    uint count;
};

shared float[256] shared_data;
layout(std140, binding = 0) uniform _Params_ubo {
    uint count;
} params;
layout(std430, binding = 1) buffer _output_block { float[] _output; };

layout(local_size_x = 256, local_size_y = 1, local_size_z = 1) in;

void main() {
    shared_data[gl_LocalInvocationIndex] = (float(gl_GlobalInvocationID[0]) * 0.5);
    barrier();
    if ((gl_LocalInvocationIndex < 128u)) {
        shared_data[gl_LocalInvocationIndex] = (shared_data[gl_LocalInvocationIndex] + shared_data[(gl_LocalInvocationIndex + 128u)]);
    }
    barrier();
    if ((gl_LocalInvocationIndex == 0u)) {
        _output[(gl_GlobalInvocationID[0] / 256u)] = shared_data[0];
    }
}
