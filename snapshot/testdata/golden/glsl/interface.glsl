// === Entry Point: vertex (vertex) ===
#version 330 core
#extension GL_ARB_compute_shader : require
uniform uint naga_vs_first_instance;

struct VertexOutput {
    vec4 position;
    float _varying;
};
struct FragmentOutput {
    float depth;
    uint sample_mask;
    float color;
};
struct Input1_ {
    uint index;
};
struct Input2_ {
    uint index;
};
layout(location = 10) in uint _p2vs_location10;
invariant gl_Position;
smooth out float _vs2fs_location1;

void main() {
    uint vertex_index = uint(gl_VertexID);
    uint instance_index = (uint(gl_InstanceID) + naga_vs_first_instance);
    uint color = _p2vs_location10;
    uint tmp = ((vertex_index + instance_index) + color);
    VertexOutput _tmp_return = VertexOutput(vec4(1.0), float(tmp));
    gl_Position = _tmp_return.position;
    _vs2fs_location1 = _tmp_return._varying;
    return;
}


// === Entry Point: fragment (fragment) ===
#version 330 core
#extension GL_ARB_compute_shader : require
struct VertexOutput {
    vec4 position;
    float _varying;
};
struct FragmentOutput {
    float depth;
    uint sample_mask;
    float color;
};
struct Input1_ {
    uint index;
};
struct Input2_ {
    uint index;
};
smooth in float _vs2fs_location1;
layout(location = 0) out float _fs2p_location0;

void main() {
    VertexOutput in_ = VertexOutput(gl_FragCoord, _vs2fs_location1);
    bool front_facing = gl_FrontFacing;
    uint sample_index = gl_SampleID;
    uint sample_mask = gl_SampleMaskIn[0];
    uint mask = (sample_mask & (1u << sample_index));
    float color_1 = (front_facing ? 1.0 : 0.0);
    FragmentOutput _tmp_return = FragmentOutput(in_._varying, mask, color_1);
    gl_FragDepth = _tmp_return.depth;
    gl_SampleMaskIn[0] = _tmp_return.sample_mask;
    _fs2p_location0 = _tmp_return.color;
    return;
}


// === Entry Point: compute (compute) ===
#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct VertexOutput {
    vec4 position;
    float _varying;
};
struct FragmentOutput {
    float depth;
    uint sample_mask;
    float color;
};
struct Input1_ {
    uint index;
};
struct Input2_ {
    uint index;
};
shared uint output_[1];


void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        output_ = uint[1](0u);
    }
    memoryBarrierShared();
    barrier();
    uvec3 global_id = gl_GlobalInvocationID;
    uvec3 local_id = gl_LocalInvocationID;
    uint local_index = gl_LocalInvocationIndex;
    uvec3 wg_id = gl_WorkGroupID;
    uvec3 num_wgs = gl_NumWorkGroups;
    output_[0] = ((((global_id.x + local_id.x) + local_index) + wg_id.x) + num_wgs.x);
    return;
}


// === Entry Point: vertex_two_structs (vertex) ===
#version 330 core
#extension GL_ARB_compute_shader : require
uniform uint naga_vs_first_instance;

struct VertexOutput {
    vec4 position;
    float _varying;
};
struct FragmentOutput {
    float depth;
    uint sample_mask;
    float color;
};
struct Input1_ {
    uint index;
};
struct Input2_ {
    uint index;
};
invariant gl_Position;

void main() {
    Input1_ in1_ = Input1_(uint(gl_VertexID));
    Input1_ in2_ = Input1_((uint(gl_InstanceID) + naga_vs_first_instance));
    uint index = 2u;
    uint _e8 = index;
    gl_Position = vec4(float(in1_.index), float(in2_.index), float(_e8), 0.0);
    return;
}

