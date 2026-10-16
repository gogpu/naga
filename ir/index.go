// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package ir

import "fmt"

// IndexableLength is the number of elements of an indexable type, for
// bounds checks. Mirrors Rust naga's proc::IndexableLength.
type IndexableLength struct {
	// Known is the number of elements when it is fixed by the type.
	Known uint32
	// Dynamic reports a length known only at run time: a runtime-sized
	// array or an unbounded binding array.
	Dynamic bool
}

// IndexableLengthOf returns the length of a vector (its size), a matrix (its
// columns) or an array or binding array, seeing through a pointer to one of
// those. It returns an error for a type that cannot be indexed. Mirrors Rust
// naga's TypeInner::indexable_length.
func IndexableLengthOf(module *Module, inner TypeInner) (IndexableLength, error) {
	switch t := inner.(type) {
	case VectorType:
		return IndexableLength{Known: uint32(t.Size)}, nil
	case MatrixType:
		return IndexableLength{Known: uint32(t.Columns)}, nil
	case ArrayType:
		if t.Size.Constant == nil {
			return IndexableLength{Dynamic: true}, nil
		}
		return IndexableLength{Known: *t.Size.Constant}, nil
	case BindingArrayType:
		if t.Size == nil {
			return IndexableLength{Dynamic: true}, nil
		}
		return IndexableLength{Known: *t.Size}, nil
	case ValuePointerType:
		if t.Size != nil {
			return IndexableLength{Known: uint32(*t.Size)}, nil
		}
	case PointerType:
		if int(t.Base) < len(module.Types) {
			switch base := module.Types[t.Base].Inner.(type) {
			case VectorType, MatrixType, ArrayType, BindingArrayType:
				return IndexableLengthOf(module, base)
			}
		}
	}
	return IndexableLength{}, fmt.Errorf("type %T is not indexable", inner)
}

// ConstIndexValue returns the value of the index expression handle in fn when
// it is known at translation time: a u32 or non-negative i32 literal, a
// scalar zero value, or a constant whose initializer is one of those. Mirrors
// Rust naga's GuardedIndex::from_expression (get_const_val_from with the u32
// conversion of proc::TryFrom<Literal>).
func ConstIndexValue(module *Module, fn *Function, handle ExpressionHandle) (uint32, bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, false
	}
	expr := fn.Expressions[handle].Kind
	if c, ok := expr.(ExprConstant); ok {
		if int(c.Constant) >= len(module.Constants) {
			return 0, false
		}
		init := module.Constants[c.Constant].Init
		if int(init) >= len(module.GlobalExpressions) {
			return 0, false
		}
		expr = module.GlobalExpressions[init].Kind
	}
	switch k := expr.(type) {
	case Literal:
		switch v := k.Value.(type) {
		case LiteralU32:
			return uint32(v), true
		case LiteralI32:
			if v >= 0 {
				return uint32(v), true
			}
		}
	case ExprZeroValue:
		if int(k.Type) < len(module.Types) {
			if s, ok := module.Types[k.Type].Inner.(ScalarType); ok &&
				(s.Kind == ScalarUint || s.Kind == ScalarSint) && s.Width == 4 {
				return 0, true
			}
		}
	}
	return 0, false
}
