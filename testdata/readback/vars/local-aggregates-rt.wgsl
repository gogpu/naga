// Function-local aggregates read and written by part, with run-time indices. src = (1, 2, 3, 4); i = 1, j = 2.
// dst[0]: a zero-initialized struct with a struct member, read by part: 0 (it started uninitialized)
// dst[1]: ss[1].a = 9 through a run-time index, then ss[1].a + ss[0].b = 9 + 2 = 11
// dst[2]: p[2].x = 8 through a run-time index of an array of vectors, then p[2].x * 10 + p[1].y = 80 + 4 = 84
// dst[3]: a run-time Compose into a struct with an array member: a = (2, 1), k = 2 -> 200 + 10 + 2 = 212
// dst[4]: a struct member that is a struct, copied and changed: inner 4, outer still 3 -> 43
// dst[5]: column components of a local matrix with columns (1, 2), (3, 4): m[1].y * 10 + m[1].x = 43
// dst[6]: a local array<u32, 6> indexed by a loop counter: 47 by strides (32, 8, 4, 2, 1) = 1, 1, 1, 1, 1 -> 11111
// dst[7]: a zero-initialized array of structs, one member written: 0 + 7 + 0 = 7
// Words 8 to 11 are not written.
struct S { a: i32, b: i32 }
struct Inner { delicious: i32 }
struct Outer { om_nom_nom: Inner, thing: u32 }
struct T { a: array<i32, 2>, k: i32 }

@group(0) @binding(0) var<storage, read> src: array<u32, 4>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32, 12>;

@compute @workgroup_size(1)
fn main() {
    let i = src[0];
    let j = src[1];

    var o: Outer;
    dst[0] = o.om_nom_nom.delicious + i32(o.thing);

    var ss = array<S, 2>(S(1, 2), S(3, 4));
    ss[i % 2u].a = 9;
    dst[1] = ss[i % 2u].a + ss[0].b;

    var p = array<vec2<f32>, 3>(vec2(1.0, 2.0), vec2(3.0, 4.0), vec2(5.0, 6.0));
    p[j].x = 8.0;
    dst[2] = i32(p[j].x * 10.0 + p[i].y);

    var t = T(array<i32, 2>(i32(j), 1), 2);
    dst[3] = t.a[0] * 100 + t.a[1] * 10 + t.k;

    var outer = Outer(Inner(i32(src[2])), 5u);
    var inner = outer.om_nom_nom;
    inner.delicious += 1;
    dst[4] = inner.delicious * 10 + outer.om_nom_nom.delicious;

    var m = mat2x2<f32>(f32(i), 2.0, 3.0, f32(src[3]));
    dst[5] = i32(m[1].y * 10.0 + m[i].x);

    var strides = array<u32, 6>(32u, 8u, 4u, 2u, 1u, 1u);
    var coords: array<u32, 6>;
    var rem = 46u + i;
    for (var d = 0u; d < 5u; d++) {
        coords[d] = rem / strides[d];
        rem = rem % strides[d];
    }
    dst[6] = i32(coords[0] + coords[1] * 10u + coords[2] * 100u + coords[3] * 1000u + coords[4] * 10000u + coords[5] * 100000u);

    var zs: array<Outer, 2>;
    zs[i].thing = 7u;
    dst[7] = zs[0].om_nom_nom.delicious + i32(zs[1].thing) + i32(zs[0].thing);
}
