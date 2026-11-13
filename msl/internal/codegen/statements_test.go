package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// Test: If/else statement generation
// =============================================================================

func TestMSL_IfElse(t *testing.T) {
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
	// Rust naga uses switch(x) without space before paren
	mustContainMSL(t, result, "switch(")
	// Case bodies are wrapped in braces with break inside (Rust naga style).
	// Fallthrough case 1 has a body, so it keeps braces and writes no break.
	mustContainMSL(t, result, "case 0: {")
	mustContainMSL(t, result, "case 1: {")
	mustContainMSL(t, result, "default: {")
	// Cases that end with a terminator (Return, Break, Continue, Kill) do NOT get
	// an extra break; appended. Since all non-fallthrough cases end with StmtReturn,
	// there should be 0 break statements. This matches Rust naga's is_terminator check.
	lines := strings.Split(result, "\n")
	breakCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "break;" {
			breakCount++
		}
	}
	if breakCount != 0 {
		t.Errorf("Expected 0 break statements (cases end with return), got %d", breakCount)
	}
}

// nonEmptyFallThroughSwitchModule builds a function whose switch has
// fall-through cases with non-empty bodies, including a non-last default:
//
//	case 1 (falls through):  v = 10
//	default (falls through): v = 20
//	case 2:                  v = 30
//
// WGSL never produces this shape (its fall-through cases are always empty);
// the SPIR-V frontend does, for OpSwitch targets that branch into the next case.
func nonEmptyFallThroughSwitchModule() *ir.Module {
	i32 := ir.TypeHandle(0)
	store := func(value ir.ExpressionHandle) []ir.Statement {
		return []ir.Statement{{Kind: ir.StmtStore{Pointer: 1, Value: value}}}
	}
	return &ir.Module{
		Types: []ir.Type{
			{Name: "", Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
		Functions: []ir.Function{
			{
				Name:      "test_fn",
				LocalVars: []ir.LocalVariable{{Name: "v", Type: i32}},
				Expressions: []ir.Expression{
					{Kind: ir.Literal{Value: ir.LiteralI32(0)}},  // 0: selector
					{Kind: ir.ExprLocalVariable{Variable: 0}},    // 1
					{Kind: ir.Literal{Value: ir.LiteralI32(10)}}, // 2
					{Kind: ir.Literal{Value: ir.LiteralI32(20)}}, // 3
					{Kind: ir.Literal{Value: ir.LiteralI32(30)}}, // 4
				},
				Body: []ir.Statement{
					{Kind: ir.StmtSwitch{
						Selector: 0,
						Cases: []ir.SwitchCase{
							{Value: ir.SwitchValueI32(1), Body: store(2), FallThrough: true},
							{Value: ir.SwitchValueDefault{}, Body: store(3), FallThrough: true},
							{Value: ir.SwitchValueI32(2), Body: store(4)},
						},
					}},
					{Kind: ir.StmtReturn{}},
				},
			},
		},
	}
}

// TestMSL_SwitchNonEmptyFallThrough pins the MSL text of a fall-through case
// with a body. Rust back/msl/writer.rs:4052-4066 writes such a case in braces
// with its body and no break (`write_block_braces = !(fall_through &&
// body.is_empty())`), so the body runs and control falls into the next case.
func TestMSL_SwitchNonEmptyFallThrough(t *testing.T) {
	result := compileModule(t, nonEmptyFallThroughSwitchModule())
	want := "    switch(0) {\n" +
		"        case 1: {\n" +
		"            v = 10;\n" +
		"        }\n" +
		"        default: {\n" +
		"            v = 20;\n" +
		"        }\n" +
		"        case 2: {\n" +
		"            v = 30;\n" +
		"            break;\n" +
		"        }\n" +
		"    }\n"
	if !strings.Contains(result, want) {
		t.Errorf("switch text mismatch.\nwant:\n%s\ngot:\n%s", want, result)
	}
}

func TestMSL_SwitchU32(t *testing.T) {
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
	mustContainMSL(t, result, "while(true) {")
	mustContainMSL(t, result, "break;")
}

func TestMSL_LoopWithContinuing(t *testing.T) {
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
	mustContainMSL(t, result, "while(true) {")
	mustContainMSL(t, result, "continue;")
}

func TestMSL_LoopWithBreakIf(t *testing.T) {
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
	mustContainMSL(t, result, "metal::discard_fragment();")
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
							{Kind: ir.StmtControlBarrier{Flags: tt.flags}},
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
		{"min", ir.AtomicMin{}, "atomic_min_explicit"},
		{"max", ir.AtomicMax{}, "atomic_max_explicit"},
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
	mustContainMSL(t, result, "int _e2 = ")
	mustContainMSL(t, result, "metal::atomic_fetch_add_explicit(&")
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
	mustContainMSL(t, result, "_e1 = ")
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
	mustContainMSL(t, result, "metal::threadgroup_barrier(metal::mem_flags::mem_threadgroup);")
	// Matches Rust naga: WorkGroupUniformLoad result is named via namer.call("") -> "unnamed"
	mustContainMSL(t, result, "unnamed")
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
	mustContainMSL(t, result, ".ready = false;")
}
