// A var<private> vector written whole, updated whole, then read per component. src = [10, 20].
// Expected acc = (10, 20) + (1, 1) = (11, 21).
// DXIL (P0-DXIL-059): a private vector got one scalar alloca; it is now refused at compile time.
@group(0) @binding(0) var<storage, read> src: array<i32, 2>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32>;

var<private> acc: vec2<i32>;

@compute @workgroup_size(1)
fn main() {
    acc = vec2<i32>(src[0], src[1]);
    acc = acc + vec2<i32>(1, 1);
    dst[0] = acc.x;
    dst[1] = acc.y;
}
