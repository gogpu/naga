fn arithmetic() {
    let one_i = 1i;
    let one_u = 1u;
    let one_f = 1.0;
    let two_i = 2i;
    let two_u = 2u;
    let two_f = 2.0;

    // unary
    let neg0 = -one_f;
    let neg1 = -vec2(one_i);

    // binary
    let add0 = two_i + one_i;
    let add1 = two_u + one_u;
    let add2 = two_f + one_f;
    let sub0 = two_i - one_i;
    let mul0 = two_f * one_f;
    let div0 = two_i / one_i;
    let rem0 = two_u % one_u;
}

fn comparison() {
    let a = 1i;
    let b = 2i;

    let eq = a == b;
    let neq = a != b;
    let lt = a < b;
    let lte = a <= b;
    let gt = a > b;
    let gte = a >= b;
}

fn bitwise() {
    let a = 1i;
    let b = 2i;

    let or0 = a | b;
    let and0 = a & b;
    let xor0 = a ^ b;
    let flip0 = ~a;
    let shl0 = a << 1u;
    let shr0 = b >> 1u;
}

@compute @workgroup_size(1)
fn main() {
    arithmetic();
    comparison();
    bitwise();
}
