package consteval

import (
	"github.com/gogpu/naga/ir"
)

// Behavior selects what the evaluator may leave unevaluated, like Rust
// naga's Behavior / WgslRestrictions.
type Behavior int

const (
	// BehaviorConst: a const-expression (module or function-local const).
	// Every expression must evaluate; anything else is an error.
	BehaviorConst Behavior = iota + 1
	// BehaviorRuntime: a function body. Expressions with constant operands
	// are evaluated; the rest, and operations Rust naga does not evaluate,
	// are left to the caller as run-time expressions.
	BehaviorRuntime
	// BehaviorOverride: an override initializer. Constant operands are
	// evaluated; override-expressions are left to the caller.
	BehaviorOverride
)

// Arena is the expression arena the evaluator reads operands from and
// appends results to: a function's expressions or Module.GlobalExpressions.
type Arena interface {
	Expression(h ir.ExpressionHandle) ir.ExpressionKind
	// Append adds an evaluated expression (Literal, Compose, Splat,
	// ZeroValue) and returns its handle.
	Append(kind ir.ExpressionKind) ir.ExpressionHandle
	// IsGlobal reports whether the arena is Module.GlobalExpressions.
	IsGlobal() bool
	// IsRuntime reports an expression that is not a const-expression even
	// though its operands are constant, such as a let-bound value.
	IsRuntime(h ir.ExpressionHandle) bool
}

// Types registers result types.
type Types interface {
	// Type returns the handle of the anonymous type inner, adding it if
	// the module has none.
	Type(inner ir.TypeInner) ir.TypeHandle
}

// Evaluator evaluates expressions over an arena, as Rust naga's
// ConstantEvaluator::try_eval_and_append does.
type Evaluator struct {
	Module   *ir.Module
	Arena    Arena
	Types    Types
	Behavior Behavior

	stages map[ir.ExpressionHandle]stage
}

// stage is the evaluation stage of an expression, Rust's ExpressionKind.
type stage int

const (
	stageConst stage = iota
	stageOverride
	stageRuntime
)

// Eval tries to evaluate kind.
//
//   - Evaluated: the result is appended to the arena (or, for a constant in
//     the module arena, is the constant's initializer) and ok is true.
//   - Left to the caller (ok false, err nil): an operand is an override or
//     run-time expression and the behavior allows that, or a function body
//     keeps an operation Rust naga does not evaluate. The caller appends
//     kind itself.
//   - Otherwise err says why the expression is not a valid const-expression.
func (e *Evaluator) Eval(kind ir.ExpressionKind) (h ir.ExpressionHandle, ok bool, err error) {
	switch e.stageOfKind(kind) {
	case stageRuntime:
		if e.Behavior == BehaviorConst {
			return 0, false, newError(ErrNotConst, "Unexpected runtime-expression")
		}
		return 0, false, nil
	case stageOverride:
		if e.Behavior == BehaviorConst {
			return 0, false, newError(ErrOverrideInConst, "Unexpected override-expression")
		}
		return 0, false, nil
	}
	h, err = e.evaluate(kind)
	if err != nil {
		if e.Behavior != BehaviorConst && FallsBack(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return h, true, nil
}

func (e *Evaluator) stageOf(h ir.ExpressionHandle) stage {
	if e.Arena.IsRuntime(h) {
		return stageRuntime
	}
	if s, ok := e.stages[h]; ok {
		return s
	}
	s := e.stageOfKind(e.Arena.Expression(h))
	if e.stages == nil {
		e.stages = map[ir.ExpressionHandle]stage{}
	}
	e.stages[h] = s
	return s
}

// stageOfKind is the latest stage among an expression's operands.
func (e *Evaluator) stageOfKind(kind ir.ExpressionKind) stage {
	switch kind.(type) {
	case ir.Literal, ir.ExprZeroValue, ir.ExprConstant:
		return stageConst
	case ir.ExprOverride:
		return stageOverride
	}
	ops, ok := operands(kind)
	if !ok {
		return stageRuntime
	}
	s := stageConst
	for _, op := range ops {
		s = max(s, e.stageOf(op))
	}
	return s
}

// operands lists the operand handles of the expressions the evaluator
// handles; ok is false for every other expression, which is run-time.
func operands(kind ir.ExpressionKind) (ops []ir.ExpressionHandle, ok bool) {
	switch k := kind.(type) {
	case ir.ExprCompose:
		return k.Components, true
	case ir.ExprSplat:
		return []ir.ExpressionHandle{k.Value}, true
	case ir.ExprSwizzle:
		return []ir.ExpressionHandle{k.Vector}, true
	case ir.ExprAccess:
		return []ir.ExpressionHandle{k.Base, k.Index}, true
	case ir.ExprAccessIndex:
		return []ir.ExpressionHandle{k.Base}, true
	case ir.ExprUnary:
		return []ir.ExpressionHandle{k.Expr}, true
	case ir.ExprBinary:
		return []ir.ExpressionHandle{k.Left, k.Right}, true
	case ir.ExprSelect:
		return []ir.ExpressionHandle{k.Condition, k.Accept, k.Reject}, true
	case ir.ExprAs:
		return []ir.ExpressionHandle{k.Expr}, true
	case ir.ExprRelational:
		return []ir.ExpressionHandle{k.Argument}, true
	case ir.ExprMath:
		return mathArgs(k), true
	}
	return nil, false
}

func mathArgs(m ir.ExprMath) []ir.ExpressionHandle {
	args := []ir.ExpressionHandle{m.Arg}
	for _, a := range []*ir.ExpressionHandle{m.Arg1, m.Arg2, m.Arg3} {
		if a != nil {
			args = append(args, *a)
		}
	}
	return args
}

func (e *Evaluator) evaluate(kind ir.ExpressionKind) (ir.ExpressionHandle, error) {
	switch k := kind.(type) {
	case ir.Literal:
		if !isFiniteLiteral(k.Value) {
			return 0, nonFiniteLiteral(k.Value)
		}
		return e.Arena.Append(k), nil
	case ir.ExprZeroValue:
		return e.Arena.Append(k), nil
	case ir.ExprConstant:
		return e.constant(k.Constant)
	case ir.ExprCompose:
		return e.compose(k)
	case ir.ExprSplat:
		v, err := e.checkAndGet(k.Value)
		if err != nil {
			return 0, err
		}
		return e.Arena.Append(ir.ExprSplat{Size: k.Size, Value: v}), nil
	case ir.ExprSwizzle:
		return e.swizzle(k)
	case ir.ExprAccessIndex:
		return e.access(k.Base, k.Index)
	case ir.ExprAccess:
		idx, err := e.indexOf(k.Index)
		if err != nil {
			return 0, err
		}
		return e.access(k.Base, idx)
	case ir.ExprUnary:
		return e.applyValue(func(v []Value) (Value, error) { return Unary(k.Op, v[0]) }, k.Expr)
	case ir.ExprBinary:
		return e.binary(k)
	case ir.ExprMath:
		return e.math(k)
	case ir.ExprAs:
		return e.as(k)
	case ir.ExprRelational:
		if e.Behavior == BehaviorRuntime && !RustEvaluatesRelational(k.Fun) {
			return 0, newError(ErrNotImplemented, "relational function")
		}
		return e.applyValue(func(v []Value) (Value, error) { return Relational(k.Fun, v[0]) }, k.Argument)
	case ir.ExprSelect:
		return e.applyValue(func(v []Value) (Value, error) { return Select(v[0], v[1], v[2]) }, k.Reject, k.Accept, k.Condition)
	}
	return 0, newError(ErrNotConst, "Unexpected runtime-expression")
}

func (e *Evaluator) math(m ir.ExprMath) (ir.ExpressionHandle, error) {
	if e.Behavior == BehaviorRuntime && !RustEvaluates(m.Fun) {
		return 0, newError(ErrNotImplemented, rustMathName(m.Fun)+" built-in function")
	}
	return e.applyValue(func(v []Value) (Value, error) { return Math(m.Fun, v...) }, mathArgs(m)...)
}

func (e *Evaluator) as(k ir.ExprAs) (ir.ExpressionHandle, error) {
	if k.Convert == nil {
		return 0, newError(ErrNotImplemented, "bitcast built-in function")
	}
	to := ir.ScalarType{Kind: k.Kind, Width: *k.Convert}
	if cols, mt, ok := e.matrix(k.Expr); ok {
		out := make([]Value, len(cols))
		for i, c := range cols {
			r, err := Cast(c, to)
			if err != nil {
				return 0, err
			}
			out[i] = r
		}
		mt.Scalar = to
		return e.appendMatrix(mt, out), nil
	}
	return e.applyValue(func(v []Value) (Value, error) { return Cast(v[0], to) }, k.Expr)
}

// applyValue reads the operands as scalar or vector values, applies f and
// appends the result.
func (e *Evaluator) applyValue(f func([]Value) (Value, error), ops ...ir.ExpressionHandle) (ir.ExpressionHandle, error) {
	vals := make([]Value, len(ops))
	for i, op := range ops {
		v, err := e.value(op)
		if err != nil {
			return 0, err
		}
		vals[i] = v
	}
	r, err := f(vals)
	if err != nil {
		return 0, err
	}
	return e.appendValue(r), nil
}

// appendValue appends a scalar as a Literal and a vector as a Compose of new
// Literals, as Rust's register_as_evaluated_expr does.
func (e *Evaluator) appendValue(v Value) ir.ExpressionHandle {
	if v.IsScalar() {
		return e.Arena.Append(ir.Literal{Value: v.comps[0]})
	}
	comps := make([]ir.ExpressionHandle, v.Len())
	for i, c := range v.comps {
		comps[i] = e.Arena.Append(ir.Literal{Value: c})
	}
	ty := e.Types.Type(ir.VectorType{Size: v.Size(), Scalar: v.Scalar()})
	return e.Arena.Append(ir.ExprCompose{Type: ty, Components: comps})
}

func isFiniteLiteral(v ir.LiteralValue) bool {
	switch v.(type) {
	case ir.LiteralF16, ir.LiteralF32, ir.LiteralF64, ir.LiteralAbstractFloat:
		_, ok := makeFloat(ScalarOf(v), floatOf(v))
		return ok
	}
	return true
}

func nonFiniteLiteral(v ir.LiteralValue) *Error {
	if f := floatOf(v); f != f { // NaN
		return newError(ErrOverflow, "Float literal is NaN")
	}
	return newError(ErrOverflow, "Float literal is infinite")
}
