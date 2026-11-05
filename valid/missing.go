package valid

import (
	"github.com/gogpu/naga/ir"
)

// MissingCapabilities returns the capabilities module needs that supported
// lacks: the ones Validate would report as missing with Options{Capabilities:
// supported}. A backend uses it to refuse a module it cannot write (the
// SupportedCapabilities of each backend package).
//
// It is a walk of the module that reads only what decides a capability
// (types, literals, global variables, the statements that gate one, entry
// point interfaces), with Validate's own checks for each, and none of the
// rest of validation: no typing of expressions, no scope or control-flow
// checks, no analysis. A backend calls it on a module that is already
// validated, so it costs a small part of a compile.
//
// It sees the capabilities Validate checks, which are the ones with a Cap*
// constant here and an IR construct to gate: a feature Validate does not
// gate (core WebGPU features, the *_NON_UNIFORM_INDEXING capabilities) is
// never reported. On a malformed module it reports what it can reach and
// does not fail. A nil module needs nothing.
func MissingCapabilities(module *ir.Module, supported Capabilities) Capabilities {
	if module == nil {
		return 0
	}
	v := &validator{
		module:                 module,
		capabilities:           supported,
		functionStages:         make([]ShaderStages, len(module.Functions)),
		functionSubgroupStages: make([]ShaderStages, len(module.Functions)),
	}
	v.subgroupStages, v.subgroupOperations = subgroupSets(Options{Capabilities: supported})
	v.collectMissing()
	return v.missing
}

// collectMissing is the walk of MissingCapabilities: the capability part of
// validateTypes, validateGlobalExpressionLiterals, validateGlobalVariables,
// validateFunctions and validateEntryPoints.
func (v *validator) collectMissing() {
	for i := range v.module.Types {
		v.collectTypeCapabilities(ir.TypeHandle(i), v.module.Types[i].Inner)
	}
	for i := range v.module.GlobalExpressions {
		if lit, ok := v.module.GlobalExpressions[i].Kind.(ir.Literal); ok {
			v.collectLiteral(lit.Value)
		}
	}
	for i := range v.module.GlobalVariables {
		// globalVariableCapability asks has for each capability the
		// variable needs, which remembers the missing ones.
		v.globalVariableCapability(&v.module.GlobalVariables[i])
	}
	for i := range v.module.Functions {
		v.context = validationContext{function: &v.module.Functions[i], subgroupStages: allStages}
		v.collectFunction(&v.module.Functions[i])
		v.functionSubgroupStages[i] = v.context.subgroupStages
		v.validatedFunctions = i + 1
	}
	for i := range v.module.EntryPoints {
		ep := &v.module.EntryPoints[i]
		v.context = validationContext{function: &ep.Function, subgroupStages: allStages}
		v.collectEntryPointCapabilities(ep)
		v.collectFunction(&ep.Function)
		if ep.Stage == ir.StageVertex && v.context.subgroupStages&ShaderStagesVertex == 0 {
			// A subgroup operation in a vertex shader needs
			// SUBGROUP_VERTEX_STAGE; Rust reports it as a forbidden stage.
			v.missing |= CapSubgroupVertexStage
		}
	}
}

// collectTypeCapabilities is validateType's capability part: the scalar of
// a scalar, vector, matrix or atomic, and what checkTypeCapabilities finds.
func (v *validator) collectTypeCapabilities(handle ir.TypeHandle, inner ir.TypeInner) {
	if inner == nil {
		return
	}
	v.checkTypeCapabilities(handle, inner)
	switch t := inner.(type) {
	case ir.ScalarType:
		v.checkScalarCapability(handle, t, "")
	case ir.VectorType:
		v.checkScalarCapability(handle, t.Scalar, "")
	case ir.MatrixType:
		v.checkScalarCapability(handle, t.Scalar, "")
	case ir.AtomicType:
		v.checkScalarCapability(handle, t.Scalar, "")
	}
}

// collectLiteral is the capability part of a literal's validation:
// literalWidthError asks has, which remembers the missing capability.
func (v *validator) collectLiteral(lit ir.LiteralValue) {
	_ = v.literalWidthError(lit)
}

// collectEntryPointCapabilities is what an entry point's interface needs:
// checkEntryPointCapabilities and the capability part of validateVaryings.
func (v *validator) collectEntryPointCapabilities(ep *ir.EntryPoint) {
	v.checkEntryPointCapabilities(ep)
	fn := &ep.Function
	for i := range fn.Arguments {
		v.collectVaryingCapabilities(fn.Arguments[i].Type, fn.Arguments[i].Binding)
	}
	if fn.Result != nil {
		v.collectVaryingCapabilities(fn.Result.Type, fn.Result.Binding)
	}
}

// collectVaryingCapabilities is the capability part of validateVarying: a
// built-in's capability, @per_primitive, and per_vertex interpolation. A
// struct without a binding is walked by its members.
func (v *validator) collectVaryingCapabilities(ty ir.TypeHandle, binding *ir.Binding) {
	if binding == nil {
		if !v.isValidTypeHandle(ty) {
			return
		}
		if st, ok := v.module.Types[ty].Inner.(ir.StructType); ok {
			for i := range st.Members {
				if st.Members[i].Binding != nil {
					v.collectVaryingCapabilities(st.Members[i].Type, st.Members[i].Binding)
				}
			}
		}
		return
	}
	switch b := (*binding).(type) {
	case ir.BuiltinBinding:
		if c := builtinCapability(b.Builtin); c != 0 {
			v.has(c)
		}
	case ir.LocationBinding:
		if b.PerPrimitive {
			v.has(CapMeshShader)
		}
		if b.Interpolation != nil && *b.Interpolation == ir.InterpolationPerVertex {
			v.has(CapPerVertex)
		}
	}
}

// collectFunction is validateFunction's capability part: the literals among
// the expressions, the stages subgroup operations and calls narrow, and the
// statements that need a capability.
func (v *validator) collectFunction(fn *ir.Function) {
	for i := range fn.Expressions {
		switch k := fn.Expressions[i].Kind.(type) {
		case ir.Literal:
			v.collectLiteral(k.Value)
		case ir.ExprSubgroupBallotResult, ir.ExprSubgroupOperationResult:
			v.context.subgroupStages &= v.subgroupStages
		case ir.ExprCallResult:
			v.narrowSubgroupStagesByCallee(k.Function)
		}
	}
	v.collectBlock(fn, fn.Body)
}

// collectBlock walks a block for the statements that gate a capability.
func (v *validator) collectBlock(fn *ir.Function, block ir.Block) {
	for i := range block {
		switch k := block[i].Kind.(type) {
		case ir.StmtBlock:
			v.collectBlock(fn, k.Block)
		case ir.StmtIf:
			v.collectBlock(fn, k.Accept)
			v.collectBlock(fn, k.Reject)
		case ir.StmtSwitch:
			for c := range k.Cases {
				v.collectBlock(fn, k.Cases[c].Body)
			}
		case ir.StmtLoop:
			v.collectBlock(fn, k.Body)
			v.collectBlock(fn, k.Continuing)
		case ir.StmtControlBarrier:
			v.collectBarrier(k.Flags)
		case ir.StmtMemoryBarrier:
			v.collectBarrier(k.Flags)
		case ir.StmtSubgroupBallot, ir.StmtSubgroupCollectiveOperation, ir.StmtSubgroupGather:
			v.context.subgroupStages &= v.subgroupStages
			v.has(CapSubgroup)
		case ir.StmtCall:
			v.narrowSubgroupStagesByCallee(k.Function)
		case ir.StmtAtomic:
			v.collectAtomic(fn, k)
		case ir.StmtImageAtomic:
			v.collectImageAtomic(fn, k)
		}
	}
}

// collectBarrier is checkBarrier's capability part.
func (v *validator) collectBarrier(flags ir.BarrierFlags) {
	if flags&ir.BarrierSubGroup != 0 {
		v.has(CapSubgroup | CapSubgroupBarrier)
	}
}

// collectAtomic is checkAtomic's capability part, with the pointer's atomic
// found by walking the access chain instead of typing the expression.
func (v *validator) collectAtomic(fn *ir.Function, st ir.StmtAtomic) {
	space, scalar, ok := v.atomicOfChain(fn, st.Pointer)
	if !ok {
		return
	}
	v.atomicNeeds(space, scalar, st.Fun, st.Result != nil)
}

// atomicOfChain follows a pointer expression that is a chain of accesses to
// a variable or an argument and returns the address space and scalar of the
// atomic it points to; false for a pointer it cannot follow or one that
// does not point to an atomic.
func (v *validator) atomicOfChain(fn *ir.Function, h ir.ExpressionHandle) (ir.AddressSpace, ir.ScalarType, bool) {
	space, ty, ok := v.chainType(fn, h, 0)
	if !ok || !v.isValidTypeHandle(ty) {
		return 0, ir.ScalarType{}, false
	}
	at, isAtomic := v.module.Types[ty].Inner.(ir.AtomicType)
	if !isAtomic {
		return 0, ir.ScalarType{}, false
	}
	return space, at.Scalar, true
}

// maxChainDepth bounds chainType's recursion on a malformed module whose
// expressions refer to each other.
const maxChainDepth = 64

// chainType returns the address space and pointee type of a pointer
// expression that is a variable, an argument or an access into one.
func (v *validator) chainType(fn *ir.Function, h ir.ExpressionHandle, depth int) (ir.AddressSpace, ir.TypeHandle, bool) {
	if depth > maxChainDepth || int(h) >= len(fn.Expressions) {
		return 0, 0, false
	}
	switch k := fn.Expressions[h].Kind.(type) {
	case ir.ExprGlobalVariable:
		if !v.isValidGlobalVariableHandle(k.Variable) {
			return 0, 0, false
		}
		gv := &v.module.GlobalVariables[k.Variable]
		return gv.Space, gv.Type, true
	case ir.ExprLocalVariable:
		if int(k.Variable) >= len(fn.LocalVars) {
			return 0, 0, false
		}
		return ir.SpaceFunction, fn.LocalVars[k.Variable].Type, true
	case ir.ExprFunctionArgument:
		if int(k.Index) >= len(fn.Arguments) || !v.isValidTypeHandle(fn.Arguments[k.Index].Type) {
			return 0, 0, false
		}
		ptr, ok := v.module.Types[fn.Arguments[k.Index].Type].Inner.(ir.PointerType)
		if !ok {
			return 0, 0, false
		}
		return ptr.Space, ptr.Base, true
	case ir.ExprAccessIndex:
		space, ty, ok := v.chainType(fn, k.Base, depth+1)
		if !ok || !v.isValidTypeHandle(ty) {
			return 0, 0, false
		}
		if st, isStruct := v.module.Types[ty].Inner.(ir.StructType); isStruct {
			if int(k.Index) >= len(st.Members) {
				return 0, 0, false
			}
			return space, st.Members[k.Index].Type, true
		}
		elem, ok := v.elementType(ty)
		return space, elem, ok
	case ir.ExprAccess:
		space, ty, ok := v.chainType(fn, k.Base, depth+1)
		if !ok || !v.isValidTypeHandle(ty) {
			return 0, 0, false
		}
		elem, ok := v.elementType(ty)
		return space, elem, ok
	}
	return 0, 0, false
}

// elementType is the element type of an array or binding array, the types
// an access chain indexes besides a struct. A vector or matrix element has
// no type handle and is never an atomic.
func (v *validator) elementType(ty ir.TypeHandle) (ir.TypeHandle, bool) {
	switch t := v.module.Types[ty].Inner.(type) {
	case ir.ArrayType:
		return t.Base, true
	case ir.BindingArrayType:
		return t.Base, true
	}
	return 0, false
}

// collectImageAtomic is checkImageAtomic, which reads the image through its
// access chain without typing, and reports only on a missing capability.
func (v *validator) collectImageAtomic(_ *ir.Function, st ir.StmtImageAtomic) {
	v.checkImageAtomic(0, st)
}
