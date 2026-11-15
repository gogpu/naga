package front

// Recursion in the call graph. GLSL forbids it, and so does Rust naga — its
// validator rejects a call to a function that is not yet processed (a cycle in
// the dependency order, valid/handles.rs). Without the check the module
// lowers, and then the SPIR-V backend fails with an internal error ("function
// N not found") and MSL/HLSL emit a call to a function declared later or not
// at all (deep validation, 2026-11-15, card P1-GLSLIN-005).

import "github.com/gogpu/naga/ir"

// rejectRecursiveCalls reports the first cycle in the module's call graph as a
// *ParseError at the declaration of the function that closes it. The graph
// includes every defined function; prototypes without a body have no calls.
func (f *Frontend) rejectRecursiveCalls() *frontError {
	const (
		unvisited = iota
		onStack
		done
	)
	state := make([]int, len(f.mod.Functions))

	var visit func(h ir.FunctionHandle) *frontError
	visit = func(h ir.FunctionHandle) *frontError {
		switch state[h] {
		case done:
			return nil
		case onStack:
			span := ir.Span{}
			if int(h) < len(f.functionSpans) {
				span = f.functionSpans[h]
			}
			return errSemantic(span,
				"Recursion is not allowed: the calls of function %q lead back to it",
				f.mod.Functions[h].Name)
		}
		state[h] = onStack
		for _, callee := range callTargets(f.mod.Functions[h].Body) {
			if int(callee) >= len(f.mod.Functions) {
				continue
			}
			if err := visit(callee); err != nil {
				return err
			}
		}
		state[h] = done
		return nil
	}
	for h := range f.mod.Functions {
		if err := visit(ir.FunctionHandle(h)); err != nil {
			return err
		}
	}
	return nil
}

// callTargets returns the functions a statement tree calls, in source order.
func callTargets(stmts []ir.Statement) []ir.FunctionHandle {
	var out []ir.FunctionHandle
	var walk func([]ir.Statement)
	walk = func(block []ir.Statement) {
		for _, stmt := range block {
			switch s := stmt.Kind.(type) {
			case ir.StmtCall:
				out = append(out, s.Function)
			case ir.StmtBlock:
				walk(s.Block)
			case ir.StmtIf:
				walk(s.Accept)
				walk(s.Reject)
			case ir.StmtSwitch:
				for i := range s.Cases {
					walk(s.Cases[i].Body)
				}
			case ir.StmtLoop:
				walk(s.Body)
				walk(s.Continuing)
			}
		}
	}
	walk(stmts)
	return out
}
