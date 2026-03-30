#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 64, local_size_y = 1, local_size_z = 1) in;

struct Particle {
    vec2 pos;
    vec2 vel;
};
struct SimParams {
    float deltaT;
    float rule1Distance;
    float rule2Distance;
    float rule3Distance;
    float rule1Scale;
    float rule2Scale;
    float rule3Scale;
};
const uint NUM_PARTICLES = 1500u;

layout(std140) uniform SimParams_block_0Compute { SimParams _group_0_binding_0_cs; };

layout(std430) readonly buffer Particles_block_1Compute {
    Particle particles[];
} _group_0_binding_1_cs;

layout(std430) buffer Particles_block_2Compute {
    Particle particles[];
} _group_0_binding_2_cs;


void main() {
    uvec3 global_invocation_id = gl_GlobalInvocationID;
    vec2 vPos_1 = vec2(0.0);
    vec2 vVel_1 = vec2(0.0);
    vec2 cMass_1 = vec2(0.0, 0.0);
    vec2 cVel_1 = vec2(0.0, 0.0);
    vec2 colVel_1 = vec2(0.0, 0.0);
    int cMassCount_1 = 0;
    int cVelCount_1 = 0;
    vec2 pos_1 = vec2(0.0);
    vec2 vel_1 = vec2(0.0);
    uint i_1 = 0u;
    uint index = global_invocation_id.x;
    if ((index >= NUM_PARTICLES)) {
        return;
    }
    vec2 _e8 = _group_0_binding_1_cs.particles[index].pos;
    vPos_1 = _e8;
    vec2 _e14 = _group_0_binding_1_cs.particles[index].vel;
    vVel_1 = _e14;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e91 = i_1;
            i_1 = (_e91 + 1u);
        }
        loop_init = false;
        uint _e36 = i_1;
        if ((_e36 >= NUM_PARTICLES)) {
            break;
        }
        uint _e39 = i_1;
        if ((_e39 == index)) {
            continue;
        }
        uint _e43 = i_1;
        vec2 _e46 = _group_0_binding_1_cs.particles[_e43].pos;
        pos_1 = _e46;
        uint _e49 = i_1;
        vec2 _e52 = _group_0_binding_1_cs.particles[_e49].vel;
        vel_1 = _e52;
        vec2 _e53 = pos_1;
        vec2 _e54 = vPos_1;
        float _e58 = _group_0_binding_0_cs.rule1Distance;
        if ((distance(_e53, _e54) < _e58)) {
            vec2 _e60 = cMass_1;
            vec2 _e61 = pos_1;
            cMass_1 = (_e60 + _e61);
            int _e63 = cMassCount_1;
            cMassCount_1 = (_e63 + 1);
        }
        vec2 _e66 = pos_1;
        vec2 _e67 = vPos_1;
        float _e71 = _group_0_binding_0_cs.rule2Distance;
        if ((distance(_e66, _e67) < _e71)) {
            vec2 _e73 = colVel_1;
            vec2 _e74 = pos_1;
            vec2 _e75 = vPos_1;
            colVel_1 = (_e73 - (_e74 - _e75));
        }
        vec2 _e78 = pos_1;
        vec2 _e79 = vPos_1;
        float _e83 = _group_0_binding_0_cs.rule3Distance;
        if ((distance(_e78, _e79) < _e83)) {
            vec2 _e85 = cVel_1;
            vec2 _e86 = vel_1;
            cVel_1 = (_e85 + _e86);
            int _e88 = cVelCount_1;
            cVelCount_1 = (_e88 + 1);
        }
    }
    int _e94 = cMassCount_1;
    if ((_e94 > 0)) {
        vec2 _e97 = cMass_1;
        int _e98 = cMassCount_1;
        vec2 _e102 = vPos_1;
        cMass_1 = ((_e97 / vec2(float(_e98))) - _e102);
    }
    int _e104 = cVelCount_1;
    if ((_e104 > 0)) {
        vec2 _e107 = cVel_1;
        int _e108 = cVelCount_1;
        cVel_1 = (_e107 / vec2(float(_e108)));
    }
    vec2 _e112 = vVel_1;
    vec2 _e113 = cMass_1;
    float _e116 = _group_0_binding_0_cs.rule1Scale;
    vec2 _e119 = colVel_1;
    float _e122 = _group_0_binding_0_cs.rule2Scale;
    vec2 _e125 = cVel_1;
    float _e128 = _group_0_binding_0_cs.rule3Scale;
    vVel_1 = (((_e112 + (_e113 * _e116)) + (_e119 * _e122)) + (_e125 * _e128));
    vec2 _e131 = vVel_1;
    vec2 _e133 = vVel_1;
    vVel_1 = (normalize(_e131) * clamp(length(_e133), 0.0, 0.1));
    vec2 _e139 = vPos_1;
    vec2 _e140 = vVel_1;
    float _e143 = _group_0_binding_0_cs.deltaT;
    vPos_1 = (_e139 + (_e140 * _e143));
    float _e147 = vPos_1.x;
    if ((_e147 < -1.0)) {
        vPos_1.x = 1.0;
    }
    float _e153 = vPos_1.x;
    if ((_e153 > 1.0)) {
        vPos_1.x = -1.0;
    }
    float _e159 = vPos_1.y;
    if ((_e159 < -1.0)) {
        vPos_1.y = 1.0;
    }
    float _e165 = vPos_1.y;
    if ((_e165 > 1.0)) {
        vPos_1.y = -1.0;
    }
    vec2 _e174 = vPos_1;
    _group_0_binding_2_cs.particles[index].pos = _e174;
    vec2 _e179 = vVel_1;
    _group_0_binding_2_cs.particles[index].vel = _e179;
    return;
}

