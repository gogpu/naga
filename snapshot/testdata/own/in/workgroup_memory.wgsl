// Workgroup shared memory, storageBarrier, workgroupBarrier.

var<workgroup> shared_data: array<f32, 256>;

struct Params {
    count: u32,
}

@group(0) @binding(0)
var<uniform> params: Params;

@group(0) @binding(1)
var<storage, read_write> output: array<f32>;

@compute @workgroup_size(256)
fn main(
    @builtin(local_invocation_index) lid: u32,
    @builtin(global_invocation_id) gid: vec3<u32>,
) {
    // Load into shared memory
    shared_data[lid] = f32(gid.x) * 0.5;

    workgroupBarrier();

    // Simple reduction: each thread reads neighbor
    if lid < 128u {
        shared_data[lid] = shared_data[lid] + shared_data[lid + 128u];
    }

    workgroupBarrier();

    // Write result
    if lid == 0u {
        output[gid.x / 256u] = shared_data[0];
    }
}
