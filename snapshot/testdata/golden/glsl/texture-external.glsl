// === Entry Point: fragment_main (fragment) ===
#version 330 core
struct NagaExternalTextureTransferFn {
    float a;
    float b;
    float g;
    float k;
};
struct NagaExternalTextureParams {
    mat4x4 yuv_conversion_matrix;
    mat3x3 gamut_conversion_matrix;
    NagaExternalTextureTransferFn src_tf;
    NagaExternalTextureTransferFn dst_tf;
    mat3x2 sample_transform;
    mat3x2 load_transform;
    uvec2 size;
    uint num_planes;
};
uniform sampler2D _group_0_binding_0_fs;

layout(location = 0) out vec4 _fs2p_location0;

vec4 test(sampler2D t) {
    vec4 a = vec4(0.0);
    vec4 b = vec4(0.0);
    vec4 c = vec4(0.0);
    uvec2 d = uvec2(0u);
    vec4 _e4 = textureLod(t, vec2(vec2(0.0)), 0.0);
    a = _e4;
    vec4 _e8 = texelFetch(t, ivec2(0));
    b = _e8;
    vec4 _e12 = texelFetch(t, ivec2(uvec2(0u)));
    c = _e12;
    d = uvec2(textureSize(t, 0).xy);
    vec4 _e16 = a;
    vec4 _e17 = b;
    vec4 _e19 = c;
    uvec2 _e21 = d;
    return (((_e16 + _e17) + _e19) + vec2(_e21).xyxy);
}

void main() {
    vec4 _e1 = test(_group_0_binding_0_fs);
    _fs2p_location0 = _e1;
    return;
}


// === Entry Point: vertex_main (vertex) ===
#version 330 core
struct NagaExternalTextureTransferFn {
    float a;
    float b;
    float g;
    float k;
};
struct NagaExternalTextureParams {
    mat4x4 yuv_conversion_matrix;
    mat3x3 gamut_conversion_matrix;
    NagaExternalTextureTransferFn src_tf;
    NagaExternalTextureTransferFn dst_tf;
    mat3x2 sample_transform;
    mat3x2 load_transform;
    uvec2 size;
    uint num_planes;
};
uniform sampler2D _group_0_binding_0_vs;


vec4 test(sampler2D t) {
    vec4 a = vec4(0.0);
    vec4 b = vec4(0.0);
    vec4 c = vec4(0.0);
    uvec2 d = uvec2(0u);
    vec4 _e4 = textureLod(t, vec2(vec2(0.0)), 0.0);
    a = _e4;
    vec4 _e8 = texelFetch(t, ivec2(0));
    b = _e8;
    vec4 _e12 = texelFetch(t, ivec2(uvec2(0u)));
    c = _e12;
    d = uvec2(textureSize(t, 0).xy);
    vec4 _e16 = a;
    vec4 _e17 = b;
    vec4 _e19 = c;
    uvec2 _e21 = d;
    return (((_e16 + _e17) + _e19) + vec2(_e21).xyxy);
}

void main() {
    vec4 _e1 = test(_group_0_binding_0_vs);
    gl_Position = _e1;
    return;
}


// === Entry Point: compute_main (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct NagaExternalTextureTransferFn {
    float a;
    float b;
    float g;
    float k;
};
struct NagaExternalTextureParams {
    mat4x4 yuv_conversion_matrix;
    mat3x3 gamut_conversion_matrix;
    NagaExternalTextureTransferFn src_tf;
    NagaExternalTextureTransferFn dst_tf;
    mat3x2 sample_transform;
    mat3x2 load_transform;
    uvec2 size;
    uint num_planes;
};
uniform sampler2D _group_0_binding_0_cs;


vec4 test(sampler2D t) {
    vec4 a = vec4(0.0);
    vec4 b = vec4(0.0);
    vec4 c = vec4(0.0);
    uvec2 d = uvec2(0u);
    vec4 _e4 = textureLod(t, vec2(vec2(0.0)), 0.0);
    a = _e4;
    vec4 _e8 = texelFetch(t, ivec2(0));
    b = _e8;
    vec4 _e12 = texelFetch(t, ivec2(uvec2(0u)));
    c = _e12;
    d = uvec2(textureSize(t, 0).xy);
    vec4 _e16 = a;
    vec4 _e17 = b;
    vec4 _e19 = c;
    uvec2 _e21 = d;
    return (((_e16 + _e17) + _e19) + vec2(_e21).xyxy);
}

void main() {
    vec4 _e1 = test(_group_0_binding_0_cs);
    return;
}

