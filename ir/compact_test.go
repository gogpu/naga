package ir

import (
	"testing"
)

// --- IsAbstractType tests ---

func TestIsAbstractType(t *testing.T) {
	types := []Type{
		{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		{Name: "", Inner: ScalarType{Kind: ScalarAbstractInt, Width: 0}},
		{Name: "", Inner: ScalarType{Kind: ScalarAbstractFloat, Width: 0}},
		{Name: "", Inner: VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarAbstractFloat, Width: 0}}},
		{Name: "", Inner: MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarAbstractFloat, Width: 0}}},
		{Name: "", Inner: ArrayType{Base: 1, Stride: 4}},  // base is abstract int
		{Name: "", Inner: ArrayType{Base: 0, Stride: 16}}, // base is concrete f32
		{Name: "", Inner: StructType{Members: nil, Span: 0}},
	}

	tests := []struct {
		name  string
		inner TypeInner
		want  bool
	}{
		{"concrete scalar f32", ScalarType{Kind: ScalarFloat, Width: 4}, false},
		{"concrete scalar i32", ScalarType{Kind: ScalarSint, Width: 4}, false},
		{"concrete scalar u32", ScalarType{Kind: ScalarUint, Width: 4}, false},
		{"concrete scalar bool", ScalarType{Kind: ScalarBool, Width: 1}, false},
		{"abstract int", ScalarType{Kind: ScalarAbstractInt, Width: 0}, true},
		{"abstract float", ScalarType{Kind: ScalarAbstractFloat, Width: 0}, true},
		{"vector with abstract float", VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarAbstractFloat}}, true},
		{"vector with concrete float", VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, false},
		{"matrix with abstract int", MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarAbstractInt}}, true},
		{"matrix with concrete float", MatrixType{Columns: Vec4, Rows: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, false},
		{"array with abstract base", ArrayType{Base: 1, Stride: 4}, true},
		{"array with concrete base", ArrayType{Base: 0, Stride: 16}, false},
		{"struct is never abstract", StructType{Members: nil, Span: 0}, false},
		{"pointer is never abstract", PointerType{Base: 0, Space: SpaceFunction}, false},
		{"sampler is never abstract", SamplerType{Comparison: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAbstractType(tt.inner, types)
			if got != tt.want {
				t.Errorf("IsAbstractType(%T) = %v, want %v", tt.inner, got, tt.want)
			}
		})
	}
}

// --- concretizeTypeInner tests ---

func TestConcretizeTypeInner(t *testing.T) {
	tests := []struct {
		name  string
		input TypeInner
		want  TypeInner
	}{
		{
			"abstract int -> sint/4",
			ScalarType{Kind: ScalarAbstractInt, Width: 0},
			ScalarType{Kind: ScalarSint, Width: 4},
		},
		{
			"abstract float -> float/4",
			ScalarType{Kind: ScalarAbstractFloat, Width: 0},
			ScalarType{Kind: ScalarFloat, Width: 4},
		},
		{
			"concrete f32 unchanged",
			ScalarType{Kind: ScalarFloat, Width: 4},
			ScalarType{Kind: ScalarFloat, Width: 4},
		},
		{
			"vector with abstract float -> concrete",
			VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarAbstractFloat}},
			VectorType{Size: Vec3, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		{
			"vector concrete unchanged",
			VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
			VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		{
			"matrix with abstract int -> concrete",
			MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarAbstractInt}},
			MatrixType{Columns: Vec2, Rows: Vec2, Scalar: ScalarType{Kind: ScalarSint, Width: 4}},
		},
		{
			"bool unchanged",
			ScalarType{Kind: ScalarBool, Width: 1},
			ScalarType{Kind: ScalarBool, Width: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := concretizeTypeInner(tt.input)
			if !typeInnerEqual(got, tt.want) {
				t.Errorf("concretizeTypeInner() = %v, want %v", got, tt.want)
			}
		})
	}

	// Struct passthrough (typeInnerEqual doesn't handle structs)
	t.Run("struct passes through", func(t *testing.T) {
		input := StructType{Members: nil, Span: 0}
		got := concretizeTypeInner(input)
		if _, ok := got.(StructType); !ok {
			t.Errorf("expected StructType passthrough, got %T", got)
		}
	})
}

// --- isPreEmitExpression tests ---

func TestIsPreEmitExpression(t *testing.T) {
	tests := []struct {
		name string
		kind ExpressionKind
		want bool
	}{
		{"Literal", Literal{Value: LiteralF32(1.0)}, true},
		{"ExprConstant", ExprConstant{Constant: 0}, true},
		{"ExprOverride", ExprOverride{Override: 0}, true},
		{"ExprZeroValue", ExprZeroValue{Type: 0}, true},
		{"ExprGlobalVariable", ExprGlobalVariable{Variable: 0}, true},
		{"ExprFunctionArgument", ExprFunctionArgument{Index: 0}, true},
		{"ExprLocalVariable", ExprLocalVariable{Variable: 0}, true},
		{"ExprBinary not pre-emit", ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}, false},
		{"ExprUnary not pre-emit", ExprUnary{Op: UnaryNegate, Expr: 0}, false},
		{"ExprLoad not pre-emit", ExprLoad{Pointer: 0}, false},
		{"ExprAccess not pre-emit", ExprAccess{Base: 0, Index: 1}, false},
		{"ExprCompose not pre-emit", ExprCompose{Type: 0, Components: nil}, false},
		{"ExprMath not pre-emit", ExprMath{Fun: MathSin, Arg: 0}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPreEmitExpression(tt.kind)
			if got != tt.want {
				t.Errorf("isPreEmitExpression(%T) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

// --- CompactUnused tests ---

func TestCompactUnused_NoEntryPoints(t *testing.T) {
	module := &Module{
		GlobalVariables: []GlobalVariable{
			{Name: "unused", Type: 0},
		},
		Functions: []Function{
			{Name: "unused_func"},
		},
	}
	CompactUnused(module)
	// With no entry points, nothing should change
	if len(module.GlobalVariables) != 1 {
		t.Errorf("expected 1 global, got %d", len(module.GlobalVariables))
	}
	if len(module.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(module.Functions))
	}
}

func TestCompactUnused_RemovesUnusedGlobals(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "used_var", Type: 0, Space: SpaceUniform},
			{Name: "unused_var", Type: 0, Space: SpaceUniform},
			{Name: "also_used", Type: 0, Space: SpaceUniform},
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: ExprGlobalVariable{Variable: 0}}, // references global 0
						{Kind: ExprGlobalVariable{Variable: 2}}, // references global 2
					},
					Body: []Statement{},
				},
			},
		},
	}

	CompactUnused(module)

	if len(module.GlobalVariables) != 2 {
		t.Fatalf("expected 2 globals after compact, got %d", len(module.GlobalVariables))
	}
	if module.GlobalVariables[0].Name != "used_var" {
		t.Errorf("expected first global 'used_var', got '%s'", module.GlobalVariables[0].Name)
	}
	if module.GlobalVariables[1].Name != "also_used" {
		t.Errorf("expected second global 'also_used', got '%s'", module.GlobalVariables[1].Name)
	}

	// Check that expression handles were remapped
	ep := module.EntryPoints[0].Function
	gv0 := ep.Expressions[0].Kind.(ExprGlobalVariable)
	gv1 := ep.Expressions[1].Kind.(ExprGlobalVariable)
	if gv0.Variable != 0 {
		t.Errorf("expected remapped global 0, got %d", gv0.Variable)
	}
	if gv1.Variable != 1 {
		t.Errorf("expected remapped global 1 (was 2), got %d", gv1.Variable)
	}
}

func TestCompactUnused_RemovesUnusedFunctions(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{Name: "used_func"},
			{Name: "unused_func"},
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageCompute,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: ExprCallResult{Function: 0}},
					},
					Body: []Statement{
						{Kind: StmtCall{Function: 0, Arguments: nil, Result: nil}},
					},
				},
			},
		},
	}

	CompactUnused(module)

	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function after compact, got %d", len(module.Functions))
	}
	if module.Functions[0].Name != "used_func" {
		t.Errorf("expected 'used_func', got '%s'", module.Functions[0].Name)
	}
}

func TestCompactUnused_AllUsed(t *testing.T) {
	module := &Module{
		GlobalVariables: []GlobalVariable{
			{Name: "g0", Type: 0},
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Expressions: []Expression{
						{Kind: ExprGlobalVariable{Variable: 0}},
					},
					Body: []Statement{},
				},
			},
		},
	}

	CompactUnused(module)

	if len(module.GlobalVariables) != 1 {
		t.Errorf("expected 1 global (all used), got %d", len(module.GlobalVariables))
	}
}

func TestCompactUnused_MeshShaderTaskPayload(t *testing.T) {
	gvh := GlobalVariableHandle(1)
	module := &Module{
		GlobalVariables: []GlobalVariable{
			{Name: "unused", Type: 0},
			{Name: "payload", Type: 0},
		},
		EntryPoints: []EntryPoint{
			{
				Name:        "main",
				Stage:       StageMesh,
				TaskPayload: &gvh,
				Function: Function{
					Expressions: []Expression{},
					Body:        []Statement{},
				},
			},
		},
	}

	CompactUnused(module)

	// payload (index 1) should be kept, unused (index 0) removed
	if len(module.GlobalVariables) != 1 {
		t.Fatalf("expected 1 global, got %d", len(module.GlobalVariables))
	}
	if module.GlobalVariables[0].Name != "payload" {
		t.Errorf("expected 'payload', got '%s'", module.GlobalVariables[0].Name)
	}
	// TaskPayload should be remapped to 0
	if *module.EntryPoints[0].TaskPayload != 0 {
		t.Errorf("expected TaskPayload remapped to 0, got %d", *module.EntryPoints[0].TaskPayload)
	}
}

// --- CompactTypes tests ---

func TestCompactTypes_EmptyModule(t *testing.T) {
	module := &Module{}
	CompactTypes(module) // should not panic
	if len(module.Types) != 0 {
		t.Errorf("expected 0 types, got %d", len(module.Types))
	}
}

func TestCompactTypes_RemovesUnreferencedAnonymousTypes(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                                      // 0: unreferenced anonymous scalar
			{Name: "vec4f", Inner: VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}}, // 1: named
			{Name: "", Inner: ScalarType{Kind: ScalarUint, Width: 4}},                                       // 2: referenced by constant
		},
		Constants: []Constant{
			{Name: "C", Type: 2},
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Arguments: []FunctionArgument{{Name: "a", Type: 1}},
				},
			},
		},
	}

	CompactTypes(module)

	// Type 0 (unreferenced anonymous) should be removed
	// Type 1 (named) kept, Type 2 (referenced by constant) kept
	if len(module.Types) != 2 {
		t.Fatalf("expected 2 types after compact, got %d", len(module.Types))
	}
	if module.Types[0].Name != "vec4f" {
		t.Errorf("expected first type 'vec4f', got '%s'", module.Types[0].Name)
	}
}

func TestCompactTypes_RemovesAbstractTypes(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},     // 0: concrete
			{Name: "abs_int", Inner: ScalarType{Kind: ScalarAbstractInt}},     // 1: abstract - must be removed even if named
			{Name: "abs_float", Inner: ScalarType{Kind: ScalarAbstractFloat}}, // 2: abstract
		},
		Constants: []Constant{
			{Name: "C", Type: 0},
		},
	}

	CompactTypes(module)

	// Both abstract types removed, even though named
	if len(module.Types) != 1 {
		t.Fatalf("expected 1 type after compact (abstract removed), got %d", len(module.Types))
	}
	if module.Types[0].Name != "f32" {
		t.Errorf("expected remaining type 'f32', got '%s'", module.Types[0].Name)
	}
}

func TestCompactTypes_RemapsHandlesInTypes(t *testing.T) {
	size := uint32(4)
	module := &Module{
		Types: []Type{
			{Name: "", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                             // 0: unreferenced anonymous
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                          // 1: named, referenced by const C
			{Name: "arr", Inner: ArrayType{Base: 1, Size: ArraySize{Constant: &size}, Stride: 16}}, // 2: referenced by global
		},
		Constants: []Constant{
			{Name: "C", Type: 1},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Type: 2}, // reference type 2 to keep it
		},
	}

	CompactTypes(module)

	// Type 0 removed, type 1->0, type 2->1
	if len(module.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(module.Types))
	}
	// Array base should be remapped from 1 to 0
	arr, ok := module.Types[1].Inner.(ArrayType)
	if !ok {
		t.Fatal("expected ArrayType at index 1")
	}
	if arr.Base != 0 {
		t.Errorf("expected array base remapped to 0, got %d", arr.Base)
	}
	// Constant type should be remapped
	if module.Constants[0].Type != 0 {
		t.Errorf("expected constant type remapped to 0, got %d", module.Constants[0].Type)
	}
}

func TestCompactTypes_UnnamedUnreferenced(t *testing.T) {
	// UNNAMED unreferenced types are removed. Named types are always kept.
	module := &Module{
		Types: []Type{
			{Name: "", Inner: ScalarType{Kind: ScalarFloat, Width: 4}}, // unnamed, referenced
			{Name: "", Inner: ScalarType{Kind: ScalarUint, Width: 4}},  // unnamed, unreferenced
		},
		Constants: []Constant{
			{Name: "x", Type: 0}, // references type[0]
		},
	}

	CompactTypes(module)

	// f32 kept (referenced), u32 removed (unnamed + unreferenced)
	if len(module.Types) != 1 {
		t.Errorf("expected 1 type (f32 kept, u32 removed), got %d", len(module.Types))
	}
}

func TestCompactTypes_NamedTypesKept(t *testing.T) {
	// ALL named types are kept even without direct references,
	// matching Rust naga's compact pass which preserves named types.
	module := &Module{
		Types: []Type{
			{Name: "MyStruct", Inner: StructType{
				Members: []StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
			{Name: "Vec2Alias", Inner: VectorType{Size: 2, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}},
		},
	}

	CompactTypes(module)

	// Both kept: MyStruct (named struct) and Vec2Alias (named alias).
	// Rust naga keeps ALL named types regardless of reference count.
	if len(module.Types) != 2 {
		t.Errorf("expected 2 types (both named, both kept), got %d", len(module.Types))
	}
}

// --- CompactConstants tests ---

func TestCompactConstants_EmptyModule(t *testing.T) {
	module := &Module{}
	CompactConstants(module) // should not panic
}

func TestCompactConstants_RemovesAbstractConstants(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Constants: []Constant{
			{Name: "NAMED_CONCRETE", Type: 0, IsAbstract: false},
			{Name: "NAMED_ABSTRACT", Type: 0, IsAbstract: true},
			{Name: "", Type: 0, IsAbstract: false}, // unnamed
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Expressions: []Expression{
						{Kind: ExprConstant{Constant: 0}},
					},
				},
			},
		},
	}

	CompactConstants(module)

	// Only NAMED_CONCRETE should survive (named + not abstract + referenced)
	if len(module.Constants) != 1 {
		t.Fatalf("expected 1 constant, got %d", len(module.Constants))
	}
	if module.Constants[0].Name != "NAMED_CONCRETE" {
		t.Errorf("expected 'NAMED_CONCRETE', got '%s'", module.Constants[0].Name)
	}
}

func TestCompactConstants_KeepsReferencedByGlobalVarInit(t *testing.T) {
	ch := ConstantHandle(1)
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
		Constants: []Constant{
			{Name: "", Type: 0}, // 0: unnamed, not directly referenced
			{Name: "", Type: 0}, // 1: unnamed, but referenced by global var init
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g", Type: 0, Init: &ch},
		},
	}

	CompactConstants(module)

	if len(module.Constants) != 1 {
		t.Fatalf("expected 1 constant (ref'd by global init), got %d", len(module.Constants))
	}
}

func TestCompactConstants_TransitiveKeepComposite(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
		Constants: []Constant{
			{Name: "", Type: 0, Value: ScalarValue{Bits: 1, Kind: ScalarUint}},              // 0: component
			{Name: "COMP", Type: 0, Value: CompositeValue{Components: []ConstantHandle{0}}}, // 1: composite referencing 0
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Expressions: []Expression{
						{Kind: ExprConstant{Constant: 1}},
					},
				},
			},
		},
	}

	CompactConstants(module)

	// Both should be kept: 1 is named+referenced, 0 is transitively needed by composite
	if len(module.Constants) != 2 {
		t.Fatalf("expected 2 constants (transitive keep), got %d", len(module.Constants))
	}
}

func TestCompactConstants_AllKept(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Constants: []Constant{
			{Name: "A", Type: 0},
			{Name: "B", Type: 0},
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Expressions: []Expression{
						{Kind: ExprConstant{Constant: 0}},
						{Kind: ExprConstant{Constant: 1}},
					},
				},
			},
		},
	}

	CompactConstants(module)

	if len(module.Constants) != 2 {
		t.Errorf("expected 2 constants (all kept), got %d", len(module.Constants))
	}
}

// --- CompactExpressions tests ---

func TestCompactExpressions_RemovesUnusedExpressions(t *testing.T) {
	retVal := ExpressionHandle(2)
	module := &Module{
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}}, // 0: unused
						{Kind: Literal{Value: LiteralF32(2.0)}}, // 1: unused
						{Kind: Literal{Value: LiteralF32(3.0)}}, // 2: used by return
					},
					Body: []Statement{
						{Kind: StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Fatalf("expected 1 expression after compact, got %d", len(fn.Expressions))
	}
	// The return value should be remapped to 0
	ret := fn.Body[0].Kind.(StmtReturn)
	if ret.Value == nil || *ret.Value != 0 {
		t.Errorf("expected return value remapped to 0")
	}
}

func TestCompactExpressions_KeepsNamedExpressions(t *testing.T) {
	module := &Module{
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}}, // 0: named
						{Kind: Literal{Value: LiteralF32(2.0)}}, // 1: not named, not used
					},
					NamedExpressions: map[ExpressionHandle]string{
						0: "x",
					},
					Body: []Statement{},
				},
			},
		},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Fatalf("expected 1 expression (named kept), got %d", len(fn.Expressions))
	}
	if _, ok := fn.NamedExpressions[0]; !ok {
		t.Error("expected named expression at handle 0")
	}
}

func TestCompactExpressions_TransitiveDependencies(t *testing.T) {
	retVal := ExpressionHandle(2)
	module := &Module{
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},              // 0: base for binary
						{Kind: Literal{Value: LiteralF32(2.0)}},              // 1: base for binary
						{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}}, // 2: used by return
					},
					Body: []Statement{
						{Kind: StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// All 3 should be kept (0,1 transitively needed by 2)
	if len(fn.Expressions) != 3 {
		t.Fatalf("expected 3 expressions (transitive), got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_AllUsed(t *testing.T) {
	retVal := ExpressionHandle(0)
	module := &Module{
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},
					},
					Body: []Statement{
						{Kind: StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Errorf("expected 1 expression (all used), got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_EmptyFunction(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{Name: "empty"},
		},
	}

	CompactExpressions(module) // should not panic
}

func TestCompactExpressions_RemapsEmitRanges(t *testing.T) {
	retVal := ExpressionHandle(2)
	module := &Module{
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(1.0)}},              // 0: unused
						{Kind: Literal{Value: LiteralF32(2.0)}},              // 1: used
						{Kind: ExprBinary{Op: BinaryAdd, Left: 1, Right: 1}}, // 2: used by return
					},
					Body: []Statement{
						{Kind: StmtEmit{Range: Range{Start: 1, End: 3}}},
						{Kind: StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// Expression 0 removed, 1->0, 2->1
	if len(fn.Expressions) != 2 {
		t.Fatalf("expected 2 expressions, got %d", len(fn.Expressions))
	}
	// Check emit range was adjusted
	emit := fn.Body[0].Kind.(StmtEmit)
	if emit.Range.Start != 0 || emit.Range.End != 2 {
		t.Errorf("expected emit range [0,2), got [%d,%d)", emit.Range.Start, emit.Range.End)
	}
}

func TestCompactExpressions_LocalVarInitKept(t *testing.T) {
	init := ExpressionHandle(0)
	module := &Module{
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageVertex,
				Function: Function{
					Name: "main",
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(0.0)}}, // 0: used by local var init
						{Kind: Literal{Value: LiteralF32(1.0)}}, // 1: unused
					},
					LocalVars: []LocalVariable{
						{Name: "x", Type: 0, Init: &init},
					},
					Body: []Statement{},
				},
			},
		},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Fatalf("expected 1 expression (local var init), got %d", len(fn.Expressions))
	}
}

// --- ReorderTypes tests ---

func TestReorderTypes_EmptyModule(t *testing.T) {
	module := &Module{}
	ReorderTypes(module) // should not panic
}

func TestReorderTypes_AlreadyOrdered(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
		},
		TypeUseOrder: []TypeHandle{0, 1},
	}

	ReorderTypes(module)

	if module.Types[0].Name != "f32" || module.Types[1].Name != "u32" {
		t.Error("types should remain in same order")
	}
}

func TestReorderTypes_ReordersBasedOnTypeUseOrder(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},  // 0
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}}, // 1
		},
		TypeUseOrder: []TypeHandle{1, 0}, // f32 first, u32 second
		Constants: []Constant{
			{Name: "C", Type: 0}, // references u32
		},
	}

	ReorderTypes(module)

	// After reorder: f32 at 0, u32 at 1
	if module.Types[0].Name != "f32" {
		t.Errorf("expected f32 at 0, got %s", module.Types[0].Name)
	}
	if module.Types[1].Name != "u32" {
		t.Errorf("expected u32 at 1, got %s", module.Types[1].Name)
	}
	// Constant should be remapped
	if module.Constants[0].Type != 1 {
		t.Errorf("expected constant type remapped to 1, got %d", module.Constants[0].Type)
	}
}

func TestReorderTypes_RemapsSpecialTypes(t *testing.T) {
	rayH := TypeHandle(1)
	module := &Module{
		Types: []Type{
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},
			{Name: "RayIntersection", Inner: StructType{Members: nil, Span: 0}},
		},
		TypeUseOrder: []TypeHandle{1, 0},
		SpecialTypes: SpecialTypes{
			RayIntersection: &rayH,
		},
	}

	ReorderTypes(module)

	// RayIntersection was at 1, now at 0
	if module.SpecialTypes.RayIntersection == nil || *module.SpecialTypes.RayIntersection != 0 {
		t.Error("expected RayIntersection remapped to 0")
	}
}

// --- DeduplicateEmits tests ---

func TestDeduplicateEmits_RemovesEmptyEmits(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "f",
				Expressions: []Expression{
					{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 0}},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 0, End: 0}}}, // empty
					{Kind: StmtEmit{Range: Range{Start: 0, End: 1}}}, // non-empty with non-pre-emit expr
					{Kind: StmtReturn{}},
				},
			},
		},
	}

	DeduplicateEmits(module)

	body := module.Functions[0].Body
	if len(body) != 2 {
		t.Fatalf("expected 2 statements after dedup (empty emit removed), got %d", len(body))
	}
}

func TestDeduplicateEmits_RemovesCoveredEmits(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "f",
				Expressions: []Expression{
					{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 0}},
					{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 0}},
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 0, End: 2}}}, // first emit
					{Kind: StmtEmit{Range: Range{Start: 0, End: 2}}}, // duplicate — covered
					{Kind: StmtReturn{}},
				},
			},
		},
	}

	DeduplicateEmits(module)

	body := module.Functions[0].Body
	if len(body) != 2 {
		t.Fatalf("expected 2 statements (duplicate removed), got %d", len(body))
	}
}

func TestDeduplicateEmits_SkipsPreEmitOnlyRanges(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{
				Name: "f",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}},              // pre-emit
					{Kind: ExprConstant{Constant: 0}},                    // pre-emit
					{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 0}}, // not pre-emit
				},
				Body: []Statement{
					{Kind: StmtEmit{Range: Range{Start: 0, End: 2}}}, // all pre-emit -> removed
					{Kind: StmtEmit{Range: Range{Start: 2, End: 3}}}, // has non-pre-emit -> kept
					{Kind: StmtReturn{}},
				},
			},
		},
	}

	DeduplicateEmits(module)

	body := module.Functions[0].Body
	if len(body) != 2 {
		t.Fatalf("expected 2 statements (pre-emit only removed), got %d", len(body))
	}
}

// --- markTypeInnerRefs tests ---

func TestMarkTypeInnerRefs(t *testing.T) {
	referenced := make([]bool, 5)

	tests := []struct {
		name  string
		inner TypeInner
		want  []int // indices that should be marked
	}{
		{"array marks base", ArrayType{Base: 2, Stride: 4}, []int{2}},
		{"struct marks members", StructType{Members: []StructMember{
			{Type: 0}, {Type: 3},
		}}, []int{0, 3}},
		{"pointer marks base", PointerType{Base: 1, Space: SpaceFunction}, []int{1}},
		{"binding array marks base", BindingArrayType{Base: 4}, []int{4}},
		{"scalar marks nothing", ScalarType{Kind: ScalarFloat, Width: 4}, nil},
		{"vector marks nothing", VectorType{Size: Vec4, Scalar: ScalarType{Kind: ScalarFloat, Width: 4}}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range referenced {
				referenced[i] = false
			}
			markTypeInnerRefs(tt.inner, referenced)
			for _, idx := range tt.want {
				if !referenced[idx] {
					t.Errorf("expected index %d to be marked", idx)
				}
			}
		})
	}
}

// --- markExprTypeRefs tests ---

func TestMarkExprTypeRefs(t *testing.T) {
	referenced := make([]bool, 5)

	tests := []struct {
		name string
		kind ExpressionKind
		want []int
	}{
		{"ZeroValue marks type", ExprZeroValue{Type: 2}, []int{2}},
		{"Compose marks type", ExprCompose{Type: 3}, []int{3}},
		{"SubgroupOperationResult marks type", ExprSubgroupOperationResult{Type: 1}, []int{1}},
		{"AtomicResult marks type", ExprAtomicResult{Ty: 4}, []int{4}},
		{"Literal marks nothing", Literal{Value: LiteralF32(1.0)}, nil},
		{"Binary marks nothing", ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range referenced {
				referenced[i] = false
			}
			markExprTypeRefs(tt.kind, referenced)
			for _, idx := range tt.want {
				if !referenced[idx] {
					t.Errorf("expected index %d to be marked", idx)
				}
			}
		})
	}
}

// --- remapTypeInner tests ---

func TestRemapTypeInner(t *testing.T) {
	remap := []TypeHandle{2, 0, 1, 3, 4}

	tests := []struct {
		name     string
		input    TypeInner
		wantBase TypeHandle // for types with base handles
	}{
		{"array remaps base", ArrayType{Base: 0, Stride: 4}, 2},
		{"pointer remaps base", PointerType{Base: 1, Space: SpaceFunction}, 0},
		{"binding array remaps base", BindingArrayType{Base: 2}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapTypeInner(tt.input, remap)
			switch r := result.(type) {
			case ArrayType:
				if r.Base != tt.wantBase {
					t.Errorf("base = %d, want %d", r.Base, tt.wantBase)
				}
			case PointerType:
				if r.Base != tt.wantBase {
					t.Errorf("base = %d, want %d", r.Base, tt.wantBase)
				}
			case BindingArrayType:
				if r.Base != tt.wantBase {
					t.Errorf("base = %d, want %d", r.Base, tt.wantBase)
				}
			}
		})
	}

	// Struct member remapping
	t.Run("struct remaps member types", func(t *testing.T) {
		input := StructType{
			Members: []StructMember{
				{Name: "a", Type: 0},
				{Name: "b", Type: 1},
			},
			Span: 8,
		}
		result := remapTypeInner(input, remap).(StructType)
		if result.Members[0].Type != 2 {
			t.Errorf("member 0 type = %d, want 2", result.Members[0].Type)
		}
		if result.Members[1].Type != 0 {
			t.Errorf("member 1 type = %d, want 0", result.Members[1].Type)
		}
	})

	// Scalar passes through unchanged
	t.Run("scalar unchanged", func(t *testing.T) {
		input := ScalarType{Kind: ScalarFloat, Width: 4}
		result := remapTypeInner(input, remap)
		if s, ok := result.(ScalarType); !ok || s != input {
			t.Errorf("scalar should be unchanged")
		}
	})
}

// --- remapExprTypeHandles tests ---

func TestRemapExprTypeHandles(t *testing.T) {
	remap := []TypeHandle{1, 0, 2}

	tests := []struct {
		name string
		kind ExpressionKind
		want TypeHandle
	}{
		{"ZeroValue", ExprZeroValue{Type: 0}, 1},
		{"Compose", ExprCompose{Type: 1}, 0},
		{"SubgroupOperationResult", ExprSubgroupOperationResult{Type: 2}, 2},
		{"AtomicResult", ExprAtomicResult{Ty: 0}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapExprTypeHandles(tt.kind, remap)
			switch r := result.(type) {
			case ExprZeroValue:
				if r.Type != tt.want {
					t.Errorf("type = %d, want %d", r.Type, tt.want)
				}
			case ExprCompose:
				if r.Type != tt.want {
					t.Errorf("type = %d, want %d", r.Type, tt.want)
				}
			case ExprSubgroupOperationResult:
				if r.Type != tt.want {
					t.Errorf("type = %d, want %d", r.Type, tt.want)
				}
			case ExprAtomicResult:
				if r.Ty != tt.want {
					t.Errorf("type = %d, want %d", r.Ty, tt.want)
				}
			}
		})
	}

	// Non-type expressions pass through
	t.Run("literal unchanged", func(t *testing.T) {
		result := remapExprTypeHandles(Literal{Value: LiteralF32(1.0)}, remap)
		if _, ok := result.(Literal); !ok {
			t.Error("literal should pass through unchanged")
		}
	})
}

// --- remapExprHandles tests ---

func TestRemapExprHandles(t *testing.T) {
	remap := []ExpressionHandle{2, 0, 1, 3}

	t.Run("Binary", func(t *testing.T) {
		result := remapExprHandles(ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}, remap)
		bin := result.(ExprBinary)
		if bin.Left != 2 || bin.Right != 0 {
			t.Errorf("got left=%d right=%d, want left=2 right=0", bin.Left, bin.Right)
		}
	})

	t.Run("Unary", func(t *testing.T) {
		result := remapExprHandles(ExprUnary{Op: UnaryNegate, Expr: 1}, remap)
		un := result.(ExprUnary)
		if un.Expr != 0 {
			t.Errorf("got expr=%d, want 0", un.Expr)
		}
	})

	t.Run("Access", func(t *testing.T) {
		result := remapExprHandles(ExprAccess{Base: 0, Index: 1}, remap)
		acc := result.(ExprAccess)
		if acc.Base != 2 || acc.Index != 0 {
			t.Errorf("got base=%d index=%d, want base=2 index=0", acc.Base, acc.Index)
		}
	})

	t.Run("AccessIndex", func(t *testing.T) {
		result := remapExprHandles(ExprAccessIndex{Base: 1, Index: 5}, remap)
		ai := result.(ExprAccessIndex)
		if ai.Base != 0 {
			t.Errorf("got base=%d, want 0", ai.Base)
		}
		if ai.Index != 5 { // compile-time index unchanged
			t.Errorf("got index=%d, want 5", ai.Index)
		}
	})

	t.Run("Compose", func(t *testing.T) {
		result := remapExprHandles(ExprCompose{Type: 0, Components: []ExpressionHandle{0, 1, 2}}, remap)
		comp := result.(ExprCompose)
		if comp.Components[0] != 2 || comp.Components[1] != 0 || comp.Components[2] != 1 {
			t.Errorf("got components %v, want [2,0,1]", comp.Components)
		}
	})

	t.Run("Load", func(t *testing.T) {
		result := remapExprHandles(ExprLoad{Pointer: 0}, remap)
		ld := result.(ExprLoad)
		if ld.Pointer != 2 {
			t.Errorf("got pointer=%d, want 2", ld.Pointer)
		}
	})

	t.Run("Select", func(t *testing.T) {
		result := remapExprHandles(ExprSelect{Condition: 0, Accept: 1, Reject: 2}, remap)
		sel := result.(ExprSelect)
		if sel.Condition != 2 || sel.Accept != 0 || sel.Reject != 1 {
			t.Errorf("got cond=%d accept=%d reject=%d", sel.Condition, sel.Accept, sel.Reject)
		}
	})

	t.Run("Math", func(t *testing.T) {
		arg1 := ExpressionHandle(1)
		result := remapExprHandles(ExprMath{Fun: MathClamp, Arg: 0, Arg1: &arg1}, remap)
		m := result.(ExprMath)
		if m.Arg != 2 {
			t.Errorf("got arg=%d, want 2", m.Arg)
		}
		if m.Arg1 == nil || *m.Arg1 != 0 {
			t.Errorf("got arg1=%v, want 0", m.Arg1)
		}
	})

	t.Run("Literal passthrough", func(t *testing.T) {
		result := remapExprHandles(Literal{Value: LiteralF32(1.0)}, remap)
		if _, ok := result.(Literal); !ok {
			t.Error("literal should pass through")
		}
	})

	t.Run("Splat", func(t *testing.T) {
		result := remapExprHandles(ExprSplat{Size: Vec4, Value: 1}, remap)
		sp := result.(ExprSplat)
		if sp.Value != 0 {
			t.Errorf("got value=%d, want 0", sp.Value)
		}
	})

	t.Run("Swizzle", func(t *testing.T) {
		result := remapExprHandles(ExprSwizzle{Size: Vec2, Vector: 0}, remap)
		sw := result.(ExprSwizzle)
		if sw.Vector != 2 {
			t.Errorf("got vector=%d, want 2", sw.Vector)
		}
	})

	t.Run("As", func(t *testing.T) {
		result := remapExprHandles(ExprAs{Expr: 1, Kind: ScalarFloat}, remap)
		as := result.(ExprAs)
		if as.Expr != 0 {
			t.Errorf("got expr=%d, want 0", as.Expr)
		}
	})

	t.Run("ArrayLength", func(t *testing.T) {
		result := remapExprHandles(ExprArrayLength{Array: 0}, remap)
		al := result.(ExprArrayLength)
		if al.Array != 2 {
			t.Errorf("got array=%d, want 2", al.Array)
		}
	})

	t.Run("Derivative", func(t *testing.T) {
		result := remapExprHandles(ExprDerivative{Axis: DerivativeX, Control: DerivativeFine, Expr: 1}, remap)
		d := result.(ExprDerivative)
		if d.Expr != 0 {
			t.Errorf("got expr=%d, want 0", d.Expr)
		}
	})

	t.Run("Relational", func(t *testing.T) {
		result := remapExprHandles(ExprRelational{Fun: RelationalIsNan, Argument: 2}, remap)
		r := result.(ExprRelational)
		if r.Argument != 1 {
			t.Errorf("got argument=%d, want 1", r.Argument)
		}
	})
}

// --- remapSampleLevel tests ---

func TestRemapSampleLevel(t *testing.T) {
	rm := func(h ExpressionHandle) ExpressionHandle { return h + 10 }

	t.Run("nil", func(t *testing.T) {
		result := remapSampleLevel(nil, rm)
		if result != nil {
			t.Error("expected nil")
		}
	})

	t.Run("auto unchanged", func(t *testing.T) {
		result := remapSampleLevel(SampleLevelAuto{}, rm)
		if _, ok := result.(SampleLevelAuto); !ok {
			t.Error("expected SampleLevelAuto")
		}
	})

	t.Run("zero unchanged", func(t *testing.T) {
		result := remapSampleLevel(SampleLevelZero{}, rm)
		if _, ok := result.(SampleLevelZero); !ok {
			t.Error("expected SampleLevelZero")
		}
	})

	t.Run("exact remaps level", func(t *testing.T) {
		result := remapSampleLevel(SampleLevelExact{Level: 5}, rm)
		exact := result.(SampleLevelExact)
		if exact.Level != 15 {
			t.Errorf("got level=%d, want 15", exact.Level)
		}
	})

	t.Run("bias remaps bias", func(t *testing.T) {
		result := remapSampleLevel(SampleLevelBias{Bias: 3}, rm)
		bias := result.(SampleLevelBias)
		if bias.Bias != 13 {
			t.Errorf("got bias=%d, want 13", bias.Bias)
		}
	})

	t.Run("gradient remaps x and y", func(t *testing.T) {
		result := remapSampleLevel(SampleLevelGradient{X: 1, Y: 2}, rm)
		grad := result.(SampleLevelGradient)
		if grad.X != 11 || grad.Y != 12 {
			t.Errorf("got x=%d y=%d, want x=11 y=12", grad.X, grad.Y)
		}
	})
}

// --- markExprHandleRefs tests ---

func TestMarkExprHandleRefs(t *testing.T) {
	refs := make([]bool, 10)

	tests := []struct {
		name string
		kind ExpressionKind
		want []int
	}{
		{"Compose", ExprCompose{Components: []ExpressionHandle{1, 3, 5}}, []int{1, 3, 5}},
		{"Splat", ExprSplat{Value: 2}, []int{2}},
		{"Swizzle", ExprSwizzle{Vector: 4}, []int{4}},
		{"Access", ExprAccess{Base: 0, Index: 1}, []int{0, 1}},
		{"AccessIndex", ExprAccessIndex{Base: 3}, []int{3}},
		{"Load", ExprLoad{Pointer: 6}, []int{6}},
		{"Unary", ExprUnary{Expr: 7}, []int{7}},
		{"Binary", ExprBinary{Left: 2, Right: 8}, []int{2, 8}},
		{"Select", ExprSelect{Condition: 1, Accept: 3, Reject: 5}, []int{1, 3, 5}},
		{"Relational", ExprRelational{Argument: 4}, []int{4}},
		{"As", ExprAs{Expr: 9}, []int{9}},
		{"ArrayLength", ExprArrayLength{Array: 0}, []int{0}},
		{"Derivative", ExprDerivative{Expr: 6}, []int{6}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range refs {
				refs[i] = false
			}
			markExprHandleRefs(tt.kind, refs)
			for _, idx := range tt.want {
				if !refs[idx] {
					t.Errorf("expected index %d to be marked", idx)
				}
			}
		})
	}
}

// --- traceStatementsForRefs tests ---

func TestTraceStatementsForRefs(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{Name: "helper", Expressions: []Expression{
				{Kind: ExprGlobalVariable{Variable: 1}},
			}},
		},
		GlobalVariables: []GlobalVariable{
			{Name: "g0"},
			{Name: "g1"},
		},
	}

	usedGlobals := make([]bool, 2)
	usedFunctions := make([]bool, 1)

	stmts := []Statement{
		{Kind: StmtCall{Function: 0}},
	}

	traceStatementsForRefs(stmts, usedGlobals, usedFunctions, module, func(f *Function) {
		for _, expr := range f.Expressions {
			if gv, ok := expr.Kind.(ExprGlobalVariable); ok {
				usedGlobals[gv.Variable] = true
			}
		}
	})

	if !usedFunctions[0] {
		t.Error("expected function 0 to be marked as used")
	}
	if !usedGlobals[1] {
		t.Error("expected global 1 to be marked (via function call)")
	}
}

func TestTraceStatementsForRefs_NestedBlocks(t *testing.T) {
	module := &Module{
		Functions: []Function{
			{Name: "f0"},
			{Name: "f1"},
		},
	}

	usedGlobals := make([]bool, 1)
	usedFunctions := make([]bool, 2)

	stmts := []Statement{
		{Kind: StmtIf{
			Condition: 0,
			Accept: []Statement{
				{Kind: StmtCall{Function: 0}},
			},
			Reject: []Statement{
				{Kind: StmtCall{Function: 1}},
			},
		}},
	}

	traceStatementsForRefs(stmts, usedGlobals, usedFunctions, module, func(_ *Function) {})

	if !usedFunctions[0] {
		t.Error("expected function 0 marked (in if-accept)")
	}
	if !usedFunctions[1] {
		t.Error("expected function 1 marked (in if-reject)")
	}
}

// --- remapGatherMode tests ---

func TestRemapGatherMode(t *testing.T) {
	rm := func(h ExpressionHandle) ExpressionHandle { return h + 100 }

	t.Run("Broadcast", func(t *testing.T) {
		result := remapGatherMode(GatherBroadcast{Index: 5}, rm)
		if g, ok := result.(GatherBroadcast); !ok || g.Index != 105 {
			t.Errorf("expected index 105, got %v", result)
		}
	})

	t.Run("Shuffle", func(t *testing.T) {
		result := remapGatherMode(GatherShuffle{Index: 3}, rm)
		if g, ok := result.(GatherShuffle); !ok || g.Index != 103 {
			t.Errorf("expected index 103, got %v", result)
		}
	})

	t.Run("ShuffleDown", func(t *testing.T) {
		result := remapGatherMode(GatherShuffleDown{Delta: 1}, rm)
		if g, ok := result.(GatherShuffleDown); !ok || g.Delta != 101 {
			t.Errorf("expected delta 101, got %v", result)
		}
	})

	t.Run("ShuffleUp", func(t *testing.T) {
		result := remapGatherMode(GatherShuffleUp{Delta: 2}, rm)
		if g, ok := result.(GatherShuffleUp); !ok || g.Delta != 102 {
			t.Errorf("expected delta 102, got %v", result)
		}
	})

	t.Run("ShuffleXor", func(t *testing.T) {
		result := remapGatherMode(GatherShuffleXor{Mask: 7}, rm)
		if g, ok := result.(GatherShuffleXor); !ok || g.Mask != 107 {
			t.Errorf("expected mask 107, got %v", result)
		}
	})

	t.Run("QuadBroadcast", func(t *testing.T) {
		result := remapGatherMode(GatherQuadBroadcast{Index: 0}, rm)
		if g, ok := result.(GatherQuadBroadcast); !ok || g.Index != 100 {
			t.Errorf("expected index 100, got %v", result)
		}
	})

	t.Run("BroadcastFirst passthrough", func(t *testing.T) {
		result := remapGatherMode(GatherBroadcastFirst{}, rm)
		if _, ok := result.(GatherBroadcastFirst); !ok {
			t.Error("expected GatherBroadcastFirst passthrough")
		}
	})

	t.Run("QuadSwap passthrough", func(t *testing.T) {
		result := remapGatherMode(GatherQuadSwap{Direction: QuadDirectionX}, rm)
		if _, ok := result.(GatherQuadSwap); !ok {
			t.Error("expected GatherQuadSwap passthrough")
		}
	})
}

// --- remapRayQueryFunction tests ---

func TestRemapRayQueryFunction(t *testing.T) {
	rm := func(h ExpressionHandle) ExpressionHandle { return h + 10 }

	t.Run("Initialize", func(t *testing.T) {
		result := remapRayQueryFunction(RayQueryInitialize{AccelerationStructure: 1, Descriptor: 2}, rm)
		init := result.(RayQueryInitialize)
		if init.AccelerationStructure != 11 || init.Descriptor != 12 {
			t.Errorf("got as=%d desc=%d, want 11, 12", init.AccelerationStructure, init.Descriptor)
		}
	})

	t.Run("Proceed", func(t *testing.T) {
		result := remapRayQueryFunction(RayQueryProceed{Result: 3}, rm)
		proc := result.(RayQueryProceed)
		if proc.Result != 13 {
			t.Errorf("got result=%d, want 13", proc.Result)
		}
	})

	t.Run("GenerateIntersection", func(t *testing.T) {
		result := remapRayQueryFunction(RayQueryGenerateIntersection{HitT: 5}, rm)
		gen := result.(RayQueryGenerateIntersection)
		if gen.HitT != 15 {
			t.Errorf("got hitT=%d, want 15", gen.HitT)
		}
	})

	t.Run("Terminate passthrough", func(t *testing.T) {
		result := remapRayQueryFunction(RayQueryTerminate{}, rm)
		if _, ok := result.(RayQueryTerminate); !ok {
			t.Error("expected RayQueryTerminate passthrough")
		}
	})
}

// --- remapAtomicFunction tests ---

func TestRemapAtomicFunction(t *testing.T) {
	rmOpt := func(h *ExpressionHandle) *ExpressionHandle {
		if h == nil {
			return nil
		}
		v := *h + 10
		return &v
	}

	t.Run("Exchange with compare", func(t *testing.T) {
		cmp := ExpressionHandle(5)
		result := remapAtomicFunction(AtomicExchange{Compare: &cmp}, rmOpt)
		ex := result.(AtomicExchange)
		if ex.Compare == nil || *ex.Compare != 15 {
			t.Errorf("expected compare=15, got %v", ex.Compare)
		}
	})

	t.Run("Exchange without compare", func(t *testing.T) {
		result := remapAtomicFunction(AtomicExchange{Compare: nil}, rmOpt)
		ex := result.(AtomicExchange)
		if ex.Compare != nil {
			t.Error("expected nil compare")
		}
	})

	t.Run("Add passthrough", func(t *testing.T) {
		result := remapAtomicFunction(AtomicAdd{}, rmOpt)
		if _, ok := result.(AtomicAdd); !ok {
			t.Error("expected AtomicAdd passthrough")
		}
	})
}

// --- visitExprTypeHandles tests ---

func TestVisitExprTypeHandles(t *testing.T) {
	var visited []TypeHandle
	visit := func(h TypeHandle) { visited = append(visited, h) }

	tests := []struct {
		name string
		kind ExpressionKind
		want []TypeHandle
	}{
		{"ZeroValue", ExprZeroValue{Type: 3}, []TypeHandle{3}},
		{"Compose", ExprCompose{Type: 5}, []TypeHandle{5}},
		{"SubgroupOperationResult", ExprSubgroupOperationResult{Type: 7}, []TypeHandle{7}},
		{"AtomicResult", ExprAtomicResult{Ty: 1}, []TypeHandle{1}},
		{"Literal no visits", Literal{Value: LiteralF32(0)}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visited = visited[:0]
			visitExprTypeHandles(tt.kind, visit)
			if len(visited) != len(tt.want) {
				t.Fatalf("visited %d handles, want %d", len(visited), len(tt.want))
			}
			for i, h := range visited {
				if h != tt.want[i] {
					t.Errorf("visited[%d] = %d, want %d", i, h, tt.want[i])
				}
			}
		})
	}
}

// --- markSampleLevelRefs tests ---

func TestMarkSampleLevelRefs(t *testing.T) {
	refs := make([]bool, 10)
	mark := func(h ExpressionHandle) {
		if int(h) < len(refs) {
			refs[h] = true
		}
	}

	t.Run("nil", func(t *testing.T) {
		markSampleLevelRefs(nil, mark) // should not panic
	})

	t.Run("Auto", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markSampleLevelRefs(SampleLevelAuto{}, mark)
		for _, r := range refs {
			if r {
				t.Error("SampleLevelAuto should not mark anything")
			}
		}
	})

	t.Run("Exact", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markSampleLevelRefs(SampleLevelExact{Level: 3}, mark)
		if !refs[3] {
			t.Error("expected index 3 marked")
		}
	})

	t.Run("Bias", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markSampleLevelRefs(SampleLevelBias{Bias: 5}, mark)
		if !refs[5] {
			t.Error("expected index 5 marked")
		}
	})

	t.Run("Gradient", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markSampleLevelRefs(SampleLevelGradient{X: 1, Y: 7}, mark)
		if !refs[1] || !refs[7] {
			t.Error("expected indices 1 and 7 marked")
		}
	})
}

// --- markStmtExprRefs tests ---

func TestMarkStmtExprRefs(t *testing.T) {
	refs := make([]bool, 20)
	reset := func() {
		for i := range refs {
			refs[i] = false
		}
	}

	t.Run("Store marks pointer and value", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtStore{Pointer: 1, Value: 3}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] {
			t.Error("expected 1 and 3 marked")
		}
	})

	t.Run("Return marks value", func(t *testing.T) {
		reset()
		v := ExpressionHandle(5)
		stmts := []Statement{
			{Kind: StmtReturn{Value: &v}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[5] {
			t.Error("expected 5 marked")
		}
	})

	t.Run("Return nil value marks nothing", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtReturn{Value: nil}},
		}
		markStmtExprRefs(stmts, refs)
		for i, r := range refs {
			if r {
				t.Errorf("unexpected mark at %d", i)
			}
		}
	})

	t.Run("If marks condition and recurses", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtIf{
				Condition: 2,
				Accept:    Block{{Kind: StmtStore{Pointer: 4, Value: 6}}},
				Reject:    Block{{Kind: StmtStore{Pointer: 8, Value: 10}}},
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[2] || !refs[4] || !refs[6] || !refs[8] || !refs[10] {
			t.Error("expected 2,4,6,8,10 marked")
		}
	})

	t.Run("Switch marks selector and case bodies", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtSwitch{
				Selector: 1,
				Cases: []SwitchCase{
					{Body: Block{{Kind: StmtStore{Pointer: 3, Value: 5}}}},
				},
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("Loop marks body, continuing, and break_if", func(t *testing.T) {
		reset()
		breakIf := ExpressionHandle(7)
		stmts := []Statement{
			{Kind: StmtLoop{
				Body:       Block{{Kind: StmtStore{Pointer: 2, Value: 4}}},
				Continuing: Block{{Kind: StmtStore{Pointer: 6, Value: 8}}},
				BreakIf:    &breakIf,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[2] || !refs[4] || !refs[6] || !refs[7] || !refs[8] {
			t.Error("expected 2,4,6,7,8 marked")
		}
	})

	t.Run("Block recurses", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtBlock{Block: Block{{Kind: StmtStore{Pointer: 1, Value: 2}}}}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[2] {
			t.Error("expected 1,2 marked")
		}
	})

	t.Run("Call marks arguments and result", func(t *testing.T) {
		reset()
		result := ExpressionHandle(9)
		stmts := []Statement{
			{Kind: StmtCall{
				Function:  0,
				Arguments: []ExpressionHandle{3, 5},
				Result:    &result,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[3] || !refs[5] || !refs[9] {
			t.Error("expected 3,5,9 marked")
		}
	})

	t.Run("ImageStore marks all fields", func(t *testing.T) {
		reset()
		arrIdx := ExpressionHandle(11)
		stmts := []Statement{
			{Kind: StmtImageStore{
				Image:      1,
				Coordinate: 3,
				ArrayIndex: &arrIdx,
				Value:      5,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] || !refs[11] {
			t.Error("expected 1,3,5,11 marked")
		}
	})

	t.Run("Emit marks range", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtEmit{Range: Range{Start: 3, End: 7}}},
		}
		markStmtExprRefs(stmts, refs)
		for h := 3; h < 7; h++ {
			if !refs[h] {
				t.Errorf("expected %d marked", h)
			}
		}
	})

	t.Run("Atomic marks pointer, value, result", func(t *testing.T) {
		reset()
		result := ExpressionHandle(12)
		stmts := []Statement{
			{Kind: StmtAtomic{
				Pointer: 1,
				Fun:     AtomicAdd{},
				Value:   3,
				Result:  &result,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] || !refs[12] {
			t.Error("expected 1,3,12 marked")
		}
	})

	t.Run("WorkGroupUniformLoad marks pointer and result", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtWorkGroupUniformLoad{Pointer: 2, Result: 4}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[2] || !refs[4] {
			t.Error("expected 2,4 marked")
		}
	})

	t.Run("RayQuery marks query and function refs", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtRayQuery{
				Query: 1,
				Fun:   RayQueryInitialize{AccelerationStructure: 3, Descriptor: 5},
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("SubgroupBallot marks predicate and result", func(t *testing.T) {
		reset()
		pred := ExpressionHandle(2)
		stmts := []Statement{
			{Kind: StmtSubgroupBallot{Result: 4, Predicate: &pred}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[2] || !refs[4] {
			t.Error("expected 2,4 marked")
		}
	})

	t.Run("SubgroupGather marks mode, argument, result", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtSubgroupGather{
				Mode:     GatherBroadcast{Index: 1},
				Argument: 3,
				Result:   5,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("ImageAtomic marks all fields", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtImageAtomic{
				Image:      1,
				Coordinate: 3,
				Fun:        AtomicAdd{},
				Value:      5,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("SubgroupCollectiveOperation marks argument and result", func(t *testing.T) {
		reset()
		stmts := []Statement{
			{Kind: StmtSubgroupCollectiveOperation{
				Argument: 2,
				Result:   4,
			}},
		}
		markStmtExprRefs(stmts, refs)
		if !refs[2] || !refs[4] {
			t.Error("expected 2,4 marked")
		}
	})
}

// --- markRayQueryFunctionRefs tests ---

func TestMarkRayQueryFunctionRefs(t *testing.T) {
	refs := make([]bool, 10)
	mark := func(h ExpressionHandle) {
		if int(h) < len(refs) {
			refs[h] = true
		}
	}

	t.Run("Initialize", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markRayQueryFunctionRefs(RayQueryInitialize{AccelerationStructure: 1, Descriptor: 3}, mark)
		if !refs[1] || !refs[3] {
			t.Error("expected 1,3 marked")
		}
	})

	t.Run("Proceed", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markRayQueryFunctionRefs(RayQueryProceed{Result: 5}, mark)
		if !refs[5] {
			t.Error("expected 5 marked")
		}
	})

	t.Run("GenerateIntersection", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markRayQueryFunctionRefs(RayQueryGenerateIntersection{HitT: 7}, mark)
		if !refs[7] {
			t.Error("expected 7 marked")
		}
	})

	t.Run("Terminate marks nothing", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markRayQueryFunctionRefs(RayQueryTerminate{}, mark)
		for i, r := range refs {
			if r {
				t.Errorf("unexpected mark at %d", i)
			}
		}
	})
}

// --- markAtomicFunctionRefs tests ---

func TestMarkAtomicFunctionRefs(t *testing.T) {
	refs := make([]bool, 10)
	markOpt := func(h *ExpressionHandle) {
		if h != nil && int(*h) < len(refs) {
			refs[*h] = true
		}
	}

	t.Run("Exchange with compare", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		cmp := ExpressionHandle(3)
		markAtomicFunctionRefs(AtomicExchange{Compare: &cmp}, markOpt)
		if !refs[3] {
			t.Error("expected 3 marked")
		}
	})

	t.Run("Exchange without compare", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markAtomicFunctionRefs(AtomicExchange{Compare: nil}, markOpt)
		for i, r := range refs {
			if r {
				t.Errorf("unexpected mark at %d", i)
			}
		}
	})

	t.Run("Add marks nothing", func(t *testing.T) {
		for i := range refs {
			refs[i] = false
		}
		markAtomicFunctionRefs(AtomicAdd{}, markOpt)
		for i, r := range refs {
			if r {
				t.Errorf("unexpected mark at %d", i)
			}
		}
	})
}

// --- markGatherModeRefs tests ---

func TestMarkGatherModeRefs(t *testing.T) {
	refs := make([]bool, 10)
	mark := func(h ExpressionHandle) {
		if int(h) < len(refs) {
			refs[h] = true
		}
	}
	reset := func() {
		for i := range refs {
			refs[i] = false
		}
	}

	t.Run("Broadcast marks index", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherBroadcast{Index: 2}, mark)
		if !refs[2] {
			t.Error("expected 2 marked")
		}
	})

	t.Run("Shuffle marks index", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherShuffle{Index: 4}, mark)
		if !refs[4] {
			t.Error("expected 4 marked")
		}
	})

	t.Run("ShuffleDown marks delta", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherShuffleDown{Delta: 1}, mark)
		if !refs[1] {
			t.Error("expected 1 marked")
		}
	})

	t.Run("ShuffleUp marks delta", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherShuffleUp{Delta: 3}, mark)
		if !refs[3] {
			t.Error("expected 3 marked")
		}
	})

	t.Run("ShuffleXor marks mask", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherShuffleXor{Mask: 5}, mark)
		if !refs[5] {
			t.Error("expected 5 marked")
		}
	})

	t.Run("QuadBroadcast marks index", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherQuadBroadcast{Index: 7}, mark)
		if !refs[7] {
			t.Error("expected 7 marked")
		}
	})

	t.Run("BroadcastFirst marks nothing", func(t *testing.T) {
		reset()
		markGatherModeRefs(GatherBroadcastFirst{}, mark)
		for i, r := range refs {
			if r {
				t.Errorf("unexpected mark at %d", i)
			}
		}
	})
}

// --- remapStmtExprHandles tests ---

func TestRemapStmtExprHandles(t *testing.T) {
	remap := []ExpressionHandle{10, 20, 30, 40, 50}

	t.Run("Store", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtStore{Pointer: 0, Value: 1}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtStore)
		if s.Pointer != 10 || s.Value != 20 {
			t.Errorf("got pointer=%d value=%d, want 10, 20", s.Pointer, s.Value)
		}
	})

	t.Run("Return with value", func(t *testing.T) {
		v := ExpressionHandle(2)
		stmts := []Statement{
			{Kind: StmtReturn{Value: &v}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtReturn)
		if s.Value == nil || *s.Value != 30 {
			t.Errorf("got %v, want 30", s.Value)
		}
	})

	t.Run("If remaps condition", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtIf{
				Condition: 0,
				Accept:    Block{{Kind: StmtStore{Pointer: 1, Value: 2}}},
				Reject:    Block{},
			}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtIf)
		if s.Condition != 10 {
			t.Errorf("condition = %d, want 10", s.Condition)
		}
		store := s.Accept[0].Kind.(StmtStore)
		if store.Pointer != 20 {
			t.Errorf("inner store pointer = %d, want 20", store.Pointer)
		}
	})

	t.Run("Switch remaps selector", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtSwitch{
				Selector: 1,
				Cases: []SwitchCase{
					{Body: Block{{Kind: StmtStore{Pointer: 2, Value: 3}}}},
				},
			}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtSwitch)
		if s.Selector != 20 {
			t.Errorf("selector = %d, want 20", s.Selector)
		}
	})

	t.Run("Loop remaps breakIf", func(t *testing.T) {
		bi := ExpressionHandle(3)
		stmts := []Statement{
			{Kind: StmtLoop{
				Body:       Block{},
				Continuing: Block{},
				BreakIf:    &bi,
			}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtLoop)
		if s.BreakIf == nil || *s.BreakIf != 40 {
			t.Errorf("breakIf = %v, want 40", s.BreakIf)
		}
	})

	t.Run("Call remaps arguments and result", func(t *testing.T) {
		result := ExpressionHandle(4)
		stmts := []Statement{
			{Kind: StmtCall{
				Function:  0,
				Arguments: []ExpressionHandle{0, 1},
				Result:    &result,
			}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtCall)
		if s.Arguments[0] != 10 || s.Arguments[1] != 20 {
			t.Errorf("args = %v, want [10, 20]", s.Arguments)
		}
		if s.Result == nil || *s.Result != 50 {
			t.Errorf("result = %v, want 50", s.Result)
		}
	})

	t.Run("Emit remaps range", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtEmit{Range: Range{Start: 1, End: 3}}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtEmit)
		// remap[1]=20, remap[3]=40
		if s.Range.Start != 20 || s.Range.End != 40 {
			t.Errorf("range = [%d,%d), want [20,40)", s.Range.Start, s.Range.End)
		}
	})

	t.Run("ImageStore remaps", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtImageStore{Image: 0, Coordinate: 1, Value: 2}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtImageStore)
		if s.Image != 10 || s.Coordinate != 20 || s.Value != 30 {
			t.Errorf("got image=%d coord=%d val=%d", s.Image, s.Coordinate, s.Value)
		}
	})

	t.Run("Block recurses", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtBlock{Block: Block{{Kind: StmtStore{Pointer: 0, Value: 1}}}}},
		}
		remapStmtExprHandles(stmts, remap)
		blk := stmts[0].Kind.(StmtBlock)
		s := blk.Block[0].Kind.(StmtStore)
		if s.Pointer != 10 || s.Value != 20 {
			t.Errorf("got pointer=%d value=%d", s.Pointer, s.Value)
		}
	})

	t.Run("Atomic remaps", func(t *testing.T) {
		result := ExpressionHandle(4)
		stmts := []Statement{
			{Kind: StmtAtomic{Pointer: 0, Fun: AtomicAdd{}, Value: 1, Result: &result}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtAtomic)
		if s.Pointer != 10 || s.Value != 20 || *s.Result != 50 {
			t.Errorf("got p=%d v=%d r=%d", s.Pointer, s.Value, *s.Result)
		}
	})

	t.Run("WorkGroupUniformLoad remaps", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtWorkGroupUniformLoad{Pointer: 0, Result: 1}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtWorkGroupUniformLoad)
		if s.Pointer != 10 || s.Result != 20 {
			t.Errorf("got p=%d r=%d", s.Pointer, s.Result)
		}
	})

	t.Run("RayQuery remaps", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtRayQuery{Query: 0, Fun: RayQueryProceed{Result: 1}}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtRayQuery)
		if s.Query != 10 {
			t.Errorf("query = %d, want 10", s.Query)
		}
	})

	t.Run("SubgroupBallot remaps", func(t *testing.T) {
		pred := ExpressionHandle(0)
		stmts := []Statement{
			{Kind: StmtSubgroupBallot{Result: 1, Predicate: &pred}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtSubgroupBallot)
		if s.Result != 20 || *s.Predicate != 10 {
			t.Errorf("got r=%d p=%d", s.Result, *s.Predicate)
		}
	})

	t.Run("SubgroupGather remaps", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtSubgroupGather{Mode: GatherBroadcast{Index: 0}, Argument: 1, Result: 2}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtSubgroupGather)
		if s.Argument != 20 || s.Result != 30 {
			t.Errorf("got arg=%d r=%d", s.Argument, s.Result)
		}
	})

	t.Run("ImageAtomic remaps", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtImageAtomic{Image: 0, Coordinate: 1, Fun: AtomicAdd{}, Value: 2}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtImageAtomic)
		if s.Image != 10 || s.Coordinate != 20 || s.Value != 30 {
			t.Errorf("got img=%d coord=%d val=%d", s.Image, s.Coordinate, s.Value)
		}
	})

	t.Run("SubgroupCollectiveOperation remaps", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtSubgroupCollectiveOperation{Argument: 0, Result: 1}},
		}
		remapStmtExprHandles(stmts, remap)
		s := stmts[0].Kind.(StmtSubgroupCollectiveOperation)
		if s.Argument != 10 || s.Result != 20 {
			t.Errorf("got arg=%d r=%d", s.Argument, s.Result)
		}
	})
}

// --- visitFunctionTypes tests ---

func TestVisitFunctionTypes(t *testing.T) {
	var visited []TypeHandle
	visit := func(h TypeHandle) { visited = append(visited, h) }

	f := &Function{
		Arguments: []FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 1},
		},
		Result: &FunctionResult{Type: 2},
		LocalVars: []LocalVariable{
			{Name: "x", Type: 3},
		},
		Expressions: []Expression{
			{Kind: ExprZeroValue{Type: 4}},
			{Kind: ExprCompose{Type: 5}},
			{Kind: Literal{Value: LiteralF32(1.0)}}, // no type handle
		},
	}

	visitFunctionTypes(f, visit)

	want := []TypeHandle{0, 1, 2, 3, 4, 5}
	if len(visited) != len(want) {
		t.Fatalf("visited %d handles, want %d", len(visited), len(want))
	}
	for i, h := range visited {
		if h != want[i] {
			t.Errorf("visited[%d] = %d, want %d", i, h, want[i])
		}
	}
}

func TestVisitFunctionTypes_NoResult(t *testing.T) {
	var visited []TypeHandle
	visit := func(h TypeHandle) { visited = append(visited, h) }

	f := &Function{
		Arguments: []FunctionArgument{{Name: "a", Type: 0}},
		Result:    nil,
	}

	visitFunctionTypes(f, visit)

	if len(visited) != 1 || visited[0] != 0 {
		t.Errorf("expected [0], got %v", visited)
	}
}

// --- safeRemapFunctionTypes tests ---

func TestSafeRemapFunctionTypes(t *testing.T) {
	remap := []TypeHandle{1, 0, 2}

	f := &Function{
		Arguments: []FunctionArgument{
			{Name: "a", Type: 0},
		},
		Result: &FunctionResult{Type: 1},
		LocalVars: []LocalVariable{
			{Name: "x", Type: 2},
		},
		Expressions: []Expression{
			{Kind: ExprZeroValue{Type: 0}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(1); return &h }()},
		},
	}

	safeRemapFunctionTypes(f, remap)

	if f.Arguments[0].Type != 1 {
		t.Errorf("arg type = %d, want 1", f.Arguments[0].Type)
	}
	if f.Result.Type != 0 {
		t.Errorf("result type = %d, want 0", f.Result.Type)
	}
	if f.LocalVars[0].Type != 2 {
		t.Errorf("local var type = %d, want 2", f.LocalVars[0].Type)
	}
	zv := f.Expressions[0].Kind.(ExprZeroValue)
	if zv.Type != 1 {
		t.Errorf("zero value type = %d, want 1", zv.Type)
	}
	if f.ExpressionTypes[0].Handle == nil || *f.ExpressionTypes[0].Handle != 0 {
		t.Errorf("expression type handle = %v, want 0", f.ExpressionTypes[0].Handle)
	}
}

func TestSafeRemapFunctionTypes_SentinelHandle(t *testing.T) {
	remap := []TypeHandle{1, 0}
	sentinel := ^TypeHandle(0)

	f := &Function{
		ExpressionTypes: []TypeResolution{
			{Handle: &sentinel},
		},
	}

	safeRemapFunctionTypes(f, remap)

	// Sentinel should remain unchanged
	if f.ExpressionTypes[0].Handle == nil || *f.ExpressionTypes[0].Handle != sentinel {
		t.Errorf("sentinel handle should remain unchanged, got %v", f.ExpressionTypes[0].Handle)
	}
}

// --- remapStmtFuncHandles tests ---

func TestRemapStmtFuncHandles(t *testing.T) {
	remap := []FunctionHandle{10, 20}

	t.Run("Call", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtCall{Function: 0, Arguments: nil}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(StmtCall)
		if s.Function != 10 {
			t.Errorf("function = %d, want 10", s.Function)
		}
	})

	t.Run("Block recurses", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtBlock{Block: Block{{Kind: StmtCall{Function: 1}}}}},
		}
		remapStmtFuncHandles(stmts, remap)
		blk := stmts[0].Kind.(StmtBlock)
		s := blk.Block[0].Kind.(StmtCall)
		if s.Function != 20 {
			t.Errorf("function = %d, want 20", s.Function)
		}
	})

	t.Run("If recurses", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtIf{
				Accept: Block{{Kind: StmtCall{Function: 0}}},
				Reject: Block{{Kind: StmtCall{Function: 1}}},
			}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(StmtIf)
		if s.Accept[0].Kind.(StmtCall).Function != 10 {
			t.Error("accept call not remapped")
		}
		if s.Reject[0].Kind.(StmtCall).Function != 20 {
			t.Error("reject call not remapped")
		}
	})

	t.Run("Switch recurses", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtSwitch{
				Cases: []SwitchCase{
					{Body: Block{{Kind: StmtCall{Function: 0}}}},
				},
			}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(StmtSwitch)
		if s.Cases[0].Body[0].Kind.(StmtCall).Function != 10 {
			t.Error("switch case call not remapped")
		}
	})

	t.Run("Loop recurses", func(t *testing.T) {
		stmts := []Statement{
			{Kind: StmtLoop{
				Body:       Block{{Kind: StmtCall{Function: 0}}},
				Continuing: Block{{Kind: StmtCall{Function: 1}}},
			}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(StmtLoop)
		if s.Body[0].Kind.(StmtCall).Function != 10 {
			t.Error("loop body call not remapped")
		}
		if s.Continuing[0].Kind.(StmtCall).Function != 20 {
			t.Error("loop continuing call not remapped")
		}
	})
}

// --- CompactExpressions with rich statement trees (covers markStmtExprRefsForCompact + remapStmtExprHandlesCompact) ---

func TestCompactExpressions_IfStatement(t *testing.T) {
	retVal := ExpressionHandle(3)
	module := &Module{
		EntryPoints: []EntryPoint{{
			Name:  "main",
			Stage: StageVertex,
			Function: Function{
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralBool(true)}}, // 0: condition
					{Kind: Literal{Value: LiteralF32(1.0)}},   // 1: used in accept store
					{Kind: Literal{Value: LiteralF32(2.0)}},   // 2: unused
					{Kind: Literal{Value: LiteralF32(3.0)}},   // 3: used by return
				},
				Body: []Statement{
					{Kind: StmtIf{
						Condition: 0,
						Accept:    Block{{Kind: StmtStore{Pointer: 1, Value: 1}}},
						Reject:    Block{},
					}},
					{Kind: StmtReturn{Value: &retVal}},
				},
			},
		}},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// Expression 2 (unused) should be removed: 0,1,3 -> 0,1,2
	if len(fn.Expressions) != 3 {
		t.Fatalf("expected 3 expressions, got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_SwitchStatement(t *testing.T) {
	module := &Module{
		EntryPoints: []EntryPoint{{
			Name:  "main",
			Stage: StageVertex,
			Function: Function{
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralI32(1)}},   // 0: selector
					{Kind: Literal{Value: LiteralF32(1.0)}}, // 1: used in case body
					{Kind: Literal{Value: LiteralF32(2.0)}}, // 2: unused
				},
				Body: []Statement{
					{Kind: StmtSwitch{
						Selector: 0,
						Cases: []SwitchCase{
							{
								Value: SwitchValueDefault{},
								Body:  Block{{Kind: StmtStore{Pointer: 1, Value: 1}}},
							},
						},
					}},
				},
			},
		}},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 2 {
		t.Fatalf("expected 2 expressions (unused removed), got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_LoopStatement(t *testing.T) {
	breakIfH := ExpressionHandle(2)
	module := &Module{
		EntryPoints: []EntryPoint{{
			Name:  "main",
			Stage: StageVertex,
			Function: Function{
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}},   // 0: used in loop body
					{Kind: Literal{Value: LiteralF32(2.0)}},   // 1: unused
					{Kind: Literal{Value: LiteralBool(true)}}, // 2: break_if
				},
				Body: []Statement{
					{Kind: StmtLoop{
						Body:       Block{{Kind: StmtStore{Pointer: 0, Value: 0}}},
						Continuing: Block{},
						BreakIf:    &breakIfH,
					}},
				},
			},
		}},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 2 {
		t.Fatalf("expected 2 expressions (unused removed), got %d", len(fn.Expressions))
	}
	loop := fn.Body[0].Kind.(StmtLoop)
	if loop.BreakIf == nil || *loop.BreakIf != 1 {
		t.Errorf("expected break_if remapped to 1, got %v", loop.BreakIf)
	}
}

func TestCompactExpressions_ImageStoreAndAtomic(t *testing.T) {
	resultH := ExpressionHandle(4)
	module := &Module{
		EntryPoints: []EntryPoint{{
			Name:  "main",
			Stage: StageCompute,
			Function: Function{
				Expressions: []Expression{
					{Kind: ExprGlobalVariable{Variable: 0}}, // 0: image
					{Kind: Literal{Value: LiteralU32(0)}},   // 1: coord
					{Kind: Literal{Value: LiteralF32(1.0)}}, // 2: value
					{Kind: Literal{Value: LiteralF32(9.9)}}, // 3: unused
					{Kind: ExprAtomicResult{Ty: 0}},         // 4: atomic result
				},
				Body: []Statement{
					{Kind: StmtImageStore{Image: 0, Coordinate: 1, Value: 2}},
					{Kind: StmtAtomic{Pointer: 0, Fun: AtomicAdd{}, Value: 1, Result: &resultH}},
				},
			},
		}},
	}

	CompactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// Expression 3 (unused) removed
	if len(fn.Expressions) != 4 {
		t.Fatalf("expected 4 expressions, got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_CallStatement(t *testing.T) {
	resultH := ExpressionHandle(2)
	module := &Module{
		Functions: []Function{
			{
				Name: "helper",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}}, // 0: unused
					{Kind: Literal{Value: LiteralF32(2.0)}}, // 1: used by return
				},
				Body: []Statement{
					{Kind: StmtReturn{Value: func() *ExpressionHandle { h := ExpressionHandle(1); return &h }()}},
				},
			},
		},
		EntryPoints: []EntryPoint{{
			Name:  "main",
			Stage: StageVertex,
			Function: Function{
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(1.0)}}, // 0: arg
					{Kind: Literal{Value: LiteralF32(9.9)}}, // 1: unused
					{Kind: ExprCallResult{Function: 0}},     // 2: call result
				},
				Body: []Statement{
					{Kind: StmtCall{Function: 0, Arguments: []ExpressionHandle{0}, Result: &resultH}},
				},
			},
		}},
	}

	CompactExpressions(module)

	// In helper: expr 0 removed, expr 1 -> 0
	if len(module.Functions[0].Expressions) != 1 {
		t.Fatalf("helper: expected 1 expression, got %d", len(module.Functions[0].Expressions))
	}
	// In main: expr 1 removed, 0 stays, 2 -> 1
	if len(module.EntryPoints[0].Function.Expressions) != 2 {
		t.Fatalf("main: expected 2 expressions, got %d", len(module.EntryPoints[0].Function.Expressions))
	}
}

// --- ReorderTypes with type dependencies ---

func TestReorderTypes_WithStructDependencies(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "MyStruct", Inner: StructType{Members: []StructMember{
				{Name: "x", Type: 1, Offset: 0},
			}, Span: 4}}, // 0: depends on 1
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}}, // 1: leaf
		},
		TypeUseOrder: []TypeHandle{0}, // struct registered first
	}

	ReorderTypes(module)

	// f32 (dependency) should come before MyStruct
	if module.Types[0].Name != "f32" {
		t.Errorf("expected f32 first (dependency), got %s", module.Types[0].Name)
	}
	if module.Types[1].Name != "MyStruct" {
		t.Errorf("expected MyStruct second, got %s", module.Types[1].Name)
	}
}

func TestReorderTypes_WithArrayDependency(t *testing.T) {
	size := uint32(4)
	module := &Module{
		Types: []Type{
			{Name: "arr", Inner: ArrayType{Base: 1, Size: ArraySize{Constant: &size}, Stride: 16}}, // 0: depends on 1
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},                          // 1: leaf
		},
		TypeUseOrder: []TypeHandle{0},
	}

	ReorderTypes(module)

	if module.Types[0].Name != "f32" {
		t.Errorf("expected f32 first, got %s", module.Types[0].Name)
	}
}

func TestReorderTypes_WithPointerDependency(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "ptr", Inner: PointerType{Base: 1, Space: SpaceFunction}}, // 0: depends on 1
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},     // 1: leaf
		},
		TypeUseOrder: []TypeHandle{0},
	}

	ReorderTypes(module)

	if module.Types[0].Name != "u32" {
		t.Errorf("expected u32 first, got %s", module.Types[0].Name)
	}
}

func TestReorderTypes_WithBindingArrayDependency(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "ba", Inner: BindingArrayType{Base: 1}},                        // 0
			{Name: "tex", Inner: ImageType{Dim: Dim2D, Class: ImageClassSampled}}, // 1: leaf
		},
		TypeUseOrder: []TypeHandle{0},
	}

	ReorderTypes(module)

	if module.Types[0].Name != "tex" {
		t.Errorf("expected tex first, got %s", module.Types[0].Name)
	}
}

// --- remapFunctionTypes tests ---

func TestRemapFunctionTypes(t *testing.T) {
	remap := []TypeHandle{2, 0, 1}

	f := &Function{
		Arguments: []FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 1},
		},
		Result: &FunctionResult{Type: 2},
		LocalVars: []LocalVariable{
			{Name: "x", Type: 0},
		},
		Expressions: []Expression{
			{Kind: ExprZeroValue{Type: 1}},
			{Kind: ExprCompose{Type: 2}},
		},
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()},
			{Value: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
	}

	remapFunctionTypes(f, remap)

	if f.Arguments[0].Type != 2 {
		t.Errorf("arg[0] type = %d, want 2", f.Arguments[0].Type)
	}
	if f.Arguments[1].Type != 0 {
		t.Errorf("arg[1] type = %d, want 0", f.Arguments[1].Type)
	}
	if f.Result.Type != 1 {
		t.Errorf("result type = %d, want 1", f.Result.Type)
	}
	if f.LocalVars[0].Type != 2 {
		t.Errorf("local var type = %d, want 2", f.LocalVars[0].Type)
	}
	zv := f.Expressions[0].Kind.(ExprZeroValue)
	if zv.Type != 0 {
		t.Errorf("zero value type = %d, want 0", zv.Type)
	}
	comp := f.Expressions[1].Kind.(ExprCompose)
	if comp.Type != 1 {
		t.Errorf("compose type = %d, want 1", comp.Type)
	}
	// ExpressionTypes handle remapped
	if f.ExpressionTypes[0].Handle == nil || *f.ExpressionTypes[0].Handle != 2 {
		t.Errorf("expression type handle = %v, want 2", f.ExpressionTypes[0].Handle)
	}
}

func TestRemapFunctionTypes_AbstractTypeRemoved(t *testing.T) {
	sentinel := ^TypeHandle(0)
	remap := []TypeHandle{sentinel, 0} // type 0 was abstract (removed), type 1 -> 0

	f := &Function{
		ExpressionTypes: []TypeResolution{
			{Handle: func() *TypeHandle { h := TypeHandle(0); return &h }()}, // references removed type
		},
	}

	remapFunctionTypes(f, remap)

	// Handle for removed abstract type should be set to nil
	if f.ExpressionTypes[0].Handle != nil {
		t.Errorf("expected nil handle for removed abstract type, got %v", f.ExpressionTypes[0].Handle)
	}
}
