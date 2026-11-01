// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"
	"maps"
	"testing"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// workgroupCompositeCases are workgroup variables of every composite shape a
// compute shader keeps in shared memory. DXC scalarizes groupshared memory:
// a vector or an array of vectors is one flat scalar array, a struct is one
// global per member (P1-DXIL-066).
var workgroupCompositeCases = []struct{ name, decls, body string }{
	{"vec2", `var<workgroup> v: vec2<u32>;`,
		`v.y = dst[0]; v.x = v.y + 1u; let w = v; dst[1] = w.x + w.y;`},
	{"vec4 whole", `var<workgroup> v: vec4<f32>;`,
		`v = vec4<f32>(1.0, 2.0, 3.0, f32(dst[0])); dst[1] = u32(v.w + v.x);`},
	{"vec3 dynamic component", `var<workgroup> v: vec3<u32>;`,
		`v = vec3<u32>(1u, 2u, 3u); v[dst[0]] = 7u; dst[1] = v[dst[2]];`},
	{"array of vec2", `var<workgroup> a: array<vec2<u32>, 4>;`,
		`a[1] = vec2<u32>(dst[0], 2u); a[dst[1]].y = 5u; dst[2] = a[1].x + a[dst[3]].y;`},
	{"array of vec4 by invocation", `var<workgroup> tile: array<vec4<f32>, 4>;`,
		`tile[i] = vec4<f32>(f32(i)); workgroupBarrier(); let t = tile[3u - i]; dst[i] = u32(t.x + t.w);`},
	{"array of vec4 component loop", `var<workgroup> tile: array<vec4<f32>, 4>;`,
		`for (var k = 0u; k < 4u; k++) { tile[k][i] = f32(k); } workgroupBarrier(); dst[i] = u32(tile[i].z);`},
	{"nested array of vec2", `var<workgroup> g: array<array<vec2<u32>, 2>, 3>;`,
		`g[i % 3u][1] = vec2<u32>(i, i + 1u); workgroupBarrier(); dst[i] = g[2][1].y + g[dst[0]][dst[1]].x;`},
	{"mat2x2", `var<workgroup> m: mat2x2<f32>;`,
		`m = mat2x2<f32>(1.0, 2.0, 3.0, f32(dst[0])); m[1].x = 5.0; let c = m[dst[1]]; dst[2] = u32(c.y + m[0][1]);`},
	{"struct of scalars", `struct S { a: u32, b: u32 }
var<workgroup> ws: S;`,
		`ws.a = dst[0]; dst[1] = ws.a + ws.b;`},
	{"struct with a vector member", `struct S { a: u32, b: vec2<u32> }
var<workgroup> ws: S;`,
		`ws.b.y = dst[0]; ws.b = ws.b + vec2<u32>(1u); dst[1] = ws.a + ws.b.y;`},
	{"struct whole", `struct S { a: u32, b: vec2<u32> }
var<workgroup> ws: S;`,
		`ws = S(dst[0], vec2<u32>(2u, 3u)); let s = ws; dst[1] = s.a + s.b.x;`},
	{"array of structs", `struct S { a: u32, b: vec2<u32> }
var<workgroup> w: array<S, 4>;`,
		`w[i].a = i; w[i].b = vec2<u32>(i, i * 2u); workgroupBarrier(); let s = w[3u - i]; dst[i] = w[2].b.y + s.a + s.b.x;`},
	{"struct with an array of vectors", `struct T { n: u32, p: array<vec4<f32>, 2> }
var<workgroup> wt: T;`,
		`wt.p[i % 2u] = vec4<f32>(f32(i)); wt.n = i; workgroupBarrier(); dst[i] = u32(wt.p[1].z) + wt.n;`},
	{"atomic beside a vector", `struct C { n: atomic<u32>, v: vec2<u32> }
var<workgroup> wc: C;`,
		`atomicAdd(&wc.n, 1u); wc.v[i % 2u] = i; workgroupBarrier(); dst[i] = atomicLoad(&wc.n) + wc.v.y;`},
}

// workgroupCompositeSource is the compute shader of a workgroupCompositeCases
// case.
func workgroupCompositeSource(decls, body string) string {
	return decls + "\n@group(0) @binding(0) var<storage, read_write> dst: array<u32>;\n" +
		"@compute @workgroup_size(4) fn main(@builtin(local_invocation_index) i: u32) {\n" + body + "\n}"
}

// scalarizedModule lowers src and runs the passes before the emitter
// through the workgroup split, zeroing workgroup memory when zero is set.
func scalarizedModule(t *testing.T, src string, zero bool) *ir.Module {
	t.Helper()
	m, err := prepareModule(lowerSource(t, src))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	opts := DefaultOptions()
	opts.SkipWorkgroupMemoryZeroInit = !zero
	if err := prepareEntryPoint(m, &m.EntryPoints[0], opts); err != nil {
		t.Fatalf("prepare entry point: %v", err)
	}
	return m
}

// TestScalarizedWorkgroupIRValidates: after the split, with the zeroing
// stores, the IR still validates: the loads and stores through the pieces
// are typed and emitted before their uses.
func TestScalarizedWorkgroupIRValidates(t *testing.T) {
	for _, tt := range workgroupCompositeCases {
		t.Run(tt.name, func(t *testing.T) {
			m := scalarizedModule(t, workgroupCompositeSource(tt.decls, tt.body), true)
			if errs, err := ir.Validate(m); err != nil || len(errs) > 0 {
				t.Fatalf("validate: %v %v", err, errs)
			}
			for _, e := range emitScopeErrors(&m.EntryPoints[0].Function) {
				t.Error(e)
			}
		})
	}
}

// emitScopeErrors checks what ir.Validate does not and Rust naga's validator
// does: every expression a statement or an emitted expression uses is in
// scope, emitted before it in its block or a block around it, the result of
// a statement before it, or one that needs no Emit.
func emitScopeErrors(fn *ir.Function) []string {
	var errs []string
	check := func(scope map[ir.ExpressionHandle]bool, h ir.ExpressionHandle, user string) {
		if !scope[h] && !isPreEmit(fn.Expressions[h].Kind) {
			errs = append(errs, fmt.Sprintf("expression [%d] %T used by %s out of scope", h, fn.Expressions[h].Kind, user))
		}
	}
	var block func(ir.Block, map[ir.ExpressionHandle]bool) map[ir.ExpressionHandle]bool
	block = func(b ir.Block, outer map[ir.ExpressionHandle]bool) map[ir.ExpressionHandle]bool {
		scope := maps.Clone(outer)
		for _, st := range b {
			switch k := st.Kind.(type) {
			case ir.StmtEmit:
				for h := k.Range.Start; h < k.Range.End; h++ {
					visitExprSubHandles(fn.Expressions[h].Kind, func(sub ir.ExpressionHandle) {
						check(scope, sub, fmt.Sprintf("[%d] %T", h, fn.Expressions[h].Kind))
					})
					scope[h] = true
				}
			case ir.StmtIf:
				check(scope, k.Condition, "an if")
				block(k.Accept, scope)
				block(k.Reject, scope)
			case ir.StmtSwitch:
				check(scope, k.Selector, "a switch")
				for _, c := range k.Cases {
					block(c.Body, scope)
				}
			case ir.StmtLoop:
				inner := block(k.Continuing, block(k.Body, scope))
				if k.BreakIf != nil {
					check(inner, *k.BreakIf, "a break-if")
				}
			case ir.StmtBlock:
				block(k.Block, scope)
			default:
				for _, r := range statementResults(k) {
					scope[r] = true
				}
				walkStmtExprs(fn, k, func(_ *ir.Function, h ir.ExpressionHandle) {
					check(scope, h, fmt.Sprintf("%T", k))
				})
			}
		}
		return scope
	}
	block(fn.Body, map[ir.ExpressionHandle]bool{})
	return errs
}

// statementResults are the expressions a statement gives a value to.
func statementResults(kind ir.StatementKind) []ir.ExpressionHandle {
	switch k := kind.(type) {
	case ir.StmtAtomic:
		if k.Result != nil {
			return []ir.ExpressionHandle{*k.Result}
		}
	case ir.StmtCall:
		if k.Result != nil {
			return []ir.ExpressionHandle{*k.Result}
		}
	case ir.StmtWorkGroupUniformLoad:
		return []ir.ExpressionHandle{k.Result}
	case ir.StmtSubgroupBallot:
		return []ir.ExpressionHandle{k.Result}
	case ir.StmtSubgroupCollectiveOperation:
		return []ir.ExpressionHandle{k.Result}
	case ir.StmtSubgroupGather:
		return []ir.ExpressionHandle{k.Result}
	}
	return nil
}

// storedAddress returns the variable and the constant index of the one
// store of a literal in fn: index -1 for a store to the whole variable.
func storedAddress(t *testing.T, m *ir.Module, fn *ir.Function) (string, int) {
	t.Helper()
	var name string
	index, found := -1, 0
	var walk func(ir.Block)
	walk = func(b ir.Block) {
		for _, st := range b {
			switch k := st.Kind.(type) {
			case ir.StmtStore:
				if _, lit := fn.Expressions[k.Value].Kind.(ir.Literal); !lit {
					continue
				}
				found++
				ptr := fn.Expressions[k.Pointer].Kind
				if acc, ok := ptr.(ir.ExprAccess); ok {
					lit, ok := fn.Expressions[acc.Index].Kind.(ir.Literal)
					if !ok {
						t.Fatalf("index %T, want a literal", fn.Expressions[acc.Index].Kind)
					}
					index = int(lit.Value.(ir.LiteralU32))
					ptr = fn.Expressions[acc.Base].Kind
				}
				g, ok := ptr.(ir.ExprGlobalVariable)
				if !ok {
					t.Fatalf("store through %T, want a piece of the variable", ptr)
				}
				name = m.GlobalVariables[g.Variable].Name
			case ir.StmtBlock:
				walk(k.Block)
			}
		}
	}
	walk(fn.Body)
	if found != 1 {
		t.Fatalf("stores of a literal = %d, want 1", found)
	}
	return name, index
}

// TestScalarizedWorkgroupAddresses: a store through a pointer into a split
// variable goes to the piece DXC makes for it, under DXC's name, at the
// scalar's index in it: element index times the scalars of an element, plus
// the component.
func TestScalarizedWorkgroupAddresses(t *testing.T) {
	const s = "struct S { a: u32, b: vec2<u32> }\n"
	tests := []struct {
		name, decls, store string
		global             string
		index              int
	}{
		{"vector", "var<workgroup> v: vec2<u32>;", "v.y = 7u;", "\x01?v@@3V?$vector@I$01@@A.v", 1},
		{"array of vectors", "var<workgroup> tile: array<vec4<f32>, 4>;", "tile[3].z = 7.0;", "\x01?tile@@3PAV?$vector@M$03@@A.v.1dim", 14},
		{"matrix", "var<workgroup> m: mat2x2<f32>;", "m[1].x = 7.0;", "\x01?m@@3V?$matrix@M$01$01@@A.v.v", 2},
		{"array of arrays of vectors", "var<workgroup> g: array<array<vec2<u32>, 2>, 3>;", "g[2][1].y = 7u;", "g.v.1dim", 11},
		{"struct scalar member", s + "var<workgroup> ws: S;", "ws.a = 7u;", "\x01?ws@@3US@@A.0", -1},
		{"struct vector member", s + "var<workgroup> ws: S;", "ws.b.x = 7u;", "\x01?ws@@3US@@A.1.v", 0},
		{"array of structs, scalar", s + "var<workgroup> w: array<S, 4>;", "w[3].a = 7u;", "\x01?w@@3PAUS@@A.0", 3},
		{"array of structs, vector", s + "var<workgroup> w: array<S, 4>;", "w[2].b.y = 7u;", "\x01?w@@3PAUS@@A.1.v.1dim", 5},
		{"array of arrays of structs", s + "var<workgroup> aa: array<array<S, 2>, 3>;", "aa[2][1].b.y = 7u;", "aa.1.v.1dim", 11},
		{"struct array member", "struct T { n: u32, p: array<vec4<f32>, 2> }\nvar<workgroup> wt: T;", "wt.p[1].w = 7.0;", "\x01?wt@@3UT@@A.1.v.1dim", 7},
		{"nested structs", "struct In { x: u32, y: vec2<u32> }\nstruct Out { k: u32, i: array<In, 3> }\nvar<workgroup> wo: Out;",
			"wo.i[2].y.x = 7u;", "\x01?wo@@3UOut@@A.1.1.v.1dim", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := scalarizedModule(t, workgroupCompositeSource(tt.decls, tt.store), false)
			name, index := storedAddress(t, m, &m.EntryPoints[0].Function)
			if name != tt.global || index != tt.index {
				t.Errorf("store to %q[%d], want %q[%d]", name, index, tt.global, tt.index)
			}
		})
	}
}

// TestScalarizedWorkgroupDynamicIndex: a run-time element index is scaled by
// the scalars of an element: a[i].y is element i*2 + 1 of the flat array.
func TestScalarizedWorkgroupDynamicIndex(t *testing.T) {
	m := scalarizedModule(t, workgroupCompositeSource("var<workgroup> a: array<vec2<u32>, 4>;", "a[i].y = 7u;"), false)
	fn := &m.EntryPoints[0].Function
	for _, st := range fn.Body {
		store, ok := st.Kind.(ir.StmtStore)
		if !ok {
			continue
		}
		acc := fn.Expressions[store.Pointer].Kind.(ir.ExprAccess)
		add, ok := fn.Expressions[acc.Index].Kind.(ir.ExprBinary)
		if !ok || add.Op != ir.BinaryAdd {
			t.Fatalf("index %T, want i*2 + 1", fn.Expressions[acc.Index].Kind)
		}
		mul, ok := fn.Expressions[add.Left].Kind.(ir.ExprBinary)
		if !ok || mul.Op != ir.BinaryMultiply {
			t.Fatalf("index base %T, want i*2", fn.Expressions[add.Left].Kind)
		}
		if _, ok := fn.Expressions[mul.Left].Kind.(ir.ExprFunctionArgument); !ok {
			t.Errorf("scaled index %T, want the argument i", fn.Expressions[mul.Left].Kind)
		}
		if lit := fn.Expressions[mul.Right].Kind.(ir.Literal); lit.Value != ir.LiteralU32(2) {
			t.Errorf("scale %v, want 2", lit.Value)
		}
		if lit := fn.Expressions[add.Right].Kind.(ir.Literal); lit.Value != ir.LiteralU32(1) {
			t.Errorf("component %v, want 1", lit.Value)
		}
		return
	}
	t.Fatal("no store")
}

// TestWorkgroupVectorKeepsEveryComponent (P1-DXIL-066): a workgroup vector,
// alone or as an array element, used only whole or through a run-time
// component index, keeps every component in its global. v0.20.4 and v0.20.5
// kept one scalar per vector: a vec3 was an i32 global, array<vec4<f32>, 4>
// a [4 x float]; a whole store wrote only .x and the index was dropped,
// valid DXIL that read back the wrong values. Every access goes through a
// GEP of the array.
func TestWorkgroupVectorKeepsEveryComponent(t *testing.T) {
	tests := []struct {
		name, decls, body string
		scalars           uint64
	}{
		{"vector", "var<workgroup> v: vec3<u32>;",
			"v = vec3<u32>(1u, 2u, 3u); v[dst[0]] = 7u; dst[1] = v[dst[2]];", 3},
		{"array of vectors", "var<workgroup> tile: array<vec4<f32>, 4>;",
			"tile[i] = vec4<f32>(f32(i)); workgroupBarrier(); let t = tile[3u - i]; dst[i] = u32(t[dst[0]]);", 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mod, fn := emitMainModule(t, workgroupCompositeSource(tt.decls, tt.body))
			var gv *module.GlobalVar
			for _, g := range mod.GlobalVars {
				if g.AddrSpace == 3 {
					gv = g
				}
			}
			if gv == nil {
				t.Fatal("no workgroup global")
			}
			if gv.VarType.Kind != module.TypeArray || uint64(gv.VarType.ElemCount) != tt.scalars {
				t.Fatalf("workgroup global is a %v, want an array of %d scalars", gv.VarType, tt.scalars)
			}
			for _, bb := range fn.BasicBlocks {
				for _, instr := range bb.Instructions {
					if (instr.Kind == module.InstrStore || instr.Kind == module.InstrLoad) && instr.Operands[0] == gv.ValueID {
						t.Errorf("a %v of the whole workgroup array, want each through a GEP", instr.Kind)
					}
				}
			}
		})
	}
}

// TestWorkgroupComposites (P1-DXIL-066): every workgroup composite compiles
// to DXIL that IDxcValidator accepts. Vectors were refused ("has no value for
// component"), an array of structs was refused, and a struct of scalars gave
// a malformed record IDxcValidator rejected or crashed on.
func TestWorkgroupComposites(t *testing.T) {
	for _, tt := range workgroupCompositeCases {
		t.Run(tt.name, func(t *testing.T) {
			blob, err := Compile(lowerSource(t, workgroupCompositeSource(tt.decls, tt.body)), DefaultOptions())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if err := Validate(blob, ValidateFull); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}
