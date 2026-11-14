// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package msl

import "github.com/gogpu/naga/valid"

// SupportedCapabilities returns the optional capabilities the MSL backend can
// write. Compile refuses a module that needs a capability outside this set
// (valid.MissingCapabilities), with an error that wraps backend.ErrUnsupported
// and names the capability as Rust prints it, "Capabilities(SHADER_FLOAT32_ATOMIC)".
// Pass it as valid.Options.Capabilities, intersected with what the device
// offers, to validate a module for this backend.
//
// The set is Rust naga's supported_capabilities
// (back/msl/mod.rs:836). Rust lists some core WebGPU features
// too (IMMEDIATES, TEXTURE_EXTERNAL, MULTISAMPLED_SHADING, CUBE_ARRAY_TEXTURES,
// LINEAR_INTERPOLATION, SHADER_FLOAT16_IN_FLOAT32) and capabilities whose IR
// is not gated yet (the *_NON_UNIFORM_INDEXING ones, COOPERATIVE_MATRIX,
// RAY_HIT_VERTEX_POSITION, RAY_TRACING_PIPELINE, DEBUG_PRINTF); package valid
// has no bit for them (ADR-010 N10, "arrive with the IR they gate"), so they
// are not listed.
func SupportedCapabilities() valid.Capabilities {
	return valid.Capabilities(0) |
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
		valid.CapShaderFloat32Atomic |
		valid.CapTextureAtomic |
		valid.CapTextureInt64Atomic |
		valid.CapShaderFloat16 |
		valid.CapShaderInt16 |
		valid.CapShaderBarycentrics |
		valid.CapMeshShader |
		valid.CapMeshShaderPointTopology |
		valid.CapPerVertex |
		// No MEMORY_DECORATION_VOLATILE, as Rust's list (back/msl/mod.rs:879):
		// Metal has no volatile, so such a module is refused.
		valid.CapMemoryDecorationCoherent
}
