#version 430 core

struct Foo {
    vec4 a;
    int b;
};

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    Foo foo;
    foo = Foo(vec4(1.0, 1.0, 1.0, 1.0), 1);
}
