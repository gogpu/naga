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
// (Rust naga's BlockContext, minus the parts control flow adds).
type functionState struct {
	id           uint32
	fn           *ir.Function
	blocks       map[uint32]ir.Block
	bodyForLabel map[uint32]int
	bodies       []body
	// phis are the local variables phi nodes and out-of-scope spills write
	// into, with the values each is written from.
	phis []phiExpression
}

// bodyFragmentKind says what one piece of a body is. Control flow adds If,
// Loop, Switch, Break and Continue fragments.
type bodyFragmentKind uint8

const (
	fragmentBlock bodyFragmentKind = iota
)

// bodyFragment is one piece of a body.
type bodyFragment struct {
	kind    bodyFragmentKind
	blockID uint32
}

// body is one block of the final IR and the fragments that build it. Its
// parent is the body containing the construct it belongs to; the function's
// top-level body is its own parent.
type body struct {
	parent    int
	fragments []bodyFragment
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

// lower assembles the body tree into the function's IR block, as Rust naga's
// BlockContext::lower.
func (fs *functionState) lower() ir.Block {
	var block ir.Block
	for _, fragment := range fs.bodies[0].fragments {
		block = append(block, fs.blocks[fragment.blockID]...)
	}
	return block
}
