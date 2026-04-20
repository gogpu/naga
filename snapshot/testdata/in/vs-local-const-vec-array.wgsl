// Regression fixture for BUG-DXIL-025: local const-init vector-element
// array indexed by dynamic vertex_index. The `gogpu/examples/triangle`
// embedded shader (gogpu/shader.go:coloredTriangleShaderSource) uses
// exactly this pattern and was the discovery point.
//
// Before the SROA fast path, emitArrayLocalVariable alloca'd
// `[3 x vec2<f32>]` and stored each literal into the alloca, but
// typeToDXIL flattened the type to `[3 x f32]` while the store/load
// paths still used stride-1 GEP indexing — the resulting LLVM 3.7
// bitcode carried malformed INST_STORE records that the dxil.dll
// bitcode reader rejected with HRESULT 0x80aa0009 'Invalid record'
// before any semantic validation ran.
//
// With the fix, the constant-init is detected at emitArrayLocalVariable
// time, the alloca is skipped entirely, and the indexed access is
// lowered to a per-lane select chain keyed on the dynamic index —
// mirroring what Mesa's nir_lower_indirect_derefs_to_if_else_trees and
// DXC's LLVM SROA + GVN passes would produce upstream.
@vertex
fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4<f32> {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(0.0, 0.5),    // top
        vec2<f32>(-0.5, -0.5),  // bottom-left
        vec2<f32>(0.5, -0.5)    // bottom-right
    );
    return vec4<f32>(positions[idx], 0.0, 1.0);
}
