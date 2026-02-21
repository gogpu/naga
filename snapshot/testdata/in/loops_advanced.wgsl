// Nested loops, break, continue, loop with continuing block.

fn nested_loops() -> i32 {
    var total: i32 = 0;
    for (var i: i32 = 0; i < 3; i = i + 1) {
        for (var j: i32 = 0; j < 3; j = j + 1) {
            total = total + i * 3 + j;
        }
    }
    return total;
}

fn loop_with_break() -> i32 {
    var sum: i32 = 0;
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        if i == 5 {
            i = i + 1;
            continue;
        }
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}

fn loop_with_continuing() -> i32 {
    var sum: i32 = 0;
    var i: i32 = 0;
    loop {
        if i >= 5 {
            break;
        }
        sum = sum + i;
        continuing {
            i = i + 1;
        }
    }
    return sum;
}

fn while_loop() -> i32 {
    var n: i32 = 100;
    while n > 1 {
        n = n / 2;
    }
    return n;
}

@compute @workgroup_size(1)
fn main() {
    let a = nested_loops();
    let b = loop_with_break();
    let c = loop_with_continuing();
    let d = while_loop();
}
