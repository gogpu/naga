#version 430 core

int nested_loops() {
    int total = 0;
    int i = 0;
    int j = 0;
    for (;;) {
        if (!((i < 3))) {
            break;
        }
        for (;;) {
            if (!((j < 3))) {
                break;
            }
            total = ((total + (i * 3)) + j);
            j = (j + 1);
        }
        i = (i + 1);
    }
    return total;
}

int loop_with_break() {
    int sum = 0;
    int i_1 = 0;
    for (;;) {
        if ((i_1 >= 10)) {
            break;
        }
        if ((i_1 == 5)) {
            i_1 = (i_1 + 1);
            continue;
        }
        sum = (sum + i_1);
        i_1 = (i_1 + 1);
    }
    return sum;
}

int loop_with_continuing() {
    int sum_2 = 0;
    int i_3 = 0;
    for (;;) {
        if ((i_3 >= 5)) {
            break;
        }
        sum_2 = (sum_2 + i_3);
        i_3 = (i_3 + 1);
    }
    return sum_2;
}

int while_loop() {
    int n = 100;
    for (;;) {
        if (!((n > 1))) {
            break;
        }
        n = (n / 2);
    }
    return n;
}

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    int _fc0 = nested_loops();
    int _fc1 = loop_with_break();
    int _fc2 = loop_with_continuing();
    int _fc3 = while_loop();
}
