package consteval

import "github.com/gogpu/naga/ir"

// Relational evaluates all, any, isNan and isInf. all and any reduce a bool
// vector to a bool and return a bool scalar unchanged. isNan and isInf are
// false for every component: a constant is always finite (Rust v30 does not
// evaluate them; see RustEvaluatesRelational).
func Relational(fun ir.RelationalFunction, v Value) (Value, error) {
	s := v.Scalar()
	switch fun {
	case ir.RelationalAll, ir.RelationalAny:
		if s.Kind != ir.ScalarBool {
			return Value{}, errInvalidRelationalArg()
		}
		want := fun == ir.RelationalAny
		for _, c := range v.comps {
			if bool(c.(ir.LiteralBool)) == want {
				return Scalar(ir.LiteralBool(want)), nil
			}
		}
		return Scalar(ir.LiteralBool(!want)), nil
	case ir.RelationalIsNan, ir.RelationalIsInf:
		if !isFloat(s) {
			return Value{}, errInvalidRelationalArg()
		}
		out := make([]ir.LiteralValue, v.Len())
		for i := range out {
			out[i] = ir.LiteralBool(false)
		}
		return v.withComponents(out), nil
	}
	return Value{}, errInvalidRelationalArg()
}

// RustEvaluatesRelational reports whether Rust naga v30 evaluates the
// function; a function body keeps isNan and isInf as run-time expressions to
// match Rust's IR.
func RustEvaluatesRelational(fun ir.RelationalFunction) bool {
	return fun == ir.RelationalAll || fun == ir.RelationalAny
}

func errInvalidRelationalArg() *Error {
	return newError(ErrInvalidOperand, "Cannot apply relational function to type")
}

// Select is select(reject, accept, condition): accept where the condition is
// true. A bool scalar condition chooses for every component; a bool vector
// chooses per component. As in Rust naga, accept is first converted to
// reject's scalar type, and a failed conversion is an error even where the
// condition picks reject.
func Select(reject, accept, cond Value) (Value, error) {
	if err := checkSelectShapes(reject, accept, cond); err != nil {
		return Value{}, err
	}
	accept, err := Cast(accept, reject.Scalar())
	if err != nil {
		return Value{}, err
	}
	return lanes(reject.Len(), func(i int) (ir.LiteralValue, error) {
		if cond.comps[min(i, cond.Len()-1)].(ir.LiteralBool) {
			return accept.comps[i], nil
		}
		return reject.comps[i], nil
	})
}

// checkSelectShapes applies Rust naga's select shape rules and messages.
func checkSelectShapes(reject, accept, cond Value) error {
	switch {
	case reject.IsScalar() != accept.IsScalar():
		return newError(ErrInvalidOperand,
			"Expected reject and accept args. to be scalars of vectors of the same type, got something else")
	case reject.IsScalar():
		if !cond.IsScalar() || cond.Scalar().Kind != ir.ScalarBool {
			return newError(ErrInvalidOperand,
				"Expected boolean expression for condition argument of `select`, got something else")
		}
	case reject.Len() != accept.Len():
		return newError(ErrInvalidOperand, "Expected vectors of the same size for reject and accept args., got "+
			rustVectorSize(reject.Len())+" and "+rustVectorSize(accept.Len()))
	case cond.Scalar().Kind != ir.ScalarBool:
		return newError(ErrInvalidOperand, "Expected boolean vector for condition arg., got something else")
	case !cond.IsScalar() && cond.Len() != reject.Len():
		return newError(ErrInvalidOperand,
			"Expected same number of vector components between condition, accept, and reject args., got something else")
	}
	return nil
}

// rustVectorSize is the Debug name of Rust's VectorSize.
func rustVectorSize(n int) string {
	switch n {
	case 2:
		return "Bi"
	case 3:
		return "Tri"
	}
	return "Quad"
}
