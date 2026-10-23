#version 430 core
uniform sampler2DShadow _group_0_binding_4_fs;

layout(location = 0) out vec4 _fs2p_location0;

float compare(sampler2DShadow t_1, vec2 uv_1) {
    float _e4 = texture(t_1, vec3(uv_1, 0.5));
    return _e4;
}

uvec2 size(sampler2DShadow t_2) {
    return uvec2(textureSize(t_2, 0).xy);
}

void main() {
    vec4 p_2 = gl_FragCoord;
    float _e5 = texture(_group_0_binding_4_fs, vec3(p_2.xy, 0.5));
    vec4 _e11 = textureGather(_group_0_binding_4_fs, vec2(p_2.xy), 0.5);
    _fs2p_location0 = (vec4(_e5) + _e11);
    return;
}

