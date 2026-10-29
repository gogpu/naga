// A loop that only ends by break-if in its continuing block; the limit is
// read at run time.
struct Out {
    count: u32,
    data: array<u32>,
}

@group(0) @binding(0) var<storage, read> limits: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> out: Out;

fn push(id: u32) {
    out.data[out.count] = id;
    out.count += 1u;
}

@compute @workgroup_size(1)
fn main() {
    out.count = 0u;
    var i = 0u;
    loop {
        push(i);
        continuing {
            i = i + 1u;
            push(100u + i);
            break if i == limits[0];
        }
    }
    push(1000u);
}
