// Atomic operations: atomicLoad, atomicStore, atomicAdd on workgroup var.

var<workgroup> shared_counter: atomic<u32>;

@group(0) @binding(0)
var<storage, read_write> result: array<u32>;

@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) lid: u32) {
    if lid == 0u {
        atomicStore(&shared_counter, 0u);
    }

    workgroupBarrier();

    atomicAdd(&shared_counter, 1u);

    workgroupBarrier();

    if lid == 0u {
        result[0] = atomicLoad(&shared_counter);
    }
}
