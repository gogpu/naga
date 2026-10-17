#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct UniformIndex {
    uint index;
};
struct Inner {
    uint y;
};
struct FragmentIn {
    uint index;
};
layout(std140) uniform type_4_block_0Fragment { unknown_type _group_0_binding_0_fs; };

layout(std140) uniform PlainData_block_1Fragment { PlainData _group_0_binding_1_fs; };

layout(std140) uniform UniformIndex_block_2Fragment { UniformIndex _group_0_binding_10_fs; };

flat in uint _vs2fs_location0;
layout(location = 0) out uint _fs2p_location0;

void main() {
    UniformIndex fragment_in = UniformIndex(_vs2fs_location0);
    uint u1_1 = 0u;
    uint uniform_index = _group_0_binding_10_fs.index;
    uint non_uniform_index = fragment_in.index;
    uint _e10 = _group_0_binding_0_fs[0].x;
    uint _e11 = u1_1;
    u1_1 = (_e11 + _e10);
    uint _e16 = _group_0_binding_0_fs[uniform_index].x;
    uint _e17 = u1_1;
    u1_1 = (_e17 + _e16);
    uint _e22 = _group_0_binding_0_fs[non_uniform_index].x;
    uint _e23 = u1_1;
    u1_1 = (_e23 + _e22);
    uint _e28 = _group_0_binding_0_fs[7].x;
    uint _e29 = u1_1;
    u1_1 = (_e29 + _e28);
    uint _e35 = _group_0_binding_0_fs[0].nested.y;
    uint _e36 = u1_1;
    u1_1 = (_e36 + _e35);
    uint _e42 = _group_0_binding_0_fs[uniform_index].nested.y;
    uint _e43 = u1_1;
    u1_1 = (_e43 + _e42);
    uint _e49 = _group_0_binding_0_fs[non_uniform_index].nested.y;
    uint _e50 = u1_1;
    u1_1 = (_e50 + _e49);
    uint _e56 = _group_0_binding_0_fs[7].nested.y;
    uint _e57 = u1_1;
    u1_1 = (_e57 + _e56);
    uint _e63 = u1_1;
    u1_1 = (_e63 + uint(_group_0_binding_0_fs[0].far.length()));
    uint _e69 = u1_1;
    u1_1 = (_e69 + uint(_group_0_binding_0_fs[uniform_index].far.length()));
    uint _e75 = u1_1;
    u1_1 = (_e75 + uint(_group_0_binding_0_fs[non_uniform_index].far.length()));
    uint _e81 = u1_1;
    u1_1 = (_e81 + uint(_group_0_binding_0_fs[7].far.length()));
    int _e87 = _group_0_binding_0_fs[0].far[0];
    uint _e89 = u1_1;
    u1_1 = (_e89 + uint(_e87));
    int _e95 = _group_0_binding_0_fs[uniform_index].far[0];
    uint _e97 = u1_1;
    u1_1 = (_e97 + uint(_e95));
    int _e103 = _group_0_binding_0_fs[non_uniform_index].far[0];
    uint _e105 = u1_1;
    u1_1 = (_e105 + uint(_e103));
    int _e111 = _group_0_binding_0_fs[7].far[0];
    uint _e113 = u1_1;
    u1_1 = (_e113 + uint(_e111));
    uint _e118 = _group_0_binding_1_fs.values[0];
    uint _e119 = u1_1;
    u1_1 = (_e119 + _e118);
    uint _e124 = u1_1;
    u1_1 = (_e124 + uint(_group_0_binding_1_fs.values.length()));
    uint _e126 = u1_1;
    _fs2p_location0 = _e126;
    return;
}

