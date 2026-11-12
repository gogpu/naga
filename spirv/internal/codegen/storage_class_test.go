package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// TestAddressSpaceStorageClasses pins the storage class of every address
// space. The switch to the generated enums corrected the task payload value:
// the hand-kept constant said 5328, which the specification assigns to
// CallableDataKHR; TaskPayloadWorkgroupEXT is 5402.
func TestAddressSpaceStorageClasses(t *testing.T) {
	if got := uint32(spvenums.StorageClassTaskPayloadWorkgroupEXT); got != 5402 {
		t.Errorf("StorageClassTaskPayloadWorkgroupEXT = %d, want 5402", got)
	}
	tests := []struct {
		space ir.AddressSpace
		want  spvenums.StorageClass
	}{
		{ir.SpaceFunction, spvenums.StorageClassFunction},
		{ir.SpacePrivate, spvenums.StorageClassPrivate},
		{ir.SpaceWorkGroup, spvenums.StorageClassWorkgroup},
		{ir.SpaceUniform, spvenums.StorageClassUniform},
		{ir.SpaceStorage, spvenums.StorageClassStorageBuffer},
		{ir.SpaceHandle, spvenums.StorageClassUniformConstant},
		{ir.SpaceImmediate, spvenums.StorageClassPushConstant},
		{ir.SpaceTaskPayload, spvenums.StorageClassTaskPayloadWorkgroupEXT},
	}
	for _, tt := range tests {
		got, err := addressSpaceToStorageClass(tt.space)
		if err != nil || got != tt.want {
			t.Errorf("addressSpaceToStorageClass(%v) = %v, %v; want %v", tt.space, got, err, tt.want)
		}
	}
}
