package front

// Contracts of the lower-level helpers of expression lowering: the exact
// constant-index rule of context.rs:556-595, Rust's literal tables
// (Literal::zero and Literal::one, types.rs:587-610), and the const-context
// name resolution of context.rs:1024-1082.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestZeroAndOneLiteralTables is Rust's Literal::zero and Literal::one
// (types.rs:587-610) for every width the IR has, with the false result for the
// kinds that have no arithmetic value.
func TestZeroAndOneLiteralTables(t *testing.T) {
	scalars := []ir.ScalarType{
		{Kind: ir.ScalarSint, Width: 2},
		{Kind: ir.ScalarSint, Width: 4},
		{Kind: ir.ScalarSint, Width: 8},
		{Kind: ir.ScalarUint, Width: 2},
		{Kind: ir.ScalarUint, Width: 4},
		{Kind: ir.ScalarUint, Width: 8},
		{Kind: ir.ScalarFloat, Width: 2},
		{Kind: ir.ScalarFloat, Width: 4},
		{Kind: ir.ScalarFloat, Width: 8},
	}
	zeroTypes := map[ir.ScalarType]any{
		{Kind: ir.ScalarSint, Width: 2}:  ir.LiteralI16(0),
		{Kind: ir.ScalarSint, Width: 4}:  ir.LiteralI32(0),
		{Kind: ir.ScalarSint, Width: 8}:  ir.LiteralI64(0),
		{Kind: ir.ScalarUint, Width: 2}:  ir.LiteralU16(0),
		{Kind: ir.ScalarUint, Width: 4}:  ir.LiteralU32(0),
		{Kind: ir.ScalarUint, Width: 8}:  ir.LiteralU64(0),
		{Kind: ir.ScalarFloat, Width: 2}: ir.LiteralF16(0),
		{Kind: ir.ScalarFloat, Width: 4}: ir.LiteralF32(0),
		{Kind: ir.ScalarFloat, Width: 8}: ir.LiteralF64(0),
	}
	oneTypes := map[ir.ScalarType]any{
		{Kind: ir.ScalarSint, Width: 2}:  ir.LiteralI16(1),
		{Kind: ir.ScalarSint, Width: 4}:  ir.LiteralI32(1),
		{Kind: ir.ScalarSint, Width: 8}:  ir.LiteralI64(1),
		{Kind: ir.ScalarUint, Width: 2}:  ir.LiteralU16(1),
		{Kind: ir.ScalarUint, Width: 4}:  ir.LiteralU32(1),
		{Kind: ir.ScalarUint, Width: 8}:  ir.LiteralU64(1),
		{Kind: ir.ScalarFloat, Width: 2}: ir.LiteralF16(1),
		{Kind: ir.ScalarFloat, Width: 4}: ir.LiteralF32(1),
		{Kind: ir.ScalarFloat, Width: 8}: ir.LiteralF64(1),
	}
	for _, scalar := range scalars {
		zero, ok := zeroLiteral(scalar)
		if !ok || zero != zeroTypes[scalar] {
			t.Errorf("zeroLiteral(%v) = %#v, %t; want %#v", scalar, zero, ok, zeroTypes[scalar])
		}
		one, ok := oneLiteral(scalar)
		if !ok || one != oneTypes[scalar] {
			t.Errorf("oneLiteral(%v) = %#v, %t; want %#v", scalar, one, ok, oneTypes[scalar])
		}
	}
	if _, ok := zeroLiteral(boolScalar); !ok {
		t.Error("zeroLiteral(bool) must be false")
	}
	if _, ok := oneLiteral(boolScalar); ok {
		t.Error("oneLiteral(bool) must not exist")
	}
	if _, ok := zeroLiteral(ir.ScalarType{Kind: ir.ScalarSint, Width: 3}); ok {
		t.Error("zeroLiteral of a three-byte int must not exist")
	}
	if _, ok := oneLiteral(ir.ScalarType{Kind: ir.ScalarUint, Width: 3}); ok {
		t.Error("oneLiteral of a three-byte uint must not exist")
	}
	if _, ok := oneLiteral(ir.ScalarType{Kind: ir.ScalarFloat, Width: 3}); ok {
		t.Error("oneLiteral of a three-byte float must not exist")
	}
}

// TestMinusOneLiteral is Rust's Literal::minus_one: only float scalars have the
// value, which is what the matrix negation needs.
func TestMinusOneLiteral(t *testing.T) {
	tests := []struct {
		scalar ir.ScalarType
		want   ir.LiteralValue
		ok     bool
	}{
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, ir.LiteralF16(-1), true},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}, ir.LiteralF32(-1), true},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, ir.LiteralF64(-1), true},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 3}, nil, false},
		{i32Scalar, nil, false},
		{boolScalar, nil, false},
	}
	for _, tt := range tests {
		got, ok := minusOneLiteral(tt.scalar)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("minusOneLiteral(%v) = %#v, %t; want %#v, %t", tt.scalar, got, ok, tt.want, tt.ok)
		}
	}
}

// TestUnaryOpDebug is Rust's Debug spelling of UnaryOperator, used in the
// matrix negation error.
func TestUnaryOpDebug(t *testing.T) {
	tests := []struct {
		op   ir.UnaryOperator
		want string
	}{
		{ir.UnaryNegate, "Negate"},
		{ir.UnaryLogicalNot, "LogicalNot"},
		{ir.UnaryBitwiseNot, "BitwiseNot"},
		{ir.UnaryOperator(9), "UnaryOperator(9)"},
	}
	for _, tt := range tests {
		if got := unaryOpDebug(tt.op); got != tt.want {
			t.Errorf("unaryOpDebug(%d) = %q, want %q", tt.op, got, tt.want)
		}
	}
}

// TestLowerUnaryNegateIntegerMatrixIsRefused is context.rs:997-1023: a matrix
// with no float scalar cannot be negated by multiplying with -1, and the error
// names the operator and the scalar.
func TestLowerUnaryNegateIntegerMatrixIsRefused(t *testing.T) {
	fe, c := newTestContext(t)
	// GLSL has no integer matrix type; the IR does, so the error path is
	// reached directly.
	mat := fe.registerType(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: i32Scalar})
	sym := declareLocal(t, c, "m", mat)
	arena := &hirArena{}
	root := arena.add(hirUnary{Op: ir.UnaryNegate,
		Expr: arena.add(hirVariable{Sym: sym}, ir.Span{})}, ir.Span{})
	_, _, err := c.lowerExpect(arena, root, rhsPos)
	if err == nil || err.Message != "Cannot apply operator Negate to type Scalar { kind: Sint, width: 4 }" {
		t.Fatalf("error = %v, want Rust's operator/type message", err)
	}
}

// TestConstantU32Sources is GlobalCtx::get_const_val_from (proc/mod.rs:571-611)
// as the access lowering reads it: a literal, a zero value of an integer type,
// a constant whose initializer is a literal or a zero value, and the shapes it
// refuses.
func TestConstantU32Sources(t *testing.T) {
	fe, c := newTestContext(t)
	uintTy := fe.registerType(u32Scalar)
	floatTy := fe.registerType(f32Scalar)
	structTy := fe.registerType(ir.StructType{})

	// Module-scope constant initializers live in the global arena.
	addGlobalInit := func(kind ir.ExpressionKind) ir.ExpressionHandle {
		h := ir.ExpressionHandle(len(fe.mod.GlobalExpressions))
		fe.mod.GlobalExpressions = append(fe.mod.GlobalExpressions, ir.Expression{Kind: kind})
		return h
	}
	addConstant := func(ty ir.TypeHandle, init ir.ExpressionHandle) ir.ConstantHandle {
		h := ir.ConstantHandle(len(fe.mod.Constants))
		fe.mod.Constants = append(fe.mod.Constants, ir.Constant{Type: ty, Init: init})
		return h
	}
	literalInit := addGlobalInit(ir.Literal{Value: ir.LiteralU32(9)})
	zeroInit := addGlobalInit(ir.ExprZeroValue{Type: uintTy})
	floatZeroInit := addGlobalInit(ir.ExprZeroValue{Type: floatTy})
	runtimeInit := addGlobalInit(ir.ExprBinary{Op: ir.BinaryAdd})

	tests := []struct {
		name string
		expr ir.ExpressionHandle
		want uint32
		ok   bool
	}{
		{"literal", c.appendExpr(ir.Literal{Value: ir.LiteralU32(5)}), 5, true},
		{"negative literal", c.appendExpr(ir.Literal{Value: ir.LiteralI32(-1)}), 0, false},
		{"float literal", c.appendExpr(ir.Literal{Value: ir.LiteralF32(2)}), 0, false},
		{"integer zero value", c.appendExpr(ir.ExprZeroValue{Type: uintTy}), 0, true},
		{"float zero value", c.appendExpr(ir.ExprZeroValue{Type: floatTy}), 0, false},
		{"struct zero value", c.appendExpr(ir.ExprZeroValue{Type: structTy}), 0, false},
		{"constant literal init", c.appendExpr(ir.ExprConstant{Constant: addConstant(uintTy, literalInit)}), 9, true},
		{"constant zero init", c.appendExpr(ir.ExprConstant{Constant: addConstant(uintTy, zeroInit)}), 0, true},
		{"constant float zero", c.appendExpr(ir.ExprConstant{Constant: addConstant(floatTy, floatZeroInit)}), 0, false},
		{"constant runtime init", c.appendExpr(ir.ExprConstant{Constant: addConstant(uintTy, runtimeInit)}), 0, false},
		{"constant out of range", c.appendExpr(ir.ExprConstant{Constant: 99}), 0, false},
		{"runtime expression", c.appendExpr(ir.ExprBinary{Op: ir.BinaryAdd}), 0, false},
	}
	for _, tt := range tests {
		got, ok := c.constantU32(tt.expr)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("%s: constantU32 = %d, %t; want %d, %t", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// TestConstantU32GlobalOutOfRange is constantU32Global's bound check: a
// constant whose initializer handle is outside the global arena is refused
// instead of panicking.
func TestConstantU32GlobalOutOfRange(t *testing.T) {
	fe, c := newTestContext(t)
	uintTy := fe.registerType(u32Scalar)
	h := ir.ConstantHandle(len(fe.mod.Constants))
	fe.mod.Constants = append(fe.mod.Constants, ir.Constant{Type: uintTy, Init: 99})
	expr := c.appendExpr(ir.ExprConstant{Constant: h})
	if _, ok := c.constantU32(expr); ok {
		t.Fatal("a constant whose initializer is outside the global arena resolved")
	}
}

// TestConstLowerVariableResolvesConstant is context.rs:1024-1082: in a const
// context a constant name lowers through the module's ExprConstant, which the
// evaluator folds to the constant's value.
func TestConstLowerVariableResolvesConstant(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	c, err := newContext(fe, true)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	scalarTy := fe.registerType(i32Scalar)
	init := ir.ExpressionHandle(len(fe.mod.GlobalExpressions))
	fe.mod.GlobalExpressions = append(fe.mod.GlobalExpressions, ir.Expression{Kind: ir.Literal{Value: ir.LiteralI32(1)}})
	constant := ir.ConstantHandle(len(fe.mod.Constants))
	fe.mod.Constants = append(fe.mod.Constants, ir.Constant{Type: scalarTy, Init: init})

	sym := &glslSymbol{Kind: symGlobal, Name: "N"}
	ref := &variableRef{expr: c.appendExpr(ir.Literal{Value: ir.LiteralI32(1)}), entryArg: -1, constTy: scalarTy}
	ref.constant = &constant
	c.vars[sym] = ref

	arena := &hirArena{}
	root := arena.add(hirVariable{Sym: sym}, ir.Span{})
	h, _, lerr := c.lowerExpect(arena, root, rhsPos)
	if lerr != nil {
		t.Fatalf("lowerExpect: %v", lerr)
	}
	lit, ok := c.Expression(h).(ir.Literal)
	if !ok || lit.Value != ir.LiteralI32(1) {
		t.Fatalf("lowered to %#v, want the constant folded to its value", c.Expression(h))
	}
}

// TestConstLowerVariableRefusesOverride pins what a pipeline-overridable name
// does in a const context: the const evaluator rejects it with Rust's
// override-in-const error (constantErrorText renders it as
// "Unexpected override-expression").
func TestConstLowerVariableRefusesOverride(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	c, err := newContext(fe, true)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	sym := &glslSymbol{Kind: symGlobal, Name: "K"}
	c.vars[sym] = &variableRef{expr: c.appendExpr(ir.ExprOverride{Override: 3}), entryArg: -1}

	arena := &hirArena{}
	root := arena.add(hirVariable{Sym: sym}, ir.Span{})
	_, _, lerr := c.lowerExpect(arena, root, rhsPos)
	if lerr == nil || lerr.Message != "Unexpected override-expression" {
		t.Fatalf("error = %v, want Rust's override-in-const message", lerr)
	}
}

// TestLowerExpectRejectsVoid is context.rs:440-453: a call that yields no value
// cannot be used where a value is needed.
func TestLowerExpectRejectsVoid(t *testing.T) {
	fe, _ := newTestContext(t)
	// A declared void function with no parameters.
	decl := &functionDeclaration{}
	fe.functions["voidFn"] = decl
	decl.overloads = append(decl.overloads, &overload{kind: overloadCall, function: 0, void: true, defined: true})
	fe.mod.Functions = append(fe.mod.Functions, ir.Function{Name: "voidFn"})

	c, err := newContext(fe, false)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	arena := &hirArena{}
	root := arena.add(hirCall{Callee: callee{Kind: calleeFunction, Name: "voidFn"}}, ir.Span{})
	_, _, lerr := c.lowerExpect(arena, root, rhsPos)
	if lerr == nil || lerr.Message != "Expression returns void" {
		t.Fatalf("error = %v, want Rust's void-expression error", lerr)
	}
}
