#version 330 core
uniform sampler2D _group_0_binding_0_fs;

uniform sampler2DShadow _group_0_binding_2_fs;

uniform sampler2DShadow _group_0_binding_2_fs__group_0_binding_3_fs;

layout(rgba8) uniform image2D _group_0_binding_4_fs;


void color() {
    vec4 phony = texture(_group_0_binding_0_fs, vec2(vec2(1.0, 2.0)));
    vec4 phony_1 = textureOffset(_group_0_binding_0_fs, vec2(vec2(1.0, 2.0)), ivec2(3, 4));
    vec4 phony_2 = textureLod(_group_0_binding_0_fs, vec2(vec2(1.0, 2.0)), 0.0);
    vec4 phony_3 = textureLod(_group_0_binding_0_fs, vec2(vec2(1.0, 2.0)), 0.0);
    vec4 phony_4 = textureGrad(_group_0_binding_0_fs, vec2(vec2(1.0, 2.0)), vec2(3.0, 4.0), vec2(5.0, 6.0));
    vec4 phony_5 = texture(_group_0_binding_0_fs, vec2(vec2(1.0, 2.0)), 1.0);
    return;
}

void depth() {
    float phony_6 = textureLod(_group_0_binding_2_fs, vec2(vec2(1.0, 2.0)), 1);
    float phony_7 = texture(_group_0_binding_2_fs__group_0_binding_3_fs, vec3(vec2(1.0, 2.0), 0.0));
    vec4 phony_8 = textureGather(_group_0_binding_2_fs__group_0_binding_3_fs, vec2(vec2(1.0, 2.0)), 0.0);
    return;
}

void storage() {
    imageStore(_group_0_binding_4_fs, ivec2(0, 1), vec4(2.0, 3.0, 4.0, 5.0));
    return;
}

void main() {
    color();
    depth();
    storage();
    return;
}

