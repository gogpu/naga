// === Entry Point: vs_main (vertex) ===
#version 330 core
struct VertexInput {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 color;
};
layout(location = 0) in vec3 _p2vs_location0;
layout(location = 1) in vec3 _p2vs_location1;
smooth out vec3 _vs2fs_location0;

void main() {
    VertexInput model = VertexInput(_p2vs_location0, _p2vs_location1);
    VertexOutput out_ = VertexOutput(vec4(0.0), vec3(0.0));
    out_.color = model.color;
    out_.clip_position = vec4(model.position, 1.0);
    VertexOutput _e8 = out_;
    gl_Position = _e8.clip_position;
    _vs2fs_location0 = _e8.color;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct VertexInput {
    vec3 position;
    vec3 color;
};
struct VertexOutput {
    vec4 clip_position;
    vec3 color;
};
smooth in vec3 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    VertexOutput in_ = VertexOutput(gl_FragCoord, _vs2fs_location0);
    vec3 color = vec3(0.0);
    int i = 0;
    float ii = 0.0;
    color = in_.color;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e23 = i;
            i = (_e23 + 1);
        }
        loop_init = false;
        int _e5 = i;
        if ((_e5 < 10)) {
        } else {
            break;
        }
        {
            int _e8 = i;
            ii = float(_e8);
            float _e12 = color.x;
            float _e13 = ii;
            color.x = (_e12 + (_e13 * 0.001));
            float _e18 = color.y;
            float _e19 = ii;
            color.y = (_e18 + (_e19 * 0.002));
        }
    }
    vec3 _e26 = color;
    _fs2p_location0 = vec4(_e26, 1.0);
    return;
}

