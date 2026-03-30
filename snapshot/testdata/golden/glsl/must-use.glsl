#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


int use_me() {
    return 10;
}

int use_return() {
    int _e0 = use_me();
    return _e0;
}

int use_assign_var() {
    int q = 0;
    int _e0 = use_me();
    q = _e0;
    int _e2 = q;
    return _e2;
}

int use_assign_let() {
    int _e0 = use_me();
    return _e0;
}

void use_phony_assign() {
    int _e0 = use_me();
    return;
}

void main() {
    int _e0 = use_return();
    int _e1 = use_assign_var();
    int _e2 = use_assign_let();
    use_phony_assign();
    return;
}

