#version 330 core
uniform sampler2DShadow _group_0_binding_0_fs;

uniform sampler2DArrayShadow _group_0_binding_1_fs;

uniform sampler2DMS _group_0_binding_2_fs;

layout(location = 0) out vec4 _fs2p_location0;

float test_textureLoad_depth_2d(ivec2 coords, int level) {
    float _e3 = texelFetch(_group_0_binding_0_fs, coords, level);
    return _e3;
}

float test_textureLoad_depth_2d_array_u(ivec2 coords_1, uint index, int level_1) {
    float _e4 = texelFetch(_group_0_binding_1_fs, ivec3(coords_1, index), level_1);
    return _e4;
}

float test_textureLoad_depth_2d_array_s(ivec2 coords_2, int index_1, int level_2) {
    float _e4 = texelFetch(_group_0_binding_1_fs, ivec3(coords_2, index_1), level_2);
    return _e4;
}

float test_textureLoad_depth_multisampled_2d(ivec2 coords_3, int _sample) {
    float _e3 = texelFetch(_group_0_binding_2_fs, coords_3, _sample);
    return _e3;
}

void main() {
    float _e2 = test_textureLoad_depth_2d(ivec2(0), 0);
    float _e6 = test_textureLoad_depth_2d_array_u(ivec2(0), 0u, 0);
    float _e10 = test_textureLoad_depth_2d_array_s(ivec2(0), 0, 0);
    float _e13 = test_textureLoad_depth_multisampled_2d(ivec2(0), 0);
    _fs2p_location0 = vec4(0.0, 0.0, 0.0, 0.0);
    return;
}

