// Returns from inside a switch inside a loop of a helper, a break that leaves only the switch, a void helper that
// returns early through a pointer argument, and a break in a switch outside any loop.
// src = [0, 3, 0, 1, 2, 0, 3, 4, 1, 0].
// h(0, 3): no return, case 3 breaks only the switch: acc = 70. h(0, 1): returns 21 + 1000 at i = 2. h(2, 0): returns
// 1000 at i = 0. bump adds only odd values: 3 + 7 = 10. The last switch breaks before r = 7u: r stays 5.
// DXIL (P0-DXIL-055, P0-DXIL-057): the inline pass turned a return into a break inside a synthetic loop, and the
// emitter sent a break in a switch to the loop's exit; a break in a switch outside a loop did not compile.
@group(0) @binding(0) var<storage, read> src: array<u32, 10>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

fn h(a: u32, b: u32) -> u32 {
    var acc = 0u;
    for (var i = 0u; i < 6u; i++) {
        switch (a + i) % 4u {
            case 2u: { if i >= b % 4u { return acc + 1000u; } }
            case 3u: { break; }
            default: { acc = acc + i; }
        }
        acc = acc + 10u;
    }
    return acc;
}

fn bump(p: ptr<function, u32>, a: u32) {
    if a % 2u == 0u { return; }
    *p = *p + a;
}

@compute @workgroup_size(1)
fn main() {
    dst[0] = h(src[0], src[1]);
    dst[1] = h(src[2], src[3]);
    dst[2] = h(src[4], src[5]);
    var x = 0u;
    bump(&x, src[6]);
    bump(&x, src[7]);
    bump(&x, src[6] + src[7]);
    dst[3] = x;
    var r = 5u;
    switch src[8] {
        case 1u: {
            if src[9] == 0u { break; }
            r = 7u;
        }
        default: { r = 9u; }
    }
    dst[4] = r;
}
