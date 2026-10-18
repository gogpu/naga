package ir

import "testing"

// overrideCompactModule: override 0 "used" (no default) is read by the entry
// point, override 1 "unused" (default 2u) by nothing, override 2 "viaGlobal"
// only through a global expression that initializes a global variable.
//
//	GlobalExpressions: 0 = 2u (init of unused), 1 = Override(2), 2 = 1u,
//	                   3 = Binary(Override(2) + 1u) (init of g)
func overrideCompactModule() *Module {
	u32 := TypeHandle(0)
	two := ExpressionHandle(0)
	gInit := ExpressionHandle(3)
	return &Module{
		Types: []Type{{Inner: ScalarType{Kind: ScalarUint, Width: 4}}},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralU32(2)}},
			{Kind: ExprOverride{Override: 2}},
			{Kind: Literal{Value: LiteralU32(1)}},
			{Kind: ExprBinary{Op: BinaryAdd, Left: 1, Right: 2}},
		},
		Overrides: []Override{
			{Name: "used", Ty: u32},
			{Name: "unused", Ty: u32, Init: &two},
			{Name: "viaGlobal", Ty: u32},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Space: SpacePrivate, Type: u32, InitExpr: &gInit},
		},
		EntryPoints: []EntryPoint{{
			Name:  "main",
			Stage: StageCompute,
			Function: Function{
				Expressions: []Expression{
					{Kind: ExprOverride{Override: 0}},
					{Kind: ExprOverride{Override: 2}},
				},
			},
		}},
	}
}

// TestCompactConstants_KeepUnusedNoRemovesUnusedOverrides: as Rust
// compact(KeepUnused::No), an override nothing uses is removed with its
// initializer, so pipeline-constant processing does not ask for its value;
// the handles of the overrides that stay are renumbered everywhere.
func TestCompactConstants_KeepUnusedNoRemovesUnusedOverrides(t *testing.T) {
	m := overrideCompactModule()
	CompactConstants(m, KeepUnusedNo)

	if got := len(m.Overrides); got != 2 {
		t.Fatalf("overrides: got %d, want 2 (used, viaGlobal)", got)
	}
	if m.Overrides[0].Name != "used" || m.Overrides[1].Name != "viaGlobal" {
		t.Fatalf("overrides: got %q, %q; want used, viaGlobal", m.Overrides[0].Name, m.Overrides[1].Name)
	}
	if got := len(m.GlobalExpressions); got != 3 {
		t.Fatalf("global expressions: got %d, want 3 (the 2u default of unused is dead)", got)
	}
	// viaGlobal moved from override 2 to override 1.
	fn := m.EntryPoints[0].Function.Expressions
	if eo, ok := fn[1].Kind.(ExprOverride); !ok || eo.Override != 1 {
		t.Errorf("function expression 1: got %#v, want ExprOverride{1}", fn[1].Kind)
	}
	if eo, ok := fn[0].Kind.(ExprOverride); !ok || eo.Override != 0 {
		t.Errorf("function expression 0: got %#v, want ExprOverride{0}", fn[0].Kind)
	}
	if eo, ok := m.GlobalExpressions[0].Kind.(ExprOverride); !ok || eo.Override != 1 {
		t.Errorf("global expression 0: got %#v, want ExprOverride{1}", m.GlobalExpressions[0].Kind)
	}
	b, ok := m.GlobalExpressions[2].Kind.(ExprBinary)
	if !ok || b.Left != 0 || b.Right != 1 {
		t.Errorf("global expression 2: got %#v, want Binary(0, 1)", m.GlobalExpressions[2].Kind)
	}
	if init := m.GlobalVariables[0].InitExpr; init == nil || *init != 2 {
		t.Errorf("g init: got %v, want 2", init)
	}
}

// TestCompactConstants_KeepUnusedYesKeepsNamedOverrides: as Rust
// compact(KeepUnused::Yes), every named override and its initializer stay.
func TestCompactConstants_KeepUnusedYesKeepsNamedOverrides(t *testing.T) {
	m := overrideCompactModule()
	CompactConstants(m, KeepUnusedYes)

	if got := len(m.Overrides); got != 3 {
		t.Fatalf("overrides: got %d, want 3", got)
	}
	if got := len(m.GlobalExpressions); got != 4 {
		t.Fatalf("global expressions: got %d, want 4", got)
	}
	if init := m.Overrides[1].Init; init == nil || *init != 0 {
		t.Errorf("unused init: got %v, want 0", init)
	}
}

// TestCompactConstants_AnonymousOverrideRemovedWhenUnused: an override with no
// name (Rust creates them for override-sized arrays) is never used by
// definition, also with KeepUnusedYes.
func TestCompactConstants_AnonymousOverrideRemovedWhenUnused(t *testing.T) {
	m := &Module{
		Types:             []Type{{Inner: ScalarType{Kind: ScalarUint, Width: 4}}},
		GlobalExpressions: []Expression{{Kind: Literal{Value: LiteralU32(4)}}},
	}
	init := ExpressionHandle(0)
	m.Overrides = []Override{{Ty: 0, Init: &init}}
	CompactConstants(m, KeepUnusedYes)
	if len(m.Overrides) != 0 || len(m.GlobalExpressions) != 0 {
		t.Errorf("got %d overrides, %d global expressions; want 0, 0", len(m.Overrides), len(m.GlobalExpressions))
	}
}

// TestCompactConstants_MeshOverrideKeepsOverride: a mesh output limit given by
// an override expression keeps the override.
func TestCompactConstants_MeshOverrideKeepsOverride(t *testing.T) {
	maxV := ExpressionHandle(0)
	m := &Module{
		Types:             []Type{{Inner: ScalarType{Kind: ScalarUint, Width: 4}}},
		GlobalExpressions: []Expression{{Kind: ExprOverride{Override: 0}}},
		Overrides:         []Override{{Name: "maxV", Ty: 0}},
		EntryPoints: []EntryPoint{{
			Name: "ms", Stage: StageMesh,
			MeshInfo: &MeshStageInfo{MaxVerticesOverride: &maxV},
		}},
	}
	CompactConstants(m, KeepUnusedNo)
	if len(m.Overrides) != 1 || len(m.GlobalExpressions) != 1 {
		t.Errorf("got %d overrides, %d global expressions; want 1, 1", len(m.Overrides), len(m.GlobalExpressions))
	}
}

// TestCompactConstants_WorkgroupSizeOverrideKept: a workgroup size given by an
// override-expression keeps its expression and the override (Rust compact
// traces workgroup_size_overrides), and its handle is renumbered.
func TestCompactConstants_WorkgroupSizeOverrideKept(t *testing.T) {
	size := ExpressionHandle(1)
	m := &Module{
		Types: []Type{{Inner: ScalarType{Kind: ScalarUint, Width: 4}}},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralU32(9)}}, // dead
			{Kind: ExprOverride{Override: 0}},
		},
		Overrides: []Override{{Name: "wg", Ty: 0}},
		EntryPoints: []EntryPoint{{
			Name: "main", Stage: StageCompute, Workgroup: [3]uint32{1, 1, 1},
			WorkgroupSizeOverrides: [3]*ExpressionHandle{&size, nil, nil},
		}},
	}
	CompactConstants(m, KeepUnusedNo)
	if len(m.Overrides) != 1 || len(m.GlobalExpressions) != 1 {
		t.Fatalf("got %d overrides, %d global expressions; want 1, 1", len(m.Overrides), len(m.GlobalExpressions))
	}
	if h := m.EntryPoints[0].WorkgroupSizeOverrides[0]; h == nil || *h != 0 {
		t.Errorf("workgroup size x: got %v, want 0", h)
	}
}

// TestCompactConstants_PendingArraySizeKeepsOverride: an array type sized by
// an override keeps it, also one without a name, and the type's handle to it
// is renumbered.
func TestCompactConstants_PendingArraySizeKeepsOverride(t *testing.T) {
	o := OverrideHandle(1)
	four := ExpressionHandle(0)
	m := &Module{
		Types: []Type{
			{Inner: ScalarType{Kind: ScalarUint, Width: 4}},
			{Inner: ArrayType{Base: 0, Size: ArraySize{Pending: &o}, Stride: 4}},
		},
		GlobalExpressions: []Expression{{Kind: Literal{Value: LiteralU32(4)}}},
		Overrides:         []Override{{Name: "unused", Ty: 0}, {Ty: 0, Init: &four}},
	}
	CompactConstants(m, KeepUnusedNo)
	if len(m.Overrides) != 1 || m.Overrides[0].Init == nil {
		t.Fatalf("overrides = %+v, want only the array size's", m.Overrides)
	}
	at := m.Types[1].Inner.(ArrayType)
	if at.Size.Pending == nil || *at.Size.Pending != 0 {
		t.Errorf("array size = %+v, want pending on override 0", at.Size)
	}
	if l, err := at.Size.Resolve(m); err != nil || l.Known != 4 {
		t.Errorf("Resolve = %+v, %v; want 4", l, err)
	}
}
