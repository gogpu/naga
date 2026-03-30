// === Entry Point: no_padding_frag (fragment) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct NoPadding {
    vec3 v3_;
    float f3_;
};
struct NeedsPadding {
    float f3_forces_padding;
    vec3 v3_needs_padding;
    float f3_;
};
smooth in vec3 _vs2fs_location0;
smooth in float _vs2fs_location1;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    NoPadding input_ = NoPadding(_vs2fs_location0, _vs2fs_location1);
    _fs2p_location0 = vec4(0.0);
    return;
}


// === Entry Point: no_padding_vert (vertex) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct NoPadding {
    vec3 v3_;
    float f3_;
};
struct NeedsPadding {
    float f3_forces_padding;
    vec3 v3_needs_padding;
    float f3_;
};
layout(location = 0) in vec3 _p2vs_location0;
layout(location = 1) in float _p2vs_location1;

void main() {
    NoPadding input_1 = NoPadding(_p2vs_location0, _p2vs_location1);
    gl_Position = vec4(0.0);
    return;
}


// === Entry Point: no_padding_comp (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 16, local_size_y = 1, local_size_z = 1) in;

struct NoPadding {
    vec3 v3_;
    float f3_;
};
struct NeedsPadding {
    float f3_forces_padding;
    vec3 v3_needs_padding;
    float f3_;
};
layout(std140) uniform NoPadding_block_0Compute { NoPadding _group_0_binding_0_cs; };

layout(std430) buffer NoPadding_block_1Compute { NoPadding _group_0_binding_1_cs; };


void main() {
    NoPadding x = NoPadding(vec3(0.0), 0.0);
    NoPadding _e2 = _group_0_binding_0_cs;
    x = _e2;
    NoPadding _e4 = _group_0_binding_1_cs;
    x = _e4;
    return;
}


// === Entry Point: needs_padding_frag (fragment) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct NoPadding {
    vec3 v3_;
    float f3_;
};
struct NeedsPadding {
    float f3_forces_padding;
    vec3 v3_needs_padding;
    float f3_;
};
smooth in float _vs2fs_location0;
smooth in vec3 _vs2fs_location1;
smooth in float _vs2fs_location2;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    NeedsPadding input_2 = NeedsPadding(_vs2fs_location0, _vs2fs_location1, _vs2fs_location2);
    _fs2p_location0 = vec4(0.0);
    return;
}


// === Entry Point: needs_padding_vert (vertex) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct NoPadding {
    vec3 v3_;
    float f3_;
};
struct NeedsPadding {
    float f3_forces_padding;
    vec3 v3_needs_padding;
    float f3_;
};
layout(location = 0) in float _p2vs_location0;
layout(location = 1) in vec3 _p2vs_location1;
layout(location = 2) in float _p2vs_location2;

void main() {
    NeedsPadding input_3 = NeedsPadding(_p2vs_location0, _p2vs_location1, _p2vs_location2);
    gl_Position = vec4(0.0);
    return;
}


// === Entry Point: needs_padding_comp (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 16, local_size_y = 1, local_size_z = 1) in;

struct NoPadding {
    vec3 v3_;
    float f3_;
};
struct NeedsPadding {
    float f3_forces_padding;
    vec3 v3_needs_padding;
    float f3_;
};
layout(std140) uniform NeedsPadding_block_0Compute { NeedsPadding _group_0_binding_2_cs; };

layout(std430) buffer NeedsPadding_block_1Compute { NeedsPadding _group_0_binding_3_cs; };


void main() {
    NeedsPadding x_1 = NeedsPadding(0.0, vec3(0.0), 0.0);
    NeedsPadding _e2 = _group_0_binding_2_cs;
    x_1 = _e2;
    NeedsPadding _e4 = _group_0_binding_3_cs;
    x_1 = _e4;
    return;
}

