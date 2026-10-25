package codegen

import "testing"

// TestZeroInitializeWorkgroupMemoryModes checks that Polyfill zeroes the
// workgroup variable with a store and a barrier, Native with a null
// initializer, and None not at all.
func TestZeroInitializeWorkgroupMemoryModes(t *testing.T) {
	const src = `
var<workgroup> shared_data: array<u32, 64>;
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) i: u32) {
	out[i] = shared_data[i];
}
`
	for _, tc := range []struct {
		name                 string
		mode                 ZeroInitializeWorkgroupMemoryMode
		barrier, initializer bool
	}{
		{"polyfill", ZeroInitWorkgroupPolyfill, true, false},
		{"native", ZeroInitWorkgroupNative, false, true},
		{"none", ZeroInitWorkgroupNone, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := DefaultOptions()
			opts.ZeroInitializeWorkgroupMemory = tc.mode
			spv, err := compileWGSLWithOpts(t, src, opts)
			if err != nil {
				t.Fatal(err)
			}
			var barrier, init bool
			for _, in := range decodeSPIRVInstructions(spv) {
				switch {
				case in.opcode == OpControlBarrier:
					barrier = true
				case in.opcode == OpVariable && StorageClass(in.words[3]) == StorageClassWorkgroup:
					init = len(in.words) > 4
				}
			}
			if barrier != tc.barrier {
				t.Errorf("barrier = %v, want %v", barrier, tc.barrier)
			}
			if init != tc.initializer {
				t.Errorf("initializer = %v, want %v", init, tc.initializer)
			}
		})
	}
}
