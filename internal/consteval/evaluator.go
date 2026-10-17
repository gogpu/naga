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
	// are appended as run-time expressions.
	BehaviorRuntime
	// BehaviorOverride: an override initializer. Constant operands are
	// evaluated; override-expressions are appended as they are.
	BehaviorOverride
)

// Arena is the expression arena the evaluator reads operands from and
// appends to: a function's expressions or Module.GlobalExpressions.
type Arena interface {
	Expression(h ir.ExpressionHandle) ir.ExpressionKind
	// Append adds an expression and returns its handle. In a function
	// arena it keeps expressions that need no Emit (Literal, Constant,
	// ZeroValue, ...) out of the pending Emit range, as Rust's append_expr.
	Append(kind ir.ExpressionKind) ir.ExpressionHandle
	// IsGlobal reports whether the arena is Module.GlobalExpressions.
	IsGlobal() bool
}

// Types registers result types.
type Types interface {
	// Type returns the handle of the anonymous type inner, adding it if
	// the module has none.
	Type(inner ir.TypeInner) ir.TypeHandle
	// Array returns the handle of the array of base with the given size,
	// with the stride of base's layout.
	Array(base ir.TypeHandle, size ir.ArraySize) ir.TypeHandle
}

// Evaluator evaluates expressions over an arena, as Rust naga's
// ConstantEvaluator::try_eval_and_append does, and tracks the evaluation
// stage of every expression in the arena (Rust's ExpressionKindTracker).
// One Evaluator serves one arena for its whole lifetime.
type Evaluator struct {
	Module   *ir.Module
	Arena    Arena
	Types    Types
	Behavior Behavior

	// stages[h] is the stage of expression h plus one; 0 means not yet
	// known (appended by the caller), computed from the expression's kind.
	stages []stage
}

// stage is the evaluation stage of an expression, Rust's ExpressionKind.
type stage uint8

const (
	stageConst stage = iota + 1
	stageOverride
	stageRuntime
)

// TryEvalAndAppend appends kind to the arena, evaluated if it can be, and
// returns the handle of the result.
//
//   - Every operand a const-expression: kind is evaluated and its value
//     (Literal, Compose, Splat, ZeroValue, or for a constant in the module
//     arena the constant's initializer) is the result.
//   - An operand an override or run-time expression: kind is appended
//     unchanged when the behavior allows that, else it is an error.
//   - In a function body an operation Rust naga does not evaluate is
//     appended unchanged as a run-time expression (Rust's IR shape), after
//     it is checked: a value the specification makes a shader-creation
//     error is reported, which Rust does not do.
//
// The error says why kind is not a valid expression here.
func (e *Evaluator) TryEvalAndAppend(kind ir.ExpressionKind) (ir.ExpressionHandle, error) {
	switch e.stageOfKind(kind) {
	case stageOverride:
		if e.Behavior == BehaviorConst {
			return 0, newError(ErrOverrideInConst, "Unexpected override-expression")
		}
		return e.append(kind, stageOverride), nil
	case stageRuntime:
		if e.Behavior != BehaviorRuntime {
			return 0, newError(ErrNotConst, "Unexpected runtime-expression")
		}
		return e.append(kind, stageRuntime), nil
	}
	h, err := e.evaluate(kind)
	if err != nil {
		if e.Behavior == BehaviorRuntime && FallsBack(err) {
			return e.append(kind, stageRuntime), nil
		}
		return 0, err
	}
	return h, nil
}

// ForceRuntime marks h as a run-time expression although its operands are
// constant: WGSL says an expression referring to a let-bound value is not a
// const-expression (Rust force_non_const).
func (e *Evaluator) ForceRuntime(h ir.ExpressionHandle) {
	e.setStage(h, stageRuntime)
}

// Literal returns the value of the scalar const-expression h, looking
// through constants and zero values (Rust get_const_val). ok is false when h
// is not a const-expression or not a scalar.
func (e *Evaluator) Literal(h ir.ExpressionHandle) (lit ir.LiteralValue, ok bool) {
	if !e.IsConst(h) {
		return nil, false
	}
	v, err := e.value(h)
	if err != nil || !v.IsScalar() {
		return nil, false
	}
	return v.Component(0), true
}

// IsConst reports whether h is a const-expression.
func (e *Evaluator) IsConst(h ir.ExpressionHandle) bool { return e.stageOf(h) == stageConst }

// IsConstOrOverride reports whether h is a const- or override-expression.
func (e *Evaluator) IsConstOrOverride(h ir.ExpressionHandle) bool {
	return e.stageOf(h) != stageRuntime
}

func (e *Evaluator) append(kind ir.ExpressionKind, s stage) ir.ExpressionHandle {
	h := e.Arena.Append(kind)
	e.setStage(h, s)
	return h
}

func (e *Evaluator) setStage(h ir.ExpressionHandle, s stage) {
	if int(h) >= len(e.stages) {
		e.stages = append(e.stages, make([]stage, int(h)+1-len(e.stages))...)
	}
	e.stages[h] = s
}

func (e *Evaluator) stageOf(h ir.ExpressionHandle) stage {
	if int(h) < len(e.stages) && e.stages[h] != 0 {
		return e.stages[h]
	}
	s := e.stageOfKind(e.Arena.Expression(h))
	e.setStage(h, s)
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

// evaluate evaluates kind, whose operands are all const-expressions, and
// appends the value.
func (e *Evaluator) evaluate(kind ir.ExpressionKind) (ir.ExpressionHandle, error) {
	switch k := kind.(type) {
	case ir.Literal:
		if !isFiniteLiteral(k.Value) {
			return 0, nonFiniteLiteral(k.Value)
		}
		return e.append(k, stageConst), nil
	case ir.ExprZeroValue:
		return e.append(k, stageConst), nil
	case ir.ExprConstant:
		if int(k.Constant) >= len(e.Module.Constants) {
			return 0, newError(ErrInvalidOperand, "Unknown constant")
		}
		if e.Arena.IsGlobal() {
			// "See through" the constant (Rust try_eval_and_append_impl).
			return e.Module.Constants[k.Constant].Init, nil
		}
		return e.append(k, stageConst), nil
	case ir.ExprCompose:
		return e.compose(k)
	case ir.ExprSplat:
		v, err := e.checkAndGet(k.Value)
		if err != nil {
			return 0, err
		}
		return e.append(ir.ExprSplat{Size: k.Size, Value: v}, stageConst), nil
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
	case ir.ExprAs:
		if k.Convert == nil {
			return 0, newError(ErrNotImplemented, "bitcast built-in function")
		}
		return e.cast(k.Expr, ir.ScalarType{Kind: k.Kind, Width: *k.Convert})
	}
	return e.evaluateNumeric(kind)
}

// evaluateNumeric evaluates a numeric operation (unary, binary, select,
// relational or math) and appends its value; in runtime behavior an
// operation Rust naga keeps in the IR is only checked.
func (e *Evaluator) evaluateNumeric(kind ir.ExpressionKind) (ir.ExpressionHandle, error) {
	r, err := e.numeric(kind)
	if e.Behavior == BehaviorRuntime && e.rustKeeps(kind) {
		// Checked, but kept in Rust's IR shape. Only a value the
		// specification makes a shader-creation error is reported; any other
		// failure leaves the expression to validation, as Rust does.
		if err != nil && isValueError(err) {
			return 0, err
		}
		return 0, newError(ErrNotImplemented, "kept as a run-time expression, as Rust naga does")
	}
	if err != nil {
		return 0, err
	}
	return e.appendResult(r), nil
}

// result is the value of a numeric operation: a scalar or vector, or the
// columns of a matrix.
type result struct {
	v      Value
	cols   []Value
	matrix ir.MatrixType
	isMat  bool
}

// numeric computes a numeric operation without appending anything.
func (e *Evaluator) numeric(kind ir.ExpressionKind) (result, error) {
	switch k := kind.(type) {
	case ir.ExprUnary:
		if cols, mt, ok := e.matrix(k.Expr); ok {
			return e.mapColumns(mt, cols, mt.Scalar, func(_ int, c Value) (Value, error) { return Unary(k.Op, c) })
		}
		return e.scalarOp(func(v []Value) (Value, error) { return Unary(k.Op, v[0]) }, k.Expr)
	case ir.ExprBinary:
		return e.binary(k)
	case ir.ExprMath:
		if kernelNotImplemented(k.Fun) {
			return result{}, newError(ErrNotImplemented, rustMathName(k.Fun)+" built-in function")
		}
		return e.scalarOp(func(v []Value) (Value, error) { return Math(k.Fun, v...) }, mathArgs(k)...)
	case ir.ExprRelational:
		return e.scalarOp(func(v []Value) (Value, error) { return Relational(k.Fun, v[0]) }, k.Argument)
	case ir.ExprSelect:
		return e.scalarOp(func(v []Value) (Value, error) { return Select(v[0], v[1], v[2]) }, k.Reject, k.Accept, k.Condition)
	}
	return result{}, newError(ErrNotConst, "Unexpected runtime-expression")
}

// rustKeeps reports an operation that Rust naga leaves unevaluated in a
// function body although its operands are constant: the @const built-ins
// it does not implement, the relational functions other than all/any, and
// arithmetic on concrete 64-bit operands (its binary_op has none).
func (e *Evaluator) rustKeeps(kind ir.ExpressionKind) bool {
	// With an abstract operand Rust's run-time expression is of abstract
	// type, which Rust then fails to concretize: there is no Rust shape to
	// keep, and the spec makes the call a const-expression.
	ops, _ := operands(kind)
	for _, op := range ops {
		if s := e.operandScalar(op); s.Kind == ir.ScalarAbstractInt || s.Kind == ir.ScalarAbstractFloat {
			return false
		}
	}
	switch k := kind.(type) {
	case ir.ExprMath:
		return !RustEvaluates(k.Fun)
	case ir.ExprRelational:
		return !RustEvaluatesRelational(k.Fun)
	case ir.ExprBinary:
		return !isComparison(k.Op) && isConcrete64(e.operandScalar(k.Left))
	}
	return false
}

func isConcrete64(s ir.ScalarType) bool {
	return s.Width == 8 && (s.Kind == ir.ScalarSint || s.Kind == ir.ScalarUint || s.Kind == ir.ScalarFloat)
}

// operandScalar is the scalar type of a constant scalar, vector or matrix
// operand; the zero ScalarType for anything else.
func (e *Evaluator) operandScalar(h ir.ExpressionHandle) ir.ScalarType {
	if _, mt, ok := e.matrix(h); ok {
		return mt.Scalar
	}
	if v, err := e.value(h); err == nil {
		return v.Scalar()
	}
	return ir.ScalarType{}
}

// scalarOp reads the operands as scalar or vector values and applies f.
func (e *Evaluator) scalarOp(f func([]Value) (Value, error), ops ...ir.ExpressionHandle) (result, error) {
	vals := make([]Value, len(ops))
	for i, op := range ops {
		v, err := e.value(op)
		if err != nil {
			return result{}, err
		}
		vals[i] = v
	}
	r, err := f(vals)
	if err != nil {
		return result{}, err
	}
	return result{v: r}, nil
}

// mapColumns applies f to each column of a matrix; the result is a matrix
// of the same shape with scalar type to.
func (e *Evaluator) mapColumns(mt ir.MatrixType, cols []Value, to ir.ScalarType, f func(i int, c Value) (Value, error)) (result, error) {
	out := make([]Value, len(cols))
	for i, c := range cols {
		r, err := f(i, c)
		if err != nil {
			return result{}, err
		}
		out[i] = r
	}
	mt.Scalar = to
	return result{cols: out, matrix: mt, isMat: true}, nil
}

func (e *Evaluator) appendResult(r result) ir.ExpressionHandle {
	if r.isMat {
		return e.appendMatrix(r.matrix, r.cols)
	}
	return e.appendValue(r.v)
}

// appendValue appends a scalar as a Literal and a vector as a Compose of new
// Literals, as Rust's register_as_evaluated_expr does.
func (e *Evaluator) appendValue(v Value) ir.ExpressionHandle {
	if v.IsScalar() {
		return e.append(ir.Literal{Value: v.comps[0]}, stageConst)
	}
	comps := make([]ir.ExpressionHandle, v.Len())
	for i, c := range v.comps {
		comps[i] = e.append(ir.Literal{Value: c}, stageConst)
	}
	ty := e.Types.Type(ir.VectorType{Size: v.Size(), Scalar: v.Scalar()})
	return e.append(ir.ExprCompose{Type: ty, Components: comps}, stageConst)
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

func (e *Evaluator) appendConst(kind ir.ExpressionKind) ir.ExpressionHandle {
	return e.append(kind, stageConst)
}
