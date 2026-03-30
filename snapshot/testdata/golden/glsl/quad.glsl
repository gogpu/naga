// === Entry Point: vert_main (vertex) ===
#version 330 core
struct VertexOutput {
    vec2 uv;
    vec4 position;
};
const float c_scale = 1.2;

layout(location = 0) in vec2 _p2vs_location0;
layout(location = 1) in vec2 _p2vs_location1;
smooth out vec2 _vs2fs_location0;

void main() {
    vec2 pos = _p2vs_location0;
    vec2 uv = _p2vs_location1;
    VertexOutput _tmp_return = VertexOutput(uv, vec4((c_scale * pos), 0.0, 1.0));
    _vs2fs_location0 = _tmp_return.uv;
    gl_Position = _tmp_return.position;
    return;
}


// === Entry Point: frag_main (fragment) ===
#version 330 core
struct VertexOutput {
    vec2 uv;
    vec4 position;
};
const float c_scale = 1.2;

uniform sampler2D _group_0_binding_0_fs;

smooth in vec2 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 uv_1 = _vs2fs_location0;
    vec4 color = texture(_group_0_binding_0_fs, vec2(uv_1));
    if ((color.w == 0.0)) {
        discard;
    }
    vec4 premultiplied = (color.w * color);
    _fs2p_location0 = premultiplied;
    return;
}


// === Entry Point: fs_extra (fragment) ===
#version 330 core
struct VertexOutput {
    vec2 uv;
    vec4 position;
};
const float c_scale = 1.2;

layout(location = 0) out vec4 _fs2p_location0;

void main() {
    _fs2p_location0 = vec4(0.0, 0.5, 0.0, 0.5);
    return;
}

