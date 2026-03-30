#version 330 core
layout(location = 0) out vec4 _fs2p_location0;

bool test_any_and_all_for_bool() {
    return true;
}

void main() {
    vec4 foo = gl_FragCoord;
    vec4 x_1 = vec4(0.0);
    vec4 y_1 = vec4(0.0);
    vec4 z_1 = vec4(0.0);
    vec4 _e1 = dFdx(foo);
    x_1 = _e1;
    vec4 _e3 = dFdy(foo);
    y_1 = _e3;
    vec4 _e5 = fwidth(foo);
    z_1 = _e5;
    vec4 _e7 = dFdx(foo);
    x_1 = _e7;
    vec4 _e8 = dFdy(foo);
    y_1 = _e8;
    vec4 _e9 = fwidth(foo);
    z_1 = _e9;
    vec4 _e10 = dFdx(foo);
    x_1 = _e10;
    vec4 _e11 = dFdy(foo);
    y_1 = _e11;
    vec4 _e12 = fwidth(foo);
    z_1 = _e12;
    bool _e13 = test_any_and_all_for_bool();
    vec4 _e14 = x_1;
    vec4 _e15 = y_1;
    vec4 _e17 = z_1;
    _fs2p_location0 = ((_e14 + _e15) * _e17);
    return;
}

