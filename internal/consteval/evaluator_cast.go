package consteval

import "github.com/gogpu/naga/ir"

// CastArray converts the leaf scalars of the const-expression h to the
// scalar type to, as Rust naga's cast_array does for WGSL's automatic
// conversions and concretization: an array constructor is converted
// element by element into a new constructor of the converted array type,
// a zero array becomes a zero value of the converted type, and anything
// else is an As conversion.
func (e *Evaluator) CastArray(h ir.ExpressionHandle, to ir.ScalarType) (ir.ExpressionHandle, error) {
	if !e.IsConst(h) {
		return 0, newError(ErrNotConst, "Subexpression(s) are not constant")
	}
	h, err := e.checkAndGet(h)
	if err != nil {
		return 0, err
	}
	switch k := e.Arena.Expression(h).(type) {
	case ir.ExprCompose:
		at, ok := e.Module.Types[k.Type].Inner.(ir.ArrayType)
		if !ok {
			break
		}
		comps := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			if comps[i], err = e.CastArray(c, to); err != nil {
				return 0, err
			}
		}
		base := at.Base
		if len(comps) > 0 {
			base = e.typeOf(comps[0])
		}
		return e.appendConst(ir.ExprCompose{Type: e.Types.Array(base, at.Size), Components: comps}), nil
	case ir.ExprZeroValue:
		if _, ok := e.Module.Types[k.Type].Inner.(ir.ArrayType); ok {
			return e.appendConst(ir.ExprZeroValue{Type: e.withLeafScalar(k.Type, to)}), nil
		}
	}
	width := to.Width
	return e.TryEvalAndAppend(ir.ExprAs{Expr: h, Kind: to.Kind, Convert: &width})
}

// typeOf is the type of an evaluated expression.
func (e *Evaluator) typeOf(h ir.ExpressionHandle) ir.TypeHandle {
	switch k := e.Arena.Expression(h).(type) {
	case ir.ExprCompose:
		return k.Type
	case ir.ExprZeroValue:
		return k.Type
	case ir.ExprSplat:
		return e.Types.Type(ir.VectorType{Size: k.Size, Scalar: e.operandScalar(k.Value)})
	}
	return e.Types.Type(e.operandScalar(h))
}

// withLeafScalar is the type th with its leaf scalar replaced by to.
func (e *Evaluator) withLeafScalar(th ir.TypeHandle, to ir.ScalarType) ir.TypeHandle {
	switch t := e.Module.Types[th].Inner.(type) {
	case ir.ScalarType:
		return e.Types.Type(to)
	case ir.VectorType:
		return e.Types.Type(ir.VectorType{Size: t.Size, Scalar: to})
	case ir.MatrixType:
		return e.Types.Type(ir.MatrixType{Columns: t.Columns, Rows: t.Rows, Scalar: to})
	case ir.ArrayType:
		return e.Types.Array(e.withLeafScalar(t.Base, to), t.Size)
	}
	return th
}

// cast is the conversion As{Convert} of a const-expression, Rust naga's
// cast: it keeps the shape of the value. A literal is converted; a vector or
// matrix constructor is converted component by component; a Splat becomes
// a Splat of the converted scalar; a ZeroValue is first expanded
// (eval_zero_value).
func (e *Evaluator) cast(h ir.ExpressionHandle, to ir.ScalarType) (ir.ExpressionHandle, error) {
	h, err := e.checkAndGet(h)
	if err != nil {
		return 0, err
	}
	if z, ok := e.Arena.Expression(h).(ir.ExprZeroValue); ok {
		if h, err = e.zeroValue(z.Type); err != nil {
			return 0, err
		}
	}
	switch k := e.Arena.Expression(h).(type) {
	case ir.Literal:
		r, err := castScalar(k.Value, to)
		if err != nil {
			return 0, err
		}
		return e.appendConst(ir.Literal{Value: r}), nil
	case ir.ExprCompose:
		switch e.Module.Types[k.Type].Inner.(type) {
		case ir.VectorType, ir.MatrixType:
		default:
			return 0, errInvalidCast()
		}
		comps := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			if comps[i], err = e.cast(c, to); err != nil {
				return 0, err
			}
		}
		return e.appendConst(ir.ExprCompose{Type: e.withLeafScalar(k.Type, to), Components: comps}), nil
	case ir.ExprSplat:
		v, err := e.cast(k.Value, to)
		if err != nil {
			return 0, err
		}
		return e.appendConst(ir.ExprSplat{Size: k.Size, Value: v}), nil
	}
	return 0, errInvalidCast()
}

func errInvalidCast() *Error {
	return newError(ErrInvalidOperand, "Cannot cast the value")
}

// zeroValue expands the zero value of ty into literals and constructors,
// one shared literal per element type, as Rust naga's eval_zero_value.
func (e *Evaluator) zeroValue(ty ir.TypeHandle) (ir.ExpressionHandle, error) {
	repeat := func(el ir.ExpressionHandle, n int) []ir.ExpressionHandle {
		out := make([]ir.ExpressionHandle, n)
		for i := range out {
			out[i] = el
		}
		return out
	}
	switch t := e.Module.Types[ty].Inner.(type) {
	case ir.ScalarType:
		return e.appendConst(ir.Literal{Value: zeroLiteral(t)}), nil
	case ir.VectorType:
		el, err := e.zeroValue(e.Types.Type(t.Scalar))
		if err != nil {
			return 0, err
		}
		return e.appendConst(ir.ExprCompose{Type: ty, Components: repeat(el, int(t.Size))}), nil
	case ir.MatrixType:
		el, err := e.zeroValue(e.Types.Type(ir.VectorType{Size: t.Rows, Scalar: t.Scalar}))
		if err != nil {
			return 0, err
		}
		return e.appendConst(ir.ExprCompose{Type: ty, Components: repeat(el, int(t.Columns))}), nil
	case ir.ArrayType:
		if t.Size.Constant == nil {
			break
		}
		el, err := e.zeroValue(t.Base)
		if err != nil {
			return 0, err
		}
		return e.appendConst(ir.ExprCompose{Type: ty, Components: repeat(el, int(*t.Size.Constant))}), nil
	case ir.StructType:
		comps := make([]ir.ExpressionHandle, len(t.Members))
		for i, m := range t.Members {
			c, err := e.zeroValue(m.Type)
			if err != nil {
				return 0, err
			}
			comps[i] = c
		}
		return e.appendConst(ir.ExprCompose{Type: ty, Components: comps}), nil
	}
	return 0, newError(ErrInvalidOperand, "Type is not constructible")
}
