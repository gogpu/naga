#version 300 es

// Cube array textures arrive with ES 3.20; glslangValidator prints
// "'samplerCubeArray' : Reserved word" for this dialect (design §15).
precision highp float;

uniform samplerCubeArray tex;

out vec4 color;

void main() {
    color = vec4(1.0);
}
