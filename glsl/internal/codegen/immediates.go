// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package codegen

import (
	"errors"
	"fmt"

	"github.com/gogpu/naga/ir"
)

// ImmediateItem is one GL uniform that carries part of the immediate data.
// See glsl.ImmediateItem.
type ImmediateItem struct {
	AccessPath string
	Type       ir.TypeInner
	Offset     uint32
	SizeBytes  uint32
}

var errMultipleImmediateData = errors.New("an entry point uses multiple immediate data variables, at most one is allowed")

// collectImmediateItems lists the GL uniforms of the entry point's
// immediate variable (Rust naga Writer::collect_immediates_items).
//
// Offsets come from the WGSL layout (StructMember.Offset), the bytes the
// application writes, where Rust re-derives them by rounding each member up
// to its alignment, so a member that follows an @size or @align member can
// get the wrong offset.
func (w *Writer) collectImmediateItems() error {
	used := false
	for handle, global := range w.module.GlobalVariables {
		if global.Space != ir.SpaceImmediate {
			continue
		}
		if w.reachable != nil && !w.reachable.hasGlobal(ir.GlobalVariableHandle(handle)) {
			continue
		}
		// The WGSL immediate-data proposal allows one statically used
		// immediate variable per entry point, and GL has one uniform name
		// per stage for it (Rust Error::MultipleImmediateData).
		if used {
			return errMultipleImmediateData
		}
		used = true
		name := w.names[nameKey{kind: nameKeyGlobalVariable, handle1: uint32(handle)}]
		if err := w.appendImmediateItems(global.Type, name, 0); err != nil {
			return err
		}
	}
	if w.clampFragDepth {
		f32 := ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}
		offset := w.options.ClampFragDepth.Offset
		w.immediateItems = append(w.immediateItems,
			ImmediateItem{AccessPath: fragImmediates + "." + fragDepthMin, Type: f32, Offset: offset, SizeBytes: 4},
			ImmediateItem{AccessPath: fragImmediates + "." + fragDepthMax, Type: f32, Offset: offset + 4, SizeBytes: 4})
	}
	return nil
}

func (w *Writer) appendImmediateItems(ty ir.TypeHandle, path string, offset uint32) error {
	switch t := w.module.Types[ty].Inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		w.immediateItems = append(w.immediateItems, ImmediateItem{
			AccessPath: path,
			Type:       t,
			Offset:     offset,
			SizeBytes:  ir.TypeSize(w.module, ty),
		})
	case ir.StructType:
		for i, m := range t.Members {
			member := w.names[nameKey{kind: nameKeyStructMember, handle1: uint32(ty), handle2: uint32(i)}]
			if err := w.appendImmediateItems(m.Type, path+"."+member, offset+m.Offset); err != nil {
				return err
			}
		}
	default:
		// WGSL excludes arrays, and structs with array members, from
		// immediate data, as Rust naga's validator does
		// (ImmediateError::InvalidArray); every other type is not host-shareable.
		return fmt.Errorf("immediate data %s: type %T is not allowed in immediate data", path, t)
	}
	return nil
}
