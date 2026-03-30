#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct UniformCompatible {
    uint val_u32_;
    int val_i32_;
    float val_f32_;
    float16_t val_f16_;
    vec2 val_f16_2_;
    vec3 val_f16_3_;
    vec4 val_f16_4_;
    float16_t final_value;
    mat2x2 val_mat2x2_;
    mat2x3 val_mat2x3_;
    mat2x4 val_mat2x4_;
    mat3x2 val_mat3x2_;
    mat3x3 val_mat3x3_;
    mat3x4 val_mat3x4_;
    mat4x2 val_mat4x2_;
    mat4x3 val_mat4x3_;
    mat4x4 val_mat4x4_;
};
struct StorageCompatible {
    float16_t val_f16_array_2_[2];
};
struct LayoutTest {
    float16_t scalar1_;
    float16_t scalar2_;
    vec3 v3_;
    float16_t tuck_in;
    float16_t scalar4_;
    uint larger;
};
const float16_t constant_variable = 9.562e-320LF;

float16_t private_variable = 0.0;

layout(std140) uniform UniformCompatible_block_0Compute { UniformCompatible _group_0_binding_0_cs; };

layout(std430) readonly buffer UniformCompatible_block_1Compute { UniformCompatible _group_0_binding_1_cs; };

layout(std430) readonly buffer StorageCompatible_block_2Compute { StorageCompatible _group_0_binding_2_cs; };

layout(std430) buffer UniformCompatible_block_3Compute { UniformCompatible _group_0_binding_3_cs; };

layout(std430) buffer StorageCompatible_block_4Compute { StorageCompatible _group_0_binding_4_cs; };


float16_t f16_function(float16_t x) {
    LayoutTest l = LayoutTest(0.0, 0.0, vec3(0.0), 0.0, 0.0, 0u);
    float16_t val = 0;
    float16_t phony = private_variable;
    float16_t _e6 = val;
    val = (_e6 + 0);
    float16_t _e8 = val;
    float16_t _e11 = val;
    val = (_e11 + (_e8 + 0));
    float _e15 = _group_0_binding_0_cs.val_f32_;
    float16_t _e16 = val;
    float16_t _e20 = val;
    val = (_e20 + float((_e15 + float(_e16))));
    float16_t _e24 = _group_0_binding_0_cs.val_f16_;
    float16_t _e27 = val;
    val = (_e27 + vec3(_e24).z);
    _group_0_binding_3_cs.val_i32_ = 65504;
    _group_0_binding_3_cs.val_i32_ = -65504;
    _group_0_binding_3_cs.val_u32_ = 65504u;
    _group_0_binding_3_cs.val_u32_ = 0u;
    _group_0_binding_3_cs.val_f32_ = 65504.0;
    _group_0_binding_3_cs.val_f32_ = -65504.0;
    float16_t _e51 = _group_0_binding_0_cs.val_f16_;
    float16_t _e54 = _group_0_binding_1_cs.val_f16_;
    _group_0_binding_3_cs.val_f16_ = (_e51 + _e54);
    vec2 _e60 = _group_0_binding_0_cs.val_f16_2_;
    vec2 _e63 = _group_0_binding_1_cs.val_f16_2_;
    _group_0_binding_3_cs.val_f16_2_ = (_e60 + _e63);
    vec3 _e69 = _group_0_binding_0_cs.val_f16_3_;
    vec3 _e72 = _group_0_binding_1_cs.val_f16_3_;
    _group_0_binding_3_cs.val_f16_3_ = (_e69 + _e72);
    vec4 _e78 = _group_0_binding_0_cs.val_f16_4_;
    vec4 _e81 = _group_0_binding_1_cs.val_f16_4_;
    _group_0_binding_3_cs.val_f16_4_ = (_e78 + _e81);
    mat2x2 _e87 = _group_0_binding_0_cs.val_mat2x2_;
    mat2x2 _e90 = _group_0_binding_1_cs.val_mat2x2_;
    _group_0_binding_3_cs.val_mat2x2_ = (_e87 + _e90);
    mat2x3 _e96 = _group_0_binding_0_cs.val_mat2x3_;
    mat2x3 _e99 = _group_0_binding_1_cs.val_mat2x3_;
    _group_0_binding_3_cs.val_mat2x3_ = (_e96 + _e99);
    mat2x4 _e105 = _group_0_binding_0_cs.val_mat2x4_;
    mat2x4 _e108 = _group_0_binding_1_cs.val_mat2x4_;
    _group_0_binding_3_cs.val_mat2x4_ = (_e105 + _e108);
    mat3x2 _e114 = _group_0_binding_0_cs.val_mat3x2_;
    mat3x2 _e117 = _group_0_binding_1_cs.val_mat3x2_;
    _group_0_binding_3_cs.val_mat3x2_ = (_e114 + _e117);
    mat3x3 _e123 = _group_0_binding_0_cs.val_mat3x3_;
    mat3x3 _e126 = _group_0_binding_1_cs.val_mat3x3_;
    _group_0_binding_3_cs.val_mat3x3_ = (_e123 + _e126);
    mat3x4 _e132 = _group_0_binding_0_cs.val_mat3x4_;
    mat3x4 _e135 = _group_0_binding_1_cs.val_mat3x4_;
    _group_0_binding_3_cs.val_mat3x4_ = (_e132 + _e135);
    mat4x2 _e141 = _group_0_binding_0_cs.val_mat4x2_;
    mat4x2 _e144 = _group_0_binding_1_cs.val_mat4x2_;
    _group_0_binding_3_cs.val_mat4x2_ = (_e141 + _e144);
    mat4x3 _e150 = _group_0_binding_0_cs.val_mat4x3_;
    mat4x3 _e153 = _group_0_binding_1_cs.val_mat4x3_;
    _group_0_binding_3_cs.val_mat4x3_ = (_e150 + _e153);
    mat4x4 _e159 = _group_0_binding_0_cs.val_mat4x4_;
    mat4x4 _e162 = _group_0_binding_1_cs.val_mat4x4_;
    _group_0_binding_3_cs.val_mat4x4_ = (_e159 + _e162);
    float16_t _e168[2] = _group_0_binding_2_cs.val_f16_array_2_;
    _group_0_binding_4_cs.val_f16_array_2_ = _e168;
    float16_t _e169 = val;
    float16_t _e171 = val;
    val = (_e171 + abs(_e169));
    float16_t _e173 = val;
    float16_t _e174 = val;
    float16_t _e175 = val;
    float16_t _e177 = val;
    val = (_e177 + clamp(_e173, _e174, _e175));
    float16_t _e179 = val;
    float16_t _e181 = val;
    float16_t _e184 = val;
    val = (_e184 + dot(vec2(_e179), vec2(_e181)));
    float16_t _e186 = val;
    float16_t _e187 = val;
    float16_t _e189 = val;
    val = (_e189 + max(_e186, _e187));
    float16_t _e191 = val;
    float16_t _e192 = val;
    float16_t _e194 = val;
    val = (_e194 + min(_e191, _e192));
    float16_t _e196 = val;
    float16_t _e198 = val;
    val = (_e198 + sign(_e196));
    float16_t _e201 = val;
    val = (_e201 + 0);
    vec2 _e205 = _group_0_binding_0_cs.val_f16_2_;
    vec2 float_vec2_ = vec2(_e205);
    _group_0_binding_3_cs.val_f16_2_ = vec2(float_vec2_);
    vec3 _e212 = _group_0_binding_0_cs.val_f16_3_;
    vec3 float_vec3_ = vec3(_e212);
    _group_0_binding_3_cs.val_f16_3_ = vec3(float_vec3_);
    vec4 _e219 = _group_0_binding_0_cs.val_f16_4_;
    vec4 float_vec4_ = vec4(_e219);
    _group_0_binding_3_cs.val_f16_4_ = vec4(float_vec4_);
    mat2x2 _e228 = _group_0_binding_0_cs.val_mat2x2_;
    _group_0_binding_3_cs.val_mat2x2_ = mat2x2(mat2x2(_e228));
    mat2x3 _e235 = _group_0_binding_0_cs.val_mat2x3_;
    _group_0_binding_3_cs.val_mat2x3_ = mat2x3(mat2x3(_e235));
    mat2x4 _e242 = _group_0_binding_0_cs.val_mat2x4_;
    _group_0_binding_3_cs.val_mat2x4_ = mat2x4(mat2x4(_e242));
    mat3x2 _e249 = _group_0_binding_0_cs.val_mat3x2_;
    _group_0_binding_3_cs.val_mat3x2_ = mat3x2(mat3x2(_e249));
    mat3x3 _e256 = _group_0_binding_0_cs.val_mat3x3_;
    _group_0_binding_3_cs.val_mat3x3_ = mat3x3(mat3x3(_e256));
    mat3x4 _e263 = _group_0_binding_0_cs.val_mat3x4_;
    _group_0_binding_3_cs.val_mat3x4_ = mat3x4(mat3x4(_e263));
    mat4x2 _e270 = _group_0_binding_0_cs.val_mat4x2_;
    _group_0_binding_3_cs.val_mat4x2_ = mat4x2(mat4x2(_e270));
    mat4x3 _e277 = _group_0_binding_0_cs.val_mat4x3_;
    _group_0_binding_3_cs.val_mat4x3_ = mat4x3(mat4x3(_e277));
    mat4x4 _e284 = _group_0_binding_0_cs.val_mat4x4_;
    _group_0_binding_3_cs.val_mat4x4_ = mat4x4(mat4x4(_e284));
    float16_t _e287 = val;
    return _e287;
}

void main() {
    float16_t _e3 = f16_function(0);
    _group_0_binding_3_cs.final_value = _e3;
    return;
}

