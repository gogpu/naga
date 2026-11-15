#version 300 es

// gl_FragColor is removed in ES 3.00 (design §15 "gl_FragColor": no).
precision highp float;

void main() {
    gl_FragColor = vec4(1.0);
}
