#version 430 core
#extension GL_ARB_compute_shader : require
layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

const bool has_point_light = false;
const float specular_param = 2.3;
const float gain = 1.1;
const float width = 0.0;
const float depth = 2.3;
const float height = 4.6;
const float inferred_f32_ = 2.718;
const uint auto_conversion = 0u;

float gain_x_10_ = 11.0;

float store_override = 0.0;


void main() {
    float t_1 = 23.0;
    bool x_1 = false;
    float gain_x_100_1 = 0.0;
    x_1 = true;
    float _e9 = gain_x_10_;
    gain_x_100_1 = (_e9 * 10.0);
    store_override = gain;
    return;
}

