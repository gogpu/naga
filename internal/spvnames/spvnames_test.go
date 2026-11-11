// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package spvnames

import "testing"

// TestCapabilityName pins names to the numbers of the SPIR-V specification
// (spirv.h) across the ranges the two hand-kept tables got wrong: 26 is not a
// capability at all, and the 44xx, 50xx and 60xx capabilities were off by
// whole ranges.
func TestCapabilityName(t *testing.T) {
	tests := []struct {
		value uint32
		name  string
	}{
		{0, "Matrix"},
		{1, "Shader"},
		{25, "ImageGatherExtended"},
		{27, "StorageImageMultisample"},
		{66, "GroupNonUniformShuffleRelative"},
		{68, "GroupNonUniformQuad"},
		{4423, "SubgroupBallotKHR"},
		{4427, "DrawParameters"},
		{4433, "StorageBuffer16BitAccess"},
		{4434, "UniformAndStorageBuffer16BitAccess"},
		{4437, "DeviceGroup"},
		{4439, "MultiView"},
		{4447, "SampleMaskPostDepthCoverage"},
		{4472, "RayQueryKHR"},
		{4479, "RayTracingKHR"},
		{5013, "StencilExportEXT"},
		{5016, "Int64ImageEXT"},
		{5283, "MeshShadingEXT"},
		{5284, "FragmentBarycentricKHR"},
		{5301, "ShaderNonUniform"},
		{5302, "RuntimeDescriptorArray"},
		{5379, "DemoteToHelperInvocation"},
		{6016, "DotProductInputAll"},
		{6017, "DotProductInput4x8Bit"},
		{6018, "DotProductInput4x8BitPacked"},
		{6019, "DotProduct"},
		{6033, "AtomicFloat32AddEXT"},
	}
	for _, tt := range tests {
		got, ok := CapabilityName(tt.value)
		if !ok || got != tt.name {
			t.Errorf("CapabilityName(%d) = %q, %v; want %q, true", tt.value, got, ok, tt.name)
		}
	}
}

// TestCapabilityNameUndefined checks that a number the specification leaves
// unused reports no name instead of borrowing the next capability's.
func TestCapabilityNameUndefined(t *testing.T) {
	// 26 falls between ImageGatherExtended (25) and
	// StorageImageMultisample (27); the old tables named 26
	// StorageImageMultisample and shifted everything above it by one.
	for _, value := range []uint32{16, 26} {
		if name, ok := CapabilityName(value); ok {
			t.Errorf("CapabilityName(%d) = %q, true; want no capability", value, name)
		}
	}
}

// TestCapabilityByName checks the name-to-number direction, canonical names
// and aliases alike, and an unknown name.
func TestCapabilityByName(t *testing.T) {
	tests := []struct {
		name  string
		value uint32
	}{
		{"Matrix", 0},
		{"StorageImageMultisample", 27},
		{"StorageBuffer16BitAccess", 4433},
		{"StorageUniformBufferBlock16", 4433}, // alias of StorageBuffer16BitAccess
		{"UniformAndStorageBuffer16BitAccess", 4434},
		{"StorageUniform16", 4434}, // alias of UniformAndStorageBuffer16BitAccess
		{"RayTracingKHR", 4479},
		{"MeshShadingEXT", 5283},
		{"ShaderNonUniformEXT", 5301}, // alias of ShaderNonUniform
		{"DotProductKHR", 6019},       // alias of DotProduct
	}
	for _, tt := range tests {
		got, ok := CapabilityByName(tt.name)
		if !ok || got != tt.value {
			t.Errorf("CapabilityByName(%q) = %d, %v; want %d, true", tt.name, got, ok, tt.value)
		}
	}
	for _, name := range []string{"", "NotACapability", "storagebuffers"} {
		if value, ok := CapabilityByName(name); ok {
			t.Errorf("CapabilityByName(%q) = %d, true; want no capability", name, value)
		}
	}
}

// TestCapabilityTableRoundTrip checks the table itself: one entry per number,
// no name given to two capabilities, every canonical name and alias resolving
// to its own number.
func TestCapabilityTableRoundTrip(t *testing.T) {
	if len(capabilityTable) == 0 {
		t.Fatal("the capability table is empty")
	}
	seenValue := make(map[uint32]string, len(capabilityTable))
	seenName := make(map[string]uint32, len(capabilityTable))
	for _, c := range capabilityTable {
		if prev, ok := seenValue[c.value]; ok {
			t.Errorf("%d is both %q and %q", c.value, prev, c.name)
		}
		seenValue[c.value] = c.name
		for _, name := range append([]string{c.name}, c.aliases...) {
			if prev, ok := seenName[name]; ok {
				t.Errorf("%q is used for both %d and %d", name, prev, c.value)
			}
			seenName[name] = c.value
		}

		name, ok := CapabilityName(c.value)
		if !ok || name != c.name {
			t.Errorf("CapabilityName(%d) = %q, %v; want %q", c.value, name, ok, c.name)
		}
		if value, ok := CapabilityByName(c.name); !ok || value != c.value {
			t.Errorf("CapabilityByName(%q) = %d, %v; want %d", c.name, value, ok, c.value)
		}
		for _, alias := range c.aliases {
			if value, ok := CapabilityByName(alias); !ok || value != c.value {
				t.Errorf("CapabilityByName(%q) = %d, %v; want %d", alias, value, ok, c.value)
			}
		}
	}
}
