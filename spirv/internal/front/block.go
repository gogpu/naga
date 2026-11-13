package front

import (
	"github.com/gogpu/naga/ir"
)

// lookupExpression is one entry of the SPIR-V expression id table (Rust
// naga's LookupExpression): the IR expression, the SPIR-V type of the result,
// and the label of the block that defines it, 0 for globals, constants and
// parameters.
type lookupExpression struct {
	handle  ir.ExpressionHandle
	typeID  uint32
	blockID uint32
}

// functionState is the function being parsed: its IR function, the
// control-flow-free blocks seen so far, and the body tree they assemble into
// (Rust naga's BlockContext).
type functionState struct {
	id     uint32
	fn     *ir.Function
	blocks map[uint32]ir.Block
	// bodyForLabel is the body each block label contributes to. A label gets
	// its entry when it is first branched to, before its block is parsed.
	bodyForLabel map[uint32]int
	// mergers says what a branch to a merge block or a continue target
	// means: the end of the construct that declared it.
	mergers map[uint32]mergeBlockInformation
	// caseLabels are the switch case and default targets whose blocks the
	// switch placed in their own bodies, and caseBodies the same targets keyed
	// by body: a branch from one of these bodies to another's label is a
	// fall-through. Both are created by the first OpSwitch.
	caseLabels map[uint32]caseRef
	caseBodies map[int]caseRef
	bodies     []body
	// phis are the local variables phi nodes and out-of-scope spills write
	// into, with the values each is written from.
	phis []phiExpression
	// combinedSamplerExpr is the sampler global's expression for each combined
	// image sampler variable of the function, by the image global's handle: a
	// load of the variable yields the pair.
	combinedSamplerExpr map[ir.GlobalVariableHandle]ir.ExpressionHandle
	// parameterSampling is how this function's arguments are sampled (Rust
	// naga's parameter_sampling): an image or sampler passed as an argument is
	// sampled through the caller's globals, so the flags travel back to them
	// once every function is parsed.
	parameterSampling []samplingFlags
}

// mergeBlockInformation is what a branch to a merge block or a continue
// target ends (Rust naga's MergeBlockInformation).
type mergeBlockInformation uint8

const (
	// mergeLoopMerge: a branch to a loop's merge block is a Break.
	mergeLoopMerge mergeBlockInformation = iota + 1
	// mergeLoopContinue: a branch to a loop's continue target is a Continue.
	mergeLoopContinue
	// mergeSelectionMerge: a branch to a selection's merge block falls off the
	// end of the If's accept or reject block.
	mergeSelectionMerge
	// mergeSwitchMerge: a branch to a switch's merge block is a Break.
	mergeSwitchMerge
)

// bodyFragmentKind says what one piece of a body is.
type bodyFragmentKind uint8

const (
	// fragmentBlock is the statements of one SPIR-V block.
	fragmentBlock bodyFragmentKind = iota
	// fragmentIf is an If whose arms are the bodies accept and reject.
	fragmentIf
	// fragmentLoop is a Loop whose body and continuing are bodies.
	fragmentLoop
	// fragmentSwitch is a Switch whose cases and default are bodies.
	fragmentSwitch
	// fragmentBreak is a Break statement.
	fragmentBreak
	// fragmentContinue is a Continue statement.
	fragmentContinue
)

// bodyFragment is one piece of a body (Rust naga's BodyFragment). The fields
// a fragment uses depend on its kind; body indices refer to
// functionState.bodies.
type bodyFragment struct {
	kind    bodyFragmentKind
	blockID uint32

	// fragmentIf: the condition and the bodies of the two arms.
	// fragmentSwitch: the selector, in condition.
	condition ir.ExpressionHandle
	accept    int
	reject    int

	// fragmentLoop: the loop body, the continuing body, the optional
	// break-if condition and the label of the loop's merge block.
	loopBody   int
	continuing int
	breakIf    *ir.ExpressionHandle
	merge      uint32

	// fragmentSwitch: the targets and their fall-through edges.
	sw *switchInfo
}

// switchInfo is the targets of one OpSwitch: the distinct literal targets in
// instruction order, then the default. Fall-through edges between them are
// recorded while the case blocks are parsed; the case order is chosen from
// them when the switch is lowered.
type switchInfo struct {
	targets []switchTarget
}

// switchTarget is one distinct target of an OpSwitch: its label, the body
// created for it and the literals that branch to it, in instruction order.
// For the default, literals are the literals at the default target, which
// all become empty fall-through cases before the default body. fallsInto and
// fallsFrom are the indices of the targets this one falls through into and
// from, -1 for none.
type switchTarget struct {
	label     uint32
	body      int
	literals  []int32
	isDefault bool
	fallsInto int
	fallsFrom int
}

// caseRef names one target of one switch.
type caseRef struct {
	sw     *switchInfo
	target int
}

// addTarget appends a target that falls through nowhere yet.
func (sw *switchInfo) addTarget(label uint32, bodyIdx int, literals []int32, isDefault bool) int {
	sw.targets = append(sw.targets, switchTarget{
		label:     label,
		body:      bodyIdx,
		literals:  literals,
		isDefault: isDefault,
		fallsInto: -1,
		fallsFrom: -1,
	})
	return len(sw.targets) - 1
}

// order returns the target indices in IR case order. The IR falls through
// only into the next case, so every fall-through chain is placed contiguously;
// chains start in target order, which keeps SPIR-V's own fall-through order
// (spirv-val requires a literal target to immediately precede the literal
// target it falls into) and a default without edges last, as Rust naga places
// it. Each target falls from at most one other (recordFallThrough), so a chain
// started at a target nothing falls into never meets a cycle; the targets left
// over are on one.
func (sw *switchInfo) order() ([]int, error) {
	out := make([]int, 0, len(sw.targets))
	for i := range sw.targets {
		if sw.targets[i].fallsFrom >= 0 {
			continue
		}
		for j := i; j >= 0; j = sw.targets[j].fallsInto {
			out = append(out, j)
		}
	}
	if len(out) == len(sw.targets) {
		return out, nil
	}
	placed := make([]bool, len(sw.targets))
	for _, i := range out {
		placed[i] = true
	}
	for i := range sw.targets {
		if !placed[i] {
			return nil, errFallThroughCycle(sw.targets[i].label)
		}
	}
	return out, nil
}

// recordFallThrough records that the case body fromBody, which ends with a
// branch to the label of switch target to, falls through into it. The branch
// must come from a body that is itself one of the same switch's targets: a
// branch from inside a nested construct would leave statements after that
// construct to run first. A target takes at most one fall-through in and out.
func (fs *functionState) recordFallThrough(fromBody int, to caseRef) error {
	sw := to.sw
	label := sw.targets[to.target].label
	from, ok := fs.caseBodies[fromBody]
	if !ok || from.sw != sw {
		return errFallThroughFromOutside(label)
	}
	src := &sw.targets[from.target]
	if src.fallsInto == to.target {
		return nil
	}
	if src.fallsInto >= 0 {
		return errFallThroughIntoTwoCases(sw.targets[src.fallsInto].label, label)
	}
	if sw.targets[to.target].fallsFrom >= 0 {
		return errMultipleFallThrough(label)
	}
	src.fallsInto = to.target
	sw.targets[to.target].fallsFrom = from.target
	return nil
}

// registerCase makes a switch target's label and body known as a case, so a
// branch to the label from another case body is a fall-through.
func (fs *functionState) registerCase(label uint32, ref caseRef) {
	if fs.caseLabels == nil {
		fs.caseLabels = make(map[uint32]caseRef)
		fs.caseBodies = make(map[int]caseRef)
	}
	fs.caseLabels[label] = ref
	fs.caseBodies[ref.sw.targets[ref.target].body] = ref
}

// body is one block of the final IR and the fragments that build it. Its
// parent is the body containing the construct it belongs to; the function's
// top-level body is its own parent. depth is how deeply the body is nested in
// the final IR, which bounds the lowering recursion.
type body struct {
	parent    int
	depth     int
	kind      bodyKind
	fragments []bodyFragment
}

// bodyKind is the construct a body is the inside of, where that matters for
// what a branch out of it may stand for.
type bodyKind uint8

const (
	// bodyPlain is the function's top level or an If arm.
	bodyPlain bodyKind = iota
	// bodyLoop is a loop body.
	bodyLoop
	// bodyContinuing is a loop's continuing body.
	bodyContinuing
	// bodySwitchCase is a switch case or default body.
	bodySwitchCase
)

// breakLeavesSwitch reports whether a Break placed in bodyIdx would end a
// switch rather than the innermost loop around it: the IR's Break ends the
// innermost loop or switch, so a branch from a switch case to the loop's
// merge block cannot be expressed as one. The walk follows the parent chain,
// as isParent does.
func (fs *functionState) breakLeavesSwitch(bodyIdx int) bool {
	for b := bodyIdx; ; b = fs.bodies[b].parent {
		switch fs.bodies[b].kind {
		case bodySwitchCase:
			return true
		case bodyLoop:
			return false
		}
		if b == 0 {
			return false
		}
	}
}

// isBackEdge reports whether a branch from bodyIdx to targetID is a loop's
// back edge: the target is the header of a loop (the header belongs to the
// loop body) and the branch comes from that loop's continuing body. A loop
// header can also be a switch case target, so this is checked before a
// branch to a case label is taken for a fall-through.
func (fs *functionState) isBackEdge(bodyIdx int, targetID uint32) bool {
	loopBody, ok := fs.bodyForLabel[targetID]
	if !ok || fs.bodies[loopBody].kind != bodyLoop {
		return false
	}
	for b := bodyIdx; ; b = fs.bodies[b].parent {
		if fs.bodies[b].kind == bodyContinuing && fs.bodies[b].parent == loopBody {
			return true
		}
		if b == 0 {
			return false
		}
	}
}

// newBody appends a body nested in parent at the given depth and returns its
// index. A body deeper than maxNesting is refused before it is created.
func (fs *functionState) newBody(parent, depth int) (int, error) {
	if depth > maxNesting {
		return 0, errNestingTooDeep
	}
	fs.bodies = append(fs.bodies, body{parent: parent, depth: depth})
	return len(fs.bodies) - 1, nil
}

// newSwitchCaseBody is newBody for a switch case or default whose target
// block will be placed in it.
func (fs *functionState) newSwitchCaseBody(parent, depth int) (int, error) {
	idx, err := fs.newBody(parent, depth)
	if err != nil {
		return 0, err
	}
	fs.bodies[idx].kind = bodySwitchCase
	return idx, nil
}

// merger extends a body with the statement a branch to a merge block or a
// continue target stands for, as Rust naga's merger in next_block.
func merger(b *body, info mergeBlockInformation) {
	switch info {
	case mergeLoopContinue:
		b.fragments = append(b.fragments, bodyFragment{kind: fragmentContinue})
	case mergeLoopMerge, mergeSwitchMerge:
		b.fragments = append(b.fragments, bodyFragment{kind: fragmentBreak})
	case mergeSelectionMerge:
		// Finishing a selection means falling off the end of the If's arm.
	}
}

// emitter collects the expressions one statement needs into a single Emit
// statement, as Rust naga's proc::Emitter. Expressions are appended to the
// function's arena; start records where a statement's expressions begin and
// finish turns the range into an Emit statement.
type emitter struct {
	start   int
	running bool
}

// startWith begins collecting expressions from the arena's current end.
func (e *emitter) startWith(expressions []ir.Expression) {
	e.start = len(expressions)
	e.running = true
}

// finish returns the Emit statement for the collected expressions, if any
// were appended, and stops collecting.
func (e *emitter) finish(expressions []ir.Expression) (ir.Statement, bool) {
	if !e.running {
		return ir.Statement{}, false
	}
	e.running = false
	if e.start >= len(expressions) {
		return ir.Statement{}, false
	}
	return ir.Statement{Kind: ir.StmtEmit{Range: ir.Range{
		Start: ir.ExpressionHandle(e.start),
		End:   ir.ExpressionHandle(len(expressions)),
	}}}, true
}

// appendExpression appends one expression to the function arena.
func appendExpression(fn *ir.Function, kind ir.ExpressionKind) ir.ExpressionHandle {
	fn.Expressions = append(fn.Expressions, ir.Expression{Kind: kind})
	return ir.ExpressionHandle(len(fn.Expressions) - 1)
}

// getExprHandle returns the expression a SPIR-V id can be referred to by in
// bodyIdx, as Rust naga's Frontend::get_expr_handle: the expression itself
// when it is in scope there, or a fresh Load of a temporary the expression is
// spilled to.
func (p *parser) getExprHandle(id uint32, lookup lookupExpression, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int) (ir.ExpressionHandle, error) {
	exprBodyIdx := 0
	if idx, ok := fs.bodyForLabel[lookup.blockID]; ok {
		exprBodyIdx = idx
	}
	if isParent(bodyIdx, exprBodyIdx, fs) {
		return lookup.handle, nil
	}
	ty, err := p.typeLookup(lookup.typeID)
	if err != nil {
		return 0, err
	}
	local := ir.LocalVariable{Type: ty.handle}
	fs.fn.LocalVars = append(fs.fn.LocalVars, local)
	localHandle := ir.LocalVariableHandle(len(fs.fn.LocalVars) - 1)

	if statement, ok := consumer.finish(fs.fn.Expressions); ok {
		*block = append(*block, statement)
	}
	pointer := appendExpression(fs.fn, ir.ExprLocalVariable{Variable: localHandle})
	consumer.startWith(fs.fn.Expressions)
	load := appendExpression(fs.fn, ir.ExprLoad{Pointer: pointer})

	// A synthetic phi, so the value is spilled where it is still in scope,
	// as Rust naga does.
	fs.phis = append(fs.phis, phiExpression{
		local:       localHandle,
		expressions: []phiSource{{id: id, blockID: lookup.blockID}},
	})
	return load, nil
}

// phiExpression is a local variable a phi node (or a spill) stores into, with
// the blocks and values it is written from.
type phiExpression struct {
	local       ir.LocalVariableHandle
	expressions []phiSource
}

// phiSource is one (SPIR-V id of the value, predecessor block) pair of a phi.
type phiSource struct {
	id      uint32
	blockID uint32
}

// isParent reports whether child is parent or inside it, as Rust naga's
// is_parent.
func isParent(child, parent int, fs *functionState) bool {
	for {
		if child == parent {
			return true
		}
		if child == 0 {
			return false
		}
		child = fs.bodies[child].parent
	}
}

// controlFlow is what a statement may do where it stands: the abilities the
// validator grants a block (return outside a continuing block, break inside a
// loop body or a switch, continue inside a loop body).
type controlFlow uint8

const (
	flowReturn controlFlow = 1 << iota
	flowBreak
	flowContinue
)

// lower assembles the body tree into the function's IR block, as Rust naga's
// BlockContext::lower.
func (fs *functionState) lower() (ir.Block, error) {
	return fs.lowerImpl(0, flowReturn)
}

// lowerImpl assembles one body, as Rust naga's lower_impl. Every child body
// is one level deeper than its parent and newBody caps the depth, so the
// recursion is bounded by maxNesting.
//
// Unlike Rust naga, a Break, Continue or Return the structured input put
// where the IR cannot express it (a branch to a loop's merge block from
// outside the loop, a return inside a continuing block) is an error here,
// not a module the validator rejects later.
func (fs *functionState) lowerImpl(bodyIdx int, flow controlFlow) (ir.Block, error) {
	var block ir.Block
	for i := range fs.bodies[bodyIdx].fragments {
		fragment := &fs.bodies[bodyIdx].fragments[i]
		switch fragment.kind {
		case fragmentBlock:
			statements := fs.blocks[fragment.blockID]
			if n := len(statements); n > 0 && flow&flowReturn == 0 {
				if _, ok := statements[n-1].Kind.(ir.StmtReturn); ok {
					return nil, errInvalidTerminator
				}
			}
			block = append(block, statements...)
		case fragmentIf:
			accept, err := fs.lowerImpl(fragment.accept, flow)
			if err != nil {
				return nil, err
			}
			reject, err := fs.lowerImpl(fragment.reject, flow)
			if err != nil {
				return nil, err
			}
			block = append(block, ir.Statement{Kind: ir.StmtIf{
				Condition: fragment.condition,
				Accept:    accept,
				Reject:    reject,
			}})
		case fragmentLoop:
			loopBody, err := fs.lowerImpl(fragment.loopBody, flow&flowReturn|flowBreak|flowContinue)
			if err != nil {
				return nil, err
			}
			continuing, err := fs.lowerImpl(fragment.continuing, 0)
			if err != nil {
				return nil, err
			}
			block = append(block, ir.Statement{Kind: ir.StmtLoop{
				Body:       loopBody,
				Continuing: continuing,
				BreakIf:    fragment.breakIf,
			}})
		case fragmentSwitch:
			cases, err := fs.lowerSwitchCases(fragment.sw, flow&(flowReturn|flowContinue)|flowBreak)
			if err != nil {
				return nil, err
			}
			block = append(block, ir.Statement{Kind: ir.StmtSwitch{Selector: fragment.condition, Cases: cases}})
		case fragmentBreak:
			if flow&flowBreak == 0 {
				return nil, errInvalidTerminator
			}
			block = append(block, ir.Statement{Kind: ir.StmtBreak{}})
		case fragmentContinue:
			if flow&flowContinue == 0 {
				return nil, errInvalidTerminator
			}
			block = append(block, ir.Statement{Kind: ir.StmtContinue{}})
		}
	}
	return block, nil
}

// lowerSwitchCases builds the IR cases of a switch in fall-through order
// (switchInfo.order). Each target gives its extra literals as empty
// fall-through cases, then its first literal (or the default) with its body,
// which falls through exactly when the target has a fall-through edge. The
// last case never does.
//
// Rust naga sets fall-through on every case whose body does not end in a
// terminator and always puts the default last without it
// (next_block.rs:2128-2240, function.rs lower_impl), so a default that
// branches into a case, and a case that branches into the default, lose
// statements or run the wrong case.
func (fs *functionState) lowerSwitchCases(sw *switchInfo, caseFlow controlFlow) ([]ir.SwitchCase, error) {
	order, err := sw.order()
	if err != nil {
		return nil, err
	}
	cases := make([]ir.SwitchCase, 0, len(sw.targets)+1)
	for _, i := range order {
		target := &sw.targets[i]
		caseBody, err := fs.lowerImpl(target.body, caseFlow)
		if err != nil {
			return nil, err
		}
		var value ir.SwitchValue = ir.SwitchValueDefault{}
		empty := target.literals
		if !target.isDefault {
			value = ir.SwitchValueI32(target.literals[0])
			empty = target.literals[1:]
		}
		for _, literal := range empty {
			cases = append(cases, ir.SwitchCase{Value: ir.SwitchValueI32(literal), FallThrough: true})
		}
		cases = append(cases, ir.SwitchCase{
			Value:       value,
			Body:        caseBody,
			FallThrough: target.fallsInto >= 0,
		})
	}
	return cases, nil
}

// isTerminator reports whether a statement ends its block, as Rust naga's
// Statement::is_terminator.
func isTerminator(statement ir.Statement) bool {
	switch statement.Kind.(type) {
	case ir.StmtBreak, ir.StmtContinue, ir.StmtReturn, ir.StmtKill:
		return true
	}
	return false
}
