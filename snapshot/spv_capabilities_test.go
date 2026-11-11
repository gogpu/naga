// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package snapshot_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogpu/naga/spirv"
)

// TestSpvCapabilityName pins the names the diagnostics print to the numbers of
// the SPIR-V specification (spirv.h), on the values the two removed tables got
// wrong. A number the specification does not define prints as itself.
func TestSpvCapabilityName(t *testing.T) {
	tests := []struct {
		value uint32
		name  string
	}{
		{25, "ImageGatherExtended"},
		{26, "26"}, // not a capability: the old tables named it StorageImageMultisample
		{27, "StorageImageMultisample"},
		{4423, "SubgroupBallotKHR"},
		{4433, "StorageBuffer16BitAccess"},
		{4437, "DeviceGroup"},
		{4479, "RayTracingKHR"},
		{5013, "StencilExportEXT"},
		{5016, "Int64ImageEXT"},
		{5283, "MeshShadingEXT"},
		{5302, "RuntimeDescriptorArray"},
		{6016, "DotProductInputAll"},
		{6033, "AtomicFloat32AddEXT"},
		{12345, "12345"},
	}
	for _, tt := range tests {
		if got := spvCapabilityName(tt.value); got != tt.name {
			t.Errorf("spvCapabilityName(%d) = %q, want %q", tt.value, got, tt.name)
		}
	}
}

// TestParseRustSPVConfigCapabilities checks a known capabilities parameter of
// a Rust TOML config, canonical and alias names alike.
func TestParseRustSPVConfigCapabilities(t *testing.T) {
	content := "[spv]\ncapabilities = [\"StorageBuffer16BitAccess\", \"StorageUniform16\"]\n"
	opts, err := parseRustSPVConfig(content, "shader.toml")
	if err != nil {
		t.Fatalf("parseRustSPVConfig: %v", err)
	}
	want := []spirv.Capability{4433, 4434} // spirv.h: StorageBuffer16BitAccess, UniformAndStorageBuffer16BitAccess
	if len(opts.AllowedCapabilities) != len(want) {
		t.Fatalf("AllowedCapabilities = %v, want %v", opts.AllowedCapabilities, want)
	}
	for _, capability := range want {
		if !opts.AllowedCapabilities.Has(capability) {
			t.Errorf("AllowedCapabilities = %v, missing %d", opts.AllowedCapabilities, capability)
		}
	}
}

// TestParseRustSPVConfigUnknownCapability proves an unknown capability name
// fails the read of its file, naming the file and the name, instead of being
// dropped: a dropped name leaves AllowedCapabilities unset, which allows every
// capability rather than the set Rust's parameters gave.
func TestParseRustSPVConfigUnknownCapability(t *testing.T) {
	content := "[spv]\ncapabilities = [\"MeshShadingEXT\", \"NotACapability\"]\n"
	opts, err := parseRustSPVConfig(content, "mesh-shader.toml")
	if err == nil {
		t.Fatalf("an unknown capability name was accepted; AllowedCapabilities = %v", opts.AllowedCapabilities)
	}
	for _, want := range []string{"mesh-shader.toml", "NotACapability"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestRustSPVCapabilitiesResolve checks that every capability name the Rust
// parameter files of the corpus give resolves to a capability. A name that did
// not would compile that shader with every capability allowed, so the corpus
// would prove nothing about it.
func TestRustSPVCapabilitiesResolve(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "rust", "in", "wgsl", "*.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no Rust parameter files found")
	}
	checked := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		inSPVSection := false
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "[") {
				inSPVSection = trimmed == "[spv]"
				continue
			}
			if !inSPVSection || !strings.HasPrefix(trimmed, "capabilities") {
				continue
			}
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) != 2 {
				t.Fatalf("%s: malformed capabilities line %q", path, trimmed)
			}
			caps, err := spvCapabilitiesFromList(parts[1])
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			checked += len(caps)
		}
	}
	if checked == 0 {
		t.Fatal("no capability names found in the Rust parameter files")
	}
	t.Logf("%d capabilities named by the Rust parameter files resolve", checked)
}

// TestRustSPVCapabilitiesOfMeshAndRayTracing checks the two names that were
// dropped silently before: the mesh-shader and ray-tracing-pipeline parameters
// now give the capabilities of the SPIR-V specification, not an empty set.
func TestRustSPVCapabilitiesOfMeshAndRayTracing(t *testing.T) {
	tests := []struct {
		shader string
		want   spirv.Capability
	}{
		{"mesh-shader", 5283},          // spirv.h: SpvCapabilityMeshShadingEXT
		{"ray-tracing-pipeline", 4479}, // spirv.h: SpvCapabilityRayTracingKHR
	}
	for _, tt := range tests {
		opts, err := readRustSPVConfig(tt.shader)
		if err != nil {
			t.Fatalf("%s: %v", tt.shader, err)
		}
		if !opts.AllowedCapabilities.Has(tt.want) {
			t.Errorf("%s: AllowedCapabilities = %v, want %d", tt.shader, opts.AllowedCapabilities, tt.want)
		}
	}
}
