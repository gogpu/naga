// A vector argument is the variable's value when the argument is evaluated: WGSL evaluates the arguments of a call
// left to right, so bump, the later argument, changes acc only after acc was passed. src = [10, 20].
// Expected: f(acc, bump(&acc)) = 10 + 0 = 10, then acc = (11, 21).
// DXIL (#128 review): an inlined callee re-loaded acc after bump and wrote 11.
@group(0) @binding(0) var<storage, read> src: array<i32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32>;

fn bump(p: ptr<function, vec2<i32>>) -> i32 {
    *p = *p + vec2<i32>(1, 1);
    return 0;
}

fn f(a: vec2<i32>, b: i32) -> i32 {
    return a.x + b;
}

@compute @workgroup_size(1)
fn main() {
    var acc = vec2<i32>(src[0], src[1]);
    dst[0] = f(acc, bump(&acc));
    dst[1] = acc.x;
    dst[2] = acc.y;
}
