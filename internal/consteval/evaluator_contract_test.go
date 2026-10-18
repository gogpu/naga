package consteval

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// These tests pin the contract of Rust naga's try_eval_and_append
// (proc/constant_evaluator.rs): the evaluator always appends, records the
// stage of what it appended, and a function body falls back to a run-time
// expression only for NotImplemented and InvalidBinaryOpArgs.

// A function body keeps an operation Rust does not evaluate as a run-time
// expression, appended unchanged; an expression using it is run-time too and
// is appended without an attempt to evaluate it.
func TestTryEvalFallbackIsAppendedAsRuntime(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	lo, hi, x := lit(t, e, ir.LiteralF32(0)), lit(t, e, ir.LiteralF32(1)), lit(t, e, ir.LiteralF32(0.5))
	k := ir.ExprMath{Fun: ir.MathSmoothStep, Arg: lo, Arg1: &hi, Arg2: &x}
	h, err := e.TryEvalAndAppend(k)
	if err != nil {
		t.Fatalf("runtime smoothstep: %v", err)
	}
	if got := (*exprs)[h].Kind; !reflect.DeepEqual(got, k) {
		t.Errorf("appended %#v, want the unevaluated %#v", got, k)
	}
	if e.IsConst(h) || e.IsConstOrOverride(h) {
		t.Error("a fallen-back expression is const")
	}
	sq, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathSqrt, Arg: h})
	if err != nil {
		t.Fatalf("sqrt of a run-time value: %v", err)
	}
	if _, ok := (*exprs)[sq].Kind.(ir.ExprMath); !ok || e.IsConst(sq) {
		t.Errorf("sqrt(runtime) = %T const=%v, want a run-time Math", (*exprs)[sq].Kind, e.IsConst(sq))
	}
}

// Operands that are run-time or override expressions are appended as they
// are, with the operand's stage.
func TestTryEvalNonConstOperandsAreAppended(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	*exprs = append(*exprs, ir.Expression{Kind: ir.ExprFunctionArgument{Index: 0}})
	one := lit(t, e, afloat(1))
	k := ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: one}
	h, err := e.TryEvalAndAppend(k)
	if err != nil || !reflect.DeepEqual((*exprs)[h].Kind, k) || e.IsConst(h) {
		t.Errorf("arg + 1: h=%d err=%v kind=%#v", h, err, (*exprs)[h].Kind)
	}

	*exprs = append(*exprs, ir.Expression{Kind: ir.ExprOverride{Override: 0}})
	ov := ir.ExpressionHandle(len(*exprs) - 1)
	h, err = e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: ov, Right: one})
	if err != nil || e.IsConst(h) || !e.IsConstOrOverride(h) {
		t.Errorf("override + 1 in a function: err=%v const=%v constOrOverride=%v", err, e.IsConst(h), e.IsConstOrOverride(h))
	}

	o, _, oexprs := newEval(BehaviorOverride)
	*oexprs = append(*oexprs, ir.Expression{Kind: ir.ExprOverride{Override: 0}})
	one = lit(t, o, afloat(1))
	h, err = o.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: one})
	if err != nil || !o.IsConstOrOverride(h) || o.IsConst(h) {
		t.Errorf("override + 1 in an override initializer: err=%v", err)
	}
}

// Rust's force_non_const: WGSL says an expression referring to a let-bound
// value is not a const-expression even when the value is constant.
func TestTryEvalForceRuntime(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	two := lit(t, e, ir.LiteralF32(2))
	if !e.IsConst(two) {
		t.Fatal("a literal is not const")
	}
	e.ForceRuntime(two)
	if e.IsConst(two) {
		t.Error("ForceRuntime left the literal const")
	}
	k := ir.ExprMath{Fun: ir.MathSqrt, Arg: two}
	h, err := e.TryEvalAndAppend(k)
	if err != nil || !reflect.DeepEqual((*exprs)[h].Kind, k) {
		t.Errorf("sqrt(let-bound) = %#v err=%v, want the unevaluated Math", (*exprs)[h].Kind, err)
	}
}

// Expressions appended by the caller without the evaluator get their stage
// from their kind: a Load is run-time, a Literal const.
func TestTryEvalStageOfCallerAppended(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	*exprs = append(*exprs,
		ir.Expression{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
		ir.Expression{Kind: ir.ExprLocalVariable{Variable: 0}},
		ir.Expression{Kind: ir.ExprLoad{Pointer: 1}},
	)
	if !e.IsConst(0) || e.IsConst(1) || e.IsConst(2) {
		t.Errorf("stages: literal %v, local %v, load %v", e.IsConst(0), e.IsConst(1), e.IsConst(2))
	}
}

// A reference to a module constant in a function body stays a Constant
// expression (Rust register_evaluated_expr); its value is copied only into
// expressions that embed it.
func TestTryEvalConstantReferenceInFunction(t *testing.T) {
	e, a, exprs := newEval(BehaviorRuntime)
	vec2f := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: tF32})
	e.Module.GlobalExpressions = []ir.Expression{
		{Kind: ir.Literal{Value: ir.LiteralF32(1)}},
		{Kind: ir.Literal{Value: ir.LiteralF32(2)}},
		{Kind: ir.ExprCompose{Type: vec2f, Components: []ir.ExpressionHandle{0, 1}}},
	}
	e.Module.Constants = []ir.Constant{{Name: "V", Type: vec2f, Init: 2}}
	c, err := e.TryEvalAndAppend(ir.ExprConstant{Constant: 0})
	if err != nil {
		t.Fatal(err)
	}
	if got := (*exprs)[c].Kind; !reflect.DeepEqual(got, ir.ExprConstant{Constant: 0}) {
		t.Errorf("V in a function = %#v, want Constant(0)", got)
	}
	if !e.IsConst(c) {
		t.Error("a constant reference is not const")
	}
	// Numeric operations copy each constant operand into the function first,
	// as Rust's check_and_get does: V twice (two lane literals and a Compose
	// each), then the result (two lanes and a Compose).
	before := len(*exprs)
	h, err := e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryMultiply, Left: c, Right: c})
	if err != nil {
		t.Fatal(err)
	}
	if got := valueAt(t, e, h); !reflect.DeepEqual(got, f32v(1, 4).Components()) {
		t.Errorf("V * V = %v", got)
	}
	if n := len(*exprs) - before; n != 9 {
		t.Errorf("V * V appended %d expressions, want 9 (two operand copies and the result, 3 each)", n)
	}
	// A swizzle embeds the components: they are copied into the function.
	sw, err := e.TryEvalAndAppend(ir.ExprSwizzle{Size: ir.Vec2, Vector: c, Pattern: [4]ir.SwizzleComponent{1, 0}})
	if err != nil {
		t.Fatal(err)
	}
	for _, comp := range (*exprs)[sw].Kind.(ir.ExprCompose).Components {
		if int(comp) >= len(*exprs) {
			t.Fatalf("swizzle component %d is not in the function arena", comp)
		}
		if _, ok := (*exprs)[comp].Kind.(ir.Literal); !ok {
			t.Errorf("swizzle component %d is %T, want a copied Literal", comp, (*exprs)[comp].Kind)
		}
	}
}

// Only a function body falls back: in an override initializer an operation
// Rust does not evaluate is an error, like Rust (has_runtime_restrictions).
func TestTryEvalNoFallbackOutsideFunctions(t *testing.T) {
	for _, b := range []Behavior{BehaviorConst, BehaviorOverride} {
		e, a, _ := newEval(b)
		m := mat2(t, e, a, [2]float32{1, 2}, [2]float32{3, 4})
		if _, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathTranspose, Arg: m}); !errors.Is(err, ErrNotImplemented) {
			t.Errorf("behavior %d: transpose(m) err=%v, want not implemented", b, err)
		}
		*e.Arena.(*testArena).exprs = append(*e.Arena.(*testArena).exprs, ir.Expression{Kind: ir.ExprFunctionArgument{Index: 0}})
		arg := ir.ExpressionHandle(len(*e.Arena.(*testArena).exprs) - 1)
		if _, err := e.TryEvalAndAppend(ir.ExprUnary{Op: ir.UnaryNegate, Expr: arg}); !errors.Is(err, ErrNotConst) {
			t.Errorf("behavior %d: -arg err=%v, want not a const-expression", b, err)
		}
	}
}

// Rust's binary_op has no I64/U64/F64 arithmetic, so a function body keeps
// it as a run-time expression; comparisons fold. A const-expression
// evaluates 64-bit arithmetic as the spec requires.
func TestTryEval64BitBinaryShape(t *testing.T) {
	r, _, rexprs := newEval(BehaviorRuntime)
	a, b := lit(t, r, ir.LiteralI64(5)), lit(t, r, ir.LiteralI64(7))
	add := ir.ExprBinary{Op: ir.BinaryAdd, Left: a, Right: b}
	h, err := r.TryEvalAndAppend(add)
	if err != nil || !reflect.DeepEqual((*rexprs)[h].Kind, add) {
		t.Errorf("runtime 5li + 7li = %#v err=%v, want the unevaluated Binary", (*rexprs)[h].Kind, err)
	}
	h, err = r.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryLess, Left: a, Right: b})
	if err != nil {
		t.Fatal(err)
	}
	if got := (*rexprs)[h].Kind; !reflect.DeepEqual(got, ir.Literal{Value: ir.LiteralBool(true)}) {
		t.Errorf("runtime 5li < 7li = %#v, want true", got)
	}

	c, _, _ := newEval(BehaviorConst)
	a, b = lit(t, c, ir.LiteralI64(5)), lit(t, c, ir.LiteralI64(7))
	h, err = c.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: a, Right: b})
	if err != nil {
		t.Fatal(err)
	}
	if got := valueAt(t, c, h); !reflect.DeepEqual(got, []ir.LiteralValue{ir.LiteralI64(12)}) {
		t.Errorf("const 5li + 7li = %v", got)
	}
}

// A domain error is a shader-creation error even in a function body.
func TestTryEvalDomainErrorInFunction(t *testing.T) {
	e, _, _ := newEval(BehaviorRuntime)
	neg := lit(t, e, ir.LiteralF32(-1))
	if _, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathSqrt, Arg: neg}); !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("runtime sqrt(-1): %v", err)
	}
}

// Better than Rust, never worse: where a function body keeps Rust's IR shape
// (an operation Rust does not evaluate), the operation is still checked, so
// the spec's shader-creation errors are reported. Rust compiles these.
func TestTryEvalRustShapedOperationsAreChecked(t *testing.T) {
	e, _, _ := newEval(BehaviorRuntime)
	one, zero := lit(t, e, ir.LiteralI64(1)), lit(t, e, ir.LiteralI64(0))
	if _, err := e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryDivide, Left: one, Right: zero}); !errors.Is(err, ErrDivisionByZero) {
		t.Errorf("runtime 1li / 0li: %v, want division by zero (Rust: run-time expression)", err)
	}
	f, n := lit(t, e, ir.LiteralF32(1)), lit(t, e, ir.LiteralI32(1000))
	if _, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathLdexp, Arg: f, Arg1: &n}); !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("runtime ldexp(1.0, 1000): %v, want out of domain (Rust: run-time expression)", err)
	}
	b := lit(t, e, ir.LiteralBool(true))
	if _, err := e.TryEvalAndAppend(ir.ExprRelational{Fun: ir.RelationalAll, Argument: b}); err != nil {
		t.Errorf("runtime all(true): %v", err)
	}
}

// Better than Rust: negating a constant matrix evaluates column-wise (Rust's
// unary_op accepts only vector Composes and rejects the shader with
// InvalidUnaryOpArg).
func TestTryEvalNegateMatrix(t *testing.T) {
	e, a, _ := newEval(BehaviorRuntime)
	m := mat2(t, e, a, [2]float32{1, 2}, [2]float32{3, 4})
	h := mustEval(t, e, ir.ExprUnary{Op: ir.UnaryNegate, Expr: m})
	cols, mt, ok := e.matrix(h)
	if !ok || mt.Columns != ir.Vec2 || !reflect.DeepEqual(cols[1].Components(), f32v(-3, -4).Components()) {
		t.Errorf("-m = %v %v (ok %v)", mt, cols, ok)
	}
}

// A malformed operand from the caller (a vector Compose whose components
// have different scalar types) is an error, never a panic.
func TestTryEvalMixedVectorIsAnError(t *testing.T) {
	e, a, exprs := newEval(BehaviorRuntime)
	vec2i := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: tI32})
	x, y := lit(t, e, ir.LiteralI32(1)), lit(t, e, ir.LiteralAbstractInt(2))
	*exprs = append(*exprs, ir.Expression{Kind: ir.ExprCompose{Type: vec2i, Components: []ir.ExpressionHandle{x, y}}})
	v := ir.ExpressionHandle(len(*exprs) - 1)
	if _, err := e.TryEvalAndAppend(ir.ExprUnary{Op: ir.UnaryNegate, Expr: v}); !errors.Is(err, ErrInvalidOperand) {
		t.Errorf("-vec2(1i, 2): err=%v, want invalid operand", err)
	}
}

// Checking an operation that Rust keeps unevaluated reports only the
// specification's value errors (overflow, domain, division, range). An
// operand form the kernel does not evaluate keeps Rust's run-time expression:
// the check never rejects a shader Rust accepts.
func TestTryEvalRustShapedCheckReportsOnlyValueErrors(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	x := lit(t, e, ir.LiteralF32(1)) // pack2x16float takes a vec2<f32>
	k := ir.ExprMath{Fun: ir.MathPack2x16float, Arg: x}
	h, err := e.TryEvalAndAppend(k)
	if err != nil || !reflect.DeepEqual((*exprs)[h].Kind, k) || e.IsConst(h) {
		t.Errorf("runtime pack2x16float(1.0) = %#v err=%v, want the unevaluated Math", (*exprs)[h].Kind, err)
	}
}

// Better than Rust: with abstract operands a built-in Rust does not
// evaluate is evaluated. Rust v30 keeps mix(1.0, 3.0, 0.5) as a run-time
// expression of abstract type and then rejects the shader ("failed to
// convert expression to a concrete type"); the spec makes it a
// const-expression. Rust's shape is kept only for concrete operands.
func TestTryEvalRustShapedWithAbstractOperandsEvaluates(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	a, b, f := lit(t, e, afloat(1)), lit(t, e, afloat(3)), lit(t, e, afloat(0.5))
	h, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathMix, Arg: a, Arg1: &b, Arg2: &f})
	if err != nil {
		t.Fatal(err)
	}
	if got := (*exprs)[h].Kind; !reflect.DeepEqual(got, ir.Literal{Value: afloat(2)}) {
		t.Errorf("runtime mix(1.0, 3.0, 0.5) = %#v, want the abstract literal 2.0", got)
	}
	ca, cb, cf := lit(t, e, ir.LiteralF32(1)), lit(t, e, ir.LiteralF32(3)), lit(t, e, ir.LiteralF32(0.5))
	k := ir.ExprMath{Fun: ir.MathMix, Arg: ca, Arg1: &cb, Arg2: &cf}
	if h, err = e.TryEvalAndAppend(k); err != nil || !reflect.DeepEqual((*exprs)[h].Kind, k) {
		t.Errorf("runtime mix(1f, 3f, 0.5f) = %#v err=%v, want Rust's run-time Math", (*exprs)[h].Kind, err)
	}
}
