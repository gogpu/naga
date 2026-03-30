#version 330 core

int index_arg_array(int a[5], int i) {
    return a[i];
}

int index_let_array(int i_1, int j) {
    int a_1[2][2] = int[2][2](int[2](1, 2), int[2](3, 4));
    return a_1[i_1][j];
}

float index_let_matrix(int i_2, int j_1) {
    mat2x2 a_2 = mat2x2(vec2(1.0, 2.0), vec2(3.0, 4.0));
    return a_2[i_2][j_1];
}

vec4 index_let_array_1d(uint vi_1) {
    int arr[5] = int[5](1, 2, 3, 4, 5);
    int value = arr[vi_1];
    return vec4(ivec4(value));
}

void main() {
    uint vi = uint(gl_VertexID);
    int _e8 = index_arg_array(int[5](1, 2, 3, 4, 5), 6);
    int _e11 = index_let_array(1, 2);
    float _e14 = index_let_matrix(1, 2);
    vec4 _e15 = index_let_array_1d(vi);
    gl_Position = _e15;
    return;
}

