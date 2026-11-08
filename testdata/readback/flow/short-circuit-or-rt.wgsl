// A short-circuit `||` chain over runtime values decides an if/else. The chain's bool local meets at two if
// merges: as `phi i1` WARP, the D3D12 software adapter of GPU-less CI runners, ran neither arm's stores, and
// as `alloca i1` D3D12's GPU-based validation dropped them (P2-DXIL-107, P2-DXIL-092). Four invocations,
// li = 0..3, three words of src each: li 0 is (10, 20, 30) → x + y + z = 60; li 1 is (70000, 20, 30),
// li 2 is (10, 70000, 30), li 3 is (10, 20, 70000) → 0. li 1 is the case that failed: the chain
// short-circuits on its first term and its value comes from a constant, not from an instruction.
@group(0) @binding(0) var<storage, read> src: array<u32>;
@group(0) @binding(1) var<storage, read_write> dst: array<u32>;

@compute @workgroup_size(4)
fn main(@builtin(local_invocation_index) li: u32) {
    let x = src[li * 3u];
    let y = src[li * 3u + 1u];
    let z = src[li * 3u + 2u];

    let limit = 65535u;
    if (x > limit || y > limit || z > limit) {
        dst[li] = 0u;
    } else {
        dst[li] = x + y + z;
    }
}
