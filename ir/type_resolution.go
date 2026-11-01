package ir

import "fmt"

// Type returns the type of the literal, always a scalar value (Rust naga's
// Literal::ty_inner). It fails only for a LiteralValue of no known kind.
func (lit Literal) Type() (TypeResolution, error) {
	switch v := lit.Value.(type) {
	case LiteralF64:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 8}}, nil
	case LiteralF16:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 2}}, nil
	case LiteralF32:
		return TypeResolution{Value: ScalarType{Kind: ScalarFloat, Width: 4}}, nil
	case LiteralU32:
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 4}}, nil
	case LiteralI32:
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 4}}, nil
	case LiteralU64:
		return TypeResolution{Value: ScalarType{Kind: ScalarUint, Width: 8}}, nil
	case LiteralI64:
		return TypeResolution{Value: ScalarType{Kind: ScalarSint, Width: 8}}, nil
	case LiteralBool:
		return TypeResolution{Value: ScalarType{Kind: ScalarBool, Width: 1}}, nil
	case LiteralAbstractInt:
		// Abstract types keep their kind, as in Rust naga's typifier; the
		// WGSL frontend concretizes them before they reach a backend.
		return TypeResolution{Value: ScalarType{Kind: ScalarAbstractInt, Width: 8}}, nil
	case LiteralAbstractFloat:
		return TypeResolution{Value: ScalarType{Kind: ScalarAbstractFloat, Width: 8}}, nil
	default:
		return TypeResolution{}, fmt.Errorf("unknown literal type: %T", v)
	}
}

// Inner returns the type r stands for: the inner type of the arena type it
// refers to, or its own value (Rust naga's TypeResolution::inner_with). types
// is the module's type arena; a handle outside it panics.
func (r TypeResolution) Inner(types []Type) TypeInner {
	if r.Handle != nil {
		return types[*r.Handle].Inner
	}
	return r.Value
}
