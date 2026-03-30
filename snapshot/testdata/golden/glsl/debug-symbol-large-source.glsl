// === Entry Point: gen_terrain_compute (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

struct ChunkData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
};
struct Vertex {
    vec3 position;
    vec3 normal;
};
struct GenData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
    uint texture_size;
    uint start_index;
};
struct GenVertexOutput {
    uint index;
    vec4 position;
    vec2 uv;
};
struct GenFragmentOutput {
    uint vert_component;
    uint index;
};
struct Camera {
    vec4 view_pos;
    mat4x4 view_proj;
};
struct Light {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 normal;
    vec3 world_pos;
};
layout(std140) uniform ChunkData_block_0Compute { ChunkData _group_0_binding_0_cs; };

layout(std430) buffer VertexBuffer_block_1Compute {
    Vertex data[];
} _group_0_binding_1_cs;

layout(std430) buffer IndexBuffer_block_2Compute {
    uint data[];
} _group_0_binding_2_cs;


vec3 permute3_(vec3 x) {
    return ((((x * 34.0) + vec3(1.0)) * x) - vec3(289.0) * trunc((((x * 34.0) + vec3(1.0)) * x) / vec3(289.0)));
}

float snoise2_(vec2 v) {
    vec2 i = vec2(0.0);
    vec2 i1_ = vec2(0.0);
    vec4 x12_ = vec4(0.0);
    vec3 m = vec3(0.0);
    vec4 C = vec4(0.21132487, 0.36602542, -0.57735026, 0.024390243);
    i = floor((v + vec2(dot(v, C.yy))));
    vec2 _e12 = i;
    vec2 _e14 = i;
    vec2 x0_ = ((v - _e12) + vec2(dot(_e14, C.xx)));
    i1_ = ((x0_.x < x0_.y) ? vec2(0.0, 1.0) : vec2(1.0, 0.0));
    vec2 _e33 = i1_;
    x12_ = ((x0_.xyxy + C.xxzz) - vec4(_e33, 0.0, 0.0));
    vec2 _e39 = i;
    i = (_e39 - vec2(289.0) * trunc(_e39 / vec2(289.0)));
    float _e44 = i.y;
    float _e46 = i1_.y;
    vec3 _e52 = permute3_((vec3(_e44) + vec3(0.0, _e46, 1.0)));
    float _e54 = i.x;
    float _e58 = i1_.x;
    vec3 _e63 = permute3_(((_e52 + vec3(_e54)) + vec3(0.0, _e58, 1.0)));
    vec4 _e65 = x12_;
    vec4 _e67 = x12_;
    vec4 _e70 = x12_;
    vec4 _e72 = x12_;
    m = max((vec3(0.5) - vec3(dot(x0_, x0_), dot(_e65.xy, _e67.xy), dot(_e70.zw, _e72.zw))), vec3(0.0));
    vec3 _e83 = m;
    vec3 _e84 = m;
    m = (_e83 * _e84);
    vec3 _e86 = m;
    vec3 _e87 = m;
    m = (_e86 * _e87);
    vec3 x_2 = ((2.0 * fract((_e63 * C.www))) - vec3(1.0));
    vec3 h = (abs(x_2) - vec3(0.5));
    vec3 ox = floor((x_2 + vec3(0.5)));
    vec3 a0_ = (x_2 - ox);
    vec3 _e106 = m;
    m = (_e106 * (vec3(1.7928429) - (0.85373473 * ((a0_ * a0_) + (h * h)))));
    vec4 _e124 = x12_;
    vec4 _e128 = x12_;
    vec3 g = vec3(((a0_.x * x0_.x) + (h.x * x0_.y)), ((a0_.yz * _e124.xz) + (h.yz * _e128.yw)));
    vec3 _e133 = m;
    return (130.0 * dot(_e133, g));
}

float fbm(vec2 p) {
    vec2 x_1 = vec2(0.0);
    float v_1 = 0.0;
    float a = 0.5;
    uint i_1 = 0u;
    x_1 = (p * 0.01);
    vec2 shift = vec2(100.0);
    vec2 cs = vec2(0.87758255, 0.47942555);
    mat2x2 rot = mat2x2(vec2(cs.x, cs.y), vec2(-(cs.y), cs.x));
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e40 = i_1;
            i_1 = (_e40 + 1u);
        }
        loop_init = false;
        uint _e24 = i_1;
        if ((_e24 < 5u)) {
        } else {
            break;
        }
        {
            float _e26 = v_1;
            float _e27 = a;
            vec2 _e28 = x_1;
            float _e29 = snoise2_(_e28);
            v_1 = (_e26 + (_e27 * _e29));
            vec2 _e32 = x_1;
            x_1 = (((rot * _e32) * 2.0) + shift);
            float _e37 = a;
            a = (_e37 * 0.5);
        }
    }
    float _e43 = v_1;
    return _e43;
}

vec3 terrain_point(vec2 p_1, vec2 min_max_height) {
    float _e5 = fbm(p_1);
    return vec3(p_1.x, mix(min_max_height.x, min_max_height.y, _e5), p_1.y);
}

Vertex terrain_vertex(vec2 p_2, vec2 min_max_height_1) {
    vec3 _e2 = terrain_point(p_2, min_max_height_1);
    vec3 _e7 = terrain_point((p_2 + vec2(0.1, 0.0)), min_max_height_1);
    vec3 tpx = (_e7 - _e2);
    vec3 _e13 = terrain_point((p_2 + vec2(0.0, 0.1)), min_max_height_1);
    vec3 tpz = (_e13 - _e2);
    vec3 _e19 = terrain_point((p_2 + vec2(-0.1, 0.0)), min_max_height_1);
    vec3 tnx = (_e19 - _e2);
    vec3 _e25 = terrain_point((p_2 + vec2(0.0, -0.1)), min_max_height_1);
    vec3 tnz = (_e25 - _e2);
    vec3 pn = normalize(cross(tpz, tpx));
    vec3 nn = normalize(cross(tnz, tnx));
    vec3 n = ((pn + nn) * 0.5);
    return Vertex(_e2, n);
}

vec2 index_to_p(uint vert_index, uvec2 chunk_size, ivec2 chunk_corner) {
    return (vec2((float(vert_index) - float((chunk_size.x + 1u)) * trunc(float(vert_index) / float((chunk_size.x + 1u)))), float((vert_index / (chunk_size.x + 1u)))) + vec2(chunk_corner));
}

vec3 color23_(vec2 p_3) {
    float _e1 = snoise2_(p_3);
    float _e10 = snoise2_((p_3 + vec2(23.0, 32.0)));
    float _e19 = snoise2_((p_3 + vec2(-43.0, 3.0)));
    return vec3(((_e1 * 0.5) + 0.5), ((_e10 * 0.5) + 0.5), ((_e19 * 0.5) + 0.5));
}

void main() {
    uvec3 gid = gl_GlobalInvocationID;
    uint vert_index_1 = gid.x;
    uvec2 _e4 = _group_0_binding_0_cs.chunk_size;
    ivec2 _e7 = _group_0_binding_0_cs.chunk_corner;
    vec2 _e8 = index_to_p(vert_index_1, _e4, _e7);
    vec2 _e14 = _group_0_binding_0_cs.min_max_height;
    Vertex _e15 = terrain_vertex(_e8, _e14);
    _group_0_binding_1_cs.data[vert_index_1] = _e15;
    uint start_index = (gid.x * 6u);
    uint _e22 = _group_0_binding_0_cs.chunk_size.x;
    uint _e26 = _group_0_binding_0_cs.chunk_size.y;
    if ((start_index >= ((_e22 * _e26) * 6u))) {
        return;
    }
    uint _e35 = _group_0_binding_0_cs.chunk_size.x;
    uint v00_ = (vert_index_1 + (gid.x / _e35));
    uint v10_ = (v00_ + 1u);
    uint _e43 = _group_0_binding_0_cs.chunk_size.x;
    uint v01_ = ((v00_ + _e43) + 1u);
    uint v11_ = (v01_ + 1u);
    _group_0_binding_2_cs.data[start_index] = v00_;
    _group_0_binding_2_cs.data[(start_index + 1u)] = v01_;
    _group_0_binding_2_cs.data[(start_index + 2u)] = v11_;
    _group_0_binding_2_cs.data[(start_index + 3u)] = v00_;
    _group_0_binding_2_cs.data[(start_index + 4u)] = v11_;
    _group_0_binding_2_cs.data[(start_index + 5u)] = v10_;
    return;
}


// === Entry Point: gen_terrain_vertex (vertex) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct ChunkData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
};
struct Vertex {
    vec3 position;
    vec3 normal;
};
struct GenData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
    uint texture_size;
    uint start_index;
};
struct GenVertexOutput {
    uint index;
    vec4 position;
    vec2 uv;
};
struct GenFragmentOutput {
    uint vert_component;
    uint index;
};
struct Camera {
    vec4 view_pos;
    mat4x4 view_proj;
};
struct Light {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 normal;
    vec3 world_pos;
};
layout(std140) uniform GenData_block_0Vertex { GenData _group_0_binding_0_vs; };

flat out uint _vs2fs_location0;
smooth out vec2 _vs2fs_location1;

vec3 permute3_(vec3 x) {
    return ((((x * 34.0) + vec3(1.0)) * x) - vec3(289.0) * trunc((((x * 34.0) + vec3(1.0)) * x) / vec3(289.0)));
}

float snoise2_(vec2 v) {
    vec2 i = vec2(0.0);
    vec2 i1_ = vec2(0.0);
    vec4 x12_ = vec4(0.0);
    vec3 m = vec3(0.0);
    vec4 C = vec4(0.21132487, 0.36602542, -0.57735026, 0.024390243);
    i = floor((v + vec2(dot(v, C.yy))));
    vec2 _e12 = i;
    vec2 _e14 = i;
    vec2 x0_ = ((v - _e12) + vec2(dot(_e14, C.xx)));
    i1_ = ((x0_.x < x0_.y) ? vec2(0.0, 1.0) : vec2(1.0, 0.0));
    vec2 _e33 = i1_;
    x12_ = ((x0_.xyxy + C.xxzz) - vec4(_e33, 0.0, 0.0));
    vec2 _e39 = i;
    i = (_e39 - vec2(289.0) * trunc(_e39 / vec2(289.0)));
    float _e44 = i.y;
    float _e46 = i1_.y;
    vec3 _e52 = permute3_((vec3(_e44) + vec3(0.0, _e46, 1.0)));
    float _e54 = i.x;
    float _e58 = i1_.x;
    vec3 _e63 = permute3_(((_e52 + vec3(_e54)) + vec3(0.0, _e58, 1.0)));
    vec4 _e65 = x12_;
    vec4 _e67 = x12_;
    vec4 _e70 = x12_;
    vec4 _e72 = x12_;
    m = max((vec3(0.5) - vec3(dot(x0_, x0_), dot(_e65.xy, _e67.xy), dot(_e70.zw, _e72.zw))), vec3(0.0));
    vec3 _e83 = m;
    vec3 _e84 = m;
    m = (_e83 * _e84);
    vec3 _e86 = m;
    vec3 _e87 = m;
    m = (_e86 * _e87);
    vec3 x_2 = ((2.0 * fract((_e63 * C.www))) - vec3(1.0));
    vec3 h = (abs(x_2) - vec3(0.5));
    vec3 ox = floor((x_2 + vec3(0.5)));
    vec3 a0_ = (x_2 - ox);
    vec3 _e106 = m;
    m = (_e106 * (vec3(1.7928429) - (0.85373473 * ((a0_ * a0_) + (h * h)))));
    vec4 _e124 = x12_;
    vec4 _e128 = x12_;
    vec3 g = vec3(((a0_.x * x0_.x) + (h.x * x0_.y)), ((a0_.yz * _e124.xz) + (h.yz * _e128.yw)));
    vec3 _e133 = m;
    return (130.0 * dot(_e133, g));
}

float fbm(vec2 p) {
    vec2 x_1 = vec2(0.0);
    float v_1 = 0.0;
    float a = 0.5;
    uint i_1 = 0u;
    x_1 = (p * 0.01);
    vec2 shift = vec2(100.0);
    vec2 cs = vec2(0.87758255, 0.47942555);
    mat2x2 rot = mat2x2(vec2(cs.x, cs.y), vec2(-(cs.y), cs.x));
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e40 = i_1;
            i_1 = (_e40 + 1u);
        }
        loop_init = false;
        uint _e24 = i_1;
        if ((_e24 < 5u)) {
        } else {
            break;
        }
        {
            float _e26 = v_1;
            float _e27 = a;
            vec2 _e28 = x_1;
            float _e29 = snoise2_(_e28);
            v_1 = (_e26 + (_e27 * _e29));
            vec2 _e32 = x_1;
            x_1 = (((rot * _e32) * 2.0) + shift);
            float _e37 = a;
            a = (_e37 * 0.5);
        }
    }
    float _e43 = v_1;
    return _e43;
}

vec3 terrain_point(vec2 p_1, vec2 min_max_height) {
    float _e5 = fbm(p_1);
    return vec3(p_1.x, mix(min_max_height.x, min_max_height.y, _e5), p_1.y);
}

Vertex terrain_vertex(vec2 p_2, vec2 min_max_height_1) {
    vec3 _e2 = terrain_point(p_2, min_max_height_1);
    vec3 _e7 = terrain_point((p_2 + vec2(0.1, 0.0)), min_max_height_1);
    vec3 tpx = (_e7 - _e2);
    vec3 _e13 = terrain_point((p_2 + vec2(0.0, 0.1)), min_max_height_1);
    vec3 tpz = (_e13 - _e2);
    vec3 _e19 = terrain_point((p_2 + vec2(-0.1, 0.0)), min_max_height_1);
    vec3 tnx = (_e19 - _e2);
    vec3 _e25 = terrain_point((p_2 + vec2(0.0, -0.1)), min_max_height_1);
    vec3 tnz = (_e25 - _e2);
    vec3 pn = normalize(cross(tpz, tpx));
    vec3 nn = normalize(cross(tnz, tnx));
    vec3 n = ((pn + nn) * 0.5);
    return Vertex(_e2, n);
}

vec2 index_to_p(uint vert_index, uvec2 chunk_size, ivec2 chunk_corner) {
    return (vec2((float(vert_index) - float((chunk_size.x + 1u)) * trunc(float(vert_index) / float((chunk_size.x + 1u)))), float((vert_index / (chunk_size.x + 1u)))) + vec2(chunk_corner));
}

vec3 color23_(vec2 p_3) {
    float _e1 = snoise2_(p_3);
    float _e10 = snoise2_((p_3 + vec2(23.0, 32.0)));
    float _e19 = snoise2_((p_3 + vec2(-43.0, 3.0)));
    return vec3(((_e1 * 0.5) + 0.5), ((_e10 * 0.5) + 0.5), ((_e19 * 0.5) + 0.5));
}

void main() {
    uint vindex = uint(gl_VertexID);
    float u = float((((vindex + 2u) / 3u) % 2u));
    float v_2 = float((((vindex + 1u) / 3u) % 2u));
    vec2 uv = vec2(u, v_2);
    vec4 position = vec4((vec2(-1.0) + (uv * 2.0)), 0.0, 1.0);
    uint _e27 = _group_0_binding_0_vs.texture_size;
    uint _e33 = _group_0_binding_0_vs.texture_size;
    uint _e40 = _group_0_binding_0_vs.start_index;
    uint index_1 = (uint(((uv.x * float(_e27)) + (uv.y * float(_e33)))) + _e40);
    GenVertexOutput _tmp_return = GenVertexOutput(index_1, position, uv);
    _vs2fs_location0 = _tmp_return.index;
    gl_Position = _tmp_return.position;
    _vs2fs_location1 = _tmp_return.uv;
    return;
}


// === Entry Point: gen_terrain_fragment (fragment) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct ChunkData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
};
struct Vertex {
    vec3 position;
    vec3 normal;
};
struct GenData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
    uint texture_size;
    uint start_index;
};
struct GenVertexOutput {
    uint index;
    vec4 position;
    vec2 uv;
};
struct GenFragmentOutput {
    uint vert_component;
    uint index;
};
struct Camera {
    vec4 view_pos;
    mat4x4 view_proj;
};
struct Light {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 normal;
    vec3 world_pos;
};
layout(std140) uniform GenData_block_0Fragment { GenData _group_0_binding_0_fs; };

flat in uint _vs2fs_location0;
smooth in vec2 _vs2fs_location1;
layout(location = 0) out uint _fs2p_location0;
layout(location = 1) out uint _fs2p_location1;

vec3 permute3_(vec3 x) {
    return ((((x * 34.0) + vec3(1.0)) * x) - vec3(289.0) * trunc((((x * 34.0) + vec3(1.0)) * x) / vec3(289.0)));
}

float snoise2_(vec2 v) {
    vec2 i = vec2(0.0);
    vec2 i1_ = vec2(0.0);
    vec4 x12_ = vec4(0.0);
    vec3 m = vec3(0.0);
    vec4 C = vec4(0.21132487, 0.36602542, -0.57735026, 0.024390243);
    i = floor((v + vec2(dot(v, C.yy))));
    vec2 _e12 = i;
    vec2 _e14 = i;
    vec2 x0_ = ((v - _e12) + vec2(dot(_e14, C.xx)));
    i1_ = ((x0_.x < x0_.y) ? vec2(0.0, 1.0) : vec2(1.0, 0.0));
    vec2 _e33 = i1_;
    x12_ = ((x0_.xyxy + C.xxzz) - vec4(_e33, 0.0, 0.0));
    vec2 _e39 = i;
    i = (_e39 - vec2(289.0) * trunc(_e39 / vec2(289.0)));
    float _e44 = i.y;
    float _e46 = i1_.y;
    vec3 _e52 = permute3_((vec3(_e44) + vec3(0.0, _e46, 1.0)));
    float _e54 = i.x;
    float _e58 = i1_.x;
    vec3 _e63 = permute3_(((_e52 + vec3(_e54)) + vec3(0.0, _e58, 1.0)));
    vec4 _e65 = x12_;
    vec4 _e67 = x12_;
    vec4 _e70 = x12_;
    vec4 _e72 = x12_;
    m = max((vec3(0.5) - vec3(dot(x0_, x0_), dot(_e65.xy, _e67.xy), dot(_e70.zw, _e72.zw))), vec3(0.0));
    vec3 _e83 = m;
    vec3 _e84 = m;
    m = (_e83 * _e84);
    vec3 _e86 = m;
    vec3 _e87 = m;
    m = (_e86 * _e87);
    vec3 x_2 = ((2.0 * fract((_e63 * C.www))) - vec3(1.0));
    vec3 h = (abs(x_2) - vec3(0.5));
    vec3 ox = floor((x_2 + vec3(0.5)));
    vec3 a0_ = (x_2 - ox);
    vec3 _e106 = m;
    m = (_e106 * (vec3(1.7928429) - (0.85373473 * ((a0_ * a0_) + (h * h)))));
    vec4 _e124 = x12_;
    vec4 _e128 = x12_;
    vec3 g = vec3(((a0_.x * x0_.x) + (h.x * x0_.y)), ((a0_.yz * _e124.xz) + (h.yz * _e128.yw)));
    vec3 _e133 = m;
    return (130.0 * dot(_e133, g));
}

float fbm(vec2 p) {
    vec2 x_1 = vec2(0.0);
    float v_1 = 0.0;
    float a = 0.5;
    uint i_1 = 0u;
    x_1 = (p * 0.01);
    vec2 shift = vec2(100.0);
    vec2 cs = vec2(0.87758255, 0.47942555);
    mat2x2 rot = mat2x2(vec2(cs.x, cs.y), vec2(-(cs.y), cs.x));
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e40 = i_1;
            i_1 = (_e40 + 1u);
        }
        loop_init = false;
        uint _e24 = i_1;
        if ((_e24 < 5u)) {
        } else {
            break;
        }
        {
            float _e26 = v_1;
            float _e27 = a;
            vec2 _e28 = x_1;
            float _e29 = snoise2_(_e28);
            v_1 = (_e26 + (_e27 * _e29));
            vec2 _e32 = x_1;
            x_1 = (((rot * _e32) * 2.0) + shift);
            float _e37 = a;
            a = (_e37 * 0.5);
        }
    }
    float _e43 = v_1;
    return _e43;
}

vec3 terrain_point(vec2 p_1, vec2 min_max_height) {
    float _e5 = fbm(p_1);
    return vec3(p_1.x, mix(min_max_height.x, min_max_height.y, _e5), p_1.y);
}

Vertex terrain_vertex(vec2 p_2, vec2 min_max_height_1) {
    vec3 _e2 = terrain_point(p_2, min_max_height_1);
    vec3 _e7 = terrain_point((p_2 + vec2(0.1, 0.0)), min_max_height_1);
    vec3 tpx = (_e7 - _e2);
    vec3 _e13 = terrain_point((p_2 + vec2(0.0, 0.1)), min_max_height_1);
    vec3 tpz = (_e13 - _e2);
    vec3 _e19 = terrain_point((p_2 + vec2(-0.1, 0.0)), min_max_height_1);
    vec3 tnx = (_e19 - _e2);
    vec3 _e25 = terrain_point((p_2 + vec2(0.0, -0.1)), min_max_height_1);
    vec3 tnz = (_e25 - _e2);
    vec3 pn = normalize(cross(tpz, tpx));
    vec3 nn = normalize(cross(tnz, tnx));
    vec3 n = ((pn + nn) * 0.5);
    return Vertex(_e2, n);
}

vec2 index_to_p(uint vert_index, uvec2 chunk_size, ivec2 chunk_corner) {
    return (vec2((float(vert_index) - float((chunk_size.x + 1u)) * trunc(float(vert_index) / float((chunk_size.x + 1u)))), float((vert_index / (chunk_size.x + 1u)))) + vec2(chunk_corner));
}

vec3 color23_(vec2 p_3) {
    float _e1 = snoise2_(p_3);
    float _e10 = snoise2_((p_3 + vec2(23.0, 32.0)));
    float _e19 = snoise2_((p_3 + vec2(-43.0, 3.0)));
    return vec3(((_e1 * 0.5) + 0.5), ((_e10 * 0.5) + 0.5), ((_e19 * 0.5) + 0.5));
}

void main() {
    GenVertexOutput in_ = GenVertexOutput(_vs2fs_location0, gl_FragCoord, _vs2fs_location1);
    float vert_component = 0.0;
    uint index = 0u;
    uint _e5 = _group_0_binding_0_fs.texture_size;
    uint _e12 = _group_0_binding_0_fs.texture_size;
    uint _e15 = _group_0_binding_0_fs.texture_size;
    uint _e23 = _group_0_binding_0_fs.start_index;
    uint i_2 = (uint(((in_.uv.x * float(_e5)) + (in_.uv.y * float((_e12 * _e15))))) + _e23);
    uint vert_index_1 = uint(floor((float(i_2) / 6.0)));
    uint comp_index = (i_2 % 6u);
    uvec2 _e34 = _group_0_binding_0_fs.chunk_size;
    ivec2 _e37 = _group_0_binding_0_fs.chunk_corner;
    vec2 _e38 = index_to_p(vert_index_1, _e34, _e37);
    vec2 _e41 = _group_0_binding_0_fs.min_max_height;
    Vertex _e42 = terrain_vertex(_e38, _e41);
    switch(comp_index) {
        case 0u: {
            vert_component = _e42.position.x;
            break;
        }
        case 1u: {
            vert_component = _e42.position.y;
            break;
        }
        case 2u: {
            vert_component = _e42.position.z;
            break;
        }
        case 3u: {
            vert_component = _e42.normal.x;
            break;
        }
        case 4u: {
            vert_component = _e42.normal.y;
            break;
        }
        case 5u: {
            vert_component = _e42.normal.z;
            break;
        }
        default: {
            break;
        }
    }
    uint _e60 = _group_0_binding_0_fs.chunk_size.x;
    uint v00_ = (vert_index_1 + (vert_index_1 / _e60));
    uint v10_ = (v00_ + 1u);
    uint _e68 = _group_0_binding_0_fs.chunk_size.x;
    uint v01_ = ((v00_ + _e68) + 1u);
    uint v11_ = (v01_ + 1u);
    switch(comp_index) {
        case 0u:
        case 3u: {
            index = v00_;
            break;
        }
        case 2u:
        case 4u: {
            index = v11_;
            break;
        }
        case 1u: {
            index = v01_;
            break;
        }
        case 5u: {
            index = v10_;
            break;
        }
        default: {
            break;
        }
    }
    index = in_.index;
    float _e77 = vert_component;
    uint ivert_component = floatBitsToUint(_e77);
    uint _e79 = index;
    GenFragmentOutput _tmp_return = GenFragmentOutput(ivert_component, _e79);
    _fs2p_location0 = _tmp_return.vert_component;
    _fs2p_location1 = _tmp_return.index;
    return;
}


// === Entry Point: vs_main (vertex) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct ChunkData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
};
struct Vertex {
    vec3 position;
    vec3 normal;
};
struct GenData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
    uint texture_size;
    uint start_index;
};
struct GenVertexOutput {
    uint index;
    vec4 position;
    vec2 uv;
};
struct GenFragmentOutput {
    uint vert_component;
    uint index;
};
struct Camera {
    vec4 view_pos;
    mat4x4 view_proj;
};
struct Light {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 normal;
    vec3 world_pos;
};
layout(std140) uniform Camera_block_0Vertex { Camera _group_0_binding_0_vs; };

layout(location = 0) in vec3 _p2vs_location0;
layout(location = 1) in vec3 _p2vs_location1;
smooth out vec3 _vs2fs_location0;
smooth out vec3 _vs2fs_location1;

vec3 permute3_(vec3 x) {
    return ((((x * 34.0) + vec3(1.0)) * x) - vec3(289.0) * trunc((((x * 34.0) + vec3(1.0)) * x) / vec3(289.0)));
}

float snoise2_(vec2 v) {
    vec2 i = vec2(0.0);
    vec2 i1_ = vec2(0.0);
    vec4 x12_ = vec4(0.0);
    vec3 m = vec3(0.0);
    vec4 C = vec4(0.21132487, 0.36602542, -0.57735026, 0.024390243);
    i = floor((v + vec2(dot(v, C.yy))));
    vec2 _e12 = i;
    vec2 _e14 = i;
    vec2 x0_ = ((v - _e12) + vec2(dot(_e14, C.xx)));
    i1_ = ((x0_.x < x0_.y) ? vec2(0.0, 1.0) : vec2(1.0, 0.0));
    vec2 _e33 = i1_;
    x12_ = ((x0_.xyxy + C.xxzz) - vec4(_e33, 0.0, 0.0));
    vec2 _e39 = i;
    i = (_e39 - vec2(289.0) * trunc(_e39 / vec2(289.0)));
    float _e44 = i.y;
    float _e46 = i1_.y;
    vec3 _e52 = permute3_((vec3(_e44) + vec3(0.0, _e46, 1.0)));
    float _e54 = i.x;
    float _e58 = i1_.x;
    vec3 _e63 = permute3_(((_e52 + vec3(_e54)) + vec3(0.0, _e58, 1.0)));
    vec4 _e65 = x12_;
    vec4 _e67 = x12_;
    vec4 _e70 = x12_;
    vec4 _e72 = x12_;
    m = max((vec3(0.5) - vec3(dot(x0_, x0_), dot(_e65.xy, _e67.xy), dot(_e70.zw, _e72.zw))), vec3(0.0));
    vec3 _e83 = m;
    vec3 _e84 = m;
    m = (_e83 * _e84);
    vec3 _e86 = m;
    vec3 _e87 = m;
    m = (_e86 * _e87);
    vec3 x_2 = ((2.0 * fract((_e63 * C.www))) - vec3(1.0));
    vec3 h = (abs(x_2) - vec3(0.5));
    vec3 ox = floor((x_2 + vec3(0.5)));
    vec3 a0_ = (x_2 - ox);
    vec3 _e106 = m;
    m = (_e106 * (vec3(1.7928429) - (0.85373473 * ((a0_ * a0_) + (h * h)))));
    vec4 _e124 = x12_;
    vec4 _e128 = x12_;
    vec3 g = vec3(((a0_.x * x0_.x) + (h.x * x0_.y)), ((a0_.yz * _e124.xz) + (h.yz * _e128.yw)));
    vec3 _e133 = m;
    return (130.0 * dot(_e133, g));
}

float fbm(vec2 p) {
    vec2 x_1 = vec2(0.0);
    float v_1 = 0.0;
    float a = 0.5;
    uint i_1 = 0u;
    x_1 = (p * 0.01);
    vec2 shift = vec2(100.0);
    vec2 cs = vec2(0.87758255, 0.47942555);
    mat2x2 rot = mat2x2(vec2(cs.x, cs.y), vec2(-(cs.y), cs.x));
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e40 = i_1;
            i_1 = (_e40 + 1u);
        }
        loop_init = false;
        uint _e24 = i_1;
        if ((_e24 < 5u)) {
        } else {
            break;
        }
        {
            float _e26 = v_1;
            float _e27 = a;
            vec2 _e28 = x_1;
            float _e29 = snoise2_(_e28);
            v_1 = (_e26 + (_e27 * _e29));
            vec2 _e32 = x_1;
            x_1 = (((rot * _e32) * 2.0) + shift);
            float _e37 = a;
            a = (_e37 * 0.5);
        }
    }
    float _e43 = v_1;
    return _e43;
}

vec3 terrain_point(vec2 p_1, vec2 min_max_height) {
    float _e5 = fbm(p_1);
    return vec3(p_1.x, mix(min_max_height.x, min_max_height.y, _e5), p_1.y);
}

Vertex terrain_vertex(vec2 p_2, vec2 min_max_height_1) {
    vec3 _e2 = terrain_point(p_2, min_max_height_1);
    vec3 _e7 = terrain_point((p_2 + vec2(0.1, 0.0)), min_max_height_1);
    vec3 tpx = (_e7 - _e2);
    vec3 _e13 = terrain_point((p_2 + vec2(0.0, 0.1)), min_max_height_1);
    vec3 tpz = (_e13 - _e2);
    vec3 _e19 = terrain_point((p_2 + vec2(-0.1, 0.0)), min_max_height_1);
    vec3 tnx = (_e19 - _e2);
    vec3 _e25 = terrain_point((p_2 + vec2(0.0, -0.1)), min_max_height_1);
    vec3 tnz = (_e25 - _e2);
    vec3 pn = normalize(cross(tpz, tpx));
    vec3 nn = normalize(cross(tnz, tnx));
    vec3 n = ((pn + nn) * 0.5);
    return Vertex(_e2, n);
}

vec2 index_to_p(uint vert_index, uvec2 chunk_size, ivec2 chunk_corner) {
    return (vec2((float(vert_index) - float((chunk_size.x + 1u)) * trunc(float(vert_index) / float((chunk_size.x + 1u)))), float((vert_index / (chunk_size.x + 1u)))) + vec2(chunk_corner));
}

vec3 color23_(vec2 p_3) {
    float _e1 = snoise2_(p_3);
    float _e10 = snoise2_((p_3 + vec2(23.0, 32.0)));
    float _e19 = snoise2_((p_3 + vec2(-43.0, 3.0)));
    return vec3(((_e1 * 0.5) + 0.5), ((_e10 * 0.5) + 0.5), ((_e19 * 0.5) + 0.5));
}

void main() {
    Vertex vertex = Vertex(_p2vs_location0, _p2vs_location1);
    mat4x4 _e3 = _group_0_binding_0_vs.view_proj;
    vec4 clip_position = (_e3 * vec4(vertex.position, 1.0));
    vec3 normal = vertex.normal;
    VertexOutput _tmp_return = VertexOutput(clip_position, normal, vertex.position);
    gl_Position = _tmp_return.clip_position;
    _vs2fs_location0 = _tmp_return.normal;
    _vs2fs_location1 = _tmp_return.world_pos;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
#extension GL_ARB_shader_storage_buffer_object : require
struct ChunkData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
};
struct Vertex {
    vec3 position;
    vec3 normal;
};
struct GenData {
    uvec2 chunk_size;
    ivec2 chunk_corner;
    vec2 min_max_height;
    uint texture_size;
    uint start_index;
};
struct GenVertexOutput {
    uint index;
    vec4 position;
    vec2 uv;
};
struct GenFragmentOutput {
    uint vert_component;
    uint index;
};
struct Camera {
    vec4 view_pos;
    mat4x4 view_proj;
};
struct Light {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 normal;
    vec3 world_pos;
};
layout(std140) uniform Camera_block_0Fragment { Camera _group_0_binding_0_fs; };

layout(std140) uniform Light_block_1Fragment { Light _group_1_binding_0_fs; };

uniform sampler2D _group_2_binding_0_fs;

uniform sampler2D _group_2_binding_2_fs;

smooth in vec3 _vs2fs_location0;
smooth in vec3 _vs2fs_location1;
layout(location = 0) out vec4 _fs2p_location0;

vec3 permute3_(vec3 x) {
    return ((((x * 34.0) + vec3(1.0)) * x) - vec3(289.0) * trunc((((x * 34.0) + vec3(1.0)) * x) / vec3(289.0)));
}

float snoise2_(vec2 v) {
    vec2 i = vec2(0.0);
    vec2 i1_ = vec2(0.0);
    vec4 x12_ = vec4(0.0);
    vec3 m = vec3(0.0);
    vec4 C = vec4(0.21132487, 0.36602542, -0.57735026, 0.024390243);
    i = floor((v + vec2(dot(v, C.yy))));
    vec2 _e12 = i;
    vec2 _e14 = i;
    vec2 x0_ = ((v - _e12) + vec2(dot(_e14, C.xx)));
    i1_ = ((x0_.x < x0_.y) ? vec2(0.0, 1.0) : vec2(1.0, 0.0));
    vec2 _e33 = i1_;
    x12_ = ((x0_.xyxy + C.xxzz) - vec4(_e33, 0.0, 0.0));
    vec2 _e39 = i;
    i = (_e39 - vec2(289.0) * trunc(_e39 / vec2(289.0)));
    float _e44 = i.y;
    float _e46 = i1_.y;
    vec3 _e52 = permute3_((vec3(_e44) + vec3(0.0, _e46, 1.0)));
    float _e54 = i.x;
    float _e58 = i1_.x;
    vec3 _e63 = permute3_(((_e52 + vec3(_e54)) + vec3(0.0, _e58, 1.0)));
    vec4 _e65 = x12_;
    vec4 _e67 = x12_;
    vec4 _e70 = x12_;
    vec4 _e72 = x12_;
    m = max((vec3(0.5) - vec3(dot(x0_, x0_), dot(_e65.xy, _e67.xy), dot(_e70.zw, _e72.zw))), vec3(0.0));
    vec3 _e83 = m;
    vec3 _e84 = m;
    m = (_e83 * _e84);
    vec3 _e86 = m;
    vec3 _e87 = m;
    m = (_e86 * _e87);
    vec3 x_2 = ((2.0 * fract((_e63 * C.www))) - vec3(1.0));
    vec3 h = (abs(x_2) - vec3(0.5));
    vec3 ox = floor((x_2 + vec3(0.5)));
    vec3 a0_ = (x_2 - ox);
    vec3 _e106 = m;
    m = (_e106 * (vec3(1.7928429) - (0.85373473 * ((a0_ * a0_) + (h * h)))));
    vec4 _e124 = x12_;
    vec4 _e128 = x12_;
    vec3 g = vec3(((a0_.x * x0_.x) + (h.x * x0_.y)), ((a0_.yz * _e124.xz) + (h.yz * _e128.yw)));
    vec3 _e133 = m;
    return (130.0 * dot(_e133, g));
}

float fbm(vec2 p) {
    vec2 x_1 = vec2(0.0);
    float v_1 = 0.0;
    float a = 0.5;
    uint i_1 = 0u;
    x_1 = (p * 0.01);
    vec2 shift = vec2(100.0);
    vec2 cs = vec2(0.87758255, 0.47942555);
    mat2x2 rot = mat2x2(vec2(cs.x, cs.y), vec2(-(cs.y), cs.x));
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e40 = i_1;
            i_1 = (_e40 + 1u);
        }
        loop_init = false;
        uint _e24 = i_1;
        if ((_e24 < 5u)) {
        } else {
            break;
        }
        {
            float _e26 = v_1;
            float _e27 = a;
            vec2 _e28 = x_1;
            float _e29 = snoise2_(_e28);
            v_1 = (_e26 + (_e27 * _e29));
            vec2 _e32 = x_1;
            x_1 = (((rot * _e32) * 2.0) + shift);
            float _e37 = a;
            a = (_e37 * 0.5);
        }
    }
    float _e43 = v_1;
    return _e43;
}

vec3 terrain_point(vec2 p_1, vec2 min_max_height) {
    float _e5 = fbm(p_1);
    return vec3(p_1.x, mix(min_max_height.x, min_max_height.y, _e5), p_1.y);
}

Vertex terrain_vertex(vec2 p_2, vec2 min_max_height_1) {
    vec3 _e2 = terrain_point(p_2, min_max_height_1);
    vec3 _e7 = terrain_point((p_2 + vec2(0.1, 0.0)), min_max_height_1);
    vec3 tpx = (_e7 - _e2);
    vec3 _e13 = terrain_point((p_2 + vec2(0.0, 0.1)), min_max_height_1);
    vec3 tpz = (_e13 - _e2);
    vec3 _e19 = terrain_point((p_2 + vec2(-0.1, 0.0)), min_max_height_1);
    vec3 tnx = (_e19 - _e2);
    vec3 _e25 = terrain_point((p_2 + vec2(0.0, -0.1)), min_max_height_1);
    vec3 tnz = (_e25 - _e2);
    vec3 pn = normalize(cross(tpz, tpx));
    vec3 nn = normalize(cross(tnz, tnx));
    vec3 n = ((pn + nn) * 0.5);
    return Vertex(_e2, n);
}

vec2 index_to_p(uint vert_index, uvec2 chunk_size, ivec2 chunk_corner) {
    return (vec2((float(vert_index) - float((chunk_size.x + 1u)) * trunc(float(vert_index) / float((chunk_size.x + 1u)))), float((vert_index / (chunk_size.x + 1u)))) + vec2(chunk_corner));
}

vec3 color23_(vec2 p_3) {
    float _e1 = snoise2_(p_3);
    float _e10 = snoise2_((p_3 + vec2(23.0, 32.0)));
    float _e19 = snoise2_((p_3 + vec2(-43.0, 3.0)));
    return vec3(((_e1 * 0.5) + 0.5), ((_e10 * 0.5) + 0.5), ((_e19 * 0.5) + 0.5));
}

void main() {
    VertexOutput in_1 = VertexOutput(gl_FragCoord, _vs2fs_location0, _vs2fs_location1);
    vec3 color = vec3(0.0);
    vec3 _e8 = color23_(vec2(1.0, 2.0));
    color = smoothstep(vec3(0.0), vec3(0.1), fract(in_1.world_pos));
    float _e26 = color.x;
    float _e28 = color.y;
    float _e31 = color.z;
    color = mix(vec3(0.5, 0.1, 0.7), vec3(0.2, 0.2, 0.2), vec3(((_e26 * _e28) * _e31)));
    vec3 _e38 = _group_1_binding_0_fs.color;
    vec3 ambient_color = (_e38 * 0.1);
    vec3 _e42 = _group_1_binding_0_fs.position;
    vec3 light_dir = normalize((_e42 - in_1.world_pos));
    vec4 _e48 = _group_0_binding_0_fs.view_pos;
    vec3 view_dir = normalize((_e48.xyz - in_1.world_pos));
    vec3 half_dir = normalize((view_dir + light_dir));
    float diffuse_strength = max(dot(in_1.normal, light_dir), 0.0);
    vec3 _e61 = _group_1_binding_0_fs.color;
    vec3 diffuse_color = (diffuse_strength * _e61);
    float specular_strength = pow(max(dot(in_1.normal, half_dir), 0.0), 32.0);
    vec3 _e71 = _group_1_binding_0_fs.color;
    vec3 specular_color = (specular_strength * _e71);
    vec3 _e75 = color;
    vec3 result = (((ambient_color + diffuse_color) + specular_color) * _e75);
    _fs2p_location0 = vec4(result, 1.0);
    return;
}

