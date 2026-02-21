// Pointer parameters to functions, pointer deref.

fn increment(p: ptr<function, i32>) {
    *p = *p + 1;
}

fn read_value(p: ptr<function, f32>) -> f32 {
    return *p;
}

fn swap(a: ptr<function, i32>, b: ptr<function, i32>) {
    let tmp = *a;
    *a = *b;
    *b = tmp;
}

fn modify_array_element(arr: ptr<function, array<f32, 4>>, idx: u32, val: f32) {
    (*arr)[idx] = val;
}

struct Data {
    value: f32,
    count: u32,
}

fn modify_struct_field(d: ptr<function, Data>) {
    (*d).value = (*d).value * 2.0;
    (*d).count = (*d).count + 1u;
}

@compute @workgroup_size(1)
fn main() {
    // Basic pointer operations
    var x: i32 = 10;
    increment(&x);
    increment(&x);

    // Read through pointer
    var f: f32 = 3.14;
    let val = read_value(&f);

    // Swap values
    var a: i32 = 1;
    var b: i32 = 2;
    swap(&a, &b);

    // Array through pointer
    var arr = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    modify_array_element(&arr, 0u, 99.0);

    // Struct through pointer
    var data = Data(1.0, 0u);
    modify_struct_field(&data);
}
