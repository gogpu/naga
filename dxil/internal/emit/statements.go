package emit

import (
	"fmt"

	"github.com/gogpu/naga/dxil/internal/module"
	"github.com/gogpu/naga/ir"
)

// emitFunctionBody emits the statements in the function body.
//
// This walks the naga IR statement tree and emits:
// - StmtEmit: evaluates expressions in the emit range
// - StmtReturn: stores outputs and returns
// - StmtStore: stores values through pointers (simplified)
// - StmtIf: conditional branching (not yet implemented)
// - StmtLoop: loop constructs (not yet implemented)
// - StmtBlock: nested statement blocks
//
// Reference: Mesa nir_to_dxil.c emit_module()
func (e *Emitter) emitFunctionBody(fn *ir.Function) error {
	return e.emitBlock(fn, fn.Body)
}

// emitBlock emits all statements in a block.
func (e *Emitter) emitBlock(fn *ir.Function, block ir.Block) error {
	for i := range block {
		if err := e.emitStatement(fn, &block[i]); err != nil {
			return err
		}
	}
	return nil
}

// emitStatement emits a single statement.
func (e *Emitter) emitStatement(fn *ir.Function, stmt *ir.Statement) error {
	switch sk := stmt.Kind.(type) {
	case ir.StmtEmit:
		return e.emitStmtEmit(fn, sk)

	case ir.StmtReturn:
		return e.emitStmtReturn(fn, sk)

	case ir.StmtStore:
		return e.emitStmtStore(fn, sk)

	case ir.StmtBlock:
		return e.emitBlock(fn, sk.Block)

	case ir.StmtIf:
		return e.emitIfStatement(fn, sk)

	case ir.StmtLoop:
		return e.emitLoopStatement(fn, sk)

	case ir.StmtBreak:
		return e.emitBreak()

	case ir.StmtContinue:
		return e.emitContinue()

	case ir.StmtKill:
		// Fragment shader discard — not yet implemented.
		return nil

	case ir.StmtCall:
		// Function calls.
		return e.emitStmtCall(fn, sk)

	default:
		return fmt.Errorf("unsupported statement kind: %T", sk)
	}
}

// emitStmtEmit evaluates expressions in the emit range.
// In naga IR, StmtEmit marks when expressions should be materialized.
func (e *Emitter) emitStmtEmit(fn *ir.Function, emit ir.StmtEmit) error {
	for h := emit.Range.Start; h < emit.Range.End; h++ {
		if _, err := e.emitExpression(fn, h); err != nil {
			return fmt.Errorf("emit range [%d]: %w", h, err)
		}
	}
	return nil
}

// emitStmtReturn handles the return statement.
//
// For entry point functions with a result binding, the return value is
// stored via dx.op.storeOutput before the void return. This matches how
// DXIL entry points work: they are void functions that store outputs
// via intrinsics.
func (e *Emitter) emitStmtReturn(fn *ir.Function, ret ir.StmtReturn) error {
	if ret.Value == nil || fn.Result == nil {
		return nil
	}

	// Evaluate the return value expression.
	valueID, err := e.emitExpression(fn, *ret.Value)
	if err != nil {
		return fmt.Errorf("return value: %w", err)
	}

	// Store the return value as output(s).
	resultType := e.ir.Types[fn.Result.Type]
	scalar, ok := scalarOfType(resultType.Inner)
	if !ok {
		// Non-scalar result type — try to handle vector.
		return e.emitReturnComposite(fn, valueID, resultType.Inner)
	}

	numComps := componentCount(resultType.Inner)
	ol := overloadForScalar(scalar)
	outputID := e.outputLocationFromBinding(fn.Result.Binding)

	for comp := 0; comp < numComps; comp++ {
		compValueID := e.getComponentID(*ret.Value, comp)
		e.emitOutputStore(outputID, comp, compValueID, ol)
	}

	return nil
}

// emitReturnComposite handles returning composite types (vectors, structs).
func (e *Emitter) emitReturnComposite(fn *ir.Function, valueID int, inner ir.TypeInner) error {
	switch t := inner.(type) {
	case ir.VectorType:
		// This is called when scalarOfType fails on the result type,
		// which shouldn't happen for VectorType. Just handle it.
		ol := overloadForScalar(t.Scalar)
		outputID := e.outputLocationFromBinding(fn.Result.Binding)
		for comp := 0; comp < int(t.Size); comp++ {
			e.emitOutputStore(outputID, comp, valueID+comp, ol)
		}
		return nil

	case ir.StructType:
		// Struct outputs: each member has its own binding.
		for _, member := range t.Members {
			if member.Binding == nil {
				continue
			}
			memberType := e.ir.Types[member.Type]
			scalar, ok := scalarOfType(memberType.Inner)
			if !ok {
				continue
			}
			numComps := componentCount(memberType.Inner)
			ol := overloadForScalar(scalar)
			outputID := e.locationFromBinding(*member.Binding)
			for comp := 0; comp < numComps; comp++ {
				e.emitOutputStore(outputID, comp, valueID+comp, ol)
			}
		}
		return nil

	default:
		return fmt.Errorf("unsupported return type: %T", inner)
	}
}

// emitStmtStore handles store statements.
//
// In naga IR, stores write a value through a pointer expression.
// In DXIL, this emits an LLVM store instruction:
//
//	store TYPE %value, TYPE* %ptr, align N
//
// Reference: Mesa nir_to_dxil.c dxil_emit_store()
func (e *Emitter) emitStmtStore(fn *ir.Function, store ir.StmtStore) error {
	ptr, err := e.emitExpression(fn, store.Pointer)
	if err != nil {
		return fmt.Errorf("store pointer: %w", err)
	}
	value, err := e.emitExpression(fn, store.Value)
	if err != nil {
		return fmt.Errorf("store value: %w", err)
	}

	// Resolve the stored value's type for alignment.
	storedTy, err := e.resolveLoadType(fn, store.Pointer)
	if err != nil {
		return fmt.Errorf("store type resolution: %w", err)
	}
	align := e.alignForType(storedTy)

	// Emit LLVM store instruction (no value produced).
	instr := &module.Instruction{
		Kind:        module.InstrStore,
		HasValue:    false,
		Operands:    []int{ptr, value, align, 0}, // ptr, value, align, isVolatile
		ReturnValue: -1,
	}
	e.currentBB.AddInstruction(instr)
	return nil
}

// emitStmtCall handles function call statements.
func (e *Emitter) emitStmtCall(fn *ir.Function, call ir.StmtCall) error {
	// Evaluate arguments.
	for _, arg := range call.Arguments {
		if _, err := e.emitExpression(fn, arg); err != nil {
			return fmt.Errorf("call argument: %w", err)
		}
	}
	return nil
}

// emitIfStatement emits an if/else construct using LLVM basic blocks.
//
// DXIL uses basic block branching for control flow:
//
//	current_bb:
//	  br i1 %cond, label %then, label %else_or_merge
//	then_bb:
//	  <accept block>
//	  br label %merge
//	else_bb: (only if reject block is non-empty)
//	  <reject block>
//	  br label %merge
//	merge_bb:
//	  <continues>
//
// Reference: Mesa nir_to_dxil.c emit_cf_list / emit_if
func (e *Emitter) emitIfStatement(fn *ir.Function, stmt ir.StmtIf) error {
	// Emit the condition expression.
	condID, err := e.emitExpression(fn, stmt.Condition)
	if err != nil {
		return fmt.Errorf("if condition: %w", err)
	}

	hasReject := len(stmt.Reject) > 0

	// Create basic blocks. We add them to mainFn now so we can
	// reference their indices for branch instructions.
	thenBB := e.mainFn.AddBasicBlock("if.then")
	thenBBIndex := len(e.mainFn.BasicBlocks) - 1

	var elseBBIndex int
	if hasReject {
		elseBB := e.mainFn.AddBasicBlock("if.else")
		elseBBIndex = len(e.mainFn.BasicBlocks) - 1
		_ = elseBB // used below via index
	}

	mergeBB := e.mainFn.AddBasicBlock("if.merge")
	mergeBBIndex := len(e.mainFn.BasicBlocks) - 1

	// Emit conditional branch from the current BB.
	falseBBIndex := mergeBBIndex
	if hasReject {
		falseBBIndex = elseBBIndex
	}
	e.currentBB.AddInstruction(module.NewBrCondInstr(thenBBIndex, falseBBIndex, condID))

	// Emit accept (then) block.
	e.currentBB = thenBB
	if err := e.emitBlock(fn, stmt.Accept); err != nil {
		return fmt.Errorf("if accept: %w", err)
	}
	// Branch from then to merge (unless block already ends with a terminator).
	if !e.blockHasTerminator(e.currentBB) {
		e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
	}

	// Emit reject (else) block if present.
	if hasReject {
		e.currentBB = e.mainFn.BasicBlocks[elseBBIndex]
		if err := e.emitBlock(fn, stmt.Reject); err != nil {
			return fmt.Errorf("if reject: %w", err)
		}
		if !e.blockHasTerminator(e.currentBB) {
			e.currentBB.AddInstruction(module.NewBrInstr(mergeBBIndex))
		}
	}

	// Continue emitting into the merge block.
	e.currentBB = mergeBB
	return nil
}

// emitLoopStatement emits a loop construct using LLVM basic blocks.
//
// DXIL loop structure:
//
//	current_bb:
//	  br label %loop_header
//	loop_header:
//	  br label %loop_body
//	loop_body:
//	  <body statements>
//	  br label %loop_continuing  (or break → br %loop_merge)
//	loop_continuing:
//	  <continuing statements>
//	  br label %loop_header      (back edge)
//	loop_merge:
//	  <continues after loop>
//
// Reference: Mesa nir_to_dxil.c emit_cf_list / emit_loop
func (e *Emitter) emitLoopStatement(fn *ir.Function, stmt ir.StmtLoop) error {
	// Create basic blocks for the loop structure.
	headerBB := e.mainFn.AddBasicBlock("loop.header")
	headerBBIndex := len(e.mainFn.BasicBlocks) - 1

	bodyBB := e.mainFn.AddBasicBlock("loop.body")
	bodyBBIndex := len(e.mainFn.BasicBlocks) - 1
	_ = bodyBBIndex // used implicitly via headerBB branch

	continuingBB := e.mainFn.AddBasicBlock("loop.continuing")
	continuingBBIndex := len(e.mainFn.BasicBlocks) - 1

	mergeBB := e.mainFn.AddBasicBlock("loop.merge")
	mergeBBIndex := len(e.mainFn.BasicBlocks) - 1

	// Branch from current BB to loop header.
	e.currentBB.AddInstruction(module.NewBrInstr(headerBBIndex))

	// Header branches unconditionally to body.
	headerBB.AddInstruction(module.NewBrInstr(bodyBBIndex))

	// Push loop context for break/continue.
	e.loopStack = append(e.loopStack, loopContext{
		continuingBBIndex: continuingBBIndex,
		mergeBBIndex:      mergeBBIndex,
	})

	// Emit body.
	e.currentBB = bodyBB
	if err := e.emitBlock(fn, stmt.Body); err != nil {
		return fmt.Errorf("loop body: %w", err)
	}
	// After body, branch to continuing (unless terminated by break/continue/return).
	if !e.blockHasTerminator(e.currentBB) {
		e.currentBB.AddInstruction(module.NewBrInstr(continuingBBIndex))
	}

	// Emit continuing block.
	e.currentBB = continuingBB
	if len(stmt.Continuing) > 0 {
		if err := e.emitBlock(fn, stmt.Continuing); err != nil {
			return fmt.Errorf("loop continuing: %w", err)
		}
	}

	// Handle break_if: conditional back-edge vs break.
	if stmt.BreakIf != nil {
		breakCondID, err := e.emitExpression(fn, *stmt.BreakIf)
		if err != nil {
			return fmt.Errorf("loop break_if: %w", err)
		}
		// If break condition is true, go to merge; otherwise loop back.
		e.currentBB.AddInstruction(module.NewBrCondInstr(mergeBBIndex, headerBBIndex, breakCondID))
	} else if !e.blockHasTerminator(e.currentBB) {
		// Unconditional back edge.
		e.currentBB.AddInstruction(module.NewBrInstr(headerBBIndex))
	}

	// Pop loop context.
	e.loopStack = e.loopStack[:len(e.loopStack)-1]

	// Continue emitting into merge block.
	e.currentBB = mergeBB
	return nil
}

// emitBreak emits a branch to the loop merge block (exits the loop).
func (e *Emitter) emitBreak() error {
	if len(e.loopStack) == 0 {
		return fmt.Errorf("break outside of loop")
	}
	ctx := e.loopStack[len(e.loopStack)-1]
	e.currentBB.AddInstruction(module.NewBrInstr(ctx.mergeBBIndex))
	return nil
}

// emitContinue emits a branch to the loop continuing block.
func (e *Emitter) emitContinue() error {
	if len(e.loopStack) == 0 {
		return fmt.Errorf("continue outside of loop")
	}
	ctx := e.loopStack[len(e.loopStack)-1]
	e.currentBB.AddInstruction(module.NewBrInstr(ctx.continuingBBIndex))
	return nil
}

// blockHasTerminator checks whether the basic block already ends with
// a terminator instruction (return or branch).
func (e *Emitter) blockHasTerminator(bb *module.BasicBlock) bool {
	if len(bb.Instructions) == 0 {
		return false
	}
	last := bb.Instructions[len(bb.Instructions)-1]
	return last.Kind == module.InstrRet || last.Kind == module.InstrBr
}

// outputLocationFromBinding extracts the output location from a binding.
func (e *Emitter) outputLocationFromBinding(b *ir.Binding) int {
	if b == nil {
		return 0
	}
	return e.locationFromBinding(*b)
}
