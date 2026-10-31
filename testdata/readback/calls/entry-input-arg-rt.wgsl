// An entry point's own input passed straight to a helper, next to an argument with a side effect.
// Two workgroups of two invocations: id.x = 0..3. bumpu runs once per invocation, so acc = 6 afterwards.
// Expected: dst[id.x] = (id.x + 0) * 10 + 6 = 6, 16, 26, 36.
// DXIL (#128 review): "function argument 0 not in value map" before #128.
@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

fn bumpu(p: ptr<function, u32>) -> u32 {
    *p = *p + 1u;
    return 0u;
}

fn f(id: vec3<u32>, k: u32) -> vec2<u32> {
    return vec2<u32>(id.x + k, 0u);
}

@compute @workgroup_size(2)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    var acc = src[0];
    let v = f(id, bumpu(&acc)).x;
    dst[id.x] = v * 10u + acc;
}
