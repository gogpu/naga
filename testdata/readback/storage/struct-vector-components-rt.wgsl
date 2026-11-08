// A whole storage struct of three vec2<u32> — six components in one element access — is loaded into a function
// variable, changed and stored back: a = (1, 2) + (10, 20) = (11, 22), b = (3, 4) + (30, 10) = (13, 14),
// c = (5, 6) + (20, 30) = (25, 36).
// P0-DXIL-050 / P1-DXIL-072: an element of more than four components was read with dx.op.rawBufferLoad, whose
// mask and alignment operands need shader model 6.2, so every shader with one was refused below 6.2 while DXC
// writes dx.op.bufferLoad there.
struct V {
    a: vec2<u32>,
    b: vec2<u32>,
    c: vec2<u32>,
}

@group(0) @binding(0) var<storage, read_write> v: V;
@group(0) @binding(1) var<storage, read> m: array<u32, 3>;

@compute @workgroup_size(1)
fn main() {
    var t = v;
    t.a = t.a + vec2<u32>(m[0], m[1]);
    t.b = t.b + vec2<u32>(m[2], m[0]);
    t.c = t.c + vec2<u32>(m[1], m[2]);
    v = t;
}
