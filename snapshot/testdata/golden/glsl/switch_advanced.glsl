#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


int classify(int x) {
    int result = 0;
    switch(x) {
        case 0: {
            result = 0;
            break;
        }
        case 1:
        case 2: {
            result = 1;
            break;
        }
        case 3:
        case 4:
        case 5: {
            result = 2;
            break;
        }
        default: {
            result = -1;
            break;
        }
    }
    int _e6 = result;
    return _e6;
}

uint switch_with_return(uint x_1) {
    switch(x_1) {
        case 0u: {
            return 100u;
        }
        case 1u: {
            return 200u;
        }
        default: {
            return 0u;
        }
    }
}

int nested_switch(int a, int b) {
    int result_1 = 0;
    switch(a) {
        case 1: {
            switch(b) {
                case 10: {
                    result_1 = 110;
                    break;
                }
                default: {
                    result_1 = 100;
                    break;
                }
            }
            break;
        }
        default: {
            result_1 = 0;
            break;
        }
    }
    int _e7 = result_1;
    return _e7;
}

void main() {
    int _e1 = classify(3);
    uint _e3 = switch_with_return(1u);
    int _e6 = nested_switch(1, 10);
    return;
}

