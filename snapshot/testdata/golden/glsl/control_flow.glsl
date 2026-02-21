#version 430 core

int test_if(int x) {
    if ((x > 0)) {
        return 1;
    } else {
        if ((x < 0)) {
            return -(1);
        } else {
            {
                return 0;
            }
        }
    }
}

int test_loop() {
    int sum = 0;
    int i = 0;
    for (;;) {
        if ((i >= 10)) {
            break;
        }
        sum = (sum + i);
        i = (i + 1);
    }
    return sum;
}

int test_for() {
    int sum_1 = 0;
    int i_2 = 0;
    for (;;) {
        if (!((i_2 < 5))) {
            break;
        }
        if ((i_2 == 3)) {
            continue;
        }
        sum_1 = (sum_1 + i_2);
        i_2 = (i_2 + 1);
    }
    return sum_1;
}

int test_while() {
    int n = 10;
    for (;;) {
        if (!((n > 0))) {
            break;
        }
        n = (n - 1);
    }
    return n;
}

int test_switch(int x) {
    int result;
    switch (x) {
        case 1:
            result = 10;
            break;
        case 2:
            result = 20;
            break;
        case 3:
            result = 20;
            break;
        default:
            result = 0;
            break;
    }
    return result;
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    int _fc1 = test_if(1);
    int _fc2 = test_loop();
    int _fc3 = test_for();
    int _fc4 = test_while();
    int _fc6 = test_switch(2);
}
