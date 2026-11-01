package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// --- blockContainsReturn ---

func TestBlockContainsReturn(t *testing.T) {
	tests := []struct {
		name  string
		block ir.Block
		want  bool
	}{
		{
			name:  "empty_block",
			block: ir.Block{},
			want:  false,
		},
		{
			name:  "direct_return",
			block: ir.Block{{Kind: ir.StmtReturn{}}},
			want:  true,
		},
		{
			name: "no_return",
			block: ir.Block{
				{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
				{Kind: ir.StmtBreak{}},
			},
			want: false,
		},
		{
			name: "return_in_if_accept",
			block: ir.Block{
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    ir.Block{{Kind: ir.StmtReturn{}}},
					Reject:    ir.Block{},
				}},
			},
			want: true,
		},
		{
			name: "return_in_if_reject",
			block: ir.Block{
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    ir.Block{},
					Reject:    ir.Block{{Kind: ir.StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "return_in_switch_case",
			block: ir.Block{
				{Kind: ir.StmtSwitch{
					Selector: 0,
					Cases: []ir.SwitchCase{
						{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtReturn{}}}},
					},
				}},
			},
			want: true,
		},
		{
			name: "return_in_loop_body",
			block: ir.Block{
				{Kind: ir.StmtLoop{
					Body:       ir.Block{{Kind: ir.StmtReturn{}}},
					Continuing: ir.Block{},
				}},
			},
			want: true,
		},
		{
			name: "return_in_loop_continuing",
			block: ir.Block{
				{Kind: ir.StmtLoop{
					Body:       ir.Block{},
					Continuing: ir.Block{{Kind: ir.StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "return_in_nested_block",
			block: ir.Block{
				{Kind: ir.StmtBlock{
					Block: ir.Block{{Kind: ir.StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "no_return_in_nested_structures",
			block: ir.Block{
				{Kind: ir.StmtIf{
					Condition: 0,
					Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
					Reject:    ir.Block{{Kind: ir.StmtContinue{}}},
				}},
				{Kind: ir.StmtSwitch{
					Selector: 0,
					Cases: []ir.SwitchCase{
						{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtBreak{}}}},
					},
				}},
				{Kind: ir.StmtLoop{
					Body:       ir.Block{{Kind: ir.StmtBreak{}}},
					Continuing: ir.Block{},
				}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockContainsReturn(tt.block)
			if got != tt.want {
				t.Errorf("blockContainsReturn = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- shouldAliasArgType ---

func TestShouldAliasArgType(t *testing.T) {
	tests := []struct {
		name  string
		inner ir.TypeInner
		want  bool
	}{
		// Types that should be aliased (not spilled)
		{"PointerType", ir.PointerType{Base: 0, Space: ir.SpaceFunction}, true},
		{"ImageType", ir.ImageType{Dim: ir.Dim2D}, true},
		{"SamplerType", ir.SamplerType{Comparison: false}, true},
		{"BindingArrayType", ir.BindingArrayType{Base: 0}, true},
		{"VectorType", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, true},
		{"MatrixType", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, true},
		{"ArrayType", ir.ArrayType{Base: 0, Stride: 16}, true},
		{"StructType", ir.StructType{Members: nil, Span: 0}, true},

		// Types that should be spilled (plain scalars)
		{"ScalarType_f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, false},
		{"ScalarType_u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, false},
		{"ScalarType_i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, false},
		{"ScalarType_bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, false},

		// Other types that are not aliased
		{"AtomicType", ir.AtomicType{Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAliasArgType(tt.inner)
			if got != tt.want {
				t.Errorf("shouldAliasArgType(%T) = %v, want %v", tt.inner, got, tt.want)
			}
		})
	}
}
