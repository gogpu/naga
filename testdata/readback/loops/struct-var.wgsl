// A loop updates one member of an initialized local struct; the other member
// must keep its initializer (wgpu-d2 DXIL loop probe, naga v0.20.3).
@group(0) @binding(0) var<storage, read_write> result: array<u32>;

struct S {
    x: u32,
    y: u32,
}

@compute @workgroup_size(1)
fn main() {
    var s = S(1u, 2u);
    for (var i = 0u; i < 3u; i++) {
        s.x = s.x + 1u;
    }
    result[0] = s.x;
    result[1] = s.y;
}
