// language: metal1.0
#include <metal_stdlib>
#include <simd/simd.h>

using metal::uint;


void main_1(
) {
    int scalar_target = {};
    int scalar = 1;
    metal::uint2 vec_target = {};
    metal::uint2 vec_ = metal::uint2(1u);
    metal::float4x3 mat_target = {};
    metal::float4x3 mat = metal::float4x3(metal::float3(1.0, 0.0, 0.0), metal::float3(0.0, 1.0, 0.0), metal::float3(0.0, 0.0, 1.0), metal::float3(0.0, 0.0, 0.0));
    int _e3 = scalar;
    scalar = as_type<int>(as_type<uint>(_e3) + as_type<uint>(1));
    scalar_target = _e3;
    int _e6 = scalar;
    int _e8 = as_type<int>(as_type<uint>(_e6) - as_type<uint>(1));
    scalar = _e8;
    scalar_target = _e8;
    metal::uint2 _e13 = vec_;
    vec_ = _e13 - metal::uint2(1u);
    vec_target = _e13;
    metal::uint2 _e17 = vec_;
    metal::uint2 _e20 = _e17 + metal::uint2(1u);
    vec_ = _e20;
    vec_target = _e20;
    metal::float4x3 _e30 = mat;
    metal::float3 _e32 = metal::float3(1.0);
    mat = _e30 + metal::float4x3(_e32, _e32, _e32, _e32);
    mat_target = _e30;
    metal::float4x3 _e35 = mat;
    metal::float3 _e37 = metal::float3(1.0);
    metal::float4x3 _e39 = _e35 - metal::float4x3(_e37, _e37, _e37, _e37);
    mat = _e39;
    mat_target = _e39;
    return;
}

fragment void main_(
) {
    main_1();
    return;
}
