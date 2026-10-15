// Fixed-size arrays, array constructors, indexing.

const DATA_SIZE: u32 = 8u;

fn sum_array(arr: array<f32, 4>) -> f32 {
    var total: f32 = 0.0;
    for (var i: u32 = 0u; i < 4u; i = i + 1u) {
        total = total + arr[i];
    }
    return total;
}

@compute @workgroup_size(1)
fn main() {
    // Array construction
    let arr1 = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    let arr2 = array<i32, 3>(10, 20, 30);
    let arr3 = array<vec2<f32>, 2>(vec2<f32>(1.0, 2.0), vec2<f32>(3.0, 4.0));

    // Indexing
    let first = arr1[0];
    let last = arr1[3];
    let vec_elem = arr3[1].y;

    // Mutable array
    var data: array<u32, 4>;
    data[0] = 100u;
    data[1] = 200u;
    data[2] = data[0] + data[1];
    data[3] = data[2] * 2u;

    // Dynamic indexing
    var idx: u32 = 2u;
    let dynamic_val = data[idx];

    // Pass to function
    let total = sum_array(arr1);
}
