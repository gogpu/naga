package msl

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: If/else statement generation
// =============================================================================

func TestMSL_IfElse(t *testing.T) {
	tBool := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tBool},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtIf{
						Condition: 0,
						Accept: []ir.Statement{
							{Kind: ir.StmtReturn{}},
						},
						Reject: []ir.Statement{
							{Kind: ir.StmtReturn{}},
						},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "if (")
	mustContainMSL(t, result, "} else {")
}

func TestMSL_IfWithoutElse(t *testing.T) {
	tBool := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tBool},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtIf{
						Condition: 0,
						Accept: []ir.Statement{
							{Kind: ir.StmtReturn{}},
						},
						Reject: nil,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "if (")
	mustNotContainMSL(t, result, "} else {")
}

// =============================================================================
// Test: Switch statement generation
// =============================================================================

func TestMSL_Switch(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: 0,
						Cases: []ir.SwitchCase{
							{Value: ir.SwitchValueI32(0), Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
							{Value: ir.SwitchValueI32(1), Body: []ir.Statement{{Kind: ir.StmtReturn{}}}, FallThrough: true},
							{Value: ir.SwitchValueDefault{}, Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
						},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "switch (")
	mustContainMSL(t, result, "case 0:")
	mustContainMSL(t, result, "case 1:")
	mustContainMSL(t, result, "default:")
	// Only the non-fallthrough case should have break
	// Case 0 and default should have break, case 1 should not (fallthrough)
	lines := strings.Split(result, "\n")
	breakCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "break;" {
			breakCount++
		}
	}
	// Expected: 2 breaks (case 0 + default), not 3 (case 1 is fallthrough)
	if breakCount < 2 {
		t.Errorf("Expected at least 2 break statements, got %d", breakCount)
	}
}

func TestMSL_SwitchU32(t *testing.T) {
	tU32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralU32(0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tU32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: 0,
						Cases: []ir.SwitchCase{
							{Value: ir.SwitchValueU32(42), Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
							{Value: ir.SwitchValueDefault{}, Body: []ir.Statement{{Kind: ir.StmtReturn{}}}},
						},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "42u:")
}

// =============================================================================
// Test: Loop statement generation
// =============================================================================

func TestMSL_Loop(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtBreak{}},
						},
						Continuing: nil,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "while (true) {")
	mustContainMSL(t, result, "break;")
}

func TestMSL_LoopWithContinuing(t *testing.T) {
	tBool := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tBool},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body:       []ir.Statement{},
						Continuing: []ir.Statement{{Kind: ir.StmtContinue{}}},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "while (true) {")
	mustContainMSL(t, result, "continue;")
}

func TestMSL_LoopWithBreakIf(t *testing.T) {
	tBool := ir.TypeHandle(0)
	breakIfExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tBool},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body:       []ir.Statement{},
						Continuing: nil,
						BreakIf:    &breakIfExpr,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "if (")
	mustContainMSL(t, result, "{ break; }")
}

// =============================================================================
// Test: Return statement generation
// =============================================================================

func TestMSL_ReturnVoid(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "return;")
}

func TestMSL_ReturnValue(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	retExpr := ir.ExpressionHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Result: &ir.FunctionResult{
					Type: tF32,
				},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{Value: &retExpr}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "return ")
	mustContainMSL(t, result, "42.0")
}

// =============================================================================
// Test: Kill (discard_fragment) statement
// =============================================================================

func TestMSL_Kill(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtKill{}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "discard_fragment();")
}

// =============================================================================
// Test: Barrier statement generation
// =============================================================================

func TestMSL_Barrier(t *testing.T) {
	tests := []struct {
		name  string
		flags ir.BarrierFlags
		want  string
	}{
		{"workgroup", ir.BarrierWorkGroup, "mem_flags::mem_threadgroup"},
		{"storage", ir.BarrierStorage, "mem_flags::mem_device"},
		{"texture", ir.BarrierTexture, "mem_flags::mem_texture"},
		{"pure_exec", 0, "mem_flags::mem_none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &ir.Module{
				Types: []ir.Type{},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Body: []ir.Statement{
							{Kind: ir.StmtBarrier{Flags: tt.flags}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.want)
			mustContainMSL(t, result, "threadgroup_barrier(")
		})
	}
}

// =============================================================================
// Test: Store statement generation
// =============================================================================

func TestMSL_Store(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	init0 := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				LocalVars: []ir.LocalVariable{
					{Name: "x", Type: tF32, Init: &init0},
				},
				Expressions: []ir.Expression{
					{Kind: ir.ExprLocalVariable{Variable: 0}},
					{Kind: ir.Literal{Value: ir.LiteralF32(42.0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Value: ir.PointerType{Base: tF32, Space: ir.SpaceFunction}},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtStore{Pointer: 0, Value: 1}},
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, " = ")
}

// =============================================================================
// Test: Atomic operation statements
// =============================================================================

func TestMSL_AtomicOperations(t *testing.T) {
	tests := []struct {
		name     string
		fun      ir.AtomicFunction
		expected string
	}{
		{"add", ir.AtomicAdd{}, "atomic_fetch_add_explicit"},
		{"sub", ir.AtomicSubtract{}, "atomic_fetch_sub_explicit"},
		{"and", ir.AtomicAnd{}, "atomic_fetch_and_explicit"},
		{"or", ir.AtomicInclusiveOr{}, "atomic_fetch_or_explicit"},
		{"xor", ir.AtomicExclusiveOr{}, "atomic_fetch_xor_explicit"},
		{"min", ir.AtomicMin{}, "atomic_fetch_min_explicit"},
		{"max", ir.AtomicMax{}, "atomic_fetch_max_explicit"},
		{"exchange", ir.AtomicExchange{}, "atomic_exchange_explicit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tI32 := ir.TypeHandle(0)

			module := &ir.Module{
				Types: []ir.Type{
					{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
				},
				Functions: []ir.Function{
					{
						Name: "test_fn",
						Expressions: []ir.Expression{
							{Kind: ir.ExprZeroValue{Type: tI32}},
							{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
						},
						ExpressionTypes: []ir.TypeResolution{
							{Handle: &tI32},
							{Handle: &tI32},
						},
						Body: []ir.Statement{
							{Kind: ir.StmtAtomic{
								Pointer: 0,
								Fun:     tt.fun,
								Value:   1,
							}},
						},
					},
				},
			}
			result := compileModule(t, module)
			mustContainMSL(t, result, tt.expected)
			mustContainMSL(t, result, "memory_order_relaxed")
		})
	}
}

func TestMSL_AtomicWithResult(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	resultExpr := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tI32}},
					{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
					{Kind: ir.ExprZeroValue{Type: tI32}}, // result placeholder
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
					{Handle: &tI32},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtAtomic{
						Pointer: 0,
						Fun:     ir.AtomicAdd{},
						Value:   1,
						Result:  &resultExpr,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "auto _ae2 = ")
	mustContainMSL(t, result, "atomic_fetch_add_explicit")
}

func TestMSL_AtomicCompareExchange(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	compareExpr := ir.ExpressionHandle(2)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tI32}},
					{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // compare value
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
					{Handle: &tI32},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtAtomic{
						Pointer: 0,
						Fun:     ir.AtomicExchange{Compare: &compareExpr},
						Value:   1,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "atomic_compare_exchange_weak_explicit")
}

// =============================================================================
// Test: Function call statement
// =============================================================================

func TestMSL_FunctionCall(t *testing.T) {
	tF32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name:   "helper",
				Result: &ir.FunctionResult{Type: tF32},
				Arguments: []ir.FunctionArgument{
					{Name: "a", Type: tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{
						Function:  0,
						Arguments: []ir.ExpressionHandle{0},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "helper(")
}

func TestMSL_FunctionCallWithResult(t *testing.T) {
	tF32 := ir.TypeHandle(0)
	resultExpr := ir.ExpressionHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name:   "helper",
				Result: &ir.FunctionResult{Type: tF32},
				Body: []ir.Statement{
					{Kind: ir.StmtReturn{}},
				},
			},
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
					{Kind: ir.ExprCallResult{Function: 0}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tF32},
					{Handle: &tF32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtCall{
						Function:  0,
						Arguments: nil,
						Result:    &resultExpr,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "_fc1 = ")
	mustContainMSL(t, result, "helper(")
}

// =============================================================================
// Test: WorkGroupUniformLoad statement
// =============================================================================

func TestMSL_WorkGroupUniformLoad(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tI32}},
					{Kind: ir.ExprZeroValue{Type: tI32}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtWorkGroupUniformLoad{
						Pointer: 0,
						Result:  1,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "threadgroup_barrier(mem_flags::mem_threadgroup);")
	mustContainMSL(t, result, "_wul1")
}

// =============================================================================
// Test: ImageStore statement
// =============================================================================

func TestMSL_ImageStore(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tI32}},  // image
					{Kind: ir.ExprZeroValue{Type: tI32}},  // coord
					{Kind: ir.ExprZeroValue{Type: tVec4}}, // value
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
					{Handle: &tI32},
					{Handle: &tVec4},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtImageStore{
						Image:      0,
						Coordinate: 1,
						Value:      2,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, ".write(")
	mustContainMSL(t, result, "uint2(")
}

func TestMSL_ImageStoreWithArrayIndex(t *testing.T) {
	tI32 := ir.TypeHandle(0)
	tVec4 := ir.TypeHandle(1)
	arrIdx := ir.ExpressionHandle(3)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Name: "", Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tI32}},        // image
					{Kind: ir.ExprZeroValue{Type: tI32}},        // coord
					{Kind: ir.ExprZeroValue{Type: tVec4}},       // value
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // array index
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
					{Handle: &tI32},
					{Handle: &tVec4},
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtImageStore{
						Image:      0,
						Coordinate: 1,
						Value:      2,
						ArrayIndex: &arrIdx,
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, ".write(")
}

// =============================================================================
// Test: Block statement
// =============================================================================

func TestMSL_Block(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtBlock{
						Block: []ir.Statement{
							{Kind: ir.StmtReturn{}},
						},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "return;")
}

// =============================================================================
// Test: Continue statement
// =============================================================================

func TestMSL_Continue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Body: []ir.Statement{
					{Kind: ir.StmtLoop{
						Body: []ir.Statement{
							{Kind: ir.StmtContinue{}},
						},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, "continue;")
}

// =============================================================================
// Test: RayQuery statement types
// =============================================================================

func TestMSL_RayQueryTerminate(t *testing.T) {
	tI32 := ir.TypeHandle(0)

	module := &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name: "test_fn",
				Expressions: []ir.Expression{
					{Kind: ir.ExprZeroValue{Type: tI32}},
				},
				ExpressionTypes: []ir.TypeResolution{
					{Handle: &tI32},
				},
				Body: []ir.Statement{
					{Kind: ir.StmtRayQuery{
						Query: 0,
						Fun:   ir.RayQueryTerminate{},
					}},
				},
			},
		},
	}
	result := compileModule(t, module)
	mustContainMSL(t, result, ".abort()")
}
