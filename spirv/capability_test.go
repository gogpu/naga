// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package spirv

import "testing"

// TestCapabilityValues pins capability numbers to the SPIR-V specification
// (spirv.h in the SPIR-V headers). A wrong number declares a different
// capability than the module uses, and nothing else in the pipeline notices.
func TestCapabilityValues(t *testing.T) {
	tests := []struct {
		name string
		got  Capability
		want uint32
	}{
		{"StorageImageWriteWithoutFormat", CapabilityStorageImageWriteWithoutFormat, 56},
		{"DrawParameters", CapabilityDrawParameters, 4427},
		{"RayQueryKHR", CapabilityRayQueryKHR, 4472},
		{"DotProductInputAll", CapabilityDotProductInputAll, 6016},
		{"DotProductInput4x8Bit", CapabilityDotProductInput4x8Bit, 6017},
	}
	for _, tt := range tests {
		if uint32(tt.got) != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, uint32(tt.got), tt.want)
		}
	}
}

func TestCapabilitySet(t *testing.T) {
	set := NewCapabilitySet(CapabilityShader, CapabilityDotProduct)
	if !set.Has(CapabilityShader) || !set.Has(CapabilityDotProduct) {
		t.Errorf("NewCapabilitySet is missing a capability: %v", set)
	}
	if set.Has(CapabilityFloat16) {
		t.Error("Has(Float16) = true in a set without it")
	}
	var nilSet CapabilitySet
	if nilSet.Has(CapabilityShader) {
		t.Error("a nil CapabilitySet holds Shader")
	}
}

func TestToCodegenCapabilitySetKeepsNil(t *testing.T) {
	if got := toCodegenCapabilitySet(nil); got != nil {
		t.Errorf("nil set converts to %v, want nil: nil AllowedCapabilities allows every capability", got)
	}
	if got := toCodegenCapabilitySet(CapabilitySet{}); got == nil || len(got) != 0 {
		t.Errorf("empty set converts to %v, want an empty non-nil map", got)
	}
	if got := toCodegenCapabilitySet(NewCapabilitySet(CapabilityDotProduct)); len(got) != 1 {
		t.Errorf("one capability converts to %d entries, want 1", len(got))
	}
}
