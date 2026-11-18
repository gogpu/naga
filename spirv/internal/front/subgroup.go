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

// subgroupElect parses OpGroupNonUniformElect: result type, result, scope —
// no value operand. ADR-014 (card P2-IR-038) gives the IR the statement.
func (p *parser) subgroupElect(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
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
	if err := p.subgroupScope(scopeID); err != nil {
		return err
	}
	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprSubgroupOperationResult{Type: resultType.handle})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	*block = append(*block, ir.Statement{Kind: ir.StmtSubgroupElect{
		Result: result,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// subgroupBallotBit parses OpGroupNonUniformInverseBallot,
// OpGroupNonUniformBallotBitExtract, OpGroupNonUniformBallotBitCount and
// OpGroupNonUniformBallotFindLSB / FindMSB: the ballot bit operations of the
// GroupNonUniformBallot capability (card P2-IR-039, ADR-014 D3). Rust naga's
// SPIR-V frontend has none of them. The counts carry their group operation
// as an operand, as glslang writes them; a clustered count is refused with
// the group-operation error, the IR having no clustered form yet.
func (p *parser) subgroupBallotBit(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
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
	if err := p.subgroupScope(scopeID); err != nil {
		return err
	}

	var op ir.SubgroupBallotBitOperation
	switch inst.op {
	case spvenums.OpGroupNonUniformInverseBallot:
		op = ir.SubgroupInverseBallot
	case spvenums.OpGroupNonUniformBallotBitExtract:
		op = ir.SubgroupBallotBitExtract
	case spvenums.OpGroupNonUniformBallotFindLSB:
		op = ir.SubgroupBallotFindLSB
	case spvenums.OpGroupNonUniformBallotFindMSB:
		op = ir.SubgroupBallotFindMSB
	case spvenums.OpGroupNonUniformBallotBitCount:
		groupOp, err := p.next()
		if err != nil {
			return err
		}
		switch spvenums.GroupOperation(groupOp) {
		case spvenums.GroupOperationReduce:
			op = ir.SubgroupBallotBitCount
		case spvenums.GroupOperationInclusiveScan:
			op = ir.SubgroupBallotBitInclusiveCount
		case spvenums.GroupOperationExclusiveScan:
			op = ir.SubgroupBallotBitExclusiveCount
		default:
			return errUnsupportedGroupOperation(groupOp)
		}
	default:
		return errUnsupportedInstruction(stateFunction, inst.op)
	}

	maskID, err := p.next()
	if err != nil {
		return err
	}
	mask, err := p.handleOf(maskID, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	var index ir.ExpressionHandle
	if op == ir.SubgroupBallotBitExtract {
		indexID, err := p.next()
		if err != nil {
			return err
		}
		if index, err = p.handleOf(indexID, fs, consumer, block, bodyIdx); err != nil {
			return err
		}
	}
	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	result := appendExpression(fs.fn, ir.ExprSubgroupOperationResult{Type: resultType.handle})
	p.storeExpression(resultID, resultTypeID, blockID, result)
	*block = append(*block, ir.Statement{Kind: ir.StmtSubgroupBallotBit{
		Op:     op,
		Mask:   mask,
		Index:  index,
		Result: result,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// subgroupCollective parses OpGroupNonUniformAll, OpGroupNonUniformAny,
// arithmetic and bitwise reduction and scan families, including the
// ClusteredReduce group operation with its optional Cluster Size operand
// (card P2-IR-040, ADR-014 D4).
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
	allOrAny := inst.op == spvenums.OpGroupNonUniformAll || inst.op == spvenums.OpGroupNonUniformAny ||
		inst.op == spvenums.OpGroupNonUniformAllEqual
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
		case spvenums.GroupOperationClusteredReduce:
			collective = ir.CollectiveOperationClusteredReduce
		default:
			return errUnsupportedGroupOperation(groupOp)
		}
	}
	// The Cluster Size operand follows the value, and only the clustered
	// group operation has it (spirv.core.grammar.json: the operand is
	// optional and appears on the ClusteredReduce form alone). The votes take
	// no group operation and their count is already checked by the dispatch.
	var clusterSize uint32
	if !allOrAny {
		if collective == ir.CollectiveOperationClusteredReduce {
			if err := inst.expect(7); err != nil {
				return err
			}
		} else if err := inst.expect(6); err != nil {
			return err
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
	if collective == ir.CollectiveOperationClusteredReduce {
		clusterSizeID, err := p.next()
		if err != nil {
			return err
		}
		if clusterSize, err = p.clusterSize(clusterSizeID); err != nil {
			return err
		}
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
		ClusterSize:  clusterSize,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// clusterSize resolves the Cluster Size operand of a ClusteredReduce
// instruction: a 32-bit integer constant instruction whose value is at least
// 1 and a power of 2 (GL_KHR_shader_subgroup §3.8; the SPIR-V specification
// states the same constraint on the operand). A signed negative constant is
// outside the rule, never reinterpreted as a large unsigned size.
func (p *parser) clusterSize(id uint32) (uint32, error) {
	constant, err := p.constantLookup(id)
	if err != nil {
		// The operand is a constant instruction: an id the module defines as
		// something else is named as such, not as an unknown id.
		return 0, errInvalidClusterSize(id)
	}
	if constant.kind.constant == nil {
		return 0, errInvalidClusterSize(id)
	}
	init := p.module.Constants[*constant.kind.constant].Init
	literal, ok := p.module.GlobalExpressions[init].Kind.(ir.Literal)
	if !ok {
		return 0, errInvalidClusterSize(id)
	}
	var size int64
	switch value := literal.Value.(type) {
	case ir.LiteralU32:
		size = int64(value)
	case ir.LiteralI32:
		size = int64(value)
	default:
		return 0, errInvalidClusterSize(id)
	}
	if size < 1 || size&(size-1) != 0 {
		return 0, errInvalidClusterSizeValue(size)
	}
	return uint32(size), nil
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
