// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/wgsl"
)

// Below SPIR-V 1.3 a subgroup barrier is refused like every other subgroup
// operation: Subgroup scope needs the subgroup capabilities
// (VUID-StandaloneSpirv-SubgroupVoteKHR-07951 at SPIR-V 1.0).
func TestSubgroupBarrierNeedsSPIRV13(t *testing.T) {
	module, _, err := wgsl.Parse(`
enable subgroups;
@compute @workgroup_size(1)
fn main() {
    subgroupBarrier();
}
`, wgsl.ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewBackend(DefaultOptions()).Compile(module)
	var vErr *backend.VersionError
	if !errors.As(err, &vErr) || vErr.Space != backend.VersionSPIRV || vErr.Required != (backend.Version{Major: 1, Minor: 3}) {
		t.Fatalf("SPIR-V %d.%d: err = %v, want a SPIR-V VersionError requiring 1.3",
			DefaultOptions().Version.Major, DefaultOptions().Version.Minor, err)
	}
	if want := "a subgroup barrier"; vErr.Feature != want {
		t.Errorf("Feature = %q, want %q", vErr.Feature, want)
	}
}

// barrierSemantics returns the memory semantics constant of every
// OpControlBarrier (operand 3) and OpMemoryBarrier (operand 2) of spv.
func barrierSemantics(t *testing.T, spv []byte) (control, memory []uint32) {
	t.Helper()
	instrs := decodeSPIRVInstructions(spv)
	consts := map[uint32]uint32{}
	for _, in := range instrs {
		if in.opcode == OpConstant && len(in.words) == 4 {
			consts[in.words[2]] = in.words[3]
		}
	}
	value := func(id uint32) uint32 {
		v, ok := consts[id]
		if !ok {
			t.Fatalf("semantics operand %%%d is not a 32-bit OpConstant", id)
		}
		return v
	}
	for _, in := range instrs {
		switch in.opcode {
		case OpControlBarrier:
			control = append(control, value(in.words[3]))
		case OpMemoryBarrier:
			memory = append(memory, value(in.words[2]))
		}
	}
	return control, memory
}

// A subgroup barrier orders the memory a subgroup can share. Vulkan requires a
// non-relaxed memory order to name at least one Vulkan storage class
// (VUID-StandaloneSpirv-MemorySemantics-10870: UniformMemory, WorkgroupMemory,
// ImageMemory or OutputMemory); SubgroupMemory is not one, and the validation
// layer rejected `subgroupBarrier()` at vkCreateShaderModule. Rust naga emits
// AcquireRelease|SubgroupMemory (0x88, back/spv/writer.rs write_control_barrier)
// and has the same bug. glslang compiles GLSL subgroupBarrier and
// subgroupMemoryBarrier (what Rust's GLSL backend writes for this barrier) to
// AcquireRelease|UniformMemory|WorkgroupMemory|AtomicCounterMemory|ImageMemory;
// we take the same storage classes but atomic counters, which WGSL and Vulkan
// do not have.
func TestSubgroupBarrierSemanticsValidForVulkan(t *testing.T) {
	const want = MemorySemanticsAcquireRelease | MemorySemanticsUniformMemory |
		MemorySemanticsWorkgroupMemory | MemorySemanticsImageMemory
	spv := compileWGSLForCapabilityTestWithOpts(t, `
enable subgroups;
@compute @workgroup_size(1)
fn main() {
    subgroupBarrier();
}
`, Options{Version: Version1_3})
	control, _ := barrierSemantics(t, spv)
	if len(control) != 1 {
		t.Fatalf("OpControlBarrier count = %d, want 1", len(control))
	}
	if got := control[0]; got != want {
		t.Errorf("subgroupBarrier semantics = %#x, want %#x (AcquireRelease|Uniform|Workgroup|Image, no SubgroupMemory)", got, want)
	}
}

// The workgroup and storage barriers keep Rust's semantics: their storage
// class bits already satisfy Vulkan.
func TestWorkgroupAndStorageBarrierSemanticsUnchanged(t *testing.T) {
	spv := compileWGSLForCapabilityTestWithOpts(t, `
@compute @workgroup_size(1)
fn main() {
    workgroupBarrier();
    storageBarrier();
    textureBarrier();
}
`, Options{Version: Version1_3})
	control, _ := barrierSemantics(t, spv)
	want := []uint32{
		MemorySemanticsAcquireRelease | MemorySemanticsWorkgroupMemory,
		MemorySemanticsAcquireRelease | MemorySemanticsUniformMemory,
		MemorySemanticsAcquireRelease | MemorySemanticsImageMemory,
	}
	if len(control) != len(want) {
		t.Fatalf("OpControlBarrier count = %d, want %d", len(control), len(want))
	}
	for i := range want {
		if control[i] != want[i] {
			t.Errorf("barrier %d semantics = %#x, want %#x", i, control[i], want[i])
		}
	}
}
