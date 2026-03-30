#version 430 core

const int o = 0;

shared uint a;

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    uint _e2 = uint(o);
    int _ae4 = atomicCompSwap(a, _e2, 1u);
    uint _e4 = _ae4;
    return;
}
