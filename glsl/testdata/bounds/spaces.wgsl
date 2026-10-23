struct U {
    a: array<vec4<f32>, 4>,
    m: mat2x2<f32>,
    ms: array<mat2x2<f32>, 3>,
}
@group(0) @binding(0) var<uniform> u: U;
@group(0) @binding(1) var<storage, read_write> rt: array<u32>;
@group(0) @binding(2) var<storage, read_write> counters: array<atomic<u32>>;
var<private> p: array<f32, 5>;
var<workgroup> wg: array<atomic<u32>, 8>;
var<workgroup> wf: array<f32, 8>;

@compute @workgroup_size(8)
fn main(@builtin(local_invocation_index) li: u32) {
    var l = array<i32, 3>(1, 2, 3);
    let i = i32(rt[li]);
    l[i] = 7;
    p[li] = f32(l[i + 1]);
    wf[li] = u.a[i].x + u.m[i].y + u.ms[i][li].x;
    atomicAdd(&wg[li], 1u);
    atomicAdd(&counters[i], 1u);
    var v = vec4<f32>(p[2]);
    v[i] = wf[li];
    let m = mat3x3<f32>(vec3(1.0), vec3(2.0), vec3(3.0));
    rt[li] = u32(m[i][li] + v[li]);
    rt[3] = arrayLength(&rt);
}
