package backend_test

import (
	"testing"

	"github.com/gogpu/naga/backend"
)

// The zero value of every shared type is the behavior that is safe on every
// device (ADR-009 rule 1), and the constants keep Rust naga's order:
// proc::BoundsCheckPolicy {Restrict, ReadZeroSkipWrite, Unchecked}. Rust's
// default policy is Unchecked; ours is the zero, Restrict.
func TestZeroValuesAreSafe(t *testing.T) {
	var policy backend.BoundsCheckPolicy
	if policy != backend.BoundsCheckRestrict {
		t.Errorf("zero BoundsCheckPolicy = %d, want BoundsCheckRestrict", policy)
	}
	order := []backend.BoundsCheckPolicy{
		backend.BoundsCheckRestrict,
		backend.BoundsCheckReadZeroSkipWrite,
		backend.BoundsCheckUnchecked,
	}
	for i, p := range order {
		if int(p) != i {
			t.Errorf("BoundsCheckPolicy %d is at position %d, want Rust's order", p, i)
		}
	}

	var policies backend.BoundsCheckPolicies
	if policies != (backend.BoundsCheckPolicies{
		Index:        backend.BoundsCheckRestrict,
		Buffer:       backend.BoundsCheckRestrict,
		ImageLoad:    backend.BoundsCheckRestrict,
		BindingArray: backend.BoundsCheckRestrict,
	}) {
		t.Errorf("zero BoundsCheckPolicies = %+v, want Restrict everywhere", policies)
	}

	var mode backend.ZeroInitializeWorkgroupMemoryMode
	if mode != backend.ZeroInitWorkgroupPolyfill {
		t.Errorf("zero ZeroInitializeWorkgroupMemoryMode = %d, want ZeroInitWorkgroupPolyfill", mode)
	}
}

// The mode names are Rust naga's ZeroInitializeWorkgroupMemoryMode variants;
// error messages of every backend print them.
func TestZeroInitializeWorkgroupMemoryModeString(t *testing.T) {
	for mode, want := range map[backend.ZeroInitializeWorkgroupMemoryMode]string{
		backend.ZeroInitWorkgroupPolyfill: "Polyfill",
		backend.ZeroInitWorkgroupNative:   "Native",
		backend.ZeroInitWorkgroupNone:     "None",
		7:                                 "ZeroInitializeWorkgroupMemoryMode(7)",
	} {
		if got := mode.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", uint8(mode), got, want)
		}
	}
}

// The policy names are Rust naga's BoundsCheckPolicy variants.
func TestBoundsCheckPolicyString(t *testing.T) {
	for policy, want := range map[backend.BoundsCheckPolicy]string{
		backend.BoundsCheckRestrict:          "Restrict",
		backend.BoundsCheckReadZeroSkipWrite: "ReadZeroSkipWrite",
		backend.BoundsCheckUnchecked:         "Unchecked",
		7:                                    "BoundsCheckPolicy(7)",
	} {
		if got := policy.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", uint8(policy), got, want)
		}
	}
}
