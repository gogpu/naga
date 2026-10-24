package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
)

// A storage buffer with a dynamic offset adds the offset, from the constant
// buffer of its bind group's dynamic offsets, to the address of every load,
// store and atomic, as the HLSL writer adds __dynamic_buffer_offsets<g>._<i>;
// one without keeps its addresses.
func TestDynamicStorageBufferOffsets(t *testing.T) {
	one := uint32(1)
	opts := DefaultOptions()
	opts.BindingMap = BindingMap{
		{Group: 0, Binding: 0}: {Register: 0},
		{Group: 0, Binding: 1}: {Register: 1, DynamicStorageBufferOffsetsIndex: &one},
		{Group: 0, Binding: 2}: {Register: 2},
	}
	opts.DynamicStorageBufferOffsetsTargets = map[uint32]OffsetsBindTarget{0: {Space: 0, Register: 4, Size: 2}}
	mod, fn := emitMainModuleOpts(t, `
@group(0) @binding(0) var<storage, read_write> fixed: array<u32>;
@group(0) @binding(1) var<storage, read_write> moved: array<atomic<u32>>;
@group(0) @binding(2) var<storage, read> src: array<u32>;
@compute @workgroup_size(1) fn main(@builtin(local_invocation_index) i: u32) {
    fixed[i] = src[i];
    atomicAdd(&moved[i], fixed[i + 1u]);
    atomicStore(&moved[0], 7u);
}
`, opts)
	offsets := map[int]bool{}
	var offsetLoads, offsetAdds int
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			switch {
			case in.Kind == module.InstrCall && in.CalledFunc != nil && in.CalledFunc.Name == "dx.op.cbufferLoadLegacy.i32":
				offsetLoads++
				offsets[in.ValueID] = true
			case in.Kind == module.InstrExtractVal && offsets[in.Operands[0]]:
				offsets[in.ValueID] = true
			case in.Kind == module.InstrBinOp && len(in.Operands) >= 2 && offsets[in.Operands[1]]:
				offsetAdds++
			}
		}
	}
	if offsetLoads != 2 || offsetAdds != 2 {
		t.Errorf("%d offset loads and %d adds, want one of each for the two accesses of moved", offsetLoads, offsetAdds)
	}
	if !hasCBVAt(mod, 0, 4) {
		t.Error("no constant buffer of dynamic offsets at b4")
	}
}

// arrayLength() of a storage buffer whose binding has a
// DynamicStorageBufferSizeIndex loads the binding size from the dynamic
// offsets constant buffer instead of asking the view for its dimensions.
func TestArrayLengthOfDynamicStorageBinding(t *testing.T) {
	offset, size := uint32(0), uint32(1)
	opts := DefaultOptions()
	opts.BindingMap = BindingMap{
		{Group: 0, Binding: 0}: {Register: 0, DynamicStorageBufferOffsetsIndex: &offset, DynamicStorageBufferSizeIndex: &size},
	}
	opts.DynamicStorageBufferOffsetsTargets = map[uint32]OffsetsBindTarget{0: {Space: 0, Register: 4, Size: 2}}
	_, fn := emitMainModuleOpts(t, `
@group(0) @binding(0) var<storage, read_write> moved: array<u32>;
@compute @workgroup_size(1) fn main() {
    moved[0] = arrayLength(&moved);
}
`, opts)
	var dims int
	var sizeLoad bool
	for _, bb := range fn.BasicBlocks {
		for _, in := range bb.Instructions {
			if in.Kind != module.InstrCall || in.CalledFunc == nil {
				continue
			}
			switch in.CalledFunc.Name {
			case "dx.op.getDimensions":
				dims++
			case "dx.op.cbufferLoadLegacy.i32":
				sizeLoad = true
			}
		}
	}
	if dims != 0 || !sizeLoad {
		t.Errorf("%d getDimensions calls and a constant buffer load %v, want the size loaded from the constant buffer only", dims, sizeLoad)
	}
}
