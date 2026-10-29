// `var x: u32;` in a loop body is a new zero-initialized variable on every
// iteration (WGSL: a variable without an initializer starts at its zero
// value). BUG-LOOP-VAR-REINIT-001: naga hoists it and never resets it, so
// the values accumulate.
@group(0) @binding(0) var<storage, read> count: array<u32, 1>;
@group(0) @binding(1) var<storage, read_write> result: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var i = 0u; i < count[0]; i++) {
        var x: u32;
        x += i + 1u;
        result[i] = x;
    }
}
