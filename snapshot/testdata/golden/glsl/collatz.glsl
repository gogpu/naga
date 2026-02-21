#version 430 core

struct Data {
    uint values[];
};

layout(std430, binding = 0) buffer data_block { Data data; };

uint collatz_iterations(uint n_base) {
    uint n = n_base;
    uint i = 0u;
    for (;;) {
        if (!((n > 1u))) {
            break;
        }
        if ((_naga_mod(n, 2u) == 0u)) {
            n = (n / 2u);
        } else {
            {
                n = ((3u * n) + 1u);
            }
        }
        i = (i + 1u);
    }
    return i;
}

layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

void main() {
    uint _fc9 = collatz_iterations(data.values[gl_GlobalInvocationID[0]]);
    data.values[gl_GlobalInvocationID[0]] = _fc9;
}
