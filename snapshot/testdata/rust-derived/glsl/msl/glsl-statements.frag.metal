// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void switchEmpty(
    int a
) {
    int a_1 = {};
    a_1 = a;
    int _e2 = a_1;
    switch(_e2) {
        default: {
            break;
        }
    }
    return;
}

void switchNoDefault(
    int a_2
) {
    int a_3 = {};
    a_3 = a_2;
    int _e2 = a_3;
    switch(_e2) {
        case 0: {
            break;
        }
        default: {
            break;
        }
    }
    return;
}

void switchCaseImplConv(
    uint a_4
) {
    uint a_5 = {};
    a_5 = a_4;
    uint _e2 = a_5;
    switch(_e2) {
        case 0u: {
            break;
        }
        default: {
            break;
        }
    }
    return;
}

void switchNoLastBreak(
    int a_6
) {
    int a_7 = {};
    int b = {};
    a_7 = a_6;
    int _e2 = a_7;
    switch(_e2) {
        default: {
            int _e3 = a_7;
            b = _e3;
            break;
        }
    }
    return;
}

void main_1(
) {
    switchEmpty(1);
    switchNoDefault(2);
    switchCaseImplConv(3u);
    switchNoLastBreak(4);
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
