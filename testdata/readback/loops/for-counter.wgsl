// A loop counter declared in the for-loop header (v0.20.3 DXIL bug: the
// counter's initializer was never stored, so the loop did not run).
@group(0) @binding(0) var<storage, read_write> result: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var j = 0u; j < 4u; j++) {
        result[j] = j + 1u;
    }
}
