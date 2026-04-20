package dxil

import "github.com/gogpu/naga/ir"

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
	markAliveFromBlock(fn.Body, fn, alive)

	// Phase 2: for each alive expression, propagate liveness to sub-expressions.
	// Process in reverse order (higher handles depend on lower handles).
	for i := numExprs - 1; i >= 0; i-- {
		if !alive[i] {
			continue
		}
		propagateExprLiveness(fn, ir.ExpressionHandle(i), alive) //nolint:gosec // i bounded by len(fn.Expressions)
	}

	// Phase 3: compute used component masks from alive access patterns.
	// Walk expressions bottom-up, tracking which components of each argument
	// member are ultimately consumed.
	result := make(map[inputUsageKey]uint8)
	computeUsedComponents(irMod, fn, alive, result)
	return result
}

// markAliveFromBlock marks expressions referenced by statements in a block as alive.
func markAliveFromBlock(block ir.Block, fn *ir.Function, alive []bool) {
	for i := range block {
		markAliveFromStmt(&block[i], fn, alive)
	}
}

// markAliveFromStmt marks expressions referenced by a statement as alive.
//
//nolint:gocyclo,cyclop,funlen // exhaustive statement-kind dispatch
func markAliveFromStmt(stmt *ir.Statement, fn *ir.Function, alive []bool) {
	switch s := stmt.Kind.(type) {
	case ir.StmtEmit:
		// Emit makes a range of expressions alive.
		for h := s.Range.Start; h < s.Range.End; h++ {
			if int(h) < len(alive) {
				alive[h] = true
			}
		}
	case ir.StmtBlock:
		markAliveFromBlock(s.Block, fn, alive)
	case ir.StmtIf:
		markExprAlive(s.Condition, alive)
		markAliveFromBlock(s.Accept, fn, alive)
		markAliveFromBlock(s.Reject, fn, alive)
	case ir.StmtSwitch:
		markExprAlive(s.Selector, alive)
		for ci := range s.Cases {
			markAliveFromBlock(s.Cases[ci].Body, fn, alive)
		}
	case ir.StmtLoop:
		markAliveFromBlock(s.Body, fn, alive)
		markAliveFromBlock(s.Continuing, fn, alive)
		if s.BreakIf != nil {
			markExprAlive(*s.BreakIf, alive)
		}
	case ir.StmtReturn:
		if s.Value != nil {
			markExprAlive(*s.Value, alive)
		}
	case ir.StmtStore:
		markExprAlive(s.Pointer, alive)
		markExprAlive(s.Value, alive)
	case ir.StmtImageStore:
		markExprAlive(s.Image, alive)
		markExprAlive(s.Coordinate, alive)
		markExprAlive(s.Value, alive)
		if s.ArrayIndex != nil {
			markExprAlive(*s.ArrayIndex, alive)
		}
	case ir.StmtAtomic:
		markExprAlive(s.Pointer, alive)
		markExprAlive(s.Value, alive)
		if s.Result != nil {
			markExprAlive(*s.Result, alive)
		}
		if ex, ok := s.Fun.(ir.AtomicExchange); ok && ex.Compare != nil {
			markExprAlive(*ex.Compare, alive)
		}
	case ir.StmtImageAtomic:
		markExprAlive(s.Image, alive)
		markExprAlive(s.Coordinate, alive)
		markExprAlive(s.Value, alive)
		if s.ArrayIndex != nil {
			markExprAlive(*s.ArrayIndex, alive)
		}
		if ex, ok := s.Fun.(ir.AtomicExchange); ok && ex.Compare != nil {
			markExprAlive(*ex.Compare, alive)
		}
	case ir.StmtCall:
		if s.Result != nil {
			markExprAlive(*s.Result, alive)
		}
		for _, arg := range s.Arguments {
			markExprAlive(arg, alive)
		}
	case ir.StmtWorkGroupUniformLoad:
		markExprAlive(s.Pointer, alive)
		markExprAlive(s.Result, alive)
	case ir.StmtBarrier:
		// No expressions.
	case ir.StmtKill:
		// No expressions.
	case ir.StmtBreak:
		// No expressions.
	case ir.StmtContinue:
		// No expressions.
	case ir.StmtSubgroupBallot:
		markExprAlive(s.Result, alive)
		if s.Predicate != nil {
			markExprAlive(*s.Predicate, alive)
		}
	case ir.StmtSubgroupCollectiveOperation:
		markExprAlive(s.Argument, alive)
		markExprAlive(s.Result, alive)
	case ir.StmtSubgroupGather:
		markExprAlive(s.Argument, alive)
		markExprAlive(s.Result, alive)
	case ir.StmtRayQuery:
		markExprAlive(s.Query, alive)
		markRayQueryAlive(s, alive)
	}
}

// markRayQueryAlive marks expressions from ray query statement sub-fields.
func markRayQueryAlive(s ir.StmtRayQuery, alive []bool) {
	switch f := s.Fun.(type) {
	case ir.RayQueryInitialize:
		markExprAlive(f.AccelerationStructure, alive)
		markExprAlive(f.Descriptor, alive)
	case ir.RayQueryProceed:
		markExprAlive(f.Result, alive)
	case ir.RayQueryGenerateIntersection:
		markExprAlive(f.HitT, alive)
	case ir.RayQueryTerminate:
		// No expressions.
	case ir.RayQueryConfirmIntersection:
		// No expressions.
	}
}

// markExprAlive marks a single expression as alive.
func markExprAlive(h ir.ExpressionHandle, alive []bool) {
	if int(h) < len(alive) {
		alive[h] = true
	}
}

// propagateExprLiveness marks sub-expressions of an alive expression as alive.
//
//nolint:gocyclo,cyclop,funlen // exhaustive expression-kind dispatch
func propagateExprLiveness(fn *ir.Function, h ir.ExpressionHandle, alive []bool) {
	if int(h) >= len(fn.Expressions) {
		return
	}
	expr := fn.Expressions[h].Kind
	switch e := expr.(type) {
	case ir.ExprAccessIndex:
		markExprAlive(e.Base, alive)
	case ir.ExprAccess:
		markExprAlive(e.Base, alive)
		markExprAlive(e.Index, alive)
	case ir.ExprSwizzle:
		markExprAlive(e.Vector, alive)
	case ir.ExprCompose:
		for _, comp := range e.Components {
			markExprAlive(comp, alive)
		}
	case ir.ExprSplat:
		markExprAlive(e.Value, alive)
	case ir.ExprLoad:
		markExprAlive(e.Pointer, alive)
	case ir.ExprUnary:
		markExprAlive(e.Expr, alive)
	case ir.ExprBinary:
		markExprAlive(e.Left, alive)
		markExprAlive(e.Right, alive)
	case ir.ExprSelect:
		markExprAlive(e.Condition, alive)
		markExprAlive(e.Accept, alive)
		markExprAlive(e.Reject, alive)
	case ir.ExprRelational:
		markExprAlive(e.Argument, alive)
	case ir.ExprMath:
		markExprAlive(e.Arg, alive)
		if e.Arg1 != nil {
			markExprAlive(*e.Arg1, alive)
		}
		if e.Arg2 != nil {
			markExprAlive(*e.Arg2, alive)
		}
		if e.Arg3 != nil {
			markExprAlive(*e.Arg3, alive)
		}
	case ir.ExprAs:
		markExprAlive(e.Expr, alive)
	case ir.ExprImageSample:
		markExprAlive(e.Image, alive)
		markExprAlive(e.Sampler, alive)
		markExprAlive(e.Coordinate, alive)
		if e.ArrayIndex != nil {
			markExprAlive(*e.ArrayIndex, alive)
		}
		if e.Offset != nil {
			markExprAlive(*e.Offset, alive)
		}
		if e.DepthRef != nil {
			markExprAlive(*e.DepthRef, alive)
		}
		if e.Level != nil {
			markSampleLevelAlive(e.Level, alive)
		}
	case ir.ExprImageLoad:
		markExprAlive(e.Image, alive)
		markExprAlive(e.Coordinate, alive)
		if e.ArrayIndex != nil {
			markExprAlive(*e.ArrayIndex, alive)
		}
		if e.Sample != nil {
			markExprAlive(*e.Sample, alive)
		}
		if e.Level != nil {
			markExprAlive(*e.Level, alive)
		}
	case ir.ExprImageQuery:
		markExprAlive(e.Image, alive)
		if qs, ok := e.Query.(ir.ImageQuerySize); ok && qs.Level != nil {
			markExprAlive(*qs.Level, alive)
		}
	case ir.ExprArrayLength:
		markExprAlive(e.Array, alive)
	case ir.ExprDerivative:
		markExprAlive(e.Expr, alive)
	case ir.ExprFunctionArgument:
		// Leaf — no sub-expressions.
	case ir.Literal:
		// Leaf.
	case ir.ExprZeroValue:
		// Leaf.
	case ir.ExprGlobalVariable:
		// Leaf.
	case ir.ExprLocalVariable:
		// Leaf.
	case ir.ExprCallResult:
		// Leaf �� result of a StmtCall.
	case ir.ExprAtomicResult:
		// Leaf — result of a StmtAtomic/StmtImageAtomic.
	case ir.ExprWorkGroupUniformLoadResult:
		// Leaf.
	case ir.ExprSubgroupBallotResult:
		// Leaf.
	case ir.ExprSubgroupOperationResult:
		// Leaf.
	case ir.ExprRayQueryProceedResult:
		// Leaf.
	case ir.ExprRayQueryGetIntersection:
		markExprAlive(e.Query, alive)
	}
}

// markSampleLevelAlive marks expressions from a SampleLevel.
func markSampleLevelAlive(level ir.SampleLevel, alive []bool) {
	switch l := level.(type) {
	case ir.SampleLevelExact:
		markExprAlive(l.Level, alive)
	case ir.SampleLevelBias:
		markExprAlive(l.Bias, alive)
	case ir.SampleLevelGradient:
		markExprAlive(l.X, alive)
		markExprAlive(l.Y, alive)
	}
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
