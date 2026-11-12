package valid

import (
	"fmt"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestRustDebugNames pins the Rust Debug spellings that VaryingError texts
// embed. A drift here changes user-visible messages away from the reference.
func TestRustDebugNames(t *testing.T) {
	scalar := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	uintScalar := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	uint32Ptr := func(v uint32) *uint32 { return &v }
	override := ir.OverrideHandle(3)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "scalar",
			got:  typeInnerDebug(ir.ScalarType{Kind: ir.ScalarSint, Width: 4}),
			want: "Scalar(Scalar { kind: Sint, width: 4 })",
		},
		{
			name: "unknown scalar kind",
			got:  typeInnerDebug(ir.ScalarType{Kind: 99, Width: 4}),
			want: "Scalar(Scalar { kind: ScalarKind(99), width: 4 })",
		},
		{
			name: "vector",
			got:  typeInnerDebug(ir.VectorType{Size: ir.Vec3, Scalar: scalar}),
			want: "Vector { size: Tri, scalar: Scalar { kind: Float, width: 4 } }",
		},
		{
			name: "matrix",
			got:  typeInnerDebug(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: scalar}),
			want: "Matrix { columns: Bi, rows: Tri, scalar: Scalar { kind: Float, width: 4 } }",
		},
		{
			name: "array constant",
			got:  typeInnerDebug(ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: uint32Ptr(2)}, Stride: 8}),
			want: "Array { base: [1], size: Constant(2), stride: 8 }",
		},
		{
			name: "array override",
			got:  typeInnerDebug(ir.ArrayType{Base: 1, Size: ir.ArraySize{Pending: &override}, Stride: 8}),
			want: "Array { base: [1], size: Pending([3]), stride: 8 }",
		},
		{
			name: "array dynamic",
			got:  typeInnerDebug(ir.ArrayType{Base: 1, Stride: 8}),
			want: "Array { base: [1], size: Dynamic, stride: 8 }",
		},
		{
			name: "binding array sized",
			got:  typeInnerDebug(ir.BindingArrayType{Base: 2, Size: uint32Ptr(4)}),
			want: "BindingArray { base: [2], size: Constant(4) }",
		},
		{
			name: "binding array unbounded",
			got:  typeInnerDebug(ir.BindingArrayType{Base: 2}),
			want: "BindingArray { base: [2], size: Dynamic }",
		},
		{
			name: "atomic",
			got:  typeInnerDebug(ir.AtomicType{Scalar: uintScalar}),
			want: "Atomic(Scalar { kind: Uint, width: 4 })",
		},
		{
			name: "pointer",
			got:  typeInnerDebug(ir.PointerType{Base: 5, Space: ir.SpaceStorage}),
			want: "Pointer { base: [5], space: Storage { .. } }",
		},
		{
			name: "struct elided",
			got:  typeInnerDebug(ir.StructType{}),
			want: "Struct { .. }",
		},
		{
			name: "value pointer elided",
			got:  typeInnerDebug(ir.ValuePointerType{Space: ir.SpaceFunction}),
			want: "ValuePointer { .. }",
		},
		{
			name: "sampler elided",
			got:  typeInnerDebug(ir.SamplerType{}),
			want: "Sampler { .. }",
		},
		{
			name: "image elided",
			got:  typeInnerDebug(ir.ImageType{}),
			want: "Image { .. }",
		},
		{
			name: "acceleration structure elided",
			got:  typeInnerDebug(ir.AccelerationStructureType{}),
			want: "AccelerationStructure { .. }",
		},
		{
			name: "ray query elided",
			got:  typeInnerDebug(ir.RayQueryType{}),
			want: "RayQuery { .. }",
		},
		{
			name: "unknown vector size",
			got:  vectorSizeDebug(ir.VectorSize(9)),
			want: "VectorSize(9)",
		},
		{
			name: "unknown address space",
			got:  addressSpaceDebug(ir.AddressSpace(99)),
			want: "AddressSpace(99)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// TestRustDebugBuiltinNames pins the Debug spelling of the built-ins the
// VaryingError messages name, and the fallback for the rest.
func TestRustDebugBuiltinNames(t *testing.T) {
	want := map[ir.BuiltinValue]string{
		ir.BuiltinBaseInstance:  "BaseInstance",
		ir.BuiltinBaseVertex:    "BaseVertex",
		ir.BuiltinCullDistance:  "CullDistance",
		ir.BuiltinPointCoord:    "PointCoord",
		ir.BuiltinWorkGroupSize: "WorkGroupSize",
	}
	for builtin, name := range want {
		if got := builtinDebug(builtin); got != name {
			t.Errorf("builtinDebug(%d) = %q, want %q", builtin, got, name)
		}
	}
	if got, wantFallback := builtinDebug(ir.BuiltinPosition), fmt.Sprintf("BuiltIn(%d)", ir.BuiltinPosition); got != wantFallback {
		t.Errorf("builtinDebug(Position) = %q, want %q", got, wantFallback)
	}
}
