// Vector swizzle: .xy, .xyz, .xyzw patterns.

@compute @workgroup_size(1)
fn main() {
    let v4 = vec4<f32>(1.0, 2.0, 3.0, 4.0);

    // Component access
    let x = v4.x;
    let y = v4.y;
    let z = v4.z;
    let w = v4.w;

    // Swizzle to smaller vectors
    let xy = v4.xy;
    let xyz = v4.xyz;
    let xyzw = v4.xyzw;
    let zw = v4.zw;

    // Duplicate components
    let xx = v4.xx;
    let yyy = v4.yyy;

    // Rearrange components
    let wzyx = v4.wzyx;
    let yx = v4.yx;
    let zxy = v4.zxy;

    // Swizzle on vec2 and vec3
    let v2 = vec2<f32>(1.0, 2.0);
    let v2_yx = v2.yx;
    let v2_xx = v2.xx;

    let v3 = vec3<f32>(1.0, 2.0, 3.0);
    let v3_zx = v3.zx;
    let v3_yxz = v3.yxz;
}
