// A loop fills a workgroup array; after the barrier its first and last
// elements are copied out (wgpu-d2 DXIL loop probe, naga v0.20.3).
@group(0) @binding(0) var<storage, read_write> result: array<u32>;

var<workgroup> a: array<u32, 4>;

@compute @workgroup_size(1)
fn main() {
    for (var j = 0u; j < 4u; j++) {
        a[j] = j + 1u;
    }
    workgroupBarrier();
    result[0] = a[0];
    result[1] = a[3];
}
