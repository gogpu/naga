// A function's variables start afresh at every call, also when the call sits in a loop and the backend inlines the
// function. src = [4]: four calls of each helper. Expected:
//   zero_init(i):  var t: u32; t += i; t += 1:                1, 2, 3, 4
//   const_init(i): var t = 5u; t += i:                        5, 6, 7, 8
//   loop_acc(2):   var sum = 0u; sum += k + 1 for k < 2:      3, 3, 3, 3
//   branch(j):     var best: u32; if j > 1 { best = j; },
//                  j = 3, 2, 1, 0:                             3, 2, 0, 0
// DXIL: the inliner copied a helper's locals into the caller with their initializer, which a caller's local gets once,
// at entry, so a call in a loop saw the value the previous call left. loop_acc gave 3, 6, 9, 12 and branch 3, 2, 2, 2;
// zero_init and const_init were right only while mem2reg promoted their locals from the initial value, and gave
// 1, 3, 6, 10 and 5, 6, 8, 11 once the P0-DXIL-046 fix kept such locals in memory.
@group(0) @binding(0) var<storage, read> src: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

fn zero_init(i: u32) -> u32 {
    var t: u32;
    t += i;
    t += 1u;
    return t;
}

fn const_init(i: u32) -> u32 {
    var t = 5u;
    t += i;
    return t;
}

fn loop_acc(n: u32) -> u32 {
    var sum = 0u;
    for (var k = 0u; k < n; k++) {
        sum += k + 1u;
    }
    return sum;
}

fn branch(j: u32) -> u32 {
    var best: u32;
    if j > 1u {
        best = j;
    }
    return best;
}

@compute @workgroup_size(1)
fn main() {
    let n = src[0];
    for (var i = 0u; i < n; i++) {
        dst[i] = zero_init(i);
        dst[4u + i] = const_init(i);
        dst[8u + i] = loop_acc(2u);
        dst[12u + i] = branch(n - 1u - i);
    }
}
