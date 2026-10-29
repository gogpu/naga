// A var declared without initializer inside an inner loop body starts at 0 on every iteration of both loops.
// out[i*n + j] = (x += j + 1) with x reset each inner iteration = j + 1. A hoisted x that is never reset
// accumulates instead.
// n = 3: 1, 2, 3, 1, 2, 3, 1, 2, 3
@group(0) @binding(0) var<storage, read> n: u32;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < n; i++) {
        for (var j = 0u; j < n; j++) {
            var x: u32;
            x += j + 1u;
            dst[i * n + j] = x;
        }
    }
}
