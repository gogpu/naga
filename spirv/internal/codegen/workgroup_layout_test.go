package codegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gogpu/naga/internal/testtools"
)

// TestWorkgroupLayoutCopyBelow14: Workgroup memory has layout-free types
// (VUID-StandaloneSpirv-None-10684), so a value moved between it and a
// buffer changes type. From SPIR-V 1.4 on that is OpCopyLogical; below, the
// backend copies the value member by member in a function per type, as Tint
// does, and keeps the version the caller asked for: before, it raised the
// module to 1.4, which a Vulkan 1.0 device rejects. Nested structs and arrays
// both ways, validated as Vulkan 1.0 and as Vulkan 1.1 with SPIR-V 1.4.
func TestWorkgroupLayoutCopyBelow14(t *testing.T) {
	spirvVal := testtools.Look(t, "spirv-val", testtools.InstallSPIRVTools)
	const source = `
struct Inner { a: u32, b: array<f32, 3> }
struct Outer { x: Inner, y: array<Inner, 2> }
var<workgroup> w: Outer;
@group(0) @binding(0) var<storage, read_write> buf: Outer;
@compute @workgroup_size(1)
fn main() {
    w = buf;
    buf = w;
    buf.y[1] = w.y[0];
}`
	for _, tc := range []struct {
		version   Version
		targetEnv string
		copyOp    bool
	}{
		{Version1_0, "vulkan1.0", false},
		{Version1_4, "vulkan1.1spv1.4", true},
	} {
		spv := compileWGSLForCapabilityTestWithOpts(t, source, Options{Version: tc.version})
		if spv[5] != tc.version.Minor {
			t.Errorf("SPIR-V %d.%d asked: module is 1.%d", tc.version.Major, tc.version.Minor, spv[5])
		}
		if got := divmodHasOpcode(spv, OpCopyLogical); got != tc.copyOp {
			t.Errorf("SPIR-V 1.%d: OpCopyLogical present %v, want %v", tc.version.Minor, got, tc.copyOp)
		}
		path := filepath.Join(t.TempDir(), "w.spv")
		if err := os.WriteFile(path, spv, 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(spirvVal, "--target-env", tc.targetEnv, path).CombinedOutput(); err != nil {
			t.Errorf("SPIR-V 1.%d, %s: spirv-val:\n%s", tc.version.Minor, tc.targetEnv, out)
		}
	}
}
