// === Entry Point: vs_main (vertex) ===
#version 330 core
struct VertexOutput {
    vec4 position;
    vec2 uv;
};
smooth out vec2 _vs2fs_location0;

void main() {
    uint vi = uint(gl_VertexID);
    float x = ((float((int(vi) / 2)) * 4.0) - 1.0);
    float y = ((float((int(vi) % 2)) * 4.0) - 1.0);
    vec2 pos = vec2(x, y);
    vec2 uv_1 = ((pos + vec2(1.0)) * 0.5);
    VertexOutput _tmp_return = VertexOutput(vec4(pos, 0.0, 1.0), uv_1);
    gl_Position = _tmp_return.position;
    _vs2fs_location0 = _tmp_return.uv;
    return;
}


// === Entry Point: fs_main (fragment) ===
#version 330 core
struct VertexOutput {
    vec4 position;
    vec2 uv;
};
uniform sampler2D _group_0_binding_0_fs;

smooth in vec2 _vs2fs_location0;
layout(location = 0) out vec4 _fs2p_location0;

void main() {
    vec2 uv = _vs2fs_location0;
    vec4 color = texture(_group_0_binding_0_fs, vec2(uv));
    _fs2p_location0 = color;
    return;
}

