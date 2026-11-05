// A local vector read by a run-time index or by a component holds its initializer, or zero without one.
// src = [2, 3]. Expected:
//   v[i] (v = (10, 11, 12, 13)):   12
//   v[j]:                          13
//   v.y:                           11
//   z[i] + 7 (var z: vec4<u32>):   7
// DXIL (P0-DXIL-091): a vector local that was never stored resolved only whole-variable loads to its initializer
// (or zero); a read of a component went to per-component allocas nothing was stored to, and read uninitialized
// memory.
@group(0) @binding(0) var<storage, read> src: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let i = src[0];
    let j = src[1];

    var v = vec4<u32>(10u, 11u, 12u, 13u);
    dst[0] = v[i];
    dst[1] = v[j];
    dst[2] = v.y;

    var z: vec4<u32>;
    dst[3] = z[i] + 7u;
}
