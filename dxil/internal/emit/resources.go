package emit

// Resource handling for DXIL emission.
//
// This file handles CBV (constant buffer views), SRV (shader resource views),
// and sampler bindings via dx.op intrinsics.
//
// Phase 1 scope: CBV via dx.op.createHandle + dx.op.cbufferLoadLegacy.
// Texture sampling and UAVs are deferred to Phase 2.
//
// Reference: Mesa nir_to_dxil.c emit_resources()
//
// NOTE: This file is a placeholder for Phase 1.5/2. The current
// implementation handles the case where shaders have NO resource
// bindings (pure vertex transform + fragment color output).
// When resource bindings are needed (uniform buffers, textures),
// this will be extended with:
//   - dx.op.createHandle(57) for binding resources
//   - dx.op.cbufferLoadLegacy(59) for reading uniform buffers
//   - dx.op.sample(60) for texture sampling
//   - !dx.resources metadata for resource declarations
