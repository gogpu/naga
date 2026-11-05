// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import "github.com/gogpu/naga/valid"

// SupportedCapabilities returns the optional capabilities the GLSL backend can
// write. Compile refuses a module that needs a capability outside this set
// (valid.MissingCapabilities), with an error that wraps backend.ErrUnsupported
// and names the capability as Rust prints it, "Capabilities(SHADER_FLOAT32_ATOMIC)".
// Pass it as valid.Options.Capabilities, intersected with what the device
// offers, to validate a module for this backend.
//
// The set is Rust naga's supported_capabilities
// (back/glsl/mod.rs:481). Rust lists some core WebGPU features
// too (IMMEDIATES, TEXTURE_EXTERNAL, MULTISAMPLED_SHADING, CUBE_ARRAY_TEXTURES,
// LINEAR_INTERPOLATION, SHADER_FLOAT16_IN_FLOAT32) and capabilities whose IR
// is not gated yet (the *_NON_UNIFORM_INDEXING ones, COOPERATIVE_MATRIX,
// RAY_HIT_VERTEX_POSITION, RAY_TRACING_PIPELINE, MEMORY_DECORATION_*,
// DEBUG_PRINTF); package valid has no bit for them
// (ADR-010 N10, "arrive with the IR they gate"), so they are not listed.
func SupportedCapabilities() valid.Capabilities {
	return valid.Capabilities(0) |
		valid.CapFloat64 |
		valid.CapPrimitiveIndex |
		valid.CapClipDistances |
		valid.CapMultiview |
		valid.CapEarlyDepthTest |
		valid.CapDualSourceBlending |
		valid.CapShaderInt64 |
		valid.CapShaderInt64AtomicAllOps |
		valid.CapTextureAtomic |
		valid.CapTextureInt64Atomic |
		valid.CapSubgroup |
		valid.CapSubgroupBarrier |
		valid.CapShaderFloat16 |
		valid.CapShaderInt16 |
		valid.CapShaderBarycentrics |
		valid.CapDrawIndex |
		valid.CapStorageTexture16BitNormFormats
}
