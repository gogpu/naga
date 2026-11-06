// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"
	"maps"
	"slices"

	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/internal/typifier"
	"github.com/gogpu/naga/internal/unsupported"
	"github.com/gogpu/naga/ir"
)

// splitLocalVariables gives every function-local aggregate of fn the shape
// DXC leaves its locals in, with the scalarizer that splits workgroup memory
// (scalarizeWorkgroupMemory): DXC's SROA (ScalarReplAggregatesHLSL.cpp,
// SROA_Helper::DoScalarReplacement) replaces a struct alloca by one alloca per
// member and an array of structs by one array per member ("Array will be
// flattened as struct of array"); DynamicIndexingVectorToArray and
// MultiDimArrayToOneDimArray turn what is left into one-dimensional arrays of
// scalars. So a local whose type is not a scalar, a vector or an array of
// scalars becomes pieces: one local per struct member, below arrays of
// structs one array per member, each a scalar, a vector kept whole (the
// emitter keeps a vector local in one alloca per component, as SROA leaves
// it), or a one-dimensional array of all its scalars. A pointer into the
// local is a run of accesses, and an access below a piece becomes an index
// into it: `p[i].x` of an `array<vec2<f32>, 3>` is element `i * 2` of an
// `array<f32, 6>`.
//
// The emitter used to keep such locals whole, with paths for each shape of
// access, and several shapes had none or a wrong one: a component of a
// run-time element of an array of vectors gave a malformed record
// (P1-DXIL-081), a member of a struct below an array or a struct gave a GEP
// of the wrong type or none at all (P1-DXIL-064, P1-DXIL-065, P1-DXIL-071),
// and arrays updated in a loop lost components (P1-DXIL-079).
//
// A local is left whole when a pointer into it is used other than by an
// access, a load or a store (a call argument), or when its initializer is not
// a constant expression.
func splitLocalVariables(m *ir.Module, fn *ir.Function) error {
	candidates := localSplitCandidates(m, fn)
	if len(candidates) == 0 {
		return nil
	}
	types, err := typifier.Function(m, fn)
	if err != nil {
		return fmt.Errorf("dxil: local variables: %w", err)
	}
	s := &wgScalarizer{
		m:      m,
		fn:     fn,
		vars:   make(map[ir.GlobalVariableHandle]*wgVar),
		locals: make(map[ir.LocalVariableHandle]*wgVar),
		locs:   make(map[ir.ExpressionHandle]*wgLoc),
		u32:    typeHandle(m, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
		types:  types,
	}
	var inits ir.Block
	for _, h := range candidates {
		lv := fn.LocalVars[h]
		v := &wgVar{ty: lv.Type, leaves: make(map[string]*wgLeaf), local: true}
		if err := s.split(v, lv.Name, lv.Type, nil, nil); err != nil {
			return fmt.Errorf("dxil: local variable %q: %w", lv.Name, err)
		}
		s.locals[h] = v
		if lv.Init != nil {
			inits = append(inits, s.splitInit(v, *lv.Init)...)
		}
	}

	n := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
	body, err := s.block(fn.Body)
	if err == nil {
		err = s.err
	}
	if err != nil {
		return fmt.Errorf("dxil: local variables: %w", err)
	}
	inits = append(inits, body...)
	fn.Body = inits

	// A load no Emit covers is evaluated where it is used: it reads the
	// pieces there too.
	for h := ir.ExpressionHandle(0); h < n; h++ {
		if load, ok := fn.Expressions[h].Kind.(ir.ExprLoad); ok {
			if l := s.loc(load.Pointer); l != nil {
				fn.Expressions[h].Kind = s.loadKind(l)
			}
		}
	}
	return s.err
}

// splitInit gives the pieces of split local v their part of its initializer
// init, a constant expression: as their own initializers when the parts can
// be read off init's operands, else as stores at the start of the function,
// which is where a local's initializer is evaluated.
func (s *wgScalarizer) splitInit(v *wgVar, init ir.ExpressionHandle) ir.Block {
	parts := make(map[string][]ir.ExpressionHandle)
	if s.initParts(v, init, v.ty, nil, parts) {
		for _, key := range slices.Sorted(maps.Keys(v.leaves)) {
			leaf := v.leaves[key]
			values := parts[key]
			lv := &s.fn.LocalVars[leaf.local]
			var h ir.ExpressionHandle
			switch {
			case leaf.single || leaf.vector:
				h = values[0]
			case allZero(s.fn, values):
				h = s.add(ir.ExprZeroValue{Type: lv.Type})
			default:
				h = s.add(ir.ExprCompose{Type: lv.Type, Components: values})
			}
			lv.Init = &h
		}
		return nil
	}
	start := s.next()
	stores := s.store(s.enter(&wgLoc{v: v, ty: v.ty}), init)
	return append(s.emitsFrom(start), stores...)
}

// initParts collects the parts of value, of type ty, reached through the
// struct members path, for the pieces of v: the value of a scalar or vector
// piece, the scalars of an array piece in its order. ok is false when a part
// cannot be read off value's operands without evaluating an access.
func (s *wgScalarizer) initParts(v *wgVar, value ir.ExpressionHandle, ty ir.TypeHandle, path []uint32, parts map[string][]ir.ExpressionHandle) bool {
	switch t := s.m.Types[ty].Inner.(type) {
	case ir.StructType:
		for i, member := range t.Members {
			e, ok := s.constElement(value, uint32(i), member.Type)
			if !ok || !s.initParts(v, e, member.Type, append(slices.Clone(path), uint32(i)), parts) {
				return false
			}
		}
		return true
	case ir.ArrayType:
		if s.holdsStruct(t.Base) {
			for i := uint32(0); i < *t.Size.Constant; i++ {
				e, ok := s.constElement(value, i, t.Base)
				if !ok || !s.initParts(v, e, t.Base, path, parts) {
					return false
				}
			}
			return true
		}
	}
	key := pathKey(path)
	leaf := v.leaves[key]
	if leaf.single || leaf.vector {
		parts[key] = append(parts[key], value)
		return true
	}
	return s.initScalars(value, ty, key, parts)
}

// initScalars appends the scalars of value, of type ty, which holds no struct,
// to parts[key].
func (s *wgScalarizer) initScalars(value ir.ExpressionHandle, ty ir.TypeHandle, key string, parts map[string][]ir.ExpressionHandle) bool {
	n, composite := s.elementCount(ty)
	if !composite {
		parts[key] = append(parts[key], value)
		return true
	}
	for i := uint32(0); i < n; i++ {
		elemTy := s.elementType(ty)
		e, ok := s.constElement(value, i, elemTy)
		if !ok || !s.initScalars(e, elemTy, key, parts) {
			return false
		}
	}
	return true
}

// elementType is the type of an element of a composite of type ty with no
// struct in it: an array's base, a vector's scalar, a matrix's column.
func (s *wgScalarizer) elementType(ty ir.TypeHandle) ir.TypeHandle {
	switch t := s.m.Types[ty].Inner.(type) {
	case ir.ArrayType:
		return t.Base
	case ir.VectorType:
		return typeHandle(s.m, t.Scalar)
	case ir.MatrixType:
		return typeHandle(s.m, ir.VectorType{Size: t.Rows, Scalar: t.Scalar})
	}
	return ty
}

// constElement returns element i, of type ty, of constant expression v when
// it can be read off v without evaluating an access: an operand of a
// composite of elements of that type, the value of a splat, or a zero.
func (s *wgScalarizer) constElement(v ir.ExpressionHandle, i uint32, ty ir.TypeHandle) (ir.ExpressionHandle, bool) {
	switch k := s.fn.Expressions[v].Kind.(type) {
	case ir.ExprZeroValue:
		return s.add(ir.ExprZeroValue{Type: ty}), true
	case ir.ExprSplat:
		return k.Value, true
	case ir.ExprCompose:
		if int(i) < len(k.Components) && len(k.Components) == s.lenOf(k.Type) && s.hasType(k.Components[i], ty) {
			return k.Components[i], true
		}
	}
	return 0, false
}

// allZero reports whether every expression of hs is a zero value.
func allZero(fn *ir.Function, hs []ir.ExpressionHandle) bool {
	for _, h := range hs {
		if _, ok := fn.Expressions[h].Kind.(ir.ExprZeroValue); !ok {
			return false
		}
	}
	return true
}

// localSplitCandidates returns the locals of fn splitLocalVariables splits:
// those of an aggregate type (not a scalar, a vector or an array of scalars),
// with a constant initializer or none, whose pointers are used only by
// accesses, loads and stores.
func localSplitCandidates(m *ir.Module, fn *ir.Function) []ir.LocalVariableHandle {
	cand := make(map[ir.LocalVariableHandle]bool)
	for i, lv := range fn.LocalVars {
		if splitLocalType(m, lv.Type) && (lv.Init == nil || constInit(fn, *lv.Init)) {
			cand[ir.LocalVariableHandle(i)] = true
		}
	}
	if len(cand) == 0 {
		return nil
	}
	dropMisusedLocals(fn, cand)

	out := make([]ir.LocalVariableHandle, 0, len(cand))
	for h := range cand {
		out = append(out, h)
	}
	slices.Sort(out)
	return out
}

// localRoot returns the candidate of cand that pointer h points into,
// following its run of accesses.
func localRoot(fn *ir.Function, cand map[ir.LocalVariableHandle]bool, h ir.ExpressionHandle) (ir.LocalVariableHandle, bool) {
	for {
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprLocalVariable:
			return k.Variable, cand[k.Variable]
		case ir.ExprAccess:
			h = k.Base
		case ir.ExprAccessIndex:
			h = k.Base
		default:
			return 0, false
		}
	}
}

// dropMisusedLocals removes from cand the locals a pointer of which is used
// other than as the base of an access, the pointer of a load or the pointer
// of a store.
func dropMisusedLocals(fn *ir.Function, cand map[ir.LocalVariableHandle]bool) {
	for _, expr := range fn.Expressions {
		irutil.ExprHandles(expr.Kind, func(sub ir.ExpressionHandle) {
			if lv, ok := localRoot(fn, cand, sub); ok && !pointerUseAllowed(expr.Kind, sub) {
				delete(cand, lv)
			}
		})
	}
	var walk func(b ir.Block)
	walk = func(b ir.Block) {
		for _, st := range b {
			switch k := st.Kind.(type) {
			case ir.StmtEmit:
				continue
			case ir.StmtStore:
				if lv, ok := localRoot(fn, cand, k.Value); ok {
					delete(cand, lv)
				}
				continue
			}
			irutil.StmtExprHandles(st.Kind, func(h ir.ExpressionHandle) {
				if lv, ok := localRoot(fn, cand, h); ok {
					delete(cand, lv)
				}
			})
			irutil.NestedBlocks(st.Kind, walk)
		}
	}
	walk(fn.Body)
}

// pointerUseAllowed reports whether an expression of kind may take pointer
// sub as an operand: as the base of an access or the pointer of a load.
func pointerUseAllowed(kind ir.ExpressionKind, sub ir.ExpressionHandle) bool {
	switch k := kind.(type) {
	case ir.ExprAccess:
		return k.Base == sub
	case ir.ExprAccessIndex, ir.ExprLoad:
		return true
	}
	return false
}

// splitLocalType reports whether a local of type ty is split: an aggregate
// other than an array of scalars, whose arrays all have a constant size and
// whose leaves are scalars, vectors or matrices.
func splitLocalType(m *ir.Module, ty ir.TypeHandle) bool {
	switch t := m.Types[ty].Inner.(type) {
	case ir.StructType, ir.MatrixType:
		return splittableType(m, ty)
	case ir.ArrayType:
		if _, scalar := m.Types[t.Base].Inner.(ir.ScalarType); scalar {
			return false
		}
		return splittableType(m, ty)
	}
	return false
}

// splittableType reports whether a value of type ty can be split into
// pieces: structs, arrays of a constant size, matrices, vectors and scalars.
func splittableType(m *ir.Module, ty ir.TypeHandle) bool {
	switch t := m.Types[ty].Inner.(type) {
	case ir.ScalarType, ir.VectorType, ir.MatrixType:
		return true
	case ir.ArrayType:
		return t.Size.Constant != nil && splittableType(m, t.Base)
	case ir.StructType:
		for _, member := range t.Members {
			if !splittableType(m, member.Type) {
				return false
			}
		}
		return true
	}
	return false
}

// constInit reports whether initializer h is a constant expression: built
// of literals, constants, overrides and zero values only.
func constInit(fn *ir.Function, h ir.ExpressionHandle) bool {
	switch k := fn.Expressions[h].Kind.(type) {
	case ir.Literal, ir.ExprConstant, ir.ExprOverride, ir.ExprZeroValue:
		return true
	case ir.ExprSplat:
		return constInit(fn, k.Value)
	case ir.ExprCompose:
		for _, c := range k.Components {
			if !constInit(fn, c) {
				return false
			}
		}
		return true
	}
	return false
}

// maxFunctionVariableBytes bounds the combined byte size of the function
// variables of an entry point, its private variables included
// (lowerPrivatesToLocals makes them locals). The bound is not about the
// GPU: a declared array length is a literal in the source, and every
// whole-variable initialization (the zero store of a local declared without
// an initializer, zeroValueFor) and every whole-array copy is written element
// by element, since DXIL has no aggregate store. DXC unrolls the same way
// (a float[4096] = (float[4096])0 gives 4097 stores, a float[65536] takes it
// 24 s); its HLSL SROA has no size threshold either (ShouldAttemptScalarRepl,
// ScalarReplAggregatesHLSL.cpp:1231, "promote every struct / array"; the
// SRThreshold, StructMemberThreshold and ArrayElementThreshold of
// ScalarReplAggregates.cpp:67-77 belong to LLVM's generic SROA, which the
// HLSL pipeline does not run). Unbounded, a shader of a few dozen bytes took
// seconds per million elements and ran the process out of memory at eight
// million. WGSL requires an implementation to support 8192 bytes of
// function variables per function and 8192 of private ones, and asks one
// that does not support more to issue an error ("Limits"); this allows
// eight times that, the 4096 sixteen-byte temporary registers of D3D's
// earlier shader models (kMaxTempRegCount, DxilConstants.h:103).
const maxFunctionVariableBytes = 65536

// variableBytesCap is where variableBytes stops counting: any size above
// maxFunctionVariableBytes is refused, so a larger count only risks overflow.
const variableBytesCap = maxFunctionVariableBytes + 1

// checkFunctionVariableBytes refuses entry point ep when its function and
// private variables take more than maxFunctionVariableBytes together.
func checkFunctionVariableBytes(m *ir.Module, ep *ir.EntryPoint) error {
	var total uint64
	for _, lv := range ep.Function.LocalVars {
		total = min(total+variableBytes(m, lv.Type), variableBytesCap)
	}
	if total > maxFunctionVariableBytes {
		return unsupported.Errorf("dxil: entry point %q declares more than %d bytes of function and private variables, the most this backend supports",
			ep.Name, maxFunctionVariableBytes)
	}
	return nil
}

// variableBytes is the byte size of a value of type ty by WGSL's layout
// rules, saturating at variableBytesCap: an array counts its stride, or its
// element's size when that is larger, times its length.
func variableBytes(m *ir.Module, ty ir.TypeHandle) uint64 {
	switch t := m.Types[ty].Inner.(type) {
	case ir.ScalarType:
		return uint64(t.Width)
	case ir.AtomicType:
		return uint64(t.Scalar.Width)
	case ir.VectorType:
		return uint64(t.Size) * uint64(t.Scalar.Width)
	case ir.MatrixType:
		rows := uint64(t.Rows)
		if rows == 3 {
			rows = 4 // a column is aligned as a vec4
		}
		return uint64(t.Columns) * rows * uint64(t.Scalar.Width)
	case ir.ArrayType:
		if t.Size.Constant == nil {
			return 0
		}
		elem := max(uint64(t.Stride), variableBytes(m, t.Base))
		return min(elem*uint64(*t.Size.Constant), variableBytesCap)
	case ir.StructType:
		total := uint64(t.Span)
		var members uint64
		for _, member := range t.Members {
			members = min(members+variableBytes(m, member.Type), variableBytesCap)
		}
		return min(max(total, members), variableBytesCap)
	}
	return 0
}
