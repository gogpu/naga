#version 430 core

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
    return ((s.position[0] * s.scale) + s.inner[0]);
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    Outer mutable_outer = Outer(vec3(1.0, 2.0, 3.0), Inner(3.14, 1u), 2.0);
    mutable_outer.scale = 5.0;
    mutable_outer.inner[0] = 42.0;
    float _fc20 = process_struct(mutable_outer);
}
