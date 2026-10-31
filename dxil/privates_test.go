// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// TestPrivateGlobalsBecomeEntryLocals (P0-DXIL-059): a var<private> is one
// variable per invocation, so after inlining (every helper that touches a
// global is inlined) it is a local of the entry point, as Mesa's
// nir_lower_global_vars_to_local makes it before nir_lower_vars_to_ssa
// (radv_shader.c). The emitter gave a private vec2 one scalar alloca: the y
// component was lost (VALID per IDxcValidator), then refused at compile time.
func TestPrivateGlobalsBecomeEntryLocals(t *testing.T) {
	tests := []struct {
		name, decl, body string
		wantAdds         int // i32 adds the shader must keep
	}{
		{"vec2", "var<private> acc: vec2<i32>;",
			"acc = vec2<i32>(dst[0], dst[1]); acc = acc + vec2<i32>(1, 1); dst[0] = acc.x; dst[1] = acc.y;", 2},
		{"vec2 with an initializer", "var<private> acc = vec2<i32>(3, 4);",
			"acc = acc + vec2<i32>(dst[0], dst[1]); dst[0] = acc.x; dst[1] = acc.y;", 2},
		{"struct", "struct S { a: i32, b: vec2<i32> }\nvar<private> s: S;",
			"s.a = dst[0]; s.b = vec2<i32>(dst[1], dst[2]); s.b = s.b + vec2<i32>(s.a, s.a); dst[0] = s.b.x; dst[1] = s.b.y;", 2},
		{"array", "var<private> arr: array<i32, 4>;",
			"arr[dst[0]] = dst[1]; arr[1] = arr[1] + dst[2]; dst[0] = arr[0] + arr[1];", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := "@group(0) @binding(0) var<storage, read_write> dst: array<i32>;\n" + tt.decl +
				"\n@compute @workgroup_size(1) fn main() {\n" + tt.body + "\n}"
			irMod, err := prepareModule(lowerSource(t, src))
			if err != nil {
				t.Fatal(err)
			}
			fn := &irMod.EntryPoints[0].Function
			for h, e := range fn.Expressions {
				if g, ok := e.Kind.(ir.ExprGlobalVariable); ok && irMod.GlobalVariables[g.Variable].Space == ir.SpacePrivate {
					t.Errorf("expression %d still names private global %q", h, irMod.GlobalVariables[g.Variable].Name)
				}
			}
			_, mfn := emitMainModule(t, src)
			adds := 0
			for _, bb := range mfn.BasicBlocks {
				for _, in := range bb.Instructions {
					if in.Kind == module.InstrBinOp && in.ResultType != nil && in.ResultType.Kind == module.TypeInteger {
						adds++
					}
				}
			}
			if adds < tt.wantAdds {
				t.Errorf("integer binary ops = %d, want at least %d (a component was dropped)", adds, tt.wantAdds)
			}
			compileWGSLToDXIL(t, src)
		})
	}
}
