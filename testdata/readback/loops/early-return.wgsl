// `return` from inside a loop: in a helper function (the search result) and
// in the entry point itself (the writes after it must not happen).
@group(0) @binding(0) var<storage, read> data: array<u32, 9>;
@group(0) @binding(1) var<storage, read_write> result: array<u32>;

// first_at_least returns the index of the first of data[0..8] that is >= t,
// or 99 if there is none.
fn first_at_least(t: u32) -> u32 {
    for (var i = 0u; i < 8u; i++) {
        if data[i] >= t {
            return i;
        }
    }
    return 99u;
}

@compute @workgroup_size(1)
fn main() {
    result[0] = first_at_least(9u);
    result[1] = first_at_least(10u);
    result[2] = first_at_least(13u);
    for (var i = 0u; i < 4u; i++) {
        result[3u + i] = i + 1u;
        if i == data[8] {
            return;
        }
    }
}
