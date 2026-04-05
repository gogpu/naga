package emit

import (
	"fmt"

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
		// Simplified: emit both branches sequentially.
		// Proper implementation needs basic block splitting.
		// For MVP, we emit only the accept block (assumes simple cases).
		return e.emitBlock(fn, sk.Accept)

	case ir.StmtLoop:
		// Simplified: emit loop body once.
		// Proper loops need back-edge basic blocks.
		return e.emitBlock(fn, sk.Body)

	case ir.StmtBreak, ir.StmtContinue:
		// Control flow stubs — handled by loop/switch structure.
		return nil

	case ir.StmtKill:
		// Fragment shader discard — not yet implemented.
		return nil

	case ir.StmtCall:
		// Function calls.
		return e.emitStmtCall(fn, sk)

	default:
		// Unsupported statement kinds are silently skipped for MVP.
		return nil
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
// In naga IR, stores write a value through a pointer expression.
func (e *Emitter) emitStmtStore(fn *ir.Function, store ir.StmtStore) error {
	// Evaluate pointer and value.
	_, err := e.emitExpression(fn, store.Pointer)
	if err != nil {
		return fmt.Errorf("store pointer: %w", err)
	}
	_, err = e.emitExpression(fn, store.Value)
	if err != nil {
		return fmt.Errorf("store value: %w", err)
	}
	// In the simplified model, stores through local variables are
	// handled implicitly via value numbering. The pointer expression
	// is mapped to the same value as the stored value.
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

// outputLocationFromBinding extracts the output location from a binding.
func (e *Emitter) outputLocationFromBinding(b *ir.Binding) int {
	if b == nil {
		return 0
	}
	return e.locationFromBinding(*b)
}
