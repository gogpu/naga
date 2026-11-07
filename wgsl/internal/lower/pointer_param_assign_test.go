// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package lower

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestCompoundAssignThroughPointerParameterLoads: `*p += x` and `(*p)++` with p
// a pointer parameter load the current value through p before the binary
// operation (P1-WGSL-108). The left side of an assignment is a reference, and
// Rust naga's StatementKind::Assign and Increment always apply the load rule to
// it (Typed::Reference); ours decided by the expression kind and left a
// FunctionArgument unloaded, so the Binary got the pointer itself and the
// validator refused it ("Operation Add can't work with Pointer"). Rust naga
// 14328f4 compiles these shaders.
func TestCompoundAssignThroughPointerParameterLoads(t *testing.T) {
	for _, src := range []string{
		`fn f(p: ptr<function, u32>, x: u32) { *p += x; }`,
		`fn f(p: ptr<function, u32>, x: u32) { (*p) += x; }`,
		`fn f(p: ptr<function, u32>) { (*p)++; }`,
		`fn f(p: ptr<function, i32>) { (*p)--; }`,
		`var<private> q: i32; fn f(p: ptr<private, i32>, x: i32) { *p *= x; }`,
	} {
		module := mustCompile(t, src)
		var fn *ir.Function
		for i := range module.Functions {
			if module.Functions[i].Name == "f" {
				fn = &module.Functions[i]
			}
		}
		if fn == nil {
			t.Fatalf("%s: function f not found", src)
		}
		found := false
		for _, e := range fn.Expressions {
			bin, ok := e.Kind.(ir.ExprBinary)
			if !ok {
				continue
			}
			found = true
			load, ok := fn.Expressions[bin.Left].Kind.(ir.ExprLoad)
			if !ok {
				t.Errorf("%s: left operand of %v is %T, want a Load through the parameter", src, bin.Op, fn.Expressions[bin.Left].Kind)
				continue
			}
			if _, ok := fn.Expressions[load.Pointer].Kind.(ir.ExprFunctionArgument); !ok {
				t.Errorf("%s: Load pointer is %T, want the FunctionArgument", src, fn.Expressions[load.Pointer].Kind)
			}
		}
		if !found {
			t.Errorf("%s: no Binary expression", src)
		}
	}
}
