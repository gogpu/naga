#version 330 core
#extension GL_ARB_shader_texture_image_samples : require
#extension GL_ARB_texture_query_levels : require
struct UniformIndex {
    uint index;
};
struct FragmentIn {
    uint index;
};
unknown_type _group_0_binding_0_fs;

unknown_type _group_0_binding_1_fs;

unknown_type _group_0_binding_2_fs;

unknown_type _group_0_binding_3_fs;

unknown_type _group_0_binding_4_fs;

unknown_type _group_0_binding_5_fs;

unknown_type _group_0_binding_6_fs;

unknown_type _group_0_binding_7_fs;

layout(std140) uniform UniformIndex_block_0Fragment { UniformIndex _group_0_binding_8_fs; };

flat in uint _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    UniformIndex fragment_in = UniformIndex(_vs2fs_location0);
    uint u1_1 = 0u;
    uvec2 u2_1 = uvec2(0u);
    float v1_1 = 0.0;
    vec4 v4_1 = vec4(0.0);
    uint uniform_index = _group_0_binding_8_fs.index;
    uint non_uniform_index = fragment_in.index;
    vec2 uv = vec2(0.0);
    ivec2 pix = ivec2(0);
    uvec2 _e22 = u2_1;
    u2_1 = (_e22 + uvec2(textureSize(_group_0_binding_0_fs[0], 0).xy));
    uvec2 _e27 = u2_1;
    u2_1 = (_e27 + uvec2(textureSize(_group_0_binding_0_fs[uniform_index], 0).xy));
    uvec2 _e32 = u2_1;
    u2_1 = (_e32 + uvec2(textureSize(_group_0_binding_0_fs[non_uniform_index], 0).xy));
    vec4 _e38 = textureGather(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), 0);
    vec4 _e39 = v4_1;
    v4_1 = (_e39 + _e38);
    vec4 _e45 = textureGather(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), 0);
    vec4 _e46 = v4_1;
    v4_1 = (_e46 + _e45);
    vec4 _e52 = textureGather(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), 0);
    vec4 _e53 = v4_1;
    v4_1 = (_e53 + _e52);
    vec4 _e60 = textureGather(_group_0_binding_4_fs[0]__group_0_binding_7_fs[0], vec2(uv), 0.0);
    vec4 _e61 = v4_1;
    v4_1 = (_e61 + _e60);
    vec4 _e68 = textureGather(_group_0_binding_4_fs[uniform_index]__group_0_binding_7_fs[uniform_index], vec2(uv), 0.0);
    vec4 _e69 = v4_1;
    v4_1 = (_e69 + _e68);
    vec4 _e76 = textureGather(_group_0_binding_4_fs[non_uniform_index]__group_0_binding_7_fs[non_uniform_index], vec2(uv), 0.0);
    vec4 _e77 = v4_1;
    v4_1 = (_e77 + _e76);
    vec4 _e82 = texelFetch(_group_0_binding_0_fs[0], pix, 0);
    vec4 _e83 = v4_1;
    v4_1 = (_e83 + _e82);
    vec4 _e88 = texelFetch(_group_0_binding_0_fs[uniform_index], pix, 0);
    vec4 _e89 = v4_1;
    v4_1 = (_e89 + _e88);
    vec4 _e94 = texelFetch(_group_0_binding_0_fs[non_uniform_index], pix, 0);
    vec4 _e95 = v4_1;
    v4_1 = (_e95 + _e94);
    uint _e100 = u1_1;
    u1_1 = (_e100 + uint(textureSize(_group_0_binding_2_fs[0], 0).z));
    uint _e105 = u1_1;
    u1_1 = (_e105 + uint(textureSize(_group_0_binding_2_fs[uniform_index], 0).z));
    uint _e110 = u1_1;
    u1_1 = (_e110 + uint(textureSize(_group_0_binding_2_fs[non_uniform_index], 0).z));
    uint _e115 = u1_1;
    u1_1 = (_e115 + uint(textureQueryLevels(_group_0_binding_1_fs[0])));
    uint _e120 = u1_1;
    u1_1 = (_e120 + uint(textureQueryLevels(_group_0_binding_1_fs[uniform_index])));
    uint _e125 = u1_1;
    u1_1 = (_e125 + uint(textureQueryLevels(_group_0_binding_1_fs[non_uniform_index])));
    uint _e130 = u1_1;
    u1_1 = (_e130 + uint(textureSamples(_group_0_binding_3_fs[0])));
    uint _e135 = u1_1;
    u1_1 = (_e135 + uint(textureSamples(_group_0_binding_3_fs[uniform_index])));
    uint _e140 = u1_1;
    u1_1 = (_e140 + uint(textureSamples(_group_0_binding_3_fs[non_uniform_index])));
    vec4 _e146 = texture(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv));
    vec4 _e147 = v4_1;
    v4_1 = (_e147 + _e146);
    vec4 _e153 = texture(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv));
    vec4 _e154 = v4_1;
    v4_1 = (_e154 + _e153);
    vec4 _e160 = texture(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv));
    vec4 _e161 = v4_1;
    v4_1 = (_e161 + _e160);
    vec4 _e168 = texture(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), 0.0);
    vec4 _e169 = v4_1;
    v4_1 = (_e169 + _e168);
    vec4 _e176 = texture(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), 0.0);
    vec4 _e177 = v4_1;
    v4_1 = (_e177 + _e176);
    vec4 _e184 = texture(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), 0.0);
    vec4 _e185 = v4_1;
    v4_1 = (_e185 + _e184);
    float _e192 = texture(_group_0_binding_4_fs[0]__group_0_binding_7_fs[0], vec3(uv, 0.0));
    float _e193 = v1_1;
    v1_1 = (_e193 + _e192);
    float _e200 = texture(_group_0_binding_4_fs[uniform_index]__group_0_binding_7_fs[uniform_index], vec3(uv, 0.0));
    float _e201 = v1_1;
    v1_1 = (_e201 + _e200);
    float _e208 = texture(_group_0_binding_4_fs[non_uniform_index]__group_0_binding_7_fs[non_uniform_index], vec3(uv, 0.0));
    float _e209 = v1_1;
    v1_1 = (_e209 + _e208);
    float _e216 = textureLod(_group_0_binding_4_fs[0]__group_0_binding_7_fs[0], vec3(uv, 0.0), 0.0);
    float _e217 = v1_1;
    v1_1 = (_e217 + _e216);
    float _e224 = textureLod(_group_0_binding_4_fs[uniform_index]__group_0_binding_7_fs[uniform_index], vec3(uv, 0.0), 0.0);
    float _e225 = v1_1;
    v1_1 = (_e225 + _e224);
    float _e232 = textureLod(_group_0_binding_4_fs[non_uniform_index]__group_0_binding_7_fs[non_uniform_index], vec3(uv, 0.0), 0.0);
    float _e233 = v1_1;
    v1_1 = (_e233 + _e232);
    vec4 _e239 = textureGrad(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), uv, uv);
    vec4 _e240 = v4_1;
    v4_1 = (_e240 + _e239);
    vec4 _e246 = textureGrad(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), uv, uv);
    vec4 _e247 = v4_1;
    v4_1 = (_e247 + _e246);
    vec4 _e253 = textureGrad(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), uv, uv);
    vec4 _e254 = v4_1;
    v4_1 = (_e254 + _e253);
    vec4 _e261 = textureLod(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), 0.0);
    vec4 _e262 = v4_1;
    v4_1 = (_e262 + _e261);
    vec4 _e269 = textureLod(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), 0.0);
    vec4 _e270 = v4_1;
    v4_1 = (_e270 + _e269);
    vec4 _e277 = textureLod(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), 0.0);
    vec4 _e278 = v4_1;
    v4_1 = (_e278 + _e277);
    vec4 _e282 = v4_1;
    imageStore(_group_0_binding_5_fs[0], pix, _e282);
    vec4 _e285 = v4_1;
    imageStore(_group_0_binding_5_fs[uniform_index], pix, _e285);
    vec4 _e288 = v4_1;
    imageStore(_group_0_binding_5_fs[non_uniform_index], pix, _e288);
    uvec2 _e289 = u2_1;
    uint _e290 = u1_1;
    vec2 v2_ = vec2((_e289 + uvec2(_e290)));
    vec4 _e294 = v4_1;
    float _e301 = v1_1;
    _fs2p_location0 = ((_e294 + vec4(v2_.x, v2_.y, v2_.x, v2_.y)) + vec4(_e301));
    return;
}

