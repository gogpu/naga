#version 300 es

// Per-type default precision statements (GLSL ES 3.00 §4.5.4): the fragment
// stage has no default for float or int.
precision highp float;
precision mediump int;

in vec2 vUV;

out vec4 color;

void main() {
    int channel = int(vUV.x);
    color = vec4(float(channel));
}
