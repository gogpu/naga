package spirv

import (
	"testing"

	"github.com/gogpu/naga/spirv/internal/codegen"
)

func TestZeroInitializeWorkgroupMemoryModeReachesCodegen(t *testing.T) {
	for mode, want := range map[ZeroInitializeWorkgroupMemoryMode]codegen.ZeroInitializeWorkgroupMemoryMode{
		ZeroInitWorkgroupPolyfill: codegen.ZeroInitWorkgroupPolyfill,
		ZeroInitWorkgroupNative:   codegen.ZeroInitWorkgroupNative,
		ZeroInitWorkgroupNone:     codegen.ZeroInitWorkgroupNone,
	} {
		got := toCodegenOptions(Options{ZeroInitializeWorkgroupMemory: mode}).ZeroInitializeWorkgroupMemory
		if got != want {
			t.Errorf("mode %d reaches codegen as %d, want %d", mode, got, want)
		}
	}
}
