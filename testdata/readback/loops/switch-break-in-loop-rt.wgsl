// In WGSL a break inside a switch case leaves the switch, not the loop around it. x = 3, i = 0..3, case 2 breaks:
// i 0 → s = 3, i 1 → s = 6, i 2 → break (switch only), i 3 → s = 9. Expected 9, in a helper and in the entry point.
// A compiler that leaves the loop at the break writes 6.
@group(0) @binding(0) var<storage, read> src: array<i32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<i32>;

fn h(x: i32) -> i32 {
    var s = 0;
    for (var i = 0; i < 4; i++) {
        switch (i) {
            case 2: { break; }
            default: { s = s + x; }
        }
    }
    return s;
}

@compute @workgroup_size(1)
fn main() {
    let x = src[0];
    dst[0] = h(x);
    var s = 0;
    for (var i = 0; i < 4; i++) {
        switch (i) {
            case 2: { break; }
            default: { s = s + x; }
        }
    }
    dst[1] = s;
}
