#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;


uint test_packed_integer_dot_product() {
    int c_5_ = (bitfieldExtract(int(1u), 0, 8) * bitfieldExtract(int(2u), 0, 8) + bitfieldExtract(int(1u), 8, 8) * bitfieldExtract(int(2u), 8, 8) + bitfieldExtract(int(1u), 16, 8) * bitfieldExtract(int(2u), 16, 8) + bitfieldExtract(int(1u), 24, 8) * bitfieldExtract(int(2u), 24, 8));
    uint c_6_ = (bitfieldExtract((3u), 0, 8) * bitfieldExtract((4u), 0, 8) + bitfieldExtract((3u), 8, 8) * bitfieldExtract((4u), 8, 8) + bitfieldExtract((3u), 16, 8) * bitfieldExtract((4u), 16, 8) + bitfieldExtract((3u), 24, 8) * bitfieldExtract((4u), 24, 8));
    uint _e7 = (5u + c_6_);
    uint _e9 = (6u + c_6_);
    int c_7_ = (bitfieldExtract(int(_e7), 0, 8) * bitfieldExtract(int(_e9), 0, 8) + bitfieldExtract(int(_e7), 8, 8) * bitfieldExtract(int(_e9), 8, 8) + bitfieldExtract(int(_e7), 16, 8) * bitfieldExtract(int(_e9), 16, 8) + bitfieldExtract(int(_e7), 24, 8) * bitfieldExtract(int(_e9), 24, 8));
    uint _e12 = (7u + c_6_);
    uint _e14 = (8u + c_6_);
    uint c_8_ = (bitfieldExtract((_e12), 0, 8) * bitfieldExtract((_e14), 0, 8) + bitfieldExtract((_e12), 8, 8) * bitfieldExtract((_e14), 8, 8) + bitfieldExtract((_e12), 16, 8) * bitfieldExtract((_e14), 16, 8) + bitfieldExtract((_e12), 24, 8) * bitfieldExtract((_e14), 24, 8));
    return c_8_;
}

void main() {
    uint _e0 = test_packed_integer_dot_product();
    return;
}

