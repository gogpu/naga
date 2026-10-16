package codegen

import (
	"fmt"

	"github.com/gogpu/naga/ir"
)

// Bounds checks for indices into arrays, vectors, matrices and binding
// arrays, after Rust naga's back/spv/index.rs and proc/index.rs.

// guardedIndex is an index that is either known at translation time or
// computed by an expression (Rust naga's proc::index::GuardedIndex).
type guardedIndex struct {
	known bool
	value uint32              // the index, when known
	expr  ir.ExpressionHandle // the index expression, when not known
}

// guardIndex classifies the index expression handle.
func (e *ExpressionEmitter) guardIndex(handle ir.ExpressionHandle) guardedIndex {
	if v, ok := ir.ConstIndexValue(e.backend.module, e.function, handle); ok {
		return guardedIndex{known: true, value: v}
	}
	return guardedIndex{expr: handle}
}

// boundsCheck is how an index must be used after its bounds check (Rust
// naga's BoundsCheckResult).
type boundsCheck struct {
	// knownInBounds: the index is the constant value.
	knownInBounds bool
	value         uint32
	// id is the index to use when not knownInBounds: the restricted index,
	// the unchecked index, or, with condition, the index to use when the
	// condition holds.
	id uint32
	// condition, when non-zero, is true exactly when the index is in bounds
	// (ReadZeroSkipWrite).
	condition uint32
}

// choosePolicy returns the bounds-check policy for indexing base (Rust
// naga's BoundsCheckPolicies::choose_policy): a binding array uses
// BindingArray, a pointer into a storage or uniform buffer uses Buffer, and
// everything else (other address spaces, values) uses Index.
func (e *ExpressionEmitter) choosePolicy(base ir.ExpressionHandle) (BoundsCheckPolicy, error) {
	res, err := ir.ResolveExpressionType(e.backend.module, e.function, base)
	if err != nil {
		return 0, fmt.Errorf("bounds check: base type: %w", err)
	}
	policies := e.backend.options.BoundsCheckPolicies
	switch t := typeResolutionInner(e.backend.module, res).(type) {
	case ir.BindingArrayType:
		return policies.BindingArray, nil
	case ir.PointerType:
		if t.Space == ir.SpaceStorage || t.Space == ir.SpaceUniform {
			return policies.Buffer, nil
		}
	case ir.ValuePointerType:
		if t.Space == ir.SpaceStorage || t.Space == ir.SpaceUniform {
			return policies.Buffer, nil
		}
	}
	return policies.Index, nil
}

// sequenceLength returns the number of elements of base: known, or the id
// of an instruction computing it (Rust naga's write_sequence_length).
func (e *ExpressionEmitter) sequenceLength(base ir.ExpressionHandle) (known, computed uint32, err error) {
	res, err := ir.ResolveExpressionType(e.backend.module, e.function, base)
	if err != nil {
		return 0, 0, fmt.Errorf("bounds check: base type: %w", err)
	}
	inner := typeResolutionInner(e.backend.module, res)
	length, err := ir.IndexableLengthOf(e.backend.module, inner)
	if err != nil {
		return 0, 0, fmt.Errorf("bounds check: %w", err)
	}
	if !length.Dynamic {
		if length.Known == 0 {
			return 0, 0, fmt.Errorf("bounds check: indexing a zero-length sequence")
		}
		return length.Known, 0, nil
	}
	if isBindingArray(e.backend.module, inner) {
		return 0, 0, fmt.Errorf("bounds check: the length of an unbounded binding array comes from a binding map, " +
			"which spirv.Options does not have; use BoundsCheckUnchecked for binding arrays")
	}
	id, err := e.emitArrayLength(ir.ExprArrayLength{Array: base})
	if err != nil {
		return 0, 0, fmt.Errorf("bounds check: %w", err)
	}
	return 0, id, nil
}

// isBindingArray reports whether inner is a binding array or a pointer to one.
func isBindingArray(module *ir.Module, inner ir.TypeInner) bool {
	switch t := inner.(type) {
	case ir.BindingArrayType:
		return true
	case ir.PointerType:
		if int(t.Base) < len(module.Types) {
			_, ok := module.Types[t.Base].Inner.(ir.BindingArrayType)
			return ok
		}
	}
	return false
}

// writeBoundsCheck applies the policy for indexing base with index (Rust
// naga's write_bounds_check). indexID is the id of the index expression when
// the index is not known.
func (e *ExpressionEmitter) writeBoundsCheck(base ir.ExpressionHandle, index guardedIndex, indexID uint32) (boundsCheck, error) {
	policy, err := e.choosePolicy(base)
	if err != nil {
		return boundsCheck{}, err
	}
	switch policy {
	case BoundsCheckRestrict:
		return e.writeRestrictedIndex(base, index, indexID)
	case BoundsCheckReadZeroSkipWrite:
		return e.writeIndexComparison(base, index, indexID)
	}
	if index.known {
		return boundsCheck{knownInBounds: true, value: index.value}, nil
	}
	return boundsCheck{id: indexID}, nil
}

// writeRestrictedIndex implements Restrict: UMin(index, length - 1), or the
// smaller constant when both are known (Rust naga's write_restricted_index).
// Like Rust, the result is u32 also for an i32 index: the unsigned view maps
// a negative index to the last element.
func (e *ExpressionEmitter) writeRestrictedIndex(base ir.ExpressionHandle, index guardedIndex, indexID uint32) (boundsCheck, error) {
	known, lengthID, err := e.sequenceLength(base)
	if err != nil {
		return boundsCheck{}, err
	}
	var maxID uint32
	if lengthID == 0 {
		if index.known {
			return boundsCheck{knownInBounds: true, value: min(index.value, known-1)}, nil
		}
		if maxID, err = e.u32Constant(known - 1); err != nil {
			return boundsCheck{}, err
		}
	} else {
		one, err := e.u32Constant(1)
		if err != nil {
			return boundsCheck{}, err
		}
		u32Type, err := e.u32Type()
		if err != nil {
			return boundsCheck{}, err
		}
		maxID = e.backend.builder.AddBinaryOp(OpISub, u32Type, lengthID, one)
	}
	if index.known {
		if indexID, err = e.u32Constant(index.value); err != nil {
			return boundsCheck{}, err
		}
	}
	u32Type, err := e.u32Type()
	if err != nil {
		return boundsCheck{}, err
	}
	restricted := e.backend.builder.AddExtInst(u32Type, e.backend.glslExtID, GLSLstd450UMin, indexID, maxID)
	return boundsCheck{id: restricted}, nil
}

// writeIndexComparison implements ReadZeroSkipWrite: OpULessThan(index,
// length), unless both are known and the index is in bounds (Rust naga's
// write_index_comparison). A known out-of-bounds index is still compared.
func (e *ExpressionEmitter) writeIndexComparison(base ir.ExpressionHandle, index guardedIndex, indexID uint32) (boundsCheck, error) {
	known, lengthID, err := e.sequenceLength(base)
	if err != nil {
		return boundsCheck{}, err
	}
	if lengthID == 0 && index.known && index.value < known {
		return boundsCheck{knownInBounds: true, value: index.value}, nil
	}
	if index.known {
		if indexID, err = e.u32Constant(index.value); err != nil {
			return boundsCheck{}, err
		}
	}
	if lengthID == 0 {
		if lengthID, err = e.u32Constant(known); err != nil {
			return boundsCheck{}, err
		}
	}
	boolType, err := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
	if err != nil {
		return boundsCheck{}, err
	}
	condition := e.backend.builder.AddBinaryOp(OpULessThan, boolType, indexID, lengthID)
	return boundsCheck{id: indexID, condition: condition}, nil
}

// u32Type returns the id of the u32 type.
func (e *ExpressionEmitter) u32Type() (uint32, error) {
	return e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarUint, Width: 4})
}

// accessPointer is a pointer built by an access chain (Rust naga's
// ExpressionPointer). With a zero condition the pointer id is ready to use.
// Otherwise the chain has ReadZeroSkipWrite checks: access is the
// OpAccessChain that computes id, and it may run only where condition holds,
// since executing it out of bounds would itself be undefined.
type accessPointer struct {
	id        uint32
	condition uint32
	access    Instruction
}

// chainLinks walks from tip through Access/AccessIndex expressions and
// returns the links (tip first) and the expression they start from.
// stopAtSpill also stops at a composite spilled to a temporary.
func (e *ExpressionEmitter) chainLinks(tip ir.ExpressionHandle, stopAtSpill bool) ([]ir.ExpressionHandle, ir.ExpressionHandle) {
	var links []ir.ExpressionHandle
	current := tip
	for {
		if stopAtSpill {
			if _, spilled := e.spilledComposites[current]; spilled {
				return links, current
			}
		}
		switch k := e.function.Expressions[current].Kind.(type) {
		case ir.ExprAccess:
			links = append(links, current)
			current = k.Base
			continue
		case ir.ExprAccessIndex:
			links = append(links, current)
			current = k.Base
			continue
		}
		return links, current
	}
}

// isStructBase reports whether base is a struct or a pointer to one: its
// members are selected by AccessIndex and never need a bounds check.
func (e *ExpressionEmitter) isStructBase(base ir.ExpressionHandle) bool {
	res, err := ir.ResolveExpressionType(e.backend.module, e.function, base)
	if err != nil {
		return false
	}
	inner := typeResolutionInner(e.backend.module, res)
	if p, ok := inner.(ir.PointerType); ok && int(p.Base) < len(e.backend.module.Types) {
		inner = e.backend.module.Types[p.Base].Inner
	}
	_, ok := inner.(ir.StructType)
	return ok
}

// chainIndices returns the access chain operands for links (tip first), in
// root-to-tip order, and the conjunction of their ReadZeroSkipWrite
// conditions (0 when there is none). Like Rust naga's write_access_chain, the
// checks are written tip first and combined with OpLogicalAnd, without short
// circuit; dynamic index values are evaluated root first beforehand.
func (e *ExpressionEmitter) chainIndices(links []ir.ExpressionHandle) ([]uint32, uint32, error) {
	indexIDs := make(map[ir.ExpressionHandle]uint32)
	for i := len(links) - 1; i >= 0; i-- {
		if a, ok := e.function.Expressions[links[i]].Kind.(ir.ExprAccess); ok {
			if !e.guardIndex(a.Index).known {
				id, err := e.emitExpression(a.Index)
				if err != nil {
					return nil, 0, err
				}
				indexIDs[links[i]] = id
			}
		}
	}

	indices := make([]uint32, len(links))
	var condition uint32
	for i, link := range links {
		var base ir.ExpressionHandle
		var index guardedIndex
		switch k := e.function.Expressions[link].Kind.(type) {
		case ir.ExprAccess:
			base, index = k.Base, e.guardIndex(k.Index)
		case ir.ExprAccessIndex:
			base, index = k.Base, guardedIndex{known: true, value: k.Index}
			if e.isStructBase(base) {
				id, err := e.u32Constant(k.Index)
				if err != nil {
					return nil, 0, err
				}
				indices[len(links)-1-i] = id
				continue
			}
		}
		check, err := e.writeBoundsCheck(base, index, indexIDs[link])
		if err != nil {
			return nil, 0, err
		}
		id := check.id
		if check.knownInBounds {
			if id, err = e.u32Constant(check.value); err != nil {
				return nil, 0, err
			}
		}
		if check.condition != 0 {
			if condition == 0 {
				condition = check.condition
			} else {
				boolType, err := e.backend.emitScalarType(ir.ScalarType{Kind: ir.ScalarBool, Width: 1})
				if err != nil {
					return nil, 0, err
				}
				condition = e.backend.builder.AddBinaryOp(OpLogicalAnd, boolType, condition, check.condition)
			}
		}
		indices[len(links)-1-i] = id
	}
	return indices, condition, nil
}

// buildAccessChain returns the access chain from rootID with indices:
// emitted now when there is no condition, deferred otherwise.
func (e *ExpressionEmitter) buildAccessChain(resultType, rootID uint32, indices []uint32, condition uint32) accessPointer {
	if condition == 0 {
		return accessPointer{id: e.backend.builder.AddAccessChain(resultType, rootID, indices...)}
	}
	id := e.backend.builder.AllocID()
	words := append([]uint32{resultType, id, rootID}, indices...)
	return accessPointer{id: id, condition: condition, access: Instruction{Opcode: OpAccessChain, Words: words}}
}

// writePointerAccess builds the pointer a load, store, atomic or call uses:
// ONE OpAccessChain from the root variable or parameter with every index of
// the Access/AccessIndex chain, as Rust naga's write_access_chain, each index
// bounds-checked by its policy. When the chain indexes a binding array with
// a non-uniform index, the pointer is decorated NonUniform
// (VUID-RuntimeSpirv-NonUniform-06274).
func (e *ExpressionEmitter) writePointerAccess(handle ir.ExpressionHandle) (accessPointer, error) {
	links, root := e.chainLinks(handle, false)
	rootID, err := e.emitPointerRoot(root)
	if err != nil {
		return accessPointer{}, err
	}
	if len(links) == 0 {
		return accessPointer{id: rootID}, nil
	}
	nonUniform := false
	for _, link := range links {
		if a, ok := e.function.Expressions[link].Kind.(ir.ExprAccess); ok {
			nonUniform = nonUniform || e.isNonUniformBindingArrayAccess(a.Base, a.Index)
		}
	}
	indices, condition, err := e.chainIndices(links)
	if err != nil {
		return accessPointer{}, err
	}
	ptrType, err := e.accessChainResultType(handle)
	if err != nil {
		return accessPointer{}, err
	}
	p := e.buildAccessChain(ptrType, rootID, indices, condition)
	if nonUniform {
		e.backend.decorateNonUniformBindingArrayAccess(p.id)
	}
	return p, nil
}

// withAccess runs body with the pointer of p: directly when p is ready, or
// only when p's condition holds, inside a selection (ReadZeroSkipWrite). With
// a non-zero valueType, body's result is merged with zero for the
// out-of-bounds path through OpPhi, and the merged id is returned. Anything
// body uses besides the pointer must be computed before the call, so that it
// dominates its later uses.
func (e *ExpressionEmitter) withAccess(p accessPointer, valueType uint32, body func(pointer uint32) (uint32, error)) (uint32, error) {
	if p.condition == 0 {
		return body(p.id)
	}
	var zero uint32
	if valueType != 0 {
		zero = e.backend.builder.AddConstantNull(valueType)
	}
	sel := e.startSelection(valueType)
	sel.ifTrue(p.condition, zero)
	e.backend.builder.funcAppend(p.access)
	value, err := body(p.id)
	if err != nil {
		return 0, err
	}
	return sel.finish(value), nil
}

// loadAccessedElement loads the element a pointer access selects, as the
// value of an Access/AccessIndex through a pointer. An element that is or
// contains a runtime-sized array cannot be loaded: its pointer is returned
// with done set, for further accesses to index. Under a ReadZeroSkipWrite
// check, neither that pointer nor a texture, sampler or acceleration
// structure (which has no zero value) can be produced, as in Rust naga
// ("Texture array out-of-bounds handling" not implemented).
func (e *ExpressionEmitter) loadAccessedElement(pointer accessPointer, elementType ir.TypeResolution, loadType uint32) (id uint32, done bool, err error) {
	if elementType.Handle != nil && e.backend.typeContainsRuntimeArray(*elementType.Handle) {
		if pointer.condition != 0 {
			return 0, false, fmt.Errorf("a runtime-sized array reached through a ReadZeroSkipWrite-checked index is not supported")
		}
		return pointer.id, true, nil
	}
	if pointer.condition != 0 {
		switch typeResolutionInner(e.backend.module, elementType).(type) {
		case ir.ImageType, ir.SamplerType, ir.AccelerationStructureType, ir.RayQueryType:
			return 0, false, fmt.Errorf("out-of-bounds handling for binding arrays of textures, samplers and " +
				"acceleration structures is not supported with the ReadZeroSkipWrite policy; use Restrict or Unchecked")
		}
	}
	id, err = e.withAccess(pointer, loadType, func(ptr uint32) (uint32, error) {
		return e.backend.builder.AddLoad(loadType, ptr), nil
	})
	return id, false, err
}

// writeVectorAccess indexes a vector value with a computed index (Rust
// naga's write_vector_access): OpCompositeExtract for an index known in
// bounds, OpVectorExtractDynamic otherwise, under ReadZeroSkipWrite only
// when the index is in bounds, with zero otherwise.
func (e *ExpressionEmitter) writeVectorAccess(resultType uint32, access ir.ExprAccess, indexID uint32) (uint32, error) {
	baseID, err := e.emitExpression(access.Base)
	if err != nil {
		return 0, err
	}
	check, err := e.writeBoundsCheck(access.Base, e.guardIndex(access.Index), indexID)
	if err != nil {
		return 0, err
	}
	if check.knownInBounds {
		return e.backend.builder.AddCompositeExtract(resultType, baseID, check.value), nil
	}
	if check.condition == 0 {
		return e.backend.builder.AddVectorExtractDynamic(resultType, baseID, check.id), nil
	}
	zero := e.backend.builder.AddConstantNull(resultType)
	sel := e.startSelection(resultType)
	sel.ifTrue(check.condition, zero)
	element := e.backend.builder.AddVectorExtractDynamic(resultType, baseID, check.id)
	return sel.finish(element), nil
}

// selection builds `if c1 && c2 && ... { body }` with an optional merged
// value, as Rust naga's back/spv/selection.rs: one OpSelectionMerge in the
// header, one OpBranchConditional per condition to the shared merge block,
// and an OpPhi there over every predecessor.
type selection struct {
	e          *ExpressionEmitter
	valueType  uint32 // 0: no merged value
	mergeLabel uint32
	incoming   []uint32 // value, predecessor label pairs
}

func (e *ExpressionEmitter) startSelection(valueType uint32) *selection {
	return &selection{e: e, valueType: valueType}
}

// ifTrue continues in a new block when cond holds; otherwise control goes to
// the merge block, where the merged value is value.
func (s *selection) ifTrue(cond, value uint32) {
	e := s.e
	if s.valueType != 0 {
		s.incoming = append(s.incoming, value, e.currentBlock.LabelID)
	}
	if s.mergeLabel == 0 {
		s.mergeLabel = e.backend.builder.AllocID()
		e.backend.builder.AddSelectionMerge(s.mergeLabel, SelectionControlNone)
	}
	next := e.backend.builder.AllocID()
	e.consumeBlock(Instruction{Opcode: OpBranchConditional, Words: []uint32{cond, next, s.mergeLabel}})
	e.setCurrentBlock(&Block{LabelID: next})
}

// finish branches to the merge block, continues there, and returns the
// merged value (value when every condition held).
func (s *selection) finish(value uint32) uint32 {
	e := s.e
	if s.mergeLabel == 0 {
		return value
	}
	if s.valueType != 0 {
		s.incoming = append(s.incoming, value, e.currentBlock.LabelID)
	}
	e.consumeBlock(makeBranchInstruction(s.mergeLabel))
	e.setCurrentBlock(&Block{LabelID: s.mergeLabel})
	if s.valueType == 0 {
		return 0
	}
	phi := e.backend.builder.AllocID()
	ib := e.newIB()
	ib.AddWord(s.valueType)
	ib.AddWord(phi)
	for _, w := range s.incoming {
		ib.AddWord(w)
	}
	e.backend.builder.funcAppend(ib.Build(OpPhi))
	return phi
}
