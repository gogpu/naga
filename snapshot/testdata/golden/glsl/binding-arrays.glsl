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
    uvec2 _e19 = u2_1;
    u2_1 = (_e19 + uvec2(textureSize(_group_0_binding_0_fs[0], 0).xy));
    uvec2 _e24 = u2_1;
    u2_1 = (_e24 + uvec2(textureSize(_group_0_binding_0_fs[uniform_index], 0).xy));
    uvec2 _e29 = u2_1;
    u2_1 = (_e29 + uvec2(textureSize(_group_0_binding_0_fs[non_uniform_index], 0).xy));
    vec4 _e34 = v4_1;
    vec4 _e39 = textureGather(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), 0);
    v4_1 = (_e34 + _e39);
    vec4 _e41 = v4_1;
    vec4 _e46 = textureGather(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), 0);
    v4_1 = (_e41 + _e46);
    vec4 _e48 = v4_1;
    vec4 _e53 = textureGather(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), 0);
    v4_1 = (_e48 + _e53);
    vec4 _e55 = v4_1;
    vec4 _e61 = textureGather(_group_0_binding_4_fs[0]__group_0_binding_7_fs[0], vec2(uv), 0.0);
    v4_1 = (_e55 + _e61);
    vec4 _e63 = v4_1;
    vec4 _e69 = textureGather(_group_0_binding_4_fs[uniform_index]__group_0_binding_7_fs[uniform_index], vec2(uv), 0.0);
    v4_1 = (_e63 + _e69);
    vec4 _e71 = v4_1;
    vec4 _e77 = textureGather(_group_0_binding_4_fs[non_uniform_index]__group_0_binding_7_fs[non_uniform_index], vec2(uv), 0.0);
    v4_1 = (_e71 + _e77);
    vec4 _e79 = v4_1;
    vec4 _e83 = texelFetch(_group_0_binding_0_fs[0], pix, 0);
    v4_1 = (_e79 + _e83);
    vec4 _e85 = v4_1;
    vec4 _e89 = texelFetch(_group_0_binding_0_fs[uniform_index], pix, 0);
    v4_1 = (_e85 + _e89);
    vec4 _e91 = v4_1;
    vec4 _e95 = texelFetch(_group_0_binding_0_fs[non_uniform_index], pix, 0);
    v4_1 = (_e91 + _e95);
    uint _e97 = u1_1;
    u1_1 = (_e97 + uint(textureSize(_group_0_binding_2_fs[0], 0).z));
    uint _e102 = u1_1;
    u1_1 = (_e102 + uint(textureSize(_group_0_binding_2_fs[uniform_index], 0).z));
    uint _e107 = u1_1;
    u1_1 = (_e107 + uint(textureSize(_group_0_binding_2_fs[non_uniform_index], 0).z));
    uint _e112 = u1_1;
    u1_1 = (_e112 + uint(textureQueryLevels(_group_0_binding_1_fs[0])));
    uint _e117 = u1_1;
    u1_1 = (_e117 + uint(textureQueryLevels(_group_0_binding_1_fs[uniform_index])));
    uint _e122 = u1_1;
    u1_1 = (_e122 + uint(textureQueryLevels(_group_0_binding_1_fs[non_uniform_index])));
    uint _e127 = u1_1;
    u1_1 = (_e127 + uint(textureSamples(_group_0_binding_3_fs[0])));
    uint _e132 = u1_1;
    u1_1 = (_e132 + uint(textureSamples(_group_0_binding_3_fs[uniform_index])));
    uint _e137 = u1_1;
    u1_1 = (_e137 + uint(textureSamples(_group_0_binding_3_fs[non_uniform_index])));
    vec4 _e142 = v4_1;
    vec4 _e147 = texture(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv));
    v4_1 = (_e142 + _e147);
    vec4 _e149 = v4_1;
    vec4 _e154 = texture(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv));
    v4_1 = (_e149 + _e154);
    vec4 _e156 = v4_1;
    vec4 _e161 = texture(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv));
    v4_1 = (_e156 + _e161);
    vec4 _e163 = v4_1;
    vec4 _e169 = texture(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), 0.0);
    v4_1 = (_e163 + _e169);
    vec4 _e171 = v4_1;
    vec4 _e177 = texture(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), 0.0);
    v4_1 = (_e171 + _e177);
    vec4 _e179 = v4_1;
    vec4 _e185 = texture(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), 0.0);
    v4_1 = (_e179 + _e185);
    float _e187 = v1_1;
    float _e193 = texture(_group_0_binding_4_fs[0]__group_0_binding_7_fs[0], vec3(uv, 0.0));
    v1_1 = (_e187 + _e193);
    float _e195 = v1_1;
    float _e201 = texture(_group_0_binding_4_fs[uniform_index]__group_0_binding_7_fs[uniform_index], vec3(uv, 0.0));
    v1_1 = (_e195 + _e201);
    float _e203 = v1_1;
    float _e209 = texture(_group_0_binding_4_fs[non_uniform_index]__group_0_binding_7_fs[non_uniform_index], vec3(uv, 0.0));
    v1_1 = (_e203 + _e209);
    float _e211 = v1_1;
    float _e217 = textureLod(_group_0_binding_4_fs[0]__group_0_binding_7_fs[0], vec3(uv, 0.0), 0.0);
    v1_1 = (_e211 + _e217);
    float _e219 = v1_1;
    float _e225 = textureLod(_group_0_binding_4_fs[uniform_index]__group_0_binding_7_fs[uniform_index], vec3(uv, 0.0), 0.0);
    v1_1 = (_e219 + _e225);
    float _e227 = v1_1;
    float _e233 = textureLod(_group_0_binding_4_fs[non_uniform_index]__group_0_binding_7_fs[non_uniform_index], vec3(uv, 0.0), 0.0);
    v1_1 = (_e227 + _e233);
    vec4 _e235 = v4_1;
    vec4 _e240 = textureGrad(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), uv, uv);
    v4_1 = (_e235 + _e240);
    vec4 _e242 = v4_1;
    vec4 _e247 = textureGrad(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), uv, uv);
    v4_1 = (_e242 + _e247);
    vec4 _e249 = v4_1;
    vec4 _e254 = textureGrad(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), uv, uv);
    v4_1 = (_e249 + _e254);
    vec4 _e256 = v4_1;
    vec4 _e262 = textureLod(_group_0_binding_1_fs[0]__group_0_binding_6_fs[0], vec2(uv), 0.0);
    v4_1 = (_e256 + _e262);
    vec4 _e264 = v4_1;
    vec4 _e270 = textureLod(_group_0_binding_1_fs[uniform_index]__group_0_binding_6_fs[uniform_index], vec2(uv), 0.0);
    v4_1 = (_e264 + _e270);
    vec4 _e272 = v4_1;
    vec4 _e278 = textureLod(_group_0_binding_1_fs[non_uniform_index]__group_0_binding_6_fs[non_uniform_index], vec2(uv), 0.0);
    v4_1 = (_e272 + _e278);
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

