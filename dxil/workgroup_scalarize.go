// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

package dxil

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gogpu/naga/dxil/internal/emit"
	"github.com/gogpu/naga/ir"
)

// scalarizeWorkgroupMemory splits every workgroup variable entry point ep
// uses that is not a scalar or an array of scalars into flat scalar
// variables, as DXC lowers groupshared memory (lib/HLSL/DxilLinker.cpp: SROA,
// then DynamicIndexingVectorToArray, then MultiDimArrayToOneDimArray): a
// struct becomes one variable per member, a vector or a matrix an array of
// its scalars (HLMatrixLowerPass names it ".v"), and arrays of those one
// array of all their scalars, with the element index scaled by the number of
// scalars an element takes. Mesa goes further before its emitter
// (dxil_nir_lower_shared_to_var): all shared memory is one array of words.
// An array of structs gives each member an array
// with one element per struct. DXIL has no vectors, and the emitter stores to
// and loads from workgroup memory only scalars and one-dimensional arrays of
// scalars: a workgroup vector used to be refused and a workgroup struct gave
// a malformed record (P1-DXIL-066).
//
// Every pointer into such a variable is a run of accesses from it; a load
// through one becomes the loads of the scalars it covers, composed back into
// its value, and a store the stores of the value's scalars. An index out of
// bounds stays inside the variable once RestrictIndexing clamps the flat
// index, as WGSL requires of an out-of-bounds access.
func scalarizeWorkgroupMemory(m *ir.Module, ep *ir.EntryPoint) error {
	var handles []ir.GlobalVariableHandle
	for h := range reachableGlobalsForEntry(m, ep) {
		gv := &m.GlobalVariables[h]
		if gv.Space == ir.SpaceWorkGroup && !flatWorkgroupType(m, gv.Type) {
			handles = append(handles, h)
		}
	}
	if len(handles) == 0 {
		return nil
	}
	slices.Sort(handles)

	s := &wgScalarizer{
		m:    m,
		fn:   &ep.Function,
		vars: make(map[ir.GlobalVariableHandle]*wgVar),
		locs: make(map[ir.ExpressionHandle]*wgLoc),
		u32:  typeHandle(m, ir.ScalarType{Kind: ir.ScalarUint, Width: 4}),
	}
	for _, h := range handles {
		if err := s.addVar(h); err != nil {
			return fmt.Errorf("dxil: workgroup variable %q: %w", m.GlobalVariables[h].Name, err)
		}
	}
	body, err := s.block(s.fn.Body)
	if err != nil {
		return fmt.Errorf("dxil: workgroup memory: %w", err)
	}
	s.fn.Body = body

	// Every use of a split variable goes through its pieces now.
	for h := range reachableGlobalsForEntry(m, ep) {
		if _, split := s.vars[h]; split {
			return fmt.Errorf("dxil: workgroup variable %q is still used whole after splitting it", m.GlobalVariables[h].Name)
		}
	}
	return nil
}

// flatWorkgroupType reports whether a workgroup variable of type ty stays as
// it is: a scalar, an atomic, or an array of either.
func flatWorkgroupType(m *ir.Module, ty ir.TypeHandle) bool {
	switch t := m.Types[ty].Inner.(type) {
	case ir.ScalarType, ir.AtomicType:
		return true
	case ir.ArrayType:
		switch m.Types[t.Base].Inner.(type) {
		case ir.ScalarType, ir.AtomicType:
			return true
		}
	}
	return false
}

// wgVar is a split workgroup variable: its type and its pieces, by the
// struct members leading to each.
type wgVar struct {
	ty     ir.TypeHandle
	leaves map[string]*wgLeaf
}

// wgLeaf is one piece of a split variable: the variable holding the scalars
// of one struct member (the whole variable without a struct), in every
// element of the arrays of structs above it.
type wgLeaf struct {
	global ir.GlobalVariableHandle
	// dims are the sizes of the arrays of structs above the member, outer
	// first.
	dims []uint32
	// count is the number of scalars of one member value.
	count uint32
	// elem is the type of one scalar: a scalar or an atomic type.
	elem ir.TypeHandle
	// single: the variable is that one scalar, not an array of them.
	single bool
}

// wgLoc is where a pointer into a split variable points: a value of type ty
// reached through the struct members path and the indices outer into arrays
// of structs; once no struct is left in ty, the piece leaf holding it, at
// scalar offset off of a member value.
type wgLoc struct {
	v     *wgVar
	ty    ir.TypeHandle
	path  []uint32
	outer []wgIndex
	leaf  *wgLeaf
	off   wgLinear
}

// wgIndex is an index of an access: the value of expression h when dyn, else
// the constant c.
type wgIndex struct {
	h   ir.ExpressionHandle
	dyn bool
	c   uint32
}

// wgLinear is c plus the sum of each term's index value times its factor.
type wgLinear struct {
	c     uint32
	terms []wgTerm
}

type wgTerm struct {
	h ir.ExpressionHandle
	m uint32
}

// plus returns l plus index i times factor.
func (l wgLinear) plus(i wgIndex, factor uint32) wgLinear {
	if !i.dyn {
		return wgLinear{c: l.c + i.c*factor, terms: l.terms}
	}
	return wgLinear{c: l.c, terms: append(slices.Clone(l.terms), wgTerm{h: i.h, m: factor})}
}

type wgScalarizer struct {
	m    *ir.Module
	fn   *ir.Function
	vars map[ir.GlobalVariableHandle]*wgVar
	// locs memoizes loc: nil for an expression that is not a pointer into a
	// split variable.
	locs map[ir.ExpressionHandle]*wgLoc
	u32  ir.TypeHandle
}

// addVar makes the pieces of workgroup variable h.
func (s *wgScalarizer) addVar(h ir.GlobalVariableHandle) error {
	gv := s.m.GlobalVariables[h]
	v := &wgVar{ty: gv.Type, leaves: make(map[string]*wgLeaf)}
	if err := s.split(v, emit.GroupsharedName(s.m, &gv), gv.Type, nil, nil); err != nil {
		return err
	}
	s.vars[h] = v
	return nil
}

// split makes the pieces of a value of type ty reached through the struct
// members path, below arrays of structs of sizes dims.
func (s *wgScalarizer) split(v *wgVar, name string, ty ir.TypeHandle, dims, path []uint32) error {
	switch t := s.m.Types[ty].Inner.(type) {
	case ir.StructType:
		for i, member := range t.Members {
			if err := s.split(v, name, member.Type, dims, append(slices.Clone(path), uint32(i))); err != nil {
				return err
			}
		}
		return nil
	case ir.ArrayType:
		if s.holdsStruct(t.Base) {
			if t.Size.Constant == nil {
				return fmt.Errorf("an array without a constant size")
			}
			return s.split(v, name, t.Base, append(slices.Clone(dims), *t.Size.Constant), path)
		}
	}
	return s.addLeaf(v, name, ty, dims, path)
}

// addLeaf adds the variable holding the scalars of the struct member path,
// a value of type ty, for every element of the arrays of structs dims.
func (s *wgScalarizer) addLeaf(v *wgVar, name string, ty ir.TypeHandle, dims, path []uint32) error {
	sh, err := s.shape(ty)
	if err != nil {
		return err
	}
	total := sh.count
	for _, d := range dims {
		total *= d
	}

	leaf := &wgLeaf{dims: dims, count: sh.count, elem: sh.elem}
	varType := ty
	if total == 1 && len(dims) == 0 && sh.arrays == 0 && sh.vectors == 0 {
		leaf.single = true
	} else {
		varType = s.arrayOf(sh.elem, total)
	}
	leaf.global = ir.GlobalVariableHandle(len(s.m.GlobalVariables)) //nolint:gosec // global count fits uint32 by IR design
	s.m.GlobalVariables = append(s.m.GlobalVariables, ir.GlobalVariable{
		Name:  name + leafSuffix(path, sh.vectors, len(dims)+sh.arrays),
		Space: ir.SpaceWorkGroup,
		Type:  varType,
	})
	v.leaves[pathKey(path)] = leaf
	return nil
}

// leafSuffix is the suffix DXC gives a piece of a groupshared variable: the
// struct members leading to it, ".v" for each level of vector it turned into
// an array (a matrix is two), and ".1dim" when it flattened more than one
// dimension into one ("tile.v.1dim" for an array of vectors, "m.v.v" for a
// matrix, "s.2.1dim" for an array of arrays member).
func leafSuffix(path []uint32, vectors, arrays int) string {
	var b strings.Builder
	for _, i := range path {
		b.WriteString(".")
		b.WriteString(strconv.FormatUint(uint64(i), 10))
	}
	b.WriteString(strings.Repeat(".v", vectors))
	if arrays >= 2 || (arrays >= 1 && vectors >= 1) {
		b.WriteString(".1dim")
	}
	return b.String()
}

func pathKey(path []uint32) string {
	var b strings.Builder
	for _, i := range path {
		b.WriteString(strconv.FormatUint(uint64(i), 10))
		b.WriteString(".")
	}
	return b.String()
}

// wgShape is the shape of a value that holds no struct: the type of its
// scalars, how many it has, and how many levels of vectors (a matrix is two)
// and of arrays it has.
type wgShape struct {
	elem    ir.TypeHandle
	count   uint32
	vectors int
	arrays  int
}

// shape returns the shape of a value of type ty, which holds no struct.
func (s *wgScalarizer) shape(ty ir.TypeHandle) (wgShape, error) {
	switch t := s.m.Types[ty].Inner.(type) {
	case ir.ScalarType, ir.AtomicType:
		return wgShape{elem: ty, count: 1}, nil
	case ir.VectorType:
		return wgShape{elem: typeHandle(s.m, t.Scalar), count: uint32(t.Size), vectors: 1}, nil
	case ir.MatrixType:
		return wgShape{elem: typeHandle(s.m, t.Scalar), count: uint32(t.Columns) * uint32(t.Rows), vectors: 2}, nil
	case ir.ArrayType:
		if t.Size.Constant == nil {
			return wgShape{}, fmt.Errorf("an array without a constant size")
		}
		sh, err := s.shape(t.Base)
		sh.count *= *t.Size.Constant
		sh.arrays++
		return sh, err
	default:
		return wgShape{}, fmt.Errorf("a value of type %T in workgroup memory", t)
	}
}

// holdsStruct reports whether a value of type ty has a struct in it.
func (s *wgScalarizer) holdsStruct(ty ir.TypeHandle) bool {
	switch t := s.m.Types[ty].Inner.(type) {
	case ir.StructType:
		return true
	case ir.ArrayType:
		return s.holdsStruct(t.Base)
	}
	return false
}

// arrayOf returns the type of an array of n elements of type elem.
func (s *wgScalarizer) arrayOf(elem ir.TypeHandle, n uint32) ir.TypeHandle {
	var width uint32
	switch t := s.m.Types[elem].Inner.(type) {
	case ir.ScalarType:
		width = uint32(t.Width)
	case ir.AtomicType:
		width = uint32(t.Scalar.Width)
	}
	for i, t := range s.m.Types {
		if a, ok := t.Inner.(ir.ArrayType); ok && t.Name == "" && a.Base == elem && a.Stride == width &&
			a.Size.Constant != nil && *a.Size.Constant == n {
			return ir.TypeHandle(i)
		}
	}
	size := n
	s.m.Types = append(s.m.Types, ir.Type{Inner: ir.ArrayType{Base: elem, Size: ir.ArraySize{Constant: &size}, Stride: width}})
	return ir.TypeHandle(len(s.m.Types) - 1) //nolint:gosec // type count fits uint32 by IR design
}

// loc returns where expression h points to when it is a pointer into a
// split variable, else nil.
func (s *wgScalarizer) loc(h ir.ExpressionHandle) *wgLoc {
	if l, ok := s.locs[h]; ok {
		return l
	}
	var l *wgLoc
	switch k := s.fn.Expressions[h].Kind.(type) {
	case ir.ExprGlobalVariable:
		if v, ok := s.vars[k.Variable]; ok {
			l = s.enter(&wgLoc{v: v, ty: v.ty})
		}
	case ir.ExprAccessIndex:
		if base := s.loc(k.Base); base != nil {
			l = s.step(base, wgIndex{c: k.Index})
		}
	case ir.ExprAccess:
		if base := s.loc(k.Base); base != nil {
			l = s.step(base, wgIndex{h: k.Index, dyn: true})
		}
	}
	s.locs[h] = l
	return l
}

// enter sets l's piece once no struct is left in l's type.
func (s *wgScalarizer) enter(l *wgLoc) *wgLoc {
	if l.leaf == nil && !s.holdsStruct(l.ty) {
		l.leaf = l.v.leaves[pathKey(l.path)]
	}
	return l
}

// step returns where element i of the value l points to is.
func (s *wgScalarizer) step(l *wgLoc, i wgIndex) *wgLoc {
	n := &wgLoc{v: l.v, path: l.path, outer: l.outer, leaf: l.leaf, off: l.off}
	switch t := s.m.Types[l.ty].Inner.(type) {
	case ir.StructType:
		if i.dyn || int(i.c) >= len(t.Members) {
			return nil
		}
		n.ty = t.Members[i.c].Type
		n.path = append(slices.Clone(l.path), i.c)
	case ir.ArrayType:
		n.ty = t.Base
		if l.leaf == nil {
			n.outer = append(slices.Clone(l.outer), i)
		} else {
			n.off = l.off.plus(i, s.scalarCount(t.Base))
		}
	case ir.VectorType:
		n.ty = typeHandle(s.m, t.Scalar)
		n.off = l.off.plus(i, 1)
	case ir.MatrixType:
		n.ty = typeHandle(s.m, ir.VectorType{Size: t.Rows, Scalar: t.Scalar})
		n.off = l.off.plus(i, uint32(t.Rows))
	default:
		return nil
	}
	return s.enter(n)
}

// scalarCount is the number of scalars of a value of type ty, which holds no
// struct.
func (s *wgScalarizer) scalarCount(ty ir.TypeHandle) uint32 {
	sh, _ := s.shape(ty) // the variable's pieces were made from these types
	return sh.count
}

// elementCount is the number of elements of a composite of type ty: members,
// array elements, components or columns.
func (s *wgScalarizer) elementCount(ty ir.TypeHandle) (uint32, bool) {
	switch t := s.m.Types[ty].Inner.(type) {
	case ir.StructType:
		return uint32(len(t.Members)), true //nolint:gosec // member count fits uint32 by IR design
	case ir.ArrayType:
		return *t.Size.Constant, true
	case ir.VectorType:
		return uint32(t.Size), true
	case ir.MatrixType:
		return uint32(t.Columns), true
	}
	return 0, false
}

// scalarPtr returns a pointer to the scalar l points to.
func (s *wgScalarizer) scalarPtr(l *wgLoc) ir.ExpressionHandle {
	leaf := l.leaf
	g := s.add(ir.ExprGlobalVariable{Variable: leaf.global}, s.pointer(s.m.GlobalVariables[leaf.global].Type))
	if leaf.single {
		return g
	}
	index := wgLinear{}
	stride := leaf.count
	for i := len(l.outer) - 1; i >= 0; i-- {
		index = index.plus(l.outer[i], stride)
		stride *= leaf.dims[i]
	}
	index.c += l.off.c
	index.terms = append(index.terms, l.off.terms...)
	return s.add(ir.ExprAccess{Base: g, Index: s.linear(index)}, s.pointer(leaf.elem))
}

// linear returns an expression computing l, as a u32.
func (s *wgScalarizer) linear(l wgLinear) ir.ExpressionHandle {
	sum := ir.ExpressionHandle(0)
	have := false
	addTerm := func(h ir.ExpressionHandle) {
		if have {
			h = s.add(ir.ExprBinary{Op: ir.BinaryAdd, Left: sum, Right: h}, s.value(s.u32))
		}
		sum, have = h, true
	}
	for _, t := range l.terms {
		h := s.asU32(t.h)
		if t.m != 1 {
			h = s.add(ir.ExprBinary{Op: ir.BinaryMultiply, Left: h, Right: s.literal(t.m)}, s.value(s.u32))
		}
		addTerm(h)
	}
	if l.c != 0 || !have {
		addTerm(s.literal(l.c))
	}
	return sum
}

// asU32 returns index h as a u32: a signed index's bits.
func (s *wgScalarizer) asU32(h ir.ExpressionHandle) ir.ExpressionHandle {
	if sc, ok := scalarOf(s.m, s.fn.ExpressionTypes[h]); ok && sc.Kind == ir.ScalarSint {
		return s.add(ir.ExprAs{Expr: h, Kind: ir.ScalarUint}, s.value(s.u32))
	}
	return h
}

// loadKind returns the expression loading the value l points to: the load of
// its scalar, or the composite of the loads of its elements.
func (s *wgScalarizer) loadKind(l *wgLoc) ir.ExpressionKind {
	n, composite := s.elementCount(l.ty)
	if !composite {
		return ir.ExprLoad{Pointer: s.scalarPtr(l)}
	}
	components := make([]ir.ExpressionHandle, n)
	for i := range components {
		e := s.step(l, wgIndex{c: uint32(i)})
		components[i] = s.add(s.loadKind(e), s.value(s.valueType(e.ty)))
	}
	return ir.ExprCompose{Type: l.ty, Components: components}
}

// valueType is the type of a value loaded from a ty: an atomic's scalar.
func (s *wgScalarizer) valueType(ty ir.TypeHandle) ir.TypeHandle {
	if a, ok := s.m.Types[ty].Inner.(ir.AtomicType); ok {
		return typeHandle(s.m, a.Scalar)
	}
	return ty
}

// store returns the statements storing value to where l points.
func (s *wgScalarizer) store(l *wgLoc, value ir.ExpressionHandle) ir.Block {
	n, composite := s.elementCount(l.ty)
	if !composite {
		return ir.Block{{Kind: ir.StmtStore{Pointer: s.scalarPtr(l), Value: value}}}
	}
	var b ir.Block
	for i := uint32(0); i < n; i++ {
		e := s.step(l, wgIndex{c: i})
		b = append(b, s.store(e, s.element(value, i, s.valueType(e.ty)))...)
	}
	return b
}

// element returns element i, of type ty, of composite value v: the operand
// it is made of when v is composed of elements of that type, a zero for a
// zero value, else an access of v.
func (s *wgScalarizer) element(v ir.ExpressionHandle, i uint32, ty ir.TypeHandle) ir.ExpressionHandle {
	switch k := s.fn.Expressions[v].Kind.(type) {
	case ir.ExprZeroValue:
		return s.add(ir.ExprZeroValue{Type: ty}, s.value(ty))
	case ir.ExprSplat:
		return k.Value
	case ir.ExprCompose:
		if int(i) < len(k.Components) && len(k.Components) == s.lenOf(k.Type) && s.hasType(k.Components[i], ty) {
			return k.Components[i]
		}
	}
	return s.add(ir.ExprAccessIndex{Base: v, Index: i}, s.value(ty))
}

func (s *wgScalarizer) lenOf(ty ir.TypeHandle) int {
	n, _ := s.elementCount(ty)
	return int(n)
}

// hasType reports whether expression h has type ty. Types other than
// scalars, vectors and matrices are the same only by handle (a struct's
// inner type is not comparable).
func (s *wgScalarizer) hasType(h ir.ExpressionHandle, ty ir.TypeHandle) bool {
	res := s.fn.ExpressionTypes[h]
	inner := res.Value
	if res.Handle != nil {
		if *res.Handle == ty {
			return true
		}
		inner = s.m.Types[*res.Handle].Inner
	}
	switch want := s.m.Types[ty].Inner.(type) {
	case ir.ScalarType:
		got, ok := inner.(ir.ScalarType)
		return ok && got == want
	case ir.VectorType:
		got, ok := inner.(ir.VectorType)
		return ok && got == want
	case ir.MatrixType:
		got, ok := inner.(ir.MatrixType)
		return ok && got == want
	}
	return false
}

// block rewrites the uses of split variables in b and the blocks in it.
func (s *wgScalarizer) block(b ir.Block) (ir.Block, error) {
	out := make(ir.Block, 0, len(b))
	for _, st := range b {
		var err error
		if out, err = s.statement(out, st); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// statement returns out with st appended, rewritten when it goes through a
// pointer into a split variable.
func (s *wgScalarizer) statement(out ir.Block, st ir.Statement) (ir.Block, error) {
	switch k := st.Kind.(type) {
	case ir.StmtEmit:
		return s.emit(out, k.Range)
	case ir.StmtStore:
		if l := s.loc(k.Pointer); l != nil {
			start := s.next()
			stores := s.store(l, k.Value)
			return append(append(out, s.emitsFrom(start)...), stores...), nil
		}
	case ir.StmtAtomic:
		if l := s.loc(k.Pointer); l != nil {
			start := s.next()
			k.Pointer = s.scalarPtr(l)
			return append(append(out, s.emitsFrom(start)...), ir.Statement{Kind: k}), nil
		}
	case ir.StmtWorkGroupUniformLoad:
		if l := s.loc(k.Pointer); l != nil {
			return append(out, s.uniformLoad(l, k)...), nil
		}
	case ir.StmtIf, ir.StmtSwitch, ir.StmtLoop, ir.StmtBlock:
		kind, err := s.nested(k)
		return append(out, ir.Statement{Kind: kind}), err
	default:
		var err error
		walkStmtExprs(s.fn, k, func(_ *ir.Function, h ir.ExpressionHandle) {
			if err == nil && s.loc(h) != nil {
				err = fmt.Errorf("a pointer into workgroup variable used by %T", k)
			}
		})
		if err != nil {
			return nil, err
		}
	}
	return append(out, st), nil
}

// nested returns a statement holding blocks with its blocks rewritten.
func (s *wgScalarizer) nested(kind ir.StatementKind) (ir.StatementKind, error) {
	var err error
	switch k := kind.(type) {
	case ir.StmtIf:
		if k.Accept, err = s.block(k.Accept); err == nil {
			k.Reject, err = s.block(k.Reject)
		}
		return k, err
	case ir.StmtSwitch:
		k.Cases = slices.Clone(k.Cases)
		for i := range k.Cases {
			if k.Cases[i].Body, err = s.block(k.Cases[i].Body); err != nil {
				return nil, err
			}
		}
		return k, nil
	case ir.StmtLoop:
		if k.Body, err = s.block(k.Body); err == nil {
			k.Continuing, err = s.block(k.Continuing)
		}
		return k, err
	case ir.StmtBlock:
		k.Block, err = s.block(k.Block)
		return k, err
	}
	return kind, nil
}

// uniformLoad returns the statements of a workgroupUniformLoad through a
// pointer into a split variable: of a scalar, the load through a pointer to
// its piece; of a composite, the loads of its scalars between the two
// barriers the load has.
func (s *wgScalarizer) uniformLoad(l *wgLoc, k ir.StmtWorkGroupUniformLoad) ir.Block {
	start := s.next()
	if _, composite := s.elementCount(l.ty); !composite {
		k.Pointer = s.scalarPtr(l)
		return append(s.emitsFrom(start), ir.Statement{Kind: k})
	}
	kind := s.loadKind(l)
	emits := s.emitsFrom(start)
	s.fn.Expressions[k.Result].Kind = kind
	barrier := ir.Statement{Kind: ir.StmtBarrier{Flags: ir.BarrierWorkGroup}}
	b := append(ir.Block{barrier}, emits...)
	return append(b,
		ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: k.Result, End: k.Result + 1}}},
		barrier)
}

// emit returns out with the Emit of r: without the pointers into split
// variables, and with each load through one rewritten into the loads of its
// pieces, emitted before it.
func (s *wgScalarizer) emit(out ir.Block, r ir.Range) (ir.Block, error) {
	segment := r.Start
	flush := func(end ir.ExpressionHandle) {
		if end > segment {
			out = append(out, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: segment, End: end}}})
		}
	}
	for h := r.Start; h < r.End; h++ {
		if s.loc(h) != nil {
			flush(h)
			segment = h + 1
			continue
		}
		kind := s.fn.Expressions[h].Kind
		if load, ok := kind.(ir.ExprLoad); ok {
			if l := s.loc(load.Pointer); l != nil {
				flush(h)
				start := s.next()
				newKind := s.loadKind(l)
				out = append(out, s.emitsFrom(start)...)
				s.fn.Expressions[h].Kind = newKind
				segment = h
				continue
			}
		}
		var err error
		visitExprSubHandles(kind, func(sub ir.ExpressionHandle) {
			if err == nil && s.loc(sub) != nil {
				err = fmt.Errorf("a pointer into workgroup variable used by %T", kind)
			}
		})
		if err != nil {
			return nil, err
		}
	}
	flush(r.End)
	return out, nil
}

// next is the handle the next added expression gets.
func (s *wgScalarizer) next() ir.ExpressionHandle {
	return ir.ExpressionHandle(len(s.fn.Expressions)) //nolint:gosec // expression count fits uint32 by IR design
}

// emitsFrom returns the Emits of the expressions added since start, leaving
// out those that are never emitted (literals, variables, zero values).
func (s *wgScalarizer) emitsFrom(start ir.ExpressionHandle) ir.Block {
	var b ir.Block
	end := s.next()
	for h := start; h < end; {
		if isPreEmit(s.fn.Expressions[h].Kind) {
			h++
			continue
		}
		first := h
		for h < end && !isPreEmit(s.fn.Expressions[h].Kind) {
			h++
		}
		b = append(b, ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{Start: first, End: h}}})
	}
	return b
}

// isPreEmit reports whether an expression of this kind is in scope without
// an Emit, as Rust naga's Expression::needs_pre_emit.
func isPreEmit(kind ir.ExpressionKind) bool {
	switch kind.(type) {
	case ir.Literal, ir.ExprConstant, ir.ExprOverride, ir.ExprZeroValue,
		ir.ExprGlobalVariable, ir.ExprFunctionArgument, ir.ExprLocalVariable:
		return true
	}
	return false
}

func (s *wgScalarizer) add(kind ir.ExpressionKind, res ir.TypeResolution) ir.ExpressionHandle {
	return appendFnExpr(s.fn, kind, res)
}

func (s *wgScalarizer) literal(v uint32) ir.ExpressionHandle {
	return s.add(ir.Literal{Value: ir.LiteralU32(v)}, s.value(s.u32))
}

func (s *wgScalarizer) value(ty ir.TypeHandle) ir.TypeResolution {
	return ir.TypeResolution{Handle: &ty}
}

func (s *wgScalarizer) pointer(base ir.TypeHandle) ir.TypeResolution {
	return ir.TypeResolution{Value: ir.PointerType{Base: base, Space: ir.SpaceWorkGroup}}
}
