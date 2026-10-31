// var<workgroup> memory is zero when the workgroup starts (WGSL: workgroup variables are zero-initialized).
// Read before any write, through a run-time index offset o = 0: sixteen array elements, then two scalars.
// Expected 18 zeros. vars/uninit-read-rt covers the function and private address spaces,
// vars/workgroup-composite-rt workgroup vectors and structs.
@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

var<workgroup> w: array<u32, 16>;
var<workgroup> x: u32;
var<workgroup> y: i32;

@compute @workgroup_size(1)
fn main() {
    let o = src[0];
    for (var i = 0u; i < 16u; i++) {
        dst[i] = w[i + o];
    }
    dst[16] = x;
    dst[17] = u32(y);
}
