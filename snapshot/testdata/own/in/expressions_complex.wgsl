// Nested expressions: compound arithmetic, select().

@compute @workgroup_size(1)
fn main() {
    let a = 1.0;
    let b = 2.0;
    let c = 3.0;
    let d = 4.0;

    // Nested arithmetic
    let expr1 = a * b + c * d;
    let expr2 = (a + b) * (c - d);
    let expr3 = a / b - c / d;
    let expr4 = (a + b + c) * d;

    // Select (ternary-like)
    let s1 = select(a, b, true);
    let s2 = select(vec3<f32>(0.0), vec3<f32>(1.0), false);
    let s3 = select(10, 20, a > b);

    // Vector operations
    let v1 = vec3<f32>(a, b, c);
    let v2 = vec3<f32>(d, c, b);
    let result = v1 * v2 + vec3<f32>(1.0);

    // Compound expressions with builtins
    let l = length(v1) * dot(v1, v2);
    let n = normalize(v1 + v2);
}
