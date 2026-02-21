// Collatz conjecture: storage buffer read/write, while loop, modulo.

struct Data {
    values: array<u32>
}

@group(0) @binding(0)
var<storage, read_write> data: Data;

fn collatz_iterations(n_base: u32) -> u32 {
    var n = n_base;
    var i: u32 = 0u;
    while n > 1u {
        if n % 2u == 0u {
            n = n / 2u;
        } else {
            n = 3u * n + 1u;
        }
        i = i + 1u;
    }
    return i;
}

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) gid: vec3<u32>) {
    data.values[gid.x] = collatz_iterations(data.values[gid.x]);
}
