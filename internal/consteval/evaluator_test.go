package consteval

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/gogpu/naga/ir"
)

// testArena is an append-only expression arena with a deduplicating type
// table, standing in for the lowerer.
type testArena struct {
	mod    *ir.Module
	exprs  *[]ir.Expression
	global bool
}

func (a *testArena) Expression(h ir.ExpressionHandle) ir.ExpressionKind { return (*a.exprs)[h].Kind }
func (a *testArena) Append(k ir.ExpressionKind) ir.ExpressionHandle {
	*a.exprs = append(*a.exprs, ir.Expression{Kind: k})
	return ir.ExpressionHandle(len(*a.exprs) - 1)
}
func (a *testArena) IsGlobal() bool                     { return a.global }
func (a *testArena) Type(inner ir.TypeInner) ir.TypeHandle {
	for i, t := range a.mod.Types {
		if reflect.DeepEqual(t.Inner, inner) {
			return ir.TypeHandle(i)
		}
	}
	a.mod.Types = append(a.mod.Types, ir.Type{Inner: inner})
	return ir.TypeHandle(len(a.mod.Types) - 1)
}

func newEval(behavior Behavior) (*Evaluator, *testArena, *[]ir.Expression) {
	mod := &ir.Module{}
	var fn []ir.Expression
	a := &testArena{mod: mod, exprs: &fn}
	return &Evaluator{Module: mod, Arena: a, Types: a, Behavior: behavior}, a, &fn
}

func mustEval(t *testing.T, e *Evaluator, k ir.ExpressionKind) ir.ExpressionHandle {
	t.Helper()
	h, err := e.TryEvalAndAppend(k)
	if err != nil || !e.IsConst(h) {
		t.Fatalf("TryEvalAndAppend(%T): err=%v, want an evaluated expression", k, err)
	}
	return h
}

func lit(t *testing.T, e *Evaluator, v ir.LiteralValue) ir.ExpressionHandle {
	t.Helper()
	return mustEval(t, e, ir.Literal{Value: v})
}

// valueAt reads back an evaluated scalar or vector.
func valueAt(t *testing.T, e *Evaluator, h ir.ExpressionHandle) []ir.LiteralValue {
	t.Helper()
	v, err := e.value(h)
	if err != nil {
		t.Fatalf("value(%d): %v", h, err)
	}
	return v.Components()
}

func TestEvalBinaryOfLiterals(t *testing.T) {
	e, _, _ := newEval(BehaviorConst)
	a, b := lit(t, e, afloat(1)), lit(t, e, afloat(2))
	h := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryAdd, Left: a, Right: b})
	if got := valueAt(t, e, h); !reflect.DeepEqual(got, []ir.LiteralValue{afloat(3)}) {
		t.Errorf("1 + 2 = %v", got)
	}
}

func TestEvalVectorResultIsComposeOfLiterals(t *testing.T) {
	e, a, exprs := newEval(BehaviorConst)
	vec2f := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: tF32})
	x := mustEval(t, e, ir.ExprCompose{Type: vec2f, Components: []ir.ExpressionHandle{
		lit(t, e, ir.LiteralF32(1)), lit(t, e, ir.LiteralF32(2))}})
	y := mustEval(t, e, ir.ExprSplat{Size: ir.Vec2, Value: lit(t, e, ir.LiteralF32(3))})
	h := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryLess, Left: x, Right: y})
	c, ok := (*exprs)[h].Kind.(ir.ExprCompose)
	if !ok {
		t.Fatalf("result is %T, want a Compose", (*exprs)[h].Kind)
	}
	if want := (ir.VectorType{Size: ir.Vec2, Scalar: tBool}); !reflect.DeepEqual(e.Module.Types[c.Type].Inner, want) {
		t.Errorf("result type %v, want %v", e.Module.Types[c.Type].Inner, want)
	}
	if got := valueAt(t, e, h); !reflect.DeepEqual(got, []ir.LiteralValue{ir.LiteralBool(true), ir.LiteralBool(true)}) {
		t.Errorf("vec2(1,2) < splat(3) = %v", got)
	}
}

func TestEvalZeroValueOperand(t *testing.T) {
	e, a, _ := newEval(BehaviorConst)
	vec3i := a.Type(ir.VectorType{Size: ir.Vec3, Scalar: tI32})
	z := mustEval(t, e, ir.ExprZeroValue{Type: vec3i})
	h := mustEval(t, e, ir.ExprMath{Fun: ir.MathAbs, Arg: z})
	want := []ir.LiteralValue{ir.LiteralI32(0), ir.LiteralI32(0), ir.LiteralI32(0)}
	if got := valueAt(t, e, h); !reflect.DeepEqual(got, want) {
		t.Errorf("abs(vec3<i32>()) = %v", got)
	}
}

func TestEvalSwizzleAndAccess(t *testing.T) {
	e, a, exprs := newEval(BehaviorConst)
	vec3f := a.Type(ir.VectorType{Size: ir.Vec3, Scalar: tF32})
	v := mustEval(t, e, ir.ExprCompose{Type: vec3f, Components: []ir.ExpressionHandle{
		lit(t, e, ir.LiteralF32(1)), lit(t, e, ir.LiteralF32(2)), lit(t, e, ir.LiteralF32(3))}})
	sw := mustEval(t, e, ir.ExprSwizzle{Size: ir.Vec2, Vector: v, Pattern: [4]ir.SwizzleComponent{2, 0}})
	if got := valueAt(t, e, sw); !reflect.DeepEqual(got, []ir.LiteralValue{ir.LiteralF32(3), ir.LiteralF32(1)}) {
		t.Errorf("v.zx = %v", got)
	}
	// AccessIndex of a Compose returns the component handle itself (Rust).
	ai := mustEval(t, e, ir.ExprAccessIndex{Base: v, Index: 1})
	if c := (*exprs)[v].Kind.(ir.ExprCompose); ai != c.Components[1] {
		t.Errorf("v[1] = handle %d, want the component handle %d", ai, c.Components[1])
	}
	if _, err := e.TryEvalAndAppend(ir.ExprAccessIndex{Base: v, Index: 3}); err == nil {
		t.Error("v[3] of a vec3 evaluated")
	}
}

func TestEvalErrorsAndFallback(t *testing.T) {
	// Const: a domain error and a not-implemented function are errors.
	e, _, _ := newEval(BehaviorConst)
	neg := lit(t, e, afloat(-1))
	if _, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathSqrt, Arg: neg}); !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("const sqrt(-1): %v", err)
	}
	m := lit(t, e, ir.LiteralF32(1))
	if _, err := e.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathTranspose, Arg: m}); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("const transpose: %v", err)
	}

	// Runtime: the domain error stays an error; not-implemented and invalid
	// binary arguments leave the expression to the caller.
	r, _, _ := newEval(BehaviorRuntime)
	neg = lit(t, r, afloat(-1))
	if _, err := r.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathSqrt, Arg: neg}); !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("runtime sqrt(-1): %v", err)
	}
	m = lit(t, r, ir.LiteralF32(1))
	if h, err := r.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathTranspose, Arg: m}); err != nil || r.IsConst(h) {
		t.Errorf("runtime transpose: err=%v, want a run-time expression", err)
	}
	// A function body keeps the functions Rust does not evaluate, after
	// checking them: smoothstep with low == high is a shader-creation error
	// (Rust keeps it as a run-time expression).
	x := lit(t, r, afloat(0.5))
	if _, err := r.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathSmoothStep, Arg: x, Arg1: &x, Arg2: &x}); !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("runtime smoothstep(0.5, 0.5, 0.5): err=%v, want out of domain", err)
	}
	// ... which a const-expression evaluates.
	c, _, _ := newEval(BehaviorConst)
	lo, hi, mid := lit(t, c, afloat(0)), lit(t, c, afloat(1)), lit(t, c, afloat(0.5))
	if h, err := c.TryEvalAndAppend(ir.ExprMath{Fun: ir.MathSmoothStep, Arg: lo, Arg1: &hi, Arg2: &mid}); err != nil || !c.IsConst(h) {
		t.Errorf("const smoothstep: err=%v", err)
	}
}

func TestEvalNonConstOperands(t *testing.T) {
	e, _, exprs := newEval(BehaviorRuntime)
	*exprs = append(*exprs, ir.Expression{Kind: ir.ExprFunctionArgument{Index: 0}})
	arg := ir.ExpressionHandle(0)
	one := lit(t, e, afloat(1))
	if h, err := e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: arg, Right: one}); err != nil || e.IsConst(h) {
		t.Errorf("runtime arg + 1: err=%v, want a run-time expression", err)
	}
	c, _, cexprs := newEval(BehaviorConst)
	*cexprs = append(*cexprs, ir.Expression{Kind: ir.ExprFunctionArgument{Index: 0}})
	one = lit(t, c, afloat(1))
	if _, err := c.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: one}); !errors.Is(err, ErrNotConst) {
		t.Errorf("const arg + 1: %v", err)
	}
	*cexprs = append(*cexprs, ir.Expression{Kind: ir.ExprOverride{Override: 0}})
	ov := ir.ExpressionHandle(len(*cexprs) - 1)
	if _, err := c.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: ov, Right: one}); !errors.Is(err, ErrOverrideInConst) {
		t.Errorf("const override + 1: %v", err)
	}
	o, _, oexprs := newEval(BehaviorOverride)
	*oexprs = append(*oexprs, ir.Expression{Kind: ir.ExprOverride{Override: 0}})
	one = lit(t, o, afloat(1))
	if h, err := o.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryAdd, Left: 0, Right: one}); err != nil || o.IsConst(h) {
		t.Errorf("override + 1 in an override initializer: err=%v", err)
	}
}

func TestEvalLiteralMustBeFinite(t *testing.T) {
	e, _, _ := newEval(BehaviorConst)
	if _, err := e.TryEvalAndAppend(ir.Literal{Value: ir.LiteralF32(float32(posInf()))}); !errors.Is(err, ErrOverflow) {
		t.Errorf("infinite literal: %v", err)
	}
}

// Module constants: the module arena sees through to the constant's
// initializer (Rust check_and_get); a function arena copies the value.
func TestEvalConstantOperand(t *testing.T) {
	g, ga, _ := newEval(BehaviorConst)
	ga.global = true
	ga.exprs = &g.Module.GlobalExpressions // the module arena is GlobalExpressions
	init := lit(t, g, afloat(4))
	g.Module.Constants = append(g.Module.Constants, ir.Constant{Name: "K", Init: init})
	h := mustEval(t, g, ir.ExprConstant{Constant: 0})
	if h != init {
		t.Errorf("module arena: Constant evaluates to %d, want its init %d", h, init)
	}
	sq := mustEval(t, g, ir.ExprMath{Fun: ir.MathSqrt, Arg: mustEval(t, g, ir.ExprConstant{Constant: 0})})
	if got := valueAt(t, g, sq); !reflect.DeepEqual(got, []ir.LiteralValue{afloat(2)}) {
		t.Errorf("sqrt(K) = %v", got)
	}

	// Function arena: the constant's value is copied into the arena.
	f, _, fexprs := newEval(BehaviorRuntime)
	f.Module.GlobalExpressions = []ir.Expression{{Kind: ir.Literal{Value: ir.LiteralF32(9)}}}
	f.Module.Constants = []ir.Constant{{Name: "K", Init: 0}}
	*fexprs = append(*fexprs, ir.Expression{Kind: ir.ExprConstant{Constant: 0}})
	sq = mustEval(t, f, ir.ExprMath{Fun: ir.MathSqrt, Arg: 0})
	if got := valueAt(t, f, sq); !reflect.DeepEqual(got, []ir.LiteralValue{ir.LiteralF32(3)}) {
		t.Errorf("function sqrt(K) = %v", got)
	}
}

func posInf() float64 { return math.Inf(1) }

func mat2(t *testing.T, e *Evaluator, a *testArena, c0, c1 [2]float32) ir.ExpressionHandle {
	t.Helper()
	vec2f := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: tF32})
	col := func(c [2]float32) ir.ExpressionHandle {
		return mustEval(t, e, ir.ExprCompose{Type: vec2f, Components: []ir.ExpressionHandle{
			lit(t, e, ir.LiteralF32(c[0])), lit(t, e, ir.LiteralF32(c[1]))}})
	}
	m := a.Type(ir.MatrixType{Columns: ir.Vec2, Rows: ir.Vec2, Scalar: tF32})
	return mustEval(t, e, ir.ExprCompose{Type: m, Components: []ir.ExpressionHandle{col(c0), col(c1)}})
}

// Column-major: mat2x2(c0 = (1, 2), c1 = (3, 4)).
func TestEvalMatrixProducts(t *testing.T) {
	e, a, _ := newEval(BehaviorConst)
	m := mat2(t, e, a, [2]float32{1, 2}, [2]float32{3, 4})
	vec2f := a.Type(ir.VectorType{Size: ir.Vec2, Scalar: tF32})
	v := mustEval(t, e, ir.ExprCompose{Type: vec2f, Components: []ir.ExpressionHandle{
		lit(t, e, ir.LiteralF32(5)), lit(t, e, ir.LiteralF32(6))}})

	mv := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryMultiply, Left: m, Right: v})
	if got := valueAt(t, e, mv); !reflect.DeepEqual(got, f32v(23, 34).Components()) {
		t.Errorf("m * v = %v, want (23, 34)", got)
	}
	vm := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryMultiply, Left: v, Right: m})
	if got := valueAt(t, e, vm); !reflect.DeepEqual(got, f32v(17, 39).Components()) {
		t.Errorf("v * m = %v, want (17, 39)", got)
	}
	n := mat2(t, e, a, [2]float32{5, 6}, [2]float32{7, 8})
	mm := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryMultiply, Left: m, Right: n})
	cols, _, ok := e.matrix(mm)
	if !ok || !reflect.DeepEqual(cols[0].Components(), f32v(23, 34).Components()) ||
		!reflect.DeepEqual(cols[1].Components(), f32v(31, 46).Components()) {
		t.Errorf("m * n columns = %v", cols)
	}
	sum := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryAdd, Left: m, Right: n})
	if cols, _, _ := e.matrix(sum); !reflect.DeepEqual(cols[1].Components(), f32v(10, 12).Components()) {
		t.Errorf("(m + n)[1] = %v", cols[1].Components())
	}
	two := lit(t, e, ir.LiteralF32(2))
	scaled := mustEval(t, e, ir.ExprBinary{Op: ir.BinaryMultiply, Left: two, Right: m})
	if cols, _, _ := e.matrix(scaled); !reflect.DeepEqual(cols[0].Components(), f32v(2, 4).Components()) {
		t.Errorf("(2 * m)[0] = %v", cols[0].Components())
	}
	if _, err := e.TryEvalAndAppend(ir.ExprBinary{Op: ir.BinaryDivide, Left: m, Right: two}); !errors.Is(err, ErrInvalidBinaryArgs) {
		t.Errorf("m / 2: %v", err)
	}
	w := uint8(4)
	cast := mustEval(t, e, ir.ExprAs{Expr: m, Kind: ir.ScalarSint, Convert: &w})
	cols, mt, ok := e.matrix(cast)
	if !ok || mt.Scalar != tI32 || !reflect.DeepEqual(cols[1].Components(),
		[]ir.LiteralValue{ir.LiteralI32(3), ir.LiteralI32(4)}) {
		t.Errorf("mat2x2<i32>(m) = %v %v (ok %v)", mt, cols, ok)
	}
}

func TestEvalZeroValueAccess(t *testing.T) {
	e, a, exprs := newEval(BehaviorConst)
	f32h := a.Type(tF32)
	vec3u := a.Type(ir.VectorType{Size: ir.Vec3, Scalar: tU32})
	st := a.Type(ir.StructType{Members: []ir.StructMember{{Type: f32h}, {Type: vec3u}}})
	z := mustEval(t, e, ir.ExprZeroValue{Type: st})
	m := mustEval(t, e, ir.ExprAccessIndex{Base: z, Index: 1})
	if zv, ok := (*exprs)[m].Kind.(ir.ExprZeroValue); !ok || zv.Type != vec3u {
		t.Errorf("zero struct .1 = %v, want ZeroValue(vec3<u32>)", (*exprs)[m].Kind)
	}
	if _, err := e.TryEvalAndAppend(ir.ExprAccessIndex{Base: z, Index: 2}); err == nil {
		t.Error("zero struct .2 evaluated")
	}
}
