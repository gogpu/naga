// A for-loop counter initialized from a run-time value, with a run-time
// bound: neither the start nor the trip count is known to the compiler.
@group(0) @binding(0) var<storage, read> bounds: array<u32, 2>;
@group(0) @binding(1) var<storage, read_write> result: array<u32>;

@compute @workgroup_size(1)
fn main() {
    for (var j = bounds[0]; j < bounds[1]; j++) {
        result[j] = j * 10u;
    }
}
