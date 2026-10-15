// Math builtins: normalize, length, dot, cross, clamp, mix, smoothstep, abs, sign, floor, ceil.

@compute @workgroup_size(1)
fn main() {
    let v3 = vec3<f32>(1.0, 2.0, 3.0);
    let v3b = vec3<f32>(4.0, 5.0, 6.0);
    let f = 0.5;

    // Geometric
    let n = normalize(v3);
    let l = length(v3);
    let d = dot(v3, v3b);
    let c = cross(v3, v3b);

    // Interpolation
    let cl = clamp(f, 0.0, 1.0);
    let m = mix(v3, v3b, 0.5);
    let s = smoothstep(0.0, 1.0, f);

    // Common math
    let a = abs(-1.0);
    let sg = sign(-5.0);
    let fl = floor(1.7);
    let ce = ceil(1.3);

    // More builtins
    let mn = min(1.0, 2.0);
    let mx = max(1.0, 2.0);
    let sq = sqrt(4.0);
    let pw = pow(2.0, 3.0);
    let ex = exp(1.0);
    let lg = log(1.0);
}
