package snapshot_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// validateCorpusKnownFailures are corpus shaders the validator rejects though
// Rust naga accepts them, each with its card. The test fails when one of them
// validates, so a fixed shader leaves this list.
var validateCorpusKnownFailures = map[string]string{}

// TestValidateCorpusRustCapabilities validates every Rust corpus shader with
// exactly the capabilities its parameters give Rust's snapshot driver
// (tests/naga/snapshots.rs: params.capabilities, else
// Capabilities::default()). Every capability gate is then no stricter than
// Rust's on the shaders Rust tests it with: a shader that needs a capability
// its parameters do not grant would fail Rust's own snapshot test.
func TestValidateCorpusRustCapabilities(t *testing.T) {
	checked := 0
	for _, s := range loadCorpus(t) {
		if s.origin != originRust {
			continue
		}
		params, err := loadRustParams(s.name)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if params.targets.has(rustTargetNoValidation) {
			continue
		}
		module, err := parseWGSL(s.source)
		if err != nil {
			continue // parse failures are other tests' business
		}
		caps := rustDefaultCapabilities
		if params.capabilities != nil {
			caps = *params.capabilities
		}
		checked++
		if _, err := valid.Validate(module, valid.Options{Capabilities: valid.Capabilities(caps)}); err != nil {
			t.Errorf("%s with Rust's capabilities %#x: %v", s.name, caps, err)
		}
	}
	if checked == 0 {
		t.Fatal("no Rust corpus shader validated")
	}
	t.Logf("%d Rust corpus shaders validate with their Rust capabilities", checked)
}

// gatedCapabilities are the capabilities valid.Validate checks.
const gatedCapabilities = valid.CapFloat64 | valid.CapPrimitiveIndex | valid.CapTextureAndSamplerBindingArray |
	valid.CapBufferBindingArray | valid.CapStorageTextureBindingArray | valid.CapStorageBufferBindingArray |
	valid.CapClipDistances | valid.CapStorageTexture16BitNormFormats | valid.CapMultiview | valid.CapEarlyDepthTest |
	valid.CapRayQuery | valid.CapDualSourceBlending | valid.CapShaderInt64 | valid.CapSubgroup |
	valid.CapSubgroupBarrier | valid.CapSubgroupVertexStage | valid.CapShaderInt64AtomicMinMax |
	valid.CapShaderInt64AtomicAllOps | valid.CapShaderFloat32Atomic | valid.CapTextureAtomic |
	valid.CapTextureInt64Atomic | valid.CapShaderFloat16 | valid.CapShaderBarycentrics | valid.CapMeshShader |
	valid.CapMeshShaderPointTopology | valid.CapPerVertex | valid.CapDrawIndex |
	valid.CapAccelerationStructureBindingArray | valid.CapShaderInt16

// TestValidateCorpusNeedsRustCapabilities: a Rust corpus shader is rejected
// without any one of the capabilities its parameters grant that the
// validator checks, so each gate fires on the shaders Rust wrote for it.
func TestValidateCorpusNeedsRustCapabilities(t *testing.T) {
	checked := 0
	for _, s := range loadCorpus(t) {
		if s.origin != originRust {
			continue
		}
		params, err := loadRustParams(s.name)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if params.capabilities == nil || params.targets.has(rustTargetNoValidation) {
			continue
		}
		caps := valid.Capabilities(*params.capabilities)
		if caps&gatedCapabilities == 0 {
			continue
		}
		module, err := parseWGSL(s.source)
		if err != nil {
			continue
		}
		for bit := range 64 {
			c := valid.Capabilities(1) << bit
			if caps&gatedCapabilities&c == 0 {
				continue
			}
			checked++
			if _, err := valid.Validate(module, valid.Options{Capabilities: caps &^ c}); err == nil {
				t.Errorf("%s validates without %s", s.name, rustCapabilityNames[bit])
			}
		}
	}
	t.Logf("%d (shader, capability) pairs of the Rust corpus need the capability", checked)
}

// TestValidateCorpus validates every corpus shader that parses, with every
// capability, as Rust naga's snapshot tests validate their inputs: every
// shader there is valid, so a rejection is our validator being stricter than
// Rust, which the project's floor forbids unless the WGSL spec requires it.
func TestValidateCorpus(t *testing.T) {
	for _, s := range loadCorpus(t) {
		module, err := parseWGSL(s.source)
		if err != nil {
			continue // parse failures are other tests' business
		}
		_, verr := valid.Validate(module, valid.Options{Capabilities: valid.CapAll})
		card, known := validateCorpusKnownFailures[s.name]
		switch {
		case verr != nil && known:
			t.Logf("known failure %s (%s): %v", s.name, card, verr)
		case verr != nil:
			t.Errorf("%s: %v", s.name, verr)
		case known:
			t.Errorf("%s validates now: remove it from validateCorpusKnownFailures and close %s", s.name, card)
		}
	}
}
