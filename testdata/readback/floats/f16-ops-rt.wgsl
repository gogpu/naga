// f16 arithmetic, f16 stores into a storage struct and f16 → f32 conversion. src = [2.0].
// a = (1.5, 2.5) * 2 = (3, 5); b = a + 0.25 = (3.25, 5.25); hs = (-3.25 + 3, 5.25 - 5) = (-0.25, 0.25);
// v = (3.25, 5.25, 3.25, 5.25) as f32. Layout: v at byte 0, h at 16, hs at 20, struct size 32.
// Words: f32 3.25 = 0x40500000, 5.25 = 0x40A80000; f16 3.25 = 0x4280, 5.25 = 0x4540, -0.25 = 0xB400, 0.25 = 0x3400,
// low half first: h = 0x45404280, hs = 0x3400B400. Words 6 and 7 are padding and stay untouched.
enable f16;

struct Out {
    v: vec4<f32>,
    h: vec2<f16>,
    hs: vec2<f16>,
}

@group(0) @binding(0) var<storage, read> src: array<f32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: Out;

@compute @workgroup_size(1)
fn main() {
    let s = f16(src[0]);
    let a = vec2<f16>(1.5h, 2.5h) * s;
    let b = a + vec2<f16>(0.25h);
    dst.h = b;
    dst.hs = vec2<f16>(-b.x + 3.0h, b.y - 5.0h);
    dst.v = vec4<f32>(vec4<f16>(b, b));
}
