// === Entry Point: vs (vertex) ===
#version 330 core
struct OurVertexShaderOutput {
    vec4 position;
    vec2 texcoord;
};
layout(location = 0) in vec2 _p2vs_location0;
smooth out vec2 _vs2fs_location0;

void main() {
    vec2 xy = _p2vs_location0;
    OurVertexShaderOutput vsOutput = OurVertexShaderOutput(vec4(0.0), vec2(0.0));
    vsOutput.position = vec4(xy, 0.0, 1.0);
    OurVertexShaderOutput _e6 = vsOutput;
    gl_Position = _e6.position;
    _vs2fs_location0 = _e6.texcoord;
    return;
}


// === Entry Point: fs (fragment) ===
#version 330 core
struct OurVertexShaderOutput {
    vec4 position;
    vec2 texcoord;
};
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    _fs2p_location0 = vec4(1.0, 0.0, 0.0, 1.0);
    return;
}

