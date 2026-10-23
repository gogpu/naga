#version 430 core
uniform sampler2D _group_0_binding_0_fs;

uniform sampler2DArray _group_0_binding_1_fs;

uniform samplerCube _group_0_binding_2_fs;

layout(location = 0) out vec4 _fs2p_location0;

float read(sampler2D t, vec2 uv) {
    float _e3 = texture(t, vec2(uv)).x;
    float _e6 = texelFetch(t, ivec2(uv), 0).x;
    return (_e3 + _e6);
}

uvec2 size(sampler2DShadow t_2) {
    return uvec2(textureSize(t_2, 0).xy);
}

void main() {
    vec4 p = gl_FragCoord;
    float _e5 = texture(_group_0_binding_0_fs, vec2(p.xy)).x;
    float _e10 = textureLod(_group_0_binding_1_fs, vec3(p.xy, 2), float(1)).x;
    float _e14 = texture(_group_0_binding_2_fs, vec3(p.xyz)).x;
    vec4 _e20 = textureGather(_group_0_binding_0_fs, vec2(p.xy), 0);
    _fs2p_location0 = (vec4(_e5, _e10, _e14, 1.0) + _e20);
    return;
}

