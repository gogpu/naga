struct A {
    half a_1_;
    half2 a_vec2_;
    half3 a_vec3_;
    half4 a_vec4_;
};

struct B {
    half b_1_;
    half2 b_vec2_;
    half3 b_vec3_;
    half4 b_vec4_;
    half2 b_mat2__0; half2 b_mat2__1;
    row_major half2x3 b_mat2x3_;
    row_major half2x4 b_mat2x4_;
    half2 b_mat3x2__0; half2 b_mat3x2__1; half2 b_mat3x2__2;
    int _pad8_0;
    row_major half3x3 b_mat3_;
    row_major half3x4 b_mat3x4_;
    half2 b_mat4x2__0; half2 b_mat4x2__1; half2 b_mat4x2__2; half2 b_mat4x2__3;
    row_major half4x3 b_mat4x3_;
    row_major half4x4 b_mat4_;
};

cbuffer global : register(b0) { A global; }
RWByteAddressBuffer global_1 : register(u1);

void main_1()
{
    half _e16 = global.a_1_;
    global_1.Store(0, _e16);
    half2 _e17 = global.a_vec2_;
    global_1.Store(4, _e17);
    half3 _e18 = global.a_vec3_;
    global_1.Store(8, _e18);
    half4 _e19 = global.a_vec4_;
    global_1.Store(16, _e19);
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    main_1();
    return;
}
