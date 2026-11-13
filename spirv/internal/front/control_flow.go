package front

import (
	"fmt"
	"sort"

	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// Structured control flow, ported from Rust naga's next_block
// (front/spv/next_block.rs, the OpPhi, OpBranch, OpBranchConditional,
// OpSwitch, OpSelectionMerge and OpLoopMerge arms). SPIR-V requires a block to
// precede every block it dominates, so one pass over the blocks sees each
// construct's header before anything inside it: the header records where its
// merge block and continue target belong, and the blocks are collected into
// the body tree that lower assembles into statements at the end.

// blockState is the block being parsed: next_block's locals.
type blockState struct {
	id       uint32
	bodyIdx  int
	block    ir.Block
	consumer emitter
	// selectionMerge is the merge block an OpSelectionMerge of this block
	// declared, so that an OpSwitch after it can promote it to a switch merge
	// whose branches are Breaks.
	selectionMerge    uint32
	hasSelectionMerge bool
}

// nextBlock parses one OpLabel block into fs.blocks and the body it
// contributes to, as Rust naga's next_block. Every block but the function's
// first was named by a branch before it, which gave it its body; the first
// block belongs to the top-level body.
func (p *parser) nextBlock(blockID uint32, fs *functionState) error {
	if _, ok := fs.blocks[blockID]; ok {
		return errDuplicateLabel(blockID)
	}
	bs := &blockState{id: blockID}
	bs.consumer.startWith(fs.fn.Expressions)
	if idx, ok := fs.bodyForLabel[blockID]; ok {
		bs.bodyIdx = idx
	} else {
		fs.bodyForLabel[blockID] = 0
	}

	for {
		inst, err := p.nextInstruction()
		if err != nil {
			return err
		}
		switch inst.op {
		case spvenums.OpPhi:
			err = p.phi(inst, fs, bs)
		case spvenums.OpSelectionMerge:
			err = p.selectionMerge(inst, fs, bs)
		case spvenums.OpLoopMerge:
			err = p.loopMerge(inst, fs, bs)
		case spvenums.OpBranch:
			return p.branch(inst, fs, bs)
		case spvenums.OpBranchConditional:
			return p.branchConditional(inst, fs, bs)
		case spvenums.OpSwitch:
			return p.switchBranch(inst, fs, bs)
		default:
			var done bool
			var terminator *ir.Statement
			done, terminator, err = p.blockInstruction(inst, fs, &bs.consumer, &bs.block, bs.bodyIdx, blockID)
			if err == nil && done {
				fs.finishBlock(bs, terminator)
				return nil
			}
		}
		if err != nil {
			return err
		}
	}
}

// finishBlock closes the block: its pending expressions are emitted, the
// terminator (if any) appended, and the block is added to its body.
func (fs *functionState) finishBlock(bs *blockState, terminator *ir.Statement) {
	if statement, ok := bs.consumer.finish(fs.fn.Expressions); ok {
		bs.block = append(bs.block, statement)
	}
	if terminator != nil {
		bs.block = append(bs.block, *terminator)
	}
	fs.blocks[bs.id] = bs.block
	fs.bodies[bs.bodyIdx].fragments = append(fs.bodies[bs.bodyIdx].fragments, bodyFragment{
		kind:    fragmentBlock,
		blockID: bs.id,
	})
}

// phi parses OpPhi: a local variable named after the result, loaded in this
// block. The stores into it are added to the end of each predecessor once
// the whole function is parsed (appendPhiStores).
func (p *parser) phi(inst instruction, fs *functionState, bs *blockState) error {
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	// Each incoming value is a (value, predecessor) pair; a dangling word
	// would be read as the next instruction.
	if (inst.wc-3)%2 != 0 {
		return errInvalidOperandCount(inst.op, inst.wc)
	}
	flushBeforeStatement(&bs.consumer, fs.fn.Expressions, &bs.block)

	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	ty, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	fs.fn.LocalVars = append(fs.fn.LocalVars, ir.LocalVariable{
		Name: fmt.Sprintf("phi_%d", resultID),
		Type: ty.handle,
	})
	local := ir.LocalVariableHandle(len(fs.fn.LocalVars) - 1)
	pointer := appendExpression(fs.fn, ir.ExprLocalVariable{Variable: local})

	inCount := int(inst.wc-3) / 2
	phi := phiExpression{local: local, expressions: make([]phiSource, 0, inCount)}
	for range inCount {
		valueID, err := p.next()
		if err != nil {
			return err
		}
		predecessor, err := p.next()
		if err != nil {
			return err
		}
		phi.expressions = append(phi.expressions, phiSource{id: valueID, blockID: predecessor})
	}
	fs.phis = append(fs.phis, phi)

	bs.consumer.startWith(fs.fn.Expressions)
	p.storeExpression(resultID, resultTypeID, bs.id, appendExpression(fs.fn, ir.ExprLoad{Pointer: pointer}))
	return nil
}

// selectionMerge parses OpSelectionMerge: the merge block continues the
// current body, and a branch to it ends the selection.
func (p *parser) selectionMerge(inst instruction, fs *functionState, bs *blockState) error {
	if err := inst.expect(3); err != nil {
		return err
	}
	mergeID, err := p.next()
	if err != nil {
		return err
	}
	// The selection control mask is not carried into the IR.
	if _, err := p.next(); err != nil {
		return err
	}
	// A block is the merge block of one construct only. A selection may merge
	// at its loop's continue target, which then ends the selection as in Rust
	// naga.
	if info, ok := fs.mergers[mergeID]; ok && info != mergeLoopContinue {
		return errDuplicateMerge(mergeID)
	}
	if _, ok := fs.bodyForLabel[mergeID]; !ok {
		fs.bodyForLabel[mergeID] = bs.bodyIdx
	}
	fs.mergers[mergeID] = mergeSelectionMerge
	bs.selectionMerge = mergeID
	bs.hasSelectionMerge = true
	return nil
}

// loopMerge parses OpLoopMerge: the loop body and the continuing body are
// created, the header joins the loop body, and the merge block continues the
// current body.
func (p *parser) loopMerge(inst instruction, fs *functionState, bs *blockState) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	mergeID, err := p.next()
	if err != nil {
		return err
	}
	continuingID, err := p.next()
	if err != nil {
		return err
	}
	// The loop control and its parameters are not carried into the IR.
	for range inst.wc - 3 {
		if _, err := p.next(); err != nil {
			return err
		}
	}
	if _, ok := fs.mergers[mergeID]; ok || mergeID == continuingID {
		return errDuplicateMerge(mergeID)
	}
	if _, ok := fs.mergers[continuingID]; ok {
		return errDuplicateMerge(continuingID)
	}

	if _, ok := fs.bodyForLabel[mergeID]; !ok {
		fs.bodyForLabel[mergeID] = bs.bodyIdx
	}
	fs.mergers[mergeID] = mergeLoopMerge

	// The continuing body sits in the loop body's scope (its parent), but is
	// lowered beside it: both are one level below the header's body.
	depth := fs.bodies[bs.bodyIdx].depth + 1
	loopBody, err := fs.newBody(bs.bodyIdx, depth)
	if err != nil {
		return err
	}
	continuing, err := fs.newBody(loopBody, depth)
	if err != nil {
		return err
	}
	fs.bodies[loopBody].kind = bodyLoop
	fs.bodies[continuing].kind = bodyContinuing
	if _, ok := fs.bodyForLabel[continuingID]; !ok {
		fs.bodyForLabel[continuingID] = continuing
	}
	fs.mergers[continuingID] = mergeLoopContinue

	// The loop header always belongs to the loop body.
	fs.bodyForLabel[bs.id] = loopBody
	fs.bodies[bs.bodyIdx].fragments = append(fs.bodies[bs.bodyIdx].fragments, bodyFragment{
		kind:       fragmentLoop,
		loopBody:   loopBody,
		continuing: continuing,
		merge:      mergeID,
	})
	bs.bodyIdx = loopBody
	return nil
}

// branch parses OpBranch. A branch to a merge block or a continue target ends
// the current body with the matching statement; any other target continues
// the current body (or, for a loop's back edge, is the header already
// placed).
func (p *parser) branch(inst instruction, fs *functionState, bs *blockState) error {
	if err := inst.expect(2); err != nil {
		return err
	}
	targetID, err := p.next()
	if err != nil {
		return err
	}
	if info, ok := fs.mergers[targetID]; ok {
		if info == mergeLoopMerge && fs.breakLeavesSwitch(bs.bodyIdx) {
			return errSwitchExitsLoop
		}
		fs.finishBlock(bs, nil)
		merger(&fs.bodies[bs.bodyIdx], info)
		return nil
	}
	// A branch to another case of the switch around this case falls through
	// into it; the cases are ordered by these edges when the switch is
	// lowered.
	if ref, ok := fs.caseLabels[targetID]; ok {
		if err := fs.recordFallThrough(bs.bodyIdx, ref); err != nil {
			return err
		}
		fs.finishBlock(bs, nil)
		return nil
	}
	// Not a merge block, and not a switch case (the switch header names
	// those): the only branch to the target, which the current block
	// dominates, so it continues this body.
	if _, ok := fs.bodyForLabel[targetID]; !ok {
		fs.bodyForLabel[targetID] = bs.bodyIdx
	}
	fs.finishBlock(bs, nil)
	return nil
}

// branchTarget is one target of OpBranchConditional and what a branch to it
// ends, if anything.
type branchTarget struct {
	id      uint32
	info    mergeBlockInformation
	isMerge bool
}

// branchConditional parses OpBranchConditional: an If whose arms start at the
// two targets, or, at the end of a loop's continuing body, the loop's
// break-if condition.
func (p *parser) branchConditional(inst instruction, fs *functionState, bs *blockState) error {
	if _, err := inst.expectAtLeast(4); err != nil {
		return err
	}
	conditionID, err := p.next()
	if err != nil {
		return err
	}
	lookup, err := p.lookupExpr(conditionID)
	if err != nil {
		return err
	}
	// Unlike Rust naga, a condition that is not a bool scalar is refused here:
	// it would become an If or a negated break-if the IR cannot type.
	conditionType, err := p.typeLookup(lookup.typeID)
	if err != nil {
		return err
	}
	if scalar, ok := p.module.Types[conditionType.handle].Inner.(ir.ScalarType); !ok || scalar.Kind != ir.ScalarBool {
		return errInvalidConditionType(conditionType.handle)
	}
	condition, err := p.getExprHandle(conditionID, lookup, fs, &bs.consumer, &bs.block, bs.bodyIdx)
	if err != nil {
		return err
	}
	var targets [2]branchTarget
	for i := range targets {
		id, err := p.next()
		if err != nil {
			return err
		}
		info, isMerge := fs.mergers[id]
		targets[i] = branchTarget{id: id, info: info, isMerge: isMerge}
	}
	// The branch weights are not carried into the IR.
	for range inst.wc - 4 {
		if _, err := p.next(); err != nil {
			return err
		}
	}

	if p.breakIf(fs, bs, condition, targets) {
		fs.finishBlock(bs, nil)
		return nil
	}

	// Every other target lies ahead: the structured algorithm cannot express
	// a branch back to a block already placed, which Rust naga would move
	// into a new body.
	for _, target := range targets {
		if target.isMerge && target.info == mergeLoopMerge && fs.breakLeavesSwitch(bs.bodyIdx) {
			return errSwitchExitsLoop
		}
		if _, parsed := fs.blocks[target.id]; !target.isMerge && (parsed || target.id == bs.id) {
			if loop := fs.misplacedLoop(bs.bodyIdx); loop != nil {
				return errLoopMergeBeforeContinue(loop.merge)
			}
			return errInvalidBranchTarget(target.id)
		}
	}

	// A target that is another case of the switch around this case is a
	// fall-through: its arm stays empty, so the If ends the case body and the
	// case falls through. Unlike Rust naga, whose arm takes that case's block
	// and leaves the case itself empty.
	var caseTarget [2]bool
	for i, target := range targets {
		if target.isMerge {
			continue
		}
		ref, ok := fs.caseLabels[target.id]
		if !ok {
			continue
		}
		if i == 1 && caseTarget[0] && targets[0].id != target.id {
			return errFallThroughIntoTwoCases(targets[0].id, target.id)
		}
		if err := fs.recordFallThrough(bs.bodyIdx, ref); err != nil {
			return err
		}
		caseTarget[i] = true
	}

	fs.finishBlock(bs, nil)
	accept, reject := targets[0], targets[1]
	if accept.id == reject.id {
		switch {
		case accept.isMerge:
			merger(&fs.bodies[bs.bodyIdx], accept.info)
		case caseTarget[0]:
		default:
			fs.bodyForLabel[accept.id] = bs.bodyIdx
		}
		return nil
	}

	depth := fs.bodies[bs.bodyIdx].depth + 1
	arms := [2]int{}
	for i, target := range targets {
		arm, err := fs.newBody(bs.bodyIdx, depth)
		if err != nil {
			return err
		}
		switch {
		case target.isMerge:
			merger(&fs.bodies[arm], target.info)
		case caseTarget[i]:
		default:
			fs.bodyForLabel[target.id] = arm
		}
		arms[i] = arm
	}
	fs.bodies[bs.bodyIdx].fragments = append(fs.bodies[bs.bodyIdx].fragments, bodyFragment{
		kind:      fragmentIf,
		condition: condition,
		accept:    arms[0],
		reject:    arms[1],
	})
	return nil
}

// breakIf handles an OpBranchConditional at the end of a loop's continuing
// body whose targets are the loop's merge block and its back edge: a
// do-while condition, WGSL's break if. It records the condition (negated
// when the true target is the back edge) on the loop and reports whether it
// did. Both target orders are tried, since SPIR-V is symmetrical here.
func (p *parser) breakIf(fs *functionState, bs *blockState, condition ir.ExpressionHandle, targets [2]branchTarget) bool {
	// OpLoopMerge made the continuing body a child of the loop body, which is
	// a child of the body holding the Loop fragment.
	parent := fs.bodies[bs.bodyIdx].parent
	grandparent := fs.bodies[parent].parent
	fragments := fs.bodies[grandparent].fragments
	if len(fragments) == 0 {
		return false
	}
	loop := &fragments[len(fragments)-1]
	if loop.kind != fragmentLoop || loop.breakIf != nil || loop.continuing != bs.bodyIdx {
		return false
	}
	for _, trueBreaks := range [2]bool{true, false} {
		breakCandidate, backEdge := targets[0], targets[1]
		if !trueBreaks {
			breakCandidate, backEdge = targets[1], targets[0]
		}
		if !breakCandidate.isMerge || breakCandidate.info != mergeLoopMerge {
			continue
		}
		// The back edge goes to the loop header, which OpLoopMerge put in the
		// loop body.
		if backEdge.isMerge {
			continue
		}
		if idx, ok := fs.bodyForLabel[backEdge.id]; !ok || idx != loop.loopBody {
			continue
		}
		breakCondition := condition
		if !trueBreaks {
			breakCondition = appendExpression(fs.fn, ir.ExprUnary{Op: ir.UnaryLogicalNot, Expr: condition})
		}
		loop.breakIf = &breakCondition
		return true
	}
	return false
}

// misplacedLoop returns the loop whose continuing body is bodyIdx when other
// fragments follow it in its enclosing body: the loop's merge block came
// before its continue construct, so breakIf could not find the loop as the
// last fragment. It returns nil otherwise. It runs only on the way to an
// error, so its walk over the fragments is paid once.
func (fs *functionState) misplacedLoop(bodyIdx int) *bodyFragment {
	if fs.bodies[bodyIdx].kind != bodyContinuing {
		return nil
	}
	parent := fs.bodies[bodyIdx].parent
	fragments := fs.bodies[fs.bodies[parent].parent].fragments
	for i := len(fragments) - 2; i >= 0; i-- {
		if fragments[i].kind == fragmentLoop && fragments[i].continuing == bodyIdx {
			return &fragments[i]
		}
	}
	return nil
}

// switchBranch parses OpSwitch: one body per distinct target, extra literals
// of a shared target as empty fall-through cases before it, and a default
// body, with the literals at the default target as empty fall-through cases
// before it. The targets whose blocks go into their own bodies are registered
// as cases, so a branch from one case into another records a fall-through
// edge; the case order follows those edges (lowerSwitchCases). The selector is
// bitcast to i32 when it is unsigned, as the IR switch takes i32 values.
func (p *parser) switchBranch(inst instruction, fs *functionState, bs *blockState) error {
	if _, err := inst.expectAtLeast(3); err != nil {
		return err
	}
	// Each case is a (literal, target) pair of 32-bit words; the selector
	// must be 32-bit for that, which is checked below.
	if (inst.wc-3)%2 != 0 {
		return errInvalidOperandCount(inst.op, inst.wc)
	}
	selectorID, err := p.next()
	if err != nil {
		return err
	}
	defaultID, err := p.next()
	if err != nil {
		return err
	}

	// An OpSelectionMerge before the switch declared its merge block: a
	// branch to it is a Break out of the switch.
	if bs.hasSelectionMerge {
		fs.mergers[bs.selectionMerge] = mergeSwitchMerge
	}

	// A case or the default at the loop's merge block would be a Break that
	// leaves only the switch (errSwitchExitsLoop).
	if fs.mergers[defaultID] == mergeLoopMerge {
		return errSwitchExitsLoop
	}
	// A default at the loop's continue target would need a Continue the
	// default body never gets; spirv-val rejects the shape (the switch header
	// does not dominate that case construct), and Rust naga breaks instead.
	if fs.mergers[defaultID] == mergeLoopContinue {
		return errSwitchDefaultAtContinue(defaultID)
	}

	depth := fs.bodies[bs.bodyIdx].depth + 1
	defaultBody, err := fs.newSwitchCaseBody(bs.bodyIdx, depth)
	if err != nil {
		return err
	}
	_, defaultClaimed := fs.bodyForLabel[defaultID]
	if !defaultClaimed {
		fs.bodyForLabel[defaultID] = defaultBody
	}

	lookup, err := p.lookupExpr(selectorID)
	if err != nil {
		return err
	}
	selectorType, err := p.typeLookup(lookup.typeID)
	if err != nil {
		return err
	}
	scalar, ok := p.module.Types[selectorType.handle].Inner.(ir.ScalarType)
	if !ok || scalar.Width != 4 || (scalar.Kind != ir.ScalarSint && scalar.Kind != ir.ScalarUint) {
		// Rust naga panics on any other selector.
		return errUnsupportedType(selectorType.handle)
	}
	selector, err := p.getExprHandle(selectorID, lookup, fs, &bs.consumer, &bs.block, bs.bodyIdx)
	if err != nil {
		return err
	}
	if scalar.Kind == ir.ScalarUint {
		selector = appendExpression(fs.fn, ir.ExprAs{Expr: selector, Kind: ir.ScalarSint})
	}

	// A literal whose target is the default target (glslang's `case 3:
	// default:`) runs the default body. Rust naga gives such a literal an
	// empty fall-through case among the others, so it runs whatever case
	// follows; here it becomes an empty fall-through case directly before the
	// default. A default that is a merge block has an empty body, and a
	// literal at it keeps the case that breaks, as in Rust naga.
	_, defaultIsMerge := fs.mergers[defaultID]
	var defaultLiterals []int32

	pairs := int(inst.wc-3) / 2
	sw := &switchInfo{targets: make([]switchTarget, 0, pairs+1)}
	targetIndex := make(map[uint32]int)
	seen := make(map[int32]bool, pairs)
	for range pairs {
		word, err := p.next()
		if err != nil {
			return err
		}
		// A 32-bit case literal is the selector's bits, read as i32.
		literal := int32(word)
		targetID, err := p.next()
		if err != nil {
			return err
		}
		if seen[literal] {
			return errRepeatedSwitchCase(literal)
		}
		seen[literal] = true
		if targetID == defaultID && !defaultIsMerge {
			defaultLiterals = append(defaultLiterals, literal)
			continue
		}
		// Cases that share a target are grouped, so the target's body is
		// built once and the other literals fall through into it.
		if i, ok := targetIndex[targetID]; ok {
			sw.targets[i].literals = append(sw.targets[i].literals, literal)
			continue
		}
		if fs.mergers[targetID] == mergeLoopMerge {
			return errSwitchExitsLoop
		}
		caseBody, err := fs.newSwitchCaseBody(bs.bodyIdx, depth)
		if err != nil {
			return err
		}
		_, isMerger := fs.mergers[targetID]
		if isMerger {
			merger(&fs.bodies[caseBody], fs.mergers[targetID])
		}
		_, claimed := fs.bodyForLabel[targetID]
		if !claimed {
			fs.bodyForLabel[targetID] = caseBody
		}
		targetIndex[targetID] = len(sw.targets)
		i := sw.addTarget(targetID, caseBody, []int32{literal}, false)
		if !isMerger && !claimed {
			fs.registerCase(targetID, caseRef{sw: sw, target: i})
		}
	}
	// One case per literal: the extra literals of a target become empty
	// fall-through cases before the target's body, and the literals at the
	// default target empty fall-through cases before the default
	// (lowerSwitchCases). The default is the last target, so without
	// fall-through edges it comes last.
	i := sw.addTarget(defaultID, defaultBody, defaultLiterals, true)
	if !defaultIsMerge && !defaultClaimed {
		fs.registerCase(defaultID, caseRef{sw: sw, target: i})
	}

	fs.finishBlock(bs, nil)
	fs.bodies[bs.bodyIdx].fragments = append(fs.bodies[bs.bodyIdx].fragments, bodyFragment{
		kind:      fragmentSwitch,
		condition: selector,
		sw:        sw,
	})
	return nil
}

// checkLabelsDefined reports a branch, merge or continue target that names no
// block of the function. Rust naga lets such a label pass; it is invalid
// SPIR-V. The smallest such label is reported, so the error is the same from
// run to run.
func (fs *functionState) checkLabelsDefined() error {
	var undefined []uint32
	for label := range fs.bodyForLabel {
		if _, ok := fs.blocks[label]; !ok {
			undefined = append(undefined, label)
		}
	}
	if len(undefined) == 0 {
		return nil
	}
	sort.Slice(undefined, func(i, j int) bool { return undefined[i] < undefined[j] })
	return errUndefinedLabel(undefined[0])
}
