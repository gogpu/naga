// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package hlsl

// BindTarget specifies the HLSL register binding for a resource.
// HLSL uses register(x#, space#) syntax for resource binding.
type BindTarget struct {
	// Space is the register space (0-based).
	// Spaces allow multiple resources to use the same register index.
	Space uint8

	// Register is the register index within the space.
	Register uint32

	// BindingArraySize is the array size for binding arrays.
	// If nil, the resource is not an array.
	BindingArraySize *uint32

	// DynamicStorageBufferOffsetsIndex is the index into the dynamic buffer offsets
	// constant buffer for this binding. When set, the generated HLSL adds the
	// dynamic offset to ByteAddressBuffer Load/Store addresses.
	// This is the index in the buffer at Options.DynamicStorageBufferOffsetsTargets.
	// Matches Rust naga's BindTarget::dynamic_storage_buffer_offsets_index.
	DynamicStorageBufferOffsetsIndex *uint32

	// RestrictIndexing indicates that this specific binding should have bounds
	// checking applied to array indices, even for Uniform address space.
	// Matches Rust naga's BindTarget::restrict_indexing.
	RestrictIndexing bool
}

// OffsetsBindTarget specifies the HLSL register binding for a dynamic buffer
// offsets constant buffer. Each group of dynamic storage buffers gets one of these.
// Matches Rust naga's OffsetsBindTarget.
type OffsetsBindTarget struct {
	Space    uint8
	Register uint32
	Size     uint32
}

// SamplerHeapBindTargets specifies bind targets for sampler heaps.
// Used with SM 6.6+ dynamic resources.
type SamplerHeapBindTargets struct {
	// StandardSamplers is the binding for non-comparison samplers.
	StandardSamplers BindTarget

	// ComparisonSamplers is the binding for comparison samplers.
	ComparisonSamplers BindTarget
}

// ResourceBinding identifies a resource in the source shader.
// This maps WGSL/SPIR-V binding points to HLSL registers.
type ResourceBinding struct {
	// Group corresponds to WGSL @group or SPIR-V DescriptorSet.
	Group uint32

	// Binding corresponds to WGSL @binding or SPIR-V Binding.
	Binding uint32
}

// RegisterType represents the HLSL register type.
type RegisterType uint8

const (
	// RegisterTypeB is for constant buffers (cbuffer).
	RegisterTypeB RegisterType = iota

	// RegisterTypeT is for textures and shader resource views.
	RegisterTypeT

	// RegisterTypeS is for samplers.
	RegisterTypeS

	// RegisterTypeU is for unordered access views (UAV).
	RegisterTypeU
)

// String returns the single-character register prefix.
func (rt RegisterType) String() string {
	switch rt {
	case RegisterTypeB:
		return "b"
	case RegisterTypeT:
		return "t"
	case RegisterTypeS:
		return "s"
	case RegisterTypeU:
		return "u"
	default:
		return "b"
	}
}

// DefaultBindTarget returns a BindTarget with default values.
// Defaults to space 0, register 0, no array.
func DefaultBindTarget() BindTarget {
	return BindTarget{
		Space:            0,
		Register:         0,
		BindingArraySize: nil,
	}
}

// WithSpace returns a copy of the BindTarget with the specified space.
func (bt BindTarget) WithSpace(space uint8) BindTarget {
	bt.Space = space
	return bt
}

// WithRegister returns a copy of the BindTarget with the specified register.
func (bt BindTarget) WithRegister(register uint32) BindTarget {
	bt.Register = register
	return bt
}

// WithArraySize returns a copy of the BindTarget with the specified array size.
func (bt BindTarget) WithArraySize(size uint32) BindTarget {
	bt.BindingArraySize = &size
	return bt
}

// ExternalTextureBindTarget specifies HLSL binding information for an external
// texture global variable. External textures are decomposed into 3 plane
// textures and a parameters cbuffer.
// Matches Rust naga's ExternalTextureBindTarget.
type ExternalTextureBindTarget struct {
	// Planes contains the bind targets for the 3 plane textures.
	Planes [3]BindTarget
	// Params is the bind target for the parameters cbuffer.
	Params BindTarget
}

// ExternalTextureBindingMap maps resource bindings to external texture bind targets.
type ExternalTextureBindingMap map[ResourceBinding]ExternalTextureBindTarget
