package entrypoint

import (
	"errors"
	"slices"
	"testing"

	"github.com/gogpu/naga/backend"
	"github.com/gogpu/naga/ir"
)

// module builds, without a frontend:
//
//	globals: g0, g1
//	f0: reads g0, dpdx           (fragment only)
//	f1: workgroupBarrier          (compute-like only)
//	f2: calls f0                  (fragment only, uses g0)
//	f3: reads g1                  (any stage, uses g1)
//	f4: kill                      (fragment only)
//	entry points: 0 vs (vertex, empty), 1 fs (fragment, calls f2), 2 cs (compute, calls f1, reads g1)
func module() *ir.Module {
	call := func(f ir.FunctionHandle) ir.Statement { return ir.Statement{Kind: ir.StmtCall{Function: f}} }
	global := func(g ir.GlobalVariableHandle) ir.Expression {
		return ir.Expression{Kind: ir.ExprGlobalVariable{Variable: g}}
	}
	return &ir.Module{
		GlobalVariables: []ir.GlobalVariable{{Name: "g0"}, {Name: "g1"}},
		Functions: []ir.Function{
			{Name: "f0", Expressions: []ir.Expression{global(0), {Kind: ir.ExprDerivative{}}}},
			{Name: "f1", Body: ir.Block{{Kind: ir.StmtControlBarrier{}}}},
			{Name: "f2", Body: ir.Block{{Kind: ir.StmtIf{Accept: ir.Block{call(0)}}}}},
			{Name: "f3", Expressions: []ir.Expression{global(1)}},
			{Name: "f4", Body: ir.Block{{Kind: ir.StmtLoop{Body: ir.Block{{Kind: ir.StmtKill{}}}}}}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "vs", Stage: ir.StageVertex},
			{Name: "fs", Stage: ir.StageFragment, Function: ir.Function{Body: ir.Block{call(2)}}},
			{Name: "cs", Stage: ir.StageCompute, Function: ir.Function{
				Expressions: []ir.Expression{global(1)},
				Body:        ir.Block{call(1)},
			}},
		},
	}
}

func TestIndex(t *testing.T) {
	m := module()
	for _, tt := range []struct {
		name    string
		sel     *backend.EntryPointSelector
		single  bool
		want    int
		wantErr bool
	}{
		{"nil, every entry point", nil, false, All, false},
		{"nil, one output, three entry points", nil, true, 0, true},
		{"by stage and name", &backend.EntryPointSelector{Stage: ir.StageCompute, Name: "cs"}, true, 2, false},
		{"name of another stage", &backend.EntryPointSelector{Stage: ir.StageVertex, Name: "cs"}, false, 0, true},
		{"missing", &backend.EntryPointSelector{Stage: ir.StageVertex, Name: "nope"}, false, 0, true},
	} {
		got, err := Index(m, tt.sel, tt.single)
		if tt.wantErr {
			if !errors.Is(err, backend.ErrEntryPointNotFound) {
				t.Errorf("%s: err = %v, want backend.ErrEntryPointNotFound", tt.name, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("%s: Index = %d, %v; want %d", tt.name, got, err, tt.want)
		}
	}
	one := module()
	one.EntryPoints = one.EntryPoints[:1]
	if got, err := Index(one, nil, true); err != nil || got != 0 {
		t.Errorf("nil, one output, one entry point: Index = %d, %v; want 0", got, err)
	}
}

func TestSelect(t *testing.T) {
	m := module()
	got, err := Select(m, &backend.EntryPointSelector{Stage: ir.StageFragment, Name: "fs"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.EntryPoints) != 1 || got.EntryPoints[0].Name != "fs" {
		t.Errorf("Select kept %v", got.EntryPoints)
	}
	if len(m.EntryPoints) != 3 {
		t.Errorf("Select changed the caller's module: %d entry points", len(m.EntryPoints))
	}
	if all, _ := Select(m, nil, false); all != m {
		t.Error("Select(nil) did not return the module itself")
	}
}

func TestFilter(t *testing.T) {
	m := module()
	for _, tt := range []struct {
		ep        int
		globals   []bool
		functions []bool
	}{
		// vs uses nothing and is available everywhere: only f3 would need a
		// global it lacks, and every stage-bound function is left out.
		{0, []bool{false, false}, []bool{false, false, false, false, false}},
		// fs (fragment only, uses g0 through f2 → f0): f0, f2, f4 are
		// fragment-available and use no global fs lacks; f1 is compute-only,
		// f3 uses g1.
		{1, []bool{true, false}, []bool{true, false, true, false, true}},
		// cs (compute-like, uses g1): f1 and f3.
		{2, []bool{false, true}, []bool{false, true, false, true, false}},
	} {
		f := NewFilter(m, tt.ep)
		if !slices.Equal(f.Globals, tt.globals) {
			t.Errorf("%s: Globals = %v, want %v", m.EntryPoints[tt.ep].Name, f.Globals, tt.globals)
		}
		if !slices.Equal(f.Functions, tt.functions) {
			t.Errorf("%s: Functions = %v, want %v", m.EntryPoints[tt.ep].Name, f.Functions, tt.functions)
		}
	}
}
