fn test_if(x: i32) -> i32 {
    if x > 0 {
        return 1;
    } else if x < 0 {
        return -1;
    } else {
        return 0;
    }
}

fn test_loop() -> i32 {
    var sum: i32 = 0;
    var i: i32 = 0;
    loop {
        if i >= 10 {
            break;
        }
        sum = sum + i;
        i = i + 1;
    }
    return sum;
}

fn test_for() -> i32 {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 5; i = i + 1) {
        if i == 3 {
            continue;
        }
        sum = sum + i;
    }
    return sum;
}

fn test_while() -> i32 {
    var n: i32 = 10;
    while n > 0 {
        n = n - 1;
    }
    return n;
}

fn test_switch(x: i32) -> i32 {
    var result: i32;
    switch x {
        case 1: {
            result = 10;
        }
        case 2, 3: {
            result = 20;
        }
        default: {
            result = 0;
        }
    }
    return result;
}

@compute @workgroup_size(1)
fn main() {
    let a = test_if(1);
    let b = test_loop();
    let c = test_for();
    let d = test_while();
    let e = test_switch(2);
}
