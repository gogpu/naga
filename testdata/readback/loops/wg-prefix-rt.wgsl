// A struct in a function variable survives a workgroup-prefix loop over storage reads: the struct is loaded
// whole from storage, one component is read-modify-written once per iteration, and the struct is stored back.
// tile = {reduced: 100, base: 7}, prefix = [1, 2, 3, 4] → reduced = 110, base = 7.
// P0-DXIL-049: gg's tilecompute draw_leaf carries its accumulator through an alloca across a wg_prefix loop;
// a phi-placement bug dropped the binding and the loop, and only a run-time check sees it.
struct Tile {
    reduced: u32,
    base: u32,
}

@group(0) @binding(0) var<storage, read_write> tile: Tile;
@group(0) @binding(1) var<storage, read> prefix: array<u32, 4>;

@compute @workgroup_size(1)
fn main() {
    var t = tile;
    var i = 0u;
    loop {
        if (i >= 4u) {
            break;
        }
        t.reduced = t.reduced + prefix[i];
        i = i + 1u;
    }
    tile = t;
}
