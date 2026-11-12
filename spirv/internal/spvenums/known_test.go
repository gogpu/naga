package spvenums

import "testing"

// TestKnownCoversEveryEnum exercises Known on each generated type: a value the
// grammar defines is known, one it does not is not.
func TestKnownCoversEveryEnum(t *testing.T) {
	cases := []struct {
		name  string
		known func() bool
		wrong func() bool
	}{
		{"Op", func() bool { return OpNop.Known() }, func() bool { return Op(0xffff).Known() }},
		{"Capability", func() bool { return CapabilityShader.Known() }, func() bool { return Capability(0xffffff).Known() }},
		{"Decoration", func() bool { return DecorationBuiltIn.Known() }, func() bool { return Decoration(0xffffff).Known() }},
		{"BuiltIn", func() bool { return BuiltInPosition.Known() }, func() bool { return BuiltIn(0xffffff).Known() }},
		{"ExecutionModel", func() bool { return ExecutionModelVertex.Known() }, func() bool { return ExecutionModel(0xffffff).Known() }},
		{"ExecutionMode", func() bool { return ExecutionModeLocalSize.Known() }, func() bool { return ExecutionMode(0xffffff).Known() }},
		{"StorageClass", func() bool { return StorageClassFunction.Known() }, func() bool { return StorageClass(0xffffff).Known() }},
		{"AddressingModel", func() bool { return AddressingModelLogical.Known() }, func() bool { return AddressingModel(0xffffff).Known() }},
		{"MemoryModel", func() bool { return MemoryModelGLSL450.Known() }, func() bool { return MemoryModel(0xffffff).Known() }},
		{"FunctionControl", func() bool { return FunctionControlNone.Known() }, func() bool { return FunctionControl(0xffffffff).Known() }},
		{"SelectionControl", func() bool { return SelectionControlNone.Known() }, func() bool { return SelectionControl(0xffffffff).Known() }},
		{"LoopControl", func() bool { return LoopControlNone.Known() }, func() bool { return LoopControl(0xffffffff).Known() }},
		{"Dim", func() bool { return Dim2D.Known() }, func() bool { return Dim(0xffffff).Known() }},
		{"ImageFormat", func() bool { return ImageFormatRgba8.Known() }, func() bool { return ImageFormat(0xffffff).Known() }},
		{"ImageOperands", func() bool { return ImageOperandsBias.Known() }, func() bool { return ImageOperands(0xffffffff).Known() }},
		{"MemoryAccess", func() bool { return MemoryAccessVolatile.Known() }, func() bool { return MemoryAccess(0xffffffff).Known() }},
		{"MemorySemantics", func() bool { return MemorySemanticsUniformMemory.Known() }, func() bool { return MemorySemantics(0xffffffff).Known() }},
		{"Scope", func() bool { return ScopeWorkgroup.Known() }, func() bool { return Scope(0xffffff).Known() }},
		{"GroupOperation", func() bool { return GroupOperationReduce.Known() }, func() bool { return GroupOperation(0xffffff).Known() }},
		{"PackedVectorFormat", func() bool { return PackedVectorFormat4x8Bit.Known() }, func() bool { return PackedVectorFormat(0xffffff).Known() }},
		{"RayFlags", func() bool { return RayFlagsNoneKHR.Known() }, func() bool { return RayFlags(0xffffffff).Known() }},
		{"GlslStd450", func() bool { return GlslStd450Radians.Known() }, func() bool { return GlslStd450(0xffffff).Known() }},
	}
	for _, tc := range cases {
		if !tc.known() {
			t.Errorf("%s: a defined value reported unknown", tc.name)
		}
		if tc.wrong() {
			t.Errorf("%s: an undefined value reported known", tc.name)
		}
	}
}
