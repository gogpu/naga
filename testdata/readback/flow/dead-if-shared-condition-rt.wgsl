// Two ifs on one condition, the first with nothing observable in it (P1-DXIL-061). DCE removes the first if; the
// condition must stay for the second, which reads nothing else of the invocation. A switch on the same condition,
// also with nothing observable, goes too. Invocations 0..3, c = li > 1: invocations 2 and 3 write dst[0] = 7.
// DXIL refused to compile this ("function argument 0 not in value map").
@group(0) @binding(0) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) li: u32) {
    let c = li > 1u;
    if c {
        var t = 1;
        t += 2;
    }
    switch u32(c) {
        case 1u: {
            var s = 3;
            s *= 2;
        }
        default: {}
    }
    if c {
        dst[0] = 7u;
    }
}
