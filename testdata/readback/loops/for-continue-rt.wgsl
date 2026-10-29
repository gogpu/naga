// `continue` in a for-loop must still run the update expression (i++); the
// skipped value is read at run time.
struct Out {
    count: u32,
    data: array<u32>,
}

@group(0) @binding(0) var<storage, read> skip: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> out: Out;

fn push(id: u32) {
    out.data[out.count] = id;
    out.count += 1u;
}

@compute @workgroup_size(1)
fn main() {
    out.count = 0u;
    for (var i = 0u; i < 4u; i++) {
        if i == skip[0] {
            continue;
        }
        push(i);
    }
}
