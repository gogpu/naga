package typifier

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestGrowArenaTypesAnOutsideArena checks the entry point a frontend uses to
// type module-scope expressions it is still lowering (the GLSL frontend's
// const context): the arena belongs to no function, and locals and arguments
// are absent.
func TestGrowArenaTypesAnOutsideArena(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	m := &ir.Module{Types: []ir.Type{{Inner: f32}}}
	exprs := []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
		{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 0}},
	}

	var typ Typifier
	if err := typ.GrowArena(m, exprs, nil, nil, 2); err != nil {
		t.Fatalf("GrowArena: %v", err)
	}
	if got := typ.Get(0); got.Value != f32 {
		t.Fatalf("literal type = %v, want %v", got.Value, f32)
	}
	if got := typ.Get(1); got.Value != f32 {
		t.Fatalf("binary type = %v, want %v", got.Value, f32)
	}

	// An arena handle out of range is an error, not a panic.
	if err := typ.GrowArena(m, exprs, nil, nil, 3); err == nil {
		t.Fatal("GrowArena accepted a handle past the arena")
	}
}
