struct ChunkData {
    uint2 chunk_size;
    int2 chunk_corner;
    float2 min_max_height;
};

struct Vertex {
    float3 position : LOC0;
    float3 normal : LOC1;
};

struct VertexBuffer {
    Vertex data[];
};

struct IndexBuffer {
    uint data[];
};

struct GenData {
    uint2 chunk_size;
    int2 chunk_corner;
    float2 min_max_height;
    uint texture_size;
    uint start_index;
};

struct GenVertexOutput {
    nointerpolation uint index : LOC0;
    float4 position : SV_Position;
    float2 uv : LOC1;
};

struct GenFragmentOutput {
    nointerpolation uint vert_component : SV_Target0;
    nointerpolation uint index : SV_Target1;
};

struct Camera {
    float4 view_pos;
    row_major float4x4 view_proj;
};

struct Light {
    float3 position;
    int _pad1_0;
    float3 color;
    int _end_pad_0;
};

struct VertexOutput {
    float4 clip_position : SV_Position;
    float3 normal : LOC0;
    float3 world_pos : LOC1;
};

cbuffer chunk_data : register(b0) { ChunkData chunk_data; }
RWStructuredBuffer<VertexBuffer> vertices_ : register(u1);
RWStructuredBuffer<IndexBuffer> indices_ : register(u2);
cbuffer gen_data : register(b0) { GenData gen_data; }
cbuffer camera : register(b0) { Camera camera; }
cbuffer light : register(b0, space1) { Light light; }
Texture2D<float4> t_diffuse : register(t0, space2);
SamplerState s_diffuse : register(s1, space2);
Texture2D<float4> t_normal : register(t2, space2);
SamplerState s_normal : register(s3, space2);

struct VertexOutput_gen_terrain_vertex {
    nointerpolation uint index_1 : LOC0;
    float2 uv : LOC1;
    float4 position : SV_Position;
};

struct FragmentInput_gen_terrain_fragment {
    nointerpolation uint index_2 : LOC0;
    float2 uv_1 : LOC1;
    float4 position_1 : SV_Position;
};

struct VertexOutput_vs_main {
    float3 normal : LOC0;
    float3 world_pos : LOC1;
    float4 clip_position : SV_Position;
};

struct FragmentInput_fs_main {
    float3 normal_1 : LOC0;
    float3 world_pos_1 : LOC1;
    float4 clip_position_1 : SV_Position;
};

float3 permute3_(float3 x)
{
    return ((((x * 34.0) + (1.0).xxx) * x) % (289.0).xxx);
}

float snoise2_(float2 v)
{
    float2 i = (float2)0;
    float2 i1_ = (float2)0;
    float4 x12_ = (float4)0;
    float3 m = (float3)0;

    float4 C = float4(0.21132487, 0.36602542, -0.57735026, 0.024390243);
    i = floor((v + (dot(v, C.yy)).xx));
    float2 _e12 = i;
    float2 _e14 = i;
    float2 x0_ = ((v - _e12) + (dot(_e14, C.xx)).xx);
    i1_ = ((x0_.x < x0_.y) ? float2(0.0, 1.0) : float2(1.0, 0.0));
    float2 _e33 = i1_;
    x12_ = ((x0_.xyxy + C.xxzz) - float4(_e33, 0.0, 0.0));
    float2 _e39 = i;
    i = (_e39 % (289.0).xx);
    float _e44 = i.y;
    float _e46 = i1_.y;
    const float3 _e52 = permute3_(((_e44).xxx + float3(0.0, _e46, 1.0)));
    float _e54 = i.x;
    float _e58 = i1_.x;
    const float3 _e63 = permute3_(((_e52 + (_e54).xxx) + float3(0.0, _e58, 1.0)));
    float4 _e65 = x12_;
    float4 _e67 = x12_;
    float4 _e70 = x12_;
    float4 _e72 = x12_;
    m = max(((0.5).xxx - float3(dot(x0_, x0_), dot(_e65.xy, _e67.xy), dot(_e70.zw, _e72.zw))), (0.0).xxx);
    float3 _e83 = m;
    float3 _e84 = m;
    m = (_e83 * _e84);
    float3 _e86 = m;
    float3 _e87 = m;
    m = (_e86 * _e87);
    float3 x_2 = ((2.0 * frac((_e63 * C.www))) - (1.0).xxx);
    float3 h = (abs(x_2) - (0.5).xxx);
    float3 ox = floor((x_2 + (0.5).xxx));
    float3 a0_ = (x_2 - ox);
    float3 _e106 = m;
    m = (_e106 * ((1.7928429).xxx - (0.85373473 * ((a0_ * a0_) + (h * h)))));
    float4 _e124 = x12_;
    float4 _e128 = x12_;
    float3 g = float3(((a0_.x * x0_.x) + (h.x * x0_.y)), ((a0_.yz * _e124.xz) + (h.yz * _e128.yw)));
    float3 _e133 = m;
    return (130.0 * dot(_e133, g));
}

float fbm(float2 p)
{
    float2 x_1 = (float2)0;
    float v_1 = 0.0;
    float a = 0.5;
    uint i_1 = 0u;

    x_1 = (p * 0.01);
    float2 shift = (100.0).xx;
    float2 cs = float2(0.87758255, 0.47942555);
    float2x2 rot = float2x2(float2(cs.x, cs.y), float2(-(cs.y), cs.x));
    uint2 loop_bound = uint2(4294967295u, 4294967295u);
    bool loop_init = true;
    while(true) {
        if (all(loop_bound == uint2(0u, 0u))) { break; }
        loop_bound -= uint2(loop_bound.y == 0u, 1u);
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
            float2 _e28 = x_1;
            const float _e29 = snoise2_(_e28);
            v_1 = (_e26 + (_e27 * _e29));
            float2 _e32 = x_1;
            x_1 = ((mul(_e32, rot) * 2.0) + shift);
            float _e37 = a;
            a = (_e37 * 0.5);
        }
    }
    float _e43 = v_1;
    return _e43;
}

float3 terrain_point(float2 p_1, float2 min_max_height)
{
    const float _e5 = fbm(p_1);
    return float3(p_1.x, lerp(min_max_height.x, min_max_height.y, _e5), p_1.y);
}

Vertex ConstructVertex(float3 arg0, float3 arg1) {
    Vertex ret = (Vertex)0;
    ret.position = arg0;
    ret.normal = arg1;
    return ret;
}

Vertex terrain_vertex(float2 p_2, float2 min_max_height_1)
{
    const float3 _e2 = terrain_point(p_2, min_max_height_1);
    const float3 _e7 = terrain_point((p_2 + float2(0.1, 0.0)), min_max_height_1);
    float3 tpx = (_e7 - _e2);
    const float3 _e13 = terrain_point((p_2 + float2(0.0, 0.1)), min_max_height_1);
    float3 tpz = (_e13 - _e2);
    const float3 _e19 = terrain_point((p_2 + float2(-0.1, 0.0)), min_max_height_1);
    float3 tnx = (_e19 - _e2);
    const float3 _e25 = terrain_point((p_2 + float2(0.0, -0.1)), min_max_height_1);
    float3 tnz = (_e25 - _e2);
    float3 pn = normalize(cross(tpz, tpx));
    float3 nn = normalize(cross(tnz, tnx));
    float3 n = ((pn + nn) * 0.5);
    const Vertex vertex_1 = ConstructVertex(_e2, n);
    return vertex_1;
}

float2 index_to_p(uint vert_index, uint2 chunk_size, int2 chunk_corner)
{
    return (float2((float(vert_index) % float((chunk_size.x + 1u))), float((vert_index / (chunk_size.x + 1u)))) + float(chunk_corner));
}

float3 color23_(float2 p_3)
{
    const float _e1 = snoise2_(p_3);
    const float _e10 = snoise2_((p_3 + float2(23.0, 32.0)));
    const float _e19 = snoise2_((p_3 + float2(-43.0, 3.0)));
    return float3(((_e1 * 0.5) + 0.5), ((_e10 * 0.5) + 0.5), ((_e19 * 0.5) + 0.5));
}

[numthreads(64, 1, 1)]
void gen_terrain_compute(uint3 gid : SV_DispatchThreadID)
{
    uint vert_index_1 = gid.x;
    uint2 _e4 = chunk_data.chunk_size;
    int2 _e7 = chunk_data.chunk_corner;
    const float2 _e8 = index_to_p(vert_index_1, _e4, _e7);
    float2 _e14 = chunk_data.min_max_height;
    const Vertex _e15 = terrain_vertex(_e8, _e14);
    vertices_.data[vert_index_1] = _e15;
    uint start_index = (gid.x * 6u);
    uint _e22 = chunk_data.chunk_size.x;
    uint _e26 = chunk_data.chunk_size.y;
    if ((start_index >= ((_e22 * _e26) * 6u))) {
        return;
    }
    uint _e35 = chunk_data.chunk_size.x;
    uint v00_ = (vert_index_1 + (gid.x / _e35));
    uint v10_ = (v00_ + 1u);
    uint _e43 = chunk_data.chunk_size.x;
    uint v01_ = ((v00_ + _e43) + 1u);
    uint v11_ = (v01_ + 1u);
    indices_.data[start_index] = v00_;
    indices_.data[(start_index + 1u)] = v01_;
    indices_.data[(start_index + 2u)] = v11_;
    indices_.data[(start_index + 3u)] = v00_;
    indices_.data[(start_index + 4u)] = v11_;
    indices_.data[(start_index + 5u)] = v10_;
    return;
}

GenVertexOutput ConstructGenVertexOutput(uint arg0, float4 arg1, float2 arg2) {
    GenVertexOutput ret = (GenVertexOutput)0;
    ret.index = arg0;
    ret.position = arg1;
    ret.uv = arg2;
    return ret;
}

VertexOutput_gen_terrain_vertex gen_terrain_vertex(uint vindex : SV_VertexID)
{
    float u = float((((vindex + 2u) / 3u) % 2u));
    float v_2 = float((((vindex + 1u) / 3u) % 2u));
    float2 uv_2 = float2(u, v_2);
    float4 position_2 = float4(((-1.0).xx + (uv_2 * 2.0)), 0.0, 1.0);
    uint _e27 = gen_data.texture_size;
    uint _e33 = gen_data.texture_size;
    uint _e40 = gen_data.start_index;
    uint index_3 = (uint(((uv_2.x * float(_e27)) + (uv_2.y * float(_e33)))) + _e40);
    const GenVertexOutput genvertexoutput = ConstructGenVertexOutput(index_3, position_2, uv_2);
    const VertexOutput_gen_terrain_vertex genvertexoutput_1 = { genvertexoutput.index, genvertexoutput.uv, genvertexoutput.position };
    return genvertexoutput_1;
}

GenFragmentOutput ConstructGenFragmentOutput(uint arg0, uint arg1) {
    GenFragmentOutput ret = (GenFragmentOutput)0;
    ret.vert_component = arg0;
    ret.index = arg1;
    return ret;
}

GenFragmentOutput gen_terrain_fragment(FragmentInput_gen_terrain_fragment fragmentinput_gen_terrain_fragment)
{
    GenVertexOutput in_ = { fragmentinput_gen_terrain_fragment.index_2, fragmentinput_gen_terrain_fragment.position_1, fragmentinput_gen_terrain_fragment.uv_1 };
    float vert_component = 0.0;
    uint index = 0u;

    uint _e5 = gen_data.texture_size;
    uint _e12 = gen_data.texture_size;
    uint _e15 = gen_data.texture_size;
    uint _e23 = gen_data.start_index;
    uint i_2 = (uint(((in_.uv.x * float(_e5)) + (in_.uv.y * float((_e12 * _e15))))) + _e23);
    uint vert_index_2 = uint(floor((float(i_2) / 6.0)));
    uint comp_index = (i_2 % 6u);
    uint2 _e34 = gen_data.chunk_size;
    int2 _e37 = gen_data.chunk_corner;
    const float2 _e38 = index_to_p(vert_index_2, _e34, _e37);
    float2 _e41 = gen_data.min_max_height;
    const Vertex _e42 = terrain_vertex(_e38, _e41);
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
    uint _e60 = gen_data.chunk_size.x;
    uint v00_1 = (vert_index_2 + (vert_index_2 / _e60));
    uint v10_1 = (v00_1 + 1u);
    uint _e68 = gen_data.chunk_size.x;
    uint v01_1 = ((v00_1 + _e68) + 1u);
    uint v11_1 = (v01_1 + 1u);
    switch(comp_index) {
        case 0u:
        case 3u: {
            index = v00_1;
            break;
        }
        case 2u:
        case 4u: {
            index = v11_1;
            break;
        }
        case 1u: {
            index = v01_1;
            break;
        }
        case 5u: {
            index = v10_1;
            break;
        }
        default: {
            break;
        }
    }
    index = in_.index;
    float _e77 = vert_component;
    uint ivert_component = asuint(_e77);
    uint _e79 = index;
    const GenFragmentOutput genfragmentoutput = ConstructGenFragmentOutput(ivert_component, _e79);
    return genfragmentoutput;
}

VertexOutput ConstructVertexOutput(float4 arg0, float3 arg1, float3 arg2) {
    VertexOutput ret = (VertexOutput)0;
    ret.clip_position = arg0;
    ret.normal = arg1;
    ret.world_pos = arg2;
    return ret;
}

VertexOutput_vs_main vs_main(Vertex vertex)
{
    float4x4 _e3 = camera.view_proj;
    float4 clip_position_2 = mul(float4(vertex.position, 1.0), _e3);
    float3 normal_2 = vertex.normal;
    const VertexOutput vertexoutput = ConstructVertexOutput(clip_position_2, normal_2, vertex.position);
    const VertexOutput_vs_main vertexoutput_1 = { vertexoutput.normal, vertexoutput.world_pos, vertexoutput.clip_position };
    return vertexoutput_1;
}

float4 fs_main(FragmentInput_fs_main fragmentinput_fs_main) : SV_Target0
{
    VertexOutput in_1 = { fragmentinput_fs_main.clip_position_1, fragmentinput_fs_main.normal_1, fragmentinput_fs_main.world_pos_1 };
    float3 color = (float3)0;

    const float3 _e8 = color23_(float2(1.0, 2.0));
    color = smoothstep((0.0).xxx, (0.1).xxx, frac(in_1.world_pos));
    float _e26 = color.x;
    float _e28 = color.y;
    float _e31 = color.z;
    color = lerp(float3(0.5, 0.1, 0.7), float3(0.2, 0.2, 0.2), (((_e26 * _e28) * _e31)).xxx);
    float3 _e38 = light.color;
    float3 ambient_color = (_e38 * 0.1);
    float3 _e42 = light.position;
    float3 light_dir = normalize((_e42 - in_1.world_pos));
    float4 _e48 = camera.view_pos;
    float3 view_dir = normalize((_e48.xyz - in_1.world_pos));
    float3 half_dir = normalize((view_dir + light_dir));
    float diffuse_strength = max(dot(in_1.normal, light_dir), 0.0);
    float3 _e61 = light.color;
    float3 diffuse_color = (diffuse_strength * _e61);
    float specular_strength = pow(max(dot(in_1.normal, half_dir), 0.0), 32.0);
    float3 _e71 = light.color;
    float3 specular_color = (specular_strength * _e71);
    float3 _e75 = color;
    float3 result = (((ambient_color + diffuse_color) + specular_color) * _e75);
    return float4(result, 1.0);
}
