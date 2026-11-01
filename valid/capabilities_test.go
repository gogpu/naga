package valid

import "testing"

// TestCapabilitiesAreRustBits: every capability has the bit of Rust naga's
// valid::Capabilities (valid/mod.rs at the pinned commit), so a value can be
// passed between the two, and the type holds Rust's bits above 31.
func TestCapabilitiesAreRustBits(t *testing.T) {
	for _, tt := range []struct {
		name string
		cap  Capabilities
		rust Capabilities
	}{
		{"FLOAT64", CapFloat64, 1 << 1},
		{"SHADER_INT64", CapShaderInt64, 1 << 16},
		{"SHADER_FLOAT16", CapShaderFloat16, 1 << 26},
	} {
		if tt.cap != tt.rust {
			t.Errorf("%s = %#x, Rust's is %#x", tt.name, uint64(tt.cap), uint64(tt.rust))
		}
	}
	const debugPrintf Capabilities = 1 << 45 // Rust's highest bit at the pin
	if !CapAll.Contains(debugPrintf) {
		t.Error("CapAll does not hold Rust's bit 45")
	}
}
