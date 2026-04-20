// Package backend holds cross-backend constants and helpers shared between
// naga's text backends (HLSL) and binary backends (DXIL) that both target
// the D3D ecosystem. Centralizing these values here prevents the HLSL
// writer and the DXIL emitter from drifting — a drift that directly
// causes D3D12 pipeline-state creation failures at the input-layout
// boundary (see BUG-DXIL-028).
//
// Anything placed here must be:
//   - a spec-level output convention, not IR semantics
//   - referenced by at least two sibling backends that otherwise would
//     duplicate a literal
//   - stable enough that external tooling (wgpu/hal, gg, ui) can rely on it
package backend

// LocationSemantic is the semantic-name prefix for user-defined
// @location(N) vertex-shader inputs and inter-stage varyings in both
// HLSL source and DXIL input/output signatures.
//
// It is a naga ecosystem convention. DXIL does not prescribe a name for
// arbitrary (non-SV_*) semantics; the backends choose one and every
// consumer must agree.
//
// Current consumers (must match this value):
//   - naga/hlsl/types.go              — HLSL source emission
//   - naga/dxil/dxil.go               — PSV signature element name
//   - naga/dxil/internal/emit         — !dx.entryPoints signature name
//   - naga/dxil/internal/container    — ISG1/OSG1 SemanticMapping
//   - wgpu/hal/dx12/pipeline.go       — D3D12_INPUT_ELEMENT_DESC.SemanticName
//
// A mismatch between the DXIL input signature and the D3D12 input layout
// causes CreateGraphicsPipelineState to return E_INVALIDARG. IDxcValidator
// cannot detect this because the DXIL container is internally consistent;
// the break only surfaces at the container-to-pipeline boundary.
//
// See BUG-DXIL-028 for the discovery trail.
const LocationSemantic = "LOC"
