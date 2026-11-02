// A variable declared before a loop and updated inside it carries its value from one iteration to the next, also
// when the loop body reads it before it stores it in the same block. src = [4, 5, 9, 2, 9]: n = 4, then four values.
// Expected:
//   for:           x += i + 1, written in the loop:              1, 3, 6, 10
//   while:         y += 2j + 1, written in the loop:             1, 4, 9, 16
//   loop:          w += i + 1, written in the loop; then i:      1, 3, 6, 10, 4
//   octaves:       v = v + a; a = a / 2 from 16; v after:        30
//   read after:    z += k * k, read only after the loop:         14
//   vec3 branches: acc += only inside if / else-if branches:    12, 4, 6
//   argmax:        if e > m { m = e; idx = k; }:                 m = 9, idx = 1
//   digits:        t % 10; t = t / 10 from 437:                  7, 3, 4
// DXIL (P0-DXIL-046): mem2reg promoted a variable whose loads and stores all sat in one block inside a loop as if
// the block ran once, so every iteration read the value before the loop: for 1, 2, 3, 4; while 1, 3, 5, 7; loop
// 1, 2, 3, 4 (its counter, read after the loop, was kept); octaves 64 (a stayed 16; v, read after the loop, was
// kept). A variable also read outside that block was right, so the read-after, vec3, argmax and digits forms guard
// the fix against over-correction.
@group(0) @binding(0) var<storage, read> src: array<u32, 5>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(1)
fn main() {
    let n = src[0];

    var x: u32;
    for (var i = 0u; i < n; i++) {
        x += i + 1u;
        dst[i] = x;
    }

    var y: u32;
    var j: u32 = 0u;
    while j < n {
        y += 2u * j + 1u;
        dst[4u + j] = y;
        j++;
    }

    var w: u32;
    var i: u32 = 0u;
    loop {
        if i >= n {
            break;
        }
        w += i + 1u;
        dst[8u + i] = w;
        i++;
    }
    dst[12] = i;

    var v: u32 = 0u;
    var a: u32 = 16u;
    for (var k = 0u; k < n; k++) {
        v = v + a;
        a = a / 2u;
    }
    dst[13] = v;

    var z: u32 = 0u;
    for (var k = 0u; k < n; k++) {
        z += k * k;
    }
    dst[14] = z;

    var acc = vec3<u32>(0u);
    for (var k = 0u; k < n; k++) {
        if k % 2u == 0u {
            acc = acc + vec3<u32>(1u, 2u, 3u);
        } else if k == 1u {
            acc = acc + vec3<u32>(10u, 0u, 0u);
        }
    }
    dst[15] = acc.x;
    dst[16] = acc.y;
    dst[17] = acc.z;

    var m: u32 = 0u;
    var idx: u32 = 0u;
    for (var k = 0u; k < n; k++) {
        let e = src[1u + k];
        if e > m {
            m = e;
            idx = k;
        }
    }
    dst[18] = m;
    dst[19] = idx;

    var t: u32 = n * 100u + 37u;
    for (var k = 0u; k < 3u; k++) {
        dst[20u + k] = t % 10u;
        t = t / 10u;
    }
}
