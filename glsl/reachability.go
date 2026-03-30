// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package glsl

import (
	"github.com/gogpu/naga/ir"
)

// reachableSet tracks which IR entities are reachable from a specific entry point.
// This enables dead code elimination: only reachable types, constants, globals,
// and functions are emitted in the GLSL output, dramatically reducing output size
// for modules with many entry points sharing a large IR.
type reachableSet struct {
	functions map[ir.FunctionHandle]struct{}
	globals   map[ir.GlobalVariableHandle]struct{}
	constants map[ir.ConstantHandle]struct{}
	types     map[ir.TypeHandle]struct{}
}

// newReachableSet creates an empty reachable set.
func newReachableSet() *reachableSet {
	return &reachableSet{
		functions: make(map[ir.FunctionHandle]struct{}),
		globals:   make(map[ir.GlobalVariableHandle]struct{}),
		constants: make(map[ir.ConstantHandle]struct{}),
		types:     make(map[ir.TypeHandle]struct{}),
	}
}

// hasFunction returns true if the function is reachable.
func (r *reachableSet) hasFunction(h ir.FunctionHandle) bool {
	_, ok := r.functions[h]
	return ok
}

// hasGlobal returns true if the global variable is reachable.
func (r *reachableSet) hasGlobal(h ir.GlobalVariableHandle) bool {
	_, ok := r.globals[h]
	return ok
}

// hasConstant returns true if the constant is reachable.
func (r *reachableSet) hasConstant(h ir.ConstantHandle) bool {
	_, ok := r.constants[h]
	return ok
}

// hasType returns true if the type is reachable.
func (r *reachableSet) hasType(h ir.TypeHandle) bool {
	_, ok := r.types[h]
	return ok
}

// collectReachable builds a reachable set for the given entry point.
// Function inclusion uses dominates_global_use: a function is included if
// the entry point's set of used globals is a superset of the function's used globals.
// This matches Rust naga's FunctionInfo::dominates_global_use algorithm.
// Functions that use NO globals are included in ALL entry points.
func collectReachable(module *ir.Module, ep *ir.EntryPoint) *reachableSet {
	rs := newReachableSet()
	c := &reachabilityCollector{
		module:          module,
		rs:              rs,
		walkedFunctions: make(map[ir.FunctionHandle]struct{}),
	}

	// Walk the entry point function for globals, constants, types
	fn := &ep.Function
	c.walkFunction(fn)

	// Build global uses for entry point (transitively through called functions)
	epGlobals := collectGlobalUses(module, fn)

	// Include functions using dominates_global_use + stage compatibility:
	// 1. epGlobals ⊇ funcGlobals (global use domination)
	// 2. function's required stage is compatible with EP stage
	epStage := ep.Stage
	for i := range module.Functions {
		funcHandle := ir.FunctionHandle(i)
		fn := &module.Functions[i]
		funcGlobals := collectGlobalUses(module, fn)

		if !dominatesGlobalUse(epGlobals, funcGlobals) {
			continue
		}
		if !stageCompatible(module, fn, epStage) {
			continue
		}

		rs.functions[funcHandle] = struct{}{}
		// Also mark function's types/constants/globals as reachable
		c.walkFunction(fn)
	}

	// Also mark types used by the entry point function's arguments and result,
	// since they drive IO declarations (layout(location=...) in/out).
	for _, arg := range fn.Arguments {
		c.markTypeReachable(arg.Type)
	}
	if fn.Result != nil {
		c.markTypeReachable(fn.Result.Type)
	}

	return rs
}

// collectGlobalUses returns the set of global variables used by a function,
// including transitive uses through called functions.
func collectGlobalUses(module *ir.Module, fn *ir.Function) map[ir.GlobalVariableHandle]struct{} {
	globals := make(map[ir.GlobalVariableHandle]struct{})
	visited := make(map[ir.FunctionHandle]struct{})
	collectGlobalUsesWalk(module, fn, globals, visited)
	return globals
}

func collectGlobalUsesWalk(module *ir.Module, fn *ir.Function, globals map[ir.GlobalVariableHandle]struct{}, visited map[ir.FunctionHandle]struct{}) {
	// Collect globals from expressions
	for _, expr := range fn.Expressions {
		if gv, ok := expr.Kind.(ir.ExprGlobalVariable); ok {
			globals[gv.Variable] = struct{}{}
		}
	}

	// Recurse into called functions
	walkBlockForCalls(fn.Body, func(funcHandle ir.FunctionHandle) {
		if _, already := visited[funcHandle]; already {
			return
		}
		visited[funcHandle] = struct{}{}
		if int(funcHandle) < len(module.Functions) {
			collectGlobalUsesWalk(module, &module.Functions[funcHandle], globals, visited)
		}
	})
}

// walkBlockForCalls walks a block recursively, calling visitor for each StmtCall.
func walkBlockForCalls(block []ir.Statement, visitor func(ir.FunctionHandle)) {
	for _, stmt := range block {
		switch k := stmt.Kind.(type) {
		case ir.StmtCall:
			visitor(k.Function)
		case ir.StmtIf:
			walkBlockForCalls(k.Accept, visitor)
			walkBlockForCalls(k.Reject, visitor)
		case ir.StmtSwitch:
			for _, sc := range k.Cases {
				walkBlockForCalls(sc.Body, visitor)
			}
		case ir.StmtLoop:
			walkBlockForCalls(k.Body, visitor)
			walkBlockForCalls(k.Continuing, visitor)
		case ir.StmtBlock:
			walkBlockForCalls(k.Block, visitor)
		}
	}
}

// stageCompatible checks if a function is compatible with the given shader stage.
// Functions using fragment-only operations (derivatives, discard) are incompatible with compute.
// Functions using compute-only operations (barriers) are incompatible with fragment/vertex.
// Matches Rust naga's available_stages analysis (simplified).
func stageCompatible(module *ir.Module, fn *ir.Function, epStage ir.ShaderStage) bool {
	stages := detectFunctionStages(fn)
	// If function has no stage restrictions, it's compatible with everything
	if stages == 0 {
		return true
	}
	return stages&(1<<epStage) != 0
}

// detectFunctionStages scans a function for stage-specific operations.
// Returns a bitmask of compatible stages (0 = all stages compatible).
func detectFunctionStages(fn *ir.Function) uint8 {
	var stages uint8
	for _, expr := range fn.Expressions {
		switch e := expr.Kind.(type) {
		case ir.ExprDerivative:
			_ = e
			stages |= 1 << ir.StageFragment
		}
	}
	scanBlockForStages(fn.Body, &stages)
	return stages
}

func scanBlockForStages(block []ir.Statement, stages *uint8) {
	for _, stmt := range block {
		switch k := stmt.Kind.(type) {
		case ir.StmtKill: // discard → fragment only
			*stages |= 1 << ir.StageFragment
		case ir.StmtBarrier: // barrier → compute only
			_ = k
			*stages |= 1 << ir.StageCompute
		case ir.StmtIf:
			scanBlockForStages(k.Accept, stages)
			scanBlockForStages(k.Reject, stages)
		case ir.StmtSwitch:
			for _, sc := range k.Cases {
				scanBlockForStages(sc.Body, stages)
			}
		case ir.StmtLoop:
			scanBlockForStages(k.Body, stages)
			scanBlockForStages(k.Continuing, stages)
		case ir.StmtBlock:
			scanBlockForStages(k.Block, stages)
		}
	}
}

// dominatesGlobalUse returns true if epGlobals is a superset of funcGlobals.
// A function with no global uses passes for any entry point.
func dominatesGlobalUse(epGlobals, funcGlobals map[ir.GlobalVariableHandle]struct{}) bool {
	for g := range funcGlobals {
		if _, ok := epGlobals[g]; !ok {
			return false
		}
	}
	return true
}

// reachabilityCollector walks the IR graph to collect reachable entities.
type reachabilityCollector struct {
	module          *ir.Module
	rs              *reachableSet
	walkedFunctions map[ir.FunctionHandle]struct{} // prevents infinite recursion
}

// walkFunction walks a function's expressions, statements, local vars, and signature.
func (c *reachabilityCollector) walkFunction(fn *ir.Function) {
	// Walk all expressions
	for _, expr := range fn.Expressions {
		c.walkExpression(expr.Kind)
	}

	// Walk all statements
	c.walkBlock(fn.Body)

	// Walk local variable types and initializers
	for _, local := range fn.LocalVars {
		c.markTypeReachable(local.Type)
	}

	// Walk argument types
	for _, arg := range fn.Arguments {
		c.markTypeReachable(arg.Type)
	}

	// Walk result type
	if fn.Result != nil {
		c.markTypeReachable(fn.Result.Type)
	}
}

// walkExpression collects references from a single expression.
func (c *reachabilityCollector) walkExpression(kind ir.ExpressionKind) {
	switch k := kind.(type) {
	case ir.ExprGlobalVariable:
		c.markGlobalReachable(k.Variable)
	case ir.ExprConstant:
		c.markConstantReachable(k.Constant)
	case ir.ExprCompose:
		c.markTypeReachable(k.Type)
	case ir.ExprZeroValue:
		c.markTypeReachable(k.Type)
	case ir.ExprCallResult:
		// Don't mark the function as reachable here — function inclusion is
		// determined by dominates_global_use in collectReachable.
		// But walk the called function to collect its types/globals/constants.
		c.walkCalledFunction(k.Function)
	case ir.ExprAs:
		// ExprAs may reference scalar types implicitly; no handle to mark.
	}
	// Other expression types (Literal, Access, Binary, Unary, etc.) don't
	// directly reference top-level IR entities by handle. Their sub-expressions
	// are walked when encountered in the expressions array.
}

// walkBlock walks a block of statements collecting references.
func (c *reachabilityCollector) walkBlock(block []ir.Statement) {
	for _, stmt := range block {
		c.walkStatement(stmt.Kind)
	}
}

// walkStatement collects references from a single statement.
func (c *reachabilityCollector) walkStatement(kind ir.StatementKind) {
	switch k := kind.(type) {
	case ir.StmtCall:
		// Walk called function for types/globals, but don't mark as reachable.
		c.walkCalledFunction(k.Function)
	case ir.StmtIf:
		c.walkBlock(k.Accept)
		c.walkBlock(k.Reject)
	case ir.StmtSwitch:
		for _, sc := range k.Cases {
			c.walkBlock(sc.Body)
		}
	case ir.StmtLoop:
		c.walkBlock(k.Body)
		c.walkBlock(k.Continuing)
	case ir.StmtBlock:
		c.walkBlock(k.Block)
	}
	// StmtEmit, StmtStore, StmtReturn, StmtBreak, StmtContinue, StmtKill,
	// StmtBarrier, StmtImageStore, StmtAtomic, StmtWorkGroupUniformLoad,
	// StmtRayQuery — these reference expressions (already walked) or have
	// no function/global/type handles.
}

// walkCalledFunction walks a called function for types/globals/constants
// without marking it as reachable (function inclusion determined by dominates_global_use).
func (c *reachabilityCollector) walkCalledFunction(h ir.FunctionHandle) {
	if _, already := c.walkedFunctions[h]; already {
		return
	}
	c.walkedFunctions[h] = struct{}{}
	if int(h) < len(c.module.Functions) {
		c.walkFunction(&c.module.Functions[h])
	}
}

// markGlobalReachable marks a global variable and its type as reachable.
func (c *reachabilityCollector) markGlobalReachable(h ir.GlobalVariableHandle) {
	if _, already := c.rs.globals[h]; already {
		return
	}
	c.rs.globals[h] = struct{}{}

	if int(h) < len(c.module.GlobalVariables) {
		global := &c.module.GlobalVariables[h]
		c.markTypeReachable(global.Type)
		// If the global has an initializer, mark that constant too.
		if global.Init != nil {
			c.markConstantReachable(*global.Init)
		}
	}
}

// markConstantReachable marks a constant and its type as reachable.
// For composite constants, also marks sub-constants transitively.
func (c *reachabilityCollector) markConstantReachable(h ir.ConstantHandle) {
	if _, already := c.rs.constants[h]; already {
		return
	}
	c.rs.constants[h] = struct{}{}

	if int(h) < len(c.module.Constants) {
		constant := &c.module.Constants[h]
		c.markTypeReachable(constant.Type)

		// For composite constants, recurse into components.
		if cv, ok := constant.Value.(ir.CompositeValue); ok {
			for _, comp := range cv.Components {
				c.markConstantReachable(comp)
			}
		}
	}
}

// markTypeReachable marks a type and its transitive dependencies as reachable.
// For structs, marks all member types. For arrays, marks the base type.
// For pointers, marks the pointee type.
func (c *reachabilityCollector) markTypeReachable(h ir.TypeHandle) {
	if _, already := c.rs.types[h]; already {
		return
	}
	c.rs.types[h] = struct{}{}

	if int(h) >= len(c.module.Types) {
		return
	}

	typ := &c.module.Types[h]
	switch inner := typ.Inner.(type) {
	case ir.StructType:
		for _, member := range inner.Members {
			c.markTypeReachable(member.Type)
		}
	case ir.ArrayType:
		c.markTypeReachable(inner.Base)
	case ir.PointerType:
		c.markTypeReachable(inner.Base)
	case ir.BindingArrayType:
		c.markTypeReachable(inner.Base)
	}
}
