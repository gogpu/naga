// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"github.com/gogpu/naga/internal/unsupported"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// SupportedCapabilities returns the optional capabilities the DXIL backend
// can write. Compile refuses a module that needs a capability outside this
// set (valid.MissingCapabilities), with an error that wraps
// backend.ErrUnsupported and names the capability as Rust prints it,
// "Capabilities(SHADER_FLOAT32_ATOMIC)".
//
// Rust naga has no DXIL backend, so this list is ours: what the emitter
// writes today, taken from what dxil.Compile accepts for a shader of each
// capability (the checks in this package that refuse a built-in or a type
// outright, and the emitter's coverage of the rest). What it leaves out:
//   - SHADER_BARYCENTRICS, DRAW_INDEX, PER_VERTEX and SHADER_INT16: refused
//     by checkBuiltins, checkLocation and checkInt16 below;
//   - SHADER_FLOAT32_ATOMIC: DXIL has no f32 atomic add, and the emitter's
//     output for one is not valid (P2-API-005);
//   - BUFFER_BINDING_ARRAY (a binding array of uniform buffers does not emit)
//     and ACCELERATION_STRUCTURE_BINDING_ARRAY (never written);
//   - SUBGROUP_VERTEX_STAGE: subgroup operations in a vertex shader do not
//     emit yet.
//
// EARLY_DEPTH_TEST is listed for its conservative form
// (@early_depth_test(greater_equal | less_equal | unchanged)), which the
// emitter writes as SV_DepthGreaterEqual / SV_DepthLessEqual; the forced form
// is refused by checkSupported until the emitter sets ForceEarlyDepthStencil
// (P2-DXIL-100).
//
// The capabilities package valid has no bit for (IMMEDIATES, TEXTURE_EXTERNAL and the other
// core or ungated capabilities) are not listed (ADR-010 N10).
func SupportedCapabilities() valid.Capabilities {
	return valid.CapFloat64 |
		valid.CapEarlyDepthTest |
		valid.CapPrimitiveIndex |
		valid.CapTextureAndSamplerBindingArray |
		valid.CapStorageTextureBindingArray |
		valid.CapStorageBufferBindingArray |
		valid.CapClipDistances |
		valid.CapStorageTexture16BitNormFormats |
		valid.CapMultiview |
		valid.CapRayQuery |
		valid.CapDualSourceBlending |
		valid.CapShaderInt64 |
		valid.CapSubgroup |
		valid.CapSubgroupBarrier |
		valid.CapShaderInt64AtomicMinMax |
		valid.CapShaderInt64AtomicAllOps |
		valid.CapTextureAtomic |
		valid.CapTextureInt64Atomic |
		valid.CapShaderFloat16 |
		valid.CapMeshShader |
		valid.CapMeshShaderPointTopology
}

// checkSupported refuses what the entry point uses that the emitter cannot
// write: a built-in or interpolation checkBuiltins names, first, so that its
// message stays specific, then a capability outside SupportedCapabilities.
func checkSupported(irModule *ir.Module, ep *ir.EntryPoint) error {
	if err := checkBuiltins(irModule, ep); err != nil {
		return err
	}
	// The emitter does not set ForceEarlyDepthStencil yet; writing the module
	// without it would drop the forced test silently (P2-DXIL-100).
	if _, ok := ep.EarlyDepthTest.(ir.EarlyDepthForce); ok {
		return unsupported.Errorf("dxil: entry point %q: @early_depth_test(force) is not supported", ep.Name)
	}
	// The container holds only what the entry point reaches; irModule holds
	// that entry point alone (entrypoint.Select in checkOptions).
	return unsupported.OutputCapabilities("dxil", irModule, unsupported.FilteredOutput(irModule, 0), SupportedCapabilities())
}
