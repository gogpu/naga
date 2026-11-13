package front

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// subgroupScope checks the execution scope operand of a subgroup instruction:
// the IR has only subgroup operations, so anything else is an error, as Rust
// naga's resolve_constant filter.
func (p *parser) subgroupScope(id uint32) error {
	scope, err := p.constantValue(id, errInvalidBarrierScope)
	if err != nil {
		return err
	}
	if scope != uint32(spvenums.ScopeSubgroup) {
		return errInvalidBarrierScope(id)
	}
	return nil
}

// subgroupBallot parses OpGroupNonUniformBallot. A predicate that is the
// constant true is dropped: it would vote for every invocation.
func (p *parser) subgroupBallot(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	scopeID, err := p.next()
	if err != nil {
		return err
	}
	predicateID, err := p.next()
	if err != nil {
		return err
	}
	if err := p.subgroupScope(scopeID); err != nil {
		return err
	}
	var predicate *ir.ExpressionHandle
	if !p.isTrueConstant(predicateID) {
		handle, err := p.handleOf(predicateID, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		predicate = &handle
	}

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprSubgroupBallotResult{})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	*block = append(*block, ir.Statement{Kind: ir.StmtSubgroupBallot{
		Result:    result,
		Predicate: predicate,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// isTrueConstant reports whether an id names the constant true, which the IR
// spells as no predicate at all.
func (p *parser) isTrueConstant(id uint32) bool {
	constant, err := p.constantLookup(id)
	if err != nil || constant.kind.constant == nil {
		return false
	}
	if int(*constant.kind.constant) >= len(p.module.Constants) {
		return false
	}
	init := p.module.Constants[*constant.kind.constant].Init
	if int(init) >= len(p.module.GlobalExpressions) {
		return false
	}
	literal, ok := p.module.GlobalExpressions[init].Kind.(ir.Literal)
	if !ok {
		return false
	}
	value, ok := literal.Value.(ir.LiteralBool)
	return ok && bool(value)
}

// subgroupCollective parses OpGroupNonUniformAll, OpGroupNonUniformAny and the
// arithmetic and bitwise reduction and scan families.
func (p *parser) subgroupCollective(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	scopeID, err := p.next()
	if err != nil {
		return err
	}
	collective := ir.CollectiveReduce
	allOrAny := inst.op == spvenums.OpGroupNonUniformAll || inst.op == spvenums.OpGroupNonUniformAny
	if !allOrAny {
		groupOp, err := p.next()
		if err != nil {
			return err
		}
		switch spvenums.GroupOperation(groupOp) {
		case spvenums.GroupOperationReduce:
			collective = ir.CollectiveReduce
		case spvenums.GroupOperationInclusiveScan:
			collective = ir.CollectiveInclusiveScan
		case spvenums.GroupOperationExclusiveScan:
			collective = ir.CollectiveExclusiveScan
		default:
			return errUnsupportedGroupOperation(groupOp)
		}
	}
	argumentID, err := p.next()
	if err != nil {
		return err
	}
	argument, err := p.handleOf(argumentID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	if err := p.subgroupScope(scopeID); err != nil {
		return err
	}
	op, err := mapSubgroupOperation(inst.op)
	if err != nil {
		return err
	}
	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprSubgroupOperationResult{Type: resultType.handle})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	*block = append(*block, ir.Statement{Kind: ir.StmtSubgroupCollectiveOperation{
		Op:           op,
		CollectiveOp: collective,
		Argument:     argument,
		Result:       result,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// subgroupGather parses the broadcast and shuffle family and
// OpGroupNonUniformQuadSwap.
func (p *parser) subgroupGather(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	scopeID, err := p.next()
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
	if err := p.subgroupScope(scopeID); err != nil {
		return err
	}

	var mode ir.GatherMode
	switch inst.op {
	case spvenums.OpGroupNonUniformBroadcastFirst:
		mode = ir.GatherBroadcastFirst{}
	case spvenums.OpGroupNonUniformQuadSwap:
		directionID, err := p.next()
		if err != nil {
			return err
		}
		direction, err := p.constantValue(directionID, func(uint32) *Error { return errInvalidOperand })
		if err != nil {
			return err
		}
		switch direction {
		case 0:
			mode = ir.GatherQuadSwap{Direction: ir.QuadDirectionX}
		case 1:
			mode = ir.GatherQuadSwap{Direction: ir.QuadDirectionY}
		case 2:
			mode = ir.GatherQuadSwap{Direction: ir.QuadDirectionDiagonal}
		default:
			return errInvalidOperand
		}
	default:
		indexID, err := p.next()
		if err != nil {
			return err
		}
		index, err := p.handleOf(indexID, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		switch inst.op {
		case spvenums.OpGroupNonUniformBroadcast:
			mode = ir.GatherBroadcast{Index: index}
		case spvenums.OpGroupNonUniformShuffle:
			mode = ir.GatherShuffle{Index: index}
		case spvenums.OpGroupNonUniformShuffleDown:
			mode = ir.GatherShuffleDown{Delta: index}
		case spvenums.OpGroupNonUniformShuffleUp:
			mode = ir.GatherShuffleUp{Delta: index}
		case spvenums.OpGroupNonUniformShuffleXor:
			mode = ir.GatherShuffleXor{Mask: index}
		case spvenums.OpGroupNonUniformQuadBroadcast:
			mode = ir.GatherQuadBroadcast{Index: index}
		default:
			return errUnsupportedInstruction(stateFunction, inst.op)
		}
	}

	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprSubgroupOperationResult{Type: resultType.handle})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	*block = append(*block, ir.Statement{Kind: ir.StmtSubgroupGather{
		Mode:     mode,
		Argument: argument,
		Result:   result,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}
