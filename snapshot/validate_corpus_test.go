package snapshot_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// validateCorpusKnownFailures are corpus shaders the validator rejects though
// Rust naga accepts them, each with its card. The test fails when one of them
// validates, so a fixed shader leaves this list.
var validateCorpusKnownFailures = map[string]string{}

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
