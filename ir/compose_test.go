package ir

import (
	"slices"
	"testing"
)

// flattenTestModule builds the arenas the FlattenCompose cases share.
// Type handles: 0 = f32, 1 = vec2<f32>, 2 = vec3<f32>, 3 = vec4<f32>,
// 4 = mat2x2<f32>.
func flattenTestModule(exprs []Expression, constants []Constant) *Module {
	f32 := ScalarType{Kind: ScalarFloat, Width: 4}
	return &Module{
		Types: []Type{
			{Inner: f32},
			{Inner: VectorType{Size: Vec2, Scalar: f32}},
			{Inner: VectorType{Size: Vec3, Scalar: f32}},
			{Inner: VectorType{Size: Vec4, Scalar: f32}},
			{Inner: MatrixType{Columns: Vec2, Rows: Vec2, Scalar: f32}},
		},
		Constants:         constants,
		GlobalExpressions: exprs,
	}
}

func flattenLit(v float32) Expression {
	return Expression{Kind: Literal{Value: LiteralF32(v)}}
}

func TestFlattenCompose(t *testing.T) {
	tests := []struct {
		name      string
		exprs     []Expression
		constants []Constant
		want      []ExpressionHandle
	}{
		{
			// vec4(vec3(vec2(6, 7), 8), 9): two levels of nested Compose.
			name: "nested compose",
			exprs: []Expression{
				flattenLit(6), flattenLit(7),
				{Kind: ExprCompose{Type: 1, Components: []ExpressionHandle{0, 1}}},
				flattenLit(8),
				{Kind: ExprCompose{Type: 2, Components: []ExpressionHandle{2, 3}}},
				flattenLit(9),
				{Kind: ExprCompose{Type: 3, Components: []ExpressionHandle{4, 5}}},
			},
			want: []ExpressionHandle{0, 1, 3, 5},
		},
		{
			// vec4(vec3(1.0), 2.0): a Splat yields its value once per lane.
			name: "splat",
			exprs: []Expression{
				flattenLit(1),
				{Kind: ExprSplat{Size: Vec3, Value: 0}},
				flattenLit(2),
				{Kind: ExprCompose{Type: 3, Components: []ExpressionHandle{1, 2}}},
			},
			want: []ExpressionHandle{0, 0, 0, 2},
		},
		{
			// const A = vec2(1, 2); vec3(A, 3): a vector Constant expands to its Init.
			name: "vector constant",
			exprs: []Expression{
				flattenLit(1), flattenLit(2),
				{Kind: ExprCompose{Type: 1, Components: []ExpressionHandle{0, 1}}},
				{Kind: ExprConstant{Constant: 0}},
				flattenLit(3),
				{Kind: ExprCompose{Type: 2, Components: []ExpressionHandle{3, 4}}},
			},
			constants: []Constant{{Name: "A", Type: 1, Init: 2}},
			want:      []ExpressionHandle{0, 1, 4},
		},
		{
			// A scalar Constant is a lane of its own.
			name: "scalar constant",
			exprs: []Expression{
				flattenLit(1),
				{Kind: ExprConstant{Constant: 0}},
				{Kind: ExprCompose{Type: 1, Components: []ExpressionHandle{1, 1}}},
			},
			constants: []Constant{{Name: "S", Type: 0, Init: 0}},
			want:      []ExpressionHandle{1, 1},
		},
		{
			// A vector ZeroValue has no scalar handles; the caller expands it.
			name: "vector zero value",
			exprs: []Expression{
				{Kind: ExprZeroValue{Type: 1}},
				flattenLit(1), flattenLit(2),
				{Kind: ExprCompose{Type: 3, Components: []ExpressionHandle{0, 1, 2}}},
			},
			want: []ExpressionHandle{0, 1, 2},
		},
		{
			// A matrix keeps its columns: only vectors flatten.
			name: "matrix columns",
			exprs: []Expression{
				flattenLit(1), flattenLit(2),
				{Kind: ExprCompose{Type: 1, Components: []ExpressionHandle{0, 1}}},
				flattenLit(3), flattenLit(4),
				{Kind: ExprCompose{Type: 1, Components: []ExpressionHandle{3, 4}}},
				{Kind: ExprCompose{Type: 4, Components: []ExpressionHandle{2, 5}}},
			},
			want: []ExpressionHandle{2, 5},
		},
		{
			// A constant whose Init points forward is malformed; it is not followed.
			name: "forward constant init",
			exprs: []Expression{
				{Kind: ExprConstant{Constant: 0}},
				flattenLit(1),
				{Kind: ExprCompose{Type: 2, Components: []ExpressionHandle{0, 1}}},
			},
			constants: []Constant{{Name: "A", Type: 1, Init: 2}},
			want:      []ExpressionHandle{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := flattenTestModule(tt.exprs, tt.constants)
			root := tt.exprs[len(tt.exprs)-1].Kind.(ExprCompose)
			got := FlattenCompose(module, root.Type, root.Components)
			if !slices.Equal(got, tt.want) {
				t.Errorf("FlattenCompose = %v, want %v", got, tt.want)
			}
		})
	}
}
