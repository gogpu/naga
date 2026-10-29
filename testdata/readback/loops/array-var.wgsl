// A loop reads and writes an initialized local array by the loop counter
// (wgpu-d2 DXIL loop probe, naga v0.20.3).
@group(0) @binding(0) var<storage, read_write> result: array<u32>;

@compute @workgroup_size(1)
fn main() {
    var a = array<u32, 2>(5u, 6u);
    for (var i = 0u; i < 2u; i++) {
        a[i] = a[i] + 1u;
        result[i] = a[i];
    }
}
