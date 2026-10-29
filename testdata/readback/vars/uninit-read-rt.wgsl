// Function and private variables read before any write. WGSL zero-initializes a var without an initializer;
// the write happens only for a run-time value the case never passes, so no compiler can drop the read.
// Expected: 0 for each (u32, i32, a vec2<u32> component, a struct field), then the private initializer 9.
struct S { a: u32, b: u32 }

@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

var<private> p: i32;
var<private> q: u32 = 9u;

@compute @workgroup_size(1)
fn main() {
    var x: u32;
    var v: vec2<u32>;
    var s: S;
    if src[0] == 7u {
        x = 5u;
        p = 6;
        v = vec2(1u, 2u);
        s.b = 3u;
        q = 4u;
    }
    dst[0] = x;
    dst[1] = bitcast<u32>(p);
    dst[2] = v.y;
    dst[3] = s.b;
    dst[4] = q;
}
