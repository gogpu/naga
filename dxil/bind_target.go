// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

// BindingLocation identifies a resource in the source shader.
// It maps a WGSL @group/@binding pair (equivalently, a SPIR-V
// DescriptorSet/Binding pair) to the corresponding DXIL register binding
// via BindingMap.
type BindingLocation struct {
	// Group corresponds to WGSL @group or SPIR-V DescriptorSet.
	Group uint32

	// Binding corresponds to WGSL @binding or SPIR-V Binding.
	Binding uint32
}

// BindTarget specifies the DXIL register binding for a resource.
// DXIL uses (space, register) just like HLSL — a resource declared at
// (space=S, lowerBound=R) in DXIL metadata corresponds to HLSL
// register(tR, spaceS) / (uR, spaceS) / (bR, spaceS) depending on class.
//
// The Register field is both the DXIL metadata "lowerBound" field and the
// dx.op.createHandle "index" argument for non-array resources (they must
// match — createHandle.index is the absolute register number, not an
// offset within the range).
type BindTarget struct {
	// Space is the register space (0-based).
	Space uint32

	// Register is the register index within the space. This is the
	// DXIL metadata "lowerBound" and, for non-array resources, also
	// the dx.op.createHandle index.
	Register uint32

	// BindingArraySize is the array size for binding arrays. If nil,
	// the resource is not an array.
	//
	// Note: the DXIL backend takes the actual array size from the IR
	// type (BindingArrayType.Size). This field is accepted for parity
	// with hlsl.BindTarget so callers can share a single binding-map
	// builder; it is currently not consulted by the DXIL backend.
	BindingArraySize *uint32
}

// BindingMap maps WGSL/SPIR-V binding locations to DXIL register bindings.
//
// The DXIL backend consults this map in analyzeResources(): for each
// global variable with a (group, binding) pair present in the map, the
// raw WGSL numbers are replaced with the mapped (space, register) before
// being written into DXIL resource metadata and dx.op.createHandle calls.
// Bindings not present in the map keep their raw WGSL numbers, preserving
// backward compatibility when the map is nil.
//
// This is the DXIL analog of hlsl.Options.BindingMap. It is required
// when the shader is consumed by a pipeline whose root signature uses a
// different register scheme than the raw WGSL binding numbers — notably
// wgpu/hal/dx12, which assigns registers via monotonic per-class counters
// (SRV=t0,t1,... / UAV=u0,u1,... / CBV=b0,b1,...).
type BindingMap map[BindingLocation]BindTarget

// SamplerHeapBindTargets specifies binding targets for the synthesized
// sampler heap arrays (nagaSamplerHeap / nagaComparisonSamplerHeap).
// Mirrors hlsl.SamplerHeapBindTargets.
//
// The D3D12 root signature allocates sampler heap arrays at specific
// (space, register) positions; these targets must match or the pipeline
// state object creation will fail with E_INVALIDARG.
type SamplerHeapBindTargets struct {
	// StandardSamplers is the binding for non-comparison samplers.
	// Default: (space=0, register=0).
	StandardSamplers BindTarget

	// ComparisonSamplers is the binding for comparison samplers.
	// Default: (space=1, register=0).
	ComparisonSamplers BindTarget
}
