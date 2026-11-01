// === Entry Point: vs (vertex) ===
#version 330 core
invariant gl_Position;

void main() {
    gl_Position = vec4(0.0);
    gl_Position.yz = vec2(-gl_Position.y, gl_Position.z * 2.0 - gl_Position.w);
    return;
}


// === Entry Point: fs (fragment) ===
#version 330 core

void main() {
    vec4 position = gl_FragCoord;
    return;
}

