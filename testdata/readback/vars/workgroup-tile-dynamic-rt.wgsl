// A workgroup array of vectors, the tile of a tiled kernel: invocation i writes tile[i] = (4i, 4i+1, 4i+2, 4i+3),
// then loads tile[3 - i] whole and reads one component at a run-time index src[i]. src = [1, 2, 3, 0]:
// tile[3][1] = 13, tile[2][2] = 10, tile[1][3] = 7, tile[0][0] = 0 → [13, 10, 7, 0].
// DXIL in v0.20.4 and v0.20.5 made the array one scalar per element ([4 x i32]): the whole store kept only .x and the
// read dropped the index, valid DXIL that read back [12, 8, 4, 0] (P1-DXIL-066).
@group(0) @binding(0) var<storage, read> src: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

var<workgroup> tile: array<vec4<u32>, 4>;

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) i: u32) {
    tile[i] = vec4<u32>(4u * i, 4u * i + 1u, 4u * i + 2u, 4u * i + 3u);
    workgroupBarrier();
    let t = tile[3u - i];
    dst[i] = t[src[i]];
}
