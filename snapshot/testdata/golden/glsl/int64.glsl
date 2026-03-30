#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct UniformCompatible {
    uint val_u32_;
    int val_i32_;
    float val_f32_;
    uint64_t val_u64_;
    uvec2 val_u64_2_;
    uvec3 val_u64_3_;
    uvec4 val_u64_4_;
    int64_t val_i64_;
    ivec2 val_i64_2_;
    ivec3 val_i64_3_;
    ivec4 val_i64_4_;
    uint64_t final_value;
};
struct StorageCompatible {
    uint64_t val_u64_array_2_[2];
    int64_t val_i64_array_2_[2];
};
const uint64_t constant_variable = 20u;

int64_t private_variable = 1l;

layout(std140) uniform UniformCompatible_block_0Compute { UniformCompatible _group_0_binding_0_cs; };

layout(std430) readonly buffer UniformCompatible_block_1Compute { UniformCompatible _group_0_binding_1_cs; };

layout(std430) readonly buffer StorageCompatible_block_2Compute { StorageCompatible _group_0_binding_2_cs; };

layout(std430) buffer UniformCompatible_block_3Compute { UniformCompatible _group_0_binding_3_cs; };

layout(std430) buffer StorageCompatible_block_4Compute { StorageCompatible _group_0_binding_4_cs; };


int64_t int64_function(int64_t x) {
    int64_t val = 20L;
    int64_t phony = private_variable;
    int64_t _e10 = val;
    val = (_e10 + ((31L - 1002003004005006L) + -9223372036854775807L));
    int64_t _e12 = val;
    int64_t _e15 = val;
    val = (_e15 + (_e12 + 5L));
    uint _e19 = _group_0_binding_0_cs.val_u32_;
    int64_t _e20 = val;
    int64_t _e24 = val;
    val = (_e24 + int((_e19 + uint(_e20))));
    int _e28 = _group_0_binding_0_cs.val_i32_;
    int64_t _e29 = val;
    int64_t _e33 = val;
    val = (_e33 + int((_e28 + int(_e29))));
    float _e37 = _group_0_binding_0_cs.val_f32_;
    int64_t _e38 = val;
    int64_t _e42 = val;
    val = (_e42 + int((_e37 + float(_e38))));
    int64_t _e46 = _group_0_binding_0_cs.val_i64_;
    int64_t _e49 = val;
    val = (_e49 + ivec3(_e46).z);
    uint64_t _e53 = _group_0_binding_0_cs.val_u64_;
    int64_t _e55 = val;
    val = (_e55 + int(_e53));
    uvec2 _e59 = _group_0_binding_0_cs.val_u64_2_;
    int64_t _e62 = val;
    val = (_e62 + ivec2(_e59).y);
    uvec3 _e66 = _group_0_binding_0_cs.val_u64_3_;
    int64_t _e69 = val;
    val = (_e69 + ivec3(_e66).z);
    uvec4 _e73 = _group_0_binding_0_cs.val_u64_4_;
    int64_t _e76 = val;
    val = (_e76 + ivec4(_e73).w);
    int64_t _e79 = val;
    val = (_e79 + -9223372036854775808L);
    int64_t _e85 = _group_0_binding_0_cs.val_i64_;
    int64_t _e88 = _group_0_binding_1_cs.val_i64_;
    _group_0_binding_3_cs.val_i64_ = (_e85 + _e88);
    ivec2 _e94 = _group_0_binding_0_cs.val_i64_2_;
    ivec2 _e97 = _group_0_binding_1_cs.val_i64_2_;
    _group_0_binding_3_cs.val_i64_2_ = (_e94 + _e97);
    ivec3 _e103 = _group_0_binding_0_cs.val_i64_3_;
    ivec3 _e106 = _group_0_binding_1_cs.val_i64_3_;
    _group_0_binding_3_cs.val_i64_3_ = (_e103 + _e106);
    ivec4 _e112 = _group_0_binding_0_cs.val_i64_4_;
    ivec4 _e115 = _group_0_binding_1_cs.val_i64_4_;
    _group_0_binding_3_cs.val_i64_4_ = (_e112 + _e115);
    int64_t _e121[2] = _group_0_binding_2_cs.val_i64_array_2_;
    _group_0_binding_4_cs.val_i64_array_2_ = _e121;
    int64_t _e122 = val;
    int64_t _e124 = val;
    val = (_e124 + abs(_e122));
    int64_t _e126 = val;
    int64_t _e127 = val;
    int64_t _e128 = val;
    int64_t _e130 = val;
    val = (_e130 + clamp(_e126, _e127, _e128));
    int64_t _e132 = val;
    int64_t _e134 = val;
    int64_t _e137 = val;
    val = (_e137 + ( + ivec2(_e132).x * ivec2(_e134).x + ivec2(_e132).y * ivec2(_e134).y));
    int64_t _e139 = val;
    int64_t _e140 = val;
    int64_t _e142 = val;
    val = (_e142 + max(_e139, _e140));
    int64_t _e144 = val;
    int64_t _e145 = val;
    int64_t _e147 = val;
    val = (_e147 + min(_e144, _e145));
    int64_t _e149 = val;
    int64_t _e151 = val;
    val = (_e151 + sign(_e149));
    int64_t _e153 = val;
    return _e153;
}

uint64_t uint64_function(uint64_t x_1) {
    uint64_t val_1 = 20uL;
    uint64_t _e8 = val_1;
    val_1 = (_e8 + ((31uL + 18446744073709551615uL) - 18446744073709551615uL));
    uint64_t _e10 = val_1;
    uint64_t _e13 = val_1;
    val_1 = (_e13 + (_e10 + 5uL));
    uint _e17 = _group_0_binding_0_cs.val_u32_;
    uint64_t _e18 = val_1;
    uint64_t _e22 = val_1;
    val_1 = (_e22 + uint((_e17 + uint(_e18))));
    int _e26 = _group_0_binding_0_cs.val_i32_;
    uint64_t _e27 = val_1;
    uint64_t _e31 = val_1;
    val_1 = (_e31 + uint((_e26 + int(_e27))));
    float _e35 = _group_0_binding_0_cs.val_f32_;
    uint64_t _e36 = val_1;
    uint64_t _e40 = val_1;
    val_1 = (_e40 + uint((_e35 + float(_e36))));
    uint64_t _e44 = _group_0_binding_0_cs.val_u64_;
    uint64_t _e47 = val_1;
    val_1 = (_e47 + uvec3(_e44).z);
    int64_t _e51 = _group_0_binding_0_cs.val_i64_;
    uint64_t _e53 = val_1;
    val_1 = (_e53 + uint(_e51));
    ivec2 _e57 = _group_0_binding_0_cs.val_i64_2_;
    uint64_t _e60 = val_1;
    val_1 = (_e60 + uvec2(_e57).y);
    ivec3 _e64 = _group_0_binding_0_cs.val_i64_3_;
    uint64_t _e67 = val_1;
    val_1 = (_e67 + uvec3(_e64).z);
    ivec4 _e71 = _group_0_binding_0_cs.val_i64_4_;
    uint64_t _e74 = val_1;
    val_1 = (_e74 + uvec4(_e71).w);
    uint64_t _e80 = _group_0_binding_0_cs.val_u64_;
    uint64_t _e83 = _group_0_binding_1_cs.val_u64_;
    _group_0_binding_3_cs.val_u64_ = (_e80 + _e83);
    uvec2 _e89 = _group_0_binding_0_cs.val_u64_2_;
    uvec2 _e92 = _group_0_binding_1_cs.val_u64_2_;
    _group_0_binding_3_cs.val_u64_2_ = (_e89 + _e92);
    uvec3 _e98 = _group_0_binding_0_cs.val_u64_3_;
    uvec3 _e101 = _group_0_binding_1_cs.val_u64_3_;
    _group_0_binding_3_cs.val_u64_3_ = (_e98 + _e101);
    uvec4 _e107 = _group_0_binding_0_cs.val_u64_4_;
    uvec4 _e110 = _group_0_binding_1_cs.val_u64_4_;
    _group_0_binding_3_cs.val_u64_4_ = (_e107 + _e110);
    uint64_t _e116[2] = _group_0_binding_2_cs.val_u64_array_2_;
    _group_0_binding_4_cs.val_u64_array_2_ = _e116;
    uint64_t _e117 = val_1;
    uint64_t _e119 = val_1;
    val_1 = (_e119 + abs(_e117));
    uint64_t _e121 = val_1;
    uint64_t _e122 = val_1;
    uint64_t _e123 = val_1;
    uint64_t _e125 = val_1;
    val_1 = (_e125 + clamp(_e121, _e122, _e123));
    uint64_t _e127 = val_1;
    uint64_t _e129 = val_1;
    uint64_t _e132 = val_1;
    val_1 = (_e132 + ( + uvec2(_e127).x * uvec2(_e129).x + uvec2(_e127).y * uvec2(_e129).y));
    uint64_t _e134 = val_1;
    uint64_t _e135 = val_1;
    uint64_t _e137 = val_1;
    val_1 = (_e137 + max(_e134, _e135));
    uint64_t _e139 = val_1;
    uint64_t _e140 = val_1;
    uint64_t _e142 = val_1;
    val_1 = (_e142 + min(_e139, _e140));
    uint64_t _e144 = val_1;
    return _e144;
}

void main() {
    uint64_t _e3 = uint64_function(67uL);
    int64_t _e5 = int64_function(60L);
    _group_0_binding_3_cs.final_value = (_e3 + uint(_e5));
    return;
}

