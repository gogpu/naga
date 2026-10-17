package lower

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gogpu/naga/internal/consteval"
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/wgsl/internal/parser"
)

// WGSL's automatic conversions for abstract types, ported from Rust naga's
// front/wgsl/lower/conversion.rs. A conversion never changes a value in
// place: it appends an As expression (or, for arrays, the evaluator's
// cast_array result) through the constant evaluator, so a constant operand
// is converted at compile time with the specification's range checks.

var (
	scalarI32 = ir.ScalarType{Kind: ir.ScalarSint, Width: 4}
	scalarU32 = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	scalarF32 = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

	scalarAbstractInt   = ir.ScalarType{Kind: ir.ScalarAbstractInt, Width: 8}
	scalarAbstractFloat = ir.ScalarType{Kind: ir.ScalarAbstractFloat, Width: 8}
)

func isAbstractScalar(s ir.ScalarType) bool {
	return s.Kind == ir.ScalarAbstractInt || s.Kind == ir.ScalarAbstractFloat
}

// automaticConversionCombine is the common type of a and b under WGSL's
// automatic conversions (Rust Scalar::automatic_conversion_combine).
func automaticConversionCombine(a, b ir.ScalarType) (ir.ScalarType, bool) {
	switch {
	case a.Kind == b.Kind:
		// Either no conversion is necessary, or none is possible: concrete
		// types never convert to concrete types.
		return a, a.Width == b.Width
	case a.Kind == ir.ScalarBool || b.Kind == ir.ScalarBool:
		return ir.ScalarType{}, false
	case a.Kind == ir.ScalarAbstractInt:
		// AbstractInt converts to AbstractFloat and every concrete number.
		return b, true
	case b.Kind == ir.ScalarAbstractInt:
		return a, true
	case a.Kind == ir.ScalarAbstractFloat && b.Kind == ir.ScalarFloat:
		return b, true
	case b.Kind == ir.ScalarAbstractFloat && a.Kind == ir.ScalarFloat:
		return a, true
	}
	// AbstractFloat and concrete integers, or two different concrete kinds.
	return ir.ScalarType{}, false
}

// automaticallyConvertsTo reports whether automatic conversions convert
// from to goal.
func automaticallyConvertsTo(from, goal ir.ScalarType) bool {
	c, ok := automaticConversionCombine(from, goal)
	return ok && c == goal
}

// concretizeScalar is the default concrete type of a scalar type.
func concretizeScalar(s ir.ScalarType) ir.ScalarType {
	switch s.Kind {
	case ir.ScalarAbstractInt:
		return scalarI32
	case ir.ScalarAbstractFloat:
		return scalarF32
	}
	return s
}

// exprResolution is the type of expression h of the current function.
func (l *Lowerer) exprResolution(h ir.ExpressionHandle) ir.TypeResolution {
	if int(h) < len(l.currentFunc.ExpressionTypes) {
		return l.currentFunc.ExpressionTypes[h]
	}
	return ir.TypeResolution{}
}

func (l *Lowerer) exprInner(h ir.ExpressionHandle) ir.TypeInner {
	return ir.TypeResInner(l.module, l.exprResolution(h))
}

// leafScalar is the scalar type of the leaves of a scalar, vector, matrix
// or array type (Rust automatically_convertible_scalar).
func (l *Lowerer) leafScalar(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.ArrayType:
		if int(t.Base) < len(l.module.Types) {
			return l.leafScalar(l.module.Types[t.Base].Inner)
		}
	}
	return ir.ScalarType{}, false
}

// isAbstractInner reports an abstract type (Rust TypeInner::is_abstract).
func (l *Lowerer) isAbstractInner(inner ir.TypeInner) bool {
	switch t := inner.(type) {
	case ir.AtomicType:
		return isAbstractScalar(t.Scalar)
	case ir.ArrayType, ir.ScalarType, ir.VectorType, ir.MatrixType:
		s, ok := l.leafScalar(t)
		return ok && isAbstractScalar(s)
	}
	return false
}

// innerConvertsTo is Rust's TypeInner::automatically_converts_to: the type
// constructors must match and the leaf scalar of from must convert to the
// leaf scalar of goal. It returns the two leaf scalars.
func (l *Lowerer) innerConvertsTo(from, goal ir.TypeInner) (ir.ScalarType, ir.ScalarType, bool) {
	var fs, gs ir.ScalarType
	switch f := from.(type) {
	case ir.ScalarType:
		g, ok := goal.(ir.ScalarType)
		if !ok {
			return fs, gs, false
		}
		fs, gs = f, g
	case ir.VectorType:
		g, ok := goal.(ir.VectorType)
		if !ok || g.Size != f.Size {
			return fs, gs, false
		}
		fs, gs = f.Scalar, g.Scalar
	case ir.MatrixType:
		g, ok := goal.(ir.MatrixType)
		if !ok || g.Columns != f.Columns || g.Rows != f.Rows {
			return fs, gs, false
		}
		fs, gs = f.Scalar, g.Scalar
	case ir.ArrayType:
		g, ok := goal.(ir.ArrayType)
		if !ok || !reflect.DeepEqual(f.Size, g.Size) || int(f.Base) >= len(l.module.Types) || int(g.Base) >= len(l.module.Types) {
			return fs, gs, false
		}
		return l.innerConvertsTo(l.module.Types[f.Base].Inner, l.module.Types[g.Base].Inner)
	default:
		return fs, gs, false
	}
	switch {
	case fs.Kind == ir.ScalarAbstractFloat && gs.Kind == ir.ScalarFloat,
		fs.Kind == ir.ScalarAbstractInt && (gs.Kind == ir.ScalarSint || gs.Kind == ir.ScalarUint ||
			gs.Kind == ir.ScalarAbstractFloat || gs.Kind == ir.ScalarFloat):
		return fs, gs, true
	}
	return fs, gs, false
}

// conversionConsensus is the best scalar type every component converts to
// (Rust automatic_conversion_consensus). base, if set, must be compatible
// too. ok is false if there is none; bad is then the index of a component
// that cannot be reconciled.
func (l *Lowerer) conversionConsensus(base *ir.ScalarType, components []ir.ExpressionHandle) (best ir.ScalarType, bad int, ok bool) {
	rest := components
	if base != nil {
		best = *base
	} else {
		if len(components) == 0 {
			return best, 0, false
		}
		s, has := l.leafScalarOfExpr(components[0])
		if !has {
			return best, 0, false
		}
		best, rest = s, components[1:]
	}
	offset := len(components) - len(rest)
	for i, c := range rest {
		s, has := l.leafScalarOfExpr(c)
		if !has {
			return best, offset + i, false
		}
		combined, can := automaticConversionCombine(best, s)
		if !can {
			return best, offset + i, false
		}
		best = combined
	}
	return best, 0, true
}

// leafScalarOfExpr is the leaf scalar of a scalar, vector or matrix
// expression (Rust TypeInner::scalar, used by the consensus).
func (l *Lowerer) leafScalarOfExpr(h ir.ExpressionHandle) (ir.ScalarType, bool) {
	switch t := l.exprInner(h).(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	}
	return ir.ScalarType{}, false
}

// convertToLeafScalar converts h to the leaf scalar type goal with an As
// expression, unless it already has it (Rust convert_to_leaf_scalar). The
// caller decides that the conversion is justified.
func (l *Lowerer) convertToLeafScalar(h ir.ExpressionHandle, goal ir.ScalarType) (ir.ExpressionHandle, error) {
	if s, ok := l.leafScalarOfExpr(h); ok && s == goal {
		return h, nil
	}
	return l.convertLeafScalar(h, goal)
}

// convertSliceToCommonLeafScalar converts every expression to goal (Rust
// convert_slice_to_common_leaf_scalar).
func (l *Lowerer) convertSliceToCommonLeafScalar(hs []ir.ExpressionHandle, goal ir.ScalarType) error {
	for i, h := range hs {
		c, err := l.convertToLeafScalar(h, goal)
		if err != nil {
			return err
		}
		hs[i] = c
	}
	return nil
}

// convertLeafScalar converts the leaves of h to goal: arrays through the
// evaluator's CastArray, everything else with an As expression (Rust
// convert_leaf_scalar).
func (l *Lowerer) convertLeafScalar(h ir.ExpressionHandle, goal ir.ScalarType) (ir.ExpressionHandle, error) {
	if _, ok := l.exprInner(h).(ir.ArrayType); ok {
		return l.eval().CastArray(h, goal)
	}
	width := goal.Width
	return l.addExpression(ir.Expression{Kind: ir.ExprAs{Expr: h, Kind: goal.Kind, Convert: &width}})
}

// tryAutomaticConversions converts h to the type goal with WGSL's automatic
// conversions (Rust try_automatic_conversions): h is returned unchanged if
// it already has that type; a concrete h of another type is a type
// mismatch, an abstract one that does not convert an automatic-conversion
// error.
func (l *Lowerer) tryAutomaticConversions(h ir.ExpressionHandle, goal ir.TypeInner) (ir.ExpressionHandle, error) {
	from := l.exprInner(h)
	if reflect.DeepEqual(from, goal) {
		return h, nil
	}
	if !l.isAbstractInner(from) {
		return 0, &conversionError{mismatch: true, dest: l.wgslType(goal), source: l.wgslType(from)}
	}
	_, gs, ok := l.innerConvertsTo(from, goal)
	if !ok {
		return 0, &conversionError{dest: l.wgslType(goal), source: l.wgslType(from)}
	}
	return l.convertLeafScalar(h, gs)
}

// tryAutomaticConversionForLeafScalar converts the leaves of h to goal with
// automatic conversions (Rust try_automatic_conversion_for_leaf_scalar).
func (l *Lowerer) tryAutomaticConversionForLeafScalar(h ir.ExpressionHandle, goal ir.ScalarType) (ir.ExpressionHandle, error) {
	from := l.exprInner(h)
	s, ok := l.leafScalar(from)
	if ok && s == goal {
		return h, nil
	}
	if !ok || !automaticallyConvertsTo(s, goal) {
		return 0, fmt.Errorf("automatic conversions cannot convert elements of `%s` to `%s`", l.wgslType(from), wgslScalar(goal))
	}
	return l.convertLeafScalar(h, goal)
}

// tryAutomaticConversionsForVector converts the arguments of a vector
// constructor with an explicit element type: scalars to goal, vectors to
// vectors of goal (Rust try_automatic_conversions_for_vector).
func (l *Lowerer) tryAutomaticConversionsForVector(hs []ir.ExpressionHandle, goal ir.ScalarType) error {
	for i, h := range hs {
		var to ir.TypeInner
		switch t := l.exprInner(h).(type) {
		case ir.ScalarType:
			to = goal
		case ir.VectorType:
			to = ir.VectorType{Size: t.Size, Scalar: goal}
		default:
			return fmt.Errorf("invalid type for constructor component at index [%d]", i)
		}
		c, err := l.tryAutomaticConversions(h, to)
		if err != nil {
			return err
		}
		hs[i] = c
	}
	return nil
}

// tryAutomaticConversionsSlice converts every expression to goal (Rust
// try_automatic_conversions_slice).
func (l *Lowerer) tryAutomaticConversionsSlice(hs []ir.ExpressionHandle, goal ir.TypeInner) error {
	for i, h := range hs {
		c, err := l.tryAutomaticConversions(h, goal)
		if err != nil {
			return err
		}
		hs[i] = c
	}
	return nil
}

// concretize converts an abstract h to its concrete type (Rust
// concretize): an AbstractInt value becomes i32, or u32 or f32 if i32
// cannot hold it; an AbstractFloat value becomes f32. A concrete h is
// returned unchanged.
func (l *Lowerer) concretize(h ir.ExpressionHandle) (ir.ExpressionHandle, error) {
	inner := l.exprInner(h)
	s, ok := l.leafScalar(inner)
	if !ok || !isAbstractScalar(s) {
		return h, nil
	}
	prefs := []ir.ScalarType{scalarF32}
	if s.Kind == ir.ScalarAbstractInt {
		prefs = []ir.ScalarType{scalarI32, scalarU32, scalarF32}
	}
	var reasons []string
	for _, to := range prefs {
		c, err := l.eval().CastArray(h, to)
		if err == nil {
			return c, nil
		}
		reasons = append(reasons, fmt.Sprintf("the expression couldn't be converted to have %s scalar type: %v", wgslScalar(to), err))
	}
	return 0, fmt.Errorf("failed to convert expression to a concrete type: this expression has type %s; %s",
		l.wgslType(inner), strings.Join(reasons, "; "))
}

// wgslScalar is the WGSL name of a scalar type, with Rust's names for the
// abstract types.
func wgslScalar(s ir.ScalarType) string {
	switch s.Kind {
	case ir.ScalarBool:
		return "bool"
	case ir.ScalarAbstractInt:
		return "{AbstractInt}"
	case ir.ScalarAbstractFloat:
		return "{AbstractFloat}"
	case ir.ScalarSint:
		return fmt.Sprintf("i%d", int(s.Width)*8)
	case ir.ScalarUint:
		return fmt.Sprintf("u%d", int(s.Width)*8)
	case ir.ScalarFloat:
		return fmt.Sprintf("f%d", int(s.Width)*8)
	}
	return fmt.Sprintf("{scalar %v}", s)
}

// wgslType formats a type as WGSL for diagnostics, like Rust's
// TypeContext::write_type_inner.
func (l *Lowerer) wgslType(inner ir.TypeInner) string {
	switch t := inner.(type) {
	case ir.ScalarType:
		return wgslScalar(t)
	case ir.VectorType:
		return fmt.Sprintf("vec%d<%s>", t.Size, wgslScalar(t.Scalar))
	case ir.MatrixType:
		return fmt.Sprintf("mat%dx%d<%s>", t.Columns, t.Rows, wgslScalar(t.Scalar))
	case ir.AtomicType:
		return "atomic<" + wgslScalar(t.Scalar) + ">"
	case ir.ArrayType:
		base := l.wgslTypeHandle(t.Base)
		if t.Size.Constant != nil {
			return fmt.Sprintf("array<%s, %d>", base, *t.Size.Constant)
		}
		return "array<" + base + ">"
	case ir.PointerType:
		// Rust address_space_str; the storage access mode is not part of our
		// pointer type, so it is not printed.
		spaces := [...]string{
			ir.SpaceFunction: "function", ir.SpacePrivate: "private", ir.SpaceWorkGroup: "workgroup",
			ir.SpaceUniform: "uniform", ir.SpaceStorage: "storage", ir.SpacePushConstant: "push_constant",
			ir.SpaceImmediate: "immediate", ir.SpaceTaskPayload: "task_payload",
		}
		if int(t.Space) < len(spaces) && spaces[t.Space] != "" {
			return "ptr<" + spaces[t.Space] + ", " + l.wgslTypeHandle(t.Base) + ">"
		}
		return "ptr<" + l.wgslTypeHandle(t.Base) + ">"
	case ir.StructType:
		return "{unnamed struct}"
	case nil:
		return "{unknown type}"
	}
	return fmt.Sprintf("{non-WGSL Naga type %T}", inner)
}

func (l *Lowerer) wgslTypeHandle(th ir.TypeHandle) string {
	if int(th) >= len(l.module.Types) {
		return "{unknown type}"
	}
	t := l.module.Types[th]
	if _, isStruct := t.Inner.(ir.StructType); isStruct && t.Name != "" {
		return t.Name
	}
	return l.wgslType(t.Inner)
}

// automaticallyConvertibleScalar is the scalar an automatic conversion to
// inner targets (Rust TypeInner::automatically_convertible_scalar).
func (l *Lowerer) automaticallyConvertibleScalar(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t, true
	case ir.VectorType:
		return t.Scalar, true
	case ir.MatrixType:
		return t.Scalar, true
	case ir.ArrayType:
		if int(t.Base) < len(l.module.Types) {
			return l.automaticallyConvertibleScalar(l.module.Types[t.Base].Inner)
		}
	}
	return ir.ScalarType{}, false
}

// pointerConvertibleScalar is the scalar a value stored through a pointer of
// type inner converts to (Rust pointer_automatically_convertible_scalar).
func (l *Lowerer) pointerConvertibleScalar(inner ir.TypeInner) (ir.ScalarType, bool) {
	switch t := inner.(type) {
	case ir.AtomicType:
		return ir.ScalarType{}, false
	case ir.PointerType:
		if int(t.Base) < len(l.module.Types) {
			return l.automaticallyConvertibleScalar(l.module.Types[t.Base].Inner)
		}
		return ir.ScalarType{}, false
	case ir.ValuePointerType:
		return t.Scalar, true
	}
	return l.automaticallyConvertibleScalar(inner)
}

// concretizeInto replaces *h by its concretized value (Rust expression() for
// an already lowered argument).
func (l *Lowerer) concretizeInto(h *ir.ExpressionHandle) error {
	c, err := l.concretize(*h)
	if err != nil {
		return err
	}
	*h = c
	return nil
}

// convertInto replaces *h by its conversion to the leaf scalar goal (Rust
// expression_with_leaf_scalar).
func (l *Lowerer) convertInto(h *ir.ExpressionHandle, goal ir.ScalarType) error {
	c, err := l.tryAutomaticConversionForLeafScalar(*h, goal)
	if err != nil {
		return err
	}
	*h = c
	return nil
}

// concretizeFloatInto converts an abstract *h to f32, for parameters that
// are floats only and have no abstract overload (modf, frexp, a ray hit
// distance); a concrete value is left to type checking.
func (l *Lowerer) concretizeFloatInto(h *ir.ExpressionHandle) error {
	if s, ok := l.leafScalarOfExpr(*h); ok && isAbstractScalar(s) {
		return l.convertInto(h, scalarF32)
	}
	return nil
}

// convertAtomicValue converts *h to the scalar of the atomic pointer points
// to (Rust expression_with_leaf_scalar with atomic_pointer's scalar).
func (l *Lowerer) convertAtomicValue(pointer ir.ExpressionHandle, h *ir.ExpressionHandle) error {
	s, ok := l.atomicPointerScalar(pointer)
	if !ok {
		return nil
	}
	return l.convertInto(h, s)
}

// atomicPointerScalar is the scalar of the atomic a pointer points to (Rust
// atomic_pointer).
func (l *Lowerer) atomicPointerScalar(pointer ir.ExpressionHandle) (ir.ScalarType, bool) {
	p, ok := l.exprInner(pointer).(ir.PointerType)
	if !ok || int(p.Base) >= len(l.module.Types) {
		return ir.ScalarType{}, false
	}
	a, ok := l.module.Types[p.Base].Inner.(ir.AtomicType)
	return a.Scalar, ok
}

// lowerAssignValue lowers the value stored by `pointer op= rhs` (Rust
// StatementKind::Assign): for a compound assignment the current value is
// loaded first, so that the left side is evaluated before the right; the
// right side converts to the scalar of the memory it is stored to (u32 for a
// shift amount); a compound assignment splats a scalar to the vector side
// and appends the binary operation.
func (l *Lowerer) lowerAssignValue(pointer ir.ExpressionHandle, op parser.TokenKind, rhs parser.Expr, increment bool, target *[]ir.Statement) (ir.ExpressionHandle, error) {
	if increment {
		return l.lowerIncrementValue(pointer, op)
	}
	compound := op != parser.TokenEqual
	var binOp ir.BinaryOperator
	goal, hasGoal := l.pointerConvertibleScalar(l.exprInner(pointer))
	var loaded ir.ExpressionHandle
	if compound {
		binOp = l.assignOpToBinary(op)
		if binOp == ir.BinaryShiftLeft || binOp == ir.BinaryShiftRight {
			goal, hasGoal = ir.ScalarType{Kind: ir.ScalarUint, Width: 4}, true
		}
		var err error
		if loaded, err = l.applyLoadRule(pointer); err != nil {
			return 0, err
		}
	}
	value, err := l.lowerExpression(rhs, target)
	if err != nil {
		return 0, err
	}
	if hasGoal {
		if value, err = l.tryAutomaticConversionForLeafScalar(value, goal); err != nil {
			return 0, err
		}
	}
	if !compound {
		return value, nil
	}
	if loaded, value, err = l.binaryOpSplat(binOp, loaded, value); err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{Kind: ir.ExprBinary{Op: binOp, Left: loaded, Right: value}})
}

// lowerIncrementValue lowers the value stored by `x++` / `x--` (Rust
// StatementKind::Increment/Decrement): the reference must be to an i32 or u32
// (or a 64-bit integer); the 1 has that type and is appended before the load.
func (l *Lowerer) lowerIncrementValue(pointer ir.ExpressionHandle, op parser.TokenKind) (ir.ExpressionHandle, error) {
	var scalar ir.ScalarType
	switch t := l.exprInner(pointer).(type) {
	case ir.ValuePointerType:
		if t.Size == nil {
			scalar = t.Scalar
		}
	case ir.PointerType:
		if int(t.Base) < len(l.module.Types) {
			scalar, _ = l.module.Types[t.Base].Inner.(ir.ScalarType)
		}
	}
	var one ir.LiteralValue
	switch scalar {
	case ir.ScalarType{Kind: ir.ScalarSint, Width: 4}:
		one = ir.LiteralI32(1)
	case ir.ScalarType{Kind: ir.ScalarUint, Width: 4}:
		one = ir.LiteralU32(1)
	case ir.ScalarType{Kind: ir.ScalarSint, Width: 8}:
		one = ir.LiteralI64(1)
	case ir.ScalarType{Kind: ir.ScalarUint, Width: 8}:
		one = ir.LiteralU64(1)
	default:
		return 0, errors.New("increment/decrement operation requires reference type to be one of i32 or u32")
	}
	right, err := l.interruptEmitter(ir.Expression{Kind: ir.Literal{Value: one}})
	if err != nil {
		return 0, err
	}
	left, err := l.applyLoadRule(pointer)
	if err != nil {
		return 0, err
	}
	return l.addExpression(ir.Expression{Kind: ir.ExprBinary{Op: l.assignOpToBinary(op), Left: left, Right: right}})
}

// abstractRule says whether a declaration concretizes an abstract
// initializer (Rust AbstractRule): let and var do, const does not.
type abstractRule int

const (
	concretizeAbstract abstractRule = iota
	allowAbstract
)

// typeAndInit is Rust naga's type_and_init for a function-local
// declaration: an initializer is converted to the declared type with
// automatic conversions; without a declared type it is concretized (unless
// rule allows abstract values) and its type becomes the declaration's.
// init is the already lowered initializer.
func (l *Lowerer) typeAndInit(name string, init ir.ExpressionHandle, explicit *ir.TypeHandle, rule abstractRule) (ir.TypeHandle, ir.ExpressionHandle, error) {
	if explicit != nil {
		goal := l.module.Types[*explicit].Inner
		converted, err := l.tryAutomaticConversions(init, goal)
		if err != nil {
			// Both mean the same thing to the reader of a declaration: the
			// initializer's type is not the declared one.
			var ce *conversionError
			if errors.As(err, &ce) {
				return 0, 0, fmt.Errorf("the type of `%s` is expected to be `%s`, but got `%s`",
					name, l.wgslTypeHandle(*explicit), ce.source)
			}
			return 0, 0, err
		}
		return *explicit, converted, nil
	}
	if rule == concretizeAbstract {
		var err error
		if init, err = l.concretize(init); err != nil {
			return 0, 0, err
		}
	}
	ty, err := l.inferTypeFromExpression(init)
	if err != nil {
		return 0, 0, err
	}
	return ty, init, nil
}

// conversionError is a value whose type does not convert to the required
// one: a concrete type that differs (Rust Error::TypeMismatch) or an
// abstract type that automatic conversions cannot convert (Rust
// Error::AutoConversion).
type conversionError struct {
	mismatch     bool
	dest, source string
}

func (e *conversionError) Error() string {
	if e.mismatch {
		return fmt.Sprintf("expected `%s`, found `%s`", e.dest, e.source)
	}
	return fmt.Sprintf("automatic conversions cannot convert `%s` to `%s`", e.source, e.dest)
}

// inConstContext runs f with the current function's evaluator in const
// behaviour: every expression f lowers must be a const-expression (Rust
// ExpressionContext::as_const for a function-local const).
func (l *Lowerer) inConstContext(f func() (ir.ExpressionHandle, error)) (ir.ExpressionHandle, error) {
	ev := l.eval()
	saved := ev.Behavior
	ev.Behavior = consteval.BehaviorConst
	defer func() { ev.Behavior = saved }()
	return f()
}

// convertMathArgs applies WGSL's automatic conversions to the arguments of
// a math built-in, following the overloads the specification defines (Rust
// naga resolves them with proc::overloads; a full port of overload
// resolution is tracked in TECH-WGSL-OVERLOADS-001):
//
//   - parameters of a fixed type (pack/unpack, the u32 offset and count of
//     extractBits/insertBits, quantizeToF16's f32) convert to it;
//   - built-ins defined only on concrete integers (bit counts, reverseBits,
//     first*Bit, extractBits, insertBits) concretize AbstractInt to i32;
//   - ldexp's exponent follows its mantissa (i32 for a concrete mantissa);
//   - every other built-in reconciles its arguments' leaf scalars, AbstractInt
//     becoming AbstractFloat for the float-only built-ins.
//
// Abstract values stay abstract where an overload accepts them, so a call
// of constant arguments is evaluated in the abstract type as the spec says.
func (l *Lowerer) convertMathArgs(fun ir.MathFunction, args []ir.ExpressionHandle) error {
	to := func(i int, s ir.ScalarType) error {
		c, err := l.tryAutomaticConversionForLeafScalar(args[i], s)
		if err != nil {
			return err
		}
		args[i] = c
		return nil
	}
	concreteInt := func(i int) error {
		if s, ok := l.leafScalarOfExpr(args[i]); ok && s.Kind == ir.ScalarAbstractInt {
			return to(i, scalarI32)
		}
		return nil
	}
	switch fun {
	case ir.MathPack4x8snorm, ir.MathPack4x8unorm, ir.MathPack2x16snorm, ir.MathPack2x16unorm,
		ir.MathPack2x16float, ir.MathQuantizeF16:
		return to(0, scalarF32)
	case ir.MathPack4xI8, ir.MathPack4xI8Clamp:
		return to(0, scalarI32)
	case ir.MathPack4xU8, ir.MathPack4xU8Clamp,
		ir.MathUnpack4x8snorm, ir.MathUnpack4x8unorm, ir.MathUnpack2x16snorm, ir.MathUnpack2x16unorm,
		ir.MathUnpack2x16float, ir.MathUnpack4xI8, ir.MathUnpack4xU8:
		return to(0, scalarU32)
	case ir.MathDot4I8Packed, ir.MathDot4U8Packed:
		for i := range args {
			if err := to(i, scalarU32); err != nil {
				return err
			}
		}
		return nil
	case ir.MathCountOneBits, ir.MathCountLeadingZeros, ir.MathCountTrailingZeros,
		ir.MathReverseBits, ir.MathFirstLeadingBit, ir.MathFirstTrailingBit:
		return concreteInt(0)
	case ir.MathExtractBits, ir.MathInsertBits:
		n := 1
		if fun == ir.MathInsertBits {
			n = 2
		}
		if c, _, ok := l.conversionConsensus(nil, args[:n]); ok {
			if c.Kind == ir.ScalarAbstractInt {
				c = scalarI32
			}
			if err := l.convertSliceToCommonLeafScalar(args[:n], c); err != nil {
				return err
			}
		}
		for i := n; i < len(args); i++ {
			if err := to(i, scalarU32); err != nil {
				return err
			}
		}
		return nil
	case ir.MathLdexp:
		// Rust has no abstract ldexp overload: an abstract mantissa becomes
		// f32 and the exponent i32 (Rust compiles ldexp(1.0, 3) to
		// ldexp(1f, 3i)).
		if s, ok := l.leafScalarOfExpr(args[0]); ok && isAbstractScalar(s) {
			if err := to(0, scalarF32); err != nil {
				return err
			}
		}
		if s, ok := l.leafScalarOfExpr(args[0]); ok && s.Kind == ir.ScalarFloat && len(args) > 1 {
			return to(1, scalarI32)
		}
		return nil
	case ir.MathModf, ir.MathFrexp:
		// Their result types are predeclared structs of the concrete
		// argument type.
		if s, ok := l.leafScalarOfExpr(args[0]); ok && s.Kind == ir.ScalarAbstractInt {
			if err := to(0, scalarAbstractFloat); err != nil {
				return err
			}
		}
		c, err := l.concretize(args[0])
		args[0] = c
		return err
	}
	consensus, _, ok := l.conversionConsensus(nil, args)
	if !ok {
		return nil // validation reports the argument types
	}
	if consensus.Kind == ir.ScalarAbstractInt && isFloatOnlyMathFunc(fun) {
		consensus = scalarAbstractFloat
	}
	if isAbstractScalar(consensus) && rustNoAbstractOverload(fun) {
		consensus = concretizeScalar(consensus)
		if isFloatOnlyMathFunc(fun) {
			consensus = scalarF32
		}
	}
	return l.convertSliceToCommonLeafScalar(args, consensus)
}

// rustNoAbstractOverload lists the float built-ins whose Rust naga overloads
// exclude the abstract types (FLOAT_ABSTRACT_UNIMPLEMENTED in
// proc/overloads/mathfunction.rs): Rust converts their abstract arguments to
// f32, which keeps its IR for these calls.
func rustNoAbstractOverload(fun ir.MathFunction) bool {
	switch fun {
	case ir.MathModf, ir.MathFrexp, ir.MathLdexp, ir.MathDistance, ir.MathFaceForward,
		ir.MathReflect, ir.MathRefract, ir.MathOuter:
		return true
	}
	return false
}
