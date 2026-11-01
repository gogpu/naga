// A helper compare-exchanges with a compare value it computes itself. src = [base 3, new value 9]; the atomic holds 5.
// cas(3, 9) compares with 3 + 2 = 5: it swaps in 9 and returns (old 5, exchanged 1). A second call compares with 5
// again and fails: (old 9, exchanged 0). Expected dst = [5, 1, 9, 0] and the atomic 9.
// DXIL (P1-API-002 B7, e5eda7c): the inliner never remapped the compare operand of an atomic compare-exchange, so in
// the caller it named an unrelated expression.
@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> a: atomic<u32>;
@group(0) @binding(2) var<storage, read_write> dst: array<u32>;

fn cas(base: u32, v: u32) -> vec2<u32> {
    let expected = base + 2u;
    var r = atomicCompareExchangeWeak(&a, expected, v);
    // A weak compare-exchange may fail spuriously: retry while the value still matches.
    for (var i = 0u; i < 64u && !r.exchanged && r.old_value == expected; i++) {
        r = atomicCompareExchangeWeak(&a, expected, v);
    }
    return vec2<u32>(r.old_value, select(0u, 1u, r.exchanged));
}

@compute @workgroup_size(1)
fn main() {
    let first = cas(src[0], src[1]);
    let second = cas(src[0], src[1]);
    dst[0] = first.x;
    dst[1] = first.y;
    dst[2] = second.x;
    dst[3] = second.y;
}
