#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


int test_if(int x) {
    if ((x > 0)) {
        return 1;
    } else {
        if ((x < 0)) {
            return -1;
        } else {
            return 0;
        }
    }
}

int test_loop() {
    int sum = 0;
    int i = 0;
    while(true) {
        int _e4 = i;
        if ((_e4 >= 10)) {
            break;
        }
        int _e7 = sum;
        int _e8 = i;
        sum = (_e7 + _e8);
        int _e10 = i;
        i = (_e10 + 1);
    }
    int _e13 = sum;
    return _e13;
}

int test_for() {
    int sum_1 = 0;
    int i_1 = 0;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e13 = i_1;
            i_1 = (_e13 + 1);
        }
        loop_init = false;
        int _e4 = i_1;
        if ((_e4 < 5)) {
        } else {
            break;
        }
        {
            int _e7 = i_1;
            if ((_e7 == 3)) {
                continue;
            }
            int _e10 = sum_1;
            int _e11 = i_1;
            sum_1 = (_e10 + _e11);
        }
    }
    int _e16 = sum_1;
    return _e16;
}

int test_while() {
    int n = 10;
    while(true) {
        int _e2 = n;
        if ((_e2 > 0)) {
        } else {
            break;
        }
        {
            int _e5 = n;
            n = (_e5 - 1);
        }
    }
    int _e8 = n;
    return _e8;
}

int test_switch(int x_1) {
    int result = 0;
    switch(x_1) {
        case 1: {
            result = 10;
            break;
        }
        case 2:
        case 3: {
            result = 20;
            break;
        }
        default: {
            result = 0;
            break;
        }
    }
    int _e5 = result;
    return _e5;
}

void main() {
    int _e1 = test_if(1);
    int _e2 = test_loop();
    int _e3 = test_for();
    int _e4 = test_while();
    int _e6 = test_switch(2);
    return;
}

