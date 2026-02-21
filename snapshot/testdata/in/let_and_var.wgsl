// Let vs var semantics, shadowing, scoping.

@compute @workgroup_size(1)
fn main() {
    // Let is immutable
    let x: f32 = 1.0;
    let y = 2.0;
    let z = x + y;

    // Var is mutable
    var a: f32 = 10.0;
    a = a + 1.0;
    a = a * 2.0;

    // Let with explicit types
    let i: i32 = 42;
    let u: u32 = 100u;
    let b: bool = true;
    let v: vec3<f32> = vec3<f32>(1.0, 2.0, 3.0);

    // Var with initialization
    var counter: u32 = 0u;
    counter = counter + 1u;
    counter = counter + 1u;

    // Shadowing in nested scope
    let value = 10;
    {
        let value = 20;
        var temp = value;
        temp = temp + 1;
    }

    // Uninitialized var
    var uninit_f: f32;
    uninit_f = 5.0;

    var uninit_v: vec2<u32>;
    uninit_v = vec2<u32>(1u, 2u);
}
