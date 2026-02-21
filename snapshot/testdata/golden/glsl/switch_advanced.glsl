#version 430 core

int classify(int x) {
    int result;
    switch (x) {
        case 0:
            result = 0;
            break;
        case 1:
            result = 1;
            break;
        case 2:
            result = 1;
            break;
        case 3:
            result = 2;
            break;
        case 4:
            result = 2;
            break;
        case 5:
            result = 2;
            break;
        default:
            result = -(1);
            break;
    }
    return result;
}

uint switch_with_return(uint x) {
    switch (x) {
        case 0u:
            return 100u;
            break;
        case 1u:
            return 200u;
            break;
        default:
            return 0u;
            break;
    }
}

int nested_switch(int a, int b) {
    int result_1 = 0;
    switch (a) {
        case 1:
            switch (b) {
                case 10:
                    result_1 = 110;
                    break;
                default:
                    result_1 = 100;
                    break;
            }
            break;
        default:
            result_1 = 0;
            break;
    }
    return result_1;
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    int _fc1 = classify(3);
    uint _fc3 = switch_with_return(1u);
    int _fc6 = nested_switch(1, 10);
}
