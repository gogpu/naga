package glsl_test

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/glsl"
)

func TestZeroInitializeWorkgroupMemory(t *testing.T) {
	const src = `
var<workgroup> shared_data: array<u32, 64>;
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) i: u32) {
	out[i] = shared_data[i];
}
`
	module := lowerWGSL(t, src)
	for _, mode := range []backend.ZeroInitializeWorkgroupMemoryMode{backend.ZeroInitWorkgroupPolyfill, backend.ZeroInitWorkgroupNone} {
		code, _, err := glsl.Compile(module, glsl.Options{LangVersion: glsl.Version430, ZeroInitializeWorkgroupMemory: mode})
		if err != nil {
			t.Fatal(err)
		}
		want := mode == backend.ZeroInitWorkgroupPolyfill
		if zeroed := strings.Contains(code, "shared_data = "); zeroed != want {
			t.Errorf("ZeroInitializeWorkgroupMemory %v: zeroed = %v\n%s", mode, zeroed, code)
		}
	}
}
