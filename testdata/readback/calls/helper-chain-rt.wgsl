// A helper inlined inside a switch case of another inlined helper keeps its body, and so does a helper whose result
// goes through a var. src = [a, b, mode 3, mode 1, mode 0] = [4, 7, 3, 1, 0].
// Expected: outer(4, 7, 3) = inner(4, 7) = 47; outer(4, 7, 1) = 4; outer(4, 7, 0) = 7; twice(4, 7) = 94;
// the switch in the entry point with mode 3 sets c = inner(4, 7) = 47.
// DXIL (P0-DXIL-055): DCE swept the stores of a chain of two locals and the result was read from an unwritten alloca.
@group(0) @binding(0) var<storage, read> src: array<u32, 5>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

fn inner(a: u32, b: u32) -> u32 {
    return a * 10u + b;
}

fn outer(a: u32, b: u32, mode: u32) -> u32 {
    switch mode {
        case 1u: { return a; }
        case 3u: { return inner(a, b); }
        default: { return b; }
    }
}

fn twice(a: u32, b: u32) -> u32 {
    return inner(a, b) * 2u;
}

@compute @workgroup_size(1)
fn main() {
    let a = src[0];
    let b = src[1];
    dst[0] = outer(a, b, src[2]);
    dst[1] = outer(a, b, src[3]);
    dst[2] = outer(a, b, src[4]);
    dst[3] = twice(a, b);
    var c = b;
    switch src[2] {
        case 1u: { c = a; }
        case 3u: { c = inner(a, b); }
        default: {}
    }
    dst[4] = c;
}
