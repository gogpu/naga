package dxil

import (
	"testing"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl"
)

// TestZeroInitWorkgroupMemory checks that the prologue stores zero to the
// workgroup variables the entry point uses, and only those, in IR that
// passes validation and compiles.
func TestZeroInitWorkgroupMemory(t *testing.T) {
	src := `
struct S { a: vec4<f32>, b: array<i32, 3> }
var<workgroup> small: array<u32, 64>;
var<workgroup> s: S;
var<workgroup> counter: atomic<u32>;
var<workgroup> unused: u32;
@group(0) @binding(0) var<storage, read_write> out: array<u32>;
@compute @workgroup_size(64)
fn main(@builtin(local_invocation_index) i: u32) {
	atomicAdd(&counter, 1u);
	out[i] = small[i] + u32(s.b[i % 3u]) + atomicLoad(&counter);
}
`
	tokens, err := wgsl.NewLexer(src).Tokenize()
	if err != nil {
		t.Fatal(err)
	}
	ast, err := wgsl.NewParser(tokens).Parse()
	if err != nil {
		t.Fatal(err)
	}
	m, err := wgsl.LowerWithSource(ast, src)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(m, DefaultOptions()); err != nil {
		t.Fatalf("compile: %v", err)
	}

	ep := &m.EntryPoints[0]
	zeroInitWorkgroupMemory(m, ep)
	if errs, err := ir.Validate(m); err != nil || len(errs) > 0 {
		t.Fatalf("validate: %v %v", err, errs)
	}

	ifStmt, ok := ep.Function.Body[1].Kind.(ir.StmtIf)
	if !ok {
		t.Fatalf("body[1] is %T, want the If of invocation 0", ep.Function.Body[1].Kind)
	}
	if _, ok := ep.Function.Body[2].Kind.(ir.StmtBarrier); !ok {
		t.Fatalf("body[2] is %T, want a barrier", ep.Function.Body[2].Kind)
	}
	stored := map[string]int{}
	var walk func(ir.Block)
	walk = func(b ir.Block) {
		for _, st := range b {
			switch k := st.Kind.(type) {
			case ir.StmtStore:
				if root, ok := workgroupRoot(m, &ep.Function, k.Pointer); ok {
					stored[root]++
				}
			case ir.StmtLoop:
				walk(k.Body)
			}
		}
	}
	walk(ifStmt.Accept)
	// small: one store in its loop; s: a, and b's loop; counter: one.
	want := map[string]int{"small": 1, "s": 2, "counter": 1}
	if len(stored) != len(want) {
		t.Errorf("stores to %v, want %v", stored, want)
	}
	for name, n := range want {
		if stored[name] != n {
			t.Errorf("%d stores to %s, want %d", stored[name], name, n)
		}
	}
}

// workgroupRoot returns the name of the workgroup variable pointer h points
// into.
func workgroupRoot(m *ir.Module, fn *ir.Function, h ir.ExpressionHandle) (string, bool) {
	for {
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprGlobalVariable:
			gv := m.GlobalVariables[k.Variable]
			return gv.Name, gv.Space == ir.SpaceWorkGroup
		case ir.ExprAccess:
			h = k.Base
		case ir.ExprAccessIndex:
			h = k.Base
		default:
			return "", false
		}
	}
}
