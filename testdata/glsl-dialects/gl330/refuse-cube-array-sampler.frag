#version 330 core

// Cube array textures arrive with GLSL 4.00 (ES 3.20); glslangValidator prints
// "'samplerCubeArray' : Reserved word" for this dialect (design §15).
uniform samplerCubeArray tex;

layout(location = 0) out vec4 color;

void main() {
    color = vec4(1.0);
}
