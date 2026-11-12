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
	bodies  []body
	// phis are the local variables phi nodes and out-of-scope spills write
	// into, with the values each is written from.
	phis []phiExpression
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

	// fragmentLoop: the loop body, the continuing body and the optional
	// break-if condition.
	loopBody   int
	continuing int
	breakIf    *ir.ExpressionHandle

	// fragmentSwitch: the cases in order, and the default body.
	cases       []switchCaseBody
	defaultBody int
}

// switchCaseBody is one case of a switch fragment: its value and its body.
type switchCaseBody struct {
	value int32
	body  int
}

// body is one block of the final IR and the fragments that build it. Its
// parent is the body containing the construct it belongs to; the function's
// top-level body is its own parent. depth is how deeply the body is nested in
// the final IR, which bounds the lowering recursion.
type body struct {
	parent    int
	depth     int
	fragments []bodyFragment
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
			caseFlow := flow&(flowReturn|flowContinue) | flowBreak
			cases := make([]ir.SwitchCase, 0, len(fragment.cases)+1)
			for _, c := range fragment.cases {
				caseBody, err := fs.lowerImpl(c.body, caseFlow)
				if err != nil {
					return nil, err
				}
				// A case whose body ends in a terminator would make the
				// fall-through unreachable, as Rust naga notes.
				cases = append(cases, ir.SwitchCase{
					Value:       ir.SwitchValueI32(c.value),
					Body:        caseBody,
					FallThrough: len(caseBody) == 0 || !isTerminator(caseBody[len(caseBody)-1]),
				})
			}
			defaultBody, err := fs.lowerImpl(fragment.defaultBody, caseFlow)
			if err != nil {
				return nil, err
			}
			cases = append(cases, ir.SwitchCase{Value: ir.SwitchValueDefault{}, Body: defaultBody})
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

// isTerminator reports whether a statement ends its block, as Rust naga's
// Statement::is_terminator.
func isTerminator(statement ir.Statement) bool {
	switch statement.Kind.(type) {
	case ir.StmtBreak, ir.StmtContinue, ir.StmtReturn, ir.StmtKill:
		return true
	}
	return false
}
