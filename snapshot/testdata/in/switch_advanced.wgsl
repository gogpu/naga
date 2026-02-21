// Switch with multiple case selectors, default.

fn classify(x: i32) -> i32 {
    var result: i32;
    switch x {
        case 0: {
            result = 0;
        }
        case 1, 2: {
            result = 1;
        }
        case 3, 4, 5: {
            result = 2;
        }
        default: {
            result = -1;
        }
    }
    return result;
}

fn switch_with_return(x: u32) -> u32 {
    switch x {
        case 0u: {
            return 100u;
        }
        case 1u: {
            return 200u;
        }
        default: {
            return 0u;
        }
    }
}

fn nested_switch(a: i32, b: i32) -> i32 {
    var result: i32 = 0;
    switch a {
        case 1: {
            switch b {
                case 10: {
                    result = 110;
                }
                default: {
                    result = 100;
                }
            }
        }
        default: {
            result = 0;
        }
    }
    return result;
}

@compute @workgroup_size(1)
fn main() {
    let a = classify(3);
    let b = switch_with_return(1u);
    let c = nested_switch(1, 10);
}
