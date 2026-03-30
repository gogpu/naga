// === Entry Point: test_direct (fragment) ===
#version 330 core
struct F16IO {
    float16_t scalar_f16_;
    float scalar_f32_;
    vec2 vec2_f16_;
    vec2 vec2_f32_;
    vec3 vec3_f16_;
    vec3 vec3_f32_;
    vec4 vec4_f16_;
    vec4 vec4_f32_;
};
smooth in float16_t _vs2fs_location0;
smooth in float _vs2fs_location1;
smooth in vec2 _vs2fs_location2;
smooth in vec2 _vs2fs_location3;
smooth in vec3 _vs2fs_location4;
smooth in vec3 _vs2fs_location5;
smooth in vec4 _vs2fs_location6;
smooth in vec4 _vs2fs_location7;
layout(location = 0) out float16_t _fs2p_location0;
layout(location = 1) out float _fs2p_location1;
layout(location = 2) out vec2 _fs2p_location2;
layout(location = 3) out vec2 _fs2p_location3;
layout(location = 4) out vec3 _fs2p_location4;
layout(location = 5) out vec3 _fs2p_location5;
layout(location = 6) out vec4 _fs2p_location6;
layout(location = 7) out vec4 _fs2p_location7;

void main() {
    float16_t scalar_f16_ = _vs2fs_location0;
    float scalar_f32_ = _vs2fs_location1;
    vec2 vec2_f16_ = _vs2fs_location2;
    vec2 vec2_f32_ = _vs2fs_location3;
    vec3 vec3_f16_ = _vs2fs_location4;
    vec3 vec3_f32_ = _vs2fs_location5;
    vec4 vec4_f16_ = _vs2fs_location6;
    vec4 vec4_f32_ = _vs2fs_location7;
    F16IO output_ = F16IO(0.0, 0.0, vec2(0.0), vec2(0.0), vec3(0.0), vec3(0.0), vec4(0.0), vec4(0.0));
    output_.scalar_f16_ = (scalar_f16_ + 0);
    output_.scalar_f32_ = (scalar_f32_ + 1.0);
    output_.vec2_f16_ = (vec2_f16_ + vec2(0));
    output_.vec2_f32_ = (vec2_f32_ + vec2(1.0));
    output_.vec3_f16_ = (vec3_f16_ + vec3(0));
    output_.vec3_f32_ = (vec3_f32_ + vec3(1.0));
    output_.vec4_f16_ = (vec4_f16_ + vec4(0));
    output_.vec4_f32_ = (vec4_f32_ + vec4(1.0));
    F16IO _e39 = output_;
    _fs2p_location0 = _e39.scalar_f16_;
    _fs2p_location1 = _e39.scalar_f32_;
    _fs2p_location2 = _e39.vec2_f16_;
    _fs2p_location3 = _e39.vec2_f32_;
    _fs2p_location4 = _e39.vec3_f16_;
    _fs2p_location5 = _e39.vec3_f32_;
    _fs2p_location6 = _e39.vec4_f16_;
    _fs2p_location7 = _e39.vec4_f32_;
    return;
}


// === Entry Point: test_struct (fragment) ===
#version 330 core
struct F16IO {
    float16_t scalar_f16_;
    float scalar_f32_;
    vec2 vec2_f16_;
    vec2 vec2_f32_;
    vec3 vec3_f16_;
    vec3 vec3_f32_;
    vec4 vec4_f16_;
    vec4 vec4_f32_;
};
smooth in float16_t _vs2fs_location0;
smooth in float _vs2fs_location1;
smooth in vec2 _vs2fs_location2;
smooth in vec2 _vs2fs_location3;
smooth in vec3 _vs2fs_location4;
smooth in vec3 _vs2fs_location5;
smooth in vec4 _vs2fs_location6;
smooth in vec4 _vs2fs_location7;
layout(location = 0) out float16_t _fs2p_location0;
layout(location = 1) out float _fs2p_location1;
layout(location = 2) out vec2 _fs2p_location2;
layout(location = 3) out vec2 _fs2p_location3;
layout(location = 4) out vec3 _fs2p_location4;
layout(location = 5) out vec3 _fs2p_location5;
layout(location = 6) out vec4 _fs2p_location6;
layout(location = 7) out vec4 _fs2p_location7;

void main() {
    F16IO input_ = F16IO(_vs2fs_location0, _vs2fs_location1, _vs2fs_location2, _vs2fs_location3, _vs2fs_location4, _vs2fs_location5, _vs2fs_location6, _vs2fs_location7);
    F16IO output_1 = F16IO(0.0, 0.0, vec2(0.0), vec2(0.0), vec3(0.0), vec3(0.0), vec4(0.0), vec4(0.0));
    output_1.scalar_f16_ = (input_.scalar_f16_ + 0);
    output_1.scalar_f32_ = (input_.scalar_f32_ + 1.0);
    output_1.vec2_f16_ = (input_.vec2_f16_ + vec2(0));
    output_1.vec2_f32_ = (input_.vec2_f32_ + vec2(1.0));
    output_1.vec3_f16_ = (input_.vec3_f16_ + vec3(0));
    output_1.vec3_f32_ = (input_.vec3_f32_ + vec3(1.0));
    output_1.vec4_f16_ = (input_.vec4_f16_ + vec4(0));
    output_1.vec4_f32_ = (input_.vec4_f32_ + vec4(1.0));
    F16IO _e40 = output_1;
    _fs2p_location0 = _e40.scalar_f16_;
    _fs2p_location1 = _e40.scalar_f32_;
    _fs2p_location2 = _e40.vec2_f16_;
    _fs2p_location3 = _e40.vec2_f32_;
    _fs2p_location4 = _e40.vec3_f16_;
    _fs2p_location5 = _e40.vec3_f32_;
    _fs2p_location6 = _e40.vec4_f16_;
    _fs2p_location7 = _e40.vec4_f32_;
    return;
}


// === Entry Point: test_copy_input (fragment) ===
#version 330 core
struct F16IO {
    float16_t scalar_f16_;
    float scalar_f32_;
    vec2 vec2_f16_;
    vec2 vec2_f32_;
    vec3 vec3_f16_;
    vec3 vec3_f32_;
    vec4 vec4_f16_;
    vec4 vec4_f32_;
};
smooth in float16_t _vs2fs_location0;
smooth in float _vs2fs_location1;
smooth in vec2 _vs2fs_location2;
smooth in vec2 _vs2fs_location3;
smooth in vec3 _vs2fs_location4;
smooth in vec3 _vs2fs_location5;
smooth in vec4 _vs2fs_location6;
smooth in vec4 _vs2fs_location7;
layout(location = 0) out float16_t _fs2p_location0;
layout(location = 1) out float _fs2p_location1;
layout(location = 2) out vec2 _fs2p_location2;
layout(location = 3) out vec2 _fs2p_location3;
layout(location = 4) out vec3 _fs2p_location4;
layout(location = 5) out vec3 _fs2p_location5;
layout(location = 6) out vec4 _fs2p_location6;
layout(location = 7) out vec4 _fs2p_location7;

void main() {
    F16IO input_original = F16IO(_vs2fs_location0, _vs2fs_location1, _vs2fs_location2, _vs2fs_location3, _vs2fs_location4, _vs2fs_location5, _vs2fs_location6, _vs2fs_location7);
    F16IO input_1 = F16IO(0.0, 0.0, vec2(0.0), vec2(0.0), vec3(0.0), vec3(0.0), vec4(0.0), vec4(0.0));
    F16IO output_2 = F16IO(0.0, 0.0, vec2(0.0), vec2(0.0), vec3(0.0), vec3(0.0), vec4(0.0), vec4(0.0));
    input_1 = input_original;
    float16_t _e5 = input_1.scalar_f16_;
    output_2.scalar_f16_ = (_e5 + 0);
    float _e10 = input_1.scalar_f32_;
    output_2.scalar_f32_ = (_e10 + 1.0);
    vec2 _e15 = input_1.vec2_f16_;
    output_2.vec2_f16_ = (_e15 + vec2(0));
    vec2 _e21 = input_1.vec2_f32_;
    output_2.vec2_f32_ = (_e21 + vec2(1.0));
    vec3 _e27 = input_1.vec3_f16_;
    output_2.vec3_f16_ = (_e27 + vec3(0));
    vec3 _e33 = input_1.vec3_f32_;
    output_2.vec3_f32_ = (_e33 + vec3(1.0));
    vec4 _e39 = input_1.vec4_f16_;
    output_2.vec4_f16_ = (_e39 + vec4(0));
    vec4 _e45 = input_1.vec4_f32_;
    output_2.vec4_f32_ = (_e45 + vec4(1.0));
    F16IO _e49 = output_2;
    _fs2p_location0 = _e49.scalar_f16_;
    _fs2p_location1 = _e49.scalar_f32_;
    _fs2p_location2 = _e49.vec2_f16_;
    _fs2p_location3 = _e49.vec2_f32_;
    _fs2p_location4 = _e49.vec3_f16_;
    _fs2p_location5 = _e49.vec3_f32_;
    _fs2p_location6 = _e49.vec4_f16_;
    _fs2p_location7 = _e49.vec4_f32_;
    return;
}


// === Entry Point: test_return_partial (fragment) ===
#version 330 core
struct F16IO {
    float16_t scalar_f16_;
    float scalar_f32_;
    vec2 vec2_f16_;
    vec2 vec2_f32_;
    vec3 vec3_f16_;
    vec3 vec3_f32_;
    vec4 vec4_f16_;
    vec4 vec4_f32_;
};
smooth in float16_t _vs2fs_location0;
smooth in float _vs2fs_location1;
smooth in vec2 _vs2fs_location2;
smooth in vec2 _vs2fs_location3;
smooth in vec3 _vs2fs_location4;
smooth in vec3 _vs2fs_location5;
smooth in vec4 _vs2fs_location6;
smooth in vec4 _vs2fs_location7;
layout(location = 0) out float16_t _fs2p_location0;

void main() {
    F16IO input_original_1 = F16IO(_vs2fs_location0, _vs2fs_location1, _vs2fs_location2, _vs2fs_location3, _vs2fs_location4, _vs2fs_location5, _vs2fs_location6, _vs2fs_location7);
    F16IO input_2 = F16IO(0.0, 0.0, vec2(0.0), vec2(0.0), vec3(0.0), vec3(0.0), vec4(0.0), vec4(0.0));
    input_2 = input_original_1;
    input_2.scalar_f16_ = 0;
    float16_t _e5 = input_2.scalar_f16_;
    _fs2p_location0 = _e5;
    return;
}


// === Entry Point: test_component_access (fragment) ===
#version 330 core
struct F16IO {
    float16_t scalar_f16_;
    float scalar_f32_;
    vec2 vec2_f16_;
    vec2 vec2_f32_;
    vec3 vec3_f16_;
    vec3 vec3_f32_;
    vec4 vec4_f16_;
    vec4 vec4_f32_;
};
smooth in float16_t _vs2fs_location0;
smooth in float _vs2fs_location1;
smooth in vec2 _vs2fs_location2;
smooth in vec2 _vs2fs_location3;
smooth in vec3 _vs2fs_location4;
smooth in vec3 _vs2fs_location5;
smooth in vec4 _vs2fs_location6;
smooth in vec4 _vs2fs_location7;
layout(location = 0) out float16_t _fs2p_location0;
layout(location = 1) out float _fs2p_location1;
layout(location = 2) out vec2 _fs2p_location2;
layout(location = 3) out vec2 _fs2p_location3;
layout(location = 4) out vec3 _fs2p_location4;
layout(location = 5) out vec3 _fs2p_location5;
layout(location = 6) out vec4 _fs2p_location6;
layout(location = 7) out vec4 _fs2p_location7;

void main() {
    F16IO input_3 = F16IO(_vs2fs_location0, _vs2fs_location1, _vs2fs_location2, _vs2fs_location3, _vs2fs_location4, _vs2fs_location5, _vs2fs_location6, _vs2fs_location7);
    F16IO output_3 = F16IO(0.0, 0.0, vec2(0.0), vec2(0.0), vec3(0.0), vec3(0.0), vec4(0.0), vec4(0.0));
    output_3.vec2_f16_.x = input_3.vec2_f16_.y;
    output_3.vec2_f16_.y = input_3.vec2_f16_.x;
    F16IO _e10 = output_3;
    _fs2p_location0 = _e10.scalar_f16_;
    _fs2p_location1 = _e10.scalar_f32_;
    _fs2p_location2 = _e10.vec2_f16_;
    _fs2p_location3 = _e10.vec2_f32_;
    _fs2p_location4 = _e10.vec3_f16_;
    _fs2p_location5 = _e10.vec3_f32_;
    _fs2p_location6 = _e10.vec4_f16_;
    _fs2p_location7 = _e10.vec4_f32_;
    return;
}

