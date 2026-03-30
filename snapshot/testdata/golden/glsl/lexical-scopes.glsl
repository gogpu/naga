#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


void blockLexicalScope(bool a) {
    {
        {
            return;
        }
    }
}

void ifLexicalScope(bool a_1) {
    if (a_1) {
        return;
    } else {
        return;
    }
}

void loopLexicalScope(bool a_2) {
    while(true) {
    }
    return;
}

void forLexicalScope(float a_3) {
    int a_4 = 0;
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            int _e8 = a_4;
            a_4 = (_e8 + 1);
        }
        loop_init = false;
        int _e3 = a_4;
        if ((_e3 < 1)) {
        } else {
            break;
        }
        {
        }
    }
    return;
}

void whileLexicalScope(int a_5) {
    while(true) {
        if ((a_5 > 2)) {
        } else {
            break;
        }
        {
        }
    }
    return;
}

void switchLexicalScope(int a_6) {
    switch(a_6) {
        case 0: {
            break;
        }
        case 1: {
            break;
        }
        default: {
            break;
        }
    }
    bool test = (a_6 == 2);
    return;
}

void main() {
    blockLexicalScope(false);
    ifLexicalScope(true);
    loopLexicalScope(false);
    forLexicalScope(1.0);
    whileLexicalScope(1);
    switchLexicalScope(1);
    return;
}

