package codegen

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestIndexConstantValue covers the constant resolution Restrict bounds checks
// use to skip a clamp: Rust naga's get_const_val_from (proc/mod.rs) reads a
// literal, a module constant's init, and a scalar zero value; everything else,
// including a negative integer, is a runtime index.
func TestIndexConstantValue(t *testing.T) {
	uintType := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}
	floatType := ir.Type{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}
	module := &ir.Module{
		Types: []ir.Type{uintType, floatType},
		Constants: []ir.Constant{
			{Type: 0, Init: 0},
			{Type: 0, Init: 1},
			{Type: 0, Init: 9}, // out-of-range global expression
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(2)}},
			{Kind: ir.ExprZeroValue{Type: 0}},
		},
	}
	fn := &ir.Function{Expressions: []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralU32(7)}},
		{Kind: ir.ExprConstant{Constant: 0}},
		{Kind: ir.ExprConstant{Constant: 1}},
		{Kind: ir.ExprConstant{Constant: 2}},
		{Kind: ir.ExprConstant{Constant: 9}},
		{Kind: ir.ExprZeroValue{Type: 0}},
		{Kind: ir.ExprZeroValue{Type: 1}},
		{Kind: ir.ExprZeroValue{Type: 9}},
		{Kind: ir.Literal{Value: ir.LiteralI32(-1)}},
		{Kind: ir.ExprLocalVariable{Variable: 0}},
	}}
	w := &Writer{module: module, currentFunction: fn}

	cases := []struct {
		name  string
		index ir.ExpressionHandle
		want  uint32
		known bool
	}{
		{"literal", 0, 7, true},
		{"module constant initialized by a literal", 1, 2, true},
		{"module constant initialized by a zero value", 2, 0, true},
		{"module constant past the arena", 3, 0, false},
		{"constant past the constants arena", 4, 0, false},
		{"zero value of an integer scalar", 5, 0, true},
		{"zero value of a float scalar", 6, 0, false},
		{"zero value of an unknown type", 7, 0, false},
		{"negative literal", 8, 0, false},
		{"a runtime value", 9, 0, false},
		{"past the expression arena", 99, 0, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, known := w.indexConstantValue(tt.index)
			if got != tt.want || known != tt.known {
				t.Errorf("indexConstantValue(%d) = (%d, %t), want (%d, %t)",
					tt.index, got, known, tt.want, tt.known)
			}
		})
	}

	if _, known := (&Writer{}).indexConstantValue(0); known {
		t.Error("a writer without a current function resolved an index")
	}
}
