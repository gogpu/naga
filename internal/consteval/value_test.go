package consteval

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

func TestScalarOf(t *testing.T) {
	tests := []struct {
		lit  ir.LiteralValue
		want ir.ScalarType
	}{
		{ir.LiteralBool(true), ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		{ir.LiteralI32(1), ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		{ir.LiteralU32(1), ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		{ir.LiteralI64(1), ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},
		{ir.LiteralU64(1), ir.ScalarType{Kind: ir.ScalarUint, Width: 8}},
		{ir.LiteralF16(1), ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}},
		{ir.LiteralF32(1), ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{ir.LiteralF64(1), ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}},
		{ir.LiteralAbstractInt(1), ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}},
		{ir.LiteralAbstractFloat(1), ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}},
	}
	for _, tt := range tests {
		if got := ScalarOf(tt.lit); got != tt.want {
			t.Errorf("ScalarOf(%T) = %+v, want %+v", tt.lit, got, tt.want)
		}
	}
}

func TestValueShape(t *testing.T) {
	s := Scalar(ir.LiteralF32(1))
	if !s.IsScalar() || s.Len() != 1 {
		t.Errorf("Scalar: IsScalar=%v Len=%d", s.IsScalar(), s.Len())
	}
	v := Vector(ir.LiteralI32(1), ir.LiteralI32(2), ir.LiteralI32(3))
	if v.IsScalar() || v.Len() != 3 || v.Size() != ir.Vec3 {
		t.Errorf("Vector: IsScalar=%v Len=%d Size=%v", v.IsScalar(), v.Len(), v.Size())
	}
	if got := v.Scalar(); got != (ir.ScalarType{Kind: ir.ScalarSint, Width: 4}) {
		t.Errorf("Vector.Scalar() = %+v", got)
	}
}

func TestVectorPanicsOnBadShape(t *testing.T) {
	for _, comps := range [][]ir.LiteralValue{
		{ir.LiteralI32(1)},
		{ir.LiteralI32(1), ir.LiteralI32(2), ir.LiteralI32(3), ir.LiteralI32(4), ir.LiteralI32(5)},
		{ir.LiteralI32(1), ir.LiteralU32(2)},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Vector(%v) did not panic", comps)
				}
			}()
			Vector(comps...)
		}()
	}
}
