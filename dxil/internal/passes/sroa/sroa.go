// Package sroa implements Scalar Replacement of Aggregates for DXIL emit.
//
// This pass decomposes struct-typed and array-typed local variables into
// individual per-member/per-element local variables when all access paths
// use constant indices (ExprAccessIndex chains). After decomposition, the
// existing mem2reg pass can promote the resulting scalar/vector locals to
// SSA form, eliminating allocas entirely.
//
// This mirrors LLVM's ScalarReplAggregates (SROA) pass that DXC applies
// before mem2reg:
//
//	DXC pipeline: HLSL → SROA → mem2reg → DCE → DXIL emit
//	Our pipeline: WGSL → IR → SROA → mem2reg → DCE → DXIL emit
//
// Scope:
//   - Struct-typed locals where every use is either:
//     (a) AccessIndex(LocalVariable, const_idx) for field-level store/load
//     (b) Load(LocalVariable) for full-struct load (e.g., return)
//   - Single-level decomposition only (nested structs within members are
//     not recursively decomposed — each member becomes one new local of
//     its member type, which may still be a vector/matrix but is no longer
//     a struct).
//
// Reference:
//   - DXC: lib/Transforms/Scalar/ScalarReplAggregates.cpp
//   - LLVM: https://llvm.org/docs/Passes.html#sroa-scalar-replacement-of-aggregates
package sroa

import (
	"github.com/gogpu/naga/ir"
)

// Run performs SROA on fn, decomposing eligible struct locals into
// per-member locals. Returns the number of variables decomposed.
//
// The function's LocalVars, Expressions, and Body are mutated in place.
// The original struct local variable slot is kept (so indices remain
// stable) but will have zero references after rewriting, causing the
// emitter's liveness check to skip its alloca.
func Run(mod *ir.Module, fn *ir.Function) int {
	if mod == nil || fn == nil || len(fn.LocalVars) == 0 {
		return 0
	}

	candidates := classify(mod, fn)
	if len(candidates) == 0 {
		return 0
	}

	count := 0
	for varIdx, info := range candidates {
		if !info.eligible {
			continue
		}
		decompose(mod, fn, varIdx, info)
		count++
	}
	return count
}

// allMembersDecomposable checks that all struct members have types that
// the emit pipeline can handle as standalone local variables. Currently
// supports scalar (f32/i32/u32/bool) and vector (vec2/vec3/vec4 of
// f32/i32/u32) types. f16, matrices, nested structs, and arrays are
// excluded because they either have limited DXIL support or require
// additional emit-layer changes.
func allMembersDecomposable(mod *ir.Module, st ir.StructType) bool {
	for _, member := range st.Members {
		if int(member.Type) >= len(mod.Types) {
			return false
		}
		inner := mod.Types[member.Type].Inner
		switch t := inner.(type) {
		case ir.ScalarType:
			// Only 32-bit types are safe.
			if t.Width != 4 {
				return false
			}
		case ir.VectorType:
			// Only 32-bit vector element types are safe.
			if t.Scalar.Width != 4 {
				return false
			}
		default:
			// Matrices, structs, arrays, atomics — not supported.
			return false
		}
	}
	return true
}

// candidateInfo holds analysis results for a single struct local.
type candidateInfo struct {
	eligible bool
	st       ir.StructType
	// lvHandle is the ExpressionHandle of the ExprLocalVariable for this var.
	lvHandle ir.ExpressionHandle
	// accessHandles maps field index to the ExpressionHandle of
	// ExprAccessIndex(lvHandle, fieldIdx). There may be multiple handles
	// per field (one per reference site).
	accessHandles map[uint32][]ir.ExpressionHandle
	// fullLoadHandles lists ExprLoad expressions that load the full struct.
	fullLoadHandles []ir.ExpressionHandle
}

// classify identifies struct locals eligible for SROA.
func classify(mod *ir.Module, fn *ir.Function) map[uint32]*candidateInfo {
	// Step 1: find ExprLocalVariable handles for struct-typed locals.
	candidates := make(map[uint32]*candidateInfo)
	lvHandleMap := make(map[ir.ExpressionHandle]uint32) // expr handle -> var index

	for h, expr := range fn.Expressions {
		lv, ok := expr.Kind.(ir.ExprLocalVariable)
		if !ok {
			continue
		}
		if int(lv.Variable) >= len(fn.LocalVars) {
			continue
		}
		local := &fn.LocalVars[lv.Variable]
		if int(local.Type) >= len(mod.Types) {
			continue
		}
		st, isSt := mod.Types[local.Type].Inner.(ir.StructType)
		if !isSt {
			continue
		}
		// Skip structs with unsupported member types (f16, matrices,
		// nested structs, arrays) that would produce invalid DXIL after
		// decomposition. Only decompose structs whose members are
		// scalar or vector of f32/i32/u32.
		if !allMembersDecomposable(mod, st) {
			continue
		}
		hh := ir.ExpressionHandle(h)
		lvHandleMap[hh] = lv.Variable
		if _, exists := candidates[lv.Variable]; !exists {
			candidates[lv.Variable] = &candidateInfo{
				eligible:      true,
				st:            st,
				lvHandle:      hh,
				accessHandles: make(map[uint32][]ir.ExpressionHandle),
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Step 2: classify all references to these locals.
	classifyExprs(fn, lvHandleMap, candidates)

	// Step 3: check statements for disqualifying uses.
	// Any use of the struct local variable handle that isn't covered above
	// (e.g., passed as call argument, used in atomic) disqualifies it.
	classifyStmts(fn, fn.Body, lvHandleMap, candidates)
	// Also check for stores inside nested blocks (loops/ifs).
	disqualifyNestedStores(fn, fn.Body, lvHandleMap, candidates)

	// Step 4: remove candidates with no actual uses (would be dead anyway).
	// Also disqualify candidates whose uses span multiple control-flow blocks
	// to avoid creating loads that Phase B's phi walker cannot handle.
	for varIdx, info := range candidates {
		if !info.eligible {
			continue
		}
		if len(info.accessHandles) == 0 && len(info.fullLoadHandles) == 0 {
			info.eligible = false
			continue
		}
		// Only decompose if ALL stores and loads are in the top-level body
		// (not nested inside loops/ifs/switches). This ensures the decomposed
		// loads don't create phi-insertion problems for Phase B.
		if !allUsesInTopLevel(fn, info) {
			info.eligible = false
			continue
		}
		_ = varIdx
	}

	return candidates
}

// classifyExprs classifies expression references to struct local handles.
func classifyExprs(fn *ir.Function, lvHandleMap map[ir.ExpressionHandle]uint32, candidates map[uint32]*candidateInfo) {
	for h, expr := range fn.Expressions {
		hh := ir.ExpressionHandle(h)
		switch ek := expr.Kind.(type) {
		case ir.ExprAccessIndex:
			varIdx, ok := lvHandleMap[ek.Base]
			if !ok {
				continue
			}
			info := candidates[varIdx]
			if !info.eligible {
				continue
			}
			if int(ek.Index) >= len(info.st.Members) {
				info.eligible = false
				continue
			}
			info.accessHandles[ek.Index] = append(info.accessHandles[ek.Index], hh)
		case ir.ExprLoad:
			varIdx, ok := lvHandleMap[ek.Pointer]
			if !ok {
				continue
			}
			info := candidates[varIdx]
			if !info.eligible {
				continue
			}
			info.fullLoadHandles = append(info.fullLoadHandles, hh)
		case ir.ExprAccess:
			if varIdx, ok := lvHandleMap[ek.Base]; ok {
				candidates[varIdx].eligible = false
			}
		}
	}
}

// classifyStmts walks the statement tree and disqualifies struct locals
// used in ways that prevent SROA (e.g., pointer escapes to function calls,
// atomics, image stores).
//
//nolint:gocognit // exhaustive statement-kind dispatch mirrors mem2reg's classifyStatements
func classifyStmts(fn *ir.Function, block ir.Block, lvHandleMap map[ir.ExpressionHandle]uint32, candidates map[uint32]*candidateInfo) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtStore:
			// Store TO an AccessIndex of a struct local is fine (handled above).
			// Store TO the struct local directly means full-struct store —
			// this is fine only if the value is a Compose.
			if varIdx, ok := lvHandleMap[sk.Pointer]; ok {
				info := candidates[varIdx]
				if !info.eligible {
					continue
				}
				// Full struct store: check if value is Compose.
				if int(sk.Value) < len(fn.Expressions) {
					if _, isCompose := fn.Expressions[sk.Value].Kind.(ir.ExprCompose); isCompose {
						// Full compose store is OK — we'll decompose it.
						continue
					}
				}
				// Any other full-struct store disqualifies.
				info.eligible = false
			}
		case ir.StmtCall:
			for _, a := range sk.Arguments {
				if varIdx, ok := lvHandleMap[a]; ok {
					candidates[varIdx].eligible = false
				}
			}
		case ir.StmtAtomic:
			if varIdx, ok := lvHandleMap[sk.Pointer]; ok {
				candidates[varIdx].eligible = false
			}
		case ir.StmtIf:
			classifyStmts(fn, sk.Accept, lvHandleMap, candidates)
			classifyStmts(fn, sk.Reject, lvHandleMap, candidates)
		case ir.StmtLoop:
			classifyStmts(fn, sk.Body, lvHandleMap, candidates)
			classifyStmts(fn, sk.Continuing, lvHandleMap, candidates)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				classifyStmts(fn, sk.Cases[ci].Body, lvHandleMap, candidates)
			}
		case ir.StmtBlock:
			classifyStmts(fn, sk.Block, lvHandleMap, candidates)
		}
	}
}

// disqualifyNestedStores marks struct locals as ineligible if any of their
// stores are inside nested control-flow blocks (loops, ifs, switches).
// Only top-level stores/loads are safe to decompose without risking
// phi-insertion issues.
func disqualifyNestedStores(fn *ir.Function, block ir.Block, lvHandleMap map[ir.ExpressionHandle]uint32, candidates map[uint32]*candidateInfo) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtIf:
			markNestedUses(fn, sk.Accept, lvHandleMap, candidates)
			markNestedUses(fn, sk.Reject, lvHandleMap, candidates)
		case ir.StmtLoop:
			markNestedUses(fn, sk.Body, lvHandleMap, candidates)
			markNestedUses(fn, sk.Continuing, lvHandleMap, candidates)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				markNestedUses(fn, sk.Cases[ci].Body, lvHandleMap, candidates)
			}
		case ir.StmtBlock:
			markNestedUses(fn, sk.Block, lvHandleMap, candidates)
		}
	}
}

// markNestedUses disqualifies any struct local whose pointer handle
// appears in stores within this nested block.
func markNestedUses(fn *ir.Function, block ir.Block, lvHandleMap map[ir.ExpressionHandle]uint32, candidates map[uint32]*candidateInfo) {
	for i := range block {
		switch sk := block[i].Kind.(type) {
		case ir.StmtStore:
			// Check if the store targets a struct local (directly or via AccessIndex).
			if varIdx, ok := resolveStructLocal(fn, sk.Pointer, lvHandleMap); ok {
				if info, exists := candidates[varIdx]; exists {
					info.eligible = false
				}
			}
		case ir.StmtIf:
			markNestedUses(fn, sk.Accept, lvHandleMap, candidates)
			markNestedUses(fn, sk.Reject, lvHandleMap, candidates)
		case ir.StmtLoop:
			markNestedUses(fn, sk.Body, lvHandleMap, candidates)
			markNestedUses(fn, sk.Continuing, lvHandleMap, candidates)
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				markNestedUses(fn, sk.Cases[ci].Body, lvHandleMap, candidates)
			}
		case ir.StmtBlock:
			markNestedUses(fn, sk.Block, lvHandleMap, candidates)
		}
	}
}

// resolveStructLocal follows AccessIndex chains to find the root local
// variable handle, then checks if it's one of our struct-local candidates.
func resolveStructLocal(fn *ir.Function, h ir.ExpressionHandle, lvHandleMap map[ir.ExpressionHandle]uint32) (uint32, bool) {
	const maxDepth = 16
	for depth := 0; depth < maxDepth; depth++ {
		if int(h) >= len(fn.Expressions) {
			return 0, false
		}
		if varIdx, ok := lvHandleMap[h]; ok {
			return varIdx, true
		}
		switch k := fn.Expressions[h].Kind.(type) {
		case ir.ExprAccessIndex:
			h = k.Base
		case ir.ExprAccess:
			h = k.Base
		default:
			return 0, false
		}
	}
	return 0, false
}

// allUsesInTopLevel checks that all full-struct loads for a struct local
// are in the top-level body (not nested in control flow). This ensures
// SROA-created per-field loads don't interfere with Phase B phi insertion.
// Stores to fields via AccessIndex are already checked by
// disqualifyNestedStores; this function only needs to verify the load site.
func allUsesInTopLevel(fn *ir.Function, info *candidateInfo) bool {
	for _, loadH := range info.fullLoadHandles {
		if !emitRangeInTopLevel(fn.Body, loadH) {
			return false
		}
	}
	return true
}

// emitRangeInTopLevel checks if the expression handle is covered by a
// StmtEmit at the top level of the block.
func emitRangeInTopLevel(block ir.Block, h ir.ExpressionHandle) bool {
	for _, st := range block {
		if emit, ok := st.Kind.(ir.StmtEmit); ok {
			if emit.Range.Start <= h && h < emit.Range.End {
				return true
			}
		}
	}
	return false
}

// decompose replaces a struct local with per-member locals and rewrites
// all references.
func decompose(_ *ir.Module, fn *ir.Function, varIdx uint32, info *candidateInfo) {
	// Step 1: Create new local variables — one per struct member.
	// Record the mapping from field index to new variable index.
	baseVarIdx := uint32(len(fn.LocalVars)) //nolint:gosec // local var count fits uint32
	fieldVarMap := make(map[uint32]uint32, len(info.st.Members))

	for i, member := range info.st.Members {
		newIdx := baseVarIdx + uint32(i)
		fieldVarMap[uint32(i)] = newIdx
		// Inherit Init from the original if it's a Compose.
		var initExpr *ir.ExpressionHandle
		origLocal := &fn.LocalVars[varIdx]
		if origLocal.Init != nil {
			if compose, ok := fn.Expressions[*origLocal.Init].Kind.(ir.ExprCompose); ok {
				if i < len(compose.Components) {
					comp := compose.Components[i]
					initExpr = &comp
				}
			}
		}
		fn.LocalVars = append(fn.LocalVars, ir.LocalVariable{
			Name: member.Name,
			Type: member.Type,
			Init: initExpr,
		})
	}

	// Step 2: Create ExprLocalVariable expressions for each new local var.
	// We append new expressions and record their handles.
	fieldExprHandles := make(map[uint32]ir.ExpressionHandle, len(info.st.Members))
	for i := range info.st.Members {
		h := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec // expression count fits
		fn.Expressions = append(fn.Expressions, ir.Expression{
			Kind: ir.ExprLocalVariable{Variable: fieldVarMap[uint32(i)]},
		})
		// Keep ExpressionTypes parallel.
		if len(fn.ExpressionTypes) == int(h) {
			th := fn.LocalVars[fieldVarMap[uint32(i)]].Type
			fn.ExpressionTypes = append(fn.ExpressionTypes, ir.TypeResolution{
				Handle: &th,
			})
		}
		fieldExprHandles[uint32(i)] = h
	}

	// Step 3: Rewrite ExprAccessIndex(lvHandle, fieldIdx) to
	// ExprLocalVariable directly. This is required for mem2reg to
	// recognize the handle as a local-var pointer in buildLocalPtrMap
	// (ExprAlias would not be recognized).
	for fieldIdx, handles := range info.accessHandles {
		newVarIdx := fieldVarMap[fieldIdx]
		for _, h := range handles {
			fn.Expressions[h].Kind = ir.ExprLocalVariable{Variable: newVarIdx}
		}
		// No need for the separate ExprLocalVariable expression we
		// created in Step 2 for this field — the rewritten AccessIndex
		// handles now serve as the pointer expressions. But keep the
		// Step 2 expressions for the full-load rewrite path (Step 4)
		// which needs distinct handles per field.
		_ = fieldExprHandles[fieldIdx]
	}

	// Step 4: Rewrite full struct loads to Compose of per-field loads.
	// Also create StmtEmit ranges for the new load expressions so that
	// mem2reg's classifyExpressions sees them as live.
	for _, loadH := range info.fullLoadHandles {
		// Record the start of new expressions for the emit range.
		newExprStart := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec

		// The full load becomes a Compose of loads from each new field local.
		components := make([]ir.ExpressionHandle, len(info.st.Members))
		for i := range info.st.Members {
			loadExprH := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec
			fn.Expressions = append(fn.Expressions, ir.Expression{
				Kind: ir.ExprLoad{Pointer: fieldExprHandles[uint32(i)]},
			})
			if len(fn.ExpressionTypes) == int(loadExprH) {
				th := fn.LocalVars[fieldVarMap[uint32(i)]].Type
				fn.ExpressionTypes = append(fn.ExpressionTypes, ir.TypeResolution{
					Handle: &th,
				})
			}
			components[i] = loadExprH
		}
		newExprEnd := ir.ExpressionHandle(len(fn.Expressions)) //nolint:gosec

		// Rewrite the original load to a Compose.
		fn.Expressions[loadH].Kind = ir.ExprCompose{Components: components}

		// Insert a StmtEmit covering the new load expressions into the
		// body. Find the StmtEmit that contains loadH and insert a new
		// emit range for the per-field loads right before it.
		bodySlice := fn.Body
		insertEmitForRange(&bodySlice, loadH, ir.Range{Start: newExprStart, End: newExprEnd})
		fn.Body = ir.Block(bodySlice)
	}
}

// insertEmitForRange finds the StmtEmit that contains targetH and
// inserts a new StmtEmit for newRange immediately before it. This
// ensures that newly created expressions (like per-field loads from SROA)
// are visible to the mem2reg pass's emitted-expression analysis.
func insertEmitForRange(block *[]ir.Statement, targetH ir.ExpressionHandle, newRange ir.Range) {
	if insertEmitInSlice(block, targetH, newRange) {
		return
	}
	// If not found at top level, search nested blocks.
	for i := range *block {
		switch sk := (*block)[i].Kind.(type) {
		case ir.StmtIf:
			a := []ir.Statement(sk.Accept)
			r := []ir.Statement(sk.Reject)
			if insertEmitInSlice(&a, targetH, newRange) {
				(*block)[i].Kind = ir.StmtIf{Condition: sk.Condition, Accept: ir.Block(a), Reject: ir.Block(r)}
				return
			}
			if insertEmitInSlice(&r, targetH, newRange) {
				(*block)[i].Kind = ir.StmtIf{Condition: sk.Condition, Accept: ir.Block(a), Reject: ir.Block(r)}
				return
			}
		case ir.StmtLoop:
			b := []ir.Statement(sk.Body)
			c := []ir.Statement(sk.Continuing)
			if insertEmitInSlice(&b, targetH, newRange) {
				(*block)[i].Kind = ir.StmtLoop{Body: ir.Block(b), Continuing: ir.Block(c), BreakIf: sk.BreakIf}
				return
			}
			if insertEmitInSlice(&c, targetH, newRange) {
				(*block)[i].Kind = ir.StmtLoop{Body: ir.Block(b), Continuing: ir.Block(c), BreakIf: sk.BreakIf}
				return
			}
		case ir.StmtSwitch:
			for ci := range sk.Cases {
				cb := []ir.Statement(sk.Cases[ci].Body)
				if insertEmitInSlice(&cb, targetH, newRange) {
					sk.Cases[ci].Body = ir.Block(cb)
					(*block)[i].Kind = sk
					return
				}
			}
		case ir.StmtBlock:
			b := []ir.Statement(sk.Block)
			if insertEmitInSlice(&b, targetH, newRange) {
				(*block)[i].Kind = ir.StmtBlock{Block: ir.Block(b)}
				return
			}
		}
	}
}

// insertEmitInSlice searches the statement slice for a StmtEmit whose
// range contains targetH and inserts a new StmtEmit for newRange
// immediately before it. Returns true if found/inserted.
func insertEmitInSlice(block *[]ir.Statement, targetH ir.ExpressionHandle, newRange ir.Range) bool {
	for i := range *block {
		if emit, ok := (*block)[i].Kind.(ir.StmtEmit); ok {
			if emit.Range.Start <= targetH && targetH < emit.Range.End {
				// Insert new emit before this one.
				newEmit := ir.Statement{Kind: ir.StmtEmit{Range: newRange}}
				*block = append(*block, ir.Statement{})
				copy((*block)[i+1:], (*block)[i:])
				(*block)[i] = newEmit
				return true
			}
		}
	}
	return false
}
