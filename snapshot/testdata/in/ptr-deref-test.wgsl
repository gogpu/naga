// Test patterns for pointer/reference/load behavior

struct Baz {
    m: mat3x2<f32>,
}

@group(0) @binding(1) var<uniform> baz: Baz;

fn pattern1_local_var_vector() {
    var vec0 = vec4<i32>(1, 2, 3, 4);
    let x = vec0.x;
    let y = vec0[1];
}

fn pattern2_storage_buffer_matrix() {
    let l3_ = baz.m[0][1];
}

fn pattern3_read_modify_write() {
    var vec0 = vec4<i32>(1, 2, 3, 4);
    vec0[1] = vec0[1] + 1;
}
