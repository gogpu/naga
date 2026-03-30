#version 430 core

const bool has_point_light = true;
const float specular_param = 2.3;
const float gain = 0.0;
const float width = 0.0;
const float height = 0.0;
const float inferred_f32 = 2.718;
const uint auto_conversion = 0u;

float gain_x_10;
float store_override;

layout(local_size_x = 1, local_size_y = 1, local_size_z = 1) in;

void main() {
    float t = (height * 5.0);
    bool x;
    float gain_x_100;
    float _e2 = (height * 5.0);
    bool a = !(has_point_light);
    x = a;
    float _e7 = gain_x_10;
    float _e9 = (_e7 * 10.0);
    gain_x_100 = _e9;
    store_override = gain;
    return;
}
