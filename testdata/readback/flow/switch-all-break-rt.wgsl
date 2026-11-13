// When every case of a switch ends in break, the break leaves only the switch: the statements after it run.
// x = 1: case 1 writes dst[0] = 20, then dst[1] = 50 after the switch.
// Loop i = 0..3, every case breaks, then s = s + x after the switch: i 0 → 1, i 1 → 1 + 10 + 1 = 12, i 2 → 13,
// i 3 → 14. Expected [20, 50, 14, fill].
// A compiler that treats the all-break switch's merge as unreachable drops the stores after it (dst[1], dst[2]
// keep the sentinel); naga's SPIR-V writer did that from v0.15.0 to v0.21.10.
@group(0) @binding(0) var<storage, read> src: array<i32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32>;

@compute @workgroup_size(1)
fn main() {
    let x = src[0];
    switch x {
        case 1: { dst[0] = 20; break; }
        default: { dst[0] = 40; break; }
    }
    dst[1] = 50;
    var s = 0;
    for (var i = 0; i < 4; i++) {
        switch i {
            case 1: { s = s + 10; break; }
            default: { break; }
        }
        s = s + x;
    }
    dst[2] = s;
}
