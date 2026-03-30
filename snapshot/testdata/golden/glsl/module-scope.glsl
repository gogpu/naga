#version 330 core
struct S {
    int x;
};
const int Value = 1;

uniform sampler2D _group_0_binding_0_fs;


void statement() {
    return;
}

S returns() {
    return S(1);
}

void call() {
    statement();
    S _e0 = returns();
    vec4 s = texture(_group_0_binding_0_fs, vec2(vec2(1.0)));
    return;
}

void main() {
    call();
    statement();
    S _e0 = returns();
    return;
}

