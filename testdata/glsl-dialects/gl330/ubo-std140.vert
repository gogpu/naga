#version 330 core

// GL 3.30 core has no layout(binding=) (that is GLSL 4.20), so the block is
// bound by the caller's ParseOptions.ResourceBindings. std140 is the only
// layout 3.30 core allows, and the default.
uniform Globals {
    mat4 mvp;
    vec4 tint;
};

layout(location = 0) in vec3 position;

out vec4 vColor;

void main() {
    vColor = tint;
    gl_Position = mvp * vec4(position, 1.0);
}
