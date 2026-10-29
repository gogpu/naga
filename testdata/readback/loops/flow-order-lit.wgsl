// Execution order of loop, continue, break, continuing and break-if, recorded
// as the WebGPU CTS flow-control harness does: push() appends an event id.
struct Out {
    count: u32,
    data: array<u32>,
}

@group(0) @binding(0) var<storage, read_write> out: Out;

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
        if i == 1u {
            continue;
        }
        if i == 3u {
            break;
        }
        push(10u + i);
        continuing {
            push(100u + i);
            i = i + 1u;
            break if i >= 5u;
        }
    }
    push(1000u);
}
