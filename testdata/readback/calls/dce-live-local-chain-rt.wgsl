// A helper result stored into the return slot of an inlined helper, whose own result goes to another live local,
// keeps its value. Each invocation picks a mode by its local_invocation_index; a = src[0..3], b = src[3..6] =
// (1, -2, 6.5) and (4, 9, 0.25), all exact in f32.
// Expected, three f32 per mode: mode 0 returns a = (1, -2, 6.5); mode 1 returns abs(a - b) = (3, 11, 6.25);
// mode 2 returns max(a, b) * 2 = (8, 18, 13); the tenth word is the untouched sentinel.
// DXIL (naga v0.20.4, fixed by d0e4cfd): dead code elimination found the dead locals once, before it marked the
// values stored into live locals, so the vec3 slot of difference() and of larger_twice(), read only by the store
// into pick()'s result, lost its store and was read back as its zero initializer: modes 1 and 2 wrote (0, 0, 0).
// The same pattern dropped the Difference and Soft Light branches of gg's blend shader; both validated.
@group(0) @binding(0) var<storage, read> src: array<f32, 6>;
@group(0) @binding(1) var<storage, read_write> dst: array<f32>;

fn difference(a: vec3<f32>, b: vec3<f32>) -> vec3<f32> {
    return abs(a - b);
}

fn larger_twice(a: vec3<f32>, b: vec3<f32>) -> vec3<f32> {
    return max(a, b) * 2.0;
}

fn pick(a: vec3<f32>, b: vec3<f32>, mode: u32) -> vec3<f32> {
    if mode == 1u {
        return difference(a, b);
    }
    if mode == 2u {
        return larger_twice(a, b);
    }
    return a;
}

@compute @workgroup_size(3)
fn main(@builtin(local_invocation_index) mode: u32) {
    let a = vec3<f32>(src[0], src[1], src[2]);
    let b = vec3<f32>(src[3], src[4], src[5]);
    let r = pick(a, b, mode);
    dst[mode * 3u] = r.x;
    dst[mode * 3u + 1u] = r.y;
    dst[mode * 3u + 2u] = r.z;
}
