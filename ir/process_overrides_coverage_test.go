package ir

import (
	"math"
	"testing"
)

// --- LiteralToFloat ---

func TestLiteralToFloat(t *testing.T) {
	tests := []struct {
		name string
		val  LiteralValue
		want float64
	}{
		{"F32", LiteralF32(3.14), float64(float32(3.14))},
		{"F64", LiteralF64(2.718), 2.718},
		{"I32_positive", LiteralI32(42), 42},
		{"I32_negative", LiteralI32(-7), -7},
		{"U32", LiteralU32(100), 100},
		{"Bool_true", LiteralBool(true), 1.0},
		{"Bool_false", LiteralBool(false), 0.0},
		{"AbstractInt", LiteralAbstractInt(999), 999},
		{"AbstractFloat", LiteralAbstractFloat(1.5), 1.5},
		// I64, U64, F16 are not handled by LiteralToFloat — they fall to default (0)
		{"I64_unhandled", LiteralI64(-123), 0},
		{"U64_unhandled", LiteralU64(456), 0},
		{"F16_unhandled", LiteralF16(1.0), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LiteralToFloat(tt.val)
			if got != tt.want {
				t.Errorf("LiteralToFloat(%v) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

// --- EvalBinaryFloat ---

func TestEvalBinaryFloat(t *testing.T) {
	tests := []struct {
		name        string
		op          BinaryOperator
		left, right float64
		want        float64
	}{
		{"add", BinaryAdd, 2, 3, 5},
		{"subtract", BinarySubtract, 10, 4, 6},
		{"multiply", BinaryMultiply, 3, 7, 21},
		{"divide", BinaryDivide, 15, 3, 5},
		{"divide_by_zero", BinaryDivide, 10, 0, 0},
		{"add_negative", BinaryAdd, -5, 3, -2},
		{"multiply_zero", BinaryMultiply, 100, 0, 0},
		{"unknown_op", BinaryEqual, 1, 2, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvalBinaryFloat(tt.op, tt.left, tt.right)
			if got != tt.want {
				t.Errorf("EvalBinaryFloat(%v, %v, %v) = %v, want %v",
					tt.op, tt.left, tt.right, got, tt.want)
			}
		})
	}
}

// --- EvalUnaryFloat ---

func TestEvalUnaryFloat(t *testing.T) {
	tests := []struct {
		name string
		op   UnaryOperator
		val  float64
		want float64
	}{
		{"negate_positive", UnaryNegate, 5, -5},
		{"negate_negative", UnaryNegate, -3, 3},
		{"negate_zero", UnaryNegate, 0, 0},
		{"logical_not_zero", UnaryLogicalNot, 0, 1},
		{"logical_not_nonzero", UnaryLogicalNot, 5, 0},
		{"logical_not_negative", UnaryLogicalNot, -1, 0},
		{"bitwise_not", UnaryBitwiseNot, 0, float64(^int64(0))},
		{"unknown_op", UnaryOperator(99), 42, 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvalUnaryFloat(tt.op, tt.val)
			if got != tt.want {
				t.Errorf("EvalUnaryFloat(%v, %v) = %v, want %v",
					tt.op, tt.val, got, tt.want)
			}
		})
	}
}

// --- makeLiteralFromProto ---

func TestMakeLiteralFromProto(t *testing.T) {
	tests := []struct {
		name  string
		proto Literal
		val   float64
		check func(t *testing.T, got Literal)
	}{
		{
			name:  "bool_true",
			proto: Literal{Value: LiteralBool(false)},
			val:   1.0,
			check: func(t *testing.T, got Literal) {
				b, ok := got.Value.(LiteralBool)
				if !ok || !bool(b) {
					t.Errorf("expected LiteralBool(true), got %T(%v)", got.Value, got.Value)
				}
			},
		},
		{
			name:  "bool_false",
			proto: Literal{Value: LiteralBool(true)},
			val:   0,
			check: func(t *testing.T, got Literal) {
				b, ok := got.Value.(LiteralBool)
				if !ok || bool(b) {
					t.Errorf("expected LiteralBool(false), got %T(%v)", got.Value, got.Value)
				}
			},
		},
		{
			name:  "i32",
			proto: Literal{Value: LiteralI32(0)},
			val:   42,
			check: func(t *testing.T, got Literal) {
				v, ok := got.Value.(LiteralI32)
				if !ok || int32(v) != 42 {
					t.Errorf("expected LiteralI32(42), got %T(%v)", got.Value, got.Value)
				}
			},
		},
		{
			name:  "u32",
			proto: Literal{Value: LiteralU32(0)},
			val:   100,
			check: func(t *testing.T, got Literal) {
				v, ok := got.Value.(LiteralU32)
				if !ok || uint32(v) != 100 {
					t.Errorf("expected LiteralU32(100), got %T(%v)", got.Value, got.Value)
				}
			},
		},
		{
			name:  "f32",
			proto: Literal{Value: LiteralF32(0)},
			val:   3.14,
			check: func(t *testing.T, got Literal) {
				v, ok := got.Value.(LiteralF32)
				if !ok || float32(v) != float32(3.14) {
					t.Errorf("expected LiteralF32(3.14), got %T(%v)", got.Value, got.Value)
				}
			},
		},
		{
			name:  "f64",
			proto: Literal{Value: LiteralF64(0)},
			val:   2.718,
			check: func(t *testing.T, got Literal) {
				v, ok := got.Value.(LiteralF64)
				if !ok || float64(v) != 2.718 {
					t.Errorf("expected LiteralF64(2.718), got %T(%v)", got.Value, got.Value)
				}
			},
		},
		{
			name:  "unknown_fallback_to_f32",
			proto: Literal{Value: LiteralI64(0)},
			val:   7.5,
			check: func(t *testing.T, got Literal) {
				v, ok := got.Value.(LiteralF32)
				if !ok || float32(v) != 7.5 {
					t.Errorf("expected fallback LiteralF32(7.5), got %T(%v)", got.Value, got.Value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeLiteralFromProto(tt.proto, tt.val)
			tt.check(t, got)
		})
	}
}

// --- needsPreEmit ---

func TestNeedsPreEmit(t *testing.T) {
	tests := []struct {
		name string
		kind ExpressionKind
		want bool
	}{
		{"Literal", Literal{Value: LiteralF32(1)}, true},
		{"ExprConstant", ExprConstant{Constant: 0}, true},
		{"ExprOverride", ExprOverride{Override: 0}, true},
		{"ExprZeroValue", ExprZeroValue{Type: 0}, true},
		{"ExprFunctionArgument", ExprFunctionArgument{Index: 0}, true},
		{"ExprGlobalVariable", ExprGlobalVariable{Variable: 0}, true},
		{"ExprLocalVariable", ExprLocalVariable{Variable: 0}, true},
		{"ExprBinary_no", ExprBinary{Op: BinaryAdd}, false},
		{"ExprUnary_no", ExprUnary{Op: UnaryNegate}, false},
		{"ExprLoad_no", ExprLoad{}, false},
		{"ExprMath_no", ExprMath{Fun: MathAbs}, false},
		{"ExprAs_no", ExprAs{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsPreEmit(tt.kind)
			if got != tt.want {
				t.Errorf("needsPreEmit(%T) = %v, want %v", tt.kind, got, tt.want)
			}
		})
	}
}

// --- filterEmitsInBlock ---

func TestFilterEmitsInBlock(t *testing.T) {
	expressions := []Expression{
		{Kind: Literal{Value: LiteralF32(1)}},       // 0: pre-emit (Literal)
		{Kind: ExprBinary{Op: BinaryAdd}},           // 1: needs emit
		{Kind: ExprConstant{Constant: 0}},           // 2: pre-emit (Constant)
		{Kind: ExprUnary{Op: UnaryNegate, Expr: 1}}, // 3: needs emit
	}

	t.Run("splits_emit_around_pre_emit_expressions", func(t *testing.T) {
		block := Block{
			{Kind: StmtEmit{Range: Range{Start: 0, End: 4}}},
		}
		result := filterEmitsInBlock(block, expressions)
		// Should produce emit ranges that skip indices 0 and 2 (pre-emit)
		// Expected: emit(1,2), emit(3,4)
		emitCount := 0
		for _, s := range result {
			if _, ok := s.Kind.(StmtEmit); ok {
				emitCount++
			}
		}
		if emitCount != 2 {
			t.Errorf("expected 2 emit statements, got %d", emitCount)
		}
	})

	t.Run("preserves_non_emit_statements", func(t *testing.T) {
		block := Block{
			{Kind: StmtStore{Pointer: 0, Value: 1}},
		}
		result := filterEmitsInBlock(block, expressions)
		if len(result) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result))
		}
		if _, ok := result[0].Kind.(StmtStore); !ok {
			t.Errorf("expected StmtStore, got %T", result[0].Kind)
		}
	})

	t.Run("recurses_into_if", func(t *testing.T) {
		block := Block{
			{Kind: StmtIf{
				Condition: 0,
				Accept:    Block{{Kind: StmtEmit{Range: Range{Start: 0, End: 2}}}},
				Reject:    Block{{Kind: StmtStore{Pointer: 0, Value: 1}}},
			}},
		}
		result := filterEmitsInBlock(block, expressions)
		if len(result) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result))
		}
		ifStmt, ok := result[0].Kind.(StmtIf)
		if !ok {
			t.Fatalf("expected StmtIf, got %T", result[0].Kind)
		}
		// Accept should have filtered emit (only index 1 needs emit)
		emitCount := 0
		for _, s := range ifStmt.Accept {
			if _, ok := s.Kind.(StmtEmit); ok {
				emitCount++
			}
		}
		if emitCount != 1 {
			t.Errorf("expected 1 emit in accept block, got %d", emitCount)
		}
	})

	t.Run("recurses_into_switch", func(t *testing.T) {
		block := Block{
			{Kind: StmtSwitch{
				Selector: 0,
				Cases: []SwitchCase{
					{Value: SwitchValueDefault{}, Body: Block{{Kind: StmtEmit{Range: Range{Start: 1, End: 2}}}}},
				},
			}},
		}
		result := filterEmitsInBlock(block, expressions)
		if len(result) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result))
		}
		if _, ok := result[0].Kind.(StmtSwitch); !ok {
			t.Errorf("expected StmtSwitch, got %T", result[0].Kind)
		}
	})

	t.Run("recurses_into_loop", func(t *testing.T) {
		block := Block{
			{Kind: StmtLoop{
				Body:       Block{{Kind: StmtEmit{Range: Range{Start: 1, End: 2}}}},
				Continuing: Block{{Kind: StmtEmit{Range: Range{Start: 3, End: 4}}}},
			}},
		}
		result := filterEmitsInBlock(block, expressions)
		if len(result) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result))
		}
		if _, ok := result[0].Kind.(StmtLoop); !ok {
			t.Errorf("expected StmtLoop, got %T", result[0].Kind)
		}
	})

	t.Run("recurses_into_block_stmt", func(t *testing.T) {
		block := Block{
			{Kind: StmtBlock{
				Block: Block{{Kind: StmtEmit{Range: Range{Start: 0, End: 1}}}},
			}},
		}
		result := filterEmitsInBlock(block, expressions)
		if len(result) != 1 {
			t.Fatalf("expected 1 statement, got %d", len(result))
		}
		bs, ok := result[0].Kind.(StmtBlock)
		if !ok {
			t.Fatalf("expected StmtBlock, got %T", result[0].Kind)
		}
		// Index 0 is Literal (pre-emit), so the emit should be removed entirely
		if len(bs.Block) != 0 {
			t.Errorf("expected 0 statements in inner block (all pre-emit), got %d", len(bs.Block))
		}
	})

	t.Run("all_pre_emit_produces_empty", func(t *testing.T) {
		block := Block{
			{Kind: StmtEmit{Range: Range{Start: 0, End: 1}}}, // Only Literal — pre-emit
		}
		result := filterEmitsInBlock(block, expressions)
		if len(result) != 0 {
			t.Errorf("expected 0 statements (all pre-emit), got %d", len(result))
		}
	})
}

// --- CloneModuleForOverrides ---

func TestCloneModuleForOverrides(t *testing.T) {
	id0 := uint16(0)
	init0 := ExpressionHandle(0)
	localInit := ExpressionHandle(1)

	src := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Overrides: []Override{
			{Name: "gain", ID: &id0, Ty: 0, Init: &init0},
			{Name: "bias", Ty: 0, Init: nil},
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(1.0)}},
		},
		Constants: []Constant{
			{Name: "c0", Type: 0, Init: 0},
		},
		Functions: []Function{
			{
				Name: "helper",
				Expressions: []Expression{
					{Kind: Literal{Value: LiteralF32(2.0)}},
				},
				ExpressionTypes: []TypeResolution{{}},
				LocalVars: []LocalVariable{
					{Name: "x", Type: 0, Init: &localInit},
				},
				NamedExpressions: map[ExpressionHandle]string{0: "lit"},
				Body:             Block{{Kind: StmtReturn{}}},
			},
		},
		EntryPoints: []EntryPoint{
			{
				Name:  "main",
				Stage: StageCompute,
				Function: Function{
					Expressions: []Expression{
						{Kind: Literal{Value: LiteralF32(3.0)}},
					},
					ExpressionTypes: []TypeResolution{{}},
					LocalVars: []LocalVariable{
						{Name: "y", Type: 0, Init: &localInit},
					},
					NamedExpressions: map[ExpressionHandle]string{0: "entry_lit"},
					Body:             Block{{Kind: StmtReturn{}}},
				},
			},
		},
	}

	dst := CloneModuleForOverrides(src)

	t.Run("types_shared", func(t *testing.T) {
		// Types are shared (not deep copied)
		if len(dst.Types) != len(src.Types) {
			t.Errorf("types length mismatch: %d vs %d", len(dst.Types), len(src.Types))
		}
	})

	t.Run("overrides_deep_copied", func(t *testing.T) {
		if len(dst.Overrides) != 2 {
			t.Fatalf("expected 2 overrides, got %d", len(dst.Overrides))
		}
		// Mutating dst override Init should not affect src
		newInit := ExpressionHandle(99)
		dst.Overrides[0].Init = &newInit
		if *src.Overrides[0].Init != init0 {
			t.Error("mutating dst override Init affected src")
		}
		// ID should also be deep copied
		newID := uint16(99)
		dst.Overrides[0].ID = &newID
		if *src.Overrides[0].ID != id0 {
			t.Error("mutating dst override ID affected src")
		}
	})

	t.Run("global_expressions_independent", func(t *testing.T) {
		dst.GlobalExpressions = append(dst.GlobalExpressions, Expression{Kind: Literal{Value: LiteralF32(99)}})
		if len(src.GlobalExpressions) != 1 {
			t.Error("appending to dst GlobalExpressions affected src")
		}
	})

	t.Run("constants_independent", func(t *testing.T) {
		dst.Constants = append(dst.Constants, Constant{Name: "new"})
		if len(src.Constants) != 1 {
			t.Error("appending to dst Constants affected src")
		}
	})

	t.Run("function_expressions_independent", func(t *testing.T) {
		dst.Functions[0].Expressions[0] = Expression{Kind: Literal{Value: LiteralF32(999)}}
		srcLit := src.Functions[0].Expressions[0].Kind.(Literal)
		if float32(srcLit.Value.(LiteralF32)) == 999 {
			t.Error("mutating dst function expressions affected src")
		}
	})

	t.Run("function_local_vars_deep_copied", func(t *testing.T) {
		newH := ExpressionHandle(77)
		dst.Functions[0].LocalVars[0].Init = &newH
		if *src.Functions[0].LocalVars[0].Init != localInit {
			t.Error("mutating dst function local var Init affected src")
		}
	})

	t.Run("function_named_expressions_independent", func(t *testing.T) {
		dst.Functions[0].NamedExpressions[0] = "changed"
		if src.Functions[0].NamedExpressions[0] != "lit" {
			t.Error("mutating dst function named expressions affected src")
		}
	})

	t.Run("entry_point_expressions_independent", func(t *testing.T) {
		dst.EntryPoints[0].Function.Expressions[0] = Expression{Kind: Literal{Value: LiteralF32(888)}}
		srcLit := src.EntryPoints[0].Function.Expressions[0].Kind.(Literal)
		if float32(srcLit.Value.(LiteralF32)) == 888 {
			t.Error("mutating dst entry point expressions affected src")
		}
	})

	t.Run("entry_point_local_vars_deep_copied", func(t *testing.T) {
		newH := ExpressionHandle(66)
		dst.EntryPoints[0].Function.LocalVars[0].Init = &newH
		if *src.EntryPoints[0].Function.LocalVars[0].Init != localInit {
			t.Error("mutating dst entry point local var Init affected src")
		}
	})

	t.Run("entry_point_named_expressions_independent", func(t *testing.T) {
		dst.EntryPoints[0].Function.NamedExpressions[0] = "modified"
		if src.EntryPoints[0].Function.NamedExpressions[0] != "entry_lit" {
			t.Error("mutating dst entry point named expressions affected src")
		}
	})
}

// --- evalFuncExprAsFloat ---

func TestEvalFuncExprAsFloat(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}},
		},
		Constants: []Constant{
			{Name: "c0", Type: 0, Init: 0},
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(10.0)}}, // constant c0 init
		},
	}

	fn := &Function{
		Name: "test",
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(3.0)}},              // 0: literal 3
			{Kind: ExprConstant{Constant: 0}},                    // 1: constant c0 (=10)
			{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}}, // 2: 3 + 10
			{Kind: ExprUnary{Op: UnaryNegate, Expr: 0}},          // 3: -3
			{Kind: ExprAccess{}},                                 // 4: unsupported
		},
	}

	t.Run("literal", func(t *testing.T) {
		val, ok := evalFuncExprAsFloat(fn, module, 0)
		if !ok || val != float64(float32(3.0)) {
			t.Errorf("expected (3.0, true), got (%v, %v)", val, ok)
		}
	})

	t.Run("constant", func(t *testing.T) {
		val, ok := evalFuncExprAsFloat(fn, module, 1)
		if !ok || val != float64(float32(10.0)) {
			t.Errorf("expected (10.0, true), got (%v, %v)", val, ok)
		}
	})

	t.Run("binary_add", func(t *testing.T) {
		val, ok := evalFuncExprAsFloat(fn, module, 2)
		if !ok || val != float64(float32(3.0))+float64(float32(10.0)) {
			t.Errorf("expected (13.0, true), got (%v, %v)", val, ok)
		}
	})

	t.Run("unary_negate", func(t *testing.T) {
		val, ok := evalFuncExprAsFloat(fn, module, 3)
		if !ok || val != -float64(float32(3.0)) {
			t.Errorf("expected (-3.0, true), got (%v, %v)", val, ok)
		}
	})

	t.Run("unsupported_kind", func(t *testing.T) {
		_, ok := evalFuncExprAsFloat(fn, module, 4)
		if ok {
			t.Error("expected (_, false) for unsupported expression kind")
		}
	})

	t.Run("out_of_range", func(t *testing.T) {
		_, ok := evalFuncExprAsFloat(fn, module, 999)
		if ok {
			t.Error("expected (_, false) for out-of-range handle")
		}
	})
}

// --- exprAsLiteral ---

func TestExprAsLiteral(t *testing.T) {
	module := &Module{
		Constants: []Constant{
			{Name: "pi", Type: 0, Init: 0},
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(3.14)}},
		},
	}

	fn := &Function{
		Expressions: []Expression{
			{Kind: Literal{Value: LiteralF32(2.0)}}, // 0: direct literal
			{Kind: ExprConstant{Constant: 0}},       // 1: constant ref
			{Kind: ExprBinary{Op: BinaryAdd}},       // 2: not a literal
		},
	}

	t.Run("direct_literal", func(t *testing.T) {
		val, lit, ok := exprAsLiteral(fn, module, 0)
		if !ok {
			t.Fatal("expected ok=true for direct literal")
		}
		if val != float64(float32(2.0)) {
			t.Errorf("val = %v, want 2.0", val)
		}
		if _, isF32 := lit.Value.(LiteralF32); !isF32 {
			t.Errorf("expected LiteralF32, got %T", lit.Value)
		}
	})

	t.Run("constant_ref", func(t *testing.T) {
		val, _, ok := exprAsLiteral(fn, module, 1)
		if !ok {
			t.Fatal("expected ok=true for constant ref")
		}
		if val != float64(float32(3.14)) {
			t.Errorf("val = %v, want ~3.14", val)
		}
	})

	t.Run("non_literal", func(t *testing.T) {
		_, _, ok := exprAsLiteral(fn, module, 2)
		if ok {
			t.Error("expected ok=false for non-literal expression")
		}
	})

	t.Run("out_of_range", func(t *testing.T) {
		_, _, ok := exprAsLiteral(fn, module, 999)
		if ok {
			t.Error("expected ok=false for out-of-range handle")
		}
	})
}

// --- tryConstFoldExpr ---

func TestTryConstFoldExpr(t *testing.T) {
	module := &Module{
		Constants: []Constant{
			{Name: "c", Type: 0, Init: 0},
		},
		GlobalExpressions: []Expression{
			{Kind: Literal{Value: LiteralF32(5.0)}},
		},
	}

	t.Run("binary_fold", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: Literal{Value: LiteralF32(3.0)}},                   // 0
				{Kind: Literal{Value: LiteralF32(4.0)}},                   // 1
				{Kind: ExprBinary{Op: BinaryMultiply, Left: 0, Right: 1}}, // 2: 3*4
			},
		}
		result, ok := tryConstFoldExpr(fn, module, 2)
		if !ok {
			t.Fatal("expected folding to succeed")
		}
		lit, isLit := result.(Literal)
		if !isLit {
			t.Fatalf("expected Literal, got %T", result)
		}
		f, isF32 := lit.Value.(LiteralF32)
		if !isF32 || float32(f) != 12 {
			t.Errorf("expected LiteralF32(12), got %v", lit.Value)
		}
	})

	t.Run("unary_negate_fold", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: Literal{Value: LiteralF32(7.0)}},     // 0
				{Kind: ExprUnary{Op: UnaryNegate, Expr: 0}}, // 1: -7
			},
		}
		result, ok := tryConstFoldExpr(fn, module, 1)
		if !ok {
			t.Fatal("expected folding to succeed")
		}
		lit := result.(Literal)
		f := lit.Value.(LiteralF32)
		if float32(f) != -7 {
			t.Errorf("expected LiteralF32(-7), got %v", f)
		}
	})

	t.Run("unary_logical_not_bool", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: Literal{Value: LiteralBool(true)}},       // 0
				{Kind: ExprUnary{Op: UnaryLogicalNot, Expr: 0}}, // 1: !true
			},
		}
		result, ok := tryConstFoldExpr(fn, module, 1)
		if !ok {
			t.Fatal("expected folding to succeed")
		}
		lit := result.(Literal)
		b, isBool := lit.Value.(LiteralBool)
		if !isBool || bool(b) {
			t.Errorf("expected LiteralBool(false), got %v", lit.Value)
		}
	})

	t.Run("non_foldable", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprLoad{}}, // 0: not a literal
			},
		}
		_, ok := tryConstFoldExpr(fn, module, 0)
		if ok {
			t.Error("expected folding to fail for non-literal expression")
		}
	})

	t.Run("binary_non_literal_operand", func(t *testing.T) {
		fn := &Function{
			Expressions: []Expression{
				{Kind: ExprLoad{}},                                   // 0: not a literal
				{Kind: Literal{Value: LiteralF32(1.0)}},              // 1
				{Kind: ExprBinary{Op: BinaryAdd, Left: 0, Right: 1}}, // 2
			},
		}
		_, ok := tryConstFoldExpr(fn, module, 2)
		if ok {
			t.Error("expected folding to fail when operand is not a literal")
		}
	})
}

// --- overrideRemapExprHandles: verifies handle remapping preserves expression structure ---

func TestOverrideRemapExprHandles(t *testing.T) {
	// handleMap: old handle -> new handle. E.g., old 0->10, 1->11, 2->12.
	handleMap := []ExpressionHandle{10, 11, 12, 13, 14}

	t.Run("access", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprAccess{Base: 0, Index: 1}, handleMap)
		a := result.(ExprAccess)
		if a.Base != 10 || a.Index != 11 {
			t.Errorf("expected {10,11}, got {%d,%d}", a.Base, a.Index)
		}
	})

	t.Run("access_index", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprAccessIndex{Base: 2, Index: 5}, handleMap)
		a := result.(ExprAccessIndex)
		if a.Base != 12 || a.Index != 5 { // Index is constant, not remapped
			t.Errorf("expected {12,5}, got {%d,%d}", a.Base, a.Index)
		}
	})

	t.Run("binary", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprBinary{Op: BinaryAdd, Left: 0, Right: 2}, handleMap)
		b := result.(ExprBinary)
		if b.Left != 10 || b.Right != 12 {
			t.Errorf("expected {10,12}, got {%d,%d}", b.Left, b.Right)
		}
	})

	t.Run("unary", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprUnary{Op: UnaryNegate, Expr: 1}, handleMap)
		u := result.(ExprUnary)
		if u.Expr != 11 {
			t.Errorf("expected 11, got %d", u.Expr)
		}
	})

	t.Run("load", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprLoad{Pointer: 3}, handleMap)
		l := result.(ExprLoad)
		if l.Pointer != 13 {
			t.Errorf("expected 13, got %d", l.Pointer)
		}
	})

	t.Run("select", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprSelect{Condition: 0, Accept: 1, Reject: 2}, handleMap)
		s := result.(ExprSelect)
		if s.Condition != 10 || s.Accept != 11 || s.Reject != 12 {
			t.Errorf("expected {10,11,12}, got {%d,%d,%d}", s.Condition, s.Accept, s.Reject)
		}
	})

	t.Run("splat", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprSplat{Size: Vec3, Value: 0}, handleMap)
		s := result.(ExprSplat)
		if s.Value != 10 || s.Size != Vec3 {
			t.Errorf("expected {10,Vec3}, got {%d,%v}", s.Value, s.Size)
		}
	})

	t.Run("swizzle", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprSwizzle{Size: Vec2, Vector: 1, Pattern: [4]SwizzleComponent{SwizzleX, SwizzleY}}, handleMap)
		s := result.(ExprSwizzle)
		if s.Vector != 11 {
			t.Errorf("expected vector=11, got %d", s.Vector)
		}
	})

	t.Run("compose", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprCompose{Type: 0, Components: []ExpressionHandle{0, 1, 2}}, handleMap)
		c := result.(ExprCompose)
		if len(c.Components) != 3 || c.Components[0] != 10 || c.Components[1] != 11 || c.Components[2] != 12 {
			t.Errorf("expected [10,11,12], got %v", c.Components)
		}
	})

	t.Run("as", func(t *testing.T) {
		w := uint8(4)
		result := overrideRemapExprHandles(ExprAs{Expr: 0, Kind: ScalarFloat, Convert: &w}, handleMap)
		a := result.(ExprAs)
		if a.Expr != 10 {
			t.Errorf("expected 10, got %d", a.Expr)
		}
	})

	t.Run("derivative", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprDerivative{Axis: DerivativeX, Control: DerivativeFine, Expr: 2}, handleMap)
		d := result.(ExprDerivative)
		if d.Expr != 12 {
			t.Errorf("expected 12, got %d", d.Expr)
		}
	})

	t.Run("relational", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprRelational{Fun: RelationalIsNan, Argument: 1}, handleMap)
		r := result.(ExprRelational)
		if r.Argument != 11 {
			t.Errorf("expected 11, got %d", r.Argument)
		}
	})

	t.Run("array_length", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprArrayLength{Array: 4}, handleMap)
		a := result.(ExprArrayLength)
		if a.Array != 14 {
			t.Errorf("expected 14, got %d", a.Array)
		}
	})

	t.Run("math_multi_arg", func(t *testing.T) {
		arg1 := ExpressionHandle(1)
		arg2 := ExpressionHandle(2)
		result := overrideRemapExprHandles(ExprMath{Fun: MathClamp, Arg: 0, Arg1: &arg1, Arg2: &arg2}, handleMap)
		m := result.(ExprMath)
		if m.Arg != 10 || *m.Arg1 != 11 || *m.Arg2 != 12 {
			t.Errorf("expected {10,11,12}, got {%d,%d,%d}", m.Arg, *m.Arg1, *m.Arg2)
		}
	})

	t.Run("math_4_args", func(t *testing.T) {
		arg1 := ExpressionHandle(1)
		arg2 := ExpressionHandle(2)
		arg3 := ExpressionHandle(3)
		result := overrideRemapExprHandles(ExprMath{Fun: MathInsertBits, Arg: 0, Arg1: &arg1, Arg2: &arg2, Arg3: &arg3}, handleMap)
		m := result.(ExprMath)
		if m.Arg != 10 || *m.Arg1 != 11 || *m.Arg2 != 12 || *m.Arg3 != 13 {
			t.Errorf("expected {10,11,12,13}, got {%d,%d,%d,%d}", m.Arg, *m.Arg1, *m.Arg2, *m.Arg3)
		}
	})

	t.Run("image_load", func(t *testing.T) {
		arrIdx := ExpressionHandle(2)
		sample := ExpressionHandle(3)
		level := ExpressionHandle(4)
		result := overrideRemapExprHandles(ExprImageLoad{
			Image: 0, Coordinate: 1, ArrayIndex: &arrIdx, Sample: &sample, Level: &level,
		}, handleMap)
		il := result.(ExprImageLoad)
		if il.Image != 10 || il.Coordinate != 11 || *il.ArrayIndex != 12 || *il.Sample != 13 || *il.Level != 14 {
			t.Error("image load handles not remapped correctly")
		}
	})

	t.Run("image_query_size_with_level", func(t *testing.T) {
		level := ExpressionHandle(1)
		result := overrideRemapExprHandles(ExprImageQuery{
			Image: 0, Query: ImageQuerySize{Level: &level},
		}, handleMap)
		iq := result.(ExprImageQuery)
		if iq.Image != 10 {
			t.Errorf("expected image=10, got %d", iq.Image)
		}
		qs := iq.Query.(ImageQuerySize)
		if qs.Level == nil || *qs.Level != 11 {
			t.Error("image query level not remapped")
		}
	})

	t.Run("alias", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprAlias{Source: 2}, handleMap)
		a := result.(ExprAlias)
		if a.Source != 12 {
			t.Errorf("expected 12, got %d", a.Source)
		}
	})

	t.Run("phi", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprPhi{Incoming: []PhiIncoming{
			{PredKey: PhiPredIfAccept, Value: 0},
			{PredKey: PhiPredIfReject, Value: 1},
		}}, handleMap)
		p := result.(ExprPhi)
		if p.Incoming[0].Value != 10 || p.Incoming[1].Value != 11 {
			t.Errorf("phi values not remapped: got %d, %d", p.Incoming[0].Value, p.Incoming[1].Value)
		}
	})

	t.Run("literal_passthrough", func(t *testing.T) {
		// Literal has no sub-expression handles — should pass through unchanged
		result := overrideRemapExprHandles(Literal{Value: LiteralF32(1.0)}, handleMap)
		_, ok := result.(Literal)
		if !ok {
			t.Errorf("expected Literal passthrough, got %T", result)
		}
	})

	t.Run("image_sample", func(t *testing.T) {
		offset := ExpressionHandle(3)
		depthRef := ExpressionHandle(4)
		result := overrideRemapExprHandles(ExprImageSample{
			Image: 0, Sampler: 1, Coordinate: 2,
			Offset: &offset, DepthRef: &depthRef,
			Level: SampleLevelExact{Level: 3},
		}, handleMap)
		is := result.(ExprImageSample)
		if is.Image != 10 || is.Sampler != 11 || is.Coordinate != 12 {
			t.Error("image sample base handles not remapped")
		}
		exact := is.Level.(SampleLevelExact)
		if exact.Level != 13 {
			t.Errorf("expected sample level=13, got %d", exact.Level)
		}
	})

	t.Run("image_sample_bias_level", func(t *testing.T) {
		result := overrideRemapExprHandles(ExprImageSample{
			Image: 0, Sampler: 1, Coordinate: 2,
			Level: SampleLevelBias{Bias: 3},
		}, handleMap)
		is := result.(ExprImageSample)
		bias := is.Level.(SampleLevelBias)
		if bias.Bias != 13 {
			t.Errorf("expected bias=13, got %d", bias.Bias)
		}
	})
}

// --- makeOverrideLiteral ---

func TestMakeOverrideLiteral(t *testing.T) {
	module := &Module{
		Types: []Type{
			{Name: "bool", Inner: ScalarType{Kind: ScalarBool, Width: 1}}, // 0
			{Name: "i32", Inner: ScalarType{Kind: ScalarSint, Width: 4}},  // 1
			{Name: "u32", Inner: ScalarType{Kind: ScalarUint, Width: 4}},  // 2
			{Name: "f32", Inner: ScalarType{Kind: ScalarFloat, Width: 4}}, // 3
			{Name: "f64", Inner: ScalarType{Kind: ScalarFloat, Width: 8}}, // 4
			{Name: "vec2f", Inner: VectorType{Size: Vec2}},                // 5: non-scalar
		},
	}

	t.Run("bool_true", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 0, 1.0)
		b, ok := lit.Value.(LiteralBool)
		if !ok || !bool(b) {
			t.Errorf("expected LiteralBool(true), got %T(%v)", lit.Value, lit.Value)
		}
	})

	t.Run("bool_false_nan", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 0, math.NaN())
		b, ok := lit.Value.(LiteralBool)
		if !ok || bool(b) {
			t.Errorf("expected LiteralBool(false), got %T(%v)", lit.Value, lit.Value)
		}
	})

	t.Run("i32", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 1, -42)
		v, ok := lit.Value.(LiteralI32)
		if !ok || int32(v) != -42 {
			t.Errorf("expected LiteralI32(-42), got %T(%v)", lit.Value, lit.Value)
		}
	})

	t.Run("u32", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 2, 255)
		v, ok := lit.Value.(LiteralU32)
		if !ok || uint32(v) != 255 {
			t.Errorf("expected LiteralU32(255), got %T(%v)", lit.Value, lit.Value)
		}
	})

	t.Run("f32", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 3, 3.14)
		v, ok := lit.Value.(LiteralF32)
		if !ok || float32(v) != float32(3.14) {
			t.Errorf("expected LiteralF32(3.14), got %T(%v)", lit.Value, lit.Value)
		}
	})

	t.Run("f64", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 4, 2.718)
		v, ok := lit.Value.(LiteralF64)
		if !ok || float64(v) != 2.718 {
			t.Errorf("expected LiteralF64(2.718), got %T(%v)", lit.Value, lit.Value)
		}
	})

	t.Run("non_scalar_fallback", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 5, 7.5)
		_, ok := lit.Value.(LiteralF32)
		if !ok {
			t.Errorf("expected fallback LiteralF32, got %T", lit.Value)
		}
	})

	t.Run("out_of_range_type_fallback", func(t *testing.T) {
		lit := makeOverrideLiteral(module, 999, 1.0)
		_, ok := lit.Value.(LiteralF32)
		if !ok {
			t.Errorf("expected fallback LiteralF32, got %T", lit.Value)
		}
	})
}
