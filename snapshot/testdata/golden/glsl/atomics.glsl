#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

shared uint shared_counter;

layout(std430) buffer type_2_block_0Compute { uint _group_0_binding_0_cs[]; };


void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        shared_counter = 0u;
    }
    memoryBarrierShared();
    barrier();
    uint lid = gl_LocalInvocationIndex;
    if ((lid == 0u)) {
        shared_counter = 0u;
    }
    memoryBarrierShared();
    barrier();
    uint _e7 = atomicAdd(shared_counter, 1u);
    memoryBarrierShared();
    barrier();
    if ((lid == 0u)) {
        uint _e13 = shared_counter;
        _group_0_binding_0_cs[0] = _e13;
        return;
    } else {
        return;
    }
}

