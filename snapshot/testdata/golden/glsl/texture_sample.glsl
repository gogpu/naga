// === Entry Point: vs_main (vertex) ===
#version 330 core
struct VertexOutput {
    vec4 position;
    vec2 uv;
};
smooth out vec2 _vs2fs_location0;
int naga_div(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs / divisor;
}

int naga_mod(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}


void main() {
    uint vi = uint(gl_VertexID);
    float x = ((float(naga_div(int(vi), 2)) * 4.0) - 1.0);
    int _e9 = int(vi);
    float y = ((float(naga_mod(_e9, 2)) * 4.0) - 1.0);
    vec2 pos = vec2(x, y);
    vec2 uv_1 = ((pos + vec2(1.0)) * 0.5);
    VertexOutput _tmp_return = VertexOutput(vec4(pos, 0.0, 1.0), uv_1);
    gl_Position = _tmp_return.position;
    _vs2fs_location0 = _tmp_return.uv;
    gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
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
int naga_div(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs / divisor;
}

int naga_mod(int lhs, int rhs) {
    int divisor = (((lhs == -2147483647 - 1) && (rhs == -1)) || (rhs == 0)) ? 1 : rhs;
    return lhs - (lhs / divisor) * divisor;
}


void main() {
    vec2 uv = _vs2fs_location0;
    vec4 color = texture(_group_0_binding_0_fs, vec2(uv));
    _fs2p_location0 = color;
    return;
}

