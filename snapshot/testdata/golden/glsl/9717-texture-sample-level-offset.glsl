#version 330 core
uniform sampler1D _group_0_binding_0_fs;

uniform sampler2D _group_0_binding_1_fs;

uniform sampler3D _group_0_binding_2_fs;

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec4 sample_1d = textureLodOffset(_group_0_binding_0_fs, 0.5, 0.0, -1);
    vec4 sample_2d = textureLodOffset(_group_0_binding_1_fs, vec2(vec2(0.5)), 0.0, ivec2(-1, 2));
    vec4 sample_3d = textureLodOffset(_group_0_binding_2_fs, vec3(vec3(0.5)), 0.0, ivec3(-1, 2, -3));
    _fs2p_location0 = ((sample_1d + sample_2d) + sample_3d);
    return;
}

