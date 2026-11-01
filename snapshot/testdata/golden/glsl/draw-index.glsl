#version 330 core
struct Input {
    uint draw_index;
};

void main() {
    Input input_ = Input(gl_FragCoord);
    gl_Position = vec4(float(input_.draw_index), 1.0, 1.0, 1.0);
    gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
    return;
}

