// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"
	"slices"
	"testing"

	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/valid"
)

// splitLocalsModule lowers src and runs splitLocalVariables on its entry
// point, after the passes Compile runs before it, without index clamps:
// restrictIndices emits its clamps where the emitter takes them, not where
// the IR validator wants them in scope.
func splitLocalsModule(t *testing.T, src string) *ir.Module {
	t.Helper()
	m, err := prepareModule(lowerSource(t, src))
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := prepareEntryPoint(m, &m.EntryPoints[0], DefaultOptions()); err != nil {
		t.Fatalf("prepare entry point: %v", err)
	}
	if err := splitLocalVariables(m, &m.EntryPoints[0].Function); err != nil {
		t.Fatalf("split: %v", err)
	}
	return m
}

// localTypeString names a local's type for the tests: "f32", "vec2<f32>",
// "array<f32, 6>".
func localTypeString(m *ir.Module, ty ir.TypeHandle) string {
	switch t := m.Types[ty].Inner.(type) {
	case ir.ScalarType:
		prefix := map[ir.ScalarKind]string{ir.ScalarSint: "i", ir.ScalarUint: "u", ir.ScalarFloat: "f", ir.ScalarBool: "bool"}[t.Kind]
		if t.Kind == ir.ScalarBool {
			return prefix
		}
		return fmt.Sprintf("%s%d", prefix, t.Width*8)
	case ir.VectorType:
		return fmt.Sprintf("vec%d<%s>", t.Size, localTypeString(m, typeHandle(m, t.Scalar)))
	case ir.ArrayType:
		return fmt.Sprintf("array<%s, %d>", localTypeString(m, t.Base), *t.Size.Constant)
	}
	return fmt.Sprintf("%T", m.Types[ty].Inner)
}

// usedLocals returns "name: type" of the locals that an expression an Emit
// covers, or a statement, still refers to, sorted.
func usedLocals(m *ir.Module, fn *ir.Function) []string {
	seen := map[ir.LocalVariableHandle]bool{}
	mark := func(h ir.ExpressionHandle) {
		if lv, ok := fn.Expressions[h].Kind.(ir.ExprLocalVariable); ok {
			seen[lv.Variable] = true
		}
	}
	blockUses(fn.Body, func(h ir.ExpressionHandle) {
		mark(h)
		irutil.ExprHandles(fn.Expressions[h].Kind, mark)
	})
	var out []string
	for h := range seen {
		out = append(out, fn.LocalVars[h].Name+": "+localTypeString(m, fn.LocalVars[h].Type))
	}
	slices.Sort(out)
	return out
}

// The pieces each local type is split into, as DXC's SROA and
// DynamicIndexingVectorToArray / MultiDimArrayToOneDimArray leave them:
// one local per struct member, below arrays of structs one array per
// member, vectors whole, and everything else one array of its scalars.
func TestSplitLocalPieces(t *testing.T) {
	const prelude = `
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
@group(0) @binding(1) var<storage, read> idx: array<u32, 4>;
struct V { p: vec2<f32>, k: f32 }
struct N { v: V, a: array<f32, 2> }
`
	tests := []struct {
		name, body string
		want       []string
	}{
		{"array of vectors", `
    var p = array<vec2<f32>, 3>(vec2(1.0), vec2(2.0), vec2(3.0));
    data[0] = p[idx[0] % 3u].y;`, []string{"p.v.1dim: array<f32, 6>"}},
		{"struct of a vector and a scalar", `
    var s = V(vec2(1.0, 2.0), 3.0);
    s.p.y = data[1];
    data[0] = s.p[idx[0] % 2u] + s.k;`, []string{"s.0: vec2<f32>", "s.1: f32"}},
		{"array of structs", `
    var vs: array<V, 4>;
    vs[idx[0] % 4u].p.x = 1.0;
    data[0] = vs[idx[1] % 4u].p.x + vs[2].k;`, []string{"vs.0.v.1dim: array<f32, 8>", "vs.1: array<f32, 4>"}},
		{"nested struct with an array member", `
    var n: N;
    n.a[idx[0] % 2u] = 1.0;
    n.v.k = 2.0;
    data[0] = n.a[1] + n.v.k + n.v.p.x;`, []string{"n.0.0: vec2<f32>", "n.0.1: f32", "n.1: array<f32, 2>"}},
		{"matrix", `
    var m = mat2x2<f32>(data[0], 2.0, 3.0, 4.0);
    data[1] = m[1].y + m[idx[0] % 2u].x;`, []string{"m.v.v: array<f32, 4>"}},
		{"array of arrays of scalars", `
    var xs: array<array<f32, 3>, 2>;
    xs[idx[0] % 2u][idx[1] % 3u] = 1.0;
    data[0] = xs[1][2];`, []string{"xs.1dim: array<f32, 6>"}},
		{"vector and array of scalars stay whole", `
    var v = vec2<f32>(data[0], 1.0);
    var a: array<f32, 3>;
    a[idx[0] % 3u] = v[idx[1] % 2u];
    data[0] = a[1];`, []string{"a: array<f32, 3>", "v: vec2<f32>"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := splitLocalsModule(t, prelude+"@compute @workgroup_size(1) fn main() {"+tt.body+"\n}\n")
			fn := &m.EntryPoints[0].Function
			got := usedLocals(m, fn)
			want := slices.Clone(tt.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("locals in use = %v, want %v", got, want)
			}
			if _, err := valid.Validate(m, valid.Options{Capabilities: valid.CapAll}); err != nil {
				t.Fatalf("validate: %v", err)
			}
		})
	}
}

// A local a pointer of which is used other than by an access, a load or a
// store is left whole: the pieces could not stand in for it there.
func TestSplitLocalLeavesOtherUsesWhole(t *testing.T) {
	m := lowerSource(t, `
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
struct V { p: vec2<f32>, k: f32 }
@compute @workgroup_size(1) fn main() {
    var s = V(vec2(1.0, 2.0), 3.0);
    data[0] = s.k;
}
`)
	fn := &m.EntryPoints[0].Function
	// A statement that takes the local's pointer as an operand, as a call
	// with a pointer argument does.
	var lvPtr ir.ExpressionHandle
	for h, e := range fn.Expressions {
		if _, ok := e.Kind.(ir.ExprLocalVariable); ok {
			lvPtr = ir.ExpressionHandle(h)
		}
	}
	fn.Body = append(ir.Block{{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{lvPtr}}}}, fn.Body...)
	if got := localSplitCandidates(m, fn); len(got) != 0 {
		t.Fatalf("candidates = %v, want none", got)
	}
}

// The initializer of a split local goes to its pieces: a constant one as the
// pieces' own initializers, so that a piece never written keeps its value.
func TestSplitLocalInitializers(t *testing.T) {
	m := splitLocalsModule(t, `
@group(0) @binding(0) var<storage, read_write> data: array<f32, 4>;
@group(0) @binding(1) var<storage, read> idx: array<u32, 4>;
struct V { p: vec2<f32>, k: f32 }
@compute @workgroup_size(1) fn main() {
    var vs = array<V, 2>(V(vec2(1.0, 2.0), 3.0), V(vec2(4.0, 5.0), 6.0));
    data[0] = vs[idx[0] % 2u].p.y + vs[idx[1] % 2u].k;
}
`)
	fn := &m.EntryPoints[0].Function
	inits := map[string][]float64{}
	for _, lv := range fn.LocalVars {
		if lv.Init == nil || lv.Name == "vs" {
			continue
		}
		compose, ok := fn.Expressions[*lv.Init].Kind.(ir.ExprCompose)
		if !ok {
			continue
		}
		for _, c := range compose.Components {
			lit, ok := fn.Expressions[c].Kind.(ir.Literal)
			if !ok {
				t.Fatalf("%s: component %T, want a literal", lv.Name, fn.Expressions[c].Kind)
			}
			inits[lv.Name] = append(inits[lv.Name], float64(lit.Value.(ir.LiteralF32)))
		}
	}
	want := map[string][]float64{"vs.0.v.1dim": {1, 2, 4, 5}, "vs.1": {3, 6}}
	for name, w := range want {
		if !slices.Equal(inits[name], w) {
			t.Errorf("%s init = %v, want %v", name, inits[name], w)
		}
	}
}
