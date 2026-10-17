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
    int64_t _e5 = val;
    val = (_e5 + ((31L - 1002003004005006L) + -9223372036854775807L));
    int64_t _e12 = val;
    int64_t _e13 = val;
    val = (_e12 + (_e13 + 5L));
    int64_t _e17 = val;
    uint _e20 = _group_0_binding_0_cs.val_u32_;
    int64_t _e21 = val;
    val = (_e17 + int((_e20 + uint(_e21))));
    int64_t _e26 = val;
    int _e29 = _group_0_binding_0_cs.val_i32_;
    int64_t _e30 = val;
    val = (_e26 + int((_e29 + int(_e30))));
    int64_t _e35 = val;
    float _e38 = _group_0_binding_0_cs.val_f32_;
    int64_t _e39 = val;
    val = (_e35 + int((_e38 + float(_e39))));
    int64_t _e44 = val;
    int64_t _e47 = _group_0_binding_0_cs.val_i64_;
    val = (_e44 + ivec3(_e47).z);
    int64_t _e51 = val;
    uint64_t _e54 = _group_0_binding_0_cs.val_u64_;
    val = (_e51 + int(_e54));
    int64_t _e57 = val;
    uvec2 _e60 = _group_0_binding_0_cs.val_u64_2_;
    val = (_e57 + ivec2(_e60).y);
    int64_t _e64 = val;
    uvec3 _e67 = _group_0_binding_0_cs.val_u64_3_;
    val = (_e64 + ivec3(_e67).z);
    int64_t _e71 = val;
    uvec4 _e74 = _group_0_binding_0_cs.val_u64_4_;
    val = (_e71 + ivec4(_e74).w);
    int64_t _e78 = val;
    val = (_e78 + -9223372036854775808L);
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
    int64_t _e123 = val;
    val = (_e122 + abs(_e123));
    int64_t _e126 = val;
    int64_t _e127 = val;
    int64_t _e128 = val;
    int64_t _e129 = val;
    val = (_e126 + clamp(_e127, _e128, _e129));
    int64_t _e132 = val;
    int64_t _e133 = val;
    int64_t _e135 = val;
    val = (_e132 + ( + ivec2(_e133).x * ivec2(_e135).x + ivec2(_e133).y * ivec2(_e135).y));
    int64_t _e139 = val;
    int64_t _e140 = val;
    int64_t _e141 = val;
    val = (_e139 + max(_e140, _e141));
    int64_t _e144 = val;
    int64_t _e145 = val;
    int64_t _e146 = val;
    val = (_e144 + min(_e145, _e146));
    int64_t _e149 = val;
    int64_t _e150 = val;
    val = (_e149 + sign(_e150));
    int64_t _e153 = val;
    return _e153;
}

uint64_t uint64_function(uint64_t x_1) {
    uint64_t val_1 = 20uL;
    uint64_t _e3 = val_1;
    val_1 = (_e3 + ((31uL + 18446744073709551615uL) - 18446744073709551615uL));
    uint64_t _e10 = val_1;
    uint64_t _e11 = val_1;
    val_1 = (_e10 + (_e11 + 5uL));
    uint64_t _e15 = val_1;
    uint _e18 = _group_0_binding_0_cs.val_u32_;
    uint64_t _e19 = val_1;
    val_1 = (_e15 + uint((_e18 + uint(_e19))));
    uint64_t _e24 = val_1;
    int _e27 = _group_0_binding_0_cs.val_i32_;
    uint64_t _e28 = val_1;
    val_1 = (_e24 + uint((_e27 + int(_e28))));
    uint64_t _e33 = val_1;
    float _e36 = _group_0_binding_0_cs.val_f32_;
    uint64_t _e37 = val_1;
    val_1 = (_e33 + uint((_e36 + float(_e37))));
    uint64_t _e42 = val_1;
    uint64_t _e45 = _group_0_binding_0_cs.val_u64_;
    val_1 = (_e42 + uvec3(_e45).z);
    uint64_t _e49 = val_1;
    int64_t _e52 = _group_0_binding_0_cs.val_i64_;
    val_1 = (_e49 + uint(_e52));
    uint64_t _e55 = val_1;
    ivec2 _e58 = _group_0_binding_0_cs.val_i64_2_;
    val_1 = (_e55 + uvec2(_e58).y);
    uint64_t _e62 = val_1;
    ivec3 _e65 = _group_0_binding_0_cs.val_i64_3_;
    val_1 = (_e62 + uvec3(_e65).z);
    uint64_t _e69 = val_1;
    ivec4 _e72 = _group_0_binding_0_cs.val_i64_4_;
    val_1 = (_e69 + uvec4(_e72).w);
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
    uint64_t _e118 = val_1;
    val_1 = (_e117 + abs(_e118));
    uint64_t _e121 = val_1;
    uint64_t _e122 = val_1;
    uint64_t _e123 = val_1;
    uint64_t _e124 = val_1;
    val_1 = (_e121 + clamp(_e122, _e123, _e124));
    uint64_t _e127 = val_1;
    uint64_t _e128 = val_1;
    uint64_t _e130 = val_1;
    val_1 = (_e127 + ( + uvec2(_e128).x * uvec2(_e130).x + uvec2(_e128).y * uvec2(_e130).y));
    uint64_t _e134 = val_1;
    uint64_t _e135 = val_1;
    uint64_t _e136 = val_1;
    val_1 = (_e134 + max(_e135, _e136));
    uint64_t _e139 = val_1;
    uint64_t _e140 = val_1;
    uint64_t _e141 = val_1;
    val_1 = (_e139 + min(_e140, _e141));
    uint64_t _e144 = val_1;
    return _e144;
}

void main() {
    uint64_t _e3 = uint64_function(67uL);
    int64_t _e5 = int64_function(60L);
    _group_0_binding_3_cs.final_value = (_e3 + uint(_e5));
    return;
}

