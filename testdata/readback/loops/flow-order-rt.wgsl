// flow-order-lit with the continue, break and break-if values read at run
// time, so the compiler cannot unroll or fold the loop.
struct Out {
    count: u32,
    data: array<u32>,
}

@group(0) @binding(0) var<storage, read> limits: array<u32, 3>;
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
        if i == limits[0] {
            continue;
        }
        if i == limits[1] {
            break;
        }
        push(10u + i);
        continuing {
            push(100u + i);
            i = i + 1u;
            break if i >= limits[2];
        }
    }
    push(1000u);
}
