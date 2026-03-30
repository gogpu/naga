#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void break_from_loop() {
    int i = 0;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e6 = i;
            i = (_e6 + 1);
        }
        loop_init = false;
        int _e2 = i;
        if ((_e2 < 4)) {
        } else {
            break;
        }
        {
            break;
        }
    }
    return;
}

void main() {
    break_from_loop();
    return;
}

