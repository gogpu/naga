#version 430 core
uniform sampler2D _group_0_binding_0_fs;

uniform sampler2DArray _group_0_binding_1_fs;

uniform sampler2DMS _group_0_binding_3_fs;

layout(location = 0) out vec4 _fs2p_location0;

uvec2 size(sampler2DShadow t_2) {
    return uvec2(textureSize(t_2, 0).xy);
}

void main() {
    vec4 p_1 = gl_FragCoord;
    ivec2 c = ivec2(p_1.xy);
    float _e5 = texelFetch(_group_0_binding_0_fs, c, 0).x;
    float _e9 = texelFetch(_group_0_binding_1_fs, ivec3(c, 1), 0).x;
    float _e12 = texelFetch(_group_0_binding_3_fs, c, 2).x;
    _fs2p_location0 = vec4(_e5, _e9, _e12, 1.0);
    return;
}

