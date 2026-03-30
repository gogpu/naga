#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

struct Inner {
    float value;
    uint flag;
};
struct Outer {
    vec3 position;
    Inner inner;
    float scale;
};

float process_struct(Outer s) {
    return ((s.position.x * s.scale) + s.inner.value);
}

void main() {
    Outer mutable_outer_1 = Outer(vec3(0.0), Inner(0.0, 0u), 0.0);
    Inner inner = Inner(3.14, 1u);
    Outer outer = Outer(vec3(1.0, 2.0, 3.0), inner, 2.0);
    vec3 pos = outer.position;
    float val = outer.inner.value;
    uint flag = outer.inner.flag;
    mutable_outer_1 = outer;
    mutable_outer_1.scale = 5.0;
    mutable_outer_1.inner.value = 42.0;
    Outer _e20 = mutable_outer_1;
    float _e21 = process_struct(_e20);
    return;
}

