// Nested loops; the inner one breaks when j passes i. The outer bound is
// read at run time.
@group(0) @binding(0) var<storage, read> bounds: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> result: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let n = bounds[0];
    for (var i = 0u; i < n; i++) {
        for (var j = 0u; j < n; j++) {
            if j > i {
                break;
            }
            result[i * n + j] = i * 10u + j;
        }
    }
}
