package ir

import (
	"testing"
)

// --- blockContainsReturn ---

func TestBlockContainsReturn(t *testing.T) {
	tests := []struct {
		name  string
		block Block
		want  bool
	}{
		{
			name:  "empty_block",
			block: Block{},
			want:  false,
		},
		{
			name:  "direct_return",
			block: Block{{Kind: StmtReturn{}}},
			want:  true,
		},
		{
			name: "no_return",
			block: Block{
				{Kind: StmtStore{Pointer: 0, Value: 1}},
				{Kind: StmtBreak{}},
			},
			want: false,
		},
		{
			name: "return_in_if_accept",
			block: Block{
				{Kind: StmtIf{
					Condition: 0,
					Accept:    Block{{Kind: StmtReturn{}}},
					Reject:    Block{},
				}},
			},
			want: true,
		},
		{
			name: "return_in_if_reject",
			block: Block{
				{Kind: StmtIf{
					Condition: 0,
					Accept:    Block{},
					Reject:    Block{{Kind: StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "return_in_switch_case",
			block: Block{
				{Kind: StmtSwitch{
					Selector: 0,
					Cases: []SwitchCase{
						{Value: SwitchValueDefault{}, Body: Block{{Kind: StmtReturn{}}}},
					},
				}},
			},
			want: true,
		},
		{
			name: "return_in_loop_body",
			block: Block{
				{Kind: StmtLoop{
					Body:       Block{{Kind: StmtReturn{}}},
					Continuing: Block{},
				}},
			},
			want: true,
		},
		{
			name: "return_in_loop_continuing",
			block: Block{
				{Kind: StmtLoop{
					Body:       Block{},
					Continuing: Block{{Kind: StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "return_in_nested_block",
			block: Block{
				{Kind: StmtBlock{
					Block: Block{{Kind: StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "no_return_in_nested_structures",
			block: Block{
				{Kind: StmtIf{
					Condition: 0,
					Accept:    Block{{Kind: StmtBreak{}}},
					Reject:    Block{{Kind: StmtContinue{}}},
				}},
				{Kind: StmtSwitch{
					Selector: 0,
					Cases: []SwitchCase{
						{Value: SwitchValueDefault{}, Body: Block{{Kind: StmtBreak{}}}},
					},
				}},
				{Kind: StmtLoop{
					Body:       Block{{Kind: StmtBreak{}}},
					Continuing: Block{},
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

// --- blockHasEarlyReturn ---

func TestBlockHasEarlyReturn(t *testing.T) {
	tests := []struct {
		name  string
		block Block
		want  bool
	}{
		{
			name:  "empty_block",
			block: Block{},
			want:  false,
		},
		{
			name: "tail_return_only_is_not_early",
			block: Block{
				{Kind: StmtStore{Pointer: 0, Value: 1}},
				{Kind: StmtReturn{}},
			},
			want: false,
		},
		{
			name: "return_in_if_accept_is_early",
			block: Block{
				{Kind: StmtIf{
					Condition: 0,
					Accept:    Block{{Kind: StmtReturn{}}},
					Reject:    Block{},
				}},
			},
			want: true,
		},
		{
			name: "return_in_if_reject_is_early",
			block: Block{
				{Kind: StmtIf{
					Condition: 0,
					Accept:    Block{},
					Reject:    Block{{Kind: StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "return_in_switch_is_early",
			block: Block{
				{Kind: StmtSwitch{
					Selector: 0,
					Cases: []SwitchCase{
						{Value: SwitchValueI32(0), Body: Block{{Kind: StmtReturn{}}}},
						{Value: SwitchValueDefault{}, Body: Block{{Kind: StmtBreak{}}}},
					},
				}},
			},
			want: true,
		},
		{
			name: "return_in_loop_body_is_early",
			block: Block{
				{Kind: StmtLoop{
					Body:       Block{{Kind: StmtReturn{}}},
					Continuing: Block{},
				}},
			},
			want: true,
		},
		{
			name: "return_in_loop_continuing_is_early",
			block: Block{
				{Kind: StmtLoop{
					Body:       Block{},
					Continuing: Block{{Kind: StmtReturn{}}},
				}},
			},
			want: true,
		},
		{
			name: "return_in_nested_block_is_early",
			block: Block{
				{Kind: StmtBlock{
					Block: Block{
						{Kind: StmtIf{
							Condition: 0,
							Accept:    Block{{Kind: StmtReturn{}}},
							Reject:    Block{},
						}},
					},
				}},
			},
			want: true,
		},
		{
			name: "no_return_anywhere",
			block: Block{
				{Kind: StmtIf{
					Condition: 0,
					Accept:    Block{{Kind: StmtBreak{}}},
					Reject:    Block{{Kind: StmtContinue{}}},
				}},
				{Kind: StmtSwitch{
					Selector: 0,
					Cases: []SwitchCase{
						{Value: SwitchValueDefault{}, Body: Block{{Kind: StmtBreak{}}}},
					},
				}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockHasEarlyReturn(tt.block)
			if got != tt.want {
				t.Errorf("blockHasEarlyReturn = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- shouldAliasArgType ---

func TestShouldAliasArgType(t *testing.T) {
	tests := []struct {
		name  string
		inner TypeInner
		want  bool
	}{
		// Types that should be aliased (not spilled)
		{"PointerType", PointerType{Base: 0, Space: SpaceFunction}, true},
		{"ImageType", ImageType{Dim: Dim2D}, true},
		{"SamplerType", SamplerType{Comparison: false}, true},
		{"BindingArrayType", BindingArrayType{Base: 0}, true},
		{"VectorType", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, true},
		{"MatrixType", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, true},
		{"ArrayType", ArrayType{Base: 0, Stride: 16}, true},
		{"StructType", StructType{Members: nil, Span: 0}, true},

		// Types that should be spilled (plain scalars)
		{"ScalarType_f32", ScalarType{Kind: ScalarFloat, Width: 4}, false},
		{"ScalarType_u32", ScalarType{Kind: ScalarUint, Width: 4}, false},
		{"ScalarType_i32", ScalarType{Kind: ScalarSint, Width: 4}, false},
		{"ScalarType_bool", ScalarType{Kind: ScalarBool, Width: 1}, false},

		// Other types that are not aliased
		{"AtomicType", AtomicType{Scalar: ScalarType{Kind: ScalarUint, Width: 4}}, false},
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
