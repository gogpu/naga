package spirv

import (
	"testing"

	"github.com/gogpu/naga/backend"
)

func TestZeroInitializeWorkgroupMemoryModeReachesCodegen(t *testing.T) {
	for _, mode := range []backend.ZeroInitializeWorkgroupMemoryMode{
		backend.ZeroInitWorkgroupPolyfill,
		backend.ZeroInitWorkgroupNative,
		backend.ZeroInitWorkgroupNone,
	} {
		got := toCodegenOptions(Options{ZeroInitializeWorkgroupMemory: mode}).ZeroInitializeWorkgroupMemory
		if got != mode {
			t.Errorf("mode %d reaches codegen as %d", mode, got)
		}
	}
}

func TestDemoteToHelperInvocationReachesCodegen(t *testing.T) {
	for _, want := range []bool{false, true} {
		got := toCodegenOptions(Options{DemoteToHelperInvocation: want}).DemoteToHelperInvocation
		if got != want {
			t.Errorf("DemoteToHelperInvocation %v reaches codegen as %v", want, got)
		}
	}
}
