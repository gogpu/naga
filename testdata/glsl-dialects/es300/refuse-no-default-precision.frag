#version 300 es

// The fragment stage's float type has no default precision in ES 3.00
// (design §15: "fragment float needs a default precision statement").
out vec4 color;

void main() {
    color = vec4(1.0);
}
