package front

import (
	"sort"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// parseFunction parses one OpFunction through OpFunctionEnd into
// Module.Functions, and records the entry point that uses it for its wrapper.
func (p *parser) parseFunction() error {
	p.lookupExpression = make(map[uint32]lookupExpression)

	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	funID, err := p.next()
	if err != nil {
		return err
	}
	// The function control word is not carried into the IR.
	if _, err := p.next(); err != nil {
		return err
	}
	funTypeID, err := p.next()
	if err != nil {
		return err
	}

	funType, ok := p.lookupFunctionType[funTypeID]
	if !ok {
		return errInvalidID(funTypeID)
	}
	if funType.returnTypeID != resultTypeID {
		return errWrongFunctionResultType(resultTypeID)
	}

	dec := p.takeDecoration(funID)
	fn := ir.Function{Name: dec.name}
	if !(p.hasVoidType && p.voidTypeID == resultTypeID) {
		result, err := p.typeLookup(resultTypeID)
		if err != nil {
			return err
		}
		fn.Result = &ir.FunctionResult{Type: result.handle}
	}

	for i, parameterTypeID := range funType.parameterTypeIDs {
		inst, err := p.nextInstruction()
		if err != nil {
			return err
		}
		if inst.op != spvenums.OpFunctionParameter || inst.wc != 3 {
			return errInvalidParameter(inst.op)
		}
		typeID, err := p.next()
		if err != nil {
			return err
		}
		id, err := p.next()
		if err != nil {
			return err
		}
		handle := appendExpression(&fn, ir.ExprFunctionArgument{Index: uint32(i)})
		p.lookupExpression[id] = lookupExpression{handle: handle, typeID: typeID}
		if typeID != parameterTypeID {
			return errWrongFunctionArgumentType(typeID)
		}
		ty, err := p.typeLookup(typeID)
		if err != nil {
			return err
		}
		argumentDec := p.takeDecoration(id)
		fn.Arguments = append(fn.Arguments, ir.FunctionArgument{Name: argumentDec.name, Type: ty.handle})
	}

	fs := &functionState{
		id:           funID,
		fn:           &fn,
		blocks:       make(map[uint32]ir.Block),
		bodyForLabel: make(map[uint32]int),
		bodies:       []body{{parent: 0}},
	}
	p.makeExpressionStorage(fs)

	// The function's blocks: OpLabel starts one, OpFunctionEnd closes the
	// function.
parameters:
	for {
		inst, err := p.nextInstruction()
		if err != nil {
			return err
		}
		switch inst.op {
		case spvenums.OpLine:
			if err := inst.expect(4); err != nil {
				return err
			}
			for range 3 {
				if _, err := p.next(); err != nil {
					return err
				}
			}
		case spvenums.OpLabel:
			if err := inst.expect(2); err != nil {
				return err
			}
			blockID, err := p.next()
			if err != nil {
				return err
			}
			if err := p.nextBlock(blockID, fs); err != nil {
				return err
			}
		case spvenums.OpFunctionEnd:
			if err := inst.expect(1); err != nil {
				return err
			}
			break parameters
		case spvenums.OpExtInst:
			// Only an ignorable non-semantic instruction may appear here.
			if _, err := inst.expectAtLeast(4); err != nil {
				return err
			}
			if _, err := p.next(); err != nil { // result type
				return err
			}
			if _, err := p.next(); err != nil { // result id
				return err
			}
			setID, err := p.next()
			if err != nil {
				return err
			}
			if !p.hasExtNonSemantic || setID != p.extNonSemanticID {
				return errUnsupportedInstruction(p.state, inst.op)
			}
			for range inst.wc - 4 {
				if _, err := p.next(); err != nil {
					return err
				}
			}
		default:
			return errUnsupportedInstruction(p.state, inst.op)
		}
	}

	if err := p.appendPhiStores(fs); err != nil {
		return err
	}
	fn.Body = fs.lower()

	handle := ir.FunctionHandle(len(p.module.Functions))
	p.module.Functions = append(p.module.Functions, fn)
	p.lookupFunction[funID] = handle
	if _, ok := p.entryPoints[funID]; ok {
		p.deferredEntryPoints = append(p.deferredEntryPoints, funID)
	}
	return nil
}

// makeExpressionStorage puts the module's globals and constants into the
// function's expression arena, so that instructions can refer to them by id
// from the start, as Rust naga's make_expression_storage. The ids are visited
// in ascending order, so the arena is reproducible from run to run.
func (p *parser) makeExpressionStorage(fs *functionState) {
	variableIDs := make([]uint32, 0, len(p.lookupVariable))
	for id := range p.lookupVariable {
		variableIDs = append(variableIDs, id)
	}
	sort.Slice(variableIDs, func(i, j int) bool { return variableIDs[i] < variableIDs[j] })
	for _, id := range variableIDs {
		variable := p.lookupVariable[id]
		handle := appendExpression(fs.fn, ir.ExprGlobalVariable{Variable: variable.handle})
		p.lookupExpression[id] = lookupExpression{handle: handle, typeID: variable.typeID}
	}

	constantIDs := make([]uint32, 0, len(p.lookupConstant))
	for id := range p.lookupConstant {
		constantIDs = append(constantIDs, id)
	}
	sort.Slice(constantIDs, func(i, j int) bool { return constantIDs[i] < constantIDs[j] })
	for _, id := range constantIDs {
		constant := p.lookupConstant[id]
		handle := appendExpression(fs.fn, constant.kind.expressionKind())
		p.lookupExpression[id] = lookupExpression{handle: handle, typeID: constant.typeID}
	}
}

// appendPhiStores emits the stores that complete every phi: each predecessor
// block gets a store of its value into the phi's local variable, spilling the
// value to a temporary when it is out of scope there (Rust naga's phi
// processing in parse_function).
func (p *parser) appendPhiStores(fs *functionState) error {
	for _, phi := range fs.phis {
		phiPointer := appendExpression(fs.fn, ir.ExprLocalVariable{Variable: phi.local})
		for _, source := range phi.expressions {
			lookup, ok := p.lookupExpression[source.id]
			if !ok {
				return errInvalidID(uint32(source.id))
			}
			predecessorBody, ok := fs.bodyForLabel[source.blockID]
			if !ok {
				predecessorBody = 0
			}
			sourceBody := 0
			if idx, ok := fs.bodyForLabel[lookup.blockID]; ok {
				sourceBody = idx
			}
			value := lookup.handle
			if !isParent(predecessorBody, sourceBody, fs) {
				ty, err := p.typeLookup(lookup.typeID)
				if err != nil {
					return err
				}
				fs.fn.LocalVars = append(fs.fn.LocalVars, ir.LocalVariable{Type: ty.handle})
				local := ir.LocalVariableHandle(len(fs.fn.LocalVars) - 1)
				pointer := appendExpression(fs.fn, ir.ExprLocalVariable{Variable: local})

				start := len(fs.fn.Expressions)
				value = appendExpression(fs.fn, ir.ExprLoad{Pointer: pointer})
				emit := ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{
					Start: ir.ExpressionHandle(start),
					End:   ir.ExpressionHandle(len(fs.fn.Expressions)),
				}}}
				predecessor := fs.blocks[source.blockID]
				predecessor = append(predecessor, emit)
				fs.blocks[source.blockID] = predecessor

				defining := fs.blocks[lookup.blockID]
				defining = append(defining, ir.Statement{Kind: ir.StmtStore{Pointer: pointer, Value: lookup.handle}})
				fs.blocks[lookup.blockID] = defining
			}
			predecessor := fs.blocks[source.blockID]
			predecessor = append(predecessor, ir.Statement{Kind: ir.StmtStore{Pointer: phiPointer, Value: value}})
			fs.blocks[source.blockID] = predecessor
		}
	}
	return nil
}

// nextBlock parses one OpLabel block. Phase 1 supports straight-line blocks:
// control-flow instructions are reported as unsupported until they are
// implemented.
func (p *parser) nextBlock(blockID uint32, fs *functionState) error {
	var consumer emitter
	consumer.startWith(fs.fn.Expressions)
	bodyIdx := 0
	if idx, ok := fs.bodyForLabel[blockID]; ok {
		bodyIdx = idx
	}

	var block ir.Block
	var terminator *ir.Statement
	for {
		inst, err := p.nextInstruction()
		if err != nil {
			return err
		}
		done, statement, err := p.blockInstruction(inst, fs, &consumer, &block, bodyIdx, blockID)
		if err != nil {
			return err
		}
		if done {
			terminator = statement
			break
		}
	}

	if statement, ok := consumer.finish(fs.fn.Expressions); ok {
		block = append(block, statement)
	}
	if terminator != nil {
		block = append(block, *terminator)
	}
	fs.blocks[blockID] = block
	fs.bodies[bodyIdx].fragments = append(fs.bodies[bodyIdx].fragments, bodyFragment{
		kind:    fragmentBlock,
		blockID: blockID,
	})
	return nil
}

// lookupExpr returns the expression table entry of an id.
func (p *parser) lookupExpr(id uint32) (lookupExpression, error) {
	lookup, ok := p.lookupExpression[id]
	if !ok {
		return lookupExpression{}, errInvalidID(id)
	}
	return lookup, nil
}

// handle returns the expression an id is referred to by in the current body.
func (p *parser) handleOf(id uint32, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int) (ir.ExpressionHandle, error) {
	lookup, err := p.lookupExpr(id)
	if err != nil {
		return 0, err
	}
	return p.getExprHandle(id, lookup, fs, consumer, block, bodyIdx)
}

// storeExpression records the expression one instruction produced.
func (p *parser) storeExpression(id, typeID uint32, blockID uint32, handle ir.ExpressionHandle) {
	p.lookupExpression[id] = lookupExpression{handle: handle, typeID: typeID, blockID: blockID}
}

// flushBeforeStatement puts the expressions collected so far into an Emit
// statement before a statement that uses them.
func flushBeforeStatement(consumer *emitter, expressions []ir.Expression, block *ir.Block) {
	if statement, ok := consumer.finish(expressions); ok {
		*block = append(*block, statement)
	}
}

// blockInstruction handles one instruction inside OpLabel. It reports whether
// the instruction ended the block, and the terminating statement if it has
// one.
func (p *parser) blockInstruction(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) (bool, *ir.Statement, error) {
	switch inst.op {
	case spvenums.OpLine:
		if err := inst.expect(4); err != nil {
			return false, nil, err
		}
		for range 3 {
			if _, err := p.next(); err != nil {
				return false, nil, err
			}
		}
	case spvenums.OpNoLine:
		if err := inst.expect(1); err != nil {
			return false, nil, err
		}
	case spvenums.OpUndef:
		if err := inst.expect(3); err != nil {
			return false, nil, err
		}
		typeID, err := p.next()
		if err != nil {
			return false, nil, err
		}
		id, err := p.next()
		if err != nil {
			return false, nil, err
		}
		ty, err := p.typeLookup(typeID)
		if err != nil {
			return false, nil, err
		}
		p.storeExpression(id, typeID, blockID, appendExpression(fs.fn, ir.ExprZeroValue{Type: ty.handle}))
	case spvenums.OpVariable:
		return false, nil, p.localVariable(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpAccessChain, spvenums.OpInBoundsAccessChain:
		return false, nil, p.accessChain(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpCompositeExtract:
		return false, nil, p.compositeExtract(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpCompositeConstruct:
		return false, nil, p.compositeConstruct(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpLoad:
		return false, nil, p.load(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpStore:
		return false, nil, p.store(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpCopyObject:
		if err := inst.expect(4); err != nil {
			return false, nil, err
		}
		typeID, err := p.next()
		if err != nil {
			return false, nil, err
		}
		id, err := p.next()
		if err != nil {
			return false, nil, err
		}
		operandID, err := p.next()
		if err != nil {
			return false, nil, err
		}
		handle, err := p.handleOf(operandID, fs, consumer, block, bodyIdx)
		if err != nil {
			return false, nil, err
		}
		p.storeExpression(id, typeID, blockID, handle)
	case spvenums.OpSNegate, spvenums.OpFNegate:
		return false, nil, p.unaryOp(inst, fs, consumer, block, bodyIdx, blockID, ir.UnaryNegate, true)
	case spvenums.OpLogicalNot:
		return false, nil, p.unaryOp(inst, fs, consumer, block, bodyIdx, blockID, ir.UnaryLogicalNot, false)
	case spvenums.OpNot:
		return false, nil, p.unaryOp(inst, fs, consumer, block, bodyIdx, blockID, ir.UnaryBitwiseNot, false)
	case spvenums.OpIAdd, spvenums.OpISub, spvenums.OpIMul,
		spvenums.OpBitwiseOr, spvenums.OpBitwiseXor, spvenums.OpBitwiseAnd,
		spvenums.OpSDiv, spvenums.OpSRem,
		spvenums.OpIEqual, spvenums.OpINotEqual:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		op, err := mapBinaryOperator(inst.op)
		if err != nil {
			return false, nil, err
		}
		anchor := signAnchorResult
		if inst.op == spvenums.OpIEqual || inst.op == spvenums.OpINotEqual {
			anchor = signAnchorOperand
		}
		return false, nil, p.binaryOpSignAdjusted(fs, consumer, block, bodyIdx, blockID, op, anchor)
	case spvenums.OpFAdd, spvenums.OpFSub, spvenums.OpFMul,
		spvenums.OpUDiv, spvenums.OpFDiv, spvenums.OpUMod, spvenums.OpFRem,
		spvenums.OpLogicalOr, spvenums.OpLogicalAnd,
		spvenums.OpFOrdEqual, spvenums.OpFUnordEqual, spvenums.OpFOrdNotEqual, spvenums.OpFUnordNotEqual,
		spvenums.OpFOrdLessThan, spvenums.OpFUnordLessThan, spvenums.OpFOrdGreaterThan, spvenums.OpFUnordGreaterThan,
		spvenums.OpFOrdLessThanEqual, spvenums.OpFUnordLessThanEqual,
		spvenums.OpFOrdGreaterThanEqual, spvenums.OpFUnordGreaterThanEqual,
		spvenums.OpLogicalEqual, spvenums.OpLogicalNotEqual,
		spvenums.OpVectorTimesScalar, spvenums.OpVectorTimesMatrix,
		spvenums.OpMatrixTimesScalar, spvenums.OpMatrixTimesVector, spvenums.OpMatrixTimesMatrix:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		op, err := mapBinaryOperator(inst.op)
		if err != nil {
			return false, nil, err
		}
		return false, nil, p.binaryOp(fs, consumer, block, bodyIdx, blockID, op)
	case spvenums.OpSMod:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		return false, nil, p.signedModulo(fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpFMod:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		return false, nil, p.floatModulo(fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpShiftLeftLogical, spvenums.OpShiftRightLogical, spvenums.OpShiftRightArithmetic:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		return false, nil, p.shiftOp(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpSGreaterThan, spvenums.OpSGreaterThanEqual, spvenums.OpSLessThan, spvenums.OpSLessThanEqual:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		op, err := mapBinaryOperator(inst.op)
		if err != nil {
			return false, nil, err
		}
		return false, nil, p.intComparison(fs, consumer, block, bodyIdx, blockID, op, ir.ScalarSint)
	case spvenums.OpUGreaterThan, spvenums.OpUGreaterThanEqual, spvenums.OpULessThan, spvenums.OpULessThanEqual:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		op, err := mapBinaryOperator(inst.op)
		if err != nil {
			return false, nil, err
		}
		return false, nil, p.intComparison(fs, consumer, block, bodyIdx, blockID, op, ir.ScalarUint)
	case spvenums.OpAny, spvenums.OpAll, spvenums.OpIsNan, spvenums.OpIsInf,
		spvenums.OpIsFinite, spvenums.OpIsNormal:
		if err := inst.expect(4); err != nil {
			return false, nil, err
		}
		return false, nil, p.relational(fs, consumer, block, bodyIdx, blockID, inst.op)
	case spvenums.OpSelect:
		if err := inst.expect(6); err != nil {
			return false, nil, err
		}
		return false, nil, p.selectExpr(fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpVectorShuffle:
		if _, err := inst.expectAtLeast(5); err != nil {
			return false, nil, err
		}
		return false, nil, p.vectorShuffle(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpBitcast, spvenums.OpConvertSToF, spvenums.OpConvertUToF,
		spvenums.OpConvertFToU, spvenums.OpConvertFToS, spvenums.OpFConvert,
		spvenums.OpUConvert, spvenums.OpSConvert:
		if err := inst.expect(4); err != nil {
			return false, nil, err
		}
		return false, nil, p.convertExpr(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpTranspose:
		if err := inst.expect(4); err != nil {
			return false, nil, err
		}
		return false, nil, p.transpose(fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpDot:
		if err := inst.expect(5); err != nil {
			return false, nil, err
		}
		return false, nil, p.mathBinary(fs, consumer, block, bodyIdx, blockID, ir.MathDot)
	case spvenums.OpExtInst:
		if _, err := inst.expectAtLeast(5); err != nil {
			return false, nil, err
		}
		return false, nil, p.extInst(inst, fs, consumer, block, bodyIdx, blockID)
	case spvenums.OpDPdxCoarse:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeX, ir.DerivativeCoarse)
	case spvenums.OpDPdyCoarse:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeY, ir.DerivativeCoarse)
	case spvenums.OpFwidthCoarse:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeWidth, ir.DerivativeCoarse)
	case spvenums.OpDPdxFine:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeX, ir.DerivativeFine)
	case spvenums.OpDPdyFine:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeY, ir.DerivativeFine)
	case spvenums.OpFwidthFine:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeWidth, ir.DerivativeFine)
	case spvenums.OpDPdx:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeX, ir.DerivativeNone)
	case spvenums.OpDPdy:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeY, ir.DerivativeNone)
	case spvenums.OpFwidth:
		return false, nil, p.derivative(inst, fs, consumer, block, bodyIdx, blockID, ir.DerivativeWidth, ir.DerivativeNone)
	case spvenums.OpControlBarrier:
		return false, nil, p.controlBarrier(inst, fs, consumer, block)
	case spvenums.OpMemoryBarrier:
		return false, nil, p.memoryBarrier(inst, fs, consumer, block)
	case spvenums.OpReturn:
		if err := inst.expect(1); err != nil {
			return false, nil, err
		}
		return true, &ir.Statement{Kind: ir.StmtReturn{}}, nil
	case spvenums.OpReturnValue:
		if err := inst.expect(2); err != nil {
			return false, nil, err
		}
		valueID, err := p.next()
		if err != nil {
			return false, nil, err
		}
		value, err := p.handleOf(valueID, fs, consumer, block, bodyIdx)
		if err != nil {
			return false, nil, err
		}
		return true, &ir.Statement{Kind: ir.StmtReturn{Value: &value}}, nil
	case spvenums.OpKill:
		if err := inst.expect(1); err != nil {
			return false, nil, err
		}
		return true, &ir.Statement{Kind: ir.StmtKill{}}, nil
	case spvenums.OpUnreachable:
		if err := inst.expect(1); err != nil {
			return false, nil, err
		}
		return true, nil, nil
	default:
		return false, nil, errUnsupportedInstruction(p.state, inst.op)
	}
	return false, nil, nil
}

// localVariable parses OpVariable inside a function: a local variable, with an
// optional constant initializer.
func (p *parser) localVariable(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	// The storage class of a function variable is always Function.
	if _, err := p.next(); err != nil {
		return err
	}
	// The variable and its initializer are pre-emit expressions: they end the
	// Emit range of the instructions before them, as Rust naga flushes its
	// emitter here.
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	var init *ir.ExpressionHandle
	if inst.wc > 4 {
		if err := inst.expect(5); err != nil {
			return err
		}
		initID, err := p.next()
		if err != nil {
			return err
		}
		constant, err := p.constantLookup(initID)
		if err != nil {
			return err
		}
		handle := appendExpression(fs.fn, constant.kind.expressionKind())
		init = &handle
	}
	dec := p.takeDecoration(resultID)
	lookupType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	ty := lookupType.handle
	if pointer, ok := p.module.Types[ty].Inner.(ir.PointerType); ok {
		ty = pointer.Base
	}
	fs.fn.LocalVars = append(fs.fn.LocalVars, ir.LocalVariable{Name: dec.name, Type: ty, Init: init})
	local := ir.LocalVariableHandle(len(fs.fn.LocalVars) - 1)
	handle := appendExpression(fs.fn, ir.ExprLocalVariable{Variable: local})
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// accessChain parses OpAccessChain: a chain of struct, matrix, vector and
// array accesses. A row-major matrix member is refused until its transposed
// load is implemented, so no module is read with the wrong matrix order.
func (p *parser) accessChain(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	baseID, err := p.next()
	if err != nil {
		return err
	}
	base, err := p.lookupExpr(baseID)
	if err != nil {
		return err
	}
	baseType, err := p.typeLookup(base.typeID)
	if err != nil {
		return err
	}
	baseHandle, err := p.getExprHandle(baseID, base, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}

	// An AccessChain result type is a pointer, but a pointer to an image or a
	// sampler is already dereferenced while its type is parsed, except through
	// a binding array.
	currentTypeID := base.typeID
	if _, isBindingArray := p.module.Types[baseType.handle].Inner.(ir.BindingArrayType); !isBindingArray {
		if !baseType.hasBase {
			return errInvalidAccessType(base.typeID)
		}
		currentTypeID = baseType.baseID
	}

	handle := baseHandle
	for range inst.wc - 4 {
		indexID, err := p.next()
		if err != nil {
			return err
		}
		index, err := p.lookupExpr(indexID)
		if err != nil {
			return err
		}
		indexHandle, err := p.getExprHandle(indexID, index, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		indexValue, hasIndex := p.resolveIndex(fs, index)

		currentType, err := p.typeLookup(currentTypeID)
		if err != nil {
			return err
		}
		switch inner := p.module.Types[currentType.handle].Inner.(type) {
		case ir.StructType:
			if !hasIndex || int(indexValue) >= len(inner.Members) {
				return errInvalidAccess
			}
			member, ok := p.lookupMember[lookupMemberKey{handle: currentType.handle, member: indexValue}]
			if !ok {
				return errInvalidAccessType(currentTypeID)
			}
			if member.rowMajor {
				return errUnsupportedRowMajorMatrix
			}
			handle = appendExpression(fs.fn, ir.ExprAccessIndex{Base: handle, Index: indexValue})
			currentTypeID = member.typeID
		case ir.MatrixType:
			if hasIndex {
				handle = appendExpression(fs.fn, ir.ExprAccessIndex{Base: handle, Index: indexValue})
			} else {
				handle = appendExpression(fs.fn, ir.ExprAccess{Base: handle, Index: indexHandle})
			}
			if !currentType.hasBase {
				return errInvalidAccessType(currentTypeID)
			}
			currentTypeID = currentType.baseID
		default:
			handle = appendExpression(fs.fn, ir.ExprAccess{Base: handle, Index: indexHandle})
			if !currentType.hasBase {
				return errInvalidAccessType(currentTypeID)
			}
			currentTypeID = currentType.baseID
		}
	}
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	return nil
}

// resolveIndex returns the constant value an index expression holds, if it is
// a plain integer constant reference.
func (p *parser) resolveIndex(fs *functionState, index lookupExpression) (uint32, bool) {
	if int(index.handle) >= len(fs.fn.Expressions) {
		return 0, false
	}
	reference, ok := fs.fn.Expressions[index.handle].Kind.(ir.ExprConstant)
	if !ok {
		return 0, false
	}
	return p.literalConstantValue(reference.Constant)
}

// literalConstantValue resolves a constant to its integer literal value.
func (p *parser) literalConstantValue(constant ir.ConstantHandle) (uint32, bool) {
	if int(constant) >= len(p.module.Constants) {
		return 0, false
	}
	init := p.module.Constants[constant].Init
	if int(init) >= len(p.module.GlobalExpressions) {
		return 0, false
	}
	literal, ok := p.module.GlobalExpressions[init].Kind.(ir.Literal)
	if !ok {
		return 0, false
	}
	switch value := literal.Value.(type) {
	case ir.LiteralU32:
		return uint32(value), true
	case ir.LiteralI32:
		return uint32(value), true
	}
	return 0, false
}

// compositeExtract parses OpCompositeExtract: access indices from a constant.
func (p *parser) compositeExtract(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	baseID, err := p.next()
	if err != nil {
		return err
	}
	base, err := p.lookupExpr(baseID)
	if err != nil {
		return err
	}
	handle, err := p.getExprHandle(baseID, base, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	typeID := base.typeID
	for range inst.wc - 4 {
		index, err := p.next()
		if err != nil {
			return err
		}
		typeLookup, err := p.typeLookup(typeID)
		if err != nil {
			return err
		}
		switch p.module.Types[typeLookup.handle].Inner.(type) {
		case ir.StructType:
			member, ok := p.lookupMember[lookupMemberKey{handle: typeLookup.handle, member: index}]
			if !ok {
				return errInvalidAccessType(typeID)
			}
			typeID = member.typeID
		case ir.ArrayType, ir.VectorType, ir.MatrixType:
			if !typeLookup.hasBase {
				return errInvalidAccessType(typeID)
			}
			typeID = typeLookup.baseID
		default:
			return errUnsupportedType(typeLookup.handle)
		}
		handle = appendExpression(fs.fn, ir.ExprAccessIndex{Base: handle, Index: index})
	}
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	return nil
}

// compositeConstruct parses OpCompositeConstruct, detecting the splat form.
func (p *parser) compositeConstruct(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	components := make([]ir.ExpressionHandle, 0, inst.wc-3)
	for range inst.wc - 3 {
		componentID, err := p.next()
		if err != nil {
			return err
		}
		handle, err := p.handleOf(componentID, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		components = append(components, handle)
	}
	ty, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	var expression ir.ExpressionKind = ir.ExprCompose{Type: ty.handle, Components: components}
	if vector, ok := p.module.Types[ty.handle].Inner.(ir.VectorType); ok &&
		len(components) == int(vector.Size) && allEqual(components) {
		expression = ir.ExprSplat{Size: vector.Size, Value: components[0]}
	}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// allEqual reports whether every handle equals the first.
func allEqual(handles []ir.ExpressionHandle) bool {
	for _, handle := range handles[1:] {
		if handle != handles[0] {
			return false
		}
	}
	return true
}

// load parses OpLoad. An image or a sampler is not loaded: the handle is
// passed through, as in Rust naga.
func (p *parser) load(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	if inst.wc != 4 {
		if err := inst.expect(5); err != nil {
			return err
		}
		// The memory access operand is ignored.
		if _, err := p.next(); err != nil {
			return err
		}
	}
	pointer, err := p.lookupExpr(pointerID)
	if err != nil {
		return err
	}
	pointerHandle, err := p.getExprHandle(pointerID, pointer, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	pointerType, err := p.typeLookup(pointer.typeID)
	if err != nil {
		return err
	}
	handle := pointerHandle
	switch p.module.Types[pointerType.handle].Inner.(type) {
	case ir.ImageType, ir.SamplerType:
	default:
		handle = appendExpression(fs.fn, ir.ExprLoad{Pointer: pointerHandle})
	}
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	return nil
}

// store parses OpStore.
func (p *parser) store(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	pointerID, err := p.next()
	if err != nil {
		return err
	}
	valueID, err := p.next()
	if err != nil {
		return err
	}
	if inst.wc != 3 {
		if err := inst.expect(4); err != nil {
			return err
		}
		// The memory access operand is ignored.
		if _, err := p.next(); err != nil {
			return err
		}
	}
	pointer, err := p.handleOf(pointerID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	value, err := p.handleOf(valueID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	*block = append(*block, ir.Statement{Kind: ir.StmtStore{Pointer: pointer, Value: value}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// unaryOp parses a unary operation. A sign-adjusted one casts the operand to
// the result's scalar kind when the types differ, as Rust naga does.
func (p *parser) unaryOp(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, op ir.UnaryOperator, signAdjusted bool) error {
	if err := inst.expect(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	operandID, err := p.next()
	if err != nil {
		return err
	}
	operand, err := p.lookupExpr(operandID)
	if err != nil {
		return err
	}
	handle, err := p.getExprHandle(operandID, operand, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	if signAdjusted && operand.typeID != resultTypeID {
		resultType, err := p.typeLookup(resultTypeID)
		if err != nil {
			return err
		}
		kind, ok := scalarKind(p.module.Types[resultType.handle].Inner)
		if !ok {
			return errInvalidAsType(resultType.handle)
		}
		handle = appendExpression(fs.fn, ir.ExprAs{Expr: handle, Kind: kind})
	}
	expression := ir.ExprUnary{Op: op, Expr: handle}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// signAnchor says which type the operands of a binary operation are cast to.
type signAnchor uint8

const (
	signAnchorResult signAnchor = iota
	signAnchorOperand
)

// binaryOp parses a binary operation without sign adjustment.
func (p *parser) binaryOp(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, op ir.BinaryOperator) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	left, err := p.handleOf(leftID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	right, err := p.handleOf(rightID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	expression := ir.ExprBinary{Op: op, Left: left, Right: right}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// binaryOpSignAdjusted parses a binary operation whose operands are cast to
// the result's (or the first operand's) scalar kind when their types differ.
func (p *parser) binaryOpSignAdjusted(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, op ir.BinaryOperator, anchor signAnchor) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	leftLookup, err := p.lookupExpr(leftID)
	if err != nil {
		return err
	}
	left, err := p.getExprHandle(leftID, leftLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	rightLookup, err := p.lookupExpr(rightID)
	if err != nil {
		return err
	}
	right, err := p.getExprHandle(rightID, rightLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}

	expectedTypeID := resultTypeID
	if anchor == signAnchorOperand {
		expectedTypeID = leftLookup.typeID
	}
	expectedType, err := p.typeLookup(expectedTypeID)
	if err != nil {
		return err
	}
	kind, ok := scalarKind(p.module.Types[expectedType.handle].Inner)
	if !ok {
		return errInvalidAsType(expectedType.handle)
	}
	if leftLookup.typeID != expectedTypeID {
		left = appendExpression(fs.fn, ir.ExprAs{Expr: left, Kind: kind})
	}
	if rightLookup.typeID != expectedTypeID {
		right = appendExpression(fs.fn, ir.ExprAs{Expr: right, Kind: kind})
	}
	expression := ir.ExprBinary{Op: op, Left: left, Right: right}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// intComparison parses a signed or unsigned integer comparison: both operands
// are cast to the comparison's kind when they differ from it.
func (p *parser) intComparison(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, op ir.BinaryOperator, kind ir.ScalarKind) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	leftLookup, err := p.lookupExpr(leftID)
	if err != nil {
		return err
	}
	left, err := p.getExprHandle(leftID, leftLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	rightLookup, err := p.lookupExpr(rightID)
	if err != nil {
		return err
	}
	right, err := p.getExprHandle(rightID, rightLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	leftKind, err := p.expressionScalarKind(leftLookup.typeID)
	if err != nil {
		return err
	}
	if leftKind != kind {
		left = appendExpression(fs.fn, ir.ExprAs{Expr: left, Kind: kind})
	}
	rightKind, err := p.expressionScalarKind(rightLookup.typeID)
	if err != nil {
		return err
	}
	if rightKind != kind {
		right = appendExpression(fs.fn, ir.ExprAs{Expr: right, Kind: kind})
	}
	expression := ir.ExprBinary{Op: op, Left: left, Right: right}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// expressionScalarKind returns the scalar kind of an expression's type.
func (p *parser) expressionScalarKind(typeID uint32) (ir.ScalarKind, error) {
	ty, err := p.typeLookup(typeID)
	if err != nil {
		return 0, err
	}
	kind, ok := scalarKind(p.module.Types[ty.handle].Inner)
	if !ok {
		return 0, errInvalidAsType(ty.handle)
	}
	return kind, nil
}

// shiftOp parses a shift: the shift amount is always converted to unsigned.
func (p *parser) shiftOp(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	op, err := mapBinaryOperator(inst.op)
	if err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	left, err := p.handleOf(leftID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	right, err := p.handleOf(rightID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	right = appendExpression(fs.fn, ir.ExprAs{Expr: right, Kind: ir.ScalarUint})
	expression := ir.ExprBinary{Op: op, Left: left, Right: right}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// signedModulo parses OpSMod as x - y * int(floor(float(x) / float(y))), as
// Rust naga does.
func (p *parser) signedModulo(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	left, err := p.handleOf(leftID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	right, err := p.handleOf(rightID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	kind, ok := scalarKind(p.module.Types[resultType.handle].Inner)
	if !ok {
		return errInvalidAsType(resultType.handle)
	}
	size, err := p.typeByteSize(resultType.handle)
	if err != nil {
		return err
	}
	width := uint8(size)
	leftCast := appendExpression(fs.fn, ir.ExprAs{Expr: left, Kind: ir.ScalarFloat, Convert: &width})
	rightCast := appendExpression(fs.fn, ir.ExprAs{Expr: right, Kind: ir.ScalarFloat, Convert: &width})
	division := appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryDivide, Left: leftCast, Right: rightCast})
	floor := appendExpression(fs.fn, ir.ExprMath{Fun: ir.MathFloor, Arg: division})
	cast := appendExpression(fs.fn, ir.ExprAs{Expr: floor, Kind: kind, Convert: &width})
	multiply := appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryMultiply, Left: cast, Right: right})
	subtract := appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinarySubtract, Left: left, Right: multiply})
	p.storeExpression(resultID, resultTypeID, blockID, subtract)
	return nil
}

// floatModulo parses OpFMod as x - y * floor(x / y).
func (p *parser) floatModulo(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	left, err := p.handleOf(leftID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	right, err := p.handleOf(rightID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	division := appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryDivide, Left: left, Right: right})
	floor := appendExpression(fs.fn, ir.ExprMath{Fun: ir.MathFloor, Arg: division})
	multiply := appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryMultiply, Left: floor, Right: right})
	subtract := appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinarySubtract, Left: left, Right: multiply})
	p.storeExpression(resultID, resultTypeID, blockID, subtract)
	return nil
}

// typeByteSize returns the byte size of the scalar or vector type, as Rust's
// TypeInner::size gives the width an As conversion needs.
func (p *parser) typeByteSize(ty ir.TypeHandle) (uint32, error) {
	if err := p.updateLayouter(); err != nil {
		return 0, err
	}
	return p.layouter.Layout(ty).Size, nil
}

// relational parses OpAny, OpAll, OpIsNan and OpIsInf.
func (p *parser) relational(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, op spvenums.Op) error {
	fun, err := mapRelationalFunction(op)
	if err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	argumentID, err := p.next()
	if err != nil {
		return err
	}
	argument, err := p.handleOf(argumentID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	expression := ir.ExprRelational{Fun: fun, Argument: argument}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// selectExpr parses OpSelect.
func (p *parser) selectExpr(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	conditionID, err := p.next()
	if err != nil {
		return err
	}
	acceptID, err := p.next()
	if err != nil {
		return err
	}
	rejectID, err := p.next()
	if err != nil {
		return err
	}
	condition, err := p.handleOf(conditionID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	accept, err := p.handleOf(acceptID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	reject, err := p.handleOf(rejectID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	expression := ir.ExprSelect{Condition: condition, Accept: accept, Reject: reject}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// vectorShuffle parses OpVectorShuffle into a swizzle where it is one, and into
// an access-and-compose otherwise, as Rust naga does.
func (p *parser) vectorShuffle(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	firstID, err := p.next()
	if err != nil {
		return err
	}
	secondID, err := p.next()
	if err != nil {
		return err
	}
	firstLookup, err := p.lookupExpr(firstID)
	if err != nil {
		return err
	}
	firstType, err := p.typeLookup(firstLookup.typeID)
	if err != nil {
		return err
	}
	first, ok := p.module.Types[firstType.handle].Inner.(ir.VectorType)
	if !ok {
		return errInvalidInnerType(firstLookup.typeID)
	}
	firstHandle, err := p.getExprHandle(firstID, firstLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	secondLookup, err := p.lookupExpr(secondID)
	if err != nil {
		return err
	}
	secondType, err := p.typeLookup(secondLookup.typeID)
	if err != nil {
		return err
	}
	second, ok := p.module.Types[secondType.handle].Inner.(ir.VectorType)
	if !ok {
		return errInvalidInnerType(secondLookup.typeID)
	}
	secondHandle, err := p.getExprHandle(secondID, secondLookup, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}

	count := inst.wc - 5
	indices := make([]uint32, 0, count)
	maxComponent := uint32(0)
	for range count {
		index, err := p.next()
		if err != nil {
			return err
		}
		if index == ^uint32(0) {
			// Undefined components read as X.
			index = 0
		}
		maxComponent = max(maxComponent, index)
		indices = append(indices, index)
	}

	var expression ir.ExpressionKind
	if maxComponent < uint32(first.Size) {
		// Rust naga maps every size beyond three to four.
		size := ir.Vec4
		switch len(indices) {
		case 2:
			size = ir.Vec2
		case 3:
			size = ir.Vec3
		}
		pattern := [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleX, ir.SwizzleX, ir.SwizzleX}
		for i, index := range indices {
			switch index {
			case 0:
				pattern[i] = ir.SwizzleX
			case 1:
				pattern[i] = ir.SwizzleY
			case 2:
				pattern[i] = ir.SwizzleZ
			default:
				pattern[i] = ir.SwizzleW
			}
		}
		expression = ir.ExprSwizzle{Size: size, Vector: firstHandle, Pattern: pattern}
	} else {
		components := make([]ir.ExpressionHandle, 0, len(indices))
		for _, index := range indices {
			switch {
			case index < uint32(first.Size):
				components = append(components, appendExpression(fs.fn, ir.ExprAccessIndex{Base: firstHandle, Index: index}))
			case index < uint32(first.Size)+uint32(second.Size):
				components = append(components, appendExpression(fs.fn, ir.ExprAccessIndex{Base: secondHandle, Index: index - uint32(first.Size)}))
			default:
				return errInvalidAccessIndex(index)
			}
		}
		resultType, err := p.typeLookup(resultTypeID)
		if err != nil {
			return err
		}
		expression = ir.ExprCompose{Type: resultType.handle, Components: components}
	}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// convertExpr parses OpBitcast and the Convert* instructions into an As
// expression.
func (p *parser) convertExpr(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	valueID, err := p.next()
	if err != nil {
		return err
	}
	value, err := p.handleOf(valueID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	scalar, ok := scalarOf(p.module.Types[resultType.handle].Inner)
	if !ok {
		return errInvalidAsType(resultType.handle)
	}
	as := ir.ExprAs{Expr: value, Kind: scalar.Kind}
	switch {
	case scalar.Kind == ir.ScalarBool:
		width := uint8(1)
		as.Convert = &width
	case inst.op == spvenums.OpBitcast:
	default:
		width := scalar.Width
		as.Convert = &width
	}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, as))
	return nil
}

// transpose parses OpTranspose.
func (p *parser) transpose(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	matrixID, err := p.next()
	if err != nil {
		return err
	}
	matrix, err := p.handleOf(matrixID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	expression := ir.ExprMath{Fun: ir.MathTranspose, Arg: matrix}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// mathBinary parses a two-argument math function written as its own
// instruction, OpDot.
func (p *parser) mathBinary(fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, fun ir.MathFunction) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	leftID, err := p.next()
	if err != nil {
		return err
	}
	rightID, err := p.next()
	if err != nil {
		return err
	}
	left, err := p.handleOf(leftID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	right, err := p.handleOf(rightID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	expression := ir.ExprMath{Fun: fun, Arg: left, Arg1: &right}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// extInst parses OpExtInst of the GLSL.std.450 set into a math function.
func (p *parser) extInst(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	setID, err := p.next()
	if err != nil {
		return err
	}
	if p.hasExtNonSemantic && setID == p.extNonSemanticID {
		for range inst.wc - 4 {
			if _, err := p.next(); err != nil {
				return err
			}
		}
		return nil
	}
	if !p.hasExtGLSLID || setID != p.extGLSLID {
		return errUnsupportedExtInstSet(setID)
	}
	instructionID, err := p.next()
	if err != nil {
		return err
	}
	fun, err := mapGlslStd450(spvenums.GlslStd450(instructionID))
	if err != nil {
		return err
	}
	argumentCount := mathArgumentCount(fun)
	if err := inst.expect(5 + uint16(argumentCount)); err != nil {
		return err
	}
	arguments := make([]ir.ExpressionHandle, 0, argumentCount)
	for range argumentCount {
		argumentID, err := p.next()
		if err != nil {
			return err
		}
		handle, err := p.handleOf(argumentID, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		arguments = append(arguments, handle)
	}
	expression := ir.ExprMath{Fun: fun, Arg: arguments[0]}
	if len(arguments) > 1 {
		expression.Arg1 = &arguments[1]
	}
	if len(arguments) > 2 {
		expression.Arg2 = &arguments[2]
	}
	if len(arguments) > 3 {
		expression.Arg3 = &arguments[3]
	}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// derivative parses one of the OpDPdx family.
func (p *parser) derivative(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32, axis ir.DerivativeAxis, control ir.DerivativeControl) error {
	if err := inst.expect(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	argumentID, err := p.next()
	if err != nil {
		return err
	}
	argument, err := p.handleOf(argumentID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	expression := ir.ExprDerivative{Axis: axis, Control: control, Expr: argument}
	p.storeExpression(resultID, resultTypeID, blockID, appendExpression(fs.fn, expression))
	return nil
}

// controlBarrier parses OpControlBarrier. Only a workgroup or subgroup
// execution scope is expressible in the IR; another scope is a warning and
// no statement, as in Rust naga.
func (p *parser) controlBarrier(inst instruction, fs *functionState, consumer *emitter, block *ir.Block) error {
	if err := inst.expect(4); err != nil {
		return err
	}
	execScopeID, err := p.next()
	if err != nil {
		return err
	}
	if _, err := p.next(); err != nil { // memory scope
		return err
	}
	semanticsID, err := p.next()
	if err != nil {
		return err
	}
	execScope, err := p.constantValue(execScopeID, errInvalidBarrierScope)
	if err != nil {
		return err
	}
	semantics, err := p.constantValue(semanticsID, errInvalidBarrierMemorySemantics)
	if err != nil {
		return err
	}
	if execScope != uint32(spvenums.ScopeWorkgroup) && execScope != uint32(spvenums.ScopeSubgroup) {
		p.warnf("Unsupported barrier execution scope: %d", execScope)
		return nil
	}
	flags := barrierFlags(0, semantics)
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	*block = append(*block, ir.Statement{Kind: ir.StmtControlBarrier{Flags: flags}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// memoryBarrier parses OpMemoryBarrier.
func (p *parser) memoryBarrier(inst instruction, fs *functionState, consumer *emitter, block *ir.Block) error {
	if err := inst.expect(3); err != nil {
		return err
	}
	scopeID, err := p.next()
	if err != nil {
		return err
	}
	semanticsID, err := p.next()
	if err != nil {
		return err
	}
	scope, err := p.constantValue(scopeID, errInvalidBarrierScope)
	if err != nil {
		return err
	}
	semantics, err := p.constantValue(semanticsID, errInvalidBarrierMemorySemantics)
	if err != nil {
		return err
	}
	var flags ir.BarrierFlags
	switch spvenums.Scope(scope) {
	case spvenums.ScopeDevice:
		flags = ir.BarrierStorage
	case spvenums.ScopeWorkgroup:
		flags = ir.BarrierWorkGroup
	case spvenums.ScopeSubgroup:
		flags = ir.BarrierSubGroup
	}
	flags = barrierFlags(flags, semantics)
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	*block = append(*block, ir.Statement{Kind: ir.StmtMemoryBarrier{Flags: flags}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// barrierFlags sets the flags the memory semantics word names.
func barrierFlags(flags ir.BarrierFlags, semantics uint32) ir.BarrierFlags {
	if semantics&uint32(spvenums.MemorySemanticsUniformMemory) != 0 {
		flags |= ir.BarrierStorage
	}
	if semantics&uint32(spvenums.MemorySemanticsWorkgroupMemory) != 0 {
		flags |= ir.BarrierWorkGroup
	}
	if semantics&uint32(spvenums.MemorySemanticsSubgroupMemory) != 0 {
		flags |= ir.BarrierSubGroup
	}
	if semantics&uint32(spvenums.MemorySemanticsImageMemory) != 0 {
		flags |= ir.BarrierTexture
	}
	return flags
}

// constantValue resolves a barrier operand constant to its integer value.
func (p *parser) constantValue(id uint32, errorFor func(uint32) *Error) (uint32, error) {
	constant, err := p.constantLookup(id)
	if err != nil {
		return 0, err
	}
	if constant.kind.constant != nil {
		if value, ok := p.literalConstantValue(*constant.kind.constant); ok {
			return value, nil
		}
	}
	return 0, errorFor(id)
}
