#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

layout(std430) buffer PrimeIndices_block_0Compute {
    uint data[];
} _group_0_binding_0_cs;


uint collatz_iterations(uint n_base) {
    uint n = 0u;
    uint i = 0u;
    n = n_base;
    while(true) {
        uint _e4 = n;
        if ((_e4 > 1u)) {
        } else {
            break;
        }
        {
            uint _e7 = n;
            if (((_e7 % 2u) == 0u)) {
                uint _e12 = n;
                n = (_e12 / 2u);
            } else {
                uint _e16 = n;
                n = ((3u * _e16) + 1u);
            }
            uint _e20 = i;
            i = (_e20 + 1u);
        }
    }
    uint _e23 = i;
    return _e23;
}

void main() {
    uvec3 global_id = gl_GlobalInvocationID;
    uint _e9 = _group_0_binding_0_cs.data[global_id.x];
    uint _e10 = collatz_iterations(_e9);
    _group_0_binding_0_cs.data[global_id.x] = _e10;
    return;
}

