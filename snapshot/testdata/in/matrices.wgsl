// Matrix construction, transpose, column access, mat * vec.

@compute @workgroup_size(1)
fn main() {
    // Identity matrix
    let identity = mat4x4<f32>(
        1.0, 0.0, 0.0, 0.0,
        0.0, 1.0, 0.0, 0.0,
        0.0, 0.0, 1.0, 0.0,
        0.0, 0.0, 0.0, 1.0,
    );

    // Smaller matrices
    let m2 = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
    let m3 = mat3x3<f32>(
        1.0, 0.0, 0.0,
        0.0, 1.0, 0.0,
        0.0, 0.0, 1.0,
    );

    // Non-square matrix
    let m4x3 = mat4x3<f32>(
        1.0, 0.0, 0.0,
        0.0, 1.0, 0.0,
        0.0, 0.0, 1.0,
        0.0, 0.0, 0.0,
    );

    // Matrix * vector
    let v = vec4<f32>(1.0, 2.0, 3.0, 1.0);
    let transformed = identity * v;

    // Matrix * matrix
    let combined = identity * identity;

    // Transpose
    let t2 = transpose(m2);

    // Column access
    let col0 = identity[0];
    let col1 = identity[1];
    let element = identity[2][3];

    // Matrix * scalar
    let scaled = m2 * 2.0;
}
