package front

// Contracts of the lowering context's pure helpers (front/glsl/context.go):
// the Emit range bookkeeping of Rust's proc::Emitter, the type resolution and
// the implicit conversion rules of context.rs:1411-1502.

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// TestEmitterRanges pins Rust's proc::Emitter: a range closes only when it is
// running and holds at least one expression.
func TestEmitterRanges(t *testing.T) {
	var e emitter
	if _, ok := e.finish(3); ok {
		t.Fatal("an emitter that never started reported a range")
	}
	e.start(3)
	if _, ok := e.finish(3); ok {
		t.Fatal("an empty range was reported")
	}
	e.start(4)
	got, ok := e.finish(7)
	if !ok || got != (ir.Range{Start: 4, End: 7}) {
		t.Fatalf("range = %+v, %t; want [4, 7)", got, ok)
	}
	if _, ok := e.finish(9); ok {
		t.Fatal("a second close reported the same range")
	}
}

// TestResolveTypeHandleRegistersValueTypes is types.rs:297-311: a value
// resolution with no handle registers an anonymous type in the module.
func TestResolveTypeHandleRegistersValueTypes(t *testing.T) {
	fe, c := newTestContext(t)
	lit := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(1)})
	h, err := c.resolveTypeHandle(lit, ir.Span{})
	if err != nil {
		t.Fatalf("resolveTypeHandle: %v", err)
	}
	if got := fe.mod.Types[h].Inner; got != f32Scalar {
		t.Fatalf("resolved type = %v, want the f32 scalar", got)
	}
	// A second resolution finds the registered type instead of adding another.
	again, err := c.resolveTypeHandle(lit, ir.Span{})
	if err != nil {
		t.Fatalf("resolveTypeHandle: %v", err)
	}
	if again != h {
		t.Fatalf("the same expression resolved to %d then %d", h, again)
	}
}

// TestInvalidateTypifierBothContexts is context.go:530-540: a const context
// resets its const typifier, a function context its own.
func TestInvalidateTypifierBothContexts(t *testing.T) {
	fe, c := newTestContext(t)
	lit := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(1)})
	if _, err := c.resolveType(lit, ir.Span{}); err != nil {
		t.Fatalf("resolveType: %v", err)
	}
	c.invalidateTypifier()
	if _, err := c.resolveType(lit, ir.Span{}); err != nil {
		t.Fatalf("resolveType after invalidate: %v", err)
	}

	constCtx, err := newContext(fe, true)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	h := constCtx.appendExpr(ir.Literal{Value: ir.LiteralF32(2)})
	if _, rerr := constCtx.resolveType(h, ir.Span{}); rerr != nil {
		t.Fatalf("const resolveType: %v", rerr)
	}
	constCtx.invalidateTypifier()
	if _, rerr := constCtx.resolveType(h, ir.Span{}); rerr != nil {
		t.Fatalf("const resolveType after invalidate: %v", rerr)
	}
}

// TestImplicitSplatRules is context.rs:1489-1502: only a scalar operand with a
// requested size is splatted.
func TestImplicitSplatRules(t *testing.T) {
	fe, c := newTestContext(t)
	scalar := mustExpr(t, c, ir.Literal{Value: ir.LiteralF32(1)})
	vectorSym := declareLocal(t, c, "v", fe.registerType(ir.VectorType{Size: ir.Vec2, Scalar: f32Scalar}))
	vector := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[vectorSym].expr})

	size := ir.Vec4
	if got, err := c.implicitSplat(scalar, ir.Span{}, nil); err != nil || got != scalar {
		t.Fatalf("splat with no size = %d, %v; want the scalar unchanged", got, err)
	}
	splatted, err := c.implicitSplat(scalar, ir.Span{}, &size)
	if err != nil {
		t.Fatalf("implicitSplat: %v", err)
	}
	splat, ok := c.Expression(splatted).(ir.ExprSplat)
	if !ok || splat.Size != ir.Vec4 {
		t.Fatalf("splat = %#v, want a vec4 splat", c.Expression(splatted))
	}
	if got, err := c.implicitSplat(vector, ir.Span{}, &size); err != nil || got != vector {
		t.Fatalf("a vector operand was splatted: %d, %v", got, err)
	}
}

// TestExprScalarComponentsAndPower is context.go:577-595: a type with no scalar
// component has no power, which is what makes the conversion rules skip it.
func TestExprScalarComponentsAndPower(t *testing.T) {
	fe, c := newTestContext(t)
	scalarSym := declareLocal(t, c, "s", fe.registerType(i32Scalar))
	scalar := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[scalarSym].expr})
	structSym := declareLocal(t, c, "st", fe.registerType(ir.StructType{}))
	structExpr := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[structSym].expr})

	got, ok, err := c.exprScalarComponents(scalar, ir.Span{})
	if err != nil || !ok || got != i32Scalar {
		t.Fatalf("exprScalarComponents = %v, %t, %v; want the i32 scalar", got, ok, err)
	}
	if _, ok, err := c.exprScalarComponents(structExpr, ir.Span{}); err != nil || ok {
		t.Fatalf("a struct reported a scalar component: %t, %v", ok, err)
	}
	power, ok, err := c.exprPower(scalar, ir.Span{})
	if err != nil || !ok || power != 0 {
		t.Fatalf("exprPower(int) = %d, %t, %v; want the sint power 0", power, ok, err)
	}
	if _, ok, err := c.exprPower(structExpr, ir.Span{}); err != nil || ok {
		t.Fatalf("a struct reported a conversion power: %t, %v", ok, err)
	}
}

// TestImplicitConversions is context.rs:1411-1459: conversion always converts,
// implicit_conversion only upwards, forced_conversion only when the type
// differs, and a target without a rank (a bool) never converts.
func TestImplicitConversions(t *testing.T) {
	fe, c := newTestContext(t)
	intSym := declareLocal(t, c, "i", fe.registerType(i32Scalar))
	intExpr := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[intSym].expr})
	floatSym := declareLocal(t, c, "f", fe.registerType(f32Scalar))
	floatExpr := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[floatSym].expr})

	converted, err := c.conversion(intExpr, ir.Span{}, f32Scalar)
	if err != nil {
		t.Fatalf("conversion: %v", err)
	}
	as, ok := c.Expression(converted).(ir.ExprAs)
	if !ok || as.Kind != ir.ScalarFloat || as.Convert == nil || *as.Convert != 4 {
		t.Fatalf("conversion = %#v, want As to f32", c.Expression(converted))
	}

	if got, err := c.implicitConversion(intExpr, ir.Span{}, f32Scalar); err != nil || got == intExpr {
		t.Fatalf("implicitConversion(int -> float) = %d, %v; want a conversion", got, err)
	}
	if got, err := c.implicitConversion(floatExpr, ir.Span{}, i32Scalar); err != nil || got != floatExpr {
		t.Fatalf("implicitConversion(float -> int) = %d, %v; want the operand unchanged (narrowing)", got, err)
	}
	if got, err := c.implicitConversion(floatExpr, ir.Span{}, boolScalar); err != nil || got != floatExpr {
		t.Fatalf("implicitConversion to bool = %d, %v; want no conversion (no rank)", got, err)
	}

	if got, err := c.forcedConversion(floatExpr, ir.Span{}, f32Scalar); err != nil || got != floatExpr {
		t.Fatalf("forcedConversion of the same type = %d, %v; want the operand", got, err)
	}
	if got, err := c.forcedConversion(intExpr, ir.Span{}, f32Scalar); err != nil || got == intExpr {
		t.Fatalf("forcedConversion(int -> float) = %d, %v; want a conversion", got, err)
	}
}

// TestBinaryImplicitConversionConvertsWeakerSide is context.rs:1461-1487,
// including the both-unranked and same-rank cases that change nothing.
func TestBinaryImplicitConversionConvertsWeakerSide(t *testing.T) {
	fe, c := newTestContext(t)
	intSym := declareLocal(t, c, "i", fe.registerType(i32Scalar))
	intExpr := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[intSym].expr})
	floatSym := declareLocal(t, c, "f", fe.registerType(f32Scalar))
	floatExpr := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[floatSym].expr})

	left, right, err := c.binaryImplicitConversion(intExpr, ir.Span{}, floatExpr, ir.Span{})
	if err != nil {
		t.Fatalf("binaryImplicitConversion: %v", err)
	}
	if left == intExpr {
		t.Fatal("the weaker left side was not converted")
	}
	if right != floatExpr {
		t.Fatalf("the float side changed to %d, want it untouched", right)
	}

	// Same rank on both sides: nothing converts.
	left, right, err = c.binaryImplicitConversion(floatExpr, ir.Span{}, floatExpr, ir.Span{})
	if err != nil || left != floatExpr || right != floatExpr {
		t.Fatalf("same-rank conversion = %d, %d, %v; want both untouched", left, right, err)
	}

	// Booleans have no rank: nothing converts.
	boolSym := declareLocal(t, c, "b", fe.registerType(boolScalar))
	boolExpr := mustExpr(t, c, ir.ExprLoad{Pointer: c.vars[boolSym].expr})
	left, right, err = c.binaryImplicitConversion(boolExpr, ir.Span{}, intExpr, ir.Span{})
	if err != nil || left != boolExpr || right != intExpr {
		t.Fatalf("unranked conversion = %d, %d, %v; want both untouched", left, right, err)
	}
}

// TestBlockSelectMemberLoads is context.go:294-303: every block member but a
// dynamically sized array is read through a load.
func TestBlockSelectMemberLoads(t *testing.T) {
	fe, c := newTestContext(t)
	dynamic := fe.registerType(ir.ArrayType{Base: fe.registerType(f32Scalar), Size: ir.ArraySize{}, Stride: 4})
	blockTy := fe.registerType(ir.StructType{Members: []ir.StructMember{
		{Name: "value", Type: fe.registerType(f32Scalar)},
		{Name: "data", Type: dynamic},
	}})
	handle := ir.GlobalVariableHandle(len(fe.mod.GlobalVariables))
	fe.mod.GlobalVariables = append(fe.mod.GlobalVariables, ir.GlobalVariable{Name: "b", Space: ir.SpaceStorage, Type: blockTy})

	if !c.blockSelectMemberLoads(handle, 0) {
		t.Fatal("a scalar block member does not load")
	}
	if c.blockSelectMemberLoads(handle, 1) {
		t.Fatal("a dynamically sized array member loads, want the array itself")
	}
	if !c.blockSelectMemberLoads(handle, 9) {
		t.Fatal("an out-of-range member index must load")
	}
	scalarHandle := ir.GlobalVariableHandle(len(fe.mod.GlobalVariables))
	fe.mod.GlobalVariables = append(fe.mod.GlobalVariables, ir.GlobalVariable{Name: "s", Type: fe.registerType(f32Scalar)})
	if !c.blockSelectMemberLoads(scalarHandle, 0) {
		t.Fatal("a non-struct global must load")
	}
}

// TestVariableUnknownName is context.go:715-734: a name that is neither
// declared nor a builtin is Rust's unknown-variable error.
func TestVariableUnknownName(t *testing.T) {
	_, c := newTestContext(t)
	_, err := c.variable(&glslSymbol{Kind: symLocal, Name: "missing"}, ir.Span{})
	if err == nil || err.Message != "Unknown variable: missing" {
		t.Fatalf("error = %v, want Rust's unknown-variable message", err)
	}
}

// TestAddGlobalExpressionInConstContext is context.go:283-292: a const context
// appends the reference directly, because the evaluator's const behavior would
// refuse a run-time global reference.
func TestAddGlobalExpressionInConstContext(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	c, err := newContext(fe, true)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	before := len(fe.mod.GlobalExpressions)
	h, aerr := c.addGlobalExpression(ir.ExprGlobalVariable{Variable: 0}, ir.Span{})
	if aerr != nil {
		t.Fatalf("addGlobalExpression: %v", aerr)
	}
	if int(h) != before {
		t.Fatalf("the expression landed at %d, want the const arena's %d", h, before)
	}
	if _, ok := fe.mod.GlobalExpressions[h].Kind.(ir.ExprGlobalVariable); !ok {
		t.Fatalf("the appended expression is %T, want the global reference", fe.mod.GlobalExpressions[h].Kind)
	}
}
