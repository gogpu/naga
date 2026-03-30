// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Reachability Analysis Tests
// =============================================================================

// buildTestModule creates a module with:
// - 6 types: f32, vec4, StructA (uses vec4), StructB (uses f32), StructC (uses vec4), StructD (unused)
// - 4 constants: c0 (used by ep), c1 (used by funcA), c2 (unused), c3 (used by funcB)
// - 4 globals: g0 (used by ep), g1 (used by funcA), g2 (unused), g3 (used by funcB)
// - 4 functions:
//   - func0 = entry point function (calls func1, uses g0, c0)
//   - func1 = helper (calls func2, uses g1, c1)
//   - func2 = helper used by func1 (uses g3, c3)
//   - func3 = unused function (uses g2, c2)
//
// Entry point "main" targets func0.
func buildTestModule() *ir.Module {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vec4 := ir.VectorType{Size: ir.Vec4, Scalar: f32}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32},  // Type 0: f32
			{Name: "", Inner: vec4}, // Type 1: vec4
			{Name: "StructA", Inner: ir.StructType{Members: []ir.StructMember{{Name: "pos", Type: 1}}}}, // Type 2: StructA (uses vec4)
			{Name: "StructB", Inner: ir.StructType{Members: []ir.StructMember{{Name: "val", Type: 0}}}}, // Type 3: StructB (uses f32)
			{Name: "StructC", Inner: ir.StructType{Members: []ir.StructMember{{Name: "col", Type: 1}}}}, // Type 4: StructC (uses vec4)
			{Name: "StructD", Inner: ir.StructType{Members: []ir.StructMember{{Name: "x", Type: 0}}}},   // Type 5: StructD (UNUSED)
		},
		Constants: []ir.Constant{
			{Name: "c0", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}}, // c0: used by ep
			{Name: "c1", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40000000}}, // c1: used by funcA
			{Name: "c2", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40400000}}, // c2: UNUSED
			{Name: "c3", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40800000}}, // c3: used by funcB
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "g0", Space: ir.SpaceUniform, Type: 0, Binding: &ir.ResourceBinding{Group: 0, Binding: 0}}, // g0: used by ep
			{Name: "g1", Space: ir.SpaceUniform, Type: 2, Binding: &ir.ResourceBinding{Group: 0, Binding: 1}}, // g1: used by funcA (StructA)
			{Name: "g2", Space: ir.SpaceUniform, Type: 3, Binding: &ir.ResourceBinding{Group: 0, Binding: 2}}, // g2: UNUSED
			{Name: "g3", Space: ir.SpaceUniform, Type: 4, Binding: &ir.ResourceBinding{Group: 0, Binding: 3}}, // g3: used by funcB (StructC)
		},
		Functions: []ir.Function{
			// func0: placeholder (EP function is inline in EntryPoint)
			{Name: "ep_main"},
			// func1: helper — references g1, c1, calls func2
			{
				Name:   "helperA",
				Result: &ir.FunctionResult{Type: 0},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 1}}, // expr0: g1
					{Kind: ir.ExprConstant{Constant: 1}},       // expr1: c1
					{Kind: ir.ExprCallResult{Function: 2}},     // expr2: call func2
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 2}},
				},
			},
			// func2: helper used by func1 — references g3, c3
			{
				Name:   "helperB",
				Result: &ir.FunctionResult{Type: 0},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 3}}, // expr0: g3
					{Kind: ir.ExprConstant{Constant: 3}},       // expr1: c3
				},
				Body: nil,
			},
			// func3: UNUSED function — references g2, c2
			{
				Name:   "unused_func",
				Result: &ir.FunctionResult{Type: 0},
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 2}}, // expr0: g2
					{Kind: ir.ExprConstant{Constant: 2}},       // expr1: c2
				},
				Body: nil,
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}}, // expr0: g0
					{Kind: ir.ExprConstant{Constant: 0}},       // expr1: c0
					{Kind: ir.ExprCallResult{Function: 1}},     // expr2: call func1
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 1}},
				},
			}},
		},
	}

	return module
}

func TestCollectReachable_FunctionsTransitive(t *testing.T) {
	module := buildTestModule()
	ep := &module.EntryPoints[0]

	rs := collectReachable(module, ep)

	// func1 is called by ep → reachable
	if !rs.hasFunction(1) {
		t.Error("func1 should be reachable (called by entry point)")
	}
	// func2 is called by func1 → reachable (transitive)
	if !rs.hasFunction(2) {
		t.Error("func2 should be reachable (called transitively via func1)")
	}
	// func3 is never called → unreachable
	if rs.hasFunction(3) {
		t.Error("func3 should NOT be reachable (never called)")
	}
}

func TestCollectReachable_Globals(t *testing.T) {
	module := buildTestModule()
	ep := &module.EntryPoints[0]

	rs := collectReachable(module, ep)

	// g0 used directly by ep
	if !rs.hasGlobal(0) {
		t.Error("g0 should be reachable (used by entry point)")
	}
	// g1 used by func1 (called by ep)
	if !rs.hasGlobal(1) {
		t.Error("g1 should be reachable (used by func1)")
	}
	// g2 used only by unused func3
	if rs.hasGlobal(2) {
		t.Error("g2 should NOT be reachable (only used by unused func3)")
	}
	// g3 used by func2 (called transitively)
	if !rs.hasGlobal(3) {
		t.Error("g3 should be reachable (used by func2)")
	}
}

func TestCollectReachable_Constants(t *testing.T) {
	module := buildTestModule()
	ep := &module.EntryPoints[0]

	rs := collectReachable(module, ep)

	if !rs.hasConstant(0) {
		t.Error("c0 should be reachable (used by entry point)")
	}
	if !rs.hasConstant(1) {
		t.Error("c1 should be reachable (used by func1)")
	}
	if rs.hasConstant(2) {
		t.Error("c2 should NOT be reachable (only used by unused func3)")
	}
	if !rs.hasConstant(3) {
		t.Error("c3 should be reachable (used by func2)")
	}
}

func TestCollectReachable_Types(t *testing.T) {
	module := buildTestModule()
	ep := &module.EntryPoints[0]

	rs := collectReachable(module, ep)

	// Type 0 (f32) is used by many things → reachable
	if !rs.hasType(0) {
		t.Error("type 0 (f32) should be reachable")
	}
	// Type 1 (vec4) is used by StructA, StructC → reachable
	if !rs.hasType(1) {
		t.Error("type 1 (vec4) should be reachable")
	}
	// Type 2 (StructA) is used by g1 → reachable
	if !rs.hasType(2) {
		t.Error("type 2 (StructA) should be reachable (used by g1)")
	}
	// Type 3 (StructB) is used by g2 → unreachable
	if rs.hasType(3) {
		t.Error("type 3 (StructB) should NOT be reachable (only used by unreachable g2)")
	}
	// Type 4 (StructC) is used by g3 → reachable
	if !rs.hasType(4) {
		t.Error("type 4 (StructC) should be reachable (used by g3)")
	}
	// Type 5 (StructD) is not referenced at all → unreachable
	if rs.hasType(5) {
		t.Error("type 5 (StructD) should NOT be reachable (never referenced)")
	}
}

func TestCollectReachable_EmptyModule(t *testing.T) {
	module := &ir.Module{
		Functions: []ir.Function{
			{Name: "ep_func"},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{}},
		},
	}

	rs := collectReachable(module, &module.EntryPoints[0])

	// With dominates_global_use, functions with no globals are included in all EPs.
	// ep_func has no globals → always included.
	if !rs.hasFunction(0) {
		t.Error("function with no globals should be reachable (dominates_global_use)")
	}
}

func TestCollectReachable_CyclicCalls(t *testing.T) {
	// EP calls func1, func1 calls func0 (cycle).
	// Should not infinite loop.
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			// func0: called by func1 (cycle target)
			{
				Name: "ep_func",
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: 1}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 1}},
				},
			},
			// func1: calls func0
			{
				Name:   "helper",
				Result: &ir.FunctionResult{Type: 0},
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: 0}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 0}},
				},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_func",
				Expressions: []ir.Expression{
					{Kind: ir.ExprCallResult{Function: 1}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{Function: 1}},
				},
			}},
		},
	}

	// Should not hang
	rs := collectReachable(module, &module.EntryPoints[0])

	if !rs.hasFunction(1) {
		t.Error("func1 should be reachable (called by ep)")
	}
}

func TestCollectReachable_NestedStatements(t *testing.T) {
	// Entry point has an if statement whose accept branch calls func1.
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			// func0: placeholder
			{Name: "ep_func"},
			// func1: called from if-accept
			{
				Name:   "inner_helper",
				Result: &ir.FunctionResult{Type: 0},
			},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_func",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtIf{
						Condition: 0,
						Accept: []ir.Statement{
							{Kind: ir.StmtCall{Function: 1}},
						},
						Reject: nil,
					}},
				},
			}},
		},
	}

	rs := collectReachable(module, &module.EntryPoints[0])

	if !rs.hasFunction(1) {
		t.Error("func1 should be reachable (called inside if-accept)")
	}
}

func TestCollectReachable_SwitchAndLoop(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			// func0: placeholder
			{Name: "ep_func"},
			{Name: "switch_case0", Result: &ir.FunctionResult{Type: 0}},
			{Name: "switch_default", Result: &ir.FunctionResult{Type: 0}},
			{Name: "loop_body", Result: &ir.FunctionResult{Type: 0}},
			{Name: "totally_unused", Result: &ir.FunctionResult{Type: 0}},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_func",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: 0,
						Cases: []ir.SwitchCase{
							{
								Value: ir.SwitchValueI32(0),
								Body: []ir.Statement{
									{Kind: ir.StmtCall{Function: 1}},
								},
							},
							{
								Value: ir.SwitchValueDefault{},
								Body: []ir.Statement{
									{Kind: ir.StmtCall{Function: 2}},
								},
							},
						},
					}},
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtCall{Function: 3}},
						},
					}},
				},
			}},
		},
	}

	rs := collectReachable(module, &module.EntryPoints[0])

	if !rs.hasFunction(1) {
		t.Error("switch_case0 should be reachable")
	}
	if !rs.hasFunction(2) {
		t.Error("switch_default should be reachable")
	}
	if !rs.hasFunction(3) {
		t.Error("loop_body should be reachable")
	}
	// With dominates_global_use, functions with no globals are included in all EPs.
	// totally_unused has no globals → always included.
	if !rs.hasFunction(4) {
		t.Error("totally_unused with no globals should be reachable (dominates_global_use)")
	}
}

// =============================================================================
// Integration Tests: Compile with Dead Code Elimination
// =============================================================================

func TestCompile_DeadCodeElimination_Functions(t *testing.T) {
	module := buildTestModule()

	source, _, err := Compile(module, Options{
		LangVersion: Version330,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// helperA and helperB should appear (reachable)
	if !strings.Contains(source, "helperA") {
		t.Error("helperA should be in output (reachable)")
	}
	if !strings.Contains(source, "helperB") {
		t.Error("helperB should be in output (reachable)")
	}

	// unused_func should NOT appear (unreachable)
	if strings.Contains(source, "unused_func") {
		t.Error("unused_func should NOT be in output (unreachable)")
	}
}

func TestCompile_DeadCodeElimination_Globals(t *testing.T) {
	module := buildTestModule()

	source, _, err := Compile(module, Options{
		LangVersion: Version330,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// g0, g1, g3 should appear (reachable) — uniform globals with bindings
	// use _group_G_binding_B_vs naming convention.
	if !strings.Contains(source, "_group_0_binding_0_vs") {
		t.Error("g0 (_group_0_binding_0_vs) should be in output (reachable)")
	}
	if !strings.Contains(source, "_group_0_binding_1_vs") {
		t.Error("g1 (_group_0_binding_1_vs) should be in output (reachable)")
	}
	if !strings.Contains(source, "_group_0_binding_3_vs") {
		t.Error("g3 (_group_0_binding_3_vs) should be in output (reachable)")
	}

	// g2 (binding 2) should NOT appear as a uniform declaration (unreachable)
	if strings.Contains(source, "_group_0_binding_2_vs") {
		t.Error("g2 (_group_0_binding_2_vs) should NOT appear (unreachable)")
	}
}

func TestCompile_DeadCodeElimination_Constants(t *testing.T) {
	module := buildTestModule()

	source, _, err := Compile(module, Options{
		LangVersion: Version330,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Rust naga emits ALL named constants regardless of reachability.
	// The GLSL backend follows this behavior — constant filtering is NOT applied.
	// Verify at least some constants appear.
	if !strings.Contains(source, "c0_") {
		t.Error("c0_ should be in output")
	}
	if !strings.Contains(source, "c1_") {
		t.Error("c1_ should be in output")
	}
}

func TestCompile_DeadCodeElimination_Types(t *testing.T) {
	module := buildTestModule()

	source, _, err := Compile(module, Options{
		LangVersion: Version330,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// StructA and StructC should appear (reachable via g1 and g3)
	if !strings.Contains(source, "StructA") {
		t.Error("StructA should be in output (reachable via g1)")
	}
	if !strings.Contains(source, "StructC") {
		t.Error("StructC should be in output (reachable via g3)")
	}

	// Note: Rust naga emits ALL struct types regardless of reachability.
	// The GLSL backend follows this behavior — struct filtering is NOT applied.
	// Only functions, globals, and constants are filtered by reachability.
}

func TestCompile_NoEntryPoint_NoFiltering(t *testing.T) {
	// Module with no entry points should emit everything (no filtering).
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32},
			{Name: "MyStruct", Inner: ir.StructType{Members: []ir.StructMember{{Name: "val", Type: 0}}}},
		},
		Constants: []ir.Constant{
			{Name: "MY_CONST", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "my_global", Space: ir.SpacePrivate, Type: 0},
		},
	}

	source, _, err := Compile(module, DefaultOptions())
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Everything should be emitted (no entry point = no filtering)
	if !strings.Contains(source, "struct MyStruct") {
		t.Error("MyStruct should be in output when no entry point")
	}
	if !strings.Contains(source, "MY_CONST") {
		t.Error("MY_CONST should be in output when no entry point")
	}
	if !strings.Contains(source, "my_global") {
		t.Error("my_global should be in output when no entry point")
	}
}

func TestCompile_OutputSizeReduction(t *testing.T) {
	// Create a module that simulates the SDF shader bloat scenario:
	// Many functions/types, but only a few used by the selected entry point.
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vec4 := ir.VectorType{Size: ir.Vec4, Scalar: f32}

	types := []ir.Type{
		{Name: "", Inner: f32},
		{Name: "", Inner: vec4},
	}

	// Add 100 struct types
	for i := 0; i < 100; i++ {
		types = append(types, ir.Type{
			Name:  fmt.Sprintf("Struct%d", i),
			Inner: ir.StructType{Members: []ir.StructMember{{Name: "val", Type: 0}}},
		})
	}

	// Add 100 constants
	constants := make([]ir.Constant, 100)
	for i := range constants {
		constants[i] = ir.Constant{
			Name:  fmt.Sprintf("const_%d", i),
			Type:  0,
			Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: uint64(i)},
		}
	}

	// Add 100 globals (each with a different struct type)
	globals := make([]ir.GlobalVariable, 100)
	for i := range globals {
		globals[i] = ir.GlobalVariable{
			Name:  fmt.Sprintf("global_%d", i),
			Space: ir.SpacePrivate,
			Type:  ir.TypeHandle(i + 2), // Use struct types
		}
	}

	// Add 50 unused functions
	functions := make([]ir.Function, 50)
	for i := 0; i < 50; i++ {
		functions[i] = ir.Function{
			Name:   fmt.Sprintf("unused_%d", i+1),
			Result: &ir.FunctionResult{Type: 0},
			Expressions: []ir.Expression{
				{Kind: ir.ExprGlobalVariable{Variable: ir.GlobalVariableHandle(i + 1)}},
				{Kind: ir.ExprConstant{Constant: ir.ConstantHandle(i + 1)}},
			},
		}
	}

	module := &ir.Module{
		Types:           types,
		Constants:       constants,
		GlobalVariables: globals,
		Functions:       functions,
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_main",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
					{Kind: ir.ExprConstant{Constant: 0}},
				},
			}},
		},
	}

	// Compile WITH dead code elimination (default)
	sourceFiltered, _, err := Compile(module, Options{
		LangVersion: Version330,
		EntryPoint:  "main",
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	// Dead code elimination filters functions and globals by reachability.
	// Note: Rust naga emits ALL struct types and ALL named constants regardless
	// of reachability. The GLSL backend follows this behavior.

	// Count function definitions (excluding main) — these ARE filtered
	funcCount := strings.Count(sourceFiltered, "unused_")
	if funcCount > 0 {
		t.Errorf("Expected 0 unused function definitions, got %d occurrences of 'unused_'", funcCount)
	}

	t.Logf("Filtered output size: %d bytes", len(sourceFiltered))
}

func TestCollectReachable_CompositeConstant(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	vec4 := ir.VectorType{Size: ir.Vec4, Scalar: f32}

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32},
			{Name: "", Inner: vec4},
		},
		Constants: []ir.Constant{
			{Name: "x", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x3f800000}},           // c0
			{Name: "y", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40000000}},           // c1
			{Name: "unused", Type: 0, Value: ir.ScalarValue{Kind: ir.ScalarFloat, Bits: 0x40400000}},      // c2: unused
			{Name: "vec", Type: 1, Value: ir.CompositeValue{Components: []ir.ConstantHandle{0, 1, 0, 1}}}, // c3: uses c0, c1
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_func",
				Expressions: []ir.Expression{
					{Kind: ir.ExprConstant{Constant: 3}}, // uses the composite constant
				},
			}},
		},
	}

	rs := collectReachable(module, &module.EntryPoints[0])

	if !rs.hasConstant(3) {
		t.Error("c3 (composite) should be reachable")
	}
	if !rs.hasConstant(0) {
		t.Error("c0 should be reachable (component of c3)")
	}
	if !rs.hasConstant(1) {
		t.Error("c1 should be reachable (component of c3)")
	}
	if rs.hasConstant(2) {
		t.Error("c2 should NOT be reachable")
	}
}

func TestCollectReachable_ArrayType(t *testing.T) {
	f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	arraySize := uint32(4)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: f32},
			{Name: "", Inner: ir.ArrayType{Base: 0, Size: ir.ArraySize{Constant: &arraySize}}},
		},
		GlobalVariables: []ir.GlobalVariable{
			{Name: "arr", Space: ir.SpacePrivate, Type: 1},
		},
		EntryPoints: []ir.EntryPoint{
			{Name: "main", Stage: ir.StageVertex, Function: ir.Function{
				Name: "ep_func",
				Expressions: []ir.Expression{
					{Kind: ir.ExprGlobalVariable{Variable: 0}},
				},
			}},
		},
	}

	rs := collectReachable(module, &module.EntryPoints[0])

	if !rs.hasType(1) {
		t.Error("array type should be reachable")
	}
	if !rs.hasType(0) {
		t.Error("f32 base type should be reachable (via array)")
	}
}
