package ir

// GlobalUse is how a function uses a global variable: Rust naga's
// valid::GlobalUse.
type GlobalUse uint8

const (
	// GlobalUseRead: data is read from the variable.
	GlobalUseRead GlobalUse = 1 << iota
	// GlobalUseWrite: data is written to the variable.
	GlobalUseWrite
	// GlobalUseQuery: information about the data is queried (its array
	// length, an image's size), or the variable is only named.
	GlobalUseQuery
	// GlobalUseAtomic: atomic operations are performed on an image.
	GlobalUseAtomic
)

// FunctionInfo is what validation learns about one function or entry point,
// its callees included: Rust naga's valid::FunctionInfo, so far its
// global_uses. Later fields are additive.
type FunctionInfo struct {
	// GlobalUses is indexed by GlobalVariableHandle; a zero entry means the
	// function does not use the variable.
	GlobalUses []GlobalUse
}

// ModuleInfo is what validation learns about a module: Rust naga's
// valid::ModuleInfo. Functions is indexed by FunctionHandle, EntryPoints
// like Module.EntryPoints. A WebGPU implementation reads from it which
// globals each entry point uses statically, and how (the workgroup storage
// limit, binding sizes, bind group layouts).
type ModuleInfo struct {
	Functions   []FunctionInfo
	EntryPoints []FunctionInfo
}

// analyzeModule computes the ModuleInfo of a valid module, as Rust naga's
// analyzer records global uses (valid/analyzer.rs: add_ref_impl, process_call).
func analyzeModule(m *Module) ModuleInfo {
	a := &useAnalysis{m: m, done: make([]bool, len(m.Functions))}
	info := ModuleInfo{
		Functions:   make([]FunctionInfo, len(m.Functions)),
		EntryPoints: make([]FunctionInfo, len(m.EntryPoints)),
	}
	a.info = info.Functions
	for h := range m.Functions {
		a.function(FunctionHandle(h))
	}
	for i := range m.EntryPoints {
		info.EntryPoints[i] = FunctionInfo{GlobalUses: a.uses(&m.EntryPoints[i].Function)}
	}
	return info
}

type useAnalysis struct {
	m    *Module
	info []FunctionInfo
	done []bool
}

func (a *useAnalysis) function(h FunctionHandle) []GlobalUse {
	if !a.done[h] {
		// The call graph of a valid module is acyclic; marking first only
		// keeps a malformed one from looping.
		a.done[h] = true
		a.info[h] = FunctionInfo{GlobalUses: a.uses(&a.m.Functions[h])}
	}
	return a.info[h].GlobalUses
}

// uses computes fn's global uses. Only an expression that is a global, or
// an access chain into one (Rust's assignable_global), passes a use on to
// the global; an operand that is a value never is one, so only the operand
// positions that can hold a pointer or a handle are recorded.
func (a *useAnalysis) uses(fn *Function) []GlobalUse {
	uses := make([]GlobalUse, len(a.m.GlobalVariables))
	assignable := make([]int, len(fn.Expressions)) // global + 1, 0 for none
	for h, e := range fn.Expressions {
		switch k := e.Kind.(type) {
		case ExprGlobalVariable:
			assignable[h] = int(k.Variable) + 1
		case ExprAccess:
			assignable[h] = assignable[k.Base]
		case ExprAccessIndex:
			assignable[h] = assignable[k.Base]
		}
	}
	add := func(h ExpressionHandle, u GlobalUse) {
		if int(h) < len(assignable) && assignable[h] != 0 {
			uses[assignable[h]-1] |= u
		}
	}
	for _, e := range fn.Expressions {
		switch k := e.Kind.(type) {
		case ExprLoad:
			add(k.Pointer, GlobalUseRead)
		case ExprImageSample:
			add(k.Image, GlobalUseRead)
			add(k.Sampler, GlobalUseRead)
		case ExprImageLoad:
			add(k.Image, GlobalUseRead)
		case ExprImageQuery:
			add(k.Image, GlobalUseQuery)
		case ExprArrayLength:
			add(k.Array, GlobalUseQuery)
		case ExprRayQueryGetIntersection:
			add(k.Query, GlobalUseRead)
		}
	}
	a.block(fn.Body, add, uses)
	// A named expression that only names a global (a phony assignment
	// `_ = g;`) still puts it in the interface.
	for h := range fn.NamedExpressions {
		if int(h) < len(assignable) && assignable[h] != 0 && uses[assignable[h]-1] == 0 {
			uses[assignable[h]-1] = GlobalUseQuery
		}
	}
	return uses
}

func (a *useAnalysis) block(b Block, add func(ExpressionHandle, GlobalUse), uses []GlobalUse) {
	for _, stmt := range b {
		switch s := stmt.Kind.(type) {
		case StmtStore:
			add(s.Pointer, GlobalUseWrite)
		case StmtImageStore:
			add(s.Image, GlobalUseWrite)
		case StmtAtomic:
			add(s.Pointer, GlobalUseRead|GlobalUseWrite)
		case StmtImageAtomic:
			add(s.Image, GlobalUseAtomic)
		case StmtWorkGroupUniformLoad:
			add(s.Pointer, GlobalUseRead)
		case StmtRayQuery:
			add(s.Query, GlobalUseRead)
			if init, ok := s.Fun.(RayQueryInitialize); ok {
				add(init.AccelerationStructure, GlobalUseRead)
			}
		case StmtCall:
			for _, arg := range s.Arguments {
				add(arg, GlobalUseRead)
			}
			if int(s.Function) < len(a.m.Functions) {
				for g, u := range a.function(s.Function) {
					uses[g] |= u
				}
			}
		case StmtBlock:
			a.block(s.Block, add, uses)
		case StmtIf:
			a.block(s.Accept, add, uses)
			a.block(s.Reject, add, uses)
		case StmtSwitch:
			for _, c := range s.Cases {
				a.block(c.Body, add, uses)
			}
		case StmtLoop:
			a.block(s.Body, add, uses)
			a.block(s.Continuing, add, uses)
		}
	}
}
