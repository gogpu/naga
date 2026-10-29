// A loop indexes a local vector by the loop counter (v0.20.3 DXIL bug: v[i]
// with a run-time index always addressed component 0).
@group(0) @binding(0) var<storage, read_write> result: array<u32>;

@compute @workgroup_size(1)
fn main() {
    var v = vec2<u32>(10u, 20u);
    for (var i = 0u; i < 2u; i++) {
        v[i] = v[i] + 1u;
    }
    result[0] = v.x;
    result[1] = v.y;
}
