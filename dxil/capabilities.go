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
// CLIP_DISTANCES is listed for arrays of 1 to 4; checkSupported refuses 5 to
// 8, which need a second SV_ClipDistance signature element (P1-DXIL-082).
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
	if err := checkClipDistances(irModule, ep); err != nil {
		return err
	}
	// The container holds only what the entry point reaches; irModule holds
	// that entry point alone (entrypoint.Select in checkOptions).
	return unsupported.OutputCapabilities("dxil", irModule, unsupported.FilteredOutput(irModule, 0), SupportedCapabilities())
}

// maxClipDistancesPerElement is the most clip distances one SV_ClipDistance
// signature element holds: one row of four components.
const maxClipDistancesPerElement = 4

// checkClipDistances refuses a clip_distances output of more than four
// elements. WGSL allows up to 8; D3D then needs a second SV_ClipDistance
// signature element (rows 0 and 1, as DXC emits for float4 + float4), which
// the emitter does not write yet (P1-DXIL-082). Without the refusal the
// container fails IDxcValidator with "Not all elements of output
// SV_ClipDistance were written".
func checkClipDistances(irModule *ir.Module, ep *ir.EntryPoint) error {
	res := ep.Function.Result
	if res == nil {
		return nil
	}
	check := func(b *ir.Binding, ty ir.TypeHandle) error {
		if b == nil || int(ty) >= len(irModule.Types) {
			return nil
		}
		if bi, ok := (*b).(ir.BuiltinBinding); !ok || bi.Builtin != ir.BuiltinClipDistances {
			return nil
		}
		arr, ok := irModule.Types[ty].Inner.(ir.ArrayType)
		if !ok || arr.Size.Constant == nil || *arr.Size.Constant <= maxClipDistancesPerElement {
			return nil
		}
		return unsupported.Errorf("dxil: entry point %q: @builtin(clip_distances) array<f32, %d> is not supported (up to %d)",
			ep.Name, *arr.Size.Constant, maxClipDistancesPerElement)
	}
	if err := check(res.Binding, res.Type); err != nil {
		return err
	}
	if int(res.Type) < len(irModule.Types) {
		if st, ok := irModule.Types[res.Type].Inner.(ir.StructType); ok {
			for i := range st.Members {
				if err := check(st.Members[i].Binding, st.Members[i].Type); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
