#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct UniformIndex {
    uint index;
};
struct FragmentIn {
    uint index;
};
layout(std140) uniform type_3_block_0Fragment { unknown_type _group_0_binding_0_fs; };

layout(std140) uniform UniformIndex_block_1Fragment { UniformIndex _group_0_binding_10_fs; };

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
    uint _e29 = u1_1;
    u1_1 = (_e29 + uint(_group_0_binding_0_fs[0].far.length()));
    uint _e35 = u1_1;
    u1_1 = (_e35 + uint(_group_0_binding_0_fs[uniform_index].far.length()));
    uint _e41 = u1_1;
    u1_1 = (_e41 + uint(_group_0_binding_0_fs[non_uniform_index].far.length()));
    uint _e43 = u1_1;
    _fs2p_location0 = _e43;
    return;
}

