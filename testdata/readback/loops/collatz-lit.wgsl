// collatz-rt with literal arguments: a compiler that inlines the function
// sees constant inputs and may fold the loops.
@group(0) @binding(0) var<storage, read_write> result: array<u32>;

fn collatz_iterations(n_base: u32) -> u32 {
    var n = n_base;
    var i: u32 = 0u;
    while n > 1u {
        if n % 2u == 0u {
            n = n / 2u;
        }
        else {
            n = 3u * n + 1u;
        }
        i = i + 1u;
    }
    return i;
}

@compute @workgroup_size(1)
fn main() {
    result[0] = collatz_iterations(1u);
    result[1] = collatz_iterations(2u);
    result[2] = collatz_iterations(3u);
    result[3] = collatz_iterations(4u);
    result[4] = collatz_iterations(7u);
    result[5] = collatz_iterations(27u);
}
