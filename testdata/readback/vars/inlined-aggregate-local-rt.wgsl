// A function's array and struct variables start afresh at every call, also when the call sits in a loop and the
// backend inlines the function. src = [3]: three calls of each helper, i = 0, 1, 2. Expected:
//   arr_acc(i):  var arr = array(5, 6); arr[1] += i:              6, 7, 8
//   st_acc(i):   var st = S(5, 6); st.b += i:                     6, 7, 8
//   nest_acc(i): var nest = ((1, 2), (5, 6)); nest[1][1] += i:    6, 7, 8
//   zero_acc(i): var zarr: array<u32, 2>; zarr[1] += i + 6:       6, 7, 8
// DXIL (P0-DXIL-091): the inliner re-initialized a helper's scalar and vector locals at every call but left an
// array or struct its initializer, which the caller sets once, at entry, so each call started from what the
// previous one left: 6, 7, 9 for the initialized ones and 6, 13, 21 for zero_acc.
struct S { a: u32, b: u32 }

@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

fn arr_acc(x: u32) -> u32 {
    var arr = array<u32, 2>(5u, 6u);
    arr[1] = arr[1] + x;
    return arr[1];
}

fn st_acc(x: u32) -> u32 {
    var st = S(5u, 6u);
    st.b = st.b + x;
    return st.b;
}

fn nest_acc(x: u32) -> u32 {
    var nest = array<array<u32, 2>, 2>(array<u32, 2>(1u, 2u), array<u32, 2>(5u, 6u));
    nest[1][1] = nest[1][1] + x;
    return nest[1][1];
}

fn zero_acc(x: u32) -> u32 {
    var zarr: array<u32, 2>;
    zarr[1] = zarr[1] + x + 6u;
    return zarr[1];
}

@compute @workgroup_size(1)
fn main() {
    let n = src[0];
    for (var i = 0u; i < n; i++) {
        dst[i] = arr_acc(i);
        dst[3u + i] = st_acc(i);
        dst[6u + i] = nest_acc(i);
        dst[9u + i] = zero_acc(i);
    }
}
