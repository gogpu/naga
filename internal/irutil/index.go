// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package irutil

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// IndexableLengthOf returns the length of a vector (its size), a matrix (its
// columns) or an array or binding array, seeing through a pointer to one of
// those. It returns an error for a type that cannot be indexed. Mirrors Rust
// naga's TypeInner::indexable_length.
func IndexableLengthOf(module *ir.Module, inner ir.TypeInner) (ir.IndexableLength, error) {
	switch t := inner.(type) {
	case ir.VectorType:
		return ir.IndexableLength{Known: uint32(t.Size)}, nil
	case ir.MatrixType:
		return ir.IndexableLength{Known: uint32(t.Columns)}, nil
	case ir.ArrayType:
		return t.Size.Resolve(module)
	case ir.BindingArrayType:
		if t.Size == nil {
			return ir.IndexableLength{Dynamic: true}, nil
		}
		return ir.IndexableLength{Known: *t.Size}, nil
	case ir.ValuePointerType:
		if t.Size != nil {
			return ir.IndexableLength{Known: uint32(*t.Size)}, nil
		}
	case ir.PointerType:
		if int(t.Base) < len(module.Types) {
			switch base := module.Types[t.Base].Inner.(type) {
			case ir.VectorType, ir.MatrixType, ir.ArrayType, ir.BindingArrayType:
				return IndexableLengthOf(module, base)
			}
		}
	}
	return ir.IndexableLength{}, fmt.Errorf("type %T is not indexable", inner)
}

// ConstIndexValue returns the value of the index expression handle in fn when
// it is known at translation time: a u32 or non-negative i32 literal, a
// scalar zero value, or a constant whose initializer is one of those. Mirrors
// Rust naga's GuardedIndex::from_expression (get_const_val_from with the u32
// conversion of proc::TryFrom<Literal>).
func ConstIndexValue(module *ir.Module, fn *ir.Function, handle ir.ExpressionHandle) (uint32, bool) {
	if int(handle) >= len(fn.Expressions) {
		return 0, false
	}
	expr := fn.Expressions[handle].Kind
	if c, ok := expr.(ir.ExprConstant); ok {
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
	case ir.Literal:
		switch v := k.Value.(type) {
		case ir.LiteralU32:
			return uint32(v), true
		case ir.LiteralI32:
			if v >= 0 {
				return uint32(v), true
			}
		}
	case ir.ExprZeroValue:
		if int(k.Type) < len(module.Types) {
			if s, ok := module.Types[k.Type].Inner.(ir.ScalarType); ok &&
				(s.Kind == ir.ScalarUint || s.Kind == ir.ScalarSint) && s.Width == 4 {
				return 0, true
			}
		}
	}
	return 0, false
}
