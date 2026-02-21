#version 430 core

struct Data {
    float value;
    uint count;
};

void increment(int p) {
    p = (p + 1);
}

float read_value(float p) {
    return p;
}

void swap(int a, int b) {
    a = b;
    b = a;
}

void modify_array_element(float[4] arr, uint idx, float val) {
    arr[idx] = val;
}

void modify_struct_field(Data d) {
    d[0] = (d[0] * 2.0);
    d[1] = (d[1] + 1u);
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    int x = 10;
    float f = 3.14;
    int a = 1;
    int b = 2;
    float arr[4] = float[4](1.0, 2.0, 3.0, 4.0);
    Data data = Data(1.0, 0u);
    /* type */ _fc2 = increment(x);
    /* type */ _fc3 = increment(x);
    float _fc6 = read_value(f);
    /* type */ _fc11 = swap(a, b);
    /* type */ _fc20 = modify_array_element(arr, 0u, 99.0);
    /* type */ _fc25 = modify_struct_field(data);
}
