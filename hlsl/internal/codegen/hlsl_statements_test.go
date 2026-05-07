// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"strings"
	"testing"

	"github.com/gogpu/naga/ir"
)

// =============================================================================
// continueCtx Tests — covers enterLoop, exitLoop, enterSwitch, exitSwitch,
// continueEncountered
// =============================================================================

func TestContinueCtx_LoopOnly(t *testing.T) {
	ctx := &continueCtx{}

	ctx.enterLoop()
	// continueEncountered inside loop but not inside switch: returns ""
	v := ctx.continueEncountered()
	if v != "" {
		t.Errorf("expected empty variable in loop without switch, got %q", v)
	}
	ctx.exitLoop()
}

func TestContinueCtx_SwitchInsideLoop(t *testing.T) {
	ctx := &continueCtx{}
	n := newNamer()

	ctx.enterLoop()
	variable := ctx.enterSwitch(n)
	if variable == "" {
		t.Error("expected non-empty variable for switch inside loop")
	}

	// continueEncountered inside switch: returns the variable
	v := ctx.continueEncountered()
	if v != variable {
		t.Errorf("expected variable %q, got %q", variable, v)
	}

	// exitSwitch should return exitContinue since continue was encountered
	result := ctx.exitSwitch()
	if result.kind != exitContinue {
		t.Errorf("expected exitContinue, got %d", result.kind)
	}
	if result.variable != variable {
		t.Errorf("expected variable %q, got %q", variable, result.variable)
	}

	ctx.exitLoop()
}

func TestContinueCtx_SwitchInsideLoop_NoContinue(t *testing.T) {
	ctx := &continueCtx{}
	n := newNamer()

	ctx.enterLoop()
	variable := ctx.enterSwitch(n)
	if variable == "" {
		t.Error("expected non-empty variable")
	}

	// No continueEncountered -> exitSwitch returns exitNone
	result := ctx.exitSwitch()
	if result.kind != exitNone {
		t.Errorf("expected exitNone, got %d", result.kind)
	}

	ctx.exitLoop()
}

func TestContinueCtx_SwitchOutsideLoop(t *testing.T) {
	ctx := &continueCtx{}
	n := newNamer()

	// Switch not inside a loop
	variable := ctx.enterSwitch(n)
	if variable != "" {
		t.Errorf("expected empty variable for switch outside loop, got %q", variable)
	}

	result := ctx.exitSwitch()
	if result.kind != exitNone {
		t.Errorf("expected exitNone, got %d", result.kind)
	}
}

func TestContinueCtx_NestedSwitch(t *testing.T) {
	ctx := &continueCtx{}
	n := newNamer()

	ctx.enterLoop()

	// Outer switch
	outerVar := ctx.enterSwitch(n)
	if outerVar == "" {
		t.Error("expected non-empty outer variable")
	}

	// Inner switch (nested) — should reuse same variable, return ""
	innerVar := ctx.enterSwitch(n)
	if innerVar != "" {
		t.Errorf("nested switch should return empty (reuse parent var), got %q", innerVar)
	}

	// Continue in inner switch
	v := ctx.continueEncountered()
	if v == "" {
		t.Error("expected non-empty variable for continue in nested switch")
	}

	// Exit inner switch -> exitBreak (propagates to outer)
	innerResult := ctx.exitSwitch()
	if innerResult.kind != exitBreak {
		t.Errorf("expected exitBreak for nested switch, got %d", innerResult.kind)
	}

	// Exit outer switch -> exitContinue (propagated from inner)
	outerResult := ctx.exitSwitch()
	if outerResult.kind != exitContinue {
		t.Errorf("expected exitContinue for outer switch, got %d", outerResult.kind)
	}

	ctx.exitLoop()
}

func TestContinueCtx_Clear(t *testing.T) {
	ctx := &continueCtx{}
	n := newNamer()

	ctx.enterLoop()
	ctx.enterSwitch(n)
	ctx.clear()
	if len(ctx.stack) != 0 {
		t.Errorf("expected empty stack after clear, got %d items", len(ctx.stack))
	}
}

// =============================================================================
// binaryOpStr Tests — covers all binary operator string mappings
// =============================================================================

func TestBinaryOpStr(t *testing.T) {
	tests := []struct {
		name string
		op   ir.BinaryOperator
		want string
	}{
		{"add", ir.BinaryAdd, "+"},
		{"subtract", ir.BinarySubtract, "-"},
		{"multiply", ir.BinaryMultiply, "*"},
		{"divide", ir.BinaryDivide, "/"},
		{"modulo", ir.BinaryModulo, "%"},
		{"equal", ir.BinaryEqual, "=="},
		{"not_equal", ir.BinaryNotEqual, "!="},
		{"less", ir.BinaryLess, "<"},
		{"less_equal", ir.BinaryLessEqual, "<="},
		{"greater", ir.BinaryGreater, ">"},
		{"greater_equal", ir.BinaryGreaterEqual, ">="},
		{"and", ir.BinaryAnd, "&"},
		{"xor", ir.BinaryExclusiveOr, "^"},
		{"or", ir.BinaryInclusiveOr, "|"},
		{"logical_and", ir.BinaryLogicalAnd, "&&"},
		{"logical_or", ir.BinaryLogicalOr, "||"},
		{"shift_left", ir.BinaryShiftLeft, "<<"},
		{"shift_right", ir.BinaryShiftRight, ">>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := binaryOpStr(tt.op)
			if got != tt.want {
				t.Errorf("binaryOpStr(%d) = %q, want %q", tt.op, got, tt.want)
			}
		})
	}
}

// =============================================================================
// scalarKindToHLSL Tests — covers all scalar type mappings
// =============================================================================

func TestScalarKindToHLSL(t *testing.T) {
	tests := []struct {
		name  string
		kind  ir.ScalarKind
		width uint8
		want  string
	}{
		{"bool", ir.ScalarBool, 1, "bool"},
		{"i32", ir.ScalarSint, 4, "int"},
		{"i64", ir.ScalarSint, 8, "int64_t"},
		{"u32", ir.ScalarUint, 4, "uint"},
		{"u64", ir.ScalarUint, 8, "uint64_t"},
		{"f16", ir.ScalarFloat, 2, "half"},
		{"f32", ir.ScalarFloat, 4, "float"},
		{"f64", ir.ScalarFloat, 8, "double"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scalarKindToHLSL(tt.kind, tt.width)
			if got != tt.want {
				t.Errorf("scalarKindToHLSL(%d, %d) = %q, want %q", tt.kind, tt.width, got, tt.want)
			}
		})
	}
}

// =============================================================================
// blockEndsWithTerminator Tests
// =============================================================================

func TestBlockEndsWithTerminator(t *testing.T) {
	tests := []struct {
		name  string
		block ir.Block
		want  bool
	}{
		{"empty", ir.Block{}, false},
		{"break", ir.Block{{Kind: ir.StmtBreak{}}}, true},
		{"continue", ir.Block{{Kind: ir.StmtContinue{}}}, true},
		{"return_void", ir.Block{{Kind: ir.StmtReturn{}}}, true},
		{"kill", ir.Block{{Kind: ir.StmtKill{}}}, true},
		{"store_no_term", ir.Block{{Kind: ir.StmtStore{Pointer: 0, Value: 1}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockEndsWithTerminator(tt.block)
			if got != tt.want {
				t.Errorf("blockEndsWithTerminator() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// mathFunctionToHLSL Tests — covers the mapping table
// =============================================================================

func TestMathFunctionToHLSL(t *testing.T) {
	tests := []struct {
		name string
		fun  ir.MathFunction
		want string
	}{
		{"abs", ir.MathAbs, "abs"},
		{"min", ir.MathMin, "min"},
		{"max", ir.MathMax, "max"},
		{"clamp", ir.MathClamp, "clamp"},
		{"saturate", ir.MathSaturate, "saturate"},
		{"cos", ir.MathCos, "cos"},
		{"sin", ir.MathSin, "sin"},
		{"tan", ir.MathTan, "tan"},
		{"acos", ir.MathAcos, "acos"},
		{"asin", ir.MathAsin, "asin"},
		{"atan", ir.MathAtan, "atan"},
		{"atan2", ir.MathAtan2, "atan2"},
		{"cosh", ir.MathCosh, "cosh"},
		{"sinh", ir.MathSinh, "sinh"},
		{"tanh", ir.MathTanh, "tanh"},
		{"ceil", ir.MathCeil, "ceil"},
		{"floor", ir.MathFloor, "floor"},
		{"round", ir.MathRound, "round"},
		{"fract", ir.MathFract, "frac"},
		{"trunc", ir.MathTrunc, "trunc"},
		{"ldexp", ir.MathLdexp, "ldexp"},
		{"exp", ir.MathExp, "exp"},
		{"exp2", ir.MathExp2, "exp2"},
		{"log", ir.MathLog, "log"},
		{"log2", ir.MathLog2, "log2"},
		{"pow", ir.MathPow, "pow"},
		{"dot", ir.MathDot, "dot"},
		{"cross", ir.MathCross, "cross"},
		{"distance", ir.MathDistance, "distance"},
		{"length", ir.MathLength, "length"},
		{"normalize", ir.MathNormalize, "normalize"},
		{"faceforward", ir.MathFaceForward, "faceforward"},
		{"reflect", ir.MathReflect, "reflect"},
		{"refract", ir.MathRefract, "refract"},
		{"sign", ir.MathSign, "sign"},
		{"fma_mad", ir.MathFma, "mad"},
		{"mix_lerp", ir.MathMix, "lerp"},
		{"step", ir.MathStep, "step"},
		{"smoothstep", ir.MathSmoothStep, "smoothstep"},
		{"sqrt", ir.MathSqrt, "sqrt"},
		{"inverseSqrt", ir.MathInverseSqrt, "rsqrt"},
		{"transpose", ir.MathTranspose, "transpose"},
		{"determinant", ir.MathDeterminant, "determinant"},
		{"radians", ir.MathRadians, "radians"},
		{"degrees", ir.MathDegrees, "degrees"},
		{"countOneBits", ir.MathCountOneBits, "countbits"},
		{"reverseBits", ir.MathReverseBits, "reversebits"},
		{"firstTrailingBit", ir.MathFirstTrailingBit, "firstbitlow"},
		{"firstLeadingBit", ir.MathFirstLeadingBit, "firstbithigh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mathFunctionToHLSL(tt.fun)
			if err != nil {
				t.Fatalf("mathFunctionToHLSL(%d) error: %v", tt.fun, err)
			}
			if got != tt.want {
				t.Errorf("mathFunctionToHLSL(%d) = %q, want %q", tt.fun, got, tt.want)
			}
		})
	}
}

// =============================================================================
// writeStatement dispatch Tests — covers writeStatement switch/dispatch
// =============================================================================

func TestWriteStatement_Emit(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	f32Handle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
		NamedExpressions: map[ir.ExpressionHandle]string{
			0: "value",
		},
	}

	w := newTestWriter(module, nil, map[ir.TypeHandle]string{0: "float"})
	setCurrentFunction(w, fn)
	// Mark expression 0 for baking
	w.needBakeExpressions[0] = struct{}{}

	err := w.writeStatement(ir.StmtEmit{Range: ir.Range{Start: 0, End: 1}})
	if err != nil {
		t.Fatalf("writeStatement(Emit): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "float") {
		t.Errorf("expected type in emit output, got: %q", got)
	}
}

func TestWriteStatement_If(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}
	boolHandle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &boolHandle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	stmt := ir.StmtIf{
		Condition: 0,
		Accept:    ir.Block{{Kind: ir.StmtBreak{}}},
		Reject:    ir.Block{{Kind: ir.StmtBreak{}}},
	}
	err := w.writeStatement(stmt)
	if err != nil {
		t.Fatalf("writeStatement(If): %v", err)
	}
	got := w.Out.String()
	mustContain(t, got, []string{"if (", "} else {", "break;"})
}

func TestWriteStatement_Break(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	err := w.writeStatement(ir.StmtBreak{})
	if err != nil {
		t.Fatalf("writeStatement(Break): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "break;") {
		t.Errorf("expected 'break;', got: %q", got)
	}
}

func TestWriteStatement_Continue(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	err := w.writeStatement(ir.StmtContinue{})
	if err != nil {
		t.Fatalf("writeStatement(Continue): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "continue;") {
		t.Errorf("expected 'continue;', got: %q", got)
	}
}

func TestWriteStatement_Kill(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	err := w.writeStatement(ir.StmtKill{})
	if err != nil {
		t.Fatalf("writeStatement(Kill): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "discard;") {
		t.Errorf("expected 'discard;', got: %q", got)
	}
}

func TestWriteStatement_ReturnVoid(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	err := w.writeStatement(ir.StmtReturn{})
	if err != nil {
		t.Fatalf("writeStatement(Return void): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "return;") {
		t.Errorf("expected 'return;', got: %q", got)
	}
}

func TestWriteStatement_ReturnValue(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	f32Handle := ir.TypeHandle(0)
	exprHandle := ir.ExpressionHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	err := w.writeStatement(ir.StmtReturn{Value: &exprHandle})
	if err != nil {
		t.Fatalf("writeStatement(Return value): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "return ") {
		t.Errorf("expected 'return', got: %q", got)
	}
}

func TestWriteStatement_Barrier(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	tests := []struct {
		name string
		stmt ir.StmtBarrier
		want string
	}{
		{"workgroup", ir.StmtBarrier{Flags: ir.BarrierWorkGroup}, "GroupMemoryBarrierWithGroupSync()"},
		{"storage", ir.StmtBarrier{Flags: ir.BarrierStorage}, "DeviceMemoryBarrierWithGroupSync()"},
		{"both", ir.StmtBarrier{Flags: ir.BarrierWorkGroup | ir.BarrierStorage}, "BarrierWithGroupSync"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.Out.Reset()
			w.Indent = 0
			err := w.writeStatement(tt.stmt)
			if err != nil {
				t.Fatalf("writeStatement(Barrier): %v", err)
			}
			got := w.Out.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("expected %q in output, got: %q", tt.want, got)
			}
		})
	}
}

func TestWriteStatement_Store(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
			{Inner: ir.PointerType{Base: 0, Space: ir.SpaceFunction}},
		},
	}
	ptrHandle := ir.TypeHandle(1)
	i32Handle := ir.TypeHandle(0)

	fn := &ir.Function{
		LocalVars: []ir.LocalVariable{
			{Name: "x", Type: 0},
		},
		Expressions: []ir.Expression{
			{Kind: ir.ExprLocalVariable{Variable: 0}},    // 0: x (pointer)
			{Kind: ir.Literal{Value: ir.LiteralI32(42)}}, // 1: 42
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &ptrHandle},
			{Handle: &i32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)
	w.localNames[0] = "x"

	err := w.writeStatement(ir.StmtStore{Pointer: 0, Value: 1})
	if err != nil {
		t.Fatalf("writeStatement(Store): %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, "x =") {
		t.Errorf("expected store to x, got: %q", got)
	}
}

// =============================================================================
// writeSwitch unit test — covers writeSwitchStatement, writeSwitchCase
// =============================================================================

func TestWriteStatement_Switch(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},
		},
	}
	i32Handle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(0)}}, // 0: selector
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	stmt := ir.StmtSwitch{
		Selector: 0,
		Cases: []ir.SwitchCase{
			{Value: ir.SwitchValueI32(0), Body: ir.Block{{Kind: ir.StmtBreak{}}}},
			{Value: ir.SwitchValueI32(1), Body: ir.Block{{Kind: ir.StmtBreak{}}}},
			{Value: ir.SwitchValueDefault{}, Body: ir.Block{{Kind: ir.StmtBreak{}}}},
		},
	}
	err := w.writeStatement(stmt)
	if err != nil {
		t.Fatalf("writeStatement(Switch): %v", err)
	}
	got := w.Out.String()
	mustContain(t, got, []string{"switch(", "case 0:", "case 1:", "default:"})
}

// =============================================================================
// writeLoop unit test — covers writeLoopStatement
// =============================================================================

func TestWriteStatement_Loop(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},
		},
	}
	boolHandle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &boolHandle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	stmt := ir.StmtLoop{
		Body:       ir.Block{{Kind: ir.StmtBreak{}}},
		Continuing: ir.Block{},
	}
	err := w.writeStatement(stmt)
	if err != nil {
		t.Fatalf("writeStatement(Loop): %v", err)
	}
	got := w.Out.String()
	mustContain(t, got, []string{"while(true)", "break;"})
}

// =============================================================================
// writeBlock unit test
// =============================================================================

func TestWriteBlock(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	block := ir.Block{
		{Kind: ir.StmtBreak{}},
		{Kind: ir.StmtContinue{}},
	}
	err := w.writeBlock(block)
	if err != nil {
		t.Fatalf("writeBlock: %v", err)
	}
	got := w.Out.String()
	mustContain(t, got, []string{"break;", "continue;"})
}

func TestWriteBlockStatement(t *testing.T) {
	module := &ir.Module{}
	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, &ir.Function{
		Expressions:     []ir.Expression{},
		ExpressionTypes: []ir.TypeResolution{},
	})

	stmt := ir.StmtBlock{Block: ir.Block{{Kind: ir.StmtBreak{}}}}
	err := w.writeStatement(stmt)
	if err != nil {
		t.Fatalf("writeStatement(Block): %v", err)
	}
	got := w.Out.String()
	mustContain(t, got, []string{"{", "break;", "}"})
}

// =============================================================================
// isIntegerBinaryOp / isIntOrFloatBinaryOp unit tests
// =============================================================================

func TestIsIntegerBinaryOp(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},  // 0: i32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 1: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // 2: u32
		},
	}
	i32Handle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)
	u32Handle := ir.TypeHandle(2)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralU32(1)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32Handle},
			{Handle: &f32Handle},
			{Handle: &u32Handle},
		},
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	// i32 op -> integer
	if !w.isIntegerBinaryOp(ir.ExprBinary{Left: 0, Right: 0}) {
		t.Error("i32 should be integer")
	}
	// u32 op -> integer
	if !w.isIntegerBinaryOp(ir.ExprBinary{Left: 2, Right: 2}) {
		t.Error("u32 should be integer")
	}
	// f32 op -> not integer
	if w.isIntegerBinaryOp(ir.ExprBinary{Left: 1, Right: 1}) {
		t.Error("f32 should not be integer")
	}

	// isIntOrFloatBinaryOp
	if !w.isIntOrFloatBinaryOp(ir.ExprBinary{Left: 0, Right: 0}) {
		t.Error("i32 should be int or float")
	}
	if !w.isIntOrFloatBinaryOp(ir.ExprBinary{Left: 1, Right: 1}) {
		t.Error("f32 should be int or float")
	}
}

// =============================================================================
// isI32Negate unit test
// =============================================================================

func TestIsI32Negate(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 4}},  // 0: i32
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 1: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarSint, Width: 8}},  // 2: i64
		},
	}
	i32Handle := ir.TypeHandle(0)
	f32Handle := ir.TypeHandle(1)
	i64Handle := ir.TypeHandle(2)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralI32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
			{Kind: ir.Literal{Value: ir.LiteralI64(1)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &i32Handle},
			{Handle: &f32Handle},
			{Handle: &i64Handle},
		},
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	if !w.isI32Negate(0) {
		t.Error("i32 should be I32 negate")
	}
	if w.isI32Negate(1) {
		t.Error("f32 should not be I32 negate")
	}
	if w.isI32Negate(2) {
		t.Error("i64 should not be I32 negate (wrong width)")
	}
}

// =============================================================================
// writeUnaryExpression unit tests
// =============================================================================

func TestWriteUnaryExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},  // 1: bool
			{Inner: ir.ScalarType{Kind: ir.ScalarUint, Width: 4}},  // 2: u32
		},
	}
	f32Handle := ir.TypeHandle(0)
	boolHandle := ir.TypeHandle(1)
	u32Handle := ir.TypeHandle(2)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
			{Kind: ir.Literal{Value: ir.LiteralU32(42)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &boolHandle},
			{Handle: &u32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	tests := []struct {
		name string
		expr ir.ExprUnary
		want string
	}{
		{"negate_f32", ir.ExprUnary{Op: ir.UnaryNegate, Expr: 0}, "-("},
		{"logical_not", ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: 1}, "!("},
		{"bitwise_not", ir.ExprUnary{Op: ir.UnaryBitwiseNot, Expr: 2}, "~("},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.Out.Reset()
			err := w.writeUnaryExpression(tt.expr)
			if err != nil {
				t.Fatalf("writeUnaryExpression: %v", err)
			}
			got := w.Out.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("expected %q in output, got: %q", tt.want, got)
			}
		})
	}
}

// =============================================================================
// writeSelectExpression unit test
// =============================================================================

func TestWriteSelectExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}}, // 0: f32
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},  // 1: bool
		},
	}
	f32Handle := ir.TypeHandle(0)
	boolHandle := ir.TypeHandle(1)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}}, // 0: condition
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},   // 1: accept
			{Kind: ir.Literal{Value: ir.LiteralF32(0.0)}},   // 2: reject
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &boolHandle},
			{Handle: &f32Handle},
			{Handle: &f32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	err := w.writeSelectExpression(ir.ExprSelect{Condition: 0, Accept: 1, Reject: 2})
	if err != nil {
		t.Fatalf("writeSelectExpression: %v", err)
	}
	got := w.Out.String()
	if !strings.Contains(got, " ? ") || !strings.Contains(got, " : ") {
		t.Errorf("expected ternary operator, got: %q", got)
	}
}

// =============================================================================
// writeRelationalExpression unit test
// =============================================================================

func TestWriteRelationalExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},                                      // 0: f32
			{Inner: ir.VectorType{Size: ir.Vec4, Scalar: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}}}, // 1: vec4<bool>
			{Inner: ir.ScalarType{Kind: ir.ScalarBool, Width: 1}},                                       // 2: bool
		},
	}
	f32Handle := ir.TypeHandle(0)
	vec4bHandle := ir.TypeHandle(1)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
			{Kind: ir.Literal{Value: ir.LiteralBool(true)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
			{Handle: &vec4bHandle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	tests := []struct {
		name string
		fun  ir.RelationalFunction
		want string
	}{
		{"all", ir.RelationalAll, "all("},
		{"any", ir.RelationalAny, "any("},
		{"isnan", ir.RelationalIsNan, "isnan("},
		{"isinf", ir.RelationalIsInf, "isinf("},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.Out.Reset()
			var argExpr ir.ExpressionHandle
			if tt.fun == ir.RelationalIsNan || tt.fun == ir.RelationalIsInf {
				argExpr = 0 // f32
			} else {
				argExpr = 1 // vec4<bool>
			}
			err := w.writeRelationalExpression(ir.ExprRelational{Fun: tt.fun, Argument: argExpr})
			if err != nil {
				t.Fatalf("writeRelationalExpression: %v", err)
			}
			got := w.Out.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("expected %q in output, got: %q", tt.want, got)
			}
		})
	}
}

// =============================================================================
// writeDerivativeExpression unit test — covers fine/coarse variants
// =============================================================================

func TestWriteDerivativeExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	f32Handle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
		NamedExpressions: make(map[ir.ExpressionHandle]string),
	}

	w := newTestWriter(module, nil, nil)
	setCurrentFunction(w, fn)

	tests := []struct {
		name string
		axis ir.DerivativeAxis
		ctrl ir.DerivativeControl
		want string
	}{
		{"ddx_none", ir.DerivativeX, ir.DerivativeNone, "ddx("},
		{"ddy_none", ir.DerivativeY, ir.DerivativeNone, "ddy("},
		{"fwidth_none", ir.DerivativeWidth, ir.DerivativeNone, "fwidth("},
		{"ddx_coarse", ir.DerivativeX, ir.DerivativeCoarse, "ddx_coarse("},
		{"ddy_coarse", ir.DerivativeY, ir.DerivativeCoarse, "ddy_coarse("},
		{"ddx_fine", ir.DerivativeX, ir.DerivativeFine, "ddx_fine("},
		{"ddy_fine", ir.DerivativeY, ir.DerivativeFine, "ddy_fine("},
		// Width + Coarse/Fine expands to abs(ddx_X(...)) + abs(ddy_X(...))
		{"fwidth_coarse", ir.DerivativeWidth, ir.DerivativeCoarse, "abs(ddx_coarse("},
		{"fwidth_fine", ir.DerivativeWidth, ir.DerivativeFine, "abs(ddx_fine("},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.Out.Reset()
			expr := ir.ExprDerivative{Axis: tt.axis, Control: tt.ctrl, Expr: 0}
			err := w.writeDerivativeExpression(expr)
			if err != nil {
				t.Fatalf("writeDerivativeExpression: %v", err)
			}
			got := w.Out.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("expected %q in output, got: %q", tt.want, got)
			}
		})
	}
}

// =============================================================================
// nameExpression / expressionTypeStr — covers naming helpers
// =============================================================================

func TestNameExpression(t *testing.T) {
	module := &ir.Module{
		Types: []ir.Type{
			{Inner: ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}},
		},
	}
	f32Handle := ir.TypeHandle(0)

	fn := &ir.Function{
		Expressions: []ir.Expression{
			{Kind: ir.Literal{Value: ir.LiteralF32(1.0)}},
		},
		ExpressionTypes: []ir.TypeResolution{
			{Handle: &f32Handle},
		},
	}

	w := newTestWriter(module, nil, map[ir.TypeHandle]string{0: "float"})
	setCurrentFunction(w, fn)

	name := w.nameExpression(0)
	if name == "" {
		t.Error("expected non-empty expression name")
	}

	typeStr := w.expressionTypeStr(0)
	if typeStr != "float" {
		t.Errorf("expected 'float', got %q", typeStr)
	}
}
