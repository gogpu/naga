// === Entry Point: main (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 16, local_size_y = 1, local_size_z = 1) in;

uniform usampler2D _group_0_binding_0_cs;

uniform usampler2DMS _group_0_binding_3_cs;

layout(rgba8ui) readonly uniform uimage2D _group_0_binding_1_cs;

uniform usampler2DArray _group_0_binding_5_cs;

layout(r32ui) readonly uniform uimage1D _group_0_binding_6_cs;

uniform usampler1D _group_0_binding_7_cs;

layout(r32ui) writeonly uniform uimage1D _group_0_binding_2_cs;


void main() {
    uvec3 local_id = gl_LocalInvocationID;
    uvec2 dim = uvec2(imageSize(_group_0_binding_1_cs).xy);
    ivec2 itc = (ivec2((dim * local_id.xy)) % ivec2(10, 20));
    uvec4 value1_ = texelFetch(_group_0_binding_0_cs, itc, int(local_id.z));
    uvec4 value1_2_ = texelFetch(_group_0_binding_0_cs, itc, int(uint(local_id.z)));
    uvec4 value2_ = texelFetch(_group_0_binding_3_cs, itc, int(local_id.z));
    uvec4 value3_ = texelFetch(_group_0_binding_3_cs, itc, int(uint(local_id.z)));
    uvec4 value4_ = imageLoad(_group_0_binding_1_cs, itc);
    uvec4 value5_ = texelFetch(_group_0_binding_5_cs, ivec3(itc, local_id.z), (int(local_id.z) + 1));
    uvec4 value6_ = texelFetch(_group_0_binding_5_cs, ivec3(itc, int(local_id.z)), (int(local_id.z) + 1));
    uvec4 value7_ = texelFetch(_group_0_binding_7_cs, int(local_id.x), int(local_id.z));
    uvec4 value8_ = imageLoad(_group_0_binding_6_cs, int(local_id.x));
    uvec4 value1u = texelFetch(_group_0_binding_0_cs, ivec2(uvec2(itc)), int(local_id.z));
    uvec4 value2u = texelFetch(_group_0_binding_3_cs, ivec2(uvec2(itc)), int(local_id.z));
    uvec4 value3u = texelFetch(_group_0_binding_3_cs, ivec2(uvec2(itc)), int(uint(local_id.z)));
    uvec4 value4u = imageLoad(_group_0_binding_1_cs, ivec2(uvec2(itc)));
    uvec4 value5u = texelFetch(_group_0_binding_5_cs, ivec3(uvec2(itc), local_id.z), (int(local_id.z) + 1));
    uvec4 value6u = texelFetch(_group_0_binding_5_cs, ivec3(uvec2(itc), int(local_id.z)), (int(local_id.z) + 1));
    uvec4 value7u = texelFetch(_group_0_binding_7_cs, int(uint(local_id.x)), int(local_id.z));
    imageStore(_group_0_binding_2_cs, itc.x, ((((value1_ + value2_) + value4_) + value5_) + value6_));
    imageStore(_group_0_binding_2_cs, int(uint(itc.x)), ((((value1u + value2u) + value4u) + value5u) + value6u));
    return;
}


// === Entry Point: depth_load (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 16, local_size_y = 1, local_size_z = 1) in;

uniform sampler2DMS _group_0_binding_4_cs;

layout(rgba8ui) readonly uniform uimage2D _group_0_binding_1_cs;

layout(r32ui) writeonly uniform uimage1D _group_0_binding_2_cs;


void main() {
    uvec3 local_id_1 = gl_LocalInvocationID;
    uvec2 dim = uvec2(imageSize(_group_0_binding_1_cs).xy);
    ivec2 itc = (ivec2((dim * local_id_1.xy)) % ivec2(10, 20));
    float val = texelFetch(_group_0_binding_4_cs, itc, int(local_id_1.z));
    imageStore(_group_0_binding_2_cs, itc.x, uvec4(uint(val)));
    return;
}


// === Entry Point: queries (vertex) ===
#version 330 core
#extension GL_ARB_texture_cube_map_array : require
uniform sampler1D _group_0_binding_0_vs;

uniform sampler2D _group_0_binding_1_vs;

uniform sampler2DArray _group_0_binding_4_vs;

uniform samplerCube _group_0_binding_5_vs;

uniform samplerCubeArray _group_0_binding_6_vs;

uniform sampler3D _group_0_binding_7_vs;

uniform sampler2DMS _group_0_binding_8_vs;


void main() {
    uint dim_1d = uint(textureSize(_group_0_binding_0_vs, 0));
    uint dim_1d_lod = uint(textureSize(_group_0_binding_0_vs, int(dim_1d)));
    uvec2 dim_2d = uvec2(textureSize(_group_0_binding_1_vs, 0).xy);
    uvec2 dim_2d_lod = uvec2(textureSize(_group_0_binding_1_vs, 1).xy);
    uvec2 dim_2d_array = uvec2(textureSize(_group_0_binding_4_vs, 0).xy);
    uvec2 dim_2d_array_lod = uvec2(textureSize(_group_0_binding_4_vs, 1).xy);
    uvec2 dim_cube = uvec2(textureSize(_group_0_binding_5_vs, 0).xy);
    uvec2 dim_cube_lod = uvec2(textureSize(_group_0_binding_5_vs, 1).xy);
    uvec2 dim_cube_array = uvec2(textureSize(_group_0_binding_6_vs, 0).xy);
    uvec2 dim_cube_array_lod = uvec2(textureSize(_group_0_binding_6_vs, 1).xy);
    uvec3 dim_3d = uvec3(textureSize(_group_0_binding_7_vs, 0).xyz);
    uvec3 dim_3d_lod = uvec3(textureSize(_group_0_binding_7_vs, 1).xyz);
    uvec2 dim_2s_ms = uvec2(textureSize(_group_0_binding_8_vs).xy);
    uint sum = ((((((((((dim_1d + dim_2d.y) + dim_2d_lod.y) + dim_2d_array.y) + dim_2d_array_lod.y) + dim_cube.y) + dim_cube_lod.y) + dim_cube_array.y) + dim_cube_array_lod.y) + dim_3d.z) + dim_3d_lod.z);
    gl_Position = vec4(float(sum));
    return;
}


// === Entry Point: levels_queries (vertex) ===
#version 330 core
#extension GL_ARB_texture_cube_map_array : require
#extension GL_ARB_shader_texture_image_samples : require
#extension GL_ARB_texture_query_levels : require
uniform sampler2D _group_0_binding_1_vs;

uniform sampler2DArray _group_0_binding_4_vs;

uniform samplerCube _group_0_binding_5_vs;

uniform samplerCubeArray _group_0_binding_6_vs;

uniform sampler3D _group_0_binding_7_vs;

uniform sampler2DMS _group_0_binding_8_vs;


void main() {
    uint num_levels_2d = uint(textureQueryLevels(_group_0_binding_1_vs));
    uint num_layers_2d = uint(textureSize(_group_0_binding_4_vs, 0).z);
    uint num_levels_2d_array = uint(textureQueryLevels(_group_0_binding_4_vs));
    uint num_layers_2d_array = uint(textureSize(_group_0_binding_4_vs, 0).z);
    uint num_levels_cube = uint(textureQueryLevels(_group_0_binding_5_vs));
    uint num_levels_cube_array = uint(textureQueryLevels(_group_0_binding_6_vs));
    uint num_layers_cube = uint(textureSize(_group_0_binding_6_vs, 0).z);
    uint num_levels_3d = uint(textureQueryLevels(_group_0_binding_7_vs));
    uint num_samples_aa = uint(textureSamples(_group_0_binding_8_vs));
    uint sum = (((((((num_layers_2d + num_layers_cube) + num_samples_aa) + num_levels_2d) + num_levels_2d_array) + num_levels_3d) + num_levels_cube) + num_levels_cube_array);
    gl_Position = vec4(float(sum));
    return;
}


// === Entry Point: texture_sample (fragment) ===
#version 330 core
#extension GL_ARB_texture_cube_map_array : require
uniform sampler1D _group_0_binding_0_fs;

uniform sampler2D _group_0_binding_1_fs;

uniform sampler2DArray _group_0_binding_4_fs;

uniform samplerCubeArray _group_0_binding_6_fs;

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec4 a = vec4(0.0);
    vec2 _e1 = vec2(0.5);
    vec3 _e3 = vec3(0.5);
    ivec2 _e6 = ivec2(3, 1);
    vec4 _e11 = texture(_group_0_binding_0_fs, 0.5);
    vec4 _e12 = a;
    a = (_e12 + _e11);
    vec4 _e16 = texture(_group_0_binding_1_fs, vec2(_e1));
    vec4 _e17 = a;
    a = (_e17 + _e16);
    vec4 _e24 = textureOffset(_group_0_binding_1_fs, vec2(_e1), ivec2(3, 1));
    vec4 _e25 = a;
    a = (_e25 + _e24);
    vec4 _e29 = textureLod(_group_0_binding_1_fs, vec2(_e1), 2.3);
    vec4 _e30 = a;
    a = (_e30 + _e29);
    vec4 _e34 = textureLodOffset(_group_0_binding_1_fs, vec2(_e1), 2.3, ivec2(3, 1));
    vec4 _e35 = a;
    a = (_e35 + _e34);
    vec4 _e40 = textureOffset(_group_0_binding_1_fs, vec2(_e1), ivec2(3, 1), 2.0);
    vec4 _e41 = a;
    a = (_e41 + _e40);
    vec4 _e45 = textureLod(_group_0_binding_1_fs, vec2(_e1), 0.0);
    vec4 _e46 = a;
    a = (_e46 + _e45);
    vec4 _e51 = texture(_group_0_binding_4_fs, vec3(_e1, 0u));
    vec4 _e52 = a;
    a = (_e52 + _e51);
    vec4 _e57 = textureOffset(_group_0_binding_4_fs, vec3(_e1, 0u), ivec2(3, 1));
    vec4 _e58 = a;
    a = (_e58 + _e57);
    vec4 _e63 = textureLod(_group_0_binding_4_fs, vec3(_e1, 0u), 2.3);
    vec4 _e64 = a;
    a = (_e64 + _e63);
    vec4 _e69 = textureLodOffset(_group_0_binding_4_fs, vec3(_e1, 0u), 2.3, ivec2(3, 1));
    vec4 _e70 = a;
    a = (_e70 + _e69);
    vec4 _e76 = textureOffset(_group_0_binding_4_fs, vec3(_e1, 0u), ivec2(3, 1), 2.0);
    vec4 _e77 = a;
    a = (_e77 + _e76);
    vec4 _e82 = texture(_group_0_binding_4_fs, vec3(_e1, 0));
    vec4 _e83 = a;
    a = (_e83 + _e82);
    vec4 _e88 = textureOffset(_group_0_binding_4_fs, vec3(_e1, 0), ivec2(3, 1));
    vec4 _e89 = a;
    a = (_e89 + _e88);
    vec4 _e94 = textureLod(_group_0_binding_4_fs, vec3(_e1, 0), 2.3);
    vec4 _e95 = a;
    a = (_e95 + _e94);
    vec4 _e100 = textureLodOffset(_group_0_binding_4_fs, vec3(_e1, 0), 2.3, ivec2(3, 1));
    vec4 _e101 = a;
    a = (_e101 + _e100);
    vec4 _e107 = textureOffset(_group_0_binding_4_fs, vec3(_e1, 0), ivec2(3, 1), 2.0);
    vec4 _e108 = a;
    a = (_e108 + _e107);
    vec4 _e113 = texture(_group_0_binding_6_fs, vec4(_e3, 0u));
    vec4 _e114 = a;
    a = (_e114 + _e113);
    vec4 _e119 = textureLod(_group_0_binding_6_fs, vec4(_e3, 0u), 2.3);
    vec4 _e120 = a;
    a = (_e120 + _e119);
    vec4 _e126 = texture(_group_0_binding_6_fs, vec4(_e3, 0u), 2.0);
    vec4 _e127 = a;
    a = (_e127 + _e126);
    vec4 _e132 = texture(_group_0_binding_6_fs, vec4(_e3, 0));
    vec4 _e133 = a;
    a = (_e133 + _e132);
    vec4 _e138 = textureLod(_group_0_binding_6_fs, vec4(_e3, 0), 2.3);
    vec4 _e139 = a;
    a = (_e139 + _e138);
    vec4 _e145 = texture(_group_0_binding_6_fs, vec4(_e3, 0), 2.0);
    vec4 _e146 = a;
    a = (_e146 + _e145);
    vec4 _e148 = a;
    _fs2p_location0 = _e148;
    return;
}


// === Entry Point: texture_sample_comparison (fragment) ===
#version 330 core
#extension GL_ARB_texture_cube_map_array : require
uniform sampler2DShadow _group_1_binding_2_fs;

uniform sampler2DArrayShadow _group_1_binding_3_fs;

uniform samplerCubeShadow _group_1_binding_4_fs;

layout(location = 0) out float _fs2p_location0;

void main() {
    float a_1 = 0.0;
    vec2 tc = vec2(0.5);
    vec3 tc3_ = vec3(0.5);
    float _e8 = texture(_group_1_binding_2_fs, vec3(tc, 0.5));
    float _e9 = a_1;
    a_1 = (_e9 + _e8);
    float _e14 = texture(_group_1_binding_3_fs, vec4(tc, 0u, 0.5));
    float _e15 = a_1;
    a_1 = (_e15 + _e14);
    float _e20 = texture(_group_1_binding_3_fs, vec4(tc, 0, 0.5));
    float _e21 = a_1;
    a_1 = (_e21 + _e20);
    float _e25 = texture(_group_1_binding_4_fs, vec4(tc3_, 0.5));
    float _e26 = a_1;
    a_1 = (_e26 + _e25);
    float _e30 = textureLod(_group_1_binding_2_fs, vec3(tc, 0.5), 0.0);
    float _e31 = a_1;
    a_1 = (_e31 + _e30);
    float _e36 = textureGrad(_group_1_binding_3_fs, vec4(tc, 0u, 0.5), vec2(0.0), vec2(0.0));
    float _e37 = a_1;
    a_1 = (_e37 + _e36);
    float _e42 = textureGrad(_group_1_binding_3_fs, vec4(tc, 0, 0.5), vec2(0.0), vec2(0.0));
    float _e43 = a_1;
    a_1 = (_e43 + _e42);
    float _e47 = textureGrad(_group_1_binding_4_fs, vec4(tc3_, 0.5), vec3(0.0), vec3(0.0));
    float _e48 = a_1;
    a_1 = (_e48 + _e47);
    float _e50 = a_1;
    _fs2p_location0 = _e50;
    return;
}


// === Entry Point: gather (fragment) ===
#version 330 core
#extension GL_ARB_texture_cube_map_array : require
uniform sampler2D _group_0_binding_1_fs;

uniform usampler2D _group_0_binding_2_fs;

uniform isampler2D _group_0_binding_3_fs;

uniform sampler2DShadow _group_1_binding_2_fs;

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 tc = vec2(0.5);
    vec4 s2d = textureGather(_group_0_binding_1_fs, vec2(tc), 1);
    vec4 s2d_offset = textureGatherOffset(_group_0_binding_1_fs, vec2(tc), ivec2(3, 1), 3);
    vec4 s2d_depth = textureGather(_group_1_binding_2_fs, vec2(tc), 0.5);
    vec4 s2d_depth_offset = textureGatherOffset(_group_1_binding_2_fs, vec2(tc), 0.5, ivec2(3, 1));
    uvec4 u = textureGather(_group_0_binding_2_fs, vec2(tc), 0);
    ivec4 i = textureGather(_group_0_binding_3_fs, vec2(tc), 0);
    vec4 f = (vec4(u) + vec4(i));
    _fs2p_location0 = ((((s2d + s2d_offset) + s2d_depth) + s2d_depth_offset) + f);
    return;
}


// === Entry Point: depth_no_comparison (fragment) ===
#version 330 core
#extension GL_ARB_texture_cube_map_array : require
uniform sampler2DShadow _group_1_binding_2_fs;

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 tc = vec2(0.5);
    float s2d = texture(_group_1_binding_2_fs, vec2(tc));
    vec4 s2d_gather = textureGather(_group_1_binding_2_fs, vec2(tc), 0);
    float s2d_level = textureLod(_group_1_binding_2_fs, vec2(tc), 1);
    _fs2p_location0 = ((vec4(s2d) + s2d_gather) + vec4(s2d_level));
    return;
}

