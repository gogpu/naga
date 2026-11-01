package irutil

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// --- IsAbstractType tests ---

func TestIsAbstractType(t *testing.T) {
	types := []ir.Type{
		{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 0}},
		{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 0}},
		{Name: "", Inner: ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 0}}},
		{Name: "", Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 0}}},
		{Name: "", Inner: ir.ArrayType{Base: 1, Stride: 4}},  // base is abstract int
		{Name: "", Inner: ir.ArrayType{Base: 0, Stride: 16}}, // base is concrete f32
		{Name: "", Inner: ir.StructType{Members: nil, Span: 0}},
	}

	tests := []struct {
		name  string
		inner ir.TypeInner
		want  bool
	}{
		{"concrete scalar f32", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, false},
		{"concrete scalar i32", ir.ScalarType{Kind: ir.ScalarSint, Width: 4}, false},
		{"concrete scalar u32", ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, false},
		{"concrete scalar bool", ir.ScalarType{Kind: ir.ScalarBool, Width: 1}, false},
		{"abstract int", ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 0}, true},
		{"abstract float", ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 0}, true},
		{"vector with abstract float", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarAbstractFloat}}, true},
		{"vector with concrete float", ir.VectorType{Size: ir.Vec3, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, false},
		{"matrix with abstract int", ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarAbstractInt}}, true},
		{"matrix with concrete float", ir.MatrixType{Columns: ir.Vec4, Rows: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, false},
		{"array with abstract base", ir.ArrayType{Base: 1, Stride: 4}, true},
		{"array with concrete base", ir.ArrayType{Base: 0, Stride: 16}, false},
		{"struct is never abstract", ir.StructType{Members: nil, Span: 0}, false},
		{"pointer is never abstract", ir.PointerType{Base: 0, Space: ir.SpaceFunction}, false},
		{"sampler is never abstract", ir.SamplerType{Comparison: false}, false},
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

// --- CompactUnused tests ---

func TestCompactUnused_NoEntryPoints(t *testing.T) {
	module := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "unused", Type: 0},
		},
		Functions: []ir.Function{
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
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "used_var", Type: 0, Space: ir.SpaceUniform},
			{Name: "unused_var", Type: 0, Space: ir.SpaceUniform},
			{Name: "also_used", Type: 0, Space: ir.SpaceUniform},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}}, // references global 0
						{Kind: ir.ExprGlobalVariable{Variable: 2}}, // references global 2
					},
					Body: []ir.Statement{},
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
	gv0 := ep.Expressions[0].Kind.(ir.ExprGlobalVariable)
	gv1 := ep.Expressions[1].Kind.(ir.ExprGlobalVariable)
	if gv0.Variable != 0 {
		t.Errorf("expected remapped global 0, got %d", gv0.Variable)
	}
	if gv1.Variable != 1 {
		t.Errorf("expected remapped global 1 (was 2), got %d", gv1.Variable)
	}
}

func TestCompactUnused_RemovesUnusedFunctions(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "used_func"},
			{Name: "unused_func"},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageCompute,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.ExprCallResult{Function: 0}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtCall{Function: 0, Arguments: nil, Result: nil}},
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
	module := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g0", Type: 0},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprGlobalVariable{Variable: 0}},
					},
					Body: []ir.Statement{},
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
	gvh := ir.GlobalVariableHandle(1)
	module := &ir.Module{
		GlobalVariables: []ir.GlobalVariable{
			{Name: "unused", Type: 0},
			{Name: "payload", Type: 0},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:        "main",
				Stage:       ir.StageMesh,
				TaskPayload: &gvh,
				Function: ir.Function{
					Expressions: []ir.Expression{},
					Body:        []ir.Statement{},
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

// --- compactTypes tests ---

func TestCompactTypes_EmptyModule(t *testing.T) {
	module := &ir.Module{}
	compactTypes(module) // should not panic
	if len(module.Types) != 0 {
		t.Errorf("expected 0 types, got %d", len(module.Types))
	}
}

func TestCompactTypes_RemovesUnreferencedAnonymousTypes(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                            // 0: unreferenced anonymous scalar
			{Name: "vec4f", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}}, // 1: named
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},                                             // 2: referenced by constant
		},
		Constants: []ir.Constant{
			{Name: "C", Type: 2},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Arguments: []ir.FunctionArgument{{Name: "a", Type: 1}},
				},
			},
		},
	}

	compactTypes(module)

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
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},     // 0: concrete
			{Name: "abs_int", Inner: ir.ScalarType{Kind: ir.ScalarAbstractInt}},     // 1: abstract - must be removed even if named
			{Name: "abs_float", Inner: ir.ScalarType{Kind: ir.ScalarAbstractFloat}}, // 2: abstract
		},
		Constants: []ir.Constant{
			{Name: "C", Type: 0},
		},
	}

	compactTypes(module)

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
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                             // 0: unreferenced anonymous
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                          // 1: named, referenced by const C
			{Name: "arr", Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &size}, Stride: 16}}, // 2: referenced by global
		},
		Constants: []ir.Constant{
			{Name: "C", Type: 1},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 2}, // reference type 2 to keep it
		},
	}

	compactTypes(module)

	// Type 0 removed, type 1->0, type 2->1
	if len(module.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(module.Types))
	}
	// Array base should be remapped from 1 to 0
	arr, ok := module.Types[1].Inner.(ir.ArrayType)
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
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // unnamed, referenced
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // unnamed, unreferenced
		},
		Constants: []ir.Constant{
			{Name: "x", Type: 0}, // references type[0]
		},
	}

	compactTypes(module)

	// f32 kept (referenced), u32 removed (unnamed + unreferenced)
	if len(module.Types) != 1 {
		t.Errorf("expected 1 type (f32 kept, u32 removed), got %d", len(module.Types))
	}
}

func TestCompactTypes_NamedTypesKept(t *testing.T) {
	// ALL named types are kept even without direct references,
	// matching Rust naga's compact pass which preserves named types.
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "MyStruct", Inner: ir.StructType{
				Members: []ir.StructMember{{Name: "x", Type: 0, Offset: 0}},
				Span:    4,
			}},
			{Name: "Vec2Alias", Inner: ir.VectorType{Size: 2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
	}

	compactTypes(module)

	// Both kept: MyStruct (named struct) and Vec2Alias (named alias).
	// Rust naga keeps ALL named types regardless of reference count.
	if len(module.Types) != 2 {
		t.Errorf("expected 2 types (both named, both kept), got %d", len(module.Types))
	}
}

// --- compactConstants tests ---

// matrixInitGlobalExpressions builds the global expression arena Rust naga
// produces for mat2x2<f32>(vec2<f32>(1.0, 2.0), vec2<f32>(3.0, 4.0)):
//
//	[0] Lit  [1] Lit  [2] Compose(vec2)  [3] Lit  [4] Lit  [5] Compose(vec2)  [6] Compose(mat2x2)
//
// followed by an unreachable [7] Lit, [8] Compose(vec3) that no root refers to.
// Type handles: 0 = mat2x2<f32>, 1 = vec2<f32>, 2 = vec3<f32>.
func matrixInitGlobalExpressions() []ir.Expression {
	lit := func(v float32) ir.Expression { return ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(v)}} }
	return []ir.Expression{
		lit(1), lit(2),
		{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{0, 1}}},
		lit(3), lit(4),
		{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{3, 4}}},
		{Kind: ir.ExprCompose{Type: 0, Components: []ir.ExpressionHandle{2, 5}}},
		lit(5),
		{Kind: ir.ExprCompose{Type: 2, Components: []ir.ExpressionHandle{7, 7, 7}}},
	}
}

func matrixInitTypes() []ir.Type {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	return []ir.Type{
		{Inner: ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: f32}},
		{Inner: ir.VectorType{Size: ir.Vec2, Scalar: f32}},
		{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}},
	}
}

// TestCompactTypes_KeepsTypesReachableFromGlobalExpressions pins Rust naga's
// compact behavior (compact/expressions.rs: Compose/ZeroValue mark their type):
// a type referenced only by a module-scope initializer — here the vec2<f32>
// column type of a matrix initializer — must survive compaction, and the
// Compose handles must be remapped to it. Previously the type was removed and
// the columns were left with a dangling handle.
func TestCompactTypes_KeepsTypesReachableFromGlobalExpressions(t *testing.T) {
	init := ir.ExpressionHandle(6)
	tests := []struct {
		name   string
		module func() *ir.Module
	}{
		{"constant init", func() *ir.Module {
			return &ir.Module{
				Types:             matrixInitTypes(),
				GlobalExpressions: matrixInitGlobalExpressions(),
				Constants:         []ir.Constant{{Name: "M", Type: 0, Init: init}},
			}
		}},
		{"global variable init", func() *ir.Module {
			return &ir.Module{
				Types:             matrixInitTypes(),
				GlobalExpressions: matrixInitGlobalExpressions(),
				GlobalVariables:   []ir.GlobalVariable{{Name: "P", Space: ir.SpacePrivate, Type: 0, InitExpr: &init}},
			}
		}},
		{"override init", func() *ir.Module {
			return &ir.Module{
				Types:             matrixInitTypes(),
				GlobalExpressions: matrixInitGlobalExpressions(),
				Overrides:         []ir.Override{{Name: "O", Ty: 0, Init: &init}},
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := tt.module()
			compactTypes(module)

			// vec2 (reachable) is kept; vec3 (only used by unreachable [8]) is removed,
			// matching Rust, which traces only used global expressions.
			if len(module.Types) != 2 {
				t.Fatalf("expected 2 types (mat2x2, vec2), got %d: %+v", len(module.Types), module.Types)
			}
			if _, ok := module.Types[0].Inner.(ir.MatrixType); !ok {
				t.Errorf("type 0: expected MatrixType, got %T", module.Types[0].Inner)
			}
			if v, ok := module.Types[1].Inner.(ir.VectorType); !ok || v.Size != ir.Vec2 {
				t.Errorf("type 1: expected vec2, got %+v", module.Types[1].Inner)
			}
			for _, h := range []int{2, 5} {
				c := module.GlobalExpressions[h].Kind.(ir.ExprCompose)
				if c.Type != 1 {
					t.Errorf("GE[%d] column Compose type = %d, want 1 (vec2)", h, c.Type)
				}
			}
			if c := module.GlobalExpressions[6].Kind.(ir.ExprCompose); c.Type != 0 {
				t.Errorf("GE[6] matrix Compose type = %d, want 0", c.Type)
			}
		})
	}
}

// TestCompactTypes_KeepsZeroValueTypeFromGlobalExpressions covers the other
// type-carrying constant expression Rust traces: ZeroValue(ty).
// IR for: const C: vec4<f32> = vec4<f32>(vec3<f32>(), 1.0);
func TestCompactTypes_KeepsZeroValueTypeFromGlobalExpressions(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: f32}}, // constant's own type
			{Inner: ir.VectorType{Size: ir.Vec3, Scalar: f32}}, // referenced only by ZeroValue
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 1}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
			{Kind: ir.ExprCompose{Type: 0, Components: []ir.ExpressionHandle{0, 1}}},
		},
		Constants: []ir.Constant{{Name: "C", Type: 0, Init: 2}},
	}

	compactTypes(module)

	if len(module.Types) != 2 {
		t.Fatalf("expected vec3 used by ZeroValue to be kept (2 types), got %d", len(module.Types))
	}
	z := module.GlobalExpressions[0].Kind.(ir.ExprZeroValue)
	if int(z.Type) >= len(module.Types) {
		t.Fatalf("ZeroValue type handle %d dangling", z.Type)
	}
	if v, ok := module.Types[z.Type].Inner.(ir.VectorType); !ok || v.Size != ir.Vec3 {
		t.Errorf("ZeroValue type = %+v, want vec3", module.Types[z.Type].Inner)
	}
}

// TestCompactTypes_IgnoresOutOfRangeConstantInit guards modules whose constants
// carry no global expression arena (hand-built IR, older producers).
func TestCompactTypes_IgnoresOutOfRangeConstantInit(t *testing.T) {
	module := &ir.Module{
		Types:     matrixInitTypes(),
		Constants: []ir.Constant{{Name: "M", Type: 0, Init: 42}},
	}
	compactTypes(module) // must not panic
	if len(module.Types) != 1 {
		t.Fatalf("expected only the constant's own type to survive, got %d", len(module.Types))
	}
}

func TestCompactConstants_EmptyModule(t *testing.T) {
	module := &ir.Module{}
	compactConstants(module, KeepUnusedYes) // should not panic
}

func TestCompactConstants_RemovesAbstractConstants(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}},
		},
		Constants: []ir.Constant{
			{Name: "NAMED_CONCRETE", Type: 0, Init: 0},
			{Name: "NAMED_ABSTRACT", Type: 1, Init: 1}, // abstract by its type, as in Rust
			{Name: "", Type: 0, Init: 2},               // unnamed
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralAbstractFloat(2)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(3)}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprConstant{Constant: 0}},
					},
				},
			},
		},
	}

	compactConstants(module, KeepUnusedYes)

	// Only NAMED_CONCRETE should survive (named + not abstract + referenced)
	if len(module.Constants) != 1 {
		t.Fatalf("expected 1 constant, got %d", len(module.Constants))
	}
	if module.Constants[0].Name != "NAMED_CONCRETE" {
		t.Errorf("expected 'NAMED_CONCRETE', got '%s'", module.Constants[0].Name)
	}
	// The removed constants' initializers go with them.
	if len(module.GlobalExpressions) != 1 || module.Constants[0].Init != 0 {
		t.Fatalf("expected the one surviving initializer at 0, got %d expressions, init %d",
			len(module.GlobalExpressions), module.Constants[0].Init)
	}
}

func TestCompactConstants_KeepsReferencedByGlobalVarInit(t *testing.T) {
	ch := ir.ConstantHandle(1)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "", Type: 0}, // 0: unnamed, not directly referenced
			{Name: "", Type: 0}, // 1: unnamed, but referenced by global var init
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g", Type: 0, Init: &ch},
		},
	}

	compactConstants(module, KeepUnusedYes)

	if len(module.Constants) != 1 {
		t.Fatalf("expected 1 constant (ref'd by global init), got %d", len(module.Constants))
	}
}

func TestCompactConstants_TransitiveKeepComposite(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		},
		Constants: []ir.Constant{
			{Name: "", Type: 0, Init: 0},     // 0: component
			{Name: "COMP", Type: 1, Init: 2}, // 1: composite whose init refers to 0
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
			{Kind: ir.ExprConstant{Constant: 0}},
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{1, 1}}},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprConstant{Constant: 1}},
					},
				},
			},
		},
	}

	compactConstants(module, KeepUnusedYes)

	// Both should be kept: 1 is named+referenced, 0 is needed through 1's init
	if len(module.Constants) != 2 {
		t.Fatalf("expected 2 constants (transitive keep), got %d", len(module.Constants))
	}
	if len(module.GlobalExpressions) != 3 {
		t.Fatalf("expected 3 global expressions kept, got %d", len(module.GlobalExpressions))
	}
}

func TestCompactConstants_AllKept(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Constants: []ir.Constant{
			{Name: "A", Type: 0},
			{Name: "B", Type: 0},
		},
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Expressions: []ir.Expression{
						{Kind: ir.ExprConstant{Constant: 0}},
						{Kind: ir.ExprConstant{Constant: 1}},
					},
				},
			},
		},
	}

	compactConstants(module, KeepUnusedYes)

	if len(module.Constants) != 2 {
		t.Errorf("expected 2 constants (all kept), got %d", len(module.Constants))
	}
}

// Rust compact traces Module.global_expressions from the kept constants,
// overrides and global variable initializers; the initializer of a removed
// abstract constant goes with it and the surviving handles are renumbered.
func TestCompactConstants_RemovesGlobalExpressionsOfRemovedConstants(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralAbstractFloat(1.5)}}, // 0: A's init
			{Kind: ir.Literal{Value: ir.LiteralF32(2)}},             // 1: X's init
		},
		Constants: []ir.Constant{
			{Name: "A", Type: 1, Init: 0},
			{Name: "X", Type: 0, Init: 1},
		},
	}

	compactConstants(module, KeepUnusedYes)

	if len(module.Constants) != 1 || module.Constants[0].Name != "X" {
		t.Fatalf("constants = %+v, want only X", module.Constants)
	}
	if len(module.GlobalExpressions) != 1 {
		t.Fatalf("%d global expressions, want 1 (A's init removed)", len(module.GlobalExpressions))
	}
	if module.Constants[0].Init != 0 {
		t.Errorf("X.Init = %d, want 0 after renumbering", module.Constants[0].Init)
	}
	if lit, ok := module.GlobalExpressions[0].Kind.(ir.Literal); !ok || lit.Value != ir.LiteralF32(2) {
		t.Errorf("global expression 0 = %#v, want Literal(F32(2))", module.GlobalExpressions[0].Kind)
	}
}

func TestCompactConstants_RemapsGlobalExpressionHandles(t *testing.T) {
	ovInit := ir.ExpressionHandle(4)
	gvInit := ir.ExpressionHandle(5)
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Inner: ir.VectorType{Size: ir.Vec2, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
			{Inner: ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}},
		},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralAbstractInt(7)}},                      // 0: dead (abstract A)
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}},                              // 1
			{Kind: ir.Literal{Value: ir.LiteralF32(2)}},                              // 2
			{Kind: ir.ExprCompose{Type: 1, Components: []ir.ExpressionHandle{1, 2}}}, // 3: V's init
			{Kind: ir.Literal{Value: ir.LiteralF32(3)}},                              // 4: override init
			{Kind: ir.ExprSplat{Size: ir.Vec2, Value: 4}},                            // 5: global init
		},
		Constants: []ir.Constant{
			{Name: "A", Type: 2, Init: 0},
			{Name: "V", Type: 1, Init: 3},
		},
		Overrides:       []ir.Override{{Name: "O", Ty: 0, Init: &ovInit}},
		GlobalVariables: []ir.GlobalVariable{{Name: "g", Space: ir.SpacePrivate, Type: 1, InitExpr: &gvInit}},
	}

	compactConstants(module, KeepUnusedYes)

	if len(module.GlobalExpressions) != 5 {
		t.Fatalf("%d global expressions, want 5", len(module.GlobalExpressions))
	}
	if got := module.Constants[0].Init; got != 2 {
		t.Errorf("V.Init = %d, want 2", got)
	}
	comp, ok := module.GlobalExpressions[2].Kind.(ir.ExprCompose)
	if !ok || len(comp.Components) != 2 || comp.Components[0] != 0 || comp.Components[1] != 1 {
		t.Errorf("V's compose = %#v, want components [0 1]", module.GlobalExpressions[2].Kind)
	}
	if got := *module.Overrides[0].Init; got != 3 {
		t.Errorf("O.Init = %d, want 3", got)
	}
	if got := *module.GlobalVariables[0].InitExpr; got != 4 {
		t.Errorf("g.InitExpr = %d, want 4", got)
	}
	if sp, ok := module.GlobalExpressions[4].Kind.(ir.ExprSplat); !ok || sp.Value != 3 {
		t.Errorf("g's splat = %#v, want value 3", module.GlobalExpressions[4].Kind)
	}
}

// A constant referenced from a live global expression is kept, with its
// initializer, even when it is unnamed.
func TestCompactConstants_KeepsConstantReferencedFromGlobalExpression(t *testing.T) {
	gvInit := ir.ExpressionHandle(1)
	module := &ir.Module{
		Types: []ir.Type{{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}}},
		GlobalExpressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralU32(9)}},  // 0: unnamed constant's init
			{Kind: ir.ExprConstant{Constant: 0}},         // 1: global init refers to it
			{Kind: ir.Literal{Value: ir.LiteralU32(10)}}, // 2: dead
		},
		Constants:       []ir.Constant{{Name: "", Type: 0, Init: 0}},
		GlobalVariables: []ir.GlobalVariable{{Name: "g", Space: ir.SpacePrivate, Type: 0, InitExpr: &gvInit}},
	}

	compactConstants(module, KeepUnusedYes)

	if len(module.Constants) != 1 {
		t.Fatalf("%d constants, want the referenced unnamed one", len(module.Constants))
	}
	if len(module.GlobalExpressions) != 2 {
		t.Errorf("%d global expressions, want 2", len(module.GlobalExpressions))
	}
}

// --- compactExpressions tests ---

func TestCompactExpressions_RemovesUnusedExpressions(t *testing.T) {
	retVal := ir.ExpressionHandle(2)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 0: unused
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}}, // 1: unused
						{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}}, // 2: used by return
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Fatalf("expected 1 expression after compact, got %d", len(fn.Expressions))
	}
	// The return value should be remapped to 0
	ret := fn.Body[0].Kind.(ir.StmtReturn)
	if ret.Value == nil || *ret.Value != 0 {
		t.Errorf("expected return value remapped to 0")
	}
}

func TestCompactExpressions_KeepsNamedExpressions(t *testing.T) {
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 0: named
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}}, // 1: not named, not used
					},
					NamedExpressions: map[ir.ExpressionHandle]string{
						0: "x",
					},
					Body: []ir.Statement{},
				},
			},
		},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Fatalf("expected 1 expression (named kept), got %d", len(fn.Expressions))
	}
	if _, ok := fn.NamedExpressions[0]; !ok {
		t.Error("expected named expression at handle 0")
	}
}

func TestCompactExpressions_TransitiveDependencies(t *testing.T) {
	retVal := ir.ExpressionHandle(2)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},              // 0: base for binary
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},              // 1: base for binary
						{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}}, // 2: used by return
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// All 3 should be kept (0,1 transitively needed by 2)
	if len(fn.Expressions) != 3 {
		t.Fatalf("expected 3 expressions (transitive), got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_AllUsed(t *testing.T) {
	retVal := ir.ExpressionHandle(0)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
					},
					Body: []ir.Statement{
						{Kind: ir.StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Errorf("expected 1 expression (all used), got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_EmptyFunction(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "empty"},
		},
	}

	compactExpressions(module) // should not panic
}

func TestCompactExpressions_RemapsEmitRanges(t *testing.T) {
	retVal := ir.ExpressionHandle(2)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},              // 0: unused
						{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},              // 1: used
						{Kind: ir.ExprBinary{Op: ir.BinaryAdd, Left: 1, Right: 1}}, // 2: used by return
					},
					Body: []ir.Statement{
						{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
						{Kind: ir.StmtReturn{Value: &retVal}},
					},
				},
			},
		},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// Expression 0 removed, 1->0, 2->1
	if len(fn.Expressions) != 2 {
		t.Fatalf("expected 2 expressions, got %d", len(fn.Expressions))
	}
	// Check emit range was adjusted
	emit := fn.Body[0].Kind.(ir.StmtEmit)
	if emit.Range.Start != 0 || emit.Range.End != 2 {
		t.Errorf("expected emit range [0,2), got [%d,%d)", emit.Range.Start, emit.Range.End)
	}
}

func TestCompactExpressions_LocalVarInitKept(t *testing.T) {
	init := ir.ExpressionHandle(0)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{
			{
				Name:  "main",
				Stage: ir.StageVertex,
				Function: ir.Function{
					Name: "main",
					Expressions: []ir.Expression{
						{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}}, // 0: used by local var init
						{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 1: unused
					},
					LocalVars: []ir.LocalVariable{
						{Name: "x", Type: 0, Init: &init},
					},
					Body: []ir.Statement{},
				},
			},
		},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 1 {
		t.Fatalf("expected 1 expression (local var init), got %d", len(fn.Expressions))
	}
}

// --- ReorderTypes tests ---

func TestReorderTypes_EmptyModule(t *testing.T) {
	module := &ir.Module{}
	ReorderTypes(module, nil) // should not panic
}

func TestReorderTypes_AlreadyOrdered(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
	}

	ReorderTypes(module, []ir.TypeHandle{0, 1})

	if module.Types[0].Name != "f32" || module.Types[1].Name != "u32" {
		t.Error("types should remain in same order")
	}
}

func TestReorderTypes_ReordersBasedOnUseOrder(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // 0
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 1
		},
		Constants: []ir.Constant{
			{Name: "C", Type: 0}, // references u32
		},
	}

	ReorderTypes(module, []ir.TypeHandle{1, 0}) // f32 first, u32 second

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
	rayH := ir.TypeHandle(1)
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
			{Name: "RayIntersection", Inner: ir.StructType{Members: nil, Span: 0}},
		},
		SpecialTypes: ir.SpecialTypes{
			RayIntersection: &rayH,
		},
	}

	ReorderTypes(module, []ir.TypeHandle{1, 0})

	// RayIntersection was at 1, now at 0
	if module.SpecialTypes.RayIntersection == nil || *module.SpecialTypes.RayIntersection != 0 {
		t.Error("expected RayIntersection remapped to 0")
	}
}

// --- markTypeInnerRefs tests ---

func TestMarkTypeInnerRefs(t *testing.T) {
	referenced := make([]bool, 5)

	tests := []struct {
		name  string
		inner ir.TypeInner
		want  []int // indices that should be marked
	}{
		{"array marks base", ir.ArrayType{Base: 2, Stride: 4}, []int{2}},
		{"struct marks members", ir.StructType{Members: []ir.StructMember{
			{Type: 0}, {Type: 3},
		}}, []int{0, 3}},
		{"pointer marks base", ir.PointerType{Base: 1, Space: ir.SpaceFunction}, []int{1}},
		{"binding array marks base", ir.BindingArrayType{Base: 4}, []int{4}},
		{"scalar marks nothing", ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, nil},
		{"vector marks nothing", ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, nil},
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
		kind ir.ExpressionKind
		want []int
	}{
		{"ZeroValue marks type", ir.ExprZeroValue{Type: 2}, []int{2}},
		{"Compose marks type", ir.ExprCompose{Type: 3}, []int{3}},
		{"SubgroupOperationResult marks type", ir.ExprSubgroupOperationResult{Type: 1}, []int{1}},
		{"AtomicResult marks type", ir.ExprAtomicResult{Ty: 4}, []int{4}},
		{"Literal marks nothing", ir.Literal{Value: ir.LiteralF32(1.0)}, nil},
		{"Binary marks nothing", ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}, nil},
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
	remap := []ir.TypeHandle{2, 0, 1, 3, 4}

	tests := []struct {
		name     string
		input    ir.TypeInner
		wantBase ir.TypeHandle // for types with base handles
	}{
		{"array remaps base", ir.ArrayType{Base: 0, Stride: 4}, 2},
		{"pointer remaps base", ir.PointerType{Base: 1, Space: ir.SpaceFunction}, 0},
		{"binding array remaps base", ir.BindingArrayType{Base: 2}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapTypeInner(tt.input, remap)
			switch r := result.(type) {
			case ir.ArrayType:
				if r.Base != tt.wantBase {
					t.Errorf("base = %d, want %d", r.Base, tt.wantBase)
				}
			case ir.PointerType:
				if r.Base != tt.wantBase {
					t.Errorf("base = %d, want %d", r.Base, tt.wantBase)
				}
			case ir.BindingArrayType:
				if r.Base != tt.wantBase {
					t.Errorf("base = %d, want %d", r.Base, tt.wantBase)
				}
			}
		})
	}

	// Struct member remapping
	t.Run("struct remaps member types", func(t *testing.T) {
		input := ir.StructType{
			Members: []ir.StructMember{
				{Name: "a", Type: 0},
				{Name: "b", Type: 1},
			},
			Span: 8,
		}
		result := remapTypeInner(input, remap).(ir.StructType)
		if result.Members[0].Type != 2 {
			t.Errorf("member 0 type = %d, want 2", result.Members[0].Type)
		}
		if result.Members[1].Type != 0 {
			t.Errorf("member 1 type = %d, want 0", result.Members[1].Type)
		}
	})

	// Scalar passes through unchanged
	t.Run("scalar unchanged", func(t *testing.T) {
		input := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		result := remapTypeInner(input, remap)
		if s, ok := result.(ir.ScalarType); !ok || s != input {
			t.Errorf("scalar should be unchanged")
		}
	})
}

// --- remapExprTypeHandles tests ---

func TestRemapExprTypeHandles(t *testing.T) {
	remap := []ir.TypeHandle{1, 0, 2}

	tests := []struct {
		name string
		kind ir.ExpressionKind
		want ir.TypeHandle
	}{
		{"ZeroValue", ir.ExprZeroValue{Type: 0}, 1},
		{"Compose", ir.ExprCompose{Type: 1}, 0},
		{"SubgroupOperationResult", ir.ExprSubgroupOperationResult{Type: 2}, 2},
		{"AtomicResult", ir.ExprAtomicResult{Ty: 0}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := remapExprTypeHandles(tt.kind, remap)
			switch r := result.(type) {
			case ir.ExprZeroValue:
				if r.Type != tt.want {
					t.Errorf("type = %d, want %d", r.Type, tt.want)
				}
			case ir.ExprCompose:
				if r.Type != tt.want {
					t.Errorf("type = %d, want %d", r.Type, tt.want)
				}
			case ir.ExprSubgroupOperationResult:
				if r.Type != tt.want {
					t.Errorf("type = %d, want %d", r.Type, tt.want)
				}
			case ir.ExprAtomicResult:
				if r.Ty != tt.want {
					t.Errorf("type = %d, want %d", r.Ty, tt.want)
				}
			}
		})
	}

	// Non-type expressions pass through
	t.Run("literal unchanged", func(t *testing.T) {
		result := remapExprTypeHandles(ir.Literal{Value: ir.LiteralF32(1.0)}, remap)
		if _, ok := result.(ir.Literal); !ok {
			t.Error("literal should pass through unchanged")
		}
	})
}

// --- RemapExpressionHandles tests ---

func TestRemapExpressionHandles(t *testing.T) {
	remap := []ir.ExpressionHandle{2, 0, 1, 3}

	t.Run("Binary", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: 1}, remap)
		bin := result.(ir.ExprBinary)
		if bin.Left != 2 || bin.Right != 0 {
			t.Errorf("got left=%d right=%d, want left=2 right=0", bin.Left, bin.Right)
		}
	})

	t.Run("Unary", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprUnary{Op: ir.UnaryNegate, Expr: 1}, remap)
		un := result.(ir.ExprUnary)
		if un.Expr != 0 {
			t.Errorf("got expr=%d, want 0", un.Expr)
		}
	})

	t.Run("Access", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprAccess{Base: 0, Index: 1}, remap)
		acc := result.(ir.ExprAccess)
		if acc.Base != 2 || acc.Index != 0 {
			t.Errorf("got base=%d index=%d, want base=2 index=0", acc.Base, acc.Index)
		}
	})

	t.Run("AccessIndex", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprAccessIndex{Base: 1, Index: 5}, remap)
		ai := result.(ir.ExprAccessIndex)
		if ai.Base != 0 {
			t.Errorf("got base=%d, want 0", ai.Base)
		}
		if ai.Index != 5 { // compile-time index unchanged
			t.Errorf("got index=%d, want 5", ai.Index)
		}
	})

	t.Run("Compose", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprCompose{Type: 0, Components: []ir.ExpressionHandle{0, 1, 2}}, remap)
		comp := result.(ir.ExprCompose)
		if comp.Components[0] != 2 || comp.Components[1] != 0 || comp.Components[2] != 1 {
			t.Errorf("got components %v, want [2,0,1]", comp.Components)
		}
	})

	t.Run("Load", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprLoad{Pointer: 0}, remap)
		ld := result.(ir.ExprLoad)
		if ld.Pointer != 2 {
			t.Errorf("got pointer=%d, want 2", ld.Pointer)
		}
	})

	t.Run("Select", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2}, remap)
		sel := result.(ir.ExprSelect)
		if sel.Condition != 2 || sel.Accept != 0 || sel.Reject != 1 {
			t.Errorf("got cond=%d accept=%d reject=%d", sel.Condition, sel.Accept, sel.Reject)
		}
	})

	t.Run("Math", func(t *testing.T) {
		arg1 := ir.ExpressionHandle(1)
		result := RemapExpressionHandles(ir.ExprMath{Fun: ir.MathClamp, Arg: 0, Arg1: &arg1}, remap)
		m := result.(ir.ExprMath)
		if m.Arg != 2 {
			t.Errorf("got arg=%d, want 2", m.Arg)
		}
		if m.Arg1 == nil || *m.Arg1 != 0 {
			t.Errorf("got arg1=%v, want 0", m.Arg1)
		}
	})

	t.Run("Literal passthrough", func(t *testing.T) {
		result := RemapExpressionHandles(ir.Literal{Value: ir.LiteralF32(1.0)}, remap)
		if _, ok := result.(ir.Literal); !ok {
			t.Error("literal should pass through")
		}
	})

	t.Run("Splat", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprSplat{Size: ir.Vec4, Value: 1}, remap)
		sp := result.(ir.ExprSplat)
		if sp.Value != 0 {
			t.Errorf("got value=%d, want 0", sp.Value)
		}
	})

	t.Run("Swizzle", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprSwizzle{Size: ir.Vec2, Vector: 0}, remap)
		sw := result.(ir.ExprSwizzle)
		if sw.Vector != 2 {
			t.Errorf("got vector=%d, want 2", sw.Vector)
		}
	})

	t.Run("As", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprAs{Expr: 1, Kind: ir.ScalarFloat}, remap)
		as := result.(ir.ExprAs)
		if as.Expr != 0 {
			t.Errorf("got expr=%d, want 0", as.Expr)
		}
	})

	t.Run("ArrayLength", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprArrayLength{Array: 0}, remap)
		al := result.(ir.ExprArrayLength)
		if al.Array != 2 {
			t.Errorf("got array=%d, want 2", al.Array)
		}
	})

	t.Run("Derivative", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprDerivative{Axis: ir.DerivativeX, Control: ir.DerivativeFine, Expr: 1}, remap)
		d := result.(ir.ExprDerivative)
		if d.Expr != 0 {
			t.Errorf("got expr=%d, want 0", d.Expr)
		}
	})

	t.Run("Relational", func(t *testing.T) {
		result := RemapExpressionHandles(ir.ExprRelational{Fun: ir.RelationalIsNan, Argument: 2}, remap)
		r := result.(ir.ExprRelational)
		if r.Argument != 1 {
			t.Errorf("got argument=%d, want 1", r.Argument)
		}
	})
}

// --- walkSampleLevel tests ---

func TestMapSampleLevel(t *testing.T) {
	rm := func(h ir.ExpressionHandle) ir.ExpressionHandle { return h + 10 }

	t.Run("nil", func(t *testing.T) {
		result := walkSampleLevel(nil, mapHandles{f: rm})
		if result != nil {
			t.Error("expected nil")
		}
	})

	t.Run("auto unchanged", func(t *testing.T) {
		result := walkSampleLevel(ir.SampleLevelAuto{}, mapHandles{f: rm})
		if _, ok := result.(ir.SampleLevelAuto); !ok {
			t.Error("expected SampleLevelAuto")
		}
	})

	t.Run("zero unchanged", func(t *testing.T) {
		result := walkSampleLevel(ir.SampleLevelZero{}, mapHandles{f: rm})
		if _, ok := result.(ir.SampleLevelZero); !ok {
			t.Error("expected SampleLevelZero")
		}
	})

	t.Run("exact remaps level", func(t *testing.T) {
		result := walkSampleLevel(ir.SampleLevelExact{Level: 5}, mapHandles{f: rm})
		exact := result.(ir.SampleLevelExact)
		if exact.Level != 15 {
			t.Errorf("got level=%d, want 15", exact.Level)
		}
	})

	t.Run("bias remaps bias", func(t *testing.T) {
		result := walkSampleLevel(ir.SampleLevelBias{Bias: 3}, mapHandles{f: rm})
		bias := result.(ir.SampleLevelBias)
		if bias.Bias != 13 {
			t.Errorf("got bias=%d, want 13", bias.Bias)
		}
	})

	t.Run("gradient remaps x and y", func(t *testing.T) {
		result := walkSampleLevel(ir.SampleLevelGradient{X: 1, Y: 2}, mapHandles{f: rm})
		grad := result.(ir.SampleLevelGradient)
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
		kind ir.ExpressionKind
		want []int
	}{
		{"Compose", ir.ExprCompose{Components: []ir.ExpressionHandle{1, 3, 5}}, []int{1, 3, 5}},
		{"Splat", ir.ExprSplat{Value: 2}, []int{2}},
		{"Swizzle", ir.ExprSwizzle{Vector: 4}, []int{4}},
		{"Access", ir.ExprAccess{Base: 0, Index: 1}, []int{0, 1}},
		{"AccessIndex", ir.ExprAccessIndex{Base: 3}, []int{3}},
		{"Load", ir.ExprLoad{Pointer: 6}, []int{6}},
		{"Unary", ir.ExprUnary{Expr: 7}, []int{7}},
		{"Binary", ir.ExprBinary{Left: 2, Right: 8}, []int{2, 8}},
		{"Select", ir.ExprSelect{Condition: 1, Accept: 3, Reject: 5}, []int{1, 3, 5}},
		{"Relational", ir.ExprRelational{Argument: 4}, []int{4}},
		{"As", ir.ExprAs{Expr: 9}, []int{9}},
		{"ArrayLength", ir.ExprArrayLength{Array: 0}, []int{0}},
		{"Derivative", ir.ExprDerivative{Expr: 6}, []int{6}},
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
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "helper", Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: 1}},
			}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g0"},
			{Name: "g1"},
		},
	}

	usedGlobals := make([]bool, 2)
	usedFunctions := make([]bool, 1)

	stmts := []ir.Statement{
		{Kind: ir.StmtCall{Function: 0}},
	}

	traceStatementsForRefs(stmts, usedFunctions, module, func(f *ir.Function) {
		for _, expr := range f.Expressions {
			if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
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
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "f0"},
			{Name: "f1"},
		},
	}

	usedFunctions := make([]bool, 2)

	stmts := []ir.Statement{
		{Kind: ir.StmtIf{
			Condition: 0,
			Accept: []ir.Statement{
				{Kind: ir.StmtCall{Function: 0}},
			},
			Reject: []ir.Statement{
				{Kind: ir.StmtCall{Function: 1}},
			},
		}},
	}

	traceStatementsForRefs(stmts, usedFunctions, module, func(_ *ir.Function) {})

	if !usedFunctions[0] {
		t.Error("expected function 0 marked (in if-accept)")
	}
	if !usedFunctions[1] {
		t.Error("expected function 1 marked (in if-reject)")
	}
}

// --- walkGatherMode tests ---

func TestMapGatherMode(t *testing.T) {
	rm := func(h ir.ExpressionHandle) ir.ExpressionHandle { return h + 100 }

	t.Run("Broadcast", func(t *testing.T) {
		result := walkGatherMode(ir.GatherBroadcast{Index: 5}, mapHandles{f: rm})
		if g, ok := result.(ir.GatherBroadcast); !ok || g.Index != 105 {
			t.Errorf("expected index 105, got %v", result)
		}
	})

	t.Run("Shuffle", func(t *testing.T) {
		result := walkGatherMode(ir.GatherShuffle{Index: 3}, mapHandles{f: rm})
		if g, ok := result.(ir.GatherShuffle); !ok || g.Index != 103 {
			t.Errorf("expected index 103, got %v", result)
		}
	})

	t.Run("ShuffleDown", func(t *testing.T) {
		result := walkGatherMode(ir.GatherShuffleDown{Delta: 1}, mapHandles{f: rm})
		if g, ok := result.(ir.GatherShuffleDown); !ok || g.Delta != 101 {
			t.Errorf("expected delta 101, got %v", result)
		}
	})

	t.Run("ShuffleUp", func(t *testing.T) {
		result := walkGatherMode(ir.GatherShuffleUp{Delta: 2}, mapHandles{f: rm})
		if g, ok := result.(ir.GatherShuffleUp); !ok || g.Delta != 102 {
			t.Errorf("expected delta 102, got %v", result)
		}
	})

	t.Run("ShuffleXor", func(t *testing.T) {
		result := walkGatherMode(ir.GatherShuffleXor{Mask: 7}, mapHandles{f: rm})
		if g, ok := result.(ir.GatherShuffleXor); !ok || g.Mask != 107 {
			t.Errorf("expected mask 107, got %v", result)
		}
	})

	t.Run("QuadBroadcast", func(t *testing.T) {
		result := walkGatherMode(ir.GatherQuadBroadcast{Index: 0}, mapHandles{f: rm})
		if g, ok := result.(ir.GatherQuadBroadcast); !ok || g.Index != 100 {
			t.Errorf("expected index 100, got %v", result)
		}
	})

	t.Run("BroadcastFirst passthrough", func(t *testing.T) {
		result := walkGatherMode(ir.GatherBroadcastFirst{}, mapHandles{f: rm})
		if _, ok := result.(ir.GatherBroadcastFirst); !ok {
			t.Error("expected GatherBroadcastFirst passthrough")
		}
	})

	t.Run("QuadSwap passthrough", func(t *testing.T) {
		result := walkGatherMode(ir.GatherQuadSwap{Direction: ir.QuadDirectionX}, mapHandles{f: rm})
		if _, ok := result.(ir.GatherQuadSwap); !ok {
			t.Error("expected GatherQuadSwap passthrough")
		}
	})
}

// --- walkRayQueryFunction tests ---

func TestMapRayQueryFunction(t *testing.T) {
	rm := func(h ir.ExpressionHandle) ir.ExpressionHandle { return h + 10 }

	t.Run("Initialize", func(t *testing.T) {
		result := walkRayQueryFunction(ir.RayQueryInitialize{AccelerationStructure: 1, Descriptor: 2}, mapHandles{f: rm})
		init := result.(ir.RayQueryInitialize)
		if init.AccelerationStructure != 11 || init.Descriptor != 12 {
			t.Errorf("got as=%d desc=%d, want 11, 12", init.AccelerationStructure, init.Descriptor)
		}
	})

	t.Run("Proceed", func(t *testing.T) {
		result := walkRayQueryFunction(ir.RayQueryProceed{Result: 3}, mapHandles{f: rm})
		proc := result.(ir.RayQueryProceed)
		if proc.Result != 13 {
			t.Errorf("got result=%d, want 13", proc.Result)
		}
	})

	t.Run("GenerateIntersection", func(t *testing.T) {
		result := walkRayQueryFunction(ir.RayQueryGenerateIntersection{HitT: 5}, mapHandles{f: rm})
		gen := result.(ir.RayQueryGenerateIntersection)
		if gen.HitT != 15 {
			t.Errorf("got hitT=%d, want 15", gen.HitT)
		}
	})

	t.Run("Terminate passthrough", func(t *testing.T) {
		result := walkRayQueryFunction(ir.RayQueryTerminate{}, mapHandles{f: rm})
		if _, ok := result.(ir.RayQueryTerminate); !ok {
			t.Error("expected RayQueryTerminate passthrough")
		}
	})
}

// --- walkAtomicFunction tests ---

func TestMapAtomicFunction(t *testing.T) {
	rm := func(h ir.ExpressionHandle) ir.ExpressionHandle { return h + 10 }

	t.Run("Exchange with compare", func(t *testing.T) {
		cmp := ir.ExpressionHandle(5)
		result := walkAtomicFunction(ir.AtomicExchange{Compare: &cmp}, mapHandles{f: rm})
		ex := result.(ir.AtomicExchange)
		if ex.Compare == nil || *ex.Compare != 15 {
			t.Errorf("expected compare=15, got %v", ex.Compare)
		}
		if cmp != 5 {
			t.Errorf("the input's compare changed to %d", cmp)
		}
	})

	t.Run("Exchange without compare", func(t *testing.T) {
		result := walkAtomicFunction(ir.AtomicExchange{Compare: nil}, mapHandles{f: rm})
		ex := result.(ir.AtomicExchange)
		if ex.Compare != nil {
			t.Error("expected nil compare")
		}
	})

	t.Run("Add passthrough", func(t *testing.T) {
		result := walkAtomicFunction(ir.AtomicAdd{}, mapHandles{f: rm})
		if _, ok := result.(ir.AtomicAdd); !ok {
			t.Error("expected AtomicAdd passthrough")
		}
	})
}

// --- visitExprTypeHandles tests ---

func TestVisitExprTypeHandles(t *testing.T) {
	var visited []ir.TypeHandle
	visit := func(h ir.TypeHandle) { visited = append(visited, h) }

	tests := []struct {
		name string
		kind ir.ExpressionKind
		want []ir.TypeHandle
	}{
		{"ZeroValue", ir.ExprZeroValue{Type: 3}, []ir.TypeHandle{3}},
		{"Compose", ir.ExprCompose{Type: 5}, []ir.TypeHandle{5}},
		{"SubgroupOperationResult", ir.ExprSubgroupOperationResult{Type: 7}, []ir.TypeHandle{7}},
		{"AtomicResult", ir.ExprAtomicResult{Ty: 1}, []ir.TypeHandle{1}},
		{"Literal no visits", ir.Literal{Value: ir.LiteralF32(0)}, nil},
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

// --- markStmtExprRefsForCompact tests ---

func TestMarkStmtExprRefsForCompact(t *testing.T) {
	refs := make([]bool, 20)
	reset := func() {
		for i := range refs {
			refs[i] = false
		}
	}

	t.Run("Store marks pointer and value", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 1, Value: 3}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] {
			t.Error("expected 1 and 3 marked")
		}
	})

	t.Run("Return marks value", func(t *testing.T) {
		reset()
		v := ir.ExpressionHandle(5)
		stmts := []ir.Statement{
			{Kind: ir.StmtReturn{Value: &v}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[5] {
			t.Error("expected 5 marked")
		}
	})

	t.Run("Return nil value marks nothing", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtReturn{Value: nil}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		for i, r := range refs {
			if r {
				t.Errorf("unexpected mark at %d", i)
			}
		}
	})

	t.Run("If marks condition and recurses", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtIf{
				Condition: 2,
				Accept:    ir.Block{{Kind: ir.StmtStore{Pointer: 4, Value: 6}}},
				Reject:    ir.Block{{Kind: ir.StmtStore{Pointer: 8, Value: 10}}},
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[2] || !refs[4] || !refs[6] || !refs[8] || !refs[10] {
			t.Error("expected 2,4,6,8,10 marked")
		}
	})

	t.Run("Switch marks selector and case bodies", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtSwitch{
				Selector: 1,
				Cases: []ir.SwitchCase{
					{Body: ir.Block{{Kind: ir.StmtStore{Pointer: 3, Value: 5}}}},
				},
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("Loop marks body, continuing, and break_if", func(t *testing.T) {
		reset()
		breakIf := ir.ExpressionHandle(7)
		stmts := []ir.Statement{
			{Kind: ir.StmtLoop{
				Body:       ir.Block{{Kind: ir.StmtStore{Pointer: 2, Value: 4}}},
				Continuing: ir.Block{{Kind: ir.StmtStore{Pointer: 6, Value: 8}}},
				BreakIf:    &breakIf,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[2] || !refs[4] || !refs[6] || !refs[7] || !refs[8] {
			t.Error("expected 2,4,6,7,8 marked")
		}
	})

	t.Run("Block recurses", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtBlock{Block: ir.Block{{Kind: ir.StmtStore{Pointer: 1, Value: 2}}}}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[2] {
			t.Error("expected 1,2 marked")
		}
	})

	t.Run("Call marks arguments and result", func(t *testing.T) {
		reset()
		result := ir.ExpressionHandle(9)
		stmts := []ir.Statement{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{3, 5},
				Result:    &result,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[3] || !refs[5] || !refs[9] {
			t.Error("expected 3,5,9 marked")
		}
	})

	t.Run("ImageStore marks all fields", func(t *testing.T) {
		reset()
		arrIdx := ir.ExpressionHandle(11)
		stmts := []ir.Statement{
			{Kind: ir.StmtImageStore{
				Image:      1,
				Coordinate: 3,
				ArrayIndex: &arrIdx,
				Value:      5,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] || !refs[11] {
			t.Error("expected 1,3,5,11 marked")
		}
	})

	// Rust's trace_block: an emitted expression is not live because it is
	// emitted.
	t.Run("Emit marks nothing", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: 7}}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		for h := range refs {
			if refs[h] {
				t.Errorf("expression %d marked", h)
			}
		}
	})

	t.Run("Atomic marks pointer, value, result", func(t *testing.T) {
		reset()
		result := ir.ExpressionHandle(12)
		stmts := []ir.Statement{
			{Kind: ir.StmtAtomic{
				Pointer: 1,
				Fun:     ir.AtomicAdd{},
				Value:   3,
				Result:  &result,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] || !refs[12] {
			t.Error("expected 1,3,12 marked")
		}
	})

	t.Run("WorkGroupUniformLoad marks pointer and result", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtWorkGroupUniformLoad{Pointer: 2, Result: 4}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[2] || !refs[4] {
			t.Error("expected 2,4 marked")
		}
	})

	t.Run("RayQuery marks query and function refs", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtRayQuery{
				Query: 1,
				Fun:   ir.RayQueryInitialize{AccelerationStructure: 3, Descriptor: 5},
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("SubgroupBallot marks predicate and result", func(t *testing.T) {
		reset()
		pred := ir.ExpressionHandle(2)
		stmts := []ir.Statement{
			{Kind: ir.StmtSubgroupBallot{Result: 4, Predicate: &pred}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[2] || !refs[4] {
			t.Error("expected 2,4 marked")
		}
	})

	t.Run("SubgroupGather marks mode, argument, result", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtSubgroupGather{
				Mode:     ir.GatherBroadcast{Index: 1},
				Argument: 3,
				Result:   5,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("ImageAtomic marks all fields", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtImageAtomic{
				Image:      1,
				Coordinate: 3,
				Fun:        ir.AtomicAdd{},
				Value:      5,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[1] || !refs[3] || !refs[5] {
			t.Error("expected 1,3,5 marked")
		}
	})

	t.Run("SubgroupCollectiveOperation marks argument and result", func(t *testing.T) {
		reset()
		stmts := []ir.Statement{
			{Kind: ir.StmtSubgroupCollectiveOperation{
				Argument: 2,
				Result:   4,
			}},
		}
		markStmtExprRefsForCompact(stmts, refs)
		if !refs[2] || !refs[4] {
			t.Error("expected 2,4 marked")
		}
	})
}

// --- RemapStatementExpressionHandles tests ---

func TestRemapStatementExpressionHandles(t *testing.T) {
	remap := []ir.ExpressionHandle{10, 20, 30, 40, 50}

	t.Run("Store", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtStore)
		if s.Pointer != 10 || s.Value != 20 {
			t.Errorf("got pointer=%d value=%d, want 10, 20", s.Pointer, s.Value)
		}
	})

	t.Run("Return with value", func(t *testing.T) {
		v := ir.ExpressionHandle(2)
		stmts := []ir.Statement{
			{Kind: ir.StmtReturn{Value: &v}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtReturn)
		if s.Value == nil || *s.Value != 30 {
			t.Errorf("got %v, want 30", s.Value)
		}
	})

	t.Run("If remaps condition", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtIf{
				Condition: 0,
				Accept:    ir.Block{{Kind: ir.StmtStore{Pointer: 1, Value: 2}}},
				Reject:    ir.Block{},
			}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtIf)
		if s.Condition != 10 {
			t.Errorf("condition = %d, want 10", s.Condition)
		}
		store := s.Accept[0].Kind.(ir.StmtStore)
		if store.Pointer != 20 {
			t.Errorf("inner store pointer = %d, want 20", store.Pointer)
		}
	})

	t.Run("Switch remaps selector", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtSwitch{
				Selector: 1,
				Cases: []ir.SwitchCase{
					{Body: ir.Block{{Kind: ir.StmtStore{Pointer: 2, Value: 3}}}},
				},
			}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtSwitch)
		if s.Selector != 20 {
			t.Errorf("selector = %d, want 20", s.Selector)
		}
	})

	t.Run("Loop remaps breakIf", func(t *testing.T) {
		bi := ir.ExpressionHandle(3)
		stmts := []ir.Statement{
			{Kind: ir.StmtLoop{
				Body:       ir.Block{},
				Continuing: ir.Block{},
				BreakIf:    &bi,
			}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtLoop)
		if s.BreakIf == nil || *s.BreakIf != 40 {
			t.Errorf("breakIf = %v, want 40", s.BreakIf)
		}
	})

	t.Run("Call remaps arguments and result", func(t *testing.T) {
		result := ir.ExpressionHandle(4)
		stmts := []ir.Statement{
			{Kind: ir.StmtCall{
				Function:  0,
				Arguments: []ir.ExpressionHandle{0, 1},
				Result:    &result,
			}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtCall)
		if s.Arguments[0] != 10 || s.Arguments[1] != 20 {
			t.Errorf("args = %v, want [10, 20]", s.Arguments)
		}
		if s.Result == nil || *s.Result != 50 {
			t.Errorf("result = %v, want 50", s.Result)
		}
	})

	t.Run("Emit remaps range", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtEmit{Range: ir.Range{Start: 1, End: 3}}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtEmit)
		// Rust adjust_stmt: by the first and last expression,
		// remap[1]=20, remap[2]=30 → [20, 31).
		if s.Range.Start != 20 || s.Range.End != 31 {
			t.Errorf("range = [%d,%d), want [20,31)", s.Range.Start, s.Range.End)
		}
	})

	t.Run("Emit ending at the arena end", func(t *testing.T) {
		// End is exclusive and equals len(remap): there is no remap entry
		// for it, the last expression decides.
		stmts := []ir.Statement{{Kind: ir.StmtEmit{Range: ir.Range{Start: 3, End: ir.ExpressionHandle(len(remap))}}}}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtEmit)
		want := ir.Range{Start: remap[3], End: remap[len(remap)-1] + 1}
		if s.Range != want {
			t.Errorf("range = %+v, want %+v", s.Range, want)
		}
	})

	t.Run("ImageStore remaps", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtImageStore{Image: 0, Coordinate: 1, Value: 2}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtImageStore)
		if s.Image != 10 || s.Coordinate != 20 || s.Value != 30 {
			t.Errorf("got image=%d coord=%d val=%d", s.Image, s.Coordinate, s.Value)
		}
	})

	t.Run("Block recurses", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtBlock{Block: ir.Block{{Kind: ir.StmtStore{Pointer: 0, Value: 1}}}}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		blk := stmts[0].Kind.(ir.StmtBlock)
		s := blk.Block[0].Kind.(ir.StmtStore)
		if s.Pointer != 10 || s.Value != 20 {
			t.Errorf("got pointer=%d value=%d", s.Pointer, s.Value)
		}
	})

	t.Run("Atomic remaps", func(t *testing.T) {
		result := ir.ExpressionHandle(4)
		stmts := []ir.Statement{
			{Kind: ir.StmtAtomic{Pointer: 0, Fun: ir.AtomicAdd{}, Value: 1, Result: &result}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtAtomic)
		if s.Pointer != 10 || s.Value != 20 || *s.Result != 50 {
			t.Errorf("got p=%d v=%d r=%d", s.Pointer, s.Value, *s.Result)
		}
	})

	t.Run("WorkGroupUniformLoad remaps", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtWorkGroupUniformLoad{Pointer: 0, Result: 1}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtWorkGroupUniformLoad)
		if s.Pointer != 10 || s.Result != 20 {
			t.Errorf("got p=%d r=%d", s.Pointer, s.Result)
		}
	})

	t.Run("RayQuery remaps", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtRayQuery{Query: 0, Fun: ir.RayQueryProceed{Result: 1}}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtRayQuery)
		if s.Query != 10 {
			t.Errorf("query = %d, want 10", s.Query)
		}
	})

	t.Run("SubgroupBallot remaps", func(t *testing.T) {
		pred := ir.ExpressionHandle(0)
		stmts := []ir.Statement{
			{Kind: ir.StmtSubgroupBallot{Result: 1, Predicate: &pred}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtSubgroupBallot)
		if s.Result != 20 || *s.Predicate != 10 {
			t.Errorf("got r=%d p=%d", s.Result, *s.Predicate)
		}
	})

	t.Run("SubgroupGather remaps", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtSubgroupGather{Mode: ir.GatherBroadcast{Index: 0}, Argument: 1, Result: 2}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtSubgroupGather)
		if s.Argument != 20 || s.Result != 30 {
			t.Errorf("got arg=%d r=%d", s.Argument, s.Result)
		}
	})

	t.Run("ImageAtomic remaps", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtImageAtomic{Image: 0, Coordinate: 1, Fun: ir.AtomicAdd{}, Value: 2}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtImageAtomic)
		if s.Image != 10 || s.Coordinate != 20 || s.Value != 30 {
			t.Errorf("got img=%d coord=%d val=%d", s.Image, s.Coordinate, s.Value)
		}
	})

	t.Run("SubgroupCollectiveOperation remaps", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtSubgroupCollectiveOperation{Argument: 0, Result: 1}},
		}
		RemapStatementExpressionHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtSubgroupCollectiveOperation)
		if s.Argument != 10 || s.Result != 20 {
			t.Errorf("got arg=%d r=%d", s.Argument, s.Result)
		}
	})
}

// --- visitFunctionTypes tests ---

func TestVisitFunctionTypes(t *testing.T) {
	var visited []ir.TypeHandle
	visit := func(h ir.TypeHandle) { visited = append(visited, h) }

	f := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 1},
		},
		Result: &ir.FunctionResult{Type: 2},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 3},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 4}},
			{Kind: ir.ExprCompose{Type: 5}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // no type handle
		},
	}

	visitFunctionTypes(f, visit)

	want := []ir.TypeHandle{0, 1, 2, 3, 4, 5}
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
	var visited []ir.TypeHandle
	visit := func(h ir.TypeHandle) { visited = append(visited, h) }

	f := &ir.Function{
		Arguments: []ir.FunctionArgument{{Name: "a", Type: 0}},
		Result:    nil,
	}

	visitFunctionTypes(f, visit)

	if len(visited) != 1 || visited[0] != 0 {
		t.Errorf("expected [0], got %v", visited)
	}
}

// --- safeRemapFunctionTypes tests ---

func TestSafeRemapFunctionTypes(t *testing.T) {
	remap := []ir.TypeHandle{1, 0, 2}

	f := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0},
		},
		Result: &ir.FunctionResult{Type: 1},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 2},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 0}},
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
	zv := f.Expressions[0].Kind.(ir.ExprZeroValue)
	if zv.Type != 1 {
		t.Errorf("zero value type = %d, want 1", zv.Type)
	}
}

func TestSafeRemapFunctionTypes_SentinelHandle(t *testing.T) {
	remap := []ir.TypeHandle{1, 0}
	sentinel := ^ir.TypeHandle(0)

	f := &ir.Function{Expressions: []ir.Expression{{Kind: ir.ExprZeroValue{Type: sentinel}}}}

	safeRemapFunctionTypes(f, remap)

	// Sentinel should remain unchanged
	if zv := f.Expressions[0].Kind.(ir.ExprZeroValue); zv.Type != sentinel {
		t.Errorf("sentinel handle should remain unchanged, got %v", zv.Type)
	}
}

// --- remapStmtFuncHandles tests ---

func TestRemapStmtFuncHandles(t *testing.T) {
	remap := []ir.FunctionHandle{10, 20}

	t.Run("Call", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtCall{Function: 0, Arguments: nil}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtCall)
		if s.Function != 10 {
			t.Errorf("function = %d, want 10", s.Function)
		}
	})

	t.Run("Block recurses", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtBlock{Block: ir.Block{{Kind: ir.StmtCall{Function: 1}}}}},
		}
		remapStmtFuncHandles(stmts, remap)
		blk := stmts[0].Kind.(ir.StmtBlock)
		s := blk.Block[0].Kind.(ir.StmtCall)
		if s.Function != 20 {
			t.Errorf("function = %d, want 20", s.Function)
		}
	})

	t.Run("If recurses", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtIf{
				Accept: ir.Block{{Kind: ir.StmtCall{Function: 0}}},
				Reject: ir.Block{{Kind: ir.StmtCall{Function: 1}}},
			}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtIf)
		if s.Accept[0].Kind.(ir.StmtCall).Function != 10 {
			t.Error("accept call not remapped")
		}
		if s.Reject[0].Kind.(ir.StmtCall).Function != 20 {
			t.Error("reject call not remapped")
		}
	})

	t.Run("Switch recurses", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtSwitch{
				Cases: []ir.SwitchCase{
					{Body: ir.Block{{Kind: ir.StmtCall{Function: 0}}}},
				},
			}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtSwitch)
		if s.Cases[0].Body[0].Kind.(ir.StmtCall).Function != 10 {
			t.Error("switch case call not remapped")
		}
	})

	t.Run("Loop recurses", func(t *testing.T) {
		stmts := []ir.Statement{
			{Kind: ir.StmtLoop{
				Body:       ir.Block{{Kind: ir.StmtCall{Function: 0}}},
				Continuing: ir.Block{{Kind: ir.StmtCall{Function: 1}}},
			}},
		}
		remapStmtFuncHandles(stmts, remap)
		s := stmts[0].Kind.(ir.StmtLoop)
		if s.Body[0].Kind.(ir.StmtCall).Function != 10 {
			t.Error("loop body call not remapped")
		}
		if s.Continuing[0].Kind.(ir.StmtCall).Function != 20 {
			t.Error("loop continuing call not remapped")
		}
	})
}

// --- compactExpressions with rich statement trees (covers markStmtExprRefsForCompact and the compaction blockMapper) ---

func TestCompactExpressions_IfStatement(t *testing.T) {
	retVal := ir.ExpressionHandle(3)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // 0: condition
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},   // 1: used in accept store
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},   // 2: unused
					{Kind: ir.Literal{Value: ir.LiteralF32(3.0)}},   // 3: used by return
				},
				Body: []ir.Statement{
					{Kind: ir.StmtIf{
						Condition: 0,
						Accept:    ir.Block{{Kind: ir.StmtStore{Pointer: 1, Value: 1}}},
						Reject:    ir.Block{},
					}},
					{Kind: ir.StmtReturn{Value: &retVal}},
				},
			},
		}},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// Expression 2 (unused) should be removed: 0,1,3 -> 0,1,2
	if len(fn.Expressions) != 3 {
		t.Fatalf("expected 3 expressions, got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_SwitchStatement(t *testing.T) {
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(1)}},   // 0: selector
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 1: used in case body
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}}, // 2: unused
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: 0,
						Cases: []ir.SwitchCase{
							{
								Value: ir.SwitchValueDefault{},
								Body:  ir.Block{{Kind: ir.StmtStore{Pointer: 1, Value: 1}}},
							},
						},
					}},
				},
			},
		}},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 2 {
		t.Fatalf("expected 2 expressions (unused removed), got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_LoopStatement(t *testing.T) {
	breakIfH := ir.ExpressionHandle(2)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},   // 0: used in loop body
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}},   // 1: unused
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // 2: break_if
				},
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body:       ir.Block{{Kind: ir.StmtStore{Pointer: 0, Value: 0}}},
						Continuing: ir.Block{},
						BreakIf:    &breakIfH,
					}},
				},
			},
		}},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	if len(fn.Expressions) != 2 {
		t.Fatalf("expected 2 expressions (unused removed), got %d", len(fn.Expressions))
	}
	loop := fn.Body[0].Kind.(ir.StmtLoop)
	if loop.BreakIf == nil || *loop.BreakIf != 1 {
		t.Errorf("expected break_if remapped to 1, got %v", loop.BreakIf)
	}
}

func TestCompactExpressions_ImageStoreAndAtomic(t *testing.T) {
	resultH := ir.ExpressionHandle(4)
	module := &ir.Module{
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageCompute,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},    // 0: image
					{Kind: ir.Literal{Value: ir.LiteralU32(0)}},   // 1: coord
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 2: value
					{Kind: ir.Literal{Value: ir.LiteralF32(9.9)}}, // 3: unused
					{Kind: ir.ExprAtomicResult{Ty: 0}},            // 4: atomic result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtImageStore{Image: 0, Coordinate: 1, Value: 2}},
					{Kind: ir.StmtAtomic{Pointer: 0, Fun: ir.AtomicAdd{}, Value: 1, Result: &resultH}},
				},
			},
		}},
	}

	compactExpressions(module)

	fn := &module.EntryPoints[0].Function
	// Expression 3 (unused) removed
	if len(fn.Expressions) != 4 {
		t.Fatalf("expected 4 expressions, got %d", len(fn.Expressions))
	}
}

func TestCompactExpressions_CallStatement(t *testing.T) {
	resultH := ir.ExpressionHandle(2)
	module := &ir.Module{
		Functions: []ir.Function{
			{
				Name: "helper",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 0: unused
					{Kind: ir.Literal{Value: ir.LiteralF32(2.0)}}, // 1: used by return
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: func() *ir.ExpressionHandle { h := ir.ExpressionHandle(1); return &h }()}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{{
			Name:  "main",
			Stage: ir.StageVertex,
			Function: ir.Function{
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}}, // 0: arg
					{Kind: ir.Literal{Value: ir.LiteralF32(9.9)}}, // 1: unused
					{Kind: ir.ExprCallResult{Function: 0}},        // 2: call result
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0, Arguments: []ir.ExpressionHandle{0}, Result: &resultH}},
				},
			},
		}},
	}

	compactExpressions(module)

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
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "MyStruct", Inner: ir.StructType{Members: []ir.StructMember{
				{Name: "x", Type: 1, Offset: 0},
			}, Span: 4}}, // 0: depends on 1
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 1: leaf
		},
	}

	ReorderTypes(module, []ir.TypeHandle{0}) // struct registered first

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
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "arr", Inner: ir.ArrayType{Base: 1, Size: ir.ArraySize{Constant: &size}, Stride: 16}}, // 0: depends on 1
			{Name: "f32", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                          // 1: leaf
		},
	}

	ReorderTypes(module, []ir.TypeHandle{0})

	if module.Types[0].Name != "f32" {
		t.Errorf("expected f32 first, got %s", module.Types[0].Name)
	}
}

func TestReorderTypes_WithPointerDependency(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "ptr", Inner: ir.PointerType{Base: 1, Space: ir.SpaceFunction}}, // 0: depends on 1
			{Name: "u32", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},     // 1: leaf
		},
	}

	ReorderTypes(module, []ir.TypeHandle{0})

	if module.Types[0].Name != "u32" {
		t.Errorf("expected u32 first, got %s", module.Types[0].Name)
	}
}

func TestReorderTypes_WithBindingArrayDependency(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "ba", Inner: ir.BindingArrayType{Base: 1}},                              // 0
			{Name: "tex", Inner: ir.ImageType{Dim: ir.Dim2D, Class: ir.ImageClassSampled}}, // 1: leaf
		},
	}

	ReorderTypes(module, []ir.TypeHandle{0})

	if module.Types[0].Name != "tex" {
		t.Errorf("expected tex first, got %s", module.Types[0].Name)
	}
}

// --- remapFunctionTypes tests ---

func TestRemapFunctionTypes(t *testing.T) {
	remap := []ir.TypeHandle{2, 0, 1}

	f := &ir.Function{
		Arguments: []ir.FunctionArgument{
			{Name: "a", Type: 0},
			{Name: "b", Type: 1},
		},
		Result: &ir.FunctionResult{Type: 2},
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 0},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprZeroValue{Type: 1}},
			{Kind: ir.ExprCompose{Type: 2}},
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
	zv := f.Expressions[0].Kind.(ir.ExprZeroValue)
	if zv.Type != 0 {
		t.Errorf("zero value type = %d, want 0", zv.Type)
	}
	comp := f.Expressions[1].Kind.(ir.ExprCompose)
	if comp.Type != 1 {
		t.Errorf("compose type = %d, want 1", comp.Type)
	}
}
