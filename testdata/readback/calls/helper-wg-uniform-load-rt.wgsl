// A helper reads workgroup memory with workgroupUniformLoad. Invocation 0 stores src[0] = 7 to the workgroup
// variable; every invocation of the workgroup of 4 calls shared_plus(src[1] = 3), which returns
// workgroupUniformLoad(&w) + 3 * 2 = 13. Expected dst = [13, 13, 13, 13].
// DXIL (P1-API-002 B7, e5eda7c): the inliner never remapped the pointer and result of a workgroupUniformLoad, so in
// the caller they named unrelated expressions.
@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

var<workgroup> w: u32;

fn shared_plus(k: u32) -> u32 {
    let extra = k * 2u;
    return workgroupUniformLoad(&w) + extra;
}

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) li: u32) {
    if li == 0u {
        w = src[0];
    }
    dst[li] = shared_plus(src[1]);
}
