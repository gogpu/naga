// Nested structs, member access, struct constructors.

struct Inner {
    value: f32,
    flag: u32,
}

struct Outer {
    position: vec3<f32>,
    inner: Inner,
    scale: f32,
}

fn process_struct(s: Outer) -> f32 {
    return s.position.x * s.scale + s.inner.value;
}

@compute @workgroup_size(1)
fn main() {
    // Struct construction
    let inner = Inner(3.14, 1u);
    let outer = Outer(vec3<f32>(1.0, 2.0, 3.0), inner, 2.0);

    // Member access
    let pos = outer.position;
    let val = outer.inner.value;
    let flag = outer.inner.flag;

    // Modify via var
    var mutable_outer = outer;
    mutable_outer.scale = 5.0;
    mutable_outer.inner.value = 42.0;

    // Pass to function
    let result = process_struct(mutable_outer);
}
