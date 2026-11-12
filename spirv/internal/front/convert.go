package front

import "github.com/gogpu/naga/spirv/internal/spvenums"

// supportedCapability reports whether this package implements the capability,
// as Rust naga's SUPPORTED_CAPABILITIES.
func supportedCapability(c spvenums.Capability) bool {
	switch c {
	case spvenums.CapabilityShader,
		spvenums.CapabilityVulkanMemoryModel,
		spvenums.CapabilityClipDistance,
		spvenums.CapabilityCullDistance,
		spvenums.CapabilitySampleRateShading,
		spvenums.CapabilityDerivativeControl,
		spvenums.CapabilityMatrix,
		spvenums.CapabilityImageQuery,
		spvenums.CapabilitySampled1D,
		spvenums.CapabilityImage1D,
		spvenums.CapabilitySampledCubeArray,
		spvenums.CapabilityImageCubeArray,
		spvenums.CapabilityStorageImageExtendedFormats,
		spvenums.CapabilityInt8,
		spvenums.CapabilityInt16,
		spvenums.CapabilityInt64,
		spvenums.CapabilityInt64Atomics,
		spvenums.CapabilityFloat16,
		spvenums.CapabilityAtomicFloat32AddEXT,
		spvenums.CapabilityFloat64,
		spvenums.CapabilityGeometry,
		spvenums.CapabilityMultiView,
		spvenums.CapabilityStorageBuffer16BitAccess,
		spvenums.CapabilityUniformAndStorageBuffer16BitAccess,
		spvenums.CapabilityGroupNonUniform,
		spvenums.CapabilityGroupNonUniformVote,
		spvenums.CapabilityGroupNonUniformArithmetic,
		spvenums.CapabilityGroupNonUniformBallot,
		spvenums.CapabilityGroupNonUniformShuffle,
		spvenums.CapabilityGroupNonUniformShuffleRelative,
		spvenums.CapabilityRuntimeDescriptorArray,
		spvenums.CapabilityStorageImageMultisample,
		spvenums.CapabilityFragmentBarycentricKHR,
		// The two array-indexing capabilities look like they should be
		// required, but they are "tricky ones" in Rust naga: the indexing is
		// read from the module's types and the capability is not enforced.
		spvenums.CapabilityUniformBufferArrayDynamicIndexing,
		spvenums.CapabilityStorageBufferArrayDynamicIndexing:
		return true
	}
	return false
}

// supportedExtension reports whether this package implements the extension,
// as Rust naga's SUPPORTED_EXTENSIONS.
func supportedExtension(name string) bool {
	switch name {
	case "SPV_KHR_storage_buffer_storage_class",
		"SPV_KHR_vulkan_memory_model",
		"SPV_KHR_multiview",
		"SPV_EXT_descriptor_indexing",
		"SPV_EXT_shader_atomic_float_add",
		"SPV_KHR_16bit_storage",
		"SPV_KHR_non_semantic_info",
		"SPV_KHR_fragment_shader_barycentric":
		return true
	}
	return false
}
