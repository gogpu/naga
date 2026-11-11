#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

int naga_div(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs / divisor;
}

int naga_mod(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

uint naga_div(uint lhs, uint rhs) {
    uint divisor = (rhs == 0u) ? 1u : rhs;
    return lhs / divisor;
}

uint naga_mod(uint lhs, uint rhs) {
    uint divisor = (rhs == 0u) ? 1u : rhs;
    return lhs - (lhs / divisor) * divisor;
}


void main() {
    int div_s = naga_div(5, 2);
    int mod_s = naga_mod(5, 2);
    uint div_u = naga_div(5u, 2u);
    uint mod_u = naga_mod(5u, 2u);
    return;
}

