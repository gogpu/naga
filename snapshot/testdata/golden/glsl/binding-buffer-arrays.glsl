#version 430 core
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
layout(std430) readonly buffer type_4_block_0Fragment { unknown_type _group_0_binding_0_fs; };

layout(std430) readonly buffer PlainData_block_1Fragment {
    uint values[];
} _group_0_binding_1_fs;

layout(std140) uniform UniformIndex_block_2Fragment { UniformIndex _group_0_binding_10_fs; };

layout(location = 0) flat in uint _vs2fs_location0;
layout(location = 0) out uint _fs2p_location0;

void main() {
    UniformIndex fragment_in = UniformIndex(_vs2fs_location0);
    uint u1_1 = 0u;
    uint uniform_index = _group_0_binding_10_fs.index;
    uint non_uniform_index = fragment_in.index;
    uint _e7 = u1_1;
    uint _e11 = _group_0_binding_0_fs[0].x;
    u1_1 = (_e7 + _e11);
    uint _e13 = u1_1;
    uint _e17 = _group_0_binding_0_fs[uniform_index].x;
    u1_1 = (_e13 + _e17);
    uint _e19 = u1_1;
    uint _e23 = _group_0_binding_0_fs[non_uniform_index].x;
    u1_1 = (_e19 + _e23);
    uint _e25 = u1_1;
    uint _e29 = _group_0_binding_0_fs[7].x;
    u1_1 = (_e25 + _e29);
    uint _e31 = u1_1;
    uint _e36 = _group_0_binding_0_fs[0].nested.y;
    u1_1 = (_e31 + _e36);
    uint _e38 = u1_1;
    uint _e43 = _group_0_binding_0_fs[uniform_index].nested.y;
    u1_1 = (_e38 + _e43);
    uint _e45 = u1_1;
    uint _e50 = _group_0_binding_0_fs[non_uniform_index].nested.y;
    u1_1 = (_e45 + _e50);
    uint _e52 = u1_1;
    uint _e57 = _group_0_binding_0_fs[7].nested.y;
    u1_1 = (_e52 + _e57);
    uint _e59 = u1_1;
    u1_1 = (_e59 + uint(_group_0_binding_0_fs[0].far.length()));
    uint _e65 = u1_1;
    u1_1 = (_e65 + uint(_group_0_binding_0_fs[uniform_index].far.length()));
    uint _e71 = u1_1;
    u1_1 = (_e71 + uint(_group_0_binding_0_fs[non_uniform_index].far.length()));
    uint _e77 = u1_1;
    u1_1 = (_e77 + uint(_group_0_binding_0_fs[7].far.length()));
    uint _e83 = u1_1;
    int _e88 = _group_0_binding_0_fs[0].far[0];
    u1_1 = (_e83 + uint(_e88));
    uint _e91 = u1_1;
    int _e96 = _group_0_binding_0_fs[uniform_index].far[0];
    u1_1 = (_e91 + uint(_e96));
    uint _e99 = u1_1;
    int _e104 = _group_0_binding_0_fs[non_uniform_index].far[0];
    u1_1 = (_e99 + uint(_e104));
    uint _e107 = u1_1;
    int _e112 = _group_0_binding_0_fs[7].far[0];
    u1_1 = (_e107 + uint(_e112));
    uint _e115 = u1_1;
    uint _e119 = _group_0_binding_1_fs.values[0];
    u1_1 = (_e115 + _e119);
    uint _e121 = u1_1;
    u1_1 = (_e121 + uint(_group_0_binding_1_fs.values.length()));
    uint _e126 = u1_1;
    _fs2p_location0 = _e126;
    return;
}

