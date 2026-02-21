struct Foo {
    a: vec4<f32>,
    b: i32,
}

@compute @workgroup_size(1)
fn main() {
    var foo: Foo;
    foo = Foo(vec4<f32>(1.0), 1);

    let m0 = mat2x2<f32>(
        1.0, 0.0,
        0.0, 1.0,
    );

    // zero value constructors
    let zvc0 = bool();
    let zvc1 = i32();
    let zvc2 = u32();
    let zvc3 = f32();
    let zvc4 = vec2<u32>();

    // constructors that infer their type from their parameters
    let cit0 = vec2(0u);
    let cit1 = mat2x2(vec2(0.), vec2(0.));
    let cit2 = array(0, 1, 2, 3);

    // conversion constructors
    let cc0 = i32(1u);
    let cc1 = f32(1i);
    let cc2 = u32(true);
}
