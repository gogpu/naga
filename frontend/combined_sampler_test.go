package frontend

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestCombinedImageSamplerZeroValueSplits pins the documented zero value: a
// zero ParseOptions splits combined image samplers.
func TestCombinedImageSamplerZeroValueSplits(t *testing.T) {
	var policy CombinedImageSamplerPolicy
	if policy != CombinedImageSamplerSplit {
		t.Errorf("zero CombinedImageSamplerPolicy = %d, want CombinedImageSamplerSplit", policy)
	}
}

// TestSamplerBindingsKey compiles a lookup the way a frontend does it.
func TestSamplerBindingsKey(t *testing.T) {
	bindings := SamplerBindings{
		{Group: 0, Binding: 1}: {Group: 0, Binding: 7},
	}
	got, ok := bindings[ir.ResourceBinding{Group: 0, Binding: 1}]
	if !ok || got != (ir.ResourceBinding{Group: 0, Binding: 7}) {
		t.Errorf("lookup = %v, %v; want {0 7}, true", got, ok)
	}
}
