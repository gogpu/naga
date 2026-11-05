package valid_test

import (
	"testing"

	"github.com/gogpu/naga/valid"
)

// TestMissingCapabilitiesAgreesWithTheGates: for every feature the gate
// table covers, MissingCapabilities reports a capability without the
// options that lack it and none with the options that grant it, so a
// backend that refuses on it refuses exactly what Validate would reject.
func TestMissingCapabilitiesAgreesWithTheGates(t *testing.T) {
	for _, tt := range capabilityGateCases {
		t.Run(tt.name, func(t *testing.T) {
			m := parse(t, tt.src)
			without := valid.Options{}
			if tt.without != nil {
				without = *tt.without
			}
			if got := valid.MissingCapabilities(m, without.Capabilities); got == 0 {
				t.Errorf("MissingCapabilities(%v) = 0, but Validate rejects the module", without.Capabilities)
			}
			if got := valid.MissingCapabilities(m, tt.with.Capabilities); got != 0 {
				t.Errorf("MissingCapabilities(%v) = %v, but Validate accepts the module", tt.with.Capabilities, got)
			}
		})
	}
}

// TestMissingCapabilities pins the exact set for features that need one
// capability, and the cases where a module needs none or any of two.
func TestMissingCapabilities(t *testing.T) {
	const (
		f32Atomic = `@group(0) @binding(0) var<storage, read_write> a: atomic<f32>;
@compute @workgroup_size(1) fn main() { atomicAdd(&a, 1.0); }`
		u64Atomic = `@group(0) @binding(0) var<storage, read_write> a: atomic<u64>;
@compute @workgroup_size(1) fn main() { _ = atomicLoad(&a); }`
		clip = `enable clip_distances;
struct Out { @builtin(position) p: vec4<f32>, @builtin(clip_distances) c: array<f32, 2> }
@vertex fn main() -> Out { var o: Out; return o; }`
		plain = `@compute @workgroup_size(1) fn main() { var x = 1.0; x += 1.0; }`
	)
	for _, tt := range []struct {
		name      string
		src       string
		supported valid.Capabilities
		want      valid.Capabilities
	}{
		{"f32 atomic, nothing supported", f32Atomic, 0, valid.CapShaderFloat32Atomic},
		{"f32 atomic, all supported", f32Atomic, valid.CapAll, 0},
		{"f32 atomic, others supported", f32Atomic, valid.CapAll &^ valid.CapShaderFloat32Atomic, valid.CapShaderFloat32Atomic},
		{"clip distances", clip, 0, valid.CapClipDistances},
		{"clip distances supported", clip, valid.CapClipDistances, 0},
		{"a module that needs nothing", plain, 0, 0},
		// The atomic type check accepts either 64-bit atomic capability, as
		// Rust's does: a backend with MIN_MAX only (Metal) still writes it.
		{"u64 atomic type, MIN_MAX only", u64Atomic, valid.CapShaderInt64 | valid.CapShaderInt64AtomicMinMax, 0},
		{"u64 atomic type, neither", u64Atomic, valid.CapShaderInt64, valid.CapShaderInt64AtomicAllOps},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := valid.MissingCapabilities(parse(t, tt.src), tt.supported); got != tt.want {
				t.Errorf("MissingCapabilities = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMissingCapabilitiesNilModule: a nil module needs nothing.
func TestMissingCapabilitiesNilModule(t *testing.T) {
	if got := valid.MissingCapabilities(nil, 0); got != 0 {
		t.Errorf("MissingCapabilities(nil) = %v, want 0", got)
	}
}

// TestCapabilitiesString: a set prints as Rust's bitflags Debug does, the
// text wgpu looks for in a missing-capability error.
func TestCapabilitiesString(t *testing.T) {
	for _, tt := range []struct {
		c    valid.Capabilities
		want string
	}{
		{valid.CapShaderFloat32Atomic, "Capabilities(SHADER_FLOAT32_ATOMIC)"},
		{valid.CapSubgroup | valid.CapSubgroupBarrier, "Capabilities(SUBGROUP | SUBGROUP_BARRIER)"},
		{0, "Capabilities(0x0)"},
	} {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("String(%#x) = %q, want %q", uint64(tt.c), got, tt.want)
		}
	}
}
