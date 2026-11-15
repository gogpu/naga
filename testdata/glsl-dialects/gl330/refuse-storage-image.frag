#version 330 core

// Image load/store is GLSL 4.20 / ES 3.10 (design §15 "Images"), so the image
// type is not part of the 3.30 core language.
layout(rgba8) uniform image2D target;

void main() {
    imageStore(target, ivec2(0), vec4(1.0));
}
