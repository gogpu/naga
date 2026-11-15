#version 330 core

// Desktop GLSL accepts precision statements for compatibility with GLSL ES
// and ignores them (GLSL 3.30 §4.5.3).
precision mediump float;
precision mediump int;

layout(location = 0) out vec4 color;

void main() {
    color = vec4(1.0);
}
