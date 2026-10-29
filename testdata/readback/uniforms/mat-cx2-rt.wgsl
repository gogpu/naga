// Two-row matrices in a uniform buffer keep the WGSL layout: columns 8 bytes apart, the next member right after.
// std140 (GLSL uniform blocks, Vulkan without uniformBufferStandardLayout) puts matrix columns 16 bytes apart,
// so a backend that declares the WGSL type as it is reads the second column and every later member from the
// wrong place.
// u = (m = [[1, 2], [3, 4]], v = (5, 6), m3 = [[7, 8], [9, 10], [11, 12]], w = (13, 14)), i = 1 from storage:
//   m by constant column: 1 2 3 4; v: 5 6; m3: 7 8 9 10 11 12; w: 13 14
//   m[i] = (3, 4); m3[i + 1] = (11, 12); the whole m loaded: m * (1, 1) = (4, 6)
struct U {
    m: mat2x2<f32>,
    v: vec2<f32>,
    m3: mat3x2<f32>,
    w: vec2<f32>,
}

@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var<storage, read> idx: array<u32, 1>;
@group(0) @binding(2) var<storage, read_write> dst: array<f32, 21>;

@compute @workgroup_size(1)
fn main() {
    let i = idx[0];
    dst[0] = u.m[0].x;
    dst[1] = u.m[0].y;
    dst[2] = u.m[1].x;
    dst[3] = u.m[1].y;
    dst[4] = u.v.x;
    dst[5] = u.v.y;
    dst[6] = u.m3[0].x;
    dst[7] = u.m3[0].y;
    dst[8] = u.m3[1].x;
    dst[9] = u.m3[1].y;
    dst[10] = u.m3[2].x;
    dst[11] = u.m3[2].y;
    dst[12] = u.w.x;
    dst[13] = u.w.y;
    let col = u.m[i];
    dst[14] = col.x;
    dst[15] = col.y;
    let col3 = u.m3[i + 1u];
    dst[16] = col3.x;
    dst[17] = col3.y;
    let m = u.m;
    let s = m * vec2<f32>(1.0, 1.0);
    dst[18] = s.x;
    dst[19] = s.y;
}
