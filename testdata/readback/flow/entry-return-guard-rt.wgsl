// The bounds guard almost every compute kernel starts with: an invocation past the end returns before it writes.
// 8 invocations, n = src[0] = 5: dst[0..5] = i * 3 + 1, dst[5..8] keep the fill.
// DXIL (P0-DXIL-052): a return in an entry point did nothing, so every invocation wrote.
@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(8)
fn main(@builtin(local_invocation_index) i: u32) {
    if i >= src[0] {
        return;
    }
    dst[i] = i * 3u + 1u;
}
