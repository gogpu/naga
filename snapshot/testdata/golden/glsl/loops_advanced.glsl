#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


int nested_loops() {
    int total = 0;
    int i = 0;
    int j = 0;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e22 = i;
            i = (_e22 + 1);
        }
        loop_init = false;
        int _e4 = i;
        if ((_e4 < 3)) {
        } else {
            break;
        }
        {
            j = 0;
            bool loop_init_1 = true;
            while(true) {
                if (!loop_init_1) {
                    int _e19 = j;
                    j = (_e19 + 1);
                }
                loop_init_1 = false;
                int _e9 = j;
                if ((_e9 < 3)) {
                } else {
                    break;
                }
                {
                    int _e12 = total;
                    int _e13 = i;
                    int _e17 = j;
                    total = ((_e12 + (_e13 * 3)) + _e17);
                }
            }
        }
    }
    int _e25 = total;
    return _e25;
}

int loop_with_break() {
    int sum = 0;
    int i_1 = 0;
    while(true) {
        int _e4 = i_1;
        if ((_e4 >= 10)) {
            break;
        }
        int _e7 = i_1;
        if ((_e7 == 5)) {
            int _e10 = i_1;
            i_1 = (_e10 + 1);
            continue;
        }
        int _e13 = sum;
        int _e14 = i_1;
        sum = (_e13 + _e14);
        int _e16 = i_1;
        i_1 = (_e16 + 1);
    }
    int _e19 = sum;
    return _e19;
}

int loop_with_continuing() {
    int sum_1 = 0;
    int i_2 = 0;
    bool loop_init_2 = true;
    while(true) {
        if (!loop_init_2) {
            int _e10 = i_2;
            i_2 = (_e10 + 1);
        }
        loop_init_2 = false;
        int _e4 = i_2;
        if ((_e4 >= 5)) {
            break;
        }
        int _e7 = sum_1;
        int _e8 = i_2;
        sum_1 = (_e7 + _e8);
    }
    int _e13 = sum_1;
    return _e13;
}

int while_loop() {
    int n = 100;
    while(true) {
        int _e2 = n;
        if ((_e2 > 1)) {
        } else {
            break;
        }
        {
            int _e5 = n;
            n = (_e5 / 2);
        }
    }
    int _e8 = n;
    return _e8;
}

void main() {
    int _e0 = nested_loops();
    int _e1 = loop_with_break();
    int _e2 = loop_with_continuing();
    int _e3 = while_loop();
    return;
}

