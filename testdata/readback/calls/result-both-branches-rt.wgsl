// A helper that returns from both sides of an if, called with a run-time argument, and its result used on both
// sides of an if in the caller. k = 2, one invocation per li in 0..3, x = li + 3:
// li 0: x 3 → g = 3 + 1 + 2 = 6, o = 6. li 1: x 4 → g = 7, o = 107. li 2: x 5 (not > 5) → g = 8, o = 108.
// li 3: x 6 → g = 6 * 2 + 2 = 14, o = 114.
@group(0) @binding(0) var<uniform> k: vec4<i32>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32>;

fn g(x: i32) -> i32 {
    if (x > 5) {
        return x * 2 + k.x;
    }
    return x + 1 + k.x;
}

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) li: u32) {
    let r = g(i32(li) + 3);
    var o = 0;
    if (li == 0u) {
        o = r;
    } else {
        o = r + 100;
    }
    dst[li] = o;
}
