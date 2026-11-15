package front

import (
	"testing"

	"github.com/gogpu/naga/ir"
)

// scalars used across the lowering tests.
var (
	f32Scalar  = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
	i32Scalar  = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	u32Scalar  = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	boolScalar = ir.ScalarType{Kind: ir.ScalarBool, Width: 1}
)

// newTestContext returns a context over an empty module with no function
// built yet, and the module it writes to.
func newTestContext(t *testing.T) (*Frontend, *Context) {
	t.Helper()
	mod := &ir.Module{}
	fe := newFrontend(mod)
	c, err := newContext(fe, false)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	return fe, c
}

func TestTypePower(t *testing.T) {
	tests := []struct {
		scalar ir.ScalarType
		power  uint32
		ok     bool
	}{
		{i32Scalar, 0, true},
		{u32Scalar, 1, true},
		{f32Scalar, 2, true},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 8}, 3, true},
		{ir.ScalarType{Kind: ir.ScalarFloat, Width: 2}, 3, true},
		{boolScalar, 0, false},
	}
	for _, tt := range tests {
		power, ok := typePower(tt.scalar)
		if ok != tt.ok || (ok && power != tt.power) {
			t.Errorf("typePower(%v) = %d, %t; want %d, %t", tt.scalar, power, ok, tt.power, tt.ok)
		}
	}
}

func TestScalarComponents(t *testing.T) {
	tests := []struct {
		name  string
		inner ir.TypeInner
		want  ir.ScalarType
		ok    bool
	}{
		{"scalar", i32Scalar, i32Scalar, true},
		{"vector", ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar}, f32Scalar, true},
		{"matrix", ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec3, Scalar: f32Scalar}, f32Scalar, true},
		{"value pointer", ir.ValuePointerType{Scalar: u32Scalar}, u32Scalar, true},
		{"struct", ir.StructType{}, ir.ScalarType{}, false},
		{"array", ir.ArrayType{Base: 0}, ir.ScalarType{}, false},
	}
	for _, tt := range tests {
		got, ok := scalarComponents(tt.inner)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("%s: scalarComponents(%v) = %v, %t; want %v, %t", tt.name, tt.inner, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRegisterTypeDeduplicates(t *testing.T) {
	fe, _ := newTestContext(t)
	first := fe.registerType(f32Scalar)
	second := fe.registerType(f32Scalar)
	if first != second {
		t.Fatalf("registerType(f32) = %d then %d, want the same handle", first, second)
	}
	if len(fe.mod.Types) != 1 {
		t.Fatalf("module has %d types after registering f32 twice, want 1", len(fe.mod.Types))
	}
	// A named type with the same inner is a different type, as Rust's
	// UniqueArena keys on the whole Type{name, inner}.
	named := fe.registerNamedType("S", f32Scalar)
	if named == first {
		t.Fatal("a named type deduplicated with an anonymous one")
	}
}

// TestEmitterSkipsPreEmitExpressions is the Rust append_expr rule
// (constant_evaluator.rs:3640-3669): an expression that needs no Emit (a
// function argument, a literal) appended while a range is open closes it and
// opens a new one, so no Emit statement covers it.
func TestEmitterSkipsPreEmitExpressions(t *testing.T) {
	fe, c := newTestContext(t)
	ty := fe.registerType(f32Scalar)
	sym := &glslSymbol{Kind: symParam, Name: "a"}
	if err := c.addFunctionArg(sym, ty, paramOut, ir.Span{}); err != nil {
		t.Fatalf("addFunctionArg: %v", err)
	}
	// The argument is a pointer in an out parameter.
	arg, err := c.addExpression(ir.ExprLoad{Pointer: c.vars[sym].expr}, ir.Span{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	lit, err := c.addExpression(ir.Literal{Value: ir.LiteralF32(1)}, ir.Span{})
	if err != nil {
		t.Fatalf("literal: %v", err)
	}
	bin, err := c.addExpression(ir.ExprBinary{Op: ir.BinaryAdd, Left: arg, Right: lit}, ir.Span{})
	if err != nil {
		t.Fatalf("binary: %v", err)
	}
	c.emitEnd()

	// The literal sits between the load and the binary: the load's range
	// closes before it and the binary's range opens after it, so there are
	// two Emit statements and neither covers the literal.
	if len(c.body) != 2 {
		t.Fatalf("body has %d statements, want 2 Emits: %+v", len(c.body), c.body)
	}
	for i, want := range []ir.Range{{Start: arg, End: arg + 1}, {Start: bin, End: bin + 1}} {
		emit, ok := c.body[i].Kind.(ir.StmtEmit)
		if !ok {
			t.Fatalf("body[%d] = %T, want StmtEmit", i, c.body[i].Kind)
		}
		if emit.Range != want {
			t.Fatalf("Emit[%d] range = [%d, %d), want [%d, %d)", i, emit.Range.Start, emit.Range.End, want.Start, want.End)
		}
		if emit.Range.Start <= lit && lit < emit.Range.End {
			t.Fatalf("Emit[%d] range [%d, %d) covers the literal %d, which needs no Emit", i, emit.Range.Start, emit.Range.End, lit)
		}
	}
}

// TestAddExpressionConstantFolds checks that the evaluator folds constant
// operands, as Rust's ConstantEvaluator does for GLSL.
func TestAddExpressionConstantFolds(t *testing.T) {
	_, c := newTestContext(t)
	a, err := c.addExpression(ir.Literal{Value: ir.LiteralF32(1)}, ir.Span{})
	if err != nil {
		t.Fatalf("first literal: %v", err)
	}
	b, err := c.addExpression(ir.Literal{Value: ir.LiteralF32(2)}, ir.Span{})
	if err != nil {
		t.Fatalf("second literal: %v", err)
	}
	sum, err := c.addExpression(ir.ExprBinary{Op: ir.BinaryAdd, Left: a, Right: b}, ir.Span{})
	if err != nil {
		t.Fatalf("binary: %v", err)
	}
	lit, ok := c.Expression(sum).(ir.Literal)
	if !ok {
		t.Fatalf("constant sum became %T, want a folded Literal", c.Expression(sum))
	}
	if lit.Value != ir.LiteralF32(3) {
		t.Fatalf("folded value = %v, want 3", lit.Value)
	}
}

func TestAddExpressionConstContextRejectsRuntime(t *testing.T) {
	fe := newFrontend(&ir.Module{})
	c, err := newContext(fe, true)
	if err != nil {
		t.Fatalf("newContext: %v", err)
	}
	// A function argument cannot exist in a const context, and a runtime
	// expression there is Rust's "Unexpected runtime-expression".
	rt, addErr := c.addExpression(ir.ExprFunctionArgument{Index: 0}, ir.Span{})
	if addErr == nil {
		t.Fatalf("adding a function argument to a const context returned %d, want an error", rt)
	}
	if addErr.Message != "Unexpected runtime-expression" {
		t.Fatalf("error = %q, want Rust's %q", addErr.Message, "Unexpected runtime-expression")
	}
}

func TestImplicitConversionConvertsWeakerSide(t *testing.T) {
	fe, c := newTestContext(t)
	argTy := fe.registerType(i32Scalar)
	sym := &glslSymbol{Kind: symParam, Name: "a"}
	if err := c.addFunctionArg(sym, argTy, paramIn, ir.Span{}); err != nil {
		t.Fatalf("addFunctionArg: %v", err)
	}
	ref := c.vars[sym]
	if !ref.load {
		t.Fatal("an in parameter the body may write should be a writable local")
	}
	// A parameter the body may write is copied into a local; load it.
	arg, err := c.addExpression(ir.ExprLoad{Pointer: ref.expr}, ir.Span{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	lit, err := c.addExpression(ir.Literal{Value: ir.LiteralF32(0.5)}, ir.Span{})
	if err != nil {
		t.Fatalf("literal: %v", err)
	}
	left, right, convErr := c.binaryImplicitConversion(arg, ir.Span{}, lit, ir.Span{})
	if convErr != nil {
		t.Fatalf("binaryImplicitConversion: %v", convErr)
	}
	as, ok := c.Expression(left).(ir.ExprAs)
	if !ok {
		t.Fatalf("the i32 side became %T, want ExprAs", c.Expression(left))
	}
	if as.Kind != ir.ScalarFloat || as.Convert == nil || *as.Convert != 4 {
		t.Fatalf("conversion = %+v, want a conversion to f32", as)
	}
	if right != lit {
		t.Fatalf("the f32 side was converted to %d, want it unchanged (%d)", right, lit)
	}
}

func TestResolveTypeOfBinary(t *testing.T) {
	fe, c := newTestContext(t)
	sym := &glslSymbol{Kind: symParam, Name: "v"}
	ty := fe.registerType(ir.VectorType{Size: ir.Vec3, Scalar: f32Scalar})
	if err := c.addFunctionArg(sym, ty, paramIn, ir.Span{}); err != nil {
		t.Fatalf("addFunctionArg: %v", err)
	}
	v, err := c.addExpression(ir.ExprFunctionArgument{Index: 0}, ir.Span{})
	if err != nil {
		t.Fatalf("argument: %v", err)
	}
	sum, err := c.addExpression(ir.ExprBinary{Op: ir.BinaryAdd, Left: v, Right: v}, ir.Span{})
	if err != nil {
		t.Fatalf("binary: %v", err)
	}
	inner, rerr := c.resolveType(sum, ir.Span{})
	if rerr != nil {
		t.Fatalf("resolveType: %v", rerr)
	}
	vec, ok := inner.(ir.VectorType)
	if !ok || vec.Size != ir.Vec3 || vec.Scalar != f32Scalar {
		t.Fatalf("type = %v, want vec3<f32>", inner)
	}
}

func TestExprPosMaybeAccessBase(t *testing.T) {
	tests := []struct {
		pos      exprPos
		constant bool
		want     exprPos
	}{
		{lhsPos, true, lhsPos},
		{lhsPos, false, lhsPos},
		{rhsPos, true, accessBasePos(true)},
		{rhsPos, false, accessBasePos(false)},
		{accessBasePos(false), true, accessBasePos(false)},
		{accessBasePos(true), false, accessBasePos(false)},
		{accessBasePos(true), true, accessBasePos(true)},
	}
	for _, tt := range tests {
		if got := tt.pos.maybeAccessBase(tt.constant); got != tt.want {
			t.Errorf("%+v.maybeAccessBase(%t) = %+v, want %+v", tt.pos, tt.constant, got, tt.want)
		}
	}
}
