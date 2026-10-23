#version 430 core
uniform sampler2D _group_0_binding_0_fs;

uniform sampler2DShadow _group_0_binding_4_fs;

layout(location = 0) out vec4 _fs2p_location0;

float read(sampler2D t, vec2 uv) {
    float _e3 = texture(t, vec2(uv)).x;
    float _e6 = texelFetch(t, ivec2(uv), 0).x;
    return (_e3 + _e6);
}

float compare(sampler2DShadow t_1, vec2 uv_1) {
    float _e4 = texture(t_1, vec3(uv_1, 0.5));
    return _e4;
}

uvec2 size(sampler2DShadow t_2) {
    return uvec2(textureSize(t_2, 0).xy);
}

void main() {
    vec4 p_3 = gl_FragCoord;
    float _e3 = read(_group_0_binding_0_fs, p_3.xy);
    float _e6 = compare(_group_0_binding_4_fs, p_3.xy);
    uvec2 _e8 = size(_group_0_binding_4_fs);
    _fs2p_location0 = vec4(_e3, _e6, vec2(_e8));
    return;
}

