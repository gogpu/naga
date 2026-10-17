package consteval

import (
	"strconv"

	"github.com/gogpu/naga/ir"
)

// constant evaluates a reference to a module constant. In the module arena
// the constant's initializer is the value (Rust "sees through" it); a
// function arena gets a copy of the initializer.
func (e *Evaluator) constant(c ir.ConstantHandle) (ir.ExpressionHandle, error) {
	if int(c) >= len(e.Module.Constants) {
		return 0, newError(ErrInvalidOperand, "Unknown constant")
	}
	init := e.Module.Constants[c].Init
	if e.Arena.IsGlobal() {
		return init, nil
	}
	return e.copyFromGlobal(init), nil
}

// checkAndGet returns an operand handle to place inside a new expression:
// constants are resolved as in constant, anything else is used as is.
func (e *Evaluator) checkAndGet(h ir.ExpressionHandle) (ir.ExpressionHandle, error) {
	if c, ok := e.Arena.Expression(h).(ir.ExprConstant); ok {
		return e.constant(c.Constant)
	}
	return h, nil
}

// copyFromGlobal deep-copies a module-scope constant value into the arena.
func (e *Evaluator) copyFromGlobal(h ir.ExpressionHandle) ir.ExpressionHandle {
	switch k := e.Module.GlobalExpressions[h].Kind.(type) {
	case ir.ExprCompose:
		comps := make([]ir.ExpressionHandle, len(k.Components))
		for i, c := range k.Components {
			comps[i] = e.copyFromGlobal(c)
		}
		return e.Arena.Append(ir.ExprCompose{Type: k.Type, Components: comps})
	case ir.ExprSplat:
		return e.Arena.Append(ir.ExprSplat{Size: k.Size, Value: e.copyFromGlobal(k.Value)})
	case ir.ExprConstant:
		return e.copyFromGlobal(e.Module.Constants[k.Constant].Init)
	default:
		return e.Arena.Append(k)
	}
}

func (e *Evaluator) compose(k ir.ExprCompose) (ir.ExpressionHandle, error) {
	comps := make([]ir.ExpressionHandle, len(k.Components))
	for i, c := range k.Components {
		r, err := e.checkAndGet(c)
		if err != nil {
			return 0, err
		}
		comps[i] = r
	}
	if vt, ok := e.Module.Types[k.Type].Inner.(ir.VectorType); ok {
		n := 0
		for _, c := range comps {
			n += e.lanes(c)
		}
		if n != int(vt.Size) {
			return 0, newError(ErrInvalidOperand, "Constructor expects "+strconv.Itoa(int(vt.Size))+" components, found "+strconv.Itoa(n))
		}
	}
	return e.Arena.Append(ir.ExprCompose{Type: k.Type, Components: comps}), nil
}

// lanes is the number of scalar components an operand of a vector
// constructor contributes.
func (e *Evaluator) lanes(h ir.ExpressionHandle) int {
	v, err := e.value(h)
	if err != nil {
		return 1
	}
	return v.Len()
}

// read returns the expression at h in the arena, or in the module's arena
// when global is set (for values reached through a constant).
func (e *Evaluator) read(global bool, h ir.ExpressionHandle) ir.ExpressionKind {
	if global {
		return e.Module.GlobalExpressions[h].Kind
	}
	return e.Arena.Expression(h)
}

// value reads an operand as a scalar or vector value, looking through
// constants, zero values, splats and nested vector constructors.
func (e *Evaluator) value(h ir.ExpressionHandle) (Value, error) {
	return e.valueIn(e.Arena.IsGlobal(), h)
}

func (e *Evaluator) valueIn(global bool, h ir.ExpressionHandle) (Value, error) {
	switch k := e.read(global, h).(type) {
	case ir.Literal:
		return Scalar(k.Value), nil
	case ir.ExprConstant:
		return e.valueIn(true, e.Module.Constants[k.Constant].Init)
	case ir.ExprZeroValue:
		switch t := e.Module.Types[k.Type].Inner.(type) {
		case ir.ScalarType:
			return Scalar(zeroLiteral(t)), nil
		case ir.VectorType:
			comps := make([]ir.LiteralValue, t.Size)
			for i := range comps {
				comps[i] = zeroLiteral(t.Scalar)
			}
			return Vector(comps...), nil
		}
	case ir.ExprSplat:
		v, err := e.valueIn(global, k.Value)
		if err != nil || !v.IsScalar() {
			return Value{}, newError(ErrInvalidOperand, "Splat is defined only on scalar values")
		}
		comps := make([]ir.LiteralValue, k.Size)
		for i := range comps {
			comps[i] = v.comps[0]
		}
		return Vector(comps...), nil
	case ir.ExprCompose:
		if _, ok := e.Module.Types[k.Type].Inner.(ir.VectorType); !ok {
			break
		}
		var comps []ir.LiteralValue
		for _, c := range k.Components {
			v, err := e.valueIn(global, c)
			if err != nil {
				return Value{}, err
			}
			comps = append(comps, v.comps...)
		}
		return Vector(comps...), nil
	}
	return Value{}, newError(ErrInvalidOperand, "Cannot apply the operation to a non-numeric value")
}

func zeroLiteral(s ir.ScalarType) ir.LiteralValue {
	if s.Kind == ir.ScalarBool {
		return ir.LiteralBool(false)
	}
	r, _ := castScalar(ir.LiteralAbstractInt(0), s)
	return r
}

// swizzle selects components of a vector: a new Compose of the selected
// component handles (Rust reuses them), a Splat of the new size, or a
// ZeroValue of the new vector type.
func (e *Evaluator) swizzle(k ir.ExprSwizzle) (ir.ExpressionHandle, error) {
	src, global, err := e.resolve(k.Vector)
	if err != nil {
		return 0, err
	}
	switch s := e.read(global, src).(type) {
	case ir.ExprZeroValue:
		if vt, ok := e.Module.Types[s.Type].Inner.(ir.VectorType); ok {
			return e.Arena.Append(ir.ExprZeroValue{Type: e.Types.Type(ir.VectorType{Size: k.Size, Scalar: vt.Scalar})}), nil
		}
	case ir.ExprSplat:
		return e.Arena.Append(ir.ExprSplat{Size: k.Size, Value: s.Value}), nil
	case ir.ExprCompose:
		vt, ok := e.Module.Types[s.Type].Inner.(ir.VectorType)
		if !ok {
			break
		}
		lanes := e.flatten(global, s)
		comps := make([]ir.ExpressionHandle, k.Size)
		for i := range comps {
			idx := int(k.Pattern[i])
			if idx >= len(lanes) {
				return 0, newError(ErrInvalidOperand, "swizzle component not present in source expression")
			}
			comps[i] = lanes[idx]
		}
		return e.Arena.Append(ir.ExprCompose{Type: e.Types.Type(ir.VectorType{Size: k.Size, Scalar: vt.Scalar}), Components: comps}), nil
	}
	return 0, newError(ErrInvalidOperand, "Can only swizzle vector constants")
}

// resolve follows a constant reference: in the module arena to the
// initializer, in a function arena to a copy of it.
func (e *Evaluator) resolve(h ir.ExpressionHandle) (ir.ExpressionHandle, bool, error) {
	r, err := e.checkAndGet(h)
	return r, e.Arena.IsGlobal(), err
}

// flatten returns the scalar component handles of a vector constructor,
// expanding nested vector constructors and splats.
func (e *Evaluator) flatten(global bool, c ir.ExprCompose) []ir.ExpressionHandle {
	var out []ir.ExpressionHandle
	for _, h := range c.Components {
		switch k := e.read(global, h).(type) {
		case ir.ExprCompose:
			if _, ok := e.Module.Types[k.Type].Inner.(ir.VectorType); ok {
				out = append(out, e.flatten(global, k)...)
				continue
			}
		case ir.ExprSplat:
			for range int(k.Size) {
				out = append(out, k.Value)
			}
			continue
		}
		out = append(out, h)
	}
	return out
}

func (e *Evaluator) indexOf(h ir.ExpressionHandle) (uint32, error) {
	v, err := e.value(h)
	if err != nil || !v.IsScalar() {
		return 0, newError(ErrInvalidOperand, "Cannot access with index of type")
	}
	switch i := v.comps[0].(type) {
	case ir.LiteralU32:
		return uint32(i), nil
	case ir.LiteralI32:
		if i >= 0 {
			return uint32(i), nil
		}
	case ir.LiteralAbstractInt:
		if i >= 0 && i <= 1<<32-1 {
			return uint32(i), nil
		}
	}
	return 0, newError(ErrInvalidOperand, "Cannot access with index of type")
}

// access is base[index] for a constant base: the component handle of a
// constructor (vector constructors are flattened), the value of a splat, or
// the zero value of the component type.
func (e *Evaluator) access(base ir.ExpressionHandle, index uint32) (ir.ExpressionHandle, error) {
	src, global, err := e.resolve(base)
	if err != nil {
		return 0, err
	}
	invalidBase := newError(ErrInvalidOperand, "Cannot access the type")
	switch s := e.read(global, src).(type) {
	case ir.ExprZeroValue:
		ty, ok := e.componentType(s.Type, index)
		if !ok {
			return 0, invalidBase
		}
		return e.Arena.Append(ir.ExprZeroValue{Type: ty}), nil
	case ir.ExprSplat:
		if index >= uint32(s.Size) {
			return 0, invalidBase
		}
		return s.Value, nil
	case ir.ExprCompose:
		comps := s.Components
		if _, ok := e.Module.Types[s.Type].Inner.(ir.VectorType); ok {
			comps = e.flatten(global, s)
		} else if _, ok := e.componentType(s.Type, 0); !ok {
			return 0, invalidBase
		}
		if int(index) >= len(comps) {
			return 0, newError(ErrInvalidOperand, "Cannot access at the index")
		}
		return comps[index], nil
	}
	return 0, invalidBase
}

// componentType returns the type of component index of a composite type:
// vector -> scalar, matrix -> column vector, sized array -> base, struct ->
// the member's type. ok is false if the type has no such component.
func (e *Evaluator) componentType(th ir.TypeHandle, index uint32) (ir.TypeHandle, bool) {
	switch t := e.Module.Types[th].Inner.(type) {
	case ir.VectorType:
		return e.Types.Type(t.Scalar), index < uint32(t.Size)
	case ir.MatrixType:
		return e.Types.Type(ir.VectorType{Size: t.Rows, Scalar: t.Scalar}), index < uint32(t.Columns)
	case ir.ArrayType:
		return t.Base, t.Size.Constant != nil && index < *t.Size.Constant
	case ir.StructType:
		if int(index) < len(t.Members) {
			return t.Members[index].Type, true
		}
	}
	return 0, false
}
