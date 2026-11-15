// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestWriteSelectExpression_Forms pins the two forms of Expression::Select in
// HLSL. Rust naga writes a C++ ternary for every select
// (back/hlsl/writer.rs:4622-4634), but dxc rejects a ternary whose condition is
// a vector ("condition for short-circuiting ternary operator must be scalar"),
// so a vector condition must become HLSL's component-wise select() intrinsic,
// select(condition, accept, reject).
func TestWriteSelectExpression_Forms(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},  // 1: bool
			{Inner: ir.VectorType{ // 2: vec4<f32>
				Size:   ir.Vec4,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
		},
	}

	// Both select forms share one function: the scalar condition is expression
	// 0 (a bool literal), the vector condition is expression 5 (a vec4<f32>
	// comparison, hence a vec4<bool>). Accept is the literal 1.0, reject 0.0,
	// so the emitted argument order is visible in the output.
	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},                                // 0: condition (scalar)
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},                                  // 1: accept
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},                                  // 2: reject
			{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{1, 1, 1, 1}}}, // 3: vec4<f32>
			{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{2, 2, 2, 2}}}, // 4: vec4<f32>
			{Kind: ir.ExprBinary{ // 5: vec4<f32> < vec4<f32> -> vec4<bool>
				Op:    ir.BinaryLess,
				Left:  3,
				Right: 4,
			}},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	tests := []struct {
		name string
		expr ir.ExprSelect
		want []string
		deny []string
	}{
		{
			name: "scalar condition stays a ternary",
			expr: ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2},
			want: []string{"(true ? 1.0 : 0.0)"},
			deny: []string{"select("},
		},
		{
			name: "vector condition uses the select intrinsic",
			expr: ir.ExprSelect{Condition: 5, Accept: 3, Reject: 4},
			// HLSL 2021's select(condition, accept, reject): dxc checks the
			// first operand as the condition (hlsl-specs proposal 0040).
			want: []string{"select((float4(1.0, 1.0, 1.0, 1.0) < float4(0.0, 0.0, 0.0, 0.0)), float4(1.0, 1.0, 1.0, 1.0), float4(0.0, 0.0, 0.0, 0.0))"},
			deny: []string{" ? "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newTestWriter(module, nil, nil)
			setCurrentFunction(t, w, fn)

			if err := w.writeSelectExpression(tt.expr); err != nil {
				t.Fatalf("writeSelectExpression: %v", err)
			}
			got := w.Out.String()

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("output %q does not contain %q", got, want)
				}
			}
			for _, deny := range tt.deny {
				if strings.Contains(got, deny) {
					t.Errorf("output %q contains %q", got, deny)
				}
			}
		})
	}
}
