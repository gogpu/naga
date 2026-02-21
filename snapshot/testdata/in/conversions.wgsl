// Type conversions: f32(u32), u32(f32), i32(f32), vec conversions, bitcast.

@compute @workgroup_size(1)
fn main() {
    // Scalar conversions
    let from_u = f32(42u);
    let from_i = f32(-7i);
    let to_u = u32(3.14);
    let to_i = i32(2.71);
    let i_to_u = u32(10i);
    let u_to_i = i32(10u);

    // Bool conversions
    let b_from_i = bool(1i);
    let b_from_u = bool(0u);
    let i_from_b = i32(true);
    let u_from_b = u32(false);
    let f_from_b = f32(true);

    // Vector conversions
    let fv = vec3<f32>(vec3<u32>(1u, 2u, 3u));
    let uv = vec3<u32>(vec3<f32>(1.5, 2.5, 3.5));
    let iv = vec3<i32>(vec3<f32>(1.0, 2.0, 3.0));

    // Bitcast
    let bits = bitcast<u32>(1.0f);
    let back = bitcast<f32>(bits);
    let vec_bits = bitcast<vec4<u32>>(vec4<f32>(1.0, 2.0, 3.0, 4.0));
}
