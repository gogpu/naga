// A workgroup vector stored whole and accessed only through run-time indices. src = [2, 1, 0, 2]:
// v = (1, 2, 3); v[2] = 7 → (1, 2, 7); reads v[1], v[0], v[2] → [2, 1, 7].
// DXIL in v0.20.4 and v0.20.5 made v one scalar: every store went to it and the index was dropped, so this read
// back [7, 7, 7] from DXIL that IDxcValidator accepts (P1-DXIL-066).
@group(0) @binding(0) var<storage, read> src: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

var<workgroup> v: vec3<u32>;

@compute @workgroup_size(1)
fn main() {
    v = vec3<u32>(1u, 2u, 3u);
    v[src[0]] = 7u;
    dst[0] = v[src[1]];
    dst[1] = v[src[2]];
    dst[2] = v[src[3]];
}
