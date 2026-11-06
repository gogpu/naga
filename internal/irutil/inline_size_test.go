package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// leafFunction has two expressions and a return: size 3.
func leafFunction() ir.Function {
	return ir.Function{
		Name:        "leaf",
		Expressions: make([]ir.Expression, 2),
		Body:        ir.Block{{Kind: ir.StmtReturn{}}},
	}
}

// callerOf calls callee twice, once inside an if: one expression and the
// statements call, if, call, return.
func callerOf(name string, callee ir.FunctionHandle) ir.Function {
	return ir.Function{
		Name:        name,
		Expressions: make([]ir.Expression, 1),
		Body: ir.Block{
			{Kind: ir.StmtCall{Function: callee}},
			{Kind: ir.StmtIf{Accept: ir.Block{{Kind: ir.StmtCall{Function: callee}}}}},
			{Kind: ir.StmtReturn{}},
		},
	}
}

// chainModule is a module of a leaf and n callers, each calling the one
// before twice, and an entry point calling the last once.
func chainModule(n int) *ir.Module {
	m := &ir.Module{Functions: []ir.Function{leafFunction()}}
	for k := 1; k <= n; k++ {
		m.Functions = append(m.Functions, callerOf("f", ir.FunctionHandle(k-1)))
	}
	m.EntryPoints = []ir.EntryPoint{{
		Name: "main",
		Function: ir.Function{Body: ir.Block{
			{Kind: ir.StmtCall{Function: ir.FunctionHandle(n)}},
		}},
	}}
	return m
}

func TestInlinedEntryPointSizes(t *testing.T) {
	// size(leaf) = 3; size(f_k) = 1 + 4 + 2*size(f_{k-1}); entry = 1 + size(f_n).
	// f1 = 11, f2 = 27, f3 = 59.
	tests := []struct {
		name  string
		mod   *ir.Module
		limit int
		want  int
	}{
		{"leaf only", chainModule(0), 1 << 20, 4},
		{"chain of 3", chainModule(3), 1 << 20, 60},
		{"saturates at the limit", chainModule(3), 50, 50},
		// 2^200 copies: the count must saturate, never wrap around.
		{"deep chain saturates", chainModule(200), 1 << 30, 1 << 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InlinedEntryPointSizes(tt.mod, tt.limit)
			if len(got) != 1 || got[0] != tt.want {
				t.Fatalf("InlinedEntryPointSizes = %v, want [%d]", got, tt.want)
			}
		})
	}
}

// A call cycle cannot be inlined: it counts as the limit, and the sizing
// itself terminates.
func TestInlinedEntryPointSizesCycle(t *testing.T) {
	m := &ir.Module{Functions: []ir.Function{
		{Name: "a", Body: ir.Block{{Kind: ir.StmtCall{Function: 1}}}},
		{Name: "b", Body: ir.Block{{Kind: ir.StmtCall{Function: 0}}}},
	}}
	m.EntryPoints = []ir.EntryPoint{{Name: "main", Function: ir.Function{
		Body: ir.Block{{Kind: ir.StmtCall{Function: 0}}},
	}}}
	if got := InlinedEntryPointSizes(m, 1000); got[0] != 1000 {
		t.Fatalf("InlinedEntryPointSizes(cycle) = %v, want [1000]", got)
	}
}

func TestInlinedEntryPointSizesNil(t *testing.T) {
	if got := InlinedEntryPointSizes(nil, 10); got != nil {
		t.Fatalf("InlinedEntryPointSizes(nil) = %v, want nil", got)
	}
}
