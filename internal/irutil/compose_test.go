package irutil

import (
	"slices"
	"testing"

	"github.com/gogpu/naga/ir"
)

// flattenTestModule builds the arenas the FlattenCompose cases share.
// Type handles: 0 = f32, 1 = vec2<f32>, 2 = vec3<f32>, 3 = vec4<f32>,
// 4 = mat2x2<f32>.
func flattenTestModule(exprs []ir.Expression, constants []ir.Constant) *ir.Module {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	return &ir.Module{
		Types: []ir.Type{
			{Inner: f32},
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}},
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}},
			{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32}},
		},
		Constants:         constants,
		GlobalExpressions: exprs,
	}
}

func flattenLit(v float32) ir.Expression {
	return ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(v)}}
}

func TestFlattenCompose(t *testing.T) {
	tests := []struct {
		name      string
		exprs     []ir.Expression
		constants []ir.Constant
		want      []ir.ExpressionHandle
	}{
		{
			// vec4(vec3(vec2(6, 7), 8), 9): two levels of nested Compose.
			name: "nested compose",
			exprs: []ir.Expression{
				flattenLit(6), flattenLit(7),
				{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
				flattenLit(8),
				{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{2, 3}}},
				flattenLit(9),
				{Kind: ir.ExprCompose{Type: 3, Components: []ir.ExpressionHandle{4, 5}}},
			},
			want: []ir.ExpressionHandle{0, 1, 3, 5},
		},
		{
			// vec4(vec3(1.0), 2.0): a Splat yields its value once per lane.
			name: "splat",
			exprs: []ir.Expression{
				flattenLit(1),
				{Kind: ir.ExprSplat{Size: ir.Vec3, Value: 0}},
				flattenLit(2),
				{Kind: ir.ExprCompose{Type: 3, Components: []ir.ExpressionHandle{1, 2}}},
			},
			want: []ir.ExpressionHandle{0, 0, 0, 2},
		},
		{
			// const A = vec2(1, 2); vec3(A, 3): a vector Constant expands to its Init.
			name: "vector constant",
			exprs: []ir.Expression{
				flattenLit(1), flattenLit(2),
				{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
				{Kind: ir.ExprConstant{Constant: 0}},
				flattenLit(3),
				{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{3, 4}}},
			},
			constants: []ir.Constant{{Name: "A", Type: 1, Init: 2}},
			want:      []ir.ExpressionHandle{0, 1, 4},
		},
		{
			// A scalar Constant is a lane of its own.
			name: "scalar constant",
			exprs: []ir.Expression{
				flattenLit(1),
				{Kind: ir.ExprConstant{Constant: 0}},
				{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{1, 1}}},
			},
			constants: []ir.Constant{{Name: "S", Type: 0, Init: 0}},
			want:      []ir.ExpressionHandle{1, 1},
		},
		{
			// A vector ZeroValue has no scalar handles; the caller expands it.
			name: "vector zero value",
			exprs: []ir.Expression{
				{Kind: ir.ExprZeroValue{Type: 1}},
				flattenLit(1), flattenLit(2),
				{Kind: ir.ExprCompose{Type: 3, Components: []ir.ExpressionHandle{0, 1, 2}}},
			},
			want: []ir.ExpressionHandle{0, 1, 2},
		},
		{
			// A matrix keeps its columns: only vectors flatten.
			name: "matrix columns",
			exprs: []ir.Expression{
				flattenLit(1), flattenLit(2),
				{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
				flattenLit(3), flattenLit(4),
				{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{3, 4}}},
				{Kind: ir.ExprCompose{Type: 4, Components: []ir.ExpressionHandle{2, 5}}},
			},
			want: []ir.ExpressionHandle{2, 5},
		},
		{
			// A constant whose Init points forward is malformed; it is not followed.
			name: "forward constant init",
			exprs: []ir.Expression{
				{Kind: ir.ExprConstant{Constant: 0}},
				flattenLit(1),
				{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{0, 1}}},
			},
			constants: []ir.Constant{{Name: "A", Type: 1, Init: 2}},
			want:      []ir.ExpressionHandle{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := flattenTestModule(tt.exprs, tt.constants)
			root := tt.exprs[len(tt.exprs)-1].Kind.(ir.ExprCompose)
			got := FlattenCompose(module, root.Type, root.Components)
			if !slices.Equal(got, tt.want) {
				t.Errorf("FlattenCompose = %v, want %v", got, tt.want)
			}
		})
	}
}
