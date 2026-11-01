package dxil

import (
	"github.com/gogpu/naga/internal/irutil"
	"github.com/gogpu/naga/ir"
)

// inputUsageKey identifies a signature element by its flat input index.
// For non-struct arguments, it's the argument's position in the flat binding list.
// For struct members, each member gets its own index.
type inputUsageKey struct {
	// argIdx is the function argument index (0-based).
	argIdx int
	// memberIdx is the struct member index within that argument (-1 for non-struct).
	memberIdx int
}

// computeInputUsedMasks analyzes the entry point function body to determine
// which components of each input argument are actually read. Returns a map
// from (argIdx, memberIdx) to a bitmask of used components (bit 0 = x, etc.).
//
// This mirrors DXC's MarkUsedSignatureElements pass (DxilPreparePasses.cpp:290)
// which iterates all loadInput instructions and ORs the column bit into the
// element's usage mask. Since DXC runs DCE before this pass, unused inputs
// have no loadInput instructions and get UsageMask = 0.
//
// We achieve the same by analyzing the IR: if an argument (or struct member)
// is never referenced by any expression reachable from the function body,
// its UsageMask = 0.
func computeInputUsedMasks(irMod *ir.Module, fn *ir.Function) map[inputUsageKey]uint8 {
	if fn == nil || len(fn.Expressions) == 0 {
		return nil
	}

	// Phase 1: determine which expressions are "alive" (reachable from body
	// statements). An expression is alive if it appears in an Emit range within
	// a reachable statement, or if it's directly referenced by a statement.
	numExprs := len(fn.Expressions)
	alive := make([]bool, numExprs)
	markAlive := func(h ir.ExpressionHandle) {
		if int(h) < len(alive) {
			alive[h] = true
		}
	}
	blockUses(fn.Body, markAlive)

	// Phase 2: for each alive expression, propagate liveness to sub-expressions.
	// Process in reverse order (higher handles depend on lower handles).
	for i := numExprs - 1; i >= 0; i-- {
		if alive[i] {
			irutil.ExprHandles(fn.Expressions[i].Kind, markAlive)
		}
	}

	// Phase 3: compute used component masks from alive access patterns.
	// Walk expressions bottom-up, tracking which components of each argument
	// member are ultimately consumed.
	result := make(map[inputUsageKey]uint8)
	computeUsedComponents(irMod, fn, alive, result)
	return result
}

// computeUsedComponents determines which components of each input argument
// are accessed by alive expressions. It handles:
// - Direct FunctionArgument use (all components of that arg)
// - Struct member access via AccessIndex (that member's components)
// - Vector component access via AccessIndex on vector (single component)
// - Swizzle (selected components)
// - Dynamic Access on vector (all components, conservative)
//
//nolint:gocognit,gocyclo,cyclop,funlen,maintidx // multi-pass analysis with expression-kind dispatch
func computeUsedComponents(irMod *ir.Module, fn *ir.Function, alive []bool, result map[inputUsageKey]uint8) {
	numExprs := len(fn.Expressions)

	// accessInfo describes an expression's relationship to a function argument.
	type accessInfo struct {
		argIdx       int
		memberIdx    int // -1 for non-struct direct arg access
		isStruct     bool
		isMemberLeaf bool // true if this is a struct member access (not component access)
	}

	// exprAccess maps expression handles to their resolved argument root.
	exprAccess := make([]accessInfo, numExprs)
	hasAccess := make([]bool, numExprs)

	// Pass 1: identify FunctionArgument expressions.
	for i := 0; i < numExprs; i++ {
		if !alive[i] {
			continue
		}
		if e, ok := fn.Expressions[i].Kind.(ir.ExprFunctionArgument); ok {
			argIdx := int(e.Index)
			argType := irMod.Types[fn.Arguments[argIdx].Type]
			_, isStruct := argType.Inner.(ir.StructType)
			exprAccess[i] = accessInfo{argIdx: argIdx, memberIdx: -1, isStruct: isStruct}
			hasAccess[i] = true
		}
	}

	// Pass 2: resolve AccessIndex/Access/Swizzle/Load chains.
	// Each access expression inherits its base's arg info and refines component access.
	for i := 0; i < numExprs; i++ {
		if !alive[i] {
			continue
		}
		switch e := fn.Expressions[i].Kind.(type) {
		case ir.ExprAccessIndex:
			baseH := int(e.Base)
			if baseH >= numExprs || !hasAccess[baseH] {
				continue
			}
			baseInfo := exprAccess[baseH]
			if baseInfo.isStruct && baseInfo.memberIdx == -1 {
				// First-level AccessIndex on a struct arg -> member access.
				exprAccess[i] = accessInfo{argIdx: baseInfo.argIdx, memberIdx: int(e.Index), isMemberLeaf: true}
				hasAccess[i] = true
			} else {
				// Component access on a vector (direct arg or struct member).
				exprAccess[i] = accessInfo{argIdx: baseInfo.argIdx, memberIdx: baseInfo.memberIdx}
				hasAccess[i] = true
			}

		case ir.ExprAccess:
			baseH := int(e.Base)
			if baseH >= numExprs || !hasAccess[baseH] {
				continue
			}
			exprAccess[i] = exprAccess[baseH]
			hasAccess[i] = true

		case ir.ExprSwizzle:
			vecH := int(e.Vector)
			if vecH >= numExprs || !hasAccess[vecH] {
				continue
			}
			exprAccess[i] = exprAccess[vecH]
			hasAccess[i] = true

		case ir.ExprLoad:
			ptrH := int(e.Pointer)
			if ptrH >= numExprs || !hasAccess[ptrH] {
				continue
			}
			exprAccess[i] = exprAccess[ptrH]
			hasAccess[i] = true
		}
	}

	// Pass 3: for each alive expression with access info, determine usage.
	// An expression "consumes" its argument when it's used by something that
	// isn't further refining the access chain. We check: is this expression
	// itself an access chain node (AccessIndex/Access/Swizzle/Load on arg)?
	// If yes, compute component bits from the access. If a FunctionArgument
	// or struct-member access is used directly (not further refined), all
	// components are consumed.
	//
	// Strategy: find the "leaf" uses of each arg access chain. A leaf is an
	// alive access-info expression that is NOT the base of another alive
	// access-info expression.
	isBase := make([]bool, numExprs)
	for i := 0; i < numExprs; i++ {
		if !alive[i] || !hasAccess[i] {
			continue
		}
		switch e := fn.Expressions[i].Kind.(type) {
		case ir.ExprAccessIndex:
			if int(e.Base) < numExprs {
				isBase[e.Base] = true
			}
		case ir.ExprAccess:
			if int(e.Base) < numExprs {
				isBase[e.Base] = true
			}
		case ir.ExprSwizzle:
			if int(e.Vector) < numExprs {
				isBase[e.Vector] = true
			}
		case ir.ExprLoad:
			if int(e.Pointer) < numExprs {
				isBase[e.Pointer] = true
			}
		}
	}

	// Now compute masks from leaf expressions.
	for i := 0; i < numExprs; i++ {
		if !alive[i] || !hasAccess[i] {
			continue
		}
		info := exprAccess[i]

		// Skip struct roots that haven't resolved to a member.
		if info.isStruct && info.memberIdx == -1 {
			// If this is a leaf (not a base for further access), it means
			// the struct is used as a whole — mark all members as fully used.
			if !isBase[i] {
				argType := irMod.Types[fn.Arguments[info.argIdx].Type]
				if st, ok := argType.Inner.(ir.StructType); ok {
					for mi := range st.Members {
						key := inputUsageKey{argIdx: info.argIdx, memberIdx: mi}
						result[key] = 0xFF
					}
				}
			}
			continue
		}

		// For non-leaf expressions (used as base of another access), skip —
		// the leaf child will provide more precise component info.
		if isBase[i] {
			continue
		}

		key := inputUsageKey{argIdx: info.argIdx, memberIdx: info.memberIdx}

		// Determine component mask from the expression type.
		switch e := fn.Expressions[i].Kind.(type) {
		case ir.ExprAccessIndex:
			if info.isMemberLeaf {
				// Struct member used directly → all its components.
				result[key] = 0xFF
			} else {
				// AccessIndex on a vector → single component.
				compBit := uint8(1) << e.Index
				result[key] |= compBit
			}

		case ir.ExprSwizzle:
			// Swizzle → specific components.
			var mask uint8
			for c := ir.VectorSize(0); c < e.Size; c++ {
				mask |= 1 << e.Pattern[c]
			}
			result[key] |= mask

		case ir.ExprAccess:
			// Dynamic access → conservative: all components.
			result[key] = 0xFF

		default:
			// FunctionArgument leaf (non-struct) or struct member leaf
			// used directly → all components.
			result[key] = 0xFF
		}
	}
}
