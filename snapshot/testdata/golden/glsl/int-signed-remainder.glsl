#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

int naga_mod(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}

ivec2 naga_mod(ivec2 lhs, ivec2 rhs) {
    return ivec2(naga_mod(lhs.x, rhs.x), naga_mod(lhs.y, rhs.y));
}

uint naga_mod(uint lhs, uint rhs) {
    uint divisor = (rhs == 0u) ? 1u : rhs;
    return lhs - (lhs / divisor) * divisor;
}


void main() {
    int mod_s = naga_mod(5, 2);
    ivec2 _e3 = ivec2(5);
    ivec2 _e4 = ivec2(2);
    ivec2 mod_s_vec = naga_mod(_e3, _e4);
    uint mod_u = naga_mod(5u, 2u);
    return;
}

