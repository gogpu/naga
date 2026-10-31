package sroa

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// makeTypeHandle returns a TypeHandle for the given index.
func makeTypeHandle(idx int) ir.TypeHandle {
	return ir.TypeHandle(idx)
}

// TestRunStructDecomposition verifies that a struct local is decomposed
// into per-member locals when all accesses use constant AccessIndex.
func TestRunStructDecomposition(t *testing.T) {
	// Types:
	// [0] f32
	// [1] vec4<f32>
	// [2] vec2<f32>
	// [3] struct { position: vec4<f32>, texcoord: vec2<f32> }
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: 4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "position", Type: makeTypeHandle(1)},
					{Name: "texcoord", Type: makeTypeHandle(2)},
				},
				Span: 24,
			}},
		},
	}

	fn := &ir.Function{
		Name: "vs",
		LocalVars: []ir.LocalVariable{
			{Name: "vsOutput", Type: makeTypeHandle(3)},
		},
		Expressions: []ir.Expression{
			// [0] ExprLocalVariable{0} — pointer to vsOutput
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			// [1] ExprAccessIndex{Base: 0, Index: 0} — &vsOutput.position
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			// [2] ExprAccessIndex{Base: 0, Index: 1} — &vsOutput.texcoord
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 1}},
			// [3] ExprCompose — vec4(0, 0, 0, 1)
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{}}},
			// [4] ExprFunctionArgument — xy
			{Kind: ir.ExprFunctionArgument{Index: 0}},
			// [5] ExprLoad{Pointer: 0} — load vsOutput (full struct)
			{Kind: ir.ExprLoad{Pointer: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 6),
		Body: ir.Block{
			// Emit range for expr [0..3]
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 3}}},
			// Store to position field
			{Kind: ir.StmtStore{Pointer: 1, Value: 3}},
			// Store to texcoord field
			{Kind: ir.StmtStore{Pointer: 2, Value: 4}},
			// Emit range for load
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 5, End: 6}}},
		},
	}

	count := Run(mod, fn)
	if count != 1 {
		t.Fatalf("expected 1 decomposition, got %d", count)
	}

	// Should have 2 new local vars (position, texcoord).
	if len(fn.LocalVars) != 3 {
		t.Fatalf("expected 3 local vars, got %d", len(fn.LocalVars))
	}
	if fn.LocalVars[1].Name != "position" {
		t.Errorf("expected LocalVars[1].Name = 'position', got %q", fn.LocalVars[1].Name)
	}
	if fn.LocalVars[2].Name != "texcoord" {
		t.Errorf("expected LocalVars[2].Name = 'texcoord', got %q", fn.LocalVars[2].Name)
	}
	if fn.LocalVars[1].Type != makeTypeHandle(1) {
		t.Errorf("expected LocalVars[1].Type = vec4, got %d", fn.LocalVars[1].Type)
	}
	if fn.LocalVars[2].Type != makeTypeHandle(2) {
		t.Errorf("expected LocalVars[2].Type = vec2, got %d", fn.LocalVars[2].Type)
	}

	// AccessIndex expressions should be rewritten to ExprLocalVariable.
	lv1, ok := fn.Expressions[1].Kind.(ir.ExprLocalVariable)
	if !ok {
		t.Errorf("expected Expressions[1] to be ExprLocalVariable, got %T", fn.Expressions[1].Kind)
	} else if lv1.Variable != 1 {
		t.Errorf("expected Expressions[1].Variable = 1, got %d", lv1.Variable)
	}
	lv2, ok := fn.Expressions[2].Kind.(ir.ExprLocalVariable)
	if !ok {
		t.Errorf("expected Expressions[2] to be ExprLocalVariable, got %T", fn.Expressions[2].Kind)
	} else if lv2.Variable != 2 {
		t.Errorf("expected Expressions[2].Variable = 2, got %d", lv2.Variable)
	}

	// Full struct load should be rewritten to ExprCompose.
	compose, ok := fn.Expressions[5].Kind.(ir.ExprCompose)
	if !ok {
		t.Fatalf("expected Expressions[5] to be ExprCompose, got %T", fn.Expressions[5].Kind)
	}
	if len(compose.Components) != 2 {
		t.Fatalf("expected 2 components in Compose, got %d", len(compose.Components))
	}
	// Each component should be an ExprLoad pointing to the new field local.
	for i, comp := range compose.Components {
		if int(comp) >= len(fn.Expressions) {
			t.Errorf("component %d handle %d out of range", i, comp)
			continue
		}
		load, isLoad := fn.Expressions[comp].Kind.(ir.ExprLoad)
		if !isLoad {
			t.Errorf("component %d: expected ExprLoad, got %T", i, fn.Expressions[comp].Kind)
			continue
		}
		if int(load.Pointer) >= len(fn.Expressions) {
			t.Errorf("component %d: load pointer %d out of range", i, load.Pointer)
			continue
		}
		lv, isLV := fn.Expressions[load.Pointer].Kind.(ir.ExprLocalVariable)
		if !isLV {
			t.Errorf("component %d: load pointer should be ExprLocalVariable, got %T", i, fn.Expressions[load.Pointer].Kind)
			continue
		}
		expectedVar := uint32(i) + 1 // new vars start at index 1
		if lv.Variable != expectedVar {
			t.Errorf("component %d: expected Variable=%d, got %d", i, expectedVar, lv.Variable)
		}
	}
}

// TestRunNoDecompositionDynamicAccess verifies that struct locals with
// dynamic access (ExprAccess) are NOT decomposed.
func TestRunNoDecompositionDynamicAccess(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: makeTypeHandle(0)},
					{Name: "b", Type: makeTypeHandle(0)},
				},
				Span: 8,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			// [0] ExprLocalVariable{0}
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			// [1] ExprAccess{Base: 0, Index: ...} — dynamic access
			{Kind: ir.ExprAccess{Base: 0, Index: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 2),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions for dynamic access, got %d", count)
	}
}

// TestRunNoDecompositionNonStruct verifies that non-struct locals are skipped.
func TestRunNoDecompositionNonStruct(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: makeTypeHandle(0)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body:            ir.Block{},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions for scalar local, got %d", count)
	}
}

// TestRunNoDecompositionCallArgument verifies that struct locals passed to
// function calls are NOT decomposed.
func TestRunNoDecompositionCallArgument(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
				},
				Span: 4,
			}},
		},
	}

	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(1)},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 1),
		Body: ir.Block{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{0},
			}},
		},
	}

	count := Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 decompositions for call argument, got %d", count)
	}
}

// TestRunWithInit verifies that struct locals with Compose init get
// proper per-member Init in the decomposed locals.
func TestRunWithInit(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "x", Type: makeTypeHandle(0)},
					{Name: "y", Type: makeTypeHandle(1)},
				},
				Span: 8,
			}},
		},
	}

	initHandle := ir.ExpressionHandle(2)
	fn := &ir.Function{
		Name: "test",
		LocalVars: []ir.LocalVariable{
			{Name: "s", Type: makeTypeHandle(2), Init: &initHandle},
		},
		Expressions: []ir.Expression{
			// [0] ExprLocalVariable{0}
			{Kind: ir.ExprLocalVariable{Variable: 0}},
			// [1] ExprAccessIndex{Base: 0, Index: 0} — &s.x
			{Kind: ir.ExprAccessIndex{Base: 0, Index: 0}},
			// [2] ExprCompose — init value
			{Kind: ir.ExprCompose{Components: []ir.ExpressionHandle{3, 4}}},
			// [3] some float literal
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			// [4] some int literal
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}},
			// [5] ExprLoad{Pointer: 0} — load full struct
			{Kind: ir.ExprLoad{Pointer: 0}},
		},
		ExpressionTypes: make([]ir.TypeResolution, 6),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 0, End: 2}}},
			{Kind: ir.StmtStore{Pointer: 1, Value: 3}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 5, End: 6}}},
		},
	}

	count := Run(mod, fn)
	if count != 1 {
		t.Fatalf("expected 1 decomposition, got %d", count)
	}

	// New locals should have Init pointing to the compose components.
	if fn.LocalVars[1].Init == nil {
		t.Fatal("expected LocalVars[1] (x) to have Init")
	}
	if *fn.LocalVars[1].Init != 3 {
		t.Errorf("expected LocalVars[1].Init = 3, got %d", *fn.LocalVars[1].Init)
	}
	if fn.LocalVars[2].Init == nil {
		t.Fatal("expected LocalVars[2] (y) to have Init")
	}
	if *fn.LocalVars[2].Init != 4 {
		t.Errorf("expected LocalVars[2].Init = 4, got %d", *fn.LocalVars[2].Init)
	}
}

// A store of a whole Compose to the struct becomes a store of each
// component to its member's local, in the store's place. The store was
// left to the struct's own slot, which nothing reads after the pass, so
// `var s = S(x, y); s.a` read the member local's zero value.
func TestRunSplitsComposeStore(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.StructType{
				Members: []ir.StructMember{
					{Name: "a", Type: makeTypeHandle(0)},
					{Name: "b", Type: makeTypeHandle(0), Offset: 4},
				},
				Span: 8,
			}},
		},
	}
	fn := &ir.Function{
		Name:      "main",
		LocalVars: []ir.LocalVariable{{Name: "s", Type: makeTypeHandle(1)}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprFunctionArgument{Index: 0}},                                // [0] x
			{Kind: ir.ExprFunctionArgument{Index: 1}},                                // [1] y
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}}, // [2] S(x, y)
			{Kind: ir.ExprLocalVariable{Variable: 0}},                                // [3] &s
			{Kind: ir.ExprAccessIndex{Base: 3, Index: 0}},                            // [4] &s.a
			{Kind: ir.ExprLoad{Pointer: 4}},                                          // [5] s.a
		},
		ExpressionTypes: make([]ir.TypeResolution, 6),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 3}}},
			{Kind: ir.StmtStore{Pointer: 3, Value: 2}},
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 4, End: 6}}},
		},
	}

	if n := Run(mod, fn); n != 1 {
		t.Fatalf("decomposed %d locals, want 1", n)
	}
	var stores []ir.StmtStore
	for _, st := range fn.Body {
		if s, ok := st.Kind.(ir.StmtStore); ok {
			stores = append(stores, s)
		}
	}
	if len(stores) != 2 {
		t.Fatalf("body has %d stores, want the 2 of the members: %+v", len(stores), fn.Body)
	}
	for i, s := range stores {
		lv, ok := fn.Expressions[s.Pointer].Kind.(ir.ExprLocalVariable)
		if !ok || fn.LocalVars[lv.Variable].Name != mod.Types[1].Inner.(ir.StructType).Members[i].Name {
			t.Errorf("store %d writes %+v, want the local of member %d", i, fn.Expressions[s.Pointer].Kind, i)
		}
		if s.Value != ir.ExpressionHandle(i) {
			t.Errorf("store %d writes value [%d], want component [%d]", i, s.Value, i)
		}
	}
	if _, ok := fn.Body[1].Kind.(ir.StmtStore); !ok {
		t.Errorf("the member stores aren't where the struct store was: %+v", fn.Body)
	}
}

// A struct local initialized from a named constant is left whole: its
// initializer has no components to give the member locals, which started at
// zero, so `var s = cs; s.a` read 0.
func TestRunKeepsConstantInitializedStruct(t *testing.T) {
	mod := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.StructType{Members: []ir.StructMember{{Name: "a", Type: makeTypeHandle(0)}}, Span: 4}},
		},
	}
	initH := ir.ExpressionHandle(0)
	fn := &ir.Function{
		Name:      "main",
		LocalVars: []ir.LocalVariable{{Name: "s", Type: makeTypeHandle(1), Init: &initH}},
		Expressions: []ir.Expression{
			{Kind: ir.ExprConstant{Constant: 0}},          // [0] cs
			{Kind: ir.ExprLocalVariable{Variable: 0}},     // [1] &s
			{Kind: ir.ExprAccessIndex{Base: 1, Index: 0}}, // [2] &s.a
			{Kind: ir.ExprLoad{Pointer: 2}},               // [3] s.a
		},
		ExpressionTypes: make([]ir.TypeResolution, 4),
		Body: ir.Block{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 2, End: 4}}},
		},
	}
	if n := Run(mod, fn); n != 0 {
		t.Fatalf("decomposed %d locals, want the constant-initialized struct left whole", n)
	}
}

// TestRunNilInputs verifies graceful handling of nil parameters.
func TestRunNilInputs(t *testing.T) {
	count := Run(nil, nil)
	if count != 0 {
		t.Errorf("expected 0 for nil inputs, got %d", count)
	}

	mod := &ir.Module{}
	count = Run(mod, nil)
	if count != 0 {
		t.Errorf("expected 0 for nil fn, got %d", count)
	}

	fn := &ir.Function{}
	count = Run(mod, fn)
	if count != 0 {
		t.Errorf("expected 0 for empty fn, got %d", count)
	}
}
