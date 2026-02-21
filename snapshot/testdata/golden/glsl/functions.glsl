#version 430 core

float add(float a, float b) {
    return (a + b);
}

vec2 multiply(vec2 a, vec2 b) {
    return (a * b);
}

vec2 test_fma() {
    return fma(vec2(2.0, 2.0), vec2(0.5, 0.5), vec2(0.5, 0.5));
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    float _fc2 = add(1.0, 2.0);
    vec2 _fc9 = multiply(vec2(2.0, 3.0), vec2(4.0, 5.0));
    vec2 _fc10 = test_fma();
}
