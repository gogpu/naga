package irutil

import "github.com/gogpu/naga/ir"

// DeduplicateEmits removes duplicate and redundant Emit statements from all functions.
// An Emit is redundant if its range is already covered by a previous Emit in the same block.
// This handles cases where the emitter flush in function calls generates duplicate ranges.
func DeduplicateEmits(module *ir.Module) {
	for i := range module.Functions {
		module.Functions[i].Body = deduplicateBlockEmits(module.Functions[i].Body, module.Functions[i].Expressions)
	}
	for i := range module.EntryPoints {
		module.EntryPoints[i].Function.Body = deduplicateBlockEmits(module.EntryPoints[i].Function.Body, module.EntryPoints[i].Function.Expressions)
	}
}

// deduplicateBlockEmits removes duplicate, empty, covered, and pre-emit-only Emit statements.
func deduplicateBlockEmits(stmts []ir.Statement, exprs []ir.Expression) []ir.Statement {
	if len(stmts) == 0 {
		return stmts
	}
	// First pass: remove empty, covered, and pre-emit-only Emits.
	emitted := make(map[ir.ExpressionHandle]bool)
	filtered := make([]ir.Statement, 0, len(stmts))
	for _, s := range stmts {
		if emit, ok := s.Kind.(ir.StmtEmit); ok {
			if redundantEmit(emit.Range, emitted, exprs) {
				continue
			}
			for h := emit.Range.Start; h < emit.Range.End; h++ {
				emitted[h] = true
			}
		}
		filtered = append(filtered, s)
	}

	// Second pass: recurse into nested blocks
	result := make([]ir.Statement, 0, len(filtered))
	for _, s := range filtered {
		result = append(result, deduplicateNestedEmits(s, exprs))
	}
	return result
}

// redundantEmit reports an Emit of r that is empty, covers only expressions
// an earlier Emit of the block emitted, or covers only expressions that need
// no Emit (Rust naga's needs_pre_emit(): Literal, Constant, ZeroValue,
// GlobalVariable, FunctionArgument, LocalVariable, Override).
func redundantEmit(r ir.Range, emitted map[ir.ExpressionHandle]bool, exprs []ir.Expression) bool {
	if r.Start == r.End {
		return true
	}
	allCovered := true
	for h := r.Start; h < r.End; h++ {
		if !emitted[h] {
			allCovered = false
			break
		}
	}
	if allCovered {
		return true
	}
	for h := r.Start; h < r.End; h++ {
		if int(h) < len(exprs) && !exprs[h].NeedsPreEmit() {
			return false
		}
	}
	return true
}

// deduplicateNestedEmits returns s with the Emits of the blocks it holds
// deduplicated.
func deduplicateNestedEmits(s ir.Statement, exprs []ir.Expression) ir.Statement {
	switch k := s.Kind.(type) {
	case ir.StmtBlock:
		k.Block = ir.Block(deduplicateBlockEmits([]ir.Statement(k.Block), exprs))
		s.Kind = k
	case ir.StmtIf:
		k.Accept = ir.Block(deduplicateBlockEmits([]ir.Statement(k.Accept), exprs))
		k.Reject = ir.Block(deduplicateBlockEmits([]ir.Statement(k.Reject), exprs))
		s.Kind = k
	case ir.StmtSwitch:
		for ci := range k.Cases {
			k.Cases[ci].Body = ir.Block(deduplicateBlockEmits([]ir.Statement(k.Cases[ci].Body), exprs))
		}
		s.Kind = k
	case ir.StmtLoop:
		k.Body = ir.Block(deduplicateBlockEmits([]ir.Statement(k.Body), exprs))
		k.Continuing = ir.Block(deduplicateBlockEmits([]ir.Statement(k.Continuing), exprs))
		s.Kind = k
	}
	return s
}
