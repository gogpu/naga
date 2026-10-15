// BUG-DXIL-005 Phase 1: minimal repro for local vector variable
// component assign — baseline to verify existing local-var path works.
//
// `var v: vec2<f32>; v.x = 1.0; out[0] = v;` — no struct wrapper.

@group(0) @binding(0) var<storage, read_write> out: array<vec2<f32>>;

@compute @workgroup_size(1)
fn main() {
    var v = vec2<f32>(0.0, 0.0);
    v.x = 1.0;
    v.y = 2.0;
    out[0] = v;
}
