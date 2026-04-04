#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 256, local_size_y = 1, local_size_z = 1) in;

struct Params {
    uint count;
};
shared float shared_data[256];

layout(std430) buffer type_3_block_0Compute { float _group_0_binding_1_cs[]; };


void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        for (uint _naga_zi_0 = 0u; _naga_zi_0 < 256u; _naga_zi_0++) {
            shared_data[_naga_zi_0] = 0.0;
        }
    }
    memoryBarrierShared();
    barrier();
    uint lid = gl_LocalInvocationIndex;
    uvec3 gid = gl_GlobalInvocationID;
    shared_data[lid] = (float(gid.x) * 0.5);
    memoryBarrierShared();
    barrier();
    if ((lid < 128u)) {
        float _e14 = shared_data[lid];
        float _e19 = shared_data[(lid + 128u)];
        shared_data[lid] = (_e14 + _e19);
    }
    memoryBarrierShared();
    barrier();
    if ((lid == 0u)) {
        float _e30 = shared_data[0];
        _group_0_binding_1_cs[(gid.x / 256u)] = _e30;
        return;
    } else {
        return;
    }
}

