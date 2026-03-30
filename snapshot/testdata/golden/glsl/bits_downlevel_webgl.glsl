#version 330 core

void main() {
    int i_1 = 0;
    ivec2 i2_1 = ivec2(0);
    ivec3 i3_1 = ivec3(0);
    ivec4 i4_1 = ivec4(0);
    uint u_1 = 0u;
    uvec2 u2_1 = uvec2(0u);
    uvec3 u3_1 = uvec3(0u);
    uvec4 u4_1 = uvec4(0u);
    vec2 f2_1 = vec2(0.0);
    vec4 f4_1 = vec4(0.0);
    ivec4 _e23 = i4_1;
    u_1 = uint((_e23[0] & 0xFF) | ((_e23[1] & 0xFF) << 8) | ((_e23[2] & 0xFF) << 16) | ((_e23[3] & 0xFF) << 24));
    uvec4 _e25 = u4_1;
    u_1 = (_e25[0] & 0xFFu) | ((_e25[1] & 0xFFu) << 8) | ((_e25[2] & 0xFFu) << 16) | ((_e25[3] & 0xFFu) << 24);
    uint _e27 = u_1;
    f4_1 = (vec4(ivec4(_e27 << 24, _e27 << 16, _e27 << 8, _e27) >> 24) / 127.0);
    uint _e29 = u_1;
    f4_1 = (vec4(_e29 & 0xFFu, _e29 >> 8 & 0xFFu, _e29 >> 16 & 0xFFu, _e29 >> 24) / 255.0);
    uint _e31 = u_1;
    f2_1 = (vec2(ivec2(_e31 << 16, _e31) >> 16) / 32767.0);
    uint _e33 = u_1;
    f2_1 = (vec2(_e33 & 0xFFFFu, _e33 >> 16) / 65535.0);
    return;
}

