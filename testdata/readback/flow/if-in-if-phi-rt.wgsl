// A var set in a nested if and a var set in a switch, then both read: the values meet at the end of each
// construct (a phi in SSA backends). Six invocations, li = 0..5:
// x: li < 2 → 1, li 2..3 → 2, li ≥ 4 → 0. y: li 0 → 1, li 1 → 3, otherwise 7.
// Expected x * 10 + y: 11, 13, 27, 27, 7, 7.
@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(6)
fn main(@builtin(local_invocation_index) li: u32) {
    var x = 0u;
    if (li < 4u) {
        if (li < 2u) {
            x = 1u;
        } else {
            x = 2u;
        }
    }
    var y = 7u;
    switch (li) {
        case 0u: { y = 1u; }
        case 1u: { y = 3u; }
        default: {}
    }
    dst[li] = x * 10u + y;
}
