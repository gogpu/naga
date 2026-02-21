fn add(a: f32, b: f32) -> f32 {
    return a + b;
}

fn multiply(a: vec2<f32>, b: vec2<f32>) -> vec2<f32> {
    return a * b;
}

fn test_fma() -> vec2<f32> {
    let a = vec2<f32>(2.0, 2.0);
    let b = vec2<f32>(0.5, 0.5);
    let c = vec2<f32>(0.5, 0.5);
    return fma(a, b, c);
}

@compute @workgroup_size(1)
fn main() {
    let sum = add(1.0, 2.0);
    let prod = multiply(vec2<f32>(2.0, 3.0), vec2<f32>(4.0, 5.0));
    let fma_result = test_fma();
}
