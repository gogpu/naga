// === Entry Point: main_vec4vec3 (fragment) ===
#version 330 core
struct FragmentOutputVec4Vec3_ {
    vec4 vec4f;
    ivec4 vec4i;
    uvec4 vec4u;
    vec3 vec3f;
    ivec3 vec3i;
    uvec3 vec3u;
};
struct FragmentOutputVec2Scalar {
    vec2 vec2f;
    ivec2 vec2i;
    uvec2 vec2u;
    float scalarf;
    int scalari;
    uint scalaru;
};
layout(location = 0) out vec4 _fs2p_location0;
layout(location = 1) out ivec4 _fs2p_location1;
layout(location = 2) out uvec4 _fs2p_location2;
layout(location = 3) out vec3 _fs2p_location3;
layout(location = 4) out ivec3 _fs2p_location4;
layout(location = 5) out uvec3 _fs2p_location5;

void main() {
    FragmentOutputVec4Vec3_ output_ = FragmentOutputVec4Vec3_(vec4(0.0), ivec4(0), uvec4(0u), vec3(0.0), ivec3(0), uvec3(0u));
    output_.vec4f = vec4(0.0);
    output_.vec4i = ivec4(0);
    output_.vec4u = uvec4(0u);
    output_.vec3f = vec3(0.0);
    output_.vec3i = ivec3(0);
    output_.vec3u = uvec3(0u);
    FragmentOutputVec4Vec3_ _e19 = output_;
    _fs2p_location0 = _e19.vec4f;
    _fs2p_location1 = _e19.vec4i;
    _fs2p_location2 = _e19.vec4u;
    _fs2p_location3 = _e19.vec3f;
    _fs2p_location4 = _e19.vec3i;
    _fs2p_location5 = _e19.vec3u;
    return;
}


// === Entry Point: main_vec2scalar (fragment) ===
#version 330 core
struct FragmentOutputVec4Vec3_ {
    vec4 vec4f;
    ivec4 vec4i;
    uvec4 vec4u;
    vec3 vec3f;
    ivec3 vec3i;
    uvec3 vec3u;
};
struct FragmentOutputVec2Scalar {
    vec2 vec2f;
    ivec2 vec2i;
    uvec2 vec2u;
    float scalarf;
    int scalari;
    uint scalaru;
};
layout(location = 0) out vec2 _fs2p_location0;
layout(location = 1) out ivec2 _fs2p_location1;
layout(location = 2) out uvec2 _fs2p_location2;
layout(location = 3) out float _fs2p_location3;
layout(location = 4) out int _fs2p_location4;
layout(location = 5) out uint _fs2p_location5;

void main() {
    FragmentOutputVec2Scalar output_1 = FragmentOutputVec2Scalar(vec2(0.0), ivec2(0), uvec2(0u), 0.0, 0, 0u);
    output_1.vec2f = vec2(0.0);
    output_1.vec2i = ivec2(0);
    output_1.vec2u = uvec2(0u);
    output_1.scalarf = 0.0;
    output_1.scalari = 0;
    output_1.scalaru = 0u;
    FragmentOutputVec2Scalar _e16 = output_1;
    _fs2p_location0 = _e16.vec2f;
    _fs2p_location1 = _e16.vec2i;
    _fs2p_location2 = _e16.vec2u;
    _fs2p_location3 = _e16.scalarf;
    _fs2p_location4 = _e16.scalari;
    _fs2p_location5 = _e16.scalaru;
    return;
}

