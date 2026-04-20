// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package emit

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestCanInlineCallee pins the Phase 1 inline eligibility gate, which is
// deliberately NARROW: inline only helpers that were excluded from
// emitHelperFunctions specifically due to the vector / aggregate return
// restriction (DXIL insertvalue/extractvalue ban). Every other class that
// routes through the `helperFunctions` miss (void helpers, global-accessing
// helpers, complex-locals helpers) must fall back to the zero path — their
// standalone emission is broken for reasons the inliner cannot fix at
// emission time.
//
// See inline.go canInlineCallee for the full rationale vs DXC's post-emit
// AlwaysInliner pass.
func TestCanInlineCallee(t *testing.T) {
	// Build a minimal ir.Module with the types we need:
	//   [0] f32
	//   [1] vec3<f32>
	//   [2] bool
	f32Ty := ir.TypeHandle(0)
	vec3Ty := ir.TypeHandle(1)
	boolTy := ir.TypeHandle(2)
	mod := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "vec3f", Inner: ir.VectorType{
				Size:   3,
				Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4},
			}},
			{Name: "bool", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}
	e := &Emitter{ir: mod}

	retH := ir.ExpressionHandle(0)

	tests := []struct {
		name string
		fn   ir.Function
		want bool
	}{
		{
			name: "vec3 return single trailing return eligible",
			fn: ir.Function{
				Name:   "permute3_like",
				Result: &ir.FunctionResult{Type: vec3Ty},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
					{Kind: ir.StmtReturn{Value: &retH}},
				},
			},
			want: true,
		},
		{
			name: "void helper NOT eligible (zero-fallback no-op is correct)",
			fn: ir.Function{
				Name: "barriers_like",
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				},
			},
			want: false,
		},
		{
			name: "scalar return NOT eligible (should route through helperFunctions)",
			fn: ir.Function{
				Name:   "scalar_helper",
				Result: &ir.FunctionResult{Type: f32Ty},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retH}},
				},
			},
			want: false,
		},
		{
			name: "bool return NOT eligible (i1 returns cause Invalid record)",
			fn: ir.Function{
				Name:   "bool_helper",
				Result: &ir.FunctionResult{Type: boolTy},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retH}},
				},
			},
			want: false,
		},
		{
			name: "nested StmtCall in vec3 body rejected in Phase 1",
			fn: ir.Function{
				Name:   "nested_caller",
				Result: &ir.FunctionResult{Type: vec3Ty},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0}},
					{Kind: ir.StmtReturn{Value: &retH}},
				},
			},
			want: false,
		},
		{
			name: "non-trailing return rejected in Phase 1",
			fn: ir.Function{
				Name:   "missing_trailing_return",
				Result: &ir.FunctionResult{Type: vec3Ty},
				Body: []ir.Statement{
					{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}}},
				},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fn := tc.fn
			got := e.canInlineCallee(&fn)
			if got != tc.want {
				t.Errorf("canInlineCallee() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCanInlineCalleeNil verifies the explicit nil guard that protects
// against a corrupt FunctionHandle in StmtCall.
func TestCanInlineCalleeNil(t *testing.T) {
	e := &Emitter{ir: &ir.Module{}}
	if e.canInlineCallee(nil) {
		t.Error("canInlineCallee(nil) = true, want false")
	}
}
