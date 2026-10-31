// A struct or array argument is the variable's value when the argument is evaluated: WGSL evaluates the arguments
// of a call left to right, so a later argument that changes the variable through a pointer doesn't change it.
// src = [seed] = [7]. Expected: fs(s, bump(&s.a)) = 7, then s.a = 8; fa(arr, bump(&arr[0])) = 7, then arr[0] = 8.
// DXIL (P0-DXIL-062): the inlined callee read the argument's local after bump, 8.
struct S { a: u32, b: vec2<u32> }

@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

fn bump(p: ptr<function, u32>) -> u32 {
    *p = *p + 1u;
    return 0u;
}

fn fs(s: S, k: u32) -> u32 {
    return s.a + k;
}

fn fa(arr: array<u32, 2>, k: u32) -> u32 {
    return arr[0] + k;
}

@compute @workgroup_size(1)
fn main() {
    let seed = src[0];
    var s = S(seed, vec2<u32>(1u, 2u));
    dst[0] = fs(s, bump(&s.a));
    dst[1] = s.a;
    var arr = array<u32, 2>(seed, 20u);
    dst[2] = fa(arr, bump(&arr[0]));
    dst[3] = arr[0];
}
