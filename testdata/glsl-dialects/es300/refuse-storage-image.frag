#version 300 es

// ES 3.00 has no image load/store (design §15 "Images": ES 3.10).
precision highp float;

layout(rgba8) uniform image2D target;

void main() {
    imageStore(target, ivec2(0), vec4(1.0));
}
