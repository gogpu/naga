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
