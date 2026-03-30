#version 330 core
struct QEFResult {
    float a;
    vec3 b;
};

QEFResult foobar(vec3 normals[12], uint count) {
    uint i = 0u;
    vec3 n0_ = vec3(0.0);
    uint j = 0u;
    vec3 n1_ = vec3(0.0);
    bool loop_init = true;
    while(true) {
        if (!loop_init) {
            uint _e10 = i;
            i = (_e10 + 1u);
        }
        loop_init = false;
        uint _e4 = i;
        if ((_e4 < count)) {
        } else {
            break;
        }
        {
            uint _e6 = i;
            n0_ = normals[_e6];
        }
    }
    bool loop_init_1 = true;
    while(true) {
        if (!loop_init_1) {
            uint _e20 = j;
            j = (_e20 + 1u);
        }
        loop_init_1 = false;
        uint _e14 = j;
        if ((_e14 < count)) {
        } else {
            break;
        }
        {
            uint _e16 = j;
            n1_ = normals[_e16];
        }
    }
    return QEFResult(0.0, vec3(0.0));
}

void main() {
    vec3 arr_1[12] = vec3[12](vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0), vec3(0.0));
    vec3 _e1[12] = arr_1;
    QEFResult _e3 = foobar(_e1, 1u);
    return;
}

