package codegen

import (
	"slices"
	"testing"

	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// SPIR-V opcode and capability of WGSL discard as demote-to-helper
// (SPIR-V 1.6, section 3.52.1). The SPV_EXT_demote_to_helper_invocation
// enumerants share both numbers with the core ones; checked against
// spirv-as and spirv-val.
const (
	opDemoteToHelperInvocation  = spvenums.Op(5380)
	capDemoteToHelperInvocation = spvenums.Capability(5379)
	demoteToHelperExtension     = "SPV_EXT_demote_to_helper_invocation"
)

// discardWGSL discards in a branch, like wgpu's readback case (P2-SPV-015).
const discardWGSL = `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    if uv.x < 0.0 && uv.y < 0.0 {
        discard;
    }
    return vec4<f32>(uv, 0.0, 1.0);
}
`

// TestDiscardKeepsOpKillByDefault: the zero Options keeps spvenums.OpKill, as Rust
// naga always writes it (back/spv/block.rs, Statement::Kill), with no
// demote capability or extension.
func TestDiscardKeepsOpKillByDefault(t *testing.T) {
	spv := compileWGSLForCapabilityTest(t, discardWGSL)
	validateSPIRVBinary(t, spv)
	validateWithVulkanSDK(t, spv)

	instrs := decodeSPIRVInstructions(spv)
	if !hasOpcodeInInstrs(instrs, spvenums.OpKill) {
		t.Error("expected OpKill for WGSL discard with the default options")
	}
	if hasOpcodeInInstrs(instrs, opDemoteToHelperInvocation) {
		t.Error("OpDemoteToHelperInvocation written without DemoteToHelperInvocation")
	}
	if extractCapabilities(spv)[uint32(capDemoteToHelperInvocation)] {
		t.Error("DemoteToHelperInvocation capability declared without the option")
	}
	if slices.Contains(extractExtensions(spv), demoteToHelperExtension) {
		t.Errorf("extension %s declared without the option", demoteToHelperExtension)
	}
}

// TestDiscardDemoteToHelperInvocation: with the option, discard becomes
// OpDemoteToHelperInvocation with its capability, and the
// SPV_EXT_demote_to_helper_invocation extension below SPIR-V 1.6 only: the
// instruction is core in 1.6. Each version is validated by spirv-val at its
// own target environment.
func TestDiscardDemoteToHelperInvocation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		version        Version
		targetEnv      string
		wantsExtension bool
	}{
		{"1.0", Version1_0, "spv1.0", true},
		{"1.3", Version1_3, "spv1.3", true},
		{"1.5", Version1_5, "spv1.5", true},
		{"1.6", Version1_6, "spv1.6", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.Version = tc.version
			opts.DemoteToHelperInvocation = true
			spv := compileWGSLForCapabilityTestWithOpts(t, discardWGSL, opts)
			validateSPIRVBinary(t, spv)
			validateWithVulkanSDKTarget(t, spv, tc.targetEnv)

			instrs := decodeSPIRVInstructions(spv)
			if !hasOpcodeInInstrs(instrs, opDemoteToHelperInvocation) {
				t.Error("expected OpDemoteToHelperInvocation for WGSL discard")
			}
			if hasOpcodeInInstrs(instrs, spvenums.OpKill) {
				t.Error("OpKill written with DemoteToHelperInvocation")
			}
			if !extractCapabilities(spv)[uint32(capDemoteToHelperInvocation)] {
				t.Error("DemoteToHelperInvocation capability missing")
			}
			if got := slices.Contains(extractExtensions(spv), demoteToHelperExtension); got != tc.wantsExtension {
				t.Errorf("extension %s declared = %v, want %v", demoteToHelperExtension, got, tc.wantsExtension)
			}
		})
	}
}

// TestDemoteToHelperInvocationWithoutDiscard: the option alone declares
// nothing; the capability and extension come with a discard, as Tint's
// writer pushes them in EmitDiscard.
func TestDemoteToHelperInvocationWithoutDiscard(t *testing.T) {
	const noDiscard = `
@fragment
fn main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv, 0.0, 1.0);
}
`
	opts := DefaultOptions()
	opts.Version = Version1_3
	opts.DemoteToHelperInvocation = true
	spv := compileWGSLForCapabilityTestWithOpts(t, noDiscard, opts)
	validateSPIRVBinary(t, spv)
	validateWithVulkanSDKTarget(t, spv, "spv1.3")

	if extractCapabilities(spv)[uint32(capDemoteToHelperInvocation)] {
		t.Error("DemoteToHelperInvocation capability declared without a discard")
	}
	if slices.Contains(extractExtensions(spv), demoteToHelperExtension) {
		t.Errorf("extension %s declared without a discard", demoteToHelperExtension)
	}
}
